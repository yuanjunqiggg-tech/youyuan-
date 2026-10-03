package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundLuaSourceReadsModulesAndProtectsConcurrentEdits(t *testing.T) {
	oldPC, oldRoot := pluginController, yyAgentRoot
	defer func() { pluginController = oldPC; yyAgentRoot = oldRoot }()
	root := t.TempDir()
	pluginController = NewPluginController()
	pluginController.SetPluginDirs(root, "")
	yyAgentRoot = filepath.Join(root, "workspaces")
	dir := filepath.Join(root, "插件文件", "real")
	os.MkdirAll(filepath.Join(dir, "modules"), 0700)
	original := strings.Repeat("-- actual source line\n", 4000) + "return {}\n"
	os.WriteFile(filepath.Join(dir, "real.lua"), []byte(original), 0600)
	os.WriteFile(filepath.Join(dir, "modules/helper.lua"), []byte("return 42\n"), 0600)
	ws := &yyWorkspace{ID: "edit", PluginID: "real", Language: "lua"}
	os.MkdirAll(yyWorkspaceRoot(ws), 0700)
	os.WriteFile(filepath.Join(yyWorkspaceRoot(ws), "main.lua"), []byte("real.lua"), 0600)
	if e := yyBindLuaPlugin(ws); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(yyWorkspaceRoot(ws), "main.lua")); !os.IsNotExist(e) {
		t.Fatal("old filename-only placeholder retained")
	}
	if code, e := yyReadProject(ws, "real.lua"); e != nil || code != original {
		t.Fatal("source not copied completely", e)
	}
	if _, e := yyReadProject(ws, "modules/helper.lua"); e != nil {
		t.Fatal(e)
	}
	edited := original + "-- workspace change\n"
	if diff := yyDiff(yyRevision{Path: "real.lua", Before: original, After: edited}); len(diff) > 12000 || !strings.Contains(diff, "+-- workspace change") {
		t.Fatal("large-file diff lost the edit or exhausted context")
	}
	if _, e := yyWriteProject(ws, "real.lua", edited); e != nil {
		t.Fatal(e)
	}
	if e := yyBindLuaPlugin(ws); e != nil {
		t.Fatal("local-only edits must survive sync", e)
	}
	if code, _ := yyReadProject(ws, "real.lua"); code != edited {
		t.Fatal("local edits lost")
	}
	external := original + "-- another editor\n"
	os.WriteFile(filepath.Join(dir, "real.lua"), []byte(external), 0600)
	if e := yyPublishBoundLua(ws); e == nil {
		t.Fatal("concurrent installed edit overwritten")
	}
	if e := yyBindLuaPlugin(ws); e == nil {
		t.Fatal("sync conflict ignored")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "real.lua")); string(data) != external {
		t.Fatal("conflict changed installed source")
	}
	os.WriteFile(filepath.Join(dir, "real.lua"), []byte(original), 0600)
	if e := yyPublishBoundLua(ws); e != nil {
		t.Fatal(e)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "real.lua")); string(data) != edited {
		t.Fatal("publish did not write installed source")
	}
	if _, _, e := yyLuaPluginSource("../real"); e == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestValidationWorkspacesArchivedWithoutUserProjectLoss(t *testing.T) {
	oldRoot, oldItems := yyAgentRoot, yyWorkspaces.items
	defer func() { yyAgentRoot = oldRoot; yyWorkspaces.items = oldItems }()
	yyAgentRoot = filepath.Join(t.TempDir(), "workspaces")
	yyWorkspaces.items = map[string]*yyWorkspace{}
	for _, ws := range []*yyWorkspace{{ID: "unicode_probe", Name: "Unicode 验证"}, {ID: "cpp_workspace_demo", Name: "用户修改后的项目"}, {ID: "user_plugin", Name: "我的插件"}} {
		os.MkdirAll(yyWorkspaceRoot(ws), 0700)
		os.WriteFile(filepath.Join(yyWorkspaceRoot(ws), "main.txt"), []byte("user edits retained"), 0600)
		ws.saveLocked()
		yyWorkspaces.items[ws.ID] = ws
	}
	yyMigrateValidationWorkspaces()
	if yyWorkspaces.items["unicode_probe"] != nil || yyWorkspaces.items["user_plugin"] == nil || yyWorkspaces.items["cpp_workspace_demo"] == nil {
		t.Fatal("migration selected wrong workspace")
	}
	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(yyAgentRoot), "工作区备份", "unicode_probe-*", "project", "main.txt"))
	if len(backups) != 1 {
		t.Fatal("backup missing")
	}
	data, _ := os.ReadFile(backups[0])
	if string(data) != "user edits retained" {
		t.Fatal("archive lost edits")
	}
}

func TestFreshAgentDoesNotCreateExampleWorkspaces(t *testing.T) {
	oldRoot, oldItems := yyAgentRoot, yyWorkspaces.items
	defer func() { yyAgentRoot = oldRoot; yyWorkspaces.items = oldItems }()
	yyWorkspaces.items = map[string]*yyWorkspace{}
	yyAgentRoutes(http.NewServeMux(), t.TempDir())
	if len(yyWorkspaces.items) != 0 {
		t.Fatal("fresh install created unsolicited projects")
	}
	entries, _ := os.ReadDir(yyAgentRoot)
	if len(entries) != 0 {
		t.Fatal("fresh install populated example directories")
	}
}
