# observerBridge：玩家输入与独立来源

### `observerBridge.events`

签名：observerBridge.events(after=0, limit=256)。返回events/cursor/head_cursor/overwritten，读取后保存cursor。字段、来源和例子如下。

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



## 0.10.41 身体跟随的重复TP优化

watchStart增加follow_radius_blocks，默认16；follow/hybrid首次仍提交一次TP以建立区域，之后只有自身服务器坐标未知/过期、跨维度或离开半径才再次TP。可设置0恢复每轮TP；半径不是动作广播保证。camera方式不使用身体距离判断。配置只影响观察原语，未改变Lua/命令链Hz、Fast发送或插件业务逻辑。watchStatus增加follow_teleports、follow_skipped_near；最后提交时间只在确实发送命令时更新。

原watch仍依赖当前机器人能执行querytarget。OP标识不保证这个命令可用；实际测试中部分子账号收到权限1但查询失败。主账号可用自己的合法游戏权限给自己账号池内的其他机器人设置相机，实现集中指挥，不必让每个节点独立查询。普通用户调用自己的池，不能借此控制其他用户或普通玩家。该方式本轮固定目标四观察节点各45/45，20位置连续移动各7/86，仍不能宣称全局连续覆盖。

Lua: observerBridge.watchStart({player="目标名",mode="hybrid",interval_seconds=0.25,follow_radius_blocks=16})
Python: host.observerBridge.watchStart({"player":"目标名","mode":"hybrid","interval_seconds":0.25,"follow_radius_blocks":16})
C++: client.observerBridge.watchStart(yy::json{{"player","目标名"},{"mode","hybrid"},{"interval_seconds",0.25},{"follow_radius_blocks",16}});
MCP游戏工具body: {"method":"observerBridge.watchStart","args":[{"player":"目标名","mode":"hybrid","interval_seconds":0.25,"follow_radius_blocks":16}]}。请按实际验收报告区分源码优化与游戏改善，不把少发TP当已实现全局监听。


## 0.10.41 观察原语方法索引

### `observerBridge.watchStart`

options={player,mode:camera|follow|hybrid,interval_seconds:2,duration_seconds:0,restore_position:true,follow_radius_blocks:16}。需本账号实际OP和可用querytarget。follow/hybrid首次TP，此后未知/过期位置、跨维度或超半径才TP；0每轮TP。camera不移动身体、不能跨维度。慢查询合并，不阻塞接包线程；不保证全局。

Lua observerBridge.watchStart；Python host.observerBridge.watchStart；C++ client.observerBridge.watchStart；MCP POST游戏工具body={method:"observerBridge.watchStart",args:[对应参数]}。

### `observerBridge.watchStatus`

无参数。返回active/config/submitted、follow_teleports/follow_skipped_near、missed_intervals、target_position、target_since、region_updated、region_timestamp、error/cleanup。区域发布不等于动作包齐全。

Lua observerBridge.watchStatus；Python host.observerBridge.watchStatus；C++ client.observerBridge.watchStatus；MCP POST游戏工具body={method:"observerBridge.watchStatus",args:[对应参数]}。

### `observerBridge.watchStop`

无参数。停止本账号自己的观察，清相机并按配置恢复位置；插件不能停其他插件拥有的任务。清理submitted/confirmed分开，默认不会停游戏连接。

Lua observerBridge.watchStop；Python host.observerBridge.watchStop；C++ client.observerBridge.watchStop；MCP POST游戏工具body={method:"observerBridge.watchStop",args:[对应参数]}。


## 摄像机原理与固定远距验收（2026-10-12）

watchStart的camera模式发送的是服务器原版 `camera "机器人名" set minecraft:free pos X Y Z` 指令，由独立工作线程查询目标坐标并更新自由摄像机；不是TP、客户端作弊视角或隐藏数据通道。follow/hybrid才会移动机器人身体。camera模式需要调用者能执行camera/querytarget，不能把接收者无OP等同无人拥有指令权限；主控可用合法OP权限给本用户账号池内的非OP接收者下发camera，接收者本身无需OP。

