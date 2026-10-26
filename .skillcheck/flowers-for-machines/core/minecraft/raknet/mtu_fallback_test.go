package raknet

import (
	"context"
	"errors"
	"net"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/raknet/internal/message"
)

func fixtureMTUError() error {
	if runtime.GOOS == "windows" {
		return syscall.Errno(10040)
	}
	return syscall.EMSGSIZE
}

type mtuFixture struct {
	mu      sync.Mutex
	reads   int
	packet  []byte
	failure error
	closed  bool
}

func (m *mtuFixture) Read(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads++
	if m.reads == 1 || m.failure != nil {
		if m.failure != nil {
			return 0, m.failure
		}
		return 0, &net.OpError{Op: "read", Net: "udp", Err: fixtureMTUError()}
	}
	return copy(b, m.packet), nil
}
func (*mtuFixture) Write(b []byte) (int, error)      { return len(b), nil }
func (m *mtuFixture) Close() error                   { m.mu.Lock(); defer m.mu.Unlock(); m.closed = true; return nil }
func (*mtuFixture) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (*mtuFixture) RemoteAddr() net.Addr             { return &net.UDPAddr{} }
func (*mtuFixture) SetDeadline(time.Time) error      { return nil }
func (*mtuFixture) SetReadDeadline(time.Time) error  { return nil }
func (*mtuFixture) SetWriteDeadline(time.Time) error { return nil }

func TestMTUDiscoveryRecoversPathMTUSignal(t *testing.T) {
	b, _ := (&message.OpenConnectionReply1{ServerGUID: 123, MTU: 1200}).MarshalBinary()
	c := &mtuFixture{packet: b}
	timer := time.NewTicker(time.Hour)
	defer timer.Stop()
	s := connState{conn: c, ticker: timer, maxTransientErrors: 2}
	if err := s.discoverMTU(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.mtu != 1200 || s.transientErrorCount != 1 || c.closed {
		t.Fatalf("fallback failed: mtu=%d retries=%d closed=%v", s.mtu, s.transientErrorCount, c.closed)
	}
	if isTransientUDPReadError(fixtureMTUError()) {
		t.Fatal("MTU signal must not be ignored outside discovery")
	}
}

func TestMTUDiscoveryHasBoundedRetries(t *testing.T) {
	c := &mtuFixture{failure: fixtureMTUError()}
	timer := time.NewTicker(time.Hour)
	defer timer.Stop()
	s := connState{conn: c, ticker: timer, maxTransientErrors: 2}
	if err := s.discoverMTU(context.Background()); !errors.Is(err, fixtureMTUError()) {
		t.Fatalf("got %v", err)
	}
	if c.reads != 3 || !c.closed {
		t.Fatalf("not bounded: reads=%d closed=%v", c.reads, c.closed)
	}
}
