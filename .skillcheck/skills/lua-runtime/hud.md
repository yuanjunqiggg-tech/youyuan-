# hud / lua-runtime

本章使用游园真实宿主绑定；先通过known/stale判断观测，不编造游戏完成状态。


### `hud.remove`

player,channel；删除本插件频道，下一调度恢复仍活跃频道；卸载插件自动清理。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `hud.set`

player,channel,{text,kind=actionbar/title/subtitle,priority=0,duration_ms=2000}；宿主统一频道仲裁，优先级最高的活跃频道显示，文本变化更新，过期恢复下一频道。默认不重播动画。仅管理经hud接口发送的内容，无法阻止绕开管理器的原始titleraw。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `hud.update`

player,channel,{text,kind=actionbar/title/subtitle,priority=0,duration_ms=2000}；宿主统一频道仲裁，优先级最高的活跃频道显示，文本变化更新，过期恢复下一频道。默认不重播动画。仅管理经hud接口发送的内容，无法阻止绕开管理器的原始titleraw。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。
