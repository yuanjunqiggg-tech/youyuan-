package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_biome_golden_test.go —— chunkbase 黄金基准（真值）一致率测试
//
// 【真值来源】D:/种子查询器自研/docs/algorithms/golden/golden-biomes.json
//   （已复制到 testdata/chunkbase-biomes.json，476KB）
//   它是从 chunkbase 的 Rust WASM 引擎导出的**事实记录**，不是我们的算法输出，
//   所以可以当作「游戏内真实群系」的近似真值。
//
// 【数据格式】踩过的坑：这**不是** RGB！每 3 字节一个像素：
//     data[3i+0] = biome id        （直接就是群系 id，不是颜色分量）
//     data[3i+1] = height 低字节
//     data[3i+2] = height 高字节    （实际恒 128，因为高度都在 32..94）
//   height = (lo | hi<<8) - 0x8000 → 该列**地表高度**（y）。
//   证据：参考实现自己的测试 D:/种子查询器自研/tests/biomes.test.mjs:14-24
//   就是按这个布局解码；拼出来的 32..94 与「深海海床~32、河流~55、森林~65」
//   完全吻合。若误当 RGB 做最近邻映射，一致率会直接掉到个位数。
//
// 【坐标换算】tile 是 32×32 像素，一像素 = 一个 biome cell（= 4 方块）。
//   tile.x/tile.z 就是 biome cell 坐标系的起点，四个 tile 位于
//   (0,0) (100,100) (-200,50) (300,-150)，互不重叠。
//   实测判别（TestSeedV2BiomeGoldenSweep 表①）：把 tile 坐标直接当 cell 坐标
//   用是 75.00%，乘 4 / 加 2 / 步长 4 分别只有 4.34% / 4.05% / 23.54%，
//   所以换算就是 1:1，别乘 4（参考实现也踩过这个 4 倍偏移）。
//
// 【基准线】改 depth 之前（Axes 的 1-83/160+off → d≈0.48，落进洞穴带）：
//   75.00%（36862/49152）。
//   改之后（biomeAt 把 depth 钉到参数表地表点 1.0）：99.98%（49140/49152）。
//   见 TestSeedV2BiomeGoldenTile。
// ═══════════════════════════════════════════════════════════════════════

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// yyCBBaselineRate 改 depth 基准之前的实测一致率（仅用于日志对比）。
const yyCBBaselineRate = 75.00

// yyCBMinRate 回归门槛：实测最优 99.98%，留 1% 余量。
const yyCBMinRate = 98.9

// ── 黄金数据结构 ──────────────────────────────────────────────────────

type yyCBTile struct {
	X    int     `json:"x"`
	Z    int     `json:"z"`
	W    int     `json:"w"`
	H    int     `json:"h"`
	Data []uint8 `json:"data"`
}

type yyCBCase struct {
	Version string     `json:"version"`
	Seed    string     `json:"seed"`
	Tiles   []yyCBTile `json:"tiles"`
}

type yyCBGolden struct {
	Note  string              `json:"note"`
	Cases map[string]yyCBCase `json:"cases"`
}

// yyCBDecodeTile 解出一像素一个的 (biomeID, heightMeters)。见文件头【数据格式】。
func yyCBDecodeTile(t *yyCBTile) (ids []int32, heights []int32) {
	n := t.W * t.H
	ids = make([]int32, n)
	heights = make([]int32, n)
	for i := 0; i < n; i++ {
		ids[i] = int32(t.Data[i*3])
		heights[i] = int32(uint16(t.Data[i*3+1])|uint16(t.Data[i*3+2])<<8) - 0x8000
	}
	return ids, heights
}

func yyCBLoad(t *testing.T) *yyCBGolden {
	t.Helper()
	p := filepath.Join("testdata", "chunkbase-biomes.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读不到 chunkbase 黄金基准 %s：%v", p, err)
	}
	var g yyCBGolden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatalf("chunkbase 黄金基准 JSON 解析失败：%v", err)
	}
	if len(g.Cases) == 0 {
		t.Fatal("chunkbase 黄金基准为空")
	}
	return &g
}

