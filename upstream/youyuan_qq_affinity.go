package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// QQ 群「按人好感度 + 情绪状态」。
//
// 设计要点：
//   - 每个人一条记录（按 QQ 存），好感 -100..100，0 为中性；
//   - 情绪由好感度 + 最近 5 分钟是否被骂共同推导，**不是**让模型自由发挥；
//   - 行为 → 好感增量集中成表（yyAffRules），便于调参和测试对齐；
//   - 落盘 <dataDir>/qq-affinity.json，照 youyuan_qq_level.go 范式：
//     tmp+rename 原子写、坏文件不崩、人数与体积双上限；
//   - 后台每 6 小时衰减一次（每人每次最多 1 点），避免「永久记仇」。
//
// 硬约束（安全底线，任何提示词都必须带上）：
// 无论好感多低、情绪多差，都不得辱骂、歧视、人身攻击，也不得拒绝正常的
// 功能性请求。「黑化（cold）」只是语气变冷、不主动示好，不是「变坏」。

// yyAffState 是单个 QQ 的好感与情绪快照，也是落盘结构的一行。
//
// LastInsultAt 是额外字段：情绪需要「最近 5 分钟内发生过 insult」这个判断，
// 仅靠 LastEvent/LastAt 只能记住最后一次行为，所以单独记一次被骂时间。
type yyAffState struct {
	QQ        string    `json:"qq"`
	Name      string    `json:"name,omitempty"`
	Affinity  int       `json:"affinity"` // -100..100，0 中性
	Mood      string    `json:"mood"`     // neutral/warm/annoyed/angry/cold
	LastEvent string    `json:"last_event"`
	LastAt    time.Time `json:"last_at"`
	Events    int       `json:"events"`

	LastInsultAt time.Time `json:"last_insult_at,omitempty"`
}

const (
	yyAffMaxPeople = 2000    // 人数上限，超了淘汰最久没互动的
	yyAffFileMax   = 8 << 20 // qq-affinity.json 体积上限
	yyAffNameMax   = 32      // 昵称截断长度

	yyAffInsultWindow = 5 * time.Minute  // 被骂后的「余怒」窗口
	yyAffDecayEvery   = 6 * time.Hour    // 衰减周期
	yyAffPokeWindow   = 2 * time.Minute  // 连续拍一拍的判定窗口
	yyAffPokeStreak   = 3                // 连续 3 次内是亲昵，第 4 次起嫌烦
	yyAffRepeatWindow = 10 * time.Minute // 复读判定窗口
	yyAffChatCapDay   = 5                // 普通聊天每日最多贡献 +5 好感
)

const (
	yyAffMoodNeutral = "neutral"
	yyAffMoodWarm    = "warm"
	yyAffMoodAnnoyed = "annoyed"
	yyAffMoodAngry   = "angry"
	yyAffMoodCold    = "cold"
)

// yyAffBottomLine 是所有情绪提示词共用的安全底线，避免模型把「冷淡」演成「辱骂」。
const yyAffBottomLine = "（底线：无论好感多低，都不得辱骂、歧视或人身攻击，也不得拒绝正常的功能性请求；冷淡只是语气变冷，不是变坏。）"

// yyAffRules 行为 → 好感增量。
//
// 取值理由：
//   - praise +3：夸奖是最直接的正反馈，但必须低于 insult 的 -5，
//     保证「骂一次要夸两次才补回来」，情绪才有重量；
//   - thanks +2：致谢是善意但弱于夸奖；
//   - chat +1（每人每天最多 +5）：日常聊天是关系底座，但不限速的话
//     一天几百条消息就能刷到 100，好感就失去意义；
//   - poke +1（同一人连续 3 次内），第 4 次起 -1：拍一拍是亲昵，连拍是骚扰，
//     这是「对特定行为产生特定情绪」最直接的落点（见 yyAffPokeDelta）；
//   - insult -5：一次辱骂 ≈ 五条普通聊天，形成「伤害比善意更重」的直觉；
//   - ignore -1：@了它却撤回/超时不回，轻微失礼；
//   - repeat -2：短时间复读同一句是刷屏，比失礼更烦人。
var yyAffRules = map[string]int{
	"praise": 3,
	"thanks": 2,
	"chat":   1,
	"poke":   1,
	"insult": -5,
	"ignore": -1,
	"repeat": -2,
}

