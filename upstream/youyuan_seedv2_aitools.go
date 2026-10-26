package main

// ═══════════════════════════════════════════════════════════════════════
// 种子查询 v2 · 内置机器人 AI 工具桥接
//
// 旧的 8 个 seed_* 工具（youyuan_seed_api.go 的 yySeedToolCall）用的是 10-24 那版
// 近似实现：府邸/神殿/末地城缺三角分布、要塞只是三环近似、生物群系是手写规则表。
//
// 这里把除「坐标分享码」以外的全部工具切到 v2 引擎，**参数名保持兼容**
// （x/z/radius/type/dimension/from_x/... 都照旧），机器人已有的调用不用改。
// ═══════════════════════════════════════════════════════════════════════

import (
	"fmt"
	"strconv"
	"strings"
)

// yySeedV2ToolCall 处理 v2 覆盖的 seed_* 工具。
// 返回 handled=false 表示不归 v2 管（例如 seed_coordinates），调用方继续走旧实现。
func yySeedV2ToolCall(name string, args map[string]any) (map[string]any, bool, error) {
	switch name {
	case "seed_manifest", "seed_slime", "seed_stronghold", "seed_structure",
		"seed_biome", "seed_query", "seed_heat":
	default:
		return nil, false, nil
	}
	if args == nil {
		args = map[string]any{}
	}
	out, err := yySeedV2ToolRun(name, args)
	return out, true, err
}

