package main

import (
	"fmt"
	"strings"
)

func yyQQValidateAdminIDs(cf yyQQConfig) error {
	if len(cf.Assistant.Nicknames) > 256 || len(cf.Assistant.CompanionKeywords) > 32 {
		return fmt.Errorf("最多256个昵称、32个陪聊关键词")
	}
	for _, n := range cf.Assistant.Nicknames {
		if len([]rune(n)) > 64 || strings.ContainsAny(n, "\r\n") {
			return fmt.Errorf("昵称最多64字符且不能换行")
		}
	}
	for _, n := range cf.Assistant.CompanionKeywords {
		if len([]rune(n)) > 64 {
			return fmt.Errorf("陪聊关键词最多64字符")
		}
	}
	if len(cf.Assistant.CompanionOwner) > 128 {
		return fmt.Errorf("服主标识过长")
	}
	if cf.Adapter != "qqofficial" {
		return nil
	}
	for _, list := range [][]string{cf.Assistant.SuperAdmins, cf.Assistant.Admins} {
		for _, id := range list {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			digits := true
			for _, r := range id {
				if r < '0' || r > '9' {
					digits = false
					break
				}
			}
			if digits {
				return fmt.Errorf("QQ官方不能用QQ号授权。请让用户 @机器人发送 .身份，然后在真实群成员列表授权，或使用超级管理员认领码")
			}
		}
	}
	return nil
}

// This endpoint is available only through the authenticated device API.
func yyQQAuthorizeSeen(user, role string) error {
	if role != "admin" && role != "super" && role != "member" {
		return fmt.Errorf("role 应为 admin/super/member")
	}
	yyQQHelper.Lock()
	_, seen := yyQQHelper.Seen[user]
	yyQQHelper.Unlock()
	if user == "" || !seen {
		return fmt.Errorf("尚未收到这个成员的真实群事件，请先 @机器人发送 .身份")
	}
	return yyQQHelperMutate(func(a *yyQQAssistantConfig) {
		filter := func(list []string) []string {
			out := []string{}
			for _, id := range list {
				if id != user {
					out = append(out, id)
				}
			}
			return out
		}
		a.Admins = filter(a.Admins)
		a.SuperAdmins = filter(a.SuperAdmins)
		if role == "admin" {
			a.Admins = append(a.Admins, user)
		}
		if role == "super" {
			a.SuperAdmins = append(a.SuperAdmins, user)
		}
	})
}

func yyQQResolveMember(cf yyQQConfig, target string) (string, error) {
	if cf.Adapter != "qqofficial" {
		return target, nil
	}
	yyQQHelper.Lock()
	defer yyQQHelper.Unlock()
	if _, ok := yyQQHelper.Seen[target]; ok {
		return target, nil
	}
	found := ""
	for id, name := range yyQQHelper.Seen {
		if alias := cf.Assistant.Nicknames[id]; alias != "" {
			name = alias
		}
		if name == target && name != id {
			if found != "" {
				return "", fmt.Errorf("昵称重复，请在APK真实群成员列表授权")
			}
			found = id
		}
	}
	if found == "" {
		return "", fmt.Errorf("未找到已收到群事件的成员，请在APK真实群成员列表选择；QQ号不能推算官方身份")
	}
	return found, nil
}
