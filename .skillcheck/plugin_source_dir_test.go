package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func yySourceBundle(t *testing.T, root, name, value string) string {
	t.Helper()
	dir := filepath.Join(root, "插件文件", name)
	if e := os.MkdirAll(filepath.Join(dir, "modules"), 0700); e != nil {
		t.Fatal(e)
	}
	files := map[string]string{
		"main.lua":               `local m=require("modules.gun_router"); function value() return m.value end; function roots() return {code=plugin.codeDir(),data=plugin.dataDir(),config=plugin.configDir()} end`,
		"modules/gun_router.lua": `return {value="` + value + `"}`,
		"manifest.json":          `{"name":"完整目录包"}`,
	}
	for name, body := range files {
		if e := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return dir
}

func TestScannedMainKeepsActualSourceDirAndExistingDataIdentity(t *testing.T) {
	root := t.TempDir()
	dir := yySourceBundle(t, root, "完整包目录", "actual-module")
	pc := NewPluginController()
	pc.SetPluginDirs(root, "")
	old := pluginController
	pluginController = pc
	t.Cleanup(func() { pluginController = old })
	if e := pc.WriteConfig("main", map[string]any{"existing": true}); e != nil {
		t.Fatal(e)
	}
	if e := pc.ScanPlugins(); e != nil {
		t.Fatal(e)
	}
	if pc.defs["main"] == nil || pc.defs["main"].SourceDir != dir {
		t.Fatal("existing main ID lost or source not retained")
	}
	bm := &BotManager{}
	if e := pc.Run(bm, "main"); e != nil {
		t.Fatal("complete directory must require its own module", e)
	}
	t.Cleanup(func() { yyStopResident(bm, "main") })
	if v, e := yyCallResident(bm, "main", "value", nil, false, false); e != nil || v != "actual-module" {
		t.Fatal(v, e)
	}
	v, e := yyCallResident(bm, "main", "roots", nil, false, false)
	if e != nil {
		t.Fatal(e)
	}
	roots := v.(map[string]any)
	if roots["code"] != dir || roots["data"] != filepath.Join(root, "插件数据文件", "main") || roots["config"] != filepath.Join(root, "插件配置文件", "main") {
		t.Fatal(roots)
	}
	if cfg, e := pc.ReadConfig("main"); e != nil || cfg["existing"] != true {
		t.Fatal("config identity changed", cfg, e)
	}
	secondary := &BotManager{}
	if v, e := yyCallResident(secondary, "main", "value", nil, false, false); e != nil || v != "actual-module" {
		t.Fatal("secondary lost module root", v, e)
	}
	t.Cleanup(func() { yyStopResident(secondary, "main") })
	if name, body, e := pc.ReadPluginFile("code", "main", ""); e != nil || name != "main.lua" || !strings.Contains(body, "gun_router") {
		t.Fatal(name, body, e)
	}
	if e := pc.WritePluginFile("code", "main", "modules/gun_router.lua", `return {value="edited"}`); e != nil {
		t.Fatal(e)
	}
	if body, e := os.ReadFile(filepath.Join(dir, "modules", "gun_router.lua")); e != nil || !strings.Contains(string(body), "edited") {
		t.Fatal(string(body), e)
	}
	if _, _, e := pc.ReadPluginFile("code", "main", "../neighbor/main.lua"); e == nil {
		t.Fatal("module editor escaped package root")
	}
}

func TestScannedTwoMainEntriesReportCollisionWithoutOverwritingOwner(t *testing.T) {
	root := t.TempDir()
	first := yySourceBundle(t, root, "alpha", "first")
	yySourceBundle(t, root, "beta", "second")
	pc := NewPluginController()
	pc.SetPluginDirs(root, "")
	e := pc.ScanPlugins()
	if e == nil || !strings.Contains(e.Error(), "多个入口") {
		t.Fatal("same entry collision silently lost", e)
	}
	if len(pc.defs) != 1 || pc.defs["main"].SourceDir != first {
		t.Fatal("collision overwrote previous ownership", pc.defs)
	}
	if dir, e := pc.CodeDir("main"); e != nil || dir != first {
		t.Fatal(dir, e)
	}
	// A later refresh also reports the ambiguity and leaves identity stable.
	if e := pc.ScanPlugins(); e == nil || pc.defs["main"].SourceDir != first {
		t.Fatal("refresh silently changed owner", e)
	}
}

func TestSourceDirBackupAndDeleteUseOwnedTreeOnly(t *testing.T) {
	root := t.TempDir()
	dir := yySourceBundle(t, root, "完整目录", "module")
	neighbor := filepath.Join(root, "插件文件", "neighbor")
	if e := os.MkdirAll(neighbor, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(neighbor, "keep.txt"), []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	pc := NewPluginController()
	pc.SetPluginDirs(root, "")
	if e := pc.ScanPlugins(); e != nil {
		t.Fatal(e)
	}
	archive, e := pc.ExportPackage("main")
	if e != nil {
		t.Fatal(e)
	}
	zr, e := zip.OpenReader(archive)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, f := range zr.File {
		if f.Name == "插件文件/main/modules/gun_router.lua" {
			found = true
		}
		if strings.Contains(f.Name, "neighbor") || strings.Contains(f.Name, "完整目录") {
			t.Fatal("backup leaked physical/neighbor root", f.Name)
		}
	}
	zr.Close()
	if !found {
		t.Fatal("source module missing from complete backup")
	}
	market, e := pc.ExportMarketPackage("main")
	if e != nil {
		t.Fatal(e)
	}
	marketZip, e := zip.OpenReader(market)
	if e != nil {
		t.Fatal(e)
	}
	marketFound := false
	for _, f := range marketZip.File {
		if f.Name == "modules/gun_router.lua" {
			marketFound = true
		}
	}
	marketZip.Close()
	if !marketFound {
		t.Fatal("market export lost original modules")
	}
	if e := pc.Delete(nil, "main"); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(dir); !os.IsNotExist(e) {
		t.Fatal("actual source remained", e)
	}
	if b, e := os.ReadFile(filepath.Join(neighbor, "keep.txt")); e != nil || string(b) != "keep" {
		t.Fatal("neighbor affected", e)
	}
}

func TestLooseSourceExportAndDeleteKeepAllOwnedFilesAndNeighbors(t *testing.T) {
	for _, entry := range []string{"loose.lua", "loose词库.json"} {
		t.Run(entry, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "插件文件")
			if e := os.MkdirAll(base, 0700); e != nil {
				t.Fatal(e)
			}
			code := "return {}"
			if strings.HasSuffix(entry, ".json") {
				code = `{"菜单项":[]}`
			}
			files := map[string]string{entry: code, "loose.manifest.json": `{"id":"loose","name":"保留功能"}`, "loose.docs.md": "manual", "neighbor.lua": "return 42", "neighbor.docs.md": "neighbor manual"}
			for name, body := range files {
				if e := os.WriteFile(filepath.Join(base, name), []byte(body), 0600); e != nil {
					t.Fatal(e)
				}
			}
			pc := NewPluginController()
			pc.SetPluginDirs(root, "")
			if e := pc.ScanPlugins(); e != nil {
				t.Fatal(e)
			}
			if e := pc.WriteConfig("loose", map[string]any{"saved": true}); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(pc.DataDir("loose"), "save.txt"), []byte("owned data"), 0600); e != nil {
				t.Fatal(e)
			}
			archive, e := pc.ExportPackage("loose")
			if e != nil {
				t.Fatal("legitimate loose backup rejected", e)
			}
			zr, e := zip.OpenReader(archive)
			if e != nil {
				t.Fatal(e)
			}
			seen := map[string]bool{}
			for _, file := range zr.File {
				seen[file.Name] = true
				if strings.Contains(file.Name, "neighbor") {
					t.Fatal("neighbor leaked", file.Name)
				}
			}
			zr.Close()
			for _, name := range []string{"插件文件/loose/" + entry, "插件文件/loose/loose.manifest.json", "插件文件/loose/loose.docs.md", "插件数据文件/loose/save.txt", "插件配置文件/loose/loose.json"} {
				if !seen[name] {
					t.Fatal("owned backup file missing", name)
				}
			}
			restored := NewPluginController()
			restored.SetPluginDirs(t.TempDir(), "")
			if id, e := restored.ImportPackage(archive); e != nil || id != "loose" {
				t.Fatal("loose backup cannot restore", id, e)
			}
			if e := restored.ScanPlugins(); e != nil {
				t.Fatal(e)
			}
			if _, body, e := restored.ReadPluginFile("code", "loose", ""); e != nil || body != code {
				t.Fatal("restored source changed", body, e)
			}
			market, e := pc.ExportMarketPackage("loose")
			if e != nil {
				t.Fatal("legitimate loose market export rejected", e)
			}
			marketZip, e := zip.OpenReader(market)
			if e != nil {
				t.Fatal(e)
			}
			marketSeen := map[string]bool{}
			for _, file := range marketZip.File {
				marketSeen[file.Name] = true
				if strings.Contains(file.Name, "neighbor") || strings.Contains(file.Name, "save.txt") {
					t.Fatal("market leaked private/neighbor files", file.Name)
				}
			}
			marketZip.Close()
			marketEntry := entry
			if strings.HasSuffix(entry, ".lua") {
				marketEntry = "main.lua"
			}
			if !marketSeen[marketEntry] || !marketSeen["manifest.json"] || !marketSeen["docs.md"] {
				t.Fatal("market omitted owned source or metadata", marketSeen)
			}
			if e := pc.Delete(nil, "loose"); e != nil {
				t.Fatal("legitimate loose deletion rejected", e)
			}
			for _, name := range []string{entry, "loose.manifest.json", "loose.docs.md"} {
				if _, e := os.Stat(filepath.Join(base, name)); !os.IsNotExist(e) {
					t.Fatal("owned file remained", name, e)
				}
			}
			for _, name := range []string{"neighbor.lua", "neighbor.docs.md"} {
				if body, e := os.ReadFile(filepath.Join(base, name)); e != nil || string(body) != files[name] {
					t.Fatal("neighbor changed", name, e)
				}
			}
			for _, dir := range []string{"插件配置文件", "插件数据文件"} {
				if _, e := os.Stat(filepath.Join(root, dir, "loose")); !os.IsNotExist(e) {
					t.Fatal("owned state not removed", dir, e)
				}
			}
		})
	}
}
