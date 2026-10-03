package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type yyQQGroup struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Number  string `json:"number,omitempty"`
	Enabled bool   `json:"enabled"`
}

var yyQQGroupClaim = struct {
	sync.Mutex
	Code  string
	Until time.Time
}{}

// nil is a legacy single-group configuration; an explicit [] means no groups.
func yyQQGroups(cf yyQQConfig) []yyQQGroup {
	if cf.Groups != nil {
		return append([]yyQQGroup{}, cf.Groups...)
	}
	id := yyQQGroupString(cf.GroupID)
	if id == "" || id == "0" {
		return nil
	}
	return []yyQQGroup{{ID: id, Name: "原互通群", Number: cf.GroupNumber, Enabled: true}}
}
func yyQQGroupEnabled(cf yyQQConfig, id string) bool {
	if id == "" {
		return false
	}
	for _, g := range yyQQGroups(cf) {
		if g.ID == id {
			return g.Enabled
		}
	}
	return false
}
func yyQQValidateGroups(cf yyQQConfig) error {
	if len(cf.Groups) > 32 {
		return fmt.Errorf("最多32个互通群")
	}
	seen := map[string]bool{}
	for _, g := range cf.Groups {
		if g.ID == "" || len(g.ID) > 128 || strings.ContainsAny(g.ID, "/\\\r\n\t ") || seen[g.ID] {
			return fmt.Errorf("群标识无效或重复")
		}
		if cf.Adapter == "qqofficial" && yyQQNumericGroup.MatchString(g.ID) {
			return fmt.Errorf("官方QQ请使用group_openid")
		}
		if cf.Adapter == "onebot" && !yyQQNumericGroup.MatchString(g.ID) {
			return fmt.Errorf("OneBot请使用数字群号")
		}
		seen[g.ID] = true
	}
	return nil
}
func yyQQReplyGroup(id string) string {
	yyQQReplies.Lock()
	defer yyQQReplies.Unlock()
	if window := yyQQReplies.Items[id]; window != nil && time.Now().Before(window.Until) {
		return window.Group
	}
	return ""
}
func yyQQSaveGroupsLocked(groups []yyQQGroup) error {
	next := yyQQ
	next.Groups = groups
	data, e := json.MarshalIndent(next, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(yyQQPath, data, 0600); e != nil {
		return e
	}
	yyQQ = next
	return nil
}
func yyQQGroupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/groups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			cf := yyQQSnapshot()
			writeJSON(w, map[string]any{"ok": true, "groups": yyQQGroups(cf), "official_numbers_are_notes": cf.Adapter == "qqofficial"})
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "GET/POST required", 405)
			return
		}
		var p struct {
			Action  string `json:"action"`
			ID      string `json:"id"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "无效请求"})
			return
		}
		if p.Action == "claim" {
			yyQQGroupClaim.Lock()
			yyQQGroupClaim.Code = yyNewKey()[:16]
			yyQQGroupClaim.Until = time.Now().Add(10 * time.Minute)
			code := yyQQGroupClaim.Code
			yyQQGroupClaim.Unlock()
			writeJSON(w, map[string]any{"ok": true, "command": "/加群 " + code, "expires_seconds": 600})
			return
		}
		yyQQMu.Lock()
		defer yyQQMu.Unlock()
		groups := yyQQGroups(yyQQ)
		found := -1
		for i, g := range groups {
			if g.ID == p.ID {
				found = i
			}
		}
		switch p.Action {
		case "add":
			p.ID = strings.TrimSpace(p.ID)
			if p.ID == "" || len(p.ID) > 128 || strings.ContainsAny(p.ID, "/\\\r\n\t ") || len(groups) >= 32 {
				writeJSON(w, map[string]any{"ok": false, "error": "群标识无效或已达32群上限"})
				return
			}
			if yyQQ.Adapter == "qqofficial" && yyQQNumericGroup.MatchString(p.ID) {
				writeJSON(w, map[string]any{"ok": false, "error": "QQ官方不能用数字群号，请生成加群码，在该群 @机器人发送"})
				return
			}
			if yyQQ.Adapter == "onebot" && !yyQQNumericGroup.MatchString(p.ID) {
				writeJSON(w, map[string]any{"ok": false, "error": "OneBot需要数字群号"})
				return
			}
			if found >= 0 {
				writeJSON(w, map[string]any{"ok": false, "error": "该群已存在"})
				return
			}
			if p.Name == "" {
				p.Name = "互通群"
			}
			groups = append(groups, yyQQGroup{ID: p.ID, Name: yyQQCardClip(p.Name, 48), Enabled: true})
		case "update":
			if found < 0 {
				writeJSON(w, map[string]any{"ok": false, "error": "找不到群"})
				return
			}
			groups[found].Enabled = p.Enabled
			if p.Name != "" {
				groups[found].Name = yyQQCardClip(p.Name, 48)
			}
		case "delete":
			if found < 0 {
				writeJSON(w, map[string]any{"ok": false, "error": "找不到群"})
				return
			}
			groups = append(groups[:found], groups[found+1:]...)
			if groups == nil {
				groups = []yyQQGroup{}
			}
		default:
			writeJSON(w, map[string]any{"ok": false, "error": "未知操作"})
			return
		}
		if e := yyQQSaveGroupsLocked(groups); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "groups": groups})
	})
}

// Called only for authenticated Tencent group events. It never admits arbitrary
// groups just because the bot has been invited there.
func yyQQEnrollOfficialGroup(group, text string) (bool, error) {
	if !yyQQHasPrefix(text, "/加群") {
		return false, nil
	}
	code := strings.TrimSpace(strings.TrimPrefix(text, "/加群"))
	yyQQGroupClaim.Lock()
	defer yyQQGroupClaim.Unlock()
	if yyQQGroupClaim.Code == "" || code != yyQQGroupClaim.Code || time.Now().After(yyQQGroupClaim.Until) {
		return false, fmt.Errorf("加群码无效或过期")
	}
	yyQQMu.Lock()
	defer yyQQMu.Unlock()
	groups := yyQQGroups(yyQQ)
	for _, g := range groups {
		if g.ID == group {
			return false, fmt.Errorf("该群已添加")
		}
	}
	if len(groups) >= 32 {
		return false, fmt.Errorf("最多32个群")
	}
	groups = append(groups, yyQQGroup{ID: group, Name: fmt.Sprintf("互通群%d", len(groups)+1), Enabled: true})
	if e := yyQQSaveGroupsLocked(groups); e != nil {
		return false, e
	}
	yyQQGroupClaim.Code = ""
	return true, nil
}
