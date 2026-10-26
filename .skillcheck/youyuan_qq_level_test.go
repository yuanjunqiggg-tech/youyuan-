package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixtures replace only private in-memory stores and injectable channels.
// No test sends a QQ message or contacts a real game server.

// yyLevelFixture 隔离全局状态：替换执行通道/播报/时钟，测试绝不真连游戏或 QQ。
// 返回可推进的冻结时钟（冷却判定经 yyLevelNow 注入）。
func yyLevelFixture(t *testing.T) *time.Time {
	t.Helper()
	oldQQ := yyQQ
	oldPath, oldCfg, oldPlayers, oldPending, oldObj, oldErr := yyLevel.Path, yyLevel.Config, yyLevel.Players, yyLevel.Pending, yyLevel.ObjectiveAdded, yyLevel.DiskError
	oldCooldown := yyLevelCooldown.last
	oldExec, oldAnnounce, oldNow := yyLevelExec, yyLevelAnnounce, yyLevelNow
	t.Cleanup(func() {
		yyQQ = oldQQ
		yyLevel.Path, yyLevel.Config, yyLevel.Players, yyLevel.Pending, yyLevel.ObjectiveAdded, yyLevel.DiskError = oldPath, oldCfg, oldPlayers, oldPending, oldObj, oldErr
		yyLevelCooldown.last = oldCooldown
		yyLevelExec, yyLevelAnnounce, yyLevelNow = oldExec, oldAnnounce, oldNow
	})
	yyQQ = yyQQConfig{Enabled: true, Adapter: "onebot", GroupID: "group-a", Groups: []yyQQGroup{{ID: "group-a", Enabled: true}, {ID: "group-b", Enabled: true}}}
	yyQQ.OneBot.BotID = "10001"
	yyLevel.Path = ""
	yyLevel.Config = yyLevelDefaultConfig()
	yyLevel.Players = map[string]yyLevelPlayer{}
	yyLevel.Pending = nil
	yyLevel.ObjectiveAdded = false
	yyLevel.DiskError = ""
	yyLevelCooldown.last = map[string]time.Time{}
	now := time.Unix(1700000000, 0)
	yyLevelNow = func() time.Time { return now }
	yyLevelExec = func(cmd string) error { return nil }
	yyLevelAnnounce = func(group, text string) error { return nil }
	return &now
}

