package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// yyMemberReset 每个用例一份临时数据目录并重置内存态，保证用例间隔离。
func yyMemberReset(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	yyMemberInit(dir)
	yyEmojiInit(dir)
	return dir
}

// yyEmojiCallerStub 替换表情调用层（绝不触碰真实 WebSocket），用完恢复。
func yyEmojiCallerStub(t *testing.T, fn func(action string, params map[string]any) (any, error)) {
	t.Helper()
	orig := yyEmojiCaller
	yyEmojiCaller = fn
	t.Cleanup(func() { yyEmojiCaller = orig })
}

// yyMemberSetBot 设置机器人 QQ 号（验证机器人自身发言不进画像），用完恢复。
func yyMemberSetBot(t *testing.T, botID string) {
	t.Helper()
	yyQQMu.Lock()
	orig := yyQQ.OneBot.BotID
	yyQQ.OneBot.BotID = botID
	yyQQMu.Unlock()
	t.Cleanup(func() {
		yyQQMu.Lock()
		yyQQ.OneBot.BotID = orig
		yyQQMu.Unlock()
	})
}

// yyMemberEvent 造一条真实的 OneBot v11 群消息事件（走正牌解析管线，
// 与生产环境 yyQQParseInbound 的输入同构）。
func yyMemberEvent(group, user, self, card, text string) map[string]any {
	return map[string]any{
		"post_type": "message", "message_type": "group",
		"group_id": json.Number(group), "user_id": json.Number(user),
		"self_id": json.Number(self), "raw_message": text,
		"message": []any{map[string]any{"type": "text", "data": map[string]any{"text": text}}},
		"sender":  map[string]any{"card": card, "role": "member"},
	}
}

func yyMemberInbound(t *testing.T, event map[string]any) yyQQInbound {
	t.Helper()
	in, ok := yyQQParseInbound(event)
	if !ok {
		t.Fatal("事件应解析为入站消息")
	}
	return in
}

func yyMemberFeed(t *testing.T, event map[string]any) {
	t.Helper()
	in := yyMemberInbound(t, event)
	yyMemberOnMessage(&in)
}

func yyDecodeJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var out map[string]any
	if e := json.NewDecoder(resp.Body).Decode(&out); e != nil {
		t.Fatalf("响应不是合法 JSON: %v", e)
	}
	return out
}

func TestYYMemberOnMessageUpsert(t *testing.T) {
	dir := yyMemberReset(t)
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "777", "张三", "大家早"))
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "777", "张三三", "第二条"))
	yyMemberFeed(t, yyMemberEvent("1002", "10001", "777", "张三", "别的群"))
	// 私聊（无群号）不进画像
	yyMemberFeed(t, map[string]any{
		"post_type": "message", "message_type": "private",
		"user_id": json.Number("10001"), "self_id": json.Number("777"),
		"raw_message": "在吗",
		"message":     []any{map[string]any{"type": "text", "data": map[string]any{"text": "在吗"}}},
		"sender":      map[string]any{"nickname": "张三"},
	})
	yyMembers.Lock()
	rows := append([]yyQQMember(nil), yyMembers.Groups["1001"]...)
	other := len(yyMembers.Groups["1002"])
	yyMembers.Unlock()
	if len(rows) != 1 || other != 1 {
		t.Fatalf("应按群隔离且私聊不入库: 1001=%d 1002=%d", len(rows), other)
	}
	if rows[0].QQ != 10001 || rows[0].Name != "张三三" || rows[0].MsgCount != 2 || rows[0].Role != "member" {
		t.Fatalf("upsert 结果不对: %+v", rows[0])
	}
	if rows[0].FirstSeen == 0 || rows[0].LastSeen < rows[0].FirstSeen {
		t.Fatalf("首末见时间戳不对: %+v", rows[0])
	}
	if got := yyMemberResolve("1001", 10001); got != "张三三" {
		t.Fatalf("resolve = %q", got)
	}
	// 持久化 round-trip：重载后计数继续累加
	yyMemberInit(dir)
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "777", "张三三", "第三条"))
	yyMembers.Lock()
	rows = append([]yyQQMember(nil), yyMembers.Groups["1001"]...)
	yyMembers.Unlock()
	if len(rows) != 1 || rows[0].MsgCount != 3 {
		t.Fatalf("round-trip 后 MsgCount 应为 3: %+v", rows)
	}
	raw, e := os.ReadFile(filepath.Join(dir, "qq-members.json"))
	if e != nil || !strings.Contains(string(raw), "张三三") {
		t.Fatalf("qq-members.json 落盘内容不对: %v", e)
	}
}

