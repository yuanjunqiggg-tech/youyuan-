# 共享机器人账号池 · 游园 0.10.32

宿主已提供账号编排，跟随、传送、旁观模式和玩家离线判断属于插件逻辑。默认不启动跟随、不生成测试插件、不替玩家注册新游戏账号。一个机器人绑定一个玩家，直到明确 release；acquire 幂等并优先复用 idle。普通平台用户只能使用自己的账号池；项目管理员可 all=true 查询与分配全部托管账号。游戏 OP、服主显示分类与平台管理员是不同权限。

## 调用与真实结果

命名空间 accountsPool 同时提供给常驻 Lua、Python 和 C++ client.accountsPool。MCP 使用 api_youyuan_account_pool，工具参数 method=GET 或 POST。HTTP 路径 /api/youyuan/account-pool。POST 请求 {"action":"acquire","args":[{"player":"疣螈","server":"94F195C41B5"}]}。GET 默认 list，可 query.action=bindings，管理员 query.all=true。

|方法|参数|结果/含义|
|---|---|---|
|list|options?，all 默认 false|accounts、process_slots；不返回游戏Token、密码或内部身份密钥|
|add|account对象：id,name,token,auth_url,server,password?,keep_alive?|保存调用者自己的账号；owner_id 由真实会话决定，客户端填写无效|
|start|id,{server?,password?,connect?}|默认connect=true，真实分配独立进程、提交进服请求；connect=false只启动内核。connection_requested 不能代替 connected|
|stop|id|关闭连接、取消保活并释放该机器人绑定；不删除凭据|
|status|id|实际 connected/engine_ready/plugins/instances 等；未进服不是成功|
|bind|id,{player,server?}|持久绑定，不自动传送，不抢占已有绑定|
|acquire|{player,server,id?,all?}|预留账号并绑定；同一目标重复调用返回原绑定，优先 idle；不会自动 start|
|release|id,{idle_seconds?}|默认60秒空闲期；期间可被重新 acquire；超时宿主停用连接。0表示立即到期，1秒维护器处理。范围0..86400|
|bindings|{all?}|持久绑定/idle/error状态；绑定不等于已经连接|
|invoke|id,{method?,path,query?,body?}|转到这一个机器人的真实 /api/ 接口；method 默认GET。账户嵌套管理用 accountsPool，不使用此方法绕过隔离|

list/acquire/bindings 默认自己的池，管理员想管理其他人的池明确传 all=true；管理员知道目标 ID 时可 start/stop/invoke。子内核插件采用该机器人所有者身份访问池，即使主宿主管理员，也不会把管理员权限自动传给用户插件。旧存量账号归原主控平台账号。池元数据 account-pool.json 持久保存，内部密钥不返回 API。

## Lua：分配、启动、检查，然后由插件自行跟随

```lua
local lease = accountsPool.acquire({player="疣螈",server="94F195C41B5"})
local requested = accountsPool.start(lease.account,{server=lease.server})
local state = accountsPool.status(lease.account)
-- state.connected 为真后，插件自己的调度器运行传送/数据采集。
-- start 请求成功仍可能因Token失效、服务端拒绝或内核启动中而未连接。
-- 必须先获取 accountsPool.invoke 调用 bot.self() 返回的真实 name，不能拿账号ID当游戏名。
local view = accountsPool.invoke(lease.account,{
  method="POST",path="/api/youyuan/game/call",
  body={method="observerBridge.localView",args={{mode="coverage"}}}
})
-- view.ok/view.result 是这个机器人实际收到的数据；未知血量不能填20。
-- 玩家退出后：
accountsPool.release(lease.account,{idle_seconds=60})
```

已有定向通信 accounts.list/status/sendMessage/sendCommandsFast/callPlugin 继续保留；其定向授权规则仍生效，不被账号池暗中修改。插件也可经 accountsPool.invoke 调用目标的 /api/youyuan/game/call，以 game.sendCommandsFast 和 args 提交自己的命令数组。命令链的 Hz 仍属于整条链，没有新增逐命令CPS或回包等待。

## Python / C++

Python: `lease=accountsPool.acquire({"player":"疣螈","server":"94F195C41B5"})`，`accountsPool.start(lease["account"],{"server":lease["server"]})`。API可能启动进程和访问网络，放普通异步任务，勿在1ms战斗回调里同步等待。C++: `auto lease=client.accountsPool.acquire(yy::json{{"player","疣螈"},{"server","94F195C41B5"}});`；具体 JSON 别名以 SDK 头文件为准，可使用 nlohmann::json 对象。Windows 的C++插件运行支持仍以现有宿主支持范围为准。

## 观测边界与部署

