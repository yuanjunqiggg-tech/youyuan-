package main

// youyuan_console_auth.go —— 本机控制台会话鉴权（DSH-04 / 安全审查 H1）
//
// 【威胁】控制台监听 127.0.0.1:19429，所有 /api/* 零鉴权且 CORS 回 *。
// 用户浏览器打开任意恶意网页，该页即可 fetch('http://127.0.0.1:19429/api/config')
// 读到游戏 Token / 服务器密码，或 POST /api/bot/console 以 OP 身份执行命令。
//
// 【修法】启动时生成 32 字节 CSPRNG token（进程内，重启即换）：
//   1. handleIndex 把 token 注入 <meta name="yy-console-token">；
//   2. 前端 youyuan_console_token.js 包装 window.fetch，给所有同源 /api/ 请求
//      加 X-YouYuan-Console-Token（44 处裸 fetch 不用逐个改）；
//   3. 本中间件校验 header / Authorization / ?token= 三种携带方式。
//
// 【必须放行的三类，否则会把用户自己锁在门外】
//   - OPTIONS 预检：预检带不了自定义头，跨站由 CORS 层 403；
//   - 远控 key：yyStartRemote 用 http://127.0.0.1:<port> 转发，带 Authorization: Bearer <cfg.Key>；
//   - 导出下载：/api/youyuan/exports/download?id=&token= 是能力型 URL，handler 内自己校验。

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	yyConsoleTokenOnce sync.Once
	yyConsoleToken     string
)

// yyConsoleTokenValue 返回本进程的控制台 token；第一次调用时生成。
func yyConsoleTokenValue() string {
	yyConsoleTokenOnce.Do(func() {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			// 极端环境拿不到 CSPRNG：退回时间摘要，仍比空 token 强。
			sum := sha256.Sum256([]byte(time.Now().String()))
			yyConsoleToken = hex.EncodeToString(sum[:])
			return
		}
		yyConsoleToken = hex.EncodeToString(b)
	})
	return yyConsoleToken
}

func yyConsoleTokenEqual(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(v), []byte(yyConsoleTokenValue())) == 1
}

func yyConsoleRemoteKey() string {
	yyRemoteMu.RLock()
	defer yyRemoteMu.RUnlock()
	return yyRemote.Key
}

// yyConsoleRequestAllowed：控制台 token 或远控 key 任一命中即可。
func yyConsoleRequestAllowed(r *http.Request) bool {
	if yyConsoleTokenEqual(r.Header.Get("X-YouYuan-Console-Token")) {
		return true
	}
	bearer := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if yyConsoleTokenEqual(bearer) {
		return true
	}
	if key := strings.TrimSpace(yyConsoleRemoteKey()); key != "" {
		if subtle.ConstantTimeCompare([]byte(bearer), []byte(key)) == 1 {
			return true
		}
	}
	if yyConsoleTokenEqual(r.URL.Query().Get("token")) {
		return true
	}
	return false
}

// yyConsoleAuth 只拦 /api/*；静态资源与 /health 直接放行。
func yyConsoleAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p != "/api" && !strings.HasPrefix(p, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		// 预检不能带自定义头；跨站预检由 CORS 层直接 403。
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		// 导出下载是能力型 URL：handler 内已校验 id + token，这里不再要求会话 token。
		if p == "/api/youyuan/exports/download" {
			next.ServeHTTP(w, r)
			return
		}
		// <img src> 带不了自定义头：头像/皮肤与市场预览图放行只读 GET。
		// 这两类只回图片，不暴露配置、不能执行命令；跨站 JS 仍被 CORS 挡住。
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if strings.HasPrefix(p, "/api/skin/") || strings.HasPrefix(p, "/api/market-proxy/") {
				next.ServeHTTP(w, r)
				return
			}
		}
		if yyConsoleRequestAllowed(r) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]any{"ok": false, "error": "控制台会话已失效，请重新打开工具箱"})
	})
}

// yyConsoleInjectToken 把 token 注入 index.html 的 <head>（幂等）。
func yyConsoleInjectToken(data []byte) []byte {
	if bytes.Contains(data, []byte("yy-console-token")) {
		return data
	}
	meta := []byte(`<meta name="yy-console-token" content="` + yyConsoleTokenValue() + `">`)
	return bytes.Replace(data, []byte("</head>"), append(meta, []byte("</head>")...), 1)
}
