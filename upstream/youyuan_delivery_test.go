package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYouYuanDisabledAgentToolCannotModifyProject(t *testing.T) {
	previous := yyAgentRoot
	yyAgentRoot = t.TempDir()
	defer func() { yyAgentRoot = previous }()
	ws := &yyWorkspace{ID: "disabled", Language: "python", AllowedTools: []string{}}
	if e := os.MkdirAll(yyWorkspaceRoot(ws), 0700); e != nil {
		t.Fatal(e)
	}
	if yyAgentAllowed(ws, "project_write") || len(yyWorkspaceToolDefs(ws)) != 0 {
		t.Fatal("empty explicit allow list reverted to defaults")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		messages := body["messages"].([]any)
		var message map[string]any
		if len(messages) <= 1 {
			message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "disabled_call", "type": "function", "function": map[string]any{"name": "project_write", "arguments": `{"path":"should-not-exist.py","content":"x=1"}`}}}}
		} else {
			message = map[string]any{"role": "assistant", "content": "done"}
		}
		writeJSON(w, map[string]any{"choices": []any{map[string]any{"message": message}}})
	}))
	defer server.Close()
	ws.run(context.Background(), &AIModel{Provider: ProviderOpenAI, BaseURL: server.URL, ModelID: "test"})
	if _, e := os.Stat(filepath.Join(yyWorkspaceRoot(ws), "should-not-exist.py")); !os.IsNotExist(e) {
		t.Fatal("disabled model tool performed a file write")
	}
}
func TestYouYuanBinaryRevisionsAndMCPDocs(t *testing.T) {
	previous := yyAgentRoot
	yyAgentRoot = t.TempDir()
	defer func() { yyAgentRoot = previous }()
	ws := &yyWorkspace{ID: "binary", Language: "native_cpp"}
	original := string([]byte{0x7f, 'E', 'L', 'F', 0, 0xff, 0xc0})
	if _, e := yyWriteProject(ws, "plugin.so", original); e != nil {
		t.Fatal(e)
	}
	revision, e := yyWriteProject(ws, "plugin.so", original+"updated")
	if e != nil {
		t.Fatal(e)
	}
	if revision.Encoding != "base64" {
		t.Fatal("binary history was not losslessly encoded")
	}
	result, bad := yyWorkspaceTool(context.Background(), ws, "project_rollback", map[string]any{"revision": revision.ID})
	if bad {
		t.Fatal(result)
	}
	data, e := os.ReadFile(filepath.Join(yyWorkspaceRoot(ws), "plugin.so"))
	actual := string(data)
	if e != nil || actual != original {
		t.Fatal("binary rollback corrupted original bytes")
	}
	for _, entry := range yyMCPResources() {
		uri := entry.(map[string]any)["uri"].(string)
		if !strings.Contains(uri, "/docs/") {
			continue
		}
		content, e := yyMCPReadResource(uri)
		if e != nil || len(content["text"].(string)) < 20 {
			t.Fatalf("resource %s invalid %v", uri, e)
		}
	}
	if _, e := yyMCPReadResource("youyuan://docs/../../private/signing.json"); e == nil {
		t.Fatal("resource path traversal accepted")
	}
}
func TestYouYuanManagedAccountIDAndToolNames(t *testing.T) {
	for _, id := range []string{"../main", "a.b", "a:b", "a/b", "账号", "main/..", ""} {
		if yyManagedAccountID(id) {
			t.Fatalf("invalid account marker accepted %q", id)
		}
	}
	for _, id := range []string{"account_1", "account-2"} {
		if !yyManagedAccountID(id) {
			t.Fatal(id)
		}
	}
	names := map[string]bool{}
	for _, r := range yyAPIRoutes {
		if names[r.Name] {
			t.Fatalf("duplicate MCP tool %s", r.Name)
		}
		names[r.Name] = true
	}
}
