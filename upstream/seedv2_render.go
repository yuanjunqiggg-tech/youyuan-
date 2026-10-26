package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_render.go —— 地图瓦片渲染（PNG 字节流）
//
// 【归属】agent: seed-render
// 【来源】D:/种子查询器自研/api/render-worker.mjs（降采样 / 图标叠加逻辑）
// 【分工】地形 + 采样 + 摘要在本文件；图标形状/栅格化/SVG 在 seedv2_icons.go
//         （本文件只调 yyV2RenderBakeIcons，不自己画图形）。
//
// 【要求】
//   - 用标准库 image / image/png，不要自己写 PNG 编码器。
//   - 输出必须是**不透明** RGB（前端用 <img> 直接贴，透明边会在深色主题下发黑）。
//   - 单张瓦片 256×256 上限；大范围由前端拼多张，不要一次渲染 1024×1024。
//   - 渲染耗时要可预期：256×256 一瓦片目标 < 200ms（Go 比 JS 快，别退步）。
//   - 结构图标必须是**原创矢量图形**（圆底 + 白图形），不许用 chunkbase 素材。
//
// 【采样策略】生物群系引擎实测 18.7µs/次（零分配纯浮点），
//   256×256 逐像素 = 65536 × 18.7µs ≈ 1.2s，绝对不能这么干。
//   所以主世界按 sampleN×sampleN 网格采样后再**最近邻**放大：
//     sampleN = clamp(min(W,H), 32, 64)，默认 64 → 4096 次 ≈ 77ms
//   最近邻而不是双线性：插值会凭空造出第三种群系色，把海岸线糊成灰带。
//   例外：bpp==1 且 W*H<=16384（贴脸看边界的小瓦片）逐像素采样，保证边界准。
//
//   下界 / 末地不经过气候噪声（那套噪声只对主世界成立）：
//   下界用稳定的低频 value noise 做四色示意分区，末地用径向主岛亮度场，
//   两者都是纯哈希，逐像素算也不费时（约 1ms），所以直接全分辨率。
//   ⚠ 下界分区是**示意**，不是真实下界群系分布，前端不要当权威数据用。
//
// 【坐标铁律】像素 → 方块 = (CX*16 + px*BlocksPerPixel, CZ*16 + py*BlocksPerPixel)
//   BiomeAtBlock 内部已做 voronoi 1:4，**不要再手乘 4**（踩过 4 倍偏移的坑）。
// ═══════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"sort"
)

// yyV2RenderOptions 的兜底值 / 采样参数。
const (
	yyV2RenderMaxPixels     = 512   // W/H 上限（路由层也夹过，这里二次防呆）
	yyV2RenderSampleMin     = 32    // 采样网格下限
	yyV2RenderSampleMax     = 64    // 采样网格上限（4096 次 ≈ 77ms）
	yyV2RenderPixelFastCap  = 16384 // bpp==1 时逐像素采样的像素数上限
	yyV2RenderDefaultIcons  = 320   // 图标上限默认值
	yyV2RenderScanHardLimit = 4000  // 单类型扫描硬上限，防极端 bpp 把扫描拖爆
	yyV2SummarySamples      = 64    // 摘要采样点数（8×8）
	yyV2SummaryScanRadius   = 512   // 摘要最近结构的扫描半径上限（区块）
)

