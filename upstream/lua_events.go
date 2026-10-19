package main

import (
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// diffPlayerEvents 对比当前在线玩家集合与上次快照，触发 onJoin/onLeave。
// 由玩家列表周期刷新循环调用（网易租赁服不发 AddPlayer/PlayerList，靠 /list 命令轮询）。
func (bm *BotManager) diffPlayerEvents() {
	current := playerInfoMgr.OnlineSet()
	bm.mu.Lock()
	prev := bm.lastOnlineSet
	bm.lastOnlineSet = current
	bm.mu.Unlock()
	if prev == nil {
		for name := range current {
			yyQQHelperRecord("observe", name, "")
		}
		return // 首次快照只记录
	}
	for name := range current {
		if !prev[name] {
			fireJoinEvent(bm, name)
		}
	}
	for name := range prev {
		if !current[name] {
			fireLeaveEvent(bm, name)
		}
	}
}

// startPlayerEventLoop 后台周期刷新玩家列表并做 join/leave 差量。仅在有人监听 onJoin/onLeave 时才执行。
func (bm *BotManager) startPlayerEventLoop() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-bm.stopCh:
				return
			case <-ticker.C:
				if !eventNeedsPlayerPolling() {
					continue
				}
				if playerInfoMgr.RefreshPlayerRoster(bm) {
					bm.diffPlayerEvents()
				}
			}
		}
	}()
}

// luaEventH 事件处理器：plugin 定位常驻VM，handler 是已有Lua函数名。
type luaEventH struct {
	owner   *BotManager
	plugin  string
	handler string
}

var (
	evMu       sync.Mutex
	evChat     []*luaEventH // onChat(player, msg)
	evJoin     []*luaEventH // onJoin(player)
	evLeave    []*luaEventH // onLeave(player)
	evBotReady []*luaEventH // onBotReady()
	evPacket   []*luaEventH // onPacket(type, dataTable)

	// evPacketSubs 通用数据包订阅（events.subscribe），按插件清理。
	evPacketSubs []*packetLuaSub
)

// packetLuaSub 一条通用数据包订阅：插件 → 包类型 → 处理器。
type packetLuaSub struct {
	owner   *BotManager
	plugin  string
	pktType string
	handler string
	unsub   func()
}

// registerPacketSub 处理 events.subscribe(packetType, handlerName)。
func registerPacketSub(L *lua.LState) {
	if L.GetGlobal("__RE_RUN__") == lua.LTrue {
		return // 重跑上下文跳过注册
	}
	plugin := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
	if plugin == "" {
		L.RaiseError("events.subscribe 需在插件上下文内调用")
		return
	}
	pktType := L.CheckString(1)
	handler := yySenseLuaHandler(L, 2)
	if pktType == "" || handler == "" {
		L.RaiseError("events.subscribe 需要 packetType 与 handlerName")
		return
	}
	evMu.Lock()
	for _, s := range evPacketSubs {
		if s.owner == abot(L) && s.plugin == plugin && s.pktType == pktType && s.handler == handler {
			evMu.Unlock()
			return // 已订阅，去重
		}
	}
	evMu.Unlock()

	// Bind subscriptions to their account kernel; other accounts must not trigger it.
	owner := abot(L)
	unsub := packetHubSubscribe(pktType, func(bm *BotManager, data map[string]any) {
		if bm == owner {
			yyQueueResidentEvent(bm, plugin, handler, []any{data})
		}
	})
	evMu.Lock()
	evPacketSubs = append(evPacketSubs, &packetLuaSub{owner: owner, plugin: plugin, pktType: pktType, handler: handler, unsub: unsub})
	evMu.Unlock()
}

