package main

import "strings"

// 助手指令格式玩家可配（用户第 16 条第 3 点：「这种限制的要让用户自己去配置」）。
// 指令 ID → 内置默认指令串；玩家用 assistant.command_aliases 覆盖后，
// 群指令匹配（yyQQAssistantCommand）、帮助文本（yyQQHelp）、能力清单
// （yyQQCmdActions）、游戏侧 .通/.Q群选择/.绑定 门禁全部以生效值为准。
var yyQQCommandDefaults = map[string]string{
	"relay":        ".通",
	"identity":     ".身份",
	"help":         "帮助",
	"status":       ".状态",
	"ai":           ".AI",
	"actions":      ".能力",
	"skills":       ".技能",
	"poke":         ".拍",
	"sticker":      ".发图",
	"voice":        ".语音",
	"faces":        ".表情包",
	"emoji":        ".表情",
	"like":         ".点赞",
	"qzone":        ".动态",
	"delete_qzone": ".删动态",
	"avatar":       ".改头像",
	"long_nick":    ".改签名",
	"nick":         ".改昵称",
	"kick":         ".踢",
	"summary":      ".总结",
	"claim":        ".认领",
	"log":          ".日志",
	"group_log":    ".群日志",
	"query":        ".查询",
	"bind":         "/绑定",
	"unbind":       "/解绑",
	"speak":        "/说",
	"authorize":    "/授权",
	"revoke":       "/撤权",
	"command":      "/命令",
	"replay":       ".再来一次",
	"bindgame":     ".绑定",
	"permission":   ".权限",
	"player":       ".玩家",
	// select_group 是游戏内选默认 QQ 群的指令（不在群助手里，属游戏→群方向），
	// 同样交给玩家配。
	"select_group": ".Q群选择",
}

// yyQQCommandText 取生效指令串：玩家别名优先；别名为空串或纯空白时回内置默认。
func yyQQCommandText(cf yyQQConfig, id string) string {
	if v, ok := cf.Assistant.CommandAliases[id]; ok {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return yyQQCommandDefaults[id]
}
