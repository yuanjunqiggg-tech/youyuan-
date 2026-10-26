package main

// 群成员画像（认人）+ 机器人表情自我认知索引。
//
// 设计目标（用户核心诉求）：机器人分得清谁是谁、能 @ 玩家、看得到自己的表情包。
//   - 成员画像按群持久化到 dataDir/qq-members.json（范式照抄 youyuan_qq_memory.go：
//     互斥锁 + map + 临时文件 + rename 原子落盘），从入站消息 yyQQInbound 学习
//     昵称、群名片、角色、活跃度与「叫我X」式别名；
//   - 表情索引把机器人自己的表情包变成模型可见的编号列表，经可注入调用层
//     （默认 yyQQOneBotCall，测试用假函数替换）拉取，缓存 dataDir/qq-emoji.json；
//     探测不到可用 action 时如实降级为手动维护模式，不假装调通；
//   - 路由只定义不注册：由主智能体在 yyRegisterQQ 里加一行
//     yyMemberRoutes(mux, dataDir)（与 yyQQImageRoutes / yyQQRagRoutes 同风格）。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ---- 成员画像（认人） ----

const (
	yyMemberAliasMax    = 10  // 每人别名上限
	yyMemberGroupMax    = 500 // 单群成员上限，超出按最久未发言淘汰
	yyMemberListDefault = 50  // 名录默认列出人数
	yyMemberBlockChars  = 600 // 群成员名录 prompt 块字数上限
)

// yyQQMember 一个群成员的画像。Aliases 最新学习的排最前（显示名取首个）。
type yyQQMember struct {
	Group     string   `json:"group"`
	QQ        int64    `json:"qq"`
	Name      string   `json:"name"`
	Aliases   []string `json:"aliases,omitempty"`
	Role      string   `json:"role,omitempty"`
	MsgCount  int      `json:"msg_count"`
	FirstSeen int64    `json:"first_seen"`
	LastSeen  int64    `json:"last_seen"`
	Notes     string   `json:"notes,omitempty"`
}

var yyMembers = struct {
	sync.Mutex
	Path      string
	Groups    map[string][]yyQQMember
	DiskError string
}{Groups: map[string][]yyQQMember{}}

// yyMemberInit 载入（或新建）dataDir/qq-members.json。文件缺失当空库；
// 损坏或超限时如实记 DiskError 并当空库用，不阻塞聊天。
func yyMemberInit(root string) {
	yyMembers.Lock()
	defer yyMembers.Unlock()
	yyMembers.Path = filepath.Join(root, "qq-members.json")
	yyMembers.Groups = map[string][]yyQQMember{}
	yyMembers.DiskError = ""
	file, e := os.Open(yyMembers.Path)
	if e != nil {
		return
	}
	defer file.Close()
	var groups map[string][]yyQQMember
	if json.NewDecoder(io.LimitReader(file, 8<<20)).Decode(&groups) != nil || len(groups) > 64 {
		yyMembers.DiskError = "invalid_or_oversized_member_index"
		return
	}
	clean := map[string][]yyQQMember{}
	for group, rows := range groups {
		group = strings.TrimSpace(group)
		if group == "" || len(rows) > yyMemberGroupMax {
			continue
		}
		kept := make([]yyQQMember, 0, len(rows))
		seen := map[int64]bool{}
		for _, row := range rows {
			if row.QQ <= 0 || seen[row.QQ] {
				continue
			}
			seen[row.QQ] = true
			row.Group = group
			row.Name = yyQQClip(strings.TrimSpace(row.Name), 64)
			row.Role = yyQQClip(strings.TrimSpace(row.Role), 16)
			row.Aliases = yyMemberCleanAliases(row.Aliases)
			if row.MsgCount < 0 {
				row.MsgCount = 0
			}
			kept = append(kept, row)
		}
		clean[group] = kept
	}
	yyMembers.Groups = clean
}

// yyMemberEnsure 惰性初始化：接线遗漏时也能从 qq-bridge.json 所在目录恢复，
// 保证「认人」不因漏一行注册而静默失效。
func yyMemberEnsure() {
	yyMembers.Lock()
	ready := yyMembers.Path != ""
	yyMembers.Unlock()
	if ready || yyQQPath == "" {
		return
	}
	yyMemberInit(filepath.Dir(yyQQPath))
}

