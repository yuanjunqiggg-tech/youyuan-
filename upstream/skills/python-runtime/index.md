## Python 常驻异步环境

每个Host创建一个常驻asyncio事件循环线程，生命周期等于插件实例；不是每条事件asyncio.run。`async def on_init(host)`、`async def on_event(event, host)`、`async def on_stop(host)` 以及注册的async回调和plugin_api导出函数都会被实际await。事件仍由宿主串行处理；一次回调等待最多120秒，超时取消该协程。后台asyncio.create_task可以在事件之间继续执行；停用取消后台Task、异步生成器并关闭循环/线程。

同步 `host.game.*` 会做宿主HTTP请求；在协程中需要避免阻塞循环时使用 `await host.invoke_async("game.sendCommandsFast", commands)` 或 `await host.call_async(path, body)`，它们把阻塞I/O放入该循环的executor。插件内自己的async函数按Python asyncio方式await。同步旧插件原写法兼容。Android仍有共享解释器/GIL，Linux每插件独立Python进程；不能称Python纯CPU代码并行无限扩展，也未声称比Lua快1000倍。

```python
import asyncio
commands=["scoreboard players add counter test 1"]*25
async def on_init(host):
    await host.invoke_async("game.sendCommandsFast", commands)
async def on_event(event, host):
    await asyncio.sleep(0)  # 让出自己的事件循环，模块变量仍保留
    return {"ok": True}
```

## 常驻调度、诊断与能力查询

旧 `util.schedule(seconds, handler)` / `setTimeout` / `cancel` 保留。新增 `scheduleSeconds(seconds, handler)`、`scheduleHz(hz, handler)`、`scheduleTick(ticks, handler)`。Tick是本地名义50ms，不是服务器时钟。频率针对整个回调/整条链；同任务不重入，不补无限积压。Python/C++一次定时回调未完成时不会继续往队列塞同任务。慢回调漏过的周期记missed，主动暂停期间也计入missed。

`util.listJobs()` 返回当前插件任务的id、handler、paused、calls、missed、failures、last_ms/max_ms/total_ms/last_error。`pauseJob(id)` / `resumeJob(id)` 只控制当前插件的任务；暂停不强杀已经执行或入队的回调。停用取消所有定时器及订阅。SDK每实例串行队列256项；Lua事件串行队列128项，满时记dropped而不新建无限goroutine。旧secondary机器人仍可按首次事件懒加载同插件VM；自己的变量不混入其他账号。

`plugin.stats()` 查常驻回调次数、错误、墙钟耗时和队列信息；Lua耗时不含获取VM锁前的等待，定时任务耗时包含其VM/SDK队列等待。CPU、内存、GC没有可靠逐实例测量，字段为null。不要把这些墙钟数当CPU负载或声称1000倍。错误保留插件文件名、行号、handler和耗时。Lua到Lua跨插件调用不再JSON往返，Python/C++仍使用必要的跨语言边界。

`plugin.has("game.sendCommandsFast")` 检查接口；`plugin.has("runtime.cpp")` 在Linux为false、Android为true。`plugin.capabilities()` 返回平台/方法列表/语言运行器能力。接口存在不等于服务端允许动作，也不等于已观测到其他玩家血量。方法文档按需读；外部MCP调用资源、任务、stats要走SDK的plugin_id/account上下文，不能当全局游戏调用。

Lua: `timer=util.scheduleHz(20,"round"); function round() game.sendCommandsFast(commands) end`。Python: `timer=host.util.scheduleHz(20, round)`；C++: `client.util.scheduleHz(20,"round")`。参数20是每秒完整回调次数，不是每秒20条命令。

## 插件资源与模块

`plugin.codeDir()` 源码目录；`plugin.dataDir()` 持久数据目录；`plugin.configDir()` 配置目录；`plugin.resourceDir()` 优先resources子目录，旧包无该目录则使用源码根；`plugin.tempDir()` 数据目录下.tmp。`plugin.readResource(relative, "utf8")` 读取文本，二进制跨语言使用 `"base64"`；`plugin.existsResource(relative)` 检查存在；`plugin.listResources(".")` 递归返回path/dir/size；`plugin.extractResource(source, destination?)` 原字节复制到自己的数据目录并创建父目录，不是解压ZIP。

相对路径不能逃出目录，不接受绝对路径、驱动器、..或符号链接。Lua默认配置/数据沿用旧目录以保兼容，旧game.dataDir不变；Python/C++ SDK数据和配置按账号目录隔离。普通Lua ZIP导入保留模块和资源，市场包保留源码及资源、排除私人数据/配置；完整备份ZIP含源码、资源、配置和数据，空目录也保留。ZIP防路径穿越与防解压炸弹的资源容量保护不影响命令发送频率。

Lua `require("lib.helper")` 搜索插件源码根的lib/helper.lua或lib/helper/init.lua，package.loaded在常驻VM内缓存；只在停用/重载后重新加载。Python推荐包内相对导入 `from .lib import helper`，不依赖不同插件的同名全局模块。

## 0.10.7 框架执行路径

`game.sendCommand(command)` 保持旧无等待调用方式；新增 `game.sendCommandFast(command, options?)` 与 `game.sendCommandsFast(commands, options?)`。后者一次宿主调用提交完整数组，没有整个数组的行数限制。魔法/ai 的无等待路径使用 SettingsCommand、保留机器人 `@s` 和相对坐标；提交按输入顺序，网络层按原定时周期合并，不逐条等回包、不默认CPS等待。`submitted` 只表示提交，`confirmed=false` 不代表游戏成功。用户主动给正数 `rate_cps` 才限速。

`game.sendCommandConfirmed` 是 `game.execute` 的别名，等待 CommandOutput；超时是未知，不自动重发。`game.executeBatch` 的 ordered/pipeline/burst/stream 保留可确认路径；其中 window 是在途请求数量，不是整个文件长度。无条件链可选 `mode="fast"`；有条件节点及其直接前一条自动切到确认路径，`@wait/@call/@pause` 控制语义保留。Hz 是完整链的目标轮次/秒，超期不积压轮次。

Lua: `game.sendCommandsFast({"scoreboard players add counter test 1", "scoreboard players add counter test 1"})`。Python: `host.game.sendCommandsFast(commands)`；C++: `client.call("game.sendCommandsFast", nlohmann::json::array({commands}))`。先校验整个数组；网络失败可能已有部分提交，不应盲目重试。Fast不保证原子或同一服务端Tick，也不能自动判断每条成功。显式 PyRPC 在当前测试服100条无执行，不能将旧源码的通道推断为当前所有服务器可用。


## 0.10.7 连续发送与整链频率
默认 rate_cps=0 表示不加人为 CPS 等待。game.sendCommand 不再逐条睡眠20ms；pipeline、submit、burst不再默认限制200条/秒。只有主动配置正数 rate_cps 时才启用发送上限。Lua、Python、C++共用这条分发路径。
发送缓冲按256包或256KiB刷新，部分缓冲由原有传输定时器刷新；不是每256条停一个周期。sendCommand返回submitted仅表示提交，最终效果用回包或计分板核对。无限循环仍应让出定时调度，不能把网络提交率当服务器执行率。
stream中正常回包success=false（如空选择器）不会中断后续无条件命令；有条件节点仍根据上一条真实结果跳过。断线、超时与未知结果不自动重发。32条以内且可装入用户窗口的短链一次提交；更长的连续段按4096条和512KiB拆段，整份TXT可超过这些分段数量。
整链Hz=1000/interval_ms。每轮包含整个文件，运行耗时算在周期内；完成后只等待本周期剩余时间，超过周期则不增加等待，不积压新轮次。每条的delay_ms独立且默认0。状态actual_hz显示真实完成轮/秒，last_round_ms是上一整轮耗时。
Lua批量结果results是可ipairs遍历的table，output是字符串table，success_count是数字。事件回调复用常驻Lua状态。
```lua
-- 同样的循环不需要改为特殊批量API，也不默认逐条限速。
for i=1,10000 do game.sendCommand("scoreboard players add @s yybench 1") end
-- 需要逐项核对结果，或按真实结果进行分支时再用批量/确认API。
local r=game.executeBatch(commands,{mode="stream",window=256,rate_cps=0})
for _,row in ipairs(r.results) do print(row.confirmed,row.success) end
```

## 0.10.3 命令链工具
当前语言通过chains命名空间调用同一机器人命令链；新增chains.trigger释放@pause。跨链行范围/次数/时限/持续与条件边沿请按需load_skill("command-chain",section="control-flow.md")。不另建高频计时器，不递归启动插件任务。

# Python 常驻游戏插件完整开发手册

## 0.9.5 命令通道与用户参数

`game.sendCommand(command, options)`、`game.execute(command, options)`、`game.executeBatch(commands, options)` 现在共用 `channel=ai/player/console/default`。使用 `game.getCommandOptions()` 查看实例默认值，`game.setCommandOptions({...})` 修改本实例默认值。工作区“命令执行参数”编辑项目 `command-options.json`，发布并重新加载后生效，AI 可以直接修改同一文件。参数还包括 `mode=ordered/pipeline/submit`、`window=1..16`、`timeout_ms=100..10000`。`game.execute("testfor @a", {"channel":"player"})` 等待真实回包；只有 `confirmed` 和 `success` 都为真才算执行成功。完整说明与示例通过 `load_skill("command-execution")` 读取，旧手册中仅单参数的写法仍兼容。

版本：APK 0.9.1 / SDK 1.1。本文对照上游插件开发文档和当前 Go 绑定生成，再针对多语言生命周期、回调、配置、错误和发布方式补充。示例是 Bedrock/网易租赁服机器人插件，不是 Java/Spigot 插件。

## 1. 能力与验证范围

Python 和 C++ 在各自常驻运行环境执行插件源码/机器码。十三个命名空间连接到已有 Go 游戏操作能力；为避免重复实现和漂移，每插件保留一个串行绑定适配状态，复用原绑定，不每次创建 Lua VM。该状态只负责宿主 API 适配，插件逻辑仍在 Python/C++。已使用用户指定认证服务真实进服，并验证 Python/C++ 指令、命令流水线、方块放置/破坏与事件回调；并非每个 API 都完成游戏验收。血量、受伤、死亡、生存挖掘等仍需进一步实服测试，不把模拟事件或成功发送当成完成。游戏模式默认保留；只有设置中显式开启“自动维护创造模式与飞行”时才启用建筑维护。

