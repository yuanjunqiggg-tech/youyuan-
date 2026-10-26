package main

import (
	"bot-apk/config"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestCameraExclusionsNeverProbeChildHTTP(t *testing.T) {
	yyPrimaryFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:19532")
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int64
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write([]byte(`{"connected":true}`)) })}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	old := yyAccounts.Items
	yyAccounts.Items = []yyManagedAccount{{ID: "capture_test", Name: "known_bot", Slot: 32}}
	t.Cleanup(func() { yyAccounts.Items = old })
	for i := 0; i < 1000; i++ {
		if !containsString(yyCaptureRobotNames(), "known_bot") {
			t.Fatal("saved robot was not excluded")
		}
	}
	if requests.Load() != 0 {
		t.Fatal("camera tick made child HTTP requests", requests.Load())
	}
}

func TestCaptureOnlyKernelRefusesPluginRun(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "capture-only"), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	pc := &PluginController{privateDir: root}
	if err := pc.Run(nil, "irrelevant"); err == nil || !containsString([]string{err.Error()}, "采集专用内核不启动业务插件") {
		t.Fatal("dedicated capture kernel reached plugin loading", err)
	}
}

func TestCaptureOnlyKernelRefusesOtherLanguageRuntime(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "capture-only"), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	oldRoot := yyRuntimeRoot
	t.Cleanup(func() { yyRuntimeRoot = oldRoot })
	mux := http.NewServeMux()
	yyRuntimeRoutes(mux, root)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest("POST", "/api/youyuan/runtime/start", strings.NewReader(`{"id":"missing"}`)))
	if !strings.Contains(recorder.Body.String(), "采集专用内核不启动业务插件") {
		t.Fatal(recorder.Body.String())
	}
}

func TestObserverStatusReportsBusinessPluginOccupancy(t *testing.T) {
	old := pluginController
	pluginController = &PluginController{running: map[string]bool{"business": true}}
	t.Cleanup(func() { pluginController = old })
	if yyObserverDimensionStatus(&BotManager{})["business_plugins_running"] != true {
		t.Fatal("active plugin was not reported")
	}
}

func BenchmarkCaptureFullDiagnosticQueue(b *testing.B) {
	bm := &BotManager{}
	s := yySenseFor(bm)
	s.Capture = yyCaptureState{Active: true, Config: yyCaptureConfig{Types: []string{"SetHealth"}}, Last: map[string]time.Time{}, pending: make(chan packet.Packet, 1)}
	pk := &packet.SetHealth{Health: 20}
	s.Capture.pending <- pk
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		yyCapturePacket(bm, pk)
	}
}

func TestCaptureProvisionWaitsForPreviousLogin(t *testing.T) {
	yyCaptureLaunches.Lock()
	old := yyCaptureLaunches.Items
	yyCaptureLaunches.Items = map[string]*yyCaptureLaunch{"joining": {Started: true, At: time.Now()}}
	yyCaptureLaunches.Unlock()
	t.Cleanup(func() { yyCaptureLaunches.Lock(); yyCaptureLaunches.Items = old; yyCaptureLaunches.Unlock() })
	v := yyCaptureProvision(yyPoolActor{}, "next", yyGlobalCaptureConfig{})
	if v["phase"] != "waiting_previous_join" || v["pending_account"] != "joining" {
		t.Fatal(v)
	}
}

func TestCaptureProvisionCoolsDownExpiredLogin(t *testing.T) {
	yyPrimaryFixture(t)
	yyAccounts.Items = []yyManagedAccount{{ID: "joining", OwnerID: "alice", CaptureOnly: true, Enabled: true}}
	stops := 0
	yyMux = http.NewServeMux()
	yyMux.HandleFunc("/api/youyuan/accounts", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["action"] != "disable" || body["id"] != "joining" {
			t.Error(body)
		}
		stops++
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	yyCaptureLaunches.Lock()
	old := yyCaptureLaunches.Items
	yyCaptureLaunches.Items = map[string]*yyCaptureLaunch{"joining": {Started: true, At: time.Now().Add(-61 * time.Second), Attempts: 1}}
	yyCaptureLaunches.Unlock()
	t.Cleanup(func() { yyCaptureLaunches.Lock(); yyCaptureLaunches.Items = old; yyCaptureLaunches.Unlock() })
	v := yyCaptureProvision(yyPoolActor{}, "joining", yyGlobalCaptureConfig{})
	if v["phase"] != "expired_login_cleanup" || stops != 1 || yyCaptureLaunches.Items["joining"].Started {
		t.Fatal(v, stops)
	}
	v = yyCaptureProvision(yyPoolActor{}, "joining", yyGlobalCaptureConfig{})
	if v["phase"] != "starting_or_cooldown" {
		t.Fatal(v)
	}
}

func TestCaptureWorkerReconnectBelongsToScheduler(t *testing.T) {
	yyPrimaryFixture(t)
	for _, capture := range []bool{true, false} {
		a := yyManagedAccount{ID: "retry_test", OwnerID: "alice", CaptureOnly: capture, KeepAlive: true}
		if err := yyWriteChildConfig(a, true); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(yyAccountDir(a.ID), "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cfg config.AppConfig
		if err = json.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.AutoReconnect == capture || !cfg.AutoConnect {
			t.Fatal("worker retry ownership incorrect", capture, cfg.AutoReconnect)
		}
	}
}

func TestCaptureFailedCleanupKeepsAdmissionGate(t *testing.T) {
	yyPrimaryFixture(t)
	yyAccounts.Items = []yyManagedAccount{{ID: "joining", OwnerID: "alice", CaptureOnly: true, Enabled: true}}
	yyMux = http.NewServeMux()
	yyMux.HandleFunc("/api/youyuan/accounts", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "fixture stop failed"})
	})
	yyCaptureLaunches.Lock()
	old := yyCaptureLaunches.Items
	yyCaptureLaunches.Items = map[string]*yyCaptureLaunch{"joining": {Started: true, At: time.Now().Add(-61 * time.Second)}}
	yyCaptureLaunches.Unlock()
	t.Cleanup(func() { yyCaptureLaunches.Lock(); yyCaptureLaunches.Items = old; yyCaptureLaunches.Unlock() })
	v := yyCaptureProvision(yyPoolActor{}, "next", yyGlobalCaptureConfig{})
	if v["phase"] != "expired_login_cleanup" || !yyCaptureLaunches.Items["joining"].Started {
		t.Fatal("failed cancellation admitted another worker", v)
	}
	if v = yyCaptureProvision(yyPoolActor{}, "next", yyGlobalCaptureConfig{}); v["phase"] != "waiting_previous_join" {
		t.Fatal(v)
	}
}
