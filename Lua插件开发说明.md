# Lua 常驻插件开发手册

游园 0.10.17。保留原版Lua调用方式，补充协议包、整链执行和常驻资源。不是Java/Spigot插件。

## 从插件到游戏

ZIP根包含manifest.json、main.lua，可带lib/、resources/、docs.md和config.json。manifest示例：

```json
{"id":"my_game_plugin","name":"我的游戏插件","type":"lua","version":"1.0.0","pre_plugins":{}}
```

源码运行一次；后续触发、事件、定时器、网页和跨插件调用复用该账号的Lua状态，顶层变量与require缓存持续保留。停用/热重载/进程结束才重建。停用清理任务和订阅；需要跨重载保存的数据写自己的数据目录。旧事件handler仍用全局函数名字符串；scheduleTick/scheduleHz也可传闭包。

```lua
local received = 0
function hello(name, args)
  received = received + 1
  game.sendTargeted(name, "§a常驻计数：" .. received)
end
game.registerTrigger({".你好"}, "hello")
```

导入、连接真实服务器、启用，游戏聊天输入`.你好`两次应收到1和2，而不是每次1。错误用pcall处理，保留文件/行号。Lua单次调用上下文最多2分钟；这不是逐条命令节流。统一宿主时钟驱动调度，错过触发默认skip并计数，不重入。

## 哪些功能可以组成原来的链

| 游戏目的 | 已有入口 | 合适的链步骤 | 结果核对 |
|---|---|---|---|
| 雪球菜单、连续计分、CD、选择器检测 | game.sendCommand / sendCommandsFast | 预读TXT，整轮按序连续提交 | 分数或菜单实际变化 |
| 上一条成功才继续 | game.execute / 有条件链 | 上一条和有条件节点设置确认点 | CommandOutput；超时未知 |
| 命令链选择与启停 | chains.menu / handleChat / start / stop | 事件触发已有独立链 | chains.get/list的状态 |
| 玩家挥臂或拾取物触发技能 | events.onSwing / onItemPickup | 收到真实事件后启动链 | source与runtime_id匹配 |
| 查玩家位置、背包或血量后分支 | game.getPlayerPos / queryInventory / player.health | 先查询或读取观测，再选择链 | unknown/stale/权限与返回值 |
| 建筑、声音、特效 | setblock/fill/playsound/particle指令 | 转成TXT命令再交同一执行器 | 区块、游戏声音和输出 |
| 真实挖掘、攻击、移动 | world.mineBlock/useBlock、bot.attack/startMovement | 插件动作回调；不能伪装成同刻原子命令 | 服务端位置、UpdateBlock、伤害等 |

TXT的0,0,0等是游园预解析语法，不发给游戏；第一位0连锁/1脉冲/2循环，第二位有无条件，第三位保留旧红石格式。`.`断开结果继承，`!`周期分界。具体控制语法读取command-chain/control-flow.md。Hz是**一整轮**目标频率；默认快速路径不逐条等回包、不每轮写文件、不重新解析TXT、不设置默认CPS等待。window是确认路径的在途数量，既不是一轮命令条数，也不是网络数据报个数。发送完成、服务端执行、同一个游戏Tick是三个不同指标；没有服务器批处理协议就不能承诺任意长链同Tick原子执行。

## 游戏连接与可观测范围

先连接对应服务器种类及其真实ID、room_id等，检查已连接与权限，再启用插件。原版游戏功能通过命令、服务器下发包、网易扩展和本地缓存组合实现，语言本身不会提高服务器权限。OP可用网易codebuilder_actorinfo查询背包，不能因此默认看到所有玩家的血量或输入。PlayerAuthInput通常是自己→服务器；旁观机器人不自动收到其他玩家的输入。RemoveActor是离开视野/移除，不足以确定死亡；Animate挥臂不能确定攻击命中或击杀者；UpdateBlock不能单独确定破坏者。没有真实数据时保留未知。

## 配置、资源、重载与完整导出

plugin.codeDir/dataDir/configDir/resourceDir/tempDir区分目录；Lua旧数据和配置路径保持兼容，SDK Python/C++数据按账号隔离。plugin.readResource读取原包resources，extractResource原字节复制到持久目录，**不是ZIP解压**。完整备份导出整个目录包含资源、配置和数据；发布到市场时排除私有数据。修改代码后先校验，再发布、热重载；重载不保留RAM，自己持久化所需状态。

## 文档与工具不是权限

本手册和每命名空间章节为离线Skill；AI默认使用工作区入口语言；玩家也能多选其他语言技能，按任务需要的包/函数读取小节，不加载全体包正文。工具是否允许调用由工作区配置决定；知道函数不代表自动获准执行。原始协议目录单独加载packet-reference，包含当前编译内核所有真实已注册包和字段；每包发送能否成功仍由服务器判断。当前血量/死亡及部分物理动作尚未完成所有真人场景验收；文档不将未测能力标成实测。

## API章节目录

- [game · 游戏命令与交互](game.md)：发送指令、定向消息、记分板、背包查询、NBT、聊天菜单与机器人动作。
- [player · 玩家资料与感知](player.md)：缓存、XUID、权限、背包、位置与已同步血量。未知血量不是20。
- [world · 方块与物理操作](world.md)：查询、setblock、真实挖掘和手持物品点击。权限替换方块与生存挖掘是两种机制。
- [events · 事件与抓包](events.md)：聊天、进退服、死亡、挥臂、实体出现与拾取，以及原始协议订阅。
- [packet · 原始协议包](packet.md)：schema/describe查询字段，on/onAll监听，send请求服务器执行；不会取得服务器未下发的数据。
- [bot · 机器人控制与路由](bot.md)：选择舰队机器人、状态、输入移动、看向坐标、近战攻击。无自动寻路保证。
- [chains · 命令链管理](chains.md)：同一套TXT预解析、整链频率、启停、跨链与聊天菜单，可供插件调用。
- [building · 建筑与颜色](building.md)：结构格式检测、解析和方块颜色匹配。这些解析函数不等于放置整个建筑。
- [media · 图片与音乐](media.md)：图片字符画、MIDI解析、音高换算；解析函数本身不自动向游戏播放。
- [plugin · 实例资源与诊断](plugin.md)：目录、资源、能力与统计；代码、配置、持久数据分开管理。
- [util · 文件、时间与调度](util.md)：JSON文件、文本、数学、时间与整个回调的定时任务。
- [api · 跨插件调用](api.md)：通过plugin_api导出函数；Lua、Python、C++可相互调用已启动实例。
- [http · 网络与JSON](http.md)：返回status/body/headers，非2xx响应也可检查，不把远端错误吞掉。
- [system · Android设备](system.md)：系统桥、通知、震动、Shizuku；桌面支持范围与Android不同。
- [storage · 缓存与事务](storage.md)：内存键值、原子加分、自动持久化与条件事务。
- [web · 插件控制页面](web.md)：声明式页面、HTML资源、动作回调与事件推送，重载清理旧注册。
- [accounts · 跨账号调用](accounts.md)：以当前控制端身份读取目标账号状态、发消息、批量提交命令或调用其常驻插件导出函数。授权按账号定向生效，跨账号递归调用被拒绝。
- [accountsPool · 共享机器人账号池](accountsPool.md)：登记、启动、绑定、借用与释放机器人，统一保存抓包配置并生成错峰观察规划。规划不等于执行，启动请求不等于进服。
- [diagnostics · 解码与事件诊断](diagnostics.md)：真实解码包计数、事件队列配置、事件追踪、可见性与战斗证据。只报告已收到的数据，被覆盖的历史标为未知。
- [entities · 实体观测快照](entities.md)：身份、位置、血量、属性与装备，只覆盖服务器已下发且未过期的数据，不扫描全世界。
- [entity · 单实体读取与伤害](entity.md)：血量字段区分已知与未知，未知最大生命保留 null；伤害事件不推断攻击者。
- [hud · HUD频道管理](hud.md)：同一玩家多插件文本按优先级仲裁显示，过期自动恢复下一频道；只管理经本接口发送的内容。
- [mcp · 本机MCP](mcp.md)：工具列表、资源目录、调用与连接信息。自动绑定当前账号，不允许指定其他账号，不导出密钥。
- [observerBridge · 观察桥与采集](observerBridge.md)：采集密钥、自身输入接入、常驻观察、轮巡、维度调度与跨账号事件汇总。submitted 不等于服务器已广播，不承诺全服覆盖。
- [scoreboard · 计分板读写](scoreboard.md)：读取已同步条目，提交原生 add/remove/set；提交成功不等于服务器已生效。

语法块是参数顺序参考；下方游戏用例为完整工作流。章节包含当前真实注册方法，不用内部Go函数名冒充插件API。

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

### `game.getBlockTileAsync`

```lua
game.getBlockTileAsync(x, y, z)
```

- 功能、参数、返回与约束：x,y,z整数；后台执行原getBlockTileRaw的WS方块查询，不传送；立即返回ticket。game.commandResult(ticket,0,true)完成时result={block,confirmed=true}，失败返回error，不编造方块。适合事件处理器，不能用于查询未加载的全世界。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.getOnlinePlayers`

```lua
game.getOnlinePlayers()
```

- 功能、参数、返回与约束：无参数；用querytarget @a读取当前在线玩家名列表，等价于game.getTarget("@a")。未连接或查询失败抛getOnlinePlayers: <err>。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.getServerTPS`

```lua
game.getServerTPS()
```

- 功能、参数、返回与约束：无参数；返回服务器上报的TPS数值。当前宿主没有原生服务器TPS数据源，内部观测恒为未知，因此实际返回nil；不使用世界时间、本地时钟、命令RTT或配置tick率代替。getTPS/getServerTPS/serverTPS/getTickRate/getServerTickRate是同一实现的五个别名。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.getServerTickRate`

```lua
game.getServerTickRate()
```

- 功能、参数、返回与约束：同getServerTPS的同一实现别名；当前宿主无服务器TPS数据源，返回nil。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.getTPS`

```lua
game.getTPS()
```

- 功能、参数、返回与约束：同getServerTPS的同一实现别名；当前宿主无服务器TPS数据源，返回nil。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.getTickRate`

```lua
game.getTickRate()
```

- 功能、参数、返回与约束：同getServerTPS的同一实现别名；当前宿主无服务器TPS数据源，返回nil。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.importBlocks`

```lua
game.importBlocks(list, options)
```

- 功能、参数、返回与约束：list为方块条目数组（每项含x/y/z等相对坐标，越界直接报错）；options支持x/y/z=基准坐标（默认0/64/0）、speed（负值按0）、region_mode=1..3、dimension=overworld|nether|the_end、anti_loss=true时逐区块testforblock确认已加载。异步启动本机结构导入，返回任务句柄，用game.importWait等待；会传送机器人并占用空间作业，失败不输出缺块结果。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.importWait`

```lua
game.importWait(job)
```

- 功能、参数、返回与约束：job为game.importBlocks返回的本机任务句柄；阻塞等待该任务结束，成功返回true，任务失败或上下文取消抛importWait: <err>。传入非本机任务句柄报『不是本机导入任务』。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.sceneStats`

```lua
game.sceneStats()
```

- 功能、参数、返回与约束：无参数。只返回已观测实体计数cached/present/fresh及packet_count/time，不复制装备与实体明细；供周期公告和监控使用。不是全服实体数量。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.sendCommandWithResp`

```lua
game.sendCommandWithResp(command, channel, seconds)
```

- 功能、参数、返回与约束：command字符串；channel默认ai（走normalizeCommandChannel校验）；seconds默认5，须为正数。发送命令并等待真实回包，返回包内容映射；超时报『等待服务器回包超时，命令可能已执行』，不自动重发。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `game.serverTPS`

```lua
game.serverTPS()
```

- 功能、参数、返回与约束：同getServerTPS的同一实现别名；当前宿主无服务器TPS数据源，返回nil。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

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

### `player.changeExperience`

```lua
player.changeExperience(options)
```

- 功能、参数、返回与约束：options={request_id,expected_session,operation:set_level|add_levels|add_points,amount,expected_level?}；当前机器人需游戏OP，原生xp命令，区分not_sent/outcome_unknown/rejected/accepted/verified；同request_id不重复发送。set_level需先读取并提交expected_level。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `player.equipment`

```lua
player.equipment(target)
```

- 功能、参数、返回与约束：name或runtime_id；head/chest/legs/feet/main_hand/off_hand/body，每槽含known/item_id/item_id_known/network_id/count/enchantments/enchantments_known/nbt/received_at/age_ms/stale。物品名来自本会话ItemRegistry；附魔来自NBT。未知槽或未同步附魔不编造。expires_at=null：装备包没有物品失效时间。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `player.experience`

```lua
player.experience()
```

- 功能、参数、返回与约束：无参数；当前机器人实际游戏等级与经验条0–1，未同步为null，total_xp未知，不发送查询命令。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `player.inventory`

```lua
player.inventory(target, window_id)
```

- 功能、参数、返回与约束：target=self,window_id=0；读取本机器人实际收到的InventoryContent/InventorySlot，不做同步背包查询。其他玩家背包服务器通常不广播，返回known=false；不把装备冒充整个背包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `player.rotation`

```lua
player.rotation(target)
```

- 功能、参数、返回与约束：name或runtime_id；pitch/yaw/head_yaw各自known、updated_at、source/stale。MovePlayer及Absolute/Delta真实角度，旋转无位移也更新。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# world · 方块与物理操作

查询、setblock、真实挖掘和手持物品点击。权限替换方块与生存挖掘是两种机制。


### `world.block`

```lua
world.block(x, y, z)
```

- 作用：查询世界某坐标的方块名（需要机器人连接，目标为当前活跃机器人）。
- 参数：`x`/`y`/`z` number 必填（整数坐标）。
- 返回：string（方块名）。
- 报错：查询失败抛 `world.block: <err>`；参数非整数抛类型错误。
- 底层：同 `game.getBlockTile`——机器人先传送加载区块，再执行 `testforblock <x> <y> <z> air` 判断方块名。**有移动副作用**（会把当前活跃机器人传送过去再传回）。

### `world.blockState`

```lua
world.blockState(x, y, z)
```

- 功能、参数、返回与约束：x,y,z 整数坐标。返回当前维度 layer=0 的最近 UpdateBlock 观测，未知 known=false；包含方块 runtime_id，不把任何更新都认定是玩家破坏。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.breakBlock`

```lua
world.breakBlock(x, y, z, mode)
```

- 功能、参数、返回与约束：x,y,z 整数坐标，mode=command 或 creative_action，默认 command。command 用 setblock ... air destroy 等待实际结果；creative_action 发创造模式破坏动作，要求游戏模式和六格范围，不代表服务器确认。生存挖掘的装备/耗时/交易流程尚不支持。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.mineBlock`

```lua
world.mineBlock(x, y, z, duration_ms, slot, face)
```

- 功能、参数、返回与约束：x,y,z,duration_ms=1000,slot=0,face=1；生存挖掘尝试，真实开始/持续/结束动作与手持库存交易，按 100ms 发送。需要最近方块观测；服务端可拒绝，返回 submitted/confirmed。硬度、工具正确与否由服务器判断。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.placeBlock`

```lua
world.placeBlock(x, y, z, block)
```

- 功能、参数、返回与约束：x,y,z 整数坐标，block 例如 minecraft:stone。用服务器 setblock 指令，返回真实结果，需要权限。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.useBlock`

```lua
world.useBlock(x, y, z, face, slot)
```

- 功能、参数、返回与约束：x,y,z,face=1,slot=0；真实手持物品点击指定方块面（可放置、开容器），需要已观测目标方块和六格范围。不是 setblock；返回提交状态，消耗与放置观察服务器。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.containerState`

```lua
world.containerState(x, y, z)
```

- 功能、参数、返回与约束：x,y,z整数。读取当前维度BlockActorData已同步的容器类型：known/is_container/block_entity_id/timestamp/stale/source，不查询网络、不返回箱内物品；新UpdateBlock使旧记录失效。未广播或过期保持未知，不证明由玩家放置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# events · 事件与抓包

聊天、进退服、死亡、挥臂、实体出现与拾取，以及原始协议订阅。


### `events.captureClear`

```lua
events.captureClear()
```

- 功能、参数、返回与约束：清空当前账号抓包缓冲，保持抓包开关。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureRead`

```lua
events.captureRead(after_sequence, limit)
```

- 功能、参数、返回与约束：after_sequence=0,limit=256，返回记录/游标/覆盖和抽样统计。缓冲有界，单条超大小截为预览；不是手机所有 App 的网络抓包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureStart`

```lua
events.captureStart(config)
```

- 功能、参数、返回与约束：config={types:[包名或*],exclude:[包名],filter:{fields:{...},every_ms:...},capacity:1..2048,max_bytes:512..65536}。开启账号级解码入站游戏抓包；默认血量/死亡/位置/方块包，认证握手不记录，* 默认不包含大区块包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureStop`

```lua
events.captureStop()
```

- 功能、参数、返回与约束：停止记录，保留已抓数据。当前账号的多个插件共用账号级记录；使用时协调抓包开关。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onBlockChange`

```lua
events.onBlockChange(handler)
```

- 功能、参数、返回与约束：handler(event)，UpdateBlock 导出 position/runtime_id/previous/cause_known=false；不能仅从方块更新认定破坏者。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onBotReady`

```lua
events.onBotReady(handlerName)
```

- 作用：注册机器人就绪事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onBotReady()`——0 参。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evBotReady, L)`。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onChat`

```lua
events.onChat(handlerName)
```

- 作用：注册聊天事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onChat(player, msg)`——触发传 2 参：`player` string（玩家名）、`msg` string（聊天内容）。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evChat, L)`；触发 `fireChatEvent` → `runEventHandlers`。实时。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onDeath`

```lua
events.onDeath(handler)
```

- 功能、参数、返回与约束：handler 回调接收一个事件对象，来源为 ActorEventDeath/InstantDeath 或 health<=0；同一已死亡状态去重，重生后可再触发。不推测死亡原因/攻击者。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onEntityRemove`

```lua
events.onEntityRemove(handler)
```

- 功能、参数、返回与约束：handler(event)；RemoveActor下发时触发，含unique_id和已知runtime_id。消失不等于死亡，也可能是离开视距。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onEntitySpawn`

```lua
events.onEntitySpawn(handler)
```

- 功能、参数、返回与约束：handler(event)；AddPlayer/AddActor/AddItemActor下发时触发，含实体类型、位置和原始fields。服务器视距内的出现，不代表世界内首次出生；血量仅取真实同步属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onHealthChange`

```lua
events.onHealthChange(handler)
```

- 功能、参数、返回与约束：handler(event)，仅在观测血量初始值或变化时入队，含 previous/health/max_health/time/source。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onHurt`

```lua
events.onHurt(handler)
```

- 功能、参数、返回与约束：handler(event)，来源为血量下降或 ActorEventHurt，两种独立来源可能各触发一次；无法识别准确攻击者，不用编造 cause。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onItemPickup`

```lua
events.onItemPickup(handler)
```

