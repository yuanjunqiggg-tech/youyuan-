package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type yyRoomCandidate struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	RoomID     string `json:"room_id"`
	Target     string `json:"target"`
	Name       string `json:"name,omitempty"`
	Number     string `json:"number,omitempty"`
	Owner      string `json:"owner,omitempty"`
	OwnerID    string `json:"owner_id,omitempty"`
	Cover      string `json:"cover_url,omitempty"`
	Avatar     string `json:"avatar_url,omitempty"`
	Players    *int64 `json:"players,omitempty"`
	MaxPlayers *int64 `json:"max_players,omitempty"`
	Version    string `json:"version,omitempty"`
}
type yyRoomResult struct {
	OK         bool              `json:"ok"`
	Complete   bool              `json:"complete"`
	Candidates []yyRoomCandidate `json:"candidates"`
	Sources    []map[string]any  `json:"sources"`
	Error      string            `json:"error,omitempty"`
	Selected   *yyRoomCandidate  `json:"selected,omitempty"`
	Remembered bool              `json:"remembered,omitempty"`
}
type yyRoomChoice struct {
	Input     string          `json:"input"`
	Candidate yyRoomCandidate `json:"candidate"`
	At        time.Time       `json:"at"`
}
type yyRoomSearch struct {
	Result yyRoomResult
	At     time.Time
}

var yyRooms = struct {
	sync.Mutex
	Searches map[string]yyRoomSearch
}{Searches: map[string]yyRoomSearch{}}

func yyRoomBinding(token, auth, input string) string {
	sum := sha256.Sum256([]byte(token + "\x00" + strings.TrimRight(strings.TrimSpace(auth), "/") + "\x00" + strings.TrimSpace(input)))
	return hex.EncodeToString(sum[:])
}
func yyRoomChoicesPath() string { return filepath.Join(yyAccounts.Root, "room-choices.json") }

