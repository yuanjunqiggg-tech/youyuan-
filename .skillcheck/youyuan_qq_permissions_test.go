package main

import "testing"

func TestQQPermissionChecksEveryCommand(t *testing.T) {
	var config yyQQConfig
	config.Command.Permission = "管理员"
	config.Command.Whitelist = []string{"/say", "/give @a stone"}
	for _, tc := range []struct {
		input, role string
		allowed     bool
	}{
		{"/say hello./give @a stone 1.", "admin", true},
		{"/say hello./op @a.", "admin", false},
		{"/sayhello bypass", "admin", false},
		{"/give @a stone 1", "member", false},
		{"/say hello", "owner", true},
		{`/say {"text":"hi./op @a."}`, "admin", true},
	} {
		if got := yyQQCommandAllowed(config, "123456", tc.role, tc.input) == nil; got != tc.allowed {
			t.Errorf("%q role %s allowed %v", tc.input, tc.role, got)
		}
	}
	config.Command.Permission = "指定用户"
	config.Command.Users = []string{"official-user-openid"}
	if yyQQCommandAllowed(config, "official-user-openid", "member", "/say hi") != nil {
		t.Fatal("specified user denied")
	}
	if yyQQCommandAllowed(config, "another-user", "owner", "/say hi") == nil {
		t.Fatal("wrong user allowed")
	}
	config.Command.Permission = "禁用"
	if yyQQCommandAllowed(config, "official-user-openid", "owner", "/say hi") == nil {
		t.Fatal("disabled command allowed")
	}
}

func TestQQUserIDsAndMessageTemplates(t *testing.T) {
	if got := yyQQGroupString(float64(1905497212)); got != "1905497212" {
		t.Fatal("QQ user id changed to exponent", got)
	}
	if yyQQOfficialText(" <@!123456> /say hello ") != "/say hello" {
		t.Fatal("command mention retained")
	}
	if yyQQOfficialText("hello <@!123456>") != "hello <@!123456>" {
		t.Fatal("ordinary inline mention removed")
	}
	for _, format := range []string{"[{player}] {message}", "[${玩家}] ${消息}"} {
		if yyQQRenderMessage(format, "Alex", "hello") != "[Alex] hello" {
			t.Fatal("message template failed")
		}
	}
}
