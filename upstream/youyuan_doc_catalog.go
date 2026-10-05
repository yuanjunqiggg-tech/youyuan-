package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func yyDocPacket(name string) (map[string]any, error) {
	data, err := aiSkillsFS.ReadFile("skills/packet-reference/catalog.json")
	if err != nil {
		return nil, err
	}
	var catalog struct {
		Packets []map[string]any `json:"packets"`
	}
	if err = json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	for _, row := range catalog.Packets {
		if row["name"] == name || fmt.Sprint(row["id"]) == name {
			return row, nil
		}
	}
	return nil, fmt.Errorf("当前内核没有注册此协议包: %s", name)
}

// yyStructuredDoc handles chapter and method reads before the legacy routes.
func yyStructuredDoc(w http.ResponseWriter, r *http.Request) bool {
	q := r.URL.Query()
	id := q.Get("language")
	if alias := map[string]string{"lua": "lua-runtime", "python": "python-runtime", "native_cpp": "cpp-runtime"}[id]; alias != "" {
		id = alias
	}
	if !strings.HasSuffix(id, "-runtime") && id != "packet-reference" {
		return false
	}
	if q.Get("catalog") == "1" {
		path := "skills/runtime-reference/catalog.json"
		if id == "packet-reference" {
			path = "skills/packet-reference/catalog.json"
		}
		data, err := aiSkillsFS.ReadFile(path)
		var value any
		if err == nil {
			err = json.Unmarshal(data, &value)
		}
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		} else {
			writeJSON(w, map[string]any{"ok": true, "catalog": value})
		}
		return true
	}
	section := q.Get("section")
	if section == "" {
		section = "index.md"
	}
	if q.Get("full") == "1" {
		section = "full.md"
	}
	method := q.Get("method")
	if method != "" {
		parts := strings.Split(method, ".")
		if len(parts) != 2 {
			writeJSON(w, map[string]any{"ok": false, "error": "需要命名空间.方法"})
			return true
		}
		section = parts[0] + ".md"
	}
	var content string
	var err error
	if name := q.Get("packet"); name != "" && id == "packet-reference" {
		var row map[string]any
		row, err = yyDocPacket(name)
		if err == nil {
			content, _ = row["content"].(string)
		}
	} else {
		var data []byte
		data, err = yyDocSection(id, section)
		content = string(data)
	}
	if err == nil && method != "" {
		marker := "### `" + method + "`"
		start := strings.Index(content, marker)
		if start < 0 {
			err = fmt.Errorf("当前文档没有该方法")
		} else {
			content = content[start:]
			if end := strings.Index(content[len(marker):], "\n### "); end >= 0 {
				content = content[:len(marker)+end]
			}
		}
	}
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
	} else {
		writeJSON(w, map[string]any{"ok": true, "content": content, "sections": skillByID(id).Sources})
	}
	return true
}

func yyDocSection(id, section string) ([]byte, error) {
	sk := skillByID(id)
	if sk == nil {
		return nil, fmt.Errorf("技能不存在")
	}
	for _, source := range sk.Sources {
		if section == source || strings.HasSuffix(source, "/"+section) {
			return aiSkillsFS.ReadFile(source)
		}
	}
	if section == "full.md" && strings.HasSuffix(id, "-runtime") {
		return aiSkillsFS.ReadFile("skills/" + id + "/full.md")
	}
	return nil, fmt.Errorf("章节不存在")
}
