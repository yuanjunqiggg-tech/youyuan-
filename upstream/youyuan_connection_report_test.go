package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 真实日志行（从 /var/lib/youyuan/connection-errors.log 原样抽取）。
func TestDropReasonKindOnRealLogLines(t *testing.T) {
	cases := []struct{ line, want string }{
		{`cause=" minecraft 172.24.62.207:41029->1.95.233.223:11863: use of closed network connection"`, "conn_closed"},
		{`cause="%disconnect.kicked.reason §c你已被封禁：5分钟（受限管理员上限）`, "banned"},
		{`cause="use of closed network connection"`, "conn_closed"},
		{`msg="RakNet remote disconnect notification"`, "server_kicked"},
		{`msg="RakNet idle timeout" src=dialer idle=30.377033003s`, "idle_timeout"},
		{`msg="handle packet: read udp 172.24.62.207:33415->1.95.233.223:11863: read: connection refused"`, "server_unreachable"},
		{`msg="write to: write udp 172.24.62.207:44279->1.95.233.223:11863: write: network is unreachable"`, "local_network"},
		{``, "unknown"},
	}
	for _, c := range cases {
		if got := yyDropReasonKind(c.line); got != c.want {
			t.Fatalf("yyDropReasonKind(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestLogFieldAndTimeOnRealLines(t *testing.T) {
	line := `time=2026-10-23T18:04:07.532+08:00 level=WARN msg="connection closed" src=dialer raddr=1.95.233.223:11863 cause=" minecraft 172.24.62.207:41029->1.95.233.223:11863: use of closed network connection"`
	at, ok := yyLogTime(line)
	if !ok {
		t.Fatal("yyLogTime failed on a real log line")
	}
	if want := time.Date(2026, 10, 23, 18, 4, 7, 532000000, time.FixedZone("CST", 8*3600)); !at.Equal(want) {
		t.Fatalf("parsed time = %v, want %v", at, want)
	}
	if got := yyLogField(line, "cause="); got != "minecraft 172.24.62.207:41029->1.95.233.223:11863: use of closed network connection" {
		t.Fatalf("cause = %q", got)
	}
	if got := yyLogField(line, "level="); got != "WARN" {
		t.Fatalf("level = %q", got)
	}
}

func TestParseConnectionLogExtractsDropEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connection-errors.log")
	body := `time=2026-10-23T17:50:54.158+08:00 level=ERROR msg="handle packet: read udp 172.24.62.207:33415->1.95.233.223:11863: read: connection refused"
time=2026-10-23T17:51:11.034+08:00 level=ERROR msg="read packet: 解码数据包 *packet.GameRulesChanged 失败：枚举类型 '100' 的未知值"
time=2026-10-23T17:55:21.847+08:00 level=WARN msg="connection closed" src=dialer cause="%disconnect.kicked.reason §c你已被封禁：5分钟（受限管理员上限）。"
time=2026-10-23T18:04:07.532+08:00 level=WARN msg="connection closed" src=dialer cause=" minecraft 172.24.62.207:41029->1.95.233.223:11863: use of closed network connection"
time=2026-10-23T18:04:31.819+08:00 level=ERROR msg="read packet: 解码数据包 *packet.BiomeDefinitionList 失败：字符串长度超出 32 位整数范围"
`
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("YY_AUTH_CONFIG_DIR", dir)
	rows := yyParseConnectionLog(0)
	// 解码失败的两行不是掉线事件，必须被过滤掉。
	if len(rows) != 3 {
		t.Fatalf("parsed %d rows, want 3: %+v", len(rows), rows)
	}
	if rows[0]["kind"] != "refused" || rows[0]["reason"] != "server_unreachable" {
		t.Fatalf("row0 = %+v", rows[0])
	}
	if rows[1]["reason"] != "banned" || rows[1]["label"] != "被服务器封禁" {
		t.Fatalf("row1 = %+v", rows[1])
	}
	if rows[2]["reason"] != "conn_closed" {
		t.Fatalf("row2 = %+v", rows[2])
	}
	// 时间必须升序，前端按新到旧展示。
	for i := 1; i < len(rows); i++ {
		if rows[i-1]["at"].(string) >= rows[i]["at"].(string) {
			t.Fatalf("rows not sorted ascending: %v", rows)
		}
	}
}

func TestNoteConnectionSampleDetectsDropAndRecovery(t *testing.T) {
	// 保存并还原全局状态，避免影响其它测试。
	yyDrops.Lock()
	savedEvents, savedTotal, savedToday, savedCnt := yyDrops.events, yyDrops.total, yyDrops.today, yyDrops.todayCnt
	savedConn, savedWas, savedPending, savedLast, savedBest := yyDrops.connectedAt, yyDrops.wasConnected, yyDrops.pending, yyDrops.lastAt, yyDrops.best
	yyDrops.Unlock()
	defer func() {
		yyDrops.Lock()
		yyDrops.events, yyDrops.total, yyDrops.today, yyDrops.todayCnt = savedEvents, savedTotal, savedToday, savedCnt
		yyDrops.connectedAt, yyDrops.wasConnected, yyDrops.pending, yyDrops.lastAt, yyDrops.best = savedConn, savedWas, savedPending, savedLast, savedBest
		yyDrops.Unlock()
	}()

	base := time.Now()
	yyNoteConnectionSample(true, "98014267")
	yyDrops.Lock()
	yyDrops.connectedAt = base // 固定起点，让 uptime 可断言
	yyDrops.Unlock()

	yyNoteConnectionSample(true, "98014267") // 持续在线不应计数
	yyDrops.Lock()
	if yyDrops.total != 0 {
		yyDrops.Unlock()
		t.Fatalf("total = %d, want 0", yyDrops.total)
	}
	yyDrops.Unlock()

	// 掉线：必须记一笔，且带上本次连接的持续时长。
	yyDrops.Lock()
	yyDrops.connectedAt = base
	yyDrops.Unlock()
	yyNoteConnectionSample(false, "")
	yyDrops.Lock()
	if yyDrops.total != 1 || len(yyDrops.events) != 1 {
		yyDrops.Unlock()
		t.Fatalf("total = %d events = %d, want 1/1", yyDrops.total, len(yyDrops.events))
	}
	if yyDrops.events[0].UptimeSeconds < 0 {
		yyDrops.Unlock()
		t.Fatalf("uptime = %v", yyDrops.events[0].UptimeSeconds)
	}
	yyDrops.Unlock()

	// 恢复：结算 away 时长。
	yyNoteConnectionSample(true, "98014267")
	yyDrops.Lock()
	if yyDrops.events[0].AwaySeconds < 0 {
		yyDrops.Unlock()
		t.Fatalf("away = %v", yyDrops.events[0].AwaySeconds)
	}
	if yyDrops.pending != -1 {
		yyDrops.Unlock()
		t.Fatalf("pending = %d, want -1", yyDrops.pending)
	}
	yyDrops.Unlock()
}

func TestAttachLogCausesFillsUnknownReason(t *testing.T) {
	at := time.Date(2026, 10, 23, 18, 4, 7, 0, time.UTC)
	events := []yyDropEvent{{At: at, Reason: "unknown"}}
	logRows := []map[string]any{
		{"at": "2026-10-23T18:04:08Z", "reason": "conn_closed"},
		{"at": "2026-10-23T12:00:00Z", "reason": "server_kicked"},
	}
	got := yyAttachLogCauses(events, logRows)
	if got[0].Reason != "conn_closed" {
		t.Fatalf("reason = %q, want conn_closed", got[0].Reason)
	}
	// 时间差超过 90 秒时不得硬凑原因。
	far := []yyDropEvent{{At: at, Reason: "unknown"}}
	got = yyAttachLogCauses(far, []map[string]any{{"at": "2026-10-23T19:30:00Z", "reason": "banned"}})
	if got[0].Reason != "unknown" {
		t.Fatalf("reason = %q, want unknown (time gap too large)", got[0].Reason)
	}
}

func TestDropHintsNonEmpty(t *testing.T) {
	for _, kind := range []string{"banned", "server_kicked", "server_unreachable", "idle_timeout", "local_network", "conn_closed", "unknown", "connection_error"} {
		if yyDropHint(kind) == "" {
			t.Fatalf("missing hint for %q", kind)
		}
		if yyDropLabel(kind) == "" {
			t.Fatalf("missing label for %q", kind)
		}
	}
}

// 重启后内存计数器清零，今日统计必须回退到日志，否则误报"今日暂无掉线"。
func TestLogTodayStatsIgnoresOtherDays(t *testing.T) {
	rows := []map[string]any{
		{"at": "2026-10-21T17:32:07+08:00", "reason": "server_kicked"},
		{"at": "2026-10-22T20:59:53+08:00", "reason": "server_kicked"},
		{"at": "2026-10-23T00:59:31+08:00", "reason": "server_unreachable"},
		{"at": "2026-10-23T04:12:45+08:00", "reason": "conn_closed"},
		{"at": "2026-10-23T17:55:21+08:00", "reason": "banned"},
		{"at": "2026-10-23T18:04:07+08:00", "reason": "conn_closed"},
		{"at": "bad-timestamp", "reason": "conn_closed"},
	}
	count, byReason, lastAt := yyLogTodayStats(rows, "2026-10-23")
	if count != 4 {
		t.Fatalf("today count = %d, want 4", count)
	}
	if byReason["conn_closed"] != 2 || byReason["banned"] != 1 || byReason["server_unreachable"] != 1 {
		t.Fatalf("byReason = %v", byReason)
	}
	if want := time.Date(2026, 10, 23, 18, 4, 7, 0, time.FixedZone("CST", 8*3600)); !lastAt.Equal(want) {
		t.Fatalf("lastAt = %v, want %v", lastAt, want)
	}
	// 非今日必须零命中。
	if c, _, _ := yyLogTodayStats(rows, "2026-10-24"); c != 0 {
		t.Fatalf("other-day count = %d, want 0", c)
	}
	if got := yyLastLogReasonToday(rows, "2026-10-23"); got != "conn_closed" {
		t.Fatalf("yyLastLogReasonToday = %q, want conn_closed", got)
	}
	if got := yyLastLogReasonToday(rows, "2026-10-24"); got != "" {
		t.Fatalf("yyLastLogReasonToday(other day) = %q, want empty", got)
	}
}

// 日志超过尾部上限时只读尾部，但今天的统计不能丢。
func TestParseConnectionLogTailKeepsToday(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connection-errors.log")
	var b []byte
	// 先灌 600KB 的历史（超过 512KB 尾部上限），再写今天的事件。
	filler := "time=2026-10-01T00:00:00+08:00 level=ERROR msg=\"read packet: 解码数据包 *packet.BiomeDefinitionList 失败：字符串长度超出 32 位整数范围\"\n"
	for len(b) < 600*1024 {
		b = append(b, filler...)
	}
	b = append(b, "time=2026-10-23T18:04:07+08:00 level=WARN msg=\"connection closed\" src=dialer cause=\" minecraft 1.2.3.4:1->5.6.7.8:2: use of closed network connection\"\n"...)
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("YY_AUTH_CONFIG_DIR", dir)
	rows := yyParseConnectionLog(0)
	if len(rows) == 0 {
		t.Fatal("tail read lost today's event")
	}
	if rows[len(rows)-1]["reason"] != "conn_closed" {
		t.Fatalf("last row = %+v", rows[len(rows)-1])
	}
}
