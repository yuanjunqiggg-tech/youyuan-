package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSDKPublishPreservesMetadata(t *testing.T) {
	old := yyAgentRoot
	yyAgentRoot = t.TempDir()
	defer func() { yyAgentRoot = old }()
	ws := &yyWorkspace{ID: "sdk_publish", Name: "SDK Metadata", Language: "python"}
	root := yyWorkspaceRoot(ws)
	if e := os.MkdirAll(root, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{"id":"other","entry":"../bad","version":"1.3.0","dependencies":{"dep":"1.0.0"},"events":["packet"]}`), 0600); e != nil {
		t.Fatal(e)
	}
	m, e := yyWorkspaceRuntimeManifest(ws, "python", "main.py")
	if e != nil || m.ID != ws.ID || m.Entry != "main.py" || m.Version != "1.3.0" || m.Dependencies["dep"] != "1.0.0" || m.Events[0] != "packet" {
		t.Fatalf("metadata: %+v %v", m, e)
	}
	if yySDKWebID("a_b:c") == yySDKWebID("a:b_c") {
		t.Fatal("web page identity collision")
	}
}
