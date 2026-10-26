package main

// 世界种子接口测试：只测参数解析、诚实标注与路由注册，不依赖真实连接。
// 运行：go test -run TestWorldSeed .

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func yyWorldSeedTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	yyWorldSeedRoutes(mux)
	return mux
}

func yyWorldSeedTestGet(t *testing.T, mux *http.ServeMux, url string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s 返回不是 JSON：%v，原始内容 %q", url, err, rec.Body.String())
	}
	return rec.Code, body
}

// 显式传 seed 时不依赖连接，且必须标明来源是 query。
func TestWorldSeedExplicitSeed(t *testing.T) {
	mux := yyWorldSeedTestMux()
	code, body := yyWorldSeedTestGet(t, mux, "/api/youyuan/world/seed?seed=-4172144997902289642")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("状态码 %d，body %v", code, body)
	}
	if body["seed_text"] != "-4172144997902289642" {
		t.Fatalf("seed_text 必须精确回显：%v", body["seed_text"])
	}
	if body["seed_known"] != true || body["source"] != "query" {
		t.Fatalf("显式种子的来源标注不正确：%v / %v", body["seed_known"], body["source"])
	}
	if body["edition"] != "bedrock" {
		t.Fatalf("版本必须诚实标注为 bedrock：%v", body["edition"])
	}
	if note, _ := body["note"].(string); !strings.Contains(note, "StartGame") {
		t.Fatalf("note 未说明种子来源：%q", note)
	}
}

// 未连接且不传 seed：返回 ok:false + 中文原因，且不能 500。
func TestWorldSeedNoConnection(t *testing.T) {
	mux := yyWorldSeedTestMux()
	code, body := yyWorldSeedTestGet(t, mux, "/api/youyuan/world/seed")
	if code == http.StatusInternalServerError {
		t.Fatal("未连接不应返回 500")
	}
	if body["ok"] != false {
		t.Fatalf("未连接应返回 ok:false：%v", body)
	}
	if msg, _ := body["error"].(string); msg == "" {
		t.Fatal("失败时必须给出中文原因")
	}
	if body["seed_known"] != false {
		t.Fatalf("未读到字段时 seed_known 应为 false：%v", body["seed_known"])
	}
}

// 结构标识不带 edition=java 必须拒绝，避免把 Java 结果当成基岩世界结论。
func TestWorldSeedStructureNeedsJavaOptIn(t *testing.T) {
	mux := yyWorldSeedTestMux()
	code, body := yyWorldSeedTestGet(t, mux, "/api/youyuan/world/seed?seed=123&structure=village")
	if code != http.StatusBadRequest {
		t.Fatalf("应返回 400，实际 %d", code)
	}
	if body["ok"] != false {
		t.Fatalf("应返回 ok:false：%v", body)
	}
	msg, _ := body["error"].(string)
	if !strings.Contains(msg, "基岩版") || !strings.Contains(msg, "edition=java") {
		t.Fatalf("错误提示未说明基岩版限制与显式开关：%q", msg)
	}
}

// 显式 edition=java 才做试算，且必须标注 applies_to_world=false。
func TestWorldSeedJavaPreview(t *testing.T) {
	mux := yyWorldSeedTestMux()
	code, body := yyWorldSeedTestGet(t, mux,
		"/api/youyuan/world/seed?seed=0&structure=village&edition=java&from_x=0&from_z=0&to_x=512&to_z=512&lod=2")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("状态码 %d，body %v", code, body)
	}
	preview, ok := body["java_preview"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 java_preview：%v", body)
	}
	if preview["applies_to_world"] != false {
		t.Fatalf("试算结果必须标注 applies_to_world=false：%v", preview["applies_to_world"])
	}
	if preview["precision"] != "candidate" {
		t.Fatalf("结构候选精度必须为 candidate：%v", preview["precision"])
	}
	count := int(preview["count"].(float64))
	if count != len(preview["chunks"].([]any)) {
		t.Fatal("count 与 chunks 长度不一致")
	}
	if count > yySeedMaxResults {
		t.Fatalf("count=%d 超过上限 %d", count, yySeedMaxResults)
	}

	// 未知结构标识要给出可读原因
	code, bad := yyWorldSeedTestGet(t, mux, "/api/youyuan/world/seed?seed=0&structure=castle&edition=java")
	if code != http.StatusBadRequest || bad["ok"] != false {
		t.Fatalf("未知结构应返回 400 + ok:false：%d %v", code, bad)
	}
}

// 非 GET 必须拒绝。
func TestWorldSeedRejectsNonGet(t *testing.T) {
	mux := yyWorldSeedTestMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/youyuan/world/seed", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST 应返回 405，实际 %d", rec.Code)
	}
}

// 新接口必须出现在 API 目录里（MCP tools/list 由该目录派生）。
func TestWorldSeedInCatalogAndReadable(t *testing.T) {
	found := false
	for _, route := range yyAPIRoutes {
		if route.Name == "api_youyuan_world_seed" {
			found = true
			if route.Path != "/api/youyuan/world/seed" {
				t.Fatalf("目录路径不一致：%s", route.Path)
			}
			if !yyMCPReadPaths[route.Path] {
				t.Fatalf("只读 MCP 连接白名单漏了 %s", route.Path)
			}
			if !yyMCPReadTool(route.Name) {
				t.Fatalf("yyMCPReadTool 未放行 %s", route.Name)
			}
		}
	}
	if !found {
		t.Fatal("API 目录缺少 api_youyuan_world_seed，MCP 不可见")
	}
	if yyAPIContract("/api/youyuan/world/seed") == "" {
		t.Fatal("缺少接口契约说明")
	}
}
