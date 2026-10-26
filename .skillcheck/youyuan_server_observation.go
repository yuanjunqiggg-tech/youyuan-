package main

// Explicit server-module observations never replace local protocol attributes.
import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"
)

var yyServerSnapshots = struct {
	sync.Mutex
	Items map[*BotManager]map[string]map[string]any
}{Items: map[*BotManager]map[string]map[string]any{}}

func yyServerSnapshotReset(bm *BotManager) {
	yyServerSnapshots.Lock()
	delete(yyServerSnapshots.Items, bm)
	yyServerSnapshots.Unlock()
}

func yyServerSnapshotRecord(bm *BotManager, session, kind string, data, row map[string]any) {
	id := yyServerID(data["entity_id"])
	if id == "" {
		return
	}
	if kind != "health_changed" && kind != "health_snapshot" {
		return
	}
	health, valid := yyServerHealth(data)
	if !valid {
		return
	}
	received, _ := yyServerNumber(row["received_at"])
	observed, timestampValid := received, true
	if ts, exists := data["timestamp_ms"]; exists {
		n, valid := yyServerNumber(ts)
		timestampValid = valid && n > 0 && n <= received+5000
		if timestampValid {
			observed = n
		}
	}
	result := map[string]any{"known": true, "entity_id": id, "session": session,
		"received_at": int64(received), "observed_at": int64(observed), "timestamp_valid": timestampValid,
		"source": "YouyuanBridge.ObserverV1", "evidence": "server_module_reported", "server_engine_verified": false,
		"data": yyObservationCopy(data)}
	for k, v := range health {
		result[k] = v
	}
	yyServerSnapshots.Lock()
	defer yyServerSnapshots.Unlock()
	cache := yyServerSnapshots.Items[bm]
	if cache == nil {
		cache = map[string]map[string]any{}
		yyServerSnapshots.Items[bm] = cache
	}
	if _, exists := cache[id]; !exists {
		for len(cache) >= yyObservationSettings().Actors {
			oldestID, oldestTime := "", int64(math.MaxInt64)
			for key, entry := range cache {
				if at := entry["received_at"].(int64); at < oldestTime {
					oldestID, oldestTime = key, at
				}
			}
			delete(cache, oldestID)
		}
	}
	cache[id] = result
}

var yyServerObservationMethods = map[string]string{
	"events.onServerObservation":    "handler(event)；YouyuanBridge/ObserverV1/Observation实际下发时回调，含模块事件、精确实体ID、收到时间和证据；需安装主动转发模块，OP不自动安装。",
	"events.onGlobalSwing":          "handler(event)；仅收到转发模块swing事件且有明确player_id时触发；不把命中攻击当空挥，不扫描最近玩家。global表示转发模块不按机器人视距过滤，不承诺模块已安装。",
	"events.onGlobalHealthChange":   "handler(event)；仅模块health_changed有有限current血量和entity_id时触发；max未知保留null；与entities.health协议缓存分开。",
	"observerBridge.serverHistory":  "options={session?,after?,limit?}；当前连接主动转发模块历史，带reset/lost/cursor/接收时间；不执行网络请求，不使用旧会话记录。",
	"observerBridge.serverSnapshot": "entity_id字符串；读取当前会话最近模块health_changed/health_snapshot，返回known/current/max/previous/来源/时间/stale。未知不填20；需已收到转发模块，不覆盖本机属性缓存。",
}

func init() {
	for method, doc := range yyServerObservationMethods {
		yyExtraMethods[method] = doc
	}
	yySenseEvents["events.onServerObservation"] = "server_observation"
	yySenseEvents["events.onGlobalSwing"] = "global_swing"
	yySenseEvents["events.onGlobalHealthChange"] = "global_health"
}

func yyServerID(v any) string {
	switch x := v.(type) {
	case string:
		if len(x) > 0 && len(x) <= 128 {
			return x
		}
	case int64:
		return strconv.FormatInt(x, 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	}
	return ""
}
func yyServerNumber(v any) (float64, bool) {
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case float32:
		n = float64(x)
	case int:
		n = float64(x)
	case int32:
		n = float64(x)
	case int64:
		n = float64(x)
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0)
}
func yyServerHealth(data map[string]any) (map[string]any, bool) {
	h, ok := data["health"].(map[string]any)
	if !ok {
		return nil, false
	}
	current, ok := yyServerNumber(h["current"])
	if !ok || current < 0 {
		return nil, false
	}
	result := map[string]any{"current": current, "max": nil, "previous": nil, "max_known": false}
	if value, exists := h["max"]; exists && value != nil {
		max, yes := yyServerNumber(value)
		if !yes || max <= 0 {
			return nil, false
		}
		if current > max {
			return nil, false
		}
		result["max"], result["max_known"] = max, true
	}
	if previous, yes := yyServerNumber(h["previous"]); yes && previous >= 0 {
		result["previous"] = previous
	}
	return result, true
}

