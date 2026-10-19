package building

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func importFixture(sx, sz int) *StructureData {
	d := &StructureData{SizeX: sx, SizeY: 1, SizeZ: sz, Blocks: map[uint64]uint32{}}
	for x := 0; x < sx; x++ {
		for z := 0; z < sz; z++ {
			d.Set(x, 0, z, &BlockInfo{Name: "minecraft:stone", States: "[]"})
		}
	}
	return d
}

func TestImportFinalFailedPlacementCannotCommitChunk(t *testing.T) {
	for _, mode := range []int{1, 2, 3} {
		for _, useFill := range []bool{false, true} {
			t.Run(string(rune('0'+mode))+map[bool]string{true: "-fill", false: "-set"}[useFill], func(t *testing.T) {
				failure := errors.New("fixture write refused")
				commits := 0
				task := &ImportTask{Data: importFixture(3, 1), Speed: -1, StopOnError: true, RegionMode: mode,
					SetBlock:   func(_, _, _ int32, _, _ string) error { return failure },
					OnProgress: func(_, _, _, _, _ int) { commits++ }}
				if useFill {
					task.FillRegion = func(_, _, _, _, _, _ int32, _, _ string) error { return failure }
				}
				if err := task.Run(); !errors.Is(err, failure) || commits != 0 {
					t.Fatalf("failed placement committed: err=%v commits=%d", err, commits)
				}
			})
		}
	}
}

func TestImportFinalCancellationCannotCommitChunk(t *testing.T) {
	stop := make(chan struct{})
	commits := 0
	task := &ImportTask{Data: importFixture(1, 1), Speed: -1, StopOnError: true, StopCh: stop,
		SetBlock:   func(_, _, _ int32, _, _ string) error { close(stop); return nil },
		OnProgress: func(_, _, _, _, _ int) { commits++ }}
	if err := task.Run(); err == nil || commits != 0 {
		t.Fatalf("cancelled final chunk committed: err=%v commits=%d", err, commits)
	}
}

func TestImportSnakeResumeIncludesRemainingOddRowChunks(t *testing.T) {
	seen := [][2]int{}
	progress := []int{}
	task := &ImportTask{Data: importFixture(32, 48), Speed: -1, StartCX: 1, StartCZ: 2,
		SetBlock:   func(_, _, _ int32, _, _ string) error { return nil },
		OnProgress: func(cx, cz, n, _, _ int) { seen = append(seen, [2]int{cx, cz}); progress = append(progress, n) }}
	if err := task.Run(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, [][2]int{{1, 2}, {1, 1}, {1, 0}}) || !reflect.DeepEqual(progress, []int{4, 5, 6}) {
		t.Fatal("odd row resume skipped work or regressed progress", seen, progress)
	}
}

