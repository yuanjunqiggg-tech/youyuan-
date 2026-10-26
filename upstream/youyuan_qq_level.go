package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// QQ 群活跃 → 经验 → 等级 → 局内游戏等级显示（Minecraft 网易服，经控制台指令）。
//
// 设计要点：
//   - 经验引擎是纯本地计算（升级曲线可配置），群消息单入口 yyLevelOnMessage；
//   - 状态持久化在 qq-levels.json（照 youyuan_qq_memory.go 范式：tmp+rename 原子写、
//     坏文件不崩、体积上限）；
//   - 局内显示走可注入的执行通道 yyLevelExec（默认复用现有 botMgr.SendConsole），
//     测试替换为假执行器，严禁真连游戏；
//   - 官方 QQ 适配器的 user 是 openid 不是 QQ 号，无法参与 map[int64]string 绑定表，
//     因此等级只按 OneBot 的数字 QQ 累计（其余用户静默跳过）。

// yyLevelConfig 等级与经验配置（持久化于 qq-levels.json，随 /api/qq/level 维护）。
type yyLevelConfig struct {
	Enabled         bool             `json:"enabled"`               // 总开关
	XPChat          int              `json:"xp_chat"`               // 每条群消息加的经验
	ChatCooldownSec int              `json:"chat_cooldown_sec"`     // 同群同 QQ 冷却秒数
	LevelBase       int              `json:"level_base"`            // 升级曲线基数：L→L+1 需 LevelBase*LevelFactor^(L-1)
	LevelFactor     float64          `json:"level_factor"`          // 升级曲线倍率（可小数）
	MaxLevel        int              `json:"max_level"`             // 等级封顶
	Announce        bool             `json:"announce"`              // 升级时群内播报
	SyncGame        bool             `json:"sync_game"`             // 变动时同步局内记分板显示
	PlayerBind      map[int64]string `json:"player_bind,omitempty"` // QQ → 玩家名绑定表（局内显示用）
}

type yyLevelPlayer struct {
	Group string    `json:"group"`
	QQ    int64     `json:"qq"`
	XP    int       `json:"xp"`
	Name  string    `json:"name,omitempty"` // 最近一次群昵称，供排行榜展示
	At    time.Time `json:"at"`
}

type yyLevelPending struct {
	Group string `json:"group"`
	QQ    int64  `json:"qq"`
}

type yyLevelRow struct {
	QQ    int64  `json:"qq"`
	Name  string `json:"name,omitempty"`
	Level int    `json:"level"`
	XP    int    `json:"xp"`
}

var yyLevel = struct {
	sync.Mutex
	Path           string
	Config         yyLevelConfig
	Players        map[string]yyLevelPlayer
	Pending        []yyLevelPending
	ObjectiveAdded bool
	DiskError      string
}{Players: map[string]yyLevelPlayer{}}

const (
	yyLevelObjective  = "qqlevel" // 局内记分板名
	yyLevelPlayersMax = 20000     // 玩家记录上限
	yyLevelPendingMax = 500       // 待补同步队列上限
	yyLevelXPMax      = 1 << 40   // 单玩家经验上限（防溢出）
	yyLevelIndexMax   = 8 << 20   // qq-levels.json 体积上限
)

func yyLevelDefaultConfig() yyLevelConfig {
	return yyLevelConfig{
		XPChat: 5, ChatCooldownSec: 60,
		LevelBase: 100, LevelFactor: 1.5, MaxLevel: 100,
		Announce: true, SyncGame: true,
		PlayerBind: map[int64]string{},
	}
}

// yyLevelNormalize 收敛配置：非法值回落默认，保证曲线数学永远可算。
func yyLevelNormalize(c *yyLevelConfig) {
	if c.XPChat < 0 {
		c.XPChat = 0
	}
	if c.XPChat > 1000 {
		c.XPChat = 1000
	}
	if c.ChatCooldownSec < 0 {
		c.ChatCooldownSec = 0
	}
	if c.ChatCooldownSec > 86400 {
		c.ChatCooldownSec = 86400
	}
	if c.LevelBase <= 0 {
		c.LevelBase = 100
	}
	if c.LevelFactor <= 0 || math.IsNaN(c.LevelFactor) || math.IsInf(c.LevelFactor, 0) {
		c.LevelFactor = 1.5
	}
	if c.MaxLevel <= 0 {
		c.MaxLevel = 100
	}
	if c.MaxLevel > 10000 {
		c.MaxLevel = 10000
	}
	if c.PlayerBind == nil {
		c.PlayerBind = map[int64]string{}
	}
}

