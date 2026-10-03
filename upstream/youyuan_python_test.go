//go:build !android

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestYouYuanPythonGameEventDispatch(t *testing.T) {
	if os.Getenv("YY_PYTHON") == "" {
		t.Skip("设置 YY_PYTHON 以执行真实解释器检查")
	}
	root := t.TempDir()
	entry := filepath.Join(root, "main.py")
	_ = os.WriteFile(entry, []byte("counter=0\ndef on_event(event,host):\n global counter\n counter+=1\n return {'ok':True,'counter':counter,'last_type':event['type']}\n"), 0644)
	key := "main:dispatch-test"
	start, err := yyPythonRuntime("start", map[string]any{"key": key, "account": "main", "entry": entry, "data_dir": filepath.Join(root, "data")})
	if err != nil {
		t.Fatal(start, err)
	}
	defer yyPythonRuntime("stop", map[string]any{"key": key})
	saved := botMgr
	bm := &BotManager{}
	botMgr = bm
	defer func() { botMgr = saved }()
	instance := &yyRuntimeInstance{Key: key, Account: "main", Manifest: yyRuntimeManifest{Language: "python"}, Queue: make(chan map[string]any, 4), Done: make(chan struct{})}
	yyRuntimeMu.Lock()
	yyRuntimeInstances[key] = instance
	yyRuntimeMu.Unlock()
	defer func() {
		close(instance.Done)
		yyRuntimeMu.Lock()
		delete(yyRuntimeInstances, key)
		yyRuntimeMu.Unlock()
	}()
	go instance.runEvents()
	yyRuntimeEmit(bm, map[string]any{"type": "chat", "message": "hello"})
	deadline := time.Now().Add(5 * time.Second)
	for {
		instance.mu.Lock()
		count := instance.Counter
		instance.mu.Unlock()
		if count > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("游戏事件没有进入插件运行时")
		}
		time.Sleep(5 * time.Millisecond)
	}
	raw, err := instance.event(map[string]any{"type": "manual"})
	var result map[string]any
	_ = json.Unmarshal([]byte(raw), &result)
	if err != nil || result["counter"] != float64(2) {
		t.Fatal("真实 Python 事件状态被重置", raw, err)
	}
}