// yyCBVersionCode 版本字符串 → versionCode（与 yyV2BiomeTableName 对齐）。
func yyCBVersionCode(v string) int32 {
	switch v {
	case "1.21.4":
		return 102104
	case "1.20":
		return 102000 // 参考实现把 1.20.x 归进 btree20
	case "1.18":
		return 101802
	}
	return 102104
}

// ── 坐标映射候选（只用于判别，判别完就固定用 M1）────────────────────

type yyCBMapping func(tx, tz, i, j int) (cx, cz int)

var yyCBMappings = []struct {
	Name string
	Fn   yyCBMapping
}{
	// 实测 75.00%：tile 坐标就是 biome cell 坐标（像素 = cell）
	{"M1_cell=tile+i", func(tx, tz, i, j int) (int, int) { return tx + i, tz + j }},
	// 若像素=区块，cell 应为 区块号×4。实测 4.34% → 假设不成立
	{"M2_cell=(tile+i)*4", func(tx, tz, i, j int) (int, int) { return (tx + i) * 4, (tz + j) * 4 }},
	// 区块中心方块 (chunk*16+8)>>2 = chunk*4+2。实测 4.05%
	{"M3_cell=(tile+i)*4+2", func(tx, tz, i, j int) (int, int) { return (tx+i)*4 + 2, (tz+j)*4 + 2 }},
	// 像素步长 4 的 cell 采样。实测 23.54%
	{"M4_cell=tile+i*4", func(tx, tz, i, j int) (int, int) { return tx + i*4, tz + j*4 }},
}

// yyCBAxesAtY 复刻 Axes，但 depth 基准 y 可调（仅测量用，不参与生产）。
// 其它 5 轴公式与生产代码逐字一致，只有 "1.0 - y/160.0" 的 y 由调用方给，
// 所以扫出来的最优值可以直接搬回 Axes 的那个常量。
func yyCBAxesAtY(c *yyV2Climate, x, z, y float64) [6]int32 {
	px := x + yyV2RandSampleDoublePerlin(c.shift, x, 0, z)*4.0
	pz := z + yyV2RandSampleDoublePerlin(c.shift, z, x, 0)*4.0

	cont := yyV2RandSampleDoublePerlin(c.continentalness, px, 0, pz)
	ero := yyV2RandSampleDoublePerlin(c.erosion, px, 0, pz)
	w := yyV2RandSampleDoublePerlin(c.weirdness, px, 0, pz)

	npParam := [4]float64{
		cont,
		ero,
		-3.0 * (math.Abs(math.Abs(w)-0.6666667) - 0.33333334),
		w,
	}
	off := yyV2RandGetSpline(yyV2RandDepthSpline, npParam) + 0.015
	d := 1.0 - y/160.0 + off

	tp := yyV2RandSampleDoublePerlin(c.temperature, px, 0, pz)
	h := yyV2RandSampleDoublePerlin(c.vegetation, px, 0, pz)

	return [6]int32{
		int32(math.Trunc(10000.0 * tp)),
		int32(math.Trunc(10000.0 * h)),
		int32(math.Trunc(10000.0 * cont)),
		int32(math.Trunc(10000.0 * ero)),
		int32(math.Trunc(10000.0 * d)),
		int32(math.Trunc(10000.0 * w)),
	}
}

// ── 打分 ──────────────────────────────────────────────────────────────

type yyCBConfig struct {
	Mapping string
	// YMode:
	//   "prod"    生产路径（gen.biomeAt，即 Axes 现状）—— 回归断言用这个
	//   "axesY"   复刻 Axes 并指定 y —— 扫参数用
	//   "goldenY" y = 该列真实地表高度 —— 衡量「按地形高度采样」能到多少
	//   "point1"  depth 直接钉在参数表的地表点 1.0（对照实验）
	YMode string
	Y     float64
}

type yyCBWrong struct {
	Case   string
	TileX  int
	TileZ  int
	CellX  int
	CellZ  int
	BlockX int // cell*4
	BlockZ int
	Height int32
	Want   int32
	Got    int32
}

