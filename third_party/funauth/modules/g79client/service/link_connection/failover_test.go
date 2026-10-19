package link_connection

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"syscall"
	"testing"
	"time"

	g79 "github.com/Yeah114/g79client"
)

func endpointFixture(ip, typ string, port int) g79.G79LinkServerEntry {
	return g79.G79LinkServerEntry{IP: ip, ServerType: typ, Port: g79.Uncertain{Raw: strconv.Itoa(port)}}
}
func refusedFixture() error {
	return &linkTCPDialError{&net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}}
}

func TestCandidatePlanDistinctInterleavedAndBounded(t *testing.T) {
	a := endpointFixture("127.0.0.1", "A", 10200)
	b := endpointFixture("127.0.0.1", "B", 10201)
	list := []g79.G79LinkServerEntry{a, a, b, endpointFixture("127.0.0.2", "A", 10200), endpointFixture("127.0.0.2", "B", 10201), endpointFixture("", "A", 0)}
	for seed := int64(0); seed < 40; seed++ {
		s := &LinkConnectionService{rnd: rand.New(rand.NewSource(seed))}
		plan := s.candidatePlan(list)
		if len(plan) != 3 || plan[0].ServerType == plan[1].ServerType {
			t.Fatalf("seed %d invalid interleave: %#v", seed, plan)
		}
		seen := map[string]bool{}
		for _, e := range plan {
			key := net.JoinHostPort(e.IP, e.Port.String())
			if seen[key] {
				t.Fatalf("duplicate %s", key)
			}
			seen[key] = true
		}
	}
}

func TestFailoverRefusalThenSuccessStops(t *testing.T) {
	plan := []g79.G79LinkServerEntry{endpointFixture("127.0.0.1", "A", 1), endpointFixture("127.0.0.1", "B", 2), endpointFixture("127.0.0.1", "A", 3)}
	calls := 0
	wanted := &LinkConnection{}
	got, err := dialLinkCandidates(context.Background(), plan, func(ctx context.Context, e *LinkEndpoint) (*LinkConnection, error) {
		calls++
		if calls == 1 {
			return nil, refusedFixture()
		}
		return wanted, nil
	})
	if err != nil || got != wanted || calls != 2 {
		t.Fatalf("got=%p err=%v calls=%d", got, err, calls)
	}
}

