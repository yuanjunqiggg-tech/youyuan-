package main

import (
	"sync"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

func TestPacketToMapBasic(t *testing.T) {
	m := packetToMap(&packet.SetTime{Time: 12345})
	if m["time"] != int64(12345) {
		t.Fatalf("time 字段 = %v, want 12345", m["time"])
	}
}

func TestPacketToMapMovePlayerNested(t *testing.T) {
	m := packetToMap(&packet.MovePlayer{
		EntityRuntimeID: 99,
		Position:        mgl32.Vec3{1, 2, 3},
		Yaw:             45.5,
		OnGround:        true,
	})
	if m["entity_runtime_id"] != uint64(99) {
		t.Fatalf("entity_runtime_id = %v, want 99", m["entity_runtime_id"])
	}
	pos, ok := m["position"].([]any)
	// 序列化统一归一化：float32 → float64（对 JSON/AI 输出友好）
	if !ok || len(pos) != 3 || pos[0] != float64(1) || pos[1] != float64(2) || pos[2] != float64(3) {
		t.Fatalf("position 应序列化为 [x,y,z]，got %v (%T)", m["position"], m["position"])
	}
	if m["on_ground"] != true {
		t.Fatalf("on_ground = %v, want true", m["on_ground"])
	}
}

// 用户要求数据包不能截断：[]byte 必须给完整 hex。
func TestPacketToMapFullBytes(t *testing.T) {
	sample := struct {
		Blob   []byte
		Fixed  [4]byte
		Text   string
	}{
		Blob:  []byte{0xde, 0xad, 0xbe, 0xef, 0x01},
		Fixed: [4]byte{0x00, 0x01, 0x02, 0x03},
		Text:  "hi",
	}
	m := packetToMap(&sample)
	if m["blob"] != "deadbeef01" {
		t.Fatalf("blob 应完整 hex=deadbeef01，got %v", m["blob"])
	}
	if m["fixed"] != "00010203" {
		t.Fatalf("fixed 应完整 hex=00010203，got %v", m["fixed"])
	}
}

func TestSnakeCase(t *testing.T) {
	cases := map[string]string{
		"EntityRuntimeID": "entity_runtime_id",
		"OnGround":        "on_ground",
		"ID":              "id",
		"XYZ":             "xyz",
		"HeadYaw":         "head_yaw",
	}
	for in, want := range cases {
		if got := snakeCase(in); got != want {
			t.Errorf("snakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPacketHubSubscribeDispatch(t *testing.T) {
	bm := &BotManager{}
	var mu sync.Mutex
	got := []string{}
	unsub := packetHubSubscribe("SetTime", func(b *BotManager, data map[string]any) {
		mu.Lock()
		got = append(got, "settime")
		mu.Unlock()
	})

	// 订阅了 SetTime：收到 SetTime 应触发
	packetHubDispatch(bm, &packet.SetTime{Time: 1})
	mu.Lock()
	n := len(got)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("订阅 SetTime 后 Dispatch 应触发 1 次，got %d", n)
	}

	// 未订阅 MovePlayer：不应触发
	packetHubDispatch(bm, &packet.MovePlayer{})
	mu.Lock()
	n = len(got)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("未订阅 MovePlayer 不应触发，got %d 次", n)
	}

	// 退订后不再触发
	unsub()
	packetHubDispatch(bm, &packet.SetTime{Time: 2})
	mu.Lock()
	n = len(got)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("退订后 Dispatch 不应触发，got %d 次", n)
	}
}
