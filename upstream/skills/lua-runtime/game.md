# game · 游戏命令与交互

发送指令、定向消息、记分板、背包查询、NBT、聊天菜单与机器人动作。


### `game.commandResult`

```lua
game.commandResult(ticket, wait_ms, consume)
```

- 功能、参数、返回与约束：ticket,wait_ms=0,consume=false；pending=true表示结果未到。completed=true附result/error；只可读本实例ticket，consume=true删除完成记录。超时不重发。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.dataDir`

```lua
game.dataDir()
```

- 作用：返回本插件数据目录的完整路径，即 `插件数据文件/<id>/`。
- 参数：无。
- 返回：string。`__PLUGIN_ID__` 为空时返回空串。
- 报错：无。
- 底层：`pluginController.DataDir(id)`。

### `game.execute`

```lua
game.execute(command, options)
```

- 作用：发送命令并等待真实CommandOutput。
- 参数：`command, options?`；按此顺序传入。
- 返回：对象：submitted/confirmed/success/success_count/output/error等；超时为结果未知。
- 底层与游戏效果：默认命令请求自动化来源，关联UUID回包。需要根据上一条成功决定下一条时使用；普通快速链避免逐条调用它。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.executeBatch`

```lua
game.executeBatch(commands, options)
```

- 功能、参数、返回与约束：commands 字符串数组，options：channel=default/ai/player/console（或0/1/2字符串），mode=ordered/pipeline/submit/burst/stream，transport=command_request/settings/pyrpc（后两者仅submit，无逐条确认），window=1..16条在途命令；burst最多1024条整批协议发送，512KiB文本，默认无固定CPS等待，不保证同Tick，timeout_ms=100..10000。支持每实例默认配置与单次覆盖。ordered 逐条等待；pipeline 仅用于互不依赖的命令，不保证服务端完成顺序；submit 不等待结果。结果逐项含 submitted/confirmed/success/error/channel，超时未知不自动重发。stream单段最多4096条/512KiB，动态在途窗口最多1024，rate_cps=0无固定CPS，网络未知停止补发，正常无目标继续无条件后续；完整说明读 command-execution 技能。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.flyStart`

```lua
game.flyStart()
```

- 作用：开始 / 停止飞行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `flyStart/flyStop: <err>`。
- 底层：把机器人切换为飞行/降落：优先走游戏接口的飞行控制（运动能力包），不可用时回退为发送带飞行标志的 `PlayerAuthInput` 包。

### `game.flyStop`

```lua
game.flyStop()
```

- 作用：开始 / 停止飞行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `flyStart/flyStop: <err>`。
- 底层：把机器人切换为飞行/降落：优先走游戏接口的飞行控制（运动能力包），不可用时回退为发送带飞行标志的 `PlayerAuthInput` 包。

### `game.getBlockData`

```lua
game.getBlockData(x, y, z)
```

- 作用：读某坐标方块的完整数据（名称 + 方块状态 + 方块实体 NBT）。读取命令方块指令、箱子物品、告示牌文字、旗帜图案等都从这里入手。会先把机器人传送过去加载区块（有移动副作用）。
- 参数：`x`/`y`/`z` number 必填。
- 返回：table `{name = 方块名, states = {状态键 = 值, ...}, nbt = {NBT 键 = 值, ...}}`（无 NBT 时 `nbt` 省略）。
- 报错：失败抛 `getBlockData: <err>`。
- 底层：传送加载区块后，用游戏的**结构保存**机制读取：先通过 WS 自动化通道执行 `structure save "<随机名>" <区域>` 把该方块连同方块实体 NBT 存成一个临时结构，再发 `StructureTemplateDataRequest`（ExportFromSave）把结构数据拉回来，解析 `block_palette`（名称+状态）与 `block_position_data`（NBT）得到完整数据。有移动副作用。

### `game.getBlockNBT`

```lua
game.getBlockNBT(x, y, z)
```

- 作用：只读方块实体 NBT。命令方块指令在 `nbt.Command`，箱子物品在 `nbt.Items`，告示牌文字在 `nbt.FrontText.Text`。
- 参数：`x`/`y`/`z` number 必填。
- 返回：table，无 NBT 时返回 `nil`。
- 报错：失败抛 `getBlockNBT: <err>`。
- 底层：同 `getBlockData` 的结构保存机制（传送 + `structure save` + `StructureTemplateDataRequest`），只保留解析出的 NBT 部分。有移动副作用。

### `game.getBlockState`

```lua
game.getBlockState(x, y, z)
```

- 作用：读方块名称与方块状态（朝向/是否点亮/开关等），不含 NBT。
- 参数：`x`/`y`/`z` number 必填。
- 返回：table `{name = 方块名, states = {...}}`。
- 报错：失败抛 `getBlockState: <err>`。
- 底层：同 `getBlockData` 的结构保存机制，只保留名称与方块状态，丢弃 NBT。有移动副作用。

### `game.getBlockTile`

```lua
game.getBlockTile(x, y, z)
```

- 作用：读某坐标的方块名。用安全版：可能先把机器人传送过去加载区块再查，避免远处区块"未加载"报错，因此有移动副作用。
- 参数：`x`/`y`/`z` number 必填。
- 返回：string（方块名/方块 ID）。
- 报错：失败抛 `getBlockTile: <err>`。
- 底层：**有移动副作用**：机器人先 `tp` 到目标上方高空（y=150）把该 x,z 的区块加载出来（`testforblock` 只对已加载区块有效，远处直接查会返回 `outOfWorld`），等游戏刻 + 约 400ms 区块就绪后查询，最后传送回原位。查询本身通过 WS 自动化通道执行 `testforblock <x> <y> <z> air`：`SuccessCount > 0` → 该坐标是空气；否则从命令失败输出里解析实际方块名（兼容网易版的参数偏移）。应在独立后台协程中调用（会传送并等待）。

### `game.getBlockTileRaw`

```lua
game.getBlockTileRaw(x, y, z)
```

- 作用：直接查询方块，不做传送加载（更快，但区块未加载可能报错）。
- 参数：`x`/`y`/`z` number 必填。
- 返回：string（方块名）。
- 报错：失败抛 `getBlockTileRaw: <err>`。
- 底层：通过 WS 自动化通道执行 `testforblock <x> <y> <z> air`：`SuccessCount > 0` → 空气；否则从失败输出解析实际方块名。**不做传送**，远区块可能因未加载返回 `outOfWorld` 报错。

### `game.getBotPos`

```lua
game.getBotPos()
```

- 作用：读取当前活跃机器人自身坐标。
- 参数：无。
- 返回：table `{x, y, z}`。
- 报错：无。
- 底层：通过 WS 自动化通道执行只读命令 `querytarget @s` 并解析坐标、朝向、维度；pitch 与飞行/疾跑状态取本地缓存。

### `game.getCommandOptions`

```lua
game.getCommandOptions()
```

- 作用：查看本实例默认命令参数。
- 参数：`无`；按此顺序传入。
- 返回：channel/mode/window/timeout_ms/rate_cps/transport对象。
- 底层与游戏效果：来自项目command-options.json或本实例setCommandOptions，不代表服务器实测速率。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.getConfig`

