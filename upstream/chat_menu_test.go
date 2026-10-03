package main

import "testing"

// 验证玩家输入解析为选项序号(0起)：支持 "1" "2" 及带标点 "1." "1、" 非法返回 -1。
func TestParseMenuChoice(t *testing.T) {
	cases := []struct {
		in    string
		count int
		want  int
	}{
		{"1", 5, 0}, {"2", 5, 1}, {"5", 5, 4},
		{"1.", 5, 0}, {"1、", 5, 0}, {" 3 ", 5, 2}, {"1。", 5, 0},
		{"0", 5, -1}, {"6", 5, -1}, {"abc", 5, -1}, {"", 5, -1},
		{"-1", 5, -1}, {"99999999999999999999", 5, -1}, {"   ", 5, -1},
	}
	for _, c := range cases {
		if got := parseMenuChoice(c.in, c.count); got != c.want {
			t.Errorf("parseMenuChoice(%q,%d)=%d, want %d", c.in, c.count, got, c.want)
		}
	}
}
