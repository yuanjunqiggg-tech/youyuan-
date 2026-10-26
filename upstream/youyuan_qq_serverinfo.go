package main

// 群服互通 · 互通端信息面板（后端）
// ---------------------------------------------------------------------------
// 用户的原话：「停服互通的话，也是要有互通端的一些信息的……列出玩家列表、
// 查看一些日志、玩家的皮肤头像、进出服务器的记录、服务器的名称查询……
// 平时互通，还要知道一些插件相关内容……还能用联通的机器人的身份去服务器里面
// 说话、运行一些指令等」。
//
// 第一性原则：这些数据早就在进程里了，只是没有出口。所以本文件不新建第二套
// 状态机，只做「把已有结构整理成面板能吃的 JSON」+「一个受控的出口」：
//
//   /api/qq/server/info     服务器名/群/机器人资料/在线人数/绑定数
//                           ← yyQQSnapshot + yyQQGroups + yyQQHelper + playerInfoMgr
//   /api/qq/server/players  玩家列表（头像用 /api/skin/online-head）
//                           ← yyQQHelper.Stats / Bindings + playerInfoMgr
//   /api/qq/server/events   进服/退服时间轴 ← yyQQHelper.Stats 的 Joined/LastJoin/LastLeave
//   /api/qq/server/logs     机器人动作 + 技能执行 ← yyQQActionRecent + yySkillLogs
//   /api/qq/server/say      以机器人身份说话/执行指令
//                           ← yyQQRelayGame（tellraw，说不了命令）+ yySkillConsoleSend
//                           危险指令不执行，转成群内授权（youyuan_qq_approval.go）
//
// 所有新标识符统一 yySrvInfo 前缀，避免与并行开发的其它模块撞名。

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	yySrvInfoDefaultLimit = 100
	yySrvInfoMaxLimit     = 500
	// yySrvInfoSayHardMax 不是内容限制，是传输上限：tellraw/控制台单条消息
	// 超过这个长度会被服务器或 OneBot 截断甚至丢弃。用户明确说过「500 字限制
	// 没必要」，所以这里只保留一条技术兜底，并且错误信息里写清楚原因。
	yySrvInfoSayHardMax = 2000
	yySrvInfoBotTTL     = 5 * time.Minute
	// yySrvInfoFillMax 单次 fill 允许的最大方块数。超过就会在主线程上
	// 一次性改几万方块，轻则卡服，重则直接踢人，属于「误操作成本极高」的操作。
	yySrvInfoFillMax = 10000
)

// ---- 机器人资料缓存 ----
//
// OneBot 的 get_login_info 是一次 WebSocket 往返（最坏 15 秒超时）。
// 面板是轮询的，绝不能每次请求都打过去。这里做 5 分钟缓存 + 过期后异步刷新：
// 接口永远只读缓存，拿不到就返回 null + error，绝不整体失败。

type yySrvInfoBotCache struct {
	sync.Mutex
	At         time.Time
	Nickname   string
	UIN        string
	Avatar     string
	Err        string
	Refreshing bool
}

var yySrvInfoBot = &yySrvInfoBotCache{}

// yySrvInfoBotProfile 读取机器人资料。返回 (资料, 错误说明)：
// 资料为 nil 表示还没有缓存，错误说明用于前端原样展示。
func yySrvInfoBotProfile(cf yyQQConfig) (map[string]any, string) {
	uin := strings.TrimSpace(cf.OneBot.BotID)
	if uin == "" {
		if cf.Adapter == "qqofficial" {
			return nil, "QQ 官方适配器没有机器人QQ号，头像与昵称读不到"
		}
		return nil, "还没有填写机器人QQ号，无法显示头像与昵称"
	}
	yySrvInfoBot.Lock()
	cached := map[string]any{}
	if yySrvInfoBot.UIN != "" {
		cached["uin"] = yySrvInfoBot.UIN
		cached["avatar"] = yySrvInfoBot.Avatar
		cached["nickname"] = yySrvInfoBot.Nickname
	}
	errText := yySrvInfoBot.Err
	stale := yySrvInfoBot.UIN == "" || time.Since(yySrvInfoBot.At) > yySrvInfoBotTTL
	needRefresh := stale && !yySrvInfoBot.Refreshing
	if needRefresh {
		yySrvInfoBot.Refreshing = true
	}
	yySrvInfoBot.Unlock()

	// 只允许 OneBot 适配器查资料，且必须已经连上（否则必然 15 秒超时）。
	if needRefresh && cf.Adapter == "onebot" && yyQQStateNow() == "open" {
		go yySrvInfoBotRefresh()
	}
	if len(cached) == 0 {
		if errText == "" {
			errText = "机器人资料尚未就绪（首次读取中），稍后刷新即可"
		}
		return nil, errText
	}
	return cached, ""
}

