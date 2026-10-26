package main

import (
	"sync"
	"testing"
	"time"
)

// ── RegionFromOrigin ──

func TestRegionFromOriginRect(t *testing.T) {
	// 矩形：W≠D（不再强制正方形）
	r := RegionFromOrigin(0, 0, 11, 6, 64)
	if r.W != 11 || r.D != 6 {
		t.Fatalf("W,D=%d,%d want 11,6", r.W, r.D)
	}
	if r.X0 != 0 || r.Z0 != 0 || r.Y != 64 {
		t.Fatalf("origin=%+v", r)
	}
	// 非零起点
	r = RegionFromOrigin(3, 5, 16, 16, 64)
	if r.X0 != 3 || r.Z0 != 5 || r.W != 16 || r.D != 16 {
		t.Fatalf("got %+v", r)
	}
}

func TestRegionFromOriginMax512(t *testing.T) {
	r := RegionFromOrigin(0, 0, 10000, 8000, 64)
	if r.W != 512 || r.D != 512 {
		t.Fatalf("W,D=%d,%d want 512,512", r.W, r.D)
	}
	// 单边超限只夹该边
	r = RegionFromOrigin(0, 0, 3, 900, 64)
	if r.W != 3 || r.D != 512 {
		t.Fatalf("rect W,D=%d,%d want 3,512", r.W, r.D)
	}
}

// ── 创建模式 ──

func TestCreateTransparent(t *testing.T) {
	em := &emitRecorder{}
	s := newDrawSessionWith(em.record, RegionFromOrigin(0, 0, 5, 5, 64), DrawModeTransparent, "", DrawSyncRealtime)
	for x := 0; x < s.region.W; x++ {
		for z := 0; z < s.region.D; z++ {
			if !s.grid[x][z].Trans {
				t.Fatalf("transparent mode cell %d,%d not transparent", x, z)
			}
		}
	}
}

func TestCreateFill(t *testing.T) {
	s := newDrawSession(RegionFromOrigin(0, 0, 5, 5, 64), DrawModeFill, "minecraft:red_concrete", DrawSyncRealtime)
	for x := 0; x < s.region.W; x++ {
		for z := 0; z < s.region.D; z++ {
			c := s.grid[x][z]
			if c.Trans || c.Block != "minecraft:red_concrete" {
				t.Fatalf("fill cell %d,%d = %+v", x, z, c)
			}
		}
	}
}

// ── planPlace ──

func TestPlanPlaceGroupsByChunk(t *testing.T) {
	region := RegionFromOrigin(0, 0, 3, 3, 64) // side 3, x0=0,z0=0
	changes := []CellChange{
		{X: 0, Z: 0, Next: Cell{Block: "minecraft:red_concrete"}},
		{X: 2, Z: 2, Next: Cell{Block: "minecraft:blue_concrete"}},
		{X: 0, Z: 2, Next: Cell{Trans: true}}, // 透明不放置
	}
	ops := planPlace(region, changes)
	// 期望：先 TP 到区块中心，再放 2 个方块；透明被跳过
	if len(ops) != 3 {
		t.Fatalf("ops=%d want 3 (1 TP + 2 setblock)", len(ops))
	}
	if !ops[0].isTP {
		t.Fatalf("op[0] should be TP")
	}
	if ops[0].chunkXZ != [2]int{0, 0} {
		t.Fatalf("TP chunk=%v want {0,0}", ops[0].chunkXZ)
	}
	for _, op := range ops[1:] {
		if op.isTP {
			t.Fatalf("unexpected TP after first")
		}
		if op.block == "" {
			t.Fatalf("setblock with empty block: %+v", op)
		}
	}
}

func TestPlanPlaceCrossChunkTP(t *testing.T) {
	// 两个不同区块的变化 → 两次 TP
	region := RegionFromOrigin(0, 0, 41, 41, 64) // side 41, spans multiple chunks
	changes := []CellChange{
		{X: 0, Z: 0, Next: Cell{Block: "minecraft:red_concrete"}},       // chunk {0,0}
		{X: 40, Z: 40, Next: Cell{Block: "minecraft:blue_concrete"}},    // chunk {2,2}
	}
	ops := planPlace(region, changes)
	tps := 0
	for _, op := range ops {
		if op.isTP {
			tps++
		}
	}
	if tps != 2 {
		t.Fatalf("TP count=%d want 2", tps)
	}
	// 每个 setblock 前有 TP
	seenTP := false
	ordered := true
	for _, op := range ops {
		if op.isTP {
			seenTP = true
		} else if !seenTP {
			ordered = false
		}
	}
	if !ordered {
		t.Fatalf("setblock appears before its chunk TP")
	}
}

// ── ApplyStroke / 实时放置 / 撤销 ──

