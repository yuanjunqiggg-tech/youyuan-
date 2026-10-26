package main

import (
	"context"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestChainGateLoadsOnConditionAndJoinsBeforeNextActivation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var starts, alive, maxAlive, probes atomic.Int32
	var paths [][]string
	lookup := func(name, account string) (yyChainPlan, yyChainConfig, error) {
		if name != "body.txt" || account != "owner" {
			t.Fatal(name, account)
		}
		return yyChainPlan{}, yyChainConfig{Name: name, Account: account}, nil
	}
	runner := func(in *yyChainInstance, c context.Context) {
		starts.Add(1)
		n := alive.Add(1)
		if n > maxAlive.Load() {
			maxAlive.Store(n)
		}
		<-c.Done()
		alive.Add(-1)
		close(in.done)
	}
	gate := &yyChainControl{Command: "condition", Then: "body.txt", PollMS: 5}
	err := yyChainRunGate(ctx, gate, yyChainConfig{Account: "owner"}, []string{"parent.txt"}, lookup,
		func(context.Context, yyChainNode, yyChainConfig) (bool, error) {
			n := probes.Add(1)
			if n == 3 && alive.Load() != 0 {
				t.Fatal("idle gate launched a child")
			}
			if n == 7 && alive.Load() != 0 {
				t.Fatal("false condition did not join child")
			}
			if n == 10 {
				cancel()
			}
			return n >= 3 && n <= 5 || n >= 8, nil
		}, func(string) {}, func(bool, bool, error) {}, func(in *yyChainInstance, c context.Context) {
			paths = append(paths, yyChainCallPath(c, "ignored"))
			runner(in, c)
		})
	if err != context.Canceled || starts.Load() != 2 || alive.Load() != 0 || maxAlive.Load() != 1 || !reflect.DeepEqual(paths, [][]string{{"parent.txt", "body.txt"}, {"parent.txt", "body.txt"}}) {
		t.Fatal(err, starts.Load(), alive.Load(), maxAlive.Load(), paths)
	}
}

func TestChainGateSlowProbeDoesNotThrottleChildAndRejectsRecursion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 480*time.Millisecond)
	defer cancel()
	var rounds atomic.Int32
	err := yyChainRunGate(ctx, &yyChainControl{Command: "condition", Then: "body.txt", PollMS: 100}, yyChainConfig{}, []string{"parent.txt"},
		func(string, string) (yyChainPlan, yyChainConfig, error) { return yyChainPlan{}, yyChainConfig{}, nil },
		func(c context.Context, n yyChainNode, _ yyChainConfig) (bool, error) {
			if !n.RequireResult {
				t.Fatal("condition accepted without a server result")
			}
			return true, yyChainSleep(c, 80*time.Millisecond)
		}, func(string) {}, func(bool, bool, error) {}, func(in *yyChainInstance, c context.Context) {
			defer close(in.done)
			for c.Err() == nil {
				rounds.Add(1)
				_ = yyChainSleep(c, 50*time.Millisecond)
			}
		})
	if err != context.DeadlineExceeded || rounds.Load() < 6 || rounds.Load() > 10 {
		t.Fatal(err, rounds.Load())
	}
	err = yyChainRunGate(context.Background(), &yyChainControl{Command: "condition", Then: "parent.txt"}, yyChainConfig{}, []string{"parent.txt"}, nil,
		func(context.Context, yyChainNode, yyChainConfig) (bool, error) { return true, nil }, func(string) {}, func(bool, bool, error) {}, nil)
	if err == nil || !strings.Contains(err.Error(), "递归") {
		t.Fatal(err)
	}
}

func TestChainPluginCallsResidentExportWithoutCrossingAccount(t *testing.T) {
	bm := &BotManager{}
	const id = "chain-api-test22"
	if err := LoadLuaPlugin(bm, id, `local n=0;plugin_api={add=function(a,b)n=n+1;return a+b end,count=function()return n end}`); err != nil {
		t.Fatal(err)
	}
	defer yyStopResident(bm, id)
	c, err := yyParseChainControl(`@plugin {"plugin_id":"chain-api-test22","function":"add","args":[2,3]}`)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := yyChainInvokePlugin(context.Background(), bm, c); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if n, err := yySenseCall(bm, "plugin.call", []any{id, "count", []any{}}); err != nil || n != float64(1) {
		t.Fatal(n, err)
	}
	if _, err := yyChainInvokePlugin(context.Background(), &BotManager{}, c); err == nil {
		t.Fatal("called another account's plugin")
	}
	if _, err := yyParseChainControl(`@plugin {"plugin_id":"x"}`); err == nil {
		t.Fatal("missing export accepted")
	}
}
