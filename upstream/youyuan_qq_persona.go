package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 机器人人格卡 / 反 AI 腔 / 群内风格学习（设计 docs/design/bot-persona.md）。
//
// 目标：群里表现像「管事的老群友」，不是客服，也不是朗读提示词的模型。
// 本文件只做三件事：人格卡持久化（qq-persona.json）、反 AI 腔提示词块、
// 群风格统计档案（qq-style.json）。群风格只存统计量（频率/长度/时段），
// 不存发送者身份；提示词块是纯文本，供主控注入 youyuan_qq_ai.go 的 system prompt。

// yyQQPersona 人格卡：机器人以什么身份、什么语气在群里说话。
type yyQQPersona struct {
	Name    string   `json:"name"`    // 群里自称，如「老白」
	Persona string   `json:"persona"` // 人设描述
	Style   string   `json:"style"`   // 语言风格
	Taboos  []string `json:"taboos"`  // 禁忌
	AntiAI  []string `json:"anti_ai"` // 反 AI 腔规则
	Enabled bool     `json:"enabled"` // 总开关
}

// yyStyleConfig 群风格学习配置，可整体关闭。
type yyStyleConfig struct {
	Enabled bool `json:"enabled"`
	MinMsgs int  `json:"min_msgs"` // 少于此样本量不出风格摘要
	TopN    int  `json:"top_n"`    // 高频词取前 N 个
}

// yyStyleSample 群风格环形缓冲的一条：只留文本与时间，不存发送者。
type yyStyleSample struct {
	Text string    `json:"t"`
	At   time.Time `json:"at"`
}

// yyStyleWord 一个高频词及其出现次数。
type yyStyleWord struct {
	Text string `json:"t"`
	N    int    `json:"n"`
}

// yyStyleStat 一个群的风格统计结果。
type yyStyleStat struct {
	Group     string        `json:"group"`
	Samples   int           `json:"samples"`
	AvgRunes  float64       `json:"avg_runes"`
	FaceRatio float64       `json:"face_ratio"` // 带表情/图片标记的消息占比
	TopWords  []yyStyleWord `json:"top_words"`
	Hours     [24]int       `json:"hours"`      // 活跃时段直方图
	PeakHours [2]int        `json:"peak_hours"` // 最活跃时段起止（含）
}

const (
	yyQQPersonaBlockMax = 800 // 提示词块总长上限（字）
	yyQQStyleBufferMax  = 200 // 每群环形缓冲条数
	yyQQStyleMaxGroups  = 32  // 全局群上限（与 yyQQMemory 一致）
	yyQQStyleSampleClip = 500 // 单条样本截断（字）
	yyQQPersonaSummary  = 120 // 风格摘要上限（字）
)

// yyQQPersonaState 人格卡与风格配置的持久化状态，范式照 youyuan_qq_memory.go：
// 启动加载、变更即存、JSON 损坏回退默认并记日志。
var yyQQPersonaState = struct {
	sync.Mutex
	Path      string
	Persona   yyQQPersona
	Style     yyStyleConfig
	DiskError string
}{Persona: yyQQPersonaDefault(), Style: yyStyleConfigDefault()}

// yyQQStyleState 群风格样本环形缓冲，落盘 qq-style.json。
var yyQQStyleState = struct {
	sync.Mutex
	Path      string
	Groups    map[string][]yyStyleSample
	DiskError string
}{Groups: map[string][]yyStyleSample{}}

// yyQQPersonaAntiAIDefault 反 AI 腔 13 条硬规则，逐条落地 bot-persona.md §2.1。
func yyQQPersonaAntiAIDefault() []string {
	return []string{
		"禁开场白：不用「好的/当然/没问题/作为/很高兴」开头",
		"禁自称AI：不说「作为AI/人工智能/语言模型/助手」，也不说「我是AI」",
		"禁客服尾：不说「希望有帮助/还有什么可以帮您/如有需要」",
		"禁书面连接：不用「首先/其次/再次/最后/总之/综上/因此/然而/此外/并且」",
		"禁列表：不用「-」「*」「1.」逐条罗列",
		"禁markdown：不用加粗、标题、引用、代码块、表格",
		"单条要短：长内容拆成两三条短句，不一口气长篇大论",
		"句末句号去掉：真人打字很少在句尾打「。」",
		"语气词少用：「呢哦呀啦哈嘛」一句最多一个",
		"表情少用：一条最多带一个表情，不要每条都带",
		"禁复述：不重复对方消息里的原句",
		"禁总结：不说「总的来说/一句话概括/以上」",
		"禁排比：不连用三个结构相近的并列分句",
	}
}

