package building

import (
	"path/filepath"
	"testing"
)

func TestMCWorldSelectedBoundsAndUnalignedChunkRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "renamed.mcworld")
	indices := make([]int32, 9)
	for i := range indices {
		indices[i] = 0
	}
	merged := map[string]any{"block_indices": []any{indices}, "palette": map[string]any{"default": map[string]any{"block_palette": []any{map[string]any{"name": "minecraft:stone", "states": map[string]any{}}}}}}
	// Both axes cross a chunk border despite a width smaller than 16.
	if e := WriteMergedStructureToMCWorld(merged, 3, 1, 3, p, 15, 64, 15); e != nil {
		t.Fatal(e)
	}
	result, e := ParseMCWorld(p)
	if e != nil {
		t.Fatal(e)
	}
	if result.SizeX != 3 || result.SizeY != 1 || result.SizeZ != 3 || len(result.Blocks) != 9 {
		t.Fatalf("lost bounds or edge blocks: %dx%dx%d blocks=%d", result.SizeX, result.SizeY, result.SizeZ, len(result.Blocks))
	}
}
