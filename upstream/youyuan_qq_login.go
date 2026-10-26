package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	// Stale = 文件很久没更新，二维码可能已经过期。
	// 注意：Stale 只用于提示，不能当成「不可用」——以前前端把 !Fresh 直接盖一层
	// 「正在刷新…」的遮罩，而 NapCat 并不会按固定间隔重写 qrcode.png，
	// 结果遮罩永远盖着，用户看到一个「正在刷新」但永远刷不出来的白框。
	Stale     bool `json:"stale"`
	Available bool `json:"available"`
	Path      string `json:"path,omitempty"`
}

// yyQQLoginView 是 GET /api/qq/login 的返回体。
type yyQQLoginView struct {
	OK         bool           `json:"ok"`
	Connected  bool           `json:"connected"`
	LoggedIn   bool           `json:"logged_in"`
	WSListening bool          `json:"ws_listening"`
	QQConnected bool          `json:"qq_connected"`
	State      string         `json:"state"`
	BotID      string         `json:"bot_id,omitempty"`
	Hint       string         `json:"hint,omitempty"`
	QRCode     yyQQQRCodeView `json:"qrcode"`
	// LoginModes 是登录方式清单。用户明确要求「界面能选密码登录」，
	// 但协议端（NapCat + QQ NT 内核）物理上没有密码登录接口，
	// 所以这里如实列出三种方式：能用的标可用，不能用的标原因，
	// 而不是放一个点了没反应的假按钮。
	LoginModes []yyQQLoginMode `json:"login_modes"`
	// Watchdog 掉线自愈状态：掉线后系统会自己重启协议端走快速登录。
	Watchdog map[string]any `json:"watchdog,omitempty"`
	// Live 是「真的还能问 QQ 一句话吗」（见 youyuan_qq_liveness.go）。
	// 只看 socket 会把「假在线」判成已登录，用户等 5 分钟也等不到扫码提示。
	Live            bool   `json:"live"`
	LiveEverChecked bool   `json:"live_ever_checked"`
	LastOKSeconds   int    `json:"last_ok_seconds"`
	OfflineSeconds  int    `json:"offline_seconds"`
	LiveError       string `json:"live_error,omitempty"`
	// NeedScan 掉线超过 5 分钟（或自愈额度用尽）→ 必须让用户扫码，不许再"等等看"。
	NeedScan bool `json:"need_scan"`
	// QRMissing 二维码文件不存在：面板要直接给「重新生成二维码」，不能只显示空白。
	QRMissing bool `json:"qr_missing"`
}

type yyQQLoginMode struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
	Desc      string `json:"desc"`
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

	// 登录成功的判据只能是「QQ 侧」：OneBot 端口在听 + 我们的 OneBot 连接是 open。
	// 以前用的是 bm.IsConnected()，那是**游戏服**机器人的连接状态，
	// 跟 QQ 登录没有任何关系——游戏掉线时会把 QQ 判成未登录，于是反复塞二维码。
	qqOpen := yyQQStateNow() == "open"
	view.QQConnected = qqOpen

	// 自愈状态要在下面判断 need_scan 之前就拿到（额度用尽 = 必须扫码）。
	view.Watchdog = yyQQWatchdogStatus()

	// 2026-10-26 第 30 条：光看 socket 会「假在线」。
	// live 只有在探测成功过之后才有意义；从没探测过时 fail-open，不误判。
	// 从没探测过时后台补一次（不阻塞面板）；已经有数据就不补。
	yyQQLiveProbeAsync()
	liveOK, lastOK, offline, liveErr, everOK, _ := yyQQLiveness()
	view.Live = liveOK
	view.LiveEverChecked = everOK
	view.LastOKSeconds = lastOK
	view.OfflineSeconds = offline
	view.LiveError = liveErr

	loggedIn := view.WSListening && qqOpen
	if everOK && !liveOK {
		loggedIn = false
	}
	view.LoggedIn = loggedIn

	// 自愈额度用尽时，watchdog 会把原因写进 LastErr；这也算「必须扫码」。
	quotaExhausted := false
	if wd, ok := view.Watchdog["error"]; ok {
		if s, ok := wd.(string); ok && strings.Contains(s, "扫码") {
			quotaExhausted = true
		}
	}
	view.NeedScan = !loggedIn && (quotaExhausted || offline >= int(yyQQOfflineEscalate.Seconds()))

	if view.LoggedIn {
		view.State = "logged_in"
		view.Hint = "机器人已登录，能力可直接使用"
	} else if view.NeedScan {
		view.State = "need_scan"
		view.Hint = "QQ 已掉线超过 5 分钟且自动恢复没成功：请用手机 QQ 扫下方二维码重新登录。"
		if quotaExhausted {
			view.Hint = "自动重启已用尽额度仍未恢复：登录态基本确定失效了，必须扫码重新登录。"
		}
	} else if view.WSListening {
		view.State = "ws_up"
		view.Hint = "OneBot 端口已监听，但桥接还没握上手，稍等几秒或点重连"
	} else {
		view.State = "need_login"
		// 掉线不等于要扫码：多数情况是协议端卡住，重启就能快速登录回来。
		// 先别把用户推去扫码，让自愈跑一轮。
		view.Hint = "QQ 未登录。系统会自动重启协议端尝试快速登录（保留登录态，通常不用扫码）；等 2~3 分钟仍不恢复，再用手机 QQ 扫下方二维码。"
	}
	view.LoginModes = []yyQQLoginMode{
		{ID: "qrcode", Label: "扫码登录", Available: true, Active: !view.LoggedIn,
			Desc: "手机 QQ 扫一下完成授权。首次登录、或登录态已经失效时走这个。"},
		{ID: "quick", Label: "快速登录", Available: true, Active: view.LoggedIn,
			Desc: "复用已保存的登录态，协议端重启后免扫码自动上线。这就是「记住密码」的等价物，也是长期不掉线的关键。"},
		{ID: "password", Label: "账号密码登录", Available: false, Active: false,
			Desc: "协议端不支持：Linux 版 QQ（NT 内核）没有明文密码登录接口，只有扫码和快速登录两种。填密码没有可提交的入口，所以不做假按钮；长期在线靠上面的快速登录 + 掉线自愈。"},
	}
	path := yyQQQRCodePath()
	if st, e := os.Stat(path); e == nil {
		// 二维码图片按 mtime+size 缓存 base64：
		// 前端每 20 秒轮询一次 /api/qq/login，原来每次都读盘 + 重新 base64，
		// 实测 10.5MB/小时的无意义流量。图片没变就直接用缓存。
		_ = st
		age := int(time.Since(st.ModTime()).Seconds())
		if age < 0 {
			age = 0
		}
		view.QRCode.Path = path
		view.QRCode.AgeSeconds = age
		view.QRCode.Fresh = age <= 110
		view.QRCode.Stale = age > 110
		// 只在未登录时读图，已登录就不必把图片塞进响应。
		if !view.LoggedIn {
			view.QRCode.DataURL = yyQQQRCodeDataURL(path, st.ModTime().UnixNano(), st.Size())
			view.QRCode.Available = view.QRCode.DataURL != ""
		}
	}
	if view.LoggedIn {
		view.QRCode = yyQQQRCodeView{}
		view.NeedScan = false
	} else if !view.QRCode.Available {
		// 没二维码文件 / 读不出来：前端不能干等着，必须给「重新生成二维码」的入口。
		view.QRMissing = true
	}
	return view
}

