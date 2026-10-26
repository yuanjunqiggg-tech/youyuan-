package main

// 种子查询器接口测试。只测纯本地算法与 HTTP 层，不依赖网络/存档。
// 运行：go test -run 'Seed' .

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func yySeedTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	yySeedRoutes(mux)
	return mux
}

func yySeedTestGet(t *testing.T, mux *http.ServeMux, url string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s 返回不是 JSON：%v，原始内容 %q", url, err, rec.Body.String())
	}
	return rec.Code, body
}

// 校验 java.util.Random 的 LCG 与 Java 实际输出一致（new Random(0).nextInt() == -1155484576）。
func TestSeedJavaRandomVector(t *testing.T) {
	r := yySeedNewJavaRandom(0)
	if got := int64(r.next(32)); got != -1155484576 {
		t.Fatalf("java.util.Random(0).nextInt() = %d，期望 -1155484576", got)
	}
}

func TestSeedSlimeDeterministic(t *testing.T) {
	mux := yySeedTestMux()
	url := "/api/seed/slime?seed=0&x=0&z=0&radius=4"

	code1, body1 := yySeedTestGet(t, mux, url)
	code2, body2 := yySeedTestGet(t, mux, url)
	if code1 != http.StatusOK || code2 != http.StatusOK {
		t.Fatalf("状态码异常：%d / %d", code1, code2)
	}
	if body1["ok"] != true {
		t.Fatalf("ok 不为 true：%v", body1)
	}
	chunks1, _ := json.Marshal(body1["chunks"])
	chunks2, _ := json.Marshal(body2["chunks"])
	if string(chunks1) != string(chunks2) {
		t.Fatalf("同一输入两次结果不一致：%s vs %s", chunks1, chunks2)
	}
	count := int(body1["count"].(float64))
	list := body1["chunks"].([]any)
	blocks := body1["blocks"].([]any)
	if count != len(list) || count != len(blocks) {
		t.Fatalf("count=%d 与 chunks=%d / blocks=%d 长度不一致", count, len(list), len(blocks))
	}
	// 区块坐标 *16 必须等于方块坐标
	for i, raw := range list {
		c := raw.([]any)
		b := blocks[i].([]any)
		if int(c[0].(float64))*16 != int(b[0].(float64)) || int(c[1].(float64))*16 != int(b[1].(float64)) {
			t.Fatalf("第 %d 个区块坐标 %v 与方块坐标 %v 不匹配", i, c, b)
		}
	}
	if int(body1["radius"].(float64)) != 4 {
		t.Fatalf("radius 回显异常：%v", body1["radius"])
	}
}

func TestSeedSlimeRadiusClamp(t *testing.T) {
	mux := yySeedTestMux()
	code, body := yySeedTestGet(t, mux, "/api/seed/slime?seed=0&x=0&z=0&radius=500")
	if code != http.StatusOK {
		t.Fatalf("状态码 %d", code)
	}
	if int(body["radius"].(float64)) != yySeedMaxSlimeRadius {
		t.Fatalf("radius 未夹到 %d：%v", yySeedMaxSlimeRadius, body["radius"])
	}
	count := int(body["count"].(float64))
	if count != len(body["chunks"].([]any)) {
		t.Fatalf("count=%d 与 chunks 长度不一致", count)
	}
	maxCells := (2*yySeedMaxSlimeRadius + 1) * (2*yySeedMaxSlimeRadius + 1)
	if count > maxCells {
		t.Fatalf("count=%d 超过半径 64 的格子总数 %d", count, maxCells)
	}
}

func TestSeedStrongholdRings(t *testing.T) {
	mux := yySeedTestMux()
	code, body := yySeedTestGet(t, mux, "/api/seed/stronghold?seed=0&count=3")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("状态码 %d，body %v", code, body)
	}
	points := body["points"].([]any)
	if len(points) != 3 {
		t.Fatalf("期望 3 个要塞点，实际 %d", len(points))
	}
	wantRing := []int{1, 2, 3}
	wantRadius := []float64{1408, 4352, 7424}
	for i, raw := range points {
		p := raw.(map[string]any)
		if int(p["ring"].(float64)) != wantRing[i] {
			t.Fatalf("第 %d 个点 ring=%v，期望 %d", i, p["ring"], wantRing[i])
		}
		x := p["x"].(float64)
		z := p["z"].(float64)
		dist := math.Hypot(x, z)
		// 环半径来自原版三环（方块），允许取整误差
		if math.Abs(dist-wantRadius[i]) > 2 {
			t.Fatalf("第 %d 个点距原点 %.1f 方块，期望约 %.0f", i, dist, wantRadius[i])
		}
		if dist <= 1000 {
			t.Fatalf("第 %d 个点距离过近（%.1f），不像要塞环位置", i, dist)
		}
	}
	if body["precision"] != "approximate" {
		t.Fatalf("要塞精度必须诚实标注为 approximate，实际 %v", body["precision"])
	}
}

