package main

import (
	"strings"
	"testing"
)

func TestOutgoingChatReadableAndBounded(t *testing.T) {
	target, text, ok := yyReadableChatCommand(`tellraw @a[name="测试 玩家"] {"rawtext":[{"text":"§a菜单\n第二行"}]}`)
	if !ok || target != `@a[name="测试 玩家"]` || text != "§a菜单\n第二行" {
		t.Fatal(target, text, ok)
	}
	bm := &BotManager{logCh: make(chan string, 1)}
	bm.yyChatSubmitted(`tellraw @a {"rawtext":[{"text":"你好"}]}`)
	bm.yyChatSubmitted(`say 被满队列跳过`)
	if !strings.Contains(<-bm.logCh, "[已提交]") {
		t.Fatal("unconfirmed outgoing log wrong")
	}
	if _, _, ok := yyReadableChatCommand("scoreboard players add @s test 1"); ok {
		t.Fatal("FUNCTION nonchat should not flood terminal")
	}
}
