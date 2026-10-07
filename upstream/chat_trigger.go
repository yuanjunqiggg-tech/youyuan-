package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ArgType 参数类型。
type ArgType int

const (
	ArgTypeStr   ArgType = iota // 字符串
	ArgTypeInt                  // 整数
	ArgTypeFloat                // 浮点数
	ArgTypeBool                 // 布尔
)

// ArgHint 参数提示：名称、类型、可选默认值。
type ArgHint struct {
	Name    string  // 参数名（展示用）
	Type    ArgType // 参数类型
	Default *any    // 默认值；nil 表示必填
}

// parseArgValue 按类型解析原始字符串。
func parseArgValue(raw string, t ArgType) (any, error) {
	switch t {
	case ArgTypeInt:
		return strconv.Atoi(strings.TrimSpace(raw))
	case ArgTypeFloat:
		return strconv.ParseFloat(strings.TrimSpace(raw), 64)
	case ArgTypeBool:
		return strconv.ParseBool(strings.TrimSpace(raw))
	default:
		return raw, nil
	}
}

// matchTriggerWord 判断 msg 是否命中任一触发词。
// 命中时返回触发的词与剩余内联参数文本。命中规则：msg 等于词，或 msg 以"词 空格"开头。
func matchTriggerWord(msg string, words []string) (word, rest string, ok bool) {
	for _, w := range words {
		if msg == w {
			return w, "", true
		}
		if len(msg) > len(w) && strings.HasPrefix(msg, w) && msg[len(w)] == ' ' {
			return w, strings.TrimSpace(msg[len(w)+1:]), true
		}
	}
	return "", "", false
}

// parseArgsFromText 从内联参数文本解析尽可能多的参数。
// 返回已解析值列表与缺失参数的下标（从提示表）。
func parseArgsFromText(rest string, hints []ArgHint) ([]any, []int) {
	fields := strings.Fields(rest)
	vals := make([]any, 0, len(hints))
	missing := make([]int, 0)
	for i, h := range hints {
		if i < len(fields) {
			if v, err := parseArgValue(fields[i], h.Type); err == nil {
				vals = append(vals, v)
				continue
			}
		}
		if h.Default != nil {
			vals = append(vals, *h.Default)
		} else {
			missing = append(missing, i)
			vals = append(vals, nil)
		}
	}
	return vals, missing
}

// splitStringArgs 把触发词后的文本按空白切分成字符串参数列表，用于 RawArgs 触发词（如 Lua）。
func splitStringArgs(rest string) []any {
	fields := strings.Fields(rest)
	args := make([]any, len(fields))
	for i, f := range fields {
		args[i] = f
	}
	return args
}

// ChatTrigger 聊天栏触发项：命中触发词后调用 Func。
type ChatTrigger struct {
	Plugin       string                          // 归属插件 ID（用于热停止/热重载按插件移除）
	TriggerWords []string                        // 触发词列表
	Args         []ArgHint                       // 参数提示
	RawArgs      bool                            // true 时把触发词后全部参数(按空格)传给 Func，不走 ArgHint 解析/交互提问
	Usage        string                          // 功能说明
	OpOnly       bool                            // 是否仅 OP 可用
	Func         func(player string, args []any) // 处理函数
}

// chatTriggers 已注册的触发词引擎（包级，单机器人场景）。
var chatTriggers = struct {
	sync.Mutex
	list []*ChatTrigger
}{}

// pluginExecSem 限制插件 handler（触发词/事件）的并发执行数。
// 聊天刷屏时产生背压（排队等待）而非无限堆积 Lua VM 与 goroutine。
var pluginExecSem = make(chan struct{}, 32)

// acquirePluginExec 占用一个插件执行名额（满则阻塞等待）。
func acquirePluginExec() { pluginExecSem <- struct{}{} }

// releasePluginExec 释放插件执行名额。
func releasePluginExec() { <-pluginExecSem }

// AddChatTrigger 注册一个触发项。
func (bm *BotManager) AddChatTrigger(t *ChatTrigger) {
	debugLog("[触发] AddChatTrigger 注册 插件=%q 触发词=%v usage=%q", t.Plugin, t.TriggerWords, t.Usage)
	chatTriggers.Lock()
	chatTriggers.list = append(chatTriggers.list, t)
	chatTriggers.Unlock()
}