func TestSeedStructureTruncated(t *testing.T) {
	mux := yySeedTestMux()
	code, body := yySeedTestGet(t, mux, "/api/seed/structure?seed=0&type=village&from_x=0&from_z=0&to_x=4000&to_z=4000")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("状态码 %d，body %v", code, body)
	}
	if body["truncated"] != true {
		t.Fatalf("超范围查询应标记 truncated=true：%v", body["truncated"])
	}
	count := int(body["count"].(float64))
	if count > yySeedMaxResults {
		t.Fatalf("count=%d 超过上限 %d", count, yySeedMaxResults)
	}
	if count != len(body["chunks"].([]any)) {
		t.Fatalf("count 与 chunks 长度不一致")
	}
	if body["precision"] != "candidate" {
		t.Fatalf("结构精度必须标注 candidate，实际 %v", body["precision"])
	}

	// 范围超限直接拒绝
	code, body = yySeedTestGet(t, mux, "/api/seed/structure?seed=0&type=village&from_x=0&to_x=9000&from_z=0&to_z=0")
	if code == http.StatusInternalServerError || body["ok"] != false {
		t.Fatalf("范围超限应返回 ok:false 且非 500，实际 %d %v", code, body)
	}
}

func TestSeedQuerySummary(t *testing.T) {
	mux := yySeedTestMux()
	code, body := yySeedTestGet(t, mux, "/api/seed/query?seed=12345&x=0&z=0&radius=8")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("状态码 %d，body %v", code, body)
	}
	summary, _ := body["summary"].(string)
	if strings.TrimSpace(summary) == "" {
		t.Fatal("summary 不能为空")
	}
	if !strings.Contains(summary, "种子 12345") || !strings.Contains(summary, "要塞") {
		t.Fatalf("summary 内容不完整：%q", summary)
	}
	if _, ok := body["strongholds"].([]any); !ok {
		t.Fatalf("缺少 strongholds：%v", body["strongholds"])
	}
	if _, ok := body["slime"].(map[string]any); !ok {
		t.Fatalf("缺少 slime：%v", body["slime"])
	}
}

func TestSeedCoordinatesRoundTrip(t *testing.T) {
	mux := yySeedTestMux()
	code, body := yySeedTestGet(t, mux, "/api/seed/coordinates?seed=123456789&x=-1234&z=5678")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("编码失败：%d %v", code, body)
	}
	share, _ := body["code"].(string)
	if share == "" || !strings.HasPrefix(share, "YY1") {
		t.Fatalf("分享码格式异常：%q", share)
	}

	code, back := yySeedTestGet(t, mux, "/api/seed/coordinates?code="+share)
	if code != http.StatusOK || back["ok"] != true {
		t.Fatalf("解码失败：%d %v", code, back)
	}
	if int64(back["seed"].(float64)) != 123456789 || int(back["x"].(float64)) != -1234 || int(back["z"].(float64)) != 5678 {
		t.Fatalf("解码结果不一致：%v", back)
	}

	// 篡改最后一位必须失败
	last := share[len(share)-1]
	alt := byte('A')
	if last == 'A' {
		alt = 'B'
	}
	tampered := share[:len(share)-1] + string(alt)
	code, bad := yySeedTestGet(t, mux, "/api/seed/coordinates?code="+tampered)
	if code == http.StatusInternalServerError {
		t.Fatal("篡改分享码不应返回 500")
	}
	if bad["ok"] != false {
		t.Fatalf("篡改后的分享码竟然解码成功：%v", bad)
	}
	if msg, _ := bad["error"].(string); msg == "" {
		t.Fatal("失败时必须给出中文原因")
	}
}

