package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// yyGateTestHandler 造一棵最小的 mux：一个 API、一个 HTML 页面、一个静态资源。
func yyGateTestHandler() http.Handler {
	inner := http.NewServeMux()
	inner.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	inner.HandleFunc("/images/logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNG"))
	})
	inner.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>正常面板</html>"))
	})
	return yyIntegrityGate(inner)
}

func yyGateDo(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

// 未篡改 → 一切放行。
func TestIntegrityGatePassesWhenClean(t *testing.T) {
	yyIntegrityReset()
	defer yyIntegrityReset()
	yyIntegritySet(yyIntegrityClean, "")

	h := yyGateTestHandler()
	for _, p := range []string{"/api/ping", "/", "/images/logo.png"} {
		if rec := yyGateDo(t, h, p); rec.Code != http.StatusOK {
			t.Fatalf("%s 应放行，实际 %d: %s", p, rec.Code, rec.Body.String())
		}
	}
}

// 篡改 + /api/* → 403 且 body 里有 tampered。
func TestIntegrityGateBlocksAPIWhenTampered(t *testing.T) {
	yyIntegrityReset()
	defer yyIntegrityReset()
	yyIntegritySet(yyIntegrityTampered, "APK 签名证书不匹配")

	h := yyGateTestHandler()
	rec := yyGateDo(t, h, "/api/ping")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("API 应被拦，实际 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tampered") {
		t.Fatalf("body 缺少 tampered: %s", rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body 不是 JSON: %s", rec.Body.String())
	}
	if body["ok"] != false || body["code"] != "tampered" {
		t.Fatalf("字段不符: %v", body)
	}
	if !strings.Contains(body["error"].(string), "完整性校验未通过") {
		t.Fatalf("错误文案不符: %v", body["error"])
	}
}

// 篡改 + MCP/health 也按 JSON 拦（远端中继按 JSON 解析）。
func TestIntegrityGateBlocksMCPAsJSON(t *testing.T) {
	yyIntegrityReset()
	defer yyIntegrityReset()
	yyIntegritySet(yyIntegrityTampered, "x")
	h := yyGateTestHandler()
	for _, p := range []string{"/mcp", "/mcp/", "/health"} {
		rec := yyGateDo(t, h, p)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "tampered") {
			t.Fatalf("%s 应 JSON 拦截，实际 %d: %s", p, rec.Code, rec.Body.String())
		}
	}
}

// 篡改 + HTML 页面 → 中文拦截页。
func TestIntegrityGateBlocksHTMLWhenTampered(t *testing.T) {
	yyIntegrityReset()
	defer yyIntegrityReset()
	yyIntegritySet(yyIntegrityTampered, "APK 签名证书不匹配")

	rec := yyGateDo(t, yyGateTestHandler(), "/")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("HTML 应被拦，实际 %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("拦截页 Content-Type 不对: %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{"客户端完整性校验未通过", "tampered", "官方渠道"} {
		if !strings.Contains(body, want) {
			t.Fatalf("拦截页缺少 %q", want)
		}
	}
}

// 静态资源同样不放行（logo/字体不开口子）。
func TestIntegrityGateBlocksStaticAssets(t *testing.T) {
	yyIntegrityReset()
	defer yyIntegrityReset()
	yyIntegritySet(yyIntegrityTampered, "x")

	rec := yyGateDo(t, yyGateTestHandler(), "/images/logo.png")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("静态资源应一并拦截，实际 %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "PNG") {
		t.Fatal("静态资源内容泄漏")
	}
}

// 校验状态未知（平台不支持 / 没拿到签名信息）→ 放行，不误杀。
func TestIntegrityGatePassesWhenUnknown(t *testing.T) {
	yyIntegrityReset()
	defer yyIntegrityReset()
	yyIntegritySet(yyIntegrityUnknown, "平台 windows 不具备自校验条件")

	h := yyGateTestHandler()
	for _, p := range []string{"/api/ping", "/", "/images/logo.png"} {
		if rec := yyGateDo(t, h, p); rec.Code != http.StatusOK {
			t.Fatalf("%s 未知状态应放行，实际 %d", p, rec.Code)
		}
	}
}

// 还没校验过（例如极端启动顺序）→ 放行，不能因为「还没算完」就把人锁在门外。
func TestIntegrityGatePassesWhenUnchecked(t *testing.T) {
	yyIntegrityReset()
	defer yyIntegrityReset()

	if rec := yyGateDo(t, yyGateTestHandler(), "/"); rec.Code != http.StatusOK {
		t.Fatalf("未校验状态应放行，实际 %d", rec.Code)
	}
}

// YY_INTEGRITY_ENFORCE=0 → 即便判定篡改也放行（开发调试开关）。
func TestIntegrityGateEnvDisablesEnforce(t *testing.T) {
	yyIntegrityReset()
	defer yyIntegrityReset()
	yyIntegritySet(yyIntegrityTampered, "x")

	t.Setenv("YY_INTEGRITY_ENFORCE", "0")
	if yyIntegrityEnforceOn() {
		t.Fatal("YY_INTEGRITY_ENFORCE=0 应关闭强制")
	}
	if rec := yyGateDo(t, yyGateTestHandler(), "/api/ping"); rec.Code != http.StatusOK {
		t.Fatalf("关闭强制后应放行，实际 %d", rec.Code)
	}
}

// 默认开启，且只有显式的 0/false/off/no 才关闭。
func TestIntegrityEnforceDefaultsOn(t *testing.T) {
	for _, v := range []string{"", "1", "true", "yes", "on", "随便什么"} {
		t.Setenv("YY_INTEGRITY_ENFORCE", v)
		if !yyIntegrityEnforceOn() {
			t.Fatalf("YY_INTEGRITY_ENFORCE=%q 不应关闭强制", v)
		}
	}
}

// 状态快照必须给出四个字段，且绝不泄露 hash 值本身。
func TestIntegrityReportHidesHashValue(t *testing.T) {
	yyIntegrityReset()
	defer yyIntegrityReset()
	yyIntegritySet(yyIntegrityClean, "")

	oldHash := frontendHash
	defer func() { frontendHash = oldHash }()
	frontendHash = "sha256:" + strings.Repeat("ab", 32)

	rep := yyIntegrityReport()
	for _, k := range []string{"checked", "ok", "reason", "frontend_hash_present"} {
		if _, ok := rep[k]; !ok {
			t.Fatalf("状态缺少字段 %q: %v", k, rep)
		}
	}
	if rep["frontend_hash_present"] != true {
		t.Fatalf("frontend_hash_present 应为 true: %v", rep)
	}
	raw, _ := json.Marshal(rep)
	if strings.Contains(string(raw), "abababab") {
		t.Fatalf("状态接口泄露了 hash 值: %s", raw)
	}
}

// 构建时只注入了 buildStamp、忘了注入 frontendHash → 必须判失败，
// 绝不能当成「开发构建」静默通过（这是本轮要堵的口子）。
func TestRunIntegrityChecksFailsWhenStampWithoutFrontendHash(t *testing.T) {
	oldHash, oldStamp := frontendHash, buildStamp
	defer func() { frontendHash, buildStamp = oldHash, oldStamp }()

	frontendHash = ""
	buildStamp = "20260101-000000-abcdef"
	res := RunIntegrityChecks()
	r, ok := res["frontend"]
	if !ok {
		t.Fatal("有 buildStamp 却没有 frontendHash 时必须产出失败结果")
	}
	if r.Passed {
		t.Fatalf("漏注 frontendHash 竟判通过: %+v", r)
	}
	if !strings.Contains(r.Message, "frontendHash") {
		t.Fatalf("失败原因应指明 frontendHash: %q", r.Message)
	}
}

// 开发构建（两者都空）→ 不产出检查项，不影响本地 go build / go test。
func TestRunIntegrityChecksSkipsDevBuild(t *testing.T) {
	oldHash, oldStamp := frontendHash, buildStamp
	defer func() { frontendHash, buildStamp = oldHash, oldStamp }()

	frontendHash, buildStamp = "", ""
	if res := RunIntegrityChecks(); len(res) != 0 {
		t.Fatalf("开发构建不应产出检查项: %v", res)
	}
}

// 摘要一致 → 通过；摘要不符 → 失败且提示篡改。
func TestRunIntegrityChecksDetectsFrontendChange(t *testing.T) {
	oldHash, oldStamp := frontendHash, buildStamp
	defer func() { frontendHash, buildStamp = oldHash, oldStamp }()

	digest, err := frontendResourceDigest(frontendFS)
	if err != nil {
		t.Fatalf("计算内嵌前端摘要失败: %v", err)
	}
	buildStamp = ""

	frontendHash = "sha256:" + digest
	if r, ok := RunIntegrityChecks()["frontend"]; !ok || !r.Passed {
		t.Fatalf("摘要一致应通过: %+v", r)
	}

	frontendHash = "sha256:" + strings.Repeat("0", 64)
	r, ok := RunIntegrityChecks()["frontend"]
	if !ok || r.Passed {
		t.Fatalf("摘要不符应失败: %+v", r)
	}
	if !strings.Contains(r.Message, "篡改") {
		t.Fatalf("失败原因应提示篡改: %q", r.Message)
	}
}

// 构建脚本算摘要的输入（磁盘上的 frontend/）必须与 go:embed 编进去的完全一致，
// 否则线上会出现「构建成功但一启动就判篡改」的假拦截。
func TestFrontendDigestDiskMatchesEmbedded(t *testing.T) {
	embedded, err := frontendResourceDigest(frontendFS)
	if err != nil {
		t.Fatalf("内嵌资源摘要失败: %v", err)
	}
	if _, statErr := os.Stat("frontend"); statErr != nil {
		t.Skip("当前目录没有 frontend/，跳过磁盘比对")
	}
	onDisk, err := frontendResourceDigest(os.DirFS("."))
	if err != nil {
		t.Fatalf("磁盘资源摘要失败: %v", err)
	}
	if embedded != onDisk {
		t.Fatalf("磁盘与内嵌前端摘要不一致（build.ps1 的枚举规则需要跟 frontendResourceDigest 对齐）: disk=%s embed=%s", shortHash(onDisk), shortHash(embedded))
	}
}

// 本机是开发/测试二进制（没跑封签脚本）时，自校验必须降级为「未知」而不是篡改。
func TestVerifySelfIntegrityDegradesInsteadOfBlocking(t *testing.T) {
	oldHash, oldStamp := frontendHash, buildStamp
	defer func() { frontendHash, buildStamp = oldHash, oldStamp }()
	frontendHash, buildStamp = "", ""

	verdict, reason := yyVerifySelfIntegrity()
	if verdict == yyIntegrityTampered {
		t.Fatalf("开发/测试二进制不应被判篡改: %s", reason)
	}
	t.Logf("verdict=%s reason=%s", verdict, reason)
}
