package building

import (
	"sync"
	"testing"
)

// recTask 记录一次 Run 的放置调用，用于断言。
type recTask struct {
	mu    sync.Mutex
	set   [][3]int
	nbt   [][3]int
	fills [][6]int
}

func (r *recTask) hasSet(p [3]int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, q := range r.set {
		if q == p {
			return true
		}
	}
	return false
}

func (r *recTask) hasNBT(p [3]int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, q := range r.nbt {
		if q == p {
			return true
		}
	}
	return false
}

// buildUnitFixture 造一个结构：
//   岛A：chunk(0,0) 放石头 (1,1,1)、(2,2,2) 和满箱子 (5,1,1)；chunk(1,0) 放石头 (17,1,1)
//   岛B：chunk(10,0) 放石头 (160,1,1)（独立普通单元）
// 返回 data 和切分结果。
func buildUnitFixture(t *testing.T) (*StructureData, *SplitResult) {
	t.Helper()
	d := mkData(200)
	setBlockAt(t, d, 1, 1, 1, "minecraft:stone", nil)
	setBlockAt(t, d, 2, 2, 2, "minecraft:stone", nil)
	setBlockAt(t, d, 5, 1, 1, "minecraft:chest", map[string]any{ // 满箱 → 抠出
		"Items": []any{map[string]any{"Name": "minecraft:diamond", "Count": int32(1)}},
	})
	setBlockAt(t, d, 17, 1, 1, "minecraft:stone", nil)
	setBlockAt(t, d, 160, 1, 1, "minecraft:stone", nil) // 岛B
	// BotCount=2：普通区块列按数量均分成 2 个大工作区 → 单元A=[(0,0),(1,0)] 岛B=[(10,0)]
	res, err := SplitStructure(d, SplitOptions{BotCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	return d, res
}

// TestRunUnitScoped 单元模式只放本单元区块，且跳过被抠出的工作区NBT位置（§8.1）。
func TestRunUnitScoped(t *testing.T) {
	d, res := buildUnitFixture(t)
	var unitA *Unit
	for _, u := range res.Units {
		if u.Kind == UnitNormal && len(u.Chunks) == 2 {
			unitA = u
		}
	}
	if unitA == nil {
		t.Fatal("没找到覆盖两个区块列的普通单元 A")
	}

	rec := &recTask{}
	it := &ImportTask{
		Data:   d,
		Speed:  20000,
		StopCh: make(chan struct{}),
		SetBlock: func(x, y, z int32, name, states string) error {
			rec.mu.Lock()
			rec.set = append(rec.set, [3]int{int(x), int(y), int(z)})
			rec.mu.Unlock()
			return nil
		},
		FillRegion: func(x1, y1, z1, x2, y2, z2 int32, name, states string) error {
			rec.mu.Lock()
			rec.fills = append(rec.fills, [6]int{int(x1), int(y1), int(z1), int(x2), int(y2), int(z2)})
			rec.mu.Unlock()
			return nil
		},
		OnNBT: func(x, y, z int32, nbt map[string]any, name, states string) {
			rec.mu.Lock()
			rec.nbt = append(rec.nbt, [3]int{int(x), int(y), int(z)})
			rec.mu.Unlock()
		},
		Teleport: func(x, y, z int) error { return nil },
	}
	if err := it.RunUnit(unitA); err != nil {
		t.Fatal(err)
	}

	// 本单元内的石头应该都被放
	for _, p := range [][3]int{{1, 1, 1}, {2, 2, 2}, {17, 1, 1}} {
		if !rec.hasSet(p) {
			t.Errorf("单元A内的方块 %v 未被放置", p)
		}
	}
	// 满箱子（被抠出）不能被普通单元放，也不能触发 OnNBT
	if rec.hasSet([3]int{5, 1, 1}) {
		t.Error("满箱子位置 (5,1,1) 不应被普通单元放置（§8.1）")
	}
	if rec.hasNBT([3]int{5, 1, 1}) {
		t.Error("满箱子不应触发普通单元的 OnNBT")
	}
	// 岛B 的方块（属于另一个单元）不能被单元A放
	if rec.hasSet([3]int{160, 1, 1}) {
		t.Error("岛B 的方块 (160,1,1) 不应被单元A放置")
	}
}

// TestRunUnitPreclearSkipped 单元模式不触发预清空（多机器人由一台先整图清一次）。
func TestRunUnitPreclearSkipped(t *testing.T) {
	d, res := buildUnitFixture(t)
	var unitA *Unit
	for _, u := range res.Units {
		if u.Kind == UnitNormal && len(u.Chunks) == 2 {
			unitA = u
		}
	}
	clearFired := false
	it := &ImportTask{
		Data:           d,
		Speed:          20000,
		PreClearMode:   1,
		StopCh:         make(chan struct{}),
		SetBlock:       func(x, y, z int32, name, states string) error { return nil },
		Teleport:       func(x, y, z int) error { return nil },
		OnClearProgress: func(c, total int) { clearFired = true },
	}
	if err := it.RunUnit(unitA); err != nil {
		t.Fatal(err)
	}
	if clearFired {
		t.Error("单元模式不应执行预清空")
	}
}

// TestRunSingleBotStillPlacesAll 单机模式（无 PlaceOnly/SkipPositions）行为不变：
// 满箱子的 setblock + OnNBT 仍会被触发。
func TestRunSingleBotStillPlacesAll(t *testing.T) {
	d, _ := buildUnitFixture(t)
	rec := &recTask{}
	it := &ImportTask{
		Data:   d,
		Speed:  20000,
		StopCh: make(chan struct{}),
		SetBlock: func(x, y, z int32, name, states string) error {
			rec.mu.Lock()
			rec.set = append(rec.set, [3]int{int(x), int(y), int(z)})
			rec.mu.Unlock()
			return nil
		},
		FillRegion: func(x1, y1, z1, x2, y2, z2 int32, name, states string) error { return nil },
		OnNBT: func(x, y, z int32, nbt map[string]any, name, states string) {
			rec.mu.Lock()
			rec.nbt = append(rec.nbt, [3]int{int(x), int(y), int(z)})
			rec.mu.Unlock()
		},
		Teleport: func(x, y, z int) error { return nil },
	}
	if err := it.Run(); err != nil {
		t.Fatal(err)
	}
	if !rec.hasSet([3]int{5, 1, 1}) {
		t.Error("单机模式满箱子 (5,1,1) 应被 setblock")
	}
	if !rec.hasNBT([3]int{5, 1, 1}) {
		t.Error("单机模式满箱子应触发 OnNBT")
	}
	if !rec.hasSet([3]int{160, 1, 1}) {
		t.Error("单机模式岛B方块 (160,1,1) 应被放置")
	}
}
