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


## 0.10.24 实际视距与收包诊断



### `diagnostics.visibility()`

返回当前账号连接的 `requested_radius_chunks`、`granted_radius_chunks`、时间及来源，`publisher.position/radius_blocks`，选中解码包计数，收包队列占用，抓包覆盖与抽样统计。未知半径为 null。`entity_broadcast_radius_known=false` 明确表示实体范围没有被服务器告知。原 `packet_counts` 只统计消费者已处理的包，不代表网络上所有数据包。源码诊断补丁新增 `ingress.types[类型]` 的 `decoded/selected/filtered/enqueued/cancelled`：依次为监听筛选前收到、被选中、被筛选、已投递、断线取消。`Unknown:ID` 区分未实现字段解码的不同包ID。仅覆盖监听注册后进入监听器的包，不含登录阶段被内核消费的包或解码失败；实时读取不是事务快照，selected可能尚在等待投递。没有逐包正文保存、抽样、写盘或新增发送限速。重连后半径与统计重建。**该ingress补丁尚未发布到0.10.24安装包或线上Linux；只有更新此源码构建才有此字段。**

### `bot.requestViewDistance(chunks, timeout_ms=5000)`

发送当前内核的 RequestChunkRadius，同时填写 ChunkRadius 和 MaxChunkRadius，等待真实 ChunkRadiusUpdated。参数 chunks 允许协议正 int32（1..2147483647），没有额外 32 区块上限；这个类型范围不等于建议运行值或服务器允许值。用户按资源情况从 16、32、64 逐步请求，查看实际批准值。大幅提高半径会增加区块数据和内存，不能靠发送极大整数获得全世界实时事件。

返回 submitted、confirmed、requested_radius_chunks、granted_radius_chunks、clamped。超时为 confirmed=false / granted_radius_chunks=null。confirmed 只确认半径回包，不确认实体广播范围。协议不带请求ID，宿主串行此高层方法；同时通过其他 raw 发包调用更改半径，无法严格关联响应，应避免。该协商只影响本次连接；不会改变命令链 Hz 或 Fast 命令限速。

Lua：

```lua
local before = diagnostics.visibility()
local result = bot.requestViewDistance(32, 5000)
print(game.jsonEncode(result))
events.onHealthChange(function(e)
    -- e.source 为真实包；未收到其他实体的新属性不能声称已知其当前血量。
    print(game.jsonEncode(e))
end)
events.onSwing(function(e) print(e.player, e.runtime_id, e.source) end)
```

Python：`host.bot.requestViewDistance(32, 5000)` 和 `host.diagnostics.visibility()`。Python 的 host 为当前插件 SDK 客户端，参数/结果与 Lua 相同。

C++：`client.bot.requestViewDistance(32, 5000)` 和 `client.diagnostics.visibility()`；Android 支持原生运行器，Linux 原生 C++ 运行器尚未支持。MCP/AI 通过实际 `api_youyuan_game_call` 或带 plugin_id 的 SDK 调用相同方法，完整技能可按需读取 `youyuan://docs/network-observation`。


完整限制与扩大覆盖方案见可选技能 network-observation。


### 入站 PyRpc 事件目录诊断（源码待发布）

显式启动包含 `PyRpc` 的抓包后，读取 `diagnostics.visibility().ingress.rpc_catalog`。
它只整理实际收到的 `ModEventS2C` 标准帧，返回 `events[]` 的 `namespace/system/event/operation_type/count/first_seen_at/last_seen_at`。
MessagePack 的字符串和二进制字符串名称均支持。不会复制事件参数到目录，不发试探请求，不改变原始包与插件订阅。
其他帧形态只增加 `unrecognised`，表示目录解析器不识别，不表示协议解码失败或服务器没有对应能力；原始抓包仍可查看。

目录容量使用 `diagnostics.configure` 的 `event_history`（1..65536），按首次发现顺序移除旧事件身份；`evicted` 显示移除次数。容量限制的是诊断目录，不是接包数量、命令频率或游戏事件广播。
一个事件移除后再次出现，其留存 `count` 重新从1开始；`observed` 是此次连接、发现采集启用期间的累计RPC数。
停止抓包后停止采集，已发现目录保留到当前游戏连接重置，时间戳使用Unix毫秒；相同名字但不同命名空间/系统/OperationType分别计数。

收到事件不能证明同名服务端处理函数可被调用，不能据此自动生成“全服挥臂”接口。
`ListenForEvent` 注册所在引擎/模块的本地监听；`NotifyToServer` 请求必须由服务器的实际处理器接收。
玩家输入代理、服务端事件转发与普通机器人收到的同步包要保留不同来源，攻击者、维度、血量未知时不编造。
此目录属于尚未部署的源码补丁，已发布0.10.24安装包和线上Linux尚无此新增字段。




## 0.10.25 接收交接与采集桥

登录阶段使用同一个同步读包者完成认证，保留应用层 ModEventS2C；先安装宿主监听器再回放缓存和启动实时读取。认证挑战本身不重复投递。缓存满时明确报错，不默默丢包。`diagnostics.visibility()` 的 `login_ingress` 与 `ingress` 是两个有重叠的阶段，不能相加计算总包数。后者含登录缓存回放；前者只保留计数及 RPC 事件名称，不保留认证参数。

