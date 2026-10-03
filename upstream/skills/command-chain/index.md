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

## 0.10.3 跨链与有界调度
支持 @call（目标源文件行号/次数/时限/持续）、@wait（默认失败后再成功的边沿）、@pause及chains.trigger。全部机器人内执行，无递归任务，无周期积压。完整本地开发资料：load_skill("command-chain",section="control-flow.md")。单次burst<=1024，默认无固定CPS，整批可用rate_cps调节，不能声称上亿/秒或每轮同Tick。

## 最新：机器人整批协议执行
mode=burst 每次最多1024条/512KiB文本，逐条真实确认，保留账号冷却。1000条实际测试出现同Tick，也出现跨2Tick；不保证每轮同Tick。不放置命令方块，条件/延迟形成等待屏障。完整参数见下文“整批协议发送”。

# 命令链开发说明

## 0.10.2 机器人内常驻命令链

所有TXT在机器人内预解析和运行，不部署命令方块，不改世界方块，不切创造。严格顺序 mode="remote" 每条等待真实结果；有界流水线 mode="pipeline"、window=1..16（默认8；window是每批最多在途命令的条数，不是毫秒或时间窗口）、delay_ms=0：连续无条件、同通道、无延迟命令成批提交，之后按顺序收集回包。遇到有条件命令、通道切换、指定延迟形成等待屏障。条件只看上一条真实成功；. 新链不继承结果；! 周期结束。每文件独立启停，run_count=0持续，正数为整轮次数。

流水线保持提交顺序，不保证服务器完成顺序，也不保证1000条同Tick。有先后依赖的无条件命令必须用严格顺序。插件/AI/命令链批量调用按账号串行调度；stream预算0时使用有界回包背压，其余方式共享发送预算，流水线最多16条在途，批量默认无固定CPS上限，burst可设置rate_cps，单条默认无额外等待；实际速率看服务器回包，不能把提交数当执行成功数。每条额外延迟可为0，不再重复强制增加20ms。

chains.save("工作.txt", text, {"mode":"pipeline","window":8,"delay_ms":0,"interval_ms":1000,"run_count":2,"enabled":False})；chains.start/stop/list/get/configure/menu/handleChat 共用UI/API/MCP同一运行器，账号隔离不变。Lua用table，C++用client.chains或通用call，Python用chains。runtime.executed记录尝试次数；submitted记录流水线已提交数；cycles仅完整轮完成后增加；超时结果未知，不自动重发危险指令。

插件 game.executeBatch 增加 transport="command_request"（默认，支持ordered/pipeline/submit）、"settings"、"pyrpc"。后两者仅支持submit，没有逐条成功确认，禁止当条件依据。settings为SettingsCommand，pyrpc为网易ExecuteCommandEvent，不伪装成有回包的调用。玩家channel使用玩家命令来源；ai与console在有回包接口中都使用自动化来源，换名字本身不提升速度。



每个 TXT 是独立运行实例。目录：插件数据文件/ds_ai_agent/command_txt/。保存时预解析，Tick 不重复拆字符串。

属性 类型,条件,红石：0连锁/1脉冲/2循环；0无条件/1有条件；第三位保留 0/1 以兼容旧 TXT，新调度器不作红石门控。未重新声明时继承属性。独立行 . 结束当前链，下一链重新开始，不继承执行结果。独立行 ! 结束周期。

```text
2,0,0
say 周期入口
0,1,0
say 上一条成功
.
1,0,0
say 独立脉冲
!
```

有条件指令要求上一条真实服务器响应成功，失败/跳过/超时均阻止后续条件链；无条件可以继续。新版按完整文件轮次调度，不论入口是脉冲还是循环。run_count=0 为持续运行，正整数 N 为执行 N 个完整轮次后自动停止，不同 TXT 独立。游戏未连接、没有权限时如实返回错误。

## 函数执行器语法与常驻调度

同一个 TXT / mcfunction 文件也能使用原版函数执行器的 `#组`、`#测试`、`#执行` 写法。`#组` 设置组名及 `通道=0/1/2`、`速率=毫秒`、`次数=N`、`前缀=命令前缀`；组在保存时被编译为独立链，可与三位状态和 `.` / `!` 混用。次数表示该组每次调度执行的次数，上限 1000；速率设置逐条延迟，实际间隔至少为文件延迟和 20 毫秒。`#测试` 定义参数模板，`#执行` 用参数展开；展开在保存时完成，不在每个周期重做。模板后的命令需要用下一条 `#组` 或 `#测试` 结束定义区域。

```mcfunction
#组:巡检 通道=1 速率=200 次数=1
1,0,0
say 巡检开始
0,1,0
testfor @a
.
#测试:问候 名字:≤Steve≥
say 欢迎 ≤名字≥
#执行:问候 名字=Alex
!
#组:心跳 通道=0
2,0,0
say 周期到达
```