账号池不保证全服动作广播。通过 invoke 逐机器人取 localView/血量/事件游标，插件按 server、session、来源账号和时间戳整合。实体 RuntimeID 按服务器和会话解释，不把不同世界相同数字当同一只实体。两秒跟随策略由插件写，可持续绑定和传送到同一个玩家周围；跨维度传送与旁观模式受游戏版本及OP权限约束，应查看真实确认/回包。不会把一次发送成功当作已经进入旁观模式。

本轮账号池/租户隔离、Lua调用、SDK调度、绑定恢复、空闲回收用本地真实宿主路由检查；官方注册和跨距离游戏动作没有新增现场实测。32是现有同时运行的独立内核位置数，不是注册账号数，也不是包接收上限。任意 native/Python 插件仍为用户自己执行端的受信代码，API账号隔离不等于同Unix用户进程下的完整文件系统沙箱。
# 游园账号中心与注册任务 · 0.10.33

本轮只修改工具箱宿主、认证服务、账号界面和开发资料，不改多功能插件。

账号页提供三个明确入口：游戏账号、机器人托管池、注册与认证任务。上方概览显示设备托管账号、真实连接数、服务器项目和绑定机器人数量；服务器项目从账号实际配置分组，可筛选多个游戏项目。点账号头像编辑工具箱头像、名称和身份分类；游戏昵称仍由认证服务提交官方修改。云端成功注册的游戏账号也显示在账号中心，即使尚未选服务器或尚未托管到本设备。

APK/工作台的注册成功回调会把真实签发的令牌保存为 `yy_<游戏凭据ID>` 的独立设备账号，不自动进服。注册网页创建的账号显示在“云端游戏账号”，点击“托管到本设备”接入当前执行端，再选择服务器。失败的任务不加入成功账号列表；待验证任务显示在注册页面，以原 pending_id 继续，不重新产生设备。

托管池使用真实 `accountsPool` 接口。列表可切换本人池或管理员全部托管池；可绑定玩家、启动到指定服务器、释放并等待60秒或停止。连接状态来自 status，不能把 enabled 或提交成功当成已进服。两秒跟随、传送和观察策略属于用户插件，不由宿主强制执行。

## 注册与验证接口

平台注册与游戏认证是两个步骤。认证网页 `/api/accounts`；工作台用 `/api/game-auth/accounts`；APK本机代理用 `/api/youyuan/game-auth`。

|action|用途|结果|
|---|---|---|
|login_4399|已有4399账号密码登录、接入Minecraft档案|成功才返回 account/login_token；HTTP202官方拒绝返回 retryable、provider_status、phase，不计成功|
|resume_4399|带原 pending_id、本人密码和需要时的 captcha/captcha_id 继续|保持同一设备；密码不持久保存|
|register_guest / resume_guest|创建游客、继续本人验证|等待实名或验证码时保存同一设备与安全验证地址|
|batch_create|count=1..10、nickname_prefix、require_nickname=true|建立游客批次；不立即注册|
|batch_next|推进当前游客账号|真实注册后进入 waiting_nickname，不能继续创建下一账号|
|batch_name|id=批次ID、nickname=2..20字符|实际提交游戏昵称；成功才计入 completed，然后允许下一项|
|batch_cancel|取消未完成任务|不会删除已创建的官方游戏账号；成功账号仍可管理|

GET 返回 accounts、pending、batches。pending 包含 last_error、verification_url、verification_kind、captcha_url、captcha_id、retryable/官方状态；不含密码或Cookie。待验证页面必须显示可操作按钮、密码/验证码字段、进度与错误；可取消和返回，不吞掉错误。

网页“每批10个”指十个不同游客档案逐个创建，官方安全验证会暂停。一个4399账号重复登录得到相同官方UID，不能据此宣称十个独立游戏账号。2026-10-10实测该用户指定4399账号得到HTTP202“请稍后再试”；增加Cookie/Origin的对照仍被拒绝，没有创建账号。本轮不把界面、模拟测试或重试成功提示当官方注册成功。

## Linux群服互通

配置与会话分开显示。AppSecret输入框隐藏不代表丢失，以 has_secret 判断；群号只是可读备注，真实发送绑定使用 group_openid。Linux正式会话本轮已鉴权并绑定群1121432132，但发送验收时没有有效被动回复窗口。QQ群官方被动回复需要真实 msg_id，5分钟内且每条最多5次；不能伪造或把所有未发送消息标成成功。需要24小时主动推送时，可切换用户自己的OneBot通道；没有配置该通道不算可用。

官方文档：https://github.com/tencent-connect/bot-docs/blob/main/docs/develop/api-v2/server-inter/message/send-receive/send.md

账号池/API按用户隔离不等于任意Python/C++代码的完整文件系统沙箱。保持模块通信、原Lua资源、Fast命令发送与整链Hz语义，不因界面改动删减宿主能力。



## 0.10.58 自动采集调度
`accountsPool.captureStatus/captureConfigure`详见本技能fleet-scheduling.md，可直接读真实plan/states，不能把submitted当覆盖率。
