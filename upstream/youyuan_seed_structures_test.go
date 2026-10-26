package main

// 结构注册表全量测试：覆盖「所有它的结构 / 不同维度 / 所有覆盖的版本」三条要求。
//
// 断言原则：
//   - 结构清单必须覆盖三个维度，ID 唯一，每个都带 since（版本）与 kind（放置类型）
//   - 维度与结构冲突必须报错；不传 dimension 时按结构自身维度自动推断
//   - 概率型结构（矿井/宝藏）必须诚实标 unsupported，不许编坐标
//   - 同心环结构（要塞）必须与 /api/seed/stronghold 的结果一致，不许两套算法
//   - manifest 必须给出完整版本矩阵，unsupported 的区间不许含糊

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

// 结构清单必须覆盖三维度、ID 唯一、必填字段齐全。
func TestStructureRegistryComplete(t *testing.T) {
	specs := yySeedStructureSpecs()
	if len(specs) != 21 {
		t.Fatalf("结构总数应为 21（主世界 17 + 下界 3 + 末地 1），实际 %d", len(specs))
	}
	seen := map[string]bool{}
	byDim := map[string]int{}
	for _, s := range specs {
		if s.ID == "" || s.Label == "" {
			t.Fatalf("结构缺 ID 或中文名：%+v", s)
		}
		if seen[s.ID] {
			t.Fatalf("结构 ID 重复：%s", s.ID)
		}
		seen[s.ID] = true
		byDim[s.Dimension]++
		switch s.Dimension {
		case yyStructDimOverworld, yyStructDimNether, yyStructDimEnd:
		default:
			t.Fatalf("%s 的维度非法：%q", s.ID, s.Dimension)
		}
		switch s.Kind {
		case yyStructKindSpread:
			if s.Spacing <= 0 || s.Separation < 0 || s.Separation >= s.Spacing {
				t.Fatalf("%s 的网格参数非法：spacing=%d separation=%d", s.ID, s.Spacing, s.Separation)
			}
		case yyStructKindConcentric, yyStructKindProbability:
		default:
			t.Fatalf("%s 的放置类型非法：%q", s.ID, s.Kind)
		}
		if s.Since == "" {
			t.Fatalf("%s 缺少 since（起始版本）", s.ID)
		}
		if s.Note == "" {
			t.Fatalf("%s 缺少中文说明", s.ID)
		}
	}
	if byDim[yyStructDimOverworld] != 17 {
		t.Fatalf("主世界结构应为 17 种，实际 %d", byDim[yyStructDimOverworld])
	}
	if byDim[yyStructDimNether] != 3 {
		t.Fatalf("下界结构应为 3 种，实际 %d", byDim[yyStructDimNether])
	}
	if byDim[yyStructDimEnd] != 1 {
		t.Fatalf("末地结构应为 1 种，实际 %d", byDim[yyStructDimEnd])
	}
}

// 别名必须解析到真实结构，且别名不能出现在清单里（避免 all 重复统计）。
func TestStructureAliases(t *testing.T) {
	cases := map[string]string{
		"temple":           "desert_pyramid",
		"witch_hut":        "swamp_hut",
		"outpost":          "pillager_outpost",
		"woodland_mansion": "mansion",
		"nether_fortress":  "fortress",
		"bastion_remnant":  "bastion",
		"endcity":          "end_city",
		"Ocean-Monument":   "monument",
		"  VILLAGE  ":      "village",
	}
	for alias, want := range cases {
		spec, ok := yySeedStructureSpecByID(alias)
		if !ok {
			t.Fatalf("别名 %q 应能解析", alias)
		}
		if spec.ID != want {
			t.Fatalf("别名 %q 应解析到 %s，实际 %s", alias, want, spec.ID)
		}
	}
	for _, s := range yySeedStructureSpecs() {
		for alias, target := range yySeedStructureAliases {
			if s.ID == alias {
				t.Fatalf("别名 %q 不应出现在清单里（会重复统计），它指向 %s", alias, target)
			}
		}
	}
	if _, ok := yySeedStructureSpecByID("不存在的结构"); ok {
		t.Fatal("未知结构应返回 false")
	}
}

