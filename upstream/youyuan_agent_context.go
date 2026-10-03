package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func yyAgentBoundText(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + fmt.Sprintf("\n[模型上下文截取前 %d 字符 / 总 %d；完整原始结果仍保存。需要剩余源码时用 project_read(start_line,lines)，技能用 load_skill(offset,max_chars)，历史用 project_history(query)。]", limit, len(runes))
}
func yyAgentContextTurns(turns []aiTurn) []aiTurn {
	out := append([]aiTurn(nil), turns...)
	for i := range out {
		out[i].Text = yyAgentBoundText(out[i].Text, 10000)
		out[i].ToolResults = append([]aiToolResult(nil), out[i].ToolResults...)
		for j := range out[i].ToolResults {
			out[i].ToolResults[j].Content = yyAgentBoundText(out[i].ToolResults[j].Content, 10000)
		}
	}
	return out
}
func yyAgentSkillText(id, section string, offset, limit int) (string, error) {
	sources := []string{}
	full := ""
	if id == "lua-runtime" {
		if section != "" && section != "plugin_doc.md" {
			return "", fmt.Errorf("章节不存在")
		}
		full = string(pluginDevDoc)
	} else {
		skill := resolveSkill(id)
		if skill == nil {
			return "", fmt.Errorf("技能不存在")
		}
		sources = skill.Sources
	}
	if len(sources) > 0 {
		selected := sources[0]
		if section != "" {
			found := false
			for _, path := range sources {
				if section == path || section == filepath.Base(path) {
					selected = path
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("章节不存在；可选章节 %s", strings.Join(sources, ", "))
			}
		}
		data, e := aiSkillsFS.ReadFile(selected)
		if e != nil {
			return "", e
		}
		full = string(data)
	}
	runes := []rune(full)
	if offset < 0 {
		offset = 0
	}
	if offset > len(runes) {
		offset = len(runes)
	}
	if limit < 1 {
		limit = 6000
	}
	if limit > 20000 {
		limit = 20000
	}
	end := offset + limit
	if end > len(runes) {
		end = len(runes)
	}
	result := string(runes[offset:end])
	if len(sources) > 1 {
		result += "\n可用章节：" + strings.Join(sources, ", ") + "；使用 section 选择章节。"
	}
	if offset != 0 || end < len(runes) {
		result += fmt.Sprintf("\n[技能字符 %d-%d / 共 %d；用 offset=%d 继续读取。]", offset, end, len(runes), end)
	}
	return result, nil
}
