package service

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/Happy2018new/nemc-tan-lobby-solver/bunker"
	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/core/raknet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/packet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/service/signaling"
)

func tanDebugf(format string, args ...any) {
	if os.Getenv("TANDBG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "[TANDBG] "+format+"\n", args...)
}

// Dialer ..
type Dialer struct {
	bunker.Authenticator
	RoomID         string
	RoomPasscode   string
	clientNetherID uint64
}

// Dial ..
func Dial(roomID string, roomPasscode string, authenticator bunker.Authenticator) (net.Conn, bunker.TanLobbyLoginResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	conn, tanLobbyLoginResp, err := DialContext(ctx, roomID, roomPasscode, authenticator)
	if err != nil {
		return nil, bunker.TanLobbyLoginResponse{}, fmt.Errorf("拨号：%v", err)
	}

	return conn, tanLobbyLoginResp, nil
}

// DialContext ..
func DialContext(
	ctx context.Context,
	roomID string,
	roomPasscode string,
	authenticator bunker.Authenticator,
) (net.Conn, bunker.TanLobbyLoginResponse, error) {
	dialer := Dialer{
		Authenticator: authenticator,
		RoomID:        roomID,
		RoomPasscode:  roomPasscode,
	}
	conn, tanLobbyLoginResp, err := dialer.DialContext(ctx)
	if err != nil {
		return nil, bunker.TanLobbyLoginResponse{}, fmt.Errorf("拨号上下文：%v", err)
	}
	return conn, tanLobbyLoginResp, nil
}

// enterTanLobbyRoom ..
func (d *Dialer) enterTanLobbyRoom(ctx context.Context, tanLobbyLoginResp bunker.TanLobbyLoginResponse) (
	remoteNetherNetID uint64,
	err error,
) {
	// Generate client nether ID and parse basic info
	d.clientNetherID = rand.Uint64()
	roomID, err := strconv.ParseUint(d.RoomID, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("进入网易大厅房间：%v", err)
	}

	// Create conn
	tanDebugf("enter room: dialing raknet=%s room=%s", tanLobbyLoginResp.RaknetServerAddress, d.RoomID)
	conn, err := raknet.DialContext(ctx, tanLobbyLoginResp.RaknetServerAddress)
	if err != nil {
		return 0, fmt.Errorf("进入网易大厅房间：%v", err)
	}
	defer conn.Close()
	tanDebugf("enter room: raknet connected")

	// Set encoder and decoder
	enc := packet.NewEncoder(conn)
	dec, err := packet.NewDecoder(conn)
	if err != nil {
		return 0, fmt.Errorf("进入网易大厅房间：%v", err)
	}

	// Send login request
	err = writePacket(enc, &packet.TanLoginRequest{
		PlayerID:   tanLobbyLoginResp.UserUniqueID,
		Rand:       tanLobbyLoginResp.RaknetRand,
		AESRand:    tanLobbyLoginResp.RaknetAESRand,
		PlayerName: tanLobbyLoginResp.UserPlayerName,
	})
	if err != nil {
		return 0, fmt.Errorf("进入网易大厅房间：%v", err)
	}

	// Handle login response
	pk, err := readPacketWithContext(ctx, conn, dec)
	if err != nil {
		return 0, fmt.Errorf("进入网易大厅房间：%v", err)
	}
	tanLoginResp, ok := pk.(*packet.TanLoginResponse)
	if !ok {
		return 0, fmt.Errorf("进入网易大厅房间：期望收到 *packet.TanLoginResponse 数据包，但实际得到 %#v", pk)
	}
	if tanLoginResp.ErrorCode != packet.TanLoginSuccess {
		return 0, fmt.Errorf("进入网易大厅房间：登录失败（错误码 = %d）", tanLoginResp.ErrorCode)
	}
	tanDebugf("enter room: tan login accepted")

	// Enable encryption
	enc.EnableEncryption(tanLobbyLoginResp.EncryptKeyBytes, tanLobbyLoginResp.DecryptKeyBytes)
	dec.EnableEncryption(tanLobbyLoginResp.EncryptKeyBytes, tanLobbyLoginResp.DecryptKeyBytes)

	// Enter room
	err = writePacket(enc, &packet.TanEnterRoomRequest{
		OwnerID:               tanLobbyLoginResp.RoomOwnerID,
		RoomID:                uint32(roomID),
		EnterPassword:         d.RoomPasscode,
		EnterTeamID:           0,
		EnterToken:            0,
		FollowTeamID:          0,
		NetherNetID:           fmt.Sprintf("%d", d.clientNetherID),
		SupportWebRTCCompress: true,
	})
	if err != nil {
		return 0, fmt.Errorf("进入网易大厅房间：%v", err)
	}

	// Handle enter room response
	pk, err = readPacketWithContext(ctx, conn, dec)
	if err != nil {
		return 0, fmt.Errorf("进入网易大厅房间：%v", err)
	}
	tanEnterRoomResp, ok := pk.(*packet.TanEnterRoomResponse)
	if !ok {
		return 0, fmt.Errorf("进入网易大厅房间：期望收到 *packet.TanEnterRoomResponse 数据包，但实际得到 %#v", pk)
	}
	if tanEnterRoomResp.ErrorCode != packet.TanEnterRoomSuccess {
		switch tanEnterRoomResp.ErrorCode {
		case packet.TanEnterRoomNotFound:
			return 0, fmt.Errorf("进入网易大厅房间：目标房间（%d）已关闭", roomID)
		case packet.TanEnterRoomWrongPasscode:
			return 0, fmt.Errorf("进入网易大厅房间：提供的房间密码不正确")
		case packet.TanEnterRoomFullOfPeople:
			return 0, fmt.Errorf("进入网易大厅房间：目标房间（%d）已满", roomID)
		default:
			return 0, fmt.Errorf("进入网易大厅房间：进入房间失败，原因未知（错误码 = %d）", tanEnterRoomResp.ErrorCode)
		}
	}
	tanDebugf("enter room: room accepted")

	// Read and handle incoming packet
	pk, err = readPacketWithContext(ctx, conn, dec)
	if err != nil {
		return 0, fmt.Errorf("进入网易大厅房间：%v", err)
	}
	switch p := pk.(type) {
	case *packet.TanNotifyServerReady:
		remoteNetherNetID, err = strconv.ParseUint(p.NetherNetID, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("进入网易大厅房间：%v", err)
		}
		tanDebugf("enter room: server ready remoteNetherNetID=%d", remoteNetherNetID)
		return remoteNetherNetID, nil
	case *packet.TanKickOutResponse:
		return 0, fmt.Errorf("进入网易大厅房间：房主将您踢出了房间")
	default:
		return 0, fmt.Errorf("进入网易大厅房间：收到未知数据包；pk = %#v", pk)
	}
}

// DialContext ..
func (d *Dialer) DialContext(ctx context.Context) (conn net.Conn, authResp bunker.TanLobbyLoginResponse, err error) {
	// First we query room info
	tanLobbyLoginResp, err := d.Authenticator.GetAccess(d.RoomID)
	if err != nil {
		return nil, bunker.TanLobbyLoginResponse{}, fmt.Errorf("拨号上下文：%v", err)
	}
	if !tanLobbyLoginResp.Success {
		return nil, bunker.TanLobbyLoginResponse{}, fmt.Errorf("拨号上下文：%v", tanLobbyLoginResp.ErrorInfo)
	}
	tanDebugf("auth response: raknet=%s signaling=%s roomOwner=%d player=%s", tanLobbyLoginResp.RaknetServerAddress, tanLobbyLoginResp.SignalingServerAddress, tanLobbyLoginResp.RoomOwnerID, tanLobbyLoginResp.UserPlayerName)

	// Then Enter tan lobby room
	remoteNetherNetID, err := d.enterTanLobbyRoom(ctx, tanLobbyLoginResp)
	if err != nil {
		return nil, bunker.TanLobbyLoginResponse{}, fmt.Errorf("拨号上下文：%v", err)
	}

	// Connect to websocket signaling server
	wsConnection, err := signaling.Dialer{
		Authenticator: d.Authenticator,
		RefreshTime:   signaling.RefreshTimeDisbale,
		NetherNetID:   d.clientNetherID,
	}.DialContext(
		ctx,
		tanLobbyLoginResp.SignalingServerAddress,
		tanLobbyLoginResp.UserUniqueID,
		tanLobbyLoginResp.SignalingSeed,
		tanLobbyLoginResp.SignalingTicket,
	)
	if err != nil {
		return nil, bunker.TanLobbyLoginResponse{}, fmt.Errorf("拨号上下文：%v", err)
	}
	defer wsConnection.Close(fmt.Errorf("拨号上下文：按预期关闭（该错误可忽略）"))
	tanDebugf("signaling: websocket connected localNetherNetID=%d", d.clientNetherID)

	// At last we can connect to remote room
	tanDebugf("nethernet: dialing remoteNetherNetID=%d", remoteNetherNetID)
	conn, err = nethernet.Dialer{}.DialContext(
		ctx,
		remoteNetherNetID,
		wsConnection,
	)
	if err != nil {
		return nil, bunker.TanLobbyLoginResponse{}, fmt.Errorf("拨号上下文：%v", err)
	}

	// Return
	return conn, tanLobbyLoginResp, nil
}
