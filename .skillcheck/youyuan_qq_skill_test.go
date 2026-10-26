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

// yyTestBuiltinEntries 把内置表转成整体保存用的条目（不带可改字段补丁）。
func yyTestBuiltinEntries() []yySkillEntry {
	builtins := yyGameSkillBuiltins()
	out := make([]yySkillEntry, 0, len(builtins))
	for _, b := range builtins {
		out = append(out, yySkillEntry{Skill: b})
	}
	return out
}

// 1. 内置表完整性与 Name 唯一。
func TestYYGameSkillBuiltinTableComplete(t *testing.T) {
	skills := yyGameSkillBuiltins()
	if len(skills) != 12 {
		t.Fatalf("expected 12 builtin skills, got %d", len(skills))
	}
	names := map[string]bool{}
	for _, s := range skills {
		if s.Name == "" || s.Label == "" || s.Desc == "" || s.Category == "" {
			t.Fatalf("builtin skill incomplete: %+v", s)
		}
		if names[s.Name] {
			t.Fatalf("duplicate builtin name: %s", s.Name)
		}
		names[s.Name] = true
		if !s.Builtin || !s.Enabled {
			t.Fatalf("builtin flags wrong: %s", s.Name)
		}
		if s.Run != "console" && s.Run != "builtin" {
			t.Fatalf("builtin run must be console/builtin: %s -> %s", s.Name, s.Run)
		}
		if s.TimeoutMS <= 0 {
			t.Fatalf("builtin timeout missing: %s", s.Name)
		}
		for _, p := range s.Params {
			switch p.Type {
			case "string", "int", "player", "enum":
			default:
				t.Fatalf("builtin %s param %s bad type %s", s.Name, p.Name, p.Type)
			}
			if p.Type == "enum" && len(p.Options) == 0 {
				t.Fatalf("builtin %s param %s enum without options", s.Name, p.Name)
			}
		}
	}
	for _, want := range []string{"online_list", "server_status", "player_info", "scoreboard_count", "say", "tp", "kick", "gamemode", "time_set", "weather", "whitelist_add", "reload"} {
		if !names[want] {
			t.Fatalf("missing builtin skill: %s", want)
		}
	}
}

// 2. 参数校验失败：缺必填、坏 player 名、enum 越界、技能不存在/已停用。
func TestYYGameSkillParamValidation(t *testing.T) {
	yyGameSkillInit(t.TempDir())
	if _, e := yyGameSkillRun("say", map[string]string{}); e == nil {
		t.Fatal("missing required param should fail")
	}
	if _, e := yyGameSkillRun("whitelist_add", map[string]string{"player": "ab"}); e == nil {
		t.Fatal("short player name should fail")
	}
	if _, e := yyGameSkillRun("whitelist_add", map[string]string{"player": "坏名字"}); e == nil {
		t.Fatal("non-ascii player name should fail")
	}
	if _, e := yyGameSkillRun("weather", map[string]string{"value": "sunny"}); e == nil {
		t.Fatal("enum out of range should fail")
	}
	if _, e := yyGameSkillRun("nope", map[string]string{}); e == nil {
		t.Fatal("unknown skill should fail")
	}
	// 停用后不可执行
	entries := yyTestBuiltinEntries()
	off := false
	for i := range entries {
		if entries[i].Skill.Name == "weather" {
			entries[i].Patch.Enabled = &off
		}
	}
	if e := yyGameSkillSaveAll(entries); e != nil {
		t.Fatalf("save failed: %v", e)
	}
	if _, e := yyGameSkillRun("weather", map[string]string{"value": "rain"}); e == nil {
		t.Fatal("disabled skill should fail")
	}
}

