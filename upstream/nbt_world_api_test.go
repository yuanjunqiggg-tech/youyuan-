package main

import (
	"fmt"
	"testing"
)

// 构造一块 mock 的 RequestStructure 返回结构数据。
// 结构与 extractCommandBlocks 解析的一致：
//   size / structure.palette.default.block_palette + block_position_data / structure.block_indices
// block_indices 用 z-fastest 线性索引，block_position_data 用线性索引作键。
func mockStructureData(size []any, palette []map[string]any, nbtByIndex map[int]map[string]any, indices []any) map[string]any {
	// block_palette 在解码后是 []any（每项 map），这里用 []any 包住以匹配解析逻辑
	paletteAny := make([]any, len(palette))
	for i, p := range palette {
		paletteAny[i] = p
	}
	blockPosData := make(map[string]any, len(nbtByIndex))
	for i, nbt := range nbtByIndex {
		blockPosData[fmt.Sprint(i)] = map[string]any{"block_entity_data": nbt}
	}
	return map[string]any{
		"size": size,
		"structure": map[string]any{
			"palette": map[string]any{
				"default": map[string]any{
					"block_palette":       paletteAny,
					"block_position_data": blockPosData,
				},
			},
			"block_indices": []any{indices},
		},
	}
}

// 单方块命令方块：名称+状态+NBT（指令）都能正确提取。
func TestParseStructureData_CommandBlock(t *testing.T) {
	data := mockStructureData(
		[]any{1, 1, 1},
		[]map[string]any{
			{"name": "minecraft:air", "states": map[string]any{}},
			{"name": "minecraft:command_block", "states": map[string]any{"facing_direction": int32(3), "conditional_bit": byte(1)}},
		},
		map[int]map[string]any{0: {"Command": "say hi", "CustomName": "cmd"}},
		[]any{1}, // layer0: 线性索引 0 → palette[1] = command_block
	)

	region := parseStructureData(data, 10, 20, 30)
	bd, ok := region["10,20,30"]
	if !ok {
		t.Fatalf("缺坐标 10,20,30，实际: %v", region)
	}
	if bd.Name != "minecraft:command_block" {
		t.Errorf("Name=%q, want command_block", bd.Name)
	}
	if bd.States["facing_direction"] != int32(3) {
		t.Errorf("states.facing_direction=%v, want 3", bd.States["facing_direction"])
	}
	if cmd, ok := bd.NBT["Command"].(string); !ok || cmd != "say hi" {
		t.Errorf("NBT.Command=%v, want 'say hi'", bd.NBT["Command"])
	}
}

// 区域读取：2×1×1 = [空气, 箱子(含物品)]，验证坐标换算与 NBT。
func TestParseStructureData_RegionWithContainer(t *testing.T) {
	// 尺寸 2(x)×1(y)×1(z)，z-fastest：线性索引 i=0 → (0,0,0) 空气，i=1 → (1,0,0) 箱子
	data := mockStructureData(
		[]any{2, 1, 1},
		[]map[string]any{
			{"name": "minecraft:air", "states": map[string]any{}},
			{"name": "minecraft:chest", "states": map[string]any{}},
		},
		map[int]map[string]any{1: {"Items": []any{map[string]any{"Count": byte(1), "Name": "minecraft:diamond"}}}},
		[]any{0, 1}, // 线性索引 0→palette[0]=air，1→palette[1]=chest
	)

	region := parseStructureData(data, 100, 60, 200)
	if len(region) != 2 {
		t.Fatalf("got %d blocks, want 2", len(region))
	}
	airBD := region["100,60,200"]
	if airBD.Name != "minecraft:air" || airBD.NBT != nil {
		t.Errorf("空气块解析错误: %+v", airBD)
	}
	chestBD := region["101,60,200"]
	if chestBD.Name != "minecraft:chest" {
		t.Errorf("chest Name=%q, want minecraft:chest", chestBD.Name)
	}
	if chestBD.NBT == nil {
		t.Fatalf("箱子缺 NBT")
	}
	items, ok := chestBD.NBT["Items"].([]any)
	if !ok || len(items) != 1 {
		t.Errorf("箱子 Items 解析错误: %v", chestBD.NBT["Items"])
	}
}

// 无 NBT 的普通方块：NBT 应为 nil。
func TestParseStructureData_PlainBlock(t *testing.T) {
	data := mockStructureData(
		[]any{1, 1, 1},
		[]map[string]any{
			{"name": "minecraft:stone", "states": map[string]any{}},
		},
		nil,
		[]any{0},
	)
	region := parseStructureData(data, 0, 0, 0)
	bd := region["0,0,0"]
	if bd == nil {
		t.Fatalf("缺 0,0,0")
	}
	if bd.Name != "minecraft:stone" {
		t.Errorf("Name=%q, want stone", bd.Name)
	}
	if bd.NBT != nil {
		t.Errorf("普通方块不应有 NBT: %v", bd.NBT)
	}
}

// buildStatesString：bool/整数/字符串都能正确格式化。
func TestBuildStatesString(t *testing.T) {
	cases := []struct {
		name   string
		states map[string]any
		want   string
	}{
		{"空状态", map[string]any{}, "[]"},
		{"朝向+条件", map[string]any{"facing_direction": int32(3), "conditional_bit": true}, `["conditional_bit"=true,"facing_direction"=3]`},
		{"布尔false", map[string]any{"lit": false}, `["lit"=false]`},
		{"字符串值", map[string]any{"wood_type": "oak"}, `["wood_type"="oak"]`},
		{"float64整数", map[string]any{"facing_direction": float64(5)}, `["facing_direction"=5]`},
	}
	for _, c := range cases {
		got := buildStatesString(c.states)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// normalizeStateValue：各类输入归一化成 string/byte/int32。
func TestNormalizeStateValue(t *testing.T) {
	cases := []struct {
		in   any
		want any
	}{
		{true, byte(1)},
		{false, byte(0)},
		{int(7), int32(7)},
		{int64(9), int32(9)},
		{float64(3.0), int32(3)},
		{"oak", "oak"},
		{int32(4), int32(4)},
		{byte(1), byte(1)},
	}
	for _, c := range cases {
		got := normalizeStateValue(c.in)
		if got != c.want {
			t.Errorf("normalizeStateValue(%v)=%v, want %v", c.in, got, c.want)
		}
	}
}
