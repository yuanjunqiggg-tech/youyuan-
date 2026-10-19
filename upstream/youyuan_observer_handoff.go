package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

func init() {
	for m, d := range map[string]string{
		"accountsPool.observationPlan": "options={accounts:[账号ID，main可选],players:[真实在线名，可空],dwell_seconds:0.5,radius_blocks:48,include_bots:false}；返回错峰路线、内部机器人标签和排除名单。普通用户只用自己的池，管理员可用授权池。只是规划，不启动或传送；48格是参考，不是必达半径。",
		"accountsPool.handoff":         "incoming_id,outgoing_id,options={player,mode:camera|follow|hybrid,interval_seconds:2,timeout_seconds:3,next_player?}；先让接替机器人观察同一目标，收到对应的新区域更新后才停止原观察，超时保留原观察。可让原观察者接着观察next_player；两端均须在自己的授权池。返回真实ready/old_released，不等于零漏包。",
	} {
		yyAccountPoolMethods[m] = true
		yyExtraMethods[m] = d
	}
}

func yyObserverAccountAllowed(actor yyPoolActor, id string) error {
	if id == "main" {
		if actor.ID != yyPoolBoundActor().ID && !actor.Admin {
			return fmt.Errorf("无权操作主账号")
		}
		return nil
	}
	_, e := yyPoolRequire(actor, id)
	return e
}
func yyObserverAccountCall(actor yyPoolActor, id, method string, args []any) (map[string]any, error) {
	if e := yyObserverAccountAllowed(actor, id); e != nil {
		return nil, e
	}
	if id == "main" {
		v, e := yySenseCall(getBotManagerLocked(), method, args)
		if e != nil {
			return nil, e
		}
		raw, e := json.Marshal(v)
		if e != nil {
			return nil, e
		}
		var r map[string]any
		e = json.Unmarshal(raw, &r)
		return r, e
	}
	v, e := yyPoolCall(actor, "accountsPool.invoke", []any{id, map[string]any{"method": "POST", "path": "/api/youyuan/game/call", "body": map[string]any{"method": method, "args": args}}})
	if e != nil {
		return nil, e
	}
	row, ok := v.(map[string]any)
	if !ok || row["ok"] != true {
		return nil, fmt.Errorf("观察账号调用失败：%v", v)
	}
	r, ok := row["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("观察账号响应格式错误")
	}
	return r, nil
}
func yyObservationPlan(names, robots, accounts []string, dwell, radius float64) map[string]any {
	excluded := map[string]bool{}
	for _, name := range robots {
		excluded[name] = true
	}
	targets := []string{}
	for _, name := range names {
		if !excluded[name] {
			targets = append(targets, name)
		}
	}
	sort.Strings(targets)
	assignments := []any{}
	// Keep one handoff reserve. Occupying all six accounts leaves no node able
	// to approach a target while its old observer remains in place.
	workers := len(accounts) - 1
	for i, id := range accounts {
		route := []string{}
		for j, name := range targets {
			if workers > 0 && j%workers == i {
				route = append(route, name)
			}
		}
		assignments = append(assignments, map[string]any{"account": id, "robot_label": "youyuan_observer", "players": route, "phase_seconds": float64(i) * dwell / float64(len(accounts)), "idle": len(route) == 0, "handoff_reserve": i == len(accounts)-1})
	}
	return map[string]any{"targets": targets, "excluded_bots": robots, "assignments": assignments, "dwell_seconds": dwell, "radius_blocks_reference": radius, "active": false, "global_coverage_guaranteed": false, "policy": "Routes are staggered; use handoff to keep an outgoing observer until incoming regional evidence. More separated targets than observers necessarily have uncovered intervals."}
}