// yyAffNow 时钟注入点：测试可冻结/推进时间（衰减、余怒、复读窗口都走它）。
var yyAffNow = time.Now

// yyAff 全局状态，互斥锁保护。
var yyAff = struct {
	sync.Mutex
	Path      string
	People    map[string]yyAffState
	DiskError string
}{People: map[string]yyAffState{}}

// yyAffChatQuota 普通聊天的「每人每天 +5」配额，仅内存（重启当日额度刷新，
// 与等级冷却表同级别容错，不值得为它多写一个字段）。
var yyAffChatQuota = struct {
	sync.Mutex
	used map[string]int // qq + "\x00" + 2006-01-02 → 当日已用次数
}{used: map[string]int{}}

// yyAffPokes 连续拍一拍计数，仅内存。
var yyAffPokes = struct {
	sync.Mutex
	last map[string]yyAffPoke
}{last: map[string]yyAffPoke{}}

type yyAffPoke struct {
	Count int
	At    time.Time
}

// yyAffRepeats 复读检测：记住某人上一句原文与时间，仅内存。
var yyAffRepeats = struct {
	sync.Mutex
	last map[string]yyAffRepeat
}{last: map[string]yyAffRepeat{}}

type yyAffRepeat struct {
	Text string
	At   time.Time
}

func yyAffClamp(v int) int {
	if v > 100 {
		return 100
	}
	if v < -100 {
		return -100
	}
	return v
}

// yyAffMoodOf 由好感度推导情绪。
//
// 阈值（与需求一致）：
//
//	>= 60  warm    挺喜欢
//	>= -10 neutral 普通（覆盖 >=10 与 >=-10 两段，中间没有缝）
//	>= -30 annoyed 不耐烦
//	>= -60 angry   上火
//	<  -60 cold    黑化：语气冷淡
//
// recentInsult 为真时，情绪至少降到 angry（原本更低就保持），
// 这样「刚被骂完」不会因为历史好感高而表现得没事人一样。
func yyAffMoodOf(affinity int, recentInsult bool) string {
	mood := yyAffMoodNeutral
	switch {
	case affinity >= 60:
		mood = yyAffMoodWarm
	case affinity >= -10:
		mood = yyAffMoodNeutral
	case affinity >= -30:
		mood = yyAffMoodAnnoyed
	case affinity >= -60:
		mood = yyAffMoodAngry
	default:
		mood = yyAffMoodCold
	}
	if recentInsult && mood != yyAffMoodAngry && mood != yyAffMoodCold {
		mood = yyAffMoodAngry
	}
	return mood
}

// yyAffRecentInsult 判断「最近 5 分钟内被这个人骂过」。时钟回拨（负时长）也按余怒处理。
func yyAffRecentInsult(st yyAffState, now time.Time) bool {
	return !st.LastInsultAt.IsZero() && now.Sub(st.LastInsultAt) < yyAffInsultWindow
}

// yyAffChatBudget 普通聊天配额：每人每天最多计入 5 次 +1。
func yyAffChatBudget(qq string) bool {
	day := yyAffNow().Format("2006-01-02")
	key := qq + "\x00" + day
	yyAffChatQuota.Lock()
	defer yyAffChatQuota.Unlock()
	if len(yyAffChatQuota.used) > 8192 {
		// 有界：清掉非今天的旧配额，仍超限就整体重置（配额只是防刷，丢一天可接受）。
		for k := range yyAffChatQuota.used {
			if !strings.HasSuffix(k, "\x00"+day) {
				delete(yyAffChatQuota.used, k)
			}
		}
		if len(yyAffChatQuota.used) > 8192 {
			yyAffChatQuota.used = map[string]int{}
		}
	}
	if yyAffChatQuota.used[key] >= yyAffChatCapDay {
		return false
	}
	yyAffChatQuota.used[key]++
	return true
}

