# events · 事件与抓包

聊天、进退服、死亡、挥臂、实体出现与拾取，以及原始协议订阅。


### `events.captureClear`

```python
host.events.captureClear()
```

- 功能、参数、返回与约束：清空当前账号抓包缓冲，保持抓包开关。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureRead`

```python
host.events.captureRead(after_sequence, limit)
```

- 功能、参数、返回与约束：after_sequence=0,limit=256，返回记录/游标/覆盖和抽样统计。缓冲有界，单条超大小截为预览；不是手机所有 App 的网络抓包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureStart`

```python
host.events.captureStart(config)
```

- 功能、参数、返回与约束：config={types:[包名或*],exclude:[包名],filter:{fields:{...},every_ms:...},capacity:1..2048,max_bytes:512..65536}。开启账号级解码入站游戏抓包；默认血量/死亡/位置/方块包，认证握手不记录，* 默认不包含大区块包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureStop`

```python
host.events.captureStop()
```

- 功能、参数、返回与约束：停止记录，保留已抓数据。当前账号的多个插件共用账号级记录；使用时协调抓包开关。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onBlockChange`

```python
host.events.onBlockChange(handler)
```

- 功能、参数、返回与约束：handler(event)，UpdateBlock 导出 position/runtime_id/previous/cause_known=false；不能仅从方块更新认定破坏者。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onBotReady`

```python
host.events.onBotReady(handlerName)
```

- 作用：注册机器人就绪事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onBotReady()`——0 参。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evBotReady, L)`。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onChat`

```python
host.events.onChat(handlerName)
```

- 作用：注册聊天事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onChat(player, msg)`——触发传 2 参：`player` string（玩家名）、`msg` string（聊天内容）。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evChat, L)`；触发 `fireChatEvent` → `runEventHandlers`。实时。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onDeath`

```python
host.events.onDeath(handler)
```

- 功能、参数、返回与约束：handler 回调接收一个事件对象，来源为 ActorEventDeath/InstantDeath 或 health<=0；同一已死亡状态去重，重生后可再触发。不推测死亡原因/攻击者。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onEntityRemove`

```python
host.events.onEntityRemove(handler)
```

- 功能、参数、返回与约束：handler(event)；RemoveActor下发时触发，含unique_id和已知runtime_id。消失不等于死亡，也可能是离开视距。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onEntitySpawn`

```python
host.events.onEntitySpawn(handler)
```

- 功能、参数、返回与约束：handler(event)；AddPlayer/AddActor/AddItemActor下发时触发，含实体类型、位置和原始fields。服务器视距内的出现，不代表世界内首次出生；血量仅取真实同步属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onHealthChange`

```python
host.events.onHealthChange(handler)
```

- 功能、参数、返回与约束：handler(event)，仅在观测血量初始值或变化时入队，含 previous/health/max_health/time/source。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onHurt`

handler(event)；血量下降优先；ActorEvent等50ms关联属性包，否则提供damage_known=false提示；hurt_signal仅供诊断；未知攻击者attacker_known=false。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onItemPickup`

```python
host.events.onItemPickup(handler)
```

- 功能、参数、返回与约束：handler(event)；TakeItemActor下发时触发，含拾取实体和掉落物runtime_id、已观测item/position。拾取者可为生物；不凭RemoveActor猜拾取。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onJoin`

```python
host.events.onJoin(handlerName)
```

- 作用：注册玩家加入事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onJoin(player)`——1 参 `player` string。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evJoin, L)`；由后台 5 秒玩家列表轮询差量触发（有约几秒延迟）。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onLeave`

```python
host.events.onLeave(handlerName)
```

- 作用：注册玩家离开事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onLeave(player)`——1 参 `player` string。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evLeave, L)`；由 5 秒轮询差量触发。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onMove`

```python
host.events.onMove(handler)
```

- 功能、参数、返回与约束：handler(event)，服务器实际位置初始值/变化时触发；含 previous_known/position/dimension/source。高频事件可用抓包抽样过滤减少负担。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onPacket`

```python
host.events.onPacket(handlerName)
```

