package main

// 自建插件市场：玩家浏览/安装，开发者上传/定价/看收益，管理员审核。
// 数据落盘 dataDir/market/，金额一律 int64 分，分成用整数运算。
// 不执行上传内容；安装时才交给现有 ImportPackage + ScanPlugins。

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	yyMarketFeePercent    = 30
	yyMarketMaxPriceCents = int64(100000000) // 100 万元，防溢出与误填
	yyMarketMaxName       = 60
	yyMarketMaxSummary    = 200
	yyMarketMaxDetail     = 20000
	yyMarketMaxReason     = 300
	yyMarketMaxChangelog  = 4000
	yyMarketMaxTags       = 8
	yyMarketMaxScreens    = 8
	yyMarketMaxMCVersions = 12
	yyMarketMaxLoaders    = 6
)

// 上传上限做成变量，测试可注入更小值；生产默认 20 MB。
var yyMarketMaxUploadBytes = int64(20 << 20)

var yyMarketAllowedExt = map[string]bool{".zip": true, ".lua": true, ".py": true, ".js": true}

var yyMarketCategorySet = map[string]bool{
	"tool": true, "gameplay": true, "automation": true, "ui": true, "library": true, "other": true,
}

type yyMarketVersion struct {
	Version    string   `json:"version"`
	Changelog  string   `json:"changelog"`
	MCVersions []string `json:"mc_versions"`
	Loader     string   `json:"loader"`
	FileName   string   `json:"file_name"`
	SizeBytes  int64    `json:"size_bytes"`
	SHA256     string   `json:"sha256"`
	CreatedAt  string   `json:"created_at"`
	File       string   `json:"file"` // 磁盘相对/绝对路径，只用于服务端安装，不进入对外视图
}