func TestYYMemberAliasLearning(t *testing.T) {
	yyMemberReset(t)
	yyMemberSetBot(t, "999999")
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "999999", "张三", "叫我小猫"))
	if got := yyMemberResolve("1001", 10001); got != "小猫" {
		t.Fatalf("别名应优先于群名片: %q", got)
	}
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "999999", "张三", "我是小狗"))
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "999999", "张三", "我的名字是喵喵"))
	yyMembers.Lock()
	aliases := append([]string(nil), yyMembers.Groups["1001"][0].Aliases...)
	yyMembers.Unlock()
	if len(aliases) != 3 || aliases[0] != "喵喵" || aliases[1] != "小狗" || aliases[2] != "小猫" {
		t.Fatalf("别名顺序不对（最新置顶）: %v", aliases)
	}
	// 重复别名：去重并置顶，不膨胀
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "999999", "张三", "叫我小猫"))
	yyMembers.Lock()
	aliases = append([]string(nil), yyMembers.Groups["1001"][0].Aliases...)
	yyMembers.Unlock()
	if len(aliases) != 3 || aliases[0] != "小猫" {
		t.Fatalf("重复别名应去重并置顶: %v", aliases)
	}
	// 机器人自身发言不学别名
	yyMemberFeed(t, yyMemberEvent("1001", "999999", "999999", "机器人", "叫我坏蛋"))
	if _, ok := yyMemberFind("1001", "坏蛋"); ok {
		t.Fatal("机器人发言被学了别名")
	}
	// CQ 段（at/image）不污染别名学习
	yyMemberFeed(t, map[string]any{
		"post_type": "message", "message_type": "group",
		"group_id": json.Number("1001"), "user_id": json.Number("10002"),
		"self_id": json.Number("999999"), "raw_message": "[CQ:at,qq=10001] 叫我[CQ:image,file=x.jpg]阿花",
		"message": []any{
			map[string]any{"type": "at", "data": map[string]any{"qq": "10001"}},
			map[string]any{"type": "text", "data": map[string]any{"text": " 叫我"}},
			map[string]any{"type": "image", "data": map[string]any{"file": "x.jpg"}},
			map[string]any{"type": "text", "data": map[string]any{"text": "阿花"}},
		},
		"sender": map[string]any{"card": "李四"},
	})
	if qq, ok := yyMemberFind("1001", "阿花"); !ok || qq != 10002 {
		t.Fatalf("剔除 CQ 段后应学到 阿花: qq=%d ok=%v", qq, ok)
	}
	// 上限 10 个，最新的保留
	for i := 0; i < 15; i++ {
		yyMemberFeed(t, yyMemberEvent("1001", "10002", "999999", "李四", "叫我名"+strconv.Itoa(i)))
	}
	yyMembers.Lock()
	aliases = append([]string(nil), yyMembers.Groups["1001"][1].Aliases...)
	yyMembers.Unlock()
	if len(aliases) != 10 || aliases[0] != "名14" {
		t.Fatalf("别名上限/置顶不对: %d %v", len(aliases), aliases)
	}
}

