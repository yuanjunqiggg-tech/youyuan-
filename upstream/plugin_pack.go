package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// zipSlipSafe 校验 zip 内路径不含 .. 段，防止路径穿越写出目录。
func zipSlipSafe(name string) bool {
	name = strings.ReplaceAll(name, `\`, "/")
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsAny(name, ":\x00") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

// 插件包导入的资源上限（var 便于测试注入更小值）：防 zip 炸弹撑爆 Android 内存。
var (
	maxPluginZipEntries  = 200
	maxPluginZipFileSize = int64(10 << 20) // 单条目解压后 ≤10MB
	maxPluginZipTotal    = int64(20 << 20) // 全部条目合计 ≤20MB
)

// validPluginID 校验插件 id：非空、非 "." / ".."、不含路径分隔符、URL/cookie 破坏性
// 字符与控制字符。id 会拼进磁盘路径与 /web/p/<id>/... URL（cookie 名另经
// webCookieName hash 派生，见 web_http.go），拒绝空格与 `? # % = &` 保证三处都安全。
// 允许中文等 Unicode 字符（如"系统测试"）以及 `.`、`-`、`_`。
func validPluginID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	for _, r := range id {
		if r == '/' || r == '\\' || r == ' ' || r == '?' || r == '#' || r == '%' || r == '=' || r == '&' || r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// ImportPackage 从 zipPath 解包安装一个插件包。识别 manifest.json / main.lua /
// 词库.json / config.json / docs.md，按 id 写入插件存储目录，返回 id（调用方随后 ScanPlugins）。
func (pc *PluginController) ImportPackage(zipPath string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("打开插件包失败: %v", err)
	}
	defer zr.Close()
	if isFullPluginBackup(&zr.Reader) {
		return pc.importFullPluginBackup(&zr.Reader)
	}

	// 收集所有条目（拒绝 zip-slip 路径；限制条目数与解压总量，防 zip 炸弹）
	entries := map[string][]byte{}
	directories := []string{}
	var total int64
	for _, f := range zr.File {
		if !zipSlipSafe(f.Name) || f.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("插件包含非法路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			directories = append(directories, strings.ReplaceAll(f.Name, `\`, "/"))
			continue
		}
		if len(entries) >= maxPluginZipEntries {
			return "", fmt.Errorf("插件包条目过多（>%d）", maxPluginZipEntries)
		}
		size := f.FileInfo().Size()
		if size > maxPluginZipFileSize {
			return "", fmt.Errorf("条目 %s 过大（>%dMB）", f.Name, maxPluginZipFileSize>>20)
		}
		total += size
		if total > maxPluginZipTotal {
			return "", fmt.Errorf("插件包解压后总量过大（>%dMB）", maxPluginZipTotal>>20)
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("读取条目 %s 失败: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("读取条目 %s 失败: %v", f.Name, err)
		}
		name := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(f.Name, `\`, "/")))
		if _, exists := entries[name]; exists {
			return "", fmt.Errorf("插件包含重复路径: %s", name)
		}
		entries[name] = data
	}

	// 按键名排序保证兜底匹配确定性（map 迭代顺序随机）
	sortedKeys := make([]string, 0, len(entries))
	for p := range entries {
		sortedKeys = append(sortedKeys, p)
	}
	sort.Strings(sortedKeys)

	// 按文件名（basename）匹配所需条目
	byBase := func(base string) ([]byte, bool) {
		if d, ok := entries[base]; ok {
			return d, true
		}
		for _, p := range sortedKeys {
			if filepath.Base(p) == base {
				return entries[p], true
			}
		}
		return nil, false
	}

	// 识别 id：优先 manifest.id，否则取代码文件前缀
	id := ""
	manData, hasMan := byBase("manifest.json")
	if hasMan {
		var man struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(manData, &man) == nil && man.ID != "" {
			id = man.ID
		}
	}
	codeData, hasLua := byBase("main.lua")
	var codeName string
	if hasLua {
		codeName = id + ".lua"
	} else if wbData, ok := byBase("词库.json"); ok {
		codeData = wbData
		hasLua = true
		codeName = id + "词库.json"
	} else {
		// 尝试任意 .json/.lua 作为代码（按排序后的路径遍历，结果确定）
		for _, p := range sortedKeys {
			b := filepath.Base(p)
			if strings.HasSuffix(b, ".lua") {
				codeData = entries[p]
				hasLua = true
				if id == "" {
					id = strings.TrimSuffix(b, ".lua")
				}
				codeName = id + ".lua"
				break
			}
			if strings.HasSuffix(b, "词库.json") {
				codeData = entries[p]
				hasLua = true
				if id == "" {
					id = strings.TrimSuffix(b, "词库.json")
				}
				codeName = id + "词库.json"
				break
			}
		}
	}
	if !hasLua {
		return "", fmt.Errorf("插件包缺少代码文件 (main.lua 或词库.json)")
	}
	if id == "" {
		return "", fmt.Errorf("无法从插件包确定 id")
	}
	if !validPluginID(id) {
		return "", fmt.Errorf("插件 id 非法: %q（不允许路径分隔符、URL/cookie 破坏性字符或控制字符）", id)
	}

	// 写入 插件文件/<id>/ 目录（每插件独立子目录）
	plugDir := filepath.Join(pc.privateDir, "插件文件", id)
	if err := os.MkdirAll(plugDir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(plugDir, codeName), codeData, 0644); err != nil {
		return "", err
	}
	if hasMan {
		if err := os.WriteFile(filepath.Join(plugDir, id+".manifest.json"), manData, 0644); err != nil {
			return "", err
		}
	}
	if docsData, ok := byBase("docs.md"); ok {
		if err := os.WriteFile(filepath.Join(plugDir, id+".docs.md"), docsData, 0644); err != nil {
			return "", err
		}
	}
	// 配置 → 插件配置文件/<id>/<id>.json
	if cfgData, ok := byBase("config.json"); ok {
		if err := os.MkdirAll(filepath.Dir(pc.ConfigPath(id)), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(pc.ConfigPath(id), cfgData, 0644); err != nil {
			return "", err
		}
	}
	// Keep every module and resource. Older importers silently discarded all
	// entries other than the legacy entry, manifest, docs and config.
	prefix := ""
	for _, key := range sortedKeys {
		if filepath.Base(key) == "manifest.json" || filepath.Base(key) == "main.lua" {
			prefix = filepath.ToSlash(filepath.Dir(key))
			if prefix == "." {
				prefix = ""
			} else {
				prefix += "/"
			}
			break
		}
	}
	for _, key := range sortedKeys {
		name := key
		if prefix != "" && strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
		}
		if name == "config.json" {
			continue
		} // imported into the private config directory
		target, e := yyProjectPath(plugDir, name)
		if e != nil {
			return "", e
		}
		if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			return "", e
		}
		if e = os.WriteFile(target, entries[key], 0644); e != nil {
			return "", e
		}
	}
	for _, name := range directories {
		if prefix != "" && strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
		}
		if name == "" {
			continue
		}
		target, e := yyProjectPath(plugDir, name)
		if e != nil {
			return "", e
		}
		if e = os.MkdirAll(target, 0755); e != nil {
			return "", e
		}
	}
	debugLog("[插件] 导入完成 id=%s code=%s", id, codeName)
	return id, nil
}

// ExportMarketPackage 仅打包可发布的代码、manifest 与文档，避免把插件运行数据上传市场。
func (pc *PluginController) ExportMarketPackage(id string) (string, error) {
	if !validPluginID(id) {
		return "", fmt.Errorf("插件 id 非法: %q", id)
	}
	filesDir := filepath.Join(pc.privateDir, "插件文件", id)
	zipPath := filepath.Join(pc.privateDir, "插件文件", id+"-market.zip")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return "", err
	}
	zf, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	// 代码文件：优先 <id>.lua，其次 <id>词库.json
	codePath := filepath.Join(filesDir, id+".lua")
	if _, err := os.Stat(codePath); err != nil {
		codePath = filepath.Join(filesDir, id+"词库.json")
		if _, err := os.Stat(codePath); err != nil {
			return "", fmt.Errorf("找不到插件代码文件 %s", id)
		}
	}
	codeName := filepath.Base(codePath)
	if strings.HasSuffix(codeName, ".lua") {
		codeName = "main.lua"
	}
	if err := addZipFile(zw, codeName, codePath); err != nil {
		return "", err
	}
	if manPath := filepath.Join(filesDir, id+".manifest.json"); fileExists(manPath) {
		if err := addZipFile(zw, "manifest.json", manPath); err != nil {
			return "", err
		}
	}
	if docsPath := filepath.Join(filesDir, id+".docs.md"); fileExists(docsPath) {
		if err := addZipFile(zw, "docs.md", docsPath); err != nil {
			return "", err
		}
	}
	if err := filepath.WalkDir(filesDir, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if path == filesDir {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("发布包不允许符号链接: %s", path)
		}
		rel, e := filepath.Rel(filesDir, path)
		if e != nil {
			return e
		}
		if d.IsDir() {
			if rel == "data" || rel == "config" || rel == ".tmp" {
				return filepath.SkipDir
			}
			return nil
		}
		if path == codePath || rel == id+".manifest.json" || rel == id+".docs.md" || rel == "main.lua" || rel == "manifest.json" || rel == "docs.md" || rel == "config.json" {
			return nil
		}
		return addZipFile(zw, filepath.ToSlash(rel), path)
	}); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return zipPath, nil
}

// addZipFile 把磁盘文件按 zipName 写入 zip writer。
func addZipFile(zw *zip.Writer, zipName, srcPath string) error {
	fw, err := zw.Create(zipName)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	_, err = fw.Write(data)
	return err
}

// fileExists 判断文件是否存在。
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
