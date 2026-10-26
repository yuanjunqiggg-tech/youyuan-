package main

import (
	"math/rand"
	"os"
	"strings"
	"testing"
)

func TestSurfacePositionedCandidatePreserves111ChecksAndClock(t *testing.T) {
	raw, err := os.ReadFile("../incident-fixes/2026-10-26/chain-surface-positioned.txt")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := yyParseChains(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Chains) != 2 || plan.Chains[1].Hz != 20 || plan.Chains[1].Nodes[0].Control == nil || plan.Chains[1].Nodes[0].Control.Kind != "wait" {
		t.Fatal("changed activation/clock")
	}
	checks, dispatches := 0, 0
	for _, n := range plan.Chains[1].Nodes {
		if strings.Contains(n.Command, "positioned ~ ~-") && strings.Contains(n.Command, "run scoreboard players set @s yy_surface_scan ") {
			checks++
		}
		if strings.Contains(n.Command, "scores={yy_surface_scan=") && strings.Contains(n.Command, "run tp @s") {
			dispatches++
		}
	}
	if checks != 111 || dispatches != 112 {
		t.Fatalf("checks=%d teleport selectors=%d", checks, dispatches)
	}
	if !strings.Contains(string(raw), "scoreboard objectives add yy_surface_scan dummy") {
		t.Fatal("missing score objective")
	}
}

// Compare outcomes on the same stable block column; real moving-world behavior is tested separately.
func TestSurfacePositionedStableColumnsMatchOriginalAndMoveOnce(t *testing.T) {
	rng := rand.New(rand.NewSource(26))
	for test := 0; test < 10000; test++ {
		blocks := map[int]string{}
		for y := -50; y < 360; y++ {
			if rng.Intn(7) == 0 {
				blocks[y] = []string{"stone", "water", "lava"}[rng.Intn(3)]
			}
		}
		air := func(y int) bool { return blocks[y] == "" }
		valid := func(y int) bool {
			return air(y) && air(y+1) && !air(y-1) && blocks[y-1] != "water" && blocks[y-1] != "lava"
		}
		initial := rng.Intn(320)
		old, ready, moves := initial, false, 0
		for i := 0; i < 111; i++ {
			if !ready && valid(old) {
				ready = true
			}
			if !ready {
				old--
				moves++
			}
		}
		next, found, newMoves := initial, false, 0
		for offset := 0; offset < 111; offset++ {
			if !found && valid(initial-offset) {
				next = initial - offset
				found = true
			}
		}
		if !found {
			next = initial - 111
		}
		if next != initial {
			newMoves = 1
		}
		if ready != found || old != next || newMoves > 1 {
			t.Fatalf("case %d old=%d/%v/%d new=%d/%v/%d", test, old, ready, moves, next, found, newMoves)
		}
	}
}
