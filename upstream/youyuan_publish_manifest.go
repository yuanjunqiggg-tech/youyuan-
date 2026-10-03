package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func yyWorkspaceRuntimeManifest(ws *yyWorkspace, language, entry string) (yyRuntimeManifest, error) {
	result := yyRuntimeManifest{ID: ws.ID, Name: ws.Name, Language: language, Entry: entry, Version: "1.0.0", Events: []string{"chat", "ready", "join", "leave"}}
	data, e := os.ReadFile(filepath.Join(yyWorkspaceRoot(ws), "manifest.json"))
	if os.IsNotExist(e) {
		return result, nil
	}
	if e != nil {
		return result, e
	}
	if len(data) > 65536 {
		return result, fmt.Errorf("manifest 超过 64 KB")
	}
	if e = json.Unmarshal(data, &result); e != nil {
		return result, e
	}
	// Publishing uses the workspace identity and fixed language entry; user metadata survives.
	result.ID = ws.ID
	result.Name = ws.Name
	result.Language = language
	result.Entry = entry
	return result, nil
}
