package main

import (
	"strings"
	"testing"
	"time"
)

func TestYYQQTriggerMatchKeyword(t *testing.T) {
	tr := yyQQTrigger{Type: "keyword", Patterns: []string{"赞我", "求赞"}}
	if _, ok := yyQQTriggerMatch(tr, "1121432132", "给我赞我一下", false); !ok {
		t.Fatal("keyword should match")
	}
	if _, ok := yyQQTriggerMatch(tr, "1121432132", "今天天气不错", false); ok {
		t.Fatal("keyword should not match")
	}
	// 大小写不敏感
	tr2 := yyQQTrigger{Type: "keyword", Patterns: []string{"List"}}
	if _, ok := yyQQTriggerMatch(tr2, "1", "发个 list 看看", false); !ok {
		t.Fatal("keyword should be case-insensitive")
	}
}

func TestYYQQTriggerMatchRegex(t *testing.T) {
	tr := yyQQTrigger{Type: "regex", Patterns: []string{`^画(个|一个)\s*(.+)$`}}
	if p, ok := yyQQTriggerMatch(tr, "1", "画个猫", false); !ok || p == "" {
		t.Fatal("regex should match")
	}
	if _, ok := yyQQTriggerMatch(tr, "1", "帮我画个猫", false); ok {
		t.Fatal("anchored regex should not match prefix text")
	}
}

func TestYYQQTriggerMatchAtBot(t *testing.T) {
	tr := yyQQTrigger{Type: "at_bot", Patterns: []string{"在吗"}}
	if _, ok := yyQQTriggerMatch(tr, "1", "在吗", false); ok {
		t.Fatal("at_bot must require mention")
	}
	if _, ok := yyQQTriggerMatch(tr, "1", "在吗", true); !ok {
		t.Fatal("at_bot should match when mentioned")
	}
}

func TestYYQQTriggerMatchAny(t *testing.T) {
	// any = 每条消息都命中，与 patterns 无关。用户实际踩过：一条叫「赞我」
	// 的 any 规则（patterns 赞我/求赞）让全群每句话都触发，只因每人每日 1 次
	// 上限挡着，看起来像「动不动就冒一句回复」。面板选 any 会藏掉触发词输入框
	// 但保留数据，用户根本看不出原因——所以语义必须在测试里锁死。
	tr := yyQQTrigger{Type: "any", Patterns: []string{"赞我", "求赞"}}
	if p, ok := yyQQTriggerMatch(tr, "1", "今天天气不错", false); !ok || p != "" {
		t.Fatalf("any should match every message with empty pattern, got p=%q ok=%v", p, ok)
	}
	// patterns 为空也要命中：老实现把 any 放在 pattern 循环里 return，
	// 空 patterns 时 any 反而不触发，是个隐蔽陷阱。
	tr.Patterns = nil
	if _, ok := yyQQTriggerMatch(tr, "1", "anything", false); !ok {
		t.Fatal("any with empty patterns should still match")
	}
	// any 不看内容，空文本也命中。
	if _, ok := yyQQTriggerMatch(tr, "1", "", false); !ok {
		t.Fatal("any should match empty text")
	}
}

func TestYYQQTriggerGroupScope(t *testing.T) {
	tr := yyQQTrigger{Type: "keyword", Patterns: []string{"hi"}, Groups: []string{"111"}}
	if _, ok := yyQQTriggerMatch(tr, "222", "hi", false); ok {
		t.Fatal("group scope must be enforced")
	}
	if _, ok := yyQQTriggerMatch(tr, "111", "hi", false); !ok {
		t.Fatal("group scope should allow listed group")
	}
	// 空 Groups 表示所有群
	tr.Groups = nil
	if _, ok := yyQQTriggerMatch(tr, "999", "hi", false); !ok {
		t.Fatal("empty group list should mean all groups")
	}
}

func TestYYQQTriggerCooldownAndDaily(t *testing.T) {
	yyQQTriggerState.Lock()
	yyQQTriggerState.last = map[string]time.Time{}
	yyQQTriggerState.used = map[string]int{}
	yyQQTriggerState.day = ""
	yyQQTriggerState.Unlock()

	tr := yyQQTrigger{ID: "t-cooldown", Cooldown: 60, Daily: 2}
	if !yyQQTriggerAllow(tr, "u1") {
		t.Fatal("first call should pass")
	}
	if yyQQTriggerAllow(tr, "u1") {
		t.Fatal("cooldown should block immediate second call")
	}
	// 另一个人不受影响
	if !yyQQTriggerAllow(tr, "u2") {
		t.Fatal("other user should not be blocked")
	}

	// 无冷却时按日限
	tr2 := yyQQTrigger{ID: "t-daily", Daily: 1}
	if !yyQQTriggerAllow(tr2, "u3") {
		t.Fatal("first daily call should pass")
	}
	if yyQQTriggerAllow(tr2, "u3") {
		t.Fatal("daily limit should block")
	}
}

func TestYYQQTriggerRender(t *testing.T) {
	vars := map[string]string{"user": "疣螈", "qq": "2382827225", "arg": "一只猫", "group": "1121432132"}
	got := yyQQTriggerRender("给 {user}({qq}) 画 {arg}", vars)
	want := "给 疣螈(2382827225) 画 一只猫"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// 未知变量保持原样，避免把用户模板吃掉
	if got := yyQQTriggerRender("{nope}", vars); got != "{nope}" {
		t.Fatalf("unknown var should stay, got %q", got)
	}
}

func TestYYQQTriggerPresetsValid(t *testing.T) {
	presets := yyQQTriggerPresets()
	if len(presets) < 4 {
		t.Fatalf("expected several presets, got %d", len(presets))
	}
	for _, p := range presets {
		if p.ID == "" || p.Name == "" || len(p.Patterns) == 0 || len(p.Actions) == 0 {
			t.Fatalf("incomplete preset: %+v", p)
		}
		for _, a := range p.Actions {
			switch a.Type {
			case "reply", "like", "emoji", "command", "relay", "image", "capability":
			default:
				t.Fatalf("unknown action type %q in preset %s", a.Type, p.ID)
			}
		}
	}
}

func TestYYQQTriggerDescribeOnlyEnabled(t *testing.T) {
	desc := yyQQTriggerDescribe([]yyQQTrigger{
		{Enabled: true, Name: "赞我", Patterns: []string{"赞我"}, Actions: []yyQQTriggerAction{{Type: "like"}}},
		{Enabled: false, Name: "关掉的", Patterns: []string{"x"}, Actions: []yyQQTriggerAction{{Type: "reply"}}},
	})
	if !strings.Contains(desc, "赞我") {
		t.Fatal("enabled rule missing")
	}
	if strings.Contains(desc, "关掉的") {
		t.Fatal("disabled rule should not be described")
	}
	if yyQQTriggerDescribe(nil) != "" {
		t.Fatal("empty triggers should produce empty description")
	}
}

func TestYYQQTrimList(t *testing.T) {
	got := yyQQTrimList([]string{" a ", "", "b", strings.Repeat("x", 200)}, 3, 10)
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d (%v)", len(got), got)
	}
	if got[0] != "a" {
		t.Fatalf("trim failed: %q", got[0])
	}
	if got[1] != "b" || len(got[2]) != 10 {
		t.Fatalf("length clamp failed: %v", got)
	}
}
