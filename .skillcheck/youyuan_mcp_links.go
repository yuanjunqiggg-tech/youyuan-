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
var yyMCPReadPaths = map[string]bool{"/api/bot/status": true, "/api/youyuan/observations": true, "/api/youyuan/runtime/list": true, "/api/youyuan/docs": true, "/api/youyuan/sdk/catalog": true}

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
