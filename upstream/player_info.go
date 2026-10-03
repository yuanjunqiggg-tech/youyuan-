package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/core/py_rpc"
	"github.com/go-gl/mathgl/mgl32"
)

// ========== 类型定义 ==========

// PlayerInfo 玩家完整信息
type PlayerInfo struct {
	Name       string        `json:"name"`
	UUID       string        `json:"uuid"`
	XUID       string        `json:"xuid"`
	EntityID   int64         `json:"entity_id"`
	RuntimeID  uint64        `json:"runtime_id"`
	Position   mgl32.Vec3    `json:"position"`
	Dimension  int           `json:"dimension"`
	IsOP       bool          `json:"is_op"`
	CmdPerm    uint32        `json:"cmd_perm"`
	PlayerPerm uint32        `json:"player_perm"`
	HeldItem   *HeldItemInfo `json:"held_item,omitempty"`
	Online     bool          `json:"online"`
	Pitch      float32       `json:"pitch"`
	Yaw        float32       `json:"yaw"`
	HeadYaw    float32       `json:"head_yaw"`
	// 皮肤数据（从 PlayerList 包缓存）
	SkinData   []byte `json:"-"`
	SkinWidth  uint32 `json:"-"`
	SkinHeight uint32 `json:"-"`
}

// HeldItemInfo 手持物品信息
type HeldItemInfo struct {
	NetworkID int32 `json:"network_id"`
	Count     int   `json:"count"`
	Damage    int   `json:"damage"`
}

// pendingSkin 暂存尚未关联到玩家名字的皮肤数据（按 UUID 索引）。
type pendingSkin struct {
	data   []byte
	width  uint32
	height uint32
}

// PlayerInfoManager 玩家信息管理器
// 监听数据包，维护所有在线玩家的状态。
// 等效于 td_prism 的 PlayerInfoMaintainer。
type PlayerInfoManager struct {
	mu      sync.RWMutex
	players map[string]*PlayerInfo // name → PlayerInfo
	byEID   map[int64]*PlayerInfo  // EntityUniqueID → PlayerInfo
	byRTID  map[uint64]*PlayerInfo // RuntimeID → PlayerInfo
	byUUID  map[string]*PlayerInfo // player UUID → PlayerInfo
	pending map[string]pendingSkin // UUID → 皮肤缓存（玩家名未知时暂存）
	botName string
}

// NewPlayerInfoManager 创建玩家信息管理器
func NewPlayerInfoManager() *PlayerInfoManager {
	return &PlayerInfoManager{
		players: make(map[string]*PlayerInfo),
		byEID:   make(map[int64]*PlayerInfo),
		byRTID:  make(map[uint64]*PlayerInfo),
		byUUID:  make(map[string]*PlayerInfo),
		pending: make(map[string]pendingSkin),
	}
}

// setUUID 更新玩家 UUID 索引，并应用暂存的皮肤数据。
// 必须在持有 pm.mu 时调用。
func (pm *PlayerInfoManager) setUUID(pi *PlayerInfo, uuidStr string) {
	if pi.UUID != "" && pi.UUID != uuidStr {
		delete(pm.byUUID, pi.UUID)
	}
	pi.UUID = uuidStr
	if uuidStr == "" {
		return
	}
	pm.byUUID[uuidStr] = pi
	if skin, ok := pm.pending[uuidStr]; ok {
		pi.SkinData = skin.data
		pi.SkinWidth = skin.width
		pi.SkinHeight = skin.height
		delete(pm.pending, uuidStr)
	}
}

// ========== 包处理 ==========

