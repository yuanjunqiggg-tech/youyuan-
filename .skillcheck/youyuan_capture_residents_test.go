package main

import (
	"strings"
	"testing"
)

func TestCaptureKeepsThreeIdleDimensionResidents(t *testing.T) {
	c := captureConfig()
	for _, players := range [][]yyCapturePlayer{nil, captureFixture(1, 0), captureFixture(8, 2)} {
		plan := yyCapturePlan(c, players, c.Accounts)
		dims := map[int32]bool{}
		for _, a := range plan {
			dims[a.Dimension] = true
			for _, p := range a.Players {
				if len(players) == 0 {
					t.Fatal("idle residents sent camera targets", p)
				}
			}
		}
		if len(plan) != 3 || len(dims) != 3 {
			t.Fatal("missing dimension residency", plan)
		}
	}
	c.FleetSteps = []yyCaptureFleetStep{{1, 3}, {8, 6}}
	plan := yyCapturePlan(c, captureFixture(8, 0), c.Accounts)
	counts := map[int32]int{}
	for _, a := range plan {
		counts[a.Dimension]++
	}
	if counts[0] != 4 || counts[1] != 1 || counts[2] != 1 {
		t.Fatal("extra workers must share load without taking residents", counts)
	}
}

func TestCaptureOverheadCameraDoesNotTiltWithTarget(t *testing.T) {
	pitch := 90.0
	cmd := yySelectorCameraCommand("observer", "human", yyWatchConfig{CameraHeight: 6, CameraPitch: &pitch})
	if !strings.Contains(cmd, "pos ~ ~6.000 ~ rot 90.000 0.000") || strings.Contains(cmd, "rotated as") || strings.Contains(cmd, "pos ^") {
		t.Fatal(cmd)
	}
}

func TestOrdinaryCaptureDefaultsCannotBeOverridden(t *testing.T) {
	no := false
	input := yyGlobalCaptureConfig{Mode: "follow", Interval: 1, Height: 99, Pitch: 0, Yaw: 42, PauseJobs: false, AutoFleet: &no, AutoPlayers: false, Players: []string{"manual"}, ResidentObservers: 1}
	got := yyGlobalCaptureForActor(input, yyPoolActor{ID: "member"})
	if got.Mode != "camera" || got.Interval != .015 || got.Height != 6 || got.Pitch != 90 || got.Yaw != 0 || !got.PauseJobs || !got.AutoPlayers || len(got.Players) != 0 || got.ResidentObservers != 3 || !*got.AutoFleet {
		t.Fatal(got)
	}
	admin := yyGlobalCaptureForActor(input, yyPoolActor{Admin: true})
	if admin.Mode != "follow" || admin.Height != 99 {
		t.Fatal("admin settings lost", admin)
	}
}
func TestTaskPanelNeverInventsTPS(t *testing.T) {
	unknown := yyTaskActionbarMetrics("建筑写入", "file", "b", 20, 100, 10, 2, 0, false)
	if !strings.Contains(unknown, "TPS 暂不可用") || strings.Contains(unknown, "20.0") {
		t.Fatal(unknown)
	}
	real := yyTaskActionbarMetrics("建筑读取", "file", "d", 20, 100, 10, 2, 18.7, true)
	if !strings.Contains(real, "TPS §f18.7") || strings.Contains(real, "%") {
		t.Fatal(real)
	}
}
