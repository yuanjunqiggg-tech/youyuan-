package building

import (
	"sync"
	"testing"
)

// keepRec 记录 Run 的放置调用，用于断言 keep 模式行为。
type keepRec struct {
	mu  sync.Mutex
	set [][3]int
	nbt [][3]int
}

// mkKeepTask 建一个带 (5,1,1) NBT 箱子、KeepMode 开启、IsAir 固定的 ImportTask。
// isAir 返回 false 表示目标位非空气。返回 task 与记录器。
func mkKeepTask(t *testing.T, isAir bool) (*ImportTask, *keepRec) {
	t.Helper()
	d := mkData(64)
	// 空 Items 的箱子 → CatNormal，留在普通放置路径，仍带 NBT → 走 hasNBT 分支
	setBlockAt(t, d, 5, 1, 1, "minecraft:chest", map[string]any{"Items": []any{}})

	rec := &keepRec{}
	it := &ImportTask{
		Data:     d,
		Speed:    20000,
		StopCh:   make(chan struct{}),
		KeepMode: true,
		IsAir: func(x, y, z int32) bool {
			return isAir
		},
		SetBlock: func(x, y, z int32, name, states string) error {
			rec.mu.Lock()
			rec.set = append(rec.set, [3]int{int(x), int(y), int(z)})
			rec.mu.Unlock()
			return nil
		},
		FillRegion: func(x1, y1, z1, x2, y2, z2 int32, name, states string) error {
			return nil
		},
		OnNBT: func(x, y, z int32, nbt map[string]any, name, states string) {
			rec.mu.Lock()
			rec.nbt = append(rec.nbt, [3]int{int(x), int(y), int(z)})
			rec.mu.Unlock()
		},
	}
	return it, rec
}

func hasPt(set [][3]int, p [3]int) bool {
	for _, q := range set {
		if q == p {
			return true
		}
	}
	return false
}

// TestKeepModeSkipsOccupiedNBT 目标位非空气时，NBT 方块既不 setblock 也不触发 OnNBT。
func TestKeepModeSkipsOccupiedNBT(t *testing.T) {
	it, rec := mkKeepTask(t, false) // IsAir=false → 目标位非空气
	if err := it.Run(); err != nil {
		t.Fatal(err)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if hasPt(rec.set, [3]int{5, 1, 1}) {
		t.Error("keep 模式：非空气目标位不应 setblock (5,1,1)")
	}
	if hasPt(rec.nbt, [3]int{5, 1, 1}) {
		t.Error("keep 模式：非空气目标位不应触发 OnNBT (5,1,1)")
	}
}

// TestKeepModePlacesAirNBT 目标位为空气时，NBT 方块正常放置。
func TestKeepModePlacesAirNBT(t *testing.T) {
	it, rec := mkKeepTask(t, true) // IsAir=true → 目标位空气
	if err := it.Run(); err != nil {
		t.Fatal(err)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if !hasPt(rec.set, [3]int{5, 1, 1}) {
		t.Error("keep 模式：空气目标位应 setblock (5,1,1)")
	}
	if !hasPt(rec.nbt, [3]int{5, 1, 1}) {
		t.Error("keep 模式：空气目标位应触发 OnNBT (5,1,1)")
	}
}