// yyV2RenderTilePNG 渲染一张瓦片，返回 PNG 字节流。
func yyV2RenderTilePNG(opt yyV2RenderOptions) ([]byte, error) {
	opt = yyV2RenderNormalizeOptions(opt)
	img := image.NewRGBA(image.Rect(0, 0, opt.W, opt.H))

	switch yyV2NormalizeDimension(opt.Dimension) {
	case yyV2DimNether:
		yyV2RenderFillNether(img, opt)
	case yyV2DimEnd:
		yyV2RenderFillEnd(img, opt)
	default:
		yyV2RenderFillOverworld(img, opt)
	}

	// layer=terrain 时完全不画图标；空 / structures 都画（structures 是「地形 + 图标」）
	if opt.Layer != "terrain" {
		filtered := yyV2RenderFilterIDs(opt.StructureIDs, opt)
		// 显式指定了 types 但全被过滤掉 → 按用户意思不画，不要偷偷退回默认集
		if len(opt.StructureIDs) == 0 || len(filtered) > 0 {
			opt.StructureIDs = filtered
			yyV2RenderBakeIcons(img, opt)
		}
	}

	var buf bytes.Buffer
	buf.Grow(1 << 15)
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// yyV2RenderNormalizeOptions 把外部入参夹到安全区间。
func yyV2RenderNormalizeOptions(opt yyV2RenderOptions) yyV2RenderOptions {
	opt.W = yyV2RenderClamp(opt.W, 32, yyV2RenderMaxPixels)
	opt.H = yyV2RenderClamp(opt.H, 32, yyV2RenderMaxPixels)
	opt.BlocksPerPixel = yyV2RenderNormalizeBPP(opt.BlocksPerPixel)
	opt.Dimension = yyV2NormalizeDimension(opt.Dimension)
	opt.Edition = yyV2NormalizeEdition(opt.Edition)
	opt.Layer = yyV2Lower(trimSpaceASCII(opt.Layer))
	if opt.MaxIcons <= 0 {
		opt.MaxIcons = yyV2RenderDefaultIcons
	}
	if opt.MaxIcons > 2000 {
		opt.MaxIcons = 2000
	}
	return opt
}

// yyV2RenderNormalizeBPP bpp 只接受 1/2/4/8/16/32，其它就近向上取整。
func yyV2RenderNormalizeBPP(bpp int) int {
	switch {
	case bpp <= 1:
		return 1
	case bpp <= 2:
		return 2
	case bpp <= 4:
		return 4
	case bpp <= 8:
		return 8
	case bpp <= 16:
		return 16
	default:
		return 32
	}
}

func yyV2RenderClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// yyV2RenderFilterIDs 只留该维度真实可用的结构（顺手把别名归一成规范 id）。
// 前端传错 id 不该让整张图没有图标，也不该把下界结构画到主世界地图上。
func yyV2RenderFilterIDs(ids []string, opt yyV2RenderOptions) []string {
	if len(ids) == 0 {
		return nil
	}
	kept := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, raw := range ids {
		id := yyV2PlaceCanonical(raw)
		if id == "" || seen[id] {
			continue
		}
		info, ok := yyV2StructureInfoByID(id, opt.VersionCode, opt.Edition)
		if !ok || info.Dim != opt.Dimension {
			continue
		}
		seen[id] = true
		kept = append(kept, id)
	}
	return kept
}

func trimSpaceASCII(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}

// ── 主世界：网格采样 + 最近邻放大 ──────────────────────────────────────

// yyV2RenderSampleOff 采样块坐标相对像素左侧的偏移。
//
// 生物群系是 1:4 的 voronoi 格，**格角**（坐标 ≡0 mod 4）到四个格等距，谁抢到
// 全看浮点误差，会在地图上留下 4 方块宽的斑点；格中心（≡2 mod 4）才是最稳的点。
// 但偏移必须留在像素自己的方块范围内，否则 bpp=1 的贴脸瓦片会整张图右移 2 像素：
//
//	bpp=1 → 0   （一个像素就是一个方块，没得挪）
//	bpp=2 → 1   （像素覆盖 2 方块，取第一块）
//	bpp≥4 → 2   （= yyV2ChunkCenterOffset，落进格中心）
//
// 实测（chunkbase 49152 点，走 BiomeAtBlock）：偏移 0 → 96.26%，偏移 2 → 99.77%。
func yyV2RenderSampleOff(bpp int) int {
	off := yyV2ChunkCenterOffset
	if off > bpp-1 {
		off = bpp - 1
	}
	if off < 0 {
		off = 0
	}
	return off
}

func yyV2RenderFillOverworld(img *image.RGBA, opt yyV2RenderOptions) {
	gen := yyV2NewBiomeGen(opt.Seed, opt.VersionCode, opt.LargeBiomes)

	// 256 项调色板一次算好，循环里只剩查表（群系 id 只在 0..255）
	var pal [256][3]uint8
	for i := 0; i < 256; i++ {
		r, g, b := yyV2BiomeColor(int32(i))
		// 深暗之域的表色是纯黑：拼图时整块黑会被误读成「没加载出来」。
		// 只在这一层提亮到 #12121a（仍是全图最暗），不改群系表本身。
		if r == 0 && g == 0 && b == 0 {
			r, g, b = 18, 18, 26
		}
		pal[i] = [3]uint8{r, g, b}
	}

	// 贴脸小瓦片：逐像素，边界才不会被采样格吃掉
	if opt.BlocksPerPixel == 1 && opt.W*opt.H <= yyV2RenderPixelFastCap {
		originX := opt.CX * 16
		originZ := opt.CZ * 16
		off := yyV2RenderSampleOff(opt.BlocksPerPixel)
		for py := 0; py < opt.H; py++ {
			row := py * img.Stride
			bz := originZ + py
			for px := 0; px < opt.W; px++ {
				c := pal[uint8(gen.BiomeAtBlock(originX+px+off, yyV2SurfaceBlockY, bz+off))]
				i := row + px*4
				img.Pix[i] = c[0]
				img.Pix[i+1] = c[1]
				img.Pix[i+2] = c[2]
				img.Pix[i+3] = 255
			}
		}
		return
	}

	n := yyV2RenderClamp(opt.W, yyV2RenderSampleMin, yyV2RenderSampleMax)
	if opt.H < n {
		n = yyV2RenderClamp(opt.H, yyV2RenderSampleMin, yyV2RenderSampleMax)
	}

	// 采样点取每个采样格的中心像素，块坐标再往格中心挪 yyV2ChunkCenterOffset 个方块。
	off := yyV2RenderSampleOff(opt.BlocksPerPixel)
	grid := make([]int32, n*n)
	originX := opt.CX * 16
	originZ := opt.CZ * 16
	for gy := 0; gy < n; gy++ {
		py := (gy*opt.H)/n + opt.H/(2*n)
		if py >= opt.H {
			py = opt.H - 1
		}
		bz := originZ + py*opt.BlocksPerPixel
		row := gy * n
		for gx := 0; gx < n; gx++ {
			px := (gx*opt.W)/n + opt.W/(2*n)
			if px >= opt.W {
				px = opt.W - 1
			}
			grid[row+gx] = gen.BiomeAtBlock(
				originX+px*opt.BlocksPerPixel+off,
				yyV2SurfaceBlockY,
				bz+off)
		}
	}

	// 最近邻放大（不要插值）
	for py := 0; py < opt.H; py++ {
		gr := (py * n / opt.H) * n
		row := py * img.Stride
		for px := 0; px < opt.W; px++ {
			c := pal[uint8(grid[gr+px*n/opt.W])]
			i := row + px*4
			img.Pix[i] = c[0]
			img.Pix[i+1] = c[1]
			img.Pix[i+2] = c[2]
			img.Pix[i+3] = 255
		}
	}
}

// ── 下界：四色示意分区 ────────────────────────────────────────────────
//
// 下界群系在主世界那套 climate 噪声里根本不存在（depth/continentalness 对
// 下界没有意义），硬套只会得到一整片主世界配色。这里改用稳定的低频
// value noise 做四块分区，让地图「看起来像下界」：
//   下界荒地 / 灵魂沙峡谷 / 绯红森林 / 诡异森林
// ⚠ 纯示意，不代表真实生成。

const (
	yyV2NetherWastes   int32 = 8
	yyV2SoulSandValley int32 = 170
	yyV2CrimsonForest  int32 = 171
	yyV2WarpedForest   int32 = 172
	yyV2NetherSaltBase       = 0x4e455448 // "NETH"
	yyV2NetherSaltFine       = 0x534f554c // "SOUL"
)

func yyV2RenderFillNether(img *image.RGBA, opt yyV2RenderOptions) {
	var pal [4][3]uint8
	ids := [4]int32{yyV2NetherWastes, yyV2SoulSandValley, yyV2CrimsonForest, yyV2WarpedForest}
	for i, id := range ids {
		r, g, b := yyV2BiomeColor(id)
		pal[i] = [3]uint8{uint8(int(r) * 9 / 10), uint8(int(g) * 9 / 10), uint8(int(b) * 9 / 10)}
	}
	originX := opt.CX * 16
	originZ := opt.CZ * 16
	for py := 0; py < opt.H; py++ {
		row := py * img.Stride
		bz := float64(originZ + py*opt.BlocksPerPixel)
		for px := 0; px < opt.W; px++ {
			bx := float64(originX + px*opt.BlocksPerPixel)
			f := 0.68*yyV2RenderValueNoise(bx/176, bz/176, opt.Seed, yyV2NetherSaltBase) +
				0.32*yyV2RenderValueNoise(bx/72, bz/72, opt.Seed, yyV2NetherSaltFine)
			var c [3]uint8
			switch {
			case f < 0.30:
				c = pal[0]
			case f < 0.55:
				c = pal[1]
			case f < 0.80:
				c = pal[2]
			default:
				c = pal[3]
			}
			i := row + px*4
			img.Pix[i] = c[0]
			img.Pix[i+1] = c[1]
			img.Pix[i+2] = c[2]
			img.Pix[i+3] = 255
		}
	}
}

// yyV2RenderValueNoise 双线性平滑的 value noise，返回 [0,1)。
func yyV2RenderValueNoise(fx, fz float64, seed int64, salt uint64) float64 {
	x0 := math.Floor(fx)
	z0 := math.Floor(fz)
	tx := yyV2RenderSmooth(fx - x0)
	tz := yyV2RenderSmooth(fz - z0)
	ix, iz := int64(x0), int64(z0)
	v00 := yyV2RenderUnitHash(ix, iz, seed, salt)
	v10 := yyV2RenderUnitHash(ix+1, iz, seed, salt)
	v01 := yyV2RenderUnitHash(ix, iz+1, seed, salt)
	v11 := yyV2RenderUnitHash(ix+1, iz+1, seed, salt)
	top := v00 + (v10-v00)*tx
	bot := v01 + (v11-v01)*tx
	return top + (bot-top)*tz
}

func yyV2RenderSmooth(t float64) float64 { return t * t * (3 - 2*t) }

// yyV2RenderUnitHash 整数格点 → [0,1) 的稳定哈希（不依赖 map / rand，跨平台一致）。
func yyV2RenderUnitHash(x, z int64, seed int64, salt uint64) float64 {
	h := salt ^ (uint64(seed) * 0x9E3779B97F4A7C15)
	h ^= uint64(x) * 0xC2B2AE3D27D4EB4F
	h ^= uint64(z) * 0x165667B19E3779F9
	h ^= h >> 29
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 32
	h *= 0x94D049BB133111EB
	h ^= h >> 31
	return float64(h>>11) / float64(uint64(1)<<53)
}

// ── 末地：整体暗紫 + 主岛稍亮 ─────────────────────────────────────────
//
// 末地不用群系表那套颜色（end_midlands 是淡黄绿，那是用于「单点查询」的
// 辨识色，铺成整张地图会变成一张亮黄纸）。这里按天空基色 #2b2b3c 走，
// 主岛（原点半径 ~1000 方块）稍亮，外岛随距离衰减到基色。

const (
	yyV2EndBaseR = 43 // #2b2b3c
	yyV2EndBaseG = 43
	yyV2EndBaseB = 60
	yyV2EndSalt  = 0x454e4421 // "END!"
)

func yyV2RenderFillEnd(img *image.RGBA, opt yyV2RenderOptions) {
	originX := opt.CX * 16
	originZ := opt.CZ * 16
	for py := 0; py < opt.H; py++ {
		row := py * img.Stride
		bz := originZ + py*opt.BlocksPerPixel
		fz := float64(bz)
		for px := 0; px < opt.W; px++ {
			bx := originX + px*opt.BlocksPerPixel
			fx := float64(bx)
			// 主岛亮度：r<1200 线性衰减，平方后过渡柔和。
			// 量化到 1/16 档：径向渐变的浮点尾数会让每个像素颜色都独一无二，
			// PNG 压不动（实测一张暗色瓦片能从 3KB 涨到 30KB+）。
			t := 1 - math.Hypot(fx, fz)/1200
			if t < 0 {
				t = 0
			}
			t *= t
			t = math.Floor(t*16) / 16
			// 星尘微扰：8 方块一个块、只取 5 档，既有颗粒感又能被 deflate 压下去
			// （连续抖动会让一张原本 3KB 的暗色瓦片涨到 38KB）
			u := yyV2RenderUnitHash(int64(bx>>4), int64(bz>>4), opt.Seed, yyV2EndSalt)
			d := (float64(int(u*5)) - 2) * 1.6
			r := yyV2RenderClampF(float64(yyV2EndBaseR)+31*t+d, 0, 255)
			g := yyV2RenderClampF(float64(yyV2EndBaseG)+31*t+d*0.8, 0, 255)
			b := yyV2RenderClampF(float64(yyV2EndBaseB)+42*t+d*1.2, 0, 255)
			i := row + px*4
			img.Pix[i] = uint8(r)
			img.Pix[i+1] = uint8(g)
			img.Pix[i+2] = uint8(b)
			img.Pix[i+3] = 255
		}
	}
}

func yyV2RenderClampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ═══════════════════════════════════════════════════════════════════════
// 首屏摘要
// ═══════════════════════════════════════════════════════════════════════

// yyV2SeedMapSummary 给前端首屏用的一句话摘要。
// 返回 **必须非 nil**（路由层判 nil 会输出 {}，前端首屏就空白）：
//
//	{"terrain":"平原为主",
//	 "biome_top":[{"name":"平原","share":0.42}, ...],
//	 "nearest":[{"id":"village","name":"村庄","x":-38,"z":12,
//	             "dist_chunks":40,"dist_blocks":640}, ...]}
//
// terrain / biome_top 用 8×8=64 点采样（约 1.2ms），按出现次数取众数 / 前五。
// nearest 跨类型按欧氏距离排序取前 5；扫描半径硬上限 512 区块，
// 免得一次摘要把整个 ±4096 区域的结构全枚举一遍。
func yyV2SeedMapSummary(seed int64, versionCode int32, edition, dimension string, cx, cz, radius int) map[string]any {
	dim := yyV2NormalizeDimension(dimension)
	ed := yyV2NormalizeEdition(edition)
	if radius <= 0 {
		radius = 256
	}
	radius = yyV2RenderClamp(radius, 8, 4096)

	counts, order := yyV2SummaryBiomeSample(seed, versionCode, ed, dim, cx, cz, radius)
	terrain := "未知"
	biomeTop := make([]map[string]any, 0, 5)
	type bt struct {
		name  string
		count int
	}
	rows := make([]bt, 0, len(order))
	for _, id := range order {
		rows = append(rows, bt{name: yyV2SummaryBiomeLabel(dim, id), count: counts[id]})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].name < rows[j].name
	})
	for i, r := range rows {
		if i >= 5 {
			break
		}
		share := math.Round(float64(r.count)/float64(yyV2SummarySamples)*100) / 100
		biomeTop = append(biomeTop, map[string]any{"name": r.name, "share": share})
	}
	if len(rows) > 0 {
		terrain = rows[0].name + "为主"
	}

	out := map[string]any{
		"dim":       dim,
		"radius":    radius,
		"terrain":   terrain,
		"biome_top": biomeTop,
		"nearest":   yyV2SummaryNearest(seed, versionCode, ed, dim, cx, cz, radius),
	}
	return out
}

