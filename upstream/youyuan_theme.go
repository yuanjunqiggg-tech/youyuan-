package main

// ═══ MCORT 插件化主题框架 · 骨架 ═══
//
// 目标（用户 2026-10-26 原话）：
//   「像 DeepSeek harness 一样可以通过插件来改背景和挂件」
//   「支持他们自己优化并做 UI 设计，只要不把 MCORT 标识去掉、不把疣螈隐藏」
//   「外接几个新玩法，跟我们接入种子查询器差不多」
//   「群服互通的样式太丑了，可以自己改，只要搭配点插件」
//   「硬编码层是需要它搭配启动运行相关插件才可以显示这些」
//
// 这一版只交付**结构骨架**，刻意不做插件任意 JS 执行：
//   1. 主题 = <dataDir>/themes/<id>/{theme.json,theme.css,theme.js,assets/}
//   2. 品牌守卫是硬编码层，主题只能改配色/布局，删不掉 "MCORT · 作者 疣螈"
//   3. 玩法插件 = <dataDir>/plugins/<id>/{plugin.json,main.js}，只发现+记录启用，不执行
//   4. 公告 = <dataDir>/announcements.json，GET 只读、POST 需管理员
//
// 术语：玩家=游戏内角色；用户/群友=QQ 侧的人；账号=登录工具箱的账号。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	yyThemeAPIVersion = 1
	// 与 mcort_brand.go 的 yyBrandSignature() 同口径，前端用字面量再写一份。
	yyThemeBrandLine = "MCORT · 作者 疣螈"
	yyThemeAssetMax  = 4 << 20
	yyThemeItemMax   = 200
)

// yyThemePluginWhitelist 是未来「真正执行插件」的唯一入口：
// 只有显式列在这里的插件 id 才允许执行 main.js。当前为空——所有插件都只记录启用状态。
var yyThemePluginWhitelist = map[string]bool{}

// yyThemeKnownRequires 是允许主题声明的依赖模块。
// 硬编码层（brand-guard）不依赖任何主题，永远在；主题槽位才依赖这些模块。
var yyThemeKnownRequires = map[string]bool{
	"theme-core":    true,
	"brand-guard":   true,
	"announcements": true,
	"plugins":       true,
	"qq-console":    true,
}

var yyThemeIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// ═══ 清单与装载结果 ═══

type yyThemeManifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	API         int               `json:"mcort_theme_api"`
	Requires    []string          `json:"requires"`
	RemoveBrand bool              `json:"remove_brand"`
	HideAuthor  bool              `json:"hide_author"`
	Variables   map[string]string `json:"variables"`
	Slots       map[string]string `json:"slots"`
}

type yyThemeInfo struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Author    string            `json:"author"`
	API       int               `json:"mcort_theme_api"`
	Requires  []string          `json:"requires"`
	Variables map[string]string `json:"variables"`
	Slots     map[string]string `json:"slots"`
	OK        bool              `json:"ok"`
	Error     string            `json:"error,omitempty"`
	HasCSS    bool              `json:"has_css"`
	HasJS     bool              `json:"has_js"`
	Assets    bool              `json:"has_assets"`
	Dir       string            `json:"-"`
}

type yyPluginManifest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Author       string   `json:"author"`
	Description  string   `json:"description"`
	Entry        string   `json:"entry"`
	API          int      `json:"mcort_plugin_api"`
	Capabilities []string `json:"capabilities"`
}

type yyPluginInfo struct {
	yyPluginManifest
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	Enabled     bool   `json:"enabled"`
	Executed    bool   `json:"executed"`
	Whitelisted bool   `json:"whitelisted"`
	Dir         string `json:"-"`
}

// ═══ 注册表（加载 / 校验 / 装配） ═══

type yyThemeRegistryState struct {
	mu      sync.RWMutex
	dataDir string
	themes  []yyThemeInfo
	plugins []yyPluginInfo
	active  string
	enabled map[string]bool
}

var yyThemes = &yyThemeRegistryState{enabled: map[string]bool{}}

