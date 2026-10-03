package building

import (
	"testing"
)

// setBlockAt 在结构 (x,y,z) 处放一个方块；带 nbt 时记录方块实体数据。
func setBlockAt(t *testing.T, d *StructureData, x, y, z int, name string, nbt map[string]any) {
	t.Helper()
	d.Set(x, y, z, &BlockInfo{Name: name, States: "[]", NBT: nbt})
}

// mkData 建一个 size³ 的空结构（稀疏 Blocks 存储）。
func mkData(size int) *StructureData {
	return &StructureData{SizeX: size, SizeY: size, SizeZ: size, Blocks: make(map[uint64]uint32)}
}

// ── 分类器：空方块 vs 有内容方块（§4.1.1） ──

func TestClassifyEmptyNBTBlocksAreNormal(t *testing.T) {
	// 空容器：Items 为空数组，无可制作内容 → CatNormal（不抠出）
	if got := ClassifyBlock("minecraft:chest", map[string]any{"Items": []any{}}); got != CatNormal {
		t.Errorf("空箱子应为 CatNormal，got %v", got)
	}
	// 全 air 的箱子 → CatNormal
	if got := ClassifyBlock("minecraft:chest", map[string]any{
		"Items": []any{map[string]any{"Name": "minecraft:air", "Count": int32(1)}},
	}); got != CatNormal {
		t.Errorf("只含 air 的箱子应为 CatNormal，got %v", got)
	}
	// 空展示框：无 Item 键 → CatNormal
	if got := ClassifyBlock("minecraft:item_frame", map[string]any{}); got != CatNormal {
		t.Errorf("空展示框应为 CatNormal，got %v", got)
	}
	// 空白旗：无 Patterns / Base → CatNormal
	if got := ClassifyBlock("minecraft:banner", map[string]any{"CustomName": "x"}); got != CatNormal {
		t.Errorf("空白旗（仅自定义名）应为 CatNormal，got %v", got)
	}
}

func TestClassifyContentNBTBlocksNeedWorkbench(t *testing.T) {
	// 有物品的箱子 → CatWorkspaceNBT
	if got := ClassifyBlock("minecraft:chest", map[string]any{
		"Items": []any{map[string]any{"Name": "minecraft:diamond", "Count": int32(1)}},
	}); got != CatWorkspaceNBT {
		t.Errorf("有物品箱子应为 CatWorkspaceNBT，got %v", got)
	}
	// 有图案的旗帜 → CatWorkspaceNBT
	if got := ClassifyBlock("minecraft:banner", map[string]any{"Patterns": []any{}}); got != CatWorkspaceNBT {
		t.Errorf("有图案旗帜应为 CatWorkspaceNBT，got %v", got)
	}
	// 有物品的展示框 → CatWorkspaceNBT
	if got := ClassifyBlock("minecraft:item_frame", map[string]any{"Item": map[string]any{}}); got != CatWorkspaceNBT {
		t.Errorf("有物品展示框应为 CatWorkspaceNBT，got %v", got)
	}
}

func TestClassifySignIsCatSign(t *testing.T) {
	if got := ClassifyBlock("minecraft:oak_sign", nil); got != CatSign {
		t.Errorf("告示牌应为 CatSign，got %v", got)
	}
	// 命令方块是"轻NBT"，独立成单元（不需要工作台），不归告示牌
	if got := ClassifyBlock("minecraft:command_block", map[string]any{"Command": "say hi"}); got != CatLightNBT {
		t.Errorf("命令方块应为 CatLightNBT，got %v", got)
	}
	// 结构方块同样归"轻NBT"
	if got := ClassifyBlock("minecraft:structure_block", map[string]any{}); got != CatLightNBT {
		t.Errorf("结构方块应为 CatLightNBT，got %v", got)
	}
}

// ── 切岛：空区域不产生单元；相邻列连成岛 ──

