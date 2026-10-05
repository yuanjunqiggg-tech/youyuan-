package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bot-apk/config"

	"github.com/OmineDev/flowers-for-machines/client"
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/core/py_rpc"
	cts "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server"
	cts_mc "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server/minecraft"
	cts_mc_a "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server/minecraft/ai_command"
	cts_mc_p "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server/minecraft/preset"
	cts_mc_v "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server/minecraft/vip_event_system"
	mei "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/interface"
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
	"github.com/OmineDev/flowers-for-machines/game_control/resources_control"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_console"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// BotManager holds the active Minecraft connection
type BotManager struct {
	physicalMu          sync.Mutex
	physicalNext        time.Time
	controlCommandMu    sync.Mutex
	controlBatchMu      sync.Mutex
	controlBatchPending atomic.Int32
	controlCommandNext  time.Time
	controlCommands     atomic.Uint64
	mu                  sync.Mutex
	cancelled           atomic.Bool
	readyReported       atomic.Bool
	client              *client.Client
	conn                *minecraft.Conn
	resources           *resources_control.Resources
	gameInterface       *game_interface.GameInterface
	pktCh               chan packet.Packet
	listenerID          string
	connected           bool
	inGame              bool  // 真正进入服务器（权限确认完成，onReady 触发）后才为 true
	gameMode            int32 // Latest mode confirmed by the server.
	isOP                bool
	opPacketConfirmed   bool // 包检测(UpdateAbilities/AdventureSettings/AddPlayer 权限>=Host)权威确认的 OP；命令验证失败不降级
	isMain              bool // 主机器人标记：连接+OP 后自动加载插件、断开时停用全部插件
	opSent              bool
	verifyPending       bool // true when waiting for gamemode response to confirm OP
	cmdPerm             uint32
	display             string
	xuid                string
	eid                 int64
	runtimeID           uint64
	idStr               string
	pos                 mgl32.Vec3
	serverCode          string
	botComponent        map[string]*int // 服务器模组列表 (mod UUID → outfit type)
	connectTime         time.Time       // when Connect() completed — used to suppress early false-OP packets
	sayTestMsg          string          // /say 测试消息，非空表示正在等待回显确认权限
	stopCh              chan struct{}
	logCh               chan string
	lastOnlineSet       map[string]bool              // 上次玩家在线集合快照（join/leave 差量对比用）
	onReady             func()                       // 权限确认后回调，用于直接发 SSE 事件
	onLoginFail         func(attempt int, err error) // 每次登录/重连失败回调，供主机器人逐次报告
	readyCh             chan struct{}                // closed when onReady fires, signals retry to stop
	nbtAssigner         *nbt_assigner.NBTAssigner
	nbtConsole          *nbt_console.Console
	commandDimension    string        // 命令维度（非 overworld 时自动加 execute in 前缀）
	wsCmdRespCh         chan struct{} // WS 命令收包同步

	// 飞行控制状态
	pitch         float32
	yaw           float32
	isFlying      bool
	flyRequested  bool // 用户请求的飞行状态，不被 UpdateAbilities 覆盖
	moveTicker    *time.Ticker
	moveStopCh    chan struct{}
	moveDirection mgl32.Vec3    // 当前持续移动方向
	moveVec       mgl32.Vec2    // 心跳中携带的 MoveVector（持续移动时用）
	moveFlags     uint32        // 心跳中携带的输入标志位（InputFlagUp/Down/Left/Right）
	jumpPressed   bool          // 跳跃键是否按下（心跳持续带跳跃标志）
	sneakPressed  bool          // 潜行键是否按下
	sprintPressed bool          // 奔跑键是否按下
	rotStop       chan struct{} // 平滑旋转取消信号；非空表示有平滑旋转在跑

	// 游戏心跳（PlayerAuthInput 定时发送）
	heartbeatStopCh  chan struct{}
	heartbeatWriteMu sync.Mutex // 保护心跳写入，避免并发写导致阻塞
	tick             uint64
	heartbeatEnabled bool // 前端打开飞行/寻路面板时启用，关闭时静默

	// SubChunkRequest 并发限流（最多 4 个同时进行）
	chunkReqSem chan struct{}

	// SSE 位置推送（避免重复发送相同位置）
	lastReportedPos mgl32.Vec3

	// 服务器权威位置
	structureExportUnavailable bool  // Reset on each connection; terrain fallback after a template timeout.
	dimension                  int32 // 当前维度（0=主世界, 1=下界, 2=末地）

	// 服务器 TPS 计算（SetTime 包的时间戳推算，用于悬浮窗显示）
	tps            float64
	lastServerTime int32
	lastRealTime   time.Time

	// 聊天消息限流
	chatMu       sync.Mutex // 保护聊天发送串行化
	lastChatTime time.Time  // 上次发送聊天消息的时间

	// BotIndex 该机器人在多机器人舰队中的编号（0 为主/单机）。用于跨机器人隔离：
	// structure 名前缀（§8.2）与 NBT 工作台坐标分配（§8.3）都依赖它保持唯一。
	BotIndex int
}

var (
	botMgr   *BotManager
	botMgrMu sync.Mutex
)

func NewBotManager() *BotManager {
	return &BotManager{
		stopCh:      make(chan struct{}),
		readyCh:     make(chan struct{}),
		logCh:       make(chan string, 100),
		chunkReqSem: make(chan struct{}, 4),
	}
}

