package main

import (
	"net/http"
	"time"
)

// 多群聊终端：一次读出所有「已启用」群的最后一条消息时间与条数。
//
// 为什么单独一条路由：面板顶部要同时显示 N 个群的未读角标，但首屏不能把
// 每个群的近 40 条消息全拉一遍（那就是 N 倍流量、N 倍首屏等待）。
// 这里只回 {群标识, 群名, 条数, 最后一条时间}，正文一条都不带；
// 前端拿最后一条时间比自己记的已读时间，只有真有新消息的群才去拉那一个群的正文。
type yyQQMessageSummaryRow struct {
	Group  string `json:"group"`
	Name   string `json:"name"`
	Total  int    `json:"total"`
	LastAt string `json:"last_at,omitempty"`
	LastIn bool   `json:"last_from_friend,omitempty"`
}

func yyQQMessageSummary() []yyQQMessageSummaryRow {
	cf := yyQQSnapshot()
	groups := yyQQGroups(cf)
	names := make(map[string]string, len(groups))
	ids := make([]string, 0, len(groups))
	for _, g := range groups {
		if !g.Enabled || g.ID == "" {
			continue
		}
		ids = append(ids, g.ID)
		names[g.ID] = g.Name
	}
	out := make([]yyQQMessageSummaryRow, 0, len(ids))
	yyQQMessages.Lock()
	for _, id := range ids {
		// 消息桶按 adapter+连接+群号 的哈希分片，改 GroupID 再算一次 key 就能取到该群的桶。
		target := cf
		target.GroupID = id
		rows := yyQQMessages.Groups[yyQQMessageKey(target)]
		row := yyQQMessageSummaryRow{Group: id, Name: names[id], Total: len(rows)}
		if row.Name == "" {
			row.Name = id
		}
		if len(rows) > 0 {
			row.LastAt = rows[len(rows)-1].Timestamp.Format(time.RFC3339Nano)
			row.LastIn = !rows[len(rows)-1].IsBot
		}
		out = append(out, row)
	}
	yyQQMessages.Unlock()
	return out
}

func yyQQMessageSummaryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/messages/summary", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "GET required"})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "groups": yyQQMessageSummary()})
	})
}
