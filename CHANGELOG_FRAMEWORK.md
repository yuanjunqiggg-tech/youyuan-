## 2026-10-06 · 0.10.11 当前新增任务

- [x] 定向跨账号 API：accounts.list/status/sendMessage/sendCommandsFast/callPlugin；Lua、Python、C++同宿主；设置授权/撤销。
- [x] MCP专用URL：独立凭证、到期和撤销、只读/管理、绑定云设备；不公开管理员总密钥。
- [ ] Grok客户端实际添加（服务端协议回归先做，不伪称Grok界面实测）。
- [x] 查明既有管理员“幽怨qi”，保留密码和权限；新管理员youyuan_admin已创建并实测登录，分配linux-server；原所有者密码不重置。
- [x] 全部可访问本地Codex聊天、源码/Git/程序/数据/私有配置已采集，五种恢复指南与脚本已完成；整合ZIP及逐文件校验以最终迁移包报告为准。
- [x] 本次25项独立进程/MCP验收、7项Worker回归、26个Skill ZIP导出回归通过；Grok客户端自身未实测。
- [x] APK公网下载原生串流/分段续传修复，Windows与Linux完整下载SHA256一致；管理员、专用MCP权限/撤销已公网验证。
- [x] 继续暂停新的游戏压力测试和原版倍率测试，历史失败保留。

# 0.10.10（2026-10-06）

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


# 0.10.9（2026-10-05）

常驻框架继续补全：统一宿主调度、防重入与取消；Fast整链、Async票据；Storage缓存多键事务；TXT监听；死亡与击杀者已知证据；建筑SubChunk真实读取及mcworld范围修复；完整三语言手册与25份按需技能、多选保存；页面异步生命周期/返回修复；注册外部协议转系统应用；Linux公网上传与本地资料库。全量未完成项目及历史压测失败见逐项核对。

# 框架变更与回滚记录

## 2026-10-03 源码保护与实施核对

- 原项目没有Git历史，保存已有源码及未发布0.10.7修改为9f90893c0e2085f1e6c22b6095593e83fcdec6e7；标签framework-existing-baseline-20261003。
- 合理采纳任务说明：兼容优先、分阶段提交、常驻复用、资源接口、调度诊断、按需AI文档、真实验收和交接。
- 不把建议中“每事件重建VM”当成当前事实；源码已经常驻。分类/风险/下一步见框架改造实施核对.md。
- 基线扫描排除构建、运行数据、私有配置，已知私有凭据未匹配源码；Git仅本地保存，未上传。

## Fast path and chain dispatch

Default no-wait ai uses SettingsCommand with bot context and original timer batching. Fast/Confirmed aliases and complete-array Fast host call added. TXT fast mode waits only at true condition barriers. Default CPS remains zero. Go tests pass; real 100/1000/10000 counters verified, repeated trials still disconnected. RakNet timeout margin and disconnect diagnostics added; sustained stability pending. No stable release claim.

## Resource and module preservation

Added shared plugin resource directories/read/list/exists/copy API. Ordinary Lua ZIP import and market export retain nested modules and binary resources, reject escaping paths and symlinks. Lua require searches plugin code root and retains module state in resident VM. Real binary/module round-trip and path traversal tests pass.

## Scheduler and diagnostics

Added whole-callback Hz/seconds/nominalTick scheduling, owner-scoped job lists/pause/resume, idempotent cancellation, SDK one outstanding timer callback, bounded Lua event delivery with secondary lazy-load compatibility. Added wall-clock/queue stats and platform capability discovery; unmapped CPU/memory/GC are null. Lua RPC avoids JSON; stack includes filename/handler/elapsed. Scheduler/queue/idempotent-cancel tests pass.

## Python persistent asyncio

One resident loop/thread per Host. Await async lifecycle/event/SDK callback/RPC results, background tasks run between events, optional async host HTTP wrappers, stop cancels tasks and closes loop/thread. Local Python regression confirms persistent loop, isolated accounts, async host invocation and complete stop. Android live validation follows in release checks.

## Final APK verification and result honesty

25x60 ordered fast chains verified at whole-chain20/40Hz; Android persistent async lifecycle/resources/ZIP state export and mobile AI/plugin layouts verified. 3000x60 fast stress disconnected and score17764, not passed. Added closed-connection recheck under send lock and explicit submission-only UI. Sustained large-burst stability and proposed Async/KV/watch/ABI remain open.


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