func (bm *BotManager) Connect(token, server, auth, password string) error {
	var e error
	auth, e = yyProviderURL(auth)
	if e != nil {
		return e
	}
	if bm.cancelled.Load() {
		return fmt.Errorf("连接已取消")
	}
	// 检测本地联机前缀：@房间号 → 走 TanLobby 中继加入
	// #房间号 → 普通联机房间，走租赁服通道
	if strings.HasPrefix(server, "@") {
		roomID := strings.TrimPrefix(server, "@")
		return bm.connectTanLobby(token, roomID, auth, password)
	}

	bm.mu.Lock()
	if bm.connected {
		bm.disconnectLocked()
	}
	// 重置 TPS 基线，避免重连后旧基线导致 TPS 跳变
	bm.tps = 0
	bm.lastServerTime = 0
	bm.lastRealTime = time.Time{}
	bm.mu.Unlock()

	cfg := client.Config{
		AuthServerAddress:    auth,
		RentalServerCode:     server,
		RentalServerPasscode: password,
		AuthServerToken:      token,
		OnLoginProgress: func(msg string) {
			debugLog("[连接诊断] %s", msg)
		},
		// 登录阶段（挑战求解）收到的包会先缓存再被排空丢弃，
		// 这里提前喂给玩家信息管理器，捕获进服时的 PlayerList/皮肤广播。
		OnLoginPacket: func(pk packet.Packet) {
			playerInfoMgr.HandlePacket(pk, bm.logCh)
		},
	}

	bm.logCh <- fmt.Sprintf("⛏ 正在连接 %s ...", server)

	const maxRetries = 3
	var c *client.Client
	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-bm.stopCh:
			bm.logCh <- fmt.Sprintf("  └→ 连接已取消")
			return fmt.Errorf("连接已取消")
		default:
		}
		debugLog("  └→ 第%d次尝试连接...", attempt)
		c, err = client.LoginRentalServer(cfg)
		if err == nil {
			break
		}
		// 每次失败都回调，主机器人据此逐次报告（而非等全部重试完才报）
		if bm.onLoginFail != nil {
			bm.onLoginFail(attempt, err)
		}
		if attempt < maxRetries {
			// 显示具体错误，但前端只在"已重试N次"的最终消息才关弹窗
			bm.logCh <- fmt.Sprintf("  └→ 第%d次失败: %v，2秒后重试", attempt, err)
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		select {
		case <-bm.stopCh:
			bm.logCh <- fmt.Sprintf("  └→ 连接已取消")
			return fmt.Errorf("连接已取消")
		default:
		}
		bm.logCh <- fmt.Sprintf("✗ 登录失败(已重试%d次): %v", maxRetries, err)
		return fmt.Errorf("登录失败：%w", err)
	}

	conn := c.Conn()
	if bm.cancelled.Load() {
		conn.Close()
		return fmt.Errorf("连接已取消")
	}
	id := conn.IdentityData()
	debugLog("✓ 已连接到服务器! 玩家: %s", id.DisplayName)
	debugLog("  ├ 服务器代码: %s", server)
	debugLog("  ├ EID: %d", conn.GameData().EntityUniqueID)
	debugLog("  └ XUID: %s", id.XUID)
	// 皮肤诊断：记录登录时携带的皮肤数据（服务器以此分发给其他玩家）
	{
		cd := conn.ClientData()
		debugLog("[皮肤] 登录ClientData: skinID=%q skinItemID=%q w=%d h=%d skinDataB64len=%d patchB64=%s armSize=%q premium=%v persona=%v trusted=%v override=%v geoVersionB64=%s",
			cd.SkinID, cd.SkinItemID, cd.SkinImageWidth, cd.SkinImageHeight, len(cd.SkinData), cd.SkinResourcePatch,
			cd.ArmSize, cd.PremiumSkin, cd.PersonaSkin, cd.TrustedSkin, cd.OverrideSkin, cd.SkinGeometryVersion)
	}

	bm.mu.Lock()
	if bm.cancelled.Load() {
		bm.mu.Unlock()
		conn.Close()
		return fmt.Errorf("连接已取消")
	}
	bm.client = c
	bm.conn = conn
	bm.display = id.DisplayName
	bm.xuid = id.XUID
	bm.eid = int64(conn.GameData().EntityUniqueID)
	bm.runtimeID = conn.GameData().EntityRuntimeID
	bm.gameMode = conn.GameData().PlayerGameMode
	bm.idStr = id.Identity
	bm.pos = conn.GameData().PlayerPosition
	bm.serverCode = server
	bm.botComponent = c.BotComponent
	bm.connectTime = time.Now()
	bm.stopCh = make(chan struct{})
	bm.connected = true
	bm.structureExportUnavailable = false
	bm.mu.Unlock()

	// 把自己注册到 players map
	opMu.Lock()
	players[bm.display] = bm.eid
	opMu.Unlock()

	yySenseReset(bm)
	debugLog("[*] 启动包监听器...")
	res := resources_control.NewResourcesControl(c) // 自动排空登录缓存 + 自动回复网易心跳/传送/重生
	bm.mu.Lock()
	bm.resources = res
	bm.pktCh = make(chan packet.Packet, 256)
	bm.mu.Unlock()
	go bm.consumePackets()
	bm.registerPacketHandlers()
	debugLog("  └→ 监听器已启动")

	// OP 确认轮询：检测到权限后发送就绪通知，无权限时定期请求授权
	// Wrap onReady to signal readyCh on initialization complete
	origReady := bm.onReady
	bm.onReady = func() {
		if !bm.readyReported.CompareAndSwap(false, true) {
			return
		}
		close(bm.readyCh)
		// 权限就绪后初始化动作接口；创造/飞行维护由设置显式启用。
		// 同步执行确保任务开始前已完成，避免任务中触发 getGameInterface 卡住心跳。
		// getGameInterface 首次失败时带重试（ensureGameInterfaceWithRetry），
		// 避免授权后首次初始化竞态导致动作接口缺失、NBT/告示牌被静默跳过。
		// 仍失败则放行（不阻塞连接），仅需动作接口的方块会跳过。
		bm.ensureGameInterfaceWithRetry()
		// 标记"正式进入"：onReady 在权限确认后触发，说明机器人真正在目标服务器内可操作。
		bm.mu.Lock()
		bm.inGame = true
		bm.mu.Unlock()
		fireBotReadyEvent(bm)
		// 启动玩家列表刷新（网易租赁服不发 PlayerList 包，用命令替代）
		go func() {
			time.Sleep(2 * time.Second)
			playerInfoMgr.RefreshPlayerList(bm)
			bm.logCh <- fmt.Sprintf("[玩家] 已刷新玩家列表: %d 人", playerInfoMgr.GetOnlineCount())
		}()
		// join/leave 周期轮询（仅当有监听时真正发命令）
		bm.startPlayerEventLoop()
		if origReady != nil {
			origReady()
		}
	}

	go bm.opConfirmLoop(conn)

	// 山头服（服务器号含非数字）需要额外模组同步
	if hasNonDigit(server) {
		debugLog("[*] 检测到山头服，发送模组同步...")
		go bm.sendModSync(conn)
	}

	// 启动游戏心跳（PlayerAuthInput 定时发送）
	// 在服务器权威移动模式下，客户端必须定期发送 PlayerAuthInput
	// 否则服务器会忽略位置变化或回退位置
	bm.startHeartbeat()
	bm.heartbeatEnabled = true // 默认开启，前端关面板时再关闭

	// 初始化寻路器
	InitPathfinder(bm)

	// 欢迎语（聊天消息）— 异步发送，不阻塞 Connect 返回
	// 尝试使用工具箱自定义提示词，如无则使用默认文本
	go func() {
		time.Sleep(500 * time.Millisecond)
		bm.yyNotice("join")

		// 匿名模式警告
		cfg := config.ReloadConfig()
		if cfg == nil || cfg.Token == "" {
			time.Sleep(600 * time.Millisecond)
			bm.yyNotice("anonymous")
		}
	}()

	// Wait for ready or disconnect. If disconnected before ready, retry.
	// 子机器人（BotIndex>0）无权限时不能永久阻塞等待 OP——那会让 FleetConnect 卡在
	// 第一个子机器人、后面的永远连不上。子机器人超时后先返回，OP 由主机器人稍后授予
	// （授予后 opConfirmLoop 仍在后台运行，检测到权限会再触发 onReady）。
	if bm.BotIndex > 0 {
		select {
		case <-bm.readyCh:
			return nil
		case <-bm.stopCh:
			bm.logCh <- fmt.Sprintf("  └→ 初始化阶段断开，重试")
			bm.mu.Lock()
			bm.disconnectLocked()
			bm.mu.Unlock()
			return fmt.Errorf("初始化期间连接断开")
		case <-time.After(3 * time.Second):
			bm.logCh <- "  └→ 子机器人无权限，先返回（等待主机器人授予 OP）"
			return nil
		}
	}
	// 主机器人：等待 OP（无超时，防止无权限时误报连接失败）
	select {
	case <-bm.readyCh:
		return nil
	case <-bm.stopCh:
		bm.logCh <- fmt.Sprintf("  └→ 初始化阶段断开，重试")
		bm.mu.Lock()
		bm.disconnectLocked()
		bm.mu.Unlock()
		return fmt.Errorf("初始化期间连接断开")
	}
}

