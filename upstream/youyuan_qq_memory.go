package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RAG is opt-in. The embedding model references an existing aiStore profile;
// credentials remain in the existing private model store, never in this index.
type yyQQMemoryConfig struct {
	Enabled        bool   `json:"enabled"`
	EmbeddingModel string `json:"embedding_model,omitempty"`
	TTLHours       int    `json:"ttl_hours,omitempty"`
	ContextChars   int    `json:"context_chars,omitempty"`
}
type yyQQMemoryRecord struct {
	Message yyQQGroupMessage `json:"message"`
	Vector  []float32        `json:"vector,omitempty"`
	Model   string           `json:"embedding_fingerprint,omitempty"`
}

var yyQQMemory = struct {
	sync.Mutex
	Path      string
	Groups    map[string][]yyQQMemoryRecord
	Epoch     map[string]uint64
	Status    map[string]string
	DiskError string
}{Groups: map[string][]yyQQMemoryRecord{}, Epoch: map[string]uint64{}, Status: map[string]string{}}
var yyQQEmbeddingSlots = make(chan struct{}, 2)

func yyQQMemoryMaintenance() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		yyQQExpireMemory(yyQQSnapshot(), now)
	}
}
func yyQQExpireMemory(cf yyQQConfig, now time.Time) {
	cutoff := now.Add(-yyQQMemoryTTL(cf))
	yyQQMessages.Lock()
	for scope, rows := range yyQQMessages.Groups {
		yyQQMessages.Groups[scope] = yyQQBoundMessages(cf, rows, now)
		if len(yyQQMessages.Groups[scope]) == 0 {
			delete(yyQQMessages.Groups, scope)
		}
	}
	yyQQMessages.Unlock()
	yyQQAIHistory.Lock()
	for session, at := range yyQQAIHistory.Last {
		if at.Before(cutoff) && !yyQQAIHistory.Busy[session] {
			delete(yyQQAIHistory.Items, session)
			delete(yyQQAIHistory.Last, session)
		}
	}
	yyQQAIHistory.Unlock()
	yyQQMemory.Lock()
	defer yyQQMemory.Unlock()
	dirty := false
	for scope, rows := range yyQQMemory.Groups {
		kept := make([]yyQQMemoryRecord, 0, len(rows))
		for _, row := range rows {
			if !row.Message.Timestamp.Before(cutoff) {
				kept = append(kept, row)
			}
		}
		if len(kept) != len(rows) {
			dirty = true
			yyQQMemory.Groups[scope] = kept
		}
		if len(kept) == 0 {
			delete(yyQQMemory.Groups, scope)
			delete(yyQQMemory.Status, scope)
		}
	}
	if dirty {
		yyQQMemorySaveLocked()
	}
}

