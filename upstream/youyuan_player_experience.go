package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

type yyExperienceView struct {
	Session    string    `json:"session"`
	Player     string    `json:"player"`
	Connected  bool      `json:"connected"`
	CanModify  bool      `json:"can_modify"`
	Level      *int      `json:"level"`
	Progress   *float32  `json:"progress"`
	LevelAt    time.Time `json:"level_updated_at"`
	ProgressAt time.Time `json:"progress_updated_at"`
	Source     string    `json:"source"`
	TotalXP    *int      `json:"total_xp"`
}

func yyPlayerExperience(bm *BotManager) yyExperienceView {
	v := yyExperienceView{Source: "UpdateAttributes"}
	if bm == nil {
		return v
	}
	bm.mu.Lock()
	v.Player = bm.display
	v.Connected = bm.connected
	v.CanModify = bm.connected && bm.isOP
	self := bm.runtimeID
	bm.mu.Unlock()
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	v.Session = s.Session
	if !v.Connected {
		return v
	}
	a := s.Actors[self]
	if a == nil {
		return v
	}
	if x, ok := a.Attributes["minecraft:player.level"]; ok && yyFiniteFloat(x.Value) && x.Value >= 0 && float64(x.Value) == math.Trunc(float64(x.Value)) {
		n := int(x.Value)
		v.Level = &n
		v.LevelAt = x.At
	}
	if x, ok := a.Attributes["minecraft:player.experience"]; ok && yyFiniteFloat(x.Value) && x.Value >= 0 && x.Value <= 1 {
		n := x.Value
		v.Progress = &n
		v.ProgressAt = x.At
	}
	// Attribute packets contain the level and current bar fraction, not a reliable
	// cumulative XP counter. Missing data is null, including after disconnect.
	return v
}

type yyExperienceChange struct {
	RequestID       string `json:"request_id"`
	ExpectedSession string `json:"expected_session"`
	ExpectedLevel   *int   `json:"expected_level,omitempty"`
	Operation       string `json:"operation"`
	Amount          int    `json:"amount"`
}

func yyExperienceCommand(v yyExperienceView, p yyExperienceChange) (string, *int, error) {
	if !v.Connected {
		return "", nil, fmt.Errorf("机器人未连接，未发送")
	}
	if !v.CanModify {
		return "", nil, fmt.Errorf("需要当前游戏账号的OP权限，未发送")
	}
	if p.ExpectedSession == "" || p.ExpectedSession != v.Session {
		return "", nil, fmt.Errorf("连接已经变化，请刷新等级数据，未发送")
	}
	if len(p.RequestID) < 8 || len(p.RequestID) > 100 || strings.ContainsAny(p.RequestID, "\r\n\x00") {
		return "", nil, fmt.Errorf("需要8–100字符的唯一request_id")
	}
	if v.Player == "" || strings.ContainsAny(v.Player, "\r\n\x00") {
		return "", nil, fmt.Errorf("未确认当前机器人游戏名")
	}
	delta := p.Amount
	var target *int
	suffix := "L"
	switch p.Operation {
	case "set_level":
		if p.Amount < 0 || p.Amount > 24791 {
			return "", nil, fmt.Errorf("目标等级需为0–24791")
		}
		if v.Level == nil || p.ExpectedLevel == nil || *v.Level != *p.ExpectedLevel {
			return "", nil, fmt.Errorf("当前等级未知或已变化，请刷新后再设置")
		}
		delta = p.Amount - *v.Level
		n := p.Amount
		target = &n
	case "add_levels":
		if delta < -24791 || delta > 24791 {
			return "", nil, fmt.Errorf("等级增减需在-24791至24791之间")
		}
		if v.Level != nil {
			n := *v.Level + delta
			if n < 0 {
				n = 0
			}
			if n > 24791 {
				return "", nil, fmt.Errorf("结果超出标准等级范围")
			}
			target = &n
		}
	case "add_points":
		if delta < 0 || delta > 1000000 {
			return "", nil, fmt.Errorf("经验点需为0–1000000；基岩版不使用负经验点命令")
		}
		suffix = ""
	default:
		return "", nil, fmt.Errorf("未知经验操作")
	}
	if delta == 0 {
		return "", target, nil
	}
	return fmt.Sprintf("xp %d%s %s", delta, suffix, strconv.Quote(v.Player)), target, nil
}

type yyExperienceResult struct {
	OK                bool             `json:"ok"`
	Status            string           `json:"status"`
	RequestID         string           `json:"request_id"`
	Submitted         bool             `json:"submitted"`
	ServerAccepted    bool             `json:"server_accepted"`
	AttributeVerified bool             `json:"attribute_verified"`
	Error             string           `json:"error,omitempty"`
	Before            yyExperienceView `json:"before"`
	After             yyExperienceView `json:"after"`
}
type yyExperienceReceipt struct {
	Fingerprint [32]byte
	Result      yyExperienceResult
}
type yyExperienceControl struct {
	sync.Mutex
	Receipts map[string]yyExperienceReceipt
	Order    []string
	At       time.Time
}

var yyExperienceControls = struct {
	sync.Mutex
	Items map[*BotManager]*yyExperienceControl
}{Items: map[*BotManager]*yyExperienceControl{}}

