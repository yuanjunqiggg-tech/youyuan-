package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// registerAllLuaAPI 注册插件可用的全部 Lua 全局 API（game/system/api/plugin/util/bot/building/media/world）。
func registerAllLuaAPI(L *lua.LState, bm *BotManager) {
	registerGameAPI(L, bm)
	registerLegacyGameAPI(L)
	// Original plugins probe these synonymous names. They share the native TPS
	// interface, and return nil when the host has no server-reported measurement.
	g := L.GetGlobal("game")
	observedTPS := L.NewFunction(func(L *lua.LState) int {
		value, known := abot(L).observedTPS()
		if known {
			L.Push(lua.LNumber(value))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	})
	for _, name := range []string{"getTPS", "getServerTPS", "serverTPS", "getTickRate", "getServerTickRate"} {
		L.SetField(g, name, observedTPS)
	}
	registerSystemAPI(L)
	registerCrossPluginAPI(L, bm)
	registerUtilAPI(L)
	registerHTTPAPI(L)
	registerBotAPI(L)
	registerBuildingAPI(L)
	registerMediaAPI(L)
	registerWorldAPI(L)
	registerPlayerAPI(L)
	registerEventAPI(L)
	registerWebAPI(L)
	registerSenseAPI(L)
	registerResourceAPI(L)
	registerSchedulerAPI(L)
	registerProfilerAPI(L)
	registerCapabilityAPI(L)
	registerPacketAPI(L)
	registerStorageAPI(L)
	registerAccountBridgeAPI(L)
	registerAccountPoolAPI(L)
}

// newLuaState 创建一个已注册全部插件 API 的 Lua 状态。
func newLuaState(bm *BotManager) *lua.LState {
	L := newYYLuaVM()
	registerAllLuaAPI(L, bm)
	return L
}

// A function can now use 5120 registers; nested calls need more than one
// frame's data stack. Allocate only the initial frame and grow on demand.
func newYYLuaVM() *lua.LState {
	return lua.NewState(lua.Options{
		RegistrySize:     5120,
		RegistryGrowStep: 5120,
		RegistryMaxSize:  5120 * lua.CallStackSize,
		CallStackSize:    lua.CallStackSize,
	})
}

// registerGameAPI 把游戏便捷 API 注册为 Lua 全局表 game。
func registerGameAPI(L *lua.LState, bm *BotManager) {
	setLuaBotCtx(L, bm)
	g := L.NewTable()
	L.SetGlobal("game", g)
	defer registerPluginCommandAPI(L, g)

	// game.sendCommand(cmd)：发送一条指令（走 sendCommandHook，可被测试替换）。
	L.SetField(g, "sendCommand", L.NewFunction(func(L *lua.LState) int {
		cmd := L.CheckString(1)
		if f := getSendCommandHook(); f != nil {
			if err := f(cmd); err != nil {
				L.RaiseError("sendCommand: %v", err)
			}
			return 0
		}
		if err := abot(L).SendCommandNoWait(ChannelAI, cmd); err != nil {
			L.RaiseError("sendCommand: %v", err)
		}
		return 0
	}))

	// game.sendTargeted(player, text)：定向发送文本给玩家。
	L.SetField(g, "sendTargeted", L.NewFunction(func(L *lua.LState) int {
		player := L.CheckString(1)
		text := L.CheckString(2)
		if err := abot(L).sendTargeted(player, text); err != nil {
			L.RaiseError("sendTargeted: %v", err)
		}
		return 0
	}))

	// game.registerTrigger(wordsTable, handlerName)：注册触发词，触发时在常驻 VM 中
	// 调用名为 handlerName 的全局 Lua 函数 (player, args)。
	L.SetField(g, "registerTrigger", L.NewFunction(func(L *lua.LState) int {
		wordsTbl := L.CheckTable(1)
		handlerName := yySenseLuaHandler(L, 2)
		var words []string
		wordsTbl.ForEach(func(_, v lua.LValue) {
			words = append(words, lua.LVAsString(v))
		})
		srcStr := lua.LVAsString(L.GetGlobal("__PLUGIN_SRC__"))
		// 触发执行时，临时 VM 重跑源码会再次走到这里；此时 __PLUGIN_SRC__ 为空，
		// 跳过重复注册，避免注册表被每次触发污染。
		if srcStr == "" {
			debugLog("[Lua] registerTrigger 跳过（执行上下文重跑，避免重复注册）")
			return 0
		}
		pid := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		debugLog("[Lua] registerTrigger 注册 插件=%q 触发词=%v handler=%q", pid, words, handlerName)
		owner := abot(L)
		owner.AddChatTrigger(&ChatTrigger{
			Plugin:       pid,
			TriggerWords: words,
			Usage:        "Lua插件触发词",
			RawArgs:      true, // 触发词后的全部参数按空格传给 Lua handler
			Func: func(player string, args []any) {
				debugLog("[Lua] 执行 handler 插件=%q handler=%q player=%s", pid, handlerName, player)
				if _, err := yyCallResident(owner, pid, handlerName, []any{player, args}, false, false); err != nil {
					debugLog("[插件] 常驻回调失败 %s.%s: %v", pid, handlerName, err)
				}
			},
		})
		return 0
	}))

	// ---- 便捷层 API ----
	// game.getScore(scoreboard, target) → number
	L.SetField(g, "getScore", L.NewFunction(func(L *lua.LState) int {
		scb := L.CheckString(1)
		target := L.CheckString(2)
		bm := abot(L)
		v, err := yyLuaWaitValue(L, func(_ context.Context) (int, error) { return bm.GetScore(scb, target) })
		if err != nil {
			L.RaiseError("getScore: %v", err)
		}
		L.Push(lua.LNumber(v))
		return 1
	}))
	// game.getMultiScore(target) → table {player={board=score}}
	L.SetField(g, "getMultiScore", L.NewFunction(func(L *lua.LState) int {
		target := L.CheckString(1)
		m, err := yyLuaWait1(L, abot(L).GetMultiScore, target)
		if err != nil {
			L.RaiseError("getMultiScore: %v", err)
		}
		L.Push(multiScoreToTable(L, m))
		return 1
	}))
	// game.getPlayerPos(target) → table {x,y,z,dimension}
	L.SetField(g, "getPlayerPos", L.NewFunction(func(L *lua.LState) int {
		target := L.CheckString(1)
		p, err := yyLuaWait1(L, abot(L).GetPlayerPos, target)
		if err != nil {
			L.RaiseError("getPlayerPos: %v", err)
		}
		t := L.NewTable()
		t.RawSetString("x", lua.LNumber(float64(p.Position.X)))
		t.RawSetString("y", lua.LNumber(float64(p.Position.Y)))
		t.RawSetString("z", lua.LNumber(float64(p.Position.Z)))
		t.RawSetString("dimension", lua.LNumber(p.Dimension))
		L.Push(t)
		return 1
	}))
	// game.getPlayerPosXYZ(target) → x,y,z
	L.SetField(g, "getPlayerPosXYZ", L.NewFunction(func(L *lua.LState) int {
		target := L.CheckString(1)
		p, err := yyLuaWait1(L, abot(L).GetPlayerPos, target)
		if err != nil {
			L.RaiseError("getPlayerPosXYZ: %v", err)
		}
		L.Push(lua.LNumber(p.Position.X))
		L.Push(lua.LNumber(p.Position.Y))
		L.Push(lua.LNumber(p.Position.Z))
		return 3
	}))
	// game.getPlayerItem(target, itemName, specialID) → number
	L.SetField(g, "getPlayerItem", L.NewFunction(func(L *lua.LState) int {
		target := L.CheckString(1)
		item := L.CheckString(2)
		special := L.OptInt(3, 0)
		bm := abot(L)
		v, err := yyLuaWaitValue(L, func(_ context.Context) (int, error) { return bm.GetPlayerItem(target, item, special) })
		if err != nil {
			L.RaiseError("getPlayerItem: %v", err)
		}
		L.Push(lua.LNumber(v))
		return 1
	}))
	// game.getBlockTile(x, y, z) → string
	// 用 GetBlockTileSafe：传送加载区块再查询，避免远处区块未加载返回 outOfWorld。
	L.SetField(g, "getBlockTile", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		bm := abot(L)
		name, err := yyLuaWaitValue(L, func(_ context.Context) (string, error) { return bm.GetBlockTileSafe(x, y, z) })
		if err != nil {
			L.RaiseError("getBlockTile: %v", err)
		}
		L.Push(lua.LString(name))
		return 1
	}))
	// game.getTarget(selector) → table of player names
	L.SetField(g, "getTarget", L.NewFunction(func(L *lua.LState) int {
		sel := L.CheckString(1)
		names, err := yyLuaWait1(L, abot(L).GetTarget, sel)
		if err != nil {
			L.RaiseError("getTarget: %v", err)
		}
		t := L.NewTable()
		for i, n := range names {
			t.RawSetInt(i+1, lua.LString(n))
		}
		L.Push(t)
		return 1
	}))
	// game.waitMsg(player, timeoutSec) → string
	L.SetField(g, "waitMsg", L.NewFunction(func(L *lua.LState) int {
		player := L.CheckString(1)
		to := time.Duration(float64(L.OptNumber(2, 30)) * float64(time.Second))
		bm := abot(L)
		msg, err := yyLuaWait(L, func(ctx context.Context) (string, error) { return bm.WaitMsgContext(ctx, player, to) })
		if err != nil {
			L.RaiseError("waitMsg: %v", err)
		}
		L.Push(lua.LString(msg))
		return 1
	}))
	// game.showMenu(player, title, optionsTable)
	L.SetField(g, "showMenu", L.NewFunction(func(L *lua.LState) int {
		player := L.CheckString(1)
		title := L.CheckString(2)
		optTbl := L.CheckTable(3)
		menu := &ChatMenu{Title: title}
		optTbl.ForEach(func(_, v lua.LValue) { menu.Options = append(menu.Options, MenuOption{Label: lua.LVAsString(v)}) })
		abot(L).ShowMenu(player, menu)
		return 0
	}))
	// game.promptChoice(player, title, optionsTable, timeoutSec) → number(index)
	L.SetField(g, "promptChoice", L.NewFunction(func(L *lua.LState) int {
		player := L.CheckString(1)
		title := L.CheckString(2)
		optTbl := L.CheckTable(3)
		to := time.Duration(float64(L.OptNumber(4, 30)) * float64(time.Second))
		menu := &ChatMenu{Title: title}
		optTbl.ForEach(func(_, v lua.LValue) { menu.Options = append(menu.Options, MenuOption{Label: lua.LVAsString(v)}) })
		bm := abot(L)
		idx, err := yyLuaWaitValue(L, func(_ context.Context) (int, error) { return bm.PromptChoice(player, menu, to) })
		if err != nil {
			L.RaiseError("promptChoice: %v", err)
		}
		if idx < 0 { // 无效/超时/取消 → nil
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LNumber(idx + 1)) // 1 起（选项1 = 1）
		return 1
	}))

	// ---- 配置与数据目录 API（需 __PLUGIN_ID__ 与 pluginController 解析路径）----
	// game.getConfig(key, default) → value from 插件配置文件/<id>.json，缺失回退 default
	L.SetField(g, "getConfig", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		def := L.Get(2)
		id := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		if id == "" {
			L.Push(def)
			return 1
		}
		cfg, err := pluginController.ReadConfig(id)
		if err == nil {
			if v, ok := cfg[key]; ok {
				L.Push(toLuaValue(L, v))
				return 1
			}
		}
		L.Push(def)
		return 1
	}))
	// game.setConfigDefault(table)：把缺失的键写入配置文件
	L.SetField(g, "setConfigDefault", L.NewFunction(func(L *lua.LState) int {
		id := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		if id == "" {
			return 0
		}
		tbl := L.CheckTable(1)
		cfg, _ := pluginController.ReadConfig(id)
		if cfg == nil {
			cfg = map[string]any{}
		}
		tbl.ForEach(func(k, v lua.LValue) {
			key := lua.LVAsString(k)
			if _, ok := cfg[key]; !ok {
				cfg[key] = luaToGo(v)
			}
		})
		_ = pluginController.WriteConfig(id, cfg)
		return 0
	}))
	// game.dataDir() → 插件数据目录路径
	L.SetField(g, "dataDir", L.NewFunction(func(L *lua.LState) int {
		id := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		if id == "" {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(pluginController.DataDir(id)))
		return 1
	}))

	// ---- 网络与 JSON 能力（让插件能做任意 HTTP 调用，如 AI 聊天、拉取数据）----
	// game.httpGet(url) → string
	L.SetField(g, "httpGet", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		body, err := yyLuaWait(L, func(ctx context.Context) (string, error) { return httpGetTextContext(ctx, url) })
		if err != nil {
			L.RaiseError("httpGet: %v", err)
		}
		L.Push(lua.LString(body))
		return 1
	}))
	// game.httpPost(url, body, headersTable) → string
	L.SetField(g, "httpPost", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		body := L.OptString(2, "")
		headers := map[string]string{}
		if t := L.OptTable(3, nil); t != nil {
			t.ForEach(func(k, v lua.LValue) { headers[lua.LVAsString(k)] = lua.LVAsString(v) })
		}
		resp, err := yyLuaWait(L, func(ctx context.Context) (string, error) { return httpPostTextContext(ctx, url, body, headers) })
		if err != nil {
			L.RaiseError("httpPost: %v", err)
		}
		L.Push(lua.LString(resp))
		return 1
	}))
	// game.jsonEncode(v) → string
	L.SetField(g, "jsonEncode", L.NewFunction(func(L *lua.LState) int {
		s, err := jsonEncodeGo(luaToGo(L.Get(1)))
		if err != nil {
			L.RaiseError("jsonEncode: %v", err)
		}
		L.Push(lua.LString(s))
		return 1
	}))
	// game.jsonDecode(s) → any（表/数字/字符串/布尔）
	L.SetField(g, "jsonDecode", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		v, err := jsonDecodeGo(s)
		if err != nil {
			L.RaiseError("jsonDecode: %v", err)
		}
		L.Push(goToLua(L, v))
		return 1
	}))

	// ---- 读取类（补齐）----
	// game.getBlockTileRaw(x,y,z) → string（不走传送加载）
	L.SetField(g, "getBlockTileRaw", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		bm := abot(L)
		name, err := yyLuaWaitValue(L, func(_ context.Context) (string, error) { return bm.GetBlockTile(x, y, z) })
		if err != nil {
			L.RaiseError("getBlockTileRaw: %v", err)
		}
		L.Push(lua.LString(name))
		return 1
	}))
	// game.getTickingAreaList() → table {name={...}}
	L.SetField(g, "getTickingAreaList", L.NewFunction(func(L *lua.LState) int {
		bm := abot(L)
		m, err := yyLuaWaitValue(L, func(_ context.Context) (map[string]*TickingAreaInfo, error) { return bm.GetTickingAreaList() })
		if err != nil {
			L.RaiseError("getTickingAreaList: %v", err)
		}
		t := L.NewTable()
		for k, v := range m {
			inner := L.NewTable()
			inner.RawSetString("dimension", lua.LString(v.Dimension))
			inner.RawSetString("start_x", lua.LNumber(v.StartX))
			inner.RawSetString("start_z", lua.LNumber(v.StartZ))
			inner.RawSetString("end_x", lua.LNumber(v.EndX))
			inner.RawSetString("end_z", lua.LNumber(v.EndZ))
			t.RawSetString(k, inner)
		}
		L.Push(t)
		return 1
	}))
	// game.queryInventory(selector) → {items={...}}
	L.SetField(g, "queryInventory", L.NewFunction(func(L *lua.LState) int {
		sel := L.CheckString(1)
		inv, err := yyLuaWait1(L, abot(L).QueryPlayerInventory, sel)
		if err != nil {
			L.RaiseError("queryInventory: %v", err)
		}
		items := L.NewTable()
		for i, it := range inv.Items {
			entry := L.NewTable()
			entry.RawSetString("name", lua.LString(it.Name))
			entry.RawSetString("count", lua.LNumber(it.Count))
			entry.RawSetString("slot", lua.LNumber(it.Slot))
			entry.RawSetString("damage", lua.LNumber(it.Damage))
			items.RawSetInt(i+1, entry)
		}
		t := L.NewTable()
		t.RawSetString("items", items)
		L.Push(t)
		return 1
	}))
	// game.isCmdSuccess(cmd) → boolean
	L.SetField(g, "isCmdSuccess", L.NewFunction(func(L *lua.LState) int {
		cmd := L.CheckString(1)
		ok, err := yyLuaWait1(L, abot(L).IsCmdSuccess, cmd)
		if err != nil {
			L.RaiseError("isCmdSuccess: %v", err)
		}
		L.Push(lua.LBool(ok))
		return 1
	}))

	// ---- 展示类 ----
	L.SetField(g, "sendTitle", L.NewFunction(func(L *lua.LState) int {
		target, title, sub := L.CheckString(1), L.CheckString(2), L.CheckString(3)
		if err := abot(L).SendTitle(target, title, sub); err != nil {
			L.RaiseError("sendTitle: %v", err)
		}
		return 0
	}))
	L.SetField(g, "sendActionbar", L.NewFunction(func(L *lua.LState) int {
		target, text := L.CheckString(1), L.CheckString(2)
		if err := abot(L).SendActionbar(target, text); err != nil {
			L.RaiseError("sendActionbar: %v", err)
		}
		return 0
	}))
	L.SetField(g, "sendRichTellraw", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		if err := abot(L).SendRichTellraw(msg); err != nil {
			L.RaiseError("sendRichTellraw: %v", err)
		}
		return 0
	}))
	L.SetField(g, "sendChat", L.NewFunction(func(L *lua.LState) int {
		target, msg := L.CheckString(1), L.CheckString(2)
		if err := abot(L).SendChat(target, msg); err != nil {
			L.RaiseError("sendChat: %v", err)
		}
		return 0
	}))
	// game.sendLines(player, text, maxLines?)：把长文本按行逐条发送。
	// 每行一条消息，避免整段超长被拦，或某一行含敏感词导致整段失效。maxLines 默认 50，超出提示截断。
	L.SetField(g, "sendLines", L.NewFunction(func(L *lua.LState) int {
		player := L.CheckString(1)
		text := L.CheckString(2)
		maxLines := L.OptInt(3, 50)
		lines := strings.Split(text, "\n")
		if len(lines) > maxLines {
			abot(L).sendTargeted(player, fmt.Sprintf("§7输出共 %d 行，仅显示前 %d 行", len(lines), maxLines))
		}
		for i := 0; i < len(lines) && i < maxLines; i++ {
			line := strings.TrimRight(lines[i], "\r")
			if line != "" {
				abot(L).sendTargeted(player, line)
			}
		}
		return 0
	}))

	// ---- 药水 ----
	// game.setEffect(player, effect, duration, level, particles)
	L.SetField(g, "setEffect", L.NewFunction(func(L *lua.LState) int {
		player, effect := L.CheckString(1), L.CheckString(2)
		dur, lvl := L.CheckInt(3), L.CheckInt(4)
		particles := L.OptBool(5, true)
		if err := abot(L).SetPlayerEffect(player, effect, dur, lvl, particles); err != nil {
			L.RaiseError("setEffect: %v", err)
		}
		return 0
	}))

	// game.setScore(board, target, score)：写入记分板（scoreboard players set）
	L.SetField(g, "setScore", L.NewFunction(func(L *lua.LState) int {
		board, target := L.CheckString(1), L.CheckString(2)
		score := L.CheckInt(3)
		cmd := fmt.Sprintf("scoreboard players set %s %s %d", target, board, score)
		if err := abot(L).SendCommandNoWait(ChannelAI, cmd); err != nil {
			L.RaiseError("setScore: %v", err)
		}
		return 0
	}))

	// ---- 机器人移动 / 飞行（fly.go）----
	L.SetField(g, "teleport", L.NewFunction(func(L *lua.LState) int {
		if e := yyPluginMovementAllowed(abot(L)); e != nil {
			L.RaiseError("%v", e)
			return 0
		}
		x, y, z := float32(L.CheckNumber(1)), float32(L.CheckNumber(2)), float32(L.CheckNumber(3))
		if err := abot(L).TeleportTo(x, y, z); err != nil {
			L.RaiseError("teleport: %v", err)
		}
		return 0
	}))
	L.SetField(g, "move", L.NewFunction(func(L *lua.LState) int {
		if e := yyPluginMovementAllowed(abot(L)); e != nil {
			L.RaiseError("%v", e)
			return 0
		}
		dx, dy, dz := float32(L.CheckNumber(1)), float32(L.CheckNumber(2)), float32(L.CheckNumber(3))
		if err := abot(L).MoveRelative(dx, dy, dz); err != nil {
			L.RaiseError("move: %v", err)
		}
		return 0
	}))
	L.SetField(g, "rotate", L.NewFunction(func(L *lua.LState) int {
		pitch, yaw := float32(L.CheckNumber(1)), float32(L.CheckNumber(2))
		seconds := float32(L.OptNumber(3, 0))
		var err error
		if seconds > 0 {
			// 在 seconds 秒内平滑转到目标角度（yaw 走最短路径）
			err = abot(L).SmoothRotateTo(pitch, yaw, seconds)
		} else {
			err = abot(L).SetRotation(pitch, yaw)
		}
		if err != nil {
			L.RaiseError("rotate: %v", err)
		}
		return 0
	}))
	L.SetField(g, "jump", L.NewFunction(func(L *lua.LState) int {
		if err := abot(L).Jump(); err != nil {
			L.RaiseError("jump: %v", err)
		}
		return 0
	}))
	L.SetField(g, "sneakStart", L.NewFunction(func(L *lua.LState) int {
		if err := abot(L).SneakStart(); err != nil {
			L.RaiseError("sneakStart: %v", err)
		}
		return 0
	}))
	L.SetField(g, "sneakStop", L.NewFunction(func(L *lua.LState) int {
		if err := abot(L).SneakStop(); err != nil {
			L.RaiseError("sneakStop: %v", err)
		}
		return 0
	}))
	L.SetField(g, "sprintStart", L.NewFunction(func(L *lua.LState) int {
		if err := abot(L).SprintStart(); err != nil {
			L.RaiseError("sprintStart: %v", err)
		}
		return 0
	}))
	L.SetField(g, "sprintStop", L.NewFunction(func(L *lua.LState) int {
		if err := abot(L).SprintStop(); err != nil {
			L.RaiseError("sprintStop: %v", err)
		}
		return 0
	}))
	L.SetField(g, "flyStart", L.NewFunction(func(L *lua.LState) int {
		if err := abot(L).StartFlying(); err != nil {
			L.RaiseError("flyStart: %v", err)
		}
		return 0
	}))
	L.SetField(g, "flyStop", L.NewFunction(func(L *lua.LState) int {
		if err := abot(L).StopFlying(); err != nil {
			L.RaiseError("flyStop: %v", err)
		}
		return 0
	}))
	L.SetField(g, "stopMove", L.NewFunction(func(L *lua.LState) int {
		if err := abot(L).StopMovement(); err != nil {
			L.RaiseError("stopMove: %v", err)
		}
		return 0
	}))
	// game.getBotPos() → {x,y,z}
	L.SetField(g, "getBotPos", L.NewFunction(func(L *lua.LState) int {
		pos := abot(L).GetPosition()
		t := L.NewTable()
		t.RawSetString("x", lua.LNumber(numField(pos, "x")))
		t.RawSetString("y", lua.LNumber(numField(pos, "y")))
		t.RawSetString("z", lua.LNumber(numField(pos, "z")))
		L.Push(t)
		return 1
	}))

	// ── 方块 NBT / 方块状态 读写（底层：nbt_world_api.go）──

	// game.getBlockData(x,y,z) → {name=..., states={...}, nbt={...}}
	L.SetField(g, "getBlockData", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		bm := abot(L)
		bd, err := yyLuaWaitValue(L, func(_ context.Context) (*BlockData, error) { return bm.GetBlockData(x, y, z) })
		if err != nil {
			L.RaiseError("getBlockData: %v", err)
		}
		t := L.NewTable()
		t.RawSetString("name", lua.LString(bd.Name))
		if bd.States != nil {
			t.RawSetString("states", goToLua(L, bd.States))
		}
		if bd.NBT != nil {
			t.RawSetString("nbt", goToLua(L, bd.NBT))
		}
		L.Push(t)
		return 1
	}))
	// game.getBlockNBT(x,y,z) → table（命令方块指令/箱子物品/告示牌文本/旗帜图案等）
	L.SetField(g, "getBlockNBT", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		bm := abot(L)
		nbt, err := yyLuaWaitValue(L, func(_ context.Context) (map[string]any, error) { return bm.GetBlockNBT(x, y, z) })
		if err != nil {
			L.RaiseError("getBlockNBT: %v", err)
		}
		if nbt == nil {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(goToLua(L, nbt))
		return 1
	}))
	// game.getBlockState(x,y,z) → {name=..., states={...}}
	L.SetField(g, "getBlockState", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		bm := abot(L)
		bd, err := yyLuaWaitValue(L, func(_ context.Context) (*BlockData, error) { return bm.GetBlockData(x, y, z) })
		if err != nil {
			L.RaiseError("getBlockState: %v", err)
		}
		t := L.NewTable()
		t.RawSetString("name", lua.LString(bd.Name))
		if bd.States != nil {
			t.RawSetString("states", goToLua(L, bd.States))
		}
		L.Push(t)
		return 1
	}))
	// game.getRegionBlockData(x,y,z,sx,sy,sz) → {["x,y,z"]={name,states,nbt}, ...}
	L.SetField(g, "getRegionBlockData", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		sx, sy, sz := L.CheckInt(4), L.CheckInt(5), L.CheckInt(6)
		bm := abot(L)
		pos, size := [3]int32{int32(x), int32(y), int32(z)}, [3]int32{int32(sx), int32(sy), int32(sz)}
		region, err := yyLuaWaitValue(L, func(_ context.Context) (map[string]*BlockData, error) { return bm.GetRegionBlockData(pos, size) })
		if err != nil {
			L.RaiseError("getRegionBlockData: %v", err)
		}
		t := L.NewTable()
		for key, bd := range region {
			e := L.NewTable()
			e.RawSetString("name", lua.LString(bd.Name))
			if bd.States != nil {
				e.RawSetString("states", goToLua(L, bd.States))
			}
			if bd.NBT != nil {
				e.RawSetString("nbt", goToLua(L, bd.NBT))
			}
			t.RawSetString(key, e)
		}
		L.Push(t)
		return 1
	}))
	// game.setBlock(x,y,z, name, states?)
	L.SetField(g, "setBlock", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		name := L.CheckString(4)
		states := L.OptString(5, "")
		if err := abot(L).SetBlock(x, y, z, name, states); err != nil {
			L.RaiseError("setBlock: %v", err)
		}
		return 0
	}))
	// game.placeNBTBlock(x,y,z, name, states?, nbtTable?)
	L.SetField(g, "placeNBTBlock", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		name := L.CheckString(4)
		states := L.OptString(5, "")
		var nbt map[string]any
		if t := L.OptTable(6, nil); t != nil {
			var ok bool
			nbt, ok = luaToGo(t).(map[string]any)
			if !ok {
				L.RaiseError("placeNBTBlock: nbtTable 必须是表")
			}
		}
		bm := abot(L)
		if _, err := yyLuaWaitValue(L, func(_ context.Context) (bool, error) {
			release := yyBeginSpatialJob(bm, "plugin-placeNBTBlock")
			defer release()
			return true, bm.PlaceNBTBlock(x, y, z, name, states, nbt)
		}); err != nil {
			L.RaiseError("placeNBTBlock: %v", err)
		}
		return 0
	}))
	// game.setBlockState(x,y,z, statesTable) → 保留原 NBT，合并后重新放置
	L.SetField(g, "setBlockState", L.NewFunction(func(L *lua.LState) int {
		x, y, z := L.CheckInt(1), L.CheckInt(2), L.CheckInt(3)
		statesTbl := L.CheckTable(4)
		states, ok := luaToGo(statesTbl).(map[string]any)
		if !ok {
			L.RaiseError("setBlockState: statesTable 必须是表")
		}
		bm := abot(L)
		if _, err := yyLuaWaitValue(L, func(_ context.Context) (bool, error) { return true, bm.SetBlockState(x, y, z, states) }); err != nil {
			L.RaiseError("setBlockState: %v", err)
		}
		return 0
	}))
}

