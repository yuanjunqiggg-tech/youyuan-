package main

import (
	"archive/zip"
	"bot-apk/config"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRobotCooperationGuidesIncludedInSelectableLanguageSkills(t *testing.T) {
	for _, language := range []string{"lua-runtime", "python-runtime", "cpp-runtime"} {
		data, err := yySkillArchive(language)
		if err != nil {
			t.Fatal(language, err)
		}
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, f := range archive.File {
			for _, chapter := range []string{"account-pool-guide.md", "observation-cooperation.md", "realtime-queries.md"} {
				if f.Name == language+"/"+chapter {
					r, err := f.Open()
					if err != nil {
						t.Fatal(err)
					}
					text, err := io.ReadAll(r)
					r.Close()
					if err != nil || (!bytes.Contains(text, []byte("accountsPool.handoff")) && !bytes.Contains(text, []byte("world.containerState"))) {
						t.Fatal(language, chapter, err)
					}
					found[chapter] = true
				}
			}
		}
		if len(found) != 3 {
			t.Fatal("可选技能缺少账号调用/协作章节", language, found)
		}
	}
}

func TestSkillLibraryPlayerChoicePersistenceAndIsolation(t *testing.T) {
	oldRoot := yyAgentRoot
	yyAgentRoot = t.TempDir()
	defer func() { yyAgentRoot = oldRoot }()
	a := &yyWorkspace{ID: "test_skill_choice_a", Language: "python", Skills: []string{}}
	b := &yyWorkspace{ID: "test_skill_choice_b", Language: "lua", Skills: []string{}}
	os.MkdirAll(yyAgentRoot+"/"+a.ID, 0700)
	os.MkdirAll(yyAgentRoot+"/"+b.ID, 0700)
	yyWorkspaces.Lock()
	yyWorkspaces.items[a.ID] = a
	yyWorkspaces.items[b.ID] = b
	yyWorkspaces.Unlock()
	defer func() {
		yyWorkspaces.Lock()
		delete(yyWorkspaces.items, a.ID)
		delete(yyWorkspaces.items, b.ID)
		yyWorkspaces.Unlock()
	}()
	mux := http.NewServeMux()
	yySkillLibraryRoutes(mux)
	toggle := func(skill string, on bool) {
		body, _ := json.Marshal(map[string]any{"id": a.ID, "skill": skill, "enabled": on})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/youyuan/agent/skills", bytes.NewReader(body)))
		var r map[string]any
		if json.Unmarshal(w.Body.Bytes(), &r) != nil || r["ok"] != true {
			t.Fatal(w.Body.String())
		}
	}
	if len(yyEffectiveAgentSkills(a)) != 0 {
		t.Fatal("new workspaces load unsolicited skills")
	}
	_, bad := yyWorkspaceTool(context.Background(), a, "load_skill", map[string]any{"skill": "mcbe-execute"})
	if !bad {
		t.Fatal("AI read unloaded skill")
	}
	toggle("mcbe-execute", true)
	got, bad := yyWorkspaceTool(context.Background(), a, "load_skill", map[string]any{"skill": "mcbe-execute", "max_chars": 100})
	if bad || len([]rune(got)) > 600 || !strings.Contains(got, "execute") {
		t.Fatal("paged read failed", got)
	}
	if len(yyEffectiveAgentSkills(b)) != 0 {
		t.Fatal("workspace selection leaked")
	}
	toggle("mcbe-execute", false)
	if len(yyEffectiveAgentSkills(a)) != 0 {
		t.Fatal("unload was overwritten")
	}
	data, e := os.ReadFile(yyAgentRoot + "/" + a.ID + "/task.json")
	if e != nil {
		t.Fatal(e)
	}
	var reload yyWorkspace
	if json.Unmarshal(data, &reload) != nil || reload.Skills == nil || len(yyEffectiveAgentSkills(&reload)) != 0 {
		t.Fatal("empty choice lost across save/reload")
	}
}

func TestBeta3SkillSectionsAndPortableArchives(t *testing.T) {
	for _, id := range []string{"mcbe-commands", "mcbe-execute", "mcbe-rawtext", "mcbe-systems", "mcbe-netease", "mcbe-particles"} {
		sk := skillByID(id)
		if sk == nil {
			t.Fatal(id)
		}
		for _, p := range sk.Sources {
			if b, e := aiSkillsFS.ReadFile(p); e != nil || len(b) < 100 {
				t.Fatal(p, e)
			}
		}
		b, e := yySkillArchive(id)
		if e != nil {
			t.Fatal(e)
		}
		z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if e != nil {
			t.Fatal(e)
		}
		seen := map[string]bool{}
		for _, f := range z.File {
			if strings.Contains(f.Name, "..") || seen[f.Name] {
				t.Fatal(f.Name)
			}
			seen[f.Name] = true
		}
		if !seen[id+"/SKILL.md"] || !seen[id+"/library.json"] {
			t.Fatal("missing portable library entry", id)
		}
	}
	first, e := yyMCPReadResource("youyuan://docs/mcbe-execute?section=01-execute.md&max_chars=80")
	if e != nil || !strings.Contains(first["text"].(string), "offset=80") {
		t.Fatal(first, e)
	}
	if _, e = yyMCPReadResource("youyuan://docs/mcbe-execute?section=../../private/token.json"); e == nil {
		t.Fatal("read arbitrary section")
	}
}

func TestMusicUsesMarketProviderAndScopedCredentials(t *testing.T) {
	config.InitConfig(t.TempDir())
	seen := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		w.WriteHeader(418)
		io.WriteString(w, "market-response")
	}))
	defer server.Close()
	cfg := &config.AppConfig{AuthURL: "https://auth.youyuanqi.dpdns.org", Token: "own-game-test-token", MarketURL: server.URL, MarketToken: "market-test-token"}
	if e := config.SaveConfig(cfg); e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "/api/music-proxy/api/music/list", nil)
	r.Header.Set("Authorization", "Bearer local-cloud-test-token")
	w := httptest.NewRecorder()
	handleMusicProxy(w, r)
	if w.Code != 418 || w.Body.String() != "market-response" || seen != "Bearer market-test-token" {
		t.Fatal("wrong music routing, credentials or HTTP status", w.Code, seen)
	}
	cfg.MarketToken = ""
	if got := yyMarketCredential(cfg, r); got != "" {
		t.Fatal("own game/cloud credential leaked", got)
	}
	cfg.AuthURL = server.URL + "/"
	if got := yyMarketCredential(cfg, r); got != "Bearer own-game-test-token" {
		t.Fatal("matching provider not automatically bound", got)
	}
	for _, u := range []string{"auto", "", "https://user:pass@example.com", "https://example.com?q=x"} {
		if _, e := yyProviderURL(u); e == nil {
			t.Fatal("invalid provider URL accepted", u)
		}
	}
}
