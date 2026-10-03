package main

// PluginDef 一个插件定义。
type PluginDef struct {
	ID          string            // 唯一 id
	Name        string            // 展示名
	Version     string            // 版本（来自 manifest）
	Type        string            // "lua" | "wordbank"
	Description string            // 功能简介
	Docs        string            // Markdown 操作文档
	PrePlugins  map[string]string // 前置插件：id → 最低版本（加载前先加载）
	Load        func(bm *BotManager) error // 注册触发项
}

// builtinPlugins 内置插件。当前为空：插件由用户自行安装/加载。
var builtinPlugins = []PluginDef{}