func yySrvInfoBotRefresh() {
	defer func() {
		yySrvInfoBot.Lock()
		yySrvInfoBot.Refreshing = false
		yySrvInfoBot.Unlock()
	}()
	info, e := yyQQOneBotCall("get_login_info", map[string]any{})
	yySrvInfoBot.Lock()
	defer yySrvInfoBot.Unlock()
	yySrvInfoBot.At = time.Now()
	if e != nil {
		yySrvInfoBot.Err = "读取机器人资料失败：" + e.Error()
		return
	}
	m, ok := info.(map[string]any)
	if !ok {
		yySrvInfoBot.Err = "机器人资料格式不认识"
		return
	}
	uin := yyQQStableID(m["user_id"])
	if uin == "" {
		yySrvInfoBot.Err = "机器人没有返回QQ号"
		return
	}
	yySrvInfoBot.UIN = uin
	yySrvInfoBot.Avatar = yyQQAvatarURL(uin)
	yySrvInfoBot.Nickname = strings.TrimSpace(fmt.Sprint(m["nickname"]))
	yySrvInfoBot.Err = ""
}

// yySrvInfoPlayerAvatar 玩家皮肤头像。沿用前端既有的写法
// （frontend/js/features/filepicker.js 里也是 /api/skin/online-head?player=）。
// 玩家不在线或没有皮肤缓存时该接口返回 JSON，前端靠 onerror 隐藏，不做二次探测。
func yySrvInfoPlayerAvatar(player string) string {
	if player == "" {
		return ""
	}
	return "/api/skin/online-head?player=" + url.QueryEscape(player)
}

// yySrvInfoReverseBindings QQ号→玩家名 反查玩家名→QQ号。
func yySrvInfoReverseBindings() map[string]string {
	out := map[string]string{}
	yyQQHelper.Lock()
	defer yyQQHelper.Unlock()
	for qq, player := range yyQQHelper.Bindings {
		if player != "" {
			out[player] = qq
		}
	}
	return out
}

// yySrvInfoOnlineSet 取当前在线集合。
// 第二个返回值表示「这份集合可不可信」：游戏没连上、或者名单还是空的
// （网易租赁服靠 /list 轮询同步，刚连上时确实可能为空）都算不可信，
// 此时玩家行的 online 字段返回 null，绝不瞎猜 true/false。
func yySrvInfoOnlineSet() (map[string]bool, bool) {
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		return nil, false
	}
	set := playerInfoMgr.OnlineSet()
	if len(set) == 0 {
		return nil, false
	}
	return set, true
}

// yySrvInfoPrimaryGroup 取第一个启用的互通群号。面板发起授权时用它作为归属群，
// 这样超管只要在任何一个互通群里回「同意 编号」都能批。
func yySrvInfoPrimaryGroup(cf yyQQConfig) string {
	for _, g := range yyQQGroups(cf) {
		if !g.Enabled {
			continue
		}
		if id := strings.TrimSpace(yyQQGroupString(g.ID)); id != "" {
			return id
		}
	}
	if n := strings.TrimSpace(cf.GroupNumber); n != "" {
		return n
	}
	return strings.TrimSpace(yyQQGroupString(cf.GroupID))
}

func yySrvInfoLimit(r *http.Request, def, max int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return def
	}
	n, e := strconv.Atoi(raw)
	if e != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// yySrvInfoOnlineSeconds 累计在线秒数：已下线的取统计值；还在线的补上本次时长。
func yySrvInfoOnlineSeconds(v *yyQQPlayerStats, now time.Time) int64 {
	if v == nil {
		return 0
	}
	total := v.OnlineSeconds
	if !v.Joined.IsZero() {
		total += int64(now.Sub(v.Joined).Seconds())
	}
	if total < 0 {
		return 0
	}
	return total
}

// ---- 路由注册 ----

// yySrvInfoRoutes 注册「互通端信息」的 5 个接口。
// 由 yyRegisterQQ 调用（那一行由主控添加，本文件只提供函数）。
func yySrvInfoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/server/info", yySrvInfoHandleInfo)
	mux.HandleFunc("/api/qq/server/players", yySrvInfoHandlePlayers)
	mux.HandleFunc("/api/qq/server/events", yySrvInfoHandleEvents)
	mux.HandleFunc("/api/qq/server/logs", yySrvInfoHandleLogs)
	mux.HandleFunc("/api/qq/server/say", yySrvInfoHandleSay)
	mux.HandleFunc("/api/qq/guard", yySrvInfoHandleGuard)
	mux.HandleFunc("/api/qq/server/player", yySrvInfoHandlePlayer)
}

func yySrvInfoRequireGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == "GET" {
		return true
	}
	writeJSON(w, map[string]any{"ok": false, "error": "只支持 GET"})
	return false
}

// ---- 1. 服务器信息 ----

