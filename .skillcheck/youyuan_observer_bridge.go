package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var yyObserverBridgeMethods = map[string]string{
	"observerBridge.create":  "options={name,source:player_input|game_server,server,player_id?}；创建独立采集密钥（仅返回一次）。客户端数据不冒充服务端确认；接入不自动获得山头服模块权限。",
	"observerBridge.sources": "无参数。采集来源、绑定服务器/玩家、最后序号/时间；不返回密钥。",
	"observerBridge.revoke":  "source_id；撤销采集密钥。",
	"observerBridge.events":  "after=0,limit=256；读取events/cursor/overwritten，保存cursor。event.data为封装，动作字段在event.data.data。swing_input:Animate挥臂或PlayerAuthInput.missed_swing空挥；attack_input攻击提交；position_input位置/视角；health_observation本人服务器血量。只覆盖已注册且实际接入来源，不覆盖机器人权威缓存；重启清空历史。详细技能player-actions。",
}

type yyObserverSource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Source   string `json:"source"`
	Server   string `json:"server"`
	Player   string `json:"player_id"`
	KeyHash  string `json:"key_hash"`
	Sequence uint64 `json:"sequence"`
	LastSeen int64  `json:"last_seen_at"`
}
type yyObserverBridge struct {
	sync.Mutex
	LocalPublish sync.Mutex
	Path         string
	Sources      map[string]yyObserverSource
	Events       []map[string]any
	Cursor       uint64
	Overwritten  uint64
}

var yyObserver = &yyObserverBridge{Sources: map[string]yyObserverSource{}}