// yyV2SummaryBiomeSample 64 点采样，返回 (id→次数, 首次出现顺序)。
func yyV2SummaryBiomeSample(seed int64, versionCode int32, edition, dim string, cx, cz, radius int) (map[int32]int, []int32) {
	counts := make(map[int32]int, 16)
	order := make([]int32, 0, 16)

	step := (radius * 2) / 8
	if step <= 0 {
		step = 1
	}
	var gen *yyV2BiomeGen
	if dim == yyV2DimOverworld {
		gen = yyV2NewBiomeGen(seed, versionCode, false)
	}
	for i := 0; i < 8; i++ {
		chx := cx - radius + step*i + step/2
		for j := 0; j < 8; j++ {
			chz := cz - radius + step*j + step/2
			var id int32
			switch dim {
			case yyV2DimNether:
				id = yyV2SummaryNetherID(chx*16+8, chz*16+8, seed)
			case yyV2DimEnd:
				if chx*chx+chz*chz <= 64*64 { // 主岛 ~1024 方块半径
					id = 41 // end_midlands
				} else {
					id = 40 // small_end_islands
				}
			default:
				id = gen.BiomeAtChunk(chx, chz)
			}
			if counts[id] == 0 {
				order = append(order, id)
			}
			counts[id]++
		}
	}
	return counts, order
}

