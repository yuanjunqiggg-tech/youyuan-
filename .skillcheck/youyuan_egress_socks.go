package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// yySOCKS5Dialer implements RFC 1928 UDP ASSOCIATE. It never falls back to
// direct UDP when the configured proxy is unavailable or rejects a request.
type yySOCKS5Dialer struct {
	Address, Username, Password string
}

func yySOCKSAddress(address string) ([]byte, error) {
	host, portText, err := net.SplitHostPort(address)
	port, e := strconv.Atoi(portText)
	if err != nil || e != nil || port < 1 || port > 65535 || host == "" || strings.ContainsAny(host, "%@/\\\x00\r\n") {
		return nil, errors.New("SOCKS5 UDP目标地址无效")
	}
	b := []byte{}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			b = append([]byte{1}, v4...)
		} else {
			b = append([]byte{4}, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return nil, errors.New("SOCKS5 UDP目标域名过长")
		}
		b = append([]byte{3, byte(len(host))}, host...)
	}
	return append(b, byte(port>>8), byte(port)), nil
}

func yyReadSOCKSAddress(r io.Reader) (string, error) {
	var typ [1]byte
	if _, e := io.ReadFull(r, typ[:]); e != nil {
		return "", e
	}
	var host string
	switch typ[0] {
	case 1, 4:
		size := 4
		if typ[0] == 4 {
			size = 16
		}
		b := make([]byte, size)
		if _, e := io.ReadFull(r, b); e != nil {
			return "", e
		}
		host = net.IP(b).String()
	case 3:
		var size [1]byte
		if _, e := io.ReadFull(r, size[:]); e != nil || size[0] == 0 {
			return "", errors.New("invalid SOCKS5 domain")
		}
		b := make([]byte, int(size[0]))
		if _, e := io.ReadFull(r, b); e != nil {
			return "", e
		}
		host = string(b)
	default:
		return "", errors.New("invalid SOCKS5 address type")
	}
	var port [2]byte
	if _, e := io.ReadFull(r, port[:]); e != nil {
		return "", e
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port[:])))), nil
}

// Errors deliberately omit endpoint, username, password and server replies.
func yySOCKSError(stage string, ctx context.Context, e error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("SOCKS5 %s: %w", stage, ctx.Err())
	}
	if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
		return fmt.Errorf("SOCKS5 %s: %w", stage, e)
	}
	return fmt.Errorf("SOCKS5 %s失败", stage)
}