func TestYYMemberResolveAndFind(t *testing.T) {
	yyMemberReset(t)
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "777", "张三", "叫我小猫"))
	yyMemberFeed(t, yyMemberEvent("1001", "10002", "777", "Alex", "大家好"))
	if got := yyMemberResolve("1001", 10001); got != "小猫" {
		t.Fatalf("别名优先: %q", got)
	}
	if got := yyMemberResolve("1001", 10002); got != "Alex" {
		t.Fatalf("无别名回落群名片: %q", got)
	}
	if got := yyMemberResolve("1001", 55555); got != "QQ:55555" {
		t.Fatalf("陌生人回落: %q", got)
	}
	if qq, ok := yyMemberFind("1001", "小猫"); !ok || qq != 10001 {
		t.Fatalf("别名精确匹配: %d %v", qq, ok)
	}
	if qq, ok := yyMemberFind("1001", "alex"); !ok || qq != 10002 {
		t.Fatalf("群名片精确匹配（大小写不敏感）: %d %v", qq, ok)
	}
	if qq, ok := yyMemberFind("1001", "小"); !ok || qq != 10001 {
		t.Fatalf("别名前缀匹配: %d %v", qq, ok)
	}
	if qq, ok := yyMemberFind("1001", "Al"); !ok || qq != 10002 {
		t.Fatalf("群名片前缀匹配: %d %v", qq, ok)
	}
	if _, ok := yyMemberFind("1001", "不存在的人"); ok {
		t.Fatal("不应匹配到不存在的人")
	}
	if _, ok := yyMemberFind("1001", "   "); ok {
		t.Fatal("空名字不应匹配")
	}
	if _, ok := yyMemberFind("9999", "小猫"); ok {
		t.Fatal("应按群隔离")
	}
}

func TestYYMemberMentionText(t *testing.T) {
	yyMemberReset(t)
	yyMemberSetBot(t, "777")
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "777", "张三", "你好"))
	yyMemberFeed(t, yyMemberEvent("1001", "10002", "777", "李四", "你好"))
	plain := &yyQQInbound{Group: "1001", User: "10001", Name: "张三"}
	if got := yyMemberMentionText(plain); got != "张三(10001)" {
		t.Fatalf("无 @ 形态: %q", got)
	}
	at := &yyQQInbound{Group: "1001", User: "10001", Name: "张三", Mentions: []string{"10002"}}
	if got := yyMemberMentionText(at); got != "@李四(10002) 张三(10001)" {
		t.Fatalf("@ 前缀形态: %q", got)
	}
	bot := &yyQQInbound{Group: "1001", User: "10001", Name: "张三", Mentions: []string{"777"}, Mentioned: true}
	if got := yyMemberMentionText(bot); got != "@机器人 张三(10001)" {
		t.Fatalf("@ 机器人形态: %q", got)
	}
	all := &yyQQInbound{Group: "1001", User: "10001", Name: "张三", Mentions: []string{"all"}, Mentioned: true}
	if got := yyMemberMentionText(all); got != "@全体成员 张三(10001)" {
		t.Fatalf("@全体形态: %q", got)
	}
	only := &yyQQInbound{Group: "1001", User: "10001", Name: "张三", Mentioned: true}
	if got := yyMemberMentionText(only); got != "@机器人 张三(10001)" {
		t.Fatalf("只有 Mentioned 的防御形态: %q", got)
	}
	stranger := &yyQQInbound{Group: "1001", User: "10003", Mentions: []string{"10009"}}
	if got := yyMemberMentionText(stranger); got != "@QQ:10009(10009) QQ:10003(10003)" {
		t.Fatalf("陌生人形态: %q", got)
	}
}

func TestYYMemberBlockTruncate(t *testing.T) {
	yyMemberReset(t)
	// 30 个成员，每人名字 30+ 字、发言数递增（活跃度排序确定）。
	for i := 1; i <= 30; i++ {
		qq := int64(10000 + i)
		name := strings.Repeat("名", 30) + strconv.Itoa(i)
		for j := 0; j < i; j++ {
			yyMemberFeed(t, yyMemberEvent("1001", strconv.FormatInt(qq, 10), "777", name, "嗨"))
		}
	}
	block := yyMemberBlock("1001", 50)
	if n := utf8.RuneCountInString(block); n > 600 {
		t.Fatalf("名录超长: %d 字", n)
	}
	if !strings.Contains(block, "未列出") {
		t.Fatalf("超出应计 omitted: %q", block)
	}
	first := block
	if idx := strings.Index(block, "、"); idx >= 0 {
		first = block[:idx]
	}
	if !strings.HasSuffix(first, "(10030)") {
		t.Fatalf("活跃度最高的应排最前: %q", first)
	}
	limited := yyMemberBlock("1001", 3)
	if !strings.Contains(limited, "27人未列出") {
		t.Fatalf("limit 截断计数不对: %q", limited)
	}
	if got := yyMemberBlock("999", 10); got != "" {
		t.Fatalf("空群应返回空串: %q", got)
	}
}

