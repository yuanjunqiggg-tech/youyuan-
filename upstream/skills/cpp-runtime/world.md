# world · 方块与物理操作

查询、setblock、真实挖掘和手持物品点击。权限替换方块与生存挖掘是两种机制。


### `world.block`

```cpp
client.world.block(x, y, z);
```

- 作用：查询世界某坐标的方块名（需要机器人连接，目标为当前活跃机器人）。
- 参数：`x`/`y`/`z` number 必填（整数坐标）。
- 返回：string（方块名）。
- 报错：查询失败抛 `world.block: <err>`；参数非整数抛类型错误。
- 底层：同 `game.getBlockTile`——机器人先传送加载区块，再执行 `testforblock <x> <y> <z> air` 判断方块名。**有移动副作用**（会把当前活跃机器人传送过去再传回）。

### `world.blockState`

```cpp
client.world.blockState(x, y, z);
```

- 功能、参数、返回与约束：x,y,z 整数坐标。返回当前维度 layer=0 的最近 UpdateBlock 观测，未知 known=false；包含方块 runtime_id，不把任何更新都认定是玩家破坏。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.breakBlock`

```cpp
client.world.breakBlock(x, y, z, mode);
```

- 功能、参数、返回与约束：x,y,z 整数坐标，mode=command 或 creative_action，默认 command。command 用 setblock ... air destroy 等待实际结果；creative_action 发创造模式破坏动作，要求游戏模式和六格范围，不代表服务器确认。生存挖掘的装备/耗时/交易流程尚不支持。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.mineBlock`

```cpp
client.world.mineBlock(x, y, z, duration_ms, slot, face);
```

- 功能、参数、返回与约束：x,y,z,duration_ms=1000,slot=0,face=1；生存挖掘尝试，真实开始/持续/结束动作与手持库存交易，按 100ms 发送。需要最近方块观测；服务端可拒绝，返回 submitted/confirmed。硬度、工具正确与否由服务器判断。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.placeBlock`

```cpp
client.world.placeBlock(x, y, z, block);
```

- 功能、参数、返回与约束：x,y,z 整数坐标，block 例如 minecraft:stone。用服务器 setblock 指令，返回真实结果，需要权限。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.useBlock`

```cpp
client.world.useBlock(x, y, z, face, slot);
```

- 功能、参数、返回与约束：x,y,z,face=1,slot=0；真实手持物品点击指定方块面（可放置、开容器），需要已观测目标方块和六格范围。不是 setblock；返回提交状态，消耗与放置观察服务器。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
