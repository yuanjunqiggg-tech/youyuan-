package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func init() {
	for method, doc := range yyChainMethods {
		yyExtraMethods[method] = doc
	}
	yyExtraMethods["bot.attack"] = "runtime_id 字符串,slot=0；真实近战库存交易，只允许当前服务器观测的六格内实体；slot 为 0..8。返回 submitted，伤害必须由服务器事件确认。"
	yyExtraMethods["world.mineBlock"] = "x,y,z,duration_ms=1000,slot=0,face=1；生存挖掘尝试，真实开始/持续/结束动作与手持库存交易，按 100ms 发送。需要最近方块观测；服务端可拒绝，返回 submitted/confirmed。硬度、工具正确与否由服务器判断。"
	yyExtraMethods["world.useBlock"] = "x,y,z,face=1,slot=0；真实手持物品点击指定方块面（可放置、开容器），需要已观测目标方块和六格范围。不是 setblock；返回提交状态，消耗与放置观察服务器。"
}

// Plugins use the same parser, persistence and runner as the command-chain UI.
var yyChainMethods = map[string]string{
	"chains.reload":     "name；只重新读取/编译TXT资源，当前轮使用原IR，下轮用新IR；不重启插件、保留启停与统计；无效修改保留原链。",
	"chains.list":       "无参数，列出当前账号的链、配置、计数、错误和状态。",
	"chains.get":        "name，读取完整 TXT、配置和状态。",
	"chains.export":     "name，返回完整 TXT 与配置，插件可自行写入导出目录。",
	"chains.validate":   "content，预解析并返回命令/条件/通道/行号；失败含报错。",
	"chains.save":       "name,content,config；config 支持 hz(模块频率上限，有限正数；TXT较慢模块保持原频率)/interval_ms(兼容毫秒周期；单独修改时同步hz)/delay_ms/run_count/channel/enabled/mode/window/rate_cps。mode=fast同账号模块串行有界提交（每批最多1024包/512KiB命令文本，业务未逐条确认；网络写失败停止本连接后续Fast链，不自动重发，重连后恢复；提交成功不代表服务器执行或TPS正常）、remote严格顺序、pipeline有界流水线、burst整批最多1024、stream动态滑动窗口1..1024（默认128，rate_cps=0不设固定CPS，回包立即补位，网络错误停止补发；正常无目标不丢弃无条件后续）；全部在机器人内运行。TXT支持 @branch {command,then,else} 一次真实检测选择一个子链并返回，@call JSON 跨链源文件行号范围/次数/时限/持续，@wait JSON 条件边沿等待，@pause手动触发；读 command-chain 技能。账号固定为当前插件账号。",
	"chains.configure":  "name,patch，合并并校验配置，保留 TXT；运行中会停止并按 enabled 重启。",
	"chains.start":      "name，启用独立常驻实例；已启用不重复启动。",
	"chains.stop":       "name，停止并等待当前实例退出。",
	"chains.trigger":    "name，释放当前链的 @pause 等待点一次；没有等待点时不会自动重启已停止的链。",
	"chains.delete":     "name，先停止再删除 TXT 与配置。",
	"chains.menu":       "player,page=1，仅向该玩家发分页聊天菜单；不会阻塞等待消息。",
	"chains.handleChat": "player,text；处理 .命令选择 与 .链 列表/启动/停止/次数/频率/通道/源码/删除。修改要求玩家 OP，返回 handled。配合 events.onChat。",
}