// 维度过滤：每个维度只返回自己的结构。
func TestStructureDimensionFilter(t *testing.T) {
	ow := yySeedStructureSpecsByDim(yyStructDimOverworld)
	ne := yySeedStructureSpecsByDim(yyStructDimNether)
	ed := yySeedStructureSpecsByDim("end") // 别名写法
	if len(ow) != 17 || len(ne) != 3 || len(ed) != 1 {
		t.Fatalf("维度过滤结果不对：overworld=%d nether=%d end=%d", len(ow), len(ne), len(ed))
	}
	for _, s := range ne {
		if s.Dimension != yyStructDimNether {
			t.Fatalf("%s 不该出现在下界清单", s.ID)
		}
	}
	if ed[0].ID != "end_city" {
		t.Fatalf("末地只有末地城，实际 %s", ed[0].ID)
	}
	if got := len(yySeedStructureSpecsByDim("all")); got != 21 {
		t.Fatalf("all 应返回全部 21 种，实际 %d", got)
	}
	if yySeedDimensionLabel(yyStructDimNether) != "下界" ||
		yySeedDimensionLabel(yyStructDimEnd) != "末地" ||
		yySeedDimensionLabel("") != "主世界" {
		t.Fatal("维度中文名不对")
	}
}

// 不传 dimension 时按结构自身维度推断；显式冲突才报错。
func TestStructureDimensionInference(t *testing.T) {
	mux := yySeedTestMux()

	// 下界要塞不传 dimension 应能直接查
	code, body := yySeedTestGet(t, mux, "/api/seed/structure?seed=12345&type=fortress&from_x=-512&from_z=-512&to_x=512&to_z=512&lod=0")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("下界要塞应自动按 nether 推断，实际 %d %v", code, body)
	}
	if body["dimension"] != yyStructDimNether {
		t.Fatalf("自动推断的维度应为 nether，实际 %v", body["dimension"])
	}
	if body["dimension_label"] != "下界" {
		t.Fatalf("维度中文名应为下界，实际 %v", body["dimension_label"])
	}

	// 末地城同理
	code, body = yySeedTestGet(t, mux, "/api/seed/structure?seed=12345&type=end_city&from_x=-512&from_z=-512&to_x=512&to_z=512&lod=0")
	if code != http.StatusOK || body["dimension"] != yyStructDimEnd {
		t.Fatalf("末地城应自动按 the_end 推断，实际 %d %v", code, body)
	}

	// 显式冲突必须拒绝
	code, body = yySeedTestGet(t, mux, "/api/seed/structure?seed=12345&type=fortress&dimension=overworld")
	if code != http.StatusBadRequest || body["ok"] != false {
		t.Fatalf("下界要塞 + overworld 应 400，实际 %d %v", code, body)
	}
	if msg, _ := body["error"].(string); msg == "" {
		t.Fatal("冲突报错必须给中文原因")
	}

	// 显式正确维度应通过
	code, body = yySeedTestGet(t, mux, "/api/seed/structure?seed=12345&type=bastion&dimension=nether&from_x=-512&from_z=-512&to_x=512&to_z=512&lod=0")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("堡垒遗迹 + nether 应通过，实际 %d %v", code, body)
	}
}

// 概率型结构必须诚实标 unsupported，且不给坐标。
func TestStructureProbabilityUnsupported(t *testing.T) {
	mux := yySeedTestMux()
	for _, typ := range []string{"mineshaft", "buried_treasure"} {
		code, body := yySeedTestGet(t, mux, "/api/seed/structure?seed=0&type="+typ+"&from_x=-128&from_z=-128&to_x=128&to_z=128&lod=2")
		if code != http.StatusOK || body["ok"] != true {
			t.Fatalf("%s 应正常返回，实际 %d %v", typ, code, body)
		}
		if body["precision"] != "unsupported" {
			t.Fatalf("%s 精度应为 unsupported，实际 %v", typ, body["precision"])
		}
		if body["kind"] != yyStructKindProbability {
			t.Fatalf("%s 放置类型应为 probability，实际 %v", typ, body["kind"])
		}
		if int(body["count"].(float64)) != 0 {
			t.Fatalf("%s 不该给出候选坐标，实际 count=%v", typ, body["count"])
		}
		if p, _ := body["probability"].(float64); p <= 0 || p >= 1 {
			t.Fatalf("%s 概率应在 (0,1) 内，实际 %v", typ, body["probability"])
		}
	}
}

