package building

import (
	"fmt"
	"os"
	"sync"
)

// cacheEntry holds a single cached StructureData — the most recently parsed
// building file. The cache is discarded on import completion or when a new
// building file is parsed (single-entry; public loads include file identity and region).
type cacheEntry struct {
	path string
	data *StructureData
}

var (
	cacheMu sync.Mutex
	cache   *cacheEntry
)

// getCached returns the cached StructureData for path if it matches the
// currently cached entry, nil otherwise.
func getCached(path string) *StructureData {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cache != nil && cache.path == path {
		return cache.data
	}
	return nil
}

// setCached stores a parsed StructureData for the given path, replacing any
// previous cache entry.
func setCached(path string, data *StructureData) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache = &cacheEntry{path: path, data: data}
}

// ClearCache discards the cached structure data, if any.
func ClearCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache = nil
}

// IsCached reports whether the given path matches the current cache entry.
func IsCached(path string) bool {
	key, err := structureCacheKey(path, nil)
	if err == nil {
		path = key
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	return cache != nil && cache.path == path
}

// InvalidateCache removes a specific path from cache.
// With the single-entry cache this is equivalent to ClearCache.
func InvalidateCache(path string) {
	ClearCache()
}

// LoadStructureFileCached wraps LoadStructureFile with a single-entry cache.
// Cached data is discarded when a new building file is parsed or when
// ClearCache is called (e.g. after import completes).
// Region parameters and file metadata are part of the cache identity.
func LoadStructureFileCached(path string, region ...[6]int) (*StructureData, error) {
	key, err := structureCacheKey(path, region)
	if err != nil {
		return nil, err
	}
	if cached := getCached(key); cached != nil {
		return cached, nil
	}
	data, err := LoadStructureFile(path, region...)
	if err != nil {
		return nil, err
	}

	setCached(key, data)
	return data, nil
}

func structureCacheKey(path string, region [][6]int) (string, error) {
	info, e := os.Stat(path)
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("%s|%d|%d|%v", path, info.Size(), info.ModTime().UnixNano(), region), nil
}

// An import rotates and freezes its data in place. Transfer cached ownership
// instead of retaining an altered preview as the source for the next run.
func TakeStructureFileCached(path string, region ...[6]int) (*StructureData, error) {
	key, e := structureCacheKey(path, region)
	if e != nil {
		return nil, e
	}
	cacheMu.Lock()
	if cache != nil && cache.path == key {
		data := cache.data
		cache = nil
		cacheMu.Unlock()
		return data, nil
	}
	cacheMu.Unlock()
	return LoadStructureFile(path, region...)
}
