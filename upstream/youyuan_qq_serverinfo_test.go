package main

// 互通端信息面板的接口测试。
// 这些 handler 直接读全局状态（yyQQ / yyQQHelper），所以每个用例都要
// 用 t.Cleanup 把自己动过的全局量还原，避免污染其它测试。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func yySrvInfoTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	yySrvInfoRoutes(mux)
	return mux
}

func yySrvInfoTestGet(t *testing.T, mux *http.ServeMux, path string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s 状态码 %d，期望 200", path, rec.Code)
	}
	var out map[string]any
	if e := json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
		t.Fatalf("%s 返回的不是 JSON：%v（%s）", path, e, rec.Body.String())
	}
	return out
}

const yySrvInfoTestToken = "test-token"

func yySrvInfoTestPost(t *testing.T, mux *http.ServeMux, path, body string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	// 指令模式现在要求令牌（H3 修复）：测试统一带上，专门测鉴权的用例自己覆盖。
	req.Header.Set("X-Bridge-Token", yySrvInfoTestToken)
	mux.ServeHTTP(rec, req)
	var out map[string]any
	if e := json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
		t.Fatalf("%s 返回的不是 JSON：%v（%s）", path, e, rec.Body.String())
	}
	return out
}

// yySrvInfoTestIsolate 备份并还原测试会碰到的全局量。
func yySrvInfoTestIsolate(t *testing.T) {
	t.Helper()
	yyQQMu.Lock()
	savedConfig := yyQQ
	yyQQ = yyQQConfig{}
	yyQQ.Assistant.MCPToken = yySrvInfoTestToken
	yyQQMu.Unlock()

	yyQQHelper.Lock()
	savedStats := yyQQHelper.Stats
	savedBindings := yyQQHelper.Bindings
	yyQQHelper.Stats = map[string]*yyQQPlayerStats{}
	yyQQHelper.Bindings = map[string]string{}
	yyQQHelper.Unlock()

	// 缓存结构内嵌了 sync.Mutex，只能逐字段备份（整体赋值会复制锁）。
	yySrvInfoBot.Lock()
	savedBotAt, savedBotNick := yySrvInfoBot.At, yySrvInfoBot.Nickname
	savedBotUIN, savedBotAvatar := yySrvInfoBot.UIN, yySrvInfoBot.Avatar
	savedBotErr := yySrvInfoBot.Err
	yySrvInfoBot.Nickname, yySrvInfoBot.UIN, yySrvInfoBot.Avatar = "", "", ""
	yySrvInfoBot.Err, yySrvInfoBot.At, yySrvInfoBot.Refreshing = "", time.Time{}, false
	yySrvInfoBot.Unlock()

	t.Cleanup(func() {
		yyQQMu.Lock()
		yyQQ = savedConfig
		yyQQMu.Unlock()

		yyQQHelper.Lock()
		yyQQHelper.Stats = savedStats
		yyQQHelper.Bindings = savedBindings
		yyQQHelper.Unlock()

		yySrvInfoBot.Lock()
		yySrvInfoBot.At, yySrvInfoBot.Nickname = savedBotAt, savedBotNick
		yySrvInfoBot.UIN, yySrvInfoBot.Avatar = savedBotUIN, savedBotAvatar
		yySrvInfoBot.Err, yySrvInfoBot.Refreshing = savedBotErr, false
		yySrvInfoBot.Unlock()
	})
}

// 用例 1：没配置机器人时，info 也要成功返回，不能 panic、不能整体失败。
func TestSrvInfoInfoWithoutConfig(t *testing.T) {
	yySrvInfoTestIsolate(t)
	mux := yySrvInfoTestMux()

	out := yySrvInfoTestGet(t, mux, "/api/qq/server/info")
	if out["ok"] != true {
		t.Fatalf("未配置时 info 应返回 ok:true，实际 %+v", out)
	}
	if name, _ := out["name"].(string); name == "" {
		t.Fatalf("服务器名不应为空（应回退到「互通服务器」），实际 %+v", out)
	}
	// 没填机器人QQ号时必须给出中文原因，而不是一个空对象。
	if out["bot"] != nil {
		t.Fatalf("未配置时 bot 应为 null，实际 %+v", out["bot"])
	}
	if errText, _ := out["error"].(string); errText == "" {
		t.Fatalf("bot 为 null 时必须带 error 说明，实际 %+v", out)
	}
	// 游戏没连上时在线人数必须是 null，不能谎报 0。
	if v, exists := out["online"]; !exists || v != nil {
		t.Fatalf("游戏未连接时 online 应为 null，实际 %+v", out["online"])
	}
}

