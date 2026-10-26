# diagnostics / python-runtime

本章使用游园真实宿主绑定；先通过known/stale判断观测，不编造游戏完成状态。


### `diagnostics.combat`

target；返回保留事件中的伤害/血量/死亡/击杀，含同一event_id的补充证据。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `diagnostics.configure`

config?；宿主持久配置：event_queue/priority_queue/runtime_queue/async_tickets/event_history/actors各1..65536，stale_ms=1..86400000。事件队列变更在插件重载时生效，其余立即影响后续操作。不是命令CPS限速，也不是包种类上限。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `diagnostics.entity`

target；同entities.get，额外完整字段来源及过期信息。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `diagnostics.stats`

当前账号解码包计数、最后包时间、实体数、事件缓冲及配置、断线/踢出诊断和整个宿主内存/GC；非单插件内存。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `diagnostics.trace`

event_id或sequence字符串；查询保留的事件及关联死亡/受伤详情；被覆盖返回known=false，不虚构历史。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。
