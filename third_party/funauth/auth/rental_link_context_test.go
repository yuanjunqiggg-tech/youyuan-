package auth

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

type gameStartLinkFixture struct {
	conn    net.Conn
	calls   atomic.Int32
	closed  atomic.Bool
	write   bool
	result  error
	entered chan struct{}
}

func (f *gameStartLinkFixture) Conn() net.Conn { return f.conn }
func (f *gameStartLinkFixture) Close() error   { f.closed.Store(true); return f.conn.Close() }
func (f *gameStartLinkFixture) SendGameStart(map[string]interface{}) error {
	f.calls.Add(1)
	if f.entered != nil {
		close(f.entered)
	}
	if f.write {
		_, err := f.conn.Write([]byte("fixture-game-start"))
		return err
	}
	return f.result
}

func TestRentalGameStartCancellationClosesBlockedWrite(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	f := &gameStartLinkFixture{conn: local, write: true, entered: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- sendRentalGameStart(ctx, f, map[string]interface{}{"fixture": true}) }()
	<-f.entered
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("GameStart blocked after cancellation")
	}
	if f.calls.Load() != 1 || !f.closed.Load() {
		t.Fatal("GameStart replayed or connection leaked")
	}
}

func TestRentalGameStartUnknownWriteAndSuccessNeverReplay(t *testing.T) {
	for _, writeErr := range []error{nil, errors.New("fixture unknown write outcome")} {
		local, peer := net.Pipe()
		f := &gameStartLinkFixture{conn: local, result: writeErr}
		err := sendRentalGameStart(context.Background(), f, nil)
		peer.Close()
		if !errors.Is(err, writeErr) || f.calls.Load() != 1 || !f.closed.Load() {
			t.Fatalf("err=%v calls=%d closed=%v", err, f.calls.Load(), f.closed.Load())
		}
	}
}

func TestRentalGameStartAlreadyCanceledDoesNotSend(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	f := &gameStartLinkFixture{conn: local}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sendRentalGameStart(ctx, f, nil)
	if !errors.Is(err, context.Canceled) || f.calls.Load() != 0 || !f.closed.Load() {
		t.Fatalf("err=%v calls=%d", err, f.calls.Load())
	}
}
