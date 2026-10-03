package main

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"bot-apk/media"
)

// 像素画画板：一个 DrawSession 是唯一真源。
// 网页画布与悬浮窗原生画布都是它的"窗口"，笔划/撤销/重做统一走这里。

// DrawCreateMode 画布创建模式。
type DrawCreateMode int

const (
	// DrawModeTransparent 纯透明：原世界区域不动，只画到的格子才放方块。
	DrawModeTransparent DrawCreateMode = iota
	// DrawModeFill 填充：把整个正方形平面在高度 Y 逐格填成一种方块。
	DrawModeFill
)

// DrawSyncMode 同步方式。
type DrawSyncMode int

const (
	// DrawSyncRealtime 每笔实时同步（默认）。
	DrawSyncRealtime DrawSyncMode = iota
	// DrawSyncBatch 批量：连续画多笔，点"应用"才一次放置。
	DrawSyncBatch
)

// Cell 画布一格。Trans 为 true 表示透明（未触碰，世界保持原样）；否则 Block 是要放置的方块。
type Cell struct {
	Block string
	Trans bool
}

// airCell 空气方块（擦除=画空气）。
func airCell() Cell { return Cell{Block: "minecraft:air"} }

// Region 绘制区域（矩形）。X0,Z0 是起点，W 是 X 方向边长，D 是 Z 方向边长，Y 是平面高度。
type Region struct {
	X0, Z0 int
	W, D   int
	Y      int
}

// abs 返回绝对值。
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// RegionFromOrigin 由起点与两条边长规整出矩形区域。
// W 为 X 方向边长，D 为 Z 方向边长，各自 1~512（超出取 512）。
func RegionFromOrigin(x1, z1, w, d, y int) Region {
	if w < 1 {
		w = 1
	}
	if d < 1 {
		d = 1
	}
	if w > 512 {
		w = 512
	}
	if d > 512 {
		d = 512
	}
	return Region{X0: x1, Z0: z1, W: w, D: d, Y: y}
}

// CellChange 单个格子的变化。
type CellChange struct {
	X, Z       int
	Prev, Next Cell
}

// Stroke 一笔：一组格子变化。
type Stroke struct {
	Changes []CellChange
}

// placeOp 一次放置操作：要么先 TP 到区块中心（isTP），要么放一个方块。
type placeOp struct {
	isTP     bool
	x, y, z  int
	block    string
	chunkXZ  [2]int
}

// DrawSession 绘制会话。
type DrawSession struct {
	mu        sync.Mutex
	region    Region
	grid      [][]Cell
	create    DrawCreateMode
	fillBlock string
	syncMode  DrawSyncMode
	palette   []media.PaletteEntry

	undoStack []*Stroke
	redoStack []*Stroke
	pending   []CellChange // 批量模式下未应用的变化
	placing   bool

	// 可注入，便于单测；nil 时用真实机器人。
	placeFn func(x, y, z int, block string) error
	readFn  func(x, y, z int) (string, error)
	tpFn    func(x, y, z int) error
	// emit 推送事件到所有界面；nil 时用真实 hub.Emit。
	emit func(event, data string)
}

// newDrawSession 新建一个会话（方块放置用真实机器人）。
func newDrawSession(region Region, create DrawCreateMode, fillBlock string, syncMode DrawSyncMode) *DrawSession {
	return newDrawSessionWith(nil, region, create, fillBlock, syncMode)
}

// newDrawSessionWith 可注入依赖（测试用）。
func newDrawSessionWith(emit func(string, string), region Region, create DrawCreateMode, fillBlock string, syncMode DrawSyncMode) *DrawSession {
	s := &DrawSession{
		region:    region,
		create:    create,
		fillBlock: fillBlock,
		syncMode:  syncMode,
		palette:   media.BlockPalette(),
		emit:      emit,
	}
	s.initGrid()
	return s
}

// initGrid 按创建模式初始化网格。
// 透明模式：全部透明（世界不动）。填充模式：全部填成 fillBlock。
func (s *DrawSession) initGrid() {
	w, d := s.region.W, s.region.D
	s.grid = make([][]Cell, w)
	for x := 0; x < w; x++ {
		s.grid[x] = make([]Cell, d)
		for z := 0; z < d; z++ {
			if s.create == DrawModeFill && s.fillBlock != "" {
				s.grid[x][z] = Cell{Block: s.fillBlock}
			} else {
				s.grid[x][z] = Cell{Trans: true}
			}
		}
	}
}

