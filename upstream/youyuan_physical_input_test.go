package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"testing"
)

func TestPhysicalInputContinuesHeartbeatTickAndRejectsReplacedConnection(t *testing.T) {
	conn := new(minecraft.Conn)
	bm := &BotManager{conn: conn, connected: true, tick: 173}
	for _, want := range []uint64{174, 175} {
		pk, err := bm.yyPhysicalInput(conn, nil)
		if err != nil || pk.Tick != want {
			t.Fatalf("physical input must continue heartbeat sequence: packet=%+v err=%v", pk, err)
		}
	}
	bm.conn = new(minecraft.Conn)
	if _, err := bm.yyPhysicalInput(conn, nil); err == nil || bm.tick != 175 {
		t.Fatal("stale action used a replacement connection or advanced its tick")
	}
}