func yyQQMemoryTTL(cf yyQQConfig) time.Duration {
	h := cf.Assistant.Memory.TTLHours
	if h < 1 {
		h = 24
	}
	if h > 720 {
		h = 720
	}
	return time.Duration(h) * time.Hour
}
func yyQQContextBudget(cf yyQQConfig) int {
	n := cf.Assistant.Memory.ContextChars
	if n < 1000 {
		n = 6000
	}
	if n > 16000 {
		n = 16000
	}
	return n
}
func yyQQEmbeddingProfile(cf yyQQConfig) (*AIModel, string) {
	if !cf.Assistant.Memory.Enabled || cf.Assistant.Memory.EmbeddingModel == "" {
		return nil, ""
	}
	m := aiStore.Get(cf.Assistant.Memory.EmbeddingModel)
	if m == nil || m.Provider != ProviderOpenAI || strings.TrimSpace(m.BaseURL) == "" || strings.TrimSpace(m.ModelID) == "" {
		return nil, ""
	}
	u, err := url.Parse(m.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, ""
	}
	ip := net.ParseIP(u.Hostname())
	if m.APIKey == "" && u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, ""
	}
	return m, fmt.Sprintf("%x", sha256.Sum256([]byte(m.BaseURL+"\x00"+m.ModelID)))
}
func yyQQEmbedding(ctx context.Context, m *AIModel, texts []string) ([][]float32, error) {
	if m == nil || len(texts) < 1 || len(texts) > 16 {
		return nil, fmt.Errorf("embedding_not_configured_or_invalid_batch")
	}
	endpoint := strings.TrimRight(m.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/embeddings") {
		if strings.HasSuffix(endpoint, "/v1") {
			endpoint += "/embeddings"
		} else {
			endpoint += "/v1/embeddings"
		}
	}
	body, _ := json.Marshal(map[string]any{"model": m.ModelID, "input": texts, "encoding_format": "float"})
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if e != nil {
		return nil, fmt.Errorf("invalid_embedding_endpoint")
	}
	req.Header.Set("Content-Type", "application/json")
	if m.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.APIKey)
	}
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Do(req)
	if e != nil {
		return nil, fmt.Errorf("embedding_transport_failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("embedding_http_%d", resp.StatusCode)
	}
	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result) != nil || len(result.Data) != len(texts) {
		return nil, fmt.Errorf("invalid_embedding_response")
	}
	vectors := make([][]float32, len(texts))
	dims := 0
	for _, row := range result.Data {
		if row.Index < 0 || row.Index >= len(texts) || vectors[row.Index] != nil || len(row.Embedding) < 1 || len(row.Embedding) > 2048 {
			return nil, fmt.Errorf("invalid_embedding_dimensions_or_index")
		}
		if dims != 0 && dims != len(row.Embedding) {
			return nil, fmt.Errorf("embedding_dimensions_changed")
		}
		dims = len(row.Embedding)
		norm := float64(0)
		for _, v := range row.Embedding {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, fmt.Errorf("invalid_embedding_value")
			}
			norm += float64(v) * float64(v)
		}
		if norm == 0 {
			return nil, fmt.Errorf("zero_embedding_vector")
		}
		vectors[row.Index] = row.Embedding
	}
	return vectors, nil
}
func yyQQMemorySaveLocked() {
	if yyQQMemory.Path == "" {
		return
	}
	raw, e := json.Marshal(yyQQMemory.Groups)
	if e == nil && len(raw) > 24<<20 {
		e = fmt.Errorf("memory_index_size_limit")
	}
	if e == nil {
		e = os.WriteFile(yyQQMemory.Path+".tmp", raw, 0600)
	}
	if e == nil {
		e = os.Rename(yyQQMemory.Path+".tmp", yyQQMemory.Path)
	}
	if e != nil {
		yyQQMemory.DiskError = "memory_persistence_failed"
	} else {
		yyQQMemory.DiskError = ""
	}
}

