package main

// ═══════════════════════════════════════════════════════════════════════
// 种子查询 v2 · HTTP 路由（2026-10-26 第 28 条）
//
// 命名空间：/api/seed/v2/*
// 旧版 /api/seed/*（youyuan_seed_api.go）保留兼容，不再演进；前端与 MCP 一律走 v2。
//
// 设计纪律：
//   1. 全部 GET、纯本机计算、无副作用、不联网、不读写用户文件。
//   2. 任何输入都不许 panic / 500：范围过大就截断并在返回里标 truncated，
//      参数非法才 400。上层 yySeedGuard 已兜 panic。
//   3. 上限一律写死在常量里，不做"看情况"的事。
// ═══════════════════════════════════════════════════════════════════════

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	yySeedV2DefaultVersion = "1.21.4"
	yySeedV2MaxRadius      = 2048 // 结构扫描半径上限（区块）
	yySeedV2MaxTypes       = 24   // 单次最多查多少种结构
	yySeedV2MaxLimit       = 20000
	yySeedV2MaxPoints      = 4096 // biome 单次最多多少个点
	yySeedV2MaxSlimeRadius = 256
)

// 版本表：与 chunkbase 实际下拉一致（Java 22 项 / Bedrock 12 项）。
// value 是传给引擎的版本号字符串，label 是给人看的（含区间说明）。
var yySeedV2Versions = map[string][][2]string{
	yyV2EditionJava: {
		{"26.4", "26.4 (实验)"}, {"26.3", "26.3"}, {"26.2", "26.2"}, {"26.1", "26.1"},
		{"1.21.4", "1.21.4"}, {"1.21.2", "1.21.2 - 1.21.3"}, {"1.21", "1.21 - 1.21.1"},
		{"1.20", "1.20"}, {"1.19.3", "1.19.3 - 1.19.4"}, {"1.19", "1.19 - 1.19.2"},
		{"1.18", "1.18"}, {"1.17", "1.17"}, {"1.16", "1.16"}, {"1.15", "1.15"},
		{"1.14", "1.14"}, {"1.13", "1.13"}, {"1.12", "1.12"}, {"1.11", "1.11"},
		{"1.10", "1.10"}, {"1.9", "1.9"}, {"1.8", "1.8"}, {"1.7", "1.7"},
	},
	yyV2EditionBedrock: {
		{"26.70", "26.70 (实验)"}, {"26.60", "26.60 (实验)"},
		{"26.50", "26.50 - 26.52"}, {"26.30", "26.30 - 26.45"}, {"26.0", "26.0 - 26.23"},
		{"1.21", "1.21 - 1.21.40"}, {"1.20", "1.20.0 - 1.20.51"},
		{"1.19", "1.19"}, {"1.18", "1.18"}, {"1.17", "1.17"},
		{"1.16", "1.16"}, {"1.14", "1.14"},
	},
}

// yySeedV2DefaultTypes 每个维度打开页面时默认勾选的结构。
var yySeedV2DefaultTypes = map[string][]string{
	yyV2DimOverworld: {"village", "woodlandMansion", "oceanMonument", "ancientCity", "pillagerOutpost", "desertTemple", "jungleTemple", "stronghold"},
	yyV2DimNether:    {"netherFortress", "bastionRemnant", "ruinedPortalNether"},
	yyV2DimEnd:       {"endCity", "endShip"},
}

// yySeedV2Query 一次请求解析出来的公共参数。
type yySeedV2Query struct {
	Seed        int64
	VersionRaw  string
	VersionCode int32
	Edition     string
	Dim         string
	Large       bool
}

