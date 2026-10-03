package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func yyUnpackPlugin(raw []byte, root string) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", err
	}
	var manifest yyRuntimeManifest
	paths := map[string]bool{}
	var total uint64
	for _, file := range archive.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		clean := filepath.Clean(filepath.FromSlash(name))
		if name == "" || strings.HasPrefix(name, "/") || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.Contains(name, ":") || file.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("ZIP 包含非法路径")
		}
		if paths[clean] {
			return "", fmt.Errorf("ZIP 包含重复路径")
		}
		paths[clean] = true
		total += file.UncompressedSize64
		if total > 64<<20 || len(archive.File) > 5000 {
			return "", fmt.Errorf("插件展开大小超过 64 MB 或文件数过多")
		}
		if name == "manifest.json" {
			reader, e := file.Open()
			if e != nil {
				return "", e
			}
			data, e := io.ReadAll(io.LimitReader(reader, 65537))
			reader.Close()
			if e != nil {
				return "", e
			}
			if len(data) > 65536 {
				return "", fmt.Errorf("manifest 过大")
			}
			if e = json.Unmarshal(data, &manifest); e != nil {
				return "", e
			}
		}
	}
	if manifest.ID == "" || manifest.ID == "." || filepath.Base(manifest.ID) != manifest.ID || strings.ContainsAny(manifest.ID, "/\\:") || manifest.ID == "native_counter" || (manifest.Language != "python" && manifest.Language != "native_cpp") || filepath.Base(manifest.Entry) != manifest.Entry || !paths[manifest.Entry] {
		return "", fmt.Errorf("根目录需要有效 Python / C++ manifest.json 与入口文件")
	}
	if _, e := os.Stat(filepath.Join(root, manifest.ID)); !os.IsNotExist(e) {
		return "", fmt.Errorf("插件已存在，请使用新标识导入以保留原数据")
	}
	temp, err := os.MkdirTemp(root, ".import-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	for _, file := range archive.File {
		target := filepath.Join(temp, filepath.FromSlash(strings.ReplaceAll(file.Name, "\\", "/")))
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "", err
		}
		reader, e := file.Open()
		if e != nil {
			return "", e
		}
		data, e := io.ReadAll(io.LimitReader(reader, (64<<20)+1))
		reader.Close()
		if e != nil {
			return "", e
		}
		if len(data) > 64<<20 {
			return "", fmt.Errorf("文件过大")
		}
		if err = os.WriteFile(target, data, 0644); err != nil {
			return "", err
		}
	}
	if manifest.Language == "native_cpp" {
		if e := yyValidateNative(filepath.Join(temp, manifest.Entry)); e != nil {
			return "", e
		}
	}
	if err = os.Rename(temp, filepath.Join(root, manifest.ID)); err != nil {
		return "", err
	}
	return manifest.ID, nil
}
func yyRuntimeImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "使用 POST 上传 ZIP", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 65<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	id, err := yyUnpackPlugin(raw, yyRuntimeRoot)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": id})
}
