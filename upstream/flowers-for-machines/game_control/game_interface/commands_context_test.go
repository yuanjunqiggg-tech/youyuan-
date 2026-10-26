package game_interface

import (
	"context"
	"errors"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"testing"
	"time"
)

func TestCommandReceiptCancellationDoesNotWaitForTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, timedOut, err := waitCommandReply(ctx, make(chan commandReply, 1), "test", 0, time.Hour)
		if timedOut {
			result <- errors.New("cancel reported as command timeout")
			return
		}
		result <- err
	}()
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel still waited for receipt timeout")
	}
	if _, _, err := (*Commands)(nil).SendWSCommandWithContext(ctx, "test", time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal("already cancelled request attempted I/O", err)
	}
}

func TestCommandReceiptPreservesSuccessAndTimeout(t *testing.T) {
	ch := make(chan commandReply, 1)
	want := &packet.CommandOutput{SuccessCount: 1}
	ch <- commandReply{output: want}
	if got, timedOut, err := waitCommandReply(context.Background(), ch, "test", 0, -1); got != want || timedOut || err != nil {
		t.Fatalf("receipt lost: %v %v %v", got, timedOut, err)
	}
	if _, timedOut, err := waitCommandReply(context.Background(), ch, "test", 0, time.Millisecond); !timedOut || err == nil {
		t.Fatal("missing receipt was not an unknown timeout")
	}
}
