package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func marketTestZip(t *testing.T, id string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	manifest, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manifest.Write([]byte(`{"id":"` + id + `","name":"Market Demo","version":"1.0.0"}`)); err != nil {
		t.Fatal(err)
	}
	code, err := zw.Create("main.lua")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = code.Write([]byte("function value() return 1 end\n")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func marketRequest(method, path string, body any, actor yyPoolActor) *http.Request {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if actor.ID != "" {
		req = req.WithContext(context.WithValue(req.Context(), yyPoolActorContext{}, actor))
	}
	return req
}

func marketCall(t *testing.T, mux *http.ServeMux, req *http.Request) (int, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)
	var out map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是 JSON（%d）：%s", recorder.Code, recorder.Body.String())
	}
	return recorder.Code, out
}

func marketNewServer(t *testing.T) (*http.ServeMux, string) {
	t.Helper()
	dataDir := t.TempDir()
	mux := http.NewServeMux()
	yyMarketRoutes(mux, dataDir)
	return mux, dataDir
}

func marketCreatePlugin(t *testing.T, mux *http.ServeMux, actor yyPoolActor, name string, price int64) string {
	t.Helper()
	status, out := marketCall(t, mux, marketRequest("POST", "/api/market/plugins", map[string]any{
		"name": name, "summary": "测试简介", "description": "测试详情",
		"category": "tool", "tags": []string{"测试"}, "price_cents": price, "license": "MIT",
	}, actor))
	if status != 200 || out["ok"] != true {
		t.Fatalf("创建插件失败：%d %v", status, out)
	}
	plugin := out["plugin"].(map[string]any)
	id, _ := plugin["id"].(string)
	if id == "" {
		t.Fatal("创建插件没有返回 id")
	}
	return id
}

func marketUploadVersion(t *testing.T, mux *http.ServeMux, actor yyPoolActor, id, version string, data []byte, fileName string) (int, map[string]any) {
	t.Helper()
	return marketCall(t, mux, marketRequest("POST", "/api/market/plugins/"+id+"/versions", map[string]any{
		"version": version, "changelog": "首个版本", "file_name": fileName,
		"content_base64": base64.StdEncoding.EncodeToString(data),
		"mc_versions":    []string{"1.21"}, "loader": "Lua",
	}, actor))
}

func marketSubmit(t *testing.T, mux *http.ServeMux, actor yyPoolActor, id string) {
	t.Helper()
	status, out := marketCall(t, mux, marketRequest("POST", "/api/market/plugins/"+id+"/submit", map[string]any{}, actor))
	if status != 200 || out["status"] != "pending" {
		t.Fatalf("提交审核失败：%d %v", status, out)
	}
}

func marketReview(t *testing.T, mux *http.ServeMux, actor yyPoolActor, id, action, reason string) (int, map[string]any) {
	t.Helper()
	return marketCall(t, mux, marketRequest("POST", "/api/market/plugins/"+id+"/review", map[string]any{
		"action": action, "reason": reason,
	}, actor))
}