// 3. 注入拒绝：参数与命令模板里的 \n \r 控制字符都被拦下。
func TestYYGameSkillInjectionRejected(t *testing.T) {
	dir := t.TempDir()
	yyGameSkillInit(dir)
	// 参数注入：player 带换行
	if _, e := yyGameSkillRun("kick", map[string]string{"player": "a\nop", "reason": "x"}); e == nil {
		t.Fatal("player injection should be rejected")
	}
	// 参数注入：msg 带回车，试图拼出第二条命令
	if _, e := yyGameSkillRun("say", map[string]string{"msg": "hi\r/kick steve"}); e == nil {
		t.Fatal("msg injection should be rejected")
	}
	// 命令模板级注入（模拟被篡改的数据文件）：渲染后的防注入兜底
	if e := yySkillGuardCommand("/say a\nop deop"); e == nil {
		t.Fatal("newline should be rejected by guard")
	}
	if e := yySkillGuardCommand("/say a\rb"); e == nil {
		t.Fatal("CR should be rejected by guard")
	}
	if e := yySkillGuardCommand("/say a\x00b"); e == nil {
		t.Fatal("NUL should be rejected by guard")
	}
	if e := yySkillGuardCommand("   "); e == nil {
		t.Fatal("blank command should be rejected")
	}
	// 数据文件里的恶意模板：加载后执行被防注入拦下
	evil := `{"skills":[{"name":"evil_one","builtin":false,"label":"恶意","desc":"x","run":"console","command":"/say hi\nop steve","enabled":true,"timeout_ms":3000}]}`
	if e := os.WriteFile(filepath.Join(dir, "qq-skills.json"), []byte(evil), 0600); e != nil {
		t.Fatal(e)
	}
	yyGameSkillInit(dir)
	if _, e := yyGameSkillRun("evil_one", map[string]string{}); e == nil {
		t.Fatal("evil template from disk should be rejected")
	}
}

// 4. 模板渲染正确：占位符替换、未知占位符保留。
func TestYYSkillRender(t *testing.T) {
	got := yySkillRender("/tp {from} {to}", map[string]string{"from": "alice_01", "to": "bob_02"})
	if got != "/tp alice_01 bob_02" {
		t.Fatalf("render got %q", got)
	}
	if got := yySkillRender("/say {msg}", map[string]string{}); got != "/say {msg}" {
		t.Fatalf("unknown placeholder should stay, got %q", got)
	}
	if got := yySkillRender("", map[string]string{"a": "b"}); got != "" {
		t.Fatalf("empty template got %q", got)
	}
	ph := yySkillPlaceholders("/give {player} diamond {count}")
	if len(ph) != 2 || ph[0] != "player" || ph[1] != "count" {
		t.Fatalf("placeholders got %v", ph)
	}
}

// 5. 假执行器验证命令文本与超时（严禁真连游戏）。
func TestYYGameSkillRunFakeExecutor(t *testing.T) {
	yyGameSkillInit(t.TempDir())
	origin := yySkillExecutor
	defer func() { yySkillExecutor = origin }()
	var gotCmd string
	var gotTimeout time.Duration
	yySkillExecutor = func(command string, timeout time.Duration) (string, error) {
		gotCmd = command
		gotTimeout = timeout
		return "ok", nil
	}
	out, e := yyGameSkillRun("weather", map[string]string{"value": "rain"})
	if e != nil {
		t.Fatalf("run failed: %v", e)
	}
	if out != "ok" {
		t.Fatalf("output passthrough got %q", out)
	}
	if gotCmd != "/weather rain" {
		t.Fatalf("command text got %q", gotCmd)
	}
	if gotTimeout != 3*time.Second {
		t.Fatalf("timeout got %v", gotTimeout)
	}
	// 默认参数值参与渲染
	gotCmd = ""
	if _, e := yyGameSkillRun("kick", map[string]string{"player": "steve_01"}); e != nil {
		t.Fatalf("kick failed: %v", e)
	}
	if gotCmd != "/kick steve_01 违规行为" {
		t.Fatalf("kick default reason got %q", gotCmd)
	}
	// 多参数顺序
	gotCmd = ""
	if _, e := yyGameSkillRun("tp", map[string]string{"from": "alice_01", "to": "bob_02"}); e != nil {
		t.Fatalf("tp failed: %v", e)
	}
	if gotCmd != "/tp alice_01 bob_02" {
		t.Fatalf("tp command got %q", gotCmd)
	}
	// 超时：执行器阻塞超过 TimeoutMS 时报错且不重发
	entries := append(yyTestBuiltinEntries(), yySkillEntry{Skill: yyGameSkill{
		Name: "slow_test", Label: "慢技能", Desc: "超时测试", Run: "console",
		Command: "/say {msg}", Channel: "ai", Format: "text", TimeoutMS: 100, Enabled: true,
		Params: []yyGameSkillParam{{Name: "msg", Label: "内容", Type: "string", Required: true}},
	}})
	if e := yyGameSkillSaveAll(entries); e != nil {
		t.Fatalf("save custom failed: %v", e)
	}
	yySkillExecutor = func(command string, timeout time.Duration) (string, error) {
		time.Sleep(500 * time.Millisecond)
		return "late", nil
	}
	if _, e := yyGameSkillRun("slow_test", map[string]string{"msg": "hi"}); e == nil || !strings.Contains(e.Error(), "超时") {
		t.Fatalf("timeout should be reported, got %v", e)
	}
}