- 功能、参数、返回与约束：handler(event)；TakeItemActor下发时触发，含拾取实体和掉落物runtime_id、已观测item/position。拾取者可为生物；不凭RemoveActor猜拾取。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onJoin`

```lua
events.onJoin(handlerName)
```

- 作用：注册玩家加入事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onJoin(player)`——1 参 `player` string。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evJoin, L)`；由后台 5 秒玩家列表轮询差量触发（有约几秒延迟）。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onLeave`

```lua
events.onLeave(handlerName)
```

- 作用：注册玩家离开事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onLeave(player)`——1 参 `player` string。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evLeave, L)`；由 5 秒轮询差量触发。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onMove`

```lua
events.onMove(handler)
```

- 功能、参数、返回与约束：handler(event)，服务器实际位置初始值/变化时触发；含 previous_known/position/dimension/source。高频事件可用抓包抽样过滤减少负担。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onPacket`

```lua
events.onPacket(handlerName)
```

- 作用：注册数据包事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onPacket(type, dataTable)`——2 参：`type` string（包类型）、`dataTable` table（数据）。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evPacket, L)`；触发 `firePacketEvent`，数据经 `goToLua` 转表。实时。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onPlayerDeath`

```lua
events.onPlayerDeath(handler)
```

- 功能、参数、返回与约束：handler(event)；只接受玩家死亡；来自真实ActorEvent/血量归零/网易ModEventS2C OnPlayerDie，附已知最后位置、维度、timestamp；没有位置保持position_known=false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onRespawn`

```lua
events.onRespawn(handler)
```

- 功能、参数、返回与约束：handler(event)，只处理服务端 RespawnStateReadyToSpawn，清除旧血量等待新属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onSwing`

```lua
events.onSwing(handler)
```

- 功能、参数、返回与约束：handler(event)；服务器AnimateActionSwingArm下发时触发，含runtime_id/player/source。挥臂不证明命中了目标，服务器未广播则无法观测。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.packetSchema`

```lua
events.packetSchema()
```

- 功能、参数、返回与约束：无参数。返回当前内核支持包类型、协议 ID、顶层字段名与类型，供 AI 根据实际协议找包而非猜字段。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribe`

```lua
events.subscribe(packetType, handlerName)
```

- 作用：**通用数据包订阅**。订阅任意一个数据包类型（按 Go 类型名，如 `"MovePlayer"`/`"SetTime"`/`"PlayerList"`/`"Text"`）。订阅后机器人只监听你订阅的包类型，收到就把**完整**数据包广播给该 handler。
- 参数：`packetType` string 必填（包类型名）；`handlerName` string 必填，回调签名 `onData(dataTable)`——1 参：`dataTable` table（包数据，字段名 snake_case；`[]byte` 给**完整 hex**，不截断）。
- 返回：无。同插件同类型同 handler 自动去重；重跑上下文跳过注册。
- 报错：参数非 string 抛类型错误；不在插件上下文调用报错。
- 底层：`registerPacketSub` → `packetHubSubscribe`（packet_hub.go）。想查有哪些包类型可订阅，可用 AI 的 `list_packets` 工具。清理：插件停止时 `RemovePluginEvents` 一并退订。
- 示例：
```lua
events.subscribe("MovePlayer", "onMove")
function onMove(data)
  -- data.entity_runtime_id, data.position={x,y,z}, data.yaw ...
  game.sendChat("@a", "玩家移动 x=" .. data.position[1])
end
```
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribeAll`

```lua
events.subscribeAll(handlerName)
```

- 作用：全量入站包订阅，回调接收一个包信封。
- 参数：`handlerName`；按此顺序传入。
- 返回：Lua无返回；SDK注册成功true；停止自动退订。
- 底层与游戏效果：信封含seq/at/direction/bot/packet_type/packet_id/fields。只序列化有订阅的包；全包模式会含高频及大区块数据，正式插件优先subscribe或subscribeFiltered；队列有界，dropped可查。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribeFiltered`

```lua
events.subscribeFiltered(packet_type, filter, handler)
```

- 功能、参数、返回与约束：packet_type 或 *，filter={fields:{字段路径:精确值},every_ms:0..60000}，handler。Python 回调接收包对象；C++/Lua 使用已注册函数名。* 接收当前账号订阅包，事件字段 _packet_type 表示真实类型，停止自动退订。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureSummary`

```lua
events.captureSummary(options)
```

- 功能、参数、返回与约束：options同events.summary；抓包计数与最近元数据，返回overwritten/sampled_out。抓包必须先开启；计数范围为当前保留环，不是全服。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.catalog`

```lua
events.catalog()
```

- 功能、参数、返回与约束：无参数。列出真实事件类型、来源及范围；不保证未下发的数据。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.define`

```lua
events.define(definition, handler)
```

- 功能、参数、返回与约束：definition为插件私有JSON映射：{name,packet,filter:{fields:{...},every_ms:0},fields:{输出字段:包字段路径}}。仅映射已解码的单一包，不执行代码、不伪造可信度；停止插件自动退订。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.moduleHistory`

```lua
events.moduleHistory(options)
```

- 功能、参数、返回与约束：options={session?,after?,limit?}；当前机器人规范模块战斗事件独立历史与module_sequence游标，避免被移动事件挤掉。limit默认256、1..4096；容量沿用diagnostics.configure的event_history，重连reset，覆盖lost；不含乐器高频事件。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.on`

```lua
events.on(kind, handler)
```

- 功能、参数、返回与约束：kind,handler；订阅宿主统一事件，kind从events.catalog查询；无需开启抓包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onContainerChange`

```lua
events.onContainerChange(handler)
```

- 功能、参数、返回与约束：handler(event)；真实BlockActorData容器类型变化，block.container_known/is_container/block_entity_id；不返回物品NBT、不把变化当作玩家放置证明。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onDamageDisplay`

```lua
events.onDamageDisplay(handler)
```

- 功能、参数、返回与约束：handler(event)；已验证BREquipDamageEventServer/GetBroadcastData模块伤害显示；target.unique_id、damage_text、position与position_known。数字为显示文字，不是确认的HP扣血；attacker_known=false、dimension_known=false，不改血量缓存。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onDimensionChange`

```lua
events.onDimensionChange(handler)
```

- 功能、参数、返回与约束：handler(event)；服务器ChangeDimension，旧维度实体数据标为过期。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onEntityHurt`

```lua
events.onEntityHurt(handler)
```

- 功能、参数、返回与约束：handler(event)；真实掉血优先；ActorEventHurt等50ms关联UpdateAttributes，未同步血量时damage_known=false；hurt_signal仅诊断，不计作另一伤害。含target、damage/health/max_health、attacker_known=false（无可靠攻击者包时），event_id。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onEntityKill`

```lua
events.onEntityKill(handler)
```

- 功能、参数、返回与约束：handler(event)；接收已验证Minecraft/vipEventSystem/PlayerKillMob<UUID>服务器模块击杀报告；攻击者与目标UniqueID精确字符串，名字仅由在线PlayerList关联。evidence=server_module_reported；不是PvP onPlayerKill，不补造血量/RuntimeID，模块未广播则不可用。同会话有界缓存按来源/攻击者/目标去重。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onEquipmentChanged`

```lua
events.onEquipmentChanged(handler)
```

- 功能、参数、返回与约束：handler(event)；服务器装备包即时缓存更新，含已更新槽位，不同步查询玩家背包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onGlobalHealthChange`

```lua
events.onGlobalHealthChange(handler)
```

- 功能、参数、返回与约束：handler(event)；仅模块health_changed有有限current血量和entity_id时触发；max未知保留null；与entities.health协议缓存分开。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onGlobalSwing`

```lua
events.onGlobalSwing(handler)
```

- 功能、参数、返回与约束：handler(event)；仅收到转发模块swing事件且有明确player_id时触发；不把命中攻击当空挥，不扫描最近玩家。global表示转发模块不按机器人视距过滤，不承诺模块已安装。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onHealthChanged`

```lua
events.onHealthChanged(handler)
```

- 功能、参数、返回与约束：同onHealthChange，含previous及真实max，未知max=null。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onObservedAction`

```lua
events.onObservedAction(handler)
```

- 功能、参数、返回与约束：handler；统一接收swing/hurt/equipment/rotation/entity_kill/damage_display。真实挥臂只来自swing；伤害和击杀不能冒充空挥。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onPlayerKill`

```lua
events.onPlayerKill(handler)
```

- 功能、参数、返回与约束：handler(event)；可信服务器death.attack翻译包提供击杀者且关联已知玩家，唯一death event_id，仅触发一次；同次death_details补充不重复计分。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onPlayerRespawn`

```lua
events.onPlayerRespawn(handler)
```

- 功能、参数、返回与约束：handler(event)；与session_join独立，死亡到存活/RespawnReady的生命周期去重。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onRotation`

```lua
events.onRotation(handler)
```

- 功能、参数、返回与约束：handler(event)；服务器角度字段变化。旋转包即使位置不变也刷新时间。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onServerObservation`

```lua
events.onServerObservation(handler)
```

- 功能、参数、返回与约束：handler(event)；YouyuanBridge/ObserverV1/Observation实际下发时回调，含模块事件、精确实体ID、收到时间和证据；需安装主动转发模块，OP不自动安装。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onSessionJoin`

```lua
events.onSessionJoin(handler)
```

- 功能、参数、返回与约束：handler(event)；真实PlayerList新增条目，不能把AddPlayer视距出现当进服。服务器不广播PlayerList则无法直接观测。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.onSessionLeave`

```lua
events.onSessionLeave(handler)
```

- 功能、参数、返回与约束：handler(event)；真实PlayerList移除条目，不把RemoveActor离开视距当退服。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `events.summary`

```lua
events.summary(options)
```

- 功能、参数、返回与约束：options={after_sequence,limit:20,types:[],entity_type,runtime_id}；只读规范事件计数和最近轻量条目，不展开装备/背包；完整事件用明确view=raw或事件游标接口。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# packet · 原始协议包

schema/describe查询字段，on/onAll监听，send请求服务器执行；不会取得服务器未下发的数据。


### `packet.describe`

```lua
packet.describe(typeNameOrId)
```

- 作用：查询一个包的实际ID和字段类型。
- 参数：`typeNameOrId`；按此顺序传入。
- 返回：name/id/fields对象；未知包抛错。
- 底层与游戏效果：可离线查询。联合类型的具体分支不能仅靠顶层schema构造，请读对应协议章节。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `packet.on`

```lua
packet.on(packetType, handlerName)
```

- 作用：events.subscribe的别名，回调接收字段对象。
- 参数：`packetType, handlerName`；按此顺序传入。
- 返回：注册结果。
- 底层与游戏效果：精确类型订阅，fields里_packet_type为真实类型。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `packet.onAll`

```lua
packet.onAll(handlerName)
```

- 作用：events.subscribeAll的别名。
- 参数：`handlerName`；按此顺序传入。
- 返回：注册结果。
- 底层与游戏效果：回调接收包信封，fields包含实际包字段。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `packet.schema`

```lua
packet.schema()
```

- 作用：查询当前编译内核的全部已注册协议包。
- 参数：`无`；按此顺序传入。
- 返回：包数组，每项name/id/fields。
- 底层与游戏效果：从真实packet.ListAllPackets枚举，不根据网上版本猜包ID。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `packet.send`

```lua
packet.send(typeNameOrId, fieldsTable)
```

- 作用：game.sendPacket的原始协议入口别名。
- 参数：`typeNameOrId, fieldsTable?`；按此顺序传入。
- 返回：提交状态，未确认。
- 底层与游戏效果：同game.sendPacket。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

# bot · 机器人控制与路由

选择舰队机器人、状态、输入移动、看向坐标、近战攻击。无自动寻路保证。


### `bot.attack`

```lua
bot.attack(runtime_id, slot)
```

- 功能、参数、返回与约束：runtime_id 字符串,slot=0；真实近战库存交易，只允许当前服务器观测的六格内实体；slot 为 0..8。返回 submitted，伤害必须由服务器事件确认。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.count`

```lua
bot.count()
```

- 作用：返回在线机器人数量。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：`GetFleetBots()`。

### `bot.current`

```lua
bot.current()
```

- 作用：返回当前活跃机器人的名字与坐标。
- 参数：无。
- 返回：table `{name, x, y, z}`；无活跃机器人返回空表；坐标查询失败省略 x/y/z。
- 报错：无。
- 底层：从舰队管理器读当前选中的机器人名，再通过 WS 自动化通道执行 `querytarget @s` 取该机器人的坐标。

### `bot.list`

```lua
bot.list()
```

- 作用：列出在线机器人及坐标。
- 参数：无。
- 返回：table，1 起索引，每个元素 `{name, x, y, z}`（坐标查询失败省略坐标字段）。
- 报错：无。
- 底层：`GetFleetBots()` 遍历 + `QueryPosition()`。

### `bot.lookAt`

```lua
bot.lookAt(x, y, z)
```

- 功能、参数、返回与约束：x,y,z 世界坐标。由已观测机器人位置计算 yaw/pitch，启用心跳发送视角；返回请求状态，无自动寻路/穿墙保证。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.select`

```lua
bot.select(nameOrIndex)
```

- 作用：切换后续 `game.*` / `world.*` 调用的目标机器人。
- 参数：`nameOrIndex` string 必填（机器人名或索引的字符串形式）。
- 返回：boolean `true`。
- 报错：未找到机器人抛 `bot.select: 未找到机器人 <名称>`。
- 底层：`selectBot(L, sel)`。

### `bot.self`

```lua
bot.self()
```

- 功能、参数、返回与约束：无参数。读取当前账号机器人的服务器观测状态，health/position/dead 可未知。AI Agent 使用此机器人，不自行拥有一个游戏位置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.startMovement`

```lua
bot.startMovement(dx, dy, dz)
```

- 功能、参数、返回与约束：dx,dy,dz 玩家输入方向，每轴 -1..1；以 50ms PlayerAuthInput 心跳交给服务器驱动运动。dx 左右，dz=-1 前进，dy 上下，不是世界坐标路径规划；返回 submitted=true,confirmed=false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.status`

```lua
bot.status()
```

- 作用：返回每个机器人的连接状态、是否 OP、错误信息。
- 参数：无。
- 返回：table，1 起索引，每个元素 `{index, name, connected, is_op, error?}`（error 仅非空时存在）。
- 报错：无。
- 底层：`FleetStatus()`。

### `bot.stopMovement`

```lua
bot.stopMovement()
```

- 功能、参数、返回与约束：无参数。清理当前机器人移动输入，仍需要服务器回包确认最终位置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.teleport`

```lua
bot.teleport(x, y, z)
```

- 功能、参数、返回与约束：x,y,z 世界坐标。执行服务器 tp @s 并等待真实命令结果，需要对应权限；没有权限时报错或 success=false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.requestViewDistance`

```lua
bot.requestViewDistance(chunks, timeout_ms)
```

- 功能、参数、返回与约束：chunks=1..2147483647协议整数，不附加32区块上限。发送RequestChunkRadius（同时设置MaxChunkRadius）并等待服务器ChunkRadiusUpdated；服务器可调小或不回复。confirmed仅表示收到半径响应，不保证远方血量/挥臂广播；超时返回confirmed=false，不冒充生效。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `bot.swingAir`

```lua
bot.swingAir()
```

- 功能、参数、返回与约束：无参数；在下一正常PlayerAuthInput心跳提交一次MissedSwing，复用当前位置/视角与递增输入Tick，不发旧式Animate。重复未发送请求合并；submitted=false表示仅排队；服务器广播/处理另行观察。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# chains · 命令链管理

同一套TXT预解析、整链频率、启停、跨链与聊天菜单，可供插件调用。


### `chains.configure`

```lua
chains.configure(name, patch)
```

- 功能、参数、返回与约束：name,patch，合并并校验配置，保留 TXT；运行中会停止并按 enabled 重启。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.delete`

```lua
chains.delete(name)
```

- 功能、参数、返回与约束：name，先停止再删除 TXT 与配置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.export`

```lua
chains.export(name)
```

- 功能、参数、返回与约束：name，返回完整 TXT 与配置，插件可自行写入导出目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.get`

```lua
chains.get(name)
```

- 功能、参数、返回与约束：name，读取完整 TXT、配置和状态。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.handleChat`

```lua
chains.handleChat(player, text)
```

- 功能、参数、返回与约束：player,text；处理 .命令选择 与 .链 列表/启动/停止/次数/频率/通道/源码/删除。修改要求玩家 OP，返回 handled。配合 events.onChat。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.list`

```lua
chains.list()
```

- 功能、参数、返回与约束：无参数，列出当前账号的链、配置、计数、错误和状态。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.menu`

```lua
chains.menu(player, page)
```

- 功能、参数、返回与约束：player,page=1，仅向该玩家发分页聊天菜单；不会阻塞等待消息。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.reload`

```lua
chains.reload(name)
```

- 功能、参数、返回与约束：name；只重新读取/编译TXT资源，当前轮使用原IR，下轮用新IR；不重启插件、保留启停与统计；无效修改保留原链。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.save`

```lua
chains.save(name, content, config)
```

- 功能、参数、返回与约束：name,content,config；config 支持 interval_ms(至少1，整链周期)/delay_ms/run_count/channel/enabled/mode/window/rate_cps。mode=fast无等待提交（条件及其前条确认）、remote严格顺序、pipeline有界流水线、burst整批最多1024、stream动态滑动窗口1..1024（默认128，rate_cps=0不设固定CPS，回包立即补位，网络错误停止补发；正常无目标不丢弃无条件后续）；全部在机器人内运行。TXT支持 @call JSON 跨链源文件行号范围/次数/时限/持续，@wait JSON 条件边沿等待，@pause手动触发；读 command-chain 技能。账号固定为当前插件账号。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.start`

```lua
chains.start(name)
```

- 功能、参数、返回与约束：name，启用独立常驻实例；已启用不重复启动。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.stop`

```lua
chains.stop(name)
```

- 功能、参数、返回与约束：name，停止并等待当前实例退出。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.trigger`

```lua
chains.trigger(name)
```

- 功能、参数、返回与约束：name，释放当前链的 @pause 等待点一次；没有等待点时不会自动重启已停止的链。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.validate`

```lua
chains.validate(content)
```

- 功能、参数、返回与约束：content，预解析并返回命令/条件/通道/行号；失败含报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

# building · 建筑与颜色

结构格式检测、解析和方块颜色匹配。这些解析函数不等于放置整个建筑。


### `building.closestColor`

```lua
building.closestColor(r, g, b)
```

- 作用：按 CIELAB 感知色差，匹配给定 RGB 最接近的颜色方块。
- 参数：`r`/`g`/`b` number 必填（0-255）。
- 返回：string（最接近的颜色方块名）。
- 报错：非数值抛类型错误。
- 底层：`media.ClosestBlockForColor(r, g, b)`。

### `building.detect`

```lua
building.detect(path)
```

- 作用：检测建筑文件的格式。
- 参数：`path` string 必填。
- 返回：string（格式名：`.mcstructure` / `.schematic` / `.mcworld` / `.bdx`）。
- 报错：非字符串抛类型错误。
- 底层：`building.DetectFormat(path)`。

### `building.isAir`

```lua
building.isAir(name)
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isCommandBlock`

```lua
building.isCommandBlock(name)
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isLava`

```lua
building.isLava(name)
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isWater`

```lua
building.isWater(name)
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.parse`

```lua
building.parse(path)
```

