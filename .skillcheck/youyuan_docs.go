package main

import (
	"net/http"
	"strings"
)

func yyDocumentationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/docs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("language") == "" && r.URL.Query().Get("method") == "" {
			writeJSON(w, map[string]any{"ok": true, "resources": yyMCPResources(), "usage": map[string]any{"query": map[string]any{"language": "mcp-connection"}}, "note": "先选择资源；文档名称写入 query.language，不是顶层或 query.uri"})
			return
		}
		if yyStructuredDoc(w, r) {
			return
		}
		language := r.URL.Query().Get("language")
		if language == "account-pool" || language == "account-bridge" || language == "command-chain-api" || language == "physical-actions" || language == "block-media" || language == "self-auth" || language == "qq-assistant" || language == "public-tunnel" || language == "mcp-connection" {
			data, e := aiSkillsFS.ReadFile("skills/" + language + "/index.md")
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "content": string(data)})
			return
		}
		if method := r.URL.Query().Get("method"); method != "" {
			if doc, ok := yyExtraMethods[method]; ok {
				writeJSON(w, map[string]any{"ok": true, "content": "### `" + method + "`\n\n" + doc + "\n\nLua 直接用命名空间函数；Python 相同方法接收 dict/list；C++ 用 client 的对应命名空间。外部MCP的全局游戏接口通过 POST /api/youyuan/game/call {method,args,account} 调用；plugin资源/调度/stats需要 POST /api/youyuan/sdk/call {plugin_id,account,method,args} 的常驻插件上下文。"})
				return
			}
		}
		if language == "lua" || language == "lua-runtime" {
			writeJSON(w, map[string]any{"ok": true, "content": string(pluginDevDoc)})
			return
		}
		name := map[string]string{"python": "python-runtime", "native_cpp": "cpp-runtime", "mcfunction": "command-chain", "python-runtime": "python-runtime", "cpp-runtime": "cpp-runtime", "command-chain": "command-chain", "command-execution": "command-execution", "toolbox-live-development": "toolbox-live-development"}[language]
		if name == "" {
			if skill := resolveSkill(language); skill != nil {
				resource, err := yyMCPReadResource("youyuan://docs/" + skill.ID)
				if err == nil {
					writeJSON(w, map[string]any{"ok": true, "content": resource["text"]})
					return
				}
			}
			writeJSON(w, map[string]any{"ok": false, "error": "未知文档"})
			return
		}
		data, err := aiSkillsFS.ReadFile("skills/" + name + "/index.md")
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		content := string(data)
		if method := r.URL.Query().Get("method"); method != "" {
			marker := "### `" + method + "`"
			index := strings.Index(content, marker)
			if index < 0 {
				writeJSON(w, map[string]any{"ok": false, "error": "文档没有该方法"})
				return
			}
			content = content[index:]
			if end := strings.Index(content[len(marker):], "\n##"); end >= 0 {
				content = content[:len(marker)+end]
			}
		}
		writeJSON(w, map[string]any{"ok": true, "content": content})
	})
}