type yyCBResult struct {
	Hit         int
	Total       int
	CaveOut     int // 我们输出洞穴群系的像素数（地表图应为 0）
	WrongByWant map[int32]int
	WrongByGot  map[int32]int
	Examples    []yyCBWrong
	PerCase     map[string]float64
}

func (r *yyCBResult) Rate() float64 {
	if r.Total == 0 {
		return 0
	}
	return 100 * float64(r.Hit) / float64(r.Total)
}

// yyCBIsCave 洞穴群系 id（滴石洞穴/繁茂洞穴/深暗之域 + 1.21.9 三个新洞穴）
func yyCBIsCave(id int32) bool {
	switch id {
	case 174, 175, 183, 187, 188, 189:
		return true
	}
	return false
}

// yyCBRun 跑一遍全量黄金基准（12 case × 4 tile × 1024 像素 = 49152 点）。
func yyCBRun(t *testing.T, g *yyCBGolden, cfg yyCBConfig) *yyCBResult {
	t.Helper()
	var mapFn yyCBMapping
	for _, m := range yyCBMappings {
		if m.Name == cfg.Mapping {
			mapFn = m.Fn
		}
	}
	if mapFn == nil {
		t.Fatalf("未知映射 %q", cfg.Mapping)
	}

	res := &yyCBResult{
		WrongByWant: map[int32]int{},
		WrongByGot:  map[int32]int{},
		PerCase:     map[string]float64{},
	}

	keys := make([]string, 0, len(g.Cases))
	for k := range g.Cases {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		c := g.Cases[key]
		gen := yyV2NewBiomeGen(yyTestSeed(t, c.Seed), yyCBVersionCode(c.Version), false)
		if gen.climate == nil || gen.table == nil {
			t.Fatalf("%s: 引擎构造失败", key)
		}
		caseHit, caseTotal := 0, 0
		for ti := range c.Tiles {
			tile := &c.Tiles[ti]
			ids, heights := yyCBDecodeTile(tile)
			for j := 0; j < tile.H; j++ {
				for i := 0; i < tile.W; i++ {
					idx := j*tile.W + i
					want := ids[idx]
					cx, cz := mapFn(tile.X, tile.Z, i, j)

					var got int32
					switch cfg.YMode {
					case "prod":
						// 生产路径：和渲染器/单点查询走同一条 code path
						got = gen.biomeAt(cx, cz)
					case "axesY":
						np := yyCBAxesAtY(gen.climate, float64(cx), float64(cz), cfg.Y)
						got = yyV2ClimateToBiome(gen.table, &np)
					case "goldenY":
						np := yyCBAxesAtY(gen.climate, float64(cx), float64(cz), float64(heights[idx]))
						got = yyV2ClimateToBiome(gen.table, &np)
					case "point1":
						np := gen.climate.Axes(float64(cx), float64(cz))
						np[4] = 10000
						got = yyV2ClimateToBiome(gen.table, &np)
					default:
						t.Fatalf("未知 YMode %q", cfg.YMode)
					}

					caseTotal++
					res.Total++
					if yyCBIsCave(got) {
						res.CaveOut++
					}
					if got == want {
						caseHit++
						res.Hit++
						continue
					}
					res.WrongByWant[want]++
					res.WrongByGot[got]++
					if len(res.Examples) < 40 {
						res.Examples = append(res.Examples, yyCBWrong{
							Case: key, TileX: tile.X, TileZ: tile.Z,
							CellX: cx, CellZ: cz, BlockX: cx * 4, BlockZ: cz * 4,
							Height: heights[idx], Want: want, Got: got,
						})
					}
				}
			}
		}
		res.PerCase[key] = 100 * float64(caseHit) / float64(caseTotal)
	}
	return res
}

// ── 回归测试（默认就跑，约 1 秒）──────────────────────────────────────

