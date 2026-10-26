package client

import (
	"context"
	"github.com/OmineDev/flowers-for-machines/core/bunker/auth"
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/raknet"
	"net"
)

// ------------------------- Config -------------------------

// Config ..
type Config struct {
	GameUpstreamDialer raknet.UpstreamDialer
	DisablePharos      bool
	OnGameRoute        func(officialSelected bool)
	// AccessAuthenticator optionally routes a key-bound login through an owned
	// gateway. Challenge responses still use AuthServerAddress. No identity is
	// changed by this transport override.
	AccessAuthenticator  minecraft.Authenticator
	AuthClient           *auth.Client    // Optional already authenticated challenge client.
	LoginContext         context.Context // Optional lifecycle of the login only.
	AuthServerAddress    string
	AuthServerToken      string
	RentalServerCode     string
	RentalServerPasscode string
	OnLoginPacket        func(packet.Packet)                             // 登录阶段每收到包时回调
	PacketFunc           func(packet.Header, []byte, net.Addr, net.Addr) // Optional caller-owned raw diagnostic hook.
	OnLoginProgress      func(string)                                    // 登录过程日志回调
}

// ------------------------- Client -------------------------

// Client ..
type Client struct {
	connection            *minecraft.Conn
	authClient            *auth.Client
	getCheckNumEverPassed bool
	cachedPacket          chan packet.Packet
	BotComponent          map[string]*int     // 服务器模组列表 (mod UUID → outfit type)
	OnLoginPacket         func(packet.Packet) // 登录阶段每收到包时回调
	OnLoginProgress       func(string)        // 登录过程进度回调
}

// Conn ..
func (c Client) Conn() *minecraft.Conn {
	return c.connection
}

// CachedPacket ..
func (c Client) CachedPacket() chan packet.Packet {
	return c.cachedPacket
}

func (c *Client) fireCallbacks(pk packet.Packet) {
	if c.OnLoginPacket != nil {
		c.OnLoginPacket(pk)
	}
	if c.OnLoginProgress != nil {
		c.OnLoginProgress("正在完成挑战...")
	}
}

// ------------------------- MCPCheckChallengesSolver -------------------------

// MCPCheckChallengesSolver ..
type MCPCheckChallengesSolver struct {
	client *Client
}

// NewChallengeSolver ..
func NewChallengeSolver(client *Client) *MCPCheckChallengesSolver {
	return &MCPCheckChallengesSolver{client: client}
}
