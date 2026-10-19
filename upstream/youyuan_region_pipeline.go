package main

import (
	"bytes"
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/nbt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type yyRegionTile struct {
	Index    int      `json:"index"`
	Offset   [3]int32 `json:"offset"`
	Position [3]int32 `json:"position"`
	Size     [3]int32 `json:"size"`
}

func yyRegionVector(v any, positive bool) ([3]int32, error) {
	var out [3]int32
	var a []any
	switch x := v.(type) {
	case []any:
		a = x
	case map[string]any:
		a = []any{x["x"], x["y"], x["z"]}
	default:
		return out, fmt.Errorf("需要三维数组或{x,y,z}")
	}
	if len(a) != 3 {
		return out, fmt.Errorf("需要三项")
	}
	lo := int64(math.MinInt32)
	if positive {
		lo = 1
	}
	for i := 0; i < 3; i++ {
		n, e := yyNBTBoundedInteger(a[i], lo, math.MaxInt32)
		if e != nil {
			return out, e
		}
		out[i] = int32(n)
	}
	return out, nil
}
func yyRegionArgs(args []any) ([3]int32, [3]int32, error) {
	var p, s [3]int32
	if len(args) < 2 {
		return p, s, fmt.Errorf("需要起点与尺寸")
	}
	var e error
	p, e = yyRegionVector(args[0], false)
	if e != nil {
		return p, s, e
	}
	s, e = yyRegionVector(args[1], true)
	if e != nil {
		return p, s, e
	}
	for i := 0; i < 3; i++ {
		if int64(p[i])+int64(s[i])-1 > math.MaxInt32 {
			return p, s, fmt.Errorf("区域坐标溢出")
		}
	}
	// The existing encoder materializes all block indices. Bound that allocation
	// using a memory budget, not command CPS or the number of blocks per chain.
	volume := int64(s[0])
	for i := 1; i < 3; i++ {
		if volume > math.MaxInt64/int64(s[i]) {
			return p, s, fmt.Errorf("区域体积溢出")
		}
		volume *= int64(s[i])
	}
	if volume > 16*1024*1024 {
		return p, s, fmt.Errorf("整区合并超过16777216格内存预算，请分区导出；此限制不涉及命令执行频率")
	}
	return p, s, nil
}
func yyRegionTiles(p, s [3]int32) []yyRegionTile {
	rows := []yyRegionTile{}
	for x := int32(0); x < s[0]; x += 16 {
		for y := int32(0); y < s[1]; y += 128 {
			for z := int32(0); z < s[2]; z += 16 {
				off := [3]int32{x, y, z}
				pos := [3]int32{p[0] + x, p[1] + y, p[2] + z}
				size := [3]int32{min(16, s[0]-x), min(128, s[1]-y), min(16, s[2]-z)}
				rows = append(rows, yyRegionTile{len(rows), off, pos, size})
			}
		}
	}
	return rows
}
func yyRegionPlan(p, s [3]int32) any {
	rows := yyRegionTiles(p, s)
	return map[string]any{"origin": p, "size": s, "tile_size": [3]int{16, 128, 16}, "tiles": rows, "total": len(rows), "same_bot_interactions": "serial", "parallel_requires": "独立同服机器人；一台机器人不能同时位于多个远端工作台"}
}
func yyRegionWorkers(main *BotManager, wanted int) []*BotManager {
	bots := []*BotManager{main}
	if wanted < 2 || main == nil {
		return bots
	}
	// Fleet is explicitly configured by the owner. Do not silently enlist other
	// independent account workspaces or log in a second copy of an account.
	main.mu.Lock()
	dimension := main.dimension
	main.mu.Unlock()
	for _, bm := range GetFleetBots() {
		if bm != nil && bm != main && bm.IsConnected() && bm.ServerCode() == main.ServerCode() {
			bm.mu.Lock()
			sameDimension := bm.dimension == dimension
			bm.mu.Unlock()
			if !sameDimension {
				continue
			}
			bots = append(bots, bm)
			if len(bots) >= wanted {
				break
			}
		}
	}
	return bots
}
func yyReadRegionTiles(main *BotManager, p, s [3]int32, wanted int, stop <-chan struct{}, progress func(int, int)) ([]map[string]any, int, error) {
	tiles := yyRegionTiles(p, s)
	bots := yyRegionWorkers(main, max(1, wanted))
	if len(bots) > len(tiles) {
		bots = bots[:len(tiles)]
	}
	out := make([]map[string]any, len(tiles))
	jobs := make(chan yyRegionTile)
	abort := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	var first error
	done := 0
	fail := func(e error) {
		mu.Lock()
		if first == nil {
			first = e
		}
		mu.Unlock()
		once.Do(func() { close(abort) })
	}
	var wg sync.WaitGroup
	for _, bm := range bots {
		releaseSpatial := yyBeginSpatialJob(bm, "region-export")
		wg.Add(1)
		go func(bot *BotManager, release func()) {
			defer release()
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					fail(fmt.Errorf("分区工作线程异常: %v", p))
				}
			}()
			for tile := range jobs {
				select {
				case <-stop:
					fail(fmt.Errorf("导出取消"))
					return
				case <-abort:
					return
				default:
				}
				data, err := func() (data map[string]any, err error) {
					lock := yyBlockOperationLock(bot)
					lock.Lock()
					defer lock.Unlock()
					for attempt := 0; attempt < 2; attempt++ {
						if !bot.IsConnected() {
							err = fmt.Errorf("机器人断线")
							break
						}
						err = bot.TP(int(tile.Position[0]+tile.Size[0]/2), int(tile.Position[1]+tile.Size[1]/2), int(tile.Position[2]+tile.Size[2]/2))
						if err != nil {
							break
						}
						time.Sleep(200 * time.Millisecond)
						data, err = bot.RequestStructure(tile.Position, tile.Size, 8*time.Second)
						if err == nil {
							break
						}
						select {
						case <-stop:
							err = fmt.Errorf("导出取消")
							attempt = 2
						case <-abort:
							attempt = 2
						default:
						}
					}
					return data, err
				}()
				if err != nil {
					fail(fmt.Errorf("单元%d %v: %w", tile.Index, tile.Position, err))
					return
				}
				got := anyToIntSlice(data["size"])
				if len(got) < 3 || got[0] != int(tile.Size[0]) || got[1] != int(tile.Size[1]) || got[2] != int(tile.Size[2]) {
					fail(fmt.Errorf("单元%d返回尺寸不符，拒绝输出缺块文件", tile.Index))
					return
				}
				data["_yy_offset"] = tile.Offset
				data["_yy_size"] = tile.Size
				out[tile.Index] = data
				mu.Lock()
				done++
				if progress != nil {
					progress(done, len(tiles))
				}
				mu.Unlock()
			}
		}(bm, releaseSpatial)
	}
