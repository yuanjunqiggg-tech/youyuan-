package main

// seedv2_render_test.go —— 瓦片渲染 / 图标 / 摘要的自检
// 跑法：go test -count=1 -run TestSeedV2Render .

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func yyV2TestOpt(seed int64, dim string, cx, cz, w, h, bpp int) yyV2RenderOptions {
	return yyV2RenderOptions{
		Seed: seed, VersionCode: 102104, Edition: "java", Dimension: dim,
		CX: cx, CZ: cz, W: w, H: h, BlocksPerPixel: bpp, MaxIcons: 320,
	}
}

func yyV2TestDecode(t *testing.T, raw []byte) (int, int, func(x, y int) color.NRGBA) {
	t.Helper()
	if len(raw) == 0 {
		t.Fatal("PNG 字节为空")
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("PNG 解码失败: %v", err)
	}
	b := img.Bounds()
	get := func(x, y int) color.NRGBA {
		return color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
	}
	return b.Dx(), b.Dy(), get
}

func TestSeedV2RenderPNGDecode(t *testing.T) {
	for _, dim := range []string{yyV2DimOverworld, yyV2DimNether, yyV2DimEnd} {
		opt := yyV2TestOpt(12345, dim, -3, 7, 96, 64, 4)
		raw, err := yyV2RenderTilePNG(opt)
		if err != nil {
			t.Fatalf("%s: 渲染失败: %v", dim, err)
		}
		w, h, get := yyV2TestDecode(t, raw)
		if w != opt.W || h != opt.H {
			t.Fatalf("%s: 尺寸 %dx%d，期望 %dx%d", dim, w, h, opt.W, opt.H)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if a := get(x, y).A; a != 255 {
					t.Fatalf("%s: 像素 (%d,%d) alpha=%d，输出必须完全不透明", dim, x, y, a)
				}
			}
		}
	}
}

func TestSeedV2RenderSeedVariation(t *testing.T) {
	seeds := []int64{1, 12345, -987654321}
	for _, dim := range []string{yyV2DimOverworld, yyV2DimNether} {
		seen := make(map[string]int64, len(seeds))
		for _, seed := range seeds {
			raw, err := yyV2RenderTilePNG(yyV2TestOpt(seed, dim, 0, 0, 256, 256, 4))
			if err != nil {
				t.Fatalf("%s seed=%d: %v", dim, seed, err)
			}
			if len(raw) <= 3072 {
				t.Fatalf("%s seed=%d: PNG 只有 %d 字节，期望 >3KB", dim, seed, len(raw))
			}
			if prev, dup := seen[string(raw)]; dup {
				t.Fatalf("%s: seed %d 与 %d 输出字节完全相同", dim, prev, seed)
			}
			seen[string(raw)] = seed
		}
	}
}

// TestSeedV2RenderLayers 结构层必须画得出图标，地形层必须一个图标像素都没有。
func TestSeedV2RenderLayers(t *testing.T) {
	pts := yyV2ScanStructures(12345, "village", 102104, yyV2EditionJava, -96, -96, 96, 96, 50)
	if len(pts) == 0 {
		t.Fatal("种子 12345 在 ±96 区块内找不到村庄，测试前提不成立")
	}
	v := pts[0]
	opt := yyV2TestOpt(12345, yyV2DimOverworld, v.X-16, v.Z-16, 256, 256, 4)
	opt.StructureIDs = []string{"village"}

	info, ok := yyV2StructureInfoByID("village", opt.VersionCode, opt.Edition)
	if !ok {
		t.Fatal("village 元信息缺失")
	}
	iconColor := color.NRGBA{R: info.Color[0], G: info.Color[1], B: info.Color[2], A: 255}

	countIcon := func(layer string) int {
		o := opt
		o.Layer = layer
		raw, err := yyV2RenderTilePNG(o)
		if err != nil {
			t.Fatalf("layer=%s: %v", layer, err)
		}
		w, h, get := yyV2TestDecode(t, raw)
		n := 0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if get(x, y) == iconColor {
					n++
				}
			}
		}
		return n
	}

	if n := countIcon("terrain"); n != 0 {
		t.Fatalf("layer=terrain 出现了 %d 个图标主色像素", n)
	}
	if n := countIcon("structures"); n < 30 {
		t.Fatalf("layer=structures 只检出 %d 个图标主色像素，图标没画出来", n)
	}
	if n := countIcon(""); n < 30 {
		t.Fatalf("layer 为空时只检出 %d 个图标主色像素", n)
	}
}

