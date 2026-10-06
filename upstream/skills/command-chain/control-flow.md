# 0.10.10 命令链修复与结构拆链

所有命令链在当前账号的机器人内预解析并长期复用，不在世界中放置命令方块。默认 mode=fast，rate_cps=0，delay_ms=0。Hz是整个TXT一次完整运行的目标轮次，不是每个节点的速率。过慢时不重入、不积压补跑。

## 发送与条件

无条件、同通道、无延迟的连续段一次交给底层，按原文順序发送；不存在Fast文件总行数上限或每批1024的限制。pipeline/burst/stream是用户明确选择的回包确认模式，它们的窗口只影响这些模式，不约束Fast。收到包的4096缓存与发送命令数量是两件事。

Fast设置逐条延迟，只做发送后等待，不自动要求回包。只有条件节点、它的直接前驱、@wait和@branch条件检测才要求真实CommandOutput。提交成功不作为条件成功。超时返回结果未知，不重发。服务器关闭sendcommandfeedback时条件/分支可能拿不到结果，应恢复反馈或使用可信事件。

## 基本格式与兼容

1,0,0为脉冲；2,0,0 20为目标整链20Hz；0,0,0为无条件连锁；0,1,0为有条件连锁。未重新声明属性沿用当前状态。`.`划分独立链并清空前一链执行结果；`!`结束当前周期。界面中的运行次数为完整TXT轮数，0持续。保留三位旧语法，红石位不模拟真实世界红石供电，不能把旧红石依赖当作已完全迁移。

旧函数执行器#组、#定义/#测试、#执行参数替换继续支持。通道ai/player/console和逐条延迟保持配置；需要结果依赖才主动切换确认通道。

## 子链调用、分支与断链

```text
say 开始
@branch {"command":"execute if score gate state matches 1","then":"技能开启.txt","else":"技能关闭.txt"}
@call {"name":"特效.txt","from_line":2,"to_line":8,"count":3}
@pause
say 已手动继续
@wait {"command":"testfor @a","poll_ms":500,"edge":true}
say 条件再次满足
```

@branch检测一次真实结果，只调用then或else一个文件，缺省分支可留空；子链结束回到下一行，不持续轮询。@call运行目标文件已编译IR指定源行范围，含两端；同账号，独立实例不会被悄悄启动。count=0必须有keep_open或duration_ms；持续调用结束于父链停止/时限。子链遵循自己的整链Hz/interval_ms，等待=周期减去本轮耗时，不再附加25毫秒下限；不创建递归任务。递归引用明确拒绝。

@wait默认先见失败、再等成功，edge=false可当前满足就继续；poll_ms控制检测频率。@pause仅等待手动trigger，UI“触发等待点”和插件chains.trigger(name)均能继续。

调用/分支不把方块转向、clone接线盲目翻译成命令。模块的状态与启停可由插件chains.start/stop/configure管理；事件如events.onSwing触发chains.start或sendCommandsFast，避免每Tick全员扫描。Lua/Python/C++均使用同一chains API，准确函数签名见各语言chains章节。

## 结构中的命令方块提取

工具→结构与指令转换→输出“提取命令方块·按朝向拆成独立链”。读取原有结构解析器保留的Command/auto/conditionalMode/TickDelay与facing_direction，箭头的下一格必须是连锁命令方块才接续；相邻但未指向的方块不合并。脉冲/循环是独立入口，闭环有限拆分并警告。下载JSON包含源坐标、NBT派生属性、连接索引和每个TXT完整原文；可一次保存全部未启动TXT，默认单轮。

接口POST /api/youyuan/convert/structure，data为文件base64，filename为文件名，target=command_chains，origin=[x,y,z]为结构原世界执行原点，rotation建议0。含~的命令用execute positioned原点+源位置包装，避免变成机器人坐标。含^还须审核原执行朝向；条件制约原方块背后关系、空节点、原生TickDelay、红石与相对执行上下文均需审核。

涉及command_block的setblock/fill/clone保留原文并列出警告；转换器不理解任意动态控制流，也不会删除真实tp/fill坐标。请核对后改为@call/@branch/chains启停。结构转任意完整玩法、自动消除坐标二分算法、通用神器效果引擎仍待实现；不把AI建议中的示意函数冒充已有API。

## 接收缓存与诊断

设置→服务器连接→收包缓存，默认4096，正整数自定义，下次重连生效；配置字段packet_queue_capacity=0采用4096。/api/bot/status返回packet_queue_depth及packet_queue_capacity。缓存是未处理包数量，不是CPS或链长度。短同步入队保序，满时对读包线程施加背压；断线/停用会解除等待，不逐包创建goroutine。

这次只做代码回归、构建和部署核验，未开展新的游戏压力测试。Fast不保证游戏服同Tick执行、原子性或固定性能倍率；既有大流量断线失败继续保留。


