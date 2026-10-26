package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_biome.go —— 1.18+ 生物群系引擎（climate 六轴 → 查表 → biome id）
//
// 【归属】agent: seed-biome
// 【来源】D:/种子查询器自研/src/biomegen.mjs + src/biomes.mjs
// 【依赖】yyV2NewClimate / yyV2Climate.Axes / yyV2VoronoiSHA /
//         yyV2VoronoiAccess3D（seedv2_rand.go）、yyV2BTreeByName（seedv2_btree.go）
//
// 【必须原样复刻的 6 个坑】（参考实现踩了半天，详见
//  docs/algorithms/BIOME_ENGINE.md 第二节，照抄注释即可）：
//   1. depth 公式里的 y 恒为 0（不是查询高度）—— 在 seedv2_rand.go 的 Axes 里；
//      另外**地表查询还要把 depth 钉到参数表的地表点 1.0**（见 biomeAt 上方
//      yyV2SurfaceDepthNP），否则地表图会大面积变洞穴群系
//   2. 查表前 np 已乘 10000 取整 —— 同上
//   3. resultingNode 的初始 ds 是「无穷大」（cubiomes 的 uint64(-1)）
//   4. 递归时传**父层 ds**，不是 dsInner
//   5. steps 跳层逻辑：idx+step 越界就换下一层步长
//   6. 叶节点 biome id = (node>>48)&0xff；子节点下标 = node>>48
//
// 【坐标换算】参考实现踩过 4 倍偏移（少乘 4 会让海洋神殿落在陆地上）：
//   BiomeAtBlockNoVoronoi = 1:1 采样，直接 ax = bx, az = bz
//   BiomeAtBlock          = 先 voronoiAccess3D 到 1:4 格点，再按**格点坐标**采样
//   绝不要再手乘 4 / 手除 4，voronoiAccess3D 已经把方块坐标算成格点坐标了。
//
// 【性能】BiomeAtBlock 是渲染热路径（256×256 瓦片 = 65536 次）。
//   climate / sha / table 全部在 yyV2NewBiomeGen 里构造一次，查询期间零分配。
// ═══════════════════════════════════════════════════════════════════════

import (
	"fmt"
	"log"
	"math"
	"sync"
)

// yyV2BiomeGen 一个种子 + 版本下的生物群系引擎。构造一次可反复查（不要每次查都重建）。
type yyV2BiomeGen struct {
	gen       int64
	versionCD int32
	large     bool

	// 构造期一次性准备好，查询期只读
	climate *yyV2Climate
	sha     uint64
	table   *yyV2BTree
}

// yyV2BiomeTableName 版本码 → btree 表名。
// 1.21.4+ 才有 pale_garden（btree21wd）；26.x 的 265000 也落进 21wd 档。
func yyV2BiomeTableName(versionCode int32) string {
	switch {
	case versionCode >= 102104:
		return "btree21wd"
	case versionCode >= 102000:
		return "btree20"
	case versionCode >= 101904:
		return "btree19"
	case versionCode >= 101902:
		return "btree192"
	default:
		return "btree18"
	}
}

// yyV2BiomeFallbackOnce 保证「表缺失降级」只 log 一次，别在渲染循环里刷屏。
var yyV2BiomeFallbackOnce sync.Once

// yyV2NewBiomeGen 构造引擎。versionCode 用于选表：
//
//	>=102104 → btree21wd ；>=102000 → btree20 ；>=101904 → btree19 ；
//	>=101902 → btree192 ；其余 → btree18
//
// 表取不到时降级到 btree18 并 log 一次（不能静默返回空表，否则整张图变同一种群系）。
func yyV2NewBiomeGen(seed int64, versionCode int32, large bool) *yyV2BiomeGen {
	g := &yyV2BiomeGen{gen: seed, versionCD: versionCode, large: large}

	want := yyV2BiomeTableName(versionCode)
	g.table = yyV2BTreeByName(want)
	if g.table == nil {
		g.table = yyV2BTreeByName("btree18")
		yyV2BiomeFallbackOnce.Do(func() {
			log.Printf("[seedv2/biome] 找不到参数表 %q（versionCode=%d），已降级到 btree18；"+
				"该种子的群系可能与游戏内不一致", want, versionCode)
		})
	}

	g.climate = yyV2NewClimate(seed, large)
	g.sha = yyV2VoronoiSHA(seed)
	return g
}

