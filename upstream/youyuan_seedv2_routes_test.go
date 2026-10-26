package main

// 种子查询 v2 路由的黄金对拍（2026-10-26 第 28 条）
//
// 真值来源：参考实现 D:/种子查询器自研/src/world.mjs，用 Node 直接跑出来：
//
//   node --input-type=module -e "
//     import {structureCandidates, STRUCTURE_CONFIG, strongholdRingPositions, isSlimeChunkJava} from './src/world.mjs';
//     ...
//   "
//
// 这里把数字写死，是为了让「哪天有人改了公式」在 go test 里当场炸掉，
// 而不是等玩家发现传送到了海里。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func yySeedV2TestMux() *http.ServeMux {
	mux := http.NewServeMux()
	yySeedV2Routes(mux)
	return mux
}

func yySeedV2TestGet(t *testing.T, mux *http.ServeMux, url string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		body = map[string]any{"_raw": rec.Body.String()}
	}
	return rec.Code, body
}

func TestSeedV2ManifestRoutes(t *testing.T) {
	mux := yySeedV2TestMux()
	code, body := yySeedV2TestGet(t, mux, "/api/seed/v2/manifest")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("manifest 异常：%d %v", code, body)
	}
	tables, _ := body["btree_tables"].([]any)
	if len(tables) != 5 {
		t.Fatalf("生物群系参数表应有 5 张，实际 %d 张：%v", len(tables), tables)
	}
	if body["biome_engine"] != true {
		t.Fatal("biome_engine 应为 true（表已装载）")
	}
	eds, _ := body["editions"].([]any)
	if len(eds) != 2 {
		t.Fatalf("版本类型应有 java + bedrock 两套，实际 %v", eds)
	}
	structs, _ := body["structures"].([]any)
	if len(structs) < 20 {
		t.Fatalf("结构清单太短：%d", len(structs))
	}
}

// TestSeedV2VillageAnchor village 是最常用的结构，公式一旦漂了整张图就废。
// 真值：seed=0，区块 [-2000,2000]² 内 13861 个村庄，前 5 个坐标见下。
func TestSeedV2VillageAnchor(t *testing.T) {
	mux := yySeedV2TestMux()
	code, body := yySeedV2TestGet(t, mux,
		"/api/seed/v2/structures?seed=0&version=1.21.4&edition=java&cx=0&cz=0&r=2000&types=village")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("structures 异常：%d %v", code, body)
	}
	counts, _ := body["counts"].(map[string]any)
	got, _ := counts["village"].(float64)
	if int(got) != 13861 {
		t.Fatalf("seed=0 村庄数量应为 13861，实际 %v", got)
	}
	list, _ := body["structures"].(map[string]any)
	pts, _ := list["village"].([]any)
	if len(pts) != 13861 {
		t.Fatalf("村庄点数应为 13861，实际 %d", len(pts))
	}
	// 只比「集合」，不比顺序：region 的遍历顺序是实现细节（先 rx 还是先 rz），
	// 但**坐标本身必须一个不差**。
	have := make(map[[2]int]bool, len(pts))
	for _, raw := range pts {
		item, _ := raw.(map[string]any)
		x, _ := item["x"].(float64)
		z, _ := item["z"].(float64)
		have[[2]int{int(x), int(z)}] = true
	}
	for _, w := range [][2]int{
		{-2000, -1999}, {-1998, -1962}, {-1991, -1923}, {-1999, -1847}, {-1989, -1822},
		{0, 0}, {2000, 2000},
	} {
		if w == [2]int{0, 0} || w == [2]int{2000, 2000} {
			continue // 这两个只是边界示意，不一定真的生成村庄
		}
		if !have[w] {
			t.Fatalf("缺少参考实现里存在的村庄 (%d,%d)", w[0], w[1])
		}
	}
	// 同一请求两次结果必须完全一致（有 map 遍历没排序的话这里会抖）
	_, body2 := yySeedV2TestGet(t, mux,
		"/api/seed/v2/structures?seed=0&version=1.21.4&edition=java&cx=0&cz=0&r=2000&types=village")
	list2, _ := body2["structures"].(map[string]any)
	pts2, _ := list2["village"].([]any)
	a, _ := json.Marshal(pts)
	b, _ := json.Marshal(pts2)
	if string(a) != string(b) {
		t.Fatal("同一请求两次结果不一致，说明顺序不稳定")
	}
}