func TestYYEmojiIndexRefreshParses(t *testing.T) {
	dir := yyMemberReset(t)
	calls := []string{}
	yyEmojiCallerStub(t, func(action string, params map[string]any) (any, error) {
		calls = append(calls, action)
		if action == "fetch_emoji_list" {
			return map[string]any{"result": []any{
				map[string]any{"emoji_id": "128077", "wording": "点赞", "url": "https://example.com/1.png"},
				map[string]any{"id": "128078", "name": "狗头"},
			}}, nil
		}
		return nil, fmt.Errorf("unsupported action")
	})
	if e := yyEmojiIndexRefresh(); e != nil {
		t.Fatalf("refresh 失败: %v", e)
	}
	if len(calls) != 1 || calls[0] != "fetch_emoji_list" {
		t.Fatalf("应只调第一个可用 action: %v", calls)
	}
	yyEmojiIndex.Lock()
	entries := append([]yyQQEmojiEntry(nil), yyEmojiIndex.Cache.Entries...)
	source := yyEmojiIndex.Cache.Source
	fetched := yyEmojiIndex.Cache.FetchedAt
	yyEmojiIndex.Unlock()
	if source != "fetch_emoji_list" || len(entries) != 2 || fetched == 0 {
		t.Fatalf("索引状态不对: source=%q entries=%d fetched=%d", source, len(entries), fetched)
	}
	if entries[0].ID != "128077" || entries[0].Wording != "点赞" || entries[0].URL != "https://example.com/1.png" {
		t.Fatalf("字段解析不对: %+v", entries[0])
	}
	if entries[1].ID != "128078" || entries[1].Wording != "狗头" {
		t.Fatalf("兼容字段名解析不对: %+v", entries[1])
	}
	// 缓存落盘 + round-trip
	raw, e := os.ReadFile(filepath.Join(dir, "qq-emoji.json"))
	if e != nil {
		t.Fatalf("缓存未落盘: %v", e)
	}
	var cache yyQQEmojiCache
	if json.Unmarshal(raw, &cache) != nil || cache.Source != "fetch_emoji_list" || len(cache.Entries) != 2 || cache.FetchedAt == 0 {
		t.Fatalf("缓存内容不对: %s", raw)
	}
	yyEmojiInit(dir)
	block := yyEmojiIndexBlock(0)
	if !strings.Contains(block, "1.点赞") || !strings.Contains(block, "2.狗头") {
		t.Fatalf("prompt 块不对: %q", block)
	}
	if n := utf8.RuneCountInString(block); n > 500 {
		t.Fatalf("块超长: %d 字", n)
	}
}

func TestYYEmojiRefreshFallsBackToCustomFace(t *testing.T) {
	yyMemberReset(t)
	yyEmojiCallerStub(t, func(action string, params map[string]any) (any, error) {
		if action == "fetch_emoji_list" {
			return nil, fmt.Errorf("action not found")
		}
		return []any{"https://example.com/a.jpg", "https://example.com/b.jpg"}, nil
	})
	if e := yyEmojiIndexRefresh(); e != nil {
		t.Fatalf("应退化到 fetch_custom_face: %v", e)
	}
	yyEmojiIndex.Lock()
	source, count := yyEmojiIndex.Cache.Source, len(yyEmojiIndex.Cache.Entries)
	yyEmojiIndex.Unlock()
	if source != "fetch_custom_face" || count != 2 {
		t.Fatalf("退化结果不对: source=%q count=%d", source, count)
	}
	if got := yyEmojiIndexBlock(0); !strings.Contains(got, "1.表情1") {
		t.Fatalf("字符串 URL 元素应编号列出: %q", got)
	}
}

