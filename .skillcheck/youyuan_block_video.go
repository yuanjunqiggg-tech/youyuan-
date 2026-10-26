package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type yyBlockVideoConfig struct {
	Name     string `json:"name"`
	Plane    string `json:"plane"`
	A        [3]int `json:"a"`
	B        [3]int `json:"b"`
	FrameMS  int    `json:"frame_ms"`
	Channel  string `json:"channel"`
	RunCount int    `json:"run_count"`
}
type yyBlockVideoJob struct {
	sync.Mutex
	Config                                 yyBlockVideoConfig
	Path                                   string
	Width, Height, Frames, Commands, Bytes int
	Previous                               []int
	Cache                                  map[uint32]int
	Updated                                time.Time
}

var yyBlockJobs = struct {
	sync.Mutex
	Items map[string]*yyBlockVideoJob
}{Items: map[string]*yyBlockVideoJob{}}
var yyVideoColors = [][3]int{{207, 213, 214}, {224, 97, 1}, {169, 48, 159}, {35, 137, 199}, {241, 175, 21}, {94, 169, 24}, {214, 101, 143}, {54, 57, 61}, {125, 125, 115}, {21, 119, 136}, {100, 32, 156}, {45, 47, 143}, {96, 60, 32}, {73, 91, 36}, {142, 33, 33}, {8, 10, 15}}
var yyVideoBlocks = []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}