func yySrvInfoHandleInfo(w http.ResponseWriter, r *http.Request) {
	if !yySrvInfoRequireGet(w, r) {
		return
	}
	cf := yyQQSnapshot()
	groups := yyQQGroups(cf)

	// 服务器名：优先用互通群的名字（用户认的是群），没有就用群号兜底。
	name := ""
	groupID, groupNumber := "", strings.TrimSpace(cf.GroupNumber)
	for _, g := range groups {
		if !g.Enabled {
			continue
		}
		if name == "" {
			name = strings.TrimSpace(g.Name)
			groupID = strings.TrimSpace(g.ID)
			if groupNumber == "" {
				groupNumber = strings.TrimSpace(g.Number)
			}
		}
	}
	if name == "" {
		if n := yyQQGroupString(cf.GroupID); n != "" && n != "0" {
			groupID = n
			name = "互通群 " + n
		}
	}
	if name == "" {
		name = "互通服务器"
	}

	serverCode := ""
	gameConnected := false
	if bm := getBotManagerLocked(); bm != nil && bm.IsConnected() {
		gameConnected = true
		serverCode = strings.TrimSpace(bm.ServerCode())
	}

	yyQQHelper.Lock()
	playerTotal := len(yyQQHelper.Stats)
	bindingTotal := len(yyQQHelper.Bindings)
	yyQQHelper.Unlock()

	bot, botErr := yySrvInfoBotProfile(cf)

	out := map[string]any{
		"ok":             true,
		"name":           name,
		"server_code":    serverCode,
		"group_id":       groupID,
		"group_number":   groupNumber,
		"group_count":    len(groups),
		"adapter":        cf.Adapter,
		"enabled":        cf.Enabled,
		"state":          yyQQStateNow(),
		"game_connected": gameConnected,
		"players":        playerTotal,
		"bindings":       bindingTotal,
		"bot_player":     cf.Assistant.BotPlayer,
		"relay_chat":     cf.Assistant.RelayChat,
		"bot":            bot,
	}
	// 在线人数：拿不到可信集合就给 null，而不是 0——0 会被读成「没人玩」。
	if set, ok := yySrvInfoOnlineSet(); ok {
		out["online"] = len(set)
	} else {
		out["online"] = nil
		out["online_note"] = "游戏未连接或在线名单尚未同步，人数暂不可知"
	}
	if botErr != "" {
		out["error"] = botErr
	}
	writeJSON(w, out)
}

// ---- 2. 玩家列表 ----

// yySrvInfoPlayerRow 玩家列表的一行。Online 用 *bool 是为了区分
// 「确定在线/离线」和「根本不知道」——后者序列化成 null。
type yySrvInfoPlayerRow struct {
	Player        string `json:"player"`
	QQ            string `json:"qq"`
	Avatar        string `json:"avatar"`
	Online        *bool  `json:"online"`
	OnlineSeconds int64  `json:"online_seconds"`
	Messages      int    `json:"messages"`
	LastMessage   string `json:"last_message"`
	Joined        string `json:"joined"`
	LastJoin      string `json:"last_join"`
	LastLeave     string `json:"last_leave"`
}

func yySrvInfoHandlePlayers(w http.ResponseWriter, r *http.Request) {
	if !yySrvInfoRequireGet(w, r) {
		return
	}
	limit := yySrvInfoLimit(r, yySrvInfoDefaultLimit, yySrvInfoMaxLimit)
	wantOnline := strings.TrimSpace(r.URL.Query().Get("online")) == "1"
	now := time.Now()
	onlineSet, onlineKnown := yySrvInfoOnlineSet()
	qqOf := yySrvInfoReverseBindings()

	yyQQHelper.Lock()
	rows := make([]yySrvInfoPlayerRow, 0, len(yyQQHelper.Stats))
	for player, v := range yyQQHelper.Stats {
		if v == nil || player == "" {
			continue
		}
		item := yySrvInfoPlayerRow{
			Player:        player,
			QQ:            qqOf[player],
			Avatar:        yySrvInfoPlayerAvatar(player),
			OnlineSeconds: yySrvInfoOnlineSeconds(v, now),
			Messages:      v.Messages,
			LastMessage:   v.LastMessage,
			Joined:        yySrvInfoStamp(v.Joined),
			LastJoin:      yySrvInfoStamp(v.LastJoin),
			LastLeave:     yySrvInfoStamp(v.LastLeave),
		}
		if onlineKnown {
			on := onlineSet[player]
			item.Online = &on
		}
		rows = append(rows, item)
	}
	yyQQHelper.Unlock()

	if wantOnline {
		kept := rows[:0]
		for _, it := range rows {
			if it.Online != nil && *it.Online {
				kept = append(kept, it)
			}
		}
		rows = kept
	}

	// 排序：在线的排前面，其次按最近上线/离线时间倒序，保证面板第一屏是有用的。
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if (a.Online != nil && *a.Online) != (b.Online != nil && *b.Online) {
			return a.Online != nil && *a.Online
		}
		return yySrvInfoLastSeen(b) < yySrvInfoLastSeen(a)
	})
	total := len(rows)
	if len(rows) > limit {
		rows = rows[:limit]
	}

	out := map[string]any{"ok": true, "players": rows, "total": total, "limit": limit}
	if !onlineKnown {
		out["note"] = "游戏未连接或在线名单尚未同步，online 字段为 null 表示未知"
	}
	writeJSON(w, out)
}

