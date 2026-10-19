package playerrelay

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/bunker/auth"
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/login"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

type Config struct {
	Listen         string            `json:"listen"`
	AllowedUUID    string            `json:"allowed_uuid"`
	AllowedUID     int64             `json:"allowed_uid"`
	AuthURL        string            `json:"auth_url"`
	GameToken      string            `json:"game_token"`
	Server         string            `json:"server"`
	ServerPassword string            `json:"server_password"`
	AuthHeaders    map[string]string `json:"auth_headers"`
	BridgeURL      string            `json:"bridge_url"`
	SourceID       string            `json:"source_id"`
	BridgeToken    string            `json:"bridge_token"`
	BridgeHeaders  map[string]string `json:"bridge_headers"`
	SequenceFile   string            `json:"sequence_file"`
	QueueCapacity  int               `json:"queue_capacity"`
	FlushMS        int               `json:"flush_ms"`
}

func (c *Config) Validate() error {
	if c.Listen == "" {
		c.Listen = "127.0.0.1:19142"
	}
	if c.AllowedUUID == "" && c.AllowedUID <= 0 {
		return fmt.Errorf("必须绑定玩家 UUID 或网易 UID；昵称不能作为认证身份")
	}
	if c.GameToken == "" || c.Server == "" || c.SourceID == "" || c.BridgeToken == "" || c.SequenceFile == "" {
		return fmt.Errorf("缺少认证、服务器或采集桥私有配置")
	}
	for _, raw := range []string{c.AuthURL, c.BridgeURL} {
		u, e := url.Parse(raw)
		if e != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost"))) {
			return fmt.Errorf("认证与采集桥需要 HTTPS，或本机 HTTP")
		}
	}
	for k := range c.BridgeHeaders {
		if strings.EqualFold(k, "X-Youyuan-Observer-Token") {
			return fmt.Errorf("不能覆盖采集桥密钥头")
		}
	}
	if c.QueueCapacity == 0 {
		c.QueueCapacity = 4096
	}
	if c.QueueCapacity < 1 || c.QueueCapacity > 1048576 {
		return fmt.Errorf("queue_capacity 必须为 1..1048576")
	}
	if c.FlushMS == 0 {
		c.FlushMS = 10
	}
	if c.FlushMS < 1 || c.FlushMS > 1000 {
		return fmt.Errorf("flush_ms 必须为 1..1000")
	}
	return nil
}

func SameIdentity(a, b login.IdentityData) bool {
	return a.Identity != "" && a.Identity == b.Identity && a.Uid == b.Uid
}

type authenticator struct {
	cfg    Config
	target string
	mu     sync.Mutex
	secret string
}