func yyVideoBounds(c yyBlockVideoConfig) (int, int, error) {
	if _, e := yyChainName(c.Name); e != nil {
		return 0, 0, e
	}
	if c.FrameMS < 1 || c.FrameMS > 60000 {
		return 0, 0, fmt.Errorf("帧间隔必须为 1..60000ms；实际帧率取决于服务端处理能力")
	}
	if _, e := normalizeCommandChannel(c.Channel); e != nil {
		return 0, 0, e
	}
	if c.RunCount < 0 || c.RunCount > 1000000 {
		return 0, 0, fmt.Errorf("运行次数无效")
	}
	for i := 0; i < 3; i++ {
		if c.A[i] < -30000000 || c.A[i] > 30000000 || c.B[i] < -30000000 || c.B[i] > 30000000 {
			return 0, 0, fmt.Errorf("坐标超出世界范围")
		}
	}
	horizontal, vertical, fixed := 0, 1, 2
	switch c.Plane {
	case "xy":
	case "xz":
		horizontal, vertical, fixed = 0, 2, 1
	case "yz":
		horizontal, vertical, fixed = 2, 1, 0
	default:
		return 0, 0, fmt.Errorf("plane 必须为 xy/xz/yz")
	}
	if c.A[fixed] != c.B[fixed] {
		return 0, 0, fmt.Errorf("选定平面的固定轴坐标必须相等")
	}
	w, h := abs(c.B[horizontal]-c.A[horizontal])+1, abs(c.B[vertical]-c.A[vertical])+1
	if w > 512 || h > 512 || w*h > 65536 {
		return 0, 0, fmt.Errorf("请缩小范围：单边最多 512，单帧最多 65536 方块")
	}
	return w, h, nil
}
func (j *yyBlockVideoJob) point(col, row int) [3]int {
	lo, hi := j.Config.A, j.Config.B
	for i := 0; i < 3; i++ {
		if lo[i] > hi[i] {
			lo[i], hi[i] = hi[i], lo[i]
		}
	}
	switch j.Config.Plane {
	case "xz":
		return [3]int{lo[0] + col, lo[1], lo[2] + row}
	case "yz":
		return [3]int{lo[0], hi[1] - row, lo[2] + col}
	default:
		return [3]int{lo[0] + col, hi[1] - row, lo[2]}
	}
}
func (j *yyBlockVideoJob) appendFrame(img image.Image) (map[string]any, error) {
	bounds := img.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 {
		return nil, fmt.Errorf("画面为空")
	}
	cells := make([]int, j.Width*j.Height)
	for y := 0; y < j.Height; y++ {
		for x := 0; x < j.Width; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x*bounds.Dx()/j.Width, bounds.Min.Y+y*bounds.Dy()/j.Height).RGBA()
			rr, gg, bb := int(r>>8), int(g>>8), int(b>>8)
			key := uint32(rr)<<16 | uint32(gg)<<8 | uint32(bb)
			index, ok := j.Cache[key]
			if !ok {
				index = yyVideoNearest(rr, gg, bb)
				if len(j.Cache) >= 65536 {
					j.Cache = map[uint32]int{}
				}
				j.Cache[key] = index
			}
			cells[y*j.Width+x] = index
		}
	}
	var out strings.Builder
	count := 0
	changed := 0
	if j.Frames == 0 {
		fmt.Fprintln(&out, "#组:视频逐帧 通道="+j.Config.Channel+" 速率=0\n1,0,0")
	}
	for _, r := range yyVideoRectangles(cells, j.Previous, j.Width, j.Height) {
		a, b := j.point(r.X, r.Y), j.point(r.X+r.W-1, r.Y+r.H-1)
		block := "minecraft:" + yyVideoBlocks[r.Color] + "_concrete"
		if r.W*r.H == 1 {
			fmt.Fprintf(&out, "setblock %d %d %d %s\n", a[0], a[1], a[2], block)
		} else {
			fmt.Fprintf(&out, "fill %d %d %d %d %d %d %s\n", a[0], a[1], a[2], b[0], b[1], b[2], block)
		}
		count++
		changed += r.W * r.H
	}
	fmt.Fprintf(&out, "@frame {\"duration_ms\":%d}\n", j.Config.FrameMS)
	if j.Bytes+out.Len() > 8<<20 || j.Frames >= 6000 {
		return nil, fmt.Errorf("输出已到安全容量，请缩短片段、增大抽帧间隔或缩小范围；已有帧可完成导入")
	}
	f, e := os.OpenFile(j.Path, os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	_, e = f.WriteString(out.String())
	closeErr := f.Close()
	if e != nil {
		return nil, e
	}
	if closeErr != nil {
		return nil, closeErr
	}
	j.Previous = cells
	j.Commands += count + 1
	j.Frames++
	j.Bytes += out.Len()
	j.Updated = time.Now()
	return map[string]any{"frames": j.Frames, "commands": j.Commands, "changed_blocks": changed, "frame_commands": count, "minimum_frame_ms": j.Config.FrameMS, "requested_frame_ms": j.Config.FrameMS, "bytes": j.Bytes}, nil
}
func yyBlockVideoRoutes(mux *http.ServeMux, dataDir string) {
	root := filepath.Join(dataDir, "conversion-jobs")
	_ = os.MkdirAll(root, 0700)
	mux.HandleFunc("/api/youyuan/media/block-video", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Action, ID, Image string
			Sequence          int                `json:"sequence"`
			Images            []string           `json:"images"`
			Config            yyBlockVideoConfig `json:"config"`
		}
		fail := func(e error) { writeJSON(w, map[string]any{"ok": false, "error": e.Error()}) }
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&p); e != nil {
			fail(e)
			return
		}
		if p.Action == "create" {
			width, height, e := yyVideoBounds(p.Config)
			if e != nil {
				fail(e)
				return
			}
			yyBlockJobs.Lock()
			defer yyBlockJobs.Unlock()
			for id, j := range yyBlockJobs.Items {
				if !j.TryLock() {
					continue
				}
				expired := time.Since(j.Updated) > 30*time.Minute
				if expired {
					_ = os.Remove(j.Path)
					delete(yyBlockJobs.Items, id)
				}
				j.Unlock()
			}
			if len(yyBlockJobs.Items) >= 2 {
				fail(fmt.Errorf("已有两个转换任务，请先完成或取消"))
				return
			}
			id := yyNewKey()
			path := filepath.Join(root, id+".txt")
			if e = os.WriteFile(path, nil, 0600); e != nil {
				fail(e)
				return
			}
			yyBlockJobs.Items[id] = &yyBlockVideoJob{Config: p.Config, Width: width, Height: height, Path: path, Cache: map[uint32]int{}, Updated: time.Now()}
			writeJSON(w, map[string]any{"ok": true, "id": id, "width": width, "height": height})
			return
		}
		yyBlockJobs.Lock()
		j := yyBlockJobs.Items[p.ID]
		yyBlockJobs.Unlock()
		if j == nil {
			fail(fmt.Errorf("转换任务不存在/已取消"))
			return
		}
		j.Lock()
		defer j.Unlock()
		switch p.Action {
		case "append_batch":
			result, e := j.appendImages(p.Sequence, p.Images)
			if e != nil {
				fail(e)
				return
			}
			result["ok"] = true
			writeJSON(w, result)
		case "append":
			if p.Sequence != j.Frames {
				fail(fmt.Errorf("帧序号应为 %d；不要并发或重复提交", j.Frames))
				return
			}
			encoded := p.Image
			if at := strings.Index(encoded, ","); at >= 0 {
				encoded = encoded[at+1:]
			}
			raw, e := base64.StdEncoding.DecodeString(encoded)
			if e != nil {
				fail(e)
				return
			}
			config, _, e := image.DecodeConfig(bytes.NewReader(raw))
			if e != nil || config.Width < 1 || config.Height < 1 || config.Width*config.Height > 65536 {
				fail(fmt.Errorf("请在前端先将画面缩放至所选范围（最多 65536 像素）"))
				return
			}
			img, _, e := image.Decode(bytes.NewReader(raw))
			if e != nil {
				fail(e)
				return
			}
			result, e := j.appendFrame(img)
			if e != nil {
				fail(e)
				return
			}
			result["ok"] = true
			writeJSON(w, result)
		case "finish":
			if j.Frames == 0 {
				fail(fmt.Errorf("尚未提交画面"))
				return
			}
			f, e := os.Open(j.Path)
			if e != nil {
				fail(e)
				return
			}
			raw, e := io.ReadAll(io.LimitReader(f, (8<<20)+1))
			f.Close()
			if e != nil {
				fail(e)
				return
			}
			result, e := yyChainResult("POST", map[string]any{"action": "save", "name": j.Config.Name, "content": string(raw), "config": yyChainConfig{Name: j.Config.Name, Account: "main", Channel: j.Config.Channel, IntervalMS: 1, DelayMS: 0, Mode: "fast", Scheduling: "file", RunCount: j.Config.RunCount}}, nil)
			if e != nil {
				fail(e)
				return
			}
			yyBlockJobs.Lock()
			delete(yyBlockJobs.Items, p.ID)
			yyBlockJobs.Unlock()
			_ = os.Remove(j.Path)
			writeJSON(w, map[string]any{"ok": true, "name": j.Config.Name, "frames": j.Frames, "commands": j.Commands, "bytes": j.Bytes, "result": result, "state": "已导入，尚未启动"})
		case "cancel":
			yyBlockJobs.Lock()
			delete(yyBlockJobs.Items, p.ID)
			yyBlockJobs.Unlock()
			_ = os.Remove(j.Path)
			writeJSON(w, map[string]any{"ok": true})
		default:
			fail(fmt.Errorf("未知操作"))
		}
	})
}
