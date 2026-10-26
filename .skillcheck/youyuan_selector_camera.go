package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

func yySelectorCameraCommand(self, target string, c yyWatchConfig) string {
	bot, _ := json.Marshal(self)
	player, _ := json.Marshal(target)
	if c.CameraRelative {
		selector := fmt.Sprintf("@a[name=%s,rm=0]", player)
		command := fmt.Sprintf("execute as %s at @s at %s", bot, selector)
		if c.CameraFollowRotation {
			command += " rotated as " + selector
		}
		command += fmt.Sprintf(" run camera @s set minecraft:free pos ^ ^%.3f ^%.3f", c.CameraHeight, -c.CameraBack)
		pitch := "~"
		if c.CameraPitch != nil {
			pitch = fmt.Sprintf("%.3f", *c.CameraPitch)
		}
		command += " rot " + pitch + " ~"
		return command
	}
	command := fmt.Sprintf("execute at %s run camera %s set minecraft:free pos ~ ~%.3f ~", player, bot, c.CameraHeight)
	if c.CameraPitch != nil {
		command += fmt.Sprintf(" rot %.3f %.3f", *c.CameraPitch, math.Mod(c.CameraYaw, 360))
	} else if c.CameraFacing {
		command += " facing " + string(player)
	}
	return command
}
// Throttled: the server-side distance test is cheap, but one extra command per
// camera switch is not free at 15ms intervals.
func (w *yyWatch) proximityCommand(self, target string) string {
	if w.Config.BodyRadius <= 0 {
		return ""
	}
	w.Lock()
	w.proximityTick++
	tick := w.proximityTick
	w.Unlock()
	if tick%10 != 1 {
		return ""
	}
	return yyWatchBodyCommand(self, target, w.Config.BodyRadius)
}

func (w *yyWatch) targets() []string {	w.Bot.mu.Lock()
	self := w.Bot.display
	w.Bot.mu.Unlock()
	names := append([]string(nil), w.Config.Players...)
	if len(names) == 0 && w.Config.Player != "" {
		names = []string{w.Config.Player}
	}
	if w.Config.AutoPlayers {
		names = nil
		for n := range playerInfoMgr.OnlineSet() {
			if yyWatchName(n) {
				names = append(names, n)
			}
		}
		sort.Strings(names)
	}
	excluded := map[string]bool{self: true}
	for _, n := range w.Config.ExcludePlayers {
		excluded[n] = true
	}
	// 红线（AGENTS.md 第 9 条）：观测目标必须排除已知机器人身份，否则机器人会互相
	// 观测、互相投放相机与传送，形成递归。reconcile 路径已在 youyuan_capture_reconcile.go
	// 里过滤过，但 watchStart 直连本函数时原先没有过滤（安全审查 H5），这里补成最后一道闸。
	for _, n := range yyCaptureRobotNames() {
		if n != "" {
			excluded[n] = true
		}
	}
	out := []string{}
	for _, n := range names {
		if !excluded[n] {
			out = append(out, n)
		}
	}
	return out
}
func (w *yyWatch) runSelectorCamera(ctx context.Context) {
	interval, _ := yyPatrolDuration(w.Config.Interval)
	next := time.Now().Add(time.Duration(w.Config.PhaseSeconds * float64(time.Second)))
	index := w.Config.TargetOffset
	deadline := time.Time{}
	if w.Config.Duration > 0 {
		d, _ := yyPatrolDuration(w.Config.Duration)
		deadline = time.Now().Add(d)
	}
	for ctx.Err() == nil && w.Bot.IsConnected() {
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return
		}
		// Fixed-rate scheduling, same contract as run(): a slow send must show up
		// as a missed interval instead of quietly stretching the period. The old
		// `time.Now().Add(interval)` made the real cycle interval+send cost, so
		// missed_intervals stayed 0 forever on the selector path — the path global
		// capture actually uses, which made the reported rate meaningless.
		next = next.Add(interval)
		if now := time.Now(); interval > 0 && next.Before(now) {
			w.Lock()
			w.Missed += uint64(now.Sub(next)/interval) + 1
			w.Unlock()
			next = now.Add(interval)
		}
		if busy, _ := yySpatialJobState(w.Bot); busy && (w.Config.PauseDuringJobs == nil || *w.Config.PauseDuringJobs) {
			continue
		}
		w.Bot.mu.Lock()
		self, dimension := w.Bot.display, w.Bot.dimension
		w.Bot.mu.Unlock()
		if w.Config.ObserveDimension != nil && dimension != *w.Config.ObserveDimension {
			// A stale body dimension used to spin here forever with no visible
			// reason. Record it so the scheduler/UI can act on it.
			w.Lock()
			w.DimensionSkips++
			w.LastSkip = "身体尚未进入分工维度"
			w.Error = w.LastSkip
			w.Unlock()
			continue
		}
		targets := w.targets()
		if len(targets) == 0 {
			continue
		}
		target := targets[index%len(targets)]
		index++
		allow := w.Config.PauseDuringJobs != nil && !*w.Config.PauseDuringJobs
		// Keep the body inside the broadcast radius. The selector-camera path
		// avoids querytarget, so the distance test runs on the server and the
		// teleport only happens when the body is actually too far away.
		if body := w.proximityCommand(self, target); body != "" {
			if sent, err := yyObserverSend(w.Bot, body, allow); sent && err != nil {
				w.Lock()
				w.SendFailures++
				w.Error = err.Error()
				w.Unlock()
			}
		}
		sent, e := yyObserverSend(w.Bot, yySelectorCameraCommand(self, target, w.Config), allow)
		if !sent {
			continue
		}
		if e != nil {
			w.Lock()
			w.SendFailures++
			w.Error = e.Error()
			failures := w.SendFailures
			w.Unlock()
			if failures >= 5 {
				return
			}
			continue
		}
		w.Lock()
		w.SendFailures = 0
		w.DimensionSkips = 0
		w.LastSkip = ""
		w.Error = ""
		w.CurrentPlayer = target
		w.Submitted++
		w.SubmittedAt = time.Now()
		w.Unlock()
	}
}