// 6. CRUD 持久化 round-trip：停用内置、新增自定义、删除自定义、损坏回退。
func TestYYGameSkillCRUDRoundTrip(t *testing.T) {
	dir := t.TempDir()
	yyGameSkillInit(dir)
	entries := yyTestBuiltinEntries()
	off := false
	for i := range entries {
		if entries[i].Skill.Name == "reload" {
			entries[i].Patch.Enabled = &off
		}
	}
	entries = append(entries, yySkillEntry{Skill: yyGameSkill{
		Name: "give_diamond", Label: "发钻石", Desc: "给玩家发钻石", Category: "custom",
		Run: "console", Command: "/give {player} diamond 1", Channel: "ai",
		Params: []yyGameSkillParam{{Name: "player", Label: "游戏ID", Type: "player", Required: true}},
		Format: "text", TimeoutMS: 3000, Enabled: true,
	}})
	if e := yyGameSkillSaveAll(entries); e != nil {
		t.Fatalf("save failed: %v", e)
	}
	// 重新加载：停用状态与自定义技能都在
	yyGameSkillInit(dir)
	if s, ok := yyGameSkillLookup("reload"); !ok || s.Enabled {
		t.Fatal("reload should stay disabled after reload")
	}
	custom, ok := yyGameSkillLookup("give_diamond")
	if !ok {
		t.Fatal("custom skill lost after reload")
	}
	if custom.Command != "/give {player} diamond 1" || custom.Label != "发钻石" {
		t.Fatalf("custom fields lost: %+v", custom)
	}
	if len(custom.Params) != 1 || custom.Params[0].Type != "player" || !custom.Params[0].Required {
		t.Fatalf("custom params lost: %+v", custom.Params)
	}
	if custom.Builtin {
		t.Fatal("custom skill must not be marked builtin")
	}
	// 删除自定义技能后不再出现
	if e := yyGameSkillDelete("give_diamond"); e != nil {
		t.Fatalf("delete failed: %v", e)
	}
	yyGameSkillInit(dir)
	if _, ok := yyGameSkillLookup("give_diamond"); ok {
		t.Fatal("custom skill should be gone after delete")
	}
	// 损坏文件：回退纯内置表并记录错误，不清空用户对其它内置的停用选择之外的状态
	if e := os.WriteFile(filepath.Join(dir, "qq-skills.json"), []byte("{not json"), 0600); e != nil {
		t.Fatal(e)
	}
	yyGameSkillInit(dir)
	if _, ok := yyGameSkillLookup("say"); !ok {
		t.Fatal("builtin table should survive corrupt file")
	}
	if yyGameSkillDiskErr() == "" {
		t.Fatal("corrupt file should report disk error")
	}
	if s, _ := yyGameSkillLookup("reload"); !s.Enabled {
		t.Fatal("builtin defaults should be restored after corrupt file")
	}
}