func yyServerObservationEmit(bm *BotManager, row map[string]any) {
	payload, ok := row["data"].(map[string]any)
	if !ok {
		return
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return
	}
	kind, _ := payload["event_type"].(string)
	// A provider's explicit event ID is stable; otherwise the received cursor is.
	session := yyServerObservationSession(bm)
	yyServerSnapshotRecord(bm, session, kind, data, row)
	eventID, _ := data["event_id"].(string)
	if eventID == "" {
		eventID = fmt.Sprintf("%s/server/%v", session, row["cursor"])
	}
	base := map[string]any{"type": "server_observation", "event_type": kind, "event_id": eventID, "session": session, "timestamp": row["received_at"], "received_at": row["received_at"], "source": "YouyuanBridge.ObserverV1", "evidence": "server_module_reported", "server_engine_verified": false, "data": yyObservationCopy(data)}
	emit := func(e map[string]any) { yyRuntimeEmit(bm, e); yySenseLuaEmit(bm, e) }
	emit(base)
	if kind == "swing" {
		id := yyServerID(data["player_id"])
		if id == "" {
			return
		}
		e := yyObservationCopy(base).(map[string]any)
		e["type"], e["player_id"] = "global_swing", id
		emit(e)
	}
	if kind == "health_changed" {
		id := yyServerID(data["entity_id"])
		if id == "" {
			return
		}
		health, valid := yyServerHealth(data)
		if !valid {
			return
		}
		e := yyObservationCopy(base).(map[string]any)
		e["type"], e["entity_id"], e["health"] = "global_health", id, health
		emit(e)
	}
}

func yyServerObservationSession(bm *BotManager) string {
	if bm == nil {
		return ""
	}
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	return s.Session
}
func yyServerObservationCall(bm *BotManager, method string, args []any) (any, error) {
	if bm == nil {
		return nil, fmt.Errorf("需要游戏账号连接上下文")
	}
	session := yyServerObservationSession(bm)
	o := map[string]any{}
	target := ""
	if method == "observerBridge.serverSnapshot" {
		if len(args) != 1 {
			return nil, fmt.Errorf("serverSnapshot(entity_id)")
		}
		target = yyServerID(args[0])
		if target == "" {
			return nil, fmt.Errorf("需要精确实体ID字符串")
		}
	} else {
		if len(args) > 1 {
			return nil, fmt.Errorf("serverHistory(options)")
		}
		if len(args) == 1 {
			var ok bool
			o, ok = args[0].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("options必须为对象")
			}
		}
	}
	after, limit := uint64(0), 256
	if v, exists := o["after"]; exists {
		n, e := yyNBTBoundedInteger(v, 0, 1<<53-1)
		if e != nil {
			return nil, e
		}
		after = uint64(n)
	}
	if v, exists := o["limit"]; exists {
		n, e := yyNBTBoundedInteger(v, 1, 4096)
		if e != nil {
			return nil, e
		}
		limit = int(n)
	}
	previous, _ := o["session"].(string)
	reset := previous != "" && previous != session
	if reset {
		after = 0
	}
	yyObserverRPCObserved.Lock()
	rows := append([]map[string]any(nil), yyObserverRPCObserved.Items[bm]...)
	head := yyObserverRPCObserved.Cursors[bm]
	yyObserverRPCObserved.Unlock()
	if method == "observerBridge.serverSnapshot" {
		result := map[string]any{"known": false, "entity_id": target, "session": session, "current": nil, "max": nil, "previous": nil, "stale": true, "reason": "当前连接未收到该实体的模块血量；请安装转发模块或使用局部UpdateAttributes"}
		yyServerSnapshots.Lock()
		entry := yyServerSnapshots.Items[bm][target]
		if entry != nil && entry["session"] == session {
			result = yyObservationCopy(entry).(map[string]any)
			result["stale"] = result["timestamp_valid"] != true || time.Now().UnixMilli()-entry["observed_at"].(int64) > int64(yyObservationSettings().StaleMS)
		}
		yyServerSnapshots.Unlock()
		return result, nil
	}
	out := []any{}
	cursor := after
	lost := uint64(0)
	if len(rows) > 0 {
		first := rows[0]["cursor"].(uint64)
		if after+1 < first {
			lost = first - after - 1
		}
	}
	for _, row := range rows {
		sequence := row["cursor"].(uint64)
		if sequence > after {
			out = append(out, yyObservationCopy(row))
			cursor = sequence
			if len(out) >= limit {
				break
			}
		}
	}
	return map[string]any{"events": out, "session": session, "cursor": cursor, "latest_sequence": head, "lost": lost, "reset": reset, "capacity": yyObservationSettings().EventHistory, "source": "server_module_reported"}, nil
}
