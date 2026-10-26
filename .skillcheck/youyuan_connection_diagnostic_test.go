package main

import "testing"

// A closed connection must say why the attempt started failing, not only what
// the last socket read returned. Before this, the history ended with "EOF" and
// nothing distinguished an auth failure from a kick or a network drop.
func TestConnectionHistoryKeepsFirstError(t *testing.T) {
	s := &yySenseState{}
	yyConnectionAppendLocked(s, "connect_attempt", "", nil)
	yyConnectionAppendLocked(s, "connect_failed", "认证失败", nil)
	yyConnectionAppendLocked(s, "connection_closed", "EOF", nil)

	if len(s.ConnectionHistory) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(s.ConnectionHistory))
	}
	last := s.ConnectionHistory[2]
	if last["first_error"] != "认证失败" {
		t.Fatalf("first error must survive to the closed entry, got %v", last["first_error"])
	}
	if last["first_error_phase"] != "connect_failed" {
		t.Fatalf("first error phase lost, got %v", last["first_error_phase"])
	}
	if last["attempt"] != uint64(1) {
		t.Fatalf("expected attempt 1, got %v", last["attempt"])
	}
}

// A new attempt must not inherit the previous attempt's first error, otherwise a
// later healthy session would keep reporting an old failure.
func TestConnectionHistoryResetsFirstErrorPerAttempt(t *testing.T) {
	s := &yySenseState{}
	yyConnectionAppendLocked(s, "connect_attempt", "", nil)
	yyConnectionAppendLocked(s, "connect_failed", "第一次失败", nil)
	yyConnectionAppendLocked(s, "connect_attempt", "", nil)
	yyConnectionAppendLocked(s, "connect_succeeded", "", nil)

	if s.ConnectionFirstErr != "" {
		t.Fatalf("first error must be cleared on a new attempt, got %q", s.ConnectionFirstErr)
	}
	last := s.ConnectionHistory[len(s.ConnectionHistory)-1]
	if last["attempt"] != uint64(2) {
		t.Fatalf("expected attempt 2, got %v", last["attempt"])
	}
	if _, exists := last["first_error"]; exists {
		t.Fatalf("healthy attempt must not carry first_error: %v", last)
	}
}

// Extra fields (server reason code) must be preserved without letting them
// overwrite the fields the diagnostic layer owns.
func TestConnectionHistoryKeepsExtraFields(t *testing.T) {
	s := &yySenseState{}
	yyConnectionAppendLocked(s, "connect_attempt", "", nil)
	yyConnectionAppendLocked(s, "server_disconnect", "服务器关闭", map[string]any{"reason_code": int32(7), "phase": "hijacked"})

	last := s.ConnectionHistory[len(s.ConnectionHistory)-1]
	if last["reason_code"] != int32(7) {
		t.Fatalf("reason_code lost: %v", last)
	}
	if last["phase"] != "server_disconnect" {
		t.Fatalf("extra fields must not overwrite phase, got %v", last["phase"])
	}
}