现代官方基岩版命令资料列出attach_to_entity（摄像机位置附着随实体移动）和targetEntity（固定位置持续看向实体）。当前网易山头服实测attach_to_entity及驼峰targetEntity均返回commands.generic.syntax；下划线target_entity与free pos ... facing "玩家名"成功。target_entity只持续追踪朝向，不能把它包装成已实现原生位置跟随。没有验证支持的功能不自动使用，不安装玩法组件，不对普通玩家修改摄像机。

本轮2026-10-12 18:56:14–18:57:47北京时间，儒雅猎杀者与火神靠谱军团身体固定相隔14450.755格，两个测试账号自身UpdateAbilities.command_permissions均为0，主控布汲岛只下指令，其接收不计分。暂停业务插件，禁止TP/teleport/含execute的测试命令，反复querytarget确认双方位置：所有采样身体位移为0。未设相机0/30；仅设置一次远处free相机30/30；target_entity20/20（目标固定）；facing选择器20/20；清相机后0/20。抓包覆盖0，捕获的是接收账号的服务器下行Animate，不是发送账号自己的出包或主控收包。

这是受控普通权限协议账号的固定距离证据，不等于未修改手机真人、多玩家移动、跨维度或长期零漏包。前面的20位置测试每秒TP256格属于移动压力试验，不能混作身体固定的远距证明。连续移动相机仍大量漏包，调高Hz没有被证明能消除此问题。测试后清相机、恢复原权限/配置并停临时账号，原业务插件恢复；此脚本不随软件作为默认运行插件。

固定距离验收：先暂停会跟随/传送的插件及任务，确认原语已停止；如果停止操作会恢复位置，必须在测量窗口之外完成。记录测试双方身份、UUID/UniqueID、原始权限包、维度与开始/结束坐标；只统计明确对应发送实体ID的下行包，不能把多个观察者的重复包相加。无OP且不主动装客户端的手机真人仍需单独验收。


## 0.10.42 导出下载中心与可配置镜头

Linux/网页插件列表→分享→导出：生成完整私有ZIP后直接下载，并保存到“导出下载中心”。中心列出文件名、日期、大小、SHA256，可重复下载、复制中心网址或只删除备份。下载中心网址为 `https://toolbox.youyuanqi.dpdns.org/?device=linux-server&exports=1`；需登录并有该执行节点权限。子账号使用 `export_account=账号ID`，由账号池校验是否本人拥有；不能通过改URL控制别人的账号。备份存在执行设备的“插件导出/”，不是原作者服务器或公开市场。每次导出独立文件，不覆盖旧备份；Lua包含插件文件/配置/数据的完整目录，Python/C++包含整个插件目录和资源。

MCP：POST `/api/plugin/export` body `{id:"插件ID"}`；Python/C++用POST `/api/youyuan/runtime/export` query `{id:"插件ID"}`。响应有export_id、export、download_url，原path/src_path保留给APK系统选择器。GET `/api/youyuan/exports`列记录；GET `/api/youyuan/exports/info?id=导出ID`读元信息；GET `/api/youyuan/exports/download?id=导出ID`由登录后的网页下载ZIP；POST `/api/youyuan/exports/remove` body `{id:"导出ID"}`只删除该备份。MCP调用download返回经权限验证的元信息且downloaded=false，不把ZIP二进制塞进AI上下文。删除需要控制权限。备份包含自己的配置和私有数据，请按需私下保管。未导出的历史磁盘ZIP不会假装已有下载记录；重新点导出即可。