// connectTanLobby 通过 TanLobby 中继加入本地联机房间。
// server 为房间号，password 为房间密码（无密码则空）。
func (bm *BotManager) connectTanLobby(token, roomID, auth, password string) error {
	bm.mu.Lock()
	if bm.connected {
		bm.disconnectLocked()
	}
	bm.mu.Unlock()

	cfg := client.Config{
		AuthServerAddress:    auth,
		RentalServerCode:     roomID,
		RentalServerPasscode: password,
		AuthServerToken:      token,
		OnLoginProgress: func(msg string) {
			debugLog("[连接诊断] %s", msg)
		},
		// 登录阶段收到的包提前喂给玩家信息管理器，捕获进服时的 PlayerList/皮肤广播。
		OnLoginPacket: func(pk packet.Packet) {
			playerInfoMgr.HandlePacket(pk, bm.logCh)
		},
	}

	bm.logCh <- fmt.Sprintf("[*] 正在加入房间 %s ...", roomID)

	const maxRetries = 5
	var c *client.Client
	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-bm.stopCh:
			bm.logCh <- fmt.Sprintf("  └→ 连接已取消")
			return fmt.Errorf("连接已取消")
		default:
		}
		debugLog("  └→ 第%d次尝试连接...", attempt)
		c, err = client.LoginTanLobbyRoom(cfg)
		if err == nil {
			break
		}
		if bm.onLoginFail != nil {
			bm.onLoginFail(attempt, err)
		}
		if attempt < maxRetries {
			// 显示具体错误，但前端只在"已重试N次"的最终消息才关弹窗
			bm.logCh <- fmt.Sprintf("  └→ 第%d次失败: %v，2秒后重试", attempt, err)
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		select {
		case <-bm.stopCh:
			bm.logCh <- fmt.Sprintf("  └→ 连接已取消")
			return fmt.Errorf("连接已取消")
		default:
		}
		bm.logCh <- fmt.Sprintf("✗ 加入房间失败(已重试%d次): %v", maxRetries, err)
		return fmt.Errorf("登录 tanlobby 失败：%w", err)
	}

	conn := c.Conn()
	if bm.cancelled.Load() {
		conn.Close()
		return fmt.Errorf("连接已取消")
	}
	id := conn.IdentityData()
	debugLog("✓ 已加入房间! 玩家: %s", id.DisplayName)
	debugLog("  ├ 房间ID: %s", roomID)
	debugLog("  ├ EID: %d", conn.GameData().EntityUniqueID)
	debugLog("  └ XUID: %s", id.XUID)

	bm.mu.Lock()
	if bm.cancelled.Load() {
		bm.mu.Unlock()
		conn.Close()
		return fmt.Errorf("连接已取消")
	}
	bm.client = c
	bm.conn = conn
	bm.display = id.DisplayName
	bm.xuid = id.XUID
	bm.eid = int64(conn.GameData().EntityUniqueID)
	bm.runtimeID = conn.GameData().EntityRuntimeID
	bm.gameMode = conn.GameData().PlayerGameMode
	bm.idStr = id.Identity
	bm.pos = conn.GameData().PlayerPosition
	bm.serverCode = roomID
	bm.botComponent = c.BotComponent
	bm.connectTime = time.Now()
	bm.stopCh = make(chan struct{})
	bm.connected = true
	bm.structureExportUnavailable = false
	bm.mu.Unlock()

	// 把自己注册到 players map
	opMu.Lock()
	players[bm.display] = bm.eid
	opMu.Unlock()

	yySenseReset(bm)
	debugLog("[*] 启动包监听器...")
	res := resources_control.NewResourcesControl(c)
	bm.mu.Lock()
	bm.resources = res
	bm.pktCh = make(chan packet.Packet, 256)
	bm.mu.Unlock()
	go bm.consumePackets()
	bm.registerPacketHandlers()
	debugLog("  └→ 监听器已启动")

	// OP 确认轮询
	origReady := bm.onReady
	bm.onReady = func() {
		if !bm.readyReported.CompareAndSwap(false, true) {
			return
		}
		close(bm.readyCh)
		// 动作接口初始化：失败时带重试，避免首次初始化竞态导致 NBT/告示牌被静默跳过。
		// 仍失败则放行（不阻塞连接），仅需动作接口的方块会跳过。
		bm.ensureGameInterfaceWithRetry()
		go func() {
			time.Sleep(2 * time.Second)
			playerInfoMgr.RefreshPlayerList(bm)
			bm.logCh <- fmt.Sprintf("[玩家] 已刷新玩家列表: %d 人", playerInfoMgr.GetOnlineCount())
		}()
		if origReady != nil {
			origReady()
		}
	}

	go bm.opConfirmLoop(conn)

	// 山头服（房间号含非数字）需要额外模组同步
	if hasNonDigit(roomID) {
		debugLog("[*] 检测到山头服，发送模组同步...")
		go bm.sendModSync(conn)
	}

	bm.startHeartbeat()
	bm.heartbeatEnabled = true
	InitPathfinder(bm)

	go func() {
		time.Sleep(500 * time.Millisecond)
		bm.yyNotice("join")
		cfg := config.ReloadConfig()
		if cfg == nil || cfg.Token == "" {
			time.Sleep(600 * time.Millisecond)
			bm.yyNotice("anonymous")
		}
	}()

	select {
	case <-bm.readyCh:
		return nil
	case <-bm.stopCh:
		bm.logCh <- fmt.Sprintf("  └→ 初始化阶段断开，重试")
		bm.mu.Lock()
		bm.disconnectLocked()
		bm.mu.Unlock()
		return fmt.Errorf("初始化期间连接断开")
	}
}

// registerPacketHandlers 订阅关心的包类型，收到后塞入 pktCh，由 consumePackets 顺序消费。
// 只订阅需要的包，避免海量区块包触发大量回调 goroutine（flowers 每包每监听器起一个 goroutine）。
func (bm *BotManager) registerPacketHandlers() {
	bm.mu.Lock()
	conn, resources, packets, stop := bm.conn, bm.resources, bm.pktCh, bm.stopCh
	bm.mu.Unlock()
	ids := []uint32{
		packet.IDText, packet.IDDisconnect, packet.IDRespawn,
		packet.IDCommandOutput, packet.IDStructureTemplateDataResponse,
		packet.IDUpdateAbilities, packet.IDAdventureSettings,
		packet.IDAddPlayer, packet.IDPlayerList, packet.IDPlayerSkin,
		packet.IDSyncSkin, packet.IDConfirmSkin, packet.IDPyRpc,
		packet.IDLevelChunk, packet.IDSubChunk,
		packet.IDPlayerLocation, packet.IDMovePlayer,
		packet.IDMobEquipment, packet.IDChangeDimension,
		packet.IDSetTime, // TPS 计算
		packet.IDSetPlayerGameType, packet.IDUpdatePlayerGameType,
		packet.IDUpdateAttributes, packet.IDSetHealth, packet.IDActorEvent,
		packet.IDAddActor, packet.IDRemoveActor, packet.IDMoveActorAbsolute, packet.IDMoveActorDelta, packet.IDUpdateBlock,
		packet.IDAnimate, packet.IDAddItemActor, packet.IDTakeItemActor,
	}
	wanted := map[uint32]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	id, err := resources.PacketListener().ListenPacketFiltered(nil, func(pk packet.Packet) bool {
		return pk != nil && (wanted[pk.ID()] || packetHubWants(yyPacketName(pk)) || yyCaptureWants(bm, yyPacketName(pk)))
	}, func(pk packet.Packet, connCloseErr error) {
		if connCloseErr != nil {
			if bm.disconnectConnection(conn) {
				select {
				case bm.logCh <- fmt.Sprintf("✗ 连接断开: %v", connCloseErr):
				default:
				}
			}
			return
		}
		if pk == nil || (!wanted[pk.ID()] && !packetHubWants(yyPacketName(pk)) && !yyCaptureWants(bm, yyPacketName(pk))) {
			return
		}
		select {
		case packets <- pk:
		case <-stop:
		}
	})
	if err != nil {
		bm.logCh <- fmt.Sprintf("✗ 注册包监听器失败: %v", err)
		return
	}
	bm.listenerID = id
}

// handleSetTime 用服务器 SetTime 包的时间戳推算服务器 TPS。
// 算法参考 ToolDelta 前置-TPS计算器：TPS = Δ服务器时间(ticks) / Δ真实时间(秒)，clamp 到 20。
// 首个包仅建立基线（避免把 lastRealTime 的零值当作真实耗时算出接近 0 的垃圾值）。
func (bm *BotManager) handleSetTime(p *packet.SetTime) {
	now := time.Now()
	bm.mu.Lock()
	if bm.lastRealTime.IsZero() {
		// 首个包：建立基线，不计算
		bm.lastServerTime = p.Time
		bm.lastRealTime = now
		bm.mu.Unlock()
		return
	}
	stDelta := int64(p.Time) - int64(bm.lastServerTime)
	rtDelta := now.Sub(bm.lastRealTime).Seconds()
	bm.lastServerTime = p.Time
	bm.lastRealTime = now
	if stDelta <= 0 || rtDelta <= 0 {
		bm.mu.Unlock()
		return
	}
	tps := float64(stDelta) / rtDelta
	if tps > 20 {
		tps = 20
	}
	bm.tps = tps
	bm.mu.Unlock()
}

// GetTPS 返回最近计算的服务器 TPS（线程安全）。
func (bm *BotManager) GetTPS() float64 {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.tps
}