```lua
game.getConfig(key, default)
```

- 作用：读本插件配置文件里某个键，缺失返回 `default`。
- 参数：`key` string 必填；`default` 任意类型可选。
- 返回：配置值（或 `default`）。
- 报错：无（读失败回退默认）。
- 底层：读取本插件配置文件 `插件配置文件/<id>/<id>.json`，命中则把值转成 Lua 返回，否则返回 `default`。依赖 `__PLUGIN_ID__`。

### `game.getMultiScore`

```lua
game.getMultiScore(target)
```

- 作用：读某目标在所有记分板上的分数。
- 参数：`target` string 必填。
- 返回：table，结构 `{player = {记分板名 = 分数, ...}}`。
- 报错：失败抛 `getMultiScore: <err>`。
- 底层：通过 **WS 自动化通道**执行 `scoreboard players list <目标>` 并等待 CommandOutput，把响应里逐条输出的"玩家·记分板·分数"（`commands.scoreboard.players.list.player.entry` 行）解析成 `{玩家 = {记分板 = 分数}}`。

### `game.getPlayerItem`

```lua
game.getPlayerItem(target, item, specialID)
```

- 作用：统计玩家背包里某种物品的数量，可带特殊数据 ID。
- 参数：`target` string、`item` string 必填；`specialID` number 可选，默认 0。
- 返回：number（数量）。
- 报错：失败抛 `getPlayerItem: <err>`。
- 底层：通过 **WS 自动化通道**执行游戏指令 `clear <目标> <物品> <特殊值> 0` 并等待 CommandOutput。**第 4 个参数 `0` 是游戏的"只统计不清除"技巧**——`clear` 的数量参数为 0 时不会真的清空物品，只报告玩家拥有多少这种物品（标准做法，可放心用于查数量）。响应为 `commands.clear.success` 时从输出的数量参数得到该物品数量；`commands.clear.failure.no.items`（玩家根本没有这种物品）返回 0；`commands.generic.syntax`（物品 ID 非法）报错。

