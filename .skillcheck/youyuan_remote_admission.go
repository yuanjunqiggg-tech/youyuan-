package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type yyRelayPacket struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

const yyRemoteRequestLimit = 32
const yyRemoteRejectionTimeout = 2 * time.Second

var errYYRemoteRejectionBacklog = errors.New("remote rejection response queue full")

// One bounded response queue and one writer per connection. The WebSocket
// reader must never wait for a slow local request or a slow rejection write.
type yyRemoteAdmission struct {
	ctx       context.Context
	cancel    context.CancelFunc
	slots     chan struct{}
	rejected  chan []byte
	writerEnd chan struct{}
	write     func(context.Context, []byte) error
	closeConn func()
	closeOnce sync.Once
	timeout   time.Duration
}

func yyNewRemoteAdmission(ctx context.Context, write func(context.Context, []byte) error, closeConn func(), timeout time.Duration) *yyRemoteAdmission {
	ctx, cancel := context.WithCancel(ctx)
	a := &yyRemoteAdmission{ctx: ctx, cancel: cancel, slots: make(chan struct{}, yyRemoteRequestLimit), rejected: make(chan []byte, yyRemoteRequestLimit), writerEnd: make(chan struct{}), write: write, closeConn: closeConn, timeout: timeout}
	go a.writeRejections()
	return a
}

func yyRemoteNotSentResponse(id string) []byte {
	traceID := id
	if traceID == "" {
		traceID = uuid.NewString()
	}
	body, _ := json.Marshal(map[string]any{
		"ok": false, "error": "设备请求已满，请稍后重试", "code": "REMOTE_REQUEST_LIMIT",
		"error_code": "not_sent", "trace_id": traceID, "submitted": false,
		"confirmed": false, "retryable": true, "auto_retried": false, "outcome": "not_sent",
	})
	response, _ := json.Marshal(map[string]any{"id": id, "status": http.StatusTooManyRequests,
		"headers": map[string]string{"Content-Type": "application/json", "Cache-Control": "no-store", "Retry-After": "1", "X-YouYuan-Trace-ID": traceID},
		"body":    base64.StdEncoding.EncodeToString(body)})
	return response
}

func (a *yyRemoteAdmission) fail() {
	a.closeOnce.Do(func() {
		a.cancel()
		a.closeConn()
	})
}

// acquire starts no goroutine. false,nil means a not-sent rejection is queued.
func (a *yyRemoteAdmission) acquire(id string) (bool, error) {
	if err := a.ctx.Err(); err != nil {
		return false, err
	}
	select {
	case a.slots <- struct{}{}:
		return true, nil
	default:
	}
	select {
	case a.rejected <- yyRemoteNotSentResponse(id):
		return false, nil
	case <-a.ctx.Done():
		return false, a.ctx.Err()
	default:
		// The peer cannot consume bounded rejection capacity. End this channel
		// instead of accumulating goroutines or preventing heartbeat reads.
		a.fail()
		return false, errYYRemoteRejectionBacklog
	}
}

func (a *yyRemoteAdmission) release() { <-a.slots }

func (a *yyRemoteAdmission) close() {
	a.cancel()
	<-a.writerEnd
}

func (a *yyRemoteAdmission) writeRejections() {
	defer close(a.writerEnd)
	for {
		select {
		case <-a.ctx.Done():
			return
		case response := <-a.rejected:
			if a.ctx.Err() != nil {
				return
			}
			writeCtx, cancel := context.WithTimeout(a.ctx, a.timeout)
			err := a.write(writeCtx, response)
			cancel()
			if err != nil {
				a.fail()
				return
			}
		}
	}
}

func yyReadRemoteRequests(ctx context.Context, conn *websocket.Conn, lastSeen *atomic.Int64, dispatch func(yyRelayPacket), poolHello func(string)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if conn == nil {
		return errors.New("remote websocket unavailable")
	}
	admission := yyNewRemoteAdmission(ctx, func(writeCtx context.Context, response []byte) error {
		return conn.Write(writeCtx, websocket.MessageText, response)
	}, func() { _ = conn.CloseNow() }, yyRemoteRejectionTimeout)
	defer admission.close()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		lastSeen.Store(time.Now().UnixNano())
		var control struct {
			Kind   string `json:"kind"`
			Secret string `json:"secret"`
		}
		if json.Unmarshal(data, &control) == nil && control.Kind == "pool_hello" {
			if poolHello != nil {
				poolHello(control.Secret)
			}
			continue
		}
		if control.Kind == "pong" {
			continue
		}
		var request yyRelayPacket
		if json.Unmarshal(data, &request) != nil {
			continue
		}
		accepted, err := admission.acquire(request.ID)
		if err != nil {
			return err
		}
		if !accepted {
			continue
		}
		go func(packet yyRelayPacket) {
			defer admission.release()
			dispatch(packet)
		}(request)
	}
}