func yyRoomRememberedTarget(token, auth, input string) string {
	yyRooms.Lock()
	defer yyRooms.Unlock()
	if choice, ok := yyRoomLoadChoices()[yyRoomBinding(token, auth, input)]; ok && yyRoomValidCandidate(choice.Candidate) {
		return choice.Candidate.Target
	}
	return input
}
func yyRoomLoadChoices() map[string]yyRoomChoice {
	choices := map[string]yyRoomChoice{}
	b, e := os.ReadFile(yyRoomChoicesPath())
	if e == nil {
		_ = json.Unmarshal(b, &choices)
	}
	return choices
}
func yyRoomRemember(key, input string, candidate yyRoomCandidate) error {
	choices := yyRoomLoadChoices()
	choices[key] = yyRoomChoice{Input: input, Candidate: candidate, At: time.Now()}
	for len(choices) > 64 {
		oldest := ""
		for k, v := range choices {
			if oldest == "" || v.At.Before(choices[oldest].At) {
				oldest = k
			}
		}
		delete(choices, oldest)
	}
	b, e := json.Marshal(choices)
	if e != nil {
		return e
	}
	path := yyRoomChoicesPath()
	if e = os.WriteFile(path+".tmp", b, 0600); e != nil {
		return e
	}
	return os.Rename(path+".tmp", path)
}
func yyRoomDiscoverRemote(r *http.Request, token, auth, input string) yyRoomResult {
	body, _ := json.Marshal(map[string]any{"keyword": input})
	request, e := http.NewRequestWithContext(r.Context(), "POST", strings.TrimRight(auth, "/")+"/api/game/discover", bytes.NewReader(body))
	if e != nil {
		return yyRoomResult{Error: "认证服务地址无效"}
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 35 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Do(request)
	if e != nil {
		return yyRoomResult{Error: "房间发现请求失败，请检查认证服务后重试"}
	}
	defer response.Body.Close()
	var result yyRoomResult
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result) != nil {
		return yyRoomResult{Error: "认证服务尚未提供房间发现，请更新认证服务"}
	}
	if response.StatusCode != 200 || !result.OK {
		return yyRoomResult{Error: "房间发现失败，请检查游戏账号登录状态及认证服务"}
	}
	seen := map[string]bool{}
	filtered := []yyRoomCandidate{}
	for _, v := range result.Candidates {
		if !yyRoomValidCandidate(v) {
			result.Complete = false
			continue
		}
		if seen[v.ID] {
			continue
		}
		seen[v.ID] = true
		filtered = append(filtered, v)
	}
	result.Candidates = filtered
	// A provider cannot make an incomplete response eligible for auto-connect.
	kinds := map[string]bool{}
	for _, s := range result.Sources {
		if s["ok"] != true || s["more"] == true {
			result.Complete = false
		}
		kind, _ := s["type"].(string)
		kinds[kind] = true
	}
	if !kinds["rental"] || !kinds["lobby"] || !kinds["local"] {
		result.Complete = false
	}
	return result
}
func yyRoomValidCandidate(v yyRoomCandidate) bool {
	if v.RoomID == "" || v.ID != v.Type+":"+v.RoomID {
		return false
	}
	switch v.Type {
	case "rental":
		return v.Target != "" && !strings.ContainsAny(v.Target, "@:#/\\")
	case "lobby":
		return yyExplicitRoomID.MatchString(v.RoomID) && v.Target == "LobbyGame:room_id="+v.RoomID
	case "local":
		return yyExplicitRoomID.MatchString(v.RoomID) && v.Target == "@"+v.RoomID
	}
	return false
}
func yyRoomDiscoveryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Op    string `json:"op"`
		Token string `json:"token"`
		Auth  string `json:"auth"`
		Input string `json:"input"`
		ID    string `json:"candidate_id"`
		Force bool   `json:"force"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&p) != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
		return
	}
	p.Input = strings.TrimSpace(p.Input)
	auth, e := yyProviderURL(p.Auth)
	if e != nil || p.Token == "" || p.Input == "" || len(p.Input) > 512 {
		writeJSON(w, map[string]any{"ok": false, "error": "请填写连接内容、本人游戏令牌与认证服务"})
		return
	}
	key := yyRoomBinding(p.Token, auth, p.Input)
	yyRooms.Lock()
	choices := yyRoomLoadChoices()
	if p.Op == "current" {
		var chosen *yyRoomChoice
		for k, v := range choices {
			if (v.Candidate.Target == p.Input || v.Input == p.Input) && k == yyRoomBinding(p.Token, auth, v.Input) && (chosen == nil || v.At.After(chosen.At)) {
				copy := v
				chosen = &copy
			}
		}
		yyRooms.Unlock()
		writeJSON(w, map[string]any{"ok": true, "choice": chosen})
		return
	}
	if choice, ok := choices[key]; ok && yyRoomValidCandidate(choice.Candidate) && !p.Force && p.Op != "select" {
		yyRooms.Unlock()
		writeJSON(w, yyRoomResult{OK: true, Selected: &choice.Candidate, Remembered: true})
		return
	}
	if p.Op == "select" {
		search, ok := yyRooms.Searches[key]
		if !ok || time.Since(search.At) > 5*time.Minute {
			yyRooms.Unlock()
			writeJSON(w, map[string]any{"ok": false, "error": "候选已过期，请重新查询"})
			return
		}
		for _, v := range search.Result.Candidates {
			if v.ID == p.ID {
				e := yyRoomRemember(key, p.Input, v)
				yyRooms.Unlock()
				if e != nil {
					writeJSON(w, map[string]any{"ok": false, "error": "无法保存房间选择"})
					return
				}
				writeJSON(w, yyRoomResult{OK: true, Selected: &v})
				return
			}
		}
		yyRooms.Unlock()
		writeJSON(w, map[string]any{"ok": false, "error": "所选候选不属于本次查询"})
		return
	}
	yyRooms.Unlock()
	result := yyRoomDiscoverRemote(r, p.Token, auth, p.Input)
	if result.OK {
		yyRooms.Lock()
		for k, v := range yyRooms.Searches {
			if time.Since(v.At) > 5*time.Minute {
				delete(yyRooms.Searches, k)
			}
		}
		if len(yyRooms.Searches) >= 64 {
			for k := range yyRooms.Searches {
				delete(yyRooms.Searches, k)
				break
			}
		}
		yyRooms.Searches[key] = yyRoomSearch{Result: result, At: time.Now()}
		yyRooms.Unlock()
	}
	if result.Error == "" && !result.OK {
		result.Error = fmt.Sprint("房间查询失败")
	}
	writeJSON(w, result)
}