// yyV2SummaryNetherID 与瓦片渲染同一套分区（保证摘要和地图不打架）。
func yyV2SummaryNetherID(bx, bz int, seed int64) int32 {
	f := 0.68*yyV2RenderValueNoise(float64(bx)/176, float64(bz)/176, seed, yyV2NetherSaltBase) +
		0.32*yyV2RenderValueNoise(float64(bx)/72, float64(bz)/72, seed, yyV2NetherSaltFine)
	switch {
	case f < 0.30:
		return yyV2NetherWastes
	case f < 0.55:
		return yyV2SoulSandValley
	case f < 0.80:
		return yyV2CrimsonForest
	default:
		return yyV2WarpedForest
	}
}

// yyV2SummaryBiomeLabel 采样 id → 展示名。下界/末地用真实群系名（末地按主岛/外岛近似）。
func yyV2SummaryBiomeLabel(dim string, id int32) string {
	if dim == yyV2DimEnd {
		if id == 41 {
			return yyV2BiomeName(41)
		}
		return yyV2BiomeName(40)
	}
	return yyV2BiomeName(id)
}

// yyV2SummaryTypeIDs 该维度默认参与统计的结构（复用路由层的默认表；
// 表缺失时退回「结构表里 Default=true 且维度匹配」）。
func yyV2SummaryTypeIDs(dim string, versionCode int32, edition string) []string {
	if ids, ok := yySeedV2DefaultTypes[dim]; ok && len(ids) > 0 {
		return ids
	}
	out := make([]string, 0, 8)
	for _, info := range yyV2StructureList(versionCode, edition) {
		if info.Dim == dim && info.Default {
			out = append(out, info.ID)
		}
	}
	return out
}