func yySrvInfoLastSeen(v yySrvInfoPlayerRow) string {
	if v.LastLeave > v.LastJoin {
		return v.LastLeave
	}
	if v.LastJoin != "" {
		return v.LastJoin
	}
	return v.Joined
}

// yySrvInfoStamp 时间统一成 RFC3339（本地时区），前端 new Date() 直接能解析。
func yySrvInfoStamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(time.Local).Format(time.RFC3339)
}

// ---- 3. 进出服事件流 ----

// yySrvInfoHandleEvents 从 Stats 派生事件。项目里没有独立的事件环形缓冲
// （yyQQAssistantEvent 只负责播报），所以按 Joined/LastJoin/LastLeave 三个
// 时间戳还原：first_join=首次观测、join=最近一次进服、leave=最近一次退服。
func yySrvInfoHandleEvents(w http.ResponseWriter, r *http.Request) {
	if !yySrvInfoRequireGet(w, r) {
		return
	}
	limit := yySrvInfoLimit(r, 50, yySrvInfoMaxLimit)
	qqOf := yySrvInfoReverseBindings()

	type event struct {
		Time   string `json:"time"`
		Type   string `json:"type"`
		Player string `json:"player"`
		QQ     string `json:"qq"`
	}

	events := []event{}
	seen := map[string]bool{}
	add := func(t time.Time, kind, player string) {
		if t.IsZero() {
			return
		}
		stamp := yySrvInfoStamp(t)
		key := kind + "|" + player + "|" + stamp
		if seen[key] {
			return
		}
		seen[key] = true
		events = append(events, event{Time: stamp, Type: kind, Player: player, QQ: qqOf[player]})
	}

	yyQQHelper.Lock()
	for player, v := range yyQQHelper.Stats {
		if v == nil || player == "" {
			continue
		}
		add(v.Joined, "first_join", player)
		add(v.LastJoin, "join", player)
		add(v.LastLeave, "leave", player)
	}
	yyQQHelper.Unlock()

	sort.SliceStable(events, func(i, j int) bool { return events[i].Time > events[j].Time })
	total := len(events)
	if len(events) > limit {
		events = events[:limit]
	}
	writeJSON(w, map[string]any{"ok": true, "events": events, "total": total, "limit": limit})
}

// ---- 4. 日志 ----

// yySrvInfoHandleLogs 合并两个内存环：机器人动作（yyQQActions.recent，60 条）
// 和技能执行（yySkillLogs，200 条）。聊天日志只经 SSE 实时推送、不落盘，
// 所以响应里用 note 说清楚「这里看不到什么」，而不是假装日志很全。
func yySrvInfoHandleLogs(w http.ResponseWriter, r *http.Request) {
	if !yySrvInfoRequireGet(w, r) {
		return
	}
	limit := yySrvInfoLimit(r, 200, yySrvInfoMaxLimit)

	type entry struct {
		Time   string `json:"time"`
		Source string `json:"source"` // action | skill
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Target string `json:"target"`
		Detail string `json:"detail"`
		OK     bool   `json:"ok"`
		Error  string `json:"error,omitempty"`
		Ms     int64  `json:"ms,omitempty"`
	}

	entries := []entry{}
	for _, rec := range yyQQActionRecent() {
		entries = append(entries, entry{
			Time:   yySrvInfoStamp(rec.Time),
			Source: "action",
			Kind:   rec.Kind,
			Name:   rec.Name,
			Target: rec.Target,
			Detail: rec.Detail,
			OK:     rec.OK,
			Error:  rec.Error,
		})
	}

	yySkillLogs.Lock()
	for _, rec := range yySkillLogs.Items {
		params := make([]string, 0, len(rec.Params))
		for k, v := range rec.Params {
			if v == "" {
				continue
			}
			params = append(params, k+"="+v)
		}
		sort.Strings(params)
		entries = append(entries, entry{
			Time:   yySrvInfoStamp(rec.Time),
			Source: "skill",
			Kind:   rec.Skill,
			Name:   "技能",
			Target: strings.Join(params, " "),
			Detail: rec.Out,
			OK:     rec.OK,
			Ms:     rec.Ms,
		})
	}
	yySkillLogs.Unlock()

	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Time > entries[j].Time })
	total := len(entries)
	if len(entries) > limit {
		entries = entries[:limit]
	}
	writeJSON(w, map[string]any{
		"ok":    true,
		"logs":  entries,
		"total": total,
		"limit": limit,
		"note":  "仅显示机器人动作与技能执行记录；聊天日志走实时推送，不落盘",
		"sources": map[string]string{
			"action": "yyQQActionRecent（最近60条）",
			"skill":  "yySkillLogs（最近200条）",
		},
	})
}

