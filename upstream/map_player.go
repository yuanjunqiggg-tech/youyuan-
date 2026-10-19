package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/disintegration/imaging"
)

// ========== 类型定义 ==========

type PixelRequest struct {
	Colour color.RGBA
	Index  uint16
}

type ImageInfo struct {
	ScaledImage   image.Image
	ContentWidth  int
	ContentHeight int
	OffsetX       int
	OffsetY       int
	TotalWidth    int
	TotalHeight   int
}

type MapGridInfo struct {
	Rows      int
	Cols      int
	MapIDs    [][]int64
	PlaneType string
}

// ========== MapPlayer ==========

type MapPlayer struct {
	bm              *BotManager
	grid            *MapGridInfo
	mapCfg          MapConfig
	mediaCfg        MediaConfig
	hasCleared      bool
	isPaused        atomic.Bool
	setupMu         sync.Mutex
	frameMu         sync.Mutex
	cacheMu         sync.Mutex
	session         *mapVideoSession
	previous        map[int64][]color.RGBA
	sendPixels      func(int64, []PixelRequest) error
	framesSubmitted atomic.Uint64
	pixelsSubmitted atomic.Uint64
}

type MapConfig struct {
	StartPos     [3]int32
	EndPos       [3]int32
	XDirection   int
	ZDirection   int
	LockMap      bool
	IsConfigured bool
}

type MediaConfig struct {
	MediaType   string
	MediaPath   string
	VideoFPS    int
	VideoSpeed  float64
	StartFrame  int
	Concurrency int
	BatchSize   int
}

func NewMapPlayer(bm *BotManager) *MapPlayer {
	return &MapPlayer{
		bm:       bm,
		previous: make(map[int64][]color.RGBA),
	}
}

// ========== 扫描物品展示框 ==========

func (mp *MapPlayer) ScanItemFrames(x1, y1, z1, x2, y2, z2 int32) (*MapGridInfo, error) {
	minX, maxX := minI32(x1, x2), maxI32(x1, x2)
	minY, maxY := minI32(y1, y2), maxI32(y1, y2)
	minZ, maxZ := minI32(z1, z2), maxI32(z1, z2)

	planeType := "horizontal"
	if minY == maxY {
		planeType = "horizontal"
	} else if minX == maxX {
		planeType = "x_fixed"
	} else if minZ == maxZ {
		planeType = "z_fixed"
	} else {
		return nil, errors.New("区域不是二维平面，需要至少一个坐标相等")
	}

	// TP 确保区块加载
	centerX := (minX + maxX) / 2
	centerY := (minY + maxY) / 2
	centerZ := (minZ + maxZ) / 2
	_ = mp.bm.TP(int(centerX), int(centerY+2), int(centerZ))
	time.Sleep(2 * time.Second)

	frames, err := mp.scanViaStructure(minX, minY, minZ, maxX, maxY, maxZ)
	if err != nil {
		return nil, fmt.Errorf("扫描物品框失败: %w", err)
	}
	if len(frames) == 0 {
		return nil, errors.New("区域内未找到地图物品框")
	}

	// 计算网格
	xSet := map[int32]struct{}{}
	zSet := map[int32]struct{}{}
	_ = zSet
	for _, f := range frames {
		xSet[f.posX] = struct{}{}
	}

	ySet := map[int32]struct{}{}
	for _, f := range frames {
		ySet[f.posY] = struct{}{}
	}

	zSet2 := map[int32]struct{}{}
	for _, f := range frames {
		zSet2[f.posZ] = struct{}{}
	}

	mapRows, mapCols := 1, 1
	switch planeType {
	case "horizontal":
		mapRows = len(zSet2)
		mapCols = len(xSet)
	case "x_fixed":
		mapRows = len(ySet)
		mapCols = len(zSet2)
	case "z_fixed":
		mapRows = len(ySet)
		mapCols = len(xSet)
	}

	sort.Slice(frames, func(i, j int) bool {
		switch planeType {
		case "horizontal":
			if frames[i].posZ != frames[j].posZ {
				return frames[i].posZ < frames[j].posZ
			}
			return frames[i].posX < frames[j].posX
		case "x_fixed":
			if frames[i].posY != frames[j].posY {
				return frames[i].posY > frames[j].posY
			}
			return frames[i].posZ < frames[j].posZ
		case "z_fixed":
			if frames[i].posY != frames[j].posY {
				return frames[i].posY > frames[j].posY
			}
			return frames[i].posX < frames[j].posX
		}
		return false
	})

	mapIDs := make([][]int64, mapRows)
	for r := range mapIDs {
		mapIDs[r] = make([]int64, mapCols)
	}
	for i, f := range frames {
		r := i / mapCols
		c := i % mapCols
		if r < mapRows && c < mapCols {
			mapIDs[r][c] = f.mapID
		}
	}

	grid := &MapGridInfo{
		Rows:      mapRows,
		Cols:      mapCols,
		MapIDs:    mapIDs,
		PlaneType: planeType,
	}
	mp.grid = grid
	mp.mapCfg.IsConfigured = true
	return grid, nil
}

