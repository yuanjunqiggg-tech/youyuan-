package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

var yyTunnel = struct {
	sync.Mutex
	Enabled                       bool `json:"enabled"`
	State, URL, Error, Path, Root string
	Logs                          []string
	Provider                      string
	SelectedProvider              string
	Cancel                        context.CancelFunc
	Done                          chan struct{}
}{State: "未启动"}
var yyTunnelURL = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

func yyTunnelBinary(root string) (string, error) {
	var path string
	if runtime.GOOS == "android" {
		raw, e := os.ReadFile(filepath.Join(root, "native-directory"))
		if e != nil {
			return "", fmt.Errorf("APK 原生程序目录未初始化：%w", e)
		}
		path = filepath.Join(strings.TrimSpace(string(raw)), "libyy_tunnel.so")
	} else {
		exe, e := os.Executable()
		if e != nil {
			return "", e
		}
		name := "cloudflared"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		path = filepath.Join(filepath.Dir(exe), name)
	}
	info, e := os.Stat(path)
	if e != nil || info.IsDir() {
		return "", fmt.Errorf("当前平台缺少内置 cloudflared 程序")
	}
	return path, nil
}
func yyTunnelPersist() error {
	raw, _ := json.Marshal(map[string]any{"enabled": yyTunnel.Enabled, "provider": yyTunnel.SelectedProvider})
	return os.WriteFile(yyTunnel.Path, raw, 0600)
}
func yyTunnelStart() error {
	yyTunnel.Lock()
	defer yyTunnel.Unlock()
	if yyTunnel.Cancel != nil {
		return nil
	}
	binary, e := yyTunnelBinary(yyTunnel.Root)
	if e != nil && yyTunnel.SelectedProvider == "cloudflare" {
		return e
	}
	yyTunnel.Enabled = true
	if e = yyTunnelPersist(); e != nil {
		yyTunnel.Enabled = false
		return e
	}
	ctx, cancel := context.WithCancel(context.Background())
	yyTunnel.Cancel = cancel
	yyTunnel.Done = make(chan struct{})
	yyTunnel.State = "分配公网地址中"
	yyTunnel.URL = ""
	yyTunnel.Error = ""
	go yyTunnelRun(ctx, binary, yyTunnel.Done)
	return nil
}
func yyTunnelStop() error {
	yyTunnel.Lock()
	yyTunnel.Enabled = false
	e := yyTunnelPersist()
	cancel, done := yyTunnel.Cancel, yyTunnel.Done
	yyTunnel.State = "正在关闭"
	yyTunnel.URL = ""
	yyTunnel.Unlock()
	if cancel != nil {
		cancel()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			return fmt.Errorf("隧道停止超时")
		}
	} else {
		yyTunnel.Lock()
		yyTunnel.State = "未启动"
		yyTunnel.Unlock()
	}
	return e
}
func yyTunnelState(state, detail string) {
	yyTunnel.Lock()
	yyTunnel.State = state
	yyTunnel.Error = detail
	if state == "重连等待" || state == "连接失败" {
		yyTunnel.URL = ""
	}
	yyTunnel.Unlock()
}
func yyTunnelPublicHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/mcp", "/mcp/":
		yyMCP(w, r)
	case "/health":
		writeJSON(w, map[string]any{"ready": true, "service": "youyuan-mcp-tunnel"})
	case "/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<!doctype html><html lang="zh"><meta name="viewport" content="width=device-width,initial-scale=1"><title>游园 MCP</title><body style="max-width:640px;margin:60px auto;padding:24px;font:16px/1.8 sans-serif"><h1>游园 MCP 公网入口</h1><p>在 MCP 客户端添加本网址下的 <b>/mcp/</b>，使用 Streamable HTTP。请求需携带 APK 中显示的连接密钥：<code>Authorization: Bearer &lt;密钥&gt;</code>。</p><p>APK 隧道关闭后此临时网址失效。此入口不提供账号设置页面。</p></body></html>`)
	default:
		http.NotFound(w, r)
	}
}
func yyTunnelRun(ctx context.Context, binary string, done chan struct{}) {
	defer close(done)
	defer func() {
		yyTunnel.Lock()
		yyTunnel.Cancel = nil
		yyTunnel.URL = ""
		yyTunnel.State = "未启动"
		yyTunnel.Unlock()
	}()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		yyTunnelState("连接失败", e.Error())
		return
	}
	server := &http.Server{Handler: http.HandlerFunc(yyTunnelPublicHandler), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 65536}
	defer server.Close()
	go server.Serve(listener)
	yyTunnel.Lock()
	selected := yyTunnel.SelectedProvider
	yyTunnel.Unlock()
	if selected == "pinggy" || binary == "" {
		yyTunnelPinggyLoop(ctx, listener.Addr().String())
		return
	}
	failures := 0
	for ctx.Err() == nil {
		yyTunnel.Lock()
		yyTunnel.URL = ""
		yyTunnel.State = "分配公网地址中"
		yyTunnel.Provider = "Cloudflare Quick Tunnel"
		yyTunnel.Unlock()
		cmd := exec.CommandContext(ctx, binary, "tunnel", "--no-autoupdate", "--protocol", "http2", "--ha-connections", "1", "--edge-ip-version", "4", "--metrics", "127.0.0.1:0", "--url", "http://"+listener.Addr().String())
		yyPrepareTunnelProcess(cmd)
		cmd.WaitDelay = 5 * time.Second
		reader, writer := io.Pipe()
		cmd.Stdout = writer
		cmd.Stderr = writer
		connected, rateLimited := false, false
		scanned := make(chan struct{})
		go func() {
			defer close(scanned)
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 8192), 128*1024)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "provisioning failed with status 429") {
					rateLimited = true
				}
				yyTunnel.Lock()
				if len(line) > 1500 {
					line = line[:1500]
				}
				yyTunnel.Logs = append(yyTunnel.Logs, line)
				if len(yyTunnel.Logs) > 64 {
					yyTunnel.Logs = yyTunnel.Logs[len(yyTunnel.Logs)-64:]
				}
				if url := yyTunnelURL.FindString(line); url != "" {
					yyTunnel.URL = url
					yyTunnel.State = "地址已分配，连接中"
				}
				if strings.Contains(line, "Registered tunnel connection") {
					connected = true
					yyTunnel.State = "已连接"
					yyTunnel.Error = ""
				}
				if strings.Contains(line, " ERR ") {
					if len(line) > 1000 {
						line = line[:1000]
					}
					yyTunnel.Error = line
				}
				yyTunnel.Unlock()
			}
			reader.Close()
		}()
		err := cmd.Run()
		writer.Close()
		<-scanned
		if ctx.Err() != nil {
			return
		}
		if selected != "cloudflare" && err != nil && !connected {
			if e := yyTunnelPinggy(ctx, listener.Addr().String()); e != nil && ctx.Err() == nil {
				err = fmt.Errorf("Cloudflare未连通；备用隧道：%w", e)
			}
			if ctx.Err() != nil {
				return
			}
		}
		if connected {
			failures = 0
		}
		delay := yyTunnelRetryDelay(failures, rateLimited)
		failures++
		detail := fmt.Sprintf("隧道进程已退出，%d秒后重连", int(delay/time.Second))
		if err != nil {
			detail = fmt.Sprintf("%s；%d秒后重连", err, int(delay/time.Second))
		}
		if rateLimited && strings.Contains(fmt.Sprint(err), "exit status") {
			detail = fmt.Sprintf("临时域名服务限制申请频率，%d秒后重试", int(delay/time.Second))
		}
		yyTunnelState("重连等待", detail)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}
