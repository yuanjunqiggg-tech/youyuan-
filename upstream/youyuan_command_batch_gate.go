package main

import "context"

// Blocked senders queue instead of repeatedly racing a hot poller for TryLock.
func (bm *BotManager) acquireControlBatch(ctx context.Context) (func(), error) {
	bm.controlBatchOnce.Do(func() { bm.controlBatchSlot = make(chan struct{}, 1) })
	select {
	case bm.controlBatchSlot <- struct{}{}:
		release := func() { <-bm.controlBatchSlot }
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		return release, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Serialize the write only: a missing reply must not reserve the account for seconds.
func (bm *BotManager) writeControlBatch(ctx context.Context, write func() error) error {
	release, err := bm.acquireControlBatch(ctx)
	if err != nil {
		return err
	}
	defer release()
	return write()
}
