package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AI 接话的三种模式（玩家在面板上选，存在 assistant.ai_mode 里，不新开配置文件）。
//
// 用户原话：「召唤AI 现在你给他改成了 .AI 这种方式，但是这样他不就限定了吗？」
// 「让选择一段时间之后再去看，他自己去选择想回的内容，你仅仅只需要提供一段时间让AI去看一次的间隔」
//
// 所以三种模式并存，谁也不替代谁：
//
//	mention  —— 只在被叫时说（.AI / @机器人），老行为，缺省值；
//	interval —— 定时自检：每 N 分钟把「上次自检之后的新消息」交给 AI，
//	            回不回、回哪条、回什么全由提示词决定，程序只提供「多久看一次」；
//	auto     —— 全自动：每条群消息都判断一次要不要接（有成本闸门）。
//
// 本文件只负责「什么时候把话交给 AI」；说什么由 youyuan_qq_ai.go 的提示词链路决定。
const (
	yyQQAIModeMention  = "mention"
	yyQQAIModeInterval = "interval"
	yyQQAIModeAuto     = "auto"
)

const (
	// 全自动模式的成本闸门：同一个人 60 秒内只判一次。没有它，群里刷屏
	// 就是每条一次模型调用，配额会被瞬间打光。
	yyQQAIAutoUserGap = 60 * time.Second
	// 每轮自检最多把多少条新消息塞进提示词（再多也只是重复上下文，纯烧 token）。
	yyQQAIIntervalScan = 40
	// 自检的轮询粒度。间隔下限是 1 分钟，20 秒一判足够准，也不至于空转太频。
	yyQQAIModeTickEvery = 20 * time.Second
	// 模型明确表示「不回」时用的标记。用方括号 ASCII，正常中文回复几乎不会撞上。
	yyQQAISkipMarker = "[SKIP]"
	// 自检状态表的上限，防止长跑进程里按用户累积的 key 无限涨。
	yyQQAIModeStateCap = 1024
)

// 两段提示词是「回到用户手上的可塑性」的核心：程序不替模型决定回不回。
// 它们进系统提示词（跟着 cf 走），不进群消息正文——否则指令会被当成
// 「群友说过的话」写进 AI 记忆，后面几轮都在复读任务说明。
const (
	yyQQAIIntervalTask = "【定时自检任务】系统每隔一段时间会把你上次自检之后的群聊新消息交给你一次，" +
		"由你自己决定要不要接话：想接就挑一条你想回的直接回（可以带上对方昵称，也可以就着话题说一句）；" +
		"不想接就只输出 " + yyQQAISkipMarker + "，不要解释、不要总结、不要列清单。" +
		"没人要求你必须说话，也没人规定你必须回哪一条；群里没人说话时沉默才是正常的。"
	yyQQAIAutoTask = "【自动接话任务】群里每来一条消息，系统都会让你判断一次要不要接：" +
		"值得接就自然地回一句（通常一两句）；不值得接就只输出 " + yyQQAISkipMarker + "，不要解释。" +
		"别每条都接——群里正常聊天时，大部分消息都不需要你开口。"
)

// yyQQAIWithTask 把本次模式的任务说明挂到系统提示词前面。
// 放在玩家自定义 Prompt 之前，玩家的设定仍然是最高优先级。
func yyQQAIWithTask(cf yyQQConfig, task string) yyQQConfig {
	cf.Assistant.Prompt = strings.TrimSpace(task + "\n" + strings.TrimSpace(cf.Assistant.Prompt))
	return cf
}

// yyQQAIMode 归一化 ai_mode：空值/拼错/旧配置一律退回 mention（现状），
// 保证升级后没人动过开关的服务器行为完全不变。
func yyQQAIMode(cf yyQQConfig) string {
	switch strings.ToLower(strings.TrimSpace(cf.Assistant.AIMode)) {
	case yyQQAIModeInterval:
		return yyQQAIModeInterval
	case yyQQAIModeAuto:
		return yyQQAIModeAuto
	}
	return yyQQAIModeMention
}

