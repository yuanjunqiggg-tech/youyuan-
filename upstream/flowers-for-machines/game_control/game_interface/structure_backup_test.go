package game_interface

import (
	"errors"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"testing"
)

func TestStructureReceiptNeverTreatsUnknownAsSuccess(t *testing.T) {
	wire := errors.New("connection closed")
	for _, tc := range []struct {
		name    string
		reply   *packet.CommandOutput
		timeout bool
		err     error
		success bool
	}{
		{"success", &packet.CommandOutput{SuccessCount: 1}, false, nil, true},
		{"rejected", &packet.CommandOutput{}, false, nil, false},
		{"timeout", nil, true, nil, false},
		{"nil", nil, false, nil, false},
		{"disconnected", nil, false, wire, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkStructureResult(tc.reply, tc.timeout, tc.err, "structure load test 1 2 3")
			if (err == nil) != tc.success {
				t.Fatal(err)
			}
			if tc.err != nil && !errors.Is(err, wire) {
				t.Fatal("lost transport error")
			}
		})
	}
}
