package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

type yyPacketFilter struct {
	Fields  map[string]any `json:"fields"`
	EveryMS int            `json:"every_ms"`
}
type yyCaptureConfig struct {
	Types      []string       `json:"types"`
	Exclude    []string       `json:"exclude"`
	Filter     yyPacketFilter `json:"filter"`
	Capacity   int            `json:"capacity"`
	MaxBytes   int            `json:"max_bytes"`
	TotalBytes int            `json:"total_bytes"`
}
type yyCaptureState struct {
	Active                     bool
	Config                     yyCaptureConfig
	Records                    []map[string]any
	Seq                        uint64
	Dropped                    uint64
	SampledOut                 uint64
	Last                       map[string]time.Time
	Head, Count, RetainedBytes int
}

func yyPacketName(pk packet.Packet) string {
	t := reflect.TypeOf(pk)
	if t == nil {
		return ""
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
func yyPacketSchema() []map[string]any {
	rows := []map[string]any{}
	for id, newPacket := range packet.ListAllPackets() {
		pk := newPacket()
		t := reflect.TypeOf(pk).Elem()
		fields := []map[string]string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.IsExported() {
				fields = append(fields, map[string]string{"name": snakeCase(f.Name), "type": f.Type.String()})
			}
		}
		rows = append(rows, map[string]any{"name": t.Name(), "id": id, "fields": fields})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["name"].(string) < rows[j]["name"].(string) })
	return rows
}
func yyPacketTypeKnown(name string) bool {
	if name == "*" {
		return true
	}
	for _, f := range packet.ListAllPackets() {
		if yyPacketName(f()) == name {
			return true
		}
	}
	return false
}
func yyPacketPrivate(name string) bool {
	return name == "Login" || name == "ClientToServerHandshake" || name == "ServerToClientHandshake"
}
func yyPacketBulk(name string) bool {
	return name == "LevelChunk" || name == "SubChunk" || name == "ResourcePackChunkData"
}
func yyCaptureTypeMatch(c yyCaptureConfig, name string) bool {
	if yyPacketPrivate(name) {
		return false
	}
	for _, v := range c.Exclude {
		if v == name || v == "*" {
			return false
		}
	}
	for _, v := range c.Types {
		if v == name {
			return true
		}
		if v == "*" && !yyPacketBulk(name) {
			return true
		}
	}
	return false
}
func yyValidateFilter(f yyPacketFilter) error {
	if f.EveryMS < 0 || f.EveryMS > 60000 {
		return fmt.Errorf("every_ms 应为 0..60000")
	}
	if len(f.Fields) > 16 {
		return fmt.Errorf("最多 16 个字段过滤")
	}
	for k := range f.Fields {
		if k == "" || len(k) > 128 {
			return fmt.Errorf("字段路径无效")
		}
	}
	return nil
}
func yyFilterMatch(f yyPacketFilter, data map[string]any) bool {
	for key, want := range f.Fields {
		var value any = data
		for _, part := range strings.Split(key, ".") {
			m, ok := value.(map[string]any)
			if !ok {
				return false
			}
			var exists bool
			value, exists = m[part]
			if !exists {
				return false
			}
		}
		a, e := json.Marshal(value)
		b, e2 := json.Marshal(want)
		if e != nil || e2 != nil || string(a) != string(b) {
			return false
		}
	}
	return true
}
func yyCaptureStart(bm *BotManager, c yyCaptureConfig) (map[string]any, error) {
	if len(c.Types) == 0 {
		c.Types = []string{"UpdateAttributes", "ActorEvent", "Animate", "MovePlayer", "UpdateBlock", "Respawn"}
	}
	if len(c.Types) > len(packet.ListAllPackets())+1 || len(c.Exclude) > len(packet.ListAllPackets())+1 {
		return nil, fmt.Errorf("类型过滤不能超过协议目录数量")
	}
	for _, v := range append(append([]string{}, c.Types...), c.Exclude...) {
		if !yyPacketTypeKnown(v) {
			return nil, fmt.Errorf("未知包类型 %s，请查询 packets/schema", v)
		}
		if yyPacketPrivate(v) {
			return nil, fmt.Errorf("认证握手包不进入抓包记录")
		}
	}
	if e := yyValidateFilter(c.Filter); e != nil {
		return nil, e
	}
	if c.Capacity == 0 {
		c.Capacity = 256
	}
	if c.Capacity < 1 || c.Capacity > 65536 {
		return nil, fmt.Errorf("容量应为 1..65536")
	}
	if c.MaxBytes == 0 {
		c.MaxBytes = 8192
	}
	if c.MaxBytes < 512 || c.MaxBytes > 65536 {
		return nil, fmt.Errorf("单条大小应为 512..65536 字节")
	}
	if c.TotalBytes == 0 {
		c.TotalBytes = 16 << 20
	}
	if c.TotalBytes < c.MaxBytes+1024 || c.TotalBytes > 256<<20 {
		return nil, fmt.Errorf("total_bytes需至少max_bytes+1024且不超过256MiB；这是序列化留存预算，非Go堆内存上限")
	}
	s := yySenseFor(bm)
	s.Lock()
	s.Capture = yyCaptureState{Active: true, Config: c, Records: make([]map[string]any, c.Capacity), Last: map[string]time.Time{}}
	s.Unlock()
	return yyCaptureStatus(bm), nil
}
func yyCaptureWants(bm *BotManager, name string) bool {
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	return s.Capture.Active && yyCaptureTypeMatch(s.Capture.Config, name)
}
func yyCapturePacket(bm *BotManager, pk packet.Packet) {
	server := ""
	if bm != nil {
		bm.mu.Lock()
		server = bm.serverCode
		bm.mu.Unlock()
	}
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	c := &s.Capture
	name := yyPacketName(pk)
	if !c.Active || !yyCaptureTypeMatch(c.Config, name) {
		return
	}
	now := time.Now()
	if now.Sub(c.Last[name]) < time.Duration(c.Config.Filter.EveryMS)*time.Millisecond {
		c.SampledOut++
		return
	}
	data := packetToMap(pk)
	if !yyFilterMatch(c.Config.Filter, data) {
		return
	}
	c.Last[name] = now
	encoded, e := json.Marshal(data)
	if e != nil {
		return
	}
	var payload any = data
	truncated := len(encoded) > c.Config.MaxBytes
	if truncated {
		payload = map[string]any{"preview": string(encoded[:c.Config.MaxBytes]), "original_bytes": len(encoded)}
	}
	c.Seq++
	row := map[string]any{"sequence": c.Seq, "time": now.Format(time.RFC3339Nano), "direction": "server_to_bot", "server": server, "type": name, "packet_id": pk.ID(), "bytes": len(encoded), "truncated": truncated, "data": payload}
	retained := len(encoded) + 512
	if truncated {
		retained = c.Config.MaxBytes + 512
	}
	row["retained_bytes"] = retained
	for c.Count > 0 && (c.Count >= c.Config.Capacity || c.RetainedBytes+retained > c.Config.TotalBytes) {
		c.evict()
	}
	if len(c.Records) != c.Config.Capacity {
		c.Records = make([]map[string]any, c.Config.Capacity)
		c.Count = 0
		c.Head = 0
		c.RetainedBytes = 0
	}
	c.Records[(c.Head+c.Count)%len(c.Records)] = row
	c.Count++
	c.RetainedBytes += retained
}
func (c *yyCaptureState) evict() {
	if c.Count == 0 {
		return
	}
	if row := c.Records[c.Head]; row != nil {
		if n, ok := row["retained_bytes"].(int); ok {
			c.RetainedBytes -= n
		}
	}
	c.Records[c.Head] = nil
	c.Head = (c.Head + 1) % len(c.Records)
	c.Count--
	c.Dropped++
}
func (c *yyCaptureState) clear() {
	c.Records = make([]map[string]any, c.Config.Capacity)
	c.Head = 0
	c.Count = 0
	c.RetainedBytes = 0
	c.Dropped = 0
}
func yyCaptureRead(bm *BotManager, after uint64, limit int) map[string]any {
	connected := bm != nil && bm.IsConnected()
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	c := &s.Capture
	rows := []map[string]any{}
	if limit < 1 || limit > 65536 {
		limit = 256
	}
	for i := 0; i < c.Count; i++ {
		r := c.Records[(c.Head+i)%len(c.Records)]
		if r["sequence"].(uint64) > after {
			rows = append(rows, r)
			if len(rows) >= limit {
				break
			}
		}
	}
	return map[string]any{"ok": true, "active": c.Active, "connected": connected, "waiting_for_connection": c.Active && !connected, "config": c.Config, "cursor": c.Seq, "retained_count": c.Count, "retained_serialized_bytes_estimate": c.RetainedBytes, "records": yyObservationCopyRows(rows), "overwritten": c.Dropped, "sampled_out": c.SampledOut, "scope": "decoded incoming game packets for this account; not device-wide TCP/TLS capture"}
}

