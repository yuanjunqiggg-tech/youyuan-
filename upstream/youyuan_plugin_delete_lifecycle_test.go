package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// Deleting a plugin with a large data tree must not hold the HTTP request open.
// The live paths disappear with a rename, and the expensive walk happens after
// the caller already has its answer.
func TestDeleteMovesLargeTreeAsideBeforeWalkingIt(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	id := "bulky67"
	pc.defs[id] = &PluginDef{ID: id}
	pc.state[id] = "unloaded"

	dataDir := filepath.Join(pc.privateDir, "插件数据文件", id)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Enough files that a synchronous RemoveAll would be visible on the clock.
	for i := 0; i < 400; i++ {
		if err := os.WriteFile(filepath.Join(dataDir, fmt.Sprintf("f%03d.bin", i)), make([]byte, 1024), 0600); err != nil {
			t.Fatal(err)
		}
	}

	start := time.Now()
	if err := pc.Delete(nil, id); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("delete blocked the caller for %v", elapsed)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatal("the live data path must be gone as soon as delete returns", err)
	}
	// The registry entry is gone immediately, so a rescan cannot resurrect it.
	if pc.defs[id] != nil {
		t.Fatal("deleted plugin is still registered")
	}
	trash := filepath.Join(pc.privateDir, ".trash")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(trash)
		if err == nil && len(entries) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	entries, _ := os.ReadDir(trash)
	t.Fatalf("background removal did not finish, %d tombstones left", len(entries))
}

// A rename that cannot happen (read-only parent, cross-device) must still leave
// the plugin deleted rather than silently resurrecting it after a restart.
func TestRemoveTreeDeferredFallsBackWhenRenameIsImpossible(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "gone")
	if err := os.MkdirAll(filepath.Join(target, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "nested", "a.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	// A trash root inside a regular file cannot be created, so the helper has to
	// fall back to removing in place.
	blocker := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := yyRemoveTreeDeferred(filepath.Join(blocker, "trash"), target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("fallback removal left the tree behind", err)
	}
	if err := yyRemoveTreeDeferred("", filepath.Join(root, "never-existed")); err != nil {
		t.Fatalf("a missing target is not an error, got %v", err)
	}
}
