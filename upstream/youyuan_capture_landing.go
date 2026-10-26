package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
)

type yyCaptureLanding struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func yyCaptureLandingsValidate(points map[string]yyCaptureLanding) error {
	for dimension, p := range points {
		if dimension != "0" && dimension != "1" && dimension != "2" {
			return fmt.Errorf("落点维度仅支持0/1/2")
		}
		for _, v := range []float64{p.X, p.Y, p.Z} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("落点坐标必须为有限数字")
			}
		}
		if math.Abs(p.X) > 30000000 || math.Abs(p.Z) > 30000000 || p.Y < -64 || p.Y > 320 {
			return fmt.Errorf("落点坐标超出世界范围")
		}
	}
	return nil
}

func yyCaptureLandingMatches(p yyCaptureLanding, d int32, row game_interface.TargetQueryingInfo) bool {
	return int32(row.Dimension) == d && math.Abs(float64(row.Position.X)-p.X) < 3 && math.Abs(float64(row.Position.Z)-p.Z) < 3 && math.Abs(float64(row.Position.Y)-1.6200104-p.Y) < 3
}

var yyCaptureLandings = struct {
	sync.Mutex
	running map[string]bool
	cancel  map[string]chan struct{}
	states  map[string]map[string]any
}{running: map[string]bool{}, cancel: map[string]chan struct{}{}, states: map[string]map[string]any{}}

func yyCaptureCancelLanding(id string) {
	yyCaptureLandings.Lock()
	defer yyCaptureLandings.Unlock()
	if cancel := yyCaptureLandings.cancel[id]; cancel != nil {
		close(cancel)
		delete(yyCaptureLandings.cancel, id)
	}
}

func yyCaptureLandingStates() map[string]any {
	yyCaptureLandings.Lock()
	defer yyCaptureLandings.Unlock()
	out := map[string]any{}
	for id, s := range yyCaptureLandings.states {
		out["landing:"+id] = s
	}
	return out
}

func yyCaptureLandingPending(id string) bool {
	yyCaptureLandings.Lock()
	defer yyCaptureLandings.Unlock()
	return yyCaptureLandings.running[id]
}

