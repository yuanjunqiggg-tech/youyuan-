package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// 语言由 manifest 指定。每个账号有独立实例；依赖目录保存在应用私有目录。
type yyRuntimeManifest struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Language     string            `json:"language"`
	Entry        string            `json:"entry"`
	Events       []string          `json:"events,omitempty"`
	Version      string            `json:"version,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}
type yyRuntimeInstance struct {
	mu        sync.Mutex
	Key       string
	Account   string
	Manifest  yyRuntimeManifest
	Native    uint64
	Counter   uint64
	Processed atomic.Uint64
	Failures  atomic.Uint64
	Dropped   atomic.Uint64
	Fatal     atomic.Bool
	LastError atomic.Value
	Stopped   bool
	Queue     chan map[string]any
	Done      chan struct{}
}

var yyRuntimeMu sync.Mutex
var yyRuntimeStartMu sync.Mutex
var yyRuntimeInstances = map[string]*yyRuntimeInstance{}
var yyRuntimeRoot string

func yyRuntimeRoutes(mux *http.ServeMux, dataDir string) {
	yySDKRoutes(mux)
	yyRuntimeManagementRoutes(mux)
	yyRuntimeRoot = filepath.Join(dataDir, "多语言插件")
	_ = os.MkdirAll(yyRuntimeRoot, 0755)
	yyArchiveBundledSamples(dataDir)
	mux.HandleFunc("/api/youyuan/runtime/import", yyRuntimeImport)
	mux.HandleFunc("/api/youyuan/runtime/export", yyRuntimeExport)
	mux.HandleFunc("/api/youyuan/account/action", yyAccountAction)
	mux.HandleFunc("/api/youyuan/library/list", yyLibraryProxy)
	mux.HandleFunc("/api/youyuan/library/upload", yyLibraryProxy)
	mux.HandleFunc("/api/youyuan/library/download", yyLibraryProxy)
	mux.HandleFunc("/api/youyuan/library/delete", yyLibraryProxy)
	mux.HandleFunc("/api/youyuan/runtime/list", yyRuntimeListHandler)
	mux.HandleFunc("/api/youyuan/runtime/start", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			ID      string `json:"id"`
			Account string `json:"account"`
			Restore bool   `json:"restore"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if p.Account == "" {
			p.Account = "main"
		}
		if filepath.Base(p.ID) != p.ID || p.ID == "." || p.ID == "" || len(p.Account) > 80 || strings.ContainsAny(p.Account, "/\\:") {
			writeJSON(w, map[string]any{"ok": false, "error": "插件标识无效"})
			return
		}
		manifest := yyRuntimeManifest{ID: p.ID, Language: "native_cpp", Entry: "libyy_counter.so", Name: "C++ 常驻计数器"}
		dir := filepath.Join(yyRuntimeRoot, p.ID)
		if p.ID != "native_counter" {
			data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
			if err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			if err = json.Unmarshal(data, &manifest); err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
		}
		if strings.ContainsAny(p.Account, "/\\:") || p.Account == "." || p.Account == ".." {
			writeJSON(w, map[string]any{"ok": false, "error": "账号标识不能包含路径字符"})
			return
		}
		if manifest.ID != p.ID || filepath.Base(manifest.Entry) != manifest.Entry {
			writeJSON(w, map[string]any{"ok": false, "error": "manifest 与插件目录不一致或入口路径无效"})
			return
		}
		yyRuntimeStartMu.Lock()
		defer yyRuntimeStartMu.Unlock()
		key := p.Account + ":" + p.ID
		if p.Restore {
			yyRuntimeChoices.Lock()
			_, wanted := yyRuntimeChoices.Items[key]
			yyRuntimeChoices.Unlock()
			if !wanted {
				writeJSON(w, map[string]any{"ok": false, "error": "插件已手动停用"})
				return
			}
		}
		yyRuntimeMu.Lock()
		_, exists := yyRuntimeInstances[key]
		yyRuntimeMu.Unlock()
		if exists {
			writeJSON(w, map[string]any{"ok": false, "error": "该账号的插件已经运行"})
			return
		}
		if e := yyRuntimeCheckDependencies(manifest, p.Account); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		instance := &yyRuntimeInstance{Key: key, Account: p.Account, Manifest: manifest, Queue: make(chan map[string]any, yyObservationSettings().RuntimeQueue), Done: make(chan struct{})}
		yySDKPrepare(instance, dir)
		var result string
		var err error
		switch manifest.Language {
		case "python":
			result, err = yyPythonRuntime("start", map[string]any{"key": key, "plugin_id": p.ID, "account": p.Account, "entry": filepath.Join(dir, manifest.Entry), "data_dir": filepath.Join(dir, "data", p.Account)})
		case "native_cpp":
			entry := manifest.Entry
			if p.ID != "native_counter" {
				entry = filepath.Join(dir, manifest.Entry)
				err = yyValidateNative(entry)
			}
			if err == nil {
				instance.Native, err = yyNativeStart(entry, p.Account, p.ID)
			}
		default:
			err = fmt.Errorf("当前内置运行时不支持 %s", manifest.Language)
		}
		if err == nil && result != "" {
			var response map[string]any
			_ = json.Unmarshal([]byte(result), &response)
			if response["ok"] == false {
				err = fmt.Errorf("%v %v", response["error"], response["errors"])
			}
		}
		if err != nil {
			yySDKClose(key)
			if manifest.Language == "python" {
				_, _ = yyPythonRuntime("stop", map[string]any{"key": key})
			}
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		yyRuntimeMu.Lock()
		yyRuntimeInstances[key] = instance
		yyRuntimeMu.Unlock()
		go instance.runEvents()
		if e := yyRememberRuntime(p.ID, p.Account, true); e != nil {
			yyEvent("plugin_persistence_error", e.Error())
		}
		writeJSON(w, map[string]any{"ok": true, "id": p.ID, "account": p.Account, "language": manifest.Language})
	})
	mux.HandleFunc("/api/youyuan/runtime/event", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			ID      string         `json:"id"`
			Account string         `json:"account"`
			Event   map[string]any `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if p.Account == "" {
			p.Account = "main"
		}
		yyRuntimeMu.Lock()
		instance := yyRuntimeInstances[p.Account+":"+p.ID]
		yyRuntimeMu.Unlock()
		if instance == nil {
			writeJSON(w, map[string]any{"ok": false, "error": "插件没有启动"})
			return
		}
		result, err := instance.event(p.Event)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(result))
	})
	mux.HandleFunc("/api/youyuan/runtime/stop", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			ID      string `json:"id"`
			Account string `json:"account"`
			Native  uint64 `json:"native_instance,omitempty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p.Account == "" {
			p.Account = "main"
		}
		yyRuntimeStartMu.Lock()
		defer yyRuntimeStartMu.Unlock()
		key := p.Account + ":" + p.ID
		yyRuntimeMu.Lock()
		current := yyRuntimeInstances[key]
		yyRuntimeMu.Unlock()
		if p.Native != 0 && (current == nil || current.Native != p.Native) {
			writeJSON(w, map[string]any{"ok": true, "skipped": "instance_replaced"})
			return
		}
		if e := yyRememberRuntime(p.ID, p.Account, false); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		yyRuntimeMu.Lock()
		instance := yyRuntimeInstances[key]
		delete(yyRuntimeInstances, key)
		yyRuntimeMu.Unlock()
		if instance != nil {
			instance.mu.Lock()
			instance.Stopped = true
			close(instance.Done)
			if instance.Manifest.Language == "python" {
				_, _ = yyPythonRuntime("stop", map[string]any{"key": key})
			} else {
				yyNativeStop(instance.Native)
			}
			instance.mu.Unlock()
			yySDKClose(key)
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	yyRestoreRuntimeLoop(mux, dataDir)
}