func TestMarketLifecycleListMineAndFreeInstall(t *testing.T) {
	mux, dataDir := marketNewServer(t)
	alice := yyPoolActor{ID: "alice", Scope: "control"}
	bob := yyPoolActor{ID: "bob", Scope: "control"}
	admin := yyPoolActor{ID: "root", Admin: true, Scope: "control"}

	id := marketCreatePlugin(t, mux, alice, "市场闭环演示", 0)
	if status, out := marketUploadVersion(t, mux, alice, id, "1.0.0", marketTestZip(t, "market_demo"), "market_demo.zip"); status != 200 {
		t.Fatalf("上传版本失败：%d %v", status, out)
	}

	// 未提交审核前，公开列表与详情都看不到。
	status, out := marketCall(t, mux, marketRequest("GET", "/api/market/plugins", nil, yyPoolActor{}))
	if status != 200 || out["total"].(float64) != 0 {
		t.Fatalf("草稿不应出现在市场：%d %v", status, out)
	}
	if status, _ = marketCall(t, mux, marketRequest("GET", "/api/market/plugins/"+id, nil, bob)); status != 404 {
		t.Fatalf("他人不应看到草稿，得到 %d", status)
	}

	marketSubmit(t, mux, alice, id)
	if status, out = marketReview(t, mux, admin, id, "reject", ""); status != 400 {
		t.Fatalf("驳回必须带理由，得到 %d %v", status, out)
	}
	if status, out = marketReview(t, mux, admin, id, "reject", "截图缺失"); status != 200 || out["reject_reason"] != "截图缺失" {
		t.Fatalf("驳回失败：%d %v", status, out)
	}
	marketSubmit(t, mux, alice, id)
	if status, out = marketReview(t, mux, admin, id, "approve", ""); status != 200 || out["status"] != "approved" {
		t.Fatalf("通过失败：%d %v", status, out)
	}

	status, out = marketCall(t, mux, marketRequest("GET", "/api/market/plugins?q=市场&category=tool&sort=new", nil, yyPoolActor{}))
	if status != 200 || out["total"].(float64) != 1 {
		t.Fatalf("市场列表应有一条：%d %v", status, out)
	}
	item := out["items"].([]any)[0].(map[string]any)
	if item["free"] != true || item["price_cents"].(float64) != 0 {
		t.Fatalf("免费标记不对：%v", item)
	}
	if status, out = marketCall(t, mux, marketRequest("GET", "/api/market/plugins?mc_version=1.21&loader=Lua", nil, yyPoolActor{})); status != 200 || out["total"].(float64) != 1 {
		t.Fatalf("MC/加载器过滤失败：%d %v", status, out)
	}
	if status, out = marketCall(t, mux, marketRequest("GET", "/api/market/plugins?mc_version=1.19", nil, yyPoolActor{})); status != 200 || out["total"].(float64) != 0 {
		t.Fatalf("不匹配的 MC 版本不应返回：%d %v", status, out)
	}

	// 重新建一个 mux，验证 JSON 落盘后（含版本文件路径）仍能读取与安装。
	reloaded := http.NewServeMux()
	yyMarketRoutes(reloaded, dataDir)
	mux = reloaded

	status, out = marketCall(t, mux, marketRequest("GET", "/api/market/plugins/"+id, nil, yyPoolActor{}))
	if status != 200 {
		t.Fatalf("详情读取失败：%d %v", status, out)
	}
	plugin := out["plugin"].(map[string]any)
	versions := plugin["versions"].([]any)
	if len(versions) != 1 {
		t.Fatalf("版本数量不对：%v", versions)
	}
	version := versions[0].(map[string]any)
	if version["sha256"] == "" || version["file_name"] != "market_demo.zip" {
		t.Fatalf("版本元数据缺失：%v", version)
	}

	// 安装走真实 ImportPackage + ScanPlugins，落到临时插件目录。
	pc := NewPluginController()
	pluginRoot := t.TempDir()
	pc.SetPluginDirs(pluginRoot, "")
	oldController := pluginController
	pluginController = pc
	t.Cleanup(func() { pluginController = oldController })

	status, out = marketCall(t, mux, marketRequest("POST", "/api/market/plugins/"+id+"/install", map[string]any{"version": "1.0.0"}, yyPoolActor{}))
	if status != 200 || out["installed"] != true {
		t.Fatalf("免费插件安装失败：%d %v", status, out)
	}
	if _, err := os.Stat(filepath.Join(pluginRoot, "插件文件", "market_demo", "main.lua")); err != nil {
		t.Fatalf("插件没有落到本机目录：%v", err)
	}

	status, out = marketCall(t, mux, marketRequest("GET", "/api/market/mine", nil, alice))
	if status != 200 {
		t.Fatalf("我的插件读取失败：%d %v", status, out)
	}
	summary := out["summary"].(map[string]any)
	if summary["installs"].(float64) != 1 || summary["gross_cents"].(float64) != 0 {
		t.Fatalf("收益/安装统计不对：%v", summary)
	}
	if summary["platform_fee_cents"].(float64) != 0 || summary["developer_net_cents"].(float64) != 0 {
		t.Fatalf("没有订单时分成必须为 0：%v", summary)
	}
	// 他人看不到 alice 的“我的插件”。
	status, out = marketCall(t, mux, marketRequest("GET", "/api/market/mine", nil, bob))
	if status != 200 || len(out["items"].([]any)) != 0 {
		t.Fatalf("租户隔离失败：%d %v", status, out)
	}
}

func TestMarketPaidInstallRefusesWithoutPayment(t *testing.T) {
	mux, _ := marketNewServer(t)
	alice := yyPoolActor{ID: "alice", Scope: "control"}
	admin := yyPoolActor{ID: "root", Admin: true, Scope: "control"}
	id := marketCreatePlugin(t, mux, alice, "付费插件", 500)
	if status, out := marketUploadVersion(t, mux, alice, id, "1.0.0", marketTestZip(t, "paid_demo"), "paid_demo.zip"); status != 200 {
		t.Fatalf("上传失败：%d %v", status, out)
	}
	marketSubmit(t, mux, alice, id)
	if status, out := marketReview(t, mux, admin, id, "approve", ""); status != 200 {
		t.Fatalf("审核失败：%d %v", status, out)
	}
	status, out := marketCall(t, mux, marketRequest("POST", "/api/market/plugins/"+id+"/install", map[string]any{}, yyPoolActor{}))
	if status != http.StatusPaymentRequired || out["code"] != "payment_not_connected" {
		t.Fatalf("付费插件必须明确拒绝：%d %v", status, out)
	}
	_, mine := marketCall(t, mux, marketRequest("GET", "/api/market/mine", nil, alice))
	if mine["summary"].(map[string]any)["installs"].(float64) != 0 {
		t.Fatal("支付未接入时安装量不能增加")
	}
}

