package main

import "testing"

// 验证通道别名统一：ai/魔法指令/0, player/玩家/1, console/控制台/2。
func TestNormalizeCommandChannel(t *testing.T) {
	cases := []struct {
		in   string
		want CommandChannel
		ok   bool
	}{
		{"ai", ChannelAI, true},
		{"魔法指令", ChannelAI, true},
		{"0", ChannelAI, true},
		{"player", ChannelPlayer, true},
		{"玩家", ChannelPlayer, true},
		{"1", ChannelPlayer, true},
		{"console", ChannelConsole, true},
		{"控制台", ChannelConsole, true},
		{"2", ChannelConsole, true},
		{"bogus", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, err := normalizeCommandChannel(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("%q: got %q err=%v, want %q", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("%q: 期望报错，却得到 %q", c.in, got)
		}
	}
}

// 验证三条规范通道合法，非法通道不合法。
func TestCommandChannelValid(t *testing.T) {
	if !ChannelAI.Valid() || !ChannelPlayer.Valid() || !ChannelConsole.Valid() {
		t.Fatal("期望三条规范通道合法")
	}
	if CommandChannel("bogus").Valid() {
		t.Fatal("期望非法通道不合法")
	}
}