新增命名空间 `observerBridge`，Lua、Python 和 C++ 通用调用同一宿主实现；插件仍须满足宿主授权配置。所有事件带来源，采集客户端报告不会写入服务器属性缓存。

| API | 参数与作用 |
|---|---|
| create | `{name, source="player_input"或"game_server", server, player_id}`；玩家来源必须绑定身份。返回独立密钥一次，存储 SHA256 摘要。 |
| sources | 无参数。来源清单、序号及最后上报时间，不返回密钥。 |
| revoke | `source_id`；撤销密钥。 |
| events | `after=0, limit=256`；分页读取采集桥事件与覆盖数。历史容量沿用可配置 EventHistory，重启清空内存历史，来源序号持久化。 |
| attachSelf | `source_id, token`；采集当前机器人自身加密前的本地提交。server 与 UUID/XUID/名称必须匹配。异步队列容量沿用 EventQueue，50ms 聚合投递；不改变游戏命令频率。 |
| inputStatus | 无参数。active/submitted/dropped/last_error/queue_capacity；submitted 是采集桥已接收，不是游戏执行成功。 |
| detachSelf | 无参数。停止自身采集，不断开机器人。 |
| rpcProbe | `timeout_ms=3000`；发送 YouyuanBridge/ObserverV1/Hello，只有匹配 request_id 的 Capabilities 才确认。 |
| rpcQuery | `{query,target?}, timeout_ms=3000`；只查询已经安装的服务端 Provider。没有模块则没有凭空读取能力。 |
| rpcEvents | `after=0,limit=256`；读取自定义 Observation，来源 server_module_reported，重连清空，独立于原生实体缓存。 |

Lua 示例：
```lua
local state = bot.self()
-- server 与 player_id 使用当前连接的真实服务器和身份，不填示例身份。
local source = observerBridge.create({name="自身采集",source="player_input",server="94F195C41B5",player_id="当前机器人名称"})
observerBridge.attachSelf(source.id, source.token)
local status = observerBridge.inputStatus()
local received = observerBridge.events(0,64)
observerBridge.detachSelf()
observerBridge.revoke(source.id)
```
Python 常驻 Host：`host.observerBridge.events(0,64)`；C++ 使用 `client.observerBridge.events(0,64)` 或通用 `client.invoke("observerBridge.events", yy::json::array({0,64}))`。密钥不写入公开文档、日志或插件发布包。

入站真实血量继续从 `UpdateAttributes.attributes[].attribute_value` 的 `minecraft:health` 更新 `entities.health(runtimeId)`；未收到值保持未知。`bot.swingAir()` 在下一正常输入心跳加入一次 PlayerAuthInput.MissedSwing，复用位置与视角，输入 Tick 递增并在重连时归零；重复排队请求合并。心跳关闭时拒绝请求，导入导出任务占用时等待心跳恢复。queued/submitted/confirmed 分开，不能把输入当服务器已接受。直接发送旧式 Animate 在本次网易服测试中导致第二机器人断线，bot.attack 已去掉该发送。攻击输入来自 InventoryTransaction 的 UseItemOnEntity/Attack，仍不证明服务器扣血。

认证交接后若服务器再次发送 StartType/GetMCPCheckNum，由同一个运行期读包者转交挑战处理器，已通过的认证不重复提交。该修正经0.10.24/0.10.25同账号对照定位，修正后 list/summon/tag/damage/kill 获服务器确认。

外部客户端/代理把真实事件摘要作为 JSONL 输入 `observer-bridge/agent.py`，POST `/api/youyuan/observer/ingest`，请求头 `X-Youyuan-Observer-Token`。该程序是认证上报器，不能自动解密或拦截游戏。每批序号持久化，重试相同序号不重复记录；默认每行立即上报。远端网关若另有登录要求，配置额外 headers。

NetAllay 集成源码及 YouyuanObserver 服务端插件入口位于 observer-bridge/netallay。协议核心5项本地回归通过；四个类已用真实 NetAllay0.1.1、Allay API0.28.0编译为Java21目标。入口读取 observer.properties 的 allowed_uuids（逗号分隔已认证UUID，默认空禁止接入），禁用时撤销监听；转发真实引擎健康变化、伤害、死亡、进退服。可取消事件明确标注阶段，不能把伤害请求视为最后扣血；攻击者无证据时返回未知。AllayReadOnlyProvider只声明在线玩家名单与指定玩家真实引擎血量，不把所有实体默认成20血。仍未在真实Allay服务器安装及运行，网易山头服不能凭OP安装Java插件；这不改变原服内核。

服务端安装后可主动转发引擎事件给已授权观测机器人，转发适配器不按机器人距离筛选。但必须由真实事件监听产生，并在服务端验证身份；跨距离/跨维度全局效果以服务端实测为准。没有匹配回复只能判断该桥未响应，不能判断全部 PyRpc 不可用。不要猜测/循环发送其他模块事件名。

## 0.10.26 多机器人联合观测

