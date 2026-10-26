package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type yyRuntimeChoice struct {
	ID      string `json:"id"`
	Account string `json:"account"`
}

var yyRuntimeChoices = struct {
	sync.Mutex
	Path  string
	Items map[string]yyRuntimeChoice
}{Items: map[string]yyRuntimeChoice{}}

func yyRememberRuntime(id, account string, enabled bool) error {
	yyRuntimeChoices.Lock()
	defer yyRuntimeChoices.Unlock()
	key := account + ":" + id
	if enabled {
		yyRuntimeChoices.Items[key] = yyRuntimeChoice{id, account}
	} else {
		delete(yyRuntimeChoices.Items, key)
	}
	if yyRuntimeChoices.Path == "" {
		return nil
	}
	data, e := json.Marshal(yyRuntimeChoices.Items)
	if e != nil {
		return e
	}
	if e = os.WriteFile(yyRuntimeChoices.Path+".tmp", data, 0600); e != nil {
		return e
	}
	return os.Rename(yyRuntimeChoices.Path+".tmp", yyRuntimeChoices.Path)
}
func yyRestoreRuntimeLoop(mux *http.ServeMux, dataDir string) {
	yyRuntimeChoices.Path = filepath.Join(dataDir, "resident-runtime-enabled.json")
	if data, e := os.ReadFile(yyRuntimeChoices.Path); e == nil {
		_ = json.Unmarshal(data, &yyRuntimeChoices.Items)
	}
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		lastError := map[string]string{}
		for range ticker.C {
			yyRuntimeChoices.Lock()
			items := make(map[string]yyRuntimeChoice, len(yyRuntimeChoices.Items))
			for key, p := range yyRuntimeChoices.Items {
				items[key] = p
			}
			yyRuntimeChoices.Unlock()
			for key, p := range items {
				yyRuntimeMu.Lock()
				_, running := yyRuntimeInstances[key]
				yyRuntimeMu.Unlock()
				if running {
					continue
				}
				// Serialize against manual stop; a stopped choice must never be resurrected.
				yyRuntimeStartMu.Lock()
				yyRuntimeChoices.Lock()
				_, wanted := yyRuntimeChoices.Items[key]
				yyRuntimeChoices.Unlock()
				yyRuntimeStartMu.Unlock()
				if !wanted {
					continue
				}
				_, data, e := yyInvoke("POST", "/api/youyuan/runtime/start", nil, map[string]any{"id": p.ID, "account": p.Account, "restore": true})
				var result map[string]any
				_ = json.Unmarshal(data, &result)
				if e != nil || result["ok"] == false {
					message := string(data)
					if e != nil {
						message = e.Error()
					}
					if lastError[key] != message {
						yyEvent("plugin_restore_error", map[string]any{"instance": key, "error": message})
						lastError[key] = message
					}
				} else {
					delete(lastError, key)
					yyEvent("plugin_restored", map[string]any{"instance": key})
				}
			}
		}
	}()
}