func TestSeedV2StructureIconSVG(t *testing.T) {
	ids := make([]string, 0, len(yyV2PlaceMetas))
	for _, m := range yyV2PlaceMetas {
		ids = append(ids, m.ID)
	}
	if len(ids) != 28 {
		t.Fatalf("结构数 %d，期望 28", len(ids))
	}
	for _, id := range ids {
		svg := yyV2StructureIconSVG(id, 24)
		if len(svg) <= 120 {
			t.Fatalf("%s: SVG 只有 %d 字节", id, len(svg))
		}
		if len(svg) < 5 || svg[:4] != "<svg" {
			t.Fatalf("%s: SVG 未以 <svg 开头: %.20q", id, svg)
		}
		if svg[len(svg)-6:] != "</svg>" {
			t.Fatalf("%s: SVG 未以 </svg> 结尾", id)
		}
		if !bytes.Contains([]byte(svg), []byte(`viewBox="0 0 24 24"`)) {
			t.Fatalf("%s: SVG 缺 viewBox", id)
		}
		for _, banned := range []string{"<image", "font-family", "xlink:href", "<use"} {
			if bytes.Contains([]byte(svg), []byte(banned)) {
				t.Fatalf("%s: SVG 引用了外部资源 %q", id, banned)
			}
		}
	}
	// 未知 id 必须是通用圆点，不能是空串
	generic := yyV2StructureIconSVG("no-such-structure", 16)
	if len(generic) <= 120 || generic[:4] != "<svg" {
		t.Fatalf("未知 id 的兜底图标异常: %.40q", generic)
	}
}

func TestSeedV2SeedMapSummary(t *testing.T) {
	for _, dim := range []string{yyV2DimOverworld, yyV2DimNether, yyV2DimEnd} {
		m := yyV2SeedMapSummary(12345, 102104, "java", dim, 0, 0, 64)
		if m == nil {
			t.Fatalf("%s: 摘要为 nil", dim)
		}
		if s, _ := m["terrain"].(string); s == "" {
			t.Fatalf("%s: terrain 为空", dim)
		}
		top, ok := m["biome_top"].([]map[string]any)
		if !ok || len(top) == 0 {
			t.Fatalf("%s: biome_top 缺失或为空: %#v", dim, m["biome_top"])
		}
		nearest, ok := m["nearest"].([]map[string]any)
		if !ok {
			t.Fatalf("%s: nearest 类型不对: %#v", dim, m["nearest"])
		}
		if len(nearest) > 5 {
			t.Fatalf("%s: nearest 返回 %d 条，上限 5", dim, len(nearest))
		}
		for _, it := range nearest {
			for _, k := range []string{"id", "name", "x", "z", "dist_chunks", "dist_blocks"} {
				if _, ok := it[k]; !ok {
					t.Fatalf("%s: nearest 条目缺字段 %q: %#v", dim, k, it)
				}
			}
		}
		t.Logf("%s 摘要: terrain=%v top=%v nearest=%d 条", dim, m["terrain"], top[0], len(nearest))
	}
}

func TestSeedV2RenderPerf(t *testing.T) {
	opt := yyV2TestOpt(12345, yyV2DimOverworld, 0, 0, 256, 256, 4)
	// 预热（建表 / 首次 log 降级）
	if _, err := yyV2RenderTilePNG(opt); err != nil {
		t.Fatal(err)
	}
	const runs = 3
	var total time.Duration
	for i := 0; i < runs; i++ {
		start := time.Now()
		if _, err := yyV2RenderTilePNG(opt); err != nil {
			t.Fatal(err)
		}
		total += time.Since(start)
	}
	avg := total / runs
	t.Logf("256×256 bpp=4 平均耗时 %v（%d 次共 %v）", avg, runs, total)
	if avg > 200*time.Millisecond {
		t.Fatalf("单瓦片耗时 %v，超过 200ms 上限", avg)
	}
}

// TestSeedV2RenderTileEndpoint 端到端：路由层 → 渲染 → PNG，必须 200 且带缓存头。
func TestSeedV2RenderTileEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	yySeedV2Routes(mux)
	for _, q := range []string{
		"/api/seed/v2/tile.png?seed=12345&cx=0&cz=0&w=128&h=128&bpp=4",
		"/api/seed/v2/tile.png?seed=12345&cx=0&cz=0&w=64&h=64&bpp=8&dim=nether&layer=terrain",
		"/api/seed/v2/tile.png?seed=-7&cx=-20&cz=13&w=64&h=64&bpp=2&dim=end&layer=structures",
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, q, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s → %d（%s）", q, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("%s Content-Type = %q", q, ct)
		}
		if rec.Header().Get("Cache-Control") == "" {
			t.Fatalf("%s 缺 Cache-Control", q)
		}
		raw := rec.Body.Bytes()
		if len(raw) < 200 || raw[0] != 0x89 || string(raw[1:4]) != "PNG" {
			t.Fatalf("%s 返回的不是 PNG（%d 字节）", q, len(raw))
		}
	}
}

