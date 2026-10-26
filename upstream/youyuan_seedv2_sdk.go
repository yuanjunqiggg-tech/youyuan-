package main

// ═══════════════════════════════════════════════════════════════════════
// 种子查询 v2 · 插件 SDK 接口
//
// 插件用 `api.call("seed.structures", {...})` 调，走的是和 HTTP 完全同一套实现
// （yyV2ScanStructures / yyV2NewBiomeGen / yyV2IsSlimeChunkV2 / yyV2StrongholdsV2），
// 不复制算法、不联网、不需要机器人连接。
//
// 注册方式与 building.* / chain.* 一致：写进 yyExtraMethods，由 yySenseCall 分派。
// ═══════════════════════════════════════════════════════════════════════

import (
	"fmt"
	"strconv"
	"strings"
)

var yySeedV2Methods = map[string]string{
	"seed.manifest":   "无参数；返回版本表、维度、结构清单与引擎自检（btree 是否装载）。不联网、不读用户文件。",
	"seed.structures": "{seed,version=1.21.4,edition=java,dim=overworld,types='village,mansion',cx=0,cz=0,r=256,limit=20000}；返回区块坐标的结构点。precision=exact 的结构可复现，candidate 的需要进游戏确认。范围过大自动截断并给 truncated=true。",
	"seed.biome":      "{seed,version,cx,cz} 或 {seed,version,points:[[cx,cz],...]}；返回生物群系 id/key/中文名/RGB。仅主世界走气候噪声，下界/末界给代表群系并注明。",
	"seed.slime":      "{seed,cx=0,cz=0,r=16}；Java 版史莱姆区块，exact。基岩版算法不同，明确拒绝而不是给错结果。",
	"seed.stronghold": "{seed,version,count=128}；要塞区块坐标，cubiomes 逐位复刻，exact。",
	"seed.summary":    "{seed,version,dim,cx,cz,r}；首屏摘要 + 最近结构，给面板/机器人回话用。",
}

func init() {
	for method, doc := range yySeedV2Methods {
		yyExtraMethods[method] = doc
	}
}

