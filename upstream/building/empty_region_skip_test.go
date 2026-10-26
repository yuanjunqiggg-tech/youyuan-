package building

import (
	"strconv"
	"testing"
)

// TestImportSkipsEmptyRegion 验证「导入跳过无实际内容的空白区域」。
//
// 构造 32×16×16 的结构，RegionSizeChunks=1 时被切成两个 16×16 区块列：
//   - 区域 (0,0)：x/z 各 4 格、3 层石头，共 48 个实心方块
//   - 区域 (1,0)：整列纯空气
//
// 期望：空白区域 (1,0) 被完全跳过——不 TP 过去、不分配 visited、不跑放置循环。
// 改动前这里会 TP 一次并跑完整轮 setblock 循环（每格都 continue），纯浪费。
func TestImportSkipsEmptyRegion(t *testing.T) {
	data := &StructureData{SizeX: 32, SizeY: 16, SizeZ: 16}
	data.Blocks = make(map[uint64]uint32)
	const wantBlocks = 4 * 4 * 3 // x 4 格 × z 4 格 × y 3 层
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			for z := 0; z < 4; z++ {
				data.Set(x, y, z, &BlockInfo{Name: "minecraft:stone", States: "[]"})
			}
		}
	}

	var tpCount, setCount int
	var tpHits []string

	task := &ImportTask{
		Data:             data,
		Speed:            100000,
		SkipPreClear:     true, // 不预清空，避免 FillRegion 干扰计数
		RegionSizeChunks: 1,    // 一个区域 = 一个 16×16 区块列
		StopCh:           make(chan struct{}),
		SetBlock: func(x, y, z int32, name, states string) error {
			setCount++
			return nil
		},
		Teleport: func(x, y, z int) error {
			tpCount++
			tpHits = append(tpHits, "("+strconv.Itoa(x)+","+strconv.Itoa(y)+","+strconv.Itoa(z)+")")
			return nil
		},
	}

	if err := task.Run(); err != nil {
		t.Fatalf("Run 失败: %v", err)
	}

	if setCount != wantBlocks {
		t.Fatalf("放置方块数应为 %d，实际 %d", wantBlocks, setCount)
	}
	// 两个区块列，只有一个有方块 → 只该 TP 一次
	if tpCount != 1 {
		t.Fatalf("空白区域应被跳过，TP 次数应为 1，实际 %d 次，坐标 %v", tpCount, tpHits)
	}
}
