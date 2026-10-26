# 常驻内核、命令缓冲与资源重载

## 连接与创造飞行维护（0.10.36）

设置的三项连接/维护开关即时保存，失败回退。`GET /api/youyuan/connection-policy`读取真实连接状态、暂停、连续失败轮数与`creative_confirmed`、`flying_confirmed`；`POST`部分更新`auto_connect`、`auto_reconnect`、`auto_creative`、`reconnect_seconds`，不会清空账号令牌。`{"resume":true}`恢复手动暂停。没有失败轮数上限；单次登录150秒超时并支持取消。新子账号维护默认开启，已有明确关闭保留。维护每2秒核对状态，发请求不视为服务器确认。改模式需OP；维持飞行与关闭控制面板独立。远端调用仍须相应账号权限。

这是游园0.10.17实际框架接口。入口语言由工作区/manifest决定，三种语言手册都可以多选；允许读取技能不等于给工具权限。正文按章节读取，没有自动联网动作。

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

TXT保存后parse成节点/控制点/行号IR，常驻调用，运行时不读TXT、不解析字符串。1,0,0脉冲；2,0,0 20为整个TXT实例20Hz；0,0,0连锁；0,1,0有条件；.独立链断开成功状态；!结束周期。未重新声明的属性继承。file调度使用整个TXT的整轮Hz；segments调度按“.”分隔独立链，每段可有独立Hz，段内仍按序执行，脉冲段只一次；同一顺序段不接受中途换Hz。不同独立段并行调度且分别防重入。第三位红石保留语法兼容，不在宿主做游戏红石门控。run_count=0持续，正数按完整文件轮次计数。

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

plugin.stats、util.listJobs和chains.list有实际调用数、耗时、missed、丢弃、队列和整轮统计。耗时是墙钟，不是CPU占用；逐插件CPU/内存/GC尚未归因，返回null。高速发送不逐条写命令日志。极端重复大链可能让RakNet/服务器承受不住，应实测最终计分；本地提交完成不等于服务端完成。可选每Tick命令预算/overflow carry、完整混合模块manifest尚未交付。Linux 0.10.19 已接入常驻 C++ 子进程及崩溃/卡死回收；Android 原生仍在宿主进程中加载。


## 服务器关闭命令反馈时

实服已验证：gamerule sendcommandfeedback false 会使 Confirmed/querytarget 等结果接口收不到回包，即使命令已经执行。Fast 不等待回包，模式维护也使用无等待创造命令和飞行输入。需要条件链、查询结果或确认时，要由服主确保服务器允许命令反馈；超时返回未知，不能把它当失败自动重发。高频无条件链不要为语法验证逐条增加确认。


## 0.10.17 兼容接口与 AI 上下文修正

常驻 Lua 在插件加载时编译一次，事件调用已有函数。终端显示队列满时只跳过显示消息，游戏 Text 包和插件事件仍继续分发；单个畸形包不会终止整个收包循环。不能把这个修复视为全部游戏数据包都已实测。

### 原插件接口补充（Lua）

`game.getOnlinePlayers()`：向已连接服务器实际查询 @a，返回玩家名数组；查询失败会报错，不用空数组伪造在线情况。

`game.sendCommandWithResp(command, channel="ai", timeout_seconds=5)`：确认式命令，返回真实 CommandOutput 表。通道支持 ai/console/player。超时报错，表示结果未知，不能自动重发。高频无条件链继续用 sendCommand/sendCommandFast 或 CommandBuffer.submitFast，不要为每条命令套确认。

`game.getServerTPS()`、`game.getTPS()`、`game.serverTPS()`、`game.getTickRate()`、`game.getServerTickRate()`：相同的服务器 SetTime 时钟观测值。连接前、尚无有效观测或观测过期返回 nil；不是客户端发送吞吐，也不保证等于服务器精确平均 TPS。

