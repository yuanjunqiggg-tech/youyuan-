package main

import (
	"encoding/json"
	"testing"
)

func TestDeepSeekToolTurnKeepsReasoningAndSelectedSkill(t *testing.T) {
	m := &AIModel{Provider: ProviderOpenAI, BaseURL: "https://api.deepseek.com", ModelID: "deepseek-flash", Thinking: "medium", Streaming: true}
	turns := []aiTurn{{Role: "assistant", Text: "", Reasoning: "考虑工具调用", ToolCalls: []aiToolCall{{ID: "call_1", Name: "project_read", Input: map[string]any{"path": "main.py"}}}}, {Role: "user", ToolResults: []aiToolResult{{ID: "call_1", Content: "print(1)"}}}}
	body, err := m.openAIBody(turns, nil)
	if err != nil { t.Fatal(err) }
	var payload struct {
		Messages []map[string]any `json:"messages"`
		Thinking map[string]any `json:"thinking"`
		StreamOptions map[string]any `json:"stream_options"`
	}
	if err = json.Unmarshal([]byte(body), &payload); err != nil { t.Fatal(err) }
	if payload.Messages[0]["reasoning_content"] != "考虑工具调用" || payload.Messages[1]["tool_call_id"] != "call_1" { t.Fatal("tool reasoning continuity lost") }
	if payload.Thinking["type"] != "enabled" || payload.StreamOptions["include_usage"] != true { t.Fatal("thinking or token accounting missing") }
	ws := &yyWorkspace{Language: "python", Skills: []string{"lua-runtime", "fmbe-display"}}
	for _, skill := range yyEffectiveAgentSkills(ws) { if skill == "lua-runtime" { t.Fatal("other language skill exposed") } }
}
