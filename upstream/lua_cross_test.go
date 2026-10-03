package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 跨插件调用：A 插件导出 plugin_api.add，B 插件经 api.require("A").add(1,2) 调用。
func TestCrossPluginCall(t *testing.T) {
	bm := &BotManager{}
	srcA := `
		plugin_api = { add = function(a, b) return a + b end }
	`
	if err := LoadLuaPlugin(bm, "A", srcA); err != nil {
		t.Fatalf("加载 A 失败: %v", err)
	}
	var got []string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = append(got, txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	L := newLuaState(bm)
	defer L.Close()
	if err := L.DoString(`local r = api.require("A").add(1, 2); game.sendTargeted("a", tostring(r))`); err != nil {
		t.Fatalf("跨插件调用失败: %v", err)
	}
	if len(got) != 1 || got[0] != "3" {
		t.Fatalf("跨插件结果错误: %v", got)
	}
}

// 跨插件调用：目标插件未加载时 api.require 返回 nil。
func TestCrossPluginMissing(t *testing.T) {
	bm := &BotManager{}
	var got []string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = append(got, txt); return nil })
	defer func() { setSendTargetedHook(old) }()
	L := newLuaState(bm)
	defer L.Close()
	if err := L.DoString(`local r = api.require("NOPE"); game.sendTargeted("a", tostring(r == nil))`); err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if len(got) != 1 || got[0] != "true" {
		t.Fatalf("缺失插件应返回 nil: %v", got)
	}
}

