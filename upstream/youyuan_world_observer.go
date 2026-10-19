package main

// Federated observations are queried on demand. No polling, game commands, or
// synchronous child HTTP calls are added to the packet receive loop.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var yyWorldSerial atomic.Uint64

func yyWorldSessionID() string {
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), yyWorldSerial.Add(1))
}

var yyWorldMethods = map[string]string{
	"observerBridge.coverage":    "options={server?}；按需汇总本宿主主账号、舰队与托管子账号的实际观测覆盖、维度和错误；不表示全服覆盖。",
	"observerBridge.health":      "runtimeId字符串,options={server?,dimension?}；读取各在线观测账号最新真实血量，附观察者/session/来源/过期状态；维度或实体身份冲突返回未知。不会覆盖本机entities缓存。",
	"observerBridge.locator":     "玩家名或UniqueID字符串,options={server?}；汇总真实PlayerLocation广播，即使没有AddPlayer也保留；包没有维度、血量或RuntimeID，这些字段明确未知。",
	"observerBridge.worldEvents": "options={server?,dimension?,cursors?,limit?}；汇总各账号规范事件，返回每账号session/sequence游标；重连标明reset，历史覆盖标明lost。再次使用返回cursors避免重复；limit是每观察者返回条数。",
	"observerBridge.localView":   "options={mode:coverage|health|events,target?,dimension?,session?,after?,limit?}；仅当前连接快照，供观测聚合使用；无递归，不执行游戏指令。",
}

func init() {
	for m, d := range yyWorldMethods {
		yyExtraMethods[m] = d
	}
}

func yyWorldOptions(args []any, index int) (map[string]any, error) {
	if len(args) <= index {
		return map[string]any{}, nil
	}
	v, ok := args[index].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("options必须为对象")
	}
	return v, nil
}
func yyWorldNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case uint64:
		return float64(n), true
	case int32:
		return float64(n), true
	}
	return 0, false
}
func yyWorldDimension(o map[string]any) (*int32, error) {
	if v, ok := o["dimension"]; ok {
		n, ok := yyWorldNumber(v)
		if !ok || n != float64(int32(n)) {
			return nil, fmt.Errorf("dimension必须为整数")
		}
		d := int32(n)
		return &d, nil
	}
	return nil, nil
}

