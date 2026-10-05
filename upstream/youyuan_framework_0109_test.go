package main

import (
	"bytes"
	"context"
	"encoding/json"
	lua "github.com/yuin/gopher-lua"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/TriM-Organization/bedrock-world-operator/chunk"
	"github.com/TriM-Organization/bedrock-world-operator/define"
)

func TestResidentStopCancelsActiveLuaBeforeWaitingForVMLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &yyLuaResident{L: lua.NewState(), ready: true, lifetime: ctx, stop: cancel, senseDone: make(chan struct{})}
	bm := &BotManager{}
	key := yyLuaKey{bm, "qa_stop_busy"}
	if err := r.L.DoString("function busy() while true do end end"); err != nil {
		t.Fatal(err)
	}
	yyLuaMu.Lock()
	yyLuaStates[key] = r
	yyLuaMu.Unlock()
	done := make(chan struct{})
	go func() { _, _ = yyCallResidentState(bm, key.id, "busy", nil, false, false, r); close(done) }()
	deadline := time.Now().Add(time.Second)
	for r.mu.TryLock() {
		r.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("callback never started")
		}
		time.Sleep(time.Millisecond)
	}
	stopped := make(chan struct{})
	go func() { yyStopResident(bm, key.id); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("stop waited for the execution timeout instead of cancelling")
	}
	<-done
}