// HandlePacket 处理玩家相关的包，更新玩家信息。
// 在 BotManager.consumePackets() 中调用。
func (pm *PlayerInfoManager) HandlePacket(pk packet.Packet, logCh chan string) {
	switch p := pk.(type) {
	case *packet.PlayerList:
		pm.handlePlayerList(p, logCh)
	case *packet.PlayerSkin:
		pm.handlePlayerSkin(p, logCh)
	case *packet.SyncSkin:
		pm.handleSyncSkin(p, logCh)
	case *packet.ConfirmSkin:
		pm.handleConfirmSkin(p, logCh)
	case *packet.PyRpc:
		pm.handlePyRpcSkin(p, logCh)
	case *packet.AddPlayer:
		pm.handleAddPlayer(p, logCh)
	case *packet.UpdateAbilities:
		pm.handleUpdateAbilities(p)
	case *packet.MobEquipment:
		pm.handleMobEquipment(p)
	case *packet.MovePlayer:
		pm.handleMovePlayer(p)
	case *packet.PlayerLocation:
		pm.handlePlayerLocation(p)
	case *packet.Respawn:
		pm.handleRespawn(p)
	}
}

func (pm *PlayerInfoManager) handlePlayerList(p *packet.PlayerList, logCh chan string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if p.ActionType == packet.PlayerListActionAdd {
		for _, e := range p.Entries {
			entry := e
			name := entry.Username
			eid := entry.EntityUniqueID

			if existing, ok := pm.byEID[eid]; ok {
				// 更新现有玩家
				existing.Name = name
				existing.XUID = entry.XUID
				existing.Online = true
				pm.setUUID(existing, entry.UUID.String())
				updateSkinInfo(existing, &entry)
			} else if existing, ok := pm.players[name]; ok {
				// 通过名称找到现有玩家
				existing.EntityID = eid
				existing.XUID = entry.XUID
				existing.Online = true
				pm.byEID[eid] = existing
				pm.setUUID(existing, entry.UUID.String())
				updateSkinInfo(existing, &entry)
			} else {
				// 新玩家
				pi := &PlayerInfo{
					Name:     name,
					XUID:     entry.XUID,
					EntityID: eid,
					Online:   true,
				}
				pm.setUUID(pi, entry.UUID.String())
				updateSkinInfo(pi, &entry)
				pm.players[name] = pi
				pm.byEID[eid] = pi
			}
			// 皮肤诊断：PlayerList ADD 携带皮肤数据时打日志
			if len(entry.Skin.SkinData) > 0 {
				if logCh != nil {
					logCh <- fmt.Sprintf("[皮肤] PlayerList: %s %dx%d", name, entry.Skin.SkinImageWidth, entry.Skin.SkinImageHeight)
				}
				dumpSkinToDebugLog("PlayerListADD name="+name+" uuid="+entry.UUID.String(), &entry.Skin)
			}
		}
	} else {
		// 玩家离开
		for _, e := range p.Entries {
			entry := e
			name := entry.Username
			if pi, ok := pm.byEID[entry.EntityUniqueID]; ok {
				pi.Online = false
				delete(pm.byEID, entry.EntityUniqueID)
				if pi.RuntimeID != 0 {
					delete(pm.byRTID, pi.RuntimeID)
				}
				if logCh != nil {
					logCh <- fmt.Sprintf("[玩家离开] %s", name)
				}
			} else if pi, ok := pm.players[name]; ok {
				pi.Online = false
				if pi.RuntimeID != 0 {
					delete(pm.byRTID, pi.RuntimeID)
				}
				if pi.EntityID != 0 {
					delete(pm.byEID, pi.EntityID)
				}
			}
		}
	}
}

// updateSkinInfo 从 PlayerListEntry 复制皮肤数据到 PlayerInfo。
func updateSkinInfo(pi *PlayerInfo, entry *protocol.PlayerListEntry) {
	pi.SkinData = entry.Skin.SkinData
	pi.SkinWidth = entry.Skin.SkinImageWidth
	pi.SkinHeight = entry.Skin.SkinImageHeight
}

