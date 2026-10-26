package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSDKRelativeFilesUseActualRuntimeDirectories(t *testing.T) {
	s := sdkFixture(t, "sdk_files")
	if e := os.MkdirAll(filepath.Join(s.Code, "resources"), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(s.Code, "resources", "guide.txt"), []byte("guide"), 0644); e != nil {
		t.Fatal(e)
	}
	if v, e := s.call("util.read_file", []any{"guide.txt"}); e != nil || v != "guide" {
		t.Fatal(v, e)
	}
	if _, e := s.call("util.write_file", []any{"nested/state.txt", "saved"}); e != nil {
		t.Fatal(e)
	}
	if v, e := os.ReadFile(filepath.Join(s.Dir, "nested/state.txt")); e != nil || string(v) != "saved" {
		t.Fatal(string(v), e)
	}
}

func sdkFixture(t *testing.T, id string) *yySDKContext {
	t.Helper()
	i := &yyRuntimeInstance{Key: "main:" + id, Account: "main", Manifest: yyRuntimeManifest{ID: id}, Queue: make(chan map[string]any, 256), Done: make(chan struct{})}
	yySDKPrepare(i, t.TempDir())
	t.Cleanup(func() { yySDKClose(i.Key) })
	return yySDKGet(i.Key)
}
func TestSDKBindingPersistenceAndErrors(t *testing.T) {
	s := sdkFixture(t, "sdk_binding")
	v, e := s.call("util.split", []any{"a,b", ","})
	if e != nil || len(v.([]any)) != 2 {
		t.Fatalf("split: %v %v", v, e)
	}
	state := s.L
	v, e = s.call("util.floor", []any{3.9})
	if e != nil || v != float64(3) || s.L != state {
		t.Fatalf("persistent binding: %v %v", v, e)
	}
	if _, e = s.call("game.sendCommand", []any{"say SDK"}); e == nil || !strings.Contains(e.Error(), "未连接") {
		t.Fatal("offline control must fail", e)
	}
	if _, e = s.call("util.noSuchMethod", nil); e == nil {
		t.Fatal("unknown API silently accepted")
	}
	if _, e = s.call("util.floor", []any{"invalid"}); e == nil {
		t.Fatal("invalid argument silently accepted")
	}
}
func TestSDKPersistentStateAndConfig(t *testing.T) {
	s := sdkFixture(t, "sdk_config")
	if _, e := s.call("plugin.save", []any{map[string]any{"counter": float64(7)}}); e != nil {
		t.Fatal(e)
	}
	v, e := s.call("plugin.state", nil)
	if e != nil || v.(map[string]any)["counter"] != float64(7) {
		t.Fatal(v, e)
	}
	for _, value := range []string{"first", "second"} {
		if _, e = s.call("game.setConfigDefault", []any{map[string]any{"value": value}}); e != nil {
			t.Fatal(e)
		}
	}
	v, e = s.call("game.getConfig", []any{"value", "fallback"})
	if e != nil || v != "first" {
		t.Fatal(v, e)
	}
}
func TestSDKTriggerBoundaryAndAccountRouting(t *testing.T) {
	s := sdkFixture(t, "sdk_trigger")
	if _, e := s.call("game.registerTrigger", []any{[]any{".test"}, "handle"}); e != nil {
		t.Fatal(e)
	}
	yySDKEmit(nil, map[string]any{"type": "chat", "player": "Alice", "message": ".testevil"})
	if len(s.Instance.Queue) != 0 {
		t.Fatal("substring incorrectly triggered")
	}
	yySDKEmit(nil, map[string]any{"type": "chat", "player": "Alice", "message": ".test a b"})
	select {
	case event := <-s.Instance.Queue:
		if event["handler"] != "handle" {
			t.Fatal(event)
		}
		args := event["args"].([]any)
		if args[0] != "Alice" || len(args[1].([]any)) != 2 {
			t.Fatal(args)
		}
	default:
		t.Fatal("trigger not delivered")
	}
	if _, e := s.call("events.onJoin", []any{"join"}); e != nil {
		t.Fatal(e)
	}
	if !yySDKNeedsPlayerPolling() {
		t.Fatal("registered native event did not enable polling")
	}
}
func TestSDKTimersCancelAndStop(t *testing.T) {
	s := sdkFixture(t, "sdk_timer")
	id, e := s.call("util.setTimeout", []any{float64(.2), "cancelled"})
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.call("util.cancel", []any{float64(id.(int64))})
	if e != nil || v != true {
		t.Fatal(v, e)
	}
	_, e = s.call("util.setTimeout", []any{float64(.01), "once"})
	if e != nil {
		t.Fatal(e)
	}
	select {
	case v := <-s.Instance.Queue:
		if v["handler"] != "once" {
			t.Fatal(v)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout missing")
	}
	yySDKClose(s.Instance.Key)
	time.Sleep(220 * time.Millisecond)
	if len(s.Instance.Queue) != 0 {
		t.Fatal("callback after cancel/stop")
	}
	if _, e = s.call("util.floor", []any{float64(1)}); e == nil {
		t.Fatal("closed SDK still callable")
	}
}
func TestSDKCrossPluginCycleRejected(t *testing.T) {
	s := sdkFixture(t, "sdk_cycle_b")
	yySDKCalls.Lock()
	yySDKCalls.Edges["main:sdk_cycle_a"] = s.Instance.Key
	yySDKCalls.Unlock()
	defer func() { yySDKCalls.Lock(); delete(yySDKCalls.Edges, "main:sdk_cycle_a"); yySDKCalls.Unlock() }()
	if _, e := s.crossCall([]any{"sdk_cycle_a", "callback"}); e == nil {
		t.Fatal("cycle not rejected")
	}
}

func TestSDKDependenciesRejectMissingAndWrongVersion(t *testing.T) {
	dep := &yyRuntimeInstance{Key: "sdk_dependency_account:dependency", Manifest: yyRuntimeManifest{ID: "dependency", Version: "1.2.0"}}
	yyRuntimeMu.Lock()
	yyRuntimeInstances[dep.Key] = dep
	yyRuntimeMu.Unlock()
	defer func() { yyRuntimeMu.Lock(); delete(yyRuntimeInstances, dep.Key); yyRuntimeMu.Unlock() }()
	manifest := yyRuntimeManifest{ID: "consumer", Dependencies: map[string]string{"dependency": "1.1.0"}}
	if e := yyRuntimeCheckDependencies(manifest, "sdk_dependency_account"); e != nil {
		t.Fatal(e)
	}
	manifest.Dependencies["dependency"] = "2.0.0"
	if e := yyRuntimeCheckDependencies(manifest, "sdk_dependency_account"); e == nil {
		t.Fatal("wrong version accepted")
	}
	if e := yyRuntimeCheckDependencies(manifest, "other_account"); e == nil {
		t.Fatal("dependency leaked across accounts")
	}
}
