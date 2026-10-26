package client

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/core/py_rpc"
)

func TestRuntimeChallengeDoesNotRepeatPassedAuthentication(t *testing.T) {
	c := &Client{getCheckNumEverPassed: true}
	p := &packet.PyRpc{Value: py_rpc.Marshal(&py_rpc.GetMCPCheckNum{})}
	if err := c.HandleRuntimeChallenge(p); err != nil {
		t.Fatal(err)
	}
}

type fakeChallengeReader struct {
	packets       []packet.Packet
	reads, writes int
	deadline      time.Time
	err           error
}

func (c *fakeChallengeReader) ReadPacket() (packet.Packet, error) {
	c.reads++
	if len(c.packets) == 0 {
		return nil, fmt.Errorf("read: %w", context.DeadlineExceeded)
	}
	p := c.packets[0]
	c.packets = c.packets[1:]
	return p, nil
}
func (c *fakeChallengeReader) WritePacket(packet.Packet) error   { c.writes++; return c.err }
func (c *fakeChallengeReader) SetReadDeadline(t time.Time) error { c.deadline = t; return nil }

func TestChallengeHandoffRetainsApplicationRPCAndNextLivePacket(t *testing.T) {
	app := &packet.PyRpc{Value: []any{[]byte("ModEventS2C"), []any{"n", "s", "e", nil}, nil}}
	c := &fakeChallengeReader{packets: []packet.Packet{&packet.PyRpc{Value: "challenge"}, app, &packet.UpdateAttributes{}, &packet.LevelChunk{}, &packet.Animate{EntityRuntimeID: 99}}}
	passed, callbacks := false, 0
	cache, err := collectChallengePackets(c, func(packet.Packet) { callbacks++ }, func(*packet.PyRpc) error { passed = true; return nil }, func() bool { return passed }, 10)
	if err != nil {
		t.Fatal(err)
	}
	var rows []packet.Packet
	for p := range cache {
		rows = append(rows, p)
	}
	if len(rows) != 3 || rows[0] != app || c.reads != 4 || c.writes != 1 || callbacks != 4 || !c.deadline.IsZero() {
		t.Fatal("handoff failed", len(rows), c.reads, c.writes, callbacks, c.deadline)
	}
	p, err := c.ReadPacket()
	if err != nil || p.(*packet.Animate).EntityRuntimeID != 99 {
		t.Fatal("first live packet stolen", p, err)
	}
}
func TestChallengeReaderReportsOverflowInsteadOfSilentLoss(t *testing.T) {
	c := &fakeChallengeReader{packets: []packet.Packet{&packet.Animate{}, &packet.Animate{}}}
	_, err := collectChallengePackets(c, nil, func(*packet.PyRpc) error { return nil }, func() bool { return false }, 1)
	if err == nil || !strings.Contains(err.Error(), "缓存已满") || !c.deadline.IsZero() {
		t.Fatal(err, c.deadline)
	}
}
func TestChallengeWorldTimeoutAndWriteFailure(t *testing.T) {
	for _, writeErr := range []error{nil, fmt.Errorf("write failed")} {
		c := &fakeChallengeReader{packets: []packet.Packet{&packet.PyRpc{}}, err: writeErr}
		_, err := collectChallengePackets(c, nil, func(*packet.PyRpc) error { return nil }, func() bool { return true }, 5)
		if (err != nil) != (writeErr != nil) || !c.deadline.IsZero() {
			t.Fatal(err, c.deadline)
		}
	}
}