type itemFrameInfo struct {
	posX  int32
	posY  int32
	posZ  int32
	mapID int64
}

func (mp *MapPlayer) scanViaStructure(minX, minY, minZ, maxX, maxY, maxZ int32) ([]*itemFrameInfo, error) {
	gi := mp.bm.gameInterface
	if gi == nil {
		return nil, errors.New("game interface 不可用")
	}

	sizeX := maxX - minX + 1
	sizeY := maxY - minY + 1
	sizeZ := maxZ - minZ + 1
	if sizeX <= 0 || sizeY <= 0 || sizeZ <= 0 {
		return nil, errors.New("无效区域")
	}
	if sizeX > 128 || sizeY > 128 || sizeZ > 128 {
		return nil, fmt.Errorf("区域过大 %dx%dx%d，最大 128", sizeX, sizeY, sizeZ)
	}

	backup := gi.StructureBackup()
	pos := protocol.BlockPos{minX, minY, minZ}
	offset := protocol.BlockPos{sizeX - 1, sizeY - 1, sizeZ - 1}

	id, err := backup.BackupOffset(pos, offset)
	if err != nil {
		return nil, fmt.Errorf("structure backup 失败: %w", err)
	}
	defer backup.DeleteStructure(id)
	if err := gi.Commands().AwaitChangesGeneral(); err != nil {
		return nil, fmt.Errorf("等待保存失败: %w", err)
	}

	// 监听 StructureTemplateDataResponse
	respCh := make(chan *packet.StructureTemplateDataResponse, 1)
	errCh := make(chan error, 1)

	li, err := gi.PacketListener().ListenPacket(
		[]uint32{packet.IDStructureTemplateDataResponse},
		func(pk packet.Packet, connErr error) {
			if connErr != nil {
				select {
				case errCh <- connErr:
				default:
				}
				return
			}
			if resp, ok := pk.(*packet.StructureTemplateDataResponse); ok {
				select {
				case respCh <- resp:
				default:
				}
			}
		},
	)
	if err != nil {
		return nil, fmt.Errorf("注册监听器失败: %w", err)
	}
	defer gi.PacketListener().DestroyListener(li)

	structureName := strings.ReplaceAll(id.String(), "-", "")
	_ = mp.bm.WritePacket(&packet.StructureTemplateDataRequest{
		StructureName: structureName,
		Position:      protocol.BlockPos{minX, minY, minZ},
		Settings: protocol.StructureSettings{
			PaletteName:               "default",
			IgnoreEntities:            true,
			IgnoreBlocks:              false,
			Size:                      protocol.BlockPos{sizeX, sizeY, sizeZ},
			Offset:                    protocol.BlockPos{0, 0, 0},
			LastEditingPlayerUniqueID: 0,
			Rotation:                  0,
			Mirror:                    0,
			Integrity:                 100,
			Seed:                      0,
			AllowNonTickingChunks:     false,
		},
		RequestType: packet.StructureTemplateRequestExportFromSave,
	})

	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	select {
	case resp := <-respCh:
		if !resp.Success {
			return nil, errors.New("结构请求返回失败")
		}
		return parseFramesFromNBT(resp.StructureTemplate, minX, minY, minZ, maxX, maxY, maxZ)
	case err := <-errCh:
		return nil, fmt.Errorf("监听器错误: %w", err)
	case <-timer.C:
		return nil, errors.New("结构请求超时")
	}
}

