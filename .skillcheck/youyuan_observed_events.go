package main

// Declarative packet adapters are owned by the subscribing plugin. They never
// invent server data and never replace the built-in observations.
import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

var yyObservedEventDocs = map[string]string{
	"events.on":               "kind, handler；订阅宿主统一事件，kind从events.catalog查询；Lua函数/函数名、Python回调、C++函数名。无需开启抓包。",
	"events.onObservedAction": "handler；统一接收swing/hurt/equipment/rotation/entity_kill/damage_display。真实挥臂只来自swing；伤害和击杀不能冒充空挥。",
	"events.catalog":          "无参数。列出真实事件类型、来源及范围；不保证未下发的数据。",
	"events.define":           "definition, handler；插件私有JSON映射：{name,packet,filter:{fields:{...},every_ms:0},fields:{输出字段:包字段路径}}。仅映射已解码的单一包，不执行代码、不伪造可信度；停止插件自动退订。",
}

func init() {
	for k, v := range yyObservedEventDocs {
		yyExtraMethods[k] = v
	}
	yySenseEvents["events.onObservedAction"] = "observed_action"
}

func yyObservedCatalog() []map[string]any {
	kinds := []string{"swing", "health", "hurt", "move", "rotation", "equipment", "death", "player_death", "player_respawn", "entity_spawn", "entity_remove", "item_pickup", "block_change", "session_join", "session_leave", "dimension_change", "entity_kill", "damage_display"}
	rows := make([]map[string]any, 0, len(kinds))
	for _, k := range kinds {
		scope := "server_synced_to_this_bot"
		if k == "entity_kill" || k == "damage_display" {
			scope = "server_module_broadcast_if_available"
		}
		rows = append(rows, map[string]any{"kind": k, "scope": scope, "global_guaranteed": false, "empty_swing_inferred": false})
	}
	return rows
}
func yyObservedKind(v any) (string, error) {
	k, ok := v.(string)
	if ok {
		for _, row := range yyObservedCatalog() {
			if row["kind"] == k {
				return k, nil
			}
		}
	}
	return "", fmt.Errorf("未知事件类型，请读取events.catalog")
}

type yyEventDefinition struct {
	Name   string            `json:"name"`
	Packet string            `json:"packet"`
	Filter yyPacketFilter    `json:"filter"`
	Fields map[string]string `json:"fields"`
}

var yyEventName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)

func yyEventDefinitionParse(v any) (yyEventDefinition, error) {
	var d yyEventDefinition
	raw, e := json.Marshal(v)
	if e != nil {
		return d, e
	}
	if e = json.Unmarshal(raw, &d); e != nil {
		return d, e
	}
	if !yyEventName.MatchString(d.Name) || d.Packet == "*" || !yyPacketTypeKnown(d.Packet) || yyPacketPrivate(d.Packet) || yyPacketBulk(d.Packet) {
		return d, fmt.Errorf("需要有效name和单一非私密、非大区块包名")
	}
	if len(d.Fields) > 32 {
		return d, fmt.Errorf("映射最多32个字段")
	}
	if e = yyValidateFilter(d.Filter); e != nil {
		return d, e
	}
	for k, p := range d.Fields {
		if !yyEventName.MatchString(k) || p == "" || len(p) > 256 {
			return d, fmt.Errorf("无效字段映射")
		}
		switch k {
		case "type", "source", "timestamp", "event_id", "data", "observed", "inferred", "server_verified", "packet_type":
			return d, fmt.Errorf("%s为保留元数据", k)
		}
	}
	return d, nil
}
func yyEventPath(data map[string]any, path string) (any, bool) {
	var v any = data
	for _, p := range strings.Split(path, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return v, true
}
func yyEventMapped(d yyEventDefinition, data map[string]any, now time.Time) map[string]any {
	e := map[string]any{"type": d.Name, "source": "custom_packet_mapping", "packet_type": d.Packet, "timestamp": now.UnixMilli(), "event_id": fmt.Sprintf("mapped:%s:%d", d.Name, now.UnixNano()), "observed": true, "inferred": false, "server_verified": false}
	for k, p := range d.Fields {
		v, ok := yyEventPath(data, p)
		if ok {
			e[k] = v
		} else {
			e[k] = nil
		}
	}
	return e
}
func yyEventAdapter(bm *BotManager, d yyEventDefinition, deliver func(map[string]any)) func() {
	var mu sync.Mutex
	var last time.Time
	stopped := false
	u := packetHubSubscribe(d.Packet, func(b *BotManager, data map[string]any) {
		if b != bm || !yyFilterMatch(d.Filter, data) {
			return
		}
		mu.Lock()
		now := time.Now()
		if stopped || now.Sub(last) < time.Duration(d.Filter.EveryMS)*time.Millisecond {
			mu.Unlock()
			return
		}
		last = now
		mu.Unlock()
		deliver(yyEventMapped(d, data, now))
	})
	return func() { mu.Lock(); stopped = true; mu.Unlock(); u() }
}