// yyAffPokeDelta 连续拍一拍：2 分钟窗口内前 3 次 +1，之后 -1（烦了）。
func yyAffPokeDelta(qq string) int {
	now := yyAffNow()
	yyAffPokes.Lock()
	defer yyAffPokes.Unlock()
	p := yyAffPokes.last[qq]
	if p.At.IsZero() || now.Sub(p.At) > yyAffPokeWindow {
		p.Count = 0
	}
	p.Count++
	p.At = now
	if len(yyAffPokes.last) >= 4096 {
		// 有界：淘汰最久一条，新条目永不因表满被丢弃。
		oldestKey, oldestAt := "", time.Time{}
		for k, v := range yyAffPokes.last {
			if oldestKey == "" || v.At.Before(oldestAt) {
				oldestKey, oldestAt = k, v.At
			}
		}
		delete(yyAffPokes.last, oldestKey)
	}
	yyAffPokes.last[qq] = p
	if p.Count <= yyAffPokeStreak {
		return 1
	}
	return -1
}

// yyAffIsRepeat 记录并判断「同一人短时间内重复同一句」。
// 判定前先写入本条，因此只有第二条起才算复读。
func yyAffIsRepeat(qq, text string) bool {
	if text == "" {
		return false
	}
	now := yyAffNow()
	yyAffRepeats.Lock()
	defer yyAffRepeats.Unlock()
	prev, ok := yyAffRepeats.last[qq]
	repeat := ok && prev.Text == text && now.Sub(prev.At) <= yyAffRepeatWindow
	if len(yyAffRepeats.last) >= 4096 {
		oldestKey, oldestAt := "", time.Time{}
		for k, v := range yyAffRepeats.last {
			if oldestKey == "" || v.At.Before(oldestAt) {
				oldestKey, oldestAt = k, v.At
			}
		}
		delete(yyAffRepeats.last, oldestKey)
	}
	yyAffRepeats.last[qq] = yyAffRepeat{Text: text, At: now}
	return repeat
}

// yyAffEvictOldestLocked 人数上限保护：淘汰 LastAt 最久远的一条。调用方需持锁。
func yyAffEvictOldestLocked() {
	oldestKey, oldestAt := "", time.Time{}
	for k, st := range yyAff.People {
		if oldestKey == "" || st.LastAt.Before(oldestAt) {
			oldestKey, oldestAt = k, st.LastAt
		}
	}
	if oldestKey != "" {
		delete(yyAff.People, oldestKey)
	}
}

// yyAffObserve 核心入口：记录一次行为并返回最新状态。
//
// kind 取值见 yyAffRules；未知 kind 按 chat 处理（宁可少算，不乱加分）。
// 同一 QQ 的行为串行落库，保证「读-改-写」不丢更新。
func yyAffObserve(qq, name, kind string) yyAffState {
	qq = strings.TrimSpace(qq)
	if qq == "" {
		return yyAffState{}
	}
	kind = strings.TrimSpace(kind)
	if _, ok := yyAffRules[kind]; !ok {
		kind = "chat"
	}

	// 增量先算（配额/连拍是有状态的，必须在锁外按序消费）。
	delta := 0
	switch kind {
	case "chat":
		if yyAffChatBudget(qq) {
			delta = yyAffRules["chat"]
		}
	case "poke":
		delta = yyAffPokeDelta(qq)
	default:
		delta = yyAffRules[kind]
	}

	now := yyAffNow()
	yyAff.Lock()
	defer yyAff.Unlock()
	st, exists := yyAff.People[qq]
	if !exists && len(yyAff.People) >= yyAffMaxPeople {
		yyAffEvictOldestLocked()
	}
	st.QQ = qq
	if trimmed := strings.TrimSpace(yyQQClip(name, yyAffNameMax)); trimmed != "" {
		st.Name = trimmed
	}
	st.Affinity = yyAffClamp(st.Affinity + delta)
	st.LastEvent = kind
	st.LastAt = now
	st.Events++
	if kind == "insult" {
		st.LastInsultAt = now
	}
	st.Mood = yyAffMoodOf(st.Affinity, yyAffRecentInsult(st, now))
	yyAff.People[qq] = st
	yyAffSaveLocked()
	return st
}

