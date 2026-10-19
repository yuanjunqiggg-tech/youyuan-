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
