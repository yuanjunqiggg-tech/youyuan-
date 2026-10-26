package main

import (
	"bot-apk/config"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var yyAutoConnection = struct {
	sync.Mutex
	busy, wanted, paused, attempted bool
	last                            time.Time
	attempts, failures              uint64
	lastError                       string
	history                         []map[string]any
}{}

var yyConfigMutationMu sync.Mutex

func yySaveConnectionCredentials(token, server, auth, password string, newProtocol bool) error {
	yyConfigMutationMu.Lock()
	defer yyConfigMutationMu.Unlock()
	c := config.LoadConfig()
	c.Token, c.ServerCode, c.AuthURL, c.ServerPass, c.UseNewProtocol = token, server, auth, password, newProtocol
	return config.SaveConfig(c)
}

var yyRoomID = regexp.MustCompile(`^[0-9]{16,20}$`)
var yyExplicitRoomID = regexp.MustCompile(`^[0-9]+$`)

func yyNormalizeServer(raw string) string {
	raw = strings.TrimSpace(raw)
	// Keep IDs as strings: a room ID exceeds JavaScript's safe integer range.
	if yyRoomID.MatchString(raw) {
		return "LobbyGame:room_id=" + raw
	}
	if strings.HasPrefix(raw, "#") && yyExplicitRoomID.MatchString(raw[1:]) {
		return "LobbyGame:room_id=" + raw[1:]
	}
	if strings.HasPrefix(raw, "LobbyGame:") && yyExplicitRoomID.MatchString(strings.TrimPrefix(raw, "LobbyGame:")) {
		return "LobbyGame:room_id=" + strings.TrimPrefix(raw, "LobbyGame:")
	}
	if strings.Contains(raw, "mc.163.com/") {
		if !strings.Contains(raw, "://") {
			raw = "https://" + raw
		}
		if u, e := url.Parse(raw); e == nil && u.Query().Get("realms") != "" {
			return u.Query().Get("realms")
		}
	}
	return raw
}

// 用户「主动断开」是权威意图：必须跨重启保留，否则服务一重启 auto_connect
// 就把它顶回来，用户看到的就是"我明明断了它自己又连上"。
// 用独立标记文件而不是写进 config.json：配置会被前端整份回写，标记文件不会被误覆盖。
var yyPauseMarkerPath string

func yyPauseStateInit(dataDir string) {
	if dataDir == "" {
		return
	}
	yyPauseMarkerPath = filepath.Join(dataDir, "auto-connect-paused")
	if _, err := os.Stat(yyPauseMarkerPath); err == nil {
		yyAutoConnection.Lock()
		yyAutoConnection.paused, yyAutoConnection.wanted = true, false
		yyAutoConnection.Unlock()
		debugLog("[连接] 检测到上次是主动断开，本次启动不自动重连，等待手动连接")
	}
}

func yyWritePauseMarker(paused bool) {
	if yyPauseMarkerPath == "" {
		return
	}
	if paused {
		_ = os.WriteFile(yyPauseMarkerPath, []byte(time.Now().Format(time.RFC3339)), 0o644)
		return
	}
	_ = os.Remove(yyPauseMarkerPath)
}

func yyBeginConnectionAttempt() bool {
	yyAutoConnection.Lock()
	defer yyAutoConnection.Unlock()
	if yyAutoConnection.busy {
		return false
	}
	yyAutoConnection.busy = true
	yyAutoConnection.wanted = true
	yyAutoConnection.paused = false
	yyWritePauseMarker(false)
	yyAutoConnection.attempted = true
	yyAutoConnection.last = time.Now()
	yyAutoConnection.attempts++
	return true
}
func yyEndConnectionAttempt(err error) {
	yyAutoConnection.Lock()
	yyAutoConnection.busy = false
	yyAutoConnection.last = time.Now()
	if err != nil {
		yyAutoConnection.failures++
		yyAutoConnection.lastError = err.Error()
		yyAutoConnection.history = append(yyAutoConnection.history, map[string]any{"at": time.Now().Format(time.RFC3339Nano), "kind": yyConnectionErrorKind(err.Error()), "error": err.Error(), "attempt": yyAutoConnection.attempts})
		if len(yyAutoConnection.history) > 64 {
			yyAutoConnection.history = yyAutoConnection.history[len(yyAutoConnection.history)-64:]
		}
	} else {
		yyAutoConnection.failures = 0
		yyAutoConnection.lastError = ""
	}
	yyAutoConnection.Unlock()
}

func yyResumeAutoConnection() {
	yyAutoConnection.Lock()
	yyAutoConnection.paused, yyAutoConnection.wanted = false, true
	yyAutoConnection.last = time.Time{}
	yyAutoConnection.Unlock()
	yyWritePauseMarker(false)
}

func yyConnectionState(c *config.AppConfig, bm *BotManager) map[string]any {
	yyAutoConnection.Lock()
	state := map[string]any{"auto_connect": c.AutoConnect, "auto_reconnect": c.AutoReconnect,
		"auto_creative": c.AutoCreative, "reconnect_seconds": c.ReconnectSeconds,
		"paused": yyAutoConnection.paused, "connecting": yyAutoConnection.busy,
		"attempts": yyAutoConnection.attempts, "consecutive_failures": yyAutoConnection.failures,
		"last_error": yyAutoConnection.lastError, "retry_limit": nil, "last_error_kind": yyConnectionErrorKind(yyAutoConnection.lastError), "failures_history": append([]map[string]any{}, yyAutoConnection.history...), "effective_retry_seconds": yyConnectionRetryDelay(time.Duration(c.ReconnectSeconds)*time.Second, yyAutoConnection.failures, yyAutoConnection.lastError).Seconds()}
	yyAutoConnection.Unlock()
	state["action_hint"] = yyConnectionActionHint(fmt.Sprint(state["last_error_kind"]))
	state["server_action_required"] = state["last_error_kind"] == "server_expired"
	state["connected"], state["creative_confirmed"], state["flying_confirmed"] = false, false, false
	if bm != nil {
		state["connected"] = bm.IsConnected()
		bm.mu.Lock()
		state["creative_confirmed"] = bm.connected && bm.gameMode == 1
		state["flying_confirmed"] = bm.connected && bm.isFlying
		bm.mu.Unlock()
	}
	return state
}

// Partial policy updates cannot erase tokens/servers or unrelated settings.
func yyConnectionPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if r.Method == "POST" {
		var patch struct {
			AutoConnect   *bool `json:"auto_connect"`
			AutoReconnect *bool `json:"auto_reconnect"`
			AutoCreative  *bool `json:"auto_creative"`
			Seconds       *int  `json:"reconnect_seconds"`
			Resume        bool  `json:"resume"`
		}
		if e := json.NewDecoder(r.Body).Decode(&patch); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
			return
		}
		if patch.Seconds != nil && (*patch.Seconds < 1 || *patch.Seconds > 86400) {
			writeJSON(w, map[string]any{"ok": false, "error": "重连间隔应为1至86400秒"})
			return
		}
		yyConfigMutationMu.Lock()
		c := config.LoadConfig()
		previousAutoReconnect := c.AutoReconnect
		if patch.AutoConnect != nil {
			c.AutoConnect = *patch.AutoConnect
		}
		if patch.AutoReconnect != nil {
			c.AutoReconnect = *patch.AutoReconnect
		}
		if patch.AutoCreative != nil {
			c.AutoCreative = *patch.AutoCreative
		}
		if patch.Seconds != nil {
			c.ReconnectSeconds = *patch.Seconds
		}
		if patch.Resume {
			c.AutoReconnect = true
		}
		e := config.SaveConfig(c)
		yyConfigMutationMu.Unlock()
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": fmt.Sprint(e)})
			return
		}
		// 只有「显式 resume」或「自动重连从关变开」才算用户要恢复自动连接。
		// 原来只要 body 里带 auto_reconnect:true 就 resume，于是前端每次同步策略
		// 都会把用户手动断开的暂停状态清掉——这就是"断了又自己连回来"。
		if patch.Resume || patch.AutoReconnect != nil && *patch.AutoReconnect && !previousAutoReconnect {
			yyResumeAutoConnection()
		}
		if patch.AutoCreative != nil && *patch.AutoCreative {
			yyTouchCurrentMaintainer()
		}
	}
	writeJSON(w, map[string]any{"ok": true, "policy": yyConnectionState(config.LoadConfig(), getBotManagerLocked())})
}