// TestSeedV2StrongholdAnchor 要塞是 exact 级，必须和 cubiomes 逐位一致。
func TestSeedV2StrongholdAnchor(t *testing.T) {
	mux := yySeedV2TestMux()
	code, body := yySeedV2TestGet(t, mux, "/api/seed/v2/stronghold?seed=0&version=1.21.4")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("stronghold 异常：%d %v", code, body)
	}
	if body["count"].(float64) != 128 {
		t.Fatalf("要塞应为 128 个，实际 %v", body["count"])
	}
	list, _ := body["strongholds"].([]any)
	want := [][2]int{{-13, -106}, {128, 55}, {-106, 79}}
	for i, w := range want {
		item, _ := list[i].(map[string]any)
		if int(item["x"].(float64)) != w[0] || int(item["z"].(float64)) != w[1] {
			t.Fatalf("第 %d 个要塞应为 (%d,%d)，实际 %v", i, w[0], w[1], item)
		}
	}
	// 2026-10-26 挑刺修正：只算了环位，没做原版「在允许群系里就近回拉」，
	// 所以不能标 exact（会害玩家按坐标传送到要塞外）。
	if body["precision"] != "ring_approx" {
		t.Fatalf("要塞精度应标 ring_approx，实际 %v", body["precision"])
	}
	// 基岩版要塞没实现，必须明确 501，不许拿 Java 环位冒充
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/seed/v2/stronghold?seed=0&edition=bedrock", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("基岩版要塞应 501，实际 %d", rec.Code)
	}
}

// TestSeedV2SlimeAnchor 史莱姆区块公式里 `^` 优先级低于 `+`，抄错就差一倍。
func TestSeedV2SlimeAnchor(t *testing.T) {
	mux := yySeedV2TestMux()
	code, body := yySeedV2TestGet(t, mux, "/api/seed/v2/slime?seed=0&cx=0&cz=0&r=16")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("slime 异常：%d %v", code, body)
	}
	if body["count"].(float64) != 121 {
		t.Fatalf("seed=0 ±16 区块内史莱姆区块应为 121 个，实际 %v", body["count"])
	}
	list, _ := body["chunks"].([]any)
	want := [][2]int{{-12, -16}, {-8, -16}, {-4, -16}, {12, -16}, {15, -16}}
	for i, w := range want {
		item, _ := list[i].(map[string]any)
		if int(item["x"].(float64)) != w[0] || int(item["z"].(float64)) != w[1] {
			t.Fatalf("第 %d 个史莱姆区块应为 (%d,%d)，实际 %v", i, w[0], w[1], item)
		}
	}
}

// TestSeedV2ParamErrors 参数错的必须 400，不许 500、不许 panic。
func TestSeedV2ParamErrors(t *testing.T) {
	mux := yySeedV2TestMux()
	for _, url := range []string{
		"/api/seed/v2/structures",
		"/api/seed/v2/slime",
		"/api/seed/v2/stronghold",
		"/api/seed/v2/summary",
	} {
		code, body := yySeedV2TestGet(t, mux, url)
		if code != http.StatusBadRequest {
			t.Fatalf("%s 缺 seed 应 400，实际 %d %v", url, code, body)
		}
		if body["ok"] != false {
			t.Fatalf("%s 错误返回应带 ok:false，实际 %v", url, body)
		}
	}
	// seed 非整数
	code, _ := yySeedV2TestGet(t, mux, "/api/seed/v2/structures?seed=abc")
	if code != http.StatusBadRequest {
		t.Fatalf("seed=abc 应 400，实际 %d", code)
	}
	// layer 非法
	code, _ = yySeedV2TestGet(t, mux, "/api/seed/v2/tile.png?seed=0&layer=nope")
	if code != http.StatusBadRequest {
		t.Fatalf("layer=nope 应 400，实际 %d", code)
	}
	// 超大半径必须被夹住而不是报错
	code, body := yySeedV2TestGet(t, mux, "/api/seed/v2/structures?seed=0&r=999999&types=village&limit=100")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("超大半径应夹住并正常返回，实际 %d %v", code, body)
	}
	if body["truncated"] != true {
		t.Fatal("limit=100 且半径被夹到 2048，必然截断，应标 truncated=true")
	}
}

// TestSeedV2TileOrNotReady 渲染引擎可能还没就绪：
// 就绪时必须回一张能解码的 PNG，没就绪时必须 503，绝不许 500。
func TestSeedV2TileOrNotReady(t *testing.T) {
	mux := yySeedV2TestMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/seed/v2/tile.png?seed=12345&cx=0&cz=0&w=64&h=64&bpp=8", nil))
	switch rec.Code {
	case http.StatusOK:
		if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("成功时 Content-Type 应为 image/png，实际 %s", ct)
		}
		if rec.Body.Len() < 100 {
			t.Fatalf("PNG 太小，可疑：%d 字节", rec.Body.Len())
		}
		if sig := rec.Body.Bytes(); len(sig) < 8 || sig[0] != 0x89 || sig[1] != 'P' {
			t.Fatal("返回的不是 PNG（魔数不对）")
		}
	case http.StatusServiceUnavailable:
		// 渲染引擎还没就绪，允许
	default:
		t.Fatalf("tile.png 只允许 200 或 503，实际 %d", rec.Code)
	}
}