Lua/Python/C++共享watchStart新增配置：camera_height_blocks（脚部坐标上方高差，默认2），camera_pitch_degrees（留空保持原行为；-90向上、90向下），camera_yaw_degrees（默认0），camera_facing_player（默认false；每次free更新时facing玩家；与固定俯仰二选一）。例如camera模式高度8、俯仰90，从玩家上方俯视；只改镜头，不移动身体。高度/俯仰不承诺扩大服务器动作广播或减少漏包。原生target_entity仍只追踪朝向，当前网易服尚不支持已测的attach_to_entity。先接替再释放、错峰规划、账号池和模块通信保留；区域就绪不等于目标事件齐全。

本轮对照须分别统计：固定低镜头/高镜头俯视、多人分区常驻/单节点切换，以及跨维度。接收与发送账号自己的权限为0；有OP主控只下指令，其收包不算普通权限结果。明确身体坐标、维度、测量时间和交集重复；受控正常权限协议账号是有效工程样本，但没有自动包含手机UI/输入协议的全部差异。


## 0.10.45 观测与低延迟

# 0.10.45 观测协作、战斗与容器接口

## 宿主三语言共用接口

`world.containerState(x,y,z)`读取当前维度真实BlockActorData类型缓存：known/is_container/block_entity_id/source/timestamp/stale/dimension/position。不读取箱内物品，不做同步WS查询。UpdateBlock替换后旧容器类型立即未知；新的BlockActorData再次建立。`events.onContainerChange(handler)`为对应规范事件。变化不等于确证某个玩家放置，没有来源时cause_known=false。

原`game.getBlockTileRaw`保留，新`game.getBlockTileAsync`后台真实查询，commandResult(ticket,0,true)消费。适合没有容器类型包时的核验，不在回调里等待。Fast清除只是submitted，不等于服务器确认已清掉，须回读更新包/Confirmed查询验收。

`game.sceneStats()`只计数，不复制或新留存全部实体明细。`game.scene({present_only=true,entity_type="minecraft:player"})`是可选过滤；无参数完整旧接口保持兼容。查一只实体用entities.get(runtimeId)，不要为查一只血量遍历整个场景。

`observerBridge.worldEvents({stream="actions",cursors=previous,limit=1024})`从独立动作历史读取：普通实体位移和非玩家旋转不会挤掉伤害/血量/挥臂等。limit可1..4096，history容量继续由diagnostics.configure(event_history)设置。默认stream=all保持旧行为。actions的lost表示发生覆盖（lost_count_known=false），不能拿全局sequence的位移间隙计算动作丢失数量。cursors按观察账号和会话保存，重连reset；没有广播的包仍未知。

`plugin.call(pluginID,exportedFunction,args数组)`只调用本账号已运行插件plugin_api的导出函数。未运行不隐式启动，私有全局不开放；忙/递归返回明确错误。Lua/Python同名，C++为client.plugin.call。MCP/HTTP：POST /api/youyuan/game/call {method:"plugin.call",args:["插件ID","函数名",[参数]]}。跨账号继续用原accounts.callPlugin/accountsPool.invoke，隔离与模块通信保留。

## 本次多功能插件的可调用接口（需要模块observation_team.lua）

```lua
local config={enabled=true,accounts={"自己的池账号ID"},targets={"目标玩家名"},
 mode="hybrid",interval_seconds=0.2,camera_height_blocks=4,camera_pitch_degrees=90,
 follow_radius_blocks=16,dwell_seconds=2,idle_seconds=60,event_limit=1024,start_accounts=true}
plugin_api.observationTeamConfigure(config)
plugin_api.observationTeamStatus()
plugin_api.observationTeamStop()
-- 空闲机器人先到位、收到新区域发布，才释放旧观察者。
plugin_api.observationTeamHandoff(incomingID,outgoingID,{player="目标",mode="hybrid",
 camera_height_blocks=4,camera_pitch_degrees=90,interval_seconds=0.2,timeout_seconds=5})
```

