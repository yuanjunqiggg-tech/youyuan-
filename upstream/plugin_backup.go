package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const pluginBackupFormat = "youyuan.plugin.backup"

var pluginBackupRoots = []string{"插件文件", "插件配置文件", "插件数据文件"}

type pluginBackupManifest struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	ID      string `json:"id"`
}

func isFullPluginBackup(zr *zip.Reader) bool {
	for _, file := range zr.File {
		if file.Name == "plugin-package.json" {
			return true
		}
	}
	return false
}

// ExportPackage is a complete, private backup. The ZIP retains every regular
// file and empty directory below the plugin's source, config and data roots.
func (pc *PluginController) ExportPackage(id string) (string, error) {
	if !validPluginID(id) {
		return "", fmt.Errorf("插件 id 非法: %q", id)
	}
	source := filepath.Join(pc.privateDir, "插件文件", id)
	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("找不到插件目录: %s", id)
	}
	parent := filepath.Join(pc.privateDir, "插件文件")
	tmp, err := os.CreateTemp(parent, "."+id+"-backup-*.zip")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	zw := zip.NewWriter(tmp)
	manifest, _ := json.Marshal(pluginBackupManifest{Format: pluginBackupFormat, Version: 1, ID: id})
	meta, err := zw.Create("plugin-package.json")
	if err == nil {
		_, err = meta.Write(manifest)
	}
	if err == nil {
		for _, root := range pluginBackupRoots {
			dir := filepath.Join(pc.privateDir, root, id)
			if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
				// Include the empty root so restoring the ZIP recreates all three directories.
				_, err = zw.Create(root + "/" + id + "/")
			} else if statErr != nil {
				err = statErr
			} else {
				err = filepath.WalkDir(dir, func(filePath string, entry os.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if entry.Type()&os.ModeSymlink != 0 {
						return fmt.Errorf("插件目录包含符号链接，不能安全导出: %s", filePath)
					}
					info, infoErr := entry.Info()
					if infoErr != nil {
						return infoErr
					}
					if !entry.IsDir() && !info.Mode().IsRegular() {
						return fmt.Errorf("插件目录包含不支持的文件类型: %s", filePath)
					}
					rel, relErr := filepath.Rel(pc.privateDir, filePath)
					if relErr != nil {
						return relErr
					}
					header, headerErr := zip.FileInfoHeader(info)
					if headerErr != nil {
						return headerErr
					}
					header.Name = filepath.ToSlash(rel)
					if entry.IsDir() {
						header.Name += "/"
						header.Method = zip.Store
					} else {
						header.Method = zip.Deflate
					}
					writer, createErr := zw.CreateHeader(header)
					if createErr != nil || entry.IsDir() {
						return createErr
					}
					file, openErr := os.Open(filePath)
					if openErr != nil {
						return openErr
					}
					_, copyErr := io.Copy(writer, file)
					closeErr := file.Close()
					if copyErr != nil {
						return copyErr
					}
					return closeErr
				})
			}
			if err != nil {
				break
			}
		}
	}
	if closeErr := zw.Close(); err == nil {
		err = closeErr
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	zipPath := filepath.Join(parent, id+".zip")
	if removeErr := os.Remove(zipPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return "", removeErr
	}
	if err := os.Rename(tmpPath, zipPath); err != nil {
		return "", err
	}
	return zipPath, nil
}

func (pc *PluginController) importFullPluginBackup(zr *zip.Reader) (string, error) {
	var manifest pluginBackupManifest
	var marker *zip.File
	for _, file := range zr.File {
		if file.Name == "plugin-package.json" {
			marker = file
			break
		}
	}
	if marker == nil || marker.UncompressedSize64 > 8192 {
		return "", fmt.Errorf("插件备份标记无效")
	}
	rc, err := marker.Open()
	if err != nil {
		return "", err
	}
	err = json.NewDecoder(io.LimitReader(rc, 8193)).Decode(&manifest)
	rc.Close()
	if err != nil || manifest.Format != pluginBackupFormat || manifest.Version != 1 || !validPluginID(manifest.ID) {
		return "", fmt.Errorf("插件备份格式或 id 无效")
	}
	const maxEntries = 10000
	const maxTotal = uint64(2 << 30)
	const maxFile = uint64(512 << 20)
	if len(zr.File) > maxEntries {
		return "", fmt.Errorf("插件备份文件过多")
	}
	var total uint64
	seen := map[string]bool{}
	for _, file := range zr.File {
		name := file.Name
		if name == "plugin-package.json" {
			continue
		}
		clean := strings.TrimSuffix(name, "/")
		if clean == "" || strings.ContainsAny(name, "\\:") || path.Clean(clean) != clean || strings.HasPrefix(clean, "/") || seen[clean] {
			return "", fmt.Errorf("插件备份包含非法或重复路径: %s", name)
		}
		seen[clean] = true
		parts := strings.Split(clean, "/")
		validRoot := false
		for _, root := range pluginBackupRoots {
			if parts[0] == root {
				validRoot = true
				break
			}
		}
		if !validRoot || len(parts) < 2 || parts[1] != manifest.ID || (len(parts) == 2 && !file.FileInfo().IsDir()) {
			return "", fmt.Errorf("插件备份路径不属于插件 %s: %s", manifest.ID, name)
		}
		for _, segment := range parts {
			if segment == "" || segment == "." || segment == ".." {
				return "", fmt.Errorf("插件备份路径无效: %s", name)
			}
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("插件备份不接受符号链接: %s", name)
		}
		if !file.FileInfo().IsDir() {
			if !file.Mode().IsRegular() || file.UncompressedSize64 > maxFile || total > maxTotal-file.UncompressedSize64 {
				return "", fmt.Errorf("插件备份文件过大或类型不支持: %s", name)
			}
			total += file.UncompressedSize64
		}
	}
	staging, err := os.MkdirTemp(pc.privateDir, ".plugin-restore-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	for _, file := range zr.File {
		if file.Name == "plugin-package.json" {
			continue
		}
		dest := filepath.Join(staging, filepath.FromSlash(file.Name))
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(dest, 0755); err != nil {
				return "", err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return "", err
		}
		rc, openErr := file.Open()
		if openErr != nil {
			return "", openErr
		}
		out, createErr := os.Create(dest)
		if createErr != nil {
			rc.Close()
			return "", createErr
		}
		written, copyErr := io.Copy(out, io.LimitReader(rc, int64(file.UncompressedSize64)+1))
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil || closeErr != nil || uint64(written) != file.UncompressedSize64 {
			return "", fmt.Errorf("插件备份文件解压失败: %s", file.Name)
		}
	}
	for _, root := range pluginBackupRoots {
		staged := filepath.Join(staging, root, manifest.ID)
		if _, statErr := os.Stat(staged); os.IsNotExist(statErr) {
			continue
		} else if statErr != nil {
			return "", statErr
		}
		target := filepath.Join(pc.privateDir, root, manifest.ID)
		if err = copyPluginBackupTree(staged, target); err != nil {
			return "", err
		}
	}
	return manifest.ID, nil
}

func copyPluginBackupTree(source, target string) error {
	return filepath.WalkDir(source, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, filePath)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		in, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
