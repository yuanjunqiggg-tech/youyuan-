package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http"
	"sync"

	"bot-apk/media"

	"github.com/TriM-Organization/bedrock-world-operator/block"
	"github.com/TriM-Organization/bedrock-world-operator/chunk"
	"github.com/TriM-Organization/bedrock-world-operator/define"
)

// drawTexColorCache 方块纹理平均色缓存。
var drawTexColorCache = map[string][3]uint8{}

// drawTextureColor 计算方块顶层纹理的平均 RGB，用于按实际颜色分类。
func drawTextureColor(name string) ([3]uint8, bool) {
	if c, ok := drawTexColorCache[name]; ok {
		return c, true
	}
	tex := drawBlockTexture(name)
	if tex == "" {
		return [3]uint8{}, false
	}
	data, err := frontendFS.ReadFile("frontend/blocks/" + tex + ".png")
	if err != nil {
		return [3]uint8{}, false
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return [3]uint8{}, false
	}
	b := img.Bounds()
	var tr, tg, tb, n uint64
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			pr, pg, pb, _ := img.At(x, y).RGBA()
			tr += uint64(pr >> 8)
			tg += uint64(pg >> 8)
			tb += uint64(pb >> 8)
			n++
		}
	}
	if n == 0 {
		return [3]uint8{}, false
	}
	c := [3]uint8{uint8(tr / n), uint8(tg / n), uint8(tb / n)}
	drawTexColorCache[name] = c
	return c, true
}

// 像素画画板 HTTP/SSE 处理器。唯一真源是 DrawSession（当前会话 drawSess）。

var (
	drawSess   *DrawSession
	drawSessMu sync.Mutex
)

// currentDrawSession 返回当前会话（无则 nil）。
func currentDrawSession() *DrawSession {
	drawSessMu.Lock()
	defer drawSessMu.Unlock()
	return drawSess
}

// setCurrentDrawSession 设置/清空当前会话。
func setCurrentDrawSession(s *DrawSession) {
	drawSessMu.Lock()
	defer drawSessMu.Unlock()
	drawSess = s
}

// drawPaletteEntries 画板调色板（colors.json 全部方块 + 基准色）。
func drawPaletteEntries() []media.PaletteEntry {
	return media.BlockPalette()
}

// ── create ──

type drawCreateReq struct {
	X1, Z1   int // 起点
	W, D     int // X 方向边长、Z 方向边长（可不等，不强制正方形）
	Y        int
	Mode     string `json:"mode"`       // "transparent" | "fill"
	FillBlock string `json:"fill_block"` // fill 模式必填
	Sync     string `json:"sync"`       // "realtime" | "batch"
}

func handleDrawCreate(w http.ResponseWriter, r *http.Request) {
	var req drawCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "参数解析失败"})
		return
	}
	// 选区完全依赖用户输入，缺任一报错
	if req.W <= 0 || req.D <= 0 {
		writeJSON(w, map[string]any{"ok": false, "error": "请填写起点X1/Z1、X边长W、Z边长D、水平高度Y"})
		return
	}
	create := DrawModeTransparent
	if req.Mode == "fill" {
		create = DrawModeFill
		if req.FillBlock == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "填充模式必须选择方块"})
			return
		}
	}
	syncMode := DrawSyncRealtime
	if req.Sync == "batch" {
		syncMode = DrawSyncBatch
	}
	region := RegionFromOrigin(req.X1, req.Z1, req.W, req.D, req.Y)
	sess := newDrawSession(region, create, req.FillBlock, syncMode)
	setCurrentDrawSession(sess)
	if create == DrawModeFill {
		sess.PlaceWholeGrid()
	}
	writeJSON(w, map[string]any{"ok": true, "state": sess.State()})
}

// ── stroke ──

type drawStrokeReq struct {
	Points []struct {
		X, Z int
		Block string
		Air   bool `json:"air"`
	} `json:"points"`
}

