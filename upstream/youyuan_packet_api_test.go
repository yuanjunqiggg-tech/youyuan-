package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

func TestPacketConstructionPrecisionAndValidation(t *testing.T) {
	p, err := yyMakePacket("Animate", map[string]any{"action_type": 1, "entity_runtime_id": "18446744073709551615"})
	if err != nil {
		t.Fatal(err)
	}
	if p.(*packet.Animate).EntityRuntimeID != ^uint64(0) {
		t.Fatal("64-bit runtime ID lost precision")
	}
	p, err = yyMakePacket("RequestChunkRadius", map[string]any{"chunk_radius": 4, "max_chunk_radius": 8})
	if err != nil || p.(*packet.RequestChunkRadius).ChunkRadius != 4 {
		t.Fatal(p, err)
	}
	for _, fields := range []map[string]any{{"misspelled_radius": 4}, {"chunk_radius": "2147483648"}, {"chunk_radius": 1.5}, {"chunk_radius": []any{4}}, {"payload_hex": "ff", "chunk_radius": 4}} {
		if _, err = yyMakePacket("RequestChunkRadius", fields); err == nil {
			t.Fatal("invalid packet accepted", fields)
		}
	}
	if _, err = yyMakePacket("Login", map[string]any{}); err == nil {
		t.Fatal("session negotiation packet allowed")
	}
	if _, err = yyMakePacket("NotARealPacket", map[string]any{}); err == nil {
		t.Fatal("invented packet accepted")
	}
	if _, err = yySendPacket(nil, []any{"RequestChunkRadius", map[string]any{"chunk_radius": 4}}); err == nil {
		t.Fatal("offline send reported success")
	}
}

func TestObservedSwingSpawnPickupDoesNotInventDeath(t *testing.T) {
	bm := senseBot(t)
	yySenseHandle(bm, &packet.AddActor{EntityRuntimeID: 88, EntityUniqueID: 99, EntityType: "minecraft:pig", Position: mgl32.Vec3{1, 2, 3}})
	yySenseHandle(bm, &packet.Animate{EntityRuntimeID: 88, ActionType: packet.AnimateActionSwingArm})
	yySenseHandle(bm, &packet.AddItemActor{EntityRuntimeID: 90, EntityUniqueID: 100, Position: mgl32.Vec3{1, 2, 3}})
	yySenseHandle(bm, &packet.TakeItemActor{ItemEntityRuntimeID: 90, TakerEntityRuntimeID: 88})
	yySenseHandle(bm, &packet.RemoveActor{EntityUniqueID: 99})
	seen := map[string]int{}
	for _, e := range yySenseFor(bm).Events {
		seen[e["type"].(string)]++
	}
	if seen["entity_spawn"] != 2 || seen["swing"] != 1 || seen["item_pickup"] != 1 || seen["entity_remove"] != 1 || seen["death"] != 0 {
		t.Fatal(seen)
	}
}

func TestDocumentationChaptersAndExactPacketSkill(t *testing.T) {
	mux := http.NewServeMux()
	yyDocumentationRoutes(mux)
	for _, language := range []string{"lua-runtime", "python-runtime", "cpp-runtime"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/youyuan/docs?language="+language+"&method=game.sendPacket", nil))
		var response map[string]any
		if json.Unmarshal(w.Body.Bytes(), &response) != nil || response["ok"] != true || !strings.Contains(response["content"].(string), "sendPacket") {
			t.Fatal(language, w.Body.String())
		}
		if _, e := yyAgentSkillText(language, "events.md", 0, 6000); e != nil {
			t.Fatal(language, e)
		}
	}
	doc, err := yyAgentSkillText("packet-reference", "Animate", 0, 20000)
	if err != nil || !strings.Contains(doc, "挥臂") || strings.Contains(doc, "## 21 ·") {
		t.Fatal(doc, err)
	}
	if _, err = yyAgentSkillText("packet-reference", "InventedPacket", 0, 500); err == nil {
		t.Fatal("invented packet has skill")
	}
}

func TestTaskProgressUsesOwnBrandAndRealCounters(t *testing.T) {
	for _, kind := range []string{"建筑写入", "建筑读取", "区域准备"} {
		text := yyTaskActionbar(kind, "测试建筑", "b", 7, 12, 2, time.Second)
		if !strings.Contains(text, "游园 · "+kind) || !strings.Contains(text, "7 §7/ 12") || strings.Contains(text, "%") || strings.Contains(text, "Prism") || strings.Contains(text, "Importer") {
			t.Fatal(text)
		}
	}
	text := yyTaskActionbar("建筑读取", "测试", "6", 5, 10, 0, 0)
	if strings.Contains(text, "完成") {
		t.Fatal("partial read looks complete", text)
	}
}