// consumePackets 单 goroutine 顺序消费 pktCh，保持与原读包循环一致的处理时序。
func (bm *BotManager) consumePackets() {
	defer func() {
		if r := recover(); r != nil {
			select {
			case bm.logCh <- fmt.Sprintf("✗ 数据包处理panic: %v", r):
			default:
			}
		}
	}()
	for {
		var pk packet.Packet
		select {
		case <-bm.stopCh:
			return
		case pk = <-bm.pktCh:
		}

		// 追踪 OP 状态（仅记录）
		func() {
			defer func() { recover() }()
			HandleOPPacket(pk, bm.logCh)
		}()
		// 追踪玩家信息（在线列表、位置、手持物品、权限）
		func() {
			defer func() { recover() }()
			bm.HandlePlayerPacket(pk)
		}()
		yySenseHandle(bm, pk)
		yyCapturePacket(bm, pk)
		// 通用数据包订阅：插件/AI 订阅的类型才序列化分发（packet_hub.go）
		func() {
			defer func() { recover() }()
			packetHubDispatch(bm, pk)
		}()

		switch p := pk.(type) {
		case *packet.SetPlayerGameType:
			bm.mu.Lock()
			bm.gameMode = p.GameType
			bm.mu.Unlock()
		case *packet.UpdatePlayerGameType:
			bm.mu.Lock()
			if p.PlayerUniqueID == bm.eid {
				bm.gameMode = p.GameType
			}
			bm.mu.Unlock()
		case *packet.SetTime:
			bm.handleSetTime(p)
			firePacketEvent(bm, "time", map[string]any{"time": p.Time})

		case *packet.Animate:
			firePacketEvent(bm, "animate", map[string]any{
				"entity_runtime_id": p.EntityRuntimeID,
				"action_type":       p.ActionType,
			})

		case *packet.LevelEvent:
			firePacketEvent(bm, "level_event", map[string]any{
				"event_type": p.EventType,
				"x":          float64(p.Position[0]),
				"y":          float64(p.Position[1]),
				"z":          float64(p.Position[2]),
				"data":       p.EventData,
			})

		case *packet.Text:
			bm.logCh <- fmt.Sprintf("[聊天] %s: %s", p.SourceName, p.Message)
			firePacketEvent(bm, "text", map[string]any{
				"player":  p.SourceName,
				"message": p.Message,
				"type":    int32(p.TextType),
			})
			// 投递给等待该玩家消息的 WaitMsg 会话（仅普通聊天）
			if p.TextType == packet.TextTypeChat {
				if bm == botMgr {
					yyQQHelperRecord("chat", p.SourceName, p.Message)
					go yyQQGameChat(p.SourceName, p.Message)
				}
				// 会话优先：玩家有活跃 WaitMsg 会话时，消息归会话消费（供参数提问）；
				// 无会话时才匹配触发词。
				if !hasWaitMsg(p.SourceName) {
					bm.handleChat(p.SourceName, p.Message)
				}
				deliverWaitMsg(p.SourceName, p.Message)
				// AI wait_chat 观察者（独立缓冲，不影响触发词/会话）
				func() {
					defer func() { recover() }()
					chatHubDispatch(bm, p.SourceName, p.Message)
				}()
			}
			// /say 回退检测：如果自己发出了测试消息，清除标记通知 opConfirmLoop
			bm.mu.Lock()
			if bm.sayTestMsg != "" && p.SourceName == bm.display && strings.Contains(p.Message, bm.sayTestMsg) {
				bm.sayTestMsg = ""
				debugLog("[权限] ✓ /say 回显成功: 拥有命令权限")
			}
			bm.mu.Unlock()

		case *packet.Disconnect:
			bm.logCh <- fmt.Sprintf("✗ 被服务器踢出: %s", p.Message)
			bm.Disconnect()
			return

		case *packet.Respawn:
			bm.pos = p.Position
			bm.logCh <- fmt.Sprintf("[重生] 位置: (%.0f, %.0f, %.0f)", p.Position[0], p.Position[1], p.Position[2])

		case *packet.CommandOutput:
			// 用户不需要服务器命令广播日志
			_ = p.OutputMessages
			// /say 回退检测：CommandOutput 成功时清除标记通知 opConfirmLoop
			bm.mu.Lock()
			if bm.sayTestMsg != "" && p.SuccessCount > 0 {
				bm.sayTestMsg = ""
				debugLog("[权限] ✓ /say CommandOutput 成功: 拥有命令权限")
			}
			bm.mu.Unlock()
			// OP verification fallback uses a read-only command
			bm.mu.Lock()
			verifying := bm.verifyPending
			bm.mu.Unlock()
			if verifying {
				bm.mu.Lock()
				bm.verifyPending = false
				bm.mu.Unlock()
				if p.SuccessCount > 0 {
					debugLog("[权限] ✓ 命令验证通过: 拥有管理员权限！")
					bm.setOP(true, 3, "CmdVerified")
					if bm.onReady != nil {
						bm.onReady()
					}
				} else {
					bm.logCh <- "[权限] ✗ 命令验证失败: 没有管理员权限"
					bm.resetOPIfUnconfirmed()
				}
			} else if p.SuccessCount > 0 {
				bm.checkSelfOP()
			}
		case *packet.StructureTemplateDataResponse:
			if ch, ok := structureCallbacks.Load(p.StructureName); ok {
				select {
				case ch.(chan map[string]any) <- p.StructureTemplate:
				default:
				}
			}

		case *packet.UpdateAbilities:
			isMe := p.AbilityData.EntityUniqueID == bm.eid
			if isMe {
				bm.setOP(p.AbilityData.CommandPermissions >= packet.CommandPermissionLevelGameDirectors,
					uint32(p.AbilityData.CommandPermissions), "UpdateAbilities")
				// 同步飞行状态
				bm.mu.Lock()
				if len(p.AbilityData.Layers) > 0 {
					bm.isFlying = p.AbilityData.Layers[0].Values&protocol.AbilityFlying != 0
				}
				bm.mu.Unlock()
			}

		case *packet.AdventureSettings:
			isMe := p.PlayerUniqueID == bm.eid
			if isMe {
				bm.setOP(p.CommandPermissionLevel >= packet.CommandPermissionLevelGameDirectors,
					p.CommandPermissionLevel, "AdventureSettings")
			}

		case *packet.PlayerList:

		case *packet.AddPlayer:
			if p.Username == bm.display {
				bm.mu.Lock()
				bm.pos = p.Position
				bm.mu.Unlock()
				pfLog("AddPlayer: position=(%.2f,%.2f,%.2f)", p.Position[0], p.Position[1], p.Position[2])
				bm.setOP(p.AbilityData.CommandPermissions >= packet.CommandPermissionLevelGameDirectors,
					uint32(p.AbilityData.CommandPermissions), "AddPlayer")
			}

		case *packet.SetActorData:

		case *packet.MoveActorDelta:

		case *packet.MoveActorAbsolute:

		case *packet.LevelChunk:
			// 只在世界数据收集开启时解析并存储区块数据（前端打开寻路/飞行面板时启用）
			if globalWorld.IsEnabled() {
				globalWorld.ParseLevelChunk(p.Position.X(), p.Position.Z(), p.SubChunkCount, p.RawPayload)
				// 如果服务器使用 SubChunkRequest 模式，发送 SubChunkRequest 获取方块数据
				if p.SubChunkCount == protocol.SubChunkRequestModeLimited || p.SubChunkCount == protocol.SubChunkRequestModeLimitless {
					select {
					case bm.chunkReqSem <- struct{}{}:
						go func(cx, cz int32, dim int32) {
							defer func() { <-bm.chunkReqSem }()
							base := protocol.SubChunkPos{cx, -4, cz}
							offsets := make([]protocol.SubChunkOffset, 24)
							for dy := int8(0); dy < 24; dy++ {
								offsets[dy] = protocol.SubChunkOffset{0, dy, 0}
							}
							bm.WritePacket(&packet.SubChunkRequest{
								Dimension: dim,
								Position:  base,
								Offsets:   offsets,
							})
						}(p.Position.X(), p.Position.Z(), p.Dimension)
					default:
					}
				}
			}

		case *packet.SubChunk:
			if globalWorld.IsEnabled() {
				globalWorld.ParseSubChunk(p)
			}

		case *packet.ChangeDimension:
			bm.mu.Lock()
			bm.dimension = p.Dimension
			bm.pos = p.Position
			bm.mu.Unlock()
		case *packet.NetworkChunkPublisherUpdate:

		case *packet.SyncActorProperty:

		case *packet.PlayerLocation:
			bm.mu.Lock()
			bm.pos = p.Position
			bm.mu.Unlock()

		case *packet.MovePlayer:
			if p.EntityRuntimeID != bm.runtimeID {
				break
			}
			bm.mu.Lock()
			bm.pos = p.Position
			bm.mu.Unlock()

		default:
			// 忽略其他包
		}
	}
}

// confirmReadyByPacket 包检测已权威确认 OP 时：无需命令验证，直接完成初始化。
// 返回 true 表示已处理（调用方应 return），避免包已确认 OP 但命令验证一直失败的场景空转。
func (bm *BotManager) confirmReadyByPacket(conn *minecraft.Conn) bool {
	bm.mu.Lock()
	opC := bm.opPacketConfirmed
	perm := bm.cmdPerm
	bm.mu.Unlock()
	if !opC {
		return false
	}
	bm.setOP(true, perm, "PacketConfirmed")
	bm.mu.Lock()
	bm.opSent = true
	bm.mu.Unlock()
	if conn != nil {
	}
	if bm.onReady != nil {
		bm.onReady()
	}
	return true
}

