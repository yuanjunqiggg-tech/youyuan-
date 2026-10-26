package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCommandAliasOverride：玩家别名优先，且群里发别名串能走到对应分支。
func TestCommandAliasOverride(t *testing.T) {
	cf := yyQQConfig{}
	cf.Assistant.CommandAliases = map[string]string{"avatar": ".换头像"}
	if got := yyQQCommandText(cf, "avatar"); got != ".换头像" {
		t.Fatal(got)
	}
	// 命中别名：走到 yyQQCmdAvatar（互通未启用时回「头像设置失败」），
	// 而不是掉到「未知指令」或者原默认指令。
	out := strings.Join(yyQQAssistantCommand(cf, "u", "u", ".换头像 https://example.com/a.png", ""), "\n")
	if !strings.Contains(out, "头像设置失败") {
		t.Fatal("别名没有走到 avatar 分支：", out)
	}
	// 被覆盖后，原默认指令不再命中（返回空 = 没有分支接管）。
	if out := strings.Join(yyQQAssistantCommand(cf, "u", "u", ".改头像 https://example.com/a.png", ""), "\n"); out != "" {
		t.Fatal("默认指令在被覆盖后不应再命中：", out)
	}
}

// TestCommandAliasEmptyFallsBack：空串/纯空白别名按内置默认处理。
func TestCommandAliasEmptyFallsBack(t *testing.T) {
	cf := yyQQConfig{}
	cf.Assistant.CommandAliases = map[string]string{"avatar": "", "relay": "   ", "nick": "\t\n"}
	if got := yyQQCommandText(cf, "avatar"); got != ".改头像" {
		t.Fatal(got)
	}
	if got := yyQQCommandText(cf, "relay"); got != ".通" {
		t.Fatal(got)
	}
	if got := yyQQCommandText(cf, "nick"); got != ".改昵称" {
		t.Fatal(got)
	}
	// 没有任何别名时全部回默认。
	plain := yyQQConfig{}
	for id, want := range map[string]string{
		"relay": ".通", "identity": ".身份", "help": "帮助", "ai": ".AI",
		"like": ".点赞", "bind": "/绑定", "bindgame": ".绑定", "select_group": ".Q群选择",
	} {
		if got := yyQQCommandText(plain, id); got != want {
			t.Fatalf("%s => %s, want %s", id, got, want)
		}
	}
}

// TestAliasDoesNotBreakDefault：无别名时默认指令全部仍命中（抽 5 个代表性指令）。
func TestAliasDoesNotBreakDefault(t *testing.T) {
	cf := yyQQConfig{}
	cases := []struct{ text, want string }{
		{".通 你好", "游戏尚未连接"}, // relay 分支：游戏桥接不在线，但确实被 relay 接管
		{".身份", "昵称："},      // identity 分支
		{".状态", "当前状态"},     // status 分支
		{".点赞 自己", "点赞失败"},  // like 分支：互通未启用，被能力闸门拦住
		{"帮助", "你的权限"},      // help 分支（含 帮助/.帮助//帮助 变体）
		{".帮助", "你的权限"},
		{"/帮助", "你的权限"},
	}
	for _, c := range cases {
		out := strings.Join(yyQQAssistantCommand(cf, "u", "u", c.text, ""), "\n")
		if !strings.Contains(out, c.want) {
			t.Fatalf("%s => %s", c.text, out)
		}
	}
}

