package building

import (
	"fmt"
	"testing"
)

func TestAdjacentSamePaletteKeepsEachPositionsNBT(t *testing.T) {
	for _, mode := range []int{1, 2, 3} {
		for _, name := range []string{"minecraft:barrel", "minecraft:standing_sign"} {
			t.Run(fmt.Sprintf("mode%d/%s", mode, name), func(t *testing.T) {
				d := &StructureData{SizeX: 4, SizeY: 1, SizeZ: 1, Blocks: map[uint64]uint32{}}
				for x := 0; x < 4; x++ {
					var n map[string]any
					if x == 1 || x == 2 {
						n = map[string]any{"marker": fmt.Sprintf("item%d", x)}
					}
					d.Set(x, 0, 0, &BlockInfo{Name: name, States: "[]", NBT: n})
				}
				got := map[int]string{}
				task := &ImportTask{Data: d, RegionMode: mode, Speed: 0, SkipPreClear: true, ImportCommands: true, StopCh: make(chan struct{}), SetBlock: func(x, y, z int32, n, s string) error { return nil }, FillRegion: func(x1, y1, z1, x2, y2, z2 int32, n, s string) error { return nil }, OnNBT: func(x, y, z int32, n map[string]any, _, _ string) { got[int(x)] = n["marker"].(string) }}
				if e := task.Run(); e != nil {
					t.Fatal(e)
				}
				if len(got) != 2 || got[1] != "item1" || got[2] != "item2" {
					t.Fatalf("neighbour NBT lost or duplicated: %v", got)
				}
			})
		}
	}
}
