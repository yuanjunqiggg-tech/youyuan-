package main

import (
	lua "github.com/yuin/gopher-lua"
)

// registerBotAPI 注册多机器人舰队查询 API（只读）。
func registerBotAPI(L *lua.LState) {
	b := L.NewTable()
	L.SetGlobal("bot", b)

	// bot.select(nameOrIndex) → 切换后续 game.*/world.* 调用的目标机器人（舰队中任意一个）。
	L.SetField(b, "select", L.NewFunction(func(L *lua.LState) int {
		sel := L.CheckString(1)
		if selectBot(L, sel) == nil {
			L.RaiseError("bot.select: 未找到机器人 %q", sel)
		}
		L.Push(lua.LTrue)
		return 1
	}))
	// bot.current() → 当前活跃机器人 {name, x, y, z}
	L.SetField(b, "current", L.NewFunction(func(L *lua.LState) int {
		cur := abot(L)
		t := L.NewTable()
		if cur != nil {
			t.RawSetString("name", lua.LString(botName(cur)))
			if pos, err := cur.QueryPosition(); err == nil && pos != nil {
				t.RawSetString("x", lua.LNumber(numField(pos, "x")))
				t.RawSetString("y", lua.LNumber(numField(pos, "y")))
				t.RawSetString("z", lua.LNumber(numField(pos, "z")))
			}
		}
		L.Push(t)
		return 1
	}))

	// bot.count() → 在线机器人数量
	L.SetField(b, "count", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(len(GetFleetBots())))
		return 1
	}))

	// bot.list() → [{name, x, y, z}]（在线机器人及坐标）
	L.SetField(b, "list", L.NewFunction(func(L *lua.LState) int {
		t := L.NewTable()
		for i, bm := range GetFleetBots() {
			entry := L.NewTable()
			entry.RawSetString("name", lua.LString(botName(bm)))
			pos, _ := bm.QueryPosition()
			if pos != nil {
				entry.RawSetString("x", lua.LNumber(numField(pos, "x")))
				entry.RawSetString("y", lua.LNumber(numField(pos, "y")))
				entry.RawSetString("z", lua.LNumber(numField(pos, "z")))
			}
			t.RawSetInt(i+1, entry)
		}
		L.Push(t)
		return 1
	}))

	// bot.status() → [{index, name, connected, is_op, error}]（FleetStatus 只读）
	L.SetField(b, "status", L.NewFunction(func(L *lua.LState) int {
		t := L.NewTable()
		for i, s := range FleetStatus() {
			entry := L.NewTable()
			entry.RawSetString("index", lua.LNumber(s.Index))
			entry.RawSetString("name", lua.LString(s.Name))
			entry.RawSetString("connected", lua.LBool(s.Connected))
			entry.RawSetString("is_op", lua.LBool(s.IsOP))
			if s.Error != "" {
				entry.RawSetString("error", lua.LString(s.Error))
			}
			t.RawSetInt(i+1, entry)
		}
		L.Push(t)
		return 1
	}))
}

// botName 返回机器人的显示名（优先 display，其次 idStr）。
func botName(bm *BotManager) string {
	if bm == nil {
		return ""
	}
	if bm.display != "" {
		return bm.display
	}
	return bm.idStr
}

// numField 从位置 map 读取数值字段（转 float64）。
func numField(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}
