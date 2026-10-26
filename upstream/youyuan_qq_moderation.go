package main

// 群规（moderation）· 硬通道：违禁词拦截。
//
// 用户原话：「一些对接接口，比如一些违禁词啊什么的，或者我不想要的一些词
// 和我叫AI记住不要干什么 不准玩家干什么，然后玩家在群里面做了 那我可以在插件层
// 禁止 也可以直接跟AI说话，让他以后都记住，然后禁止掉这种行为」
//
// 本文件负责「插件层硬禁止」：命中规则的群消息在进入 RAG / 游戏转发 / AI 之前
// 就被拦下，处置分四档：
//   warn      不拦消息，只把禁令写进 AI 提示词（软劝阻，见 youyuan_qq_moderation_nl.go）
//   block     直接丢弃该消息，可选引用回执
//   recall    block + 撤回该消息（OneBot delete_msg，机器人须是管理员）
//   recall_ban block + 撤回 + 禁言 10 分钟（set_group_ban）
// 撤回/禁言走 yyQQRunAction（限流+留痕），能力开关默认关闭——没开就自动降级成
// block，消息照样拦，只是不做撤回/禁言。软通道（AI 长期禁令）在
// youyuan_qq_moderation_nl.go，双通道共用本文件的规则存储。

import (
	"encoding/json"
	"fmt"
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

const (
	yyQQModMaxRules   = 100 // 规则总数上限
	yyQQModMaxPattern = 60  // 单条匹配内容上限（字）
	yyQQModMaxReason  = 60  // 单条理由上限（字）
	yyQQModBanMinutes = 10  // recall_ban 的禁言时长（分钟）
	yyQQModNotifyGap  = 30  // 同一人被拦后的回执冷却（秒），防刷屏
)

// yyQQModerationRule 一条群规。Kind/Action 的合法值见 Normalize。
type yyQQModerationRule struct {
	Pattern string    `json:"pattern"` // 匹配内容：子串或正则源码（均已做小写/半角/去零宽规范化）
	Kind    string    `json:"kind"`    // substring | regex
	Action  string    `json:"action"`  // warn | block | recall | recall_ban
	Reason  string    `json:"reason"`  // 给被拦者看的理由
	Enabled bool      `json:"enabled"` // 关掉的规则不参与匹配，但保留配置
	Source  string    `json:"source"`  // manual=面板配置 | natural=自然语言禁令生成
	AddedAt time.Time `json:"added_at"`
}

// yyQQModerationState 规则持久化状态，范式照 youyuan_qq_persona.go：
// 启动加载、变更即存、JSON 损坏回退空规则并记日志。
type yyQQModerationState struct {
	sync.Mutex
	Path      string
	Rules     []yyQQModerationRule
	DiskError string
	reCache   map[string]*regexp.Regexp
	notifyAt  map[string]time.Time // 回执冷却：userID -> 上次回执时间
}

var yyQQModeration = yyQQModerationState{reCache: map[string]*regexp.Regexp{}, notifyAt: map[string]time.Time{}}

// yyQQModerationInit 启动加载 qq-moderation.json；缺失用空规则，损坏回退空规则。
func yyQQModerationInit(root string) {
	yyQQModeration.Lock()
	defer yyQQModeration.Unlock()
	yyQQModeration.Path = filepath.Join(root, "qq-moderation.json")
	yyQQModeration.Rules = nil
	yyQQModeration.DiskError = ""
	file, e := os.Open(yyQQModeration.Path)
	if e != nil {
		return
	}
	defer file.Close()
	var stored struct {
		Rules []yyQQModerationRule `json:"rules"`
	}
	if json.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&stored) != nil {
		yyQQModeration.DiskError = "invalid_moderation_config"
		yyQQLog("群规配置损坏，已按空规则运行（原文件保留为 .broken）")
		if raw, e2 := os.ReadFile(yyQQModeration.Path); e2 == nil {
			_ = os.WriteFile(yyQQModeration.Path+".broken", raw, 0600)
		}
		return
	}
	yyQQModeration.Rules = yyQQModerationNormalizeLocked(stored.Rules)
}

