package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Opt-in provider integration; no credentials are embedded in source or APK.
func TestQQCasualChatRealProvider(t *testing.T) {
	path := os.Getenv("YY_QQ_AI_LIVE_CONFIG")
	if path == "" {
		t.Skip("real provider credentials not selected")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var settings struct {
		BaseURL string `json:"base_url"`
		Key     string `json:"api_key"`
		Model   string `json:"model_id"`
	}
	if e = json.Unmarshal(raw, &settings); e != nil {
		t.Fatal(e)
	}
	if settings.Model == "" {
		settings.Model = "deepseek-flash"
	}
	name := "qa-real-casual"
	if e = aiStore.Add(&AIModel{Name: name, Provider: ProviderOpenAI, BaseURL: settings.BaseURL, APIKey: settings.Key, ModelID: settings.Model, Thinking: "off", MaxOutputTokens: 512}); e != nil {
		t.Fatal(e)
	}
	cf := yyQQConfig{GroupID: "qa-group-a"}
	cf.Assistant.Model = name
	report := []map[string]any{}
	for _, question := range []string{"你好，今天有点累，随便聊几句。", "给我一段温和的鼓励，不需要查询游戏。"} {
		answer, e := yyQQAskAI(cf, "qa-casual-member", question)
		if e != nil {
			t.Fatal(e)
		}
		if strings.TrimSpace(answer) == "" {
			t.Fatal("empty casual reply")
		}
		yyEventsMu.Lock()
		last := yyEvents[len(yyEvents)-1]
		yyEventsMu.Unlock()
		event := last["data"].(string)
		if !strings.Contains(event, "工具0") {
			t.Fatal("casual chat used tools: " + event)
		}
		report = append(report, map[string]any{"question": question, "answer": answer, "tool_calls": 0, "usage": event})
	}
	if out := os.Getenv("YY_QQ_AI_LIVE_REPORT"); out != "" {
		data, _ := json.MarshalIndent(report, "", "  ")
		if e = os.WriteFile(out, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
