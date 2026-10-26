package protocol

import (
	"bytes"
	"os"
	"testing"
)

func TestNetEaseActualGameRules20261023(t *testing.T) {
	wire, err := os.ReadFile("testdata/netease_20261023_gamerules.bin")
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) != 728 || wire[0] != 0x48 {
		t.Fatal("wrong fixture frame")
	}
	r := NewReader(bytes.NewBuffer(wire[1:]), 0, true)
	rules, ok := ReadGameRulesAdaptive(r)
	if !ok || len(rules) != 38 || r.BufferedLen() != 0 {
		t.Fatalf("real rules not decoded: ok=%v count=%d remaining=%d", ok, len(rules), r.BufferedLen())
	}
	values := map[string]any{}
	for _, rule := range rules {
		values[rule.Name] = rule.Value
	}
	if values["maxCommandChainLength"] != uint32(65535) || values["randomTickSpeed"] != uint32(1) || values["doInsomnia"] != true {
		t.Fatalf("integer value or following rule misaligned: %+v", values)
	}
}

