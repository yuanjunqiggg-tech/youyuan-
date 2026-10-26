package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 这些测试全部不监听端口：只用 t.TempDir() 与 httptest.NewRecorder，
// 避免与正在运行的服务抢端口（硬约束：不许跑 go test -count=1 .）。

func yyThemeTestWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败：%v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写文件失败：%v", err)
	}
}

func yyThemeTestManifest(id string, extra string) string {
	return `{"id":"` + id + `","name":"极光","version":"1.0.0","author":"社区","mcort_theme_api":1` + extra + `}`
}

// ── 品牌硬编码守卫 ──

func TestYYThemeGuardRejectsBrandRemoval(t *testing.T) {
	cases := []struct {
		name string
		css  string
		js   string
		want bool // 是否应被拒绝
	}{
		{"裸隐藏", `.yy-brand-mark{display:none}`, "", true},
		{"注释绕过", `.yy-brand-mark{/* keep */display:none!important}`, "", true},
		{"空白绕过", ".yy-brand-mark \n {\n display : none ;\n }", "", true},
		{"透明隐藏", `#mcort-brand-signature{opacity:0}`, "", true},
		{"零字号", `.mcort-brand{font-size:0}`, "", true},
		{"重定义品牌变量", `:root{--mcort-brand:#000}`, "", true},
		{"带前缀的变量", `:root{--mcort-brand-color:red}`, "", true},
		{"JS 移除节点", "", `document.querySelector('.yy-brand-mark').remove()`, true},
		{"JS 改 display", "", `document.getElementById('mcort-brand-signature').style.display='none'`, true},
		{"只改颜色", `.yy-brand-mark{color:#fff;background:#000}`, "", false},
		{"注释里提到", `/* .yy-brand-mark{display:none} 这是笔记 */ .x{color:red}`, "", false},
		{"正常主题", `:root{--mcort-theme-primary:#0af}.yyq-root{--phx-primary:#0af}`, "", false},
	}
	for _, c := range cases {
		err := yyThemeGuardText(c.css)
		if c.js != "" {
			err = yyThemeGuardText(c.js)
		}
		if c.want && err == nil {
			t.Errorf("%s：应当拒绝，实际通过", c.name)
		}
		if !c.want && err != nil {
			t.Errorf("%s：不应当拒绝，实际 %v", c.name, err)
		}
	}
}

func TestYYThemeValidateManifest(t *testing.T) {
	base := yyThemeManifest{ID: "aurora", Name: "极光", Version: "1.0.0", API: 1}
	if err := yyThemeValidate("aurora", base, ":root{}", ""); err != nil {
		t.Fatalf("合法清单被拒：%v", err)
	}
	bad := base
	bad.RemoveBrand = true
	if err := yyThemeValidate("aurora", bad, ":root{}", ""); err == nil || !strings.Contains(err.Error(), "remove_brand") {
		t.Fatalf("remove_brand 应被拒，得到 %v", err)
	}
	bad = base
	bad.HideAuthor = true
	if err := yyThemeValidate("aurora", bad, ":root{}", ""); err == nil || !strings.Contains(err.Error(), "hide_author") {
		t.Fatalf("hide_author 应被拒，得到 %v", err)
	}
	bad = base
	bad.ID = "other"
	if err := yyThemeValidate("aurora", bad, ":root{}", ""); err == nil {
		t.Fatalf("id 与目录名不一致应被拒")
	}
	bad = base
	bad.API = 2
	if err := yyThemeValidate("aurora", bad, ":root{}", ""); err == nil {
		t.Fatalf("不支持的 api 版本应被拒")
	}
	bad = base
	bad.Requires = []string{"unknown-module"}
	if err := yyThemeValidate("aurora", bad, ":root{}", ""); err == nil {
		t.Fatalf("未知依赖应被拒")
	}
	bad = base
	bad.Variables = map[string]string{"--phx-primary": "#000"}
	if err := yyThemeValidate("aurora", bad, ":root{}", ""); err == nil {
		t.Fatalf("越权变量应被拒")
	}
	bad = base
	bad.Variables = map[string]string{"--mcort-theme-primary": "#000;position:fixed"}
	if err := yyThemeValidate("aurora", bad, ":root{}", ""); err == nil {
		t.Fatalf("变量值注入应被拒")
	}
	bad = base
	bad.Slots = map[string]string{"header": "<img src=x onerror=alert(1)>"}
	if err := yyThemeValidate("aurora", bad, ":root{}", ""); err == nil {
		t.Fatalf("槽位不允许 HTML")
	}
	ok := base
	ok.Variables = map[string]string{"--mcort-theme-primary": "#00aaff", "--mcort-theme-radius": "10px"}
	ok.Slots = map[string]string{"header": "极光 v1", "floating": "assets/wid.png"}
	if err := yyThemeValidate("aurora", ok, ":root{--mcort-theme-primary:#00aaff}", ""); err != nil {
		t.Fatalf("合法主题被拒：%v", err)
	}
}