### `game.getPlayerPos`

```lua
game.getPlayerPos(target)
```

- 作用：读玩家坐标与所在维度。
- 参数：`target` string 必填（可为玩家名或 `@a`/`@p` 选择器）。
- 返回：table `{x, y, z, dimension}`。
- 报错：失败抛 `getPlayerPos: <err>`。
- 底层：通过 **WS 自动化通道**执行只读命令 `querytarget <目标>` 并等待响应，解析返回的坐标（玩家脚部位置）并修正成可读值：正数向下取整到两位小数、负数先减 1、Y 再减 1.62（眼睛高度）；俯仰角 pitch 从工具箱维护的玩家缓存补取。`querytarget` 无副作用。

### `game.getPlayerPosXYZ`

```lua
game.getPlayerPosXYZ(target)
```

- 作用：读玩家坐标，直接返回三个数字。
- 参数：`target` string 必填。
- 返回：三个 number，依次 x、y、z。
- 报错：失败抛 `getPlayerPosXYZ: <err>`。
- 底层：同 `getPlayerPos`——执行 `querytarget` 后只取返回坐标的 x、y、z 三个数。

### `game.getRegionBlockData`

```lua
game.getRegionBlockData(x, y, z, sx, sy, sz)
```

- 作用：读一块区域（起点 `x,y,z`，尺寸 `sx×sy×sz`）所有方块的名称+状态+NBT，一次结构保存返回，适合整片区域/区块探查。
- 参数：6 个 number 均必填，尺寸需为正。
- 返回：table，键为 `"x,y,z"` 绝对坐标，值为 `{name, states, nbt}`。
- 报错：失败抛 `getRegionBlockData: <err>`。
- 底层：机器人先传送到区域中心上方加载区块，再**一次结构保存**（`structure save` + `StructureTemplateDataRequest`）把整片区域拉回，按 `block_indices` 线性索引还原成 `"x,y,z"` → 方块数据的映射。有移动副作用。

### `game.getScore`

```lua
game.getScore(scoreboard, target)
```

- 作用：读某记分板里某个目标（玩家名）的分数。
- 参数：`scoreboard` string 必填；`target` string 必填。
- 返回：number（分数）。
- 报错：失败抛 `getScore: <err>`。
- 底层：通过 **WS 自动化通道**执行 `scoreboard players test <目标> <记分板> 0 0`（借用"分数是否在 0~0 之间"的判断把分数值带出来）并等待 CommandOutput，从响应的 `Parameters[0]` 读出分数。响应是 `commands.scoreboard.objectiveNotFound`（计分板不存在）、`...players.list.player.empty`（目标不存在）或 `...players.score.notFound`（目标在该记分板无分数）时，分别报对应错误。

### `game.getTarget`

```lua
game.getTarget(selector)
```

- 作用：解析选择器，返回匹配的玩家名数组。
- 参数：`selector` string 必填。
- 返回：table（玩家名字符串数组，1 起索引）。
- 报错：失败抛 `getTarget: <err>`。
- 底层：通过 **WS 自动化通道**执行 `testfor <选择器>` 并等待响应，把输出参数里逗号分隔的玩家名列表解析成数组。选择器语法错误（`commands.generic.syntax`）时报错；无匹配返回空表。

