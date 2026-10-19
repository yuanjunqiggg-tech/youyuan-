package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type yyRuntimeBlockedWriter struct {
	header  http.Header
	entered chan struct{}
	release chan struct{}
}

func (w *yyRuntimeBlockedWriter) Header() http.Header { return w.header }
func (w *yyRuntimeBlockedWriter) WriteHeader(int)     {}
func (w *yyRuntimeBlockedWriter) Write(p []byte) (int, error) {
	close(w.entered)
	<-w.release
	return len(p), nil
}

func yyRuntimeContentionFixture(t testing.TB, installed int) (*BotManager, *yyRuntimeInstance, string) {
	t.Helper()
	root := t.TempDir()
	for n := 0; n < installed; n++ {
		id := fmt.Sprintf("plugin_%03d", n)
		dir := filepath.Join(root, id)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(yyRuntimeManifest{ID: id, Language: "python", Entry: "main.py", Name: "运行器并发基准"})
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	oldRoot, oldBot := yyRuntimeRoot, botMgr
	bm := &BotManager{}
	instance := &yyRuntimeInstance{Key: "main:contention", Account: "main", Manifest: yyRuntimeManifest{ID: "contention", Language: "python"}, Queue: make(chan map[string]any, 7)}
	yyRuntimeMu.Lock()
	oldInstances := yyRuntimeInstances
	yyRuntimeInstances = map[string]*yyRuntimeInstance{instance.Key: instance}
	yyRuntimeMu.Unlock()
	yyRuntimeRoot, botMgr = root, bm
	t.Cleanup(func() {
		yyRuntimeMu.Lock()
		yyRuntimeInstances = oldInstances
		yyRuntimeMu.Unlock()
		yyRuntimeRoot, botMgr = oldRoot, oldBot
	})
	return bm, instance, root
}

func TestRuntimeListSlowClientCannotBlockEventDelivery(t *testing.T) {
	bm, instance, _ := yyRuntimeContentionFixture(t, 3)
	writer := &yyRuntimeBlockedWriter{header: make(http.Header), entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		yyRuntimeListHandler(writer, httptest.NewRequest("GET", "/api/youyuan/runtime/list", nil))
		close(done)
	}()
	defer func() { close(writer.release); <-done }()
	select {
	case <-writer.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("list response never reached writer")
	}
	delivered := make(chan struct{})
	go func() {
		for n := 0; n < 1000; n++ {
			yyRuntimeEmit(bm, map[string]any{"type": "chat", "sequence": n})
			if event := <-instance.Queue; event["sequence"] != n {
				panic("event payload or order changed")
			}
		}
		close(delivered)
	}()
	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("slow list response blocked runtime event delivery")
	}
	if instance.Dropped.Load() != 0 {
		t.Fatal("event lost")
	}
}

func TestRuntimeListPreservesCatalogCountersAndQueueCapacity(t *testing.T) {
	_, instance, root := yyRuntimeContentionFixture(t, 3)
	instance.Processed.Store(12)
	instance.Failures.Store(2)
	instance.Dropped.Store(1)
	instance.LastError.Store("visible error")
	instance.Queue <- map[string]any{"type": "chat"}
	// Invalid manifests and loose files stay excluded, as before.
	if err := os.Mkdir(filepath.Join(root, "invalid"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "invalid", "manifest.json"), []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	yyRuntimeListHandler(response, httptest.NewRequest("GET", "/api/youyuan/runtime/list", nil))
	var body struct {
		OK        bool                `json:"ok"`
		Installed []yyRuntimeManifest `json:"installed"`
		Instances []map[string]any    `json:"instances"`
		Languages []string            `json:"languages"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || len(body.Installed) != 3 || len(body.Instances) != 1 || len(body.Languages) != 3 {
		t.Fatal(response.Body.String())
	}
	row := body.Instances[0]
	if row["queue_capacity"] != float64(7) || row["queue_depth"] != float64(1) || row["processed"] != float64(12) || row["failures"] != float64(2) || row["dropped"] != float64(1) || row["last_error"] != "visible error" {
		t.Fatal(row)
	}
}

// Same files, same successful event delivery and same listing work. The old
// lock scope is reproduced explicitly; no simulated delay, event sampling or
// callback deletion is used. ns/op measures the whole pair; event_wait_ns/op
// isolates the registry contention visible to the game receive goroutine.
func BenchmarkRuntimeListingEventContention(b *testing.B) {
	for _, installed := range []int{8, 64} {
		for _, legacy := range []bool{true, false} {
			label := "snapshot"
			if legacy {
				label = "legacy_lock_scope"
			}
			b.Run(fmt.Sprintf("plugins_%d/%s", installed, label), func(b *testing.B) {
				bm, instance, root := yyRuntimeContentionFixture(b, installed)
				b.ResetTimer()
				var wait time.Duration
				for n := 0; n < b.N; n++ {
					entered, done := make(chan struct{}), make(chan struct{})
					go func() {
						var live []map[string]any
						if legacy {
							yyRuntimeMu.Lock()
							live = yyRuntimeListSnapshotLocked()
						} else {
							live = yyRuntimeListSnapshot()
						}
						close(entered)
						catalog := yyRuntimeInstalled(root)
						if len(catalog) != installed {
							panic("catalog changed")
						}
						writeJSON(httptest.NewRecorder(), map[string]any{"ok": true, "installed": catalog, "instances": live})
						if legacy {
							yyRuntimeMu.Unlock()
						}
						close(done)
					}()
					<-entered
					started := time.Now()
					yyRuntimeEmit(bm, map[string]any{"type": "chat", "sequence": n})
					wait += time.Since(started)
					if event := <-instance.Queue; event["sequence"] != n {
						b.Fatal("event changed")
					}
					<-done
				}
				b.StopTimer()
				if instance.Dropped.Load() != 0 {
					b.Fatal("event lost")
				}
				b.ReportMetric(float64(wait.Nanoseconds())/float64(b.N), "event_wait_ns/op")
			})
		}
	}
}