// handlePlayerSkin 处理皮肤广播包（PlayerSkin）。
// 网易服务器会把玩家的皮肤更新/进服皮肤转发给所有客户端，
// 这是获取已在线玩家皮肤的主要来源。
func (pm *PlayerInfoManager) handlePlayerSkin(p *packet.PlayerSkin, logCh chan string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	uuidStr := p.UUID.String()
	pi := pm.byUUID[uuidStr]
	if pi == nil {
		// 尚未建立 UUID → 名字映射（玩家在 bot 进服前已在线），暂存皮肤
		if len(p.Skin.SkinData) > 0 {
			pm.pending[uuidStr] = pendingSkin{
				data:   append([]byte(nil), p.Skin.SkinData...),
				width:  p.Skin.SkinImageWidth,
				height: p.Skin.SkinImageHeight,
			}
		}
		if logCh != nil {
			logCh <- fmt.Sprintf("[皮肤] PlayerSkin(未匹配名字): %s %dx%d", uuidStr[:8], p.Skin.SkinImageWidth, p.Skin.SkinImageHeight)
		}
		dumpSkinToDebugLog("PlayerSkin(未匹配) uuid="+uuidStr, &p.Skin)
		return
	}
	pi.SkinData = p.Skin.SkinData
	pi.SkinWidth = p.Skin.SkinImageWidth
	pi.SkinHeight = p.Skin.SkinImageHeight
	if logCh != nil {
		logCh <- fmt.Sprintf("[皮肤] PlayerSkin: %s %dx%d", pi.Name, p.Skin.SkinImageWidth, p.Skin.SkinImageHeight)
	}
	dumpSkinToDebugLog("PlayerSkin name="+pi.Name+" uuid="+uuidStr, &p.Skin)
}

// handlePyRpcSkin 记录服务器下发的皮肤相关 PyRpc（如 SyncVipSkinUuid），
// 用于确认网易皮肤注册/同步机制的真实载荷格式。
func (pm *PlayerInfoManager) handlePyRpcSkin(p *packet.PyRpc, logCh chan string) {
	if p.Value == nil {
		return
	}
	content, err := py_rpc.Unmarshal(p.Value)
	if err != nil {
		return
	}
	switch c := content.(type) {
	case *py_rpc.SyncVipSkinUUID:
		b, _ := json.Marshal(c.MakeGo())
		debugLog("[皮肤] 收到 SyncVipSkinUuid: %s", string(b))
		if logCh != nil {
			logCh <- "[皮肤] 收到 SyncVipSkinUuid rpc"
		}
	}
}

// handleSyncSkin 处理网易 SyncSkin 包（皮肤同步，网易专属协议）。
// 结构字段为推测值，仅记录收到的内容用于诊断。
func (pm *PlayerInfoManager) handleSyncSkin(p *packet.SyncSkin, logCh chan string) {
	var uuids []string
	for _, pd := range p.PlayerData {
		uuids = append(uuids, pd.UUID.String())
	}
	debugLog("[皮肤] SyncSkin 收到: playerData=%d uuids=%v playerUIDs=%v unknown=%v skinItemIDs=%v",
		len(p.PlayerData), uuids, p.PlayerUIDs, p.Unknown, p.SkinItemIDs)
	dumpSkinToDebugLog("SyncSkin", &p.Skin)
	if logCh != nil {
		logCh <- fmt.Sprintf("[皮肤] SyncSkin: %d 条记录", len(p.PlayerData))
	}
}

// handleConfirmSkin 处理网易 ConfirmSkin 包（皮肤确认，网易专属协议）。
// 结构字段为推测值，仅记录收到的内容用于诊断。
func (pm *PlayerInfoManager) handleConfirmSkin(p *packet.ConfirmSkin, logCh chan string) {
	var uuids []string
	for _, pd := range p.PlayerData {
		uuids = append(uuids, pd.UUID.String())
	}
	debugLog("[皮肤] ConfirmSkin 收到: playerData=%d uuids=%v uids=%v unknown3=%v",
		len(p.PlayerData), uuids, p.Uids, p.Unknown3)
	if logCh != nil {
		logCh <- fmt.Sprintf("[皮肤] ConfirmSkin: %d 条记录", len(p.PlayerData))
	}
}

