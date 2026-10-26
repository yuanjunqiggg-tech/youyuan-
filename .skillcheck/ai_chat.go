package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// 工具循环最大迭代次数，仅作死循环安全阀（不向用户报错，超限静默结束）。
const aiMaxToolIter = 100

// aiSSE 流式事件写入器（text/event-stream）。
type aiSSE struct {
	w  http.ResponseWriter
	f  http.Flusher
	mu sync.Mutex
}

func newAISSE(w http.ResponseWriter) (*aiSSE, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	return &aiSSE{w: w, f: f}, true
}

// event 写一个 SSE 事件（JSON data）。
func (s *aiSSE) event(ev string, v any) {
	data, _ := json.Marshal(v)
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", ev, string(data))
	s.f.Flush()
}

// handleAIChat 聊天补全：上下文注入 + 工具循环 + 流式回传。
func handleAIChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelName   string `json:"model_name"`
		PluginID    string `json:"plugin_id"`
		Mode        string `json:"mode"`         // "" | "plan" | "goal"
		SessionName string `json:"session_name"` // 当前会话标题（AI 可改名）
		Messages    []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
		return
	}
	if req.ModelName == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "请先选择一个模型"})
		return
	}
	model := aiStore.Get(req.ModelName)
	if model == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "找不到模型 " + req.ModelName})
		return
	}
	if len(req.Messages) == 0 {
		writeJSON(w, map[string]any{"ok": false, "error": "没有对话内容"})
		return
	}

	// 流式响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	sse, ok := newAISSE(w)
	if !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "客户端不支持流式"})
		return
	}

	// 上下文：系统提示（含开发文档与当前插件）
	sessName := req.SessionName
	messages := []aiTurn{{Role: "system", Text: buildAISystem(req.PluginID, req.Mode, sessName)}}
	for _, m := range req.Messages {
		role := m.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		if strings.TrimSpace(m.Content) != "" {
			messages = append(messages, aiTurn{Role: role, Text: m.Content})
		}
	}
	// 确保至少有一个 user 消息
	if len(messages) == 1 {
		sse.event("ai_error", map[string]any{"error": "没有对话内容"})
		return
	}

	hooks := aiStreamHooks{
		onChunk: func(text string) {
			sse.event("ai_chunk", map[string]any{"text": text})
		},
		onThinking: func(text string) {
			sse.event("ai_thinking", map[string]any{"text": text})
		},
		onUsage: func(input, output int) {
			sse.event("ai_usage", map[string]any{"input": input, "output": output})
		},
	}
	ctx := r.Context()

	// 工具循环
	for iter := 0; iter < aiMaxToolIter; iter++ {
		turn, err := model.complete(ctx, messages, aiToolDefsAll(), hooks)
		if err != nil {
			sse.event("ai_error", map[string]any{"error": err.Error()})
			return
		}
		messages = append(messages, turn)

		if len(turn.ToolCalls) == 0 {
			sse.event("ai_done", map[string]any{"done": true})
			return
		}
		// 执行工具并回传结果
		var results []aiToolResult
		for _, tc := range turn.ToolCalls {
			if tc.Name == "user_question" {
				// 用户询问：弹窗让用户选择，答案回传模型
				res, isErr := askUser(ctx, sse, tc.Input)
				results = append(results, aiToolResult{ID: tc.ID, Content: res, IsError: isErr})
				sse.event("ai_tool_done", map[string]any{"name": tc.Name, "result": res, "is_error": isErr})
				continue
			}
			if tc.Name == "rename_session" {
				// AI 改会话标题
				newName := strings.TrimSpace(aiArgStr(tc.Input, "title"))
				if newName == "" {
					newName = strings.TrimSpace(aiArgStr(tc.Input, "name"))
				}
				res, isErr := "", true
				if newName == "" {
					res = "参数 title 不能为空"
				} else if sessName == "" {
					res = "当前没有会话可重命名"
				} else if err := sessionStore.Rename(sessName, newName); err != nil {
					res = "重命名失败: " + err.Error()
				} else {
					sessName = newName
					sse.event("ai_session_title", map[string]any{"name": newName})
					res = "已将会话标题改为: " + newName
					isErr = false
				}
				results = append(results, aiToolResult{ID: tc.ID, Content: res, IsError: isErr})
				sse.event("ai_tool_done", map[string]any{"name": tc.Name, "result": res, "is_error": isErr})
				continue
			}
			sse.event("ai_tool_start", map[string]any{"name": tc.Name, "input": tc.Input})
			res, isErr := execTool(tc.Name, tc.Input)
			results = append(results, aiToolResult{ID: tc.ID, Content: res, IsError: isErr})
			sse.event("ai_tool_done", map[string]any{"name": tc.Name, "result": res, "is_error": isErr})
		}
		messages = append(messages, aiTurn{Role: "user", ToolResults: results})
	}
	// 超过安全上限（通常不会发生）：静默结束，不向用户报错。
	sse.event("ai_done", map[string]any{"done": true})
}

