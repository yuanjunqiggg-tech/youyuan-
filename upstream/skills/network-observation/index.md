# 网络视距与接包诊断（游园 0.10.24）

## 数据可信度

`UpdateAttributes` 的 `minecraft:health`、`ActorEvent` 和 `Animate` 是当前游戏连接实际收到的服务器数据。宿主对这三类包不做距离过滤。`player.nearby(radius)` 只过滤查询结果，不过滤接收。抓包 `capacity` 是历史记录条数，`every_ms` 是可选抽样；Lua/Python 事件队列是待执行回调数量。这三者均不是服务器视距。

服务器能够分别控制区块、实体跟踪和动作广播。`ChunkRadiusUpdated` 的批准值可能小于 `RequestChunkRadius` 的请求值；`NetworkChunkPublisherUpdate.radius` 的单位是方块，不是区块。扩大区块半径不保证所有实体的血量、挥臂都广播。没有收到数据，必须返回未知或旧观测；不能填满血、拿最近玩家当攻击者，也不能把轮询所得当成连续实时事件。

## 当前实际接口

### `diagnostics.visibility()`

返回当前账号连接的 `requested_radius_chunks`、`granted_radius_chunks`、时间及来源，`publisher.position/radius_blocks`，选中解码包计数，收包队列占用，抓包覆盖与抽样统计。未知半径为 null。`entity_broadcast_radius_known=false` 明确表示实体范围没有被服务器告知。原 `packet_counts` 只统计消费者已处理的包，不代表网络上所有数据包。源码诊断补丁新增 `ingress.types[类型]` 的 `decoded/selected/filtered/enqueued/cancelled`：依次为监听筛选前收到、被选中、被筛选、已投递、断线取消。`Unknown:ID` 区分未实现字段解码的不同包ID。仅覆盖监听注册后进入监听器的包，不含登录阶段被内核消费的包或解码失败；实时读取不是事务快照，selected可能尚在等待投递。没有逐包正文保存、抽样、写盘或新增发送限速。重连后半径与统计重建。**该ingress补丁尚未发布到0.10.24安装包或线上Linux；只有更新此源码构建才有此字段。**

### `bot.requestViewDistance(chunks, timeout_ms=5000)`

发送当前内核的 RequestChunkRadius，同时填写 ChunkRadius 和 MaxChunkRadius，等待真实 ChunkRadiusUpdated。参数 chunks 允许协议正 int32（1..2147483647），没有额外 32 区块上限；这个类型范围不等于建议运行值或服务器允许值。用户按资源情况逐步请求，查看实际批准值。本轮 16、32 已得到真实批准；请求64时连接关闭，原因未确定，不将64列为已验证或默认建议。大幅提高半径会增加区块数据和内存，不能靠发送极大整数获得全世界实时事件。

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

## 无视机器人距离的可靠路径

1. **服务器事件转发**：在拥有修改权限的游戏服务端安装事件模块，监听真实伤害/死亡/动作，再经经授权的服务器通道转给机器人。网易 ModSDK 有 ActuallyHurtServerEvent（实际伤害）、DamageEvent（伤害前）、PlayerDieEvent 等服务端事件；与机器人视距无关的来源是服务器模块，而不是客户端凭空获取。需要在实际山头服装入模块并核验，不把本文方案称为已部署。即使服务端接口也需要分别核对挥空手和命中事件，不能用命中代替挥臂。
2. **多账号分区观察**：把已授权机器人布置到不同区域，以各自实际连接收集数据。跨账号通信 API 已存在，但多机器人覆盖有限，不能保证无限范围。同一实体 RuntimeID 不应直接跨会话合并，要以可靠 UUID/UniqueID 及服务器/维度校验。
3. **按需靠近观测**：有权限时移动机器人到目标附近，获取新属性。可用于查询，不适合称为全服实时攻击监听。扫描之间可能遗漏事件。
4. **服务器命令/RPC查询**：只查询服务器确实公开的数据。tickingarea 让区域持续加载，不等于让远方机器人自动收到实体包。

## 调试流程

先读取 visibility。抓包 types=UpdateAttributes,ActorEvent,Animate,AddActor,RemoveActor，every_ms=0；按 runtime_id 筛选，保留足够容量。对同一个已加载目标分别在近处和远处触发操作，检查命令是否服务器确认成功，再比对收到的真实包。停止抓包只返回元数据，不把所有历史强行灌入 AI；需要时按 cursor 分页读取。

## 研究来源与边界

- ToolDelta `tooldelta/internal/packet_handler.py`：动态添加包种类及回调优先级，没有证据表明它能强迫服务器广播未发送的实体包。
- gophertunnel `minecraft/protocol/packet/chunk_radius_updated.go`：明确区分请求半径和服务器批准半径。
- Dragonfly `server/world/loader.go` / `server/session/world.go`：区块 Loader 的 Viewer 范围和实体显示/动作发送流程，说明广播范围可由服务端控制。这是公开服务端实现的证据，不能据此宣称网易内部实现完全相同。
- 网易官方 ModSDK：服务器事件能力要在相应游戏版本/服务器内安装模块；工具箱客户端订阅普通协议包不会自动获得这些服务端事件。

本轮实测数字见版本验收报告，不用模拟远距离输入测试替代游戏服务器广播实测。


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


## 0.10.37 玩家协议代理（独立实验组件）

可运行入口是`cmd/youyuan-player-gateway`，两侧真实网易认证和密钥绑定；玩家必须实际经过该UDP入口。中心可以留在另一维度；普通手机/启动器地址适配尚未完成，不能自动监听未接入玩家。

插件继续调用共同的`observerBridge.events(after, limit)`，读取`reply.events`与`reply.cursor`。Lua使用同名函数；Python使用`host.observerBridge.events(cursor,256)`；C++使用`client.invoke("observerBridge.events", yy::json::array({cursor,256}))`；MCP通过通用game.call调用同名方法。不覆盖原模块通信、原实体缓存或命令Fast路径。

外层event包含`kind/player_id/source/event_id/received_at/server_confirmed`；event.data是采集包封装，其中`packet_type/direction/observed_at/data`保留原始来源。Animate只表示挥臂；PlayerAuthInput的内层`missed_swing=true`表示明确空挥输入；服务器UpdateAttributes的`current/max`是本人收到的实际血量。客户端输入不能当作服务器命中结果。

实测（2026-10-11北京时间19:26:56–19:27:40）：中心dimension2、协议测试客户端dimension0；Animate10/10、MissedSwing5/5、本人血量18/20；21投递0丢弃，同机投递P95=11.881ms。ItemRegistry观测失败1次但原帧仍转发，治疗命令失败；真人手机入口、多玩家管理与24小时稳定性未验收。源码技能和文档已更新，线上0.10.36嵌入资料未在此独立发布中替换。

使用与完整限制：`observer-bridge/玩家协议代理使用与验收.md`。不要反复从cursor=0处理历史；不要把输入桥误写成`events.onGlobalSwing`服务端模块自动订阅。
