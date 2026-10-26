package main

import (
	"bot-apk/building"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	lua "github.com/yuin/gopher-lua"
)

func TestNativeTPSDoesNotEstimateWorldTimeChanges(t *testing.T) {
	bm := &BotManager{connected: true}
	// World time may pause, jump through /time, roll backwards or advance at
	// another rate. None of these is a native server TPS measurement.
	for _, worldTime := range []int32{0, 0, 1, 20000, 24000, 1, -1, 2147483647} {
		bm.handleSetTime(&packet.SetTime{Time: worldTime})
		if value, known := bm.observedTPS(); known || value != 0 {
			t.Fatalf("world clock %d invented TPS: %v %v", worldTime, value, known)
		}
		if bm.GetTPS() != nil {
			t.Fatalf("world clock %d exposed a numeric TPS", worldTime)
		}
	}
	for _, method := range []string{"game.getServerTPS", "game.getTPS", "game.serverTPS", "game.getTickRate", "game.getServerTickRate"} {
		value, err := yyPublicGameCall(bm, method, nil)
		if err != nil || value != nil {
			t.Fatalf("connected host without native TPS must return null: %s %v %v", method, value, err)
		}
	}
}

func TestNativeTPSLuaAliasesReturnNil(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	registerAllLuaAPI(L, &BotManager{connected: true})
	if err := L.DoString(`
		for _, name in ipairs({"getTPS", "getServerTPS", "serverTPS", "getTickRate", "getServerTickRate"}) do
			assert(type(game[name]) == "function", name .. " missing")
			assert(game[name]() == nil, name .. " invented TPS")
		end
	`); err != nil {
		t.Fatal(err)
	}
}

func TestNativeTPSFloatingSnapshotIsNull(t *testing.T) {
	botMgrMu.Lock()
	previous := botMgr
	botMgr = &BotManager{connected: true}
	botMgrMu.Unlock()
	defer func() {
		botMgrMu.Lock()
		botMgr = previous
		botMgrMu.Unlock()
	}()
	encoded, err := json.Marshal(snapshotStatus())
	if err != nil || !strings.Contains(string(encoded), `"tps":null`) {
		t.Fatalf("floating snapshot must preserve unknown TPS: %s %v", encoded, err)
	}
}

func TestNativeTPSUnknownDoesNotBlockImport(t *testing.T) {
	bm := &BotManager{connected: true}
	if bm.GetTPS() != nil {
		t.Fatal("fixture unexpectedly has native TPS")
	}
	data := &building.StructureData{SizeX: 1, SizeY: 1, SizeZ: 1, Blocks: map[uint64]uint32{}}
	data.Set(0, 0, 0, &building.BlockInfo{Name: "minecraft:stone", States: "[]"})
	placed, committed := 0, 0
	task := &building.ImportTask{Data: data, Speed: 10000, StopOnError: true,
		SetBlock:   func(_, _, _ int32, _, _ string) error { placed++; return nil },
		OnProgress: func(_, _, _, _, _ int) { committed++ }}
	if err := task.Run(); err != nil || placed != 1 || committed != 1 {
		t.Fatalf("unknown TPS blocked import: %v placed=%d committed=%d", err, placed, committed)
	}
}