// Only copy live counters under the registry lock. Directory reads and response
// writes may block on storage or a slow client; neither may stall game event
// delivery or plugin start/stop, which share yyRuntimeMu.
func yyRuntimeListSnapshot() []map[string]any {
	yyRuntimeMu.Lock()
	defer yyRuntimeMu.Unlock()
	return yyRuntimeListSnapshotLocked()
}

func yyRuntimeListSnapshotLocked() []map[string]any {
	list := make([]map[string]any, 0, len(yyRuntimeInstances))
	for _, instance := range yyRuntimeInstances {
		last := instance.LastError.Load()
		if last == nil {
			last = ""
		}
		list = append(list, map[string]any{"id": instance.Manifest.ID, "language": instance.Manifest.Language, "account": instance.Account, "state": "running", "queue_depth": len(instance.Queue), "queue_capacity": cap(instance.Queue), "processed": instance.Processed.Load(), "failures": instance.Failures.Load(), "dropped": instance.Dropped.Load(), "last_error": last})
	}
	return list
}

func yyRuntimeInstalled(root string) []yyRuntimeManifest {
	installed := []yyRuntimeManifest{}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, e := os.ReadFile(filepath.Join(root, entry.Name(), "manifest.json"))
		var manifest yyRuntimeManifest
		if e == nil && json.Unmarshal(data, &manifest) == nil {
			installed = append(installed, manifest)
		}
	}
	return installed
}