// yyQQQRCodeCache 缓存二维码的 data URL，键是文件的 mtime+size。
// 图片不变就不重复读盘和 base64；变了自动失效。
var yyQQQRCodeCache = struct {
	sync.Mutex
	stamp   string
	dataURL string
}{}

func yyQQQRCodeDataURL(path string, mtime, size int64) string {
	stamp := fmt.Sprintf("%d:%d", mtime, size)
	yyQQQRCodeCache.Lock()
	defer yyQQQRCodeCache.Unlock()
	if yyQQQRCodeCache.stamp == stamp {
		return yyQQQRCodeCache.dataURL
	}
	raw, e := os.ReadFile(path)
	if e != nil || len(raw) == 0 || len(raw) >= 512*1024 {
		yyQQQRCodeCache.stamp = ""
		yyQQQRCodeCache.dataURL = ""
		return ""
	}
	yyQQQRCodeCache.stamp = stamp
	yyQQQRCodeCache.dataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
	return yyQQQRCodeCache.dataURL
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
	// 重新拉起 NapCat 出二维码。
	// 之前面板上「正在刷新…」是纯前端文案，后端根本没有刷新动作，所以永远刷不出来。
	// 这里给一个真实入口：执行环境变量 YY_NAPCAT_RESTART_CMD 指定的命令
	// （比如 systemctl restart napcat 或 docker restart napcat）。
	// 没配就明确告诉用户，不要假装在刷新。
	mux.HandleFunc("/api/qq/login/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
			return
		}
		cmdLine := strings.TrimSpace(os.Getenv("YY_NAPCAT_RESTART_CMD"))
		if cmdLine == "" {
			writeJSON(w, map[string]any{"ok": false, "code": "no_restart_cmd",
				"error": "服务端没有配置重启命令，无法自动重新生成二维码。在 box 上设置环境变量 YY_NAPCAT_RESTART_CMD（例如 systemctl restart napcat）后重启工具即可；也可以手动重启 NapCat。"})
			return
		}
		// 不能用 r.Context()：用户点完就关页面会把重启打断，
		// 结果 NapCat 半停不停。用独立 ctx。
		// 超时必须 ≥ napcat.service 的 TimeoutStopSec(30s)：QQ NT 正常退出
		// 就可能耗满 30 秒，旧的 20 秒超时会让「重新生成二维码」几乎必失败
		// （表现为前端弹 exit status 1），而守望犬用 90 秒所以能成功。
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		out, e := exec.CommandContext(ctx, "sh", "-c", cmdLine).CombinedOutput()
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "重启失败：" + e.Error(), "output": string(out)})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "output": string(out)})
	})
}
