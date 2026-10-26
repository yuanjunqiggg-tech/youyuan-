package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 所有测试只碰内存状态与可注入时钟：不发 QQ 消息、不连游戏、不写真实数据目录。

// yyAffFixture 隔离全局好感状态，并返回可推进的冻结时钟。
func yyAffFixture(t *testing.T) *time.Time {
	t.Helper()
	oldPath, oldPeople, oldErr := yyAff.Path, yyAff.People, yyAff.DiskError
	oldNow, oldQuota, oldPokes, oldRepeats := yyAffNow, yyAffChatQuota.used, yyAffPokes.last, yyAffRepeats.last
	t.Cleanup(func() {
		yyAff.Path, yyAff.People, yyAff.DiskError = oldPath, oldPeople, oldErr
		yyAffNow = oldNow
		yyAffChatQuota.used, yyAffPokes.last, yyAffRepeats.last = oldQuota, oldPokes, oldRepeats
	})
	yyAff.Path = ""
	yyAff.People = map[string]yyAffState{}
	yyAff.DiskError = ""
	yyAffChatQuota.used = map[string]int{}
	yyAffPokes.last = map[string]yyAffPoke{}
	yyAffRepeats.last = map[string]yyAffRepeat{}
	now := time.Unix(1700000000, 0)
	yyAffNow = func() time.Time { return now }
	return &now
}

// 夸它 → 好感上涨；普通聊天每日封顶 +5。
func TestQQAffPraiseRaisesAndChatCapped(t *testing.T) {
	yyAffFixture(t)
	if st := yyAffOnMessage("12345", "小明", "你好厉害啊"); st.Affinity != 3 || st.LastEvent != "praise" {
		t.Fatal("夸奖未加分", st)
	}
	if st := yyAffOnMessage("12345", "小明", "好棒"); st.Affinity != 6 || st.Mood != yyAffMoodNeutral {
		t.Fatal("二次夸奖不对", st)
	}
	// 普通聊天：每天最多 +5。
	for i := 0; i < 10; i++ {
		yyAffOnMessage("777", "阿强", "今天天气不错"+strings.Repeat("啊", i))
	}
	if st, _ := yyAffStateOf("777"); st.Affinity != 5 {
		t.Fatal("普通聊天未封顶在 +5", st)
	}
	// 次日恢复额度。
	yyAffNow = func() time.Time { return time.Unix(1700000000, 0).Add(24 * time.Hour) }
	if st := yyAffOnMessage("777", "阿强", "第二天了"); st.Affinity != 6 {
		t.Fatal("跨天未恢复聊天额度", st)
	}
}

// 骂它 → 好感下降且情绪至少 angry（即使总分还没到 -60）。
func TestQQAffInsultDropsToAngry(t *testing.T) {
	clock := yyAffFixture(t)
	st := yyAffObserve("12345", "小明", "insult")
	if st.Affinity != -5 || st.Mood != yyAffMoodAngry {
		t.Fatal("被骂后应愤怒", st)
	}
	// 余怒窗口内继续掉分。
	for i := 0; i < 2; i++ {
		st = yyAffObserve("12345", "小明", "insult")
	}
	if st.Affinity != -15 || st.Mood != yyAffMoodAngry {
		t.Fatal("多次辱骂后情绪不对", st)
	}
	// 过了 5 分钟余怒消失，按分数落到 annoyed；情绪在下次事件/衰减时重算。
	*clock = clock.Add(yyAffInsultWindow + time.Minute)
	st = yyAffObserve("12345", "小明", "ignore")
	if st.Affinity != -16 || st.Mood != yyAffMoodAnnoyed {
		t.Fatal("余怒过期后应按分数推导情绪", st)
	}
	// 好感极低 → 黑化。
	yyAff.People["12345"] = yyAffState{QQ: "12345", Name: "小明", Affinity: -70, LastAt: *clock}
	if st := yyAffObserve("12345", "小明", "chat"); st.Mood != yyAffMoodCold {
		t.Fatal("低于 -60 应黑化", st)
	}
}

// 连续拍一拍：前 3 次 +1，第 4 次起烦了 -1。
func TestQQAffPokeStreakRaisesThenDrops(t *testing.T) {
	clock := yyAffFixture(t)
	want := []int{1, 2, 3, 2, 1}
	for i, expected := range want {
		*clock = clock.Add(10 * time.Second) // 同一 2 分钟窗口内连拍
		st := yyAffObserve("12345", "小明", "poke")
		if st.Affinity != expected {
			t.Fatalf("第 %d 次拍一拍好感 %d，期望 %d", i+1, st.Affinity, expected)
		}
	}
	// 间隔超过窗口后重新计数：又是亲昵的 +1。
	*clock = clock.Add(yyAffPokeWindow + time.Second)
	if st := yyAffObserve("12345", "小明", "poke"); st.Affinity != 2 {
		t.Fatal("窗口外应重置连拍计数", st)
	}
}