// dumpSkinToDebugLog 把皮肤的完整元数据写入设备日志文件，
// 用于定位"其他玩家收得到广播但不渲染"的问题。
func dumpSkinToDebugLog(prefix string, s *protocol.Skin) {
	if s == nil {
		debugLog("[皮肤] %s: <nil>", prefix)
		return
	}
	var patchStr string
	if len(s.SkinResourcePatch) > 0 {
		patchStr = string(s.SkinResourcePatch)
	}
	geoIDs := ""
	geoParseErr := ""
	if len(s.SkinGeometry) > 0 {
		var g map[string]json.RawMessage
		if err := json.Unmarshal(s.SkinGeometry, &g); err != nil {
			geoParseErr = err.Error()
		} else {
			keys := make([]string, 0, len(g))
			for k := range g {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			geoIDs = strings.Join(keys, ",")
		}
		// 详细几何结构：每个几何的 identifier / 贴图尺寸 / 骨骼数
		if rawGeoms, ok := g["minecraft:geometry"]; ok {
			var geoms []struct {
				Description struct {
					Identifier    string `json:"identifier"`
					TextureWidth  int    `json:"texture_width"`
					TextureHeight int    `json:"texture_height"`
				} `json:"description"`
				Bones []json.RawMessage `json:"bones"`
			}
			if err := json.Unmarshal(rawGeoms, &geoms); err == nil {
				for _, gm := range geoms {
					debugLog("[皮肤]   几何: %s texture=%dx%d bones=%d", gm.Description.Identifier, gm.Description.TextureWidth, gm.Description.TextureHeight, len(gm.Bones))
				}
			}
		}
	}
	debugLog("[皮肤] %s\n  SkinID=%q PlayFabID=%q FullID=%q CapeID=%q\n  w=%d h=%d dataLen=%d capeW=%d capeH=%d capeLen=%d\n  patch=%s\n  geometryTopKeys=%s geometryErr=%q\n  armSize=%q skinColour=%q premium=%v persona=%v capeOnClassic=%v override=%v trusted=%v primaryUser=%v\n  geoVersion=%q animationDataLen=%d animations=%d pieces=%d tints=%d",
		prefix,
		s.SkinID, s.PlayFabID, s.FullID, s.CapeID,
		s.SkinImageWidth, s.SkinImageHeight, len(s.SkinData), s.CapeImageWidth, s.CapeImageHeight, len(s.CapeData),
		patchStr,
		geoIDs, geoParseErr,
		s.ArmSize, s.SkinColour, s.PremiumSkin, s.PersonaSkin, s.PersonaCapeOnClassicSkin, s.OverrideAppearance, s.Trusted, s.PrimaryUser,
		string(s.GeometryDataEngineVersion), len(s.AnimationData), len(s.Animations), len(s.PersonaPieces), len(s.PieceTintColours),
	)
}

// GetPlayerSkin 获取指定玩家的皮肤缓存数据。
// 返回 skinData, width, height，以及是否可用。
func (pm *PlayerInfoManager) GetPlayerSkin(name string) ([]byte, uint32, uint32, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pi := pm.players[name]
	if pi == nil || len(pi.SkinData) == 0 {
		return nil, 0, 0, false
	}
	return pi.SkinData, pi.SkinWidth, pi.SkinHeight, true
}

func (pm *PlayerInfoManager) handleAddPlayer(p *packet.AddPlayer, logCh chan string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	name := p.Username
	rtid := p.EntityRuntimeID

	if pi, ok := pm.players[name]; ok {
		pi.RuntimeID = rtid
		pi.Position = p.Position
		pi.Pitch = p.Pitch
		pi.Yaw = p.Yaw
		pi.HeadYaw = p.HeadYaw
		pm.byRTID[rtid] = pi
		if p.HeldItem.Stack.ItemType.NetworkID != 0 {
			pi.HeldItem = &HeldItemInfo{
				Count:     int(p.HeldItem.Stack.Count),
				Damage:    int(p.HeldItem.Stack.MetadataValue),
				NetworkID: p.HeldItem.StackNetworkID,
			}
		}
		// 从 AddPlayer 的 AbilityData 获取权限
		ab := p.AbilityData
		pi.CmdPerm = uint32(ab.CommandPermissions)
		pi.PlayerPerm = uint32(ab.PlayerPermissions)
		pi.IsOP = ab.CommandPermissions >= packet.CommandPermissionLevelGameDirectors
	} else {
		pi := &PlayerInfo{
			Name:       name,
			RuntimeID:  rtid,
			Position:   p.Position,
			Pitch:      p.Pitch,
			Yaw:        p.Yaw,
			HeadYaw:    p.HeadYaw,
			Online:     true,
			CmdPerm:    uint32(p.AbilityData.CommandPermissions),
			PlayerPerm: uint32(p.AbilityData.PlayerPermissions),
			IsOP:       p.AbilityData.CommandPermissions >= packet.CommandPermissionLevelGameDirectors,
		}
		pm.players[name] = pi
		pm.byRTID[rtid] = pi
	}
}

func (pm *PlayerInfoManager) handleUpdateAbilities(p *packet.UpdateAbilities) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	eid := p.AbilityData.EntityUniqueID
	cmd := p.AbilityData.CommandPermissions

	if pi, ok := pm.byEID[eid]; ok {
		pi.CmdPerm = uint32(cmd)
		pi.PlayerPerm = uint32(p.AbilityData.PlayerPermissions)
		pi.IsOP = cmd >= packet.CommandPermissionLevelGameDirectors
	} else {
		// 通过 runtime ID 查找
		for _, pi := range pm.players {
			if pi.EntityID == eid {
				pi.CmdPerm = uint32(cmd)
				pi.PlayerPerm = uint32(p.AbilityData.PlayerPermissions)
				pi.IsOP = cmd >= packet.CommandPermissionLevelGameDirectors
				break
			}
		}
	}
}

