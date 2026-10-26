package main

import (
	"context"
	"encoding/json"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
	"math"
	"strings"
	"testing"
	"time"
)

func TestWatchCameraHeightPitchAndSelectorWithoutTeleport(t *testing.T) {
	for _, value := range []map[string]any{{"player": "p", "camera_height_blocks": math.Inf(1)}, {"player": "p", "camera_pitch_degrees": 91}, {"player": "p", "camera_pitch_degrees": 90, "camera_facing_player": true}} {
		if _, e := yyWatchParse(value); e == nil {
			t.Fatal(value)
		}
	}
	c, e := yyWatchParse(map[string]any{"player": "p", "camera_height_blocks": 8, "camera_pitch_degrees": 90})
	if e != nil {
		t.Fatal(e)
	}
	command, p := yyWatchCameraCommand("observer", c, mgl32.Vec3{100, 64, 100})
	if p[1] != 72 || !strings.Contains(command, "rot 90.000 0.000") || strings.Contains(command, "tp ") {
		t.Fatal(command, p)
	}
	c, e = yyWatchParse(map[string]any{"player": "p", "camera_facing_player": true})
	if e != nil {
		t.Fatal(e)
	}
	command, p = yyWatchCameraCommand("observer", c, mgl32.Vec3{100, 64, 100})
	if p[1] != 66 || !strings.HasSuffix(command, ` facing "p"`) {
		t.Fatal(command, p)
	}
}

func TestWatchRejectsInvalidModesAndIntervals(t *testing.T) {
	for _, c := range []map[string]any{{"player": "p", "mode": "bogus"}, {"player": "p", "interval_seconds": 0}, {"player": "p", "interval_seconds": math.Inf(1)}, {"player": "a\nkill @a"}} {
		if _, err := yyWatchParse(c); err == nil {
			t.Fatal(c)
		}
	}
	if c, err := yyWatchParse(map[string]any{"player": "p", "mode": "hybrid", "interval_seconds": .001}); err != nil || c.Interval != .001 {
		t.Fatal(c, err)
	}
}

func TestWatchMicrosecondConfigAndReportedDispatchLimit(t *testing.T) {
	for _, ms := range []float64{0.1, 0.001} {
		original := yyGlobalCaptureConfig{Interval: ms / 1000, Mode: "camera", AutoPlayers: true}
		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var saved yyGlobalCaptureConfig
		if err := json.Unmarshal(raw, &saved); err != nil {
			t.Fatal(err)
		}
		config, err := yyWatchParse(yyGlobalWatchOptions(saved))
		if err != nil {
			t.Fatal(err)
		}
		duration, err := yyPatrolDuration(config.Interval)
		if err != nil || duration != time.Duration(ms*float64(time.Millisecond)) {
			t.Fatalf("%gms became %v: %v", ms, duration, err)
		}
		w := &yyWatch{Bot: senseBot(t), Config: config, SubmissionGap: 53 * time.Millisecond}
		status := w.snapshot()
		if status["requested_interval_ms"] != ms || status["command_limit_per_second"] != 20 || status["minimum_command_interval_ms"] != float64(50) || status["last_host_submission_interval_ms"] != float64(53) {
			t.Fatal(status)
		}
	}
}

func TestMicrosecondIdleCameraCanCancelWithoutSending(t *testing.T) {
	bm := senseBot(t)
	bm.connected = true
	w := &yyWatch{Bot: bm, Config: yyWatchConfig{Interval: .000001}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.runSelectorCamera(ctx); close(done) }()
	time.Sleep(15 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("idle wait did not honor cancellation")
	}
	if w.Submitted != 0 || bm.controlCommands.Load() != 0 {
		t.Fatal("idle observer sent commands")
	}
}
func TestWatchFreshRegionEvidence(t *testing.T) {
	now := time.Now()
	target := mgl32.Vec3{500, 64, 500}
	if yyWatchRegionReady(target, now.Add(-time.Millisecond), target, now) {
		t.Fatal("accepted old region")
	}
	if yyWatchRegionReady(mgl32.Vec3{0, 64, 0}, now.Add(time.Millisecond), target, now) {
		t.Fatal("accepted another region")
	}
	if !yyWatchRegionReady(target, now.Add(time.Millisecond), target, now) {
		t.Fatal("rejected fresh matching region")
	}
}

