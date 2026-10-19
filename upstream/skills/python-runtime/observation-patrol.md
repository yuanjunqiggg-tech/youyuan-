## 0.10.35 统一观测事件、可选轮巡与账号令牌

观测事件由宿主解析真实服务器下发包，Lua、Python、C++不用先开启抓包。`events.catalog()`列出类型；`events.on(kind,handler)`按类型订阅；`events.onObservedAction(handler)`合并接收swing、hurt、equipment、rotation、entity_kill、damage_display，保留原type和source。伤害事件不等于空挥，未知攻击者保持未知。模块广播不一定存在于其他租赁服。事件缓存/历史、收包队列容量可以通过diagnostics配置，但扩大留存不能让服务器广播未发送的数据。

Lua（函数或函数名）：
```lua
events.on("swing", function(e)
  -- 真Animate挥臂；不证明命中了生物
  print(e.player, e.runtime_id, e.timestamp, e.source)
end)
events.onHealthChanged(function(e)
  print(e.runtime_id, e.previous, e.health, e.max_health)
end)
events.onObservedAction(function(e) print(e.type, e.source) end)
```
Python：`events.on("swing", handler)`，`events.onHealthChanged(handler)`；C++：`on("handler", callback)`注册函数后，`client.events.on("swing","handler")`。常驻运行器投递到插件自己的回调队列；不重跑源码。停止/重载插件退订。高频回调避免逐条写盘或同步HTTP。

### 插件私有自定义事件格式
`events.define(definition,handler)`：同一格式三语言可用，映射订阅属于此插件，停止后删除，不污染其他插件事件，不执行自定义代码。字段路径使用`packet.describe()`提供的真实snake_case字段。此版字段路径支持嵌套对象，不支持数组下标，血量请直接用宿主health事件。
```lua
events.define({name="arm_seen", packet="Animate",
  filter={fields={action_type=1}, every_ms=0},
  fields={runtime_id="entity_runtime_id", action="action_type"}},
  function(e) print(e.runtime_id, e.action) end)
```
输出含`source="custom_packet_mapping"`、`packet_type`、`timestamp`、`event_id`、`observed=true`、`inferred=false`、`server_verified=false`。最后一项表示自定义语义未由宿主认证：收到字段不能证明玩家命中、全局广播或服务器确认了操作。缺失映射字段为nil/null。限制私密握手、大区块、通配包映射以保护敏感凭证和内存；它不是额外游戏命令CPS限制。

### 单机器人顺序观察
```lua
local r = observerBridge.patrolStart({players={"玩家甲","玩家乙"},
  interval_seconds=0.001, single_follow_seconds=2,
  duration_seconds=60, restore_position=true})
-- 一次建立专用目标与分数甲=1、乙=2；逐个tp @s到分数1、2、1、2
local s = observerBridge.patrolStatus()
-- 在不需要时主动停止；插件卸载自动取消自己的轮巡
observerBridge.patrolStop()
```
必须当前机器人OP且目标真实在线。空players取当前在线列表并排除自己；默认未开启。固定路线只在开启时分配一次编号；新增/离开玩家后应停止并重开来刷新路线。编号只保存在本次专用计分板，不覆盖玩家其他积分。每次新建独立目标，停止时提交移除。循环TP使用Fast路径，不等每条回包。单玩家按single_follow_seconds执行，不在1ms重复堆发。间隔允许更细的正值，系统调度器和服务器Tick仍是实际限制；调度超时合并遗漏间隔，不无限追补积压。

`patrolStatus`区分submitted（本机写入）、rounds（路线轮数）、missed_intervals（宿主错过计划间隔）、position_updates（真实位置事件）、by_player（观测事件），最后位置附时间。位置包可合并/重复，不能把position_updates称为真实TP完成次数；`actual_tp_rate_known=false`。停止/断线取消发送，duration_seconds自动停止；restore_position默认请求恢复起点。清理是无等待提交，是否生效要再读位置/计分板结果。重复启动被拒绝直到之前清理结束。

单机器人轮巡是时分采样：并非连续同时覆盖所有玩家。更高Hz不一定更高事件命中率，过快TP可能还没同步周围实体便离开。只有实际近处收到的动作才计入观测；没有收到就不补造。约48格是当前测试假设，不是宿主硬编码限制或所有服务器固定上限。

### 多机器人协作
已有accountsPool.acquire/bind/start/invoke/release供插件管理自己的账号池；管理员all=true可按权限管理全部账号。每个机器人只能一个实际游戏会话，不能重复同一Token。给每个账号选择目标后，`accountsPool.invoke`调用该账号`/api/youyuan/game/call`，method为patrolStart，players只填对应玩家，single_follow_seconds按需配置。退出等待/再分配由业务插件决定；release支持idle_seconds默认60。`observerBridge.coverage/health/worldEvents`按需汇总各真实连接观测者，游标避免重复读取；global_coverage_guaranteed一直为false，缺乏观测的动作不伪造。

### 令牌查看复制
账号卡片“复制令牌”；编辑页“显示/隐藏Token”“复制Token”；云端游戏账号“查看/复制令牌”。增加游园独立验证服务选项，避免云端新账号默认地址选不上。接口POST `/api/youyuan/accounts` `{action:"token",id:"账号ID"}`：仅原有账号授权者控制范围，read授权拒绝、普通用户他人账号拒绝，响应no-store。常规GET列表继续不带Token。云端账号沿用本人token签发接口，发起需要平台会话；不读他人账号凭证。剪贴板被浏览器拒绝时提供可选中文本框手动复制。

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