send:
	for _, tile := range tiles {
		select {
		case jobs <- tile:
		case <-stop:
			fail(fmt.Errorf("导出取消"))
			break send
		case <-abort:
			break send
		}
	}
	close(jobs)
	wg.Wait()
	if first != nil {
		return nil, len(bots), first
	}
	return out, len(bots), nil
}
func yyExportRegionJob(bm *BotManager, args []any) (any, error) {
	p, s, e := yyRegionArgs(args)
	if e != nil {
		return nil, e
	}
	workers := 1
	if len(args) > 2 {
		opts, ok := args[2].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("options须为对象")
		}
		if v := opts["workers"]; v != nil {
			n, e := yyNBTBoundedInteger(v, 1, 65536)
			if e != nil {
				return nil, e
			}
			workers = int(n)
		}
	}
	return yyStartNBTJob(bm, func() (any, error) {
		chunks, used, e := yyReadRegionTiles(bm, p, s, workers, nil, nil)
		if e != nil {
			return nil, e
		}
		merged := mergeChunkStructures(chunks, s[0], s[1], s[2])
		if merged == nil {
			return nil, fmt.Errorf("无法合并区域")
		}
		var b bytes.Buffer
		e = nbt.NewEncoderWithEncoding(&b, nbt.LittleEndian).Encode(map[string]any{"format_version": int32(1), "size": []int32{s[0], s[1], s[2]}, "structure_world_origin": []int32{p[0], p[1], p[2]}, "structure": merged})
		if e != nil {
			return nil, e
		}
		root := filepath.Join(filepath.Dir(yyRuntimeRoot), "exports")
		if yyRuntimeRoot == "" {
			return nil, fmt.Errorf("宿主存储目录未初始化")
		}
		if e = os.MkdirAll(root, 0700); e != nil {
			return nil, e
		}
		path := filepath.Join(root, "region-"+fmt.Sprint(time.Now().UnixNano())+".mcstructure")
		if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
			return nil, e
		}
		return map[string]any{"path": path, "bytes": b.Len(), "tiles": len(chunks), "workers_requested": workers, "workers_used": used, "nbt_preserved": true, "entities_exported": false, "format": "mcstructure"}, nil
	})
}
