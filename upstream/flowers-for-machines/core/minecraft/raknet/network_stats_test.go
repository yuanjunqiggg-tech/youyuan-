package raknet

import (
	"testing"
	"time"
)

func TestNetworkStatsReportsQueueWithoutExposingPayload(t *testing.T) {
	c := &Conn{retransmission: newRecoveryQueue(), datagramsSent: 12, datagramsResent: 2, datagramsAcknowledged: 9}
	c.retransmission.add(1, &packet{content: []byte{1, 2, 3}})
	c.rtt.Store(int64(90 * time.Millisecond))
	s := c.NetworkStats()
	if s["unacknowledged_datagrams"] != 1 || s["unacknowledged_bytes"] != 3 || s["rtt_ms"] != float64(90) || s["resent_datagrams"] != uint64(2) {
		t.Fatal(s)
	}
	if len(c.retransmission.unacknowledged) != 1 {
		t.Fatal("read mutated retransmission state")
	}
}