// Player names only identify accounts within the actual server. Do not leak or
// exclude names from another project's game server when planning this route.
func yyObservationRobotNames(views []yyWorldView, server string) []string {
	robots := []string{}
	if server == "" {
		return robots
	}
	for _, view := range views {
		if view.Data["connected"] == true && view.Data["server"] == server {
			if name, ok := view.Data["name"].(string); ok && name != "" {
				robots = append(robots, name)
			}
		}
	}
	return robots
}
func yyObservationSameServer(a, b map[string]any) error {
	if a["connected"] != true || b["connected"] != true {
		return fmt.Errorf("两个观察账号均须真实连接")
	}
	x, _ := a["server"].(string)
	y, _ := b["server"].(string)
	if x == "" || x != y {
		return fmt.Errorf("两个观察账号必须在同一个服务器；不能跨服交接")
	}
	return nil
}
func yyPoolObservationCall(actor yyPoolActor, method string, args []any) (any, error) {
	if method == "accountsPool.observationPlan" {
		o, e := yyPoolOptions(args, 0)
		if e != nil {
			return nil, e
		}
		var ids, names []string
		raw, _ := json.Marshal(o["accounts"])
		if e = json.Unmarshal(raw, &ids); e != nil || len(ids) < 2 || len(ids) > 32 {
			return nil, fmt.Errorf("请选择2～32个独立观察账号")
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				return nil, fmt.Errorf("观察账号重复")
			}
			seen[id] = true
			if e = yyObserverAccountAllowed(actor, id); e != nil {
				return nil, e
			}
		}
		if v, ok := o["players"]; ok {
			raw, _ = json.Marshal(v)
			if e = json.Unmarshal(raw, &names); e != nil {
				return nil, fmt.Errorf("players需要玩家名数组")
			}
		}
		online := playerInfoMgr.OnlineSet()
		if len(names) == 0 {
			for name, on := range online {
				if on {
					names = append(names, name)
				}
			}
		}
		seen = map[string]bool{}
		for _, name := range names {
			if !yyWatchName(name) || !online[name] || seen[name] {
				return nil, fmt.Errorf("目标%s不是唯一在线玩家", name)
			}
			seen[name] = true
		}
		dwell, radius := .5, 48.
		if v, ok := o["dwell_seconds"]; ok {
			dwell, ok = yyWorldNumber(v)
			if !ok {
				return nil, fmt.Errorf("dwell_seconds需要数值")
			}
		}
		if _, e = yyPatrolDuration(dwell); e != nil {
			return nil, e
		}
		if v, ok := o["radius_blocks"]; ok {
			radius, ok = yyWorldNumber(v)
			if !ok || radius < 1 || radius > 1024 {
				return nil, fmt.Errorf("radius_blocks需要1..1024参考值")
			}
		}
		bm := getBotManagerLocked()
		robots := yyObservationRobotNames(yyWorldCollect(bm, "coverage", map[string]any{}), bm.ServerCode())
		include, _ := o["include_bots"].(bool)
		if include {
			if !actor.Admin {
				return nil, fmt.Errorf("仅管理员受控验收可包含机器人目标")
			}
			robots = nil
		}
		return yyObservationPlan(names, robots, ids, dwell, radius), nil
	}
	if actor.Scope == "read" {
		return nil, fmt.Errorf("交接需要控制授权")
	}
	if len(args) < 3 {
		return nil, fmt.Errorf("需要incoming_id、outgoing_id和options")
	}
	incoming, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("incoming_id无效")
	}
	outgoing, ok := args[1].(string)
	if !ok || incoming == outgoing {
		return nil, fmt.Errorf("需要两个不同观察账号")
	}
	for _, id := range []string{incoming, outgoing} {
		if e := yyObserverAccountAllowed(actor, id); e != nil {
			return nil, e
		}
	}
	views := []map[string]any{}
	for _, id := range []string{incoming, outgoing} {
		v, e := yyObserverAccountCall(actor, id, "observerBridge.localView", []any{map[string]any{"mode": "coverage"}})
		if e != nil {
			return nil, e
		}
		views = append(views, v)
	}
	if e := yyObservationSameServer(views[0], views[1]); e != nil {
		return nil, e
	}
	o, e := yyPoolOptions(args, 2)
	if e != nil {
		return nil, e
	}
	cfg, e := yyWatchParse(o)
	if e != nil {
		return nil, e
	}
	timeout := 3.
	if v, ok := o["timeout_seconds"]; ok {
		timeout, ok = yyWorldNumber(v)
		if !ok || timeout <= 0 || timeout > 60 {
			return nil, fmt.Errorf("timeout_seconds需要0～60之间的正值")
		}
	}
	old, e := yyObserverAccountCall(actor, outgoing, "observerBridge.watchStatus", nil)
	if e != nil {
		return nil, e
	}
	oldConfig, _ := old["config"].(map[string]any)
	if old["active"] != true || oldConfig["player"] != cfg.Player {
		return nil, fmt.Errorf("原观察者没有常驻观察这个目标")
	}
	active, e := yyObserverAccountCall(actor, incoming, "observerBridge.watchStatus", nil)
	if e != nil {
		return nil, e
	}
	if active["active"] == true {
		return nil, fmt.Errorf("接替机器人正在观察；选择空闲机器人")
	}
	started, e := yyObserverAccountCall(actor, incoming, "observerBridge.watchStart", []any{o})
	if e != nil {
		return nil, e
	}
	result := map[string]any{"incoming": incoming, "outgoing": outgoing, "player": cfg.Player, "ready": false, "old_released": false, "incoming_start": started, "global_coverage_guaranteed": false}
	deadline := time.Now().Add(time.Duration(timeout * float64(time.Second)))
	for time.Now().Before(deadline) {
		status, err := yyObserverAccountCall(actor, incoming, "observerBridge.watchStatus", nil)
		if err != nil {
			e = err
			break
		}
		result["incoming_status"] = status
		if status["active"] != true {
			e = fmt.Errorf("接替观察中止：%v", status["error"])
			break
		}
		if status["region_updated"] == true {
			result["ready"] = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if result["ready"] != true {
		_, cleanup := yyObserverAccountCall(actor, incoming, "observerBridge.watchStop", nil)
		result["error"] = "接替超时，原观察保持运行"
		if e != nil {
			result["error"] = e.Error()
		}
		if cleanup != nil {
			result["cleanup_error"] = cleanup.Error()
		}
		return result, nil
	}
	stopped, e := yyObserverAccountCall(actor, outgoing, "observerBridge.watchStop", nil)
	if e != nil {
		result["error"] = e.Error()
		return result, nil
	}
	result["old_status"] = stopped
	result["old_released"] = stopped["active"] == false
	if next, ok := o["next_player"].(string); ok && next != "" && result["old_released"] == true {
		nextConfig := map[string]any{}
		for k, v := range o {
			nextConfig[k] = v
		}
		nextConfig["player"] = next
		r, e := yyObserverAccountCall(actor, outgoing, "observerBridge.watchStart", []any{nextConfig})
		result["outgoing_next"] = r
		if e != nil {
			result["next_error"] = e.Error()
		}
	}
	return result, nil
}
