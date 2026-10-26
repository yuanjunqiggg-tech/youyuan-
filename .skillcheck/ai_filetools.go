package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// aiDebugLogPath 允许 AI 读取的插件调试日志（沙箱白名单例外）。
const aiDebugLogPath = "/storage/emulated/0/Download/prism_debug.log"

// aiSessionModifiedFiles 记录本次会话中 AI 修改过的插件文件（绝对路径）。
// 用于「存档/读档」快照。只记录插件目录内文件。
var aiSessionModified = struct {
	sync.Mutex
	files map[string]bool
}{files: map[string]bool{}}

func aiNoteModified(path string) {
	aiSessionModified.Lock()
	aiSessionModified.files[path] = true
	aiSessionModified.Unlock()
}

func aiModifiedList() []string {
	aiSessionModified.Lock()
	defer aiSessionModified.Unlock()
	out := make([]string, 0, len(aiSessionModified.files))
	for p := range aiSessionModified.files {
		out = append(out, p)
	}
	return out
}

func aiResetModified() {
	aiSessionModified.Lock()
	aiSessionModified.files = map[string]bool{}
	aiSessionModified.Unlock()
}

// aiPluginSubdirs 允许 AI 读写的插件相关目录（相对 privateDir）。
var aiPluginSubdirs = []string{"插件文件", "插件数据文件", "插件配置文件"}