func handleDrawStroke(w http.ResponseWriter, r *http.Request) {
	sess := currentDrawSession()
	if sess == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "未创建画板"})
		return
	}
	var req drawStrokeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "参数解析失败"})
		return
	}
	pts := make([]StrokePoint, 0, len(req.Points))
	for _, p := range req.Points {
		c := Cell{Block: p.Block}
		if p.Air || p.Block == "" {
			c = airCell()
		}
		pts = append(pts, StrokePoint{X: p.X, Z: p.Z, Cell: c})
	}
	if sess.Placing() {
		writeJSON(w, map[string]any{"ok": false, "error": "机器人放置中，请稍候", "busy": true})
		return
	}
	sess.ApplyStroke(pts)
	writeJSON(w, map[string]any{"ok": true, "state": sess.State()})
}

// ── undo / redo / mode / apply ──

func handleDrawUndo(w http.ResponseWriter, r *http.Request) {
	sess := currentDrawSession()
	if sess == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "未创建画板"})
		return
	}
	sess.Undo()
	writeJSON(w, map[string]any{"ok": true, "state": sess.State()})
}

func handleDrawRedo(w http.ResponseWriter, r *http.Request) {
	sess := currentDrawSession()
	if sess == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "未创建画板"})
		return
	}
	sess.Redo()
	writeJSON(w, map[string]any{"ok": true, "state": sess.State()})
}

func handleDrawMode(w http.ResponseWriter, r *http.Request) {
	sess := currentDrawSession()
	if sess == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "未创建画板"})
		return
	}
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "参数解析失败"})
		return
	}
	if req.Mode == "batch" {
		sess.SetSyncMode(DrawSyncBatch)
	} else {
		sess.SetSyncMode(DrawSyncRealtime)
	}
	writeJSON(w, map[string]any{"ok": true, "state": sess.State()})
}

func handleDrawApply(w http.ResponseWriter, r *http.Request) {
	sess := currentDrawSession()
	if sess == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "未创建画板"})
		return
	}
	sess.Apply()
	writeJSON(w, map[string]any{"ok": true, "state": sess.State()})
}

// ── sync（读取平面，流式）──

func drawRegionJSON(r Region) map[string]any {
	return map[string]any{"x0": r.X0, "z0": r.Z0, "w": r.W, "d": r.D, "y": r.Y}
}

func handleDrawSync(w http.ResponseWriter, r *http.Request) {
	sess := currentDrawSession()
	if sess == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "未创建画板"})
		return
	}
	bm := getBotManagerLocked()
	if bm == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "机器人未连接"})
		return
	}
	region := sess.Region()
	go func() {
		// 同步开始：前端显示动画/绿框、禁用同步按钮与绘画
		hub.Emit("draw_sync", mustJSON(map[string]any{"active": true, "region": drawRegionJSON(region)}))
		err := streamPlane(bm, region, func(processed, total int, cells []CellChange) {
			sess.SetCells(cells)
			hub.Emit("draw_sync", mustJSON(map[string]any{
				"active": true, "current": processed, "total": total,
				"region": drawRegionJSON(region), "cells": changesToJSON(cells),
			}))
		})
		if err != nil {
			hub.Emit("draw_error", mustJSON(map[string]any{"error": "同步失败: " + err.Error()}))
		}
		// 同步结束：前端隐藏动画/绿框、恢复按钮与绘画
		hub.Emit("draw_sync", mustJSON(map[string]any{"active": false, "region": drawRegionJSON(region)}))
	}()
	writeJSON(w, map[string]any{"ok": true})
}