// yyMemberSaveLocked 原子落盘（调用方持锁）。范式同 youyuan_qq_memory.go。
func yyMemberSaveLocked() {
	if yyMembers.Path == "" {
		return
	}
	raw, e := json.Marshal(yyMembers.Groups)
	if e == nil && len(raw) > 8<<20 {
		e = fmt.Errorf("member_index_size_limit")
	}
	if e == nil {
		e = os.WriteFile(yyMembers.Path+".tmp", raw, 0600)
	}
	if e == nil {
		e = os.Rename(yyMembers.Path+".tmp", yyMembers.Path)
	}
	if e != nil {
		yyMembers.DiskError = "member_persistence_failed"
	} else {
		yyMembers.DiskError = ""
	}
}

// yyMemberOnMessage 成员画像 upsert：每条群消息更新 Name/Role/MsgCount/LastSeen，
// 并从文本学习「叫我X」式别名。机器人自身发言与私聊（无群号）不进画像。
func yyMemberOnMessage(m *yyQQInbound) {
	if m == nil || m.Group == "" {
		return
	}
	qq, e := strconv.ParseInt(strings.TrimSpace(m.User), 10, 64)
	if e != nil || qq <= 0 {
		return
	}
	cf := yyQQSnapshot()
	if m.User == yyQQBotID(cf) {
		return // 机器人自己的发言：既不计入统计，也不学别名
	}
	yyMemberEnsure()
	yyMembers.Lock()
	defer yyMembers.Unlock()
	rows := yyMembers.Groups[m.Group]
	idx := -1
	for i := range rows {
		if rows[i].QQ == qq {
			idx = i
			break
		}
	}
	now := time.Now().Unix()
	if idx < 0 {
		if len(rows) >= yyMemberGroupMax {
			oldest := 0
			for i := 1; i < len(rows); i++ {
				if rows[i].LastSeen < rows[oldest].LastSeen {
					oldest = i
				}
			}
			rows = append(rows[:oldest], rows[oldest+1:]...)
		}
		rows = append(rows, yyQQMember{Group: m.Group, QQ: qq, FirstSeen: now})
		idx = len(rows) - 1
	}
	if rows[idx].FirstSeen == 0 {
		rows[idx].FirstSeen = now
	}
	// Name 取 m.Name（解析时已从 sender 的 card/nickname 等择优）；为空时
	// 再直接看 Raw.sender 兜底，按 inbound.go 的实际字段来。
	name := strings.TrimSpace(m.Name)
	if name == "" {
		if sender, ok := m.Raw["sender"].(map[string]any); ok {
			name = strings.TrimSpace(yyQQSenderName(sender))
		}
	}
	if name != "" {
		rows[idx].Name = yyQQClip(name, 64)
	}
	if role := strings.TrimSpace(m.Role); role != "" {
		rows[idx].Role = yyQQClip(role, 16)
	}
	rows[idx].MsgCount++
	rows[idx].LastSeen = now
	if alias, ok := yyMemberLearnAlias(yyMemberPlainText(*m)); ok {
		yyMemberAddAliasLocked(&rows[idx], alias)
	}
	yyMembers.Groups[m.Group] = rows
	yyMemberSaveLocked()
}

// yyMemberPlainText 只取 text 段拼成纯文本（at/image 等段整体剔除），
// 避免 CQ 残留污染别名学习；没有段信息时退回 m.Text。
func yyMemberPlainText(m yyQQInbound) string {
	if len(m.Segments) == 0 {
		return m.Text
	}
	var b strings.Builder
	for _, seg := range m.Segments {
		if seg.Type == "text" {
			b.WriteString(seg.Data["text"])
		}
	}
	return b.String()
}

// yyMemberAliasRe 别名学习规则：「叫我X / 我是X / 我的名字是X」，
// X 取 1-12 个非空白、非中英句逗叹问字符。
var yyMemberAliasRe = regexp.MustCompile(`(?:叫我|我是|我的名字是)\s*([^\s，。！？!?]{1,12})`)

func yyMemberLearnAlias(text string) (string, bool) {
	hit := yyMemberAliasRe.FindStringSubmatch(text)
	if hit == nil {
		return "", false
	}
	alias := strings.Trim(hit[1], " \t,.;:!?~～、，。！？")
	if alias == "" || len([]rune(alias)) > 12 {
		return "", false
	}
	// 防御 CQ 残留与 at 占位混进别名。
	if strings.ContainsAny(alias, "[]@") {
		return "", false
	}
	return alias, true
}

