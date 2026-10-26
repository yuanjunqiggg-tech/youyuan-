package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

// ============================================================
// 调试日志
// ============================================================

var (
	pfLogMu sync.Mutex
	pfLogFs []*os.File
)

func pfLogOpen() {
	// 多路径兜底：App 私有目录最可靠，其次 Download，最后 /tmp。
	paths := []string{
		"/data/data/org.youyuan.toolbox/files/pathfinder.log",
		"/storage/emulated/0/Download/pathfinder.log",
		"/tmp/pathfinder.log",
	}
	for _, p := range paths {
		os.MkdirAll(filepath.Dir(p), 0755)
		if f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
			pfLogFs = append(pfLogFs, f)
		}
	}
	pfLog("=== Pathfinder started at %s ===", time.Now().Format(time.RFC3339))
}

func pfLog(format string, args ...any) {
	pfLogMu.Lock()
	defer pfLogMu.Unlock()
	msg := fmt.Sprintf("[PF %s] %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
	for _, f := range pfLogFs {
		fmt.Fprint(f, msg)
		f.Sync()
	}
}

// ============================================================
// 控制状态 — 心跳读取这些标志位来发送 PlayerAuthInput
// ============================================================

type ControlState struct {
	Forward bool
	Back    bool
	Left    bool
	Right   bool
	Jump    bool
	Sprint  bool
	Sneak   bool
}

func (cs *ControlState) ToInputFlags() uint64 {
	var flags uint64
	if cs.Forward {
		flags |= 1 << packet.InputFlagUp
	}
	if cs.Back {
		flags |= 1 << packet.InputFlagDown
	}
	if cs.Left {
		flags |= 1 << packet.InputFlagLeft
	}
	if cs.Right {
		flags |= 1 << packet.InputFlagRight
	}
	if cs.Jump {
		flags |= 1 << packet.InputFlagJumping
		flags |= 1 << packet.InputFlagJumpDown
		flags |= 1 << packet.InputFlagJumpPressedRaw
		flags |= 1 << packet.InputFlagJumpCurrentRaw
	}
	if cs.Sprint {
		flags |= 1 << packet.InputFlagStartSprinting
		flags |= 1 << packet.InputFlagSprinting
	}
	if cs.Sneak {
		flags |= 1 << packet.InputFlagStartSneaking
		flags |= 1 << packet.InputFlagSneaking
	}
	return flags
}

func (cs *ControlState) MoveVector() mgl32.Vec2 {
	var dx, dz float32
	if cs.Left {
		dx -= 1
	}
	if cs.Right {
		dx += 1
	}
	if cs.Forward {
		dz -= 1
	}
	if cs.Back {
		dz += 1
	}
	len := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if len > 0 {
		dx /= len
		dz /= len
	}
	return mgl32.Vec2{dx, dz}
}

// ============================================================
// A* 节点
// ============================================================

type AStarNode struct {
	X, Y, Z int
}

// AStarNodeWithCost 带代价的节点
type AStarNodeWithCost struct {
	Node AStarNode
	F    float64 // f = g + h
	G    float64 // 从起点到这里的代价
}

// AStarSorter 用于排序开放列表
type AStarSorter []AStarNodeWithCost

func (s AStarSorter) Len() int           { return len(s) }
func (s AStarSorter) Less(i, j int) bool { return s[i].F < s[j].F }
func (s AStarSorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// ============================================================
// 目标类型
// ============================================================

type Goal interface {
	IsEnd(pos mgl32.Vec3) bool
	Heuristic(pos mgl32.Vec3) float64
	IsEndNode(n AStarNode) bool
	HeuristicNode(n AStarNode) float64
}

type GoalBlock struct {
	X, Y, Z float64
}

func (g *GoalBlock) IsEnd(pos mgl32.Vec3) bool {
	return math.Abs(float64(pos[0])-g.X) < 0.5 &&
		math.Abs(float64(pos[1])-g.Y) < 1.5 &&
		math.Abs(float64(pos[2])-g.Z) < 0.5
}
func (g *GoalBlock) Heuristic(pos mgl32.Vec3) float64 {
	return manhattanDist(float64(pos[0]), float64(pos[1]), float64(pos[2]), g.X, g.Y, g.Z)
}
func (g *GoalBlock) IsEndNode(n AStarNode) bool {
	return n.X == int(g.X) && n.Y == int(g.Y) && n.Z == int(g.Z)
}
func (g *GoalBlock) HeuristicNode(n AStarNode) float64 {
	return manhattanDist(float64(n.X), float64(n.Y), float64(n.Z), g.X, g.Y, g.Z)
}

type GoalNear struct {
	X, Y, Z float64
	Range   float64
}

func (g *GoalNear) IsEnd(pos mgl32.Vec3) bool {
	dx := float64(pos[0]) - g.X
	dy := float64(pos[1]) - g.Y
	dz := float64(pos[2]) - g.Z
	return math.Sqrt(dx*dx+dy*dy+dz*dz) <= g.Range
}
func (g *GoalNear) Heuristic(pos mgl32.Vec3) float64 {
	dx := float64(pos[0]) - g.X
	dy := float64(pos[1]) - g.Y
	dz := float64(pos[2]) - g.Z
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if dist <= g.Range {
		return 0
	}
	return dist - g.Range
}
func (g *GoalNear) IsEndNode(n AStarNode) bool {
	dx := float64(n.X) - g.X
	dy := float64(n.Y) - g.Y
	dz := float64(n.Z) - g.Z
	return math.Sqrt(dx*dx+dy*dy+dz*dz) <= g.Range
}
func (g *GoalNear) HeuristicNode(n AStarNode) float64 {
	dx := float64(n.X) - g.X
	dy := float64(n.Y) - g.Y
	dz := float64(n.Z) - g.Z
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if dist <= g.Range {
		return 0
	}
	return dist - g.Range
}

type GoalXZ struct {
	X, Z float64
}

func (g *GoalXZ) IsEnd(pos mgl32.Vec3) bool {
	return math.Abs(float64(pos[0])-g.X) < 0.5 && math.Abs(float64(pos[2])-g.Z) < 0.5
}
func (g *GoalXZ) Heuristic(pos mgl32.Vec3) float64 {
	return manhattanDist(float64(pos[0]), 0, float64(pos[2]), g.X, 0, g.Z)
}
func (g *GoalXZ) IsEndNode(n AStarNode) bool {
	return n.X == int(g.X) && n.Z == int(g.Z)
}
func (g *GoalXZ) HeuristicNode(n AStarNode) float64 {
	return manhattanDist(float64(n.X), 0, float64(n.Z), g.X, 0, g.Z)
}

func manhattanDist(x1, y1, z1, x2, y2, z2 float64) float64 {
	return math.Abs(x1-x2) + math.Abs(y1-y2) + math.Abs(z1-z2)
}

// ============================================================
// A* 寻路算法
// ============================================================

const (
	AStarMaxNodes  = 2000
	WalkCost       = 1.0
	DiagonalCost   = 1.414
	JumpCost       = 2.0
	FallCostPerBLock = 0.5
)

// AStar 执行 A* 搜索，返回路径节点列表（世界坐标）
// 返回 nil 表示无路径
func AStar(w *World, startX, startY, startZ int, goal Goal) []AStarNode {
	startNode := AStarNode{X: startX, Y: startY, Z: startZ}
	startKey := nodeKey(startNode)

	pfLog("AStar: start=(%d,%d,%d) h=%.1f", startX, startY, startZ, goal.HeuristicNode(startNode))

	openSet := []AStarNodeWithCost{{Node: startNode, F: goal.HeuristicNode(startNode), G: 0}}
	gScore := map[string]float64{startKey: 0}
	cameFrom := map[string]string{}

	iterations := 0
	maxNodes := AStarMaxNodes

	// 记录已探索节点中离目标最近的一个：当探索范围不足够不到终点时，
	// 返回一条"朝目标方向能走多远就走多远"的部分路径，让机器人分段向目标摸索。
	bestNode := startNode
	bestH := goal.HeuristicNode(startNode)

	for len(openSet) > 0 && iterations < maxNodes {
		iterations++

		// 取 f 最小的节点
		sort.Sort(AStarSorter(openSet))
		current := openSet[0]
		openSet = openSet[1:]
		node := current.Node
		nodeK := nodeKey(node)

		if goal.IsEndNode(node) {
			// 重建路径
			path := reconstructPath(cameFrom, nodeK, startNode)
			pfLog("AStar: path found! nodes=%d pathLen=%d iterations=%d", len(gScore), len(path), iterations)
			return path
		}

		// 记录离目标最近的可达节点（启发式最小 ≈ 朝目标方向最远）
		if h := goal.HeuristicNode(node); h < bestH {
			bestH = h
			bestNode = node
		}

		// 展开邻居
		neighbors := getNeighbors(w, node)

		for _, nb := range neighbors {
			nbKey := nodeKey(nb.Node)
			tentativeG := gScore[nodeK] + nb.Cost

			if oldG, ok := gScore[nbKey]; !ok || tentativeG < oldG {
				gScore[nbKey] = tentativeG
				cameFrom[nbKey] = nodeK
				f := tentativeG + goal.HeuristicNode(nb.Node)
				openSet = append(openSet, AStarNodeWithCost{Node: nb.Node, F: f, G: tentativeG})
			}
		}
	}

	// 探索范围不足：若已朝目标方向探索出新的可达节点，返回它的部分路径。
	// 机器人走到这里后 run() 会重新寻路，继续朝目标推进（分段摸索）。
	if bestNode != startNode {
		path := reconstructPath(cameFrom, nodeKey(bestNode), startNode)
		pfLog("AStar: partial path to nearest (%d,%d,%d) len=%d iterations=%d",
			bestNode.X, bestNode.Y, bestNode.Z, len(path), iterations)
		return path
	}

	pfLog("AStar: no path found! iterations=%d openSet=%d gScore=%d", iterations, len(openSet), len(gScore))
	return nil
}

// NeighborCost 邻居节点及其代价
type NeighborCost struct {
	Node AStarNode
	Cost float64
}

// getNeighbors 获取 A* 展开的邻居节点
// 移动类型: 平地走、对角走、跳上1格、落下1-3格
func getNeighbors(w *World, node AStarNode) []NeighborCost {
	neighbors := []NeighborCost{}
	x, y, z := node.X, node.Y, node.Z

	// 8 方向
	dirs := [][2]int{
		{1, 0}, {-1, 0}, {0, 1}, {0, -1}, // 直线
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1}, // 对角
	}

	for _, d := range dirs {
		dx, dz := d[0], d[1]
		nx, nz := x+dx, z+dz
		isDiag := dx != 0 && dz != 0

		// --- 平地走 ---
		if w.IsSafe(nx, y, nz) {
			if isDiag {
				// 对角需要两个相邻直线方向都是可通行的
				if w.IsWalkable(x+dx, y, z) && w.IsWalkable(x, y, nz) {
					neighbors = append(neighbors, NeighborCost{AStarNode{nx, y, nz}, DiagonalCost})
				}
			} else {
				neighbors = append(neighbors, NeighborCost{AStarNode{nx, y, nz}, WalkCost})
			}
			continue
		}

		// --- 跳上 1 格 ---
		if !isDiag && w.IsWalkable(x, y+2, z) && w.IsSafe(nx, y+1, nz) {
			neighbors = append(neighbors, NeighborCost{AStarNode{nx, y + 1, nz}, JumpCost})
		}

		// --- 落下 1-3 格 ---
		if !isDiag {
			for drop := 1; drop <= 3; drop++ {
				if w.IsSafe(nx, y-drop, nz) {
					// 检查下落路径是否通畅
					clear := true
					for h := 0; h < drop; h++ {
						if !w.IsWalkable(nx, y-h, nz) {
							clear = false
							break
						}
					}
					if clear {
						neighbors = append(neighbors, NeighborCost{AStarNode{nx, y - drop, nz}, WalkCost + float64(drop)*FallCostPerBLock})
					}
					break
				}
			}
		}
	}

	return neighbors
}

// nodeKey 将节点转为 map key
func nodeKey(n AStarNode) string {
	return fmt.Sprintf("%d,%d,%d", n.X, n.Y, n.Z)
}

// nodeKeyToPos 将 key 解析为坐标
func nodeKeyToPos(key string) (int, int, int) {
	var x, y, z int
	fmt.Sscanf(key, "%d,%d,%d", &x, &y, &z)
	return x, y, z
}

// reconstructPath 从 cameFrom 重建路径
func reconstructPath(cameFrom map[string]string, currentKey string, start AStarNode) []AStarNode {
	path := []AStarNode{}
	key := currentKey

	for key != "" {
		x, y, z := nodeKeyToPos(key)
		path = append([]AStarNode{{x, y, z}}, path...)
		prev, ok := cameFrom[key]
		if !ok {
			break
		}
		key = prev
		// 检查是否回到起点
		if key == nodeKey(start) {
			x, y, z = nodeKeyToPos(key)
			path = append([]AStarNode{{x, y, z}}, path...)
			break
		}
	}

	return path
}

// ============================================================
// Pathfinder — 寻路 + 移动控制
// ============================================================

type PathStatus int

const (
	PathStatusIdle PathStatus = iota
	PathStatusSearching
	PathStatusMoving
	PathStatusBlocked
	PathStatusDone
	PathStatusFailed
)

type Pathfinder struct {
	bm          *BotManager
	goal        Goal
	status      PathStatus
	stopCh      chan struct{}
	cs          ControlState
	csMu        sync.Mutex
	path        []AStarNode
	pathIndex   int
	stuckCount  int
	lastPos     mgl32.Vec3

	// 导航用真实位置（服务器权威移动下 bm.pos 不回传更新，需定时 querytarget 查询）
	navPos    mgl32.Vec3 // 最近一次查询到的服务器真实位置
	lastQuery time.Time  // 上次查询位置的时间

	// 持续跟随
	followPlayer string    // 跟随的玩家名（非空=跟随模式）
	followLast   time.Time // 上次刷新跟随目标的时间
	followTx     float64   // 当前路径所指向的目标坐标，用于判断玩家是否移动了
	followTy     float64
	followTz     float64
}

func NewPathfinder(bm *BotManager) *Pathfinder {
	pfLogOpen()
	return &Pathfinder{
		bm:     bm,
		stopCh: make(chan struct{}),
		status: PathStatusIdle,
	}
}

func (pf *Pathfinder) GetControlState() ControlState {
	pf.csMu.Lock()
	defer pf.csMu.Unlock()
	return pf.cs
}

func (pf *Pathfinder) SetControlState(forward, back, left, right, jump, sprint, sneak bool) {
	pf.csMu.Lock()
	defer pf.csMu.Unlock()
	pf.cs.Forward = forward
	pf.cs.Back = back
	pf.cs.Left = left
	pf.cs.Right = right
	pf.cs.Jump = jump
	pf.cs.Sprint = sprint
	pf.cs.Sneak = sneak
}

func (pf *Pathfinder) ClearControlStates() {
	pf.SetControlState(false, false, false, false, false, false, false)
}

func (pf *Pathfinder) Status() PathStatus {
	return pf.status
}

func (pf *Pathfinder) StatusString() string {
	switch pf.status {
	case PathStatusIdle:
		return "空闲"
	case PathStatusSearching:
		return "寻路中..."
	case PathStatusMoving:
		return "移动中"
	case PathStatusBlocked:
		return "卡住"
	case PathStatusDone:
		return "已到达"
	case PathStatusFailed:
		return "失败"
	}
	return "未知"
}

// Goto 导航到目标
func (pf *Pathfinder) Goto(goal Goal) error {
	pfLog("=== Goto request ===")

	if pf.status != PathStatusIdle && pf.status != PathStatusDone && pf.status != PathStatusFailed {
		pf.Stop()
		time.Sleep(200 * time.Millisecond)
	}

	pf.goal = goal
	pf.stopCh = make(chan struct{})
	pf.path = nil
	pf.pathIndex = 0
	pf.stuckCount = 0
	pf.followPlayer = "" // 普通 Goto 会结束跟随模式

	// 获取当前位置（取整作为 A* 起点）
	pos := pf.bm.GetPosition()
	sx := int(math.Floor(float64(pos["x"].(float32))))
	sy := int(math.Floor(float64(pos["y"].(float32))))
	sz := int(math.Floor(float64(pos["z"].(float32))))

	pfLog("Goto: start=(%d,%d,%d) world=%s", sx, sy, sz, func() string {
		if globalWorld == nil {
			return "nil"
		}
		return fmt.Sprintf("chunks=%d", len(globalWorld.columns))
	}())

	// 计算路径
	pf.status = PathStatusSearching
	if len(globalWorld.columns) == 0 {
		// 没有世界数据，使用直线路径（直接朝目标走）
		pfLog("Goto: no world data, using straight-line path")
		var tx, ty, tz int
		switch g := goal.(type) {
		case *GoalBlock:
			tx, ty, tz = int(g.X), int(g.Y), int(g.Z)
		case *GoalNear:
			tx, ty, tz = int(g.X), int(g.Y), int(g.Z)
		case *GoalXZ:
			tx, ty, tz = int(g.X), sy, int(g.Z)
		}
		pf.path = []AStarNode{
			{X: sx, Y: sy, Z: sz},
			{X: tx, Y: ty, Z: tz},
		}
	} else {
		pf.path = AStar(globalWorld, sx, sy, sz, goal)
	}

	if pf.path == nil || len(pf.path) == 0 {
		// A* 失败但区块存在，可能是子区块请求模式没有方块数据
		// 回退到直线路径
		pfLog("Goto: A* failed but chunks=%d exist, using straight-line fallback", len(globalWorld.columns))
		var tx, ty, tz int
		switch g := goal.(type) {
		case *GoalBlock:
			tx, ty, tz = int(g.X), int(g.Y), int(g.Z)
		case *GoalNear:
			tx, ty, tz = int(g.X), int(g.Y), int(g.Z)
		case *GoalXZ:
			tx, ty, tz = int(g.X), sy, int(g.Z)
		}
		if tx == sx && ty == sy && tz == sz {
			pf.status = PathStatusFailed
			pfLog("Goto: target is same as start, no path needed")
			return fmt.Errorf("目标就是当前位置")
		}
		pf.path = []AStarNode{
			{X: sx, Y: sy, Z: sz},
			{X: tx, Y: ty, Z: tz},
		}
		pfLog("Goto: straight-line fallback path created, length=%d", len(pf.path))
	}

	pfLog("Goto: path found! length=%d nodes", len(pf.path))
	for i, n := range pf.path {
		if i < 10 || i >= len(pf.path)-3 {
			pfLog("  path[%d]: (%d,%d,%d)", i, n.X, n.Y, n.Z)
		} else if i == 10 {
			pfLog("  ... (%d more nodes)", len(pf.path)-13)
		}
	}

	pf.status = PathStatusMoving
	pf.pathIndex = 1 // 跳过起点
	go pf.run()
	return nil
}

// Follow 持续跟随玩家。跟随模式下每 500ms 把目标刷新为玩家当前位置，
// 玩家移动时机器人会一直追着走，直到调用 Stop() 才停止。
func (pf *Pathfinder) Follow(player string) error {
	pfLog("=== Follow request: %s ===", player)

	if pf.status != PathStatusIdle && pf.status != PathStatusDone && pf.status != PathStatusFailed {
		pf.Stop()
		time.Sleep(200 * time.Millisecond)
	}

	pi := pf.bm.GetPlayer(player)
	if pi == nil {
		return fmt.Errorf("未找到玩家 %s", player)
	}

	initGoal := &GoalNear{
		X: float64(pi.Position[0]), Y: float64(pi.Position[1]), Z: float64(pi.Position[2]),
		Range: 2,
	}
	pf.goal = initGoal
	pf.followPlayer = player
	pf.followLast = time.Now()
	pf.followTx, pf.followTy, pf.followTz = initGoal.X, initGoal.Y, initGoal.Z
	pf.stopCh = make(chan struct{})
	pf.path = nil // run() 首轮会自行计算路径
	pf.pathIndex = 0
	pf.stuckCount = 0

	pf.status = PathStatusSearching
	go pf.run()
	return nil
}

// Stop 停止导航
func (pf *Pathfinder) Stop() {
	pfLog("Pathfinder: STOP requested")
	select {
	case <-pf.stopCh:
	default:
		close(pf.stopCh)
	}
	pf.ClearControlStates()
	pf.status = PathStatusIdle
	pf.path = nil
	pf.followPlayer = ""
}

// IsActive 是否活跃
func (pf *Pathfinder) IsActive() bool {
	return pf.status == PathStatusSearching || pf.status == PathStatusMoving || pf.status == PathStatusBlocked
}

// goalCoord 从目标对象取出坐标（供日志/调试用）
func goalCoord(g Goal) (float64, float64, float64) {
	switch t := g.(type) {
	case *GoalBlock:
		return t.X, t.Y, t.Z
	case *GoalNear:
		return t.X, t.Y, t.Z
	case *GoalXZ:
		return t.X, 0, t.Z
	}
	return 0, 0, 0
}

// run 寻路跟随循环
func (pf *Pathfinder) run() {
	bm := pf.bm
	defer func() {
		if pf.status == PathStatusMoving {
			pf.status = PathStatusIdle
		}
		bm.StopMovement()
		bm.JumpStop()
		bm.SprintStop()
		pf.ClearControlStates()
	}()

	// 初始真实位置：服务器权威移动下 bm.pos 不回传更新，导航必须用 querytarget 查询
	pf.navPos, _ = bm.QueryBotPos()
	pf.lastQuery = time.Now()

	tk := time.NewTicker(100 * time.Millisecond)
	defer tk.Stop()

	stuckCount := 0
	recalcCount := 0
	moving := false

	for {
		select {
		case <-pf.stopCh:
			return
		case <-tk.C:
		}

		// 每 500ms 用 querytarget <自己名字> 查询真实位置并做卡住判定。
		// 相机助手/玩家列表都用这个拿到真实坐标，可靠；bm.pos 自身不随移动更新。
		if time.Since(pf.lastQuery) >= 500*time.Millisecond {
			pf.lastQuery = time.Now()
			if newPos, ok := bm.QueryBotPos(); ok {
				d := math.Sqrt(float64(
					(newPos[0]-pf.navPos[0])*(newPos[0]-pf.navPos[0]) +
						(newPos[1]-pf.navPos[1])*(newPos[1]-pf.navPos[1]) +
						(newPos[2]-pf.navPos[2])*(newPos[2]-pf.navPos[2]),
				))
				if d < 0.4 {
					stuckCount++
				} else {
					stuckCount = 0
					recalcCount = 0 // 有推进就重置重算上限，长距离才能一段一段向前
				}
				pf.navPos = newPos
				pfLog("query pos=(%.1f,%.1f,%.1f) moved=%.2f stuck=%d", newPos[0], newPos[1], newPos[2], d, stuckCount)
			}
		}
		curPos := pf.navPos
		curX, curY, curZ := curPos[0], curPos[1], curPos[2]

		// 跟随模式：周期刷新目标为玩家当前位置
		if pf.followPlayer != "" && time.Since(pf.followLast) > 500*time.Millisecond {
			pf.followLast = time.Now()
			pi := bm.GetPlayer(pf.followPlayer)
			if pi == nil {
				pfLog("Follow: player %s 不在线，停止跟随", pf.followPlayer)
				pf.status = PathStatusFailed
				return
			}
			ng := &GoalNear{X: float64(pi.Position[0]), Y: float64(pi.Position[1]), Z: float64(pi.Position[2]), Range: 2}
			pf.goal = ng
			if math.Abs(ng.X-pf.followTx)+math.Abs(ng.Y-pf.followTy)+math.Abs(ng.Z-pf.followTz) > 1.5 {
				pf.followTx, pf.followTy, pf.followTz = ng.X, ng.Y, ng.Z
				pf.path = nil // 触发重算
			}
		}

		// 到达判定
		if pf.goal.IsEnd(curPos) {
			if pf.followPlayer != "" {
				bm.StopMovement()
				moving = false
				pf.path = nil
				continue
			}
			pfLog("=== GOAL REACHED! pos=(%.1f,%.1f,%.1f) ===", curX, curY, curZ)
			bm.StopMovement()
			pf.status = PathStatusDone
			return
		}

		// 路径为空或已走完 → 重新寻路（含 A* 分段推进）
		if pf.path == nil || pf.pathIndex >= len(pf.path) {
			pfLog("Path exhausted/nil, recalculating...")
			pf.recalculatePath(curPos)
			if pf.path == nil {
				pfLog("Recalculate failed!")
				pf.status = PathStatusFailed
				return
			}
			pf.pathIndex = 1
			recalcCount++
			if recalcCount > 8 {
				pfLog("Too many recalculations, giving up")
				pf.status = PathStatusFailed
				return
			}
			continue
		}

		// 当前目标路径点
		target := pf.path[pf.pathIndex]
		targetPos := mgl32.Vec3{float32(target.X) + 0.5, float32(target.Y), float32(target.Z) + 0.5}

		dx := float64(targetPos[0]) - float64(curX)
		dz := float64(targetPos[2]) - float64(curZ)
		dy := float64(targetPos[1]) - float64(curY)
		horizDist := math.Sqrt(dx*dx + dz*dz)

		// 卡住 6 次查询（约 3 秒）无推进 → 重新寻路
		if stuckCount >= 6 {
			pfLog("  STUCK for ~3s, recalculating...")
			stuckCount = 0
			pf.recalculatePath(curPos)
			if pf.path == nil {
				pfLog("Recalculate failed!")
				pf.status = PathStatusFailed
				return
			}
			pf.pathIndex = 1
			recalcCount++
			if recalcCount > 8 {
				pfLog("Too many recalculations, giving up")
				pf.status = PathStatusFailed
				return
			}
			continue
		}

		// 到达当前路径点
		if horizDist < 0.6 && math.Abs(dy) < 1.5 {
			pf.pathIndex++
			pfLog("  waypoint %d reached, next=%d/%d", pf.pathIndex-1, pf.pathIndex, len(pf.path))
			continue
		}

		// 面向目标（决定前进方向）
		lookYaw := math.Atan2(dz, dx) * 180 / math.Pi
		lookYaw = math.Mod(lookYaw+270, 360)
		lookPitch := float64(0)
		if horizDist > 0.1 {
			lookPitch = -math.Atan2(dy, horizDist) * 180 / math.Pi
		}
		bm.SetRotation(float32(lookPitch), float32(lookYaw))

		// 移动：用飞行验证过的机制（前进 + 视角方向），不再走 ControlState 覆盖
		if horizDist > 0.3 {
			if !moving {
				bm.StartMoveDirection(0, 0, -1) // forward
				moving = true
			}
			if dy > 0.5 {
				bm.JumpStart()
			} else {
				bm.JumpStop()
			}
		} else {
			if moving {
				bm.StopMovement()
				moving = false
			}
			bm.JumpStop()
		}

		// 状态日志
		gx, gy, gz := goalCoord(pf.goal)
		pos := bm.GetPosition()
		botYaw := pos["yaw"].(float32)
		flying := pos["isFlying"].(bool)
		pfLog("tick navpos=(%.1f,%.1f,%.1f) goal=(%.0f,%.0f,%.0f) lookYaw=%.1f botYaw=%.1f dist=%.2f dy=%.2f stuck=%d path=%d/%d flying=%v",
			curX, curY, curZ, gx, gy, gz, lookYaw, botYaw, horizDist, dy, stuckCount, pf.pathIndex, len(pf.path), flying)
	}
}

// recalculatePath 重新计算路径。
// A* 无路径（无世界数据或寻路失败）时回退为直线路径，直接朝目标走，
// 而不是判失败停下——让机器人在直线兜底下靠卡住闪避尽量绕过去。
func (pf *Pathfinder) recalculatePath(curPos mgl32.Vec3) {
	sx := int(math.Floor(float64(curPos[0])))
	sy := int(math.Floor(float64(curPos[1])))
	sz := int(math.Floor(float64(curPos[2])))

	pfLog("Recalculate from (%d,%d,%d)", sx, sy, sz)

	var newPath []AStarNode
	if globalWorld != nil && len(globalWorld.columns) > 0 {
		newPath = AStar(globalWorld, sx, sy, sz, pf.goal)
	}

	if newPath != nil && len(newPath) > 1 {
		pf.path = newPath
		pf.pathIndex = 1
		pfLog("Recalculate: new path length=%d", len(newPath))
		return
	}

	// 直线兜底：直接朝目标走
	var tx, ty, tz int
	switch g := pf.goal.(type) {
	case *GoalBlock:
		tx, ty, tz = int(g.X), int(g.Y), int(g.Z)
	case *GoalNear:
		tx, ty, tz = int(g.X), int(g.Y), int(g.Z)
	case *GoalXZ:
		tx, ty, tz = int(g.X), sy, int(g.Z)
	}
	if tx == sx && ty == sy && tz == sz {
		pf.path = nil
		pfLog("Recalculate: already at target")
		return
	}
	pf.path = []AStarNode{{X: sx, Y: sy, Z: sz}, {X: tx, Y: ty, Z: tz}}
	pf.pathIndex = 1
	pfLog("Recalculate: straight-line fallback, len=%d", len(pf.path))
}

// ============================================================
// BotManager 集成
// ============================================================

var globalPF *Pathfinder

func InitPathfinder(bm *BotManager) {
	globalPF = NewPathfinder(bm)
	pfLog("Pathfinder initialized")
}

func GetPFControlState() ControlState {
	if globalPF == nil {
		return ControlState{}
	}
	return globalPF.GetControlState()
}

func HasPFControl() bool {
	if globalPF == nil {
		return false
	}
	cs := globalPF.GetControlState()
	return cs.Forward || cs.Back || cs.Left || cs.Right || cs.Jump || cs.Sprint || cs.Sneak
}