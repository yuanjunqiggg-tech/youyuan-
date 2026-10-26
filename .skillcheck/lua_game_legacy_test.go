package main

import (
	lua "github.com/yuin/gopher-lua"
	"testing"
)

func TestLegacyImportPreservesStatesNBTAndLargeList(t *testing.T) {
	L := newYYLuaVM()
	defer L.Close()
	if err := L.DoString(`blocks={};for i=0,2499 do blocks[#blocks+1]={x=i,y=0,z=0,name="minecraft:oak_stairs",states={facing_direction=2,upside_down_bit=false},nbt={CustomName="kept"}} end`); err != nil {
		t.Fatal(err)
	}
	data, err := yyLegacyImportData(L.GetGlobal("blocks").(*lua.LTable))
	if err != nil {
		t.Fatal(err)
	}
	if data.Len() != 2500 {
		t.Fatalf("lost blocks: %d", data.Len())
	}
	block := data.Get(2499, 0, 0)
	if block.States != `["facing_direction"=2,"upside_down_bit"=false]` {
		t.Fatal(block.States)
	}
	if data.GetNBT(2499, 0, 0)["CustomName"] != "kept" {
		t.Fatal("lost NBT")
	}
}

func TestLegacyImportRejectsInvalidStatesInsteadOfDropping(t *testing.T) {
	L := newYYLuaVM()
	defer L.Close()
	if err := L.DoString(`blocks={{x=0,y=0,z=0,name="stone",states={unsupported={value=1}}}}`); err != nil {
		t.Fatal(err)
	}
	if _, err := yyLegacyImportData(L.GetGlobal("blocks").(*lua.LTable)); err == nil {
		t.Fatal("invalid state silently dropped")
	}
}