// ---- 5. 以机器人身份说话 / 执行指令 ----

// yySrvInfoDangerRules 危险命令黑名单。
// 这不是「一票否决」：命中后不会执行，而是转成群内授权项，由超级管理员
// 回「同意 编号」才真正下发（见 youyuan_qq_approval.go）。Reason 是给群里
// 看的一句话，所以要写「为什么危险」，而不是「禁止」。
// yySrvInfoRule 一条拦截规则。Locked 只是「默认锁定」：初始勾选、不建议关，
// 但玩家显式关掉时照样尊重——服务器是玩家的，开关归他，不锁死。
type yySrvInfoRule struct {
	Prefix string `json:"prefix"`
	Reason string `json:"reason"`
	Locked bool   `json:"locked"`
}

var yySrvInfoDangerRules = []yySrvInfoRule{
	{Prefix: "op ", Reason: "op 会给人提权，且不可逆", Locked: true},
	{Prefix: "deop ", Reason: "deop 会撤销别人的管理权限", Locked: true},
	{Prefix: "stop", Reason: "stop 会直接停服", Locked: true},
	{Prefix: "kick ", Reason: "kick 会踢人下线"},
	{Prefix: "ban ", Reason: "ban 会封禁玩家", Locked: true},
	{Prefix: "ban-ip ", Reason: "ban-ip 会封禁 IP", Locked: true},
	{Prefix: "pardon ", Reason: "pardon 会解封"},
	{Prefix: "whitelist off", Reason: "关闭白名单会让任何人进服", Locked: true},
	{Prefix: "whitelist reload", Reason: "重载白名单会改变谁能进服"},
	{Prefix: "difficulty ", Reason: "改难度会影响全服体验"},
	{Prefix: "gamerule ", Reason: "改游戏规则会影响全服"},
	{Prefix: "kill @a", Reason: "批量击杀玩家"},
	{Prefix: "kill @e", Reason: "批量清除实体"},
	{Prefix: "gamemode ", Reason: "改玩家游戏模式"},
	{Prefix: "give ", Reason: "发物品可被用于刷物品"},
	{Prefix: "summon ", Reason: "刷实体"},
	{Prefix: "setblock ", Reason: "改方块"},
	{Prefix: "clone ", Reason: "批量复制方块"},
	{Prefix: "tp ", Reason: "传送玩家"},
	{Prefix: "spreadplayers ", Reason: "打散玩家"},
	{Prefix: "effect ", Reason: "给药水效果"},
	{Prefix: "enchant ", Reason: "附魔"},
	{Prefix: "xp ", Reason: "刷经验"},
	{Prefix: "clear ", Reason: "清空背包"},
	{Prefix: "worldborder ", Reason: "改世界边界"},
	{Prefix: "forceload ", Reason: "强制加载区块"},
	{Prefix: "save-off", Reason: "关闭存档会丢数据"},
	{Prefix: "save-all", Reason: "强制存盘会卡服"},
	{Prefix: "reload", Reason: "重载数据包会卡服"},
}

// yySrvInfoEffectiveRules 把玩家配置叠加到默认名单上：
// 默认规则玩家想关就关——含默认锁定的那 6 条，显式写了就尊重，
// 额外的自定义前缀追加在后面。
func yySrvInfoEffectiveRules(cf yyQQConfig) []yySrvInfoRule {
	off := make(map[string]bool, len(cf.Assistant.DangerDisabled))
	for _, v := range cf.Assistant.DangerDisabled {
		if s := strings.ToLower(strings.TrimSpace(v)); s != "" {
			off[s] = true
		}
	}
	out := make([]yySrvInfoRule, 0, len(yySrvInfoDangerRules)+len(cf.Assistant.DangerExtra))
	seen := make(map[string]bool, len(yySrvInfoDangerRules))
	for _, r := range yySrvInfoDangerRules {
		seen[r.Prefix] = true
		if off[strings.ToLower(strings.TrimSpace(r.Prefix))] {
			continue
		}
		out = append(out, r)
	}
	for _, v := range cf.Assistant.DangerExtra {
		prefix := strings.ToLower(strings.TrimSpace(v))
		if prefix == "" || seen[prefix] {
			continue
		}
		seen[prefix] = true
		out = append(out, yySrvInfoRule{Prefix: prefix, Reason: "已被玩家列入自定义拦截"})
	}
	return out
}