// yyMemberAddAliasLocked 别名去重后置顶（最新自我称呼优先），超上限丢最旧。
func yyMemberAddAliasLocked(row *yyQQMember, alias string) {
	kept := make([]string, 0, len(row.Aliases)+1)
	kept = append(kept, alias)
	for _, a := range row.Aliases {
		if a == alias {
			continue
		}
		kept = append(kept, a)
	}
	if len(kept) > yyMemberAliasMax {
		kept = kept[:yyMemberAliasMax]
	}
	row.Aliases = kept
}

// yyMemberCleanAliases 载入时清洗别名：去空白、去重、限长、剔非法字符。
func yyMemberCleanAliases(aliases []string) []string {
	if len(aliases) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(aliases))
	for _, a := range aliases {
		a = strings.TrimSpace(a)
		if a == "" || seen[a] || len([]rune(a)) > 12 || strings.ContainsAny(a, "[]@") {
			continue
		}
		seen[a] = true
		out = append(out, a)
		if len(out) >= yyMemberAliasMax {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// yyMemberResolve 显示名：别名优先，否则群名片，否则 "QQ:<qq>"。
func yyMemberResolve(group string, qq int64) string {
	if qq <= 0 {
		return ""
	}
	yyMemberEnsure()
	yyMembers.Lock()
	defer yyMembers.Unlock()
	for _, row := range yyMembers.Groups[group] {
		if row.QQ != qq {
			continue
		}
		if len(row.Aliases) > 0 {
			return row.Aliases[0]
		}
		if row.Name != "" {
			return row.Name
		}
		break
	}
	return "QQ:" + strconv.FormatInt(qq, 10)
}

// yyMemberFind 按名字/别名找 QQ 号（「@玩家」解析用）：精确优先、前缀次之，
// 均大小写不敏感；同名时别名先于群名片参与匹配。
func yyMemberFind(group, name string) (int64, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false
	}
	yyMemberEnsure()
	yyMembers.Lock()
	defer yyMembers.Unlock()
	rows := yyMembers.Groups[group]
	lower := strings.ToLower(name)
	for _, row := range rows {
		for _, a := range row.Aliases {
			if strings.ToLower(a) == lower {
				return row.QQ, true
			}
		}
	}
	for _, row := range rows {
		if row.Name != "" && strings.ToLower(row.Name) == lower {
			return row.QQ, true
		}
	}
	for _, row := range rows {
		for _, a := range row.Aliases {
			if strings.HasPrefix(strings.ToLower(a), lower) {
				return row.QQ, true
			}
		}
	}
	for _, row := range rows {
		if row.Name != "" && strings.HasPrefix(strings.ToLower(row.Name), lower) {
			return row.QQ, true
		}
	}
	return 0, false
}

// yyMemberDisplayName 单个成员的显示名（不含 QQ 号）：别名优先，否则群名片，
// 否则 fallbackName，最后 "QQ:<qq>"。
func yyMemberDisplayName(group string, qq int64, fallbackName string) string {
	if qq <= 0 {
		return fallbackName
	}
	yyMemberEnsure()
	yyMembers.Lock()
	defer yyMembers.Unlock()
	for _, row := range yyMembers.Groups[group] {
		if row.QQ != qq {
			continue
		}
		if len(row.Aliases) > 0 {
			return row.Aliases[0]
		}
		if row.Name != "" {
			return row.Name
		}
		break
	}
	if fallbackName != "" {
		return fallbackName
	}
	return "QQ:" + strconv.FormatInt(qq, 10)
}

// yyMemberMentionText 入站消息的一行「谁在说话、@ 了谁」：
// 无 @ 时 "张三(10001)"；被 @ 时 @ 目标按 Mentions/Mentioned 实际结构作前缀，
// 形如 "@李四(10002) 张三(10001)"（@全体成员 原样输出）。
func yyMemberMentionText(m *yyQQInbound) string {
	if m == nil {
		return ""
	}
	speaker := yyMemberSpeakerLabel(m)
	prefix := yyMemberMentionPrefix(m)
	if prefix == "" {
		return speaker
	}
	return prefix + " " + speaker
}

// yyMemberSpeakerLabel 说话人标签："昵称(QQ号)"，认不出时退回入站昵称。
func yyMemberSpeakerLabel(m *yyQQInbound) string {
	qq, e := strconv.ParseInt(strings.TrimSpace(m.User), 10, 64)
	if e != nil || qq <= 0 {
		return strings.TrimSpace(m.Name)
	}
	name := yyMemberDisplayName(m.Group, qq, strings.TrimSpace(m.Name))
	return name + "(" + strconv.FormatInt(qq, 10) + ")"
}

// yyMemberMentionPrefix @ 目标前缀：按 Mentions 逐个解析（机器人自身渲染为
// @机器人，qq=all 渲染为 @全体成员）；Mentioned 为真而 Mentions 为空时
// （个别适配器只给 Mentioned）防御性补一个 @机器人。
func yyMemberMentionPrefix(m *yyQQInbound) string {
	if len(m.Mentions) == 0 {
		if m.Mentioned {
			return "@机器人"
		}
		return ""
	}
	botID := yyQQBotID(yyQQSnapshot())
	parts := make([]string, 0, len(m.Mentions))
	for _, target := range m.Mentions {
		target = strings.TrimSpace(target)
		if target == "all" {
			parts = append(parts, "@全体成员")
			continue
		}
		qq, e := strconv.ParseInt(target, 10, 64)
		if e != nil || qq <= 0 {
			continue
		}
		if botID != "" && target == botID {
			parts = append(parts, "@机器人")
			continue
		}
		// 统一 "@显示名(QQ号)"：认得出是昵称，认不出显示名回落 "QQ:<qq>"。
		parts = append(parts, "@"+yyMemberResolve(m.Group, qq)+"("+target+")")
	}
	if len(parts) == 0 {
		if m.Mentioned {
			return "@机器人"
		}
		return ""
	}
	return strings.Join(parts, " ")
}

// yyMemberBlock 群成员名录（prompt 用）：按活跃度（MsgCount 降序）取前 limit
// 人，"张三(10001)、李四(10002)" 形态，总长 ≤600 字；装不下的计 omitted
// 写在末尾，让模型知道有遗漏、避免编造。
func yyMemberBlock(group string, limit int) string {
	if limit <= 0 {
		limit = yyMemberListDefault
	}
	yyMemberEnsure()
	yyMembers.Lock()
	rows := append([]yyQQMember(nil), yyMembers.Groups[group]...)
	yyMembers.Unlock()
	total := len(rows)
	if total == 0 {
		return ""
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].MsgCount != rows[j].MsgCount {
			return rows[i].MsgCount > rows[j].MsgCount
		}
		return rows[i].QQ < rows[j].QQ
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	var b strings.Builder
	shown := 0
	// 末尾要给 "…等N人未列出" 留位：取最坏长度（5 位数群）也放得下。
	const suffixReserve = 24
	for i, row := range rows {
		item := yyMemberLabel(row)
		if i > 0 {
			item = "、" + item
		}
		if utf8.RuneCountInString(b.String())+utf8.RuneCountInString(item)+suffixReserve > yyMemberBlockChars {
			break
		}
		b.WriteString(item)
		shown++
	}
	out := b.String()
	if omitted := total - shown; omitted > 0 {
		// 计数必须始终可见：模型靠它知道名录有遗漏，不能为了压字数把提示丢掉。
		out += "…等" + strconv.Itoa(omitted) + "人未列出"
	}
	return out
}

// yyMemberLabel 名录条目："显示名(QQ号)"。
func yyMemberLabel(row yyQQMember) string {
	name := row.Name
	if len(row.Aliases) > 0 {
		name = row.Aliases[0]
	}
	if name == "" {
		name = "QQ:" + strconv.FormatInt(row.QQ, 10)
	}
	return name + "(" + strconv.FormatInt(row.QQ, 10) + ")"
}

// yyMemberAliasEdit 面板用别名增删：成员不存在时增别名会建桩（Name 留空，
// 显示名回落 "QQ:<qq>"）；删别名要求成员与别名都已存在，不存在如实报错。
func yyMemberAliasEdit(group string, qq int64, alias string, add bool) error {
	group = strings.TrimSpace(group)
	if group == "" {
		return fmt.Errorf("缺少群号")
	}
	if qq <= 0 {
		return fmt.Errorf("QQ 号无效")
	}
	alias = strings.TrimSpace(alias)
	if add {
		if alias == "" {
			return fmt.Errorf("别名为空")
		}
		if len([]rune(alias)) > 24 {
			return fmt.Errorf("别名过长（最多 24 字）")
		}
		if strings.ContainsAny(alias, "[]@\n\r\t") {
			return fmt.Errorf("别名含有非法字符")
		}
	} else if alias == "" {
		return fmt.Errorf("缺少要删除的别名")
	}
	yyMemberEnsure()
	yyMembers.Lock()
	defer yyMembers.Unlock()
	rows := yyMembers.Groups[group]
	idx := -1
	for i := range rows {
		if rows[i].QQ == qq {
			idx = i
			break
		}
	}
	if add {
		if idx < 0 {
			if len(rows) >= yyMemberGroupMax {
				return fmt.Errorf("该群成员档案已满")
			}
			now := time.Now().Unix()
			rows = append(rows, yyQQMember{Group: group, QQ: qq, FirstSeen: now, LastSeen: now})
			idx = len(rows) - 1
		}
		yyMemberAddAliasLocked(&rows[idx], alias)
	} else {
		if idx < 0 {
			return fmt.Errorf("没有这个成员的档案")
		}
		kept := make([]string, 0, len(rows[idx].Aliases))
		found := false
		for _, a := range rows[idx].Aliases {
			if a == alias {
				found = true
				continue
			}
			kept = append(kept, a)
		}
		if !found {
			return fmt.Errorf("没有这个别名：%s", alias)
		}
		rows[idx].Aliases = kept
	}
	yyMembers.Groups[group] = rows
	yyMemberSaveLocked()
	return nil
}

// ---- 表情自我认知索引 ----

const (
	yyEmojiMax         = 500 // 索引条目上限
	yyEmojiBlockChars  = 500 // prompt 块字数上限
	yyEmojiListDefault = 100 // 块默认列出条数
)

// yyEmojiCaller 可注入的 OneBot 调用层：默认走真实 WebSocket
// （yyQQOneBotCall，youyuan_qq.go 里的通用 action 调用），测试用假函数替换，
// 绝不改 youyuan_qq.go。
var yyEmojiCaller func(action string, params map[string]any) (any, error) = yyQQOneBotCall

// yyEmojiActions 探测顺序：先试常见的 fetch_emoji_list，再退到本仓库已验证
// 可用的 fetch_custom_face（youyuan_qq_caps.go 的收藏表情，只读白名单里就有）。
// 两个都不可用时如实降级为手动维护模式，不假装调通。
var yyEmojiActions = []string{"fetch_emoji_list", "fetch_custom_face"}

// yyQQEmojiEntry 一个表情：ID（贴表情用的编号）、Wording（名字）、URL（图片）。
type yyQQEmojiEntry struct {
	ID      string `json:"id"`
	Wording string `json:"wording"`
	URL     string `json:"url,omitempty"`
}

// yyQQEmojiCache 缓存文件形状（qq-emoji.json）。Source 为成功时的 action 名；
// "manual" 表示手工维护（自动接口不可用的降级形态）；空串表示从未填充。
type yyQQEmojiCache struct {
	FetchedAt int64            `json:"fetched_at"`
	Source    string           `json:"source"`
	Entries   []yyQQEmojiEntry `json:"entries"`
}

var yyEmojiIndex = struct {
	sync.Mutex
	Path      string
	Cache     yyQQEmojiCache
	LastError string
	DiskError string
}{}

// yyEmojiInit 载入（或新建）dataDir/qq-emoji.json；损坏时如实记 DiskError 并
// 当空索引用，不阻塞链路。
func yyEmojiInit(root string) {
	yyEmojiIndex.Lock()
	defer yyEmojiIndex.Unlock()
	yyEmojiIndex.Path = filepath.Join(root, "qq-emoji.json")
	yyEmojiIndex.Cache = yyQQEmojiCache{}
	yyEmojiIndex.LastError = ""
	yyEmojiIndex.DiskError = ""
	file, e := os.Open(yyEmojiIndex.Path)
	if e != nil {
		return
	}
	defer file.Close()
	var cache yyQQEmojiCache
	if json.NewDecoder(io.LimitReader(file, 4<<20)).Decode(&cache) != nil {
		yyEmojiIndex.DiskError = "invalid_emoji_index"
		return
	}
	seen := map[string]bool{}
	kept := make([]yyQQEmojiEntry, 0, len(cache.Entries))
	for _, entry := range cache.Entries {
		entry.ID = strings.TrimSpace(entry.ID)
		entry.Wording = yyQQClip(strings.TrimSpace(entry.Wording), 40)
		entry.URL = yyQQClip(strings.TrimSpace(entry.URL), 300)
		if entry.ID == "" || seen[entry.ID] {
			continue
		}
		seen[entry.ID] = true
		kept = append(kept, entry)
		if len(kept) >= yyEmojiMax {
			break
		}
	}
	cache.Entries = kept
	cache.Source = strings.TrimSpace(cache.Source)
	yyEmojiIndex.Cache = cache
}

func yyEmojiEnsure() {
	yyEmojiIndex.Lock()
	ready := yyEmojiIndex.Path != ""
	yyEmojiIndex.Unlock()
	if ready || yyQQPath == "" {
		return
	}
	yyEmojiInit(filepath.Dir(yyQQPath))
}

// yyEmojiSaveLocked 原子落盘（调用方持锁）。
func yyEmojiSaveLocked() {
	if yyEmojiIndex.Path == "" {
		return
	}
	raw, e := json.Marshal(yyEmojiIndex.Cache)
	if e == nil && len(raw) > 4<<20 {
		e = fmt.Errorf("emoji_index_size_limit")
	}
	if e == nil {
		e = os.WriteFile(yyEmojiIndex.Path+".tmp", raw, 0600)
	}
	if e == nil {
		e = os.Rename(yyEmojiIndex.Path+".tmp", yyEmojiIndex.Path)
	}
	if e != nil {
		yyEmojiIndex.DiskError = "emoji_persistence_failed"
	} else {
		yyEmojiIndex.DiskError = ""
	}
}

// yyEmojiIndexRefresh 经可注入调用层拉取表情列表并写缓存（带 fetched_at）。
// 成功：条目整体替换、Source 记为 action 名；全部失败：返回 error、降级手动
// 维护模式，且不清空旧缓存（模型仍看得到上一次的索引）。
func yyEmojiIndexRefresh() error {
	yyEmojiEnsure()
	var lastErr error
	for _, action := range yyEmojiActions {
		data, e := yyEmojiCaller(action, map[string]any{})
		if e != nil {
			lastErr = e
			continue
		}
		entries := yyEmojiParse(data)
		if len(entries) == 0 {
			lastErr = fmt.Errorf("%s 返回了空列表", action)
			continue
		}
		yyEmojiIndex.Lock()
		yyEmojiIndex.Cache = yyQQEmojiCache{FetchedAt: time.Now().Unix(), Source: action, Entries: entries}
		yyEmojiIndex.LastError = ""
		yyEmojiSaveLocked()
		yyEmojiIndex.Unlock()
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的表情列表接口")
	}
	yyEmojiIndex.Lock()
	yyEmojiIndex.LastError = lastErr.Error()
	if yyEmojiIndex.Cache.Source == "" {
		// 从未拉到过自动索引：转手动维护模式；旧条目（可能为空）保留。
		yyEmojiIndex.Cache.Source = "manual"
		yyEmojiSaveLocked()
	}
	yyEmojiIndex.Unlock()
	return lastErr
}

// yyEmojiParse 把 action 返回值解析成 {ID, Wording, URL}。兼容三种形状：
// 裸数组（元素为对象或 URL 字符串）、{"result":[...]}、{"data":[...]} /
// {"emoji_list":[...]}；字段名兼容 id/emoji_id/emojiId、wording/name/desc、
// url/image。
func yyEmojiParse(data any) []yyQQEmojiEntry {
	var list []any
	switch v := data.(type) {
	case []any:
		list = v
	case map[string]any:
		for _, key := range []string{"result", "data", "emoji_list", "emojiList", "list"} {
			if rows, ok := v[key].([]any); ok {
				list = rows
				break
			}
		}
	}
	out := []yyQQEmojiEntry{}
	for i, item := range list {
		switch t := item.(type) {
		case string:
			url := strings.TrimSpace(t)
			if url == "" {
				continue
			}
			out = append(out, yyQQEmojiEntry{ID: url, Wording: "表情" + strconv.Itoa(i+1), URL: url})
		case map[string]any:
			entry := yyQQEmojiEntry{
				ID:      yyEmojiPick(t, "id", "emoji_id", "emojiId", "emojiID", "face_id"),
				Wording: yyEmojiPick(t, "wording", "name", "desc", "title"),
				URL:     yyEmojiPick(t, "url", "image", "thumb"),
			}
			if entry.ID == "" {
				entry.ID = entry.Wording
			}
			if entry.Wording == "" {
				entry.Wording = "表情" + entry.ID
			}
			if entry.ID == "" {
				continue
			}
			out = append(out, entry)
		}
		if len(out) >= yyEmojiMax {
			break
		}
	}
	return out
}

// yyEmojiPick 按候选键名取第一个非空字符串值（兼容数字型 ID）。
func yyEmojiPick(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok {
			if s := strings.TrimSpace(yyQQStableID(v)); s != "" {
				return yyQQClip(s, 300)
			}
		}
	}
	return ""
}

