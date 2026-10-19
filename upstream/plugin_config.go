package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigPath 返回插件配置文件路径：<privateDir>/插件配置文件/<id>/<id>.json
func (pc *PluginController) ConfigPath(id string) string {
	return filepath.Join(pc.privateDir, "插件配置文件", id, id+".json")
}

// DataDir 返回插件数据目录：<privateDir>/插件数据文件/<id>/，不存在时自动创建。
func (pc *PluginController) DataDir(id string) string {
	dir := filepath.Join(pc.privateDir, "插件数据文件", id)
	_ = os.MkdirAll(dir, 0755)
	return dir
}

// ReadConfig 读取插件配置文件为 map。文件缺失返回空 map，不报错。
func (pc *PluginController) ReadConfig(id string) (map[string]any, error) {
	cfg := map[string]any{}
	data, err := os.ReadFile(pc.ConfigPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// WriteConfig 把配置写回 <privateDir>/插件配置文件/<id>.json（0644，父目录自动创建）。
func (pc *PluginController) WriteConfig(id string, cfg map[string]any) error {
	path := pc.ConfigPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// UpdateMeta 更新插件 manifest 里的展示名与简介（用于「重命名插件 / 改小描述」）。
// name 非空才覆盖展示名；description 为空则清空。写入后重扫刷新列表。
func (pc *PluginController) UpdateMeta(id, name, description string) error {
	dir, err := pc.CodeDir(id)
	if err != nil {
		return err
	}
	manPath := filepath.Join(dir, id+".manifest.json")
	var man pluginManifest
	if data, err := os.ReadFile(manPath); err == nil {
		_ = json.Unmarshal(data, &man)
	}
	if man.ID == "" {
		man.ID = id
	}
	if name != "" {
		man.Name = name
	}
	man.Description = description
	data, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(manPath, data, 0644); err != nil {
		return err
	}
	return pc.ScanPlugins()
}

// EnsurePluginFiles 确保插件的数据目录与配置文件存在（插件加载时调用）。
// 数据目录自动创建；配置文件缺失时写空 {}，这样 getConfig 不返回空、插件数据有落点。
func (pc *PluginController) EnsurePluginFiles(id string) {
	_ = pc.DataDir(id)
	if _, err := os.Stat(pc.ConfigPath(id)); err != nil {
		_ = pc.WriteConfig(id, map[string]any{})
	}
}

// ListDataDir 返回插件数据目录下的文件名列表（不含子目录）。
func (pc *PluginController) ListDataDir(id string) ([]string, error) {
	dir := pc.DataDir(id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
