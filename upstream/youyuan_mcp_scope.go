package main

// MCP 服务器的「归属 + 连接方式」登记表。
//
// 为什么单独一张表：工作区里的 yyMCPServer 属于「某个账号的某次任务」，
// 而一台 MCP 到底是给全群用、还是某个人自己的机器，属于「服务器运维决定」，
// 两者生命周期不同。工作区重启、插件重发都不该丢掉「这台 MCP 是谁的」。
//
// 落盘：<dataDir>/mcp-servers.json（与 mcp-links.json 同目录，0600）。
// 登记过的 id 会接管该 id 的连接参数（含 stdio/sse/自定义请求头），
// 没登记的 id 继续走 yyMCPRequest 里的老 HTTP 逻辑，不会影响存量工作区。

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// yyMCPActorLocal 表示「工具箱本机的账号操作员」：这台机器本身就是他的，
	// 他在网页控制台里配置/调试 MCP，不需要再按群友身份拦。
	yyMCPActorLocal = "local"

	yyMCPScopePublic  = "public"
	yyMCPScopePrivate = "private"

	yyMCPAccessAll   = "all"
	yyMCPAccessAdmin = "admin"
	yyMCPAccessOwner = "owner_only"

	yyMCPTransportStdio = "stdio"
	yyMCPTransportHTTP  = "http"
	yyMCPTransportSSE   = "sse"
)

// yyMCPConn 是一条 MCP 服务器登记记录。
// scope/owner/access 回答「谁能用」，transport/url/command/args/headers 回答「怎么连」。
type yyMCPConn struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Scope     string            `json:"scope"`
	Owner     string            `json:"owner"`
	Access    string            `json:"access"`
	Transport string            `json:"transport"`
	URL       string            `json:"url"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Headers   map[string]string `json:"headers"`
	// Note 是使用者自己写的备注：公网直连 / 内网穿透 / 官方通道。
	// 不参与判定，但换机器时能一眼看出这条当初是怎么打通的。
	Note    string `json:"note"`
	Enabled bool   `json:"enabled"`

	session string
	version string
}

var yyMCPConns = struct {
	sync.RWMutex
	Items []yyMCPConn
	Path  string
}{}

func yyMCPConnLoad(path string) {
	yyMCPConns.Lock()
	defer yyMCPConns.Unlock()
	yyMCPConns.Path = path
	raw, e := os.ReadFile(path)
	if e != nil {
		return
	}
	var items []yyMCPConn
	if json.Unmarshal(raw, &items) != nil {
		return
	}
	for i := range items {
		_ = yyMCPConnNormalize(&items[i])
	}
	yyMCPConns.Items = items
}

// yyMCPConnNormalize 把缺省值补齐，并拒绝自相矛盾的组合。
func yyMCPConnNormalize(c *yyMCPConn) error {
	c.ID = strings.TrimSpace(c.ID)
	if c.ID == "" || len(c.ID) > 64 || strings.ContainsAny(c.ID, "/\\:'\"<>` ") {
		return fmt.Errorf("MCP 标识只能是 1–64 个字母数字（可含 - _.），不能带空格或引号")
	}
	if c.Name = strings.TrimSpace(c.Name); c.Name == "" {
		c.Name = c.ID
	}
	switch c.Scope {
	case "":
		c.Scope = yyMCPScopePublic
	case yyMCPScopePublic, yyMCPScopePrivate:
	default:
		return fmt.Errorf("scope 只支持 public / private")
	}
	c.Owner = strings.TrimSpace(c.Owner)
	if c.Scope == yyMCPScopePrivate {
		if c.Owner == "" {
			return fmt.Errorf("绑定给个人的 MCP 必须填归属人（群友 QQ 号）")
		}
		// 私有的含义就是「只有主人能用」，再给个 access=all 会自相矛盾。
		c.Access = yyMCPAccessOwner
	}
	switch c.Access {
	case "":
		c.Access = yyMCPAccessAll
	case yyMCPAccessAll, yyMCPAccessAdmin, yyMCPAccessOwner:
	default:
		return fmt.Errorf("access 只支持 all / admin / owner_only")
	}
	if c.Access == yyMCPAccessOwner && c.Owner == "" {
		return fmt.Errorf("access=owner_only 时必须填归属人")
	}
	switch c.Transport {
	case "":
		if strings.TrimSpace(c.Command) != "" {
			c.Transport = yyMCPTransportStdio
		} else {
			c.Transport = yyMCPTransportHTTP
		}
	case yyMCPTransportStdio, yyMCPTransportHTTP, yyMCPTransportSSE:
	default:
		return fmt.Errorf("transport 只支持 stdio / http / sse")
	}
	c.URL = strings.TrimSpace(c.URL)
	c.Command = strings.TrimSpace(c.Command)
	if c.Transport == yyMCPTransportStdio {
		if c.Command == "" {
			return fmt.Errorf("stdio 需要启动命令")
		}
		c.URL = ""
	} else {
		u, e := url.Parse(c.URL)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("http/sse 需要填 http(s) 地址，例如 https://你的机器/mcp")
		}
	}
	clean := map[string]string{}
	for k, v := range c.Headers {
		if k = strings.TrimSpace(k); k != "" {
			clean[k] = v
		}
	}
	if len(clean) == 0 {
		c.Headers = nil
	} else {
		c.Headers = clean
	}
	return nil
}