// TestHelpUsesAlias：别名改造后帮助文本显示生效指令，不再出现被覆盖的默认值。
func TestHelpUsesAlias(t *testing.T) {
	cf := yyQQConfig{}
	cf.Assistant.CommandAliases = map[string]string{
		"avatar": ".换头像", "relay": ".喊话", "help": "菜单",
	}
	help := yyQQHelp(cf, 2)
	if !strings.Contains(help, ".换头像") {
		t.Fatal("帮助文本没有用别名 .换头像：", help)
	}
	if strings.Contains(help, ".改头像") {
		t.Fatal("帮助文本仍显示被覆盖的默认指令 .改头像")
	}
	if !strings.Contains(help, ".喊话 消息") || !strings.Contains(help, "菜单 / .菜单") {
		t.Fatal("帮助文本没有同步 relay/help 别名：", help)
	}
	// 无别名时帮助保持内置默认（仅超管条目只在够权限的视图里出现）。
	plain := yyQQHelp(yyQQConfig{}, 0)
	if !strings.Contains(plain, ".通 消息") {
		t.Fatal("默认帮助文本不对：", plain)
	}
	if strings.Contains(plain, ".改头像") {
		t.Fatal("成员视图不该出现仅超管的 .改头像")
	}
	if root := yyQQHelp(yyQQConfig{}, 2); !strings.Contains(root, ".改头像") {
		t.Fatal("超管视图应出现 .改头像：", root)
	}
}

// TestTriggerCapabilityExec：type=capability 的动作走能力执行链
// （NapCat 不在线时执行必然失败，但限流留痕证明走的是 capability 分支，
// 而不是「未知动作」被静默丢掉）。
func TestTriggerCapabilityExec(t *testing.T) {
	saved := yyQQ
	defer func() { yyQQ = saved }()
	yyQQ = yyQQConfig{}
	yyQQ.Enabled = true
	yyQQ.Adapter = "onebot"
	yyQQ.Assistant.Enabled = true
	countKind := func(kind string) int {
		n := 0
		for _, r := range yyQQActionRecent() {
			if r.Kind == kind {
				n++
			}
		}
		return n
	}
	before := countKind("like")
	vars := map[string]string{"user": "测试员", "qq": "990001", "group": "1", "role": "member", "arg": "", "pattern": "赞我"}
	rule := yyQQTrigger{
		ID: "t-cap", Enabled: true, Name: "能力点赞", Type: "keyword",
		Patterns: []string{"赞我"},
		Actions:  []yyQQTriggerAction{{Type: "capability", Params: map[string]string{"kind": "like", "arg": "{qq} 2"}}},
	}
	yyQQTriggerExec(yyQQ, rule, vars, "990001", "m1")
	if countKind("like") <= before {
		t.Fatal("capability 动作没有到达 like 能力执行链")
	}
	// 未知能力必须被拒绝，且不产生任何执行痕迹。
	recent := len(yyQQActionRecent())
	bad := yyQQTrigger{
		ID: "t-cap-bad", Enabled: true, Name: "坏规则", Type: "keyword",
		Patterns: []string{"x"},
		Actions:  []yyQQTriggerAction{{Type: "capability", Params: map[string]string{"kind": "nope", "arg": ""}}},
	}
	yyQQTriggerExec(yyQQ, bad, vars, "990001", "m2")
	if len(yyQQActionRecent()) != recent {
		t.Fatal("未知能力不应产生执行痕迹")
	}
}

