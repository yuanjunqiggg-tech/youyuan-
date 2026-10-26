package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func exportReq(method, path, body string, actor yyPoolActor) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), yyPoolActorContext{}, actor))
}
func TestExportCenterCompleteSnapshotDownloadsAndTenantIsolation(t *testing.T) {
	dir := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(dir, "")
	id := "test_export"
	for name, value := range map[string]string{"插件文件/test_export/test_export.lua": "return {}", "插件文件/test_export/modules/b.lua": "return 7", "插件配置文件/test_export/config.json": "{\"saved\":true}", "插件数据文件/test_export/command_txt/链.txt": "say test"} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0700)
		if e := os.WriteFile(p, []byte(value), 0600); e != nil {
			t.Fatal(e)
		}
	}
	source, e := pc.ExportPackage(id)
	if e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	old := yyExportDirectory
	defer func() { yyExportDirectory = old }()
	yyExportCenterRoutes(mux, dir)
	owner := yyPoolActor{ID: "alice", Scope: "control"}
	other := yyPoolActor{ID: "bob", Scope: "control"}
	admin := yyPoolActor{ID: "root", Admin: true, Scope: "control"}
	v, e := yyRetainExport(yyExportDirectory, source, id, "lua", owner)
	if e != nil {
		t.Fatal(e)
	}
	// Later exports overwrite their staging file, not the immutable download snapshot.
	if e = os.WriteFile(source, []byte("new export"), 0600); e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, exportReq("GET", v.Download, "", owner))
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" {
		t.Fatal(w.Code, w.Body.String())
	}
	if w.Header().Get("X-Content-SHA256") != v.SHA256 || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal(w.Header())
	}
	z, e := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if e != nil {
		t.Fatal(e)
	}
	names := map[string]bool{}
	for _, f := range z.File {
		names[f.Name] = true
	}
	if !names["插件数据文件/test_export/command_txt/链.txt"] || !names["插件文件/test_export/modules/b.lua"] {
		t.Fatal(names)
	}
	for _, a := range []yyPoolActor{owner, other, admin} {
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, exportReq("GET", "/api/youyuan/exports", "", a))
		var r struct {
			Items []yyPluginExport `json:"items"`
		}
		json.Unmarshal(w.Body.Bytes(), &r)
		expected := 1
		if a.ID == other.ID {
			expected = 0
		}
		if len(r.Items) != expected {
			t.Fatal(a.ID, w.Body.String())
		}
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, exportReq("GET", v.Download, "", other))
	if w.Code != 404 {
		t.Fatal("another user read backup", w.Code)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, exportReq("POST", "/api/youyuan/exports/remove", `{"id":"`+v.ID+`"}`, other))
	if w.Code != 404 {
		t.Fatal("another user removed backup", w.Code)
	}
	for _, bad := range []string{"../config", "..%2Fconfig", ""} {
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, exportReq("GET", "/api/youyuan/exports/download?id="+bad, "", owner))
		if w.Code != 404 {
			t.Fatal("bad id", bad, w.Code)
		}
	}
	reader := owner
	reader.Scope = "read"
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, exportReq("POST", "/api/youyuan/exports/remove", `{"id":"`+v.ID+`"}`, reader))
	if w.Code != 403 {
		t.Fatal("read session deleted file")
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, exportReq("POST", "/api/youyuan/exports/remove", `{"id":"`+v.ID+`"}`, owner))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if _, e = os.Stat(filepath.Join(dir, "插件文件", id, id+".lua")); e != nil {
		t.Fatal("delete backup removed plugin", e)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, exportReq("GET", v.Download, "", owner))
	if w.Code != 404 {
		t.Fatal("deleted backup still available")
	}
}
func TestRuntimeExportRetainedZipHasCompleteResourceTree(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "p")
	os.MkdirAll(filepath.Join(p, "resources"), 0700)
	for name, value := range map[string]string{"manifest.json": `{"id":"p","language":"python","entry":"main.py"}`, "main.py": "def on_init(host): pass", "resources/a.txt": "payload"} {
		if e := os.WriteFile(filepath.Join(p, filepath.FromSlash(name)), []byte(value), 0600); e != nil {
			t.Fatal(e)
		}
	}
	source, e := yyRuntimeExportPackage(root, "p")
	if e != nil {
		t.Fatal(e)
	}
	dest := t.TempDir()
	v, e := yyRetainExport(dest, source, "p", "runtime", yyPoolActor{ID: "owner"})
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.OpenReader(filepath.Join(dest, v.ID+".zip"))
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	found := false
	for _, f := range z.File {
		if f.Name == "resources/a.txt" {
			r, _ := f.Open()
			b, _ := io.ReadAll(r)
			r.Close()
			found = string(b) == "payload"
		}
	}
	if !found {
		t.Fatal("resource file missing")
	}
}

func TestMCPExportDownloadReturnsMetadataWithoutReadingZip(t *testing.T) {
	oldRemote, oldMux := yyRemote, yyMux
	defer func() { yyRemote = oldRemote; yyMux = oldMux }()
	yyRemote.Key = "export-mcp-fixture"
	yyMux = http.NewServeMux()
	binaryReads, infoReads := 0, 0
	yyMux.HandleFunc("/api/youyuan/exports/download", func(w http.ResponseWriter, r *http.Request) { binaryReads++; w.Write([]byte("binary")) })
	yyMux.HandleFunc("/api/youyuan/exports/info", func(w http.ResponseWriter, r *http.Request) {
		infoReads++
		if r.URL.Query().Get("id") != "fixture" {
			t.Fatal(r.URL)
		}
		writeJSON(w, map[string]any{"ok": true, "downloaded": false, "download_url": "/api/youyuan/exports/download?id=fixture"})
	})
	r := httptest.NewRequest("POST", "/mcp/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"api_call","arguments":{"path":"/api/youyuan/exports/download","query":{"id":"fixture"}}}}`))
	r.Header.Set("Authorization", "Bearer "+yyRemote.Key)
	w := httptest.NewRecorder()
	yyMCP(w, r)
	if binaryReads != 0 || infoReads != 1 || !strings.Contains(w.Body.String(), `downloaded\":false`) {
		t.Fatal(binaryReads, infoReads, w.Body.String())
	}
}
