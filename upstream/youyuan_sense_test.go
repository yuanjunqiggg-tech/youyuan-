package main

import (
	"strings"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	lua "github.com/yuin/gopher-lua"
)

func senseBot(t *testing.T) *BotManager {
	t.Helper()
	bm := &BotManager{runtimeID: 42, display: "Robot", logCh: make(chan string, 256)}
	t.Cleanup(func() { yySenses.Lock(); delete(yySenses.Items, bm); yySenses.Unlock() })
	return bm
}

func TestCommandAwaitKeepsBufferedReplyAfterDeadline(t *testing.T) {
	result := make(chan yyCommandReply, 1)
	output := &packet.CommandOutput{SuccessCount: 1}
	stop := make(chan struct{})
	close(stop)
	for i := 0; i < 100; i++ {
		result <- yyCommandReply{p: output}
		r, err := yyAwaitCommandReply(result, time.Now().Add(-time.Second), stop)
		if err != nil || r.p != output {
			t.Fatalf("buffered response lost: %v", err)
		}
	}
	if _, err := yyAwaitCommandReply(result, time.Now().Add(-time.Second), nil); err == nil {
		t.Fatal("missing response reported successful")
	}
	if _, err := yyAwaitCommandReply(result, time.Now().Add(time.Minute), stop); err == nil {
		t.Fatal("stopped connection reported successful")
	}
}
func senseHealth(bm *BotManager, id uint64, value float32) {
	yySenseHandle(bm, &packet.UpdateAttributes{EntityRuntimeID: id, Attributes: []protocol.Attribute{{AttributeValue: protocol.AttributeValue{Name: "minecraft:health", Value: value, Max: 20}}}})
}
func TestSenseUnknownHealthDeathDedupRespawn(t *testing.T) {
	bm := senseBot(t)
	if yySenseActor(bm, "self")["known"] != false {
		t.Fatal("invented state")
	}
	senseHealth(bm, 42, 12)
	v := yySenseActor(bm, "self")
	if *v["health"].(*float32) != 12 || v["health_known"] != true {
		t.Fatal(v)
	}
	senseHealth(bm, 42, 0)
	yySenseHandle(bm, &packet.ActorEvent{EntityRuntimeID: 42, EventType: packet.ActorEventDeath})
	s := yySenseFor(bm)
	deaths := 0
	for _, e := range s.Events {
		if e["type"] == "death" {
			deaths++
		}
	}
	if deaths != 1 {
		t.Fatal("duplicate death", deaths)
	}
	yySenseHandle(bm, &packet.Respawn{EntityRuntimeID: 42, State: packet.RespawnStateSearchingForSpawn})
	if *yySenseActor(bm, "self")["dead"].(*bool) != true {
		t.Fatal("searching is not respawn")
	}
	yySenseHandle(bm, &packet.Respawn{EntityRuntimeID: 42, State: packet.RespawnStateReadyToSpawn, Position: mgl32.Vec3{1, 2, 3}})
	v = yySenseActor(bm, "self")
	if v["health_known"] != false || *v["dead"].(*bool) {
		t.Fatal(v)
	}
	senseHealth(bm, 42, 20)
	senseHealth(bm, 42, 0)
	deaths = 0
	for _, e := range s.Events {
		if e["type"] == "death" {
			deaths++
		}
	}
	if deaths != 2 {
		t.Fatal(deaths)
	}
	yySenseReset(bm)
	if yySenseActor(bm, "self")["known"] != false {
		t.Fatal("stale session data")
	}
}
func TestSenseAbsoluteDeltaAndDimensions(t *testing.T) {
	bm := senseBot(t)
	yySenseHandle(bm, &packet.MoveActorDelta{EntityRuntimeID: 88, Flags: packet.MoveActorDeltaFlagHasX, Position: mgl32.Vec3{3, 0, 0}})
	if yySenseActor(bm, "88")["position_known"] != false {
		t.Fatal("partial unknown position invented")
	}
	yySenseHandle(bm, &packet.AddActor{EntityRuntimeID: 88, EntityUniqueID: 99, Position: mgl32.Vec3{10, 20, 30}})
	yySenseHandle(bm, &packet.MoveActorDelta{EntityRuntimeID: 88, Flags: packet.MoveActorDeltaFlagHasY, Position: mgl32.Vec3{0, 25, 0}})
	if yySenseActor(bm, "88")["position"].(mgl32.Vec3) != (mgl32.Vec3{10, 25, 30}) {
		t.Fatal("delta is absolute coordinate, not offset")
	}
	yySenseHandle(bm, &packet.ChangeDimension{Dimension: 1, Position: mgl32.Vec3{1, 2, 3}})
	if yySenseActor(bm, "88")["position_known"] != false {
		t.Fatal("old-dimension position leaked")
	}
	yySenseHandle(bm, &packet.UpdateBlock{Position: protocol.BlockPos{1, 2, 3}, NewBlockRuntimeID: 7})
	v, e := yySenseCall(bm, "world.blockState", []any{float64(1), float64(2), float64(3)})
	// Public observation copies normalize numbers through JSON; verify the
	// actual unchanged API value rather than an internal packet field type.
	if e != nil || v.(map[string]any)["runtime_id"] != float64(7) {
		t.Fatal(v, e)
	}
}
func TestSensePacketCaptureFilterSamplingAndBounds(t *testing.T) {
	bm := senseBot(t)
	_, e := yyCaptureStart(bm, yyCaptureConfig{Types: []string{"MovePlayer"}, Capacity: 2, Filter: yyPacketFilter{Fields: map[string]any{"entity_runtime_id": float64(42)}}})
	if e != nil {
		t.Fatal(e)
	}
	yyCapturePacket(bm, &packet.MovePlayer{EntityRuntimeID: 99})
	for i := 0; i < 4; i++ {
		yyCapturePacket(bm, &packet.MovePlayer{EntityRuntimeID: 42, Position: mgl32.Vec3{float32(i), 0, 0}})
	}
	v := yyCaptureRead(bm, 0, 10)
	if len(v["records"].([]map[string]any)) != 2 || v["overwritten"] != uint64(2) {
		t.Fatal(v)
	}
	if _, e = yyCaptureStart(bm, yyCaptureConfig{Types: []string{"Login"}}); e == nil {
		t.Fatal("credentials recorded")
	}
	if _, e = yyCaptureStart(bm, yyCaptureConfig{Types: []string{"TypoPacket"}}); e == nil {
		t.Fatal("unknown packet silently accepted")
	}
	_, _ = yyCaptureStart(bm, yyCaptureConfig{Types: []string{"MovePlayer"}, Filter: yyPacketFilter{EveryMS: 100}})
	yyCapturePacket(bm, &packet.MovePlayer{})
	yyCapturePacket(bm, &packet.MovePlayer{})
	if yyCaptureRead(bm, 0, 10)["sampled_out"] != uint64(1) {
		t.Fatal("sampling ineffective")
	}
}
func TestSenseAccountAndFilteredSDKCallbacks(t *testing.T) {
	bm := senseBot(t)
	other := senseBot(t)
	botMgrMu.Lock()
	old := botMgr
	botMgr = bm
	botMgrMu.Unlock()
	t.Cleanup(func() { botMgrMu.Lock(); botMgr = old; botMgrMu.Unlock() })
	s := sdkFixture(t, "sense_sdk")
	if _, e := s.call("events.onDeath", []any{"death_handler"}); e != nil {
		t.Fatal(e)
	}
	senseHealth(other, 42, 0)
	if len(s.Instance.Queue) != 0 {
		t.Fatal("other account event leaked")
	}
	senseHealth(bm, 42, 0)
	if len(s.Instance.Queue) != 1 {
		t.Fatal("death not routed")
	}
	<-s.Instance.Queue
	if _, e := s.call("events.subscribeFiltered", []any{"MovePlayer", map[string]any{"fields": map[string]any{"entity_runtime_id": float64(42)}, "every_ms": float64(100)}, "movement"}); e != nil {
		t.Fatal(e)
	}
	packetHubDispatch(bm, &packet.MovePlayer{EntityRuntimeID: 99})
	packetHubDispatch(bm, &packet.MovePlayer{EntityRuntimeID: 42})
	packetHubDispatch(bm, &packet.MovePlayer{EntityRuntimeID: 42})
	if len(s.Instance.Queue) != 1 {
		t.Fatal("filter/sample failed", len(s.Instance.Queue))
	}
	yySDKClose(s.Instance.Key)
	if packetHubUnsubscribedCount("MovePlayer") != 0 {
		t.Fatal("subscription leaked")
	}
}
func TestSenseLuaCallbacksRetainState(t *testing.T) {
	bm := senseBot(t)
	id := "sense_lua"
	if e := yyLoadResident(bm, id, "n=0;function dead(e) n=n+1 end;function read() return n end;events.onDeath('dead')", false); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { yyStopResident(bm, id) })
	senseHealth(bm, 42, 0)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		v, e := yyCallResident(bm, id, "read", nil, false, false)
		if e == nil && v == float64(1) {
			yyStopResident(bm, id)
			senseHealth(bm, 42, 20)
			senseHealth(bm, 42, 0)
			yyLuaMu.Lock()
			r := yyLuaStates[yyLuaKey{bm, id}]
			yyLuaMu.Unlock()
			if r != nil {
				t.Fatal("stopped plugin recreated")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Lua semantic callback not invoked")
}
func TestCommandWindowDispatchesBeforeWaitAndRetainsFailures(t *testing.T) {
	submitted := 0
	seen := []string{}
	rows := yyExecuteWindowed([]string{"a", "b", "c", "d", "e"}, 3, func(command string) (func() (map[string]any, error), error) {
		submitted++
		seen = append(seen, command)
		n := submitted
		return func() (map[string]any, error) {
			if n <= 3 && submitted != 3 {
				t.Fatal("waited before complete window", submitted)
			}
			if n > 3 && submitted != 5 {
				t.Fatal(submitted)
			}
			return map[string]any{"submitted": true, "confirmed": true, "success": command != "b"}, nil
		}, nil
	})
	if strings.Join(seen, "") != "abcde" || len(rows) != 5 || rows[1]["success"] != false {
		t.Fatal(rows)
	}
}
func TestAgentSkillPromptDoesNotInjectEntireManual(t *testing.T) {
	ws := &yyWorkspace{Language: "python"}
	text := yyAgentPreferencePrompt(ws)
	if len([]rune(text)) > 4500 || !strings.Contains(text, "技能") {
		t.Fatal("manual context is not bounded", len([]rune(text)))
	}
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	if L.GetField(L.GetGlobal("player"), "health").Type() != lua.LTFunction {
		t.Fatal("new methods missing Lua binding")
	}
}
func TestCapturePendingMainSettingsTransferOnce(t *testing.T) {
	pending := yySenseFor(nil)
	pending.Lock()
	old := pending.Capture
	pending.Unlock()
	t.Cleanup(func() { pending.Lock(); pending.Capture = old; pending.Unlock() })
	if _, e := yyCaptureStart(nil, yyCaptureConfig{Types: []string{"ActorEvent"}}); e != nil {
		t.Fatal(e)
	}
	if yyCaptureRead(nil, 0, 1)["waiting_for_connection"] != true {
		t.Fatal("offline capture not labelled waiting")
	}
	bm := senseBot(t)
	yyCaptureTransfer(nil, bm)
	if yyCaptureRead(bm, 0, 1)["active"] != true || yyCaptureRead(nil, 0, 1)["active"] != false {
		t.Fatal("pending config not transferred")
	}
	other := senseBot(t)
	yyCaptureTransfer(nil, other)
	if yyCaptureRead(other, 0, 1)["active"] != false {
		t.Fatal("pending config leaked to another account")
	}
}
