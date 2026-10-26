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
	L.SetField(w, "block", L.GetField(L.GetGlobal("game"), "getBlockTile"))
}
