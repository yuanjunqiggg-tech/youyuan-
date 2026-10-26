package playerrelay

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OmineDev/flowers-for-machines/client"
	"github.com/OmineDev/flowers-for-machines/core/bunker/auth"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/login"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type probeAuthenticator struct {
	base    *authenticator
	address string
}

func (a probeAuthenticator) GetAccess(ctx context.Context, pub []byte) (auth.AuthResponse, error) {
	r, e := a.base.GetAccess(ctx, pub)
	if e == nil {
		r.RentalServerIP = a.address
	}
	return r, e
}

// RunLiveProbe uses a real authenticated protocol client, not fabricated observer
// events. It does not substitute for acceptance on a human NetEase game UI.
func RunLiveProbe(ctx context.Context, c Config, ready func(string)) (map[string]any, error) {
	result := map[string]any{"started_at": time.Now().UTC().Format(time.RFC3339Nano), "human_client_test": false}
	key, e := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if e != nil {
		return result, e
	}
	pub, e := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if e != nil {
		return result, e
	}
	access := &authenticator{cfg: c}
	r, e := access.GetAccess(ctx, pub)
	if e != nil {
		return result, e
	}
	var chain struct {
		Chain []string `json:"chain"`
	}
	if json.Unmarshal([]byte(r.ChainInfo), &chain) != nil {
		return result, fmt.Errorf("认证链格式无效")
	}
	var id login.IdentityData
	for _, raw := range chain.Chain {
		t, e := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.ES384})
		if e != nil {
			continue
		}
		var claims struct {
			Extra login.IdentityData `json:"extraData"`
		}
		if t.UnsafeClaimsWithoutVerification(&claims) == nil && claims.Extra.Identity != "" {
			id = claims.Extra
		}
	}
	// These claims only choose the expected identity. The real gateway listener
	// subsequently verifies the complete certificate chain and client signature.
	if id.Identity == "" {
		return result, fmt.Errorf("认证链未提供玩家 UUID")
	}
	c.AllowedUUID = id.Identity
	c.AllowedUID = id.Uid
	result["player"] = id.DisplayName
	result["player_uuid"] = id.Identity
	result["netease_uid"] = id.Uid
	g := &Gateway{Config: c, access: access}
	gctx, cancel := context.WithCancel(ctx)
	defer cancel()
	addr := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- g.Serve(gctx, func(a string) { addr <- a }) }()
	var address string
	select {
	case address = <-addr:
	case e := <-done:
		return result, e
	case <-ctx.Done():
		return result, ctx.Err()
	}
	if ready != nil {
		ready(address)
	}
	challengeClient, e := auth.CreateClientWithSession(&auth.ClientOptions{AuthServer: c.AuthURL}, access.secret)
	if e != nil {
		cancel()
		<-done
		return result, e
	}
	cl, e := client.LoginRentalServer(client.Config{AuthClient: challengeClient, LoginContext: ctx, AuthServerAddress: c.AuthURL, AuthServerToken: c.GameToken, RentalServerCode: c.Server, RentalServerPasscode: c.ServerPassword, AccessAuthenticator: probeAuthenticator{base: access, address: address}})
	if e != nil {
		cancel()
		<-done
		result["gateway"] = g.Stats.Snapshot()
		return result, fmt.Errorf("协议客户端未完成进服：%s", safeLoginError(e))
	}
	conn := cl.Conn()
	defer conn.Close()
	result["logged_in"] = true
	result["runtime_id"] = fmt.Sprint(conn.GameData().EntityRuntimeID)
	result["spawn_position"] = conn.GameData().PlayerPosition
	result["spawn_dimension"] = conn.GameData().Dimension
	// Drain application packets, preserving one reader and runtime challenges.
	go func() {
		for range cl.CachedPacket() {
		}
		for {
			pk, e := conn.ReadPacket()
			if e != nil {
				return
			}
			if p, ok := pk.(*packet.PyRpc); ok {
				_ = cl.HandleRuntimeChallenge(p)
			}
		}
	}()
	// The caller can teleport this independent client using the main bot, then
	// trigger swings on demand over its existing management API in a later stage.
	sent := 0
	for i := 0; i < 10; i++ {
		if e = conn.WritePacket(&packet.Animate{ActionType: packet.AnimateActionSwingArm, EntityRuntimeID: conn.GameData().EntityRuntimeID}); e != nil {
			break
		}
		if e = conn.Flush(); e != nil {
			break
		}
		sent++
		select {
		case <-ctx.Done():
			e = ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
		if e != nil {
			break
		}
	}
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
	}
	airSent := 0
	if e == nil {
		for i := 0; i < 5; i++ {
			p := &packet.PlayerAuthInput{Position: conn.GameData().PlayerPosition, Pitch: conn.GameData().Pitch, Yaw: conn.GameData().Yaw, HeadYaw: conn.GameData().Yaw, Tick: uint64(i + 1), InputData: protocol.NewBitset(packet.PlayerAuthInputBitsetSize), InputMode: packet.InputModeTouch}
			p.InputData.Set(packet.InputFlagMissedSwing)
			if e = conn.WritePacket(p); e != nil {
				break
			}
			if e = conn.Flush(); e != nil {
				break
			}
			airSent++
			select {
			case <-ctx.Done():
				e = ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
	}
	result["gateway"] = g.Stats.Snapshot()
	result["input_packets_sent"] = sent
	result["missed_swing_packets_sent"] = airSent
	_ = conn.Close()
	cancel()
	<-done
	result["finished_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return result, e
}