// yyQQModerationSaveLocked 原子落盘（先写 .tmp 再 rename），失败记 DiskError。
func yyQQModerationSaveLocked() {
	if yyQQModeration.Path == "" {
		return
	}
	raw, e := json.Marshal(struct {
		Rules []yyQQModerationRule `json:"rules"`
	}{yyQQModeration.Rules})
	if e == nil && len(raw) > 1<<20 {
		e = fmt.Errorf("moderation_config_size_limit")
	}
	if e == nil {
		e = os.WriteFile(yyQQModeration.Path+".tmp", raw, 0600)
	}
	if e == nil {
		e = os.Rename(yyQQModeration.Path+".tmp", yyQQModeration.Path)
	}
	if e != nil {
		yyQQModeration.DiskError = "moderation_persistence_failed"
	} else {
		yyQQModeration.DiskError = ""
	}
}

// yyQQModerationNormalizeLocked 清洗规则列表：去重、校验、补默认值。
// 调用方必须已持有 yyQQModeration 锁。已存在同 pattern+kind 的规则保留
// AddedAt 与 Source（面板整表回传时自然语言的来源标记不丢）。
func yyQQModerationNormalizeLocked(in []yyQQModerationRule) []yyQQModerationRule {
	existing := map[string]yyQQModerationRule{}
	for _, r := range yyQQModeration.Rules {
		existing[r.Pattern+"\x00"+r.Kind] = r
	}
	out := make([]yyQQModerationRule, 0, len(in))
	seen := map[string]bool{}
	for _, r := range in {
		kind := strings.ToLower(strings.TrimSpace(r.Kind))
		if kind != "substring" && kind != "regex" {
			kind = "substring"
		}
		action := strings.ToLower(strings.TrimSpace(r.Action))
		switch action {
		case "warn", "block", "recall", "recall_ban":
		default:
			action = "block"
		}
		pattern := yyQQModerationNormText(r.Pattern)
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || len([]rune(pattern)) > yyQQModMaxPattern {
			continue
		}
		if kind == "regex" {
			if _, e := regexp.Compile(pattern); e != nil {
				continue // 编译不过的正则直接丢弃，不进缓存
			}
		}
		key := pattern + "\x00" + kind
		if seen[key] {
			continue
		}
		seen[key] = true
		rule := yyQQModerationRule{
			Pattern: pattern,
			Kind:    kind,
			Action:  action,
			Reason:  yyPersonaClip(strings.TrimSpace(r.Reason), yyQQModMaxReason),
			Enabled: r.Enabled,
			AddedAt: r.AddedAt,
			Source:  strings.TrimSpace(r.Source),
		}
		if rule.AddedAt.IsZero() {
			rule.AddedAt = time.Now()
		}
		if rule.Source != "natural" {
			rule.Source = "manual"
		}
		if old, ok := existing[key]; ok {
			rule.AddedAt = old.AddedAt
			rule.Source = old.Source
		}
		out = append(out, rule)
		if len(out) >= yyQQModMaxRules {
			break
		}
	}
	return out
}

// yyQQModerationNormText 匹配侧规范化：全角转半角、去零宽字符、小写、
// 连续空白压成一个空格。规则与消息都过它，保证「ＳＢ」和「sb」命中同一条。
func yyQQModerationNormText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0xff01 && r <= 0xff5e {
			r -= 0xfee0 // 全角 ASCII → 半角（转完继续走下面的小写化，Ｓ→s）
		}
		switch {
		case r == 0x200b || r == 0x200c || r == 0x200d || r == 0xfeff:
			continue // 零宽字符：有人用它们插字绕过检测
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		default:
			b.WriteRune(r)
		}
	}
	// 连续空白（含全角空格已转半角）压成单个空格
	parts := strings.Fields(b.String())
	return strings.Join(parts, " ")
}

