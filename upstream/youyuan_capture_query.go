package main

import (
	"encoding/json"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
)

func yyCaptureQueryRows(p *packet.CommandOutput) []game_interface.TargetQueryingInfo {
	if p == nil || p.SuccessCount == 0 {
		return nil
	}
	parse := func(raw []byte) []game_interface.TargetQueryingInfo {
		var entries []json.RawMessage
		if json.Unmarshal(raw, &entries) != nil {
			return nil
		}
		out := []game_interface.TargetQueryingInfo{}
		for _, entry := range entries {
			var required struct {
				Dimension *uint8 `json:"dimension"`
				ID        *int64 `json:"id"`
				UUID      string `json:"uniqueId"`
			}
			if json.Unmarshal(entry, &required) != nil || required.Dimension == nil || !yyDimensionIDValid(int32(*required.Dimension)) || required.ID == nil && required.UUID == "" {
				continue
			}
			var v game_interface.TargetQueryingInfo
			if json.Unmarshal(entry, &v) == nil {
				out = append(out, v)
			}
		}
		return out
	}
	for _, msg := range p.OutputMessages {
		for _, value := range msg.Parameters {
			if rows := parse([]byte(value)); len(rows) > 0 {
				return rows
			}
		}
	}
	var dataset struct {
		Details json.RawMessage `json:"details"`
	}
	if json.Unmarshal([]byte(p.DataSet), &dataset) == nil {
		raw := dataset.Details
		var text string
		if json.Unmarshal(raw, &text) == nil {
			raw = []byte(text)
		}
		return parse(raw)
	}
	return nil
}
