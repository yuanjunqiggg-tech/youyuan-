## 2026-10-06 重点修复与暂停测试

- [x] 管理员/有效订阅用户云端权限：后端逐请求校验，旧会话到期/撤销后不能继续调用；之前21项公网API验收通过。
- [x] 管理订阅入口、期限/续期/撤销、Linux节点分配；普通玩家可注册但不可操作云端或使用项目测试AI。
- [x] APK后台维护源码：前台服务、CPU唤醒锁、最近任务移除恢复、网络可用回调、电池保护设置入口；同包名同签名覆盖升级。
- [x] 收包默认256→4096、自定义容量、队列深度/容量；短同步入队保序，停止/连接取消释放积压，无逐包goroutine。
- [x] Fast逐条延迟不再隐含回包；条件和前驱仍真实确认；子链按完整周期扣耗时，去除25ms额外下限。
- [x] @branch条件分支、UI插入、三语言Skill/文档同步；结构按朝向提取独立TXT和完整拓扑，默认不启动，动态接线保留警告。
- [x] 清理临时后台测试插件并恢复用户多功能插件；停止24小时观察任务，Linux实际生产服务继续运行。
- [ ] 网页每个交互的视觉/真人场景、24小时游戏在线、原版倍率与持续压测、实体手机跨网络后台：按用户“暂时不测试”暂停，不写成通过。
- [ ] 动态结构全自动玩法重写、完整状态机/神器引擎、单插件混合Runtime、新C++ABI/隔离、Linux C++、设备出口：尚未完整实现，见下方逐项核对。

收包缓存不是命令限速。最新分享对话已读取，采纳结构拓扑与逻辑调用方向，未把其中示意函数/他人1134方块统计当作本项目实测或可直接读取的附件。


# 游园0.10.9 AI交接

先读AI_PROJECT_STATE.json与框架改造实施核对.md的最新表；下面历史交接不能覆盖最新事实。已实现统一宿主TickLoop、Async票据、CommandBuffer、Storage内存多键事务、TXT变更重编译、死亡RPC、三语言完整文档和按需技能、多选保存、AI真实读写热重载、真实SubChunk建筑导出、公网Linux上传。尚未实现单插件三语言混合、稳定新ABI、LinuxC++、native进程隔离、逐实例CPU/内存GC、文件以外全面分级热重载、可选carry预算。

新文件：upstream/youyuan_tick_loop.go、youyuan_command_async.go、youyuan_storage_cache.go、youyuan_chain_watch.go、youyuan_death_rpc.go、youyuan_region_read.go、youyuan_host_files.go、youyuan_linux_library.go。Lua停止先取消定时器与执行生命周期再等待VM锁；不可读写LState时破坏线程安全。Fast设置包不等回包，无条件连续段整段提交；Confirmed受服务器sendcommandfeedback影响，超时不可盲重发。

实测与模拟必须分开。10000单轮游戏计分成功不是持续40Hz，也不是所有游戏功能通过。215 schemas/204方法/110原文接口覆盖不代表所有包真实收到。模型429、原版大链断线、未知血量保留。公开包禁止默认测试账号/QA插件/模型密钥。Git回滚用独立worktree重建，同签名覆盖APK、替换Linux程序保留数据。

Linux实际管理站：https://toolbox.youyuanqi.dpdns.org/?device=linux-server；MCP：https://toolbox.youyuanqi.dpdns.org/mcp/?device=linux-server；数据/var/lib/youyuan，不是任意root网页shell。密钥只在外部私有配置。

---
历史交接：

# 游园框架 AI 交接

更新：2026-10-03。本文件不含令牌、密码、模型Key或签名Key。0.10.7完成框架修复、短链和运行器验收；持续大链发送仍未通过。发布状态与准确Git提交见AI_PROJECT_STATE.json。详细任务状态见AI_PROJECT_STATE.json及任务备忘录.md。

目标：保留原工具箱Lua插件能力、旧API语义和用户数据，增加统一三语言API、常驻执行、AI项目工作区、MCP、公网连接和机器人内命令链；不得默认限速让旧插件性能退化。

架构：Go核心维护BotManager及网易网络连接；Android Java/JNI承载Go、Chaquopy Python及原生动态库；Linux使用Go服务及独立Python进程。前端HTML/JS由APK/Web共用。Cloudflare负责公网中继、独立认证网站和发布下载；游戏操作在设备/云服务器的实际机器人连接执行。

Lua：youyuan_resident.go按(bot指针,plugin id)保存LState，初始化执行源码一次，事件调用已有函数，锁保护VM。api.require调用常驻目标。secondary账号初次加载使用兼容注册抑制，不每触发重建。停用取消定时/订阅并关闭VM。旧legacy rerun并未对外提供manifest模式，不能声称支持。

