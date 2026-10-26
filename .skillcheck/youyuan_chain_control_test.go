package main

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestChainCallRangeOrderAndBarrier(t *testing.T) {
	parent, e := yyParseChains("say first\n@call {\"name\":\"child.txt\",\"from_line\":3,\"to_line\":4,\"count\":2}\nsay last")
	if e != nil {
		t.Fatal(e)
	}
	child, _ := yyParseChains("# header\nsay excluded\nsay child1\nsay child2\nsay excluded2")
	var got []string
	single := func(_ context.Context, n yyChainNode, _ yyChainConfig) (bool, error) {
		got = append(got, n.Command)
		return true, nil
	}
	batch := func(_ context.Context, nodes []yyChainNode, _ yyChainConfig) ([]map[string]any, error) {
		var out []map[string]any
		for _, n := range nodes {
			if n.Control != nil {
				t.Fatal("control sent as game command")
			}
			got = append(got, n.Command)
			out = append(out, map[string]any{"confirmed": true, "success": true})
		}
		return out, nil
	}
	lookup := func(name, account string) (yyChainPlan, yyChainConfig, error) {
		if name != "child.txt" || account != "main" {
			t.Fatal(name, account)
		}
		return child, yyChainConfig{Mode: "burst", Account: "main", IntervalMS: 25}, nil
	}
	e = yyChainExecuteGraph(context.Background(), parent, yyChainConfig{Name: "parent.txt", Mode: "burst", Account: "main"}, []string{"parent.txt"}, lookup, single, batch, func() bool { return false }, func(string) {}, func(bool, bool, error) {})
	if e != nil || !reflect.DeepEqual(got, []string{"say first", "say child1", "say child2", "say child1", "say child2", "say last"}) {
		t.Fatalf("%v %v", got, e)
	}
}
func TestChainRejectRecursiveAndCancelPersistent(t *testing.T) {
	p, _ := yyParseChains("@call {\"name\":\"self.txt\",\"keep_open\":true}\nsay forbidden")
	sends := 0
	send := func(context.Context, yyChainNode, yyChainConfig) (bool, error) { sends++; return true, nil }
	lookup := func(string, string) (yyChainPlan, yyChainConfig, error) { return p, yyChainConfig{}, nil }
	batch := func(context.Context, []yyChainNode, yyChainConfig) ([]map[string]any, error) {
		t.Fatal("unexpected batch")
		return nil, nil
	}
	e := yyChainExecuteGraph(context.Background(), p, yyChainConfig{}, []string{"self.txt"}, lookup, send, batch, func() bool { return false }, func(string) {}, func(bool, bool, error) {})
	if e == nil || !strings.Contains(e.Error(), "递归") || sends != 0 {
		t.Fatal(e, sends)
	}
	p, _ = yyParseChains("@pause\nsay forbidden")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	e = yyChainExecuteGraph(ctx, p, yyChainConfig{}, nil, lookup, send, batch, func() bool { return false }, func(string) {}, func(bool, bool, error) {})
	if e == nil || sends != 0 || time.Since(start) > time.Second {
		t.Fatal(e, sends)
	}
}
func TestChainWaitRisingEdge(t *testing.T) {
	p, e := yyParseChains("@wait {\"command\":\"testfor @a\",\"poll_ms\":100}\nsay resumed")
	if e != nil {
		t.Fatal(e)
	}
	samples := []bool{true, false, false, true}
	calls := 0
	resumed := false
	e = yyChainExecuteGraph(context.Background(), p, yyChainConfig{}, nil, nil, func(_ context.Context, n yyChainNode, _ yyChainConfig) (bool, error) {
		if n.Command == "say resumed" {
			resumed = true
			return true, nil
		}
		v := samples[calls]
		calls++
		return v, nil
	}, nil, func() bool { return false }, func(string) {}, func(bool, bool, error) {})
	if e != nil || calls != 4 || !resumed {
		t.Fatal(e, calls, resumed)
	}
}
func TestCommandBudgetCancelDoesNotReserve(t *testing.T) {
	bm := &BotManager{connected: true, stopCh: make(chan struct{}), controlCommandNext: time.Now().Add(time.Hour)}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	e := bm.waitControlCommandRateContext(ctx, 1024, 1024, 200)
	if e == nil || bm.controlCommands.Load() != 0 {
		t.Fatal("canceled reservation sent commands", e)
	}
	bm.controlCommandMu.Lock()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel2()
	e = bm.waitControlCommandRateContext(ctx2, 1, 16, 200)
	bm.controlCommandMu.Unlock()
	if e == nil {
		t.Fatal("mutex wait ignored cancellation")
	}
}
func TestChainDurationEndsInvocationAndResumes(t *testing.T) {
	p, _ := yyParseChains("@call {\"name\":\"child.txt\",\"count\":0,\"duration_ms\":60}\nsay resumed")
	child, _ := yyParseChains("say child")
	count := 0
	resumed := false
	e := yyChainExecuteGraph(context.Background(), p, yyChainConfig{}, nil, func(string, string) (yyChainPlan, yyChainConfig, error) {
		return child, yyChainConfig{IntervalMS: 25}, nil
	}, func(_ context.Context, n yyChainNode, _ yyChainConfig) (bool, error) {
		if n.Command == "say resumed" {
			resumed = true
		} else {
			count++
		}
		return true, nil
	}, nil, func() bool { return false }, func(string) {}, func(bool, bool, error) {})
	if e != nil || !resumed || count < 1 || count > 4 {
		t.Fatal(e, resumed, count)
	}
	for _, s := range []string{"@call {\"name\":\"c.txt\",\"count\":0}", "@wait {\"command\":\"testfor @a\",\"poll_ms\":1}", "@call {\"name\":\"../a.txt\"}", "@call {\"name\":\"a.txt\",\"unknown\":true}"} {
		if _, err := yyParseChains(s); err == nil {
			t.Fatal("invalid control accepted", s)
		}
	}
}

func TestBufferedCanceledBudgetDoesNotCountUnsentNodes(t *testing.T) {
	p, _ := yyParseChains("say a\nsay b\nsay c")
	ctx, cancel := context.WithCancel(context.Background())
	records := 0
	yyChainExecuteBuffered(ctx, p, yyChainConfig{Mode: "burst"}, 0, nil, func([]yyChainNode) ([]map[string]any, error) { cancel(); return nil, ctx.Err() }, func(bool, bool, error) { records++ })
	if records != 0 {
		t.Fatal("unsent canceled window counted as executed", records)
	}
}
