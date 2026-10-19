package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type yyPluginExport struct {
	ID       string `json:"id"`
	PluginID string `json:"plugin_id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Owner    string `json:"-"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
	Created  string `json:"created_at"`
	Download string `json:"download_url"`
}
type yyPluginExportDisk struct {
	yyPluginExport
	OwnerID string `json:"owner_id"`
}

var yyExportDirectory string
var yyPluginExportMu sync.Mutex

func yyExportID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, e := hex.DecodeString(id)
	return e == nil && strings.ToLower(id) == id
}
func yyExportActor(r *http.Request, write bool) (yyPoolActor, error) {
	a, e := yyPoolActorRequest(r)
	// A child engine is only reached through its loopback parent proxy, which
	// already checks account ownership. It has pool-parent.json, not a separate
	// platform login; use its assigned owner rather than requiring another login.
	if (e != nil || a.ID == "") && yyAccounts.Worker && !yyPoolHasActor(r) && r.Header.Get("X-YouYuan-Pool-Source") == "" && r.Header.Get("X-YouYuan-Pool-Secret") == "" {
		raw, readErr := os.ReadFile(filepath.Join(yyAccounts.Root, "pool-parent.json"))
		var child yyPoolChildIdentity
		if readErr == nil && json.Unmarshal(raw, &child) == nil && child.Owner != "" && child.Source != "" && child.Secret != "" {
			a = yyPoolActor{ID: child.Owner, Scope: "control"}
			e = nil
		}
	}
	if e != nil || a.ID == "" {
		return a, fmt.Errorf("请先登录自己的游园账号")
	}
	if write && a.Scope == "read" {
		return a, fmt.Errorf("当前会话只有读取权限")
	}
	return a, nil
}
func yyRetainExport(root, source, id, kind string, actor yyPoolActor) (yyPluginExport, error) {
	v := yyPluginExport{PluginID: id, Kind: kind, Owner: actor.ID, Created: time.Now().Format(time.RFC3339Nano)}
	if !validPluginID(id) || actor.ID == "" {
		return v, fmt.Errorf("缺少有效插件或用户")
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return v, e
	}
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return v, e
	}
	v.ID = hex.EncodeToString(b)
	info, e := os.Lstat(source)
	if e != nil {
		return v, e
	}
	if !info.Mode().IsRegular() {
		return v, fmt.Errorf("导出源不是普通文件")
	}
	in, e := os.Open(source)
	if e != nil {
		return v, e
	}
	defer in.Close()
	file := filepath.Join(root, v.ID+".zip")
	out, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return v, e
	}
	success := false
	defer func() {
		if !success {
			os.Remove(file)
			os.Remove(filepath.Join(root, v.ID+".json"))
		}
	}()
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(out, h), in)
	ce := out.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return v, e
	}
	v.Bytes = n
	v.SHA256 = hex.EncodeToString(h.Sum(nil))
	v.Name = id + "-" + time.Now().Format("20060102-150405") + ".zip"
	v.Download = "/api/youyuan/exports/download?id=" + v.ID
	raw, e := json.MarshalIndent(yyPluginExportDisk{yyPluginExport: v, OwnerID: actor.ID}, "", "  ")
	if e != nil {
		return v, e
	}
	if e = os.WriteFile(filepath.Join(root, v.ID+".json"), raw, 0600); e != nil {
		return v, e
	}
	success = true
	return v, nil
}
func yyExportRecord(root, id string, actor yyPoolActor) (yyPluginExport, error) {
	if !yyExportID(id) {
		return yyPluginExport{}, os.ErrNotExist
	}
	raw, e := os.ReadFile(filepath.Join(root, id+".json"))
	if e != nil {
		return yyPluginExport{}, e
	}
	var d yyPluginExportDisk
	if e = json.Unmarshal(raw, &d); e != nil {
		return d.yyPluginExport, e
	}
	if d.ID != id || d.OwnerID == "" || (!actor.Admin && d.OwnerID != actor.ID) {
		return yyPluginExport{}, os.ErrNotExist
	}
	d.Owner = d.OwnerID
	return d.yyPluginExport, nil
}
func yyExportResponse(w http.ResponseWriter, r *http.Request, source, id, kind string) {
	a, e := yyExportActor(r, true)
	if e != nil {
		http.Error(w, e.Error(), 403)
		return
	}
	v, e := yyRetainExport(yyExportDirectory, source, id, kind, a)
	if e != nil {
		writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
		return
	}
	stable := filepath.Join(yyExportDirectory, v.ID+".zip")
	writeJSON(w, map[string]any{"ok": true, "path": stable, "src_path": stable, "name": v.Name, "export": v, "download_url": v.Download, "export_id": v.ID})
}
func yyExportCenterRoutes(mux *http.ServeMux, dataDir string) {
	root := filepath.Join(dataDir, "插件导出")
	yyExportDirectory = root
	mux.HandleFunc("/api/youyuan/exports", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "使用GET查看导出记录", 405)
			return
		}
		a, e := yyExportActor(r, false)
		if e != nil {
			http.Error(w, e.Error(), 403)
			return
		}
		files, e := os.ReadDir(root)
		if e != nil && !os.IsNotExist(e) {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		items := []yyPluginExport{}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			v, e := yyExportRecord(root, strings.TrimSuffix(f.Name(), ".json"), a)
			if e != nil {
				continue
			}
			if _, e = os.Stat(filepath.Join(root, v.ID+".zip")); e == nil {
				items = append(items, v)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Created > items[j].Created })
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, map[string]any{"ok": true, "items": items, "storage": "当前执行设备的私有插件导出目录"})
	})
	mux.HandleFunc("/api/youyuan/exports/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "使用GET下载", 405)
			return
		}
		a, e := yyExportActor(r, false)
		if e != nil {
			http.Error(w, e.Error(), 403)
			return
		}
		v, e := yyExportRecord(root, r.URL.Query().Get("id"), a)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(root, v.ID+".zip")
		info, e := os.Lstat(path)
		if e != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(v.Name))
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Content-SHA256", v.SHA256)
		http.ServeFile(w, r, path)
	})
	mux.HandleFunc("/api/youyuan/exports/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "使用GET", 405)
			return
		}
		a, e := yyExportActor(r, false)
		if e != nil {
			http.Error(w, e.Error(), 403)
			return
		}
		v, e := yyExportRecord(root, r.URL.Query().Get("id"), a)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "export": v, "download_url": v.Download, "content_type": "application/zip", "downloaded": false, "note": "这是经权限校验的下载信息；文件需由登录后的浏览器下载，不把ZIP二进制装入AI上下文。"})
	})
	mux.HandleFunc("/api/youyuan/exports/remove", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "使用POST删除备份", 405)
			return
		}
		a, e := yyExportActor(r, true)
		if e != nil {
			http.Error(w, e.Error(), 403)
			return
		}
		var q struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil {
			http.Error(w, "无效JSON", 400)
			return
		}
		v, e := yyExportRecord(root, q.ID, a)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		for _, ext := range []string{".zip", ".json"} {
			if e = os.Remove(filepath.Join(root, v.ID+ext)); e != nil && !os.IsNotExist(e) {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		}
		writeJSON(w, map[string]any{"ok": true, "id": v.ID, "plugin_deleted": false})
	})
}
