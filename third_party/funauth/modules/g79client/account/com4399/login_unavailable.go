package com4399

import "fmt"

// LoginUnavailableError describes a provider refusal, not a completed login.
type LoginUnavailableError struct{ StatusCode int }

func (e *LoginUnavailableError) Error() string {
	return fmt.Sprintf("4399官方暂未接受本次登录（HTTP %d）。账号未注册成功；请保留密码，稍后重试或在官方客户端完成本人验证后重新接入。", e.StatusCode)
}