// ---------- 群消息接入 ----------

var (
	yyAffPraiseWords = []string{"好棒", "真棒", "太棒", "厉害", "优秀", "喜欢你", "爱你", "崇拜", "牛", "good", "great", "awesome", "nice", "love you"}
	yyAffThanksWords = []string{"谢谢", "感谢", "谢了", "多谢", "辛苦", "thanks", "thank you", "thx"}
	yyAffInsultWords = []string{"滚", "傻", "蠢", "闭嘴", "废物", "垃圾", "去死", "智障", "有病", "stupid", "idiot", "shut up"}
)

// yyAffClassify 关键词分类。粗粒度匹配（子串）是刻意的：
// 误判代价只有一两点好感，比引入分类器便宜得多；后续要升级只需改这里。
func yyAffClassify(text string) string {
	lower := strings.ToLower(text)
	for _, w := range yyAffInsultWords {
		if strings.Contains(lower, w) {
			return "insult"
		}
	}
	for _, w := range yyAffThanksWords {
		if strings.Contains(lower, w) {
			return "thanks"
		}
	}
	for _, w := range yyAffPraiseWords {
		if strings.Contains(lower, w) {
			return "praise"
		}
	}
	return "chat"
}

// yyAffOnMessage 群消息入口钩子：识别 praise/thanks/insult/chat/repeat 并更新好感。
//
// 优先级：insult（最重）> repeat（防复读刷正面词）> thanks > praise > chat。
// 空文本（图片/表情包）不记账，直接返回当前状态。
func yyAffOnMessage(qq, name, text string) yyAffState {
	qq = strings.TrimSpace(qq)
	text = strings.TrimSpace(text)
	if qq == "" {
		return yyAffState{}
	}
	if text == "" {
		st, _ := yyAffStateOf(qq)
		return st
	}
	kind := ""
	if yyAffClassify(text) == "insult" {
		kind = "insult"
	} else if yyAffIsRepeat(qq, text) {
		kind = "repeat"
	} else {
		kind = yyAffClassify(text)
	}
	return yyAffObserve(qq, name, kind)
}

// ---------- 提示词 ----------

// yyAffPromptFor 返回一段中文短句，供 system prompt 追加。
//
// 硬约束（不要删）：所有分支都拼上 yyAffBottomLine。好感 -100 也只是「冷淡」，
// 不得辱骂、歧视、人身攻击，也不得拒绝正常的功能性请求。
func yyAffPromptFor(qq string) string {
	qq = strings.TrimSpace(qq)
	st, ok := yyAffStateOf(qq)
	who := qq
	if ok && st.Name != "" {
		who = st.Name
	}
	if strings.TrimSpace(who) == "" {
		who = "这个人"
	}
	if !ok {
		return fmt.Sprintf("你还不了解 %s，先自然交流，别太热情。%s", who, yyAffBottomLine)
	}
	mood := yyAffMoodOf(st.Affinity, yyAffRecentInsult(st, yyAffNow()))
	var line string
	switch mood {
	case yyAffMoodWarm:
		line = fmt.Sprintf("你挺喜欢 %s（好感 %d/100），可以更热情、更主动开玩笑。", who, st.Affinity)
	case yyAffMoodAnnoyed:
		line = fmt.Sprintf("你对 %s 有点不耐烦（好感 %d/100），回复简短一点，别主动找话题。", who, st.Affinity)
	case yyAffMoodAngry:
		line = fmt.Sprintf("你现在对 %s 有点火（好感 %d/100），回复短一点、带点不耐烦，但不要辱骂。", who, st.Affinity)
	case yyAffMoodCold:
		line = fmt.Sprintf("你已经不太想理 %s 了（好感 %d/100），语气冷淡、简短，不主动示好，但仍然提供事实性帮助。", who, st.Affinity)
	default:
		line = fmt.Sprintf("你和 %s 的关系普通，说话自然点。", who)
	}
	return line + yyAffBottomLine
}

