package main

import (
	"encoding/json"
	"fmt"
)

// toolGameCall 把 game.* 函数名映射到 BotManager 方法执行（完整 game API）。
// 前置要求：机器人已连接即可（不校验 OP——OP 检测在网易服不可靠，会误拦真 OP 的机器人）。
func toolGameCall(args map[string]any) (string, bool) {
	fn := aiArgStr(args, "function")
	if fn == "" {
		return "参数 function 不能为空（如 sendCommand / getPlayerPos）", true
	}
	if _, ok := yyExtraMethods[fn]; ok {
		argv, _ := args["args"].([]any)
		value, e := yySenseCall(getBotManagerLocked(), fn, argv)
		if e != nil {
			return e.Error(), true
		}
		return mustJSON(value), false
	}
	if !botOnline() {
		return "机器人未连接，无法调用 game API", true
	}
	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()
	if bm == nil {
		return "机器人不可用", true
	}

	var argv []any
	if a, ok := args["args"].([]any); ok {
		argv = a
	}

	switch fn {
	// ── 指令与消息 ──
	case "sendCommand", "execute", "executeBatch":
		result, err := yyPluginCommandCall(bm, "game."+fn, argv, yyDefaultCommandOptions())
		if err != nil {
			return err.Error(), true
		}
		return mustJSON(result), false
	case "sendChat":
		return twoArgsErr(bm.SendChat(argString(argv, 0), argString(argv, 1)))
	case "sendTitle":
		return threeArgsErr(bm.SendTitle(argString(argv, 0), argString(argv, 1), argString(argv, 2)))
	case "sendActionbar":
		return twoArgsErr(bm.SendActionbar(argString(argv, 0), argString(argv, 1)))
	case "sendRichTellraw":
		return oneArgErr(bm.SendRichTellraw(argString(argv, 0)))
	case "sendTargeted":
		return twoArgsErr(bm.sendTargeted(argString(argv, 0), argString(argv, 1)))

	// ── 查询 ──
	case "getPlayerPos":
		p, err := bm.GetPlayerPos(argString(argv, 0))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		return fmt.Sprintf(`{"x":%v,"y":%v,"z":%v,"dimension":%d}`, p.Position.X, p.Position.Y, p.Position.Z, p.Dimension), false
	case "getPlayerPosXYZ":
		x, y, z, err := bm.GetPlayerPosXYZ(argString(argv, 0))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		return fmt.Sprintf(`{"x":%v,"y":%v,"z":%v}`, x, y, z), false
	case "getScore":
		s, err := bm.GetScore(argString(argv, 0), argString(argv, 1))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		return fmt.Sprintf("%d", s), false

	// ── 记分板 ──
	case "getMultiScore":
		ms, err := bm.GetMultiScore(argString(argv, 0))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		j, _ := json.Marshal(ms)
		return string(j), false
	case "setScore":
		board, target := argString(argv, 0), argString(argv, 1)
		score := argInt(argv, 2)
		cmd := fmt.Sprintf("scoreboard players set %s %s %d", target, board, score)
		return oneArgErr(bm.SendCommandNoWait(ChannelAI, cmd))

	// ── 物品 / 背包 ──
	case "getPlayerItem":
		n, err := bm.GetPlayerItem(argString(argv, 0), argString(argv, 1), argInt(argv, 2))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		return fmt.Sprintf("%d", n), false
	case "queryInventory":
		inv, err := bm.QueryPlayerInventory(argString(argv, 0))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		j, _ := json.Marshal(inv)
		return string(j), false

	// ── 玩家 / 区域 ──
	case "getTarget":
		names, err := bm.GetTarget(argString(argv, 0))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		j, _ := json.Marshal(names)
		return string(j), false
	case "getTickingAreaList":
		tal, err := bm.GetTickingAreaList()
		if err != nil {
			return "失败: " + err.Error(), true
		}
		j, _ := json.Marshal(tal)
		return string(j), false

	// ── 命令回读 ──
	case "isCmdSuccess":
		ok, err := bm.IsCmdSuccess(argString(argv, 0))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		return fmt.Sprintf("%v", ok), false
	case "getBlockTile":
		x, y, z := argInt(argv, 0), argInt(argv, 1), argInt(argv, 2)
		name, err := bm.GetBlockTile(x, y, z)
		if err != nil {
			return "失败: " + err.Error(), true
		}
		return name, false
	case "getBlockTileSafe":
		x, y, z := argInt(argv, 0), argInt(argv, 1), argInt(argv, 2)
		name, err := bm.GetBlockTileSafe(x, y, z)
		if err != nil {
			return "失败: " + err.Error(), true
		}
		return name, false

	// ── 方块 NBT / 方块状态 读写（底层：nbt_world_api.go）──
	case "getBlockData":
		bd, err := bm.GetBlockData(argInt(argv, 0), argInt(argv, 1), argInt(argv, 2))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		j, _ := json.Marshal(bd)
		return string(j), false
	case "getBlockNBT":
		nbt, err := bm.GetBlockNBT(argInt(argv, 0), argInt(argv, 1), argInt(argv, 2))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		if nbt == nil {
			return "{}", false
		}
		j, _ := json.Marshal(nbt)
		return string(j), false
	case "getBlockState":
		name, states, err := bm.GetBlockState(argInt(argv, 0), argInt(argv, 1), argInt(argv, 2))
		if err != nil {
			return "失败: " + err.Error(), true
		}
		if states == nil {
			states = map[string]any{}
		}
		j, _ := json.Marshal(map[string]any{"name": name, "states": states})
		return string(j), false
	case "getRegionBlockData":
		x, y, z := argInt(argv, 0), argInt(argv, 1), argInt(argv, 2)
		sx, sy, sz := argInt(argv, 3), argInt(argv, 4), argInt(argv, 5)
		region, err := bm.GetRegionBlockData(
			[3]int32{int32(x), int32(y), int32(z)},
			[3]int32{int32(sx), int32(sy), int32(sz)})
		if err != nil {
			return "失败: " + err.Error(), true
		}
		j, _ := json.Marshal(region)
		return string(j), false
	case "setBlock":
		x, y, z := argInt(argv, 0), argInt(argv, 1), argInt(argv, 2)
		return oneArgErr(bm.SetBlock(x, y, z, argString(argv, 3), argString(argv, 4)))
	case "placeNBTBlock":
		x, y, z := argInt(argv, 0), argInt(argv, 1), argInt(argv, 2)
		name := argString(argv, 3)
		states := argString(argv, 4)
		var nbt map[string]any
		if len(argv) > 5 {
			if m, ok := argv[5].(map[string]any); ok {
				nbt = m
			}
		}
		return oneArgErr(bm.PlaceNBTBlock(x, y, z, name, states, nbt))
	case "setBlockState":
		x, y, z := argInt(argv, 0), argInt(argv, 1), argInt(argv, 2)
		var states map[string]any
		if len(argv) > 3 {
			if m, ok := argv[3].(map[string]any); ok {
				states = m
			}
		}
		if states == nil {
			return "setBlockState 需要 states 对象参数", true
		}
		return oneArgErr(bm.SetBlockState(x, y, z, states))
	case "getBotPos":
		m, _ := json.Marshal(bm.GetPosition())
		return string(m), false
	case "getTPS":
		return fmt.Sprintf("%.1f", bm.GetTPS()), false

	// ── 移动 / 飞行 ──
	case "teleport":
		x, y, z := argFloat(argv, 0), argFloat(argv, 1), argFloat(argv, 2)
		return oneArgErr(bm.TeleportTo(float32(x), float32(y), float32(z)))
	case "move":
		dx, dy, dz := argFloat(argv, 0), argFloat(argv, 1), argFloat(argv, 2)
		return oneArgErr(bm.MoveRelative(float32(dx), float32(dy), float32(dz)))
	case "rotate":
		pitch, yaw := argFloat(argv, 0), argFloat(argv, 1)
		sec := argFloat(argv, 2)
		if sec > 0 {
			// 在 sec 秒内平滑转到目标角度（yaw 走最短路径）
			return oneArgErr(bm.SmoothRotateTo(float32(pitch), float32(yaw), float32(sec)))
		}
		return oneArgErr(bm.SetRotation(float32(pitch), float32(yaw)))
	case "jump":
		return oneArgErr(bm.Jump())
	case "flyStart":
		return oneArgErr(bm.StartFlying())
	case "flyStop":
		return oneArgErr(bm.StopFlying())

	default:
		return fmt.Sprintf("未知 game 函数: %s", fn), true
	}
}

// 结果包装：无错误 → "OK"，有错误 → 错误文本。
func oneArgErr(err error) (string, bool) {
	if err != nil {
		return "失败: " + err.Error(), true
	}
	return "OK", false
}
func twoArgsErr(err error) (string, bool)   { return oneArgErr(err) }
func threeArgsErr(err error) (string, bool) { return oneArgErr(err) }

// ── 参数取值 ──
func argString(args []any, i int) string {
	if i < len(args) {
		if s, ok := args[i].(string); ok {
			return s
		}
	}
	return ""
}
func argInt(args []any, i int) int {
	if i < len(args) {
		switch v := args[i].(type) {
		case float64:
			return int(v)
		case json.Number:
			if n, err := v.Int64(); err == nil {
				return int(n)
			}
		case int:
			return v
		}
	}
	return 0
}
func argFloat(args []any, i int) float64 {
	if i < len(args) {
		switch v := args[i].(type) {
		case float64:
			return v
		case json.Number:
			if f, err := v.Float64(); err == nil {
				return f
			}
		case int:
			return float64(v)
		}
	}
	return 0
}