func TestMarketUploadValidation(t *testing.T) {
	mux, _ := marketNewServer(t)
	alice := yyPoolActor{ID: "alice", Scope: "control"}
	id := marketCreatePlugin(t, mux, alice, "校验插件", 0)

	status, out := marketUploadVersion(t, mux, alice, id, "1.0.0", []byte("MZ"), "bad.exe")
	if status != http.StatusUnsupportedMediaType || out["code"] != "unsupported_file" {
		t.Fatalf("非法扩展名应 415：%d %v", status, out)
	}
	status, out = marketUploadVersion(t, mux, alice, id, "1.0.0", []byte(""), "empty.zip")
	if status != 400 {
		t.Fatalf("空文件应 400：%d %v", status, out)
	}
	status, out = marketUploadVersion(t, mux, alice, id, "1.0.0", []byte("PK"), "../escape.zip")
	if status != 400 {
		t.Fatalf("路径穿越文件名应 400：%d %v", status, out)
	}

	oldMax := yyMarketMaxUploadBytes
	yyMarketMaxUploadBytes = 4
	t.Cleanup(func() { yyMarketMaxUploadBytes = oldMax })
	status, out = marketUploadVersion(t, mux, alice, id, "1.0.0", []byte("12345"), "big.zip")
	if status != http.StatusRequestEntityTooLarge || out["code"] != "too_large" {
		t.Fatalf("超限应 413：%d %v", status, out)
	}
	yyMarketMaxUploadBytes = oldMax

	if status, out = marketUploadVersion(t, mux, alice, id, "1.0.0", []byte("PK"), "ok.zip"); status != 200 {
		t.Fatalf("正常上传失败：%d %v", status, out)
	}
	status, out = marketUploadVersion(t, mux, alice, id, "1.0.0", []byte("PK"), "ok.zip")
	if status != http.StatusConflict {
		t.Fatalf("重复版本应 409：%d %v", status, out)
	}
}

func TestMarketTenantIsolationAndReviewPermissions(t *testing.T) {
	mux, _ := marketNewServer(t)
	alice := yyPoolActor{ID: "alice", Scope: "control"}
	bob := yyPoolActor{ID: "bob", Scope: "control"}

	id := marketCreatePlugin(t, mux, alice, "隔离插件", 0)
	if status, out := marketUploadVersion(t, mux, bob, id, "1.0.0", []byte("PK"), "x.zip"); status != 403 {
		t.Fatalf("他人上传应 403：%d %v", status, out)
	}
	if status, out := marketCall(t, mux, marketRequest("POST", "/api/market/plugins/"+id+"/submit", map[string]any{}, bob)); status != 403 {
		t.Fatalf("他人提交应 403：%d %v", status, out)
	}
	if status, out := marketReview(t, mux, bob, id, "approve", ""); status != 403 {
		t.Fatalf("非管理员审核应 403：%d %v", status, out)
	}

	// 没有版本不能提交。
	if status, out := marketCall(t, mux, marketRequest("POST", "/api/market/plugins/"+id+"/submit", map[string]any{}, alice)); status != 400 {
		t.Fatalf("无版本提交应 400：%d %v", status, out)
	}
	// 创建必须登录。
	if status, _ := marketCall(t, mux, marketRequest("POST", "/api/market/plugins", map[string]any{"name": "匿名"}, yyPoolActor{})); status != 401 {
		t.Fatalf("未登录创建应 401，得到 %d", status)
	}
	// 只读会话不能写。
	reader := yyPoolActor{ID: "alice", Scope: "read"}
	if status, out := marketCall(t, mux, marketRequest("POST", "/api/market/plugins", map[string]any{"name": "只读会话"}, reader)); status != 403 {
		t.Fatalf("只读会话创建应 403：%d %v", status, out)
	}
}

func TestMarketSplitInteger(t *testing.T) {
	cases := []struct {
		gross, net, fee int64
	}{
		{0, 0, 0},
		{1, 0, 1},
		{500, 350, 150},
		{999, 699, 300},
		{1000, 700, 300},
	}
	for _, c := range cases {
		net, fee := yyMarketSplit(c.gross)
		if net != c.net || fee != c.fee {
			t.Fatalf("gross=%d 得到 net=%d fee=%d，期望 %d/%d", c.gross, net, fee, c.net, c.fee)
		}
		if net+fee != c.gross {
			t.Fatalf("gross=%d 分成不守恒：%d+%d", c.gross, net, fee)
		}
	}
}

func TestCatalogMarketRoutes(t *testing.T) {
	paths := map[string]bool{}
	for _, route := range yyAPIRoutes {
		paths[route.Path] = true
	}
	for _, path := range []string{"/api/market/plugins", "/api/market/plugins/", "/api/market/mine"} {
		if !paths[path] {
			t.Fatalf("catalog 缺少 %s", path)
		}
	}
	if !yyMCPReadPaths["/api/market/plugins"] {
		t.Fatal("市场列表应进入只读白名单")
	}
	if yyMCPReadPaths["/api/market/mine"] {
		t.Fatal("我的收益涉及账号身份，不应进入只读白名单")
	}
}