同一账号持续跟随一个目标，人数超过观察者数量时分时轮巡，切换错峰；有空闲接替者时用真实handoff，超时保持旧观察。账号不足/无备用时存在覆盖空档，不承诺每0.05秒全服全部玩家都收到，也不承诺不丢包。相机只是当前维度镜头；hybrid近距身体跟随可以跨维度，需要正常游戏权限。玩家不会被移动；真实测试应明确观察者/发送者OP状态与测量中是否TP。静默超时release按用户自己的池回收，不删除游戏凭据。凭据失效要真实重新认证，不能靠界面伪装成功。

动作汇总只接受已连接观察者的server_to_bot_decoded事件，按服务器/维度/RuntimeID分组，不把他人账号输入假装普通玩家广播。组合方法扩展视距覆盖范围，但不是全局引擎监听。

## 战斗显示语义

18格内近期挥臂或已知射击活动与真实掉血相关联；多名候选不强行取最近玩家。单候选也是关联推断，不能当PvP击杀确证。只有匹配玩家显示，取消向256格旁观者群发。血量和最大值来自属性；护甲未知显示未知，不用未知装备推算真实护甲。一个combat-health管理频道，1200ms到期清除/恢复低优先级频道，不反复播放Title。相同血量的重复原始/规范事件合并；重新回血再受伤仍可显示。

真实屏幕延迟、多人跨维度丢包率和连续放置回读仍要单独验收；源码模拟通过不等于游戏验收。界面保存区不浮动；导航透镜不再采样背景文字，浏览器控制超时仍需视觉复验。


# 三维度常驻观察与服务端相对镜头

全局抓包默认auto_players=true，不必逐人添加。默认dimension_routing=true；dimension_accounts={"0":"自己账号ID","1":"自己账号ID","2":"自己账号ID"}配置主世界/下界/末地，三个ID必须不同且在accounts中。不指定映射时按accounts顺序分配0/1/2，账号不足明确缺少维度，不自动宣称覆盖。

observerBridge.assignDimension({dimension=0})：只控制当前机器人。需OP，关闭其auto_creative并按实际game_mode=6维护旁观；身体维度不匹配才正常TP到该维度安全锚点20/120/20。空间导入导出作业暂停角色维护。ready来自实际维度与模式，submitted只代表请求。调用不会给其他玩家授予OP；权限要由服主先正常授予。
observerBridge.dimensionStatus()：真实连接、维度、game_mode、is_op。

watchStart参数新增observe_dimension=0/1/2、camera_relative=true、camera_follow_rotation=true、camera_back_blocks=5。镜头使用execute as观察机器人 at @s at @a[name="目标",rm=0] rotated as @a[...] run camera @s set minecraft:free pos ^ ^4 ^-5 rot 90 ~。rm=0按本服务器实测只匹配当前执行维度；外维度不会改变观察机器人的镜头。依旧需要周期执行；camera set/ease不是自动实体附着。身体维度不符时观察等待分工恢复，不跨维度伪造包。

Lua：observerBridge.assignDimension({dimension=1})；Python：host.observerBridge.assignDimension({"dimension":1})；C++：client.observerBridge.assignDimension(yy::json{{"dimension",1}})。三语言共享同一宿主语义。global配置API=/api/youyuan/global-capture，当前用户只选自己的账号。

2026-10-14测试说明：最初三维度10/10与单机46/60记录用了实体缓存判断维度；复验发现缓存与连接层维度冲突，因此这些维度归属结果撤销验收，原始记录保留。严格复验固定接收者在下界、无OP发送者在主世界，原生同维度半径选择器返回falseCondition；两个开镜头窗口均收到目标挥臂0/10，两个关闭窗口也是0/10。摄像机指令成功不能证明跨维度收包。默认保留三机器人分工，尚不承诺任意真人、全服零丢包或单机快速切维度替代。角色常驻与同维度动作复验另见交付报告。

0.10.56：低频角色维护用服务器确认命令，随后读取连接层真实维度和模式；高频镜头仍只提交不等待回包。bot.self的自身维度改读连接层Spawn/ChangeDimension，避免实体缓存误报；其他实体继续保留实际收到的数据。不匹配的维度不能显示ready。
