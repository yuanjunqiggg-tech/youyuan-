package main

import (
	"encoding/json"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentForkPreservesExplicitDisabledToolsAcrossReload(t *testing.T) {
	oldRoot, oldStore := yyAgentRoot, aiStore
	yyAgentRoot, aiStore = t.TempDir(), &aiModelStore{}
	defer func() { yyAgentRoot, aiStore = oldRoot, oldStore }()
	mux := http.NewServeMux()
	yyAgentForkRoutes(mux)
	for _, disabled := range []bool{false, true} {
		source := &yyWorkspace{ID: "fork_permission_probe", Language: "lua", Turns: []aiTurn{{Role: "user", Text: "continue"}}, Model: "not-configured"}
		if disabled {
			source.AllowedTools = []string{}
		}
		if err := os.MkdirAll(yyWorkspaceRoot(source), 0700); err != nil {
			t.Fatal(err)
		}
		yyWorkspaces.Lock()
		yyWorkspaces.items[source.ID] = source
		yyWorkspaces.Unlock()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/youyuan/agent/fork", strings.NewReader(`{"id":"fork_permission_probe","turn_index":0}`)))
		var result struct {
			OK bool   `json:"ok"`
			ID string `json:"id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.OK {
			t.Fatal(w.Body.String(), err)
		}
		raw, err := os.ReadFile(filepath.Join(yyAgentRoot, result.ID, "task.json"))
		if err != nil {
			t.Fatal(err)
		}
		var restored yyWorkspace
		if err := json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if disabled && (restored.AllowedTools == nil || yyAgentAllowed(&restored, "api_call")) {
			t.Fatal("fork reopened disabled tools")
		}
		if !disabled && !yyAgentAllowed(&restored, "api_call") {
			t.Fatal("default capability lost")
		}
		yyWorkspaces.Lock()
		delete(yyWorkspaces.items, result.ID)
		delete(yyWorkspaces.items, source.ID)
		yyWorkspaces.Unlock()
	}
}

func TestModelCapacitySurvivesAPIAndDisk(t *testing.T) {
	old := aiStore
	defer func() { aiStore = old }()
	dir := t.TempDir()
	aiStore = &aiModelStore{}
	aiStore.setPath(dir)
	body := `{"name":"probe","provider":"openai","base_url":"https://example.invalid","api_key":"private-probe","model_id":"model","context_window":64000,"max_output_tokens":4000,"compression_percent":75}`
	w := httptest.NewRecorder()
	handleAIModels(w, httptest.NewRequest("POST", "/api/ai/models", strings.NewReader(body)))
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatal(w.Body.String())
	}
	aiStore = &aiModelStore{}
	aiStore.setPath(dir)
	w = httptest.NewRecorder()
	handleAIModels(w, httptest.NewRequest("GET", "/api/ai/models", nil))
	for _, want := range []string{`"context_window":64000`, `"max_output_tokens":4000`, `"compression_percent":75`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatal("lost persisted setting", want, w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), "private-probe") {
		t.Fatal("unmasked secret")
	}
	// A real write failure must not report success or replace the in-memory model.
	before := aiStore.Get("probe")
	aiStore.path = filepath.Join(dir, "missing", "models.json")
	updated := *before
	updated.ContextWindow = 128000
	if aiStore.Add(&updated) == nil {
		t.Fatal("write failure hidden")
	}
	if aiStore.Get("probe").ContextWindow != 64000 {
		t.Fatal("failed save changed active config")
	}
	if aiStore.Delete("probe") == nil || aiStore.Get("probe") == nil {
		t.Fatal("failed delete changed config")
	}
}

func TestChatObserverStillReceivesWaitMsgInput(t *testing.T) {
	bm := &BotManager{}
	id := "wait_observer_probe"
	c := &captureHook{}
	old := getSendTargetedHook()
	setSendTargetedHook(func(p, s string) error { c.add(s); return nil })
	defer setSendTargetedHook(old)
	if err := LoadLuaPlugin(bm, id, `function observe(p,m) game.sendTargeted(p,"seen:"..m) end events.onChat("observe")`); err != nil {
		t.Fatal(err)
	}
	defer RemovePluginEvents(id)
	ch := make(chan string, 1)
	playerWaitMsgMu.Lock()
	playerWaitMsgCbs["WaitProbe"] = ch
	playerWaitMsgMu.Unlock()
	defer func() { playerWaitMsgMu.Lock(); delete(playerWaitMsgCbs, "WaitProbe"); playerWaitMsgMu.Unlock() }()
	bm.handleChat("WaitProbe", "reply")
	if !deliverWaitMsg("WaitProbe", "reply") {
		t.Fatal("wait session lost its input")
	}
	if <-ch != "reply" {
		t.Fatal("wrong input")
	}
	deadline := time.Now().Add(time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond * 5)
	}
	if c.count() != 1 {
		t.Fatal("onChat was suppressed", c.all())
	}
}

func TestPacketSubscriptionIgnoresOtherKernel(t *testing.T) {
	owner, other := &BotManager{}, &BotManager{}
	id := "packet_owner_probe"
	c := &captureHook{}
	old := getSendTargetedHook()
	setSendTargetedHook(func(p, s string) error { c.add(s); return nil })
	defer setSendTargetedHook(old)
	if err := LoadLuaPlugin(owner, id, `function observe(p) game.sendTargeted("probe","time:"..p.time) end events.subscribe("SetTime","observe")`); err != nil {
		t.Fatal(err)
	}
	defer RemovePluginEvents(id)
	packetHubDispatch(other, &packet.SetTime{Time: 111})
	packetHubDispatch(owner, &packet.SetTime{Time: 222})
	deadline := time.Now().Add(time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond * 5)
	}
	if got := c.all(); len(got) != 1 || got[0] != "time:222" {
		t.Fatal("cross-account event leak", got)
	}
}

func TestSkillSearchMultipleMethodNamesReturnsReadableOffset(t *testing.T) {
	result, err := yySearchWorkspaceSkills(&yyWorkspace{Language: "lua", Skills: []string{"lua-runtime"}}, "onBotReady onChat setScore sendTargeted")
	if err {
		t.Fatal(result)
	}
	var r struct {
		Matches []struct {
			Skill, Section string
			Offset         int
			Excerpt        string
		}
	}
	if e := json.Unmarshal([]byte(result), &r); e != nil || len(r.Matches) == 0 {
		t.Fatal("multi-method search returned nothing", result, e)
	}
	for _, m := range r.Matches {
		text, e := yyAgentSkillText(m.Skill, m.Section, m.Offset, 1200)
		if e != nil || !strings.HasPrefix(text, m.Excerpt) {
			t.Fatal("offset does not locate matched source", e)
		}
	}
}
