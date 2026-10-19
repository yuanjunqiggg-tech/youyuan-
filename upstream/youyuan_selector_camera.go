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
func (w *yyWatch) targets() []string {
	w.Bot.mu.Lock()
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
		next = time.Now().Add(interval)
		if busy, _ := yySpatialJobState(w.Bot); busy && (w.Config.PauseDuringJobs == nil || *w.Config.PauseDuringJobs) {
			continue
		}
		w.Bot.mu.Lock()
		self, dimension := w.Bot.display, w.Bot.dimension
		w.Bot.mu.Unlock()
		if w.Config.ObserveDimension != nil && dimension != *w.Config.ObserveDimension {
			continue
		}
		targets := w.targets()
		if len(targets) == 0 {
			continue
		}
		target := targets[index%len(targets)]
		index++
		allow := w.Config.PauseDuringJobs != nil && !*w.Config.PauseDuringJobs
		sent, e := yyObserverSend(w.Bot, yySelectorCameraCommand(self, target, w.Config), allow)
		if !sent {
			continue
		}
		if e != nil {
			w.Lock()
			w.Error = e.Error()
			w.Unlock()
			return
		}
		w.Lock()
		w.CurrentPlayer = target
		w.Submitted++
		w.SubmittedAt = time.Now()
		w.Unlock()
	}
}