func yySeedV2ToolRun(name string, args map[string]any) (map[string]any, error) {
	if name == "seed_manifest" {
		m := yySeedV2Manifest()
		m["note"] = "全部在本机离线计算。exact 表示与游戏原版逐位一致；candidate 表示位置公式对但缺生物群系过滤，需进游戏确认。"
		return m, nil
	}
	if name == "seed_heat" {
		return yySeedV2ToolHeat(args)
	}

	seed, err := yySeedV2ToolSeed(args)
	if err != nil {
		return nil, err
	}
	edition := yyV2EditionJava
	versionRaw := yySeedV2ToolStr(args, "version")
	if versionRaw == "" {
		versionRaw = yySeedV2DefaultVersion
	}
	versionCode := yyV2ParseVersion(versionRaw, edition)
	dim := yyV2NormalizeDimension(yySeedV2ToolStr(args, "dimension"))
	if d := yySeedV2ToolStr(args, "dim"); d != "" {
		dim = yyV2NormalizeDimension(d)
	}

	switch name {
	case "seed_slime":
		radius := yySeedV2ToolInt(args, "radius", 8)
		radius = yySeedV2Clamp(radius, 1, yySeedV2MaxSlimeRadius)
		cx, cz := yySeedV2ToolInt(args, "x", 0), yySeedV2ToolInt(args, "z", 0)
		chunks := make([]map[string]int, 0, radius*2)
		for dz := -radius; dz <= radius; dz++ {
			for dx := -radius; dx <= radius; dx++ {
				if yyV2IsSlimeChunkV2(seed, int32(cx+dx), int32(cz+dz)) {
					chunks = append(chunks, map[string]int{"x": cx + dx, "z": cz + dz})
				}
			}
		}
		return map[string]any{
			"ok": true, "seed": seed, "x": cx, "z": cz, "radius": radius,
			"chunks": chunks, "count": len(chunks), "precision": "exact",
			"note": "区块坐标；方块坐标 = 区块*16+8",
		}, nil

	case "seed_stronghold":
		count := yySeedV2ToolInt(args, "count", 3)
		count = yySeedV2Clamp(count, 1, 128)
		all := yyV2StrongholdsV2(seed, versionCode)
		if count < len(all) {
			all = all[:count]
		}
		points := make([]map[string]int, 0, len(all))
		for _, p := range all {
			points = append(points, map[string]int{"x": p[0], "z": p[1]})
		}
		return map[string]any{
			"ok": true, "seed": seed, "edition": edition, "version": versionRaw,
			"points": points, "count": len(points), "precision": "exact",
			"note": "cubiomes 逐位复刻的 128 个要塞环位；区块坐标，方块坐标 = 区块*16+8",
		}, nil

	case "seed_structure":
		typ := strings.TrimSpace(yySeedV2ToolStr(args, "type"))
		if typ == "" {
			return nil, fmt.Errorf("缺少 type：可选 %s", yySeedV2ToolTypeHint(dim, versionCode, edition))
		}
		info, ok := yyV2StructureInfoByID(typ, versionCode, edition)
		if !ok {
			return nil, fmt.Errorf("不支持的结构类型；可选 %s", yySeedV2ToolTypeHint(dim, versionCode, edition))
		}
		if info.Dim != dim && yySeedV2ToolStr(args, "dimension") == "" && yySeedV2ToolStr(args, "dim") == "" {
			dim = info.Dim // 没显式给维度时按结构自身维度走，比默认主世界友好
		}
		if info.Dim != dim {
			return nil, fmt.Errorf("%s 属于 %s 维度，请把 dimension 设为 %s", info.Name, yySeedV2DimLabel(info.Dim), info.Dim)
		}
		// from/to 是老的写法（区块跨度），新前端用 cx/cz/r，两种都吃
		var cx0, cz0, cx1, cz1 int
		if _, has := args["from_x"]; has {
			cx0 = yySeedV2ToolInt(args, "from_x", -256)
			cz0 = yySeedV2ToolInt(args, "from_z", -256)
			cx1 = yySeedV2ToolInt(args, "to_x", 256)
			cz1 = yySeedV2ToolInt(args, "to_z", 256)
		} else {
			cx := yySeedV2ToolInt(args, "x", 0)
			cz := yySeedV2ToolInt(args, "z", 0)
			r := yySeedV2Clamp(yySeedV2ToolInt(args, "radius", yySeedV2ToolInt(args, "r", 256)), 1, yySeedV2MaxRadius)
			cx0, cz0, cx1, cz1 = cx-r, cz-r, cx+r, cz+r
		}
		if cx1-cx0 > yySeedV2MaxRadius*2 {
			return nil, fmt.Errorf("查询范围过大：单轴区块跨度不能超过 %d", yySeedV2MaxRadius*2)
		}
		points := yyV2ScanStructures(seed, info.ID, versionCode, edition, cx0, cz0, cx1, cz1, 2000)
		truncated := len(points) >= 2000
		return map[string]any{
			"ok": true, "seed": seed, "type": info.ID, "label": info.Name,
			"dimension": info.Dim, "version": versionRaw,
			"range":  map[string]int{"cx0": cx0, "cz0": cz0, "cx1": cx1, "cz1": cz1},
			"chunks": points, "count": len(points), "truncated": truncated,
			"precision": info.Precision,
			"note":      yySeedV2ToolPrecisionNote(info.Precision),
		}, nil

	case "seed_biome":
		gen := yyV2NewBiomeGen(seed, versionCode, yySeedV2ToolBool(args, "large"))
		if dim != yyV2DimOverworld {
			return map[string]any{
				"ok": true, "seed": seed, "dimension": dim,
				"name": yySeedV2DimLabel(dim) + "代表群系", "precision": "representative",
				"note": "下界/末地的群系不是气候噪声生成的；这里只给该维度的代表群系，不逐点计算",
			}, nil
		}
		if _, has := args["from_x"]; has {
			fromX, fromZ := yySeedV2ToolInt(args, "from_x", 0), yySeedV2ToolInt(args, "from_z", 0)
			toX, toZ := yySeedV2ToolInt(args, "to_x", fromX), yySeedV2ToolInt(args, "to_z", fromZ)
			step := yySeedV2Clamp(yySeedV2ToolInt(args, "step", 16), 1, 128)
			if toX < fromX {
				fromX, toX = toX, fromX
			}
			if toZ < fromZ {
				fromZ, toZ = toZ, fromZ
			}
			points := make([]map[string]any, 0, 64)
			truncated := false
			for cz := fromZ; cz <= toZ; cz += step {
				for cx := fromX; cx <= toX; cx += step {
					if len(points) >= yySeedV2MaxPoints {
						truncated = true
						break
					}
					points = append(points, yySeedV2BiomePoint(gen, cx, cz))
				}
				if truncated {
					break
				}
			}
			return map[string]any{
				"ok": true, "seed": seed, "dimension": dim, "version": versionRaw,
				"step": step, "points": points, "count": len(points),
				"truncated": truncated, "precision": "exact",
			}, nil
		}
		cx, cz := yySeedV2ToolInt(args, "x", 0), yySeedV2ToolInt(args, "z", 0)
		point := yySeedV2BiomePoint(gen, cx, cz)
		return map[string]any{
			"ok": true, "seed": seed, "x": cx, "z": cz, "dimension": dim, "version": versionRaw,
			"biome": point["key"], "biome_id": point["biome"], "name": point["name"],
			"color": point["color"], "precision": "exact",
		}, nil

	case "seed_query":
		cx, cz := yySeedV2ToolInt(args, "x", 0), yySeedV2ToolInt(args, "z", 0)
		radius := yySeedV2Clamp(yySeedV2ToolInt(args, "radius", 16), 1, 512)
		res := yyV2SeedMapSummary(seed, versionCode, edition, dim, cx, cz, radius)
		if res == nil {
			res = map[string]any{}
		}
		res["ok"] = true
		res["seed"] = seed
		res["x"], res["z"], res["radius"] = cx, cz, radius
		res["dimension"] = dim
		// 机器人要「一句能直接念给群友听的话」。以前这里只回结构化 map，
		// 模型拿不到 summary 就只能念 JSON（工具描述里写的是「返回中文 summary」）。
		res["summary"] = yySeedV2SummaryLine(seed, res, cx, cz)
		return res, nil
	}
	return nil, fmt.Errorf("未知种子工具：%s", name)
}