// yySeedV2Call 插件 SDK 分派入口。args[0] 必须是参数对象（可省略）。
func yySeedV2Call(method string, args []any) (any, error) {
	if _, ok := yySeedV2Methods[method]; !ok {
		return nil, fmt.Errorf("未知种子接口：%s", method)
	}
	opt := map[string]any{}
	if len(args) > 0 && args[0] != nil {
		given, ok := args[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s 需要参数对象", method)
		}
		for k, v := range given {
			opt[k] = v
		}
	}
	if method == "seed.manifest" {
		return yySeedV2Manifest(), nil
	}

	seed, err := yySeedV2OptInt64(opt, "seed")
	if err != nil {
		return nil, err
	}
	edition := yyV2NormalizeEdition(yySeedV2OptString(opt, "edition", yyV2EditionJava))
	versionRaw := yySeedV2OptString(opt, "version", yySeedV2DefaultVersion)
	versionCode := yyV2ParseVersion(versionRaw, edition)
	dim := yyV2NormalizeDimension(yySeedV2OptString(opt, "dim", yyV2DimOverworld))
	large := yySeedV2OptBool(opt, "large")

	switch method {
	case "seed.structures":
		cx, err := yySeedV2OptInt(opt, "cx", 0)
		if err != nil {
			return nil, err
		}
		cz, err := yySeedV2OptInt(opt, "cz", 0)
		if err != nil {
			return nil, err
		}
		radius, err := yySeedV2OptInt(opt, "r", 256)
		if err != nil {
			return nil, err
		}
		limit, err := yySeedV2OptInt(opt, "limit", 4000)
		if err != nil {
			return nil, err
		}
		radius = yySeedV2Clamp(radius, 1, yySeedV2MaxRadius)
		limit = yySeedV2Clamp(limit, 1, yySeedV2MaxLimit)
		ids := yySeedV2OptTypes(opt, dim, versionCode, edition)
		out := map[string][]yyV2StructPos{}
		counts := map[string]int{}
		total := 0
		truncated := false
		for _, id := range ids {
			remain := limit - total
			if remain <= 0 {
				truncated = true
				break
			}
			got := yyV2ScanStructures(seed, id, versionCode, edition,
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
		return map[string]any{
			"seed": seed, "version": versionRaw, "version_code": versionCode,
			"edition": edition, "dim": dim,
			"range":     map[string]int{"cx0": cx - radius, "cz0": cz - radius, "cx1": cx + radius, "cz1": cz + radius},
			"structures": out, "counts": counts, "total": total, "truncated": truncated, "types": ids,
			"note": "坐标为区块坐标；方块坐标 = 区块*16+8",
		}, nil

	case "seed.biome":
		if dim != yyV2DimOverworld {
			key, name := "minecraft:nether_wastes", "下界荒地"
			if dim == yyV2DimEnd {
				key, name = "minecraft:the_end", "末地"
			}
			return map[string]any{
				"biomes": []map[string]any{{"biome": 0, "key": key, "name": name}},
				"note":   "下界/末地的群系不是气候噪声生成的，这里只给该维度的代表群系",
			}, nil
		}
		gen := yyV2NewBiomeGen(seed, versionCode, large)
		pts, err := yySeedV2OptPoints(opt)
		if err != nil {
			return nil, err
		}
		if len(pts) == 0 {
			cx, _ := yySeedV2OptInt(opt, "cx", 0)
			cz, _ := yySeedV2OptInt(opt, "cz", 0)
			pts = [][2]int{{cx, cz}}
		}
		if len(pts) > yySeedV2MaxPoints {
			pts = pts[:yySeedV2MaxPoints]
		}
		list := make([]map[string]any, 0, len(pts))
		for _, p := range pts {
			list = append(list, yySeedV2BiomePoint(gen, p[0], p[1]))
		}
		return map[string]any{"biomes": list, "count": len(list)}, nil

	case "seed.slime":
		if edition != yyV2EditionJava {
			return nil, fmt.Errorf("史莱姆区块目前只支持 Java 版；基岩版算法不同，未实现")
		}
		cx, _ := yySeedV2OptInt(opt, "cx", 0)
		cz, _ := yySeedV2OptInt(opt, "cz", 0)
		radius, _ := yySeedV2OptInt(opt, "r", 16)
		radius = yySeedV2Clamp(radius, 1, yySeedV2MaxSlimeRadius)
		chunks := make([]map[string]int, 0, radius*2)
		for dz := -radius; dz <= radius; dz++ {
			for dx := -radius; dx <= radius; dx++ {
				if yyV2IsSlimeChunkV2(seed, int32(cx+dx), int32(cz+dz)) {
					chunks = append(chunks, map[string]int{"x": cx + dx, "z": cz + dz})
				}
			}
		}
		return map[string]any{"chunks": chunks, "count": len(chunks), "precision": "exact"}, nil

	case "seed.stronghold":
		count, _ := yySeedV2OptInt(opt, "count", 128)
		count = yySeedV2Clamp(count, 1, 128)
		all := yyV2StrongholdsV2(seed, versionCode)
		if count < len(all) {
			all = all[:count]
		}
		list := make([]map[string]int, 0, len(all))
		for _, p := range all {
			list = append(list, map[string]int{"x": p[0], "z": p[1]})
		}
		return map[string]any{"strongholds": list, "count": len(list), "precision": "exact"}, nil

	case "seed.summary":
		cx, _ := yySeedV2OptInt(opt, "cx", 0)
		cz, _ := yySeedV2OptInt(opt, "cz", 0)
		radius, _ := yySeedV2OptInt(opt, "r", 256)
		radius = yySeedV2Clamp(radius, 16, 512)
		res := yyV2SeedMapSummary(seed, versionCode, edition, dim, cx, cz, radius)
		if res == nil {
			res = map[string]any{}
		}
		res["seed"] = seed
		res["version"] = versionRaw
		res["edition"] = edition
		res["dim"] = dim
		return res, nil
	}
	return nil, fmt.Errorf("未实现的种子接口：%s", method)
}

// ── 参数读取（插件传进来的数字可能是 float64 / int / string，全部吃下）──

func yySeedV2OptString(opt map[string]any, key, def string) string {
	if v, ok := opt[key].(string); ok {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return def
}

func yySeedV2OptBool(opt map[string]any, key string) bool {
	switch v := opt[key].(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return yySeedV2Bool(v)
	}
	return false
}

func yySeedV2OptInt(opt map[string]any, key string, def int) (int, error) {
	raw, ok := opt[key]
	if !ok || raw == nil {
		return def, nil
	}
	switch v := raw.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return def, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("%s 必须是整数", key)
		}
		return n, nil
	}
	return 0, fmt.Errorf("%s 必须是整数", key)
}

func yySeedV2OptInt64(opt map[string]any, key string) (int64, error) {
	raw, ok := opt[key]
	if !ok || raw == nil {
		return 0, fmt.Errorf("缺少 %s", key)
	}
	switch v := raw.(type) {
	case float64:
		return int64(v), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		s := strings.TrimSpace(v)
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s 必须是整数", key)
		}
		return n, nil
	}
	return 0, fmt.Errorf("%s 必须是整数", key)
}

