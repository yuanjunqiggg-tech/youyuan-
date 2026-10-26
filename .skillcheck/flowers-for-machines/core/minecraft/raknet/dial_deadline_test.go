package raknet

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/raknet/internal"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/raknet/internal/message"
)

// Real pipe reads exercise expired deadlines; writes model non-blocking UDP datagrams.
type deadlineReadFixture struct {
	net.Conn
	reads      atomic.Int64
	mu         sync.Mutex
	deadline   time.Time
	readErrors []error
}

func (c *deadlineReadFixture) Read(b []byte) (int, error) {
	call := c.reads.Add(1)
	if call <= int64(len(c.readErrors)) {
		return 0, c.readErrors[call-1]
	}
	n, err := c.Conn.Read(b)
	if errors.Is(err, io.ErrClosedPipe) {
		return n, net.ErrClosed
	}
	return n, err
}
func (*deadlineReadFixture) Write(b []byte) (int, error) { return len(b), nil }
func (*deadlineReadFixture) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19132}
}
func (c *deadlineReadFixture) SetDeadline(deadline time.Time) error {
	c.mu.Lock()
	c.deadline = deadline
	c.mu.Unlock()
	return c.Conn.SetDeadline(deadline)
}

func deadlineFixture(t *testing.T) (*deadlineReadFixture, net.Conn, Dialer) {
	t.Helper()
	a, b := net.Pipe()
	c := &deadlineReadFixture{Conn: a}
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	return c, b, Dialer{ErrorLog: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestClientListenDeadlineStopsAfterOneRead(t *testing.T) {
	raw, _, dialer := deadlineFixture(t)
	if err := raw.SetDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	conn := newConn(internal.ConnToPacketConn(raw), raw.RemoteAddr(), 1200, dialerConnectionHandler{l: dialer.ErrorLog})
	defer conn.closeImmediately()
	done := make(chan struct{})
	go func() { dialer.clientListen(conn, raw); close(done) }()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		_ = raw.Close()
		<-done
		t.Fatalf("expired read deadline kept looping: reads=%d", raw.reads.Load())
	}
	if reads := raw.reads.Load(); reads != 1 {
		t.Fatalf("expired read retried %d times", reads)
	}
	select {
	case <-conn.Context().Done():
	default:
		t.Fatal("terminal deadline left the RakNet connection alive")
	}
}

func TestConnectReturnsSocketDeadlineBeforeParentDeadline(t *testing.T) {
	raw, _, dialer := deadlineFixture(t)
	if err := raw.SetDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := dialer.connect(ctx, &connState{conn: raw, raddr: raw.RemoteAddr(), mtu: 1200})
		done <- err
	}()
	select {
	case err := <-done:
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() || ctx.Err() != nil {
			t.Fatalf("lost socket deadline or waited for parent: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		cancel()
		_ = raw.Close()
		<-done
		t.Fatal("failed reader did not wake final connect")
	}
	if raw.reads.Load() != 1 {
		t.Fatalf("socket timeout caused busy reads: %d", raw.reads.Load())
	}
}

func TestConnectContextDeadlineClosesBlockedReader(t *testing.T) {
	raw, _, dialer := deadlineFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := dialer.connect(ctx, &connState{conn: raw, raddr: raw.RemoteAddr(), mtu: 1200})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("context deadline lost: %v", err)
	}
	if _, err := raw.Conn.Read(make([]byte, 1)); !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("timed out dial left underlying read open: %v", err)
	}
}

func TestConnectSuccessClearsDeadlineAndOutlivesDialContext(t *testing.T) {
	raw, peer, dialer := deadlineFixture(t)
	if err := raw.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	addr := netip.MustParseAddrPort("127.0.0.1:19132")
	accepted := &message.ConnectionRequestAccepted{ClientAddress: addr}
	for i := range accepted.SystemAddresses {
		accepted.SystemAddresses[i] = addr
	}
	payload, err := accepted.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	wire := bytes.NewBuffer([]byte{bitFlagDatagram, 0, 0, 0})
	(&packet{reliability: reliabilityReliableOrdered, content: payload}).write(wire)
	writeDone := make(chan error, 1)
	go func() { _, err := peer.Write(wire.Bytes()); writeDone <- err }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := dialer.connect(ctx, &connState{conn: raw, raddr: raw.RemoteAddr(), mtu: 1200})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.closeImmediately()
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	raw.mu.Lock()
	cleared := raw.deadline.IsZero()
	raw.mu.Unlock()
	if !cleared {
		t.Fatal("successful connection kept the handshake socket deadline")
	}
	cancel()
	select {
	case <-conn.Context().Done():
		t.Fatal("successful session inherited the dial context cancellation")
	default:
	}
}