// yyQQAIIntervalMinutes 返回夹取后的自检间隔分钟数：未设置(0) 兜底 10，夹到 1..1440。
// 读取时夹取而不是保存时夹取——面板填了越界值也只会被修正，不会让通道卡死。
func yyQQAIIntervalMinutes(cf yyQQConfig) int {
	m := cf.Assistant.AIIntervalMinutes
	if m <= 0 {
		m = 10
	}
	if m < 1 {
		m = 1
	}
	if m > 1440 {
		m = 1440
	}
	return m
}

// yyQQAINightBounds 深夜时段边界（0..23）。默认 23 点开始、7 点结束；相等 = 不降权。
func yyQQAINightBounds(cf yyQQConfig) (int, int) {
	start, end := cf.Assistant.AINightStartHour, cf.Assistant.AINightEndHour
	if start < 0 || start > 23 {
		start = 23
	}
	if end < 0 || end > 23 {
		end = 7
	}
	return start, end
}

// yyQQAINightFactor 深夜降权倍数（1..10，默认 2）。倍数的含义是「判断间隔放宽几倍」：
// 2 = 半夜每两轮才看一次。用倍数而不是概率，是为了让行为可预期、可复验。
func yyQQAINightFactor(cf yyQQConfig) int {
	f := cf.Assistant.AINightFactor
	if f <= 0 {
		f = 2
	}
	if f > 10 {
		f = 10
	}
	return f
}

// yyQQAIInNight 当前是否落在深夜时段。跨零点区间（23..7）要按两段算。
func yyQQAIInNight(cf yyQQConfig, now time.Time) bool {
	start, end := yyQQAINightBounds(cf)
	if start == end {
		return false
	}
	h := now.Hour()
	if start < end {
		return h >= start && h < end
	}
	return h >= start || h < end
}

// yyQQAIParseClock 解析 "HH:MM"/"HH:MM:SS" 为「当天第几分钟」。空串或非法返回 ok=false。
func yyQQAIParseClock(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		return 0, false
	}
	h, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// yyQQAIQuietBounds 静默窗口（"HH:MM"）。两端都填了才生效，否则视为未设置。
func yyQQAIQuietBounds(cf yyQQConfig) (int, int, bool) {
	start, okStart := yyQQAIParseClock(cf.Assistant.AIQuietStart)
	end, okEnd := yyQQAIParseClock(cf.Assistant.AIQuietEnd)
	if !okStart || !okEnd || start == end {
		return 0, 0, false
	}
	return start, end, true
}

// yyQQAIInQuiet 当前是否落在静默窗口内。窗口内定时自检与全自动都不开口，
// 但被 @ 和 .AI 仍然照常回——静默是「别主动找人说话」，不是「拒答」。
func yyQQAIInQuiet(cf yyQQConfig, now time.Time) bool {
	start, end, ok := yyQQAIQuietBounds(cf)
	if !ok {
		return false
	}
	cur := now.Hour()*60 + now.Minute()
	if start < end {
		return cur >= start && cur < end
	}
	return cur >= start || cur < end
}

// 调度状态：只在内存里，不落盘——它记的是「这一次进程里看到哪了」，
// 落盘反而会让重启后的第一轮误以为很久没自检过。重启后按「最近一个间隔」重新起算。
var yyQQAIModeState = struct {
	sync.Mutex
	LastCheck map[string]time.Time // 群 -> 上次发起自检的时间
	Cursor    map[string]time.Time // 群 -> 已交付给 AI 的最新消息时间
	LastAuto  map[string]time.Time // 群+用户 -> 上次自动判断时间
}{LastCheck: map[string]time.Time{}, Cursor: map[string]time.Time{}, LastAuto: map[string]time.Time{}}

var yyQQAIModeLoopOnce sync.Once