func yyRuntimeListHandler(w http.ResponseWriter, r *http.Request) {
	list := yyRuntimeListSnapshot()
	installed := yyRuntimeInstalled(yyRuntimeRoot)
	writeJSON(w, map[string]any{"ok": true, "installed": installed, "instances": list, "languages": []string{"python", "native_cpp", "lua_compat"}, "dependency_dir": yyRuntimeRoot})
}

func (instance *yyRuntimeInstance) event(event map[string]any) (string, error) {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	return instance.eventLocked(event)
}

func (instance *yyRuntimeInstance) rpcEvent(event map[string]any) (string, error) {
	// Callers register a wait edge before entering. Genuine cycles are rejected
	// there; ordinary concurrent calls queue instead of spuriously failing.
	instance.mu.Lock()
	defer instance.mu.Unlock()
	return instance.eventLocked(event)
}

func (instance *yyRuntimeInstance) eventLocked(event map[string]any) (string, error) {
	if instance.Stopped {
		return "", fmt.Errorf("插件已经停止")
	}
	instance.Counter++
	instance.Processed.Add(1)
	if instance.Manifest.Language == "python" {
		text, err := yyPythonRuntime("event", map[string]any{"key": instance.Key, "event": event})
		instance.noteResult(text, err)
		return text, err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	text, err := yyNativeEvent(instance.Native, string(data))
	instance.noteResult(text, err)
	if err != nil && instance.Fatal.CompareAndSwap(false, true) {
		// Native transport errors mean the isolated worker has exited or timed
		// out. Stop through the normal lifecycle after releasing instance.mu.
		// This cancels subscriptions/timers and prevents ghost callbacks.
		go func() {
			_, _, stopErr := yyInvoke("POST", "/api/youyuan/runtime/stop", nil, map[string]any{"id": instance.Manifest.ID, "account": instance.Account, "native_instance": instance.Native})
			if stopErr != nil {
				yyEvent("native_stop_error", stopErr.Error())
			}
		}()
	}
	return text, err
}
func (instance *yyRuntimeInstance) noteResult(text string, err error) {
	var reply map[string]any
	if err == nil {
		_ = json.Unmarshal([]byte(text), &reply)
		if reply["ok"] == false {
			err = fmt.Errorf("%v", reply["error"])
		}
	}
	if err != nil {
		instance.Failures.Add(1)
		instance.LastError.Store(err.Error())
	}
}
func (instance *yyRuntimeInstance) runEvents() {
	for {
		select {
		case <-instance.Done:
			return
		case event := <-instance.Queue:
			completions := []chan struct{}{}
			stripCompletion := func(e map[string]any) {
				if done, ok := e["_timer_done"].(chan struct{}); ok {
					delete(e, "_timer_done")
					completions = append(completions, done)
				}
			}
			stripCompletion(event)
			var result string
			var err error
			if instance.Manifest.Language == "python" {
				batch := []map[string]any{event}
			drain:
				for len(batch) < 16 {
					select {
					case next := <-instance.Queue:
						stripCompletion(next)
						batch = append(batch, next)
					default:
						break drain
					}
				}
				if len(batch) == 1 {
					result, err = instance.event(event)
				} else {
					instance.mu.Lock()
					if instance.Stopped {
						err = fmt.Errorf("插件已经停止")
					} else {
						instance.Counter += uint64(len(batch))
						instance.Processed.Add(uint64(len(batch)))
						result, err = yyPythonRuntime("events", map[string]any{"key": instance.Key, "events": batch})
						instance.noteResult(result, err)
					}
					instance.mu.Unlock()
				}
			} else {
				result, err = instance.event(event)
			}
			var response map[string]any
			if err == nil {
				_ = json.Unmarshal([]byte(result), &response)
				if response["ok"] == false {
					err = fmt.Errorf("%v %v", response["error"], response["errors"])
				}
			}
			if err != nil {
				yyEvent("plugin_error", map[string]any{"instance": instance.Key, "error": err.Error()})
			}
			for _, done := range completions {
				close(done)
			}
		}
	}
}

// 接收线程只入队；每个插件实例顺序执行，慢插件不会阻塞游戏收包线程。
func yyRuntimeEmit(bm *BotManager, event map[string]any) {
	yySDKEmit(bm, event)
	yyRuntimeMu.Lock()
	defer yyRuntimeMu.Unlock()
	for _, instance := range yyRuntimeInstances {
		if yyAccountBot(instance.Account) != bm {
			continue
		}
		kind, _ := event["type"].(string)
		if len(instance.Manifest.Events) > 0 {
			found := false
			for _, allowed := range instance.Manifest.Events {
				if allowed == kind {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		} else if kind == "packet" {
			continue
		}
		select {
		case instance.Queue <- event:
		default:
			instance.Dropped.Add(1)
			yyEvent("plugin_error", map[string]any{"instance": instance.Key, "error": "事件队列已满，当前事件未投递"})
		}
	}
}
