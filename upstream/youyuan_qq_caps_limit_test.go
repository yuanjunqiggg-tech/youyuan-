package main

import (
	"testing"
	"time"
)

// 能力限流参数从"写死"改成"玩家可覆盖"，这几个用例守住三件事：
// 1) 没覆盖时行为和以前完全一致；2) 覆盖只影响改过的那一项；
// 3) 执行期的 yyQQActionPolicy 读的是覆盖后的值（否则面板改了等于没改）。
func TestCapLimitDefaultsWhenNoOverride(t *testing.T) {
	a := yyQQAssistantConfig{}
	got, ok := yyQQCapEffectiveOf(a, "like")
	if !ok {
		t.Fatal("like 应该在能力注册表里")
	}
	if got.CoolSeconds != 120 || got.Daily != 3 || got.GapSeconds != 10 || got.MinLevel != 0 {
		t.Fatalf("默认值被改动：%+v", got)
	}
}

func TestCapLimitOverrideOnlyTouchesGivenField(t *testing.T) {
	cool := 0
	a := yyQQAssistantConfig{CapLimits: map[string]yyQQCapLimit{"like": {CoolSeconds: &cool}}}
	got, _ := yyQQCapEffectiveOf(a, "like")
	if got.CoolSeconds != 0 {
		t.Fatalf("冷却应被改成 0，实际 %d", got.CoolSeconds)
	}
	if got.Daily != 3 || got.GapSeconds != 10 || got.MinLevel != 0 {
		t.Fatalf("没改的字段不应变化：%+v", got)
	}
	// 其它能力不受影响
	other, _ := yyQQCapEffectiveOf(a, "qzone")
	if other.CoolSeconds != 300 || other.MinLevel != 1 {
		t.Fatalf("别的能力被误改：%+v", other)
	}
}

func TestCapLimitZeroMeansUnlimitedNotUnset(t *testing.T) {
	zero := 0
	a := yyQQAssistantConfig{CapLimits: map[string]yyQQCapLimit{"qzone": {Daily: &zero}}}
	got, _ := yyQQCapEffectiveOf(a, "qzone")
	if got.Daily != 0 {
		t.Fatalf("daily=0 应表示不限次，实际 %d", got.Daily)
	}
	if (yyQQCapLimit{Daily: &zero}).empty() {
		t.Fatal("给了 daily=0 的覆盖不应被判成空覆盖")
	}
}

func TestCapLimitNormalizeClamps(t *testing.T) {
	neg, big, lv := -5, 999999, 9
	got := yyQQCapLimitNormalize(yyQQCapLimit{CoolSeconds: &neg, Daily: &big, MinLevel: &lv})
	if *got.CoolSeconds != 0 {
		t.Fatalf("负数冷却应夹到 0，实际 %d", *got.CoolSeconds)
	}
	if *got.Daily != 10000 {
		t.Fatalf("超大每日次数应夹到 10000，实际 %d", *got.Daily)
	}
	if *got.MinLevel != 2 {
		t.Fatalf("最低角色应夹到 2，实际 %d", *got.MinLevel)
	}
}

func TestCapLimitDrivesActionPolicy(t *testing.T) {
	old := yyQQSnapshot()
	t.Cleanup(func() {
		yyQQMu.Lock()
		yyQQ.Assistant.CapLimits = old.Assistant.CapLimits
		yyQQMu.Unlock()
	})
	cool, gap, daily, lv := 7, 2, 99, 2
	yyQQMu.Lock()
	yyQQ.Assistant.CapLimits = map[string]yyQQCapLimit{"like": {
		CoolSeconds: &cool, GapSeconds: &gap, Daily: &daily, MinLevel: &lv,
	}}
	yyQQMu.Unlock()

	minLevel, coolD, gapD, dailyN := yyQQActionPolicy("like")
	if minLevel != 2 || coolD != 7*time.Second || gapD != 2*time.Second || dailyN != 99 {
		t.Fatalf("执行期没读到玩家覆盖：level=%d cool=%v gap=%v daily=%d", minLevel, coolD, gapD, dailyN)
	}
}

func TestCapsViewReportsCustom(t *testing.T) {
	a := yyQQAssistantConfig{}
	for _, row := range yyQQCapsView(a) {
		if row["custom"] == true {
			t.Fatalf("没改过参数却标记成 custom：%v", row["kind"])
		}
		if row["defaults"] == nil {
			t.Fatalf("缺少 defaults，前端没法做「恢复默认」：%v", row["kind"])
		}
	}
	cool := 0
	a.CapLimits = map[string]yyQQCapLimit{"like": {CoolSeconds: &cool}}
	for _, row := range yyQQCapsView(a) {
		if row["kind"] == "like" && row["custom"] != true {
			t.Fatal("改过参数的能力应标记 custom")
		}
	}
}
