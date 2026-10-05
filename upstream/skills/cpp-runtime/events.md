# events · 事件与抓包

聊天、进退服、死亡、挥臂、实体出现与拾取，以及原始协议订阅。


### `events.captureClear`

```cpp
client.events.captureClear();
```

- 功能、参数、返回与约束：清空当前账号抓包缓冲，保持抓包开关。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureRead`

```cpp
client.events.captureRead(after_sequence, limit);
```

- 功能、参数、返回与约束：after_sequence=0,limit=256，返回记录/游标/覆盖和抽样统计。缓冲有界，单条超大小截为预览；不是手机所有 App 的网络抓包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureStart`

```cpp
client.events.captureStart(config);
```

- 功能、参数、返回与约束：config={types:[包名或*],exclude:[包名],filter:{fields:{...},every_ms:...},capacity:1..2048,max_bytes:512..65536}。开启账号级解码入站游戏抓包；默认血量/死亡/位置/方块包，认证握手不记录，* 默认不包含大区块包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureStop`

```cpp
client.events.captureStop();
```

- 功能、参数、返回与约束：停止记录，保留已抓数据。当前账号的多个插件共用账号级记录；使用时协调抓包开关。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onBlockChange`

```cpp
client.events.onBlockChange(handler);
```

- 功能、参数、返回与约束：handler(event)，UpdateBlock 导出 position/runtime_id/previous/cause_known=false；不能仅从方块更新认定破坏者。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onBotReady`

```cpp
client.events.onBotReady(handlerName);
```

- 作用：注册机器人就绪事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onBotReady()`——0 参。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evBotReady, L)`。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onChat`

```cpp
client.events.onChat(handlerName);
```

- 作用：注册聊天事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onChat(player, msg)`——触发传 2 参：`player` string（玩家名）、`msg` string（聊天内容）。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evChat, L)`；触发 `fireChatEvent` → `runEventHandlers`。实时。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onDeath`

```cpp
client.events.onDeath(handler);
```

- 功能、参数、返回与约束：handler 回调接收一个事件对象，来源为 ActorEventDeath/InstantDeath 或 health<=0；同一已死亡状态去重，重生后可再触发。不推测死亡原因/攻击者。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onEntityRemove`

```cpp
client.events.onEntityRemove(handler);
```

- 功能、参数、返回与约束：handler(event)；RemoveActor下发时触发，含unique_id和已知runtime_id。消失不等于死亡，也可能是离开视距。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onEntitySpawn`

```cpp
client.events.onEntitySpawn(handler);
```

- 功能、参数、返回与约束：handler(event)；AddPlayer/AddActor/AddItemActor下发时触发，含实体类型、位置和原始fields。服务器视距内的出现，不代表世界内首次出生；血量仅取真实同步属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onHealthChange`

```cpp
client.events.onHealthChange(handler);
```

- 功能、参数、返回与约束：handler(event)，仅在观测血量初始值或变化时入队，含 previous/health/max_health/time/source。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onHurt`

```cpp
client.events.onHurt(handler);
```

- 功能、参数、返回与约束：handler(event)，来源为血量下降或 ActorEventHurt，两种独立来源可能各触发一次；无法识别准确攻击者，不用编造 cause。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onItemPickup`

```cpp
client.events.onItemPickup(handler);
```

- 功能、参数、返回与约束：handler(event)；TakeItemActor下发时触发，含拾取实体和掉落物runtime_id、已观测item/position。拾取者可为生物；不凭RemoveActor猜拾取。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onJoin`

```cpp
client.events.onJoin(handlerName);
```

- 作用：注册玩家加入事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onJoin(player)`——1 参 `player` string。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evJoin, L)`；由后台 5 秒玩家列表轮询差量触发（有约几秒延迟）。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onLeave`

```cpp
client.events.onLeave(handlerName);
```

- 作用：注册玩家离开事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onLeave(player)`——1 参 `player` string。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evLeave, L)`；由 5 秒轮询差量触发。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onMove`

```cpp
client.events.onMove(handler);
```

- 功能、参数、返回与约束：handler(event)，服务器实际位置初始值/变化时触发；含 previous_known/position/dimension/source。高频事件可用抓包抽样过滤减少负担。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onPacket`

```cpp
client.events.onPacket(handlerName);
```

- 作用：注册数据包事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onPacket(type, dataTable)`——2 参：`type` string（包类型）、`dataTable` 对象或数组（数据）。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evPacket, L)`；触发 `firePacketEvent`，数据经 `goToLua` 转表。实时。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onRespawn`

```cpp
client.events.onRespawn(handler);
```

- 功能、参数、返回与约束：handler(event)，只处理服务端 RespawnStateReadyToSpawn，清除旧血量等待新属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onSwing`

```cpp
client.events.onSwing(handler);
```

- 功能、参数、返回与约束：handler(event)；服务器AnimateActionSwingArm下发时触发，含runtime_id/player/source。挥臂不证明命中了目标，服务器未广播则无法观测。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.packetSchema`

```cpp
client.events.packetSchema();
```

- 功能、参数、返回与约束：无参数。返回当前内核支持包类型、协议 ID、顶层字段名与类型，供 AI 根据实际协议找包而非猜字段。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribe`

```cpp
client.events.subscribe(packetType, handlerName);
```

- 作用：**通用数据包订阅**。订阅任意一个数据包类型（按 Go 类型名，如 `"MovePlayer"`/`"SetTime"`/`"PlayerList"`/`"Text"`）。订阅后机器人只监听你订阅的包类型，收到就把**完整**数据包广播给该 handler。
- 参数：`packetType` string 必填（包类型名）；`handlerName` string 必填，回调签名 `onData(dataTable)`——1 参：`dataTable` 对象或数组（包数据，字段名 snake_case；`[]byte` 给**完整 hex**，不截断）。
- 返回：无。同插件同类型同 handler 自动去重；重跑上下文跳过注册。
- 报错：参数非 string 抛类型错误；不在插件上下文调用报错。
- 底层：`registerPacketSub` → `packetHubSubscribe`（packet_hub.go）。想查有哪些包类型可订阅，可用 AI 的 `list_packets` 工具。清理：插件停止时 `RemovePluginEvents` 一并退订。
- 示例：

- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribeAll`

```cpp
client.events.subscribeAll(handlerName);
```

- 作用：全量入站包订阅，回调接收一个包信封。
- 参数：`handlerName`；按此顺序传入。
- 返回：Lua无返回；SDK注册成功true；停止自动退订。
- 底层与游戏效果：信封含seq/at/direction/bot/packet_type/packet_id/fields。只序列化有订阅的包；全包模式会含高频及大区块数据，正式插件优先subscribe或subscribeFiltered；队列有界，dropped可查。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribeFiltered`

```cpp
client.events.subscribeFiltered(packet_type, filter, handler);
```

- 功能、参数、返回与约束：packet_type 或 *，filter={fields:{字段路径:精确值},every_ms:0..60000}，handler。Python 回调接收包对象；C++/Lua 使用已注册函数名。* 接收当前账号订阅包，事件字段 _packet_type 表示真实类型，停止自动退订。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。
