package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// pluginManifest 插件可选的元数据（插件文件/<id>.manifest.json）。
type pluginManifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Author      string            `json:"author"`
	PrePlugins  map[string]string `json:"pre_plugins"` // 前置插件：id → 最低版本
}

// readPluginManifest 读取插件文件目录下可选的 <id>.manifest.json；不存在或解析失败返回 nil。
func readPluginManifest(dir, id string) *pluginManifest {
	data, err := os.ReadFile(filepath.Join(dir, id+".manifest.json"))
	if err != nil {
		return nil
	}
	var m pluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		debugLog("[插件] manifest 解析失败 id=%s err=%v", id, err)
		return nil
	}
	return &m
}

// readPluginDocs 读取插件文件目录下可选的 <id>.docs.md；不存在返回空串。
func readPluginDocs(dir, id string) string {
	data, err := os.ReadFile(filepath.Join(dir, id+".docs.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// BuildDefFromFile 根据磁盘上的插件文件构建 PluginDef。path 为插件代码文件的完整路径：
//   - <id>.lua：Lua 插件，id=文件名（去 .lua）
//   - <id>词库.json 或 <id>.json：词库插件，id 去掉 "词库" 后缀，Type=wordbank
//
// 可选元数据：<id>.manifest.json（name/version/description/type/author）、<id>.docs.md（Docs）。
// Load 闭包在 Run 时从磁盘读取代码并注册触发项（以 id 归属）。
func BuildDefFromFile(path string) (*PluginDef, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext != ".lua" && ext != ".json" {
		return nil, fmt.Errorf("不支持的插件文件类型: %s", base)
	}
	rawID := strings.TrimSuffix(base, ext)
	id := strings.TrimSuffix(rawID, "词库")
	if id == "" {
		id = rawID
	}

	typ := "lua"
	if ext == ".json" {
		typ = "wordbank"
	}
	name := base
	desc := ""
	var prePlugins map[string]string
	version := ""
	if man := readPluginManifest(dir, id); man != nil {
		if man.Name != "" {
			name = man.Name
		}
		if man.Description != "" {
			desc = man.Description
		}
		if man.Type != "" {
			typ = man.Type
		}
		prePlugins = man.PrePlugins
		version = man.Version
	}
	docs := readPluginDocs(dir, id)

	codePath := path
	loadID := id
	return &PluginDef{
		ID:          id,
		Name:        name,
		Version:     version,
		Type:        typ,
		Description: desc,
		Docs:        docs,
		PrePlugins:  prePlugins,
		SourceDir:   dir,
		SourceFile:  codePath,
		Load: func(bm *BotManager) error {
			if typ == "wordbank" {
				data, err := os.ReadFile(codePath)
				if err != nil {
					return err
				}
				triggers, err := loadWordBank(bm, data)
				if err != nil {
					return err
				}
				for _, t := range triggers {
					t.Plugin = loadID
					bm.AddChatTrigger(t)
				}
				return nil
			}
			src, err := os.ReadFile(codePath)
			if err != nil {
				return err
			}
			return LoadLuaPlugin(bm, loadID, string(src), dir)
		},
	}, nil
}
