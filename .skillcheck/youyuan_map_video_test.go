package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMapMCPJSONPreservesValues(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/map/play", strings.NewReader(`{"fps":30,"speed":1.5,"path":"/tmp/视频.mp4","x1":-304}`))
	r.Header.Set("Content-Type", "application/json")
	if !mapRequestForm(httptest.NewRecorder(), r) || r.FormValue("fps") != "30" || r.FormValue("path") != "/tmp/视频.mp4" || r.FormValue("x1") != "-304" {
		t.Fatal("MCP parameters lost", r.Form)
	}
	r = httptest.NewRequest("POST", "/api/map/scan", strings.NewReader(`{"x1":`))
	r.Header.Set("Content-Type", "application/json")
	if mapRequestForm(httptest.NewRecorder(), r) {
		t.Fatal("malformed JSON silently accepted")
	}
}

func TestMapVideoReadsOutputCanvasAndEnds(t *testing.T) {
	// Source size is deliberately irrelevant. The decoder output is 128x128.
	pixels := bytes.Repeat([]byte{23, 45, 67, 255}, 128*128*2)
	s := &mapVideoSession{stop: make(chan struct{}), frames: make(chan *ImageInfo, 2), output: io.NopCloser(bytes.NewReader(pixels))}
	s.wg.Add(1)
	go s.readFrames(128, 128)
	n := 0
	for f := range s.frames {
		n++
		if f.TotalWidth != 128 || f.TotalHeight != 128 || f.ScaledImage.At(127, 127) != (color.RGBA{23, 45, 67, 255}) {
			t.Fatal("corrupt canvas frame", f)
		}
	}
	s.wg.Wait()
	if n != 2 || s.err != nil {
		t.Fatalf("frames=%d err=%v", n, s.err)
	}
}

func TestMapVideoTruncatedOutputReportsError(t *testing.T) {
	s := &mapVideoSession{stop: make(chan struct{}), frames: make(chan *ImageInfo, 2), output: io.NopCloser(bytes.NewReader(make([]byte, 127)))}
	s.wg.Add(1)
	go s.readFrames(128, 128)
	for range s.frames {
		t.Fatal("partial frame emitted")
	}
	s.wg.Wait()
	if s.err == nil {
		t.Fatal("partial frame silently accepted")
	}
}

func TestMapVideoStopClosesBlockedPipeBeforeWait(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()
	s := &mapVideoSession{stop: make(chan struct{}), frames: make(chan *ImageInfo, 2), output: r}
	s.wg.Add(1)
	go s.readFrames(128, 128)
	done := make(chan struct{})
	go func() { s.shutdown(); s.shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop waited before unblocking decoder")
	}
}

func TestMapVideoEOFStopsPlayback(t *testing.T) {
	mp := NewMapPlayer(nil)
	s := &mapVideoSession{stop: make(chan struct{}), frames: make(chan *ImageInfo)}
	close(s.frames)
	done := make(chan error, 1)
	go func() { done <- mp.playVideoLoop(s, time.Millisecond) }()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("EOF left playback alive")
	}
}

func TestMapVideoDeltaAndFailedSubmissionRetry(t *testing.T) {
	mp := NewMapPlayer(nil)
	mp.grid = &MapGridInfo{Rows: 1, Cols: 1, MapIDs: [][]int64{{73}}}
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	info := &ImageInfo{ScaledImage: img, ContentWidth: 128, ContentHeight: 128, TotalWidth: 128, TotalHeight: 128}
	var mu sync.Mutex
	counts := []int{}
	fail := false
	mp.sendPixels = func(id int64, p []PixelRequest) error {
		mu.Lock()
		defer mu.Unlock()
		if id != 73 {
			t.Errorf("map=%d", id)
		}
		counts = append(counts, len(p))
		if fail {
			return errors.New("injected send failure")
		}
		return nil
	}
	if e := mp.ProcessFrame(info); e != nil {
		t.Fatal(e)
	}
	if e := mp.ProcessFrame(info); e != nil {
		t.Fatal(e)
	}
	img.SetRGBA(2, 3, color.RGBA{255, 0, 0, 255})
	fail = true
	if e := mp.ProcessFrame(info); e == nil {
		t.Fatal("send error hidden")
	}
	fail = false
	if e := mp.ProcessFrame(info); e != nil {
		t.Fatal(e)
	}
	if len(counts) != 3 || counts[0] != 16384 || counts[1] != 1 || counts[2] != 1 {
		t.Fatalf("submitted pixels %v", counts)
	}
}

func TestMapVideoCanvasAndFilter(t *testing.T) {
	w, h, e := mapVideoCanvas(&MapGridInfo{Rows: 3, Cols: 4})
	if e != nil || w != 512 || h != 384 {
		t.Fatal(w, h, e)
	}
	if _, _, e = mapVideoCanvas(&MapGridInfo{Rows: 128, Cols: 128}); e == nil {
		t.Fatal("decoded memory overflow accepted")
	}
	f := mapVideoFilter(w, h, 30)
	if !strings.Contains(f, "fps=30") || !strings.Contains(f, "scale=512:384") || !strings.Contains(f, "pad=512:384") {
		t.Fatal(f)
	}
}

func TestMapVideoPaddingIsClearedAndRGBAOffsetHandled(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 12, 12))
	base.SetRGBA(5, 6, color.RGBA{8, 9, 10, 255})
	sub := base.SubImage(image.Rect(5, 6, 7, 8))
	info := &ImageInfo{ScaledImage: sub, ContentWidth: 2, ContentHeight: 2, OffsetX: 1, OffsetY: 1, TotalWidth: 128, TotalHeight: 128}
	delta, next := mapFrameDifference(info, 0, 0, nil)
	if len(delta) != 16384 || next[129] != (color.RGBA{8, 9, 10, 255}) || next[0] != (color.RGBA{A: 255}) {
		t.Fatal("padding or subimage origin corrupted")
	}
}

var mapVideoPixelSink []PixelRequest

func BenchmarkMapVideoStaticTile(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	info := &ImageInfo{ScaledImage: img, ContentWidth: 128, ContentHeight: 128}
	_, old := mapFrameDifference(info, 0, 0, nil)
	b.Run("legacy_full_pixels", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			mapVideoPixelSink = extractPixels(info, 0, 0, true)
		}
	})
	b.Run("native_difference", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			mapVideoPixelSink, _ = mapFrameDifference(info, 0, 0, old)
		}
	})
}
