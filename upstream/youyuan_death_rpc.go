package main

import (
	"fmt"
	"strconv"
)

// NetEase event shape cross-checked with ToolDelta PluginMarket's
// 前置_玩家死亡检测. Unknown payloads remain available to raw packet listeners.
// Do not search arbitrary RPC text for "death": only this exact event is used.
func yyDeathRPC(value any) (id int64, dead bool, ok bool) {
	outer, yes := value.([]any)
	if !yes || len(outer) != 3 || yyRPCString(outer[0]) != "ModEventS2C" {
		return
	}
	inner, yes := outer[1].([]any)
	if !yes || len(inner) != 4 || yyRPCString(inner[2]) != "OnPlayerDie" {
		return
	}
	data, yes := inner[3].(map[string]any)
	if !yes {
		return
	}
	dead, yes = data["die"].(bool)
	if !yes {
		return
	}
	pid, exists := data["pid"]
	if !exists {
		return
	}
	id, e := strconv.ParseInt(fmt.Sprint(pid), 10, 64)
	if e != nil {
		return 0, false, false
	}
	return id, dead, true
}
func yyRPCString(v any) string {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	s, _ := v.(string)
	return s
}

func yySenseEventMatches(kind string, e map[string]any) bool {
	if kind == "player_death" {
		return e["type"] == "death" && e["entity_type"] == "minecraft:player"
	}
	return kind == e["type"]
}
