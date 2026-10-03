package main

import (
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
var yyQQRecent = map[string]time.Time{}
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
			if err == nil {
				yyQQ = next
				yyQQState = "connecting"
				yyQQDetail = "配置已保存，正在重新接入"
			}
			if yyQQCancel != nil {
				yyQQCancel()
			}
			yyQQMu.Unlock()
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
		yyQQMu.Lock()
		defer yyQQMu.Unlock()
		writeJSON(w, map[string]any{"ok": true, "enabled": yyQQ.Enabled, "adapter": yyQQ.Adapter, "group": yyQQ.GroupID, "state": yyQQState, "detail": yyQQDetail, "warnings": []string{}})
	})
	mux.HandleFunc("/api/qq/send", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		err := yyQQSend(p.Text, "")
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/qq/webhook", yyQQWebhook)
	go yyQQConnectLoop()
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
				time.Sleep(10 * time.Second)
				continue
			}
			yyQQMu.Lock()
			if yyQQState != "open" {
				yyQQState = "waiting_callback"
				yyQQDetail = "凭据已配置，等待 QQ 官方签名回调；不是已连通"
			}
			yyQQMu.Unlock()
			cancel()
			time.Sleep(3 * time.Second)
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
		conn, _, err := websocket.Dial(ctx, cf.OneBot.URL, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + cf.OneBot.Token}}})
		if err == nil {
			yyQQMu.Lock()
			yyQQConn = conn
			yyQQState = "open"
			yyQQDetail = "OneBot WebSocket 已连接"
			yyQQMu.Unlock()
			conn.SetReadLimit(2 * 1024 * 1024)
			for {
				_, raw, e := conn.Read(ctx)
				if e != nil {
					err = e
					break
				}
				var event map[string]any
				if json.Unmarshal(raw, &event) != nil {
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
				if event["post_type"] != "message" || event["message_type"] != "group" || !yyQQGroupEnabled(yyQQSnapshot(), yyQQGroupString(event["group_id"])) {
					continue
				}
				sender, _ := event["sender"].(map[string]any)
				name := fmt.Sprint(sender["nickname"])
				role, _ := sender["role"].(string)
				text, _ := event["raw_message"].(string)
				current := yyQQSnapshot()
				current.GroupID = yyQQGroupString(event["group_id"])
				id := fmt.Sprint(event["message_id"])
				yyQQRememberReply(yyQQGroupString(current.GroupID), id)
				go yyQQToGame(current, name, yyQQGroupString(event["user_id"]), role, text, id)
			}
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
		time.Sleep(3 * time.Second)
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
func yyQQToGame(cf yyQQConfig, name, userID, role, text, id string) {
	yyQQToGameMention(cf, name, userID, role, text, id, false)
}
func yyQQToGameMention(cf yyQQConfig, name, userID, role, text, id string, mentioned bool) {
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
		yyQQMu.Lock()
		now := time.Now()
		for key, at := range yyQQRecent {
			if now.Sub(at) > 10*time.Minute {
				delete(yyQQRecent, key)
			}
		}
		_, seen := yyQQRecent[id]
		yyQQRecent[id] = now
		yyQQMu.Unlock()
		if seen {
			return
		}
	}
	yyQQHelper.Lock()
	if userID != "" && (len(yyQQHelper.Seen) < 256 || yyQQHelper.Seen[userID] != "") {
		yyQQHelper.Seen[userID] = name
		yyQQHelper.Dirty = true
	}
	yyQQHelper.Unlock()
	if cf.Assistant.Enabled && yyQQCompanionHandle(cf, userID, text, id, mentioned) {
		return
	}
	if cf.Assistant.Enabled && yyQQAssistantHandle(cf, name, userID, role, text, id) {
		return
	}
	if cf.Assistant.Enabled {
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
		if yyQQConfirmBinding(player, text) {
			return
		}
		if !yyQQHasPrefix(text, ".通") {
			return
		}
		text = strings.TrimSpace(strings.TrimPrefix(text, ".通"))
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
	yyQQMu.Lock()
	conn := yyQQConn
	echo := yyNewKey()
	waiter := make(chan map[string]any, 1)
	yyQQPending[echo] = waiter
	yyQQMu.Unlock()
	defer func() { yyQQMu.Lock(); delete(yyQQPending, echo); yyQQMu.Unlock() }()
	if conn == nil {
		return fmt.Errorf("OneBot 尚未连接")
	}
	group, e := strconv.ParseInt(yyQQGroupString(cf.GroupID), 10, 64)
	if e != nil {
		return fmt.Errorf("OneBot 群号无效")
	}
	data, _ := json.Marshal(map[string]any{"action": "send_group_msg", "params": map[string]any{"group_id": group, "message": message, "auto_escape": autoEscape}, "echo": echo})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		return err
	}
	select {
	case ack := <-waiter:
		if ack["retcode"] != float64(0) {
			return fmt.Errorf("OneBot 返回失败：%s", mustJSON(ack))
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("OneBot 回执超时，发送结果未知")
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