### `game.getTickingAreaList`

```lua
game.getTickingAreaList()
```

- 作用：读当前加载的常驻区域（ticking area）列表。
- 参数：无。
- 返回：table `{区域名 = {dimension, start_x, start_z, end_x, end_z}}`。
- 报错：失败抛 `getTickingAreaList: <err>`。
- 底层：通过 **WS 自动化通道**执行 `tickingarea list all-dimensions` 并等待响应，把命令返回的多行文本输出（每维度一组，每组里 `- 名称: (startX, startZ) to (endX, endZ)`）解析成结构化表。

### `game.httpGet`

```lua
game.httpGet(url)
```

- 作用：发起 HTTP GET 请求，返回响应体文本。
- 参数：`url` string 必填。
- 返回：string（响应体）。
- 报错：失败抛 `httpGet: <err>`。
- 底层：发起一次真实的 HTTP GET 请求，把响应体按 UTF-8 文本返回；网络错误或非 2xx 状态码都会抛错。

### `game.httpPost`

```lua
game.httpPost(url, body, headers)
```

- 作用：发起 HTTP POST 请求，可带自定义请求头。
- 参数：`url` string 必填；`body` string 可选，默认 `""`；`headers` table（键值均为 string）可选。
- 返回：string（响应体）。
- 报错：失败抛 `httpPost: <err>`。
- 底层：发起一次真实的 HTTP POST 请求（可带请求头与请求体），把响应体按 UTF-8 文本返回；网络错误或非 2xx 状态码都会抛错。

### `game.isCmdSuccess`

```lua
game.isCmdSuccess(cmd)
```

- 作用：执行一条指令并判断是否成功，返回布尔。
- 参数：`cmd` string 必填。
- 返回：boolean（是否成功）。
- 报错：失败抛 `isCmdSuccess: <err>`。
- 底层：通过 **WS 自动化通道**执行指令并**等待 CommandOutput**，命令响应的 `SuccessCount > 0` 即视为成功。无 OP 或语法错误的指令，服务器返回失败输出，此函数返回 `false`。

### `game.jsonDecode`

```lua
game.jsonDecode(s)
```

- 作用：把 JSON 字符串解析成 Lua 值。
- 参数：`s` string 必填。
- 返回：任意 Lua 值（表/数字/字符串/布尔）。
- 报错：解析失败抛 `jsonDecode: <err>`。
- 底层：把 JSON 解析成对应 Lua 类型（对象→表、数组→1 起索引的表、数字/字符串/布尔原样返回）。

> 这几个是挂在 `game` 上的轻量便捷方法；需要完整请求控制（超时、多种方法、错误码）时用 4.12 节的 `http` 模块。

### `game.jsonEncode`

```lua
game.jsonEncode(v)
```

- 作用：把任意 Lua 值编码成 JSON 字符串。
- 参数：`v` 任意类型。
- 返回：string（JSON）。
- 报错：序列化失败抛 `jsonEncode: <err>`。
- 底层：把 Lua 值映射成对应的 JSON 类型（表→对象/数组、数字/字符串/布尔原样）后编码成字符串。

### `game.jump`

```lua
game.jump()
```

- 作用：让机器人跳一下。
- 参数：无。
- 返回：无。
- 报错：失败抛 `jump: <err>`。
- 底层：模拟按一次跳跃键：置位跳跃标志让心跳携带跳跃输入，约 200ms 后自动松开。

### `game.move`

```lua
game.move(dx, dy, dz)
```

- 作用：让机器人相对当前位置移动。
- 参数：`dx`/`dy`/`dz` number 必填。
- 返回：无。
- 报错：失败抛 `move: <err>`。
- 底层：在当前坐标上累加位移后发一个 `MovePlayer` 包（`MoveModeTeleport`），瞬间完成相对移动。

### `game.placeNBTBlock`

