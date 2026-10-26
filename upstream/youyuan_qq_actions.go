package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// QQ 机器人的扩展能力：点赞、QQ空间动态、自我形象（头像/签名/昵称）。
// 所有动作复用群服桥接的 OneBot WebSocket echo 通道，不另开连接；
// 仅 OneBot(NapCat) 适配器支持，QQ 官方适配器没有这些接口。

type yyQQActionRecord struct {
	Time   time.Time `json:"time"`
	Kind   string    `json:"kind"`
	Caller string    `json:"caller"`
	Name   string    `json:"name"`
	Target string    `json:"target"`
	Detail string    `json:"detail"`
	OK     bool      `json:"ok"`
	Error  string    `json:"error,omitempty"`
}

type yyQQActionGate struct {
	mu      sync.Mutex
	cool    map[string]time.Time // 冷却键 kind|caller|target
	gap     map[string]time.Time // 同类操作最小间隔 kind|caller
	dayUsed map[string]int       // 当日成功次数 kind|caller
	day     string
	recent  []yyQQActionRecord
}

var yyQQActions = &yyQQActionGate{cool: map[string]time.Time{}, gap: map[string]time.Time{}, dayUsed: map[string]int{}}

const yyQQActionKeep = 60

// minLevel: 0 成员 1 管理员 2 超级管理员；cool 对同一 caller+target 生效。
// 数值来自注册表默认值叠加玩家在「能力」里改过的覆盖值（CapLimits），
// 所以面板上调完冷却/每日次数，这里执行时立刻就是新值，不用重启。
func yyQQActionPolicy(kind string) (minLevel int, cool, gap time.Duration, daily int) {
	if c, ok := yyQQCapEffectiveOf(yyQQSnapshot().Assistant, kind); ok {
		return c.MinLevel, time.Duration(c.CoolSeconds) * time.Second, time.Duration(c.GapSeconds) * time.Second, c.Daily
	}
	return 2, time.Minute, 10 * time.Second, 10
}

func (g *yyQQActionGate) stamp(kind, caller, target string, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.day != now.Format("2006-01-02") {
		g.day = now.Format("2006-01-02")
		g.dayUsed = map[string]int{}
		g.cool = map[string]time.Time{}
		g.gap = map[string]time.Time{}
	}
	g.cool[kind+"|"+caller+"|"+target] = now
	g.gap[kind+"|"+caller] = now
}
func (g *yyQQActionGate) allow(kind, caller, target string, now time.Time) error {
	_, cool, gap, daily := yyQQActionPolicy(kind)
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.day != now.Format("2006-01-02") {
		return nil
	}
	if last, ok := g.cool[kind+"|"+caller+"|"+target]; ok {
		if wait := cool - now.Sub(last); wait > 0 {
			return fmt.Errorf("操作太频繁，请 %d 秒后再试", int(wait.Seconds())+1)
		}
	}
	if last, ok := g.gap[kind+"|"+caller]; ok {
		if wait := gap - now.Sub(last); wait > 0 {
			return fmt.Errorf("机器人正在处理同类操作，请 %d 秒后再试", int(wait.Seconds())+1)
		}
	}
	if daily > 0 && g.dayUsed[kind+"|"+caller] >= daily {
		return fmt.Errorf("今日次数已用完（每天最多 %d 次）", daily)
	}
	return nil
}
func (g *yyQQActionGate) succeed(kind, caller string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.day != time.Now().Format("2006-01-02") {
		g.day = time.Now().Format("2006-01-02")
		g.dayUsed = map[string]int{}
	}
	g.dayUsed[kind+"|"+caller]++
}
func (g *yyQQActionGate) log(rec yyQQActionRecord) {
	g.mu.Lock()
	g.recent = append([]yyQQActionRecord{rec}, g.recent...)
	if len(g.recent) > yyQQActionKeep {
		g.recent = g.recent[:yyQQActionKeep]
	}
	g.mu.Unlock()
	if rec.OK {
		yyQQLog("QQ能力 " + rec.Kind + " by " + rec.Name + "：" + rec.Detail)
	} else {
		yyQQLog("QQ能力 " + rec.Kind + " by " + rec.Name + " 失败：" + rec.Error)
	}
}
func yyQQActionRecent() []yyQQActionRecord {
	yyQQActions.mu.Lock()
	defer yyQQActions.mu.Unlock()
	return append([]yyQQActionRecord(nil), yyQQActions.recent...)
}

