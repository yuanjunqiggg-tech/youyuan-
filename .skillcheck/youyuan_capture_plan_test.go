package main

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func captureFixture(n int, d int32) []yyCapturePlayer {
	out := []yyCapturePlayer{}
	for i := 0; i < n; i++ {
		out = append(out, yyCapturePlayer{Name: fmt.Sprintf("human-%02d", i), Dimension: d, Known: true})
	}
	return out
}
func captureConfig() yyGlobalCaptureConfig {
	return yyGlobalCaptureConfig{Accounts: []string{"a", "b", "c", "d", "e", "f"}, DimensionAccounts: map[string]string{"0": "a", "1": "b", "2": "c"}, Interval: .016}
}
func TestCaptureHumanThresholdsAndManualDeployment(t *testing.T) {
	for _, v := range []struct{ n, cap, want int }{{0, 0, 0}, {1, 0, 1}, {2, 0, 2}, {3, 0, 3}, {10, 0, 3}, {11, 0, 5}, {14, 0, 5}, {15, 0, 6}, {22, 0, 6}, {5, 4, 4}, {10, 6, 6}, {15, 4, 4}} {
		if got := yyCaptureDesired(v.n, v.cap); got != v.want {
			t.Fatalf("%+v got %d", v, got)
		}
	}
}
func TestCaptureIdleDimensionsHelpAndReturnHome(t *testing.T) {
	c := captureConfig()
	c.AccountLimit = 6
	players := captureFixture(10, 0)
	plan := yyCapturePlan(c, players, c.Accounts)
	if len(plan) != 6 {
		t.Fatal(plan)
	}
	seen := map[string]bool{}
	phases := map[float64]bool{}
	for _, a := range plan {
		if len(a.Players) == 0 {
			continue
		}
		if a.Dimension != 0 || phases[a.Phase] {
			t.Fatal(a)
		}
		phases[a.Phase] = true
		for _, n := range a.Players {
			if seen[n] {
				t.Fatalf("duplicate primary target %s", n)
			}
			seen[n] = true
		}
	}
	if len(seen) != 10 {
		t.Fatal(seen)
	}
	players = append(players, yyCapturePlayer{Name: "nether-human", Dimension: 1, Known: true}, yyCapturePlayer{Name: "end-human", Dimension: 2, Known: true})
	plan = yyCapturePlan(c, players, c.Accounts)
	home := map[string]int32{}
	for _, a := range plan {
		home[a.Account] = a.Dimension
	}
	if home["b"] != 1 || home["c"] != 2 {
		t.Fatal("resident roles failed to return", plan)
	}
}
func TestCaptureFailoverAndSmallGroups(t *testing.T) {
	c := captureConfig()
	players := captureFixture(2, 1)
	// Preferred a is unavailable; an independent ready account substitutes.
	plan := yyCapturePlan(c, players, []string{"b", "d"})
	if len(plan) != 2 {
		t.Fatal(plan)
	}
	if len(plan[0].Players) > 0 && len(plan[1].Players) > 0 && plan[0].Players[0] == plan[1].Players[0] {
		t.Fatal("two observers assigned same player", plan)
	}
	if len(yyCapturePlan(c, players[:1], []string{"d"})) != 1 || len(yyCapturePlan(c, nil, c.Accounts)) != 3 {
		t.Fatal("one/zero player policy")
	}
}
func TestSharedCaptureDoesNotGrantGenericAccountControl(t *testing.T) {
	yyPoolTestSetup(t)
	actor := yyPoolActor{ID: "alice", Scope: "control"}
	yyAccounts.Items = append(yyAccounts.Items, yyManagedAccount{ID: "shared", OwnerID: "bob", Name: "shared-observer", SharedCapture: true})
	if !yyCaptureAllowed(actor, "shared") || yyCaptureAllowed(actor, "bob_bot") {
		t.Fatal("capture ownership boundary")
	}
	if _, e := yyPoolRequire(actor, "shared"); e == nil {
		t.Fatal("shared flag exposed generic file/token control")
	}
	if _, e := yyCaptureCall(actor, "shared", "game.execute", []any{"op someone"}); e == nil {
		t.Fatal("shared capture allowed arbitrary commands")
	}
}
func TestDownloadHeadersExposedToDirectFrontend(t *testing.T) {
	w := httptest.NewRecorder()
	yyDownloadCORS(w)
	for _, s := range []string{"Content-Disposition", "X-Content-SHA256", "Content-Length"} {
		if !strings.Contains(w.Header().Get("Access-Control-Expose-Headers"), s) {
			t.Fatal(s)
		}
	}
}

func TestFleetDocumentationIsReadableFromAllPublishedLanguageSkills(t *testing.T) {
	for _, id := range []string{"lua-runtime", "python-runtime", "cpp-runtime", "observer-bridge"} {
		text, e := yyAgentSkillText(id, "fleet-scheduling.md", 0, 20000)
		if e != nil || !strings.Contains(text, "accountsPool.captureConfigure") || !strings.Contains(text, "dimension_known=false") {
			t.Fatal(id, e, text)
		}
	}
}
func TestAuth400BackoffDoesNotTurnIntoFastRetryLoop(t *testing.T) {
	if yyConnectionErrorKind("认证 HTTP 400: 请先登录") != "credential_rejected" {
		t.Fatal("lost authentication cause")
	}
	previous := time.Duration(0)
	for i := uint64(1); i < 30; i++ {
		d := yyConnectionRetryDelay(time.Second, i, "认证 HTTP 400: 请先登录")
		if d < previous || d > 5*time.Minute {
			t.Fatal(i, d)
		}
		previous = d
	}
	if yyConnectionRetryDelay(time.Second, 30, "transport disconnected") != time.Second {
		t.Fatal("unrelated path changed")
	}
}

func TestReleasedCaptureWorkerGetsFreshStartupGrace(t *testing.T) {
	yyPoolTestSetup(t)
	actor := yyPoolActor{ID: "alice", Scope: "control"}
	old := yyCaptureLaunches.Items
	yyCaptureLaunches.Items = map[string]*yyCaptureLaunch{"alice_bot": {IdleAt: time.Now().Add(-time.Hour)}}
	t.Cleanup(func() { yyCaptureLaunches.Items = old })
	yyAccounts.Items[0].CaptureOnly = true
	yyAccounts.Items[0].Enabled = false
	yyCaptureRetire(actor, map[string]bool{}, yyGlobalCaptureConfig{IdleSeconds: 60})
	if !yyCaptureLaunches.Items["alice_bot"].IdleAt.IsZero() {
		t.Fatal("old deadline retained while account was released")
	}
	yyAccounts.Items[0].Enabled = true
	yyCaptureRetire(actor, map[string]bool{}, yyGlobalCaptureConfig{IdleSeconds: 60})
	l := yyCaptureLaunches.Items["alice_bot"]
	if time.Since(l.IdleAt) > time.Second || l.Busy {
		t.Fatal("newly enabled worker was immediately retired")
	}
}