// emitEvent 安全地发事件。
func (s *DrawSession) emitEvent(event, data string) {
	if s.emit != nil {
		s.emit(event, data)
		return
	}
	hub.Emit(event, data)
}

// emitBusy 推送放置锁状态。
func (s *DrawSession) emitBusy(busy bool) {
	s.emitEvent("draw_busy", mustJSON(map[string]any{"busy": busy}))
}

// emitUpdate 推送画布差异（供其它界面镜像 + 前端更新）。
func (s *DrawSession) emitUpdate(changes []CellChange) {
	s.emitEvent("draw_update", mustJSON(map[string]any{
		"changes": changesToJSON(changes),
		"state":   s.stateLocked(),
	}))
}

func changesToJSON(changes []CellChange) []map[string]any {
	out := make([]map[string]any, 0, len(changes))
	for _, ch := range changes {
		out = append(out, map[string]any{
			"x": ch.X, "z": ch.Z,
			"prev": cellJSON(ch.Prev), "next": cellJSON(ch.Next),
		})
	}
	return out
}

func cellJSON(c Cell) map[string]any {
	return map[string]any{"b": c.Block, "t": c.Trans}
}

// getBot 返回当前机器人（放置/读取用）。
func (s *DrawSession) getBot() *BotManager {
	return getBotManager()
}

// baselineCell 首次触碰透明格时，读取该格世界的原值作为撤销基线。
func (s *DrawSession) baselineCell(x, z int) Cell {
	wx := s.region.X0 + x
	wz := s.region.Z0 + z
	if s.readFn != nil {
		if b, err := s.readFn(wx, s.region.Y, wz); err == nil && b != "" {
			return Cell{Block: b}
		}
		return airCell()
	}
	if bm := s.getBot(); bm != nil {
		if b, err := bm.GetBlockTile(wx, s.region.Y, wz); err == nil && b != "" {
			return Cell{Block: b}
		}
	}
	return airCell()
}

// ApplyStroke 应用一笔绘制。points 为网格坐标 (x,z) 与要画的方块。
// 放置中拒绝新笔画。
func (s *DrawSession) ApplyStroke(points []StrokePoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.placing {
		return
	}
	stroke := &Stroke{}
	for _, p := range points {
		if p.X < 0 || p.X >= s.region.W || p.Z < 0 || p.Z >= s.region.D {
			continue
		}
		prev := s.grid[p.X][p.Z]
		if prev.Trans {
			prev = s.baselineCell(p.X, p.Z)
		}
		next := p.Cell
		if prev == next {
			continue
		}
		s.grid[p.X][p.Z] = next
		stroke.Changes = append(stroke.Changes, CellChange{X: p.X, Z: p.Z, Prev: prev, Next: next})
	}
	if len(stroke.Changes) == 0 {
		return
	}
	s.undoStack = append(s.undoStack, stroke)
	s.redoStack = nil
	s.emitUpdate(stroke.Changes)
	s.placeOrQueue(stroke.Changes)
}

// StrokePoint 一笔中的一个点。
type StrokePoint struct {
	X, Z int
	Cell Cell
}

// placeOrQueue 按当前同步方式：实时→立即放置；批量→累积到 pending。
func (s *DrawSession) placeOrQueue(changes []CellChange) {
	if s.syncMode == DrawSyncBatch {
		s.pending = append(s.pending, changes...)
		return
	}
	s.runPlacement(changes)
}

// runPlacement 异步放置。放置期间锁 drawing，完成后解锁。
func (s *DrawSession) runPlacement(changes []CellChange) {
	s.placing = true
	s.emitBusy(true)
	go func() {
		defer func() {
			s.mu.Lock()
			s.placing = false
			s.mu.Unlock()
			s.emitBusy(false)
		}()
		ops := planPlace(s.region, changes)
		for _, op := range ops {
			if op.isTP {
				s.doTP(op.x, op.y, op.z)
				time.Sleep(80 * time.Millisecond)
				continue
			}
			if op.block == "" {
				continue
			}
			s.doPlace(op.x, op.y, op.z, op.block)
		}
	}()
}

