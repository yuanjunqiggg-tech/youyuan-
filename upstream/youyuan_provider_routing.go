package main

import (
	"bot-apk/config"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Persist and use concrete provider addresses, never UI placeholders.
func yyProviderURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, e := url.Parse(raw)
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("请选择令牌所属的验证服务器，或填写完整 http(s) 地址；自动选择占位符不能作为服务器地址")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func yyMarketCredential(c *config.AppConfig, r *http.Request) string {
	// Authorization authenticates this device/cloud session and must not leave it.
	if v := r.Header.Get("X-YouYuan-Market-Authorization"); v != "" {
		return v
	}
	if c.MarketToken != "" {
		return "Bearer " + c.MarketToken
	}
	auth, e := yyProviderURL(c.AuthURL)
	if e != nil {
		return ""
	}
	market, e := yyProviderURL(yyMarketOrigin(c))
	if e != nil {
		return ""
	}
	if auth == market && c.Token != "" {
		return "Bearer " + c.Token
	}
	return ""
}