func fixtureConnectionRefused() syscall.Errno {
	if runtime.GOOS == "windows" {
		return syscall.Errno(10061)
	}
	return syscall.ECONNREFUSED
}

func TestClientListenRefusedStopsAfterOneWrappedRead(t *testing.T) {
	raw, _, dialer := deadlineFixture(t)
	refused := fixtureConnectionRefused()
	raw.readErrors = []error{&net.OpError{Op: "read", Net: "udp", Err: os.NewSyscallError("read", refused)}}
	conn := newConn(internal.ConnToPacketConn(raw), raw.RemoteAddr(), 1200, dialerConnectionHandler{l: dialer.ErrorLog})
	defer conn.closeImmediately()
	done := make(chan error, 1)
	go func() { done <- dialer.clientListen(conn, raw) }()
	select {
	case err := <-done:
		if !errors.Is(err, refused) || raw.reads.Load() != 1 || conn.Context().Err() == nil {
			t.Fatalf("refused read survived: reads=%d context=%v err=%v", raw.reads.Load(), conn.Context().Err(), err)
		}
	case <-time.After(300 * time.Millisecond):
		_ = raw.Close()
		<-done
		t.Fatalf("refused socket kept reading: %d reads", raw.reads.Load())
	}
}

func TestConnectRefusedWakesBeforeParentDeadline(t *testing.T) {
	raw, _, dialer := deadlineFixture(t)
	refused := fixtureConnectionRefused()
	raw.readErrors = []error{&net.OpError{Op: "read", Net: "udp", Err: os.NewSyscallError("read", refused)}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := dialer.connect(ctx, &connState{conn: raw, raddr: raw.RemoteAddr(), mtu: 1200})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, refused) || ctx.Err() != nil || raw.reads.Load() != 1 {
			t.Fatalf("refused dial did not stop promptly: reads=%d err=%v", raw.reads.Load(), err)
		}
	case <-time.After(300 * time.Millisecond):
		cancel()
		_ = raw.Close()
		<-done
		t.Fatal("refused reader did not wake connect")
	}
}

func TestClientListenKeepsOtherRetryableReadErrors(t *testing.T) {
	errnos := []syscall.Errno{syscall.EINTR, syscall.EHOSTUNREACH, syscall.ECONNRESET}
	if runtime.GOOS == "windows" {
		errnos = append(errnos, syscall.Errno(10065), syscall.Errno(10054))
	}
	for _, errno := range errnos {
		t.Run(errno.Error(), func(t *testing.T) {
			raw, peer, dialer := deadlineFixture(t)
			raw.readErrors = []error{&net.OpError{Op: "read", Net: "udp", Err: os.NewSyscallError("read", errno)}}
			conn := newConn(internal.ConnToPacketConn(raw), raw.RemoteAddr(), 1200, dialerConnectionHandler{l: dialer.ErrorLog})
			defer conn.closeImmediately()
			done := make(chan error, 1)
			go func() { done <- dialer.clientListen(conn, raw) }()
			written := make(chan error, 1)
			go func() { _, err := peer.Write([]byte{0}); written <- err }()
			select {
			case err := <-written:
				if err != nil || conn.Context().Err() != nil || raw.reads.Load() < 2 {
					t.Fatalf("retryable read killed connection: errno=%v context=%v err=%v", errno, conn.Context().Err(), err)
				}
			case <-time.After(300 * time.Millisecond):
				_ = peer.Close()
				<-written
				t.Fatal("retryable read never resumed")
			}
			_ = raw.Close()
			<-done
		})
	}
}
