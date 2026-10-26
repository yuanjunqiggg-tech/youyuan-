# player · 玩家资料与感知

缓存、XUID、权限、背包、位置与已同步血量。未知血量不是20。


### `player.count`

```lua
player.count()
```

- 作用：返回在线玩家数量。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：玩家缓存表当前条数。

### `player.get`

```lua
player.get(name)
```

- 作用：按名字查玩家信息。
- 参数：`name` string 必填。
- 返回：table `{name, xuid, unique_id, is_op, online, dimension, x, y, z, held_item?}`（`held_item` 仅非空时出现，为网络 ID）。玩家不存在返回**空表**（不是 nil）。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表，不向游戏发指令。

### `player.getInventory`

```lua
player.getInventory(name)
```

- 作用：走当前活跃机器人查询某玩家背包物品清单。
- 参数：`name` string 必填。
- 返回：table `{items = {{name, count, slot, damage}, ...}}`。
- 报错：失败抛 `player.getInventory: <err>`。
- 底层：同 `game.queryInventory`——通过 WS 自动化通道执行网易特有命令 `codebuilder_actorinfo inventory <玩家名>`，从响应 DataSet（JSON）解析物品清单。

### `player.getUniqueID`

```lua
player.getUniqueID(name)
```

- 作用：查玩家实体唯一 ID。
- 参数：`name` string 必填。
- 返回：number；玩家不存在返回 **0**。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表的实体唯一 ID 字段。

### `player.getXUID`

```lua
player.getXUID(name)
```

- 作用：查玩家 XUID。
- 参数：`name` string 必填。
- 返回：string；玩家不存在返回**空串**。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表的 XUID 字段。

### `player.health`

```lua
player.health(target)
```

- 功能、参数、返回与约束：target 为 self、玩家名或 runtime_id 字符串，默认 self。返回已观测信息含 health_known/health/max_health/health_time/stale，未知为 null，不推算其他玩家满血。仅能获取服务器给本机器人同步的属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `player.isOP`

```lua
player.isOP(name)
```

- 作用：判断玩家是否 OP。
- 参数：`name` string 必填。
- 返回：boolean（玩家存在且为 OP 才为 true，含不存在时返回 false）。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表（`IsOP` 由 `UpdateAbilities`/`AddPlayer` 等数据包维护）。

### `player.list`

```lua
player.list()
```

- 作用：列出全部在线玩家信息表。
- 参数：无。
- 返回：table（1 起索引，每个元素是 `player.get` 同款信息表）。
- 报错：无。
- 底层：遍历玩家缓存表。

### `player.nearby`

```lua
player.nearby(radius)
```

- 功能、参数、返回与约束：radius=0..512，默认 32；按机器人已观测位置计算同维度、仍存在的玩家距离并排序。不包括客户端不可见或未知坐标的玩家，机器人坐标未知报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `player.position`

```lua
player.position(target)
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


## 角色等级与经验（0.10.73）

`player.experience()` 读取当前机器人服务器 UpdateAttributes 缓存，返回 `session/player/connected/can_modify/level/progress/level_updated_at/progress_updated_at/source/total_xp`。`progress` 是0–1经验条；未同步字段为nil，JSON为null。累计经验总值没有可信来源，`total_xp` 不推算。断线不返回旧等级作为当前值。

`player.changeExperience({request_id=..., expected_session=..., operation="set_level", amount=..., expected_level=...})` 仅修改当前受控机器人，需要游戏OP。先读取再提交真实session；设置等级还必须带刚读到的expected_level，防止用过时读数做差值。另支持 `add_levels`（可负）与 `add_points`（非负）。使用原生xp命令并等待服务器回执，结果 `not_sent/outcome_unknown/rejected/accepted/verified/unchanged` 区分未发送、结果未知、拒绝、已接受、属性已核验与无需修改。未知结果不自动重发。

同一运行进程、同机器人保留最近64个请求票据及参数指纹，相同ID不会重复发送；同ID改参数会拒绝。这是有界幂等窗口，不是跨重启永久票据。HTTP读取 `GET /api/youyuan/player/experience?request_id=...` 可核最近票据；POST为同一修改合同。公共宿主MCP/SDK通过 `player.experience` 与 `player.changeExperience` 注册，平台/账号访问校验仍生效。累计经验与游戏属性修改不等于平台账号等级或权限修改。
