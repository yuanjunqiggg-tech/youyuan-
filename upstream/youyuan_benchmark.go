package main

import (
	"encoding/json"
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

var yyBenchmarkMu sync.Mutex

const yyBenchLua = "counter=0; function step() counter=counter+1; return counter end"

func yyBenchmarkMedian(rounds int, fn func() error) (float64, error) {
	values := []float64{}
	for trial := 0; trial < 3; trial++ {
		start := time.Now()
		for n := 0; n < rounds; n++ {
			if e := fn(); e != nil {
				return 0, e
			}
		}
		values = append(values, float64(time.Since(start).Nanoseconds())/float64(rounds))
	}
	sort.Float64s(values)
	return values[1], nil
}
func yyRunBenchmark(rounds int) map[string]any {
	yyBenchmarkMu.Lock()
	defer yyBenchmarkMu.Unlock()
	result := map[string]any{"ok": true, "rounds_per_trial": rounds, "trials": 3, "platform": getPlatformName(), "arch": runtime.GOARCH, "baseline": "复现源码中每次新建 Lua VM、重新加载源码并执行回调的策略；不是原 APK 整机实测", "workload": "常驻计数回调，不访问游戏、网络或文件；包含各语言正常宿主桥接与事件 JSON 成本", "nanoseconds_per_event": map[string]any{}, "speedup_vs_reset_lua": map[string]any{}, "errors": map[string]string{}}
	values := result["nanoseconds_per_event"].(map[string]any)
	ratios := result["speedup_vs_reset_lua"].(map[string]any)
	errors := result["errors"].(map[string]string)
	baseline, e := yyBenchmarkMedian(rounds, func() error {
		L := newYYLuaVM()
		defer L.Close()
		if e := L.DoString(yyBenchLua); e != nil {
			return e
		}
		return L.CallByParam(lua.P{Fn: L.GetGlobal("step"), NRet: 0, Protect: true})
	})
	if e != nil {
		result["ok"] = false
		errors["baseline"] = e.Error()
		return result
	}
	values["reset_lua"] = baseline
	L := newYYLuaVM()
	defer L.Close()
	_ = L.DoString(yyBenchLua)
	step := L.GetGlobal("step")
	persistent, e := yyBenchmarkMedian(rounds, func() error { return L.CallByParam(lua.P{Fn: step, NRet: 0, Protect: true}) })
	if e == nil {
		values["persistent_lua"] = persistent
		ratios["persistent_lua"] = baseline / persistent
	}
	key := fmt.Sprintf("benchmark_%d", time.Now().UnixNano())
	directory := filepath.Join(yyAgentRoot, ".benchmark")
	_ = os.MkdirAll(directory, 0700)
	entry := filepath.Join(directory, "main.py")
	_ = os.WriteFile(entry, []byte("counter=0\ndef on_event(event,host):\n global counter\n counter+=1\n return {'ok':True,'counter':counter}\n"), 0600)
	start := time.Now()
	text, e := yyPythonRuntime("start", map[string]any{"key": key, "account": "benchmark", "entry": entry, "data_dir": directory})
	var response map[string]any
	_ = json.Unmarshal([]byte(text), &response)
	if e == nil && response["ok"] == false {
		e = fmt.Errorf("%v", response["error"])
	}
	result["python_startup_ms"] = float64(time.Since(start).Microseconds()) / 1000
	if e == nil {
		defer yyPythonRuntime("stop", map[string]any{"key": key})
		python, e := yyBenchmarkMedian(rounds, func() error {
			reply, e := yyPythonRuntime("event", map[string]any{"key": key, "event": map[string]any{"type": "manual"}})
			if e != nil {
				return e
			}
			var state map[string]any
			if e = json.Unmarshal([]byte(reply), &state); e != nil {
				return e
			}
			if state["ok"] == false {
				return fmt.Errorf("%v", state["error"])
			}
			return nil
		})
		if e == nil {
			values["python"] = python
			ratios["python"] = baseline / python
		} else {
			errors["python"] = e.Error()
		}
		events := make([]map[string]any, 16)
		for i := range events {
			events[i] = map[string]any{"type": "manual"}
		}
		batched, e := yyBenchmarkMedian((rounds+15)/16, func() error {
			reply, e := yyPythonRuntime("events", map[string]any{"key": key, "events": events})
			if e != nil {
				return e
			}
			var response map[string]any
			if e = json.Unmarshal([]byte(reply), &response); e != nil {
				return e
			}
			if response["ok"] != true {
				return fmt.Errorf("Python 批处理失败 %s", reply)
			}
			return nil
		})
		if e == nil {
			values["python_batch_16"] = batched / 16
			ratios["python_batch_16"] = baseline / (batched / 16)
		} else {
			errors["python_batch_16"] = e.Error()
		}
	} else {
		errors["python"] = e.Error()
	}
	handle, e := yyNativeStart("libyy_counter.so", "benchmark", "native_counter")
	if e == nil {
		defer yyNativeStop(handle)
		native, e := yyBenchmarkMedian(rounds, func() error { _, e := yyNativeEvent(handle, `{"type":"manual"}`); return e })
		if e == nil {
			values["native_cpp"] = native
			ratios["native_cpp"] = baseline / native
		} else {
			errors["native_cpp"] = e.Error()
		}
	} else {
		errors["native_cpp"] = e.Error()
	}
	return result
}
func yyBenchmarkRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/runtime/benchmark", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeJSON(w, map[string]any{"ok": false, "error": "使用 POST 明确启动计数基准"})
			return
		}
		var p struct {
			Rounds int `json:"rounds"`
		}
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p.Rounds < 100 {
			p.Rounds = 500
		}
		if p.Rounds > 5000 {
			p.Rounds = 5000
		}
		writeJSON(w, yyRunBenchmark(p.Rounds))
	})
}