// 衰减：向 0 收敛，每次最多 1 点且不跨过 0。
func TestQQAffDecayTowardZero(t *testing.T) {
	clock := yyAffFixture(t)
	yyAff.People["111"] = yyAffState{QQ: "111", Affinity: 50, Mood: yyAffMoodNeutral, LastAt: *clock}
	yyAff.People["222"] = yyAffState{QQ: "222", Affinity: -50, Mood: yyAffMoodNeutral, LastAt: *clock}
	yyAff.People["333"] = yyAffState{QQ: "333", Affinity: 1, Mood: yyAffMoodNeutral, LastAt: *clock}
	yyAff.People["444"] = yyAffState{QQ: "444", Affinity: 0, Mood: yyAffMoodNeutral, LastAt: *clock}
	if n := yyAffDecay(); n != 3 {
		t.Fatal("应只对 3 个非零好感计数", n)
	}
	for qq, want := range map[string]int{"111": 49, "222": -49, "333": 0, "444": 0} {
		if st, _ := yyAffStateOf(qq); st.Affinity != want {
			t.Fatalf("%s 衰减后 %d，期望 %d", qq, st.Affinity, want)
		}
	}
	// 0 不会反向跑。
	yyAffDecay()
	if st, _ := yyAffStateOf("333"); st.Affinity != 0 {
		t.Fatal("0 好感衰减后不应变成负数", st)
	}
}

// 人数上限：超过 2000 人淘汰 LastAt 最久的一条。
func TestQQAffEvictsOldestOverCap(t *testing.T) {
	yyAffFixture(t)
	base := time.Unix(1700000000, 0)
	for i := 0; i < yyAffMaxPeople; i++ {
		qq := "user-" + strings.Repeat("0", 4-len(itoaSmall(i))) + itoaSmall(i)
		yyAff.People[qq] = yyAffState{
			QQ: qq, Affinity: 0, Mood: yyAffMoodNeutral,
			LastAt: base.Add(time.Duration(i) * time.Second),
		}
	}
	if yyAffTotal() != yyAffMaxPeople {
		t.Fatal("初始人数不对", yyAffTotal())
	}
	yyAffNow = func() time.Time { return base.Add(time.Hour) }
	yyAffObserve("newcomer", "新人", "praise")
	if yyAffTotal() != yyAffMaxPeople {
		t.Fatal("超上限后人数未收敛", yyAffTotal())
	}
	if _, ok := yyAffStateOf("user-0000"); ok {
		t.Fatal("最久没互动的人应被淘汰")
	}
	if st, ok := yyAffStateOf("newcomer"); !ok || st.Affinity != 3 {
		t.Fatal("新人应被保留", st, ok)
	}
}

// 黑化提示词必须带安全底线：不得辱骂、仍提供事实性帮助。
func TestQQAffColdPromptKeepsSafetyBottomLine(t *testing.T) {
	clock := yyAffFixture(t)
	yyAff.People["999"] = yyAffState{QQ: "999", Name: "阿强", Affinity: -78, Mood: yyAffMoodCold, LastAt: *clock}
	prompt := yyAffPromptFor("999")
	for _, must := range []string{"阿强", "-78", "不得辱骂", "事实性帮助", "不主动示好"} {
		if !strings.Contains(prompt, must) {
			t.Fatalf("黑化提示词缺少 %q：%s", must, prompt)
		}
	}
	// 刚被骂过：即使历史好感很高，也应至少降到 angry。
	recent := yyAffState{QQ: "888", Name: "小红", Affinity: 80, LastInsultAt: *clock}
	if mood := yyAffMoodOf(recent.Affinity, yyAffRecentInsult(recent, *clock)); mood != yyAffMoodAngry {
		t.Fatal("余怒窗口内应至少 angry", mood)
	}
	// 未记录的人也有中性文案。
	if got := yyAffPromptFor("404"); !strings.Contains(got, "还不了解") || !strings.Contains(got, "不得辱骂") {
		t.Fatal("陌生人提示词不对", got)
	}
}

