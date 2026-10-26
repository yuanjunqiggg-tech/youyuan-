package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Exact IDs and names of our old validation fixtures. User projects with other
// names/IDs are retained; recognized fixtures are moved with their full history.
var yyOldValidationWorkspaces = map[string]string{
	"agent_live_verification": "AI · 真实文件编辑验证", "unicode_probe": "Unicode 验证",
	"command_workspace_demo": "命令链 · 条件与独立链", "cpp_workspace_demo": "C++ · 常驻原生计数",
	"lua_workspace_demo": "Lua · 常驻聊天计数", "python_workspace_demo": "Python · 常驻事件计数",
	"verify_manage_v095_008afa": "AI 插件管理验证", "verify_manage_v095_f56884": "AI 插件管理验证",
	"verify_v095_414014": "0.9.5 AI实测", "verify_v095_df0679": "0.9.5 AI实测",
	"ui_v095_1790790885560": "UI参数验证", "ui_v095_1790790886328": "UI参数验证",
	"ui_v095_1790793680640": "UI参数验证", "ui_v095_1790793681319": "UI参数验证",
	"ui_v095_1790794269020": "UI参数验证", "ui_v095_1790794269706": "UI参数验证",
	"ui_v095_1790796508756": "UI参数验证", "ui_v095_1790796509421": "UI参数验证",
}

func yyArchiveWorkspace(id string) error {
	if !yyWorkspaceID(id) {
		return fmt.Errorf("工作区标识无效")
	}
	yyFileMu.Lock()
	defer yyFileMu.Unlock()
	yyWorkspaces.Lock()
	defer yyWorkspaces.Unlock()
	ws := yyWorkspaces.items[id]
	if ws == nil {
		return fmt.Errorf("工作区不存在")
	}
	ws.Lock()
	defer ws.Unlock()
	if ws.cancel != nil {
		return fmt.Errorf("请先暂停正在运行的 AI 任务")
	}
	source, e := yyProjectPath(yyAgentRoot, id)
	if e != nil {
		return e
	}
	backup := filepath.Join(filepath.Dir(yyAgentRoot), "工作区备份")
	if e = os.MkdirAll(backup, 0700); e != nil {
		return e
	}
	target, e := yyProjectPath(backup, id+"-"+fmt.Sprint(time.Now().UnixNano()))
	if e != nil {
		return e
	}
	if e = os.Rename(source, target); e != nil {
		return e
	}
	delete(yyWorkspaces.items, id)
	return nil
}
func yyMigrateValidationWorkspaces() {
	for id, name := range yyOldValidationWorkspaces {
		ws, e := yyGetWorkspace(id)
		if e != nil {
			continue
		}
		ws.Lock()
		match := ws.Name == name
		ws.Unlock()
		if match {
			if e = yyArchiveWorkspace(id); e != nil {
				debugLog("[工作区备份] %s: %v", id, e)
			}
		}
	}
}
func yyAgentMaintenanceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/agent/archive", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var p struct {
			ID string `json:"id"`
		}
		e := json.NewDecoder(r.Body).Decode(&p)
		if e == nil {
			e = yyArchiveWorkspace(p.ID)
		}
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "note": "工作区与历史已归档；没有删除已安装插件"})
	})
	mux.HandleFunc("/api/youyuan/agent/sync-plugin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var p struct {
			ID string `json:"id"`
		}
		e := json.NewDecoder(r.Body).Decode(&p)
		var ws *yyWorkspace
		if e == nil {
			ws, e = yyGetWorkspace(p.ID)
		}
		if e == nil {
			if ws.Language != "lua" || ws.PluginID == "" {
				e = fmt.Errorf("工作区未绑定 Lua 插件")
			} else {
				e = yyBindLuaPlugin(ws)
			}
		}
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "workspace": yyWorkspaceView(ws)})
	})
}