// 用例 2：塞两条假统计后，玩家列表要返回 2 条且字段齐全。
func TestSrvInfoPlayersFromStats(t *testing.T) {
	yySrvInfoTestIsolate(t)
	mux := yySrvInfoTestMux()

	now := time.Now()
	yyQQHelper.Lock()
	yyQQHelper.Stats["Alice"] = &yyQQPlayerStats{
		Joined:        now.Add(-2 * time.Hour),
		LastJoin:      now.Add(-2 * time.Hour),
		OnlineSeconds: 3600,
		Messages:      12,
		LastMessage:   "大家好",
	}
	yyQQHelper.Stats["Bob"] = &yyQQPlayerStats{
		LastJoin:  now.Add(-30 * time.Minute),
		LastLeave: now.Add(-10 * time.Minute),
		Messages:  3,
	}
	yyQQHelper.Bindings["10001"] = "Alice"
	yyQQHelper.Unlock()

	out := yySrvInfoTestGet(t, mux, "/api/qq/server/players")
	if out["ok"] != true {
		t.Fatalf("players 应返回 ok:true，实际 %+v", out)
	}
	rows, _ := out["players"].([]any)
	if len(rows) != 2 {
		t.Fatalf("期望 2 条玩家，实际 %d 条：%+v", len(rows), out["players"])
	}

	// 字段齐全 + 绑定反查（QQ号→玩家名 反成 玩家名→QQ号）
	found := false
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		for _, key := range []string{"player", "qq", "avatar", "online", "online_seconds", "messages", "last_message", "joined", "last_join", "last_leave"} {
			if _, exists := row[key]; !exists {
				t.Fatalf("玩家行缺少字段 %s：%+v", key, row)
			}
		}
		if row["player"] == "Alice" {
			found = true
			if row["qq"] != "10001" {
				t.Fatalf("Alice 的 QQ 应为 10001，实际 %+v", row["qq"])
			}
			if av, _ := row["avatar"].(string); !strings.Contains(av, "/api/skin/online-head") {
				t.Fatalf("头像应沿用 /api/skin/online-head，实际 %q", av)
			}
			// Alice 的 Joined 非零表示她还在线，累计时长要补上本次时长。
			if secs, _ := row["online_seconds"].(float64); secs < 3600 {
				t.Fatalf("在线秒数应至少 3600，实际 %v", row["online_seconds"])
			}
		}
	}
	if !found {
		t.Fatalf("没有找到 Alice：%+v", rows)
	}
	// 游戏未连接，online 必须是 null（不瞎猜）。
	if out["note"] == nil {
		t.Fatalf("在线状态未知时应有 note 说明，实际 %+v", out)
	}
}

// 用例 3：危险命令必须被拒绝（/op someone）。
func TestSrvInfoSayRejectsDangerousCommand(t *testing.T) {
	yySrvInfoTestIsolate(t)
	mux := yySrvInfoTestMux()

	out := yySrvInfoTestPost(t, mux, "/api/qq/server/say", `{"text":"/op someone","mode":"command"}`)
	if out["ok"] != false {
		t.Fatalf("危险命令必须被拒绝，实际 %+v", out)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "op") {
		t.Fatalf("拒绝原因应提到 op，实际 %q", msg)
	}
}

// 用例 4：空内容必须被拒绝。
func TestSrvInfoSayRejectsEmptyText(t *testing.T) {
	yySrvInfoTestIsolate(t)
	mux := yySrvInfoTestMux()

	out := yySrvInfoTestPost(t, mux, "/api/qq/server/say", `{"text":"   ","mode":"chat"}`)
	if out["ok"] != false {
		t.Fatalf("空内容必须被拒绝，实际 %+v", out)
	}
	if msg, _ := out["error"].(string); msg == "" {
		t.Fatalf("空内容拒绝时必须给中文原因，实际 %+v", out)
	}
}

// 补充：fill 体积超限要被挡，小范围要放行（只校验守卫函数，不真的发命令）。
func TestSrvInfoCommandGuardFill(t *testing.T) {
	if e := yySrvInfoCommandGuard("/fill 0 0 0 200 200 200 stone"); e == nil {
		t.Fatalf("超大 fill 应被拒绝")
	}
	if e := yySrvInfoCommandGuard("/fill 0 0 0 5 5 5 stone"); e != nil {
		t.Fatalf("小范围 fill 应放行，实际 %v", e)
	}
	if e := yySrvInfoCommandGuard("/fill ~ ~ ~ ~5 ~5 ~5 stone"); e == nil {
		t.Fatalf("相对坐标 fill 应被拒绝（无法估算体积）")
	}
	if e := yySrvInfoCommandGuard("list"); e == nil {
		t.Fatalf("指令模式必须以 / 开头")
	}
}