// ── 扫描 / 装载 ──

func TestYYThemeScan(t *testing.T) {
	dir := t.TempDir()
	yyThemeTestWrite(t, filepath.Join(dir, "themes", "aurora", "theme.json"), yyThemeTestManifest("aurora", `,"requires":["theme-core"],"slots":{"header":"极光"}`))
	yyThemeTestWrite(t, filepath.Join(dir, "themes", "aurora", "theme.css"), `:root{--mcort-theme-primary:#0af}`)
	yyThemeTestWrite(t, filepath.Join(dir, "themes", "bad", "theme.json"), yyThemeTestManifest("bad", `,"remove_brand":true`))
	yyThemeTestWrite(t, filepath.Join(dir, "themes", "bad", "theme.css"), `:root{}`)
	yyThemeTestWrite(t, filepath.Join(dir, "themes", "half", "theme.json"), yyThemeTestManifest("half", ""))
	yyThemeTestWrite(t, filepath.Join(dir, "themes", "broken", "theme.json"), `{not json`)
	yyThemeTestWrite(t, filepath.Join(dir, "themes", "broken", "theme.css"), `:root{}`)

	themes := yyThemeScan(dir)
	if len(themes) != 4 {
		t.Fatalf("应发现 4 个主题目录，得到 %d", len(themes))
	}
	got := map[string]yyThemeInfo{}
	for _, th := range themes {
		got[th.ID] = th
	}
	if !got["aurora"].OK {
		t.Fatalf("aurora 应通过：%+v", got["aurora"])
	}
	if got["bad"].OK || !strings.Contains(got["bad"].Error, "remove_brand") {
		t.Fatalf("bad 应因 remove_brand 被拒：%+v", got["bad"])
	}
	if got["half"].OK || !strings.Contains(got["half"].Error, "theme.css") {
		t.Fatalf("half 应因缺 theme.css 被拒：%+v", got["half"])
	}
	if got["broken"].OK {
		t.Fatalf("broken 应被拒")
	}
	if !got["aurora"].HasCSS || got["aurora"].Slots["header"] != "极光" {
		t.Fatalf("aurora 元信息丢了：%+v", got["aurora"])
	}
}

func TestYYThemeLoadActiveFallback(t *testing.T) {
	dir := t.TempDir()
	// 没有主题目录，但 active 指向一个不存在的 id：必须回退为空
	yyThemeTestWrite(t, filepath.Join(dir, "theme-active.json"), `{"active":"ghost"}`)
	yyThemeLoad(dir)
	_, active := yyThemeSnapshot()
	if active != "" {
		t.Fatalf("不可用的主题不应保持激活，得到 %q", active)
	}
}

// ── 资源路径与 CSS 重写 ──

func TestYYThemeAssetFile(t *testing.T) {
	dir := t.TempDir()
	ok := []string{"theme.css", "theme.js", "assets/bg.png", "assets/sub/a.webp"}
	for _, f := range ok {
		if _, good := yyThemeAssetFile(dir, f); !good {
			t.Errorf("%s 应允许", f)
		}
	}
	bad := []string{"", "../x", "assets/../../etc/passwd", "/etc/passwd", "main.js", "assets\\..\\x", "theme.json"}
	for _, f := range bad {
		if _, good := yyThemeAssetFile(dir, f); good {
			t.Errorf("%s 应拒绝", f)
		}
	}
}

func TestYYThemeRewriteCSS(t *testing.T) {
	in := `body{background:url("assets/bg.png")} .a{background:url(assets/sub/a.png)} ` +
		`.b{background:url(https://cdn.example.com/x.png)} .c{background:url(/static/y.png)} ` +
		`.d{background:url(data:image/png;base64,AAAA)}`
	out := yyThemeRewriteCSS(in, "aurora")
	if !strings.Contains(out, "/api/mcort/theme/asset?theme=aurora&file=assets%2Fbg.png") {
		t.Fatalf("相对路径未重写：%s", out)
	}
	if !strings.Contains(out, "assets%2Fsub%2Fa.png") {
		t.Fatalf("子目录未重写：%s", out)
	}
	if !strings.Contains(out, "url(https://cdn.example.com/x.png)") {
		t.Fatalf("外链不应被改写：%s", out)
	}
	if !strings.Contains(out, "url(/static/y.png)") {
		t.Fatalf("站内绝对路径不应被改写：%s", out)
	}
	if !strings.Contains(out, "data:image/png;base64,AAAA") {
		t.Fatalf("data URI 不应被改写：%s", out)
	}
}