// Prune on access and on ingestion; expired text/vectors are also removed from disk.
func yyQQMemoryPruneLocked(cf yyQQConfig) {
	scope := yyQQMessageKey(cf)
	rows := yyQQMemory.Groups[scope]
	kept := make([]yyQQMemoryRecord, 0, len(rows))
	for _, row := range rows {
		if !row.Message.Timestamp.Before(time.Now().Add(-yyQQMemoryTTL(cf))) {
			kept = append(kept, row)
		}
	}
	if len(kept) != len(rows) {
		yyQQMemory.Groups[scope] = kept
		yyQQMemorySaveLocked()
	}
}
func yyQQMemoryInit(root string) {
	yyQQMemory.Lock()
	defer yyQQMemory.Unlock()
	yyQQMemory.Path = filepath.Join(root, "qq-group-memory.json")
	file, e := os.Open(yyQQMemory.Path)
	if e != nil {
		return
	}
	defer file.Close()
	var groups map[string][]yyQQMemoryRecord
	if json.NewDecoder(io.LimitReader(file, 24<<20)).Decode(&groups) != nil || len(groups) > 32 {
		yyQQMemory.DiskError = "invalid_or_oversized_memory_index"
		return
	}
	now := time.Now()
	cf := yyQQSnapshot()
	for key, rows := range groups {
		if len(key) != 64 || len(rows) > 160 {
			delete(groups, key)
			continue
		}
		valid := []yyQQMemoryRecord{}
		for _, r := range rows {
			if r.Message.Timestamp.Before(now.Add(-yyQQMemoryTTL(cf))) || r.Message.Timestamp.After(now.Add(time.Minute)) || len(r.Vector) > 2048 {
				continue
			}
			r.Message.Text = yyQQClip(r.Message.Text, 700)
			valid = append(valid, r)
		}
		groups[key] = valid
	}
	yyQQMemory.Groups = groups
	yyQQMemorySaveLocked()
}
func yyQQMemoryRemember(cf yyQQConfig, msg yyQQGroupMessage) {
	if !cf.Assistant.Memory.Enabled || msg.User == "" || msg.IsBot {
		return
	}
	scope := yyQQMessageKey(cf)
	msg.Text = yyQQClip(msg.Text, 700)
	yyQQMemory.Lock()
	if _, ok := yyQQMemory.Groups[scope]; !ok && len(yyQQMemory.Groups) >= 32 {
		oldest := ""
		var at time.Time
		for k, rows := range yyQQMemory.Groups {
			if len(rows) == 0 || oldest == "" || rows[len(rows)-1].Message.Timestamp.Before(at) {
				oldest = k
				if len(rows) > 0 {
					at = rows[len(rows)-1].Message.Timestamp
				}
			}
		}
		delete(yyQQMemory.Groups, oldest)
		delete(yyQQMemory.Status, oldest)
	}
	rows := yyQQMemory.Groups[scope]
	kept := []yyQQMemoryRecord{}
	for _, r := range rows {
		if !r.Message.Timestamp.Before(time.Now().Add(-yyQQMemoryTTL(cf))) {
			if msg.ID != "" && r.Message.ID == msg.ID {
				yyQQMemory.Unlock()
				return
			}
			kept = append(kept, r)
		}
	}
	kept = append(kept, yyQQMemoryRecord{Message: msg})
	if len(kept) > 160 {
		kept = kept[len(kept)-160:]
	}
	yyQQMemory.Groups[scope] = kept
	epoch := yyQQMemory.Epoch[scope]
	yyQQMemorySaveLocked()
	yyQQMemory.Unlock()
	m, fingerprint := yyQQEmbeddingProfile(cf)
	if m == nil {
		yyQQMemory.Lock()
		yyQQMemory.Status[scope] = "lexical_fallback_embedding_not_configured"
		yyQQMemory.Unlock()
		return
	}
	select {
	case yyQQEmbeddingSlots <- struct{}{}:
		go func() {
			defer func() { <-yyQQEmbeddingSlots }()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			yyQQMemoryIndex(ctx, cf, msg, epoch, m, fingerprint)
		}()
	default:
		yyQQMemory.Lock()
		yyQQMemory.Status[scope] = "lexical_fallback_embedding_busy"
		yyQQMemory.Unlock()
	}
}
func yyQQMemoryIndex(ctx context.Context, cf yyQQConfig, msg yyQQGroupMessage, epoch uint64, m *AIModel, fingerprint string) {
	scope := yyQQMessageKey(cf)
	yyQQMemory.Lock()
	current := yyQQMemory.Epoch[scope] == epoch
	found := false
	for _, row := range yyQQMemory.Groups[scope] {
		if row.Message.ID == msg.ID && row.Message.User == msg.User && row.Message.Timestamp.Equal(msg.Timestamp) {
			found = true
			break
		}
	}
	yyQQMemory.Unlock()
	if !current || !found {
		return
	}
	vectors, e := yyQQEmbedding(ctx, m, []string{msg.Text})
	yyQQMemory.Lock()
	defer yyQQMemory.Unlock()
	if yyQQMemory.Epoch[scope] != epoch {
		return
	}
	if e != nil {
		yyQQMemory.Status[scope] = "lexical_fallback_" + e.Error()
		return
	}
	for i, r := range yyQQMemory.Groups[scope] {
		if r.Message.ID == msg.ID && r.Message.User == msg.User && r.Message.Timestamp.Equal(msg.Timestamp) {
			yyQQMemory.Groups[scope][i].Vector = vectors[0]
			yyQQMemory.Groups[scope][i].Model = fingerprint
			yyQQMemory.Status[scope] = "vector_indexed"
			yyQQMemorySaveLocked()
			return
		}
	}
}
func yyQQCosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return -1
	}
	dot, aa, bb := 0.0, 0.0, 0.0
	for i, v := range a {
		x, y := float64(v), float64(b[i])
		dot += x * y
		aa += x * x
		bb += y * y
	}
	if aa == 0 || bb == 0 {
		return -1
	}
	return dot / math.Sqrt(aa*bb)
}
func yyQQLexicalScore(query, text string) float64 {
	q := []rune(strings.ToLower(query))
	t := strings.ToLower(text)
	score := 0.0
	if len(q) > 160 {
		q = q[:160]
	}
	seen := map[string]bool{}
	for i, r := range q {
		if r <= ' ' {
			continue
		}
		term := string(r)
		if i+1 < len(q) {
			term = string(q[i : i+2])
		}
		if !seen[term] && strings.Contains(t, term) {
			score++
			seen[term] = true
		}
	}
	return score
}
func yyQQMemorySearch(ctx context.Context, cf yyQQConfig, query string, limit int) ([]yyQQGroupMessage, string) {
	return yyQQMemorySearchWithReply(ctx, cf, query, "", limit)
}
func yyQQMemorySearchWithReply(ctx context.Context, cf yyQQConfig, query, replyTo string, limit int) ([]yyQQGroupMessage, string) {
	if !cf.Assistant.Memory.Enabled {
		return nil, "disabled"
	}
	if !yyQQGroupEnabled(cf, yyQQGroupString(cf.GroupID)) {
		return nil, "group_disabled"
	}
	if limit < 1 || limit > 5 {
		limit = 5
	}
	scope := yyQQMessageKey(cf)
	yyQQMemory.Lock()
	yyQQMemoryPruneLocked(cf)
	epoch := yyQQMemory.Epoch[scope]
	rows := append([]yyQQMemoryRecord(nil), yyQQMemory.Groups[scope]...)
	yyQQMemory.Unlock()
	if len(rows) == 0 {
		return nil, "empty"
	}
	mode := "lexical_fallback_embedding_not_configured"
	var vector []float32
	m, fingerprint := yyQQEmbeddingProfile(cf)
	if m != nil {
		select {
		case yyQQEmbeddingSlots <- struct{}{}:
			vectors, e := yyQQEmbedding(ctx, m, []string{yyQQClip(query, 2000)})
			<-yyQQEmbeddingSlots
			if e == nil {
				vector = vectors[0]
				mode = "vector_with_lexical_fallback_for_unindexed"
			} else {
				mode = "lexical_fallback_" + e.Error()
			}
		default:
			mode = "lexical_fallback_embedding_busy"
		}
	}
	type scored struct {
		message yyQQGroupMessage
		score   float64
	}
	hits := []scored{}
	cutoff := time.Now().Add(-yyQQMemoryTTL(cf))
	for _, r := range rows {
		if r.Message.Timestamp.Before(cutoff) {
			continue
		}
		score := yyQQLexicalScore(query, r.Message.Text) / 1000
		if len(vector) > 0 && r.Model == fingerprint && len(r.Vector) == len(vector) {
			score = yyQQCosine(vector, r.Vector)
		}
		if replyTo != "" && r.Message.ID == replyTo {
			score = 2
		} // Exact received reference outranks semantic guesses.
		if score > 0 {
			hits = append(hits, scored{r.Message, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score == hits[j].score {
			return hits[i].message.Timestamp.After(hits[j].message.Timestamp)
		}
		return hits[i].score > hits[j].score
	})
	result := []yyQQGroupMessage{}
	for i := 0; i < len(hits) && i < limit; i++ {
		result = append(result, hits[i].message)
	}
	yyQQMemory.Lock()
	defer yyQQMemory.Unlock()
	if yyQQMemory.Epoch[scope] != epoch {
		return nil, "cleared_during_search"
	}
	yyQQMemory.Status[scope] = mode
	return result, mode
}
func yyQQContextEpoch(cf yyQQConfig) uint64 {
	yyQQMessages.Lock()
	defer yyQQMessages.Unlock()
	return yyQQMessages.Epoch[yyQQMessageKey(cf)]
}
func yyQQMemoryClear(cf yyQQConfig, id string) {
	scope := yyQQMessageKey(cf)
	yyQQMessages.Lock()
	yyQQMessages.Epoch[scope]++
	if id == "" {
		delete(yyQQMessages.Groups, scope)
	} else {
		rows := []yyQQGroupMessage{}
		for _, r := range yyQQMessages.Groups[scope] {
			if r.ID != id {
				rows = append(rows, r)
			}
		}
		yyQQMessages.Groups[scope] = rows
	}
	yyQQMessages.Unlock()
	yyQQCompanions.Lock()
	yyQQCompanions.Epoch[scope]++
	for key := range yyQQCompanions.Sessions {
		if strings.HasPrefix(key, scope+"\x00") {
			delete(yyQQCompanions.Sessions, key)
		}
	}
	yyQQCompanions.Unlock()
	yyQQAIHistory.Lock()
	for key := range yyQQAIHistory.Items {
		if strings.HasPrefix(key, scope+"\x00") {
			delete(yyQQAIHistory.Items, key)
			delete(yyQQAIHistory.Last, key)
		}
	}
	yyQQAIHistory.Unlock()
	yyQQMemory.Lock()
	defer yyQQMemory.Unlock()
	yyQQMemory.Epoch[scope]++
	if id == "" {
		delete(yyQQMemory.Groups, scope)
	} else {
		rows := []yyQQMemoryRecord{}
		for _, r := range yyQQMemory.Groups[scope] {
			if r.Message.ID != id {
				rows = append(rows, r)
			}
		}
		yyQQMemory.Groups[scope] = rows
	}
	yyQQMemory.Status[scope] = "cleared"
	yyQQMemorySaveLocked()
}
func yyQQMemoryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/memory", func(w http.ResponseWriter, r *http.Request) {
		cf := yyQQSnapshot()
		if group := r.URL.Query().Get("group"); group != "" {
			cf.GroupID = group
		}
		if !yyQQGroupEnabled(cf, yyQQGroupString(cf.GroupID)) {
			writeJSON(w, map[string]any{"ok": false, "error": "这个群未启用"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			m, fingerprint := yyQQEmbeddingProfile(cf)
			yyQQMemory.Lock()
			yyQQMemoryPruneLocked(cf)
			rows := yyQQMemory.Groups[yyQQMessageKey(cf)]
			count, indexed := 0, 0
			for _, row := range rows {
				if time.Since(row.Message.Timestamp) <= yyQQMemoryTTL(cf) {
					count++
					if m != nil && len(row.Vector) > 0 && row.Model == fingerprint {
						indexed++
					}
				}
			}
			status, diskError := yyQQMemory.Status[yyQQMessageKey(cf)], yyQQMemory.DiskError
			yyQQMemory.Unlock()
			if !cf.Assistant.Memory.Enabled {
				status = "disabled"
			} else if m == nil {
				status = "lexical_fallback_embedding_not_configured"
			}
			writeJSON(w, map[string]any{"ok": true, "enabled": cf.Assistant.Memory.Enabled, "entries": count, "indexed_entries": indexed, "embedding_configured": m != nil, "status": status, "ttl_hours": int(yyQQMemoryTTL(cf) / time.Hour), "context_chars": yyQQContextBudget(cf), "persistence_error": diskError, "source": "received_events_only", "bot_identity": "智能助手，不能冒充真人"})
		case http.MethodDelete:
			yyQQMemoryClear(cf, r.URL.Query().Get("message_id"))
			yyQQMemory.Lock()
			err := yyQQMemory.DiskError
			yyQQMemory.Unlock()
			writeJSON(w, map[string]any{"ok": err == "", "error": err, "inflight_answers_invalidated": true})
		default:
			http.Error(w, "GET/DELETE required", http.StatusMethodNotAllowed)
		}
	})
}
