package main

import (
	lua "github.com/yuin/gopher-lua"
	"testing"
)

func TestObserverBridgeLuaBindingsAndPublicSchema(t *testing.T) {
	L := newLuaState(senseBot(t))
	defer L.Close()
	defer cancelLuaCtx(L)
	ns := L.GetGlobal("observerBridge")
	if ns.Type() != lua.LTTable {
		t.Fatal("missing Lua namespace")
	}
	for method := range yyObserverBridgeMethods {
		name := method[len("observerBridge."):]
		if L.GetField(ns, name).Type() != lua.LTFunction {
			t.Fatalf("missing Lua function %s", method)
		}
	}
	value, err := yyPublicGameCall(nil, "packet.describe", []any{"Animate"})
	if err != nil {
		t.Fatal(err)
	}
	if value.(map[string]any)["name"] != "Animate" {
		t.Fatal(value)
	}
}