// TestSeedV2RenderWriteSamples 生成三张样张（默认跳过，避免测试污染工作区）。
// 刷新样张：YYV2_SAMPLES=1 go test -count=1 -run TestSeedV2RenderWriteSamples .
func TestSeedV2RenderWriteSamples(t *testing.T) {
	dir := os.Getenv("YYV2_SAMPLES_DIR")
	if dir == "" {
		t.Skip("未设置 YYV2_SAMPLES_DIR，跳过样张写出")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dim := range []string{yyV2DimOverworld, yyV2DimNether, yyV2DimEnd} {
		raw, err := yyV2RenderTilePNG(yyV2TestOpt(12345, dim, 0, 0, 256, 256, 4))
		if err != nil {
			t.Fatal(err)
		}
		name := dir + "/tile-" + dim + ".png"
		if err := os.WriteFile(name, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s %d 字节", name, len(raw))
	}
}

// TestSeedV2RenderSampleOffset 采样点必须落在像素自己的方块范围内，
// 且 bpp≥4 时要取 voronoi 格中心（+2）而不是格角（+0）。
// 这条锁住 bpp=1 贴脸瓦片不被整体右移 2 像素的回归。
func TestSeedV2RenderSampleOffset(t *testing.T) {
	gen := yyV2NewBiomeGen(12345, 102104, false)
	want := func(bx, bz int) color.NRGBA {
		r, g, b := yyV2BiomeColor(gen.BiomeAtBlock(bx, yyV2SurfaceBlockY, bz))
		if r == 0 && g == 0 && b == 0 {
			r, g, b = 18, 18, 26
		}
		return color.NRGBA{R: r, G: g, B: b, A: 255}
	}
	for _, c := range []struct {
		bpp int
		off int
	}{{1, 0}, {2, 1}, {4, 2}, {8, 2}, {16, 2}} {
		opt := yyV2TestOpt(12345, yyV2DimOverworld, 0, 0, 32, 32, c.bpp)
		opt.Layer = "terrain"
		raw, err := yyV2RenderTilePNG(opt)
		if err != nil {
			t.Fatalf("bpp=%d: %v", c.bpp, err)
		}
		_, _, get := yyV2TestDecode(t, raw)
		// 只抽查边界与中心几个点（bpp=1 是逐像素，32×32 全查太慢）
		for _, p := range [][2]int{{0, 0}, {31, 31}, {7, 13}, {16, 16}} {
			bx := p[0]*c.bpp + c.off
			bz := p[1]*c.bpp + c.off
			if got, exp := get(p[0], p[1]), want(bx, bz); got != exp {
				t.Fatalf("bpp=%d 像素(%d,%d) = #%02x%02x%02x，期望取方块(%d,%d) 的 #%02x%02x%02x",
					c.bpp, p[0], p[1], got.R, got.G, got.B, bx, bz, exp.R, exp.G, exp.B)
			}
		}
	}
}

// TestSeedV2RenderTerrainSane 粗略验色：整片海洋/沙漠/雪原的瓦片要落在预期色域。
func TestSeedV2RenderTerrainSane(t *testing.T) {
	// 直接构造三种极端群系的判定，确认调色板符合直觉（海洋蓝 / 沙漠黄 / 雪原白）
	check := func(name string, id int32, want func(r, g, b uint8) bool) {
		r, g, b := yyV2BiomeColor(id)
		if !want(r, g, b) {
			t.Fatalf("%s(%d) 颜色 #%02x%02x%02x 不符合预期", name, id, r, g, b)
		}
	}
	check("海洋", 0, func(r, g, b uint8) bool { return b > 80 && b > r+40 && b > g+40 })
	check("沙漠", 2, func(r, g, b uint8) bool { return r > 200 && g > 100 && b < 80 })
	check("雪原", 12, func(r, g, b uint8) bool { return r > 240 && g > 240 && b > 240 })

	// 渲染一张全图，确认不是单色（说明引擎真的在出不同群系）
	raw, err := yyV2RenderTilePNG(yyV2TestOpt(12345, yyV2DimOverworld, -512, -512, 256, 256, 4))
	if err != nil {
		t.Fatal(err)
	}
	_, _, get := yyV2TestDecode(t, raw)
	distinct := map[color.NRGBA]int{}
	for y := 0; y < 256; y += 8 {
		for x := 0; x < 256; x += 8 {
			distinct[get(x, y)]++
		}
	}
	if len(distinct) < 2 {
		t.Fatalf("±512 区块的瓦片只有 %d 种颜色，生物群系引擎没在出图", len(distinct))
	}
	t.Logf("大范围瓦片采样到 %d 种颜色", len(distinct))
	_ = fmt.Sprintf
}

// ── 生产路径群系准确率（chunkbase 黄金基准）──────────────────────────
//
// 为什么要在渲染层再测一次：`seedv2_biome_golden_test.go` 测的是 `Axes()` 直采，
// 跳过了 voronoi 和坐标换算，天然 99.98%；玩家实际看到的是渲染管线
// （格中心偏移 → 调色板 → PNG），那条路真实准确率只有 99.7%。
// 这里按 1 像素 = 1 voronoi 格渲染真实瓦片，逐像素和黄金基准比色。
const yyV2RenderGoldenMinRate = 99.0

type yyV2GoldenTile struct {
	X    int     `json:"x"`
	Z    int     `json:"z"`
	W    int     `json:"w"`
	H    int     `json:"h"`
	Data []uint8 `json:"data"` // 每 3 字节：biome id / height lo / height hi
}
type yyV2GoldenCase struct {
	Version string           `json:"version"`
	Seed    string           `json:"seed"`
	Tiles   []yyV2GoldenTile `json:"tiles"`
}
type yyV2GoldenFile struct {
	Cases map[string]yyV2GoldenCase `json:"cases"`
}

func yyV2GoldenSeed(s string) int64 {
	var v int64
	neg := false
	for i := 0; i < len(s); i++ {
		if i == 0 && s[i] == '-' {
			neg = true
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			break
		}
		v = v*10 + int64(s[i]-'0')
	}
	if neg {
		return -v
	}
	return v
}

func TestSeedV2RenderBiomeAccuracy(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "chunkbase-biomes.json"))
	if err != nil {
		t.Skipf("没有黄金基准 %v，跳过", err)
	}
	var gf yyV2GoldenFile
	if err := json.Unmarshal(raw, &gf); err != nil {
		t.Fatalf("黄金基准解析失败: %v", err)
	}
	ok, total := 0, 0
	for _, c := range gf.Cases {
		code := yyV2ParseVersion(c.Version, yyV2EditionJava)
		for _, tile := range c.Tiles {
			// CX/CZ 是区块坐标，bpp=4 时一个像素 = 一个格 = 4 方块，
			// 只有格坐标能被 4 整除的基准瓦片才能 1:1 对齐
			if tile.X%4 != 0 || tile.Z%4 != 0 {
				continue
			}
			opt := yyV2RenderOptions{
				Seed: yyV2GoldenSeed(c.Seed), VersionCode: code, Edition: yyV2EditionJava,
				Dimension: yyV2DimOverworld, CX: tile.X / 4, CZ: tile.Z / 4,
				W: tile.W, H: tile.H, BlocksPerPixel: 4, Layer: "terrain",
			}
			png, err := yyV2RenderTilePNG(opt)
			if err != nil {
				t.Fatalf("%s/%s: %v", c.Version, c.Seed, err)
			}
			_, _, get := yyV2TestDecode(t, png)
			for j := 0; j < tile.H; j++ {
				for i := 0; i < tile.W; i++ {
					id := int32(tile.Data[(j*tile.W+i)*3])
					r, g, b := yyV2BiomeColor(id)
					if r == 0 && g == 0 && b == 0 { // 与渲染层对深暗之域的提亮保持一致
						r, g, b = 18, 18, 26
					}
					if px := get(i, j); px.R == r && px.G == g && px.B == b {
						ok++
					}
					total++
				}
			}
		}
	}
	if total == 0 {
		t.Fatal("黄金基准里没有格对齐的瓦片，测试没跑到东西")
	}
	rate := float64(ok) * 100 / float64(total)
	t.Logf("渲染路径 vs chunkbase 一致率 %.3f%%（%d/%d）", rate, ok, total)
	if rate < yyV2RenderGoldenMinRate {
		t.Fatalf("渲染路径一致率 %.2f%%，低于门槛 %.1f%%", rate, yyV2RenderGoldenMinRate)
	}
}