// registerEventAPI 注册事件监听 API：events.onChat/onJoin/onLeave/onBotReady/onPacket(handlerName)。
// 事件触发时调用对应账号的常驻 LState，不重跑插件源码。
func registerEventAPI(L *lua.LState) {
	e := L.NewTable()
	L.SetGlobal("events", e)
	L.SetField(e, "onChat", L.NewFunction(func(L *lua.LState) int {
		registerEvent(&evChat, L)
		return 0
	}))
	L.SetField(e, "onJoin", L.NewFunction(func(L *lua.LState) int {
		registerEvent(&evJoin, L)
		return 0
	}))
	L.SetField(e, "onLeave", L.NewFunction(func(L *lua.LState) int {
		registerEvent(&evLeave, L)
		return 0
	}))
	L.SetField(e, "onBotReady", L.NewFunction(func(L *lua.LState) int {
		registerEvent(&evBotReady, L)
		return 0
	}))
	L.SetField(e, "onPacket", L.NewFunction(func(L *lua.LState) int {
		registerEvent(&evPacket, L)
		return 0
	}))
	// events.subscribe(packetType, handlerName)：通用数据包订阅。
	// 机器人只监听已订阅的包类型，收到后调用常驻 handler(dataTable)。
	// dataTable 是完整序列化的包数据（字段 snake_case，[]byte 为完整 hex，不截断）。
	L.SetField(e, "subscribe", L.NewFunction(func(L *lua.LState) int {
		registerPacketSub(L)
		return 0
	}))
}

// registerEvent 把当前插件的 handler 注册进事件列表（重跑上下文跳过；同插件同 handler 去重）。
func registerEvent(list *[]*luaEventH, L *lua.LState) {
	if L.GetGlobal("__RE_RUN__") == lua.LTrue {
		return
	}
	plugin := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
	handler := yySenseLuaHandler(L, 1)
	evMu.Lock()
	defer evMu.Unlock()
	for _, h := range *list {
		if h.owner == abot(L) && h.plugin == plugin && h.handler == handler {
			return
		}
	}
	*list = append(*list, &luaEventH{owner: abot(L), plugin: plugin, handler: handler})
}

// ---- 触发入口 ----

func fireChatEvent(bm *BotManager, player, msg string) {
	if bm != nil {
		yyRecordChat(bm, player, msg)
	}
	yyRuntimeEmit(bm, map[string]any{"type": "chat", "player": player, "message": msg})
	runEventHandlers(bm, evChat, player, msg)
}
func fireJoinEvent(bm *BotManager, name string) {
	yyQQAssistantEvent(bm, map[string]any{"type": "join", "player": name})
	yyRuntimeEmit(bm, map[string]any{"type": "join", "player": name})
	runEventHandlers(bm, evJoin, name)
}
func fireLeaveEvent(bm *BotManager, name string) {
	yyQQAssistantEvent(bm, map[string]any{"type": "leave", "player": name})
	yyRuntimeEmit(bm, map[string]any{"type": "leave", "player": name})
	runEventHandlers(bm, evLeave, name)
}
func fireBotReadyEvent(bm *BotManager) {
	yyRuntimeEmit(bm, map[string]any{"type": "ready"})
	yyReplayBotReady(bm, "")
}

func yyReplayBotReady(bm *BotManager, onlyPlugin string) {
	evMu.Lock()
	handlers := append([]*luaEventH(nil), evBotReady...)
	evMu.Unlock()
	for _, h := range handlers {
		if h.owner != nil && h.owner != bm {
			continue
		}
		if onlyPlugin != "" && h.plugin != onlyPlugin {
			continue
		}
		yyLuaMu.Lock()
		r := yyLuaStates[yyLuaKey{bm, h.plugin}]
		yyLuaMu.Unlock()
		if r == nil {
			continue
		} // Its later load will replay readiness.
		if _, delivered := r.botReadyDelivered.LoadOrStore(h.handler, true); delivered {
			continue
		}
		yyQueueResidentEvent(bm, h.plugin, h.handler, nil)
	}
}
func firePacketEvent(bm *BotManager, ptype string, data map[string]any) {
	yyRuntimeEmit(bm, map[string]any{"type": "packet", "packet_type": ptype, "data": data})
	runEventHandlers(bm, evPacket, ptype, data)
}

// eventNeedsPlayerPolling 是否有 onJoin/onLeave 监听（决定是否要跑玩家列表周期轮询）。
func eventNeedsPlayerPolling() bool {
	cf := yyQQSnapshot()
	if cf.Enabled && cf.Assistant.Enabled {
		return true
	}
	if yySDKNeedsPlayerPolling() {
		return true
	}
	evMu.Lock()
	defer evMu.Unlock()
	return len(evJoin) > 0 || len(evLeave) > 0
}