// yyQQAIModeLoopStart 起自检循环，重复调用只起一次。
// 这里用懒启动（第一条群消息进来时调）而不是改别人的启动文件：
// 14 个 agent 并行开发，少碰一个共享文件就少一次冲突。
func yyQQAIModeLoopStart() {
	yyQQAIModeLoopOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(yyQQAIModeTickEvery)
			defer ticker.Stop()
			for now := range ticker.C {
				yyQQAIModeTick(now)
			}
		}()
	})
}

// yyQQAIModeTick 是定时器的一轮：只有 interval 模式需要它。
func yyQQAIModeTick(now time.Time) {
	cf := yyQQSnapshot()
	if !cf.Enabled || !cf.Assistant.Enabled || cf.Assistant.Model == "" {
		return
	}
	if yyQQAIMode(cf) != yyQQAIModeInterval {
		return
	}
	if aiStore.Get(cf.Assistant.Model) == nil {
		return
	}
	// 多群：每条消息都是按群存储的（yyQQMessageKey 含群标识），
	// 所以自检也要逐群判断，不能只盯 GroupID 那一个。
	for _, group := range yyQQGroups(cf) {
		if !group.Enabled || group.ID == "" {
			continue
		}
		perGroup := cf
		perGroup.GroupID = group.ID
		yyQQAIIntervalMaybe(perGroup, now)
	}
}

// yyQQAIIntervalMaybe 判断这个群这一轮是不是该自检了。
func yyQQAIIntervalMaybe(cf yyQQConfig, now time.Time) {
	if yyQQAIInQuiet(cf, now) {
		return
	}
	// 槽位满了就整轮跳过，并且**不推进 LastCheck**：等下一轮重试，
	// 否则这十分钟的新消息会被永久跳过。
	if len(yyQQAISlots) >= cap(yyQQAISlots) {
		return
	}
	key := yyQQMessageKey(cf)
	interval := time.Duration(yyQQAIIntervalMinutes(cf)) * time.Minute
	if yyQQAIInNight(cf, now) {
		interval *= time.Duration(yyQQAINightFactor(cf))
	}
	yyQQAIModeState.Lock()
	last := yyQQAIModeState.LastCheck[key]
	if !last.IsZero() && now.Sub(last) < interval {
		yyQQAIModeState.Unlock()
		return
	}
	yyQQAIModeState.LastCheck[key] = now
	since := yyQQAIModeState.Cursor[key]
	yyQQAIModeState.Unlock()
	// 第一次跑（刚切到 interval，或进程刚重启）没有游标：只看「最近一个间隔」
	// 的新消息，不要拿缓冲区里的陈年旧账去让 AI 接话。
	if since.IsZero() {
		since = now.Add(-interval)
	}
	go yyQQAIIntervalRun(cf, since)
}

// yyQQAIIntervalRun 把「上次自检之后的新消息」交给 AI，由它自己决定要不要接。
func yyQQAIIntervalRun(cf yyQQConfig, since time.Time) {
	rows := yyQQRecentMessages(cf, 60)
	bot := yyQQBotID(cf)
	fresh := make([]yyQQGroupMessage, 0, len(rows))
	var newest time.Time
	for _, row := range rows {
		if row.Timestamp.After(newest) {
			newest = row.Timestamp
		}
		if row.IsBot || row.User == "" || row.User == bot {
			continue
		}
		if !since.IsZero() && !row.Timestamp.After(since) {
			continue
		}
		fresh = append(fresh, row)
	}
	// 游标先推进再生成：生成失败也不该在下一轮把同一批消息再喂一遍模型
	//（失败重试会把配额烧在重复内容上）。
	if !newest.IsZero() {
		yyQQAIModeState.Lock()
		yyQQAIModeState.Cursor[yyQQMessageKey(cf)] = newest
		yyQQAIModeState.Unlock()
	}
	if len(fresh) == 0 {
		// 这段时间群里没人说话：自检没什么可看的，安静跳过。
		return
	}
	if len(fresh) > yyQQAIIntervalScan {
		fresh = fresh[len(fresh)-yyQQAIIntervalScan:]
	}
	var text strings.Builder
	text.WriteString("上次自检之后，群里新出现了这些消息：\n")
	for _, row := range fresh {
		fmt.Fprintf(&text, "%s：%s\n", row.Name, yyQQClip(row.Text, 200))
	}
	text.WriteString("现在轮到你决定要不要接话。")
	// 用最后一位发言者当锚点：好感度提示、历史会话都会落到「正在聊的人」身上，
	// 比传空用户更自然。replyID 留空，回哪条由模型自己决定。
	anchor := fresh[len(fresh)-1]
	chat := yyQQAIWithTask(cf, yyQQAIIntervalTask)
	answer, e := yyQQAskAIWithReplyGuard(chat, anchor.User, text.String(), "", nil)
	if e != nil {
		yyQQLog("定时自检失败：" + e.Error())
		yyQQAIFailLog("[" + yyQQGroupString(cf.GroupID) + "] 定时自检失败：" + e.Error())
		return
	}
	answer, skip := yyQQAISkipText(answer)
	if skip || answer == "" {
		yyQQLog(fmt.Sprintf("定时自检：AI 选择不接话（新消息 %d 条）", len(fresh)))
		return
	}
	yyQQFeedbackTo(cf, answer, "")
}

