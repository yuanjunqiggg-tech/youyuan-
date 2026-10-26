package main

import (
	"strings"
	"testing"
)

func TestFleetConnectionPanelIsCompactAndNotImport(t *testing.T) {
	for _, c := range [][2]int{{0, 0}, {0, 4}, {2, 4}, {4, 4}, {9, 4}, {-1, 4}} {
		text := yyFleetConnectionPanel("游园", c[0], c[1])
		if strings.Count(text, "\n") != 2 || strings.Count(text, "━") != 12 || strings.Contains(text, "导入") || strings.Contains(text, "prism") {
			t.Fatal(text)
		}
	}
}
