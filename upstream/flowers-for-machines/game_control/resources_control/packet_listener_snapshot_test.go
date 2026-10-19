package resources_control

import (
	"context"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestListenerMutationKeepsInFlightOrder(t *testing.T) {
	p := NewPacketListener(context.Background())
	var got []string
	var removed string
	changed := false
	_, err := p.ListenPacketOrdered(nil, nil, func(packet.Packet, error) {
		got = append(got, "any")
		if !changed {
			changed = true
			p.DestroyListener(removed)
			_, err := p.ListenPacketOrdered([]uint32{packet.IDSetTime}, nil, func(packet.Packet, error) { got = append(got, "new") })
			if err != nil {
				t.Error(err)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	removed, err = p.ListenPacketOrdered([]uint32{packet.IDSetTime}, nil, func(packet.Packet, error) { got = append(got, "old") })
	if err != nil {
		t.Fatal(err)
	}
	p.onPacket(&packet.SetTime{})
	p.onPacket(&packet.SetTime{})
	if !reflect.DeepEqual(got, []string{"any", "old", "any", "new"}) {
		t.Fatalf("subscription mutation changed an in-flight packet: %v", got)
	}
}

func TestOrderedListenerDeliversEveryPacketInSequence(t *testing.T) {
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
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			p.onPacket(&packet.SetTime{Time: int32(i)})
		}
	}()
	for i := 0; i < 1000; i++ {
		select {
		case pk := <-queue:
			if got := pk.(*packet.SetTime).Time; got != int32(i) {
				t.Fatalf("packet %d got %d", i, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("packet %d was not delivered", i)
		}
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("producer did not finish")
	}
}

func TestListenerConcurrentRegistrationDoesNotLoseStableSubscriber(t *testing.T) {
	p := NewPacketListener(context.Background())
	var count atomic.Uint64
	_, err := p.ListenPacketOrdered(nil, nil, func(packet.Packet, error) { count.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				p.onPacket(&packet.SetTime{})
			}
		}()
	}
	for i := 0; i < 500; i++ {
		id, err := p.ListenPacketOrdered([]uint32{packet.IDSetTime}, nil, func(packet.Packet, error) {})
		if err != nil {
			t.Fatal(err)
		}
		p.DestroyListener(id)
	}
	wg.Wait()
	if got := count.Load(); got != 4000 {
		t.Fatalf("stable subscriber got %d of 4000 packets", got)
	}
}

func TestFilteredAsyncListenerStillReturnsBeforeCallback(t *testing.T) {
	p := NewPacketListener(context.Background())
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer close(release)
	var rejected atomic.Int32
	_, err := p.ListenPacketFiltered(nil, func(packet.Packet) bool { return false }, func(packet.Packet, error) { rejected.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.ListenPacket(nil, func(packet.Packet, error) { close(entered); <-release })
	if err != nil {
		t.Fatal(err)
	}
	go func() { p.onPacket(&packet.SetTime{}); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("async listener blocked receive")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("async callback did not run")
	}
	if rejected.Load() != 0 {
		t.Fatal("filtered callback was executed")
	}
}

func TestListenerCancellationAndCloseStopFurtherDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := NewPacketListener(ctx)
	var calls atomic.Int32
	closed := make(chan struct{})
	_, err := p.ListenPacketOrdered(nil, nil, func(_ packet.Packet, err error) {
		if err != nil {
			close(closed)
			return
		}
		calls.Add(1)
	})
	if err != nil {
		t.Fatal(err)
	}
	p.onPacket(&packet.SetTime{})
	cancel()
	p.onPacket(&packet.SetTime{})
	if _, err := p.ListenPacket(nil, func(packet.Packet, error) {}); err == nil {
		t.Fatal("registered listener after cancellation")
	}
	p.handleConnClose(context.Canceled)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close callback was not delivered")
	}
	if calls.Load() != 1 {
		t.Fatalf("cancelled session delivered %d packets", calls.Load())
	}
	// An explicit close also clears readers when a caller has not cancelled ctx.
	open := NewPacketListener(context.Background())
	_, err = open.ListenPacketOrdered(nil, nil, func(pk packet.Packet, err error) {
		if pk != nil {
			calls.Add(1)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	open.handleConnClose(context.Canceled)
	open.onPacket(&packet.SetTime{})
	if calls.Load() != 1 {
		t.Fatal("closed listener retained live callbacks")
	}
}

// Baseline is the pre-change receive path, with identical callbacks and filter.
func legacyListenerOnPacket(p *PacketListener, pk packet.Packet) {
	p.mu.Lock()
	select {
	case <-p.ctx.Done():
		p.mu.Unlock()
		return
	default:
	}
	listeners := make([]singleListener, 0, len(p.anyPacketListeners)+len(p.specificPacketListeners[pk.ID()]))
	listeners = append(listeners, p.anyPacketListeners...)
	listeners = append(listeners, p.specificPacketListeners[pk.ID()]...)
	p.mu.Unlock()
	for _, listener := range listeners {
		if listener.filter != nil && !listener.filter(pk) {
			continue
		}
		if listener.ordered {
			listener.callback(pk, nil)
		} else {
			go listener.callback(pk, nil)
		}
	}
}

func BenchmarkOrderedListenerDispatch(b *testing.B) {
	for _, listeners := range []int{0, 1, 8} {
		for _, legacy := range []bool{true, false} {
			name := "current"
			if legacy {
				name = "legacy"
			}
			b.Run(name+"/listeners_"+strconv.Itoa(listeners), func(b *testing.B) {
				p := NewPacketListener(context.Background())
				calls := 0
				for i := 0; i < listeners; i++ {
					var ids []uint32
					if i%2 != 0 {
						ids = []uint32{packet.IDSetTime}
					}
					_, err := p.ListenPacketOrdered(ids, func(pk packet.Packet) bool { return pk.ID() == packet.IDSetTime }, func(packet.Packet, error) { calls++ })
					if err != nil {
						b.Fatal(err)
					}
				}
				pk := &packet.SetTime{Time: 100}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if legacy {
						legacyListenerOnPacket(p, pk)
					} else {
						p.onPacket(pk)
					}
				}
				b.StopTimer()
				if calls != b.N*listeners {
					b.Fatalf("callbacks %d, want %d", calls, b.N*listeners)
				}
			})
		}
	}
}

func BenchmarkListenerRegistrationChange(b *testing.B) {
	for _, listeners := range []int{0, 1, 8} {
		b.Run("stable_"+strconv.Itoa(listeners), func(b *testing.B) {
			p := NewPacketListener(context.Background())
			for i := 0; i < listeners; i++ {
				if _, err := p.ListenPacketOrdered(nil, nil, func(packet.Packet, error) {}); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				id, err := p.ListenPacketOrdered([]uint32{packet.IDText, packet.IDSetTime}, nil, func(packet.Packet, error) {})
				if err != nil {
					b.Fatal(err)
				}
				p.DestroyListener(id)
			}
		})
	}
}