func parseFramesFromNBT(template map[string]interface{}, minX, minY, minZ, maxX, maxY, maxZ int32) ([]*itemFrameInfo, error) {
	var frames []*itemFrameInfo

	entities, _ := template["block_entities"].([]interface{})
	if entities == nil {
		// 尝试 palette 路径
		if palette, ok := template["palette"].([]interface{}); ok {
			for _, p := range palette {
				if pm, ok := p.(map[string]interface{}); ok {
					if be, ok := pm["block_entities"].([]interface{}); ok {
						entities = be
						break
					}
				}
			}
		}
	}

	for _, raw := range entities {
		be, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := be["id"].(string)
		if !strings.HasSuffix(id, "item_frame") && !strings.HasSuffix(id, "glow_item_frame") {
			continue
		}
		x, xok := numToInt32(be["x"])
		y, yok := numToInt32(be["y"])
		z, zok := numToInt32(be["z"])
		if !xok || !yok || !zok {
			continue
		}
		if x < minX || x > maxX || y < minY || y > maxY || z < minZ || z > maxZ {
			continue
		}
		item, ok := be["Item"].(map[string]interface{})
		if !ok {
			continue
		}
		itemName, _ := item["Name"].(string)
		if !strings.HasSuffix(itemName, "filled_map") {
			continue
		}
		tag, ok := item["tag"].(map[string]interface{})
		if !ok {
			continue
		}
		mapID, ok := numToInt64(tag["map_uuid"])
		if !ok {
			continue
		}
		frames = append(frames, &itemFrameInfo{posX: x, posY: y, posZ: z, mapID: mapID})
	}
	return frames, nil
}

func numToInt32(v interface{}) (int32, bool) {
	switch val := v.(type) {
	case int32:
		return val, true
	case int64:
		return int32(val), true
	case int:
		return int32(val), true
	case float64:
		return int32(val), true
	default:
		return 0, false
	}
}

func numToInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case int32:
		return int64(val), true
	case int:
		return int64(val), true
	case float64:
		return int64(val), true
	default:
		return 0, false
	}
}

// ========== 地图操作 ==========

func (mp *MapPlayer) LockMap(mapID int64) error {
	return mp.bm.WritePacket(&packet.MapCreateLockedCopy{
		OriginalMapID: mapID, NewMapID: mapID,
	})
}

func (mp *MapPlayer) SendMapPixels(mapID int64, pixels []PixelRequest) error {
	cp := make([]protocol.PixelRequest, len(pixels))
	for i, p := range pixels {
		cp[i] = protocol.PixelRequest{Colour: p.Colour, Index: p.Index}
	}
	return mp.bm.WritePacket(&packet.MapInfoRequest{
		MapID: mapID, ClientPixels: cp,
	})
}

// ========== 图片处理 ==========

func (mp *MapPlayer) LoadAndResizeImage(path string) (*ImageInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开图片失败: %w", err)
	}
	defer f.Close()

	orig, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	totalW := mp.grid.Cols * 128
	totalH := mp.grid.Rows * 128
	scaled := imaging.Fit(orig, totalW, totalH, imaging.Lanczos)
	bounds := scaled.Bounds()

	return &ImageInfo{
		ScaledImage:   scaled,
		ContentWidth:  bounds.Dx(),
		ContentHeight: bounds.Dy(),
		OffsetX:       (totalW - bounds.Dx()) / 2,
		OffsetY:       (totalH - bounds.Dy()) / 2,
		TotalWidth:    totalW,
		TotalHeight:   totalH,
	}, nil
}