// RemovePluginTriggers 移除指定插件的所有触发项，返回移除数量。
func (bm *BotManager) RemovePluginTriggers(plugin string) int {
	chatTriggers.Lock()
	defer chatTriggers.Unlock()
	kept := chatTriggers.list[:0]
	removed := 0
	for _, t := range chatTriggers.list {
		if t.Plugin == plugin {
			removed++
			continue
		}
		kept = append(kept, t)
	}
	chatTriggers.list = kept
	return removed
}

// ActivePluginIDs 返回当前有触发项在册的插件 ID 列表（去重）。
func (bm *BotManager) ActivePluginIDs() []string {
	chatTriggers.Lock()
	defer chatTriggers.Unlock()
	seen := map[string]bool{}
	var ids []string
	for _, t := range chatTriggers.list {
		if t.Plugin != "" && !seen[t.Plugin] {
			seen[t.Plugin] = true
			ids = append(ids, t.Plugin)
		}
	}
	return ids
}

// isPlayerOp 判断玩家是否 OP（best-effort，通过 PlayerInfoManager）。
func isPlayerOp(player string) bool {
	if pi := playerInfoMgr.GetPlayer(player); pi != nil {
		return pi.IsOP
	}
	return false
}

// sendCommandHook / sendTargetedHook 是可替换的发送钩子，用于测试与隔离。
// 测试中替换它们以捕获指令/定向消息，无需连接真实服务器。
// 读写走 hooksMu + getter/setter：钩子在测试边界被替换，而异步 handler 可能仍在
// 后台 goroutine 里读取——直接读写全局变量会数据竞争（-race 可复现）。
var (
	hooksMu          sync.RWMutex
	sendCommandHook  func(cmd string) error
	sendTargetedHook func(player, text string) error
)

func getSendCommandHook() func(cmd string) error {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	return sendCommandHook
}

func setSendCommandHook(f func(cmd string) error) {
	hooksMu.Lock()
	sendCommandHook = f
	hooksMu.Unlock()
}

func getSendTargetedHook() func(player, text string) error {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	return sendTargetedHook
}

func setSendTargetedHook(f func(player, text string) error) {
	hooksMu.Lock()
	sendTargetedHook = f
	hooksMu.Unlock()
}

// sendTargeted 向指定玩家定向发送一条文本（定向 tellraw，不广播）。
func (bm *BotManager) sendTargeted(player, text string) error {
	debugLog("[发送] sendTargeted player=%s text=%q", player, text)
	if f := getSendTargetedHook(); f != nil {
		return f(player, text)
	}
	// 对齐 Tooldelta say_to：用网易 AI 通道（SendWOCmd/SendAICommand），并用 @a[name=...] 选择器。
	// 之前用 SendWSCommand（标准通道）在网易服不执行，导致指令发出但玩家看不到。
	err := bm.SendWOCmd(fmt.Sprintf(`tellraw %s {"rawtext":[{"text":%s}]}`, toPlayerSelector(player), mustJSON(text)))
	debugLog("[发送] sendTargeted 结果 err=%v", err)
	return err
}

// toPlayerSelector 把玩家名转成目标选择器（对齐 Tooldelta）。
func toPlayerSelector(p string) string {
	if strings.HasPrefix(p, "@") {
		return p
	}
	return fmt.Sprintf(`@a[name=%s]`, mustJSON(p))
}

// ShowTriggers 向玩家定向展示所有可用触发项（仅 OP 项对 OP 可见）。
func (bm *BotManager) ShowTriggers(player string, op bool) {
	chatTriggers.Lock()
	triggers := chatTriggers.list
	chatTriggers.Unlock()
	for _, t := range triggers {
		if t.OpOnly && !op {
			continue
		}
		words := strings.Join(t.TriggerWords, "/")
		line := fmt.Sprintf("§e%s §7- §r%s", words, t.Usage)
		_ = bm.sendTargeted(player, line)
	}
}

