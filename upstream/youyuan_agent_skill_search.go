package main

import (
	"path/filepath"
	"sort"
	"strings"
)

func yySearchWorkspaceSkills(ws *yyWorkspace, query string) (string, bool) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return "请输入功能或方法名", true
	}
	ws.Lock()
	ids := yyEffectiveAgentSkills(ws)
	ws.Unlock()
	rows := []map[string]any{}
	tokens := strings.Fields(query)
	for _, id := range ids {
		sk := resolveSkill(id)
		if sk == nil {
			continue
		}
		for _, source := range sk.Sources {
			data, e := aiSkillsFS.ReadFile(source)
			if e != nil {
				continue
			}
			offset := 0
			for i, line := range strings.Split(string(data), "\n") {
				lower, score := strings.ToLower(line), 0
				if strings.Contains(lower, query) {
					score = 100
				} else if len(tokens) > 1 {
					for _, token := range tokens {
						if strings.Contains(lower, token) {
							score++
						}
					}
				}
				if score > 0 {
					rows = append(rows, map[string]any{"skill": id, "section": filepath.Base(source), "line": i + 1, "offset": offset, "score": score, "excerpt": yyAgentBoundText(line, 350)})
				}
				offset += len([]rune(line)) + 1
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i]["score"].(int) > rows[j]["score"].(int) })
	more := len(rows) > 12
	if more {
		rows = rows[:12]
	}
	return mustJSON(map[string]any{"matches": rows, "more_possible": more, "hint": "load_skill(skill,section,offset,max_chars)直接读取匹配位置；多词按相关性匹配，单包字段用packet_schema(name)"}), false
}
