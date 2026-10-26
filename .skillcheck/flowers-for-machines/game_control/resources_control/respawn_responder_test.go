package resources_control

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

type respawnWrite struct {
	packet packet.Packet
	at     time.Time
}

func respawnFixture(t *testing.T) (*Resources, chan respawnWrite, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	writes := make(chan respawnWrite, 12)
	r := &Resources{
		listener: NewPacketListener(ctx), commands: NewCommandRequestCallback(ctx),
		itemStack: NewItemStackOperationManager(ctx), container: NewContainerManager(ctx),
	}
	r.respawn = newRespawnResponder(ctx, func(pk packet.Packet) error {
		writes <- respawnWrite{pk, time.Now()}
		return nil
	})
	return r, writes, cancel
}

func nextRespawnWrite(t *testing.T, writes <-chan respawnWrite) respawnWrite {
	t.Helper()
	select {
	case got := <-writes:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("respawn acknowledgement was not written")
		return respawnWrite{}
	}
}

func waitRespawnIdle(t *testing.T, r *respawnResponder) {
	t.Helper()
	select {
	case r.gate <- struct{}{}:
		<-r.gate
	case <-time.After(time.Second):
		t.Fatal("respawn worker did not stop")
	}
}

func TestRespawnReadyDoesNotBlockFollowingPacketAndKeepsFiveAcks(t *testing.T) {
	r, writes, _ := respawnFixture(t)
	seen := make(chan packet.Packet, 2)
	_, err := r.listener.ListenPacketOrdered(nil, nil, func(pk packet.Packet, _ error) { seen <- pk })
	if err != nil {
		t.Fatal(err)
	}
	position := mgl32.Vec3{1, 64, -2}
	done := make(chan struct{})
	started := time.Now()
	go func() {
		r.handlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn, Position: position})
		r.handlePacket(&packet.Animate{EntityRuntimeID: 12})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("ready-to-spawn stalled the receive loop")
	}
	t.Logf("ready plus following packet receive time: %v", time.Since(started))
	if _, ok := (<-seen).(*packet.Respawn); !ok {
		t.Fatal("respawn delivery reordered")
	}
	if _, ok := (<-seen).(*packet.Animate); !ok {
		t.Fatal("following packet was lost")
	}
	input := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	input.Set(packet.InputFlagStartFlying)
	want := &packet.PlayerAuthInput{Position: position, InputData: input}
	var previous time.Time
	for i := 0; i < 5; i++ {
		got := nextRespawnWrite(t, writes)
		if !reflect.DeepEqual(got.packet, want) {
			t.Fatalf("ack %d changed fields: %#v", i, got.packet)
		}
		if !previous.IsZero() && got.at.Sub(previous) < 140*time.Millisecond {
			t.Fatal("ack pacing was removed")
		}
		previous = got.at
	}
	waitRespawnIdle(t, r.respawn)
	t.Logf("five acknowledgements including original final wait: %v", time.Since(started))
	select {
	case <-writes:
		t.Fatal("wrote more than five acknowledgements")
	default:
	}
}

func TestRespawnSearchKeepsHandshakeFieldsAndWaitsForReadySequence(t *testing.T) {
	r, writes, _ := respawnFixture(t)
	checkSearch := func(id uint64) {
		t.Helper()
		first := nextRespawnWrite(t, writes).packet
		second := nextRespawnWrite(t, writes).packet
		if !reflect.DeepEqual(first, &packet.Respawn{State: packet.RespawnStateClientReadyToSpawn, EntityRuntimeID: id}) {
			t.Fatalf("client-ready fields changed: %#v", first)
		}
		if !reflect.DeepEqual(second, &packet.PlayerAction{EntityRuntimeID: id, ActionType: protocol.PlayerActionRespawn, BlockFace: -1}) {
			t.Fatalf("respawn action fields changed: %#v", second)
		}
	}
	r.respawn.search(42)
	checkSearch(42)
	r.handlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn})
	_ = nextRespawnWrite(t, writes)
	done := make(chan struct{})
	go func() { r.respawn.search(43); close(done) }()
	select {
	case <-done:
		t.Fatal("search interleaved the active ready sequence")
	case <-time.After(20 * time.Millisecond):
	}
	for i := 1; i < 5; i++ {
		if _, ok := nextRespawnWrite(t, writes).packet.(*packet.PlayerAuthInput); !ok {
			t.Fatal("search bypassed remaining ready acknowledgements")
		}
	}
	checkSearch(43)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("search did not finish")
	}
	waitRespawnIdle(t, r.respawn)
}

func TestOverlappingRespawnsSerializeWithoutDroppingRequests(t *testing.T) {
	r, writes, _ := respawnFixture(t)
	first, second := mgl32.Vec3{1, 64, 2}, mgl32.Vec3{3, 70, 4}
	r.handlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn, Position: first})
	got := nextRespawnWrite(t, writes)
	if got.packet.(*packet.PlayerAuthInput).Position != first {
		t.Fatal("first sequence changed")
	}
	done := make(chan struct{})
	go func() {
		r.handlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn, Position: second})
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("overlap spawned a second worker")
	case <-time.After(20 * time.Millisecond):
	}
	for i := 1; i < 10; i++ {
		got := nextRespawnWrite(t, writes).packet.(*packet.PlayerAuthInput)
		want := first
		if i >= 5 {
			want = second
		}
		if got.Position != want {
			t.Fatalf("ack %d belongs to wrong respawn: %v", i, got.Position)
		}
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second request remained blocked")
	}
	waitRespawnIdle(t, r.respawn)
}

func TestRespawnCancellationReleasesWorkerAndOverlappingReader(t *testing.T) {
	r, writes, cancel := respawnFixture(t)
	r.handlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn})
	_ = nextRespawnWrite(t, writes)
	done := make(chan struct{})
	go func() { r.handlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn}); close(done) }()
	start := time.Now()
	cancel()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("cancel did not release overlapping reader")
	}
	waitRespawnIdle(t, r.respawn)
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("cancel waited for the acknowledgement timer")
	}
	r.handlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn})
	select {
	case <-writes:
		t.Fatal("cancelled connection wrote another acknowledgement")
	case <-time.After(170 * time.Millisecond):
	}
}

func TestRespawnResourceCloseStopsWithoutParentCancellation(t *testing.T) {
	r, writes, _ := respawnFixture(t)
	r.handlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn})
	_ = nextRespawnWrite(t, writes)
	r.handleConnClose(context.DeadlineExceeded)
	waitRespawnIdle(t, r.respawn)
	r.handlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn})
	select {
	case <-writes:
		t.Fatal("resource read failure left a live respawn responder")
	case <-time.After(170 * time.Millisecond):
	}
}
