package resources_control

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/google/uuid"
)

func TestCommandReplyRetiresBeforeCallbackRuns(t *testing.T) {
	c := NewCommandRequestCallback(context.Background())
	release := make(chan struct{})
	defer close(release)
	id := uuid.New()
	c.SetCommandRequestCallback(id, func(*packet.CommandOutput, error) { <-release })
	c.onCommandOutput(&packet.CommandOutput{CommandOrigin: protocol.CommandOrigin{UUID: id}})
	c.mu.Lock()
	remaining := len(c.callback)
	c.mu.Unlock()
	if remaining != 0 {
		t.Fatal("completed command retained its registration until the waiter runs", remaining)
	}
}

func TestCommandTenThousandRepliesDoNotRetainOrReplayCallbacks(t *testing.T) {
	const n = 10000
	c := NewCommandRequestCallback(context.Background())
	ids := make([]uuid.UUID, n)
	done := make(chan struct{}, 2*n)
	var calls atomic.Int64
	for i := range ids {
		ids[i] = uuid.New()
		c.SetCommandRequestCallback(ids[i], func(*packet.CommandOutput, error) {
			calls.Add(1)
			done <- struct{}{}
		})
	}
	for _, id := range ids {
		p := &packet.CommandOutput{CommandOrigin: protocol.CommandOrigin{UUID: id}}
		c.onCommandOutput(p)
		c.onCommandOutput(p)
	}
	c.mu.Lock()
	remaining := len(c.callback)
	c.mu.Unlock()
	if remaining != 0 {
		t.Fatal("completed receipts accumulated", remaining)
	}
	c.handleConnClose(errors.New("test closure after replies"))
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for i := 0; i < n; i++ {
		select {
		case <-done:
		case <-timer.C:
			t.Fatal("receipt delivery stalled", i)
		}
	}
	if calls.Load() != n {
		t.Fatal("a duplicate receipt or closure replayed a completed callback", calls.Load())
	}
}
