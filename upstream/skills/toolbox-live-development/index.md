## 插件资源与模块

`plugin.codeDir()` 源码目录；`plugin.dataDir()` 持久数据目录；`plugin.configDir()` 配置目录；`plugin.resourceDir()` 优先resources子目录，旧包无该目录则使用源码根；`plugin.tempDir()` 数据目录下.tmp。`plugin.readResource(relative, "utf8")` 读取文本，二进制跨语言使用 `"base64"`；`plugin.existsResource(relative)` 检查存在；`plugin.listResources(".")` 递归返回path/dir/size；`plugin.extractResource(source, destination?)` 原字节复制到自己的数据目录并创建父目录，不是解压ZIP。

相对路径不能逃出目录，不接受绝对路径、驱动器、..或符号链接。Lua默认配置/数据沿用旧目录以保兼容，旧game.dataDir不变；Python/C++ SDK数据和配置按账号目录隔离。普通Lua ZIP导入保留模块和资源，市场包保留源码及资源、排除私人数据/配置；完整备份ZIP含源码、资源、配置和数据，空目录也保留。ZIP防路径穿越与防解压炸弹的资源容量保护不影响命令发送频率。

Lua `require("lib.helper")` 搜索插件源码根的lib/helper.lua或lib/helper/init.lua，package.loaded在常驻VM内缓存；只在停用/重载后重新加载。Python推荐包内相对导入 `from .lib import helper`，不依赖不同插件的同名全局模块。

## 0.10.7 框架执行路径

`game.sendCommand(command)` 保持旧无等待调用方式；新增 `game.sendCommandFast(command, options?)` 与 `game.sendCommandsFast(commands, options?)`。后者一次宿主调用提交完整数组，没有整个数组的行数限制。魔法/ai 的无等待路径使用 SettingsCommand、保留机器人 `@s` 和相对坐标；提交按输入顺序，网络层按原定时周期合并，不逐条等回包、不默认CPS等待。`submitted` 只表示提交，`confirmed=false` 不代表游戏成功。用户主动给正数 `rate_cps` 才限速。

`game.sendCommandConfirmed` 是 `game.execute` 的别名，等待 CommandOutput；超时是未知，不自动重发。`game.executeBatch` 的 ordered/pipeline/burst/stream 保留可确认路径；其中 window 是在途请求数量，不是整个文件长度。无条件链可选 `mode="fast"`；有条件节点及其直接前一条自动切到确认路径，`@wait/@call/@pause` 控制语义保留。Hz 是完整链的目标轮次/秒，超期不积压轮次。

Lua: `game.sendCommandsFast({"scoreboard players add counter test 1", "scoreboard players add counter test 1"})`。Python: `host.game.sendCommandsFast(commands)`；C++: `client.call("game.sendCommandsFast", nlohmann::json::array({commands}))`。先校验整个数组；网络失败可能已有部分提交，不应盲目重试。Fast不保证原子或同一服务端Tick，也不能自动判断每条成功。显式 PyRPC 在当前测试服100条无执行，不能将旧源码的通道推断为当前所有服务器可用。


## 常驻调度、诊断与能力查询

旧 `util.schedule(seconds, handler)` / `setTimeout` / `cancel` 保留。新增 `scheduleSeconds(seconds, handler)`、`scheduleHz(hz, handler)`、`scheduleTick(ticks, handler)`。Tick是本地名义50ms，不是服务器时钟。频率针对整个回调/整条链；同任务不重入，不补无限积压。Python/C++一次定时回调未完成时不会继续往队列塞同任务。慢回调漏过的周期记missed，主动暂停期间也计入missed。

`util.listJobs()` 返回当前插件任务的id、handler、paused、calls、missed、failures、last_ms/max_ms/total_ms/last_error。`pauseJob(id)` / `resumeJob(id)` 只控制当前插件的任务；暂停不强杀已经执行或入队的回调。停用取消所有定时器及订阅。SDK每实例串行队列256项；Lua事件串行队列128项，满时记dropped而不新建无限goroutine。旧secondary机器人仍可按首次事件懒加载同插件VM；自己的变量不混入其他账号。

