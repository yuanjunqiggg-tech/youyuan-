package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentDefaultsOpenToolsButPreservePlayerRestrictions(t *testing.T) {
	ws := &yyWorkspace{Language: "mcfunction"}
	for _, name := range []string{"game_call", "game_scene", "packet_schema", "listen_packet", "api_call", "plugin_manage"} {
		if !yyAgentAllowed(ws, name) {
			t.Fatal("default agent lacks", name)
		}
	}
	if !yyAgentAllowed(ws, "project_validate") || !yyAgentAllowed(ws, "mcp_call") {
		t.Fatal("validator/MCP unavailable")
	}
	ws.AllowedTools = []string{"project_validate"}
	if yyAgentAllowed(ws, "api_call") || yyAgentAllowed(ws, "game_call") {
		t.Fatal("explicit disabled tool reopened")
	}
}

func TestBundledSampleMigrationPreservesFilesAndUserPlugins(t *testing.T) {
	old := yyRuntimeRoot
	data := t.TempDir()
	yyRuntimeRoot = filepath.Join(data, "多语言插件")
	defer func() { yyRuntimeRoot = old }()
	for _, v := range []struct{ id, name string }{{"yy_sense_python_demo", "Python 血量与位置监控"}, {"user", "自己的插件"}} {
		dir := filepath.Join(yyRuntimeRoot, v.id)
		_ = os.MkdirAll(dir, 0700)
		raw, _ := json.Marshal(yyRuntimeManifest{ID: v.id, Name: v.name, Language: "python", Entry: "main.py"})
		_ = os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0600)
		_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte("edited by user"), 0600)
	}
	yyArchiveBundledSamples(data)
	if _, e := os.Stat(filepath.Join(yyRuntimeRoot, "yy_sense_python_demo")); !os.IsNotExist(e) {
		t.Fatal("sample remains installed")
	}
	backups, _ := filepath.Glob(filepath.Join(data, "示例备份", "*", "main.py"))
	if len(backups) != 1 {
		t.Fatal(backups)
	}
	raw, _ := os.ReadFile(backups[0])
	if string(raw) != "edited by user" {
		t.Fatal("data lost")
	}
	if _, e := os.Stat(filepath.Join(yyRuntimeRoot, "user", "main.py")); e != nil {
		t.Fatal(e)
	}
}

func TestControlCommandsOnlyPaceWhenConfigured(t *testing.T) {
	bm := &BotManager{connected: true, stopCh: make(chan struct{})}
	for n := 0; n < 5; n++ {
		if e := bm.waitControlCommandSlot(); e != nil {
			t.Fatal(e)
		}
	}
	if !bm.controlCommandNext.IsZero() || bm.controlCommands.Load() != 5 {
		t.Fatal("default dispatch must not reserve a future delay")
	}
	start := time.Now()
	for n := 0; n < 5; n++ {
		if e := bm.waitControlCommandRateContext(context.Background(), 1, 1, 200); e != nil {
			t.Fatal(e)
		}
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatal("explicit rate was ignored")
	}
	close(bm.stopCh)
	if e := bm.waitControlCommandSlot(); e == nil {
		t.Fatal("stopped wait accepted")
	}
}
