package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func yySkillCatalog(language string) []map[string]any {
	out := []map[string]any{}
	for _, id := range yyAvailableAgentSkills(language) {
		sk := skillByID(id)
		sections := []string{}
		for _, p := range sk.Sources {
			sections = append(sections, filepath.Base(p))
		}
		out = append(out, map[string]any{"id": id, "name": sk.Name, "description": sk.Description, "sections": sections})
	}
	return out
}

func yyAllSkillCatalog() []map[string]any {
	out := []map[string]any{}
	seen := map[string]bool{}
	for _, language := range []string{"lua", "python", "native_cpp", "mcfunction"} {
		for _, row := range yySkillCatalog(language) {
			id := row["id"].(string)
			if !seen[id] {
				out = append(out, row)
				seen[id] = true
			}
		}
	}
	return out
}

func yySkillArchive(id string) ([]byte, error) {
	sk := skillByID(id)
	if sk == nil && id != "lua-runtime" {
		return nil, fmt.Errorf("技能不存在")
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	add := func(path string, data []byte) error {
		w, e := z.Create(path)
		if e != nil {
			return e
		}
		_, e = w.Write(data)
		return e
	}
	if sk == nil {
		if e := add("lua-runtime/plugin_doc.md", pluginDevDoc); e != nil {
			return nil, e
		}
	} else {
		for _, path := range sk.Sources {
			data, e := aiSkillsFS.ReadFile(path)
			if e != nil {
				return nil, e
			}
			// Keep references/ links within a portable, independent library.
			name := strings.TrimPrefix(path, "skills/")
			if strings.HasPrefix(name, "mcbe-commands/") {
				name = id + "/" + strings.TrimPrefix(name, "mcbe-commands/")
			}
			if e = add(name, data); e != nil {
				return nil, e
			}
		}
	}
	info := map[string]any{"id": id, "app_version": yyAppVersion, "loading": "用户在工作区技能库选择加载；按章节分页读取，无联网依赖"}
	if id != "mcbe-commands" {
		if e := add(id+"/SKILL.md", []byte("# "+id+"\n\n游园独立离线技能包。library.json 记录版本；目录内文件是可按需读取的章节。用户主动加载，读取文档不会运行游戏操作。\n")); e != nil {
			return nil, e
		}
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	if e := add(id+"/library.json", data); e != nil {
		return nil, e
	}
	if e := z.Close(); e != nil {
		return nil, e
	}
	return buf.Bytes(), nil
}

func yySkillLibraryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/skills/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("skill")
		data, e := yySkillArchive(id)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if r.Method == "POST" {
			dir := filepath.Join(filepath.Dir(yyAgentRoot), "skill-exports")
			e = os.MkdirAll(dir, 0700)
			path := filepath.Join(dir, id+".zip")
			if e == nil {
				e = os.WriteFile(path, data, 0600)
			}
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "src_path": path})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.zip"`)
		w.Write(data)
	})
	mux.HandleFunc("/api/youyuan/agent/skills", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			ID      string `json:"id"`
			Skill   string `json:"skill"`
			Enabled bool   `json:"enabled"`
		}
		id := r.URL.Query().Get("id")
		if r.Method == "POST" {
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&p); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			id = p.ID
		} else if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		ws, e := yyGetWorkspace(id)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		ws.Lock()
		defer ws.Unlock()
		if r.Method == "POST" {
			found := false
			for _, sk := range yyAvailableAgentSkills(ws.Language) {
				if sk == p.Skill {
					found = true
					break
				}
			}
			if !found {
				writeJSON(w, map[string]any{"ok": false, "error": "当前语言不支持这份技能"})
				return
			}
			next := []string{}
			for _, sk := range yyEffectiveAgentSkills(ws) {
				if sk != p.Skill {
					next = append(next, sk)
				}
			}
			if p.Enabled {
				next = append(next, p.Skill)
			}
			old := ws.Skills
			ws.Skills = next
			if e = ws.saveLocked(); e != nil {
				ws.Skills = old
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		}
		writeJSON(w, map[string]any{"ok": true, "skills": yyEffectiveAgentSkills(ws), "catalog": yySkillCatalog(ws.Language), "language": ws.Language})
	})
	// Manual preview is available before loading and does not change the AI selection.
	mux.HandleFunc("/api/youyuan/skills/read", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		offset, _ := strconv.Atoi(q.Get("offset"))
		content, e := yyAgentSkillText(q.Get("skill"), q.Get("section"), offset, 3000)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "content": content})
	})
}