这些接口汇总当前宿主管理的已连接机器人实际收到的服务器解码包。没有收到的数据仍然未知。不会更改本机 entities 缓存，不自动创建机器人、不执行查询命令、不在收包线程进行网络访问，也不保证覆盖全世界。

| 调用 | 用法 |
|---|---|
| observerBridge.coverage(options) | options.server 默认当前服。返回观察账号、名字、位置、维度、会话、实际观测实体/已知血量数量和错误。账号断线明确显示。 |
| observerBridge.health(runtimeId, options) | runtimeId 必须十进制字符串；options 可指定 server、dimension。查询在线观察者最新真实血量，返回 current/max/previous/damage/stale/source 和 observer。身份或维度冲突不猜测，known=false。 |
| observerBridge.locator(target, options) | target 是玩家名或 UniqueID 字符串，读取真实 PlayerLocation 广播；没有 AddPlayer 也保留。返回 dimension_known=false、runtime_id=null，不把位置广播当血量或维度证据。隐藏类型不返回假坐标。 |
| observerBridge.worldEvents(options) | options 包含 server、dimension、cursors、limit；返回联合规范事件与每观察者游标。limit 默认256，1..4096，属于每次读取条数而非游戏命令/收包限速。 |
| observerBridge.localView(options) | 当前账号本地视图，用于聚合，不递归查询兄弟账号。mode=coverage/health/events；health 用 target；events 用 after、session、limit。 |

Lua 示例（目标 ID 使用真实抓包或实体事件取得的 ID，不能照抄示例数字）：
```lua
local coverage = observerBridge.coverage()
local result = observerBridge.health("真实RuntimeID", {dimension=0})
if result.known and not result.health.stale then
    print(result.health.current, result.health.max, result.observer.name)
end
local cursors = {}
-- 每次需要读取时调用；不要同步死循环或用1ms轮询HTTP。
local page = observerBridge.worldEvents({cursors=cursors,limit=256})
cursors = page.cursors
for _,event in ipairs(page.events) do
    print(event.type,event.observer.name,event.observation_id)
end
```
Python：`host.observerBridge.health(runtime_id, {"dimension":0})`；C++：`client.observerBridge.health(runtime_id, yy::json{{"dimension",0}})`；三者均通过相同宿主实现。MCP 使用游戏调用工具，method 填上述完整名称，args 为参数数组；开发文档 Resources 可读取此页。内置 AI 加载对应语言/观测技能后也能查看这些接口，不强制加载全部技能。

事件来源标为 server_to_bot_decoded；客户端输入桥仍为本地提交，不能把它当服务器已执行。多个观察者对同一事件的不同目击保留来源，不能直接把联合事件条数用作击杀计数。observation_id=账号/会话/序号，可去重同一观察记录；跨观察者业务去重需真实目标、时间和事件证据。下一页传返回的 cursors，重连 reset=true，历史丢失 lost>0；失败账号的旧游标保留，恢复后继续读取。complete 只代表本轮所有已启动账号查询无错误，并非全服覆盖完整。

当前控制器只联通自己的主账号、舰队与托管子账号；其他用户或其他机器要通过显式认证采集桥接入，不能自动跨用户读取。托管子进程仅查询自己；控制器在1秒内按需并发读取，错误显式返回。维度、服务器、连接会话不同不能混合实体；不存在的实体、已移除实体和断线缓存不充当有效观测。

修复了旧宿主把其他玩家 PlayerLocation 写入 bm.pos 的问题。现在仅相同 UniqueID 的坐标类型更新机器人自身位置，Hide 不将位置清零，其他玩家的广播进入独立 locator 缓存；MovePlayer 与 Respawn 同样校验身份。locator 缓存有可配置实体容量并在重连清空。该包没有维度字段，所以不能按维度查询 locator，也不能据它断言跨维度监听成立。

可直接用于：主插件显示远处观察机器人收到的生物血量、查看各观察账号覆盖的维度、从主账号统一读取远处健康/受伤/生命周期等已规范化事件。空挥、攻击归属、全服装备是否可读仍以服务器实际下发的包为准，接口没有创造隐藏数据。

## 0.10.27 真实远距离服务器模块战斗事件

来源是山头测试服实际接收的 PyRpc ModEventS2C，不是猜测包名，也没有强制请求服务器隐藏数据。本适配只解码经过记录验证的两个模块广播，operation_type=32346342；不能因为其他 ModEventS2C 常用9753608而忽略它。模块不存在或不广播时，接口不会伪造事件。尚未证明跨维度或任意服务器可用。

| 接口 | 参数与含义 |
|---|---|
| events.onEntityKill(handler) | 服务器 Minecraft/vipEventSystem/PlayerKillMob<UUID> 模块报告。event.attacker.unique_id、target.unique_id 都是精确十进制字符串；attacker_known 表示模块明确报告该攻击者。名字只关联实际在线 PlayerList。没有目标类型、RuntimeID、健康值就返回未知。不是 PvP onPlayerKill。 |
| events.onDamageDisplay(handler) | BREquipDamageEventServer/BREquipDamageEventServerSystem/GetBroadcastData。target.unique_id、damage_text、position、position_known。damage_text 如 '-33' 是显示文字，damage_known=false；不是真实HP扣血。无攻击者、维度字段，因此 attacker_known=false、dimension_known=false。 |
| events.moduleHistory(options) | options.session、after、limit；独立战斗记录，移动与乐器高频事件不会挤掉它。返回 session/cursor/latest_sequence/lost/reset/events/capacity。limit默认256、1..4096；容量使用diagnostics.configure的event_history。首次after=0；后续传返回cursor及session。重连清空并reset，覆盖记录会lost。 |