覆盖 game、player、world、events、util、plugin、api、bot、building、media、http、system、web。可发送指令/消息、查询记分板、玩家、背包、方块与 NBT、移动飞行、读写配置/状态、HTTP、注册回调、创建插件控制页、调用其他插件。完整方法清单也可读取 GET /api/youyuan/sdk/catalog，机器接口 POST /api/youyuan/sdk/call；MCP 的 api_call 可调用它们。

不是所有原版插件二进制/语言可以原样导入：Lua 仍使用原 Lua 插件入口，Python 使用 Python manifest，C++ 使用目标 Android ABI。ToolDelta 的 Player 对象、装饰器和 NTvector 的 engine 模块未被假装直接兼容，需迁移到本 SDK。

## 2. 目录、导入与控制

ZIP 根目录是 manifest.json 与入口文件，可包括资源、www/、辅助源码。

```json
{"id":"my_plugin","name":"我的游戏插件","language":"python","entry":"main.py","events":["chat","ready","join","leave"],"version":"1.0.0","dependencies":{}}
```

Python entry 为 main.py，language 为 python；C++ 为 plugin.so / native_cpp。manifest.events 只过滤通用 on_event/原生 event；events.onChat 等显式监听独立工作。留空订阅常规事件，packet 必须明确订阅以免大量无用事件。文件导入使用手机系统选择器或网页当前设备；不是读取云服务器磁盘。

在多语言常驻插件页导入 ZIP、启动、停止或重新加载。编辑不会偷偷修改当前模块内存；发布后用重新加载启用新版本。重新加载重建插件实例，状态通过持久保存恢复。运行页显示队列深度、处理次数、失败与最近错误。不同独立账号目录和内核分开；不要用一个网易身份同时登录多个服务器。

## 3. 生命周期与并发

接收线程只排队，每实例队列 256。每实例按顺序执行回调，Python 已有积压最多 16 条批处理，不等凑批。队列满记录丢弃，不能把丢弃事件当执行成功。长等待会拖延本实例其他事件，操作应分批。后台线程自己创建时必须自己结束；停止不能强杀任意 Python 线程或中断任意 C++ 调用。

状态在调用间保留，停止后重新加载会销毁旧模块/对象。Android 进程死亡不保留 RAM，使用 state/save 持久化。重载自动清理 SDK 定时器、包订阅与控制页。定时任务不重新执行整份源码。SDK 不施加原 Lua 的两分钟脚本中断到任意原生插件；不能声称任意死循环都会自动中断。

账号 Android 进程独立；同账号 Python 插件共享 CPython/GIL与公共依赖，每插件入口和相对导入使用独立包前缀。推荐 from .helper import xxx，避免普通 import helper 在 sys.modules 中与其他插件同名冲突。第三方 C 扩展必须与 Android CPython ABI 匹配，桌面 wheel 通常不能直接用。C++ 崩溃会影响本账号进程，插件不是不可信代码安全沙箱。

## 4. 参数、返回值和错误

字符串/数字/布尔/空值、数组与对象可跨语言；不传函数指针、文件对象、Lua userdata。Python 的命名空间方法接受位置参数；回调函数会变成注册 ID。C++ 使用 yy::json，中文 UTF-8；C++ 方法 delete 使用 delete_。Lua 风格的数值通常通过 JSON number 返回，多个返回值为 JSON 数组，可 Python x,y,z=game.getPlayerPosXYZ("@s")。连续序号表变数组，对象表变 dict/object，空表通常为对象；明确列表接口归一化为空列表。XUID 使用字符串，不用浮点保存账号身份；player.getUniqueID 直接返回宿主 64 位整数。

参数错误、账号离线、未知方法、宿主失败都会抛 Python RuntimeError / C++ std::runtime_error。game.sendCommand 是不等待结果的发送；只有成功发送，不能证明执行效果。要求判断成功用 game.execute 或 isCmdSuccess，检查真实 success。服务器未授权 OP、命令不合法、区块未加载、认证失败必须如实处理。

## 5. 权限与游戏操作

连接账号：设置游戏 Token、验证服务、真实服务器号，直到状态显示真正已连接。插件不生成网易身份，也不能绕过服务器权限。聊天/部分玩家缓存读取与管理指令权限不同；give、setblock、结构、记分板等通常需要服务器授予相应命令权限。方块/NBT 操作可能传送机器人加载区块，涉及大区域时分批并观察返回。控制机器人移动/飞行影响机器人实体，不保证改变所有玩家状态。

Bedrock 常用指令 say、tellraw、give、clear、scoreboard、setblock、fill、execute、tp、effect、structure。选择器 @s/@a/@e/@p/@r，参数 r/rm/c 与 Java distance/limit 不同。不要套用 Java /data 或 Java NBT 语法。正式游戏用例：进服授予权限 → 启动示例 → 游戏聊天输入 .插件状态 或 .原生状态 → 查看定向回执、插件计数和日志。真实指令、方块操作已验证；具体真人触发词和每项事件需逐项测试，不把手工事件注入当真人发言。

## 6. 模型和 MCP

AI 帮写使用本语言技能、此手册和项目文件；生成代码后必须检查 API 返回与 ABI。MCP 能读取实际工具和资源，调用 SDK API 的字段为 plugin_id、account、method、args。SDK 实例先启动；MCP 的游戏业务同样需要真正进服。模型、MCP、设备密钥分别配置，不使用游戏 Token 作为模型密钥。

## 7. 开源借鉴

借鉴 ToolDelta 的游戏控制门面、生命周期、明确插件导出及事件回调方式；NTvector 的原生宿主/嵌入 Python 和重载设计作为结构参考，不声称直接兼容其二进制。代码另行实现，本项目保持原源码 AGPL-3.0 归属。C++ JSON 使用 nlohmann/json 3.11.3（MIT，头文件保留许可证）。

- https://github.com/ToolDelta-Basic/ToolDelta/blob/main/tooldelta/plugin_load/classic_plugin/plugin_cls.py
- https://github.com/lurete-cn/NTvector
- https://github.com/nlohmann/json/tree/v3.11.3
- https://learn.microsoft.com/en-us/minecraft/creator/commands/

## 8. Python 可运行示例

模块自动注入 game/player/world/events/util/api 等命名空间，也可显式用 host.game。on_init(host) 加载时调用；on_event(event,host) 是可选通用入口；on_stop(host) 停止时调用。SDK 回调只有其规定参数，不额外加 host。

```python
"""Ready-to-run game controller. All game mutations require a connected bot."""
counter = 0
ticks = 0

def on_init(h):
    global counter
    counter = h.state().get('counter', 0)
    game.setConfigDefault({'trigger': '.插件状态'})
    game.registerTrigger([game.getConfig('trigger', '.插件状态')], on_trigger)
    events.onJoin(on_join)
    util.schedule(2, on_tick)
    web.page('control', 'Python 插件控制', [
        {'type': 'text', 'key': 'counter', 'label': '事件计数'},
        {'type': 'button', 'action': 'status', 'label': '读取真实状态'},
    ])
    web.action('control', 'status', on_web)

def on_trigger(player_name, args):
    global counter
    counter += 1
    host.save({'counter': counter})
    game.sendTargeted(player_name, '§aPython 常驻插件已处理 %d 次请求' % counter)

def on_join(player_name):
    game.sendTargeted(player_name, '§a欢迎进入服务器！')

def on_tick():
    global ticks
    ticks += 1

def on_web(request):
    return {'counter': counter, 'ticks': ticks, 'status': host.status()}

def add(a, b): return a+b
def state(): return {'counter': counter, 'ticks': ticks, 'account': host.account}
def sum_cpp(a,b): return api.require('yy_sdk_cpp_demo').add(a,b)
def execute(command): return game.execute(command)
plugin_api = {'add': add, 'state': state, 'sum_cpp': sum_cpp, 'execute': execute}

def on_event(event,h):
    return {'ok':True, 'counter':counter, 'ticks':ticks}

def on_stop(h): h.save({'counter':counter})
```

安装 SDK Python 控制示例 ZIP。可以先在离线状态查看定时计数，消息和 give 等游戏操作需进服。跨语言 sum_cpp 需要同实例启动 yy_sdk_cpp_demo；目标未启动或忙碌返回错误。

```python
try:
    result = game.execute('give @a stone 1')
    if not result.get('success'):
        print('游戏执行失败:', result.get('output'))
except RuntimeError as error:
    print('连接或宿主调用失败:', error)
```

添加辅助源码 helper.py 后用 from .helper import xxx。pip 纯 Python 依赖可随项目一起导入；APK 没有保证任何第三方包都可安装。上传、文件搜索、编辑、Diff、历史和回滚在项目工作区完成。

## 9. 全部宿主方法参考

## game 命名空间

### `game.dataDir`