// 7. Builtin 删除/改名被拒，且被拒后注册表不被破坏。
func TestYYGameSkillBuiltinProtected(t *testing.T) {
	yyGameSkillInit(t.TempDir())
	if e := yyGameSkillDelete("say"); e == nil {
		t.Fatal("builtin delete should be rejected")
	}
	// 整体保存时漏掉一个内置技能
	entries := yyTestBuiltinEntries()
	kept := make([]yySkillEntry, 0, len(entries))
	for _, e := range entries {
		if e.Skill.Name != "kick" {
			kept = append(kept, e)
		}
	}
	if e := yyGameSkillSaveAll(kept); e == nil {
		t.Fatal("dropping a builtin should be rejected")
	}
	// 把 say 改名成 broadcast（少了一个内置、多了一个同名模板的自定义）
	renamed := yyTestBuiltinEntries()
	for i := range renamed {
		if renamed[i].Skill.Name == "say" {
			renamed[i].Skill.Name = "broadcast"
		}
	}
	if e := yyGameSkillSaveAll(renamed); e == nil {
		t.Fatal("renaming a builtin should be rejected")
	}
	// 注册表保持原状
	if s, ok := yyGameSkillLookup("say"); !ok || !s.Enabled {
		t.Fatal("builtin say must survive rejected save")
	}
	if _, ok := yyGameSkillLookup("broadcast"); ok {
		t.Fatal("renamed builtin must not be created")
	}
	if _, ok := yyGameSkillLookup("kick"); !ok {
		t.Fatal("other builtins must survive rejected save")
	}
}

// 8. 自定义技能保存校验：非法 run、未声明占位符、重复名都被拒。
func TestYYGameSkillSaveAllValidatesCustom(t *testing.T) {
	yyGameSkillInit(t.TempDir())
	base := yyTestBuiltinEntries()
	badRun := append(append([]yySkillEntry{}, base...), yySkillEntry{Skill: yyGameSkill{
		Name: "x_skill", Label: "X", Run: "builtin", Enabled: true,
	}})
	if e := yyGameSkillSaveAll(badRun); e == nil {
		t.Fatal("custom builtin run should be rejected")
	}
	badTpl := append(append([]yySkillEntry{}, base...), yySkillEntry{Skill: yyGameSkill{
		Name: "y_skill", Label: "Y", Run: "console", Command: "/give {player} stone", Enabled: true,
	}})
	if e := yyGameSkillSaveAll(badTpl); e == nil {
		t.Fatal("undeclared placeholder should be rejected")
	}
	dup := append(append([]yySkillEntry{}, base...),
		yySkillEntry{Skill: yyGameSkill{Name: "z_skill", Label: "Z", Run: "console", Command: "/say hi", Enabled: true}},
		yySkillEntry{Skill: yyGameSkill{Name: "z_skill", Label: "Z2", Run: "console", Command: "/say hi", Enabled: true}},
	)
	if e := yyGameSkillSaveAll(dup); e == nil {
		t.Fatal("duplicate name should be rejected")
	}
	// 合法自定义可通过，且默认值被补全
	good := append(append([]yySkillEntry{}, base...), yySkillEntry{Skill: yyGameSkill{
		Name: "good_one", Label: "好技能", Desc: "测试", Run: "console", Command: "/say {msg}", Enabled: true,
		Params: []yyGameSkillParam{{Name: "msg", Label: "内容", Type: "string", Required: true}},
	}})
	if e := yyGameSkillSaveAll(good); e != nil {
		t.Fatalf("valid custom should pass: %v", e)
	}
	s, ok := yyGameSkillLookup("good_one")
	if !ok || s.Channel != "ai" || s.Format != "text" || s.TimeoutMS != 3000 || s.Builtin {
		t.Fatalf("defaults not applied: %+v", s)
	}
}

