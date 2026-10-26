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