func TestSeedBadParams(t *testing.T) {
	mux := yySeedTestMux()
	cases := []string{
		"/api/seed/slime?seed=abc",
		"/api/seed/slime?x=0&z=0",
		"/api/seed/slime?seed=0&x=abc",
		"/api/seed/stronghold?seed=abc",
		"/api/seed/structure?seed=0",
		"/api/seed/structure?seed=0&type=unknown",
		"/api/seed/query?seed=abc",
		"/api/seed/coordinates?code=YY1BAD",
		"/api/seed/coordinates?seed=abc",
	}
	for _, url := range cases {
		code, body := yySeedTestGet(t, mux, url)
		if code == http.StatusInternalServerError {
			t.Fatalf("%s 返回了 500", url)
		}
		if body["ok"] != false {
			t.Fatalf("%s 应返回 ok:false，实际 %v", url, body)
		}
		if msg, _ := body["error"].(string); msg == "" {
			t.Fatalf("%s 缺少中文错误原因", url)
		}
	}

	// 非 GET 必须拒绝
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/seed/manifest", nil))
	if rec.Code == http.StatusInternalServerError || rec.Code == http.StatusOK {
		t.Fatalf("POST 应被拒绝，实际状态码 %d", rec.Code)
	}
}

func TestSeedAITools(t *testing.T) {
	tools := yySeedAITools()
	if len(tools) == 0 {
		t.Fatal("yySeedAITools 不能为空")
	}
	names := map[string]bool{}
	for _, tool := range tools {
		if tool.Name == "" || tool.Description == "" || tool.Parameters == nil {
			t.Fatalf("工具定义不完整：%+v", tool)
		}
		if !strings.HasPrefix(tool.Name, "seed_") {
			t.Fatalf("工具名应统一用 seed_ 前缀：%s", tool.Name)
		}
		names[tool.Name] = true
	}
	for _, want := range []string{"seed_manifest", "seed_slime", "seed_stronghold", "seed_structure", "seed_query", "seed_coordinates"} {
		if !names[want] {
			t.Fatalf("缺少工具 %s", want)
		}
	}

	// 工具调用与 HTTP 接口结果一致（同一套核心函数）
	//
	// 2026-10-26 第 28 条：seed_slime 已经切到 v2 引擎（史莱姆公式两版一致，
	// 但 v2 的返回结构是 {x,z}，旧版是 [x,z] 之类的紧凑写法），所以这里
	// 对标的是 /api/seed/v2/slime，而不是旧的 /api/seed/slime。
	out, err := yySeedToolCall("seed_slime", map[string]any{"seed": float64(0), "x": float64(0), "z": float64(0), "radius": float64(4)})
	if err != nil || out["ok"] != true {
		t.Fatalf("工具调用失败：%v %v", out, err)
	}
	mux := yySeedTestMux()
	yySeedV2Routes(mux)
	_, httpBody := yySeedTestGet(t, mux, "/api/seed/v2/slime?seed=0&cx=0&cz=0&r=4")
	a, _ := json.Marshal(out["chunks"])
	b, _ := json.Marshal(httpBody["chunks"])
	if string(a) != string(b) {
		t.Fatalf("工具与 HTTP（v2）结果不一致：%s vs %s", a, b)
	}
	if _, err := yySeedToolCall("seed_unknown", nil); err == nil {
		t.Fatal("未知工具应返回错误")
	}
}

func TestSeedManifest(t *testing.T) {
	mux := yySeedTestMux()
	code, body := yySeedTestGet(t, mux, "/api/seed/manifest")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("manifest 异常：%d %v", code, body)
	}
	caps, ok := body["capabilities"].([]any)
	if !ok || len(caps) == 0 {
		t.Fatalf("capabilities 缺失：%v", body["capabilities"])
	}
	got := map[string]string{}
	for _, raw := range caps {
		c := raw.(map[string]any)
		got[c["id"].(string)] = c["precision"].(string)
	}
	// 精度必须诚实：史莱姆 exact，要塞 approximate，结构 candidate
	if got["slime"] != "exact" || got["structure"] != "candidate" || got["stronghold"] != "approximate" {
		t.Fatalf("精度标注不符合预期：%v", got)
	}
	limits, ok := body["limits"].(map[string]any)
	if !ok || int(limits["max_results"].(float64)) != yySeedMaxResults {
		t.Fatalf("limits 缺失或不正确：%v", body["limits"])
	}
	if int(limits["max_range_chunks"].(float64)) != yySeedMaxChunkRange {
		t.Fatalf("limits.max_range_chunks 不正确：%v", limits["max_range_chunks"])
	}
}