```lua
game.placeNBTBlock(x, y, z, name, states, nbtTable)
```

- 作用：放置一个带 NBT 的特殊方块（命令方块/结构方块/告示牌/箱子/旗帜等），自动按方块类型选择最快可靠的放置方式：
  - 命令方块：`{Command = "say hi", Conditional = 1, TickDelay = 0}`；
  - 告示牌：`{FrontText = {Text = "你好", Color = "black"}, ...}`；
  - 箱子：`{Items = {{Count = 1, Name = "minecraft:diamond", Slot = 0}, ...}}`。
- 参数：`name` string 必填；`states` string 可选；`nbtTable` table 可选。
- 返回：无。
- 报错：失败抛 `placeNBTBlock: <err>`。
- 底层：按 `building.ClassifyBlock` 对方块分类后选择不同游戏机制：命令方块/结构方块 → **直接发 NBT 包**（快）；告示牌 → 直接发 `PlaceSign` 包（快）；重 NBT 方块（箱子/旗帜等）→ 机器人**放到远程工作台 + 结构复制**（慢，需要结构权限）；普通方块 → `setblock` 指令。

### `game.promptChoice`

```lua
game.promptChoice(player, title, options, timeoutSec)
```

- 作用：显示菜单并**阻塞等待玩家选择**，返回所选选项下标。
- 参数：`player`/`title` string、`options` table 必填；`timeoutSec` number 可选，默认 30。
- 返回：number 或 nil。成功返回 **1 起** 下标（第一个是 1）；超时/取消/无效输入返回 `nil`。
- 报错：失败抛 `promptChoice: <err>`。
- 底层：先用 `showMenu` 把菜单发给玩家，再用 `waitMsg` 的机制**阻塞等待该玩家的下一条聊天消息**，把它解析成序号（支持 `1`、`1.`、`1、` 等写法）。超时或输入无效返回 `nil`。

### `game.queryInventory`

```lua
game.queryInventory(selector)
```

- 作用：查询玩家背包物品清单。
- 参数：`selector` string 必填。
- 返回：table `{items = {{name, count, slot, damage}, ...}}`。
- 报错：失败抛 `queryInventory: <err>`。
- 底层：通过 WS 自动化通道执行网易特有的 `codebuilder_actorinfo inventory <选择器>` 并等待响应，从命令响应的 DataSet（JSON）里解析出 `{name, count, slot, damage}` 物品清单。需要机器人具备网易 AI/指令权限。

### `game.registerTrigger`

```lua
game.registerTrigger(wordsTable, handlerName)
```

- 作用：注册聊天触发词，常驻实例调用handler(player,args)。
- 参数：`wordsTable, handlerName`；按此顺序传入。
- 返回：Lua无返回；SDK注册成功为true。
- 底层与游戏效果：源码只加载一次；玩家发“触发词 参数”后在该插件常驻状态中调用函数。args为参数数组，不重置全局变量。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `game.rotate`

```lua
game.rotate(pitch, yaw, seconds)
```

- 作用：设置机器人朝向。
- 参数：`pitch`/`yaw` number 必填；`seconds` number 可选——省略或 `0` 时**立即转到**目标角度，`>0` 时在 seconds 秒内**平滑转动**过去（yaw 走最短路径，如从 350° 转到 10° 只转 20°）。
- 返回：无。
- 报错：失败抛 `rotate: <err>`。
- 底层：只更新机器人本地视角状态，心跳（50ms）通过 `PlayerAuthInput` 包把新视角发送到服务器。秒数 `>0` 时先在本地逐帧插值，再由心跳带出中间帧。

### `game.scene`

```lua
game.scene()
```