- 作用：加载并解析建筑结构文件，返回尺寸、方块统计、NBT、是否含命令方块。
- 参数：`path` string 必填。
- 返回：table `{size={x,y,z}, blocks=number, nbt=number, has_command=boolean}`（`blocks` 为非空气方块数，`nbt` 为 NBT 方块数）。
- 报错：加载失败抛 `building.parse: <err>`。
- 底层：`building.LoadStructureFile(path)`，遍历方块计数，读 `NBTBlocks`/`HasCommand`。

### `building.containerData`

```lua
building.containerData(name, items, options)
```

- 功能、参数、返回与约束：容器名,Items数组,options?；生成经过类型/槽位检查的方块NBT，不连接游戏。options支持custom_name。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.exportRegion`

```lua
building.exportRegion(start, size, options)
```

- 功能、参数、返回与约束：起点,尺寸,{workers:1}?；异步按区块读取并保存mcstructure。只使用已配置的同服Fleet机器人，失败不输出缺块文件。返回job_id，用nbtJob查询本机文件路径。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.importLimits`

```lua
building.importLimits(limits)
```

- 功能、参数、返回与约束：{x?,z?}；返回常加载官方上限和当前导入器实际粒度；不执行游戏命令，不代表当前服务器已验收。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.nbtCapabilities`

```lua
building.nbtCapabilities()
```

- 功能、参数、返回与约束：无参数；实际底层支持的方块/特殊物品及制作行为目录。支持表不是全服实测保证。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.nbtJob`

```lua
building.nbtJob(job_id)
```

- 功能、参数、返回与约束：job_id；读取当前账号的NBT任务状态；submitted与目标结构读回verified区分。任务内存留存，重启不恢复。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.normalizeNBT`

```lua
building.normalizeNBT(nbt)
```

- 功能、参数、返回与约束：NBT对象；按Bedrock物品字段恢复Count/Slot/ench/Damage等NBT类型，拒绝越界，保留未知字段。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.placeContainer`

```lua
building.placeContainer(x, y, z, name, items, options)
```

- 功能、参数、返回与约束：x,y,z,容器名,Items数组,options?；异步使用真实NBT工作台制作、铁砧改名、附魔及结构复制；返回job_id，用nbtJob查验证结果。会移动机器人、初始化时清背包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.placeSkull`

```lua
building.placeSkull(x, y, z, skullType, states)
```

- 功能、参数、返回与约束：x,y,z,SkullType(0..6),states?；异步通过真实头颅物品放置和结构缓存复制；返回job_id，使用nbtJob读取实际SkullType。会使用NBT工作台并移动机器人。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.planRegion`

```lua
building.planRegion(start, size)
```

- 功能、参数、返回与约束：{x,y,z},{sx,sy,sz}；按16x128x16分块，返回不重叠完整覆盖的单元。规划不等于执行。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.resumeImport`

```lua
building.resumeImport(task_id)
```

- 功能、参数、返回与约束：task_id；按保留的原坐标、文件校验和分区布局恢复。文件改变或机器人不就绪明确报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.startImport`

```lua
building.startImport(options)
```

- 功能、参数、返回与约束：{path,x,y,z,speed,dimension?,rotation?,region_size_chunks?,preload_size_chunks?,ticking_preload?,multi?,bot_names?}；region_size_chunks=0沿用旧档位，1..10为单机每批边长；preload_size_chunks=0沿用批次，或选择批次整数倍的大窗口复用一个常加载区。10需要X/Z对齐16。multi当前按单区块单元分工。返回提交状态；用taskList查进度。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.stopImport`

```lua
building.stopImport()
```

- 功能、参数、返回与约束：无参数；请求停止当前账号作业，等待清理和断点落盘期间禁止启动重叠作业。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `building.taskList`

```lua
building.taskList()
```

- 功能、参数、返回与约束：无参数；列出当前账号/当前服务器的真实作业断点、原参数、进度和状态。running不等于服务器已确认所有方块。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# media · 图片与音乐

图片字符画、MIDI解析、音高换算；解析函数本身不自动向游戏播放。


### `media.imageToAscii`

```lua
media.imageToAscii(path, maxW)
```

- 作用：把图片文件转成 ASCII 字符画行数组。
- 参数：`path` string 必填；`maxW` number 可选，默认 60。
- 返回：table（字符串数组，1 起索引，每行一个 ASCII 串）。
- 报错：打开/解码/转换失败抛 `media.imageToAscii: <err>`。
- 底层：`os.Open` + `image.Decode`（支持 gif/jpeg/png）→ `media.ImageToASCII(img, maxW)`。

### `media.noteToPitch`

```lua
media.noteToPitch(note)
```

- 作用：把 MIDI 音符号换算为频率（Hz）。
- 参数：`note` number 必填（强转 `uint8`）。
- 返回：number（音高频率）。
- 报错：非数值抛类型错误。
- 底层：`media.NoteToPitch(uint8(note))`。

### `media.parseMidi`

```lua
media.parseMidi(path)
```

- 作用：解析 MIDI 文件，返回格式/轨道/分辨率/速度及音符列表。
- 参数：`path` string 必填。
- 返回：table `{format, tracks, division, tempo, notes={{tick, channel, note, velocity, duration, sound}}}`（`sound` 仅非空时出现）。
- 报错：读文件/解析失败抛 `media.parseMidi: <err>`。
- 底层：`os.ReadFile` → `media.ParseMIDIFile(data)`。

# plugin · 实例资源与诊断

目录、资源、能力与统计；代码、配置、持久数据分开管理。


### `plugin.capabilities`

```lua
plugin.capabilities()
```

- 功能、参数、返回与约束：无参数，当前平台可用API名称和语言运行器能力；按需读取，不自动整库塞AI上下文。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.codeDir`

```lua
plugin.codeDir()
```

- 功能、参数、返回与约束：无参数，当前插件源码根目录。Lua模块可require，模块只在该常驻VM首次require时加载。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.configDir`

```lua
plugin.configDir()
```

- 功能、参数、返回与约束：无参数，当前插件配置目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.dataDir`

```lua
plugin.dataDir()
```

- 功能、参数、返回与约束：无参数，当前插件持久数据目录；SDK按账号隔离。旧game.dataDir保持兼容。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.dependencies`

```lua
plugin.dependencies()
```

- 作用：返回当前插件的依赖声明（`pre_plugins`）。
- 参数：无。
- 返回：表（原样透传全局 `plugin_dependencies` 的值）。
- 报错：无。
- 底层：读全局 `plugin_dependencies`。

### `plugin.existsResource`

```lua
plugin.existsResource(relative)
```

- 功能、参数、返回与约束：relative_path，检查资源是否存在；非法路径报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.extractResource`

```lua
plugin.extractResource(source, destination)
```

- 功能、参数、返回与约束：source_relative,destination_relative默认同源；将资源原字节复制至plugin.dataDir中的目标，创建父目录。不是解压ZIP，不写任意外部路径。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.has`

```lua
plugin.has(method)
```

- 功能、参数、返回与约束：method或runtime.lua/runtime.python/runtime.cpp，查询宿主API是否提供；不代表服务器一定支持动作或已观测到血量。Linux C++当前false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.id`

```lua
plugin.id()
```

- 作用：返回当前插件的 id 字符串。
- 参数：无。
- 返回：string。
- 报错：无。
- 底层：读全局 `__PLUGIN_ID__`。

### `plugin.listResources`

```lua
plugin.listResources(relative)
```

- 功能、参数、返回与约束：relative_path默认.，递归列出目录和文件，含path/dir/size；不跟随符号链接。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.readResource`

```lua
plugin.readResource(relative, encoding)
```

- 功能、参数、返回与约束：relative_path,encoding=utf8；读取资源字符串；二进制跨JSON使用encoding=base64。拒绝绝对路径、越界与符号链接。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.resourceDir`

```lua
plugin.resourceDir()
```

- 功能、参数、返回与约束：无参数；有resources子目录则使用该目录，否则兼容使用代码目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.stats`

```lua
plugin.stats()
```

- 功能、参数、返回与约束：无参数，当前常驻实例回调数、失败、墙钟耗时及队列深度/丢弃；CPU/内存/GC当前未逐实例测量，返回null，不虚构性能倍数。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.tempDir`

```lua
plugin.tempDir()
```

- 功能、参数、返回与约束：无参数；插件数据目录下.tmp，可自行删除临时文件。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.call`

```lua
plugin.call(pluginID, function, args)
```

- 功能、参数、返回与约束：pluginID,exportedFunction,args数组；调用当前账号正在运行的常驻插件plugin_api导出函数，不重跑源码、不自动启用已停用插件。目标正在执行/循环调用明确报错，参数结果保留结构。MCP可经game/call调用。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `plugin.exportFile`

```lua
plugin.exportFile(relative_file)
```

- 功能、参数、返回与约束：relative_file；取得当前插件生成文件的下载信息，保持原格式/原字节，不是备份整个插件。文件须已写入plugin.dataDir。链接需登录当前账号，snapshot=false表示下载当前文件版本。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `plugin.listOutputs`

```lua
plugin.listOutputs(relative_dir)
```

- 功能、参数、返回与约束：relative_dir默认.；列出当前插件数据目录内生成的文件，返回path/name/bytes/modified_at/download_url，不返回文件内容。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# util · 文件、时间与调度

JSON文件、文本、数学、时间与整个回调的定时任务。


### `util.cancel`

```lua
util.cancel(id)
```

- 作用：取消一个已注册的定时任务（schedule/setTimeout）。
- 参数：`id` number 必填。
- 返回：boolean——`true` 找到并取消，`false` 不存在。
- 报错：无。
- 底层：`cancelSchedJob(int64(id))`，关闭任务的 cancel channel。

### `util.ceil`

```lua
util.ceil(n)
```

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（math.Round：正负0.5均远离0取整）。

### `util.floor`

```lua
util.floor(n)
```

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（math.Round：正负0.5均远离0取整）。

### `util.join`

```lua
util.join(tbl, sep)
```

- 作用：把 table 的所有值转字符串后用分隔符拼接。注意：遍历整表，非数组顺序不一定与下标一致。
- 参数：`tbl` table、`sep` string 必填。
- 返回：string。
- 报错：无。
- 底层：`t.ForEach` + `LVAsString` + `strings.Join`。

### `util.json_load`

```lua
util.json_load(path)
```

- 作用：读取文件并解析为 JSON 对象。
- 参数：`path` string 必填。
- 返回：任意 Lua 值（表/数字/字符串…）。数字用 `UseNumber()` 解码避免精度损失。
- 报错：读文件失败或 JSON 解析失败抛 `json_load: <err>`。
- 底层：`os.ReadFile` + `json.Decoder`（`UseNumber`）→ `goToLua`。

### `util.json_write`

```lua
util.json_write(path, data)
```

- 作用：把任意 Lua 值序列化为带缩进的 JSON 写入文件，自动建父目录。
- 参数：`path` string 必填；`data` 任意类型必填。
- 返回：boolean `true`。
- 报错：建目录/序列化/写文件失败抛 `json_write: <err>`。
- 底层：`os.MkdirAll` + `json.MarshalIndent(data,"","  ")` + `os.WriteFile`。

### `util.listJobs`

```lua
util.listJobs()
```

- 功能、参数、返回与约束：无参数，当前插件任务列表：id/handler/paused/calls/missed/failures/last_ms/max_ms/total_ms/last_error；耗时为墙钟含VM排队及回调，不是CPU时间。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `util.now`

```lua
util.now()
```

- 作用：返回当前本地时间。
- 参数：无。
- 返回：string，格式固定 `"2006-01-02 15:04:05"`。
- 报错：无。
- 底层：`time.Now().Format("2006-01-02 15:04:05")`。

### `util.pauseJob`

```lua
util.pauseJob(id)
```

- 功能、参数、返回与约束：id，暂停该插件任务后续触发；已经执行/入队的一次不强制中断。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `util.pos`

```lua
util.pos(str)
```

- 作用：把空格分隔的坐标字符串（如 `"100 64 200"`）解析成坐标表，最多取前 3 个 token，缺失项补 0。
- 参数：`str` string 必填。
- 返回：table `{x=number, y=number, z=number}`。
- 报错：无。
- 底层：`strings.Fields` + `strconv.ParseFloat`。

### `util.read_file`

```lua
util.read_file(path)
```

- 作用：读取文件全部内容按字符串返回。
- 参数：`path` string 必填。
- 返回：string。
- 报错：读取失败抛 `read_file: <err>`。
- 底层：`os.ReadFile`。

### `util.resumeJob`

```lua
util.resumeJob(id)
```

- 功能、参数、返回与约束：id，恢复后续触发；不会补发暂停期间的积压轮次。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `util.round`

```lua
util.round(n)
```

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（math.Round：正负0.5均远离0取整）。

### `util.schedule`

```lua
util.schedule(intervalSec, handlerName)
```

- 作用：按秒调用整个回调，周期0.001秒至1年。
- 参数：`intervalSec, handlerName`；按此顺序传入。
- 返回：定时任务ID；util.cancel(id)取消。
- 底层与游戏效果：源码与模块缓存复用；同任务不重入，不补无限积压。回调里整条链的所有命令连续提交，不是每次只发一条。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.scheduleHz`

```lua
util.scheduleHz(hz, handler)
```

- 功能、参数、返回与约束：hz,handler；本地完整回调的频率，0<hz<=1000。慢回调不重入、不补积压轮次；统一时钟、skip策略；查看listJobs的missed。不是服务端同步Tick。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.scheduleSeconds`

```lua
util.scheduleSeconds(seconds, handler)
```

- 功能、参数、返回与约束：seconds,handler；schedule别名，0.001秒..一年。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.scheduleTick`

```lua
util.scheduleTick(ticks, handler)
```

- 功能、参数、返回与约束：ticks,handler；ticks>=1，统一宿主Tick Loop；本地名义Tick=50ms，不宣称服务端同Tick同步。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.setTimeout`

```lua
util.setTimeout(sec, handlerName)
```

- 作用：延时后调用一次回调，可设0秒。
- 参数：`sec, handlerName`；按此顺序传入。
- 返回：定时任务ID。
- 底层与游戏效果：只调用已加载的函数，停用或重载自动取消所属定时任务。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.split`

```lua
util.split(str, sep)
```

- 作用：按分隔符切分字符串为数组。
- 参数：`str` string、`sep` string 必填。
- 返回：table（字符串数组，1 起索引）。
- 报错：无。
- 底层：`strings.Split` + `RawSetInt`。

### `util.strip_color`

```lua
util.strip_color(text)
```

- 作用：去掉文本里的 Minecraft 颜色/格式码（`§` 与 `&` 及其后一个字符）。
- 参数：`text` string 必填。
- 返回：string。
- 报错：无。
- 底层：手写循环用 `strings.Builder` 逐字节过滤。

### `util.time_fmt`

```lua
util.time_fmt(ts)
```

- 作用：把 Unix 时间戳格式化为可读的本地时间。
- 参数：`ts` number 可选，默认当前时间戳。
- 返回：string。
- 报错：无。
- 底层：`time.Unix(ts,0).Format("2006-01-02 15:04:05")`。

### `util.timestamp`

```lua
util.timestamp()
```

- 作用：返回当前 Unix 秒级时间戳。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：`time.Now().Unix()`。

### `util.trim`

```lua
util.trim(str)
```

- 作用：去除字符串两端空白。
- 参数：`str` string 必填。
- 返回：string。
- 报错：无。
- 底层：`strings.TrimSpace`。

### `util.write_file`

```lua
util.write_file(path, content)
```

- 作用：把字符串写入文件，自动建父目录，权限 0644。
- 参数：`path` string、`content` string 必填。
- 返回：boolean `true`。
- 报错：建目录/写文件失败抛 `write_file: <err>`。
- 底层：`os.MkdirAll` + `os.WriteFile(path, []byte(content), 0644)`。

### `util.monotonicMs`

```lua
util.monotonicMs()
```

- 功能、参数、返回与约束：无参数，进程启动以来单调时间毫秒，适合间隔计算；不是Unix时间。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `util.serverTick`

```lua
util.serverTick()
```

- 功能、参数、返回与约束：返回known/tick/source/updated_at；仅观测UpdateAttributes的真实非零tick，不以本地50ms计时冒充服务器tick。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# api · 跨插件调用

通过plugin_api导出函数；Lua、Python、C++可相互调用已启动实例。


### `api.require`

```lua
api.require(id)
```

- 作用：获得跨插件调用代理；目标需已经加载并导出plugin_api。
- 参数：`id`；按此顺序传入。
- 返回：Lua代理表或nil；Python代理对象；C++yy::RemotePlugin。
- 底层与游戏效果：Lua→Lua调用同一常驻实例，不重跑源码；跨语言通过序列化桥调用目标导出函数。API不是安装依赖的包管理器。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

## api.call 可直达的附加方法

下面 7 个方法在宿主 `yyExtraMethods` 里有真实实现，但没有登记成独立命名空间，因此不能写成 `seed.xxx()`；插件里只能经 `api.call("<方法名>", {...})`（Python `host.api.call`，C++ `client.api.call`）或 HTTP `/api/youyuan/game/call` 调用。它们不在命名空间注册表里，`api.catalog` 的命名空间列表不会列出。

| 方法 | 说明 |
|---|---|
| `seed.manifest` | 无参数；返回版本表、维度、结构清单与引擎自检（btree 是否装载）。不联网、不读用户文件。 |
| `seed.structures` | {seed,version=1.21.4,edition=java,dim=overworld,types='village,mansion',cx=0,cz=0,r=256,limit=20000}；返回区块坐标的结构点。precision=exact 的结构可复现，candidate 的需要进游戏确认。范围过大自动截断并给 truncated=true。 |
| `seed.biome` | {seed,version,cx,cz} 或 {seed,version,points:[[cx,cz],...]}；返回生物群系 id/key/中文名/RGB。仅主世界走气候噪声，下界/末界给代表群系并注明。 |
| `seed.slime` | {seed,cx=0,cz=0,r=16}；Java 版史莱姆区块，exact。基岩版算法不同，明确拒绝而不是给错结果。 |
| `seed.stronghold` | {seed,version,count=128}；要塞区块坐标，cubiomes 逐位复刻，exact。 |
| `seed.summary` | {seed,version,dim,cx,cz,r}；首屏摘要 + 最近结构，给面板/机器人回话用。 |
| `packet.catalog` | 无参数；本版本所有注册包名/ID/字段数/能否抓包，单包字段用 packet.schema 或 /api/youyuan/packets/schema?name=Animate；schema 存在不代表服务器广播。 |

<!-- 2026-10-26 补写：源码注册面同步 -->

# http · 网络与JSON

返回status/body/headers，非2xx响应也可检查，不把远端错误吞掉。


### `http.delete`

```lua
http.delete(url, opts)
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.get`

```lua
http.get(url, opts)
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.json_decode`

```lua
http.json_decode(s)
```

- 作用：把 JSON 字符串解析为 Lua 值。
- 参数：`s` string 必填。
- 返回：任意 Lua 值。
- 报错：解析失败抛 `http.json_decode: <err>`。
- 底层：同 `game.jsonDecode`——把 JSON 解析成对应 Lua 类型（对象→表、数组→1 起索引的表）。

