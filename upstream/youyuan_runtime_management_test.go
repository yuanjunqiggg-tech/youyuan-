package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDeletionCannotEscapeAndLeavesOtherPlugin(t *testing.T) {
	old := yyRuntimeRoot
	yyRuntimeRoot = t.TempDir()
	defer func() { yyRuntimeRoot = old }()
	for _, id := range []string{"target", "neighbor"} {
		os.MkdirAll(filepath.Join(yyRuntimeRoot, id, "data"), 0700)
		os.WriteFile(filepath.Join(yyRuntimeRoot, id, "data", "save.json"), []byte("saved"), 0600)
	}
	mux := http.NewServeMux()
	yyRuntimeManagementRoutes(mux)
	for _, id := range []string{"..", "target/../neighbor", "target"} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest("POST", "/api/youyuan/runtime/delete", strings.NewReader(`{"id":"`+id+`"}`)))
		if id != "target" && !strings.Contains(recorder.Body.String(), `"ok":false`) {
			t.Fatal(recorder.Body.String())
		}
	}
	if _, e := os.Stat(filepath.Join(yyRuntimeRoot, "target")); !os.IsNotExist(e) {
		t.Fatal("target not removed")
	}
	if data, e := os.ReadFile(filepath.Join(yyRuntimeRoot, "neighbor", "data", "save.json")); e != nil || string(data) != "saved" {
		t.Fatal("neighbor data changed", e)
	}
}
