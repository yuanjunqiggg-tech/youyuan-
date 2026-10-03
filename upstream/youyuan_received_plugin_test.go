package main

import (
	lua "github.com/yuin/gopher-lua"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReceivedR20BCompileAndTimezone(t *testing.T) {
	path := os.Getenv("YY_RECEIVED_PLUGIN_SOURCE")
	if path == "" {
		t.Skip("no supplied plugin selected")
	}
	state := lua.NewState()
	defer state.Close()
	if _, e := state.LoadFile(path); e != nil {
		t.Fatal(e)
	} // Compile only; do not activate its game handlers.
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	text := string(source)
	start := strings.Index(text, "function beijing_fmt(")
	end := strings.Index(text[start:], "\nend")
	if start < 0 || end < 0 {
		t.Fatal("missing timezone helper")
	}
	snippet := "BEIJING_OFFSET=8*3600\n" + text[start:start+end+4]
	old := time.Local
	defer func() { time.Local = old }()
	for _, name := range []string{"UTC", "Asia/Shanghai"} {
		time.Local, _ = time.LoadLocation(name)
		if e = state.DoString(snippet + "\nassert(beijing_fmt(1790810400)=='2026-10-01 07:20:00')"); e != nil {
			t.Fatal(name, e)
		}
	}
}
