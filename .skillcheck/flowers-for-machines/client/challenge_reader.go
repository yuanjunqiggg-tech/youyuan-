package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

type challengeReader interface {
	ReadPacket() (packet.Packet, error)
	WritePacket(packet.Packet) error
	SetReadDeadline(time.Time) error
}

// One synchronous reader owns the connection until this function returns.
// No blocked challenge goroutine may steal the first post-login packet.
func collectChallengePackets(conn challengeReader, callback func(packet.Packet), rpc func(*packet.PyRpc) error, passed func() bool, capacity int) (cached chan packet.Packet, err error) {
	if capacity < 1 {
		capacity = 32767
	}
	cached = make(chan packet.Packet, capacity)
	defer close(cached)
	defer conn.SetReadDeadline(time.Time{})
	deadline := time.Now().Add(120 * time.Second)
	if err = conn.SetReadDeadline(deadline); err != nil {
		return
	}
	ready, chunkSeen := false, false
	for {
		if !time.Now().Before(deadline) {
			if ready {
				return cached, nil
			}
			return cached, fmt.Errorf("未通过 MCPC 验证挑战：%w", context.DeadlineExceeded)
		}
		pk, readErr := conn.ReadPacket()
		if readErr != nil {
			if ready && errors.Is(readErr, context.DeadlineExceeded) {
				return cached, nil // Keep the previous optional world-start wait.
			}
			return cached, readErr
		}
		if pk == nil {
			return cached, fmt.Errorf("登录读取返回空数据包")
		}
		if callback != nil {
			callback(pk)
		}
		retain := true
		if p, ok := pk.(*packet.PyRpc); ok {
			if err = rpc(p); err != nil {
				return cached, err
			}
			// Challenge messages were handled here; do not authenticate twice.
			// Application ModEventS2C frames used to be discarded unconditionally.
			retain = applicationLoginRPC(p.Value)
		}
		if _, ok := pk.(*packet.LevelChunk); ok {
			chunkSeen = true
		}
		if retain {
			select {
			case cached <- pk:
			default:
				return cached, fmt.Errorf("登录缓存已满（%d包）；中止接入，未静默丢弃后继续运行", capacity)
			}
		}
		if !ready && passed() {
			ready = true
			deadline = time.Now().Add(10 * time.Second)
			if err = conn.SetReadDeadline(deadline); err != nil {
				return
			}
			if err = conn.WritePacket(&packet.RequestChunkRadius{ChunkRadius: 12}); err != nil {
				return
			}
		}
		if ready && chunkSeen {
			return cached, nil
		}
	}
}

func applicationLoginRPC(value any) bool {
	v, ok := value.([]any)
	if !ok || len(v) != 3 {
		return false
	}
	switch name := v[0].(type) {
	case string:
		return name == "ModEventS2C"
	case []byte:
		return string(name) == "ModEventS2C"
	}
	return false
}