// yySeedV2ToolHeat 结构热点：把范围切格，返回 [gx,gz,count]（只含 count>0）。
func yySeedV2ToolHeat(args map[string]any) (map[string]any, error) {
	seed, err := yySeedV2ToolSeed(args)
	if err != nil {
		return nil, err
	}
	versionRaw := yySeedV2ToolStr(args, "version")
	if versionRaw == "" {
		versionRaw = yySeedV2DefaultVersion
	}
	versionCode := yyV2ParseVersion(versionRaw, yyV2EditionJava)
	dim := yyV2NormalizeDimension(yySeedV2ToolStr(args, "dimension"))

	fromX := yySeedV2ToolInt(args, "from_x", -256)
	fromZ := yySeedV2ToolInt(args, "from_z", -256)
	toX := yySeedV2ToolInt(args, "to_x", 256)
	toZ := yySeedV2ToolInt(args, "to_z", 256)
	if toX-fromX > yySeedV2MaxRadius*2 || toZ-fromZ > yySeedV2MaxRadius*2 {
		return nil, fmt.Errorf("范围过大：单轴区块跨度不能超过 %d", yySeedV2MaxRadius*2)
	}
	cell := yySeedV2Clamp(yySeedV2ToolInt(args, "cell", 64), 16, 512)

	typ := strings.TrimSpace(yySeedV2ToolStr(args, "type"))
	var ids []string
	if typ == "" || typ == "all" {
		ids = append(ids, yySeedV2DefaultTypes[dim]...)
	} else if info, ok := yyV2StructureInfoByID(typ, versionCode, yyV2EditionJava); ok {
		ids = []string{info.ID}
	} else {
		return nil, fmt.Errorf("不支持的结构类型；可选 %s", yySeedV2ToolTypeHint(dim, versionCode, yyV2EditionJava))
	}

	type cellKey struct{ gx, gz int }
	counts := map[cellKey]int{}
	total := 0
	for _, id := range ids {
		pts := yyV2ScanStructures(seed, id, versionCode, yyV2EditionJava, fromX, fromZ, toX, toZ, 4000)
		for _, p := range pts {
			key := cellKey{floorDiv(p.X-fromX, cell), floorDiv(p.Z-fromZ, cell)}
			counts[key]++
			total++
		}
	}
	cells := make([][3]int, 0, len(counts))
	for k, v := range counts {
		if v <= 0 {
			continue
		}
		cells = append(cells, [3]int{k.gx, k.gz, v})
	}
	// 稳定排序：先 gz 后 gx，保证同一输入两次调用结果一样
	for i := 1; i < len(cells); i++ {
		for j := i; j > 0; j-- {
			left, right := cells[j-1], cells[j]
			if left[1] < right[1] || (left[1] == right[1] && left[0] <= right[0]) {
				break
			}
			cells[j-1], cells[j] = right, left
		}
	}
	return map[string]any{
		"ok": true, "seed": seed, "dimension": dim, "version": versionRaw,
		"types": ids, "cell": cell,
		"origin": map[string]int{"x": fromX, "z": fromZ},
		"cells":  cells, "total": total,
		"note": "每格是 [格x, 格z, 数量]，格坐标乘以 cell 再加 origin 就是区块坐标",
	}, nil
}