### `http.json_encode`

```lua
http.json_encode(v)
```

- 作用：把 Lua 值转成 JSON 字符串。
- 参数：`v` 任意 Lua 值。
- 返回：string。
- 报错：序列化失败抛 `http.json_encode: <err>`。
- 底层：同 `game.jsonEncode`——把 Lua 值映射成 JSON 类型后编码成字符串。

### `http.post`

```lua
http.post(url, body, opts)
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.put`

```lua
http.put(url, body, opts)
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.querify`

```lua
http.querify(paramsTable)
```

- 作用：把参数表编码成 URL query 字符串。
- 参数：`paramsTable` table 必填（键值都转字符串）。
- 返回：string，如 `"a=1&b=2"`（按键排序，同键累加）。
- 报错：非 table 抛类型错误。
- 底层：`url.Values` + `vals.Encode()`。

> 与 `game.httpGet/httpPost` 的区别：`game.*` 返回纯文本、非 200 直接抛错，适合简单场景；`http.*` 返回完整响应对象、携带状态码与响应头、非 2xx 也能拿 body，适合需要细粒度控制的场景。

### `http.request`

```lua
http.request(method, url, body, opts)
```

- 作用：发送任意 method 的 HTTP 请求，返回完整响应对象。
- 参数：`method` string 必填；`url` string 必填；`body` string 可选，默认 `""`；`opts` table 可选（含 `headers` 表、`timeout` 秒）。
- 返回：table。成功 `{ok=true, status=number, body=string, headers=table}`；传输错误 `{ok=false, error=string}`。
- 报错：无（错误放进 `error` 字段，不抛异常）。
- 底层：`httpRequestFull(method, u, body, headers, timeout)`。

# system · Android设备

系统桥、通知、震动、Shizuku；桌面支持范围与Android不同。


### `system.call`

```lua
system.call(method, paramsTable)
```

- 作用：通用底层透传——把 `method` 和参数表交给 Java 分发执行，返回任意结果。已支持的方法：`shell`、`shizukuStatus`、`shizukuRequest`、`vibrate`、`notify`、`launchApp`；未知方法返回 `{"ok":false,"error":"unknown method: ..."}`。
- 参数：`method` string 必填；`paramsTable` table 可选。
- 返回：任意 Lua 值（Java 返回 JSON 解码结果）；无返回推 `nil`。
- 报错：参数序列化失败或桥调用失败抛 `system.call: <err>`。
- 底层：参数 `json.Marshal` → `callAndroidSystem` → JNI → Java `FloatingBridge.call` switch 分发。

### `system.notify`

```lua
system.notify(title, body)
```

- 作用：发系统状态栏通知（**不需要** Shizuku/ADB 权限）。
- 参数：`title` string 必填；`body` string 可选，默认 `""`。
- 返回：table `{ok=true}`；异常时 `{ok=false, error=...}`。
- 报错：桥调用失败抛 `system.notify: <err>`。
- 底层：参数 `{"title","body"}` → JNI → Java `postDirectNotification`。

### `system.platform`

```lua
system.platform()
```

- 作用：返回当前运行平台。
- 参数：无。
- 返回：string——`"android"` 或 `"desktop"`。
- 报错：无。
- 底层：`getPlatformName()`（不经 JNI 桥）。

### `system.shell`

```lua
system.shell(cmd)
```

- 作用：以 Shizuku(ADB) 身份执行一条 shell 命令。
- 参数：`cmd` string 必填。
- 返回：table `{ok=bool, exit=number, stdout=string, stderr=string, uid=number, error?=string}`。命令本身失败不抛异常，走 `ok=false`。
- 报错：仅桥调用失败抛 `system.shell: <err>`。
- 底层：参数 `{"command":cmd}` → JNI → Java `runShell`：空命令/Shizuku 未运行/未授权各有明确错误；否则经反射调 `Shizuku.newProcess` 执行 `/system/bin/sh -c <cmd>`，读 stdout/stderr、取退出码与 uid。

### `system.shizukuRequest`

```lua
system.shizukuRequest()
```

- 作用：弹出系统授权对话框，申请 Shizuku ADB 权限。
- 参数：无。
- 返回：table `{ok=bool, requested=bool, error?=string}`。
- 报错：桥调用失败抛 `system.shizukuRequest: <err>`。
- 底层：JNI → Java `requestShizukuPermission`：pingBinder 失败返回未运行；否则 `Shizuku.requestPermission()` 弹授权框。

### `system.shizukuStatus`

```lua
system.shizukuStatus()
```

- 作用：查询 Shizuku 运行状态与授权情况。
- 参数：无。
- 返回：table `{available=bool, granted=bool, uid=number}`（失败时 uid=-1）。
- 报错：桥调用失败抛 `system.shizukuStatus: <err>`。
- 底层：JNI → Java `shizukuStatusJson`：`Shizuku.pingBinder()`、`checkSelfPermission()==GRANTED`、`getUid()`。

### `system.vibrate`

```lua
system.vibrate(ms)
```

- 作用：触发手机震动（**不需要** Shizuku/ADB 权限）。
- 参数：`ms` number 可选，默认 500（毫秒）。
- 返回：table `{ok=true}`；异常时 `{ok=false, error=...}`。
- 报错：桥调用失败抛 `system.vibrate: <err>`。
- 底层：参数 `{"ms":ms}` → JNI → Java `vibrate`（Vibrator）。

# storage · 缓存与事务

内存键值、原子加分、自动持久化与条件事务。


### `storage.atomicAdd`

```lua
storage.atomicAdd(key, delta)
```

- 功能、参数、返回与约束：key,delta=1；持锁原子加，默认从0开始；非数字报错，不发生读改写竞争。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.delete`

```lua
storage.delete(key)
```

- 功能、参数、返回与约束：key；删除缓存键，后台持久化。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.flush`

```lua
storage.flush()
```

- 功能、参数、返回与约束：无参数；同步写临时文件、Sync、原子替换storage.json；失败报真实路径错误并保留dirty。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.get`

```lua
storage.get(key, default)
```

- 功能、参数、返回与约束：key,default?；内存读取并复制值，账号/插件的数据目录独立；首次读取storage.json一次。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.set`

```lua
storage.set(key, value)
```

- 功能、参数、返回与约束：key,value；复制JSON可表示的值到内存，后台约1秒自动原子持久化；重要提交用flush或transaction。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.transaction`

```lua
storage.transaction(operations)
```

- 功能、参数、返回与约束：operations数组；op=set/add/delete/check,key,value。check值不符则整批回滚；整批验证后持锁同步持久化成功才替换缓存。不是任意Lua函数/跨文件事务。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

# web · 插件控制页面

声明式页面、HTML资源、动作回调与事件推送，重载清理旧注册。


### `web.action`

```lua
web.action(page, action, handler)
```

- 作用：绑定一个交互动作——浏览器对 `/web/api/<插件id>/<页面名>` 发请求时，调用常驻实例的 `handler` 处理。
- 参数：`page` string、`action` string 必填；`handler` **string 必填**（全局**函数名字符串**，不是函数引用）。
- 返回：无。
- 报错：缺插件上下文/空名/重复注册抛异常。
- 底层：`registerWebAction`；执行时新建 LState（2 分钟超时）进入常驻实例后 `CallByParam` 调 handler，返回值经 `luaToGo` 作为响应 `data` 回传。受独立并发信号量（容量 8）约束。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `web.emit`

```lua
web.emit(page, event, data)
```

- 作用：把一条事件实时推送给该插件的全部 SSE 客户端。声明式页面内置 JS 按 key 更新控件；自定义页 JS 自行监听。用于后台状态/进度推送。
- 参数：`page` string 必填（作为 SSE 事件名）；`event` string 必填（会写入数据 map 的 `__event` 键）；`data` 任意 Lua 值必填。
- 返回：无。
- 报错：数据无法 JSON 序列化抛 `web.emit: <err>`。
- 底层：`pushWebEvent(plugin, page, dataJSON)`，构造 SSE 消息写入所有客户端通道（通道满则丢弃，不阻塞 Lua）。**此方法在重跑上下文不跳过**（供 handler 内推送用）。

### `web.page`

```lua
web.page(name, desc, controls)
```

- 作用：声明一个**声明式页面**，在门户 `/web/` 显示卡片，点进由内置模板渲染控件页。
- 参数：`name` string 必填（插件内唯一，不含 `/ \ # ? %` 及控制字符）；`desc` string 可选，默认 `""`；`controls` table 可选（控件表，可读字段 `type`=text|button|input|table|switch、`key`、`label`、`action`、`placeholder`、`columns`）。
- 返回：无。
- 报错：空名/非法字符/同插件页面名重复注册抛异常。
- 底层：`registerWebPage(&webPage{Kind:"declarative",...})`，写入全局 `webPages` map。

### `web.serve`

```lua
web.serve(name, entry)
```

- 作用：声明一个**自定义 HTML 页面**，直接服务 `www/` 目录下对应入口文件。
- 参数：`name` string 必填（规则同 `web.page`）；`entry` string 可选，默认 `"index.html"`（相对 `www/`）。
- 返回：无。
- 报错：同 `web.page`。
- 底层：`registerWebPage(&webPage{Kind:"html", Entry:entry})`；静态根目录 `插件文件/<id>/www`，经 `webStaticSafe` 防路径逃逸 + `serveWebStatic` 提供。


# accounts · 跨账号调用

以当前控制端身份读取目标账号状态、发消息、批量提交命令或调用其常驻插件导出函数。授权按账号定向生效，跨账号递归调用被拒绝。


### `accounts.callPlugin`

```lua
accounts.callPlugin(target, pluginID, function, args)
```

- 功能、参数、返回与约束：target,pluginID,function,args数组；调用目标常驻插件的导出函数，不重跑源码。禁止跨账号递归调用。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accounts.list`

```lua
accounts.list()
```

- 功能、参数、返回与约束：无参数；列出本控制端的账号ID、名称，不返回令牌。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accounts.sendCommandsFast`

```lua
accounts.sendCommandsFast(target, commands, options)
```

- 功能、参数、返回与约束：target,commands,options?；在目标账号顺序提交完整命令数组，不等待逐条回包。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accounts.sendMessage`

```lua
accounts.sendMessage(target, text)
```

- 功能、参数、返回与约束：target,text；授权后由目标机器人发游戏聊天，不伪装玩家。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accounts.status`

```lua
accounts.status(target)
```

- 功能、参数、返回与约束：target；按定向授权读取目标账号连接状态。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

# accountsPool · 共享机器人账号池

登记、启动、绑定、借用与释放机器人，统一保存抓包配置并生成错峰观察规划。规划不等于执行，启动请求不等于进服。


### `accountsPool.acquire`

```lua
accountsPool.acquire(options)
```

- 功能、参数、返回与约束：options；预留空闲机器人并绑定player/server，不自动创建游戏账号。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.add`

```lua
accountsPool.add(account)
```

- 功能、参数、返回与约束：account；保存自己账号的Token/auth_url/server，owner_id由宿主绑定。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.bind`

```lua
accountsPool.bind(id, options)
```

- 功能、参数、返回与约束：id,options；player/server，持久绑定；不自动传送。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.bindings`

```lua
accountsPool.bindings(options)
```

- 功能、参数、返回与约束：options?；只读自己的绑定状态；管理员all=true。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.captureConfigure`

```lua
accountsPool.captureConfigure(patch)
```

- 功能、参数、返回与约束：patch对象；统一保存enabled/accounts/auto_fleet/auto_provision/account_limit/fleet_steps/idle_seconds/player_limit/enforce_player_limit和镜头参数。本人或明确shared_capture账号可选；自动1/2/3、>10为5、>=15为6台，数量非0为人工基础投入；fleet_steps=[{min_players:1,observers:1},...]可自定义递增人数阶梯，最多32台子进程，不限双机器人；无真人停镜头，专用采集进程60秒回收。分维度回防/借调/错峰，失败冷却替补。修改会影响本设备统一观察，不创建插件。dimension_landings按0/1/2配置{x,y,z}（其他维度标识暂不支持落点坐标）；仅配置过的维度启用上线前救援，实际坐标及子连接确认后停止TP；无确认60秒停止子连接，states中landing:<id>可核对结果。game_mode设默认模式，dimension_game_modes按维度标识覆盖；keep保持当前。invisible/quick_tp/auto_respawn分别选择隐身、自动靠近和死亡自动重生；省略或auto_respawn=null沿用账号设置。dimension_accounts与dimension_game_modes支持字符串维度标识：0主世界/1下界/2末地老配置照旧，也接受dm、minecraft:overworld等自定义标识，最多8条，格式限1～40位小写字母数字与_:.-。当前运行时只能调度主世界/下界/末地，映射不到的维度保留配置、跳过并记一条warn，不会崩溃也不会猜维度。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.captureRoster`

```lua
accountsPool.captureRoster(options)
```

- 功能、参数、返回与约束：options={include_bots:false}；只读本服当前名单和原生querytarget真实维度，用于诊断。include_bots=true仅诊断受管机器人，不改变生产观察排除规则，不传送/发镜头；维度缺失标unknown。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.captureStatus`

```lua
accountsPool.captureStatus()
```

- 功能、参数、返回与约束：无参数；读取宿主全局抓包config/plan/states/candidates。真实玩家维度未知不会猜测；active/submitted不是全局无漏包证明。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.handoff`

```lua
accountsPool.handoff(incoming_id, outgoing_id, options)
```

- 功能、参数、返回与约束：incoming_id,outgoing_id,options={player,mode:camera|follow|hybrid,interval_seconds:2,timeout_seconds:3,next_player?}；先让接替机器人观察同一目标，收到对应的新区域更新后才停止原观察，超时保留原观察。可让原观察者接着观察next_player；两端均须在自己的授权池。返回真实ready/old_released，不等于零漏包。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.invoke`

```lua
accountsPool.invoke(id, options)
```

- 功能、参数、返回与约束：id,options；method/path/query/body，通过该机器人真实API执行，path仅/api。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.list`

```lua
accountsPool.list(options)
```

- 功能、参数、返回与约束：options?；普通用户只列自己的机器人，管理员可all=true。无凭据返回。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.observationPlan`

```lua
accountsPool.observationPlan(options)
```

- 功能、参数、返回与约束：options={accounts:[账号ID，main可选],players:[真实在线名，可空],dwell_seconds:0.5,radius_blocks:48,include_bots:false}；返回错峰路线、内部机器人标签和排除名单。普通用户只用自己的池，管理员可用授权池。只是规划，不启动或传送；48格是参考，不是必达半径。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.release`

```lua
accountsPool.release(id, options)
```

- 功能、参数、返回与约束：id,options?；解除绑定；idle_seconds默认60，期间可再次acquire；到期停用。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.start`

```lua
accountsPool.start(id, options)
```

- 功能、参数、返回与约束：id,options?；真实启动独立内核并请求连接指定server，成功请求不等于进服。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.status`

```lua
accountsPool.status(id)
```

- 功能、参数、返回与约束：id；读取该机器人真实连接和插件状态。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `accountsPool.stop`

```lua
accountsPool.stop(id)
```

- 功能、参数、返回与约束：id；断开并停用此机器人，取消保活。
- 宿主实现：以当前控制端身份执行，跨账号需定向授权；失败返回错误，不伪造成功。

<!-- 2026-10-26 补写：源码注册面同步 -->

# diagnostics · 解码与事件诊断

真实解码包计数、事件队列配置、事件追踪、可见性与战斗证据。只报告已收到的数据，被覆盖的历史标为未知。


### `diagnostics.combat`

```lua
diagnostics.combat(target)
```

- 功能、参数、返回与约束：target；返回保留事件中的伤害/血量/死亡/击杀，含同一event_id的补充证据。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `diagnostics.configure`

```lua
diagnostics.configure(config)
```

- 功能、参数、返回与约束：config?；宿主持久配置：event_queue/priority_queue/runtime_queue/async_tickets/event_history/actors各1..65536，stale_ms=1..86400000。事件队列变更在插件重载时生效，其余立即影响后续操作。不是命令CPS限速，也不是包种类上限。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `diagnostics.entity`

```lua
diagnostics.entity(target)
```

- 功能、参数、返回与约束：target；同entities.get，额外完整字段来源及过期信息。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `diagnostics.receiveTransport`

```lua
diagnostics.receiveTransport()
```

- 功能、参数、返回与约束：无参数；当前连接协商协议、真实注册包ID与解码前帧/字节/未知包计数。decode_panics是解码中断，trailing_bytes_frames是完成已知字段后剩余扩展字节，decoded_frames是完成解码的帧，returned_packets是返回上层的包数；decode_errors是旧版异常合计，不能当丢包数。只保存数字元数据，不保存认证或包内容。不依赖服务器组件。单批数量限制与业务留存容量分别报告，不冒充全服广播权限。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `diagnostics.stats`

```lua
diagnostics.stats()
```

- 功能、参数、返回与约束：当前账号解码包计数、最后包时间、实体数、事件缓冲及配置、断线/踢出诊断和整个宿主内存/GC；非单插件内存。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `diagnostics.trace`

```lua
diagnostics.trace(event_id)
```

- 功能、参数、返回与约束：event_id或sequence字符串；查询保留的事件及关联死亡/受伤详情；被覆盖返回known=false，不虚构历史。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `diagnostics.visibility`

```lua
diagnostics.visibility()
```

- 功能、参数、返回与约束：无参数。实际区块半径、Publisher、筛选前decoded/selected/filtered和enqueued/cancelled逐类型计数。ingress含监听阶段与登录缓存回放，login_ingress单独记录挑战回调，两者有重叠不能相加；不含解码失败。显式PyRpc抓包时记录应用事件名称、不留参数；登录目录亦只留名称，容量event_history。记录不代表可调用或全服广播。无宿主距离过滤；区块半径不等于实体广播范围。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# entities · 实体观测快照

身份、位置、血量、属性与装备，只覆盖服务器已下发且未过期的数据，不扫描全世界。


### `entities.attributes`

```lua
entities.attributes(target)
```

- 功能、参数、返回与约束：target；返回服务器同步的属性字典，value/min/max/updated_at/source；armor仅服务器同步minecraft:armor时已知，不按穿戴猜护甲点数。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `entities.get`

```lua
entities.get(target)
```

- 功能、参数、返回与约束：runtime_id十进制字符串或玩家名；服务器观测快照，含身份/位置/health结构/attributes/装备/视角/present/stale。未收到为known=false，不扫描全世界。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `entities.health`

```lua
entities.health(target)
```

- 功能、参数、返回与约束：同entity.health，读取服务器UpdateAttributes/SetHealth缓存，不发送查询命令。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `entities.nearby`

```lua
entities.nearby(position, radius, dimension)
```

- 功能、参数、返回与约束：position=[x,y,z],radius=32,dimension?；已观测、仍存在且位置未过期的实体；不查询不可见区域。位置省略或self时使用机器人实际位置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# entity · 单实体读取与伤害

血量字段区分已知与未知，未知最大生命保留 null；伤害事件不推断攻击者。


### `entity.health`

```lua
entity.health(target)
```

- 功能、参数、返回与约束：target=self或runtime_id字符串；current/max/previous/damage/known/max_known/stale/updated_at/source。最大生命未知为null，绝不假定20。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `entity.onDamage`

```lua
entity.onDamage(handler)
```

- 功能、参数、返回与约束：同events.onEntityHurt；血量下降确认伤害，只有ActorEvent时数量未知，不能由最近玩家或挥臂猜攻击者。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

<!-- 2026-10-26 补写：源码注册面同步 -->

# hud · HUD频道管理

同一玩家多插件文本按优先级仲裁显示，过期自动恢复下一频道；只管理经本接口发送的内容。


### `hud.remove`

```lua
hud.remove(player, channel)
```

- 功能、参数、返回与约束：player,channel；删除本插件频道，下一调度恢复仍活跃频道；卸载插件自动清理。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `hud.set`

```lua
hud.set(player, channel, options)
```

- 功能、参数、返回与约束：player,channel,{text,kind=actionbar/title/subtitle,priority=0,duration_ms=2000}；宿主统一频道仲裁，优先级最高的活跃频道显示，文本变化更新，过期恢复下一频道。默认不重播动画。仅管理经hud接口发送的内容，无法阻止绕开管理器的原始titleraw。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `hud.update`

```lua
hud.update(player, channel, options)
```

- 功能、参数、返回与约束：player,channel,{text,kind=actionbar/title/subtitle,priority=0,duration_ms=2000}；宿主统一频道仲裁，优先级最高的活跃频道显示，文本变化更新，过期恢复下一频道。默认不重播动画。仅管理经hud接口发送的内容，无法阻止绕开管理器的原始titleraw。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# mcp · 本机MCP

工具列表、资源目录、调用与连接信息。自动绑定当前账号，不允许指定其他账号，不导出密钥。


### `mcp.call`

```lua
mcp.call(tool_name, arguments)
```

- 功能、参数、返回与约束：tool_name,arguments对象；使用本机真实MCP工具分发与权限，自动绑定当前账号，不允许指定别的账号。不访问旧固定云桥，不导出密钥。
- 宿主实现：走本机真实MCP服务端，与插件实例无关；连接失败直接报错。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `mcp.connection`

```lua
mcp.connection()
```

- 功能、参数、返回与约束：无参数，读取当前实例真实本机MCP地址、远端状态与账号。密钥由宿主内部使用，不返回给模型或聊天。
- 宿主实现：走本机真实MCP服务端，与插件实例无关；连接失败直接报错。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `mcp.readResource`

```lua
mcp.readResource(uri)
```

- 功能、参数、返回与约束：uri，按需读取真实MCP资源。
- 宿主实现：走本机真实MCP服务端，与插件实例无关；连接失败直接报错。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `mcp.resources`

```lua
mcp.resources()
```

- 功能、参数、返回与约束：无参数，读取本机MCP资源目录。
- 宿主实现：走本机真实MCP服务端，与插件实例无关；连接失败直接报错。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `mcp.tools`

```lua
mcp.tools()
```

- 功能、参数、返回与约束：无参数，读取本机实际MCP工具列表，不自动把全部说明塞进AI上下文。
- 宿主实现：走本机真实MCP服务端，与插件实例无关；连接失败直接报错。

<!-- 2026-10-26 补写：源码注册面同步 -->

# observerBridge · 观察桥与采集

采集密钥、自身输入接入、常驻观察、轮巡、维度调度与跨账号事件汇总。submitted 不等于服务器已广播，不承诺全服覆盖。


### `observerBridge.assignDimension`

```lua
observerBridge.assignDimension(options)
```

- 功能、参数、返回与约束：options={dimension:0|1|2,player?,landing?:{x,y,z},game_mode?:keep|survival|creative|adventure|spectator,invisible?:bool,auto_respawn?:bool}；仅控制本机器人，关闭auto_creative维护；未指定模式保持当前。跨维度优先靠近该维度目标玩家；无玩家时仅使用明确配置的落点，否则保留当前位置。导入导出期间暂停；ready按真实身体维度判断。需本账号OP。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.attachSelf`

