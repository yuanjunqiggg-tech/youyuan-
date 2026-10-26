package main

import (
	"testing"
	"time"
)

func TestRichChatBackpressureReleasesHostVM(t *testing.T) {
	bm := &BotManager{}
	const id = "rich-chat-host-pressure"
	if err := LoadLuaPlugin(bm, id, `n=0;function send()game.sendRichTellraw('probe')end;function event(e)n=n+1 end;function read()return n end`); err != nil {
		t.Fatal(err)
	}
	r, _ := yyGetResident(bm, id)
	started, release := make(chan struct{}), make(chan struct{})
	old := getSendTargetedHook()
	setSendTargetedHook(func(string, string) error { close(started); <-release; return nil })
	done := make(chan error, 1)
	defer func() { close(release); <-done; setSendTargetedHook(old); yyStopResident(bm, id) }()
	go func() { _, err := yyCallResident(bm, id, "send", nil, false, false); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("rich chat never reached transport")
	}
	yyQueueSenseLua(r, bm, id, "event", []any{map[string]any{"type": "health"}})
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		value, err := yyCallResident(bm, id, "read", nil, false, true)
		if err == nil && value == float64(1) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("rich chat transport wait held the Lua VM and blocked health callbacks")
}