`plugin.stats()` 查常驻回调次数、错误、墙钟耗时和队列信息；Lua耗时不含获取VM锁前的等待，定时任务耗时包含其VM/SDK队列等待。CPU、内存、GC没有可靠逐实例测量，字段为null。不要把这些墙钟数当CPU负载或声称1000倍。错误保留插件文件名、行号、handler和耗时。Lua到Lua跨插件调用不再JSON往返，Python/C++仍使用必要的跨语言边界。

`plugin.has("game.sendCommandsFast")` 检查接口；`plugin.has("runtime.cpp")` 在Linux为false、Android为true。`plugin.capabilities()` 返回平台/方法列表/语言运行器能力。接口存在不等于服务端允许动作，也不等于已观测到其他玩家血量。方法文档按需读；外部MCP调用资源、任务、stats要走SDK的plugin_id/account上下文，不能当全局游戏调用。

Lua: `timer=util.scheduleHz(20,"round"); function round() game.sendCommandsFast(commands) end`。Python: `timer=host.util.scheduleHz(20, round)`；C++: `client.util.scheduleHz(20,"round")`。参数20是每秒完整回调次数，不是每秒20条命令。


## 0.10.7 连续发送与整链频率
默认 rate_cps=0 表示不加人为 CPS 等待。game.sendCommand 不再逐条睡眠20ms；pipeline、submit、burst不再默认限制200条/秒。只有主动配置正数 rate_cps 时才启用发送上限。Lua、Python、C++共用这条分发路径。
默认ai无等待用SettingsCommand并按原网络定时周期合并；可确认/其他通道仍有内部有界批次，不限制TXT总行数。sendCommand返回submitted仅表示提交，最终效果用回包或计分板核对。无限循环仍应让出定时调度，不能把网络提交率当服务器执行率。
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

# 游园实际开发与验收技能（APK 0.9.6 / SDK 1.2）

这是每个 Python、C++、Lua、命令链工作区自动获得的基础开发资料。语言手册分别为 python-runtime、cpp-runtime、lua-runtime、command-chain；完整手册按 load_skill(offset,max_chars) 分页。更多实例读 load_skill("toolbox-live-development",section="examples.md")，验收与排错读 section="verification.md"。自定义提示词、附加技能与工具权限保留；拥有文档不等于拥有该工具权限。

## 先观察，再操作

用 game_scene 读取当前账号观测；未开放该工具时用已允许的 api_call 调用 GET /api/youyuan/sense/snapshot。game.scene、bot.self、player.position/health/nearby 读取同账号数据。未知位置、血量、死亡状态必须保留未知；缺少 health 数据不等于满血。服务器通常不会向机器人同步所有玩家完整血量。events.onDeath/onHealthChange/onHurt/onMove/onRespawn/onBlockChange 是当前可注册的事件接口，具体触发取决于机器人实际收到的协议数据。world.breakBlock/placeBlock/blockState、bot.startMovement/stopMovement/lookAt/teleport 的参数看本语言方法手册，检查返回和游戏权限，不用假事件证明游戏效果。

## 游戏模式与真实性

新版默认不强制把机器人切换为创造。设置中 auto_creative（自动维护创造模式与飞行）是显式开关，受伤、死亡、生存挖掘测试先关闭；查询 scene.bot.game_mode 确认模式。OP 探测使用 testfor @s；有连接不等于有 OP。真实游戏中已验证生存模式连续八次指令后仍保留，测试后恢复原模式；受伤与死亡已取得真实事件，生存挖掘的完整动作仍需另行验收。

## 常驻运行与性能

Python 执行 Python 代码，C++ 执行目标 ABI 的原生库，Lua 执行 Lua。每实例保留状态与串行回调队列，不每次触发重新加载源码；进程停止后 RAM 不保留，需要 plugin.save / host.save。Python/C++ 宿主 API 复用一个常驻适配状态，并非完全移除 Lua API 适配。不能承诺所有任务快 1000 倍；网络往返、服务器 Tick 和命令权限决定大量操作延迟。C++ APK 没有内置编译器，需要 NDK 或已配置的 MCP 编译服务；下载匹配 ABI 的 plugin.so 再发布，不把源码上传当编译成功。

