package main

import "testing"

// 验证链追踪：含上下朝向的链能正确沿朝向走通。
// impulse(0,0,0 z+) -> chain(0,0,1 y+) -> chain(0,1,1 z+) -> chain(0,1,2 z+)
func TestTraceChains_WithVertical(t *testing.T) {
	blocks := []CBSimpleBlock{
		{X: 0, Y: 0, Z: 0, Type: "impulse", Facing: "z+"},
		{X: 0, Y: 0, Z: 1, Type: "chain", Facing: "y+"},
		{X: 0, Y: 1, Z: 1, Type: "chain", Facing: "z+"},
		{X: 0, Y: 1, Z: 2, Type: "chain", Facing: "z+"},
	}
	chains := traceCommandBlockChains(blocks)
	if len(chains) != 1 {
		t.Fatalf("got %d chains, want 1", len(chains))
	}
	if len(chains[0].Blocks) != 4 {
		t.Fatalf("chain has %d blocks, want 4", len(chains[0].Blocks))
	}
	// 验证顺序和坐标（应沿朝向依次走通）
	want := [][3]int{{0, 0, 0}, {0, 0, 1}, {0, 1, 1}, {0, 1, 2}}
	for i, w := range want {
		b := chains[0].Blocks[i]
		if b.X != w[0] || b.Y != w[1] || b.Z != w[2] {
			t.Errorf("block[%d] at (%d,%d,%d), want (%d,%d,%d)", i, b.X, b.Y, b.Z, w[0], w[1], w[2])
		}
	}
}

// 验证面向解析：facing_direction 各值映射到正确的方向名（含上下）。
func TestFacingNames(t *testing.T) {
	// 模拟提取：0->y-, 1->y+, 2->z-, 3->z+, 4->x-, 5->x+
	want := map[int]string{0: "y-", 1: "y+", 2: "z-", 3: "z+", 4: "x-", 5: "x+"}
	for fd, name := range want {
		if got := cbFacingNames[fd]; got != name {
			t.Errorf("fd=%d -> %q, want %q", fd, got, name)
		}
	}
}