// 同心环结构（要塞）必须与 /api/seed/stronghold 完全一致，不许两套算法。
func TestStructureStrongholdMatchesDedicatedEndpoint(t *testing.T) {
	mux := yySeedTestMux()
	_, base := yySeedTestGet(t, mux, "/api/seed/stronghold?seed=12345&count=3")
	pts := base["points"].([]any)
	if len(pts) != 3 {
		t.Fatalf("要塞应有 3 个点，实际 %d", len(pts))
	}
	_, body := yySeedTestGet(t, mux, "/api/seed/structure?seed=12345&type=stronghold&from_x=-600&from_z=-600&to_x=600&to_z=600&lod=2")
	if body["precision"] != "approximate" {
		t.Fatalf("要塞精度应为 approximate，实际 %v", body["precision"])
	}
	if body["kind"] != yyStructKindConcentric {
		t.Fatalf("要塞放置类型应为 concentric，实际 %v", body["kind"])
	}
	got := map[[2]int]bool{}
	for _, c := range body["chunks"].([]any) {
		p := c.([]any)
		got[[2]int{int(p[0].(float64)), int(p[1].(float64))}] = true
	}
	// 结构接口按上限（16）返回全部要塞，专用接口 count=3 只取前 3 个环；
	// 前 3 个必须是它的子集，证明两者用的是同一套算法。
	if len(got) != yySeedMaxStrongholds {
		t.Fatalf("要塞点应为上限 %d 个，实际 %d", yySeedMaxStrongholds, len(got))
	}
	for _, p := range pts {
		m := p.(map[string]any)
		key := [2]int{int(m["chunk_x"].(float64)), int(m["chunk_z"].(float64))}
		if !got[key] {
			t.Fatalf("要塞点 %v 未出现在结构接口结果里", key)
		}
	}
}

// 热点图：下界/末地维度也要能出格子，概率型结构不参与。
func TestStructureHeatDimensions(t *testing.T) {
	mux := yySeedTestMux()
	for _, dim := range []string{yyStructDimNether, yyStructDimEnd} {
		code, body := yySeedTestGet(t, mux, "/api/seed/heat?seed=12345&type=all&dimension="+dim+"&from_x=-512&from_z=-512&to_x=512&to_z=512&lod=2")
		if code != http.StatusOK || body["ok"] != true {
			t.Fatalf("%s 热点图应正常返回，实际 %d %v", dim, code, body)
		}
		types := body["types"].([]any)
		want := len(yySeedStructureSpecsByDim(dim))
		if len(types) != want {
			t.Fatalf("%s 热点图应含 %d 种结构，实际 %v", dim, want, types)
		}
		for _, tv := range types {
			id, _ := tv.(string)
			spec, ok := yySeedStructureSpecByID(id)
			if !ok {
				t.Fatalf("%s 热点图出现未知结构 %q", dim, id)
			}
			if spec.Kind == yyStructKindProbability {
				t.Fatalf("%s 是概率型结构，不该出现在热点图里", spec.ID)
			}
			if spec.Dimension != dim {
				t.Fatalf("%s 不该出现在 %s 维度热点图里", spec.ID, dim)
			}
		}
	}
	// 主世界热点图不含概率型结构
	_, body := yySeedTestGet(t, mux, "/api/seed/heat?seed=12345&type=all&from_x=-256&from_z=-256&to_x=256&to_z=256&lod=2")
	for _, tv := range body["types"].([]any) {
		id, _ := tv.(string)
		if id == "mineshaft" || id == "buried_treasure" {
			t.Fatalf("主世界热点图不该含概率型结构：%s", id)
		}
	}
}