// util 工具库：时间/字符串/坐标/JSON 文件读写往返。
func TestUtilAPIs(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()

	for _, fn := range []string{"now", "timestamp", "time_fmt", "json_load", "json_write",
		"read_file", "write_file", "strip_color", "split", "join", "trim", "floor", "ceil", "round", "pos"} {
		if L.GetField(L.GetGlobal("util"), fn).Type() == 0 {
			t.Fatalf("util.%s 未绑定", fn)
		}
	}

	var got []string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = append(got, txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	dir := t.TempDir()
	cfg := filepath.ToSlash(filepath.Join(dir, "sub", "cfg.json"))
	if err := L.DoString(`util.json_write("` + cfg + `", {a=1, name="x"})`); err != nil {
		t.Fatalf("json_write: %v", err)
	}
	if _, err := os.Stat(cfg); err != nil {
		t.Fatalf("json_write 未生成文件: %v", err)
	}
	// 读回并验证
	if err := L.DoString(`local d = util.json_load("` + cfg + `"); game.sendTargeted("a", d.a .. "/" .. d.name)`); err != nil {
		t.Fatalf("json_load: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] != "1/x" {
		t.Fatalf("json 往返错误: %v", got)
	}
	// split/join/pos/strip_color
	got = nil
	if err := L.DoString(`local p = util.split("1,2,3", ","); local q = util.join(p, "-"); local pos = util.pos("10 20 30"); game.sendTargeted("a", q .. "|" .. pos.x .. "," .. pos.y .. "," .. pos.z .. "|" .. util.strip_color("&a绿"))`); err != nil {
		t.Fatalf("util 字符串/坐标: %v", err)
	}
	if len(got) == 0 || !strings.Contains(got[len(got)-1], "1-2-3|10,20,30|绿") {
		t.Fatalf("util 输出错误: %v", got)
	}
}

// 能力表绑定：game/bot/building/media/world/api/plugin/util 均存在。
func TestCapabilityTablesBound(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	check := func(table string, fns ...string) {
		tbl := L.GetGlobal(table)
		for _, fn := range fns {
			if L.GetField(tbl, fn).Type() == 0 {
				t.Fatalf("%s.%s 未绑定", table, fn)
			}
		}
	}
	check("game", "getTickingAreaList", "queryInventory", "isCmdSuccess", "sendTitle", "sendActionbar",
		"sendRichTellraw", "setEffect", "teleport", "move", "flyStart", "flyStop", "getBotPos")
	check("bot", "count", "list", "status")
	check("building", "detect", "parse", "isAir", "closestColor")
	check("media", "imageToAscii", "noteToPitch", "parseMidi")
	check("world", "block")
	check("api", "require")
	check("plugin", "id", "dependencies")
	check("system", "shell", "platform")
}

// 前置依赖：Run B 会先递归 Run 其前置 A；缺失前置则报错并置 error。
func TestDependencyResolution(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	bm := &BotManager{}

	// A 无前置（版本 1.0.0）
	if err := pc.AddDef(&PluginDef{ID: "A", Name: "A", Version: "1.0.0", Load: func(*BotManager) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	// B 依赖 A（最低版本 1.0.0，满足）
	if err := pc.AddDef(&PluginDef{ID: "B", Name: "B", PrePlugins: map[string]string{"A": "1.0.0"},
		Load: func(*BotManager) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if err := pc.Run(bm, "B"); err != nil {
		t.Fatalf("Run B 应成功（自动加载前置 A）: %v", err)
	}
	if !pc.running["A"] || !pc.running["B"] {
		t.Fatalf("前置 A 或 B 未运行: A=%v B=%v", pc.running["A"], pc.running["B"])
	}
	// 前置版本过低 → 报错
	if err := pc.AddDef(&PluginDef{ID: "C", Name: "C", PrePlugins: map[string]string{"A": "9.0.0"},
		Load: func(*BotManager) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if err := pc.Run(bm, "C"); err == nil || !strings.Contains(err.Error(), "版本过低") {
		t.Fatalf("版本不足应报错, got: %v", err)
	}
	// 缺失前置 → 报错并置 error
	if err := pc.AddDef(&PluginDef{ID: "D", Name: "D", PrePlugins: map[string]string{"MISSING": "1.0.0"},
		Load: func(*BotManager) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if err := pc.Run(bm, "D"); err == nil || !strings.Contains(err.Error(), "缺少前置插件") {
		t.Fatalf("缺失前置应报错, got: %v", err)
	}
	if st := pc.status("D"); st == nil || st.State != "error" {
		t.Fatalf("D 应置 error: %+v", st)
	}
}

// 依赖环：A↔B 互相依赖必须报错，而不是无限递归栈溢出崩溃。
func TestDependencyCycle(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	bm := &BotManager{}
	if err := pc.AddDef(&PluginDef{ID: "cycA", Name: "A", Version: "1.0.0", PrePlugins: map[string]string{"cycB": "1.0.0"},
		Load: func(*BotManager) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if err := pc.AddDef(&PluginDef{ID: "cycB", Name: "B", Version: "1.0.0", PrePlugins: map[string]string{"cycA": "1.0.0"},
		Load: func(*BotManager) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if err := pc.Run(bm, "cycA"); err == nil || !strings.Contains(err.Error(), "依赖成环") {
		t.Fatalf("依赖环应报错, got: %v", err)
	}
	// 自依赖同样要报错
	pc2 := NewPluginController()
	pc2.SetPluginDirs(t.TempDir(), "")
	if err := pc2.AddDef(&PluginDef{ID: "self", Name: "S", Version: "1.0.0", PrePlugins: map[string]string{"self": "1.0.0"},
		Load: func(*BotManager) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if err := pc2.Run(bm, "self"); err == nil || !strings.Contains(err.Error(), "依赖成环") {
		t.Fatalf("自依赖应报错, got: %v", err)
	}
}

// 版本比较 helper 单测。
func TestVersionAtLeast(t *testing.T) {
	cases := [][3]any{
		{"2.0.0", "1.0.0", true},
		{"1.5.0", "1.5.0", true},
		{"1.4.9", "1.5.0", false},
		{"v2.0.0", "2.0.0", true},
	}
	for _, c := range cases {
		cur, min := c[0].(string), c[1].(string)
		if got := versionAtLeast(cur, min); got != c[2].(bool) {
			t.Fatalf("versionAtLeast(%s,%s)=%v 期望 %v", cur, min, got, c[2])
		}
	}
	// 依赖运行要触发注册的 source（跨插件需已加载）
	_ = time.Now()
}
