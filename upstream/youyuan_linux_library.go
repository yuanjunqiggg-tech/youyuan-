package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var yyLinuxLibraryRoot string

type yyLinuxLibraryItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
	CreatedAt string `json:"created_at"`
}

func yyLocalLibrary(w http.ResponseWriter, r *http.Request) {
	root := yyLinuxLibraryRoot
	action := strings.TrimPrefix(r.URL.Path, "/api/youyuan/library/")
	fail := func(e error) { writeJSON(w, map[string]any{"ok": false, "error": e.Error()}) }
	if action == "list" {
		items := []yyLinuxLibraryItem{}
		files, e := os.ReadDir(root)
		if e != nil && !os.IsNotExist(e) {
			fail(e)
			return
		}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".json") {
				b, e := os.ReadFile(filepath.Join(root, f.Name()))
				if e == nil {
					var v yyLinuxLibraryItem
					if json.Unmarshal(b, &v) == nil {
						items = append(items, v)
					}
				}
			}
		}
		writeJSON(w, map[string]any{"ok": true, "items": items, "platform": "linux", "storage": "当前Linux服务器磁盘"})
		return
	}
	if action == "upload" && r.Method == "POST" {
		if e := r.ParseMultipartForm(1 << 20); e != nil {
			fail(e)
			return
		}
		defer r.MultipartForm.RemoveAll()
		cat := r.FormValue("category")
		if cat != "plugins" && cat != "buildings" && cat != "music" {
			http.Error(w, "资料分类无效", 400)
			return
		}
		file, head, e := r.FormFile("file")
		if e != nil {
			fail(e)
			return
		}
		defer file.Close()
		if e = os.MkdirAll(root, 0700); e != nil {
			fail(e)
			return
		}
		id := yyNewKey()[:32]
		path := filepath.Join(root, id+".data")
		out, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			fail(e)
			return
		}
		hash := sha256.New()
		size, e := io.Copy(io.MultiWriter(out, hash), file)
		ce := out.Close()
		if e == nil {
			e = ce
		}
		if e != nil {
			os.Remove(path)
			fail(e)
			return
		}
		item := yyLinuxLibraryItem{ID: id, Name: head.Filename, Category: cat, Bytes: size, SHA256: hex.EncodeToString(hash.Sum(nil)), CreatedAt: time.Now().Format(time.RFC3339)}
		b, _ := json.Marshal(item)
		if e = os.WriteFile(filepath.Join(root, id+".json"), b, 0600); e != nil {
			os.Remove(path)
			fail(e)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "item": item, "platform": "linux", "storage": "当前Linux服务器磁盘"})
		return
	}
	id := r.URL.Query().Get("id")
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" {
		http.Error(w, "资料标识无效", 400)
		return
	}
	meta := filepath.Join(root, id+".json")
	b, e := os.ReadFile(meta)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	var item yyLinuxLibraryItem
	if e = json.Unmarshal(b, &item); e != nil {
		fail(e)
		return
	}
	if action == "download" {
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(item.Name))
		w.Header().Set("X-Content-SHA256", item.SHA256)
		http.ServeFile(w, r, filepath.Join(root, id+".data"))
		return
	}
	if action == "delete" && r.Method == "POST" {
		if e = os.Remove(filepath.Join(root, id+".data")); e != nil {
			fail(e)
			return
		}
		if e = os.Remove(meta); e != nil {
			fail(e)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	http.NotFound(w, r)
}
