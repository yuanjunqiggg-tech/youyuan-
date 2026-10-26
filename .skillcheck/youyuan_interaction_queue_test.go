package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestQueuedChatIOCannotBlockEntityFIFO(t *testing.T) {
	bm := &BotManager{}
	const id = "chat-io-entity-fifo"
	if err := LoadLuaPlugin(bm, id, `n=0;function read()return n end;function chat(p,m)return slow()end;function entity(e)assert(e.seq==n+1);n=e.seq end;events.onChat('chat')`); err != nil {
		t.Fatal(err)
	}
	r, err := yyGetResident(bm, id)
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	defer func() { close(release); yyStopResident(bm, id) }()
	r.mu.Lock()
	r.L.SetGlobal("slow", r.L.NewFunction(func(L *lua.LState) int {
		_, err := yyLuaWaitValue(L, func(ctx context.Context) (bool, error) {
			close(started)
			select {
			case <-release:
				return true, nil
			case <-ctx.Done():
				return false, ctx.Err()
			}
		})
		if err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))
	r.mu.Unlock()
	fireChatEvent(bm, "test-player", "test-interaction")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("chat did not enter I/O")
	}
	for i := 1; i <= 200; i++ {
		yyQueueResidentEvent(bm, id, "entity", []any{map[string]any{"seq": i}})
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		v, err := yyCallResident(bm, id, "read", nil, false, true)
		if err == nil && v == float64(200) {
			if r.senseDropped.Load() != 0 || r.callbackFailures.Load() != 0 {
				t.Fatal("lost events or changed entity ordering")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(fmt.Sprintf("a chat waiting for I/O blocked ordinary entity callbacks; drops=%d", r.senseDropped.Load()))
}

func TestNativeTextAndPriorityEventsUseSeparateLanes(t *testing.T) {
	r := &yyLuaResident{senseQueue: make(chan yySenseDelivery, 1), priorityQueue: make(chan yySenseDelivery, 1), interactionQueue: make(chan yySenseDelivery, 1)}
	for _, event := range []map[string]any{{"_packet_type": "Text"}, {"packet_type": "Text"}, {"type": "text"}, {"type": "chat"}} {
		if yySenseDeliveryQueue(r, []any{event}) != r.interactionQueue {
			t.Fatalf("text event not isolated: %v", event)
		}
	}
	if yySenseDeliveryQueue(r, []any{"player", "message"}, true) != r.interactionQueue {
		t.Fatal("legacy onChat arguments lost their lane")
	}
	if yySenseDeliveryQueue(r, []any{map[string]any{"type": "hurt"}}) != r.priorityQueue || yySenseDeliveryQueue(r, []any{map[string]any{"_packet_type": "AddActor"}}) != r.senseQueue {
		t.Fatal("chat isolation changed ordinary or priority routing")
	}
}
