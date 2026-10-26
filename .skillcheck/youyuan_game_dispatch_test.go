package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnknownAPIReturnsJSON(t *testing.T) {
	w := httptest.NewRecorder()
	handleIndex(w, httptest.NewRequest("GET", "/api/no_such_route", nil))
	if w.Code != 404 || !strings.Contains(w.Header().Get("Content-Type"), "application/json") || !strings.Contains(w.Body.String(), "not_found") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestPublicTPSUnknownAndStatefulImportAreExplicit(t *testing.T) {
	for _, method := range []string{"game.getServerTPS", "game.getTPS", "game.serverTPS", "game.getTickRate", "game.getServerTickRate"} {
		v, err := yyPublicGameCall(nil, method, nil)
		if err != nil || v != nil {
			t.Fatalf("unknown TPS must stay unknown: %s %v %v", method, v, err)
		}
	}
	if _, err := yyPublicGameCall(nil, "game.importBlocks", nil); err == nil {
		t.Fatal("stateful import cannot return an unusable serialized userdata")
	}
}
func TestPublicCommandSharesActualBinding(t *testing.T) {
	// Fast and default paths validate connection, while this pure formatter is
	// a base Lua binding outside yyExtraMethods. It must not be called a stub.
	value, e := yyPublicGameCall(nil, "game.dataDir", nil)
	if e != nil || value == nil {
		t.Fatalf("base binding inaccessible: %v %v", value, e)
	}
	if yyCanonicalGameMethod("sendCommandFast") != "game.sendCommandFast" {
		t.Fatal("legacy name not normalized")
	}
	if _, e := yyPublicGameCall(nil, "game.nonexistent", nil); e == nil {
		t.Fatal("unknown method pretended to work")
	}
}
