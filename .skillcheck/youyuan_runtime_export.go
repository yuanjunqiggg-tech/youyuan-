package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Export a whole Python/C++ plugin directory with importable root layout.
func yyRuntimeExportPackage(root, id string) (string, error) {
	if !validPluginID(id) {
		return "", fmt.Errorf("插件 id 无效")
	}
	dir := filepath.Join(root, id)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("插件目录不存在")
	}
	temp, err := os.CreateTemp(root, "."+id+"-export-*.zip")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	writer := zip.NewWriter(temp)
	err = filepath.WalkDir(dir, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == dir {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("插件含符号链接，不能安全导出")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !entry.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("插件含不支持的文件类型")
		}
		rel, err := filepath.Rel(dir, filePath)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if entry.IsDir() {
			header.Name += "/"
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}
		out, err := writer.CreateHeader(header)
		if err != nil || entry.IsDir() {
			return err
		}
		in, err := os.Open(filePath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	output := filepath.Join(root, id+".zip")
	if removeErr := os.Remove(output); removeErr != nil && !os.IsNotExist(removeErr) {
		return "", removeErr
	}
	if err = os.Rename(tempPath, output); err != nil {
		return "", err
	}
	return output, nil
}

func yyRuntimeExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "使用 GET 或 POST", http.StatusMethodNotAllowed)
		return
	}
	if _, err := yyExportActor(r, true); err != nil {
		http.Error(w, err.Error(), 403)
		return
	}
	yyPluginExportMu.Lock()
	defer yyPluginExportMu.Unlock()
	id := r.URL.Query().Get("id")
	path, err := yyRuntimeExportPackage(yyRuntimeRoot, id)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if r.Method == http.MethodPost {
		yyExportResponse(w, r, path, id, "runtime")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=plugin.zip")
	http.ServeFile(w, r, path)
}
