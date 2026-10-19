package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func yyAdmissionFixture(humans int) yyAdmissionSnapshot {
	x := yyAdmissionSnapshot{Scope: "device-owner", Session: "server/connection-1", PrimaryName: "observer", BoundOwner: true, Connected: true, OP: true, RobotsKnown: true, Complete: true, Names: []string{"observer"}, Robots: []string{"observer", "worker-one", "worker-two"}}
	for i := 1; i <= humans; i++ {
		x.Names = append(x.Names, fmt.Sprintf("human-%02d", i))
	}
	return x
}

func TestAdmissionRunsWithObservationOffAndOnlyRejects23rdNewHuman(t *testing.T) {
	state, now := yyAdmissionState{}, time.Unix(1000, 0)
	policy := yyAdmissionPolicy{ObservationEnabled: false, Enforce: true} // Default limit22.
	x, commands := yyAdmissionFixture(22), []string{}
	read := func() yyAdmissionSnapshot { return x }
	reject := func(name string, limit int) yyAdmissionAttempt {
		if limit != 22 {
			t.Fatalf("default limit %d", limit)
		}
		commands = append(commands, name)
		return yyAdmissionAttempt{Sent: true, Confirmed: true}
	}
	if r := state.step(policy, now, read, reject); r.Phase != "baseline" || len(commands) != 0 {
		t.Fatal(r, commands)
	}
	x.Names = append(x.Names, "worker-one", "worker-two")
	if r := state.step(policy, now.Add(time.Second), read, reject); r.Humans == nil || *r.Humans != 22 || len(commands) != 0 {
		t.Fatal("robots counted as humans", r, commands)
	}
	x.Names = append(x.Names, "human-23")
	r := state.step(policy, now.Add(2*time.Second), read, reject)
	if !reflect.DeepEqual(commands, []string{"human-23"}) || len(r.Attempts) != 1 || !r.Attempts[0].Confirmed {
		t.Fatal(r, commands)
	}
	// Turning observation on/off does not reclassify established users as new.
	policy.ObservationEnabled = true
	state.step(policy, now.Add(3*time.Second), read, reject)
	policy.ObservationEnabled = false
	state.step(policy, now.Add(4*time.Second), read, reject)
	if len(commands) != 1 {
		t.Fatal("uncertain/remaining newcomer blindly retried", commands)
	}
}

func TestAdmissionNeverRejectsExistingRosterWhenEnabledLoweredOrReconnected(t *testing.T) {
	now := time.Unix(1000, 0)
	state, x, commands := yyAdmissionState{}, yyAdmissionFixture(25), []string{}
	read := func() yyAdmissionSnapshot { return x }
	reject := func(name string, limit int) yyAdmissionAttempt {
		commands = append(commands, name)
		return yyAdmissionAttempt{Sent: true}
	}
	policy := yyAdmissionPolicy{Enforce: true, Limit: 22}
	state.step(policy, now, read, reject)
	policy.Limit = 10
	state.step(policy, now.Add(time.Second), read, reject)
	if len(commands) != 0 {
		t.Fatal("existing users rejected on startup or lower limit", commands)
	}
	x.Session = "server/connection-2"
	x.Names = append(x.Names, "present-before-reconnect")
	state.step(policy, now.Add(2*time.Second), read, reject)
	policy.Enforce = false
	reads := 0
	state.step(policy, now.Add(3*time.Second), func() yyAdmissionSnapshot { reads++; return x }, reject)
	if reads != 0 {
		t.Fatal("disabled admission queried server")
	}
	policy.Enforce = true
	x.Names = append(x.Names, "present-before-enable")
	state.step(policy, now.Add(4*time.Second), read, reject)
	if len(commands) != 0 {
		t.Fatal("existing users rejected after enabling/reconnection", commands)
	}
}

func TestAdmissionUnknownRosterOrAuthorityFailsClosedAndRebaselines(t *testing.T) {
	cases := map[string]func(*yyAdmissionSnapshot){
		"not bound owner":        func(x *yyAdmissionSnapshot) { x.BoundOwner = false },
		"disconnected":           func(x *yyAdmissionSnapshot) { x.Connected = false },
		"no OP":                  func(x *yyAdmissionSnapshot) { x.OP = false },
		"unknown robots":         func(x *yyAdmissionSnapshot) { x.RobotsKnown = false },
		"failed roster":          func(x *yyAdmissionSnapshot) { x.Complete = false },
		"empty roster":           func(x *yyAdmissionSnapshot) { x.Names = nil },
		"observer absent":        func(x *yyAdmissionSnapshot) { x.Names = x.Names[1:] },
		"duplicate name":         func(x *yyAdmissionSnapshot) { x.Names = append(x.Names, "human-01") },
		"unknown blank identity": func(x *yyAdmissionSnapshot) { x.Names = append(x.Names, "") },
		"empty session":          func(x *yyAdmissionSnapshot) { x.Session = "" },
	}
	for name, invalidate := range cases {
		t.Run(name, func(t *testing.T) {
			state, now, x, calls := yyAdmissionState{}, time.Unix(1000, 0), yyAdmissionFixture(22), 0
			policy := yyAdmissionPolicy{Enforce: true, Limit: 22}
			read := func() yyAdmissionSnapshot { return x }
			reject := func(string, int) yyAdmissionAttempt { calls++; return yyAdmissionAttempt{Sent: true} }
			state.step(policy, now, read, reject)
			x = yyAdmissionFixture(23)
			invalidate(&x)
			state.step(policy, now.Add(time.Second), read, reject)
			if calls != 0 {
				t.Fatal("invalid snapshot rejected someone")
			}
			// The recovery roster can contain already-present people. It is a new
			// baseline, not a synthetic burst of join events after an empty failure.
			x = yyAdmissionFixture(24)
			r := state.step(policy, now.Add(2*time.Second), read, reject)
			if calls != 0 || r.Phase != "baseline" {
				t.Fatal("recovery caused false newcomer", r, calls)
			}
		})
	}
}

func TestAdmissionOnlyCountsFullRosterAndOnlyNewEntriesCanBeRejected(t *testing.T) {
	state, now, x := yyAdmissionState{}, time.Unix(1000, 0), yyAdmissionFixture(24)
	policy, rejected := yyAdmissionPolicy{Enforce: true, Limit: 22}, []string{}
	read := func() yyAdmissionSnapshot { return x }
	reject := func(name string, limit int) yyAdmissionAttempt {
		rejected = append(rejected, name)
		return yyAdmissionAttempt{Sent: true}
	}
	state.step(policy, now, read, reject)
	x.Names = append(x.Names, "z-new", "a-new")
	state.step(policy, now.Add(time.Second), read, reject)
	if !reflect.DeepEqual(rejected, []string{"a-new", "z-new"}) {
		t.Fatal("existing excess users selected", rejected)
	}
	// A scope change cannot inherit the previous owner's join baseline.
	x.Scope = "other-owner"
	x.Names = append(x.Names, "other-owner-existing")
	state.step(policy, now.Add(2*time.Second), read, reject)
	if len(rejected) != 2 {
		t.Fatal("cross-owner baseline reused", rejected)
	}
}
