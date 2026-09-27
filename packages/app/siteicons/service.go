// Package siteicons fetches website icons and caches them per vault in a file sealed by that vault.
package siteicons

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/app/privatefile"
	"github.com/dortanes/ravenpass/packages/vault"
	"golang.org/x/sync/singleflight"
)

const (
	cachePurpose    = "site-icons"
	concurrentHosts = 4
	writeDelay      = 2 * time.Second
	// maxCacheFileSize is a sealed cache's ciphertext limit plus its envelope.
	maxCacheFileSize = 32<<20 + 1<<10
)

// Vault is the open vault as the icon cache needs it; sealing for a vault no longer open fails.
type Vault interface {
	OpenVaultID() (vault.ID, error)
	SealDeviceCache(id vault.ID, name string, plaintext []byte) ([]byte, error)
	OpenDeviceCache(id vault.ID, name string, sealed []byte) ([]byte, error)
}

// openCache is the open vault's cache, loaded on the first request after an unlock.
type openCache struct {
	vaultID vault.ID
	icons   *cache
	dirty   bool
}

// Service answers icon requests for the open vault, one fetch per site and concurrentHosts at once.
type Service struct {
	directory  string
	vault      Vault
	source     Source
	now        func() time.Time
	writeDelay time.Duration
	slots      chan struct{}
	flights    singleflight.Group

	mu      sync.Mutex
	enabled bool
	// generation changes on every drop; work started under an older one never touches the cache.
	generation uint64
	work       context.Context
	cancel     context.CancelFunc
	open       *openCache
	timer      *time.Timer
}

// New returns a service caching icons in directory.
func New(directory string, vault Vault, source Source, enabled bool) (*Service, error) {
	if directory == "" || vault == nil || source == nil {
		return nil, errors.New("icon directory, vault and icon source are required")
	}
	service := &Service{
		directory:  directory,
		vault:      vault,
		source:     source,
		now:        time.Now,
		writeDelay: writeDelay,
		slots:      make(chan struct{}, concurrentHosts),
		enabled:    enabled,
	}
	service.work, service.cancel = context.WithCancel(context.Background())
	return service, nil
}

// Icon returns a site's base64 PNG icon, empty for none; a stale one returns while a fresh one is fetched.
func (s *Service) Icon(site string) (string, error) {
	s.mu.Lock()
	found, searched, err := s.find(site)
	generation, work := s.generation, s.work
	s.mu.Unlock()

	switch {
	case err != nil || !searched:
		return "", err
	case found.fresh:
		return found.icon, nil
	case found.known && found.icon != "":
		go s.fetch(work, generation, site)
		return found.icon, nil
	default:
		return s.fetch(work, generation, site), nil
	}
}

// Cached returns the icon the cache holds for a site, fresh or stale, without fetching.
func (s *Service) Cached(site string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, _, err := s.find(site)
	return found.icon, err
}

// find looks a site up in the open cache; false while icons are off or for a never-public site. The caller holds s.mu.
func (s *Service) find(site string) (lookup, bool, error) {
	if !s.enabled || site == "" || neverPublic(site) {
		return lookup{}, false, nil
	}
	if err := s.load(); err != nil {
		return lookup{}, false, err
	}
	return s.open.icons.lookup(site, s.now()), true, nil
}

// SetEnabled turns icon loading on or off; off stops every fetch and deletes every cache.
func (s *Service) SetEnabled(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	if enabled {
		return nil
	}
	s.drop()
	return os.RemoveAll(s.directory)
}

