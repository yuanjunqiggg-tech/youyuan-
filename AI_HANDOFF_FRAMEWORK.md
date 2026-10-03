# 游园框架 AI 交接

更新：2026-10-03。本文件不含令牌、密码、模型Key或签名Key。公开版本当前0.10.6，开发0.10.7尚未通过最终持续发送验收。详细任务状态见AI_PROJECT_STATE.json及任务备忘录.md。

目标：保留原工具箱Lua插件能力、旧API语义和用户数据，增加统一三语言API、常驻执行、AI项目工作区、MCP、公网连接和机器人内命令链；不得默认限速让旧插件性能退化。

架构：Go核心维护BotManager及网易网络连接；Android Java/JNI承载Go、Chaquopy Python及原生动态库；Linux使用Go服务及独立Python进程。前端HTML/JS由APK/Web共用。Cloudflare负责公网中继、独立认证网站和发布下载；游戏操作在设备/云服务器的实际机器人连接执行。

Lua：youyuan_resident.go按(bot指针,plugin id)保存LState，初始化执行源码一次，事件调用已有函数，锁保护VM。api.require调用常驻目标。secondary账号初次加载使用兼容注册抑制，不每触发重建。停用取消定时/订阅并关闭VM。旧legacy rerun并未对外提供manifest模式，不能声称支持。

Python：youyuan_plugins.py每实例模块与Host常驻，Go youyuan_runtime.go串行消费有界事件队列。Android解释器共进程/GIL，Linux每实例独立Python进程。当前awaitable回调尚待适配；不要声称asyncio已完整。C++：Android通过create_v2/event/destroy常驻实例，有C++异常边界；Linux适配器明确未接入，同进程SIGSEGV不能隔离。不要承诺1000倍性能。

命令：game.sendCommand及Fast默认ai无等待使用SettingsCommand并保持机器人@s/相对坐标；原网络层定时合并。旧源码PyRPC在当前测试服100条实际未执行，禁止默认恢复PyRPC或宣称原版APK就是该通道。Confirmed用AutomationPlayer/CommandRequest及UUID回包。sendCommandsFast完整数组一次宿主边界，无整数组行数限制。TXT fast模式仅条件节点及前条需要回包，普通命令不等待。提交不等于服务器成功。连续压测尚未完成，不能标稳定。

Scheduler：旧util.schedule采用Go ticker并同步调用常驻handler，同任务不重入；不同任务共享VM锁。现有packet回调还有产生等待goroutine的风险。新增调度必须有界、统计missed和暂停恢复。scheduleTick如果使用本地50ms必须明确名义Tick，不保证服务器同Tick。CommandBuffer应一次边界提交整数组；游戏批处理不保证原子或同一服务器Tick。

FUNCTION：youyuan_commands.go、youyuan_chain_buffered.go在保存时预解析，循环不重复解析文本。类型0连锁/1脉冲/2循环；条件依赖真实回包；红石位保留兼容但无意义门控已撤去。点结束独立链、!结束周期；@call目标文件行范围、次数、时限；@wait边沿；@pause及chains.trigger恢复。Hz按完整链轮次，period减本轮耗时，超期立即下一轮但不积压轮次。逐条delay_ms默认0。

Resource：当前旧game.dataDir及util文件API可用；正式plugin资源相对路径API已加入youyuan_resource.go；普通LuaZIP现在保留模块和资源，package.path搜索源码目录。ZIP须完整保留源码/资源/配置/数据；拒绝路径穿越。Storage：旧JSON磁盘state保持原语义，原子替换文件不是多键事务。Event bus已有chat/packet/sense/SDK事件与跨插件RPC，LState只在自己的锁/串行回调内访问。网络写有连接sendMu保护；插件错误边界不等于原生进程隔离。

线程模型：Lua每(bot,id)互斥执行，native/Python实例mu串行；SDK的256项队列满时计dropped。跨插件同步RPC用TryLock避免循环等待。慢HTTP可能占当前插件，不能阻塞整个网络读取；新增Async经受控队列回调。Stop必须清理新增定时器、异步请求、监听和订阅。

已有证据：3000×60=180000全部计分核对60.965秒，2952.514是单条命令/秒，约0.984整链轮/秒。2026-10-02一次10000分数核对0.8075秒后断连；2026-10-03一次10000核对1.3363秒后断连，均不能称持续稳定。原版用户报告一轮10000可行，但原版同条件计时未测到。不要篡改失败样本。内核Go测试、布局360/420/1440此前通过；完整APK最终版本尚未公开。

已实现：Lua/Python/Android C++常驻、预解析链、动态窗口、批处理、跨链API、完整项目AI读取/搜索/编辑/Diff/回滚/附件ZIP/会话/按需Skill、MCP、QQ真实互通、独立认证及自有令牌进服。未验收：可靠死亡/血量、多账号长稳、实际联机房间、群陪聊、设备出口节点、B站实时画面声音。暂缓：昼夜API、完整Entity对象、大规模ABI/进程沙箱、长期1h/6h压力。不要预置QA插件或测试Key给普通用户。

重要路径：upstream/bot_conn.go、command_dispatch.go、youyuan_plugin_commands.go、youyuan_command_stream.go、youyuan_commands.go、youyuan_resident.go、lua_util.go、lua_events.go、youyuan_runtime.go、youyuan_runtime_desktop.go、youyuan_sdk.go、android/app/src/main/python/youyuan_plugins.py、sdk/cpp/youyuan.hpp、skills/、frontend/js/pages/plugins.js。任务工作目录../work含构建及真实测试脚本，../outputs保留报告，project/private与外部私有凭据不进Git/发布包。

推荐基线：9f90893c0e2085f1e6c22b6095593e83fcdec6e7，标签framework-existing-baseline-20261003。这是既有源码保护点，含未发布修改，不是已验收稳定基线。近期提交见AI_PROJECT_STATE.json；不存在的20个历史提交不得编造。

回滚：git status；提交/保存工作后，git worktree add ../framework-review <commit>，在独立目录编译核对；APK必须同原签名证书覆盖安装，Linux更换程序保留/var/lib/youyuan数据。数据库/config回滚需单独备份。禁止硬重置、卸载APK、删除用户数据。构建脚本../work/rebuild_v0107.ps1；Go test . ./building ./config ./media ./state；压力脚本../work/snowball_10000.py仅私有测试计分，不影响玩家；不要直接运行测试ZIP中的@a、kill、tp、replaceitem。

示例旧兼容：Lua for i=1,10000 do game.sendCommand('scoreboard players add counter test 1') end。结果confirmed=false只表示已提交，必须独立游戏计分核验。条件链使用game.execute或executeBatch可确认路径，不用无等待返回值猜上一条成功。新增API及限制同步开发文档、当前语言Skill、外部MCP按需资源，正文不整库塞上下文。
