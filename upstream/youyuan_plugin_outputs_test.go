package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginGeneratedOutputsPreserveBinaryAndRejectOtherAccounts(t *testing.T) {
	dir := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(dir, "")
	id := "output_test"
	os.MkdirAll(filepath.Join(dir, "插件文件", id), 0700)
	root := pc.DataDir(id)
	os.MkdirAll(filepath.Join(root, "建筑"), 0700)
	raw := []byte{0, 1, 255, 0, 42, 10}
	os.WriteFile(filepath.Join(root, "建筑", "附魔.nbt"), raw, 0600)
	old := pluginController
	pluginController = pc
	defer func() { pluginController = old }()
	yyIdentity.Lock()
	saved := yyIdentity.Data.User
	yyIdentity.Data.User = map[string]any{"id": "alice"}
	yyIdentity.Unlock()
	defer func() { yyIdentity.Lock(); yyIdentity.Data.User = saved; yyIdentity.Unlock() }()
	mux := http.NewServeMux()
	yyPluginOutputRoutes(mux)
	owner := yyPoolActor{ID: "alice", Scope: "control"}
	q := url.Values{"id": {id}, "file": {"建筑/附魔.nbt"}}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, exportReq("GET", "/api/youyuan/plugin-outputs/download?"+q.Encode(), "", owner))
	if w.Code != 200 || string(w.Body.Bytes()) != string(raw) || w.Header().Get("Content-Disposition") == "" {
		t.Fatalf("binary download corrupt %d %q", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, exportReq("GET", "/api/youyuan/plugin-outputs?id="+id, "", owner))
	var v struct {
		OK    bool             `json:"ok"`
		Items []map[string]any `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &v)
	if !v.OK || len(v.Items) != 1 || v.Items[0]["path"] != "建筑/附魔.nbt" {
		t.Fatalf("not the generated files %s", w.Body.String())
	}
	for _, f := range []string{"../secret", "/etc/passwd", "C:/secret", "."} {
		q.Set("file", f)
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, exportReq("GET", "/api/youyuan/plugin-outputs/download?"+q.Encode(), "", owner))
		if w.Code == 200 {
			t.Fatalf("escape allowed %s", f)
		}
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, exportReq("GET", "/api/youyuan/plugin-outputs?id="+id, "", yyPoolActor{ID: "bob", Scope: "control"}))
	if w.Code != 403 {
		t.Fatal("other user could read main outputs")
	}
}

func TestResourceOutputAPIUsesPluginDataAndRuntimeAccount(t *testing.T) {
	dir := t.TempDir()
	roots := yyResourceRoots{Code: filepath.Join(dir, "插件"), Data: filepath.Join(dir, "data", "secondary"), Kind: "runtime"}
	os.MkdirAll(roots.Data, 0700)
	os.WriteFile(filepath.Join(roots.Data, "export.zip"), []byte("PK\x00\xff"), 0600)
	v, e := yyResourceCall(roots, "plugin.exportFile", []any{"export.zip"})
	if e != nil {
		t.Fatal(e)
	}
	info := v.(map[string]any)
	u, _ := url.Parse(info["download_url"].(string))
	if u.Query().Get("kind") != "runtime" || u.Query().Get("runtime_account") != "secondary" {
		t.Fatal(info)
	}
	list, e := yyResourceCall(roots, "plugin.listOutputs", nil)
	if e != nil || len(list.([]map[string]any)) != 1 {
		t.Fatalf("%v %v", list, e)
	}
	if _, e = yyResourceCall(roots, "plugin.exportFile", []any{"../elsewhere"}); e == nil {
		t.Fatal("escaped plugin data")
	}
}

func TestGeneratedOutputSkillAndCatalogAreReadable(t *testing.T) {
	for _, lang := range []string{"lua-runtime", "python-runtime", "cpp-runtime"} {
		if _, e := yyDocSection(lang, "plugin-outputs.md"); e != nil {
			t.Fatalf("%s missing skill: %v", lang, e)
		}
		text, e := yyDocSection(lang, "plugin.md")
		if e != nil || !strings.Contains(string(text), "### `plugin.exportFile`") {
			t.Fatalf("%s missing method docs", lang)
		}
	}
	data, e := aiSkillsFS.ReadFile("skills/runtime-reference/catalog.json")
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		Methods []struct {
			Method string `json:"method"`
		} `json:"methods"`
	}
	if e = json.Unmarshal(data, &v); e != nil {
		t.Fatal(e)
	}
	found := map[string]bool{}
	for _, m := range v.Methods {
		found[m.Method] = true
	}
	for _, method := range []string{"plugin.exportFile", "plugin.listOutputs"} {
		if !found[method] {
			t.Fatalf("catalog missing %s", method)
		}
	}
}

func TestExportChildUsesAssignedOwnerWithoutSeparatePlatformLogin(t *testing.T) {
	savedRoot, savedWorker := yyAccounts.Root, yyAccounts.Worker
	yyAccounts.Root = t.TempDir()
	yyAccounts.Worker = true
	yyIdentity.Lock()
	savedUser := yyIdentity.Data.User
	yyIdentity.Data.User = nil
	yyIdentity.Unlock()
	defer func() {
		yyAccounts.Root = savedRoot
		yyAccounts.Worker = savedWorker
		yyIdentity.Lock()
		yyIdentity.Data.User = savedUser
		yyIdentity.Unlock()
	}()
	data, _ := json.Marshal(yyPoolChildIdentity{Source: "alice_bot", Owner: "alice", Secret: "local-test-key"})
	os.WriteFile(filepath.Join(yyAccounts.Root, "pool-parent.json"), data, 0600)
	actor, e := yyExportActor(httptest.NewRequest("GET", "/api/youyuan/plugin-outputs?id=demo", nil), false)
	if e != nil || actor.ID != "alice" || actor.Admin {
		t.Fatalf("lost child ownership: %+v %v", actor, e)
	}
	r := httptest.NewRequest("GET", "/api/youyuan/plugin-outputs", nil)
	r.Header.Set("X-YouYuan-Actor", "forged")
	if _, e = yyExportActor(r, false); e == nil {
		t.Fatal("forged actor must not fall back to child owner")
	}
}