func yyMCPConnSaveLocked() error {
	if yyMCPConns.Path == "" {
		return fmt.Errorf("MCP 登记表未初始化")
	}
	raw, e := json.MarshalIndent(yyMCPConns.Items, "", "  ")
	if e != nil {
		return e
	}
	temp := yyMCPConns.Path + ".tmp"
	if e = os.WriteFile(temp, raw, 0600); e != nil {
		return e
	}
	return os.Rename(temp, yyMCPConns.Path)
}

func yyMCPConnSnapshot() []yyMCPConn {
	yyMCPConns.RLock()
	defer yyMCPConns.RUnlock()
	out := make([]yyMCPConn, len(yyMCPConns.Items))
	copy(out, yyMCPConns.Items)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func yyMCPConnGet(id string) (yyMCPConn, bool) {
	yyMCPConns.RLock()
	defer yyMCPConns.RUnlock()
	for _, c := range yyMCPConns.Items {
		if c.ID == id {
			return c, true
		}
	}
	return yyMCPConn{}, false
}

func yyMCPConnUpsert(c yyMCPConn) error {
	yyMCPConns.Lock()
	defer yyMCPConns.Unlock()
	index := -1
	for i := range yyMCPConns.Items {
		if yyMCPConns.Items[i].ID == c.ID {
			index = i
			break
		}
	}
	if index >= 0 {
		c.session = yyMCPConns.Items[index].session
		c.version = yyMCPConns.Items[index].version
		yyMCPConns.Items[index] = c
	} else {
		yyMCPConns.Items = append(yyMCPConns.Items, c)
	}
	return yyMCPConnSaveLocked()
}

func yyMCPConnDelete(id string) error {
	yyMCPConns.Lock()
	defer yyMCPConns.Unlock()
	next := make([]yyMCPConn, 0, len(yyMCPConns.Items))
	for _, c := range yyMCPConns.Items {
		if c.ID != id {
			next = append(next, c)
		}
	}
	yyMCPConns.Items = next
	return yyMCPConnSaveLocked()
}

func yyMCPConnRememberSession(id, session, version string) {
	yyMCPConns.Lock()
	defer yyMCPConns.Unlock()
	for i := range yyMCPConns.Items {
		if yyMCPConns.Items[i].ID != id {
			continue
		}
		if session != "" {
			yyMCPConns.Items[i].session = session
		}
		if version != "" {
			yyMCPConns.Items[i].version = version
		}
		return
	}
}

// yyMCPScopeGuard 是唯一的权限判定入口：scope × access × 角色。
// 返回 nil 表示这次调用可以放行。
func yyMCPScopeGuard(id, actor string, level int) error {
	if id == "" {
		return nil // 交给下面的「服务器不存在」报错，别在这里改语义
	}
	c, ok := yyMCPConnGet(id)
	if !ok {
		// 没登记的老配置：沿用历史行为（QQ 侧这三个工具本身只对管理员下发）。
		return nil
	}
	if !c.Enabled {
		return fmt.Errorf("MCP「%s」已停用", c.Name)
	}
	if actor == yyMCPActorLocal {
		return nil // 本机操作员：这台机器是他的，不受群友身份限制
	}
	if c.Scope == yyMCPScopePrivate {
		if c.Owner == "" || c.Owner != actor {
			return fmt.Errorf("「%s」是别人绑定的专属 MCP，你只能用公用 MCP", c.Name)
		}
		return nil
	}
	switch c.Access {
	case yyMCPAccessAll:
		return nil
	case yyMCPAccessAdmin:
		if level <= 0 {
			return fmt.Errorf("「%s」只对管理员开放", c.Name)
		}
		return nil
	case yyMCPAccessOwner:
		if c.Owner != "" && c.Owner == actor {
			return nil
		}
		return fmt.Errorf("「%s」只对绑定人开放", c.Name)
	}
	return nil
}

// yyMCPConnVisible 列出这个身份实际能调的服务器（顺序稳定，便于模型复现）。
func yyMCPConnVisible(actor string, level int) []yyMCPConn {
	out := []yyMCPConn{}
	for _, c := range yyMCPConnSnapshot() {
		if yyMCPScopeGuard(c.ID, actor, level) == nil && c.Enabled {
			out = append(out, c)
		}
	}
	return out
}

// yyMCPConnOpenToAll 判断是否至少有一台「全群可用」的 MCP。
// 只有成立时才把 MCP 工具下发给普通群友，否则维持「只有管理员看得到」。
func yyMCPConnOpenToAll() bool {
	for _, c := range yyMCPConnSnapshot() {
		if c.Enabled && c.Scope == yyMCPScopePublic && c.Access == yyMCPAccessAll {
			return true
		}
	}
	return false
}

// yyMCPConnPublicView 面板/模型侧看到的形状：密钥只回显「已设置」，不回声。
func yyMCPConnPublicView(c yyMCPConn) map[string]any {
	keys := []string{}
	for k := range c.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	row := map[string]any{
		"id": c.ID, "name": c.Name, "scope": c.Scope, "owner": c.Owner, "access": c.Access,
		"transport": c.Transport, "url": c.URL, "command": c.Command, "args": c.Args,
		"note": c.Note, "enabled": c.Enabled, "header_keys": keys, "has_headers": len(keys) > 0,
	}
	return row
}

// yyMCPConnRequestDirect 给「不在任何工作区里」的登记服务器用：
// 调用方只要知道 id，连接参数全部来自登记表。
// 工作区那条路自己管初始化握手，这条路要自己补一次（MCP 规定先 initialize）。
func yyMCPConnRequestDirect(ctx context.Context, id, method string, params any) ([]byte, error) {
	c, ok := yyMCPConnGet(id)
	if !ok || !c.Enabled {
		return nil, fmt.Errorf("MCP「%s」未启用或不存在", id)
	}
	if c.version == "" && method != "initialize" {
		if _, _, e := yyMCPConnRequest(ctx, &yyMCPServer{ID: id}, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "youyuan-agent", "version": "0.8.0"}}); e != nil {
			return nil, e
		}
		if c.Transport != yyMCPTransportStdio {
			if _, _, e := yyMCPConnRequest(ctx, &yyMCPServer{ID: id}, "notifications/initialized", map[string]any{}); e != nil {
				return nil, e
			}
		}
	}
	out, handled, e := yyMCPConnRequest(ctx, &yyMCPServer{ID: id}, method, params)
	if !handled {
		return nil, fmt.Errorf("MCP「%s」还没登记连接方式", id)
	}
	return out, e
}