func TestRegionFallbackNegativeCoordinatesNBTAndMissing(t *testing.T) {
	pos, size := [3]int32{-1, 64, -1}, [3]int32{2, 1, 2}
	columns := map[[2]int32][]*chunk.SubChunk{}
	for _, x := range []int32{-1, 0} {
		for _, z := range []int32{-1, 0} {
			columns[[2]int32{x, z}] = []*chunk.SubChunk{nil}
		}
	}
	data, e := yySubChunksToStructure(pos, size, columns, []map[string]any{{"x": int32(0), "y": int32(64), "z": int32(0), "id": "Chest"}})
	if e != nil {
		t.Fatal(e)
	}
	structure := data["structure"].(map[string]any)
	indices := structure["block_indices"].([]any)
	if !reflect.DeepEqual(indices[0], []int32{0, 0, 0, 0}) || !reflect.DeepEqual(indices[1], []int32{-1, -1, -1, -1}) {
		t.Fatal(indices)
	}
	bpd := structure["palette"].(map[string]any)["default"].(map[string]any)["block_position_data"].(map[string]any)
	if bpd["3"] == nil {
		t.Fatal("Z-fastest NBT index wrong", bpd)
	}
	delete(columns, [2]int32{0, 0})
	if _, e = yySubChunksToStructure(pos, size, columns, nil); e == nil {
		t.Fatal("invented air for missing chunk")
	}
	if _, e = yySubChunksToStructure([3]int32{}, [3]int32{2147483647, 2147483647, 2147483647}, nil, nil); e == nil {
		t.Fatal("volume overflow accepted")
	}
}
func TestSubChunkFailureNeverMeansAir(t *testing.T) {
	base := protocol.SubChunkPos{0, 4, 0}
	r := &packet.SubChunk{SubChunkEntries: []protocol.SubChunkEntry{{Offset: [3]int8{0, 0, 0}, Result: 255}}}
	_, _, missing, e := decodeSubs(r, base, 4, 5, define.Dimension(0).Range())
	if e != nil {
		t.Fatal(e)
	}
	if len(missing) != 2 {
		t.Fatal("unknown or omitted entry treated as air", missing)
	}
}
func TestFastChain60000NodesOneSubmission(t *testing.T) {
	p, e := yyParseChains(strings.Repeat("scoreboard players add @s bench 1\n", 60000))
	if e != nil {
		t.Fatal(e)
	}
	calls, total := 0, 0
	yyChainExecuteBuffered(context.Background(), p, yyChainConfig{Mode: "fast"}, 0, func(yyChainNode) (bool, error) {
		t.Fatal("unconditional fast chain waited for result")
		return false, nil
	}, func(nodes []yyChainNode) ([]map[string]any, error) {
		calls++
		rows := make([]map[string]any, len(nodes))
		for i := range rows {
			rows[i] = map[string]any{"submitted": true, "confirmed": false}
		}
		return rows, nil
	}, func(ok, skip bool, e error) {
		if !ok || skip || e != nil {
			t.Fatal(ok, skip, e)
		}
		total++
	})
	if calls != 1 || total != 60000 {
		t.Fatal(calls, total)
	}
}
func TestStorageAtomicUpdatesTransactionsIsolationAndRestart(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if _, e := yyStorageCall(dir, "storage.atomicAdd", []any{"score", float64(1)}); e != nil {
					t.Error(e)
				}
			}
		}()
	}
	wg.Wait()
	v, e := yyStorageCall(dir, "storage.get", []any{"score"})
	if e != nil || v != float64(800) {
		t.Fatal(v, e)
	}
	if v, _ = yyStorageCall(other, "storage.get", []any{"score"}); v != nil {
		t.Fatal("cross-plugin storage leaked")
	}
	ops := []any{map[string]any{"op": "set", "key": "score", "value": float64(0)}, map[string]any{"op": "check", "key": "score", "value": float64(800)}}
	if _, e = yyStorageCall(dir, "storage.transaction", []any{ops}); e == nil {
		t.Fatal("failed check committed")
	}
	v, _ = yyStorageCall(dir, "storage.get", []any{"score"})
	if v != float64(800) {
		t.Fatal("failed transaction changed cache")
	}
	ops = []any{map[string]any{"op": "check", "key": "score", "value": float64(800)}, map[string]any{"op": "add", "key": "score", "value": float64(2)}, map[string]any{"op": "set", "key": "paid", "value": true}}
	if _, e = yyStorageCall(dir, "storage.transaction", []any{ops}); e != nil {
		t.Fatal(e)
	}
	c, _ := yyStorageFor(dir)
	yyStorageCaches.Lock()
	delete(yyStorageCaches.items, c.Path)
	yyStorageCaches.Unlock()
	v, e = yyStorageCall(dir, "storage.get", []any{"score"})
	if e != nil || v != float64(802) {
		t.Fatal("restart persistence", v, e)
	}
	if _, e = yyStorageCall(dir, "storage.flush", nil); e != nil {
		t.Fatal(e)
	}
}
func TestExactDeathRPCAndStructuredEventCompatibility(t *testing.T) {
	bm := senseBot(t)
	yySenseHandle(bm, &packet.AddPlayer{Username: "Player", EntityRuntimeID: 91, AbilityData: protocol.AbilityData{EntityUniqueID: 9223372036854000000}})
	v := []any{"ModEventS2C", []any{"Minecraft", "event", "OnPlayerDie", map[string]any{"pid": "9223372036854000000", "die": true}}, nil}
	yySenseHandle(bm, &packet.PyRpc{Value: v})
	yySenseHandle(bm, &packet.PyRpc{Value: v})
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	count := 0
	for _, e := range s.Events {
		if e["type"] == "death" {
			count++
			if e["player"] != "Player" || e["timestamp"] == nil || e["data"] == nil || !yySenseEventMatches("player_death", e) {
				t.Fatal(e)
			}
		}
	}
	if count != 1 {
		t.Fatal("death duplication or lost event", count)
	}
	if _, _, ok := yyDeathRPC([]any{"other", []any{"OnPlayerDie"}, nil}); ok {
		t.Fatal("guessed unrelated RPC")
	}
}
func TestSkillsMultiSelectAtomicAndAllLanguagesAvailable(t *testing.T) {
	old := yyAgentRoot
	yyAgentRoot = t.TempDir()
	defer func() { yyAgentRoot = old }()
	ws := &yyWorkspace{ID: "qa_multi109", Language: "lua", Skills: []string{}}
	os.MkdirAll(yyAgentRoot+"/"+ws.ID, 0700)
	yyWorkspaces.Lock()
	yyWorkspaces.items[ws.ID] = ws
	yyWorkspaces.Unlock()
	defer func() { yyWorkspaces.Lock(); delete(yyWorkspaces.items, ws.ID); yyWorkspaces.Unlock() }()
	mux := http.NewServeMux()
	yySkillLibraryRoutes(mux)
	post := func(skills []string) bool {
		b, _ := json.Marshal(map[string]any{"id": ws.ID, "skills": skills})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/youyuan/agent/skills", bytes.NewReader(b)))
		var v map[string]any
		json.Unmarshal(w.Body.Bytes(), &v)
		return v["ok"] == true
	}
	selected := []string{"lua-runtime", "python-runtime", "cpp-runtime"}
	if !post(selected) || !reflect.DeepEqual(ws.Skills, selected) {
		t.Fatal("multiple language selection failed")
	}
	if post([]string{"lua-runtime", "unknown"}) || !reflect.DeepEqual(ws.Skills, selected) {
		t.Fatal("invalid selection partially saved")
	}
	if !post([]string{}) || ws.Skills == nil || len(ws.Skills) != 0 {
		t.Fatal("empty selection lost")
	}
}

func TestLuaCommandBufferBuildDoesNotDispatch(t *testing.T) {
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	if e := L.DoString(`local b=CommandBuffer();for i=1,10000 do b:add("say test") end;assert(b:size()==10000);b:clear();assert(b:size()==0)`); e != nil {
		t.Fatal(e)
	}
}

