package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func xpValue(n int) *int { return &n }
func xpFixtureView() yyExperienceView {
	return yyExperienceView{Session: "fixture-session", Player: "测试机器人", Connected: true, CanModify: true, Level: xpValue(5), LevelAt: time.Now()}
}
func xpRequest(id string) yyExperienceChange {
	return yyExperienceChange{RequestID: id, ExpectedSession: "fixture-session", Operation: "set_level", ExpectedLevel: xpValue(5), Amount: 7}
}
func TestPlayerExperienceUsesRealAttributesAndZeroIsKnown(t *testing.T) {
	bm := &BotManager{connected: true, isOP: true, runtimeID: 73, display: "fixture"}
	t.Cleanup(func() { yySenses.Lock(); delete(yySenses.Items, bm); yySenses.Unlock() })
	if v := yyPlayerExperience(bm); v.Level != nil || v.Progress != nil {
		t.Fatal("missing became zero", v)
	}
	s := yySenseFor(bm)
	s.Lock()
	a := s.actor(73, time.Now())
	yyObserveAttribute(a, protocol.AttributeValue{Name: "minecraft:player.level", Value: 0, Max: 24791}, time.Now(), "fixture")
	yyObserveAttribute(a, protocol.AttributeValue{Name: "minecraft:player.experience", Value: .25, Max: 1}, time.Now(), "fixture")
	s.Unlock()
	v := yyPlayerExperience(bm)
	if v.Level == nil || *v.Level != 0 || v.Progress == nil || *v.Progress != .25 || v.TotalXP != nil {
		t.Fatal(v)
	}
	bm.connected = false
	if v = yyPlayerExperience(bm); v.Level != nil || v.Progress != nil || v.CanModify {
		t.Fatal("disconnected cache presented as current", v)
	}
}
func TestPlayerExperienceValidatesBeforeWriting(t *testing.T) {
	v := xpFixtureView()
	for _, mode := range []string{"session", "permission", "unknown", "stale-level", "injection", "negative-points"} {
		t.Run(mode, func(t *testing.T) {
			x := v
			p := xpRequest("validation")
			switch mode {
			case "session":
				p.ExpectedSession = "old"
			case "permission":
				x.CanModify = false
			case "unknown":
				x.Level = nil
			case "stale-level":
				p.ExpectedLevel = xpValue(4)
			case "injection":
				x.Player = "name\nkill @a"
			case "negative-points":
				p.Operation = "add_points"
				p.Amount = -1
			}
			if _, _, e := yyExperienceCommand(x, p); e == nil {
				t.Fatal("unsafe write accepted")
			}
		})
	}
	cmd, target, e := yyExperienceCommand(v, xpRequest("normal-change"))
	if e != nil || cmd != `xp 2L "测试机器人"` || *target != 7 {
		t.Fatal(cmd, target, e)
	}
}
func TestPlayerExperienceConfirmsReadbackAndDoesNotReplay(t *testing.T) {
	bm := &BotManager{}
	v := xpFixtureView()
	p := xpRequest("receipt-unique")
	writes := 0
	send := func(cmd string) (*packet.CommandOutput, bool, error) {
		writes++
		v.Level = xpValue(7)
		v.LevelAt = v.LevelAt.Add(time.Second)
		return &packet.CommandOutput{SuccessCount: 1}, false, nil
	}
	r := yyApplyExperience(context.Background(), bm, p, func() yyExperienceView { return v }, send)
	if !r.OK || r.Status != "verified" || !r.AttributeVerified {
		t.Fatal(r)
	}
	again := yyApplyExperience(context.Background(), bm, p, func() yyExperienceView { return v }, send)
	if writes != 1 || again.Status != "verified" {
		t.Fatal(writes, again)
	}
	p.Amount = 8
	conflict := yyApplyExperience(context.Background(), bm, p, func() yyExperienceView { return v }, send)
	if writes != 1 || conflict.Status != "not_sent" {
		t.Fatal(conflict, writes)
	}
}
func TestPlayerExperienceUnknownAndRejectedAreNotSuccess(t *testing.T) {
	for _, mode := range []string{"timeout", "rejected"} {
		t.Run(mode, func(t *testing.T) {
			bm := &BotManager{}
			v := xpFixtureView()
			writes := 0
			p := xpRequest("receipt-" + mode)
			send := func(string) (*packet.CommandOutput, bool, error) {
				writes++
				if mode == "timeout" {
					return nil, true, fmt.Errorf("timeout")
				}
				return &packet.CommandOutput{}, false, nil
			}
			r := yyApplyExperience(context.Background(), bm, p, func() yyExperienceView { return v }, send)
			if r.OK || r.AttributeVerified || r.ServerAccepted {
				t.Fatal(r)
			}
			yyApplyExperience(context.Background(), bm, p, func() yyExperienceView { return v }, send)
			if writes != 1 {
				t.Fatal("write replayed")
			}
		})
	}
}
func TestPlayerExperienceAPIAndMCPDefinitions(t *testing.T) {
	for _, name := range []string{"player.experience", "player.changeExperience"} {
		if yyExtraMethods[name] == "" {
			t.Fatal("undocumented", name)
		}
	}
	old := botMgr
	botMgr = nil
	t.Cleanup(func() { botMgr = old })
	w := httptest.NewRecorder()
	yyPlayerExperienceHandler(w, httptest.NewRequest("GET", "/api/youyuan/player/experience", nil))
	var result map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil || result["ok"] != true {
		t.Fatal(w.Body.String(), e)
	}
	w = httptest.NewRecorder()
	yyPlayerExperienceHandler(w, httptest.NewRequest(http.MethodPost, "/api/youyuan/player/experience", bytes.NewBufferString(`{"request_id":"api-fixture","amount":1,"operation":"add_levels"}`)))
	if !strings.Contains(w.Body.String(), `"status":"not_sent"`) {
		t.Fatal(w.Body.String())
	}
}
