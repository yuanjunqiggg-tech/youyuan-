# 玩家输入采集与服务端事件桥

## 0.10.37 独立玩家协议代理（源码已实测，线上宿主仍0.10.36）

现在有可执行的`cmd/youyuan-player-gateway`，不是仅将JSONL喂给采集桥。玩家连接必须实际经过UDP代理；两侧独立认证/加密，上游申请同一玩家的公钥绑定网易登录链，UUID/UID必须匹配。资源包、世界、PyRpc与未知扩展按原应用帧转发，中心机器人可以留在另一维度。此组件没有自动修改普通手机的网易启动器。

采集工具仍使用`observerBridge.events(after,limit)`，Lua/Python/C++/MCP保持共用API。`swing_input`来源Animate只表示挥臂；来源PlayerAuthInput且`data.data.missed_swing=true`才表示明确空挥输入。`health_observation`来自该接入客户端收到的UpdateAttributes。输入不等于服务器判定命中、健康数据不等于全服任意实体。不能把`events.onGlobalSwing`服务端模块监听误写成本代理的自动订阅。

2026-10-11真实协议客户端验收：中心dimension2、接入者dimension0；10次Animate+5次MissedSwing，中心15条挥臂/空挥；5位置、1本人血量18/20；21投递0丢弃，代理采集到同机中心入库P95=11.881ms。1次ItemRegistry观测解码失败，原帧仍转发；治疗命令失败，不能用它宣称实际扣血/治疗变化验收。真人手机入口、生产多用户路由与长时稳定性仍未完成。新技能源需宿主重新构建后才成为线上内置内容，不能声称0.10.36已自动更新。

完整使用与证据见项目`observer-bridge/玩家协议代理使用与验收.md`及`outputs/0.10.37-玩家协议代理实测.json`。采用可选技能加载，保留现有跨模块通信与命令Fast路径，不增加命令CPS限制。

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

## 2026-10-11 接入路线核查（研究状态）

当前已实现的attachSelf只采集工具箱自己的机器人出站输入；agent.py只负责认证上报。不能告知用户普通玩家客户端已自动接入。两授权机器人实测24格Animate5/5、2000格0/5；远处自身输入向中心转发5/5是组末批量转发，没有真人客户端或实时延迟验收。

0.10.34修复了本服Animate缺少float32 Data导致的协议错位；0.10.36在近距离直接提交Animate并收到真实同实体动画。上文0.10.25关于旧式Animate断线的记录是历史定位结果，不应解释为当前所有Animate都不可用。Animate本身只证明挥臂动画，不能保证每次都是空挥。

新接入优先研究正常玩家加密前输入挂点，再将真实事件独立上传。替代内核NTvector固定a71da8d的结构化解码只有8类包，未内置Animate/UpdateAttributes；其engine协议回调是入站，不是出站Hook。Waterdog固定07910a2能接收网易客户端，但LoginData给后端重建SELF_SIGNED，不能据此宣称已能登录网易官方租赁服。官方后端需要有效网易登录和公钥绑定证书、运行期挑战。普通UDP转发密文不能提供明文动作。

外部采集或代理只覆盖实际接入的玩家；未接入玩家仍依赖服务器同步范围。用户已取消侵入后台方向。不得把客户端报告改成服务器确认，不得把伤害/乐器演奏/陈旧快照当作远处空挥。保留现有三语言常驻、账号池、模块通信及快速命令路径；不为采集上传添加游戏命令CPS限速。具体设计见项目2026-10-11-连接方式重审与远处动作接入方案.md；这是研究设计，替代驱动和真人采集验收尚未完成。



# 0.10.48 全局抓包、作业暂停与旧注册兼容

## 设置位置和多选
设置→服务器连接→应用启动时自动连接下方→全局抓包。独立开关，配置按钮打开自己的账号与玩家多选卡片；保存全部配置后生效。技能加载库也使用明显的复选框、选中边框和计数，支持全选/清空/统一保存。

默认全局抓包关闭、camera模式，只移动本机器人原生摄像机，身体不跟随。可选择多个玩家，或动态观察在线玩家。手动勾选玩家会自动关闭动态全选，避免选中名单被自动模式忽略。开启跟随/混合模式可能传送机器人身体，会改变区块加载、触发服务端限制、抢占建筑工作位置；不会故意传送玩家。默认排除已连接所选观察账号的真实机器人名字。

每次镜头切换间隔可配置毫秒，不等于命令链节点频率。完整轮巡约为目标数量×切换间隔。高频负责提交命令，不能保证服务器同样频率完成、零漏包或全服血量同步；原生摄像机的维度与广播限制仍存在。

