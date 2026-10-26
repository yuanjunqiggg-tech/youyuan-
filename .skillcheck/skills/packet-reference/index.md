# 游园 0.10.23：真实观测 API、事件与容量配置

本轮修改宿主，不要求插件自己发同步查询来拼血量、装备或视角。协议数据只覆盖服务器确实给当前机器人发送的范围。以下“已实现”指源代码及相应测试；线上观测结果另列于验收文件，不能把接口存在写成已完成全服实测。

## 容量不是包种类，也不是命令频率

原来的128对应Lua普通回调队列和未读取异步命令ticket的默认容量。Python/C++回调队列原为256。协议目录从内核动态生成，没有“只能解析128种包”的限制。

在设置 → 实时感知与抓包 → 观测工作台，可以用数值输入框和保存按钮调整。高级调用：

```lua
diagnostics.configure({
  event_queue=2048, priority_queue=2048, runtime_queue=2048,
  async_tickets=4096, event_history=4096, actors=4096,
  stale_ms=15000
})
```

容量字段各为1～65536，过期阈值单位毫秒。保存到宿主数据目录`observation-limits.json`。队列容量在插件重载后生效；其他字段影响后续操作。默认值保持兼容：128 / 128 / 256 / 128 / 512 / 2048，过期15000ms。大容量只增大留存空间，不增加服务器广播范围，也不人为限制Fast命令CPS。异步结果读完应使用`game.commandResult(ticket,0,true)`消费，避免长期积压。

原始包记录单独配置：

```lua
events.captureStart({types={"UpdateAttributes","ActorEvent","MobEquipment",
  "MobArmourEquipment","MovePlayer","MoveActorDelta"},
  capacity=8192,max_bytes=8192,total_bytes=32*1024*1024,
  filter={every_ms=0}})
```

`capacity`和单次读取`limit`最高65536。默认留存序列化预算16MiB，最高256MiB；实际Go对象内存还包含运行时开销，因此预算不称为物理内存上限。环形缓冲按时间保留最近记录，覆盖旧记录时统计`overwritten`；不会每收到一包搬动全部记录。握手认证包继续排除，`*`默认不包含大区块包；可以明确选择正常大包类型。包类型筛选不再只允许32个名称。

## 接口与真实来源

| 功能 | 新接口 | 数据来源 / 边界 |
|---|---|---|
| 实体状态 | `entities.get(id)`、`diagnostics.entity(id)` | 当前会话实体身份、坐标、属性、装备、视角；移除及旧维度数据标记过期 |
| 血量 | `entity.health(id)`、`entities.health(id)` | `UpdateAttributes`的`minecraft:health`及`SetHealth`；current/max/previous/damage/known/max_known/stale/source |
| 属性及护甲点 | `entities.attributes(id)` | 原样保存同步属性；护甲点只取真实`minecraft:armor`，不凭穿戴算出一个假的护甲值 |
| 装备 | `player.equipment(name)` | `MobEquipment`、`MobArmourEquipment`、机器人自身Inventory包；ItemRegistry及连接GameData.Items翻译网络ID |
| 背包 | `player.inventory("self",windowId)` | 机器人实际收到的`InventoryContent`和`InventorySlot`；他人的完整私有背包未知 |
| 视角 | `player.rotation(name)` | `AddPlayer/AddActor`、`MovePlayer`、`MoveActorAbsolute/Delta`真实角度；仅旋转也更新，各角度单独known/time/source |
| 附近实体 | `entities.nearby({x,y,z},radius,dimension)` | 只筛选已同步、存在、同维度且位置新鲜的实体；无全世界扫描 |
| 受伤 | `events.onEntityHurt`、`entity.onDamage`、旧`events.onHurt` | 优先实际掉血，ActorEvent提示等50ms关联属性包；无属性时仍提供damage_known=false的真实受伤信号 |
| 击杀 | `events.onPlayerKill` | 服务器死亡翻译消息关联已观测玩家，唯一死亡event_id；重复详情不会重复加分 |
| 死亡及重生 | `events.onPlayerDeath/onPlayerRespawn` | ActorEvent、血量归零、网易已知OnPlayerDie、RespawnReady；重生独立于进服 |
| 进退服及维度 | `events.onSessionJoin/onSessionLeave/onDimensionChange` | PlayerList增删与ChangeDimension；视距出现/消失不冒充进退服 |
| 屏幕频道 | `hud.set/update/remove` | 按玩家、title/subtitle/actionbar类别仲裁，优先级和到期回退；只管理走HUD接口的内容 |
| 计分板 | `scoreboard.get/set/increment` | get取实际SetScore缓存；set/increment提交原生命令，提交不等于服务器成功 |
| 调试 | `diagnostics.trace/combat/stats`、`plugin.stats` | 包数量、来源、过期、事件编号、排队/回调墙钟耗时、丢弃、全宿主内存/GC、断线原因 |
| 时间 | `util.monotonicMs()`、`util.serverTick()` | 单调毫秒；serverTick只有真实非零UpdateAttributes tick才known=true。本地scheduleTick不冒充服务器同步Tick |

