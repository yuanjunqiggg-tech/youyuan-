package main

import (
	lua "github.com/yuin/gopher-lua"
)

// playerInfoToTable 把 PlayerInfo 转成 Lua 表。
func playerInfoToTable(L *lua.LState, pi *PlayerInfo) *lua.LTable {
	t := L.NewTable()
	if pi == nil {
		return t
	}
	t.RawSetString("name", lua.LString(pi.Name))
	t.RawSetString("xuid", lua.LString(pi.XUID))
	t.RawSetString("unique_id", lua.LNumber(pi.EntityID))
	t.RawSetString("is_op", lua.LBool(pi.IsOP))
	t.RawSetString("online", lua.LBool(pi.Online))
	t.RawSetString("dimension", lua.LNumber(pi.Dimension))
	t.RawSetString("x", lua.LNumber(float64(pi.Position[0])))
	t.RawSetString("y", lua.LNumber(float64(pi.Position[1])))
	t.RawSetString("z", lua.LNumber(float64(pi.Position[2])))
	if pi.HeldItem != nil {
		t.RawSetString("held_item", lua.LNumber(pi.HeldItem.NetworkID))
	}
	return t
}

// registerPlayerAPI 注册玩家数据查询 API：player.get/getXUID/getUniqueID/list/count/isOP/getInventory。
func registerPlayerAPI(L *lua.LState) {
	p := L.NewTable()
	L.SetGlobal("player", p)

	// player.get(name) → 玩家信息表（含 xuid/unique_id/is_op/坐标）
	L.SetField(p, "get", L.NewFunction(func(L *lua.LState) int {
		pi := playerInfoMgr.GetPlayer(L.CheckString(1))
		L.Push(playerInfoToTable(L, pi))
		return 1
	}))
	L.SetField(p, "getXUID", L.NewFunction(func(L *lua.LState) int {
		pi := playerInfoMgr.GetPlayer(L.CheckString(1))
		if pi == nil {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(pi.XUID))
		return 1
	}))
	L.SetField(p, "getUniqueID", L.NewFunction(func(L *lua.LState) int {
		pi := playerInfoMgr.GetPlayer(L.CheckString(1))
		if pi == nil {
			L.Push(lua.LNumber(0))
			return 1
		}
		L.Push(lua.LNumber(pi.EntityID))
		return 1
	}))
	L.SetField(p, "isOP", L.NewFunction(func(L *lua.LState) int {
		pi := playerInfoMgr.GetPlayer(L.CheckString(1))
		L.Push(lua.LBool(pi != nil && pi.IsOP))
		return 1
	}))
	// player.list() → 在线玩家表
	L.SetField(p, "list", L.NewFunction(func(L *lua.LState) int {
		t := L.NewTable()
		for i, pi := range playerInfoMgr.GetAllPlayers() {
			t.RawSetInt(i+1, playerInfoToTable(L, pi))
		}
		L.Push(t)
		return 1
	}))
	L.SetField(p, "count", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(len(playerInfoMgr.GetAllPlayers())))
		return 1
	}))
	// player.getInventory(name) → {items=[{name,count,slot,damage}]}（走活跃机器人查询）
	L.SetField(p, "getInventory", L.NewFunction(func(L *lua.LState) int {
		sel := L.CheckString(1)
		inv, err := abot(L).QueryPlayerInventory(sel)
		if err != nil {
			L.RaiseError("player.getInventory: %v", err)
		}
		items := L.NewTable()
		for i, it := range inv.Items {
			entry := L.NewTable()
			entry.RawSetString("name", lua.LString(it.Name))
			entry.RawSetString("count", lua.LNumber(it.Count))
			entry.RawSetString("slot", lua.LNumber(it.Slot))
			entry.RawSetString("damage", lua.LNumber(it.Damage))
			items.RawSetInt(i+1, entry)
		}
		t := L.NewTable()
		t.RawSetString("items", items)
		L.Push(t)
		return 1
	}))
}
