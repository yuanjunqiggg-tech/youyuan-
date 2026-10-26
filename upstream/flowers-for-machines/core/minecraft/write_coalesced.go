package minecraft

import (
	"context"
	"runtime"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// WritePacketsBounded preserves tick coalescing and the module's packet order.
// It neither waits for command replies nor flushes at module boundaries.
func (conn *Conn) WritePacketsBounded(ctx context.Context, packets []packet.Packet) (attempted bool, err error) {
	if err := conn.lockModuleSend(ctx); err != nil {
		return false, err
	}
	locked := true
	defer func() {
		if locked {
			conn.sendMu.Unlock()
		}
	}()
	bytes := 0
	for _, p := range conn.bufferedSend {
		bytes += len(p)
	}
	for index, p := range packets {
		if err := ctx.Err(); err != nil {
			return attempted, err
		}
		select {
		case <-conn.ctx.Done():
			return attempted, conn.closeErr("write module")
		default:
		}
		attempted = true
		start := len(conn.bufferedSend)
		if err := conn.writePacketLocked(p); err != nil {
			return attempted, err
		}
		for _, b := range conn.bufferedSend[start:] {
			bytes += len(b)
		}
		if len(conn.bufferedSend) < 256 && bytes < 256*1024 {
			continue
		}
		err := conn.enc.Encode(conn.bufferedSend)
		for i := range conn.bufferedSend {
			conn.bufferedSend[i] = nil
		}
		conn.bufferedSend = conn.bufferedSend[:0]
		bytes = 0
		if err != nil {
			return attempted, err
		}
		if index+1 < len(packets) {
			// Let waiting control/reply traffic enter after a complete transport batch.
			conn.sendMu.Unlock()
			locked = false
			runtime.Gosched()
			if err := conn.lockModuleSend(ctx); err != nil {
				return attempted, err
			}
			locked = true
			for _, buffered := range conn.bufferedSend {
				bytes += len(buffered)
			}
		}
	}
	return attempted, nil
}

func (conn *Conn) lockModuleSend(ctx context.Context) error {
	if !conn.sendMu.TryLock() {
		// A stalled flush must not prevent a waiting module from being stopped.
		wait := time.NewTicker(time.Millisecond)
		defer wait.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-conn.ctx.Done():
				return conn.closeErr("write module")
			case <-wait.C:
			}
			if conn.sendMu.TryLock() {
				break
			}
		}
	}
	return nil
}

// NetworkStats contains only counters, never packets, commands, addresses or credentials.
func (conn *Conn) NetworkStats() map[string]any {
	if source, ok := conn.conn.(interface{ NetworkStats() map[string]any }); ok {
		return source.NetworkStats()
	}
	return map[string]any{"available": false}
}