// TestSeedV2PluginSDK 插件接口与 HTTP 必须同一份实现。
func TestSeedV2PluginSDK(t *testing.T) {
	out, err := yySeedV2Call("seed.stronghold", []any{map[string]any{
		"seed": float64(0), "version": "1.21.4", "count": float64(3),
	}})
	if err != nil {
		t.Fatalf("seed.stronghold 失败：%v", err)
	}
	m, _ := out.(map[string]any)
	list, _ := m["strongholds"].([]map[string]int)
	if len(list) != 3 {
		t.Fatalf("插件接口应返回 3 个要塞，实际 %d", len(list))
	}
	if list[0]["x"] != -13 || list[0]["z"] != -106 {
		t.Fatalf("插件接口与 HTTP 结果不一致：%v", list[0])
	}

	// 未注册的方法要报错，不能静默成功
	if _, err := yySeedV2Call("seed.nope", nil); err == nil {
		t.Fatal("未知 seed 方法应报错")
	}
	// 缺 seed 要报错
	if _, err := yySeedV2Call("seed.slime", []any{map[string]any{}}); err == nil {
		t.Fatal("缺 seed 应报错")
	}
	// 基岩版史莱姆要明确拒绝，不许给错结果
	if _, err := yySeedV2Call("seed.slime", []any{map[string]any{
		"seed": float64(0), "edition": "bedrock",
	}}); err == nil {
		t.Fatal("基岩版史莱姆应明确拒绝")
	}
}

// TestSeedV2MCPCatalog MCP 工具是从 yyAPIRoutes 生成的，路径必须真实存在。
func TestSeedV2MCPCatalog(t *testing.T) {
	want := map[string]bool{
		"api_seed_v2_manifest": false, "api_seed_v2_meta": false, "api_seed_v2_tile": false,
		"api_seed_v2_structures": false, "api_seed_v2_biome": false,
		"api_seed_v2_slime": false, "api_seed_v2_stronghold": false, "api_seed_v2_summary": false,
	}
	paths := map[string]bool{}
	for _, r := range yyAPIRoutes {
		if _, ok := want[r.Name]; ok {
			want[r.Name] = true
		}
		paths[r.Path] = true
	}
	for name, ok := range want {
		if !ok {
			t.Fatalf("API 目录缺少 %s，MCP 就看不到这个工具", name)
		}
	}
	for _, p := range []string{
		"/api/seed/v2/manifest", "/api/seed/v2/meta", "/api/seed/v2/tile.png",
		"/api/seed/v2/structures", "/api/seed/v2/biome",
		"/api/seed/v2/slime", "/api/seed/v2/stronghold", "/api/seed/v2/summary",
	} {
		if !paths[p] {
			t.Fatalf("目录里没有路径 %s", p)
		}
		if !yyMCPReadPaths[p] {
			t.Fatalf("%s 不在 MCP 只读白名单里，只读连接会调不到", p)
		}
	}
	// 插件 SDK 六个方法都要注册进 yyExtraMethods，否则插件调不到
	for _, m := range []string{"seed.manifest", "seed.structures", "seed.biome", "seed.slime", "seed.stronghold", "seed.summary"} {
		if _, ok := yyExtraMethods[m]; !ok {
			t.Fatalf("插件接口 %s 没注册进 yyExtraMethods", m)
		}
	}
}

