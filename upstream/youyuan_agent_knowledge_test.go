package main

import (
	"strings"
	"testing"
)

func TestWorkspaceKnowledgePreservesToolRestrictions(t *testing.T) {
	for _, language := range []string{"python", "native_cpp", "lua", "mcfunction"} {
		ws := &yyWorkspace{Language: language, Skills: []string{"fmbe-display"}, AllowedTools: []string{"project_read"}}
		skills := yyEffectiveAgentSkills(ws)
		if len(skills) != 1 || skills[0] != "fmbe-display" {
			t.Fatalf("player selection was widened: %s %v", language, skills)
		}
		prompt := yyAgentPreferencePrompt(ws)
		for _, marker := range []string{"load_skill", "fmbe-display"} {
			if !strings.Contains(prompt, marker) {
				t.Fatalf("%s missing %s", language, marker)
			}
		}
		for _, forbidden := range []string{"auto_creative", "game.executeBatch", "examples.md", "https://"} {
			if strings.Contains(prompt, forbidden) {
				t.Fatalf("%s inlined documentation %s", language, forbidden)
			}
		}
		if len(ws.Skills) != 1 || ws.Skills[0] != "fmbe-display" {
			t.Fatal("saved customization mutated")
		}
		if yyAgentAllowed(ws, "game_call") || yyAgentAllowed(ws, "api_call") || !yyAgentAllowed(ws, "project_read") {
			t.Fatal("documentation widened tool permissions")
		}
	}
}

func TestAgentKnowledgeSections(t *testing.T) {
	control, err := yyAgentSkillText("command-chain", "control-flow.md", 0, 20000)
	if err != nil || !strings.Contains(control, "keep_open=true") || !strings.Contains(control, "父链") || !strings.Contains(control, "poll_ms") {
		t.Fatal("missing registered control skill section", err)
	}
	if _, err := yyAgentSkillText("command-chain", "../private/token.json", 0, 6000); err == nil {
		t.Fatal("unregistered chain section accepted")
	}
	for _, section := range []string{"examples.md", "verification.md"} {
		content, err := yyAgentSkillText("toolbox-live-development", section, 0, 20000)
		if err != nil || len(content) < 200 {
			t.Fatalf("section %s: %v", section, err)
		}
	}
	for _, section := range []string{"python_seed.py", "cpp_seed.cpp", "python_observer.py", "cpp_observer.cpp"} {
		content, err := yyAgentSkillText("toolbox-live-development", section, 0, 20000)
		if err != nil || !strings.Contains(content, "game") {
			t.Fatalf("missing actual example code %s: %v", section, err)
		}
	}
	if _, err := yyAgentSkillText("toolbox-live-development", "../private/qq-config.json", 0, 6000); err == nil {
		t.Fatal("unregistered section accepted")
	}
}