// ── 公告存储与接口 ──

func TestYYAnnouncementStore(t *testing.T) {
	store := yyNewAnnouncementStore(t.TempDir())
	a, err := store.Upsert(yyAnnouncement{Title: "开服", Content: "今晚八点"}, "create")
	if err != nil {
		t.Fatalf("创建失败：%v", err)
	}
	if a.ID == "" || a.Author != "疣螈" && a.Author != "" {
		t.Fatalf("创建结果异常：%+v", a)
	}
	b, err := store.Upsert(yyAnnouncement{Title: "置顶", Content: "看这里", Pinned: true}, "create")
	if err != nil {
		t.Fatalf("创建失败：%v", err)
	}
	list := store.List()
	if len(list) != 2 || list[0].ID != b.ID {
		t.Fatalf("置顶应排在最前：%+v", list)
	}
	if _, err = store.Upsert(yyAnnouncement{ID: a.ID, Title: "开服（改）"}, "update"); err != nil {
		t.Fatalf("更新失败：%v", err)
	}
	found := false
	for _, it := range store.List() {
		if it.ID == a.ID && it.Title == "开服（改）" {
			found = true
		}
	}
	if !found {
		t.Fatalf("更新未生效")
	}
	if _, err = store.Upsert(yyAnnouncement{ID: a.ID}, "delete"); err != nil {
		t.Fatalf("删除失败：%v", err)
	}
	if len(store.List()) != 1 {
		t.Fatalf("删除后应剩 1 条")
	}
	if _, err = store.Upsert(yyAnnouncement{ID: "nope"}, "delete"); err == nil {
		t.Fatalf("删除不存在的公告应报错")
	}
	if _, err = store.Upsert(yyAnnouncement{ID: a.ID}, "weird"); err == nil {
		t.Fatalf("未知 op 应报错")
	}
}

// yyThemeTestNoIdentity 把平台身份清空，保证管理员判定只走回环分支。
func yyThemeTestNoIdentity(t *testing.T) {
	t.Helper()
	yyIdentity.Lock()
	saved := yyIdentity.Data
	yyIdentity.Data = yyPlatformIdentity{}
	yyIdentity.Unlock()
	t.Cleanup(func() {
		yyIdentity.Lock()
		yyIdentity.Data = saved
		yyIdentity.Unlock()
	})
}