```lua
observerBridge.attachSelf(source_id, token)
```

- 功能、参数、返回与约束：source_id,token；将当前已连接机器人的自身加密前输入接入采集桥，player_id必须匹配本机UUID/XUID/名字。异步采集、不阻塞命令发送；不等于其他玩家已接入。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.coverage`

```lua
observerBridge.coverage(options)
```

- 功能、参数、返回与约束：options={server?}；按需汇总本宿主主账号、舰队与托管子账号的实际观测覆盖、维度和错误；不表示全服覆盖。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.create`

```lua
observerBridge.create(options)
```

- 功能、参数、返回与约束：options={name,source:player_input|game_server,server,player_id?}；创建独立采集密钥（仅返回一次）。客户端数据不冒充服务端确认；接入不自动获得山头服模块权限。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.detachSelf`

```lua
observerBridge.detachSelf()
```

- 功能、参数、返回与约束：无参数。停止当前机器人的输入采集，游戏连接和其他来源不受影响。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.dimensionStatus`

```lua
observerBridge.dimensionStatus()
```

- 功能、参数、返回与约束：读取本机器人实际维度、游戏模式、连接和OP；不能将摄像机位置当身体维度。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.events`

```lua
observerBridge.events(after, limit)
```

- 功能、参数、返回与约束：after=0,limit=256；读取events/cursor/overwritten，保存cursor。event.data为封装，动作字段在event.data.data。swing_input:Animate挥臂或PlayerAuthInput.missed_swing空挥；attack_input攻击提交；position_input位置/视角；health_observation本人服务器血量。只覆盖已注册且实际接入来源，不覆盖机器人权威缓存；重启清空历史。详细技能player-actions。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.health`

```lua
observerBridge.health(runtimeId, options)
```

- 功能、参数、返回与约束：runtimeId字符串,options={server?,dimension?}；读取各在线观测账号最新真实血量，附观察者/session/来源/过期状态；维度或实体身份冲突返回未知。不会覆盖本机entities缓存。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.inputStatus`

```lua
observerBridge.inputStatus()
```

- 功能、参数、返回与约束：无参数。自身输入桥运行状态、采集记录数、未投递数和错误；不代表游戏处理成功。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.jobState`

```lua
observerBridge.jobState()
```

- 功能、参数、返回与约束：返回当前账号导入导出等空间作业的paused/reasons；插件先检查paused再发传送，避免高频重复触发暂停错误。作业结束自动恢复，普通查询和非移动命令保持可用。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.localView`

```lua
observerBridge.localView(options)
```

- 功能、参数、返回与约束：options={mode:coverage|health|events,target?,dimension?,session?,after?,limit?}；仅当前连接快照，供观测聚合使用；无递归，不执行游戏指令。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.locator`

```lua
observerBridge.locator(target, options)
```

- 功能、参数、返回与约束：玩家名或UniqueID字符串,options={server?}；汇总真实PlayerLocation广播，即使没有AddPlayer也保留；包没有维度、血量或RuntimeID，这些字段明确未知。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.patrolStart`

```lua
observerBridge.patrolStart(options)
```

- 功能、参数、返回与约束：options={players:[真实在线名],interval_seconds:0.001,single_follow_seconds:2,duration_seconds:0,restore_position:true}。空players自动选择当前真人/其他玩家，排除自己；每次开启分配独立计分板编号一次，刷新需停止重开。按1..N顺序TP只发不等，单玩家使用single_follow_seconds。默认关闭；0.001是提交目标间隔，不是服务器生效保证；插件停止自动取消。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.patrolStatus`

```lua
observerBridge.patrolStatus()
```

- 功能、参数、返回与约束：无参数。真实submitted/rounds、missed_intervals、position_updates、by_player事件计数、最后实际坐标、错误与清理状态；不是全局覆盖证明。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.patrolStop`

```lua
observerBridge.patrolStop()
```

- 功能、参数、返回与约束：无参数。取消当前账号轮巡，移除本次专用分数目标；默认恢复起点。插件不能关闭其他插件拥有的轮巡。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.revoke`

```lua
observerBridge.revoke(source_id)
```

- 功能、参数、返回与约束：source_id；撤销采集密钥。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.rpcEvents`

```lua
observerBridge.rpcEvents(after, limit)
```

- 功能、参数、返回与约束：after=0,limit=256；读取当前连接收到的YouyuanBridge/ObserverV1/Observation；来源server_module_reported，与协议属性缓存分开。重连清空，不代表已验证引擎事件。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.rpcProbe`

```lua
observerBridge.rpcProbe(timeout_ms)
```

- 功能、参数、返回与约束：timeout_ms=3000；向YouyuanBridge/ObserverV1发送Hello，只在对应Capabilities回复匹配request_id时confirmed=true。超时不代表其他网易模块不可用。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.rpcQuery`

```lua
observerBridge.rpcQuery(query, timeout_ms)
```

- 功能、参数、返回与约束：{query,target?},timeout_ms=3000；查询已安装YouyuanBridge服务端只读Provider；收到对应Reply才confirmed=true。没有Provider不会凭空读取服务端数据。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.serverHistory`

```lua
observerBridge.serverHistory(options)
```

- 功能、参数、返回与约束：options={session?,after?,limit?}；当前连接主动转发模块历史，带reset/lost/cursor/接收时间；不执行网络请求，不使用旧会话记录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.serverSnapshot`

```lua
observerBridge.serverSnapshot(entity_id)
```

- 功能、参数、返回与约束：entity_id字符串；读取当前会话最近模块health_changed/health_snapshot，返回known/current/max/previous/来源/时间/stale。未知不填20；需已收到转发模块，不覆盖本机属性缓存。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.sources`

```lua
observerBridge.sources()
```

- 功能、参数、返回与约束：无参数。采集来源、绑定服务器/玩家、最后序号/时间；不返回密钥。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.watchStart`

```lua
observerBridge.watchStart(options)
```

- 功能、参数、返回与约束：options={player或players:[],auto_players:false,exclude_players:[],camera_selector:false,mode:camera|follow|hybrid,interval_seconds:2,camera_height_blocks:4,camera_pitch_degrees:90,camera_yaw_degrees:0,pause_during_jobs:true,duration_seconds:0,restore_position:true,follow_radius_blocks:16,body_radius_blocks:15}；camera模式只用摄像机视角、不切旁观模式，默认可把身体保持在目标body_radius_blocks内（0关闭）；远处可见性以实际下发包核对，切换期间会有观察空档；players/auto_players或camera_selector启用原生execute at选择器相机切换，不逐次querytarget、不TP被观察玩家；间隔属于一次目标切换，一轮约目标数乘间隔，submitted不能视为收到所有包。follow/hybrid可能TP并查询位置；导入导出期间默认暂停，插件可先查jobState再传送。需OP，不保证跨维度或全局广播；停止插件自动取消。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.watchStatus`

```lua
observerBridge.watchStatus()
```

- 功能、参数、返回与约束：返回本账号常驻观察目标、真实区域发布时间、submitted、missed_intervals、错误。region_updated只是收到对应区域包，不等于零漏包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.watchStop`

```lua
observerBridge.watchStop()
```

- 功能、参数、返回与约束：取消本账号常驻观察，清除本次相机并恢复已确认的原位置；不会关闭其他插件拥有的观察。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `observerBridge.worldEvents`

```lua
observerBridge.worldEvents(options)
```

- 功能、参数、返回与约束：options={server?,dimension?,cursors?,limit?,stream:combat|actions}；汇总各账号规范事件，返回每账号session/sequence游标；重连标明reset，历史覆盖标明lost。再次使用返回cursors避免重复；limit是每观察者返回条数。combat独立保留真实战斗事件，区域切换的生物出现/消失不会挤掉命中。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# scoreboard · 计分板读写

读取已同步条目，提交原生 add/remove/set；提交成功不等于服务器已生效。


### `scoreboard.get`

```lua
scoreboard.get(objective, player)
```

- 功能、参数、返回与约束：objective,player；读取SetScore已同步条目，未广播known=false，不同步查询全服。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `scoreboard.increment`

```lua
scoreboard.increment(objective, player, amount)
```

- 功能、参数、返回与约束：objective,player,amount整数；提交原生scoreboard add/remove，不本地读改写，避免竞态；提交不等于服务器成功。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

### `scoreboard.set`

```lua
scoreboard.set(objective, player, value)
```

- 功能、参数、返回与约束：objective,player,value整数；提交计分板设置，submitted=true/confirmed=false，调用Confirmed命令或同步SetScore才能验证结果。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

<!-- 2026-10-26 补写：源码注册面同步 -->

# 常驻内核、命令缓冲与资源重载

这是游园0.10.9实际框架接口。入口语言由工作区/manifest决定，三种语言手册都可以多选；允许读取技能不等于给工具权限。正文按章节读取，没有自动联网动作。

## 常驻与模块

Lua加载/编译main.lua一次，事件调用现存函数，全局/闭包/package.loaded保留。require("modules.death")查插件代码目录下modules/death.lua；模块return一个表，在同一Lua实例内缓存。Python是每插件长驻模块和asyncio Loop。C++是长驻对象。重载整个插件时内存会重建，应把业务数据放Storage。旧语法保持兼容，不回退到每事件重跑源码。

```lua
local death=require("modules.death")
events.onPlayerDeath("died")
function died(e) death.record(e) end
-- modules/death.lua
-- return {record=function(e) storage.atomicAdd("deaths:"..e.player,1) end}
```

## 统一时钟与防重入

util.scheduleTick(1,handler)每个名义宿主Tick，Tick=50ms；scheduleTick(20,handler)每秒。scheduleHz(20,handler)、scheduleHz(10,handler)、scheduleHz(1,handler)使用同一调度Loop。Lua可传函数或全局名称；Python可传函数；C++先用on注册再传名称。当前策略skip：一个回调还没执行完，就跳过后续到期并累计missed，不补排队。不是服务器同Tick确认，调度器不能把远端服务器的50ms延迟消除。

```lua
local n=0
local job=util.scheduleTick(1,function() n=n+1 end)
-- util.listJobs()有calls/missed/last_ms/max_ms/total_ms/last_error
-- util.pauseJob(job); util.resumeJob(job); util.cancel(job)
```

## 三条命令路径

game.sendCommand或sendCommandFast：只发送，默认魔法SettingsCommand，不逐条UUID/回包/日志。sendCommandAsync：返回ticket，后台等CommandOutput；game.commandResult(ticket,0,true)读取完成结果并consume。sendCommandConfirmed/game.execute：当前回调等待真实结果。普通高频FUNCTION用Fast；仅有成功条件/查询结果才用确认。异步ticket最多128个未消费项是可见内存保护，Fast不经过这项限制。

```lua
local b=CommandBuffer()
for i=1,10000 do b:add("scoreboard players add @s ybench 1") end
local r=b:submitFast() -- 一次宿主边界，整数组按序提交；没有1024行限制
-- r.submitted是宿主接收数量，必须再查真实分数，不代表同游戏Tick执行。
local ticket=game.sendCommandAsync("scoreboard players list @s").ticket
-- 下一次回调查询：game.commandResult(ticket,0,true)
```

Python用host.command_buffer().add(cmd).submitFast()；C++用yy::CommandBuffer b(client); b.add(cmd); b.submitFast();。三种语言都可以直接sendCommandsFast命令数组。Lua缓冲在Lua表中构建，Python在list中构建，C++在json数组中构建；提交一次。Python/C++桥仍会对这一整个数组编码JSON，没有声称零拷贝/全原生ABI。

## FUNCTION IR与文件监听

TXT保存后parse成节点/控制点/行号IR，常驻调用，运行时不读TXT、不解析字符串。1,0,0脉冲；2,0,0 20为整个TXT实例20Hz；0,0,0连锁；0,1,0有条件；.独立链断开成功状态；!结束周期。未重新声明的属性继承。一个TXT实例只有一个整轮Hz；不同频率建独立TXT。第三位红石保留语法兼容，不在宿主做游戏红石门控。run_count=0持续，正数按完整文件轮次计数。

固定目录插件数据文件/ds_ai_agent/command_txt/。外部改TXT后，宿主每秒检查mtime/size，只有修改才读文件重新编译；无效修改保留原IR和报错。chains.reload(name)可立即资源重载，当前轮旧IR、下轮新IR，计数和启停保留。修改config/module仍需相应实例重载；完整reloadConfig/reloadModule尚未实现，不能假装只改一份Lua就不重启。

## Storage与事务

storage.get/set/atomicAdd/delete是插件数据目录中的内存缓存。后台约1秒把dirty数据原子写storage.json。flush同步持久化；进程被杀前还没flush的最近修改可能丢失，不能把自动保存当每次提交都已落盘。

```lua
storage.atomicAdd("points",1)
storage.transaction({
 {op="check",key="points",value=100},
 {op="add",key="points",value=-20},
 {op="set",key="paid",value=true}
})
storage.flush()
```

transaction先验证所有操作，在同一锁内复制/更新，再同步写成功后替换缓存；check失败不写入。支持set/add/delete/check数组，Python用dict/list，C++用yy::json。这不是任意语言函数事务，也不是跨文件/跨插件分布式事务。

## 性能观察与未完成部分

plugin.stats、util.listJobs和chains.list有实际调用数、耗时、missed、丢弃、队列和整轮统计。耗时是墙钟，不是CPU占用；逐插件CPU/内存/GC尚未归因，返回null。高速发送不逐条写命令日志。极端重复大链可能让RakNet/服务器承受不住，应实测最终计分；本地提交完成不等于服务端完成。可选每Tick命令预算/overflow carry、完整混合模块manifest、Linux C++适配器和native进程崩溃隔离尚未交付。


# 协议到游戏功能的实际开发

先查询plugin.capabilities/packet.describe，再按需订阅。只使用本账号真正收到的包。优先高层事件，不每Tick抓所有包；原始包和高层事件可以组合，不能凭字段未知编造动作来源。

## 挥空气、死亡、移动与拾取

| 功能 | 已可调用的接口 | 包与实际边界 |
|---|---|---|
| 挥手/挥空气触发技能 | events.onSwing(handler) | AnimateActionSwingArm；不证明命中谁，旁观机器人需要服务器广播 |
| 玩家死亡/最后位置 | events.onPlayerDeath(handler) | ActorEventDeath/InstantDeath、真实血量归零、网易OnPlayerDie；last_position只有position_known=true时可用 |
| 全实体死亡 | events.onDeath(handler) | 去重；RemoveActor单独不当死亡 |
| 血量/受伤 | player.health、events.onHealthChange/onHurt | UpdateAttributes/SetHealth/ActorEvent；其他玩家未广播血量仍unknown |
| 玩家/机器人位置 | player.position、bot.self、events.onMove | MovePlayer/MoveActorAbsolute/Delta；只更新实际包含的轴 |
| 生物/掉落物出现 | events.onEntitySpawn | AddPlayer/AddActor/AddItemActor，视野进入不等于首次生成 |
| 掉落物拾取 | events.onItemPickup | TakeItemActor映射掉落物和拾取者，拾取者可能是生物 |
| 方块状态变化 | events.onBlockChange、world.blockState | UpdateBlock；不能只凭它推断破坏者 |
| 挖掘/放置/攻击/移动 | world.mineBlock/useBlock、bot.attack、bot.startMovement | 实际发送物理动作/交易/输入，服务器按距离、装备、模式和权限校验 |

死亡事件统一结构保留旧顶层字段，并增加event.player/type/data/timestamp；timestamp为毫秒Unix时间，time为RFC3339。data是同一事件数据的副本。原events.onChat(player,message)、events.subscribe(type,handler)的旧签名保留，不强改旧插件。onPlayerDeath只处理玩家；onDeath可含生物，注意entity_type。

```lua
function swung(e)
  if e.player~="" then
    storage.atomicAdd("swing:"..e.player,1)
    -- 可以按自己的权限规则调用chains.start("技能.txt")
  end
