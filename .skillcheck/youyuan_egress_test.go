package main

import (
	"bot-apk/config"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/client"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/raknet"
)

// This is an owned loopback SOCKS5 server, not a public/external proxy. Its
// relay opens a distinct UDP socket to a real loopback target and returns the
// actual target replies in SOCKS5 datagrams.
type yyEgressFixture struct {
	tcp                      net.Listener
	udp                      *net.UDPConn
	up                       *net.UDPConn
	mu                       sync.Mutex
	controls                 []net.Conn
	target                   string
	client                   *net.UDPAddr
	username, password       string
	stall, reject, badFrames atomic.Bool
}

func yyNewEgressFixture(t *testing.T, target, username, password string) *yyEgressFixture {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	u, e := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	up, e := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	f := &yyEgressFixture{tcp: l, udp: u, up: up, target: target, username: username, password: password}
	t.Cleanup(func() {
		_ = l.Close()
		_ = u.Close()
		_ = up.Close()
		f.mu.Lock()
		for _, c := range f.controls {
			_ = c.Close()
		}
		f.mu.Unlock()
	})
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			f.mu.Lock()
			f.controls = append(f.controls, c)
			f.mu.Unlock()
			go f.control(c)
		}
	}()
	go f.relayToTarget()
	go f.relayToClient()
	return f
}
func (f *yyEgressFixture) control(c net.Conn) {
	defer c.Close()
	if f.stall.Load() {
		_, _ = io.Copy(io.Discard, c)
		return
	}
	var greet [3]byte
	if _, e := io.ReadFull(c, greet[:]); e != nil {
		return
	}
	method := byte(0)
	if f.username != "" {
		method = 2
	}
	if greet != [3]byte{5, 1, method} {
		return
	}
	if _, e := c.Write([]byte{5, method}); e != nil {
		return
	}
	if method == 2 {
		var first [2]byte
		if _, e := io.ReadFull(c, first[:]); e != nil {
			return
		}
		u := make([]byte, int(first[1]))
		if _, e := io.ReadFull(c, u); e != nil {
			return
		}
		var size [1]byte
		if _, e := io.ReadFull(c, size[:]); e != nil {
			return
		}
		p := make([]byte, int(size[0]))
		if _, e := io.ReadFull(c, p); e != nil {
			return
		}
		if first[0] != 1 || string(u) != f.username || string(p) != f.password {
			_, _ = c.Write([]byte{1, 1})
			return
		}
		_, _ = c.Write([]byte{1, 0})
	}
	var cmd [3]byte
	if _, e := io.ReadFull(c, cmd[:]); e != nil {
		return
	}
	if _, e := yyReadSOCKSAddress(c); e != nil {
		return
	}
	if cmd != [3]byte{5, 3, 0} {
		return
	}
	if f.reject.Load() {
		_, _ = c.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	addr, _ := yySOCKSAddress(f.udp.LocalAddr().String())
	if _, e := c.Write(append([]byte{5, 0, 0}, addr...)); e != nil {
		return
	}
	_, _ = io.Copy(io.Discard, c)
}
func (f *yyEgressFixture) relayToTarget() {
	b := make([]byte, 65535)
	for {
		n, peer, e := f.udp.ReadFromUDP(b)
		if e != nil {
			return
		}
		if n < 4 {
			continue
		}
		r := bytes.NewReader(b[3:n])
		target, e := yyReadSOCKSAddress(r)
		if e != nil || target != f.target {
			continue
		}
		a, e := net.ResolveUDPAddr("udp", target)
		if e != nil {
			continue
		}
		f.mu.Lock()
		f.client = peer
		f.mu.Unlock()
		_, _ = f.up.WriteToUDP(b[n-r.Len():n], a)
	}
}
func (f *yyEgressFixture) relayToClient() {
	b := make([]byte, 65535)
	for {
		n, source, e := f.up.ReadFromUDP(b)
		if e != nil {
			return
		}
		f.mu.Lock()
		peer := f.client
		f.mu.Unlock()
		if peer == nil {
			continue
		}
		h, _ := yySOCKSAddress(source.String())
		out := append(append([]byte{0, 0, 0}, h...), b[:n]...)
		if f.badFrames.Load() {
			fragment := append([]byte{}, out...)
			fragment[2] = 1
			_, _ = f.udp.WriteToUDP(fragment, peer)
			_, _ = f.udp.WriteToUDP([]byte{0, 0, 0, 99}, peer)
		}
		_, _ = f.udp.WriteToUDP(out, peer)
	}
}

func TestYYEgressSOCKS5RealUDPRoundTrip(t *testing.T) {
	target, e := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer target.Close()
	f := yyNewEgressFixture(t, target.LocalAddr().String(), "fixture-user", "fixture-private-password")
	f.badFrames.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	d := yySOCKS5Dialer{Address: f.tcp.Addr().String(), Username: f.username, Password: f.password}
	c, e := d.DialContext(ctx, "udp", target.LocalAddr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	cancel() // Completed login context must not close a live game transport.
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	payload := bytes.Repeat([]byte{0, 1, 2, 255}, 800)
	if n, e := c.Write(payload); e != nil || n != len(payload) {
		t.Fatalf("write=%d,%v", n, e)
	}
	_ = target.SetReadDeadline(time.Now().Add(3 * time.Second))
	b := make([]byte, 65535)
	n, source, e := target.ReadFromUDP(b)
	if e != nil || !bytes.Equal(b[:n], payload) {
		t.Fatalf("target payload %d,%v", n, e)
	}
	if source.String() != f.up.LocalAddr().String() || source.String() == c.LocalAddr().String() {
		t.Fatalf("target did not see relay socket")
	}
	_, _ = target.WriteToUDP(b[:n], source)
	n, e = c.Read(b)
	if e != nil || !bytes.Equal(b[:n], payload) {
		t.Fatalf("response %d,%v", n, e)
	}
	if c.RemoteAddr().String() != target.LocalAddr().String() {
		t.Fatal("RakNet must see target RemoteAddr")
	}
	t.Logf("3200-byte real UDP round trip through owned SOCKS5 relay; target source equals relay; fragmented/malformed frames ignored")
}

func TestYYEgressSOCKS5RakNetConnection(t *testing.T) {
	l, e := (raknet.ListenConfig{}).Listen("127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	f := yyNewEgressFixture(t, l.Addr().String(), "", "")
	s := &yyEgressSession{mode: "socks5"}
	d := raknet.Dialer{UpstreamDialer: yyEgressDialer{session: s, upstream: yySOCKS5Dialer{Address: f.tcp.Addr().String()}}}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, e := l.Accept()
		if e == nil {
			accepted <- c
		}
	}()
	c, e := d.DialContext(ctx, l.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	var server net.Conn
	select {
	case server = <-accepted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	defer server.Close()
	_ = c.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, e = c.Write([]byte{0xfe, 1, 2, 3}); e != nil {
		t.Fatal(e)
	}
	_ = server.SetReadDeadline(time.Now().Add(3 * time.Second))
	b := make([]byte, 100)
	n, e := server.Read(b)
	if e != nil || !bytes.Equal(b[:n], []byte{0xfe, 1, 2, 3}) {
		t.Fatalf("RakNet payload=%x,%v", b[:n], e)
	}
	s.Lock()
	sent, received, assoc := s.sent, s.received, s.association
	s.Unlock()
	if sent == 0 || received == 0 || !assoc {
		t.Fatal("missing actual route counters")
	}
	t.Logf("RakNet established and application payload delivered through actual SOCKS UDP relay (sent=%d received=%d)", sent, received)
}

func TestYYEgressSOCKS5CancellationAndNoFallback(t *testing.T) {
	f := yyNewEgressFixture(t, "127.0.0.1:9", "", "")
	f.stall.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, e := (yySOCKS5Dialer{Address: f.tcp.Addr().String()}).DialContext(ctx, "udp", "127.0.0.1:9")
	if e == nil || time.Since(started) > 2*time.Second {
		t.Fatalf("cancel=%v", e)
	}
	f2 := yyNewEgressFixture(t, "127.0.0.1:9", "private-user", "private-password")
	f2.reject.Store(true)
	_, e = (yySOCKS5Dialer{Address: f2.tcp.Addr().String(), Username: f2.username, Password: f2.password}).DialContext(context.Background(), "udp", "127.0.0.1:9")
	if e == nil || strings.Contains(e.Error(), f2.username) || strings.Contains(e.Error(), f2.password) || strings.Contains(e.Error(), f2.tcp.Addr().String()) {
		t.Fatalf("rejection not safely reported: %v", e)
	}
	t.Log("Stalled negotiation cancelled; UDP rejection fails closed without direct fallback; credentials omitted from errors")
}

func TestYYEgressSOCKS5ControlLifetimeAndDeadlines(t *testing.T) {
	f := yyNewEgressFixture(t, "127.0.0.1:9", "", "")
	c, e := (yySOCKS5Dialer{Address: f.tcp.Addr().String()}).DialContext(context.Background(), "udp", "127.0.0.1:9")
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
	_, e = c.Read(make([]byte, 20))
	var ne net.Error
	if !errors.As(e, &ne) || !ne.Timeout() {
		t.Fatalf("timeout identity lost: %v", e)
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	f.mu.Lock()
	for _, control := range f.controls {
		_ = control.Close()
	}
	f.mu.Unlock()
	_, e = c.Read(make([]byte, 20))
	if !errors.Is(e, net.ErrClosed) {
		t.Fatalf("closing control did not close UDP association: %v", e)
	}
	t.Log("Read deadlines remain net.Error timeouts; closing SOCKS TCP control closes UDP and retains net.ErrClosed identity")
}

func TestYYEgressPrivateConfigAndRuntime(t *testing.T) {
	config.InitConfig(t.TempDir())
	post := func(body string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/youyuan/game-egress", strings.NewReader(body))
		yyGameEgressHandler(w, r)
		var v map[string]any
		if json.Unmarshal(w.Body.Bytes(), &v) != nil {
			t.Fatal(w.Body.String())
		}
		return v
	}
	v := post(`{"mode":"socks5","proxy_authorized":true,"proxy_address":"127.0.0.1:10080","proxy_username":"private-name","proxy_password":"private-secret"}`)
	if v["ok"] != true {
		t.Fatal(v)
	}
	if strings.Contains(string(yyEgressTestJSON(t, v)), "private-secret") {
		t.Fatal("password disclosed")
	}
	v = post(`{"mode":"socks5","proxy_password":"","proxy_username":"","proxy_address":""}`)
	if v["ok"] != true {
		t.Fatal(v)
	}
	p, e := yyLoadEgressProxy()
	if e != nil || p.Password != "private-secret" || p.Username != "private-name" {
		t.Fatal("empty UI values erased saved proxy")
	}
	c := config.LoadConfig()
	if strings.Contains(string(yyEgressTestJSON(t, c)), "private-secret") || strings.Contains(string(yyEgressTestJSON(t, c)), "private-name") {
		t.Fatal("proxy credentials leaked into normal config")
	}
	bm := NewBotManager()
	var cfg client.Config
	if e := bm.yyConfigureGameEgress(&cfg, "12345678"); e != nil {
		t.Fatal(e)
	}
	status := bm.EgressStatus()
	if status["verified_game_session"] != false || status["udp_association_established"] != false {
		t.Fatal("unopened route reported verified")
	}
	if strings.Contains(string(yyEgressTestJSON(t, status)), "private-") {
		t.Fatal("secret in runtime")
	}
	if e := bm.yyConfigureGameEgress(nil, "@12345"); e == nil {
		t.Fatal("TanLobby proxy silently bypassed")
	}
	if bm.EgressStatus()["state"] != "unsupported" {
		t.Fatal("unsupported route status missing")
	}
	v = post(`{"mode":"system","clear_proxy_password":true}`)
	if v["ok"] != true {
		t.Fatal(v)
	}
	p, _ = yyLoadEgressProxy()
	if p.Password != "" {
		t.Fatal("explicit password clearing failed")
	}
	t.Log("Private password retained on empty save, explicitly clearable, absent from normal config/runtime; unopened route and TanLobby not falsely verified")
}
func yyEgressTestJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
