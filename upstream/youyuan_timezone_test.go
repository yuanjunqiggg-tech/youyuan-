package main

import (
	"strings"
	"testing"
	"time"
)

func TestChatLogHistoricalUTCDisplaysBeijing(t *testing.T) {
	oldZone := time.Local
	t.Setenv("YY_TIMEZONE", "Asia/Shanghai")
	t.Setenv("TZ", "UTC")
	yyInitTimeZone()
	defer func() { time.Local = oldZone }()
	yyObservation.Lock()
	old := yyObservation.Chat
	yyObservation.Chat = []yyChatRecord{{Time: "2026-10-01T02:34:56Z", Player: "p", Message: "old UTC"}, {Time: "2026-10-01T10:35:56+08:00", Player: "p", Message: "offset already present"}}
	yyObservation.Unlock()
	defer func() { yyObservation.Lock(); yyObservation.Chat = old; yyObservation.Unlock() }()
	logs := yyQQChatLogs(2)
	if !strings.Contains(logs, "10-01 10:34:56") || !strings.Contains(logs, "10-01 10:35:56") || strings.Contains(logs, "18:35:56") {
		t.Fatal(logs)
	}
}