func TestFailoverOnlyTCPRefusalAndAtMostThree(t *testing.T) {
	plan := make([]g79.G79LinkServerEntry, 7)
	for _, err := range []error{context.DeadlineExceeded, errors.New("LoginV2 rejected"), &net.OpError{Op: "write", Net: "tcp", Err: syscall.ECONNREFUSED}, &linkTCPDialError{&net.OpError{Op: "dial", Net: "tcp", Err: syscall.ETIMEDOUT}}} {
		calls := 0
		_, got := dialLinkCandidates(context.Background(), plan, func(context.Context, *LinkEndpoint) (*LinkConnection, error) { calls++; return nil, err })
		if !errors.Is(got, err) || calls != 1 {
			t.Fatalf("unsafe retry: %v calls=%d", got, calls)
		}
	}
	calls := 0
	_, err := dialLinkCandidates(context.Background(), plan, func(context.Context, *LinkEndpoint) (*LinkConnection, error) { calls++; return nil, refusedFixture() })
	if !errors.Is(err, syscall.ECONNREFUSED) || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestFailoverCancellationPreventsNextDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := dialLinkCandidates(ctx, make([]g79.G79LinkServerEntry, 3), func(context.Context, *LinkEndpoint) (*LinkConnection, error) {
		calls++
		cancel()
		return nil, refusedFixture()
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestDialEndpointCancellationInterruptsHandshakeWrite(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	entered := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &LinkConnectionService{}
	ctx = WithDialer(ctx, func(context.Context, string, string) (net.Conn, error) { close(entered); return local, nil })
	done := make(chan error, 1)
	go func() {
		_, err := s.DialEndpoint(ctx, &LinkEndpoint{Entry: endpointFixture("127.0.0.1", "A", 10200)})
		done <- err
	}()
	<-entered
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handshake write leaked after cancellation")
	}
}

func TestActualRefusedSocketIsSafeFailover(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &LinkConnectionService{}
	_, err = s.DialEndpoint(ctx, &LinkEndpoint{Entry: endpointFixture("127.0.0.1", "A", addr.Port)})
	var stage *linkTCPDialError
	if !errors.As(err, &stage) || !isConnectionRefused(err) {
		t.Fatal(fmt.Sprintf("expected classified TCP refusal, got %v", err))
	}
}

func TestDialEndpointCancellationReleasesLoginWait(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	ready, peerDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(peerDone)
		_, _ = io.ReadFull(peer, make([]byte, 272))
		close(ready)
		_, _ = io.Copy(io.Discard, peer)
	}()
	s := &LinkConnectionService{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = WithDialer(ctx, func(context.Context, string, string) (net.Conn, error) { return local, nil })
	done := make(chan error, 1)
	go func() {
		_, err := s.DialEndpoint(ctx, &LinkEndpoint{Entry: endpointFixture("127.0.0.1", "A", 10200)})
		done <- err
	}()
	<-ready
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("login wait leaked")
	}
	select {
	case <-peerDone:
	case <-time.After(time.Second):
		t.Fatal("socket not closed after cancelled login wait")
	}
}

func TestDialDoesNotFailOverBadOfficialHandshake(t *testing.T) {
	calls := 0
	peerDone := make(chan struct{})
	s := &LinkConnectionService{rnd: rand.New(rand.NewSource(1)), servers: []g79.G79LinkServerEntry{endpointFixture("127.0.0.1", "A", 10200), endpointFixture("127.0.0.1", "B", 10201)}}
	fakeDial := func(context.Context, string, string) (net.Conn, error) {
		calls++
		local, peer := net.Pipe()
		go func() {
			defer close(peerDone)
			defer peer.Close()
			_, _ = io.ReadFull(peer, make([]byte, 272))
			_, _ = peer.Write(make([]byte, 256))
		}()
		return local, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := s.Dial(WithDialer(ctx, fakeDial))
	if err == nil || calls != 1 {
		t.Fatalf("handshake must be required and must not trigger another endpoint: calls=%d err=%v", calls, err)
	}
	select {
	case <-peerDone:
	case <-time.After(time.Second):
		t.Fatal("handshake fixture leaked")
	}
}

func TestWithDialerIsRequestScopedAcrossDerivedContexts(t *testing.T) {
	type requestKey struct{}
	s := &LinkConnectionService{rnd: rand.New(rand.NewSource(8)), servers: []g79.G79LinkServerEntry{
		endpointFixture("127.0.0.1", "A", 10200), endpointFixture("127.0.0.1", "B", 10201), endpointFixture("127.0.0.2", "A", 10200),
	}}
	results := make(chan error, 2)
	for id := 0; id < 2; id++ {
		go func(id int) {
			calls, seen := 0, map[string]bool{}
			ctx := WithDialer(context.WithValue(context.Background(), requestKey{}, id), func(ctx context.Context, network, address string) (net.Conn, error) {
				calls++
				if ctx.Value(requestKey{}) != id || network != "tcp" || seen[address] {
					return nil, errors.New("request dialer crossed context or repeated endpoint")
				}
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 10*time.Second {
					return nil, errors.New("transport lost login deadline")
				}
				seen[address] = true
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
			})
			_, err := s.Dial(ctx)
			if calls != 3 || !isConnectionRefused(err) {
				results <- fmt.Errorf("request %d calls=%d err=%v", id, calls, err)
				return
			}
			results <- nil
		}(id)
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("request-scoped transport blocked")
		}
	}
}
