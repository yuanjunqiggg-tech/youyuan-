package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Fork keeps the original conversation and project untouched. Revisions made
// after the selected message are reversed in the new project when timestamps
// are available; old pre-0.9.2 turns can still fork their conversation.
func yyAgentForkRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/agent/fork", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, map[string]any{"ok": false, "error": "只支持 POST"})
			return
		}
		var p struct {
			ID        string `json:"id"`
			TurnIndex int    `json:"turn_index"`
			Message   string `json:"message"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		source, e := yyGetWorkspace(p.ID)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		source.Lock()
		if p.TurnIndex < 0 || p.TurnIndex >= len(source.Turns) || source.Turns[p.TurnIndex].Role != "user" {
			source.Unlock()
			writeJSON(w, map[string]any{"ok": false, "error": "请选择一条已完成的玩家消息"})
			return
		}
		selected := source.Turns[p.TurnIndex]
		if p.Message == "" {
			p.Message = selected.Text
		}
		if len(p.Message) > 32000 {
			source.Unlock()
			writeJSON(w, map[string]any{"ok": false, "error": "消息超过 32000 字符"})
			return
		}
		id := fmt.Sprintf("branch_%d", time.Now().UnixNano())
		fork := &yyWorkspace{ID: id, Name: source.Name + " · 分支", Language: source.Language,
			CustomPrompt: source.CustomPrompt, Skills: append([]string{}, yyEffectiveAgentSkills(source)...),
			AllowedTools: append([]string(nil), source.AllowedTools...), Model: source.Model,
			State: "待开始", PluginID: source.PluginID, SourceEntry: source.SourceEntry, Turns: append([]aiTurn(nil), source.Turns[:p.TurnIndex]...),
			Pending: []string{p.Message}}
		fork.SourceHashes = map[string]string{}
		for path, hash := range source.SourceHashes {
			fork.SourceHashes[path] = hash
		}
		for _, server := range source.Servers {
			server.Session = ""
			server.Version = ""
			fork.Servers = append(fork.Servers, server)
		}
		source.Unlock()
		if e := os.MkdirAll(yyWorkspaceRoot(fork), 0700); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		yyFileMu.Lock()
		for _, file := range yyProjectFiles(source) {
			name := file["path"].(string)
			content, readErr := yyReadProjectRaw(source, name)
			if readErr != nil {
				e = readErr
				break
			}
			target, pathErr := yyProjectPath(yyWorkspaceRoot(fork), name)
			if pathErr != nil {
				e = pathErr
				break
			}
			if pathErr = os.MkdirAll(filepath.Dir(target), 0700); pathErr != nil {
				e = pathErr
				break
			}
			if pathErr = os.WriteFile(target, content, 0600); pathErr != nil {
				e = pathErr
				break
			}
		}
		rewound := selected.Time != ""
		if e == nil && rewound {
			cutoff, parseErr := time.Parse(time.RFC3339Nano, selected.Time)
			if parseErr != nil {
				rewound = false
			} else {
				for _, rev := range yyProjectHistory(source) {
					revTime, timeErr := time.Parse(time.RFC3339Nano, rev.Time)
					if timeErr != nil || revTime.Before(cutoff) {
						continue
					}
					target, pathErr := yyProjectPath(yyWorkspaceRoot(fork), rev.Path)
					if pathErr != nil {
						e = pathErr
						break
					}
					if !rev.Existed {
						_ = os.Remove(target)
						continue
					}
					before := []byte(rev.Before)
					if rev.Encoding == "base64" {
						before, pathErr = base64.StdEncoding.DecodeString(rev.Before)
						if pathErr != nil {
							e = pathErr
							break
						}
					}
					if pathErr = os.MkdirAll(filepath.Dir(target), 0700); pathErr != nil {
						e = pathErr
						break
					}
					if pathErr = os.WriteFile(target, before, 0600); pathErr != nil {
						e = pathErr
						break
					}
				}
			}
		}
		yyFileMu.Unlock()
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		fork.eventLocked("user", p.Message)
		if e = fork.saveLocked(); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		yyWorkspaces.Lock()
		yyWorkspaces.items[id] = fork
		yyWorkspaces.Unlock()
		started := yyRunWorkspace(fork, fork.Model) == nil
		writeJSON(w, map[string]any{"ok": true, "id": id, "started": started, "files_rewound": rewound})
	})
}
