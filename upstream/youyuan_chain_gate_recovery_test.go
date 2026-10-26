package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestChainGateRetriesOnlyProvenReadConditions(t *testing.T) {
	for _, command := range []string{"testfor @a[tag=x]", "execute unless entity @a[scores={雪球菜单=1..}] unless entity @a[tag=x] run testfor @a"} {
		if !yyChainGateReadOnly(command) {
			t.Fatal("read rejected", command)
		}
	}
	for _, command := range []string{"scoreboard players add x y 1", "execute summon pig run testfor @a", "execute unless entity @a[tag=x] run summon pig", "execute unless entity @a[tag=x] run execute summon pig run testfor @a"} {
		if yyChainGateReadOnly(command) {
			t.Fatal("write accepted for retry", command)
		}
	}
}

func TestChainGateReadOutageJoinsChildAndRecoversWithoutFalseBranch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var alive, starts, errorsRecorded atomic.Int32
	probes := 0
	err := yyChainRunGate(ctx, &yyChainControl{Command: "testfor @a[tag=request]", Then: "body.txt", Else: "idle.txt", PollMS: 100}, yyChainConfig{}, []string{"main.txt"},
		func(name, _ string) (yyChainPlan, yyChainConfig, error) {
			if name != "body.txt" {
				t.Fatal("unknown read selected false branch", name)
			}
			return yyChainPlan{}, yyChainConfig{}, nil
		},
		func(context.Context, yyChainNode, yyChainConfig) (bool, error) {
			probes++
			switch probes {
			case 2:
				return false, errors.New("connection not ready")
			case 3:
				if alive.Load() != 0 {
					t.Fatal("old child survived read outage")
				}
			case 4:
				cancel()
			}
			return true, nil
		}, func(string) {}, func(_, _ bool, e error) {
			if e != nil {
				errorsRecorded.Add(1)
			}
		},
		func(in *yyChainInstance, c context.Context) {
			starts.Add(1)
			if alive.Add(1) != 1 {
				t.Error("overlapping children")
			}
			<-c.Done()
			alive.Add(-1)
			close(in.done)
		})
	if !errors.Is(err, context.Canceled) || starts.Load() != 2 || alive.Load() != 0 || errorsRecorded.Load() != 1 {
		t.Fatal(err, starts.Load(), alive.Load(), errorsRecorded.Load())
	}
}

func TestChainGateUnknownWriteIsNotRetried(t *testing.T) {
	probes := 0
	network := errors.New("unknown write result")
	err := yyChainRunGate(context.Background(), &yyChainControl{Command: "scoreboard players add x y 1", Then: "body.txt", PollMS: 100}, yyChainConfig{}, nil, nil,
		func(context.Context, yyChainNode, yyChainConfig) (bool, error) { probes++; return false, network }, func(string) {}, func(bool, bool, error) {}, nil)
	if !errors.Is(err, network) || probes != 1 {
		t.Fatal("unknown side effect was repeated", err, probes)
	}
}
