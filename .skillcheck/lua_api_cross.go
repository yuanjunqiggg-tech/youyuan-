package main

import (
	"encoding/json"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// pluginSources 记录已加载插件的源码标识；跨插件调用复用常驻 Runtime，不重跑源码。
var (
	pluginSourcesMu  sync.Mutex
	pluginSources    = map[string]string{}
	pluginSourceDirs = map[string]string{}
)

// rememberPluginSource 记录插件源码，用于加载状态与兼容接口查询。
func rememberPluginSource(id, src string, sourceDirs ...string) {
	if id == "" {
		return
	}
	pluginSourcesMu.Lock()
	defer pluginSourcesMu.Unlock()
	pluginSources[id] = src
	delete(pluginSourceDirs, id)
	if len(sourceDirs) > 0 {
		pluginSourceDirs[id] = sourceDirs[0]
	}
}

func getPluginSourceDir(id string) string {
	pluginSourcesMu.Lock()
	defer pluginSourcesMu.Unlock()
	return pluginSourceDirs[id]
}

// getPluginSource 取插件源码。
func getPluginSource(id string) (string, bool) {
	pluginSourcesMu.Lock()
	defer pluginSourcesMu.Unlock()
	s, ok := pluginSources[id]
	return s, ok
}

// registerCrossPluginAPI 注册跨插件调用与插件上下文 API：
//   - api.require(id) → 目标插件 plugin_api 表的懒生成代理（调用常驻实例）
//   - plugin.id / plugin.dependencies（插件自身上下文，读 __PLUGIN_ID__ / plugin_dependencies）
func registerCrossPluginAPI(L *lua.LState, bm *BotManager) {
	api := L.NewTable()
	L.SetGlobal("api", api)
	L.SetField(api, "require", L.NewFunction(func(L *lua.LState) int {
		id := L.CheckString(1)
		if _, ok := getPluginSource(id); !ok && yySDKPluginContext(abot(L), id) == nil {
			L.Push(lua.LNil)
			return 1
		}
		// 返回带 __index 的表：访问任意 key 时懒生成该 key 的调用 stub
		tbl := L.NewTable()
		mt := L.NewTable()
		mt.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
			key := lua.LVAsString(L.CheckAny(2))
			L.Push(newCrossPluginStub(L, bm, id, key))
			return 1
		}))
		L.SetMetatable(tbl, mt)
		L.Push(tbl)
		return 1
	}))

	plugin := L.NewTable()
	L.SetGlobal("plugin", plugin)
	L.SetField(plugin, "id", L.NewFunction(func(L *lua.LState) int {
		L.Push(L.GetGlobal("__PLUGIN_ID__"))
		return 1
	}))
	L.SetField(plugin, "dependencies", L.NewFunction(func(L *lua.LState) int {
		L.Push(L.GetGlobal("plugin_dependencies"))
		return 1
	}))
}

// newCrossPluginStub 返回一个 Lua 函数：序列化参数 → callPluginFunction → 结果回传调用方。
func newCrossPluginStub(L *lua.LState, bm *BotManager, pluginID, funcName string) *lua.LFunction {
	return L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		args := make([]any, 0, n)
		for i := 1; i <= n; i++ {
			args = append(args, luaToGo(L.Get(i)))
		}
		active := abot(L)
		caller := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		account := "main"
		if ctx := yySDKPluginContext(active, pluginID); ctx != nil {
			account = ctx.Instance.Account
		}
		if caller != "" {
			release, e := yyEnterPluginCall(account+":"+caller, account+":"+pluginID)
			if e != nil {
				L.RaiseError("%v", e)
				return 0
			}
			defer release()
		}
		if yySDKPluginContext(active, pluginID) == nil {
			result, e := yyCallResident(active, pluginID, funcName, args, true, caller == "")
			if e != nil {
				L.RaiseError("api.require(%q).%s: %v", pluginID, funcName, e)
				return 0
			}
			L.Push(goToLua(L, result))
			return 1
		}
		argsJSON, err := json.Marshal(args)
		if err != nil {
			L.RaiseError("api 跨插件调用参数序列化失败: %v", err)
		}
		res, err := callPluginFunction(active, pluginID, funcName, string(argsJSON))
		if err != nil {
			L.RaiseError("api.require(%q).%s: %v", pluginID, funcName, err)
		}
		if res != "" {
			if v, e := jsonDecodeGo(res); e == nil {
				L.Push(goToLua(L, v))
				return 1
			}
		}
		L.Push(lua.LNil)
		return 1
	})
}

// callPluginFunction 调用目标常驻实例的 plugin_api[funcName]。
// JSON 是此兼容入口的参数/结果格式；目标插件须已经加载。
func callPluginFunction(bm *BotManager, pluginID, funcName, argsJSON string) (string, error) {
	if text, e, handled := yySDKExport(bm, pluginID, funcName, argsJSON); handled {
		return text, e
	}
	var args []any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
	}
	result, err := yyCallResident(bm, pluginID, funcName, args, true, true)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	out, err := json.Marshal(result)
	return string(out), err
}

// callPluginFunctionAny 供 AI 工具调用插件**任意**函数（调试用）。
// 解析顺序：全局作用域优先（绝大多数插件函数是全局 function xxx() end），
// 再回退到 plugin_api 导出表。与 callPluginFunction 的差异在解析更宽松。
func callPluginFunctionAny(bm *BotManager, pluginID, funcName, argsJSON string) (string, error) {
	if text, e, handled := yySDKExport(bm, pluginID, funcName, argsJSON); handled {
		return text, e
	}
	var args []any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
	}
	result, err := yyCallResident(bm, pluginID, funcName, args, false, true)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	out, err := json.Marshal(result)
	return string(out), err
}