// ---------- 衰减 ----------

// yyAffDecay 好感向 0 收敛：每人每次最多 1 点（不跨过 0），衰减后重算情绪。
// 每 6 小时一次 ≈ 一天最多 4 点，100 分大约 25 天回到中性，
// 既能淡忘旧怨，也不会让长期关系一夜清零。
func yyAffDecay() int {
	now := yyAffNow()
	yyAff.Lock()
	defer yyAff.Unlock()
	changed := 0
	for key, st := range yyAff.People {
		switch {
		case st.Affinity > 0:
			st.Affinity--
			changed++
		case st.Affinity < 0:
			st.Affinity++
			changed++
		}
		st.Mood = yyAffMoodOf(st.Affinity, yyAffRecentInsult(st, now))
		yyAff.People[key] = st
	}
	yyAffSaveLocked()
	return changed
}

// yyAffDecayOnce 保证后台衰减循环只启动一次（重复接线不会起多个 goroutine）。
var yyAffDecayOnce sync.Once

// yyAffStartDecayLoop 启动后台衰减（每 6 小时一次）。由接线方在启动阶段调用，
// 未调用则不会自动衰减；测试不调用，避免 goroutine 泄漏。
func yyAffStartDecayLoop() {
	yyAffDecayOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(yyAffDecayEvery)
			defer ticker.Stop()
			for range ticker.C {
				yyAffDecay()
			}
		}()
	})
}

// ---------- 持久化 ----------

type yyAffDisk struct {
	People map[string]yyAffState `json:"people"`
}

// yyAffInit 载入 qq-affinity.json。文件缺失/损坏/超限都不致命：
// 退回空表并记 DiskError，绝不让机器人起不来。
func yyAffInit(root string) {
	yyAff.Lock()
	defer yyAff.Unlock()
	yyAff.Path = filepath.Join(root, "qq-affinity.json")
	yyAff.People = map[string]yyAffState{}
	yyAff.DiskError = ""
	file, e := os.Open(yyAff.Path)
	if e != nil {
		return
	}
	defer file.Close()
	var disk yyAffDisk
	if e := json.NewDecoder(io.LimitReader(file, yyAffFileMax)).Decode(&disk); e != nil {
		yyAff.DiskError = "invalid_affinity_state"
		return
	}
	now := yyAffNow()
	for key, st := range disk.People {
		qq := strings.TrimSpace(st.QQ)
		if qq == "" {
			qq = strings.TrimSpace(key)
		}
		if qq == "" {
			continue // 无 QQ 的行直接丢弃
		}
		st.QQ = qq
		st.Affinity = yyAffClamp(st.Affinity)
		st.Name = strings.TrimSpace(yyQQClip(st.Name, yyAffNameMax))
		// 情绪一律按好感重算，容忍手改文件造成的 mood/affinity 不一致。
		st.Mood = yyAffMoodOf(st.Affinity, yyAffRecentInsult(st, now))
		yyAff.People[qq] = st
	}
	for len(yyAff.People) > yyAffMaxPeople {
		yyAffEvictOldestLocked()
	}
}

// yyAffSaveLocked 原子落盘（tmp + rename）。调用方需持锁。
func yyAffSaveLocked() {
	if yyAff.Path == "" {
		return // 未初始化（测试/纯内存）时不写盘
	}
	raw, e := json.Marshal(yyAffDisk{People: yyAff.People})
	if e == nil && len(raw) > yyAffFileMax {
		e = fmt.Errorf("affinity_size_limit")
	}
	if e == nil {
		if e = os.WriteFile(yyAff.Path+".tmp", raw, 0600); e == nil {
			e = os.Rename(yyAff.Path+".tmp", yyAff.Path)
		}
	}
	if e != nil {
		yyAff.DiskError = "affinity_persistence_failed"
	} else {
		yyAff.DiskError = ""
	}
}