func TestSplitIslands(t *testing.T) {
	d := mkData(64)
	// 3 个方块分处不相邻区块列 (0,0) (0,1) (3,3)；空区域不产生单元
	setBlockAt(t, d, 1, 1, 1, "minecraft:stone", nil)
	setBlockAt(t, d, 1, 1, 17, "minecraft:stone", nil)
	setBlockAt(t, d, 50, 1, 50, "minecraft:stone", nil)

	// BotCount=2 → 非空区块列均分成 2 个大工作区
	res, err := SplitStructure(d, SplitOptions{BotCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	var normalUnits []*Unit
	for _, u := range res.Units {
		if u.Kind == UnitNormal {
			normalUnits = append(normalUnits, u)
		}
	}
	if len(normalUnits) != 2 {
		t.Fatalf("BotCount=2 应切 2 个普通大工作区，got %d", len(normalUnits))
	}
	if res.TotalSolid != 3 || res.NormalSolid != 3 {
		t.Fatalf("TotalSolid=%d NormalSolid=%d, 期望 3/3", res.TotalSolid, res.NormalSolid)
	}
	// 每个非空区块列恰好被覆盖一次（无重叠、无遗漏）
	seen := map[[2]int]int{}
	for _, u := range normalUnits {
		for _, c := range u.Chunks {
			seen[c]++
		}
	}
	if len(seen) != 3 {
		t.Fatalf("应覆盖 3 个非空区块列，got %d", len(seen))
	}
	for _, c := range [][2]int{{0, 0}, {0, 1}, {3, 3}} {
		if seen[c] != 1 {
			t.Errorf("区块列 %v 应覆盖 1 次，got %d", c, seen[c])
		}
	}
}

// ── 正确性底线 §8.1：每个方块恰好属于一个单元，NBT 位置被抠出 ──

func TestNoOverlapEachBlockInOneUnit(t *testing.T) {
	d := mkData(32)
	setBlockAt(t, d, 0, 1, 0, "minecraft:stone", nil)                         // 普通
	setBlockAt(t, d, 1, 1, 0, "minecraft:chest", map[string]any{"Items": []any{}}) // 空箱 → 普通
	setBlockAt(t, d, 2, 1, 0, "minecraft:chest", map[string]any{ // 满箱 → 工作区NBT
		"Items": []any{map[string]any{"Name": "minecraft:diamond", "Count": int32(1)}},
	})
	setBlockAt(t, d, 3, 1, 0, "minecraft:oak_sign", map[string]any{"Text": "hi"}) // 告示牌
	setBlockAt(t, d, 4, 1, 0, "minecraft:command_block", map[string]any{"Command": "say hi"}) // 轻NBT → 普通

	res, err := SplitStructure(d, SplitOptions{})
	if err != nil {
		t.Fatal(err)
	}

	nbtCount := map[[3]int]int{}
	signSet := map[[3]int]bool{}
	skipCount := map[[3]int]int{} // 普通单元列内被抠出的位置（§8.1）
	normalCols := map[[2]int]int{}
	for _, u := range res.Units {
		switch u.Kind {
		case UnitNBT:
			for _, p := range u.Positions {
				nbtCount[p]++
			}
		case UnitSign:
			for _, p := range u.Positions {
				signSet[p] = true
			}
		case UnitNormal:
			for _, c := range u.Chunks {
				normalCols[c]++
			}
			for _, p := range u.Skipped {
				skipCount[p]++
			}
		}
	}

	covered := 0
	for _, bp := range d.AllBlocks() {
		info := &d.Palette[bp.Index]
		if info.IsAir {
			continue
		}
		p := [3]int{int(bp.X), int(bp.Y), int(bp.Z)}
		nbt := d.GetNBT(int(bp.X), int(bp.Y), int(bp.Z))
		covered++
		switch ClassifyBlock(info.Name, nbt) {
		case CatWorkspaceNBT:
			// 每个工作区NBT位置：恰好归入一个 NBT 单元，且被所在普通单元的 Skipped 覆盖一次
			if nbtCount[p] != 1 {
				t.Errorf("工作区NBT位置 %v 出现在 %d 个 NBT 单元，应为 1", p, nbtCount[p])
			}
			if skipCount[p] != 1 {
				t.Errorf("工作区NBT位置 %v 被普通单元 Skipped 覆盖 %d 次，应为 1（§8.1）", p, skipCount[p])
			}
		case CatSign:
			if !signSet[p] {
				t.Errorf("告示牌位置 %v 不在任何告示牌单元", p)
			}
		default:
			c := [2]int{p[0] / ChunkSize, p[2] / ChunkSize}
			if n := normalCols[c]; n != 1 {
				t.Errorf("普通方块位置 %v 的列 %v 被覆盖 %d 次，应为 1", p, c, n)
			}
		}
	}
	if covered != res.TotalSolid {
		t.Errorf("覆盖方块 %d ≠ TotalSolid %d", covered, res.TotalSolid)
	}
}

// ── 海量 NBT 阈值（§4.3） ──

func TestMassiveNBTThreshold(t *testing.T) {
	d := mkData(64)
	// 12 个满箱子（工作区NBT）+ 20 个石头 → 12/32=37.5% ≥ 10% → 海量
	for i := 0; i < 12; i++ {
		setBlockAt(t, d, i*2, 1, 1, "minecraft:chest", map[string]any{
			"Items": []any{map[string]any{"Name": "minecraft:diamond", "Count": int32(1)}},
		})
	}
	for i := 0; i < 20; i++ {
		setBlockAt(t, d, 30+i, 1, 1, "minecraft:stone", nil)
	}

	res, err := SplitStructure(d, SplitOptions{NBTUnitTarget: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsMassiveNBT {
		t.Fatalf("应判定为海量 NBT（%d/%d）", res.NBTBlockCount, res.TotalSolid)
	}
	var nbtUnits int
	for _, u := range res.Units {
		if u.Kind == UnitNBT {
			nbtUnits++
		}
	}
	if nbtUnits != 3 { // 12 个 / 每单元 5 → 3 份并行
		t.Fatalf("海量 NBT 应拆成 3 个 NBT 单元，got %d", nbtUnits)
	}
}

func TestNonMassiveNBTSingleUnit(t *testing.T) {
	d := mkData(64)
	// 3 个满箱 + 100 个石头 → 3/103 ≈ 2.9% < 10% → 非海量，单个 NBT 单元
	for i := 0; i < 3; i++ {
		setBlockAt(t, d, i*2, 1, 1, "minecraft:chest", map[string]any{
			"Items": []any{map[string]any{"Name": "minecraft:diamond", "Count": int32(1)}},
		})
	}
	for i := 0; i < 100; i++ {
		setBlockAt(t, d, 10+i%40, 1, 2+i/40, "minecraft:stone", nil)
	}
	res, err := SplitStructure(d, SplitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsMassiveNBT {
		t.Fatal("不应判定为海量 NBT")
	}
	var nbtUnits int
	for _, u := range res.Units {
		if u.Kind == UnitNBT {
			nbtUnits++
			if len(u.Positions) != 3 {
				t.Fatalf("非海量 NBT 单元应含全部 3 个位置，got %d", len(u.Positions))
			}
		}
	}
	if nbtUnits != 1 {
		t.Fatalf("非海量 NBT 应只有 1 个单元，got %d", nbtUnits)
	}
}

// ── 普通单元按方块数降序（§10.3） ──

func TestLargeBuildingSubdividesIntoMultipleUnits(t *testing.T) {
	d := mkData(1024)
	// 单个连续岛：横跨 32 个区块列（z 方向），每列放一个方块保持"非空"
	for cx := 0; cx < 32; cx++ {
		for cz := 0; cz < 8; cz++ {
			setBlockAt(t, d, cx*16+1, 1, cz*16+1, "minecraft:stone", nil)
		}
	}
	res, err := SplitStructure(d, SplitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var normalUnits int
	for _, u := range res.Units {
		if u.Kind == UnitNormal {
			normalUnits++
		}
	}
	if normalUnits < 2 {
		t.Fatalf("32 列的岛应切出 ≥2 个普通单元，got %d", normalUnits)
	}
}

func TestNormalUnitsCoverAllColumns(t *testing.T) {
	d := mkData(128)
	// 岛 A：列 (0,0) 的 10 个方块
	for i := 0; i < 10; i++ {
		setBlockAt(t, d, i, 1, 0, "minecraft:stone", nil)
	}
	// 岛 B：列 (3,0)(4,0) 的 3 个方块
	for i := 0; i < 3; i++ {
		setBlockAt(t, d, 60+i*2, 1, 0, "minecraft:stone", nil)
	}
	res, err := SplitStructure(d, SplitOptions{BotCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	var normalUnits []*Unit
	for _, u := range res.Units {
		if u.Kind == UnitNormal {
			normalUnits = append(normalUnits, u)
		}
	}
	if len(normalUnits) != 3 {
		t.Fatalf("BotCount=3 应切 3 个普通大工作区，got %d", len(normalUnits))
	}
	// 所有非空区块列恰好覆盖一次（无重叠、无遗漏）
	seen := map[[2]int]int{}
	for _, u := range normalUnits {
		for _, c := range u.Chunks {
			seen[c]++
		}
	}
	// 非空区块列：x=0..9→列0, x=60,62,64→列3,3,4
	wantCols := map[[2]int]int{{0, 0}: 1, {3, 0}: 1, {4, 0}: 1}
	if len(seen) != len(wantCols) {
		t.Fatalf("应覆盖 %d 个非空区块列，got %d (%v)", len(wantCols), len(seen), seen)
	}
	for c := range wantCols {
		if seen[c] != 1 {
			t.Errorf("区块列 %v 应覆盖 1 次，got %d", c, seen[c])
		}
	}
}
