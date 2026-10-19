package main

import (
	"bot-apk/building"
	"image"
	"image/color"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBlockVideoDeltaAndPlanes(t *testing.T) {
	for _, plane := range []string{"xy", "xz", "yz"} {
		c := yyBlockVideoConfig{Name: "video.txt", Plane: plane, A: [3]int{10, 20, 30}, B: [3]int{11, 21, 30}, FrameMS: 500, Channel: "ai", RunCount: 1}
		if plane == "xz" {
			c.B = [3]int{11, 20, 31}
		}
		if plane == "yz" {
			c.B = [3]int{10, 21, 31}
		}
		w, h, e := yyVideoBounds(c)
		if e != nil || w != 2 || h != 2 {
			t.Fatal(w, h, e)
		}
		path := filepath.Join(t.TempDir(), "video.txt")
		_ = os.WriteFile(path, nil, 0600)
		j := &yyBlockVideoJob{Config: c, Width: w, Height: h, Path: path, Cache: map[uint32]int{}}
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		for i := range 4 {
			img.Set(i%2, i/2, color.RGBA{R: 207, G: 213, B: 214, A: 255})
		}
		a, e := j.appendFrame(img)
		// A uniform 2x2 image is one exact rectangle, no longer two rows.
		if e != nil || a["changed_blocks"] != 4 || a["frame_commands"] != 1 {
			t.Fatal(a, e)
		}
		b, e := j.appendFrame(img)
		if e != nil || b["changed_blocks"] != 0 || b["frame_commands"] != 0 {
			t.Fatal(b, e)
		}
		img.Set(0, 0, color.Black)
		_, e = j.appendFrame(img)
		if e != nil {
			t.Fatal(e)
		}
		text, _ := os.ReadFile(path)
		plan, e := yyParseChains(string(text))
		if e != nil || plan.Commands != 5 {
			t.Fatal(plan, e, string(text))
		}
		if strings.Contains(string(text), "titleraw") {
			t.Fatal("screen text instead of blocks")
		}
	}
	_, _, e := yyVideoBounds(yyBlockVideoConfig{Name: "a.txt", Plane: "xy", A: [3]int{}, B: [3]int{512, 512, 1}, FrameMS: 100, Channel: "ai"})
	if e == nil {
		t.Fatal("non-planar bounds accepted")
	}
}
func TestStructureConvertRoundTrip(t *testing.T) {
	s := &building.StructureData{SizeX: 2, SizeY: 2, SizeZ: 2, Palette: []building.BlockInfo{{Name: "minecraft:air", States: "[]"}}, Blocks: map[uint64]uint32{}}
	s.Set(1, 0, 1, &building.BlockInfo{Name: "minecraft:stone", States: "[]"})
	s.Set(0, 1, 0, &building.BlockInfo{Name: "minecraft:oak_log", States: `["pillar_axis"="x"]`})
	raw, e := yyStructureEncode(s)
	if e != nil {
		t.Fatal(e)
	}
	round, e := yyParseConversion(raw, ".mcstructure")
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range [][3]int{{1, 0, 1}, {0, 1, 0}} {
		a, b := s.Get(p[0], p[1], p[2]), round.Get(p[0], p[1], p[2])
		if a == nil || b == nil || a.Name != b.Name || a.States != b.States {
			t.Fatal(a, b)
		}
	}
}
func TestQQReplyWindowQuotaAndExpiry(t *testing.T) {
	yyQQReplies.Lock()
	old := yyQQReplies.Items
	yyQQReplies.Items = map[string]*yyQQReplyWindow{}
	yyQQReplies.Unlock()
	defer func() { yyQQReplies.Lock(); yyQQReplies.Items = old; yyQQReplies.Unlock() }()
	if _, _, e := yyQQReserveReply("group", ""); e == nil {
		t.Fatal("active push accepted")
	}
	yyQQRememberReply("group", "message")
	for n := 1; n <= 5; n++ {
		id, seq, e := yyQQReserveReply("group", "")
		if e != nil || id != "message" || seq != n {
			t.Fatal(id, seq, e)
		}
	}
	if _, _, e := yyQQReserveReply("group", "message"); e == nil {
		t.Fatal("quota exceeded")
	}
	yyQQRememberReply("group", "expired")
	yyQQReplies.Lock()
	yyQQReplies.Items["expired"].Until = time.Now().Add(-time.Second)
	yyQQReplies.Unlock()
	if _, _, e := yyQQReserveReply("group", "expired"); e == nil {
		t.Fatal("expired accepted")
	}
	if _, _, e := yyQQReserveReply("other", "message"); e == nil {
		t.Fatal("cross-group reply accepted")
	}
}
func TestPluginChainsSaveConfigureAndIsolation(t *testing.T) {
	oldMux, oldRoot := yyMux, yyChainRoot
	yyChains.Lock()
	old := yyChains.items
	yyChains.items = map[string]*yyChainInstance{}
	yyChains.Unlock()
	defer func() { yyMux, yyChainRoot = oldMux, oldRoot; yyChains.Lock(); yyChains.items = old; yyChains.Unlock() }()
	yyMux = http.NewServeMux()
	yyCommandRoutes(yyMux, t.TempDir())
	bm := getBotManagerLocked()
	_, e := yyChainPluginCall(bm, "chains.save", []any{"managed.txt", "1,0,0\ntestfor @s", map[string]any{"run_count": float64(2)}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = yyChainPluginCall(bm, "chains.configure", []any{"managed.txt", map[string]any{"interval_ms": float64(777)}})
	if e != nil {
		t.Fatal(e)
	}
	r, e := yyChainPluginCall(bm, "chains.get", []any{"managed.txt"})
	if e != nil {
		t.Fatal(e)
	}
	v := r.(map[string]any)
	if v["config"].(map[string]any)["interval_ms"] != float64(777) || !strings.Contains(v["content"].(string), "testfor @s") {
		t.Fatal(v)
	}
	_, e = yyChainPluginCall(bm, "chains.configure", []any{"managed.txt", map[string]any{"account": "other"}})
	if e == nil {
		t.Fatal("account override accepted")
	}
	_, e = yyChainPluginCall(bm, "chains.save", []any{"../bad.txt", "say hi"})
	if e == nil {
		t.Fatal("path escape accepted")
	}
	_, e = yyChainPluginCall(&BotManager{}, "chains.delete", []any{"managed.txt"})
	if e == nil {
		t.Fatal("cross account delete accepted")
	}
}
