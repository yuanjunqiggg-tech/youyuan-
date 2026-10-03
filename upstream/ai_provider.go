package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ── 内部统一对话/工具模型 ──────────────────────────────────────────────

// aiToolCall 模型发起的一次工具调用。
type aiToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// aiToolResult 一次工具执行的返回（供模型继续推理）。
type aiToolResult struct {
	ID      string
	Content string
	IsError bool
}

// aiTurn 一轮对话。Text 为助手/用户文本；助手可带 ToolCalls，工具结果单独以 ToolResults 表示。
// JSON tag 用 role/content，保证会话保存/读回与前端字段一致。
type aiTurn struct {
	Role         string         `json:"role"`
	Text         string         `json:"content"`
	Reasoning    string         `json:"reasoning_content,omitempty"`
	Time         string         `json:"time,omitempty"`
	ToolCalls    []aiToolCall   `json:"tool_calls,omitempty"`
	ToolResults  []aiToolResult `json:"tool_results,omitempty"`
	InputTokens  int            `json:"input_tokens,omitempty"`
	OutputTokens int            `json:"output_tokens,omitempty"`
}

// ToolDef 暴露给模型的工具定义（JSON Schema parameters）。
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// aiStreamHooks 流式回传回调（供 ai_chat 转发给前端）。
type aiStreamHooks struct {
	onChunk    func(text string)
	onToolCall func(tc aiToolCall)
	onThinking func(text string)       // 思考/推理过程增量
	onUsage    func(input, output int) // 本次请求的 token 用量
	onFinish   func(reason string)
}

// ── 流式 HTTP 工具 ────────────────────────────────────────────────────

// aiHTTPStream 发起流式 POST，逐行读取 SSE/JSON 行，逐行回调 handleLine。
// headers 附加请求头（含 Content-Type）。返回非 2xx 时附带响应体片段错误。
func aiHTTPStream(ctx context.Context, url, body string, headers map[string]string, handleLine func(line string)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 0} // 流式响应可长，超时交给 ctx / 会话
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		buf := make([]byte, 600)
		n, _ := resp.Body.Read(buf)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(buf[:n]), 300))
	}
	var sb strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		sb.WriteString(line)
		handleLine(line)
	}
	if err := sc.Err(); err != nil {
		return sb.String(), err
	}
	return sb.String(), nil
}

// AI providers may hold a non-streaming request open with keep-alive bytes while
// inference is queued. Lua HTTP calls retain their shorter timeout.
func aiHTTPPostText(ctx context.Context, url, body string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 12 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 300))
	}
	return string(data), nil
}

// ── 主入口：按模型配置发起一次补全，流式/非流式统一 ──────────────────

// 指数退避重试参数：首启+最多3次重试，间隔 2s/4s/8s。
const (
	aiRetryMaxAttempts = 3
	aiRetryBaseDelay   = 2 * time.Second
)

// complete 请求一次模型补全，带指数退避重试。返回完整助手回合（文本+工具调用），
// 同时通过 hooks 流式回传。若某次调用在报错前已输出部分流，则不重试（避免重复内容）。
func (m *AIModel) complete(ctx context.Context, messages []aiTurn, tools []ToolDef, hooks aiStreamHooks) (aiTurn, error) {
	var lastErr error
	backoff := aiRetryBaseDelay
	for attempt := 0; attempt <= aiRetryMaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return aiTurn{}, ctx.Err()
			case <-time.After(backoff):
			}
		}
		emitted := 0
		wrapped := aiStreamHooks{
			onChunk: func(t string) {
				emitted++
				if hooks.onChunk != nil {
					hooks.onChunk(t)
				}
			},
			onToolCall: func(tc aiToolCall) {
				emitted++
				if hooks.onToolCall != nil {
					hooks.onToolCall(tc)
				}
			},
			onThinking: func(t string) {
				emitted++
				if hooks.onThinking != nil {
					hooks.onThinking(t)
				}
			},
			onUsage:  hooks.onUsage,
			onFinish: hooks.onFinish,
		}
		turn, err := m.completeOnce(ctx, messages, tools, wrapped)
		if err == nil {
			return turn, nil
		}
		lastErr = err
		if emitted > 0 {
			// 已向客户端输出内容，中断重试以免重复
			return turn, err
		}
		backoff *= 2
	}
	return aiTurn{}, lastErr
}

