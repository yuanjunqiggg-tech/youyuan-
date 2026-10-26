package main

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

type yyObserverInputSession struct {
	Cancel    context.CancelFunc
	Conn      *minecraft.Conn
	ID        string
	Capacity  int
	Active    atomic.Bool
	Dropped   atomic.Uint64
	Submitted atomic.Uint64
	LastError atomic.Value
}

func init() {
	yyObserverBridgeMethods["observerBridge.attachSelf"] = "source_id,token；将当前已连接机器人的自身加密前输入接入采集桥，player_id必须匹配本机UUID/XUID/名字。异步采集、不阻塞命令发送；不等于其他玩家已接入。"
	yyExtraMethods["observerBridge.attachSelf"] = yyObserverBridgeMethods["observerBridge.attachSelf"]
	for method, doc := range map[string]string{"observerBridge.inputStatus": "无参数。自身输入桥运行状态、采集记录数、未投递数和错误；不代表游戏处理成功。", "observerBridge.detachSelf": "无参数。停止当前机器人的输入采集，游戏连接和其他来源不受影响。"} {
		yyObserverBridgeMethods[method] = doc
		yyExtraMethods[method] = doc
	}
}

func yyObserverSelfStatus(bm *BotManager, method string, args []any) (any, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("此接口无参数")
	}
	var s *yyObserverInputSession
	if bm != nil {
		bm.mu.Lock()
		s = bm.inputBridge
		bm.mu.Unlock()
	}
	if s == nil {
		return map[string]any{"active": false}, nil
	}
	if method == "observerBridge.detachSelf" {
		bm.mu.Lock()
		if bm.inputBridge != s {
			bm.mu.Unlock()
			return nil, fmt.Errorf("采集实例已改变，请重新读取状态")
		}
		s.Active.Store(false)
		s.Conn.SetOutboundPacketObserver(nil)
		s.Cancel()
		bm.mu.Unlock()
	}
	last := ""
	if v := s.LastError.Load(); v != nil {
		last = v.(string)
	}
	return map[string]any{"active": s.Active.Load(), "source_id": s.ID, "queue_capacity": s.Capacity, "submitted": s.Submitted.Load(), "dropped": s.Dropped.Load(), "last_error": last, "server_confirmed": false}, nil
}

// Copy just the observable input facts, never inventory contents or credentials.
func yyObserverInputPacket(pk packet.Packet) []map[string]any {
	base := func(kind, name string, data map[string]any) map[string]any {
		return map[string]any{"kind": kind, "packet_type": name, "observation": "local_packet_submission", "data": data}
	}
	switch p := pk.(type) {
	case *packet.Animate:
		if p.ActionType == packet.AnimateActionSwingArm {
			return []map[string]any{base("swing_input", "Animate", map[string]any{"runtime_id": strconv.FormatUint(p.EntityRuntimeID, 10), "missed_swing_known": false})}
		}
	case *packet.PlayerAuthInput:
		data := map[string]any{"position": p.Position, "pitch": p.Pitch, "yaw": p.Yaw, "tick": strconv.FormatUint(p.Tick, 10)}
		rows := []map[string]any{base("position_input", "PlayerAuthInput", data)}
		if p.InputData.Len() > packet.InputFlagMissedSwing && p.InputData.Load(packet.InputFlagMissedSwing) {
			rows = append(rows, base("swing_input", "PlayerAuthInput", map[string]any{"tick": strconv.FormatUint(p.Tick, 10), "missed_swing": true}))
		}
		return rows
	case *packet.InventoryTransaction:
		if d, ok := p.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && d.ActionType == protocol.UseItemOnEntityActionAttack {
			return []map[string]any{base("attack_input", "InventoryTransaction", map[string]any{"target_runtime_id": strconv.FormatUint(d.TargetEntityRuntimeID, 10), "position": d.Position})}
		}
	}
	return nil
}

func yyObserverAttachSelf(bm *BotManager, args []any) (any, error) {
	if bm == nil || len(args) != 2 {
		return nil, fmt.Errorf("attachSelf(source_id,token)需要已连接账号")
	}
	id, ok := args[0].(string)
	token, yes := args[1].(string)
	if !ok || !yes {
		return nil, fmt.Errorf("source_id和token需字符串")
	}
	bm.mu.Lock()
	conn, server, uuid, xuid, name := bm.conn, bm.serverCode, bm.idStr, bm.xuid, bm.display
	bm.mu.Unlock()
	if conn == nil {
		return nil, fmt.Errorf("机器人未连接")
	}
	yyObserver.Lock()
	source, exists := yyObserver.Sources[id]
	yyObserver.Unlock()
	if !exists || source.Source != "player_input" || source.Server != server || (source.Player != uuid && source.Player != xuid && source.Player != name) {
		return nil, fmt.Errorf("采集来源未绑定此服务器和当前机器人身份")
	}
	// Verify credentials without consuming a sequence or returning their hash.
	if !yyObserverValidToken(source, token) {
		return nil, fmt.Errorf("采集密钥无效")
	}
	queue := make(chan []map[string]any, yyObservationSettings().EventQueue)
	ctx, cancel := context.WithCancel(conn.Context())
	session := &yyObserverInputSession{Cancel: cancel, Conn: conn, ID: id, Capacity: cap(queue)}
	session.Active.Store(true)
	bm.mu.Lock()
	if old := bm.inputBridge; old != nil {
		old.Active.Store(false)
		old.Cancel()
	}
	bm.inputBridge = session
	conn.SetOutboundPacketObserver(func(pk packet.Packet) {
		if !session.Active.Load() {
			return
		}
		if rows := yyObserverInputPacket(pk); len(rows) > 0 {
			select {
			case queue <- rows:
			default:
				session.Dropped.Add(uint64(len(rows)))
			}
		}
	})
	bm.mu.Unlock()
	go func() {
		defer session.Active.Store(false)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		pending := []map[string]any{}
		flush := func() {
			if len(pending) == 0 {
				return
			}
			result, err := yyObserver.publishLocal(token, id, pending)
			if err != nil {
				session.Dropped.Add(uint64(len(pending)))
				session.LastError.Store(err.Error())
			} else if n, ok := result["accepted"].(int); ok {
				session.Submitted.Add(uint64(n))
			} else {
				session.Dropped.Add(uint64(len(pending)))
				session.LastError.Store("sequence重用，数据未再次投递")
			}
			pending = nil
		}
		for {
			select {
			case <-ctx.Done():
				session.Active.Store(false)
				for {
					select {
					case rows := <-queue:
						session.Dropped.Add(uint64(len(rows)))
					default:
						flush()
						return
					}
				}
			case rows := <-queue:
				pending = append(pending, rows...)
				if len(pending) >= 512 {
					flush()
				}
			case <-ticker.C:
				flush()
			}
		}
	}()
	return map[string]any{"attached": true, "source_id": id, "player_id": source.Player, "scope": "this robot's own local submissions only", "server_confirmed": false, "queue_capacity": cap(queue), "bridge_flush_ms": 50}, nil
}
