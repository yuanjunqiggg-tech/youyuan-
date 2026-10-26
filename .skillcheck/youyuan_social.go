package main

import (
	"bot-apk/config"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// One token has one game session. This queue uses the existing native connection
// lifecycle, never fictitious accounts or synthetic player counts.
type yyVisit struct {
	Server   string `json:"server"`
	Password string `json:"password,omitempty"`
	Seconds  int    `json:"seconds"`
}
type yySocialConfig struct {
	LikePath    string `json:"like_path"`
	MessagePath string `json:"message_path"`
}
type yySocialJob struct {
	Kind    string           `json:"kind"`
	State   string           `json:"state"`
	Index   int              `json:"index"`
	Total   int              `json:"total"`
	Results []map[string]any `json:"results"`
	cancel  context.CancelFunc
}

var yySocial = struct {
	sync.Mutex
	Job    yySocialJob
	Config yySocialConfig
	Path   string
}{Job: yySocialJob{State: "待开始", Results: []map[string]any{}}}

func yySocialResult(result map[string]any) {
	yySocial.Lock()
	defer yySocial.Unlock()
	yySocial.Job.Results = append(yySocial.Job.Results, result)
	if len(yySocial.Job.Results) > 2000 {
		yySocial.Job.Results = yySocial.Job.Results[1:]
	}
	yySocial.Job.Index++
}
func yySocialWait(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
func yyProviderCall(ctx context.Context, method, path string, params map[string]any) (any, error) {
	cfg := config.ReloadConfig()
	if cfg.Token == "" || cfg.AuthURL == "" {
		return nil, fmt.Errorf("请先在设置保存账号和验证服务器")
	}
	if !strings.HasPrefix(path, "/api/") || strings.ContainsAny(path, "?#") || strings.Contains(path, "..") {
		return nil, fmt.Errorf("验证服务端点必须是 /api/ 开头的相对路径")
	}
	params["login_token"] = cfg.Token
	origin, e := yyProviderURL(cfg.AuthURL)
	if e != nil {
		return nil, e
	}
	target := origin + path
	var body io.Reader
	if method == "GET" {
		query := url.Values{}
		for k, v := range params {
			query.Set(k, fmt.Sprint(v))
		}
		target += "?" + query.Encode()
	} else {
		data, _ := json.Marshal(params)
		body = bytes.NewReader(data)
	}
	req, e := http.NewRequestWithContext(ctx, method, target, body)
	if e != nil {
		return nil, e
	}
	if method != "GET" {
		req.Header.Set("Content-Type", "application/json")
	}
	// Authentication errors and 429 are returned once. Mutations are never retried automatically.
	response, e := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if e != nil {
		return nil, fmt.Errorf("验证服务请求失败")
	}
	defer response.Body.Close()
	data, e := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if e != nil {
		return nil, e
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("验证服务 HTTP %d", response.StatusCode)
	}
	var result any
	if e = json.Unmarshal(data, &result); e != nil {
		return nil, fmt.Errorf("验证服务返回非 JSON 数据，该服务不支持此接口")
	}
	return result, nil
}
func yyQueueConnect(ctx context.Context, v yyVisit) error {
	cfg := config.ReloadConfig()
	body, _ := json.Marshal(map[string]any{"token": cfg.Token, "auth": cfg.AuthURL, "server": v.Server, "password": v.Password})
	recorder := httptest.NewRecorder()
	handleBotConnect(recorder, httptest.NewRequest("POST", "/api/bot/connect", bytes.NewReader(body)))
	var result map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &result)
	if result["ok"] != true {
		return fmt.Errorf("连接未启动: %v", result["error"])
	}
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("90 秒内未完成进服与权限确认")
		case <-tick.C:
			bm := getBotManagerLocked()
			if bm != nil && bm.IsJoined() {
				return nil
			}
		}
	}
}
func yyRunVisits(ctx context.Context, visits []yyVisit) {
	defer yyFinishSocial()
	for _, v := range visits {
		if ctx.Err() != nil {
			return
		}
		e := yyQueueConnect(ctx, v)
		result := map[string]any{"server": v.Server, "joined": e == nil}
		if e != nil {
			result["error"] = e.Error()
		}
		yySocialResult(result)
		if e == nil {
			seconds := v.Seconds
			if seconds < 1 {
				seconds = 60
			}
			yySocialWait(ctx, time.Duration(seconds)*time.Second)
		}
		yyPauseAutoConnection()
		if bm := getBotManagerLocked(); bm != nil {
			bm.Disconnect()
		}
		// Let any cancelled dial finish before another token session is attempted.
		for {
			yyAutoConnection.Lock()
			busy := yyAutoConnection.busy
			yyAutoConnection.Unlock()
			if !busy {
				break
			}
			if !yySocialWait(ctx, time.Second) {
				return
			}
		}
	}
}
func yyFinishSocial() {
	yySocial.Lock()
	defer yySocial.Unlock()
	if yySocial.Job.State == "运行中" {
		yySocial.Job.State = "已完成"
	}
	yySocial.Job.cancel = nil
}
func yyRunSocialBatch(ctx context.Context, kind string, targets, messages []string, interval time.Duration) {
	defer yyFinishSocial()
	yySocial.Lock()
	c := yySocial.Config
	yySocial.Unlock()
	for _, target := range targets {
		for _, message := range messages {
			if ctx.Err() != nil {
				return
			}
			result := map[string]any{"target": target, "message": message}
			if kind == "game_message" {
				bm := yyAccountBot(target)
				if target == "main" {
					bm = getBotManagerLocked()
				}
				if bm == nil || !bm.IsConnected() {
					result["error"] = "账号未连接"
				} else {
					_, e := bm.SendConsole(message)
					if e != nil {
						result["error"] = e.Error()
					} else {
						result["submitted"] = true
						result["note"] = "已提交聊天包，游戏协议不提供送达回执"
					}
				}
			} else {
				path := c.LikePath
				params := map[string]any{"uid": target}
				if kind == "private_message" {
					path = c.MessagePath
					params["message"] = message
				}
				response, e := yyProviderCall(ctx, "POST", path, params)
				if e != nil {
					result["error"] = e.Error()
				} else {
					result["provider_response"] = response
				}
			}
			yySocialResult(result)
			if !yySocialWait(ctx, interval) {
				return
			}
		}
	}
}
func yySocialRoutes(mux *http.ServeMux, dataDir string) {
	yySocial.Path = filepath.Join(dataDir, "social-adapter.json")
	if data, e := os.ReadFile(yySocial.Path); e == nil {
		_ = json.Unmarshal(data, &yySocial.Config)
	}
	mux.HandleFunc("/api/youyuan/social/search", func(w http.ResponseWriter, r *http.Request) {
		params := map[string]any{"offset": r.URL.Query().Get("offset"), "sort_type": r.URL.Query().Get("sort_type"), "order_type": 0}
		path := "/api/phoenix/server/list"
		if keyword := r.URL.Query().Get("keyword"); keyword != "" {
			path = "/api/phoenix/server/search"
			params["keyword"] = keyword
		}
		result, e := yyProviderCall(r.Context(), "GET", path, params)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "data": result})
	})
	mux.HandleFunc("/api/youyuan/social", func(w http.ResponseWriter, r *http.Request) {
		yySocial.Lock()
		defer yySocial.Unlock()
		if r.Method == "GET" {
			writeJSON(w, map[string]any{"ok": true, "job": yySocial.Job, "config": yySocial.Config})
			return
		}
		var p struct {
			Action     string         `json:"action"`
			Kind       string         `json:"kind"`
			Visits     []yyVisit      `json:"visits"`
			Targets    []string       `json:"targets"`
			Messages   []string       `json:"messages"`
			IntervalMS int            `json:"interval_ms"`
			Config     yySocialConfig `json:"config"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if p.Action == "config" {
			data, _ := json.Marshal(p.Config)
			if e := os.WriteFile(yySocial.Path, data, 0600); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			yySocial.Config = p.Config
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if p.Action == "stop" {
			if yySocial.Job.cancel != nil {
				yySocial.Job.cancel()
				if yySocial.Job.Kind == "visits" {
					yyPauseAutoConnection()
					if bm := getBotManagerLocked(); bm != nil {
						go bm.Disconnect()
					}
				}
			}
			yySocial.Job.State = "已停止"
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if yySocial.Job.cancel != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "已有批量任务正在运行，请先停止"})
			return
		}
		total := len(p.Targets) * len(p.Messages)
		if p.Kind == "visits" {
			total = len(p.Visits)
			for _, v := range p.Visits {
				if strings.TrimSpace(v.Server) == "" {
					writeJSON(w, map[string]any{"ok": false, "error": "服务器号不能为空"})
					return
				}
			}
		} else if p.Kind == "like" {
			p.Messages = []string{""}
			total = len(p.Targets)
			if yySocial.Config.LikePath == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "此验证服务的点赞端点尚未配置。已找到的公开仓库未提供该协议，不能假定点赞成功"})
				return
			}
		} else if p.Kind == "private_message" {
			if yySocial.Config.MessagePath == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "请配置验证服务提供的私信端点"})
				return
			}
		} else if p.Kind != "game_message" {
			writeJSON(w, map[string]any{"ok": false, "error": "未知任务类型"})
			return
		}
		if total < 1 || total > 10000 {
			writeJSON(w, map[string]any{"ok": false, "error": "任务数量应为 1 到 10000"})
			return
		}
		if p.Kind == "game_message" {
			for _, text := range p.Messages {
				if strings.HasPrefix(strings.TrimSpace(text), "/") {
					writeJSON(w, map[string]any{"ok": false, "error": "批量聊天请填写普通消息，命令请使用命令链条"})
					return
				}
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		yySocial.Job = yySocialJob{Kind: p.Kind, State: "运行中", Total: total, Results: []map[string]any{}, cancel: cancel}
		if p.Kind == "visits" {
			go yyRunVisits(ctx, p.Visits)
		} else {
			if p.IntervalMS < 2100 {
				p.IntervalMS = 2100
			}
			go yyRunSocialBatch(ctx, p.Kind, p.Targets, p.Messages, time.Duration(p.IntervalMS)*time.Millisecond)
		}
		writeJSON(w, map[string]any{"ok": true})
	})
}