type yyMarketPlugin struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Summary      string            `json:"summary"`
	Description  string            `json:"description"`
	Category     string            `json:"category"`
	Tags         []string          `json:"tags"`
	AuthorID     string            `json:"author_id"`
	AuthorName   string            `json:"author_name"`
	PriceCents   int64             `json:"price_cents"`
	License      string            `json:"license"`
	Status       string            `json:"status"`
	RejectReason string            `json:"reject_reason,omitempty"`
	Screenshots  []string          `json:"screenshots"`
	MCVersions   []string          `json:"mc_versions"`
	Loaders      []string          `json:"loaders"`
	LatestVer    string            `json:"latest_version"`
	Downloads    int64             `json:"downloads"`
	Installs     int64             `json:"installs"`
	RatingSum    int64             `json:"rating_sum"`
	RatingCount  int64             `json:"rating_count"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
	Versions     []yyMarketVersion `json:"versions"`
}

type yyMarketOrder struct {
	PluginID   string `json:"plugin_id"`
	BuyerID    string `json:"buyer_id"`
	GrossCents int64  `json:"gross_cents"`
	CreatedAt  string `json:"created_at"`
}

type yyMarketIndex struct {
	Plugins []*yyMarketPlugin `json:"plugins"`
}

type yyMarketOrders struct {
	Orders []yyMarketOrder `json:"orders"`
}

type yyMarketStore struct {
	mu         sync.Mutex
	root       string
	indexPath  string
	ordersPath string
	plugins    []*yyMarketPlugin
	orders     []yyMarketOrder
	loaded     bool
}

func yyMarketNewStore(root string) *yyMarketStore {
	return &yyMarketStore{
		root:       root,
		indexPath:  filepath.Join(root, "plugins.json"),
		ordersPath: filepath.Join(root, "orders.json"),
	}
}

func (s *yyMarketStore) loadLocked() error {
	if s.loaded {
		return nil
	}
	if raw, err := os.ReadFile(s.indexPath); err == nil {
		var index yyMarketIndex
		if err := json.Unmarshal(raw, &index); err != nil {
			return fmt.Errorf("插件市场索引损坏: %w", err)
		}
		s.plugins = index.Plugins
	} else if !os.IsNotExist(err) {
		return err
	}
	if raw, err := os.ReadFile(s.ordersPath); err == nil {
		var orders yyMarketOrders
		if err := json.Unmarshal(raw, &orders); err != nil {
			return fmt.Errorf("插件市场订单损坏: %w", err)
		}
		s.orders = orders.Orders
	} else if !os.IsNotExist(err) {
		return err
	}
	s.loaded = true
	return nil
}

func yyMarketWriteJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".market-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(name)
		}
	}()
	if _, err = tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(name, 0600); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func (s *yyMarketStore) saveLocked() error {
	if err := yyMarketWriteJSON(s.indexPath, yyMarketIndex{Plugins: s.plugins}); err != nil {
		return err
	}
	return yyMarketWriteJSON(s.ordersPath, yyMarketOrders{Orders: s.orders})
}

func (s *yyMarketStore) findLocked(id string) *yyMarketPlugin {
	for _, p := range s.plugins {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func yyMarketJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func yyMarketErr(w http.ResponseWriter, status int, code, message string) {
	yyMarketJSON(w, status, map[string]any{"ok": false, "code": code, "error": message})
}

func yyMarketActor(r *http.Request) (yyPoolActor, bool) {
	actor, err := yyPoolActorRequest(r)
	if err != nil || actor.ID == "" {
		return actor, false
	}
	return actor, true
}

func yyMarketWriteActor(w http.ResponseWriter, r *http.Request) (yyPoolActor, bool) {
	actor, ok := yyMarketActor(r)
	if !ok {
		yyMarketErr(w, http.StatusUnauthorized, "login_required", "请先登录自己的游园账号")
		return actor, false
	}
	if actor.Scope == "read" {
		yyMarketErr(w, http.StatusForbidden, "forbidden", "当前会话只有读取权限")
		return actor, false
	}
	return actor, true
}

func yyMarketAuthorName(actor yyPoolActor) string {
	yyIdentity.Lock()
	defer yyIdentity.Unlock()
	user := yyIdentity.Data.User
	if user != nil {
		if id, _ := user["id"].(string); id == actor.ID {
			for _, key := range []string{"username", "nickname", "name", "display_name"} {
				if name, _ := user[key].(string); strings.TrimSpace(name) != "" {
					return strings.TrimSpace(name)
				}
			}
		}
	}
	return actor.ID
}

func yyMarketRuneLen(value string) int {
	return len([]rune(value))
}

func yyMarketClean(value string, limit int) (string, error) {
	value = strings.TrimSpace(value)
	if yyMarketRuneLen(value) > limit {
		return "", fmt.Errorf("内容过长（上限 %d 字符）", limit)
	}
	return value, nil
}

func yyMarketCleanList(values []string, limit int, itemLimit int) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || yyMarketRuneLen(value) > itemLimit || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func yyMarketScreenshotOK(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func yyMarketCleanScreenshots(values []string) []string {
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if yyMarketScreenshotOK(value) && len(out) < yyMarketMaxScreens {
			out = append(out, value)
		}
	}
	return out
}

func yyMarketNewID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "mk" + strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return "mk" + hex.EncodeToString(buf)
}

func yyMarketNow() string {
	return time.Now().Format(time.RFC3339)
}

// yyMarketSplit 返回开发者实收与平台服务费，单位分。
// net = gross * 70 / 100（向下取整），fee = gross - net，两者之和恒等于 gross。
func yyMarketSplit(gross int64) (net int64, fee int64) {
	if gross <= 0 {
		return 0, 0
	}
	net = gross * 70 / 100
	return net, gross - net
}

func yyMarketRating(p *yyMarketPlugin) float64 {
	if p.RatingCount <= 0 {
		return 0
	}
	return float64(p.RatingSum) / float64(p.RatingCount)
}

func yyMarketCard(p *yyMarketPlugin) map[string]any {
	return map[string]any{
		"id":             p.ID,
		"name":           p.Name,
		"summary":        p.Summary,
		"author":         p.AuthorName,
		"author_id":      p.AuthorID,
		"category":       p.Category,
		"tags":           yyMarketCleanList(p.Tags, yyMarketMaxTags, 20),
		"price_cents":    p.PriceCents,
		"free":           p.PriceCents == 0,
		"downloads":      p.Downloads,
		"installs":       p.Installs,
		"rating":         yyMarketRating(p),
		"rating_count":   p.RatingCount,
		"latest_version": p.LatestVer,
		"updated_at":     p.UpdatedAt,
	}
}

func yyMarketVersionView(v yyMarketVersion) map[string]any {
	return map[string]any{
		"version":     v.Version,
		"changelog":   v.Changelog,
		"mc_versions": yyMarketCleanList(v.MCVersions, yyMarketMaxMCVersions, 24),
		"loader":      v.Loader,
		"file_name":   v.FileName,
		"size_bytes":  v.SizeBytes,
		"sha256":      v.SHA256,
		"created_at":  v.CreatedAt,
	}
}

func yyMarketDetail(p *yyMarketPlugin) map[string]any {
	view := yyMarketCard(p)
	view["description"] = p.Description
	view["license"] = p.License
	view["screenshots"] = p.Screenshots
	view["mc_versions"] = p.MCVersions
	view["loaders"] = p.Loaders
	view["status"] = p.Status
	versions := make([]any, 0, len(p.Versions))
	for _, version := range p.Versions {
		versions = append(versions, yyMarketVersionView(version))
	}
	view["versions"] = versions
	return view
}

func yyMarketRoutes(mux *http.ServeMux, dataDir string) {
	store := yyMarketNewStore(filepath.Join(dataDir, "market"))
	mux.HandleFunc("/api/market/plugins", store.handlePlugins)
	mux.HandleFunc("/api/market/plugins/", store.handlePlugin)
	mux.HandleFunc("/api/market/mine", store.handleMine)
}

func (s *yyMarketStore) handlePlugins(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.list(w, r)
	case "POST":
		s.create(w, r)
	default:
		yyMarketErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "只支持 GET 或 POST")
	}
}

// yyMarketSupports 判断插件（含任一版本）是否声明支持某个 MC 版本或加载器。
func yyMarketSupports(p *yyMarketPlugin, value string, mcVersion bool) bool {
	values := p.Loaders
	if mcVersion {
		values = p.MCVersions
	}
	for _, item := range values {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	for _, version := range p.Versions {
		if mcVersion {
			for _, item := range version.MCVersions {
				if strings.EqualFold(item, value) {
					return true
				}
			}
			continue
		}
		if strings.EqualFold(version.Loader, value) {
			return true
		}
	}
	return false
}

func (s *yyMarketStore) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	search := strings.ToLower(strings.TrimSpace(query.Get("q")))
	category := strings.TrimSpace(query.Get("category"))
	mcVersion := strings.TrimSpace(query.Get("mc_version"))
	loader := strings.TrimSpace(query.Get("loader"))
	sortBy := query.Get("sort")
	if sortBy == "" {
		sortBy = "hot"
	}
	page, _ := strconv.Atoi(query.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(query.Get("page_size"))
	if pageSize < 1 {
		pageSize = 12
	}
	if pageSize > 50 {
		pageSize = 50
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	matches := make([]*yyMarketPlugin, 0, len(s.plugins))
	for _, p := range s.plugins {
		if p.Status != "approved" {
			continue
		}
		if category != "" && category != "all" && p.Category != category {
			continue
		}
		if mcVersion != "" && !yyMarketSupports(p, mcVersion, true) {
			continue
		}
		if loader != "" && !yyMarketSupports(p, loader, false) {
			continue
		}
		if search != "" {
			haystack := strings.ToLower(p.Name + " " + p.Summary + " " + p.AuthorName + " " + strings.Join(p.Tags, " "))
			if !strings.Contains(haystack, search) {
				continue
			}
		}
		matches = append(matches, p)
	}
	switch sortBy {
	case "new":
		sort.SliceStable(matches, func(i, j int) bool { return matches[i].CreatedAt > matches[j].CreatedAt })
	case "price":
		sort.SliceStable(matches, func(i, j int) bool {
			if matches[i].PriceCents != matches[j].PriceCents {
				return matches[i].PriceCents < matches[j].PriceCents
			}
			return matches[i].Name < matches[j].Name
		})
	default:
		sort.SliceStable(matches, func(i, j int) bool {
			if matches[i].Installs != matches[j].Installs {
				return matches[i].Installs > matches[j].Installs
			}
			return matches[i].UpdatedAt > matches[j].UpdatedAt
		})
	}
	total := len(matches)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := make([]any, 0, end-start)
	for _, p := range matches[start:end] {
		items = append(items, yyMarketCard(p))
	}
	counts := map[string]int{}
	for _, p := range s.plugins {
		if p.Status == "approved" {
			counts[p.Category]++
		}
	}
	yyMarketJSON(w, http.StatusOK, map[string]any{
		"ok": true, "items": items, "total": total, "page": page, "page_size": pageSize,
		"sort": sortBy, "category": category, "categories": counts,
		"mc_version": mcVersion, "loader": loader,
	})
}

type yyMarketCreateRequest struct {
	Name        string   `json:"name"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	PriceCents  int64    `json:"price_cents"`
	License     string   `json:"license"`
	Screenshots []string `json:"screenshots"`
	MCVersions  []string `json:"mc_versions"`
	Loaders     []string `json:"loaders"`
}