64位RuntimeID、UniqueID使用十进制字符串，避免Lua/JSON浮点精度丢失。旧`bot.self()`、`player.health()`仍保留平面health/max_health字段；新结构在`health_state`中。`entities.get()`的health直接是结构，不能混用两个字段形态。

装备每个槽返回`known/item_id/item_id_known/network_id/enchantments/enchantments_known/received_at/age_ms/stale`。已明确收到空物品才返回minecraft:air；未收到槽位不假定空。NBT没有附魔字段时，无法断定无附魔。装备包不含物品有效期，因此`expires_at=null,expires_at_known=false`；更新时间与失效时间不是同一个概念。

## 插件示例

```lua
events.onEntityHurt(function(e)
  local h=entities.health(e.target_runtime_id)
  if not h.known or h.stale then return end
  if e.attacker_known and e.attacker and e.attacker.name then
    hud.update(e.attacker.name,"combat",{text=string.format("%s：%.1f / %s",
      e.target.name~="" and e.target.name or e.target.entity_type,
      h.current,tostring(h.max or "未知")),priority=100,duration_ms=2000})
  end
end)

events.onPlayerKill(function(e)
  -- 宿主会去重，自己的业务也可保存event_id抵抗插件重启后的重复消费。
  local key="kill:"..e.event_id
  if not storage.get(key) then
    storage.set(key,true)
    scoreboard.increment("kills",e.killer,1)
  end
end)
```

没有可靠攻击者证据时，当前`attacker_known=false`，示例不会向附近所有人显示，也不会猜最近玩家。`Animate`只是挥臂。普通`ActorEvent`没有攻击者ID；不能据此承诺“任何人打任何实体都百分之百识别”。服务器死亡翻译消息可以提供击杀归属，与每次普通攻击归属是两回事。

Python使用`host.entities.health('5332')`、`host.events.onEntityHurt(callback)`等同名接口。C++使用`host.entities.health("5332")`和已注册回调名。SDK目录与MCP真实宿主API目录来自同一注册表。Linux目前原生C++运行器仍未开放；C++头文件提供绑定不代表Linux加载native插件已验收。Lua与Python可按实际平台运行。

## 事件与队列

Lua受伤、血量、死亡、击杀、重生、进退服、维度、装备事件有独立优先队列和消费者。普通事件另排队；网络读包只做缓存和投递。共享Lua状态仍由VM锁保护，纯CPU回调不能真正同时执行；宿主等待I/O的协作释放允许其他回调继续。Python/C++队列容量也能配置，但本轮未把它们的原生运行器重写为多线程战斗队列，不能宣称三个语言已做到完全一样的隔离。

`plugin.stats()`新增priority_queue_depth/capacity和last/max_queue_wait_ms。回调耗时仍是墙钟，不冒充CPU；`diagnostics.stats()`的heap/GC是全宿主数据，不冒充单插件分配。50ms受伤关联是事件证据合并窗口，不是命令发送延迟，也不会给循环命令链增加逐条回包等待。

