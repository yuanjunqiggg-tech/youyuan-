package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Conversation pages use durable turns, rather than the bounded live event log.
func yyAgentSessionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/agent/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		var p struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		source, e := yyGetWorkspace(p.ID)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		source.Lock()
		ws := &yyWorkspace{ID: fmt.Sprintf("session_%d", time.Now().UnixNano()), Name: p.Name, Language: source.Language, Model: source.Model, CustomPrompt: source.CustomPrompt, Skills: append([]string{}, yyEffectiveAgentSkills(source)...), AllowedTools: source.AllowedTools, PluginID: source.PluginID, SourceEntry: source.SourceEntry, State: "待开始"}
		if source.AllowedTools != nil {
			ws.AllowedTools = append([]string{}, source.AllowedTools...)
		}
		ws.SourceHashes = map[string]string{}
		for path, hash := range source.SourceHashes {
			ws.SourceHashes[path] = hash
		}
		for _, server := range source.Servers {
			server.Session = ""
			server.Version = ""
			ws.Servers = append(ws.Servers, server)
		}
		if ws.Name == "" {
			ws.Name = source.Name + " · 新会话"
		}
		source.Unlock()
		yyFileMu.Lock()
		e = os.MkdirAll(yyWorkspaceRoot(ws), 0700)
		total := 0
		if e == nil {
			for _, file := range yyProjectFiles(source) {
				name := file["path"].(string)
				var data []byte
				data, e = yyReadProjectRaw(source, name)
				if e != nil {
					break
				}
				total += len(data)
				if total > 128<<20 {
					e = fmt.Errorf("项目超过128MiB，请缩小附件后创建新会话")
					break
				}
				var path string
				path, e = yyProjectPath(yyWorkspaceRoot(ws), name)
				if e != nil {
					break
				}
				if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
					break
				}
				if e = os.WriteFile(path, data, 0600); e != nil {
					break
				}
			}
		}
		yyFileMu.Unlock()
		if e == nil {
			e = ws.saveLocked()
		}
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		yyWorkspaces.Lock()
		yyWorkspaces.items[ws.ID] = ws
		yyWorkspaces.Unlock()
		writeJSON(w, map[string]any{"ok": true, "id": ws.ID, "name": ws.Name})
	})
	mux.HandleFunc("/api/youyuan/agent/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		var p struct {
			ID string `json:"id"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		ws, e := yyGetWorkspace(p.ID)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		ws.Lock()
		defer ws.Unlock()
		if e = ws.saveLocked(); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "saved_at": time.Now().Format(time.RFC3339), "turns": len(ws.Turns), "state": ws.State})
	})
	mux.HandleFunc("/api/youyuan/agent/conversation", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		ws, e := yyGetWorkspace(r.URL.Query().Get("id"))
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		ws.Lock()
		defer ws.Unlock()
		total := len(ws.Turns)
		limit := 20
		end := total
		if raw := r.URL.Query().Get("before"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n >= 0 && n < end {
				end = n
			}
		}
		start := end - limit
		if start < 0 {
			start = 0
		}
		rows := []map[string]any{}
		for index := start; index < end; index++ {
			t := ws.Turns[index]
			rows = append(rows, map[string]any{"index": index, "role": t.Role, "content": t.Text, "time": t.Time, "tool_calls": t.ToolCalls, "tool_results": t.ToolResults})
		}
		// No MCP credentials or model keys are included in this view.
		writeJSON(w, map[string]any{"ok": true, "name": ws.Name, "state": ws.State, "turns": rows, "total": total, "before": start, "has_more": start > 0, "summary": ws.Summary})
	})
}