func yyChainResult(method string, body any, query map[string]any) (map[string]any, error) {
	_, raw, e := yyInvoke(method, "/api/youyuan/chains", query, body)
	if e != nil {
		return nil, e
	}
	var out map[string]any
	if e = json.Unmarshal(raw, &out); e != nil {
		return nil, e
	}
	if out["ok"] == false {
		return nil, fmt.Errorf("%v", out["error"])
	}
	return out, nil
}
func yyChainOwns(bm *BotManager, c yyChainConfig) bool {
	if c.Account == "" || c.Account == "main" {
		return bm == getBotManagerLocked()
	}
	return bm != nil && yyAccountBot(c.Account) == bm
}
func yyChainRowsFor(bm *BotManager) ([]map[string]any, error) {
	result, e := yyChainResult("GET", nil, nil)
	if e != nil {
		return nil, e
	}
	rows := []map[string]any{}
	for _, item := range result["files"].([]any) {
		row := item.(map[string]any)
		var cfg yyChainConfig
		raw, _ := json.Marshal(row["config"])
		_ = json.Unmarshal(raw, &cfg)
		if yyChainOwns(bm, cfg) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}
func yyChainPluginCall(bm *BotManager, method string, args []any) (any, error) {
	if method == "chains.handleChat" {
		player, e := yyArgString(args, 0, "")
		if e != nil {
			return nil, e
		}
		msg, e := yyArgString(args, 1, "")
		if e != nil {
			return nil, e
		}
		return yyChainChat(bm, player, msg)
	}
	if method == "chains.menu" {
		player, e := yyArgString(args, 0, "")
		if e != nil || player == "" {
			return nil, fmt.Errorf("需要玩家名")
		}
		page, e := yyArgNumber(args, 1, 1)
		if e != nil || page < 1 || page > 100000 {
			return nil, fmt.Errorf("页码无效")
		}
		return yyChainMenu(bm, player, int(page))
	}
	rows, e := yyChainRowsFor(bm)
	if e != nil {
		return nil, e
	}
	if method == "chains.list" {
		return map[string]any{"files": rows}, nil
	}
	if method == "chains.validate" {
		content, e := yyArgString(args, 0, "")
		if e != nil {
			return nil, e
		}
		return yyChainResult("POST", map[string]any{"action": "preview", "name": "preview.txt", "content": content}, nil)
	}
	name, e := yyArgString(args, 0, "")
	if e != nil {
		return nil, e
	}
	if _, e = yyChainName(name); e != nil {
		return nil, e
	}
	var row map[string]any
	for _, r := range rows {
		if r["config"].(map[string]any)["name"] == name {
			row = r
			break
		}
	}
	if row == nil {
		yyChains.Lock()
		other := yyChains.items[name] != nil
		yyChains.Unlock()
		if other {
			return nil, fmt.Errorf("该链属于其他账号")
		}
		if method != "chains.save" {
			return nil, fmt.Errorf("命令链不存在")
		}
	}
	switch method {
	case "chains.get", "chains.export":
		value, e := yyChainResult("GET", nil, map[string]any{"name": name})
		if e != nil {
			return nil, e
		}
		value["config"] = row["config"]
		value["status"] = row
		return value, nil
	case "chains.save", "chains.configure":
		config := map[string]any{"name": name, "account": "main", "interval_ms": 1000, "run_count": 0, "enabled": false}
		if bm != getBotManagerLocked() && bm != nil {
			config["account"] = bm.idStr
		}
		content := ""
		patchIndex := 2
		if row != nil {
			config = row["config"].(map[string]any)
		}
		if method == "chains.configure" {
			value, e := yyChainResult("GET", nil, map[string]any{"name": name})
			if e != nil {
				return nil, e
			}
			content = value["content"].(string)
			patchIndex = 1
		} else {
			content, e = yyArgString(args, 1, "")
			if e != nil {
				return nil, e
			}
		}
		if len(args) > patchIndex {
			patch, ok := args[patchIndex].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("配置需要对象")
			}
			for k, v := range patch {
				switch k {
				case "hz", "interval_ms", "delay_ms", "run_count", "channel", "enabled", "mode", "window", "rate_cps", "scheduling", "bootstrap":
					config[k] = v
				default:
					return nil, fmt.Errorf("不支持配置字段 %s", k)
				}
			}
			// Old clients edit milliseconds only: do not retain a stale Hz value.
			if interval, changed := patch["interval_ms"]; changed {
				if _, explicit := patch["hz"]; !explicit {
					raw, err := json.Marshal(interval)
					var ms float64
					if err != nil || json.Unmarshal(raw, &ms) != nil || ms <= 0 {
						return nil, fmt.Errorf("interval_ms 必须是正数")
					}
					config["hz"] = 1000 / ms
				}
			}
		}
		return yyChainResult("POST", map[string]any{"action": "save", "name": name, "content": content, "config": config}, nil)
	case "chains.start", "chains.stop", "chains.delete", "chains.trigger", "chains.reload":
		return yyChainResult("POST", map[string]any{"action": strings.TrimPrefix(method, "chains."), "name": name}, nil)
	}
	return nil, fmt.Errorf("未知命令链接口 %s", method)
}
func yyChainMenu(bm *BotManager, player string, page int) (any, error) {
	if player == "" || len(player) > 128 || strings.HasPrefix(player, "@") || strings.ContainsAny(player, "\"\\\r\n") {
		return nil, fmt.Errorf("需要明确且有效的玩家名")
	}
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("游戏未连接")
	}
	rows, e := yyChainRowsFor(bm)
	if e != nil {
		return nil, e
	}
	pages := (len(rows) + 7) / 8
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
	}
	lines := []string{fmt.Sprintf("§6命令链 %d/%d§r", page, pages)}
	for i := (page - 1) * 8; i < len(rows) && i < page*8; i++ {
		r := rows[i]
		lines = append(lines, fmt.Sprintf("%d. %s · %v", i+1, r["config"].(map[string]any)["name"], r["state"]))
	}
	lines = append(lines, ".链 列表 页码 | .链 启动/停止 序号或文件名", ".链 次数 序号 N（0持续） | .链 频率 序号 毫秒", ".链 通道 序号 ai/player/console | .链 源码/删除 序号")
	cmd := fmt.Sprintf("tellraw %s %s", toPlayerSelector(player), mustJSON(map[string]any{"rawtext": []any{map[string]any{"text": strings.Join(lines, "\n")}}}))
	r, e := bm.SendConsole("/" + cmd)
	if e != nil {
		return nil, e
	}
	return map[string]any{"page": page, "pages": pages, "count": len(rows), "result": r}, nil
}
func yyChainChat(bm *BotManager, player, text string) (any, error) {
	if player == "" || len(player) > 128 || strings.HasPrefix(player, "@") || strings.ContainsAny(player, "\"\\\r\n") {
		return nil, fmt.Errorf("需要明确且有效的玩家名")
	}
	f := strings.Fields(text)
	if len(f) == 0 || (f[0] != ".命令选择" && f[0] != ".链") {
		return map[string]any{"handled": false}, nil
	}
	if len(f) == 1 || f[0] == ".命令选择" || f[1] == "列表" {
		page := 1
		if len(f) > 2 {
			page, _ = strconv.Atoi(f[2])
			if page < 1 {
				page = 1
			}
		}
		result, e := yyChainMenu(bm, player, page)
		return map[string]any{"handled": true, "result": result}, e
	}
	rows, e := yyChainRowsFor(bm)
	if e != nil {
		return nil, e
	}
	if len(f) < 3 {
		return nil, fmt.Errorf("格式：.链 操作 序号或文件名 [参数]")
	}
	name := strings.Join(f[2:], " ")
	if len(f) > 3 && (f[1] == "次数" || f[1] == "频率" || f[1] == "通道") {
		name = strings.Join(f[2:len(f)-1], " ")
	}
	if n, e := strconv.Atoi(name); e == nil {
		if n < 1 || n > len(rows) {
			return nil, fmt.Errorf("链序号无效")
		}
		name = rows[n-1]["config"].(map[string]any)["name"].(string)
	}
	info := playerInfoMgr.GetPlayer(player)
	if info == nil || !info.IsOP {
		return nil, fmt.Errorf("命令链控制与源码查看需要当前玩家 OP")
	}
	method := map[string]string{"启动": "chains.start", "停止": "chains.stop", "删除": "chains.delete", "源码": "chains.get"}[f[1]]
	args := []any{name}
	if f[1] == "次数" || f[1] == "频率" || f[1] == "通道" {
		if len(f) < 4 {
			return nil, fmt.Errorf("缺少参数")
		}
		patch := map[string]any{}
		key := map[string]string{"次数": "run_count", "频率": "interval_ms", "通道": "channel"}[f[1]]
		if key == "channel" {
			patch[key] = f[len(f)-1]
		} else {
			n, e := strconv.Atoi(f[len(f)-1])
			if e != nil {
				return nil, e
			}
			patch[key] = n
		}
		method = "chains.configure"
		args = append(args, patch)
	}
	if method == "" {
		return nil, fmt.Errorf("不支持该聊天操作")
	}
	value, e := yyChainPluginCall(bm, method, args)
	if e != nil {
		return nil, e
	}
	reply := "命令链操作完成：" + f[1] + " " + name
	if f[1] == "源码" {
		reply = value.(map[string]any)["content"].(string)
		r := []rune(reply)
		if len(r) > 1500 {
			reply = string(r[:1500]) + "\n…完整源码请在 APK 查看"
		}
	}
	if bm != nil {
		if e = bm.sendTargeted(player, reply); e != nil {
			return nil, e
		}
	}
	return map[string]any{"handled": true, "result": value}, nil
}
