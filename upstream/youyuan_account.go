package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func yyAccountBot(account string) *BotManager {
	if account == "" || account == "main" {
		return getBotManagerLocked()
	}
	bots := GetFleetBots()
	if n, err := strconv.Atoi(account); err == nil && n >= 1 && n <= len(bots) {
		return bots[n-1]
	}
	for _, bm := range bots {
		if botName(bm) == account || bm.idStr == account {
			return bm
		}
	}
	return nil
}

// 多账号调用不交换全局 botMgr，避免并发请求串号。
func yyAccountAction(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Account string `json:"account"`
		Action  string `json:"action"`
		Input   string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	bm := yyAccountBot(p.Account)
	if p.Action == "status" {
		writeJSON(w, map[string]any{"ok": true, "account": p.Account, "connected": bm != nil && bm.IsConnected()})
		return
	}
	if bm == nil || !bm.IsConnected() {
		writeJSON(w, map[string]any{"ok": false, "error": "目标账号未连接"})
		return
	}
	switch p.Action {
	case "console":
		result, err := bm.SendConsole(p.Input)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "result": result})
	case "position":
		value, err := bm.QueryPosition()
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "position": value})
	default:
		writeJSON(w, map[string]any{"ok": false, "error": "未知账号操作"})
	}
}
