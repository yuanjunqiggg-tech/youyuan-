package main

import (
	"encoding/json"
	"strconv"

	lua "github.com/yuin/gopher-lua"
)

// registerSystemAPI 把系统交互 API 注册为 Lua 全局表 system。
// 通过通用 call-return 桥（callAndroidSystem）经 JNI 让 Java 侧以 Shizuku ADB/root 权限
// 执行 shell 命令，实现与系统交互（发通知/连蓝牙/发信号等皆由 shell 命令覆盖）。
func registerSystemAPI(L *lua.LState) {
	s := L.NewTable()
	L.SetGlobal("system", s)

	// system.platform() → "android" / "desktop"（运行平台，非设备型号）
	L.SetField(s, "platform", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(getPlatformName()))
		return 1
	}))

	// system.call(method, paramsTable?) → any
	// 通用底层透传：method 交给 Java FloatingBridge.call 分发，paramsTable 作为参数 JSON。
	L.SetField(s, "call", L.NewFunction(func(L *lua.LState) int {
		method := L.CheckString(1)
		var params map[string]any
		if t := L.OptTable(2, nil); t != nil {
			switch v := luaToGo(t).(type) {
			case map[string]any:
				params = v
			case []any:
				// luaToGo 现在把数组表转成 []any；system.call 参数需字符串键对象，转回旧格式
				params = map[string]any{}
				for i, item := range v {
					params[strconv.Itoa(i+1)] = item
				}
			}
		}
		return callSystemToLua(L, "system.call", method, params)
	}))

	// system.shell(cmd) → table {ok, exit, stdout, stderr, uid, error}
	// 以 Shizuku(ADB/root) 权限执行一条 shell 命令。
	L.SetField(s, "shell", L.NewFunction(func(L *lua.LState) int {
		cmd := L.CheckString(1)
		return callSystemToLua(L, "system.shell", "shell", map[string]any{"command": cmd})
	}))

	// system.shizukuStatus() → table {available, granted, uid}
	L.SetField(s, "shizukuStatus", L.NewFunction(func(L *lua.LState) int {
		return callSystemToLua(L, "system.shizukuStatus", "shizukuStatus", map[string]any{})
	}))

	// system.shizukuRequest() → table {ok, requested}
	// 触发系统授权对话框，申请 Shizuku ADB 权限。
	L.SetField(s, "shizukuRequest", L.NewFunction(func(L *lua.LState) int {
		return callSystemToLua(L, "system.shizukuRequest", "shizukuRequest", map[string]any{})
	}))

	// system.vibrate(ms?) → 触发手机震动（经 Java Vibrator，不需 shell）
	L.SetField(s, "vibrate", L.NewFunction(func(L *lua.LState) int {
		ms := L.OptInt(1, 500)
		return callSystemToLua(L, "system.vibrate", "vibrate", map[string]any{"ms": ms})
	}))

	// system.notify(title, body?) → 发系统状态栏通知（经 Java NotificationManager）
	L.SetField(s, "notify", L.NewFunction(func(L *lua.LState) int {
		title := L.CheckString(1)
		body := L.OptString(2, "")
		return callSystemToLua(L, "system.notify", "notify", map[string]any{"title": title, "body": body})
	}))
}

// callSystemToLua 调 callAndroidSystem，把返回的 JSON 解码成 Lua 值推给栈顶并返回参数个数(1)。
// 桥不可用/出错时 RaiseError。
func callSystemToLua(L *lua.LState, fnName, method string, params map[string]any) int {
	pj, err := json.Marshal(params)
	if err != nil {
		L.RaiseError("%s: %v", fnName, err)
	}
	res, err := callAndroidSystem(method, string(pj))
	if err != nil {
		L.RaiseError("%s: %v", fnName, err)
	}
	if res != "" {
		if v, e := jsonDecodeGo(res); e == nil {
			L.Push(goToLua(L, v))
			return 1
		}
	}
	L.Push(lua.LNil)
	return 1
}