func yyTunnelRetryDelay(failures int, rateLimited bool) time.Duration {
	if failures < 0 {
		failures = 0
	}
	if failures > 5 {
		failures = 5
	}
	delay := 10 * time.Second * time.Duration(1<<failures)
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	if rateLimited && delay < time.Minute {
		delay = time.Minute
	}
	return delay
}
func yyTunnelRoutes(mux *http.ServeMux, root string) {
	yyTunnel.Root = root
	yyTunnel.Path = filepath.Join(root, "youyuan-tunnel.json")
	raw, _ := os.ReadFile(yyTunnel.Path)
	var saved struct {
		Enabled  bool
		Provider string
	}
	json.Unmarshal(raw, &saved)
	yyTunnel.Enabled = saved.Enabled
	yyTunnel.SelectedProvider = saved.Provider
	if yyTunnel.SelectedProvider == "" {
		yyTunnel.SelectedProvider = "auto"
	}
	mux.HandleFunc("/api/youyuan/tunnel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var p struct {
				Action   string
				Provider string
			}
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&p); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			var e error
			switch p.Action {
			case "start":
				if p.Provider != "" {
					if p.Provider != "auto" && p.Provider != "cloudflare" && p.Provider != "pinggy" {
						e = fmt.Errorf("provider支持auto/cloudflare/pinggy")
						break
					}
					yyTunnel.Lock()
					if yyTunnel.Cancel != nil && p.Provider != yyTunnel.SelectedProvider {
						e = fmt.Errorf("请先关闭当前隧道再切换服务")
					} else {
						yyTunnel.SelectedProvider = p.Provider
					}
					yyTunnel.Unlock()
					if e != nil {
						break
					}
				}
				e = yyTunnelStart()
			case "stop":
				e = yyTunnelStop()
			default:
				e = fmt.Errorf("action 支持 start/stop")
			}
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		} else if r.Method != "GET" {
			http.Error(w, "GET/POST required", 405)
			return
		}
		binary, e := yyTunnelBinary(root)
		yyTunnel.Lock()
		provider := yyTunnel.Provider
		if provider == "" {
			provider = "Cloudflare Quick Tunnel"
		}
		state := map[string]any{"ok": true, "available": e == nil || yyTunnel.SelectedProvider != "cloudflare", "selected_provider": yyTunnel.SelectedProvider, "enabled": yyTunnel.Enabled, "state": yyTunnel.State, "url": yyTunnel.URL, "error": yyTunnel.Error, "logs": append([]string(nil), yyTunnel.Logs...), "native_program": filepath.Base(binary), "provider": provider, "temporary": true, "exposes": "MCP only"}
		if yyTunnel.URL != "" {
			state["mcp_url"] = yyTunnel.URL + "/mcp/"
		}
		yyTunnel.Unlock()
		yyRemoteMu.RLock()
		state["key"] = yyRemote.Key
		yyRemoteMu.RUnlock()
		writeJSON(w, state)
	})
	if yyTunnel.Enabled {
		go func() {
			if e := yyTunnelStart(); e != nil {
				yyTunnelState("连接失败", e.Error())
			}
		}()
	}
}
