package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Metadata only: no prompts, command text, keys, file contents or raw packets.
type yyMCPTraceRecord struct {
	Owner string
	Meta  map[string]any
}

var yyMCPTraces = struct {
	sync.Mutex
	Rows []yyMCPTraceRecord
}{}

func yyMCPOutcome(verb string, status int, payload map[string]any, err error) (string, bool, bool) {
	confirmed := false
	row := payload
	if nested, ok := payload["result"].(map[string]any); ok {
		row = nested
	}
	if row["confirmed"] == true {
		confirmed = true
	}
	if confirmed && row["success"] == false {
		return "server_rejected", false, true
	}
	failure := status >= 400 || err != nil || payload["ok"] == false || (payload["error"] != nil && payload["error"] != "")
	if failure {
		message := fmt.Sprint(payload["error"])
		if err != nil {
			message = err.Error()
		}
		lower := strings.ToLower(message)
		if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline") || strings.Contains(message, "超时") {
			// Without explicit submitted=false, a write timeout is ambiguous.
			if verb != "GET" && row["submitted"] != false {
				return "timeout_unknown", false, confirmed
			}
			return "timeout_not_sent", true, confirmed
		}
		if status == 502 || status == 503 || status == 504 {
			if verb == "GET" {
				return "unavailable", true, confirmed
			}
			return "unavailable_write_unknown", false, confirmed
		}
		return "failed", false, confirmed
	}
	if confirmed {
		return "server_confirmed", false, true
	}
	if row["submitted"] == true {
		return "submitted_unconfirmed", false, false
	}
	return "completed", false, false
}
func yyMCPTraceEnd(r *http.Request, id, name, verb, path string, status int, payload map[string]any, err error, started time.Time) map[string]any {
	outcome, retryable, confirmed := yyMCPOutcome(verb, status, payload, err)
	meta := map[string]any{"trace_id": id, "tool": name, "method": verb, "api_path": strings.SplitN(path, "?", 2)[0], "http_status": status, "elapsed_ms": float64(time.Since(started).Microseconds()) / 1000, "outcome": outcome, "retryable": retryable, "execution_confirmed": confirmed, "at": started.Format(time.RFC3339Nano), "auto_retried": false}
	owner := ""
	if a, e := yyPoolActorRequest(r); e == nil {
		owner = a.ID
	}
	yyMCPTraces.Lock()
	yyMCPTraces.Rows = append(yyMCPTraces.Rows, yyMCPTraceRecord{owner, meta})
	if len(yyMCPTraces.Rows) > 256 {
		yyMCPTraces.Rows = append([]yyMCPTraceRecord(nil), yyMCPTraces.Rows[len(yyMCPTraces.Rows)-256:]...)
	}
	yyMCPTraces.Unlock()
	return meta
}
func yyMCPTraceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/mcp/traces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		actor, e := yyPoolActorRequest(r)
		if e != nil {
			http.Error(w, "身份未确认", 401)
			return
		}
		id := r.URL.Query().Get("trace_id")
		rows := []map[string]any{}
		yyMCPTraces.Lock()
		for _, x := range yyMCPTraces.Rows {
			if (actor.Admin || x.Owner == actor.ID) && (id == "" || x.Meta["trace_id"] == id) {
				rows = append(rows, x.Meta)
			}
		}
		yyMCPTraces.Unlock()
		if id == "" && len(rows) > 30 {
			rows = rows[len(rows)-30:]
		}
		writeJSON(w, map[string]any{"ok": true, "traces": rows, "known": len(rows) > 0, "retention": "last 256 calls, memory only; payloads excluded", "boundary": "host route execution only; a successful call is not automatically a game receipt"})
	})
}
