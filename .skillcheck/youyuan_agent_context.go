package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Fold all selected history into the private summary. If it exceeds one
// provider request, summarize successive chunks instead of losing the tail.
func (ws *yyWorkspace) summarizeHistory(ctx context.Context, model *AIModel, previous string, turns []aiTurn) (string, error) {
	requestModel := *model
	if requestModel.outputBudget() > 2048 {
		requestModel.MaxOutputTokens = 2048
	}
	budget := requestModel.contextWindow() - requestModel.outputBudget() - 1024
	raw, err := json.Marshal(turns)
	if err != nil {
		return "", err
	}
	runes := []rune(string(raw))
	for len(runes) > 0 {
		prefix := "已有任务记忆：\n" + previous + "\n本次历史片段：\n"
		messages := func(n int) []aiTurn {
			return []aiTurn{{Role: "system", Text: "请压缩任务上下文，保留目标、已修改文件、工具真实结果、错误、未完成步骤和用户约束。不编造。合并已有记忆与本次片段，精简只返回摘要。"}, {Role: "user", Text: prefix + string(runes[:n])}}
		}
		lo, hi := 0, len(runes)
		for lo < hi {
			mid := lo + (hi-lo+1)/2
			if requestModel.contextEstimate(messages(mid), nil) <= budget {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		if lo == 0 {
			return "", fmt.Errorf("摘要请求无法容纳已有记忆；原始历史保留，请调整模型容量")
		}
		result, e := requestModel.complete(ctx, messages(lo), nil, aiStreamHooks{})
		if e != nil {
			return "", e
		}
		if strings.TrimSpace(result.Text) == "" {
			return "", fmt.Errorf("模型返回空摘要，原始历史保留")
		}
		if result.FinishReason == "length" || result.FinishReason == "max_tokens" {
			return "", fmt.Errorf("模型摘要触及输出长度上限；原始历史保留，不用截断摘要替代任务记忆")
		}
		ws.Lock()
		ws.addUsageLocked(result, model)
		ws.Unlock()
		previous = result.Text
		runes = runes[lo:]
	}
	return previous, nil
}

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
		out[i].ToolResults = append([]aiToolResult(nil), out[i].ToolResults...)
	}
	return out
}
func yyAgentSkillText(id, section string, offset, limit int) (string, error) {
	sources := []string{}
	full := ""
	if id == "lua-runtime" && section == "plugin_doc.md" {
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
			if id == "packet-reference" && !strings.Contains(section, ".") && !strings.Contains(section, "/") {
				row, e := yyDocPacket(section)
				if e != nil {
					return "", e
				}
				full, _ = row["content"].(string)
				found = true
			}
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
		if full == "" {
			data, e := aiSkillsFS.ReadFile(selected)
			if e != nil {
				return "", e
			}
			full = string(data)
		}
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