所有事件带 source=PyRpc、evidence=server_module_reported、module.namespace/system/event/operation_type、event_id、module_sequence、timestamp。不会据模块显示更新 entities.health，真实血量继续来自 UpdateAttributes。observerBridge.worldEvents 保留这个证据类型，另外标明 transport_evidence=server_to_bot_decoded。多观察者各自的 event_id 不等于跨账号击杀唯一号；业务跨观察者去重要按服务器/模块/攻击者/目标UniqueID处理。

Lua：
```lua
events.onEntityKill(function(e)
    local who = e.attacker.name or e.attacker.unique_id
    print(who .. " 击杀 " .. e.target.unique_id)
    -- 可在这里给明确的攻击者计分；不要拿目标缺失血量当满血。
end)
events.onDamageDisplay(function(e)
    print(e.target.unique_id, e.damage_text, e.position_known)
    -- 只用于模块报告的伤害显示；不要拿它做真实血量统计。
end)
local page = events.moduleHistory({after=0,limit=32})
-- 下一次按需读取：events.moduleHistory({session=page.session,after=page.cursor})
```
Python 通过 `host.events.onEntityKill("handler_name")` 注册宿主回调（见 Python SDK 的 handler 机制），C++ 通过 `client.events.onEntityKill("handler_name")`。三语言均可读取 moduleHistory；MCP 游戏调用 method='events.moduleHistory'、args=[options]，无需插件订阅上下文。内置AI可以按语言加载本地观测技能，看到完整示例与真实性边界。技能没有自动全部加载。

击杀去重：当前连接会话中以模块来源、攻击者UniqueID、目标UniqueID做有界缓存，容量沿用event_history；连接重建清空，缓存淘汰后旧报告重放可能再次出现，业务长期积分需自己的持久幂等键。伤害显示是连续更新，不用 requestID=1 去重；该编号可重复，不能当全局事件唯一号。

本次真人疣螈远处测试收到三条生物击杀报告及31条模块伤害显示。4753条乐器演奏事件保留在原抓包能力中，不自动变成战斗事件，也不能把它们叫作空挥。严格远距离阶段没有收到这个玩家的普通 Animate、MobEquipment、UpdateAttributes 或连续视角更新；不要把其他实体的包计到他身上。已有血量4/20后来过期仍明确stale，绝不充当远距离实时血量。

## 0.10.28 Lua坐标表修复

实际真人回调落盘确认了2次击杀和20条伤害显示，也发现显示position曾传成Lua字符串。现在position使用三语言都支持的数组绑定：Lua为table，可以直接读取e.position[1]、[2]、[3]；Python/C++/MCP仍为数值数组。position_known=false时不使用坐标。专门回归覆盖Lua数组类型与三个数值，不仅检查JSON形态。

真人最新证据保留原测试版本0.10.27和真实北京时间23:11:08～23:38:19，不把旧测试改写成0.10.28实测。该阶段11条击杀报告、56条伤害显示，最新击杀23:35:44.188，Lua插件真实回调2次击杀/20次显示并落盘；临时脚本和测试牛已清理。普通原生动作与全局实时血量仍未证明。

## 0.10.29 主动服务端转发与三语言接口

目标主要是普通网易租赁服，不依赖用户上传玩法组件。以下诊断接口在机器人本机运行；主动服务端模块为另一个可选方案，不能用山头服模块实测替代租赁服广播能力。

本地协议底层Dial使用readerLimits=false，已关闭单批包数量限制。监听器在本地业务筛选之前计数；captureStart({types={"*"},filter={every_ms=0}})可观测已下发解码包，显式指定LevelChunk等大包才能抓它们。登录握手私密数据不进入公开抓包。这里的缓存容量不是服务器广播权限；修改它不会制造未下发的数据。

新增共同API：

| API | 参数 | 实际作用 |
|---|---|---|
| diagnostics.receiveTransport | 无参数 | 当前协商协议ID/版本、注册包ID列表、解码前帧数/字节数/未知包/解码失败；不保存包内容和认证信息；不需要服务端组件 |
| events.onServerObservation | handler(event) | 主动模块所有事件；e.event_type、e.data、e.event_id、e.session、e.received_at、e.source、e.evidence |
| events.onGlobalSwing | handler(event) | 只接受有明确player_id的swing报告；不把伤害、命中攻击或任意Animate伪装为空挥 |
| events.onGlobalHealthChange | handler(event) | 有明确entity_id且实际current合法的health_changed；返回e.health.current/max/previous |
| observerBridge.serverHistory | {session?,after=0,limit=256} | 当前连接历史与cursor/reset/lost/capacity；不是网络查询，重连清空 |
| observerBridge.serverSnapshot | 精确entity_id字符串 | 独立实体快照缓存，known/current/max/previous、session、received_at/observed_at/stale/timestamp_valid；不受无关历史滚动影响 |

