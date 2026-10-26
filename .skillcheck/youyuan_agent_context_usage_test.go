package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestContextPercentAndProviderCalibration(t *testing.T) {
	// Production ds configuration: the old output-reservation clamp produced
	// 55,488, even though the configured 80% threshold is 284,800.
	large := &AIModel{ContextWindow: 356000, MaxOutputTokens: 300000, CompressionPercent: 80}
	if large.compressionThreshold() != 284800 {
		t.Fatal("large model threshold clamped by output reservation", large.compressionThreshold())
	}
	m := &AIModel{Provider: ProviderOpenAI, ModelID: "test", ContextWindow: 32768, MaxOutputTokens: 8192}
	if m.compressionThreshold() != 26214 {
		t.Fatal("80% silently lowered", m.compressionThreshold())
	}
	ws := &yyWorkspace{}
	ws.recordContextLocked(m, 1000, 400)
	if n, s := ws.contextTokensLocked(m, 2000); n != 800 || s != "provider_usage_calibrated_estimate" {
		t.Fatal(n, s)
	}
	other := *m
	other.ModelID = "different"
	if n, s := ws.contextTokensLocked(&other, 2000); n != 2000 || s != "local_estimate" {
		t.Fatal("model change reused measurement", n, s)
	}
	ws.recordContextLocked(m, 1000, 0)
	if ws.ContextMeasurement.InputTokens != 400 {
		t.Fatal("missing usage erased measurement")
	}
}

func TestAgentContextDoesNotSilentlyCutRequestedFilesOrTaskHistory(t *testing.T) {
	text := strings.Repeat("file line\n", 2200) + "IMPORTANT_LAST_LINE"
	turns := []aiTurn{{Role: "user", Text: text}, {Role: "user", ToolResults: []aiToolResult{{ID: "read", Content: text}}}}
	context := yyAgentContextTurns(turns)
	if context[0].Text != text || context[1].ToolResults[0].Content != text {
		t.Fatal("requested content silently truncated before provider request")
	}
}

func TestSummaryIncludesTailOfLargeHistory(t *testing.T) {
	requests := 0
	seenTail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		requests++
		for _, m := range b.Messages {
			if strings.Contains(m.Content, "IMPORTANT_HISTORY_TAIL") {
				seenTail = true
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "已保留任务记忆"}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 10}})
	}))
	defer server.Close()
	m := &AIModel{Provider: ProviderOpenAI, BaseURL: server.URL, Thinking: ThinkingOff, ContextWindow: 4096, MaxOutputTokens: 512}
	ws := &yyWorkspace{}
	_, err := ws.summarizeHistory(context.Background(), m, "原有目标", []aiTurn{{Role: "user", Text: strings.Repeat("source file line\n", 4000) + "IMPORTANT_HISTORY_TAIL"}})
	if err != nil || requests < 2 || !seenTail {
		t.Fatal("summary skipped later history", err, requests, seenTail)
	}
}

func TestContextEstimateIgnoresUnsentMetadata(t *testing.T) {
	m := &AIModel{Provider: ProviderOpenAI, Thinking: ThinkingOff}
	turns := []aiTurn{{Role: "assistant", Text: "ok"}}
	before := m.contextEstimate(turns, nil)
	turns[0].Time = strings.Repeat("x", 8000)
	turns[0].Reasoning = strings.Repeat("hidden", 8000)
	turns[0].InputTokens = 999999
	if after := m.contextEstimate(turns, nil); after != before {
		t.Fatal(before, after)
	}
	if n := m.contextEstimate([]aiTurn{{Role: "user", Text: strings.Repeat("return x + 1;\n", 2000)}}, nil); n >= 20000 {
		t.Fatal("still counts every ASCII character as a token", n)
	}
	m.BaseURL = "https://api.deepseek.com"
	if n := m.contextEstimate(turns, nil); n <= before {
		t.Fatal("provider-transmitted reasoning omitted")
	}
}

func TestWorkspaceCompressionStaysPrivateAndUsesUsage(t *testing.T) {
	for _, compress := range []bool{false, true} {
		t.Run(fmt.Sprint(compress), func(t *testing.T) {
			oldRoot := yyAgentRoot
			yyAgentRoot = t.TempDir()
			defer func() { yyAgentRoot = oldRoot }()
			calls, summaries := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Messages []struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"messages"`
					Max int `json:"max_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				calls++
				text := "已修改并验证。"
				if len(body.Messages) > 0 && strings.HasPrefix(body.Messages[0].Content, "请压缩任务上下文") {
					summaries++
					text = "目标：修复插件。原始历史保留。"
				} else {
					if !strings.Contains(body.Messages[0].Content, "默认用2到4个短句") {
						t.Error("concise policy missing")
					}
					if body.Max <= 0 {
						t.Error("no output capacity")
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": text}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 1200, "completion_tokens": 20}})
			}))
			defer server.Close()
			m := &AIModel{Provider: ProviderOpenAI, BaseURL: server.URL, ModelID: "fixture", Thinking: ThinkingOff, ContextWindow: 32768, MaxOutputTokens: 8192}
			ws := &yyWorkspace{ID: "test", Name: "插件修复", Language: "lua", AllowedTools: []string{}, Skills: []string{}, Turns: []aiTurn{{Role: "user", Text: strings.Repeat("read code\n", 800)}, {Role: "assistant", Text: "已经读取"}, {Role: "user", Text: "修复"}, {Role: "assistant", Text: "保留进度"}, {Role: "user", Text: "继续"}}}
			_ = os.MkdirAll(yyWorkspaceRoot(ws), 0700)
			messages := append([]aiTurn{{Role: "system", Text: yyWorkspacePrompt(ws)}}, yyAgentContextTurns(ws.Turns)...)
			input := 1000
			if compress {
				input = 27000
			}
			ws.recordContextLocked(m, m.contextEstimate(messages, nil), input)
			ws.run(context.Background(), m)
			if ws.State != "已完成" {
				t.Fatal(ws.State, ws.Error)
			}
			if compress && (summaries != 1 || ws.Summary == "" || ws.SummaryUntil == 0) {
				t.Fatal("summary not preserved", summaries, ws.SummaryUntil)
			}
			if !compress && summaries != 0 {
				t.Fatal("compressed below configured percentage")
			}
			for _, event := range ws.Events {
				if event.Kind == "summary" || strings.Contains(event.Text, "原始历史保留") {
					t.Fatal("private summary leaked into live conversation")
				}
			}
			if ws.ContextMeasurement.InputTokens != 1200 {
				t.Fatal("actual provider input usage not recorded")
			}
			if calls != 1+summaries {
				t.Fatal("unexpected repeated compression", calls, summaries)
			}
		})
	}
}