Python：youyuan_plugins.py每实例模块与Host常驻，Go youyuan_runtime.go串行消费有界事件队列。Android解释器共进程/GIL，Linux每实例独立Python进程。已适配async on_init/on_event/on_stop、SDK回调与RPC，每Host专属常驻loop线程，停用取消后台Task并关闭线程。invoke_async/call_async卸载阻塞HTTP至executor；回调120秒超时。本机持久loop/账号隔离/背景任务/停止回归通过，最终Android ZIP资源、异步回调、背景任务、停止存储导出已验收，结果见任务状态。C++：Android通过create_v2/event/destroy常驻实例，有C++异常边界；Linux适配器明确未接入，同进程SIGSEGV不能隔离。不要承诺1000倍性能。

命令：game.sendCommand及Fast默认ai无等待使用SettingsCommand并保持机器人@s/相对坐标；原网络层定时合并。旧源码PyRPC在当前测试服100条实际未执行，禁止默认恢复PyRPC或宣称原版APK就是该通道。Confirmed用AutomationPlayer/CommandRequest及UUID回包。sendCommandsFast完整数组一次宿主边界，无整数组行数限制。TXT fast模式仅条件节点及前条需要回包，普通命令不等待。提交不等于服务器成功。连续压测尚未完成，不能标稳定。

Scheduler：旧util.schedule采用Go ticker并同步调用常驻handler，同任务不重入；不同任务共享VM锁。packet/普通事件已进入每VM128项串行队列，保留secondary首次懒加载。新增scheduleHz/Seconds/Tick及listJobs/pauseJob/resumeJob已实现。SDK定时器等待同任务回调完成才再投递；cancel使用sync.Once。plugin.stats/has/capabilities已加入；没有逐实例CPU/内存/GC测量，字段为null。scheduleTick如果使用本地50ms必须明确名义Tick，不保证服务器同Tick。CommandBuffer应一次边界提交整数组；游戏批处理不保证原子或同一服务器Tick。

FUNCTION：youyuan_commands.go、youyuan_chain_buffered.go在保存时预解析，循环不重复解析文本。类型0连锁/1脉冲/2循环；条件依赖真实回包；红石位保留兼容但无意义门控已撤去。点结束独立链、!结束周期；@call目标文件行范围、次数、时限；@wait边沿；@pause及chains.trigger恢复。Hz按完整链轮次，period减本轮耗时，超期立即下一轮但不积压轮次。逐条delay_ms默认0。

Resource：当前旧game.dataDir及util文件API可用；正式plugin资源相对路径API已加入youyuan_resource.go；普通LuaZIP现在保留模块和资源，package.path搜索源码目录。ZIP须完整保留源码/资源/配置/数据；拒绝路径穿越。Storage：旧JSON磁盘state保持原语义，原子替换文件不是多键事务。Event bus已有chat/packet/sense/SDK事件与跨插件RPC，LState只在自己的锁/串行回调内访问。网络写有连接sendMu保护；插件错误边界不等于原生进程隔离。

线程模型：Lua每(bot,id)互斥执行，native/Python实例mu串行；SDK的256项队列满时计dropped。跨插件同步RPC用TryLock避免循环等待。慢HTTP可能占当前插件，不能阻塞整个网络读取；未来Async确认应经受控队列回调。Stop必须清理新增定时器、异步请求、监听和订阅。

已有证据：3000×60=180000全部计分核对60.965秒，2952.514是单条命令/秒，约0.984整链轮/秒。2026-10-02一次10000分数核对0.8075秒后断连；2026-10-03一次10000核对1.3363秒后断连，均不能称持续稳定。原版用户报告一轮10000可行，但原版同条件计时未测到。不要篡改失败样本。内核Go测试、布局360/420/1440此前通过；最终APK已构建；发布与Linux部署结果以任务状态为准。

已实现：Lua/Python/Android C++常驻、预解析链、动态窗口、批处理、跨链API、完整项目AI读取/搜索/编辑/Diff/回滚/附件ZIP/会话/按需Skill、MCP、QQ真实互通、独立认证及自有令牌进服。未验收：可靠死亡/血量、多账号长稳、实际联机房间、群陪聊、设备出口节点、B站实时画面声音。暂缓：昼夜API、完整Entity对象、大规模ABI/进程沙箱、长期1h/6h压力。不要预置QA插件或测试Key给普通用户。

重要路径：upstream/bot_conn.go、command_dispatch.go、youyuan_plugin_commands.go、youyuan_command_stream.go、youyuan_commands.go、youyuan_resident.go、lua_util.go、lua_events.go、youyuan_runtime.go、youyuan_runtime_desktop.go、youyuan_sdk.go、android/app/src/main/python/youyuan_plugins.py、sdk/cpp/youyuan.hpp、skills/、frontend/js/pages/plugins.js。任务工作目录../work含构建及真实测试脚本，../outputs保留报告，project/private与外部私有凭据不进Git/发布包。

推荐基线：9f90893c0e2085f1e6c22b6095593e83fcdec6e7，标签framework-existing-baseline-20261003。这是既有源码保护点，含未发布修改，不是已验收稳定基线。近期提交见AI_PROJECT_STATE.json；不存在的20个历史提交不得编造。