func TestImportChecksNegativeWorldChunkBoundaries(t *testing.T) {
	checked := map[[2]int]bool{}
	task := &ImportTask{Data: importFixture(16, 16), BaseX: -1, BaseZ: -1, Speed: -1,
		ChunkLoaded: func(cx, cz int) bool { checked[[2]int{cx, cz}] = true; return true },
		SetBlock:    func(_, _, _ int32, _, _ string) error { return nil }}
	if err := task.Run(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(checked, map[[2]int]bool{{-1, -1}: true, {-1, 0}: true, {0, -1}: true, {0, 0}: true}) {
		t.Fatal("world chunk boundary was not checked", checked)
	}
}

func TestImportUnitTransportFailureCannotComplete(t *testing.T) {
	failure := errors.New("fixture disconnected")
	task := &ImportTask{Data: importFixture(1, 1), Speed: -1,
		SetBlock: func(_, _, _ int32, _, _ string) error { return failure }}
	if err := task.RunUnit(&Unit{ID: 1, Kind: UnitNormal, Chunks: [][2]int{{0, 0}}}); !errors.Is(err, failure) {
		t.Fatal("failed unit reported successful", err)
	}
}

func TestImportChunkLoadTimeoutCannotPlaceOrCommit(t *testing.T) {
	placed, committed := 0, 0
	task := &ImportTask{Data: importFixture(1, 1), Speed: -1,
		ChunkLoaded: func(_, _ int) bool { return false },
		SetBlock:    func(_, _, _ int32, _, _ string) error { placed++; return nil },
		OnProgress:  func(_, _, _, _, _ int) { committed++ }}
	if err := task.Run(); err == nil || !strings.Contains(err.Error(), "区块加载超时") || placed != 0 || committed != 0 {
		t.Fatalf("unloaded chunk was accepted: err=%v placed=%d commits=%d", err, placed, committed)
	}
}

func TestImportChunkLoadRetryDoesNotRecheckConfirmedColumns(t *testing.T) {
	calls := map[[2]int]int{}
	task := &ImportTask{ChunkLoaded: func(x, z int) bool {
		key := [2]int{x, z}
		calls[key]++
		return x == 0 || calls[key] > 1
	}}
	if err := task.waitChunksLoaded(0, 0, 31, 15); err != nil {
		t.Fatal(err)
	}
	if calls[[2]int{0, 0}] != 1 || calls[[2]int{1, 0}] != 2 {
		t.Fatal("already confirmed columns were re-probed", calls)
	}
}

func TestImportResumeRewindsActualTraversalForChunksAndRegions(t *testing.T) {
	for _, tc := range []struct{ cx, cz, sx, sz, mode, x, z int }{
		{1, 0, 32, 48, 1, 1, 2},
		{1, 2, 32, 48, 1, 0, 1},
		{2, 0, 64, 64, 2, 0, 2},
		{0, 0, 16, 16, 1, 0, 0},
	} {
		x, z, err := ImportResumeStart(tc.cx, tc.cz, tc.sx, tc.sz, tc.mode, 2)
		if err != nil || x != tc.x || z != tc.z {
			t.Fatal(tc, x, z, err)
		}
	}
	if _, _, err := ImportResumeStart(9, 0, 16, 16, 1, 2); err == nil {
		t.Fatal("out-of-range checkpoint accepted")
	}
}

func TestImport34ChunksLastTwoFailedRemainResumable(t *testing.T) {
	data := &StructureData{SizeX: 1, SizeY: 1, SizeZ: 34 * 16, Blocks: map[uint64]uint32{}}
	for z := 0; z < 34; z++ {
		data.Set(0, 0, z*16, &BlockInfo{Name: "minecraft:stone", States: "[]"})
	}
	world := map[int32]bool{}
	lastX, lastZ, progress := 0, 0, 0
	task := &ImportTask{Data: data, Speed: -1, StopOnError: true,
		SetBlock: func(_, _ int32, z int32, _, _ string) error {
			if z >= 32*16 {
				return errors.New("fixture last two chunks unavailable")
			}
			world[z] = true
			return nil
		},
		OnProgress: func(cx, cz, n, _, _ int) { lastX, lastZ, progress = cx, cz, n }}
	if err := task.Run(); err == nil || progress != 32 || lastZ != 31 || len(world) != 32 {
		t.Fatal("failed tail became non-resumable", err, progress, lastZ, len(world))
	}
	x, z, err := ImportResumeStart(lastX, lastZ, data.SizeX, data.SizeZ, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	task = &ImportTask{Data: data, Speed: -1, StopOnError: true, StartCX: x, StartCZ: z,
		SetBlock:   func(_, _ int32, z int32, _, _ string) error { world[z] = true; return nil },
		OnProgress: func(_, _, n, _, _ int) { progress = n }}
	if err := task.Run(); err != nil || progress != 34 || len(world) != 34 {
		t.Fatal("resume left gaps", err, progress, len(world))
	}
}

func TestImportPreClearTeleportFailureCannotClearOrCommit(t *testing.T) {
	for _, mode := range []int{1, 2, 3} {
		t.Run(string(rune('0'+mode)), func(t *testing.T) {
			failure := errors.New("fixture teleport refused")
			clears, placements, commits, clearDone := 0, 0, 0, 0
			task := &ImportTask{Data: importFixture(1, 1), Speed: -1, PreClearMode: mode,
				Teleport:    func(_, _, _ int) error { return failure },
				FillRegion:  func(_, _, _, _, _, _ int32, _, _ string) error { clears++; return nil },
				SetBlock:    func(_, _, _ int32, _, _ string) error { placements++; return nil },
				OnProgress:  func(_, _, _, _, _ int) { commits++ },
				OnClearDone: func() { clearDone++ }}
			if err := task.Run(); !errors.Is(err, failure) || clears != 0 || placements != 0 || commits != 0 || clearDone != 0 {
				t.Fatalf("failed pre-clear teleport continued work: err=%v clears=%d placements=%d commits=%d clearDone=%d", err, clears, placements, commits, clearDone)
			}
		})
	}
}

func TestImportPreClearWriteFailureStopsBeforeNextTeleport(t *testing.T) {
	for _, mode := range []int{2, 3} {
		t.Run(string(rune('0'+mode)), func(t *testing.T) {
			failure := errors.New("fixture clear refused")
			teleports, clears, clearDone := 0, 0, 0
			task := &ImportTask{Data: importFixture(1, 34*16), Speed: -1, PreClearMode: mode, StopOnError: true,
				Teleport:    func(_, _, _ int) error { teleports++; return nil },
				FillRegion:  func(_, _, _, _, _, _ int32, _, _ string) error { clears++; return failure },
				OnClearDone: func() { clearDone++ }}
			if err := task.Run(); !errors.Is(err, failure) || teleports != 1 || clears != 1 || clearDone != 0 {
				t.Fatalf("failed clear continued across map: err=%v teleports=%d clears=%d clearDone=%d", err, teleports, clears, clearDone)
			}
		})
	}
}

func TestImportPreClearCancellationDoesNotTeleport(t *testing.T) {
	for _, mode := range []int{1, 2, 3} {
		t.Run(string(rune('0'+mode)), func(t *testing.T) {
			stop := make(chan struct{})
			close(stop)
			teleports := 0
			task := &ImportTask{Data: importFixture(1, 1), Speed: -1, PreClearMode: mode, StopCh: stop,
				Teleport:   func(_, _, _ int) error { teleports++; return nil },
				FillRegion: func(_, _, _, _, _, _ int32, _, _ string) error { return nil }}
			if err := task.Run(); err == nil || teleports != 0 {
				t.Fatalf("cancelled pre-clear teleported: err=%v teleports=%d", err, teleports)
			}
		})
	}
}
