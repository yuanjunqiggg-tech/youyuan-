package minecraft

import (
	"sync"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

func TestInputTickContinuesAcrossHeartbeatRespawnAndTeleport(t *testing.T) {
	c := coalescedFixture()
	var ticks []uint64
	c.SetOutboundPacketObserver(func(pk packet.Packet) { ticks = append(ticks, pk.(*packet.PlayerAuthInput).Tick) })
	flags := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	flags.Set(packet.InputFlagHandledTeleport)
	for i, tick := range []uint64{170, 0, 171, 0, 0, 180} {
		p := &packet.PlayerAuthInput{Tick: tick, Position: mgl32.Vec3{1, 64, 2}, InputData: flags, InputMode: packet.InputModeMouse}
		if err := c.WritePacket(p); err != nil {
			t.Fatal(err)
		}
		if p.Tick != tick || p.Position[1] != 64 || !p.InputData.Load(packet.InputFlagHandledTeleport) {
			t.Fatal("caller packet or teleport flags changed")
		}
		want := []uint64{170, 171, 172, 173, 174, 180}[i]
		if ticks[i] != want {
			t.Fatal(ticks)
		}
	}
}

func TestInputTickSerialisesConcurrentWritersAndResetsOnNewConnection(t *testing.T) {
	c := coalescedFixture()
	var ticks []uint64
	c.SetOutboundPacketObserver(func(pk packet.Packet) { ticks = append(ticks, pk.(*packet.PlayerAuthInput).Tick) })
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.WritePacket(&packet.PlayerAuthInput{InputData: protocol.NewBitset(packet.PlayerAuthInputBitsetSize)}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for i, tick := range ticks {
		if tick != uint64(i+1) {
			t.Fatal(ticks)
		}
	}
	if len(ticks) != 40 || coalescedFixture().inputTick != 0 {
		t.Fatal("connection sequence leaked")
	}
	c.inputTick = ^uint64(0)
	if err := c.WritePacket(&packet.PlayerAuthInput{}); err == nil {
		t.Fatal("tick wrapped")
	}
}