func (d yySOCKS5Dialer) DialContext(ctx context.Context, network, target string) (net.Conn, error) {
	if network != "udp" && network != "udp4" && network != "udp6" {
		return nil, errors.New("SOCKS5游戏出口只支持UDP")
	}
	// RakNet serialises RemoteAddr into its connection handshake, so it must
	// have a concrete IP. Authentication normally supplies a numeric address;
	// hostname targets use the host resolver before the UDP proxy is opened.
	host, port, targetErr := net.SplitHostPort(target)
	if targetErr == nil && net.ParseIP(host) == nil {
		ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if e != nil || len(ips) == 0 {
			return nil, errors.New("游戏UDP目标域名解析失败")
		}
		target = net.JoinHostPort(ips[0].String(), port)
	}
	header, err := yySOCKSAddress(target)
	if err != nil {
		return nil, err
	}
	if _, err = yySOCKSAddress(d.Address); err != nil || len(d.Username) > 255 || len(d.Password) > 255 {
		return nil, errors.New("SOCKS5代理配置无效")
	}
	control, err := (&net.Dialer{}).DialContext(ctx, "tcp", d.Address)
	if err != nil {
		return nil, yySOCKSError("控制连接", ctx, err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = control.Close()
		}
	}()
	stop := context.AfterFunc(ctx, func() { _ = control.Close() })
	defer stop()
	deadline := time.Now().Add(15 * time.Second)
	if t, exists := ctx.Deadline(); exists && t.Before(deadline) {
		deadline = t
	}
	_ = control.SetDeadline(deadline)
	method := byte(0)
	if d.Username != "" || d.Password != "" {
		method = 2
	}
	if _, err = control.Write([]byte{5, 1, method}); err != nil {
		return nil, yySOCKSError("方法协商", ctx, err)
	}
	var reply [2]byte
	if _, err = io.ReadFull(control, reply[:]); err != nil || reply[0] != 5 || reply[1] != method {
		return nil, yySOCKSError("方法协商被拒绝", ctx, err)
	}
	if method == 2 {
		b := append([]byte{1, byte(len(d.Username))}, d.Username...)
		b = append(b, byte(len(d.Password)))
		b = append(b, d.Password...)
		if _, err = control.Write(b); err != nil {
			return nil, yySOCKSError("代理认证", ctx, err)
		}
		if _, err = io.ReadFull(control, reply[:]); err != nil || reply[0] != 1 || reply[1] != 0 {
			return nil, yySOCKSError("代理认证被拒绝", ctx, err)
		}
	}
	// An unspecified client endpoint is permitted by RFC 1928 when the client
	// does not yet know the UDP source port; the TCP connection scopes lifetime.
	if _, err = control.Write([]byte{5, 3, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return nil, yySOCKSError("UDP关联请求", ctx, err)
	}
	var response [3]byte
	if _, err = io.ReadFull(control, response[:]); err != nil || response != [3]byte{5, 0, 0} {
		return nil, yySOCKSError("UDP关联被拒绝", ctx, err)
	}
	relay, err := yyReadSOCKSAddress(control)
	if err != nil {
		return nil, yySOCKSError("UDP关联响应", ctx, err)
	}
	host, port, err = net.SplitHostPort(relay)
	if err != nil || port == "0" {
		return nil, errors.New("SOCKS5 UDP关联返回无效端口")
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host, _, err = net.SplitHostPort(control.RemoteAddr().String())
		if err != nil {
			return nil, errors.New("SOCKS5 UDP关联返回无效地址")
		}
		relay = net.JoinHostPort(host, port)
	}
	udp, err := (&net.Dialer{}).DialContext(ctx, "udp", relay)
	if err != nil {
		return nil, yySOCKSError("UDP中继连接", ctx, err)
	}
	if !stop() || ctx.Err() != nil {
		_ = udp.Close()
		return nil, yySOCKSError("UDP关联取消", ctx, ctx.Err())
	}
	_ = control.SetDeadline(time.Time{})
	c := &yySOCKSUDPConn{Conn: udp, control: control, header: append([]byte{0, 0, 0}, header...), target: target, readBuf: make([]byte, 65535)}
	// The login context is detached now. Cancelling the completed login must
	// not terminate a live game. Closing either transport closes both sides.
	go func() { _, _ = io.Copy(io.Discard, control); _ = c.Close() }()
	ok = true
	return c, nil
}

type yySOCKSUDPConn struct {
	net.Conn
	control   net.Conn
	header    []byte
	target    string
	readMu    sync.Mutex
	readBuf   []byte
	closeOnce sync.Once
}

// Preserve OS error identity (e.g. path MTU/closed/deadline) for RakNet while
// keeping the proxy's private socket address out of user-facing error text.
type yySOCKSIOError struct {
	stage string
	cause error
}

func (e *yySOCKSIOError) Error() string { return "SOCKS5 UDP" + e.stage + "失败" }
func (e *yySOCKSIOError) Unwrap() error { return e.cause }
func (e *yySOCKSIOError) Timeout() bool {
	var n net.Error
	return errors.As(e.cause, &n) && n.Timeout()
}
func (e *yySOCKSIOError) Temporary() bool {
	var n net.Error
	return errors.As(e.cause, &n) && n.Temporary()
}

func (c *yySOCKSUDPConn) RemoteAddr() net.Addr {
	host, port, _ := net.SplitHostPort(c.target)
	n, _ := strconv.Atoi(port)
	return &net.UDPAddr{IP: net.ParseIP(host), Port: n}
}
func (c *yySOCKSUDPConn) Close() error {
	var err error
	c.closeOnce.Do(func() { err = c.Conn.Close(); _ = c.control.Close() })
	return err
}
func (c *yySOCKSUDPConn) Write(p []byte) (int, error) {
	if len(p)+len(c.header) > 65507 {
		return 0, errors.New("SOCKS5 UDP数据报过长")
	}
	b := make([]byte, len(c.header)+len(p))
	copy(b, c.header)
	copy(b[len(c.header):], p)
	n, e := c.Conn.Write(b)
	if e != nil {
		return 0, &yySOCKSIOError{stage: "发送", cause: e}
	}
	if n != len(b) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}
func (c *yySOCKSUDPConn) Read(p []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for {
		n, e := c.Conn.Read(c.readBuf)
		if e != nil {
			return 0, &yySOCKSIOError{stage: "接收", cause: e}
		}
		b := c.readBuf[:n]
		// Fragmented SOCKS UDP packets are unsupported: never deliver fragments
		// to RakNet as though they were complete datagrams.
		if len(b) < 4 || b[0] != 0 || b[1] != 0 || b[2] != 0 {
			continue
		}
		r := bytes.NewReader(b[3:])
		source, err := yyReadSOCKSAddress(r)
		if err != nil {
			continue
		}
		wantedHost, wantedPort, _ := net.SplitHostPort(c.target)
		sourceHost, sourcePort, _ := net.SplitHostPort(source)
		if wantedPort != sourcePort {
			continue
		}
		if ip := net.ParseIP(wantedHost); ip != nil && !ip.Equal(net.ParseIP(sourceHost)) {
			continue
		}
		payload := b[len(b)-r.Len():]
		return copy(p, payload), nil
	}
}
