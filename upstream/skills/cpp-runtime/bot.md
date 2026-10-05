# bot · 机器人控制与路由

选择舰队机器人、状态、输入移动、看向坐标、近战攻击。无自动寻路保证。


### `bot.attack`

```cpp
client.bot.attack(runtime_id, slot);
```

- 功能、参数、返回与约束：runtime_id 字符串,slot=0；真实近战库存交易，只允许当前服务器观测的六格内实体；slot 为 0..8。返回 submitted，伤害必须由服务器事件确认。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.count`

```cpp
client.bot.count();
```

- 作用：返回在线机器人数量。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：`GetFleetBots()`。

### `bot.current`

```cpp
client.bot.current();
```

- 作用：返回当前活跃机器人的名字与坐标。
- 参数：无。
- 返回：对象或数组 `{name, x, y, z}`；无活跃机器人返回空表；坐标查询失败省略 x/y/z。
- 报错：无。
- 底层：从舰队管理器读当前选中的机器人名，再通过 WS 自动化通道执行 `querytarget @s` 取该机器人的坐标。

### `bot.list`

```cpp
client.bot.list();
```

- 作用：列出在线机器人及坐标。
- 参数：无。
- 返回：对象或数组，1 起索引，每个元素 `{name, x, y, z}`（坐标查询失败省略坐标字段）。
- 报错：无。
- 底层：`GetFleetBots()` 遍历 + `QueryPosition()`。

### `bot.lookAt`

```cpp
client.bot.lookAt(x, y, z);
```

- 功能、参数、返回与约束：x,y,z 世界坐标。由已观测机器人位置计算 yaw/pitch，启用心跳发送视角；返回请求状态，无自动寻路/穿墙保证。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.select`

```cpp
client.bot.select(nameOrIndex);
```

- 作用：切换后续 `game.*` / `world.*` 调用的目标机器人。
- 参数：`nameOrIndex` string 必填（机器人名或索引的字符串形式）。
- 返回：boolean `true`。
- 报错：未找到机器人抛 `bot.select: 未找到机器人 <名称>`。
- 底层：`selectBot(L, sel)`。

### `bot.self`

```cpp
client.bot.self();
```

- 功能、参数、返回与约束：无参数。读取当前账号机器人的服务器观测状态，health/position/dead 可未知。AI Agent 使用此机器人，不自行拥有一个游戏位置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.startMovement`

```cpp
client.bot.startMovement(dx, dy, dz);
```

- 功能、参数、返回与约束：dx,dy,dz 玩家输入方向，每轴 -1..1；以 50ms PlayerAuthInput 心跳交给服务器驱动运动。dx 左右，dz=-1 前进，dy 上下，不是世界坐标路径规划；返回 submitted=true,confirmed=false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.status`

```cpp
client.bot.status();
```

- 作用：返回每个机器人的连接状态、是否 OP、错误信息。
- 参数：无。
- 返回：对象或数组，1 起索引，每个元素 `{index, name, connected, is_op, error?}`（error 仅非空时存在）。
- 报错：无。
- 底层：`FleetStatus()`。

### `bot.stopMovement`

```cpp
client.bot.stopMovement();
```

- 功能、参数、返回与约束：无参数。清理当前机器人移动输入，仍需要服务器回包确认最终位置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.teleport`

```cpp
client.bot.teleport(x, y, z);
```

- 功能、参数、返回与约束：x,y,z 世界坐标。执行服务器 tp @s 并等待真实命令结果，需要对应权限；没有权限时报错或 success=false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
