package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type yySummaryOptions struct {
	After      uint64   `json:"after_sequence"`
	Limit      int      `json:"limit"`
	Types      []string `json:"types"`
	EntityType string   `json:"entity_type"`
	RuntimeID  string   `json:"runtime_id"`
}

func init() {
	yyExtraMethods["events.summary"] = "options={after_sequence,limit:20,types:[],entity_type,runtime_id}；只读规范事件计数和最近轻量条目，不展开装备/背包；完整事件用明确view=raw或事件游标接口。"
	yyExtraMethods["events.captureSummary"] = "options同events.summary；抓包计数与最近元数据，返回overwritten/sampled_out。抓包必须先开启；计数范围为当前保留环，不是全服。"
	yyExtraMethods["packet.catalog"] = "无参数；本版本所有注册包名/ID/字段数/能否抓包，单包字段用packet.schema或/api/youyuan/packets/schema?name=Animate；schema存在不代表服务器广播。"
}
func yyPacketCatalog() []map[string]any {
	rows := []map[string]any{}
	for _, x := range yyPacketSchema() {
		n := x["name"].(string)
		rows = append(rows, map[string]any{"name": n, "id": x["id"], "field_count": len(x["fields"].([]map[string]string)), "capture_available": !yyPacketPrivate(n), "bulk": yyPacketBulk(n), "delivery_guaranteed": false})
	}
	return rows
}
func yySummaryFromQuery(r *http.Request) yySummaryOptions {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	after, _ := strconv.ParseUint(q.Get("after_sequence"), 10, 64)
	types := []string{}
	if t := q.Get("types"); t != "" {
		types = strings.Split(t, ",")
	}
	return yySummaryOptions{after, limit, types, q.Get("entity_type"), q.Get("runtime_id")}
}
func yyObservationSummary(bm *BotManager, o yySummaryOptions, capture bool) map[string]any {
	connected := bm != nil && bm.IsConnected()
	bot := map[string]any{"connected": connected}
	var self uint64
	if bm != nil {
		bm.mu.Lock()
		self = bm.runtimeID
		bot["name"] = bm.display
		bot["dimension"] = bm.dimension
		bot["game_mode"] = bm.gameMode
		bot["runtime_id"] = strconv.FormatUint(self, 10)
		bm.mu.Unlock()
	}
	if o.Limit <= 0 {
		o.Limit = 20
	}
	if o.Limit > 100 {
		o.Limit = 100
	}
	allowed := map[string]bool{}
	for _, t := range o.Types {
		allowed[t] = true
	}
	counts := map[string]int{}
	entityCounts := map[string]int{}
	recent := []map[string]any{}
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	if a := s.Actors[self]; a != nil {
		bot["health"] = yyHealthSnapshot(a, connected)
		bot["position"] = a.Position
		bot["position_known"] = a.PositionKnown
		bot["position_stale"] = yyObservationStale(a.PositionTime, a.Present, connected)
	}
	for _, a := range s.Actors {
		if a.Present {
			entityCounts[a.EntityType]++
		}
	}
	visit := func(row map[string]any) {
		kind := fmt.Sprint(row["type"])
		seq, _ := row["sequence"].(uint64)
		if seq <= o.After || len(allowed) > 0 && !allowed[kind] {
			return
		}
		if o.EntityType != "" && row["entity_type"] != o.EntityType {
			return
		}
		rid := fmt.Sprint(row["runtime_id"])
		if d, ok := row["data"].(map[string]any); ok && d["entity_runtime_id"] != nil {
			rid = fmt.Sprint(d["entity_runtime_id"])
		}
		if o.RuntimeID != "" && rid != o.RuntimeID {
			return
		}
		counts[kind]++
		compact := map[string]any{}
		for _, key := range []string{"sequence", "type", "time", "timestamp", "event_id", "player", "entity_type", "runtime_id", "observed", "inferred", "source"} {
			if v, ok := row[key]; ok {
				compact[key] = v
			}
		}
		if rid != "<nil>" {
			compact["runtime_id"] = rid
		}
		recent = append(recent, compact)
		if len(recent) > o.Limit {
			recent = recent[1:]
		}
	}
	cursor := s.Seq
	overwritten, sampled := uint64(0), uint64(0)
	if capture {
		c := &s.Capture
		cursor = c.Seq
		overwritten, sampled = c.Dropped, c.SampledOut
		for i := 0; i < c.Count; i++ {
			visit(c.Records[(c.Head+i)%len(c.Records)])
		}
	} else {
		for _, x := range s.Events {
			visit(x)
		}
	}
	return map[string]any{"ok": true, "view": "summary", "connected": connected, "bot": bot, "cursor": cursor, "counts": counts, "recent": yyObservationCopyRows(recent), "observed_entity_counts": entityCounts, "overwritten": overwritten, "sampled_out": sampled, "scope": "this account retained decoded observations; not global server entities", "raw_hint": "explicit query view=raw or events.captureRead; full download via packets/export"}
}
func yyObservationSummaryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/observations/summary", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		account := r.URL.Query().Get("account")
		bm := yyAccountBot(account)
		if account != "" && account != "main" && bm == nil {
			http.Error(w, "账号不存在", 404)
			return
		}
		writeJSON(w, yyObservationSummary(bm, yySummaryFromQuery(r), r.URL.Query().Get("source") == "packets"))
	})
}