// yyQQPersonaDefault 默认人设，从 bot-persona.md §1 的「老白」预设提炼：
// 群友语气、口语化、短句、可用表情、不暴露 AI 身份。
func yyQQPersonaDefault() yyQQPersona {
	return yyQQPersona{
		Name:    "老白",
		Persona: "开服就在、嘴上嫌弃但会管事的老群友，本服的事都门清。不是客服，不复读提示词，不暴露AI身份。",
		Style:   "群友语气，口语化，短句，像手机打字，可以带表情，不端着。",
		Taboos:  []string{"自称客服", "自称AI", "markdown排版", "长篇大论", "说教", "客气寒暄"},
		AntiAI:  yyQQPersonaAntiAIDefault(),
		Enabled: true,
	}
}

func yyStyleConfigDefault() yyStyleConfig {
	return yyStyleConfig{Enabled: true, MinMsgs: 20, TopN: 8}
}

// yyPersonaClip 按字数硬截断（不加标记），避免截出半个汉字。
func yyPersonaClip(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

// yyPersonaList 清洗字符串列表：去空白、丢空项、限条数、限条长。
func yyPersonaList(in []string, maxItems, maxRunes int) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, yyPersonaClip(v, maxRunes))
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

// yyQQPersonaNormalize 只做裁剪清洗，不填默认值；是否回退默认由调用方判定。
func yyQQPersonaNormalize(p yyQQPersona) yyQQPersona {
	p.Name = yyPersonaClip(strings.TrimSpace(p.Name), 24)
	p.Persona = yyPersonaClip(strings.TrimSpace(p.Persona), 400)
	p.Style = yyPersonaClip(strings.TrimSpace(p.Style), 400)
	p.Taboos = yyPersonaList(p.Taboos, 12, 40)
	p.AntiAI = yyPersonaList(p.AntiAI, 24, 160)
	return p
}

func yyStyleConfigNormalize(c yyStyleConfig) yyStyleConfig {
	if c.MinMsgs <= 0 {
		c.MinMsgs = 20
	}
	if c.MinMsgs > 500 {
		c.MinMsgs = 500
	}
	if c.TopN <= 0 {
		c.TopN = 8
	}
	if c.TopN > 20 {
		c.TopN = 20
	}
	return c
}

// yyQQPersonaInit 启动加载：人格配置与群风格样本两个文件。
// 文件缺失用默认；JSON 损坏回退默认并写日志（照 memory 范式）。
func yyQQPersonaInit(root string) {
	yyQQPersonaState.Lock()
	yyQQPersonaState.Path = filepath.Join(root, "qq-persona.json")
	file, e := os.Open(yyQQPersonaState.Path)
	if e != nil {
		yyQQPersonaState.Persona = yyQQPersonaDefault()
		yyQQPersonaState.Style = yyStyleConfigDefault()
		yyQQPersonaState.DiskError = ""
		yyQQPersonaState.Unlock()
		yyQQStyleInit(root)
		return
	}
	defer file.Close()
	var stored struct {
		Persona *yyQQPersona   `json:"persona"`
		Style   *yyStyleConfig `json:"style"`
	}
	if json.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&stored) != nil {
		yyQQPersonaState.DiskError = "invalid_persona_config"
		yyQQPersonaState.Persona = yyQQPersonaDefault()
		yyQQPersonaState.Style = yyStyleConfigDefault()
		yyQQPersonaState.Unlock()
		yyQQLog("QQ人格配置损坏，已回退默认设置")
		yyQQStyleInit(root)
		return
	}
	persona, style := yyQQPersonaDefault(), yyStyleConfigDefault()
	if stored.Persona != nil {
		if p := yyQQPersonaNormalize(*stored.Persona); p.Name != "" && len(p.AntiAI) > 0 {
			persona = p
		}
	}
	if stored.Style != nil {
		style = yyStyleConfigNormalize(*stored.Style)
	}
	yyQQPersonaState.Persona = persona
	yyQQPersonaState.Style = style
	yyQQPersonaState.DiskError = ""
	yyQQPersonaSaveLocked()
	yyQQPersonaState.Unlock()
	yyQQStyleInit(root)
}