// 加经验 + 冷却（10 秒内二次不加）+ 升级播报与局内同步联动。
func TestQQLevelOnMessageXPCooldownLevelUpAnnounceAndSync(t *testing.T) {
	clock := yyLevelFixture(t)
	yyLevel.Config = yyLevelConfig{Enabled: true, XPChat: 5, ChatCooldownSec: 10, LevelBase: 10, LevelFactor: 1, MaxLevel: 100, Announce: true, SyncGame: true, PlayerBind: map[int64]string{12345: "Steve"}}
	announced := make(chan string, 4)
	yyLevelAnnounce = func(group, text string) error { announced <- group + "|" + text; return nil }
	execd := make(chan string, 4)
	yyLevelExec = func(cmd string) error { execd <- cmd; return nil }
	msg := func() *yyQQInbound {
		return &yyQQInbound{Group: "group-a", User: "12345", Name: "小明", Text: "在吗"}
	}

	if up, level := yyLevelOnMessage(msg()); up || level != 1 {
		t.Fatal("首条消息不应升级", up, level)
	}
	if got := yyLevelXP("group-a", 12345); got != 5 {
		t.Fatal("经验未入账", got)
	}
	select {
	case <-announced:
		t.Fatal("未升级不应播报")
	case <-execd:
		t.Fatal("未升级不应同步局内")
	default:
	}
	// 10 秒冷却内二次不加。
	*clock = clock.Add(3 * time.Second)
	if up, level := yyLevelOnMessage(msg()); up || level != 1 {
		t.Fatal("冷却内二次消息被计数", up, level)
	}
	if got := yyLevelXP("group-a", 12345); got != 5 {
		t.Fatal("同群同 QQ 冷却被绕过", got)
	}
	// 冷却后恢复，并借这次升级验证播报与局内同步。
	*clock = clock.Add(11 * time.Second)
	if up, level := yyLevelOnMessage(msg()); !up || level != 2 {
		t.Fatal("冷却后应加经验并升到 2 级", up, level)
	}
	if got := yyLevelXP("group-a", 12345); got != 10 {
		t.Fatal("冷却后经验未入账", got)
	}
	select {
	case got := <-announced:
		if !strings.HasPrefix(got, "group-a|") || !strings.Contains(got, "小明") || !strings.Contains(got, "Lv.2") {
			t.Fatal("播报内容不对", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("升级未播报")
	}
	select {
	case cmd := <-execd:
		if cmd != "scoreboard objectives add qqlevel dummy 等级" {
			t.Fatal("首次同步应先建记分板", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("升级未同步局内")
	}
	select {
	case cmd := <-execd:
		if cmd != "scoreboard players set \"Steve\" qqlevel 2" {
			t.Fatal("玩家等级写入命令不对", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("缺少玩家等级写入命令")
	}
	// 私聊、机器人自身、未启用群、非数字 QQ（官方 openid）都不计经验。
	before := yyLevelXP("group-a", 12345)
	for _, m := range []*yyQQInbound{
		{Group: "group-a", User: "12345", IsPrivate: true, Text: "x"},
		{Group: "group-a", User: "10001", Text: "x"},
		{Group: "group-c", User: "999", Text: "x"},
		{Group: "group-a", User: "openid-abc", Text: "x"},
	} {
		if up, _ := yyLevelOnMessage(m); up {
			t.Fatal("非法入站被计经验", m)
		}
	}
	if got := yyLevelXP("group-a", 12345); got != before {
		t.Fatal("非法入站改变了经验", got)
	}
}

// 升级阈值数学：L→L+1 需 LevelBase*LevelFactor^(L-1)，含小数 factor 与封顶。
func TestQQLevelCurveMathWithFractionalFactor(t *testing.T) {
	yyLevelFixture(t)
	cf := yyLevelConfig{LevelBase: 100, LevelFactor: 1.5, MaxLevel: 10}
	yyLevelNormalize(&cf)
	for _, c := range []struct {
		xp               int
		level, cur, need int
	}{
		{0, 1, 0, 100}, {99, 1, 99, 100}, {100, 2, 0, 150}, {249, 2, 149, 150},
		{250, 3, 0, 225}, {474, 3, 224, 225}, {475, 4, 0, 338},
	} {
		level, cur, need, _ := yyLevelProgressOf(cf, c.xp)
		if level != c.level || cur != c.cur || need != c.need {
			t.Fatalf("xp=%d 得到 Lv.%d %d/%d，期望 Lv.%d %d/%d", c.xp, level, cur, need, c.level, c.cur, c.need)
		}
	}
	// MaxLevel 封顶：再多经验也停在封顶等级。
	level, cur, need, pct := yyLevelProgressOf(cf, 100000)
	if level != 10 || cur != 0 || need != 0 || pct != 100 {
		t.Fatal("封顶失败", level, cur, need, pct)
	}
	// 倍率 1 = 线性曲线。
	flat := yyLevelConfig{LevelBase: 50, LevelFactor: 1, MaxLevel: 5}
	yyLevelNormalize(&flat)
	if l, c, n, _ := yyLevelProgressOf(flat, 120); l != 3 || c != 20 || n != 50 {
		t.Fatal("线性曲线不对", l, c, n)
	}
	// 非法配置收敛：base<=0 / factor<=0 / NaN 都回落默认。
	broken := yyLevelConfig{LevelBase: 0, LevelFactor: 0, MaxLevel: 0}
	yyLevelNormalize(&broken)
	if broken.LevelBase != 100 || broken.LevelFactor != 1.5 || broken.MaxLevel != 100 {
		t.Fatal("非法配置未收敛", broken)
	}
}

// 进度百分比与查询文案（含满级文案）。
func TestQQLevelProgressPercentAndQueryText(t *testing.T) {
	yyLevelFixture(t)
	yyLevel.Config = yyLevelConfig{LevelBase: 100, LevelFactor: 2, MaxLevel: 100}
	for _, c := range []struct {
		xp  int
		pct float64
	}{{0, 0}, {25, 25}, {50, 50}, {100, 0}, {150, 25}, {500, 50}} {
		_, _, _, pct := yyLevelProgress(c.xp)
		if pct < c.pct-0.001 || pct > c.pct+0.001 {
			t.Fatalf("xp=%d 百分比 %v，期望 %v", c.xp, pct, c.pct)
		}
	}
	if got := yyLevelQuery("group-a", 999); got != "Lv.1｜经验 0/100（还差 100 升级）" {
		t.Fatal("无记录查询文案不对", got)
	}
	yyLevelAddXP("group-a", 12345, 350) // 100+200=300 升 Lv.3，余 50；Lv.3→Lv.4 需 400
	if got := yyLevelQuery("group-a", 12345); got != "Lv.3｜经验 50/400（还差 350 升级）" {
		t.Fatal("查询文案不对", got)
	}
	yyLevel.Config.MaxLevel = 3
	if got := yyLevelQuery("group-a", 12345); got != "Lv.3｜经验 已满级" {
		t.Fatal("满级文案不对", got)
	}
}

// 持久化 round-trip：经验、配置、绑定表、待同步队列重载后全部恢复。
func TestQQLevelPersistenceRoundTrip(t *testing.T) {
	yyLevelFixture(t)
	dir := t.TempDir()
	yyLevelInit(dir)
	if e := yyLevelUpdate(func(c *yyLevelConfig) {
		c.Enabled = true
		c.XPChat = 7
		c.ChatCooldownSec = 30
		c.LevelBase = 50
		c.LevelFactor = 2.5
		c.MaxLevel = 20
		c.PlayerBind = map[int64]string{12345: "Steve", 67890: "Alex"}
	}); e != nil {
		t.Fatal(e)
	}
	yyLevelAddXP("group-a", 12345, 120)
	yyLevelAddXP("group-b", 67890, 30)
	yyLevelObserveName("group-a", 12345, "小明")
	yyLevelEnqueuePending("group-a", 777) // 无绑定，入队
	yyLevelInit(dir)                      // 重新载入
	if got := yyLevelXP("group-a", 12345); got != 120 {
		t.Fatal("经验 round-trip 丢失", got)
	}
	if got := yyLevelXP("group-b", 67890); got != 30 {
		t.Fatal("跨群经验丢失", got)
	}
	cf := yyLevelConfigSnapshot()
	if !cf.Enabled || cf.XPChat != 7 || cf.ChatCooldownSec != 30 || cf.LevelBase != 50 || cf.LevelFactor != 2.5 || cf.MaxLevel != 20 {
		t.Fatal("配置 round-trip 丢失", cf)
	}
	if cf.PlayerBind[12345] != "Steve" || cf.PlayerBind[67890] != "Alex" {
		t.Fatal("绑定表 round-trip 丢失", cf.PlayerBind)
	}
	if len(yyLevel.Pending) != 1 || yyLevel.Pending[0].QQ != 777 {
		t.Fatal("待同步队列 round-trip 丢失", yyLevel.Pending)
	}
	raw, e := os.ReadFile(filepath.Join(dir, "qq-levels.json"))
	if e != nil || !json.Valid(raw) {
		t.Fatal("持久化文件不是合法 JSON", e)
	}
	if !strings.Contains(string(raw), "小明") {
		t.Fatal("昵称未持久化", string(raw))
	}
}

// 局内命令渲染：假执行器验证，含首次 objectives add 只发一次（含重载后）。
func TestQQLevelGameSyncCommandsAndObjectiveOnce(t *testing.T) {
	yyLevelFixture(t)
	dir := t.TempDir()
	yyLevelInit(dir)
	var cmds []string
	yyLevelExec = func(cmd string) error { cmds = append(cmds, cmd); return nil }
	if e := yyLevelUpdate(func(c *yyLevelConfig) {
		c.SyncGame = true
		c.LevelBase = 100
		c.LevelFactor = 1
		c.PlayerBind = map[int64]string{12345: "Steve"}
	}); e != nil {
		t.Fatal(e)
	}
	yyLevelAddXP("group-a", 12345, 250) // Lv.3
	if e := yyLevelSyncGame("group-a", 12345); e != nil {
		t.Fatal(e)
	}
	want := []string{"scoreboard objectives add qqlevel dummy 等级", "scoreboard players set \"Steve\" qqlevel 3"}
	if len(cmds) != 2 || cmds[0] != want[0] || cmds[1] != want[1] {
		t.Fatal("首次同步命令不对", cmds)
	}
	// 第二次只写玩家等级。
	cmds = nil
	if e := yyLevelSyncGame("group-a", 12345); e != nil {
		t.Fatal(e)
	}
	if len(cmds) != 1 || cmds[0] != want[1] {
		t.Fatal("objectives add 重复发送", cmds)
	}
	// 状态位持久化：重载后依然不重发 add。
	yyLevelInit(dir)
	cmds = nil
	if e := yyLevelSyncGame("group-a", 12345); e != nil {
		t.Fatal(e)
	}
	if len(cmds) != 1 || cmds[0] != want[1] {
		t.Fatal("重载后 objectives add 重发", cmds)
	}
	// 关闭 SyncGame 后直接跳过。
	yyLevelUpdate(func(c *yyLevelConfig) { c.SyncGame = false })
	cmds = nil
	if e := yyLevelSyncGame("group-a", 12345); e != nil {
		t.Fatal(e)
	}
	if len(cmds) != 0 {
		t.Fatal("关闭同步后仍发指令", cmds)
	}
}

// 玩家名注入被拒：\n \r 控制字符、双引号、空名、超长名一律拒绝且不执行指令。
func TestQQLevelRejectsPlayerNameInjection(t *testing.T) {
	yyLevelFixture(t)
	var cmds []string
	yyLevelExec = func(cmd string) error { cmds = append(cmds, cmd); return nil }
	for _, bad := range []string{"bad\nname", "bad\rname", "bad\x00name", "bad\"name", "   ", strings.Repeat("x", 49)} {
		yyLevel.Config = yyLevelConfig{SyncGame: true, LevelBase: 100, LevelFactor: 1, PlayerBind: map[int64]string{12345: bad}}
		if _, ok := yyLevelSafePlayer(bad); ok {
			t.Fatal("非法玩家名通过校验", bad)
		}
		if e := yyLevelSyncGame("group-a", 12345); e == nil {
			t.Fatal("注入玩家名被放行", bad)
		}
		if len(cmds) != 0 {
			t.Fatal("非法玩家名仍执行了指令", cmds)
		}
		if len(yyLevel.Pending) != 0 {
			t.Fatal("非法玩家名不应入待同步队列")
		}
	}
	// 合法名字（含空格与中文）正常放行并以 JSON 引号嵌入。
	yyLevel.Config = yyLevelConfig{SyncGame: true, LevelBase: 100, LevelFactor: 1, PlayerBind: map[int64]string{12345: " 小 明 "}}
	if safe, ok := yyLevelSafePlayer(" 小 明 "); !ok || safe != "小 明" {
		t.Fatal("合法玩家名被拒", safe, ok)
	}
	if e := yyLevelSyncGame("group-a", 12345); e != nil {
		t.Fatal(e)
	}
	if len(cmds) != 2 || cmds[1] != "scoreboard players set \"小 明\" qqlevel 1" {
		t.Fatal("中文/空格玩家名命令不对", cmds)
	}
}

// 执行失败/无绑定入 pendingSync，重连后 yyLevelFlushPending 补同步。
func TestQQLevelFlushPendingAfterReconnect(t *testing.T) {
	yyLevelFixture(t)
	dir := t.TempDir()
	yyLevelInit(dir)
	var cmds []string
	broken := true
	yyLevelExec = func(cmd string) error {
		if broken {
			return fmt.Errorf("游戏通道未连接")
		}
		cmds = append(cmds, cmd)
		return nil
	}
	if e := yyLevelUpdate(func(c *yyLevelConfig) {
		c.SyncGame = true
		c.LevelBase = 100
		c.LevelFactor = 1
		c.PlayerBind = map[int64]string{12345: "Steve"}
	}); e != nil {
		t.Fatal(e)
	}
	// 通道断开：同步失败入队。
	if e := yyLevelSyncGame("group-a", 12345); e == nil {
		t.Fatal("通道断开应报错")
	}
	if len(yyLevel.Pending) != 1 {
		t.Fatal("失败未入待同步队列", yyLevel.Pending)
	}
	// 无绑定的也入队（等绑定后补同步）。
	yyLevelEnqueuePending("group-a", 777)
	// 重连后补同步：只冲已绑定的，未绑定的留队。
	broken = false
	if n := yyLevelFlushPending(); n != 1 {
		t.Fatal("补同步条数不对", n)
	}
	if len(yyLevel.Pending) != 1 || yyLevel.Pending[0].QQ != 777 {
		t.Fatal("未绑定条目应留在队列", yyLevel.Pending)
	}
	if len(cmds) != 2 || cmds[0] != "scoreboard objectives add qqlevel dummy 等级" || cmds[1] != "scoreboard players set \"Steve\" qqlevel 1" {
		t.Fatal("补同步命令不对", cmds)
	}
	// 队列持久化：重载后仍在。
	yyLevelInit(dir)
	if len(yyLevel.Pending) != 1 || yyLevel.Pending[0].QQ != 777 {
		t.Fatal("待同步队列未持久化", yyLevel.Pending)
	}
}

// 排行榜：跨群隔离、按经验降序、同经验按 QQ 升序、n 的边界。
func TestQQLevelTopRanking(t *testing.T) {
	yyLevelFixture(t)
	yyLevel.Config = yyLevelConfig{LevelBase: 100, LevelFactor: 1, MaxLevel: 100}
	yyLevelAddXP("group-a", 111, 250)
	yyLevelAddXP("group-a", 222, 150)
	yyLevelAddXP("group-a", 333, 250)
	yyLevelAddXP("group-b", 444, 999)
	yyLevelObserveName("group-a", 111, "甲")
	yyLevelObserveName("group-a", 333, "丙")
	rows := yyLevelTop("group-a", 2)
	if len(rows) != 2 {
		t.Fatal("条数不对", rows)
	}
	if rows[0].QQ != 111 || rows[0].Level != 3 || rows[0].Name != "甲" || rows[0].XP != 250 {
		t.Fatal("第一名不对", rows[0])
	}
	if rows[1].QQ != 333 || rows[1].Name != "丙" {
		t.Fatal("同经验应按 QQ 升序", rows[1])
	}
	if len(yyLevelTop("group-a", 0)) != 3 {
		t.Fatal("n<=0 应取默认 10")
	}
	if len(yyLevelTop("group-a", 1000)) != 3 {
		t.Fatal("n 超上限应截到 100")
	}
	if len(yyLevelTop("group-a", 1)) != 1 {
		t.Fatal("n=1 应只返回一条")
	}
}

// API：GET/POST /api/qq/level 与 GET /api/qq/level/top，含注入与群校验。
func TestQQLevelAPIConfigTopAndGuards(t *testing.T) {
	yyLevelFixture(t)
	dir := t.TempDir()
	mux := http.NewServeMux()
	yyLevelRoutes(mux, dir)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/qq/level", strings.NewReader(body)))
		return w
	}
	// GET 默认配置 + 统计 + 绑定表。
	w := get("/api/qq/level")
	if !strings.Contains(w.Body.String(), "\"ok\":true") || !strings.Contains(w.Body.String(), "\"level_factor\":1.5") {
		t.Fatal("GET 默认配置不对", w.Body.String())
	}
	// POST 改配置 + 绑定（空玩家名视为删除）。
	w = post("{\"enabled\":true,\"xp_chat\":7,\"chat_cooldown_sec\":30,\"level_base\":50,\"level_factor\":2.5,\"max_level\":20,\"announce\":false,\"sync_game\":true,\"player_bind\":{\"12345\":\"Steve\",\"67890\":\"  \"}}")
	if !strings.Contains(w.Body.String(), "\"ok\":true") {
		t.Fatal("POST 配置失败", w.Body.String())
	}
	cf := yyLevelConfigSnapshot()
	if !cf.Enabled || cf.XPChat != 7 || cf.ChatCooldownSec != 30 || cf.LevelBase != 50 || cf.LevelFactor != 2.5 || cf.MaxLevel != 20 || cf.Announce {
		t.Fatal("POST 未生效", cf)
	}
	if len(cf.PlayerBind) != 1 || cf.PlayerBind[12345] != "Steve" {
		t.Fatal("绑定表不对（空玩家名应视为删除）", cf.PlayerBind)
	}
	// GET 反映新配置与绑定表。
	w = get("/api/qq/level")
	if !strings.Contains(w.Body.String(), "\"xp_chat\":7") || !strings.Contains(w.Body.String(), "\"player\":\"Steve\"") {
		t.Fatal("GET 未反映新配置", w.Body.String())
	}
	// 注入玩家名与非法 QQ 号都被拒，且不影响已存配置。
	if w = post("{\"player_bind\":{\"12345\":\"bad\nname\"}}"); strings.Contains(w.Body.String(), "\"ok\":true") {
		t.Fatal("注入玩家名被接受", w.Body.String())
	}
	if w = post("{\"player_bind\":{\"not-a-qq\":\"Steve\"}}"); strings.Contains(w.Body.String(), "\"ok\":true") {
		t.Fatal("非法 QQ 号被接受", w.Body.String())
	}
	if cf = yyLevelConfigSnapshot(); len(cf.PlayerBind) != 1 || cf.PlayerBind[12345] != "Steve" {
		t.Fatal("被拒的请求污染了配置", cf.PlayerBind)
	}
	// 排行榜：base50 factor2.5，12345 得 60 经验升 Lv.2，222 只有 10。
	yyLevelAddXP("group-a", 12345, 60)
	yyLevelAddXP("group-a", 222, 10)
	w = get("/api/qq/level/top?group=group-a&n=1")
	if !strings.Contains(w.Body.String(), "\"ok\":true") || !strings.Contains(w.Body.String(), "\"qq\":12345") || strings.Contains(w.Body.String(), "\"qq\":222") {
		t.Fatal("排行榜不对", w.Body.String())
	}
	// 未启用群拒绝。
	if w = get("/api/qq/level/top?group=group-c"); strings.Contains(w.Body.String(), "\"ok\":true") {
		t.Fatal("未启用群被接受", w.Body.String())
	}
	// 方法限制。
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/qq/level", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal("DELETE 应被拒绝", w.Code)
	}
}
