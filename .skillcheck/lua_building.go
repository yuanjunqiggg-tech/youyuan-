package main

import (
	lua "github.com/yuin/gopher-lua"

	"bot-apk/building"
	"bot-apk/media"
)

// registerBuildingAPI 注册建筑结构解析/分类 API（纯函数，无连接）。
func registerBuildingAPI(L *lua.LState) {
	b := L.NewTable()
	L.SetGlobal("building", b)

	// building.detect(path) → 格式名（.mcstructure/.schematic/.mcworld/.bdx）
	L.SetField(b, "detect", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		L.Push(lua.LString(building.DetectFormat(path)))
		return 1
	}))

	// building.parse(path) → {size:{x,y,z}, blocks, nbt, has_command}
	L.SetField(b, "parse", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		data, err := building.LoadStructureFile(path)
		if err != nil {
			L.RaiseError("building.parse: %v", err)
		}
		t := L.NewTable()
		size := L.NewTable()
		size.RawSetString("x", lua.LNumber(data.SizeX))
		size.RawSetString("y", lua.LNumber(data.SizeY))
		size.RawSetString("z", lua.LNumber(data.SizeZ))
		t.RawSetString("size", size)
		// 统计非空气方块数
		total := 0
		if data.BlocksFlat != nil {
			for _, idx := range data.BlocksFlat {
				if idx != 0 {
					total++
				}
			}
		} else {
			total = len(data.Blocks)
		}
		t.RawSetString("blocks", lua.LNumber(total))
		t.RawSetString("nbt", lua.LNumber(len(data.NBTBlocks)))
		t.RawSetString("has_command", lua.LBool(data.HasCommand))
		L.Push(t)
		return 1
	}))

	// building.isAir(name) / isWater / isLava / isCommandBlock
	L.SetField(b, "isAir", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(building.IsAirBlock(L.CheckString(1))))
		return 1
	}))
	L.SetField(b, "isWater", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(building.IsWaterBlock(L.CheckString(1))))
		return 1
	}))
	L.SetField(b, "isLava", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(building.IsLavaBlock(L.CheckString(1))))
		return 1
	}))
	L.SetField(b, "isCommandBlock", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(building.IsCommandBlock(L.CheckString(1))))
		return 1
	}))

	// building.closestColor(r, g, b) → 最近色方块名（CIELAB 匹配）
	L.SetField(b, "closestColor", L.NewFunction(func(L *lua.LState) int {
		r, g, bb := uint8(L.CheckNumber(1)), uint8(L.CheckNumber(2)), uint8(L.CheckNumber(3))
		L.Push(lua.LString(media.ClosestBlockForColor(r, g, bb)))
		return 1
	}))
}