// completeOnce 请求一次模型补全（无重试）。
func (m *AIModel) completeOnce(ctx context.Context, messages []aiTurn, tools []ToolDef, hooks aiStreamHooks) (aiTurn, error) {
	if m.Provider == ProviderAnthropic {
		return m.completeAnthropic(ctx, messages, tools, hooks)
	}
	return m.completeOpenAI(ctx, messages, tools, hooks)
}

// ── Anthropic（Claude Messages API） ──────────────────────────────────

func (m *AIModel) completeAnthropic(ctx context.Context, messages []aiTurn, tools []ToolDef, hooks aiStreamHooks) (aiTurn, error) {
	base := strings.TrimRight(m.BaseURL, "/")
	// 兼容用户填 http://x/anthropic 或 http://x/anthropic/v1：统一拼到 /v1/messages
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	url := base + "/messages"
	body, err := m.anthropicBody(messages, tools)
	if err != nil {
		return aiTurn{}, err
	}
	headers := map[string]string{
		"x-api-key":         m.APIKey,
		"anthropic-version": "2023-06-01",
	}
	if m.Streaming {
		return m.anthropicStream(ctx, url, body, headers, hooks)
	}
	// 非流式：一次请求拿完整 JSON
	text, err := aiHTTPPostText(ctx, url, body, headers)
	if err != nil {
		return aiTurn{}, err
	}
	turn, err := m.parseAnthropicResponse([]byte(text))
	if err != nil {
		return aiTurn{}, err
	}
	if hooks.onChunk != nil && turn.Text != "" {
		hooks.onChunk(turn.Text)
	}
	for _, tc := range turn.ToolCalls {
		if hooks.onToolCall != nil {
			hooks.onToolCall(tc)
		}
	}
	if hooks.onUsage != nil && (turn.InputTokens > 0 || turn.OutputTokens > 0) {
		hooks.onUsage(turn.InputTokens, turn.OutputTokens)
	}
	if hooks.onFinish != nil {
		hooks.onFinish("end")
	}
	return turn, nil
}

// anthropicBody 组装 Anthropic 请求体。
func (m *AIModel) anthropicBody(messages []aiTurn, tools []ToolDef) (string, error) {
	var system []map[string]any
	var msgs []map[string]any
	for _, t := range messages {
		switch t.Role {
		case "system":
			system = append(system, map[string]any{"type": "text", "text": t.Text})
		case "user":
			if len(t.ToolResults) > 0 {
				// 工具结果以 tool_result 内容块回传给 Claude
				var blocks []map[string]any
				for _, tr := range t.ToolResults {
					blocks = append(blocks, map[string]any{
						"type":        "tool_result",
						"tool_use_id": tr.ID,
						"content":     tr.Content,
						"is_error":    tr.IsError,
					})
				}
				msgs = append(msgs, map[string]any{"role": "user", "content": blocks})
			} else {
				msgs = append(msgs, map[string]any{"role": "user", "content": t.Text})
			}
		case "assistant":
			var blocks []map[string]any
			if t.Text != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": t.Text})
			}
			for _, tc := range t.ToolCalls {
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": tc.Input,
				})
			}
			msgs = append(msgs, map[string]any{"role": "assistant", "content": blocks})
		}
	}
	var toolObjs []map[string]any
	for _, t := range tools {
		toolObjs = append(toolObjs, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.Parameters,
		})
	}
	payload := map[string]any{
		"model":      m.ModelID,
		"max_tokens": m.outputBudget(),
		"stream":     m.Streaming,
		"messages":   msgs,
	}
	if len(system) > 0 {
		payload["system"] = system
	}
	if len(toolObjs) > 0 {
		payload["tools"] = toolObjs
	}
	if m.Thinking != ThinkingOff {
		payload["thinking"] = map[string]any{"type": "enabled", "budget_tokens": thinkingBudget[m.Thinking]}
		payload["max_tokens"] = m.outputBudget()
		budget := thinkingBudget[m.Thinking]
		if budget >= m.outputBudget() {
			budget = m.outputBudget() / 2
		}
		payload["thinking"] = map[string]any{"type": "enabled", "budget_tokens": budget}
	}
	b, err := json.Marshal(payload)
	return string(b), err
}

