package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func yyMCPResources() []any {
	list := []any{}
	for _, entry := range [][3]string{{"youyuan://api/catalog", "API 目录", "application/json"}, {"youyuan://runtime/status", "常驻插件状态", "application/json"}, {"youyuan://accounts/status", "账号状态（令牌脱敏）", "application/json"}, {"youyuan://docs/python-runtime", "Python 开发说明", "text/markdown"}, {"youyuan://docs/cpp-runtime", "C++ 开发说明", "text/markdown"}, {"youyuan://docs/lua-runtime", "Lua 宿主说明", "text/markdown"}, {"youyuan://docs/command-chain", "命令链说明", "text/markdown"}, {"youyuan://docs/bedrock-commands", "Bedrock 指令技能", "text/markdown"}} {
		list = append(list, map[string]any{"uri": entry[0], "name": entry[1], "mimeType": entry[2]})
	}
	list = append(list,
		map[string]any{"uri": "youyuan://docs/command-chain-control", "name": "跨链调用与条件等待", "mimeType": "text/markdown"},
		map[string]any{"uri": "youyuan://docs/qq-assistant", "name": "群服互通助手", "mimeType": "text/markdown"},
		map[string]any{"uri": "youyuan://docs/self-auth", "name": "游园独立游戏认证", "mimeType": "text/markdown"},
		map[string]any{"uri": "youyuan://docs/mcp-connection", "name": "MCP接入教程", "mimeType": "text/markdown"},
		map[string]any{"uri": "youyuan://docs/public-tunnel", "name": "APK 临时公网 MCP", "mimeType": "text/markdown"},
		map[string]any{"uri": "youyuan://docs/command-chain-api", "name": "插件命令链管理", "mimeType": "text/markdown"},
		map[string]any{"uri": "youyuan://docs/physical-actions", "name": "机器人物理动作", "mimeType": "text/markdown"},
		map[string]any{"uri": "youyuan://docs/block-media", "name": "方块视频与转换", "mimeType": "text/markdown"},
		map[string]any{"uri": "youyuan://docs/command-execution", "name": "插件命令执行", "mimeType": "text/markdown"},
		map[string]any{"uri": "youyuan://docs/toolbox-live-development", "name": "运行能力与验收范围", "mimeType": "text/markdown"})
	seen := map[string]bool{}
	for _, raw := range list {
		seen[raw.(map[string]any)["uri"].(string)] = true
	}
	for _, sk := range aiSkills {
		uri := "youyuan://docs/" + sk.ID
		if !seen[uri] {
			list = append(list, map[string]any{"uri": uri, "name": sk.Name, "mimeType": "text/markdown"})
		}
	}
	list = append(list, map[string]any{"uri": "youyuan://skills/catalog", "name": "技能加载库与章节目录", "mimeType": "application/json"})
	return list
}
func yyMCPReadResource(uri string) (map[string]any, error) {
	parsed, e := url.Parse(uri)
	if e != nil {
		return nil, e
	}
	if strings.HasPrefix(uri, "youyuan://docs/") && parsed.RawQuery != "" {
		q := parsed.Query()
		id := strings.TrimPrefix(parsed.Path, "/")
		off, _ := strconv.Atoi(q.Get("offset"))
		limit, _ := strconv.Atoi(q.Get("max_chars"))
		if limit == 0 {
			limit = 3000
		}
		content, e := yyAgentSkillText(id, q.Get("section"), off, limit)
		if e != nil {
			return nil, e
		}
		return map[string]any{"uri": uri, "mimeType": "text/markdown", "text": content}, nil
	}
	content := ""
	mime := "application/json"
	switch uri {
	case "youyuan://skills/catalog":
		content = mustJSON(yyAllSkillCatalog())
	case "youyuan://docs/command-chain-control":
		data, e := aiSkillsFS.ReadFile("skills/command-chain/control-flow.md")
		if e != nil {
			return nil, e
		}
		content, mime = string(data), "text/markdown"
	case "youyuan://api/catalog":
		content = mustJSON(yyAPIRoutes)
	case "youyuan://runtime/status", "youyuan://accounts/status":
		path := "/api/youyuan/runtime/list"
		if strings.Contains(uri, "accounts") {
			path = "/api/youyuan/accounts"
		}
		status, data, e := yyInvoke("GET", path, nil, nil)
		if e != nil {
			return nil, e
		}
		if status >= 400 {
			return nil, fmt.Errorf("读取资源失败 %d", status)
		}
		content = string(data)
	default:
		if strings.HasPrefix(uri, "youyuan://docs/") {
			skill := strings.TrimPrefix(uri, "youyuan://docs/")
			mime = "text/markdown"
			if skill == "lua-runtime" {
				content = string(pluginDevDoc)
			} else if skill == "python-runtime" || skill == "cpp-runtime" || skill == "command-chain" {
				data, e := aiSkillsFS.ReadFile("skills/" + skill + "/index.md")
				if e != nil {
					return nil, e
				}
				content = string(data)
			} else if s := resolveSkill(skill); s != nil {
				for _, path := range s.Sources {
					data, e := aiSkillsFS.ReadFile(path)
					if e != nil {
						return nil, e
					}
					content += string(data) + "\n"
				}
			} else {
				return nil, fmt.Errorf("技能资源不存在")
			}
		} else if strings.HasPrefix(uri, "youyuan://account/") {
			parts := strings.SplitN(strings.TrimPrefix(uri, "youyuan://account/"), "/", 4)
			if len(parts) != 4 || parts[1] != "project" || !yyWorkspaceID(parts[0]) || !yyWorkspaceID(parts[2]) {
				return nil, fmt.Errorf("项目资源路径无效")
			}
			decoded, e := url.PathUnescape(parts[3])
			if e != nil {
				return nil, e
			}
			path := "/api/youyuan/agent/file"
			if parts[0] != "main" {
				path = yyAccountsPrefix + parts[0] + path
			}
			status, data, e := yyInvoke("GET", path, map[string]any{"id": parts[2], "path": decoded}, nil)
			if e != nil {
				return nil, e
			}
			var result map[string]any
			if status >= 400 || json.Unmarshal(data, &result) != nil || result["ok"] != true {
				return nil, fmt.Errorf("项目资源不可读取")
			}
			if result["binary"] == true {
				return map[string]any{"uri": uri, "mimeType": "application/octet-stream", "blob": result["base64"]}, nil
			}
			content, _ = result["content"].(string)
			mime = "text/plain"
		} else {
			return nil, fmt.Errorf("资源不存在")
		}
	}
	return map[string]any{"uri": uri, "mimeType": mime, "text": content}, nil
}
