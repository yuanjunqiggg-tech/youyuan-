package main

// Shared plugin API adapter. Python/C++ code stays in its resident runtime;
// one persistent binding state per instance reuses the existing Go game API.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

var yySDKNames = []string{"game", "system", "api", "plugin", "util", "bot", "building", "media", "world", "player", "events", "http", "web", "chains", "packet", "storage", "accounts"}

func yySDKWebID(key string) string {
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("sdk_%x", hash[:12])
}

type yySDKListener struct {
	Kind, Handler, Packet string
	Words                 []string
}
type yySDKContext struct {
	Mu                  sync.Mutex
	BindingMu           sync.Mutex
	Instance            *yyRuntimeInstance
	Dir                 string
	L                   *lua.LState
	Listeners           []yySDKListener
	Timers              map[int64]chan struct{}
	Unsubs              []func()
	Next                int64
	Closed              bool
	CommandOptions      yyCommandOptions
	CommandOptionsError error
}

var yySDKContexts = struct {
	sync.Mutex
	Items map[string]*yySDKContext
}{Items: map[string]*yySDKContext{}}
var yySDKCalls = struct {
	sync.Mutex
	Edges map[string]string
}{Edges: map[string]string{}}

func yySDKPrepare(instance *yyRuntimeInstance, dir string) {
	yySDKContexts.Lock()
	defer yySDKContexts.Unlock()
	s := &yySDKContext{Instance: instance, Dir: filepath.Join(dir, "data", instance.Account), Timers: map[int64]chan struct{}{}, CommandOptions: yyDefaultCommandOptions()}
	if data, err := os.ReadFile(filepath.Join(dir, "command-options.json")); err == nil {
		var options map[string]any
		s.CommandOptionsError = json.Unmarshal(data, &options)
		if s.CommandOptionsError == nil {
			s.CommandOptions, s.CommandOptionsError = yyMergeCommandOptions(s.CommandOptions, options)
		}
	}
	yySDKContexts.Items[instance.Key] = s
}
func yySDKGet(key string) *yySDKContext {
	yySDKContexts.Lock()
	defer yySDKContexts.Unlock()
	return yySDKContexts.Items[key]
}
func yySDKClose(key string) {
	if s := yySDKGet(key); s != nil {
		yyClearCommandTickets(yyAsyncCommandKey{yyAccountBot(s.Instance.Account), key})
	}
	yySDKContexts.Lock()
	s := yySDKContexts.Items[key]
	delete(yySDKContexts.Items, key)
	yySDKContexts.Unlock()
	if s == nil {
		return
	}
	s.Mu.Lock()
	s.Closed = true
	for id := range s.Timers {
		cancelSchedJob(id)
		delete(s.Timers, id)
	}
	unsubs := s.Unsubs
	s.Unsubs = nil
	s.Listeners = nil
	s.Mu.Unlock()
	for _, u := range unsubs {
		u()
	}
	RemovePluginWeb(yySDKWebID(key))
	s.BindingMu.Lock()
	if s.L != nil {
		cancelLuaCtx(s.L)
		s.L.Close()
		s.L = nil
	}
	s.BindingMu.Unlock()
}
func (s *yySDKContext) enqueue(handler string, args []any) {
	s.Mu.Lock()
	closed := s.Closed
	s.Mu.Unlock()
	if closed {
		return
	}
	event := map[string]any{"type": "sdk_callback", "handler": handler, "args": args}
	select {
	case s.Instance.Queue <- event:
	case <-s.Instance.Done:
	default:
		n := s.Instance.Dropped.Add(1)
		if n&(n-1) == 0 {
			yyEvent("plugin_error", map[string]any{"instance": s.Instance.Key, "error": "SDK callback queue full", "dropped": n})
		}
	}
}
func yySDKEmit(bm *BotManager, event map[string]any) {
	yySDKContexts.Lock()
	list := make([]*yySDKContext, 0, len(yySDKContexts.Items))
	for _, s := range yySDKContexts.Items {
		list = append(list, s)
	}
	yySDKContexts.Unlock()
	for _, s := range list {
		if yyAccountBot(s.Instance.Account) != bm {
			continue
		}
		s.Mu.Lock()
		hooks := append([]yySDKListener(nil), s.Listeners...)
		s.Mu.Unlock()
		kind, _ := event["type"].(string)
		for _, h := range hooks {
			switch h.Kind {
			case "trigger":
				if kind != "chat" {
					continue
				}
				text, _ := event["message"].(string)
				for _, word := range h.Words {
					if text == word || strings.HasPrefix(text, word+" ") {
						args := []any{}
						for _, x := range strings.Fields(strings.TrimPrefix(text, word)) {
							args = append(args, x)
						}
						s.enqueue(h.Handler, []any{event["player"], args})
						break
					}
				}
			case "chat":
				if kind == "chat" {
					s.enqueue(h.Handler, []any{event["player"], event["message"]})
				}
			case "join", "leave":
				if kind == h.Kind {
					s.enqueue(h.Handler, []any{event["player"]})
				}
			case "ready":
				if kind == "ready" {
					s.enqueue(h.Handler, []any{})
				}
			case "packet":
				if kind == "packet" {
					s.enqueue(h.Handler, []any{event["packet_type"], event["data"]})
				}
			default:
				if yySenseEventMatches(h.Kind, event) && (kind == "death" || kind == "health" || kind == "hurt" || kind == "move" || kind == "respawn" || kind == "block_change" || kind == "swing" || kind == "entity_spawn" || kind == "entity_remove" || kind == "item_pickup") {
					s.enqueue(h.Handler, []any{event})
				}
			}
		}
	}
}
func yySDKNeedsPlayerPolling() bool {
	yyRuntimeMu.Lock()
	for _, i := range yyRuntimeInstances {
		for _, event := range i.Manifest.Events {
			if event == "join" || event == "leave" {
				yyRuntimeMu.Unlock()
				return true
			}
		}
		if len(i.Manifest.Events) == 0 {
			yyRuntimeMu.Unlock()
			return true
		}
	}
	yyRuntimeMu.Unlock()
	yySDKContexts.Lock()
	list := []*yySDKContext{}
	for _, s := range yySDKContexts.Items {
		list = append(list, s)
	}
	yySDKContexts.Unlock()
	for _, s := range list {
		s.Mu.Lock()
		for _, h := range s.Listeners {
			if h.Kind == "join" || h.Kind == "leave" {
				s.Mu.Unlock()
				return true
			}
		}
		s.Mu.Unlock()
	}
	return false
}
func yySDKRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/sdk/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var p struct {
			ID      string `json:"plugin_id"`
			Account string `json:"account"`
			Calls   []struct {
				Method string `json:"method"`
				Args   []any  `json:"args"`
			} `json:"calls"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if p.Account == "" {
			p.Account = "main"
		}
		s := yySDKGet(p.Account + ":" + p.ID)
		if s == nil || len(p.Calls) == 0 || len(p.Calls) > 64 {
			writeJSON(w, map[string]any{"ok": false, "error": "实例未运行或调用数不在 1..64"})
			return
		}
		values := []any{}
		for _, call := range p.Calls {
			v, e := s.call(call.Method, call.Args)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error(), "completed": len(values), "results": values, "scope": "previous successful operations are not rolled back"})
				return
			}
			values = append(values, v)
		}
		writeJSON(w, map[string]any{"ok": true, "results": values})
	})
	mux.HandleFunc("/api/youyuan/sdk/pages", func(w http.ResponseWriter, r *http.Request) {
		out := []map[string]any{}
		for _, p := range listWebPages() {
			yySDKContexts.Lock()
			for key, s := range yySDKContexts.Items {
				if p.Plugin == yySDKWebID(key) {
					out = append(out, map[string]any{"plugin_id": s.Instance.Manifest.ID, "account": s.Instance.Account, "page": p})
				}
			}
			yySDKContexts.Unlock()
		}
		writeJSON(w, map[string]any{"ok": true, "pages": out})
	})
	mux.HandleFunc("/api/youyuan/sdk/catalog", func(w http.ResponseWriter, r *http.Request) {
		L := newLuaState(nil)
		defer L.Close()
		defer cancelLuaCtx(L)
		out := map[string][]string{}
		for _, name := range yySDKNames {
			if t, ok := L.GetGlobal(name).(*lua.LTable); ok {
				t.ForEach(func(k, v lua.LValue) {
					if v.Type() == lua.LTFunction {
						out[name] = append(out[name], k.String())
					}
				})
				sort.Strings(out[name])
			}
		}
		out["api"] = []string{"call", "require"}
		out["game"] = append(out["game"], "execute")
		out["web"] = append(out["web"], "invoke")
		out["plugin"] = append(out["plugin"], "state", "save")
		writeJSON(w, map[string]any{"ok": true, "sdk_version": "1.15", "namespaces": out, "event_callback": "sdk_callback", "binding_lifetime": "one per resident plugin instance", "game_execution": "requires connected account and server permissions"})
	})
	mux.HandleFunc("/api/youyuan/sdk/call", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var p struct {
			ID      string `json:"plugin_id"`
			Account string `json:"account"`
			Method  string `json:"method"`
			Args    []any  `json:"args"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
		if err := decoder.Decode(&p); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if p.Account == "" {
			p.Account = "main"
		}
		s := yySDKGet(p.Account + ":" + p.ID)
		if s == nil {
			writeJSON(w, map[string]any{"ok": false, "error": "插件实例未启动"})
			return
		}
		value, err := s.call(p.Method, p.Args)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "result": value})
	})
}
func yySDKString(args []any, index int) (string, error) {
	if index >= len(args) {
		return "", fmt.Errorf("缺少第 %d 个参数", index+1)
	}
	s, ok := args[index].(string)
	if !ok || s == "" {
		return "", fmt.Errorf("第 %d 个参数必须是非空字符串", index+1)
	}
	return s, nil
}
func (s *yySDKContext) call(method string, args []any) (value any, err error) {
	method = yySDKPacketAlias(method)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: %v", method, r)
		}
	}()
	s.Mu.Lock()
	closed := s.Closed
	s.Mu.Unlock()
	if closed {
		return nil, fmt.Errorf("插件已停止")
	}
	if _, ok := yyStorageDocs[method]; ok {
		return yyStorageCall(s.Dir, method, args)
	}
	if method == "game.sendPacket" {
		return yySendPacket(yyAccountBot(s.Instance.Account), args)
	}
	if _, ok := yyResourceDocs[method]; ok {
		return yyResourceCall(yyResourceRoots{Code: filepath.Join(yyRuntimeRoot, s.Instance.Manifest.ID), Data: s.Dir, Config: filepath.Join(s.Dir, "config")}, method, args)
	}
	if method == "plugin.stats" {
		return map[string]any{"processed": s.Instance.Processed.Load(), "failures": s.Instance.Failures.Load(), "dropped": s.Instance.Dropped.Load(), "queue_depth": len(s.Instance.Queue), "queue_capacity": cap(s.Instance.Queue), "cpu_ms": nil, "memory_bytes": nil, "gc_ms": nil}, nil
	}
	if method == "plugin.capabilities" {
		return yyCapabilities(), nil
	}
	if method == "plugin.has" {
		name, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		return yyHasCapability(name), nil
	}
	if _, ok := yySchedulerDocs[method]; ok {
		if strings.HasPrefix(method, "util.schedule") {
			if len(args) != 2 {
				return nil, fmt.Errorf("需要间隔/频率和回调")
			}
			d, e := yyScheduleDuration(method, args[0])
			if e != nil {
				return nil, e
			}
			handler, e := yySDKString(args, 1)
			if e != nil {
				return nil, e
			}
			return s.schedule(d, false, handler), nil
		}
		return yySchedulerControl("sdk:"+s.Instance.Key, nil, method, args)
	}
	if method == "game.setCommandOptions" || method == "game.getCommandOptions" || method == "game.sendCommand" || method == "game.sendCommandFast" || method == "game.sendCommandsFast" || method == "game.sendCommandConfirmed" || method == "game.sendCommandAsync" || method == "game.commandResult" || method == "game.execute" || method == "game.executeBatch" {
		return s.pluginCommandCall(method, args)
	}
	if _, ok := yyAccountBridgeMethods[method]; ok {
		return yyBridgeSDKCall(s.Instance.Manifest.ID, method, args)
	}
	if _, ok := yyExtraMethods[method]; ok {
		return s.senseCall(method, args)
	}
	switch method {
	case "web.invoke":
		page, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		action, e := yySDKString(args, 1)
		if e != nil {
			return nil, e
		}
		fields := map[string]any{}
		if len(args) > 2 {
			var ok bool
			fields, ok = args[2].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("页面参数必须是对象")
			}
		}
		result, e, _ := yySDKWebAction(yySDKWebID(s.Instance.Key), page, action, fields)
		return result, e
	case "player.getUniqueID":
		name, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		pi := playerInfoMgr.GetPlayer(name)
		if pi == nil {
			return int64(0), nil
		}
		return pi.EntityID, nil
	case "game.registerTrigger":
		if len(args) != 2 {
			return nil, fmt.Errorf("registerTrigger(words, handler)")
		}
		words, ok := args[0].([]any)
		if !ok || len(words) == 0 {
			return nil, fmt.Errorf("words 必须是非空数组")
		}
		handler, e := yySDKString(args, 1)
		if e != nil {
			return nil, e
		}
		h := yySDKListener{Kind: "trigger", Handler: handler}
		for _, v := range words {
			word, ok := v.(string)
			if !ok || word == "" {
				return nil, fmt.Errorf("触发词不能为空")
			}
			h.Words = append(h.Words, word)
		}
		s.addListener(h)
		return true, nil
	case "events.onChat", "events.onJoin", "events.onLeave", "events.onBotReady", "events.onPacket":
		handler, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		kind := map[string]string{"events.onChat": "chat", "events.onJoin": "join", "events.onLeave": "leave", "events.onBotReady": "ready", "events.onPacket": "packet"}[method]
		s.addListener(yySDKListener{Kind: kind, Handler: handler})
		return true, nil
	case "events.subscribe", "events.subscribeAll":
		if method == "events.subscribeAll" {
			h, e := yySDKString(args, 0)
			if e != nil {
				return nil, e
			}
			s.Mu.Lock()
			defer s.Mu.Unlock()
			if s.Closed {
				return nil, fmt.Errorf("插件已停止")
			}
			for _, old := range s.Listeners {
				if old.Kind == "subscribeAll" && old.Handler == h {
					return true, nil
				}
			}
			u := packetHubSubscribe("*", func(bm *BotManager, data map[string]any) {
				if bm == yyAccountBot(s.Instance.Account) {
					s.enqueue(h, []any{yyPacketEnvelope(bm, data)})
				}
			})
			s.Unsubs = append(s.Unsubs, u)
			s.Listeners = append(s.Listeners, yySDKListener{Kind: "subscribeAll", Handler: h})
			return true, nil
		}
		packet, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		handler, e := yySDKString(args, 1)
		if e != nil {
			return nil, e
		}
		s.Mu.Lock()
		defer s.Mu.Unlock()
		if s.Closed {
			return nil, fmt.Errorf("插件已停止")
		}
		for _, h := range s.Listeners {
			if h.Kind == "subscribe" && h.Packet == packet && h.Handler == handler {
				return true, nil
			}
		}
		u := packetHubSubscribe(packet, func(bm *BotManager, data map[string]any) {
			if bm == yyAccountBot(s.Instance.Account) {
				s.enqueue(handler, []any{data})
			}
		})
		s.Unsubs = append(s.Unsubs, u)
		s.Listeners = append(s.Listeners, yySDKListener{Kind: "subscribe", Packet: packet, Handler: handler})
		return true, nil
	case "util.schedule", "util.setTimeout":
		if len(args) != 2 {
			return nil, fmt.Errorf("需要秒数和回调")
		}
		seconds, ok := args[0].(float64)
		if !ok || seconds < 0 || seconds > maxSchedIntervalSec || seconds != seconds {
			return nil, fmt.Errorf("无效的定时秒数")
		}
		if method == "util.schedule" && seconds < 0.001 {
			return nil, fmt.Errorf("重复任务最短 0.001 秒")
		}
		handler, e := yySDKString(args, 1)
		if e != nil {
			return nil, e
		}
		return s.schedule(time.Duration(seconds*float64(time.Second)), method == "util.setTimeout", handler), nil
	case "util.cancel":
		if len(args) != 1 {
			return nil, fmt.Errorf("需要 timer ID")
		}
		number, ok := args[0].(float64)
		if !ok {
			return nil, fmt.Errorf("无效 timer ID")
		}
		s.Mu.Lock()
		defer s.Mu.Unlock()
		_, ok = s.Timers[int64(number)]
		if ok {
			cancelSchedJob(int64(number))
			delete(s.Timers, int64(number))
		}
		return ok, nil
	case "plugin.id":
		return s.Instance.Manifest.ID, nil
	case "plugin.dependencies":
		if s.Instance.Manifest.Dependencies == nil {
			return map[string]string{}, nil
		}
		return s.Instance.Manifest.Dependencies, nil
	case "game.dataDir":
		_ = os.MkdirAll(s.Dir, 0755)
		return s.Dir, nil
	case "plugin.state":
		var state any = map[string]any{}
		data, e := os.ReadFile(filepath.Join(s.Dir, "state.json"))
		if e == nil {
			e = json.Unmarshal(data, &state)
		}
		if os.IsNotExist(e) {
			e = nil
		}
		return state, e
	case "plugin.save":
		if len(args) != 1 {
			return nil, fmt.Errorf("save 需要一个 JSON 对象")
		}
		return true, s.writeJSON("state.json", args[0])
	case "game.getConfig":
		cfg := map[string]any{}
		data, _ := os.ReadFile(filepath.Join(s.Dir, "config.json"))
		_ = json.Unmarshal(data, &cfg)
		key, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		if value, ok := cfg[key]; ok {
			return value, nil
		}
		if len(args) > 1 {
			return args[1], nil
		}
		return nil, nil
	case "game.setConfigDefault":
		if len(args) != 1 {
			return nil, fmt.Errorf("需要默认配置对象")
		}
		defaults, ok := args[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("默认配置必须是对象")
		}
		s.Mu.Lock()
		defer s.Mu.Unlock()
		cfg := map[string]any{}
		data, _ := os.ReadFile(filepath.Join(s.Dir, "config.json"))
		_ = json.Unmarshal(data, &cfg)
		for k, v := range defaults {
			if _, ok := cfg[k]; !ok {
				cfg[k] = v
			}
		}
		return true, s.writeJSON("config.json", cfg)
	case "api.call":
		return s.crossCall(args)
	case "api.require":
		return nil, fmt.Errorf("请使用 SDK 的 api.require 代理或 api.call(plugin, function, args)")
	}
	s.BindingMu.Lock()
	defer s.BindingMu.Unlock()
	if s.L == nil {
		s.L = newLuaState(yyAccountBot(s.Instance.Account))
		s.L.SetGlobal("__PLUGIN_ID__", lua.LString(yySDKWebID(s.Instance.Key)))
	}
	L := s.L
	old := abot(L)
	main := yyAccountBot(s.Instance.Account)
	cancelLuaCtx(L)
	setLuaBotCtx(L, main)
	if old != nil && old.IsConnected() {
		if c, ok := L.Context().Value(luaBotCtxKey{}).(*luaBotCtx); ok {
			c.active = old
		}
	}
	defer cancelLuaCtx(L)
	top := L.GetTop()
	defer L.SetTop(top)
	parts := strings.Split(method, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("格式必须是 namespace.method")
	}
	allowed := false
	for _, name := range yySDKNames {
		if name == parts[0] {
			allowed = true
		}
	}
	if !allowed {
		return nil, fmt.Errorf("未知命名空间")
	}
	if yySDKGameRequired(method) && (abot(L) == nil || !abot(L).IsConnected()) {
		return nil, fmt.Errorf("目标游戏账号未连接，不能执行 %s", method)
	}
	if method == "game.execute" {
		command, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		return abot(L).SendConsole("/" + strings.TrimLeft(command, "/"))
	}
	table, ok := L.GetGlobal(parts[0]).(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("未知命名空间")
	}
	fn := table.RawGetString(parts[1])
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("未知接口 %s", method)
	}
	values := make([]lua.LValue, len(args))
	for i, a := range args {
		values[i] = goToLua(L, a)
	}
	if e := L.CallByParam(lua.P{Fn: fn, NRet: lua.MultRet, Protect: true}, values...); e != nil {
		return nil, e
	}
	count := L.GetTop() - top
	if count == 0 {
		return nil, nil
	}
	if count == 1 {
		result := luaToGo(L.Get(top + 1))
		if method == "player.list" || method == "bot.list" || method == "bot.status" || method == "game.getTarget" || method == "game.getTickingAreaList" {
			if object, ok := result.(map[string]any); ok && len(object) == 0 {
				return []any{}, nil
			}
		}
		return result, nil
	}
	result := make([]any, count)
	for i := range result {
		result[i] = luaToGo(L.Get(top + i + 1))
	}
	return result, nil
}
func yySDKGameRequired(method string) bool {
	if strings.HasPrefix(method, "world.") {
		return true
	}
	if strings.HasPrefix(method, "game.") {
		for _, prefix := range []string{"game.http", "game.json", "game.getConfig", "game.setConfig", "game.dataDir"} {
			if strings.HasPrefix(method, prefix) {
				return false
			}
		}
		return true
	}
	return method == "player.getInventory"
}
func (s *yySDKContext) writeJSON(name string, value any) error {
	data, e := json.Marshal(value)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(s.Dir, 0755); e != nil {
		return e
	}
	path := filepath.Join(s.Dir, name)
	if e = os.WriteFile(path+".tmp", data, 0600); e != nil {
		return e
	}
	return os.Rename(path+".tmp", path)
}
func (s *yySDKContext) addListener(h yySDKListener) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Closed {
		return
	}
	for _, old := range s.Listeners {
		if old.Kind == h.Kind && old.Handler == h.Handler && strings.Join(old.Words, "\x00") == strings.Join(h.Words, "\x00") {
			return
		}
	}
	s.Listeners = append(s.Listeners, h)
}
func (s *yySDKContext) schedule(delay time.Duration, once bool, handler string) int64 {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Closed {
		return 0
	}
	var id int64
	id = newSchedJobWith("sdk:"+s.Instance.Key, handler, delay, once, nil, func(cancel <-chan struct{}) error {
		completion := make(chan struct{})
		event := map[string]any{"type": "sdk_callback", "handler": handler, "args": []any{}, "_timer_done": completion}
		select {
		case s.Instance.Queue <- event:
		case <-cancel:
			return nil
		case <-s.Instance.Done:
			return nil
		default:
			s.Instance.Dropped.Add(1)
			return fmt.Errorf("定时回调队列已满，跳过本次")
		}
		// One outstanding invocation per timer. No 40Hz timer can enqueue
		// hundreds of unfinished callbacks behind a slow Python/C++ handler.
		select {
		case <-completion:
			return nil
		case <-cancel:
			return nil
		case <-s.Instance.Done:
			return nil
		}
	}, func() { s.Mu.Lock(); delete(s.Timers, id); s.Mu.Unlock() })
	schedMu.Lock()
	s.Timers[id] = schedJobs[id].cancel
	schedMu.Unlock()
	return id
}
func (s *yySDKContext) crossCall(args []any) (any, error) {
	target, e := yySDKString(args, 0)
	if e != nil {
		return nil, e
	}
	handler, e := yySDKString(args, 1)
	if e != nil {
		return nil, e
	}
	values := []any{}
	if len(args) > 2 {
		var ok bool
		values, ok = args[2].([]any)
		if !ok {
			return nil, fmt.Errorf("函数参数必须是数组")
		}
	}
	key := s.Instance.Account + ":" + target
	yySDKCalls.Lock()
	for cursor := key; cursor != ""; cursor = yySDKCalls.Edges[cursor] {
		if cursor == s.Instance.Key {
			yySDKCalls.Unlock()
			return nil, fmt.Errorf("拒绝跨插件循环调用")
		}
	}
	yySDKCalls.Edges[s.Instance.Key] = key
	yySDKCalls.Unlock()
	defer func() { yySDKCalls.Lock(); delete(yySDKCalls.Edges, s.Instance.Key); yySDKCalls.Unlock() }()
	yyRuntimeMu.Lock()
	instance := yyRuntimeInstances[key]
	yyRuntimeMu.Unlock()
	if instance != nil {
		text, e := instance.rpcEvent(map[string]any{"type": "sdk_rpc", "handler": handler, "args": values})
		if e != nil {
			return nil, e
		}
		var reply map[string]any
		if e = json.Unmarshal([]byte(text), &reply); e != nil {
			return nil, e
		}
		if reply["ok"] == false {
			return nil, fmt.Errorf("%v", reply["error"])
		}
		return reply["result"], nil
	}
	data, _ := json.Marshal(values)
	text, e := callPluginFunction(yyAccountBot(s.Instance.Account), target, handler, string(data))
	if e != nil {
		return nil, e
	}
	var reply any
	e = json.Unmarshal([]byte(text), &reply)
	return reply, e
}