// yyQQModerationMatchLocked 规则是否命中已规范化的消息文本。调用方持锁。
func yyQQModerationMatchLocked(rule yyQQModerationRule, norm string) bool {
	if rule.Kind == "regex" {
		re, ok := yyQQModeration.reCache[rule.Pattern]
		if !ok {
			compiled, e := regexp.Compile(rule.Pattern)
			if e != nil {
				return false
			}
			yyQQModeration.reCache[rule.Pattern] = compiled
			re = compiled
		}
		return re.MatchString(norm)
	}
	return strings.Contains(norm, rule.Pattern)
}

// yyQQModerationBlockHit 群消息是否命中硬拦截规则（warn 级不拦，见 prompt 块）。
// 管理员与超管不拦——群规是管群友的，管事的人自己发的消息不触发。
func yyQQModerationBlockHit(cf yyQQConfig, text, userID string) (yyQQModerationRule, bool) {
	if yyQQRole(cf, userID) >= 1 {
		return yyQQModerationRule{}, false
	}
	norm := yyQQModerationNormText(text)
	if norm == "" {
		return yyQQModerationRule{}, false
	}
	yyQQModeration.Lock()
	defer yyQQModeration.Unlock()
	for _, rule := range yyQQModeration.Rules {
		if !rule.Enabled || rule.Action == "warn" {
			continue
		}
		if yyQQModerationMatchLocked(rule, norm) {
			return rule, true
		}
	}
	return yyQQModerationRule{}, false
}

// yyQQModerationCanRecall recall/recall_ban 处置是否真的执行撤回。
// 官方适配器或能力开关没开 → false，此时 enforce 自动降级成只拦不撤。
func yyQQModerationCanRecall(cf yyQQConfig, action string) bool {
	if action != "recall" && action != "recall_ban" {
		return false
	}
	return cf.Adapter == "onebot" && yyQQCapEnabled(cf.Assistant, "recall")
}

// yyQQModerationCanBan recall_ban 处置是否真的执行禁言。
func yyQQModerationCanBan(cf yyQQConfig, action string) bool {
	if action != "recall_ban" {
		return false
	}
	return cf.Adapter == "onebot" && yyQQCapEnabled(cf.Assistant, "ban")
}

// yyQQModerationEnforce 执行拦截处置。机器人不是管理员时撤回/禁言会失败，
// 只记日志，不影响拦截本身。
func yyQQModerationEnforce(cf yyQQConfig, rule yyQQModerationRule, userID, name, text, id string) {
	reason := rule.Reason
	if reason == "" {
		reason = "违反群规"
	}
	group := yyQQGroupString(cf.GroupID)
	clip := yyPersonaClip(strings.TrimSpace(text), 40)
	yyQQLog("群规拦截[" + rule.Action + "] " + name + "(" + userID + ")：" + clip + " 原因：" + reason)
	if yyQQModerationCanRecall(cf, rule.Action) && id != "" {
		if e := yyQQRunAction("recall", "系统·群规", "群规", userID,
			"撤回 "+name+" 的消息（"+reason+"）", 2, func() (string, error) {
				return "", yyQQActionRecall(group, id)
			}); e != nil {
			yyQQLog("群规撤回失败：" + e.Error())
		}
	}
	if yyQQModerationCanBan(cf, rule.Action) {
		if e := yyQQRunAction("ban", "系统·群规", "群规", userID,
			"禁言 "+name+" "+strconv.Itoa(yyQQModBanMinutes)+" 分钟（"+reason+"）", 2, func() (string, error) {
				return "", yyQQActionBan(group, userID, yyQQModBanMinutes)
			}); e != nil {
			yyQQLog("群规禁言失败：" + e.Error())
		}
	}
	yyQQModerationNotify(cf, rule, userID, reason, id)
}

