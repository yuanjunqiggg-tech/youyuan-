package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type yyRemoteConfig struct {
	URL         string `json:"url"`
	DeviceToken string `json:"device_token"`
	Device      string `json:"device"`
	Enabled     bool   `json:"enabled"`
	Key         string `json:"key"`
}

var yyRemoteMu sync.RWMutex
var yyRemote yyRemoteConfig
var yyRemotePath string
var yyRemoteState = "未配置"
var yyRemoteCancel context.CancelFunc
var yyMux *http.ServeMux
var yyEventsMu sync.Mutex
var yyEvents []map[string]any
var yyEventSeq int64

func yyNewKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func yyReadRemote(dataDir string) {
	yyRemotePath = filepath.Join(dataDir, "youyuan-remote.json")
	yyRemoteMu.Lock()
	defer yyRemoteMu.Unlock()
	data, _ := os.ReadFile(yyRemotePath)
	_ = json.Unmarshal(data, &yyRemote)
	if yyRemote.Key == "" {
		yyRemote.Key = yyNewKey()
	}
	data, _ = json.MarshalIndent(yyRemote, "", "  ")
	_ = os.WriteFile(yyRemotePath, data, 0600)
}
func yyEvent(kind string, data any) {
	yyEventsMu.Lock()
	defer yyEventsMu.Unlock()
	yyEventSeq++
	yyEvents = append(yyEvents, map[string]any{"seq": yyEventSeq, "type": kind, "data": data})
	if len(yyEvents) > 2000 {
		yyEvents = yyEvents[len(yyEvents)-2000:]
	}
}
func yyRegisterAPI(mux *http.ServeMux, dataDir string) {
	yyMux = mux
	yyReadRemote(dataDir)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		platform := getPlatformName()
		backend := "内置原生 Go 内核"
		if platform == "android" {
			backend = "APK 内置原生内核"
		}
		accountID := "main"
		if yyAccounts.Worker {
			accountID = filepath.Base(dataDir)
		}
		result := map[string]any{"ready": true, "version": yyAppVersion, "platform": platform, "backend": backend, "tools": len(yyAPIRoutes) + 2, "account_id": accountID}
		if platform == "linux" {
			if raw, err := os.ReadFile(filepath.Join(dataDir, "linux-direct.json")); err == nil {
				var direct map[string]any
				if json.Unmarshal(raw, &direct) == nil {
					result["linux_direct_url"] = direct["url"]
				}
			}
		}
		writeJSON(w, result)
	})
	mux.HandleFunc("/api/youyuan/remote", func(w http.ResponseWriter, r *http.Request) {
		yyRemoteMu.Lock()
		defer yyRemoteMu.Unlock()
		if r.Method == "POST" {
			var patch map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			candidate := yyRemote
			for name, value := range patch {
				switch name {
				case "url":
					_ = json.Unmarshal(value, &candidate.URL)
				case "device_token":
					_ = json.Unmarshal(value, &candidate.DeviceToken)
				case "device":
					_ = json.Unmarshal(value, &candidate.Device)
				case "enabled":
					_ = json.Unmarshal(value, &candidate.Enabled)
				}
			}
			if candidate.Enabled {
				u, err := url.Parse(candidate.URL)
				if err != nil || u.Scheme != "wss" || u.Host == "" || candidate.DeviceToken == "" {
					writeJSON(w, map[string]any{"ok": false, "error": "请填写 wss:// 云端地址和设备密钥"})
					return
				}
			}
			yyRemote = candidate
			data, _ := json.MarshalIndent(yyRemote, "", "  ")
			if err := os.WriteFile(yyRemotePath, data, 0600); err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			if yyRemoteCancel != nil {
				yyRemoteCancel()
			}
		}
		accountID := "main"
		if yyAccounts.Worker {
			accountID = filepath.Base(dataDir)
		}
		writeJSON(w, map[string]any{"ok": true, "config": map[string]any{"url": yyRemote.URL, "device": yyRemote.Device, "enabled": yyRemote.Enabled, "has_device_token": yyRemote.DeviceToken != ""}, "state": yyRemoteState, "key": yyRemote.Key, "account_id": accountID, "local_mcp_url": "http://127.0.0.1:" + yyCorePort + "/mcp/", "public_urls": yyPublicURLs(yyRemote.URL), "relay_connected": yyRemote.Enabled && yyRemoteState == "已连接"})
	})
	mux.HandleFunc("/api/youyuan/catalog", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "routes": yyAPIRoutes})
	})
	mux.HandleFunc("/api/youyuan/events", func(w http.ResponseWriter, r *http.Request) {
		var cursor int64
		fmt.Sscan(r.URL.Query().Get("after"), &cursor)
		yyEventsMu.Lock()
		defer yyEventsMu.Unlock()
		events := []map[string]any{}
		for _, event := range yyEvents {
			if event["seq"].(int64) > cursor {
				events = append(events, event)
			}
		}
		writeJSON(w, map[string]any{"ok": true, "events": events, "cursor": yyEventSeq})
	})
	mux.HandleFunc("/mcp", yyMCP)
	mux.HandleFunc("/mcp/", yyMCP)
	yyTunnelRoutes(mux, dataDir)
	yyMCPLinksRoutes(mux, dataDir)
	yyRegisterQQ(mux, dataDir)
	yyObserverBridgeRoutes(mux, dataDir)
	yyRuntimeRoutes(mux, dataDir)
	yyCommandRoutes(mux, dataDir)
	yyHostFilesRoutes(mux, dataDir)
	yyExportCenterRoutes(mux, dataDir)
	yyPluginOutputRoutes(mux)
	yyBlockVideoRoutes(mux, dataDir)
	yyConverterRoutes(mux)
	yyAgentRoutes(mux, dataDir)
	yyNoticeRoutes(mux, dataDir)
	yySocialRoutes(mux, dataDir)
	yyAccountsRoutes(mux, dataDir)
	yyAccountBridgeRoutes(mux, dataDir)
	yyAccountPoolRoutes(mux, dataDir)
	yyObservationRoutes(mux, dataDir)
	yySenseRoutes(mux, dataDir)
	yyMCPTraceRoutes(mux)
	yyObservationSummaryRoutes(mux)
	yyAgentPreferencesRoutes(mux)
	yySkillLibraryRoutes(mux)
	yyBenchmarkRoutes(mux)
	yyAppearanceRoutes(mux, dataDir)
	yyDocumentationRoutes(mux)
	yyIdentityRoutes(mux, dataDir)
	yyMailRoutes(mux, dataDir)
	yyGameAuthRoutes(mux)
	yyStartAutoConnection()
}

