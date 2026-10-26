package main

// 游戏技能注册表（去 AI 化）。
//
// 核心约束：AI 永远不生成原始控制台命令，只能从本注册表选择技能执行。
// 技能 = 数据（命令模板 + 参数 schema + 权限 + 超时 + 格式化），执行是确定性的：
// 参数校验 → 模板渲染 → 防注入 → 经可注入的执行通道下发 → 按 Format 格式化 → 留痕。
// 面板与群指令、AI 工具调用走同一条 yyGameSkillRun，避免「面板一个样、执行另一个样」。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// yyGameSkillParam 技能参数定义。Type: string|int|player|enum。
type yyGameSkillParam struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  string   `json:"default,omitempty"`
	Options  []string `json:"options,omitempty"` // enum 取值
}

// yyGameSkill 一个可注册的游戏技能。Builtin 技能不可删、不可改名，只能停用或调整权限与超时。
type yyGameSkill struct {
	Name      string             `json:"name"`
	Label     string             `json:"label"`
	Desc      string             `json:"desc"`
	Category  string             `json:"category"`
	MinRole   int                `json:"min_role"` // 0 成员 1 管理员 2 超级管理员
	Run       string             `json:"run"`      // console | api | builtin
	Command   string             `json:"command"`  // 命令模板，占位符 {player} {arg} 等对应参数名
	Channel   string             `json:"channel"`  // ai | console | player
	Params    []yyGameSkillParam `json:"params"`
	Format    string             `json:"format"` // text | list | json | count
	TimeoutMS int                `json:"timeout_ms"`
	Confirm   bool               `json:"confirm"` // 执行前需人工确认（如 reload）
	Enabled   bool               `json:"enabled"`
	Builtin   bool               `json:"builtin"`
}

// yySkillLogEntry 技能执行留痕（内存环形，最近 200 条）。
type yySkillLogEntry struct {
	Time   time.Time         `json:"time"`
	Skill  string            `json:"skill"`
	Params map[string]string `json:"params"`
	OK     bool              `json:"ok"`
	Ms     int64             `json:"ms"`
	Out    string            `json:"out"`
}

// yySkillPatch 记录面板显式提交的可改字段；指针用于区分「未提交」与「提交了 false」。
type yySkillPatch struct {
	Enabled   *bool `json:"enabled"`
	MinRole   *int  `json:"min_role"`
	TimeoutMS *int  `json:"timeout_ms"`
	Confirm   *bool `json:"confirm"`
}

// yySkillEntry 整体保存时的一条提交（内置技能只认可 Patch 里的可改字段）。
type yySkillEntry struct {
	Skill yyGameSkill
	Patch yySkillPatch
}

var yySkillNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)
var yySkillParamNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
var yySkillPlayerName = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)
var yySkillPlaceholder = regexp.MustCompile(`\{([a-zA-Z][a-zA-Z0-9_]*)\}`)
var yySkillFirstInt = regexp.MustCompile(`[0-9]+`)

const yySkillLogKeep = 200
const yySkillToolPrefix = "skill_"

// yySkillMaxCount 技能条数上限。原来是写死的 100（内置 12 + 自定义 88），
// 用户要求「以后玩家想添加到四五十个，就是把权限放开」：放到 200。
// 内置仍然只有 12 个，其余全是玩家自定义，所以「能加到四五十个」是硬保证。
const yySkillMaxCount = 200

// yyGameSkillStore 注册表运行态。Path 为空表示未初始化（测试外的默认路径由 yyGameSkillInit 设置）。
var yyGameSkillStore = struct {
	sync.Mutex
	Path    string
	Items   []yyGameSkill
	DiskErr string
}{}

// yySkillLogs 留痕环形缓冲，最新在前。
var yySkillLogs = struct {
	sync.Mutex
	Items []yySkillLogEntry
}{}

// yySkillExec 技能执行通道：下发一条命令并返回输出文本。可整体替换（测试用），
// 默认实现走 youyuan_qq.go 现有的控制台发送函数 botMgr.SendConsole。
type yySkillExec func(command string, timeout time.Duration) (string, error)

var yySkillExecutor yySkillExec = yySkillConsoleSend

// yySkillOnlinePlayers 内置技能 online_list 的在线名单来源，可整体替换（测试用）；
// 默认实现走 youyuan_qq_assistant.go 现有的 yyQQOnlinePlayers（经 bot 连接查询 list）。
var yySkillOnlinePlayers = yyQQOnlinePlayers