// yyMCPConnRequest 是所有出站 MCP 请求的分流点。
// handled=false 表示这条 id 没登记，调用方继续走原来的 HTTP 实现。
func yyMCPConnRequest(ctx context.Context, server *yyMCPServer, method string, params any) ([]byte, bool, error) {
	c, ok := yyMCPConnGet(server.ID)
	if !ok {
		return nil, false, nil
	}
	if !c.Enabled {
		return nil, true, fmt.Errorf("MCP「%s」已停用", c.Name)
	}
	var out []byte
	var e error
	if c.Transport == yyMCPTransportStdio {
		out, e = yyMCPStdioRPC(ctx, c, method, params)
	} else {
		out, e = yyMCPHTTPRPC(ctx, &c, method, params)
	}
	return out, true, e
}

func yyMCPHTTPRPC(ctx context.Context, c *yyMCPConn, method string, params any) ([]byte, error) {
	body := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		body["params"] = params
	}
	notification := strings.HasPrefix(method, "notifications/")
	if !notification {
		body["id"] = time.Now().UnixNano()
	}
	payload, _ := json.Marshal(body)
	request, e := http.NewRequestWithContext(ctx, "POST", c.URL, bytes.NewReader(payload))
	if e != nil {
		return nil, e
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.Headers {
		request.Header.Set(k, v)
	}
	if c.session != "" {
		request.Header.Set("Mcp-Session-Id", c.session)
	}
	version := c.version
	if version == "" {
		version = "2025-06-18"
	}
	request.Header.Set("MCP-Protocol-Version", version)
	response, e := (&http.Client{Timeout: 90 * time.Second}).Do(request)
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("MCP HTTP %d（检查地址、token 和是否需要内网穿透）", response.StatusCode)
	}
	if notification {
		return nil, nil
	}
	var rpc map[string]json.RawMessage
	if strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		scan := bufio.NewScanner(response.Body)
		scan.Buffer(make([]byte, 4096), 2<<20)
		for scan.Scan() {
			line := strings.TrimSpace(scan.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			var message map[string]json.RawMessage
			if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &message) == nil && message["id"] != nil {
				rpc = message
				break
			}
		}
		if rpc == nil {
			return nil, fmt.Errorf("MCP SSE 未返回请求结果")
		}
	} else if e = json.NewDecoder(response.Body).Decode(&rpc); e != nil {
		return nil, e
	}
	if rpc["error"] != nil {
		return nil, fmt.Errorf("MCP %s", rpc["error"])
	}
	if method == "initialize" {
		var result struct {
			Version string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(rpc["result"], &result)
		c.session = response.Header.Get("Mcp-Session-Id")
		c.version = result.Version
		yyMCPConnRememberSession(c.ID, c.session, c.version)
	}
	return rpc["result"], nil
}