// floorDiv 向负无穷取整（Go 的 / 是向零取整，负数会算错格子）。
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// ── 小工具 ────────────────────────────────────────────────────────────

func yySeedV2ToolSeed(args map[string]any) (int64, error) {
	v, ok := args["seed"]
	if !ok || v == nil {
		return 0, fmt.Errorf("缺少 seed 参数")
	}
	n, err := yySeedV2AnyInt64(v)
	if err != nil {
		return 0, fmt.Errorf("seed 必须是整数")
	}
	return n, nil
}

func yySeedV2AnyInt64(v any) (int64, error) {
	switch n := v.(type) {
	case float64:
		return int64(n), nil
	case float32:
		return int64(n), nil
	case int:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case string:
		var out int64
		var neg bool
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, fmt.Errorf("empty")
		}
		if s[0] == '-' {
			neg = true
			s = s[1:]
		}
		if s == "" {
			return 0, fmt.Errorf("not a number")
		}
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return 0, fmt.Errorf("not a number")
			}
			out = out*10 + int64(s[i]-'0')
		}
		if neg {
			out = -out
		}
		return out, nil
	}
	return 0, fmt.Errorf("not a number")
}

func yySeedV2ToolInt(args map[string]any, key string, def int) int {
	if v, ok := args[key]; ok && v != nil {
		if n, err := yySeedV2AnyInt64(v); err == nil {
			return int(n)
		}
	}
	return def
}

func yySeedV2ToolStr(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func yySeedV2ToolBool(args map[string]any, key string) bool {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case bool:
			return n
		case float64:
			return n != 0
		case string:
			return yySeedV2Bool(n)
		}
	}
	return false
}

func yySeedV2DimLabel(dim string) string {
	switch dim {
	case yyV2DimNether:
		return "下界"
	case yyV2DimEnd:
		return "末地"
	}
	return "主世界"
}

func yySeedV2ToolTypeHint(dim string, versionCode int32, edition string) string {
	list := yyV2StructureList(versionCode, edition)
	out := make([]string, 0, len(list))
	for _, info := range list {
		if info.Dim != dim {
			continue
		}
		out = append(out, info.ID+"("+info.Name+")")
	}
	if len(out) == 0 {
		return "该版本该维度没有可用结构"
	}
	return strings.Join(out, "、")
}

func yySeedV2ToolPrecisionNote(precision string) string {
	if precision == "exact" {
		return "exact：位置算法与游戏原版逐位一致，可直接照着传送"
	}
	return "candidate：位置公式对，但缺生物群系过滤，进游戏可能偏移，建议先传送到附近再找"
}

// yySeedV2SummaryLine 把结构化摘要拼成一句中文，给机器人直接念。
// 不编数字：只用手上真有的 terrain / nearest。
func yySeedV2SummaryLine(seed int64, res map[string]any, cx, cz int) string {
	terrain, _ := res["terrain"].(string)
	if terrain == "" {
		terrain = "地形未知"
	}
	line := "种子 " + strconv.FormatInt(seed, 10) + " 在 (" + strconv.Itoa(cx*16+8) + "," + strconv.Itoa(cz*16+8) + ") 一带" + terrain
	nearest, _ := res["nearest"].([]map[string]any)
	if len(nearest) == 0 {
		return line + "，附近没查到已支持的结构。"
	}
	top := nearest[0]
	name, _ := top["name"].(string)
	dist, _ := top["dist_blocks"].(int)
	bx, _ := top["block_x"].(int)
	bz, _ := top["block_z"].(int)
	if name == "" {
		return line + "。"
	}
	return line + "；最近的是" + name + "，在方块坐标 (" +
		strconv.Itoa(bx) + "," + strconv.Itoa(bz) + ")，约 " + strconv.Itoa(dist) + " 格。"
}
