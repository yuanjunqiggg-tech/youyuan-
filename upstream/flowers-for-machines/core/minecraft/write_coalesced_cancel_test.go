package minecraft

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestModuleCancellationWhileWaitingForSharedTransport(t *testing.T) {
	c := coalescedFixture()
	c.sendMu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	type result struct {
		attempted bool
		err       error
	}
	done := make(chan result, 1)
	go func() { a, e := c.WritePacketsBounded(ctx, coalescedPackets(2)); done <- result{a, e} }()
	select {
	case got := <-done:
		c.sendMu.Unlock()
		if got.attempted || !errors.Is(got.err, context.DeadlineExceeded) || len(c.bufferedSend) != 0 {
			t.Fatal("canceled waiter submitted or lost cancellation", got, len(c.bufferedSend))
		}
	case <-time.After(200 * time.Millisecond):
		c.sendMu.Unlock()
		<-done
		t.Fatal("command module cancellation waits for another writer to release the transport")
	}
}

func TestModuleConnectionClosureWhileWaitingForSharedTransport(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	c := coalescedFixture()
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	c.ctx, c.conn = ctx, a
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	closed := errors.New("peer closed during shared writer wait")
	done := make(chan error, 1)
	go func() { _, err := c.WritePacketsBounded(context.Background(), coalescedPackets(2)); done <- err }()
	cancel(closed)
	select {
	case err := <-done:
		if !errors.Is(err, closed) || len(c.bufferedSend) != 0 {
			t.Fatal("connection closure lost or sent after closure", err, len(c.bufferedSend))
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("closed connection kept module waiting for another writer")
	}
}

func TestModuleResumesAfterSharedTransportIsReleased(t *testing.T) {
	c := coalescedFixture()
	c.sendMu.Lock()
	done := make(chan error, 1)
	go func() { _, err := c.WritePacketsBounded(context.Background(), coalescedPackets(2)); done <- err }()
	select {
	case err := <-done:
		c.sendMu.Unlock()
		t.Fatal("module passed a held send lock", err)
	case <-time.After(10 * time.Millisecond):
	}
	c.sendMu.Unlock()
	select {
	case err := <-done:
		if err != nil || len(c.bufferedSend) != 2 || !c.sendMu.TryLock() {
			t.Fatal("module failed to resume or leaked send lock", err, len(c.bufferedSend))
		}
		c.sendMu.Unlock()
	case <-time.After(time.Second):
		t.Fatal("module did not resume after writer finished")
	}
}