// 复读：短时间内同一句第二次起算 repeat（-2），并可覆盖正/负面词的加分。
func TestQQAffRepeatDetected(t *testing.T) {
	yyAffFixture(t)
	if st := yyAffOnMessage("12345", "小明", "谢谢"); st.Affinity != 2 || st.LastEvent != "thanks" {
		t.Fatal("首次致谢不对", st)
	}
	if st := yyAffOnMessage("12345", "小明", "谢谢"); st.Affinity != 0 || st.LastEvent != "repeat" {
		t.Fatal("复读未被识别", st)
	}
	// 超过复读窗口后重新按 thanks 计。
	yyAffNow = func() time.Time { return time.Unix(1700000000, 0).Add(yyAffRepeatWindow + time.Minute) }
	if st := yyAffOnMessage("12345", "小明", "谢谢"); st.Affinity != 2 || st.LastEvent != "thanks" {
		t.Fatal("窗口外复读应重新计为致谢", st)
	}
}

// 持久化：round-trip 恢复；损坏文件不崩、退回空表。
func TestQQAffPersistenceRoundTripAndCorruptFile(t *testing.T) {
	yyAffFixture(t)
	dir := t.TempDir()
	yyAffInit(dir)
	yyAffObserve("12345", "小明", "praise")
	yyAffObserve("12345", "小明", "insult")
	yyAffObserve("67890", "小红", "thanks")
	yyAffInit(dir) // 重新载入
	if st, ok := yyAffStateOf("12345"); !ok || st.Affinity != -2 || st.Name != "小明" || st.Mood != yyAffMoodAngry {
		t.Fatal("round-trip 丢失或情绪未重算", st, ok)
	}
	if st, ok := yyAffStateOf("67890"); !ok || st.Affinity != 2 {
		t.Fatal("第二人 round-trip 丢失", st, ok)
	}
	raw, e := os.ReadFile(filepath.Join(dir, "qq-affinity.json"))
	if e != nil || !strings.Contains(string(raw), "小明") {
		t.Fatal("落盘文件内容不对", e, string(raw))
	}
	// 坏文件：不崩，空表 + DiskError。
	if e := os.WriteFile(filepath.Join(dir, "qq-affinity.json"), []byte("{broken"), 0600); e != nil {
		t.Fatal(e)
	}
	yyAffInit(dir)
	if yyAffTotal() != 0 || yyAff.DiskError == "" {
		t.Fatal("坏文件未安全回落", yyAffTotal(), yyAff.DiskError)
	}
}

// 面板接口：GET 单个/榜单、POST 手动校正并夹到 -100..100、方法限制。
func TestQQAffRoutesAPI(t *testing.T) {
	yyAffFixture(t)
	dir := t.TempDir()
	mux := http.NewServeMux()
	yyAffRoutes(mux, dir)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/qq/affinity", strings.NewReader(body)))
		return w
	}
	// POST 校正越界值被夹住。
	if w := post(`{"qq":"12345","name":"小明","affinity":-150}`); !strings.Contains(w.Body.String(), `"ok":true`) || !strings.Contains(w.Body.String(), `"affinity":-100`) {
		t.Fatal("POST 未夹到 -100", w.Body.String())
	}
	if w := post(`{"qq":"12345","affinity":100}`); !strings.Contains(w.Body.String(), `"affinity":100`) {
		t.Fatal("POST 未夹到 100", w.Body.String())
	}
	// GET 单个。
	w := get("/api/qq/affinity?qq=12345")
	if !strings.Contains(w.Body.String(), `"found":true`) || !strings.Contains(w.Body.String(), `"name":"小明"`) || !strings.Contains(w.Body.String(), "prompt") {
		t.Fatal("GET 单个不对", w.Body.String())
	}
	// 未记录的人。
	if w := get("/api/qq/affinity?qq=404"); !strings.Contains(w.Body.String(), `"found":false`) {
		t.Fatal("GET 未知用户不对", w.Body.String())
	}
	// 榜单：好感降序 + 总人数。
	post(`{"qq":"222","name":"小红","affinity":60}`)
	w = get("/api/qq/affinity")
	if !strings.Contains(w.Body.String(), `"total":2`) || !strings.Contains(w.Body.String(), `"qq":"222"`) {
		t.Fatal("GET 榜单不对", w.Body.String())
	}
	if strings.Index(w.Body.String(), `"qq":"12345"`) > strings.Index(w.Body.String(), `"qq":"222"`) {
		t.Fatal("榜单未按好感降序", w.Body.String())
	}
	// 缺 qq 拒绝；方法限制。
	if w := post(`{"affinity":1}`); strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatal("缺 qq 被接受", w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/qq/affinity", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal("DELETE 应被拒绝", w.Code)
	}
}

// itoaSmall 测试内小整数转字符串，避免为造数据引入 strconv 依赖噪音。
func itoaSmall(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