func (a *authenticator) GetAccess(ctx context.Context, pub []byte) (auth.AuthResponse, error) {
	for attempt := 0; attempt < 4; attempt++ {
		r, e := a.getAccessOnce(ctx, pub)
		if e == nil || !(strings.Contains(e.Error(), "HTTP 429") || strings.Contains(e.Error(), "请求过于频繁")) {
			return r, e
		}
		if attempt == 3 {
			return r, e
		}
		// Authentication service throttling only. Never delay gameplay packets.
		select {
		case <-ctx.Done():
			return auth.AuthResponse{}, ctx.Err()
		case <-time.After(time.Duration(5<<attempt) * time.Second):
		}
	}
	return auth.AuthResponse{}, fmt.Errorf("认证重试已结束")
}
func (a *authenticator) getAccessOnce(ctx context.Context, pub []byte) (auth.AuthResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	h := &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if a.secret == "" {
		newReq, e := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(a.cfg.AuthURL, "/")+"/api/new", nil)
		if e != nil {
			return auth.AuthResponse{}, fmt.Errorf("认证地址无效")
		}
		for k, v := range a.cfg.AuthHeaders {
			newReq.Header.Set(k, v)
		}
		newReq.Header.Set("User-Agent", "YouYuan-Player-Gateway/0.1")
		sessionResp, e := h.Do(newReq)
		if e != nil {
			return auth.AuthResponse{}, fmt.Errorf("认证会话请求失败")
		}
		secret, e := io.ReadAll(io.LimitReader(sessionResp.Body, 65536))
		_ = sessionResp.Body.Close()
		if e != nil || sessionResp.StatusCode != 200 || len(secret) == 0 {
			return auth.AuthResponse{}, fmt.Errorf("认证会话被拒绝（HTTP %d）", sessionResp.StatusCode)
		}
		a.secret = string(secret)
	}
	b, _ := json.Marshal(auth.AuthRequest{FBToken: a.cfg.GameToken, ServerCode: a.cfg.Server, ServerPassword: a.cfg.ServerPassword, ClientPublicKey: base64.StdEncoding.EncodeToString(pub)})
	r, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(a.cfg.AuthURL, "/")+"/api/phoenix/login", bytes.NewReader(b))
	if e != nil {
		return auth.AuthResponse{}, fmt.Errorf("认证请求构造失败")
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "YouYuan-Player-Gateway/0.1")
	r.Header.Set("Authorization", "Bearer "+a.secret)
	for k, v := range a.cfg.AuthHeaders {
		r.Header.Set(k, v)
	}
	resp, e := h.Do(r)
	if e != nil {
		return auth.AuthResponse{}, fmt.Errorf("认证网络请求失败")
	}
	defer resp.Body.Close()
	var data auth.AuthResponse
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&data) != nil || !data.SuccessStates || data.ChainInfo == "" || data.RentalServerIP == "" {
		reason := safeLoginError(fmt.Errorf("%s", data.Information))
		for _, secret := range append([]string{a.cfg.GameToken, a.secret}, headerSecrets(a.cfg)...) {
			if secret != "" {
				reason = strings.ReplaceAll(reason, secret, "[密钥已隐藏]")
			}
		}
		return auth.AuthResponse{}, fmt.Errorf("认证服务拒绝连接（HTTP %d，code=%d）：%s", resp.StatusCode, data.Translation, reason)
	}
	if data.FBToken != "" {
		a.cfg.GameToken = data.FBToken
	}
	if a.target != "" {
		data.RentalServerIP = a.target
	} // Used only by the private protocol probe.
	return data, nil
}

type Stats struct {
	ObservationError atomic.Value
	Phase            atomic.Value
	SessionError     atomic.Value
	stages           map[uint32]uint64
	logMu            sync.Mutex
	log              []string
	PlayerFrames     atomic.Uint64
	ServerFrames     atomic.Uint64
	DecodeErrors     atomic.Uint64
	Delivered        atomic.Uint64
	Dropped          atomic.Uint64
	Active           atomic.Bool
	LastDelivery     atomic.Int64
	LastError        atomic.Value
}

func (s *Stats) Snapshot() map[string]any {
	phase := "waiting_for_player"
	if v := s.Phase.Load(); v != nil {
		phase = v.(string)
	}
	se := ""
	oe := ""
	if v := s.ObservationError.Load(); v != nil {
		oe = v.(string)
	}
	if v := s.SessionError.Load(); v != nil {
		se = v.(string)
	}
	s.logMu.Lock()
	logs := append([]string{}, s.log...)
	stages := map[uint32]uint64{}
	for k, v := range s.stages {
		stages[k] = v
	}
	s.logMu.Unlock()
	e := ""
	if v := s.LastError.Load(); v != nil {
		e = v.(string)
	}
	return map[string]any{"last_observation_error": oe, "phase": phase, "session_error": se, "handshake_packet_counts": stages, "login_log": logs, "active": s.Active.Load(), "player_frames": s.PlayerFrames.Load(), "server_frames": s.ServerFrames.Load(), "observation_decode_errors": s.DecodeErrors.Load(), "bridge_delivered": s.Delivered.Load(), "bridge_dropped": s.Dropped.Load(), "last_delivery_unix_ms": s.LastDelivery.Load(), "last_error": e, "scope": "authenticated routed player only", "server_execution_confirmed": false}
}

