package building

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"strings"
	"testing"
)

// buildCorruptNBT builds a compound body (what parseNBTCompound reads) with a
// single field of the given tagType and a bogus length.
// LevelDB block-entity NBT uses Little-Endian encoding.
func buildCorruptNBT(tagType byte, corruptLen uint32) []byte {
	var buf bytes.Buffer
	buf.WriteByte(tagType)              // field tag type
	binary.Write(&buf, binary.LittleEndian, uint16(4)) // name length = 4 (LE)
	buf.WriteString("data")             // name
	binary.Write(&buf, binary.LittleEndian, corruptLen) // bogus length (LE)
	return buf.Bytes()
}

func TestNBTCorrruptedLengthsDoNotBlowMemory(t *testing.T) {
	cases := []struct {
		name    string
		tagType byte
		corrupt uint32
	}{
		{"ByteArray", 0x07, 0xFFFFFFFF}, // 4GB
		{"IntArray", 0x0b, 0x3FFFFFFF},  // 16GB worth of int32
		{"LongArray", 0x0c, 0x1FFFFFFF}, // 32GB worth of int64
		{"List", 0x09, 0x7FFFFFFF},      // 2B items
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := buildCorruptNBT(c.tagType, c.corrupt)
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			_, err := parseNBTCompound(bytes.NewReader(data))

			runtime.GC()
			runtime.ReadMemStats(&m2)
			allocated := m2.TotalAlloc - m1.TotalAlloc

			t.Logf("err=%v allocated=%d bytes", err, allocated)
			if err == nil {
				t.Fatalf("expected error for corrupt length, got nil")
			}
			if allocated > 50_000_000 {
				t.Fatalf("ALLOCATED %d bytes! Corrupt NBT not bounded!", allocated)
			}
		})
	}
}

func TestNBTDeepNestingDoesNotStackOverflow(t *testing.T) {
	// Build a deeply nested compound structure (nested compounds)
	var buf bytes.Buffer
	// 1000 nested compounds
	for i := 0; i < 1000; i++ {
		buf.WriteByte(0x0a)   // TAG_Compound field
		buf.Write([]byte{0x00, 0x00}) // empty name
	}
	// The innermost compound body

	_, err := parseNBTCompound(bytes.NewReader(buf.Bytes()))
	t.Logf("deep nesting err=%v", err)
	if err == nil {
		t.Fatalf("expected depth-limit error for deeply nested NBT")
	}
	if !strings.Contains(err.Error(), "deep") && !strings.Contains(err.Error(), "End") {
		t.Fatalf("unexpected error: %v", err)
	}
}
