package building

import (
	"errors"
	"testing"
)

func TestNBTFailureDoesNotAdvanceRegionOrReportComplete(t *testing.T) {
	for _, mode := range []int{1, 2, 4} {
		t.Run(string(rune('0'+mode)), func(t *testing.T) {
			d := &StructureData{SizeX: 2, SizeY: 1, SizeZ: 1, Blocks: map[uint64]uint32{}}
			for x := 0; x < 2; x++ {
				d.Set(x, 0, 0, &BlockInfo{Name: "minecraft:barrel", States: "[]", NBT: map[string]any{"marker": x}})
			}
			fail := errors.New("structure load not confirmed")
			called, progress := 0, 0
			task := &ImportTask{Data: d, RegionMode: mode, Speed: 0, SkipPreClear: true, ImportCommands: true, StopCh: make(chan struct{}), SetBlock: func(int32, int32, int32, string, string) error { return nil }, OnNBTChecked: func(int32, int32, int32, map[string]any, string, string) error { called++; return fail }, OnProgress: func(int, int, int, int, int) { progress++ }}
			if err := task.Run(); !errors.Is(err, fail) {
				t.Fatalf("NBT failure swallowed: %v", err)
			}
			if called != 1 || progress != 0 {
				t.Fatalf("advanced failed region: NBT=%d progress=%d", called, progress)
			}
		})
	}
}
