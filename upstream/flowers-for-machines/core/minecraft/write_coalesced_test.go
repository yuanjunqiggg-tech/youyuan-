package minecraft

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func coalescedFixture() *Conn {
	return &Conn{ctx: context.Background(), proto: proto{}, hdr: &packet.Header{}}
}
func coalescedPackets(n int) []packet.Packet {
	p := make([]packet.Packet, n)
	for i := range p {
		p[i] = &packet.SettingsCommand{CommandLine: fmt.Sprintf("execute as Bot at @s run execute if entity @s[tag=qa_missing] run scoreboard players set @s qa %d", i), SuppressOutput: true}
	}
	return p
}
func TestCoalescedModulePreservesBytesAndTickBuffer(t *testing.T) {
	packets := coalescedPackets(164)
	a, b := coalescedFixture(), coalescedFixture()
	for _, p := range packets {
		if err := a.WritePacketBounded(p); err != nil {
			t.Fatal(err)
		}
	}
	if attempted, err := b.WritePacketsBounded(context.Background(), packets); !attempted || err != nil {
		t.Fatal(attempted, err)
	}
	if len(a.bufferedSend) != 164 || len(b.bufferedSend) != 164 {
		t.Fatal("module boundary flushed early")
	}
	for i := range a.bufferedSend {
		if !bytes.Equal(a.bufferedSend[i], b.bufferedSend[i]) {
			t.Fatalf("wire bytes/order changed at %d", i)
		}
	}
}
func TestCoalescedModuleRetainsBoundAndCancellation(t *testing.T) {
	c := coalescedFixture()
	out := new(bytes.Buffer)
	c.enc = packet.NewEncoder(out)
	if _, err := c.WritePacketsBounded(context.Background(), coalescedPackets(300)); err != nil {
		t.Fatal(err)
	}
	if len(c.bufferedSend) != 44 || out.Len() == 0 {
		t.Fatal("256-packet bound lost", len(c.bufferedSend))
	}
	c = coalescedFixture()
	ctx, cancel := context.WithCancel(context.Background())
	seen := 0
	c.SetOutboundPacketObserver(func(packet.Packet) {
		seen++
		if seen == 7 {
			cancel()
		}
	})
	if attempted, err := c.WritePacketsBounded(ctx, coalescedPackets(164)); !attempted || err != context.Canceled || len(c.bufferedSend) != 7 {
		t.Fatal(attempted, err, len(c.bufferedSend))
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	c = coalescedFixture()
	if attempted, err := c.WritePacketsBounded(ctx, coalescedPackets(2)); attempted || err != context.Canceled || len(c.bufferedSend) != 0 {
		t.Fatal(attempted, err)
	}
}
func BenchmarkCoalescedModule(b *testing.B) {
	packets := coalescedPackets(164)
	for _, module := range []bool{false, true} {
		b.Run(fmt.Sprintf("module_%v", module), func(b *testing.B) {
			c := coalescedFixture()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				c.bufferedSend = c.bufferedSend[:0]
				if module {
					c.WritePacketsBounded(context.Background(), packets)
				} else {
					for _, p := range packets {
						c.WritePacketBounded(p)
					}
				}
			}
		})
	}
}