// yyQQPersonaSaveLocked 原子落盘（先写 .tmp 再 rename），失败记 DiskError。
func yyQQPersonaSaveLocked() {
	if yyQQPersonaState.Path == "" {
		return
	}
	raw, e := json.Marshal(struct {
		Persona yyQQPersona   `json:"persona"`
		Style   yyStyleConfig `json:"style"`
	}{yyQQPersonaState.Persona, yyQQPersonaState.Style})
	if e == nil && len(raw) > 1<<20 {
		e = fmt.Errorf("persona_config_size_limit")
	}
	if e == nil {
		e = os.WriteFile(yyQQPersonaState.Path+".tmp", raw, 0600)
	}
	if e == nil {
		e = os.Rename(yyQQPersonaState.Path+".tmp", yyQQPersonaState.Path)
	}
	if e != nil {
		yyQQPersonaState.DiskError = "persona_persistence_failed"
	} else {
		yyQQPersonaState.DiskError = ""
	}
}

// yyQQStyleInit 加载群风格样本；损坏或超限时回退空档并记错误。
func yyQQStyleInit(root string) {
	yyQQStyleState.Lock()
	defer yyQQStyleState.Unlock()
	yyQQStyleState.Path = filepath.Join(root, "qq-style.json")
	yyQQStyleState.Groups = map[string][]yyStyleSample{}
	yyQQStyleState.DiskError = ""
	file, e := os.Open(yyQQStyleState.Path)
	if e != nil {
		return
	}
	defer file.Close()
	var groups map[string][]yyStyleSample
	if json.NewDecoder(io.LimitReader(file, 8<<20)).Decode(&groups) != nil || len(groups) > yyQQStyleMaxGroups {
		yyQQStyleState.DiskError = "invalid_or_oversized_style_index"
		return
	}
	for key, rows := range groups {
		if key == "" {
			delete(groups, key)
			continue
		}
		kept := make([]yyStyleSample, 0, yyQQStyleBufferMax)
		for _, r := range rows {
			r.Text = yyPersonaClip(strings.TrimSpace(r.Text), yyQQStyleSampleClip)
			if r.Text == "" || r.At.After(time.Now().Add(time.Hour)) {
				continue
			}
			kept = append(kept, r)
		}
		if len(kept) > yyQQStyleBufferMax {
			kept = kept[len(kept)-yyQQStyleBufferMax:]
		}
		if len(kept) == 0 {
			delete(groups, key)
			continue
		}
		groups[key] = kept
	}
	yyQQStyleState.Groups = groups
	yyQQStyleSaveLocked()
}

func yyQQStyleSaveLocked() {
	if yyQQStyleState.Path == "" {
		return
	}
	raw, e := json.Marshal(yyQQStyleState.Groups)
	if e == nil && len(raw) > 8<<20 {
		e = fmt.Errorf("style_index_size_limit")
	}
	if e == nil {
		e = os.WriteFile(yyQQStyleState.Path+".tmp", raw, 0600)
	}
	if e == nil {
		e = os.Rename(yyQQStyleState.Path+".tmp", yyQQStyleState.Path)
	}
	if e != nil {
		yyQQStyleState.DiskError = "style_persistence_failed"
	} else {
		yyQQStyleState.DiskError = ""
	}
}