func yyMarketApplyRequest(p *yyMarketPlugin, req yyMarketCreateRequest) error {
	name, err := yyMarketClean(req.Name, yyMarketMaxName)
	if err != nil {
		return err
	}
	if yyMarketRuneLen(name) < 2 {
		return fmt.Errorf("插件名称至少 2 个字符")
	}
	summary, err := yyMarketClean(req.Summary, yyMarketMaxSummary)
	if err != nil {
		return err
	}
	description, err := yyMarketClean(req.Description, yyMarketMaxDetail)
	if err != nil {
		return err
	}
	license, err := yyMarketClean(req.License, 40)
	if err != nil {
		return err
	}
	if req.PriceCents < 0 || req.PriceCents > yyMarketMaxPriceCents {
		return fmt.Errorf("价格必须在 0 到 %d 分之间", yyMarketMaxPriceCents)
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "other"
	}
	if !yyMarketCategorySet[category] {
		return fmt.Errorf("分类无效")
	}
	p.Name = name
	p.Summary = summary
	p.Description = description
	p.License = license
	p.Category = category
	p.PriceCents = req.PriceCents
	p.Tags = yyMarketCleanList(req.Tags, yyMarketMaxTags, 20)
	p.Screenshots = yyMarketCleanScreenshots(req.Screenshots)
	p.MCVersions = yyMarketCleanList(req.MCVersions, yyMarketMaxMCVersions, 24)
	p.Loaders = yyMarketCleanList(req.Loaders, yyMarketMaxLoaders, 24)
	return nil
}

