package main

// Visibility is negotiated by the game server. Capture retention and entity
// query radii must never be reported as a server broadcast radius.
import (
	"fmt"
	"math"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

var yyVisibilityMethods = map[string]string{
	"diagnostics.visibility":  "无参数。实际区块半径、Publisher、筛选前decoded/selected/filtered和enqueued/cancelled逐类型计数。ingress含监听阶段与登录缓存回放，login_ingress单独记录挑战回调，两者有重叠不能相加；不含解码失败。显式PyRpc抓包时记录应用事件名称、不留参数；登录目录亦只留名称，容量event_history。记录不代表可调用或全服广播。无宿主距离过滤；区块半径不等于实体广播范围。",
	"bot.requestViewDistance": "chunks,timeout_ms=5000；chunks=1..2147483647协议整数，不附加32区块上限。发送RequestChunkRadius（同时设置MaxChunkRadius）并等待服务器ChunkRadiusUpdated；服务器可调小或不回复。confirmed仅表示收到半径响应，不保证远方血量/挥臂广播；超时返回confirmed=false，不冒充生效。",
}

func init() {
	for k, v := range yyVisibilityMethods {
		yyExtraMethods[k] = v
	}
}

func yyVisibilityRecordRequest(bm *BotManager, radius int32) {
	s := yySenseFor(bm)
	s.Lock()
	s.RequestedRadius, s.RequestedRadiusAt = radius, time.Now()
	s.Unlock()
}

func yyVisibilitySnapshot(bm *BotManager) map[string]any {
	depth, capacity, connected := 0, 0, false
	var ingress, loginIngress *yyPacketIngress
	if bm != nil {
		bm.mu.Lock()
		depth, capacity, connected = len(bm.pktCh), cap(bm.pktCh), bm.connected
		ingress = bm.packetIngress
		loginIngress = bm.loginIngress
		bm.mu.Unlock()
	}
	ingressSnapshot := ingress.snapshot()
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	counts := make(map[string]uint64, len(s.PacketCounts))
	for k, v := range s.PacketCounts {
		counts[k] = v
	}
	requested := any(nil)
	if !s.RequestedRadiusAt.IsZero() {
		requested = s.RequestedRadius
	}
	granted := any(nil)
	if !s.GrantedRadiusAt.IsZero() {
		granted = s.GrantedRadius
	}
	publisher := any(nil)
	if !s.PublisherAt.IsZero() {
		publisher = map[string]any{"position": s.PublisherPosition, "radius_blocks": s.PublisherRadius, "timestamp": s.PublisherAt.UnixMilli(), "source": "NetworkChunkPublisherUpdate"}
	}
	return map[string]any{
		"connected": connected, "host_distance_filter": false,
		"requested_radius_chunks": requested, "requested_at": yyVisibilityTime(s.RequestedRadiusAt),
		"granted_radius_chunks": granted, "granted_at": yyVisibilityTime(s.GrantedRadiusAt), "granted_source": s.GrantedRadiusSource,
		"publisher": publisher, "entity_broadcast_radius_known": false,
		"observed_entities": len(s.Actors), "packet_counts": counts,
		"ingress":             ingressSnapshot,
		"login_ingress":       loginIngress.snapshot(),
		"packet_count_scope":  "selected incoming decoded packets in this connection session; not every wire packet",
		"receive_queue_depth": depth, "receive_queue_capacity": capacity,
		"capture_retained": s.Capture.Count, "capture_overwritten": s.Capture.Dropped, "capture_sampled_out": s.Capture.SampledOut,
		"scope": "server-supplied observations for this account; chunk visibility does not guarantee entity attributes or actions",
	}
}

func yyVisibilityTime(at time.Time) any {
	if at.IsZero() {
		return nil
	}
	return at.UnixMilli()
}

func yyVisibilityCall(bm *BotManager, method string, args []any) (any, error) {
	if method == "diagnostics.visibility" {
		if len(args) != 0 {
			return nil, fmt.Errorf("visibility()不接收参数")
		}
		return yyVisibilitySnapshot(bm), nil
	}
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("requestViewDistance(chunks, timeout_ms=5000)")
	}
	radius, err := yyNBTBoundedInteger(args[0], 1, math.MaxInt32)
	if err != nil {
		return nil, fmt.Errorf("chunks: %w", err)
	}
	timeout := int64(5000)
	if len(args) == 2 {
		timeout, err = yyNBTBoundedInteger(args[1], 100, 30000)
		if err != nil {
			return nil, fmt.Errorf("timeout_ms: %w", err)
		}
	}
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("目标游戏账号未连接")
	}
	s := yySenseFor(bm)
	// Protocol has no request ID. Serialize this high-level negotiation; raw
	// requests from unrelated writers cannot be correlated by the protocol.
	s.ViewRequestMu.Lock()
	defer s.ViewRequestMu.Unlock()
	bm.mu.Lock()
	conn, resources := bm.conn, bm.resources
	bm.mu.Unlock()
	if conn == nil || resources == nil {
		return nil, fmt.Errorf("连接尚未初始化")
	}
	responses := make(chan int32, 1)
	id, err := resources.PacketListener().ListenPacketOrdered([]uint32{packet.IDChunkRadiusUpdated}, nil, func(pk packet.Packet, closeErr error) {
		if closeErr != nil {
			return
		}
		if p, ok := pk.(*packet.ChunkRadiusUpdated); ok {
			select {
			case responses <- p.ChunkRadius:
			default:
			}
		}
	})
	if err != nil {
		return nil, err
	}
	defer resources.PacketListener().DestroyListener(id)
	if err = bm.WritePacket(&packet.RequestChunkRadius{ChunkRadius: int32(radius), MaxChunkRadius: int32(radius)}); err != nil {
		return nil, err
	}
	result := map[string]any{"submitted": true, "confirmed": false, "requested_radius_chunks": radius, "granted_radius_chunks": nil, "entity_broadcast_guaranteed": false, "response_correlation": "no protocol request ID; avoid simultaneous raw radius requests"}
	timer := time.NewTimer(time.Duration(timeout) * time.Millisecond)
	defer timer.Stop()
	select {
	case granted := <-responses:
		s.Lock()
		s.GrantedRadius, s.GrantedRadiusAt, s.GrantedRadiusSource = granted, time.Now(), "ChunkRadiusUpdated"
		s.Unlock()
		result["confirmed"], result["granted_radius_chunks"], result["clamped"] = true, granted, int64(granted) < radius
	case <-timer.C:
		result["error"] = "服务器未在等待时间内返回半径；请求生效与否未知"
	case <-conn.Context().Done():
		return nil, fmt.Errorf("等待视距响应时游戏连接关闭")
	}
	return result, nil
}
