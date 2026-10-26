package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/google/uuid"
)

type yyObserverRPCWaiter struct {
	Bot   *BotManager
	Reply chan map[string]any
}

var yyObserverRPCPending = struct {
	sync.Mutex
	Items map[string]yyObserverRPCWaiter
}{Items: map[string]yyObserverRPCWaiter{}}

var yyObserverRPCObserved = struct {
	sync.Mutex
	Items   map[*BotManager][]map[string]any
	Cursors map[*BotManager]uint64
}{Items: map[*BotManager][]map[string]any{}, Cursors: map[*BotManager]uint64{}}

func init() {
	for method, doc := range map[string]string{
		"observerBridge.rpcProbe":  "timeout_ms=3000；向YouyuanBridge/ObserverV1发送Hello，只在对应Capabilities回复匹配request_id时confirmed=true。超时不代表其他网易模块不可用。",
		"observerBridge.rpcQuery":  "{query,target?},timeout_ms=3000；查询已安装YouyuanBridge服务端只读Provider；收到对应Reply才confirmed=true。没有Provider不会凭空读取服务端数据。",
		"observerBridge.rpcEvents": "after=0,limit=256；读取当前连接收到的YouyuanBridge/ObserverV1/Observation；来源server_module_reported，与协议属性缓存分开。重连清空，不代表已验证引擎事件。",
	} {
		yyObserverBridgeMethods[method] = doc
		yyExtraMethods[method] = doc
	}
}

func yyObserverRPCReset(bm *BotManager) {
	yyServerSnapshotReset(bm)
	yyObserverRPCObserved.Lock()
	delete(yyObserverRPCObserved.Items, bm)
	delete(yyObserverRPCObserved.Cursors, bm)
	yyObserverRPCObserved.Unlock()
}
func yyObserverRPCRead(bm *BotManager, args []any) (any, error) {
	after, limit := uint64(0), 256
	if len(args) > 2 {
		return nil, fmt.Errorf("rpcEvents(after=0,limit=256)")
	}
	if len(args) > 0 {
		n, e := yyNBTBoundedInteger(args[0], 0, 1<<53-1)
		if e != nil {
			return nil, e
		}
		after = uint64(n)
	}
	if len(args) > 1 {
		n, e := yyNBTBoundedInteger(args[1], 1, 65536)
		if e != nil {
			return nil, e
		}
		limit = int(n)
	}
	yyObserverRPCObserved.Lock()
	defer yyObserverRPCObserved.Unlock()
	rows := []map[string]any{}
	cursor := after
	for _, r := range yyObserverRPCObserved.Items[bm] {
		if r["cursor"].(uint64) > after {
			rows = append(rows, r)
			cursor = r["cursor"].(uint64)
			if len(rows) >= limit {
				break
			}
		}
	}
	raw, _ := json.Marshal(rows)
	var copied []map[string]any
	_ = json.Unmarshal(raw, &copied)
	return map[string]any{"events": copied, "cursor": cursor, "head_cursor": yyObserverRPCObserved.Cursors[bm], "source": "server_module_reported", "server_engine_verified": false}, nil
}

