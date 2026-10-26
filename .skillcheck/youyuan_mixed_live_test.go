//go:build !android

package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Run explicitly with YY_COMM_FIXTURE_DIR pointing to compiled Linux fixtures.
// No game connection is mocked; this test verifies actual language runtimes only.
func TestLinuxMixedResidentCommunication(t *testing.T) {
	fixtures := os.Getenv("YY_COMM_FIXTURE_DIR")
	if runtime.GOOS != "linux" || fixtures == "" {
		t.Skip("requires explicit Linux C++/Python fixtures")
	}
	oldMux, oldPort, oldRoot := yyMux, yyCorePort, yyRuntimeRoot
	defer func() { yyMux, yyCorePort, yyRuntimeRoot = oldMux, oldPort, oldRoot }()
	yyMux = http.NewServeMux()
	yyRuntimeRoutes(yyMux, t.TempDir())
	server := httptest.NewServer(yyMux)
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	yyCorePort = port
	request := func(path string, body any) map[string]any {
		t.Helper()
		_, raw, e := yyInvoke("POST", path, nil, body)
		if e != nil {
			t.Fatal(e)
		}
		var answer map[string]any
		if e = json.Unmarshal(raw, &answer); e != nil {
			t.Fatal(string(raw), e)
		}
		return answer
	}
	for _, spec := range []struct{ id, language, entry string }{
		{"check_python", "python", "main.py"}, {"check_cpp", "native_cpp", "plugin.so"},
	} {
		dir := filepath.Join(yyRuntimeRoot, spec.id)
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		data, e := os.ReadFile(filepath.Join(fixtures, spec.entry))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, spec.entry), data, 0644); e != nil {
			t.Fatal(e)
		}
		manifest, _ := json.Marshal(yyRuntimeManifest{ID: spec.id, Language: spec.language, Entry: spec.entry, Version: "1.0.0"})
		if e = os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0644); e != nil {
			t.Fatal(e)
		}
		if answer := request("/api/youyuan/runtime/start", map[string]any{"id": spec.id}); answer["ok"] != true {
			t.Fatal(answer)
		}
		id := spec.id
		defer request("/api/youyuan/runtime/stop", map[string]any{"id": id})
	}
	bm := yyAccountBot("main")
	if bm != nil {
		t.Fatal("acceptance must not use a live game account")
	}
	lua := `local n=0; plugin_api={bump=function(v)n=n+v;return n end,value=function()return n end,
	py=function(v)return api.require('check_python').bump(v)end,
	cpp=function(v)return api.require('check_cpp').bump(v)end}`
	if e := LoadLuaPlugin(bm, "check_lua", lua); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(bm, "check_lua")
	call := func(id, fn string, args []any) any {
		t.Helper()
		data, _ := json.Marshal(args)
		text, e := callPluginFunction(bm, id, fn, string(data))
		if e != nil {
			t.Fatal(id, fn, e)
		}
		var result any
		if e = json.Unmarshal([]byte(text), &result); e != nil {
			t.Fatal(text, e)
		}
		return result
	}
	for _, tc := range []struct {
		label, id, fn string
		n, want       float64
	}{
		{"Python resident first", "check_python", "bump", 2, 2},
		{"Python resident second", "check_python", "bump", 3, 5},
		{"C++ resident first", "check_cpp", "bump", 4, 4},
		{"C++ resident second", "check_cpp", "bump", 5, 9},
		{"Python -> C++", "check_python", "cpp", 6, 15},
		{"C++ -> Python", "check_cpp", "py", 7, 12},
		{"Python -> Lua", "check_python", "lua", 8, 8},
		{"C++ -> Lua", "check_cpp", "lua", 9, 17},
		{"Lua -> Python", "check_lua", "py", 10, 22},
		{"Lua -> C++", "check_lua", "cpp", 11, 26},
	} {
		t.Run(tc.label, func(t *testing.T) {
			if got := call(tc.id, tc.fn, []any{tc.n}); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	for id, want := range map[string]float64{"check_python": 22, "check_cpp": 26, "check_lua": 17} {
		if got := call(id, "value", nil); got != want {
			t.Fatalf("%s state lost: %v", id, got)
		}
	}
}

// Explicit protocol fixtures, real resident Lua/Python/ELF runtimes. This does
// not claim that an optional provider is installed on a production game server.
func TestLinuxServerObservationRuntime(t *testing.T) {
	fixtures := os.Getenv("YY_OBSERVER_FIXTURE_DIR")
	if runtime.GOOS != "linux" || fixtures == "" {
		t.Skip("requires explicit observer runtime fixtures")
	}
	oldMux, oldPort, oldRoot := yyMux, yyCorePort, yyRuntimeRoot
	defer func() { yyMux, yyCorePort, yyRuntimeRoot = oldMux, oldPort, oldRoot }()
	yyMux = http.NewServeMux()
	yyRuntimeRoutes(yyMux, t.TempDir())
	server := httptest.NewServer(yyMux)
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	yyCorePort = port
	request := func(path string, body any) map[string]any {
		t.Helper()
		_, raw, e := yyInvoke("POST", path, nil, body)
		if e != nil {
			t.Fatal(e)
		}
		var answer map[string]any
		if e = json.Unmarshal(raw, &answer); e != nil {
			t.Fatal(e)
		}
		return answer
	}
	for _, spec := range []struct{ id, language, entry string }{{"observe_python", "python", "observer.py"}, {"observe_cpp", "native_cpp", "observer.so"}} {
		dir := filepath.Join(yyRuntimeRoot, spec.id)
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		data, e := os.ReadFile(filepath.Join(fixtures, spec.entry))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, spec.entry), data, 0644); e != nil {
			t.Fatal(e)
		}
		manifest, _ := json.Marshal(yyRuntimeManifest{ID: spec.id, Language: spec.language, Entry: spec.entry, Version: "0.10.29"})
		os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0644)
		if answer := request("/api/youyuan/runtime/start", map[string]any{"id": spec.id}); answer["ok"] != true {
			t.Fatal(answer)
		}
		id := spec.id
		defer request("/api/youyuan/runtime/stop", map[string]any{"id": id})
	}
	if bm := yyAccountBot("main"); bm != nil {
		t.Fatal("test must not bind production game")
	}
	if e := LoadLuaPlugin(nil, "observe_lua", `local n=0;local last={}; events.onGlobalSwing(function(e)n=n+1;last=e end);events.onGlobalHealthChange(function(e)n=n+1;last=e end);plugin_api={metrics=function()return {count=n,last=last}end}`); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(nil, "observe_lua")
	defer yyObserverRPCReset(nil)
	serverObservationFixture(nil, "swing", map[string]any{"player_id": "9007199254740999"})
	serverObservationFixture(nil, "health_changed", map[string]any{"entity_id": "9007199254740999", "health": map[string]any{"current": 14.0, "max": 40.0}})
	for _, id := range []string{"observe_lua", "observe_python", "observe_cpp"} {
		t.Run(id, func(t *testing.T) {
			deadline := time.Now().Add(8 * time.Second)
			var result map[string]any
			for time.Now().Before(deadline) {
				text, e := callPluginFunction(nil, id, "metrics", "[]")
				if e != nil {
					// Resident Lua rejects synchronous RPC while its queued callback
					// owns the state. Wait for that callback, rather than interpreting
					// this transient readback response as a missing event.
					if strings.Contains(e.Error(), "正在执行") {
						time.Sleep(10 * time.Millisecond)
						continue
					}
					t.Fatal(e)
				}
				if e = json.Unmarshal([]byte(text), &result); e != nil {
					t.Fatal(e)
				}
				if result["count"] == float64(2) {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			last, ok := result["last"].(map[string]any)
			if !ok {
				t.Fatal(result)
			}
			health, ok := last["health"].(map[string]any)
			if result["count"] != float64(2) || last["entity_id"] != "9007199254740999" || !ok || health["current"] != 14.0 || health["max"] != 40.0 || last["server_engine_verified"] != false {
				t.Fatal(result)
			}
		})
	}
}