// TestSeedV2StructureCoverage 各结构的点数必须和参考实现一致。
// 真值来自 src/world.mjs 的 structureCandidates(0n, cfg, -1000,-1000,1000,1000)。
// 只挑「配置唯一、不含版本分支」的结构，避免把版本差异误判成 bug。
func TestSeedV2StructureCoverage(t *testing.T) {
	cases := []struct {
		typ  string
		dim  string
		want int
	}{
		{"woodlandMansion", "overworld", 618}, // 三角分布，最容易抄错的一个
		{"oceanMonument", "overworld", 3870},  // 三角分布
		{"endCity", "end", 10009},             // 三角分布
		{"ancientCity", "overworld", 6964},
		{"desertTemple", "overworld", 3891},
		{"witchHut", "overworld", 3897},
		{"shipwreck", "overworld", 6977},
		{"trailRuin", "overworld", 3452},
		{"igloo", "overworld", 3890},
		{"jungleTemple", "overworld", 3882},
		{"pillagerOutpost", "overworld", 3898},
		{"oceanRuin", "overworld", 10023},
		{"ruinedPortalOW", "overworld", 2503},
	}
	mux := yySeedV2TestMux()
	for _, c := range cases {
		url := "/api/seed/v2/structures?seed=0&version=1.21.4&edition=java&dim=" + c.dim +
			"&cx=0&cz=0&r=1000&types=" + c.typ
		code, body := yySeedV2TestGet(t, mux, url)
		if code != http.StatusOK || body["ok"] != true {
			t.Fatalf("%s 请求失败：%d %v", c.typ, code, body)
		}
		counts, _ := body["counts"].(map[string]any)
		got := 0
		if v, ok := counts[c.typ].(float64); ok {
			got = int(v)
		}
		if got != c.want {
			t.Errorf("%s 在 ±1000 区块内应有 %d 个，实际 %d", c.typ, c.want, got)
		}
	}
}

// TestSeedV2VersionBranches 版本分支是第二个最容易漂的地方（第一个是三角分布）。
// 真值来自 src/world.mjs 的 configForVersion + structureCandidates。
func TestSeedV2VersionBranches(t *testing.T) {
	cases := []struct {
		typ, version, edition string
		want                  int
	}{
		// 村庄：1.18 前后 spacing 34→32；基岩版 1.18 前是 27/10
		{"village", "1.16", "java", 3892},
		{"village", "1.18", "java", 3456},
		{"village", "1.16", "bedrock", 5494},
		// 前哨：基岩版固定 80/24，跟 Java 的 32/8 差一个数量级
		{"pillagerOutpost", "1.21.4", "java", 3898},
		{"pillagerOutpost", "1.21", "bedrock", 623},
		// 沉船：1.16 前 16/8
		{"shipwreck", "1.15", "java", 15625},
	}
	mux := yySeedV2TestMux()
	for _, c := range cases {
		url := "/api/seed/v2/structures?seed=0&edition=" + c.edition + "&version=" + c.version +
			"&cx=0&cz=0&r=1000&types=" + c.typ
		code, body := yySeedV2TestGet(t, mux, url)
		if code != http.StatusOK || body["ok"] != true {
			t.Fatalf("%s %s %s 请求失败：%d %v", c.typ, c.version, c.edition, code, body)
		}
		counts, _ := body["counts"].(map[string]any)
		got := 0
		if v, ok := counts[c.typ].(float64); ok {
			got = int(v)
		}
		if got != c.want {
			t.Errorf("%s %s/%s 应有 %d 个，实际 %d", c.typ, c.version, c.edition, c.want, got)
		}
	}
	// 海洋废墟 1.15 应该判定为「该版本没有这个结构」，而不是给个错误坐标
	_, body := yySeedV2TestGet(t, mux,
		"/api/seed/v2/structures?seed=0&version=1.15&cx=0&cz=0&r=1000&types=oceanRuin")
	if _, has := body["counts"].(map[string]any)["oceanRuin"]; has {
		t.Fatalf("1.15 不应有海底废墟，却返回了结果：%v", body["counts"])
	}
}

// TestSeedV2IconSVG 图标出口：28 种结构都要有图，前端 DOM 层直接用这个。
func TestSeedV2IconSVG(t *testing.T) {
	mux := yySeedV2TestMux()
	list := yyV2StructureList(yyV2ParseVersion("1.21.4", yyV2EditionJava), yyV2EditionJava)
	if len(list) < 20 {
		t.Fatalf("结构清单太短：%d", len(list))
	}
	for _, info := range list {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
			"/api/seed/v2/icon.svg?id="+info.ID+"&size=20", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s 图标应 200，实际 %d", info.ID, rec.Code)
		}
		body := rec.Body.String()
		if !strings.HasPrefix(body, "<svg") || !strings.HasSuffix(body, "</svg>") {
			t.Fatalf("%s 图标不是完整 SVG：%.80s", info.ID, body)
		}
		if len(body) < 120 {
			t.Fatalf("%s 图标太短，可能是空图：%d 字节", info.ID, len(body))
		}
	}
	// 未知 id 不该 500，要么给兜底图要么 400
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/seed/v2/icon.svg?id=not_a_structure", nil))
	if rec.Code == http.StatusInternalServerError {
		t.Fatal("未知结构不该 500")
	}
	// 缺 id 必须 400
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/seed/v2/icon.svg", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("缺 id 应 400，实际 %d", rec.Code)
	}
}
