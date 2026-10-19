//go:build js && wasm

package main

import (
	"context"
	"errors"
	"fmt"
	link "github.com/Yeah114/g79client/service/link_connection"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"syscall/js"
	"time"
)

func withWorkersTCP(ctx context.Context, factory js.Value) context.Context {
	return link.WithDialer(ctx, func(ctx context.Context, network, address string) (result net.Conn, err error) {
		return dialWorkersTCP(ctx, factory, network, address)
	})
}
func dialWorkersTCP(ctx context.Context, factory js.Value, network, address string) (result net.Conn, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("Workers Link TCP bridge: %v", p)
		}
	}()
	if network != "tcp" {
		return nil, fmt.Errorf("unsupported Link network %s", network)
	}
	c := &workersConn{socket: factory.Invoke(address), remote: workersAddr(address), closed: make(chan struct{}), deadlineChanged: make(chan struct{})}
	if _, e := c.await(ctx, c.socket.Get("opened"), 0); e != nil {
		c.Close()
		return nil, e
	}
	return c, nil
}

type workersAddr string

func (a workersAddr) Network() string { return "tcp" }
func (a workersAddr) String() string  { return string(a) }

type workersResult struct {
	value js.Value
	err   error
}
type workersConn struct {
	socket                      js.Value
	remote                      workersAddr
	closed                      chan struct{}
	once                        sync.Once
	mu                          sync.Mutex
	readMu, writeMu             sync.Mutex
	readDeadline, writeDeadline time.Time
	deadlineChanged             chan struct{}
}

func workersJSError(v js.Value) error {
	message := "Workers Link TCP error"
	if v.Type() == js.TypeObject && v.Get("message").Type() == js.TypeString {
		message = v.Get("message").String()
	} else if v.Type() == js.TypeString {
		message = v.String()
	}
	// Only explicit refusal during connection establishment is eligible for the
	// library's transport failover; generic handshake errors stay unclassified.
	if strings.Contains(strings.ToLower(message), "connection refused") {
		return fmt.Errorf("Workers TCP: %w", syscall.ECONNREFUSED)
	}
	return errors.New(message)
}
func (c *workersConn) await(ctx context.Context, promise js.Value, kind int) (js.Value, error) {
	done := make(chan workersResult, 1)
	resolve := js.FuncOf(func(_ js.Value, args []js.Value) any {
		v := js.Undefined()
		if len(args) > 0 {
			v = args[0]
		}
		done <- workersResult{value: v}
		return nil
	})
	reject := js.FuncOf(func(_ js.Value, args []js.Value) any {
		e := errors.New("Workers TCP failed")
		if len(args) > 0 {
			e = workersJSError(args[0])
		}
		done <- workersResult{err: e}
		return nil
	})
	promise.Call("then", resolve, reject)
	defer func() { go func() { resolve.Release(); reject.Release() }() }()
	for {
		c.mu.Lock()
		changed := c.deadlineChanged
		deadline := time.Time{}
		if kind == 1 {
			deadline = c.readDeadline
		} else if kind == 2 {
			deadline = c.writeDeadline
		}
		c.mu.Unlock()
		var timer *time.Timer
		var timeout <-chan time.Time
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				c.Close()
				<-done
				return js.Undefined(), os.ErrDeadlineExceeded
			}
			timer = time.NewTimer(remaining)
			timeout = timer.C
		}
		stop := func() {
			if timer != nil {
				timer.Stop()
			}
		}
		select {
		case r := <-done:
			stop()
			return r.value, r.err
		case <-ctx.Done():
			stop()
			c.Close()
			<-done
			return js.Undefined(), ctx.Err()
		case <-c.closed:
			stop()
			<-done
			return js.Undefined(), net.ErrClosed
		case <-timeout:
			c.Close()
			<-done
			return js.Undefined(), os.ErrDeadlineExceeded
		case <-changed:
			stop()
		}
	}
}
func (c *workersConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	c.readMu.Lock()
	defer c.readMu.Unlock()
	value, err := c.await(context.Background(), c.socket.Call("read", len(p)), 1)
	if err != nil {
		return 0, err
	}
	if value.Get("byteLength").Int() == 0 {
		return 0, io.EOF
	}
	return js.CopyBytesToGo(p, value), nil
}
func (c *workersConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	bytes := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(bytes, p)
	value, err := c.await(context.Background(), c.socket.Call("write", bytes), 2)
	if err != nil {
		return 0, err
	}
	return value.Int(), nil
}
func (c *workersConn) Close() error {
	c.once.Do(func() { close(c.closed); c.socket.Call("close") })
	return nil
}
func (c *workersConn) LocalAddr() net.Addr  { return workersAddr("Workers TCP") }
func (c *workersConn) RemoteAddr() net.Addr { return c.remote }
func (c *workersConn) setDeadline(t time.Time, kind int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if kind != 2 {
		c.readDeadline = t
	}
	if kind != 1 {
		c.writeDeadline = t
	}
	close(c.deadlineChanged)
	c.deadlineChanged = make(chan struct{})
	return nil
}
func (c *workersConn) SetDeadline(t time.Time) error      { return c.setDeadline(t, 0) }
func (c *workersConn) SetReadDeadline(t time.Time) error  { return c.setDeadline(t, 1) }
func (c *workersConn) SetWriteDeadline(t time.Time) error { return c.setDeadline(t, 2) }
