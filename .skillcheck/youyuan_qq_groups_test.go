package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQQMultipleGroupIsolationAndExplicitEmpty(t *testing.T) {
	cf := yyQQConfig{GroupID: "legacy"}
	if !yyQQGroupEnabled(cf, "legacy") {
		t.Fatal("legacy config lost")
	}
	cf.Groups = []yyQQGroup{}
	if yyQQGroupEnabled(cf, "legacy") {
		t.Fatal("deleted group resurrected")
	}
	cf.Groups = []yyQQGroup{{ID: "a", Enabled: true}, {ID: "b", Enabled: false}, {ID: "c", Enabled: true}}
	if !yyQQGroupEnabled(cf, "a") || yyQQGroupEnabled(cf, "b") || yyQQGroupEnabled(cf, "legacy") {
		t.Fatal("group allowlist")
	}
	yyQQRememberReply("a", "group-a-test-message")
	yyQQRememberReply("c", "group-c-test-message")
	if yyQQReplyGroup("group-c-test-message") != "c" || yyQQReplyGroup("missing") != "" {
		t.Fatal("reply routed to wrong group")
	}
	_, _, e := yyQQReserveReply("a", "group-c-test-message")
	if e == nil {
		t.Fatal("reply reused in another group")
	}
}
func TestQQGroupEnrollmentWriteFailureAndOneUse(t *testing.T) {
	old, path := yyQQ, yyQQPath
	defer func() { yyQQ = old; yyQQPath = path }()
	yyQQ = yyQQConfig{GroupID: "primary"}
	yyQQGroupClaim.Lock()
	yyQQGroupClaim.Code = "test-enroll-code"
	yyQQGroupClaim.Until = time.Now().Add(time.Minute)
	yyQQGroupClaim.Unlock()
	yyQQPath = filepath.Join(t.TempDir(), "missing", "config.json")
	if ok, e := yyQQEnrollOfficialGroup("new-group", "/加群 test-enroll-code"); ok || e == nil || yyQQGroupEnabled(yyQQ, "new-group") {
		t.Fatal("unpersisted group admitted")
	}
	yyQQPath = filepath.Join(t.TempDir(), "config.json")
	if ok, e := yyQQEnrollOfficialGroup("new-group", "/加群 test-enroll-code"); !ok || e != nil {
		t.Fatal(e)
	}
	if !yyQQGroupEnabled(yyQQ, "primary") || !yyQQGroupEnabled(yyQQ, "new-group") {
		t.Fatal("lost groups")
	}
	if ok, _ := yyQQEnrollOfficialGroup("attacker-group", "/加群 test-enroll-code"); ok {
		t.Fatal("claim reused")
	}
}
func TestQQLogsNewestFirst(t *testing.T) {
	yyObservation.Lock()
	old := yyObservation.Chat
	yyObservation.Chat = []yyChatRecord{{Player: "p", Message: "oldest"}, {Player: "p", Message: "middle"}, {Player: "p", Message: "newest"}}
	yyObservation.Unlock()
	defer func() { yyObservation.Lock(); yyObservation.Chat = old; yyObservation.Unlock() }()
	text := yyQQChatLogs(2)
	if strings.Contains(text, "oldest") || strings.Index(text, "newest") > strings.Index(text, "middle") {
		t.Fatal(text)
	}
}