// yySeedV2OptTypes 解析 types（字符串逗号分隔，或数组），并过滤到该版本+维度可用集合。
func yySeedV2OptTypes(opt map[string]any, dim string, versionCode int32, edition string) []string {
	var raw []string
	switch v := opt["types"].(type) {
	case string:
		for _, part := range strings.Split(v, ",") {
			if p := strings.TrimSpace(part); p != "" {
				raw = append(raw, p)
			}
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				raw = append(raw, strings.TrimSpace(s))
			}
		}
	case []string:
		raw = append(raw, v...)
	}
	if len(raw) == 0 {
		raw = append(raw, yySeedV2DefaultTypes[dim]...)
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, id := range raw {
		info, ok := yyV2StructureInfoByID(id, versionCode, edition)
		if !ok || info.Dim != dim || seen[info.ID] {
			continue
		}
		seen[info.ID] = true
		out = append(out, info.ID)
		if len(out) >= yySeedV2MaxTypes {
			break
		}
	}
	return out
}

// yySeedV2OptPoints 读 points:[[x,z],...] 或 points:[{x,z},...]。
func yySeedV2OptPoints(opt map[string]any) ([][2]int, error) {
	raw, ok := opt["points"]
	if !ok || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("points 必须是数组")
	}
	out := make([][2]int, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case []any:
			if len(v) < 2 {
				return nil, fmt.Errorf("points 里的每一项至少要有 x 和 z")
			}
			x, err1 := yySeedV2AnyInt(v[0])
			z, err2 := yySeedV2AnyInt(v[1])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("points 坐标必须是整数")
			}
			out = append(out, [2]int{x, z})
		case map[string]any:
			x, _ := yySeedV2OptInt(v, "x", 0)
			z, _ := yySeedV2OptInt(v, "z", 0)
			out = append(out, [2]int{x, z})
		case []int:
			if len(v) < 2 {
				return nil, fmt.Errorf("points 里的每一项至少要有 x 和 z")
			}
			out = append(out, [2]int{v[0], v[1]})
		default:
			return nil, fmt.Errorf("points 里的每一项必须是 [x,z] 或 {x,z}")
		}
	}
	return out, nil
}

func yySeedV2AnyInt(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	case int64:
		return int(n), nil
	}
	return 0, fmt.Errorf("not int")
}