// yySeedV2Routes 注册 /api/seed/v2/* 全部 GET 接口。
func yySeedV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/seed/v2/manifest", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, yySeedV2Manifest())
	}))

	mux.HandleFunc("/api/seed/v2/meta", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		q, ok := yySeedV2ReadQuery(w, r, false)
		if !ok {
			return
		}
		list := yyV2StructureList(q.VersionCode, q.Edition)
		items := make([]map[string]any, 0, len(list))
		for _, info := range list {
			items = append(items, yySeedV2StructureJSON(info))
		}
		writeJSON(w, map[string]any{
			"ok": true, "version": q.VersionRaw, "version_code": q.VersionCode,
			"edition": q.Edition, "structures": items,
			"dimensions": yySeedV2DimensionJSON(),
		})
	}))

	mux.HandleFunc("/api/seed/v2/tile.png", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		yySeedV2Tile(w, r)
	}))

	mux.HandleFunc("/api/seed/v2/structures", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		yySeedV2Structures(w, r)
	}))

	mux.HandleFunc("/api/seed/v2/biome", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		yySeedV2Biome(w, r)
	}))

	mux.HandleFunc("/api/seed/v2/slime", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		yySeedV2Slime(w, r)
	}))

	mux.HandleFunc("/api/seed/v2/stronghold", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		yySeedV2Stronghold(w, r)
	}))

	mux.HandleFunc("/api/seed/v2/summary", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		yySeedV2Summary(w, r)
	}))

	// 结构图标：前端 DOM 图标层直接 <img src="/api/seed/v2/icon.svg?id=village">。
	// 放在后端出，是为了「PNG 瓦片上的图标」和「前端叠加的图标」用同一份几何定义，
	// 不会一个圆一个方。
	mux.HandleFunc("/api/seed/v2/icon.svg", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			yySeedFail(w, http.StatusBadRequest, "缺少 id 参数")
			return
		}
		size, err := yySeedParamInt(r, "size", 20)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		size = yySeedV2Clamp(size, 8, 128)
		svg := yyV2StructureIconSVG(id, size)
		if svg == "" {
			yySeedFail(w, http.StatusBadRequest, "未知结构："+id)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(svg))
	}))
}

// ── 参数解析 ──────────────────────────────────────────────────────────

// yySeedV2ReadQuery 解析公共参数。needSeed=false 时 seed 可缺省（manifest/meta 用）。
func yySeedV2ReadQuery(w http.ResponseWriter, r *http.Request, needSeed bool) (yySeedV2Query, bool) {
	q := yySeedV2Query{Edition: yyV2EditionJava, Dim: yyV2DimOverworld}
	seed, hasSeed, err := yySeedParamInt64(r, "seed")
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return q, false
	}
	if !hasSeed && needSeed {
		yySeedFail(w, http.StatusBadRequest, "缺少 seed 参数")
		return q, false
	}
	q.Seed = seed
	q.Edition = yyV2NormalizeEdition(r.URL.Query().Get("edition"))
	q.Dim = yyV2NormalizeDimension(r.URL.Query().Get("dim"))
	q.Large = yySeedV2Bool(r.URL.Query().Get("large"))

	raw := strings.TrimSpace(r.URL.Query().Get("version"))
	if raw == "" {
		raw = yySeedV2DefaultVersion
	}
	if len(raw) > 16 {
		yySeedFail(w, http.StatusBadRequest, "version 过长")
		return q, false
	}
	q.VersionRaw = raw
	q.VersionCode = yyV2ParseVersion(raw, q.Edition)
	return q, true
}

func yySeedV2Bool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func yySeedV2Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// yySeedV2TypeList 解析 types 参数；空则用该维度默认集。
func yySeedV2TypeList(r *http.Request, dim string, versionCode int32, edition string) []string {
	raw := strings.TrimSpace(r.URL.Query().Get("types"))
	var out []string
	if raw == "" {
		out = append(out, yySeedV2DefaultTypes[dim]...)
	} else {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if len(part) > 40 {
				continue
			}
			out = append(out, part)
		}
	}
	// 只保留该版本+维度真实可用的，避免前端传错就静默丢
	valid := make([]string, 0, len(out))
	seen := map[string]bool{}
	for _, id := range out {
		if seen[id] {
			continue
		}
		info, ok := yyV2StructureInfoByID(id, versionCode, edition)
		if !ok || info.Dim != dim {
			continue
		}
		seen[info.ID] = true
		valid = append(valid, info.ID)
		if len(valid) >= yySeedV2MaxTypes {
			break
		}
	}
	return valid
}

// ── handlers ──────────────────────────────────────────────────────────