// yyEmojiIndexBlock 编号+名字纯文本列表（≤500 字）供 prompt 使用；
// 空索引返回空串（调用方据此不注入，避免浪费 token）。
func yyEmojiIndexBlock(limit int) string {
	if limit <= 0 {
		limit = yyEmojiListDefault
	}
	yyEmojiEnsure()
	yyEmojiIndex.Lock()
	entries := append([]yyQQEmojiEntry(nil), yyEmojiIndex.Cache.Entries...)
	yyEmojiIndex.Unlock()
	if len(entries) == 0 {
		return ""
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	var b strings.Builder
	shown := 0
	// 末尾要给 " …等N个" 留位：取最坏长度也放得下。
	const suffixReserve = 20
	for i, entry := range entries {
		name := entry.Wording
		if name == "" {
			name = entry.ID
		}
		item := strconv.Itoa(i+1) + "." + name
		if i > 0 {
			item = " " + item
		}
		if utf8.RuneCountInString(b.String())+utf8.RuneCountInString(item)+suffixReserve > yyEmojiBlockChars {
			break
		}
		b.WriteString(item)
		shown++
	}
	out := b.String()
	if omitted := len(entries) - shown; omitted > 0 {
		// 计数必须始终可见：模型靠它知道列表有遗漏，不能为了压字数把提示丢掉。
		out += " …等" + strconv.Itoa(omitted) + "个"
	}
	return out
}

// yyEmojiManualEdit 手动维护模式（POST /api/qq/emoji 增删条目）：ID 省略时
// 用名称充当，同 ID 覆盖。自动索引存在时手工编辑只做增量补充、Source 保持
// 原 action 名；从未拉到过自动索引时 Source 记 "manual"。
func yyEmojiManualEdit(add bool, entry yyQQEmojiEntry) error {
	yyEmojiEnsure()
	yyEmojiIndex.Lock()
	defer yyEmojiIndex.Unlock()
	if add {
		entry.ID = strings.TrimSpace(entry.ID)
		entry.Wording = yyQQClip(strings.TrimSpace(entry.Wording), 40)
		entry.URL = yyQQClip(strings.TrimSpace(entry.URL), 300)
		if entry.ID == "" {
			entry.ID = entry.Wording
		}
		if entry.ID == "" {
			return fmt.Errorf("缺少表情 ID 或名称")
		}
		if entry.Wording == "" {
			entry.Wording = "表情" + entry.ID
		}
		for i := range yyEmojiIndex.Cache.Entries {
			if yyEmojiIndex.Cache.Entries[i].ID == entry.ID {
				yyEmojiIndex.Cache.Entries[i] = entry
				yyEmojiManualSourceLocked()
				yyEmojiSaveLocked()
				return nil
			}
		}
		if len(yyEmojiIndex.Cache.Entries) >= yyEmojiMax {
			return fmt.Errorf("表情索引已满（%d 条）", yyEmojiMax)
		}
		yyEmojiIndex.Cache.Entries = append(yyEmojiIndex.Cache.Entries, entry)
	} else {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			return fmt.Errorf("缺少要删除的表情 ID")
		}
		kept := make([]yyQQEmojiEntry, 0, len(yyEmojiIndex.Cache.Entries))
		found := false
		for _, e := range yyEmojiIndex.Cache.Entries {
			if e.ID == id {
				found = true
				continue
			}
			kept = append(kept, e)
		}
		if !found {
			return fmt.Errorf("没有这个表情：%s", id)
		}
		yyEmojiIndex.Cache.Entries = kept
	}
	yyEmojiManualSourceLocked()
	yyEmojiSaveLocked()
	return nil
}