// yyStyleConfigSnapshot 读当前风格配置（短锁，不与样本锁嵌套）。
func yyStyleConfigSnapshot() yyStyleConfig {
	yyQQPersonaState.Lock()
	defer yyQQPersonaState.Unlock()
	return yyQQPersonaState.Style
}

// yyPersonaPromptBlock 人格卡 + 反AI规则 + 群风格摘要，拼一段纯文本
// （无 markdown 围栏），总长 ≤800 字，超出截断并在末尾标注「（略 N 字）」；
// 人格关闭或整体为空时返回 ""。供主控注入 youyuan_qq_ai.go 的 system prompt。
func yyPersonaPromptBlock() string {
	yyQQPersonaState.Lock()
	p := yyQQPersonaState.Persona
	yyQQPersonaState.Unlock()
	if !p.Enabled {
		return ""
	}
	var b strings.Builder
	if p.Persona != "" {
		b.WriteString("【人格】你在群里叫「" + p.Name + "」，" + p.Persona + "\n")
	} else if p.Name != "" {
		b.WriteString("【人格】你在群里叫「" + p.Name + "」。\n")
	}
	if p.Style != "" {
		b.WriteString("【说话风格】" + p.Style + "\n")
	}
	if len(p.Taboos) > 0 {
		b.WriteString("【禁忌】" + strings.Join(p.Taboos, "、") + "\n")
	}
	if len(p.AntiAI) > 0 {
		rules := make([]string, 0, len(p.AntiAI))
		for i, rule := range p.AntiAI {
			rules = append(rules, strconv.Itoa(i+1)+"."+rule)
		}
		b.WriteString("【反AI腔】" + strings.Join(rules, "；") + "\n")
	}
	if summary := yyStyleSummary(yyQQGroupString(yyQQSnapshot().GroupID)); summary != "" {
		b.WriteString("【群风格】" + summary + "\n")
	}
	block := strings.TrimRight(b.String(), "\n")
	runes := []rune(block)
	if len(runes) <= yyQQPersonaBlockMax {
		return block
	}
	// 超长截断：末尾标注省略字数，总长仍 ≤800。
	marker := "（略 0 字）"
	for {
		keep := yyQQPersonaBlockMax - len([]rune(marker))
		if keep < 1 {
			keep = 1
		}
		next := "（略 " + strconv.Itoa(len(runes)-keep) + " 字）"
		if next == marker {
			break
		}
		marker = next
	}
	keep := yyQQPersonaBlockMax - len([]rune(marker))
	return string(runes[:keep]) + marker
}

