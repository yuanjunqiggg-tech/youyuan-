package main

import (
	"fmt"
	"sync"
)

// webControl 声明式页面上的一个控件。
type webControl struct {
	Type        string   `json:"type"`                  // text | button | input | table | switch
	Key         string   `json:"key,omitempty"`         // 数据键（emit 的数据按 key 更新控件）
	Label       string   `json:"label,omitempty"`       // 展示文字
	Action      string   `json:"action,omitempty"`      // button 触发的动作名
	Placeholder string   `json:"placeholder,omitempty"` // input 占位提示
	Columns     []string `json:"columns,omitempty"`     // table 列名
}

// webPage 一个插件注册的网页页面。
type webPage struct {
	Plugin   string       `json:"plugin"`
	Name     string       `json:"name"` // 插件内唯一
	Desc     string       `json:"desc"`
	Kind     string       `json:"kind"` // "declarative" | "html"
	Controls []webControl `json:"controls,omitempty"`
	Entry    string       `json:"entry,omitempty"` // html 模式入口（相对 www/，默认 index.html）
}

// webActionH 一个交互动作：请求到达时重跑插件源码并调用 Handler 函数。
type webActionH struct {
	plugin  string
	page    string
	action  string
	handler string // Lua 函数名
}

// webClient 一个已连接的 SSE 客户端。closed 先置位再 close(ch)，push 在锁内检查，防向已关闭通道发送。
type webClient struct {
	ch     chan string
	closed bool
}

var (
	webMu      sync.Mutex
	webPages   = map[string]map[string]*webPage{} // plugin → 页面名 → 页面
	webActions = map[string][]*webActionH{}       // plugin → 动作列表
	webClients = map[string]map[*webClient]bool{} // plugin → SSE 客户端
)

// registerWebPage 注册页面；同插件内页面名重复报错。页面名不允许含路径分隔符、
// URL 保留字符或控制字符（页面名会拼进 /web/p/<插件id>/<页面名> URL 与 SSE 事件名）。
func registerWebPage(p *webPage) error {
	if p.Plugin == "" || p.Name == "" {
		return fmt.Errorf("web.page: 插件与页面名不能为空")
	}
	for _, r := range p.Name {
		if r == '/' || r == '\\' || r == '#' || r == '?' || r == '%' || r < 0x20 || r == 0x7f {
			return fmt.Errorf("web.page: 页面名不允许包含路径分隔符、URL 保留字符或控制字符: %q", p.Name)
		}
	}
	webMu.Lock()
	defer webMu.Unlock()
	byName := webPages[p.Plugin]
	if byName == nil {
		byName = map[string]*webPage{}
		webPages[p.Plugin] = byName
	}
	if _, ok := byName[p.Name]; ok {
		return fmt.Errorf("web.page: 插件 %s 已注册页面 %q", p.Plugin, p.Name)
	}
	byName[p.Name] = p
	return nil
}

// registerWebAction 注册动作；同插件同页面同动作名重复注册报错。
func registerWebAction(plugin, page, action, handler string) error {
	if plugin == "" {
		return fmt.Errorf("web.action: 缺少插件上下文")
	}
	if page == "" || action == "" || handler == "" {
		return fmt.Errorf("web.action: 页面名/动作名/函数名不能为空")
	}
	webMu.Lock()
	defer webMu.Unlock()
	for _, h := range webActions[plugin] {
		if h.page == page && h.action == action {
			return fmt.Errorf("web.action: 插件 %s 页面 %q 动作 %q 已注册", plugin, page, action)
		}
	}
	webActions[plugin] = append(webActions[plugin], &webActionH{plugin: plugin, page: page, action: action, handler: handler})
	return nil
}

// getWebPage 取页面。
func getWebPage(plugin, page string) (*webPage, bool) {
	webMu.Lock()
	defer webMu.Unlock()
	p, ok := webPages[plugin][page]
	return p, ok
}

// lookupWebAction 找动作。
func lookupWebAction(plugin, page, action string) (*webActionH, bool) {
	webMu.Lock()
	defer webMu.Unlock()
	for _, h := range webActions[plugin] {
		if h.page == page && h.action == action {
			return h, true
		}
	}
	return nil, false
}

// listWebPages 返回全部页面（门户用）。
func listWebPages() []*webPage {
	webMu.Lock()
	defer webMu.Unlock()
	var out []*webPage
	for _, byName := range webPages {
		for _, p := range byName {
			out = append(out, p)
		}
	}
	return out
}

// subscribeWebEvents 注册一个 SSE 客户端，返回持有通道的客户端句柄。
// 检查"插件有页面"与"注册客户端"在同一临界区完成，与 RemovePluginWeb 无竞态；
// 插件无页面时返回已关闭通道的客户端（handler 会立即退出），不再产生僵尸流。
func subscribeWebEvents(plugin string) *webClient {
	webMu.Lock()
	defer webMu.Unlock()
	if len(webPages[plugin]) == 0 {
		ch := make(chan string)
		close(ch)
		return &webClient{ch: ch, closed: true}
	}
	c := &webClient{ch: make(chan string, 64)}
	if webClients[plugin] == nil {
		webClients[plugin] = map[*webClient]bool{}
	}
	webClients[plugin][c] = true
	return c
}

// unsubscribeWebEvents 移除 SSE 客户端。
func unsubscribeWebEvents(plugin string, c *webClient) {
	webMu.Lock()
	if set, ok := webClients[plugin]; ok {
		delete(set, c)
	}
	webMu.Unlock()
}

// pushWebEvent 把一条事件推给插件的全部 SSE 客户端（通道满则丢弃，绝不阻塞 Lua 执行）。
func pushWebEvent(plugin, page, dataJSON string) {
	msg := "event: " + page + "\ndata: " + dataJSON + "\n\n"
	webMu.Lock()
	for c := range webClients[plugin] {
		if c.closed {
			continue
		}
		select {
		case c.ch <- msg:
		default: // 客户端消费慢，丢弃本条
		}
	}
	webMu.Unlock()
}

// hasWebPages 判断插件是否注册过页面（SSE 端点用：未加载插件的连接直接 404，避免僵尸流）。
func hasWebPages(plugin string) bool {
	webMu.Lock()
	defer webMu.Unlock()
	return len(webPages[plugin]) > 0
}

// RemovePluginWeb 移除插件全部页面/动作并关闭其 SSE 连接（插件停止/重载/加载失败回滚时调用）。
func RemovePluginWeb(plugin string) {
	webMu.Lock()
	delete(webPages, plugin)
	delete(webActions, plugin)
	clients := webClients[plugin]
	delete(webClients, plugin)
	for c := range clients {
		c.closed = true
		close(c.ch)
	}
	webMu.Unlock()
}