缓存使用可配置settings.actors与settings.stale_ms；模块最大血量未知为null，不默认20，不从damage_text反推出生命值。ID为字符串，禁止用浮点数传超长RuntimeID或UniqueID。server_engine_verified=false明确表示宿主收到模块报告，不能代替本轮引擎验收。原生entities.health(runtimeId)、player.equipment(name)、player.rotation(name)继续基于实际下发包，不覆盖或冒充为全服实时数据。

Lua常驻插件：
```lua
events.onGlobalHealthChange(function(e)
  local h = e.health
  print("模块血量 " .. e.entity_id .. ":" .. tostring(h.current))
end)
events.onServerObservation(function(e)
  if e.event_type == "player_death" then
    local d = e.data -- entity_id/attacker_id/attacker_known/cause
    -- 仅attacker_known为true时归属击杀，按event_id去重。
  end
end)
local history = observerBridge.serverHistory({after=0,limit=32})
local snapshot = observerBridge.serverSnapshot("精确实体ID")
```

Python常驻插件（使用注入的命名空间，不每次启动解释器）：
```python
def changed(event):
    health = event['health']
    print(event['entity_id'], health['current'], health['max'])
def on_init(host):
    events.onGlobalHealthChange(changed)
    events.onGlobalSwing(lambda event: print(event['player_id']))
def snapshot(entity_id):
    return observerBridge.serverSnapshot(entity_id)
plugin_api = {'snapshot': snapshot}
```

C++常驻插件（使用sdk/cpp/youyuan.hpp，注册回调名称）：
```cpp
class Observer : public yy::Plugin {
  yy::json last_;
public:
  Observer(const char* context, yy::HostAPI host): Plugin(context,host) {
    on("health", [this](const yy::json& args) {
      last_ = args.at(0); return nullptr;
    });
    client.events.onGlobalHealthChange("health");
    expose("last", [this](const yy::json&) { return last_; });
  }
};
YY_PLUGIN(Observer)
```

events.onGlobalSwing的global表示已安装的服务端模块不按观测者视距筛选，不表示任意第三方服务器都支持。NetAllay API0.29.0模块已编译，真实Allay启动未验收；网易ModSDK行为包具备血量/攻击/死亡，只读查询与引擎注入身份白名单，但没有经核实的网易服务端空挥接口，明确swing_supported=false。必须安装在游戏服务器引擎中；仅在工具箱装一个Lua/ZIP不会建立该能力。rpcProbe匹配Capabilities确认接入后再使用。

Linux C++实际已有Python3独立进程ELF运行器，修正了plugin.has/capabilities曾错误显示“不支持”的信息；Android仍在宿主进程内执行native，不能承诺相同崩溃隔离。SDK回调和跨模块通信保留，新增接口不改变命令Fast路径、整条链频率或加命令CPS限制。


## 0.10.37 玩家协议代理（独立实验组件）

可运行入口是`cmd/youyuan-player-gateway`，两侧真实网易认证和密钥绑定；玩家必须实际经过该UDP入口。中心可以留在另一维度；普通手机/启动器地址适配尚未完成，不能自动监听未接入玩家。

插件继续调用共同的`observerBridge.events(after, limit)`，读取`reply.events`与`reply.cursor`。Lua使用同名函数；Python使用`host.observerBridge.events(cursor,256)`；C++使用`client.invoke("observerBridge.events", yy::json::array({cursor,256}))`；MCP通过通用game.call调用同名方法。不覆盖原模块通信、原实体缓存或命令Fast路径。

外层event包含`kind/player_id/source/event_id/received_at/server_confirmed`；event.data是采集包封装，其中`packet_type/direction/observed_at/data`保留原始来源。Animate只表示挥臂；PlayerAuthInput的内层`missed_swing=true`表示明确空挥输入；服务器UpdateAttributes的`current/max`是本人收到的实际血量。客户端输入不能当作服务器命中结果。

实测（2026-10-11北京时间19:26:56–19:27:40）：中心dimension2、协议测试客户端dimension0；Animate10/10、MissedSwing5/5、本人血量18/20；21投递0丢弃，同机投递P95=11.881ms。ItemRegistry观测失败1次但原帧仍转发，治疗命令失败；真人手机入口、多玩家管理与24小时稳定性未验收。源码技能和文档已更新，线上0.10.36嵌入资料未在此独立发布中替换。

使用与完整限制：`observer-bridge/玩家协议代理使用与验收.md`。不要反复从cursor=0处理历史；不要把输入桥误写成`events.onGlobalSwing`服务端模块自动订阅。


## 0.10.38 动作API与MCP同步

# 动作 API、事件与真实覆盖范围

## 读取玩家代理动作（已实现的共同入口）

Lua、Python、C++和MCP读取同一个`observerBridge.events(after, limit)`，参数是上一页cursor和本页条数。返回`events/cursor/head_cursor/overwritten`。保存cursor并按event_id去重，不能每轮从0重新执行历史。历史是内存，宿主重启会清空；空列表不代表所有玩家没有动作。

