package main

import (
	"fmt"
	"testing"
)

// 纯文本：每行一条命令。链首脉冲、其余连锁、无延迟；默认"脉冲需红石、连锁保持开启"。
func TestParseCBProject_PlainText(t *testing.T) {
	proj := parseCBProject("say a\nsay b\nsay c")
	if len(proj.Blocks) != 3 {
		t.Fatalf("got %d blocks", len(proj.Blocks))
	}
	want := []struct {
		typ string
		rs  bool // 需红石
	}{
		{"impulse", true}, {"chain", false}, {"chain", false},
	}
	for i, w := range want {
		b := proj.Blocks[i]
		if b.Type != w.typ || b.NeedsRS != w.rs || b.TickDelay != 0 {
			t.Errorf("block[%d]: got type=%q rs=%v delay=%d, want type=%q rs=%v delay=0",
				i, b.Type, b.NeedsRS, b.TickDelay, w.typ, w.rs)
		}
	}
}

// 空行开新链：第二条链重新为脉冲，且链首（ChainStart）标记正确。
func TestParseCBProject_EmptyLineNewChain(t *testing.T) {
	proj := parseCBProject("say a\nsay b\n\nsay c\nsay d")
	wantTypes := []string{"impulse", "chain", "impulse", "chain"}
	wantStart := []bool{true, false, true, false}
	for i := range wantTypes {
		b := proj.Blocks[i]
		if b.Type != wantTypes[i] {
			t.Errorf("block[%d] type=%q want %q", i, b.Type, wantTypes[i])
		}
		if b.ChainStart != wantStart[i] {
			t.Errorf("block[%d] ChainStart=%v want %v", i, b.ChainStart, wantStart[i])
		}
	}
}

// 验证布局：每条链首在 x 方向逐条排开（旁边另起），链内沿朝向延伸。
func TestCBPlacementLayout(t *testing.T) {
	proj := parseCBProject("say a\nsay b\n\nsay c")
	// 链1：a(0,0,0 链首 z+), b(0,0,1)；链2：c(1,0,0 链首)
	x, y, z := 0, 0, 0
	cx, cy, cz := x, y, z
	var pos []string
	for _, b := range proj.Blocks {
		if b.ChainStart {
			cx, cy, cz = x, y, z
			x++
		}
		pos = append(pos, fmt.Sprintf("%d,%d,%d", cx, cy, cz))
		cx, cy, cz = nextCBPos(cx, cy, cz, b.Facing)
	}
	want := []string{"0,0,0", "0,0,1", "1,0,0"}
	for i, w := range want {
		if pos[i] != w {
			t.Errorf("block[%d] at %s want %s", i, pos[i], w)
		}
	}
}

// 混合：显式类型继承 + 属性覆盖 + 空行新链。
func TestParseCBProject_Mixed(t *testing.T) {
	content := "//工程:测试\n# 类型=循环 延迟=2\nsay rep\nsay chain\n\n# 朝向=x- 有条件=是\nsay newchain"
	proj := parseCBProject(content)
	// 空行后 say newchain 是新链首 → 脉冲。共 3 个方块。
	if len(proj.Blocks) != 3 {
		t.Fatalf("got %d blocks: %+v", len(proj.Blocks), proj.Blocks)
	}
	if proj.Name != "测试" {
		t.Errorf("name=%q want 测试", proj.Name)
	}
	b0, b1, b2 := proj.Blocks[0], proj.Blocks[1], proj.Blocks[2]
	if b0.Type != "repeating" || b0.TickDelay != 2 {
		t.Errorf("b0: got type=%q delay=%d, want repeating delay=2", b0.Type, b0.TickDelay)
	}
	if b1.Type != "repeating" {
		t.Errorf("b1 type=%q want repeating (inherited)", b1.Type)
	}
	if b2.Type != "impulse" || b2.Facing != "x-" || !b2.Conditional {
		t.Errorf("b2: got type=%q facing=%q cond=%v, want impulse x- cond=true", b2.Type, b2.Facing, b2.Conditional)
	}
}
