# entity / python-runtime

本章使用游园真实宿主绑定；先通过known/stale判断观测，不编造游戏完成状态。


### `entity.health`

target=self或runtime_id字符串；current/max/previous/damage/known/max_known/stale/updated_at/source。最大生命未知为null，绝不假定20。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `entity.onDamage`

同events.onEntityHurt；血量下降确认伤害，只有ActorEvent时数量未知，不能由最近玩家或挥臂猜攻击者。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。