## 指令、批处理与命令链

新增共同技能 command-execution：三语言插件可指定每实例默认通道、逐次覆盖通道和批量 ordered/pipeline/submit 模式。工作区“命令执行参数”保存为 command-options.json，AI 和用户编辑同一文件；发布并重载后生效。修改执行配置前先加载这份技能。

AI 模型配置现支持 context_window、max_output_tokens、compression_percent。按所选模型的上下文估算（含系统提示、摘要和工具参数文档）触发压缩，同时预留输出空间，原始历史保留。服务端模型列表如返回 context_window 等元数据，选择候选可自动填入；未提供时由用户填写容量，默认 32768 是保守配置，不声称供应商真实容量。估算并非精确 Tokenizer；超过不可压缩的当前输入预算时保留任务并提示缩小输入。

game.sendCommand 仅发送；game.execute 等待真实 success/output。game.executeBatch 的 pipeline 适合无依赖指令，支持在飞批次；不是一个服务器端原子事务。依赖上一条结果的操作必须等待真实成功，超时结果未知，不能自动重发造成重复。终端 /say A./say B. 按段解析；小数和 JSON 字符串内的点保留。TXT 状态为 类型,条件,红石，独立行 . 分链、! 结束周期；保存时预解析，各实例独立调度。详情读 command-chain。

常驻 TXT 编辑器也支持函数执行器的 `#组`、`#测试`、`#执行`，可指定每组通道、速率、前缀、次数并预览预解析结果。生成命令链前先加载 command-chain 技能；`玩家`通道是真实玩家来源，`魔法指令`和`控制台`当前都使用自动化来源，不能承诺独立的服务器控制台权限。

## 过滤抓包

先用 packet_schema 查询字段，再用 events.subscribeFiltered 或 events.captureStart/captureStop/captureList。只捕获本账号游戏连接实际收到的解码包，可按包类型和 every_ms 抽样，容量有界。订阅不等于拦截或改写包；不是手机其他应用的全局网络抓包。持续高频观察应过滤、节流、查看丢弃计数，停止任务时释放自己建立的订阅和捕获。

## 公网 MCP 和账号隔离

APK 内置本机 HTTP/MCP 服务及连接云端的主动 WebSocket 中继客户端，需要设置正确的设备密钥并保持中继已连接。GET /api/youyuan/remote 返回 public_urls.console/mcp/api 和 relay_connected；界面展示实际 URL。云端 /mcp/ 用云端控制密钥，本机 /mcp 用本机密钥，两者不要混用。公网并不意味着所有人无密码使用；账号、设备与独立账号功能区必须正确路由。设备关闭、进程被杀或中继离线时工具不可执行，不把地址存在当联通成功。工具名与资源从实际 MCP 列表发现，api_call 仍受当前工作区权限约束。

## 工作台、示例与持续任务

任务、项目文件、修订、摘要和历史按工作区保存，使用 project_list/read/search/write/patch/history/rollback 和已有的 MCP 工具。terminal_log 可读取最近 200 条游戏连接、插件状态和任务结果事件；game_scene、packet_schema 与 listen_packet 用于实际感知和抓包。用户运行中输入用于引导当前任务；工具失败后保留已完成工作。AI 状态轮询新增结果时，用户向上阅读保持位置；聊天面板的“上页 / 下页 / 最新”按钮和滚轮可浏览长历史，回到底部恢复跟随。阅读暂停不等于任务停止。复制、修改或重发历史消息会建立独立分支；有时间戳的消息会按修订历史回退分支文件，旧消息需核对文件。多工作区可切换，Token 用量来自供应商 usage，费用按用户设置的每百万 Token 单价估算，不代表供应商账单。现有控制、感知实例不是只能显示的隐藏功能；其 ID、导出 API 和网页动作见 examples.md，可据此继续开发。

## 更新与插件迁移

