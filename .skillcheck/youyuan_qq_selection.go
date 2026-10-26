package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Player choices survive restarts; prompts expire and never reinterpret unrelated chat.
var yyQQPlayerGroups = map[string]string{}

type yyQQGroupChoice struct {
	IDs     []string
	Message string
	Until   time.Time
}

var yyQQChoices = map[string]yyQQGroupChoice{}

func yyQQDisplayName(cf yyQQConfig, user, fallback string) string {
	if n := strings.TrimSpace(cf.Assistant.Nicknames[user]); n != "" {
		return n
	}
	if fallback != "" && fallback != user {
		return fallback
	}
	return "群成员"
}
func yyQQGroupLabel(cf yyQQConfig, id string) string {
	for _, g := range yyQQGroups(cf) {
		if g.ID == id {
			if g.Number != "" {
				return g.Number
			}
			if g.Name != "" {
				return g.Name
			}
		}
	}
	return "互通群"
}
func yyQQGameNotice(player, text string) {
	if player != "" {
		if e := yyQQTellraw(`@a[name=`+strconv.Quote(player)+`]`, text); e != nil {
			yyQQLog("玩家回执失败：" + e.Error())
		}
	}
}
func yyQQSelection(cf yyQQConfig, player, text string, now time.Time) (handled bool, group, message, notice string) {
	choose := yyQQHasPrefix(text, ".Q群选择")
	send := yyQQHasPrefix(text, ".通")
	if !choose && !send {
		return
	}
	handled = true
	groups := []yyQQGroup{}
	for _, g := range yyQQGroups(cf) {
		if g.Enabled {
			groups = append(groups, g)
		}
	}
	if len(groups) == 0 {
		notice = "没有启用的互通群，请联系管理员。"
		return
	}
	yyQQHelper.Lock()
	defer yyQQHelper.Unlock()
	for p, c := range yyQQChoices {
		if now.After(c.Until) {
			delete(yyQQChoices, p)
		}
	}
	if choose {
		arg := strings.TrimSpace(strings.TrimPrefix(text, ".Q群选择"))
		if arg != "" {
			n, e := strconv.Atoi(arg)
			c, ok := yyQQChoices[player]
			if e != nil || !ok || now.After(c.Until) || n < 1 || n > len(c.IDs) {
				notice = "选择已过期或序号无效，请发送 .Q群选择 重新查看列表。"
				return
			}
			group = c.IDs[n-1]
			if !yyQQGroupEnabled(cf, group) {
				notice = "这个群已停用，请发送 .Q群选择 重新选择。"
				return
			}
			if len(yyQQPlayerGroups) >= 4096 && yyQQPlayerGroups[player] == "" {
				notice = "默认群记录已满，请联系管理员。"
				group = ""
				return
			}
			yyQQPlayerGroups[player] = group
			yyQQHelper.Dirty = true
			delete(yyQQChoices, player)
			message = c.Message
			notice = "默认QQ群已设为 " + yyQQGroupLabel(cf, group) + "。以后使用 .通 内容；更换群请发送 .Q群选择。"
			return
		}
	} else {
		message = strings.TrimSpace(strings.TrimPrefix(text, ".通"))
		group = yyQQPlayerGroups[player]
		if group != "" && yyQQGroupEnabled(cf, group) {
			if message == "" {
				notice = "当前默认QQ群：" + yyQQGroupLabel(cf, group) + "。发送 .通 内容；更换请发送 .Q群选择。"
			}
			return
		}
	}
	if len(yyQQChoices) >= 256 && yyQQChoices[player].Until.IsZero() {
		notice = "选群请求较多，请稍后再试。"
		message = ""
		group = ""
		return
	}
	ids := []string{}
	lines := []string{"请选择默认QQ群（5分钟内有效）："}
	for i, g := range groups {
		ids = append(ids, g.ID)
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, yyQQGroupLabel(cf, g.ID)))
	}
	lines = append(lines, "发送 .Q群选择 1 选择第1个群。之后 .通 内容会发往默认群；发送 .Q群选择 可重新选群。")
	if message != "" {
		lines = append(lines, "你的第一条消息已暂存，选择后自动转发。")
	}
	yyQQChoices[player] = yyQQGroupChoice{ids, message, now.Add(5 * time.Minute)}
	notice = strings.Join(lines, "\n")
	message = ""
	group = ""
	return
}
func yyQQGameSelection(cf yyQQConfig, player, text string) bool {
	handled, group, message, notice := yyQQSelection(cf, player, text, time.Now())
	if !handled {
		return false
	}
	if notice != "" {
		yyQQGameNotice(player, notice)
	}
	if group != "" && message != "" {
		format := cf.GameToGroup.Format
		if format == "" {
			format = "<${玩家}> ${消息}"
		}
		job := yyQQOutgoingJob{Text: yyQQRenderMessage(format, player, message), Group: group, Player: player}
		select {
		case yyQQOutgoing <- job:
		default:
			yyQQGameNotice(player, "发送队列已满，消息没有发送，请稍后再试。")
		}
	}
	return true
}