// BiomeAtBlock 方块坐标的生物群系 id（含 voronoi 1:4 缩放）。
// 对应参考实现 getBiomeIdVoronoi：voronoiAccess3D → 格点坐标 → 气候采样。
// 注意 voronoiAccess3D 返回的是 1:4 **格点坐标**（不是方块坐标），直接喂 Axes，
// 不要再 ×4 或 ÷4。
// 返回的是**该列地表的群系**：by 只参与 voronoi 选格，不参与 depth
// （depth 钉在参数表地表点，见 yyV2SurfaceDepthNP）。
func (g *yyV2BiomeGen) BiomeAtBlock(bx, by, bz int) int32 {
	gx, _, gz := yyV2VoronoiAccess3D(g.sha, bx, by, bz)
	return g.biomeAt(gx, gz)
}

// BiomeAtBlockNoVoronoi 方块坐标的生物群系 id（1:1 采样，无 voronoi）。
// 只用于诊断/对拍，正常渲染走 BiomeAtBlock。
func (g *yyV2BiomeGen) BiomeAtBlockNoVoronoi(bx, bz int) int32 {
	return g.biomeAt(bx, bz)
}

// yyV2SurfaceBlockY 地表查询用的采样高度。
// 0 与参考实现 biomegen.mjs 的 sampleClimate(x, z) 一致；voronoi 是 3D 的，
// y 换一层会换到另一个格子，导致同一位置和参考工具对不上。
const yyV2SurfaceBlockY = 0

// yyV2ChunkCenterOffset 区块内采样点的偏移。
//
// 这里踩过一个坑：区块中心是 8，但 8 % 4 == 0，而生物群系是 1:4 的 voronoi 格，
// 偏移 0 正好落在**格角**上——四个格到角点等距，谁抢到看浮点误差，
// 相邻格之间会出现 4 方块宽的斑点。格中心是 2（≡2 mod 4），也就是 8+2=10。
// 实测（chunkbase 黄金基准 49152 点，走 BiomeAtBlock）：
//   偏移 +0 → 96.26% ；+1 → 98.49% ；+2 → 99.77% ；+3 → 98.60%
const yyV2ChunkCenterOffset = 2

// BiomeAtChunk 区块坐标的生物群系 id（取区块内 voronoi 格中心，不是区块角）。
func (g *yyV2BiomeGen) BiomeAtChunk(cx, cz int) int32 {
	return g.BiomeAtBlock(cx*16+yyV2ChunkCenterOffset, yyV2SurfaceBlockY, cz*16+yyV2ChunkCenterOffset)
}

// ── 地表 depth 钉点（2026-10-26 校准，seed-biome-calib）─────────────────
//
// 【问题】地表图（渲染 / BiomeAtChunk / 结构 ocean 判定）问的是「这一列
// 地表的群系」，但 depth 轴原来走的是 Axes 里 `1 - 83/160 + off ≈ 0.48`，
// 落进了**洞穴带**，于是地表大片变繁茂洞穴/深暗之域：
//
//	seed=12345 ±512 方块 289 点：繁茂洞穴 117（40%）+ 深暗之域 34（12%）。
//
// 【参数表结构】每个群系的 depth 盒子只有三类（btree21wd/btree18 转储实测）：
//
//	地表群系   point(0.0) 与 point(1.0) —— 同一个群系成对出现两片叶子
//	洞穴群系   span(0.2, 0.9)（滴石洞穴 174 / 繁茂洞穴 175）
//	深暗之域   point(1.0) 与 point(1.1)（183）
//
// 查表是「到参数盒的最近平方距离」，所以 depth 落在 0.2~0.9 中间时，
// 到洞穴带的距离比对地表点的距离更近，地表就被洞穴群系抢走。
//
// 【做法】地表查询把 depth 直接钉在参数表的地表点 1.0，**不用** 1-y/160：
// 任何 y 都只会得到 1-y/160+off 这个带抖动的值，抖到 0.2~0.9 就进洞。
//
// 【实测】chunkbase 黄金基准（12 case × 4 tile × 1024 = 49152 点）：
//
//	depth=1-y/160+off, y=83（原实现） 75.00%  错 12290
//	y=48 → 86.15%    y=64 → 76.35%    y=112 → 85.65%   y=128 → 95.54%
//	y=0（d=1+off）  99.98%  ✗ 但 7 种子洞穴审计 9.5% 深暗之域 → 否决
//	钉 0.0          99.97%  错 17，洞穴 0/65340
//	钉 1.0          99.98%  错 12，洞穴 0/65340   ← 采纳
//
// 「用该列真实地表高度当 y」只有 76.70%，说明 chunkbase 的地表图本来
// 就不是按地形高度采样的，所以别把 BiomeAtBlock 的 by 传进来当 depth。
// 剩下 12 个错点全在海岸线/针叶林边界，属于单格边界分歧，不是系统误差。
const yyV2SurfaceDepthNP int32 = 10000