// yyQQModerationNotify 给被拦者回执（引用原消息）。同一人 30 秒内只回一次，
// 否则有人连发违禁词就是给机器人刷屏的机会。
func yyQQModerationNotify(cf yyQQConfig, rule yyQQModerationRule, userID, reason, id string) {
	if id == "" {
		return
	}
	yyQQModeration.Lock()
	last, ok := yyQQModeration.notifyAt[userID]
	if ok && time.Since(last) < yyQQModNotifyGap*time.Second {
		yyQQModeration.Unlock()
		return
	}
	yyQQModeration.notifyAt[userID] = time.Now()
	if len(yyQQModeration.notifyAt) > 512 {
		for k, at := range yyQQModeration.notifyAt {
			if time.Since(at) > 10*time.Minute {
				delete(yyQQModeration.notifyAt, k)
			}
		}
	}
	yyQQModeration.Unlock()
	msg := "你的消息涉及「" + reason + "」，已被群规拦下。"
	if rule.Action == "recall" || rule.Action == "recall_ban" {
		msg += "消息已撤回，请遵守群规。"
	}
	yyQQFeedbackTo(cf, msg, id)
}

// yyQQModerationSnapshot 供路由与测试读取当前规则（拷贝，避免持锁外泄）。
func yyQQModerationSnapshot() ([]yyQQModerationRule, string) {
	yyQQModeration.Lock()
	defer yyQQModeration.Unlock()
	return append([]yyQQModerationRule(nil), yyQQModeration.Rules...), yyQQModeration.DiskError
}

// yyQQModerationReplace 整表替换（面板保存用），返回规范化后的规则。
func yyQQModerationReplace(in []yyQQModerationRule) ([]yyQQModerationRule, error) {
	yyQQModeration.Lock()
	defer yyQQModeration.Unlock()
	yyQQModeration.Rules = yyQQModerationNormalizeLocked(in)
	yyQQModerationSaveLocked()
	return append([]yyQQModerationRule(nil), yyQQModeration.Rules...), yyQQModeration.diskErrLocked()
}

func (s *yyQQModerationState) diskErrLocked() error {
	if s.DiskError != "" {
		return fmt.Errorf("配置保存失败：%s", s.DiskError)
	}
	return nil
}

// yyQQModerationRemove 删除一条（按 pattern+kind）。
func yyQQModerationRemove(pattern, kind string) bool {
	pattern = yyQQModerationNormText(pattern)
	kind = strings.ToLower(strings.TrimSpace(kind))
	yyQQModeration.Lock()
	defer yyQQModeration.Unlock()
	out := make([]yyQQModerationRule, 0, len(yyQQModeration.Rules))
	removed := false
	for _, r := range yyQQModeration.Rules {
		if r.Pattern == pattern && r.Kind == kind {
			removed = true
			continue
		}
		out = append(out, r)
	}
	if !removed {
		return false
	}
	yyQQModeration.Rules = out
	yyQQModerationSaveLocked()
	return true
}

// yyQQModerationAddNatural 自然语言禁令落地：生成一条 substring 规则（硬通道）。
// 返回规则与是否新增（已存在同 pattern 时只确保 enabled）。
func yyQQModerationAddNatural(pattern, reason string) (yyQQModerationRule, bool) {
	pattern = yyQQModerationNormText(pattern)
	if pattern == "" {
		return yyQQModerationRule{}, false
	}
	yyQQModeration.Lock()
	defer yyQQModeration.Unlock()
	for i, r := range yyQQModeration.Rules {
		if r.Pattern == pattern && r.Kind == "substring" {
			if !r.Enabled {
				yyQQModeration.Rules[i].Enabled = true
				yyQQModerationSaveLocked()
				return yyQQModeration.Rules[i], false
			}
			return r, false
		}
	}
	rule := yyQQModerationRule{
		Pattern: pattern, Kind: "substring", Action: "block",
		Reason: reason, Enabled: true, Source: "natural", AddedAt: time.Now(),
	}
	yyQQModeration.Rules = append(yyQQModeration.Rules, rule)
	yyQQModerationSaveLocked()
	return rule, true
}