end
function died(e)
  storage.atomicAdd("death:"..e.player,1)
  if e.position_known then storage.set("lastdeath:"..e.player,{position=e.last_position,dimension=e.dimension,time=e.timestamp}) end
end
events.onSwing("swung")
events.onPlayerDeath("died")
```

```python
def on_init(host):
    def died(event):
        host.storage.atomicAdd('death:'+event['player'],1)
        if event.get('position_known'):
            host.storage.set('lastdeath:'+event['player'],event['last_position'])
    host.events.onPlayerDeath(died)
```

```cpp
// 放在yy::Plugin构造中：on接收args数组，args.at(0)是真实Event。
on("died",[this](const yy::json& args){
  const auto& e=args.at(0);
  client.storage.atomicAdd("death:"+e.at("player").get<std::string>(),1);
  return yy::json(true);
});
client.events.onPlayerDeath("died");
```

## 网易死亡RPC

精确识别Value=["ModEventS2C",[namespace,system,"OnPlayerDie",{pid,die}],...]
pid为玩家UniqueID，不是RuntimeID；die=true死亡，false重生。只在能映射已观测玩家时触发，不搜索RPC文本猜死亡。游戏是否下发这个事件仍需实际环境抓包验证，结构识别测试不冒充真人死亡实测。raw PyRpc仍可packet.on("PyRpc",...)查看，不能任意重放登录事件。

## 用AI形成操作闭环

AI先project_list/project_read读完整源码，skill_search查询“挥臂/死亡/物品”，load_skill按章节读，packet_schema(name="Animate")只读该包字段和用途。需要原始游戏观察时listen_packet/list_packets，先记录目标类型，不开全包长期高频日志。发包或命令后检查真实响应、位置、方块或记分板。project_write可以新建模块/入口，不止改名字；project_patch能精确替换行；project_validate后project_publish，并plugin_manage start/reload，terminal_log确认。

命令链工作区默认只提供文件、语法校验和链发布工具；Lua/Python/C++工作区可按权限提供游戏/抓包/MCP工具。多选语言技能只开放文档，没有把未授权工具一起开放。玩家可以在新消息中引导运行中的任务；工具失败继续原任务，文件修订与分叉保留。

## 原始包层覆盖更广的玩法

SetActorData：实际metadata状态；MobEquipment/ArmorEquipment：同步装备；InventoryContent/Slot：本机或授权容器背包；InventoryTransaction/ItemStackRequest：本机交互请求；MobEffect：收到的药水效果；LevelSoundEvent：同步声音；SetScore/ScoreboardObjective：计分板；PlayerList：在线资料和皮肤；ChangeDimension：维度；ClientboundMapItemData：地图数据；BossEvent：首领条；PyRpc：网易扩展事件。各包精确字段在packet-reference单包目录，不从别的版本猜ID。

并非所有这些包都有高层函数：玩家三维速度/效果缓存、GUI箱子菜单、特殊物品自动识别、完整world cache和AI寻路仍需开发与真实测试。当前schema登记215种，不等于另外AI提到的“300+包”全部有解析器；未知入站包可原始观察，但没有语义时必须标记unknown。


## 0.10.9 补充：击杀者、维护开关与Linux文件

自动维护创造/飞行在新配置中默认开启；设置 auto_creative=false 可用于受伤或死亡测试，已有用户明确关闭的值保留。权限检查自身不改变模式；维护只对机器人自身处理模式/飞行变更，不反复逐Tick发送维护命令。

击杀者来源是服务器 Text 翻译型死亡消息，必须是实际已观察到的玩家。event.killer_known=true 才能把 killer 当作确定的玩家。OnPlayerDie 只提供死亡/复活，不含可靠击杀者；如果它先到，events.onPlayerDeath 可能先得到未知击杀者，随后通过 events.subscribe("death_details","handler") 收到服务器的补充。events.onDeath或死亡记录查询保留相关字段；无死亡广播时不会编造击杀者。

上传/下载工具箱文件位于设置的“文件选择器与界面”，网页选择 device=linux-server 时保存在该Linux实例的 /var/lib/youyuan/用户上传。上传支持文本、源码、ZIP等，不自动执行或覆盖同名文件。Linux的自建资料库也保存在自己的服务器磁盘，不再把文件拆分写入Cloudflare Durable Objects；Cloudflare保留网页、身份与控制中继。网页控制工具箱应用，不是任意root终端。

自己的源码附件通过 project_list/project_read/project_search 读取；真正修改后依次校验、发布、重载，不能用文件名替代正文。命令链反复运行已编译IR，只有TXT被修改时重新读取和解析。


## 服务器关闭命令反馈时

实服已验证：gamerule sendcommandfeedback false 会使 Confirmed/querytarget 等结果接口收不到回包，即使命令已经执行。Fast 不等待回包，模式维护也使用无等待创造命令和飞行输入。需要条件链、查询结果或确认时，要由服主确保服务器允许命令反馈；超时返回未知，不能把它当失败自动重发。高频无条件链不要为语法验证逐条增加确认。


## 0.10.12 独立段与本机MCP补充
# 游园 0.10.12 · R21A原生命令链与本机MCP

频率属于整条独立段，绝非每条命令。Lua菜单/业务常驻；Go负责独立调度与网络热点，不额外创建Python/C++插件条目。

```text
1,0,0
scoreboard objectives add demo dummy
.
2,0,0 20
scoreboard players add fast demo 1
0,0,0
scoreboard players add fast_second demo 1
.
2,0,0 5
scoreboard players add slow demo 1
!
```

`scheduling=auto`：写了循环Hz时自动独立段调度；`segments`显式独立调度；`file`保留整文件顺序循环兼容。
`bootstrap=true`让文件开头连续脉冲先执行，再启动其他段；非开头脉冲不阻塞兄弟段。R21A适配默认启用bootstrap。
不同段可并行调度，每段内部保持顺序，任务超期不重入、不积压补跑轮次。
`run_count=0`持续；正整数在segments模式对每个循环段计数，脉冲每次启动一次。
加载/修改时预解析成常驻IR；一秒检查资源mtime，未变化不读文件。显式reload立即检查。
修改的段停止当前激活再用新IR启动，已完成脉冲也会重新激活；未改变的段保持时钟/状态。段身份按文件位置，插入/重排可能使后续段视作修改。删除的段取消，累计统计保留。
宿主配置语法通过不代表游戏命令语法通过。动态条件的前条必须每轮取真实响应，不能缓存上轮成功来判断这轮。
Fast无条件不等回包、不逐条日志、rate_cps=0不人为CPS限速。命令Buffer一次跨插件边界，底层仍按协议逐包提交。
并行调度不是服务器同Tick原子执行，不能承诺一亿次/秒；submitted只代表宿主已提交。
segments状态逐段包含target_hz/actual_hz/cycles/submitted/state/error；脉冲Hz=0。父cycles为各段轮数和，不当作一个文件Hz。
@call是有界、等待返回的调用，目标原生实例不会自动后台开启；持续调用停在该位置。多文件/分段的后台并行由独立实例承担。

## 三语言链调用

Lua / Python：`chains.save(name,text,{scheduling="segments",bootstrap=true,mode="fast",interval_ms=50,run_count=0,enabled=false})`（Python对象使用字典与True/False）。随后start/stop/reload/configure。
C++：`client.chains.save(name,text,options)`，相同JSON字段。Linux C++宿主仍未实现，Android支持native。

## 插件本机MCP

|接口|含义|
|---|---|
|mcp.connection()|真实本机地址、实例账号、远端状态；不返回鉴权秘密|
|mcp.tools()|真实Tools目录|
|mcp.resources()|真实Resources目录|
|mcp.readResource(uri)|按需读取资料|
|mcp.call(tool,args)|真实本机MCP分发，自动绑定当前账号|

Lua/Python直接mcp.call；C++ client.mcp.call。参数例如 `{method="GET"}`；具体tools名称从mcp.tools获取，不能虚构。
密钥由宿主内部注入，模型只读工具结果。不同账号的控制用accounts定向授权，不通过arguments.account绕过。
完整MCP属于已授权本机插件控制能力，不是恶意本地代码沙箱；不可让普通群成员得到管理工具。
R21A内置MCP不再调用旧固定ai-api云桥，不嵌入共享平台密钥；自然语言控制任务进入本机已配置模型的AI工作区。
`.AIAgent状态`看真实连接；`.AIAgent列表`看本机工作区，先配置模型后再提交控制任务。

## AI及时性与公告

保留原[Ds]、正在思考中、Token本次/窗口累计。思考提示移到最外层聊天入口，Fast提交先于旧包装检查/读档；没有新增阶段提示。
提示词保留R21A六文件，用户数据版优先，首次任务按需读取并缓存，reloadPrompts可清缓存。
API报告的重试usage也累计，不虚报价格。聊天模型HTTP仍在该LuaVM同步等待，原生链不被此VM等待阻塞；真正的Lua交互并发需要进一步拆异步任务。
公告保留R21A内容，但计分板仅更新变化行，避免每次重建objective。

## 管理员忘记密码

管理员用户页面可生成/显示/复制并设置新密码，随后目标网页登录会话失效；保留游戏账号、数据、订阅和节点分配。
旧PBKDF2哈希不能反查成明文。项目所有者可以重置管理员与自己；普通管理员只能重置成员，不能夺取所有者。
新密码仅在本次设置页面和交付私有文件可查看，服务器不保存明文；再次忘记就重新设置。
平台网页登录账号与游戏账号是两种身份。一个游戏账号不应同时在APK与Linux重复登录；Linux默认负责挂机，APK独立选择另一个可用账号。


## 0.10.13 就绪与多文件编辑
常驻插件加载/热重载晚于进服时，宿主为该插件补发onBotReady；同Runtime同handler一次，重载新Runtime重新初始化，不重跑事件源码。
MCP api_plugin_file_read/write支持scope=code,file=modules/core.lua及scope=data,file=ai_prompts/core.txt。file为空兼容旧入口。AI可以按真实行号分页读取模块并修改；不得将只有main.lua文件名当作业务源码。
Chat OAuth接口见MCP接入教程：/mcp/oauth/linux-server，先登录并授权，只让管理员/有效订阅使用。源码公开包没有任何访问令牌。


# AI能力与运行回归 · 0.10.15

工作区保存项目和任务，并不限制Agent只能改当前插件。默认开放已注册工具，api_call调用工具箱真实功能区API；plugin_manage(action,plugin_id)可指定其他已安装插件，省略plugin_id使用本工作区插件。其他工作区先查询真实ID再读取或修改。玩家显式关闭的工具与技能保持关闭。消息分支和新会话保留显式空工具列表，重载不能自动恢复默认权限。

技能默认可读取全部库，正文按需读取。skill_search(query)支持多个方法名，以相关性排序，返回skill、section、line、offset；直接load_skill(skill,section,offset,max_chars)读相应位置，避免从头读整份手册。主入口仍遵守当前项目语言；不能把Lua语句写进Python入口。

每次运行32次工具后保存检查点等待继续，连续3次失败另列实际错误；单次模型回复不再额外限制8个工具。这是Agent运行保护，独立于插件AI和命令链调度，不限制game.sendCommandFast、CommandBuffer、TXT整链Hz。该检查点不是供应商上下文长度。模型上下文、输出预留和压缩阈值独立保存。

POST /api/youyuan/agent/model {id,model}独立保存模型选择，不改变提示词、技能、工具权限。GET /api/ai/models返回context_window、max_output_tokens、compression_percent。模型编辑留空密钥保留原密钥，保存失败不更改当前有效配置；密钥不在公开列表返回明文。

聊天输入：WaitMsg拥有该玩家的触发词输入，但onChat观察仍收到一次消息；AI wait_chat观察独立缓冲。指定events.subscribe按账号内核隔离，subscribeAll返回完整包封装。Text、LevelEvent、SetActorData、NetworkChunkPublisherUpdate、SyncActorProperty等基础消费所需包应由接收表放行。

listen_packet用真实Go类型名，未知名称或未连接明确失败。收到空列表仅表示该观察窗口没有匹配包，不代表该功能不存在或已经执行。所有玩家的血量与击杀者只能来自实际服务器数据。发送、服务器回包与游戏可见结果分开验收。

本轮Go回归：配置磁盘重载/失败回滚、聊天等待期间观察、跨账号订阅隔离、多词技能定位测试通过。Linux曾真实AI生成并发布Lua，读取计分7；工作区后来归档，测试插件停用。最新版本UI手机/桌面截图、真实玩家.help/死亡显示、普通Chat挂载MCP、完整混合Runtime与LinuxC++仍保留待办；见需求逐项验收表。

确认查询修复：旧WS/玩家WithResp实际无限等待，宿主现改为5秒超时并明确报告反馈/权限/连接问题。R21A初始化用Fast保持sendcommandfeedback=true，commandblockoutput=false，避免关闭自己依赖的查询回包。该超时不限制Fast发包或整链Hz。此前最终构建复验在第9项getScore遇到35秒HTTP超时，已保留失败记录，修复后重新验收。


## 0.10.16 宽寄存器内核
单函数编译寄存器上限已提升至5120，包含参数、局部变量与编译器临时值，并不等于可以声明5120个local再无限调用。VM数据栈初始5120，嵌套时按需增长至1310720槽位，调用栈256。超限编译错误带required/limit和源文件位置。Lua源码语法与常驻require机制保留；原引擎预编译FunctionProto字节码不兼容，需重新编译源码。内部指令从uint32扩展到uint64，字节码存储容量翻倍，不等于整个插件内存翻倍，也不代表命令处理速度翻倍。通过plugin.capabilities().lua_limits读取当前真实配置。

## 0.10.29 主动服务端转发与三语言接口

目标主要是普通网易租赁服，不依赖用户上传玩法组件。以下诊断接口在机器人本机运行；主动服务端模块为另一个可选方案，不能用山头服模块实测替代租赁服广播能力。

本地协议底层Dial使用readerLimits=false，已关闭单批包数量限制。监听器在本地业务筛选之前计数；captureStart({types={"*"},filter={every_ms=0}})可观测已下发解码包，显式指定LevelChunk等大包才能抓它们。登录握手私密数据不进入公开抓包。这里的缓存容量不是服务器广播权限；修改它不会制造未下发的数据。

新增共同API：

| API | 参数 | 实际作用 |
|---|---|---|
| diagnostics.receiveTransport | 无参数 | 当前协商协议ID/版本、注册包ID列表、解码前帧数/字节数/未知包/解码失败；不保存包内容和认证信息；不需要服务端组件 |
| events.onServerObservation | handler(event) | 主动模块所有事件；e.event_type、e.data、e.event_id、e.session、e.received_at、e.source、e.evidence |
| events.onGlobalSwing | handler(event) | 只接受有明确player_id的swing报告；不把伤害、命中攻击或任意Animate伪装为空挥 |
| events.onGlobalHealthChange | handler(event) | 有明确entity_id且实际current合法的health_changed；返回e.health.current/max/previous |
| observerBridge.serverHistory | {session?,after=0,limit=256} | 当前连接历史与cursor/reset/lost/capacity；不是网络查询，重连清空 |
| observerBridge.serverSnapshot | 精确entity_id字符串 | 独立实体快照缓存，known/current/max/previous、session、received_at/observed_at/stale/timestamp_valid；不受无关历史滚动影响 |

缓存使用可配置settings.actors与settings.stale_ms；模块最大血量未知为null，不默认20，不从damage_text反推出生命值。ID为字符串，禁止用浮点数传超长RuntimeID或UniqueID。server_engine_verified=false明确表示宿主收到模块报告，不能代替本轮引擎验收。原生entities.health(runtimeId)、player.equipment(name)、player.rotation(name)继续基于实际下发包，不覆盖或冒充为全服实时数据。

Lua常驻插件：
```lua
events.onGlobalHealthChange(function(e)
  local h = e.health
  print("模块血量 " .. e.entity_id .. ":" .. tostring(h.current))
end)
events.onServerObservation(function(e)
  if e.event_type == "player_death" then
    local d = e.data -- entity_id/attacker_id/attacker_known/cause
    -- 仅attacker_known为true时归属击杀，按event_id去重。
  end
end)
local history = observerBridge.serverHistory({after=0,limit=32})
local snapshot = observerBridge.serverSnapshot("精确实体ID")
```

Python常驻插件（使用注入的命名空间，不每次启动解释器）：
```python
def changed(event):
    health = event['health']
    print(event['entity_id'], health['current'], health['max'])
def on_init(host):
    events.onGlobalHealthChange(changed)
    events.onGlobalSwing(lambda event: print(event['player_id']))
def snapshot(entity_id):
    return observerBridge.serverSnapshot(entity_id)
