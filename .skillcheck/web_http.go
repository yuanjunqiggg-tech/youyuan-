package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// webActionSem 网页动作的独立并发上限：与游戏内触发词信号量分离，
// 防止局域网访客用慢请求占满触发词名额拖垮游戏内处理。
var webActionSem = make(chan struct{}, 8)

// handleWeb 插件网页总入口：/web/* 分发。
//
//	/web/                       门户首页
//	/web/p/<插件id>/<页面名>     页面本体
//	/web/api/<插件id>/<页面名>   交互动作
//	/web/events/<插件id>         SSE 实时推送
//	/web/static/<插件id>/<路径>  自定义 HTML 静态资源
func handleWeb(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/web/"), "/")
	switch {
	case r.URL.Path == "/web" || r.URL.Path == "/web/":
		handleWebPortal(w, r)
	case parts[0] == "p" && len(parts) == 3:
		handleWebPage(w, r, parts[1], parts[2])
	case parts[0] == "api" && len(parts) == 3:
		handleWebAPI(w, r, parts[1], parts[2])
	case parts[0] == "events" && len(parts) == 2:
		handleWebEvents(w, r, parts[1])
	case parts[0] == "static" && len(parts) >= 3:
		handleWebStatic(w, r, parts[1], strings.Join(parts[2:], "/"))
	default:
		http.NotFound(w, r)
	}
}

// ---- 密码门卫 ----

// getWebPassword 取插件配置里的 web_password；未配置返回 ""。
func getWebPassword(plugin string) string {
	cfg, err := pluginController.ReadConfig(plugin)
	if err != nil {
		return ""
	}
	if v, ok := cfg["web_password"].(string); ok {
		return v
	}
	return ""
}

// webPWHash 密码的 cookie 值（sha256 hex，避免明文存 cookie）。
func webPWHash(pw string) string {
	sum := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(sum[:])
}

// webCookieName 密码 cookie 名：插件 id 含中文等非 RFC token 字符时，
// SetCookie 会静默丢弃整条 cookie，因此 cookie 名用 id 的 sha256 前 8 字节派生。
func webCookieName(plugin string) string {
	sum := sha256.Sum256([]byte(plugin))
	return "web_auth_" + hex.EncodeToString(sum[:4])
}

