# player · 玩家资料与感知

缓存、XUID、权限、背包、位置与已同步血量。未知血量不是20。


### `player.count`

```cpp
client.player.count();
```

- 作用：返回在线玩家数量。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：玩家缓存表当前条数。

### `player.get`

```cpp
client.player.get(name);
```

- 作用：按名字查玩家信息。
- 参数：`name` string 必填。
- 返回：对象或数组 `{name, xuid, unique_id, is_op, online, dimension, x, y, z, held_item?}`（`held_item` 仅非空时出现，为网络 ID）。玩家不存在返回**空表**（不是 nil）。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表，不向游戏发指令。

### `player.getInventory`

```cpp
client.player.getInventory(name);
```

- 作用：走当前活跃机器人查询某玩家背包物品清单。
- 参数：`name` string 必填。
- 返回：对象或数组 `{items = {{name, count, slot, damage}, ...}}`。
- 报错：失败抛 `player.getInventory: <err>`。
- 底层：同 `game.queryInventory`——通过 WS 自动化通道执行网易特有命令 `codebuilder_actorinfo inventory <玩家名>`，从响应 DataSet（JSON）解析物品清单。

### `player.getUniqueID`

```cpp
client.player.getUniqueID(name);
```

- 作用：查玩家实体唯一 ID。
- 参数：`name` string 必填。
- 返回：number；玩家不存在返回 **0**。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表的实体唯一 ID 字段。

### `player.getXUID`

```cpp
client.player.getXUID(name);
```

- 作用：查玩家 XUID。
- 参数：`name` string 必填。
- 返回：string；玩家不存在返回**空串**。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表的 XUID 字段。

### `player.health`

```cpp
client.player.health(target);
```

- 功能、参数、返回与约束：target 为 self、玩家名或 runtime_id 字符串，默认 self。返回已观测信息含 health_known/health/max_health/health_time/stale，未知为 null，不推算其他玩家满血。仅能获取服务器给本机器人同步的属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `player.isOP`

```cpp
client.player.isOP(name);
```

- 作用：判断玩家是否 OP。
- 参数：`name` string 必填。
- 返回：boolean（玩家存在且为 OP 才为 true，含不存在时返回 false）。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表（`IsOP` 由 `UpdateAbilities`/`AddPlayer` 等数据包维护）。

### `player.list`

```cpp
client.player.list();
```

- 作用：列出全部在线玩家信息表。
- 参数：无。
- 返回：对象或数组（1 起索引，每个元素是 `player.get` 同款信息表）。
- 报错：无。
- 底层：遍历玩家缓存表。

### `player.nearby`

```cpp
client.player.nearby(radius);
```

- 功能、参数、返回与约束：radius=0..512，默认 32；按机器人已观测位置计算同维度、仍存在的玩家距离并排序。不包括客户端不可见或未知坐标的玩家，机器人坐标未知报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `player.position`

```cpp
client.player.position(target);
```

- 功能、参数、返回与约束：target 同 player.health。返回 position_known/position/position_time/dimension/stale；坐标为收到服务器包的观测，不把本地移动请求当服务器确认。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `player.equipment`

name或runtime_id；head/chest/legs/feet/main_hand/off_hand/body，每槽含known/item_id/item_id_known/network_id/count/enchantments/enchantments_known/nbt/received_at/age_ms/stale。物品名来自本会话ItemRegistry；附魔来自NBT。未知槽或未同步附魔不编造。expires_at=null：装备包没有物品失效时间。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `player.inventory`

target=self,window_id=0；读取本机器人实际收到的InventoryContent/InventorySlot，不做同步背包查询。其他玩家背包服务器通常不广播，返回known=false；不把装备冒充整个背包。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `player.rotation`

name或runtime_id；pitch/yaw/head_yaw各自known、updated_at、source/stale。MovePlayer及Absolute/Delta真实角度，旋转无位移也更新。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。
