package building

import "testing"

func TestTickingAreaWorldAlignmentLimit(t *testing.T) {
	for _, c := range []struct {
		x1, z1, x2, z2 int
		valid          bool
	}{
		{0, 0, 159, 159, true}, {-160, -160, -1, -1, true},
		{1, 1, 160, 160, false}, {-1, -1, 158, 158, false},
		{1, 1, 144, 144, true}, {159, 159, 0, 0, true}, {0, 0, 1599, 15, true}, {0, 0, 1615, 15, false},
	} {
		if err := ValidateTickingArea(c.x1, c.z1, c.x2, c.z2); (err == nil) != c.valid {
			t.Fatalf("%+v: %v", c, err)
		}
	}
}

func TestExplicitImportRegionAndResume(t *testing.T) {
	for _, c := range []struct {
		size, x, z     int
		preload, valid bool
	}{
		{0, 1, 1, false, true}, {5, 1, 1, false, true}, {10, 0, 0, true, true},
		{10, -160, -160, true, true}, {10, 1, 0, true, false}, {9, -1, -1, true, true},
		{10, 0, 0, false, false}, {11, 0, 0, true, false}, {-1, 0, 0, true, false},
	} {
		if err := ValidateImportRegion(c.size, c.x, c.z, c.preload); (err == nil) != c.valid {
			t.Fatalf("%+v: %v", c, err)
		}
	}
	if (&ImportTask{RegionMode: 3}).regionChunks() != 4 || (&ImportTask{RegionMode: 3, RegionSizeChunks: 10}).regionChunks() != 10 {
		t.Fatal("legacy or explicit region changed")
	}
	x, z, e := ImportResumeStart(15, 15, 320, 320, 1, 0, 10)
	if e != nil || x != 10 || z != 10 {
		t.Fatal(x, z, e)
	}
}

func TestTenChunkBatchCreatesExactWorldArea(t *testing.T) {
	var bounds [4]int
	task := &ImportTask{Data: importFixture(160, 160), RegionSizeChunks: 10, Speed: -1, SetBlock: func(_, _, _ int32, _, _ string) error { return nil },
		TickingPreload: func(_ string, x1, z1, x2, z2 int) error { bounds = [4]int{x1, z1, x2, z2}; return nil }, TickingCleanup: func() {}}
	if err := task.Run(); err != nil {
		t.Fatal(err)
	}
	if bounds != [4]int{0, 0, 159, 159} {
		t.Fatal(bounds)
	}
}

func TestLargePreloadWindowReusesAreaAcrossSmallBatches(t *testing.T) {
	for _, window := range []int{0, 10} {
		adds, releases, cleaned, placed := 0, 0, 0, 0
		task := &ImportTask{Data: importFixture(160, 160), RegionSizeChunks: 5, PreloadSizeChunks: window, Speed: 0,
			SetBlock: func(_, _, _ int32, _, _ string) error { placed++; return nil },
			FillRegion: func(x1, y1, z1, x2, y2, z2 int32, _, _ string) error {
				placed += int((x2 - x1 + 1) * (y2 - y1 + 1) * (z2 - z1 + 1))
				return nil
			},
			TickingPreload: func(_ string, x1, z1, x2, z2 int) error { adds++; return ValidateTickingArea(x1, z1, x2, z2) },
			TickingRelease: func(string) error { releases++; return nil }, TickingCleanup: func() { cleaned++ }}
		if err := task.Run(); err != nil {
			t.Fatal(err)
		}
		want := 4
		if window == 10 {
			want = 1
		}
		if adds != want || releases != want-1 || cleaned != 1 || placed != 25600 {
			t.Fatal(window, adds, releases, cleaned, placed)
		}
	}
	for _, size := range []int{-1, 3, 7, 11} {
		if ValidateImportPreload(size, 5, 0, 0, true) == nil {
			t.Fatal("accepted invalid preload", size)
		}
	}
}
