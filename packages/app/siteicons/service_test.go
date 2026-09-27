package siteicons

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

// sessionVault seals with a real vault session and refuses once locked.
type sessionVault struct {
	mu      sync.Mutex
	session *vault.Session
	id      vault.ID
	locked  bool
}

func newSessionVault(t *testing.T) *sessionVault {
	t.Helper()
	created, err := vault.Create()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(created.Session.Lock)
	head, err := created.Session.Head()
	if err != nil {
		t.Fatal(err)
	}
	return &sessionVault{session: created.Session, id: head.VaultID}
}

func (v *sessionVault) lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.locked = true
}

func (v *sessionVault) OpenVaultID() (vault.ID, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.locked {
		return vault.ID{}, vault.ErrLocked
	}
	return v.id, nil
}

func (v *sessionVault) SealDeviceCache(id vault.ID, name string, plaintext []byte) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.locked || id != v.id {
		return nil, vault.ErrLocked
	}
	return v.session.SealDeviceCache(name, plaintext)
}

func (v *sessionVault) OpenDeviceCache(id vault.ID, name string, sealed []byte) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.locked || id != v.id {
		return nil, vault.ErrLocked
	}
	return v.session.OpenDeviceCache(name, sealed)
}

// fakeSource answers every site with the same result and counts the fetches.
type fakeSource struct {
	mu      sync.Mutex
	icon    []byte
	err     error
	calls   int
	gate    chan struct{}
	started chan struct{}
}

func (f *fakeSource) Fetch(ctx context.Context, _ string) ([]byte, error) {
	f.mu.Lock()
	f.calls++
	gate, started := f.gate, f.started
	icon, err := f.icon, f.err
	f.mu.Unlock()
	if started != nil {
		started <- struct{}{}
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return icon, err
}

func (f *fakeSource) answer(icon []byte, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.icon, f.err = icon, err
}

func (f *fakeSource) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) read() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(by)
}

func newTestService(t *testing.T, directory string, owner Vault, source Source) (*Service, *testClock) {
	t.Helper()
	service, err := New(directory, owner, source, true)
	if err != nil {
		t.Fatal(err)
	}
	clock := &testClock{now: time.Unix(1_790_000_000, 0)}
	service.now = clock.read
	service.writeDelay = time.Hour
	t.Cleanup(func() { _ = service.SetEnabled(false) })
	return service, clock
}

func testIcon(t *testing.T, red uint8) []byte {
	t.Helper()
	source := picture(IconSize, IconSize)
	source.Pix[0] = red
	return encodePNG(t, source)
}