// TestTriggerCapabilityBanParsesArg：ban 的 arg 约定「QQ号 [分钟数]」，
// 只给 QQ 号时默认 10 分钟；解析失败不进入执行链。
func TestTriggerCapabilityBanParsesArg(t *testing.T) {
	saved := yyQQ
	defer func() { yyQQ = saved }()
	yyQQ = yyQQConfig{}
	yyQQ.Enabled = true
	yyQQ.Adapter = "onebot"
	yyQQ.Assistant.Enabled = true
	yyQQ.Assistant.SuperAdmins = []string{"990002", "990003"}
	yyQQ.Assistant.Caps = map[string]bool{"ban": true}
	rule := func(arg string) yyQQTrigger {
		return yyQQTrigger{
			ID: "t-ban", Enabled: true, Name: "触发词禁言", Type: "keyword",
			Patterns: []string{"骂人词"},
			Actions:  []yyQQTriggerAction{{Type: "capability", Params: map[string]string{"kind": "ban", "arg": arg}}},
		}
	}
	countBan := func() int {
		n := 0
		for _, r := range yyQQActionRecent() {
			if r.Kind == "ban" {
				n++
			}
		}
		return n
	}
	// 「QQ号 5」：解析成功，进入执行链（OneBot 不在线会失败并留痕）。
	before := countBan()
	yyQQTriggerExec(yyQQ, rule("{qq} 5"), map[string]string{"user": "超管", "qq": "990002", "group": "1", "role": "owner"}, "990002", "m1")
	if countBan() <= before {
		t.Fatal("ban 参数解析后没有进入执行链")
	}
	// 只有 QQ 号：默认 10 分钟，同样进入执行链。
	// 换一个调用者：ban 对同一 caller 有 30 秒冷却，同人会直接被限流挡下。
	before = countBan()
	yyQQTriggerExec(yyQQ, rule("{qq}"), map[string]string{"user": "超管2", "qq": "990003", "group": "1", "role": "owner"}, "990003", "m2")
	if countBan() <= before {
		t.Fatal("ban 缺省分钟数没有进入执行链")
	}
	// 参数非法：不进入执行链。
	before = countBan()
	yyQQTriggerExec(yyQQ, rule("不是QQ号"), map[string]string{"user": "超管", "qq": "990002", "group": "1", "role": "owner"}, "990002", "m3")
	if countBan() != before {
		t.Fatal("非法 ban 参数不应进入执行链")
	}
}

// TestTriggerCapabilityOptionsFromRegistry：面板下拉选项来自能力注册表，不写死。
func TestTriggerCapabilityOptionsFromRegistry(t *testing.T) {
	opts := yyQQTriggerCapabilityOptions()
	if len(opts) != len(yyQQCapabilities)-2 {
		t.Fatalf("选项应来自注册表（去掉 face/game_skill），got %d", len(opts))
	}
	seen := map[string]bool{}
	for _, o := range opts {
		if o["value"] == "" || o["label"] == "" {
			t.Fatal("选项缺字段", o)
		}
		if o["value"] == "face" || o["value"] == "game_skill" {
			t.Fatal("只读罗列/游戏指令不应进这条通道", o)
		}
		seen[o["value"]] = true
	}
	if !seen["ban"] || !seen["avatar"] || !seen["kick"] {
		t.Fatal("ban/avatar/kick 必须可被触发词调用")
	}
}

// TestCommandAliasMergePatchSemantics：POST /api/qq/config 的 assistant 段对
// map 是深合并（yyMergePatch 逐 key 递归）——增/改变名直接生效；删一个别名
// 用 null 或空串，最终都落成空串，yyQQCommandText 按空值回默认，等价于删除。
// 这里锁定该语义，防止以后有人把深合并改成整 map 替换时悄悄破坏别名。
func TestCommandAliasMergePatchSemantics(t *testing.T) {
	current := map[string]any{"assistant": map[string]any{"command_aliases": map[string]any{
		"avatar": ".换头像", "like": ".赞",
	}}}
	yyMergePatch(current, map[string]any{"assistant": map[string]any{"command_aliases": map[string]any{
		"avatar": nil, "relay": ".喊话",
	}}})
	raw, _ := json.Marshal(current)
	var next yyQQConfig
	if e := json.Unmarshal(raw, &next); e != nil {
		t.Fatal(e)
	}
	got := next.Assistant.CommandAliases
	if got["avatar"] != "" || got["like"] != ".赞" || got["relay"] != ".喊话" {
		t.Fatalf("深合并语义不对：%+v", got)
	}
	if yyQQCommandText(next, "avatar") != ".改头像" {
		t.Fatal("null 别名应按默认处理")
	}
	if yyQQCommandText(next, "relay") != ".喊话" {
		t.Fatal("新增别名没有生效")
	}
}
