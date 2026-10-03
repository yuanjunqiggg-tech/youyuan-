package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"testing"
)

func TestOldConnectionCannotDisconnectReplacement(t *testing.T) {
	old, replacement := new(minecraft.Conn), new(minecraft.Conn)
	bm := &BotManager{conn: replacement, connected: true, stopCh: make(chan struct{})}
	if bm.disconnectConnection(old) || bm.disconnectConnection(nil) {
		t.Fatal("stale listener disconnected the replacement")
	}
	if bm.conn != replacement || !bm.connected {
		t.Fatal("replacement connection changed")
	}
	select {
	case <-bm.stopCh:
		t.Fatal("replacement producers stopped")
	default:
	}
}