## 原生选择器镜头
observerBridge.watchStart支持原player字符串（兼容旧写法）以及players数组、auto_players、camera_selector、exclude_players、pause_during_jobs。多目标或camera_selector=true的纯相机模式执行：
execute at "目标玩家" run camera "观察机器人" set minecraft:free pos ~ ~4 ~ rot 90 0
目标坐标由游戏执行器原生解析，轮巡不在每个切换前等待querytarget或命令回包；只记录submitted。普通单目标旧watchStart保留原来区域确认路径，避免破坏已有handoff逻辑。

选择器路径未核验位置时target_position_known=false、target_dimension_known=false、region_updated=false，不能把零坐标或提交次数作为全局覆盖成功。需要交接区域确认时继续用旧单目标watch/handoff。跟随/混合多目标路径依然存在必要位置查询，不能把纯镜头Fast路径速度直接当身体跨维度跟随速度。

## 原生配置API
GET /api/youyuan/global-capture：config、states和global_coverage_guaranteed=false。
POST支持部分字段更新；例如{enabled:false}保留其他配置。配置字段：enabled、accounts数组、players数组、auto_players、mode(camera/hybrid/follow)、interval_seconds、camera_height_blocks、camera_pitch_degrees、camera_yaw_degrees、pause_during_jobs。只控制自己绑定设备/账号池，不能选别人的账号；控制权限与只读权限分开。
程序保存到设备global-capture.json，重启后恢复开关和参数。只管理自己的已连接账号，失效/未连接显示错误，不伪造已连接；不覆盖其他插件已运行的观察任务。需先在账号管理连接要用的机器人。关闭后释放本模块自己启动的任务。

## 导入导出作业暂停
宿主按机器人实例维护空间作业租约，覆盖主程序导入/导出/修补/地图画/皮肤建筑、多机器人导入及game.importBlocks插件导入。区域导出在每个实际参与机器人上增加租约，building.exportRegion与placeContainer的NBT异步作业也会暂停本账号观察。作业开始暂停原生跟随/TP轮巡；作业结束才恢复。多个作业引用计数，结束一个不能误恢复仍被另一作业占用的机器人。
默认相机也暂停，避免抢区块视距；配置pause_during_jobs=false只允许纯相机继续，身体跟随仍暂停。停止观察期间不会把机器人恢复传送到旧起点去打断导入导出。
game.teleport/game.move及常见自己TP的插件命令在作业期间明确返回暂停错误；普通聊天、查询和非移动命令不受影响。不关闭/重载插件，不重置Lua/Python状态。任意插件自行构造原始移动包或绕过标准API不在这种合作式暂停保证内，不能宣称隔离全部自定义行为。

Lua：
```lua
local job=observerBridge.jobState()
if not job.paused then
  observerBridge.watchStart({players={"甲","乙"},mode="camera",camera_selector=true,
   interval_seconds=0.025,camera_height_blocks=4,camera_pitch_degrees=90,pause_during_jobs=true})
end
```
Python同名接口：observerBridge.jobState()，observerBridge.watchStart({"players":["甲","乙"],"mode":"camera","camera_selector":True,"interval_seconds":0.025})。
C++：client.observerBridge.jobState()、client.observerBridge.watchStart(yy::json对象)，ABI保持不变。跨账号保留accountsPool.invoke；配置由插件自行决定，程序页面不要求依赖特定多功能插件。
高频传送插件应先检查jobState().paused并跳过当前轮，避免反复触发暂停错误。直接错误不是“已成功传送”，不得忽略错误后编造结果。

## 旧Lua注册接口兼容
注册参数保持兼容：game.registerTrigger(words,handler)、events.onChat(handler)、events.onJoin(handler)、events.onLeave(handler)、events.onBotReady(handler)、events.onPacket(handler)、events.subscribe(packetType,handler)、util.schedule(seconds,handler)、util.setTimeout(seconds,handler)、web.action(page,action,handler)。handler可为原全局函数名或函数对象。回调参数分别是onChat的(name,message)、onJoin/onLeave的(name)、onBotReady的无参数、onPacket的(packetType,data)、subscribe的(data)，不要把回调参数当注册参数。
上述注册现在也接受Lua函数对象（包括模块返回的函数与闭包）。加载一次、常驻回调，不重跑main.lua。事件、包订阅和停止清理按机器人账号隔离；停止一个账号的同名插件不应移除另一个账号的事件/定时器。
旧ZIP导入仍保留manifest、main.lua、modules、command_txt等资源目录。新的测试实际导入旧ZIP，读取模块，再触发聊天回调；不是只检查语法。没有提供的旧宿主私有注册接口不会伪造一个空函数来骗过导入；遇到具体缺失函数需按原定义补接并验收。

