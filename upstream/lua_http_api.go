package main

import (
	"net/url"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// registerHTTPAPI 注册详细网络能力到全局 http 表：
//   http.request(method, url, body?, opts?) → {ok, status, body, headers, error}
//   http.get(url, opts?) / post / put / delete
//   http.json_encode(v) / http.json_decode(s)
//   http.querify(paramsTable) → "a=1&b=2"
// opts = { headers = {..}, timeout = 秒 }
// 区别于 game.httpGet/httpPost（返回纯文本、错误即报错）：http.* 返回完整响应对象，
// 非 2xx 也返回 body，传输错误放进 error 字段而不抛异常。
func registerHTTPAPI(L *lua.LState) {
	h := L.NewTable()
	L.SetGlobal("http", h)

	// http.request(method, url, body?, opts?)
	L.SetField(h, "request", L.NewFunction(func(L *lua.LState) int {
		method := L.CheckString(1)
		u := L.CheckString(2)
		body := L.OptString(3, "")
		return pushHTTPResult(L, method, u, body, L.Get(4))
	}))
	L.SetField(h, "get", L.NewFunction(func(L *lua.LState) int {
		return pushHTTPResult(L, "GET", L.CheckString(1), "", L.Get(2))
	}))
	L.SetField(h, "post", L.NewFunction(func(L *lua.LState) int {
		return pushHTTPResult(L, "POST", L.CheckString(1), L.OptString(2, ""), L.Get(3))
	}))
	L.SetField(h, "put", L.NewFunction(func(L *lua.LState) int {
		return pushHTTPResult(L, "PUT", L.CheckString(1), L.OptString(2, ""), L.Get(3))
	}))
	L.SetField(h, "delete", L.NewFunction(func(L *lua.LState) int {
		return pushHTTPResult(L, "DELETE", L.CheckString(1), "", L.Get(2))
	}))

	L.SetField(h, "json_encode", L.NewFunction(func(L *lua.LState) int {
		s, err := jsonEncodeGo(luaToGo(L.Get(1)))
		if err != nil {
			L.RaiseError("http.json_encode: %v", err)
		}
		L.Push(lua.LString(s))
		return 1
	}))
	L.SetField(h, "json_decode", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		v, err := jsonDecodeGo(s)
		if err != nil {
			L.RaiseError("http.json_decode: %v", err)
		}
		L.Push(goToLua(L, v))
		return 1
	}))

	// http.querify({a="1", b=2}) → "a=1&b=2"
	L.SetField(h, "querify", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		vals := url.Values{}
		tbl.ForEach(func(k, v lua.LValue) {
			vals.Add(lua.LVAsString(k), lua.LVAsString(v))
		})
		L.Push(lua.LString(vals.Encode()))
		return 1
	}))
}

// pushHTTPResult 执行请求并把完整结果（含非 2xx body 与错误信息）推为 Lua 表。
func pushHTTPResult(L *lua.LState, method, u, body string, opts lua.LValue) int {
	timeout := 30 * time.Second
	headers := map[string]string{}
	if t, ok := opts.(*lua.LTable); ok {
		if hd := t.RawGetString("headers"); hd != lua.LNil {
			if ht, ok := hd.(*lua.LTable); ok {
				ht.ForEach(func(k, v lua.LValue) { headers[lua.LVAsString(k)] = lua.LVAsString(v) })
			}
		}
		if to := t.RawGetString("timeout"); to != lua.LNil {
			if n, ok := to.(lua.LNumber); ok {
				timeout = time.Duration(float64(n) * float64(time.Second))
			}
		}
	}
	status, rheaders, rbody, err := httpRequestFull(method, u, body, headers, timeout)
	out := L.NewTable()
	if err != nil {
		out.RawSetString("ok", lua.LFalse)
		out.RawSetString("error", lua.LString(err.Error()))
		L.Push(out)
		return 1
	}
	out.RawSetString("ok", lua.LTrue)
	out.RawSetString("status", lua.LNumber(status))
	out.RawSetString("body", lua.LString(rbody))
	rh := L.NewTable()
	for k, v := range rheaders {
		rh.RawSetString(k, lua.LString(v))
	}
	out.RawSetString("headers", rh)
	L.Push(out)
	return 1
}
