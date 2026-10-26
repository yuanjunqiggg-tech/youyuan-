package main

import (
	"sort"
	"strings"
	"time"
)

// Admission has no observation switch, worker-launcher or camera dependency.
// Its owner/session baseline only accepts complete, authenticated rosters.
type yyAdmissionState struct {
	scope       string
	session     string
	initialized bool
	previous    map[string]bool
	attempted   map[string]time.Time
}

type yyAdmissionPolicy struct {
	// ObservationEnabled describes the caller's switch, never admission authority.
	ObservationEnabled bool
	Enforce            bool
	Limit              int
}

type yyAdmissionSnapshot struct {
	Scope, Session, PrimaryName string
	BoundOwner, Connected, OP   bool
	RobotsKnown, Complete       bool
	Names, Robots               []string
}

type yyAdmissionAttempt struct {
	Name      string `json:"name"`
	Sent      bool   `json:"attempted"`
	Confirmed bool   `json:"confirmed"`
}

type yyAdmissionResult struct {
	Phase    string               `json:"phase"`
	Humans   *int                 `json:"humans"`
	Limit    int                  `json:"player_limit"`
	Attempts []yyAdmissionAttempt `json:"attempts,omitempty"`
}

// Only the injected reject callback can submit a command. Tests use a recorder.
func (s *yyAdmissionState) step(policy yyAdmissionPolicy, now time.Time, read func() yyAdmissionSnapshot, reject func(string, int) yyAdmissionAttempt) yyAdmissionResult {
	limit := policy.Limit
	if limit == 0 {
		limit = 22
	}
	out := yyAdmissionResult{Phase: "disabled", Limit: limit}
	if !policy.Enforce || limit < 1 || limit > 1000 {
		*s = yyAdmissionState{}
		return out
	}
	x := read()
	if x.Scope != s.scope || x.Session != s.session {
		*s = yyAdmissionState{scope: x.Scope, session: x.Session}
	}
	out.Phase = "waiting_verified_roster"
	valid := x.BoundOwner && x.Scope != "" && x.Session != "" && x.Connected && x.OP && x.RobotsKnown && x.Complete && x.PrimaryName != "" && len(x.Names) > 0
	raw := map[string]bool{}
	for _, name := range x.Names {
		if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) || raw[name] {
			valid = false
		}
		raw[name] = true
	}
	// A connected observer must occur in a full /list response. Incomplete or
	// failed snapshots never turn the recovery roster into synthetic new joins.
	if !valid || !raw[x.PrimaryName] {
		s.initialized, s.previous = false, nil
		return out
	}
	robots := map[string]bool{x.PrimaryName: true}
	for _, name := range x.Robots {
		robots[name] = true
	}
	humans := map[string]bool{}
	for name := range raw {
		if !robots[name] {
			humans[name] = true
		}
	}
	count := len(humans)
	out.Humans = &count
	if !s.initialized {
		s.initialized, s.previous = true, humans
		out.Phase = "baseline"
		return out
	}
	out.Phase = "ready"
	newNames := []string{}
	for name := range humans {
		if !s.previous[name] {
			newNames = append(newNames, name)
		}
	}
	sort.Strings(newNames)
	over := len(humans) - limit
	for _, name := range newNames {
		if over <= 0 {
			break
		}
		if at, exists := s.attempted[name]; exists && now.Sub(at) < 30*time.Second {
			continue
		}
		attempt := reject(name, limit)
		attempt.Name = name
		out.Attempts = append(out.Attempts, attempt)
		if attempt.Sent {
			if s.attempted == nil {
				s.attempted = map[string]time.Time{}
			}
			s.attempted[name] = now
			over--
		}
	}
	// An unknown command result is not blindly replayed on the next tick.
	s.previous = humans
	for name, at := range s.attempted {
		if !humans[name] && now.Sub(at) >= 30*time.Second {
			delete(s.attempted, name)
		}
	}
	return out
}