默认通道在编辑页选择。`1 / 玩家` 用玩家来源发送，受玩家权限约束；`0 / 魔法指令` 和 `2 / 控制台` 在当前协议里都使用自动化命令来源，两者并非服务器真实控制台权限。发送的是游戏协议 `CommandRequest`，不是聊天文本，也不会在世界中放置实体命令方块。编辑页“预览解析与通道”可在保存前检查每条命令的组、通道、链和周期；运行时条件链依据 `CommandOutput.SuccessCount` 判断成功，超时视为未知并阻断有条件的后续命令。对玩家通道高频批量发送应配置适当速率。

终端 /say A./say B. 分段按顺序发送。JSON 字符串和数字小数点保留。单条 / 指令不需点结尾。


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

## 当前调度与 AI 工作区
管理页设置运行次数：0 持续执行，1 只执行一轮，N 执行 N 轮。次数统计整个文件的完整轮次；#组 次数是每轮内部组的重复次数，两者相乘。每轮结束后按 interval_ms 等待，最小 25 毫秒，命令之间至少 20 毫秒，同账号指令接口共享最大约 50 条/秒发送节奏。等待回包与延迟会降低实际速率；周期不会重叠。未连接时不消耗轮数。连续三次传输错误/超时自动暂停，服务端明确失败继续遵循条件链，不自动重发未知结果。
第三位红石属性只用于读取旧格式，当前 APK 不模拟真实红石；管理页移除红石/脉冲按钮，整文件按运行次数执行。
命令链 AI 只需要本语法和 Bedrock 命令文档，使用 project_read、project_patch/project_write 编辑，project_validate 查看当前文件的真实语法错误，修正后按用户要求 project_publish。校验只验证文件格式和属性，游戏具体命令语义最终由服务器返回结果确认。MCP 可按需启用；默认不抓包、不获取实体、不管理其他插件。


## 0.9.7 插件管理接口

同一运行器可以从 Lua/Python/C++ 的 chains 命名空间管理，包括聊天分页菜单。写插件时加载 command-chain-api；TXT工作区只需本语法技能。图片/视频生成方块TXT的流程见 block-media。


## 整批协议发送（burst，实验性）
所有执行仍在机器人内，不放置命令方块。`game.executeBatch(commands, {mode:"burst",channel:"ai",transport:"command_request",window:1024,timeout_ms:8000})` 一次最多1024条、512KiB命令文本，将这段封装成一个网络批次，并逐条收集真实确认。整批默认 rate_cps=0（无固定CPS），0.10.4 可设置 rate_cps=20..20000，避免循环无限洪泛。网络批次相同不保证服务器同Tick执行，应依据实测首尾 `time query gametime` 和真实结果判断。
命令链 config.mode=burst、window=1..1024。无条件、相同通道、没有逐条延迟的连续段整批发送；遇到有条件命令、通道变化或延迟会等待结果，保持条件链语义。这些等待需要网络往返，不能承诺含条件的一千条都同Tick。remote严格逐条确认；pipeline窗口1..16；burst整批顺序提交，服务器实际顺序须验收。


## 0.10.4 完整附件与可调发送预算
AI工作区附件入口在对话输入区，ZIP保留目录结构，长粘贴保存文件；按需加载 toolbox-live-development 的 workspace-files.md（命令链也可从 command-chain 技能加载该章节）。用 project_read(offset,max_chars) 读取长行尾部，project_list(prefix,offset,limit) 分页列文件。
整批 game.executeBatch 的 options 和 chains.save/configure 的 config 都支持 rate_cps：0不加固定CPS等待，20..20000是配置的发送预算，非实际保证吞吐。UI可直接修改，条件等待仍是屏障，轮次不会重叠，回执超时不自动重发。


### 动态滑动窗口（0.10.4）

`game.executeBatch(commands, {mode="stream", channel="ai", window=512, rate_cps=0, timeout_ms=8000})`。Python/C++使用相同SDK options对象。stream单段1..4096条、文本512KiB，最大在途window=1..1024（默认128）。从16条起，成功回包累计达到当前窗口后倍增到上限；任何回包都释放槽位，不等待最早请求，也不等整窗全部回复。新补位命令立即按小批发送，不等待底层50ms刷新。`rate_cps=0`仅由在途窗口与回包背压约束；正数20..20000启用自定义共享账号发送预算。网络错误、超时、断开停止新发送，已发送回包仍收集，未知不自动重发。

命令链config同样支持`mode="stream",window=512,rate_cps=0`。有条件节点、通道变化、延迟、@call/@wait/@pause形成回包屏障。保持提交顺序；不保证独立命令的服务端完成顺序或同一游戏Tick。窗口仅服务于无真实依赖片段。整轮执行耗时算在周期内，仅等待剩余时间，不会堆积40Hz的未完成轮次。输出results逐条submitted/confirmed/success，peak_inflight是实测最大在途，network_batches是实际发送批数。