| 动作/数据 | kind及来源 | 事件内字段 | 本轮验收 |
|---|---|---|---|
| 挥臂 | swing_input / Animate | runtime_id，missed_swing_known=false | 10发出10收到；仅挥臂，不能判断命中 |
| 明确空挥输入 | swing_input / PlayerAuthInput | missed_swing=true，tick | 5发出5收到；是客户端输入，非服务器命中结果 |
| 攻击输入 | attack_input / InventoryTransaction | target_runtime_id，position | 已实现解析，未真人验收；不是成功伤害 |
| 移动/转动视角输入 | position_input / PlayerAuthInput | position，pitch，yaw，tick | 收到5条位置输入；未真人验收纯转头 |
| 接入者本人血量 | health_observation / UpdateAttributes | current，max，known，runtime_id，source，tick | 服务器真实18/20；未实测血量变化 |

外层event含kind/player_id/source/source_id/event_id/received_at/server_confirmed。`event.data`是采集封装，包含packet_type/direction/observed_at/data；真正动作字段在`event.data.data`。RuntimeID和Tick是十进制字符串，避免64位精度丢失。服务器血量保留direction=server_to_player、source=server_packet；外部来源不自动成为中心服务器权威缓存。

MCP工具调用：使用通用`/api/youyuan/game/call`，POST参数如下。

```json
{"method":"observerBridge.events","args":[0,256]}
```

后续用返回的cursor代替0。资源为`youyuan://docs/player-actions`；也能通过技能读取API按需加载`player-actions/index.md`。这不是要求模型自己抓原始包猜字段。

Lua：
```lua
local reply = observerBridge.events(cursor or 0, 256)
for _, e in ipairs(reply.events) do
    local packet = e.data
    local d = packet.data
    if e.kind == "swing_input" and d.missed_swing == true then
        print(e.player_id, "空挥输入", d.tick)
    elseif e.kind == "health_observation" then
        print(e.player_id, d.current, d.max)
    end
end
cursor = reply.cursor
```

Python：
```python
reply = host.observerBridge.events(cursor, 256)
for e in reply["events"]:
    fields = e["data"]["data"]
    if e["kind"] == "swing_input" and fields.get("missed_swing") is True:
        print(e["player_id"], fields["tick"])
cursor = reply["cursor"]
```

C++：
```cpp
auto reply = client.invoke("observerBridge.events", yy::json::array({cursor,256}));
for (const auto& e : reply.at("events")) {
    const auto& fields = e.at("data").at("data");
    if (e.at("kind") == "swing_input" && fields.value("missed_swing", false)) {
        // e.at("player_id")及fields.at("tick")；按业务更新已有状态。
    }
}
cursor = reply.at("cursor").get<uint64_t>();
```

## 操纵机器人与观察他人是两类 API

`bot.swingAir()`让当前连接机器人排队提交一次MissedSwing；`bot.attack(runtime_id,slot)`提交近距攻击交易；`world.mineBlock`与`world.useBlock`分别是破坏/交互提交。提交成功不等于服务器执行成功，操作仍受服务器实际权限与同步状态约束。

原生监听`events.onSwing`、`events.onEntityHurt`、`events.onPlayerDeath`以及`player.rotation`、`player.equipment`继续读取服务器同步给机器人的资料，不能通过增加客户端解析字段使未下发的信息出现。`entities.health(runtimeId)`属于原生缓存；它不会自动查询或合并代理玩家的本人血量。

`events.onGlobalSwing`是已安装并确认能力的服务端模块报告通道，**不是**本代理事件的自动回调。插件读取代理动作使用上面的observerBridge.events。不要把代理输入、原生广播、服务端模块报告混为一个无条件全局接口。

## 覆盖、延迟和未完成项

独立代理0.10.37只有实际经过UDP入口、认证UUID/UID匹配的玩家才被覆盖。本轮中心在末地dimension2、协议客户端在主世界dimension0，说明已接入玩家向中心上报不依赖中心视距；未接入的其他玩家仍不能自动监听。普通网易手机启动器一键路由、多玩家生产入口和24小时稳定性未完成。

2026-10-11北京时间19:26:56–19:27:40：21投递0丢弃，P50=7.907ms、P95=11.881ms。这只是同机代理采集到中心入库，不是公网真人输入到插件显示。手机网络、轮询周期与显示调度会增加总延迟。ItemRegistry162观测解析失败1次，原帧仍转发；不能据此宣称装备全部可用。

使用、认证配置及失败记录见独立交付包中的《玩家协议代理使用与验收.md》。资料同步不自动将普通玩家接入代理，也不表示所有动作均已完成实测。

## 0.10.39 相机观察：真实结果与边界

这是网易山头服94F195C41B5上的实验结果。挥臂对照中只有观察机器人具有OP；发送挥臂的受控账号已通过自身UpdateAbilities确认command_permissions=0、player_permissions=1。远处玩家血量另用受控账号进行小伤害测试，不将该血量试验称为普通权限手机验收。主机器人身体固定，只对它自己的相机发指令，不修改其他玩家视角。相机不提供跨维度观察，也不等于所有区域同时订阅。