// yyGameSkillBuiltins 内置技能表：每次启动以代码为准重新生成，
// 旧数据文件只能覆盖「可改字段」，防止升级后内置技能被旧文件固化或删失。
func yyGameSkillBuiltins() []yyGameSkill {
	return []yyGameSkill{
		{
			Name: "online_list", Label: "在线玩家", Desc: "查询当前在线玩家名单与人数（每页12人，可用页码翻页）",
			Category: "query", MinRole: 0, Run: "builtin", Channel: "ai",
			Params: []yyGameSkillParam{
				{Name: "page", Label: "页码", Type: "int", Required: false, Default: "1"},
			},
			Format: "list", TimeoutMS: 3000, Enabled: true, Builtin: true,
		},
		{
			Name: "server_status", Label: "服务器状态", Desc: "查询游戏连接与QQ桥接状态",
			Category: "query", MinRole: 0, Run: "builtin", Channel: "ai",
			Format: "text", TimeoutMS: 3000, Enabled: true, Builtin: true,
		},
		{
			Name: "player_info", Label: "玩家资料", Desc: "查询指定玩家的观测资料（在线状态与发言统计）",
			Category: "query", MinRole: 0, Run: "builtin", Channel: "ai",
			Params: []yyGameSkillParam{{Name: "player", Label: "游戏ID", Type: "player", Required: true}},
			Format: "text", TimeoutMS: 3000, Enabled: true, Builtin: true,
		},
		{
			Name: "scoreboard_count", Label: "记分板数量", Desc: "只查询记分板数量，不暴露记分项名称",
			Category: "query", MinRole: 1, Run: "console", Command: "scoreboard objectives list",
			Channel: "ai", Format: "count", TimeoutMS: 3000, Enabled: true, Builtin: true,
		},
		{
			Name: "say", Label: "服务器广播", Desc: "以服务器身份向全服广播一条消息",
			Category: "world", MinRole: 1, Run: "console", Command: "/say {msg}",
			Channel: "ai", Format: "text", TimeoutMS: 3000, Enabled: true, Builtin: true,
			Params: []yyGameSkillParam{{Name: "msg", Label: "广播内容", Type: "string", Required: true}},
		},
		{
			Name: "tp", Label: "传送玩家", Desc: "把一名玩家传送到另一名玩家所在位置",
			Category: "player", MinRole: 1, Run: "console", Command: "/tp {from} {to}",
			Channel: "ai", Format: "text", TimeoutMS: 3000, Enabled: true, Builtin: true,
			Params: []yyGameSkillParam{
				{Name: "from", Label: "被传送者游戏ID", Type: "player", Required: true},
				{Name: "to", Label: "目标玩家游戏ID", Type: "player", Required: true},
			},
		},
		{
			Name: "kick", Label: "踢出玩家", Desc: "把指定玩家踢出服务器",
			Category: "moderate", MinRole: 1, Run: "console", Command: "/kick {player} {reason}",
			Channel: "ai", Format: "text", TimeoutMS: 3000, Enabled: true, Builtin: true,
			Params: []yyGameSkillParam{
				{Name: "player", Label: "游戏ID", Type: "player", Required: true},
				{Name: "reason", Label: "原因", Type: "string", Required: false, Default: "违规行为"},
			},
		},
		{
			Name: "gamemode", Label: "修改游戏模式", Desc: "修改指定玩家的游戏模式",
			Category: "player", MinRole: 1, Run: "console", Command: "/gamemode {mode} {player}",
			Channel: "ai", Format: "text", TimeoutMS: 3000, Enabled: true, Builtin: true,
			Params: []yyGameSkillParam{
				{Name: "mode", Label: "游戏模式", Type: "enum", Required: true, Options: []string{"survival", "creative", "adventure", "spectator"}},
				{Name: "player", Label: "游戏ID", Type: "player", Required: true},
			},
		},
		{
			Name: "time_set", Label: "设置时间", Desc: "设置服务器世界时间",
			Category: "world", MinRole: 1, Run: "console", Command: "/time set {value}",
			Channel: "ai", Format: "text", TimeoutMS: 3000, Enabled: true, Builtin: true,
			Params: []yyGameSkillParam{{Name: "value", Label: "时间", Type: "enum", Required: true, Options: []string{"day", "noon", "night"}}},
		},
		{
			Name: "weather", Label: "设置天气", Desc: "设置服务器天气",
			Category: "world", MinRole: 1, Run: "console", Command: "/weather {value}",
			Channel: "ai", Format: "text", TimeoutMS: 3000, Enabled: true, Builtin: true,
			Params: []yyGameSkillParam{{Name: "value", Label: "天气", Type: "enum", Required: true, Options: []string{"clear", "rain", "thunder"}}},
		},
		{
			Name: "whitelist_add", Label: "添加白名单", Desc: "把指定玩家加入服务器白名单",
			Category: "admin", MinRole: 2, Run: "console", Command: "/whitelist add {player}",
			Channel: "ai", Format: "text", TimeoutMS: 3000, Enabled: true, Builtin: true,
			Params: []yyGameSkillParam{{Name: "player", Label: "游戏ID", Type: "player", Required: true}},
		},
		{
			Name: "reload", Label: "重载服务器", Desc: "重载服务器配置与数据包，影响全服，执行前需确认",
			Category: "admin", MinRole: 2, Run: "console", Command: "/reload",
			Channel: "ai", Format: "text", TimeoutMS: 5000, Enabled: true, Builtin: true, Confirm: true,
		},
	}
}

// ---- 注册表加载与持久化（qq-skills.json） ----

