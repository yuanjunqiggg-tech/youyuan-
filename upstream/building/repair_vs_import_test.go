package building

import (
	"testing"
	"time"
)

// TestPaletteIndex0IsAir 验证 getOrCreatePaletteIndex 确保索引 0 是空气，
// 防止非空气方块占用索引 0 被放置代码误判为空气而跳过。
func TestPaletteIndex0IsAir(t *testing.T) {
	data := &StructureData{SizeX: 16, SizeY: 16, SizeZ: 16}
	data.Blocks = make(map[uint64]uint32)

	// 第一个被解析的是非空气方块（模拟真实解析中最先出现石头的情况）
	idx := data.getOrCreatePaletteIndex("minecraft:stone", "[]")
	if idx == 0 {
		t.Fatal("非空气方块不应占用索引 0")
	}
	if data.Palette[0].Name != "minecraft:air" {
		t.Fatalf("索引 0 应为空气，实际为 %s", data.Palette[0].Name)
	}
}

// TestFillRegionVsLineByLine 验证正常导入（FillRegion 优化）与修补模式
// （逐方块放置）对含石头连续区域 + 草方块孤立点的结构，放置覆盖一致。
func TestFillRegionVsLineByLine(t *testing.T) {
	size := 16
	data := &StructureData{SizeX: size, SizeY: size, SizeZ: size}
	data.Blocks = make(map[uint64]uint32)
	// 石头连续区域 5-11 x 5-11 x 0-2（3 层）
	for y := 0; y < 3; y++ {
		for x := 5; x < 12; x++ {
			for z := 5; z < 12; z++ {
				data.Set(x, y, z, &BlockInfo{Name: "minecraft:stone", States: "[]"})
			}
		}
	}
	// 草方块孤立点
	data.Set(2, 0, 2, &BlockInfo{Name: "minecraft:grass_block", States: `["snowy"=false]`})
	data.Set(13, 0, 2, &BlockInfo{Name: "minecraft:grass_block", States: `["snowy"=false]`})

	runMode := func(useFill bool) map[[3]int]string {
		placed := make(map[[3]int]string)
		record := func(x, y, z int32, name string) {
			placed[[3]int{int(x), int(y), int(z)}] = name
		}
		task := &ImportTask{
			Data:       data,
			Speed:      100000,
			chunkSolid: map[[2]int]int{{0, 0}: 1},
			SetBlock: func(x, y, z int32, name, states string) error {
				record(x, y, z, name)
				return nil
			},
			StopCh: make(chan struct{}),
		}
		if useFill {
			task.FillRegion = func(x1, y1, z1, x2, y2, z2 int32, name, states string) error {
				for x := x1; x <= x2; x++ {
					for y := y1; y <= y2; y++ {
						for z := z1; z <= z2; z++ {
							record(x, y, z, name)
						}
					}
				}
				return nil
			}
		}
		task.placeChunk(0, 0, size, size, time.Nanosecond)
		return placed
	}

	withFill := runMode(true)    // 正常导入
	lineByLine := runMode(false) // 修补模式

	if len(withFill) != len(lineByLine) {
		t.Errorf("正常导入(FillRegion)放置 %d 个，修补模式(逐方块)放置 %d 个，应一致",
			len(withFill), len(lineByLine))
	}
	for pos, name := range lineByLine {
		if _, ok := withFill[pos]; !ok {
			t.Errorf("正常导入缺失方块 (%d,%d,%d) %s", pos[0], pos[1], pos[2], name)
		}
	}
}
