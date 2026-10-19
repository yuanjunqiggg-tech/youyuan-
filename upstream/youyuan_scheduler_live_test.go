package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Opt-in wall-clock measurement: real resident Lua and the real host scheduler.
// There is no game connection and loop iterations are not game commands.
func TestLinuxSchedulerLiveMeasurement(t *testing.T) {
	output := os.Getenv("YY_SCHEDULER_REPORT")
	if output == "" {
		t.Skip("set YY_SCHEDULER_REPORT for live measurement")
	}
	var rows []map[string]any
	for _, cfg := range []struct{ us, iterations int }{{1000, 0}, {5000, 0}, {50000, 0}, {500000, 0}, {1000, 10000}, {500, 0}, {100, 0}} {
		id := fmt.Sprintf("scheduler-measure-%d-%d", cfg.us, cfg.iterations)
		bm := &BotManager{}
		src := fmt.Sprintf("rounds=0; work=0; function measured_tick() rounds=rounds+1; for i=1,%d do work=work+1 end; probe() end; function state() return {rounds=rounds,work=work} end", cfg.iterations)
		if err := LoadLuaPlugin(bm, id, src); err != nil {
			t.Fatal(err)
		}
		yyLuaMu.Lock()
		resident := yyLuaStates[yyLuaKey{bm, id}]
		yyLuaMu.Unlock()
		var mu sync.Mutex
		var times []time.Time
		resident.mu.Lock()
		resident.L.SetGlobal("probe", resident.L.NewFunction(func(L *lua.LState) int { mu.Lock(); times = append(times, time.Now()); mu.Unlock(); return 0 }))
		resident.mu.Unlock()
		interval := time.Duration(cfg.us) * time.Microsecond
		cpuBefore, cpuOK := schedulerMeasurementCPU()
		started := time.Now()
		jobID := newSchedJob(id, "measured_tick", interval, false, bm)
		schedMu.Lock()
		job := schedJobs[jobID]
		schedMu.Unlock()
		time.Sleep(3 * time.Second)
		cancelSchedJob(jobID)
		// Cancellation prevents new starts; wait for the running callback to finish.
		deadline := time.Now().Add(time.Second)
		for {
			schedMu.Lock()
			running := job.running
			schedMu.Unlock()
			if !running {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("callback did not finish")
			}
			time.Sleep(time.Millisecond)
		}
		measured := time.Since(started)
		cpuAfter, cpuAfterOK := schedulerMeasurementCPU()
		state, err := yyCallResident(bm, id, "state", nil, false, false)
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		stamps := append([]time.Time(nil), times...)
		mu.Unlock()
		gaps := []float64{}
		for i := 1; i < len(stamps); i++ {
			gaps = append(gaps, float64(stamps[i].Sub(stamps[i-1]))/float64(time.Millisecond))
		}
		sort.Float64s(gaps)
		percentile := func(p float64) float64 {
			if len(gaps) == 0 {
				return 0
			}
			return gaps[int(float64(len(gaps)-1)*p)]
		}
		calls := job.calls.Load()
		if calls == 0 {
			calls = 1
		}
		row := map[string]any{"requested_interval_ms": float64(cfg.us) / 1000, "lua_loop_iterations_per_round": cfg.iterations, "duration_seconds": measured.Seconds(), "completed_rounds": len(stamps), "rounds_per_second": float64(len(stamps)) / measured.Seconds(), "gap_p50_ms": percentile(.5), "gap_p95_ms": percentile(.95), "gap_max_ms": percentile(1), "missed": job.missed.Load(), "failures": job.failures.Load(), "callback_average_ms": float64(job.totalNS.Load()) / float64(calls) / 1e6, "state": state}
		if cpuOK && cpuAfterOK {
			row["process_cpu_seconds"] = cpuAfter - cpuBefore
			row["process_cpu_percent_one_core"] = 100 * (cpuAfter - cpuBefore) / measured.Seconds()
		}
		rows = append(rows, row)
		yyStopResident(bm, id)
		if len(stamps) == 0 || job.failures.Load() != 0 {
			t.Fatalf("scheduler failed: %v", row)
		}
	}
	report := map[string]any{"environment": "actual Linux resident Lua host scheduler", "game_connected": false, "game_commands_executed": false, "measurements": rows}
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, bytes, 0600); err != nil {
		t.Fatal(err)
	}
}

// Linux /proc reports process CPU in CLK_TCK units; the runner supplies getconf CLK_TCK.
func schedulerMeasurementCPU() (float64, bool) {
	ticks, e := strconv.ParseFloat(os.Getenv("YY_CPU_TICKS"), 64)
	if e != nil || ticks <= 0 {
		return 0, false
	}
	b, e := os.ReadFile("/proc/self/stat")
	if e != nil {
		return 0, false
	}
	end := strings.LastIndex(string(b), ")")
	if end < 0 {
		return 0, false
	}
	f := strings.Fields(string(b)[end+1:])
	if len(f) < 13 {
		return 0, false
	}
	u, e := strconv.ParseFloat(f[11], 64)
	if e != nil {
		return 0, false
	}
	sys, e := strconv.ParseFloat(f[12], 64)
	if e != nil {
		return 0, false
	}
	return (u + sys) / ticks, true
}
