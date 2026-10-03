package main

import (
	lua "github.com/yuin/gopher-lua"
)

// registerWorldAPI 注册世界/方块查询 API（需机器人连接，走 GetBlockTileSafe 传送加载，
// 目标为当前活跃机器人 abot(L)）。
func registerWorldAPI(L *lua.LState) {
	w := L.NewTable()
	L.SetGlobal("world", w)

	// world.block(x, y, z) → 方块名（传送加载再查询，防 outOfWorld）
	L.SetField(w, "block", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		name, err := abot(L).GetBlockTileSafe(x, y, z)
		if err != nil {
			L.RaiseError("world.block: %v", err)
		}
		L.Push(lua.LString(name))
		return 1
	}))
}
