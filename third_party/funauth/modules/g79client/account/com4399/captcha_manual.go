//go:build js || youyuan_manual_captcha

package com4399

import (
	"fmt"
	"net/http"
)

// The web authentication runtime returns the official challenge to its owner.
// It has no native OCR library and must not silently solve a verification step.
func RecognizeCaptcha(string) (string, error) {
	return "", fmt.Errorf("验证码需要账号本人填写")
}
func RecognizeCaptchaWithHTTPClient(string, *http.Client) (string, error) {
	return "", fmt.Errorf("验证码需要账号本人填写")
}
func InitCaptchaEngine(string, string, string) error {
	return fmt.Errorf("本认证运行时使用本人手动验证码")
}
