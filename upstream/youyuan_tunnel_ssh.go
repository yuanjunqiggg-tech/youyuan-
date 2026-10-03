package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var yyPinggyURL = regexp.MustCompile(`https://[a-z0-9.-]+\.(?:pinggy-free\.link|pinggy\.link)`)
var yyPinggyPinMu sync.Mutex

func yyTunnelPinggyLoop(ctx context.Context, origin string) {
	failures := 0
	for ctx.Err() == nil {
		e := yyTunnelPinggy(ctx, origin)
		if ctx.Err() != nil {
			return
		}
		yyTunnel.Lock()
		connected := yyTunnel.State == "已连接"
		yyTunnel.Unlock()
		if connected {
			failures = 0
		}
		delay := yyTunnelRetryDelay(failures, false)
		failures++
		yyTunnelState("重连等待", fmt.Sprintf("SSH隧道：%v；%d秒后重连", e, int(delay/time.Second)))
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func yyPinggyHostKey(root string, key ssh.PublicKey) error {
	yyPinggyPinMu.Lock()
	defer yyPinggyPinMu.Unlock()
	path := filepath.Join(root, "tunnel-pinggy-host.pub")
	encoded := ssh.MarshalAuthorizedKey(key)
	old, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return os.WriteFile(path, encoded, 0600)
	}
	if e != nil {
		return e
	}
	if !bytes.Equal(bytes.TrimSpace(old), bytes.TrimSpace(encoded)) {
		return fmt.Errorf("Pinggy SSH主机指纹改变，已拒绝连接；请核对服务方指纹")
	}
	return nil
}
func yyTunnelPinggy(ctx context.Context, origin string) error {
	yyTunnelState("正在连接备用隧道", "")
	yyTunnel.Lock()
	root := yyTunnel.Root
	yyTunnel.Provider = "Pinggy SSH 443"
	yyTunnel.Unlock()
	raw, e := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", "free.pinggy.io:443")
	if e != nil {
		return e
	}
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(20 * time.Second))
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			raw.Close()
		case <-done:
		}
	}()
	config := &ssh.ClientConfig{User: "qr", Auth: []ssh.AuthMethod{ssh.Password("")}, Timeout: 20 * time.Second, HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error { return yyPinggyHostKey(root, key) }}
	transport, channels, requests, e := ssh.NewClientConn(raw, "free.pinggy.io:443", config)
	if e != nil {
		return e
	}
	_ = raw.SetDeadline(time.Time{})
	client := ssh.NewClient(transport, channels, requests)
	defer client.Close()
	listener, e := client.Listen("tcp", "0.0.0.0:0")
	if e != nil {
		return fmt.Errorf("SSH反向转发：%w", e)
	}
	defer listener.Close()
	// Only forward to the dedicated MCP-only HTTP listener, never the main API port.
	slots := make(chan struct{}, 32)
	go func() {
		for {
			remote, e := listener.Accept()
			if e != nil {
				return
			}
			select {
			case slots <- struct{}{}:
			default:
				remote.Close()
				continue
			}
			go func() {
				defer func() { <-slots }()
				defer remote.Close()
				local, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", origin)
				if e != nil {
					return
				}
				defer local.Close()
				_ = remote.SetDeadline(time.Now().Add(120 * time.Second))
				_ = local.SetDeadline(time.Now().Add(120 * time.Second))
				copied := make(chan struct{})
				go func() { _, _ = io.Copy(remote, local); close(copied) }()
				_, _ = io.Copy(local, remote)
				remote.Close()
				local.Close()
				<-copied
			}()
		}
	}()
	session, e := client.NewSession()
	if e != nil {
		return e
	}
	defer session.Close()
	output, e := session.StdoutPipe()
	if e != nil {
		return e
	}
	session.Stderr = io.Discard
	if e = session.Shell(); e != nil {
		return e
	}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 128*1024)
	for scanner.Scan() {
		line := scanner.Text()
		yyTunnel.Lock()
		if len(line) > 1500 {
			line = line[:1500]
		}
		yyTunnel.Logs = append(yyTunnel.Logs, line)
		if len(yyTunnel.Logs) > 64 {
			yyTunnel.Logs = yyTunnel.Logs[len(yyTunnel.Logs)-64:]
		}
		if address := yyPinggyURL.FindString(strings.ToLower(line)); address != "" {
			yyTunnel.URL = address
			yyTunnel.State = "已连接"
			yyTunnel.Error = ""
		}
		yyTunnel.Unlock()
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if e = scanner.Err(); e != nil {
		return e
	}
	return session.Wait()
}
