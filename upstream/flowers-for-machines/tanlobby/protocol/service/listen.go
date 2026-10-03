package service

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"github.com/Happy2018new/nemc-tan-lobby-solver/bunker"
	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/core/raknet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/encoding"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/packet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/service/signaling"
)

// ListenConfig ..
type ListenConfig struct {
	bunker.Authenticator
	RoomConfig
	serverNetherID    uint64
	raknetConnection  net.Conn
	roomPlayerCount   atomic.Int32
	netherNetListener *nethernet.Listener
}

// Listen ..
func Listen(roomConfig RoomConfig, authenticator bunker.Authenticator) (
	listenConfig *ListenConfig,
	listener *nethernet.Listener,
	roomID uint32,
	err error,
) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	listenConfig, listener, roomID, err = ListenContext(ctx, roomConfig, authenticator)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("监听：%v", err)
	}

	return
}

// ListenContext ..
func ListenContext(ctx context.Context, roomConfig RoomConfig, authenticator bunker.Authenticator) (
	listenConfig *ListenConfig,
	listener *nethernet.Listener,
	roomID uint32,
	err error,
) {
	listenConfig = &ListenConfig{
		Authenticator: authenticator,
		RoomConfig:    roomConfig,
	}
	listener, roomID, err = listenConfig.ListenContext(ctx)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("监听上下文：%v",err)
	}
	return
}

// createTanLobbyRoom ..
func (l *ListenConfig) createTanLobbyRoom(
	ctx context.Context,
	tanLobbyCreateResp bunker.TanLobbyCreateResponse,
) (
	conn net.Conn,
	enc *packet.Encoder,
	dec *packet.Decoder,
	roomID uint32,
	err error,
) {
	// Create conn
	conn, err = raknet.DialContext(ctx, tanLobbyCreateResp.RaknetServerAddress)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：%v",err)
	}

	// Set encoder and decoder
	enc = packet.NewEncoder(conn)
	dec, err = packet.NewDecoder(conn)
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：%v",err)
	}

	// Send login request
	err = writePacket(enc, &packet.TanLoginRequest{
		PlayerID:   tanLobbyCreateResp.UserUniqueID,
		Rand:       tanLobbyCreateResp.RaknetRand,
		AESRand:    tanLobbyCreateResp.RaknetAESRand,
		PlayerName: tanLobbyCreateResp.UserPlayerName,
	})
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：%v",err)
	}

	// Handle login response
	pk, err := readPacketWithContext(ctx, conn, dec)
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：%v",err)
	}
	tanLoginResp, ok := pk.(*packet.TanLoginResponse)
	if !ok {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：期望收到 *packet.TanLoginResponse 数据包，但实际得到 %#v", pk)
	}
	if tanLoginResp.ErrorCode != packet.TanLoginSuccess {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：登录失败（错误码 = %d）", tanLoginResp.ErrorCode)
	}

	// Enable encryption
	enc.EnableEncryption(tanLobbyCreateResp.EncryptKeyBytes, tanLobbyCreateResp.DecryptKeyBytes)
	dec.EnableEncryption(tanLobbyCreateResp.EncryptKeyBytes, tanLobbyCreateResp.DecryptKeyBytes)

	// Create room
	err = writePacket(enc, &packet.TanCreateRoomRequest{
		Capacity: l.RoomConfig.MaxPlayerCount,
		Privacy:  l.RoomConfig.RoomPrivacy,
		Name:     "",
		Tips: encoding.RoomTips{
			LevelID:            "World",
			GameType:           0,
			ConstantTestString: "test",
			Vioce:              0,
			ProtocolID:         42,
			GameVersion:        "1.21.120.0",
		},
		ItemIDs:      l.RoomConfig.UsedModItemIDs,
		MinLevel:     0,
		PvP:          l.RoomConfig.AllowPvP,
		TeamID:       0,
		PlayerAuth:   l.RoomConfig.PlayerPermission,
		Password:     l.RoomConfig.RoomPasscode,
		Slogan:       l.RoomConfig.RoomName,
		MapID:        0,
		EnableWebRTC: true,
		OwnerPing:    3,
		PerfLv:       1,
	})
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：%v",err)
	}

	// Read create room response
	pk, err = readPacketWithContext(ctx, conn, dec)
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：%v",err)
	}
	tanCreateRoomResp, ok := pk.(*packet.TanCreateRoomResponse)
	if !ok {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：期望收到 *packet.TanEnterRoomResponse 数据包，但实际得到 %#v", pk)
	}

	// Handle create room response
	if tanCreateRoomResp.ErrorCode == packet.TanCreateRoomNeedVipToSetRoomName {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：只能使用内置房间名称（需要 VIP 才能设置自定义名称）")
	}
	if tanCreateRoomResp.ErrorCode != packet.TanCreateRoomSuccess {
		_ = conn.Close()
		return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：创建网易大厅房间失败（错误码 = %d）", tanCreateRoomResp.ErrorCode)
	}

	// Set room tag if needed
	if len(l.RoomConfig.RoomTagList) > 0 {
		err = writePacket(enc, &packet.TanSetTagListRequest{
			TagList: l.RoomConfig.RoomTagList,
		})
		if err != nil {
			_ = conn.Close()
			return nil, nil, nil, 0, fmt.Errorf("创建网易大厅房间：%v",err)
		}
	}

	// Return
	return conn, enc, dec, tanCreateRoomResp.RoomID, nil
}

