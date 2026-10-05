package main

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	lua "github.com/yuin/gopher-lua"
)

var yyExtraMethods = map[string]string{
	"events.onSwing":            "handler(event)；服务器AnimateActionSwingArm下发时触发，含runtime_id/player/source。挥臂不证明命中了目标，服务器未广播则无法观测。",
	"events.onEntitySpawn":      "handler(event)；AddPlayer/AddActor/AddItemActor下发时触发，含实体类型、位置和原始fields。服务器视距内的出现，不代表世界内首次出生；血量仅取真实同步属性。",
	"events.onEntityRemove":     "handler(event)；RemoveActor下发时触发，含unique_id和已知runtime_id。消失不等于死亡，也可能是离开视距。",
	"events.onItemPickup":       "handler(event)；TakeItemActor下发时触发，含拾取实体和掉落物runtime_id、已观测item/position。拾取者可为生物；不凭RemoveActor猜拾取。",
	"game.sendPacket":           "typeNameOrId, fields?；当前账号发送完整协议包。字段名和类型通过packet.describe查询。支持嵌套对象、向量数组、hex字节、十进制字符串64位ID；联合接口类型需高级动作或完整payload_hex。submitted不代表服务器接受，登录生命周期不能重发。",
	"game.sendCommandFast":      "command, options?；无等待提交，魔法/ai 默认使用 SettingsCommand 并保持机器人 @s/相对坐标；其他通道保持来源。rate_cps 默认0，不逐条等回包。confirmed=false 不能当游戏成功。",
	"game.sendCommandsFast":     "commands数组, options?；一次插件宿主边界按顺序提交完整数组，不限整数组行数，不等待逐条回包。先校验全部命令；中途网络失败报告已提交数量，不自动重发，不保证同一服务器Tick或原子执行。",
	"game.sendCommandConfirmed": "command, options?；game.execute别名，等待真实CommandOutput，超时为未知，不自动重发。只在依赖前条结果时使用。",
	"game.scene":                "无参数。当前账号的机器人和已观测实体快照；含 known、stale、数据时间和来源。AI 没有独立游戏实体，bot 是当前账号控制的机器人。",
	"game.executeBatch":         "commands 字符串数组，options：channel=default/ai/player/console（或0/1/2字符串），mode=ordered/pipeline/submit/burst/stream，transport=command_request/settings/pyrpc（后两者仅submit，无逐条确认），window=1..16条在途命令；burst最多1024条整批协议发送，512KiB文本，默认无固定CPS等待，不保证同Tick，timeout_ms=100..10000。支持每实例默认配置与单次覆盖。ordered 逐条等待；pipeline 仅用于互不依赖的命令，不保证服务端完成顺序；submit 不等待结果。结果逐项含 submitted/confirmed/success/error/channel，超时未知不自动重发。stream单段最多4096条/512KiB，动态在途窗口最多1024，rate_cps=0无固定CPS，网络未知停止补发，正常无目标继续无条件后续；完整说明读 command-execution 技能。",
	"player.health":             "target 为 self、玩家名或 runtime_id 字符串，默认 self。返回已观测信息含 health_known/health/max_health/health_time/stale，未知为 null，不推算其他玩家满血。仅能获取服务器给本机器人同步的属性。",
	"player.position":           "target 同 player.health。返回 position_known/position/position_time/dimension/stale；坐标为收到服务器包的观测，不把本地移动请求当服务器确认。",
	"player.nearby":             "radius=0..512，默认 32；按机器人已观测位置计算同维度、仍存在的玩家距离并排序。不包括客户端不可见或未知坐标的玩家，机器人坐标未知报错。",
	"bot.self":                  "无参数。读取当前账号机器人的服务器观测状态，health/position/dead 可未知。AI Agent 使用此机器人，不自行拥有一个游戏位置。",
	"bot.startMovement":         "dx,dy,dz 玩家输入方向，每轴 -1..1；以 50ms PlayerAuthInput 心跳交给服务器驱动运动。dx 左右，dz=-1 前进，dy 上下，不是世界坐标路径规划；返回 submitted=true,confirmed=false。",
	"bot.stopMovement":          "无参数。清理当前机器人移动输入，仍需要服务器回包确认最终位置。",
	"bot.lookAt":                "x,y,z 世界坐标。由已观测机器人位置计算 yaw/pitch，启用心跳发送视角；返回请求状态，无自动寻路/穿墙保证。",
	"bot.teleport":              "x,y,z 世界坐标。执行服务器 tp @s 并等待真实命令结果，需要对应权限；没有权限时报错或 success=false。",
	"world.breakBlock":          "x,y,z 整数坐标，mode=command 或 creative_action，默认 command。command 用 setblock ... air destroy 等待实际结果；creative_action 发创造模式破坏动作，要求游戏模式和六格范围，不代表服务器确认。生存挖掘的装备/耗时/交易流程尚不支持。",
	"world.placeBlock":          "x,y,z 整数坐标，block 例如 minecraft:stone。用服务器 setblock 指令，返回真实结果，需要权限。",
	"world.blockState":          "x,y,z 整数坐标。返回当前维度 layer=0 的最近 UpdateBlock 观测，未知 known=false；包含方块 runtime_id，不把任何更新都认定是玩家破坏。",
	"events.onDeath":            "handler 回调接收一个事件对象，来源为 ActorEventDeath/InstantDeath 或 health<=0；同一已死亡状态去重，重生后可再触发。不推测死亡原因/攻击者。",
	"events.onHealthChange":     "handler(event)，仅在观测血量初始值或变化时入队，含 previous/health/max_health/time/source。",
	"events.onHurt":             "handler(event)，来源为血量下降或 ActorEventHurt，两种独立来源可能各触发一次；无法识别准确攻击者，不用编造 cause。",
	"events.onMove":             "handler(event)，服务器实际位置初始值/变化时触发；含 previous_known/position/dimension/source。高频事件可用抓包抽样过滤减少负担。",
	"events.onRespawn":          "handler(event)，只处理服务端 RespawnStateReadyToSpawn，清除旧血量等待新属性。",
	"events.onBlockChange":      "handler(event)，UpdateBlock 导出 position/runtime_id/previous/cause_known=false；不能仅从方块更新认定破坏者。",
	"events.subscribeFiltered":  "packet_type 或 *，filter={fields:{字段路径:精确值},every_ms:0..60000}，handler。Python 回调接收包对象；C++/Lua 使用已注册函数名。* 接收当前账号订阅包，事件字段 _packet_type 表示真实类型，停止自动退订。",
	"events.captureStart":       "config={types:[包名或*],exclude:[包名],filter:{fields:{...},every_ms:...},capacity:1..2048,max_bytes:512..65536}。开启账号级解码入站游戏抓包；默认血量/死亡/位置/方块包，认证握手不记录，* 默认不包含大区块包。",
	"events.captureStop":        "停止记录，保留已抓数据。当前账号的多个插件共用账号级记录；使用时协调抓包开关。",
	"events.captureRead":        "after_sequence=0,limit=256，返回记录/游标/覆盖和抽样统计。缓冲有界，单条超大小截为预览；不是手机所有 App 的网络抓包。",
	"events.captureClear":       "清空当前账号抓包缓冲，保持抓包开关。",
	"events.packetSchema":       "无参数。返回当前内核支持包类型、协议 ID、顶层字段名与类型，供 AI 根据实际协议找包而非猜字段。",
}
var yySenseEvents = map[string]string{"events.onDeath": "death", "events.onHealthChange": "health", "events.onHurt": "hurt", "events.onMove": "move", "events.onRespawn": "respawn", "events.onBlockChange": "block_change", "events.onSwing": "swing", "events.onEntitySpawn": "entity_spawn", "events.onEntityRemove": "entity_remove", "events.onItemPickup": "item_pickup"}