---
以下保留原有接口与格式参考；发送语义和新增修复以以上0.10.10说明为准。

# 0.10.3 机器人命令链：跨链调用、等待与有界循环

所有控制流运行在机器人内，不向世界放置命令方块。控制行以 @ 开头，是预解析节点，永远不作为游戏命令发送。控制节点会隔开整批窗口，必须等前段真实回包后进入。普通条件行仍以真实上一条成功决定继续。

## 调用其他链的部分源码

```text
1,0,0
scoreboard players set phase task 1
@call {"name":"子链.txt","from_line":3,"to_line":8,"count":2,"duration_ms":10000,"keep_open":false}
scoreboard players set phase task 2
@wait {"command":"execute if score gate task matches 1","poll_ms":500,"edge":true}
!
```

from_line/to_line是目标文件物理源码行号，包含端点，0为不限；头部、注释和空行也计入源码行号，但只有选中范围中的已解析节点执行。切片保留保存时解析的类型、条件、通道、延迟和分组属性；切片第一条条件命令没有上一条成功时会跳过。target必须是同账号的直接 .txt/.mcfunction文件名。父链等子片段结束才执行后面的命令，子片段使用目标保存的通道、执行方式、窗口、组重复和间隔；目标自己的 enabled/run_count不会被改变，也不会替代已启动的独立任务。如果目标同时在独立运行，两者都会执行，应主动停掉独立任务避免重复。

count默认1；正数为本次片段轮数。count=0为持续，必须指定 duration_ms>0或 keep_open=true。keep_open=true且省略count时持续；显式count优先。duration_ms为本次调用最长墙钟时长，0无限时，最多24小时；到期停止本次片段并继续父链，未完成的已发送命令结果不保证，不能撤销服务端已经收到的动作。父链停止会取消子片段，不启动后台子任务。父链每个完整轮次都会再次执行这条调用。

不能递归引用自身或间接调用环，最大调用深度8。需要“自闭链”时让同一个调用 count=0/keep_open 重复，或文件 run_count=0持续；不会用递归创建越来越多任务。

## 断开、再次满足条件和手动触发

@wait {"command":"execute if score gate task matches 1","poll_ms":500,"edge":true}
停在当前位置，默认先观察条件失败，然后等待下一次成功才放行。这样条件一直保持真也不会每轮立刻重开。edge=false会在下一次检测成功立即继续。检测需用只有成功/失败的条件命令；不要把give/setblock等写操作当检查命令，检查命令本身确实会被发送。poll_ms为100..60000，默认500，并共享账号发送预算。正常失败是条件未满足；超时/断线是错误，暂停而不冒充满足条件。

@pause
停在当前位置等一次手动触发。管理页点“触发等待点”，或插件 chains.trigger("主链.txt")、MCP通过当前账号SDK调用同一方法放行。触发一次只保留一个信号，不无限累加；没有等待点时不会自动重启已停止的链。停止始终可以取消等待。

断链可以在一段末尾放 @wait/@pause；重新满足条件后，当前段继续，整轮结束后下轮从入口开始。可以用另一条链修改gate计分板，以成功回包连接两个分支。独立 . 分段不继承执行结果；并行独立文件可以各自启停，账号的命令发送仍统一限流。

## 频率、吞吐和崩溃保护

interval_ms>=25，表示一轮完成后的最短休息时间，25ms是40Hz调度意图，不保证服务器每秒执行40轮。上一轮没结束不新建下一轮、不补发错过的Tick。40Hz不是每条命令都创建40个任务。3000条循环一次会分成最多1024条的窗口，每批等回包再推进；失败结果不自动重发。控制调用/条件/不同通道/逐条延迟形成批次屏障。

同账号默认批量预算count*5ms；0.10.4整批rate_cps=20..20000可调整为count/rate_cps秒（单条至少20ms），200是默认值，额外等待服务器回包。最多一个批量调用在途，包含运行中最多8个批量调用等待，超出返回繁忙；停止能取消排队、冷却和回包等待。已发送的最多一个窗口仍可能在服务器完成，不能把“本机停止”当撤销服务器动作。

1000条一次网络整批不等于单个命令、不等于1000倍服务器算力。20TPS不是20条命令/秒；真实样本中千条轻量命令曾在同一gametime内执行，也有跨Tick。没有每轮同Tick保证。不要声明上亿次每秒、无限自闭频率或fill重操作与计分板轻操作相同性能。

UI预览、插件chains.validate使用同一解析器并返回真实行号/control节点；读取文件→小范围修改→预览校验→明确启动→用回包/计分板验证。新增示例属于文档，不默认安装、启动为插件。Lua: chains.trigger("主链.txt")；Python: chains.trigger("主链.txt")；C++: client.chains.trigger("主链.txt")。三语言参数使用各自JSON对象/table，不用独立循环读取TXT。