## 验收与尚未完成

已用真实协议类型构造测试：非20最大血量、伤前/伤后、未知最大值、NaN拒绝、旋转无移动、延迟物品注册表、真实附魔NBT、ID复用、死亡后离开视距仍关联击杀、重复kill去重、重生去重、函数回调、原异步绑定、配置持久化、3000环形记录顺序、预算覆盖、HUD优先恢复、SetScore缓存清除。完整测试和Linux结果保存在本轮输出。

仍待真实游戏配合验证：装备换装与俯仰变化的广播范围、其他玩家死亡翻译包、战斗HUD肉眼效果、长时间多插件混合负载。普通受伤的全覆盖攻击者识别取决于服务器是否提供可靠来源，当前没有伪造实现。自动重连已有宿主机制，本轮加入关闭/踢出/成功连接诊断；逐次重连失败的完整结构化报告和跨重启持久诊断仍待补齐。

以前分享对话入口已重新尝试读取，其中页面未返回完整正文／超时。本轮没有把未读取的内容当事实；以用户贴出的UpdateAttributes字段与当前实际内核协议定义核对实现。


---

# 游戏协议与玩法技能

游园当前内核实际注册 **215种** 协议包。包名、ID、字段和嵌套类型都可以离线查；保留原版game.sendPacket/events.subscribe/events.subscribeAll并提供packet别名。不是把网上1.26的ID直接填到网易内核。

## 按玩法找包

| 想做的玩法 | 原始包 | 高级入口 | 不能从它单独知道的东西 |
|---|---|---|---|
| 挥臂技能/挥空菜单 | Animate | events.onSwing | 命中目标、击杀者 |
| 实体/BOSS/掉落物出现 | AddActor/AddItemActor/AddPlayer | events.onEntitySpawn | 全世界实体、未同步的血量 |
| 拾取奖励 | TakeItemActor | events.onItemPickup | 未观测掉落物的完整NBT |
| 血量、受伤、死亡 | UpdateAttributes/SetHealth/ActorEvent/DeathInfo | player.health、events.onHealthChange/onHurt/onDeath | 服务器没传的其他玩家血量、准确击杀者 |
| 速度、方向、加速度 | MovePlayer/MoveActorAbsolute/Delta/SetActorMotion | player.position、events.onMove | 未观测位置、服务端权威原因 |
| 持有/穿戴特殊物品 | MobEquipment/MobArmourEquipment | events.subscribe | 所有玩家全部背包 |
| 自己的箱子菜单与槽位 | ContainerOpen/InventoryContent/InventorySlot/Close | events.subscribe | 他人私有容器、伪造交易 |
| 使用物品、挖掘、攻击 | PlayerAuthInput/PlayerAction/InventoryTransaction | world.mineBlock/useBlock、bot.attack | 他人的所有客户端请求 |
| 方块变化、区块与结构 | UpdateBlock/LevelChunk/SubChunk/Structure* | world.blockState/getBlockTile、结构工具 | 确认破坏者、已实现全世界缓存 |
| 命令、权限与计分板 | AvailableCommands/CommandRequest/Output/SetScore | game.execute/sendCommand/getScore | 绕过权限、任意长链同刻原子性 |
| 网易特殊事件 | PyRpc/NeteaseJson/SyncSkin | packet.on/describe/send | 未知事件结构、BDS服务端全权限 |

## 三种语言监听同一个包

```lua
function arm(data)
  if data.action_type == 1 then print("挥臂实体", data.entity_runtime_id) end
end
events.subscribe("Animate", "arm")
```

```python
def arm(data):
    if data.get("action_type") == 1:
        print("挥臂实体", data["entity_runtime_id"])
def on_init(host):
    host.packet.on("Animate", arm)
```

