package client

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/OmineDev/flowers-for-machines/core/bunker/auth"
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/core/py_rpc"
	cts "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server"
	cts_mc "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server/minecraft"
	cts_mc_p "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server/minecraft/preset"
	cts_mc_v "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server/minecraft/vip_event_system"
	mei "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/interface"
)

// openConnection 通过 authenticator 连接到租赁服，
// 并初始化 Minecraft 连接。返回 BotComponent 模组列表供后续模组同步使用。
func openConnection(
	ctx context.Context,
	authenticator minecraft.Authenticator,
) (conn *minecraft.Conn, botComponent map[string]*int, err error) {
	// prepare
	var dialer minecraft.Dialer
	var authResponse auth.AuthResponse
	var connectionLog *os.File

	// create connection
	dialer = minecraft.Dialer{
		Authenticator:        authenticator,
		DownloadResourcePack: func(id uuid.UUID, version string, current, total int) bool { return false },
	}
	if root := os.Getenv("YY_AUTH_CONFIG_DIR"); root != "" {
		if logFile, e := os.OpenFile(filepath.Join(root, "connection-errors.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); e == nil {
			connectionLog = logFile
			dialer.ErrorLog = slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelWarn}))
		}
	}
	conn, authResponse, err = dialer.DialContext(ctx, "raknet")
	if err != nil {
		if connectionLog != nil { connectionLog.Close() }
		return nil, nil, err
	}
	if connectionLog != nil { go func() { <-conn.Context().Done(); connectionLog.Close() }() }

	// capture bot component for return — used later by sendModSync
	botComponent = authResponse.BotComponent

	// 发送登录后的初始化包序列
	postLoginPackets(conn, authResponse.BotComponent, authResponse.BotSkin.ItemID)

	// return
	return
}

// postLoginPackets 发送登录后的初始化包序列（网易租赁服/TanLobby 通用）。
// botComponent 为模组列表，skinItemID 为皮肤资源 ID（entity_id）。
//
// 不发送 vanilla PlayerSkin 包：实测网易服务器不会转发它（设备日志中
// 没有回传记录），皮肤分发完全依赖登录 JWT ClientData —— 服务器据此
// 构建 PlayerList ADD 广播给其他玩家。参考实现（Conbit / bot-filler /
// raabel）也都没有此包。
func postLoginPackets(conn *minecraft.Conn, botComponent map[string]*int, skinItemID string) {
	runtimeid := fmt.Sprintf("%d", conn.GameData().EntityUniqueID)

	// 发送 LOGIN_UID
	conn.WritePacket(&packet.NeteaseJson{
		Data: []byte(
			fmt.Sprintf(
				`{"eventName":"LOGIN_UID","resid":"","uid":"%d"}`,
				conn.IdentityData().Uid,
			),
		),
	})

	// 发送 ClientCacheStatus（网易需要此包声明缓存状态）
	conn.WritePacket(&packet.ClientCacheStatus{
		Enabled: false,
	})

	// 模组同步 — SyncUsingMod
	{
		modUUIDs := make([]any, 0)
		botComp := make(map[string]int64, 0)
		for modUUID, outfitType := range botComponent {
			modUUIDs = append(modUUIDs, modUUID)
			if outfitType != nil {
				botComp[modUUID] = int64(*outfitType)
			}
		}
		conn.WritePacket(&packet.PyRpc{
			Value: py_rpc.Marshal(&py_rpc.SyncUsingMod{
				modUUIDs,
				conn.ClientData().SkinID,
				skinItemID,
				true,
				botComp,
			}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	}

	// SyncVipSkinUuid — 与 raabel（老官方框架）对齐：
	// 客户端进服时声明 VIP 皮肤 UUID（皮肤注册通道）。flowers 之前缺失此包。
	conn.WritePacket(&packet.PyRpc{
		Value:         py_rpc.Marshal(&py_rpc.SyncVipSkinUUID{nil}),
		OperationType: packet.PyRpcOperationTypeSend,
	})

	// ClientLoadAddonsFinishedFromGac
	conn.WritePacket(&packet.PyRpc{
		Value:         py_rpc.Marshal(&py_rpc.ClientLoadAddonsFinishedFromGac{}),
		OperationType: packet.PyRpcOperationTypeSend,
	})

	// GetLoadedInstances
	{
		event := cts_mc_p.GetLoadedInstances{PlayerRuntimeID: runtimeid}
		module := cts_mc.Preset{Module: &mei.DefaultModule{Event: &event}}
		park := cts.Minecraft{Default: mei.Default{Module: &module}}
		conn.WritePacket(&packet.PyRpc{
			Value: py_rpc.Marshal(&py_rpc.ModEvent{
				Package: &park,
				Type:    py_rpc.ModEventClientToServer,
			}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	}

	// ArenaGamePlayerFinishLoad
	conn.WritePacket(&packet.PyRpc{
		Value:         py_rpc.Marshal(&py_rpc.ArenaGamePlayerFinishLoad{}),
		OperationType: packet.PyRpcOperationTypeSend,
	})

	// PlayerUiInit
	{
		event := cts_mc_v.PlayerUiInit{RuntimeID: runtimeid}
		module := cts_mc.VIPEventSystem{Module: &mei.DefaultModule{Event: &event}}
		park := cts.Minecraft{Default: mei.Default{Module: &module}}
		conn.WritePacket(&packet.PyRpc{
			Value: py_rpc.Marshal(&py_rpc.ModEvent{
				Package: &park,
				Type:    py_rpc.ModEventClientToServer,
			}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	}

	// ClientInitUIFinishedEventFromGac
	conn.WritePacket(&packet.PyRpc{
		Value: py_rpc.Marshal(&py_rpc.Default{
			NAME: "ClientInitUIFinishedEventFromGac",
			Data: []any{},
		}),
		OperationType: packet.PyRpcOperationTypeSend,
	})
}
