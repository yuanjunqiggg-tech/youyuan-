package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"bot-apk/config"
)

// yyGameAuthOrigin returns the configured authentication origin, falling back
// to the product authentication host so a fresh install without a saved
// config still reaches the real service.
func yyGameAuthOrigin() string {
	if cfg := config.LoadConfig(); cfg != nil && strings.TrimSpace(cfg.AuthURL) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.AuthURL), "/")
	}
	return yySecAuthOriginFallback()
}

// Credentials are managed through the owner's authenticated cloud vault.
// Device keys stay in the private configuration and never reach page markup.
func yyGameAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/game-auth", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "POST" {
			http.Error(w, "GET/POST required", 405)
			return
		}
		yyRemoteMu.RLock()
		key := yyRemote.DeviceToken
		yyRemoteMu.RUnlock()
		if key == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "请先登录游园平台账号，或在公网控制设置中配置自己的设备密钥"})
			return
		}
		body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 100000))
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求过大"})
			return
		}
		req, e := http.NewRequestWithContext(r.Context(), r.Method, yyGameAuthOrigin()+"/api/accounts", bytes.NewReader(body))
		if e != nil {
			http.Error(w, "请求失败", 500)
			return
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "YouYuan-Toolbox/"+yyAppVersion)
		client := &http.Client{Timeout: 75 * time.Second, Transport: http.DefaultTransport}
		resp, e := client.Do(req)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "独立认证服务连接失败，请检查网络后重试"})
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, io.LimitReader(resp.Body, 200000))
	})
}
