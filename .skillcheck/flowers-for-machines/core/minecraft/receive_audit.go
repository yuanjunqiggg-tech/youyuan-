package minecraft

// Count after batch decryption/decompression, before semantic decoding and
// application listeners. Retain no payload, address, key, login or RPC data.
import (
	"sort"
	"sync"
	"sync/atomic"
)

type receiveAuditEntry struct {
	frames          atomic.Uint64
	bytes           atomic.Uint64
	unknown         atomic.Uint64
	decodeErrors    atomic.Uint64
	decodePanics    atomic.Uint64
	decodedFrames   atomic.Uint64
	trailingFrames  atomic.Uint64
	returnedPackets atomic.Uint64
}
type receiveAudit struct {
	entries      sync.Map // protocol packet ID -> *receiveAuditEntry
	headerErrors atomic.Uint64
}

func (a *receiveAudit) entry(id uint32) *receiveAuditEntry {
	if e, ok := a.entries.Load(id); ok {
		return e.(*receiveAuditEntry)
	}
	e, _ := a.entries.LoadOrStore(id, &receiveAuditEntry{})
	return e.(*receiveAuditEntry)
}
func (a *receiveAudit) parsed(id uint32, length int, known bool) {
	e := a.entry(id)
	e.frames.Add(1)
	e.bytes.Add(uint64(length))
	if !known {
		e.unknown.Add(1)
	}
}

// ReceiveDiagnostics reports the negotiated client transport and actual
// received frame metadata. It does not infer server distance/OP broadcasts.
func (conn *Conn) ReceiveDiagnostics() map[string]any {
	rows := []map[string]any{}
	conn.receiveAudit.entries.Range(func(key, value any) bool {
		e := value.(*receiveAuditEntry)
		rows = append(rows, map[string]any{"packet_id": key, "parsed_frames": e.frames.Load(), "payload_bytes": e.bytes.Load(), "unknown_to_registry": e.unknown.Load(), "decode_errors": e.decodeErrors.Load(),
			"decode_panics": e.decodePanics.Load(), "decoded_frames": e.decodedFrames.Load(), "trailing_bytes_frames": e.trailingFrames.Load(), "returned_packets": e.returnedPackets.Load()})
		return true
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i]["packet_id"].(uint32) < rows[j]["packet_id"].(uint32) })
	ids := make([]uint32, 0, len(conn.pool))
	for id := range conn.pool {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return map[string]any{"protocol_id": conn.proto.ID(), "protocol_version": conn.proto.Ver(),
		"batch_packet_limit_disabled": !conn.readerLimits, "reader_limits_enabled": conn.readerLimits,
		"disconnect_on_unknown": conn.disconnectOnUnknownPacket, "disconnect_on_invalid": conn.disconnectOnInvalidPacket,
		"registered_packet_ids": ids, "registered_packet_count": len(ids), "received": rows, "header_errors": conn.receiveAudit.headerErrors.Load(),
		"scope":            "parsed game frames after decryption/decompression, before packet decode and application filtering; excludes failed batch decryption/decompression and raw UDP loss",
		"payload_retained": false, "server_broadcast_override": false}
}