// biomeAt 气候采样 + 查表。整数坐标转 float64 后交给 Axes。
func (g *yyV2BiomeGen) biomeAt(x, z int) int32 {
	if g.climate == nil || g.table == nil {
		return 0
	}
	np := g.climate.Axes(float64(x), float64(z))
	np[4] = yyV2SurfaceDepthNP // depth 钉地表点，见上
	return yyV2ClimateToBiome(g.table, &np)
}

// ── 参数表查表（cubiomes climateToBiome 的等价实现）─────────────────────
//
// 【2026-10-26 性能改造】原来每个节点都要现算 6 次
// 「移位 → 掩码 → 参数下标越界判断 → 切片取值 → int32 转 float64」，
// profile 里 yyV2BTreeDist 一个人吃掉 50% CPU（单点 18.7µs，一张 256×256
// 瓦片要 1.2 秒）。改造做法：
//   1. 每张表第一次用时把 44195 个节点的 6 个参数下标预解成 [6]uint16（一次性）
//   2. ds 全程用 int64 累加，彻底不碰 float64（单轴最大偏差约 12000，
//      平方 1.44e8，6 轴最多 8.6e8，int64 绰绰有余）
//   3. 「无穷大」用 math.MaxInt64 表示，和原来 uint64(-1) 语义一致
// 算法本身一个字符没改，改的只是「怎么读那张表」。

// yyV2BTreeIndex 查表用的预解索引（每张表只建一次，只读）。
type yyV2BTreeIndex struct {
	// box[node] = 6 个参数盒，按 lo,hi 交错存在 int16 里
	// （参数值域 -12000~10000，int16 装得下；一张表 44195 节点 ≈ 1.03MB）
	box   [][12]int16
	steps []int32
	nodes []uint64
	order int
	// bad 只在 .dat 里出现非法参数下标时为 true（loader 正常时不会发生）
	bad []bool
}

var yyV2BTreeIndexCache sync.Map // *yyV2BTree -> *yyV2BTreeIndex

// yyV2BTreeIndexOf 取（必要时构建）某张表的预解索引。
func yyV2BTreeIndexOf(bt *yyV2BTree) *yyV2BTreeIndex {
	if bt == nil || len(bt.Nodes) == 0 {
		return nil
	}
	if v, ok := yyV2BTreeIndexCache.Load(bt); ok {
		return v.(*yyV2BTreeIndex)
	}
	built := &yyV2BTreeIndex{
		box:   make([][12]int16, len(bt.Nodes)),
		steps: bt.Steps,
		nodes: bt.Nodes,
		order: bt.Order,
	}
	for i, node := range bt.Nodes {
		var row [12]int16
		bad := false
		for k := 0; k < 6; k++ {
			pidx := int((node >> (8 * uint(k))) & 0xFF)
			if pidx >= len(bt.Param) {
				bad = true
				break
			}
			row[k*2] = int16(bt.Param[pidx][0])
			row[k*2+1] = int16(bt.Param[pidx][1])
		}
		if bad {
			if built.bad == nil {
				built.bad = make([]bool, len(bt.Nodes))
			}
			built.bad[i] = true
			continue
		}
		built.box[i] = row
	}
	actual, _ := yyV2BTreeIndexCache.LoadOrStore(bt, built)
	return actual.(*yyV2BTreeIndex)
}

