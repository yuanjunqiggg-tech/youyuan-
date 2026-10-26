package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHostFileUploadDownloadAndCollision(t *testing.T) {
	mux := http.NewServeMux()
	root := t.TempDir()
	yyHostFilesRoutes(mux, root)
	upload := func() map[string]any {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("file", "资源.zip")
		part.Write([]byte("zip payload"))
		form.Close()
		req := httptest.NewRequest("POST", "/api/youyuan/files/upload", &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		var result map[string]any
		json.Unmarshal(w.Body.Bytes(), &result)
		if result["ok"] != true {
			t.Fatal(result)
		}
		return result
	}
	first, second := upload(), upload()
	if first["name"] == second["name"] {
		t.Fatal("overwrote existing file")
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/youyuan/files/download?name=%E8%B5%84%E6%BA%90.zip", nil))
	if w.Code != 200 || w.Body.String() != "zip payload" {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/youyuan/files/download?name=../config.json", nil))
	if w.Code != 400 {
		t.Fatal("path traversal", w.Code)
	}
}
func TestLinuxLibraryUsesLocalDiskAndSurvivesRestart(t *testing.T) {
	old := yyLinuxLibraryRoot
	yyLinuxLibraryRoot = t.TempDir()
	defer func() { yyLinuxLibraryRoot = old }()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	form.WriteField("category", "plugins")
	part, _ := form.CreateFormFile("file", "plugin.zip")
	io.WriteString(part, "fixture")
	form.Close()
	req := httptest.NewRequest("POST", "/api/youyuan/library/upload", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	w := httptest.NewRecorder()
	yyLocalLibrary(w, req)
	var response struct {
		Ok   bool               `json:"ok"`
		Item yyLinuxLibraryItem `json:"item"`
	}
	json.Unmarshal(w.Body.Bytes(), &response)
	if !response.Ok {
		t.Fatal(w.Body.String())
	}
	if b, e := os.ReadFile(filepath.Join(yyLinuxLibraryRoot, response.Item.ID+".data")); e != nil || string(b) != "fixture" {
		t.Fatal(e, string(b))
	}
	w = httptest.NewRecorder()
	yyLocalLibrary(w, httptest.NewRequest("GET", "/api/youyuan/library/list", nil))
	if !bytes.Contains(w.Body.Bytes(), []byte(response.Item.ID)) {
		t.Fatal("metadata not persisted")
	}
	w = httptest.NewRecorder()
	yyLocalLibrary(w, httptest.NewRequest("GET", "/api/youyuan/library/download?id="+response.Item.ID, nil))
	if w.Body.String() != "fixture" {
		t.Fatal(w.Body.String())
	}
}
