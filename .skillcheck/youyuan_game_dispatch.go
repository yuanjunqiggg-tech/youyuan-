package main

import (
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"strings"
)

func yyCanonicalGameMethod(method string) string {
	method = strings.TrimSpace(method)
	if !strings.Contains(method, ".") {
		return "game." + method
	}
	return method
}

// Public calls share the actual Lua binding implementation. Stateful callbacks
// still require SDK/plugin context; a temporary binding is not a fake plugin.
func yyPublicGameCall(bm *BotManager, method string, args []any) (any, error) {
	method = yyCanonicalGameMethod(method)
	if _, ok := yyExtraMethods[method]; ok {
		return yySenseCall(bm, method, args)
	}
	switch method {
	case "game.getServerTPS", "game.getTPS", "game.serverTPS", "game.getTickRate", "game.getServerTickRate":
		if bm == nil {
			return nil, nil
		}
		value, known := bm.observedTPS()
		if !known {
			return nil, nil
		}
		return value, nil
	case "game.importBlocks", "game.importWait":
		return nil, fmt.Errorf("%s使用Lua常驻实例任务句柄；公网工具请使用建筑导入接口，不能经临时JSON调用传递userdata", method)
	case "game.sendCommand", "game.execute":
		return yyPluginCommandCall(bm, method, args, yyDefaultCommandOptions())
	case "game.sendCommandAsync", "game.commandResult", "game.registerTrigger":
		return nil, fmt.Errorf("%s需要常驻插件上下文：使用/api/youyuan/sdk/call", method)
	}
	if !strings.HasPrefix(method, "game.") {
		return nil, fmt.Errorf("API未注册：%s", method)
	}
	L := newLuaState(bm)
	defer L.Close()
	defer cancelLuaCtx(L)
	fn := L.GetField(L.GetGlobal("game"), strings.TrimPrefix(method, "game."))
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("API未注册：%s；读取/api/youyuan/sdk/catalog", method)
	}
	argv := make([]lua.LValue, len(args))
	for i, v := range args {
		argv[i] = goToLua(L, v)
	}
	if e := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, argv...); e != nil {
		return nil, e
	}
	if L.Get(-1) == lua.LNil {
		return map[string]any{"submitted": true, "confirmed": false}, nil
	}
	return luaToGo(L.Get(-1)), nil
}
