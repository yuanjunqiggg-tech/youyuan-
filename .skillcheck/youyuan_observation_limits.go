package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// Limits concern retained observations/results, never command CPS or packet types.
type yyObservationLimits struct {
	EventQueue    int `json:"event_queue"`
	PriorityQueue int `json:"priority_queue"`
	RuntimeQueue  int `json:"runtime_queue"`
	AsyncTickets  int `json:"async_tickets"`
	EventHistory  int `json:"event_history"`
	Actors        int `json:"actors"`
	StaleMS       int `json:"stale_ms"`
}

var yyObservationConfig atomic.Value
var yyObservationConfigMu sync.Mutex
var yyObservationConfigPath string

func init() {
	yyObservationConfig.Store(yyObservationLimits{EventQueue: 256, PriorityQueue: 256, RuntimeQueue: 256, AsyncTickets: 128, EventHistory: 512, Actors: 2048, StaleMS: 15000})
}
func yyObservationSettings() yyObservationLimits {
	return yyObservationConfig.Load().(yyObservationLimits)
}
func yyValidateObservationLimits(v yyObservationLimits) error {
	for _, n := range []int{v.EventQueue, v.PriorityQueue, v.RuntimeQueue, v.AsyncTickets, v.EventHistory, v.Actors} {
		if n < 1 || n > 65536 {
			return fmt.Errorf("容量需1..65536，不限制协议包种类和Fast命令频率")
		}
	}
	if v.StaleMS < 1 || v.StaleMS > 86400000 {
		return fmt.Errorf("stale_ms需1..86400000")
	}
	return nil
}
func yyLoadObservationSettings(root string) {
	yyObservationConfigMu.Lock()
	defer yyObservationConfigMu.Unlock()
	yyObservationConfigPath = filepath.Join(root, "observation-limits.json")
	if b, e := os.ReadFile(yyObservationConfigPath); e == nil {
		v := yyObservationSettings()
		if json.Unmarshal(b, &v) == nil && yyValidateObservationLimits(v) == nil {
			yyObservationConfig.Store(v)
		}
	}
}
func yyConfigureObservationSettings(args []any) (any, error) {
	yyObservationConfigMu.Lock()
	defer yyObservationConfigMu.Unlock()
	v := yyObservationSettings()
	if len(args) > 0 {
		if _, ok := args[0].(map[string]any); !ok {
			return nil, fmt.Errorf("需要配置对象")
		}
		b, e := json.Marshal(args[0])
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &v); e != nil {
			return nil, e
		}
		if e = yyValidateObservationLimits(v); e != nil {
			return nil, e
		}
		if yyObservationConfigPath != "" {
			b, _ = json.MarshalIndent(v, "", "  ")
			temp := yyObservationConfigPath + ".tmp"
			if e = os.WriteFile(temp, b, 0600); e != nil {
				return nil, e
			}
			if e = os.Rename(temp, yyObservationConfigPath); e != nil {
				return nil, e
			}
		}
		yyObservationConfig.Store(v)
	}
	return map[string]any{"settings": v, "queue_changes_require_plugin_reload": true, "scope": "host", "packet_types_limited": false, "fast_command_cps_limited": false}, nil
}