// RemovePluginEvents 移除某插件的全部事件监听与数据包订阅（插件停止时调用）。
func RemovePluginEvents(plugin string, owners ...*BotManager) {
	evMu.Lock()
	evChat = filterEvents(evChat, plugin, owners...)
	evJoin = filterEvents(evJoin, plugin, owners...)
	evLeave = filterEvents(evLeave, plugin, owners...)
	evBotReady = filterEvents(evBotReady, plugin, owners...)
	evPacket = filterEvents(evPacket, plugin, owners...)
	// 通用数据包订阅：先收集要退订的，再释放锁后调用（unsub 里会再抢 packetHub 锁，避免死锁）
	var toUnsub []func()
	kept := evPacketSubs[:0]
	for _, s := range evPacketSubs {
		if s.plugin == plugin && (len(owners) == 0 || s.owner == owners[0]) {
			toUnsub = append(toUnsub, s.unsub)
			continue
		}
		kept = append(kept, s)
	}
	evPacketSubs = kept
	evMu.Unlock()
	for _, u := range toUnsub {
		u()
	}
}

func filterEvents(list []*luaEventH, plugin string, owners ...*BotManager) []*luaEventH {
	out := list[:0]
	for _, h := range list {
		if h.plugin != plugin || (len(owners) > 0 && h.owner != owners[0]) {
			out = append(out, h)
		}
	}
	return out
}

// ---- 执行 ----

// runEventHandlers 拷贝事件列表后异步派发：不阻塞收包循环（consumePackets）。
// handler 在同一 goroutine 内按注册顺序串行执行；若同步执行，onChat 里调用
// game.waitMsg/promptChoice 会死锁（玩家回复需由收包循环投递，循环被自身阻塞）。
func runEventHandlers(bm *BotManager, list []*luaEventH, args ...any) {
	evMu.Lock()
	handlers := make([]*luaEventH, len(list))
	copy(handlers, list)
	evMu.Unlock()
	if len(handlers) == 0 {
		return
	}
	for _, h := range handlers {
		if h.owner != nil && h.owner != bm {
			continue
		}
		yyQueueResidentEvent(bm, h.plugin, h.handler, args)
	}
}

func yyQueueResidentEvent(bm *BotManager, id, handler string, args []any) {
	key := yyLuaKey{bm, id}
	yyLuaMu.Lock()
	if pending := yyLazyEvents[key]; pending != nil {
		select {
		case pending <- yySenseDelivery{Bot: bm, Plugin: id, Handler: handler, Args: args}:
		default:
		}
		yyLuaMu.Unlock()
		return
	}
	r := yyLuaStates[key]
	if r != nil {
		yyLuaMu.Unlock()
		yyQueueSenseLua(r, bm, id, handler, args)
		return
	}
	// Retain legacy secondary-bot lazy initialization, but create at most one
	// initializer for this VM. Late events cannot resurrect a stopped plugin.
	active := false
	for k, state := range yyLuaStates {
		if k.id == id && state != nil {
			active = true
			break
		}
	}
	if !active {
		yyLuaMu.Unlock()
		return
	}
	queue := make(chan yySenseDelivery, 128)
	queue <- yySenseDelivery{Bot: bm, Plugin: id, Handler: handler, Args: args}
	yyLazyEvents[key] = queue
	yyLuaMu.Unlock()
	go func() {
		r, e := yyGetResident(bm, id)
		for {
			yyLuaMu.Lock()
			select {
			case d := <-queue:
				yyLuaMu.Unlock()
				if e == nil {
					yyQueueSenseLua(r, d.Bot, d.Plugin, d.Handler, d.Args)
				}
			default:
				delete(yyLazyEvents, key)
				yyLuaMu.Unlock()
				return
			}
		}
	}()
}

var yyLazyEvents = map[yyLuaKey]chan yySenseDelivery{}

// callEventHandler 在临时 LState 重跑插件源码并调用 handler（事件归因到触发它的机器人 bm）。
func callEventHandler(bm *BotManager, h *luaEventH, args ...any) {
	if _, err := yyCallResident(bm, h.plugin, h.handler, args, false, false); err != nil {
		debugLog("[事件] 常驻回调失败: %v", err)
	}
}
func callPacketSubscriber(bm *BotManager, plugin, pktType, handler string, data map[string]any) {
	if _, err := yyCallResident(bm, plugin, handler, []any{data}, false, false); err != nil {
		debugLog("[订阅] 常驻回调失败: %v", err)
	}
}