func TestWatchFollowRadiusAvoidsRepeatedTeleportAndKeepsRecovery(t *testing.T) {
	body := mgl32.Vec3{0, 64, 0}
	if yyWatchNeedsTeleport(true, body, mgl32.Vec3{8, 64, 0}, 0, 0, 16) {
		t.Fatal("near observer should stay resident")
	}
	for _, c := range []struct {
		known     bool
		target    mgl32.Vec3
		dimension int32
		radius    float64
	}{
		{false, body, 0, 16}, {true, body, 1, 16}, {true, mgl32.Vec3{17, 64, 0}, 0, 16}, {true, body, 0, 0},
	} {
		if !yyWatchNeedsTeleport(c.known, body, c.target, 0, c.dimension, c.radius) {
			t.Fatal(c)
		}
	}
	if c, err := yyWatchParse(map[string]any{"player": "p"}); err != nil || c.FollowRadius != 16 {
		t.Fatal(c, err)
	}
	if _, err := yyWatchParse(map[string]any{"player": "p", "follow_radius_blocks": -1}); err == nil {
		t.Fatal("negative follow radius accepted")
	}
}
func TestObservationPlanExcludesBotsAndStaggersSixAccounts(t *testing.T) {
	p := yyObservationPlan([]string{"bot", "u1", "u2", "u3", "u4", "u5", "u6", "u7"}, []string{"bot"}, []string{"1", "2", "3", "4", "5", "6"}, .6, 48)
	assignments := p["assignments"].([]any)
	seen := map[string]bool{}
	for i, x := range assignments {
		v := x.(map[string]any)
		if math.Abs(v["phase_seconds"].(float64)-float64(i)*.1) > .00001 {
			t.Fatal(v)
		}
		for _, name := range v["players"].([]string) {
			if name == "bot" || seen[name] {
				t.Fatal(v)
			}
			seen[name] = true
		}
	}
	if len(seen) != 7 || p["global_coverage_guaranteed"] != false {
		t.Fatal(p)
	}
}
func TestObservationRejectsCrossServerOrOfflineHandoff(t *testing.T) {
	a := map[string]any{"connected": true, "server": "A"}
	for _, b := range []map[string]any{{"connected": false, "server": "A"}, {"connected": true, "server": "B"}, {"connected": true, "server": ""}} {
		if yyObservationSameServer(a, b) == nil {
			t.Fatal(b)
		}
	}
	if err := yyObservationSameServer(a, a); err != nil {
		t.Fatal(err)
	}
}

func TestObservationBotExclusionIsScopedToActualServer(t *testing.T) {
	views := []yyWorldView{
		{Data: map[string]any{"connected": true, "server": "A", "name": "our-bot"}},
		{Data: map[string]any{"connected": true, "server": "B", "name": "other-project-bot"}},
		{Data: map[string]any{"connected": false, "server": "A", "name": "offline-bot"}},
	}
	robots := yyObservationRobotNames(views, "A")
	if len(robots) != 1 || robots[0] != "our-bot" {
		t.Fatal(robots)
	}
	if len(yyObservationRobotNames(views, "")) != 0 {
		t.Fatal("unknown server exposed bot names")
	}
}
func TestSkinEmptyListPreservesCacheAndSyncMatchesOnlyOneUUID(t *testing.T) {
	pm := NewPlayerInfoManager()
	u := uuid.New()
	pixels := make([]byte, 64*64*4)
	pixels[0] = 91
	skin := protocol.Skin{SkinData: pixels, SkinImageWidth: 64, SkinImageHeight: 64}
	pm.HandlePacket(&packet.PlayerList{ActionType: packet.PlayerListActionAdd, Entries: []protocol.PlayerListEntry{{UUID: u, Username: "p", EntityUniqueID: 1, Skin: skin}}}, nil)
	pm.HandlePacket(&packet.PlayerList{ActionType: packet.PlayerListActionAdd, Entries: []protocol.PlayerListEntry{{UUID: u, Username: "p", EntityUniqueID: 1}}}, nil)
	b, _, _, ok := pm.GetPlayerSkin("p")
	if !ok || b[0] != 91 {
		t.Fatal("empty packet erased skin")
	}
	b[0] = 3
	b, _, _, _ = pm.GetPlayerSkin("p")
	if b[0] != 91 {
		t.Fatal("skin accessor leaked mutable buffer")
	}
	pixels[0] = 42
	pm.HandlePacket(&packet.SyncSkin{Skin: skin, PlayerData: []protocol.NeteasePlayerData{{UUID: u}}}, nil)
	b, _, _, _ = pm.GetPlayerSkin("p")
	if b[0] != 42 {
		t.Fatal("unique SyncSkin not cached")
	}
	pixels[0] = 8
	pm.HandlePacket(&packet.SyncSkin{Skin: skin, PlayerData: []protocol.NeteasePlayerData{{UUID: u}, {UUID: uuid.New()}}}, nil)
	b, _, _, _ = pm.GetPlayerSkin("p")
	if b[0] != 42 {
		t.Fatal("ambiguous SyncSkin invented an avatar")
	}
}