// yyV2BiomeNodes 返回表节点数（参考实现里 bt.len 未定义时用 nodes.length 兜底，
// 这里 Node 数就是唯一真相）。
func yyV2BiomeNodes(bt *yyV2BTree) int { return len(bt.Nodes) }

// dist 对应参考实现 npDist：到节点参数盒的平方距离。
// 节点越界 / 参数下标越界一律返回 MaxInt64（等价 JS 的 Infinity），不 panic。
func (ix *yyV2BTreeIndex) dist(np *[6]int32, idx int) int64 {
	if idx < 0 || idx >= len(ix.box) {
		return math.MaxInt64
	}
	if ix.bad != nil && ix.bad[idx] {
		return math.MaxInt64
	}
	b := &ix.box[idx]
	var ds int64
	for k := 0; k < 6; k++ {
		v := np[k]
		lo := int32(b[k*2])
		hi := int32(b[k*2+1])
		var d int32
		if a := v - hi; a > 0 {
			d = a
		} else if c := lo - v; c > 0 {
			d = c
		}
		ds += int64(d) * int64(d)
	}
	return ds
}

// yyV2BTreeDist 保留旧名字，内部走预解索引（供诊断/测试用）。
func yyV2BTreeDist(bt *yyV2BTree, np *[6]int32, idx int) int64 {
	ix := yyV2BTreeIndexOf(bt)
	if ix == nil {
		return math.MaxInt64
	}
	return ix.dist(np, idx)
}

// resultingNode 对应参考实现 resultingNode（cubiomes 同名函数）。
//
// ds 是**父层**的距离：cubiomes 递归时故意传父层的 ds（历史 bug #4：传 dsInner
// 会把子层的 best 污染成 0，查表结果全错）。alt 是上一层已选中的叶子。
func (ix *yyV2BTreeIndex) resultingNode(np *[6]int32, idx, alt int, ds int64, depth int) int {
	btLen := len(ix.nodes)
	if depth >= len(ix.steps) || ix.steps[depth] == 0 {
		return idx
	}
	// 当前层步长；若 idx+step 越界则往下一层借步长（cubiomes steps 跳层逻辑）
	step := int(ix.steps[depth])
	d := depth + 1
	for idx+step >= btLen && d < len(ix.steps) {
		step = int(ix.steps[d])
		d++
	}
	if idx < 0 || idx >= btLen {
		return alt
	}
	inner := int(ix.nodes[idx] >> 48)
	leaf := alt
	best := ds
	for i, cnt := 0, ix.order; i < cnt; i++ {
		if inner >= 0 && inner < btLen {
			dsInner := ix.dist(np, inner)
			if dsInner < best {
				// 递归传父层 ds（不是 dsInner）
				leaf2 := ix.resultingNode(np, inner, leaf, ds, depth+1)
				dsLeaf2 := dsInner
				if inner != leaf2 {
					dsLeaf2 = ix.dist(np, leaf2)
				}
				if dsLeaf2 < best {
					best = dsLeaf2
					leaf = leaf2
				}
			}
		}
		inner += step
		if inner >= btLen {
			break
		}
	}
	return leaf
}

// yyV2ResultingNode 兼容旧签名（float64 无穷大 → int64）。
func yyV2ResultingNode(bt *yyV2BTree, np *[6]int32, idx, alt int, ds float64, depth int) int {
	ix := yyV2BTreeIndexOf(bt)
	if ix == nil {
		return alt
	}
	var d int64 = math.MaxInt64
	if !math.IsInf(ds, 1) && !math.IsNaN(ds) {
		d = int64(ds)
	}
	return ix.resultingNode(np, idx, alt, d, depth)
}