// streamPlane 读取正方形区域在固定高度 Y 的方块（SubChunkRequest 协议级）。
// 每读一个区块即回调 onChunk（流式），cells 为该区块落在区域内的格子变化。
func streamPlane(bm *BotManager, region Region, onChunk func(processed, total int, cells []CellChange)) error {
	t := &SubChunkExportTask{
		bm:        bm,
		minX:      int32(region.X0), maxX: int32(region.X0 + region.W - 1),
		minY:      int32(region.Y), maxY: int32(region.Y),
		minZ:      int32(region.Z0), maxZ: int32(region.Z0 + region.D - 1),
		dimension: 0,
	}
	if _, err := bm.getGameInterface(); err != nil {
		return err
	}
	if err := t.requestChunkRadius(); err != nil {
		return err
	}
	dimRange := define.Dimension(t.dimension).Range()
	x0i, z0i := int32(region.X0), int32(region.Z0)
	xMax, zMax := int32(region.X0+region.W-1), int32(region.Z0+region.D-1)
	ly := byte(region.Y & 0xF)
	chunkMinX, chunkMaxX := x0i>>4, xMax>>4
	chunkMinZ, chunkMaxZ := z0i>>4, zMax>>4
	total := int((chunkMaxX - chunkMinX + 1) * (chunkMaxZ - chunkMinZ + 1))
	processed := 0
	for cx := chunkMinX; cx <= chunkMaxX; cx++ {
		for cz := chunkMinZ; cz <= chunkMaxZ; cz++ {
			if err := t.moveToChunkCenter(cx, cz, 400); err != nil {
				return err
			}
			subs, _, err := t.fetchSubChunks(cx, cz, dimRange)
			if err != nil {
				return err
			}
			var sub *chunk.SubChunk
			if len(subs) > 0 {
				sub = subs[0] // minY==maxY → 只有 1 层
			}
			var cells []CellChange
			for dx := 0; dx < 16; dx++ {
				wx := cx*16 + int32(dx)
				if wx < x0i || wx > xMax {
					continue
				}
				px := int(wx - x0i)
				for dz := 0; dz < 16; dz++ {
					wz := cz*16 + int32(dz)
					if wz < z0i || wz > zMax {
						continue
					}
					pz := int(wz - z0i)
					bName := "minecraft:air"
					if sub != nil {
						rid := sub.Block(byte(dx), ly, byte(dz), 0)
						if rid != 0 {
							if name, _, found := block.RuntimeIDToState(rid); found && name != "" {
								bName = name
							}
						}
					}
					cells = append(cells, CellChange{X: px, Z: pz, Next: Cell{Block: bName}})
				}
			}
			processed++
			onChunk(processed, total, cells)
		}
	}
	return nil
}

// ── palette / state / custom ──

func handleDrawPalette(w http.ResponseWriter, r *http.Request) {
	entries := drawPaletteEntries()
	type palItem struct {
		Name string `json:"name"`
		R    uint8  `json:"r"`
		G    uint8  `json:"g"`
		B    uint8  `json:"b"`
		Tex  string `json:"tex"`
		TC   [3]uint8 `json:"tc"` // 纹理平均色（按实际颜色分类）
	}
	out := make([]palItem, 0, len(entries))
	for _, e := range entries {
		item := palItem{Name: e.Name, R: e.R, G: e.G, B: e.B, Tex: drawBlockTexture(e.Name)}
		if c, ok := drawTextureColor(e.Name); ok {
			item.TC = c
		}
		out = append(out, item)
	}
	writeJSON(w, map[string]any{"ok": true, "palette": out})
}

// drawBlockTexture 返回方块的顶层纹理文件名（不含 .png，置于 frontend/blocks/），
// 无纹理返回 ""。优先已知重命名，再试精确匹配，再试 _top 顶层纹理。
func drawBlockTexture(name string) string {
	short := normalizeBlockName(name)
	cands := drawTextureCandidates(short)
	for _, c := range cands {
		if _, err := frontendFS.ReadFile("frontend/blocks/" + c + ".png"); err == nil {
			return c
		}
	}
	return ""
}