// TestSeedV2BiomeGoldenTile 用 chunkbase 真值固化地表图一致率。
//
// 历史：depth 走 1-83/160+off ≈ 0.48 → 75.00%，且地表图大面积出现洞穴群系
// （seed=12345 ±512 方块：繁茂洞穴 40% + 深暗之域 12%）。
// 现在 biomeAt 把 depth 钉在参数表地表点 1.0 → 99.98%，洞穴群系 0 像素。
func TestSeedV2BiomeGoldenTile(t *testing.T) {
	g := yyCBLoad(t)
	r := yyCBRun(t, g, yyCBConfig{Mapping: "M1_cell=tile+i", YMode: "prod"})

	keys := make([]string, 0, len(r.PerCase))
	for k := range r.PerCase {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("  %-18s %6.2f%%", k, r.PerCase[k])
	}
	t.Logf("chunkbase 地表图一致率：%.2f%%（%d/%d，错 %d 点）｜改 depth 前基线 %.2f%%",
		r.Rate(), r.Hit, r.Total, r.Total-r.Hit, yyCBBaselineRate)
	t.Logf("地表图里输出为洞穴群系的像素数：%d（应为 0）", r.CaveOut)

	if r.CaveOut != 0 {
		t.Errorf("地表图出现 %d 个洞穴群系像素，depth 基准又被拉回洞穴带了", r.CaveOut)
	}
	if r.Rate() < yyCBMinRate {
		t.Errorf("chunkbase 一致率 %.2f%%，低于门槛 %.2f%%（实测最优 99.98%%）",
			r.Rate(), yyCBMinRate)
		for i, e := range r.Examples {
			if i >= 8 {
				break
			}
			t.Logf("  错例 %s tile(%d,%d) 方块(%d,%d) h=%d 真值=%d 我们=%d",
				e.Case, e.TileX, e.TileZ, e.BlockX, e.BlockZ, e.Height, e.Want, e.Got)
		}
	}
}

// ── 诊断：参数扫描（只在 CB_SWEEP=1 时跑）────────────────────────────
//
//	CB_SWEEP=1 go test -count=1 -run TestSeedV2BiomeGoldenSweep -v .
func TestSeedV2BiomeGoldenSweep(t *testing.T) {
	if os.Getenv("CB_SWEEP") == "" {
		t.Skip("设 CB_SWEEP=1 才跑参数扫描（约 15 秒）")
	}
	g := yyCBLoad(t)

	t.Log("=== ① 坐标映射判别（y=83，与改前生产代码一致）===")
	for _, m := range yyCBMappings {
		r := yyCBRun(t, g, yyCBConfig{Mapping: m.Name, YMode: "axesY", Y: 83})
		t.Logf("  %-24s %6.2f%%  (%d/%d)", m.Name, r.Rate(), r.Hit, r.Total)
	}

	t.Log("=== ② depth 基准 y 扫描（映射固定 M1）——历史证据：任何 y 都救不回来，因为洞穴带是连续的 ===")
	for _, y := range []float64{0, 40, 48, 55, 63, 70, 79, 83, 96, 112, 128, 144, 160} {
		r := yyCBRun(t, g, yyCBConfig{Mapping: "M1_cell=tile+i", YMode: "axesY", Y: y})
		t.Logf("  y=%-4.0f %6.2f%%  (错 %d)  d0=%.3f", y, r.Rate(), r.Total-r.Hit, 1-y/160)
	}

	t.Log("=== ③ 对照组（钉点 / 真实地表高度）===")
	r := yyCBRun(t, g, yyCBConfig{Mapping: "M1_cell=tile+i", YMode: "goldenY"})
	t.Logf("  %-24s %6.2f%%  (错 %d)  ← 按地形高度采样也不行", "y=每列真实地表高度", r.Rate(), r.Total-r.Hit)
	r = yyCBRun(t, g, yyCBConfig{Mapping: "M1_cell=tile+i", YMode: "point0"})
	t.Logf("  %-24s %6.2f%%  (错 %d)", "depth 钉 0.0", r.Rate(), r.Total-r.Hit)
	r = yyCBRun(t, g, yyCBConfig{Mapping: "M1_cell=tile+i", YMode: "point1"})
	t.Logf("  %-24s %6.2f%%  (错 %d)  ← 采纳（= 生产 biomeAt 的做法）", "depth 钉 1.0", r.Rate(), r.Total-r.Hit)
}