func (bm *BotManager) opConfirmLoop(conn *minecraft.Conn) {
	// 快速确认：不等包，直接发 WS 命令看返回值
	// WS 命令（AutomationPlayer origin）的 CommandOutput 可靠
	time.Sleep(1500 * time.Millisecond) // 等登录基本完成

	for attempt := 1; attempt <= 2; attempt++ {
		select {
		case <-bm.stopCh:
			return
		default:
		}
		if bm.confirmReadyByPacket(conn) {
			return
		}

		bm.logCh <- fmt.Sprintf("[权限] 验证权限(第%d次)...", attempt)

		// 使用只读 testfor 验证指令权限，保留当前游戏模式
		cmd := yyOPProbeCommand
		resp, isTimeout, err := bm.SendWSCommandWithTimeout(cmd, time.Second)

		if err != nil {
			if isTimeout {
				bm.logCh <- fmt.Sprintf("[权限] 命令超时(第%d次)", attempt)
				continue
			}
			// gameInterface 可能未就绪 → 降级：直接发命令包
			bm.logCh <- fmt.Sprintf("[权限] gameInterface 未就绪，降级到原始包检测")
			bm.detectOPLegacy(conn)
			return
		}

		if resp != nil && resp.SuccessCount > 0 {
			debugLog("[权限] ✓ 命令验证通过: 拥有管理员权限！")
			bm.setOP(true, 3, "CmdVerified")
			if bm.onReady != nil {
				bm.onReady()
			}
			return
		}

		bm.logCh <- fmt.Sprintf("[权限] 命令验证失败(第%d次)", attempt)
		time.Sleep(1 * time.Second)
	}

	// 两次都失败 → 进入等待授权模式：只发两遍提示，之后静默检测
	bm.logCh <- "[权限] 权限验证未通过，进入等待授权模式"
	bm.noPermNotifyLoop(conn)
}

// detectOPLegacy 降级到原始包检测 + /say 兜底
func (bm *BotManager) detectOPLegacy(conn *minecraft.Conn) {
	noOPStart := time.Now()
	proactiveDone := false
	sayTestDone := false
	noPermConfirmed := false
	lastNotifyTime := time.Time{}

	for {
		select {
		case <-bm.stopCh:
			return
		default:
		}
		if bm.confirmReadyByPacket(conn) {
			return
		}

		bm.mu.Lock()
		isOP := bm.isOP
		opSent := bm.opSent
		bm.mu.Unlock()

		if isOP && opSent {
			return
		}

		// 从包中检测到 OP 但未验证
		if isOP && !opSent {
			bm.logCh <- "[权限] 检测到OP包，尝试命令验证..."
			cmd := yyOPProbeCommand
			resp, _, err := bm.SendWSCommandWithTimeout(cmd, time.Second)
			if err == nil && resp != nil && resp.SuccessCount > 0 {
				debugLog("[权限] ✓ 命令验证通过")
				bm.setOP(true, 3, "CmdVerified")
				if bm.onReady != nil {
					bm.onReady()
				}
				return
			}
			bm.resetOPIfUnconfirmed()
		}

		// 主动验证（无 OP 包时）
		if !proactiveDone && time.Since(noOPStart) > 8*time.Second {
			proactiveDone = true
			bm.logCh <- "[权限] 未检测到OP包，主动发送命令验证..."
			cmd := yyOPProbeCommand
			resp, _, err := bm.SendWSCommandWithTimeout(cmd, time.Second)
			if err == nil && resp != nil && resp.SuccessCount > 0 {
				debugLog("[权限] ✓ 主动验证通过")
				bm.setOP(true, 3, "CmdVerified")
				if bm.onReady != nil {
					bm.onReady()
				}
				return
			}
			bm.logCh <- "[权限] 主动验证超时，假定无权限"
		}

		// /say 兜底
		if proactiveDone && !sayTestDone && time.Since(noOPStart) > 12*time.Second {
			sayTestDone = true
			bm.logCh <- "[权限] 尝试 /say 测试..."
			testMsg := "Welcome"
			bm.mu.Lock()
			bm.sayTestMsg = testMsg
			bm.mu.Unlock()
			conn.WritePacket(&packet.CommandRequest{
				CommandLine: "say " + testMsg,
				CommandOrigin: protocol.CommandOrigin{
					Origin:         protocol.CommandOriginPlayer,
					UUID:           uuid.MustParse(bm.idStr),
					PlayerUniqueID: bm.eid,
				},
				Version: 105,
			})
			time.Sleep(1 * time.Second)
			bm.mu.Lock()
			stillWaiting := bm.sayTestMsg != ""
			bm.mu.Unlock()
			if !stillWaiting {
				debugLog("[权限] ✓ /say 回显成功: 拥有基本命令权限")
				bm.setOP(true, 1, "SayTest")
				if bm.onReady != nil {
					bm.onReady()
				}
				return
			}
			bm.sayTestMsg = ""
			bm.logCh <- "[权限] /say 未回显，确认无命令权限"
			noPermConfirmed = true
			lastNotifyTime = time.Now()
			bm.sendNoPermNotify(conn)
		}

		// 无权限时：每5秒用权威命令 /say 重新检测权限。
		// 先检测——通过则直接完成初始化，只有未通过才发送求授权消息
		if noPermConfirmed && time.Since(lastNotifyTime) > 5*time.Second {
			lastNotifyTime = time.Now()
			bm.logCh <- "[权限] 重新检测权限..."
			resp, _, err := bm.SendWSCommandWithTimeout("say Welcome", time.Second)
			if err == nil && resp != nil && resp.SuccessCount > 0 {
				debugLog("[权限] ✓ 命令验证通过: 拥有管理员权限！")
				bm.setOP(true, 3, "CmdVerified")
				if bm.onReady != nil {
					bm.onReady()
				}
				return
			}
			bm.sendNoPermNotify(conn)
		}

		time.Sleep(2 * time.Second)
	}
}

// noPermNotifyLoop 无权限等待授权模式：
//  1. 发两遍"本机器人由XXX召唤，请给予权限"（间隔5秒）
//  2. 之后静默，用包检测（UpdateAbilities/AdventureSettings）判断机器人自身 OP，
//     并用原始 /say 回显兜底
//  3. 检测到权限后必定发送"已完成初始化"
func (bm *BotManager) noPermNotifyLoop(conn *minecraft.Conn) {
	// 第一遍提示
	bm.sendNoPermNotify(conn)
	notifyCount := 1

	for {
		select {
		case <-bm.stopCh:
			return
		default:
		}

		time.Sleep(5 * time.Second)

		// 包检测：授予 OP 时服务器一定发 UpdateAbilities/AdventureSettings，
		// HandleOPPacket 已填充 opPlayers[bm.display]，checkSelfOP 据此判断
		bm.checkSelfOP()
		bm.mu.Lock()
		isOP := bm.isOP
		bm.mu.Unlock()
		if isOP {
			debugLog("[权限] ✓ 包检测到 OP: 拥有管理员权限！")
			bm.setOP(true, 3, "CmdVerified")
			if bm.onReady != nil {
				bm.onReady()
			}
			return
		}

		// 原始 /say 回显兜底检测（不依赖 gameInterface）
		bm.logCh <- "[权限] 静默检测权限..."
		testMsg := "Welcome"
		bm.mu.Lock()
		bm.sayTestMsg = testMsg
		bm.mu.Unlock()
		conn.WritePacket(&packet.CommandRequest{
			CommandLine: "say " + testMsg,
			CommandOrigin: protocol.CommandOrigin{
				Origin:         protocol.CommandOriginPlayer,
				UUID:           uuid.MustParse(bm.idStr),
				PlayerUniqueID: bm.eid,
			},
			Version: 105,
		})
		time.Sleep(1 * time.Second)
		bm.mu.Lock()
		stillWaiting := bm.sayTestMsg != ""
		bm.sayTestMsg = ""
		bm.mu.Unlock()

		if !stillWaiting {
			// 检测到权限 → 必定发送"已完成初始化"
			debugLog("[权限] ✓ /say 回显成功: 拥有命令权限！")
			bm.setOP(true, 1, "SayTest")
			if bm.onReady != nil {
				bm.onReady()
			}
			return
		}

		// 未检测到权限，且还没发满两遍
		if notifyCount < 2 {
			bm.sendNoPermNotify(conn)
			notifyCount++
		}
	}
}

// sendChatMessage 发送普通聊天消息，自动限流（每2秒1条）和截断（最长50字符）。
// 只对普通聊天 (packet.TextTypeChat) 生效，指令类不受影响。
func (bm *BotManager) sendChatMessage(msg string) {
	bm.chatMu.Lock()
	defer bm.chatMu.Unlock()

	// 截断到50字符（按 rune 计算，尊重中文等宽字符）
	runes := []rune(msg)
	if len(runes) > 50 {
		runes = runes[:50]
		msg = string(runes)
	}

	// 限流：距离上次发送不足2秒则等待
	elapsed := time.Since(bm.lastChatTime)
	if elapsed < 2*time.Second {
		time.Sleep(2*time.Second - elapsed)
	}

	bm.mu.Lock()
	conn := bm.conn
	display := bm.display
	xuid := bm.xuid
	bm.mu.Unlock()

	if conn == nil {
		return
	}

	conn.WritePacket(&packet.Text{
		TextType:   packet.TextTypeChat,
		Message:    msg,
		SourceName: display,
		XUID:       xuid,
	})
	bm.lastChatTime = time.Now()
}