// yySrvInfoCommandGuard 校验一条要下发的命令。
// 只做「格式 + 危险动作」两件事，参数合法性交给服务器自己判断。
func yySrvInfoCommandGuard(cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return fmt.Errorf("指令不能为空")
	}
	if !strings.HasPrefix(cmd, "/") {
		return fmt.Errorf("指令模式必须以 / 开头（例如 /list）")
	}
	// 换行/控制字符会把一条命令拆成多条，必须挡在渲染之前。
	if e := yySkillGuardCommand(cmd); e != nil {
		return e
	}
	body := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(cmd, "/")))
	body = strings.Join(strings.Fields(body), " ")
	// 1.13+ 接受带命名空间前缀的指令：`/minecraft:op Steve` 与原版 `op` 等价。
	// 不剥掉这层前缀，黑名单就是形同虚设（安全审查实测可绕过）。
	stripNS := func(v string) string {
		for strings.HasPrefix(v, "minecraft:") {
			v = strings.TrimPrefix(v, "minecraft:")
		}
		return v
	}
	body = stripNS(body)

	// execute ... run <真命令> 是个明显的绕过口子，剥掉外壳再判一次。
	for i := 0; i < 4 && strings.HasPrefix(body, "execute "); i++ {
		idx := strings.LastIndex(body, " run ")
		if idx < 0 {
			break
		}
		body = strings.TrimSpace(body[idx+len(" run "):])
		body = stripNS(body)
	}

	for _, rule := range yySrvInfoEffectiveRules(yyQQSnapshot()) {
		if body == strings.TrimSpace(rule.Prefix) || strings.HasPrefix(body, rule.Prefix) {
			return errors.New(rule.Reason)
		}
	}
	if strings.HasPrefix(body, "fill ") {
		if e := yySrvInfoFillGuard(body); e != nil {
			return e
		}
	}
	return nil
}

// yySrvInfoFillGuard 限制 fill 的方块数。相对坐标（~）无法在本地算出体积，
// 一律拒绝——宁可让人改用绝对坐标，也不要放一个可能填几万方块的命令过去。
func yySrvInfoFillGuard(body string) error {
	fields := strings.Fields(body)
	if len(fields) < 7 {
		return fmt.Errorf("fill 参数不完整：需要 fill x1 y1 z1 x2 y2 z2 block")
	}
	nums := make([]int, 0, 6)
	for _, f := range fields[1:7] {
		if strings.Contains(f, "~") || strings.Contains(f, "^") {
			return fmt.Errorf("fill 用了相对坐标，本地无法估算体积，请改用绝对坐标")
		}
		v, e := strconv.Atoi(f)
		if e != nil {
			return fmt.Errorf("fill 坐标必须是整数：%s", f)
		}
		nums = append(nums, v)
	}
	dx := nums[3] - nums[0]
	dy := nums[4] - nums[1]
	dz := nums[5] - nums[2]
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dz < 0 {
		dz = -dz
	}
	volume := (dx + 1) * (dy + 1) * (dz + 1)
	if volume > yySrvInfoFillMax {
		return fmt.Errorf("fill 体积 %d 方块，超过单次上限 %d，请分批执行", volume, yySrvInfoFillMax)
	}
	return nil
}

// 面板说话不再限流：用户明确说「三秒限流没必要」。
// 唯一保留的是传输层硬上限（yySrvInfoSayHardMax）与危险指令授权。