func TestYYEmojiRefreshFailureKeepsCache(t *testing.T) {
	dir := yyMemberReset(t)
	if e := yyEmojiManualEdit(true, yyQQEmojiEntry{ID: "e1", Wording: "微笑"}); e != nil {
		t.Fatal(e)
	}
	if e := yyEmojiManualEdit(true, yyQQEmojiEntry{ID: "e2", Wording: "狗头"}); e != nil {
		t.Fatal(e)
	}
	yyEmojiCallerStub(t, func(action string, params map[string]any) (any, error) {
		return nil, fmt.Errorf("OneBot 尚未连接")
	})
	if e := yyEmojiIndexRefresh(); e == nil {
		t.Fatal("全部 action 失败应返回 error")
	}
	yyEmojiIndex.Lock()
	count, lastErr := len(yyEmojiIndex.Cache.Entries), yyEmojiIndex.LastError
	yyEmojiIndex.Unlock()
	if count != 2 {
		t.Fatalf("失败不清空旧缓存: count=%d", count)
	}
	if !strings.Contains(lastErr, "OneBot 尚未连接") {
		t.Fatalf("last_error 应记录真实原因: %q", lastErr)
	}
	if got := yyEmojiIndexBlock(0); !strings.Contains(got, "1.微笑") || !strings.Contains(got, "2.狗头") {
		t.Fatalf("旧索引应仍对模型可见: %q", got)
	}
	// 落盘的旧缓存重载后仍在
	yyEmojiInit(dir)
	if got := yyEmojiIndexBlock(0); !strings.Contains(got, "1.微笑") {
		t.Fatalf("重载后旧索引丢失: %q", got)
	}
}

func TestYYEmojiManualModeFromScratch(t *testing.T) {
	dir := yyMemberReset(t)
	yyEmojiCallerStub(t, func(action string, params map[string]any) (any, error) {
		return nil, fmt.Errorf("OneBot 尚未连接")
	})
	if e := yyEmojiIndexRefresh(); e == nil {
		t.Fatal("全新安装 + 无连接应返回 error")
	}
	yyEmojiIndex.Lock()
	source, count := yyEmojiIndex.Cache.Source, len(yyEmojiIndex.Cache.Entries)
	yyEmojiIndex.Unlock()
	if source != "manual" || count != 0 {
		t.Fatalf("应降级手动模式: source=%q count=%d", source, count)
	}
	// 手动增（ID 省略时用名称充当）
	if e := yyEmojiManualEdit(true, yyQQEmojiEntry{Wording: "摸摸头"}); e != nil {
		t.Fatal(e)
	}
	yyEmojiIndex.Lock()
	entries := append([]yyQQEmojiEntry(nil), yyEmojiIndex.Cache.Entries...)
	yyEmojiIndex.Unlock()
	if len(entries) != 1 || entries[0].ID != "摸摸头" || entries[0].Wording != "摸摸头" {
		t.Fatalf("ID 省略时应用名称充当: %+v", entries)
	}
	// 同 ID 覆盖不重复
	if e := yyEmojiManualEdit(true, yyQQEmojiEntry{ID: "摸摸头", Wording: "摸摸头 v2"}); e != nil {
		t.Fatal(e)
	}
	yyEmojiIndex.Lock()
	entries = append([]yyQQEmojiEntry(nil), yyEmojiIndex.Cache.Entries...)
	yyEmojiIndex.Unlock()
	if len(entries) != 1 || entries[0].Wording != "摸摸头 v2" {
		t.Fatalf("同 ID 应覆盖: %+v", entries)
	}
	// 删
	if e := yyEmojiManualEdit(false, yyQQEmojiEntry{ID: "摸摸头"}); e != nil {
		t.Fatal(e)
	}
	yyEmojiIndex.Lock()
	count = len(yyEmojiIndex.Cache.Entries)
	yyEmojiIndex.Unlock()
	if count != 0 {
		t.Fatalf("删除后应为空: %d", count)
	}
	if e := yyEmojiManualEdit(false, yyQQEmojiEntry{ID: "没有这个"}); e == nil {
		t.Fatal("删不存在的表情应报错")
	}
	// 手动模式重载后仍保持
	yyEmojiInit(dir)
	yyEmojiIndex.Lock()
	source, count = yyEmojiIndex.Cache.Source, len(yyEmojiIndex.Cache.Entries)
	yyEmojiIndex.Unlock()
	if source != "manual" || count != 0 {
		t.Fatalf("重载后手动模式应保持: source=%q count=%d", source, count)
	}
}

