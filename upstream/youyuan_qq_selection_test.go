package main

import (
	"strings"
	"testing"
	"time"
)

func TestQQPlayerSelectionIsolation(t *testing.T) {
	yyQQPlayerGroups = map[string]string{}
	yyQQChoices = map[string]yyQQGroupChoice{}
	cf := yyQQConfig{Groups: []yyQQGroup{{ID: "opaque-a", Name: "第一群", Number: "123456", Enabled: true}, {ID: "opaque-b", Name: "第二群", Number: "654321", Enabled: true}}}
	now := time.Now()
	h, g, m, n := yyQQSelection(cf, "甲", ".通 第一条", now)
	if !h || g != "" || m != "" || !strings.Contains(n, "123456") {
		t.Fatal(h, g, m, n)
	}
	_, g, m, n = yyQQSelection(cf, "甲", ".Q群选择 2", now)
	if g != "opaque-b" || m != "第一条" || !strings.Contains(n, "654321") {
		t.Fatal(g, m, n)
	}
	_, g, m, _ = yyQQSelection(cf, "甲", ".通 后续", now)
	if g != "opaque-b" || m != "后续" {
		t.Fatal(g, m)
	}
	_, g, _, _ = yyQQSelection(cf, "乙", ".通", now)
	if g != "" {
		t.Fatal("another player inherited choice")
	}
	_, g, _, _ = yyQQSelection(cf, "乙", ".Q群选择 1", now.Add(6*time.Minute))
	if g != "" {
		t.Fatal("expired selection accepted")
	}
	cf.Groups[1].Enabled = false
	_, g, _, n = yyQQSelection(cf, "甲", ".通 失效群", now)
	if g != "" || !strings.Contains(n, "请选择") {
		t.Fatal(g, n)
	}
	if yyQQGroupLabel(cf, "opaque-a") != "123456" {
		t.Fatal("opaque group id leaked")
	}
}
func TestQQCompanionTimeoutAndOwner(t *testing.T) {
	yyQQCompanions.Sessions = map[string]yyQQCompanionSession{}
	yyQQCompanions.Epoch = map[string]uint64{}
	cf := yyQQConfig{GroupID: "g"}
	cf.Assistant.Companion = true
	cf.Assistant.Model = "test"
	cf.Assistant.CompanionOwner = "owner"
	cf.Assistant.CompanionKeywords = []string{"服主"}
	now := time.Now()
	k, e, s, initial, ok, _ := yyQQCompanionBegin(cf, "a", "服主在吗", false, now)
	if !ok || !initial {
		t.Fatal("not started")
	}
	if yyQQCompanionCurrent(cf, k, e, s, now.Add(31*time.Second)) {
		t.Fatal("session did not expire")
	}
	k, e, s, _, ok, _ = yyQQCompanionBegin(cf, "a", "后续消息", false, now.Add(10*time.Second))
	if !ok {
		t.Fatal("not continued")
	}
	yyQQCompanionBegin(cf, "owner", "来了", false, now.Add(11*time.Second))
	if yyQQCompanionCurrent(cf, k, e, s, now.Add(12*time.Second)) {
		t.Fatal("owner did not stop session")
	}
	if _, _, _, _, ok, _ = yyQQCompanionBegin(cf, "b", "普通消息", false, now); ok {
		t.Fatal("untriggered user activated")
	}
	if _, _, _, _, ok, _ = yyQQCompanionBegin(cf, "b", "/命令 say test", true, now); ok {
		t.Fatal("command swallowed")
	}
}
func TestQQFriendlyIdentity(t *testing.T) {
	cf := yyQQConfig{}
	cf.Assistant.Nicknames = map[string]string{"opaque": "疣"}
	if yyQQDisplayName(cf, "opaque", "opaque") != "疣" || yyQQDisplayName(cf, "other", "other") != "群成员" {
		t.Fatal("opaque identity leaked")
	}
}
