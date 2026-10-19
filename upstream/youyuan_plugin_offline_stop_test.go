package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOfflinePluginCanBeStopped(t *testing.T) {
	previousController := pluginController
	botMgrMu.Lock()
	previousBot := botMgr
	botMgr = NewBotManager()
	botMgrMu.Unlock()
	pluginController = NewPluginController()
	t.Cleanup(func() {
		pluginController = previousController
		botMgrMu.Lock()
		botMgr = previousBot
		botMgrMu.Unlock()
	})
	pluginController.running["offline-probe"] = true
	w := httptest.NewRecorder()
	handlePluginStop(w, httptest.NewRequest("POST", "/api/plugin/stop", strings.NewReader(`{"id":"offline-probe"}`)))
	var r map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r["ok"] != true || pluginController.running["offline-probe"] {
		t.Fatal(w.Body.String())
	}
}
