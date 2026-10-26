package main

import (
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
	"math"
	"testing"
)

func TestCaptureLandingRequiresRealDimensionAndFeetPosition(t *testing.T) {
	p := yyCaptureLanding{X: -10000, Y: 100, Z: -10000}
	row := game_interface.TargetQueryingInfo{Dimension: 1, Position: game_interface.TargetQueryingPos{X: -9999.5, Y: 101.62001, Z: -9999.5}}
	if !yyCaptureLandingMatches(p, 1, row) {
		t.Fatal("native querytarget eye height rejected")
	}
	row.Dimension = 0
	if yyCaptureLandingMatches(p, 1, row) {
		t.Fatal("camera/other dimension accepted")
	}
	row.Dimension = 1
	row.Position.X = 20
	if yyCaptureLandingMatches(p, 1, row) {
		t.Fatal("old unsafe landing accepted")
	}
}

func TestCaptureLandingValidationAndNoImplicitLanding(t *testing.T) {
	for _, p := range []yyCaptureLanding{{X: math.NaN()}, {Y: math.Inf(1)}, {X: 30000001}, {Y: -65}, {Y: 321}} {
		if yyCaptureLandingsValidate(map[string]yyCaptureLanding{"1": p}) == nil {
			t.Fatal(p)
		}
	}
	if yyCaptureLandingsValidate(map[string]yyCaptureLanding{"3": {}}) == nil {
		t.Fatal("invalid dimension")
	}
	if yyCaptureLandingsValidate(nil) != nil {
		t.Fatal("old config should remain valid")
	}
	if e := yyCaptureArmLanding(yyManagedAccount{}); e != nil {
		t.Fatal("business bot affected", e)
	}
}

func TestCaptureLandingCancelIsIdempotent(t *testing.T) {
	id := "landing-cancel-test"
	cancel := make(chan struct{})
	yyCaptureLandings.Lock()
	yyCaptureLandings.cancel[id] = cancel
	yyCaptureLandings.Unlock()
	yyCaptureCancelLanding(id)
	yyCaptureCancelLanding(id)
	select {
	case <-cancel:
	default:
		t.Fatal("stop did not cancel old rescue before slot reuse")
	}
}
