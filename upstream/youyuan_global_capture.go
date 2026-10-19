package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type yyGlobalCaptureConfig struct {
	Enabled           bool                 `json:"enabled"`
	Accounts          []string             `json:"accounts"`
	Players           []string             `json:"players"`
	AutoPlayers       bool                 `json:"auto_players"`
	Mode              string               `json:"mode"`
	Interval          float64              `json:"interval_seconds"`
	Height            float64              `json:"camera_height_blocks"`
	Pitch             float64              `json:"camera_pitch_degrees"`
	Yaw               float64              `json:"camera_yaw_degrees"`
	// Pointer so an old global-capture.json (written before the field existed)
	// keeps the safe default instead of silently disabling body proximity.
	BodyRadius        *float64             `json:"body_radius_blocks"`
	PauseJobs         bool                 `json:"pause_during_jobs"`
	DimensionRouting  *bool                `json:"dimension_routing"`
	DimensionAccounts map[string]string    `json:"dimension_accounts"`
	AutoFleet         *bool                `json:"auto_fleet"`
	AutoProvision     *bool                `json:"auto_provision"`
	AccountLimit      int                  `json:"account_limit"`
	FleetSteps        []yyCaptureFleetStep `json:"fleet_steps"`
	ResidentObservers int                  `json:"resident_observers"`
	IdleSeconds       int                  `json:"idle_seconds"`
	PlayerLimit       int                  `json:"player_limit"`
	EnforceLimit      bool                 `json:"enforce_player_limit"`
}

var yyGlobalCapture = struct {
	sync.Mutex
	Config     yyGlobalCaptureConfig
	Actor      yyPoolActor
	Path       string
	Generation uint64
	States     map[string]any
	Plan       any
}{Config: yyGlobalCaptureConfig{Mode: "camera", Interval: .015, Height: 6, Pitch: 90, PauseJobs: true, AutoPlayers: true}, States: map[string]any{}}

var yyGlobalCaptureLoopOnce sync.Once

// yyGlobalBodyRadius resolves the effective body radius. Absent means the safe
// default (64 blocks) because the server only broadcasts entity and attribute
// packets near the bot body; an explicit 0 disables the proximity command.
func yyGlobalBodyRadius(c yyGlobalCaptureConfig) float64 {
	if c.BodyRadius == nil {
		return 64
	}
	return *c.BodyRadius
}