func (mp *MapPlayer) ProcessFrame(info *ImageInfo) error {
	mp.frameMu.Lock()
	defer mp.frameMu.Unlock()
	if mp.grid == nil || info == nil || info.ScaledImage == nil || len(mp.grid.MapIDs) != mp.grid.Rows {
		return errors.New("地图画布或帧数据无效")
	}
	for _, row := range mp.grid.MapIDs {
		if len(row) != mp.grid.Cols {
			return errors.New("展示框网格不完整，请重新扫描")
		}
	}
	conc := mp.mediaCfg.Concurrency
	if conc <= 0 {
		conc = 4
	}
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error
	for r := 0; r < mp.grid.Rows; r++ {
		for c := 0; c < mp.grid.Cols; c++ {
			mid := mp.grid.MapIDs[r][c]
			wg.Add(1)
			sem <- struct{}{}
			go func(row, col int, id int64) {
				defer wg.Done()
				defer func() { <-sem }()
				mp.cacheMu.Lock()
				previous := mp.previous[id]
				mp.cacheMu.Unlock()
				pixels, next := mapFrameDifference(info, row, col, previous)
				var err error
				if len(pixels) > 0 {
					if mp.mapCfg.LockMap && previous == nil {
						err = mp.LockMap(id)
					}
					if err == nil {
						if mp.sendPixels != nil {
							err = mp.sendPixels(id, pixels)
						} else {
							err = mp.SendMapPixels(id, pixels)
						}
					}
				}
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					return
				}
				mp.pixelsSubmitted.Add(uint64(len(pixels)))
				mp.cacheMu.Lock()
				if mp.previous == nil {
					mp.previous = make(map[int64][]color.RGBA)
				}
				mp.previous[id] = next
				mp.cacheMu.Unlock()
			}(r, c, mid)
		}
	}
	wg.Wait()
	if firstErr == nil {
		mp.hasCleared = true
		mp.framesSubmitted.Add(1)
	}
	return firstErr
}

func extractPixels(info *ImageInfo, row, col int, clear bool) []PixelRequest {
	xOff := col * 128
	yOff := row * 128
	pixels := make([]PixelRequest, 0, 128*128)

	for ly := 0; ly < 128; ly++ {
		for lx := 0; lx < 128; lx++ {
			ax := xOff + lx
			ay := yOff + ly
			ix := ax - info.OffsetX
			iy := ay - info.OffsetY

			var c color.RGBA
			if ix >= 0 && ix < info.ContentWidth && iy >= 0 && iy < info.ContentHeight {
				r, g, b, a := info.ScaledImage.At(ix, iy).RGBA()
				c = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			} else if clear {
				c = color.RGBA{}
			} else {
				continue
			}
			pixels = append(pixels, PixelRequest{Colour: c, Index: uint16(ly*128 + lx)})
		}
	}
	return pixels
}

// ========== 图片模式 ==========

func (mp *MapPlayer) PlayImage(path string) error {
	mp.setupMu.Lock()
	defer mp.setupMu.Unlock()
	mp.stopSessionLocked()
	if mp.grid == nil {
		return errors.New("请先扫描地图区域")
	}
	info, err := mp.LoadAndResizeImage(path)
	if err != nil {
		return err
	}
	mp.resetPixels()
	return mp.ProcessFrame(info)
}

// Stop terminates the decoder before waiting for its reader. Each playback
// owns its channels; replacing a video cannot redirect an old reader into the
// new playback or leave its loop waiting forever after EOF.
func (mp *MapPlayer) Stop() {
	mp.setupMu.Lock()
	defer mp.setupMu.Unlock()
	mp.stopSessionLocked()
}

func (mp *MapPlayer) stopSessionLocked() {
	if mp.session != nil {
		mp.session.shutdown()
		<-mp.session.done
		mp.session = nil
	}
	mp.frameMu.Lock()
	mp.frameMu.Unlock()
	mp.isPaused.Store(false)
}

func (mp *MapPlayer) resetPixels() {
	mp.frameMu.Lock()
	mp.cacheMu.Lock()
	mp.previous = make(map[int64][]color.RGBA)
	mp.hasCleared = false
	mp.cacheMu.Unlock()
	mp.frameMu.Unlock()
}

func minI32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func maxI32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