// doTP 传送到坐标。
func (s *DrawSession) doTP(x, y, z int) {
	if s.tpFn != nil {
		s.tpFn(x, y, z)
		return
	}
	if bm := s.getBot(); bm != nil {
		bm.TP(x, y, z)
	}
}

// doPlace 放置一个方块（普通方块 setblock，无 NBT）。
func (s *DrawSession) doPlace(x, y, z int, block string) {
	if s.placeFn != nil {
		s.placeFn(x, y, z, block)
		return
	}
	if bm := s.getBot(); bm != nil {
		bm.SendBlockCmd(fmt.Sprintf("setblock %d %d %d %s", x, y, z, normalizeBlockName(block)))
	}
}

// PlaceWholeGrid 把当前网格所有非透明格整体放置一次（填充创建时填平面用）。
func (s *DrawSession) PlaceWholeGrid() {
	s.mu.Lock()
	if s.placing {
		s.mu.Unlock()
		return
	}
	var changes []CellChange
	w, d := s.region.W, s.region.D
	for x := 0; x < w; x++ {
		for z := 0; z < d; z++ {
			c := s.grid[x][z]
			if c.Trans {
				continue
			}
			changes = append(changes, CellChange{X: x, Z: z, Prev: Cell{}, Next: c})
		}
	}
	s.mu.Unlock()
	if len(changes) > 0 {
		s.runPlacement(changes)
	}
}

// Apply 批量模式下把累积的变化一次性放置。
func (s *DrawSession) Apply() {
	s.mu.Lock()
	if s.placing {
		s.mu.Unlock()
		return
	}
	if len(s.pending) == 0 {
		s.mu.Unlock()
		return
	}
	changes := s.pending
	s.pending = nil
	s.mu.Unlock()
	s.runPlacement(changes)
}

// SetSyncMode 切换实时/批量。
func (s *DrawSession) SetSyncMode(m DrawSyncMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncMode = m
}

// Undo 撤销最后一笔。
func (s *DrawSession) Undo() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.placing || len(s.undoStack) == 0 {
		return
	}
	st := s.undoStack[len(s.undoStack)-1]
	s.undoStack = s.undoStack[:len(s.undoStack)-1]
	var rev []CellChange
	for i := len(st.Changes) - 1; i >= 0; i-- {
		ch := st.Changes[i]
		s.grid[ch.X][ch.Z] = ch.Prev
		rev = append(rev, CellChange{X: ch.X, Z: ch.Z, Prev: ch.Next, Next: ch.Prev})
	}
	s.redoStack = append(s.redoStack, st)
	s.emitUpdate(rev)
	s.placeOrQueue(rev)
}

// Redo 重做（撤销的逆）。
func (s *DrawSession) Redo() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.placing || len(s.redoStack) == 0 {
		return
	}
	st := s.redoStack[len(s.redoStack)-1]
	s.redoStack = s.redoStack[:len(s.redoStack)-1]
	var redo []CellChange
	for _, ch := range st.Changes {
		s.grid[ch.X][ch.Z] = ch.Next
		redo = append(redo, CellChange{X: ch.X, Z: ch.Z, Prev: ch.Prev, Next: ch.Next})
	}
	s.undoStack = append(s.undoStack, st)
	s.emitUpdate(redo)
	s.placeOrQueue(redo)
}