// drawTextureCandidates 生成按优先级排序的纹理候选（含 bedrock 重命名）。
func drawTextureCandidates(short string) []string {
	var out []string
	// 已知重命名（bedrock/旧版资源包命名）
	remap := map[string]string{
		"white_wool": "wool_colored_white", "orange_wool": "wool_colored_orange",
		"magenta_wool": "wool_colored_magenta", "light_blue_wool": "wool_colored_light_blue",
		"yellow_wool": "wool_colored_yellow", "lime_wool": "wool_colored_lime",
		"pink_wool": "wool_colored_pink", "gray_wool": "wool_colored_gray",
		"light_gray_wool": "wool_colored_silver", "cyan_wool": "wool_colored_cyan",
		"purple_wool": "wool_colored_purple", "blue_wool": "wool_colored_blue",
		"brown_wool": "wool_colored_brown", "green_wool": "wool_colored_green",
		"red_wool": "wool_colored_red", "black_wool": "wool_colored_black",

		"white_concrete": "concrete_white", "orange_concrete": "concrete_orange",
		"magenta_concrete": "concrete_magenta", "light_blue_concrete": "concrete_light_blue",
		"yellow_concrete": "concrete_yellow", "lime_concrete": "concrete_lime",
		"pink_concrete": "concrete_pink", "gray_concrete": "concrete_gray",
		"light_gray_concrete": "concrete_silver", "cyan_concrete": "concrete_cyan",
		"purple_concrete": "concrete_purple", "blue_concrete": "concrete_blue",
		"brown_concrete": "concrete_brown", "green_concrete": "concrete_green",
		"red_concrete": "concrete_red", "black_concrete": "concrete_black",

		// 陶瓦在 bedrock 资源包里是旧版 hardened_clay_stained_* 命名
		"stained_hardened_clay": "hardened_clay",
		"white_terracotta":      "hardened_clay_stained_white", "orange_terracotta": "hardened_clay_stained_orange",
		"magenta_terracotta":    "hardened_clay_stained_magenta", "light_blue_terracotta": "hardened_clay_stained_light_blue",
		"yellow_terracotta":     "hardened_clay_stained_yellow", "lime_terracotta": "hardened_clay_stained_lime",
		"pink_terracotta":       "hardened_clay_stained_pink", "gray_terracotta": "hardened_clay_stained_gray",
		"light_gray_terracotta": "hardened_clay_stained_silver", "cyan_terracotta": "hardened_clay_stained_cyan",
		"purple_terracotta":     "hardened_clay_stained_purple", "blue_terracotta": "hardened_clay_stained_blue",
		"brown_terracotta":      "hardened_clay_stained_brown", "green_terracotta": "hardened_clay_stained_green",
		"red_terracotta":        "hardened_clay_stained_red", "black_terracotta": "hardened_clay_stained_black",

		"grass_block": "grass_top",

		// 树叶
		"acacia_leaves": "leaves_acacia_opaque", "birch_leaves": "leaves_birch_opaque",
		"spruce_leaves": "leaves_spruce_opaque", "pale_oak_leaves": "pale_oak_leaves_opaque",
		"azalea_leaves_flowered": "azalea_leaves_flowers",
		// 花岗岩 / 木头 / 菌柄（一律用顶层纹理）
		"granite":       "stone_granite",
		"cherry_wood":   "cherry_log_top",
		"crimson_stem":  "crimson_log_top", "crimson_hyphae": "crimson_log_top",
		"warped_stem":   "warped_stem_top", "warped_hyphae": "warped_stem_top",
		// 铜
		"waxed_exposed_chiseled_copper":  "exposed_chiseled_copper",
		"waxed_oxidized_chiseled_copper": "oxidized_chiseled_copper",
	}
	if f, ok := remap[short]; ok {
		out = append(out, f)
	}
	// 精确匹配与顶层/侧面优先
	out = append(out, short+"_top", short+"_side", short)
	return out
}

func handleDrawState(w http.ResponseWriter, r *http.Request) {
	sess := currentDrawSession()
	if sess == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "未创建画板"})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "state": sess.State()})
}

// close 关闭当前画板会话（回到创建状态）。
func handleDrawClose(w http.ResponseWriter, r *http.Request) {
	setCurrentDrawSession(nil)
	writeJSON(w, map[string]any{"ok": true})
}

// custom 自定义方块：前端直接按方块名绘制，无需注册表。
func handleDrawCustom(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Block string `json:"block"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Block == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少方块名"})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "block": req.Block})
}
