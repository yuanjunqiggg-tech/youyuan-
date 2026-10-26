package main

import (
	"encoding/json"
	"sync"
	"time"
)

// One reconcile owns desired state. Per-account calls run concurrently, while
// each watch has exactly one cancellable timer and a bounded start operation.
func yyGlobalCaptureRun() {
	owned := map[string]bool{}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	admission := yyAdmissionState{}
	for range ticker.C {
		yyGlobalCapture.Lock()
		c, actor, g := yyGlobalCapture.Config, yyGlobalCapture.Actor, yyGlobalCapture.Generation
		yyGlobalCapture.Unlock()
		if actor.ID == "" || actor.ID != yyPoolBoundActor().ID {
			admission = yyAdmissionState{}
			continue
		}
		actor.Admin = false
		states := map[string]any{}
		selected := map[string]bool{}
		assignments := []yyCaptureAssignment{}
		summary := map[string]any{"updated_at": time.Now().Format(time.RFC3339Nano), "global_coverage_guaranteed": false}
		excluded, robotIdentitiesKnown := yyCaptureRobotNamesWithStatus()
		summary["robot_identities_verified"] = robotIdentitiesKnown
		staleSlots, unverifiedOnline := yyCaptureAuditLog.snapshot()
		if len(staleSlots) > 0 {
			summary["identity_skipped_offline_accounts"] = staleSlots
		}
		if len(unverifiedOnline) > 0 {
			summary["identity_unverified_online_accounts"] = unverifiedOnline
		}
		// Server admission is independent of observation. Its adapter only reads
		// a complete roster and may reject verified newcomers; it never observes
		// or provisions accounts when capture is disabled.
		admissionResult := yyCaptureApplyAdmission(&admission, c, actor, g, excluded, robotIdentitiesKnown)
		summary["admission"] = admissionResult
		summary["player_limit"], summary["enforce_player_limit"] = admissionResult.Limit, c.EnforceLimit
		for _, attempt := range admissionResult.Attempts {
			states["admission:"+attempt.Name] = map[string]any{"attempted": attempt.Sent, "confirmed": attempt.Confirmed}
		}
		if c.Enabled {
			players, queryError := yyCapturePlayers(excluded)
			if !c.AutoPlayers {
				allowed := map[string]bool{}
				for _, n := range c.Players {
					allowed[n] = true
				}
				filtered := []yyCapturePlayer{}
				for _, p := range players {
					if allowed[p.Name] {
						filtered = append(filtered, p)
					}
				}
				players = filtered
			}
			summary["real_players"], summary["dimension_query_error"], summary["players"], summary["excluded_robots"] = len(players), queryError, players, excluded
			desired := yyCaptureConfiguredDesired(c, len(players))
			summary["desired_observers"] = desired
			summary["candidate_accounts"] = len(c.Accounts)
			ready := []string{}
			var mu sync.Mutex
			var wg sync.WaitGroup
			byID := map[string]map[string]any{}
			for _, id := range c.Accounts {
				if !yyCaptureAllowed(actor, id) {
					continue
				}
				wg.Add(1)
				go func(id string) {
					defer wg.Done()
					v, e := yyCaptureCall(actor, id, "observerBridge.dimensionStatus", nil)
					if e != nil {
						v = map[string]any{"connected": false, "error": e.Error()}
					}
					if v["connected"] == true {
						watch, err := yyCaptureCall(actor, id, "observerBridge.watchStatus", nil)
						if err == nil && watch["active"] == true {
							cfg, _ := watch["config"].(map[string]any)
							if cfg["control_label"] != "global_capture" {
								v["busy"] = true
							}
						}
					}
					mu.Lock()
					byID[id] = v
					mu.Unlock()
				}(id)
			}
			wg.Wait()
			for _, id := range c.Accounts {
				v := byID[id]
				if v["busy"] == true {
					states[id] = map[string]any{"phase": "occupied", "error": "其他观察任务占用，不抢控制；使用其他账号替补"}
					continue
				}
				if id != "main" && v["connected"] == true {
					root := byID["main"]
					if root == nil {
						root, _ = yyCaptureCall(actor, "main", "observerBridge.dimensionStatus", nil)
					}
					want, _ := root["server"].(string)
					have, _ := v["server"].(string)
					if want != "" && have != "" && want != have {
						states[id] = map[string]any{"phase": "different_server", "error": "此账号在其他服务器，未擅自切换已有连接", "server": have}
						continue
					}
				}
				if v["connected"] == true && v["is_op"] == true {
					ready = append(ready, id)
				} else if v["connected"] == true {
					if name, ok := v["name"].(string); ok && desired > 0 {
						yyCaptureGrantOP(actor, id, name, c)
					}
					states[id] = map[string]any{"phase": "permission_required", "error": "已连接，但采集观察需要本机器人OP", "status": v}
				}
			}
			// Start only the shortfall, try the next candidate when one is cooling
			// down. A saved token is never reported as a successful game login.
			short := desired - len(ready)
			if !robotIdentitiesKnown {
				short = 0
				summary["provision_hold_reason"] = "已有机器人身份尚未确认，暂停增派，防止把机器人误认真人递归投放"
			}
			for _, id := range c.Accounts {
				if short <= 0 {
					break
				}
				v := byID[id]
				if v["connected"] == true || !yyCaptureAllowed(actor, id) {
					continue
				}
				s := yyCaptureProvision(actor, id, c)
				states[id] = s
				if s["phase"] == "starting" {
					short--
				}
			}
			assignments = yyCapturePlan(c, players, ready)
			if c.DimensionRouting != nil && !*c.DimensionRouting {
				for i := range assignments {
					if d, ok := byID[assignments[i].Account]["dimension"].(float64); ok {
						assignments[i].Dimension = int32(d)
					}
				}
			}
			if c.AutoFleet != nil && !*c.AutoFleet {
				assignments = nil
				homes := yyGlobalDimensionAssignments(c)
				for i, id := range ready {
					targets := []string{}
					d, known := homes[id]
					for _, p := range players {
						if !known || !p.Known || p.Dimension == d {
							targets = append(targets, p.Name)
						}
					}
					if len(targets) > 0 {
						assignments = append(assignments, yyCaptureAssignment{Account: id, Dimension: d, Players: targets, Phase: c.Interval * float64(i) / float64(max(1, len(ready)))})
					}
				}
			}
			for _, a := range assignments {
				selected[a.Account] = true
			}
			for _, a := range assignments {
				wg.Add(1)
				go func(a yyCaptureAssignment) {
					defer wg.Done()
					id := a.Account
					o := yyGlobalWatchOptions(c)
					o["auto_players"] = false
					o["players"] = a.Players
					o["exclude_players"] = excluded
					// World-up offset must not tilt with a player looking down.
					o["camera_relative"] = false
					o["camera_follow_rotation"] = false
					o["camera_back_blocks"] = 0
					o["observe_dimension"] = a.Dimension
					o["phase_seconds"] = a.Phase
					o["target_offset"] = a.Offset
					if len(a.Players) > 0 {
						o["player"] = a.Players[0]
					}
					fingerprint, _ := json.Marshal(o)
					v, e := yyCaptureCall(actor, id, "observerBridge.watchStatus", nil)
					if e == nil && v["active"] == true {
						cfg, _ := v["config"].(map[string]any)
						if cfg["control_label"] != "global_capture" {
							mu.Lock()
							states[id] = map[string]any{"phase": "occupied", "error": "已有其他插件观察任务，未替换"}
							mu.Unlock()
							return
						}
						if cfg["plan_signature"] != string(fingerprint) {
							_, e = yyCaptureCall(actor, id, "observerBridge.watchStop", nil)
							v["active"] = false
						}
					}
					role := map[string]any{}
					if e == nil {
						role, e = yyCaptureCall(actor, id, "observerBridge.assignDimension", []any{map[string]any{"dimension": a.Dimension}})
					}
					if e == nil && role["ready"] == true && v["active"] != true && len(a.Players) > 0 {
						o["plan_signature"] = string(fingerprint)
						v, e = yyCaptureCall(actor, id, "observerBridge.watchStart", []any{o})
					}
					if v == nil {
						v = map[string]any{}
					}
					v["assignment"] = a
					v["residency"] = role
					if e != nil {
						v["phase"] = "error"
						v["error"] = e.Error()
					} else if role["ready"] != true {
						v["phase"] = "waiting_dimension"
					} else if len(a.Players) == 0 {
						v["phase"] = "resident_idle"
					} else {
						v["phase"] = "observing"
					}
					mu.Lock()
					states[id] = v
					mu.Unlock()
				}(a)
			}
			wg.Wait()
		}
		// Also discover our saved watches after process restart; stop stale ones
		// even when the in-memory ownership map was empty.
		for _, id := range c.Accounts {
			if !selected[id] {
				owned[id] = true
			}
		}
		for id := range owned {
			if selected[id] {
				continue
			}
			v, e := yyCaptureCall(actor, id, "observerBridge.watchStatus", nil)
			if e == nil {
				cfg, _ := v["config"].(map[string]any)
				if cfg["control_label"] == "global_capture" {
					yyCaptureCall(actor, id, "observerBridge.watchStop", nil)
				}
			}
			delete(owned, id)
		}
		for id := range selected {
			owned[id] = true
		}
		yyCaptureRetire(actor, selected, c)
		for id, v := range yyCaptureIdleStates(actor, selected) {
			if _, present := states[id]; !present {
				states[id] = v
			}
		}
		summary["assignments"] = assignments
		summary["planned_observers"] = len(assignments)
		active := 0
		for _, s := range states {
			v, _ := s.(map[string]any)
			if v["phase"] == "observing" && v["active"] == true {
				active++
			}
		}
		summary["active_observers"] = active
		if desired, ok := summary["desired_observers"].(int); ok {
			summary["shortfall"] = max(0, desired-active)
		}
		yyGlobalCapture.Lock()
		if yyGlobalCapture.Generation == g {
			yyGlobalCapture.States = states
			yyGlobalCapture.Plan = summary
		}
		yyGlobalCapture.Unlock()
	}
}
func yyCaptureRoster(excluded []string) (map[string]bool, int) {
	ex := map[string]bool{}
	for _, n := range excluded {
		ex[n] = true
	}
	out := map[string]bool{}
	for n := range playerInfoMgr.OnlineSet() {
		if !ex[n] {
			out[n] = true
		}
	}
	return out, len(out)
}