// Control operations return metadata only. Packet payloads can be megabytes;
// they belong to explicit cursor reads, not every start/stop tool response.
func yyCaptureStatus(bm *BotManager) map[string]any { return yyCaptureRead(bm, ^uint64(0), 1) }
func yySenseRoutes(mux *http.ServeMux, dataDir string) {
	yyLoadObservationSettings(dataDir)
	mux.HandleFunc("/api/youyuan/game/call", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Account string `json:"account"`
			Method  string `json:"method"`
			Args    []any  `json:"args"`
		}
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		bm := yyAccountBot(p.Account)
		if p.Account != "" && p.Account != "main" && bm == nil {
			writeJSON(w, map[string]any{"ok": false, "error": "账号不存在"})
			return
		}
		p.Method = yyCanonicalGameMethod(p.Method)
		if _, ok := yyExtraMethods[p.Method]; !ok && !strings.HasPrefix(p.Method, "game.") {
			writeJSON(w, map[string]any{"ok": false, "error": "API未注册", "code": "not_registered", "method": p.Method, "catalog": "/api/youyuan/sdk/catalog"})
			return
		}
		if _, ok := yySenseEvents[p.Method]; ok || p.Method == "events.subscribeFiltered" {
			writeJSON(w, map[string]any{"ok": false, "error": "事件订阅需要常驻插件回调；公网使用抓包/事件游标读取"})
			return
		}
		v, e := yyPublicGameCall(bm, p.Method, p.Args)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "result": v})
	})
	mux.HandleFunc("/api/youyuan/sense/snapshot", func(w http.ResponseWriter, r *http.Request) {
		account := r.URL.Query().Get("account")
		bm := yyAccountBot(account)
		if account != "" && account != "main" && bm == nil {
			writeJSON(w, map[string]any{"ok": false, "error": "目标账号不存在"})
			return
		}
		if r.URL.Query().Get("view") == "summary" {
			writeJSON(w, yyObservationSummary(bm, yySummaryFromQuery(r), false))
			return
		}
		writeJSON(w, map[string]any{"ok": true, "scene": yySenseSnapshot(bm)})
	})
	mux.HandleFunc("/api/youyuan/sense/events", func(w http.ResponseWriter, r *http.Request) {
		account := r.URL.Query().Get("account")
		bm := yyAccountBot(account)
		if account != "" && account != "main" && bm == nil {
			writeJSON(w, map[string]any{"ok": false, "error": "目标账号不存在"})
			return
		}
		if r.URL.Query().Get("view") == "summary" {
			writeJSON(w, yyObservationSummary(bm, yySummaryFromQuery(r), false))
			return
		}
		s := yySenseFor(bm)
		s.Lock()
		rows := append([]map[string]any{}, s.Events...)
		seq := s.Seq
		s.Unlock()
		writeJSON(w, map[string]any{"ok": true, "events": rows, "cursor": seq})
	})
	mux.HandleFunc("/api/youyuan/packets/schema", func(w http.ResponseWriter, r *http.Request) {
		if name := r.URL.Query().Get("name"); name != "" {
			rows := []map[string]any{}
			for _, row := range yyPacketSchema() {
				if row["name"] == name {
					rows = append(rows, row)
				}
			}
			writeJSON(w, map[string]any{"ok": true, "packets": rows})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "packets": yyPacketSchema()})
	})
	mux.HandleFunc("/api/youyuan/packets/capture", func(w http.ResponseWriter, r *http.Request) {
		account := r.URL.Query().Get("account")
		bm := yyAccountBot(account)
		if account != "" && account != "main" && bm == nil {
			writeJSON(w, map[string]any{"ok": false, "error": "目标账号不存在"})
			return
		}
		if r.Method == "GET" {
			if r.URL.Query().Get("view") == "summary" {
				writeJSON(w, yyObservationSummary(bm, yySummaryFromQuery(r), true))
				return
			}
			writeJSON(w, yyCaptureRead(bm, 0, 256))
			return
		}
		var p struct {
			Action string          `json:"action"`
			Config yyCaptureConfig `json:"config"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		switch p.Action {
		case "start":
			v, e := yyCaptureStart(bm, p.Config)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			} else {
				writeJSON(w, v)
			}
		case "stop", "clear":
			s := yySenseFor(bm)
			s.Lock()
			if p.Action == "stop" {
				s.Capture.Active = false
			} else {
				s.Capture.clear()
			}
			s.Unlock()
			writeJSON(w, yyCaptureStatus(bm))
		default:
			writeJSON(w, map[string]any{"ok": false, "error": "action 应为 start/stop/clear"})
		}
	})
	mux.HandleFunc("/api/youyuan/packets/export", func(w http.ResponseWriter, r *http.Request) {
		account := r.URL.Query().Get("account")
		bm := yyAccountBot(account)
		if account != "" && account != "main" && bm == nil {
			writeJSON(w, map[string]any{"ok": false, "error": "目标账号不存在"})
			return
		}
		v := yyCaptureRead(bm, 0, 65536)
		if r.Method == "POST" {
			path := filepath.Join(dataDir, "packet_exports", "youyuan-packets.ndjson")
			if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			file, e := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			enc := json.NewEncoder(file)
			for _, row := range v["records"].([]map[string]any) {
				if e = enc.Encode(row); e != nil {
					break
				}
			}
			closeErr := file.Close()
			if e != nil || closeErr != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "抓包导出写入失败"})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "path": path, "filename": "youyuan-packets.ndjson"})
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=youyuan-packets.ndjson")
		enc := json.NewEncoder(w)
		for _, row := range v["records"].([]map[string]any) {
			_ = enc.Encode(row)
		}
	})
}

// Pending main-account settings survive creation/replacement of its connection object.
// Fleet accounts do not inherit the main account's pending capture.
func yyCaptureTransfer(old, next *BotManager) {
	if old == next {
		return
	}
	from := yySenseFor(old)
	from.Lock()
	copyState := from.Capture
	copyState.Records = nil // new connection must not erase the old ring through shared backing arrays.
	from.Capture.Active = false
	from.Unlock()
	if !copyState.Active {
		return
	}
	copyState.clear()
	copyState.Seq = 0
	copyState.Dropped = 0
	copyState.SampledOut = 0
	copyState.Last = map[string]time.Time{}
	to := yySenseFor(next)
	to.Lock()
	to.Capture = copyState
	to.Unlock()
}
