package main

import "testing"

// 验证能执行一段 Lua，并读取其返回的字符串值。
func TestRunLuaSnippet(t *testing.T) {
	out, err := runLuaSnippet(`return "hello " .. "lua"`)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if out != "hello lua" {
		t.Fatalf("输出错误: %q", out)
	}
}
