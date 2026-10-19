package minecraft

import (
	"bytes"
	"encoding/binary"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"io"
	"log/slog"
	"math"
	"sync"
	"testing"
)

func TestReceiveAuditCountsBeforeDecode(t *testing.T) {
	c := &Conn{proto: proto{}, pool: packet.Pool{}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	b := bytes.NewBuffer(nil)
	h := packet.Header{PacketID: 1000}
	if e := h.Write(b); e != nil {
		t.Fatal(e)
	}
	b.Write([]byte{1, 2, 3})
	p, e := parseData(b.Bytes(), c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.decode(c); e != nil {
		t.Fatal(e)
	}
	r := c.ReceiveDiagnostics()
	rows := r["received"].([]map[string]any)
	if len(rows) != 1 || rows[0]["parsed_frames"] != uint64(1) || rows[0]["unknown_to_registry"] != uint64(1) || rows[0]["payload_bytes"] != uint64(3) || r["batch_packet_limit_disabled"] != true || r["payload_retained"] != false {
		t.Fatal(r)
	}
	if _, e = parseData([]byte{0x80}, c); e == nil {
		t.Fatal("bad header accepted")
	}
	if c.ReceiveDiagnostics()["header_errors"] != uint64(1) {
		t.Fatal("missing header failure")
	}
}

func auditPacket(t *testing.T, id uint32, payload []byte) (*Conn, []packet.Packet, error) {
	t.Helper()
	c := &Conn{proto: proto{}, pool: packet.Pool{
		packet.IDAnimate:   func() packet.Packet { return &packet.Animate{} },
		packet.IDSetHealth: func() packet.Packet { return &packet.SetHealth{} },
	}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	b := bytes.NewBuffer(nil)
	h := packet.Header{PacketID: id}
	if err := h.Write(b); err != nil {
		t.Fatal(err)
	}
	b.Write(payload)
	p, err := parseData(b.Bytes(), c)
	if err != nil {
		t.Fatal(err)
	}
	pks, err := p.decode(c)
	return c, pks, err
}

func TestAnimate121120WireAndAudit(t *testing.T) {
	// Independent wire fixture: zigzag action=1, unsigned runtime=4303,
	// little-endian animation data=1.5. Matches the observed 7-byte layout.
	wire := []byte{2, 0xcf, 0x21, 0, 0, 0xc0, 0x3f}
	c, pks, err := auditPacket(t, packet.IDAnimate, wire)
	if err != nil || len(pks) != 1 {
		t.Fatalf("packets=%v err=%v", pks, err)
	}
	pk := pks[0].(*packet.Animate)
	if pk.ActionType != 1 || pk.EntityRuntimeID != 4303 || pk.Data != 1.5 {
		t.Fatal(pk)
	}
	r := c.ReceiveDiagnostics()["received"].([]map[string]any)[0]
	if r["decode_errors"] != uint64(0) || r["decode_panics"] != uint64(0) || r["trailing_bytes_frames"] != uint64(0) || r["decoded_frames"] != uint64(1) || r["returned_packets"] != uint64(1) {
		t.Fatal(r)
	}
	// Outbound swing packets also require Data, rather than a truncated legacy write.
	b := bytes.NewBuffer(nil)
	pk.Marshal(c.proto.NewWriter(b, 0))
	if !bytes.Equal(b.Bytes(), wire) {
		t.Fatalf("wire=%x want=%x", b.Bytes(), wire)
	}
}

func TestAnimateRowingAndTruncation(t *testing.T) {
	for _, action := range []byte{0, 2} { // zigzag encodings of 128 and 129
		wire := []byte{0x80 | action, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0}
		binary.LittleEndian.PutUint32(wire[7:], math.Float32bits(0.25))
		_, pks, err := auditPacket(t, packet.IDAnimate, wire)
		if err != nil || len(pks) != 1 {
			t.Fatalf("%x: %v", wire, err)
		}
		pk := pks[0].(*packet.Animate)
		if pk.BoatRowingTime != 0.25 {
			t.Fatal(pk)
		}
	}
	c, pks, err := auditPacket(t, packet.IDAnimate, []byte{2, 1})
	r := c.ReceiveDiagnostics()["received"].([]map[string]any)[0]
	if err == nil || len(pks) != 0 || r["decode_panics"] != uint64(1) || r["decoded_frames"] != uint64(0) || r["returned_packets"] != uint64(0) {
		t.Fatalf("%v %v %v", pks, err, r)
	}
}

func TestReceiveAuditExtensionIsNotDroppedPacket(t *testing.T) {
	// SetHealth=20 followed by an unknown extension byte. Lenient delivery
	// remains compatible, while diagnostics distinguish it from a panic.
	c, pks, err := auditPacket(t, packet.IDSetHealth, []byte{40, 0xaa})
	r := c.ReceiveDiagnostics()["received"].([]map[string]any)[0]
	if err != nil || len(pks) != 1 || r["decode_errors"] != uint64(1) || r["decode_panics"] != uint64(0) || r["decoded_frames"] != uint64(1) || r["trailing_bytes_frames"] != uint64(1) || r["returned_packets"] != uint64(1) {
		t.Fatalf("%v %v %v", pks, err, r)
	}
}
func TestReceiveAuditMalformedAndConcurrentCounters(t *testing.T) {
	c := &Conn{proto: proto{}, pool: packet.Pool{packet.IDSetHealth: func() packet.Packet { return &packet.SetHealth{} }}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	b := bytes.NewBuffer(nil)
	h := packet.Header{PacketID: packet.IDSetHealth}
	h.Write(b)
	p, e := parseData(b.Bytes(), c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.decode(c); e == nil {
		t.Fatal("truncated packet accepted")
	}
	var w sync.WaitGroup
	for i := 0; i < 8; i++ {
		w.Add(1)
		go func() {
			defer w.Done()
			for j := 0; j < 1000; j++ {
				c.receiveAudit.parsed(1000, 2, false)
			}
		}()
	}
	w.Wait()
	r := c.ReceiveDiagnostics()
	rows := r["received"].([]map[string]any)
	if len(rows) != 2 || rows[0]["decode_errors"] != uint64(1) || rows[1]["parsed_frames"] != uint64(8000) {
		t.Fatal(r)
	}
}