调用形式：`host.game.dataDir()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

无参数，返回当前插件/实例的私有持久数据目录，自动创建目录。不要向其他实例目录写入。

### `game.execute`

调用形式：`host.game.execute(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

执行指令并等待真实结果。参数 command 是一条 Bedrock 指令或终端支持的 / 指令链。返回对象包含 success/output，链式执行还包含 results。传输成功不等于 success 为 true，权限拒绝、命令语法错误、超时不能当成功。未连接抛错误。

### `game.executeBatch`

调用形式：`host.game.executeBatch(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

commands 字符串数组，options 对象：mode=ordered/pipeline/submit，window=1..16，timeout_ms=100..10000。ordered 默认严格逐条等待；pipeline 仅用于互不依赖的命令，每窗口按输入顺序发送再汇总回包，不保证服务端完成顺序；submit 不等待结果。结果逐项含 submitted/confirmed/success/error，超时未知且不自动重发。

### `game.flyStart`

调用形式：`host.game.flyStart()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：开始 / 停止飞行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `flyStart/flyStop: <err>`。
- 底层：`abot(L).StartFlying()/StopFlying()`（fly.go）。

### `game.flyStop`

调用形式：`host.game.flyStop()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：开始 / 停止飞行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `flyStart/flyStop: <err>`。
- 底层：`abot(L).StartFlying()/StopFlying()`（fly.go）。

### `game.getBlockData`

调用形式：`host.game.getBlockData(x, y, z)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读某坐标方块的完整数据（名称 + 方块状态 + 方块实体 NBT）。读取命令方块指令、箱子物品、告示牌文字、旗帜图案等都从这里入手。会先把机器人传送过去加载区块（有移动副作用）。
- 参数：`x`/`y`/`z` number 必填。
- 返回：JSON 表结构 `{name = 方块名, states = {状态键 = 值, ...}, nbt = {NBT 键 = 值, ...}}`（无 NBT 时 `nbt` 省略）。
- 报错：失败抛 `getBlockData: <err>`。
- 底层：调 `abot(L).GetBlockData(x, y, z)`。

### `game.getBlockNBT`

调用形式：`host.game.getBlockNBT(x, y, z)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：只读方块实体 NBT。命令方块指令在 `nbt.Command`，箱子物品在 `nbt.Items`，告示牌文字在 `nbt.FrontText.Text`。
- 参数：`x`/`y`/`z` number 必填。
- 返回：JSON 表结构，无 NBT 时返回 `nil`。
- 报错：失败抛 `getBlockNBT: <err>`。
- 底层：调 `abot(L).GetBlockNBT(x, y, z)`。

### `game.getBlockState`

调用形式：`host.game.getBlockState(x, y, z)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读方块名称与方块状态（朝向/是否点亮/开关等），不含 NBT。
- 参数：`x`/`y`/`z` number 必填。
- 返回：JSON 表结构 `{name = 方块名, states = {...}}`。
- 报错：失败抛 `getBlockState: <err>`。
- 底层：调 `abot(L).GetBlockState(x, y, z)`。

### `game.getBlockTile`

调用形式：`host.game.getBlockTile(x, y, z)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读某坐标的方块名。用安全版：可能先把机器人传送过去加载区块再查，避免远处区块"未加载"报错，因此有移动副作用。
- 参数：`x`/`y`/`z` number 必填。
- 返回：string（方块名/方块 ID）。
- 报错：失败抛 `getBlockTile: <err>`。
- 底层：调 `abot(L).GetBlockTileSafe(x, y, z)`。

### `game.getBlockTileRaw`

调用形式：`host.game.getBlockTileRaw(x, y, z)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：直接查询方块，不做传送加载（更快，但区块未加载可能报错）。
- 参数：`x`/`y`/`z` number 必填。
- 返回：string（方块名）。
- 报错：失败抛 `getBlockTileRaw: <err>`。
- 底层：调 `abot(L).GetBlockTile(x, y, z)`。

### `game.getBotPos`

调用形式：`host.game.getBotPos()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读取当前活跃机器人自身坐标。
- 参数：无。
- 返回：JSON 表结构 `{x, y, z}`。
- 报错：无。
- 底层：`abot(L).GetPosition()`。

---

### `game.getConfig`

调用形式：`host.game.getConfig(key, default)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 key, default=None；返回插件当前实例数据目录内 config.json 的键，缺失用 default。配置按插件和实例分开。

### `game.getMultiScore`

调用形式：`host.game.getMultiScore(target)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读某目标在所有记分板上的分数。
- 参数：`target` string 必填。
- 返回：JSON 表结构，结构 `{player = {记分板名 = 分数, ...}}`。
- 报错：失败抛 `getMultiScore: <err>`。
- 底层：调 `abot(L).GetMultiScore(target)`，经 `multiScoreToTable` 转换。

### `game.getPlayerItem`

调用形式：`host.game.getPlayerItem(target, item, specialID)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：统计玩家背包里某种物品的数量，可带特殊数据 ID。
- 参数：`target` string、`item` string 必填；`specialID` number 可选，默认 0。
- 返回：number（数量）。
- 报错：失败抛 `getPlayerItem: <err>`。
- 底层：调 `abot(L).GetPlayerItem(target, item, special)`。

### `game.getPlayerPos`

调用形式：`host.game.getPlayerPos(target)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读玩家坐标与所在维度。
- 参数：`target` string 必填（可为玩家名或 `@a`/`@p` 选择器）。
- 返回：JSON 表结构 `{x, y, z, dimension}`。
- 报错：失败抛 `getPlayerPos: <err>`。
- 底层：调 `abot(L).GetPlayerPos(target)`。

### `game.getPlayerPosXYZ`

调用形式：`host.game.getPlayerPosXYZ(target)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读玩家坐标，直接返回三个数字。
- 参数：`target` string 必填。
- 返回：三个 number，依次 x、y、z。
- 报错：失败抛 `getPlayerPosXYZ: <err>`。
- 底层：调 `abot(L).GetPlayerPosXYZ(target)`。

### `game.getRegionBlockData`

调用形式：`host.game.getRegionBlockData(x, y, z, sx, sy, sz)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读一块区域（起点 `x,y,z`，尺寸 `sx×sy×sz`）所有方块的名称+状态+NBT，一次结构保存返回，适合整片区域/区块探查。
- 参数：6 个 number 均必填，尺寸需为正。
- 返回：JSON 表结构，键为 `"x,y,z"` 绝对坐标，值为 `{name, states, nbt}`。
- 报错：失败抛 `getRegionBlockData: <err>`。
- 底层：调 `abot(L).GetRegionBlockData(...)`。

### `game.getScore`

调用形式：`host.game.getScore(scoreboard, target)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读某记分板里某个目标（玩家名）的分数。
- 参数：`scoreboard` string 必填；`target` string 必填。
- 返回：number（分数）。
- 报错：失败抛 `getScore: <err>`。
- 底层：调 `abot(L).GetScore(scb, target)`。

### `game.getTarget`

调用形式：`host.game.getTarget(selector)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：解析选择器，返回匹配的玩家名数组。
- 参数：`selector` string 必填。
- 返回：JSON 表结构（玩家名字符串数组，1 起索引）。
- 报错：失败抛 `getTarget: <err>`。
- 底层：调 `abot(L).GetTarget(sel)`。

### `game.getTickingAreaList`

调用形式：`host.game.getTickingAreaList()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读当前加载的常驻区域（ticking area）列表。
- 参数：无。
- 返回：JSON 表结构 `{区域名 = {dimension, start_x, start_z, end_x, end_z}}`。
- 报错：失败抛 `getTickingAreaList: <err>`。
- 底层：调 `abot(L).GetTickingAreaList()`。

### `game.httpGet`

调用形式：`host.game.httpGet(url)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：发起 HTTP GET 请求，返回响应体文本。
- 参数：`url` string 必填。
- 返回：string（响应体）。
- 报错：失败抛 `httpGet: <err>`。
- 底层：`httpGetText(url)`。

### `game.httpPost`

调用形式：`host.game.httpPost(url, body, headers)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：发起 HTTP POST 请求，可带自定义请求头。
- 参数：`url` string 必填；`body` string 可选，默认 `""`；`headers` JSON 表结构（键值均为 string）可选。
- 返回：string（响应体）。
- 报错：失败抛 `httpPost: <err>`。
- 底层：`httpPostText(url, body, headers)`。

### `game.isCmdSuccess`

调用形式：`host.game.isCmdSuccess(cmd)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：执行一条指令并判断是否成功，返回布尔。
- 参数：`cmd` string 必填。
- 返回：boolean（是否成功）。
- 报错：失败抛 `isCmdSuccess: <err>`。
- 底层：调 `abot(L).IsCmdSuccess(cmd)`。

### `game.jsonDecode`

调用形式：`host.game.jsonDecode(s)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把 JSON 字符串解析成 Lua 值。
- 参数：`s` string 必填。
- 返回：任意 Lua 值（表/数字/字符串/布尔）。
- 报错：解析失败抛 `jsonDecode: <err>`。
- 底层：`jsonDecodeGo(s)` 后 `goToLua`。

> 这几个是挂在 `game` 上的轻量便捷方法；需要完整请求控制（超时、多种方法、错误码）时用 4.12 节的 `http` 模块。

### `game.jsonEncode`

调用形式：`host.game.jsonEncode(v)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把任意 Lua 值编码成 JSON 字符串。
- 参数：`v` 任意类型。
- 返回：string（JSON）。
- 报错：序列化失败抛 `jsonEncode: <err>`。
- 底层：`jsonEncodeGo(luaToGo(L.Get(1)))`。

### `game.jump`

调用形式：`host.game.jump()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：让机器人跳一下。
- 参数：无。
- 返回：无。
- 报错：失败抛 `jump: <err>`。
- 底层：`abot(L).Jump()`。

### `game.move`

调用形式：`host.game.move(dx, dy, dz)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：让机器人相对当前位置移动。
- 参数：`dx`/`dy`/`dz` number 必填。
- 返回：无。
- 报错：失败抛 `move: <err>`。
- 底层：`abot(L).MoveRelative(dx, dy, dz)`。

### `game.placeNBTBlock`

调用形式：`host.game.placeNBTBlock(x, y, z, name, states, nbtTable)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：放置一个带 NBT 的特殊方块（命令方块/结构方块/告示牌/箱子/旗帜等），自动按方块类型选择最快可靠的放置方式：
  - 命令方块：`{Command = "say hi", Conditional = 1, TickDelay = 0}`；
  - 告示牌：`{FrontText = {Text = "你好", Color = "black"}, ...}`；
  - 箱子：`{Items = {{Count = 1, Name = "minecraft:diamond", Slot = 0}, ...}}`。
- 参数：`name` string 必填；`states` string 可选；`nbtTable` JSON 表结构 可选。
- 返回：无。
- 报错：失败抛 `placeNBTBlock: <err>`。
- 底层：调 `abot(L).PlaceNBTBlock(...)`，内部按 `building.ClassifyBlock` 分发（命令/结构→直接发包，告示牌→`PlaceSign`，重NBT箱子/旗帜→工作台+结构复制）。

### `game.promptChoice`

调用形式：`host.game.promptChoice(player, title, options, timeoutSec)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：显示菜单并**阻塞等待玩家选择**，返回所选选项下标。
- 参数：`player`/`title` string、`options` JSON 表结构 必填；`timeoutSec` number 可选，默认 30。
- 返回：number 或 nil。成功返回 **1 起** 下标（第一个是 1）；超时/取消/无效输入返回 `nil`。
- 报错：失败抛 `promptChoice: <err>`。
- 底层：构造 `ChatMenu`，调 `abot(L).PromptChoice(player, menu, timeout)`。

### `game.queryInventory`

调用形式：`host.game.queryInventory(selector)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：查询玩家背包物品清单。
- 参数：`selector` string 必填。
- 返回：JSON 表结构 `{items = {{name, count, slot, damage}, ...}}`。
- 报错：失败抛 `queryInventory: <err>`。
- 底层：调 `abot(L).QueryPlayerInventory(sel)`。

### `game.registerTrigger`

调用形式：`host.game.registerTrigger(wordsTable, handlerName)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 words 为非空字符串数组，handler 为回调。只匹配完整触发词或触发词加空格边界，回调参数为 (player_name, args_list)。Python 可传函数对象或模块函数名；C++ 传已用 on() 注册的名称。匹配后进入该插件的串行队列，不重载插件。

### `game.rotate`

调用形式：`host.game.rotate(pitch, yaw, seconds?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：设置机器人朝向。
- 参数：`pitch`/`yaw` number 必填；`seconds` number 可选——省略或 `0` 时**立即转到**目标角度，`>0` 时在 seconds 秒内**平滑转动**过去（yaw 走最短路径，如从 350° 转到 10° 只转 20°）。
- 返回：无。
- 报错：失败抛 `rotate: <err>`。
- 底层：`abot(L).SetRotation(pitch, yaw)`（秒数>0 走 `SmoothRotateTo(pitch,yaw,seconds)`）。

### `game.scene`

调用形式：`host.game.scene(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

无参数。当前账号的机器人和已观测实体快照；含 known、stale、数据时间和来源。AI 没有独立游戏实体，bot 是当前账号控制的机器人。

### `game.sendActionbar`

调用形式：`host.game.sendActionbar(target, text)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：给玩家显示动作栏消息（物品栏上方那行）。
- 参数：`target`/`text` string 必填。
- 返回：无。
- 报错：失败抛 `sendActionbar: <err>`。
- 底层：调 `abot(L).SendActionbar(target, text)`。

### `game.sendChat`

调用形式：`host.game.sendChat(target, message)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：给指定玩家或选择器（如 `@a`）发一条聊天消息。
- 参数：`target` string 必填；`message` string 必填。
- 返回：无。
- 报错：失败抛 `sendChat: <err>`。
- 底层：调 `abot(L).SendChat(target, msg)`。

### `game.sendCommand`

调用形式：`host.game.sendCommand(cmd)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：发送一条游戏指令（等价于游戏内执行 `/cmd`），只发不等待结果。
- 参数：`cmd` — string，必填。
- 返回：无。
- 报错：若 sendCommandHook 返回错误则抛 `sendCommand: <err>`。
- 底层：调 `abot(L).SendCommandNoWait(ChannelAI, cmd)`。

### `game.sendLines`

调用形式：`host.game.sendLines(player, text, maxLines)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把长文本**按换行逐条发送**，避免单条超长被截断、或某行含敏感词整段被拦。超行数时先提示"仅显示前 N 行"。空行跳过。
- 参数：`player` string 必填；`text` string 必填；`maxLines` number 可选，默认 50。
- 返回：无。
- 报错：无。
- 底层：`strings.Split(text,"\n")` 逐行调 `abot(L).sendTargeted(...)`。

### `game.sendRichTellraw`

调用形式：`host.game.sendRichTellraw(msg)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：发送原生 tellraw 富文本（支持 JSON 样式），常用于公告。
- 参数：`msg` string 必填。
- 返回：无。
- 报错：失败抛 `sendRichTellraw: <err>`。
- 底层：调 `abot(L).SendRichTellraw(msg)`。

### `game.sendTargeted`

调用形式：`host.game.sendTargeted(player, text)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：只给指定玩家发一条消息，不广播给其他人。`§` 前缀可带颜色（如 `"§a成功！"`）。
- 参数：`player` string 必填；`text` string 必填。
- 返回：无。
- 报错：无（底层错误被忽略）。
- 底层：调 `abot(L).sendTargeted(player, text)`。

### `game.sendTitle`

调用形式：`host.game.sendTitle(target, title, subtitle)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：给玩家显示大标题 + 副标题。
- 参数：`target`/`title`/`subtitle` 均 string 必填。
- 返回：无。
- 报错：失败抛 `sendTitle: <err>`。
- 底层：调 `abot(L).SendTitle(target, title, sub)`。

### `game.setBlock`

调用形式：`host.game.setBlock(x, y, z, name, states)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：放置一个普通方块，可带方块状态字符串。
- 参数：`name` string 必填；`states` string 可选（如 `["facing_direction"=3]`，默认空）。
- 返回：无。
- 报错：失败抛 `setBlock: <err>`。
- 底层：调 `abot(L).SetBlock(x, y, z, name, states)`，走网易 AI 命令通道 setblock。

### `game.setBlockState`

调用形式：`host.game.setBlockState(x, y, z, statesTable)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：修改某方块的方块状态并保留原 NBT（如把命令方块设为条件模式、把方块朝向改朝北）。会先读当前方块，合并新状态后重新放置。
- 参数：`statesTable` JSON 表结构 必填，键 = 状态名，值 = 数值/布尔（如 `{conditional_bit = 1, facing_direction = 3}`）。
- 返回：无。
- 报错：失败抛 `setBlockState: <err>`。
- 底层：调 `abot(L).SetBlockState(x, y, z, states)`。

> **注意**：箱子/旗帜这类"重 NBT 方块"写入在网易下走远程工作台 + 结构复制，较慢且需要结构权限；命令方块/告示牌直接发包，快。读取和写入都会传送机器人加载区块，属正常副作用。

### `game.setConfigDefault`

调用形式：`host.game.setConfigDefault(table)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 defaults 是 JSON 对象，只补缺失键，不覆盖已有设置。写入当前插件/实例的 config.json。

### `game.setEffect`

调用形式：`host.game.setEffect(player, effect, duration, level, particles)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：给玩家施加药水效果。
- 参数：`player` string、`effect` string、`duration` number、`level` number 必填；`particles` boolean 可选，默认 `true`。
- 返回：无。
- 报错：失败抛 `setEffect: <err>`。
- 底层：调 `abot(L).SetPlayerEffect(player, effect, dur, lvl, particles)`。

### `game.setScore`

调用形式：`host.game.setScore(board, target, score)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：写入记分板，执行 `scoreboard players set`。
- 参数：`board` string、`target` string、`score` number 均必填。
- 返回：无。
- 报错：失败抛 `setScore: <err>`。
- 底层：拼指令 `scoreboard players set <target> <board> <score>`，调 `abot(L).SendCommandNoWait(ChannelAI, cmd)`。

### `game.showMenu`

调用形式：`host.game.showMenu(player, title, options)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：给玩家显示一个聊天栏菜单（标题 + 若干选项），只展示不等待。
- 参数：`player` string、`title` string 必填；`options` JSON 表结构（字符串数组）必填。
- 返回：无。
- 报错：无。
- 底层：构造 `ChatMenu`，调 `abot(L).ShowMenu(player, menu)`。

### `game.sneakStart`

调用形式：`host.game.sneakStart()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：开始 / 停止潜行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sneakStart/sneakStop: <err>`。
- 底层：`abot(L).SneakStart()/SneakStop()`。

### `game.sneakStop`

调用形式：`host.game.sneakStop()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：开始 / 停止潜行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sneakStart/sneakStop: <err>`。
- 底层：`abot(L).SneakStart()/SneakStop()`。

### `game.sprintStart`

调用形式：`host.game.sprintStart()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：开始 / 停止疾跑。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sprintStart/sprintStop: <err>`。
- 底层：`abot(L).SprintStart()/SprintStop()`。

### `game.sprintStop`

调用形式：`host.game.sprintStop()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：开始 / 停止疾跑。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sprintStart/sprintStop: <err>`。
- 底层：`abot(L).SprintStart()/SprintStop()`。

### `game.stopMove`

调用形式：`host.game.stopMove()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：停止机器人当前移动。
- 参数：无。
- 返回：无。
- 报错：失败抛 `stopMove: <err>`。
- 底层：`abot(L).StopMovement()`。

### `game.teleport`

调用形式：`host.game.teleport(x, y, z)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：传送机器人到绝对坐标。
- 参数：`x`/`y`/`z` number 必填。
- 返回：无。
- 报错：失败抛 `teleport: <err>`。
- 底层：`abot(L).TeleportTo(x, y, z)`。

### `game.waitMsg`

调用形式：`host.game.waitMsg(player, timeoutSec)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：**阻塞等待玩家在聊天栏发一条消息**，返回内容。用于要输入（领地、数量等）。
- 参数：`player` string 必填；`timeoutSec` number 可选，默认 30。
- 返回：string（收到的消息）。超时返回 `nil`。
- 报错：失败抛 `waitMsg: <err>`。
- 底层：调 `abot(L).WaitMsg(player, timeout)`。注意它"独占"该玩家的消息——同一玩家有活跃会话时消息被它消费。

## api 命名空间

### `api.call`

调用形式：`host.api.call(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 plugin_id, function_name, args_array。只调用目标插件明确导出的接口。Python 导出 plugin_api 字典；C++ 用 expose()；Lua 用 plugin_api。返回目标函数 JSON 结果。目标忙碌、自己调用自己、循环调用、目标未运行或未导出均报错，避免互相等待死锁。

### `api.require`

调用形式：`host.api.require(id)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

Python：api.require("目标插件").函数(参数)。C++：client.api.require("目标插件").call("函数", json::array({...}))。代理本身不代表目标已存在，实际调用时验证目标。使用同一游戏内核/实例标识，跨独立进程账号不隐式共享实例。

## plugin 命名空间

### `plugin.dependencies`

调用形式：`host.plugin.dependencies()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

无参数，返回 manifest.dependencies 声明的前置插件与最低版本；缺省为空对象。启动前检查同实例的依赖已经启动，最低版本用数字版本如 1.0.0；空串或 * 不限制版本。前置未启动或版本不足拒绝启动，不自动安装 pip 包/原生库，也不跨账号共享依赖。

### `plugin.id`

调用形式：`host.plugin.id()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

无参数，返回当前 manifest.json 的插件 ID。它不是游戏账号 ID；游戏实例标识见 host.account / client.account()。

### `plugin.save`

调用形式：`host.plugin.save(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 value 是可序列化 JSON 值，原子替换当前实例 state.json。Python 也可用 host.save(dict)。全局变量保留到实例停止，跨进程恢复依赖主动保存。

### `plugin.state`

调用形式：`host.plugin.state(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

无参数，读当前实例 state.json，缺失返回空对象。Python 可用 host.state()。

## bot 命名空间

### `bot.count`

调用形式：`host.bot.count()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：返回在线机器人数量。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：`GetFleetBots()`。

### `bot.current`

调用形式：`host.bot.current()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：返回当前活跃机器人的名字与坐标。
- 参数：无。
- 返回：JSON 表结构 `{name, x, y, z}`；无活跃机器人返回空表；坐标查询失败省略 x/y/z。
- 报错：无。
- 底层：`abot(L)` + `botName` + `cur.QueryPosition()`。

### `bot.list`

调用形式：`host.bot.list()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：列出在线机器人及坐标。
- 参数：无。
- 返回：JSON 表结构，1 起索引，每个元素 `{name, x, y, z}`（坐标查询失败省略坐标字段）。
- 报错：无。
- 底层：`GetFleetBots()` 遍历 + `QueryPosition()`。

### `bot.lookAt`

调用形式：`host.bot.lookAt(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

x,y,z 世界坐标。由已观测机器人位置计算 yaw/pitch，启用心跳发送视角；返回请求状态，无自动寻路/穿墙保证。

### `bot.select`

调用形式：`host.bot.select(nameOrIndex)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：切换后续 `game.*` / `world.*` 调用的目标机器人。
- 参数：`nameOrIndex` string 必填（机器人名或索引的字符串形式）。
- 返回：boolean `true`。
- 报错：未找到机器人抛 `bot.select: 未找到机器人 <名称>`。
- 底层：`selectBot(L, sel)`。

### `bot.self`

调用形式：`host.bot.self(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

无参数。读取当前账号机器人的服务器观测状态，health/position/dead 可未知。AI Agent 使用此机器人，不自行拥有一个游戏位置。

### `bot.startMovement`

调用形式：`host.bot.startMovement(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

dx,dy,dz 玩家输入方向，每轴 -1..1；以 50ms PlayerAuthInput 心跳交给服务器驱动运动。dx 左右，dz=-1 前进，dy 上下，不是世界坐标路径规划；返回 submitted=true,confirmed=false。

### `bot.status`

调用形式：`host.bot.status()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：返回每个机器人的连接状态、是否 OP、错误信息。
- 参数：无。
- 返回：JSON 表结构，1 起索引，每个元素 `{index, name, connected, is_op, error?}`（error 仅非空时存在）。
- 报错：无。
- 底层：`FleetStatus()`。

---

### `bot.stopMovement`

调用形式：`host.bot.stopMovement(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

无参数。清理当前机器人移动输入，仍需要服务器回包确认最终位置。

### `bot.teleport`

调用形式：`host.bot.teleport(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

x,y,z 世界坐标。执行服务器 tp @s 并等待真实命令结果，需要对应权限；没有权限时报错或 success=false。

## building 命名空间

### `building.closestColor`

调用形式：`host.building.closestColor(r, g, b)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：按 CIELAB 感知色差，匹配给定 RGB 最接近的颜色方块。
- 参数：`r`/`g`/`b` number 必填（0-255）。
- 返回：string（最接近的颜色方块名）。
- 报错：非数值抛类型错误。
- 底层：`media.ClosestBlockForColor(r, g, b)`。

---

### `building.detect`

调用形式：`host.building.detect(path)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：检测建筑文件的格式。
- 参数：`path` string 必填。
- 返回：string（格式名：`.mcstructure` / `.schematic` / `.mcworld` / `.bdx`）。
- 报错：非字符串抛类型错误。
- 底层：`building.DetectFormat(path)`。

### `building.isAir`

调用形式：`host.building.isAir(name)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isCommandBlock`

调用形式：`host.building.isCommandBlock(name)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isLava`

调用形式：`host.building.isLava(name)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isWater`

调用形式：`host.building.isWater(name)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.parse`

调用形式：`host.building.parse(path)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：加载并解析建筑结构文件，返回尺寸、方块统计、NBT、是否含命令方块。
- 参数：`path` string 必填。
- 返回：JSON 表结构 `{size={x,y,z}, blocks=number, nbt=number, has_command=boolean}`（`blocks` 为非空气方块数，`nbt` 为 NBT 方块数）。
- 报错：加载失败抛 `building.parse: <err>`。
- 底层：`building.LoadStructureFile(path)`，遍历方块计数，读 `NBTBlocks`/`HasCommand`。

## events 命名空间

### `events.captureClear`

调用形式：`host.events.captureClear(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

清空当前账号抓包缓冲，保持抓包开关。

### `events.captureRead`

调用形式：`host.events.captureRead(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

after_sequence=0,limit=256，返回记录/游标/覆盖和抽样统计。缓冲有界，单条超大小截为预览；不是手机所有 App 的网络抓包。

### `events.captureStart`

调用形式：`host.events.captureStart(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

config={types:[包名或*],exclude:[包名],filter:{fields:{...},every_ms:...},capacity:1..2048,max_bytes:512..65536}。开启账号级解码入站游戏抓包；默认血量/死亡/位置/方块包，认证握手不记录，* 默认不包含大区块包。

### `events.captureStop`

调用形式：`host.events.captureStop(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

停止记录，保留已抓数据。当前账号的多个插件共用账号级记录；使用时协调抓包开关。

### `events.onBlockChange`

调用形式：`host.events.onBlockChange(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler(event)，UpdateBlock 导出 position/runtime_id/previous/cause_known=false；不能仅从方块更新认定破坏者。

### `events.onBotReady`

调用形式：`host.events.onBotReady(handlerName)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler 无参数；只有真正游戏连接完成才触发，创建认证客户端不算就绪。

### `events.onChat`

调用形式：`host.events.onChat(handlerName)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler 回调参数 (player_name, message)，消息来自真实游戏事件。

### `events.onDeath`

调用形式：`host.events.onDeath(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler 回调接收一个事件对象，来源为 ActorEventDeath/InstantDeath 或 health<=0；同一已死亡状态去重，重生后可再触发。不推测死亡原因/攻击者。

### `events.onHealthChange`

调用形式：`host.events.onHealthChange(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler(event)，仅在观测血量初始值或变化时入队，含 previous/health/max_health/time/source。

### `events.onHurt`

调用形式：`host.events.onHurt(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler(event)，来源为血量下降或 ActorEventHurt，两种独立来源可能各触发一次；无法识别准确攻击者，不用编造 cause。

### `events.onJoin`

调用形式：`host.events.onJoin(handlerName)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler 回调参数 (player_name)。注册后开启玩家轮询；租赁服进出服事件来源是约 5 秒轮询差量，不保证瞬时到达。

### `events.onLeave`

调用形式：`host.events.onLeave(handlerName)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler 回调参数 (player_name)。注册后开启玩家轮询，首次快照只记录集合，不伪造加入事件。

### `events.onMove`

调用形式：`host.events.onMove(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler(event)，服务器实际位置初始值/变化时触发；含 previous_known/position/dimension/source。高频事件可用抓包抽样过滤减少负担。

### `events.onPacket`

调用形式：`host.events.onPacket(handlerName)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler 回调参数 (packet_type, data)。这是内核显式转发的通用事件，不保证所有协议包都对应此事件；全协议指定订阅请用 events.subscribe。

### `events.onRespawn`

调用形式：`host.events.onRespawn(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

handler(event)，只处理服务端 RespawnStateReadyToSpawn，清除旧血量等待新属性。

### `events.packetSchema`

调用形式：`host.events.packetSchema(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

无参数。返回当前内核支持包类型、协议 ID、顶层字段名与类型，供 AI 根据实际协议找包而非猜字段。

### `events.subscribe`

调用形式：`host.events.subscribe(packetType, handlerName)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 packet_type, handler；指定协议包名称，Python handler(data_dict)，C++ 回调 args[0] 为数据。订阅按当前账号过滤，停止后退订；不支持通过回调返回值拦截底层包。协议 ID/名称请按本机捕获结果使用。

### `events.subscribeFiltered`

调用形式：`host.events.subscribeFiltered(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

packet_type 或 *，filter={fields:{字段路径:精确值},every_ms:0..60000}，handler。Python 回调接收包对象；C++/Lua 使用已注册函数名。* 接收当前账号订阅包，事件字段 _packet_type 表示真实类型，停止自动退订。

## http 命名空间

### `http.delete`

调用形式：`host.http.delete(url, opts?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.get`

调用形式：`host.http.get(url, opts?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.json_decode`

调用形式：`host.http.json_decode(s)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把 JSON 字符串解析为 Lua 值。
- 参数：`s` string 必填。
- 返回：任意 Lua 值。
- 报错：解析失败抛 `http.json_decode: <err>`。
- 底层：`jsonDecodeGo(s)` → `goToLua`。

### `http.json_encode`

调用形式：`host.http.json_encode(v)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把 Lua 值转成 JSON 字符串。
- 参数：`v` 任意 Lua 值。
- 返回：string。
- 报错：序列化失败抛 `http.json_encode: <err>`。
- 底层：`jsonEncodeGo(luaToGo(...))`。

### `http.post`

调用形式：`host.http.post(url, body?, opts?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.put`

调用形式：`host.http.put(url, body?, opts?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.querify`

调用形式：`host.http.querify(paramsTable)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把参数表编码成 URL query 字符串。
- 参数：`paramsTable` JSON 表结构 必填（键值都转字符串）。
- 返回：string，如 `"a=1&b=2"`（按键排序，同键累加）。
- 报错：非 JSON 表结构 抛类型错误。
- 底层：`url.Values` + `vals.Encode()`。

> 与 `game.httpGet/httpPost` 的区别：`game.*` 返回纯文本、非 200 直接抛错，适合简单场景；`http.*` 返回完整响应对象、携带状态码与响应头、非 2xx 也能拿 body，适合需要细粒度控制的场景。

---

### `http.request`

调用形式：`host.http.request(method, url, body?, opts?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：发送任意 method 的 HTTP 请求，返回完整响应对象。
- 参数：`method` string 必填；`url` string 必填；`body` string 可选，默认 `""`；`opts` JSON 表结构 可选（含 `headers` 表、`timeout` 秒）。
- 返回：JSON 表结构。成功 `{ok=true, status=number, body=string, headers=JSON 表结构}`；传输错误 `{ok=false, error=string}`。
- 报错：无（错误放进 `error` 字段，不抛异常）。
- 底层：`httpRequestFull(method, u, body, headers, timeout)`。

## media 命名空间

### `media.imageToAscii`

调用形式：`host.media.imageToAscii(path, maxW?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把图片文件转成 ASCII 字符画行数组。
- 参数：`path` string 必填；`maxW` number 可选，默认 60。
- 返回：JSON 表结构（字符串数组，1 起索引，每行一个 ASCII 串）。
- 报错：打开/解码/转换失败抛 `media.imageToAscii: <err>`。
- 底层：`os.Open` + `image.Decode`（支持 gif/jpeg/png）→ `media.ImageToASCII(img, maxW)`。

### `media.noteToPitch`

调用形式：`host.media.noteToPitch(note)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把 MIDI 音符号换算为频率（Hz）。
- 参数：`note` number 必填（强转 `uint8`）。
- 返回：number（音高频率）。
- 报错：非数值抛类型错误。
- 底层：`media.NoteToPitch(uint8(note))`。

### `media.parseMidi`

调用形式：`host.media.parseMidi(path)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：解析 MIDI 文件，返回格式/轨道/分辨率/速度及音符列表。
- 参数：`path` string 必填。
- 返回：JSON 表结构 `{format, tracks, division, tempo, notes={{tick, channel, note, velocity, duration, sound}}}`（`sound` 仅非空时出现）。
- 报错：读文件/解析失败抛 `media.parseMidi: <err>`。
- 底层：`os.ReadFile` → `media.ParseMIDIFile(data)`。

---

## player 命名空间

### `player.count`

调用形式：`host.player.count()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：返回在线玩家数量。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：`playerInfoMgr.GetAllPlayers`。

### `player.get`

调用形式：`host.player.get(name)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：按名字查玩家信息。
- 参数：`name` string 必填。
- 返回：JSON 表结构 `{name, xuid, unique_id, is_op, online, dimension, x, y, z, held_item?}`（`held_item` 仅非空时出现，为网络 ID）。玩家不存在返回**空表**（不是 nil）。
- 报错：`name` 非 string 抛类型错误。
- 底层：`playerInfoMgr.GetPlayer` + `playerInfoToTable`。

### `player.getInventory`

调用形式：`host.player.getInventory(name)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：走当前活跃机器人查询某玩家背包物品清单。
- 参数：`name` string 必填。
- 返回：JSON 表结构 `{items = {{name, count, slot, damage}, ...}}`。
- 报错：失败抛 `player.getInventory: <err>`。
- 底层：`abot(L).QueryPlayerInventory(sel)`。

---

### `player.getUniqueID`

调用形式：`host.player.getUniqueID(name)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：查玩家实体唯一 ID。
- 参数：`name` string 必填。
- 返回：number；玩家不存在返回 **0**。
- 报错：`name` 非 string 抛类型错误。
- 底层：`playerInfoMgr.GetPlayer`，取 `EntityID`。

### `player.getXUID`

调用形式：`host.player.getXUID(name)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：查玩家 XUID。
- 参数：`name` string 必填。
- 返回：string；玩家不存在返回**空串**。
- 报错：`name` 非 string 抛类型错误。
- 底层：`playerInfoMgr.GetPlayer`。

### `player.health`

调用形式：`host.player.health(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

target 为 self、玩家名或 runtime_id 字符串，默认 self。返回已观测信息含 health_known/health/max_health/health_time/stale，未知为 null，不推算其他玩家满血。仅能获取服务器给本机器人同步的属性。

### `player.isOP`

调用形式：`host.player.isOP(name)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：判断玩家是否 OP。
- 参数：`name` string 必填。
- 返回：boolean（玩家存在且为 OP 才为 true，含不存在时返回 false）。
- 报错：`name` 非 string 抛类型错误。
- 底层：`playerInfoMgr.GetPlayer`，取 `IsOP`。

### `player.list`

调用形式：`host.player.list()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：列出全部在线玩家信息表。
- 参数：无。
- 返回：JSON 表结构（1 起索引，每个元素是 `player.get` 同款信息表）。
- 报错：无。
- 底层：`playerInfoMgr.GetAllPlayers` + `playerInfoToTable`。

### `player.nearby`

调用形式：`host.player.nearby(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

radius=0..512，默认 32；按机器人已观测位置计算同维度、仍存在的玩家距离并排序。不包括客户端不可见或未知坐标的玩家，机器人坐标未知报错。

### `player.position`

调用形式：`host.player.position(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

target 同 player.health。返回 position_known/position/position_time/dimension/stale；坐标为收到服务器包的观测，不把本地移动请求当服务器确认。

## system 命名空间

### `system.call`

调用形式：`host.system.call(method, paramsTable?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：通用底层透传——把 `method` 和参数表交给 Java 分发执行，返回任意结果。已支持的方法：`shell`、`shizukuStatus`、`shizukuRequest`、`vibrate`、`notify`、`launchApp`；未知方法返回 `{"ok":false,"error":"unknown method: ..."}`。
- 参数：`method` string 必填；`paramsTable` JSON 表结构 可选。
- 返回：任意 Lua 值（Java 返回 JSON 解码结果）；无返回推 `nil`。
- 报错：参数序列化失败或桥调用失败抛 `system.call: <err>`。
- 底层：参数 `json.Marshal` → `callAndroidSystem` → JNI → Java `FloatingBridge.call` switch 分发。

### `system.notify`

调用形式：`host.system.notify(title, body?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：发系统状态栏通知（**不需要** Shizuku/ADB 权限）。
- 参数：`title` string 必填；`body` string 可选，默认 `""`。
- 返回：JSON 表结构 `{ok=true}`；异常时 `{ok=false, error=...}`。
- 报错：桥调用失败抛 `system.notify: <err>`。
- 底层：参数 `{"title","body"}` → JNI → Java `postDirectNotification`。

---

### `system.platform`

调用形式：`host.system.platform()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：返回当前运行平台。
- 参数：无。
- 返回：string——`"android"` 或 `"desktop"`。
- 报错：无。
- 底层：`getPlatformName()`（不经 JNI 桥）。

### `system.shell`

调用形式：`host.system.shell(cmd)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：以 Shizuku(ADB) 身份执行一条 shell 命令。
- 参数：`cmd` string 必填。
- 返回：JSON 表结构 `{ok=bool, exit=number, stdout=string, stderr=string, uid=number, error?=string}`。命令本身失败不抛异常，走 `ok=false`。
- 报错：仅桥调用失败抛 `system.shell: <err>`。
- 底层：参数 `{"command":cmd}` → JNI → Java `runShell`：空命令/Shizuku 未运行/未授权各有明确错误；否则经反射调 `Shizuku.newProcess` 执行 `/system/bin/sh -c <cmd>`，读 stdout/stderr、取退出码与 uid。

### `system.shizukuRequest`

调用形式：`host.system.shizukuRequest()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：弹出系统授权对话框，申请 Shizuku ADB 权限。
- 参数：无。
- 返回：JSON 表结构 `{ok=bool, requested=bool, error?=string}`。
- 报错：桥调用失败抛 `system.shizukuRequest: <err>`。
- 底层：JNI → Java `requestShizukuPermission`：pingBinder 失败返回未运行；否则 `Shizuku.requestPermission()` 弹授权框。

### `system.shizukuStatus`

调用形式：`host.system.shizukuStatus()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：查询 Shizuku 运行状态与授权情况。
- 参数：无。
- 返回：JSON 表结构 `{available=bool, granted=bool, uid=number}`（失败时 uid=-1）。
- 报错：桥调用失败抛 `system.shizukuStatus: <err>`。
- 底层：JNI → Java `shizukuStatusJson`：`Shizuku.pingBinder()`、`checkSelfPermission()==GRANTED`、`getUid()`。

### `system.vibrate`

调用形式：`host.system.vibrate(ms?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：触发手机震动（**不需要** Shizuku/ADB 权限）。
- 参数：`ms` number 可选，默认 500（毫秒）。
- 返回：JSON 表结构 `{ok=true}`；异常时 `{ok=false, error=...}`。
- 报错：桥调用失败抛 `system.vibrate: <err>`。
- 底层：参数 `{"ms":ms}` → JNI → Java `vibrate`（Vibrator）。

## util 命名空间

### `util.cancel`

调用形式：`host.util.cancel(id)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 timer_id，取消自己插件的定时器。取消成功返回 true，已经结束/不存在返回 false。不能取消其他插件任务。

### `util.ceil`

调用形式：`host.util.ceil(n)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（`round` 是 Go 的"四舍六入五成双"，对 0.5 特判）。

### `util.floor`

调用形式：`host.util.floor(n)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（`round` 是 Go 的"四舍六入五成双"，对 0.5 特判）。

### `util.join`

调用形式：`host.util.join(tbl, sep)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把 JSON 表结构 的所有值转字符串后用分隔符拼接。注意：遍历整表，非数组顺序不一定与下标一致。
- 参数：`tbl` JSON 表结构、`sep` string 必填。
- 返回：string。
- 报错：无。
- 底层：`t.ForEach` + `LVAsString` + `strings.Join`。

### `util.json_load`

调用形式：`host.util.json_load(path)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读取文件并解析为 JSON 对象。
- 参数：`path` string 必填。
- 返回：任意 Lua 值（表/数字/字符串…）。数字用 `UseNumber()` 解码避免精度损失。
- 报错：读文件失败或 JSON 解析失败抛 `json_load: <err>`。
- 底层：`os.ReadFile` + `json.Decoder`（`UseNumber`）→ `goToLua`。

### `util.json_write`

调用形式：`host.util.json_write(path, data)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把任意 Lua 值序列化为带缩进的 JSON 写入文件，自动建父目录。
- 参数：`path` string 必填；`data` 任意类型必填。
- 返回：boolean `true`。
- 报错：建目录/序列化/写文件失败抛 `json_write: <err>`。
- 底层：`os.MkdirAll` + `json.MarshalIndent(data,"","  ")` + `os.WriteFile`。

### `util.now`

调用形式：`host.util.now()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：返回当前本地时间。
- 参数：无。
- 返回：string，格式固定 `"2006-01-02 15:04:05"`。
- 报错：无。
- 底层：`time.Now().Format("2006-01-02 15:04:05")`。

### `util.pos`

调用形式：`host.util.pos(str)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把空格分隔的坐标字符串（如 `"100 64 200"`）解析成坐标表，最多取前 3 个 token，缺失项补 0。
- 参数：`str` string 必填。
- 返回：JSON 表结构 `{x=number, y=number, z=number}`。
- 报错：无。
- 底层：`strings.Fields` + `strconv.ParseFloat`。

### `util.read_file`

调用形式：`host.util.read_file(path)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：读取文件全部内容按字符串返回。
- 参数：`path` string 必填。
- 返回：string。
- 报错：读取失败抛 `read_file: <err>`。
- 底层：`os.ReadFile`。

### `util.round`

调用形式：`host.util.round(n)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（`round` 是 Go 的"四舍六入五成双"，对 0.5 特判）。

### `util.schedule`

调用形式：`host.util.schedule(intervalSec, handlerName)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 interval_seconds, handler；重复间隔 0.01 秒到一年。返回当前插件拥有的 timer ID。回调不带参数，由串行事件队列执行。取消只控制未来排队，已排队/执行的回调可能完成；停止插件清理全部任务。不要用它阻塞收包线程。

### `util.setTimeout`

调用形式：`host.util.setTimeout(sec, handlerName)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 seconds, handler；0 秒到一年，仅排队一次；返回当前插件 timer ID。Python 函数对象或函数名，C++ 已注册回调名。

### `util.split`

调用形式：`host.util.split(str, sep)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：按分隔符切分字符串为数组。
- 参数：`str` string、`sep` string 必填。
- 返回：JSON 表结构（字符串数组，1 起索引）。
- 报错：无。
- 底层：`strings.Split` + `RawSetInt`。

### `util.strip_color`

调用形式：`host.util.strip_color(text)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：去掉文本里的 Minecraft 颜色/格式码（`§` 与 `&` 及其后一个字符）。
- 参数：`text` string 必填。
- 返回：string。
- 报错：无。
- 底层：手写循环用 `strings.Builder` 逐字节过滤。

### `util.time_fmt`

调用形式：`host.util.time_fmt(ts?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把 Unix 时间戳格式化为可读的本地时间。
- 参数：`ts` number 可选，默认当前时间戳。
- 返回：string。
- 报错：无。
- 底层：`time.Unix(ts,0).Format("2006-01-02 15:04:05")`。

### `util.timestamp`

调用形式：`host.util.timestamp()`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：返回当前 Unix 秒级时间戳。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：`time.Now().Unix()`。

### `util.trim`

调用形式：`host.util.trim(str)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：去除字符串两端空白。
- 参数：`str` string 必填。
- 返回：string。
- 报错：无。
- 底层：`strings.TrimSpace`。

### `util.write_file`

调用形式：`host.util.write_file(path, content)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把字符串写入文件，自动建父目录，权限 0644。
- 参数：`path` string、`content` string 必填。
- 返回：boolean `true`。
- 报错：建目录/写文件失败抛 `write_file: <err>`。
- 底层：`os.MkdirAll` + `os.WriteFile(path, []byte(content), 0644)`。

## world 命名空间

### `world.block`

调用形式：`host.world.block(x, y, z)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：查询世界某坐标的方块名（需要机器人连接，目标为当前活跃机器人）。
- 参数：`x`/`y`/`z` number 必填（整数坐标）。
- 返回：string（方块名）。
- 报错：查询失败抛 `world.block: <err>`；参数非整数抛类型错误。
- 底层：`abot(L).GetBlockTileSafe(x, y, z)`（会传送加载区块防 outOfWorld）。

---

### `world.blockState`

调用形式：`host.world.blockState(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

x,y,z 整数坐标。返回当前维度 layer=0 的最近 UpdateBlock 观测，未知 known=false；包含方块 runtime_id，不把任何更新都认定是玩家破坏。

### `world.breakBlock`

调用形式：`host.world.breakBlock(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

x,y,z 整数坐标，mode=command 或 creative_action，默认 command。command 用 setblock ... air destroy 等待实际结果；creative_action 发创造模式破坏动作，要求游戏模式和六格范围，不代表服务器确认。生存挖掘的装备/耗时/交易流程尚不支持。

### `world.placeBlock`

调用形式：`host.world.placeBlock(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

x,y,z 整数坐标，block 例如 minecraft:stone。用服务器 setblock 指令，返回真实结果，需要权限。

## web 命名空间

### `web.action`

调用形式：`host.web.action(page, action, handler)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 page_name, action_name, handler。Python handler(request_dict)，C++ 回调 args[0]。动作调用保留当前运行实例；目标忙碌会明确报错，结果返回 JSON。停止后页面和动作一起移除。

### `web.emit`

调用形式：`host.web.emit(page, event, data)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：把一条事件实时推送给该插件的全部 SSE 客户端。声明式页面内置 JS 按 key 更新控件；自定义页 JS 自行监听。用于后台状态/进度推送。
- 参数：`page` string 必填（作为 SSE 事件名）；`event` string 必填（会写入数据 map 的 `__event` 键）；`data` 任意 Lua 值必填。
- 返回：无。
- 报错：数据无法 JSON 序列化抛 `web.emit: <err>`。
- 底层：`pushWebEvent(plugin, page, dataJSON)`，构造 SSE 消息写入所有客户端通道（通道满则丢弃，不阻塞 Lua）。**此方法在重跑上下文不跳过**（供 handler 内推送用）。

---

### `web.invoke`

调用形式：`host.web.invoke(参数见下方说明)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

参数 page_name, action_name, request_dict。外部控制端调用已注册的常驻动作，返回真实处理结果；本插件正在执行时调用自己的动作会被拒绝，避免递归死锁。网页控制台支持声明式页面；自定义 HTML 与 www 资源通过本机插件页面路径访问。

### `web.page`

调用形式：`host.web.page(name, desc?, controls?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：声明一个**声明式页面**，在门户 `/web/` 显示卡片，点进由内置模板渲染控件页。
- 参数：`name` string 必填（插件内唯一，不含 `/ \ # ? %` 及控制字符）；`desc` string 可选，默认 `""`；`controls` JSON 表结构 可选（控件表，可读字段 `type`=text|button|input|JSON 表结构|switch、`key`、`label`、`action`、`placeholder`、`columns`）。
- 返回：无。
- 报错：空名/非法字符/同插件页面名重复注册抛异常。
- 底层：`registerWebPage(&webPage{Kind:"declarative",...})`，写入全局 `webPages` map。

### `web.serve`

调用形式：`host.web.serve(name, entry?)`。位置参数与原命名一致；可选参数可省略，不照抄 Lua 语法进本语言。

- 作用：声明一个**自定义 HTML 页面**，直接服务 `www/` 目录下对应入口文件。
- 参数：`name` string 必填（规则同 `web.page`）；`entry` string 可选，默认 `"index.html"`（相对 `www/`）。
- 返回：无。
- 报错：同 `web.page`。
- 底层：`registerWebPage(&webPage{Kind:"html", Entry:entry})`；静态根目录 `插件文件/<id>/www`，经 `webStaticSafe` 防路径逃逸 + `serveWebStatic` 提供。

## 10. 验收、排错与迁移步骤

1. 先用离线 util、配置、计时和导出验证插件能加载，再检查 SDK 运行状态和错误计数。
2. 实际进服，观察 connected/权限；出现认证错误先解决身份，不把发送函数返回当完成。
3. 小范围测试指令、玩家查询、定向回复，再测试方块/结构/移动；保留游戏返回和日志。
4. 跨插件调用只导出必要函数；目标忙碌错误可以稍后重试，不构造互相调用的环。
5. 异常后查看最近错误、MCP 返回及日志；保存持久数据，停止再重新加载。C++ 崩溃则恢复账号进程。
6. 原 Lua 迁移：保留 API 方法名称，把 Lua table/pcall/函数名换成本语言结构/异常/回调；重写源码语法。框架不是 Lua → Python/C++ 自动编译器。
7. 原 ToolDelta/NTvector 迁移：将自身 engine/game_ctrl 接口和 Player 对象映射到本 SDK；不把未实现的兼容包当可直接导入。

方法清单是当前源码能力对照，不是每项游戏业务的成功报告。开发者应保存 API 响应、权限状态、真实游戏截图/日志作为验收依据。


## 游园 0.9.1 工作区共同能力与实例入口

# 游园实际开发与验收技能（APK 0.9.1 / SDK 1.1）

这是每个 Python、C++、Lua、命令链工作区自动获得的基础开发资料。语言手册分别为 python-runtime、cpp-runtime、lua-runtime、command-chain；完整手册按 load_skill(offset,max_chars) 分页。更多实例读 load_skill("toolbox-live-development",section="examples.md")，验收与排错读 section="verification.md"。自定义提示词、附加技能与工具权限保留；拥有文档不等于拥有该工具权限。

## 先观察，再操作

用 game_scene 读取当前账号观测；未开放该工具时用已允许的 api_call 调用 GET /api/youyuan/sense/snapshot。game.scene、bot.self、player.position/health/nearby 读取同账号数据。未知位置、血量、死亡状态必须保留未知；缺少 health 数据不等于满血。服务器通常不会向机器人同步所有玩家完整血量。events.onDeath/onHealthChange/onHurt/onMove/onRespawn/onBlockChange 是当前可注册的事件接口，具体触发取决于机器人实际收到的协议数据。world.breakBlock/placeBlock/blockState、bot.startMovement/stopMovement/lookAt/teleport 的参数看本语言方法手册，检查返回和游戏权限，不用假事件证明游戏效果。

## 游戏模式与真实性

新版默认不强制把机器人切换为创造。设置中 auto_creative（自动维护创造模式与飞行）是显式开关，受伤、死亡、生存挖掘测试先关闭；查询 scene.bot.game_mode 确认模式。OP 探测使用 testfor @s；有连接不等于有 OP。真实游戏中已验证生存模式连续八次指令后仍保留，测试后恢复原模式；尚未完成真实受伤、死亡与生存挖掘验收。

## 常驻运行与性能

Python 执行 Python 代码，C++ 执行目标 ABI 的原生库，Lua 执行 Lua。每实例保留状态与串行回调队列，不每次触发重新加载源码；进程停止后 RAM 不保留，需要 plugin.save / host.save。Python/C++ 宿主 API 复用一个常驻适配状态，并非完全移除 Lua API 适配。不能承诺所有任务快 1000 倍；网络往返、服务器 Tick 和命令权限决定大量操作延迟。C++ APK 没有内置编译器，需要 NDK 或已配置的 MCP 编译服务；下载匹配 ABI 的 plugin.so 再发布，不把源码上传当编译成功。

## 指令、批处理与命令链

game.sendCommand 仅发送；game.execute 等待真实 success/output。game.executeBatch 的 pipeline 适合无依赖指令，支持在飞批次；不是一个服务器端原子事务。依赖上一条结果的操作必须等待真实成功，超时结果未知，不能自动重发造成重复。终端 /say A./say B. 按段解析；小数和 JSON 字符串内的点保留。TXT 状态为 类型,条件,红石，独立行 . 分链、! 结束周期；保存时预解析，各实例独立调度。详情读 command-chain。

## 过滤抓包

先用 packet_schema 查询字段，再用 events.subscribeFiltered 或 events.captureStart/captureStop/captureList。只捕获本账号游戏连接实际收到的解码包，可按包类型和 every_ms 抽样，容量有界。订阅不等于拦截或改写包；不是手机其他应用的全局网络抓包。持续高频观察应过滤、节流、查看丢弃计数，停止任务时释放自己建立的订阅和捕获。

## 公网 MCP 和账号隔离

APK 内置本机 HTTP/MCP 服务及连接云端的主动 WebSocket 中继客户端，需要设置正确的设备密钥并保持中继已连接。GET /api/youyuan/remote 返回 public_urls.console/mcp/api 和 relay_connected；界面展示实际 URL。云端 /mcp/ 用云端控制密钥，本机 /mcp 用本机密钥，两者不要混用。公网并不意味着所有人无密码使用；账号、设备与独立账号功能区必须正确路由。设备关闭、进程被杀或中继离线时工具不可执行，不把地址存在当联通成功。工具名与资源从实际 MCP 列表发现，api_call 仍受当前工作区权限约束。

## 工作台、示例与持续任务

任务、项目文件、修订、摘要和历史按工作区保存，使用 project_list/read/search/write/patch/history/rollback 和已有的 MCP 工具。用户运行中输入用于引导当前任务；工具失败后保留已完成工作。AI 状态轮询新增结果时，用户向上阅读保持位置；点“回到最新状态”或滚到底部恢复跟随。阅读暂停不等于任务停止。现有控制、感知实例不是只能显示的隐藏功能；其 ID、导出 API 和网页动作见 examples.md，可据此继续开发。

## 更新、插件迁移与 QQ

更新站点 https://updates.youyuanqi.dpdns.org/；应用内下载校验 SHA-256、包名、版本和同一签名，再调用 Android 安装确认，覆盖安装保存数据。不要卸载或清空应用数据。多功能插件 ds_ai_agent 已做兼容导入，旧 TPS 包名映射到 SetTime，去掉源码内共享模型密钥；业务问题按用户要求暂缓，不能声称全部修好。

QQ 官方 AppID/AppSecret、群消息回调与允许执行指令的用户名单分别配置；群号不能替代 group_openid，真实回调负责绑定。官方群主动推送自 2025-04-21 停用，被动回复有窗口和数量限制；连续游戏聊天转发不能承诺无限发送。可按实际需要使用 OneBot 适配器。公开文档不写用户 Token、AppSecret 或设备控制密钥。游戏连接已验证不等于 QQ 双向互通已验收。


# 内置实例、导出和开发入口

实例通过 GET /api/youyuan/runtime/list 获取真实存在与状态；POST /api/youyuan/runtime/start 或 stop 传 id/account。原生示例依赖 APK 对应 ABI 的内置库，桌面运行不保证自动存在。它们默认由用户启动；不要自动启用会发欢迎消息的插件。

| 实例 ID | 语言 | 能力及入口 |
| --- | --- | --- |
| yy_sdk_python_demo | Python | 游戏聊天触发 `.插件状态`，持久计数、2 秒计时、玩家加入回调；控制页 control/status；导出 add/state/sum_cpp/execute。 |
| yy_sdk_cpp_demo | C++ | 原生游戏控制、定时和触发示例；源码与实际导出请读 SDK 的 cpp_seed.cpp 和 youyuan.hpp。 |
| yy_sense_python_demo | Python | 血量、受伤、死亡、移动、重生、方块变化监听；UpdateAttributes 250ms 过滤。控制页 monitor 的 scene/capture_start/capture_stop；导出 scene/metrics/batch_probe/health/capture_start/capture_stop。 |
| yy_sense_cpp_demo | C++ | 相同感知监听，原生对象保存计数；monitor 网页动作；导出 scene/metrics/health/batch_probe。捕获在停止对象时释放。 |

通过 POST /api/youyuan/sdk/call 调用已启动实例，JSON 字段 plugin_id、account、method、args。例如：

```json
{"plugin_id":"yy_sense_python_demo","account":"root","method":"game.scene","args":[]}
```

这读取宿主方法；跨插件导出函数使用本语言 `api.call("yy_sense_python_demo","metrics",[])` 或 Python `api.require("yy_sense_python_demo").metrics()`。调用前确认目标已运行，同实例隔离范围内操作；目标忙碌、递归调用和不存在的导出返回错误。控制页入口从 GET /api/youyuan/sdk/pages 发现；网页动作可用 web.invoke("monitor","scene",{})，不编造未注册动作。

Python 新工作区含可修改入口模板；C++ 新工作区含 youyuan.hpp、third_party/json.hpp 和入口模板。Python 使用 on_init/on_event/on_stop、原生使用 YY_PLUGIN 与 yy::Plugin；四个完整示例也已嵌入技能，调用 load_skill("toolbox-live-development",section="python_seed.py")、section="cpp_seed.cpp"、section="python_observer.py"、section="cpp_observer.cpp" 直接读取真实代码，再用 project_write/patch 生成或修改项目。源码同样位于交付包 upstream/sdk。技能读取不会自动把全部示例写入每个项目。

示例中的模拟事件只能检验回调语法和状态保留；验证真实死亡必须保存来自游戏的事件、实体标识、模式和时间。monitor 显示未知血量是正确观测，不要用 20 覆盖它。修改示例发布后重新加载；仅改磁盘文件不会替换当前运行模块。

## AI 工具与负载约束 · 0.9.5
工具和 Skill 根据语言独立配置。project_validate 读取当前项目文件并返回实际校验错误，C++ 的语义与 ABI 仍需编译器。project_publish 添加或更新当前项目的插件；plugin_manage(action=start/stop/reload) 只操作这个工作区已发布插件。game_call(function=execute,args=[命令,执行选项]) 可操作游戏，必须依据真实回包判断；服务器权限与用户任务范围仍适用。api_call、网络请求、系统 Shell 可在工具配置中显式启用，默认不为普通任务提供这些宽泛入口。MCP 文档按 mcp_describe 单工具加载。
同一进程最多 2 个运行中 AI 任务，每轮最多 8 次工具、单次任务最多 32 次工具/24 轮模型请求，连续 3 次工具失败暂停。保留文件、修改历史和原始任务供继续，限制不是重置任务。Skill 默认每页 3000 字，读取需要的小节即可。模型流式输出合并小块，页面只轮询新增事件，隐藏页面暂停轮询；原始工具结果保存在任务历史。
插件、AI、命令链的 SendCommandCh/SendCommandNoWait/executeBatch 通道共享账号的 20 毫秒间隔（最大约 50 条/秒），有响应等待时实际频率更低；建筑独立方块传输不受此限制。旧接口或外部 MCP 不能用来绕过节流。插件定时器仍由作者决定，推荐事件驱动、过滤、去重、限制观察量，避免无意义循环。
默认不安装常驻示例。升级时已自动生成的示例移动到应用数据目录的“示例备份”，保留源码和数据并取消恢复运行。示例作为开发文档与可选模板。皮肤由玩家进服数据包默认缓存，不需启用额外插件；在线皮肤列表自动刷新，建造时仍须选择目标玩家。


## 0.9.7 命令链与物理动作接口

命令链管理现在属于内置 `chains` 命名空间，可列出、读取、保存、配置、启停、导出、删除和通过聊天菜单控制。Python 使用 `chains.list()` 与 dict 配置；C++ 使用 `client.chains.list()`，其参数对象为 yy::json；Lua 使用相同的方法名。完整参数、返回值和语言示例按需加载 command-chain-api 技能。

新增 `bot.attack`、`world.mineBlock`、`world.useBlock`，同一账号使用实际库存/游戏协议动作，返回提交状态；是否产生伤害、破坏或放置必须观察服务器结果。不要把 setblock 当作生存挖掘。完整限制和参数按需加载 physical-actions。

视频方块画的解码、范围、fill/setblock差分和命令链导入使用 block-media 技能。三份技能都随 APK 打包，AI 可按需加载；公网 MCP 可按需读同名资源。没有把整套文档塞进身份提示词。


0.10.2：常驻链支持机器人有界流水线 mode=pipeline/window=1..16；真实条件屏障；批量执行可选择settings/pyrpc提交通道（无回包）。详细使用按需加载command-chain-api、command-execution技能。


## 0.10.4 完整附件与可调发送预算
AI工作区附件入口在对话输入区，ZIP保留目录结构，长粘贴保存文件；按需加载 toolbox-live-development 的 workspace-files.md（命令链也可从 command-chain 技能加载该章节）。用 project_read(offset,max_chars) 读取长行尾部，project_list(prefix,offset,limit) 分页列文件。
整批 game.executeBatch 的 options 和 chains.save/configure 的 config 都支持 rate_cps：0不加固定CPS等待，20..20000是配置的发送预算，非实际保证吞吐。UI可直接修改，条件等待仍是屏障，轮次不会重叠，回执超时不自动重发。


### 动态滑动窗口（0.10.4）

`game.executeBatch(commands, {mode="stream", channel="ai", window=512, rate_cps=0, timeout_ms=8000})`。Python/C++使用相同SDK options对象。stream单段1..4096条、文本512KiB，最大在途window=1..1024（默认128）。从16条起，成功回包累计达到当前窗口后倍增到上限；任何回包都释放槽位，不等待最早请求，也不等整窗全部回复。新补位命令立即按小批发送，不等待底层50ms刷新。`rate_cps=0`仅由在途窗口与回包背压约束；正数20..20000启用自定义共享账号发送预算。网络错误、超时、断开停止新发送，已发送回包仍收集，未知不自动重发。

命令链config同样支持`mode="stream",window=512,rate_cps=0`。有条件节点、通道变化、延迟、@call/@wait/@pause形成回包屏障。保持提交顺序；不保证独立命令的服务端完成顺序或同一游戏Tick。窗口仅服务于无真实依赖片段。整轮执行耗时算在周期内，仅等待剩余时间，不会堆积40Hz的未完成轮次。输出results逐条submitted/confirmed/success，peak_inflight是实测最大在途，network_batches是实际发送批数。
