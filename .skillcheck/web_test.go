package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// 注册表：页面注册/去重/移除。
func TestWebRegistryRegisterAndRemove(t *testing.T) {
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webClients = map[string]map[*webClient]bool{}
	webMu.Unlock()

	if err := registerWebPage(&webPage{Plugin: "p1", Name: "控制台", Kind: "declarative"}); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if err := registerWebPage(&webPage{Plugin: "p1", Name: "控制台", Kind: "declarative"}); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Fatalf("重名应报错, got: %v", err)
	}
	if err := registerWebPage(&webPage{Plugin: "p1", Name: "a/b", Kind: "declarative"}); err == nil {
		t.Fatal("页面名含 / 应报错")
	}
	_ = registerWebPage(&webPage{Plugin: "p2", Name: "其他页", Kind: "html", Entry: "index.html"})
	if len(listWebPages()) != 2 {
		t.Fatalf("应列出 2 个页面, got %d", len(listWebPages()))
	}
	if err := registerWebAction("p1", "控制台", "refresh", "onRefresh"); err != nil {
		t.Fatalf("动作注册失败: %v", err)
	}
	if err := registerWebAction("p1", "控制台", "refresh", "onRefresh2"); err == nil {
		t.Fatal("重复动作应报错")
	}
	if _, ok := lookupWebAction("p1", "控制台", "refresh"); !ok {
		t.Fatal("动作应可查到")
	}
	RemovePluginWeb("p1")
	if _, ok := getWebPage("p1", "控制台"); ok {
		t.Fatal("移除后页面应消失")
	}
	if _, ok := lookupWebAction("p1", "控制台", "refresh"); ok {
		t.Fatal("移除后动作应消失")
	}
}

// SSE 客户端：订阅收到 pushWebEvent；RemovePluginWeb 关闭通道。
func TestWebPushAndChannelClose(t *testing.T) {
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webClients = map[string]map[*webClient]bool{}
	webMu.Unlock()
	// 订阅要求插件已注册页面（原子订阅：无页面直接返回已关闭通道）
	_ = registerWebPage(&webPage{Plugin: "p1", Name: "pg", Kind: "declarative"})
	defer RemovePluginWeb("p1")

	c := subscribeWebEvents("p1")
	defer unsubscribeWebEvents("p1", c)
	pushWebEvent("p1", "pg", `{"x":1}`)
	select {
	case msg := <-c.ch:
		if !strings.Contains(msg, "event: pg") || !strings.Contains(msg, `"x":1`) {
			t.Fatalf("SSE 消息格式错误: %q", msg)
		}
	default:
		t.Fatal("应收到推送")
	}
	RemovePluginWeb("p1")
	select {
	case _, open := <-c.ch:
		if open {
			t.Fatal("移除后通道应关闭")
		}
	default:
		t.Fatal("移除后通道应立即关闭")
	}
}