// parseAnthropicResponse 解析非流式 Anthropic 响应。
func (m *AIModel) parseAnthropicResponse(data []byte) (aiTurn, error) {
	var resp struct {
		Content []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return aiTurn{}, fmt.Errorf("解析 Anthropic 响应失败: %v", err)
	}
	turn := aiTurn{Role: "assistant", InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens}
	for _, c := range resp.Content {
		switch c.Type {
		case "text":
			turn.Text += c.Text
		case "tool_use":
			turn.ToolCalls = append(turn.ToolCalls, aiToolCall{ID: c.ID, Name: c.Name, Input: c.Input})
		}
	}
	return turn, nil
}

// anthropicStream 流式解析 Anthropic SSE。
func (m *AIModel) anthropicStream(ctx context.Context, url, body string, headers map[string]string, hooks aiStreamHooks) (aiTurn, error) {
	turn := aiTurn{Role: "assistant"}
	// 工具输入可能经 content_block_delta 的 input_json_delta 分片下发，按 block 累积。
	type pending struct {
		id    string
		name  string
		input strings.Builder
	}
	var pendingBlock *pending
	var inputTokens, outputTokens int
	_, err := aiHTTPStream(ctx, url, body, headers, func(line string) {
		if !strings.HasPrefix(line, "data:") {
			return
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			return
		}
		var ev struct {
			Type  string `json:"type"`
			Delta struct {
				Type           string `json:"type"`
				Text           string `json:"text"`
				ThinkingDelta  string `json:"thinking_delta"`
				PartialJSON    string `json:"partial_json"`     // Anthropic tool_use 输入增量
				InputJSONDelta string `json:"input_json_delta"` // 兼容某些代理
			} `json:"delta"`
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			ContentBlock struct {
				Type  string         `json:"type"`
				ID    string         `json:"id"`
				Name  string         `json:"name"`
				Input map[string]any `json:"input"`
			} `json:"content_block"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			return
		}
		switch ev.Type {
		case "message_start":
			inputTokens = ev.Message.Usage.InputTokens
		case "content_block_start":
			if ev.ContentBlock.Type == "tool_use" {
				pendingBlock = &pending{id: ev.ContentBlock.ID, name: ev.ContentBlock.Name}
				// 部分实现把完整 input 直接放在 start 块里
				if ev.ContentBlock.Input != nil {
					if b, err := json.Marshal(ev.ContentBlock.Input); err == nil {
						pendingBlock.input.Write(b)
					}
				}
			}
		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				turn.Text += ev.Delta.Text
				if hooks.onChunk != nil {
					hooks.onChunk(ev.Delta.Text)
				}
			case "thinking_delta":
				if hooks.onThinking != nil && ev.Delta.ThinkingDelta != "" {
					hooks.onThinking(ev.Delta.ThinkingDelta)
				}
			case "input_json_delta":
				if pendingBlock != nil {
					s := ev.Delta.PartialJSON
					if s == "" {
						s = ev.Delta.InputJSONDelta
					}
					pendingBlock.input.WriteString(s)
				}
			}
		case "content_block_stop":
			if pendingBlock != nil {
				input := map[string]any{}
				if strings.TrimSpace(pendingBlock.input.String()) != "" {
					_ = json.Unmarshal([]byte(pendingBlock.input.String()), &input)
				}
				tc := aiToolCall{ID: pendingBlock.id, Name: pendingBlock.name, Input: input}
				turn.ToolCalls = append(turn.ToolCalls, tc)
				if hooks.onToolCall != nil {
					hooks.onToolCall(tc)
				}
				pendingBlock = nil
			}
		case "message_delta":
			outputTokens = ev.Usage.OutputTokens
		case "message_stop":
			turn.InputTokens, turn.OutputTokens = inputTokens, outputTokens
			if hooks.onUsage != nil {
				hooks.onUsage(inputTokens, outputTokens)
			}
			if hooks.onFinish != nil {
				hooks.onFinish("end")
			}
		}
	})
	return turn, err
}

// ── OpenAI（及兼容系 Chat Completions） ───────────────────────────────

func (m *AIModel) completeOpenAI(ctx context.Context, messages []aiTurn, tools []ToolDef, hooks aiStreamHooks) (aiTurn, error) {
	base := strings.TrimRight(m.BaseURL, "/")
	// 用户可能填 https://x/v1 或 https://x（无 /v1）。统一拼到 /chat/completions。
	if !strings.HasSuffix(base, "/v1") && !strings.HasSuffix(base, "/chat/completions") {
		base += "/v1"
	}
	if strings.HasSuffix(base, "/chat/completions") {
		base = strings.TrimSuffix(base, "/chat/completions")
	}
	url := base + "/chat/completions"
	body, err := m.openAIBody(messages, tools)
	if err != nil {
		return aiTurn{}, err
	}
	headers := map[string]string{}
	if m.APIKey != "" {
		headers["Authorization"] = "Bearer " + m.APIKey
	}
	if m.Streaming {
		return m.openAIStream(ctx, url, body, headers, hooks)
	}
	text, err := aiHTTPPostText(ctx, url, body, headers)
	if err != nil {
		return aiTurn{}, err
	}
	turn, err := m.parseOpenAIResponse([]byte(text))
	if err != nil {
		return aiTurn{}, err
	}
	if hooks.onChunk != nil && turn.Text != "" {
		hooks.onChunk(turn.Text)
	}
	for _, tc := range turn.ToolCalls {
		if hooks.onToolCall != nil {
			hooks.onToolCall(tc)
		}
	}
	if hooks.onUsage != nil && (turn.InputTokens > 0 || turn.OutputTokens > 0) {
		hooks.onUsage(turn.InputTokens, turn.OutputTokens)
	}
	if hooks.onFinish != nil {
		hooks.onFinish("end")
	}
	return turn, nil
}

// openAIBody 组装 OpenAI 兼容请求体。
func (m *AIModel) openAIBody(messages []aiTurn, tools []ToolDef) (string, error) {
	var msgs []map[string]any
	for _, t := range messages {
		switch t.Role {
		case "system":
			msgs = append(msgs, map[string]any{"role": "system", "content": t.Text})
		case "user":
			if len(t.ToolResults) > 0 {
				// 每个工具结果一条 tool 消息
				for _, tr := range t.ToolResults {
					msgs = append(msgs, map[string]any{
						"role":         "tool",
						"tool_call_id": tr.ID,
						"content":      tr.Content,
					})
				}
			} else {
				msgs = append(msgs, map[string]any{"role": "user", "content": t.Text})
			}
		case "assistant":
			msg := map[string]any{"role": "assistant", "content": t.Text}
			if strings.Contains(strings.ToLower(m.BaseURL), "deepseek.com") {
				msg["reasoning_content"] = t.Reasoning
			}
			if len(t.ToolCalls) > 0 {
				var tcs []map[string]any
				for _, tc := range t.ToolCalls {
					args, _ := json.Marshal(tc.Input)
					tcs = append(tcs, map[string]any{
						"id":   tc.ID,
						"type": "function",
						"function": map[string]any{
							"name":      tc.Name,
							"arguments": string(args),
						},
					})
				}
				msg["tool_calls"] = tcs
			}
			msgs = append(msgs, msg)
		}
	}
	var toolObjs []map[string]any
	for _, t := range tools {
		toolObjs = append(toolObjs, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}
	payload := map[string]any{
		"model":      m.ModelID,
		"messages":   msgs,
		"stream":     m.Streaming,
		"max_tokens": m.outputBudget(),
	}
	if m.Streaming {
		// 请求流式 token 用量（部分兼容服务支持）
		payload["stream_options"] = map[string]any{"include_usage": true}
	}
	if len(toolObjs) > 0 {
		payload["tools"] = toolObjs
		payload["tool_choice"] = "auto"
	}
	if m.Thinking != ThinkingOff {
		if e, ok := thinkingEffort[m.Thinking]; ok {
			payload["reasoning_effort"] = e
		}
	}
	if strings.Contains(strings.ToLower(m.BaseURL), "deepseek.com") {
		if m.Thinking == ThinkingOff {
			payload["thinking"] = map[string]string{"type": "disabled"}
		} else {
			payload["thinking"] = map[string]string{"type": "enabled"}
			if m.Thinking == "medium" {
				payload["reasoning_effort"] = "high"
			}
		}
	}
	b, err := json.Marshal(payload)
	return string(b), err
}

// parseOpenAIResponse 解析非流式 OpenAI 兼容响应。
func (m *AIModel) parseOpenAIResponse(data []byte) (aiTurn, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return aiTurn{}, fmt.Errorf("解析 OpenAI 响应失败: %v", err)
	}
	if len(resp.Choices) == 0 {
		return aiTurn{}, fmt.Errorf("空响应")
	}
	msg := resp.Choices[0].Message
	turn := aiTurn{Role: "assistant", Text: msg.Content, Reasoning: msg.ReasoningContent, InputTokens: resp.Usage.PromptTokens, OutputTokens: resp.Usage.CompletionTokens}
	for _, tc := range msg.ToolCalls {
		input := map[string]any{}
		if strings.TrimSpace(tc.Function.Arguments) != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		}
		turn.ToolCalls = append(turn.ToolCalls, aiToolCall{ID: tc.ID, Name: tc.Function.Name, Input: input})
	}
	return turn, nil
}

// openAIStream 流式解析 OpenAI 兼容 SSE（按 tool_calls index 合并分片）。
func (m *AIModel) openAIStream(ctx context.Context, url, body string, headers map[string]string, hooks aiStreamHooks) (aiTurn, error) {
	turn := aiTurn{Role: "assistant"}
	// 流式工具分片需要按 index 累积 name/arguments。
	type partial struct {
		index int
		tc    *aiToolCall
		args  strings.Builder
	}
	var parts []partial
	var argsDone []bool
	_, err := aiHTTPStream(ctx, url, body, headers, func(line string) {
		if !strings.HasPrefix(line, "data:") {
			return
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			return
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			return
		}
		for _, ch := range ev.Choices {
			if ch.Delta.ReasoningContent != "" {
				turn.Reasoning += ch.Delta.ReasoningContent
				if hooks.onThinking != nil {
					hooks.onThinking(ch.Delta.ReasoningContent)
				}
			}
			if ch.Delta.Content != "" {
				turn.Text += ch.Delta.Content
				if hooks.onChunk != nil {
					hooks.onChunk(ch.Delta.Content)
				}
			}
			for _, tc := range ch.Delta.ToolCalls {
				idx := tc.Index
				for len(parts) <= idx {
					parts = append(parts, partial{index: len(parts)})
					argsDone = append(argsDone, false)
				}
				p := &parts[idx]
				if p.tc == nil {
					p.tc = &aiToolCall{ID: tc.ID, Name: tc.Function.Name, Input: map[string]any{}}
				}
				if tc.Function.Name != "" {
					p.tc.Name = tc.Function.Name
				}
				p.args.WriteString(tc.Function.Arguments)
				argsDone[idx] = true
			}
			if ch.FinishReason == "tool_calls" {
				// 结束时固化分片
			}
		}
		if ev.Usage.PromptTokens > 0 || ev.Usage.CompletionTokens > 0 {
			turn.InputTokens = ev.Usage.PromptTokens
			turn.OutputTokens = ev.Usage.CompletionTokens
			if hooks.onUsage != nil {
				hooks.onUsage(ev.Usage.PromptTokens, ev.Usage.CompletionTokens)
			}
		}
	})
	// 把累积的分片组装进 turn
	for _, p := range parts {
		if p.tc == nil {
			continue
		}
		if argsDone[p.index] && strings.TrimSpace(p.args.String()) != "" {
			_ = json.Unmarshal([]byte(p.args.String()), &p.tc.Input)
		}
		turn.ToolCalls = append(turn.ToolCalls, *p.tc)
		if hooks.onToolCall != nil {
			hooks.onToolCall(*p.tc)
		}
	}
	if hooks.onFinish != nil {
		hooks.onFinish("end")
	}
	return turn, err
}
