package main

import (
	"encoding/json"
	"unicode/utf8"
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
	threshold := m.contextWindow() * percent / 100
	if available := m.contextWindow() - m.outputBudget() - 512; threshold > available {
		threshold = available
	}
	return threshold
}

func yyEstimateContextTokens(messages []aiTurn, tools []ToolDef) int {
	data, _ := json.Marshal(map[string]any{"messages": messages, "tools": tools})
	// Count Unicode characters plus framing conservatively. This is an estimate;
	// each provider's tokenizer and tool framing differ.
	return utf8.RuneCount(data) + 256
}
