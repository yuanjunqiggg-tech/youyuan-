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