// yyThemeLoad 扫描 themes/ 与 plugins/，并读取当前启用主题。
// 必须是 pure-ish：不启动 goroutine、不监听端口，方便测试直接调用。
func yyThemeLoad(dataDir string) []yyThemeInfo {
	themes := yyThemeScan(dataDir)
	enabled := yyThemeReadEnabled(dataDir)
	plugins := yyThemeScanPlugins(dataDir, enabled)
	active := yyThemeReadActive(dataDir)
	if active != "" && !yyThemeUsable(themes, active) {
		active = ""
	}
	yyThemes.mu.Lock()
	yyThemes.dataDir = dataDir
	yyThemes.themes = themes
	yyThemes.plugins = plugins
	yyThemes.active = active
	yyThemes.enabled = enabled
	yyThemes.mu.Unlock()
	return themes
}

func yyThemeUsable(themes []yyThemeInfo, id string) bool {
	for _, t := range themes {
		if t.ID == id {
			return t.OK
		}
	}
	return false
}

func yyThemeReadActive(dataDir string) string {
	raw, err := os.ReadFile(filepath.Join(dataDir, "theme-active.json"))
	if err != nil {
		return ""
	}
	var p struct {
		Active string `json:"active"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return ""
	}
	return p.Active
}

func yyThemeReadEnabled(dataDir string) map[string]bool {
	out := map[string]bool{}
	raw, err := os.ReadFile(filepath.Join(dataDir, "plugins-enabled.json"))
	if err != nil {
		return out
	}
	var p struct {
		Enabled map[string]bool `json:"enabled"`
	}
	if json.Unmarshal(raw, &p) == nil && p.Enabled != nil {
		out = p.Enabled
	}
	return out
}

func yyThemeSaveEnabled(dataDir string, enabled map[string]bool) error {
	raw, err := json.MarshalIndent(map[string]any{"version": 1, "enabled": enabled}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dataDir, "plugins-enabled.json")
	if err = os.WriteFile(path+".tmp", raw, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func yyThemeSaveActive(dataDir, id string) error {
	raw, err := json.MarshalIndent(map[string]any{"active": id}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dataDir, "theme-active.json")
	if err = os.WriteFile(path+".tmp", raw, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// yyThemeScan 只负责发现 + 校验，不写任何文件。
func yyThemeScan(dataDir string) []yyThemeInfo {
	root := filepath.Join(dataDir, "themes")
	entries, err := os.ReadDir(root)
	if err != nil {
		return []yyThemeInfo{}
	}
	out := []yyThemeInfo{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		info := yyThemeLoadOne(dir, e.Name())
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func yyThemeLoadOne(dir, dirName string) yyThemeInfo {
	info := yyThemeInfo{ID: dirName, Dir: dir}
	raw, err := os.ReadFile(filepath.Join(dir, "theme.json"))
	if err != nil {
		info.Error = "缺少 theme.json"
		return info
	}
	var m yyThemeManifest
	if err = json.Unmarshal(raw, &m); err != nil {
		info.Error = "theme.json 不是合法 JSON：" + err.Error()
		return info
	}
	css, err := os.ReadFile(filepath.Join(dir, "theme.css"))
	if err != nil {
		info.Error = "缺少 theme.css"
		return info
	}
	js := []byte{}
	hasJS := false
	if data, e := os.ReadFile(filepath.Join(dir, "theme.js")); e == nil {
		js, hasJS = data, true
	}
	_, assetsErr := os.Stat(filepath.Join(dir, "assets"))

	info = yyThemeInfo{
		ID: m.ID, Name: m.Name, Version: m.Version, Author: m.Author, API: m.API,
		Requires: m.Requires, Variables: m.Variables, Slots: m.Slots,
		HasCSS: true, HasJS: hasJS, Assets: assetsErr == nil, Dir: dir,
	}
	if e := yyThemeValidate(dirName, m, string(css), string(js)); e != nil {
		info.Error = e.Error()
		return info
	}
	info.OK = true
	return info
}

// ═══ 校验：品牌硬编码守卫 ═══

// yyThemeValidate 是唯一的准入闸门。任何一条不过，主题直接判为不可用。
func yyThemeValidate(dirName string, m yyThemeManifest, css, js string) error {
	if !yyThemeIDPattern.MatchString(m.ID) {
		return fmt.Errorf("id 只能是小写字母/数字/.-_（当前 %q）", m.ID)
	}
	if m.ID != dirName {
		return fmt.Errorf("id 必须与目录名一致（目录 %q，清单 %q）", dirName, m.ID)
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("name / version 不能为空")
	}
	if m.API != yyThemeAPIVersion {
		return fmt.Errorf("mcort_theme_api 只支持 %d（当前 %d）", yyThemeAPIVersion, m.API)
	}
	if m.RemoveBrand {
		return fmt.Errorf("拒绝加载：manifest 声明了 remove_brand=true，MCORT 标识属于硬编码层，主题无权移除")
	}
	if m.HideAuthor {
		return fmt.Errorf("拒绝加载：manifest 声明了 hide_author=true，作者署名属于硬编码层，主题无权隐藏")
	}
	for _, r := range m.Requires {
		if !yyThemeKnownRequires[r] {
			return fmt.Errorf("不认识的依赖 %q；可用：theme-core / brand-guard / announcements / plugins / qq-console", r)
		}
	}
	for k, v := range m.Variables {
		if !yyThemeVariableName(k) {
			return fmt.Errorf("变量 %q 不在可覆盖范围内（必须以 --mcort-theme- 开头）", k)
		}
		if !yyThemeVariableValue(v) {
			return fmt.Errorf("变量 %q 的值包含不允许的字符", k)
		}
	}
	for slot, v := range m.Slots {
		if slot != "header" && slot != "sidebar_footer" && slot != "floating" {
			return fmt.Errorf("槽位 %q 不支持；只有 header / sidebar_footer / floating", slot)
		}
		if strings.ContainsAny(v, "<>") || len([]rune(v)) > 160 {
			return fmt.Errorf("槽位 %q 只能是纯文本或 assets/ 图片路径（≤160 字，不允许 HTML）", slot)
		}
	}
	if e := yyThemeGuardText(css); e != nil {
		return e
	}
	if e := yyThemeGuardText(js); e != nil {
		return e
	}
	return nil
}

// yyThemeVariableName / Value 限制主题能碰到的变量面。品牌相关变量永远不在白名单。
func yyThemeVariableName(name string) bool {
	if !strings.HasPrefix(name, "--mcort-theme-") {
		return false
	}
	lower := strings.ToLower(name)
	return !strings.Contains(lower, "brand") && !strings.Contains(lower, "author")
}

func yyThemeVariableValue(v string) bool {
	if len(v) > 200 {
		return false
	}
	lower := strings.ToLower(v)
	for _, bad := range []string{"<", ">", "{", "}", ";", "expression(", "javascript:", "\\", "@import", "url("} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return true
}

var yyThemeBrandMarkers = []string{
	"yy-brand-mark",
	"mcort-brand",
	"mcort-brand-signature",
	"data-mcort-brand",
}

var yyThemeBrandVarPattern = regexp.MustCompile(`--mcort-brand[a-z0-9_-]*:`)

// yyThemeGuardText 对 CSS/JS 文本做去注释 + 去空白归一化后判定。
// 去注释是故意的：挡住 `.yy-brand-mark{/*x*/display:none}` 这类绕过。
func yyThemeGuardText(text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	norm := yyThemeNormalize(text)
	if norm == "" {
		return nil
	}
	// 1) 直接改写品牌变量
	if yyThemeBrandVarPattern.MatchString(norm) {
		return fmt.Errorf("拒绝加载：主题试图重定义品牌变量 --mcort-brand*，该变量属于硬编码层")
	}
	// 2) 隐藏/缩小品牌标识的选择器
	for _, marker := range yyThemeBrandMarkers {
		for idx := 0; ; {
			pos := strings.Index(norm[idx:], marker)
			if pos < 0 {
				break
			}
			start := idx + pos
			window := norm[start:]
			if len(window) > 320 {
				window = window[:320]
			}
			if yyThemeWindowHidesBrand(window) {
				return fmt.Errorf("拒绝加载：主题试图隐藏 %s（MCORT 标识与作者署名不可隐藏）", marker)
			}
			// JS 里的编程式移除
			if strings.Contains(window, ".remove(") || strings.Contains(window, "removechild") ||
				strings.Contains(window, "style.display") || strings.Contains(window, "visibility") ||
				strings.Contains(window, "innerhtml=") {
				return fmt.Errorf("拒绝加载：theme.js 试图操作品牌节点 %s，该节点属于硬编码层", marker)
			}
			idx = start + len(marker)
		}
	}
	return nil
}

func yyThemeWindowHidesBrand(window string) bool {
	open := strings.Index(window, "{")
	if open < 0 {
		return false
	}
	end := strings.Index(window[open:], "}")
	if end < 0 {
		end = len(window) - open
	}
	block := window[open : open+end]
	for _, bad := range []string{
		"display:none", "visibility:hidden", "opacity:0", "font-size:0",
		"height:0", "width:0", "clip:rect(0", "left:-9999", "content-visibility:hidden",
	} {
		if strings.Contains(block, bad) {
			return true
		}
	}
	return false
}

// yyThemeNormalize 去注释、转小写、去掉所有空白与引号，便于稳定匹配。
func yyThemeNormalize(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); i++ {
		if i+1 < len(text) && text[i] == '/' && text[i+1] == '*' {
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				break
			}
			i += end + 3
			continue
		}
		c := text[i]
		switch c {
		case ' ', '\t', '\r', '\n', '\f', '\v', '"', '\'':
			continue
		}
		b.WriteByte(c)
	}
	return strings.ToLower(b.String())
}

// ═══ CSS 资源地址重写 ═══

// RE2 不支持反向引用，所以把单引号/双引号/无引号写成三个分支再取非空那个。
var yyThemeCSSURLPattern = regexp.MustCompile(`url\(\s*(?:'([^']*)'|"([^"]*)"|([^)'"]*))\s*\)`)

// yyThemeRewriteCSS 把主题包内的相对 url(...) 改写成受控的 asset 接口，
// 否则浏览器会按页面路径解析，assets/ 永远 404。
func yyThemeRewriteCSS(css, themeID string) string {
	return yyThemeCSSURLPattern.ReplaceAllStringFunc(css, func(m string) string {
		sub := yyThemeCSSURLPattern.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		target := sub[1]
		if target == "" {
			target = sub[2]
		}
		if target == "" {
			target = sub[3]
		}
		target = strings.TrimSpace(target)
		low := strings.ToLower(target)
		if target == "" || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "#") ||
			strings.HasPrefix(low, "data:") || strings.HasPrefix(low, "http:") || strings.HasPrefix(low, "https:") {
			return m
		}
		target = strings.TrimPrefix(target, "./")
		return `url("/api/mcort/theme/asset?theme=` + url.QueryEscape(themeID) + `&file=` + url.QueryEscape(target) + `")`
	})
}

// yyThemeAssetFile 只允许 theme.css / theme.js / assets/ 下的文件。
func yyThemeAssetFile(themeDir, file string) (string, bool) {
	if file == "" || strings.Contains(file, "..") || strings.Contains(file, "\\") || strings.HasPrefix(file, "/") {
		return "", false
	}
	clean := filepath.Clean(file)
	if clean != "theme.css" && clean != "theme.js" && !strings.HasPrefix(clean, "assets"+string(filepath.Separator)) {
		return "", false
	}
	full := filepath.Join(themeDir, clean)
	rel, err := filepath.Rel(themeDir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

func yyThemeAssetType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".woff2":
		return "font/woff2"
	}
	return "application/octet-stream"
}

// ═══ 玩法插件：只发现，不执行 ═══

func yyThemeScanPlugins(dataDir string, enabled map[string]bool) []yyPluginInfo {
	root := filepath.Join(dataDir, "plugins")
	entries, err := os.ReadDir(root)
	if err != nil {
		return []yyPluginInfo{}
	}
	out := []yyPluginInfo{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		info := yyPluginInfo{Dir: dir}
		raw, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
		if err != nil {
			info.ID = e.Name()
			info.Error = "缺少 plugin.json"
			out = append(out, info)
			continue
		}
		var m yyPluginManifest
		if err = json.Unmarshal(raw, &m); err != nil {
			info.ID = e.Name()
			info.Error = "plugin.json 不是合法 JSON"
			out = append(out, info)
			continue
		}
		info = yyPluginInfo{yyPluginManifest: m, Dir: dir}
		if m.Entry == "" {
			m.Entry = "main.js"
			info.Entry = m.Entry
		}
		if !yyThemeIDPattern.MatchString(m.ID) || m.ID != e.Name() {
			info.Error = "id 必须是目录名，且只用小写字母/数字/.-_"
			out = append(out, info)
			continue
		}
		if m.API != yyThemeAPIVersion {
			info.Error = fmt.Sprintf("mcort_plugin_api 只支持 %d", yyThemeAPIVersion)
			out = append(out, info)
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, m.Entry)); err != nil {
			info.Error = "入口文件不存在：" + m.Entry
			out = append(out, info)
			continue
		}
		info.OK = true
		info.Enabled = enabled[m.ID]
		info.Whitelisted = yyThemePluginWhitelist[m.ID]
		info.Executed = false // 执行层未开放
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ═══ 公告存储 ═══

type yyAnnouncement struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Severity  string `json:"severity"`
	Pinned    bool   `json:"pinned"`
	Author    string `json:"author"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type yyAnnouncementStore struct {
	mu    sync.Mutex
	path  string
	items []yyAnnouncement
}

func yyNewAnnouncementStore(dataDir string) *yyAnnouncementStore {
	s := &yyAnnouncementStore{path: filepath.Join(dataDir, "announcements.json")}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	var p struct {
		Items []yyAnnouncement `json:"items"`
	}
	if json.Unmarshal(raw, &p) == nil {
		s.items = p.Items
	}
	return s
}

func (s *yyAnnouncementStore) saveLocked() error {
	raw, err := json.MarshalIndent(map[string]any{"version": 1, "items": s.items}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(s.path+".tmp", raw, 0600); err != nil {
		return err
	}
	return os.Rename(s.path+".tmp", s.path)
}

func (s *yyAnnouncementStore) List() []yyAnnouncement {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]yyAnnouncement{}, s.items...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

func yyAnnouncementClean(v string, limit int) (string, error) {
	v = strings.TrimSpace(v)
	if len([]rune(v)) > limit {
		return "", fmt.Errorf("超过 %d 字上限", limit)
	}
	return v, nil
}

func (s *yyAnnouncementStore) Upsert(a yyAnnouncement, op string) (yyAnnouncement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	switch op {
	case "", "create":
		if len(s.items) >= yyThemeItemMax {
			return a, fmt.Errorf("公告数量已达上限 %d 条，请先删除旧公告", yyThemeItemMax)
		}
		a.ID = fmt.Sprintf("ann-%d", now)
		a.CreatedAt, a.UpdatedAt = now, now
		if a.Severity == "" {
			a.Severity = "info"
		}
		s.items = append(s.items, a)
		if err := s.saveLocked(); err != nil {
			return a, err
		}
		return a, nil
	case "update":
		for i := range s.items {
			if s.items[i].ID != a.ID {
				continue
			}
			if a.Title != "" {
				s.items[i].Title = a.Title
			}
			if a.Content != "" {
				s.items[i].Content = a.Content
			}
			if a.Severity != "" {
				s.items[i].Severity = a.Severity
			}
			s.items[i].Pinned = a.Pinned
			s.items[i].UpdatedAt = now
			if err := s.saveLocked(); err != nil {
				return a, err
			}
			return s.items[i], nil
		}
		return a, fmt.Errorf("公告不存在")
	case "delete":
		next := make([]yyAnnouncement, 0, len(s.items))
		found := false
		for _, it := range s.items {
			if it.ID == a.ID {
				found = true
				continue
			}
			next = append(next, it)
		}
		if !found {
			return a, fmt.Errorf("公告不存在")
		}
		s.items = next
		return a, s.saveLocked()
	}
	return a, fmt.Errorf("op 只支持 create / update / delete")
}

// ═══ 管理员判定 ═══

// yyThemeIsAdmin：绑定平台账号时看 role/legacy_owner；没绑定时只信任本机回环。
func yyThemeIsAdmin(r *http.Request) bool {
	if actor, err := yyPoolActorRequest(r); err == nil {
		return actor.Admin
	}
	return yyThemeLoopback(r)
}

func yyThemeLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ═══ HTTP 路由 ═══

// yyThemeRoutes 同时完成装载与接线：yyRegisterAPI 里调用一次即可。
func yyThemeRoutes(mux *http.ServeMux, dataDir string) {
	yyThemeLoad(dataDir)
	ann := yyNewAnnouncementStore(dataDir)

	mux.HandleFunc("/api/mcort/themes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		themes, active := yyThemeSnapshot()
		writeJSON(w, map[string]any{
			"ok": true, "api": yyThemeAPIVersion, "active": active,
			"themes": themes, "brand": yyThemeBrandLine,
			"note": "theme.js 只做打包与审阅，不执行；执行层需要显式白名单 + 二次确认",
		})
	})

	mux.HandleFunc("/api/mcort/theme", func(w http.ResponseWriter, r *http.Request) {
		themes, active := yyThemeSnapshot()
		if r.Method == "GET" {
			for _, t := range themes {
				if t.ID == active {
					writeJSON(w, map[string]any{"ok": true, "active": active, "theme": t,
						"css_url": "/api/mcort/theme/asset?theme=" + url.QueryEscape(t.ID) + "&file=theme.css"})
					return
				}
			}
			writeJSON(w, map[string]any{"ok": true, "active": "", "theme": nil})
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !yyThemeIsAdmin(r) {
			w.WriteHeader(http.StatusForbidden)
			writeJSON(w, map[string]any{"ok": false, "error": "只有管理员可以切换主题"})
			return
		}
		var p struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "JSON 无效"})
			return
		}
		p.ID = strings.TrimSpace(p.ID)
		if p.ID != "" && !yyThemeUsable(themes, p.ID) {
			writeJSON(w, map[string]any{"ok": false, "error": "主题不存在或未通过校验"})
			return
		}
		if err := yyThemeSaveActive(dataDir, p.ID); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		yyThemes.mu.Lock()
		yyThemes.active = p.ID
		yyThemes.mu.Unlock()
		writeJSON(w, map[string]any{"ok": true, "active": p.ID})
	})

	mux.HandleFunc("/api/mcort/theme/asset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("theme")
		file := r.URL.Query().Get("file")
		if !yyThemeIDPattern.MatchString(id) {
			http.NotFound(w, r)
			return
		}
		themes, _ := yyThemeSnapshot()
		var dir string
		for _, t := range themes {
			if t.ID == id && t.OK {
				dir = t.Dir
				break
			}
		}
		if dir == "" {
			http.NotFound(w, r)
			return
		}
		full, ok := yyThemeAssetFile(dir, file)
		if !ok {
			http.NotFound(w, r)
			return
		}
		st, err := os.Stat(full)
		if err != nil || st.IsDir() || st.Size() > yyThemeAssetMax {
			http.NotFound(w, r)
			return
		}
		data, err := os.ReadFile(full)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(strings.ToLower(file), ".css") {
			data = []byte(yyThemeRewriteCSS(string(data), id))
		}
		w.Header().Set("Content-Type", yyThemeAssetType(file))
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(data)
	})

	mux.HandleFunc("/api/mcort/announcements", yyThemeAnnouncements(ann))

	mux.HandleFunc("/api/mcort/plugins", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			plugins := yyThemePluginsSnapshot()
			writeJSON(w, map[string]any{
				"ok": true, "plugins": plugins, "api": yyThemeAPIVersion,
				"note": "当前只发现与记录启用状态，不执行任何插件 JS",
			})
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !yyThemeIsAdmin(r) {
			w.WriteHeader(http.StatusForbidden)
			writeJSON(w, map[string]any{"ok": false, "error": "只有管理员可以启用插件"})
			return
		}
		var p struct {
			Op      string `json:"op"`
			ID      string `json:"id"`
			Confirm bool   `json:"confirm"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "JSON 无效"})
			return
		}
		if !p.Confirm {
			writeJSON(w, map[string]any{"ok": false, "error": "启用插件需要二次确认（confirm:true）"})
			return
		}
		if p.Op == "" {
			p.Op = "enable"
		}
		if p.Op != "enable" && p.Op != "disable" {
			writeJSON(w, map[string]any{"ok": false, "error": "op 只支持 enable / disable"})
			return
		}
		plugins := yyThemePluginsSnapshot()
		found := false
		for _, pl := range plugins {
			if pl.ID == p.ID {
				found = true
				if p.Op == "enable" && !pl.OK {
					writeJSON(w, map[string]any{"ok": false, "error": "插件未通过校验，不能启用"})
					return
				}
				break
			}
		}
		if !found {
			writeJSON(w, map[string]any{"ok": false, "error": "插件不存在"})
			return
		}
		yyThemes.mu.Lock()
		if yyThemes.enabled == nil {
			yyThemes.enabled = map[string]bool{}
		}
		if p.Op == "enable" {
			yyThemes.enabled[p.ID] = true
		} else {
			delete(yyThemes.enabled, p.ID)
		}
		enabled := map[string]bool{}
		for k, v := range yyThemes.enabled {
			enabled[k] = v
		}
		yyThemes.mu.Unlock()
		if err := yyThemeSaveEnabled(dataDir, enabled); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		yyThemes.mu.Lock()
		yyThemes.plugins = yyThemeScanPlugins(dataDir, enabled)
		yyThemes.mu.Unlock()
		writeJSON(w, map[string]any{
			"ok": true, "id": p.ID, "enabled": p.Op == "enable", "executed": false,
			"note": "已记录启用状态。执行层未开放：插件 main.js 不会被加载或运行",
		})
	})
}