// yyQQRunAction 执行一个受控的机器人能力：校验适配器与权限、限流、留痕。
func yyQQRunAction(kind, caller, name, target, detail string, level int, run func() (string, error)) error {
	cf := yyQQSnapshot()
	if !cf.Enabled {
		return fmt.Errorf("群服互通未启用")
	}
	if cf.Adapter != "onebot" {
		return fmt.Errorf("当前是 QQ 官方适配器，不支持这个能力（需要 OneBot/NapCat 适配器）")
	}
	if !yyQQCapEnabled(cf.Assistant, kind) {
		label := kind
		if c, ok := yyQQCapabilityOf(kind); ok {
			label = c.Label
		}
		return fmt.Errorf("能力「%s」已在设置里关闭，先在工具箱「群服互通 · 机器人能力」打开", label)
	}
	minLevel, _, _, _ := yyQQActionPolicy(kind)
	if level < minLevel {
		return fmt.Errorf("权限不足：这个能力需要%s及以上", []string{"成员", "管理员", "超级管理员"}[minLevel])
	}
	now := time.Now()
	if e := yyQQActions.allow(kind, caller, target, now); e != nil {
		return e
	}
	yyQQActions.stamp(kind, caller, target, now)
	out, e := run()
	if e == nil {
		yyQQActions.succeed(kind, caller)
		if out != "" {
			detail = out
		}
	}
	yyQQActions.log(yyQQActionRecord{Time: now, Kind: kind, Caller: caller, Name: name, Target: target, Detail: detail, OK: e == nil, Error: func() string {
		if e != nil {
			return e.Error()
		}
		return ""
	}()})
	return e
}

// ---- OneBot 动作薄封装 ----

func yyQQActionLike(userID string, times int) error {
	_, e := yyQQOneBotCall("send_like", map[string]any{"user_id": userID, "times": times})
	return e
}
func yyQQActionQzone(content string, ugcRight int) (string, error) {
	data, e := yyQQOneBotCall("send_qzone_msg", map[string]any{"content": content, "ugc_right": ugcRight})
	if e != nil {
		return "", e
	}
	if m, ok := data.(map[string]any); ok {
		return yyQQStableID(m["tid"]), nil
	}
	return "", nil
}
func yyQQActionDeleteQzone(tid string) error {
	_, e := yyQQOneBotCall("delete_qzone_msg", map[string]any{"tid": tid})
	return e
}
func yyQQActionAvatar(file string) error {
	_, e := yyQQOneBotCall("set_qq_avatar", map[string]any{"file": file})
	return e
}
func yyQQActionLongNick(text string) error {
	_, e := yyQQOneBotCall("set_long_nick", map[string]any{"longNick": text})
	return e
}
func yyQQActionNick(name string) error {
	_, e := yyQQOneBotCall("set_qq_profile", map[string]any{"nickname": name})
	return e
}

// ---- 群指令处理 ----