// MCP 与 UI 调用同一个真实 HTTP 路由，不经内置 AI，也不伪造业务成功。
func yyInvoke(method, path string, query map[string]any, body any) (int, []byte, error) {
	return yyInvokeWithHeaders(method, path, query, body, nil)
}
func yyInvokeWithHeaders(method, path string, query map[string]any, body any, headers http.Header) (int, []byte, error) {
	if !strings.HasPrefix(path, "/api/") {
		return 0, nil, fmt.Errorf("仅允许工具箱 API 路径")
	}
	u, err := url.Parse(path)
	if err != nil {
		return 0, nil, err
	}
	q := u.Query()
	for name, value := range query {
		q.Set(name, fmt.Sprint(value))
	}
	u.RawQuery = q.Encode()
	data, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req := httptest.NewRequest(method, u.String(), bytes.NewReader(data))
	for k, values := range headers {
		req.Header[k] = values
	}
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	yyMux.ServeHTTP(recorder, req)
	return recorder.Code, recorder.Body.Bytes(), nil
}
func yyMCP(w http.ResponseWriter, r *http.Request) {
	yyRemoteMu.RLock()
	key := yyRemote.Key
	yyRemoteMu.RUnlock()
	supplied := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	readOnly := false
	if strings.HasPrefix(r.URL.Path, "/mcp/link/") {
		grant, ok := yyMCPLinkLookup(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/mcp/link/"), "/"))
		if !ok {
			http.Error(w, "专用连接已撤销或过期", 401)
			return
		}
		supplied = key
		readOnly = grant.Scope == "read"
	}
	if subtle.ConstantTimeCompare([]byte(supplied), []byte(key)) != 1 {
		http.Error(w, "MCP 密钥不正确", 401)
		return
	}
	if r.Method == "GET" {
		w.Header().Set("Allow", "POST, DELETE")
		w.WriteHeader(405)
		return
	}
	if r.Method == "DELETE" {
		w.WriteHeader(200)
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var packet struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		yyMCPError(w, nil, -32700, "无效 JSON 请求")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		yyMCPError(w, nil, -32700, "请求必须包含一个 JSON-RPC 消息")
		return
	}
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) || json.Unmarshal(raw, &packet) != nil {
		yyMCPError(w, nil, -32600, "请求必须为 JSON-RPC 对象")
		return
	}
	if packet.JSONRPC != "2.0" || !yyMCPValidID(packet.ID) {
		yyMCPError(w, nil, -32600, "无效 JSON-RPC 请求")
		return
	}
	// Streamable HTTP clients may send responses to server requests. This
	// stateless server has no outstanding requests, but must acknowledge them.
	if packet.Method == "" && len(packet.ID) > 0 && (len(packet.Result) > 0) != (len(packet.Error) > 0) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if packet.Method == "" || len(packet.Result) > 0 || len(packet.Error) > 0 {
		yyMCPError(w, packet.ID, -32600, "请求缺少方法或混入响应字段")
		return
	}
	if len(packet.ID) == 0 {
		w.WriteHeader(202)
		return
	}
	if version := r.Header.Get("MCP-Protocol-Version"); packet.Method != "initialize" && version != "" && !yyMCPVersionSupported(version) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		yyMCPError(w, packet.ID, -32600, "不支持此协议版本，请重新 initialize 协商")
		return
	}
	if len(packet.Params) > 0 && !bytes.Equal(bytes.TrimSpace(packet.Params), []byte("null")) && !bytes.HasPrefix(bytes.TrimSpace(packet.Params), []byte("{")) {
		yyMCPError(w, packet.ID, -32602, "MCP 参数必须为对象")
		return
	}
	if readOnly {
		if err := yyMCPReadCheck(packet.Method, packet.Params); err != nil {
			writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": packet.ID, "error": map[string]any{"code": -32003, "message": err.Error()}})
			return
		}
	}
	var result any
	switch packet.Method {
	case "initialize":
		var p map[string]any
		_ = json.Unmarshal(packet.Params, &p)
		version, _ := p["protocolVersion"].(string)
		if !yyMCPVersionSupported(version) {
			version = "2025-06-18"
		}
		result = map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}, "resources": map[string]any{"subscribe": false, "listChanged": false}}, "serverInfo": map[string]any{"name": "游园工具箱 MCP", "version": yyAppVersion}}
	case "ping":
		result = map[string]any{}
	case "resources/list":
		result = map[string]any{"resources": yyMCPResources()}
	case "resources/templates/list":
		result = map[string]any{"resourceTemplates": []any{map[string]any{"uriTemplate": "youyuan://account/{account}/project/{workspace}/{path}", "name": "账号项目文件", "description": "读取实际项目文件；account=main 或独立账号 ID", "mimeType": "text/plain"}}}
	case "resources/read":
		var p struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(packet.Params, &p); err != nil || p.URI == "" {
			yyMCPError(w, packet.ID, -32602, "缺少有效资源 URI")
			return
		}
		resource, e := yyMCPReadResource(p.URI)
		if e != nil {
			writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": packet.ID, "error": map[string]any{"code": -32002, "message": e.Error()}})
			return
		}
		result = map[string]any{"contents": []any{resource}}
	case "tools/list":
		tools := yyMCPTools()
		if readOnly {
			filtered := []any{}
			for _, item := range tools {
				name, _ := item.(map[string]any)["name"].(string)
				if yyMCPReadTool(name) {
					filtered = append(filtered, item)
				}
			}
			tools = filtered
		}
		result = map[string]any{"tools": tools}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(packet.Params, &p); err != nil || p.Name == "" {
			yyMCPError(w, packet.ID, -32602, "工具名称或参数无效")
			return
		}
		for _, field := range []string{"method", "account", "path"} {
			if value, present := p.Arguments[field]; present {
				if _, ok := value.(string); !ok {
					yyMCPError(w, packet.ID, -32602, field+" 必须为字符串")
					return
				}
			}
		}
		if value, present := p.Arguments["query"]; present {
			if _, ok := value.(map[string]any); !ok {
				yyMCPError(w, packet.ID, -32602, "query 必须为对象")
				return
			}
		}
		path := ""
		for _, route := range yyAPIRoutes {
			if route.Name == p.Name {
				path = route.Path
				break
			}
		}
		if p.Name == "api_catalog" {
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": mustJSON(yyAPIRoutes)}}, "isError": false}
			break
		}
		if p.Name == "api_call" {
			path, _ = p.Arguments["path"].(string)
		}
		if path == "" {
			yyMCPError(w, packet.ID, -32602, "工具不存在或未提供 API 路径")
			return
		}
		method, _ := p.Arguments["method"].(string)
		if method == "" {
			method = "GET"
		}
		if method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" {
			yyMCPError(w, packet.ID, -32602, "不支持的 API 方法")
			return
		}
		query, _ := p.Arguments["query"].(map[string]any)
		if account, _ := p.Arguments["account"].(string); account != "" && account != "main" {
			if !yyWorkspaceID(account) {
				yyMCPError(w, packet.ID, -32602, "账号 ID 无效")
				return
			}
			path = yyAccountsPrefix + account + path
		}
		// Download metadata is useful to an agent; a potentially huge private ZIP
		// must not be converted to garbled text or consume its entire context.
		if method == "GET" && strings.HasSuffix(strings.SplitN(path, "?", 2)[0], "/api/youyuan/exports/download") {
			path = strings.Replace(path, "/api/youyuan/exports/download", "/api/youyuan/exports/info", 1)
		}
		if method == "GET" && strings.HasSuffix(strings.SplitN(path, "?", 2)[0], "/api/youyuan/plugin-outputs/download") {
			path = strings.Replace(path, "/api/youyuan/plugin-outputs/download", "/api/youyuan/plugin-outputs/info", 1)
		}
		if query == nil {
			query = map[string]any{}
		}
		if method == "GET" && query["view"] == nil && (strings.HasSuffix(path, "/api/youyuan/sense/events") || strings.HasSuffix(path, "/api/youyuan/sense/snapshot") || strings.HasSuffix(path, "/api/youyuan/packets/capture")) {
			query["view"] = "summary"
		}
		traceID, started := yyNewKey()[:24], time.Now()
		if v := r.Header.Get("X-YouYuan-Trace-ID"); len(v) > 0 && len(v) <= 64 && strings.IndexFunc(v, func(c rune) bool {
			return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-')
		}) < 0 {
			traceID = v
		}
		headers := yyPoolForwardHeaders(r)
		headers.Set("X-YouYuan-Trace-ID", traceID)
		status, content, err := yyInvokeWithHeaders(method, path, query, p.Arguments["body"], headers)
		failed := status >= 400 || err != nil
		var payload map[string]any
		if json.Unmarshal(content, &payload) == nil {
			if ok, present := payload["ok"]; present && ok == false {
				failed = true
			}
			if payload["error"] != nil && payload["error"] != "" {
				failed = true
			}
		}
		message := string(content)
		if err != nil {
			message = err.Error()
		}
		meta := yyMCPTraceEnd(r, traceID, p.Name, method, path, status, payload, err, started)
		if meta["outcome"] == "server_rejected" {
			failed = true
		}
		result = map[string]any{"content": []any{map[string]any{"type": "text", "text": message}}, "isError": failed, "structuredContent": map[string]any{"mcp_meta": meta}}
	default:
		writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": packet.ID, "error": map[string]any{"code": -32601, "message": "未知方法"}})
		return
	}
	writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": packet.ID, "result": result})
}