// yyEmojiManualSourceLocked 手工编辑后若从未有过自动来源，标记为 manual
// （调用方持锁）。
func yyEmojiManualSourceLocked() {
	if yyEmojiIndex.Cache.Source == "" {
		yyEmojiIndex.Cache.Source = "manual"
		yyEmojiIndex.Cache.FetchedAt = time.Now().Unix()
	}
}

// yyMemberRoutes 成员画像 + 表情索引面板接口。只定义不注册，由主智能体在
// youyuan_qq.go 的 yyRegisterQQ 里加一行 yyMemberRoutes(mux, dataDir)
// （与 yyQQImageRoutes / yyQQRagRoutes 同风格）；群消息入口另加一行
// yyMemberOnMessage(&in) 完成「认人」接线（in 为 yyQQParseInbound 的产物）。
func yyMemberRoutes(mux *http.ServeMux, dataDir string) {
	yyMemberInit(dataDir)
	yyEmojiInit(dataDir)

	mux.HandleFunc("/api/qq/members", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		group := strings.TrimSpace(r.URL.Query().Get("group"))
		if group == "" {
			group = yyQQGroupString(yyQQSnapshot().GroupID)
		}
		if group == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "缺少群号"})
			return
		}
		yyMemberEnsure()
		yyMembers.Lock()
		rows := append([]yyQQMember(nil), yyMembers.Groups[group]...)
		diskError := yyMembers.DiskError
		yyMembers.Unlock()
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].MsgCount != rows[j].MsgCount {
				return rows[i].MsgCount > rows[j].MsgCount
			}
			return rows[i].QQ < rows[j].QQ
		})
		if rows == nil {
			rows = []yyQQMember{}
		}
		writeJSON(w, map[string]any{"ok": true, "group": group, "count": len(rows), "members": rows, "persistence_error": diskError})
	})

	mux.HandleFunc("/api/qq/members/alias", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var p struct {
			Group  string `json:"group"`
			QQ     any    `json:"qq"`
			Alias  string `json:"alias"`
			Action string `json:"action"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p) != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		qq, e := strconv.ParseInt(yyQQStableID(p.QQ), 10, 64)
		if e != nil || qq <= 0 {
			writeJSON(w, map[string]any{"ok": false, "error": "QQ 号无效"})
			return
		}
		group := strings.TrimSpace(p.Group)
		add := strings.ToLower(strings.TrimSpace(p.Action)) != "del"
		if e := yyMemberAliasEdit(group, qq, p.Alias, add); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "display_name": yyMemberResolve(group, qq)})
	})

	mux.HandleFunc("/api/qq/emoji", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			yyEmojiEnsure()
			yyEmojiIndex.Lock()
			cache := yyEmojiIndex.Cache
			lastError, diskError := yyEmojiIndex.LastError, yyEmojiIndex.DiskError
			yyEmojiIndex.Unlock()
			entries := cache.Entries
			if entries == nil {
				entries = []yyQQEmojiEntry{}
			}
			writeJSON(w, map[string]any{
				"ok": true, "count": len(entries), "entries": entries,
				"source":     cache.Source,
				"manual":     cache.Source == "" || cache.Source == "manual",
				"fetched_at": cache.FetchedAt,
				"last_error": lastError, "persistence_error": diskError,
			})
		case http.MethodPost:
			var p struct {
				Action  string `json:"action"`
				ID      string `json:"id"`
				Wording string `json:"wording"`
				URL     string `json:"url"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p) != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
				return
			}
			add := strings.ToLower(strings.TrimSpace(p.Action)) != "del"
			if e := yyEmojiManualEdit(add, yyQQEmojiEntry{ID: p.ID, Wording: p.Wording, URL: p.URL}); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			yyEmojiIndex.Lock()
			count, source := len(yyEmojiIndex.Cache.Entries), yyEmojiIndex.Cache.Source
			yyEmojiIndex.Unlock()
			writeJSON(w, map[string]any{"ok": true, "count": count, "source": source})
		default:
			http.Error(w, "GET/POST required", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/qq/emoji/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		e := yyEmojiIndexRefresh()
		yyEmojiIndex.Lock()
		count, source := len(yyEmojiIndex.Cache.Entries), yyEmojiIndex.Cache.Source
		yyEmojiIndex.Unlock()
		if e != nil {
			// 如实降级：告诉你为什么失败、保留了几条旧索引、现在是不是手动模式。
			writeJSON(w, map[string]any{"ok": false, "error": e.Error(), "degraded_to_manual": true, "kept_entries": count, "source": source})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "count": count, "source": source})
	})
}
