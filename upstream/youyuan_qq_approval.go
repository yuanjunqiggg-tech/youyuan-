package main

// 群内危险指令授权
// ---------------------------------------------------------------------------
// 用户原话：「指令形式，一开始就加个黑名单拦截，他没办法做到一些东西，比如
// OP、ban、fill 这种，他要在聊天群里面问我同不同意，我说同意或者别的，或者
// 相应的菜单编号输入了，怎么简便怎么来吧，反正就是我同意了才行」。
//
// 所以这里不是「拒绝」，是「挂起」：
//   1. 机器人想执行黑名单里的指令 → 不执行，建一条待授权项，群里回一句人话带 4 位编号；
//   2. 超级管理员（yyQQRole==2）在群里回「同意 1234」/「拒绝 1234」/「1234」；
//   3. 批准才真正下发；拒绝就作废；10 分钟没人理自动过期。
//
// 顺带解决「重复执行」：每次成功执行都记一条 last（按 群+人 保留 5 条），
// `.再来一次` 重放最近一条——重放仍然要过黑名单，不会因为有记录就跳过授权。
//
// 所有新标识符统一 yyAppr 前缀，避免与并行开发的模块撞名。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	yyApprTTL      = 10 * time.Minute // 待授权项的有效期
	yyApprMaxItems = 200              // 队列上限（含已决，超过就淘汰最老的已决项）
	yyApprKeepLast = 5                // 每人每群保留几条「上次执行」

	yyApprKindCommand = "command" // 群 /命令 的原文，可能多行命令链
	yyApprKindConsole = "console" // 面板单条控制台指令
)