// yyLevelNow 冷却判定的时钟；测试可注入以冻结/推进时间。
var yyLevelNow = time.Now

// yyLevelCooldown 同群同 QQ 的冷却表（仅内存，不持久化）。
var yyLevelCooldown = struct {
	sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

// yyLevelCooldownAllow 同群同 QQ 在冷却内只放行一次。sec<=0 表示不冷却。
func yyLevelCooldownAllow(group string, qq int64, sec int) bool {
	if sec <= 0 {
		return true
	}
	key := group + "\x00" + strconv.FormatInt(qq, 10)
	now := yyLevelNow()
	yyLevelCooldown.Lock()
	defer yyLevelCooldown.Unlock()
	if at, ok := yyLevelCooldown.last[key]; ok && now.Sub(at) < time.Duration(sec)*time.Second {
		return false
	}
	if len(yyLevelCooldown.last) >= 4096 {
		// 有界：淘汰最旧一条，新条目永不因表满被丢弃。
		oldestKey, oldestAt := "", time.Time{}
		for k, at := range yyLevelCooldown.last {
			if oldestKey == "" || at.Before(oldestAt) {
				oldestKey, oldestAt = k, at
			}
		}
		delete(yyLevelCooldown.last, oldestKey)
	}
	yyLevelCooldown.last[key] = now
	return true
}

// ---------- 升级曲线数学 ----------

// yyLevelNeedOf L→L+1 所需经验：LevelBase*LevelFactor^(L-1)。
func yyLevelNeedOf(cf yyLevelConfig, level int) int {
	if level < 1 {
		level = 1
	}
	need := float64(cf.LevelBase) * math.Pow(cf.LevelFactor, float64(level-1))
	if math.IsNaN(need) || need < 0 {
		return 0
	}
	if need > float64(yyLevelXPMax) {
		return yyLevelXPMax
	}
	return int(math.Round(need))
}

func yyLevelMaxOf(cf yyLevelConfig) int {
	if cf.MaxLevel < 1 {
		return 1
	}
	return cf.MaxLevel
}

// yyLevelProgressOf 由总经验推 (等级, 当前级经验, 升级所需, 百分比)。
// 满级后不再累计级内经验（cur/need 归零、pct=100）。
func yyLevelProgressOf(cf yyLevelConfig, xp int) (level, cur, need int, pct float64) {
	if xp < 0 {
		xp = 0
	}
	level, cur = 1, xp
	max := yyLevelMaxOf(cf)
	for level < max {
		need := yyLevelNeedOf(cf, level)
		if cur < need {
			break
		}
		cur -= need
		level++
	}
	if level >= max {
		return level, 0, 0, 100
	}
	need = yyLevelNeedOf(cf, level)
	if need > 0 {
		pct = float64(cur) * 100 / float64(need)
	}
	return level, cur, need, pct
}

// yyLevelProgress 按当前配置计算经验进度。
func yyLevelProgress(xp int) (level, cur, need int, pct float64) {
	return yyLevelProgressOf(yyLevelConfigSnapshot(), xp)
}

func yyLevelLevelOf(cf yyLevelConfig, xp int) int {
	level, _, _, _ := yyLevelProgressOf(cf, xp)
	return level
}

// ---------- 持久化（照 youyuan_qq_memory.go 范式） ----------

func yyLevelConfigSnapshot() yyLevelConfig {
	yyLevel.Lock()
	defer yyLevel.Unlock()
	cf := yyLevel.Config
	bind := make(map[int64]string, len(cf.PlayerBind))
	for k, v := range cf.PlayerBind {
		bind[k] = v
	}
	cf.PlayerBind = bind
	return cf
}

func yyLevelKey(group string, qq int64) string {
	return group + "\x00" + strconv.FormatInt(qq, 10)
}

type yyLevelDisk struct {
	Config         yyLevelConfig            `json:"config"`
	Players        map[string]yyLevelPlayer `json:"players"`
	Pending        []yyLevelPending         `json:"pending,omitempty"`
	ObjectiveAdded bool                     `json:"objective_added"`
}

func yyLevelMarshalLocked(cf yyLevelConfig) ([]byte, error) {
	raw, e := json.Marshal(yyLevelDisk{Config: cf, Players: yyLevel.Players, Pending: yyLevel.Pending, ObjectiveAdded: yyLevel.ObjectiveAdded})
	if e == nil && len(raw) > yyLevelIndexMax {
		e = fmt.Errorf("level_index_size_limit")
	}
	return raw, e
}

func yyLevelWriteLocked(raw []byte) error {
	if yyLevel.Path == "" {
		return nil
	}
	if e := os.WriteFile(yyLevel.Path+".tmp", raw, 0600); e != nil {
		return e
	}
	return os.Rename(yyLevel.Path+".tmp", yyLevel.Path)
}

func yyLevelSaveLocked() {
	if yyLevel.Path == "" {
		return
	}
	raw, e := yyLevelMarshalLocked(yyLevel.Config)
	if e == nil {
		e = yyLevelWriteLocked(raw)
	}
	if e != nil {
		yyLevel.DiskError = "level_persistence_failed"
	} else {
		yyLevel.DiskError = ""
	}
}

// yyLevelInit 载入 qq-levels.json：tmp+rename 原子写、坏文件回落默认不崩。
func yyLevelInit(root string) {
	yyLevel.Lock()
	defer yyLevel.Unlock()
	yyLevel.Path = filepath.Join(root, "qq-levels.json")
	file, e := os.Open(yyLevel.Path)
	if e != nil {
		yyLevel.Config = yyLevelDefaultConfig()
		yyLevelNormalize(&yyLevel.Config)
		return
	}
	defer file.Close()
	var saved yyLevelDisk
	if json.NewDecoder(io.LimitReader(file, yyLevelIndexMax)).Decode(&saved) != nil {
		yyLevel.DiskError = "invalid_or_oversized_level_index"
		yyLevel.Config = yyLevelDefaultConfig()
		yyLevelNormalize(&yyLevel.Config)
		return
	}
	players := map[string]yyLevelPlayer{}
	for _, row := range saved.Players {
		if len(players) >= yyLevelPlayersMax {
			break
		}
		if row.Group == "" || row.QQ == 0 || row.XP < 0 {
			continue
		}
		if row.XP > yyLevelXPMax {
			row.XP = yyLevelXPMax
		}
		if len([]rune(row.Name)) > 64 {
			row.Name = string([]rune(row.Name)[:64])
		}
		// 键一律由 group+QQ 重建，容忍手改文件造成的键错位。
		players[yyLevelKey(row.Group, row.QQ)] = row
	}
	pending := make([]yyLevelPending, 0, len(saved.Pending))
	seen := map[string]bool{}
	for _, item := range saved.Pending {
		if len(pending) >= yyLevelPendingMax || item.Group == "" || item.QQ == 0 {
			continue
		}
		key := yyLevelKey(item.Group, item.QQ)
		if seen[key] {
			continue
		}
		seen[key] = true
		pending = append(pending, item)
	}
	yyLevel.Players = players
	yyLevel.Pending = pending
	yyLevel.ObjectiveAdded = saved.ObjectiveAdded
	yyLevel.Config = saved.Config
	yyLevelNormalize(&yyLevel.Config)
	yyLevelSaveLocked()
}

// yyLevelUpdate 改配置并持久化：先写盘成功才换内存（与 yyQQHelperMutate 一致）。
func yyLevelUpdate(change func(*yyLevelConfig)) error {
	yyLevel.Lock()
	defer yyLevel.Unlock()
	next := yyLevel.Config
	next.PlayerBind = make(map[int64]string, len(yyLevel.Config.PlayerBind))
	for k, v := range yyLevel.Config.PlayerBind {
		next.PlayerBind[k] = v
	}
	change(&next)
	yyLevelNormalize(&next)
	raw, e := yyLevelMarshalLocked(next)
	if e == nil {
		e = yyLevelWriteLocked(raw)
	}
	if e != nil {
		return e
	}
	yyLevel.Config = next
	yyLevel.DiskError = ""
	return nil
}

// ---------- 经验引擎 ----------

// yyLevelAddXP 给 (group, qq) 加 n 点经验，返回是否升级与当前等级。
func yyLevelAddXP(group string, qq int64, n int) (leveledUp bool, level int) {
	if group == "" || qq == 0 || n <= 0 {
		return false, 0
	}
	yyLevel.Lock()
	defer yyLevel.Unlock()
	cf := yyLevel.Config
	key := yyLevelKey(group, qq)
	row := yyLevel.Players[key]
	if row.XP < 0 {
		row.XP = 0
	}
	before := yyLevelLevelOf(cf, row.XP)
	row.XP += n
	if row.XP > yyLevelXPMax {
		row.XP = yyLevelXPMax
	}
	row.Group, row.QQ, row.At = group, qq, yyLevelNow()
	if _, exists := yyLevel.Players[key]; !exists && len(yyLevel.Players) >= yyLevelPlayersMax {
		// 有界：超上限淘汰最旧一条，新玩家永不因表满被丢弃。
		oldestKey, oldestAt := "", time.Time{}
		for k, r := range yyLevel.Players {
			if oldestKey == "" || r.At.Before(oldestAt) {
				oldestKey, oldestAt = k, r.At
			}
		}
		delete(yyLevel.Players, oldestKey)
	}
	yyLevel.Players[key] = row
	yyLevelSaveLocked()
	level = yyLevelLevelOf(cf, row.XP)
	return level > before, level
}

// yyLevelObserveName 记最近群昵称（排行榜展示用），不改变经验。
func yyLevelObserveName(group string, qq int64, name string) {
	name = strings.TrimSpace(yyQQClip(name, 64))
	if group == "" || qq == 0 || name == "" {
		return
	}
	yyLevel.Lock()
	defer yyLevel.Unlock()
	key := yyLevelKey(group, qq)
	row, ok := yyLevel.Players[key]
	if !ok || row.Name == name {
		return
	}
	row.Name = name
	row.Group, row.QQ = group, qq
	yyLevel.Players[key] = row
	yyLevelSaveLocked()
}

func yyLevelXP(group string, qq int64) int {
	yyLevel.Lock()
	defer yyLevel.Unlock()
	return yyLevel.Players[yyLevelKey(group, qq)].XP
}

// yyLevelQuery 返回一行人类可读的等级进度，如「Lv.7｜经验 350/500（还差 150 升级）」。
func yyLevelQuery(group string, qq int64) string {
	cf := yyLevelConfigSnapshot()
	level, cur, need, _ := yyLevelProgressOf(cf, yyLevelXP(group, qq))
	if level >= yyLevelMaxOf(cf) {
		return fmt.Sprintf("Lv.%d｜经验 已满级", level)
	}
	return fmt.Sprintf("Lv.%d｜经验 %d/%d（还差 %d 升级）", level, cur, need, need-cur)
}

// yyLevelTop 排行榜：按总经验降序，同经验按 QQ 升序保证确定。
func yyLevelTop(group string, n int) []yyLevelRow {
	if n <= 0 {
		n = 10
	}
	if n > 100 {
		n = 100
	}
	cf := yyLevelConfigSnapshot()
	yyLevel.Lock()
	rows := make([]yyLevelRow, 0, 16)
	for _, row := range yyLevel.Players {
		if row.Group != group {
			continue
		}
		rows = append(rows, yyLevelRow{QQ: row.QQ, Name: row.Name, Level: yyLevelLevelOf(cf, row.XP), XP: row.XP})
	}
	yyLevel.Unlock()
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].XP != rows[j].XP {
			return rows[i].XP > rows[j].XP
		}
		return rows[i].QQ < rows[j].QQ
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// ---------- 群消息单入口 ----------

// yyLevelAnnounce 群播报通道。默认走现有群发送链路；测试替换，绝不真发 QQ。
var yyLevelAnnounce = func(group, text string) error {
	cf := yyQQSnapshot()
	if !yyQQGroupEnabled(cf, group) {
		return fmt.Errorf("这个群未启用")
	}
	cf.GroupID = group
	return yyQQSendToGroup(cf, text, "")
}

// yyLevelOnMessage 群消息单入口：同群同 QQ 冷却内只加一次经验；
// 升级时返回 (true, 新等级)，并异步播报与同步局内显示（网络 IO 不堵 WS 读循环）。
func yyLevelOnMessage(m *yyQQInbound) (leveledUp bool, level int) {
	if m == nil || m.IsPrivate || m.Group == "" || m.User == "" {
		return false, 0
	}
	cf := yyQQSnapshot()
	if m.User == yyQQBotID(cf) || !yyQQGroupEnabled(cf, m.Group) {
		return false, 0
	}
	lv := yyLevelConfigSnapshot()
	if !lv.Enabled || lv.XPChat <= 0 {
		return false, 0
	}
	qq, e := strconv.ParseInt(strings.TrimSpace(m.User), 10, 64)
	if e != nil || qq == 0 {
		// 官方适配器的 openid 不是 QQ 号，无法参与数字 QQ 绑定表，静默跳过。
		return false, 0
	}
	if !yyLevelCooldownAllow(m.Group, qq, lv.ChatCooldownSec) {
		// 冷却中：不加经验，但仍返回当前等级，调用方可直接展示。
		return false, yyLevelLevelOf(lv, yyLevelXP(m.Group, qq))
	}
	leveledUp, level = yyLevelAddXP(m.Group, qq, lv.XPChat)
	yyLevelObserveName(m.Group, qq, m.Name)
	if !leveledUp {
		return false, level
	}
	group, name := m.Group, m.Name
	go func() {
		if lv.Announce {
			who := strings.TrimSpace(yyQQClip(name, 32))
			if who == "" {
				who = strconv.FormatInt(qq, 10)
			}
			if e := yyLevelAnnounce(group, fmt.Sprintf("恭喜 %s 升到 Lv.%d！", who, level)); e != nil {
				yyQQLog("等级播报发送失败：" + e.Error())
			}
		}
		if lv.SyncGame {
			if e := yyLevelSyncGame(group, qq); e != nil {
				yyQQLog("等级同步局内失败：" + e.Error())
			}
		}
	}()
	return true, level
}

// ---------- 局内等级显示（Minecraft 网易服，经控制台指令） ----------

// yyLevelExec 局内指令执行通道。默认复用现有 botMgr.SendConsole（youyuan_qq_triggers.go
// 同款读法）；测试替换为假执行器，严禁真连游戏。
var yyLevelExec = func(cmd string) error {
	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()
	if bm == nil || !bm.IsConnected() {
		return fmt.Errorf("游戏通道未连接")
	}
	_, e := bm.SendConsole(cmd)
	return e
}

// yyLevelSafePlayer 校验玩家名：拒绝 \n、\r 与控制字符（r<0x20 / 0x7f 已覆盖），
// 也拒绝双引号——记分板指令里玩家名以 JSON 引号形式嵌入，引号会破坏指令结构。
func yyLevelSafePlayer(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 48 {
		return "", false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f || r == 0x22 {
			return "", false
		}
	}
	return name, true
}

// yyLevelCommands 渲染局内指令：首次先建记分板（只发一次），再写玩家等级。
func yyLevelCommands(player string, level int, objectiveAdded bool) ([]string, error) {
	safe, ok := yyLevelSafePlayer(player)
	if !ok {
		return nil, fmt.Errorf("玩家名含非法字符，已拒绝执行")
	}
	quoted, _ := json.Marshal(safe)
	cmds := []string{}
	if !objectiveAdded {
		cmds = append(cmds, "scoreboard objectives add "+yyLevelObjective+" dummy 等级")
	}
	cmds = append(cmds, fmt.Sprintf("scoreboard players set %s %s %d", quoted, yyLevelObjective, level))
	return cmds, nil
}

func yyLevelObjectiveSent() bool {
	yyLevel.Lock()
	defer yyLevel.Unlock()
	return yyLevel.ObjectiveAdded
}

func yyLevelMarkObjectiveSent() {
	yyLevel.Lock()
	defer yyLevel.Unlock()
	if yyLevel.ObjectiveAdded {
		return
	}
	yyLevel.ObjectiveAdded = true
	yyLevelSaveLocked()
}

func yyLevelEnqueuePending(group string, qq int64) {
	if group == "" || qq == 0 {
		return
	}
	yyLevel.Lock()
	defer yyLevel.Unlock()
	key := yyLevelKey(group, qq)
	for _, item := range yyLevel.Pending {
		if yyLevelKey(item.Group, item.QQ) == key {
			return // 去重
		}
	}
	if len(yyLevel.Pending) >= yyLevelPendingMax {
		yyLevel.Pending = yyLevel.Pending[1:] // 丢最旧，新待同步永不因队满被丢弃
	}
	yyLevel.Pending = append(yyLevel.Pending, yyLevelPending{Group: group, QQ: qq})
	yyLevelSaveLocked()
}

func yyLevelDropPending(group string, qq int64) {
	yyLevel.Lock()
	defer yyLevel.Unlock()
	key := yyLevelKey(group, qq)
	kept := yyLevel.Pending[:0]
	for _, item := range yyLevel.Pending {
		if yyLevelKey(item.Group, item.QQ) != key {
			kept = append(kept, item)
		}
	}
	yyLevel.Pending = kept
	yyLevelSaveLocked()
}

// yyLevelSyncGame 把 QQ 的等级写进局内记分板（scoreboard players set）。
// 无绑定则跳过并入待同步队列；执行失败也入队，等重连后 yyLevelFlushPending 补同步。
func yyLevelSyncGame(group string, qq int64) error {
	cf := yyLevelConfigSnapshot()
	if !cf.SyncGame || group == "" || qq == 0 {
		return nil
	}
	raw, bound := cf.PlayerBind[qq]
	if !bound {
		yyLevelEnqueuePending(group, qq) // 无绑定：跳过并入队，等绑定后补同步
		return nil
	}
	level, _, _, _ := yyLevelProgressOf(cf, yyLevelXP(group, qq))
	objectiveAdded := yyLevelObjectiveSent()
	cmds, e := yyLevelCommands(strings.TrimSpace(raw), level, objectiveAdded)
	if e != nil {
		return e // 绑定值非法（空白/注入字符）：拒绝执行， surfaced 给配置方
	}
	for i, cmd := range cmds {
		if e := yyLevelExec(cmd); e != nil {
			yyLevelEnqueuePending(group, qq)
			return e
		}
		if i == 0 && !objectiveAdded {
			// 记分板已建成功才记状态位：只发一次，重启后也不重发。
			yyLevelMarkObjectiveSent()
		}
	}
	return nil
}

// yyLevelFlushPending 重连后补同步：返回成功同步条数。
// 仍未绑定的留在队列里，等绑定后再冲。
func yyLevelFlushPending() int {
	cf := yyLevelConfigSnapshot()
	yyLevel.Lock()
	pending := append([]yyLevelPending(nil), yyLevel.Pending...)
	yyLevel.Unlock()
	done := 0
	for _, item := range pending {
		if strings.TrimSpace(cf.PlayerBind[item.QQ]) == "" {
			continue
		}
		if e := yyLevelSyncGame(item.Group, item.QQ); e == nil {
			yyLevelDropPending(item.Group, item.QQ)
			done++
		}
	}
	return done
}

// ---------- 面板接口 ----------

// yyLevelRoutes 等级面板接口。root 用于定位 qq-levels.json（照 yyQQAssistantRoutes 范式）。
func yyLevelRoutes(mux *http.ServeMux, root string) {
	yyLevelInit(root)
	mux.HandleFunc("/api/qq/level", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cf := yyLevelConfigSnapshot()
			yyLevel.Lock()
			players, pending, objective, diskErr := len(yyLevel.Players), len(yyLevel.Pending), yyLevel.ObjectiveAdded, yyLevel.DiskError
			groups := map[string]bool{}
			for _, row := range yyLevel.Players {
				groups[row.Group] = true
			}
			yyLevel.Unlock()
			bindings := make([]map[string]any, 0, len(cf.PlayerBind))
			for qq, player := range cf.PlayerBind {
				bindings = append(bindings, map[string]any{"qq": qq, "player": player})
			}
			sort.Slice(bindings, func(i, j int) bool { return bindings[i]["qq"].(int64) < bindings[j]["qq"].(int64) })
			writeJSON(w, map[string]any{
				"ok": true, "config": cf,
				"stats":    map[string]any{"players": players, "groups": len(groups), "pending": pending, "objective_added": objective, "persistence_error": diskErr},
				"bindings": bindings,
			})
		case http.MethodPost:
			yyLevelPostConfig(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
		}
	})
	mux.HandleFunc("/api/qq/level/top", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
			return
		}
		cf := yyQQSnapshot()
		group := strings.TrimSpace(r.URL.Query().Get("group"))
		if group == "" {
			group = yyQQGroupString(cf.GroupID)
		}
		if !yyQQGroupEnabled(cf, group) {
			writeJSON(w, map[string]any{"ok": false, "error": "这个群未启用"})
			return
		}
		n := 10
		if v := strings.TrimSpace(r.URL.Query().Get("n")); v != "" {
			if parsed, e := strconv.Atoi(v); e == nil {
				n = parsed
			}
		}
		writeJSON(w, map[string]any{"ok": true, "group": group, "rows": yyLevelTop(group, n)})
	})
}

