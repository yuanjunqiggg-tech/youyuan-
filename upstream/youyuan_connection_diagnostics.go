package main

import (
	"strings"
	"time"
)

func yyConnectionErrorKind(message string) string {
	s := strings.ToLower(message)
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	switch {
	case strings.Contains(s, "1005026") || strings.Contains(s, "山头服已过期") || strings.Contains(s, "服务器已过期"):
		return "server_expired"
	case strings.Contains(s, "请先登录") || strings.Contains(s, "token不存在") || strings.Contains(s, "凭据过期") || strings.Contains(s, "invalid token") || strings.Contains(s, "密码错误"):
		return "credential_rejected"
	case strings.Contains(s, "429") || strings.Contains(s, "too many") || strings.Contains(s, "频繁"):
		return "rate_limited"
	case strings.Contains(s, "http 400"):
		return "auth_bad_request"
	case strings.Contains(s, "1010") || strings.Contains(s, "http 403"):
		return "auth_access_denied"
	default:
		return "connection_error"
	}
}

func yyConnectionActionHint(kind string) string {
	switch kind {
	case "server_expired":
		return "认证服务报告服务器已过期，请在网易确认服务器有效期，续期或更换服务器后重新连接。自动重连会保留设置并继续退避尝试。"
	case "credential_rejected":
		return "游戏登录凭据被拒绝，请在账号管理完成原账号登录或安全验证后再连接。"
	case "rate_limited":
		return "认证服务限制请求频率，正在退避等待；请避免反复手动重连。"
	default:
		return ""
	}
}
func yyConnectionRetryDelay(base time.Duration, failures uint64, message string) time.Duration {
	if failures == 0 {
		return base
	}
	kind := yyConnectionErrorKind(message)
	if kind == "connection_error" {
		return base
	}
	if base < 15*time.Second {
		base = 15 * time.Second
	}
	for i := uint64(1); i < failures && base < 5*time.Minute; i++ {
		base *= 2
	}
	if base > 5*time.Minute {
		base = 5 * time.Minute
	}
	return base
}