func yySeedV2Manifest() map[string]any {
	tables := yyV2BTreeList()
	editions := make([]map[string]any, 0, 2)
	for _, ed := range []struct{ id, name string }{
		{yyV2EditionJava, "Java 版"}, {yyV2EditionBedrock, "基岩版"},
	} {
		pairs := yySeedV2Versions[ed.id]
		vers := make([]map[string]any, 0, len(pairs))
		for _, p := range pairs {
			vers = append(vers, map[string]any{
				"label": p[1], "version": p[0], "code": yyV2ParseVersion(p[0], ed.id),
			})
		}
		editions = append(editions, map[string]any{"id": ed.id, "name": ed.name, "versions": vers})
	}
	// 结构清单按 Java 最新版给一份全量（前端首屏渲染图标选择器用）
	all := yyV2StructureList(yyV2ParseVersion("1.21.4", yyV2EditionJava), yyV2EditionJava)
	structs := make([]map[string]any, 0, len(all))
	for _, info := range all {
		structs = append(structs, yySeedV2StructureJSON(info))
	}
	return map[string]any{
		"ok":            true,
		"engine":        "seedv2",
		"btree_tables":  tables,
		"biome_engine":  len(tables) > 0,
		"render_engine": len(tables) > 0,
		"editions":      editions,
		"dimensions":    yySeedV2DimensionJSON(),
		"structures":    structs,
		"note":          "全部计算在本机完成，不联网、不上传种子",
	}
}

func yySeedV2DimensionJSON() []map[string]any {
	return []map[string]any{
		{"id": yyV2DimOverworld, "name": "主世界"},
		{"id": yyV2DimNether, "name": "下界"},
		{"id": yyV2DimEnd, "name": "末地"},
	}
}

func yySeedV2StructureJSON(info yyV2StructureInfo) map[string]any {
	return map[string]any{
		"id": info.ID, "name": info.Name, "dim": info.Dim,
		"precision": info.Precision, "placement": info.Placement,
		"color":   yySeedV2Hex(info.Color),
		"default": info.Default,
		"java":    []int32{info.Java[0], info.Java[1]},
		"bedrock": []int32{info.Bedrock[0], info.Bedrock[1]},
	}
}

func yySeedV2Hex(c [3]uint8) string {
	const hex = "0123456789abcdef"
	out := []byte{'#', 0, 0, 0, 0, 0, 0}
	for i := 0; i < 3; i++ {
		out[1+i*2] = hex[c[i]>>4]
		out[2+i*2] = hex[c[i]&0xf]
	}
	return string(out)
}

