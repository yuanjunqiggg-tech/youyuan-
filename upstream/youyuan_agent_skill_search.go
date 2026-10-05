package main

import (
	"path/filepath"
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
			for i, line := range strings.Split(string(data), "\n") {
				if strings.Contains(strings.ToLower(line), query) {
					rows = append(rows, map[string]any{"skill": id, "section": filepath.Base(source), "line": i + 1, "excerpt": yyAgentBoundText(line, 350)})
					break
				}
			}
			if len(rows) >= 12 {
				return mustJSON(map[string]any{"matches": rows, "more_possible": true}), false
			}
		}
	}
	return mustJSON(map[string]any{"matches": rows, "hint": "load_skill(skill,section)读取该章节；单包字段用packet_schema(name)"}), false
}
