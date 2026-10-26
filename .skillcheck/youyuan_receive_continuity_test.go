package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"testing"
	"time"
)

// Exercise the receive loop itself, not a direct call to the Lua chat handler.
func TestReceiveChatContinuesWhenTerminalQueueIsFull(t *testing.T) {
	bm := &BotManager{stopCh: make(chan struct{}), pktCh: make(chan packet.Packet, 4), logCh: make(chan string, 1)}
	bm.logCh <- "terminal backlog"
	c := &captureHook{}
	old := getSendTargetedHook()
	setSendTargetedHook(func(p, s string) error { c.add(s); return nil })
	defer setSendTargetedHook(old)
	const id = "receive_backlog_probe"
	if err := LoadLuaPlugin(bm, id, `function observe(p,m) game.sendTargeted(p,"seen:"..m) end events.onChat("observe")`); err != nil {
		t.Fatal(err)
	}
	defer RemovePluginEvents(id)
	done := make(chan struct{})
	go func() { bm.consumePackets(); close(done) }()
	defer func() {
		close(bm.stopCh)
		// Unblock the old implementation too, so a failing regression test leaks no goroutine.
		select {
		case <-bm.logCh:
		default:
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("receive loop failed to stop")
		}
	}()
	bm.pktCh <- &packet.Text{TextType: packet.TextTypeChat, SourceName: "ReceiveProbe", Message: ".help"}
	// Simulate a malformed decoded packet between valid messages. Recovery must
	// happen per packet, not at the goroutine boundary.
	bm.pktCh <- (*packet.Text)(nil)
	bm.pktCh <- &packet.Text{TextType: packet.TextTypeChat, SourceName: "ReceiveProbe", Message: "second"}
	deadline := time.Now().Add(time.Second)
	for c.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if c.count() != 2 {
		t.Fatalf("terminal backlog suppressed plugin chat delivery: %v", c.all())
	}
}