func yySrvInfoHandleSay(w http.ResponseWriter, r *http.Request) {
	// 只允许 POST：GET 会被浏览器预取/爬虫触发，不能用来往游戏里发东西。
	if r.Method != "POST" {
		writeJSON(w, map[string]any{"ok": false, "error": "只支持 POST"})
		return
	}
	var p struct {
		Text string `json:"text"`
		Mode string `json:"mode"`
	}
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&p); e != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
		return
	}
	text := strings.TrimSpace(p.Text)
	mode := strings.TrimSpace(p.Mode)
	if mode == "" {
		mode = "chat"
	}
	if mode != "chat" && mode != "command" {
		writeJSON(w, map[string]any{"ok": false, "error": "mode 只能是 chat 或 command"})
		return
	}
	if text == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "内容不能为空"})
		return
	}
	// 指令模式等于远程执行，必须带令牌（安全审查 H3）。
	// 说话模式只影响聊天框，保留免令牌，方便面板直接用。
	if mode == "command" && !yyBridgeTokenOK(r) {
		writeJSON(w, map[string]any{"ok": false, "error": yyBridgeTokenHint()})
		return
	}

	// 只保留传输上限：超过这个长度服务器/OneBot 会截断甚至丢消息，
	// 跟「内容多长算合适」无关，所以错误信息里要说清楚是技术限制。
	if runes := []rune(text); len(runes) > yySrvInfoSayHardMax {
		writeJSON(w, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("单条超过 %d 字，这是传输上限（服务器与 OneBot 的单条长度限制），不是内容限制", yySrvInfoSayHardMax),
		})
		return
	}

	if mode == "command" {
		if e := yySrvInfoCommandGuard(text); e != nil {
			// 格式问题（没写 /、含控制字符）不排队：审批也不该让非法输入过关。
			if !strings.HasPrefix(text, "/") || yyApprPayloadBlocked(text) != "" {
				yySrvInfoLogSay(mode, text, false, e.Error())
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			// 危险指令不再一票否决：挂起成授权项，等超级管理员在群里点头。
			// 面板这边返回 ok:false（确实没执行），同时带上编号方便去群里批。
			cf := yyQQSnapshot()
			group := yySrvInfoPrimaryGroup(cf)
			item, ae := yyApprRequest(cf, group, "panel", "控制面板", yyApprKindConsole, text)
			if ae != nil {
				yySrvInfoLogSay(mode, text, false, e.Error()+"；授权申请失败："+ae.Error())
				writeJSON(w, map[string]any{"ok": false, "error": e.Error() + "；授权申请失败：" + ae.Error()})
				return
			}
			// 面板发起时群里也得看见，否则超管根本不知道有东西等他点头。
			if group != "" {
				select {
				case yyQQOutgoing <- yyQQOutgoingJob{Group: group, Text: "⚠️ 控制面板想执行一条有风险的指令（" + e.Error() + "）\n同意执行请回复：同意 " + item.ID + "\n拒绝请回复：拒绝 " + item.ID}:
				default:
					yyQQLog("授权提示未能入队（发送队列已满），编号 " + item.ID)
				}
			}
			hint := fmt.Sprintf("已在群里发起授权，编号 %s。请超级管理员在群内回复「同意 %s」。", item.ID, item.ID)
			yySrvInfoLogSay(mode, text, false, e.Error()+"；"+hint)
			writeJSON(w, map[string]any{
				"ok":          false,
				"error":       e.Error() + "；已提交授权申请",
				"pending":     true,
				"approval_id": item.ID,
				"expires_in":  int(yyApprTTL.Seconds()),
				"hint":        hint,
			})
			return
		}
	}

	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		msg := "游戏尚未连接，消息没有发出去"
		yySrvInfoLogSay(mode, text, false, msg)
		writeJSON(w, map[string]any{"ok": false, "error": msg})
		return
	}

	var output string
	var e error
	if mode == "command" {
		// 复用技能注册表同一条控制台通道（botMgr.SendConsole + 超时看护），
		// 不另写一套 TCP/WS 协议。
		output, e = yySkillConsoleSend(text, 5*time.Second)
	} else {
		// 说话走 tellraw：它只会把文字当成文字，不可能被解释成斜杠命令，
		// 所以群友让机器人「说」一句话不会变成执行一条指令。
		e = yyQQRelayGame(text)
		output = "已发送到全服"
	}
	if e != nil {
		yySrvInfoLogSay(mode, text, false, e.Error())
		writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
		return
	}
	yySrvInfoLogSay(mode, text, true, output)

	out := map[string]any{"ok": true, "mode": mode, "text": text, "output": output}
	writeJSON(w, out)
}

// yySrvInfoLogSay 每次说话/执行都留一条动作记录，复用 yyQQActions 的环形与日志。
func yySrvInfoLogSay(mode, text string, ok bool, detail string) {
	rec := yyQQActionRecord{
		Time:   time.Now(),
		Kind:   "server_say",
		Caller: "panel",
		Name:   "控制面板",
		Target: mode,
		Detail: yyQQClip(text, 80),
		OK:     ok,
	}
	if !ok {
		rec.Error = detail
	}
	yyQQActions.log(rec)
}

