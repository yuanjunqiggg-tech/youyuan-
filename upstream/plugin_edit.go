package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 新建插件代码模板（Lua）。占位注释引导用户从零开始写。
const luaPluginTemplate = `-- 插件：{{NAME}}
-- 这里写你的 Lua 插件代码。
-- 可用 API 见「插件管理 → 编辑 → API」。
return {}
`

// wordbankPluginTemplate 词库插件最小 JSON 结构。
const wordbankPluginTemplate = `{
  "菜单项": []
}
`

// CreatePlugin 新建一个插件并注册：按 type 生成代码模板 + manifest，
// 写入 插件文件/<id>/ 目录，随后 ScanPlugins 注册。返回生成的 id。
func (pc *PluginController) CreatePlugin(typ, name, desc string) (string, error) {
	if typ != "lua" && typ != "wordbank" {
		return "", fmt.Errorf("不支持的插件类型: %s（仅支持 lua / wordbank）", typ)
	}
	id := idFromName(name, func(candidate string) bool {
		_, err := os.Stat(filepath.Join(pc.privateDir, "插件文件", candidate))
		return os.IsNotExist(err)
	})
	if id == "" {
		id = "plugin"
	}
	if !validPluginID(id) {
		return "", fmt.Errorf("插件 id 非法: %q（名称含不允许的字符）", id)
	}

	plugDir := filepath.Join(pc.privateDir, "插件文件", id)
	if err := os.MkdirAll(plugDir, 0755); err != nil {
		return "", err
	}

	// 代码文件：lua → <id>.lua；wordbank → <id>词库.json
	codeName := id + ".lua"
	if typ == "wordbank" {
		codeName = id + "词库.json"
	}
	code := luaPluginTemplate
	if typ == "wordbank" {
		code = wordbankPluginTemplate
	}
	code = strings.ReplaceAll(code, "{{NAME}}", name)
	if err := os.WriteFile(filepath.Join(plugDir, codeName), []byte(code), 0644); err != nil {
		return "", err
	}

	// manifest：name/version/description/type/author
	man := map[string]any{
		"id":          id,
		"name":        name,
		"version":     "0.1.0",
		"description": desc,
		"type":        typ,
	}
	manData, _ := json.MarshalIndent(man, "", "  ")
	if err := os.WriteFile(filepath.Join(plugDir, id+".manifest.json"), manData, 0644); err != nil {
		return "", err
	}

	// 空操作文档（docs.md），编辑页「文档」标签可查看/续写
	if err := os.WriteFile(filepath.Join(plugDir, id+".docs.md"), []byte(""), 0644); err != nil {
		return "", err
	}

	// 注册（扫描到磁盘文件）
	_ = pc.ScanPlugins()
	debugLog("[插件] 创建 id=%s type=%s name=%s", id, typ, name)
	return id, nil
}

// idFromName 从名称生成合法 id：去掉 validPluginID 禁用的字符；
// 若与现有插件重名则追加 -2、-3… 直到 free(id) 返回 true。
func idFromName(name string, free func(id string) bool) string {
	var b strings.Builder
	for _, r := range name {
		switch r {
		case '/', '\\', ' ', '?', '#', '%', '=', '&':
			continue
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	base := b.String()
	if base == "" || base == "." || base == ".." {
		return ""
	}
	for i := 2; ; i++ {
		candidate := base
		if i > 2 {
			candidate = fmt.Sprintf("%s-%d", base, i-1)
		}
		if free(candidate) {
			return candidate
		}
	}
}

// CodeFileName 返回插件代码文件名（<id>.lua / main.lua / <id>词库.json / <id>.json），
// 按序探测第一个存在的；不存在返回 ok=false。
func (pc *PluginController) CodeFileName(id string) (string, bool) {
	dir := filepath.Join(pc.privateDir, "插件文件", id)
	for _, cand := range []string{id + ".lua", "main.lua", id + "词库.json", id + ".json"} {
		if _, err := os.Stat(filepath.Join(dir, cand)); err == nil {
			return cand, true
		}
	}
	return "", false
}

// resolvePluginFile 解析插件可编辑文件为磁盘绝对路径并做安全校验。
// scope: code (empty file means the legacy entry; otherwise a relative module),
// docs (legacy manual), data (relative resource path). Reject escaping/symlinks.
func (pc *PluginController) resolvePluginFile(scope, id, file string) (string, error) {
	if !validPluginID(id) {
		return "", fmt.Errorf("插件 id 非法: %q", id)
	}
	switch scope {
	case "code":
		if file != "" {
			return yyProjectPath(filepath.Join(pc.privateDir, "插件文件", id), file)
		}
		name, ok := pc.CodeFileName(id)
		if !ok {
			return "", fmt.Errorf("找不到插件代码文件: %s", id)
		}
		return filepath.Join(pc.privateDir, "插件文件", id, name), nil
	case "docs":
		return filepath.Join(pc.privateDir, "插件文件", id, id+".docs.md"), nil
	case "data":
		if file == "" || file == "." || file == ".." {
			return "", fmt.Errorf("非法文件名: %q", file)
		}
		return yyProjectPath(filepath.Join(pc.privateDir, "插件数据文件", id), file)
	default:
		return "", fmt.Errorf("未知文件范围: %s", scope)
	}
}

// ReadPluginFile 读取插件可编辑文件内容；二进制（含 \x00）返回明确错误。
// 返回实际文件名与内容。
func (pc *PluginController) ReadPluginFile(scope, id, file string) (string, string, error) {
	path, err := pc.resolvePluginFile(scope, id, file)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return filepath.Base(path), "", nil // 文件不存在视为空内容（如 docs 首次查看）
		}
		return "", "", err
	}
	if strings.IndexByte(string(data), 0) >= 0 {
		return "", "", fmt.Errorf("二进制文件不支持编辑")
	}
	return filepath.Base(path), string(data), nil
}

// WritePluginFile 把内容写回插件可编辑文件（父目录自动创建，0644）。
func (pc *PluginController) WritePluginFile(scope, id, file, content string) error {
	path, err := pc.resolvePluginFile(scope, id, file)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