// buildAISystem 构建系统提示：按正在编辑的插件注入开发文档 + 当前插件源码/配置 + 工具说明 + 工作模式。
func buildAISystem(pluginID, mode, sessionName string) string {
	var sb strings.Builder
	sb.WriteString("你是《我的世界》基岩版租赁服「游园工具箱」的插件开发助手，帮助用户编写和修改 Lua 插件或词库插件。")
	sb.WriteString("\n你可以用工具：网络请求、DuckDuckGo 网络搜索、读取/列出插件、读写插件文件、机器人连接时调用游戏内 game API。")
	sb.WriteString("\n写 Lua 代码时：调用游戏能力一律用 game.* 全局 API；要给出完整、可直接运行的代码，不要用省略号占位；")
	sb.WriteString("写完后应主动用 write_plugin_file 工具（scope=code）把代码直接保存进插件文件。")

	// 反幻觉 + 工具调用纪律：AI 常"声称已调用但没真正发起 tool_calls"，或臆想工具结果。
	sb.WriteString("\n\n【工具调用规则】调用工具必须真正发起 tool_calls，绝不能只在回复文字里写\"我已调用XX工具\"或\"调用XX后……\"而不发起实际调用——每个你声称要做的工具动作，都必须对应一条真实的 tool_calls。")
	sb.WriteString("工具执行完会把真实结果回传给你（可能是空数组 []，空结果不代表失败）。以回传的真实结果为准，不要臆想/编造工具结果。")
	sb.WriteString("不依赖先后关系的工具可以一次并行调用多个；有依赖时等前一个结果返回后再调用下一个。")
	if sessionName != "" {
		sb.WriteString("\n当前会话标题：" + sessionName + "。若你觉得这个标题不够贴切，可用 rename_session 工具改一个更贴切的标题。")
	}
	sb.WriteString("\n若用户要求用到基岩版指令/FMbe 动画等专项知识，参考下方【技能知识库】（已加载的技能）。")

	// 注入已加载技能的知识库
	if ctx := skillStore.BuildContext(); ctx != "" {
		sb.WriteString(ctx)
	}

	if pluginID != "" {
		sb.WriteString("\n\n── 当前正在编辑的插件 ──")
		sb.WriteString("\n插件 id: " + pluginID)
		if st := pluginController.status(pluginID); st != nil {
			sb.WriteString(fmt.Sprintf("\n名称: %s\n类型: %s\n描述: %s", st.Name, st.Type, st.Description))
		}
		if codeName, ok := pluginController.CodeFileName(pluginID); ok {
			if _, content, err := pluginController.ReadPluginFile("code", pluginID, ""); err == nil && strings.TrimSpace(content) != "" {
				sb.WriteString("\n\n当前插件源码（" + codeName + "）：\n" + content)
			}
		}
		if cfg, err := pluginController.ReadConfig(pluginID); err == nil && len(cfg) > 0 {
			data, _ := json.MarshalIndent(cfg, "", "  ")
			sb.WriteString("\n\n当前插件配置：\n" + string(data))
		}
		isWordbank := false
		if st := pluginController.status(pluginID); st != nil {
			isWordbank = st.Type == "wordbank"
		}
		if isWordbank {
			sb.WriteString("\n\n── 词库插件开发文档 ──\n" + string(wordbankDevDoc))
		} else {
			sb.WriteString("\n\n── Lua 插件开发文档 ──\n" + string(pluginDevDoc))
		}
	}
	if botOnline() {
		sb.WriteString("\n\n机器人已连接，game_call 工具可用，你可以直接在游戏内查询/操作。")
	} else {
		sb.WriteString("\n\n机器人当前未连接，game_call 工具会返回错误，不要依赖它。")
	}

	// 工作模式指令
	switch mode {
	case "plan":
		sb.WriteString("\n\n【当前为计划模式】先分析用户需求并给出简明方案/步骤，不要直接修改插件文件；")
		sb.WriteString("需求不清或用 user_question 让用户确认后再动手。确认后才允许写文件。")
	case "goal":
		sb.WriteString("\n\n【当前为目标模式】直接朝用户目标执行，边做边简述进展；")
		sb.WriteString("能在 1-2 步完成就不问，直接改文件并验证。")
	}
	return sb.String()
}
