package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// wordBankItem JSON 词库的一个菜单项。
// 注意：字段名必须用英文导出名——Go 把以中文字符开头的字段视为未导出，
// encoding/json 不会为其赋值（中文字符不通过 unicode.IsUpper）。
type wordBankItem struct {
	Desc         string        `json:"说明"`
	TriggerWords []string      `json:"触发词"`
	ArgDefs      []argHintJSON `json:"需要的参数(没有则填[])"`
	Usage        string        `json:"功能简介"`
	Commands     []string      `json:"触发后执行的指令"`
	OpOnly       bool          `json:"仅OP可用"`
}

// argHintJSON 词库项的参数定义。
type argHintJSON struct {
	Name    string `json:"参数名"`
	Type    string `json:"类型"` // int/str/bool/float
	Default *any   `json:"默认值(没有则为null)"`
}

// wordBankFile 词库文件根。
type wordBankFile struct {
	Items []wordBankItem `json:"菜单项"`
}

// argTypeFromWord 把词库的类型名转成 ArgType。
func argTypeFromWord(s string) ArgType {
	switch strings.ToLower(s) {
	case "int", "整数":
		return ArgTypeInt
	case "float", "小数":
		return ArgTypeFloat
	case "bool", "布尔":
		return ArgTypeBool
	default:
		return ArgTypeStr
	}
}

// loadWordBank 解析 JSON 词库为触发器列表。每个触发词生成一个触发器，
// 其 Func 依次执行指令列表（替换 [玩家名] 与 [参数:N] 占位符）。
func loadWordBank(bm *BotManager, data []byte) ([]*ChatTrigger, error) {
	var f wordBankFile
	if err := json.Unmarshal(data, &f); err != nil {
		debugLog("[词库] 解析失败 err=%v", err)
		return nil, err
	}
	debugLog("[词库] 解析成功 菜单项数=%d", len(f.Items))
	var out []*ChatTrigger
	for _, item := range f.Items {
		debugLog("[词库] 菜单项 简介=%q 触发词=%v 指令数=%d 仅OP=%v", item.Desc, item.TriggerWords, len(item.Commands), item.OpOnly)
		hints := make([]ArgHint, 0, len(item.ArgDefs))
		for _, p := range item.ArgDefs {
			hints = append(hints, ArgHint{Name: p.Name, Type: argTypeFromWord(p.Type), Default: p.Default})
		}
		cmds := item.Commands
		out = append(out, &ChatTrigger{
			TriggerWords: item.TriggerWords,
			Args:         hints,
			Usage:        item.Usage,
			OpOnly:       item.OpOnly,
			Func: func(player string, args []any) {
				for _, cmd := range cmds {
					full := execCommand(cmd, player, args)
					if f := getSendCommandHook(); f != nil {
						_ = f(full)
						continue
					}
					if err := bm.SendCommandNoWait(ChannelPlayer, full); err != nil {
						_ = bm.sendTargeted(player, "§c指令执行失败")
					}
				}
			},
		})
	}
	return out, nil
}

// execCommand 把 [玩家名] 与 [参数:N]（N 从 1 起）占位符替换为实际值。
func execCommand(cmd, player string, args []any) string {
	out := strings.ReplaceAll(cmd, "[玩家名]", player)
	for i, a := range args {
		ph := "[参数:" + strconv.Itoa(i+1) + "]"
		out = strings.ReplaceAll(out, ph, fmt.Sprint(a))
	}
	return out
}
