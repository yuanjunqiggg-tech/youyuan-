package main

import (
	"sort"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// Discovery is opt-in via capture of PyRpc. Never retain RPC arguments, issue
// guessed requests, change packet selection or infer a remote callable API.
type yyRPCIngressCatalog struct {
	mu           sync.Mutex
	rows         map[yyRPCIngressKey]*yyRPCIngressEntry
	order        []yyRPCIngressKey // FIFO by first observation, not per-packet history.
	observed     uint64
	unrecognised uint64
	evicted      uint64
	capacity     int
}

type yyRPCIngressKey struct {
	Envelope, Namespace, System, Event string
	Operation                          uint32
}

type yyRPCIngressEntry struct {
	Count       uint64
	First, Last int64
}

func yyRPCIngressName(value any) (string, bool) {
	// The protocol uses MessagePack binary names in some versions. Bound name
	// storage without truncating two distinct names into the same identity.
	switch v := value.(type) {
	case string:
		if len(v) > 0 && len(v) <= 256 {
			return v, true
		}
	case []byte:
		if len(v) > 0 && len(v) <= 256 {
			return string(v), true
		}
	}
	return "", false
}

func yyRPCIngressIdentity(p *packet.PyRpc) (key yyRPCIngressKey, ok bool) {
	if p == nil {
		return
	}
	outer, yes := p.Value.([]any)
	if !yes || len(outer) != 3 {
		return
	}
	key.Envelope, yes = yyRPCIngressName(outer[0])
	if !yes || key.Envelope != "ModEventS2C" {
		return
	}
	inner, yes := outer[1].([]any)
	if !yes || len(inner) != 4 {
		return
	}
	key.Namespace, yes = yyRPCIngressName(inner[0])
	if !yes {
		return
	}
	key.System, yes = yyRPCIngressName(inner[1])
	if !yes {
		return
	}
	key.Event, yes = yyRPCIngressName(inner[2])
	if !yes {
		return
	}
	key.Operation = p.OperationType
	return key, true
}

func (c *yyRPCIngressCatalog) observe(p *packet.PyRpc, capacity int) {
	if capacity < 1 {
		capacity = 1
	}
	if capacity > 65536 {
		capacity = 65536
	}
	key, recognised := yyRPCIngressIdentity(p)
	now := time.Now().UnixMilli()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.capacity = capacity
	c.observed++
	// Apply capacity reductions even for unrecognised frames. Eviction limits
	// diagnostic retention only; original packet and listeners remain untouched.
	for len(c.rows) > capacity {
		c.evictOldest()
	}
	if !recognised {
		c.unrecognised++
		return
	}
	if row := c.rows[key]; row != nil {
		row.Count++
		row.Last = now
		return
	}
	if c.rows == nil {
		c.rows = make(map[yyRPCIngressKey]*yyRPCIngressEntry)
	}
	if len(c.rows) >= capacity {
		c.evictOldest()
	}
	c.rows[key] = &yyRPCIngressEntry{Count: 1, First: now, Last: now}
	c.order = append(c.order, key)
}

func (c *yyRPCIngressCatalog) evictOldest() {
	delete(c.rows, c.order[0])
	c.order[0] = yyRPCIngressKey{} // Release name strings from the backing array.
	c.order = c.order[1:]
	c.evicted++
}

func yyRPCIngressSnapshot(a *yyPacketIngress) map[string]any {
	rows := []map[string]any{}
	var observed, unrecognised, evicted uint64
	capacity := 0
	if a != nil {
		c := &a.rpc
		c.mu.Lock()
		observed, unrecognised, evicted, capacity = c.observed, c.unrecognised, c.evicted, c.capacity
		for key, row := range c.rows {
			rows = append(rows, map[string]any{
				"envelope": key.Envelope, "namespace": key.Namespace, "system": key.System,
				"event": key.Event, "operation_type": key.Operation, "count": row.Count,
				"first_seen_at": row.First, "last_seen_at": row.Last,
			})
		}
		c.mu.Unlock()
	}
	sort.Slice(rows, func(i, j int) bool {
		for _, field := range []string{"namespace", "system", "event"} {
			x, y := rows[i][field].(string), rows[j][field].(string)
			if x != y {
				return x < y
			}
		}
		return rows[i]["operation_type"].(uint32) < rows[j]["operation_type"].(uint32)
	})
	return map[string]any{
		"events": rows, "observed": observed, "unrecognised": unrecognised, "evicted": evicted,
		"capacity": capacity, "collect_when": "explicit PyRpc capture is active",
		"scope":     "received ModEventS2C names only; no arguments retained, no proof of remote callable handlers or global broadcast",
		"retention": "FIFO unique event identities; counts restart after eviction; session reset on reconnect",
	}
}