func yyWorldLocal(bm *BotManager, o map[string]any) (map[string]any, error) {
	mode, _ := o["mode"].(string)
	if mode != "coverage" && mode != "health" && mode != "events" && mode != "locator" {
		return nil, fmt.Errorf("未知观测模式")
	}
	d, err := yyWorldDimension(o)
	if err != nil {
		return nil, err
	}
	r := map[string]any{"connected": false, "entities": []any{}, "events": []any{}, "coverage": "delivered_packets_only"}
	if bm == nil {
		return r, nil
	}
	s := yySenseFor(bm)
	bm.mu.Lock()
	defer bm.mu.Unlock()
	s.Lock()
	defer s.Unlock()
	r["connected"], r["server"], r["name"], r["dimension"], r["position"], r["session"] = bm.connected, bm.serverCode, bm.display, bm.dimension, bm.pos, s.Session
	r["account_id"] = "main"
	if yyAccounts.Worker {
		r["account_id"] = filepath.Base(yyAccounts.Root)
	}
	count, healthCount := 0, 0
	for _, a := range s.Actors {
		if a.Present {
			count++
			if a.Health != nil {
				healthCount++
			}
		}
	}
	r["observed_entities"], r["known_health"] = count, healthCount
	if !bm.connected {
		return r, nil
	}
	if mode == "locator" {
		target, _ := o["target"].(string)
		r["entities"] = yyLocatorSnapshot(s, target, true)
	}
	if mode == "health" {
		target, _ := o["target"].(string)
		id, err := strconv.ParseUint(target, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("runtimeId需十进制字符串")
		}
		if a := s.Actors[id]; a != nil && a.Present && (d == nil || a.Dimension == *d) {
			r["entities"] = []any{map[string]any{"runtime_id": target, "unique_id": strconv.FormatInt(a.UniqueID, 10), "dimension": a.Dimension, "name": a.Name, "entity_type": a.EntityType, "health": yyObservationCopy(yyHealthSnapshot(a, true))}}
		}
	}
	if mode == "events" {
		history := s.Events
		stream, _ := o["stream"].(string)
		actions := stream == "actions"
		if actions {
			history = s.ActionEvents
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
		requested, _ := o["session"].(string)
		reset := requested != "" && requested != s.Session
		if reset {
			after = s.SessionBase
		}
		if after < s.SessionBase {
			after = s.SessionBase
		}
		oldest := s.Seq + 1
		if len(history) > 0 {
			oldest = history[0]["sequence"].(uint64)
		}
		lost := uint64(0)
		if oldest > after+1 {
			lost = oldest - after - 1
		}
		if actions {
			lost = 0
			if after < s.ActionEvictedSeq {
				lost = 1
			}
			r["lost_count_known"] = false
			r["stream"] = "actions"
		}
		rows := []any{}
		cursor := after
		for _, e := range history {
			seq := e["sequence"].(uint64)
			if seq <= after {
				continue
			}
			cursor = seq
			dim, ok := yyWorldNumber(e["dimension"])
			if d != nil && (!ok || int32(dim) != *d) {
				continue
			}
			rows = append(rows, yyObservationCopy(e))
			if len(rows) >= limit {
				break
			}
		}
		if len(rows) < limit {
			cursor = s.Seq
		}
		r["events"], r["reset"], r["lost"], r["cursor"], r["latest_sequence"] = rows, reset, lost, cursor, s.Seq
	}
	return r, nil
}

type yyWorldView struct {
	Account string
	Data    map[string]any
	Error   string
}

// Query only registered children, not every cached BotManager ever created.
func yyWorldCollect(bm *BotManager, mode string, o map[string]any) []yyWorldView {
	optionsFor := func(account string) map[string]any {
		n := map[string]any{"mode": mode}
		for _, k := range []string{"target", "dimension", "limit", "stream"} {
			if v, ok := o[k]; ok {
				n[k] = v
			}
		}
		if cursors, ok := o["cursors"].(map[string]any); ok {
			if c, ok := cursors[account].(map[string]any); ok {
				n["after"], n["session"] = c["sequence"], c["session"]
			}
		}
		return n
	}
	views := []yyWorldView{}
	addLocal := func(id string, b *BotManager) {
		v, e := yyWorldLocal(b, optionsFor(id))
		row := yyWorldView{Account: id, Data: v}
		if e != nil {
			row.Error = e.Error()
		}
		views = append(views, row)
	}
	addLocal("main", bm)
	for i, b := range GetFleetBots() {
		if b != bm {
			addLocal(fmt.Sprintf("fleet_%d", i), b)
		}
	}
	yyAccounts.Lock()
	profiles := append([]yyManagedAccount(nil), yyAccounts.Items...)
	worker := yyAccounts.Worker
	yyAccounts.Unlock()
	if worker {
		return views
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, a := range profiles {
		if a.Slot == 0 {
			continue
		}
		wg.Add(1)
		go func(a yyManagedAccount) {
			defer wg.Done()
			row := yyWorldView{Account: a.ID}
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				row.Error = "观测请求超时"
				mu.Lock()
				views = append(views, row)
				mu.Unlock()
				return
			}
			data, _ := json.Marshal(map[string]any{"method": "observerBridge.localView", "args": []any{optionsFor(a.ID)}})
			res, err := yyChildRequest(ctx, a, http.MethodPost, "/api/youyuan/game/call", data, "application/json")
			if err == nil {
				defer res.Body.Close()
				var envelope struct {
					OK     bool           `json:"ok"`
					Result map[string]any `json:"result"`
					Error  string         `json:"error"`
				}
				err = json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&envelope)
				if err == nil {
					if res.StatusCode != 200 || !envelope.OK {
						err = fmt.Errorf("子账号观测失败: %s", envelope.Error)
					} else if envelope.Result["account_id"] != a.ID {
						err = fmt.Errorf("运行位置账号不匹配")
					} else {
						row.Data = envelope.Result
					}
				}
			}
			if err != nil {
				row.Error = err.Error()
			}
			mu.Lock()
			views = append(views, row)
			mu.Unlock()
		}(a)
	}
	wg.Wait()
	sort.Slice(views, func(i, j int) bool { return views[i].Account < views[j].Account })
	return views
}