plugin_api = {'snapshot': snapshot}
```

C++常驻插件（使用sdk/cpp/youyuan.hpp，注册回调名称）：
```cpp
class Observer : public yy::Plugin {
  yy::json last_;
public:
  Observer(const char* context, yy::HostAPI host): Plugin(context,host) {
    on("health", [this](const yy::json& args) {
      last_ = args.at(0); return nullptr;
    });
    client.events.onGlobalHealthChange("health");
    expose("last", [this](const yy::json&) { return last_; });
  }
};
YY_PLUGIN(Observer)
```

events.onGlobalSwing的global表示已安装的服务端模块不按观测者视距筛选，不表示任意第三方服务器都支持。NetAllay API0.29.0模块已编译，真实Allay启动未验收；网易ModSDK行为包具备血量/攻击/死亡，只读查询与引擎注入身份白名单，但没有经核实的网易服务端空挥接口，明确swing_supported=false。必须安装在游戏服务器引擎中；仅在工具箱装一个Lua/ZIP不会建立该能力。rpcProbe匹配Capabilities确认接入后再使用。

Linux C++实际已有Python3独立进程ELF运行器，修正了plugin.has/capabilities曾错误显示“不支持”的信息；Android仍在宿主进程内执行native，不能承诺相同崩溃隔离。SDK回调和跨模块通信保留，新增接口不改变命令Fast路径、整条链频率或加命令CPS限制。


# 机器人调用与多账号协作 · 游园 0.10.40

这是宿主接口说明。跟随玩家、选择路线、轮换频率和离线回收策略由插件决定；安装工具箱不会自动启动示例插件。Lua、Python、C++与MCP使用同一套已注册方法。

## 玩家怎样添加账号

进入「账号 → 机器人托管池」，添加独立账号、填写它的游戏Token、签发该Token的认证网址和目标服务器。成功保存后会出现在账号列表。保存不是进服；点击启动，并以实际连接状态为准。同一个游戏账号重复登录可能互相踢下线。

|字段|填写内容|注意|
|---|---|---|
|id|例如 observer_01|本工具箱内唯一ID，不是游戏昵称；不要用中文或空格|
|name|例如观察机器人1|账号管理显示名；真实游戏名读取bot.self().name|
|token|这个游戏账号的有效Token|必须对应auth_url；不要放入公开技能文件或打印日志|
|auth_url|签发该Token的认证服务完整HTTPS网址|例如自己的认证服务；不能把其他站点的Token直接换网址使用|
|server|所选服务器的实际连接标识|山头服示例94F195C41B5；其他类型使用连接页/认证服务实际支持的标识，不能把room_id假装成已支持的独立参数|
|password|目标游戏服务器密码，可选|不是平台登录密码|
|keep_alive|true或false|连接保活配置；失败要查看实际错误，不表示永不掉线|

账号池自动绑定当前真实平台用户。普通用户只能管理自己的机器人；平台管理员可管理授权范围内的全部托管账号。平台管理员、游戏OP、服主分类分别是不同的权限。列表不返回Token；用户在自己账号的凭据入口查看、复制，不把密钥公开给其他人。

## 已注册的方法

|方法|参数|作用|
|---|---|---|
|accountsPool.add|account对象|保存Token、认证网址、服务器到自己的池|
|accountsPool.list|{all:false}可省略|自己的账号与实际连接状态；管理员可all:true|
|accountsPool.start|id,{server,password,connect:true}|启动独立内核并请求进服，返回connection_requested|
|accountsPool.status|id|读取connected、engine_ready等实际状态|
|accountsPool.stop|id|停止账号连接和保活；不删除凭据|
|accountsPool.acquire|{player,server,id?}|预留空闲机器人并绑定；不自动进服|
|accountsPool.bind|id,{player,server}|持续绑定目标；不自动传送|
|accountsPool.release|id,{idle_seconds:60}|释放绑定，等待期间可复用，期满停止连接|
|accountsPool.bindings|{all:false}可省略|持久绑定状态|
|accountsPool.invoke|id,{method,path,query?,body?}|调用这个机器人的真实/api/接口|
|accountsPool.observationPlan|{accounts:[id...],players:[],dwell_seconds:2,radius_blocks:48}|产生错峰路线，排除已知机器人，预留一个接替节点；不执行路线|
|accountsPool.handoff|incoming_id,outgoing_id,{player,mode,interval_seconds,timeout_seconds,next_player?}|尝试先接替再释放；只有新区域发布匹配目标时ready=true；失败保留原观察|

invoke内的method默认GET，path仅支持真实/api/路径，不能借此管理或盗用别人的账号池。跨账号命令、消息和插件函数也保留accounts.sendCommandsFast、accounts.sendMessage、accounts.callPlugin，沿用原定向授权。模块通信没有取消。

## Lua：添加、请求进服和读取结果

```lua
-- Token应来自用户私有配置，不要硬编码进公开ZIP。
accountsPool.add({id="observer_01",name="观察机器人1",
  token=config.game_token,auth_url=config.auth_url,
  server="94F195C41B5",keep_alive=true})
local requested = accountsPool.start("observer_01", {server="94F195C41B5"})
local state = accountsPool.status("observer_01")
-- 游戏连接是异步的。state.connected=false时稍后查询；不要在事件回调里忙等。
local response = accountsPool.invoke("observer_01", {
  method="POST",path="/api/youyuan/game/call",
  body={method="bot.self",args={}}
})
if response.ok then
  local real_game_name = response.result.name
end
```

这里config代表插件自己读取的私有配置表，不是宿主自动注入的全局变量。参数错误或权限不足会抛Lua错误，可用pcall处理。跨账号网络等待释放Lua VM锁，游戏接包与其他回调不用等它结束。

## Python与C++：同一调用格式

Python插件已经注入accountsPool和observerBridge，也可从on_init(host)使用host.accountsPool、host.observerBridge。C++使用SDK的yy::Client。不要把运行支持范围等同所有平台均可加载任意ELF。

```python
state = accountsPool.status("observer_01")
response = accountsPool.invoke("observer_01", {
    "method":"POST", "path":"/api/youyuan/game/call",
    "body":{"method":"bot.self", "args":[]}
})
if response.get("ok"):
    real_game_name = response["result"]["name"]
```

```cpp
auto state = client.accountsPool.status("observer_01");
auto response = client.accountsPool.invoke("observer_01", yy::json{
  {"method","POST"},{"path","/api/youyuan/game/call"},
  {"body",{{"method","bot.self"},{"args",yy::json::array()}}}
});
```

## 观察与事件：插件不必逐次猜包

当前账号调用observerBridge.watchStart({player,mode,interval_seconds})启动可选单目标观察，watchStatus读取状态，watchStop停止。mode为camera/follow/hybrid：相机观察、移动本机器人、两者一起；默认camera。通过accountsPool.invoke向另一个账号发同样的方法即可控制那个账号。不会强制玩家视角改变。跨维度只能尝试follow/hybrid并查看服务器结果，camera不支持跨维度。

已封装events.onSwing、events.onHealthChange、events.onEntityHurt、events.onPlayerDeath等事件，实体读取entities.get/health、player.equipment/rotation和bot.self；完整字段、真实来源与未知值规则读相应语言章节。observerBridge.worldEvents({server,cursors,limit})按需合并多个账号实际收到的事件；保存返回cursors，下次继续读取，不能重复累计。observerBridge.health(runtimeId,{server,dimension})查询观测血量；RuntimeID要结合服务器、维度和会话解释。

间隔是提交/调度目标，不是服务器广播承诺。只发命令的Fast通道不会逐条等回包；watch的位置查询为确认查询并可能受网络延迟影响，慢周期合并且记录missed_intervals。region_updated只证明收到匹配的NetworkChunkPublisherUpdate，不证明每个挥臂都收到了。48格仅为此前测试参考，不保证全服、跨维度或零漏包。多节点不能生成服务器没有下发的包。

## MCP与AI怎么调用、怎么查文档

已授权的MCP游戏调用工具接受以下JSON，其他方法只替换method/args：

```json
{"method":"accountsPool.invoke","args":["observer_01",{"method":"POST","path":"/api/youyuan/game/call","body":{"method":"bot.self","args":[]}}]}
```

HTTP也可POST /api/youyuan/account-pool，body为{"action":"status","args":["observer_01"]}。公网需已有OAuth/控制授权，不把游戏Token当成MCP登录凭据。技能库中的account-pool-guide、observation-cooperation及对应语言的事件/实体章节可按需加载；GET /api/youyuan/sdk/catalog查询实际注册的方法，MCP使用api_youyuan_docs的query.language和query.method读取对应说明。AI应先读实际方法说明，再生成代码，不能猜不存在的接口。

## 出错怎样判断

|现象|判断与处理|
|---|---|
|connection_requested=true但connected=false|只是进服请求；查询status/连接日志，检查有效Token和认证服务|
|同账号被踢或反复重连|排查重复登录；不是靠加快重试解决|
|观察需要OP、目标查询失败|核对这个机器人实际权限、目标是否在线以及支持的命令通道|
|submitted=true、confirmed=false|只提交了；不能当服务器已执行或已传送|
|handoff ready=false|接替未取得对应区域证据，原观察保留；检查incoming_status.error|
|事件没有、health未知或stale=true|当前来源没有同步到新数据；不能填假血量或推断最近玩家为攻击者|
|无权操作目标账号|使用本人池；管理员仅在授权范围内管理|

实际测试记录与未完成事项另外保存；文档中的示例不表示所有服务器都已验证。


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


## 0.10.41 身体跟随的重复TP优化

watchStart增加follow_radius_blocks，默认16；follow/hybrid首次仍提交一次TP以建立区域，之后只有自身服务器坐标未知/过期、跨维度或离开半径才再次TP。可设置0恢复每轮TP；半径不是动作广播保证。camera方式不使用身体距离判断。配置只影响观察原语，未改变Lua/命令链Hz、Fast发送或插件业务逻辑。watchStatus增加follow_teleports、follow_skipped_near；最后提交时间只在确实发送命令时更新。

原watch仍依赖当前机器人能执行querytarget。OP标识不保证这个命令可用；实际测试中部分子账号收到权限1但查询失败。主账号可用自己的合法游戏权限给自己账号池内的其他机器人设置相机，实现集中指挥，不必让每个节点独立查询。普通用户调用自己的池，不能借此控制其他用户或普通玩家。该方式本轮固定目标四观察节点各45/45，20位置连续移动各7/86，仍不能宣称全局连续覆盖。

Lua: observerBridge.watchStart({player="目标名",mode="hybrid",interval_seconds=0.25,follow_radius_blocks=16})
Python: host.observerBridge.watchStart({"player":"目标名","mode":"hybrid","interval_seconds":0.25,"follow_radius_blocks":16})
C++: client.observerBridge.watchStart(yy::json{{"player","目标名"},{"mode","hybrid"},{"interval_seconds",0.25},{"follow_radius_blocks",16}});
MCP游戏工具body: {"method":"observerBridge.watchStart","args":[{"player":"目标名","mode":"hybrid","interval_seconds":0.25,"follow_radius_blocks":16}]}。请按实际验收报告区分源码优化与游戏改善，不把少发TP当已实现全局监听。


## 摄像机原理与固定远距验收（2026-10-12）

watchStart的camera模式发送的是服务器原版 `camera "机器人名" set minecraft:free pos X Y Z` 指令，由独立工作线程查询目标坐标并更新自由摄像机；不是TP、客户端作弊视角或隐藏数据通道。follow/hybrid才会移动机器人身体。camera模式需要调用者能执行camera/querytarget，不能把接收者无OP等同无人拥有指令权限；主控可用合法OP权限给本用户账号池内的非OP接收者下发camera，接收者本身无需OP。

现代官方基岩版命令资料列出attach_to_entity（摄像机位置附着随实体移动）和targetEntity（固定位置持续看向实体）。当前网易山头服实测attach_to_entity及驼峰targetEntity均返回commands.generic.syntax；下划线target_entity与free pos ... facing "玩家名"成功。target_entity只持续追踪朝向，不能把它包装成已实现原生位置跟随。没有验证支持的功能不自动使用，不安装玩法组件，不对普通玩家修改摄像机。

本轮2026-10-12 18:56:14–18:57:47北京时间，儒雅猎杀者与火神靠谱军团身体固定相隔14450.755格，两个测试账号自身UpdateAbilities.command_permissions均为0，主控布汲岛只下指令，其接收不计分。暂停业务插件，禁止TP/teleport/含execute的测试命令，反复querytarget确认双方位置：所有采样身体位移为0。未设相机0/30；仅设置一次远处free相机30/30；target_entity20/20（目标固定）；facing选择器20/20；清相机后0/20。抓包覆盖0，捕获的是接收账号的服务器下行Animate，不是发送账号自己的出包或主控收包。

这是受控普通权限协议账号的固定距离证据，不等于未修改手机真人、多玩家移动、跨维度或长期零漏包。前面的20位置测试每秒TP256格属于移动压力试验，不能混作身体固定的远距证明。连续移动相机仍大量漏包，调高Hz没有被证明能消除此问题。测试后清相机、恢复原权限/配置并停临时账号，原业务插件恢复；此脚本不随软件作为默认运行插件。

固定距离验收：先暂停会跟随/传送的插件及任务，确认原语已停止；如果停止操作会恢复位置，必须在测量窗口之外完成。记录测试双方身份、UUID/UniqueID、原始权限包、维度与开始/结束坐标；只统计明确对应发送实体ID的下行包，不能把多个观察者的重复包相加。无OP且不主动装客户端的手机真人仍需单独验收。


## 0.10.42 导出下载中心与可配置镜头

Linux/网页插件列表→分享→导出：生成完整私有ZIP后直接下载，并保存到“导出下载中心”。中心列出文件名、日期、大小、SHA256，可重复下载、复制中心网址或只删除备份。下载中心网址为 `https://toolbox.youyuanqi.dpdns.org/?device=linux-server&exports=1`；需登录并有该执行节点权限。子账号使用 `export_account=账号ID`，由账号池校验是否本人拥有；不能通过改URL控制别人的账号。备份存在执行设备的“插件导出/”，不是原作者服务器或公开市场。每次导出独立文件，不覆盖旧备份；Lua包含插件文件/配置/数据的完整目录，Python/C++包含整个插件目录和资源。

MCP：POST `/api/plugin/export` body `{id:"插件ID"}`；Python/C++用POST `/api/youyuan/runtime/export` query `{id:"插件ID"}`。响应有export_id、export、download_url，原path/src_path保留给APK系统选择器。GET `/api/youyuan/exports`列记录；GET `/api/youyuan/exports/info?id=导出ID`读元信息；GET `/api/youyuan/exports/download?id=导出ID`由登录后的网页下载ZIP；POST `/api/youyuan/exports/remove` body `{id:"导出ID"}`只删除该备份。MCP调用download返回经权限验证的元信息且downloaded=false，不把ZIP二进制塞进AI上下文。删除需要控制权限。备份包含自己的配置和私有数据，请按需私下保管。未导出的历史磁盘ZIP不会假装已有下载记录；重新点导出即可。

Lua/Python/C++共享watchStart新增配置：camera_height_blocks（脚部坐标上方高差，默认2），camera_pitch_degrees（留空保持原行为；-90向上、90向下），camera_yaw_degrees（默认0），camera_facing_player（默认false；每次free更新时facing玩家；与固定俯仰二选一）。例如camera模式高度8、俯仰90，从玩家上方俯视；只改镜头，不移动身体。高度/俯仰不承诺扩大服务器动作广播或减少漏包。原生target_entity仍只追踪朝向，当前网易服尚不支持已测的attach_to_entity。先接替再释放、错峰规划、账号池和模块通信保留；区域就绪不等于目标事件齐全。

本轮对照须分别统计：固定低镜头/高镜头俯视、多人分区常驻/单节点切换，以及跨维度。接收与发送账号自己的权限为0；有OP主控只下指令，其收包不算普通权限结果。明确身体坐标、维度、测量时间和交集重复；受控正常权限协议账号是有效工程样本，但没有自动包含手机UI/输入协议的全部差异。


## 0.10.43 插件生成文件

plugin.listOutputs(relative_dir) 与 plugin.exportFile(relative_file) 已接实际数据目录；生成文件可在网页下载原格式。完整使用说明见《插件生成文件下载说明.md》或当前语言技能 plugin-outputs.md。不要混淆整包备份与运行结果导出。


# 0.10.44 实时回调与查询

事件和定时器共享常驻 Lua 状态；不要在每个 UpdateBlock、50ms 周期或战斗事件里同步查询服务器。保留原 game.getBlockTileRaw，新增不占用 Lua 回调的真实 WS 查询：

```lua
local pending
function inspect_tick()
  if pending then
    local reply=game.commandResult(pending,0,true)
    if reply.pending then return end
    pending=nil
    if reply.error then print(reply.error) else print(reply.result.block) end
  else
    pending=game.getBlockTileAsync(1,64,1).ticket
  end
end
util.scheduleTick(2,"inspect_tick")
```
仅查询已加载区块，不传送；错误不当成空气。ticket 与插件/账号绑定，完成后consume，卸载清理；同一业务可保持一项在途，避免不必要重复查询。该策略只管理方块查询，不给命令链Fast添加回包等待或CPS限速。

Python同名：`ticket=game.getBlockTileAsync(x,y,z)`；C++：`client.game.getBlockTileAsync(x,y,z)`，同样用commandResult取得真实结果。

周期公告或人数监控用 `game.sceneStats()`（Python相同；C++ client.game.sceneStats）。返回cached（含离开视距后保留记录）、present（当前存在）、fresh/active_actors（当前会话不陈旧）、packet_count、time；不是全服实体计数。需要详细状态时用entities.get(runtimeId)，按实际变化读取；game.scene保留完整快照，适合手动诊断而非高频遍历。

plugin.stats()/MCP /api/plugin/profiler?id=... 另有last_vm_wait_ms、max_vm_wait_ms、last_callback_error，区分入队等待、等待Lua VM和回调体耗时。max是自本次插件启动累计的最大值，不能拿旧峰值当当前延迟。CPU/单插件内存未测量时继续null。

没有因此扩大未广播数据覆盖；未收到远处血量仍未知。界面响应时间、收包到插件回调、命令提交到服务器确认是不同指标，验收分别记录。


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


# 0.10.46 群成员、聊天与观测调用

## QQ昵称与真实身份
昵称显示优先管理员设置的nicknames，再用事件实际提供的群名片/card、nickname等，未知显示群成员。真实成员标识与昵称的映射仅保存在用户私有配置，不内置到公开APK。openid仍用于权限，不从昵称反推权限，也不能把openid翻译成未收到的QQ号；可以在真实群成员卡片修改显示昵称。成员标识仅在折叠的身份详情中显示。OneBot优先群名片。

## 自然群聊
assistant.companion打开群聊；companion_all_messages允许实际收到的普通消息触发。已有关键词/@/30秒会话继续工作，首次等待5秒，服主回复停止，30秒无新消息结束。去掉每条回复前的AI陪聊标签，不冒充服主。群内命令不作为普通聊天。

GET /api/qq/messages?group=已配置群标识&limit=30：只返回这个群实际接收的近期消息，name/text/timestamp/id；不返回成员内部ID。最多80条内存记录，重启从新事件继续；不声称读取QQ客户端全部历史。GET /api/qq/assistant 的members值是可读昵称，键仍是权限身份。MCP专用工具api_qq_messages以及api_call同样可读。

AI只获得本群近期12条背景，每条最多400字符；明确标记为不可信用户消息，不能赋予权限。成员模式遮蔽坐标；工具白名单仍由宿主强制执行。每个人与每个群的对话状态分开。官方QQ是否下发普通群消息由平台能力决定；只接收@时仍可@继续，不宣称能够看到未下发消息。

## 绘图配置与真实图片发送
设置→群服互通→绘图模型→添加或更新绘图接口：填独立的配置名称、API地址、图像模型ID、API密钥，点击保存并选用。设置存assistant.image_model，密钥在用户自己的模型库，公开APK不内置密钥。密钥留空保留原配置。文字模型与绘图模型独立；未选择绘图模型就不给群AI挂image_generate工具。

当前图像适配：POST {base}/v1/images/generations（base已有/v1不重复），model/prompt/n=1/size=1024x1024/response_format=b64_json；只接受有效PNG/JPEG的data[0].b64_json，不获取任意模型返回URL。描述1–2000字符；响应最多4MiB，QQ当前上传预算2MiB。接口返回错误不会报告生成成功。图片经官方/files上传得到file_info，然后/messages发送；OneBot用base64图片段。两步都成功才给AI已发送结果。没有绘图接口时不能拿DeepSeek文本生成当图片生成。

群内示例：@机器人 .AI 帮我画一张Minecraft森林小屋。收到图片后可以继续聊天。资料卡/玩家头像仍使用已有真实PNG渲染与皮肤缓存，与AI生成分开。当前只用模拟服务核对请求与图像回读，尚未连接付费绘图提供商，不宣称实景绘图通过。

## 插件直接控制宿主观察
以下为宿主API，不要求安装某个特定多功能插件。用户插件决定策略、目标和频率。

