package main

import (
	"bot-apk/config"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type yyNoticeConfig struct {
	Broadcast  bool   `json:"broadcast"`
	Tasks      bool   `json:"tasks"`
	Creator    string `json:"creator"`
	Join       string `json:"join"`
	Ready      string `json:"ready"`
	Permission string `json:"permission"`
	Anonymous  string `json:"anonymous"`
}

var yyNoticeMu sync.Mutex
var yyNotices = yyNoticeConfig{Creator: "设备用户", Join: "游园已连接服务器", Ready: "{bot} 已准备就绪", Permission: "权限校验未通过，请为 {bot} 授予所需权限", Anonymous: "当前为匿名连接，部分功能不可用"}

func yyNoticeSnapshot() yyNoticeConfig {
	yyNoticeMu.Lock()
	defer yyNoticeMu.Unlock()
	return yyNotices
}
func yyNoticeText(kind string) string {
	c := yyNoticeSnapshot()
	switch kind {
	case "join":
		return c.Join
	case "ready":
		return c.Ready
	case "permission":
		return c.Permission
	case "anonymous":
		return c.Anonymous
	}
	return ""
}
func (bm *BotManager) yyNotice(kind string) {
	c := yyNoticeSnapshot()
	text := yyNoticeText(kind)
	text = strings.NewReplacer("{bot}", bm.display, "{creator}", c.Creator).Replace(text)
	if text != "" && c.Broadcast {
		bm.sendChatMessage(text)
	}
	// The actual permission failure remains visible even when its game broadcast is suppressed.
	if kind == "permission" {
		text = "权限校验尚未通过。" + text
	}
	if hub != nil {
		hub.Emit("log", mustJSON("[连接提示] "+text))
	}
	yyEvent("connection_notice", map[string]any{"kind": kind, "text": text, "broadcast": c.Broadcast})
}
func (bm *BotManager) yyTaskNotice(text string) {
	if yyNoticeSnapshot().Tasks {
		bm.SendTellraw(text)
	}
	if hub != nil {
		hub.Emit("log", mustJSON("[作业] "+text))
	}
}
func yyMarketOrigin(c *config.AppConfig) string {
	if c.MarketURL != "" {
		return strings.TrimRight(c.MarketURL, "/")
	}
	return "https://prism.adblanlu.qzz.io"
}
func yyNoticeRoutes(mux *http.ServeMux, dataDir string) {
	path := filepath.Join(dataDir, "connection-notices.json")
	if data, e := os.ReadFile(path); e == nil {
		_ = json.Unmarshal(data, &yyNotices)
	}
	mux.HandleFunc("/api/youyuan/notices", func(w http.ResponseWriter, r *http.Request) {
		yyNoticeMu.Lock()
		defer yyNoticeMu.Unlock()
		if r.Method == "POST" {
			candidate := yyNotices
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&candidate); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			data, _ := json.MarshalIndent(candidate, "", "  ")
			if e := os.WriteFile(path, data, 0600); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			yyNotices = candidate
		}
		writeJSON(w, map[string]any{"ok": true, "config": yyNotices})
	})
	mux.HandleFunc("/api/youyuan/feedback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeJSON(w, map[string]any{"ok": false, "error": "请提交反馈内容"})
			return
		}
		var p struct {
			Content string `json:"content"`
			Text    string `json:"text"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if p.Content == "" {
			p.Content = p.Text
		}
		if strings.TrimSpace(p.Content) == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "反馈不能为空"})
			return
		}
		dir := filepath.Join(dataDir, "feedback")
		_ = os.MkdirAll(dir, 0700)
		id := time.Now().UTC().Format("20060102T150405.000000000")
		data, _ := json.Marshal(map[string]any{"id": id, "content": p.Content, "time": time.Now()})
		if e := os.WriteFile(filepath.Join(dir, id+".json"), data, 0600); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": id, "message": "反馈已保存到设备，可通过公网 API 读取反馈目录"})
	})
	mux.HandleFunc("/api/market-proxy/", handleMarketProxy)
	mux.HandleFunc("/api/market/download", handleMarketDownload)
	mux.HandleFunc("/api/market/contribute", handleMarketContribute)
	mux.HandleFunc("/api/plugin-market-proxy/", handlePluginMarketProxy)
	mux.HandleFunc("/api/plugin-market/install", handlePluginMarketInstall)
	mux.HandleFunc("/api/plugin-market/contribute", handlePluginMarketContribute)
}