func yyWorldMerge(mode string, o map[string]any, views []yyWorldView) map[string]any {
	server, _ := o["server"].(string)
	observations := []any{}
	rows := []any{}
	observers := []any{}
	errors := []any{}
	cursors := map[string]any{}
	if previous, ok := o["cursors"].(map[string]any); ok {
		for account, cursor := range previous {
			cursors[account] = yyObservationCopy(cursor)
		}
	}
	online := 0
	for _, v := range views {
		if v.Error != "" {
			errors = append(errors, map[string]any{"account": v.Account, "error": v.Error})
			continue
		}
		if server != "" && v.Data["server"] != server {
			continue
		}
		info := map[string]any{"account": v.Account}
		for _, k := range []string{"connected", "server", "name", "dimension", "position", "session", "observed_entities", "known_health"} {
			info[k] = v.Data[k]
		}
		observers = append(observers, info)
		if v.Data["connected"] != true {
			continue
		}
		online++
		for _, k := range []string{"entities", "events"} {
			if list, ok := v.Data[k].([]any); ok {
				for _, item := range list {
					e, ok := item.(map[string]any)
					if !ok {
						continue
					}
					copy := yyObservationCopy(e).(map[string]any)
					copy["observer"] = info
					copy["transport_evidence"] = "server_to_bot_decoded"
					if _, exists := copy["evidence"]; !exists {
						copy["evidence"] = "server_to_bot_decoded"
					}
					if k == "entities" {
						observations = append(observations, copy)
					} else {
						copy["observation_id"] = fmt.Sprintf("%s/%v/%v", v.Account, v.Data["session"], e["sequence"])
						rows = append(rows, copy)
					}
				}
			}
		}
		if mode == "events" {
			cursors[v.Account] = map[string]any{"session": v.Data["session"], "sequence": v.Data["cursor"], "reset": v.Data["reset"], "lost": v.Data["lost"]}
		}
	}
	r := map[string]any{"server": server, "observers": observers, "online_observers": online, "errors": errors, "complete": len(errors) == 0, "global_coverage_guaranteed": false, "scope": "union_of_connected_observers", "observations": observations}
	if mode == "events" {
		sort.SliceStable(rows, func(i, j int) bool {
			a, _ := yyWorldNumber(rows[i].(map[string]any)["timestamp"])
			b, _ := yyWorldNumber(rows[j].(map[string]any)["timestamp"])
			return a < b
		})
		r["events"], r["cursors"] = rows, cursors
		r["deduplication"] = "per_observer_session_cursor; duplicate sightings retained with provenance"
	}
	if mode == "health" {
		r["known"] = false
		r["reason"] = "在线观测者未收到目标的可用血量"
		identities := map[string]bool{}
		var best map[string]any
		latest := int64(-1)
		for _, v := range observations {
			e := v.(map[string]any)
			h := e["health"].(map[string]any)
			if h["known"] != true {
				continue
			}
			observer := e["observer"].(map[string]any)
			dim, _ := yyWorldNumber(e["dimension"])
			key := fmt.Sprintf("%v/%d/%v", observer["server"], int32(dim), e["unique_id"])
			identities[key] = true
			t, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(h["updated_at"]))
			if ms := t.UnixNano(); ms > latest {
				latest = ms
				best = e
			}
		}
		if len(identities) > 1 {
			r["reason"] = "观察结果实体身份或维度冲突，请明确服务器/维度后重新查询"
			r["ambiguous"] = true
		} else if best != nil {
			r["known"], r["health"], r["observer"], r["entity"] = true, best["health"], best["observer"], best
			delete(r, "reason")
		}
	}
	if mode == "locator" {
		r["known"] = false
		r["dimension_known"] = false
		r["reason"] = "未收到目标PlayerLocation坐标"
		var best map[string]any
		latest := int64(-1)
		identities := map[string]bool{}
		for _, v := range observations {
			e := v.(map[string]any)
			if e["known"] != true {
				continue
			}
			identities[fmt.Sprint(e["unique_id"])] = true
			t, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(e["updated_at"]))
			if ms := t.UnixNano(); ms > latest {
				latest = ms
				best = e
			}
		}
		if len(identities) > 1 {
			r["ambiguous"] = true
			r["reason"] = "同名玩家身份冲突，请使用UniqueID"
		} else if best != nil {
			r["known"], r["locator"], r["observer"] = true, best, best["observer"]
			delete(r, "reason")
		}
	}
	return r
}

