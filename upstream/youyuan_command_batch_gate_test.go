package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCommandBatchCanceledWaiterDoesNotHoldOrStealSlot(t *testing.T) {
	bm := &BotManager{}
	release, err := bm.acquireControlBatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		release, err := bm.acquireControlBatch(ctx)
		if release != nil {
			release()
		}
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked behind a live batch")
	}
	if len(bm.controlBatchSlot) != 1 {
		t.Fatal("waiter released the owner's slot")
	}
	release()
	for i := 0; i < 100; i++ {
		if release, err := bm.acquireControlBatch(ctx); !errors.Is(err, context.Canceled) || release != nil {
			t.Fatal("canceled caller acquired slot", err)
		}
	}
	if len(bm.controlBatchSlot) != 0 {
		t.Fatal("canceled acquisition leaked the slot")
	}
	release, err = bm.acquireControlBatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestCommandBatchContentionKeepsSingleOwnerAndAllWaitersProgress(t *testing.T) {
	bm := &BotManager{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	var owners, completed atomic.Int32
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				release, err := bm.acquireControlBatch(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				if owners.Add(1) != 1 {
					t.Error("batches overlapped")
				}
				time.Sleep(10 * time.Microsecond)
				owners.Add(-1)
				completed.Add(1)
				release()
			}
		}()
	}
	wg.Wait()
	if completed.Load() != 320 || owners.Load() != 0 || len(bm.controlBatchSlot) != 0 {
		t.Fatal("batch stalled or leaked", completed.Load(), owners.Load())
	}
}

func TestCommandBatchMissingReplyDoesNotBlockAnotherCaller(t *testing.T) {
	bm := &BotManager{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sent, reply, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer func() { close(reply); <-finished }()
	go func() {
		defer close(finished)
		yyExecuteWindowed([]string{"slow"}, 1, func(string) (func() (map[string]any, error), error) {
			err := bm.writeControlBatch(ctx, func() error { close(sent); return nil })
			return func() (map[string]any, error) {
				<-reply
				return map[string]any{"confirmed": true, "success": true}, nil
			}, err
		})
	}()
	<-sent
	if err := bm.writeControlBatch(ctx, func() error { return nil }); err != nil {
		t.Fatal("another query waited behind an unrelated command reply", err)
	}
	select {
	case <-finished:
		t.Fatal("slow caller fabricated a reply")
	default:
	}
	if len(bm.controlBatchSlot) != 0 {
		t.Fatal("completed write retained the account slot")
	}
}