// yyV2ClimateToBiome 与 cubiomes climateToBiome 等价：返回 biome id。
//
// 初始 ds 必须是「无穷大」（cubiomes 用 uint64(-1)，JS 用 Infinity，这里用
// math.MaxInt64）：传 -1 会让 dsInner < best 永假，leaf 永不更新 —— 历史 bug #3。
func yyV2ClimateToBiome(bt *yyV2BTree, np *[6]int32) int32 {
	ix := yyV2BTreeIndexOf(bt)
	if ix == nil {
		return 0
	}
	idx := ix.resultingNode(np, 0, 0, math.MaxInt64, 0)
	if idx < 0 || idx >= len(ix.nodes) {
		return 0
	}
	return int32((ix.nodes[idx] >> 48) & 0xFF)
}

// ── 群系表（id → key / 中文名 / 渲染色）──────────────────────────────
//
// 67 条，逐条搬自参考实现 src/biomes.mjs 的 BIOME_BY_ID（id 与 RGB 一位不改）。
// 中文名按 MC 官方简体译名；187~189 是 1.21.9 之后的新洞穴群系，官方尚无
// 稳定译名，这里按字面意译。
//
// 颜色纪律：这套 RGB 是「看地图」的配色（海洋深浅可辨、雪原纯白、沙漠黄、
// 恶地橙红），**不是**原版 grass 色。别改成按温度插值的方案，整张图会发灰。

type yyV2BiomeDef struct {
	Key  string
	Name string
	R    uint8
	G    uint8
	B    uint8
}

// yyV2BiomeDefs 生物群系静态定义表。
var yyV2BiomeDefs = map[int32]yyV2BiomeDef{
	0:   {"ocean", "海洋", 0, 0, 112},
	1:   {"plains", "平原", 141, 179, 96},
	2:   {"desert", "沙漠", 250, 148, 24},
	3:   {"windswept_hills", "风袭丘陵", 96, 96, 96},
	4:   {"forest", "森林", 5, 102, 33},
	5:   {"taiga", "针叶林", 11, 102, 89},
	6:   {"swamp", "沼泽", 7, 249, 178},
	7:   {"river", "河流", 0, 0, 255},
	8:   {"nether_wastes", "下界荒地", 191, 59, 59},
	9:   {"the_end", "末地", 128, 128, 255},
	10:  {"frozen_ocean", "冻洋", 112, 112, 214},
	11:  {"frozen_river", "冻河", 160, 160, 255},
	12:  {"snowy_plains", "雪原", 255, 255, 255},
	14:  {"mushroom_fields", "蘑菇岛", 255, 0, 255},
	16:  {"beach", "沙滩", 250, 222, 85},
	18:  {"windswept_forest", "风袭森林", 34, 85, 28},
	21:  {"jungle", "丛林", 83, 123, 9},
	23:  {"sparse_jungle", "稀疏丛林", 98, 139, 23},
	24:  {"deep_ocean", "深海", 0, 0, 48},
	25:  {"stony_shore", "石岸", 162, 162, 132},
	26:  {"snowy_beach", "积雪的沙滩", 250, 240, 192},
	27:  {"birch_forest", "桦木森林", 48, 116, 68},
	29:  {"dark_forest", "黑森林", 64, 81, 26},
	30:  {"snowy_taiga", "积雪的针叶林", 49, 85, 74},
	32:  {"old_growth_pine_taiga", "原始松木针叶林", 89, 102, 81},
	35:  {"savanna", "热带草原", 189, 178, 95},
	36:  {"savanna_plateau", "热带高原", 167, 157, 100},
	37:  {"badlands", "恶地", 217, 69, 21},
	38:  {"wooded_badlands", "疏林恶地", 176, 151, 101},
	40:  {"small_end_islands", "末地小型岛屿", 0, 0, 42},
	41:  {"end_midlands", "末地内陆", 235, 248, 182},
	42:  {"end_highlands", "末地高地", 195, 189, 137},
	43:  {"end_barrens", "末地荒地", 144, 144, 114},
	44:  {"warm_ocean", "暖水海洋", 0, 0, 172},
	45:  {"lukewarm_ocean", "温水海洋", 0, 0, 144},
	46:  {"cold_ocean", "冷水海洋", 32, 32, 112},
	47:  {"deep_warm_ocean", "暖水深海", 0, 0, 80},
	48:  {"deep_lukewarm_ocean", "温水深海", 0, 0, 64},
	49:  {"deep_cold_ocean", "冷水深海", 32, 32, 56},
	50:  {"deep_frozen_ocean", "冰冻深海", 64, 64, 144},
	131: {"windswept_gravelly_hills", "风袭沙砾丘陵", 136, 136, 136},
	132: {"flower_forest", "繁花森林", 45, 142, 73},
	140: {"ice_spikes", "冰刺之地", 180, 220, 220},
	155: {"old_growth_birch_forest", "原始桦木森林", 88, 156, 108},
	160: {"old_growth_spruce_taiga", "原始云杉针叶林", 129, 142, 121},
	163: {"windswept_savanna", "风袭热带草原", 229, 218, 135},
	165: {"eroded_badlands", "侵蚀恶地", 255, 109, 61},
	168: {"bamboo_jungle", "竹林", 118, 142, 20},
	170: {"soul_sand_valley", "灵魂沙峡谷", 94, 56, 48},
	171: {"crimson_forest", "绯红森林", 221, 8, 8},
	172: {"warped_forest", "诡异森林", 73, 144, 123},
	173: {"basalt_deltas", "玄武岩三角洲", 64, 54, 54},
	174: {"dripstone_caves", "溶洞", 193, 165, 143},
	175: {"lush_caves", "繁茂洞穴", 223, 150, 52},
	177: {"meadow", "草甸", 140, 164, 112},
	178: {"grove", "雪林", 146, 178, 160},
	179: {"snowy_slopes", "积雪山坡", 218, 241, 241},
	180: {"frozen_peaks", "冰封山峰", 205, 215, 255},
	181: {"jagged_peaks", "尖峭山峰", 186, 188, 182},
	182: {"stony_peaks", "裸岩山峰", 209, 209, 209},
	183: {"deep_dark", "深暗之域", 0, 0, 0},
	184: {"mangrove_swamp", "红树林沼泽", 36, 196, 142},
	185: {"cherry_grove", "樱花树林", 247, 185, 220},
	186: {"pale_garden", "苍白之园", 108, 111, 150},
	187: {"sulfur_caves", "硫磺洞穴", 200, 200, 40},
	188: {"dappled_forest", "斑驳森林", 154, 63, 53},
	189: {"ice_caves", "冰洞", 140, 209, 255},
}

