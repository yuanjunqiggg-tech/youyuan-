package main

import (
	"testing"
	"time"
)

func TestYYQQNumericQQ(t *testing.T) {
	for _, ok := range []string{"3199053371", "12345", "1058407697"} {
		if !yyQQNumericQQ(ok) {
			t.Fatalf("%s 应是合法QQ号", ok)
		}
	}
	for _, bad := range []string{"", "1234", "1234567890123", "abc123", "123 45", "3199053371a"} {
		if yyQQNumericQQ(bad) {
			t.Fatalf("%q 不应是合法QQ号", bad)
		}
	}
}

func TestYYQQActionPolicy(t *testing.T) {
	if level, _, _, _ := yyQQActionPolicy("like"); level != 0 {
		t.Fatalf("点赞应对成员开放，得到 %d", level)
	}
	if level, _, _, _ := yyQQActionPolicy("qzone"); level != 1 {
		t.Fatalf("动态应对管理员开放，得到 %d", level)
	}
	for _, kind := range []string{"avatar", "long_nick", "nick", "delete_qzone"} {
		if level, _, _, _ := yyQQActionPolicy(kind); level != 2 {
			t.Fatalf("%s 应对超级管理员开放，得到 %d", kind, level)
		}
	}
}

func TestYYQQActionGateCooldownAndDaily(t *testing.T) {
	gate := &yyQQActionGate{cool: map[string]time.Time{}, gap: map[string]time.Time{}, dayUsed: map[string]int{}}
	now := time.Now()
	if e := gate.allow("like", "u1", "10001", now); e != nil {
		t.Fatalf("首次应放行：%v", e)
	}
	gate.stamp("like", "u1", "10001", now)
	if e := gate.allow("like", "u1", "10001", now.Add(30*time.Second)); e == nil {
		t.Fatal("冷却期内应拒绝")
	}
	if e := gate.allow("like", "u1", "10001", now.Add(3*time.Minute)); e != nil {
		t.Fatalf("冷却结束后应放行：%v", e)
	}
	gate.stamp("like", "u1", "10001", now.Add(3*time.Minute))
	if e := gate.allow("like", "u1", "20002", now.Add(3*time.Minute)); e == nil {
		t.Fatal("同类操作间隔内应拒绝")
	}
	for i := 0; i < 3; i++ {
		gate.succeed("like", "u1")
	}
	if e := gate.allow("like", "u1", "30003", now.Add(10*time.Minute)); e == nil {
		t.Fatal("达到每日上限应拒绝")
	}
	// 换一天重新放行
	if e := gate.allow("like", "u1", "30003", now.Add(30*time.Hour)); e != nil {
		t.Fatalf("跨天后应放行：%v", e)
	}
}

func TestYYQQActionGateLog(t *testing.T) {
	gate := &yyQQActionGate{cool: map[string]time.Time{}, gap: map[string]time.Time{}, dayUsed: map[string]int{}}
	gate.log(yyQQActionRecord{Kind: "qzone", Name: "测试", Detail: "内容", OK: true})
	gate.log(yyQQActionRecord{Kind: "like", Name: "测试", Detail: "QQ1 ×1", OK: false, Error: "失败"})
	if len(gate.recent) != 2 || gate.recent[0].Kind != "like" {
		t.Fatalf("日志应最新在前，得到 %+v", gate.recent)
	}
}