// 9. AI 入口：工具定义形状与 invoke 走注册表执行。
func TestYYSkillToolDefsAndInvoke(t *testing.T) {
	yyGameSkillInit(t.TempDir())
	defs := yySkillToolDefs()
	if len(defs) != 12 {
		t.Fatalf("expected 12 tool defs, got %d", len(defs))
	}
	seen := map[string]bool{}
	for _, d := range defs {
		fn, ok := d["function"].(map[string]any)
		if !ok || d["type"] != "function" {
			t.Fatalf("tool def shape wrong: %+v", d)
		}
		name, _ := fn["name"].(string)
		if !strings.HasPrefix(name, "skill_") || seen[name] {
			t.Fatalf("bad tool name %q", name)
		}
		seen[name] = true
		params, ok := fn["parameters"].(map[string]any)
		if !ok || params["type"] != "object" {
			t.Fatalf("missing parameters for %s", name)
		}
		if _, e := json.Marshal(d); e != nil {
			t.Fatalf("tool def not serializable: %v", e)
		}
	}
	origin := yySkillExecutor
	defer func() { yySkillExecutor = origin }()
	var gotCmd string
	yySkillExecutor = func(command string, timeout time.Duration) (string, error) {
		gotCmd = command
		return "done", nil
	}
	out, e := yySkillInvoke("skill_gamemode", map[string]any{"mode": "creative", "player": "steve_01"})
	if e != nil {
		t.Fatalf("invoke failed: %v", e)
	}
	if out != "done" {
		t.Fatalf("invoke output got %q", out)
	}
	if gotCmd != "/gamemode creative steve_01" {
		t.Fatalf("invoke command got %q", gotCmd)
	}
	// 数字参数（JSON number 解码成 float64）也要能跑
	gotCmd = ""
	if _, e := yySkillInvoke("skill_say", map[string]any{"msg": float64(42)}); e != nil {
		t.Fatalf("numeric arg failed: %v", e)
	}
	if gotCmd != "/say 42" {
		t.Fatalf("numeric arg command got %q", gotCmd)
	}
	// AI 传越权 enum 被拒，AI 无法绕过注册表
	if _, e := yySkillInvoke("skill_weather", map[string]any{"value": "sunny"}); e == nil {
		t.Fatal("bad enum should be rejected")
	}
	if _, e := yySkillInvoke("skill_not_exist", map[string]any{}); e == nil {
		t.Fatal("unknown skill should be rejected")
	}
}

// 10. 留痕：字段完整、环形上限 200。
func TestYYGameSkillLogRing(t *testing.T) {
	yyGameSkillInit(t.TempDir())
	origin := yySkillExecutor
	defer func() { yySkillExecutor = origin }()
	yySkillExecutor = func(command string, timeout time.Duration) (string, error) { return "ok", nil }
	yySkillLogs.Lock()
	yySkillLogs.Items = nil
	yySkillLogs.Unlock()
	for i := 0; i < 5; i++ {
		if _, e := yyGameSkillRun("say", map[string]string{"msg": "m"}); e != nil {
			t.Fatal(e)
		}
	}
	entries := yySkillLogView(0)
	if len(entries) != 5 {
		t.Fatalf("expected 5 log entries, got %d", len(entries))
	}
	if entries[0].Skill != "say" || !entries[0].OK || entries[0].Ms < 0 || entries[0].Out != "ok" {
		t.Fatalf("log entry fields wrong: %+v", entries[0])
	}
	if entries[0].Params["msg"] != "m" {
		t.Fatalf("log params wrong: %+v", entries[0].Params)
	}
	// 失败也留痕
	if _, e := yyGameSkillRun("say", map[string]string{}); e == nil {
		t.Fatal("missing param should fail")
	}
	if entries := yySkillLogView(1); len(entries) != 1 || entries[0].OK {
		t.Fatalf("failure should be logged: %+v", entries)
	}
	// 环形上限 200
	for i := 0; i < 210; i++ {
		_, _ = yyGameSkillRun("say", map[string]string{"msg": "x"})
	}
	if got := len(yySkillLogView(0)); got != 200 {
		t.Fatalf("ring should cap at 200, got %d", got)
	}
}

