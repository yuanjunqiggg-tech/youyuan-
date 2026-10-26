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
