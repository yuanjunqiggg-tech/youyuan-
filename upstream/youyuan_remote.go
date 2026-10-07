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
		writeJSON(w, map[string]any{"ok": true, "config": map[string]any{"url": yyRemote.URL, "enabled": yyRemote.Enabled, "has_device_token": yyRemote.DeviceToken != ""}, "state": yyRemoteState, "key": yyRemote.Key, "account_id": accountID, "local_mcp_url": "http://127.0.0.1:" + yyCorePort + "/mcp/", "public_urls": yyPublicURLs(yyRemote.URL), "relay_connected": yyRemote.Enabled && yyRemoteState == "已连接"})
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
	yyRuntimeRoutes(mux, dataDir)
	yyCommandRoutes(mux, dataDir)
	yyHostFilesRoutes(mux, dataDir)
	yyBlockVideoRoutes(mux, dataDir)
	yyConverterRoutes(mux)
	yyAgentRoutes(mux, dataDir)
	yyNoticeRoutes(mux, dataDir)
	yySocialRoutes(mux, dataDir)
	yyAccountsRoutes(mux, dataDir)
	yyAccountBridgeRoutes(mux, dataDir)
	yyObservationRoutes(mux, dataDir)
	yySenseRoutes(mux, dataDir)
	yyAgentPreferencesRoutes(mux)
	yySkillLibraryRoutes(mux)
	yyBenchmarkRoutes(mux)
	yyAppearanceRoutes(mux, dataDir)
	yyDocumentationRoutes(mux)
	yyIdentityRoutes(mux, dataDir)
	yyGameAuthRoutes(mux)
	yyStartAutoConnection()
}

// MCP 与 UI 调用同一个真实 HTTP 路由，不经内置 AI，也不伪造业务成功。
func yyInvoke(method, path string, query map[string]any, body any) (int, []byte, error) {
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
		w.WriteHeader(405)
		return
	}
	if r.Method == "DELETE" {
		w.WriteHeader(200)
		return
	}
	var packet struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&packet); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if len(packet.ID) == 0 {
		w.WriteHeader(202)
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
		if version == "" {
			version = "2025-03-26"
		}
		result = map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}, "resources": map[string]any{"subscribe": false, "listChanged": false}}, "serverInfo": map[string]any{"name": "游园 APK 内置 MCP", "version": yyAppVersion}}
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
		_ = json.Unmarshal(packet.Params, &p)
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
		_ = json.Unmarshal(packet.Params, &p)
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
		method, _ := p.Arguments["method"].(string)
		if method == "" {
			method = "GET"
		}
		query, _ := p.Arguments["query"].(map[string]any)
		if account, _ := p.Arguments["account"].(string); account != "" && account != "main" {
			if !yyWorkspaceID(account) {
				writeJSON(w, map[string]any{"error": "账号 ID 无效"})
				return
			}
			path = yyAccountsPrefix + account + path
		}
		status, content, err := yyInvoke(method, path, query, p.Arguments["body"])
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
		result = map[string]any{"content": []any{map[string]any{"type": "text", "text": message}}, "isError": failed}
	default:
		writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": packet.ID, "error": map[string]any{"code": -32601, "message": "未知方法"}})
		return
	}
	writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": packet.ID, "result": result})
}
func yyMCPTools() []any {
	tools := []any{}
	schema := map[string]any{"type": "object", "properties": map[string]any{"method": map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "DELETE"}, "default": "GET"}, "account": map[string]any{"type": "string", "description": "独立账号 ID；留空为主账号"}, "query": map[string]any{"type": "object"}, "body": map[string]any{"description": "API 原始 JSON 请求体"}}}
	for _, route := range yyAPIRoutes {
		tools = append(tools, map[string]any{"name": route.Name, "description": "直接调用工具箱 " + route.Path + "，返回真实响应。", "inputSchema": schema})
	}
	tools = append(tools, map[string]any{"name": "api_catalog", "description": "读取完整原生 API 清单", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}})
	tools = append(tools, map[string]any{"name": "api_call", "description": "调用任意已实现 API；用于后续新增插件 API", "inputSchema": map[string]any{"type": "object", "required": []string{"path"}, "properties": map[string]any{"path": map[string]any{"type": "string"}, "method": map[string]any{"type": "string"}, "account": map[string]any{"type": "string", "description": "独立账号 ID；留空为主账号"}, "query": map[string]any{"type": "object"}, "body": map[string]any{}}}})
	return tools
}

type yyRelayPacket struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
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
			options := &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + cfg.DeviceToken}}}
			conn, _, err := websocket.Dial(ctx, cfg.URL, options)
			if err == nil {
				delay = time.Second
				conn.SetReadLimit(48 * 1024 * 1024)
				yyRemoteMu.Lock()
				yyRemoteState = "已连接"
				yyRemoteMu.Unlock()
				client := &http.Client{Timeout: 140 * time.Second, Transport: &http.Transport{Proxy: nil, MaxIdleConnsPerHost: 32}}
				requests := make(chan struct{}, 32)
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
			relayReadLoop:
				for {
					_, data, readErr := conn.Read(ctx)
					if readErr != nil {
						err = readErr
						break
					}
					lastSeen.Store(time.Now().UnixNano())
					var control struct {
						Kind string `json:"kind"`
					}
					if json.Unmarshal(data, &control) == nil && control.Kind == "pong" {
						continue
					}
					var packet yyRelayPacket
					if json.Unmarshal(data, &packet) != nil {
						continue
					}
					select {
					case requests <- struct{}{}:
					case <-ctx.Done():
						break relayReadLoop
					}
					go func(p yyRelayPacket) {
						defer func() { <-requests }()
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
						}
						if e != nil {
							output, _ = json.Marshal(map[string]any{"ok": false, "error": e.Error()})
						}
						result, _ := json.Marshal(map[string]any{"id": p.ID, "status": status, "headers": headers, "body": base64.StdEncoding.EncodeToString(output)})
						_ = conn.Write(ctx, websocket.MessageText, result)
					}(packet)
				}
				conn.CloseNow()
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
