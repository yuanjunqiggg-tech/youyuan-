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
	idle                                           []*lua.LState
	closed                                         bool
	ready                                          bool
	err                                            error
	lifetime                                       context.Context
	stop                                           context.CancelFunc
	senseQueue                                     chan yySenseDelivery
	priorityQueue                                  chan yySenseDelivery
	interactionQueue                               chan yySenseDelivery
	queueLastWaitNS, queueMaxWaitNS                atomic.Int64
	senseDone                                      chan struct{}
	senseStop                                      sync.Once
	senseDropped                                   atomic.Uint64
	botReadyDelivered                              sync.Map
	callbackCalls, callbackFailures                atomic.Uint64
	callbackLastNS, callbackMaxNS, callbackTotalNS atomic.Int64
	activeHandler                                  atomic.Value
	activeSince                                    atomic.Int64
	lastSlowCallback                               atomic.Value
	lastCallbackError                              atomic.Value
	vmLastWaitNS, vmMaxWaitNS                      atomic.Int64
	ioWaiting, ioCalls, ioTotalNS, ioMaxNS         atomic.Int64
}
type yySenseDelivery struct {
	Bot             *BotManager
	Plugin, Handler string
	Args            []any
	QueuedAt        time.Time
	Interactive     bool
}

func yyQueueSenseLua(r *yyLuaResident, bm *BotManager, id, handler string, args []any, interactive ...bool) {
	select {
	case <-r.senseDone:
		return
	default:
	}
	select {
	case yySenseDeliveryQueue(r, args, interactive...) <- yySenseDelivery{Bot: bm, Plugin: id, Handler: handler, Args: args, QueuedAt: time.Now()}:
	case <-r.senseDone:
	default:
		r.senseDropped.Add(1)
	}
}
func yySenseDeliveryQueue(r *yyLuaResident, args []any, interactive ...bool) chan yySenseDelivery {
	if len(interactive) > 0 && interactive[0] && r.interactionQueue != nil {
		return r.interactionQueue
	}
	if len(args) > 0 {
		if e, ok := args[0].(map[string]any); ok {
			// Chat can wait for commands or prompts; do not hold the entity FIFO behind it.
			if r.interactionQueue != nil && (e["_packet_type"] == "Text" || e["packet_type"] == "Text" || e["type"] == "chat" || e["type"] == "text") {
				return r.interactionQueue
			}
			switch e["type"] {
			case "hurt", "health", "death", "death_details", "player_kill", "respawn", "session_join", "session_leave", "dimension_change", "equipment":
				if r.priorityQueue != nil {
					return r.priorityQueue
				}
			}
		}
	}
	return r.senseQueue
}
func (r *yyLuaResident) runSense() { r.runSenseQueue(r.senseQueue) }
func (r *yyLuaResident) runSenseQueue(queue <-chan yySenseDelivery) {
	for {
		select {
		case <-r.senseDone:
			return
		default:
		}
		select {
		case <-r.senseDone:
			return
		case d := <-queue:
			wait := time.Since(d.QueuedAt).Nanoseconds()
			r.queueLastWaitNS.Store(wait)
			for old := r.queueMaxWaitNS.Load(); wait > old && !r.queueMaxWaitNS.CompareAndSwap(old, wait); old = r.queueMaxWaitNS.Load() {
			}
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

func yyLoadResident(bm *BotManager, id, src string, secondary bool, sourceDirs ...string) error {
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
	limits := yyObservationSettings()
	r := &yyLuaResident{senseQueue: make(chan yySenseDelivery, limits.EventQueue), priorityQueue: make(chan yySenseDelivery, limits.PriorityQueue), interactionQueue: make(chan yySenseDelivery, limits.PriorityQueue), senseDone: make(chan struct{})}
	r.lifetime, r.stop = context.WithCancel(context.Background())
	r.mu.Lock()
	yyLuaStates[key] = r
	yyLuaMu.Unlock()
	defer r.mu.Unlock()
	L := newLuaState(bm)
	r.L = L
	setLuaBotCtxParent(L, bm, r.lifetime)
	L.SetGlobal("__PLUGIN_ID__", lua.LString(id))
	codeDir := ""
	if secondary {
		codeDir = getPluginSourceDir(id)
	}
	if len(sourceDirs) > 0 {
		codeDir = sourceDirs[0]
	}
	if codeDir != "" {
		L.SetGlobal("__YY_PLUGIN_CODE_DIR__", lua.LString(codeDir))
	}
	yyLuaModulePath(L)
	yyLegacyFileBindings(L)
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
		go r.runSenseQueue(r.priorityQueue)
		go r.runSenseQueue(r.interactionQueue)
		// OP readiness may precede import or hot reload. Replay after all
		// modules have defined their final callbacks, without re-running source.
		connected := false
		if bm != nil {
			bm.mu.Lock()
			connected = bm.inGame
			bm.mu.Unlock()
		}
		if connected {
			yyReplayBotReady(bm, id)
		}
	}
	return r.err
}

func yyStopResident(bm *BotManager, id string) {
	yyHudClearOwner(bm, id)
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
		// The resident lifetime is already cancelled so active work can exit.
		// Cleanup receives its own bounded context; it may stop native chains
		// without reviving the plugin or registering background work.
		if fn, ok := r.L.GetGlobal("on_stop").(*lua.LFunction); ok {
			cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			setLuaBotCtxParent(r.L, bm, cleanup)
			if err := r.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}); err != nil {
				debugLog("[插件] %s.on_stop: %v", id, err)
			}
			cancelLuaCtx(r.L)
			cancel()
		}
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
	waitStarted := time.Now()
	if nonblocking {
		if !r.mu.TryLock() {
			return nil, fmt.Errorf("目标插件正在执行或发生循环调用：%s", id)
		}
	} else {
		r.mu.Lock()
	}
	defer r.mu.Unlock()
	waitNS := time.Since(waitStarted).Nanoseconds()
	r.vmLastWaitNS.Store(waitNS)
	for old := r.vmMaxWaitNS.Load(); waitNS > old && !r.vmMaxWaitNS.CompareAndSwap(old, waitNS); old = r.vmMaxWaitNS.Load() {
	}
	if r.closed || !r.ready {
		return nil, fmt.Errorf("插件 %s 已停止或加载失败", id)
	}
	// A callback has its own stack, but shares globals/modules/upvalues with
	// the resident VM. This permits safe cooperative I/O suspension without
	// rebuilding source or concurrently executing Lua instructions.
	var L *lua.LState
	if n := len(r.idle); n > 0 {
		L = r.idle[n-1]
		r.idle = r.idle[:n-1]
	} else {
		var inheritedCancel context.CancelFunc
		L, inheritedCancel = r.L.NewThread()
		if inheritedCancel != nil {
			inheritedCancel()
		}
	}
	defer func() {
		L.SetTop(0)
		if !r.closed && len(r.idle) < 8 {
			r.idle = append(r.idle, L)
		}
	}()
	started := time.Now()
	r.activeHandler.Store(fnName)
	r.activeSince.Store(started.UnixNano())
	defer func() {
		r.activeSince.Store(0)
		ns := time.Since(started).Nanoseconds()
		if ns >= int64(100*time.Millisecond) {
			r.lastSlowCallback.Store(map[string]any{"handler": fnName, "elapsed_ms": float64(ns) / 1e6, "started_at": started.Format(time.RFC3339Nano), "finished_at": time.Now().Format(time.RFC3339Nano), "includes_io_wait": true})
		}
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
	L.SetContext(context.WithValue(L.Context(), yyLuaWaitKey{}, r))
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
		r.lastCallbackError.Store(map[string]any{"handler": fnName, "time": time.Now().Format(time.RFC3339Nano), "error": err.Error()})
		return nil, fmt.Errorf("plugin=%s handler=%s elapsed_ms=%.3f: %w", id, fnName, float64(time.Since(started))/float64(time.Millisecond), err)
	}
	if L.Get(-1) == lua.LNil {
		return nil, nil
	}
	return luaToGo(L.Get(-1)), nil
}
