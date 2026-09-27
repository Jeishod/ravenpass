package siteicons

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

const (
	cacheVersion = 3
	iconLifetime = 30 * 24 * time.Hour
	missLifetime = 7 * 24 * time.Hour
	maxSites     = 2000
	// maxCacheBytes stays under the 32 MiB a sealed device cache may hold.
	maxCacheBytes = 24 << 20
	// siteOverhead bounds what one site adds to the encoded cache beyond its name and icon.
	siteOverhead = 64
)

var errCacheUnreadable = errors.New("icon cache is unreadable")

// siteRecord is a site's base64 PNG icon, empty for none, and CheckedAt in Unix seconds.
type siteRecord struct {
	Icon      string `json:"icon"`
	CheckedAt int64  `json:"checkedAt"`
}

type cacheFile struct {
	Version int                   `json:"version"`
	Sites   map[string]siteRecord `json:"sites"`
}

// lookup is what the cache holds for a site.
type lookup struct {
	known bool
	icon  string
	fresh bool
}

// cache holds the icons of one vault's sites in memory, bounded by site count and size.
type cache struct {
	sites map[string]siteRecord
	size  int
}

func newCache() *cache {
	return &cache{sites: make(map[string]siteRecord)}
}

// decodeCache reads an encoded cache; anything but a well-formed current version is errCacheUnreadable.
func decodeCache(data []byte) (*cache, error) {
	var file cacheFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != cacheVersion {
		return nil, errCacheUnreadable
	}
	result := newCache()
	for site, record := range file.Sites {
		if site == "" || record.CheckedAt <= 0 {
			return nil, errCacheUnreadable
		}
		if _, err := base64.StdEncoding.DecodeString(record.Icon); err != nil {
			return nil, errCacheUnreadable
		}
		result.put(site, record)
	}
	return result, nil
}

func (c *cache) encode() ([]byte, error) {
	return json.Marshal(cacheFile{Version: cacheVersion, Sites: c.sites})
}

func (c *cache) lookup(site string, now time.Time) lookup {
	record, known := c.sites[site]
	if !known {
		return lookup{}
	}
	lifetime := iconLifetime
	if record.Icon == "" {
		lifetime = missLifetime
	}
	fresh := now.Before(time.Unix(record.CheckedAt, 0).Add(lifetime))
	return lookup{known: true, icon: record.Icon, fresh: fresh}
}

// store records a fetched PNG, or nil for none, and returns the icon as the cache holds it.
func (c *cache) store(site string, icon []byte, now time.Time) string {
	record := siteRecord{Icon: base64.StdEncoding.EncodeToString(icon), CheckedAt: now.Unix()}
	c.put(site, record)
	return record.Icon
}

// put records a site and evicts the sites checked longest ago until the cache is within bounds.
func (c *cache) put(site string, record siteRecord) {
	c.remove(site)
	c.sites[site] = record
	c.size += recordSize(site, record)
	for len(c.sites) > maxSites || c.size > maxCacheBytes {
		c.remove(c.oldest())
	}
}

func (c *cache) remove(site string) {
	if record, known := c.sites[site]; known {
		c.size -= recordSize(site, record)
		delete(c.sites, site)
	}
}

func (c *cache) oldest() string {
	var oldest string
	var checkedAt int64
	for site, record := range c.sites {
		if oldest == "" || record.CheckedAt < checkedAt || record.CheckedAt == checkedAt && site < oldest {
			oldest, checkedAt = site, record.CheckedAt
		}
	}
	return oldest
}

func recordSize(site string, record siteRecord) int {
	return len(site) + len(record.Icon) + siteOverhead
}
