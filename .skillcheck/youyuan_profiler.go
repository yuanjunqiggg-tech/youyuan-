package main

import (
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"net/http"
	"time"
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
	return map[string]any{"io_waiting": r.ioWaiting.Load(), "io_calls": r.ioCalls.Load(), "io_total_wait_ms": float64(r.ioTotalNS.Load()) / 1e6, "io_max_wait_ms": float64(r.ioMaxNS.Load()) / 1e6, "callbacks": r.callbackCalls.Load(), "failures": r.callbackFailures.Load(), "last_ms": float64(r.callbackLastNS.Load()) / 1e6, "max_ms": float64(r.callbackMaxNS.Load()) / 1e6, "total_ms": float64(r.callbackTotalNS.Load()) / 1e6, "interaction_queue_depth": len(r.interactionQueue), "interaction_queue_capacity": cap(r.interactionQueue), "queue_depth": len(r.senseQueue), "queue_capacity": cap(r.senseQueue), "priority_queue_depth": len(r.priorityQueue), "priority_queue_capacity": cap(r.priorityQueue), "last_queue_wait_ms": float64(r.queueLastWaitNS.Load()) / 1e6, "max_queue_wait_ms": float64(r.queueMaxWaitNS.Load()) / 1e6, "last_vm_wait_ms": float64(r.vmLastWaitNS.Load()) / 1e6, "max_vm_wait_ms": float64(r.vmMaxWaitNS.Load()) / 1e6, "last_callback_error": r.lastCallbackError.Load(), "dropped": r.senseDropped.Load(), "cpu_ms": nil, "memory_bytes": nil, "gc_ms": nil}, nil
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

// This snapshot never takes the Lua VM lock: it remains readable while a
// callback is blocked in a host call. It uses the same account boundary as SDK.
func yyProfilerRoute(mux *http.ServeMux) {
	mux.HandleFunc("/api/plugin/profiler", func(w http.ResponseWriter, r *http.Request) {
		bm := yyAccountBot(r.URL.Query().Get("account"))
		id := r.URL.Query().Get("id")
		value, e := yyLuaStats(bm, id)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		yyLuaMu.Lock()
		resident := yyLuaStates[yyLuaKey{bm, id}]
		yyLuaMu.Unlock()
		if resident != nil {
			value.(map[string]any)["last_slow_callback"] = resident.lastSlowCallback.Load()
			since := resident.activeSince.Load()
			if since != 0 {
				snapshot := value.(map[string]any)
				snapshot["active_handler"] = resident.activeHandler.Load()
				snapshot["active_ms"] = float64(time.Now().UnixNano()-since) / 1e6
			}
		}
		writeJSON(w, map[string]any{"ok": true, "stats": value})
	})
}