// SetPlaneFromRead 用读取到的平面填充网格（同步/导出平面用）。
// plane[x][z] 为方块名；空字符串视为空气。
func (s *DrawSession) SetPlaneFromRead(plane [][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var changes []CellChange
	w, d := s.region.W, s.region.D
	for x := 0; x < w && x < len(plane); x++ {
		for z := 0; z < d && z < len(plane[x]); z++ {
			block := plane[x][z]
			prev := s.grid[x][z]
			next := Cell{Block: block}
			if block == "" {
				next = airCell()
			}
			if prev == next {
				continue
			}
			s.grid[x][z] = next
			changes = append(changes, CellChange{X: x, Z: z, Prev: prev, Next: next})
		}
	}
	if len(changes) > 0 {
		s.emitUpdate(changes)
	}
}

// SetCells 增量设置网格格子（同步平面流式用，不推 draw_update；由调用方推 draw_sync）。
func (s *DrawSession) SetCells(cells []CellChange) {
	s.mu.Lock()
	for _, c := range cells {
		if c.X >= 0 && c.X < s.region.W && c.Z >= 0 && c.Z < s.region.D {
			s.grid[c.X][c.Z] = c.Next
		}
	}
	s.mu.Unlock()
}

// SetPlane 用整平面替换网格（同步/导出平面用，异步调用）。
// plane[x][z] 为方块名；空字符串视为空气。完成后推 draw_plane 事件（紧凑整平面）。
func (s *DrawSession) SetPlane(plane [][]string) {
	s.mu.Lock()
	w, d := s.region.W, s.region.D
	for x := 0; x < w && x < len(plane); x++ {
		for z := 0; z < d && z < len(plane[x]); z++ {
			b := plane[x][z]
			if b == "" {
				b = "minecraft:air"
			}
			s.grid[x][z] = Cell{Block: b}
		}
	}
	region := s.region
	s.mu.Unlock()
	s.emitEvent("draw_plane", mustJSON(map[string]any{
		"region": map[string]any{"x0": region.X0, "z0": region.Z0, "w": w, "d": d, "y": region.Y},
		"grid":   plane,
	}))
}

// stateLocked 当前状态（需持有锁）。
func (s *DrawSession) stateLocked() map[string]any {
	w, d := s.region.W, s.region.D
	grid := make([][]any, w)
	for x := 0; x < w; x++ {
		row := make([]any, d)
		for z := 0; z < d; z++ {
			row[z] = cellJSON(s.grid[x][z])
		}
		grid[x] = row
	}
	return map[string]any{
		"region":   map[string]any{"x0": s.region.X0, "z0": s.region.Z0, "w": w, "d": d, "y": s.region.Y},
		"create":   s.create,
		"fill":     s.fillBlock,
		"mode":     s.syncMode,
		"grid":     grid,
		"can_undo": len(s.undoStack) > 0,
		"can_redo": len(s.redoStack) > 0,
		"placing":  s.placing,
		"pending":  len(s.pending),
	}
}

// State 返回当前会话状态（供 /api/draw/state）。
func (s *DrawSession) State() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateLocked()
}

// Region 返回会话区域（线程安全）。
func (s *DrawSession) Region() Region {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.region
}

// Placing 返回当前是否处于放置中（线程安全）。
func (s *DrawSession) Placing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.placing
}

// Palette 返回调色板。
func (s *DrawSession) Palette() []media.PaletteEntry {
	return s.palette
}

// planPlace 把格子变化规划成放置操作序列：按区块分组，每区块先 TP 到区块中心再放该区块所有方块。
// 返回顺序即执行顺序（先TP后放，区块间按左上角排序以保证确定性）。
func planPlace(region Region, changes []CellChange) []placeOp {
	type chunkKey struct{ cx, cz int }
	groups := map[chunkKey][]CellChange{}
	var keys []chunkKey
	for _, ch := range changes {
		if ch.Next.Trans {
			continue // 透明：不放置
		}
		wx := region.X0 + ch.X
		wz := region.Z0 + ch.Z
		key := chunkKey{wx >> 4, wz >> 4}
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], ch)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].cx != keys[j].cx {
			return keys[i].cx < keys[j].cx
		}
		return keys[i].cz < keys[j].cz
	})
	var ops []placeOp
	for _, k := range keys {
		ops = append(ops, placeOp{isTP: true, x: k.cx*16 + 8, y: region.Y + 1, z: k.cz*16 + 8, chunkXZ: [2]int{k.cx, k.cz}})
		cs := groups[k]
		sort.Slice(cs, func(i, j int) bool {
			if cs[i].X != cs[j].X {
				return cs[i].X < cs[j].X
			}
			return cs[i].Z < cs[j].Z
		})
		for _, ch := range cs {
			ops = append(ops, placeOp{
				x:      region.X0 + ch.X,
				y:      region.Y,
				z:      region.Z0 + ch.Z,
				block:  ch.Next.Block,
				chunkXZ: [2]int{k.cx, k.cz},
			})
		}
	}
	return ops
}