func TestSharedTickClosureUsesResidentState(t *testing.T) {
	bm := senseBot(t)
	id := "qa_tick_closure109"
	if e := yyLoadResident(bm, id, `ticks=0;util.scheduleTick(1,function() ticks=ticks+1 end);function value() return ticks end`, false); e != nil {
		t.Fatal(e)
	}
	defer func() { RemovePluginSchedules(id); yyStopResident(bm, id) }()
	time.Sleep(140 * time.Millisecond)
	v, e := yyCallResident(bm, id, "value", nil, false, false)
	if e != nil || v.(float64) < 1 {
		t.Fatal(v, e)
	}
}

func TestTXTInlineWholeRoundHzAndResourceReloadKeepsState(t *testing.T) {
	p, e := yyParseChains("2,0,0 20\nsay a\n0,0,0\nsay b\n!\n")
	if e != nil || p.Hz != 20 || p.Commands != 2 {
		t.Fatal(p, e)
	}
	root := yyChainRoot
	yyChainRoot = t.TempDir()
	defer func() { yyChainRoot = root }()
	in := &yyChainInstance{Config: yyChainConfig{Name: "watch.txt", Enabled: true}, Plan: p, Cycles: 47, Submitted: 94, State: "运行中"}
	if e = os.WriteFile(yyChainRoot+"/watch.txt", []byte("2,0,0 10\nsay new\n!\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = yyReloadChainResource(in); e != nil {
		t.Fatal(e)
	}
	if in.Plan.Hz != 10 || in.Cycles != 47 || in.Submitted != 94 || !in.Config.Enabled || in.State != "运行中" {
		t.Fatal("resource reload reset instance", in)
	}
	old := in.Plan
	os.WriteFile(yyChainRoot+"/watch.txt", []byte("bad invalid header\n9,1,1\n"), 0600)
	if e = yyReloadChainResource(in); e == nil || !reflect.DeepEqual(old, in.Plan) {
		t.Fatal("bad source replaced running IR")
	}
}

func TestAsyncCommandOwnershipAndNoFalseConfirmation(t *testing.T) {
	owner := yyAsyncCommandKey{Plugin: "qa_async109"}
	defer yyClearCommandTickets(owner)
	v, e := yyAsyncCommandCall(owner, "game.sendCommandAsync", []any{"say test"}, yyDefaultCommandOptions())
	if e != nil {
		t.Fatal(e)
	}
	id := float64(v.(map[string]any)["ticket"].(uint64))
	if _, e = yyAsyncCommandCall(yyAsyncCommandKey{Plugin: "other"}, "game.commandResult", []any{id}, yyDefaultCommandOptions()); e == nil {
		t.Fatal("cross-owner ticket access")
	}
	v, e = yyAsyncCommandCall(owner, "game.commandResult", []any{id, float64(1000), true}, yyDefaultCommandOptions())
	if e != nil {
		t.Fatal(e)
	}
	result := v.(map[string]any)
	if result["completed"] != true || result["error"] == nil {
		t.Fatal("disconnected command claimed success", result)
	}
	if _, e = yyAsyncCommandCall(owner, "game.commandResult", []any{id}, yyDefaultCommandOptions()); e == nil {
		t.Fatal("consumed ticket survived")
	}
}

func TestDeathKillerRequiresServerTranslationAndKnownPlayer(t *testing.T) {
	bm := senseBot(t)
	yySenseHandle(bm, &packet.AddPlayer{Username: "Victim", EntityRuntimeID: 101, AbilityData: protocol.AbilityData{EntityUniqueID: 101}})
	yySenseHandle(bm, &packet.AddPlayer{Username: "Attacker", EntityRuntimeID: 102, AbilityData: protocol.AbilityData{EntityUniqueID: 102}})
	yySenseHandle(bm, &packet.Text{TextType: packet.TextTypeChat, Message: "death.attack.player", Parameters: []string{"Victim", "Attacker"}})
	s := yySenseFor(bm)
	s.Lock()
	if s.Actors[101].Dead != nil && *s.Actors[101].Dead {
		t.Fatal("chat falsely classified as death")
	}
	s.Unlock()
	yySenseHandle(bm, &packet.PyRpc{Value: []any{"ModEventS2C", []any{"Minecraft", "event", "OnPlayerDie", map[string]any{"pid": "101", "die": true}}, nil}})
	yySenseHandle(bm, &packet.Text{TextType: packet.TextTypeTranslation, Message: "death.attack.player", Parameters: []string{"Victim", "Attacker"}})
	s.Lock()
	defer s.Unlock()
	if !s.Actors[101].KillerKnown || s.Actors[101].Killer != "Attacker" {
		t.Fatal("missing killer")
	}
	deaths := 0
	for _, e := range s.Events {
		if e["type"] == "death" {
			deaths++
			if e["killer_known"] != true || e["killer"] != "Attacker" {
				t.Fatal(e)
			}
		}
	}
	if deaths != 1 {
		t.Fatal("duplicate death", deaths)
	}
}
