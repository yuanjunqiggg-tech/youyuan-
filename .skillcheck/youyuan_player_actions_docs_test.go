package main

import (
	"strings"
	"testing"
)

func TestPlayerActionsResourceAndSkillMatch(t *testing.T) {
	found := false
	for _, r := range yyMCPResources() {
		if r.(map[string]any)["uri"] == "youyuan://docs/player-actions" {
			found = true
		}
	}
	if !found {
		t.Fatal("actions resource not discoverable")
	}
	resource, err := yyMCPReadResource("youyuan://docs/player-actions")
	if err != nil {
		t.Fatal(err)
	}
	content := resource["text"].(string)
	for _, s := range []string{"observerBridge.events", "missed_swing", "attack_input", "position_input", "health_observation", "未接入", "11.881", "host.observerBridge.events", "client.invoke"} {
		if !strings.Contains(content, s) {
			t.Fatalf("missing %s", s)
		}
	}
	skill, err := yyAgentSkillText("player-actions", "index.md", 0, 16000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(skill, strings.TrimSpace(content)) {
		t.Fatal("MCP and AI skill disagree")
	}
}
