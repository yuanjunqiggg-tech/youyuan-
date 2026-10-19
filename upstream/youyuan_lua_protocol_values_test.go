package main

import (
	"github.com/go-gl/mathgl/mgl32"
	lua "github.com/yuin/gopher-lua"
	"testing"
)

// The real 01034 Linux probe delivered health as strings and maximum health
// as a Go pointer address in Lua, while Python/C++ received 8/10 correctly.
func TestLuaProtocolHealthNumbersAndOptionalValues(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	maximum, previous := float32(10), float32(10)
	var missing *float32
	event := map[string]any{"runtime_id": "9007199254740993", "health": float32(8), "max_health": &maximum, "previous": &previous, "dimension": int32(0), "unknown": missing}
	event["data"] = map[string]any{"health": float32(8), "max_health": &maximum}
	L.SetGlobal("event", goToLua(L, event))
	if err := L.DoString(`assert(event.runtime_id=="9007199254740993"); assert(type(event.health)=="number" and event.health==8); assert(type(event.max_health)=="number" and event.max_health==10); assert(event.previous==10 and type(event.dimension)=="number"); assert(event.unknown==nil); assert(event.data.health==8 and event.data.max_health==10)`); err != nil {
		t.Fatal(err)
	}
}

func TestLuaProtocolVectorsAndTypedNBT(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	type Damage float32
	fields := map[string]any{"position": mgl32.Vec3{1.25, 2.5, 3.75}, "damage": Damage(2), "nbt": map[string]int16{"lvl": 5}, "bytes": []uint8{1, 2, 3}, "flag": uint8(1)}
	L.SetGlobal("fields", goToLua(L, fields))
	if err := L.DoString(`assert(type(fields.position)=="table" and fields.position[3]==3.75); assert(fields.damage==2 and fields.nbt.lvl==5 and fields.bytes[2]==2 and fields.flag==1)`); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyArgumentTablesPreserveStructuredProtocolValues(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	value := float32(14)
	L.SetGlobal("args", argsToTable(L, []any{map[string]any{"health": &value, "dimension": int32(1)}, [2]float32{4, 5}, nil}))
	if err := L.DoString(`assert(args[1].health==14 and args[1].dimension==1); assert(args[2][1]==4 and args[2][2]==5); assert(args[3]==nil)`); err != nil {
		t.Fatal(err)
	}
}
