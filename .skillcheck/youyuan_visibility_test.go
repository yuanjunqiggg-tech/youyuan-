package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	"testing"
)

func TestVisibilityDefaultCaptureKeepsConsecutiveSwings(t *testing.T) {
	b := senseBot(t)
	if _, err := yyCaptureStart(b, yyCaptureConfig{}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		yyCapturePacket(b, &packet.Animate{EntityRuntimeID: 555, ActionType: packet.AnimateActionSwingArm})
	}
	r := yyCaptureRead(b, 0, 10)
	if len(r["records"].([]map[string]any)) != 2 || r["sampled_out"] != uint64(0) {
		t.Fatal("default capture hid consecutive swing packets", r)
	}
}

func TestVisibilityFarPacketsHaveNoHostDistanceFilter(t *testing.T) {
	b := senseBot(t)
	yySenseHandle(b, &packet.AddActor{EntityRuntimeID: 555, EntityUniqueID: 555, EntityType: "minecraft:zombie", Position: mgl32.Vec3{100000, 60, -100000}})
	senseHealth(b, 555, 13)
	yySenseHandle(b, &packet.Animate{EntityRuntimeID: 555, ActionType: packet.AnimateActionSwingArm})
	v := yySenseActor(b, "555")
	if v["health_known"] != true || *v["health"].(*float32) != 13 {
		t.Fatal("far received health discarded", v)
	}
	s := yySenseFor(b)
	s.Lock()
	swings := 0
	for _, e := range s.Events {
		if e["type"] == "swing" && e["runtime_id"] == "555" {
			swings++
		}
	}
	s.Unlock()
	if swings != 1 {
		t.Fatal("far received swing discarded", swings)
	}
}

func TestVisibilitySeparatesNegotiationPublisherAndCapture(t *testing.T) {
	b := senseBot(t)
	b.pktCh = make(chan packet.Packet, 8192)
	yyVisibilityRecordRequest(b, 64)
	yySenseHandle(b, &packet.ChunkRadiusUpdated{ChunkRadius: 12})
	yySenseHandle(b, &packet.NetworkChunkPublisherUpdate{Position: protocol.BlockPos{100, 60, 300}, Radius: 128})
	v := yyVisibilitySnapshot(b)
	if v["requested_radius_chunks"] != int32(64) || v["granted_radius_chunks"] != int32(12) || v["receive_queue_capacity"] != 8192 {
		t.Fatal(v)
	}
	if v["publisher"].(map[string]any)["radius_blocks"] != uint32(128) || v["entity_broadcast_radius_known"] != false || v["host_distance_filter"] != false {
		t.Fatal(v)
	}
	yySenseReset(b)
	v = yyVisibilitySnapshot(b)
	if v["publisher"] != nil || v["granted_radius_chunks"] != nil || len(v["packet_counts"].(map[string]uint64)) != 0 {
		t.Fatal("stale connection telemetry", v)
	}
}

func TestVisibilityRequestValidationAndCatalog(t *testing.T) {
	for _, a := range [][]any{{}, {0}, {-1}, {1.5}, {int64(2147483648)}, {1, 0}, {1, 5000, 1}} {
		if _, e := yyVisibilityCall(nil, "bot.requestViewDistance", a); e == nil {
			t.Fatal("invalid radius accepted", a)
		}
	}
	for k := range yyVisibilityMethods {
		if yyExtraMethods[k] == "" {
			t.Fatal("missing binding", k)
		}
	}
}