// yyGameSkillInit 加载 dataDir/qq-skills.json。内置表每次以代码为准合并：
// 文件里的内置条目只覆盖可改字段，自定义技能整条载入；文件损坏时回退到纯内置表并记录 DiskErr。
func yyGameSkillInit(root string) {
	yyGameSkillStore.Lock()
	defer yyGameSkillStore.Unlock()
	yyGameSkillStore.Path = filepath.Join(root, "qq-skills.json")
	yyGameSkillStore.Items = yyGameSkillBuiltins()
	yyGameSkillStore.DiskErr = ""
	raw, e := os.ReadFile(yyGameSkillStore.Path)
	if e != nil {
		return // 首次运行或文件缺失：纯内置表
	}
	var saved struct {
		Skills []json.RawMessage `json:"skills"`
	}
	if json.Unmarshal(raw, &saved) != nil || saved.Skills == nil {
		yyGameSkillStore.DiskErr = "qq-skills.json 损坏，已回退到内置技能表"
		return
	}
	yyGameSkillStore.Items = yyGameSkillMergeLocked(yyGameSkillStore.Items, saved.Skills)
}

// yyGameSkillMergeLocked 把文件条目合并到内置表上（调用方已持锁）。
func yyGameSkillMergeLocked(base []yyGameSkill, stored []json.RawMessage) []yyGameSkill {
	byName := make(map[string]int, len(base))
	for i, s := range base {
		byName[s.Name] = i
	}
	out := append([]yyGameSkill(nil), base...)
	for _, item := range stored {
		var head struct {
			Name    string `json:"name"`
			Builtin bool   `json:"builtin"`
		}
		if json.Unmarshal(item, &head) != nil {
			continue
		}
		name := strings.TrimSpace(head.Name)
		if name == "" {
			continue
		}
		if idx, ok := byName[name]; ok {
			// 内置技能：命令模板与参数定义永远以代码为准，只接受可改字段。
			var patch yySkillPatch
			if json.Unmarshal(item, &patch) == nil {
				if patch.Enabled != nil {
					out[idx].Enabled = *patch.Enabled
				}
				if patch.MinRole != nil {
					out[idx].MinRole = *patch.MinRole
				}
				if patch.TimeoutMS != nil {
					out[idx].TimeoutMS = *patch.TimeoutMS
				}
				if patch.Confirm != nil {
					out[idx].Confirm = *patch.Confirm
				}
			}
			continue
		}
		if head.Builtin {
			continue // 旧版本内置技能已被移除：丢弃，不借文件复活
		}
		var custom yyGameSkill
		if json.Unmarshal(item, &custom) != nil {
			continue
		}
		custom.Name = name
		custom.Builtin = false
		out = append(out, custom)
	}
	return out
}

// yyGameSkillSaveLocked 落盘（调用方已持锁）。内置项只写可改字段，
// 避免旧文件在新版本里固化命令模板；自定义项写完整定义。先写 .tmp 再 Rename，权限 0600。
func yyGameSkillSaveLocked() error {
	if yyGameSkillStore.Path == "" {
		return nil
	}
	items := make([]any, 0, len(yyGameSkillStore.Items))
	for _, s := range yyGameSkillStore.Items {
		if s.Builtin {
			items = append(items, map[string]any{
				"name": s.Name, "builtin": true, "enabled": s.Enabled,
				"min_role": s.MinRole, "timeout_ms": s.TimeoutMS, "confirm": s.Confirm,
			})
			continue
		}
		items = append(items, s)
	}
	raw, e := json.MarshalIndent(map[string]any{"skills": items}, "", "  ")
	if e != nil {
		return e
	}
	tmp := yyGameSkillStore.Path + ".tmp"
	if e = os.WriteFile(tmp, raw, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, yyGameSkillStore.Path)
}

func yyGameSkillDiskErr() string {
	yyGameSkillStore.Lock()
	defer yyGameSkillStore.Unlock()
	return yyGameSkillStore.DiskErr
}

// yyGameSkillLookup 按名取技能（返回副本）。
func yyGameSkillLookup(name string) (yyGameSkill, bool) {
	yyGameSkillStore.Lock()
	defer yyGameSkillStore.Unlock()
	name = strings.TrimSpace(name)
	for _, s := range yyGameSkillStore.Items {
		if s.Name == name {
			return s, true
		}
	}
	return yyGameSkill{}, false
}

// yyGameSkillByName 取内置表里的定义（保存校验用）。
func yyGameSkillByName(name string) (yyGameSkill, bool) {
	for _, s := range yyGameSkillBuiltins() {
		if s.Name == name {
			return s, true
		}
	}
	return yyGameSkill{}, false
}

// yyGameSkillAll 返回全部技能副本（面板列表用）。
func yyGameSkillAll() []yyGameSkill {
	yyGameSkillStore.Lock()
	defer yyGameSkillStore.Unlock()
	return append([]yyGameSkill(nil), yyGameSkillStore.Items...)
}

