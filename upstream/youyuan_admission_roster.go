package main

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// /list is independent of dimension visibility and the manual observation
// filter. A names output is authoritative only after successful completion;
// unknown/empty output, duplicate names and any count mismatch fail closed.
func yyAdmissionListNames(p *packet.CommandOutput) ([]string, bool) {
	if p == nil || p.SuccessCount == 0 {
		return nil, false
	}
	text, expected := "", -1
	for _, msg := range p.OutputMessages {
		if !msg.Success {
			return nil, false
		}
		if msg.Message == "commands.players.list.names" && len(msg.Parameters) == 1 {
			if text != "" {
				return nil, false
			}
			text = msg.Parameters[0]
		}
		if msg.Message == "commands.players.list" && len(msg.Parameters) >= 1 {
			n, err := strconv.Atoi(msg.Parameters[0])
			if err != nil || n < 1 {
				return nil, false
			}
			expected = n
		}
	}
	if text == "" && p.DataSet != "" {
		var data struct {
			Players string `json:"players"`
		}
		if json.Unmarshal([]byte(p.DataSet), &data) == nil {
			text = data.Players
		}
	}
	if strings.TrimSpace(text) == "" {
		return nil, false
	}
	names, seen := []string{}, map[string]bool{}
	for _, raw := range strings.Split(text, ",") {
		name := strings.TrimSpace(raw)
		if name == "" || seen[name] || strings.ContainsAny(name, "\r\n\x00") {
			return nil, false
		}
		seen[name] = true
		names = append(names, name)
	}
	if expected >= 0 && len(names) != expected {
		return nil, false
	}
	return names, true
}
