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
