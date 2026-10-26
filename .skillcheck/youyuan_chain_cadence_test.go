package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft"
)

func TestExplicitChainCadenceCapsModulesWithoutSpeedingSlowSegments(t *testing.T) {
	for _, tc := range []struct {
		source, cap float64
		want        time.Duration
	}{
		{20, 1, time.Second},
		{1, 1, time.Second},
		{1.0 / 60, 1, time.Minute},
		{0, 1, time.Second},
		{20, 0, 50 * time.Millisecond},
	} {
		if got := yyChainPeriod(yyChainPlan{Hz: tc.source}, yyChainConfig{Hz: tc.cap, IntervalMS: 1000}); got != tc.want {
			t.Fatalf("source=%g cap=%g got=%v want=%v", tc.source, tc.cap, got, tc.want)
		}
	}
}

func TestLegacyFrequencyEditReplacesStaleHzAndPreservesStoppedState(t *testing.T) {
	oldMux, oldRoot := yyMux, yyChainRoot
	yyChains.Lock()
	old := yyChains.items
	yyChains.items = map[string]*yyChainInstance{}
	yyChains.Unlock()
	defer func() { yyMux, yyChainRoot = oldMux, oldRoot; yyChains.Lock(); yyChains.items = old; yyChains.Unlock() }()
	yyMux = http.NewServeMux()
	yyCommandRoutes(yyMux, t.TempDir())
	bm := getBotManagerLocked()
	text := "2,0,0 20\nlist\n.\n2,0,0 0.016666666666666666\nlist\n!"
	_, err := yyChainPluginCall(bm, "chains.save", []any{"cadence.txt", text, map[string]any{"hz": float64(20), "interval_ms": float64(50)}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = yyChainPluginCall(bm, "chains.configure", []any{"cadence.txt", map[string]any{"interval_ms": float64(1000)}})
	if err != nil {
		t.Fatal(err)
	}
	in := yyChains.items["cadence.txt"]
	if in.Config.Hz != 1 || in.Config.IntervalMS != 1000 || in.Config.Enabled || in.Config.RunCount != 0 {
		t.Fatalf("incoherent edit: %+v", in.Config)
	}
	if len(in.Plan.Chains) != 2 || yyChainPeriod(yyChainPlan{Hz: in.Plan.Chains[0].Hz}, in.Config) != time.Second || yyChainPeriod(yyChainPlan{Hz: in.Plan.Chains[1].Hz}, in.Config) != time.Minute {
		t.Fatal("TXT modules ignored cap or slower module was accelerated")
	}
	_, err = yyChainPluginCall(bm, "chains.configure", []any{"cadence.txt", map[string]any{"hz": float64(2)}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg := yyChains.items["cadence.txt"].Config; cfg.Hz != 2 || cfg.IntervalMS != 500 || cfg.Enabled {
		t.Fatal(cfg)
	}
	_, err = yyChainPluginCall(bm, "chains.configure", []any{"cadence.txt", map[string]any{"interval_ms": float64(0)}})
	if err == nil {
		t.Fatal("zero interval accepted")
	}
}

func TestImportPreloadCallbacksRespectConnectionAndSetting(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		preload, release, cleanup := yyTickingCallbacks(&BotManager{conn: &minecraft.Conn{}, connected: true}, "qa_import_", enabled)
		if (preload != nil) != enabled || (release != nil) != enabled || (cleanup != nil) != enabled {
			t.Fatal("preload setting ignored")
		}
	}
}