// startTanLobbyRoom ..
func (l *ListenConfig) startTanLobbyRoom(ctx context.Context, enc *packet.Encoder, dec *packet.Decoder) (err error) {
	// Set channel to recieve set tag list response and error
	pkChannel := make(chan packet.TanSetTagListResponse)
	errChannel := make(chan error, 1)

	// Handle incoming raknet packet
	go func() {
		for {
			// Read packet
			pk, err := readPacket(dec)
			if err != nil {
				// Always send error to the channel
				errChannel <- err
				close(errChannel)
				// Then close the room and return
				l.CloseRoom()
				return
			}
			// Handle packet
			switch p := pk.(type) {
			case *packet.TanNewGuestResponse:
				l.roomPlayerCount.Add(1)
				writePacket(enc, &packet.TanNotifyServerReady{
					ServerAddress:         "127.0.0.1|19132",
					ServerRaknetGuid:      "",
					RTCRoomID:             "",
					NetherNetID:           fmt.Sprintf("%d", l.serverNetherID),
					WebRTCCompressEnabled: true,
				})
			case *packet.TanLeaveRoomResponse:
				l.roomPlayerCount.Add(-1)
			case *packet.TanSetTagListResponse:
				pkChannel <- *p
				close(pkChannel)
			}
		}
	}()

	// Handle set tag list response
	if len(l.RoomConfig.RoomTagList) > 0 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("启动网易大厅房间：%v",ctx.Err())
		case err = <-errChannel:
			return fmt.Errorf("启动网易大厅房间：%v",err)
		case p := <-pkChannel:
			if p.ErrorCode == packet.TanSetTagListExceedLimit {
				return fmt.Errorf("启动网易大厅房间：最多只能设置 2 个标签")
			}
			if p.ErrorCode != packet.TanSetTagListSuccess {
				return fmt.Errorf("启动网易大厅房间：设置房间标签失败（错误码 = %d）", p.ErrorCode)
			}
		}
	}

	// Return
	return nil
}

// ListenContext ..
func (l *ListenConfig) ListenContext(ctx context.Context) (listener *nethernet.Listener, roomID uint32, err error) {
	// Prepare
	var enc *packet.Encoder
	var dec *packet.Decoder
	l.serverNetherID = rand.Uint64()

	// Get room create info
	tanLobbyCreateResp, err := l.Authenticator.GetCreate()
	if err != nil {
		return nil, 0, fmt.Errorf("监听上下文：%v",err)
	}
	if !tanLobbyCreateResp.Success {
		return nil, 0, fmt.Errorf("监听上下文：%v",tanLobbyCreateResp.ErrorInfo)
	}

	// Create tan lobby room
	l.raknetConnection, enc, dec, roomID, err = l.createTanLobbyRoom(ctx, tanLobbyCreateResp)
	if err != nil {
		return nil, 0, fmt.Errorf("监听上下文：%v",err)
	}

	// Connect to websocket signaling server
	wsConnection, err := signaling.Dialer{
		Authenticator: l.Authenticator,
		RefreshTime:   l.RoomRefreshTime,
		NetherNetID:   l.serverNetherID,
	}.DialContext(
		ctx,
		tanLobbyCreateResp.SignalingServerAddress,
		tanLobbyCreateResp.UserUniqueID,
		tanLobbyCreateResp.SignalingSeed,
		tanLobbyCreateResp.SignalingTicket,
	)
	if err != nil {
		_ = l.raknetConnection.Close()
		return nil, 0, fmt.Errorf("监听上下文：%v",err)
	}

	// Init listen config
	listenConfig := nethernet.ListenConfig{
		ConnContext: func(parent context.Context, conn *nethernet.Conn) context.Context {
			ctx, cancel := context.WithTimeout(parent, time.Second*30)
			go func() {
				<-ctx.Done()
				cancel()
			}()
			return ctx
		},
	}

	// Create listener
	l.netherNetListener, err = listenConfig.Listen(wsConnection)
	if err != nil {
		_ = l.raknetConnection.Close()
		wsConnection.Close(fmt.Errorf("监听上下文：%v",err))
		return nil, 0, fmt.Errorf("监听上下文：%v",err)
	}

	// Start tan lobby room
	err = l.startTanLobbyRoom(ctx, enc, dec)
	if err != nil {
		_ = l.raknetConnection.Close()
		wsConnection.Close(fmt.Errorf("监听上下文：%v",err))
		return nil, 0, fmt.Errorf("监听上下文：%v",err)
	}

	// Return
	return l.netherNetListener, roomID, nil
}

// PlayerCount ..
func (l *ListenConfig) PlayerCount() int32 {
	return l.roomPlayerCount.Load()
}

// CloseRoom ..
func (l *ListenConfig) CloseRoom() {
	if l.raknetConnection != nil {
		_ = l.raknetConnection.Close()
	}
	if l.netherNetListener != nil {
		_ = l.netherNetListener.Close()
	}
}
