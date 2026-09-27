package vaultservice

import (
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	usageVersion    byte = 1
	usageRecordSize int  = 24
	maxUsageEntries int  = 50
)

// ErrUsageCorrupt reports a device usage record that fails to decode.
var ErrUsageCorrupt = errors.New("device usage record is invalid")

type usageEntry struct {
	id         vault.ID
	lastUsedAt int64
}

// usageLog lists recently used credentials, most recent first; it must hold only IDs and timestamps.
type usageLog struct {
	entries []usageEntry
}

func (l *usageLog) touch(id vault.ID, at int64) {
	next := make([]usageEntry, 0, min(len(l.entries)+1, maxUsageEntries))
	next = append(next, usageEntry{id: id, lastUsedAt: at})
	for _, entry := range l.entries {
		if len(next) == maxUsageEntries {
			break
		}
		if entry.id == id {
			continue
		}
		next = append(next, entry)
	}
	l.entries = next
}

func (l usageLog) encode() []byte {
	data := make([]byte, 1+len(l.entries)*usageRecordSize)
	data[0] = usageVersion
	offset := 1
	for _, entry := range l.entries {
		copy(data[offset:offset+16], entry.id[:])
		binary.BigEndian.PutUint64(data[offset+16:offset+usageRecordSize], uint64(entry.lastUsedAt))
		offset += usageRecordSize
	}
	return data
}

func decodeUsage(data []byte) (usageLog, error) {
	if len(data) == 0 || data[0] != usageVersion {
		return usageLog{}, ErrUsageCorrupt
	}
	body := data[1:]
	if len(body)%usageRecordSize != 0 || len(body)/usageRecordSize > maxUsageEntries {
		return usageLog{}, ErrUsageCorrupt
	}
	count := len(body) / usageRecordSize
	log := usageLog{entries: make([]usageEntry, 0, count)}
	seen := make(map[vault.ID]struct{}, count)
	for i := range count {
		record := body[i*usageRecordSize:]
		var id vault.ID
		copy(id[:], record[:16])
		lastUsedAt := int64(binary.BigEndian.Uint64(record[16:usageRecordSize]))
		if lastUsedAt <= 0 {
			return usageLog{}, ErrUsageCorrupt
		}
		if _, duplicate := seen[id]; duplicate {
			return usageLog{}, ErrUsageCorrupt
		}
		seen[id] = struct{}{}
		log.entries = append(log.entries, usageEntry{id: id, lastUsedAt: lastUsedAt})
	}
	return log, nil
}

// Usage reports each recorded credential's last use in Unix milliseconds.
func (s *Service) Usage() (map[vault.ID]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, ErrNotReady
	}
	return s.lastUses()
}

// lastUses is Usage for a caller that holds s.mu with a vault open.
func (s *Service) lastUses() (map[vault.ID]int64, error) {
	head, err := s.session.Head()
	if err != nil {
		return nil, err
	}
	log := s.loadUsage(head.VaultID.String())
	result := make(map[vault.ID]int64, len(log.entries))
	for _, entry := range log.entries {
		result[entry.id] = entry.lastUsedAt
	}
	return result, nil
}

// setFirst orders an item whose flag is set before one whose flag is not.
func setFirst(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return -1
	default:
		return 1
	}
}

// byUseThenLabel orders by most recent use, then by label without case.
func byUseThenLabel(aUsed, bUsed int64, aLabel, bLabel string) int {
	if order := cmp.Compare(bUsed, aUsed); order != 0 {
		return order
	}
	return strings.Compare(strings.ToLower(aLabel), strings.ToLower(bLabel))
}

// MarkUsed records an existing credential as used now.
func (s *Service) MarkUsed(id vault.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	entries, err := s.session.List()
	if err != nil {
		return err
	}
	known := false
	for _, entry := range entries {
		if entry.ID == id {
			known = true
			break
		}
	}
	if !known {
		return vault.ErrNotFound
	}
	return s.recordUse(id)
}

// recordUse records an item the open vault holds as used now. The caller holds s.mu.
func (s *Service) recordUse(id vault.ID) error {
	head, err := s.session.Head()
	if err != nil {
		return err
	}
	name := head.VaultID.String()
	log := s.loadUsage(name)
	log.touch(id, time.Now().UnixMilli())
	if err := s.keys.SaveUsageRecord(name, log.encode()); err != nil {
		return fmt.Errorf("store recent usage: %w", err)
	}
	s.device.rememberUsage(log)
	return nil
}

// loadUsage never fails; an unreadable or invalid record reads as empty.
func (s *Service) loadUsage(vaultID string) usageLog {
	if s.device.usageLoaded {
		return s.device.usage
	}
	log := usageLog{}
	if data, err := s.keys.LoadUsageRecord(vaultID); err == nil {
		if decoded, err := decodeUsage(data); err == nil {
			log = decoded
		}
	}
	s.device.rememberUsage(log)
	return log
}