func yyMCPError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}

func yyMCPValidID(id json.RawMessage) bool {
	if len(id) == 0 {
		return true // notification
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(id))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return false
	}
	switch value.(type) {
	case string, json.Number:
		return true
	default:
		return false
	}
}

func yyMCPVersionSupported(version string) bool {
	return version == "2025-03-26" || version == "2025-06-18"
}
func yyMCPTools() []any {
	tools := []any{}
	schema := map[string]any{"type": "object", "properties": map[string]any{"method": map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "DELETE"}, "default": "GET"}, "account": map[string]any{"type": "string", "description": "独立账号 ID；留空为主账号"}, "query": map[string]any{"type": "object"}, "body": map[string]any{"description": "API 原始 JSON 请求体"}}}
	for _, route := range yyAPIRoutes {
		description := "直接调用工具箱 " + route.Path + "，返回真实响应。"
		if contract := yyAPIContract(route.Path); contract != "" {
			description += " " + contract
		}
		toolSchema := schema
		if route.Name == "api_youyuan_docs" {
			description = "按需读取已部署开发文档或技能。无参数返回资源目录；文档名放在 query.language，例如 {query:{language:'mcp-connection'}}；Lua/Python/C++ 用 lua-runtime/python-runtime/cpp-runtime。不要把 URI 或语言写在顶层。query.method/section 可读单个接口或章节。"
			toolSchema = map[string]any{"type": "object", "properties": map[string]any{"account": map[string]any{"type": "string"}, "query": map[string]any{"type": "object", "properties": map[string]any{"language": map[string]any{"type": "string", "description": "资源目录 URI 中 docs/ 后的技能 ID，例如 mcp-connection、lua-runtime、python-runtime、cpp-runtime、command-chain-api、packet-reference"}, "method": map[string]any{"type": "string", "description": "命名空间.方法，如 game.sendCommandFast"}, "section": map[string]any{"type": "string", "description": "该技能目录中的章节文件"}, "catalog": map[string]any{"type": "string", "enum": []string{"1"}}, "full": map[string]any{"type": "string", "enum": []string{"1"}}}}}}
		}
		if route.Name == "api_youyuan_exports_download" {
			description = "浏览器下载完整插件ZIP；MCP调用只返回本人有权访问的文件名、大小、SHA256和下载路径，downloaded=false，不把ZIP二进制塞入上下文。"
		}
		if route.Name == "api_youyuan_plugin_outputs_download" {
			description = "插件运行时生成文件的下载入口，保留原格式；MCP调用只返回文件元信息和认证下载路径，downloaded=false。不是完整插件源码备份。"
		}
		tools = append(tools, map[string]any{"name": route.Name, "description": description, "inputSchema": toolSchema})
	}
	tools = append(tools, map[string]any{"name": "api_catalog", "description": "读取完整原生 API 清单", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}})
	tools = append(tools, map[string]any{"name": "api_call", "description": "调用任意已实现 API；用于后续新增插件 API", "inputSchema": map[string]any{"type": "object", "required": []string{"path"}, "properties": map[string]any{"path": map[string]any{"type": "string"}, "method": map[string]any{"type": "string"}, "account": map[string]any{"type": "string", "description": "独立账号 ID；留空为主账号"}, "query": map[string]any{"type": "object"}, "body": map[string]any{}}}})
	return tools
}