- 功能、参数、返回与约束：无参数。当前账号的机器人和已观测实体快照；含 known、stale、数据时间和来源。AI 没有独立游戏实体，bot 是当前账号控制的机器人。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.sendActionbar`

```lua
game.sendActionbar(target, text)
```

- 作用：给玩家显示动作栏消息（物品栏上方那行）。
- 参数：`target`/`text` string 必填。
- 返回：无。
- 报错：失败抛 `sendActionbar: <err>`。
- 底层：通过当前账号命令发送通道发 `titleraw <target> actionbar {"rawtext":[{"text":"..."}]}`。

### `game.sendChat`

```lua
game.sendChat(target, message)
```

- 作用：给指定玩家或选择器（如 `@a`）发一条聊天消息。
- 参数：`target` string 必填；`message` string 必填。
- 返回：无。
- 报错：失败抛 `sendChat: <err>`。
- 底层：通过当前账号命令发送通道发 `tellraw <target> {"rawtext":[{"text":"..."}]}`（target 为 `@a` 或玩家名），文本里的 `"` 会自动转义。

### `game.sendCommand`

```lua
game.sendCommand(cmd, options)
```

- 作用：无等待按顺序提交命令；没有默认逐条CPS等待。
- 参数：`cmd, options?`；按此顺序传入。
- 返回：Lua旧单参数写法保持无返回值；Python/C++返回提交信息（submitted/confirmed）。
- 底层与游戏效果：当前默认魔法/ai使用SettingsCommand，套execute as机器人 at @s保留执行者与相对坐标；player/console为相应CommandRequest。不会逐条等待CommandOutput。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.sendCommandAsync`

```lua
game.sendCommandAsync(command, options)
```

- 功能、参数、返回与约束：command,options?；立刻返回ticket，不阻塞插件回调；后台等待真实CommandOutput，使用game.commandResult读取。最多128个未读取ticket/实例，满时明确报错；Fast路径不走该窗口。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.sendCommandConfirmed`

```lua
game.sendCommandConfirmed(command, options)
```

- 功能、参数、返回与约束：command, options?；game.execute别名，等待真实CommandOutput，超时为未知，不自动重发。只在依赖前条结果时使用。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.sendCommandFast`

```lua
game.sendCommandFast(command, options)
```

- 功能、参数、返回与约束：command, options?；无等待提交，魔法/ai 默认使用 SettingsCommand 并保持机器人 @s/相对坐标；其他通道保持来源。rate_cps 默认0，不逐条等回包。confirmed=false 不能当游戏成功。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.sendCommandsFast`

```lua
game.sendCommandsFast(commands, options)
```

- 功能、参数、返回与约束：commands数组, options?；一次插件宿主边界按顺序提交完整数组，不限整数组行数，不等待逐条回包。先校验全部命令；中途网络失败报告已提交数量，不自动重发，不保证同一服务器Tick或原子执行。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.sendLines`

```lua
game.sendLines(player, text, maxLines)
```

- 作用：把长文本**按换行逐条发送**，避免单条超长被截断、或某行含敏感词整段被拦。超行数时先提示"仅显示前 N 行"。空行跳过。
- 参数：`player` string 必填；`text` string 必填；`maxLines` number 可选，默认 50。
- 返回：无。
- 报错：无。
- 底层：把长文本按 `\n` 切分，逐行用 `sendTargeted` 的定向 tellraw 机制发送；超过 `maxLines` 先提示"仅显示前 N 行"，空行跳过。

### `game.sendPacket`

```lua
game.sendPacket(typeNameOrId, fieldsTable)
```

- 作用：发送一个已校验的协议包，支持包名或协议ID。
- 参数：`typeNameOrId, fieldsTable?`；按此顺序传入。
- 返回：submitted=true, confirmed=false；校验、编码或网络错误抛异常。
- 底层与游戏效果：反射填充当前内核真实类型后conn.WritePacket；字段用snake_case；bytes/UUID为hex，Vec为数组，64位ID建议十进制字符串。联合接口类型不猜测，使用高级动作API或payload_hex完整包体。登录生命周期不可重发。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.sendRichTellraw`

```lua
game.sendRichTellraw(msg)
```

- 作用：发送原生 tellraw 富文本（支持 JSON 样式），常用于公告。
- 参数：`msg` string 必填。
- 返回：无。
- 报错：失败抛 `sendRichTellraw: <err>`。
- 底层：通过当前账号命令发送通道发 `tellraw @a {"rawtext":[{"text":"..."}]}`，`msg` 里的 `"` 自动转义后嵌入 JSON。

