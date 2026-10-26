package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	QueueDropped               uint64
	SampledOut                 uint64
	Last                       map[string]time.Time
	Head, Count, RetainedBytes int
	// Serialization runs on a dedicated worker. Keeping packetToMap/JSON off the
	// single packet-consumer goroutine is what stops capture from throttling
	// every other packet path and eventually the RakNet receive window.
	gen      uint64
	stored   uint64
	inflight int
	pending  chan packet.Packet
	done     chan struct{}
}

// yyCaptureQueueSize is the burst buffer between the packet-consumer goroutine
// and the single serialization worker. 4096 covers a 3k-packet spike without
// dropping; yyCaptureEnqueueRetries then covers anything larger.
const yyCaptureQueueSize = 4096

// yyCaptureEnqueueRetries bounds how long a burst may yield before a packet is
// counted as dropped. The worker consumes O(1) per item under the lock, so a
// few hundred yields normally make room; the cap keeps the receive path from
// spinning forever if the worker stalls.
const yyCaptureEnqueueRetries = 512

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
	if s.Capture.done != nil {
		close(s.Capture.done)
	}
	gen := s.Capture.gen + 1
	next := yyCaptureState{Active: true, Config: c, Records: make([]map[string]any, c.Capacity), Last: map[string]time.Time{}, gen: gen, pending: make(chan packet.Packet, yyCaptureQueueSize), done: make(chan struct{})}
	s.Capture = next
	s.Unlock()
	if bm != nil {
		bm.captureMatcher.Store(yyCaptureBuildMatcher(c))
	}
	go yyCaptureDrain(bm, next.pending, next.done, gen)
	return yyCaptureStatus(bm), nil
}
// A published, immutable matcher keeps the receive thread off the sense mutex.
// The consumer holds that mutex while emitting events; blocking the receive
// thread there is what pushed the RakNet window over its limit under load.
type yyCaptureMatcher struct {
	types    map[string]bool
	exclude  map[string]bool
	wildcard bool
}

func yyCaptureBuildMatcher(c yyCaptureConfig) *yyCaptureMatcher {
	m := &yyCaptureMatcher{types: map[string]bool{}, exclude: map[string]bool{}}
	for _, v := range c.Types {
		if v == "*" {
			m.wildcard = true
			continue
		}
		m.types[v] = true
	}
	for _, v := range c.Exclude {
		m.exclude[v] = true
	}
	return m
}

func (m *yyCaptureMatcher) match(name string) bool {
	if m == nil || yyPacketPrivate(name) {
		return false
	}
	if m.exclude[name] || m.exclude["*"] {
		return false
	}
	if m.types[name] {
		return true
	}
	return m.wildcard && !yyPacketBulk(name)
}

func yyCaptureWants(bm *BotManager, name string) bool {
	if bm != nil {
		return bm.captureMatcher.Load().match(name)
	}
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	return s.Capture.Active && yyCaptureTypeMatch(s.Capture.Config, name)
}
// yyCapturePacket is the only capture work allowed on the packet-consumer
// goroutine: cheap filtering and a bounded, non-blocking hand-off.
func yyCapturePacket(bm *BotManager, pk packet.Packet) {
	if bm == nil {
		return
	}
	s := yySenseFor(bm)
	s.Lock()
	c := &s.Capture
	if !c.Active {
		s.Unlock()
		return
	}
	name := yyPacketName(pk)
	if !yyCaptureTypeMatch(c.Config, name) {
		s.Unlock()
		return
	}
	if every := c.Config.Filter.EveryMS; every > 0 {
		now := time.Now()
		if now.Sub(c.Last[name]) < time.Duration(every)*time.Millisecond {
			c.SampledOut++
			s.Unlock()
			return
		}
		c.Last[name] = now
	}
	if c.pending == nil {
		c.QueueDropped++
		s.Unlock()
		return
	}
	ch := c.pending
	// Never yield while holding the state lock: the store worker needs it.
	s.Unlock()
	select {
	case ch <- pk:
		return
	default:
	}
	// A bounded queue keeps memory flat, but a burst must not turn straight into
	// loss. Each store is O(1) under the lock now, so yielding a few times lets
	// the worker drain and usually makes room. Only after that do we count a drop.
	for i := 0; i < yyCaptureEnqueueRetries; i++ {
		runtime.Gosched()
		select {
		case ch <- pk:
			return
		default:
		}
	}
	s.Lock()
	if s.Capture.pending == ch {
		s.Capture.QueueDropped++
	}
	s.Unlock()
}

func yyCaptureDrain(bm *BotManager, ch chan packet.Packet, done chan struct{}, gen uint64) {
	for {
		select {
		case <-done:
			return
		case pk, ok := <-ch:
			if !ok {
				return
			}
			// Mark the item as taken but not yet written. Without this the read
			// path sees an empty queue and an unchanged stored counter while
			// serialization is still running, and wrongly reports "drained".
			yyCaptureInflight(bm, gen, 1)
			func() {
				// The decrement must survive any panic in serialization, or the
				// flush path would wait for a counter that never returns to zero.
				defer yyCaptureInflight(bm, gen, -1)
				yyCaptureStore(bm, pk, gen)
			}()
		}
	}
}

// yyCaptureInflight adjusts the in-progress counter only for the state that
// still owns this generation, so a restart cannot corrupt the new state.
func yyCaptureInflight(bm *BotManager, gen uint64, delta int) {
	s := yySenseFor(bm)
	s.Lock()
	if s.Capture.gen == gen {
		s.Capture.inflight += delta
	}
	s.Unlock()
}

