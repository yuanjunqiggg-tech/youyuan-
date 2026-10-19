package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Opt-in original market ZIP, actual import/controller/resident/event path.
// The targeted transport is captured; this is not remote game confirmation.
func TestMarketFree201OriginalPackageFunctions(t *testing.T) {
	zipPath := os.Getenv("YY_MARKET_TEST_ZIP")
	if zipPath == "" {
		t.Skip("select original free market package")
	}
	pc := NewPluginController()
	root := t.TempDir()
	pc.SetPluginDirs(root, "")
	id, e := pc.ImportPackage(zipPath)
	if e != nil {
		t.Fatal(e)
	}
	if e = pc.ScanPlugins(); e != nil {
		t.Fatal(e)
	}
	bm := &BotManager{logCh: make(chan string, 1024)}
	replies := make(chan string, 1024)
	old := sendTargetedHook
	setSendTargetedHook(func(player, text string) error {
		if player != "market-test-player" {
			t.Errorf("wrong recipient %q", player)
		}
		replies <- text
		return nil
	})
	defer setSendTargetedHook(old)
	if e = pc.Run(bm, id); e != nil {
		t.Fatal(e)
	}
	defer pc.Stop(bm, id)
	source, e := os.ReadFile(filepath.Join(root, "插件文件", id, "main.lua"))
	if e != nil {
		source, e = os.ReadFile(filepath.Join(root, id, "main.lua"))
		if e != nil {
			t.Fatal(e)
		}
	}
	matches := regexp.MustCompile(`\["([^"]+)"\]\s*=\s*"([^"]+)"`).FindAllStringSubmatch(string(source), -1)
	lookup := map[string]map[string]bool{}
	for _, m := range matches {
		if lookup[m[2]] == nil {
			lookup[m[2]] = map[string]bool{}
		}
		lookup[m[2]][m[1]] = true
	}
	if len(lookup) < 100 {
		t.Fatalf("missing original item map %d", len(lookup))
	}
	check := func(msg string, valid func(string) bool) {
		bm.handleChat("market-test-player", msg)
		select {
		case text := <-replies:
			if !valid(text) {
				t.Fatalf("%q -> %q", msg, text)
			}
		case <-time.After(time.Second):
			t.Fatalf("chat did not run %q", msg)
		}
	}
	for name, ids := range lookup {
		check("id "+name, func(text string) bool {
			for en := range ids {
				if strings.HasSuffix(text, en) {
					return true
				}
			}
			return false
		})
	}
	check(" ID   钻石剑 ", func(s string) bool { return strings.HasSuffix(s, "diamond_sword") })
	check("id", func(s string) bool { return strings.Contains(s, "用法") })
	check("id 游园不存在的物品", func(s string) bool { return strings.Contains(s, "未找到") })
	bm.handleChat("market-test-player", "普通聊天，不应触发")
	select {
	case text := <-replies:
		t.Fatalf("unrelated chat replied %q", text)
	case <-time.After(30 * time.Millisecond):
	}
	if e = pc.Stop(bm, id); e != nil {
		t.Fatal(e)
	}
	bm.handleChat("market-test-player", "id 钻石剑")
	select {
	case text := <-replies:
		t.Fatalf("stopped plugin replied %q", text)
	case <-time.After(30 * time.Millisecond):
	}
	if e = pc.Run(bm, id); e != nil {
		t.Fatal(e)
	}
	check("id 钻石剑", func(s string) bool { return strings.HasSuffix(s, "diamond_sword") })
	t.Logf("original ZIP imported; %d distinct item lookups plus trim/case/usage/not-found/ignore/stop/restart verified", len(lookup))
}
