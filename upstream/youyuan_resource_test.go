package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReimportRefreshesInstalledMetadata(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	for _, desc := range []string{"old", "new"} {
		_, e := pc.ImportPackage(writeZip(t, map[string]string{
			"manifest.json": `{"id":"upgrade-test","description":"` + desc + `","runtime_mode":"persistent"}`,
			"main.lua":      "value='" + desc + "'", "docs.md": desc,
		}))
		if e != nil {
			t.Fatal(e)
		}
		pc.ScanPlugins()
		pc.mu.Lock()
		def := pc.defs["upgrade-test"]
		pc.mu.Unlock()
		if def == nil || def.Description != desc || def.Docs != desc {
			t.Fatalf("stale metadata: %+v", def)
		}
	}
}

func TestResourceImportModuleAndFullRoundTrip(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	id, e := pc.ImportPackage(writeZip(t, map[string]string{
		"bundle/manifest.json":              `{"id":"resource-test"}`,
		"bundle/main.lua":                   `local m=require("lib.counter");plugin_api={next=function()return m.next()end}`,
		"bundle/lib/counter.lua":            `local n=0;return {next=function()n=n+1;return n end}`,
		"bundle/resources/audio/binary.dat": "\x00\x01\xff",
		"bundle/resources/readme.txt":       "中文资源",
		"bundle/config.json":                `{"enabled":true}`,
	}))
	if e != nil {
		t.Fatal(e)
	}
	r := yyResourceRoots{Code: filepath.Join(pc.privateDir, "插件文件", id), Data: pc.DataDir(id), Config: filepath.Dir(pc.ConfigPath(id))}
	if v, e := yyResourceCall(r, "plugin.readResource", []any{"readme.txt"}); e != nil || v != "中文资源" {
		t.Fatalf("lost resource %v %v", v, e)
	}
	if v, e := yyResourceCall(r, "plugin.readResource", []any{"audio/binary.dat", "base64"}); e != nil || v != "AAH/" {
		t.Fatalf("binary corrupt %v %v", v, e)
	}
	if _, e := yyResourceCall(r, "plugin.extractResource", []any{"audio/binary.dat", "copy.dat"}); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(filepath.Join(r.Data, "copy.dat"))
	if e != nil || string(data) != "\x00\x01\xff" {
		t.Fatal("binary copy corrupt")
	}
	old := pluginController
	pluginController = pc
	defer func() { pluginController = old }()
	code, e := os.ReadFile(filepath.Join(r.Code, id+".lua"))
	if e != nil {
		t.Fatal(e)
	}
	bm := &BotManager{}
	if e = yyLoadResident(bm, id, string(code), false); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(bm, id)
	for i := 1; i <= 3; i++ {
		v, e := yyCallResident(bm, id, "next", nil, true, false)
		if e != nil || v != float64(i) {
			t.Fatalf("module reload %v %v", v, e)
		}
	}
	for _, path := range []string{"../secret", "/etc/passwd", "C:/secret", `..\secret`} {
		if _, e := yyResourceCall(r, "plugin.readResource", []any{path}); e == nil {
			t.Fatalf("escape accepted %s", path)
		}
	}
	market, e := pc.ExportMarketPackage(id)
	if e != nil {
		t.Fatal(e)
	}
	copy := NewPluginController()
	copy.SetPluginDirs(t.TempDir(), "")
	if _, e = copy.ImportPackage(market); e != nil {
		t.Fatal(e)
	}
	if data, e = os.ReadFile(filepath.Join(copy.privateDir, "插件文件", id, "lib", "counter.lua")); e != nil || len(data) == 0 {
		t.Fatal("market module lost")
	}
}

func TestResourceSymlinkAndDrivePathsRejected(t *testing.T) {
	for _, path := range []string{"../x", `a\..\x`, "C:/x", "/x"} {
		if zipSlipSafe(path) {
			t.Fatalf("bad zip path accepted %q", path)
		}
	}
	if !zipSlipSafe("notes..txt") {
		t.Fatal("legitimate filename rejected")
	}
}
