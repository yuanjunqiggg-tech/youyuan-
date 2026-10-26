package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MenuOption 聊天栏菜单的一个选项。
type MenuOption struct {
	Label string // 展示文字
}

// ChatMenu 聊天栏菜单。
type ChatMenu struct {
	Title   string
	Options []MenuOption
}

// parseMenuChoice 解析玩家输入，返回选中的选项序号(0起)。
// 支持 "1" "2" 及带标点 "1." "1、"；越界或非数字返回 -1。
func parseMenuChoice(input string, optionCount int) int {
	input = strings.TrimSpace(input)
	input = strings.TrimSuffix(input, ".")
	input = strings.TrimSuffix(input, "、")
	input = strings.TrimSuffix(input, "。")
	n, err := strconv.Atoi(input)
	if err != nil {
		return -1
	}
	if n < 1 || n > optionCount {
		return -1
	}
	return n - 1
}

// ShowMenu 将菜单以定向 tellraw 形式只发送给指定玩家（绝不广播）。
// 每行一条 tellraw 指令，避免发送给其他玩家，也避免单行过长。
func (bm *BotManager) ShowMenu(player string, menu *ChatMenu) {
	sel := toPlayerSelector(player)
	bm.SendWOCmd(fmt.Sprintf(`tellraw %s {"rawtext":[{"text":%s}]}`, sel, mustJSON("§6"+menu.Title+"§r")))
	for i, opt := range menu.Options {
		line := fmt.Sprintf("§e%d. §r%s", i+1, opt.Label)
		bm.SendWOCmd(fmt.Sprintf(`tellraw %s {"rawtext":[{"text":%s}]}`, sel, mustJSON(line)))
	}
}

// PromptChoice 向玩家展示菜单并等待其输入序号，返回选中选项下标(0起)。
func (bm *BotManager) PromptChoice(player string, menu *ChatMenu, timeout time.Duration) (int, error) {
	bm.ShowMenu(player, menu)
	msg, err := bm.WaitMsg(player, timeout)
	if err != nil {
		return -1, err
	}
	idx := parseMenuChoice(msg, len(menu.Options))
	if idx < 0 {
		return -1, fmt.Errorf("输入无效: %q", msg)
	}
	return idx, nil
}
