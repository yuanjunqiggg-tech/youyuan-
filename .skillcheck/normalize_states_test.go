package main

import "testing"

// TestNormalizeBlockStates 验证命令方块朝向提取兼容带引号与不带引号两种 states 格式。
// 回归: 此前 normalizeBlockStates 只匹配带引号 "facing_direction"=，而 BDX 生成不带引号的
// [facing_direction=4,...]，导致匹配失败 → setblock 无朝向 → 命令方块全部默认朝下。
func TestNormalizeBlockStates(t *testing.T) {
	cases := []struct {
		name   string
		states string
		want   string
	}{
		// 不带引号（BDX 生成格式）
		{"bdx_unquoted_north", "[conditional_bit=0,facing_direction=2]", "2"},
		{"bdx_unquoted_west", "[conditional_bit=0,facing_direction=4]", "4"},
		// 带引号（代码库标准格式）
		{"quoted_south", `["conditional_bit"=false,"facing_direction"=3]`, "3"},
		// facing_direction=0（朝下）→ 空，让网易默认朝下
		{"down_zero", "[facing_direction=0,conditional_bit=0]", ""},
		// 非命令方块原样透传
		{"non_cmd", `["facing"=4]`, `["facing"=4]`},
		// 命令方块无朝向 → 空
		{"cmd_no_facing", `["conditional_bit"=true]`, ""},
	}
	for _, c := range cases {
		got := normalizeBlockStates("minecraft:command_block", c.states)
		if c.name == "non_cmd" {
			got = normalizeBlockStates("minecraft:furnace", c.states)
		}
		if got != c.want {
			t.Errorf("%s: normalizeBlockStates(%q) = %q, want %q", c.name, c.states, got, c.want)
		}
	}
}
