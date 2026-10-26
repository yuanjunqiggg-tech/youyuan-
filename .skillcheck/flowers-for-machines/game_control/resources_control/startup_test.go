package resources_control

import (
	"context"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestListenersInstalledBeforeLoginCacheAndLiveReader(t *testing.T) {
	r := &Resources{listener: NewPacketListener(context.Background())}
	cached := make(chan packet.Packet, 3)
	cached <- &packet.Animate{EntityRuntimeID: 11}
	cached <- nil
	cached <- &packet.Animate{EntityRuntimeID: 22}
	close(cached)
	seen := []uint64{}
	done := make(chan struct{})
	r.startWithCache(cached, func(r *Resources) {
		_, err := r.listener.ListenPacketOrdered(nil, nil, func(p packet.Packet, _ error) {
			seen = append(seen, p.(*packet.Animate).EntityRuntimeID)
		})
		if err != nil {
			t.Fatal(err)
		}
	}, func() {
		r.handlePacket(&packet.Animate{EntityRuntimeID: 33})
		close(done)
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("live reader did not start")
	}
	if len(seen) != 3 || seen[0] != 11 || seen[1] != 22 || seen[2] != 33 {
		t.Fatalf("login or live packets lost/reordered: %v", seen)
	}
}
