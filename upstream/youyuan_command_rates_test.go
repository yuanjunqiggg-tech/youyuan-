package main

import (
	lua "github.com/yuin/gopher-lua"
	"testing"
	"time"
)

func TestExplicitRateOverridesLegacySingleAndBatchPace(t *testing.T) {
	for _, c := range []struct {
		count, rate int
		want        time.Duration
	}{
		{1, 0, 0}, {16, 0, 0},
		{1, 500, 2 * time.Millisecond}, {16, 1000, 16 * time.Millisecond},
		{25, 1000, 25 * time.Millisecond},
	} {
		if got := yyControlDispatchPace(c.count, c.rate); got != c.want {
			t.Fatalf("count=%d rate=%d got=%v want=%v", c.count, c.rate, got, c.want)
		}
	}
}

func TestChainFrequencyAppliesToWholeRound(t *testing.T) {
	if yyChainRoundWait(50*time.Millisecond, 20*time.Millisecond) != 30*time.Millisecond {
		t.Fatal("round work must count toward period")
	}
	if yyChainRoundWait(50*time.Millisecond, 80*time.Millisecond) != 0 {
		t.Fatal("overrun must not add another period")
	}
}

func TestLuaBatchResultsRemainIterable(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("reply", goToLua(L, map[string]any{"results": []map[string]any{
		{"confirmed": true, "success_count": uint32(1), "output": []string{"first"}},
		{"confirmed": true, "success_count": uint32(0), "output": []string{"no target"}},
	}}))
	if err := L.DoString(`local n=0;for _,r in ipairs(reply.results) do
assert(r.confirmed);assert(type(r.success_count)=="number");assert(type(r.output)=="table");n=n+1
end;assert(n==2);assert(reply.results[2].output[1]=="no target")`); err != nil {
		t.Fatal(err)
	}
}
