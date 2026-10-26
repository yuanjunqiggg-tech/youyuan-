package main

import (
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"math"
	"sort"
	"strings"
	"time"
)

var yySchedulerDocs = map[string]string{
	"util.scheduleHz":      "hz,handler；本地完整回调的频率，有限正数，无人为1000Hz上限。慢回调不重入、不补积压轮次；查看listJobs的missed。请求亚毫秒频率会提高CPU占用，实际完成频率受回调/系统影响；不是服务端同步Tick。",
	"util.scheduleSeconds": "seconds,handler；schedule别名，正数秒，可小于0.001秒；必须能表示为正的宿主时间间隔，最长一年。",
	"util.scheduleTick":    "ticks,handler；ticks>=1，统一宿主Tick Loop；本地名义Tick=50ms，不宣称服务端同Tick同步。",
	"util.listJobs":        "无参数，当前插件任务列表：id/handler/paused/calls/missed/failures/last_ms/max_ms/total_ms/last_error；耗时为墙钟含VM排队及回调，不是CPU时间。",
	"util.pauseJob":        "id，暂停该插件任务后续触发；已经执行/入队的一次不强制中断。",
	"util.resumeJob":       "id，恢复后续触发；不会补发暂停期间的积压轮次。",
}

func init() {
	for m, d := range yySchedulerDocs {
		yyExtraMethods[m] = d
	}
}

func yyScheduleDuration(method string, value any) (time.Duration, error) {
	n, ok := value.(float64)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("需要有限的频率/间隔")
	}
	switch method {
	case "util.scheduleHz":
		if n <= 0 {
			return 0, fmt.Errorf("Hz需有限正数")
		}
		n = 1 / n
	case "util.scheduleTick":
		if n < 1 || n != math.Trunc(n) {
			return 0, fmt.Errorf("ticks需整数且至少1")
		}
		n *= .05
	}
	if n <= 0 || n > maxSchedIntervalSec || n*float64(time.Second) < 1 {
		return 0, fmt.Errorf("间隔需正数且可表示为宿主时间，最长一年")
	}
	return time.Duration(n * float64(time.Second)), nil
}

func yySchedulerControl(owner string, bm *BotManager, method string, args []any) (any, error) {
	schedMu.Lock()
	defer schedMu.Unlock()
	if method == "util.listJobs" {
		ids := []int64{}
		for id, j := range schedJobs {
			if j.plugin == owner && j.bm == bm {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		rows := []map[string]any{}
		for _, id := range ids {
			j := schedJobs[id]
			last := j.lastError.Load()
			if last == nil {
				last = ""
			}
			rows = append(rows, map[string]any{"id": id, "handler": j.handler, "paused": j.paused.Load(), "interval_ms": float64(j.interval) / float64(time.Millisecond), "calls": j.calls.Load(), "missed": j.missed.Load(), "failures": j.failures.Load(), "last_ms": float64(j.lastNS.Load()) / 1e6, "max_ms": float64(j.maxNS.Load()) / 1e6, "total_ms": float64(j.totalNS.Load()) / 1e6, "last_error": last})
		}
		return rows, nil
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("需要任务id")
	}
	id, ok := args[0].(float64)
	if !ok || id != math.Trunc(id) {
		return nil, fmt.Errorf("任务id需要整数")
	}
	j := schedJobs[int64(id)]
	if j == nil || j.plugin != owner || j.bm != bm {
		return false, nil
	}
	j.paused.Store(method == "util.pauseJob")
	if method == "util.resumeJob" {
		select {
		case j.resume <- struct{}{}:
		default:
		}
	}
	return true, nil
}

func registerSchedulerAPI(L *lua.LState) {
	u := L.GetGlobal("util").(*lua.LTable)
	for method := range yySchedulerDocs {
		method := method
		L.SetField(u, strings.TrimPrefix(method, "util."), L.NewFunction(func(L *lua.LState) int {
			id := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
			if id == "" {
				L.RaiseError("需要插件上下文")
				return 0
			}
			if strings.HasPrefix(method, "util.schedule") {
				if L.GetGlobal("__RE_RUN__") == lua.LTrue {
					L.Push(lua.LNumber(0))
					return 1
				}
				d, e := yyScheduleDuration(method, float64(L.CheckNumber(1)))
				if e != nil {
					L.RaiseError("%v", e)
					return 0
				}
				handler := ""
				if fn, ok := L.Get(2).(*lua.LFunction); ok {
					schedMu.Lock()
					seq := schedNext + 1
					schedMu.Unlock()
					handler = fmt.Sprintf("__yy_tick_callback_%d", seq)
					L.SetGlobal(handler, fn)
				} else {
					handler = L.CheckString(2)
				}
				timer := newSchedJob(id, handler, d, false, schedBot(L))
				L.Push(lua.LNumber(timer))
				return 1
			}
			args := []any{}
			for i := 1; i <= L.GetTop(); i++ {
				args = append(args, luaToGo(L.Get(i)))
			}
			v, e := yySchedulerControl(id, schedBot(L), method, args)
			if e != nil {
				L.RaiseError("%v", e)
				return 0
			}
			L.Push(goToLua(L, v))
			return 1
		}))
	}
}