// sendNoPermNotify 无权限时发送求授权提示消息。受 sendChatMessage 限流保护。
func (bm *BotManager) sendNoPermNotify(conn *minecraft.Conn) {
	if conn == nil {
		return
	}
	bm.yyNotice("permission")
}

func (bm *BotManager) checkSelfOP() {
	opMu.Lock()
	info, ok := opPlayers[bm.display]
	opMu.Unlock()
	if ok && info.CmdPerm >= packet.CommandPermissionLevelGameDirectors {
		bm.setOP(true, uint32(info.CmdPerm), "opPlayers")
	}
}

func (bm *BotManager) setOP(op bool, perm uint32, source string) {
	bm.mu.Lock()
	wasOP := bm.isOP
	bm.isOP = op
	// 包来源与命令确认都会同步 opPacketConfirmed；非 OP 的包来源也会清掉它（权威）。
	bm.opPacketConfirmed = op
	bm.cmdPerm = perm
	// opSent 只在命令验证通过(source=CmdVerified)或超时回退(source=TimeoutFallback)时设置
	// 包检测(source=UpdateAbilities/AdventureSettings/opPlayers)不设置 opSent
	markSent := (source == "CmdVerified" || source == "TimeoutFallback")
	if markSent {
		bm.opSent = true
	}
	bm.mu.Unlock()

	if wasOP != op {
		debugLog("[权限] OP状态变化: isOP=%v cmdPerm=%d (来源:%s)", op, perm, source)
	}
	if op && markSent {
		debugLog("[权限] ✓ OP确认! cmdPerm=%d (来源:%s)", perm, source)
	}
	// 主机器人取得 OP：自动加载所有未被持久禁用的插件（只触发一次，wasOP!=op 已保证）。
	if op && !wasOP && bm.isMain {
		debugLog("[插件] 主机器人已获得OP，开始自动加载插件...")
		go pluginController.AutoLoadPlugins(bm)
	}
}

// resetOPIfUnconfirmed 命令验证失败时的守卫回退：仅当包检测未权威确认 OP 才清掉 isOP，
// 避免"gamemode creative 验证失败"覆盖 UpdateAbilities/AdventureSettings 已确认的真 OP。
func (bm *BotManager) resetOPIfUnconfirmed() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if !bm.opPacketConfirmed {
		bm.isOP = false
		bm.opSent = false
	}
}

func (bm *BotManager) disconnectLocked() {
	// 0. 先关心跳
	globalWorld.Reset()
	bm.stopHeartbeat()
	// 1. 再关 stopCh → 停止所有生产者（心跳、包监听等）
	if bm.stopCh != nil {
		select {
		case <-bm.stopCh:
		default:
			close(bm.stopCh)
		}
	}
	// 2. 再关 logCh → SSE goroutine 会在排空缓冲后自然退出
	// Keep the bounded log channel alive until pending login producers finish.
	if bm.conn != nil {
		bm.conn.Close()
		bm.conn = nil
	}
	bm.connected = false
	// 主机器人断开：停用（瞬态停止运行）全部插件，不改变禁用状态，重连后自动再加载。
	if bm.isMain {
		go pluginController.StopAllPlugins(bm)
	}
}

func (bm *BotManager) Disconnect() {
	bm.cancelled.Store(true)
	// 先在锁保护下读取 conn 和必要字段，避免持有锁时调用 WritePacket
	bm.mu.Lock()
	conn := bm.conn
	connected := bm.connected
	display := bm.display
	idStr := bm.idStr
	eid := bm.eid
	bm.mu.Unlock()

	// 发送断开包（不持有 bm.mu，避免 WritePacket 阻塞级联）
	if conn != nil && connected {
		// 1. Tell server we're leaving — no OP required
		conn.WritePacket(&packet.Disconnect{
			Message: "游园工具箱断开连接",
		})
		// 2. Also try /kick if we have OP — graceful server-side removal
		conn.WritePacket(&packet.CommandRequest{
			CommandLine: "kick " + `"` + display + `"`,
			CommandOrigin: protocol.CommandOrigin{
				Origin:         protocol.CommandOriginPlayer,
				UUID:           uuid.MustParse(idStr),
				PlayerUniqueID: eid,
			},
			Version: 105,
		})
		// Give packets time to flush
		time.Sleep(300 * time.Millisecond)
	}

	// 重新获取锁执行清理
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.conn == conn {
		bm.disconnectLocked()
	}
}

// An old listener may finish after a new connection has already been installed.
// Its close notification must never disconnect the replacement connection.
func (bm *BotManager) disconnectConnection(conn *minecraft.Conn) bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if conn == nil || bm.conn != conn {
		return false
	}
	bm.disconnectLocked()
	return true
}

func (bm *BotManager) IsConnected() bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.connected && (bm.conn == nil || bm.conn.Context().Err() == nil)
}

// IsJoined 判断机器人是否"正式进入"服务器：不仅要已连接，还要 onReady 触发过
// （权限确认完成，机器人真正在目标服务器内可操作）。调度器用它判断机器人是否可用，
// 避免把"连接了但误进别服/未真正进服"的机器人当成在线并派发任务。
func (bm *BotManager) IsJoined() bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.connected && bm.inGame
}

func (bm *BotManager) IsOP() bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	// 包检测权威确认的 OP 即使命令验证失败（isOP 被守卫回退）也成立——防止真 OP 被误判。
	return bm.isOP || bm.opPacketConfirmed
}

func (bm *BotManager) ServerCode() string {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.serverCode
}

// SendConsole sends chat or command input from the console tab.
// Plain text → Text packet. /command → WS Automation channel with response.
// Returns structured result for the HTTP handler to return directly to the frontend,
// avoiding log-channel flooding.
func (bm *BotManager) sendConsoleOne(input string) (map[string]any, error) {
	if len(input) > 0 && input[0] == '/' {
		cmd := input[1:]
		bm.logCh <- fmt.Sprintf("[控制台] %s", input)
		// 用 WS 自动化通道发送命令并等待响应（CommandOriginAutomationPlayer）
		// 返回结果直接走 HTTP 响应，不走 SSE 日志，避免洪水
		resp, isTimeout, err := bm.SendWSCommandWithTimeout(cmd, 10*time.Second)
		if err != nil {
			if isTimeout {
				bm.logCh <- fmt.Sprintf("[控制台] 命令超时: %s", cmd)
				return map[string]any{"ok": true, "note": "命令已发送，等待响应超时（可能已执行）"}, nil
			}
			return nil, err
		}
		// 解析命令输出
		result := map[string]any{"ok": true, "cmd": input}
		messages := make([]string, 0, len(resp.OutputMessages))
		for _, msg := range resp.OutputMessages {
			if msg.Message != "" {
				messages = append(messages, formatCommandMessage(msg.Message, msg.Parameters))
			}
		}
		if len(messages) > 0 {
			result["output"] = messages
		}
		result["success"] = resp.SuccessCount > 0
		result["success_count"] = resp.SuccessCount
		return result, nil
	}
	// 普通聊天消息
	bm.mu.Lock()
	if !bm.connected || bm.conn == nil {
		bm.mu.Unlock()
		return nil, fmt.Errorf("未连接")
	}
	bm.mu.Unlock()
	bm.sendChatMessage(input)
	bm.logCh <- fmt.Sprintf("[聊天] %s", input)
	return map[string]any{"ok": true, "type": "chat"}, nil
}

func (bm *BotManager) SendCommand(cmd string) error {
	return bm.SendAICommand(cmd)
}

var structureCallbacks sync.Map

func (bm *BotManager) RequestStructure(pos [3]int32, size [3]int32, timeout time.Duration) (map[string]any, error) {
	if err := yyValidateRegion(pos, size); err != nil {
		return nil, err
	}
	bm.mu.Lock()
	fallback := bm.structureExportUnavailable
	bm.mu.Unlock()
	if fallback {
		return bm.yyReadRegionSubChunks(pos, size)
	}
	data, err := bm.requestStructureTemplate(pos, size, timeout)
	if err == nil {
		return data, nil
	}
	if !bm.IsConnected() {
		return nil, err
	}
	bm.mu.Lock()
	bm.structureExportUnavailable = true
	bm.mu.Unlock()
	bm.logCh <- "[导出] 结构模板未返回有效数据，改用服务器子区块读取；不会把缺失数据当空气"
	data, fallbackErr := bm.yyReadRegionSubChunks(pos, size)
	if fallbackErr != nil {
		return nil, fmt.Errorf("结构通道: %v；子区块通道: %w", err, fallbackErr)
	}
	return data, nil
}

