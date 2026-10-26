package playerrelay

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/bunker/auth"
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/login"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// This test CA is confined to localhost fixtures with authentication disabled.
// The production Gateway never uses it and never disables authentication.
type fixtureAccess struct{ address string }

func (a fixtureAccess) GetAccess(_ context.Context, pub []byte) (auth.AuthResponse, error) {
	k, e := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if e != nil {
		return auth.AuthResponse{}, e
	}
	p, e := x509.ParsePKIXPublicKey(pub)
	if e != nil {
		return auth.AuthResponse{}, e
	}
	s, e := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES384, Key: k}, (&jose.SignerOptions{}).WithHeader("x5u", login.MarshalPublicKey(&k.PublicKey)))
	if e != nil {
		return auth.AuthResponse{}, e
	}
	c := jwt.Claims{Issuer: "NetEase", Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour)), NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute))}
	first, e := jwt.Signed(s).Claims(c).Claims(map[string]any{"identityPublicKey": login.MarshalPublicKey(&k.PublicKey)}).Serialize()
	if e != nil {
		return auth.AuthResponse{}, e
	}
	second, e := jwt.Signed(s).Claims(c).Claims(map[string]any{"identityPublicKey": login.MarshalPublicKey(p.(*ecdsa.PublicKey)), "extraData": login.IdentityData{Identity: "11111111-1111-4111-8111-111111111111", DisplayName: "fixture", Uid: 123}}).Serialize()
	if e != nil {
		return auth.AuthResponse{}, e
	}
	b, _ := json.Marshal(map[string]any{"chain": []string{first, second}})
	return auth.AuthResponse{SuccessStates: true, ChainInfo: string(b), RentalServerIP: a.address}, nil
}

func TestEncryptedLoginOnlyPreservesOpaqueWorldAndInput(t *testing.T) {
	l, e := (minecraft.ListenConfig{LoginOnly: true, AuthenticationDisabled: true, AllowUnknownPackets: true, Compression: packet.NeteaseCompression(), FlushRate: time.Millisecond}).Listen("raknet", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	accepted := make(chan *minecraft.Conn, 1)
	go func() {
		c, e := l.Accept()
		if e == nil {
			accepted <- c.(*minecraft.Conn)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d, reply, e := (minecraft.Dialer{LoginOnly: true, Authenticator: fixtureAccess{l.Addr().String()}, FlushRate: time.Millisecond}).DialContext(ctx, "raknet")
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	if reply.ChainInfo == "" {
		t.Fatal("successful dial lost authentication response")
	}
	var s *minecraft.Conn
	select {
	case s = <-accepted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	defer s.Close()
	_ = s.SetReadDeadline(time.Now().Add(3 * time.Second))
	_ = d.SetReadDeadline(time.Now().Add(3 * time.Second))
	// Includes an unregistered packet. ReadBytes must preserve exact payload and
	// application ordering rather than decode/re-encode or drop custom protocols.
	f := frame(&packet.Unknown{PacketID: 0x3fe, Payload: []byte{9, 8, 7, 6, 5, 0xff}})
	for _, pair := range [][2]*minecraft.Conn{{s, d}, {d, s}} {
		if _, e = pair[0].Write(f); e != nil {
			t.Fatal(e)
		}
		if e = pair[0].Flush(); e != nil {
			t.Fatal(e)
		}
		got, e := pair[1].ReadBytes()
		if e != nil || !bytes.Equal(got, f) {
			t.Fatalf("opaque frame changed %x %v", got, e)
		}
	}
	for i := 0; i < 1000; i++ {
		if _, e = d.Write(f); e != nil {
			t.Fatal(e)
		}
	}
	if e = d.Flush(); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 1000; i++ {
		got, e := s.ReadBytes()
		if e != nil || !bytes.Equal(got, f) {
			t.Fatalf("batch frame %d: %v", i, e)
		}
	}
}

func TestAuthenticatedListenerRejectsFixtureCA(t *testing.T) {
	l, e := (minecraft.ListenConfig{LoginOnly: true, AllowUnknownPackets: true, Compression: packet.NeteaseCompression()}).Listen("raknet", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, _, e := (minecraft.Dialer{LoginOnly: true, Authenticator: fixtureAccess{l.Addr().String()}}).DialContext(ctx, "raknet")
	if c != nil {
		c.Close()
	}
	if e == nil {
		t.Fatal("untrusted certificate root was accepted")
	}
}