type Gateway struct {
	Config Config
	Stats  Stats
	mu     sync.Mutex
	inUse  bool
	queue  chan Event
	access *authenticator
}

func (g *Gateway) Serve(ctx context.Context, ready func(string)) error {
	if e := g.Config.Validate(); e != nil {
		return e
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	delivery, e := newDelivery(g.Config)
	if e != nil {
		return e
	}
	defer delivery.Close()
	g.queue = make(chan Event, g.Config.QueueCapacity)
	if g.access == nil {
		g.access = &authenticator{cfg: g.Config}
	}
	// Authentication is deliberately enabled; self-signed, unverified identities
	// must not gain a session just by knowing the proxy address.
	l, e := (minecraft.ListenConfig{PacketFunc: func(h packet.Header, _ []byte, _, _ net.Addr) {
		g.Stats.logMu.Lock()
		if g.Stats.stages == nil {
			g.Stats.stages = map[uint32]uint64{}
		}
		g.Stats.stages[h.PacketID]++
		g.Stats.logMu.Unlock()
	}, ErrorLog: slog.New(slog.NewTextHandler(gatewayLog{g}, nil)), LoginOnly: true, MaximumPlayers: 1, AllowUnknownPackets: true, FlushRate: time.Duration(g.Config.FlushMS) * time.Millisecond, Compression: packet.NeteaseCompression()}).Listen("raknet", g.Config.Listen)
	if e != nil {
		return e
	}
	defer l.Close()
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	defer stop()
	senderDone := make(chan struct{})
	go func() { defer close(senderDone); g.deliver(ctx, delivery) }()
	defer func() { cancel(); <-senderDone }()
	if ready != nil {
		ready(l.Addr().String())
	}
	for {
		c, e := l.Accept()
		if e != nil {
			return e
		}
		down := c.(*minecraft.Conn)
		g.mu.Lock()
		busy := g.inUse
		if !busy {
			g.inUse = true
		}
		g.mu.Unlock()
		if busy {
			_ = down.Close()
			continue
		}
		go g.session(ctx, down)
	}
}

func (g *Gateway) session(parent context.Context, down *minecraft.Conn) {
	g.Stats.Phase.Store("authenticating_upstream")
	g.Stats.SessionError.Store("")
	defer func() {
		g.Stats.Active.Store(false)
		g.Stats.Phase.Store("listening")
		g.mu.Lock()
		g.inUse = false
		g.mu.Unlock()
	}()
	defer down.Close()
	id := down.IdentityData()
	if (g.Config.AllowedUUID != "" && id.Identity != g.Config.AllowedUUID) || (g.Config.AllowedUID > 0 && id.Uid != g.Config.AllowedUID) {
		g.Stats.LastError.Store("incoming_identity_rejected")
		g.Stats.SessionError.Store("incoming_identity_rejected")
		return
	}
	loginCtx, cancel := context.WithTimeout(parent, 60*time.Second)
	stopDown := context.AfterFunc(down.Context(), cancel)
	d := minecraft.Dialer{LoginOnly: true, PreserveClientData: true, ClientData: down.ClientData(), Protocol: down.Proto(), Authenticator: g.access, FlushRate: time.Duration(g.Config.FlushMS) * time.Millisecond}
	up, _, e := d.DialContext(loginCtx, "raknet")
	stopDown()
	cancel()
	if e != nil {
		g.Stats.LastError.Store("upstream_login_failed: " + safeLoginError(e))
		g.Stats.SessionError.Store("upstream_login_failed: " + safeLoginError(e))
		return
	}
	defer up.Close()
	if !SameIdentity(id, up.IdentityData()) {
		g.Stats.LastError.Store("upstream_identity_mismatch")
		g.Stats.SessionError.Store("upstream_identity_mismatch")
		return
	}
	g.Stats.Active.Store(true)
	g.Stats.Phase.Store("forwarding")
	ctx, cancelSession := context.WithCancel(parent)
	defer cancelSession()
	stop := context.AfterFunc(ctx, func() { _ = up.Close(); _ = down.Close() })
	defer stop()
	o := new(Observer)
	done := make(chan error, 2)
	go func() { done <- g.pump(down, up, true, o) }()
	go func() { done <- g.pump(up, down, false, o) }()
	firstErr := <-done
	if parent.Err() == nil && firstErr != nil {
		g.Stats.SessionError.Store("session_closed: " + safeLoginError(firstErr))
	}
	cancelSession()
	_ = up.Close()
	_ = down.Close()
	<-done
}

func safeLoginError(e error) string {
	// Network error strings may contain request URLs. Never include credentials
	// or full auth-service response bodies in public diagnostics.
	s := regexp.MustCompile(`https?://[^\s]+`).ReplaceAllString(e.Error(), "[认证地址]")
	s = regexp.MustCompile(`[A-Za-z0-9_-]{40,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`).ReplaceAllString(s, "[JWT已隐藏]")
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}

type gatewayLog struct{ g *Gateway }

func (w gatewayLog) Write(p []byte) (int, error) {
	s := string(p)
	for _, v := range append([]string{w.g.Config.GameToken, w.g.Config.BridgeToken}, headerSecrets(w.g.Config)...) {
		if v != "" {
			s = strings.ReplaceAll(s, v, "[密钥已隐藏]")
		}
	}
	s = safeLoginError(fmt.Errorf("%s", s))
	w.g.Stats.LastError.Store(s)
	w.g.Stats.logMu.Lock()
	w.g.Stats.log = append(w.g.Stats.log, s)
	if len(w.g.Stats.log) > 8 {
		w.g.Stats.log = w.g.Stats.log[len(w.g.Stats.log)-8:]
	}
	w.g.Stats.logMu.Unlock()
	return len(p), nil
}
func headerSecrets(c Config) []string {
	v := []string{}
	for _, x := range c.AuthHeaders {
		v = append(v, x)
	}
	for _, x := range c.BridgeHeaders {
		v = append(v, x)
	}
	return v
}

func (g *Gateway) pump(src, dst *minecraft.Conn, fromPlayer bool, o *Observer) error {
	for {
		frame, e := src.ReadBytes()
		if e != nil {
			return e
		}
		if fromPlayer {
			g.Stats.PlayerFrames.Add(1)
		} else {
			g.Stats.ServerFrames.Add(1)
		}
		// Forward first, in original order and without reserialising application
		// packets. Observation/HTTP latency must never delay normal game traffic.
		if _, e = dst.Write(frame); e != nil {
			return e
		}
		h := packet.Header{}
		if h.Read(bytes.NewBuffer(frame)) == nil && h.PacketID == packet.IDDisconnect {
			_ = dst.Flush()
			return fmt.Errorf("peer_disconnect_packet_forwarded")
		}
		events, e := o.Observe(frame, fromPlayer)
		if e != nil {
			g.Stats.DecodeErrors.Add(1)
			g.Stats.ObservationError.Store(e.Error())
		}
		for _, event := range events {
			select {
			case g.queue <- event:
			default:
				g.Stats.Dropped.Add(1)
			}
		}
	}
}

// ReadPrivateConfig intentionally keeps credentials out of command-line args.
func ReadPrivateConfig(path string) (Config, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return Config{}, e
	}
	var c Config
	if e = json.Unmarshal(b, &c); e != nil {
		return Config{}, fmt.Errorf("私有配置 JSON 无效")
	}
	e = c.Validate()
	return c, e
}