func TestYYAnnouncementHandlerAdmin(t *testing.T) {
	yyThemeTestNoIdentity(t)
	store := yyNewAnnouncementStore(t.TempDir())
	h := yyThemeAnnouncements(store)

	remote := httptest.NewRequest("POST", "/api/mcort/announcements",
		bytes.NewBufferString(`{"title":"越权","content":"x"}`))
	remote.RemoteAddr = "203.0.113.9:5000"
	rw := httptest.NewRecorder()
	h(rw, remote)
	if rw.Code != http.StatusForbidden {
		t.Fatalf("非本机非管理员应 403，得到 %d %s", rw.Code, rw.Body.String())
	}

	local := httptest.NewRequest("POST", "/api/mcort/announcements",
		bytes.NewBufferString(`{"title":"开服","content":"今晚八点","severity":"warning","pinned":true}`))
	local.RemoteAddr = "127.0.0.1:5000"
	rw = httptest.NewRecorder()
	h(rw, local)
	if rw.Code != 200 || !strings.Contains(rw.Body.String(), `"ok":true`) {
		t.Fatalf("本机应允许发布：%d %s", rw.Code, rw.Body.String())
	}

	get := httptest.NewRequest("GET", "/api/mcort/announcements", nil)
	get.RemoteAddr = "127.0.0.1:5000"
	rw = httptest.NewRecorder()
	h(rw, get)
	if rw.Code != 200 {
		t.Fatalf("读取失败：%d", rw.Code)
	}
	var out struct {
		OK            bool             `json:"ok"`
		Admin         bool             `json:"admin"`
		Brand         string           `json:"brand"`
		Announcements []yyAnnouncement `json:"announcements"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是 JSON：%v", err)
	}
	if !out.OK || !out.Admin || len(out.Announcements) != 1 {
		t.Fatalf("读取结果不对：%+v", out)
	}
	if out.Brand != yyThemeBrandLine {
		t.Fatalf("署名口径不一致：%q", out.Brand)
	}

	empty := httptest.NewRequest("POST", "/api/mcort/announcements",
		bytes.NewBufferString(`{"title":"","content":"x"}`))
	empty.RemoteAddr = "127.0.0.1:5000"
	rw = httptest.NewRecorder()
	h(rw, empty)
	if !strings.Contains(rw.Body.String(), `"ok":false`) {
		t.Fatalf("空标题应被拒：%s", rw.Body.String())
	}
}

// ── 插件：只发现，不执行 ──

func TestYYThemePluginsOnlyRecord(t *testing.T) {
	yyThemeTestNoIdentity(t)
	dir := t.TempDir()
	yyThemeTestWrite(t, filepath.Join(dir, "plugins", "timer", "plugin.json"),
		`{"id":"timer","name":"计时器","version":"0.1.0","author":"社区","entry":"main.js","mcort_plugin_api":1}`)
	yyThemeTestWrite(t, filepath.Join(dir, "plugins", "timer", "main.js"), `console.log('不应该被执行')`)
	yyThemeTestWrite(t, filepath.Join(dir, "plugins", "noentry", "plugin.json"),
		`{"id":"noentry","name":"缺入口","version":"0.1.0","mcort_plugin_api":1}`)

	plugins := yyThemeScanPlugins(dir, map[string]bool{})
	if len(plugins) != 2 {
		t.Fatalf("应发现 2 个插件，得到 %d", len(plugins))
	}
	if plugins[0].ID != "noentry" || plugins[0].OK {
		t.Fatalf("缺入口的插件应判为不可用：%+v", plugins[0])
	}
	if !plugins[1].OK || plugins[1].Executed || plugins[1].Enabled {
		t.Fatalf("timer 应可用但未启用未执行：%+v", plugins[1])
	}

	mux := http.NewServeMux()
	yyThemeRoutes(mux, dir)

	// 没有 confirm：只提示，不落盘
	req := httptest.NewRequest("POST", "/api/mcort/plugins",
		bytes.NewBufferString(`{"op":"enable","id":"timer"}`))
	req.RemoteAddr = "127.0.0.1:5000"
	rw := httptest.NewRecorder()
	mux.ServeHTTP(rw, req)
	if !strings.Contains(rw.Body.String(), `"ok":false`) {
		t.Fatalf("缺二次确认应被拒：%s", rw.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/mcort/plugins",
		bytes.NewBufferString(`{"op":"enable","id":"timer","confirm":true}`))
	req.RemoteAddr = "127.0.0.1:5000"
	rw = httptest.NewRecorder()
	mux.ServeHTTP(rw, req)
	if !strings.Contains(rw.Body.String(), `"ok":true`) || !strings.Contains(rw.Body.String(), `"executed":false`) {
		t.Fatalf("启用只应记录状态：%s", rw.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "plugins-enabled.json")); err != nil {
		t.Fatalf("启用状态应落盘：%v", err)
	}
	plugins = yyThemeScanPlugins(dir, yyThemeReadEnabled(dir))
	for _, p := range plugins {
		if p.ID == "timer" && (!p.Enabled || p.Executed) {
			t.Fatalf("timer 状态不对：%+v", p)
		}
	}
}

// ── 首页注入 ──

func TestYYThemeInjectIndex(t *testing.T) {
	in := []byte(`<html><head><title>x</title></head><body><div id="app"></div></body></html>`)
	out := yyThemeInjectIndex(in)
	if !bytes.Contains(out, []byte("/css/mcort_theme.css")) || !bytes.Contains(out, []byte("/js/mcort_theme.js")) {
		t.Fatalf("没有注入资源：%s", out)
	}
	if bytes.Index(out, []byte("mcort_theme.css")) > bytes.Index(out, []byte("</head>")) {
		t.Fatalf("样式应在 </head> 之前")
	}
	if bytes.Index(out, []byte("mcort_theme.js")) > bytes.Index(out, []byte("</body>")) {
		t.Fatalf("脚本应在 </body> 之前")
	}
	again := yyThemeInjectIndex(out)
	if !bytes.Equal(out, again) {
		t.Fatalf("重复注入应幂等")
	}
}
