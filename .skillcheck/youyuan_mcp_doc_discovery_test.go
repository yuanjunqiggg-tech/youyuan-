package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestYYMCPDocumentDiscovery(t *testing.T) {
	mux := http.NewServeMux()
	yyDocumentationRoutes(mux)
	for _, path := range []string{"/api/youyuan/docs", "/api/youyuan/docs?language=mcp-connection", "/api/youyuan/docs?language=bedrock-commands"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["ok"] != true {
			t.Fatalf("%s: %s", path, w.Body.String())
		}
		if path == "/api/youyuan/docs" {
			if len(body["resources"].([]any)) < 20 {
				t.Fatal("missing resource directory")
			}
		} else if len(body["content"].(string)) < 100 {
			t.Fatal("empty document")
		}
	}
	for _, raw := range yyMCPTools() {
		tool := raw.(map[string]any)
		if tool["name"] == "api_youyuan_docs" {
			schema := tool["inputSchema"].(map[string]any)
			properties := schema["properties"].(map[string]any)
			query := properties["query"].(map[string]any)
			if query["properties"].(map[string]any)["language"] == nil {
				t.Fatal("language missing from schema")
			}
			return
		}
	}
	t.Fatal("document tool missing")
}
