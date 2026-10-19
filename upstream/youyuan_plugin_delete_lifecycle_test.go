package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeletePluginClosesResidentBeforeRemovingFiles(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	id := "delete-resident67"
	bm := NewBotManager()
	if err := LoadLuaPlugin(bm, id, `events.onChat(function()end);util.setTimeout(3600,function()end);plugin_api={alive=function()return true end}`); err != nil {
		t.Fatal(err)
	}
	yyLuaMu.Lock()
	resident := yyLuaStates[yyLuaKey{bm, id}]
	yyLuaMu.Unlock()
	t.Cleanup(func() { yyStopResident(bm, id) })
	pc.running[id] = true
	pc.defs[id] = &PluginDef{ID: id}
	root := filepath.Join(pc.privateDir, "插件数据文件", id)
	os.MkdirAll(root, 0700)
	os.WriteFile(filepath.Join(root, "state.json"), []byte("{}"), 0600)
	if err := pc.Delete(nil, id); err != nil {
		t.Fatal(err)
	}
	resident.mu.Lock()
	closed := resident.closed
	resident.mu.Unlock()
	if !closed {
		t.Fatal("deleted plugin VM is still alive")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("plugin data remains", err)
	}
	yyLuaMu.Lock()
	exists := yyLuaStates[yyLuaKey{bm, id}] != nil
	yyLuaMu.Unlock()
	if exists {
		t.Fatal("resident registry survived deletion")
	}
	schedMu.Lock()
	defer schedMu.Unlock()
	for _, job := range schedJobs {
		if job.plugin == id {
			select {
			case <-job.cancel:
			default:
				t.Fatal("timer survived deletion")
			}
		}
	}
}
func TestDeleteReportsFilesystemFailureAndDoesNotPretendSuccess(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	pc.removeTree = func(path string) error {
		if strings.Contains(path, "插件数据文件") {
			return errors.New("fixture: disk permission denied")
		}
		return os.RemoveAll(path)
	}
	pc.defs["failure67"] = &PluginDef{ID: "failure67"}
	err := pc.Delete(nil, "failure67")
	if err == nil || !strings.Contains(err.Error(), "未能删除") {
		t.Fatal(err)
	}
	if pc.defs["failure67"] == nil || pc.state["failure67"] != "error" {
		t.Fatal("failure was hidden from the user")
	}
}
func TestDeleteDoesNotRaceAnActiveLoad(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	pc.loading["loading67"] = true
	if err := pc.Delete(nil, "loading67"); err == nil {
		t.Fatal("deleted plugin during loading")
	}
}
