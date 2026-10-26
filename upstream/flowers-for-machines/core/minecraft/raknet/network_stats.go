package raknet

import "time"

func (conn *Conn) NetworkStats() map[string]any {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	bytes := 0
	var oldest time.Duration
	now := time.Now()
	for _, r := range conn.retransmission.unacknowledged {
		bytes += len(r.pk.content)
		if age := now.Sub(r.timestamp); age > oldest {
			oldest = age
		}
	}
	return map[string]any{"available": true, "transport": "raknet", "sent_datagrams": conn.datagramsSent, "resent_datagrams": conn.datagramsResent, "acknowledged_datagrams": conn.datagramsAcknowledged, "unacknowledged_datagrams": len(conn.retransmission.unacknowledged), "unacknowledged_bytes": bytes, "oldest_unacknowledged_ms": oldest.Milliseconds(), "rtt_ms": float64(conn.rtt.Load()) / float64(time.Millisecond)}
}