// Forget deletes a vault's cache file, for a vault erased from this device.
func (s *Service) Forget(id vault.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.open != nil && s.open.vaultID == id {
		s.drop()
	}
	if err := os.Remove(s.path(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Release writes pending changes and forgets the icons; call it while the vault can still seal.
func (s *Service) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.write()
	s.drop()
}

// fetch asks the source for a site's icon and records it; a site that did not answer is not recorded.
func (s *Service) fetch(work context.Context, generation uint64, site string) string {
	icon, err, _ := s.flights.Do(fmt.Sprint(generation, "/", site), func() (any, error) {
		if recorded, fresh := s.recorded(generation, site); fresh {
			return recorded, nil
		}
		select {
		case s.slots <- struct{}{}:
		case <-work.Done():
			return "", work.Err()
		}
		defer func() { <-s.slots }()
		data, err := s.source.Fetch(work, site)
		if err != nil {
			return "", err
		}
		return s.record(generation, site, data), nil
	})
	if err != nil {
		return ""
	}
	return icon.(string)
}

// recorded returns the icon generation's cache holds for a site and whether it is fresh.
func (s *Service) recorded(generation uint64, site string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.generation || s.open == nil {
		return "", false
	}
	found := s.open.icons.lookup(site, s.now())
	return found.icon, found.fresh
}

// record stores a fetch result in the cache it was fetched for and returns the stored icon.
func (s *Service) record(generation uint64, site string, icon []byte) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.generation || s.open == nil {
		return ""
	}
	stored := s.open.icons.store(site, icon, s.now())
	s.open.dirty = true
	s.scheduleWrite(generation)
	return stored
}

// load makes the open vault's cache current; an unreadable file is deleted and the cache starts empty.
func (s *Service) load() error {
	id, err := s.vault.OpenVaultID()
	if err != nil {
		return err
	}
	if s.open != nil && s.open.vaultID == id {
		return nil
	}
	s.drop()
	icons, err := s.readFile(id)
	if err != nil {
		return err
	}
	s.open = &openCache{vaultID: id, icons: icons}
	return nil
}

func (s *Service) readFile(id vault.ID) (*cache, error) {
	path := s.path(id)
	sealed, err := privatefile.Read(path, maxCacheFileSize)
	if errors.Is(err, fs.ErrNotExist) {
		return newCache(), nil
	}
	if err != nil {
		return s.discard(path)
	}
	plaintext, err := s.vault.OpenDeviceCache(id, cachePurpose, sealed)
	if err != nil {
		if unreadable(err) {
			return s.discard(path)
		}
		return nil, err
	}
	icons, err := decodeCache(plaintext)
	if err != nil {
		return s.discard(path)
	}
	return icons, nil
}

// unreadable reports a cache file whose own content failed to open.
func unreadable(err error) bool {
	return errors.Is(err, vault.ErrAuthentication) || errors.Is(err, vault.ErrMalformed) ||
		errors.Is(err, vault.ErrUnsupported) || errors.Is(err, vault.ErrResourceLimit)
}

func (s *Service) discard(path string) (*cache, error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return newCache(), nil
}

func (s *Service) scheduleWrite(generation uint64) {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.writeDelay, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if generation == s.generation {
			s.write()
		}
	})
}

// write seals and writes the open cache if it changed; changes that cannot be written are dropped.
func (s *Service) write() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if s.open == nil || !s.open.dirty {
		return
	}
	s.open.dirty = false
	plaintext, err := s.open.icons.encode()
	if err != nil {
		slog.Warn("encode site icon cache", "err", err)
		return
	}
	sealed, err := s.vault.SealDeviceCache(s.open.vaultID, cachePurpose, plaintext)
	clear(plaintext)
	if err != nil {
		slog.Warn("seal site icon cache", "err", err)
		return
	}
	if err := privatefile.Write(s.path(s.open.vaultID), sealed); err != nil {
		slog.Warn("write site icon cache", "err", err)
	}
}

// drop forgets the open cache and stops the work started for it.
func (s *Service) drop() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.open = nil
	s.cancel()
	s.generation++
	s.work, s.cancel = context.WithCancel(context.Background())
}

func (s *Service) path(id vault.ID) string {
	return filepath.Join(s.directory, id.String())
}