// 11. HTTP 路由：GET 列表与元数据、POST /run 校验、DELETE 内置被拒、GET /log。
func TestYYQQSkillRoutes(t *testing.T) {
	yyGameSkillInit(t.TempDir())
	mux := http.NewServeMux()
	yyQQSkillRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	r, e := http.Get(srv.URL + "/api/qq/skills")
	if e != nil {
		t.Fatal(e)
	}
	var view map[string]any
	if e := json.NewDecoder(r.Body).Decode(&view); e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if view["ok"] != true {
		t.Fatalf("GET not ok: %+v", view)
	}
	skills, _ := view["skills"].([]any)
	if len(skills) != 12 {
		t.Fatalf("expected 12 skills, got %d", len(skills))
	}
	types, _ := view["types"].([]any)
	if len(types) != 4 {
		t.Fatalf("param type metadata missing: %+v", view["types"])
	}

	// POST /run 缺必填参数被拒（校验先于执行，不会连游戏）
	r2, e := http.Post(srv.URL+"/api/qq/skills/run", "application/json", strings.NewReader(`{"name":"say","params":{}}`))
	if e != nil {
		t.Fatal(e)
	}
	var run map[string]any
	if e := json.NewDecoder(r2.Body).Decode(&run); e != nil {
		t.Fatal(e)
	}
	r2.Body.Close()
	if run["ok"] != false {
		t.Fatalf("run without params should fail: %+v", run)
	}

	// DELETE 内置技能被拒
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/qq/skills?name=say", nil)
	r3, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	var del map[string]any
	if e := json.NewDecoder(r3.Body).Decode(&del); e != nil {
		t.Fatal(e)
	}
	r3.Body.Close()
	if del["ok"] != false {
		t.Fatalf("builtin delete should be rejected: %+v", del)
	}

	// GET /log
	r4, e := http.Get(srv.URL + "/api/qq/skills/log?limit=10")
	if e != nil {
		t.Fatal(e)
	}
	var logs map[string]any
	if e := json.NewDecoder(r4.Body).Decode(&logs); e != nil {
		t.Fatal(e)
	}
	r4.Body.Close()
	if logs["ok"] != true {
		t.Fatalf("log route not ok: %+v", logs)
	}
}

// 12. 路由自初始化：yyQQPath 已设置时，路由注册即从数据目录加载 qq-skills.json。
func TestYYQQSkillRoutesSelfInit(t *testing.T) {
	dir := t.TempDir()
	saved := `{"skills":[{"name":"disk_one","builtin":false,"label":"磁盘技能","desc":"来自文件","run":"console","command":"/say {msg}","enabled":true,"timeout_ms":3000,"params":[{"name":"msg","label":"内容","type":"string","required":true}]}]}`
	if e := os.WriteFile(filepath.Join(dir, "qq-skills.json"), []byte(saved), 0600); e != nil {
		t.Fatal(e)
	}
	oldPath := yyQQPath
	yyQQPath = filepath.Join(dir, "qq-bridge.json")
	t.Cleanup(func() { yyQQPath = oldPath })
	// 清空注册表，迫使路由走自初始化分支
	yyGameSkillStore.Lock()
	oldPath, oldItems := yyGameSkillStore.Path, yyGameSkillStore.Items
	yyGameSkillStore.Path = ""
	yyGameSkillStore.Items = nil
	yyGameSkillStore.Unlock()
	t.Cleanup(func() {
		yyGameSkillStore.Lock()
		yyGameSkillStore.Path, yyGameSkillStore.Items = oldPath, oldItems
		yyGameSkillStore.Unlock()
	})

	mux := http.NewServeMux()
	yyQQSkillRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	r, e := http.Get(srv.URL + "/api/qq/skills")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	var view struct {
		OK     bool          `json:"ok"`
		Skills []yyGameSkill `json:"skills"`
	}
	if e := json.NewDecoder(r.Body).Decode(&view); e != nil {
		t.Fatal(e)
	}
	if !view.OK || len(view.Skills) != 13 {
		t.Fatalf("expected 12 builtins + 1 from disk, got %d", len(view.Skills))
	}
	for _, s := range view.Skills {
		if s.Name == "disk_one" && s.Command != "/say {msg}" {
			t.Fatalf("disk skill command lost: %+v", s)
		}
	}
}

// 13. 工具定义按权限过滤：成员看不到超级管理员技能，超管看全量。
func TestYYSkillToolDefsLevel(t *testing.T) {
	yyGameSkillInit(t.TempDir())
	member := yySkillToolDefsLevel(0)
	names := map[string]bool{}
	for _, d := range member {
		fn, ok := d["function"].(map[string]any)
		if !ok {
			t.Fatalf("tool def shape wrong: %+v", d)
		}
		name, _ := fn["name"].(string)
		if !strings.HasPrefix(name, "skill_") {
			t.Fatalf("bad tool name %q", name)
		}
		names[name] = true
		s, ok := yyGameSkillLookup(strings.TrimPrefix(name, "skill_"))
		if !ok || s.MinRole > 0 {
			t.Fatalf("member must not see %s", name)
		}
	}
	if !names["skill_online_list"] {
		t.Fatal("member should see online_list")
	}
	if names["skill_whitelist_add"] || names["skill_reload"] {
		t.Fatal("member must not see admin skills")
	}
	if got := len(yySkillToolDefsLevel(2)); got != 12 {
		t.Fatalf("super admin should see all 12, got %d", got)
	}
}

