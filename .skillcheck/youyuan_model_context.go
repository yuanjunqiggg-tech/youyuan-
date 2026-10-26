package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"unicode"
)

// Conservative fallback, not a claim about a vendor's advertised context limit.
func (m *AIModel) contextWindow() int {
	if m.ContextWindow > 0 {
		return m.ContextWindow
	}
	return 32768
}
func (m *AIModel) outputBudget() int {
	if m.MaxOutputTokens > 0 {
		return m.MaxOutputTokens
	}
	budget := 8192
	if budget > m.contextWindow()/4 {
		budget = m.contextWindow() / 4
	}
	return budget
}
func (m *AIModel) compressionThreshold() int {
	percent := m.CompressionPercent
	if percent == 0 {
		percent = 80
	}
	return m.contextWindow() * percent / 100
}

func yyEstimateContextTokens(messages []aiTurn, tools []ToolDef) int {
	return (&AIModel{Provider: ProviderOpenAI, Thinking: ThinkingOff}).contextEstimate(messages, tools)
}

// Estimate fields actually sent to the provider, excluding timestamps and
// stored usage. Tokenization varies by model; real input usage calibrates it.
func (m *AIModel) contextEstimate(messages []aiTurn, tools []ToolDef) int {
	var body string
	if m.Provider == ProviderAnthropic {
		body, _ = m.anthropicBody(messages, tools)
	} else {
		body, _ = m.openAIBody(messages, tools)
	}
	var payload map[string]json.RawMessage
	_ = json.Unmarshal([]byte(body), &payload)
	data, _ := json.Marshal(map[string]json.RawMessage{"messages": payload["messages"], "system": payload["system"], "tools": payload["tools"]})
	units := 0
	for _, r := range string(data) {
		if r < 128 {
			units++
		} else if unicode.Is(unicode.Han, r) {
			units += 6
		} else {
			units += 8
		}
	}
	return (units+3)/4 + 32 + len(messages)*4
}

type yyContextMeasurement struct {
	Model           string `json:"model"`
	InputTokens     int    `json:"input_tokens"`
	RequestEstimate int    `json:"request_estimate"`
}

func (m *AIModel) contextIdentity() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(m.Provider+"\x00"+m.BaseURL+"\x00"+m.ModelID+"\x00"+m.Thinking)))
}
func (ws *yyWorkspace) contextTokensLocked(m *AIModel, estimate int) (int, string) {
	r := ws.ContextMeasurement
	if r.Model == m.contextIdentity() && r.InputTokens > 0 && r.RequestEstimate > 0 {
		return int((int64(estimate)*int64(r.InputTokens) + int64(r.RequestEstimate) - 1) / int64(r.RequestEstimate)), "provider_usage_calibrated_estimate"
	}
	return estimate, "local_estimate"
}
func (ws *yyWorkspace) recordContextLocked(m *AIModel, estimate, input int) {
	if input > 0 {
		ws.ContextMeasurement = yyContextMeasurement{Model: m.contextIdentity(), InputTokens: input, RequestEstimate: estimate}
	}
}
