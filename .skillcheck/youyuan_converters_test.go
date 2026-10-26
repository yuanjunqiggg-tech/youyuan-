package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func conversionTestCall(t *testing.T, path string, body any) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	yyConverterRoutes(mux)
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(string(raw))))
	var out map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e, w.Body.String())
	}
	return out
}
func TestConversionStructureRoundTripAndMerge(t *testing.T) {
	raw := `{"size":[4,1,1],"blocks":[{"x":0,"y":0,"z":0,"name":"minecraft:stone"},{"x":1,"y":0,"z":0,"name":"minecraft:stone"}]}`
	call := func(target string, rotation int) map[string]any {
		return conversionTestCall(t, "/api/youyuan/convert/structure", map[string]any{"data": base64.StdEncoding.EncodeToString([]byte(raw)), "filename": "sample.json", "target": target, "rotation": rotation, "origin": []int{10, 80, 20}, "relative": true, "merge": true})
	}
	out := call("txt", 0)
	if out["ok"] != true || out["commands"] != float64(1) || !strings.Contains(out["content"].(string), "fill ~10 ~80 ~20 ~11 ~80 ~20 minecraft:stone") {
		t.Fatal(out)
	}
	parsed, e := yyParseChains(out["content"].(string))
	if e != nil || parsed.Commands != 1 {
		t.Fatal(parsed, e)
	}
	out = call("json", 0)
	s, e := yyParseConversion([]byte(out["content"].(string)), ".json")
	if e != nil || s.SizeX != 4 || len(s.AllBlocks()) != 2 {
		t.Fatal(s, e)
	}
	out = call("mcstructure", 90)
	data, _ := base64.StdEncoding.DecodeString(out["data"].(string))
	s, e = yyParseConversion(data, ".mcstructure")
	if e != nil || s.SizeX != 1 || s.SizeZ != 4 || len(s.AllBlocks()) != 2 {
		t.Fatal(s, e)
	}
	_, count := yyStructureCommands(s, [3]int{}, false, false, true)
	if count != 4 {
		t.Fatal("air missing", count)
	}
	for _, bad := range []string{`{"size":[1,1,1],"blocks":[{"x":2,"name":"stone"}]}`, `{"size":[9223372036854775807,4,4],"blocks":[]}`, `[{"x":-1,"name":"stone"}]`} {
		if _, e = yyParseBlockJSON([]byte(bad)); e == nil {
			t.Fatal("invalid JSON accepted", bad)
		}
	}
}
func TestConversionCommandsExecutionAndLossDetection(t *testing.T) {
	out := conversionTestCall(t, "/api/youyuan/convert/commands", map[string]any{"content": "/say hi\r\n/say bye", "mode": "execute", "selector": "@a[tag=builder]", "output": "mcfunction"})
	if out["ok"] != true || out["commands"] != float64(2) || !strings.Contains(out["content"].(string), "execute as @a[tag=builder] at @s run say hi") {
		t.Fatal(out)
	}
	for _, body := range []map[string]any{{"content": "1,0,0\nsay hi", "output": "mcfunction"}, {"content": "1,0,0\nsay hi", "mode": "execute", "selector": "@a"}, {"content": "say hi", "mode": "execute", "selector": "@a\nsay inject"}} {
		if out = conversionTestCall(t, "/api/youyuan/convert/commands", body); out["ok"] != false {
			t.Fatal(out)
		}
	}
}
