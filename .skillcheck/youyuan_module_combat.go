package main

// Read-only adapters for server-module broadcasts captured on the test realm.
// They report module evidence, never synthesize health or infer nearby attackers.
import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/google/uuid"
)

var yyModuleCombatDocs = map[string]string{
	"events.onEntityKill":    "handler(event)；接收已验证Minecraft/vipEventSystem/PlayerKillMob<UUID>服务器模块击杀报告；攻击者与目标UniqueID精确字符串，名字仅由在线PlayerList关联。evidence=server_module_reported；不是PvP onPlayerKill，不补造血量/RuntimeID，模块未广播则不可用。同会话有界缓存按来源/攻击者/目标去重。",
	"events.onDamageDisplay": "handler(event)；已验证BREquipDamageEventServer/GetBroadcastData模块伤害显示；target.unique_id、damage_text、position与position_known。数字为显示文字，不是确认的HP扣血；attacker_known=false、dimension_known=false，不改血量缓存。",
	"events.moduleHistory":   "options={session?,after?,limit?}；当前机器人规范模块战斗事件独立历史与module_sequence游标，避免被移动事件挤掉。limit默认256、1..4096；容量沿用diagnostics.configure的event_history，重连reset，覆盖lost；不含乐器高频事件。",
}

func init() {
	for m, d := range yyModuleCombatDocs {
		yyExtraMethods[m] = d
	}
	yySenseEvents["events.onEntityKill"] = "entity_kill"
	yySenseEvents["events.onDamageDisplay"] = "damage_display"
}

func yyModuleUID(v any) (int64, bool) {
	// MessagePack preserves integer width; reject floating IDs rather than round.
	switch n := v.(type) {
	case int64:
		return n, n != 0
	case int:
		return int64(n), n != 0
	case uint64:
		if n <= math.MaxInt64 {
			return int64(n), n != 0
		}
	case string:
		id, err := strconv.ParseInt(n, 10, 64)
		return id, err == nil && id != 0
	case []byte:
		return yyModuleUID(string(n))
	}
	return 0, false
}

func yyModuleIdentity(s *yySenseState, id int64) map[string]any {
	r := map[string]any{"unique_id": strconv.FormatInt(id, 10), "runtime_id": nil, "name": nil, "name_known": false, "uuid": nil, "xuid": nil, "entity_type": nil, "health": nil}
	for _, v := range s.Identities {
		if v.UniqueID == id && v.Online {
			r["name"], r["name_known"], r["uuid"], r["xuid"], r["entity_type"] = v.Name, yyKnownPlayerName(v.Name), v.UUID, v.XUID, "minecraft:player"
			break
		}
	}
	// IDs may be absent from the local view: do not create an actor to fill them.
	for _, v := range s.Actors {
		if v.UniqueID == id && v.Present {
			r["runtime_id"], r["entity_type"] = strconv.FormatUint(v.RuntimeID, 10), v.EntityType
			break
		}
	}
	return r
}

func yyModulePosition(v any) ([]any, bool) {
	if m, ok := v.(map[string]any); ok {
		if m["__type__"] != "tuple" {
			return nil, false
		}
		v = m["value"]
	}
	a, ok := v.([]any)
	if !ok || len(a) != 3 {
		return nil, false
	}
	// []any is supported by both JSON and the resident Lua binding.
	pos := make([]any, 3)
	for i, v := range a {
		n, ok := yyWorldNumber(v)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, false
		}
		pos[i] = n
	}
	return pos, true
}