const webLoginPage = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>需要密码</title>
<style>body{font-family:system-ui;background:#1b1f27;color:#e8eaf0;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
.box{background:#242a36;padding:32px;border-radius:12px;text-align:center}
input{padding:10px 14px;border-radius:8px;border:1px solid #3a4252;background:#1b1f27;color:#e8eaf0;margin:12px 0}
button{padding:10px 24px;border-radius:8px;border:0;background:#4f8cff;color:#fff;cursor:pointer}
.err{color:#ff7a7a;font-size:13px}</style></head><body>
<div class="box"><h3>该页面需要密码</h3>{{if .Err}}<div class="err">{{.Err}}</div>{{end}}
<form method="post"><input type="password" name="password" placeholder="输入访问密码" autofocus><br><button type="submit">进入</button></form></div>
</body></html>`

// webGate 页面级门卫：已通过 → true；否则处理登录表单/写表单，返回 false（响应已写）。
func webGate(w http.ResponseWriter, r *http.Request, plugin string) bool {
	pw := getWebPassword(plugin)
	if pw == "" {
		return true
	}
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if r.FormValue("password") == pw {
			http.SetCookie(w, &http.Cookie{Name: webCookieName(plugin), Value: webPWHash(pw), Path: "/web/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 30 * 24 * 3600})
			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return false
		}
		_ = webLoginTmpl.Execute(w, map[string]any{"Err": "密码错误"})
		return false
	}
	if c, err := r.Cookie(webCookieName(plugin)); err == nil && c.Value == webPWHash(pw) {
		return true
	}
	_ = webLoginTmpl.Execute(w, map[string]any{})
	return false
}

// webGatePassive API/SSE/静态资源门卫：无 cookie 直接 401（不渲染登录表单；
// 用户先通过页面门卫拿到 cookie，同源后续请求自动携带）。
func webGatePassive(w http.ResponseWriter, r *http.Request, plugin string) bool {
	pw := getWebPassword(plugin)
	if pw == "" {
		return true
	}
	if c, err := r.Cookie(webCookieName(plugin)); err == nil && c.Value == webPWHash(pw) {
		return true
	}
	http.Error(w, "需要密码", http.StatusUnauthorized)
	return false
}

// ---- 门户 ----

const webPortalTemplate = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>插件网页</title>
<style>body{font-family:system-ui;background:#1b1f27;color:#e8eaf0;margin:0;padding:24px}
h1{font-size:20px}.cards{display:flex;flex-wrap:wrap;gap:16px}
.card{background:#242a36;border-radius:12px;padding:16px 20px;min-width:220px}
.card a{color:#4f8cff;text-decoration:none;font-size:16px}
.card .desc{color:#9aa3b5;font-size:13px;margin-top:6px}
.card .lock{font-size:12px;color:#e0b64f;margin-top:4px}
.empty{color:#9aa3b5}</style></head><body>
<h1>插件网页</h1>
{{if .Pages}}<div class="cards">{{range .Pages}}<div class="card">
<a href="/web/p/{{.Plugin}}/{{.Name}}">{{.Name}}</a>
<div class="desc">{{.Desc}}</div>
{{if .Locked}}<div class="lock">🔒 需要密码</div>{{end}}
<div class="desc" style="margin-top:2px">{{.Plugin}}</div>
</div>{{end}}</div>
{{else}}<div class="empty">暂无已加载的插件页面</div>{{end}}
</body></html>`

// handleWebPortal 门户首页：列出全部已加载插件页面。
func handleWebPortal(w http.ResponseWriter, r *http.Request) {
	type card struct {
		Plugin, Name, Desc string
		Locked             bool
	}
	var cards []card
	for _, p := range listWebPages() {
		cards = append(cards, card{Plugin: p.Plugin, Name: p.Name, Desc: p.Desc, Locked: getWebPassword(p.Plugin) != ""})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webPortalTmpl.Execute(w, map[string]any{"Pages": cards})
}

// ---- 声明式页面 ----

const webDeclarativeTemplate = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<style>body{font-family:system-ui;background:#1b1f27;color:#e8eaf0;margin:0;padding:24px}
h1{font-size:20px}.desc{color:#9aa3b5;font-size:13px;margin-bottom:20px}
.ctrl{margin-bottom:18px}.label{font-size:13px;color:#9aa3b5;margin-bottom:6px}
.value{background:#242a36;border-radius:8px;padding:12px;min-height:18px}
input[type=text]{background:#242a36;border:1px solid #3a4252;border-radius:8px;padding:10px;color:#e8eaf0;width:220px}
button{background:#4f8cff;border:0;border-radius:8px;padding:10px 20px;color:#fff;cursor:pointer}
table{border-collapse:collapse;width:100%;max-width:480px}td,th{border:1px solid #3a4252;padding:8px 12px;text-align:left}
.switch{display:inline-flex;align-items:center;gap:8px;font-size:14px}
a.back{color:#4f8cff;text-decoration:none;font-size:13px}</style></head><body>
<a class="back" href="/web/">← 返回门户</a>
<h1>{{.Title}}</h1>
{{if .Desc}}<div class="desc">{{.Desc}}</div>{{end}}
{{range .Controls}}
{{if eq .Type "text"}}<div class="ctrl"><div class="label">{{.Label}}</div><div class="value" data-key="{{.Key}}">-</div></div>{{end}}
{{if eq .Type "button"}}<div class="ctrl"><button data-action="{{.Action}}" onclick="doAction(this)">{{.Label}}</button></div>{{end}}
{{if eq .Type "input"}}<div class="ctrl"><div class="label">{{.Label}}</div><input type="text" data-key="{{.Key}}" placeholder="{{.Placeholder}}"></div>{{end}}
{{if eq .Type "switch"}}<div class="ctrl"><label class="switch"><input type="checkbox" data-key="{{.Key}}">{{.Label}}</label></div>{{end}}
{{if eq .Type "table"}}<div class="ctrl"><div class="label">{{.Label}}</div><table data-key="{{.Key}}"><thead><tr>{{range .Columns}}<th>{{.}}</th>{{end}}</tr></thead><tbody></tbody></table></div>{{end}}
{{end}}
<script>
var PAGE = {{.PageJSON}};
function setVal(key, v) {
  var el = document.querySelector('[data-key="' + key + '"]');
  if (!el) return;
  if (el.tagName === 'TABLE') {
    var tb = el.querySelector('tbody'); tb.innerHTML = '';
    (v || []).forEach(function(row) {
      var tr = document.createElement('tr');
      (row || []).forEach(function(cell) {
        var td = document.createElement('td'); td.textContent = cell; tr.appendChild(td);
      });
      tb.appendChild(tr);
    });
  } else if (el.type === 'checkbox') el.checked = !!v;
  else if (el.tagName === 'INPUT') el.value = v;
  else el.textContent = v;
}
var es = new EventSource('/web/events/' + PAGE.plugin);
es.addEventListener(PAGE.page, function(e) {
  var d = JSON.parse(e.data);
  for (var k in d) setVal(k, d[k]);
});
function doAction(btn) {
  var args = {};
  document.querySelectorAll('[data-key]').forEach(function(el) {
    var k = el.getAttribute('data-key');
    if (el.type === 'checkbox') args[k] = el.checked;
    else if (el.value !== undefined) args[k] = el.value;
  });
  fetch('/web/api/' + PAGE.plugin + '/' + PAGE.page, {
    method: 'POST', headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({action: btn.getAttribute('data-action'), args: args})
  }).then(function(r) {
    if (!r.ok) return r.text().then(function(t) { throw new Error('HTTP ' + r.status + (t ? ': ' + t : '')); });
    return r.json();
  }).then(function(res) { if (res.ok === false) alert('操作失败: ' + (res.error || '')); })
    .catch(function(e) { alert('请求失败: ' + e.message); });
}
</script></body></html>`

// 三个模板预编译：避免每个请求都重新 Parse（声明式页面模板较大，属热点路径）。
var (
	webPortalTmpl      = template.Must(template.New("portal").Parse(webPortalTemplate))
	webLoginTmpl       = template.Must(template.New("login").Parse(webLoginPage))
	webDeclarativeTmpl = template.Must(template.New("page").Parse(webDeclarativeTemplate))
)

// handleWebPage 页面本体：声明式渲染或自定义 HTML 入口。
func handleWebPage(w http.ResponseWriter, r *http.Request, plugin, page string) {
	if !webGate(w, r, plugin) {
		return
	}
	pg, ok := getWebPage(plugin, page)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if pg.Kind == "html" {
		serveWebStatic(w, r, plugin, pg.Entry)
		return
	}
	pageJSON, _ := json.Marshal(map[string]string{"plugin": plugin, "page": page})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webDeclarativeTmpl.Execute(w, map[string]any{
		"Title":    pg.Name,
		"Desc":     pg.Desc,
		"Controls": pg.Controls,
		"PageJSON": template.JS(pageJSON),
	})
}

// ---- 交互动作 ----

// handleWebAPI 交互请求：POST JSON {action, args} 或 GET ?action=&k=v → callWebAction。
// handler 在 HTTP 服务 goroutine 里执行 Lua，受独立网页动作并发上限 webActionSem 约束。
func handleWebAPI(w http.ResponseWriter, r *http.Request, plugin, page string) {
	if !webGatePassive(w, r, plugin) {
		return
	}
	var req struct {
		Action string         `json:"action"`
		Args   map[string]any `json:"args"`
	}
	if r.Method == http.MethodPost {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
				return
			}
		}
	} else {
		req.Action = r.URL.Query().Get("action")
		req.Args = map[string]any{}
		for k, vs := range r.URL.Query() {
			if k != "action" && len(vs) > 0 {
				req.Args[k] = vs[0]
			}
		}
	}
	// 规格 §七：未注册动作与插件未加载 → 404；handler 执行错误才 500。
	if _, ok := getPluginSource(plugin); !ok {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]any{"ok": false, "error": "插件未加载"})
		return
	}
	if _, ok := lookupWebAction(plugin, page, req.Action); !ok {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]any{"ok": false, "error": "未注册动作"})
		return
	}
	webActionSem <- struct{}{}
	defer func() { <-webActionSem }()
	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()
	if bm == nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]any{"ok": false, "error": "机器人未连接"})
		return
	}
	result, err := callWebAction(bm, plugin, page, req.Action, req.Args)
	if err != nil {
		// 先设 Content-Type 再写状态码：WriteHeader 之后设置的响应头不会上送
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "data": result})
}

// ---- SSE ----

// handleWebEvents SSE 实时推送：向客户端流式转发该插件的 web.emit 事件。
// 插件停止/重载（RemovePluginWeb 关闭通道）或客户端断开时退出。
func handleWebEvents(w http.ResponseWriter, r *http.Request, plugin string) {
	if !webGatePassive(w, r, plugin) {
		return
	}
	if !hasWebPages(plugin) {
		http.NotFound(w, r)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "不支持流式响应", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	c := subscribeWebEvents(plugin)
	defer unsubscribeWebEvents(plugin, c)
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	_, _ = fmt.Fprint(w, ":ok\n\n")
	fl.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ":hb\n\n")
			fl.Flush()
		case msg, open := <-c.ch:
			if !open {
				return // 插件已停止/重载
			}
			_, _ = fmt.Fprint(w, msg)
			fl.Flush()
		}
	}
}

// ---- 自定义 HTML 静态托管 ----

// webWWWRoot 插件自定义网页根目录：<privateDir>/插件文件/<id>/www。
func webWWWRoot(plugin string) string {
	if root := yySDKWebRoot(plugin); root != "" {
		return root
	}
	return filepath.Join(pluginController.privateDir, "插件文件", plugin, "www")
}

// webStaticSafe 校验相对路径不逃出根目录，返回磁盘绝对路径。
func webStaticSafe(root, rel string) (string, bool) {
	cleaned := filepath.Clean("/" + rel)
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == ".." {
			return "", false
		}
	}
	full := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(cleaned, "/")))
	if !strings.HasPrefix(full, root+string(filepath.Separator)) && full != root {
		return "", false
	}
	return full, true
}

// handleWebStatic 自定义页面的静态资源（css/js/图片）。
func handleWebStatic(w http.ResponseWriter, r *http.Request, plugin, rel string) {
	if !webGatePassive(w, r, plugin) {
		return
	}
	serveWebStatic(w, r, plugin, rel)
}

// serveWebStatic 从 www 目录按相对路径服务一个文件（页面入口与静态资源共用）。
// 不用 http.ServeFile：其内置 containsDotDot 会对任何含 ".." 的 URL 路径直接回 400，
// 而我们的路径已由 webStaticSafe 约束在根目录内，此处只需对缺失文件回 404。
func serveWebStatic(w http.ResponseWriter, r *http.Request, plugin, rel string) {
	root := webWWWRoot(plugin)
	full, ok := webStaticSafe(root, rel)
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, filepath.Base(full), info.ModTime(), f)
}
