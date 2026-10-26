package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func awaitProtocolSample(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return data
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("sample not written: %s", path)
	return nil
}

func TestProtocolSamplesWhitelistOnceAndSizeLimit(t *testing.T) {
	if yyProtocolMetadataSampler("") != nil {
		t.Fatal("sampling enabled without explicit directory")
	}
	root := t.TempDir()
	capture := yyProtocolMetadataSampler(root)
	for _, id := range []uint32{packet.IDLogin, packet.IDText, packet.IDPyRpc} {
		capture(packet.Header{PacketID: id}, []byte("private"), nil, nil)
	}
	files, _ := os.ReadDir(root)
	if len(files) != 0 {
		t.Fatal("private packet sampled")
	}
	first := []byte{1, 2, 3}
	capture(packet.Header{PacketID: packet.IDGameRulesChanged}, first, nil, nil)
	first[0] = 9
	capture(packet.Header{PacketID: packet.IDGameRulesChanged}, []byte("second"), nil, nil)
	got := awaitProtocolSample(t, filepath.Join(root, fmt.Sprintf("%d.bin", packet.IDGameRulesChanged)))
	if !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("sample overwritten or alias retained: %v", got)
	}
	capture(packet.Header{PacketID: packet.IDBiomeDefinitionList}, make([]byte, yyProtocolSampleLimit+1), nil, nil)
	awaitProtocolSample(t, filepath.Join(root, fmt.Sprintf("%d.oversized", packet.IDBiomeDefinitionList)))
	if _, err := os.Stat(filepath.Join(root, fmt.Sprintf("%d.bin", packet.IDBiomeDefinitionList))); !os.IsNotExist(err) {
		t.Fatal("oversized payload retained")
	}
}