`game.importBlocks(blocks, options)`：创建本机建筑导入任务。blocks 是数组，每项为 `{x=相对X,y=相对Y,z=相对Z,name="minecraft:stone",states=状态字符串或表,nbt=NBT表}`；相对坐标非负，绝对起点放在 options。options 支持 x/y/z、dimension、speed、region_mode、anti_loss。speed=0 无用户固定速率，正数为自定义速率；旧插件 -1 兼容映射到0。region_mode 为1/2/3，即16/32/64格区域；anti_loss=true 启用区块可查询探测，不等于每个方块收到确认。真实导入由本机宿主进行，不把结构传给原作者服务。插件停用取消任务。没有1000方块限制；内部坐标编码要求每轴相对坐标小于2^20。

`game.importWait(job)`：等待上述任务，发送/导入错误明确报错；true 表示本机导入流程完成，不代表每个方块都有独立服务器确认。job 是 Lua 本实例的 userdata，不能直接跨语言 JSON 传递。Python/C++ 的结构任务请使用 building 工具接口，不能假装这个 userdata 可以跨语言序列化。

```lua
local job=game.importBlocks({
 {x=0,y=0,z=0,name="minecraft:oak_stairs",states={facing_direction=2,upside_down_bit=false}},
 {x=1,y=0,z=0,name="minecraft:stone"}
},{x=100,y=70,z=100,dimension="overworld",speed=20,region_mode=1,anti_loss=true})
assert(game.importWait(job))
```

### 寄存器、上下文和任务连续性

本项目 Lua 编译器单函数寄存器上限为5120，指令格式扩展为64位。原 Lua 源码保持兼容，旧预编译字节码不兼容。此改动不等于游戏命令速度增加。

压缩阈值只由 context_window × compression_percent 计算。356000、80% 得284800，不再减掉 max_output_tokens 后偷偷提前压缩。请求输出预算按剩余上下文另行计算，不覆盖用户保存的配置。上下文估算覆盖实际发送的提示、消息和工具定义；有供应商 input_tokens 时用于校准，界面明确标作估算，未声称精确 tokenizer。

不再把消息或工具结果统一截到10000字符。压缩按输入预算分段覆盖全部选中历史，保留原始记录、文件、任务状态和工具结果。摘要保存为内部上下文，不作为普通聊天刷屏；压缩失败保留原历史。模型输出触及长度上限时显示等待继续，保留任务，不把截断的回复标为完成。工具调用参数 JSON 不完整会报错，不能当空参数执行。

内置AI回复默认说明实际改动、验证结果和阻碍，避免重复计划、长篇自述和输出压缩摘要。工作区可读取实际文件、技能章节和 MCP 工具；玩家禁用的工具仍禁用。文件/技能分页接口的 offset 是读取协议，不是忽略尾部内容的理由。

### 本轮验证范围

Linux 全量 R21B 核心加载、原文件资源、原始两份 TXT、本机 MCP 派发、真实服务器回送 .help 到注册 onChat 均通过有限验收。真人屏幕是否显示全部菜单、真人死亡与击杀者、经济/公会等业务逐项测试尚未全部完成。

MCP 服务端 OAuth+PKCE、initialize、tools/list、tools/call、resources/read、刷新令牌已实测；普通 Chat 客户端挂载仍需单独验证，不能用服务器通过代替客户端成功。


## 游戏出口线路（0.10.73）

`GET /api/youyuan/game-egress` 返回配置和当前连接的实际运行状态；POST仅保存下一次连接设置，不断开当前游戏。system使用机器人实际宿主网络（Android为手机，Linux为Linux），网页HTTP中继不等于游戏UDP出口。official选择Pharos候选，失败回落会在运行状态明确标记。socks5使用获授权自有节点的UDP ASSOCIATE，必须显式确认授权，失败不静默直连。TanLobby WebRTC暂不支持指定SOCKS5出口，明确拒绝而不是谎报生效。

代理密码独立保存于私有配置，读取接口只返回has_password；普通配置及运行遥测不带密码。`udp_association_established`、`verified_game_session`、实际UDP字节和数据报计数分别核验；连通本地代理不等于外部节点或公网出口IP已通过。配置默认system，不把Linux网络宣称玩家手机网络。
