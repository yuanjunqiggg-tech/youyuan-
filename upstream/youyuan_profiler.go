package main

import (
	"fmt"
	lua "github.com/yuin/gopher-lua"
)

func init() {
	yyExtraMethods["plugin.stats"] = "无参数，当前常驻实例回调数、失败、墙钟耗时及队列深度/丢弃；CPU/内存/GC当前未逐实例测量，返回null，不虚构性能倍数。"
}

func yyLuaStats(bm *BotManager, id string) (any, error) {
	yyLuaMu.Lock()
	r := yyLuaStates[yyLuaKey{bm, id}]
	yyLuaMu.Unlock()
	if r == nil {
		return nil, fmt.Errorf("实例未运行")
	}
	return map[string]any{"callbacks": r.callbackCalls.Load(), "failures": r.callbackFailures.Load(), "last_ms": float64(r.callbackLastNS.Load()) / 1e6, "max_ms": float64(r.callbackMaxNS.Load()) / 1e6, "total_ms": float64(r.callbackTotalNS.Load()) / 1e6, "queue_depth": len(r.senseQueue), "queue_capacity": cap(r.senseQueue), "dropped": r.senseDropped.Load(), "cpu_ms": nil, "memory_bytes": nil, "gc_ms": nil}, nil
}
func registerProfilerAPI(L *lua.LState) {
	p := L.GetGlobal("plugin").(*lua.LTable)
	L.SetField(p, "stats", L.NewFunction(func(L *lua.LState) int {
		v, e := yyLuaStats(abot(L), lua.LVAsString(L.GetGlobal("__PLUGIN_ID__")))
		if e != nil {
			L.RaiseError("%v", e)
			return 0
		}
		L.Push(goToLua(L, v))
		return 1
	}))
}