// yyV2BiomeCount 已登记群系数（自检/报告用，测试会校验 == 67）。
const yyV2BiomeCount = 67

// yyV2BiomeUnknownKeyFmt 未知 id 的 key 模板：带 id 方便排查（不用裸 "minecraft:unknown"，
// 否则日志里看不出到底是哪个 id 缺表）。
const yyV2BiomeUnknownKeyFmt = "minecraft:unknown_%d"

// yyV2BiomeName 生物群系 id → 中文名（未知 id 返回 "未知(N)"）。
func yyV2BiomeName(id int32) string {
	if d, ok := yyV2BiomeDefs[id]; ok {
		return d.Name
	}
	return fmt.Sprintf("未知(%d)", id)
}

// yyV2BiomeKey 生物群系 id → 英文 key（如 minecraft:plains），用于插件/MCP 对齐。
// 未知 id 返回 "minecraft:unknown_N"。
func yyV2BiomeKey(id int32) string {
	if d, ok := yyV2BiomeDefs[id]; ok {
		return "minecraft:" + d.Key
	}
	return fmt.Sprintf(yyV2BiomeUnknownKeyFmt, id)
}

// yyV2BiomeColor 生物群系 id → 渲染颜色（RGB）。未知 id 给中性灰 (128,128,128)，
// 在地图上能一眼看出「这里没登记」。
func yyV2BiomeColor(id int32) (uint8, uint8, uint8) {
	if d, ok := yyV2BiomeDefs[id]; ok {
		return d.R, d.G, d.B
	}
	return 128, 128, 128
}
