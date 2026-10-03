package building

import "testing"

// Regression for bug #2: exported .mcworld names embed the region as [x y z]~[x y z].
// The parser must read the bracketed coords (space OR comma separated) and must not
// fall back to grabbing the last 6 numbers, which extra digits in the name corrupt.
func TestParseMCWorldRegion(t *testing.T) {
	want := [6]int{15000, -60, 15000, 15076, 0, 15051}
	cases := map[string]string{
		"space separated":       "指令区2@[15000 -60 15000]~[15076 0 15051](2).mcworld",
		"comma separated":       "名称@[15000,-60,15000]~[15076,0,15051].mcworld",
		"comma and space":       "x@[15000, -60, 15000]~[15076, 0, 15051].mcworld",
		"extra trailing digits":  "world2@[15000 -60 15000]~[15076 0 15051] (copy 3).mcworld",
	}
	for name, fn := range cases {
		if got := parseMCWorldRegion(fn); got != want {
			t.Errorf("%s: parseMCWorldRegion(%q) = %v, want %v", name, fn, got, want)
		}
	}
}