func (s *yyMarketStore) create(w http.ResponseWriter, r *http.Request) {
	actor, ok := yyMarketWriteActor(w, r)
	if !ok {
		return
	}
	var req yyMarketCreateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&req); err != nil {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", "请求内容不是有效 JSON")
		return
	}
	now := yyMarketNow()
	plugin := &yyMarketPlugin{
		ID:         yyMarketNewID(),
		AuthorID:   actor.ID,
		AuthorName: yyMarketAuthorName(actor),
		Status:     "draft",
		CreatedAt:  now,
		UpdatedAt:  now,
		Versions:   []yyMarketVersion{},
	}
	if err := yyMarketApplyRequest(plugin, req); err != nil {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	s.plugins = append(s.plugins, plugin)
	if err := s.saveLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	yyMarketJSON(w, http.StatusOK, map[string]any{"ok": true, "plugin": yyMarketDetail(plugin)})
}

func (s *yyMarketStore) handleMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		yyMarketErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "只支持 GET")
		return
	}
	actor, ok := yyMarketActor(r)
	if !ok {
		yyMarketErr(w, http.StatusUnauthorized, "login_required", "请先登录自己的游园账号")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	items := []any{}
	grossByPlugin := map[string]int64{}
	orderCount := 0
	for _, order := range s.orders {
		grossByPlugin[order.PluginID] += order.GrossCents
		orderCount++
	}
	var gross, installs int64
	for _, p := range s.plugins {
		if p.AuthorID != actor.ID {
			continue
		}
		pluginGross := grossByPlugin[p.ID]
		net, fee := yyMarketSplit(pluginGross)
		item := yyMarketDetail(p)
		item["gross_cents"] = pluginGross
		item["platform_fee_cents"] = fee
		item["developer_net_cents"] = net
		item["version_count"] = len(p.Versions)
		item["reject_reason"] = p.RejectReason
		items = append(items, item)
		gross += pluginGross
		installs += p.Installs
	}
	net, fee := yyMarketSplit(gross)
	yyMarketJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"items": items,
		"summary": map[string]any{
			"gross_cents":         gross,
			"platform_fee_cents":  fee,
			"developer_net_cents": net,
			"installs":            installs,
			"plugin_count":        len(items),
			"order_count":         orderCount,
			"fee_percent":         yyMarketFeePercent,
		},
	})
}