现有命令接口即可使用，不存在另一个已经全局监听的隐藏API：

```lua
-- x/y/z为合法、已选择的观察坐标。使用实际观察机器人游戏名。
local r = game.execute('camera "观察机器人游戏名" set minecraft:free pos 100 80 200',
                       {channel="ai", timeout_ms=6000})
-- r.success只确认命令执行；还应核对该视角实际收到实体包。
-- 结束时恢复当前机器人的正常视角：
game.execute('camera @s clear', {channel="ai", timeout_ms=6000})
```

Python使用`host.game.execute(command, options)`；Linux C++使用`client.invoke("game.execute", yy::json::array({command,options}))`。同名命令、权限和服务器结果完全一致，不因换语言扩大接收范围。循环调度需要自行停止并清理；Fast提交不能代替观察区域已下发的确认。

原生动作仍通过`events.onSwing`或`events.on("swing", handler)`订阅；真实掉血通过`events.onHealthChanged`、`events.onEntityHurt`。无需先开启抓包才能接收这些宿主事件。`entities.health(runtimeId)`读取现有缓存，检查known/max_known/stale；未知护甲、攻击者保持未知。

2026-10-11北京时间23:09：24格挥臂10/10；1500格无相机0/10；相机观察远处10/10；清除相机0/10。发送的是受控账号真实Animate，不证明未修改手机的空挥输入，也不能判断命中。

23:15–23:16远处牛血量：观察中10→9；离开该视角后一次真实伤害未收到属性变化，原缓存标记stale；恢复视角重新同步为8，下一次伤害8→7。初始最大血量为实际同步的10，不默认20。Lua回调亦收到真实变化；测得约0.03–1.57ms是宿主事件时间到回调入口，校准误差±0.65ms，不是玩家操作到HUD的端到端延迟。

23:41–23:42，20个相隔较远的地点，每组12秒：

| 相机提交频率 | CameraInstruction | NetworkChunkPublisherUpdate | 受控账号挥臂收到/发送 |
|---|---:|---:|---:|
| 5Hz | 60 | 59 | 3/60 |
| 20Hz | 240 | 238 | 0/60 |
| 40Hz | 480 | 239 | 0/60 |

固定动作周期可能与巡回周期错开，上表不能据此认定socket丢包。缓存覆盖0、收包队列末尾0、账号未掉线；40Hz虽收到480条相机指令，实际区域发布仅239次、11种位置（含过渡位置），不能把指令回显算成20区域完整覆盖。一次只看一处会漏掉其他处的瞬时动作，增加Hz无法承诺零漏检。错峰复核、账号可用数量及原始包另列验收报告。没有5～6个有效账号时不得用同账号重复登录假装并行验收。

2026-10-12北京时间00:07:40–00:08:18，独立玩家账号“超耐磨的星魂”位于约1500格外：相机观察中实际血量20→19，观察机器人收到ActorEvent和UpdateAttributes，缓存为current=19、max=20、previous=20、damage=1；清除相机后，同样真实伤害没有收到目标属性包，旧值标记stale；恢复视角后重新收到。测试结束已治疗并恢复模式、位置和维护配置，移除临时平台和加载区域。不是仅根据命令返回值猜测血量。

00:10:12–00:13:09，主观察机器人加5个独立子机器人，20个相隔较远的观察地点。5个发送账号全部实际撤销OP；逐账号固定观察正对照均为10发10收。并发动作使用90～310ms随机间隔，避免固定周期碰巧错开。每个速度运行12秒，并在每轮20次提交后移动轮巡起点，避免高频下总跳过同一批坐标。

| 请求切换频率 | 实际提交/相机回显 | 区域发布包 | 5账号挥臂收到/发出 | 宿主CPU累计秒 |
|---|---:|---:|---:|---:|
| 100Hz | 1200/1200 | 240 | 13/309（4.2%） | 2.53 |
| 400Hz | 4800/4800 | 230 | 20/300（6.7%） | 6.18 |
| 500Hz | 5976/5976 | 238 | 18/303（5.9%） | 7.35 |

500Hz下平均提交间隔约2.0ms，20地点的请求轮巡约40.2ms；但实际区域发布仍约19～20次/秒。命令成功与相机回显不表示服务器已为该地点同步实体。不能把“请求一圈小于50ms”当成“所有玩家每50ms都被有效观察”。上表是动作可见性缺失比例，不是socket丢包率；抓包缓存覆盖0，5个发送账号各阶段仍连接，非OP。视角变化输入与移动包条数也不一一对应，不能拿它作为挥臂无遗漏的证明。

账号恢复已修复：4个云端账号的会话刷新code10时，使用原保存SDK凭据重新认证，核对原游戏ID后更新会话；令牌保持不变。四账号均真实登录并参与上表测试；测试后保留账号池记录，停用临时运行实例，恢复原权限/模式/位置。

结论：该服相机确实能带来远处实体和玩家血量，以及普通权限账号挥臂，但单相机快速轮巡没有达到多人无遗漏目标。精确战斗显示应保持目标附近的观察节点，或分配多个独立节点；单相机只可作为有覆盖空档的采样方式。真人未修改手机、跨维度和24小时稳定性仍未完成，不能宣称全局无漏包。完整数据和原始包在0.10.39相机验收报告中。

