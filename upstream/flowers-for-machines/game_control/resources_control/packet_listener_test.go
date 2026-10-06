package resources_control

import (
	"context"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"testing"
	"time"
)

func TestOrderedPacketDeliveryAndUnsubscribeOutsideLock(t *testing.T) {
	p := NewPacketListener(context.Background())
	var id string
	calls := 0
	var err error
	id, err = p.ListenPacketOrdered([]uint32{packet.IDText, packet.IDSetTime}, nil, func(pk packet.Packet, _ error) {
		calls++
		p.DestroyListener(id) // Must not deadlock, and must remove every subscribed ID.
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { p.onPacket(&packet.Text{}); p.onPacket(&packet.SetTime{}); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("callback cannot unsubscribe")
	}
	if calls != 1 {
		t.Fatalf("listener survived unsubscribe: %d calls", calls)
	}
}

func TestOrderedQueuePreservesOrderAndReleasesOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := NewPacketListener(ctx)
	queue := make(chan packet.Packet, 1)
	_, err := p.ListenPacketOrdered(nil, nil, func(pk packet.Packet, _ error) {
		select {
		case queue <- pk:
		case <-ctx.Done():
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	p.onPacket(&packet.Text{Message: "first"})
	entered, done := make(chan struct{}), make(chan struct{})
	go func() { close(entered); p.onPacket(&packet.Text{Message: "second"}); close(done) }()
	<-entered
	select {
	case <-done:
		t.Fatal("full queue bypassed bounded backpressure")
	case <-time.After(20 * time.Millisecond):
	}
	if got := (<-queue).(*packet.Text).Message; got != "first" {
		t.Fatal(got)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queue did not resume")
	}
	if got := (<-queue).(*packet.Text).Message; got != "second" {
		t.Fatal(got)
	}
	p.onPacket(&packet.Text{Message: "third"})
	done = make(chan struct{})
	go func() { p.onPacket(&packet.Text{Message: "blocked"}); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("connection cancellation did not release producer")
	}
}
