package main

import (
	"bot-apk/config"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/client"
)

type yyEgressProxyConfig struct {
	Address  string `json:"address"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func yyEgressMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "system"
	}
	switch mode {
	case "system", "official", "socks5":
		return mode, nil
	}
	return "", errors.New("游戏出口应为system、official或socks5")
}

func yyLoadEgressProxy() (yyEgressProxyConfig, error) {
	var p yyEgressProxyConfig
	path := config.GameEgressProxyPath()
	if path == "" {
		return p, errors.New("游戏出口配置目录尚未初始化")
	}
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return p, nil
	}
	if e != nil {
		return p, errors.New("无法读取游戏出口私有配置")
	}
	defer f.Close()
	if json.NewDecoder(io.LimitReader(f, 16385)).Decode(&p) != nil {
		return p, errors.New("游戏出口私有配置无效")
	}
	return p, nil
}

func yyValidateEgressProxy(p yyEgressProxyConfig) error {
	if _, e := yySOCKSAddress(p.Address); e != nil || strings.ContainsAny(p.Address, " \t\r\n") {
		return errors.New("代理地址应为host:port，不能含URL或凭据")
	}
	if len(p.Username) > 255 || len(p.Password) > 255 || (p.Username == "" && p.Password != "") {
		return errors.New("代理用户名/密码长度应不超过255字节，密码认证需要用户名")
	}
	return nil
}

func yySaveEgressProxy(p yyEgressProxyConfig) error {
	path := config.GameEgressProxyPath()
	if path == "" {
		return errors.New("游戏出口配置目录尚未初始化")
	}
	b, _ := json.Marshal(p)
	if e := os.WriteFile(path+".tmp", b, 0600); e != nil {
		return errors.New("无法保存游戏出口私有配置")
	}
	if e := os.Rename(path+".tmp", path); e != nil {
		_ = os.Remove(path + ".tmp")
		return errors.New("无法提交游戏出口私有配置")
	}
	return nil
}

type yyEgressSession struct {
	sync.Mutex
	mode, state, transport, errorKind        string
	officialSelected                         bool
	association                              bool
	opened, closed                           uint64
	sent, received, sentBytes, receivedBytes uint64
	updated                                  time.Time
}

func (s *yyEgressSession) update(state, kind string) {
	if s == nil {
		return
	}
	s.Lock()
	s.state, s.errorKind, s.updated = state, kind, time.Now()
	s.Unlock()
}

func (bm *BotManager) EgressStatus() map[string]any {
	hostKind := runtime.GOOS
	scope := "host_system_network"
	if hostKind == "android" {
		scope = "android_host_network"
	}
	result := map[string]any{"mode": "system", "state": "not_started", "transport": "udp", "host_kind": hostKind, "route_scope": scope, "browser_is_udp_egress": false, "player_device_egress_verified": false, "public_ip_verified": false, "udp_association_established": false, "verified_game_session": false, "applies_on": "next_connection"}
	if bm == nil {
		return result
	}
	bm.mu.Lock()
	s, connected, inGame := bm.egress, bm.connected, bm.inGame
	bm.mu.Unlock()
	if s == nil {
		return result
	}
	s.Lock()
	defer s.Unlock()
	result["mode"], result["state"], result["transport"] = s.mode, s.state, s.transport
	result["official_candidate_selected"], result["udp_association_established"] = s.officialSelected, s.association
	result["official_fallback_to_system"] = s.mode == "official" && !s.officialSelected && s.opened > 0
	result["error_kind"], result["updated_at"] = s.errorKind, s.updated.UTC().Format(time.RFC3339Nano)
	result["datagrams_sent"], result["datagrams_received"] = s.sent, s.received
	result["payload_bytes_sent"], result["payload_bytes_received"] = s.sentBytes, s.receivedBytes
	result["open_transports"], result["closed_transports"] = s.opened-s.closed, s.closed
	result["verified_game_session"] = connected && inGame && s.sent > 0 && s.received > 0
	if s.mode == "socks5" {
		result["route_scope"] = "configured_authorized_proxy"
	}
	if s.officialSelected {
		result["route_scope"] = "official_pharos_candidate_via_host_network"
	}
	return result
}

// yyConfigureGameEgress snapshots configuration once per connection. A later
// settings save cannot switch an established game connection's UDP transport.
func (bm *BotManager) yyConfigureGameEgress(cfg *client.Config, server string) error {
	yyConfigMutationMu.Lock()
	defer yyConfigMutationMu.Unlock()
	c := config.LoadConfig()
	mode, e := yyEgressMode(c.GameEgressMode)
	s := &yyEgressSession{mode: mode, state: "configured", transport: "udp", updated: time.Now()}
	bm.mu.Lock()
	bm.egress = s
	bm.mu.Unlock()
	if e != nil {
		s.update("failed", "invalid_mode")
		return e
	}
	if strings.HasPrefix(server, "@") {
		s.Lock()
		s.transport = "tanlobby_webrtc"
		s.Unlock()
		if mode != "system" {
			s.update("unsupported", "tanlobby_egress_unsupported")
			return errors.New("联机房间WebRTC暂不支持指定游戏出口；请选择系统网络，不能静默绕过代理")
		}
		s.update("system_transport_uninstrumented", "")
		return nil
	}
	cfg.DisablePharos = mode != "official"
	cfg.OnGameRoute = func(selected bool) {
		s.Lock()
		s.officialSelected = selected
		s.updated = time.Now()
		s.Unlock()
	}
	var upstream interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	} = &net.Dialer{}
	if mode == "socks5" {
		if !c.GameEgressProxyAuthorized {
			s.update("failed", "proxy_authorization_required")
			return errors.New("使用自有代理前须确认有权使用该出口")
		}
		p, err := yyLoadEgressProxy()
		if err == nil {
			err = yyValidateEgressProxy(p)
		}
		if err != nil {
			s.update("failed", "invalid_proxy_configuration")
			return err
		}
		upstream = yySOCKS5Dialer{Address: p.Address, Username: p.Username, Password: p.Password}
	}
	cfg.GameUpstreamDialer = yyEgressDialer{upstream: upstream, session: s}
	return nil
}

type yyEgressDialer struct {
	upstream interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}
	session *yyEgressSession
}

func (d yyEgressDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.session.update("dialing", "")
	c, e := d.upstream.DialContext(ctx, network, address)
	if e != nil {
		d.session.update("failed", "udp_dial_failed")
		if d.session.mode == "socks5" {
			return nil, e
		}
		return nil, errors.New("游戏系统UDP出口拨号失败")
	}
	d.session.Lock()
	d.session.opened++
	d.session.association = d.session.mode == "socks5"
	d.session.state, d.session.updated = "transport_established", time.Now()
	d.session.Unlock()
	return &yyEgressConn{Conn: c, session: d.session}, nil
}

type yyEgressConn struct {
	net.Conn
	session *yyEgressSession
	once    sync.Once
}

func (c *yyEgressConn) Read(p []byte) (int, error) {
	n, e := c.Conn.Read(p)
	if e == nil {
		c.session.Lock()
		c.session.received++
		c.session.receivedBytes += uint64(n)
		c.session.state = "receiving"
		c.session.updated = time.Now()
		c.session.Unlock()
	}
	return n, e
}
func (c *yyEgressConn) Write(p []byte) (int, error) {
	n, e := c.Conn.Write(p)
	if e == nil {
		c.session.Lock()
		c.session.sent++
		c.session.sentBytes += uint64(n)
		c.session.updated = time.Now()
		c.session.Unlock()
	}
	return n, e
}
func (c *yyEgressConn) Close() error {
	var e error
	c.once.Do(func() {
		e = c.Conn.Close()
		c.session.Lock()
		c.session.closed++
		if c.session.closed == c.session.opened {
			c.session.state = "closed"
		}
		c.session.updated = time.Now()
		c.session.Unlock()
	})
	return e
}

func yyGameEgressHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	yyConfigMutationMu.Lock()
	defer yyConfigMutationMu.Unlock()
	c := config.LoadConfig()
	p, e := yyLoadEgressProxy()
	if e != nil {
		writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
		return
	}
	if r.Method == "POST" {
		var patch struct {
			Mode          *string `json:"mode"`
			Authorized    *bool   `json:"proxy_authorized"`
			Address       string  `json:"proxy_address"`
			Username      string  `json:"proxy_username"`
			Password      string  `json:"proxy_password"`
			ClearPassword bool    `json:"clear_proxy_password"`
			ClearProxy    bool    `json:"clear_proxy"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&patch) != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "无效出口配置JSON"})
			return
		}
		if patch.Mode != nil {
			c.GameEgressMode, e = yyEgressMode(*patch.Mode)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		}
		if patch.Authorized != nil {
			c.GameEgressProxyAuthorized = *patch.Authorized
		}
		changed := patch.ClearProxy || patch.ClearPassword || patch.Address != "" || patch.Username != "" || patch.Password != ""
		if patch.ClearProxy {
			p = yyEgressProxyConfig{}
			c.GameEgressProxyAuthorized = false
		}
		if patch.Address != "" {
			p.Address = strings.TrimSpace(patch.Address)
		}
		if patch.Username != "" {
			p.Username = patch.Username
		}
		if patch.Password != "" {
			p.Password = patch.Password
		}
		if patch.ClearPassword {
			p.Password = ""
		}
		mode, _ := yyEgressMode(c.GameEgressMode)
		if mode == "socks5" || p.Address != "" {
			e = yyValidateEgressProxy(p)
		}
		if e == nil && mode == "socks5" && !c.GameEgressProxyAuthorized {
			e = errors.New("请先确认有权使用此SOCKS5出口")
		}
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if changed {
			e = yySaveEgressProxy(p)
		}
		if e == nil {
			e = config.SaveConfig(c)
		}
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "保存游戏出口配置失败"})
			return
		}
	}
	mode, _ := yyEgressMode(c.GameEgressMode)
	writeJSON(w, map[string]any{"ok": true, "configuration": map[string]any{"mode": mode, "proxy_authorized": c.GameEgressProxyAuthorized, "proxy_configured": p.Address != "", "proxy_address": p.Address, "proxy_username": p.Username, "has_password": p.Password != "", "empty_secret_preserves_existing": true, "applies_on": "next_connection"}, "runtime": getBotManagerLocked().EgressStatus()})
}