## 0.10.40 多机器人错峰观察与交接

在「设置 → 实时感知与抓包 → 管理观测协作」操作。宿主提供可组合接口，不自动安装跟随插件，不强制启动观察。观察账号需要自己的有效登录、OP和同服连接；普通用户仅能操作自己的账号池，管理员沿用既有授权池。

### 常驻单目标观察

`observerBridge.watchStart({player, mode, interval_seconds, duration_seconds, restore_position})`。
`mode` 为 camera（相机）、follow（移动本机器人）、hybrid（两者），默认 camera；interval_seconds 默认2，可配置正有限值，不限制为20Hz。慢查询串行合并，missed_intervals记录未赶上的时段。没有逐事件扫描或包处理线程里的HTTP查询。玩家离线/查询失败/账号断线会停止；插件卸载自动取消自己的观察。

`watchStatus()` 返回 active/config/submitted/last_submitted_at/target_since/region_updated/region_timestamp/region_position/missed_intervals/error/cleanup。submitted只代表提交；region_updated要求本次目标对应的真实NetworkChunkPublisherUpdate，不能当成所有实体已同步或零丢包。重复提交同一位置不会重置已建立的区域证据。只有camera方式不能跨维度。

`watchStop()` 停止自己的观察，清除本次相机；follow/hybrid默认恢复已确认的原位置，restore_position=false可不恢复。清理结果分别标明submitted/confirmed，Fast清理不会冒充回包成功。与当前账号轮巡互斥，避免两个任务争抢位置。MCP手动控制具备该账号既有权限。

### 六账号规划：五个观察，一个补位

`accountsPool.observationPlan({accounts:[id1,...,id6],players:[],dwell_seconds:2,radius_blocks:48})`。
players省略时取真实在线列表。返回 assignments（account/players/phase_seconds/handoff_reserve/robot_label）、excluded_bots和targets。已知在线机器人默认排除；内部标签youyuan_observer用于规划，不偷偷给普通玩家写计分板。最后一个账号作为补位，其他账号分摊目标，轮换需调用者按路线驱动。规划本身不会登录、传送或启动任何插件。

48格是本服已有试验的参考值，不是协议承诺；广播还依赖区块、维度、服务端兴趣范围。六个机器人无法不间断覆盖20个相互分离的位置。错峰可减少覆盖空档，不能让服务器未下发的包出现。

`accountsPool.handoff(incoming_id,outgoing_id,{player,mode,interval_seconds,timeout_seconds:5,next_player?})`：接替者必须空闲，原观察者必须正在观察同一目标。两者真实连接且同服；先启用接替者，看到匹配的本次区域发布才停止原观察。超时取消接替、保留原观察；返回ready/old_released/incoming_status/old_status/error。next_player可让释放后的原观察者进入下一目标。真实覆盖仍需用worldEvents按观察账号/session/sequence去重与核对；不能把区域发布当作击杀或挥臂证据。

### 三种语言和MCP用同一套方法

Lua：
```lua
local status = observerBridge.watchStart({player="目标游戏名",mode="hybrid",interval_seconds=2})
local plan = accountsPool.observationPlan({accounts={"main","账号2","账号3","账号4","账号5","账号6"},dwell_seconds=2})
-- 插件调度策略自行选择；不需要在接包回调中反复查询。
```
Python常驻插件：
```python
status = observerBridge.watchStart({"player":"目标游戏名", "mode":"hybrid", "interval_seconds":2})
plan = accountsPool.observationPlan({"accounts":["main","账号2","账号3","账号4","账号5","账号6"], "dwell_seconds":2})
```
C++（youyuan.hpp）：
```cpp
auto status = client.observerBridge.watchStart(yy::json{{"player","目标游戏名"},{"mode","hybrid"},{"interval_seconds",2}});
auto plan = client.accountsPool.observationPlan(yy::json{{"accounts",{"main","账号2","账号3","账号4","账号5","账号6"}},{"dwell_seconds",2}});
```
MCP通用游戏调用工具（已有OAuth/控制授权）：
```json
{"method":"accountsPool.handoff","args":["账号2","main",{"player":"目标游戏名","mode":"hybrid","interval_seconds":2,"timeout_seconds":5}]}
```

### 皮肤和QQ群名单

独立认证原账号的皮肤组件与资源地址随登录传回。账号网页「选择皮肤组件」只选择账号已有的组件，不购买；提供商拒绝时显示真实错误，成功后重新连接游戏应用。默认读取原账号设置，不为缺失皮肤强制更改账号。玩家缓存接收PlayerList/PlayerSkin与单UUID明确归属的SyncSkin；空皮肤不会覆盖已有数据，归属不明时保留未知。

QQ群 `list [页码]` 或 `.玩家 [页码]` 返回12人一页的真实在线名单及收到的头像；`.查询 游戏ID`保留资料卡。缺失皮肤显示“尚未收到皮肤”，图片上传失败回退文字。统计为本设备实际观测，无坐标和私密凭据。此版本API实施和实景验收的范围见独立验收报告，不将文档示例视作测试成功。

