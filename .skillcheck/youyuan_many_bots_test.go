package main

import (
	"fmt"
	"testing"
)

func TestCaptureManyAccountPolicyAndFailover(t *testing.T) {
	c := captureConfig()
	c.Accounts = nil
	c.FleetSteps = []yyCaptureFleetStep{{1, 1}, {3, 3}, {8, 8}, {15, 12}, {22, 24}}
	if err := yyCaptureFleetStepsValidate(c.FleetSteps); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 32; i++ {
		c.Accounts = append(c.Accounts, fmt.Sprintf("worker-%02d", i))
	}
	players := captureFixture(22, 0)
	for i := range players {
		players[i].Dimension = int32(i % 3)
	}
	check := func(ready []string, want int) {
		t.Helper()
		plan := yyCapturePlan(c, players, ready)
		if len(plan) != want {
			t.Fatalf("got %d observers, want %d", len(plan), want)
		}
		accounts, humans, phases := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, a := range plan {
			if accounts[a.Account] {
				t.Fatal("duplicate observer", a.Account)
			}
			accounts[a.Account] = true
			key := fmt.Sprintf("%d:%g", a.Dimension, a.Phase)
			if phases[key] {
				t.Fatal("same-dimension phases collided", key)
			}
			phases[key] = true
			for _, name := range a.Players {
				humans[name] = true
			}
		}
		for _, p := range players {
			if !humans[p.Name] {
				t.Fatal("unassigned player", p.Name)
			}
		}
	}
	check(c.Accounts, 24)
	// Removing four unavailable accounts selects subsequent ready candidates.
	check(c.Accounts[4:], 24)
	check(c.Accounts[20:], 12)
	c.AccountLimit = 10
	check(c.Accounts, 10)
	if yyCaptureConfiguredDesired(c, 0) != 3 {
		t.Fatal("missing idle dimension residents")
	}
	c.AccountLimit = 0
	c.FleetSteps = []yyCaptureFleetStep{{1, 32}}
	check(c.Accounts, 32)
}

func TestCapturePolicyRejectsAmbiguousOrUnboundedSteps(t *testing.T) {
	for _, steps := range [][]yyCaptureFleetStep{
		{{2, 1}}, {{1, 0}}, {{1, 33}}, {{1, 1}, {1, 2}}, {{1, 1}, {1001, 2}},
	} {
		if yyCaptureFleetStepsValidate(steps) == nil {
			t.Fatal("accepted invalid policy", steps)
		}
	}
}

func TestConnectionRecognizesRoomIDsWithoutPrecisionLoss(t *testing.T) {
	for raw, want := range map[string]string{
		"4691202525342713927":                   "LobbyGame:room_id=4691202525342713927",
		" #4691202525342713927 ":                "LobbyGame:room_id=4691202525342713927",
		"LobbyGame:4691202525342713927":         "LobbyGame:room_id=4691202525342713927",
		"LobbyGame:room_id=4691202525342713927": "LobbyGame:room_id=4691202525342713927",
		"123456":                                "123456", "94F195C41B5": "94F195C41B5", "94F195C41B5_1": "94F195C41B5_1",
		"mc.163.com/open/mc_realms/?realms=94F195C41B5": "94F195C41B5",
	} {
		if got := yyNormalizeServer(raw); got != want {
			t.Fatalf("%q: got %q, want %q", raw, got, want)
		}
	}
}
