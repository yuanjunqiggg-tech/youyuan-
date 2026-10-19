package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func remoteSaturate(t *testing.T, admission *yyRemoteAdmission) {
	t.Helper()
	for i := 0; i < yyRemoteRequestLimit; i++ {
		if accepted, err := admission.acquire(fmt.Sprint(i)); !accepted || err != nil {
			t.Fatalf("slot %d not admitted: %v", i, err)
		}
	}
}

func remoteAssertNotSent(t *testing.T, raw []byte, id string) {
	t.Helper()
	var envelope struct {
		ID      string            `json:"id"`
		Status  int               `json:"status"`
		Body    string            `json:"body"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.ID != id || envelope.Status != 429 {
		t.Fatalf("wrong relay response: %s error=%v", raw, err)
	}
	body, err := base64.StdEncoding.DecodeString(envelope.Body)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result["trace_id"] != id || result["outcome"] != "not_sent" || result["submitted"] != false || result["retryable"] != true || result["confirmed"] != false || result["auto_retried"] != false || envelope.Headers["Content-Type"] != "application/json" || envelope.Headers["X-YouYuan-Trace-ID"] != id {
		t.Fatalf("incorrect not-sent contract: %s headers=%v", body, envelope.Headers)
	}
}

func TestRemoteAdmissionSaturationRejectsWithoutWaitingAndReleaseRestoresCapacity(t *testing.T) {
	responses := make(chan []byte, 1)
	a := yyNewRemoteAdmission(context.Background(), func(_ context.Context, b []byte) error { responses <- b; return nil }, func() {}, time.Second)
	defer a.close()
	remoteSaturate(t, a)
	if accepted, err := a.acquire("request-33"); accepted || err != nil {
		t.Fatal(accepted, err)
	}
	select {
	case raw := <-responses:
		remoteAssertNotSent(t, raw, "request-33")
	case <-time.After(time.Second):
		t.Fatal("saturation blocked rejection")
	}
	a.release()
	if accepted, err := a.acquire("replacement"); !accepted || err != nil {
		t.Fatal("released capacity did not recover", accepted, err)
	}
	if len(a.slots) != 32 {
		t.Fatal("active request cap changed", len(a.slots))
	}
}

func TestRemoteAdmissionRejectionBacklogIsBoundedAndClosesConnection(t *testing.T) {
	started := make(chan struct{})
	var closed, writes atomic.Int32
	a := yyNewRemoteAdmission(context.Background(), func(ctx context.Context, _ []byte) error {
		writes.Add(1)
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}, func() { closed.Add(1) }, time.Second)
	defer a.close()
	remoteSaturate(t, a)
	_, _ = a.acquire("writer-active")
	<-started
	for i := 0; i < 32; i++ {
		if accepted, err := a.acquire(fmt.Sprint(i)); accepted || err != nil {
			t.Fatal("bounded queue filled too early", i, accepted, err)
		}
	}
	if accepted, err := a.acquire("queue-overflow"); accepted || !errors.Is(err, errYYRemoteRejectionBacklog) {
		t.Fatal("full rejection queue did not close channel", accepted, err)
	}
	select {
	case <-a.writerEnd:
	case <-time.After(time.Second):
		t.Fatal("rejection writer leaked after backlog closure")
	}
	if closed.Load() != 1 || writes.Load() != 1 || len(a.rejected) != 32 || len(a.slots) != 32 {
		t.Fatal("unbounded rejection work or incorrect close", closed.Load(), writes.Load(), len(a.rejected), len(a.slots))
	}
}

func TestRemoteAdmissionRejectionWriteHasDeadlineAndFailureClosesOnce(t *testing.T) {
	closed := make(chan struct{}, 1)
	a := yyNewRemoteAdmission(context.Background(), func(ctx context.Context, _ []byte) error {
		deadline, exists := ctx.Deadline()
		if !exists || time.Until(deadline) > 100*time.Millisecond {
			t.Error("rejection write has no short deadline")
		}
		<-ctx.Done()
		return ctx.Err()
	}, func() { closed <- struct{}{} }, 20*time.Millisecond)
	defer a.close()
	remoteSaturate(t, a)
	_, _ = a.acquire("blocked-peer")
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("blocked rejection write did not close current channel")
	}
	if accepted, err := a.acquire("after-close"); accepted || !errors.Is(err, context.Canceled) {
		t.Fatal("unavailable channel accepted work", accepted, err)
	}
}

func TestRemoteAdmissionCancellationAndUnavailableConnectionDoNotDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var dispatched atomic.Int32
	dispatch := func(yyRelayPacket) { dispatched.Add(1) }
	if err := yyReadRemoteRequests(ctx, nil, &atomic.Int64{}, dispatch, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := yyReadRemoteRequests(context.Background(), nil, &atomic.Int64{}, dispatch, nil); err == nil {
		t.Fatal("nil websocket accepted")
	}
	a := yyNewRemoteAdmission(ctx, func(context.Context, []byte) error { t.Error("cancelled writer ran"); return nil }, func() {}, time.Second)
	if accepted, err := a.acquire("cancelled"); accepted || !errors.Is(err, context.Canceled) {
		t.Fatal(accepted, err)
	}
	a.close()
	if len(a.slots) != 0 || dispatched.Load() != 0 {
		t.Fatal("cancelled work leaked or dispatched")
	}
}

func TestRemoteClosedWebSocketStopsRejectionWriterAndReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connections, handlerDone := make(chan *websocket.Conn, 1), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		connections <- conn
		<-handlerDone
		_ = conn.CloseNow()
	}))
	defer server.Close()
	defer close(handlerDone)
	peer, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.CloseNow()
	conn := <-connections
	_ = conn.CloseNow()
	var closed, dispatched atomic.Int32
	a := yyNewRemoteAdmission(ctx, func(writeCtx context.Context, response []byte) error {
		return conn.Write(writeCtx, websocket.MessageText, response)
	}, func() { closed.Add(1); _ = conn.CloseNow() }, time.Second)
	defer a.close()
	remoteSaturate(t, a)
	_, _ = a.acquire("closed-socket")
	select {
	case <-a.writerEnd:
	case <-time.After(time.Second):
		t.Fatal("closed connection retained rejection writer")
	}
	if closed.Load() != 1 {
		t.Fatal("write failure did not request current channel close", closed.Load())
	}
	if err := yyReadRemoteRequests(ctx, conn, &atomic.Int64{}, func(yyRelayPacket) { dispatched.Add(1) }, nil); err == nil || dispatched.Load() != 0 {
		t.Fatal("closed connection dispatched work or retained reader", err, dispatched.Load())
	}
}

func TestRemoteSaturatedWebSocketKeepsPongControlAndSlowHTTPIsolated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, release := make(chan string, 64), make(chan struct{}, 32)
	var calls, rejectedWrites atomic.Int32
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("id") == "rejected-write" {
			rejectedWrites.Add(1)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		started <- r.URL.Query().Get("id")
		select {
		case <-release:
			_, _ = io.WriteString(w, `{"ok":true}`)
		case <-r.Context().Done():
		}
	}))
	defer local.Close()
	// Cancellation must precede server.Close even if an assertion fails.
	defer cancel()
	connections, handlerDone := make(chan *websocket.Conn, 1), make(chan struct{})
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		connections <- conn
		<-handlerDone
		_ = conn.CloseNow()
	}))
	defer wsServer.Close()
	defer close(handlerDone)
	peer, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.CloseNow()
	conn := <-connections
	var lastSeen, active atomic.Int64
	finished, dispatched := make(chan error, 1), make(chan string, 64)
	poolHello := make(chan string, 1)
	go func() {
		finished <- yyReadRemoteRequests(ctx, conn, &lastSeen, func(p yyRelayPacket) {
			active.Add(1)
			defer active.Add(-1)
			body, _ := base64.StdEncoding.DecodeString(p.Body)
			request, _ := http.NewRequestWithContext(ctx, p.Method, local.URL+p.Path, bytes.NewReader(body))
			response, err := local.Client().Do(request)
			if err == nil {
				_, _ = io.Copy(io.Discard, response.Body)
				response.Body.Close()
			}
			dispatched <- p.ID
		}, func(secret string) { poolHello <- secret })
	}()
	responses := make(chan []byte, 64)
	peerEnd := make(chan struct{})
	go func() {
		defer close(peerEnd)
		for {
			_, b, err := peer.Read(ctx)
			if err != nil {
				return
			}
			responses <- b
		}
	}()
	send := func(value any) {
		t.Helper()
		b, _ := json.Marshal(value)
		writeCtx, done := context.WithTimeout(ctx, time.Second)
		defer done()
		if err := peer.Write(writeCtx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 32; i++ {
		id := fmt.Sprintf("slow-%d", i)
		send(yyRelayPacket{ID: id, Method: "GET", Path: "/api/slow?id=" + id})
	}
	for i := 0; i < 32; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("slow HTTP requests did not occupy 32 slots")
		}
	}
	send(yyRelayPacket{ID: "rejected-write", Method: "POST", Path: "/api/slow?id=rejected-write", Body: base64.StdEncoding.EncodeToString([]byte("must-not-execute"))})
	select {
	case b := <-responses:
		remoteAssertNotSent(t, b, "rejected-write")
	case <-time.After(time.Second):
		t.Fatal("33rd request blocked the actual WebSocket reader")
	}
	lastSeen.Store(0)
	send(map[string]string{"kind": "pong"})
	send(map[string]string{"kind": "pool_hello", "secret": "fixture-only"})
	select {
	case <-poolHello:
	case <-time.After(time.Second):
		t.Fatal("saturated reader stopped processing subsequent control packets")
	}
	if lastSeen.Load() == 0 {
		t.Fatal("pong did not refresh heartbeat timestamp")
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, time.Second)
	defer pingCancel()
	if err := peer.Ping(pingCtx); err != nil {
		t.Fatal("saturation blocked protocol ping/pong", err)
	}
	if calls.Load() != 32 || rejectedWrites.Load() != 0 || active.Load() != 32 {
		t.Fatal("overload executed a rejected write or exceeded active limit", calls.Load(), rejectedWrites.Load(), active.Load())
	}
	release <- struct{}{}
	select {
	case <-dispatched:
	case <-time.After(time.Second):
		t.Fatal("slow request failed to release")
	}
	// The exact release/acquire transition is covered above; wait for the
	// asynchronous HTTP callback to return before a real replacement frame.
	until := time.Now().Add(time.Second)
	for active.Load() != 31 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(time.Millisecond)
	send(yyRelayPacket{ID: "replacement", Method: "GET", Path: "/api/slow?id=replacement"})
	select {
	case id := <-started:
		if id != "replacement" {
			t.Fatal("wrong request resumed", id)
		}
	case <-time.After(time.Second):
		t.Fatal("released WebSocket capacity did not accept replacement")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("cancelled read loop did not exit")
	}
	select {
	case <-peerEnd:
	case <-time.After(time.Second):
		t.Fatal("cancelled peer reader leaked")
	}
	until = time.Now().Add(time.Second)
	for active.Load() != 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if active.Load() != 0 {
		t.Fatal("cancelled local HTTP work leaked", active.Load())
	}
}
