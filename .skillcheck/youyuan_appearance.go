package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type yyAppearance struct {
	Background string   `json:"background"`
	Covers     []string `json:"covers"`
	Opacity    float64  `json:"opacity"`
}

var yyAppearanceMu sync.Mutex

func yyAppearanceID(id string) bool {
	if len(id) != 64 {
		return false
	}
	_, e := hex.DecodeString(id)
	return e == nil
}
func yyAppearanceRoutes(mux *http.ServeMux, dataDir string) {
	dir := filepath.Join(dataDir, "appearance")
	_ = os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "config.json")
	read := func() yyAppearance {
		p := yyAppearance{Covers: []string{}, Opacity: 0.88}
		if data, e := os.ReadFile(path); e == nil {
			_ = json.Unmarshal(data, &p)
		}
		return p
	}
	save := func(p yyAppearance) error {
		data, e := json.MarshalIndent(p, "", "  ")
		if e != nil {
			return e
		}
		if e = os.WriteFile(path+".tmp", data, 0600); e != nil {
			return e
		}
		return os.Rename(path+".tmp", path)
	}
	mux.HandleFunc("/api/youyuan/appearance", func(w http.ResponseWriter, r *http.Request) {
		yyAppearanceMu.Lock()
		defer yyAppearanceMu.Unlock()
		p := read()
		if r.Method == "POST" {
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&p); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			all := append([]string{}, p.Covers...)
			if p.Background != "" {
				all = append(all, p.Background)
			}
			for _, id := range all {
				if !yyAppearanceID(id) {
					writeJSON(w, map[string]any{"ok": false, "error": "图片标识无效"})
					return
				}
				if _, e := os.Stat(filepath.Join(dir, id)); e != nil {
					writeJSON(w, map[string]any{"ok": false, "error": "图片文件不存在"})
					return
				}
			}
			if len(p.Covers) > 20 {
				writeJSON(w, map[string]any{"ok": false, "error": "当前工作台支持 20 张自定义封面"})
				return
			}
			if p.Opacity < 0.5 {
				p.Opacity = 0.5
			}
			if p.Opacity > 1 {
				p.Opacity = 1
			}
			if e := save(p); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
		}
		writeJSON(w, map[string]any{"ok": true, "appearance": p})
	})
	mux.HandleFunc("/api/youyuan/appearance/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
		if e := r.ParseMultipartForm(9 << 20); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, _, e := r.FormFile("file")
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		defer file.Close()
		data, e := io.ReadAll(io.LimitReader(file, (8<<20)+1))
		if e != nil || len(data) > 8<<20 {
			writeJSON(w, map[string]any{"ok": false, "error": "单张图片不超过 8 MB"})
			return
		}
		cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 16384 || cfg.Height > 16384 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
			writeJSON(w, map[string]any{"ok": false, "error": "请使用有效的 JPEG/PNG 图片，最多 4000 万像素"})
			return
		}
		hash := sha256.Sum256(data)
		id := fmt.Sprintf("%x", hash)
		if e = os.WriteFile(filepath.Join(dir, id), data, 0600); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": id, "width": cfg.Width, "height": cfg.Height})
	})
	mux.HandleFunc("/api/youyuan/appearance/file", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if !yyAppearanceID(id) {
			http.Error(w, "invalid image", 400)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=86400")
		http.ServeFile(w, r, filepath.Join(dir, id))
	})
}
