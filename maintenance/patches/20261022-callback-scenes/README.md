# 回调积压修复维护入口

仅维护此目录的三个模块与对应原生回调通道，不覆盖并行工作流的插件、UI或账号修改。

- `entity_hurt_info.lua`：投射物分支不再复制整个场景；实际玩家列表逐个使用player.position补位置/runtime_id，单实体伤害快照按runtime_id查询。
- `health_display.lua`：缓存按实体ID查询，不每次遍历历史全景。
- `yy_view_jitter_probe.lua`：仅请求present玩家作身份映射。
- 原生Text/聊天回调有独立有界FIFO，普通实体回调不再等待聊天外部I/O。三条固定队列不是每包创建无界协程；CPU密集Lua仍共享VM，后续聊天仍可能等待本通道前一个聊天回调。

原生源码：`upstream/youyuan_resident.go`、`lua_events.go`、`youyuan_profiler.go`；回归`youyuan_interaction_queue_test.go`。实际14:53部署SHA为`3781cf5cce066417e69e653e4a52386dbcb4d65d872b5b8ce3ed73f135a0fbc1`，不是build目录的旧产物。部署保留当时线上前端与其他代理源树更新。

在`project/upstream`运行真实Lua模块校验：

```powershell
go run ../maintenance/patches/20261022-callback-scenes/check.go ../maintenance/patches/20261022-callback-scenes/modules
```

模块替换前核实际文件SHA与manifest前值，原子替换保留UID/GID/权限；不重放旧阶段生成器覆盖新的定点查询修复。回滚分别见14:20、14:35、14:53的受保护备份，确切路径和SHA在外层outputs部署证据中。不要复制密码或控制令牌到补丁。

已验证：完整主包/building/state测试与vet通过；恢复观测后120.59秒处理额外69602次回调，丢弃增量0。启动已累计229次丢弃保留，不能说整个进程从未丢弃。该证据的雪球菜单当时停用，不代替菜单20Hz验收。
