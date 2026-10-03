package main

import (
	"bot-apk/config"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"
)

var yyAutoConnection = struct {
	sync.Mutex
	busy, wanted, paused, attempted bool
	last                            time.Time
}{}

func yyNormalizeServer(raw string) string {
	raw = strings.TrimSpace(raw)
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
func yyBeginConnectionAttempt() bool {
	yyAutoConnection.Lock()
	defer yyAutoConnection.Unlock()
	if yyAutoConnection.busy {
		return false
	}
	yyAutoConnection.busy = true
	yyAutoConnection.wanted = true
	yyAutoConnection.paused = false
	yyAutoConnection.attempted = true
	yyAutoConnection.last = time.Now()
	return true
}
func yyEndConnectionAttempt() {
	yyAutoConnection.Lock()
	yyAutoConnection.busy = false
	yyAutoConnection.Unlock()
}
func yyPauseAutoConnection() {
	yyAutoConnection.Lock()
	yyAutoConnection.wanted = false
	yyAutoConnection.paused = true
	yyAutoConnection.Unlock()
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
	if now.Sub(yyAutoConnection.last) < time.Duration(interval)*time.Second {
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
			if !yyMayAutoConnect(c, connected, now) {
				continue
			}
			body, _ := json.Marshal(map[string]any{"token": c.Token, "server": c.ServerCode, "auth": c.AuthURL, "password": c.ServerPass, "use_new_protocol": c.UseNewProtocol})
			request := httptest.NewRequest("POST", "/api/bot/connect", strings.NewReader(string(body)))
			handleBotConnect(httptest.NewRecorder(), request)
		}
	}()
}