// yyGameSkillEnabled 返回启用中的技能副本（AI 工具枚举用）。
func yyGameSkillEnabled() []yyGameSkill {
	yyGameSkillStore.Lock()
	defer yyGameSkillStore.Unlock()
	out := make([]yyGameSkill, 0, len(yyGameSkillStore.Items))
	for _, s := range yyGameSkillStore.Items {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out
}

// yyGameSkillNames 按权限等级返回可用技能名（0 成员 1 管理员 2 超管）。
func yyGameSkillNames(level int) []string {
	yyGameSkillStore.Lock()
	defer yyGameSkillStore.Unlock()
	names := make([]string, 0, len(yyGameSkillStore.Items))
	for _, s := range yyGameSkillStore.Items {
		if !s.Enabled || level < s.MinRole {
			continue
		}
		names = append(names, s.Name)
	}
	return names
}

// yySkillRoleName 权限等级的中文名（0 成员 1 管理员 2 超级管理员）。
func yySkillRoleName(level int) string {
	switch level {
	case 1:
		return "管理员"
	case 2:
		return "超级管理员"
	default:
		return "成员"
	}
}

// yyGameSkillDelete 删除一个自定义技能；内置技能不可删除（只能停用）。
func yyGameSkillDelete(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("缺少技能名")
	}
	yyGameSkillStore.Lock()
	defer yyGameSkillStore.Unlock()
	for i, s := range yyGameSkillStore.Items {
		if s.Name != name {
			continue
		}
		if s.Builtin {
			return fmt.Errorf("内置技能不可删除，只能停用")
		}
		yyGameSkillStore.Items = append(yyGameSkillStore.Items[:i], yyGameSkillStore.Items[i+1:]...)
		return yyGameSkillSaveLocked()
	}
	return fmt.Errorf("技能不存在：%s", name)
}