// Web actions registered by a native/Python SDK instance use its resident handler.
func yySDKWebAction(plugin, page, action string, args map[string]any) (any, error, bool) {
	yySDKContexts.Lock()
	var found *yySDKContext
	for key, s := range yySDKContexts.Items {
		if plugin == yySDKWebID(key) {
			found = s
			break
		}
	}
	yySDKContexts.Unlock()
	if found == nil {
		return nil, nil, false
	}
	h, ok := lookupWebAction(plugin, page, action)
	if !ok {
		return nil, fmt.Errorf("动作不存在"), true
	}
	text, e := found.Instance.rpcEvent(map[string]any{"type": "sdk_callback", "handler": h.handler, "args": []any{args}})
	if e != nil {
		return nil, e, true
	}
	var reply map[string]any
	e = json.Unmarshal([]byte(text), &reply)
	if reply["ok"] == false {
		return nil, fmt.Errorf("%v", reply["error"]), true
	}
	return reply["result"], e, true
}

func yySDKPluginContext(bm *BotManager, id string) *yySDKContext {
	yySDKContexts.Lock()
	defer yySDKContexts.Unlock()
	for _, s := range yySDKContexts.Items {
		if s.Instance.Manifest.ID == id && yyAccountBot(s.Instance.Account) == bm && (bm != nil || s.Instance.Account == "main") {
			return s
		}
	}
	return nil
}
func yySDKExport(bm *BotManager, id, handler, argsJSON string) (string, error, bool) {
	s := yySDKPluginContext(bm, id)
	if s == nil {
		return "", nil, false
	}
	var args []any
	if argsJSON != "" {
		if e := json.Unmarshal([]byte(argsJSON), &args); e != nil {
			return "", e, true
		}
	}
	text, e := s.Instance.rpcEvent(map[string]any{"type": "sdk_rpc", "handler": handler, "args": args})
	if e != nil {
		return "", e, true
	}
	var reply map[string]any
	if e = json.Unmarshal([]byte(text), &reply); e != nil {
		return "", e, true
	}
	if reply["ok"] == false {
		return "", fmt.Errorf("%v", reply["error"]), true
	}
	data, e := json.Marshal(reply["result"])
	return string(data), e, true
}
func yySDKWebRoot(plugin string) string {
	yySDKContexts.Lock()
	defer yySDKContexts.Unlock()
	for key, s := range yySDKContexts.Items {
		if plugin == yySDKWebID(key) {
			return filepath.Join(yyRuntimeRoot, s.Instance.Manifest.ID, "www")
		}
	}
	return ""
}