type yySenseLuaHook struct {
	Bot                   *BotManager
	Plugin, Kind, Handler string
}

var yySenseHooks = struct {
	sync.Mutex
	Items []yySenseLuaHook
}{}
var yySenseLuaPackets = struct {
	sync.Mutex
	Items map[yyLuaKey][]func()
}{Items: map[yyLuaKey][]func(){}}

func yySenseLuaClear(bm *BotManager, id string) {
	yySenseLuaPackets.Lock()
	key := yyLuaKey{bm, id}
	unsubs := yySenseLuaPackets.Items[key]
	delete(yySenseLuaPackets.Items, key)
	yySenseLuaPackets.Unlock()
	for _, u := range unsubs {
		u()
	}
	yySenseHooks.Lock()
	defer yySenseHooks.Unlock()
	rows := yySenseHooks.Items[:0]
	for _, h := range yySenseHooks.Items {
		if h.Bot != bm || h.Plugin != id {
			rows = append(rows, h)
		}
	}
	yySenseHooks.Items = rows
}
func yySenseLuaEmit(bm *BotManager, e map[string]any) {
	yySenseHooks.Lock()
	rows := append([]yySenseLuaHook{}, yySenseHooks.Items...)
	yySenseHooks.Unlock()
	for _, h := range rows {
		if h.Bot != bm || h.Kind != e["type"] {
			continue
		}
		yyLuaMu.Lock()
		r := yyLuaStates[yyLuaKey{bm, h.Plugin}]
		yyLuaMu.Unlock()
		if r == nil {
			continue
		}
		yyQueueSenseLua(r, bm, h.Plugin, h.Handler, []any{e})
	}
}
func registerSenseAPI(L *lua.LState) {
	L.SetGlobal("chains", L.NewTable())
	for method := range yyExtraMethods {
		if method == "game.executeBatch" || method == "game.sendCommandFast" || method == "game.sendCommandsFast" || method == "game.sendCommandConfirmed" {
			continue
		} // registered with instance command defaults
		parts := strings.Split(method, ".")
		table, ok := L.GetGlobal(parts[0]).(*lua.LTable)
		if !ok {
			continue
		}
		method := method
		L.SetField(table, parts[1], L.NewFunction(func(L *lua.LState) int {
			args := []any{}
			for i := 1; i <= L.GetTop(); i++ {
				args = append(args, luaToGo(L.Get(i)))
			}
			if kind, ok := yySenseEvents[method]; ok {
				pid := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
				if pid == "" {
					L.RaiseError("需插件上下文")
				}
				h := yySenseLuaHook{abot(L), pid, kind, L.CheckString(1)}
				yySenseHooks.Lock()
				exists := false
				for _, old := range yySenseHooks.Items {
					if old == h {
						exists = true
					}
				}
				if !exists {
					yySenseHooks.Items = append(yySenseHooks.Items, h)
				}
				yySenseHooks.Unlock()
				L.Push(lua.LTrue)
				return 1
			}
			if method == "events.subscribeFiltered" {
				pid := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
				kind := L.CheckString(1)
				handler := L.CheckString(3)
				if pid == "" || !yyPacketTypeKnown(kind) {
					L.RaiseError("需插件上下文及真实包类型")
					return 0
				}
				filter := yyPacketFilter{}
				encoded, _ := json.Marshal(args[1])
				if e := json.Unmarshal(encoded, &filter); e != nil {
					L.RaiseError("%v", e)
					return 0
				}
				if e := yyValidateFilter(filter); e != nil {
					L.RaiseError("%v", e)
					return 0
				}
				bm := abot(L)
				var stopped atomic.Bool
				var mu sync.Mutex
				var last time.Time
				u := packetHubSubscribe(kind, func(b *BotManager, data map[string]any) {
					if stopped.Load() || b != bm || !yyFilterMatch(filter, data) {
						return
					}
					mu.Lock()
					if time.Since(last) < time.Duration(filter.EveryMS)*time.Millisecond {
						mu.Unlock()
						return
					}
					last = time.Now()
					mu.Unlock()
					yyLuaMu.Lock()
					resident := yyLuaStates[yyLuaKey{bm, pid}]
					yyLuaMu.Unlock()
					if resident == nil {
						return
					}
					yyQueueSenseLua(resident, bm, pid, handler, []any{data})
				})
				yySenseLuaPackets.Lock()
				key := yyLuaKey{bm, pid}
				yySenseLuaPackets.Items[key] = append(yySenseLuaPackets.Items[key], func() { stopped.Store(true); u() })
				yySenseLuaPackets.Unlock()
				L.Push(lua.LTrue)
				return 1
			}
			v, e := yySenseCall(abot(L), method, args)
			if e != nil {
				L.RaiseError("%s: %v", method, e)
				return 0
			}
			L.Push(goToLua(L, v))
			return 1
		}))
	}
}
func yyArgString(args []any, index int, def string) (string, error) {
	if len(args) <= index {
		return def, nil
	}
	v, ok := args[index].(string)
	if !ok {
		return "", fmt.Errorf("参数 %d 需要字符串", index+1)
	}
	return v, nil
}
func yyArgNumber(args []any, index int, def float64) (float64, error) {
	if len(args) <= index {
		return def, nil
	}
	v, ok := args[index].(float64)
	if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("参数 %d 需要有限数字", index+1)
	}
	return v, nil
}
func yySenseVec(args []any) (mgl32.Vec3, error) {
	var p mgl32.Vec3
	for i := 0; i < 3; i++ {
		if len(args) <= i {
			return p, fmt.Errorf("需要 x,y,z")
		}
		n, e := yyArgNumber(args, i, 0)
		if e != nil || math.Abs(n) > 30000000 {
			return p, fmt.Errorf("坐标/方向无效")
		}
		p[i] = float32(n)
	}
	return p, nil
}
func yySenseCall(bm *BotManager, method string, args []any) (any, error) {
	if method == "plugin.capabilities" {
		return yyCapabilities(), nil
	}
	if method == "plugin.has" {
		name, e := yyArgString(args, 0, "")
		if e != nil {
			return nil, e
		}
		return yyHasCapability(name), nil
	}
	if _, ok := yyResourceDocs[method]; ok {
		return nil, fmt.Errorf("资源API需要插件上下文，请通过插件或 sdk/call 的 plugin_id/account 调用")
	}
	if _, ok := yySchedulerDocs[method]; ok {
		return nil, fmt.Errorf("调度API需要插件上下文，请通过插件或 sdk/call 的 plugin_id/account 调用")
	}
	if method == "plugin.stats" {
		return nil, fmt.Errorf("stats需要插件上下文")
	}
	if method == "game.sendCommandFast" || method == "game.sendCommandsFast" || method == "game.sendCommandConfirmed" {
		return yyPluginCommandCall(bm, method, args, yyDefaultCommandOptions())
	}
	if strings.HasPrefix(method, "chains.") {
		return yyChainPluginCall(bm, method, args)
	}
	if method == "bot.attack" || method == "world.mineBlock" || method == "world.useBlock" {
		return yyPhysicalAction(bm, method, args)
	}
	switch method {
	case "game.sendPacket":
		return yySendPacket(bm, args)
	case "game.scene":
		return yySenseSnapshot(bm), nil
	case "bot.self", "player.health", "player.position":
		target, e := yyArgString(args, 0, "self")
		if e != nil {
			return nil, e
		}
		if method == "bot.self" {
			target = "self"
		}
		return yySenseActor(bm, target), nil
	case "events.packetSchema":
		return yyPacketSchema(), nil
	case "events.captureRead":
		after, e := yyArgNumber(args, 0, 0)
		if e != nil || after < 0 || after > float64(1<<53) {
			return nil, fmt.Errorf("无效游标")
		}
		limit, e := yyArgNumber(args, 1, 256)
		if e != nil || limit < 1 || limit > 2048 {
			return nil, fmt.Errorf("limit 需要 1..2048")
		}
		return yyCaptureRead(bm, uint64(after), int(limit)), nil
	case "events.captureStart":
		cfg := yyCaptureConfig{}
		if len(args) > 0 {
			data, e := json.Marshal(args[0])
			if e != nil {
				return nil, e
			}
			if e = json.Unmarshal(data, &cfg); e != nil {
				return nil, e
			}
		}
		return yyCaptureStart(bm, cfg)
	case "events.captureStop", "events.captureClear":
		s := yySenseFor(bm)
		s.Lock()
		if method == "events.captureStop" {
			s.Capture.Active = false
		} else {
			s.Capture.Records = []map[string]any{}
			s.Capture.Dropped = 0
		}
		s.Unlock()
		return yyCaptureRead(bm, 0, 256), nil
	case "player.nearby":
		radius, e := yyArgNumber(args, 0, 32)
		if e != nil || radius < 0 || radius > 512 {
			return nil, fmt.Errorf("半径需要 0..512")
		}
		origin := yySenseActor(bm, "self")
		if origin["position_known"] != true || origin["connected"] != true {
			return nil, fmt.Errorf("机器人实际位置未知或未连接")
		}
		pos := origin["position"].(mgl32.Vec3)
		actors := yySenseSnapshot(bm)["actors"].([]map[string]any)
		rows := []map[string]any{}
		for _, a := range actors {
			if a["entity_type"] != "minecraft:player" || a["runtime_id"] == origin["runtime_id"] || a["position_known"] != true || a["present"] != true || a["dimension"] != origin["dimension"] || a["position_stale"] == true {
				continue
			}
			distance := float64(a["position"].(mgl32.Vec3).Sub(pos).Len())
			if distance <= radius {
				a["distance"] = distance
				rows = append(rows, a)
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i]["distance"].(float64) < rows[j]["distance"].(float64) })
		return rows, nil
	case "world.blockState":
		p, e := yySenseVec(args)
		if e != nil {
			return nil, e
		}
		for _, v := range p {
			if float32(int32(v)) != v {
				return nil, fmt.Errorf("方块坐标需要整数")
			}
		}
		if bm == nil {
			return map[string]any{"known": false}, nil
		}
		bm.mu.Lock()
		d := bm.dimension
		bm.mu.Unlock()
		s := yySenseFor(bm)
		s.Lock()
		v := s.Blocks[yyBlockKey(d, protocol.BlockPos{int32(p[0]), int32(p[1]), int32(p[2])}, 0)]
		s.Unlock()
		if v == nil {
			return map[string]any{"known": false, "reason": "未收到该方块 UpdateBlock"}, nil
		}
		return v, nil
	}
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("游戏账号未连接，不能执行 %s", method)
	}
	if method == "game.executeBatch" {
		return yyCommandBatch(bm, args)
	}
	if method == "bot.stopMovement" {
		if e := bm.StopMovement(); e != nil {
			return nil, e
		}
		return map[string]any{"submitted": true, "confirmed": false}, nil
	}
	p, e := yySenseVec(args)
	if e != nil {
		return nil, e
	}
	switch method {
	case "bot.startMovement":
		for _, v := range p {
			if math.Abs(float64(v)) > 1 {
				return nil, fmt.Errorf("方向每轴需 -1..1")
			}
		}
		bm.SetHeartbeatEnabled(true)
		if e = bm.StartMoveDirection(p[0], p[1], p[2]); e != nil {
			return nil, e
		}
		return map[string]any{"submitted": true, "confirmed": false, "space": "player_input"}, nil
	case "bot.lookAt":
		self := yySenseActor(bm, "self")
		if self["position_known"] != true {
			return nil, fmt.Errorf("机器人位置未知")
		}
		delta := p.Sub(self["position"].(mgl32.Vec3))
		if delta.Len() == 0 {
			return nil, fmt.Errorf("目标与机器人重合")
		}
		yaw := float32(math.Atan2(-float64(delta[0]), float64(delta[2])) * 180 / math.Pi)
		pitch := float32(-math.Atan2(float64(delta[1]), math.Hypot(float64(delta[0]), float64(delta[2]))) * 180 / math.Pi)
		bm.SetHeartbeatEnabled(true)
		if e = bm.SetRotation(pitch, yaw); e != nil {
			return nil, e
		}
		return map[string]any{"submitted": true, "confirmed": false, "pitch": pitch, "yaw": yaw}, nil
	case "bot.teleport":
		return bm.SendConsole(fmt.Sprintf("/tp @s %g %g %g", p[0], p[1], p[2]))
	case "world.breakBlock", "world.placeBlock":
		for _, v := range p {
			if float32(int32(v)) != v {
				return nil, fmt.Errorf("方块坐标需要整数")
			}
		}
		suffix := "minecraft:air destroy"
		if method == "world.placeBlock" {
			block, e := yyArgString(args, 3, "")
			if e != nil || !regexp.MustCompile(`^[a-z0-9_]+:[a-z0-9_]+$`).MatchString(block) {
				return nil, fmt.Errorf("需要 namespaced 方块名，例如 minecraft:stone")
			}
			suffix = block
		} else {
			mode, e := yyArgString(args, 3, "command")
			if e != nil {
				return nil, e
			}
			if mode == "creative_action" {
				bm.mu.Lock()
				conn := bm.conn
				id := bm.runtimeID
				gameMode := bm.gameMode
				bm.mu.Unlock()
				if conn == nil || gameMode != 1 {
					return nil, fmt.Errorf("需要创造模式；生存挖掘交易尚未实现")
				}
				self := yySenseActor(bm, "self")
				if self["position_known"] != true || p.Sub(self["position"].(mgl32.Vec3)).Len() > 6 {
					return nil, fmt.Errorf("需已知机器人位置且目标在六格内")
				}
				e = conn.WritePacket(&packet.PlayerAction{EntityRuntimeID: id, ActionType: protocol.PlayerActionCreativePlayerDestroyBlock, BlockPosition: protocol.BlockPos{int32(p[0]), int32(p[1]), int32(p[2])}})
				if e != nil {
					return nil, e
				}
				return map[string]any{"submitted": true, "confirmed": false, "completion": "observe world.blockState/events.onBlockChange"}, nil
			}
			if mode != "command" {
				return nil, fmt.Errorf("mode 仅支持 command/creative_action；生存挖掘尚未实现")
			}
		}
		return bm.SendConsole(fmt.Sprintf("/setblock %d %d %d %s", int32(p[0]), int32(p[1]), int32(p[2]), suffix))
	}
	return nil, fmt.Errorf("未知感知方法 %s", method)
}
func (s *yySDKContext) senseCall(method string, args []any) (any, error) {
	if strings.HasPrefix(method, "events.capture") && s.Instance.Account != "main" && yyAccountBot(s.Instance.Account) == nil {
		return nil, fmt.Errorf("目标账号不存在")
	}
	if kind, ok := yySenseEvents[method]; ok {
		h, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		s.addListener(yySDKListener{Kind: kind, Handler: h})
		return true, nil
	}
	if method == "events.subscribeFiltered" {
		kind, e := yySDKString(args, 0)
		if e != nil || !yyPacketTypeKnown(kind) {
			return nil, fmt.Errorf("未知包类型")
		}
		if len(args) != 3 {
			return nil, fmt.Errorf("subscribeFiltered(type,filter,handler)")
		}
		h, e := yySDKString(args, 2)
		if e != nil {
			return nil, e
		}
		filter := yyPacketFilter{}
		data, e := json.Marshal(args[1])
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(data, &filter); e != nil {
			return nil, e
		}
		if e = yyValidateFilter(filter); e != nil {
			return nil, e
		}
		var mu sync.Mutex
		var last time.Time
		s.Mu.Lock()
		defer s.Mu.Unlock()
		if s.Closed {
			return nil, fmt.Errorf("插件已停止")
		}
		u := packetHubSubscribe(kind, func(bm *BotManager, data map[string]any) {
			if bm != yyAccountBot(s.Instance.Account) || !yyFilterMatch(filter, data) {
				return
			}
			mu.Lock()
			if time.Since(last) < time.Duration(filter.EveryMS)*time.Millisecond {
				mu.Unlock()
				return
			}
			last = time.Now()
			mu.Unlock()
			s.enqueue(h, []any{data})
		})
		s.Unsubs = append(s.Unsubs, u)
		return true, nil
	}
	return yySenseCall(yyAccountBot(s.Instance.Account), method, args)
}