func yyQQNumericQQ(value string) bool {
	if len(value) < 5 || len(value) > 12 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func yyQQCmdLike(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	target, times := user, 1
	fields := strings.Fields(arg)
	if len(fields) > 0 && fields[0] != "自己" {
		target = fields[0]
		if !yyQQNumericQQ(target) {
			return reply("格式：.点赞自己 或 .点赞 QQ号 [次数1-10]")
		}
	}
	if len(fields) > 1 {
		v, e := strconv.Atoi(fields[1])
		if e != nil || v < 1 || v > 10 {
			return reply("点赞次数为 1-10")
		}
		times = v
	}
	if level == 0 && target != user {
		return reply("普通成员只能给自己点赞；管理员可以帮任何人点赞。")
	}
	e := yyQQRunAction("like", user, name, target, fmt.Sprintf("QQ%s ×%d", target, times), level, func() (string, error) {
		return "", yyQQActionLike(target, times)
	})
	if e != nil {
		return reply("点赞失败：" + e.Error())
	}
	if target == user {
		return reply(fmt.Sprintf("已给你（QQ%s）点赞 %d 次。", target, times))
	}
	return reply(fmt.Sprintf("已给 QQ%s 点赞 %d 次。", target, times))
}
func yyQQCmdQzone(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	content := strings.TrimSpace(arg)
	if content == "" {
		return reply("格式：.动态 内容（1-500字，所有人可见）")
	}
	if len([]rune(content)) > 500 {
		return reply("动态内容最长500字")
	}
	if strings.ContainsAny(content, "\r\n") {
		content = strings.Join(strings.Fields(content), " ")
	}
	tid := ""
	e := yyQQRunAction("qzone", user, name, "", yyQQCardClip(content, 40), level, func() (string, error) {
		out, err := yyQQActionQzone(content, 1)
		tid = out
		return "", err
	})
	if e != nil {
		return reply("动态发表失败：" + e.Error())
	}
	if tid != "" {
		return reply(fmt.Sprintf("动态已发表（所有人可见）。删除请发送：.删动态 %s", tid))
	}
	return reply("动态已提交，但未返回 tid，暂时无法用指令删除。")
}
func yyQQCmdDeleteQzone(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	tid := strings.TrimSpace(arg)
	if tid == "" {
		return reply("格式：.删动态 tid（tid 来自 .动态 的回复）")
	}
	e := yyQQRunAction("delete_qzone", user, name, tid, tid, level, func() (string, error) {
		return "", yyQQActionDeleteQzone(tid)
	})
	if e != nil {
		return reply("删除失败：" + e.Error())
	}
	return reply("动态已删除：" + tid)
}
func yyQQCmdAvatar(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	file := strings.TrimSpace(arg)
	if file == "" {
		return reply("格式：.改头像 图片URL（http/https）或 base64")
	}
	if len(file) > 4000000 {
		return reply("图片数据过大")
	}
	if !strings.HasPrefix(file, "http://") && !strings.HasPrefix(file, "https://") && len(file) < 128 {
		return reply("只支持 http/https 图片链接或以 base64 开头的图片数据")
	}
	e := yyQQRunAction("avatar", user, name, "", yyQQCardClip(file, 60), level, func() (string, error) {
		return "", yyQQActionAvatar(file)
	})
	if e != nil {
		return reply("头像设置失败：" + e.Error())
	}
	return reply("QQ头像已更新。")
}
func yyQQCmdLongNick(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	text := strings.TrimSpace(arg)
	if text == "" || len([]rune(text)) > 100 || strings.ContainsAny(text, "\r\n") {
		return reply("签名内容需 1-100 字且不能换行")
	}
	e := yyQQRunAction("long_nick", user, name, "", yyQQCardClip(text, 40), level, func() (string, error) {
		return "", yyQQActionLongNick(text)
	})
	if e != nil {
		return reply("签名设置失败：" + e.Error())
	}
	return reply("个性签名已更新：" + text)
}
func yyQQCmdNick(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	nick := strings.TrimSpace(arg)
	if nick == "" || len([]rune(nick)) > 64 || strings.ContainsAny(nick, "\r\n") {
		return reply("昵称需 1-64 字且不能换行")
	}
	e := yyQQRunAction("nick", user, name, "", nick, level, func() (string, error) {
		return "", yyQQActionNick(nick)
	})
	if e != nil {
		return reply("昵称设置失败：" + e.Error())
	}
	return reply("QQ昵称已更新：" + nick)
}
func yyQQCmdActions(cf yyQQConfig, level int) []string {
	// 指令名取生效值（玩家别名优先）：玩家把 .改头像 改成 .换头像，这里同步显示。
	cmd := func(id string) string { return yyQQCommandText(cf, id) }
	text := "✦ 游园 · 机器人能力\n━━━━━━━━━━━━━━\n👍 点赞：" + cmd("like") + " QQ号 [次数1-10]\n😄 表情回应：回复某条消息发 " + cmd("emoji") + " [表情ID]，不填用 👍\n🖼 收藏表情：" + cmd("faces") + " 看机器人收藏了哪些图\n📮 动态：" + cmd("qzone") + " 内容（≤500字，所有人可见）；删除用 " + cmd("delete_qzone") + " tid\n🧑 形象（超级管理员）：" + cmd("avatar") + " 图片URL / " + cmd("long_nick") + " 内容 / " + cmd("nick") + " 名字\n👢 移出群聊（管理员）：" + cmd("kick") + " QQ号 [原因]\n所有操作均有限流与留痕，请合理使用。"
	return []string{text}
}

// ---- HTTP 面板接口 ----

func yyQQActionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/actions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			cf := yyQQSnapshot()
			writeJSON(w, map[string]any{"ok": true, "actions": yyQQActionRecent(), "caps": yyQQCapsView(cf.Assistant)})
			return
		}
		if r.Method != "POST" {
			http.Error(w, "GET/POST required", 405)
			return
		}
		var p struct {
			Action  string `json:"action"`
			Target  string `json:"target"`
			Times   int    `json:"times"`
			Content string `json:"content"`
			File    string `json:"file"`
			Tid     string `json:"tid"`
			Group   string `json:"group"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024*1024)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		// 面板调用者已通过设备令牌认证，按超级管理员执行，仍受同一限流约束。
		caller, name := "panel", "控制面板"
		var e error
		var out string
		switch p.Action {
		case "like":
			if !yyQQNumericQQ(strings.TrimSpace(p.Target)) {
				writeJSON(w, map[string]any{"ok": false, "error": "请填写对方QQ号"})
				return
			}
			times := p.Times
			if times < 1 || times > 10 {
				times = 1
			}
			out, e = "", yyQQRunAction("like", caller, name, strings.TrimSpace(p.Target), fmt.Sprintf("QQ%s ×%d", strings.TrimSpace(p.Target), times), 2, func() (string, error) {
				return "", yyQQActionLike(strings.TrimSpace(p.Target), times)
			})
		case "qzone":
			content := strings.TrimSpace(p.Content)
			if content == "" || len([]rune(content)) > 500 {
				writeJSON(w, map[string]any{"ok": false, "error": "动态内容需 1-500 字"})
				return
			}
			if strings.ContainsAny(content, "\r\n") {
				content = strings.Join(strings.Fields(content), " ")
			}
			e = yyQQRunAction("qzone", caller, name, "", yyQQCardClip(content, 40), 2, func() (string, error) {
				tid, err := yyQQActionQzone(content, 1)
				out = tid
				return "", err
			})
		case "delete_qzone":
			if strings.TrimSpace(p.Tid) == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "缺少 tid"})
				return
			}
			out, e = "", yyQQRunAction("delete_qzone", caller, name, strings.TrimSpace(p.Tid), strings.TrimSpace(p.Tid), 2, func() (string, error) {
				return "", yyQQActionDeleteQzone(strings.TrimSpace(p.Tid))
			})
		case "avatar":
			file := strings.TrimSpace(p.File)
			if file == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "缺少图片URL或base64"})
				return
			}
			out, e = "", yyQQRunAction("avatar", caller, name, "", yyQQCardClip(file, 60), 2, func() (string, error) {
				return "", yyQQActionAvatar(file)
			})
		case "long_nick":
			text := strings.TrimSpace(p.Content)
			if text == "" || len([]rune(text)) > 100 || strings.ContainsAny(text, "\r\n") {
				writeJSON(w, map[string]any{"ok": false, "error": "签名需 1-100 字且不能换行"})
				return
			}
			out, e = "", yyQQRunAction("long_nick", caller, name, "", yyQQCardClip(text, 40), 2, func() (string, error) {
				return "", yyQQActionLongNick(text)
			})
		case "nick":
			nick := strings.TrimSpace(p.Content)
			if nick == "" || len([]rune(nick)) > 64 || strings.ContainsAny(nick, "\r\n") {
				writeJSON(w, map[string]any{"ok": false, "error": "昵称需 1-64 字且不能换行"})
				return
			}
			out, e = "", yyQQRunAction("nick", caller, name, "", nick, 2, func() (string, error) {
				return "", yyQQActionNick(nick)
			})
		case "sticker":
			// 收藏表情「点一下直发」：以机器人身份把图片发到指定群，
			// 不传 group 时发到当前默认启用群。仍过能力开关与限流。
			file := strings.TrimSpace(p.File)
			if !yyStickerURL(file) {
				writeJSON(w, map[string]any{"ok": false, "error": "收藏表情的图片地址无效（只支持 http/https）"})
				return
			}
			cf := yyQQSnapshot()
			group := strings.TrimSpace(p.Group)
			if group == "" {
				group = yyQQGroupString(cf.GroupID)
			}
			if group == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "还没有默认互通群，请先指定群"})
				return
			}
			e = yyQQRunAction("sticker", caller, name, "", yyQQCardClip(file, 60), 2, func() (string, error) {
				return "", yyStickerSend(group, file)
			})
		default:
			writeJSON(w, map[string]any{"ok": false, "error": "action 支持 like/qzone/delete_qzone/avatar/long_nick/nick/sticker"})
			return
		}
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "tid": out})
	})
	mux.HandleFunc("/api/qq/groups/available", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "GET required", 405)
			return
		}
		cf := yyQQSnapshot()
		if cf.Adapter != "onebot" {
			writeJSON(w, map[string]any{"ok": false, "error": "仅 OneBot 适配器支持拉取群列表"})
			return
		}
		data, e := yyQQOneBotCall("get_group_list", map[string]any{})
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		list, _ := data.([]any)
		out := []map[string]any{}
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, map[string]any{
				"group_id":         yyQQStableID(m["group_id"]),
				"group_name":       fmt.Sprint(m["group_name"]),
				"member_count":     m["member_count"],
				"max_member_count": m["max_member_count"],
			})
		}
		writeJSON(w, map[string]any{"ok": true, "groups": out})
	})
}
