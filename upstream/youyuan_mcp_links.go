package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type yyMCPLink struct {
	Token   string `json:"token"`
	Scope   string `json:"scope"`
	Expires int64  `json:"expires"`
	Name    string `json:"name"`
}

var yyMCPLinks = struct {
	sync.RWMutex
	Items []yyMCPLink
	Path  string
}{}
var yyMCPReadPaths = map[string]bool{
	"/api/bot/status": true, "/api/youyuan/observations": true, "/api/youyuan/runtime/list": true,
	"/api/youyuan/docs": true, "/api/youyuan/sdk/catalog": true, "/api/youyuan/world/seed": true, "/api/market/plugins": true,
	// 统一快照是七项只读概览的聚合，本身不含写操作；给只读连接用能少打 6 次请求。
	"/api/youyuan/snapshot": true,
	// 种子查询全部是 GET + 纯本机计算（/api/seed/*），只读连接可以查；
	// 但批量/高频计算请在本机（插件或页面）做，不要反复打这几个接口。
	"/api/seed/manifest": true, "/api/seed/slime": true, "/api/seed/stronghold": true,
	"/api/seed/structure": true, "/api/seed/query": true, "/api/seed/heat": true,
	"/api/seed/coordinates": true,
	"/api/seed/biome":       true,
	// 种子查询 v2：同样是纯 GET + 纯本机计算。tile.png 只回图片字节，
	// 对只读连接也开放（页面本身就可能由只读连接访问）。
	"/api/seed/v2/manifest":   true,
	"/api/seed/v2/icon.svg":   true,
	"/api/seed/v2/meta":       true,
	"/api/seed/v2/tile.png":   true,
	"/api/seed/v2/structures": true,
	"/api/seed/v2/biome":      true,
	"/api/seed/v2/slime":      true,
	"/api/seed/v2/stronghold": true,
	"/api/seed/v2/summary":    true,
	// 帮助菜单是纯 GET + 内存读数（注册表投影），只读连接可以查。
	"/api/qq/help": true,
	// 多群未读汇总：纯 GET + 内存桶计数，只回条数与时间不回正文，只读连接可以查。
	"/api/qq/messages/summary": true,
	// 转发现状（模式/间隔/上限/排队条数）是纯 GET + 内存读数，只读连接可以查；
	// 改模式（POST）需要管理连接。
	"/api/qq/relay": true,
	// 知识库目录树与单篇正文都是纯 GET + 本地库读取（内置技能读内嵌文件），
	// 只读连接可以查；上传/保存/删除不开放给只读连接。
	"/api/qq/kb/tree": true, "/api/qq/kb/doc": true,
	// MCORT 公告读取是纯 GET + 本地 JSON；写入（POST）仍需管理员，故不在此表。
	"/api/mcort/announcements": true,
	// 按账号取皮肤是纯 GET + 本地磁盘缓存（命中后不打远端），只读连接可以查。
	"/api/skin/account-head": true, "/api/skin/account-texture": true,
	// 组件码（skin_item_id）→ 纹理同为纯 GET，命中组件码缓存后不打远端；
	// 组件码列表是本地 JSON 的只读投影，同样给只读连接。
	"/api/skin/item-texture": true, "/api/skin/item-library": true}

func yyMCPReadTool(name string) bool {
	for _, r := range yyAPIRoutes {
		if r.Name == name && yyMCPReadPaths[r.Path] {
			return true
		}
	}
	return false
}
func yyMCPReadCheck(method string, params json.RawMessage) error {
	switch method {
	case "initialize", "ping", "tools/list", "resources/list", "resources/templates/list":
		return nil
	case "resources/read":
		var p struct {
			URI string `json:"uri"`
		}
		_ = json.Unmarshal(params, &p)
		if strings.HasPrefix(p.URI, "youyuan://docs/") || p.URI == "youyuan://skills/catalog" || p.URI == "youyuan://runtime/status" || p.URI == "youyuan://accounts/status" {
			return nil
		}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(params, &p)
		m, _ := p.Arguments["method"].(string)
		if yyMCPReadTool(p.Name) && (m == "" || m == "GET") {
			return nil
		}
	}
	return fmt.Errorf("此连接仅授权状态和开发文档查询；修改文件、运行指令请另建管理连接")
}
func yyMCPLinkLookup(token string) (yyMCPLink, bool) {
	yyMCPLinks.RLock()
	defer yyMCPLinks.RUnlock()
	for _, g := range yyMCPLinks.Items {
		if g.Token == token && g.Expires > time.Now().UnixMilli() {
			return g, true
		}
	}
	return yyMCPLink{}, false
}
func yyMCPLinksRoutes(mux *http.ServeMux, root string) {
	yyMCPLinks.Path = filepath.Join(root, "mcp-links.json")
	raw, _ := os.ReadFile(yyMCPLinks.Path)
	_ = json.Unmarshal(raw, &yyMCPLinks.Items)
	mux.HandleFunc("/api/youyuan/mcp-links", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Op    string `json:"op"`
			Token string `json:"token"`
			Scope string `json:"scope"`
			Hours int    `json:"hours"`
			Name  string `json:"name"`
		}
		if r.Method == "POST" {
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); e != nil {
				http.Error(w, "JSON无效", 400)
				return
			}
		} else if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		yyMCPLinks.Lock()
		defer yyMCPLinks.Unlock()
		items := append([]yyMCPLink{}, yyMCPLinks.Items...)
		var created *yyMCPLink
		switch p.Op {
		case "", "list":
		case "create":
			if p.Scope == "" {
				p.Scope = "read"
			}
			if p.Scope != "read" && p.Scope != "control" {
				http.Error(w, "scope支持read/control", 400)
				return
			}
			if p.Hours == 0 {
				p.Hours = 720
			}
			if p.Hours < 1 || p.Hours > 8760 {
				http.Error(w, "有效期为1–8760小时", 400)
				return
			}
			active := 0
			for _, g := range items {
				if g.Expires > time.Now().UnixMilli() {
					active++
				}
			}
			if active >= 128 {
				http.Error(w, "请先撤销不用的连接", 409)
				return
			}
			// Prune expired entries, keeping the independent key separate from the main key.
			next := []yyMCPLink{}
			for _, g := range items {
				if g.Expires > time.Now().UnixMilli() {
					next = append(next, g)
				}
			}
			g := yyMCPLink{Token: yyNewKey(), Scope: p.Scope, Expires: time.Now().Add(time.Duration(p.Hours) * time.Hour).UnixMilli(), Name: p.Name}
			items = append(next, g)
			created = &g
		case "revoke":
			next := []yyMCPLink{}
			for _, g := range items {
				if g.Token != p.Token {
					next = append(next, g)
				}
			}
			items = next
		default:
			http.Error(w, "op支持list/create/revoke", 400)
			return
		}
		if p.Op == "create" || p.Op == "revoke" {
			raw, _ := json.MarshalIndent(items, "", "  ")
			e := os.WriteFile(yyMCPLinks.Path+".tmp", raw, 0600)
			if e == nil {
				e = os.Rename(yyMCPLinks.Path+".tmp", yyMCPLinks.Path)
			}
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			yyMCPLinks.Items = items
		}
		result := map[string]any{"ok": true, "links": items}
		if created != nil {
			result["created"] = created
		}
		writeJSON(w, result)
	})
}