// yySrvInfoHandleGuard 危险指令名单的读写。玩家可以关掉任何规则（含默认锁定的
// 6 条——服务器是他的，开关归他，不锁死），也可以加自己的前缀。
func yySrvInfoHandleGuard(w http.ResponseWriter, r *http.Request) {
	cf := yyQQSnapshot()
	switch r.Method {
	case http.MethodGet:
		disabled := map[string]bool{}
		for _, v := range cf.Assistant.DangerDisabled {
			disabled[strings.ToLower(strings.TrimSpace(v))] = true
		}
		rows := make([]map[string]any, 0, len(yySrvInfoDangerRules))
		for _, rule := range yySrvInfoDangerRules {
			rows = append(rows, map[string]any{
				"prefix": rule.Prefix, "reason": rule.Reason, "locked": rule.Locked,
				"enabled": !disabled[strings.ToLower(strings.TrimSpace(rule.Prefix))],
			})
		}
		writeJSON(w, map[string]any{"ok": true, "rules": rows,
			"extra": cf.Assistant.DangerExtra,
			"note":  "默认 31 条，op/deop/stop/ban/ban-ip/whitelist off 默认锁定；锁不锁你说了算，关掉的指令将直接执行。",
		})
	case http.MethodPost:
		var p struct {
			Disabled []string `json:"disabled"`
			Extra    []string `json:"extra"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		if len(p.Disabled) > 200 || len(p.Extra) > 200 {
			writeJSON(w, map[string]any{"ok": false, "error": "规则太多（上限 200 条）"})
			return
		}
		clean := func(in []string) []string {
			out := make([]string, 0, len(in))
			seen := map[string]bool{}
			for _, v := range in {
				s := strings.ToLower(strings.TrimSpace(v))
				if s == "" || len([]rune(s)) > 64 || seen[s] {
					continue
				}
				seen[s] = true
				out = append(out, s)
			}
			return out
		}
		disabled := clean(p.Disabled)
		extra := clean(p.Extra)
		if e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
			a.DangerDisabled = disabled
			a.DangerExtra = extra
		}); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "disabled": disabled, "extra": extra})
	default:
		http.Error(w, "GET/POST required", 405)
	}
}

// ---- 6. 单个玩家聚合 ----

// yySrvInfoHandlePlayer 一次给齐「点开一个人」需要的全部字段。
//
// 为什么不在前端拼：列表每页只放三样（头像/名字/在线），点开才需要 QQ、权限、
// 在线时长、发言数、最近一句话、皮肤卡、积分。如果前端并发打 4 个接口，
// 抽屉会先闪一次空壳、再逐块补齐，看起来像坏了；这里只读投影一次返回。
//
// 数据来源全部是已有的内存结构，本函数不写任何状态：
//
//	playerStats / Bindings → 时长、发言、最近发言、QQ
//	yyQQRole               → 0/1/2（成员/管理员/超管）
//	yyLevelXP + 曲线        → 等级、本级经验、升级所需
//	/api/qq/player-card    → 形象预览（这里只给 URL，PNG 由那个接口渲染）
func yySrvInfoHandlePlayer(w http.ResponseWriter, r *http.Request) {
	if !yySrvInfoRequireGet(w, r) {
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少 name 参数"})
		return
	}
	cf := yyQQSnapshot()
	now := time.Now()
	qq := yySrvInfoReverseBindings()[name]

	yyQQHelper.Lock()
	v := yyQQHelper.Stats[name]
	row := yySrvInfoPlayerRow{Player: name, QQ: qq}
	if v != nil {
		row.OnlineSeconds = yySrvInfoOnlineSeconds(v, now)
		row.Messages = v.Messages
		row.LastMessage = v.LastMessage
		row.Joined = yySrvInfoStamp(v.Joined)
		row.LastJoin = yySrvInfoStamp(v.LastJoin)
		row.LastLeave = yySrvInfoStamp(v.LastLeave)
	}
	yyQQHelper.Unlock()

	if v == nil && qq == "" {
		// 名字打错和「没进过服」对用户是同一件事：都查不到，给一句能改的话。
		writeJSON(w, map[string]any{
			"ok":    false,
			"error": "没有找到这个玩家：可能名字打错了，或者他还没进过服",
		})
		return
	}
	row.Avatar = yySrvInfoPlayerAvatar(name)
	if set, known := yySrvInfoOnlineSet(); known {
		on := set[name]
		row.Online = &on
	}

	role := yyQQRole(cf, qq)

	// 积分：等级按 QQ 记账，因此没绑 QQ 的人天然没有积分，要说清楚原因。
	group := yySrvInfoPrimaryGroup(cf)
	lvlCfg := yyLevelConfigSnapshot()
	xp := 0
	if qq != "" {
		if qqID, e := strconv.ParseInt(qq, 10, 64); e == nil {
			xp = yyLevelXP(group, qqID)
		}
	}
	level, cur, need, pct := yyLevelProgressOf(lvlCfg, xp)
	levelOut := map[string]any{
		"enabled": lvlCfg.Enabled,
		"level":   level,
		"xp":      xp,
		"cur":     cur,
		"need":    need,
		"percent": int(pct),
		"group":   group,
	}
	switch {
	case !lvlCfg.Enabled:
		levelOut["note"] = "积分（群活跃等级）没有开启，可在「等级」面板打开"
	case qq == "":
		levelOut["note"] = "这个人还没绑定 QQ，积分按 QQ 记账，所以查不到"
	case xp == 0:
		levelOut["note"] = "还没有积分：需要在群里说过话才会累计"
	}

	out := map[string]any{
		"ok":             true,
		"player":         row.Player,
		"qq":             row.QQ,
		"role":           role,
		"role_text":      yyQQRoleName(role),
		"online":         row.Online,
		"online_seconds": row.OnlineSeconds,
		"messages":       row.Messages,
		"last_message":   row.LastMessage,
		"joined":         row.Joined,
		"last_join":      row.LastJoin,
		"last_leave":     row.LastLeave,
		"avatar":         row.Avatar,
		// 形象预览：服务端渲染好的整张玩家卡，直接当 <img src> 用。
		"card":  "/api/qq/player-card?player=" + url.QueryEscape(name),
		"level": levelOut,
		// 标签留空数组而不是 null：前端 forEach 不用判空，也不会误显示成「0 个」。
		// 没有数据源就不编数据——插件原生的标签接口还没接进来。
		"tags":      []any{},
		"tags_note": "玩家标签需要插件原生接口，当前版本还没接入，先留空",
	}
	if _, known := yySrvInfoOnlineSet(); !known {
		out["note"] = "游戏未连接或在线名单尚未同步，online 为 null 表示未知"
	}
	writeJSON(w, out)
}
