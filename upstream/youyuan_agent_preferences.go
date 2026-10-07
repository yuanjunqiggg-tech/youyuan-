package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func yyDefaultAgentSkills(language string) []string {
	switch language {
	case "python":
		return []string{"bedrock-commands", "python-runtime"}
	case "native_cpp":
		return []string{"bedrock-commands", "cpp-runtime"}
	case "lua":
		return []string{"bedrock-commands", "lua-runtime"}
	default:
		return []string{"bedrock-commands", "command-chain"}
	}
}

// Players choose knowledge per workspace. An explicit empty selection stays empty.
func yyAvailableAgentSkills(language string) []string {
	result := []string{}

	for _, sk := range aiSkills {
		{
			result = append(result, sk.ID)
		}
	}
	return result
}
func yyEffectiveAgentSkills(ws *yyWorkspace) []string {
	result := []string{}
	available := map[string]bool{}
	for _, id := range yyAvailableAgentSkills(ws.Language) {
		available[id] = true
	}
	selected := ws.Skills
	if selected == nil {
		selected = yyAvailableAgentSkills(ws.Language)
	}
	for _, id := range selected {
		if available[id] {
			result = append(result, id)
			delete(available, id)
		}
	}
	return result
}
func yyDefaultAgentTools(language string) []string {
	tools := []string{}
	for _, tool := range yyAgentToolDefs() {
		tools = append(tools, tool.Name)
	}
	return tools
}
func yyWorkspaceToolDefs(ws *yyWorkspace) []ToolDef {
	ws.Lock()
	tools := ws.AllowedTools
	if tools != nil {
		tools = append([]string{}, tools...)
	}
	language := ws.Language
	ws.Unlock()
	if tools == nil {
		tools = yyDefaultAgentTools(language)
	}
	allowed := map[string]bool{}
	for _, name := range tools {
		allowed[name] = true
	}
	out := []ToolDef{}
	for _, tool := range yyAgentToolDefs() {
		if allowed[tool.Name] {
			out = append(out, tool)
		}
	}
	return out
}
func yyAgentAllowed(ws *yyWorkspace, name string) bool {
	for _, tool := range yyWorkspaceToolDefs(ws) {
		if tool.Name == name {
			return true
		}
	}
	return false
}
func yyAgentPreferencePrompt(ws *yyWorkspace) string {
	text := "\n当前工作区可按需读取的技能："
	skills := yyEffectiveAgentSkills(ws)
	text += strings.Join(skills, ", ") + "。简单任务只读取相关小节，load_skill 默认每页 3000 字；不要为简单指令遍历整个手册或反复加载同一页。\n"
	text += "新工作区默认开放当前语言适用的全部技能，这表示可按需读取而非已经读完。玩家仍可在技能库关闭任意技能；关闭后不自行启用或通过外部 MCP 绕过选择。离线章节无需联网。\n"
	if ws.Language == "mcfunction" {
		text += "你的当前任务环境是命令链。优先读取 command-chain 语法并使用 project_validate 检查；简单写链任务不必抓包或遍历手册。需要联调时可使用已开放的工具。\n"
	} else {
		text += "你是当前语言的常驻插件开发助手。依据任务按需读取语言 Skill；先确认连接和包类型，再抓包。project_publish 添加或更新当前插件；plugin_manage 可指定 plugin_id 管理其他已安装插件。api_call 可访问工具箱各功能区的真实API并读取其他工作区文件。当前工作区是项目保存位置，不是全部能力的边界。跨工作区操作应先查询实际ID，不能猜测或覆盖无关项目。高影响操作只在玩家明确要求的范围内执行。\n"
	}
	text += "工具默认全部可用，玩家关闭的除外；技能默认可按需读取，不会预先塞入全部正文。每次运行32个工具后保存检查点等待继续，连续3次失败时保留具体错误；这只控制AI任务，不限制插件命令执行。够用即停止，使用skill_search返回的offset读取相关位置，避免重复读整份文档。\n"
	if ws.CustomPrompt != "" {
		text += "工作区自定义要求：\n" + ws.CustomPrompt + "\n"
	}
	return text
}
func yyAgentPreferencesRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/agent/cloud-model", func(w http.ResponseWriter, r *http.Request) {
		key := yyAgentCloudKey()
		allowed := yyAgentCloudAllowed(r.Context(), key)
		if r.Method == http.MethodGet {
			writeJSON(w, map[string]any{"ok": true, "allowed": allowed})
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		if !allowed {
			w.WriteHeader(http.StatusForbidden)
			writeJSON(w, map[string]any{"ok": false, "error": "云端测试 AI 仅供项目测试账号使用，请在模型配置中填写自己的 API Key"})
			return
		}
		err := aiStore.Add(&AIModel{Name: "游园云端 AI", Provider: ProviderOpenAI, BaseURL: "https://toolbox.youyuanqi.dpdns.org/ai/v1", APIKey: key, ModelID: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", Thinking: ThinkingOff, Streaming: false})
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "model": "游园云端 AI"})
	})
	mux.HandleFunc("/api/youyuan/agent/preferences", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			ID     string   `json:"id"`
			Prompt string   `json:"custom_prompt"`
			Skills []string `json:"skills"`
			Tools  []string `json:"allowed_tools"`
			Model  string   `json:"model"`
		}
		id := r.URL.Query().Get("id")
		if r.Method == "POST" {
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&p); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			id = p.ID
		}
		ws, e := yyGetWorkspace(id)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		ws.Lock()
		defer ws.Unlock()
		if r.Method == "POST" {
			available := map[string]bool{}
			for _, tool := range yyAgentToolDefs() {
				available[tool.Name] = true
			}
			for _, name := range p.Tools {
				if !available[name] {
					writeJSON(w, map[string]any{"ok": false, "error": "未知工具 " + name})
					return
				}
			}
			validSkills := map[string]bool{}
			for _, id := range yyAvailableAgentSkills(ws.Language) {
				validSkills[id] = true
			}
			for _, id := range p.Skills {
				if !validSkills[id] {
					writeJSON(w, map[string]any{"ok": false, "error": "当前语言不支持技能 " + id})
					return
				}
			}
			ws.CustomPrompt = p.Prompt
			ws.Skills = append([]string{}, p.Skills...)
			ws.AllowedTools = append([]string{}, p.Tools...)
			if p.Model != "" {
				ws.Model = p.Model
			}
			if e = ws.saveLocked(); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		}
		skills := yyEffectiveAgentSkills(ws)
		tools := ws.AllowedTools
		if tools == nil {
			tools = yyDefaultAgentTools(ws.Language)
		}
		list := []map[string]string{}
		for _, tool := range yyAgentToolDefs() {
			list = append(list, map[string]string{"name": tool.Name, "description": tool.Description})
		}
		writeJSON(w, map[string]any{"ok": true, "custom_prompt": ws.CustomPrompt, "skills": skills, "allowed_tools": tools, "tools": list, "available_skills": yyAvailableAgentSkills(ws.Language), "skill_catalog": yySkillCatalog(ws.Language), "language": ws.Language, "model": ws.Model})
	})
	mux.HandleFunc("/api/youyuan/agent/model", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		var p struct {
			ID    string `json:"id"`
			Model string `json:"model"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if aiStore.Get(p.Model) == nil {
			writeJSON(w, map[string]any{"ok": false, "error": "模型未配置"})
			return
		}
		ws, e := yyGetWorkspace(p.ID)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		ws.Lock()
		defer ws.Unlock()
		previous := ws.Model
		ws.Model = p.Model
		if e = ws.saveLocked(); e != nil {
			ws.Model = previous
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "model": ws.Model})
	})
}