应用内下载校验 SHA-256、包名、版本和同一签名，再调用 Android 安装确认，覆盖安装保存数据。不要卸载或清空应用数据。市场下载的是 ZIP 插件包，安装后进入本地 Lua 插件目录；市场返回 JSON 错误或加密包时必须显示真实错误，不能假装已安装。区域迁移在“其他”里提供顺序模式：来源服执行实际建筑导出到手机，确认文件生成后切服，再向当前服执行实际建筑导入；这不是两个账号并发迁移，不要在导出完成前切服。多功能插件 ds_ai_agent 已做兼容导入，旧 TPS 包名映射到 SetTime，去掉源码内共享模型密钥；业务问题按用户要求暂缓，不能声称全部修好。

## 0.9.4 导出与外部 MCP

兼容 Lua 插件的私有备份通过 /api/plugin/export-saf 导出插件文件、插件配置文件、插件数据文件的完整目录 ZIP；市场发布包不包含私人配置与运行数据。Python/C++ 通过 /api/youyuan/runtime/export 导出其完整插件目录，ZIP 根目录保留 manifest.json、入口文件及资源子目录。账号聊天和在线采样从 /api/youyuan/observations/export 导出到用户选择的位置。外部 MCP 先 mcp_list 获取简短目录，mcp_describe 只读取选中工具的 inputSchema，再 mcp_call；当前语言 Skill 仍通过 load_skill 分页读取。公网连接地址可配置兼容的自建中继，当前界面显示由该连接推导的候选 MCP/控制台地址，只有连接状态确认后才能当作可用网址。

## AI 工具与负载约束 · 0.9.5
工具和 Skill 根据语言独立配置。project_validate 读取当前项目文件并返回实际校验错误，C++ 的语义与 ABI 仍需编译器。project_publish 添加或更新当前项目的插件；plugin_manage(action=start/stop/reload) 只操作这个工作区已发布插件。game_call(function=execute,args=[命令,执行选项]) 可操作游戏，必须依据真实回包判断；服务器权限与用户任务范围仍适用。api_call、网络请求、系统 Shell 可在工具配置中显式启用，默认不为普通任务提供这些宽泛入口。MCP 文档按 mcp_describe 单工具加载。
同一进程最多 2 个运行中 AI 任务，每轮最多 8 次工具、单次任务最多 32 次工具/24 轮模型请求，连续 3 次工具失败暂停。保留文件、修改历史和原始任务供继续，限制不是重置任务。Skill 默认每页 3000 字，读取需要的小节即可。模型流式输出合并小块，页面只轮询新增事件，隐藏页面暂停轮询；原始工具结果保存在任务历史。
插件、AI、命令链的 SendCommandCh/SendCommandNoWait/executeBatch 通道共享账号的 20 毫秒间隔（最大约 50 条/秒），有响应等待时实际频率更低；建筑独立方块传输不受此限制。经由这些通道的 MCP 调用也遵守节流；直接构造底层数据包的旧接口不在此范围内。插件定时器仍由作者决定，推荐事件驱动、过滤、去重、限制观察量，避免无意义循环。
默认不安装常驻示例。升级时已自动生成的示例移动到应用数据目录的“示例备份”，保留源码和数据并取消恢复运行。示例作为开发文档与可选模板。皮肤由玩家进服数据包默认缓存，不需启用额外插件；在线皮肤列表自动刷新，建造时仍须选择目标玩家。

已安装的 Python/C++ 插件在管理页可以启停、查看目录、导出整个 ZIP、删除，或把现有源码载入 AI 工作区。工作区插件操作使用 project_publish 发布，plugin_manage(action=start/stop/reload/delete)。删除清除插件目录与数据，仅在玩家明确要求时执行；仅有 C++ 二进制时先提供源码及编译 MCP，不要假装能够直接修改二进制。

## 已安装 Lua 插件的真实源码工作区

