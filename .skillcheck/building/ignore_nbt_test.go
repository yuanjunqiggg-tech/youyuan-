package building

import "testing"

// TestRunUnitIgnoresSimpleNBT 单元模式：带 NBT 的普通方块（磁石等）当普通方块放置，
// 不逐块 TP 补发（否则阻塞大工作区）。
func TestRunUnitIgnoresSimpleNBT(t *testing.T) {
	d := mkData(64)
	// 磁石带 NBT（trackingHandle），但 IsWorkspaceNBTBlock 返回 false → CatNormal 留在普通单元
	setBlockAt(t, d, 1, 1, 1, "minecraft:lodestone", map[string]any{"trackingHandle": map[string]any{"id": int32(1)}})
	setBlockAt(t, d, 2, 1, 1, "minecraft:stone", nil)

	res, err := SplitStructure(d, SplitOptions{BotCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	var u *Unit
	for _, uu := range res.Units {
		if uu.Kind == UnitNormal {
			u = uu
			break
		}
	}
	if u == nil {
		t.Fatal("没找到普通单元")
	}
	// 磁石不应被归为工作区NBT
	for _, uu := range res.Units {
		if uu.Kind == UnitNBT {
			for _, p := range uu.Positions {
				if p == [3]int{1, 1, 1} {
					t.Fatal("磁石不应被分类为工作区NBT")
				}
			}
		}
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
	if err := it.RunUnit(u); err != nil {
		t.Fatal(err)
	}
	// 磁石被放置（setblock 或 fill 覆盖）
	placed := false
	for _, p := range rec.set {
		if p == [3]int{1, 1, 1} {
			placed = true
		}
	}
	if !placed {
		for _, f := range rec.fills {
			if f[0] <= 1 && f[3] >= 1 && f[1] <= 1 && f[4] >= 1 && f[2] <= 1 && f[5] >= 1 {
				placed = true
			}
		}
	}
	if !placed {
		t.Error("磁石应被放置（单元模式忽略NBT后当普通方块放）")
	}
	if rec.hasNBT([3]int{1, 1, 1}) {
		t.Error("单元模式磁石不应触发 OnNBT（不逐块 TP 补发）")
	}
}

// TestSingleBotStillOnNBTForSimpleNBT 单机模式：带 NBT 普通方块仍触发 OnNBT（回归保护）。
func TestSingleBotStillOnNBTForSimpleNBT(t *testing.T) {
	d := mkData(64)
	setBlockAt(t, d, 1, 1, 1, "minecraft:lodestone", map[string]any{"trackingHandle": map[string]any{}})

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
	if !rec.hasSet([3]int{1, 1, 1}) {
		t.Error("单机模式磁石应被 setblock")
	}
	if !rec.hasNBT([3]int{1, 1, 1}) {
		t.Error("单机模式磁石应触发 OnNBT")
	}
}