// ---------- 查询 ----------

// yyAffStateOf 取单人快照；无记录返回 zero + false。
func yyAffStateOf(qq string) (yyAffState, bool) {
	yyAff.Lock()
	defer yyAff.Unlock()
	st, ok := yyAff.People[strings.TrimSpace(qq)]
	return st, ok
}

// yyAffTotal 已记录总人数。
func yyAffTotal() int {
	yyAff.Lock()
	defer yyAff.Unlock()
	return len(yyAff.People)
}

// yyAffTop 好感降序 Top N，同分按 QQ 升序保证输出确定。
func yyAffTop(n int) []yyAffState {
	if n <= 0 {
		n = 20
	}
	if n > 200 {
		n = 200
	}
	yyAff.Lock()
	rows := make([]yyAffState, 0, len(yyAff.People))
	for _, st := range yyAff.People {
		rows = append(rows, st)
	}
	yyAff.Unlock()
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Affinity != rows[j].Affinity {
			return rows[i].Affinity > rows[j].Affinity
		}
		return rows[i].QQ < rows[j].QQ
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// ---------- 面板接口 ----------

// yyAffRoutes 好感度面板接口。root 用于定位 qq-affinity.json。
// 只有接口层调用，不做权限判断（面板本身已受设备鉴权）。
func yyAffRoutes(mux *http.ServeMux, root string) {
	yyAffInit(root)
	mux.HandleFunc("/api/qq/affinity", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			yyAffGetAPI(w, r)
		case http.MethodPost:
			yyAffPostAPI(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
		}
	})
}

// yyAffGetAPI GET /api/qq/affinity?qq=：单个；不带 qq 返回 Top 20 + 总人数。
func yyAffGetAPI(w http.ResponseWriter, r *http.Request) {
	qq := strings.TrimSpace(r.URL.Query().Get("qq"))
	if qq != "" {
		st, ok := yyAffStateOf(qq)
		if !ok {
			writeJSON(w, map[string]any{
				"ok": true, "found": false, "qq": qq,
				"total": yyAffTotal(), "prompt": yyAffPromptFor(qq),
			})
			return
		}
		writeJSON(w, map[string]any{
			"ok": true, "found": true, "person": st,
			"total": yyAffTotal(), "prompt": yyAffPromptFor(qq),
		})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "total": yyAffTotal(), "top": yyAffTop(20)})
}

// yyAffPostAPI POST /api/qq/affinity：body {qq,name?,affinity?} 手动校正好感。
func yyAffPostAPI(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		QQ       string  `json:"qq"`
		Name     *string `json:"name"`
		Affinity *int    `json:"affinity"`
	}
	if e := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); e != nil {
		writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
		return
	}
	qq := strings.TrimSpace(payload.QQ)
	if qq == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "qq 不能为空"})
		return
	}
	now := yyAffNow()
	yyAff.Lock()
	st, ok := yyAff.People[qq]
	if !ok {
		st = yyAffState{QQ: qq, LastAt: now}
	}
	if payload.Name != nil {
		st.Name = strings.TrimSpace(yyQQClip(*payload.Name, yyAffNameMax))
	}
	if payload.Affinity != nil {
		st.Affinity = yyAffClamp(*payload.Affinity)
	}
	st.QQ = qq
	st.Mood = yyAffMoodOf(st.Affinity, yyAffRecentInsult(st, now))
	yyAff.People[qq] = st
	yyAffSaveLocked()
	yyAff.Unlock()
	writeJSON(w, map[string]any{"ok": true, "person": st, "prompt": yyAffPromptFor(qq)})
}