从插件编辑页进入 AI 帮写时，工作区绑定当前插件 ID，将该插件源码目录的入口、模块、插件说明及资源复制到项目，保留实际文件名。界面显示绑定 ID、源码入口及文件数。文件名只是索引；project_read 按行读取真实内容，project_search 查找定义与引用，不需要玩家重新上传。大文件分页读取，project_patch 用唯一 old/new 文本修改；工具 Diff 只返回变化附近，完整文件与修改历史仍保存，避免百万字符源码的整份 Diff 挤爆模型上下文。

修改先保存工作区。project_validate 检查相应文件；project_publish 将已修改文件写回对应的真实插件目录；plugin_manage(reload) 才让已发布修改在运行实例生效。仅打开文件或保存工作区并不等于运行插件已更新。同步已安装源码会保留工作区未发布的修改，双方同时修改同一文件时返回冲突，不能强制覆盖。源码不存在、无读取权限、含不允许的符号链接或超出容量时会报告接入失败，不用模板冒充源码。

此入口只导入当前插件的源码与资源目录，不自动把其他插件、应用密钥或 ds_ai_agent 自己的工作区历史塞入上下文。运行数据不在源码快照内；确需读取时使用已允许的 API/MCP 查询当前插件对应的数据文件，按需读取并避免回显密钥。

全新安装不生成演示或验收工作区。识别到旧验收项目时，将完整目录与历史移动到 ds_ai_agent/工作区备份，不删除用户自建项目；“移出工作区”也是归档操作，不停用或删除已安装插件。


## 0.9.7 命令链与物理动作接口

命令链管理现在属于内置 `chains` 命名空间，可列出、读取、保存、配置、启停、导出、删除和通过聊天菜单控制。Python 使用 `chains.list()` 与 dict 配置；C++ 使用 `client.chains.list()`，其参数对象为 yy::json；Lua 使用相同的方法名。完整参数、返回值和语言示例按需加载 command-chain-api 技能。

新增 `bot.attack`、`world.mineBlock`、`world.useBlock`，同一账号使用实际库存/游戏协议动作，返回提交状态；是否产生伤害、破坏或放置必须观察服务器结果。不要把 setblock 当作生存挖掘。完整限制和参数按需加载 physical-actions。

视频方块画的解码、范围、fill/setblock差分和命令链导入使用 block-media 技能。三份技能都随 APK 打包，AI 可按需加载；公网 MCP 可按需读同名资源。没有把整套文档塞进身份提示词。


## 0.10.4 完整附件与可调发送预算
AI工作区附件入口在对话输入区，ZIP保留目录结构，长粘贴保存文件；按需加载 toolbox-live-development 的 workspace-files.md（命令链也可从 command-chain 技能加载该章节）。用 project_read(offset,max_chars) 读取长行尾部，project_list(prefix,offset,limit) 分页列文件。
整批 game.executeBatch 的 options 和 chains.save/configure 的 config 都支持 rate_cps：0不加固定CPS等待，20..20000是配置的发送预算，非实际保证吞吐。UI可直接修改，条件等待仍是屏障，轮次不会重叠，回执超时不自动重发。


### 动态滑动窗口（0.10.4）

`game.executeBatch(commands, {mode="stream", channel="ai", window=512, rate_cps=0, timeout_ms=8000})`。Python/C++使用相同SDK options对象。stream单段1..4096条、文本512KiB，最大在途window=1..1024（默认128）。从16条起，成功回包累计达到当前窗口后倍增到上限；任何回包都释放槽位，不等待最早请求，也不等整窗全部回复。新补位命令立即按小批发送，不等待底层50ms刷新。`rate_cps=0`仅由在途窗口与回包背压约束；正数20..20000启用自定义共享账号发送预算。网络错误、超时、断开停止新发送，已发送回包仍收集，未知不自动重发。

命令链config同样支持`mode="stream",window=512,rate_cps=0`。有条件节点、通道变化、延迟、@call/@wait/@pause形成回包屏障。保持提交顺序；不保证独立命令的服务端完成顺序或同一游戏Tick。窗口仅服务于无真实依赖片段。整轮执行耗时算在周期内，仅等待剩余时间，不会堆积40Hz的未完成轮次。输出results逐条submitted/confirmed/success，peak_inflight是实测最大在途，network_batches是实际发送批数。