// 14. online_list 分页与权限入口：每页12人、页码越界收敛、越权执行被拒。
func TestYYGameSkillOnlineListPagingAndRole(t *testing.T) {
	yyGameSkillInit(t.TempDir())
	origin := yySkillOnlinePlayers
	defer func() { yySkillOnlinePlayers = origin }()
	many := make([]string, 0, 30)
	for i := 1; i <= 30; i++ {
		many = append(many, fmt.Sprintf("player_%02d", i))
	}
	yySkillOnlinePlayers = func() ([]string, error) { return many, nil }

	// 默认第1页：头部含总数与页码，正文12人
	out, e := yyGameSkillRun("online_list", map[string]string{})
	if e != nil {
		t.Fatalf("page1 failed: %v", e)
	}
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[0], "当前在线 30 人（第 1/3 页）") {
		t.Fatalf("page1 head got %q", lines[0])
	}
	if len(lines) != 13 {
		t.Fatalf("page1 should have 12 names, got %d lines", len(lines))
	}
	if lines[1] != "player_01" || lines[12] != "player_12" {
		t.Fatalf("page1 range wrong: %q..%q", lines[1], lines[12])
	}
	// 第2页
	out, e = yyGameSkillRun("online_list", map[string]string{"page": "2"})
	if e != nil {
		t.Fatalf("page2 failed: %v", e)
	}
	lines = strings.Split(out, "\n")
	if lines[1] != "player_13" || lines[12] != "player_24" {
		t.Fatalf("page2 range wrong: %q..%q", lines[1], lines[12])
	}
	// 末页不满12人
	out, e = yyGameSkillRun("online_list", map[string]string{"page": "3"})
	if e != nil {
		t.Fatalf("page3 failed: %v", e)
	}
	lines = strings.Split(out, "\n")
	if len(lines) != 7 || lines[1] != "player_25" || lines[6] != "player_30" {
		t.Fatalf("page3 wrong: %d lines %q..%q", len(lines), lines[1], lines[6])
	}
	// 页码越界收敛到末页/首页
	if out, e = yyGameSkillRun("online_list", map[string]string{"page": "99"}); e != nil || !strings.Contains(out, "第 3/3 页") {
		t.Fatalf("overflow page should clamp to last: %q %v", out, e)
	}
	if out, e = yyGameSkillRun("online_list", map[string]string{"page": "0"}); e != nil || !strings.Contains(out, "第 1/3 页") {
		t.Fatalf("page 0 should clamp to first: %q %v", out, e)
	}
	// 非整数页码被参数校验拒绝
	if _, e = yyGameSkillRun("online_list", map[string]string{"page": "abc"}); e == nil {
		t.Fatal("non-int page should be rejected")
	}
	// 空名单
	yySkillOnlinePlayers = func() ([]string, error) { return nil, nil }
	if out, e = yyGameSkillRun("online_list", map[string]string{}); e != nil || out != "当前没有玩家在线" {
		t.Fatalf("empty list got %q %v", out, e)
	}
	// 权限入口：成员调超管技能被拒，超管放行
	origin2 := yySkillExecutor
	defer func() { yySkillExecutor = origin2 }()
	yySkillExecutor = func(command string, timeout time.Duration) (string, error) { return "ok", nil }
	if _, e := yyGameSkillRunAs("whitelist_add", map[string]string{"player": "steve_01"}, 0); e == nil {
		t.Fatal("member should not run admin skill")
	}
	if _, e := yyGameSkillRunAs("whitelist_add", map[string]string{"player": "steve_01"}, 2); e != nil {
		t.Fatalf("super admin should pass: %v", e)
	}
}
