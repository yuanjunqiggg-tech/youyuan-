package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVideoBatchSequenceAndAtomicRollback(t *testing.T) {
	var imageData bytes.Buffer
	png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	frame := base64.StdEncoding.EncodeToString(imageData.Bytes())
	makeJob := func() *yyBlockVideoJob {
		p := filepath.Join(t.TempDir(), "video.txt")
		os.WriteFile(p, nil, 0600)
		return &yyBlockVideoJob{Config: yyBlockVideoConfig{Name: "video.txt", Plane: "xy", FrameMS: 33, Channel: "ai"}, Path: p, Width: 2, Height: 2, Cache: map[uint32]int{}}
	}
	j := makeJob()
	result, e := j.appendImages(0, []string{frame, frame})
	if e != nil || j.Frames != 2 || result["batch_frame_commands"] != 1 {
		t.Fatal(e, result, j.Frames)
	}
	if _, e = j.appendImages(0, []string{frame}); e == nil {
		t.Fatal("duplicate sequence accepted")
	}
	before := j.Bytes
	if _, e = j.appendImages(2, []string{frame, "not base64!"}); e == nil || j.Bytes != before || j.Frames != 2 {
		t.Fatal("decode failure changed accepted prefix")
	}
	probe := makeJob()
	probe.appendImages(0, []string{frame})
	length := probe.Bytes
	j = makeJob()
	j.Bytes = (8 << 20) - length - 1
	original := j.Bytes
	os.Truncate(j.Path, int64(original))
	if _, e = j.appendImages(0, []string{frame, frame}); e == nil {
		t.Fatal("capacity failure missing")
	}
	st, _ := os.Stat(j.Path)
	if j.Frames != 0 || j.Commands != 0 || j.Previous != nil || j.Bytes != original || st.Size() != int64(original) {
		t.Fatal("failed batch left partial output", j.Frames, j.Bytes, st.Size())
	}
}

func TestVideoRectanglesPreserveEveryChangedPixel(t *testing.T) {
	rng := rand.New(rand.NewSource(58))
	for trial := 0; trial < 60; trial++ {
		w, h := 31, 19
		cells, previous := make([]int, w*h), make([]int, w*h)
		for i := range cells {
			cells[i] = rng.Intn(4)
			previous[i] = rng.Intn(4)
		}
		drawn := append([]int{}, previous...)
		touch := make([]int, len(cells))
		for _, r := range yyVideoRectangles(cells, previous, w, h) {
			for y := r.Y; y < r.Y+r.H; y++ {
				for x := r.X; x < r.X+r.W; x++ {
					i := y*w + x
					drawn[i] = r.Color
					touch[i]++
				}
			}
		}
		for i, v := range cells {
			if drawn[i] != v || touch[i] > 1 || (v == previous[i] && touch[i] != 0) {
				t.Fatalf("trial%d pixel%d", trial, i)
			}
		}
	}
}
func TestVideoUniformFrameIsOneFillRatherThan128Rows(t *testing.T) {
	c := make([]int, 128*128)
	rs := yyVideoRectangles(c, nil, 128, 128)
	if len(rs) != 1 || rs[0].W*rs[0].H != len(c) {
		t.Fatal(rs)
	}
	big := yyVideoRectangles(make([]int, 512*128), nil, 512, 128)
	if len(big) != 2 {
		t.Fatal("fill volume boundary", big)
	}
	for i, c := range yyVideoColors {
		if got := yyVideoNearest(c[0], c[1], c[2]); got != i {
			t.Fatalf("palette exact colour %d mapped to %d", i, got)
		}
	}
}
func TestVideoFrameWaitIsPerFrameWithoutServerProbe(t *testing.T) {
	plan, e := yyParseChains("#组:video 通道=ai 速率=0\n1,0,0\nsetblock 0 0 0 stone\n@frame {\"duration_ms\":25}\nsetblock 1 0 0 stone\n@frame {\"duration_ms\":25}\n")
	if e != nil {
		t.Fatal(e)
	}
	sent := 0
	start := time.Now()
	e = yyChainExecuteGraph(context.Background(), plan, yyChainConfig{Mode: "fast"}, nil, nil, func(context.Context, yyChainNode, yyChainConfig) (bool, error) {
		t.Fatal("unexpected server probe")
		return false, nil
	}, func(_ context.Context, n []yyChainNode, _ yyChainConfig) ([]map[string]any, error) {
		out := []map[string]any{}
		for range n {
			sent++
			out = append(out, map[string]any{"submitted": true, "success": true})
		}
		return out, nil
	}, func() bool { return false }, func(string) {}, func(bool, bool, error) {})
	if e != nil || sent != 2 || time.Since(start) < 45*time.Millisecond {
		t.Fatal(e, sent, time.Since(start))
	}
}
func BenchmarkVideoRectanglesUniform128(b *testing.B) {
	c := make([]int, 128*128)
	for i := 0; i < b.N; i++ {
		yyVideoRectangles(c, nil, 128, 128)
	}
}