func yySeedV2Tile(w http.ResponseWriter, r *http.Request) {
	q, ok := yySeedV2ReadQuery(w, r, true)
	if !ok {
		return
	}
	cx, err := yySeedParamInt(r, "cx", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	cz, err := yySeedParamInt(r, "cz", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	wpx, err := yySeedParamInt(r, "w", 256)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	hpx, err := yySeedParamInt(r, "h", 256)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	bpp, err := yySeedParamInt(r, "bpp", 4)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	maxIcons, err := yySeedParamInt(r, "maxicons", 320)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	// bpp 只接受这几档，其它一律就近取整，避免前端传 3 就出奇怪像素
	switch {
	case bpp <= 1:
		bpp = 1
	case bpp <= 2:
		bpp = 2
	case bpp <= 4:
		bpp = 4
	case bpp <= 8:
		bpp = 8
	case bpp <= 16:
		bpp = 16
	default:
		bpp = 32
	}
	layer := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("layer")))
	if layer != "" && layer != "terrain" && layer != "structures" {
		yySeedFail(w, http.StatusBadRequest, "layer 只能是 terrain 或 structures")
		return
	}
	opt := yyV2RenderOptions{
		Seed: q.Seed, VersionCode: q.VersionCode, Edition: q.Edition,
		Dimension: q.Dim, LargeBiomes: q.Large,
		CX: cx, CZ: cz,
		W: yySeedV2Clamp(wpx, 32, 512), H: yySeedV2Clamp(hpx, 32, 512),
		BlocksPerPixel: bpp, Layer: layer,
		StructureIDs: yySeedV2TypeList(r, q.Dim, q.VersionCode, q.Edition),
		MaxIcons:     yySeedV2Clamp(maxIcons, 0, 2000),
	}
	png, err := yyV2RenderTilePNG(opt)
	if err != nil {
		yySeedFail(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if len(png) == 0 {
		yySeedFail(w, http.StatusServiceUnavailable, "渲染引擎尚未就绪")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Seed-Engine", "seedv2")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func yySeedV2Structures(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q, ok := yySeedV2ReadQuery(w, r, true)
	if !ok {
		return
	}
	cx, err := yySeedParamInt(r, "cx", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	cz, err := yySeedParamInt(r, "cz", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	radius, err := yySeedParamInt(r, "r", 256)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := yySeedParamInt(r, "limit", yySeedV2MaxLimit)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	radius = yySeedV2Clamp(radius, 1, yySeedV2MaxRadius)
	limit = yySeedV2Clamp(limit, 1, yySeedV2MaxLimit)

	types := yySeedV2TypeList(r, q.Dim, q.VersionCode, q.Edition)
	out := map[string][]yyV2StructPos{}
	counts := map[string]int{}
	total := 0
	truncated := false
	for _, id := range types {
		remain := limit - total
		if remain <= 0 {
			truncated = true
			break
		}
		got := yyV2ScanStructures(q.Seed, id, q.VersionCode, q.Edition,
			cx-radius, cz-radius, cx+radius, cz+radius, remain)
		if len(got) == 0 {
			continue
		}
		if len(got) >= remain {
			truncated = true
		}
		out[id] = got
		counts[id] = len(got)
		total += len(got)
	}
	writeJSON(w, map[string]any{
		"ok": true, "seed": q.Seed,
		"version": q.VersionRaw, "version_code": q.VersionCode, "edition": q.Edition, "dim": q.Dim,
		"range": map[string]int{
			"cx0": cx - radius, "cz0": cz - radius, "cx1": cx + radius, "cz1": cz + radius,
		},
		"structures": out, "counts": counts, "total": total,
		"truncated": truncated, "types": types,
		"elapsed_ms": time.Since(start).Milliseconds(),
	})
}

func yySeedV2Biome(w http.ResponseWriter, r *http.Request) {
	q, ok := yySeedV2ReadQuery(w, r, true)
	if !ok {
		return
	}
	if q.Dim != yyV2DimOverworld {
		// 下界/末地的群系不是气候噪声算出来的，明确拒绝而不是瞎给
		yySeedV2BiomeSimple(w, r, q)
		return
	}
	gen := yyV2NewBiomeGen(q.Seed, q.VersionCode, q.Large)
	xs := r.URL.Query()["x"]
	zs := r.URL.Query()["z"]
	if len(xs) > 0 || len(zs) > 0 {
		if len(xs) != len(zs) {
			yySeedFail(w, http.StatusBadRequest, "x 与 z 数量必须一致")
			return
		}
		if len(xs) > yySeedV2MaxPoints {
			yySeedFail(w, http.StatusBadRequest, "一次最多查 "+strconv.Itoa(yySeedV2MaxPoints)+" 个点")
			return
		}
		points := make([]map[string]any, 0, len(xs))
		for i := range xs {
			cx, e1 := strconv.Atoi(strings.TrimSpace(xs[i]))
			cz, e2 := strconv.Atoi(strings.TrimSpace(zs[i]))
			if e1 != nil || e2 != nil {
				yySeedFail(w, http.StatusBadRequest, "坐标必须是整数")
				return
			}
			points = append(points, yySeedV2BiomePoint(gen, cx, cz))
		}
		writeJSON(w, map[string]any{"ok": true, "points": points, "count": len(points)})
		return
	}
	// 网格模式：cx/cz 为中心，r 半径（区块），step 间隔
	cx, err := yySeedParamInt(r, "cx", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	cz, err := yySeedParamInt(r, "cz", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	radius, err := yySeedParamInt(r, "r", 32)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	step, err := yySeedParamInt(r, "step", 8)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	radius = yySeedV2Clamp(radius, 1, 512)
	step = yySeedV2Clamp(step, 1, 64)
	n := (radius*2)/step + 1
	if n*n > yySeedV2MaxPoints {
		yySeedFail(w, http.StatusBadRequest, "网格点太多，请减小 r 或加大 step")
		return
	}
	points := make([]map[string]any, 0, n*n)
	for dz := -radius; dz <= radius; dz += step {
		for dx := -radius; dx <= radius; dx += step {
			points = append(points, yySeedV2BiomePoint(gen, cx+dx, cz+dz))
		}
	}
	writeJSON(w, map[string]any{
		"ok": true, "points": points, "count": len(points),
		"range": map[string]int{"cx": cx, "cz": cz, "r": radius, "step": step},
	})
}

func yySeedV2BiomePoint(gen *yyV2BiomeGen, cx, cz int) map[string]any {
	id := gen.BiomeAtChunk(cx, cz)
	r, g, b := yyV2BiomeColor(id)
	return map[string]any{
		"x": cx, "z": cz, "biome": id,
		"key": yyV2BiomeKey(id), "name": yyV2BiomeName(id),
		"color": yySeedV2Hex([3]uint8{r, g, b}),
		"block_x": cx*16 + 8, "block_z": cz*16 + 8,
	}
}

// yySeedV2BiomeSimple 下界/末地的群系：按固定规则给，不装成气候噪声算出来的。
func yySeedV2BiomeSimple(w http.ResponseWriter, r *http.Request, q yySeedV2Query) {
	key, name := "minecraft:nether_wastes", "下界荒地"
	if q.Dim == yyV2DimEnd {
		key, name = "minecraft:the_end", "末地"
	}
	cx, _ := yySeedParamInt(r, "cx", 0)
	cz, _ := yySeedParamInt(r, "cz", 0)
	writeJSON(w, map[string]any{
		"ok": true,
		"points": []map[string]any{{
			"x": cx, "z": cz, "biome": 0, "key": key, "name": name, "color": "#2b2b3c",
		}},
		"note": "下界/末地的群系不是气候噪声生成的，这里只给该维度的代表群系，不做逐点计算",
	})
}

func yySeedV2Slime(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q, ok := yySeedV2ReadQuery(w, r, true)
	if !ok {
		return
	}
	if q.Edition != yyV2EditionJava {
		yySeedFail(w, http.StatusBadRequest, "史莱姆区块目前只支持 Java 版；基岩版算法不同，未实现")
		return
	}
	cx, err := yySeedParamInt(r, "cx", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	cz, err := yySeedParamInt(r, "cz", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	radius, err := yySeedParamInt(r, "r", 16)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	radius = yySeedV2Clamp(radius, 1, yySeedV2MaxSlimeRadius)
	chunks := make([]map[string]int, 0, radius*2)
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			if yyV2IsSlimeChunkV2(q.Seed, int32(cx+dx), int32(cz+dz)) {
				chunks = append(chunks, map[string]int{"x": cx + dx, "z": cz + dz})
			}
		}
	}
	writeJSON(w, map[string]any{
		"ok": true, "seed": q.Seed,
		"range": map[string]int{"cx0": cx - radius, "cz0": cz - radius, "cx1": cx + radius, "cz1": cz + radius},
		"chunks": chunks, "count": len(chunks),
		"precision":  "exact",
		"elapsed_ms": time.Since(start).Milliseconds(),
	})
}

func yySeedV2Stronghold(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q, ok := yySeedV2ReadQuery(w, r, true)
	if !ok {
		return
	}
	count, err := yySeedParamInt(r, "count", 128)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	// 基岩版要塞是另一套算法（只有 3 个、按固定环+群系搜），Go 侧没实现。
	// 拿 Java 的 128 环位冒充 = 给玩家 100% 错的坐标，所以直接拒绝。
	if q.Edition != yyV2EditionJava {
		yySeedFail(w, http.StatusNotImplemented, "基岩版要塞算法尚未实现（与 Java 完全不同），请切到 Java 版查询")
		return
	}
	count = yySeedV2Clamp(count, 1, 128)
	all := yyV2StrongholdsV2(q.Seed, q.VersionCode)
	if count < len(all) {
		all = all[:count]
	}
	out := make([]map[string]int, 0, len(all))
	for _, p := range all {
		out = append(out, map[string]int{"x": p[0], "z": p[1]})
	}
	writeJSON(w, map[string]any{
		"ok": true, "seed": q.Seed, "strongholds": out, "count": len(out),
		"precision":  "ring_approx",
		"elapsed_ms": time.Since(start).Milliseconds(),
		"note":       "环位候选（cubiomes initFirstStronghold 逐位复刻）；原版还会在允许的群系里就近回拉几十格，所以落地后要在附近 1~2 区块内找。方块坐标 = 区块*16+8",
	})
}

func yySeedV2Summary(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q, ok := yySeedV2ReadQuery(w, r, true)
	if !ok {
		return
	}
	cx, err := yySeedParamInt(r, "cx", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	cz, err := yySeedParamInt(r, "cz", 0)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	radius, err := yySeedParamInt(r, "r", 256)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	radius = yySeedV2Clamp(radius, 16, 512)
	res := yyV2SeedMapSummary(q.Seed, q.VersionCode, q.Edition, q.Dim, cx, cz, radius)
	if res == nil {
		res = map[string]any{}
	}
	res["ok"] = true
	res["seed"] = q.Seed
	res["version"] = q.VersionRaw
	res["edition"] = q.Edition
	res["dim"] = q.Dim
	res["elapsed_ms"] = time.Since(start).Milliseconds()
	writeJSON(w, res)
}
