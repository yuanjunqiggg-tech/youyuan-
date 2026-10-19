package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func yyHostFilesRoutes(mux *http.ServeMux, dataDir string) {
	yyUploadSessionRoutes(mux, dataDir)
	yyLinuxLibraryRoot = filepath.Join(dataDir, "本地资料库")
	root := filepath.Join(dataDir, "用户上传")
	valid := func(name string) bool {
		return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00")
	}
	mux.HandleFunc("/api/youyuan/files", func(w http.ResponseWriter, r *http.Request) {
		entries, e := os.ReadDir(root)
		if e != nil && !os.IsNotExist(e) {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		items := []map[string]any{}
		for _, f := range entries {
			if !f.IsDir() {
				if s, e := f.Info(); e == nil {
					items = append(items, map[string]any{"name": f.Name(), "bytes": s.Size(), "modified": s.ModTime().Format(time.RFC3339), "path": filepath.Join(root, f.Name())})
				}
			}
		}
		writeJSON(w, map[string]any{"ok": true, "storage": "工具箱当前执行设备", "platform": getPlatformName(), "directory": root, "items": items})
	})
	mux.HandleFunc("/api/youyuan/files/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		if e := r.ParseMultipartForm(1 << 20); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, head, e := r.FormFile("file")
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		defer file.Close()
		name := head.Filename
		if !valid(name) {
			writeJSON(w, map[string]any{"ok": false, "error": "文件名包含路径"})
			return
		}
		if e = os.MkdirAll(root, 0700); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		base, ext := strings.TrimSuffix(name, filepath.Ext(name)), filepath.Ext(name)
		var dst *os.File
		for i := 0; i < 100; i++ {
			candidate := name
			if i > 0 {
				candidate = fmt.Sprintf("%s (%d)%s", base, i, ext)
			}
			dst, e = os.OpenFile(filepath.Join(root, candidate), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e == nil {
				name = candidate
				break
			}
			if !os.IsExist(e) {
				break
			}
		}
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		n, e := io.Copy(dst, file)
		ce := dst.Close()
		if e == nil {
			e = ce
		}
		if e != nil {
			os.Remove(dst.Name())
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "name": name, "bytes": n, "path": dst.Name(), "platform": getPlatformName()})
	})
	mux.HandleFunc("/api/youyuan/files/download", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if !valid(name) {
			http.Error(w, "无效文件名", 400)
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
		http.ServeFile(w, r, filepath.Join(root, name))
	})
}
