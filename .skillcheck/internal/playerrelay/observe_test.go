package playerrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/login"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func frame(p packet.Packet) []byte {
	b := new(bytes.Buffer)
	_ = (&packet.Header{PacketID: p.ID()}).Write(b)
	p.Marshal(protocol.NewWriter(b, 0))
	return b.Bytes()
}
func TestObservedInputAndRealHealth(t *testing.T) {
	o := &Observer{runtimeID: 10720}
	f := frame(&packet.Animate{ActionType: packet.AnimateActionSwingArm, EntityRuntimeID: 10720})
	original := bytes.Clone(f)
	e, err := o.Observe(f, true)
	if err != nil || len(e) != 1 || e[0]["kind"] != "swing_input" {
		t.Fatalf("%v %v", e, err)
	}
	if !bytes.Equal(f, original) {
		t.Fatal("observer modified forwarded packet")
	}
	e, err = o.Observe(frame(&packet.Animate{ActionType: 1, EntityRuntimeID: 99}), true)
	if err == nil || len(e) != 0 {
		t.Fatal("accepted forged other entity input")
	}
	p := &packet.PlayerAuthInput{InputData: protocol.NewBitset(packet.PlayerAuthInputBitsetSize), Tick: 123}
	p.InputData.Set(packet.InputFlagMissedSwing)
	e, err = o.Observe(frame(p), true)
	if err != nil || len(e) != 2 || e[1]["kind"] != "swing_input" {
		t.Fatalf("input %v %v", e, err)
	}
	a := &packet.UpdateAttributes{EntityRuntimeID: 10720, Attributes: []protocol.Attribute{{AttributeValue: protocol.AttributeValue{Name: "minecraft:health", Value: 14.14, Max: 40}}}}
	e, err = o.Observe(frame(a), false)
	if err != nil || len(e) != 1 || e[0]["data"].(map[string]any)["max"] != float32(40) {
		t.Fatalf("health %v %v", e, err)
	}
	a.EntityRuntimeID = 99
	e, err = o.Observe(frame(a), false)
	if err != nil || len(e) != 0 {
		t.Fatal("leaked other entity health as own")
	}
	if _, err = o.Observe([]byte{byte(packet.IDAnimate), 0xff}, true); err == nil {
		t.Fatal("malformed observation accepted")
	}
}
func TestIdentityBinding(t *testing.T) {
	a := login.IdentityData{Identity: "a", Uid: 42}
	b := a
	if !SameIdentity(a, b) {
		t.Fatal("same rejected")
	}
	b.Uid = 43
	if SameIdentity(a, b) {
		t.Fatal("UID mismatch accepted")
	}
	b = a
	b.Identity = "b"
	if SameIdentity(a, b) {
		t.Fatal("UUID mismatch accepted")
	}
	if SameIdentity(login.IdentityData{}, login.IdentityData{}) {
		t.Fatal("unknown identities accepted")
	}
}

func TestVersionedMetadataKeepsKnownIdentityPrefix(t *testing.T) {
	o := new(Observer)
	b := new(bytes.Buffer)
	_ = (&packet.Header{PacketID: packet.IDStartGame}).Write(b)
	w := protocol.NewWriter(b, 0)
	unique := int64(43)
	runtime := uint64(123)
	w.Varint64(&unique)
	w.Varuint64(&runtime)
	b.Write([]byte{0xff, 0xaa, 0x99})
	if _, e := o.Observe(b.Bytes(), false); e != nil || o.runtimeID != 123 {
		t.Fatalf("identity prefix %v", e)
	}
	f := frame(&packet.ItemRegistry{Items: []protocol.ItemEntry{{Name: "minecraft:shield", RuntimeID: 355}}})
	f = append(f, 0x99, 0xff)
	if _, e := o.Observe(f, false); e != nil || o.shieldID != 355 {
		t.Fatalf("registry prefix %v", e)
	}
}
func TestDeliveryRetriesSameSequenceAndPersists(t *testing.T) {
	var requests []uint64
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Youyuan-Observer-Token") != "secret" {
			t.Error("missing authentication")
		}
		var b struct {
			Sequence uint64  `json:"sequence"`
			Events   []Event `json:"events"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		requests = append(requests, b.Sequence)
		if len(requests) == 1 {
			w.WriteHeader(503)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"accepted": len(b.Events)}})
	}))
	defer h.Close()
	c := Config{SequenceFile: filepath.Join(t.TempDir(), "sequence.json"), BridgeURL: h.URL, SourceID: "test", BridgeToken: "secret"}
	d, err := newDelivery(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newDelivery(c); err == nil {
		t.Fatal("concurrent sequence owner accepted")
	}
	if n, err := d.send(context.Background(), []Event{{"kind": "swing_input"}}); n != 1 || err != nil {
		t.Fatalf("%d %v", n, err)
	}
	d.Close()
	d, err = newDelivery(c)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.sequence != 1 || len(requests) != 2 || requests[0] != 1 || requests[1] != 1 {
		t.Fatalf("sequence mismatch %d %v", d.sequence, requests)
	}
}