// yyQQAIModeAutoJudge 是全自动模式的入口：每条群消息都问一次 AI「这条要不要接」。
// 成本闸门有三道：同一用户 60 秒内只判一次、深夜按倍数放宽、静默窗口内完全不判。
func yyQQAIModeAutoJudge(cf yyQQConfig, user, text, id string) {
	if yyQQAIMode(cf) != yyQQAIModeAuto {
		return
	}
	if user == "" || strings.TrimSpace(text) == "" {
		return
	}
	// 指令（.开头/斜杠）有自己的确定性处理，不烧模型配额。
	if strings.HasPrefix(text, ".") || strings.HasPrefix(text, "/") {
		return
	}
	if aiStore.Get(cf.Assistant.Model) == nil {
		return
	}
	now := time.Now()
	if yyQQAIInQuiet(cf, now) {
		return
	}
	gap := yyQQAIAutoUserGap
	if yyQQAIInNight(cf, now) {
		gap *= time.Duration(yyQQAINightFactor(cf))
	}
	key := yyQQMessageKey(cf) + "\x00" + user
	yyQQAIModeState.Lock()
	last := yyQQAIModeState.LastAuto[key]
	if !last.IsZero() && now.Sub(last) < gap {
		yyQQAIModeState.Unlock()
		return
	}
	yyQQAIModeState.LastAuto[key] = now
	if len(yyQQAIModeState.LastAuto) > yyQQAIModeStateCap {
		for k, at := range yyQQAIModeState.LastAuto {
			if now.Sub(at) > time.Hour {
				delete(yyQQAIModeState.LastAuto, k)
			}
		}
	}
	yyQQAIModeState.Unlock()
	if len(yyQQAISlots) >= cap(yyQQAISlots) {
		return
	}
	go func() {
		chat := yyQQAIWithTask(cf, yyQQAIAutoTask)
		answer, e := yyQQAskAIWithReplyGuard(chat, user, text, id, nil)
		if e != nil {
			yyQQLog("自动接话失败：" + e.Error())
			return
		}
		answer, skip := yyQQAISkipText(answer)
		if skip || answer == "" {
			return
		}
		yyQQFeedbackTo(cf, answer, id)
	}()
}

// yyQQAISkipText 识别模型「选择不回」的答复。
// 只要答复里出现跳过标记就整条丢弃——模型既然表态了，就不该把标记本身发到群里。
func yyQQAISkipText(answer string) (string, bool) {
	clean := strings.TrimSpace(answer)
	if clean == "" {
		return "", true
	}
	if strings.Contains(strings.ToUpper(clean), yyQQAISkipMarker) || strings.Contains(clean, "【跳过】") {
		return "", true
	}
	return clean, false
}
