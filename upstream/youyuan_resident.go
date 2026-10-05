package main

import (
	"context"
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 每个机器人、每个插件持有一个常驻状态。事件只调用已编译函数，不重跑源码。
type yyLuaKey struct {
	bot *BotManager
	id  string
}
type yyLuaResident struct {
	mu                                             sync.Mutex
	L                                              *lua.LState
	closed                                         bool
	ready                                          bool
	err                                            error
	lifetime                                       context.Context
	stop                                           context.CancelFunc
	senseQueue                                     chan yySenseDelivery
	senseDone                                      chan struct{}
	senseStop                                      sync.Once
	senseDropped                                   atomic.Uint64
	callbackCalls, callbackFailures                atomic.Uint64
	callbackLastNS, callbackMaxNS, callbackTotalNS atomic.Int64
}
type yySenseDelivery struct {
	Bot             *BotManager
	Plugin, Handler string
	Args            []any
}

func yyQueueSenseLua(r *yyLuaResident, bm *BotManager, id, handler string, args []any) {
	select {
	case <-r.senseDone:
		return
	default:
	}
	select {
	case r.senseQueue <- yySenseDelivery{bm, id, handler, args}:
	case <-r.senseDone:
	default:
		r.senseDropped.Add(1)
	}
}
func (r *yyLuaResident) runSense() {
	for {
		select {
		case <-r.senseDone:
			return
		default:
		}
		select {
		case <-r.senseDone:
			return
		case d := <-r.senseQueue:
			if _, e := yyCallResidentState(d.Bot, d.Plugin, d.Handler, d.Args, false, false, r); e != nil {
				debugLog("[感知] %s.%s: %v", d.Plugin, d.Handler, e)
			}
		}
	}
}

var yyLuaMu sync.Mutex
var yyLuaStates = map[yyLuaKey]*yyLuaResident{}

func yyGetResident(bm *BotManager, id string) (*yyLuaResident, error) {
	key := yyLuaKey{bm, id}
	yyLuaMu.Lock()
	r := yyLuaStates[key]
	yyLuaMu.Unlock()
	if r != nil {
		return r, nil
	}
	src, ok := getPluginSource(id)
	if !ok {
		return nil, fmt.Errorf("插件 %q 未加载", id)
	}
	if err := yyLoadResident(bm, id, src, true); err != nil {
		return nil, err
	}
	yyLuaMu.Lock()
	r = yyLuaStates[key]
	yyLuaMu.Unlock()
	return r, nil
}

func yyLoadResident(bm *BotManager, id, src string, secondary bool) error {
	key := yyLuaKey{bm, id}
	yyLuaMu.Lock()
	if previous := yyLuaStates[key]; previous != nil {
		yyLuaMu.Unlock()
		if secondary {
			return nil
		}
		yyStopResident(bm, id)
		yyLuaMu.Lock()
	}
	r := &yyLuaResident{senseQueue: make(chan yySenseDelivery, 128), senseDone: make(chan struct{})}
	r.lifetime, r.stop = context.WithCancel(context.Background())
	r.mu.Lock()
	yyLuaStates[key] = r
	yyLuaMu.Unlock()
	defer r.mu.Unlock()
	L := newLuaState(bm)
	r.L = L
	setLuaBotCtxParent(L, bm, r.lifetime)
	L.SetGlobal("__PLUGIN_ID__", lua.LString(id))
	yyLuaModulePath(L)
	if pluginController != nil {
		if config, err := pluginController.ReadConfig(id); err == nil {
			if raw, ok := config["command_options"]; ok {
				options, err := yyMergeCommandOptions(yyDefaultCommandOptions(), raw)
				if err != nil {
					L.Close()
					r.closed = true
					r.err = err
					return err
				}
				L.SetGlobal("__YY_COMMAND_OPTIONS__", goToLua(L, yyCommandOptionsMap(options)))
			}
		}
	}
	if secondary {
		L.SetGlobal("__RE_RUN__", lua.LTrue)
	} else {
		L.SetGlobal("__PLUGIN_SRC__", lua.LString(src))
	}
	fn, e := L.Load(strings.NewReader(src), id+".lua")
	if e == nil {
		L.Push(fn)
		e = L.PCall(0, lua.MultRet, nil)
	}
	r.err = e
	cancelLuaCtx(L)
	r.ready = r.err == nil
	if r.err != nil {
		yySenseLuaClear(bm, id)
		r.senseStop.Do(func() { close(r.senseDone) })
		L.Close()
		r.closed = true
	} else {
		go r.runSense()
	}
	return r.err
}

func yyStopResident(bm *BotManager, id string) {
	yyClearCommandTickets(yyAsyncCommandKey{bm, id})
	yySenseLuaClear(bm, id)
	key := yyLuaKey{bm, id}
	yyLuaMu.Lock()
	r := yyLuaStates[key]
	delete(yyLuaStates, key)
	yyLuaMu.Unlock()
	if r == nil {
		return
	}
	if r.stop != nil {
		r.stop()
	}
	r.senseStop.Do(func() {
		if r.senseDone != nil {
			close(r.senseDone)
		}
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed && r.L != nil {
		cancelLuaCtx(r.L)
		r.L.Close()
		r.closed = true
	}
}

func yyCallResident(bm *BotManager, id, fnName string, args []any, exported, nonblocking bool) (any, error) {
	r, err := yyGetResident(bm, id)
	if err != nil {
		return nil, err
	}
	return yyCallResidentState(bm, id, fnName, args, exported, nonblocking, r)
}
func yyCallResidentState(bm *BotManager, id, fnName string, args []any, exported, nonblocking bool, r *yyLuaResident) (any, error) {
	if nonblocking {
		if !r.mu.TryLock() {
			return nil, fmt.Errorf("目标插件正在执行或发生循环调用：%s", id)
		}
	} else {
		r.mu.Lock()
	}
	defer r.mu.Unlock()
	if r.closed || !r.ready {
		return nil, fmt.Errorf("插件 %s 已停止或加载失败", id)
	}
	L := r.L
	started := time.Now()
	defer func() {
		ns := time.Since(started).Nanoseconds()
		r.callbackCalls.Add(1)
		r.callbackLastNS.Store(ns)
		r.callbackTotalNS.Add(ns)
		for old := r.callbackMaxNS.Load(); ns > old && !r.callbackMaxNS.CompareAndSwap(old, ns); old = r.callbackMaxNS.Load() {
		}
	}()
	parent := r.lifetime
	if parent == nil {
		parent = context.Background()
	}
	setLuaBotCtxParent(L, bm, parent)
	defer cancelLuaCtx(L)
	top := L.GetTop()
	defer L.SetTop(top)
	fn := L.GetGlobal(fnName)
	if exported {
		fn = lua.LNil
	}
	if fn == lua.LNil {
		api := L.GetGlobal("plugin_api")
		if api != lua.LNil {
			fn = L.GetField(api, fnName)
		}
	}
	if fn == lua.LNil {
		return nil, fmt.Errorf("插件 %s 没有函数 %s", id, fnName)
	}
	values := make([]lua.LValue, 0, len(args))
	for _, v := range args {
		values = append(values, goToLua(L, v))
	}
	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, values...); err != nil {
		r.callbackFailures.Add(1)
		return nil, fmt.Errorf("plugin=%s handler=%s elapsed_ms=%.3f: %w", id, fnName, float64(time.Since(started))/float64(time.Millisecond), err)
	}
	if L.Get(-1) == lua.LNil {
		return nil, nil
	}
	return luaToGo(L.Get(-1)), nil
}