// goToLua 把 Go 值（map/slice/基本类型）转为 Lua 值。
func goToLua(L *lua.LState, v any) lua.LValue {
	switch x := v.(type) {
	case nil:
		return lua.LNil
	case string:
		return lua.LString(x)
	case bool:
		return lua.LBool(x)
	case float64:
		return lua.LNumber(x)
	case float32:
		return lua.LNumber(x)
	case int:
		return lua.LNumber(x)
	case int64:
		return lua.LNumber(float64(x))
	case int32:
		return lua.LNumber(x)
	case *float32:
		if x == nil {
			return lua.LNil
		}
		return lua.LNumber(*x)
	case *float64:
		if x == nil {
			return lua.LNil
		}
		return lua.LNumber(*x)
	case uint32:
		return lua.LNumber(x)
	case uint64:
		return lua.LNumber(float64(x))
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return lua.LNumber(f)
		}
		return lua.LString(x.String())
	case map[string]any:
		t := L.NewTable()
		for k, val := range x {
			t.RawSetString(k, goToLua(L, val))
		}
		return t
	case []any:
		t := L.NewTable()
		for i, val := range x {
			t.RawSetInt(i+1, goToLua(L, val))
		}
		return t
	case []map[string]any:
		t := L.NewTable()
		for i, val := range x {
			t.RawSetInt(i+1, goToLua(L, val))
		}
		return t
	case []string:
		t := L.NewTable()
		for i, val := range x {
			t.RawSetInt(i+1, lua.LString(val))
		}
		return t
	default:
		// Protocol values include narrower numbers, optional pointers, and
		// fixed-size vectors. Keep their types without a JSON round trip.
		value := reflect.ValueOf(v)
		switch value.Kind() {
		case reflect.Pointer:
			if value.IsNil() {
				return lua.LNil
			}
			return goToLua(L, value.Elem().Interface())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return lua.LNumber(value.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return lua.LNumber(value.Uint())
		case reflect.Float32, reflect.Float64:
			return lua.LNumber(value.Float())
		case reflect.String:
			return lua.LString(value.String())
		case reflect.Bool:
			return lua.LBool(value.Bool())
		case reflect.Array, reflect.Slice:
			t := L.NewTable()
			for i := 0; i < value.Len(); i++ {
				t.RawSetInt(i+1, goToLua(L, value.Index(i).Interface()))
			}
			return t
		case reflect.Map:
			if value.Type().Key().Kind() == reflect.String {
				t := L.NewTable()
				iter := value.MapRange()
				for iter.Next() {
					t.RawSetString(iter.Key().String(), goToLua(L, iter.Value().Interface()))
				}
				return t
			}
		}
		return lua.LString(fmt.Sprintf("%v", x))
	}
}