func icon(t *testing.T, service *Service, site string) string {
	t.Helper()
	value, err := service.Icon(site)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestConcurrentRequestsForOneSiteShareOneFetch(t *testing.T) {
	png := testIcon(t, 1)
	source := &fakeSource{icon: png, gate: make(chan struct{}), started: make(chan struct{}, 10)}
	service, _ := newTestService(t, t.TempDir(), newSessionVault(t), source)
	results := make(chan string, 10)
	for range 10 {
		go func() {
			value, _ := service.Icon("google.com")
			results <- value
		}()
	}
	<-source.started
	close(source.gate)
	want := base64.StdEncoding.EncodeToString(png)
	for range 10 {
		if got := <-results; got != want {
			t.Fatalf("a request got %q", got)
		}
	}
	if calls := source.count(); calls != 1 {
		t.Fatalf("%d fetches for one site", calls)
	}
}

func TestSiteWithoutAnIconIsAskedAgainAfterSevenDays(t *testing.T) {
	source := &fakeSource{}
	service, clock := newTestService(t, t.TempDir(), newSessionVault(t), source)
	if got := icon(t, service, "plain.example"); got != "" {
		t.Fatalf("a site without an icon gave %q", got)
	}
	clock.advance(missLifetime - time.Second)
	if got := icon(t, service, "plain.example"); got != "" || source.count() != 1 {
		t.Fatalf("a recorded miss was fetched again within seven days: %q, %d fetches", got, source.count())
	}
	clock.advance(time.Second)
	icon(t, service, "plain.example")
	if calls := source.count(); calls != 2 {
		t.Fatalf("a miss seven days old was not asked again: %d fetches", calls)
	}
}

func TestSiteThatNeverAnsweredIsNotStored(t *testing.T) {
	directory := t.TempDir()
	source := &fakeSource{err: ErrUnanswered}
	service, _ := newTestService(t, directory, newSessionVault(t), source)
	for range 2 {
		if got := icon(t, service, "offline.example"); got != "" {
			t.Fatalf("an unanswered site gave %q", got)
		}
	}
	if calls := source.count(); calls != 2 {
		t.Fatalf("an unanswered site was fetched %d times, want every request", calls)
	}
	service.Release()
	if entries, _ := os.ReadDir(directory); len(entries) != 0 {
		t.Fatalf("an unanswered site wrote %d cache files", len(entries))
	}
}

func TestIconIsRefreshedAfterThirtyDaysWhileTheStaleOneIsServed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		first, second := testIcon(t, 1), testIcon(t, 2)
		source := &fakeSource{icon: first}
		service, clock := newTestService(t, t.TempDir(), newSessionVault(t), source)
		stale := base64.StdEncoding.EncodeToString(first)
		if got := icon(t, service, "google.com"); got != stale {
			t.Fatalf("first icon = %q", got)
		}
		clock.advance(iconLifetime - time.Second)
		icon(t, service, "google.com")
		if calls := source.count(); calls != 1 {
			t.Fatalf("an icon was fetched again within thirty days: %d fetches", calls)
		}
		clock.advance(time.Second)
		source.answer(second, nil)
		if got := icon(t, service, "google.com"); got != stale {
			t.Fatalf("the stale icon was not served during the refresh: %q", got)
		}
		synctest.Wait()
		if got := icon(t, service, "google.com"); got != base64.StdEncoding.EncodeToString(second) {
			t.Fatalf("the icon was not refreshed: %q", got)
		}
	})
}

func TestCacheSurvivesAReopenSealedByTheVault(t *testing.T) {
	directory := t.TempDir()
	owner := newSessionVault(t)
	png := testIcon(t, 1)
	source := &fakeSource{icon: png}
	service, _ := newTestService(t, directory, owner, source)
	want := icon(t, service, "google.com")
	service.Release()

	sealed, err := os.ReadFile(filepath.Join(directory, owner.id.String()))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("google.com")) {
		t.Fatal("the cache file names a site in plaintext")
	}
	info, err := os.Stat(filepath.Join(directory, owner.id.String()))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("cache file mode = %04o", info.Mode().Perm())
	}

	reopenedSource := &fakeSource{err: ErrUnanswered}
	reopened, _ := newTestService(t, directory, owner, reopenedSource)
	if got := icon(t, reopened, "google.com"); got != want {
		t.Fatalf("icon after a reopen = %q", got)
	}
	if calls := reopenedSource.count(); calls != 0 {
		t.Fatalf("a cached icon was fetched again after a reopen: %d fetches", calls)
	}
}

func TestChangesAreWrittenAfterTheDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		directory := t.TempDir()
		owner := newSessionVault(t)
		service, _ := newTestService(t, directory, owner, &fakeSource{icon: testIcon(t, 1)})
		service.writeDelay = writeDelay
		icon(t, service, "google.com")
		path := filepath.Join(directory, owner.id.String())
		time.Sleep(writeDelay - time.Millisecond)
		synctest.Wait()
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("the cache was written before the delay: %v", err)
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("the cache was not written after the delay: %v", err)
		}
	})
}