// yyV2SummaryNearest 半径内最近 5 个结构（跨类型按距离排）。
func yyV2SummaryNearest(seed int64, versionCode int32, edition, dim string, cx, cz, radius int) []map[string]any {
	out := make([]map[string]any, 0, 5)
	ids := yyV2SummaryTypeIDs(dim, versionCode, edition)
	if len(ids) == 0 {
		return out
	}
	scanR := radius
	if scanR > yyV2SummaryScanRadius {
		scanR = yyV2SummaryScanRadius
	}

	type nearItem struct {
		id   string
		name string
		x    int
		z    int
		dist float64
	}
	items := make([]nearItem, 0, 32)
	for _, raw := range ids {
		id := yyV2PlaceCanonical(raw)
		if id == "" {
			continue
		}
		info, ok := yyV2StructureInfoByID(id, versionCode, edition)
		if !ok || info.Dim != dim {
			continue
		}
		pts := yyV2ScanStructures(seed, id, versionCode, edition,
			cx-scanR, cz-scanR, cx+scanR, cz+scanR, 512)
		for _, p := range pts {
			dx := float64(p.X - cx)
			dz := float64(p.Z - cz)
			items = append(items, nearItem{
				id: p.Type, name: info.Name, x: p.X, z: p.Z,
				dist: math.Hypot(dx, dz),
			})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].dist != items[j].dist {
			return items[i].dist < items[j].dist
		}
		if items[i].id != items[j].id {
			return items[i].id < items[j].id
		}
		if items[i].x != items[j].x {
			return items[i].x < items[j].x
		}
		return items[i].z < items[j].z
	})
	if len(items) > 5 {
		items = items[:5]
	}
	for _, it := range items {
		dc := int(math.Round(it.dist))
		out = append(out, map[string]any{
			"id":          it.id,
			"name":        it.name,
			"x":           it.x,
			"z":           it.z,
			"dist_chunks": dc,
			"dist_blocks": dc * 16,
		})
	}
	return out
}