// argsToTable 把 Go []any 转为 Lua 表（1 起索引）。
func argsToTable(L *lua.LState, args []any) *lua.LTable {
	tbl := L.NewTable()
	for i, a := range args {
		tbl.RawSetInt(i+1, toLuaValue(L, a))
	}
	return tbl
}

// toLuaValue 把 Go 值转成 Lua 值。
func toLuaValue(L *lua.LState, v any) lua.LValue {
	return goToLua(L, v)
}

// luaToGo 把 Lua 值转成 Go any（table/number/bool/string），供写入 JSON 配置与 web 数据。
// 表若键为连续整数 1..n 则转为数组（web 表格行数据依赖数组语义），否则转为对象。
func luaToGo(v lua.LValue) any {
	switch x := v.(type) {
	case *lua.LTable:
		if arr, isArr := luaTableToArray(x); isArr {
			return arr
		}
		out := map[string]any{}
		x.ForEach(func(k, val lua.LValue) {
			out[lua.LVAsString(k)] = luaToGo(val)
		})
		return out
	case lua.LNumber:
		return float64(x)
	case lua.LBool:
		return bool(x)
	case lua.LString:
		return string(x)
	default:
		return nil
	}
}

// luaTableToArray 若表键全部为连续整数 1..n 则转为 []any 并返回 true；空表、对象表或混合键表返回 false。
func luaTableToArray(t *lua.LTable) ([]any, bool) {
	n := t.Len()
	if n == 0 {
		return nil, false
	}
	arr := make([]any, 0, n)
	for i := 1; i <= n; i++ {
		v := t.RawGetInt(i)
		if v == lua.LNil {
			return nil, false
		}
		arr = append(arr, luaToGo(v))
	}
	// 混合键表不得当数组处理：除 1..n 整数键外的任何键都会被静默丢弃
	extra := 0
	t.ForEach(func(k, _ lua.LValue) {
		if kn, ok := k.(lua.LNumber); ok {
			i := int(kn)
			if i >= 1 && i <= n && kn == lua.LNumber(i) {
				return
			}
		}
		extra++
	})
	return arr, extra == 0
}

// multiScoreToTable 把 GetMultiScore 的 map 转成 Lua 表。
func multiScoreToTable(L *lua.LState, m map[string]map[string]int) *lua.LTable {
	outer := L.NewTable()
	for player, boards := range m {
		inner := L.NewTable()
		for board, score := range boards {
			inner.RawSetString(board, lua.LNumber(score))
		}
		outer.RawSetString(player, inner)
	}
	return outer
}

// LoadLuaPluginSource 加载一段 Lua 插件源码（注册 game.* 并执行）。
func LoadLuaPluginSource(bm *BotManager, src string) error {
	return LoadLuaPlugin(bm, "", src)
}

// LoadLuaPlugin 加载一段 Lua 插件源码，并用 id 标记其触发项（用于生命周期管理/移除）。
func LoadLuaPlugin(bm *BotManager, id, src string, sourceDirs ...string) error {
	rememberPluginSource(id, src, sourceDirs...)
	return yyLoadResident(bm, id, src, false, sourceDirs...)
}