```cpp
// 放在yy::Plugin派生类构造函数中
on("arm", [](const yy::json& args) {
  const auto& data=args.at(0);
  if(data.value("action_type",0)==1) return data.at("entity_runtime_id");
  return yy::json(nullptr);
});
client.packet.on("Animate", "arm");
```

## 查询与发包闭环

先packet.describe("RequestChunkRadius")查询字段，然后packet.send("RequestChunkRadius", {chunk_radius:4,max_chunk_radius:8})请求；订阅ChunkRadiusUpdated查看服务器实际允许范围。数字ID可用，但以当前schema为准。Lua字典用{chunk_radius=4}，Python用{"chunk_radius":4}，C++用yy::json{{"chunk_radius",4}}。payload_hex只包含完整包体，不加header、压缩或加密层；不能与结构字段混合。

支持嵌套struct、数组/向量、map、数字、布尔、string、hex字节/UUID、bitset索引数组。未知字段、溢出、错误类型提前报错；InventoryTransaction等interface联合分支优先高级API，不能把任意字典当合法事务。生命周期握手由连接系统负责。submitted/confirmed=false不是服务器执行成功。监听自己出站请求也不能伪装成旁观收到其他玩家输入。

## 接成原来的命令链

事件回调可以chains.start("技能.txt")启动预编译链，或game.sendCommandsFast(commands)按序提交预加载指令。不要在每次包回调重新读TXT、写JSON、创建VM。需要变量参与时只生成必要命令；同一事件可只选一条链。有条件命令按真实CommandOutput做Barrier；没有结果依赖就保持原sendCommand连续发送方式。20Hz是整轮目标，每轮含所有节点。

## 速度与死亡的可信度

位置样本差分：速度=(新坐标-旧坐标)/真实采样秒；水平速度=sqrt(vx²+vz²)，垂直速度=vy。瞬移、维度变化、离视距、长时间无同步时重置采样；位置噪声不能直接判挂。实体消失不是死亡；受伤+附近玩家也不足以确认击杀者。网易ModEventS2C/OnPlayerDie等必须先用真实包验证事件结构再封装，不把其他框架的事件名直接塞给服务器。

技能章节：index.md为玩法说明；catalog.md为全部包；可用section="Animate"精确读取单包，不要把215包全部加载到简单任务上下文。


## 0.10.34 接收诊断与 1.21.120 挥臂协议

diagnostics.receiveTransport() 的 received 每项新增：
- decoded_frames：已读取完当前定义字段的帧。
- decode_panics：字段读取中断；没有返回正常包。
- trailing_bytes_frames：读取已知字段后仍有扩展字节，不等于丢包。
- returned_packets：协议转换后返回上层的包数，不保证插件回调完成。
- decode_errors：保留旧异常合计；禁止直接当丢包数量。
这些都是当前连接累计值，应对两个时间点取差值；重连后重新计数。

当前网易协议860/游戏1.21.120的 Animate 读取 action_type、entity_runtime_id、固定float32 data，划船动作另外读取 boat_rowing_time。packet.on("animate", handler) 事件保留旧字段，增加 data/boat_rowing_time。Lua、Python、C++使用同一解码链路，不需要重新抓包猜字段。挥臂的action_type=1；data是协议动画参数，不能自行解释为伤害或生命值。UpdateAttributes 的 minecraft:health 才是实体血量来源。

接收统计没有保存原始认证、地址或载荷。队列/历史容量、解码兼容性与服务器广播范围必须分别判断：修改本地缓冲不能生成服务器未发送的包；OP也不能据此宣称全服原生挥臂/血量已实现。普通租赁服的全局原生动作与血量仍未通过真人跨距离/跨维度验收。

## 0.10.35 可选观察与事件
宿主统一events.on/catalog/onObservedAction/define与observerBridge.patrolStart/Status/Stop见各语言observation-patrol.md；默认不运行轮巡。账号池invoke可以给各独立机器人选择单玩家跟随；所有事件保留范围/来源，不能把受伤模拟为空挥。账号界面新增查看和复制令牌。
