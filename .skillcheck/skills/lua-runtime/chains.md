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

# chains · 命令链管理

同一套TXT预解析、整链频率、启停、跨链与聊天菜单，可供插件调用。


### `chains.configure`

```lua
chains.configure(name, patch)
```

- 功能、参数、返回与约束：name,patch，合并并校验配置，保留 TXT；运行中会停止并按 enabled 重启。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.delete`

```lua
chains.delete(name)
```

- 功能、参数、返回与约束：name，先停止再删除 TXT 与配置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.export`

```lua
chains.export(name)
```

- 功能、参数、返回与约束：name，返回完整 TXT 与配置，插件可自行写入导出目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.get`

```lua
chains.get(name)
```

- 功能、参数、返回与约束：name，读取完整 TXT、配置和状态。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.handleChat`

```lua
chains.handleChat(player, text)
```

- 功能、参数、返回与约束：player,text；处理 .命令选择 与 .链 列表/启动/停止/次数/频率/通道/源码/删除。修改要求玩家 OP，返回 handled。配合 events.onChat。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.list`

```lua
chains.list()
```

- 功能、参数、返回与约束：无参数，列出当前账号的链、配置、计数、错误和状态。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.menu`

```lua
chains.menu(player, page)
```

- 功能、参数、返回与约束：player,page=1，仅向该玩家发分页聊天菜单；不会阻塞等待消息。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.reload`

```lua
chains.reload(name)
```

- 功能、参数、返回与约束：name；只重新读取/编译TXT资源，当前轮使用原IR，下轮用新IR；不重启插件、保留启停与统计；无效修改保留原链。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.save`

```lua
chains.save(name, content, config)
```

- 功能、参数、返回与约束：name,content,config；config 支持 interval_ms(至少1，整链周期)/delay_ms/run_count/channel/enabled/mode/window/rate_cps。mode=fast无等待提交（条件及其前条确认）、remote严格顺序、pipeline有界流水线、burst整批最多1024、stream动态滑动窗口1..1024（默认128，rate_cps=0不设固定CPS，回包立即补位，网络错误停止补发；正常无目标不丢弃无条件后续）；全部在机器人内运行。TXT支持 @call JSON 跨链源文件行号范围/次数/时限/持续，@wait JSON 条件边沿等待，@pause手动触发；读 command-chain 技能。账号固定为当前插件账号。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.start`

```lua
chains.start(name)
```

- 功能、参数、返回与约束：name，启用独立常驻实例；已启用不重复启动。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.stop`

```lua
chains.stop(name)
```

- 功能、参数、返回与约束：name，停止并等待当前实例退出。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.trigger`

```lua
chains.trigger(name)
```

- 功能、参数、返回与约束：name，释放当前链的 @pause 等待点一次；没有等待点时不会自动重启已停止的链。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.validate`

```lua
chains.validate(content)
```

- 功能、参数、返回与约束：content，预解析并返回命令/条件/通道/行号；失败含报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