func yyCaptureStore(bm *BotManager, pk packet.Packet, gen uint64) {
	server := ""
	if bm != nil {
		bm.mu.Lock()
		server = bm.serverCode
		bm.mu.Unlock()
	}
	s := yySenseFor(bm)
	// Phase 1: snapshot the config under the lock, then release it.
	// packetToMap and json.Marshal are the expensive part. Running them while
	// holding yySenseState starved the packet-consumer goroutine on the shared
	// lock, which is what filled the bounded queue and showed up as capture loss.
	s.Lock()
	live := &s.Capture
	if !live.Active || live.gen != gen {
		s.Unlock()
		return
	}
	cfg := live.Config
	s.Unlock()

	name := yyPacketName(pk)
	if !yyCaptureTypeMatch(cfg, name) {
		return
	}
	data := packetToMap(pk)
	if !yyFilterMatch(cfg.Filter, data) {
		return
	}
	now := time.Now()
	encoded, e := json.Marshal(data)
	if e != nil {
		return
	}
	var payload any = data
	truncated := len(encoded) > cfg.MaxBytes
	if truncated {
		payload = map[string]any{"preview": string(encoded[:cfg.MaxBytes]), "original_bytes": len(encoded)}
	}
	retained := len(encoded) + 512
	if truncated {
		retained = cfg.MaxBytes + 512
	}
	row := map[string]any{
		"sequence":       0,
		"time":           now.Format(time.RFC3339Nano),
		"direction":      "server_to_bot",
		"server":         server,
		"type":           name,
		"packet_id":      pk.ID(),
		"bytes":          len(encoded),
		"truncated":      truncated,
		"data":           payload,
		"retained_bytes": retained,
	}

	// Phase 2: O(1) ring write under the lock. Sequence is assigned here so the
	// chronology reflects store order even though serialization ran unlocked.
	s.Lock()
	defer s.Unlock()
	c := &s.Capture
	if !c.Active || c.gen != gen {
		return
	}
	c.Seq++
	row["sequence"] = c.Seq
	for c.Count > 0 && (c.Count >= cfg.Capacity || c.RetainedBytes+retained > cfg.TotalBytes) {
		c.evict()
	}
	if len(c.Records) != cfg.Capacity {
		c.Records = make([]map[string]any, cfg.Capacity)
		c.Count = 0
		c.Head = 0
		c.RetainedBytes = 0
	}
	c.Records[(c.Head+c.Count)%len(c.Records)] = row
	c.Count++
	c.RetainedBytes += retained
	c.stored++
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
	c.QueueDropped = 0
	c.inflight = 0
	c.SampledOut = 0
}
// yyCaptureFlush waits (bounded) for the serialization worker to consume what is
// already queued, so a diagnostic read reflects everything accepted so far.
// It never runs on the packet-consumer goroutine.
func yyCaptureFlush(bm *BotManager, timeout time.Duration) {
	s := yySenseFor(bm)
	deadline := time.Now().Add(timeout)
	last := uint64(1 << 63)
	for {
		s.Lock()
		pending, stored, inflight := len(s.Capture.pending), s.Capture.stored, s.Capture.inflight
		s.Unlock()
		if pending == 0 && inflight == 0 && stored == last {
			return
		}
		if time.Now().After(deadline) {
			return
		}
		last = stored
		time.Sleep(time.Millisecond)
	}
}

func yyCaptureRead(bm *BotManager, after uint64, limit int) map[string]any {
	yyCaptureFlush(bm, 250*time.Millisecond)
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
	return map[string]any{"ok": true, "active": c.Active, "connected": connected, "waiting_for_connection": c.Active && !connected, "config": c.Config, "cursor": c.Seq, "retained_count": c.Count, "retained_serialized_bytes_estimate": c.RetainedBytes, "queue_depth": len(c.pending), "queue_capacity": cap(c.pending), "queue_inflight": c.inflight, "queue_dropped": c.QueueDropped, "records": yyObservationCopyRows(rows), "overwritten": c.Dropped, "sampled_out": c.SampledOut, "scope": "decoded incoming game packets for this account; not device-wide TCP/TLS capture"}
}

// Control operations return metadata only. Packet payloads can be megabytes;
// they belong to explicit cursor reads, not every start/stop tool response.
func yyCaptureStatus(bm *BotManager) map[string]any { return yyCaptureRead(bm, ^uint64(0), 1) }
// yyCaptureStop is the single place that turns capture off, so the published
// matcher and the stored state cannot disagree.
func yyCaptureStop(bm *BotManager) {
	s := yySenseFor(bm)
	s.Lock()
	s.Capture.Active = false
	if bm != nil {
		bm.captureMatcher.Store(nil)
	}
	s.Unlock()
}

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
			if p.Action == "stop" {
				yyCaptureStop(bm)
			} else {
				s := yySenseFor(bm)
				s.Lock()
				s.Capture.clear()
				s.Unlock()
			}
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
	if from.Capture.done != nil {
		close(from.Capture.done)
	}
	from.Capture.Active = false
	from.Capture.pending = nil
	from.Capture.done = nil
	from.Unlock()
	if old != nil {
		old.captureMatcher.Store(nil)
	}
	if !copyState.Active {
		return
	}
	copyState.clear()
	copyState.Seq = 0
	copyState.Dropped = 0
	copyState.QueueDropped = 0
	copyState.SampledOut = 0
	copyState.Last = map[string]time.Time{}
	copyState.gen++
	copyState.pending = make(chan packet.Packet, yyCaptureQueueSize)
	copyState.done = make(chan struct{})
	to := yySenseFor(next)
	to.Lock()
	to.Capture = copyState
	to.Unlock()
	if next != nil {
		next.captureMatcher.Store(yyCaptureBuildMatcher(copyState.Config))
	}
	go yyCaptureDrain(next, copyState.pending, copyState.done, copyState.gen)
}
