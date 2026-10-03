package client

import (
	"context"
	"fmt"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/bunker/auth"
)

// LoginRentalServer ..
func LoginRentalServer(cfg Config) (client *Client, err error) {
	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("向认证服务器请求令牌...")
	}
	authClient, err := auth.CreateClient(&auth.ClientOptions{
		AuthServer: cfg.AuthServerAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("登录租赁服务器失败：%v", err)
	}

	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("认证客户端已初始化，等待验证服务确认账号...")
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*600)
	defer cancelFunc()

	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("正在验证账号并请求游戏服务器连接...")
	}
	authenticator := auth.NewAccessWrapper(
		authClient,
		cfg.RentalServerCode,
		cfg.RentalServerPasscode,
		cfg.AuthServerToken,
		"", "",
	)
	conn, botComponent, err := openConnection(ctx, authenticator)
	if err != nil {
		return nil, fmt.Errorf("登录租赁服务器失败：%v", err)
	}

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
		return nil, fmt.Errorf("登录租赁服务器失败：%v", err)
	}

	if cfg.OnLoginProgress != nil {
		cfg.OnLoginProgress("挑战通过，登录完成!")
	}

	return client, nil
}