func TestApplyStrokeRealtimePlaces(t *testing.T) {
	var mu sync.Mutex
	var placed []string
	done := make(chan struct{})
	em := &emitRecorder{}
	s := newDrawSessionWith(em.record, RegionFromOrigin(0, 0, 3, 3, 64), DrawModeTransparent, "", DrawSyncRealtime)
	s.readFn = func(x, y, z int) (string, error) { return "minecraft:stone", nil } // 世界基线
	count := 0
	s.placeFn = func(x, y, z int, block string) error {
		mu.Lock()
		placed = append(placed, block)
		count++
		if count == 2 {
			close(done)
		}
		mu.Unlock()
		return nil
	}

	s.ApplyStroke([]StrokePoint{
		{X: 0, Z: 0, Cell: Cell{Block: "minecraft:red_concrete"}},
		{X: 1, Z: 1, Cell: Cell{Block: "minecraft:red_concrete"}},
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("placement did not run")
	}
	// 放置了 2 个 red_concrete（1个stroke → 1次 TP + 2 setblock）
	if len(placed) != 2 {
		t.Fatalf("placed=%v want 2 blocks", placed)
	}
	for _, b := range placed {
		if b != "minecraft:red_concrete" {
			t.Fatalf("placed block=%q", b)
		}
	}
	// 网格已更新
	if s.grid[0][0].Block != "minecraft:red_concrete" {
		t.Fatalf("grid[0][0]=%+v", s.grid[0][0])
	}
}

func TestPlacementLocksDuringPlacing(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	em := &emitRecorder{}
	s := newDrawSessionWith(em.record, RegionFromOrigin(0, 0, 3, 3, 64), DrawModeTransparent, "", DrawSyncRealtime)
	s.readFn = func(x, y, z int) (string, error) { return "minecraft:stone", nil }
	s.placeFn = func(x, y, z int, block string) error {
		close(blocked)
		<-release
		return nil
	}
	s.ApplyStroke([]StrokePoint{{X: 0, Z: 0, Cell: Cell{Block: "minecraft:red_concrete"}}})
	<-blocked // 确保正在放置中
	if !s.Placing() {
		t.Fatalf("expected placing=true while placing")
	}
	// 放置中再次画 → 被拒绝（网格保持透明未改动）
	s.ApplyStroke([]StrokePoint{{X: 1, Z: 1, Cell: Cell{Block: "minecraft:blue_concrete"}}})
	if !s.grid[1][1].Trans {
		t.Fatalf("stroke during placing should be rejected, grid=%+v", s.grid[1][1])
	}
	close(release)
}

func TestUndoRedo(t *testing.T) {
	em := &emitRecorder{}
	s := newDrawSessionWith(em.record, RegionFromOrigin(0, 0, 3, 3, 64), DrawModeTransparent, "", DrawSyncBatch)
	s.readFn = func(x, y, z int) (string, error) { return "minecraft:stone", nil }
	s.ApplyStroke([]StrokePoint{{X: 0, Z: 0, Cell: Cell{Block: "minecraft:red_concrete"}}})
	if s.grid[0][0].Block != "minecraft:red_concrete" {
		t.Fatalf("after stroke grid=%+v", s.grid[0][0])
	}
	s.Undo()
	// undo 恢复到世界基线 stone
	if s.grid[0][0].Block != "minecraft:stone" || s.grid[0][0].Trans {
		t.Fatalf("after undo grid=%+v want stone", s.grid[0][0])
	}
	s.Redo()
	if s.grid[0][0].Block != "minecraft:red_concrete" {
		t.Fatalf("after redo grid=%+v", s.grid[0][0])
	}
}

func TestBatchApply(t *testing.T) {
	var mu sync.Mutex
	placed := 0
	done := make(chan struct{})
	em := &emitRecorder{}
	s := newDrawSessionWith(em.record, RegionFromOrigin(0, 0, 3, 3, 64), DrawModeTransparent, "", DrawSyncBatch)
	s.readFn = func(x, y, z int) (string, error) { return "minecraft:stone", nil }
	s.placeFn = func(x, y, z int, block string) error { mu.Lock(); placed++; mu.Unlock(); return nil }

	// 批量模式下多笔画不立即放置
	s.ApplyStroke([]StrokePoint{{X: 0, Z: 0, Cell: Cell{Block: "minecraft:red_concrete"}}})
	s.ApplyStroke([]StrokePoint{{X: 1, Z: 1, Cell: Cell{Block: "minecraft:blue_concrete"}}})
	if s.stateLocked()["pending"].(int) != 2 {
		t.Fatalf("pending=%v want 2", s.stateLocked()["pending"])
	}
	// 触发 apply 完成放置
	s.placeFn = func(x, y, z int, block string) error {
		mu.Lock(); placed++; mu.Unlock()
		if placed == 2 {
			close(done)
		}
		return nil
	}
	s.Apply()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("apply placement did not run")
	}
	if placed != 2 {
		t.Fatalf("placed=%d want 2", placed)
	}
	if s.stateLocked()["pending"].(int) != 0 {
		t.Fatalf("pending after apply=%v want 0", s.stateLocked()["pending"])
	}
}

// emitRecorder 记录 emit 调用。
type emitRecorder struct {
	mu sync.Mutex
	ev []string
}

func (e *emitRecorder) record(event, data string) {
	e.mu.Lock()
	e.ev = append(e.ev, event)
	e.mu.Unlock()
}
