package main

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// registerWebAPI 注册插件网页 API 到全局 web 表：
//
//	web.page(页面名, 简介?, 控件表?)     声明式页面
//	web.serve(页面名, 入口?)             自定义 HTML 页面（www/ 目录，默认 index.html）
//	web.action(页面名, 动作名, 函数名)   交互动作
//	web.emit(页面名, 事件名, 数据)       实时推送到该插件的所有页面
//
// 注册类调用在重跑上下文（__RE_RUN__）自动跳过；web.emit 不跳过（handler 内推送用）。
func registerWebAPI(L *lua.LState) {
	w := L.NewTable()
	L.SetGlobal("web", w)

	L.SetField(w, "page", L.NewFunction(func(L *lua.LState) int {
		if L.GetGlobal("__RE_RUN__") == lua.LTrue {
			return 0
		}
		name := L.CheckString(1)
		desc := L.OptString(2, "")
		var controls []webControl
		if t := L.OptTable(3, nil); t != nil {
			controls = parseWebControls(t)
		}
		plugin := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		if err := registerWebPage(&webPage{Plugin: plugin, Name: name, Desc: desc, Kind: "declarative", Controls: controls}); err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))

	L.SetField(w, "serve", L.NewFunction(func(L *lua.LState) int {
		if L.GetGlobal("__RE_RUN__") == lua.LTrue {
			return 0
		}
		name := L.CheckString(1)
		entry := L.OptString(2, "index.html")
		plugin := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		if err := registerWebPage(&webPage{Plugin: plugin, Name: name, Kind: "html", Entry: entry}); err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))

	L.SetField(w, "action", L.NewFunction(func(L *lua.LState) int {
		if L.GetGlobal("__RE_RUN__") == lua.LTrue {
			return 0
		}
		page := L.CheckString(1)
		action := L.CheckString(2)
		handler := yySenseLuaHandler(L, 3)
		plugin := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		if err := registerWebAction(plugin, page, action, handler); err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))

	L.SetField(w, "emit", L.NewFunction(func(L *lua.LState) int {
		plugin := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		page := L.CheckString(1)
		event := L.CheckString(2)
		v := luaToGo(L.Get(3))
		// 事件名并入数据（保留键 __event）：自定义页 JS 可按事件名分发；
		// 声明式页面无同名控件，内置 JS 遍历数据键时自动忽略。
		if m, ok := v.(map[string]any); ok {
			m["__event"] = event
		}
		data, err := jsonEncodeGo(v)
		if err != nil {
			L.RaiseError("web.emit: %v", err)
		}
		pushWebEvent(plugin, page, data)
		return 0
	}))
}

// parseWebControls 把 Lua 控件表（数组，元素为字段表）转成 []webControl。
func parseWebControls(tbl *lua.LTable) []webControl {
	var out []webControl
	tbl.ForEach(func(_, v lua.LValue) {
		ct, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		c := webControl{
			Type:        lua.LVAsString(ct.RawGetString("type")),
			Key:         lua.LVAsString(ct.RawGetString("key")),
			Label:       lua.LVAsString(ct.RawGetString("label")),
			Action:      lua.LVAsString(ct.RawGetString("action")),
			Placeholder: lua.LVAsString(ct.RawGetString("placeholder")),
		}
		if cols, ok := ct.RawGetString("columns").(*lua.LTable); ok {
			cols.ForEach(func(_, v lua.LValue) { c.Columns = append(c.Columns, lua.LVAsString(v)) })
		}
		out = append(out, c)
	})
	return out
}

// callWebAction 执行一个交互动作：重跑插件源码并调用 handler。
// args 为请求带来的控件值（map），作为单个 req 表参数传入；返回值转 Go 值回传。
// 复用现有执行模型：LState 带 2 分钟超时（cancelLuaCtx）、注册类调用在重跑上下文跳过。
func callWebAction(bm *BotManager, plugin, page, action string, args map[string]any) (result any, err error) {
	if value, e, handled := yySDKWebAction(plugin, page, action, args); handled {
		return value, e
	}
	defer func() {
		if r := recover(); r != nil {
			debugLog("[Web] callWebAction panic plugin=%s page=%s action=%s err=%v", plugin, page, action, r)
			result = nil
			err = fmt.Errorf("插件 %s 动作 %s/%s 执行崩溃: %v", plugin, page, action, r)
		}
	}()
	src, ok := getPluginSource(plugin)
	if !ok {
		return nil, fmt.Errorf("插件 %s 未加载", plugin)
	}
	h, ok := lookupWebAction(plugin, page, action)
	if !ok {
		return nil, fmt.Errorf("插件 %s 未注册动作 %s/%s", plugin, page, action)
	}
	L := newYYLuaVM()
	defer L.Close()
	defer cancelLuaCtx(L)
	registerAllLuaAPI(L, bm)
	L.SetGlobal("__RE_RUN__", lua.LTrue)
	L.SetGlobal("__PLUGIN_ID__", lua.LString(plugin))
	if err := L.DoString(src); err != nil {
		return nil, fmt.Errorf("重跑插件源码失败: %v", err)
	}
	fn := L.GetGlobal(h.handler)
	if fn == lua.LNil {
		return nil, fmt.Errorf("插件 %s 未定义函数 %s", plugin, h.handler)
	}
	req := L.NewTable()
	for k, v := range args {
		req.RawSetString(k, goToLua(L, v))
	}
	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, req); err != nil {
		return nil, err
	}
	ret := L.Get(-1)
	L.Pop(1)
	return luaToGo(ret), nil
}
