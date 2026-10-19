package main

import (
	"fmt"
	"testing"
)

// A full block cache must not erase every container record at once. The old
// overflow path replaced the whole map with an empty one, so a busy area lost
// every chest it had ever reported and world.containerState silently flipped
// from known to unknown.
func TestBlockCacheOverflowKeepsRecentContainers(t *testing.T) {
	blocks := map[string]map[string]any{}
	const total = 4096
	for i := 0; i < total; i++ {
		blocks[fmt.Sprintf("0:0:%d:0:0", i)] = map[string]any{
			"timestamp":       int64(1000 + i),
			"container_known": true,
		}
	}
	dropped := yyBlockCacheTrim(blocks)
	if dropped != total/2 {
		t.Fatalf("expected half the cache to be evicted, got %d", dropped)
	}
	if len(blocks) != total/2 {
		t.Fatalf("expected %d records retained, got %d", total/2, len(blocks))
	}
	// The retained half must be the newer one, not an arbitrary selection.
	for i := 0; i < total/2; i++ {
		if _, ok := blocks[fmt.Sprintf("0:0:%d:0:0", i)]; ok {
			t.Fatalf("oldest record %d should have been evicted", i)
		}
	}
	if _, ok := blocks[fmt.Sprintf("0:0:%d:0:0", total-1)]; !ok {
		t.Fatal("newest record must survive trimming")
	}
}

// A cache below the cap must never be touched, so ordinary sessions keep every
// container they saw.
func TestBlockCacheTrimLeavesSmallCachesAlone(t *testing.T) {
	blocks := map[string]map[string]any{
		"0:1:2:3:0": {"timestamp": int64(1)},
		"0:4:5:6:0": {"timestamp": int64(2)},
	}
	if dropped := yyBlockCacheTrim(blocks); dropped != 0 || len(blocks) != 2 {
		t.Fatalf("small cache must be untouched, dropped=%d left=%d", dropped, len(blocks))
	}
	if dropped := yyBlockCacheTrim(map[string]map[string]any{}); dropped != 0 {
		t.Fatal("empty cache must be untouched")
	}
}
