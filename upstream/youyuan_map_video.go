package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Preserve browser form callers while allowing MCP JSON callers to use the
// same routes. Never silently treat an unread JSON body as all-zero positions.
func mapRequestForm(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return true
	}
	var values map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&values); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "地图请求 JSON 无效: " + err.Error()})
		return false
	}
	r.Form = url.Values{}
	r.PostForm = r.Form
	for k, v := range values {
		switch x := v.(type) {
		case string:
			r.Form.Set(k, x)
		case float64:
			r.Form.Set(k, fmt.Sprint(x))
		case bool:
			r.Form.Set(k, fmt.Sprint(x))
		}
	}
	return true
}

func handleMapStatus(w http.ResponseWriter, r *http.Request) {
	mapPlayerMu.Lock()
	player := mapPlayer
	mapPlayerMu.Unlock()
	_, ffmpegErr := exec.LookPath("ffmpeg")
	result := map[string]any{"ok": true, "ffmpeg_available": ffmpegErr == nil, "configured": false, "running": false, "server_display_confirmed": false}
	if player != nil {
		player.setupMu.Lock()
		result["configured"] = player.grid != nil
		result["frames_submitted"] = player.framesSubmitted.Load()
		result["pixels_submitted"] = player.pixelsSubmitted.Load()
		result["target_fps"] = player.mediaCfg.VideoFPS
		if s := player.session; s != nil {
			select {
			case <-s.done:
			default:
				result["running"] = true
			}
			s.mu.Lock()
			if s.err != nil {
				result["error"] = s.err.Error()
			}
			s.mu.Unlock()
		}
		player.setupMu.Unlock()
	}
	writeJSON(w, result)
}

// A decoder's dimensions are its OUTPUT canvas, never the source stream.
// The bound is a decoded-frame memory budget, not a command/CPS limit.
func mapVideoCanvas(grid *MapGridInfo) (int, int, error) {
	if grid == nil || grid.Rows < 1 || grid.Cols < 1 {
		return 0, 0, errors.New("请先扫描地图区域")
	}
	if grid.Rows > 128 || grid.Cols > 128 {
		return 0, 0, errors.New("展示框数量超出当前扫描范围，请拆分画布")
	}
	w, h := int64(grid.Cols)*128, int64(grid.Rows)*128
	if w > 16384 || h > 16384 || w*h*4 > 64<<20 {
		return 0, 0, errors.New("当前画布解码内存超过单帧64MiB，请减少展示框或拆分画布")
	}
	return int(w), int(h), nil
}

func mapVideoFilter(w, h, fps int) string {
	return fmt.Sprintf("fps=%d,scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black", fps, w, h, w, h)
}

func mapFrameDifference(info *ImageInfo, row, col int, old []color.RGBA) ([]PixelRequest, []color.RGBA) {
	next := make([]color.RGBA, 128*128)
	var delta []PixelRequest
	fast, _ := info.ScaledImage.(*image.RGBA)
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			ix, iy := col*128+x-info.OffsetX, row*128+y-info.OffsetY
			v := color.RGBA{A: 255}
			if ix >= 0 && iy >= 0 && ix < info.ContentWidth && iy < info.ContentHeight {
				b := info.ScaledImage.Bounds()
				if fast != nil {
					at := fast.PixOffset(b.Min.X+ix, b.Min.Y+iy)
					v = color.RGBA{fast.Pix[at], fast.Pix[at+1], fast.Pix[at+2], fast.Pix[at+3]}
				} else {
					r, g, b, a := info.ScaledImage.At(b.Min.X+ix, b.Min.Y+iy).RGBA()
					v = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
				}
			}
			i := y*128 + x
			next[i] = v
			if i >= len(old) || old[i] != v {
				delta = append(delta, PixelRequest{Colour: v, Index: uint16(i)})
			}
		}
	}
	return delta, next
}

type mapVideoSession struct {
	stop   chan struct{}
	frames chan *ImageInfo
	done   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
	cmd    *exec.Cmd
	output io.ReadCloser
	mu     sync.Mutex
	err    error
}