func (bm *BotManager) requestStructureTemplate(pos [3]int32, size [3]int32, timeout time.Duration) (map[string]any, error) {
	bm.mu.Lock()
	if !bm.connected || bm.conn == nil {
		bm.mu.Unlock()
		return nil, fmt.Errorf("未连接")
	}

	// First save the area as a structure so NBT data (commands, chests, etc.) is included.
	// /structure save must be called before ExportFromSave or the server won't return block NBT.
	name := uuid.New().String()
	safeName := strings.ReplaceAll(name, "-", "")
	defer bm.cleanupStructure(safeName)
	saveCmd := fmt.Sprintf(`structure save "%s" %d %d %d %d %d %d`,
		safeName,
		pos[0], pos[1], pos[2],
		pos[0]+size[0]-1, pos[1]+size[1]-1, pos[2]+size[2]-1,
	)
	bm.mu.Unlock()

	// Use WS command to save and wait for completion
	if bm.gameInterface != nil {
		resp, timedOut, saveErr := bm.gameInterface.Commands().SendWSCommandWithTimeout(saveCmd, 8*time.Second)
		if saveErr != nil {
			return nil, fmt.Errorf("结构保存失败: %w", saveErr)
		}
		if !timedOut && (resp == nil || resp.SuccessCount == 0) {
			return nil, fmt.Errorf("服务器拒绝结构保存")
		}
		bm.gameInterface.Commands().AwaitChangesGeneral()
	} else {
		bm.mu.Lock()
		conn := bm.conn
		bm.mu.Unlock()
		if conn != nil {
			if e := conn.WritePacket(&packet.SettingsCommand{
				CommandLine:    saveCmd,
				SuppressOutput: true,
			}); e != nil {
				return nil, e
			}
		}
		time.Sleep(1 * time.Second)
	}

	bm.mu.Lock()
	respCh := make(chan map[string]any, 1)
	structureCallbacks.Store(safeName, respCh)
	structureCallbacks.Store("mystructure:"+safeName, respCh)
	defer structureCallbacks.Delete(safeName)
	defer structureCallbacks.Delete("mystructure:" + safeName)
	conn := bm.conn
	eid := bm.eid
	bm.mu.Unlock()

	if conn != nil {
		if e := conn.WritePacket(&packet.StructureTemplateDataRequest{
			StructureName: safeName,
			Position:      protocol.BlockPos{pos[0], pos[1], pos[2]},
			Settings: protocol.StructureSettings{
				PaletteName:               "default",
				IgnoreEntities:            true,
				IgnoreBlocks:              false,
				Size:                      protocol.BlockPos{size[0], size[1], size[2]},
				Offset:                    protocol.BlockPos{0, 0, 0},
				LastEditingPlayerUniqueID: eid,
				Rotation:                  0, Mirror: 0, Integrity: 100, Seed: 0,
				AllowNonTickingChunks: false,
			},
			RequestType: packet.StructureTemplateRequestExportFromSave,
		}); e != nil {
			return nil, e
		}
	}

	select {
	case data := <-respCh:
		if len(data) == 0 {
			return nil, fmt.Errorf("服务器结构响应不含数据")
		}
		structureCallbacks.Delete(safeName)
		return data, nil
	case <-time.After(timeout):
		structureCallbacks.Delete(safeName)
		return nil, fmt.Errorf("等待结构数据超时")
	}
}

// cleanupStructure deletes a saved structure from the server to avoid accumulation.
func (bm *BotManager) cleanupStructure(name string) {
	if bm.gameInterface != nil {
		bm.gameInterface.Commands().SendWSCommand(
			fmt.Sprintf(`structure delete "%s"`, name),
		)
	} else {
		bm.mu.Lock()
		conn := bm.conn
		bm.mu.Unlock()
		if conn != nil {
			conn.WritePacket(&packet.SettingsCommand{
				CommandLine:    fmt.Sprintf(`structure delete "%s"`, name),
				SuppressOutput: true,
			})
		}
	}
}

func (bm *BotManager) SendTellraw(msg string) error {
	return bm.SendWOCmd(fmt.Sprintf(`tellraw @a {"rawtext":[{"text":"%s"}]}`, msg))
}

func (bm *BotManager) SendWOCmd(cmd string) error {
	return bm.SendAICommand(cmd)
}

func (bm *BotManager) SendTitleraw(target, mode, text string) error {
	json := fmt.Sprintf(`{"rawtext":[{"text":"%s"}]}`, text)
	cmd := fmt.Sprintf("titleraw %s %s %s", target, mode, json)
	return bm.SendWOCmd(cmd)
}

func (bm *BotManager) TP(x, y, z int) error {
	bm.mu.Lock()
	if !bm.connected {
		bm.mu.Unlock()
		return fmt.Errorf("未连接")
	}
	bm.mu.Unlock()
	return bm.SendAICommand(fmt.Sprintf("tp @s %d %d %d", x, y, z))
}

// quotedName returns the bot's display name wrapped in double quotes
// for use in commands (e.g. "botname" instead of @s).
// display is set once during Connect and never modified — safe to read without lock.
// SendBlockCmd 发送方块操作命令（setblock/fill），走网易 AI 命令通道（PyRpc）。
// AI 命令通道相比普通 CommandRequest 拥有更高权限，在网易租赁服上更可靠。
func (bm *BotManager) SendBlockCmd(cmd string) error {
	return bm.SendAICommand(cmd)
}

// botIsAir 构造一个检查世界坐标 (x,y,z) 是否为空气的函数，供 keep 模式使用。
// testforblock 匹配时命令成功返回（SuccessCount>0）。连接不可用时返回 true，
// 避免过度拦截、保持原导入行为。
func botIsAir(bm *BotManager) func(x, y, z int32) bool {
	return func(x, y, z int32) bool {
		if bm == nil || bm.conn == nil {
			return true
		}
		resp, isTimeout, err := bm.SendWSCommandWithTimeout(
			fmt.Sprintf("testforblock %d %d %d minecraft:air", x, y, z),
			1*time.Second,
		)
		return !isTimeout && err == nil && resp != nil && resp.SuccessCount > 0
	}
}

// normalizeDimension 将前端维度名转为 Minecraft execute in 可用的维度名。
// 前端传 "nether"，但 Minecraft 用 "the_nether"。
// DM 是网易自定义维度（如 dm3, dm4），保留原样以便 execute in dm3 run 使用。
func normalizeDimension(name string) string {
	lower := strings.ToLower(name)
	switch lower {
	case "nether", "minecraft:nether", "the_nether", "minecraft:the_nether":
		return "the_nether"
	case "end", "the_end", "minecraft:the_end":
		return "the_end"
	}
	// 检查是否为 DM 维度（dm3, dm4, dm5, ...）
	if strings.HasPrefix(lower, "dm") {
		// 保留原样，execute in dm3 run 在网易租赁服上有效
		return lower
	}
	// 其他自定义维度也保留原样
	return lower
}

// dimNameToID 将维度名映射为 MCBE 维度 ID（0=overworld, 1=nether, 2=end）。
// DM 维度（如 dm3）返回对应的数字 ID（3）。
// 供 export_subchunk.go、sign_place.go、task_dispatch.go 等共享使用。
func dimNameToID(name string) int {
	lower := strings.ToLower(name)
	switch lower {
	case "nether", "minecraft:nether", "the_nether", "minecraft:the_nether":
		return 1
	case "end", "the_end", "minecraft:the_end":
		return 2
	}
	// DM 维度：dm3 → 3, dm4 → 4
	if strings.HasPrefix(lower, "dm") {
		var id int
		if n, _ := fmt.Sscanf(lower, "dm%d", &id); n == 1 && id >= 3 {
			return id
		}
	}
	return 0
}

// SetDimension 设置命令执行的维度。非 overworld 时后续命令自动加 execute in 前缀。
func (bm *BotManager) SetDimension(name string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.commandDimension = normalizeDimension(name)
}

// WrapCommandInDimension 在命令前加 execute in <dim> run 前缀（非 overworld 时）。
func (bm *BotManager) WrapCommandInDimension(cmd string) string {
	bm.mu.Lock()
	dim := bm.commandDimension
	bm.mu.Unlock()
	return wrapCommandInDimension(cmd, dim)
}

// WrapCommandInDimensionWithDim 与 WrapCommandInDimension 相同，但不获取锁。
// 供 SendPlayerCommand/SendWSCommand 在已释放锁后调用，避免自死锁。
func (bm *BotManager) WrapCommandInDimensionWithDim(cmd string, dim string) string {
	return wrapCommandInDimension(cmd, dim)
}