func yyObserverRPCCall(bm *BotManager, method string, args []any) (any, error) {
	event, expected, timeout := "Hello", "Capabilities", int64(3000)
	data := map[string]any{"protocol": 1}
	if method == "observerBridge.rpcQuery" {
		event, expected = "Query", "Reply"
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("rpcQuery(options,timeout_ms=3000)")
		}
		options, ok := args[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("需要只读查询对象")
		}
		query, ok := options["query"].(string)
		if !ok || query == "" || len(query) > 128 {
			return nil, fmt.Errorf("需要query名称")
		}
		data["query"] = query
		if target, ok := options["target"].(string); ok {
			if len(target) > 256 {
				return nil, fmt.Errorf("target过长")
			}
			data["target"] = target
		}
		args = args[1:]
	} else if len(args) > 1 {
		return nil, fmt.Errorf("rpcProbe(timeout_ms=3000)")
	}
	if len(args) > 0 {
		var err error
		timeout, err = yyNBTBoundedInteger(args[0], 1, 30000)
		if err != nil {
			return nil, err
		}
	}
	if bm == nil {
		return nil, fmt.Errorf("未连接")
	}
	bm.mu.Lock()
	conn := bm.conn
	bm.mu.Unlock()
	if conn == nil {
		return nil, fmt.Errorf("未连接")
	}
	id := uuid.NewString()
	data["request_id"] = id
	wait := make(chan map[string]any, 1)
	yyObserverRPCPending.Lock()
	yyObserverRPCPending.Items[id] = yyObserverRPCWaiter{Bot: bm, Reply: wait}
	yyObserverRPCPending.Unlock()
	defer func() {
		yyObserverRPCPending.Lock()
		delete(yyObserverRPCPending.Items, id)
		yyObserverRPCPending.Unlock()
	}()
	pk := &packet.PyRpc{OperationType: packet.PyRpcOperationTypeSend, Value: []any{"ModEventC2S", []any{"YouyuanBridge", "ObserverV1", event, data}, nil}}
	started := time.Now()
	if err := conn.WritePacket(pk); err != nil {
		return nil, err
	}
	if err := conn.Flush(); err != nil {
		return nil, err
	}
	timer := time.NewTimer(time.Duration(timeout) * time.Millisecond)
	defer timer.Stop()
	result := map[string]any{"request_id": id, "namespace": "YouyuanBridge", "system": "ObserverV1", "event": event, "expected_reply": expected, "submitted": true, "confirmed": false}
	select {
	case reply := <-wait:
		if reply["event"] == expected {
			result["confirmed"] = true
			result["reply"] = reply["data"]
		} else {
			result["error"] = "unexpected_reply"
		}
	case <-timer.C:
		result["error"] = "no_matching_reply"
		result["scope"] = "only this optional bridge module probe; not all PyRpc capabilities"
	case <-conn.Context().Done():
		result["error"] = "connection_closed"
	}
	result["elapsed_ms"] = time.Since(started).Milliseconds()
	return result, nil
}

func yyObserverRPCReceive(bm *BotManager, p *packet.PyRpc) {
	key, ok := yyRPCIngressIdentity(p)
	if !ok || key.Namespace != "YouyuanBridge" || key.System != "ObserverV1" || (key.Event != "Capabilities" && key.Event != "Reply" && key.Event != "Observation") {
		return
	}
	outer := p.Value.([]any)
	inner := outer[1].([]any)
	data, ok := inner[3].(map[string]any)
	if !ok {
		return
	}
	if _, err := yyNBTBoundedInteger(data["protocol"], 1, 1); err != nil {
		return
	}
	if key.Event == "Observation" {
		kind, ok := data["event_type"].(string)
		if !ok || kind == "" || len(kind) > 128 {
			return
		}
		raw, err := json.Marshal(data)
		if err != nil || len(raw) > 8192 {
			return
		}
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		yyObserverRPCObserved.Lock()
		yyObserverRPCObserved.Cursors[bm]++
		seq := yyObserverRPCObserved.Cursors[bm]
		row := map[string]any{"cursor": seq, "event_type": kind, "source": "server_module_reported", "transport": "PyRpc.ModEventS2C", "received_at": time.Now().UnixMilli(), "data": payload, "server_engine_verified": false}
		rows := append(yyObserverRPCObserved.Items[bm], row)
		limit := yyObservationSettings().EventHistory
		if len(rows) > limit {
			n := len(rows) - limit
			for i := 0; i < n; i++ {
				rows[i] = nil
			}
			rows = rows[n:]
		}
		yyObserverRPCObserved.Items[bm] = rows
		yyObserverRPCObserved.Unlock()
		yyServerObservationEmit(bm, row)
		return
	}
	id := yyRPCString(data["request_id"])
	if id == "" {
		return
	}
	yyObserverRPCPending.Lock()
	wait, exists := yyObserverRPCPending.Items[id]
	yyObserverRPCPending.Unlock()
	if !exists || wait.Bot != bm {
		return
	}
	select {
	case wait.Reply <- map[string]any{"event": key.Event, "data": data}:
	default:
	}
}
