package minecraft

import (
	"context"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// WritePacketsBounded preserves tick coalescing and wire order with one module lock.
// It neither waits for command replies nor flushes at module boundaries.
func (conn *Conn) WritePacketsBounded(ctx context.Context, packets []packet.Packet) (attempted bool, err error) {
	conn.sendMu.Lock()
	defer conn.sendMu.Unlock()
	bytes := 0
	for _, p := range conn.bufferedSend {
		bytes += len(p)
	}
	for _, p := range packets {
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
	}
	return attempted, nil
}

// NetworkStats contains only counters, never packets, commands, addresses or credentials.
func (conn *Conn) NetworkStats() map[string]any {
	if source, ok := conn.conn.(interface{ NetworkStats() map[string]any }); ok {
		return source.NetworkStats()
	}
	return map[string]any{"available": false}
}