func yyRemoteDialHeaders(deviceToken, device string) http.Header {
	headers := http.Header{
		"Authorization": []string{"Bearer " + deviceToken},
		"User-Agent":    []string{"YouYuan-Toolbox/" + yyAppVersion},
	}
	// 设备标识头：中继按此把连接放进对应设备桶（如 linux-server）。
	// 不带该头时中继默认 primary-device，多设备部署会桶错位（MCP/邮件中继全 503）。
	// URL 查询参数 ?device= 只是兼容兜底，头才是正路。
	if device != "" {
		headers.Set("X-YouYuan-Device", device)
	}
	return headers
}

func yyStartRemote(port, dataDir string, mux *http.ServeMux) {
	go func() {
		delay := time.Second
		for {
			yyRemoteMu.Lock()
			cfg := yyRemote
			ctx, cancel := context.WithCancel(context.Background())
			yyRemoteCancel = cancel
			yyRemoteMu.Unlock()
			if !cfg.Enabled {
				cancel()
				time.Sleep(time.Second)
				continue
			}
			yyRemoteMu.Lock()
			yyRemoteState = "连接中"
			yyRemoteMu.Unlock()
			options := &websocket.DialOptions{HTTPHeader: yyRemoteDialHeaders(cfg.DeviceToken, cfg.Device)}
			conn, _, err := websocket.Dial(ctx, cfg.URL, options)
			if err == nil {
				delay = time.Second
				conn.SetReadLimit(48 * 1024 * 1024)
				yyRemoteMu.Lock()
				yyRemoteState = "已连接"
				yyRemoteMu.Unlock()
				client := &http.Client{Timeout: 140 * time.Second, Transport: &http.Transport{Proxy: nil, MaxIdleConnsPerHost: 32}}
				var lastSeen atomic.Int64
				lastSeen.Store(time.Now().UnixNano())
				go func() {
					ticker := time.NewTicker(20 * time.Second)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							if time.Since(time.Unix(0, lastSeen.Load())) > 55*time.Second {
								conn.CloseNow()
								return
							}
							pingCtx, done := context.WithTimeout(ctx, 10*time.Second)
							e := conn.Write(pingCtx, websocket.MessageText, []byte(`{"kind":"ping"}`))
							done()
							if e != nil {
								conn.CloseNow()
								return
							}
						}
					}
				}()
				err = yyReadRemoteRequests(ctx, conn, &lastSeen, func(p yyRelayPacket) {
					parsedPath, parseErr := url.ParseRequestURI(p.Path)
					if parseErr != nil || !(strings.HasPrefix(parsedPath.Path, "/api/") || strings.HasPrefix(parsedPath.Path, "/mcp") || parsedPath.Path == "/health") {
						return
					}
					content, e := base64.StdEncoding.DecodeString(p.Body)
					if e != nil {
						return
					}
					req, e := http.NewRequestWithContext(ctx, p.Method, "http://127.0.0.1:"+port+p.Path, bytes.NewReader(content))
					if e != nil {
						return
					}
					for name, value := range p.Headers {
						if strings.EqualFold(name, "host") {
							continue
						}
						req.Header.Set(name, value)
					}
					req.Header.Set("Authorization", "Bearer "+cfg.Key)
					response, e := client.Do(req)
					status := 502
					output := []byte(`{"ok":false,"error":"APK 请求失败"}`)
					headers := map[string]string{"Content-Type": "application/json"}
					if e == nil {
						status = response.StatusCode
						output, e = io.ReadAll(response.Body)
						response.Body.Close()
						headers["Content-Type"] = response.Header.Get("Content-Type")
						for _, name := range []string{"Content-Disposition", "Cache-Control", "X-Content-SHA256", "X-Content-Type-Options"} {
							if value := response.Header.Get(name); value != "" {
								headers[name] = value
							}
						}
					}
					if e != nil {
						output, _ = json.Marshal(map[string]any{"ok": false, "error": e.Error()})
					}
					result, _ := json.Marshal(map[string]any{"id": p.ID, "status": status, "headers": headers, "body": base64.StdEncoding.EncodeToString(output)})
					_ = conn.Write(ctx, websocket.MessageText, result)
				}, func(secret string) {
					yyPoolRelayIdentity.Lock()
					yyPoolRelayIdentity.Secret = secret
					yyPoolRelayIdentity.Unlock()
				})
				conn.CloseNow()
				yyPoolRelayIdentity.Lock()
				yyPoolRelayIdentity.Secret = ""
				yyPoolRelayIdentity.Unlock()
				client.CloseIdleConnections()
			}
			cancel()
			yyRemoteMu.Lock()
			yyRemoteState = "重连等待"
			if err != nil {
				yyRemoteState += "：" + err.Error()
			}
			yyRemoteMu.Unlock()
			time.Sleep(delay)
			if delay < 30*time.Second {
				delay *= 2
			}
		}
	}()
}