func (pm *PlayerInfoManager) handleMobEquipment(p *packet.MobEquipment) {
	pm.mu.RLock()
	pi, ok := pm.byRTID[p.EntityRuntimeID]
	pm.mu.RUnlock()
	if !ok {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	// 再次检查（释放锁后可能变化）
	if pi, ok = pm.byRTID[p.EntityRuntimeID]; !ok {
		return
	}
	if p.NewItem.Stack.ItemType.NetworkID != 0 {
		pi.HeldItem = &HeldItemInfo{

			Count:     int(p.NewItem.Stack.Count),
			Damage:    int(p.NewItem.Stack.MetadataValue),
			NetworkID: p.NewItem.StackNetworkID,
		}
	} else {
		pi.HeldItem = nil
	}
}

func (pm *PlayerInfoManager) handleMovePlayer(p *packet.MovePlayer) {
	pm.mu.RLock()
	pi, ok := pm.byRTID[p.EntityRuntimeID]
	pm.mu.RUnlock()
	if !ok {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pi, ok = pm.byRTID[p.EntityRuntimeID]; ok {
		pi.Position = p.Position
		pi.Pitch = p.Pitch
		pi.Yaw = p.Yaw
		pi.HeadYaw = p.HeadYaw
	}
}

func (pm *PlayerInfoManager) handlePlayerLocation(p *packet.PlayerLocation) {
	pm.mu.RLock()
	pi, ok := pm.byEID[p.EntityUniqueID]
	pm.mu.RUnlock()
	if !ok {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pi, ok = pm.byEID[p.EntityUniqueID]; ok {
		pi.Position = p.Position
	}
}

func (pm *PlayerInfoManager) handleRespawn(p *packet.Respawn) {
	pm.mu.RLock()
	pi, ok := pm.byRTID[p.EntityRuntimeID]
	pm.mu.RUnlock()
	if !ok {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pi, ok = pm.byRTID[p.EntityRuntimeID]; ok {
		pi.Position = p.Position
	}
}

// ========== 命令查询（网易租赁服不发送 PlayerList 包时的替代方案） ==========

// RefreshPlayerList 通过 /list + querytarget 命令刷新玩家列表
// 网易租赁服不发 PlayerList/AddPlayer 包，用命令替代
func (pm *PlayerInfoManager) RefreshPlayerList(bm *BotManager) {
	// 1. 用 /list 获取玩家名称列表
	// 输出格式: OutputMessages[1].Parameters[0] = "name1, name2, ..."
	resp, err := bm.SendWSCommandWithResp("list")
	if err != nil {
		return
	}

	playerNames := parseListOutput(resp)
	if len(playerNames) == 0 {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 先标记所有玩家离线
	for _, pi := range pm.players {
		pi.Online = false
	}

	// 2. 对每个玩家单独 querytarget 获取位置
	for _, name := range playerNames {
		infos, err := bm.DoQuerytarget(`"` + name + `"`)
		if err != nil {
			continue
		}
		if len(infos) == 0 {
			continue
		}
		info := infos[0]
		eid := info.PlayerUniqueID
		pos := mgl32.Vec3{info.Position.X, info.Position.Y, info.Position.Z}
		dim := int(info.Dimension)

		if pi, ok := pm.players[name]; ok {
			// 更新已有玩家
			pi.Online = true
			pi.EntityID = eid
			pi.Position = pos
			pi.Dimension = dim
			pi.Yaw = info.YRot
			pm.byEID[eid] = pi
			pm.setUUID(pi, info.PlayerUUID.String())
		} else {
			// 新玩家
			pi := &PlayerInfo{
				Name:      name,
				EntityID:  eid,
				Position:  pos,
				Dimension: dim,
				Yaw:       info.YRot,
				Online:    true,
			}
			pm.players[name] = pi
			pm.byEID[eid] = pi
			pm.setUUID(pi, info.PlayerUUID.String())
		}

	}

	// 同步更新全局 players 表（先清空再添加，避免离线玩家残留）
	opMu.Lock()
	for k := range players {
		delete(players, k)
	}
	// 添加机器人自身
	if bm := getBotManager(); bm != nil {
		players[bm.GetBotName()] = bm.eid
	}
	// 添加在线玩家
	for _, pi := range pm.players {
		if pi.Online {
			players[pi.Name] = pi.EntityID
		}
	}
	opMu.Unlock()
}

// parseListOutput 解析 /list 命令输出，返回玩家名列表
func parseListOutput(resp *packet.CommandOutput) []string {
	if resp == nil || len(resp.OutputMessages) == 0 {
		return nil
	}

	// 查找包含玩家名的消息
	// 格式1: commands.players.list.names → Parameters[0] = "name1, name2, ..."
	for _, msg := range resp.OutputMessages {
		if msg.Message == "commands.players.list.names" && len(msg.Parameters) > 0 {
			names := strings.Split(msg.Parameters[0], ", ")
			var result []string
			for _, n := range names {
				n = strings.TrimSpace(n)
				if n != "" {
					result = append(result, n)
				}
			}
			return result
		}
	}

	// 格式2: 从 DataSet 解析
	if resp.DataSet != "" {
		var ds struct {
			Players string `json:"players"`
		}
		if err := json.Unmarshal([]byte(resp.DataSet), &ds); err == nil && ds.Players != "" {
			names := strings.Split(ds.Players, ", ")
			var result []string
			for _, n := range names {
				n = strings.TrimSpace(n)
				if n != "" {
					result = append(result, n)
				}
			}
			return result
		}
	}

	return nil
}

// ========== 查询方法 ==========

// GetPlayer 通过名称获取玩家信息。
// 等效于 td_prism 的 getPlayerByName()。
func (pm *PlayerInfoManager) GetPlayer(name string) *PlayerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.players[name]
}

// GetPlayerByEID 通过 EntityUniqueID 获取玩家信息。
func (pm *PlayerInfoManager) GetPlayerByEID(eid int64) *PlayerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.byEID[eid]
}

// GetPlayerByRuntimeID 通过 RuntimeID 获取玩家信息。
func (pm *PlayerInfoManager) GetPlayerByRuntimeID(rtid uint64) *PlayerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.byRTID[rtid]
}

// GetAllPlayers 获取所有在线玩家。
// 等效于 td_prism 的 getAllPlayers()。
func (pm *PlayerInfoManager) GetAllPlayers() []*PlayerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var result []*PlayerInfo
	for _, pi := range pm.players {
		if pi.Online {
			result = append(result, pi)
		}
	}
	return result
}

// OnlineSet 返回当前在线玩家名集合（用于 join/leave 差量对比）。
func (pm *PlayerInfoManager) OnlineSet() map[string]bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make(map[string]bool, len(pm.players))
	for _, pi := range pm.players {
		if pi.Online {
			out[pi.Name] = true
		}
	}
	return out
}