func TestYYMemberRoutesAPI(t *testing.T) {
	dir := yyMemberReset(t)
	mux := http.NewServeMux()
	yyMemberRoutes(mux, dir) // 注册即初始化（重载空库）
	yyMemberFeed(t, yyMemberEvent("1001", "10001", "777", "张三", "嗨"))
	yyMemberFeed(t, yyMemberEvent("1001", "10002", "777", "李四", "嗨"))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	get := func(path string) map[string]any {
		t.Helper()
		resp, e := http.Get(srv.URL + path)
		if e != nil {
			t.Fatalf("GET %s: %v", path, e)
		}
		return yyDecodeJSON(t, resp)
	}
	post := func(path, body string) map[string]any {
		t.Helper()
		resp, e := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
		if e != nil {
			t.Fatalf("POST %s: %v", path, e)
		}
		return yyDecodeJSON(t, resp)
	}

	members := get("/api/qq/members?group=1001")
	if members["ok"] != true || members["count"].(float64) != 2 {
		t.Fatalf("成员列表不对: %v", members)
	}
	if get("/api/qq/members?group=999")["count"].(float64) != 0 {
		t.Fatal("空群应返回 0")
	}
	added := post("/api/qq/members/alias", `{"group":"1001","qq":10001,"alias":"小张","action":"add"}`)
	if added["ok"] != true || added["display_name"] != "小张" {
		t.Fatalf("增别名不对: %v", added)
	}
	if qq, ok := yyMemberFind("1001", "小张"); !ok || qq != 10001 {
		t.Fatalf("新别名应可匹配: %d %v", qq, ok)
	}
	del := post("/api/qq/members/alias", `{"group":"1001","qq":10001,"alias":"小张","action":"del"}`)
	if del["ok"] != true || del["display_name"] != "张三" {
		t.Fatalf("删别名不对: %v", del)
	}
	bad := post("/api/qq/members/alias", `{"group":"1001","qq":10001,"alias":"不存在","action":"del"}`)
	if bad["ok"] != false {
		t.Fatalf("删不存在的别名应失败: %v", bad)
	}
	badQQ := post("/api/qq/members/alias", `{"group":"1001","qq":"abc","alias":"x"}`)
	if badQQ["ok"] != false {
		t.Fatalf("非法 QQ 号应失败: %v", badQQ)
	}
	if resp, e := http.Post(srv.URL+"/api/qq/members", "application/json", strings.NewReader("{}")); e == nil {
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("GET 接口应拒绝 POST: %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	emoji := get("/api/qq/emoji")
	if emoji["ok"] != true || emoji["manual"] != true || emoji["count"].(float64) != 0 {
		t.Fatalf("空索引应报手动模式: %v", emoji)
	}
	manual := post("/api/qq/emoji", `{"action":"add","id":"e1","wording":"微笑"}`)
	if manual["ok"] != true || manual["count"].(float64) != 1 {
		t.Fatalf("手动增表情不对: %v", manual)
	}
	emoji = get("/api/qq/emoji")
	if emoji["count"].(float64) != 1 || emoji["source"] != "manual" {
		t.Fatalf("手动模式状态不对: %v", emoji)
	}
	// 刷新失败：如实降级且不丢手动条目
	yyEmojiCallerStub(t, func(action string, params map[string]any) (any, error) {
		return nil, fmt.Errorf("OneBot 尚未连接")
	})
	failed := post("/api/qq/emoji/refresh", "{}")
	if failed["ok"] != false || failed["degraded_to_manual"] != true || failed["kept_entries"].(float64) != 1 {
		t.Fatalf("刷新失败响应不对: %v", failed)
	}
	emoji = get("/api/qq/emoji")
	if emoji["count"].(float64) != 1 {
		t.Fatalf("刷新失败后条目应保留: %v", emoji)
	}
	if last, _ := emoji["last_error"].(string); !strings.Contains(last, "OneBot 尚未连接") {
		t.Fatalf("last_error 应透出: %v", emoji)
	}
}
