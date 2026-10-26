package main

import "testing"

// 构造一个 z-fastest 线性索引（idx = x*sy*sz + y*sz + z）的命令方块结构数据。
// 区域 3×2×4，放置：
//   - repeating_command_block @ (1,1,3) → idx 15
//   - chain_command_block      @ (1,1,2) → idx 14
func buildCBStructureData(sizeAny any, layer0Any any) map[string]any {
	palette := []any{
		map[string]any{"name": "minecraft:air", "states": map[string]any{}},
		map[string]any{
			"name": "minecraft:repeating_command_block",
			"states": map[string]any{"facing_direction": float64(3), "conditional_bit": float64(1)},
		},
		map[string]any{
			"name": "minecraft:chain_command_block",
			"states": map[string]any{"facing_direction": float64(3), "conditional_bit": float64(0)},
		},
	}
	blockPosData := map[string]any{
		"15": map[string]any{"block_entity_data": map[string]any{
			"Command": "say repeat", "auto": float64(1), "TickDelay": float64(0),
			"ExecuteOnFirstTick": float64(0), "TrackOutput": float64(1),
		}},
		"14": map[string]any{"block_entity_data": map[string]any{
			"Command": "say chain", "auto": float64(0), "TickDelay": float64(2),
			"ExecuteOnFirstTick": float64(0), "TrackOutput": float64(1),
		}},
	}
	return map[string]any{
		"size": sizeAny,
		"structure": map[string]any{
			"palette": map[string]any{
				"default": map[string]any{
					"block_palette":      palette,
					"block_position_data": blockPosData,
				},
			},
			"block_indices": []any{layer0Any},
		},
	}
}

func findBlock(blocks []CBSimpleBlock, cmd string) *CBSimpleBlock {
	for i := range blocks {
		if blocks[i].Command == cmd {
			return &blocks[i]
		}
	}
	return nil
}

// 导出识别的核心回归：size 用 [N]int32 数组（flowers-for-machines nbt 包真实解码形态），
// block_indices 用 []int32。修复前 size 断言失败会直接返回空列表 → "未找到命令方块"。
func TestExtractCommandBlocks_ZFastest(t *testing.T) {
	layer0 := make([]int32, 3*2*4) // 24, 默认 air(0)
	layer0[15] = 1                 // repeating @ (1,1,3)
	layer0[14] = 2                 // chain @ (1,1,2)

	data := buildCBStructureData([3]int32{3, 2, 4}, layer0)
	blocks := extractCommandBlocks(data, 0, 0, 0)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 command blocks, got %d", len(blocks))
	}

	rep := findBlock(blocks, "say repeat")
	if rep == nil {
		t.Fatal("repeating command block not found")
	}
	if rep.X != 1 || rep.Y != 1 || rep.Z != 3 {
		t.Errorf("repeating coords: got (%d,%d,%d), want (1,1,3)", rep.X, rep.Y, rep.Z)
	}
	if rep.Type != "repeating" {
		t.Errorf("repeating type: got %q, want repeating", rep.Type)
	}
	if !rep.Conditional {
		t.Error("repeating conditional: got false, want true (conditional_bit=1)")
	}
	if rep.Facing != "z+" {
		t.Errorf("repeating facing: got %q, want z+", rep.Facing)
	}

	ch := findBlock(blocks, "say chain")
	if ch == nil {
		t.Fatal("chain command block not found")
	}
	if ch.X != 1 || ch.Y != 1 || ch.Z != 2 {
		t.Errorf("chain coords: got (%d,%d,%d), want (1,1,2)", ch.X, ch.Y, ch.Z)
	}
	if ch.Type != "chain" {
		t.Errorf("chain type: got %q, want chain", ch.Type)
	}
	if ch.Conditional {
		t.Error("chain conditional: got true, want false")
	}
	if ch.TickDelay != 2 {
		t.Errorf("chain tick_delay: got %d, want 2", ch.TickDelay)
	}
	if ch.Auto {
		t.Error("chain auto: got true, want false (auto=0 → needs redstone)")
	}
}

// block_indices 层用 [N]int32 定长数组形态（另一处 NBT 解码可能），验证 anyToIntSlice 反射兜底。
func TestExtractCommandBlocks_ArrayIndices(t *testing.T) {
	layer0 := [24]int32{} // 默认 air(0)
	layer0[15] = 1
	layer0[14] = 2

	data := buildCBStructureData([]int32{3, 2, 4}, layer0)
	blocks := extractCommandBlocks(data, 0, 0, 0)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 command blocks, got %d", len(blocks))
	}
	rep := findBlock(blocks, "say repeat")
	if rep == nil || rep.X != 1 || rep.Y != 1 || rep.Z != 3 {
		t.Fatalf("repeating not extracted correctly: %+v", rep)
	}
}