Lua：
```lua
observerBridge.watchStart({player="目标游戏名",mode="hybrid",interval_seconds=0.2,
 follow_radius_blocks=16,camera_height_blocks=4,camera_pitch_degrees=90,restore_position=true})
local state=observerBridge.watchStatus()
observerBridge.watchStop()
-- 自己的另一个机器人账号；实际配置中填写自己的池ID。
accountsPool.invoke("池账号ID","observerBridge.watchStart",{{player="目标游戏名",mode="follow",interval_seconds=2}})
local plan=accountsPool.observationPlan({accounts={"池账号ID"},players={"目标游戏名"},dwell_seconds=2})
```
Python：
```python
observerBridge.watchStart({"player":"目标游戏名","mode":"hybrid","interval_seconds":0.2,
 "camera_height_blocks":4,"camera_pitch_degrees":90,"follow_radius_blocks":16})
observerBridge.watchStop()
accountsPool.invoke("池账号ID", "observerBridge.watchStatus", [])
```
C++通过现有client.observerBridge.watchStart(yy::json配置对象)（或client.invoke("observerBridge.watchStart", yy::json::array({配置对象}))）同名调用；实际SDK类型按cpp-runtime章节。MCP用/api/youyuan/game/call {method:"observerBridge.watchStart",args:[配置]}；跨账号用accountsPool.invoke。UI复制插件调用代码按钮以当前参数生成Lua代码。

watch/plan/handoff接口有明确差别：watch真实提交镜头/身体跟随，plan只规划，不启动账号；handoff先收到接替区域更新才释放旧观察。必须记录OP、维度、身体/镜头是否移动。扩大覆盖不等于全服广播，不保证不丢包。停止插件自动取消其拥有的观察任务，保留模块通信和原来的低级接口。

## 抓包与房间搜索
抓包逐条默认折叠，可全部折叠/展开、暂停显示、隐藏内容；暂停和隐藏不停止后台记录，停止按钮才停止记录。预览最多12000字符，NDJSON导出保留宿主已记录的内容；UI预览裁剪不会改动导出。原始包若超过后台记录预算，truncated字段会明确标记，不能把这种记录称为完整原包。页面刷新保留展开条目和滚动位置，不重置到末尾。协议详情和事件时间线默认折叠。

搜索并选择房间打开独立房间搜索页：选择已登录游戏凭据、搜索关键词或room_id，选择后写回连接设置，仍需保存。没有凭据时明确提示添加，不强制跳认证。GET/POST认证房间API本轮返回成功但没有匹配房间，不把空列表当进入房间验收。

独立认证网址：https://auth.youyuanqi.dpdns.org 。平台账号注册（QQ邮箱验证码）与添加网易/4399游戏凭据不同：先平台注册/登录，再接入游戏账号，得到游戏令牌，最后连接游戏。嵌入页加入独立入口与步骤，不把4399游戏列表当Minecraft档案建立成功。实名/验证码按官方要求由账号本人完成。

## 借鉴记录（设计比较，未复制第三方源代码）
- tencent-connect/openclaw-qqbot：https://github.com/tencent-connect/openclaw-qqbot — 腾讯官方MIT项目，群消息/引用上下文与图片发送；绘图能力来自配置模型和技能。
- HuHoBot/PenguinAgent：https://github.com/HuHoBot/PenguinAgent — 昵称缓存、分级权限、多群与按需Skill。Bukkit/Java服务端功能不能直接当网易外接机器人能力。
- Coloryr/Minecraft_QQ-C-Server-：https://github.com/Coloryr/Minecraft_QQ-C-Server- — OneBot桥接、绑定与昵称管理。
- Minecraft-QQBot/BotServer：https://github.com/Minecraft-QQBot/BotServer — 多服/群管理与配置组织。项目已归档，研究设计，不替换本机认证或直接运行未知依赖。

现场屏幕自动化仍报nodeRepl.fetch/超时；源码/接口测试与真实点击滚动验收分开记录。未完成事项见当前未完成清单.md。


# 0.10.48 全局抓包、作业暂停与旧注册兼容

## 设置位置和多选
设置→服务器连接→应用启动时自动连接下方→全局抓包。独立开关，配置按钮打开自己的账号与玩家多选卡片；保存全部配置后生效。技能加载库也使用明显的复选框、选中边框和计数，支持全选/清空/统一保存。

默认全局抓包关闭、camera模式，只移动本机器人原生摄像机，身体不跟随。可选择多个玩家，或动态观察在线玩家。手动勾选玩家会自动关闭动态全选，避免选中名单被自动模式忽略。开启跟随/混合模式可能传送机器人身体，会改变区块加载、触发服务端限制、抢占建筑工作位置；不会故意传送玩家。默认排除已连接所选观察账号的真实机器人名字。

每次镜头切换间隔可配置毫秒，不等于命令链节点频率。完整轮巡约为目标数量×切换间隔。高频负责提交命令，不能保证服务器同样频率完成、零漏包或全服血量同步；原生摄像机的维度与广播限制仍存在。

## 原生选择器镜头
observerBridge.watchStart支持原player字符串（兼容旧写法）以及players数组、auto_players、camera_selector、exclude_players、pause_during_jobs。多目标或camera_selector=true的纯相机模式执行：
execute at "目标玩家" run camera "观察机器人" set minecraft:free pos ~ ~4 ~ rot 90 0
目标坐标由游戏执行器原生解析，轮巡不在每个切换前等待querytarget或命令回包；只记录submitted。普通单目标旧watchStart保留原来区域确认路径，避免破坏已有handoff逻辑。

选择器路径未核验位置时target_position_known=false、target_dimension_known=false、region_updated=false，不能把零坐标或提交次数作为全局覆盖成功。需要交接区域确认时继续用旧单目标watch/handoff。跟随/混合多目标路径依然存在必要位置查询，不能把纯镜头Fast路径速度直接当身体跨维度跟随速度。

## 原生配置API
GET /api/youyuan/global-capture：config、states和global_coverage_guaranteed=false。
POST支持部分字段更新；例如{enabled:false}保留其他配置。配置字段：enabled、accounts数组、players数组、auto_players、mode(camera/hybrid/follow)、interval_seconds、camera_height_blocks、camera_pitch_degrees、camera_yaw_degrees、pause_during_jobs。只控制自己绑定设备/账号池，不能选别人的账号；控制权限与只读权限分开。
程序保存到设备global-capture.json，重启后恢复开关和参数。只管理自己的已连接账号，失效/未连接显示错误，不伪造已连接；不覆盖其他插件已运行的观察任务。需先在账号管理连接要用的机器人。关闭后释放本模块自己启动的任务。

## 导入导出作业暂停
宿主按机器人实例维护空间作业租约，覆盖主程序导入/导出/修补/地图画/皮肤建筑、多机器人导入及game.importBlocks插件导入。区域导出在每个实际参与机器人上增加租约，building.exportRegion与placeContainer的NBT异步作业也会暂停本账号观察。作业开始暂停原生跟随/TP轮巡；作业结束才恢复。多个作业引用计数，结束一个不能误恢复仍被另一作业占用的机器人。
默认相机也暂停，避免抢区块视距；配置pause_during_jobs=false只允许纯相机继续，身体跟随仍暂停。停止观察期间不会把机器人恢复传送到旧起点去打断导入导出。
game.teleport/game.move及常见自己TP的插件命令在作业期间明确返回暂停错误；普通聊天、查询和非移动命令不受影响。不关闭/重载插件，不重置Lua/Python状态。任意插件自行构造原始移动包或绕过标准API不在这种合作式暂停保证内，不能宣称隔离全部自定义行为。

Lua：
```lua
local job=observerBridge.jobState()
if not job.paused then
  observerBridge.watchStart({players={"甲","乙"},mode="camera",camera_selector=true,
   interval_seconds=0.025,camera_height_blocks=4,camera_pitch_degrees=90,pause_during_jobs=true})
end
```
Python同名接口：observerBridge.jobState()，observerBridge.watchStart({"players":["甲","乙"],"mode":"camera","camera_selector":True,"interval_seconds":0.025})。
C++：client.observerBridge.jobState()、client.observerBridge.watchStart(yy::json对象)，ABI保持不变。跨账号保留accountsPool.invoke；配置由插件自行决定，程序页面不要求依赖特定多功能插件。
高频传送插件应先检查jobState().paused并跳过当前轮，避免反复触发暂停错误。直接错误不是“已成功传送”，不得忽略错误后编造结果。

## 旧Lua注册接口兼容
注册参数保持兼容：game.registerTrigger(words,handler)、events.onChat(handler)、events.onJoin(handler)、events.onLeave(handler)、events.onBotReady(handler)、events.onPacket(handler)、events.subscribe(packetType,handler)、util.schedule(seconds,handler)、util.setTimeout(seconds,handler)、web.action(page,action,handler)。handler可为原全局函数名或函数对象。回调参数分别是onChat的(name,message)、onJoin/onLeave的(name)、onBotReady的无参数、onPacket的(packetType,data)、subscribe的(data)，不要把回调参数当注册参数。
上述注册现在也接受Lua函数对象（包括模块返回的函数与闭包）。加载一次、常驻回调，不重跑main.lua。事件、包订阅和停止清理按机器人账号隔离；停止一个账号的同名插件不应移除另一个账号的事件/定时器。
旧ZIP导入仍保留manifest、main.lua、modules、command_txt等资源目录。新的测试实际导入旧ZIP，读取模块，再触发聊天回调；不是只检查语法。没有提供的旧宿主私有注册接口不会伪造一个空函数来骗过导入；遇到具体缺失函数需按原定义补接并验收。

## 本轮证据与限度
原生选择器相机命令在Linux游戏服对本机器人测试，服务器confirmed=true、success_count=1，随后camera @s clear同样确认成功；没有身体TP。这只证明该指令路径可执行，不证明远距离真人动作、多人覆盖率或跨维度血量。
新测试覆盖：旧字符串/函数回调不重置与跨账号隔离、旧ZIP模块和TXT导入、空间租约嵌套与恢复、原生相机不拼querytarget/TP、账号池越权拒绝、多选账号/玩家/技能一次保存。界面行为用模拟DOM测试；实景视觉验收单独记录。


## 0.10.49 多选边界修复与实测
全选列表包含机器人自己时，多目标轮巡自动跳过自身，继续观察其他目标；原单目标只选自己仍明确报错。账号排除名单继续生效。旧抓包页移除重复配置卡片，入口统一在设置→自动连接下方。
0.10.48真实Linux测试：观察非OP子账号的原生镜头40次/秒提交；单方块导出生成272字节mcstructure，导出期间提交计数固定2867，结束后增长至2873，作业状态恢复。测试后关闭全局抓包、停止任务并确认清除镜头。仅为配置/暂停恢复实测，不是远距离多人收包率证明。


# 0.10.50 收包与常驻运行性能

优化保持原始协议字段与旧插件调用形式。packetToMap缓存固定的字段索引与snake_case名，仍生成独立结果，完整转换二进制hex/数组/NBT；不会截短订阅包。订阅列表仅在增删时发布不可变快照，分发不再每包复制列表或争抢注册锁；同次分发的回调顺序及退订语义保持。协议目录所有构造包字段对照旧转换器，带数据的移动/血量及大ID对照通过。

同一台Linux、同一组fixture、单核、各3次：0.10.49移动包转换中位4846ns，当前1681ns（2.88倍）；血量包8082ns到3919ns（2.06倍）；订阅分发5173ns到1934ns（2.67倍）。这是解码后的宿主转换/分发微基准，不是全游戏或网络RTT倍率。移动车体状态缓存约292ns保持；不会把未变化指标包装成加速。

保留game.getScore、getPlayerPosXYZ、getPlayerItem、getBlockTile、getBlockTileRaw、getTickingAreaList、getBlockData/getBlockNBT/getBlockState/getRegionBlockData及placeNBTBlock/setBlockState的同步返回/错误；等待Go网络/文件/工作台时释放常驻Lua锁，其他回调和定时器可继续运行。Lua输入先复制为Go值，返回后再拿锁写Lua；不在两个Go线程同时执行同一份Lua指令，不重跑源码或丢弃模块状态。旧查询实际耗时仍可能包含网络等待。

区域读取、NBT放置与状态修改也使用按账号空间作业租约，避免观察跟随抢占操作位置；参与作业的内部传送保留，不拦成空功能。初始化阶段保留原初始化顺序，不能将回调期协作等待承诺成初始化所有网络I/O都不等待。

plugin.stats()/MCP /api/plugin/profiler增加io_waiting、io_calls、io_total_wait_ms、io_max_wait_ms，区分网络等待与执行墙钟。total/max为本实例生命周期统计，io_total可能包含多个并行等待，不是CPU时间；cpu_ms等未测字段继续null。Python/C++通过同一宿主接收转换/订阅路径获益；本轮Lua旧同步调用锁释放修复不表示Python/C++单语言性能达到了同样倍率。

性能验收：原0.10.49真实game.getRegionBlockData一次1130.55ms，10ms心跳在这段时间运行0次；新版本需要以同样接口真实复测并保存证据。另做连接/确认RTT/队列丢弃样本，不能只用本地空计数循环判断真实服务端完成。

没有改变握手协议ID、加密方式、服务端权限和条件链结果依赖。客户端优化不能使服务端没有广播的包自行出现；当前成果不宣称整体200倍或2000倍，也不通过删字段、停功能或降低事件覆盖率换数据。

方法依据：Go官方性能诊断与CPU profile https://go.dev/doc/diagnostics；fixture CPU样本显示旧snakeCase累计约27.65%，因此优先消除反复转换和分配，而非猜测网络协议。


# MCP联调约定与真实状态 · 0.10.57

宿主提供连接、原始包、账号进程、文件、回调队列和重载；插件实现血条/HUD、业务与跟随策略。API存在不是服务器全局广播保证。摄像机、OP也不保证跨维度目标包。三个常驻旁观按实际连接维度0/1/2分工；有受控下界远距无OP来源20/20记录，不承诺任意真人零丢包。

## 调用格式
MCP工具顶层account为空/main是主号，独立账号用账号池id。method是HTTP方法，query是GET查询参数，body是POST原始对象；游戏方法放body.method，参数放body.args。
例：api_call({path:"/api/youyuan/game/call",method:"POST",account:"main",body:{method:"bot.self",args:[]}})。
game.execute / sendCommandWithResp是确认通道；sendCommandFast只发不等，不代表执行成功。高频FUNCTION不要等待逐条确认；有真实成功依赖的条件才设置确认边界。

## 摘要与原始事件
MCP GET sense/snapshot、sense/events、packets/capture默认view=summary，返回bot简要位置/血量、计数和最近20条元数据。query支持limit、types逗号列表、runtime_id、after_sequence；规范事件还支持entity_type。过滤不创造未收到数据；counts来自保留窗口。完整内容明确query.view=raw；大文件用packets/export下载，不放AI上下文。
Lua events.summary({types={"swing","health"},limit=20})；Python host.events.summary({"types":["swing","health"],"limit":20})；C++ client.events.summary(yy::json{{"types",{"swing","health"}},{"limit",20}})。抓包摘要用events.captureSummary同样参数。
packet.catalog()列所有注册包ID/名称/字段数/是否能抓包。每个普通已解码包可用events.subscribe("包名",handler)；自定义转换events.define({name="my_event",packet="Animate",fields={target="entity_runtime_id"}},handler)。登录/握手隐私数据不进入抓包；大区块包不是小动作事件。UpdateAttributes是真实health来源；Animate只代表实际动作包，不用伤害伪造空挥；ActorEvent不保证带可靠攻击者。所有字段用packet.schema或/api/youyuan/packets/schema?name=Animate按需读取。

## trace与失败恢复
每次MCP工具调用structuredContent.mcp_meta含trace_id、http_status、elapsed_ms、outcome、retryable、execution_confirmed、auto_retried=false。公网中继同trace ID跟随到宿主；GET /api/youyuan/mcp/traces?trace_id=...读取最近元数据。保留256条内存记录，只含本人/管理员可读范围，不存密钥、命令正文、源码或聊天提示词。
not_sent表示中继未提交，可重试；timeout_unknown/write_result_unknown表示写操作可能已执行，必须读任务/状态后再决定，不盲目重复放置、计分或兑换。server_rejected表示收到明确失败回包，submitted_unconfirmed不是执行成功。云端503/504会带submitted/confirmed/retryable；不偷偷重试非幂等写。

## 长NBT操作
优先building.placeContainer(x,y,z,"minecraft:barrel",Items,options)返回job_id，再building.nbtJob(job_id)查询状态与verified。会占用真实作业位置、可能清理制作背包，先检查说明并暂停观察运动；不能在5秒同步调用里假装完成整条NBT自测。placeNBTBlock旧同步接口仍保留兼容，超时结果可能未知。当前没有统一覆盖所有旧同步方法的持久异步任务；重启后内存NBT任务不恢复。

## 账号池最小成功路径
1. accountsPool.list({all=false})获得自己账号id；has_token只证明存了凭据。
2. accountsPool.start(id,{server="服务器标识",connect=true})启动；不是“请求成功就进服成功”。
3. accountsPool.status(id)轮询engine_ready/connected/phase/fleet_ready。只有connected=true才验证当前server，credential_state=verified_for_current_server。
4. accountsPool.invoke(id,{path="/api/youyuan/game/call",method="POST",body={method="bot.self",args={}}})调用该账号。
/api/fleet是旧bot_tokens同进程池，空[]不代表账号管理池没有机器人。4399平台注册/启动器绑定/能进入网易游戏是不同步骤；宿主不能根据存有token就声称全部注册通过。

## 群服互通
死亡和重生不是退出/进入。名单以成功/list的名字为准，坐标查询失败不注销在线状态；周期事件只刷新名单，不逐玩家同步坐标。真正死亡/重生的事件仍由实际包与缓存处理。
.日志查游戏最近聊天；.群日志 [1..80]查机器人真实收到的近期QQ群消息，默认20，可分段。不是QQ全部历史或原生合并转发卡。普通成员仍隐藏坐标。
能发送图片不等于能理解表情包；当前图片发送/绘图配置可用，图像语义识别和表情图库尚未实现，不能让文字模型假装看图。QQ官方窗口/权限仍有效。

## 源码边界
公网宿主源码、SDK压缩包、Git目录、source map等下载路径已拦截；MCP资源来自本机已部署文档，不自动连接GitHub。玩家自己的插件项目仍可读写/导出，不能为隐藏宿主而砍掉开发功能。网页HTML/JS必须发给浏览器所以可查看；APK二进制也不能承诺无法逆向。源码审计不能替代多用户越权与部署目录完整验证。


## 0.10.60 全局采集控制

宿主新增 accountsPool.captureStatus / captureConfigure / captureRoster；完整三语言例子、账号共享边界、动态借调回防、投放数量、失败冷却与真实收包限制，见对应语言技能 fleet-scheduling.md。三语言调用同一宿主，不需要复制调度到业务插件。高频镜头提交不等待逐条回包；维度/就绪判断使用真实连接状态。主名单排除全部受管机器人；captureRoster 的 include_bots 仅供诊断。MCP可在本机技能目录读取这一章，无需访问GitHub。


## 0.10.61 视频运行说明
详见同目录《视频运行与验收-0.10.61.md》。新路径为认证后HTTP/MCP工具，未新增语言专有函数；不要杜撰Lua/Python/C++绑定。运行中R21B插件可按需读取host_skills/video-runtime-01061.md。