func yyGlobalWatchOptions(c yyGlobalCaptureConfig) map[string]any {
	return map[string]any{"players": c.Players, "auto_players": c.AutoPlayers, "camera_selector": true, "mode": c.Mode, "interval_seconds": c.Interval, "camera_height_blocks": c.Height, "camera_pitch_degrees": c.Pitch, "camera_yaw_degrees": c.Yaw, "body_radius_blocks": yyGlobalBodyRadius(c), "pause_during_jobs": c.PauseJobs, "restore_position": true, "control_label": "global_capture"}
}
func yyGlobalDimensionAssignments(c yyGlobalCaptureConfig) map[string]int32 {
	out := map[string]int32{}
	if c.DimensionRouting != nil && !*c.DimensionRouting {
		return out
	}
	if len(c.DimensionAccounts) > 0 {
		for key, id := range c.DimensionAccounts {
			if id != "" {
				switch key {
				case "0":
					out[id] = 0
				case "1":
					out[id] = 1
				case "2":
					out[id] = 2
				}
			}
		}
		return out
	}
	for i, id := range c.Accounts {
		out[id] = int32(i % 3)
	}
	return out
}
func yyGlobalCaptureValidate(c yyGlobalCaptureConfig, actor yyPoolActor) error {
	if c.ResidentObservers < 0 || c.ResidentObservers > 3 {
		return fmt.Errorf("常驻维度数需1..3，0使用默认3维度；实际数量受可用账号限制")
	}
	if err := yyCaptureFleetStepsValidate(c.FleetSteps); err != nil {
		return err
	}
	if c.AccountLimit < 0 || c.AccountLimit > yyAccountProcessSlots || c.IdleSeconds < 0 || c.IdleSeconds > 3600 || c.PlayerLimit < 0 || c.PlayerLimit > 1000 {
		return fmt.Errorf("基础投放数量需0..32（0自动），空闲回收0..3600秒，真人上限0..1000（0为22）")
	}
	if actor.Scope == "read" {
		return fmt.Errorf("需要账号池控制权限")
	}
	if len(c.Accounts) == 0 && c.Enabled {
		return fmt.Errorf("请至少选择一个本人或明确共享的机器人账号")
	}
	seen := map[string]bool{}
	own := yyCaptureAvailable(actor)
	allowed := map[string]bool{"main": actor.ID == yyPoolBoundActor().ID}
	for _, a := range own {
		allowed[a.ID] = true
	}
	for _, id := range c.Accounts {
		if !allowed[id] || seen[id] {
			return fmt.Errorf("仅可选择本人或明确共享的有效账号，且不能重复")
		}
		seen[id] = true
	}
	seenDimensionAccounts := map[string]bool{}
	for dimension, id := range c.DimensionAccounts {
		if dimension != "0" && dimension != "1" && dimension != "2" {
			return fmt.Errorf("维度仅支持0主世界/1下界/2末地")
		}
		if id != "" {
			if !seen[id] || seenDimensionAccounts[id] {
				return fmt.Errorf("维度机器人必须已选中，每个账号仅负责一个维度")
			}
			seenDimensionAccounts[id] = true
		}
	}
	if _, e := yyWatchParse(yyGlobalWatchOptions(c)); e != nil {
		return e
	}
	return nil
}
func yyGlobalCaptureRoutes(mux *http.ServeMux, root string) {
	if yyAccounts.Worker {
		return
	}
	yyGlobalCapture.Lock()
	yyGlobalCapture.Path = filepath.Join(root, "global-capture.json")
	var saved struct {
		Config yyGlobalCaptureConfig
		Actor  yyPoolActor
	}
	if raw, e := os.ReadFile(yyGlobalCapture.Path); e == nil && json.Unmarshal(raw, &saved) == nil {
		yyGlobalCapture.Config = saved.Config
		if yyGlobalCapture.Config.DimensionRouting == nil {
			yes := true
			yyGlobalCapture.Config.DimensionRouting = &yes
		}
		yyGlobalCapture.Actor = saved.Actor
	}
	yyGlobalCapture.Unlock()
	mux.HandleFunc("/api/youyuan/global-capture", func(w http.ResponseWriter, r *http.Request) {
		actor, e := yyPoolActorRequest(r)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if actor.ID != yyPoolBoundActor().ID {
			writeJSON(w, map[string]any{"ok": false, "error": "只能配置自己设备的账号池"})
			return
		}
		if r.Method == "POST" {
			var patch map[string]json.RawMessage
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&patch) != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "配置格式无效"})
				return
			}
			yyGlobalCapture.Lock()
			previous := yyGlobalCapture.Config
			yyGlobalCapture.Unlock()
			base, _ := json.Marshal(previous)
			fields := map[string]json.RawMessage{}
			json.Unmarshal(base, &fields)
			for key, value := range patch {
				if _, known := fields[key]; !known {
					writeJSON(w, map[string]any{"ok": false, "error": "未知配置项：" + key})
					return
				}
				fields[key] = value
			}
			merged, _ := json.Marshal(fields)
			var c yyGlobalCaptureConfig
			if json.Unmarshal(merged, &c) != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "配置参数类型无效"})
				return
			}
			c = yyGlobalCaptureForActor(c, actor)
			if e := yyGlobalCaptureValidate(c, actor); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			raw, _ := json.MarshalIndent(map[string]any{"Config": c, "Actor": actor}, "", "  ")
			yyGlobalCapture.Lock()
			e := os.WriteFile(yyGlobalCapture.Path+".tmp", raw, 0600)
			if e == nil {
				e = os.Rename(yyGlobalCapture.Path+".tmp", yyGlobalCapture.Path)
			}
			if e == nil {
				yyGlobalCapture.Config = c
				yyGlobalCapture.Actor = actor
				yyGlobalCapture.Generation++
			}
			yyGlobalCapture.Unlock()
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "配置保存失败"})
				return
			}
		} else if r.Method != "GET" {
			http.Error(w, "GET/POST required", 405)
			return
		}
		yyGlobalCapture.Lock()
		defer yyGlobalCapture.Unlock()
		writeJSON(w, map[string]any{"ok": true, "config": yyGlobalCapture.Config, "advanced_allowed": actor.Admin, "states": yyGlobalCapture.States, "plan": yyGlobalCapture.Plan, "candidates": yyCaptureCandidates(actor), "dimension_assignments": yyGlobalDimensionAssignments(yyGlobalCapture.Config), "single_switch_replaces_residents": false, "global_coverage_guaranteed": false, "note": "自动调度按当前真人名单与可验证维度分配；机器人全部排除。借调可能改变观察机身体维度；纯相机不跟随TP玩家。submitted仅为提交；未知维度明确待探测，不承诺零漏包。"})
	})
	yyGlobalCaptureLoopOnce.Do(func() { go yyGlobalCaptureRun() })
}

// Ordinary users manage their own authorized accounts and capacity, not transport internals.
func yyGlobalCaptureForActor(c yyGlobalCaptureConfig, actor yyPoolActor) yyGlobalCaptureConfig {
	if actor.Admin {
		return c
	}
	yes := true
	c.Mode = "camera"
	c.Interval, c.Height, c.Pitch, c.Yaw = .015, 6, 90, 0
	c.PauseJobs, c.AutoPlayers = true, true
	c.Players = nil
	c.AutoFleet, c.AutoProvision, c.DimensionRouting = &yes, &yes, &yes
	c.ResidentObservers = 3
	return c
}
