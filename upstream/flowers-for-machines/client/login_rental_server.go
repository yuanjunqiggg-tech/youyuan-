package client

import (
	"context"
	"fmt"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/bunker/auth"
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
)

// LoginRentalServer ..
func LoginRentalServer(cfg Config) (client *Client, err error) {
	parent := cfg.LoginContext
	if parent == nil {
		parent = context.Background()
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("向认证服务器请求令牌...")
	}
	authClient := cfg.AuthClient
	if authClient == nil {
		authClient, err = auth.CreateClient(&auth.ClientOptions{
			AuthServer: cfg.AuthServerAddress,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("登录租赁服务器失败：%v", err)
	}

	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("认证客户端已初始化，等待验证服务确认账号...")
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	ctx, cancelFunc := context.WithTimeout(parent, time.Second*600)
	defer cancelFunc()

	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("正在验证账号并请求游戏服务器连接...")
	}
	var authenticator minecraft.Authenticator = auth.NewAccessWrapper(
		authClient,
		cfg.RentalServerCode,
		cfg.RentalServerPasscode,
		cfg.AuthServerToken,
		"", "",
	)
	if cfg.AccessAuthenticator != nil {
		authenticator = cfg.AccessAuthenticator
	}
	conn, botComponent, err := openConnection(ctx, authenticator, cfg)
	if err != nil {
		return nil, fmt.Errorf("登录租赁服务器失败：%v", err)
	}
	// Closing an incomplete connection also interrupts the synchronous
	// challenge reader. Detach this hook before returning a live session.
	stopCancellation := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancellation()

	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("发送初始化包 (登录UID/模组同步)...")
	}

	client = &Client{
		connection:      conn,
		authClient:      authClient,
		BotComponent:    botComponent,
		OnLoginPacket:   cfg.OnLoginPacket,
		OnLoginProgress: cfg.OnLoginProgress,
	}

	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("等待完成服务器挑战...")
	}
	err = NewChallengeSolver(client).CopeChallenge()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("登录租赁服务器失败：%v", err)
	}
	if err = ctx.Err(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !stopCancellation() {
		_ = conn.Close()
		return nil, context.Canceled
	}

	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("挑战通过，登录完成!")
	}

	return client, nil
}