// 用例 6：单人聚合接口一次给齐详情抽屉需要的字段。
// 抽屉不允许「先出空壳再补数据」，所以这里逐个字段校验存在性。
func TestSrvInfoSinglePlayer(t *testing.T) {
	yySrvInfoTestIsolate(t)
	mux := yySrvInfoTestMux()

	now := time.Now()
	yyQQHelper.Lock()
	yyQQHelper.Stats["Alice"] = &yyQQPlayerStats{
		Joined:        now.Add(-time.Hour),
		LastJoin:      now.Add(-time.Hour),
		OnlineSeconds: 600,
		Messages:      7,
		LastMessage:   "来下界挖矿",
	}
	yyQQHelper.Bindings["10001"] = "Alice"
	yyQQHelper.Unlock()
	yyQQMu.Lock()
	yyQQ.Assistant.Admins = []string{"10001"}
	yyQQMu.Unlock()

	out := yySrvInfoTestGet(t, mux, "/api/qq/server/player?name=Alice")
	if out["ok"] != true {
		t.Fatalf("单人详情应返回 ok:true，实际 %+v", out)
	}
	for _, key := range []string{"player", "qq", "role", "role_text", "online", "online_seconds", "messages", "last_message", "level", "tags"} {
		if _, exists := out[key]; !exists {
			t.Fatalf("详情缺少字段 %s：%+v", key, out)
		}
	}
	if out["player"] != "Alice" || out["qq"] != "10001" {
		t.Fatalf("玩家名/QQ 不对：%+v", out)
	}
	// 权限：管理员名单里的人必须是 role=1、角色名「管理员」。
	if role, _ := out["role"].(float64); role != 1 {
		t.Fatalf("Alice 应是管理员（role=1），实际 %+v", out["role"])
	}
	if out["role_text"] != "管理员" {
		t.Fatalf("role_text 应为「管理员」，实际 %+v", out["role_text"])
	}
	// 游戏未连接：online 必须是 null，不能谎报离线。
	if v, exists := out["online"]; !exists || v != nil {
		t.Fatalf("在线状态未知时 online 应为 null，实际 %+v", out["online"])
	}
	if secs, _ := out["online_seconds"].(float64); secs < 600 {
		t.Fatalf("在线秒数应至少 600，实际 %v", out["online_seconds"])
	}
	if msg, _ := out["messages"].(float64); msg != 7 {
		t.Fatalf("发言数应为 7，实际 %v", out["messages"])
	}
	if out["last_message"] != "来下界挖矿" {
		t.Fatalf("最近发言不对：%+v", out["last_message"])
	}
	// 形象预览必须指向服务端渲染好的玩家卡，前端直接当 <img src> 用。
	if card, _ := out["card"].(string); !strings.Contains(card, "/api/qq/player-card?player=Alice") {
		t.Fatalf("card 应指向 /api/qq/player-card，实际 %q", card)
	}
	// 积分块必须存在且带 enabled，前端据此决定显示「未启用」还是数值。
	level, ok := out["level"].(map[string]any)
	if !ok {
		t.Fatalf("level 应为对象，实际 %+v", out["level"])
	}
	if _, exists := level["enabled"]; !exists {
		t.Fatalf("level 缺少 enabled：%+v", level)
	}
	// 标签：没有数据源就返回空数组，不能是 null（前端不用判空），也不能编数据。
	tags, ok := out["tags"].([]any)
	if !ok || len(tags) != 0 {
		t.Fatalf("tags 应为空数组，实际 %+v", out["tags"])
	}
}

// 用例 7：单人接口的两种失败态都要有中文原因，不能 500。
func TestSrvInfoSinglePlayerMissing(t *testing.T) {
	yySrvInfoTestIsolate(t)
	mux := yySrvInfoTestMux()

	out := yySrvInfoTestGet(t, mux, "/api/qq/server/player")
	if out["ok"] != false {
		t.Fatalf("缺 name 应返回 ok:false，实际 %+v", out)
	}
	out = yySrvInfoTestGet(t, mux, "/api/qq/server/player?name=Ghost")
	if out["ok"] != false {
		t.Fatalf("查不到的人应返回 ok:false，实际 %+v", out)
	}
	if msg, _ := out["error"].(string); msg == "" {
		t.Fatalf("失败时必须给中文原因，实际 %+v", out)
	}
}

// 用例 8：只绑了 QQ、还没有统计记录的人（比如刚绑定没进过服），
// 详情也要成功返回，不能被判成「查不到」。
func TestSrvInfoSinglePlayerBoundOnly(t *testing.T) {
	yySrvInfoTestIsolate(t)
	mux := yySrvInfoTestMux()

	yyQQHelper.Lock()
	yyQQHelper.Bindings["20002"] = "Carol"
	yyQQHelper.Unlock()

	out := yySrvInfoTestGet(t, mux, "/api/qq/server/player?name=Carol")
	if out["ok"] != true {
		t.Fatalf("已绑定 QQ 的玩家应能查到，实际 %+v", out)
	}
	if out["qq"] != "20002" {
		t.Fatalf("QQ 应为 20002，实际 %+v", out["qq"])
	}
	if out["role_text"] != "成员" {
		t.Fatalf("默认角色应是成员，实际 %+v", out["role_text"])
	}
}