// yyApprItem 一条待授权项。落盘字段与 JSON 一一对应，重启后未决项不丢。
type yyApprItem struct {
	ID            string    `json:"id"` // 4 位数字编号，群里就回这个
	Kind          string    `json:"kind"`
	Payload       string    `json:"payload"` // 要执行的原文
	Reason        string    `json:"reason"`  // 为什么被判为危险（给人看的）
	RequestedBy   string    `json:"requested_by"`
	RequestedName string    `json:"requested_name"`
	Group         string    `json:"group"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Status        string    `json:"status"` // pending / approved / rejected / expired
	DecidedBy     string    `json:"decided_by,omitempty"`
	DecidedName   string    `json:"decided_name,omitempty"`
	Result        string    `json:"result,omitempty"`
}

// yyApprLast 一条「上次成功执行」，只为重放服务。
type yyApprLast struct {
	Kind    string    `json:"kind"`
	Payload string    `json:"payload"`
	At      time.Time `json:"at"`
}

// yyApprStore 内存队列 + 落盘位置。用命名类型才挂得上方法。
type yyApprStore struct {
	sync.Mutex
	Path  string
	Seq   int
	Items []yyApprItem
	Last  map[string][]yyApprLast
}

var yyAppr = yyApprStore{Last: map[string][]yyApprLast{}}

// yyApprExec 执行钩子：默认 nil，走真实的游戏通道；测试里替换掉，不碰网络。
var yyApprExec func(yyApprItem) (string, error)

// yyApprInit 指定落盘位置并载入历史。dir 为空表示只留在内存里。
func yyApprInit(dir string) {
	yyAppr.Lock()
	defer yyAppr.Unlock()
	yyAppr.Path = ""
	if dir != "" {
		yyAppr.Path = filepath.Join(dir, "qq-approvals.json")
	}
	yyAppr.Items = nil
	yyAppr.Last = map[string][]yyApprLast{}
	if yyAppr.Path == "" {
		return
	}
	raw, e := os.ReadFile(yyAppr.Path)
	if e != nil {
		return
	}
	var saved struct {
		Items []yyApprItem            `json:"items"`
		Last  map[string][]yyApprLast `json:"last"`
	}
	if json.Unmarshal(raw, &saved) != nil {
		return
	}
	yyAppr.Items = saved.Items
	if saved.Last != nil {
		yyAppr.Last = saved.Last
	}
	yyAppr.expireLocked(time.Now())
}

func (s *yyApprStore) expireLocked(now time.Time) {
	for i := range s.Items {
		if s.Items[i].Status == "pending" && now.After(s.Items[i].ExpiresAt) {
			s.Items[i].Status = "expired"
		}
	}
}

func yyApprSaveLocked() {
	if yyAppr.Path == "" {
		return
	}
	raw, e := json.MarshalIndent(map[string]any{"items": yyAppr.Items, "last": yyAppr.Last}, "", "  ")
	if e != nil {
		return
	}
	tmp := yyAppr.Path + ".tmp"
	if e = os.WriteFile(tmp, raw, 0600); e != nil {
		yyQQLog("授权队列落盘失败：" + e.Error())
		return
	}
	if e = os.Rename(tmp, yyAppr.Path); e != nil {
		yyQQLog("授权队列落盘失败：" + e.Error())
	}
}

// yyApprPruneLocked 控制队列体积：先清过期，超上限时先扔最老的已决项。
func yyApprPruneLocked(now time.Time) {
	yyAppr.expireLocked(now)
	if len(yyAppr.Items) <= yyApprMaxItems {
		return
	}
	kept := make([]yyApprItem, 0, yyApprMaxItems)
	// 已决/过期的按时间从新到旧补满，未决的永远保留。
	pending := make([]yyApprItem, 0, len(yyAppr.Items))
	decided := make([]yyApprItem, 0, len(yyAppr.Items))
	for _, it := range yyAppr.Items {
		if it.Status == "pending" {
			pending = append(pending, it)
		} else {
			decided = append(decided, it)
		}
	}
	kept = append(kept, pending...)
	room := yyApprMaxItems - len(kept)
	if room < 0 {
		room = 0
	}
	if len(decided) > room {
		decided = decided[len(decided)-room:]
	}
	kept = append(kept, decided...)
	yyAppr.Items = kept
}

// yyApprNewIDLocked 4 位数字编号。避开所有尚未过期的编号，防止「同意 1234」
// 命中一条更老的、别人刚建的项目。
func yyApprNewIDLocked(now time.Time) string {
	used := map[string]bool{}
	for _, it := range yyAppr.Items {
		if it.Status == "pending" || now.Sub(it.CreatedAt) < yyApprTTL {
			used[it.ID] = true
		}
	}
	for i := 0; i < 9000; i++ {
		yyAppr.Seq = (yyAppr.Seq + 1) % 9000
		id := strconv.Itoa(1000 + yyAppr.Seq)
		if !used[id] {
			return id
		}
	}
	return strconv.Itoa(1000 + now.Nanosecond()%9000)
}

// yyApprRequest 建一条待授权项。group 为空表示任意群都能批（面板发起的）。
func yyApprRequest(cf yyQQConfig, group, user, name, kind, payload string) (yyApprItem, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return yyApprItem{}, fmt.Errorf("没有要执行的指令")
	}
	if kind == "" {
		kind = yyApprKindCommand
	}
	now := time.Now()
	yyAppr.Lock()
	defer yyAppr.Unlock()
	yyApprPruneLocked(now)
	pending := 0
	for _, it := range yyAppr.Items {
		if it.Status == "pending" {
			pending++
		}
	}
	if pending >= yyApprMaxItems {
		return yyApprItem{}, fmt.Errorf("待授权的指令已有 %d 条，请先处理", pending)
	}
	item := yyApprItem{
		ID:            yyApprNewIDLocked(now),
		Kind:          kind,
		Payload:       payload,
		Reason:        yyApprDangerReason(payload),
		RequestedBy:   user,
		RequestedName: name,
		Group:         group,
		CreatedAt:     now,
		ExpiresAt:     now.Add(yyApprTTL),
		Status:        "pending",
	}
	yyAppr.Items = append(yyAppr.Items, item)
	yyApprSaveLocked()
	return item, nil
}

// yyApprPayloadBlocked 这些不是「危险动作」，而是「根本不该进队列」的输入：
// 含控制字符（防注入）。审批也不该放行，所以单独挡在授权之前。
func yyApprPayloadBlocked(payload string) string {
	for _, line := range strings.Split(payload, "\n") {
		if yySkillHasControl(line) {
			return "指令含控制字符，已拒绝（防注入，不走审批）"
		}
	}
	return ""
}

// yyApprDangerReason 返回被判为危险的原因；空字符串表示可以直接执行。
func yyApprDangerReason(payload string) string {
	for _, line := range strings.Split(payload, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if e := yySrvInfoCommandGuard("/" + strings.TrimLeft(line, "/")); e != nil {
			return e.Error()
		}
	}
	return ""
}

// yyApprPending 取出某个群的待决项（group 为空表示不限群）。
func yyApprPending(group string) []yyApprItem {
	now := time.Now()
	yyAppr.Lock()
	defer yyAppr.Unlock()
	yyAppr.expireLocked(now)
	out := make([]yyApprItem, 0, 4)
	for _, it := range yyAppr.Items {
		if it.Status != "pending" {
			continue
		}
		if group != "" && it.Group != "" && it.Group != group {
			continue
		}
		out = append(out, it)
	}
	return out
}

// yyApprDecide 批准/拒绝。只有超级管理员能批（yyQQRole 为 2）。
// 批准后立即执行，执行结果记回条目，并作为「上次执行」留给 `.再来一次`。
func yyApprDecide(id, approver, name string, approve bool) error {
	cf := yyQQSnapshot()
	if yyQQRole(cf, approver) < 2 {
		return fmt.Errorf("只有超级管理员可以批准或拒绝执行")
	}
	now := time.Now()
	yyAppr.Lock()
	yyAppr.expireLocked(now)
	index := -1
	for i := range yyAppr.Items {
		if yyAppr.Items[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		yyAppr.Unlock()
		return fmt.Errorf("编号 %s 不存在（可能已被清理）", id)
	}
	item := yyAppr.Items[index]
	switch item.Status {
	case "expired":
		yyAppr.Unlock()
		return fmt.Errorf("编号 %s 已过期（超过 %d 分钟未确认），请重新发起", id, int(yyApprTTL.Minutes()))
	case "approved", "rejected":
		yyAppr.Unlock()
		return fmt.Errorf("编号 %s 已经处理过了", id)
	}
	if approve {
		yyAppr.Items[index].Status = "approved"
	} else {
		yyAppr.Items[index].Status = "rejected"
	}
	yyAppr.Items[index].DecidedBy = approver
	yyAppr.Items[index].DecidedName = name
	yyApprSaveLocked()
	yyAppr.Unlock()

	if !approve {
		yyQQLog("授权 " + id + " 被 " + name + " 拒绝：" + yyQQClip(item.Payload, 80))
		return nil
	}

	out, e := yyApprExecute(cf, item)
	result := out
	if e != nil {
		result = "执行失败：" + e.Error()
	}
	yyAppr.Lock()
	for i := range yyAppr.Items {
		if yyAppr.Items[i].ID == id {
			yyAppr.Items[i].Result = yyQQClip(result, 300)
			break
		}
	}
	yyApprSaveLocked()
	yyAppr.Unlock()

	rec := yyQQActionRecord{
		Time:   time.Now(),
		Kind:   "approval",
		Caller: item.RequestedBy,
		Name:   item.RequestedName,
		Target: id,
		Detail: yyQQClip(item.Payload, 80),
		OK:     e == nil && yyApprLooksOK(out),
	}
	if !rec.OK {
		rec.Error = yyQQClip(result, 120)
	}
	yyQQActions.log(rec)
	if rec.OK {
		yyApprRemember(item.Group, item.RequestedBy, item.Kind, item.Payload)
	}
	if e != nil {
		return e
	}
	return nil
}

// yyApprExecute 走下发的真实通道：面板单条走控制台，群 /命令 走命令链。
func yyApprExecute(cf yyQQConfig, item yyApprItem) (string, error) {
	if yyApprExec != nil {
		return yyApprExec(item)
	}
	if item.Kind == yyApprKindConsole {
		return yySkillConsoleSend(item.Payload, 8*time.Second)
	}
	return yyQQExecuteText(cf, item.Payload), nil
}

// yyApprLooksOK 指令链通道用字符串回执，只能靠关键字判断有没有真的执行。
func yyApprLooksOK(out string) bool {
	for _, mark := range []string{"失败", "语法错误", "未执行", "拒绝", "无此", "未知", "超时", "不合法", "1..128"} {
		if strings.Contains(out, mark) {
			return false
		}
	}
	return strings.TrimSpace(out) != ""
}

// ---- 重复执行 ----

func yyApprLastKey(group, user string) string { return group + "\x00" + user }

// yyApprRemember 记一条「上次成功执行」，同一人同一群保留最近 5 条。
func yyApprRemember(group, user, kind, payload string) {
	payload = strings.TrimSpace(payload)
	if user == "" || payload == "" {
		return
	}
	key := yyApprLastKey(group, user)
	yyAppr.Lock()
	defer yyAppr.Unlock()
	list := yyAppr.Last[key]
	for len(list) > 0 && list[0].Payload == payload {
		list = list[1:]
	}
	next := append([]yyApprLast{{Kind: kind, Payload: payload, At: time.Now()}}, list...)
	if len(next) > yyApprKeepLast {
		next = next[:yyApprKeepLast]
	}
	yyAppr.Last[key] = next
	yyApprSaveLocked()
}

// yyApprLastOf 取该人该群最近一次成功执行的指令。
func yyApprLastOf(group, user string) (yyApprLast, bool) {
	yyAppr.Lock()
	defer yyAppr.Unlock()
	list := yyAppr.Last[yyApprLastKey(group, user)]
	if len(list) == 0 {
		return yyApprLast{}, false
	}
	return list[0], true
}

// ---- 群消息入口 ----

// yyApprParseVote 解析「同意 1234 / 拒绝 1234 / 1234 / 1 / 同意」。
// verb: "approve" / "reject" / ""（只给了编号）。
func yyApprParseVote(text string) (verb, id string, ok bool) {
	s := strings.TrimSpace(text)
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "."), "/"))
	s = strings.ReplaceAll(s, "　", " ")
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "", "", false
	}
	approveWords := []string{"同意", "批准", "通过", "可以", "执行吧", "ok", "OK", "yes"}
	rejectWords := []string{"拒绝", "不同意", "驳回", "不行", "别执行", "no"}
	rest := s
	for _, w := range approveWords {
		if strings.HasPrefix(s, w) {
			verb, rest = "approve", strings.TrimSpace(strings.TrimPrefix(s, w))
			break
		}
	}
	if verb == "" {
		for _, w := range rejectWords {
			if strings.HasPrefix(s, w) {
				verb, rest = "reject", strings.TrimSpace(strings.TrimPrefix(s, w))
				break
			}
		}
	}
	if verb != "" {
		rest = strings.TrimSpace(strings.TrimLeft(rest, "#编号:："))
		if rest == "" {
			return verb, "", true
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return verb, "", true
		}
		return verb, fields[0], true
	}
	// 纯数字：4 位当编号，1 位（1..9）当「只有一个待决项」的快捷确认。
	if len(s) <= 4 && s != "" {
		for _, r := range s {
			if r < '0' || r > '9' {
				return "", "", false
			}
		}
		return "", s, true
	}
	return "", "", false
}

// yyApprTake 群消息里的授权回复入口。
// 只在「这个群有 pending 项」且发言人是超级管理员时接管，避免吞掉普通聊天。
func yyApprTake(group, user, text string) (bool, string) {
	verb, id, ok := yyApprParseVote(text)
	if !ok {
		return false, ""
	}
	pending := yyApprPending(group)
	if len(pending) == 0 {
		return false, ""
	}
	cf := yyQQSnapshot()
	role := yyQQRole(cf, user)
	name := yyQQDisplayName(cf, user, user)

	var target *yyApprItem
	for i := range pending {
		if pending[i].ID == id {
			target = &pending[i]
			break
		}
	}
	if target == nil && len(pending) == 1 && (id == "1" || verb != "") {
		target = &pending[0]
	}
	if target == nil {
		// 编号对不上，或没有编号：不接管，交给普通指令链路。
		return false, ""
	}
	if role < 2 {
		if verb == "" && id != target.ID {
			return false, ""
		}
		return true, "这条要超级管理员点头才行（你当前是" + yyQQRoleName(role) + "）。等服主回复「同意 " + target.ID + "」。"
	}
	if verb == "" && id != target.ID && len(pending) != 1 {
		return false, ""
	}
	approve := verb != "reject"
	if err := yyApprDecide(target.ID, user, name, approve); err != nil {
		return true, "处理失败：" + err.Error()
	}
	if !approve {
		return true, "🚫 已拒绝 " + target.ID + "，指令没有执行。"
	}
	detail := ""
	yyAppr.Lock()
	for _, it := range yyAppr.Items {
		if it.ID == target.ID {
			detail = it.Result
			break
		}
	}
	yyAppr.Unlock()
	if strings.TrimSpace(detail) == "" {
		return true, "✅ 已批准 " + target.ID + " 并执行。"
	}
	return true, "✅ 已批准 " + target.ID + " 并执行：\n" + yyQQClip(detail, 600)
}

func yyQQRoleName(role int) string {
	if role >= 2 {
		return "超级管理员"
	}
	if role == 1 {
		return "管理员"
	}
	return "成员"
}

// yyApprRunOrAsk 统一的「执行或申请授权」入口。
// 过守卫就直接执行并记一条上次执行；没过就挂起并返回人话提示。
func yyApprRunOrAsk(cf yyQQConfig, group, user, name, kind, payload string) []string {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return []string{"要执行的指令是空的。"}
	}
	if blocked := yyApprPayloadBlocked(payload); blocked != "" {
		return []string{blocked}
	}
	if reason := yyApprDangerReason(payload); reason != "" {
		item, e := yyApprRequest(cf, group, user, name, kind, payload)
		if e != nil {
			return []string{"⚠️ 这条指令有风险（" + reason + "），但授权申请没建成：" + e.Error()}
		}
		return []string{"⚠️ 这条指令有风险（" + reason + "）。\n同意执行请回复：同意 " + item.ID + "\n拒绝请回复：拒绝 " + item.ID + "\n（" + fmt.Sprint(int(yyApprTTL.Minutes())) + " 分钟内有效）"}
	}
	out, e := yyApprExecute(cf, yyApprItem{Kind: kind, Payload: payload, Group: group, RequestedBy: user, RequestedName: name})
	if e != nil {
		return []string{e.Error()}
	}
	if yyApprLooksOK(out) {
		yyApprRemember(group, user, kind, payload)
	}
	return []string{out}
}

// ---- 路由：/api/qq/roles ----

func yyApprRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/roles", yyApprHandleRoles)
	mux.HandleFunc("/api/qq/approvals", yyApprHandleApprovals)
}

type yyApprWho struct {
	QQ   string `json:"qq"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// yyApprHandleRoles 只读：告诉面板「现在谁是管理员、谁是超级管理员」。
// 名字优先用 yyQQHelper.Bindings（QQ→游戏名）反查，没有就退到 Seen 昵称，再没有留空。
func yyApprHandleRoles(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeJSON(w, map[string]any{"ok": false, "error": "只支持 GET"})
		return
	}
	cf := yyQQSnapshot()
	yyQQHelper.Lock()
	bindings := make(map[string]string, len(yyQQHelper.Bindings))
	for qq, player := range yyQQHelper.Bindings {
		bindings[qq] = player
	}
	seen := make(map[string]string, len(yyQQHelper.Seen))
	for qq, nick := range yyQQHelper.Seen {
		seen[qq] = nick
	}
	yyQQHelper.Unlock()

	build := func(list []string, role string) []yyApprWho {
		out := make([]yyApprWho, 0, len(list))
		seenQQ := map[string]bool{}
		for _, qq := range list {
			qq = strings.TrimSpace(qq)
			if qq == "" || seenQQ[qq] {
				continue
			}
			seenQQ[qq] = true
			name := strings.TrimSpace(bindings[qq])
			if name == "" {
				name = strings.TrimSpace(seen[qq])
			}
			out = append(out, yyApprWho{QQ: qq, Name: name, Role: role})
		}
		return out
	}
	supers := build(cf.Assistant.SuperAdmins, "super_admin")
	admins := build(cf.Assistant.Admins, "admin")
	writeJSON(w, map[string]any{
		"ok":           true,
		"super_admins": supers,
		"admins":       admins,
		"note":         "只有超级管理员能批准群内危险指令；名单在面板「群服互通 · 助手」里配置",
	})
}

// yyApprHandleApprovals 只读：给面板看队列现状（谁申请的、批没批）。
func yyApprHandleApprovals(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeJSON(w, map[string]any{"ok": false, "error": "只支持 GET"})
		return
	}
	now := time.Now()
	yyAppr.Lock()
	yyAppr.expireLocked(now)
	rows := make([]yyApprItem, 0, len(yyAppr.Items))
	rows = append(rows, yyAppr.Items...)
	yyAppr.Unlock()
	// 新的在前，面板第一屏才有用。
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	writeJSON(w, map[string]any{"ok": true, "approvals": rows, "ttl_seconds": int(yyApprTTL.Seconds())})
}
