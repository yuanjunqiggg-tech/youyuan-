package main

import (
	"encoding/base64"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// QQ 登录态自检。
//
// 背景：NapCat 掉登录后进程仍然存活，只是反复出二维码，OneBot WS 端口从不监听。
// 之前面板只会显示 "connection refused"，用户完全不知道要去扫码。
// 这里把「登录态 / WS 是否在监听 / 二维码是否新鲜」做成一个接口，
// 前端据此自动切换成「扫码登录」视图，并在登录成功后自动收起。

// yyNapCatDir NapCat 的安装目录，二维码落在 <dir>/cache/qrcode.png。
func yyNapCatDir() string {
	if v := os.Getenv("YY_NAPCAT_DIR"); v != "" {
		return v
	}
	return "/opt/qqbot/napcat"
}

func yyQQQRCodePath() string {
	return filepath.Join(yyNapCatDir(), "cache", "qrcode.png")
}

// yyQQWSListening 探测 OneBot WebSocket 端口是否真的在监听。
// 只看进程存活是不够的：未登录时 NapCat 活着但不监听。
func yyQQWSListening(wsURL string) bool {
	host := "127.0.0.1:3001"
	if wsURL != "" {
		trimmed := wsURL
		for _, p := range []string{"ws://", "wss://", "http://", "https://"} {
			if len(trimmed) > len(p) && trimmed[:len(p)] == p {
				trimmed = trimmed[len(p):]
				break
			}
		}
		// 去掉路径部分
		for i := 0; i < len(trimmed); i++ {
			if trimmed[i] == '/' {
				trimmed = trimmed[:i]
				break
			}
		}
		if trimmed != "" {
			host = trimmed
		}
	}
	conn, e := net.DialTimeout("tcp", host, 800*time.Millisecond)
	if e != nil {
		return false
	}
	_ = conn.Close()
	return true
}

type yyQQQRCodeView struct {
	DataURL    string `json:"data_url,omitempty"`
	AgeSeconds int    `json:"age_seconds"`
	Fresh      bool   `json:"fresh"`
	Available  bool   `json:"available"`
	Path       string `json:"path,omitempty"`
}

// yyQQLoginView 是 GET /api/qq/login 的返回体。
type yyQQLoginView struct {
	OK         bool           `json:"ok"`
	Connected  bool           `json:"connected"`
	LoggedIn   bool           `json:"logged_in"`
	WSListening bool          `json:"ws_listening"`
	State      string         `json:"state"`
	BotID      string         `json:"bot_id,omitempty"`
	Hint       string         `json:"hint,omitempty"`
	QRCode     yyQQQRCodeView `json:"qrcode"`
}

// yyQQLoginSnapshot 汇总登录态。qrTTL 是二维码有效期（NapCat 约 2 分钟刷新一次）。
func yyQQLoginSnapshot() yyQQLoginView {
	cf := yyQQSnapshot()
	view := yyQQLoginView{OK: true, BotID: cf.OneBot.BotID}

	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()
	if bm != nil {
		view.Connected = bm.IsConnected()
	}
	view.WSListening = yyQQWSListening(cf.OneBot.URL)

	// 登录成功的判据：WS 在监听且桥接已连上。
	view.LoggedIn = view.WSListening && view.Connected
	if view.LoggedIn {
		view.State = "logged_in"
		view.Hint = "机器人已登录，能力可直接使用"
	} else if view.WSListening {
		view.State = "ws_up"
		view.Hint = "OneBot 端口已监听，但桥接还没握上手，稍等几秒或点重连"
	} else {
		view.State = "need_login"
		view.Hint = "QQ 未登录：用手机 QQ 扫描下方二维码完成授权"
	}

	path := yyQQQRCodePath()
	if st, e := os.Stat(path); e == nil {
		age := int(time.Since(st.ModTime()).Seconds())
		if age < 0 {
			age = 0
		}
		view.QRCode.Path = path
		view.QRCode.AgeSeconds = age
		view.QRCode.Fresh = age <= 110
		// 只在未登录时读图，已登录就不必把图片塞进响应。
		if !view.LoggedIn {
			if raw, e2 := os.ReadFile(path); e2 == nil && len(raw) > 0 && len(raw) < 512*1024 {
				view.QRCode.Available = true
				view.QRCode.DataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
			}
		}
	}
	if view.LoggedIn {
		view.QRCode = yyQQQRCodeView{}
	}
	return view
}

// yyQQLoginRoutes 注册登录态接口。
func yyQQLoginRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
			return
		}
		writeJSON(w, yyQQLoginSnapshot())
	})
}