func yyExperienceControlFor(bm *BotManager) *yyExperienceControl {
	yyExperienceControls.Lock()
	defer yyExperienceControls.Unlock()
	c := yyExperienceControls.Items[bm]
	if c == nil {
		// Bounded retention across old robot sessions, no background process.
		if len(yyExperienceControls.Items) >= 128 {
			var oldest *BotManager
			at := time.Now()
			for k, v := range yyExperienceControls.Items {
				if v.At.Before(at) {
					oldest, at = k, v.At
				}
			}
			delete(yyExperienceControls.Items, oldest)
		}
		c = &yyExperienceControl{Receipts: map[string]yyExperienceReceipt{}}
		yyExperienceControls.Items[bm] = c
	}
	c.At = time.Now()
	return c
}
func yyApplyExperience(ctx context.Context, bm *BotManager, p yyExperienceChange, read func() yyExperienceView, send func(string) (*packet.CommandOutput, bool, error)) yyExperienceResult {
	c := yyExperienceControlFor(bm)
	c.Lock()
	defer c.Unlock()
	raw, _ := json.Marshal(p)
	hash := sha256.Sum256(raw)
	if old, ok := c.Receipts[p.RequestID]; ok {
		if old.Fingerprint != hash {
			return yyExperienceResult{Status: "not_sent", RequestID: p.RequestID, Error: "request_id已经用于另一操作"}
		}
		return old.Result
	}
	before := read()
	result := yyExperienceResult{RequestID: p.RequestID, Status: "not_sent", Before: before, After: before}
	command, target, e := yyExperienceCommand(before, p)
	if e != nil {
		result.Error = e.Error()
		return result
	}
	if command == "" {
		result.OK = true
		result.Status = "unchanged"
		return result
	}
	// Recheck identity immediately before sending. No automatic retry of writes.
	if read().Session != before.Session {
		result.Error = "连接已变化，未发送"
		return result
	}
	select {
	case <-ctx.Done():
		result.Error = ctx.Err().Error()
		return result
	default:
	}
	result.Submitted = true
	response, timedOut, e := send(command)
	result.After = read()
	if e != nil || timedOut || response == nil {
		result.Status = "outcome_unknown"
		result.Error = "未确认命令回执，可能已执行；刷新属性或使用原request_id查询，不能自动重发"
	} else if response.SuccessCount == 0 {
		result.Status = "rejected"
		result.Error = "服务器拒绝了经验修改，请检查游戏权限和命令回执"
	} else {
		result.OK = true
		result.ServerAccepted = true
		result.Status = "accepted"
		until := time.NewTimer(1200 * time.Millisecond)
		tick := time.NewTicker(40 * time.Millisecond)
	wait:
		for {
			v := read()
			result.After = v
			if v.Session != before.Session {
				break
			}
			verified := false
			if target != nil {
				verified = v.Level != nil && *v.Level == *target && v.LevelAt.After(before.LevelAt)
			} else {
				verified = v.LevelAt.After(before.LevelAt) || v.ProgressAt.After(before.ProgressAt)
			}
			if verified {
				result.AttributeVerified = true
				result.Status = "verified"
				break
			}
			select {
			case <-ctx.Done():
				break wait
			case <-until.C:
				break wait
			case <-tick.C:
			}
		}
		until.Stop()
		tick.Stop()
	}
	if len(c.Order) >= 64 {
		delete(c.Receipts, c.Order[0])
		c.Order = c.Order[1:]
	}
	c.Order = append(c.Order, p.RequestID)
	c.Receipts[p.RequestID] = yyExperienceReceipt{hash, result}
	return result
}
func yyPlayerExperienceHandler(w http.ResponseWriter, r *http.Request) {
	bm := getBotManagerLocked()
	if r.Method == http.MethodGet {
		v := yyPlayerExperience(bm)
		out := map[string]any{"ok": true, "experience": v, "idempotency_window": 64}
		if id := r.URL.Query().Get("request_id"); id != "" {
			c := yyExperienceControlFor(bm)
			c.Lock()
			old, ok := c.Receipts[id]
			c.Unlock()
			if ok {
				out["receipt"] = old.Result
			} else {
				out["receipt_known"] = false
			}
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var p yyExperienceChange
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); e != nil {
		writeJSON(w, map[string]any{"ok": false, "status": "not_sent", "error": "无效经验修改请求"})
		return
	}
	result := yyApplyExperience(r.Context(), bm, p, func() yyExperienceView { return yyPlayerExperience(bm) }, func(cmd string) (*packet.CommandOutput, bool, error) {
		return bm.SendWSCommandWithTimeout(cmd, 5*time.Second)
	})
	writeJSON(w, result)
}
func init() {
	yyExtraMethods["player.experience"] = "无参数；当前机器人实际游戏等级与经验条0–1，未同步为null，total_xp未知，不发送查询命令。"
	yyExtraMethods["player.changeExperience"] = "options={request_id,expected_session,operation:set_level|add_levels|add_points,amount,expected_level?}；当前机器人需游戏OP，原生xp命令，区分not_sent/outcome_unknown/rejected/accepted/verified；同request_id不重复发送。set_level需先读取并提交expected_level。"
}
