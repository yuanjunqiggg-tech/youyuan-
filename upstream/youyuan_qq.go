package main

import (
	"bot-apk/config"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 官方 QQ Webhook 与 OneBot 是两套独立适配器，不借用原作者的订阅服务器。
type yyQQConfig struct {
	Enabled     bool        `json:"enabled"`
	Adapter     string      `json:"adapter"`
	GroupID     any         `json:"group_id"`
	GroupNumber string      `json:"group_number,omitempty"`
	Groups      []yyQQGroup `json:"groups"`
	OneBot      struct {
		BotID string `json:"bot_id,omitempty"`
		URL   string `json:"ws_url"`
		Token string `json:"token,omitempty"`
	} `json:"onebot"`
	Official struct {
		AppID       string `json:"app_id"`
		Secret      string `json:"app_secret,omitempty"`
		Transport   string `json:"transport,omitempty"`
		Acknowledge *bool  `json:"acknowledge,omitempty"`
	} `json:"qqofficial"`
	Command struct {
		Permission string   `json:"permission"`
		Whitelist  []string `json:"whitelist"`
		Users      []string `json:"users"`
	} `json:"command"`
	GameToGroup struct {
		Format   string   `json:"format"`
		Prefixes []string `json:"only_prefixes"`
	} `json:"game_to_group"`
	Assistant yyQQAssistantConfig `json:"assistant"`
	// 掉线自愈参数：随 qq-bridge.json 落盘，面板可调。
	// MaxPerHour 是旧字段，等价于 MaxRestarts，仅为读旧配置保留。
	Watchdog struct {
		IntervalSeconds int `json:"interval_seconds,omitempty"`
		MaxRestarts     int `json:"max_restarts,omitempty"`
		MaxPerHour      int `json:"max_per_hour,omitempty"`
	} `json:"watchdog,omitempty"`
}

var yyQQMu sync.Mutex
var yyQQ yyQQConfig
var yyQQPath string
var yyQQState = "unconfigured"
var yyQQDetail = "填写你自己的机器人连接信息"
var yyQQConn *websocket.Conn
var yyQQCancel context.CancelFunc
var yyQQPending = map[string]chan map[string]any{}

type yyQQOutgoingJob struct{ Text, Group, Player string }

var yyQQOutgoing = make(chan yyQQOutgoingJob, 256)
var yyQQHTTP = &http.Client{Timeout: 15 * time.Second}

func yyQQLog(text string) {
	yyEvent("qq_log", text)
	if hub != nil {
		hub.Emit("qq_log", mustJSON(text))
	}
}
func yyQQSnapshot() yyQQConfig { yyQQMu.Lock(); defer yyQQMu.Unlock(); return yyQQ }
func yyRegisterQQ(mux *http.ServeMux, dataDir string) {
	yyQQPath = filepath.Join(dataDir, "qq-bridge.json")
	// 已处理消息 id 落盘：重连/重启后适配器重推同一条消息时不能再来一遍副作用。
	yyQQClaimsInit(dataDir)
	yyQQOfficialRoutes(mux)
	data, _ := os.ReadFile(yyQQPath)
	_ = json.Unmarshal(data, &yyQQ)
	var stored map[string]json.RawMessage
	_ = json.Unmarshal(data, &stored)
	if stored["assistant"] == nil {
		yyQQ.Assistant.Enabled = true
	}
	yyQQAssistantRoutes(mux, dataDir)
	yyQQGroupRoutes(mux)
	yyQQActionRoutes(mux)
	yyQQCapRoutes(mux)
	yyQQLoginRoutes(mux)
	// 掉线自愈：端口长时间不监听就重启协议端（保留登录态走快速登录，不逼用户扫码）。
	yyQQWatchdogRoutes(mux)
	// AI 能力自检：让面板能直接看到「AI 现在能用哪些工具、模型支不支持看图」。
	yyQQAISelfRoutes(mux)
	// AI 工具清单：全量工具 + 分组 + 逐项开关（保存后立即影响实际下发的工具）。
	yyQQToolRoutes(mux)
	yyQQTriggerRoutes(mux)
	yyQQImageRoutes(mux, dataDir)
	mux.HandleFunc("/api/qq/sandbox", yyQQSandboxHandler)
	// 互通开放接口：插件/MCP 通过 /api/bridge/* 调能力、订阅事件、配规则。
	yyBridgeRoutes(mux, dataDir)
	// 种子查询器：本机种子→群系/结构/坐标，机器人也能注册使用。
	yySeedRoutes(mux)
	// 种子查询器 v2：真 climate 噪声生物群系 + 结构放置逐位复刻 + PNG 瓦片渲染。
	// 前端页面、插件 SDK(seed.*)、MCP(api_seed_v2_*) 全部走这一套，同一份实现。
	yySeedV2Routes(mux)
	// 互通端信息：玩家列表/皮肤头像/进出记录/日志/以机器人身份说话。
	yySrvInfoRoutes(mux)
	// 头像/皮肤本地缓存代理：境外源太慢（实测 1.2~3.1s），缓存后 0.6ms。
	yySkinRoutes(mux, dataDir)
	// 情感系统：按人好感度 + 情绪，面板可查可校正。
	yyAffRoutes(mux, dataDir)
	// 群规：违禁词硬拦截 + 超管自然语言禁令（AI 侧走人格卡，永久生效）。
	yyQQModerationInit(dataDir)
	yyQQModerationRoutes(mux)
	yyQQModerationAIRoutes(mux)
	// 人格卡接线：反AI腔/禁忌/群风格提示词的载体，自然语言禁令写进这里，
	// 重启不丢（此前整套 persona 已实现但没有任何调用方）。
	yyQQPersonaInit(dataDir)
	// 人格与群风格的读写接口（/api/qq/persona）。
	//
	// 这里必须显式注册：yyQQPersonaRoutes 此前**只出现在测试文件里**，
	// 生产代码只调了 Init 没调 Routes，于是该接口线上一直 404——
	// 用户在互通页保存提示词，请求打到 /api/qq/persona 得到 not_found，
	// 返回再看提示词就没了（用户反馈「我保存了之后，后面我返回看了一下，提示词没了」）。
	yyQQPersonaRoutes(mux)
	yyGameSkillInit(dataDir)
	yyQQSkillRoutes(mux)
	// 主动搭话：先加载 qq-proactive.json（yyProactiveRoutes 内部调 yyProactiveInit），
	// 注册面板接口（默认关闭，用户自己开），再起 tick 轮询。
	yyProactiveRoutes(mux, dataDir)
	yyProactiveStart()
	yyQQSyncBotBinding()
	yyQQNormalizeConfig(&yyQQ, yyQQConfig{})
	if yyQQ.Adapter == "" {
		yyQQ.Adapter = "onebot"
	}
	if yyQQ.Command.Permission == "" {
		yyQQ.Command.Permission = "管理员"
	}
	mux.HandleFunc("/api/qq/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var patch map[string]any
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			yyQQMu.Lock()
			raw, _ := json.Marshal(yyQQ)
			var current map[string]any
			_ = json.Unmarshal(raw, &current)
			yyMergePatch(current, patch)
			raw, _ = json.Marshal(current)
			var next yyQQConfig
			err := json.Unmarshal(raw, &next)
			if err == nil && next.Adapter != "onebot" && next.Adapter != "qqofficial" {
				err = fmt.Errorf("未知适配器")
			}
			if err != nil {
				yyQQMu.Unlock()
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			yyQQNormalizeConfig(&next, yyQQ)
			// 语音模式只允许 off/auto/always，面板传了别的值就归一化成空（=不发语音），
			// 避免脏配置让 always 分支误判。
			next.Assistant.VoiceMode = yyQQVoiceModeNormalize(next.Assistant.VoiceMode)
			if err := yyQQValidateAdminIDs(next); err != nil {
				yyQQMu.Unlock()
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			if err := yyQQValidateGroups(next); err != nil {
				yyQQMu.Unlock()
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			if next.Official.Transport != "" && next.Official.Transport != "webhook" && next.Official.Transport != "websocket" {
				yyQQMu.Unlock()
				writeJSON(w, map[string]any{"ok": false, "error": "官方接收方式支持 webhook/websocket"})
				return
			}
			raw, _ = json.MarshalIndent(next, "", "  ")
			err = os.WriteFile(yyQQPath, raw, 0600)
			// 只有「连接参数」变了才断开重连。
			// 以前是保存任何人设/提示词都无条件 yyQQCancel()，结果用户改一句话术，
			// QQ 桥接就被踢一次，表现出来就是「动不动就刷新/掉线」。
			reconnect := next.Enabled != yyQQ.Enabled ||
				next.Adapter != yyQQ.Adapter ||
				next.OneBot.URL != yyQQ.OneBot.URL ||
				next.OneBot.Token != yyQQ.OneBot.Token ||
				next.Official.AppID != yyQQ.Official.AppID ||
				next.Official.Secret != yyQQ.Official.Secret ||
				next.Official.Transport != yyQQ.Official.Transport ||
				mustJSON(yyQQGroups(next)) != mustJSON(yyQQGroups(yyQQ))
			if err == nil {
				yyQQ = next
				if reconnect {
					yyQQState = "connecting"
					yyQQDetail = "连接参数已变更，正在重新接入"
				}
			}
			if yyQQCancel != nil && reconnect {
				yyQQCancel()
			}
			yyQQMu.Unlock()
			yyQQSyncBotBinding()
			if err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
		}
		cf := yyQQSnapshot()
		raw, _ := json.Marshal(cf)
		var view map[string]any
		_ = json.Unmarshal(raw, &view)
		view["configured"] = (cf.Adapter == "onebot" && cf.OneBot.URL != "" && len(yyQQGroups(cf)) > 0) || (cf.Adapter == "qqofficial" && cf.Official.AppID != "" && cf.Official.Secret != "")
		ob := view["onebot"].(map[string]any)
		ob["has_token"] = cf.OneBot.Token != ""
		delete(ob, "token")
		official := view["qqofficial"].(map[string]any)
		official["has_secret"] = cf.Official.Secret != ""
		delete(official, "app_secret")
		helper := view["assistant"].(map[string]any)
		helper["has_mcp_token"] = cf.Assistant.MCPToken != ""
		delete(helper, "mcp_token")
		writeJSON(w, map[string]any{"ok": true, "config": view})
	})
	mux.HandleFunc("/api/qq/status", func(w http.ResponseWriter, r *http.Request) {
		// 2026-10-26 第 30 条：状态带也要能看出「假在线」。
		// 只看 state 的话，socket 还开着就永远是「已连接」，
		// 用户盯着绿点等 5 分钟也等不到扫码提示。
		live, lastOK, offline, liveErr, everOK, _ := yyQQLiveness()
		needScan := everOK && !live && offline >= int(yyQQOfflineEscalate.Seconds())
		yyQQMu.Lock()
		defer yyQQMu.Unlock()
		out := map[string]any{
			"ok": true, "enabled": yyQQ.Enabled, "adapter": yyQQ.Adapter, "group": yyQQ.GroupID,
			"state": yyQQState, "detail": yyQQDetail, "warnings": []string{},
			"live": live, "live_ever_checked": everOK,
			"last_ok_seconds": lastOK, "offline_seconds": offline,
			"need_scan": needScan,
		}
		if liveErr != "" {
			out["live_error"] = liveErr
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("/api/qq/send", func(w http.ResponseWriter, r *http.Request) {
		// 面板「以机器人身份说话」的终端用这里。
		// group 为空 = 发到当前默认群；给了 group 就发到指定群（要求该群已启用）。
		var p struct {
			Text  string `json:"text"`
			Group string `json:"group"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		p.Text = strings.TrimSpace(p.Text)
		if p.Text == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "消息不能为空"})
			return
		}
		if len([]rune(p.Text)) > 8000 {
			// 8000 是 OneBot/官方接口单条消息的实际传输上限，属于技术限制，
			// 不是「内容限制」。空消息仍然拒绝——空串根本发不出去。
			writeJSON(w, map[string]any{"ok": false, "error": "单条消息超过 8000 字，这是 QQ 接口的传输上限（不是内容限制），请拆成几条发"})
			return
		}
		cf := yyQQSnapshot()
		var err error
		if g := strings.TrimSpace(p.Group); g != "" {
			if !cf.Enabled {
				writeJSON(w, map[string]any{"ok": false, "error": "群服互通未启用"})
				return
			}
			if !yyQQGroupEnabled(cf, g) {
				writeJSON(w, map[string]any{"ok": false, "error": "目标群未启用：" + g})
				return
			}
			cf.GroupID = g
			err = yyQQSendToGroup(cf, p.Text, "")
		} else {
			err = yyQQSend(p.Text, "")
		}
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		yyQQLog("面板以机器人身份发消息：" + p.Text)
		writeJSON(w, map[string]any{"ok": true})
	})
	// 手动重连：面板上「掉线横幅 → 重连」按钮调这里。
	// 只做一件事：掐掉当前 WS 连接，让 yyQQConnectLoop 自己走退避重连，
	// 不重建 goroutine，避免重复连接把 NapCat 打死。
	mux.HandleFunc("/api/qq/reconnect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		yyQQMu.Lock()
		conn := yyQQConn
		cancel := yyQQCancel
		yyQQState = "reconnecting"
		yyQQDetail = "面板手动触发重连"
		yyQQMu.Unlock()
		if conn != nil {
			_ = conn.CloseNow()
		}
		if cancel != nil {
			cancel()
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/qq/webhook", yyQQWebhook)
	go yyQQConnectLoop()
	go yyQQMemoryMaintenance()
	// 服主插话清场依赖 assistant.companion_owner：为空时该分支永不触发，
	// 群里任何人插话都清不掉别人的陪聊会话。这里只提示，不改行为。
	if yyQQ.Assistant.CompanionOwner == "" {
		yyQQLog("未配置服主（assistant.companion_owner），服主插话清场不生效")
	}
	// 每 30 秒用 /list 校准在线名单：PlayerList 的 REMOVE 包漏收会让
	// 已离线的人一直算在在线里（用户实测 1 人显示 4 人）。
	yyOnlineSyncStart()
	// 好感度 6 小时向 0 衰减一次，避免永久记仇。
	yyAffStartDecayLoop()
	// 掉线自愈守护：QQ 掉登录时 NapCat 不监听端口，光靠桥接重连永远连不上，
	// 必须把协议端拉起来让它自己快速登录。没有这个守护，用户只能一遍遍扫码。
	yyQQWatchdogStart()
	go func() {
		for job := range yyQQOutgoing {
			var err error
			if job.Group == "" {
				err = yyQQSend(job.Text, "")
			} else {
				cf := yyQQSnapshot()
				if !cf.Enabled || !yyQQGroupEnabled(cf, job.Group) {
					err = fmt.Errorf("目标群已停用")
				} else {
					cf.GroupID = job.Group
					err = yyQQSendToGroup(cf, job.Text, "")
				}
			}
			if err != nil {
				yyQQLog("群消息发送失败：" + err.Error())
				if job.Player != "" {
					yyQQGameNotice(job.Player, "转发失败："+err.Error()+"\n官方QQ需要群成员近期 @机器人；请在目标群 @机器人发任意指令后再试。")
				}
			} else if job.Player != "" {
				yyQQGameNotice(job.Player, "已转发到QQ群 "+yyQQGroupLabel(yyQQSnapshot(), job.Group))
			}
		}
	}()
}
func yyMergePatch(current, patch map[string]any) {
	for key, value := range patch {
		if child, ok := value.(map[string]any); ok {
			target, _ := current[key].(map[string]any)
			if target == nil {
				target = map[string]any{}
			}
			yyMergePatch(target, child)
			current[key] = target
		} else {
			if (key == "token" || key == "app_secret" || key == "mcp_token") && value == "" {
				continue
			}
			current[key] = value
		}
	}
}
func yyQQConnectLoop() {
	// 重连退避：以前这里硬编码 3 秒，用户在设置里把「重连间隔」改成 10 秒也没用，
	// 因为这个循环根本不读配置——这就是「改完下次看还是 3 秒」的根因。
	// 现在读 cf.ReconnectSeconds（默认 10），失败次数越多退避越久，并加抖动，
	// 避免多实例同时重连把 NapCat 打挂（掉线频繁的诱因之一）。
	fails := 0
	backoff := func() time.Duration {
		base := config.LoadConfig().ReconnectSeconds
		if base <= 0 {
			base = 10
		}
		d := time.Duration(base) * time.Second
		for i := 0; i < fails && i < 4; i++ {
			d *= 2
		}
		if d > 90*time.Second {
			d = 90 * time.Second
		}
		// ±20% 抖动
		jitter := time.Duration(float64(d) * 0.2 * (float64(time.Now().UnixNano()%1000)/500 - 1))
		return d + jitter
	}
	reset := func() { fails = 0 }
	bump := func() { fails++ }
	for {
		cf := yyQQSnapshot()
		ctx, cancel := context.WithCancel(context.Background())
		yyQQMu.Lock()
		yyQQCancel = cancel
		yyQQMu.Unlock()
		if !cf.Enabled {
			yyQQMu.Lock()
			yyQQState = "stopped"
			yyQQMu.Unlock()
			cancel()
			time.Sleep(time.Second)
			continue
		}
		if cf.Adapter == "qqofficial" {
			if cf.Official.Transport == "websocket" {
				err := yyQQOfficialConnect(ctx, cf)
				interrupted := ctx.Err() != nil
				cancel()
				if err != nil && !interrupted {
					yyQQMu.Lock()
					yyQQState = "error"
					yyQQDetail = err.Error()
					yyQQMu.Unlock()
				}
				bump()
				time.Sleep(backoff())
				continue
			}
			yyQQMu.Lock()
			if yyQQState != "open" {
				yyQQState = "waiting_callback"
				yyQQDetail = "凭据已配置，等待 QQ 官方签名回调；不是已连通"
			}
			yyQQMu.Unlock()
			cancel()
			time.Sleep(backoff())
			continue
		}
		if cf.OneBot.URL == "" {
			yyQQMu.Lock()
			yyQQState = "unconfigured"
			yyQQMu.Unlock()
			cancel()
			time.Sleep(time.Second)
			continue
		}
		yyQQMu.Lock()
		yyQQState = "connecting"
		yyQQMu.Unlock()
		// 拨号超时：以前用 background ctx 拨号，NapCat 卡住时会一直挂着。
		dialCtx, dialCancel := context.WithTimeout(ctx, 15*time.Second)
		conn, _, err := websocket.Dial(dialCtx, cf.OneBot.URL, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + cf.OneBot.Token}}})
		dialCancel()
		if err == nil {
			yyQQMu.Lock()
			yyQQConn = conn
			yyQQState = "open"
			yyQQDetail = "OneBot WebSocket 已连接"
			yyQQMu.Unlock()
			reset()
			conn.SetReadLimit(2 * 1024 * 1024)
			// 心跳：OneBot 这条链路以前既不发 ping 也没有读超时，
			// 链路被 NAT/负载均衡静默回收后 Read 会永久阻塞，状态还停在 "open"，
			// 于是「看着在线其实早断了」。每 30 秒 ping 一次，10 秒内没 pong 就主动断开重连。
			pingCtx, pingStop := context.WithCancel(ctx)
			go func(c *websocket.Conn) {
				t := time.NewTicker(30 * time.Second)
				defer t.Stop()
				for {
					select {
					case <-pingCtx.Done():
						return
					case <-t.C:
						pctx, pcancel := context.WithTimeout(pingCtx, 10*time.Second)
						e := c.Ping(pctx)
						pcancel()
						if e != nil {
							yyQQLog("OneBot 心跳失败，主动断开重连：" + e.Error())
							_ = c.CloseNow()
							return
						}
					}
				}
			}(conn)
			for {
				_, raw, e := conn.Read(ctx)
				if e != nil {
					err = e
					break
				}
				var event map[string]any
				decoder := json.NewDecoder(strings.NewReader(string(raw)))
				decoder.UseNumber() // QQ IDs may exceed float64's exact integer range.
				if decoder.Decode(&event) != nil {
					continue
				}
				if echo, ok := event["echo"].(string); ok {
					yyQQMu.Lock()
					waiter := yyQQPending[echo]
					yyQQMu.Unlock()
					if waiter != nil {
						select {
						case waiter <- event:
						default:
						}
					}
					continue
				}
				in, ok := yyQQParseInbound(event)
				if !ok {
					continue
				}
				current := yyQQSnapshot()
				if self := yyQQStableID(event["self_id"]); self != "" {
					current.OneBot.BotID = self
					yyQQMu.Lock()
					if yyQQ.OneBot.URL == current.OneBot.URL {
						yyQQ.OneBot.BotID = self
					}
					yyQQMu.Unlock()
					yyQQSyncBotBinding()
				}
				if in.IsPrivate {
					// 私聊：本期只做结构化记录与触发词，不进游戏转发/助手链路（现状没有，不新造）。
					yyQQInboundPrivate(current, in)
					continue
				}
				if !yyQQGroupEnabled(current, in.Group) {
					continue
				}
				current.GroupID = in.Group
				yyQQRememberReply(in.Group, in.ID)
				// 入站上下文先于分派落库：生图引用（reference:"quoted"）等后续功能按 message_id 取用。
				yyQQInboundStore(in)
				yyQQInboundResolveReply(current, &in)
				// 情感系统：每条群消息都过一遍好感判定（夸/谢/骂/刷屏）。
				// 做成 goroutine 里的同步调用会拖慢读循环，这里只做纯内存运算，很快。
				yyAffOnMessage(in.User, in.Name, in.Text)
				go yyQQToGameMentionMeta(current, in.Name, in.User, in.Role, in.Text, in.ID, in.Mentioned, in.ReplyID, in.User != "" && in.User == current.OneBot.BotID)
			}
			pingStop()
			conn.CloseNow()
		}
		cancel()
		yyQQMu.Lock()
		yyQQConn = nil
		yyQQState = "reconnecting"
		if err != nil {
			yyQQDetail = err.Error()
		}
		yyQQMu.Unlock()
		bump()
		time.Sleep(backoff())
	}
}

var yyQQLeadingMention = regexp.MustCompile(`^\s*<@!?[0-9A-Za-z_-]+>\s*`)

func yyQQOfficialText(text string) string {
	return strings.TrimSpace(yyQQLeadingMention.ReplaceAllString(text, ""))
}
func yyQQRenderMessage(format, player, text string) string {
	return strings.NewReplacer("${玩家}", player, "${消息}", text, "{player}", player, "{message}", text).Replace(format)
}
func yyQQCommandAllowed(cf yyQQConfig, userID, role, text string) error {
	permitted := cf.Command.Permission == "所有人" || (cf.Command.Permission == "管理员" && (role == "admin" || role == "owner")) || (cf.Command.Permission == "群主" && role == "owner")
	if cf.Command.Permission == "指定用户" {
		for _, id := range cf.Command.Users {
			if id != "" && id == userID {
				permitted = true
			}
		}
	}
	if !permitted {
		return fmt.Errorf("群指令权限不足")
	}
	for _, command := range yySplitConsole(text) {
		if !strings.HasPrefix(command, "/") {
			return fmt.Errorf("指令链含无效指令")
		}
		if len(cf.Command.Whitelist) == 0 {
			continue
		}
		allowed := false
		for _, prefix := range cf.Command.Whitelist {
			prefix = strings.TrimSpace(prefix)
			if prefix != "" && (command == prefix || strings.HasPrefix(command, prefix+" ")) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("指令链含白名单外的指令：%s", strings.Fields(command)[0])
		}
	}
	return nil
}
func yyQQFeedback(text, id string) {
	if err := yyQQSend(text, id); err != nil {
		yyQQLog("群执行回执发送失败：" + err.Error())
	}
}

// AI replies retain their received group even when message IDs collide across groups.
func yyQQFeedbackTo(cf yyQQConfig, text, id string) {
	next := yyQQSnapshot()
	next.GroupID = yyQQGroupString(cf.GroupID)
	if !next.Enabled || !yyQQGroupEnabled(next, yyQQGroupString(cf.GroupID)) || yyQQMessageKey(next) != yyQQMessageKey(cf) {
		return
	}
	if err := yyQQSendToGroup(next, text, id); err != nil {
		yyQQLog("群执行回执发送失败：" + err.Error())
	}
}
func yyQQToGame(cf yyQQConfig, name, userID, role, text, id string) {
	yyQQToGameMention(cf, name, userID, role, text, id, false)
}
func yyQQToGameMention(cf yyQQConfig, name, userID, role, text, id string, mentioned bool) {
	yyQQToGameMentionMeta(cf, name, userID, role, text, id, mentioned, "", false)
}
func yyQQToGameMentionMeta(cf yyQQConfig, name, userID, role, text, id string, mentioned bool, replyID string, isBot bool) {
	if userID == "" || isBot {
		return
	}
	next := yyQQSnapshot()
	sourceGroup := yyQQGroupString(cf.GroupID)
	if !next.Enabled || next.Adapter != cf.Adapter || !yyQQGroupEnabled(next, sourceGroup) || next.Official.AppID != cf.Official.AppID {
		return
	}
	cf = next
	cf.GroupID = sourceGroup
	text = strings.TrimSpace(text)
	command := strings.HasPrefix(text, "/")
	if id != "" {
		// 幂等：同一条 message_id 只能驱动一次副作用。
		// 这里以前是进程内的 yyQQRecent（10 分钟），重启/重连后表是空的，
		// 适配器再推一次同样的消息，触发词动作就会整条重跑（点赞两次、指令打两次）。
		// 现在改成落盘的认领表：先记账再执行，跨重启也认得这条消息。
		if !yyQQClaimMessage(cf, id) {
			return
		}
	}
	yyQQHelper.Lock()
	if userID != "" && (len(yyQQHelper.Seen) < 256 || yyQQHelper.Seen[userID] != "") {
		name = yyQQDisplayName(cf, userID, name)
		if name == "群成员" && yyQQHelper.Seen[userID] != "" && yyQQHelper.Seen[userID] != userID {
			name = yyQQHelper.Seen[userID]
		}
		yyQQHelper.Seen[userID] = name
		yyQQHelper.Dirty = true
	}
	yyQQHelper.Unlock()
	// 超管自然语言禁令：「以后不准 玩家XXX」→ 群规硬拦截 + AI 长期记住。
	// 放在群规检查之前，禁令指令本身永远不会被自己的规则误伤。
	if yyQQNaturalBan(cf, userID, name, text, id) {
		return
	}
	// 群规硬通道：命中即拦——不进 RAG、不转发游戏、不触发 AI。
	// warn 级规则不在这里拦（消息照常流动），它们只进 AI 提示词软劝阻。
	if rule, hit := yyQQModerationBlockHit(cf, text, userID); hit {
		yyQQModerationEnforce(cf, rule, userID, name, text, id)
		return
	}
	// 主动搭话的活跃度输入：每条真实群消息更新该群的窗口内消息数、最后发言时间与
	// 话题词。位置刻意放在幂等门（yyQQClaimMessage）之后——适配器重推同一条消息
	// 不重复计数；也放在群规硬拦之后——违禁词不进 AI 的话题提示。这里只写内存，
	// 不改动下面的触发词、@ 回复与转发流程。
	yyProactiveOnMessage(&yyQQInbound{Group: sourceGroup, User: userID, Text: text})
	yyQQRememberGroupMessageMeta(cf, userID, name, text, id, replyID, false)
	if cf.Assistant.Enabled && yyQQTriggerRun(cf, name, userID, role, text, id, mentioned) {
		return
	}
	// 被 @ 且不是指令 = 用户在直接跟它说话，必须由 AI 回答。
	//
	// 以前这里直接落到下面的 `if cf.Assistant.Enabled { ... return }`，
	// 也就是「@机器人 说一句话 → 什么都不发生」。用户只有发 `.AI 问题`
	// 才拿得到回复，所以他判断「AI 跟他没存在一样」。
	// 现在：@ 就回；companion 开着的话顺手续上会话，后面的消息还能接着聊。
	if cf.Assistant.Enabled && mentioned && !command &&
		!strings.HasPrefix(text, ".") && !strings.HasPrefix(text, "/") &&
		strings.TrimSpace(text) != "" && cf.Assistant.Model != "" {
		if cf.Assistant.Companion {
			// 只续期，不重复回复——回复由下面这条快速路径负责，
			// 免得 companion 的 5 秒延迟让 @ 显得像没反应。
			yyQQCompanionBegin(cf, userID, text, true, time.Now())
		}
		if yyQQMentionReply(cf, name, userID, text, id) {
			return
		}
	}
	if cf.Assistant.Enabled && yyQQCompanionHandle(cf, userID, text, id, mentioned) {
		return
	}
	if cf.Assistant.Enabled && yyQQAssistantHandle(cf, name, userID, role, text, id) {
		return
	}
	if cf.Assistant.Enabled {
		// 群聊互通开关：助手模式下，非指令的普通群聊也转发进游戏（[群] 昵称：内容），
		// 游戏侧仍用 .通 回复，避免双向无限循环。
		if cf.Assistant.RelayChat && !command && !strings.HasPrefix(text, ".") && text != "帮助" && text != "list" {
			botMgrMu.Lock()
			bm := botMgr
			botMgrMu.Unlock()
			if bm != nil && bm.IsConnected() {
				if _, e := bm.SendConsole("[群] " + name + "：" + text); e != nil {
					yyQQLog("群聊转发失败：" + e.Error())
				}
			}
		}
		return
	}
	if command {
		if err := yyQQCommandAllowed(cf, userID, role, text); err != nil {
			yyQQLog(err.Error())
			yyQQFeedback(err.Error(), id)
			return
		}
	}
	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()
	if bm == nil || !bm.IsConnected() {
		yyQQLog("游戏尚未连接，群消息未转发")
		if command {
			yyQQFeedback("游戏尚未连接，指令未执行", id)
		}
		return
	}
	input := "[群] " + name + "：" + text
	if command {
		input = text
	}
	result, err := bm.SendConsole(input)
	if err != nil {
		yyQQLog("游戏转发失败：" + err.Error())
		if command {
			yyQQFeedback("执行失败："+err.Error(), id)
		}
		return
	}
	yyQQLog("群→服：" + text + "；" + mustJSON(result))
	if !command && cf.Adapter == "qqofficial" && (cf.Official.Acknowledge == nil || *cf.Official.Acknowledge) {
		yyQQFeedback("已转发到游戏。", id)
	}
	if command {
		reply := mustJSON(result)
		if len([]rune(reply)) > 1200 {
			reply = string([]rune(reply)[:1200]) + "…（完整记录见终端）"
		}
		yyQQFeedback("执行回执："+reply, id)
	}
}
func yyQQGameChat(player, text string) {
	cf := yyQQSnapshot()
	if !cf.Enabled {
		return
	}
	if strings.HasPrefix(text, "[群]") {
		return
	}
	if cf.Assistant.Enabled {
		if yyQQGameSelection(cf, player, text) {
			return
		}
		if yyQQConfirmBinding(cf, player, text) {
			return
		}
		// 游戏→群的转发指令也走别名表（relay，默认 .通）。
		relay := yyQQCommandText(cf, "relay")
		if !yyQQHasPrefix(text, relay) {
			return
		}
		text = strings.TrimSpace(strings.TrimPrefix(text, relay))
		if text == "" {
			return
		}
	}
	if len(cf.GameToGroup.Prefixes) > 0 && !cf.Assistant.Enabled {
		matched := false
		for _, p := range cf.GameToGroup.Prefixes {
			if strings.HasPrefix(text, p) {
				matched = true
			}
		}
		if !matched {
			return
		}
	}
	format := cf.GameToGroup.Format
	if format == "" {
		format = "<${玩家}> ${消息}"
	}
	format = yyQQRenderMessage(format, player, text)
	select {
	case yyQQOutgoing <- yyQQOutgoingJob{Text: format}:
	default:
		yyQQLog("群服发送队列已满，当前消息未发送")
	}
}
func yyQQSend(text, msgID string) error {
	cf := yyQQSnapshot()
	if !cf.Enabled {
		return fmt.Errorf("群服互通未启用")
	}
	if msgID != "" {
		group := yyQQReplyGroup(msgID)
		if !yyQQGroupEnabled(cf, group) {
			return fmt.Errorf("来源群已停用或回复已过期")
		}
		cf.GroupID = group
		return yyQQSendToGroup(cf, text, msgID)
	}
	var failures []string
	targets := 0
	for _, group := range yyQQGroups(cf) {
		if !group.Enabled {
			continue
		}
		targets++
		target := cf
		target.GroupID = group.ID
		if e := yyQQSendToGroup(target, text, ""); e != nil {
			failures = append(failures, group.Name+"："+e.Error())
		}
	}
	if targets == 0 {
		return fmt.Errorf("没有启用的互通群")
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "；"))
	}
	return nil
}
func yyQQSendToGroup(cf yyQQConfig, text, msgID string) error {
	if cf.Adapter == "qqofficial" {
		return yyQQOfficialSend(cf, text, msgID)
	}
	return yyQQSendOneBot(cf, text, true)
}
func yyQQSendOneBot(cf yyQQConfig, message any, autoEscape bool) error {
	group, e := strconv.ParseInt(yyQQGroupString(cf.GroupID), 10, 64)
	if e != nil {
		return fmt.Errorf("OneBot 群号无效")
	}
	_, e = yyQQOneBotCall("send_group_msg", map[string]any{"group_id": group, "message": message, "auto_escape": autoEscape})
	return e
}

// yyQQOneBotCall 通过群服桥接的 WebSocket 调用任意 OneBot action 并等待回执。
// 返回 ack 的 data 字段（可能是对象或数组）；失败时 error 已含 OneBot 的说明。
func yyQQOneBotCall(action string, params map[string]any) (any, error) {
	return yyQQOneBotCallTimeout(action, params, 15*time.Second)
}

// yyQQOneBotCallTimeout 同上，但可指定超时。
// 存活探测必须用短超时（2~3 秒）：掉线时 OneBot 可能既不回执也不关连接，
// 用默认 15 秒会把探测线程和面板接口一起拖住。
func yyQQOneBotCallTimeout(action string, params map[string]any, timeout time.Duration) (any, error) {
	yyQQMu.Lock()
	conn := yyQQConn
	echo := yyNewKey()
	waiter := make(chan map[string]any, 1)
	yyQQPending[echo] = waiter
	yyQQMu.Unlock()
	defer func() { yyQQMu.Lock(); delete(yyQQPending, echo); yyQQMu.Unlock() }()
	if conn == nil {
		return nil, fmt.Errorf("OneBot 尚未连接")
	}
	data, _ := json.Marshal(map[string]any{"action": action, "params": params, "echo": echo})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		return nil, err
	}
	select {
	case ack := <-waiter:
		if yyQQStableID(ack["retcode"]) != "0" {
			return nil, fmt.Errorf("OneBot 返回失败：%s", mustJSON(ack))
		}
		return ack["data"], nil
	case <-ctx.Done():
		return nil, fmt.Errorf("OneBot 回执超时，结果未知")
	}
}
func yyQQPrivateKey(secret string) ed25519.PrivateKey {
	seed := []byte(secret)
	for len(seed) > 0 && len(seed) < 32 {
		seed = append(seed, []byte(secret)...)
	}
	if len(seed) < 32 {
		return nil
	}
	return ed25519.NewKeyFromSeed(seed[:32])
}
func yyQQWebhook(w http.ResponseWriter, r *http.Request) {
	cf := yyQQSnapshot()
	key := yyQQPrivateKey(cf.Official.Secret)
	if !cf.Enabled || cf.Adapter != "qqofficial" || key == nil {
		http.Error(w, "官方机器人未配置", 503)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var event struct {
		Op   int            `json:"op"`
		Type string         `json:"t"`
		ID   string         `json:"id"`
		Data map[string]any `json:"d"`
	}
	if json.Unmarshal(raw, &event) != nil {
		http.Error(w, "JSON 无效", 400)
		return
	}
	appID := r.Header.Get("X-Bot-Appid")
	if appID != "" && appID != cf.Official.AppID {
		http.Error(w, "机器人 AppID 不匹配", 401)
		return
	}
	if event.Op != 13 || r.Header.Get("X-Signature-Ed25519") != "" {
		timestamp := r.Header.Get("X-Signature-Timestamp")
		sig, err := hex.DecodeString(r.Header.Get("X-Signature-Ed25519"))
		if err != nil || !ed25519.Verify(key.Public().(ed25519.PublicKey), append([]byte(timestamp), raw...), sig) {
			http.Error(w, "QQ 回调签名无效", 401)
			return
		}
	} else if appID == "" || appID != cf.Official.AppID {
		http.Error(w, "验证请求缺少匹配的 X-Bot-Appid", 401)
		return
	}
	if event.Op == 13 {
		plain, _ := event.Data["plain_token"].(string)
		at, _ := event.Data["event_ts"].(string)
		signature := ed25519.Sign(key, []byte(at+plain))
		writeJSON(w, map[string]any{"plain_token": plain, "signature": hex.EncodeToString(signature)})
		return
	}
	yyQQMu.Lock()
	yyQQState = "open"
	yyQQDetail = "已收到并验证 QQ 官方回调"
	yyQQReplies.Lock()
	yyQQReplies.Callback = time.Now()
	yyQQReplies.Unlock()
	yyQQMu.Unlock()
	yyQQOfficialEvent(cf, event.Type, event.ID, event.Data)
	writeJSON(w, map[string]any{"op": 12})
}
func yyQQOfficialSend(cf yyQQConfig, text, msgID string) error {
	return yyQQSendOfficialWindow(cf, text, msgID)
}
func urlPathEscape(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "/", "%2F"), "?", "%3F")
}

func yyQQGroupString(value any) string {
	if value == nil {
		return ""
	}
	if number, ok := value.(float64); ok {
		return strconv.FormatFloat(number, 'f', 0, 64)
	}
	return fmt.Sprint(value)
}
