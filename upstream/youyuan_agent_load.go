package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Bound simultaneous inference/tool loops; projects remain independent and resumable.
var yyAgentSlots = make(chan struct{}, 2)
var yyAgentSlotMu sync.Mutex

func yyManageWorkspacePlugin(ws *yyWorkspace, action string) (string, bool) {
	if action != "start" && action != "stop" && action != "reload" && action != "delete" {
		return "action 应为 start/stop/reload/delete", true
	}
	ws.Lock()
	id, language := ws.PluginID, ws.Language
	ws.Unlock()
	if language == "mcfunction" {
		return "命令链工作区不管理插件", true
	}
	if id == "" {
		if language == "python" || language == "native_cpp" {
			id = ws.ID
			if _, e := os.Stat(filepath.Join(yyRuntimeRoot, id, "manifest.json")); e != nil {
				return "先使用 project_publish 发布当前工作区", true
			}
		} else {
			return "先使用 project_publish 发布当前工作区", true
		}
	}
	invoke := func(path string) (string, bool) {
		_, data, e := yyInvoke("POST", path, nil, map[string]any{"id": id, "account": "main"})
		if e != nil {
			return e.Error(), true
		}
		var result map[string]any
		_ = json.Unmarshal(data, &result)
		return string(data), result["ok"] != true
	}
	if language == "lua" {
		if action == "start" {
			action = "run"
		}
		return invoke("/api/plugin/" + action)
	}
	if action == "reload" {
		if result, failed := invoke("/api/youyuan/runtime/stop"); failed {
			return result, true
		}
		action = "start"
	}
	return invoke("/api/youyuan/runtime/" + action)
}

func yyTryAgentSlot() error {
	yyAgentSlotMu.Lock()
	defer yyAgentSlotMu.Unlock()
	select {
	case yyAgentSlots <- struct{}{}:
		return nil
	default:
		return fmt.Errorf("已有 2 个 AI 任务运行；请暂停其中一个再继续，当前任务已保存")
	}
}
