package protocol

import (
	"bytes"
	"testing"
)

func TestAllocationBoundsRemainEnabledForClientReaders(t *testing.T) {
	for _, fn := range []struct {
		name string
		run  func(*Reader)
	}{
		{"string", func(r *Reader) { var v string; r.String(&v) }},
		{"bytes", func(r *Reader) { var v []byte; r.ByteSlice(&v) }},
		{"slice", func(r *Reader) { r.SliceLimit(0xffffffff, 1024) }},
	} {
		t.Run(fn.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("oversized allocation was not rejected")
				}
			}()
			fn.run(NewReader(bytes.NewBuffer([]byte{0xff, 0xff, 0xff, 0x3f}), 0, false))
		})
	}
}

func TestReaderRejectsTruncatedBytesAndKeepsValidPayload(t *testing.T) {
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("truncated bytes accepted")
			}
		}()
		var v []byte
		NewReader(bytes.NewBuffer([]byte{20, 1}), 0, false).ByteSlice(&v)
	}()
	var v []byte
	NewReader(bytes.NewBuffer([]byte{3, 1, 2, 3}), 0, false).ByteSlice(&v)
	if !bytes.Equal(v, []byte{1, 2, 3}) {
		t.Fatal(v)
	}
}

func TestComplexSliceAllocationRejectedBeforeMake(t *testing.T) {
	r := NewReader(bytes.NewBuffer(nil), 0, false)
	type bigElement [4096]byte
	var values []bigElement
	defer func() {
		if recover() == nil {
			t.Fatal("256 MiB array was not rejected before allocation")
		}
		if values != nil {
			t.Fatal("allocated rejected array")
		}
	}()
	FuncSliceOfLen(r, 65536, &values, func(*bigElement) {})
}

func TestSliceBudgetIsCumulativeAndClientListsAbove1024StillWork(t *testing.T) {
	r := NewReader(bytes.NewBuffer(make([]byte, 1500)), 0, false)
	var values []byte
	FuncSliceOfLen(r, 1500, &values, r.Uint8)
	if len(values) != 1500 {
		t.Fatal(len(values))
	}
	r.SliceAllocation(1, 20<<20)
	defer func() {
		if recover() == nil {
			t.Fatal("nested allocation budget bypassed")
		}
	}()
	r.SliceAllocation(1, 20<<20)
}
