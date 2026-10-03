package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Only the selected plugin's source tree is imported. Application credentials,
// other plugins and the Agent's own task/history directory are outside this root.
func yyLuaPluginSource(id string) (string, string, error) {
	if pluginController == nil || !validPluginID(id) {
		return "", "", fmt.Errorf("插件标识无效")
	}
	root := filepath.Join(pluginController.privateDir, "插件文件")
	dir, e := yyProjectPath(root, id)
	if e != nil {
		return "", "", e
	}
	entry, ok := pluginController.CodeFileName(id)
	if !ok {
		return "", "", fmt.Errorf("找不到插件 %s 的实际源码，未创建空壳工作区", id)
	}
	return dir, entry, nil
}

func yyBindLuaPlugin(ws *yyWorkspace) error {
	dir, entry, e := yyLuaPluginSource(ws.PluginID)
	if e != nil {
		return e
	}
	files := map[string][]byte{}
	var total int64
	e = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("源码目录包含符号链接，未导入")
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if total > 65<<20 || len(files) >= 2000 {
			return fmt.Errorf("插件源码超过项目容量，请分批导入")
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, err = yyProjectPath(yyWorkspaceRoot(ws), rel); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = data
		return nil
	})
	if e != nil {
		return e
	}
	// A three-way comparison preserves local edits and refuses concurrent changes.
	yyFileMu.Lock()
	defer yyFileMu.Unlock()
	ws.Lock()
	defer ws.Unlock()
	if ws.cancel != nil {
		return fmt.Errorf("请先暂停 AI 任务再同步源码")
	}
	legacyAlias := ""
	if entry != "main.lua" && files["main.lua"] == nil {
		path, err := yyProjectPath(yyWorkspaceRoot(ws), "main.lua")
		if err != nil {
			return err
		}
		if data, err := os.ReadFile(path); err == nil {
			// Old ReadPluginFile call accidentally copied its filename return into
			// main.lua. This is a recognized invalid placeholder, not user code.
			if strings.TrimSpace(string(data)) == entry || (ws.SourceEntry == "" && string(data) == string(files[entry])) {
				legacyAlias = path
			}
		}
	}
	hashes := map[string]string{}
	changes := map[string][]byte{}
	for path, data := range files {
		dest, err := yyProjectPath(yyWorkspaceRoot(ws), path)
		if err != nil {
			return err
		}
		before, err := os.ReadFile(dest)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		old := ws.SourceHashes[path]
		live := yyFileHash(string(data))
		current := yyFileHash(string(before))
		if err == nil && current != live && (old == "" || (old != current && old != live)) {
			return fmt.Errorf("源码同步冲突：%s；工作区修改已保留，请先比较或发布", path)
		}
		if err != nil || current == old {
			changes[path] = data
		}
		hashes[path] = live
	}
	for path, data := range changes {
		dest, _ := yyProjectPath(yyWorkspaceRoot(ws), path)
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return e
		}
		if e = os.WriteFile(dest, data, 0600); e != nil {
			return e
		}
	}
	ws.SourceEntry = entry
	ws.SourceHashes = hashes
	if legacyAlias != "" {
		if e = os.Remove(legacyAlias); e != nil {
			return e
		}
	}
	return ws.saveLocked()
}

func yyPublishBoundLua(ws *yyWorkspace) error {
	yyFileMu.Lock()
	defer yyFileMu.Unlock()
	dir, _, e := yyLuaPluginSource(ws.PluginID)
	if e != nil {
		return e
	}
	ws.Lock()
	hashes := map[string]string{}
	for k, v := range ws.SourceHashes {
		hashes[k] = v
	}
	ws.Unlock()
	changes := map[string][]byte{}
	for _, file := range yyProjectFiles(ws) {
		path := file["path"].(string)
		if path == "command-options.json" || strings.HasPrefix(path, "data/") || strings.HasPrefix(path, "attachments/") {
			continue
		}
		data, e := yyReadProjectRaw(ws, path)
		if e != nil {
			return e
		}
		old, tracked := hashes[path]
		updated := yyFileHash(string(data))
		if tracked && old == updated {
			continue
		}
		target, e := yyProjectPath(dir, path)
		if e != nil {
			return e
		}
		before, e := os.ReadFile(target)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if (tracked && yyFileHash(string(before)) != old) || (!tracked && e == nil && string(before) != string(data)) {
			return fmt.Errorf("已安装源码已被其他编辑修改：%s；请先同步并合并", path)
		}
		changes[path] = data
	}
	// Check every destination before writing any file; do not overwrite conflicts.
	for path, data := range changes {
		target, _ := yyProjectPath(dir, path)
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		temp := target + ".yy-publish"
		if e = os.WriteFile(temp, data, 0600); e != nil {
			return e
		}
		if e = os.Rename(temp, target); e != nil {
			return e
		}
		hashes[path] = yyFileHash(string(data))
	}
	ws.Lock()
	defer ws.Unlock()
	ws.SourceHashes = hashes
	return ws.saveLocked()
}