// Arm before child authentication: a bot in a bad saved chunk may never finish login.
func yyCaptureArmLanding(a yyManagedAccount) error {
	if !a.CaptureOnly {
		return nil
	}
	yyGlobalCapture.Lock()
	c := yyGlobalCapture.Config
	yyGlobalCapture.Unlock()
	d, assigned := yyGlobalDimensionAssignments(c)[a.ID]
	p, configured := c.DimensionLandings[fmt.Sprint(d)]
	if !assigned || !configured {
		return nil
	}
	if err := yyCaptureLandingsValidate(c.DimensionLandings); err != nil {
		return err
	}
	if !yyWatchName(a.Name) {
		return fmt.Errorf("落点救援需要已核实的游戏昵称")
	}
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() || !bm.IsOP() {
		return fmt.Errorf("落点救援需要同服主机器人在线且有OP")
	}
	bm.mu.Lock()
	server := bm.serverCode
	bm.mu.Unlock()
	if server != a.Server {
		return fmt.Errorf("落点救援不能跨服务器操作")
	}
	if busy, reasons := yySpatialJobState(bm); busy {
		return fmt.Errorf("空间作业正在进行（%v），未启动落点救援", reasons)
	}
	if status, e := yyChildJSON(a, "GET", "/api/bot/status", nil); e == nil && status["connected"] == true {
		return nil
	}
	yyCaptureLandings.Lock()
	if yyCaptureLandings.running[a.ID] {
		yyCaptureLandings.Unlock()
		return fmt.Errorf("此账号上一轮落点救援尚未结束")
	}
	for id, running := range yyCaptureLandings.running {
		if running {
			yyCaptureLandings.Unlock()
			return fmt.Errorf("等待上一台采集机器人确认落点: %s", id)
		}
	}
	yyCaptureLandings.running[a.ID] = true
	cancel := make(chan struct{})
	yyCaptureLandings.cancel[a.ID] = cancel
	yyCaptureLandings.Unlock()
	name, _ := json.Marshal(a.Name)
	command := fmt.Sprintf("execute in %s run tp %s %.3f %.3f %.3f", yyWatchDimension(d), name, p.X, p.Y, p.Z)
	// One acknowledged request at a time; no fire-and-forget queue or per-player fanout.
	_, timed, err := bm.SendWSCommandWithTimeout(command, 700*time.Millisecond)
	if err != nil || timed {
		yyCaptureLandings.Lock()
		delete(yyCaptureLandings.running, a.ID)
		delete(yyCaptureLandings.cancel, a.ID)
		yyCaptureLandings.Unlock()
		return fmt.Errorf("落点救援未能提前启动，未继续登录")
	}
	go func() {
		phase, message := "failed", "60秒内未确认落点及子连接，已停止本次连接"
		var actual any
		state := func() {
			yyCaptureLandings.Lock()
			yyCaptureLandings.states[a.ID] = map[string]any{"phase": phase, "dimension": d, "target": p, "verified_position": actual, "error": message, "updated_at": time.Now().Format(time.RFC3339Nano)}
			yyCaptureLandings.Unlock()
		}
		phase = "placing"
		message = ""
		state()
		defer func() {
			if phase == "failed" {
				if current, ok := yyManagedFind(a.ID); ok && current.CaptureOnly && current.Enabled && current.Slot == a.Slot {
					if _, e := yyPoolCall(yyPoolActor{ID: yyPoolOwner(a), Scope: "control"}, "accountsPool.stop", []any{a.ID}); e != nil {
						message += "；停止失败: " + e.Error()
					}
				}
			}
			state()
			yyCaptureLandings.Lock()
			delete(yyCaptureLandings.running, a.ID)
			delete(yyCaptureLandings.cancel, a.ID)
			yyCaptureLandings.Unlock()
		}()
		deadline := time.Now().Add(60 * time.Second)
		lastQuery := time.Time{}
		timeouts := 0
		for time.Now().Before(deadline) {
			select {
			case <-cancel:
				phase = "cancelled"
				message = "账号已主动停止，未影响后续连接"
				return
			default:
			}
			yyGlobalCapture.Lock()
			currentConfig := yyGlobalCapture.Config
			yyGlobalCapture.Unlock()
			current, configured := currentConfig.DimensionLandings[fmt.Sprint(d)]
			role, assigned := yyGlobalDimensionAssignments(currentConfig)[a.ID]
			if !configured || current != p || !assigned || role != d {
				phase = "cancelled"
				message = "维度落点配置已改变"
				return
			}
			if current, ok := yyManagedFind(a.ID); !ok || !current.Enabled || !current.CaptureOnly || current.Slot != a.Slot {
				phase = "cancelled"
				message = "账号已停止或用途改变"
				return
			}
			if !bm.IsConnected() {
				phase = "failed"
				message = "主连接断开，已停止救援与子连接"
				return
			}
			if busy, reasons := yySpatialJobState(bm); busy {
				phase = "failed"
				message = fmt.Sprintf("空间作业正在进行（%v），未并行传送", reasons)
				return
			}
			began := time.Now()
			receipt, timed, e := bm.SendWSCommandWithTimeout(command, 700*time.Millisecond)
			if e != nil || timed {
				timeouts++
			} else {
				timeouts = 0
			}
			if timeouts >= 2 {
				phase = "failed"
				message = "连续两次无回执，停止发送及子连接"
				return
			}
			if e == nil && !timed && receipt != nil && receipt.SuccessCount > 0 && time.Since(lastQuery) >= time.Second {
				lastQuery = time.Now()
				q, timed, e := bm.SendWSCommandWithTimeout("querytarget "+string(name), 700*time.Millisecond)
				rows := yyCaptureQueryRows(q)
				if e == nil && !timed && len(rows) == 1 && yyCaptureLandingMatches(p, d, rows[0]) {
					actual = rows[0]
					if status, e := yyChildJSON(a, "GET", "/api/bot/status", nil); e == nil && status["connected"] == true {
						phase = "verified"
						message = ""
						return
					}
				}
			}
			if remaining := 100*time.Millisecond - time.Since(began); remaining > 0 {
				time.Sleep(remaining)
			}
		}
		phase = "failed"
		message = "60秒内未确认落点及子连接，已停止本次连接"
	}()
	return nil
}