// 聚合查询按维度切：下界/末地没有史莱姆区块与要塞。
func TestQueryDimension(t *testing.T) {
	mux := yySeedTestMux()
	code, body := yySeedTestGet(t, mux, "/api/seed/query?seed=12345&x=0&z=0&radius=16&lod=0&dimension=nether")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("下界聚合查询应正常，实际 %d %v", code, body)
	}
	if body["dimension"] != yyStructDimNether || body["dimension_label"] != "下界" {
		t.Fatalf("维度字段不对：%v / %v", body["dimension"], body["dimension_label"])
	}
	if n := int(body["slime"].(map[string]any)["count"].(float64)); n != 0 {
		t.Fatalf("下界没有史莱姆区块，实际 %d", n)
	}
	if n := len(body["strongholds"].([]any)); n != 0 {
		t.Fatalf("下界没有要塞，实际 %d", n)
	}
	structs := body["structures"].([]any)
	if len(structs) != 3 {
		t.Fatalf("下界应有 3 种结构，实际 %d", len(structs))
	}
	if msg, _ := body["summary"].(string); msg == "" {
		t.Fatal("聚合查询必须给中文 summary")
	}
}

// manifest 必须给出结构清单、维度清单与完整版本矩阵。
func TestManifestCoverage(t *testing.T) {
	m := yySeedManifest()

	structs := m["structures"].([]map[string]any)
	if len(structs) != 21 {
		t.Fatalf("manifest.structures 应为 21 条，实际 %d", len(structs))
	}
	dims := m["dimensions"].([]map[string]any)
	if len(dims) != 3 {
		t.Fatalf("manifest.dimensions 应为 3 条，实际 %d", len(dims))
	}
	vers := m["version_matrix"].([]map[string]any)
	want := []string{"1.7-1.12", "1.13-1.17", "1.18-1.18.2", "1.19-1.19.4", "1.20-1.20.6", "1.21+", "bedrock"}
	if len(vers) != len(want) {
		t.Fatalf("版本矩阵应有 %d 档，实际 %d", len(want), len(vers))
	}
	for i, w := range want {
		if vers[i]["id"] != w {
			t.Fatalf("版本矩阵第 %d 档应为 %s，实际 %v", i, w, vers[i]["id"])
		}
		if vers[i]["note"] == "" {
			t.Fatalf("版本 %s 缺中文说明", w)
		}
	}
	// 基岩版必须明确 unsupported
	if vers[len(vers)-1]["biome_precision"] != "unsupported" ||
		vers[len(vers)-1]["structure_precision"] != "unsupported" {
		t.Fatal("基岩版必须标 unsupported，不许冒充支持")
	}
	// 1.7-1.12 结构必须 unsupported（参数与 1.13+ 不同）
	if vers[0]["structure_precision"] != "unsupported" {
		t.Fatal("1.7-1.12 结构参数与现在不同，必须标 unsupported")
	}

	// 前端版本下拉要的是真实游戏版本，不能拿区间矩阵顶替
	gv := m["game_versions"].([]string)
	if len(gv) < 10 {
		t.Fatalf("game_versions 至少应给 10 个真实版本，实际 %d", len(gv))
	}
	for _, must := range []string{"1.21", "1.20", "1.19", "1.18", "1.17", "1.16", "1.13", "1.7"} {
		found := false
		for _, v := range gv {
			if v == must {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("game_versions 必须含算法边界版本 %s，实际 %v", must, gv)
		}
	}
}

// 前端结构表必须与后端注册表对齐：后端加了结构而前端没加，图钉就永远画不出来。
// 这是「写了没接线」的自动化检查，不靠人记。
func TestFrontendStructureIDsMatchBackend(t *testing.T) {
	path := "frontend/js/features/seed_map.js"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读不到前端结构表 %s：%v", path, err)
	}
	js := string(raw)
	for _, spec := range yySeedStructureSpecs() {
		// 只查 YSM_STRUCT_META 里有没有这个 ID（形如 `  village:  {`）
		if !strings.Contains(js, "\n  "+spec.ID+":") {
			t.Fatalf("前端 YSM_STRUCT_META 缺结构 %q（后端有、前端没有 → 该结构永远画不出来）", spec.ID)
		}
	}
	// 反向：前端不能留着后端已不认的旧 ID
	for _, dead := range []string{"temple:", "endcity:"} {
		if strings.Contains(js, "\n  "+dead) {
			t.Fatalf("前端还留着后端已改名的旧 ID %q，图钉会画不出来", dead)
		}
	}
}

