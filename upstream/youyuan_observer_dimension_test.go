package main

import (
	"bot-apk/config"
	"math"
	"strings"
	"testing"
)

func TestEmptyDimensionUsesOnlyExplicitLanding(t *testing.T) {
	d := int32(1)
	o := yyObserverDimensionOptions{Dimension: &d}
	if yyObserverDimensionTransfer("Observer", o) != "" {
		t.Fatal("unconfigured landing must not load arbitrary chunks")
	}
	o.Landing = &yyCaptureLanding{X: -10000, Y: 100, Z: -10000}
	if got := yyObserverDimensionTransfer("Observer", o); got != `execute in nether run tp "Observer" -10000.000 100.000 -10000.000` {
		t.Fatal(got)
	}
	o.Player = "Target"
	if got := yyObserverDimensionTransfer("Observer", o); got != `execute in nether at @a[name="Target"] run tp "Observer" ~ ~6 ~` {
		t.Fatal("live target must take precedence over idle landing", got)
	}
}

func TestDimensionAssignmentRejectsInvalidLandingBeforeSending(t *testing.T) {
	for _, p := range []yyCaptureLanding{{Y: 321}, {X: 30000001}, {Y: math.Inf(1)}} {
		bm := &BotManager{connected: true, dimension: 0, isOP: true}
		_, err := yyObserverDimensionCall(bm, "observerBridge.assignDimension", []any{map[string]any{"dimension": 1, "landing": p}})
		if err == nil {
			t.Fatal("invalid landing accepted", p)
		}
	}
}

func TestDimensionAssignmentDoesNotForceGameMode(t *testing.T) {
	config.InitConfig(t.TempDir())
	cfg := config.LoadConfig()
	cfg.AutoCreative = false
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int32{0, 1, 2, 6} {
		bm := &BotManager{connected: true, dimension: 1, gameMode: mode, display: "observer", isOP: true}
		v, err := yyObserverDimensionCall(bm, "observerBridge.assignDimension", []any{map[string]any{"dimension": 1}})
		if err != nil {
			t.Fatal(err)
		}
		status := v.(map[string]any)
		if status["ready"] != true || status["submitted"] != false || bm.gameMode != mode {
			t.Fatal(status, bm.gameMode)
		}
		if len(status["controls"].([]any)) != 0 {
			t.Fatal("sent game commands while already in assigned dimension")
		}
	}
}

func TestEmptyDimensionDoesNotLoadArbitraryLandingChunks(t *testing.T) {
	config.InitConfig(t.TempDir())
	bm := &BotManager{connected: true, dimension: 0, isOP: true, display: "observer"}
	v, err := yyObserverDimensionCall(bm, "observerBridge.assignDimension", []any{map[string]any{"dimension": 1}})
	if err != nil {
		t.Fatal(err)
	}
	s := v.(map[string]any)
	if s["ready"] != false || s["submitted"] != false || len(s["controls"].([]any)) != 0 {
		t.Fatal(s)
	}
}

func TestDimensionRolesPartitionAndDoNotPretendSingleCoverage(t *testing.T) {
	roles := yyGlobalDimensionAssignments(yyGlobalCaptureConfig{Accounts: []string{"a", "b", "c"}})
	if roles["a"] != 0 || roles["b"] != 1 || roles["c"] != 2 {
		t.Fatal(roles)
	}
	single := yyGlobalDimensionAssignments(yyGlobalCaptureConfig{Accounts: []string{"a"}})
	if len(single) != 1 || single["a"] != 0 {
		t.Fatal(single)
	}
	roles = yyGlobalDimensionAssignments(yyGlobalCaptureConfig{Accounts: []string{"a", "b", "c"}, DimensionAccounts: map[string]string{"0": "c", "1": "a", "2": "b"}})
	if roles["c"] != 0 || roles["a"] != 1 || roles["b"] != 2 {
		t.Fatal(roles)
	}
	off := false
	if got := yyGlobalDimensionAssignments(yyGlobalCaptureConfig{Accounts: []string{"a"}, DimensionRouting: &off}); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestRelativeCameraUsesNativeEntityFrameAndDimensionFilter(t *testing.T) {
	dim := int32(1)
	pitch := 90.0
	c, e := yyWatchParse(map[string]any{"auto_players": true, "observe_dimension": dim, "camera_relative": true, "camera_follow_rotation": true, "camera_height_blocks": 4, "camera_back_blocks": 5, "camera_pitch_degrees": pitch})
	if e != nil {
		t.Fatal(e)
	}
	command := yySelectorCameraCommand("observer", "target", c)
	for _, want := range []string{`execute as "observer" at @s at @a[name="target",rm=0]`, `rotated as @a[name="target",rm=0]`, `camera @s set minecraft:free pos ^ ^4.000 ^-5.000 rot 90.000 ~`} {
		if !strings.Contains(command, want) {
			t.Fatal(command, want)
		}
	}
	for _, forbidden := range []string{"querytarget", " tp ", "ease"} {
		if strings.Contains(command, forbidden) {
			t.Fatal(command)
		}
	}
	// 2026-10-26 第 27 条：维度上限 2 → 31，dm3/dm4 必须能配。
	if _, e = yyWatchParse(map[string]any{"auto_players": true, "observe_dimension": 3}); e != nil {
		t.Fatalf("observe_dimension=3（dm3）应被接受，实际报错 %v", e)
	}
	// 超出上限 31 的才是非法值
	if _, e = yyWatchParse(map[string]any{"auto_players": true, "observe_dimension": 40}); e == nil {
		t.Fatal("invalid dimension accepted")
	}
}
func TestResidencyStatusReportsBodyRatherThanCamera(t *testing.T) {
	bm := &BotManager{connected: true, dimension: 2, gameMode: 6, display: "observer", isOP: true}
	s := yyObserverDimensionStatus(bm)
	if s["dimension"] != int32(2) || s["game_mode"] != int32(6) || s["connected"] != true {
		t.Fatal(s)
	}
	bm.connected = false
	if yyObserverDimensionStatus(bm)["connected"] != false {
		t.Fatal("disconnected reported ready")
	}
}

func TestSelfSenseDoesNotReuseStaleActorDimension(t *testing.T) {
	bm := senseBot(t)
	bm.connected, bm.dimension, bm.gameMode = true, 1, 6
	s := yySenseFor(bm)
	s.Lock()
	s.Actors[42] = &yyActorObservation{RuntimeID: 42, Name: "Robot", Dimension: 0, Present: true}
	s.Actors[43] = &yyActorObservation{RuntimeID: 43, Name: "Player", Dimension: 2, Present: true}
	s.Unlock()
	for _, target := range []string{"self", "Robot", "42"} {
		v := yySenseActor(bm, target)
		if v["dimension"] != int32(1) || v["game_mode"] != int32(6) || v["dimension_known"] != true {
			t.Fatal(target, v)
		}
	}
	if v := yySenseActor(bm, "43"); v["dimension"] != int32(2) {
		t.Fatal("other actor was overwritten", v)
	}
	if _, err := yyObserverDimensionCall(bm, "observerBridge.assignDimension", []any{map[string]any{"dimension": 0}}); err == nil {
		t.Fatal("non-OP residency accepted")
	}
}