### `game.sendTargeted`

```lua
game.sendTargeted(player, text)
```

- 作用：只给指定玩家发一条消息，不广播给其他人。`§` 前缀可带颜色（如 `"§a成功！"`）。
- 参数：`player` string 必填；`text` string 必填。
- 返回：无。
- 报错：无（底层错误被忽略）。
- 底层：通过**当前账号命令发送通道**（无等待默认SettingsCommand，保留机器人执行上下文）发定向 tellraw：`tellraw @a[name="<玩家>"] {"rawtext":[{"text":"..."}]}`。只发给该玩家、绝不广播；用 `@a[name=...]` 选择器是为了兼容网易服（普通通道在网易不执行）。

### `game.sendTitle`

```lua
game.sendTitle(target, title, subtitle)
```

- 作用：给玩家显示大标题 + 副标题。
- 参数：`target`/`title`/`subtitle` 均 string 必填。
- 返回：无。
- 报错：失败抛 `sendTitle: <err>`。
- 底层：通过当前账号命令发送通道发 `titleraw <target> title {"rawtext":[{"text":"..."}]}`；副标题非空时再追加一条 `titleraw <target> subtitle {"rawtext":[...]}`。

### `game.setBlock`

```lua
game.setBlock(x, y, z, name, states)
```

- 作用：放置一个普通方块，可带方块状态字符串。
- 参数：`name` string 必填；`states` string 可选（如 `["facing_direction"=3]`，默认空）。
- 返回：无。
- 报错：失败抛 `setBlock: <err>`。
- 底层：通过**当前账号命令发送通道**发 `setblock <x> <y> <z> <方块> <states>`（PyRpc `ExecuteCommandEvent`，权限比普通命令通道高，在网易租赁服更可靠），发完稍等片刻返回。

### `game.setBlockState`

```lua
game.setBlockState(x, y, z, statesTable)
```

- 作用：修改某方块的方块状态并保留原 NBT（如把命令方块设为条件模式、把方块朝向改朝北）。会先读当前方块，合并新状态后重新放置。
- 参数：`statesTable` table 必填，键 = 状态名，值 = 数值/布尔（如 `{conditional_bit = 1, facing_direction = 3}`）。
- 返回：无。
- 报错：失败抛 `setBlockState: <err>`。
- 底层：先按 `getBlockData` 的结构保存机制读回当前方块的名称、状态与 NBT，把新状态合并进去后，再按 `placeNBTBlock` 的方式**重新放置**该方块（保留原 NBT）。有移动副作用（需要传送读取）。

> **注意**：箱子/旗帜这类"重 NBT 方块"写入在网易下走远程工作台 + 结构复制，较慢且需要结构权限；命令方块/告示牌直接发包，快。读取和写入都会传送机器人加载区块，属正常副作用。

### `game.setCommandOptions`

```lua
game.setCommandOptions(options)
```

