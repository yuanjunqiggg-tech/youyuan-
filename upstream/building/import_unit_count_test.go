package building

import "testing"

func TestDenseUnitCountsOnlyItsFilteredPositions(t *testing.T) {
	d := importFixture(32, 16)
	d.Set(17, 0, 1, &BlockInfo{Name: "minecraft:water", States: "[]"})
	for _, flat := range []bool{false, true} {
		if flat {
			d.BlocksFlat = make([]uint32, 32*16)
			for pos, idx := range d.Blocks {
				x, _, z := unpackPos(pos)
				d.BlocksFlat[int(z)*32+int(x)] = idx
			}
		}
		placed := 0
		task := &ImportTask{Data: d, ExcludeWater: true, Speed: 0, SetBlock: func(x, y, z int32, name, states string) error {
			if x < 16 || x >= 32 || x == 17 && z == 1 || x == 19 && z == 1 || name == "minecraft:water" {
				t.Fatalf("wrote excluded location %d,%d %s", x, z, name)
			}
			placed++
			return nil
		}}
		if err := task.RunUnit(&Unit{Kind: UnitNormal, Chunks: [][2]int{{1, 0}}, Skipped: [][3]int{{19, 0, 1}}}); err != nil {
			t.Fatal(err)
		}
		if task.TotalSolidBlocks != 254 || placed != 254 {
			t.Fatal(flat, task.TotalSolidBlocks, placed)
		}
	}
}