// 前端群系色板必须覆盖后端全部群系键，否则那一类会画成中性灰（等于没渲染）。
func TestFrontendBiomePaletteCoversBackend(t *testing.T) {
	raw, err := os.ReadFile("frontend/js/features/seed_map.js")
	if err != nil {
		t.Fatalf("读不到前端色板：%v", err)
	}
	js := string(raw)
	exact := jsSection(js, "var YSM_BIOME_EXACT = {", "};")
	patSection := jsSection(js, "var YSM_BIOME_MATCH = [", "];")

	// 抽 YSM_BIOME_EXACT 的键
	exactKeys := map[string]bool{}
	for _, line := range strings.Split(exact, "\n") {
		line = strings.TrimSpace(line)
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		if k != "" && !strings.ContainsAny(k, " '\"[{") {
			exactKeys[k] = true
		}
	}
	// 抽 YSM_BIOME_MATCH 的关键词
	var pats []string
	for _, line := range strings.Split(patSection, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "['") {
			continue
		}
		end := strings.Index(line[2:], "'")
		if end > 0 {
			pats = append(pats, line[2:2+end])
		}
	}
	if len(exactKeys) < 10 || len(pats) < 20 {
		t.Fatalf("色板解析失败：exact=%d patterns=%d", len(exactKeys), len(pats))
	}

	var missing []string
	for key := range yyBiomeTable {
		if exactKeys[key] {
			continue
		}
		hit := false
		for _, p := range pats {
			if strings.Contains(key, p) {
				hit = true
				break
			}
		}
		if !hit {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("前端色板缺这些群系（会画成灰色）：%v", missing)
	}
}

// jsSection 取 JS 里 start 到 end 之间的文本（end 取首次出现）。
func jsSection(js, start, end string) string {
	i := strings.Index(js, start)
	if i < 0 {
		return ""
	}
	rest := js[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return rest
	}
	return rest[:j]
}

// 结构清单里的 salt/spacing 必须与查询接口返回一致（防止两份参数漂移）。
func TestStructureParamsEchoed(t *testing.T) {
	mux := yySeedTestMux()
	for _, spec := range yySeedStructureSpecs() {
		if spec.Kind != yyStructKindSpread {
			continue
		}
		code, body := yySeedTestGet(t, mux, "/api/seed/structure?seed=7&type="+spec.ID+"&from_x=0&from_z=0&to_x=64&to_z=64&lod=2")
		if code != http.StatusOK || body["ok"] != true {
			t.Fatalf("%s 查询失败：%d %v", spec.ID, code, body)
		}
		if int(body["spacing"].(float64)) != spec.Spacing {
			t.Fatalf("%s spacing 不一致：接口 %v / 注册表 %d", spec.ID, body["spacing"], spec.Spacing)
		}
		if int(body["separation"].(float64)) != spec.Separation {
			t.Fatalf("%s separation 不一致：接口 %v / 注册表 %d", spec.ID, body["separation"], spec.Separation)
		}
		if int64(body["salt"].(float64)) != spec.Salt {
			t.Fatalf("%s salt 不一致：接口 %v / 注册表 %d", spec.ID, body["salt"], spec.Salt)
		}
		if body["precision"] != "candidate" {
			t.Fatalf("%s 网格结构精度应为 candidate，实际 %v", spec.ID, body["precision"])
		}
	}
}
