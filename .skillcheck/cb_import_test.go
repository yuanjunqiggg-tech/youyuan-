package main

import "testing"

// 导出(generateCBProject) → 导入(parseCBProject) round-trip：
// 验证导入解析器能还原本工具导出的工程，保证导入链路可用。
func TestCBProjectRoundTrip(t *testing.T) {
	chains := []CBlockChain{
		{
			Blocks: []CBSimpleBlock{
				{X: 0, Y: 0, Z: 0, Command: "say start", Type: "impulse", Facing: "z+", Conditional: false, Auto: true, TickDelay: 0},
				{X: 0, Y: 0, Z: 1, Command: "say cond", Type: "chain", Facing: "z+", Conditional: true, Auto: false, TickDelay: 0},
				{X: 0, Y: 0, Z: 2, Command: "say rep", Type: "repeating", Facing: "z+", Conditional: false, Auto: false, TickDelay: 3, ExeFirstTick: true},
			},
		},
	}

	content := generateCBProject(chains)
	proj := parseCBProject(content)

	if len(proj.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d\ncontent:\n%s", len(proj.Blocks), content)
	}

	// 断言每个 block 的属性被正确还原
	checks := []struct {
		cmd      string
		typ      string
		facing   string
		cond     bool
		needsRS  bool
		tick     int
		exeFirst bool
	}{
		{"say start", "impulse", "z+", false, false, 0, false},
		{"say cond", "chain", "z+", true, true, 0, false},
		{"say rep", "repeating", "z+", false, true, 3, true},
	}
	for i, c := range checks {
		b := proj.Blocks[i]
		if b.Command != c.cmd || b.Type != c.typ || b.Facing != c.facing ||
			b.Conditional != c.cond || b.NeedsRS != c.needsRS ||
			b.TickDelay != c.tick || b.ExeFirst != c.exeFirst {
			t.Errorf("block[%d] mismatch: got {cmd=%q type=%q facing=%q cond=%v rs=%v tick=%d exe=%v}, want {%q,%q,%q,%v,%v,%d,%v}",
				i, b.Command, b.Type, b.Facing, b.Conditional, b.NeedsRS, b.TickDelay, b.ExeFirst,
				c.cmd, c.typ, c.facing, c.cond, c.needsRS, c.tick, c.exeFirst)
		}
	}
}
