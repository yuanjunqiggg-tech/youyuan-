package main

import (
	"runtime"
	"strings"
	"testing"
)

// AI 工具 lua_call：直接调用 Lua API 函数（不经过插件）。
func TestToolLuaCallPlatform(t *testing.T) {
	res, isErr := toolLuaCall(map[string]any{"module": "system", "func": "platform"})
	if isErr {
		t.Fatalf("system.platform 不应报错: %s", res)
	}
	if !strings.Contains(res, runtime.GOOS) {
		t.Fatalf("system.platform 应返回平台名, got %s", res)
	}
}

func TestToolLuaCallUnknownModule(t *testing.T) {
	res, isErr := toolLuaCall(map[string]any{"module": "no_such_module", "func": "x"})
	if !isErr || !strings.Contains(res, "未知模块") {
		t.Fatalf("未知模块应报错, got %s (isErr=%v)", res, isErr)
	}
}

func TestToolLuaCallUnknownFunc(t *testing.T) {
	res, isErr := toolLuaCall(map[string]any{"module": "game", "func": "not_a_real_func"})
	if !isErr || !strings.Contains(res, "无函数") {
		t.Fatalf("未知函数应报错, got %s (isErr=%v)", res, isErr)
	}
}

func TestToolLuaCallMissingParams(t *testing.T) {
	res, isErr := toolLuaCall(map[string]any{})
	if !isErr || !strings.Contains(res, "不能为空") {
		t.Fatalf("缺参应报错, got %s (isErr=%v)", res, isErr)
	}
}
