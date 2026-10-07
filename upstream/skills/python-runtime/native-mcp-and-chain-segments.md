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
