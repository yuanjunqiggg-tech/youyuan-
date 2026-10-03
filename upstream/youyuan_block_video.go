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
	if c.FrameMS < 100 || c.FrameMS > 60000 {
		return 0, 0, fmt.Errorf("帧间隔必须为 100..60000ms")
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
			key := uint32(rr>>3)<<10 | uint32(gg>>3)<<5 | uint32(bb>>3)
			index, ok := j.Cache[key]
			if !ok {
				best := int(^uint(0) >> 1)
				for i, c := range yyVideoColors {
					dr, dg, db := rr-c[0], gg-c[1], bb-c[2]
					d := dr*dr + dg*dg + db*db
					if d < best {
						best, index = d, i
					}
				}
				j.Cache[key] = index
			}
			cells[y*j.Width+x] = index
		}
	}
	var out strings.Builder
	count := 0
	changed := 0
	fmt.Fprintf(&out, "#组:视频帧%d 通道=%s 速率=20\n1,0,0\n", j.Frames+1, j.Config.Channel)
	for y := 0; y < j.Height; y++ {
		for x := 0; x < j.Width; {
			i := y*j.Width + x
			if j.Previous != nil && j.Previous[i] == cells[i] {
				x++
				continue
			}
			end := x
			for end+1 < j.Width && cells[y*j.Width+end+1] == cells[i] && (j.Previous == nil || j.Previous[y*j.Width+end+1] != cells[i]) {
				end++
			}
			a, b := j.point(x, y), j.point(end, y)
			block := "minecraft:" + yyVideoBlocks[cells[i]] + "_concrete"
			if x == end {
				fmt.Fprintf(&out, "setblock %d %d %d %s\n", a[0], a[1], a[2], block)
			} else {
				fmt.Fprintf(&out, "fill %d %d %d %d %d %d %s\n", a[0], a[1], a[2], b[0], b[1], b[2], block)
			}
			count++
			changed += end - x + 1
			x = end + 1
		}
	}
	if count == 0 {
		out.Reset()
	}
	wait := j.Config.FrameMS - count*20
	if wait < 20 {
		wait = 20
	}
	fmt.Fprintf(&out, "#组:视频等待%d 通道=%s 速率=%d\n1,0,0\ntestfor @s\n", j.Frames+1, j.Config.Channel, wait)
	if j.Commands+count+1 > 32768 || j.Bytes+out.Len() > 8<<20 || j.Frames >= 6000 {
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
	return map[string]any{"frames": j.Frames, "commands": j.Commands, "changed_blocks": changed, "frame_commands": count, "minimum_frame_ms": count*20 + wait, "requested_frame_ms": j.Config.FrameMS, "bytes": j.Bytes}, nil
}
func yyBlockVideoRoutes(mux *http.ServeMux, dataDir string) {
	root := filepath.Join(dataDir, "conversion-jobs")
	_ = os.MkdirAll(root, 0700)
	mux.HandleFunc("/api/youyuan/media/block-video", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Action, ID, Image string
			Sequence          int                `json:"sequence"`
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
			result, e := yyChainResult("POST", map[string]any{"action": "save", "name": j.Config.Name, "content": string(raw), "config": yyChainConfig{Name: j.Config.Name, Account: "main", Channel: j.Config.Channel, IntervalMS: 100, DelayMS: 20, RunCount: j.Config.RunCount}}, nil)
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
