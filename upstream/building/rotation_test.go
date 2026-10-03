package building

import (
	"testing"
)

func TestRotateBlockStates(t *testing.T) {
	tests := []struct {
		name     string
		block    string
		states   string
		degrees  int
		expected string
		desc     string
	}{
		// ── 命令方块 facing_direction ──
		{"cmd_north→west_90", "minecraft:command_block", `["facing_direction"=2]`, 90, `["facing_direction"=4]`, "命令方块 北→西 90°"},
		{"cmd_south→east_90", "minecraft:command_block", `["facing_direction"=3]`, 90, `["facing_direction"=5]`, "命令方块 南→东 90°"},
		{"cmd_west→south_90", "minecraft:command_block", `["facing_direction"=4]`, 90, `["facing_direction"=3]`, "命令方块 西→南 90°"},
		{"cmd_east→north_90", "minecraft:command_block", `["facing_direction"=5]`, 90, `["facing_direction"=2]`, "命令方块 东→北 90°"},
		{"cmd_north→south_180", "minecraft:command_block", `["facing_direction"=2]`, 180, `["facing_direction"=3]`, "命令方块 北→南 180°"},
		{"cmd_south→north_180", "minecraft:command_block", `["facing_direction"=3]`, 180, `["facing_direction"=2]`, "命令方块 南→北 180°"},
		{"cmd_west→east_180", "minecraft:command_block", `["facing_direction"=4]`, 180, `["facing_direction"=5]`, "命令方块 西→东 180°"},
		{"cmd_north→east_270", "minecraft:command_block", `["facing_direction"=2]`, 270, `["facing_direction"=5]`, "命令方块 北→东 270°"},
		// 垂直不旋转
		{"cmd_down_unchanged_90", "minecraft:command_block", `["facing_direction"=0]`, 90, `["facing_direction"=0]`, "命令方块 朝下不变 90°"},
		{"cmd_up_unchanged_180", "minecraft:command_block", `["facing_direction"=1]`, 180, `["facing_direction"=1]`, "命令方块 朝上不变 180°"},

		// ── 字符串 4 向 facing ──
		{"facing_north→west_90", "minecraft:furnace", `["facing"="north"]`, 90, `["facing"="west"]`, "熔炉 北→西 90°"},
		{"facing_north→south_180", "minecraft:furnace", `["facing"="north"]`, 180, `["facing"="south"]`, "熔炉 北→南 180°"},
		{"facing_north→east_270", "minecraft:furnace", `["facing"="north"]`, 270, `["facing"="east"]`, "熔炉 北→东 270°"},

		// ── 字符串 6 向 minecraft:facing_direction ──
		{"sixway_north→west_90", "minecraft:observer", `["minecraft:facing_direction"="north"]`, 90, `["minecraft:facing_direction"="west"]`, "观察者 北→西 90°"},
		{"sixway_down_unchanged", "minecraft:observer", `["minecraft:facing_direction"="down"]`, 90, `["minecraft:facing_direction"="down"]`, "观察者 朝下不变"},

		// ── 楼梯 weirdo_direction ──
		{"stairs_east→north_90", "minecraft:oak_stairs", `["weirdo_direction"=0]`, 90, `["weirdo_direction"=3]`, "楼梯 东→北 90°"},
		{"stairs_east→west_180", "minecraft:oak_stairs", `["weirdo_direction"=0]`, 180, `["weirdo_direction"=1]`, "楼梯 东→西 180°"},
		{"stairs_east→south_270", "minecraft:oak_stairs", `["weirdo_direction"=0]`, 270, `["weirdo_direction"=2]`, "楼梯 东→南 270°"},

		// ── 中继器 direction ──
		{"repeater_south→east_90", "minecraft:repeater", `["direction"=0]`, 90, `["direction"=3]`, "中继器 南→东 90°"},
		{"repeater_south→north_180", "minecraft:repeater", `["direction"=0]`, 180, `["direction"=2]`, "中继器 南→北 180°"},

		// ── 轴 axis ──
		{"axis_x→z_90", "minecraft:oak_log", `["axis"="x"]`, 90, `["axis"="z"]`, "原木 x→z 90°"},
		{"axis_x_unchanged_180", "minecraft:oak_log", `["axis"="x"]`, 180, `["axis"="x"]`, "原木 x 不变 180°"},
		{"axis_z→x_270", "minecraft:oak_log", `["axis"="z"]`, 270, `["axis"="x"]`, "原木 z→x 270°"},
		{"axis_y_unchanged", "minecraft:oak_log", `["axis"="y"]`, 90, `["axis"="y"]`, "原木 y 不变"},

		// ── 火把 torch_facing_direction ──
		{"torch_east→north_90", "minecraft:torch", `["torch_facing_direction"=1]`, 90, `["torch_facing_direction"=4]`, "火把 东→北 90°"},
		{"torch_east→west_180", "minecraft:torch", `["torch_facing_direction"=1]`, 180, `["torch_facing_direction"=2]`, "火把 东→西 180°"},
		{"torch_up_unchanged", "minecraft:torch", `["torch_facing_direction"=5]`, 90, `["torch_facing_direction"=5]`, "地上火把不变"},

		// ── 多属性混合 ──
		{"cmd_mixed_180", "minecraft:repeating_command_block",
			`["conditional"=false,"facing_direction"=2]`, 180,
			`["conditional"=false,"facing_direction"=3]`,
			"循环命令方块 conditional+朝向 180°"},
		{"cmd_mixed_keep_conditional", "minecraft:chain_command_block",
			`["conditional"=true,"facing_direction"=5]`, 90,
			`["conditional"=true,"facing_direction"=2]`,
			"链命令方块 conditional 保留，朝向 5→2 90°"},

		// ── 无状态变化 ──
		{"air_unchanged", "minecraft:air", `[]`, 90, `[]`, "空气不变"},
		{"empty_states", "minecraft:stone", `[]`, 180, `[]`, "石头不变"},
		{"no_facing_states", "minecraft:glass", `["color"="white"]`, 90, `["color"="white"]`, "玻璃颜色不变"},
		{"degrees_zero", "minecraft:command_block", `["facing_direction"=2]`, 0, `["facing_direction"=2]`, "0° 不变"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rotateBlockStates(tt.block, tt.states, tt.degrees)
			if got != tt.expected {
				t.Errorf("[%s] rotateBlockStates(%q, %q, %d)\n  got:      %s\n  expected: %s", tt.desc, tt.block, tt.states, tt.degrees, got, tt.expected)
			}
		})
	}
}