// Caller holds s.Lock. Exact namespace/system validation precedes body parsing.
func yyModuleCombatEvents(s *yySenseState, p *packet.PyRpc, now time.Time) []map[string]any {
	key, ok := yyRPCIngressIdentity(p)
	// This realm used operation 32346342 for actual inbound combat broadcasts.
	// Generic ModEventS2C uses Recv, but that assumption would discard these.
	if !ok || p.OperationType != 32346342 {
		return nil
	}
	inner := p.Value.([]any)[1].([]any)
	body, ok := inner[3].(map[string]any)
	if !ok {
		return nil
	}
	base := func(kind string) map[string]any {
		return map[string]any{"type": kind, "source": "PyRpc", "evidence": "server_module_reported", "module": map[string]any{"namespace": key.Namespace, "system": key.System, "event": key.Event, "operation_type": p.OperationType}, "time": now.Format(time.RFC3339Nano), "dimension": nil, "dimension_known": false, "health_known": false}
	}
	if key.Namespace == "Minecraft" && key.System == "vipEventSystem" && strings.HasPrefix(key.Event, "PlayerKillMob") {
		suffix := strings.TrimPrefix(key.Event, "PlayerKillMob")
		if _, e := uuid.Parse(suffix); e != nil || len(suffix) != 36 {
			return nil
		}
		attacker, aok := yyModuleUID(body["playerId"])
		victim, vok := yyModuleUID(body["victim"])
		if !aok || !vok {
			return nil
		}
		dedup := key.Namespace + "/" + key.System + "/" + key.Event + "/" + strconv.FormatInt(attacker, 10) + "/" + strconv.FormatInt(victim, 10)
		if s.ModuleKills == nil {
			s.ModuleKills = map[string]bool{}
		}
		if s.ModuleKills[dedup] {
			return nil
		}
		s.ModuleKills[dedup] = true
		s.ModuleKillOrder = append(s.ModuleKillOrder, dedup)
		capacity := yyObservationSettings().EventHistory
		for len(s.ModuleKillOrder) > capacity {
			delete(s.ModuleKills, s.ModuleKillOrder[0])
			s.ModuleKillOrder = s.ModuleKillOrder[1:]
		}
		e := base("entity_kill")
		e["attacker"], e["target"], e["attacker_known"] = yyModuleIdentity(s, attacker), yyModuleIdentity(s, victim), true
		e["player"] = e["attacker"].(map[string]any)["name"]
		return []map[string]any{e}
	}
	if key.Namespace != "BREquipDamageEventServer" || key.System != "BREquipDamageEventServerSystem" || key.Event != "GetBroadcastData" {
		return nil
	}
	targets, ok := body["targetInfo"].(map[string]any)
	if !ok {
		return nil
	}
	// Preserve all accepted packet entries; history capacity controls retention.
	keys := make([]string, 0, len(targets))
	for k := range targets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []map[string]any{}
	for _, id := range keys {
		uid, ok := yyModuleUID(id)
		if !ok {
			continue
		}
		data, ok := targets[id].(map[string]any)
		if !ok {
			continue
		}
		text, ok := data["damage"].(string)
		if !ok || len(text) == 0 || len(text) > 256 {
			continue
		}
		e := base("damage_display")
		e["target"], e["damage_text"], e["damage_known"], e["attacker_known"] = yyModuleIdentity(s, uid), text, false, false
		pos, known := yyModulePosition(data["pos"])
		e["position"], e["position_known"] = pos, known
		out = append(out, e)
	}
	return out
}

func yyModuleRemember(s *yySenseState, e map[string]any) {
	if e["evidence"] != "server_module_reported" {
		return
	}
	s.ModuleSeq++
	e["module_sequence"] = s.ModuleSeq
	e["event_id"] = fmt.Sprintf("%s/module/%d", s.Session, s.ModuleSeq)
	s.ModuleEvents = append(s.ModuleEvents, e)
	capacity := yyObservationSettings().EventHistory
	if len(s.ModuleEvents) > capacity {
		s.ModuleEvents = s.ModuleEvents[len(s.ModuleEvents)-capacity:]
	}
}

func yyModuleHistory(bm *BotManager, args []any) (any, error) {
	o, err := yyWorldOptions(args, 0)
	if err != nil {
		return nil, err
	}
	limit := 256
	if v, ok := o["limit"]; ok {
		n, ok := yyWorldNumber(v)
		if !ok || n < 1 || n > 4096 || n != float64(int(n)) {
			return nil, fmt.Errorf("limit需1..4096")
		}
		limit = int(n)
	}
	after := uint64(0)
	if v, ok := o["after"]; ok {
		n, ok := yyWorldNumber(v)
		if !ok || n < 0 || n >= 1<<53 || n != float64(uint64(n)) {
			return nil, fmt.Errorf("after需非负安全整数")
		}
		after = uint64(n)
	}
	if bm == nil {
		return map[string]any{"connected": false, "events": []any{}}, nil
	}
	bm.mu.Lock()
	connected := bm.connected
	bm.mu.Unlock()
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	requested, _ := o["session"].(string)
	reset := requested != "" && requested != s.Session
	if reset {
		after = 0
	}
	oldest := s.ModuleSeq + 1
	if len(s.ModuleEvents) > 0 {
		oldest = s.ModuleEvents[0]["module_sequence"].(uint64)
	}
	lost := uint64(0)
	if oldest > after+1 {
		lost = oldest - after - 1
	}
	rows := []any{}
	cursor := after
	for _, e := range s.ModuleEvents {
		seq := e["module_sequence"].(uint64)
		if seq <= after {
			continue
		}
		rows = append(rows, yyObservationCopy(e))
		cursor = seq
		if len(rows) >= limit {
			break
		}
	}
	return map[string]any{"connected": connected, "session": s.Session, "cursor": cursor, "latest_sequence": s.ModuleSeq, "lost": lost, "reset": reset, "events": rows, "capacity": yyObservationSettings().EventHistory, "scope": "received_server_module_reports"}, nil
}