func yyTouchCurrentMaintainer() {
	if bm := getBotManagerLocked(); bm != nil && bm.IsConnected() {
		go func() {
			if gi, e := bm.getGameInterface(); e == nil && gi.Maintainer() != nil {
				if e = gi.Maintainer().TouchMaintainer(gi); e != nil {
					debugLog("[模式维护] %v", e)
				}
			}
		}()
	}
}
func yyPauseAutoConnection() {
	yyAutoConnection.Lock()
	yyAutoConnection.wanted = false
	yyAutoConnection.paused = true
	yyAutoConnection.Unlock()
	yyWritePauseMarker(true)
}
func yyMayAutoConnect(c *config.AppConfig, connected bool, now time.Time) bool {
	yyAutoConnection.Lock()
	defer yyAutoConnection.Unlock()
	if connected {
		yyAutoConnection.wanted = true
		return false
	}
	if yyAutoConnection.busy || yyAutoConnection.paused || c.Token == "" || c.ServerCode == "" || c.AuthURL == "" {
		return false
	}
	interval := c.ReconnectSeconds
	if interval < 1 {
		interval = 10
	}
	if now.Sub(yyAutoConnection.last) < yyConnectionRetryDelay(time.Duration(interval)*time.Second, yyAutoConnection.failures, yyAutoConnection.lastError) {
		return false
	}
	return c.AutoConnect && !yyAutoConnection.attempted || c.AutoReconnect && yyAutoConnection.wanted
}
func yyStartAutoConnection() {
	go func() {
		timer := time.NewTicker(time.Second)
		defer timer.Stop()
		for now := range timer.C {
			c := config.ReloadConfig()
			bm := getBotManagerLocked()
			connected := bm != nil && bm.IsConnected()
			// 掉线可观测：连上之后又被踢的跳变只有这里能看到，记进诊断报告。
			server := ""
			if bm != nil {
				server = bm.ServerCode()
			}
			yyNoteConnectionSample(connected, server)
			if !yyMayAutoConnect(c, connected, now) {
				continue
			}
			body, _ := json.Marshal(map[string]any{"token": c.Token, "server": c.ServerCode, "auth": c.AuthURL, "password": c.ServerPass, "use_new_protocol": c.UseNewProtocol})
			request := httptest.NewRequest("POST", "/api/bot/connect", strings.NewReader(string(body)))
			handleBotConnect(httptest.NewRecorder(), request)
		}
	}()
}