func TestUnreadableCacheIsDiscarded(t *testing.T) {
	owner := newSessionVault(t)
	other := newSessionVault(t)
	fromOtherVault, err := other.session.SealDeviceCache(cachePurpose, []byte(`{"version":1,"sites":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	fromOtherPurpose, err := owner.session.SealDeviceCache("other-cache", []byte(`{"version":1,"sites":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	wrongVersion, err := owner.session.SealDeviceCache(cachePurpose, []byte(`{"version":1,"sites":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"garbage":            []byte("not a cache"),
		"another vault":      fromOtherVault,
		"another purpose":    fromOtherPurpose,
		"another version":    wrongVersion,
		"an empty file":      {},
		"a truncated header": fromOtherPurpose[:10],
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, owner.id.String())
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			source := &fakeSource{icon: testIcon(t, 1)}
			service, _ := newTestService(t, directory, owner, source)
			if got := icon(t, service, "google.com"); got == "" || source.count() != 1 {
				t.Fatalf("icon over an unreadable cache = %q after %d fetches", got, source.count())
			}
			if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("the unreadable cache was kept: %v", err)
			}
			service.Release()
			reopened, _ := newTestService(t, directory, owner, &fakeSource{err: ErrUnanswered})
			if got := icon(t, reopened, "google.com"); got == "" {
				t.Fatal("the rebuilt cache did not keep the icon")
			}
		})
	}
}

func TestTurningIconsOffStopsFetchingAndDeletesTheCache(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "icons")
	owner := newSessionVault(t)
	source := &fakeSource{icon: testIcon(t, 1)}
	service, _ := newTestService(t, directory, owner, source)
	icon(t, service, "google.com")
	service.Release()
	if _, err := os.Stat(filepath.Join(directory, owner.id.String())); err != nil {
		t.Fatal(err)
	}

	source.mu.Lock()
	source.gate, source.started = make(chan struct{}), make(chan struct{}, 10)
	source.mu.Unlock()
	pending := make(chan string, 1)
	go func() {
		value, _ := service.Icon("github.com")
		pending <- value
	}()
	<-source.started
	if err := service.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if got := <-pending; got != "" {
		t.Fatalf("a fetch stopped by turning icons off gave %q", got)
	}
	if _, err := os.Stat(directory); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the icon directory was kept: %v", err)
	}
	calls := source.count()
	if got := icon(t, service, "google.com"); got != "" {
		t.Fatalf("icons off gave %q", got)
	}
	service.Release()
	if source.count() != calls {
		t.Fatal("a site was contacted while icons are off")
	}
	if _, err := os.Stat(directory); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the icon directory came back: %v", err)
	}
}

func TestForgettingAVaultDeletesOnlyItsCache(t *testing.T) {
	directory := t.TempDir()
	owner := newSessionVault(t)
	service, _ := newTestService(t, directory, owner, &fakeSource{icon: testIcon(t, 1)})
	icon(t, service, "google.com")
	service.Release()
	other := filepath.Join(directory, vault.ID{1}.String())
	if err := os.WriteFile(other, []byte("sealed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := service.Forget(owner.id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, owner.id.String())); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the forgotten vault's cache was kept: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("another vault's cache was deleted: %v", err)
	}
	if err := service.Forget(owner.id); err != nil {
		t.Fatalf("forgetting a vault without a cache: %v", err)
	}
}

func TestSiteThatCanNeverBePublicIsNeitherFetchedNorRecorded(t *testing.T) {
	directory := t.TempDir()
	owner := newSessionVault(t)
	source := &fakeSource{icon: testIcon(t, 1)}
	service, _ := newTestService(t, directory, owner, source)
	local := []string{"localhost", "app.localhost", "127.0.0.1", "192.168.1.1", "10.0.0.1", "169.254.1.1", "::1", "::ffff:192.168.1.1", "0.0.0.0", "fe80::1"}
	for _, site := range local {
		if got := icon(t, service, site); got != "" {
			t.Fatalf("%s gave %q", site, got)
		}
	}
	if calls := source.count(); calls != 0 {
		t.Fatalf("local sites reached the source %d times", calls)
	}
	service.Release()
	if entries, _ := os.ReadDir(directory); len(entries) != 0 {
		t.Fatalf("local sites wrote %d cache files", len(entries))
	}
	if got := icon(t, service, "example.com"); got == "" || source.count() != 1 {
		t.Fatalf("a public site gave %q after %d fetches", got, source.count())
	}
	owner.lock()
	for _, site := range local {
		if got, err := service.Icon(site); got != "" || err != nil {
			t.Fatalf("%s read the cache of a locked vault: %q, %v", site, got, err)
		}
	}
}

func TestNeverPublic(t *testing.T) {
	for site, want := range map[string]bool{
		"localhost":          true,
		"app.localhost":      true,
		"127.0.0.1":          true,
		"127.8.9.10":         true,
		"10.0.0.1":           true,
		"172.16.0.1":         true,
		"192.168.1.1":        true,
		"169.254.1.1":        true,
		"0.0.0.0":            true,
		"::1":                true,
		"::":                 true,
		"fe80::1":            true,
		"fd00::1":            true,
		"ff02::1":            true,
		"::ffff:10.0.0.1":    true,
		"example.com":        false,
		"localhost.example":  false,
		"mylocalhost":        false,
		"8.8.8.8":            false,
		"2606:4700::1111":    false,
		"::ffff:8.8.8.8":     false,
		"xn--bcher-kva.test": false,
	} {
		if got := neverPublic(site); got != want {
			t.Errorf("neverPublic(%q) = %v, want %v", site, got, want)
		}
	}
}

func TestCachedServesWhatTheCacheHoldsWithoutFetching(t *testing.T) {
	directory := t.TempDir()
	owner := newSessionVault(t)
	source := &fakeSource{icon: testIcon(t, 1)}
	service, clock := newTestService(t, directory, owner, source)
	want := icon(t, service, "google.com")
	cached := func(from *Service, site string) string {
		t.Helper()
		value, err := from.Cached(site)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	if got := cached(service, "google.com"); got != want {
		t.Fatalf("a cached icon read %q", got)
	}
	if got := cached(service, "github.com"); got != "" || source.count() != 1 {
		t.Fatalf("a site the cache does not hold gave %q after %d fetches", got, source.count())
	}
	clock.advance(iconLifetime)
	if got := cached(service, "google.com"); got != want || source.count() != 1 {
		t.Fatalf("a stale icon read %q after %d fetches", got, source.count())
	}
	service.Release()

	reopenedSource := &fakeSource{icon: testIcon(t, 2)}
	reopened, _ := newTestService(t, directory, owner, reopenedSource)
	if got := cached(reopened, "google.com"); got != want || reopenedSource.count() != 0 {
		t.Fatalf("the cache file gave %q after %d fetches", got, reopenedSource.count())
	}
	if got := cached(reopened, "127.0.0.1"); got != "" {
		t.Fatalf("a site that can never be public gave %q", got)
	}
	if err := reopened.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if got := cached(reopened, "google.com"); got != "" {
		t.Fatalf("icons off gave %q", got)
	}
	if err := reopened.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	owner.lock()
	if _, err := reopened.Cached("google.com"); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("cached while locked: %v", err)
	}
	if reopenedSource.count() != 0 {
		t.Fatalf("reading the cache fetched %d times", reopenedSource.count())
	}
}

func TestLockedVaultServesNoIconAndDropsUnsavedChanges(t *testing.T) {
	directory := t.TempDir()
	owner := newSessionVault(t)
	service, _ := newTestService(t, directory, owner, &fakeSource{icon: testIcon(t, 1)})
	icon(t, service, "google.com")
	owner.lock()
	service.Release()
	if entries, _ := os.ReadDir(directory); len(entries) != 0 {
		t.Fatalf("a locked vault's cache was written: %d files", len(entries))
	}
	if _, err := service.Icon("google.com"); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("icon while locked: %v", err)
	}
}