// yyQQModerationRoutes 面板接口：GET 列表（含 AI 长期禁令投影）、POST 整表保存、
// DELETE ?pattern=&kind= 删一条。
func yyQQModerationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/moderation", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rules, diskErr := yyQQModerationSnapshot()
			if rules == nil {
				rules = []yyQQModerationRule{}
			}
			writeJSON(w, map[string]any{
				"ok": true, "rules": rules, "taboos": yyQQPersonaTaboos(), "disk_error": diskErr,
				"note": "命中即拦：消息不会进 AI、不会转发进游戏。撤回/禁言要先在「机器人能力」里打开对应开关；" +
					"没开时自动降级为只拦不撤。超管在群里直接说「以后不准 玩家XXX」也能添加。",
			})
		case http.MethodPost:
			// rules / taboos 都用指针：只有真的出现在请求体里才替换，
			// 否则「只保存长期禁令」会把整张群规规则表清空（以前就是这个坑）。
			// enabled 同样用指针：缺省=启用。否则手写 JSON 漏了 enabled 会存出
			// 一条永远不生效的规则，用户只会觉得「加了但没用」。
			var p struct {
				Rules *[]struct {
					Pattern string `json:"pattern"`
					Kind    string `json:"kind"`
					Action  string `json:"action"`
					Reason  string `json:"reason"`
					Enabled *bool  `json:"enabled"`
				} `json:"rules"`
				Taboos *[]string `json:"taboos"`
			}
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&p); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
				return
			}
			if p.Rules == nil && p.Taboos == nil {
				writeJSON(w, map[string]any{"ok": false, "error": "没有要保存的内容"})
				return
			}
			rules, diskErr := yyQQModerationSnapshot()
			if p.Rules != nil {
				if len(*p.Rules) > yyQQModMaxRules {
					writeJSON(w, map[string]any{"ok": false, "error": "规则太多（上限 100 条）"})
					return
				}
				in := make([]yyQQModerationRule, 0, len(*p.Rules))
				for _, r := range *p.Rules {
					rule := yyQQModerationRule{Pattern: r.Pattern, Kind: r.Kind, Action: r.Action, Reason: r.Reason}
					rule.Enabled = r.Enabled == nil || *r.Enabled
					in = append(in, rule)
				}
				var e error
				rules, e = yyQQModerationReplace(in)
				if e != nil {
					writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
					return
				}
			}
			taboos := yyQQPersonaTaboos()
			if p.Taboos != nil {
				taboos = yyQQPersonaSetTaboos(*p.Taboos)
			}
			if rules == nil {
				rules = []yyQQModerationRule{}
			}
			if p.Rules != nil {
				rules, diskErr = yyQQModerationSnapshot() // 落盘结果以保存后为准
			}
			writeJSON(w, map[string]any{"ok": true, "rules": rules, "taboos": taboos, "disk_error": diskErr})
		case http.MethodDelete:
			pattern := r.URL.Query().Get("pattern")
			kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
			if pattern == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "缺少 pattern"})
				return
			}
			// 长期禁令（taboos）存在人格配置里，不在 Rules 表里。
			// 以前这里只查 Rules，于是 kind=taboo 永远 ok:false——HTTP 还是 200，
			// 前端看起来「点了删除没反应」。这里单独走人格表。
			if kind == "taboo" {
				want := yyQQModerationNormText(pattern)
				cur := yyQQPersonaTaboos()
				next := make([]string, 0, len(cur))
				removed := false
				for _, t := range cur {
					if yyQQModerationNormText(t) == want {
						removed = true
						continue
					}
					next = append(next, t)
				}
				if removed {
					yyQQPersonaSetTaboos(next)
				}
				writeJSON(w, map[string]any{"ok": removed, "taboos": yyQQPersonaTaboos()})
				return
			}
			if kind == "" {
				kind = "substring"
			}
			writeJSON(w, map[string]any{"ok": yyQQModerationRemove(pattern, kind)})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
