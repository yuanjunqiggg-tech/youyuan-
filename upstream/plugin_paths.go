package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// CodeDir keeps an installed package's actual source ownership while the
// existing plugin ID still owns its configuration, data and lifecycle.
func (pc *PluginController) CodeDir(id string) (string, error) {
	if !validPluginID(id) {
		return "", fmt.Errorf("插件 id 非法: %q", id)
	}
	pc.mu.Lock()
	base := filepath.Join(pc.privateDir, "插件文件")
	def := pc.defs[id]
	dir := filepath.Join(base, id)
	if def != nil && def.SourceDir != "" {
		dir = def.SourceDir
	}
	pc.mu.Unlock()
	rel, e := filepath.Rel(base, dir)
	if e != nil {
		return "", e
	}
	return yyProjectPath(base, rel)
}

// looseCodeFiles returns only explicitly owned siblings for the legacy loose
// file layout. It never walks the shared root or claims neighboring modules.
func (pc *PluginController) looseCodeFiles(id string) ([]string, bool, error) {
	dir, e := pc.CodeDir(id)
	if e != nil {
		return nil, false, e
	}
	base := filepath.Join(pc.privateDir, "插件文件")
	if filepath.Clean(dir) != filepath.Clean(base) {
		return nil, false, nil
	}
	pc.mu.Lock()
	def := pc.defs[id]
	entry := ""
	if def != nil {
		entry = def.SourceFile
	}
	pc.mu.Unlock()
	if entry == "" || filepath.Clean(filepath.Dir(entry)) != filepath.Clean(base) {
		return nil, true, fmt.Errorf("插件 %s 缺少明确入口归属", id)
	}
	files := []string{}
	for _, name := range []string{filepath.Base(entry), id + ".manifest.json", id + ".docs.md"} {
		owned, err := yyProjectPath(base, name)
		if err != nil {
			return nil, true, err
		}
		st, err := os.Stat(owned)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, true, err
		}
		if !st.Mode().IsRegular() {
			return nil, true, fmt.Errorf("插件文件不是普通文件: %s", name)
		}
		files = append(files, owned)
	}
	return files, true, nil
}

// Destructive/export operations require an isolated package tree. A loose
// source file may never authorize removing or archiving the shared root.
func (pc *PluginController) isolatedCodeDir(id string) (string, error) {
	dir, e := pc.CodeDir(id)
	if e != nil {
		return "", e
	}
	base := filepath.Join(pc.privateDir, "插件文件")
	if filepath.Clean(dir) == filepath.Clean(base) {
		return "", fmt.Errorf("插件 %s 为共享目录中的独立文件，请先备份并整理为独立插件目录", id)
	}
	return dir, nil
}
