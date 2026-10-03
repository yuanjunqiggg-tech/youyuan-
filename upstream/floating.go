package main

import (
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// floatingHub 接收 SSEHub.Emit 转发的事件，分发给悬浮窗（WS 客户端 + JNI 回调）。
var floatingHub = NewSSEHub()

// 初始化：把主 hub 的转发接到 floatingHub，并启动每秒状态推送。
func initFloatingHub() {
	hub.secondary = floatingHub
	startStatusTicker()
}

// s2c 消息信封
type wsEnvelope struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// handleFloatingWS 升级 /ws 为 WebSocket，订阅 floatingHub 推送，并处理 Java 上行命令。
func handleFloatingWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.CloseNow()

	ch := make(chan string, 200)
	floatingHub.Add(ch)
	defer floatingHub.Remove(ch)

	// 连接即发初始快照
	floatingHub.Emit("status", mustJSON(snapshotStatus()))

	ctx := r.Context()
	go func() {
		for msg := range ch {
			event, data := splitSSEMessage(msg)
			c.Write(ctx, websocket.MessageText, []byte(mustJSON(wsEnvelope{Type: event, Data: json.RawMessage(data)})))
		}
	}()

	for {
		_, raw, err := c.Read(ctx)
		if err != nil {
			return
		}
		var env wsEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			continue
		}
		handleFloatingCommand(env.Type, env.Data)
	}
}

// musicNoteName 把 MC 音高(pitch)换算成显示用音符名。仅用于悬浮窗展示。
func musicNoteName(pitch float64) string {
	if pitch <= 0 {
		return ""
	}
	// MIDI note = round(12*log2(pitch)) + 60，再映射到名字。
	n := int((12*math.Log2(pitch) + 60) + 0.5)
	names := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	return names[((n%12)+12)%12]
}

// splitSSEMessage 解析 "event: x\ndata: y\n\n" 得到 (x, y)。
func splitSSEMessage(msg string) (string, string) {
	var event, data string
	for _, line := range splitLines(msg) {
		switch {
		case len(line) > 7 && line[:7] == "event: ":
			event = line[7:]
		case len(line) > 6 && line[:6] == "data: ":
			data = line[6:]
		}
	}
	return event, data
}

// 命令处理器注册表：type -> handler
var floatingCommands = map[string]func(data any){
	"get_snapshot": func(data any) {
		floatingHub.Emit("status", mustJSON(snapshotStatus()))
	},
	// 悬浮窗「停止任务」按钮
	"task_stop": func(data any) {
		taskMu.Lock()
		defer taskMu.Unlock()
		if activeTask != nil {
			activeTask.Stop()
			activeTask = nil
		}
	},
}

func handleFloatingCommand(typ string, data any) {
	if fn, ok := floatingCommands[typ]; ok {
		fn(data)
	}
}

// snapshotStatus 生成初始状态快照。
func snapshotStatus() map[string]any {
	botMgrMu.Lock()
	defer botMgrMu.Unlock()
	connected := botMgr != nil && botMgr.IsConnected()
	isOP := false
	server := ""
	tps := 0.0
	if botMgr != nil {
		isOP = botMgr.IsOP()
		server = botMgr.ServerCode()
		tps = botMgr.GetTPS()
	}
	return map[string]any{
		"connected": connected,
		"is_op":     isOP,
		"server":    server,
		"tps":       tps,
		"ts":        time.Now().Unix(),
	}
}

// startStatusTicker 每秒向 floatingHub 推送一次 status，保证悬浮窗状态/TPS 实时刷新。
func startStatusTicker() {
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			floatingHub.Emit("status", mustJSON(snapshotStatus()))
		}
	}()
}
