package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Runtime output is data produced by a plugin, not its source/config backup.
func yyOutputInfo(r yyResourceRoots, name string) (map[string]any, error) {
	path, e := yyProjectPath(r.Data, name)
	if e != nil {
		return nil, e
	}
	st, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("只能下载普通文件")
	}
	rel, _ := filepath.Rel(r.Data, path)
	q := url.Values{"id": {filepath.Base(r.Code)}, "file": {filepath.ToSlash(rel)}}
	if r.Kind == "runtime" {
		q.Set("kind", "runtime")
		q.Set("runtime_account", filepath.Base(r.Data))
	}
	return map[string]any{"plugin_id": filepath.Base(r.Code), "kind": q.Get("kind"), "path": filepath.ToSlash(rel), "name": st.Name(), "bytes": st.Size(), "modified_at": st.ModTime().Format("2006-01-02T15:04:05.999999999Z07:00"), "download_url": "/api/youyuan/plugin-outputs/download?" + q.Encode(), "snapshot": false}, nil
}

func yyListOutputs(r yyResourceRoots, name string) ([]map[string]any, error) {
	root, e := yyProjectPath(r.Data, name)
	if e != nil {
		return nil, e
	}
	rows := []map[string]any{}
	e = filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if os.IsNotExist(e) && path == root {
			return nil
		}
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".tmp" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, e := filepath.Rel(r.Data, path)
		if e != nil {
			return e
		}
		v, e := yyOutputInfo(r, rel)
		if e != nil {
			return e
		}
		rows = append(rows, v)
		return nil
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i]["modified_at"].(string) > rows[j]["modified_at"].(string) })
	return rows, e
}

func yyOutputRoots(req *http.Request) (yyResourceRoots, error) {
	if _, e := yyExportActor(req, false); e != nil {
		return yyResourceRoots{}, e
	}
	// The parent accounts proxy also guards ownership; direct access must do so too.
	if e := yyPoolInvocationAllowed(req, "main"); e != nil {
		return yyResourceRoots{}, e
	}
	id := req.URL.Query().Get("id")
	if !validPluginID(id) {
		return yyResourceRoots{}, fmt.Errorf("插件标识无效")
	}
	switch req.URL.Query().Get("kind") {
	case "", "lua":
		if pluginController == nil {
			return yyResourceRoots{}, fmt.Errorf("Lua宿主未就绪")
		}
		pluginController.mu.Lock()
		private := pluginController.privateDir
		pluginController.mu.Unlock()
		code, e := yyProjectPath(filepath.Join(private, "插件文件"), id)
		if e != nil {
			return yyResourceRoots{}, e
		}
		if st, e := os.Stat(code); e != nil || !st.IsDir() {
			return yyResourceRoots{}, os.ErrNotExist
		}
		return yyResourceRoots{Code: code, Data: filepath.Join(private, "插件数据文件", id)}, nil
	case "runtime":
		code, e := yyRuntimeDirectory(id)
		if e != nil {
			return yyResourceRoots{}, e
		}
		if _, e = os.Stat(filepath.Join(code, "manifest.json")); e != nil {
			return yyResourceRoots{}, e
		}
		account := req.URL.Query().Get("runtime_account")
		if account == "" {
			account = "main"
		}
		if !yyManagedAccountID(account) || strings.ContainsAny(account, "/\\:") || strings.HasPrefix(account, ".") {
			return yyResourceRoots{}, fmt.Errorf("账号标识无效")
		}
		if e = yyPoolInvocationAllowed(req, account); e != nil {
			return yyResourceRoots{}, e
		}
		return yyResourceRoots{Code: code, Data: filepath.Join(code, "data", account), Kind: "runtime"}, nil
	}
	return yyResourceRoots{}, fmt.Errorf("kind为lua或runtime")
}

func yyPluginOutputRoutes(mux *http.ServeMux) {
	for _, suffix := range []string{"", "/info", "/download"} {
		suffix := suffix
		mux.HandleFunc("/api/youyuan/plugin-outputs"+suffix, func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodGet && !(suffix == "/download" && req.Method == http.MethodHead) {
				http.Error(w, "使用GET", 405)
				return
			}
			roots, e := yyOutputRoots(req)
			if e != nil {
				http.Error(w, e.Error(), 403)
				return
			}
			w.Header().Set("Cache-Control", "private, no-store")
			if suffix == "" {
				rows, e := yyListOutputs(roots, ".")
				if e != nil {
					writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
					return
				}
				writeJSON(w, map[string]any{"ok": true, "items": rows, "storage": "当前账号执行设备上的插件数据目录"})
				return
			}
			name := req.URL.Query().Get("file")
			v, e := yyOutputInfo(roots, name)
			if e != nil {
				http.Error(w, "文件不存在或路径不允许", 404)
				return
			}
			if suffix == "/info" {
				writeJSON(w, map[string]any{"ok": true, "file": v, "downloaded": false})
				return
			}
			path, e := yyProjectPath(roots.Data, name)
			if e != nil {
				http.NotFound(w, req)
				return
			}
			f, e := os.Open(path)
			if e != nil {
				http.NotFound(w, req)
				return
			}
			defer f.Close()
			st, e := f.Stat()
			if e != nil || !st.Mode().IsRegular() {
				http.NotFound(w, req)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(st.Name()))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			http.ServeContent(w, req, st.Name(), st.ModTime(), f)
		})
	}
}