// GetOnlineCount 获取在线玩家数量。
func (pm *PlayerInfoManager) GetOnlineCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	count := 0
	for _, pi := range pm.players {
		if pi.Online {
			count++
		}
	}
	return count
}

// IsPlayerOnline 检查玩家是否在线。
func (pm *PlayerInfoManager) IsPlayerOnline(name string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if pi, ok := pm.players[name]; ok {
		return pi.Online
	}
	return false
}

// ========== BotManager 集成 ==========

// 全局玩家信息管理器
var playerInfoMgr = NewPlayerInfoManager()

// EnsurePlayerInfo 确保 BotManager 的 consumePackets 中调用玩家信息管理器。
// 在 BotManager.registerPacketHandlers 注册时需要包含更多的包类型：
// IDMobEquipment, IDPlayerLocation, IDMovePlayer
func (bm *BotManager) EnsurePlayerInfo() {
	// playerInfoMgr 已在 consumePackets 中通过 HandlePlayerPacket 调用
}

// HandlePlayerPacket 处理玩家信息包，由 consumePackets 调用。
func (bm *BotManager) HandlePlayerPacket(pk packet.Packet) {
	playerInfoMgr.HandlePacket(pk, bm.logCh)
}

// GetPlayerInfo 获取玩家信息管理器实例。
func (bm *BotManager) GetPlayerInfo() *PlayerInfoManager {
	return playerInfoMgr
}

