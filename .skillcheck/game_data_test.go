package main

import "testing"

// 验证方块名本地化清理。
func TestCleanTileName(t *testing.T) {
	cases := map[string]string{
		"%tile.bedrock.name": "bedrock",
		"%tile.air.name":     "air",
		"stone":              "stone",
		"":                   "",
	}
	for in, want := range cases {
		if got := cleanTileName(in); got != want {
			t.Errorf("cleanTileName(%q)=%q, want %q", in, got, want)
		}
	}
}