// Lua web 表：page/action 注册，重跑上下文跳过；动作调用参数与返回值。
func TestWebLuaAPIRegisterAndCall(t *testing.T) {
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webMu.Unlock()
	bm := &BotManager{}
	src := `
		web.page("p1", "d", {
			{type="text", key="info"},
			{type="button", label="刷新", action="refresh"},
		})
		web.action("p1", "echo", "onEcho")
		function onEcho(req)
			return {got = req.x, list = {{1, 2}, {3, 4}}}
		end
	`
	if err := LoadLuaPlugin(bm, "wapi", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	defer RemovePluginWeb("wapi")
	pg, ok := getWebPage("wapi", "p1")
	if !ok || len(pg.Controls) != 2 {
		t.Fatalf("页面/控件未注册: %+v", pg)
	}
	result, err := callWebAction(bm, "wapi", "p1", "echo", map[string]any{"x": "hi"})
	if err != nil {
		t.Fatalf("callWebAction: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok || m["got"] != "hi" {
		t.Fatalf("返回值错误: %+v", result)
	}
	arr, ok := m["list"].([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("list 应为数组: %+v", m["list"])
	}
	if _, err := callWebAction(bm, "wapi", "p1", "nope", nil); err == nil {
		t.Fatal("未注册动作应报错")
	}
}

// web.emit 在带插件上下文的 LState 中推送（handler 场景模拟）。
func TestWebLuaEmit(t *testing.T) {
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webClients = map[string]map[*webClient]bool{}
	webMu.Unlock()
	bm := &BotManager{}
	if err := LoadLuaPlugin(bm, "wemit", `web.page("p1", "d", {})`); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	defer RemovePluginWeb("wemit")
	c := subscribeWebEvents("wemit")
	defer unsubscribeWebEvents("wemit", c)

	L := newLuaState(bm)
	L.SetGlobal("__PLUGIN_ID__", lua.LString("wemit"))
	err := L.DoString(`web.emit("p1", "data", {list = {{"张三", 100}}, info = "ok"})`)
	L.Close()
	if err != nil {
		t.Fatalf("emit 失败: %v", err)
	}
	select {
	case msg := <-c.ch:
		if !strings.Contains(msg, `event: p1`) || !strings.Contains(msg, `"list"`) {
			t.Fatalf("SSE 消息格式错误: %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未收到 SSE 事件")
	}
}

// 混合键表不得当数组（防静默丢键）。
func TestLuaToGoMixedKeyTable(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	defer cancelLuaCtx(L)
	if err := L.DoString(`t = {10, 20, x = 30}`); err != nil {
		t.Fatal(err)
	}
	m, ok := luaToGo(L.GetGlobal("t")).(map[string]any)
	if !ok || m["x"] != float64(30) || m["1"] != float64(10) {
		t.Fatalf("混合键表应转为对象: %+v", luaToGo(L.GetGlobal("t")))
	}
}

// web.serve 注册 html 页；页面名控制字符与动作空字段被拒。
func TestWebServeAndValidation(t *testing.T) {
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webMu.Unlock()
	bm := &BotManager{}
	if err := LoadLuaPlugin(bm, "wserv", `web.serve("仪表盘")`); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	defer RemovePluginWeb("wserv")
	pg, ok := getWebPage("wserv", "仪表盘")
	if !ok || pg.Kind != "html" || pg.Entry != "index.html" {
		t.Fatalf("web.serve 页面错误: %+v", pg)
	}
	if err := registerWebPage(&webPage{Plugin: "x", Name: "a\nb", Kind: "declarative"}); err == nil {
		t.Fatal("页面名含控制字符应报错")
	}
	if err := registerWebAction("x", "", "a", "h"); err == nil {
		t.Fatal("空页面名应报错")
	}
	if err := registerWebAction("x", "p", "a", ""); err == nil {
		t.Fatal("空函数名应报错")
	}
}

// handler 报错通过 callWebAction 以 error 传出。
func TestCallWebActionHandlerError(t *testing.T) {
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webMu.Unlock()
	bm := &BotManager{}
	src := `
		web.page("p1", "d", {})
		web.action("p1", "boom", "onBoom")
		function onBoom(req) error("炸了") end
	`
	if err := LoadLuaPlugin(bm, "werr", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	defer RemovePluginWeb("werr")
	if _, err := callWebAction(bm, "werr", "p1", "boom", nil); err == nil || !strings.Contains(err.Error(), "炸了") {
		t.Fatalf("handler 错误应传出, got: %v", err)
	}
}

// 门户列出所有插件页面；声明式页面渲染控件。
func TestWebPortalAndDeclarativeRender(t *testing.T) {
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webMu.Unlock()
	bm := &BotManager{}
	src := `
		web.page("控制台", "测试页面", {
			{type="text", key="info", label="信息"},
			{type="button", label="刷新", action="refresh"},
			{type="input", key="player", placeholder="玩家名"},
			{type="table", key="list", columns={"玩家", "金币"}},
		})
	`
	if err := LoadLuaPlugin(bm, "wportal", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	defer RemovePluginWeb("wportal")

	rr := httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/", nil))
	if !strings.Contains(rr.Body.String(), "控制台") || !strings.Contains(rr.Body.String(), "wportal") {
		t.Fatalf("门户未列出页面: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/p/wportal/控制台", nil))
	body := rr.Body.String()
	for _, want := range []string{`data-key="info"`, `data-action="refresh"`, `data-key="player"`, `data-key="list"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("页面缺控件 %s: %s", want, body)
		}
	}

	rr = httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/p/wportal/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("未注册页面应 404: %d", rr.Code)
	}
}

// 动作端点：POST JSON 参数传 handler、返回值回传；未注册动作 500+ok:false；GET query 形式。
func TestWebActionEndpoint(t *testing.T) {
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webMu.Unlock()
	// handleWebAPI 要求 botMgr 非空（否则返回"机器人未连接"）
	botMgrMu.Lock()
	oldBm := botMgr
	botMgr = &BotManager{}
	botMgrMu.Unlock()
	defer func() {
		botMgrMu.Lock()
		botMgr = oldBm
		botMgrMu.Unlock()
	}()
	bm := &BotManager{}
	src := `
		web.page("p1", "d", {})
		web.action("p1", "echo", "onEcho")
		function onEcho(req)
			return {got = req.x, n = 42}
		end
	`
	if err := LoadLuaPlugin(bm, "wact", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	defer RemovePluginWeb("wact")

	rr := httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("POST", "/web/api/wact/p1", strings.NewReader(`{"action":"echo","args":{"x":"hello"}}`)))
	var res struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil || !res.OK {
		t.Fatalf("action 失败: %s %v", rr.Body.String(), err)
	}
	if res.Data["got"] != "hello" {
		t.Fatalf("参数未传到 handler: %+v", res.Data)
	}

	rr = httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("POST", "/web/api/wact/p1", strings.NewReader(`{"action":"nope"}`)))
	if rr.Code != http.StatusNotFound || !strings.Contains(rr.Body.String(), "未注册动作") {
		t.Fatalf("未注册动作应 404: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/api/wact/p1?action=echo&x=qq", nil))
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil || !res.OK || res.Data["got"] != "qq" {
		t.Fatalf("GET query 形式失败: %s %v", rr.Body.String(), err)
	}
}

// SSE 端点：写 :ok 头后流式等待；上下文取消即退出。
func TestWebEventsHTTP(t *testing.T) {
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webClients = map[string]map[*webClient]bool{}
	webMu.Unlock()
	bm := &BotManager{}
	if err := LoadLuaPlugin(bm, "wess", `web.page("p1", "d", {})`); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	defer RemovePluginWeb("wess")

	req := httptest.NewRequest("GET", "/web/events/wess", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handleWeb(rr, req)
		close(done)
	}()
	time.Sleep(80 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler 未随上下文取消退出")
	}
	body := rr.Body.String()
	if !strings.Contains(body, ":ok") || !strings.Contains(rr.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("SSE 头/首帧错误: %s", body)
	}
}

// 密码门卫：设 web_password 后，页面无 cookie 出登录表单；POST 正确密码种 cookie；
// 带 cookie 直通；API 无 cookie 401。
func TestWebPasswordGate(t *testing.T) {
	pluginController.SetPluginDirs(t.TempDir(), "")
	if err := pluginController.WriteConfig("wpass", map[string]any{"web_password": "1234"}); err != nil {
		t.Fatal(err)
	}
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webMu.Unlock()
	_ = registerWebPage(&webPage{Plugin: "wpass", Name: "p1", Kind: "declarative"})
	defer RemovePluginWeb("wpass")

	rr := httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/p/wpass/p1", nil))
	if !strings.Contains(rr.Body.String(), "需要密码") {
		t.Fatalf("应显示登录表单: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/web/p/wpass/p1", strings.NewReader("password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handleWeb(rr, req)
	if !strings.Contains(rr.Body.String(), "密码错误") {
		t.Fatalf("错误密码应提示: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/web/p/wpass/p1", strings.NewReader("password=1234"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handleWeb(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("正确密码应 302: %d", rr.Code)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != webCookieName("wpass") {
		t.Fatalf("应种 cookie: %+v", cookies)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/web/p/wpass/p1", nil)
	req.AddCookie(cookies[0])
	handleWeb(rr, req)
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "需要密码") {
		t.Fatalf("带 cookie 应直接进入: %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("POST", "/web/api/wpass/p1", strings.NewReader(`{"action":"x"}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("API 无 cookie 应 401: %d", rr.Code)
	}

	_ = registerWebPage(&webPage{Plugin: "wopen", Name: "p1", Kind: "declarative"})
	defer RemovePluginWeb("wopen")
	rr = httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/p/wopen/p1", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("无密码插件应直通: %d", rr.Code)
	}
}

// 自定义 HTML 托管：www 入口与静态资源可访问；路径穿越 404。
func TestWebStaticAndTraversal(t *testing.T) {
	tmp := t.TempDir()
	pluginController.SetPluginDirs(tmp, "")
	os.MkdirAll(filepath.Join(tmp, "插件文件", "wstat", "www"), 0755)
	os.WriteFile(filepath.Join(tmp, "插件文件", "wstat", "www", "index.html"), []byte("<h1>自定义</h1>"), 0644)
	os.WriteFile(filepath.Join(tmp, "插件文件", "wstat", "www", "a.css"), []byte("body{}"), 0644)

	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webMu.Unlock()
	_ = registerWebPage(&webPage{Plugin: "wstat", Name: "p1", Kind: "html", Entry: "index.html"})
	defer RemovePluginWeb("wstat")

	rr := httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/p/wstat/p1", nil))
	if !strings.Contains(rr.Body.String(), "自定义") {
		t.Fatalf("html 入口未返回: %s", rr.Body.String())
	}
	rr = httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/static/wstat/a.css", nil))
	if rr.Body.String() != "body{}" {
		t.Fatalf("静态资源错误: %q", rr.Body.String())
	}
	rr = httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/static/wstat/../main.go", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("路径穿越应 404: %d", rr.Code)
	}
}

// 控制器停止插件后，页面与动作被清理。
func TestWebPageRemovedOnPluginStop(t *testing.T) {
	pluginController.SetPluginDirs(t.TempDir(), "")
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webClients = map[string]map[*webClient]bool{}
	webMu.Unlock()
	bm := &BotManager{}
	src := `
		web.page("p1", "d", {})
		web.action("p1", "a", "onA")
		function onA(req) return {ok=1} end
	`
	def := &PluginDef{ID: "wlife", Load: func(bm *BotManager) error { return LoadLuaPlugin(bm, "wlife", src) }}
	if err := pluginController.AddDef(def); err != nil {
		t.Fatal(err)
	}
	if err := pluginController.Run(bm, "wlife"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok := getWebPage("wlife", "p1"); !ok {
		t.Fatal("页面应已注册")
	}
	if err := pluginController.Stop(bm, "wlife"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, ok := getWebPage("wlife", "p1"); ok {
		t.Fatal("停止后页面应被移除")
	}
	if _, ok := lookupWebAction("wlife", "p1", "a"); ok {
		t.Fatal("停止后动作应被移除")
	}
	pluginController.Delete(bm, "wlife")
}

// SSE 对设了密码且无 cookie 的插件返回 401。
func TestWebEventsPassword401(t *testing.T) {
	pluginController.SetPluginDirs(t.TempDir(), "")
	if err := pluginController.WriteConfig("wssp", map[string]any{"web_password": "pw"}); err != nil {
		t.Fatal(err)
	}
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webClients = map[string]map[*webClient]bool{}
	webMu.Unlock()
	_ = registerWebPage(&webPage{Plugin: "wssp", Name: "p1", Kind: "declarative"})
	defer RemovePluginWeb("wssp")

	rr := httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/events/wssp", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("SSE 无 cookie 应 401: %d", rr.Code)
	}
	// 未注册插件 → 404
	rr = httptest.NewRecorder()
	handleWeb(rr, httptest.NewRequest("GET", "/web/events/nosuch", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("未注册插件 SSE 应 404: %d", rr.Code)
	}
}

// action handler 死循环被执行超时中断（注入小超时验证 HTTP 路径的保护）。
func TestWebActionTimeout(t *testing.T) {
	old := luaExecTimeout
	luaExecTimeout = 100 * time.Millisecond
	defer func() { luaExecTimeout = old }()
	webMu.Lock()
	webPages = map[string]map[string]*webPage{}
	webActions = map[string][]*webActionH{}
	webMu.Unlock()
	// handleWebAPI 要求 botMgr 非空（否则返回"机器人未连接"）
	botMgrMu.Lock()
	oldBm := botMgr
	botMgr = &BotManager{}
	botMgrMu.Unlock()
	defer func() {
		botMgrMu.Lock()
		botMgr = oldBm
		botMgrMu.Unlock()
	}()
	bm := &BotManager{}
	src := `
		web.page("p1", "d", {})
		web.action("p1", "spin", "onSpin")
		function onSpin(req) while true do end end
	`
	if err := LoadLuaPlugin(bm, "wtime", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	defer RemovePluginWeb("wtime")
	rr := httptest.NewRecorder()
	start := time.Now()
	handleWeb(rr, httptest.NewRequest("POST", "/web/api/wtime/p1", strings.NewReader(`{"action":"spin"}`)))
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("超时中断过慢: %v", elapsed)
	}
	if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), `"ok":false`) {
		t.Fatalf("超时应返回错误: %d %s", rr.Code, rr.Body.String())
	}
}

// 中文插件名的密码 cookie 名必须可被浏览器接受（纯 ASCII token 字符）。
func TestWebCookieNameASCII(t *testing.T) {
	for _, id := range []string{"系统测试", "core_utils", "a-b.c"} {
		name := webCookieName(id)
		for _, r := range name {
			if r < 0x21 || r > 0x7e {
				t.Fatalf("cookie 名 %q 含非 ASCII 字符（会被 SetCookie 静默丢弃）", name)
			}
		}
		if !strings.HasPrefix(name, "web_auth_") {
			t.Fatalf("cookie 名应带前缀: %q", name)
		}
	}
}
