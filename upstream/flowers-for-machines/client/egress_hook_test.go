package client

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/bunker/auth"
)

type egressHookAuthenticator struct{}

func (egressHookAuthenticator) GetAccess(context.Context, []byte) (auth.AuthResponse, error) {
	return auth.AuthResponse{RentalServerIP: "127.0.0.1:19132"}, nil
}

type egressHookDialer struct {
	called          bool
	target, network string
	fail            error
}

func (d *egressHookDialer) DialContext(_ context.Context, network, target string) (net.Conn, error) {
	d.called = true
	d.target = target
	d.network = network
	return nil, d.fail
}

func TestGameEgressHookReachesRentalUDP(t *testing.T) {
	failure := errors.New("owned fixture stops before UDP connection")
	d := &egressHookDialer{fail: failure}
	selected, reported := true, false
	_, _, e := openConnection(context.Background(), egressHookAuthenticator{}, Config{GameUpstreamDialer: d, DisablePharos: true, OnGameRoute: func(v bool) { selected, reported = v, true }})
	if !errors.Is(e, failure) || !d.called || d.network != "udp" || d.target != "127.0.0.1:19132" {
		t.Fatalf("rental UDP hook not used: called=%v network=%s target=%s error=%v", d.called, d.network, d.target, e)
	}
	if !reported || selected {
		t.Fatal("system/owned proxy unexpectedly selected Pharos")
	}
	t.Log("client.openConnection -> minecraft.Dialer -> RakNet.UpstreamDialer reaches unchanged authenticated UDP target; no external network used")
}
