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
