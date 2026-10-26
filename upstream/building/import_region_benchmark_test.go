package building

import (
	"fmt"
	"testing"
)

// Measures batching geometry only; no game server, network or NBT is exercised.
func regionBenchmarkFixture() *StructureData {
	d := &StructureData{SizeX: 160, SizeY: 32, SizeZ: 160, Blocks: make(map[uint64]uint32)}
	for x := 0; x < d.SizeX; x++ {
		for y := 0; y < d.SizeY; y++ {
			for z := 0; z < d.SizeZ; z++ {
				d.Set(x, y, z, &BlockInfo{Name: "minecraft:stone", States: "[]"})
			}
		}
	}
	return d
}

func BenchmarkImportRegionGeometry(b *testing.B) {
	d := regionBenchmarkFixture()
	for _, side := range []int{1, 3, 5, 7, 9, 10} {
		b.Run(fmt.Sprintf("side_%d", side), func(b *testing.B) {
			commands := 0
			blocks := 0
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				task := &ImportTask{Data: d, RegionSizeChunks: side, Speed: 0, StopOnError: true,
					SetBlock: func(_, _, _ int32, _, _ string) error { commands++; blocks++; return nil },
					FillRegion: func(x1, y1, z1, x2, y2, z2 int32, _, _ string) error {
						commands++
						blocks += int((x2 - x1 + 1) * (y2 - y1 + 1) * (z2 - z1 + 1))
						return nil
					},
					TickingPreload: func(_ string, x1, z1, x2, z2 int) error { return ValidateTickingArea(x1, z1, x2, z2) },
					TickingRelease: func(string) error { return nil }, TickingCleanup: func() {}}
				if err := task.Run(); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			if blocks != b.N*160*32*160 {
				b.Fatal("batching lost blocks", blocks)
			}
			b.ReportMetric(float64(commands)/float64(b.N), "commands/op")
			b.ReportMetric(float64(blocks)/float64(b.N), "blocks/op")
		})
	}
}

func BenchmarkImportSingleUnitLargeSource(b *testing.B) {
	d := regionBenchmarkFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		placed := 0
		task := &ImportTask{Data: d, Speed: 0, SetBlock: func(_, _, _ int32, _, _ string) error { placed++; return nil },
			FillRegion: func(x1, y1, z1, x2, y2, z2 int32, _, _ string) error {
				placed += int((x2 - x1 + 1) * (y2 - y1 + 1) * (z2 - z1 + 1))
				return nil
			}}
		if err := task.RunUnit(&Unit{Kind: UnitNormal, Chunks: [][2]int{{4, 4}}}); err != nil {
			b.Fatal(err)
		}
		if placed != 16*16*32 {
			b.Fatal("unit changed coverage", placed)
		}
	}
}

func BenchmarkFreezePaletteDense(b *testing.B) {
	d := regionBenchmarkFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.FreezePalette()
	}
}