// yyGameSkillSaveAll 整体保存：内置技能必须全部在列（不可删除）、不可改名，
// 只接受 Enabled/MinRole/TimeoutMS/Confirm；自定义技能需通过完整校验。
// 任一被拒则整体不保存，内存注册表保持原状。
func yyGameSkillSaveAll(entries []yySkillEntry) error {
	if len(entries) > yySkillMaxCount {
		return fmt.Errorf("技能最多 %d 条", yySkillMaxCount)
	}
	seen := make(map[string]bool, len(entries))
	next := make([]yyGameSkill, 0, len(entries))
	for _, entry := range entries {
		s := entry.Skill
		s.Name = strings.TrimSpace(s.Name)
		if !yySkillNameRe.MatchString(s.Name) {
			return fmt.Errorf("技能名不合法（小写字母/数字/下划线，3-32位）：%q", s.Name)
		}
		if seen[s.Name] {
			return fmt.Errorf("技能名重复：%s", s.Name)
		}
		seen[s.Name] = true
		if base, ok := yyGameSkillByName(s.Name); ok {
			// 内置技能：定义以代码为准，只应用面板显式提交的可改字段。
			if entry.Patch.Enabled != nil {
				base.Enabled = *entry.Patch.Enabled
			}
			if entry.Patch.MinRole != nil {
				base.MinRole = *entry.Patch.MinRole
			}
			if entry.Patch.TimeoutMS != nil {
				base.TimeoutMS = *entry.Patch.TimeoutMS
			}
			if entry.Patch.Confirm != nil {
				base.Confirm = *entry.Patch.Confirm
			}
			next = append(next, base)
			continue
		}
		normalized, e := yySkillValidateCustom(s, entry.Patch)
		if e != nil {
			return e
		}
		next = append(next, normalized)
	}
	var missing []string
	for _, b := range yyGameSkillBuiltins() {
		if !seen[b.Name] {
			missing = append(missing, b.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("内置技能不可删除：%s", strings.Join(missing, "、"))
	}
	yyGameSkillStore.Lock()
	defer yyGameSkillStore.Unlock()
	yyGameSkillStore.Items = next
	return yyGameSkillSaveLocked()
}

// yySkillValidateCustom 自定义技能完整校验，通过时返回补全默认值后的技能。
func yySkillValidateCustom(s yyGameSkill, patch yySkillPatch) (yyGameSkill, error) {
	s.Builtin = false
	s.Enabled = true
	if patch.Enabled != nil {
		s.Enabled = *patch.Enabled
	}
	if patch.MinRole != nil {
		s.MinRole = *patch.MinRole
	}
	if patch.TimeoutMS != nil {
		s.TimeoutMS = *patch.TimeoutMS
	}
	if patch.Confirm != nil {
		s.Confirm = *patch.Confirm
	}
	if strings.TrimSpace(s.Label) == "" {
		return s, fmt.Errorf("技能 %s 缺少显示名", s.Name)
	}
	if len([]rune(s.Label)) > 40 {
		return s, fmt.Errorf("技能 %s 的显示名最长40字", s.Name)
	}
	if len([]rune(s.Desc)) > 200 {
		return s, fmt.Errorf("技能 %s 的说明最长200字", s.Name)
	}
	if s.MinRole < 0 || s.MinRole > 2 {
		return s, fmt.Errorf("技能 %s 的权限只能是 0/1/2", s.Name)
	}
	if s.TimeoutMS == 0 {
		s.TimeoutMS = 3000
	}
	if s.TimeoutMS < 100 || s.TimeoutMS > 30000 {
		return s, fmt.Errorf("技能 %s 的超时需在 100-30000 毫秒", s.Name)
	}
	if s.Channel == "" {
		s.Channel = "ai"
	}
	if s.Channel != "ai" && s.Channel != "console" && s.Channel != "player" {
		return s, fmt.Errorf("技能 %s 的通道只能是 ai/console/player", s.Name)
	}
	if s.Format == "" {
		s.Format = "text"
	}
	switch s.Format {
	case "text", "list", "json", "count":
	default:
		return s, fmt.Errorf("技能 %s 的格式只能是 text/list/json/count", s.Name)
	}
	switch s.Run {
	case "console", "api":
		if strings.TrimSpace(s.Command) == "" {
			return s, fmt.Errorf("技能 %s 缺少命令模板", s.Name)
		}
		if e := yySkillGuardCommand(s.Command); e != nil {
			return s, fmt.Errorf("技能 %s 的命令模板不合法：%v", s.Name, e)
		}
		declared := make(map[string]bool, len(s.Params))
		for _, p := range s.Params {
			declared[p.Name] = true
		}
		for _, ph := range yySkillPlaceholders(s.Command) {
			if !declared[ph] {
				return s, fmt.Errorf("技能 %s 的模板占位符 {%s} 没有对应参数", s.Name, ph)
			}
		}
	case "builtin":
		return s, fmt.Errorf("builtin 执行方式仅限内置技能")
	default:
		return s, fmt.Errorf("技能 %s 的执行方式只支持 console/api", s.Name)
	}
	names := make(map[string]bool, len(s.Params))
	for _, p := range s.Params {
		if !yySkillParamNameRe.MatchString(p.Name) {
			return s, fmt.Errorf("参数名不合法（小写字母/数字/下划线）：%q", p.Name)
		}
		if names[p.Name] {
			return s, fmt.Errorf("技能 %s 的参数名重复：%s", s.Name, p.Name)
		}
		names[p.Name] = true
		if len([]rune(p.Label)) > 40 {
			return s, fmt.Errorf("参数 %s 的显示名最长40字", p.Name)
		}
		switch p.Type {
		case "string", "int", "player", "enum":
		default:
			return s, fmt.Errorf("参数 %s 的类型只支持 string/int/player/enum", p.Name)
		}
		if p.Type == "enum" {
			if len(p.Options) == 0 {
				return s, fmt.Errorf("枚举参数 %s 缺少选项", p.Name)
			}
			for _, opt := range p.Options {
				if opt == "" || strings.ContainsAny(opt, " \t\r\n") {
					return s, fmt.Errorf("枚举参数 %s 的选项不能为空或含空白", p.Name)
				}
			}
		}
		if p.Default != "" {
			if yySkillHasControl(p.Default) {
				return s, fmt.Errorf("参数 %s 的默认值不能含控制字符", p.Name)
			}
			if len([]rune(p.Default)) > 240 {
				return s, fmt.Errorf("参数 %s 的默认值最长240字", p.Name)
			}
		}
	}
	return s, nil
}

// ---- 确定性执行 ----

// yyGameSkillRun 按名执行一个已注册技能：取技能（不存在/已停用报错）→ 参数校验 →
// 模板渲染 → 防注入 → 经执行通道下发（可注入，测试替换）→ 超时控制 →
// 按 Format 格式化 → 留痕。AI、群指令、面板手动测试都走这一条路。
// 本函数不区分调用者权限（等同超级管理员直调）：AI 入口应在调用前自行定级，
// 或改用带等级校验的 yyGameSkillRunAs。
func yyGameSkillRun(name string, params map[string]string) (string, error) {
	return yyGameSkillRunAs(name, params, 2)
}

// yyGameSkillRunAs 带调用者权限等级的执行入口：level < MinRole 直接拒绝并留痕。
// 群指令、面板等知道调用者身份的路径用它；AI 侧也应优先用它兜底
// （yySkillToolDefsLevel 只负责让低权限用户看不到高权限工具，展示过滤不等于执行拦截）。
func yyGameSkillRunAs(name string, params map[string]string, level int) (string, error) {
	skill, ok := yyGameSkillLookup(name)
	if !ok {
		return "", fmt.Errorf("技能不存在：%s", name)
	}
	if !skill.Enabled {
		return "", fmt.Errorf("技能「%s」已停用", skill.Label)
	}
	if level < skill.MinRole {
		e := fmt.Errorf("权限不足：技能「%s」需要%s权限", skill.Label, yySkillRoleName(skill.MinRole))
		yySkillLog(skill, params, false, 0, e.Error())
		return "", e
	}
	return yyGameSkillRunChecked(skill, params)
}

// yyGameSkillRunChecked 已取到技能后的执行管线（调用方已完成存在/停用/权限检查）。
func yyGameSkillRunChecked(skill yyGameSkill, params map[string]string) (string, error) {
	values, e := yySkillValidateParams(skill, params)
	if e != nil {
		// 校验被拒也留痕：面板与排查看得到「谁在尝试什么」，不只是成功记录。
		yySkillLog(skill, params, false, 0, e.Error())
		return "", e
	}
	command := yySkillRender(skill.Command, values)
	if skill.Run != "builtin" {
		if e := yySkillGuardCommand(command); e != nil {
			yySkillLog(skill, params, false, 0, e.Error())
			return "", e
		}
	}
	timeout := time.Duration(skill.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	started := time.Now()
	out, e := yySkillExecute(skill, command, values, timeout)
	ms := time.Since(started).Milliseconds()
	if e != nil {
		yySkillLog(skill, params, false, ms, e.Error())
		return "", e
	}
	text := yySkillFormat(skill, out)
	yySkillLog(skill, params, true, ms, text)
	return text, nil
}

// yySkillExecute 按 Run 分派：builtin 走 Go 函数；console/api 走可注入执行通道，
// 并统一套上 TimeoutMS 超时看护——默认控制台与测试替身受同一条约束。
// 超时只报告「结果未知，未重发」，绝不自动重试（tp/reload 不可逆）。
func yySkillExecute(s yyGameSkill, command string, values map[string]string, timeout time.Duration) (string, error) {
	if s.Run == "builtin" {
		return yySkillBuiltinRun(s, values)
	}
	exec := yySkillExecutor
	if exec == nil {
		exec = yySkillConsoleSend
	}
	type outcome struct {
		text string
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		text, err := exec(command, timeout)
		done <- outcome{text, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.text, r.err
	case <-timer.C:
		return "", fmt.Errorf("命令已提交但服务器确认超时，未重发")
	}
}

// yySkillConsoleSend 默认执行通道：经 youyuan_qq.go 现有的控制台发送函数
// botMgr.SendConsole 下发（内部走 WS 自动化通道并等待回执）。
// 超时看护在外层 yySkillExecute，这里只负责连接检查与输出提取。
func yySkillConsoleSend(command string, timeout time.Duration) (string, error) {
	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()
	if bm == nil || !bm.IsConnected() {
		return "", fmt.Errorf("游戏尚未连接，指令未执行")
	}
	raw, e := bm.SendConsole(command)
	if e != nil {
		return "", e
	}
	return yySkillConsoleText(raw), nil
}

// yySkillConsoleText 从控制台回执里提取可读输出文本。
func yySkillConsoleText(raw map[string]any) string {
	if raw == nil {
		return "命令已执行"
	}
	if lines, ok := raw["output"].([]string); ok && len(lines) > 0 {
		return strings.Join(lines, "\n")
	}
	if note, ok := raw["note"].(string); ok && note != "" {
		return note
	}
	if ok, _ := raw["ok"].(bool); ok {
		return "命令已执行，无输出"
	}
	return mustJSON(raw)
}

// yySkillBuiltinRun 内置技能的 Go 实现（不经控制台）。
func yySkillBuiltinRun(s yyGameSkill, values map[string]string) (string, error) {
	switch s.Name {
	case "online_list":
		names, e := yySkillOnlinePlayers()
		if e != nil {
			return "", e
		}
		if len(names) == 0 {
			return "当前没有玩家在线", nil
		}
		const per = 12 // 每页12人
		total := len(names)
		pages := (total + per - 1) / per
		page := 1
		if raw := strings.TrimSpace(values["page"]); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				page = n
			}
		}
		if page < 1 {
			page = 1
		}
		if page > pages {
			page = pages
		}
		start := (page - 1) * per
		end := start + per
		if end > total {
			end = total
		}
		head := "当前在线 " + strconv.Itoa(total) + " 人（第 " + strconv.Itoa(page) + "/" + strconv.Itoa(pages) + " 页）"
		return head + "\n" + strings.Join(names[start:end], "\n"), nil
	case "server_status":
		game := "未连接"
		if bm := getBotManagerLocked(); bm != nil && bm.IsConnected() {
			game = "已连接"
		}
		return "游戏连接：" + game + "\nQQ桥接状态：" + yyQQStateNow(), nil
	case "player_info":
		return yyQQPlayerSummary(values["player"]), nil
	}
	return "", fmt.Errorf("内置技能未实现：%s", s.Name)
}

// yySkillValidateParams 逐个校验参数：必填、类型、player 名合法 [A-Za-z0-9_]{3,16}、enum 取值。
// 任一失败即报错，不执行。返回用于渲染的值表。
func yySkillValidateParams(s yyGameSkill, params map[string]string) (map[string]string, error) {
	values := make(map[string]string, len(s.Params))
	for _, p := range s.Params {
		raw, exists := params[p.Name]
		raw = strings.TrimSpace(raw)
		if !exists || raw == "" {
			if p.Default != "" {
				raw = strings.TrimSpace(p.Default)
			} else if p.Required {
				return nil, fmt.Errorf("缺少参数 %s（%s）", p.Name, yySkillParamLabel(p))
			} else {
				continue
			}
		}
		switch p.Type {
		case "player":
			if !yySkillPlayerName.MatchString(raw) {
				return nil, fmt.Errorf("参数 %s 需要合法游戏ID：3-16位字母、数字或下划线", p.Name)
			}
		case "int":
			if _, e := strconv.Atoi(raw); e != nil {
				return nil, fmt.Errorf("参数 %s 需要整数", p.Name)
			}
		case "enum":
			if len(p.Options) == 0 {
				return nil, fmt.Errorf("参数 %s 的枚举选项为空，技能不可用", p.Name)
			}
			matched := false
			for _, opt := range p.Options {
				if opt == raw {
					matched = true
					break
				}
			}
			if !matched {
				return nil, fmt.Errorf("参数 %s 只能是：%s", p.Name, strings.Join(p.Options, "、"))
			}
		case "string", "":
			if yySkillHasControl(raw) {
				return nil, fmt.Errorf("参数 %s 不能含换行或控制字符", p.Name)
			}
			if len([]rune(raw)) > 240 {
				return nil, fmt.Errorf("参数 %s 最长240字", p.Name)
			}
		default:
			return nil, fmt.Errorf("参数 %s 的类型 %s 不支持", p.Name, p.Type)
		}
		values[p.Name] = raw
	}
	return values, nil
}

func yySkillParamLabel(p yyGameSkillParam) string {
	if strings.TrimSpace(p.Label) != "" {
		return p.Label
	}
	return p.Name
}

// yySkillHasControl 是否含控制字符（含换行、回车、NUL、DEL）。
func yySkillHasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// yySkillRender 展开命令模板占位符 {name}；未知占位符原样保留，再由防注入校验兜底。
func yySkillRender(tpl string, values map[string]string) string {
	if tpl == "" || len(values) == 0 {
		return tpl
	}
	pairs := make([]string, 0, len(values)*2)
	for k, v := range values {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(tpl)
}

// yySkillGuardCommand 防注入：拒绝空命令、换行、回车与一切控制字符，
// 保证一条技能最终只发一条命令，参数里的 \n \r 无法拼接出第二条指令。
func yySkillGuardCommand(command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("命令为空，已拒绝")
	}
	if strings.ContainsAny(command, "\r\n") {
		return fmt.Errorf("命令含换行，已拒绝（防注入）")
	}
	if yySkillHasControl(command) {
		return fmt.Errorf("命令含控制字符，已拒绝（防注入）")
	}
	return nil
}

// yySkillPlaceholders 提取模板里的占位符名（去重，保持出现顺序）。
func yySkillPlaceholders(tpl string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range yySkillPlaceholder.FindAllStringSubmatch(tpl, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// yySkillFormat 按 Format 修剪执行结果。count 只回输出里的第一段整数（如记分板数量），
// 取不到就原样返回；其余格式统一截断到 3200 字（对齐 yyQQParts 的分片大小）。
func yySkillFormat(s yyGameSkill, out string) string {
	if s.Format == "count" {
		if m := yySkillFirstInt.FindString(out); m != "" {
			return m
		}
	}
	return yyQQClip(out, 3200)
}

// ---- 留痕（内存环形 200 条） ----

func yySkillLog(s yyGameSkill, params map[string]string, ok bool, ms int64, out string) {
	copied := make(map[string]string, len(params))
	for k, v := range params {
		copied[k] = v
	}
	entry := yySkillLogEntry{Time: time.Now(), Skill: s.Name, Params: copied, OK: ok, Ms: ms, Out: yyQQClip(out, 500)}
	yySkillLogs.Lock()
	yySkillLogs.Items = append([]yySkillLogEntry{entry}, yySkillLogs.Items...)
	if len(yySkillLogs.Items) > yySkillLogKeep {
		yySkillLogs.Items = yySkillLogs.Items[:yySkillLogKeep]
	}
	yySkillLogs.Unlock()
	if ok {
		yyQQLog("游戏技能 " + s.Name + " 执行成功（" + strconv.FormatInt(ms, 10) + "ms）")
	} else {
		yyQQLog("游戏技能 " + s.Name + " 执行失败：" + out)
	}
}

// yySkillLogView 取最近 limit 条留痕（新在前）；limit<=0 或超出长度时取全部。
func yySkillLogView(limit int) []yySkillLogEntry {
	yySkillLogs.Lock()
	defer yySkillLogs.Unlock()
	if limit <= 0 || limit > len(yySkillLogs.Items) {
		limit = len(yySkillLogs.Items)
	}
	return append([]yySkillLogEntry(nil), yySkillLogs.Items[:limit]...)
}

// ---- AI 侧唯一入口 ----

// yySkillToolDefs 把启用技能枚举成 function-calling 工具定义（OpenAI tools 线格式，
// 与 youyuan_qq_ai.go 请求体里 tools 数组的形状一致：{"type":"function","function":{...}}）。
// 每个启用技能一个工具，参数 schema 由注册表生成——AI 只能调这些工具，不能生成原始命令。
func yySkillToolDefs() []map[string]any {
	skills := yyGameSkillEnabled()
	defs := make([]map[string]any, 0, len(skills))
	for _, s := range skills {
		properties := make(map[string]any, len(s.Params))
		required := make([]string, 0, len(s.Params))
		for _, p := range s.Params {
			properties[p.Name] = yySkillParamSchema(p)
			if p.Required {
				required = append(required, p.Name)
			}
		}
		desc := s.Label + "：" + s.Desc + "。只能使用本工具执行，不要自行编写原始游戏命令。"
		if s.Confirm {
			desc += "执行前需先向用户确认。"
		}
		defs = append(defs, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        yySkillToolPrefix + s.Name,
				"description": desc,
				"parameters": map[string]any{
					"type":       "object",
					"properties": properties,
					"required":   required,
				},
			},
		})
	}
	return defs
}

// yySkillParamSchema 由参数定义生成 JSON Schema 片段。
func yySkillParamSchema(p yyGameSkillParam) map[string]any {
	schema := map[string]any{"type": "string", "description": yySkillParamLabel(p)}
	switch p.Type {
	case "int":
		schema["type"] = "integer"
	case "player":
		schema["description"] = yySkillParamLabel(p) + "（3-16位字母、数字或下划线）"
	case "enum":
		schema["enum"] = append([]string(nil), p.Options...)
	}
	if p.Default != "" {
		schema["default"] = p.Default
	}
	return schema
}

// yySkillInvoke AI 工具调用入口：把模型传来的 args 规整成字符串参数表，
// 内部走 yyGameSkillRun——校验、防注入、留痕与其它入口完全一致，AI 无法绕过注册表。
func yySkillInvoke(name string, args map[string]any) (string, error) {
	skill := strings.TrimPrefix(strings.TrimSpace(name), yySkillToolPrefix)
	params := make(map[string]string, len(args))
	for k, v := range args {
		switch t := v.(type) {
		case string:
			params[k] = t
		case float64:
			params[k] = strconv.FormatFloat(t, 'f', -1, 64)
		case bool:
			params[k] = strconv.FormatBool(t)
		case nil:
			params[k] = ""
		default:
			raw, _ := json.Marshal(v)
			params[k] = string(raw)
		}
	}
	return yyGameSkillRun(skill, params)
}

// ---- HTTP 面板接口 ----

// yyQQSkillRoutes 技能注册表面板接口，风格与 yyQQTriggerRoutes 一致：
// GET 列表（含参数类型元数据）、POST 整体保存、DELETE 删除自定义技能、
// POST /run 手动测试执行、GET /log 执行留痕。
func yyQQSkillRoutes(mux *http.ServeMux) {
	// 数据目录从 yyQQPath 推导（yyRegisterQQ 先设置 yyQQPath 再注册路由）；
	// 未初始化时（测试或独立调用）静默跳过，注册表仍以内置技能工作。
	if strings.TrimSpace(yyGameSkillStore.Path) == "" && strings.TrimSpace(yyQQPath) != "" {
		yyGameSkillInit(filepath.Dir(yyQQPath))
	}
	mux.HandleFunc("/api/qq/skills", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]any{
				"ok":       true,
				"skills":   yyGameSkillAll(),
				"builtin":  "内置技能不可删除、不可改名，只能停用或调整权限与超时",
				"runs":     []string{"console", "api", "builtin"},
				"formats":  []string{"text", "list", "json", "count"},
				"types":    []string{"string", "int", "player", "enum"},
				"channels": []string{"ai", "console", "player"},
				"roles":    []string{"成员", "管理员", "超级管理员"},
				"disk_err": yyGameSkillDiskErr(),
			})
		case http.MethodPost:
			var payload struct {
				Skills []json.RawMessage `json:"skills"`
			}
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512*1024)).Decode(&payload); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
				return
			}
			entries := make([]yySkillEntry, 0, len(payload.Skills))
			for _, raw := range payload.Skills {
				var entry yySkillEntry
				if e := json.Unmarshal(raw, &entry.Skill); e != nil {
					writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
					return
				}
				if e := json.Unmarshal(raw, &entry.Patch); e != nil {
					writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
					return
				}
				entries = append(entries, entry)
			}
			if e := yyGameSkillSaveAll(entries); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "skills": yyGameSkillAll()})
		case http.MethodDelete:
			if e := yyGameSkillDelete(r.URL.Query().Get("name")); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "skills": yyGameSkillAll()})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
		}
	})
	mux.HandleFunc("/api/qq/skills/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var p struct {
			Name   string            `json:"name"`
			Params map[string]string `json:"params"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		out, e := yyGameSkillRun(p.Name, p.Params)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "output": out})
	})
	mux.HandleFunc("/api/qq/skills/log", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, e := strconv.Atoi(v); e == nil && n > 0 {
				limit = n
			}
		}
		if limit > yySkillLogKeep {
			limit = yySkillLogKeep
		}
		writeJSON(w, map[string]any{"ok": true, "entries": yySkillLogView(limit)})
	})
}

// yySkillToolDefsLevel 按权限等级过滤后的工具定义（0 成员 1 管理员 2 超级管理员）。
// 权限差异体现在 schema 里：低权限用户看不到高权限工具，就不会尝试调用
// （对齐设计文档 §5.3）；AI 接线优先用本函数。执行侧仍由 yyGameSkillRun 兜底校验。
func yySkillToolDefsLevel(level int) []map[string]any {
	all := yySkillToolDefs()
	if level >= 2 {
		return all
	}
	out := make([]map[string]any, 0, len(all))
	for _, d := range all {
		fn, ok := d["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		s, ok := yyGameSkillLookup(strings.TrimPrefix(name, yySkillToolPrefix))
		if !ok || level < s.MinRole {
			continue
		}
		out = append(out, d)
	}
	return out
}
