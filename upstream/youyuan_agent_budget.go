package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Zero round/tool counts mean no local count ceiling. Duration is always
// explicit so a forgotten provider loop cannot run indefinitely in a shared host.
type yyAgentRunBudget struct {
	MaxRounds  int `json:"max_rounds"`
	MaxTools   int `json:"max_tools"`
	MaxMinutes int `json:"max_minutes"`
}

func yyEffectiveRunBudget(b *yyAgentRunBudget) yyAgentRunBudget {
	if b == nil {
		return yyAgentRunBudget{MaxMinutes: 60}
	}
	result := *b
	if result.MaxMinutes == 0 {
		result.MaxMinutes = 60
	}
	return result
}
func (b yyAgentRunBudget) validate() error {
	if b.MaxRounds < 0 || b.MaxRounds > 10000 || b.MaxTools < 0 || b.MaxTools > 100000 || b.MaxMinutes < 1 || b.MaxMinutes > 1440 {
		return fmt.Errorf("轮次0–10000、工具请求0–100000（0不设次数上限），时间1–1440分钟")
	}
	return nil
}
func (b yyAgentRunBudget) roundsReached(rounds int) bool {
	return b.MaxRounds > 0 && rounds >= b.MaxRounds
}
func (b yyAgentRunBudget) toolsReached(tools int) bool { return b.MaxTools > 0 && tools >= b.MaxTools }
func yyAgentBudgetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/agent/run-budget", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		id := r.URL.Query().Get("id")
		var p struct {
			ID     string           `json:"id"`
			Budget yyAgentRunBudget `json:"budget"`
		}
		if r.Method == "POST" {
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "无效运行预算"})
				return
			}
			id = p.ID
			if e := p.Budget.validate(); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		}
		ws, e := yyGetWorkspace(id)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		ws.Lock()
		defer ws.Unlock()
		if r.Method == "POST" {
			old := ws.RunBudget
			copy := p.Budget
			ws.RunBudget = &copy
			if e := ws.saveLocked(); e != nil {
				ws.RunBudget = old
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		}
		writeJSON(w, map[string]any{"ok": true, "budget": yyEffectiveRunBudget(ws.RunBudget), "running": ws.cancel != nil, "applies_to": "next_run", "consecutive_failure_hard_stop": false})
	})
}