func yyWorldCall(bm *BotManager, method string, args []any) (any, error) {
	index := 0
	if method == "observerBridge.health" || method == "observerBridge.locator" {
		index = 1
	}
	o, err := yyWorldOptions(args, index)
	if err != nil {
		return nil, err
	}
	o = yyObservationCopy(o).(map[string]any)
	if v, exists := o["stream"]; exists {
		s, ok := v.(string)
		if !ok || (s != "all" && s != "actions") {
			return nil, fmt.Errorf("stream需all/actions")
		}
	}
	if _, err = yyWorldDimension(o); err != nil {
		return nil, err
	}
	if v, ok := o["limit"]; ok {
		n, valid := yyWorldNumber(v)
		if !valid || n < 1 || n > 4096 || n != float64(int(n)) {
			return nil, fmt.Errorf("limit需1..4096")
		}
	}
	if v, ok := o["server"]; ok {
		if server, valid := v.(string); !valid || server == "" {
			return nil, fmt.Errorf("server需非空字符串")
		}
	}
	if method == "observerBridge.localView" {
		return yyWorldLocal(bm, o)
	}
	mode := "coverage"
	if method == "observerBridge.locator" {
		mode = "locator"
		if len(args) < 1 {
			return nil, fmt.Errorf("需要玩家名或UniqueID")
		}
		target, ok := args[0].(string)
		if !ok || target == "" {
			return nil, fmt.Errorf("目标需非空字符串")
		}
		if _, ok := o["dimension"]; ok {
			return nil, fmt.Errorf("PlayerLocation没有维度字段，不能按维度查询")
		}
		o["target"] = target
	}
	if method == "observerBridge.health" {
		mode = "health"
		if len(args) < 1 {
			return nil, fmt.Errorf("需要runtimeId")
		}
		target, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("runtimeId必须是字符串")
		}
		if _, err = strconv.ParseUint(target, 10, 64); err != nil {
			return nil, fmt.Errorf("runtimeId需十进制字符串")
		}
		o["target"] = target
	}
	if method == "observerBridge.worldEvents" {
		mode = "events"
	}
	if _, ok := o["server"]; !ok && bm != nil {
		bm.mu.Lock()
		o["server"] = bm.serverCode
		bm.mu.Unlock()
	}
	if o["server"] == "" || o["server"] == nil {
		return nil, fmt.Errorf("请连接目标服务器或提供options.server")
	}
	return yyWorldMerge(mode, o, yyWorldCollect(bm, mode, o)), nil
}
