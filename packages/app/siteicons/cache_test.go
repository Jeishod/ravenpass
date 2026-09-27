package siteicons

import (
	"fmt"
	"testing"
	"time"
)

func TestCacheFreshness(t *testing.T) {
	start := time.Unix(1_790_000_000, 0)
	icons := newCache()
	icons.store("icon.example", []byte{1, 2, 3}, start)
	icons.store("miss.example", nil, start)
	tests := []struct {
		site  string
		after time.Duration
		fresh bool
	}{
		{"icon.example", iconLifetime - time.Second, true},
		{"icon.example", iconLifetime, false},
		{"miss.example", missLifetime - time.Second, true},
		{"miss.example", missLifetime, false},
	}
	for _, test := range tests {
		found := icons.lookup(test.site, start.Add(test.after))
		if !found.known || found.fresh != test.fresh {
			t.Fatalf("%s after %v = %+v", test.site, test.after, found)
		}
	}
	if found := icons.lookup("icon.example", start); found.icon != "AQID" {
		t.Fatalf("icon = %q", found.icon)
	}
	if found := icons.lookup("unknown.example", start); found.known {
		t.Fatalf("an unknown site was found: %+v", found)
	}
}

func TestCacheEvictsTheSitesCheckedLongestAgo(t *testing.T) {
	start := time.Unix(1_790_000_000, 0)
	icons := newCache()
	for i := range maxSites + 2 {
		icons.store(fmt.Sprintf("site%d.example", i), nil, start.Add(time.Duration(i)*time.Second))
	}
	if len(icons.sites) != maxSites {
		t.Fatalf("cache holds %d sites", len(icons.sites))
	}
	for _, evicted := range []string{"site0.example", "site1.example"} {
		if icons.lookup(evicted, start).known {
			t.Fatalf("%s was kept", evicted)
		}
	}
	if !icons.lookup(fmt.Sprintf("site%d.example", maxSites+1), start).known {
		t.Fatal("the newest site was evicted")
	}
}

func TestCacheStaysWithinItsSize(t *testing.T) {
	start := time.Unix(1_790_000_000, 0)
	icons := newCache()
	large := make([]byte, maxCacheBytes/3)
	for i := range 4 {
		icons.store(fmt.Sprintf("site%d.example", i), large, start.Add(time.Duration(i)*time.Second))
	}
	encoded, err := icons.encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > maxCacheBytes {
		t.Fatalf("encoded cache is %d bytes", len(encoded))
	}
	if icons.lookup("site0.example", start).known || !icons.lookup("site3.example", start).known {
		t.Fatal("the size bound did not evict the oldest site")
	}
}

func TestCacheRoundTripsAndRefusesOtherContent(t *testing.T) {
	start := time.Unix(1_790_000_000, 0)
	icons := newCache()
	icons.store("google.com", []byte{0x89, 'P', 'N', 'G'}, start)
	icons.store("miss.example", nil, start)
	encoded, err := icons.encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeCache(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.lookup("google.com", start) != icons.lookup("google.com", start) || !decoded.lookup("miss.example", start).known {
		t.Fatalf("decoded cache = %+v", decoded.sites)
	}
	for _, content := range []string{
		"",
		"{",
		`{"version":1,"sites":{}}`,
		`{"sites":{}}`,
		`{"version":1,"sites":{"":{"icon":"","checkedAt":1}}}`,
		`{"version":1,"sites":{"a.example":{"icon":"%%%","checkedAt":1}}}`,
		`{"version":1,"sites":{"a.example":{"icon":"","checkedAt":0}}}`,
	} {
		if _, err := decodeCache([]byte(content)); err == nil {
			t.Fatalf("%q was read as a cache", content)
		}
	}
}