- 作用：注册数据包事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onPacket(type, dataTable)`——2 参：`type` string（包类型）、`dataTable` 对象或数组（数据）。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evPacket, L)`；触发 `firePacketEvent`，数据经 `goToLua` 转表。实时。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onPlayerDeath`

```python
host.events.onPlayerDeath(handler)
```

- 功能、参数、返回与约束：handler(event)；只接受玩家死亡；来自真实ActorEvent/血量归零/网易ModEventS2C OnPlayerDie，附已知最后位置、维度、timestamp；没有位置保持position_known=false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onRespawn`

```python
host.events.onRespawn(handler)
```

- 功能、参数、返回与约束：handler(event)，只处理服务端 RespawnStateReadyToSpawn，清除旧血量等待新属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onSwing`

```python
host.events.onSwing(handler)
```

- 功能、参数、返回与约束：handler(event)；服务器AnimateActionSwingArm下发时触发，含runtime_id/player/source。挥臂不证明命中了目标，服务器未广播则无法观测。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.packetSchema`

```python
host.events.packetSchema()
```

- 功能、参数、返回与约束：无参数。返回当前内核支持包类型、协议 ID、顶层字段名与类型，供 AI 根据实际协议找包而非猜字段。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribe`

```python
host.events.subscribe(packetType, handlerName)
```

- 作用：**通用数据包订阅**。订阅任意一个数据包类型（按 Go 类型名，如 `"MovePlayer"`/`"SetTime"`/`"PlayerList"`/`"Text"`）。订阅后机器人只监听你订阅的包类型，收到就把**完整**数据包广播给该 handler。
- 参数：`packetType` string 必填（包类型名）；`handlerName` string 必填，回调签名 `onData(dataTable)`——1 参：`dataTable` 对象或数组（包数据，字段名 snake_case；`[]byte` 给**完整 hex**，不截断）。
- 返回：无。同插件同类型同 handler 自动去重；重跑上下文跳过注册。
- 报错：参数非 string 抛类型错误；不在插件上下文调用报错。
- 底层：`registerPacketSub` → `packetHubSubscribe`（packet_hub.go）。想查有哪些包类型可订阅，可用 AI 的 `list_packets` 工具。清理：插件停止时 `RemovePluginEvents` 一并退订。
- 示例：

- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribeAll`

```python
host.events.subscribeAll(handlerName)
```

- 作用：全量入站包订阅，回调接收一个包信封。
- 参数：`handlerName`；按此顺序传入。
- 返回：Lua无返回；SDK注册成功true；停止自动退订。
- 底层与游戏效果：信封含seq/at/direction/bot/packet_type/packet_id/fields。只序列化有订阅的包；全包模式会含高频及大区块数据，正式插件优先subscribe或subscribeFiltered；队列有界，dropped可查。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribeFiltered`

```python
host.events.subscribeFiltered(packet_type, filter, handler)
```

- 功能、参数、返回与约束：packet_type 或 *，filter={fields:{字段路径:精确值},every_ms:0..60000}，handler。Python 回调接收包对象；C++/Lua 使用已注册函数名。* 接收当前账号订阅包，事件字段 _packet_type 表示真实类型，停止自动退订。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onDimensionChange`

handler(event)；服务器ChangeDimension，旧维度实体数据标为过期。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onEntityHurt`

handler(event)；真实掉血优先；ActorEventHurt等50ms关联UpdateAttributes，未同步血量时damage_known=false；hurt_signal仅诊断，不计作另一伤害。含target、damage/health/max_health、attacker_known=false（无可靠攻击者包时），event_id。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onEquipmentChanged`

handler(event)；服务器装备包即时缓存更新，含已更新槽位，不同步查询玩家背包。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onHealthChanged`

同onHealthChange，含previous及真实max，未知max=null。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onPlayerKill`

handler(event)；可信服务器death.attack翻译包提供击杀者且关联已知玩家，唯一death event_id，仅触发一次；同次death_details补充不重复计分。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onPlayerRespawn`

handler(event)；与session_join独立，死亡到存活/RespawnReady的生命周期去重。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onRotation`

handler(event)；服务器角度字段变化。旋转包即使位置不变也刷新时间。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onSessionJoin`

handler(event)；真实PlayerList新增条目，不能把AddPlayer视距出现当进服。服务器不广播PlayerList则无法直接观测。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onSessionLeave`

handler(event)；真实PlayerList移除条目，不把RemoveActor离开视距当退服。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。


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
