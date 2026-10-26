package main

import (
	"fmt"
	"strings"
)

// 拍一拍：群聊拍群友、私聊拍好友。
// NapCat 在不同版本里对这两个动作的命名不一致（group_poke / send_group_poke，
// friend_poke / send_friend_poke），所以先发新名字，只有明确「不认识这个动作」时
// 才回退旧名字；网络超时、权限不足之类的错误不回退，避免同一个动作被重复执行。

// yyPokeCall 是 yyQQOneBotCall 的变量别名，只为测试能打桩（不要直接改它）。
var yyPokeCall = yyQQOneBotCall

// yyPokeGroup 拍一拍群成员。groupID/userID 用字符串数字，与 yyQQActionLike、
// yyQQActionKick 传参习惯一致（OneBot 两种都收，字符串不会在大 QQ 号上丢精度）。
func yyPokeGroup(groupID, userID string) error {
	group := strings.TrimSpace(yyQQGroupString(groupID))
	user := strings.TrimSpace(userID)
	if group == "" {
		return fmt.Errorf("缺少群号，无法拍一拍")
	}
	if user == "" {
		return fmt.Errorf("请指定要拍的 QQ 号")
	}
	params := map[string]any{"group_id": group, "user_id": user}
	_, e := yyPokeCall("group_poke", params)
	if e == nil {
		return nil
	}
	if !yyPokeActionMissing(e) {
		return e
	}
	if _, e2 := yyPokeCall("send_group_poke", params); e2 != nil {
		return fmt.Errorf("%v；回退 send_group_poke 也失败：%v", e, e2)
	}
	return nil
}

// yyPokeFriend 拍一拍好友（私聊）。同样带动作名回退。
func yyPokeFriend(userID string) error {
	user := strings.TrimSpace(userID)
	if user == "" {
		return fmt.Errorf("请指定要拍的 QQ 号")
	}
	params := map[string]any{"user_id": user}
	_, e := yyPokeCall("friend_poke", params)
	if e == nil {
		return nil
	}
	if !yyPokeActionMissing(e) {
		return e
	}
	if _, e2 := yyPokeCall("send_friend_poke", params); e2 != nil {
		return fmt.Errorf("%v；回退 send_friend_poke 也失败：%v", e, e2)
	}
	return nil
}

// yyPokeActionMissing 判断错误是不是「OneBot 那边没有这个动作」。
// 只有这种情况才值得换一个动作名重试；其他错误（超时、风控、权限）重试无意义。
func yyPokeActionMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// OneBot v11 约定的「不支持的 action」retcode 是 1404，NapCat 也沿用。
	if strings.Contains(msg, "1404") {
		return true
	}
	for _, mark := range []string{
		"unsupported action", "unknown action", "action not found", "no such action",
		"action 不存在", "动作不存在", "未知动作", "不支持的动作", "不支持该动作",
	} {
		if strings.Contains(msg, mark) {
			return true
		}
	}
	// 兜底：同时提到 action 和「不支持/未实现/未找到」也算。
	if strings.Contains(msg, "action") {
		for _, mark := range []string{"不支持", "未实现", "未找到", "不存在", "unknown", "unsupported", "not found", "not implemented"} {
			if strings.Contains(msg, mark) {
				return true
			}
		}
	}
	return false
}

// yyPokeTargetArg 从 ".拍" 的参数里解析目标 QQ，支持三种写法：
//   - 空参数 / "自己" / "我"：拍发指令的人
//   - "@12345678" 纯数字 QQ 号（消息文本里的 @ 已经被还原成 @QQ号）
//   - "[CQ:at,qq=12345678]" 原始 CQ 码（部分入口不做文本化）
//
// 返回 ok=false 表示参数不是能识别的 QQ 号，调用方给格式提示。
func yyPokeTargetArg(arg, user string) (string, bool) {
	raw := strings.TrimSpace(arg)
	if raw == "" || raw == "自己" || raw == "我" {
		return user, true
	}
	// CQ 码里的 at 交给现成的解析器，不自己拆逗号。
	for _, qq := range yyQQInboundMentions(yyQQParseCQSegments(raw)) {
		if qq != "all" && yyQQNumericQQ(qq) {
			return qq, true
		}
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return user, true
	}
	target := strings.TrimSpace(strings.TrimPrefix(fields[0], "@"))
	if yyQQNumericQQ(target) {
		return target, true
	}
	return "", false
}

// yyQQCmdPoke .拍 [QQ号或@某人]：受能力开关与限流约束。
func yyQQCmdPoke(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	target, ok := yyPokeTargetArg(arg, user)
	if !ok {
		return reply("格式：.拍 或 .拍 QQ号 或 .拍 @某人")
	}
	group := yyQQGroupString(cf.GroupID)
	if group == "" {
		return reply("还没选群，无法拍一拍。")
	}
	e := yyQQRunAction("poke", user, name, target, "拍 QQ"+target, level, func() (string, error) {
		return "", yyPokeGroup(group, target)
	})
	if e != nil {
		return reply("拍一拍失败：" + e.Error())
	}
	if target == user {
		return reply("拍了拍自己。")
	}
	return reply("拍了拍 QQ" + target + "。")
}
