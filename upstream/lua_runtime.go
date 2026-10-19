package main

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// runLuaSnippet 执行一段 Lua 代码，返回其 return 值的字符串表示。
// 用于单测与简单脚本。
func runLuaSnippet(code string) (string, error) {
	L := newYYLuaVM()
	defer L.Close()
	if err := L.DoString(code); err != nil {
		return "", err
	}
	if v := L.Get(-1); v != lua.LNil {
		return fmt.Sprint(lua.LVAsString(v)), nil
	}
	return "", nil
}
