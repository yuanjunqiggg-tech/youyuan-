package main

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestCaptureBehaviorOnlyChangesExplicitSettings(t *testing.T) {
	if got := yyCaptureBehaviorCommands("Bot", yyObserverDimensionOptions{}); len(got) != 0 {
		t.Fatal(got)
	}
	off := false
	o := yyObserverDimensionOptions{GameMode: "spectator", Invisible: &off}
	want := []string{`gamemode spectator "Bot"`, `effect "Bot" invisibility 0`}
	if got := yyCaptureBehaviorCommands("Bot", o); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if got := yyCaptureBehaviorCommands(`Bot" @a`, o); len(got) != 0 {
		t.Fatal("untrusted selector escaped its target", got)
	}
	if yyCaptureGameModeValid("creative @a") {
		t.Fatal("unvalidated game mode")
	}
}

func TestCaptureBehaviorRefreshIsBoundedAndFailedCommandsRetry(t *testing.T) {
	on := true
	bm := &BotManager{display: "Bot"}
	o := yyObserverDimensionOptions{GameMode: "adventure", Invisible: &on}
	writes := 0
	send := func(string) error { writes++; return nil }
	for range 50 {
		if err := yyCaptureApplyBehavior(bm, o, send); err != nil {
			t.Fatal(err)
		}
	}
	if writes != 2 {
		t.Fatal("appearance commands flooded the connection", writes)
	}
	bm.captureBehaviorAt = time.Now().Add(-2 * time.Minute)
	if err := yyCaptureApplyBehavior(bm, o, func(string) error { return errors.New("rejected") }); err == nil {
		t.Fatal("failure hidden")
	}
	if err := yyCaptureApplyBehavior(bm, o, send); err != nil || writes != 4 {
		t.Fatal(err, writes)
	}
}

func TestCaptureQuickTPOffDisablesBothMovementPaths(t *testing.T) {
	off := false
	o := yyGlobalWatchOptions(yyGlobalCaptureConfig{Mode: "hybrid", QuickTP: &off})
	if o["mode"] != "camera" || o["body_radius_blocks"] != float64(0) {
		t.Fatal(o)
	}
	// 老配置没有 body_radius_blocks 时走默认值，2026-10-26 起默认是 48 格。
	legacy := yyGlobalWatchOptions(yyGlobalCaptureConfig{Mode: "hybrid"})
	if legacy["mode"] != "hybrid" || legacy["body_radius_blocks"] != float64(48) {
		t.Fatal("old settings 默认半径应为 48", legacy)
	}
}
