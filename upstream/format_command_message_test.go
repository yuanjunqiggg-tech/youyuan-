package main

import "testing"

// Covers the fix for the broken formatCommandMessage: Java-style %1$s placeholders
// now substitute correctly and no longer double-append params via the fallback branch.
func TestFormatCommandMessage(t *testing.T) {
	cases := []struct {
		name   string
		msg    string
		params []string
		want   string
	}{
		{"java positional", "Set %1$s at %2$s", []string{"stone", "0 0 0"}, "Set stone at 0 0 0"},
		{"c style percent-s", "Placed %s blocks", []string{"64"}, "Placed 64 blocks"},
		{"localization key", "commands.setblock.success", []string{"1", "2", "3"}, "commands.setblock.success: 1, 2, 3"},
		{"no params", "already done", nil, "already done"},
	}
	for _, c := range cases {
		if got := formatCommandMessage(c.msg, c.params); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
