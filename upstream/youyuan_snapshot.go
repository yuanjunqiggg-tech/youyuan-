package main

import (
	"encoding/json"
	"net/http"
)

// 统一快照：把首页概览原来要打的 7 条 GET 合并成 1 条。
//
// 为什么要合并：工具箱对外走 Cloudflare Worker + Durable Object 中转，
// 每一条 /api/* 请求 = 1 次 Worker 计数 + 1 次 DO 计数（双扣）。
// 首页概览原先把 bot/status、remote、runtime/list、task/list、plugin/list、
// qq/status、connection-diagnostics 拆成 7 条并发请求，
// 一个可见标签页 30 秒一轮就是 14 次/分钟；合并后只剩 2 次/分钟。
//
// 实现方式：进程内直调既有 handler（yyInvoke 走同一张 mux），
// 不复制任何业务逻辑，避免出现两份会各自漂移的实现。
// 任何单条失败只影响该字段，不拖垮整包（客户端仍能拿到其余数据）。
func yySnapshotHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items := []struct {
		key     string
		path    string
		note    string
		omitNil bool
	}{
		{"game", "/api/bot/status", "游戏服务器连接状态", false},
		{"remote", "/api/youyuan/remote", "公网设备中转状态", false},
		{"runtime", "/api/youyuan/runtime/list", "常驻插件实例", false},
		{"tasks", "/api/task/list", "任务列表", false},
		{"plugins", "/api/plugin/list", "兼容插件列表", false},
		{"qq", "/api/qq/status", "群服互通状态", false},
		{"diag", "/api/youyuan/connection-diagnostics", "连接诊断（含今日掉线）", true},
	}
	out := map[string]any{"ok": true, "snapshot": true}
	failed := 0
	for _, item := range items {
		code, raw, err := yyInvoke("GET", item.path, nil, nil)
		if err != nil {
			failed++
			out[item.key] = map[string]any{"ok": false, "error": err.Error()}
			continue
		}
		var parsed any
		if len(raw) == 0 || json.Unmarshal(raw, &parsed) != nil {
			// 空响应或非 JSON：如实标注，不伪造成功。
			failed++
			out[item.key] = map[string]any{"ok": false, "error": "响应不是 JSON", "status": code}
			continue
		}
		out[item.key] = parsed
	}
	// 全部失败时把整体 ok 置否，让前端能区分“设备离线”和“部分字段缺失”。
	if failed == len(items) {
		out["ok"] = false
		out["error"] = "全部子接口读取失败"
	}
	writeJSON(w, out)
}