// resolveAIFilePath 沙箱解析：只允许插件目录内文件 + 调试日志白名单，拒绝系统路径/越界。
func resolveAIFilePath(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("路径不能为空")
	}
	if filepath.Clean(raw) == filepath.Clean(aiDebugLogPath) {
		return aiDebugLogPath, nil
	}
	if filepath.IsAbs(raw) {
		return "", fmt.Errorf("不允许访问系统路径: %s", raw)
	}
	base := pluginController.privateDir
	if base == "" {
		return "", fmt.Errorf("插件存储未初始化")
	}
	// 相对路径相对 base 解析：先规范，再校验不越界、且落在插件子目录。
	rel := filepath.Clean(raw)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法路径")
	}
	allowed := false
	for _, sub := range aiPluginSubdirs {
		if rel == sub || strings.HasPrefix(rel, sub+string(filepath.Separator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("只允许读写插件目录（插件文件/插件数据文件/插件配置文件）")
	}
	return filepath.Join(base, rel), nil
}

// isPluginFilePath 判断绝对路径是否落在插件相关目录内（存档/恢复快照时校验）。
func isPluginFilePath(abs string) bool {
	base := pluginController.privateDir
	if base == "" {
		return false
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	for _, sub := range aiPluginSubdirs {
		if rel == sub || strings.HasPrefix(rel, sub+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// parseLinesArg 解析行范围参数："all" 或 "10" 或 "10-20"。返回 0 起下标闭区间。
func parseLinesArg(s string) (start, end int, all bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "all" {
		return 0, 0, true, nil
	}
	if i, e := strconv.Atoi(s); e == nil {
		return i - 1, i - 1, false, nil
	}
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		a, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		b, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if e1 == nil && e2 == nil && a >= 1 && b >= a {
			return a - 1, b - 1, false, nil
		}
	}
	return 0, 0, false, fmt.Errorf("无法解析行范围: %q（用 all 或 10 或 10-20）", s)
}

// ── 工具实现 ───────────────────────────────────────────────────────────

// toolReadFile 读插件文件（可带行范围）。返回带行号内容。
func toolReadFile(args map[string]any) (string, bool) {
	path, err := resolveAIFilePath(aiArgStr(args, "path"))
	if err != nil {
		return err.Error(), true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "读取失败: " + err.Error(), true
	}
	lines := strings.Split(string(data), "\n")
	start, end, all, err := parseLinesArg(aiArgStr(args, "lines"))
	if err != nil {
		return err.Error(), true
	}
	if !all {
		if start < 0 {
			start = 0
		}
		if end >= len(lines) {
			end = len(lines) - 1
		}
		if start > end || start >= len(lines) {
			return "行范围超出文件（共 " + strconv.Itoa(len(lines)) + " 行）", false
		}
		lines = lines[start : end+1]
	}
	var sb strings.Builder
	for i, ln := range lines {
		sb.WriteString(fmt.Sprintf("%4d| %s\n", start+i+1, ln))
	}
	return strings.TrimRight(sb.String(), "\n"), false
}

// toolWriteFile 写插件文件（整体覆盖）。
func toolWriteFile(args map[string]any) (string, bool) {
	path, err := resolveAIFilePath(aiArgStr(args, "path"))
	if err != nil {
		return err.Error(), true
	}
	content := aiArgStr(args, "content")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "写入失败: " + err.Error(), true
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "写入失败: " + err.Error(), true
	}
	aiNoteModified(path)
	return fmt.Sprintf("已写入 %s（%d 字节）", path, len(content)), false
}

// toolEditLines 按行范围替换（sed 式）：把 [start,end] 行换成 new_content。
func toolEditLines(args map[string]any) (string, bool) {
	path, err := resolveAIFilePath(aiArgStr(args, "path"))
	if err != nil {
		return err.Error(), true
	}
	start, end, all, err := parseLinesArg(aiArgStr(args, "lines"))
	if err != nil {
		return err.Error(), true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "读取失败: " + err.Error(), true
	}
	trailingNL := strings.HasSuffix(string(data), "\n")
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if !all {
		if start < 0 || start >= len(lines) || end >= len(lines) || start > end {
			return fmt.Sprintf("行范围超出文件（共 %d 行）", len(lines)), true
		}
	} else {
		start, end = 0, len(lines)-1
	}
	newContent := strings.TrimRight(aiArgStr(args, "new_content"), "\n")
	var newLines []string
	if newContent != "" {
		newLines = strings.Split(newContent, "\n")
	}
	out := make([]string, 0, len(lines)+len(newLines))
	out = append(out, lines[:start]...)
	out = append(out, newLines...)
	out = append(out, lines[end+1:]...)
	joined := strings.Join(out, "\n")
	if trailingNL {
		joined += "\n"
	}
	if err := os.WriteFile(path, []byte(joined), 0644); err != nil {
		return "写入失败: " + err.Error(), true
	}
	aiNoteModified(path)
	return fmt.Sprintf("已替换 %s 的第 %d-%d 行为 %d 行（现共 %d 行）", path, start+1, end+1, len(newLines), len(out)), false
}

// toolListDir 列插件目录。
func toolListDir(args map[string]any) (string, bool) {
	path, err := resolveAIFilePath(aiArgStr(args, "path"))
	if err != nil {
		return err.Error(), true
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "目录不存在: " + path, false
		}
		return "读取失败: " + err.Error(), true
	}
	var sb strings.Builder
	for _, e := range entries {
		kind := "f"
		if e.IsDir() {
			kind = "d"
		}
		info, _ := e.Info()
		size := ""
		if info != nil && !e.IsDir() {
			size = " " + strconv.FormatInt(info.Size(), 10) + "B"
		}
		sb.WriteString(fmt.Sprintf("%s  %s%s\n", kind, e.Name(), size))
	}
	return strings.TrimRight(sb.String(), "\n"), false
}

// toolReadLog 读插件调试日志尾部。
func toolReadLog(args map[string]any) (string, bool) {
	data, err := os.ReadFile(aiDebugLogPath)
	if err != nil {
		return "读取日志失败: " + err.Error(), true
	}
	lines := strings.Split(string(data), "\n")
	n := aiArgInt(args, "lines", 100)
	if n <= 0 {
		n = 100
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var sb strings.Builder
	for _, ln := range lines {
		sb.WriteString(ln + "\n")
	}
	return strings.TrimRight(sb.String(), "\n"), false
}

// aiFileToolDefs 补充文件类工具的 JSON Schema 定义。
func aiFileToolDefs() []ToolDef {
	return []ToolDef{
		{
			Name:        "read_file",
			Description: "读取插件文件（源码/数据/配置），可指定行范围。path 用相对路径，如 插件文件/领地/领地.lua；lines 填 all 或 10 或 10-20。返回带行号内容。只允许插件目录内的文件。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":  map[string]any{"type": "string", "description": "相对路径，如 插件文件/xxx/xxx.lua"},
					"lines": map[string]any{"type": "string", "description": "行范围 all/10/10-20，默认 all"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "把内容整体写入插件文件（覆盖）。path 相对路径。只允许插件目录内的文件。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "相对路径"},
					"content": map[string]any{"type": "string", "description": "要写入的完整内容"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "edit_lines",
			Description: "按行范围替换插件文件内容（sed 式）。lines 填 10-20 或 all；把 [start,end] 行整体替换为 new_content。只允许插件目录内的文件。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":        map[string]any{"type": "string", "description": "相对路径"},
					"lines":       map[string]any{"type": "string", "description": "要替换的行范围，如 10-20"},
					"new_content": map[string]any{"type": "string", "description": "替换后的内容"},
				},
				"required": []string{"path", "lines"},
			},
		},
		{
			Name:        "list_dir",
			Description: "列出插件目录下的文件。path 相对路径，如 插件文件/xxx 或 插件数据文件/xxx。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "相对目录路径"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "read_log",
			Description: "读取插件运行调试日志（prism_debug.log）尾部。lines 指定读取行数，默认 100。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"lines": map[string]any{"type": "integer", "description": "读取尾部行数，默认 100"},
				},
			},
		},
	}
}