## 本轮证据与限度
原生选择器相机命令在Linux游戏服对本机器人测试，服务器confirmed=true、success_count=1，随后camera @s clear同样确认成功；没有身体TP。这只证明该指令路径可执行，不证明远距离真人动作、多人覆盖率或跨维度血量。
新测试覆盖：旧字符串/函数回调不重置与跨账号隔离、旧ZIP模块和TXT导入、空间租约嵌套与恢复、原生相机不拼querytarget/TP、账号池越权拒绝、多选账号/玩家/技能一次保存。界面行为用模拟DOM测试；实景视觉验收单独记录。


## 0.10.49 多选边界修复与实测
全选列表包含机器人自己时，多目标轮巡自动跳过自身，继续观察其他目标；原单目标只选自己仍明确报错。账号排除名单继续生效。旧抓包页移除重复配置卡片，入口统一在设置→自动连接下方。
0.10.48真实Linux测试：观察非OP子账号的原生镜头40次/秒提交；单方块导出生成272字节mcstructure，导出期间提交计数固定2867，结束后增长至2873，作业状态恢复。测试后关闭全局抓包、停止任务并确认清除镜头。仅为配置/暂停恢复实测，不是远距离多人收包率证明。


# 0.10.50 收包与常驻运行性能

优化保持原始协议字段与旧插件调用形式。packetToMap缓存固定的字段索引与snake_case名，仍生成独立结果，完整转换二进制hex/数组/NBT；不会截短订阅包。订阅列表仅在增删时发布不可变快照，分发不再每包复制列表或争抢注册锁；同次分发的回调顺序及退订语义保持。协议目录所有构造包字段对照旧转换器，带数据的移动/血量及大ID对照通过。

同一台Linux、同一组fixture、单核、各3次：0.10.49移动包转换中位4846ns，当前1681ns（2.88倍）；血量包8082ns到3919ns（2.06倍）；订阅分发5173ns到1934ns（2.67倍）。这是解码后的宿主转换/分发微基准，不是全游戏或网络RTT倍率。移动车体状态缓存约292ns保持；不会把未变化指标包装成加速。

保留game.getScore、getPlayerPosXYZ、getPlayerItem、getBlockTile、getBlockTileRaw、getTickingAreaList、getBlockData/getBlockNBT/getBlockState/getRegionBlockData及placeNBTBlock/setBlockState的同步返回/错误；等待Go网络/文件/工作台时释放常驻Lua锁，其他回调和定时器可继续运行。Lua输入先复制为Go值，返回后再拿锁写Lua；不在两个Go线程同时执行同一份Lua指令，不重跑源码或丢弃模块状态。旧查询实际耗时仍可能包含网络等待。

区域读取、NBT放置与状态修改也使用按账号空间作业租约，避免观察跟随抢占操作位置；参与作业的内部传送保留，不拦成空功能。初始化阶段保留原初始化顺序，不能将回调期协作等待承诺成初始化所有网络I/O都不等待。

plugin.stats()/MCP /api/plugin/profiler增加io_waiting、io_calls、io_total_wait_ms、io_max_wait_ms，区分网络等待与执行墙钟。total/max为本实例生命周期统计，io_total可能包含多个并行等待，不是CPU时间；cpu_ms等未测字段继续null。Python/C++通过同一宿主接收转换/订阅路径获益；本轮Lua旧同步调用锁释放修复不表示Python/C++单语言性能达到了同样倍率。

性能验收：原0.10.49真实game.getRegionBlockData一次1130.55ms，10ms心跳在这段时间运行0次；新版本需要以同样接口真实复测并保存证据。另做连接/确认RTT/队列丢弃样本，不能只用本地空计数循环判断真实服务端完成。

没有改变握手协议ID、加密方式、服务端权限和条件链结果依赖。客户端优化不能使服务端没有广播的包自行出现；当前成果不宣称整体200倍或2000倍，也不通过删字段、停功能或降低事件覆盖率换数据。

方法依据：Go官方性能诊断与CPU profile https://go.dev/doc/diagnostics；fixture CPU样本显示旧snakeCase累计约27.65%，因此优先消除反复转换和分配，而非猜测网络协议。


## 自动投放、错峰与维度借调
参阅fleet-scheduling.md；主名单排除全部受管机器人；无真人停止。控制面可由插件和MCP调用。