func wrapCommandInDimension(cmd string, dim string) string {
	if dim == "" || strings.EqualFold(dim, "overworld") || strings.EqualFold(dim, "minecraft:overworld") {
		return cmd
	}
	trimmed := strings.TrimSpace(cmd)
	if strings.HasPrefix(trimmed, "/") {
		trimmed = trimmed[1:]
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "execute in ") {
		return cmd
	}
	return fmt.Sprintf("execute in %s run %s", dim, trimmed)
}

// SendAICommand 通过网易 AI 命令通道（PyRpc ExecuteCommandEvent）发送命令。
// 自动加 "execute run" 前缀以保证命令执行上下文（匹配 NexusEgo 的 SendSettingsCommand 行为）。
func (bm *BotManager) SendAICommand(cmd string) error {
	bm.mu.Lock()
	conn := bm.conn
	bm.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("未连接")
	}
	cmd = bm.WrapCommandInDimension(cmd)
	if len(cmd) > 0 && cmd[0] == '/' {
		cmd = cmd[1:]
	}
	event := cts_mc_a.ExecuteCommandEvent{
		CommandLine:      "execute run " + cmd,
		CommandRequestID: uuid.New(),
	}
	module := cts_mc.AICommand{Module: &mei.DefaultModule{Event: &event}}
	park := cts.Minecraft{Default: mei.Default{Module: &module}}
	return conn.WritePacket(&packet.PyRpc{
		Value: py_rpc.Marshal(&py_rpc.ModEvent{
			Package: &park,
			Type:    py_rpc.ModEventClientToServer,
		}),
		OperationType: packet.PyRpcOperationTypeSend,
	})
}

// SendWSCommand 通过 WebSocket 自动化通道（CommandRequest + AutomationPlayer origin）发送命令。
// 此通道可获取命令响应，适用于需要确认的操作（如 tickingarea、验证命令）。
func (bm *BotManager) SendWSCommand(cmd string) error {
	bm.mu.Lock()
	if !bm.connected || bm.conn == nil {
		bm.mu.Unlock()
		return fmt.Errorf("未连接")
	}
	conn := bm.conn
	dim := bm.commandDimension
	bm.mu.Unlock()
	cmd = bm.WrapCommandInDimensionWithDim(cmd, dim)
	if len(cmd) > 0 && cmd[0] == '/' {
		cmd = cmd[1:]
	}

	return conn.WritePacket(&packet.CommandRequest{
		CommandLine: cmd,
		CommandOrigin: protocol.CommandOrigin{
			Origin:    protocol.CommandOriginAutomationPlayer,
			UUID:      uuid.New(),
			RequestID: uuid.NewString(),
		},
		Version: 44,
	})
}

// Default AI fire-and-forget uses the original transport timer coalescing.
// Player/console packets retain bounded coalescing; neither adds a per-command sleep.
func (bm *BotManager) sendCommandBuffered(channel CommandChannel, cmd string) error {
	bm.mu.Lock()
	conn, connected, dim, name := bm.conn, bm.connected, bm.commandDimension, bm.display
	bm.mu.Unlock()
	if !connected || conn == nil {
		return fmt.Errorf("未连接")
	}
	cmd = strings.TrimPrefix(bm.WrapCommandInDimensionWithDim(cmd, dim), "/")
	if channel == ChannelAI {
		// SettingsCommand has no CommandOutput flood. Preserve the bot's @s
		// and relative-coordinate context, rather than using server origin.
		return conn.WritePacket(yyFastSettingsPacket(cmd, name))
	}
	origin := uint32(protocol.CommandOriginAutomationPlayer)
	if channel == ChannelPlayer {
		origin = protocol.CommandOriginPlayer
	}
	return conn.WritePacketBounded(&packet.CommandRequest{CommandLine: cmd,
		CommandOrigin: protocol.CommandOrigin{Origin: origin, UUID: uuid.New(), RequestID: uuid.NewString()}, Version: 44})
}

func yyFastSettingsPacket(cmd, name string) *packet.SettingsCommand {
	if name != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(name)
		cmd = fmt.Sprintf(`execute as @a[name="%s"] at @s run %s`, escaped, cmd)
	}
	return &packet.SettingsCommand{CommandLine: cmd, SuppressOutput: true}
}

// SendPlayerCommand 以玩家身份发送命令（CommandOriginPlayer），如同真实玩家在聊天栏输入。
func (bm *BotManager) SendPlayerCommand(cmd string) error {
	bm.mu.Lock()
	if !bm.connected || bm.conn == nil {
		bm.mu.Unlock()
		return fmt.Errorf("未连接")
	}
	conn := bm.conn
	dim := bm.commandDimension
	bm.mu.Unlock()
	cmd = bm.WrapCommandInDimensionWithDim(cmd, dim)
	if len(cmd) > 0 && cmd[0] == '/' {
		cmd = cmd[1:]
	}

	return conn.WritePacket(&packet.CommandRequest{
		CommandLine: cmd,
		CommandOrigin: protocol.CommandOrigin{
			Origin:    protocol.CommandOriginPlayer,
			UUID:      uuid.New(),
			RequestID: uuid.NewString(),
		},
		Version: 44,
	})
}

// WritePacket writes a raw packet to the Minecraft connection.
func (bm *BotManager) WritePacket(pk packet.Packet) error {
	bm.mu.Lock()
	if !bm.connected || bm.conn == nil {
		bm.mu.Unlock()
		return fmt.Errorf("未连接")
	}
	conn := bm.conn
	bm.mu.Unlock()

	conn.WritePacket(pk)
	return nil
}

func (bm *BotManager) quotedName() string {
	return `"` + bm.display + `"`
}

func hasNonDigit(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return true
		}
	}
	return false
}

// sendModSync 发送模组同步和初始化事件（山头服需要）
func (bm *BotManager) sendModSync(conn *minecraft.Conn) {
	time.Sleep(2 * time.Second) // 等登录包风暴平息

	runtimeid := fmt.Sprintf("%d", conn.GameData().EntityUniqueID)

	// 不发送 SyncUsingMod：验证服务器 outfit_info 不可靠，
	// 发空列表反而告诉服务器"我没模组"导致被踢。
	// 资源包已在登录阶段通过 ResourcePackClientResponse(0x03) 声明"全都有"。
	// 参考 Crow-Brother-Ads 的做法，只发后续初始化事件即可。

	conn.WritePacket(&packet.PyRpc{
		Value:         py_rpc.Marshal(&py_rpc.ClientLoadAddonsFinishedFromGac{}),
		OperationType: packet.PyRpcOperationTypeSend,
	})

	{
		event := cts_mc_p.GetLoadedInstances{PlayerRuntimeID: runtimeid}
		module := cts_mc.Preset{Module: &mei.DefaultModule{Event: &event}}
		park := cts.Minecraft{Default: mei.Default{Module: &module}}
		conn.WritePacket(&packet.PyRpc{
			Value: py_rpc.Marshal(&py_rpc.ModEvent{
				Package: &park,
				Type:    py_rpc.ModEventClientToServer,
			}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	}

	conn.WritePacket(&packet.PyRpc{
		Value:         py_rpc.Marshal(&py_rpc.ArenaGamePlayerFinishLoad{}),
		OperationType: packet.PyRpcOperationTypeSend,
	})

	{
		event := cts_mc_v.PlayerUiInit{RuntimeID: runtimeid}
		module := cts_mc.VIPEventSystem{Module: &mei.DefaultModule{Event: &event}}
		park := cts.Minecraft{Default: mei.Default{Module: &module}}
		conn.WritePacket(&packet.PyRpc{
			Value: py_rpc.Marshal(&py_rpc.ModEvent{
				Package: &park,
				Type:    py_rpc.ModEventClientToServer,
			}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	}
}

func formatCommandMessage(msg string, params []string) string {
	if len(params) == 0 {
		return msg
	}
	// Try Java-style %1$s, %2$s, etc.
	substituted := false
	for i, p := range params {
		ph := fmt.Sprintf("%%%d$s", i+1)
		if strings.Contains(msg, ph) {
			msg = strings.ReplaceAll(msg, ph, p)
			substituted = true
		}
	}
	if substituted {
		return msg
	}
	// Replace C-style %s with actual parameters
	if strings.Contains(msg, "%s") || strings.Contains(msg, "%d") {
		for _, p := range params {
			msg = strings.Replace(msg, "%s", p, 1)
		}
		return msg
	}
	// Localization key — append params
	return msg + ": " + strings.Join(params, ", ")
}