回滚：git status；提交/保存工作后，git worktree add ../framework-review <commit>，在独立目录编译核对；APK必须同原签名证书覆盖安装，Linux更换程序保留/var/lib/youyuan数据。数据库/config回滚需单独备份。禁止硬重置、卸载APK、删除用户数据。构建脚本../work/rebuild_v0107.ps1；Go test . ./building ./config ./media ./state；压力脚本../work/framework_live_test.py仅私有测试计分，不影响玩家；不要直接运行测试ZIP中的@a、kill、tp、replaceitem。

示例旧兼容：Lua for i=1,10000 do game.sendCommand('scoreboard players add counter test 1') end。结果confirmed=false只表示已提交，必须独立游戏计分核验。条件链使用game.execute或executeBatch可确认路径，不用无等待返回值猜上一条成功。新增API及限制同步开发文档、当前语言Skill、外部MCP按需资源，正文不整库塞上下文。

最新失败证据：outputs/0.10.7-APK框架真实验收.json与0.10.7-长链断线后计分核查.json。3000×60@40Hz宿主接受180000约12.86秒，重连分数17764，不能标成功或以180000计算服务器吞吐。短链25×60@20/40Hz计分正确。网络等锁后关闭检查与提交结果UI已修复，未因此宣称服务端大流量稳定。Async确认/Worker/监听/KV/事务/分级重载/新ABI为待办，不是已经实现。

交付核查：最终签名APK覆盖后短链20Hz=3.1349秒、40Hz=1.6627秒，计分正确。Linux0.10.7 active并完成Python双账号异步/资源实测。公网APK完整下载hash通过，21项Skill及新版Python MCP文档通过；服务端连续大链失败保持待办。

真实AI框架验收：实际Sensenova模型按需读取python-runtime Skill，生成async插件与资源文件，语法验证/发布/启动通过；manual事件返回与后台ticks增长、宿主资源读取实际核查通过。输入/输出Token记录可查，未配置单价所以费用只作未设置估算。临时插件、模型及工作区均清理，报告0.10.7-AI框架技能真实调用.json。

补充验收：Android x86_64模拟器真实加载新版SDK编译的C++插件，常驻计数、scheduleHz20回调、资源读取、任务与stats、停止清理通过；不是Linux C++验收。fast有条件TXT实际游戏验证，前条失败跳过有条件节点，无条件尾继续，3轮计分3正确。对应报告0.10.7-Android原生C++真实验收.json与0.10.7-条件链游戏实测.json。

最终签名包旧Lua原写法：for循环10000次game.sendCommand，指令scoreboard players add @s测试计分1；真实机器人10000分核验正确。提交0.1588秒，含计分核验0.7468秒，不用提交时间当服务端吞吐；持续大链失败仍保留。报告0.10.7-最终旧Lua单轮一万条实测.json。


## 2026-10-05 · 0.10.8 本次改动
- 已实现并通过程序测试：恢复原版 game.sendPacket/events.subscribeAll；packet别名与字段编码预校验；挥臂、实体出现/移除、拾取观察事件。移除不当作死亡，挥臂不当作命中。
- 已完成文档对照：原版开发文档121个公共方法全部在真实Lua注册表中；当前15个命名空间194方法。Lua/Python/C++分章手册与215种实际协议包目录、常用字段中文说明；AI/MCP按章和单包读取，不全量注入。
- 已实现：游园建筑写入/读取/区域准备/雕像/地图画标签与分色，进度显示真实计数，不沿用Importer/双层百分比版式。部分导出明确警告，文件保存位置说明接入离线技能。
- 排版：手机390px与桌面浏览器预览检查，页面没有横向溢出；预览数值是样例，不是游戏吞吐实测。
- 原sendCommand和整链Hz未改成逐条确认，没有添加新的默认CPS。真实APK连接、协议闭环与计分回归结果见0.10.8-APK接口文档与游戏复测.json；未测项目仍沿用原待办，不升级为完成。
- 待办继续保留：长链连续大量提交的断线/长期稳定性、血量死亡真人场景、真实room_id联机、Linux C++宿主、出口节点、QQ陪聊多人验收、实时视频声音。


### 0.10.8 真实验收补充
- 模拟器APK0.10.8实际连接山头服、OP通过；Python协议包指定订阅和全部订阅收到ChunkRadiusUpdated，服务器实际返回半径5。
- 原Lua game.sendCommand循环一万条：提交0.1038秒；含游戏分数核对0.7421秒，真实计分一万。只是单轮，不扩大为180000连续稳定或固定同Tick。
- 建筑导出实际失败：1x1x1结构读取8.65秒后超时；/structure save服务端成功，但三个请求类型均未观测到结构响应。尚未确定服务端拒绝原因，不能记为完整可用；APK会显示失败，不伪造导出文件。真实大区域导入/导出仍待验证。


0.10.8 已发布：Cloudflare部署294a0061-44f4-4498-beab-e01af717c1de；公网APK完整下载SHA256与本地一致，公网MCP单包/语言文档及真实进服状态通过。Linux服务active0.10.8；建筑结构导出失败待修，连续大链旧问题保留。