// handleChat 匹配触发词并分发。命中后：解析内联参数，缺失参数时交互提问，然后调用 Func。
func (bm *BotManager) handleChat(player, msg string) {
	fireChatEvent(bm, player, msg)
	// WaitMsg owns trigger input, not observation: onChat must still see it.
	if hasWaitMsg(player) {
		return
	}
	debugLog("[触发] handleChat 进入 player=%s msg=%q", player, msg)
	chatTriggers.Lock()
	triggers := chatTriggers.list
	chatTriggers.Unlock()
	debugLog("[触发] 注册表触发词数量: %d", len(triggers))
	for _, t := range triggers {
		if t.OpOnly && !isPlayerOp(player) {
			debugLog("[触发] 跳过仅OP触发词 %v (玩家 %s 非OP)", t.TriggerWords, player)
			continue
		}
		word, rest, ok := matchTriggerWord(msg, t.TriggerWords)
		debugLog("[触发] 尝试匹配 %v → ok=%v word=%q rest=%q 插件=%q", t.TriggerWords, ok, word, rest, t.Plugin)
		if !ok || word == "" {
			continue
		}
		debugLog("[触发] 命中触发词! 插件=%q 词=%q 剩余参数=%q", t.Plugin, word, rest)
		var args []any
		var missing []int
		if t.RawArgs {
			// Lua 等脚本触发词：把触发词后的全部参数(按空格)直接传给 handler，不限制参数个数。
			args = splitStringArgs(rest)
			debugLog("[触发] RawArgs 参数 args=%v", args)
		} else {
			args, missing = parseArgsFromText(rest, t.Args)
			debugLog("[触发] 解析参数 args=%v 缺失下标=%v", args, missing)
		}
		debugLog("[触发] 调用 Func player=%s args=%v 缺失=%v", player, args, missing)
		// 命中后的处理（含交互提问）整体异步派发：askForArgs 会 WaitMsg 等待玩家下一条
		// 回复，而回复必须由收包循环投递——同步执行会阻塞收包循环导致提问永远等不到回复。
		// handler 内调用 game API（发指令等回复）同理。Tooldelta 同样异步派发。
		go func(t *ChatTrigger, player string, args []any, missing []int) {
			defer func() {
				if r := recover(); r != nil {
					pid := t.Plugin
					if pid == "" {
						pid = "(无归属)"
					}
					msg := fmt.Sprintf("[插件] 插件 %s 崩溃(致命): %v", pid, r)
					bm.logCh <- msg
					debugLog("%s", msg)
				}
			}()
			acquirePluginExec()
			defer releasePluginExec()
			if len(missing) > 0 {
				debugLog("[触发] 参数缺失，开始交互提问")
				filled, err := bm.askForArgs(player, t.Args, missing)
				if err != nil {
					debugLog("[触发] 交互提问失败/取消 err=%v", err)
					_ = bm.sendTargeted(player, "§c操作取消或输入无效")
					return
				}
				for i, idx := range missing {
					args[idx] = filled[i]
				}
				debugLog("[触发] 交互提问后 args=%v", args)
			}
			if t.Func != nil {
				t.Func(player, args)
			}
		}(t, player, args, missing)
		return
	}
	debugLog("[触发] 无任何触发词命中")
}

// argTypeLabel 返回参数类型的中文提示。
func argTypeLabel(t ArgType) string {
	switch t {
	case ArgTypeInt:
		return "整数"
	case ArgTypeFloat:
		return "数字(小数)"
	case ArgTypeBool:
		return "true/false"
	default:
		return "文本"
	}
}

// askForArgs 交互式向玩家提问缺失参数，返回按 missing 顺序收集的值。
// 每个参数：定向提示 → 等待玩家输入 → 解析；输入 "取消" 则中止返回 error。
func (bm *BotManager) askForArgs(player string, hints []ArgHint, missing []int) ([]any, error) {
	vals := make([]any, 0, len(missing))
	for _, idx := range missing {
		h := hints[idx]
		_ = bm.sendTargeted(player, fmt.Sprintf("§e请输入参数 %s（%s），输入 §c取消 §e中止：", h.Name, argTypeLabel(h.Type)))
		for {
			msg, err := bm.WaitMsg(player, 30*time.Second)
			if err != nil {
				return nil, err
			}
			if strings.EqualFold(msg, "取消") || strings.EqualFold(msg, "cancel") {
				return nil, errors.New("用户取消")
			}
			v, err := parseArgValue(msg, h.Type)
			if err != nil {
				_ = bm.sendTargeted(player, "§c输入格式不对，请重新输入")
				continue
			}
			vals = append(vals, v)
			break
		}
	}
	return vals, nil
}
