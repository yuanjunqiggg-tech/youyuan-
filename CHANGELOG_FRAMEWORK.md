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