func init() {
	for method, doc := range yyObserverBridgeMethods {
		yyExtraMethods[method] = doc
	}
}
func (b *yyObserverBridge) saveLocked() error {
	if b.Path == "" {
		return nil
	}
	data, err := json.Marshal(b.Sources)
	if err != nil {
		return err
	}
	if err = os.WriteFile(b.Path+".tmp", data, 0600); err != nil {
		return err
	}
	return os.Rename(b.Path+".tmp", b.Path)
}
func (b *yyObserverBridge) create(options map[string]any) (map[string]any, error) {
	get := func(key string) string { v, _ := options[key].(string); return v }
	kind, server, name := get("source"), get("server"), get("name")
	if (kind != "player_input" && kind != "game_server") || server == "" || name == "" || len(name) > 256 || len(server) > 256 || len(get("player_id")) > 256 {
		return nil, fmt.Errorf("需name、server及source=player_input/game_server；文本长度最多256字节")
	}
	if kind == "player_input" && get("player_id") == "" {
		return nil, fmt.Errorf("玩家输入桥必须绑定player_id")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	id := hex.EncodeToString(hash[:12])
	source := yyObserverSource{ID: id, Name: name, Source: kind, Server: server, Player: get("player_id"), KeyHash: hex.EncodeToString(hash[:])}
	b.Lock()
	defer b.Unlock()
	if b.Sources == nil {
		b.Sources = map[string]yyObserverSource{}
	}
	b.Sources[id] = source
	if err := b.saveLocked(); err != nil {
		delete(b.Sources, id)
		return nil, err
	}
	return map[string]any{"id": id, "token": token, "source": kind, "server": server, "player_id": source.Player, "ingest_path": "/api/youyuan/observer/ingest", "protocol": 1}, nil
}

type yyObserverBatch struct {
	ID       string           `json:"source_id"`
	Sequence uint64           `json:"sequence"`
	Events   []map[string]any `json:"events"`
}

func (b *yyObserverBridge) ingest(token string, batch yyObserverBatch) (map[string]any, error) {
	if len(token) != 64 {
		return nil, fmt.Errorf("采集密钥无效")
	}
	if batch.Sequence == 0 || batch.Sequence >= 1<<53 || len(batch.Events) == 0 || len(batch.Events) > 2048 {
		return nil, fmt.Errorf("sequence需1..2^53-1；每批1..2048项，分批是采集传输预算，不限制游戏命令")
	}
	// Validate before modifying sequence, preserving retries for rejected batches.
	for _, e := range batch.Events {
		kind, _ := e["kind"].(string)
		switch kind {
		case "swing_input", "attack_input", "position_input", "health_observation", "server_event":
		default:
			return nil, fmt.Errorf("未定义采集事件kind：%q", kind)
		}
		data, err := json.Marshal(e)
		if err != nil || len(data) > 8192 {
			return nil, fmt.Errorf("单条采集摘要需合法JSON且不超过8192字节；完整包请使用原始抓包")
		}
	}
	hash := sha256.Sum256([]byte(token))
	supplied := hex.EncodeToString(hash[:])
	b.Lock()
	defer b.Unlock()
	source, ok := b.Sources[batch.ID]
	if !ok || subtle.ConstantTimeCompare([]byte(supplied), []byte(source.KeyHash)) != 1 {
		return nil, fmt.Errorf("采集密钥无效或已撤销")
	}
	if batch.Sequence <= source.Sequence {
		return map[string]any{"accepted": 0, "duplicate": true, "cursor": b.Cursor}, nil
	}
	previous := source
	source.Sequence = batch.Sequence
	source.LastSeen = time.Now().UnixMilli()
	b.Sources[source.ID] = source
	if err := b.saveLocked(); err != nil {
		b.Sources[source.ID] = previous
		return nil, err
	}
	for i, e := range batch.Events {
		// Copy on receipt: external payload may never replace provenance or identity.
		encoded, _ := json.Marshal(e)
		var data map[string]any
		_ = json.Unmarshal(encoded, &data)
		b.Cursor++
		row := map[string]any{"cursor": b.Cursor, "event_id": source.ID + ":" + strconv.FormatUint(batch.Sequence, 10) + ":" + strconv.Itoa(i), "kind": e["kind"], "source_id": source.ID, "source": source.Source, "server": source.Server, "player_id": source.Player, "received_at": source.LastSeen, "data": data, "server_confirmed": false, "evidence": "reported_by_registered_bridge"}
		b.Events = append(b.Events, row)
	}
	limit := yyObservationSettings().EventHistory
	if len(b.Events) > limit {
		n := len(b.Events) - limit
		for i := 0; i < n; i++ {
			b.Events[i] = nil
		}
		b.Events = b.Events[n:]
		b.Overwritten += uint64(n)
	}
	return map[string]any{"accepted": len(batch.Events), "duplicate": false, "cursor": b.Cursor}, nil
}
func (b *yyObserverBridge) call(method string, args []any) (any, error) {
	if method == "observerBridge.create" {
		if len(args) != 1 {
			return nil, fmt.Errorf("create(options)")
		}
		m, ok := args[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("需要配置对象")
		}
		return b.create(m)
	}
	b.Lock()
	defer b.Unlock()
	switch method {
	case "observerBridge.sources":
		rows := []map[string]any{}
		for _, s := range b.Sources {
			rows = append(rows, map[string]any{"id": s.ID, "name": s.Name, "source": s.Source, "server": s.Server, "player_id": s.Player, "sequence": s.Sequence, "last_seen_at": s.LastSeen})
		}
		return map[string]any{"sources": rows, "events_retained": len(b.Events), "cursor": b.Cursor, "overwritten": b.Overwritten}, nil
	case "observerBridge.revoke":
		if len(args) != 1 {
			return nil, fmt.Errorf("revoke(source_id)")
		}
		id, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("source_id需字符串")
		}
		s, exists := b.Sources[id]
		if !exists {
			return map[string]any{"revoked": false}, nil
		}
		delete(b.Sources, id)
		if err := b.saveLocked(); err != nil {
			b.Sources[id] = s
			return nil, err
		}
		return map[string]any{"revoked": true}, nil
	case "observerBridge.events":
		after, limit := uint64(0), 256
		if len(args) > 2 {
			return nil, fmt.Errorf("events(after=0,limit=256)")
		}
		if len(args) > 0 {
			n, e := yyNBTBoundedInteger(args[0], 0, 1<<53-1)
			if e != nil {
				return nil, e
			}
			after = uint64(n)
		}
		if len(args) > 1 {
			n, e := yyNBTBoundedInteger(args[1], 1, 65536)
			if e != nil {
				return nil, e
			}
			limit = int(n)
		}
		rows := []map[string]any{}
		cursor := after
		for _, row := range b.Events {
			if row["cursor"].(uint64) > after {
				rows = append(rows, row)
				cursor = row["cursor"].(uint64)
				if len(rows) >= limit {
					break
				}
			}
		}
		// JSON-copy results so an API caller cannot mutate stored event history.
		encoded, _ := json.Marshal(rows)
		var copyRows []map[string]any
		_ = json.Unmarshal(encoded, &copyRows)
		return map[string]any{"events": copyRows, "cursor": cursor, "head_cursor": b.Cursor, "overwritten": b.Overwritten, "history": "memory; resets on host restart", "coverage": "participating registered sources only; not automatic global observation"}, nil
	}
	return nil, fmt.Errorf("未知采集桥API")
}
func yyObserverBridgeRoutes(mux *http.ServeMux, root string) {
	yyObserver.Lock()
	yyObserver.Path = filepath.Join(root, "observer-sources.json")
	if raw, err := os.ReadFile(yyObserver.Path); err == nil {
		var rows map[string]yyObserverSource
		if json.Unmarshal(raw, &rows) == nil && rows != nil {
			yyObserver.Sources = rows
		}
	}
	yyObserver.Unlock()
	mux.HandleFunc("/api/youyuan/observer/ingest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			http.Error(w, "POST required", 405)
			return
		}
		var batch yyObserverBatch
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&batch); err != nil {
			http.Error(w, "无效采集请求", 400)
			return
		}
		result, err := yyObserver.ingest(r.Header.Get("X-Youyuan-Observer-Token"), batch)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "result": result})
	})
}

func yyObserverValidToken(source yyObserverSource, token string) bool {
	if len(token) != 64 {
		return false
	}
	hash := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(hash[:])), []byte(source.KeyHash)) == 1
}

func (b *yyObserverBridge) publishLocal(token, id string, events []map[string]any) (map[string]any, error) {
	b.LocalPublish.Lock()
	defer b.LocalPublish.Unlock()
	b.Lock()
	seq := b.Sources[id].Sequence + 1
	b.Unlock()
	return b.ingest(token, yyObserverBatch{ID: id, Sequence: seq, Events: events})
}