// yyMCPStdioRPC 用「起一个进程、跑完就收」的方式跑本机 stdio MCP：
// 每次调用重新 initialize 一次，代价是几百毫秒，换来的是不用管长驻进程的
// 生命周期和僵尸进程。stdio MCP 通常是本机小工具，这个代价可以接受。
func yyMCPStdioRPC(ctx context.Context, c yyMCPConn, method string, params any) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, c.Command, c.Args...)
	cmd.Stderr = nil
	stdin, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		return nil, e
	}
	if e = cmd.Start(); e != nil {
		return nil, fmt.Errorf("启动 %q 失败：%v", c.Command, e)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	reader := bufio.NewReaderSize(stdout, 1<<20)
	send := func(name string, value any, withID bool) (json.RawMessage, error) {
		body := map[string]any{"jsonrpc": "2.0", "method": name}
		if value != nil {
			body["params"] = value
		}
		if withID {
			body["id"] = 1
		}
		data, _ := json.Marshal(body)
		if _, e := stdin.Write(append(data, '\n')); e != nil {
			return nil, e
		}
		if !withID {
			return nil, nil
		}
		for line := 0; line < 500; line++ {
			buf, e := reader.ReadBytes('\n')
			if e != nil && len(bytes.TrimSpace(buf)) == 0 {
				return nil, fmt.Errorf("stdio MCP 没有返回结果：%v", e)
			}
			trimmed := bytes.TrimSpace(buf)
			if len(trimmed) == 0 {
				continue
			}
			var message map[string]json.RawMessage
			if json.Unmarshal(trimmed, &message) != nil || message["id"] == nil {
				continue
			}
			if message["error"] != nil {
				return nil, fmt.Errorf("MCP %s", message["error"])
			}
			return message["result"], nil
		}
		return nil, fmt.Errorf("stdio MCP 响应行数过多")
	}
	if method != "initialize" {
		if _, e := send("initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "youyuan-agent", "version": "0.8.0"}}, true); e != nil {
			return nil, e
		}
		_, _ = send("notifications/initialized", map[string]any{}, false)
	}
	result, e := send(method, params, true)
	if e != nil {
		return nil, e
	}
	if method == "initialize" {
		// 记下协议版本，下一次就不用为了「没有 session」再白起一个进程。
		var hello struct {
			Version string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(result, &hello)
		yyMCPConnRememberSession(c.ID, "", hello.Version)
	}
	return []byte(result), nil
}

// yyMCPConnTest 真正连一次：initialize + tools/list，把能证明「通了」的都回显。
func yyMCPConnTest(ctx context.Context, c yyMCPConn) map[string]any {
	start := time.Now()
	out := map[string]any{"id": c.ID, "transport": c.Transport, "scope": c.Scope, "access": c.Access}
	if e := yyMCPConnNormalize(&c); e != nil {
		out["ok"] = false
		out["error"] = e.Error()
		return out
	}
	switch c.Transport {
	case yyMCPTransportStdio:
		out["target"] = strings.TrimSpace(c.Command + " " + strings.Join(c.Args, " "))
	default:
		out["target"] = c.URL
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	call := func(method string, params any) ([]byte, error) {
		if c.Transport == yyMCPTransportStdio {
			return yyMCPStdioRPC(ctx, c, method, params)
		}
		return yyMCPHTTPRPC(ctx, &c, method, params)
	}
	init, e := call("initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "youyuan-toolbox", "version": yyAppVersion}})
	if e != nil {
		out["ok"] = false
		out["error"] = e.Error()
		out["elapsed_ms"] = time.Since(start).Milliseconds()
		return out
	}
	var hello struct {
		Version    string `json:"protocolVersion"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	_ = json.Unmarshal(init, &hello)
	out["protocol"] = hello.Version
	out["server"] = hello.ServerInfo.Name
	out["server_version"] = hello.ServerInfo.Version
	if latest, ok := yyMCPConnGet(c.ID); ok {
		// 有些服务器（尤其是穿透后的）第二次请求必须带 Mcp-Session-Id，
		// 这里把刚握到手的 session 接上，否则测试会假报 400。
		c.session, c.version = latest.session, latest.version
	}
	if c.Transport != yyMCPTransportStdio {
		if _, e := call("notifications/initialized", map[string]any{}); e != nil {
			out["ok"] = false
			out["error"] = e.Error()
			out["elapsed_ms"] = time.Since(start).Milliseconds()
			return out
		}
	}
	tools, e := call("tools/list", map[string]any{})
	if e != nil {
		// 握手成功、目录取不到也要如实说：至少地址和鉴权是对的。
		out["ok"] = true
		out["partial"] = true
		out["tools_error"] = e.Error()
	} else {
		var page struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		_ = json.Unmarshal(tools, &page)
		names := make([]string, 0, len(page.Tools))
		for _, t := range page.Tools {
			names = append(names, t.Name)
		}
		if len(names) > 20 {
			names = names[:20]
		}
		out["ok"] = true
		out["tool_count"] = len(page.Tools)
		out["tools"] = names
	}
	out["elapsed_ms"] = time.Since(start).Milliseconds()
	return out
}

// yyQQMCPServerArg 解析模型传来的 server；缺省回落到 group（设置页里的老通道）。
func yyQQMCPServerArg(args map[string]any) string {
	if s := strings.TrimSpace(aiArgStr(args, "server")); s != "" {
		return s
	}
	return "group"
}

// yyQQMCPScopeGuard 是 QQ 群里的准入判定。
// 没登记的 id（例如设置页里那台 "group"）继续按「只给管理员」处理，
// 登记过的则完全交给 scope × access。
func yyQQMCPScopeGuard(id, actor string, level int) error {
	if _, ok := yyMCPConnGet(id); !ok {
		if level <= 0 {
			return fmt.Errorf("外部 MCP「%s」还没在工具箱里登记归属，只有管理员能用", id)
		}
		return nil
	}
	return yyMCPScopeGuard(id, actor, level)
}

// yyQQMCPToolDefs 是 QQ 侧的三个 MCP 工具定义。
// 管理员和「有全群可用 MCP」两种情况共用这一份，避免两处 schema 走偏。
func yyQQMCPToolDefs() []ToolDef {
	str := func() map[string]any { return map[string]any{"type": "string"} }
	return []ToolDef{
		{Name: "mcp_list", Description: "列出外部 MCP 工具目录；填 server 就只看那台服务器", Parameters: yySchema(map[string]any{"server": str(), "cursor": str()})},
		{Name: "mcp_describe", Description: "查看指定外部 MCP 工具的参数文档", Parameters: yySchema(map[string]any{"server": str(), "tool": str()}, "tool")},
		{Name: "mcp_call", Description: "调用外部 MCP 工具；先 describe 查阅参数", Parameters: yySchema(map[string]any{"server": str(), "tool": str(), "arguments": map[string]any{"type": "object"}}, "tool")},
	}
}

// yyMCPScopeRoutes 是面板用的读写入口。只有本机控制台会调它，
// 所以和 mcp-links 一样不做额外的账号校验，而是靠「不登记进只读白名单」
// 保证外部只读连接永远碰不到它。
func yyMCPScopeRoutes(mux *http.ServeMux, root string) {
	yyMCPConnLoad(filepath.Join(root, "mcp-servers.json"))
	mux.HandleFunc("/api/youyuan/mcp/scope", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Op     string    `json:"op"`
			ID     string    `json:"id"`
			Server yyMCPConn `json:"server"`
			Owner  string    `json:"owner"`
			Access string    `json:"access"`
			Enabled *bool    `json:"enabled"`
		}
		if r.Method == "POST" {
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); e != nil {
				http.Error(w, "JSON 无效", 400)
				return
			}
		} else if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		switch p.Op {
		case "", "list":
		case "save":
			server := p.Server
			if server.ID == "" {
				server.ID = p.ID
			}
			if e := yyMCPConnNormalize(&server); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			if old, ok := yyMCPConnGet(server.ID); ok {
				// 留空 headers 表示「不改」，避免面板回显不出密钥却把已存的抹掉。
				if server.Headers == nil && len(old.Headers) > 0 {
					server.Headers = old.Headers
				}
			}
			if e := yyMCPConnUpsert(server); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		case "delete":
			if e := yyMCPConnDelete(p.ID); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		case "quick":
			// 面板上的两个开关：启用/停用、改归属人（不必重填整条配置）。
			c, ok := yyMCPConnGet(p.ID)
			if !ok {
				writeJSON(w, map[string]any{"ok": false, "error": "这条 MCP 不存在"})
				return
			}
			if p.Enabled != nil {
				c.Enabled = *p.Enabled
			}
			if p.Owner != "" {
				c.Owner = p.Owner
			}
			if p.Access != "" {
				c.Access = p.Access
			}
			if e := yyMCPConnNormalize(&c); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			if e := yyMCPConnUpsert(c); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		case "test":
			// 「先测再存」：表单带了地址/命令就测表单里的值，只给 id 才测已保存的那条。
			c := p.Server
			if c.ID == "" {
				c.ID = p.ID
			}
			fromForm := strings.TrimSpace(c.URL) != "" || strings.TrimSpace(c.Command) != ""
			if !fromForm {
				saved, ok := yyMCPConnGet(p.ID)
				if !ok {
					writeJSON(w, map[string]any{"ok": false, "error": "这条 MCP 还没保存，先填地址或启动命令"})
					return
				}
				c = saved
			} else if saved, ok := yyMCPConnGet(c.ID); ok && len(c.Headers) == 0 && len(saved.Headers) > 0 {
				// 面板不回显密钥，直接测时要把已保存的请求头带上，否则会被 401 误导。
				c.Headers = saved.Headers
			}
			if e := yyMCPConnNormalize(&c); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, yyMCPConnTest(r.Context(), c))
			return
		default:
			writeJSON(w, map[string]any{"ok": false, "error": "op 支持 list/save/delete/quick/test"})
			return
		}
		rows := []map[string]any{}
		for _, c := range yyMCPConnSnapshot() {
			rows = append(rows, yyMCPConnPublicView(c))
		}
		writeJSON(w, map[string]any{"ok": true, "servers": rows, "open_to_all": yyMCPConnOpenToAll()})
	})
}