func yyThemeSnapshot() ([]yyThemeInfo, string) {
	yyThemes.mu.RLock()
	defer yyThemes.mu.RUnlock()
	return append([]yyThemeInfo{}, yyThemes.themes...), yyThemes.active
}

func yyThemePluginsSnapshot() []yyPluginInfo {
	yyThemes.mu.RLock()
	defer yyThemes.mu.RUnlock()
	return append([]yyPluginInfo{}, yyThemes.plugins...)
}

func yyThemeAnnouncements(store *yyAnnouncementStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			writeJSON(w, map[string]any{
				"ok": true, "announcements": store.List(),
				"admin": yyThemeIsAdmin(r), "brand": yyThemeBrandLine,
			})
		case "POST":
			if !yyThemeIsAdmin(r) {
				w.WriteHeader(http.StatusForbidden)
				writeJSON(w, map[string]any{"ok": false, "error": "只有管理员可以发布公告"})
				return
			}
			var p struct {
				Op       string `json:"op"`
				ID       string `json:"id"`
				Title    string `json:"title"`
				Content  string `json:"content"`
				Severity string `json:"severity"`
				Pinned   bool   `json:"pinned"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&p); err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "JSON 无效"})
				return
			}
			op := strings.TrimSpace(p.Op)
			if op == "" {
				op = "create"
			}
			if op == "delete" {
				if strings.TrimSpace(p.ID) == "" {
					writeJSON(w, map[string]any{"ok": false, "error": "缺少公告 id"})
					return
				}
				if _, err := store.Upsert(yyAnnouncement{ID: p.ID}, "delete"); err != nil {
					writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
					return
				}
				writeJSON(w, map[string]any{"ok": true, "deleted": p.ID})
				return
			}
			title, err := yyAnnouncementClean(p.Title, 80)
			if err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "标题" + err.Error()})
				return
			}
			content, err := yyAnnouncementClean(p.Content, 8000)
			if err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "正文" + err.Error()})
				return
			}
			if title == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "标题不能为空"})
				return
			}
			sev := strings.TrimSpace(p.Severity)
			if sev != "" && sev != "info" && sev != "warning" && sev != "critical" {
				writeJSON(w, map[string]any{"ok": false, "error": "severity 只支持 info / warning / critical"})
				return
			}
			if op == "update" && strings.TrimSpace(p.ID) == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "更新公告需要 id"})
				return
			}
			out, err := store.Upsert(yyAnnouncement{
				ID: p.ID, Title: title, Content: content, Severity: sev,
				Pinned: p.Pinned, Author: "疣螈",
			}, op)
			if err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "announcement": out, "brand": yyThemeBrandLine})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

// ═══ 首页注入：不改 index.html，也能把主题层接上 ═══

const yyThemeIndexCSS = `<link rel="stylesheet" href="/css/mcort_theme.css?v=2026102601">`
const yyThemeIndexJS = `<script src="/js/mcort_theme.js?v=2026102601"></script>`

// yyThemeInjectIndex 在 </head> 前插样式、</body> 前插脚本。
// 脚本放在最后，保证品牌署名节点的注入晚于业务脚本，主题样式压不住它。
func yyThemeInjectIndex(data []byte) []byte {
	if bytes.Contains(data, []byte("mcort_theme.js")) {
		return data
	}
	out := bytes.Replace(data, []byte("</head>"), []byte(yyThemeIndexCSS+"</head>"), 1)
	out = bytes.Replace(out, []byte("</body>"), []byte(yyThemeIndexJS+"</body>"), 1)
	return out
}