// yyStyleObserve 群风格学习单入口：主控接在群消息读循环里喂数据。
// 自维护每群最近 200 条环形缓冲并即时落盘；风格学习整体关闭时直接忽略。
func yyStyleObserve(group, text string) {
	group = strings.TrimSpace(group)
	if group == "" {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if !yyStyleConfigSnapshot().Enabled {
		return
	}
	yyQQStyleState.Lock()
	defer yyQQStyleState.Unlock()
	if _, ok := yyQQStyleState.Groups[group]; !ok && len(yyQQStyleState.Groups) >= yyQQStyleMaxGroups {
		oldest, at := "", time.Now()
		for key, rows := range yyQQStyleState.Groups {
			last := time.Time{}
			if n := len(rows); n > 0 {
				last = rows[n-1].At
			}
			if oldest == "" || last.Before(at) {
				oldest, at = key, last
			}
		}
		delete(yyQQStyleState.Groups, oldest)
	}
	rows := append(yyQQStyleState.Groups[group], yyStyleSample{Text: yyPersonaClip(text, yyQQStyleSampleClip), At: time.Now()})
	if len(rows) > yyQQStyleBufferMax {
		rows = rows[len(rows)-yyQQStyleBufferMax:]
	}
	yyQQStyleState.Groups[group] = rows
	yyQQStyleSaveLocked()
}

// yyStyleLearn 计算一个群的风格统计：表情/图片频率、平均长度、
// top 高频词（2-4 字，停用词过滤）、活跃时段直方图。
func yyStyleLearn(group string) yyStyleStat {
	cfg := yyStyleConfigSnapshot()
	yyQQStyleState.Lock()
	rows := append([]yyStyleSample(nil), yyQQStyleState.Groups[group]...)
	yyQQStyleState.Unlock()
	stat := yyStyleStat{Group: group, Samples: len(rows)}
	if len(rows) == 0 {
		return stat
	}
	counts := map[string]int{}
	total, faces := 0, 0
	for _, row := range rows {
		total += len([]rune(row.Text))
		if yyStyleHasFace(row.Text) {
			faces++
		}
		stat.Hours[row.At.Hour()]++
		for _, w := range yyStyleWords(row.Text) {
			counts[w]++
		}
	}
	stat.AvgRunes = float64(total) / float64(len(rows))
	stat.FaceRatio = float64(faces) / float64(len(rows))
	stat.TopWords = yyStyleTopWords(counts, cfg.TopN)
	stat.PeakHours = yyStylePeakHours(stat.Hours)
	return stat
}

// yyStyleHasFace 消息是否带表情/图片类标记（每条计一次）。
func yyStyleHasFace(text string) bool {
	for _, mark := range []string{"[CQ:face", "[CQ:image", "[Face", "[表情]", "[图片]"} {
		if strings.Contains(text, mark) {
			return true
		}
	}
	return false
}

// yyStyleStop 停用字表：整词都由停用字组成的 n-gram 丢弃。
const yyStyleStop = "的了是我你他她它在啊吧呢吗哦嗯"

// yyStyleWords 提取 2-4 字中文滑窗词，每条消息内去重；
// 「了的」这类整词停用的丢弃，「绷不住了」这类含实义的保留。
func yyStyleWords(text string) []string {
	var cjk []rune
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			cjk = append(cjk, r)
		}
	}
	seen := map[string]bool{}
	var out []string
	for n := 2; n <= 4 && n <= len(cjk); n++ {
		for i := 0; i+n <= len(cjk); i++ {
			w := string(cjk[i : i+n])
			if seen[w] || yyStyleAllStop(w) {
				continue
			}
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func yyStyleAllStop(word string) bool {
	for _, r := range word {
		if !strings.ContainsRune(yyStyleStop, r) {
			return false
		}
	}
	return true
}

// yyStyleTopWords 按次数排序，次数相同取更长、再取字典序，保证输出确定。
func yyStyleTopWords(counts map[string]int, topN int) []yyStyleWord {
	if topN <= 0 {
		topN = 8
	}
	words := make([]yyStyleWord, 0, len(counts))
	for text, n := range counts {
		words = append(words, yyStyleWord{Text: text, N: n})
	}
	sort.SliceStable(words, func(i, j int) bool {
		if words[i].N != words[j].N {
			return words[i].N > words[j].N
		}
		li, lj := len([]rune(words[i].Text)), len([]rune(words[j].Text))
		if li != lj {
			return li > lj
		}
		return words[i].Text < words[j].Text
	})
	if len(words) > topN {
		words = words[:topN]
	}
	return words
}

// yyStylePeakHours 活跃时段：取峰值小时，向两侧扩展到不足峰值一半为止。
func yyStylePeakHours(hours [24]int) [2]int {
	peak, best := 0, -1
	for h, n := range hours {
		if n > best {
			peak, best = h, n
		}
	}
	if best <= 0 {
		return [2]int{0, 0}
	}
	start, end := peak, peak
	for start > 0 && hours[start-1]*2 >= best {
		start--
	}
	for end < 23 && hours[end+1]*2 >= best {
		end++
	}
	return [2]int{start, end}
}

// yyStyleSummary 返回 ≤120 字中文摘要，如「本群习惯：消息偏短、表情多、20-23 点最活跃」。
// 风格学习关闭或样本不足时返回 ""。
func yyStyleSummary(group string) string {
	cfg := yyStyleConfigSnapshot()
	if !cfg.Enabled {
		return ""
	}
	stat := yyStyleLearn(group)
	if stat.Samples < cfg.MinMsgs {
		return ""
	}
	var b strings.Builder
	b.WriteString("本群习惯：")
	b.WriteString(yyStyleLenWord(stat.AvgRunes))
	b.WriteString("、")
	b.WriteString(yyStyleFaceWord(stat.FaceRatio))
	b.WriteString("、")
	if stat.PeakHours[0] == stat.PeakHours[1] {
		b.WriteString(strconv.Itoa(stat.PeakHours[0]) + "点最活跃")
	} else {
		b.WriteString(strconv.Itoa(stat.PeakHours[0]) + "-" + strconv.Itoa(stat.PeakHours[1]) + "点最活跃")
	}
	if len(stat.TopWords) > 0 {
		b.WriteString("、高频词「" + stat.TopWords[0].Text + "」")
	}
	return yyPersonaClip(b.String(), yyQQPersonaSummary)
}

func yyStyleLenWord(avg float64) string {
	switch {
	case avg <= 15: // 设计 §3.4：不到 15 字算短句
		return "消息偏短"
	case avg >= 40:
		return "消息偏长"
	default:
		return "消息长短适中"
	}
}

func yyStyleFaceWord(ratio float64) string {
	switch {
	case ratio >= 0.5:
		return "表情多"
	case ratio >= 0.15:
		return "偶尔带表情"
	default:
		return "很少发表情"
	}
}

// yyQQPersonaRoutes 人格与群风格的读写接口，风格与 yyQQTriggerRoutes 一致：
// 方法不对 405，参数不合法 400，一律 JSON。
func yyQQPersonaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/persona", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			yyQQPersonaState.Lock()
			persona, style := yyQQPersonaState.Persona, yyQQPersonaState.Style
			yyQQPersonaState.Unlock()
			yyQQStyleState.Lock()
			keys := make([]string, 0, len(yyQQStyleState.Groups))
			for key := range yyQQStyleState.Groups {
				keys = append(keys, key)
			}
			yyQQStyleState.Unlock()
			sort.Strings(keys)
			groups := map[string]string{}
			for _, key := range keys {
				if summary := yyStyleSummary(key); summary != "" {
					groups[key] = summary
				}
			}
			writeJSON(w, map[string]any{"ok": true, "persona": persona, "style": style, "groups": groups})
		case http.MethodPost:
			var payload struct {
				Persona *yyQQPersona   `json:"persona"`
				Style   *yyStyleConfig `json:"style"`
			}
			if e := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); e != nil {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, map[string]any{"ok": false, "error": "请求不是合法JSON"})
				return
			}
			if payload.Persona == nil && payload.Style == nil {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, map[string]any{"ok": false, "error": "没有要保存的内容"})
				return
			}
			var nextPersona yyQQPersona
			var nextStyle yyStyleConfig
			havePersona, haveStyle := false, false
			if payload.Persona != nil {
				p := yyQQPersonaNormalize(*payload.Persona)
				if p.Name == "" {
					w.WriteHeader(http.StatusBadRequest)
					writeJSON(w, map[string]any{"ok": false, "error": "人格名称不能为空"})
					return
				}
				if len(p.AntiAI) == 0 {
					w.WriteHeader(http.StatusBadRequest)
					writeJSON(w, map[string]any{"ok": false, "error": "反AI腔规则至少保留1条"})
					return
				}
				nextPersona, havePersona = p, true
			}
			if payload.Style != nil {
				nextStyle, haveStyle = yyStyleConfigNormalize(*payload.Style), true
			}
			yyQQPersonaState.Lock()
			if havePersona {
				yyQQPersonaState.Persona = nextPersona
			} else {
				nextPersona = yyQQPersonaState.Persona
			}
			if haveStyle {
				yyQQPersonaState.Style = nextStyle
			} else {
				nextStyle = yyQQPersonaState.Style
			}
			yyQQPersonaSaveLocked()
			diskError := yyQQPersonaState.DiskError
			yyQQPersonaState.Unlock()
			if diskError != "" {
				writeJSON(w, map[string]any{"ok": false, "error": diskError})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "persona": nextPersona, "style": nextStyle})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
		}
	})
}
