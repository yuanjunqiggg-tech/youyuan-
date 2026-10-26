package main

import "testing"

func TestImportLimitsPublicCallAndValidation(t *testing.T) {
	v, e := yyPublicGameCall(nil, "building.importLimits", []any{map[string]any{"x": -1, "z": 0}})
	if e != nil {
		t.Fatal(e)
	}
	m := v.(map[string]any)
	if m["max_region_size_chunks"] != 9 || m["max_chunks_per_ticking_area"] != 100 || m["server_verified"] != false {
		t.Fatal(m)
	}
	for _, c := range []struct {
		kind  string
		p     map[string]any
		valid bool
	}{
		{"import", map[string]any{"region_size_chunks": 10}, true},
		{"import", map[string]any{"region_size_chunks": 10, "x": 1}, false},
		{"import", map[string]any{"region_size_chunks": 10.5}, false},
		{"import", map[string]any{"region_size_chunks": 5, "preload_size_chunks": 10}, true},
		{"import", map[string]any{"region_size_chunks": 5, "preload_size_chunks": 7}, false},
		{"import", map[string]any{"preload_size_chunks": 10.5}, false},
		{"import_multi", map[string]any{"preload_size_chunks": 10}, false},
		{"import_multi", map[string]any{"region_size_chunks": 10}, false},
		{"import_multi", map[string]any{}, true},
		{"import_multi", map[string]any{"region_mode": 4, "preload_size_chunks": 1}, true},
	} {
		if err := yyValidateImportRegionParams(c.kind, c.p); (err == nil) != c.valid {
			t.Fatal(c, err)
		}
	}
}
