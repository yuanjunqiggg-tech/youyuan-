package main

import "testing"

// AI 工具 call_plugin_function：调用插件任意**全局**函数。
func TestCallPluginFunctionAnyGlobal(t *testing.T) {
	bm := &BotManager{}
	src := `
		function myAdd(a, b)
			return a + b
		end
	`
	if err := LoadLuaPlugin(bm, "anyFn", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	res, err := callPluginFunctionAny(bm, "anyFn", "myAdd", `[2, 3]`)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res != "5" {
		t.Fatalf("myAdd(2,3) 应=5, got %s", res)
	}
}

// 回退：不在全局、但导出在 plugin_api 的函数也能调。
func TestCallPluginFunctionAnyPluginAPI(t *testing.T) {
	bm := &BotManager{}
	src := `
		plugin_api = { greet = function(name) return "hi " .. name end }
	`
	if err := LoadLuaPlugin(bm, "anyApi", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	res, err := callPluginFunctionAny(bm, "anyApi", "greet", `["world"]`)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res != `"hi world"` {
		t.Fatalf("greet 应=hi world, got %s", res)
	}
}

// 不存在的函数报错。
func TestCallPluginFunctionAnyMissing(t *testing.T) {
	bm := &BotManager{}
	if err := LoadLuaPlugin(bm, "noFn", `function foo() return 1 end`); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if _, err := callPluginFunctionAny(bm, "noFn", "doesNotExist", ""); err == nil {
		t.Fatal("调用不存在的函数应报错")
	}
}

// 工具入口：toolCallPluginFunction 能识别参数并调用。
func TestToolCallPluginFunction(t *testing.T) {
	bm := &BotManager{}
	if err := LoadLuaPlugin(bm, "toolFn", `function double(x) return x * 2 end`); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	res, isErr := toolCallPluginFunction(map[string]any{
		"plugin_id": "toolFn",
		"func_name": "double",
		"args":      []any{21},
	})
	if isErr {
		t.Fatalf("不应报错: %s", res)
	}
	if res != "42" {
		t.Fatalf("double(21) 应=42, got %s", res)
	}
}