func (s *mapVideoSession) shutdown() {
	s.once.Do(func() {
		close(s.stop)
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		if s.output != nil {
			_ = s.output.Close()
		}
	})
	s.wg.Wait()
}

func (s *mapVideoSession) readFrames(w, h int) {
	defer s.wg.Done()
	defer close(s.frames)
	defer func() {
		if s.output != nil {
			_ = s.output.Close()
		}
		if s.cmd != nil {
			if err := s.cmd.Wait(); err != nil {
				select {
				case <-s.stop:
					return
				default:
				}
				s.mu.Lock()
				if s.err == nil {
					s.err = fmt.Errorf("视频解码失败：%w", err)
				}
				s.mu.Unlock()
			}
		}
	}()
	r := bufio.NewReaderSize(s.output, 64<<10)
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		_, err := io.ReadFull(r, img.Pix)
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
			}
			if err != io.EOF {
				s.mu.Lock()
				s.err = fmt.Errorf("视频帧数据不完整：%w", err)
				s.mu.Unlock()
			}
			return
		}
		info := &ImageInfo{ScaledImage: img, ContentWidth: w, ContentHeight: h, TotalWidth: w, TotalHeight: h}
		select {
		case s.frames <- info:
		case <-s.stop:
			return
		}
	}
}

func (mp *MapPlayer) PlayVideo(cfg MediaConfig) (playErr error) {
	mp.setupMu.Lock()
	mp.stopSessionLocked()
	w, h, err := mapVideoCanvas(mp.grid)
	if err != nil {
		mp.setupMu.Unlock()
		return err
	}
	if cfg.VideoFPS == 0 {
		cfg.VideoFPS = 30
	}
	if cfg.VideoSpeed == 0 {
		cfg.VideoSpeed = 1
	}
	period := float64(time.Second) / (float64(cfg.VideoFPS) * cfg.VideoSpeed)
	if cfg.VideoFPS < 1 || cfg.StartFrame < 0 || cfg.VideoSpeed <= 0 || math.IsNaN(period) || math.IsInf(period, 0) || period < 1 || period > float64(math.MaxInt64) {
		mp.setupMu.Unlock()
		return errors.New("无效的目标帧率、播放速度或起始帧")
	}
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		mp.setupMu.Unlock()
		return fmt.Errorf("未找到ffmpeg：%w", err)
	}
	args := []string{"-nostdin", "-v", "error", "-ss", fmt.Sprintf("%.6f", float64(cfg.StartFrame)/float64(cfg.VideoFPS)), "-i", cfg.MediaPath,
		"-an", "-vf", mapVideoFilter(w, h, cfg.VideoFPS), "-f", "rawvideo", "-pix_fmt", "rgba", "-"}
	cmd := exec.Command(path, args...)
	output, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		if output != nil {
			_ = output.Close()
		}
		mp.setupMu.Unlock()
		return fmt.Errorf("启动视频解码失败：%w", err)
	}
	s := &mapVideoSession{stop: make(chan struct{}), frames: make(chan *ImageInfo, 2), done: make(chan struct{}), cmd: cmd, output: output}
	mp.session, mp.mediaCfg = s, cfg
	mp.resetPixels()
	mp.framesSubmitted.Store(0)
	mp.pixelsSubmitted.Store(0)
	s.wg.Add(1)
	go s.readFrames(w, h)
	mp.setupMu.Unlock()
	defer close(s.done)
	defer s.shutdown()
	defer func() {
		if playErr != nil {
			s.mu.Lock()
			s.err = playErr
			s.mu.Unlock()
		}
	}()
	return mp.playVideoLoop(s, time.Duration(period))
}

func (mp *MapPlayer) playVideoLoop(s *mapVideoSession, period time.Duration) error {
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return nil
		case <-ticker.C:
			if mp.isPaused.Load() {
				continue
			}
			select {
			case <-s.stop:
				return nil
			case info, ok := <-s.frames:
				if !ok {
					s.mu.Lock()
					err := s.err
					s.mu.Unlock()
					return err
				}
				select {
				case <-s.stop:
					return nil
				default:
				}
				if err := mp.ProcessFrame(info); err != nil {
					return fmt.Errorf("地图像素提交失败：%w", err)
				}
			default:
			}
		}
	}
}
