package main

import (
	"encoding/json"
	"strings"
)

// Observe readable chat submissions only. No per-command logging in FUNCTION.
func yyReadableChatCommand(cmd string) (string, string, bool) {
	cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "/"))
	if strings.HasPrefix(cmd, "say ") {
		return "@a", strings.TrimPrefix(cmd, "say "), true
	}
	if !strings.HasPrefix(cmd, "tellraw ") {
		return "", "", false
	}
	begin := strings.Index(cmd, "{")
	if begin < 0 {
		return "", "", false
	}
	target := strings.TrimSpace(strings.TrimPrefix(cmd[:begin], "tellraw "))
	var raw struct {
		Rawtext []struct {
			Text      string `json:"text"`
			Translate string `json:"translate"`
		} `json:"rawtext"`
	}
	if json.Unmarshal([]byte(cmd[begin:]), &raw) != nil {
		return "", "", false
	}
	var text strings.Builder
	for _, part := range raw.Rawtext {
		if part.Text != "" {
			text.WriteString(part.Text)
		} else {
			text.WriteString(part.Translate)
		}
	}
	return target, text.String(), true
}
func (bm *BotManager) yyChatSubmitted(cmd string) {
	target, text, ok := yyReadableChatCommand(cmd)
	if !ok || bm == nil {
		return
	}
	message := "[CHAT_OUT][已提交] → " + target + " | " + text
	select {
	case bm.logCh <- message:
	default:
	} // display pressure never blocks send
}