// yyLevelPostConfig 处理 POST /api/qq/level：整体替换配置与 QQ→玩家名绑定表。
func yyLevelPostConfig(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Enabled         *bool             `json:"enabled"`
		XPChat          *int              `json:"xp_chat"`
		ChatCooldownSec *int              `json:"chat_cooldown_sec"`
		LevelBase       *int              `json:"level_base"`
		LevelFactor     *float64          `json:"level_factor"`
		MaxLevel        *int              `json:"max_level"`
		Announce        *bool             `json:"announce"`
		SyncGame        *bool             `json:"sync_game"`
		PlayerBind      map[string]string `json:"player_bind"`
	}
	if e := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); e != nil {
		writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
		return
	}
	bind := map[int64]string{}
	for rawQQ, player := range payload.PlayerBind {
		qq, e := strconv.ParseInt(strings.TrimSpace(rawQQ), 10, 64)
		if e != nil || qq == 0 {
			writeJSON(w, map[string]any{"ok": false, "error": "QQ 号无效：" + rawQQ})
			return
		}
		player = strings.TrimSpace(player)
		if player == "" {
			continue // 空玩家名 = 删除该绑定
		}
		if _, ok := yyLevelSafePlayer(player); !ok {
			writeJSON(w, map[string]any{"ok": false, "error": "玩家名含非法字符（换行/控制字符/引号）：" + player})
			return
		}
		bind[qq] = player
	}
	e := yyLevelUpdate(func(c *yyLevelConfig) {
		if payload.Enabled != nil {
			c.Enabled = *payload.Enabled
		}
		if payload.XPChat != nil {
			c.XPChat = *payload.XPChat
		}
		if payload.ChatCooldownSec != nil {
			c.ChatCooldownSec = *payload.ChatCooldownSec
		}
		if payload.LevelBase != nil {
			c.LevelBase = *payload.LevelBase
		}
		if payload.LevelFactor != nil {
			c.LevelFactor = *payload.LevelFactor
		}
		if payload.MaxLevel != nil {
			c.MaxLevel = *payload.MaxLevel
		}
		if payload.Announce != nil {
			c.Announce = *payload.Announce
		}
		if payload.SyncGame != nil {
			c.SyncGame = *payload.SyncGame
		}
		if payload.PlayerBind != nil {
			c.PlayerBind = bind
		}
	})
	if e != nil {
		writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "config": yyLevelConfigSnapshot()})
}