- 作用：更新本实例默认命令参数，不修改其他插件。
- 参数：`options`；按此顺序传入。
- 返回：合并后的options对象。
- 底层与游戏效果：channel=ai/player/console/default，mode=ordered/pipeline/submit/burst/stream，window、timeout_ms、rate_cps；rate_cps=0不加人为CPS等待。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.setConfigDefault`

```lua
game.setConfigDefault(table)
```

- 作用：把配置文件中**缺失的键**写入默认值（已有的键不覆盖）。
- 参数：`table` 必填（键值任意类型）。
- 返回：无。
- 报错：无（写失败忽略）。
- 底层：先读 `插件配置文件/<id>/<id>.json`，把其中缺失的键补写进去（已存在的键不动）。依赖 `__PLUGIN_ID__`。

### `game.setEffect`

```lua
game.setEffect(player, effect, duration, level, particles)
```

- 作用：给玩家施加药水效果。
- 参数：`player` string、`effect` string、`duration` number、`level` number 必填；`particles` boolean 可选，默认 `true`。
- 返回：无。
- 报错：失败抛 `setEffect: <err>`。
- 底层：通过 **WS 自动化通道**执行 `effect <玩家> <效果> <时长秒> <等级> <是否显示粒子>` 并等待响应，响应 `commands.effect.success` 才算成功。`commands.generic.noTargetMatch`（没有匹配目标）时报错；等级上限 255、时长上限 1000000 秒，超出直接报错。

### `game.setScore`

```lua
game.setScore(board, target, score)
```

- 作用：写入记分板，执行 `scoreboard players set`。
- 参数：`board` string、`target` string、`score` number 均必填。
- 返回：无。
- 报错：失败抛 `setScore: <err>`。
- 底层：拼指令 `scoreboard players set <target> <board> <score>`，通过**当前账号命令发送通道**发出，不等响应。

### `game.showMenu`

```lua
game.showMenu(player, title, options)
```

- 作用：给玩家显示一个聊天栏菜单（标题 + 若干选项），只展示不等待。
- 参数：`player` string、`title` string 必填；`options` table（字符串数组）必填。
- 返回：无。
- 报错：无。
- 底层：通过当前账号命令发送通道给该玩家发**多条定向 tellraw**（绝不广播）：标题一条，每个选项一条 `§e序号. 选项文字`。

### `game.sneakStart`

```lua
game.sneakStart()
```

- 作用：开始 / 停止潜行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sneakStart/sneakStop: <err>`。
- 底层：设置潜行标志，心跳持续携带潜行输入；`sneakStop` 额外发一个带 `StopSneaking` 标志的 `PlayerAuthInput` 包。

### `game.sneakStop`

```lua
game.sneakStop()
```

- 作用：开始 / 停止潜行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sneakStart/sneakStop: <err>`。
- 底层：设置潜行标志，心跳持续携带潜行输入；`sneakStop` 额外发一个带 `StopSneaking` 标志的 `PlayerAuthInput` 包。

### `game.sprintStart`

```lua
game.sprintStart()
```

- 作用：开始 / 停止疾跑。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sprintStart/sprintStop: <err>`。
- 底层：设置奔跑标志，心跳持续携带奔跑输入。

### `game.sprintStop`

```lua
game.sprintStop()
```

- 作用：开始 / 停止疾跑。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sprintStart/sprintStop: <err>`。
- 底层：设置奔跑标志，心跳持续携带奔跑输入。

### `game.stopMove`

```lua
game.stopMove()
```

- 作用：停止机器人当前移动。
- 参数：无。
- 返回：无。
- 报错：失败抛 `stopMove: <err>`。
- 底层：关闭持续移动的本地循环并清空移动输入标志，心跳随之不再携带方向输入。

### `game.teleport`

```lua
game.teleport(x, y, z)
```

- 作用：传送机器人到绝对坐标。
- 参数：`x`/`y`/`z` number 必填。
- 返回：无。
- 报错：失败抛 `teleport: <err>`。
- 底层：发一个 `MovePlayer` 包（`MoveModeTeleport`）把机器人瞬间传送到目标坐标——**不是 `/tp` 指令**，是客户端传送包，服务器按传送处理。

### `game.waitMsg`

```lua
game.waitMsg(player, timeoutSec)
```

- 作用：**阻塞等待玩家在聊天栏发一条消息**，返回内容。用于要输入（领地、数量等）。
- 参数：`player` string 必填；`timeoutSec` number 可选，默认 30。
- 返回：string（收到的消息）。超时返回 `nil`。
- 报错：失败抛 `waitMsg: <err>`。
- 底层：在机器人**收包循环**里为该玩家注册一个"等待通道"，玩家下一条聊天消息到达时投递给等待者。它**独占**该玩家的消息——同一玩家已有活跃等待会话时，新消息会被它消费掉，`onChat` 等其它 handler 收不到。


## 服务器关闭命令反馈时

实服已验证：gamerule sendcommandfeedback false 会使 Confirmed/querytarget 等结果接口收不到回包，即使命令已经执行。Fast 不等待回包，模式维护也使用无等待创造命令和飞行输入。需要条件链、查询结果或确认时，要由服主确保服务器允许命令反馈；超时返回未知，不能把它当失败自动重发。高频无条件链不要为语法验证逐条增加确认。
