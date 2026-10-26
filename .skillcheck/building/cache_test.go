package building

import (
	"testing"
)

// TestCacheHitByPath verifies that caching a file and retrieving it by the
// same path returns the cached data.
func TestCacheHitByPath(t *testing.T) {
	data := &StructureData{SizeX: 1, SizeY: 1, SizeZ: 1}
	setCached("path/a.mcstructure", data)

	got := getCached("path/a.mcstructure")
	if got == nil {
		t.Fatal("expected cache hit for same path")
	}
	if got != data {
		t.Fatal("cache returned wrong data")
	}
}

// TestCacheMissDifferentPath verifies that a different path misses the cache.
func TestCacheMissDifferentPath(t *testing.T) {
	setCached("path/a.mcstructure", &StructureData{SizeX: 1, SizeY: 1, SizeZ: 1})

	got := getCached("path/b.mcstructure")
	if got != nil {
		t.Fatal("expected cache miss for different path")
	}
}

// TestCacheOverwrite verifies that parsing a new file replaces the cached entry.
func TestCacheOverwrite(t *testing.T) {
	setCached("path/a.mcstructure", &StructureData{SizeX: 1, SizeY: 1, SizeZ: 1})
	setCached("path/b.mcstructure", &StructureData{SizeX: 2, SizeY: 2, SizeZ: 2})

	// Old path should miss
	if got := getCached("path/a.mcstructure"); got != nil {
		t.Fatal("expected cache miss for overwritten path")
	}
	// New path should hit
	if got := getCached("path/b.mcstructure"); got == nil {
		t.Fatal("expected cache hit for new path")
	}
}

// TestClearCache verifies that clearing the cache causes a miss.
func TestClearCache(t *testing.T) {
	setCached("path/a.mcstructure", &StructureData{SizeX: 1, SizeY: 1, SizeZ: 1})
	ClearCache()

	got := getCached("path/a.mcstructure")
	if got != nil {
		t.Fatal("expected cache miss after ClearCache")
	}
}