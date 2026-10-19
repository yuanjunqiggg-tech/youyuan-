package main

import (
	lua "github.com/yuin/gopher-lua"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
)

func init() {
	yyExtraMethods["plugin.has"] = "method或runtime.lua/runtime.python/runtime.cpp，查询宿主API与运行器依赖；不代表服务器支持或已观测到血量。Linux C++为独立进程，需要Python3/YY_PYTHON及当前ABI的.so。"
	yyExtraMethods["plugin.capabilities"] = "无参数，当前平台可用API名称和语言运行器能力；按需读取，不自动整库塞AI上下文。"
}

var yyCapabilityOnce sync.Once
var yyCapabilityMethods []string

func yyCapabilityList() []string {
	yyCapabilityOnce.Do(func() {
		L := newLuaState(nil)
		defer L.Close()
		defer cancelLuaCtx(L)
		for _, name := range yySDKNames {
			if table, ok := L.GetGlobal(name).(*lua.LTable); ok {
				table.ForEach(func(k, v lua.LValue) {
					if v.Type() == lua.LTFunction {
						yyCapabilityMethods = append(yyCapabilityMethods, name+"."+k.String())
					}
				})
			}
		}
		sort.Strings(yyCapabilityMethods)
	})
	return append([]string(nil), yyCapabilityMethods...)
}
func yyLanguageCapabilities() map[string]bool {
	python := runtime.GOOS == "android"
	if !python {
		if name := os.Getenv("YY_PYTHON"); name != "" {
			_, e := exec.LookPath(name)
			python = e == nil
		} else {
			for _, name := range []string{"python3", "python"} {
				if _, e := exec.LookPath(name); e == nil {
					python = true
					break
				}
			}
		}
	}
	return map[string]bool{"runtime.lua": true, "runtime.python": python, "runtime.cpp": runtime.GOOS == "android" || (runtime.GOOS == "linux" && python)}
}
func yyHasCapability(method string) bool {
	if strings.HasPrefix(method, "runtime.") {
		return yyLanguageCapabilities()[method]
	}
	list := yyCapabilityList()
	i := sort.SearchStrings(list, method)
	return i < len(list) && list[i] == method
}
func yyCapabilities() any {
	return map[string]any{"platform": runtime.GOOS, "methods": yyCapabilityList(), "languages": yyLanguageCapabilities(), "server_support_requires_validation": true,
		"method_status": "registered binding; not proof of server support or permission", "api_registration_count_limit": nil,
		"lua_limits": map[string]any{"engine": "YouYuan wide-register fork of GopherLua 1.1.2 / Lua 5.1", "compiler_registers_per_function": lua.MaxRegisters, "instruction_word_bits": 64, "serialized_upstream_bytecode_compatible": false, "registry_initial": 5120, "registry_growth_enabled": true, "registry_max": 5120 * lua.CallStackSize, "call_stack": lua.CallStackSize}}
}
func registerCapabilityAPI(L *lua.LState) {
	p := L.GetGlobal("plugin").(*lua.LTable)
	L.SetField(p, "has", L.NewFunction(func(L *lua.LState) int { L.Push(lua.LBool(yyHasCapability(L.CheckString(1)))); return 1 }))
	L.SetField(p, "capabilities", L.NewFunction(func(L *lua.LState) int { L.Push(goToLua(L, yyCapabilities())); return 1 }))
}
