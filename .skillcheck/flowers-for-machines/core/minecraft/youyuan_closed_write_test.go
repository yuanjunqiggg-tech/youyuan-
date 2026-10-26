package minecraft

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestWritePacketLockedRejectsClosureDuringSendLockWait(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	ctx, cancel := context.WithCancelCause(context.Background())
	c := &Conn{ctx: ctx, conn: a}
	c.sendMu.Lock()
	closed := errors.New("remote closed while sender waited")
	cancel(closed)
	c.sendMu.Unlock()
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if err := c.writePacketLocked(&packet.SettingsCommand{CommandLine: "say test"}); !errors.Is(err, closed) {
		t.Fatalf("closed connection accepted a packet: %v", err)
	}
	if len(c.bufferedSend) != 0 {
		t.Fatal("packet appended to dead connection")
	}
}
