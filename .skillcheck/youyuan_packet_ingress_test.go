package main

import (
	"sync"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestIngressDistinguishesFilteredFromUnreceived(t *testing.T) {
	b := senseBot(t)
	a := &yyPacketIngress{started: time.Now()}
	b.packetIngress = a
	p := &packet.PlayerAction{EntityRuntimeID: 123}
	if yySelectIncomingPacket(b, p, nil, a) {
		t.Fatal("unsubscribed action should remain unselected")
	}
	v := yyVisibilitySnapshot(b)
	rows := v["ingress"].(map[string]any)["types"].(map[string]map[string]any)
	if rows["PlayerAction"]["decoded"] != uint64(1) || rows["PlayerAction"]["filtered"] != uint64(1) {
		t.Fatal(rows)
	}
	if _, exists := rows["Animate"]; exists {
		t.Fatal("invented an unreceived packet", rows)
	}
	if len(v["packet_counts"].(map[string]uint64)) != 0 {
		t.Fatal("filtered packet masqueraded as consumed", v)
	}
	if _, err := yyCaptureStart(b, yyCaptureConfig{Types: []string{"PlayerAction"}}); err != nil {
		t.Fatal(err)
	}
	if !yySelectIncomingPacket(b, p, nil, a) {
		t.Fatal("capture subscription did not select previously filtered type")
	}
	a.delivery(p, true)
	r := a.snapshot()["types"].(map[string]map[string]any)["PlayerAction"]
	if r["decoded"] != uint64(2) || r["selected"] != uint64(1) || r["filtered"] != uint64(1) || r["enqueued"] != uint64(1) {
		t.Fatal(r)
	}
}

func TestIngressCancellationAndReconnectIsolation(t *testing.T) {
	b := senseBot(t)
	old := &yyPacketIngress{started: time.Now()}
	b.packetIngress = old
	p := &packet.Animate{EntityRuntimeID: 12}
	if !yySelectIncomingPacket(b, p, map[uint32]bool{packet.IDAnimate: true}, old) {
		t.Fatal("core packet filtered")
	}
	old.delivery(p, false)
	if old.snapshot()["types"].(map[string]map[string]any)["Animate"]["cancelled"] != uint64(1) {
		t.Fatal("cancelled delivery lost")
	}
	yySenseReset(b)
	if yyVisibilitySnapshot(b)["ingress"].(map[string]any)["active"] != false {
		t.Fatal("old connection audit survived reset")
	}
	fresh := &yyPacketIngress{started: time.Now()}
	b.packetIngress = fresh
	old.observe(p, true) // A closing old listener still owns its old session audit.
	if len(fresh.snapshot()["types"].(map[string]map[string]any)) != 0 {
		t.Fatal("old listener contaminated new session")
	}
}

func TestIngressKeepsUnknownPacketIDsSeparate(t *testing.T) {
	a := &yyPacketIngress{}
	a.observe(&packet.Unknown{PacketID: 800, Payload: []byte{1}}, false)
	a.observe(&packet.Unknown{PacketID: 801, Payload: []byte{2}}, false)
	r := a.snapshot()["types"].(map[string]map[string]any)
	if len(r) != 2 || r["Unknown:800"]["packet_id"] != uint32(800) || r["Unknown:801"]["packet_id"] != uint32(801) {
		t.Fatal("unknown packet types collapsed", r)
	}
}

func TestIngressConcurrentCounts(t *testing.T) {
	a := &yyPacketIngress{}
	p := &packet.Animate{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				a.observe(p, true)
				a.delivery(p, true)
				_ = a.snapshot()
			}
		}()
	}
	wg.Wait()
	r := a.snapshot()["types"].(map[string]map[string]any)["Animate"]
	if r["decoded"] != uint64(8000) || r["selected"] != uint64(8000) || r["enqueued"] != uint64(8000) {
		t.Fatal(r)
	}
}

func BenchmarkIngressNumericAudit(b *testing.B) {
	a := &yyPacketIngress{}
	p := &packet.Animate{}
	a.observe(p, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.observe(p, true)
		a.delivery(p, true)
	}
}