func (s *yyMarketStore) handlePlugin(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/market/plugins/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		yyMarketErr(w, http.StatusNotFound, "not_found", "缺少插件 id")
		return
	}
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	switch {
	case action == "" && r.Method == "GET":
		s.detail(w, r, id)
	case action == "versions" && r.Method == "POST":
		s.uploadVersion(w, r, id)
	case action == "submit" && r.Method == "POST":
		s.submit(w, r, id)
	case action == "review" && r.Method == "POST":
		s.review(w, r, id)
	case action == "install" && r.Method == "POST":
		s.install(w, r, id)
	case action == "edit" && r.Method == "POST":
		s.edit(w, r, id)
	default:
		yyMarketErr(w, http.StatusNotFound, "not_found", "接口不存在")
	}
}

func (s *yyMarketStore) detail(w http.ResponseWriter, r *http.Request, id string) {
	actor, _ := yyPoolActorRequest(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	plugin := s.findLocked(id)
	if plugin == nil {
		yyMarketErr(w, http.StatusNotFound, "not_found", "插件不存在")
		return
	}
	owner := actor.ID != "" && actor.ID == plugin.AuthorID
	if plugin.Status != "approved" && !owner && !actor.Admin {
		yyMarketErr(w, http.StatusNotFound, "not_found", "插件不存在")
		return
	}
	view := yyMarketDetail(plugin)
	if !owner && !actor.Admin {
		delete(view, "status")
	}
	yyMarketJSON(w, http.StatusOK, map[string]any{"ok": true, "plugin": view})
}

func (s *yyMarketStore) edit(w http.ResponseWriter, r *http.Request, id string) {
	actor, ok := yyMarketWriteActor(w, r)
	if !ok {
		return
	}
	var req yyMarketCreateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&req); err != nil {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", "请求内容不是有效 JSON")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	plugin := s.findLocked(id)
	if plugin == nil {
		yyMarketErr(w, http.StatusNotFound, "not_found", "插件不存在")
		return
	}
	if plugin.AuthorID != actor.ID {
		yyMarketErr(w, http.StatusForbidden, "forbidden", "只能修改自己发布的插件")
		return
	}
	if plugin.Status != "draft" && plugin.Status != "rejected" && plugin.Status != "offline" {
		yyMarketErr(w, http.StatusConflict, "conflict", "审核中或已上架的插件不能改基础信息")
		return
	}
	if err := yyMarketApplyRequest(plugin, req); err != nil {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	plugin.UpdatedAt = yyMarketNow()
	if err := s.saveLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	yyMarketJSON(w, http.StatusOK, map[string]any{"ok": true, "plugin": yyMarketDetail(plugin)})
}

func yyMarketSafeFileName(name string) (string, string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || strings.Contains(name, "/") || strings.ContainsAny(name, ":\x00") || name == "." || name == ".." {
		return "", "", fmt.Errorf("文件名无效")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !yyMarketAllowedExt[ext] {
		return "", "", fmt.Errorf("不支持的文件类型，仅允许 .zip/.lua/.py/.js")
	}
	return name, ext, nil
}

func (s *yyMarketStore) uploadVersion(w http.ResponseWriter, r *http.Request, id string) {
	actor, ok := yyMarketWriteActor(w, r)
	if !ok {
		return
	}

	var (
		fileName   string
		data       []byte
		version    string
		changelog  string
		loader     string
		mcVersions []string
	)
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, yyMarketMaxUploadBytes+(4<<20))
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			yyMarketErr(w, http.StatusBadRequest, "invalid_request", "解析上传内容失败: "+err.Error())
			return
		}
		version = r.FormValue("version")
		changelog = r.FormValue("changelog")
		loader = r.FormValue("loader")
		if raw := r.FormValue("mc_versions"); raw != "" {
			mcVersions = strings.Split(raw, ",")
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			yyMarketErr(w, http.StatusBadRequest, "invalid_request", "缺少文件字段 file")
			return
		}
		defer file.Close()
		data, err = io.ReadAll(io.LimitReader(file, yyMarketMaxUploadBytes+1))
		if err != nil {
			yyMarketErr(w, http.StatusBadRequest, "invalid_request", "读取上传文件失败")
			return
		}
		fileName = header.Filename
	} else {
		limit := yyMarketMaxUploadBytes*4/3 + (1 << 20)
		var req struct {
			Version       string   `json:"version"`
			Changelog     string   `json:"changelog"`
			MCVersions    []string `json:"mc_versions"`
			Loader        string   `json:"loader"`
			FileName      string   `json:"file_name"`
			ContentBase64 string   `json:"content_base64"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit)).Decode(&req); err != nil {
			yyMarketErr(w, http.StatusRequestEntityTooLarge, "too_large", "上传内容过大或不是有效 JSON")
			return
		}
		version, changelog, loader, mcVersions = req.Version, req.Changelog, req.Loader, req.MCVersions
		fileName = req.FileName
		content := strings.TrimSpace(req.ContentBase64)
		if idx := strings.Index(content, ","); idx >= 0 && strings.HasPrefix(content, "data:") {
			content = content[idx+1:]
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			yyMarketErr(w, http.StatusBadRequest, "invalid_request", "文件内容不是有效的 base64")
			return
		}
		data = decoded
	}

	if len(data) == 0 {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", "上传文件为空")
		return
	}
	if int64(len(data)) > yyMarketMaxUploadBytes {
		yyMarketErr(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("文件超过 %d MB", yyMarketMaxUploadBytes>>20))
		return
	}
	cleanName, _, err := yyMarketSafeFileName(fileName)
	if err != nil {
		code, status := "invalid_request", http.StatusBadRequest
		if strings.Contains(err.Error(), "不支持的文件类型") {
			code, status = "unsupported_file", http.StatusUnsupportedMediaType
		}
		yyMarketErr(w, status, code, err.Error())
		return
	}
	version, err = yyMarketClean(version, 32)
	if err != nil || version == "" {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", "版本号不能为空且不超过 32 字符")
		return
	}
	for _, r := range version {
		if !(r == '.' || r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			yyMarketErr(w, http.StatusBadRequest, "invalid_request", "版本号只能包含字母、数字、点、横线、下划线")
			return
		}
	}
	changelog, err = yyMarketClean(changelog, yyMarketMaxChangelog)
	if err != nil {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	loader, err = yyMarketClean(loader, 24)
	if err != nil {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	plugin := s.findLocked(id)
	if plugin == nil {
		yyMarketErr(w, http.StatusNotFound, "not_found", "插件不存在")
		return
	}
	if plugin.AuthorID != actor.ID {
		yyMarketErr(w, http.StatusForbidden, "forbidden", "只能给自己发布的插件上传版本")
		return
	}
	if plugin.Status == "pending" {
		yyMarketErr(w, http.StatusConflict, "conflict", "审核中不能替换版本，请等待审核结果")
		return
	}
	for _, existing := range plugin.Versions {
		if existing.Version == version {
			yyMarketErr(w, http.StatusConflict, "conflict", "版本号已存在")
			return
		}
	}
	filePath, sum, err := yyMarketStoreFile(s.root, plugin.ID, data)
	if err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", "保存文件失败: "+err.Error())
		return
	}
	now := yyMarketNow()
	item := yyMarketVersion{
		Version:    version,
		Changelog:  changelog,
		MCVersions: yyMarketCleanList(mcVersions, yyMarketMaxMCVersions, 24),
		Loader:     loader,
		FileName:   cleanName,
		SizeBytes:  int64(len(data)),
		SHA256:     sum,
		CreatedAt:  now,
		File:       filePath,
	}
	plugin.Versions = append(plugin.Versions, item)
	plugin.LatestVer = version
	plugin.UpdatedAt = now
	if len(item.MCVersions) > 0 {
		plugin.MCVersions = yyMarketCleanList(append(plugin.MCVersions, item.MCVersions...), yyMarketMaxMCVersions, 24)
	}
	if loader != "" {
		plugin.Loaders = yyMarketCleanList(append(plugin.Loaders, loader), yyMarketMaxLoaders, 24)
	}
	if plugin.Status == "approved" {
		plugin.Status = "pending"
	}
	if err := s.saveLocked(); err != nil {
		os.Remove(filePath)
		plugin.Versions = plugin.Versions[:len(plugin.Versions)-1]
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	yyMarketJSON(w, http.StatusOK, map[string]any{"ok": true, "version": yyMarketVersionView(item), "status": plugin.Status})
}

func yyMarketStoreFile(root, pluginID string, data []byte) (string, string, error) {
	dir := filepath.Join(root, "files", pluginID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	name := hex.EncodeToString(buf) + ".bin"
	path := filepath.Join(dir, name)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return "", "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return "", "", err
	}
	sum := sha256.Sum256(data)
	return filepath.ToSlash(filepath.Join("files", pluginID, name)), hex.EncodeToString(sum[:]), nil
}

func (s *yyMarketStore) submit(w http.ResponseWriter, r *http.Request, id string) {
	actor, ok := yyMarketWriteActor(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	plugin := s.findLocked(id)
	if plugin == nil {
		yyMarketErr(w, http.StatusNotFound, "not_found", "插件不存在")
		return
	}
	if plugin.AuthorID != actor.ID {
		yyMarketErr(w, http.StatusForbidden, "forbidden", "只能提交自己发布的插件")
		return
	}
	switch plugin.Status {
	case "draft", "rejected", "offline":
	default:
		yyMarketErr(w, http.StatusConflict, "conflict", "当前状态不能提交审核")
		return
	}
	if len(plugin.Versions) == 0 {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", "至少上传一个版本后才能提交审核")
		return
	}
	plugin.Status = "pending"
	plugin.RejectReason = ""
	plugin.UpdatedAt = yyMarketNow()
	if err := s.saveLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	yyMarketJSON(w, http.StatusOK, map[string]any{"ok": true, "status": plugin.Status})
}

func (s *yyMarketStore) review(w http.ResponseWriter, r *http.Request, id string) {
	actor, ok := yyMarketWriteActor(w, r)
	if !ok {
		return
	}
	if !actor.Admin {
		yyMarketErr(w, http.StatusForbidden, "forbidden", "只有平台管理员可以审核")
		return
	}
	var req struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", "请求内容不是有效 JSON")
		return
	}
	action := strings.TrimSpace(req.Action)
	if action != "approve" && action != "reject" {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", "action 只能是 approve 或 reject")
		return
	}
	reason, err := yyMarketClean(req.Reason, yyMarketMaxReason)
	if err != nil {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if action == "reject" && reason == "" {
		yyMarketErr(w, http.StatusBadRequest, "invalid_request", "驳回必须填写理由")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	plugin := s.findLocked(id)
	if plugin == nil {
		yyMarketErr(w, http.StatusNotFound, "not_found", "插件不存在")
		return
	}
	if plugin.Status != "pending" {
		yyMarketErr(w, http.StatusConflict, "conflict", "只有待审核的插件可以审核")
		return
	}
	if action == "approve" {
		plugin.Status = "approved"
		plugin.RejectReason = ""
	} else {
		plugin.Status = "rejected"
		plugin.RejectReason = reason
	}
	plugin.UpdatedAt = yyMarketNow()
	if err := s.saveLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	yyMarketJSON(w, http.StatusOK, map[string]any{"ok": true, "status": plugin.Status, "reject_reason": plugin.RejectReason})
}

func (s *yyMarketStore) install(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Version string `json:"version"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "market_store_error", err.Error())
		return
	}
	plugin := s.findLocked(id)
	if plugin == nil || plugin.Status != "approved" {
		yyMarketErr(w, http.StatusNotFound, "not_found", "插件不存在或未上架")
		return
	}
	if plugin.PriceCents > 0 {
		yyMarketErr(w, http.StatusPaymentRequired, "payment_not_connected", "支付未接入，暂不能购买或安装付费插件")
		return
	}
	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = plugin.LatestVer
	}
	var selected *yyMarketVersion
	for i := range plugin.Versions {
		if plugin.Versions[i].Version == version {
			selected = &plugin.Versions[i]
			break
		}
	}
	if selected == nil {
		yyMarketErr(w, http.StatusNotFound, "not_found", "找不到指定版本")
		return
	}
	if !strings.EqualFold(filepath.Ext(selected.FileName), ".zip") {
		yyMarketErr(w, http.StatusUnsupportedMediaType, "unsupported_file", "该格式暂不支持一键安装，请下载后手动导入")
		return
	}
	if pluginController == nil {
		yyMarketErr(w, http.StatusInternalServerError, "install_failed", "插件控制器未就绪")
		return
	}
	packagePath := selected.File
	if !filepath.IsAbs(packagePath) {
		packagePath = filepath.Join(s.root, filepath.FromSlash(packagePath))
	}
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "install_failed", "插件目录不可用")
		return
	}
	packageAbs, err := filepath.Abs(packagePath)
	if err != nil || !strings.HasPrefix(packageAbs, rootAbs+string(os.PathSeparator)) {
		yyMarketErr(w, http.StatusInternalServerError, "install_failed", "插件包路径无效")
		return
	}
	importedID, err := pluginController.ImportPackage(packageAbs)
	if err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "install_failed", "导入失败: "+err.Error())
		return
	}
	if err := pluginController.ScanPlugins(); err != nil {
		yyMarketErr(w, http.StatusInternalServerError, "install_failed", "刷新插件列表失败: "+err.Error())
		return
	}
	plugin.Installs++
	plugin.UpdatedAt = yyMarketNow()
	_ = s.saveLocked()
	yyMarketJSON(w, http.StatusOK, map[string]any{
		"ok": true, "installed": true, "plugin_id": plugin.ID, "version": selected.Version,
		"location": "插件文件/" + importedID, "sha256": selected.SHA256,
	})
}
