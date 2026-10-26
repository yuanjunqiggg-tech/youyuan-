package main

import (
	"sync"
	"time"
)

// ========== 通知分发器 ==========
//
// 服务端通过心跳 `pending_messages` 推送通知。客户端把消息存进 push_messages.json 后，
// 由本分发器把每条通知：
//   1. 经 hub 转发到 Web UI(/stream) 与悬浮窗(floatingHub → JNI)，前端按 display_mode 渲染；
//   2. 执行需要 Go/游戏内处理的模式(marquee/chat/tellraw)与动作(command/launch_app)；
//   3. 一次性/临时模式在分发后立即 ack；常驻(banner/modal)等用户关闭时由前端 ack。

var (
	notifMu       sync.Mutex
	emittedPush   = map[string]bool{} // 进程内去重：同一常驻通知本进程只分发一次
	pendingBotMsgs []queuedBotMsg     // 机器人未连接时积压的游戏内消息
	botCmdWake    = make(chan struct{}, 1)
)

// queuedBotMsg 积压待发消息：kind 为 cmd(指令) 或 chat(聊天)。
type queuedBotMsg struct {
	kind    string
	payload string
}

func init() {
	go flushBotCommandsLoop()
}

// notifyPayload 发给前端/悬浮窗的通知载荷。
type notifyPayload struct {
	ID          string `json:"id"`
	DisplayMode string `json:"display_mode"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Severity    string `json:"severity"`
	Action      string `json:"action"`
	ActionData  string `json:"action_data"`
	CanDismiss  bool   `json:"can_dismiss"`
}

// resolveDisplayMode 兼容回退：display_mode 为空时按 type 推断。
func resolveDisplayMode(m PushMessage) string {
	if m.DisplayMode != "" {
		return m.DisplayMode
	}
	switch m.Type {
	case "command":
		return "none"
	case "announcement":
		return "banner"
	case "version_update":
		return "popup"
	default:
		return "toast"
	}
}

// isTransient 返回该模式是否分发后即 ack(一次性/临时/游戏内投递)。
func isTransient(mode string) bool {
	switch mode {
	case "toast", "popup", "marquee", "chat", "tellraw", "none":
		return true
	}
	return false
}

// dispatchNewPushMessages 分发新增推送消息。由心跳在 savePushMessages 后调用。
func dispatchNewPushMessages(dataDir string) {
	msgs := loadPushMessages(dataDir)

	// 第一步：取锁收集待处理消息并标记 emitted，随即释放锁。
	// 不能在持锁时执行 executeInGame —— 它会再次加锁(Go 的 Mutex 不可重入)。
	notifMu.Lock()
	var toProcess []PushMessage
	for i := range msgs {
		m := msgs[i]
		if emittedPush[m.ID] {
			continue
		}
		emittedPush[m.ID] = true
		toProcess = append(toProcess, m)
	}
	notifMu.Unlock()

	var toAck []string
	for _, m := range toProcess {
		mode := resolveDisplayMode(m)

		p := notifyPayload{
			ID:          m.ID,
			DisplayMode: mode,
			Type:        m.Type,
			Title:       m.Title,
			Body:        m.Body,
			Severity:    "info",
			Action:      m.Action,
			ActionData:  m.ActionData,
			CanDismiss:  true,
		}
		if hub != nil {
			hub.Emit("notify", mustJSON(p))
		}

		executeInGame(m, mode)

		if isTransient(mode) {
			toAck = append(toAck, m.ID)
		}
	}

	if len(toAck) > 0 {
		ackPushMessages(dataDir, toAck)
		ackPushToServer(toAck)
	}
}

// executeInGame 处理需要游戏内投递/Go 侧执行的模式与动作。
func executeInGame(m PushMessage, mode string) {
	// display_mode 驱动游戏内投递
	switch mode {
	case "marquee":
		startMarquee(MarqueeConfig{Text: m.Body, Width: 40, Speed: 200, Color: "a"})
	case "chat":
		if bm := getBotManager(); bm != nil && bm.IsConnected() {
			bm.sendChatMessage(m.Body)
		} else {
			enqueueBotMessage("chat", m.Body)
		}
	case "tellraw":
		if bm := getBotManager(); bm != nil && bm.IsConnected() {
			bm.SendTellraw(m.Body)
		} else {
			enqueueBotMessage("cmd", tellrawCmd(m.Body))
		}
	}

	// action 驱动
	if m.Action == "command" && m.ActionData != "" {
		enqueueBotMessage("cmd", m.ActionData)
	}
	if m.Action == "launch_app" && m.ActionData != "" {
		launchAndroidApp(m.ActionData)
	}
}

// tellrawCmd 把纯文本包成 tellraw 指令字符串。
func tellrawCmd(text string) string {
	return `tellraw @a {"rawtext":[{"text":` + mustJSON(text) + `}]}`
}

// enqueueBotMessage 入队一条游戏内消息；若机器人已连接则立即发送。
func enqueueBotMessage(kind, payload string) {
	if payload == "" {
		return
	}
	bm := getBotManager()
	if bm != nil && bm.IsConnected() {
		sendBotMessage(bm, kind, payload)
		return
	}
	notifMu.Lock()
	pendingBotMsgs = append(pendingBotMsgs, queuedBotMsg{kind: kind, payload: payload})
	notifMu.Unlock()
	select {
	case botCmdWake <- struct{}{}:
	default:
	}
}

// sendBotMessage 按类型发送一条游戏内消息。
func sendBotMessage(bm *BotManager, kind, payload string) {
	switch kind {
	case "chat":
		bm.sendChatMessage(payload)
	default:
		bm.SendWOCmd(payload)
	}
}

// flushBotCommandsLoop 轮询：机器人连接后发送积压消息。
func flushBotCommandsLoop() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-botCmdWake:
		case <-t.C:
		}
		bm := getBotManager()
		if bm == nil || !bm.IsConnected() {
			continue
		}
		notifMu.Lock()
		if len(pendingBotMsgs) == 0 {
			notifMu.Unlock()
			continue
		}
		msgs := pendingBotMsgs
		pendingBotMsgs = nil
		notifMu.Unlock()
		for _, m := range msgs {
			sendBotMessage(bm, m.kind, m.payload)
		}
	}
}
