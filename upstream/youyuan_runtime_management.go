package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func yyRuntimeDirectory(id string) (string, error) {
	if !yyWorkspaceID(id) {
		return "", fmt.Errorf("插件标识无效")
	}
	root, e := filepath.Abs(yyRuntimeRoot)
	if e != nil {
		return "", e
	}
	target, e := yyProjectPath(root, id)
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(root, target)
	if e != nil || rel != id || target == root {
		return "", fmt.Errorf("插件目录越界")
	}
	return target, nil
}

func yyRuntimeManagementRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/runtime/files", func(w http.ResponseWriter, r *http.Request) {
		dir, e := yyRuntimeDirectory(r.URL.Query().Get("id"))
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		entries := []map[string]any{}
		e = filepath.WalkDir(dir, func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if path == dir {
				return nil
			}
			if len(entries) >= 5000 {
				return fmt.Errorf("目录文件超过 5000 个，请导出查看")
			}
			rel, _ := filepath.Rel(dir, path)
			info, e := d.Info()
			if e != nil {
				return e
			}
			entries = append(entries, map[string]any{"path": filepath.ToSlash(rel), "directory": d.IsDir(), "size": info.Size(), "symlink": d.Type()&os.ModeSymlink != 0})
			return nil
		})
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "directory": dir, "files": entries})
	})
	mux.HandleFunc("/api/youyuan/runtime/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var p struct {
			ID string `json:"id"`
		}
		if e := json.NewDecoder(r.Body).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		dir, e := yyRuntimeDirectory(p.ID)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		yyRuntimeStartMu.Lock()
		defer yyRuntimeStartMu.Unlock()
		yyRuntimeMu.Lock()
		instances := []*yyRuntimeInstance{}
		for key, in := range yyRuntimeInstances {
			if in.Manifest.ID == p.ID {
				instances = append(instances, in)
				delete(yyRuntimeInstances, key)
			}
		}
		yyRuntimeMu.Unlock()
		for _, in := range instances {
			in.mu.Lock()
			in.Stopped = true
			close(in.Done)
			if in.Manifest.Language == "python" {
				_, _ = yyPythonRuntime("stop", map[string]any{"key": in.Key})
			} else {
				yyNativeStop(in.Native)
			}
			in.mu.Unlock()
			yySDKClose(in.Key)
		}
		yyRuntimeChoices.Lock()
		choices := []yyRuntimeChoice{}
		for _, choice := range yyRuntimeChoices.Items {
			if choice.ID == p.ID {
				choices = append(choices, choice)
			}
		}
		yyRuntimeChoices.Unlock()
		for _, choice := range choices {
			if e = yyRememberRuntime(choice.ID, choice.Account, false); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		}
		// dir is absolute and validated as one direct child of the runtime root above.
		if e = os.RemoveAll(dir); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": p.ID})
	})
}

func yyInstalledRuntimeFiles(ws *yyWorkspace, id string) error {
	dir, e := yyRuntimeDirectory(id)
	if e != nil {
		return e
	}
	var total int64
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if path == dir {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if rel == "data" || strings.HasPrefix(rel, "data"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("插件含符号链接，请先导出源码后导入")
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		total += info.Size()
		if total > 65<<20 {
			return fmt.Errorf("项目源码超过 65MB")
		}
		dest, e := yyProjectPath(yyWorkspaceRoot(ws), rel)
		if e != nil {
			return e
		}
		content, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return e
		}
		return os.WriteFile(dest, content, 0600)
	})
}
