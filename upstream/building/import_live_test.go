package building

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestImportMovesOneAreaMatchingBatchAndReportsBeforeCommit(t *testing.T) {
	for _, c := range []struct{ mode, width int }{{1, 16}, {3, 64}, {4, 80}} {
		t.Run(fmt.Sprint(c.width), func(t *testing.T) {
			events := []string{}
			liveBlocks, liveChunks, commits := 0, 0, 0
			task := &ImportTask{Data: importFixture(c.width+16, 16), Speed: -1, RegionMode: c.mode, BaseX: 320, BaseZ: -160, StopOnError: true,
				TickingPreload: func(n string, x1, z1, x2, z2 int) error {
					events = append(events, fmt.Sprintf("add:%d:%d:%d", x1, x2, z2-z1+1))
					return nil
				},
				TickingRelease: func(n string) error { events = append(events, "remove"); return nil }, TickingCleanup: func() { events = append(events, "cleanup") },
				Teleport:    func(x, y, z int) error { events = append(events, "tp"); return nil },
				ChunkLoaded: func(int, int) bool { t.Fatal("client load gate ran"); return false },
				SetBlock:    func(_, _, _ int32, _, _ string) error { return nil },
				FillRegion:  func(_, _, _, _, _, _ int32, _, _ string) error { return nil },
				OnSubmitted: func(b, ch int) {
					if b < liveBlocks || ch < liveChunks {
						t.Fatal("live counters regressed")
					}
					liveBlocks, liveChunks = b, ch
				},
				OnProgress: func(_, _, _, _, b int) {
					if liveBlocks == 0 {
						t.Fatal("progress arrived only after chunk commit")
					}
					commits++
				}}
			if err := task.Run(); err != nil {
				t.Fatal(err)
			}
			want := []string{fmt.Sprintf("add:320:%d:16", 319+c.width), "tp", "remove", fmt.Sprintf("add:%d:%d:16", 320+c.width, 335+c.width), "tp", "cleanup"}
			if !reflect.DeepEqual(events, want) {
				t.Fatal(events, want)
			}
			if liveBlocks != (c.width+16)*16 || liveChunks != c.width/16+1 || commits != liveChunks {
				t.Fatal(liveBlocks, liveChunks, commits)
			}
		})
	}
}

func TestImportAreaFailureStopsBeforeWritesAndStillCleans(t *testing.T) {
	failure := errors.New("server rejected area")
	writes, cleaned := 0, false
	task := &ImportTask{Data: importFixture(1, 1), Speed: -1, TickingPreload: func(string, int, int, int, int) error { return failure }, TickingCleanup: func() { cleaned = true }, SetBlock: func(_, _, _ int32, _, _ string) error { writes++; return nil }}
	if err := task.Run(); !errors.Is(err, failure) || writes != 0 || !cleaned {
		t.Fatal(err, writes, cleaned)
	}
}

func TestImportRejectedWriteNotCountedAsSubmitted(t *testing.T) {
	failure := errors.New("write refused")
	blocks := 0
	task := &ImportTask{Data: importFixture(1, 1), Speed: -1, StopOnError: true, OnSubmitted: func(b, _ int) { blocks = b }, SetBlock: func(_, _, _ int32, _, _ string) error { return failure }}
	if err := task.Run(); !errors.Is(err, failure) || blocks != 0 {
		t.Fatal(err, blocks)
	}
}

func TestNonzeroAirPaletteNeverWritesOrInflatesBlocks(t *testing.T) {
	for _, mode := range []int{1, 3, 4} {
		data := importFixture(80, 16)
		data.Palette[1].Name = "minecraft:air"
		data.Palette[1].IsAir = true
		writes, reported, live := 0, 0, 0
		task := &ImportTask{Data: data, Speed: -1, RegionMode: mode,
			SetBlock:   func(_, _, _ int32, _, _ string) error { writes++; return nil },
			FillRegion: func(_, _, _, _, _, _ int32, _, _ string) error { writes++; return nil },
			OnProgress: func(_, _, _, _, b int) { reported = b }, OnSubmitted: func(b, _ int) { live = b }}
		if err := task.Run(); err != nil || writes != 0 || reported != 0 || live != 0 {
			t.Fatal(mode, err, writes, reported, live)
		}
	}
}

func TestResumedImportCountersIncludeCompletedPrefix(t *testing.T) {
	for _, mode := range []int{1, 3, 4} {
		width := (&ImportTask{RegionMode: mode}).regionChunks() * ChunkSize
		lastBlocks, lastChunks, committed := 0, 0, 0
		task := &ImportTask{Data: importFixture(width+16, 16), Speed: -1, RegionMode: mode, StartCX: width / 16,
			SetBlock:    func(_, _, _ int32, _, _ string) error { return nil },
			FillRegion:  func(_, _, _, _, _, _ int32, _, _ string) error { return nil },
			OnSubmitted: func(b, c int) { lastBlocks, lastChunks = b, c },
			OnProgress:  func(_, _, _, _, b int) { committed = b }}
		want := (width + 16) * 16
		if err := task.Run(); err != nil || lastBlocks != want || committed != want || lastChunks != width/16+1 {
			t.Fatal(mode, err, lastBlocks, committed, lastChunks, want)
		}
	}
}

func TestImportPassCountersNeverDoubleCountEarlierPasses(t *testing.T) {
	for _, useFill := range []bool{false, true} {
		data := &StructureData{SizeX: 3, SizeY: 1, SizeZ: 1, Blocks: map[uint64]uint32{}}
		data.Set(0, 0, 0, &BlockInfo{Name: "minecraft:stone", States: "[]"})
		data.Set(1, 0, 0, &BlockInfo{Name: "minecraft:sand", States: "[]", IsGravity: true})
		data.Set(2, 0, 0, &BlockInfo{Name: "minecraft:torch", States: "[]", IsDependent: true})
		writes, reported := 0, 0
		task := &ImportTask{Data: data, Speed: -1,
			SetBlock:   func(_, _, _ int32, _, _ string) error { writes++; return nil },
			OnProgress: func(_, _, _, _, b int) { reported = b }}
		if useFill {
			task.FillRegion = func(_, _, _, _, _, _ int32, _, _ string) error { t.Fatal("unexpected fill"); return nil }
		}
		if err := task.Run(); err != nil || writes != 3 || reported != 3 {
			t.Fatal(useFill, err, writes, reported)
		}
	}
}