// GetPlayer 获取指定名称的玩家信息。
func (bm *BotManager) GetPlayer(name string) *PlayerInfo {
	return playerInfoMgr.GetPlayer(name)
}

// GetAllPlayers 获取所有在线玩家。
func (bm *BotManager) GetAllPlayers() []*PlayerInfo {
	return playerInfoMgr.GetAllPlayers()
}

// IsPlayerOnline 检查玩家是否在线。
func (bm *BotManager) IsPlayerOnline(name string) bool {
	return playerInfoMgr.IsPlayerOnline(name)
}

// GetBotName 获取机器人名称。
// 等效于 td_prism 的 get_robotname()。
func (bm *BotManager) GetBotName() string {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.display
}

// GetBotInfo 获取机器人自身的玩家信息。
// 等效于 td_prism 的 getBotInfo()。
func (bm *BotManager) GetBotInfo() *PlayerInfo {
	bm.mu.Lock()
	name := bm.display
	bm.mu.Unlock()
	return playerInfoMgr.GetPlayer(name)
}

// IsPlayerOP 检查指定玩家是否为 OP。
func (bm *BotManager) IsPlayerOP(name string) bool {
	pi := playerInfoMgr.GetPlayer(name)
	if pi == nil {
		return false
	}
	return pi.IsOP
}
