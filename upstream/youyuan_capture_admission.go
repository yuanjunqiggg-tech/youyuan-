package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// This adapter never starts a worker, changes dimension or sends camera calls.
// The same authorized connection serves /list and admission commands.
func yyCaptureApplyAdmission(state *yyAdmissionState, c yyGlobalCaptureConfig, actor yyPoolActor, generation uint64, robots []string, robotsKnown bool) yyAdmissionResult {
	var bm *BotManager
	var session time.Time
	read := func() yyAdmissionSnapshot {
		x := yyAdmissionSnapshot{Scope: actor.ID, BoundOwner: actor.ID != "" && actor.ID == yyPoolBoundActor().ID, Robots: robots, RobotsKnown: robotsKnown}
		bm = getBotManagerLocked()
		if !x.BoundOwner || bm == nil {
			return x
		}
		bm.mu.Lock()
		x.Connected, x.OP, x.PrimaryName = bm.connected, bm.isOP, bm.display
		session = bm.connectTime
		x.Session = bm.serverCode + "/" + session.Format(time.RFC3339Nano)
		bm.mu.Unlock()
		x.Connected = x.Connected && bm.IsConnected()
		if !x.Connected || !x.OP || !x.RobotsKnown || x.PrimaryName == "" || session.IsZero() {
			return x
		}
		p, timed, err := bm.SendWSCommandWithTimeout("list", 2*time.Second)
		if err == nil && !timed {
			x.Names, x.Complete = yyAdmissionListNames(p)
		}
		return x
	}
	reject := func(name string, limit int) yyAdmissionAttempt {
		out := yyAdmissionAttempt{Name: name}
		if bm == nil || getBotManagerLocked() != bm || actor.ID != yyPoolBoundActor().ID {
			return out
		}
		yyGlobalCapture.Lock()
		current := yyGlobalCapture.Generation == generation && yyGlobalCapture.Config.EnforceLimit
		yyGlobalCapture.Unlock()
		bm.mu.Lock()
		current = current && bm.connected && bm.isOP && bm.connectTime.Equal(session)
		bm.mu.Unlock()
		current = current && bm.IsConnected()
		if !current {
			return out
		}
		q, _ := json.Marshal(name)
		reason, _ := json.Marshal(fmt.Sprintf("很抱歉，当前服务器已达到%d名玩家的接待上限。为保障游戏运行与连接稳定，本次暂无法接纳您的加入。感谢您的理解，期待您稍后再次加入。", limit))
		p, timed, err := bm.SendWSCommandWithTimeout(fmt.Sprintf("kick %s %s", q, reason), 2*time.Second)
		out.Sent = true
		out.Confirmed = err == nil && !timed && p != nil && p.SuccessCount > 0
		return out
	}
	return state.step(yyAdmissionPolicy{ObservationEnabled: c.Enabled, Enforce: c.EnforceLimit, Limit: c.PlayerLimit}, time.Now(), read, reject)
}
