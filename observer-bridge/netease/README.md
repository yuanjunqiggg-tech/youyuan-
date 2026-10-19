# 网易服务端观测桥 0.10.29

这份行为包需要由服主放进**实际游戏服务器的组件环境**。放进游园工具箱Linux插件目录不会启动网易引擎事件；游戏OP也不等于有组件上传权限。当前山头服尚未验证安装成功，握手无匹配回复。

在开发者工具/组件管理中导入 `YouyuanObserverBP`，打开其中的 `youyuan_observer/config.py`，把 `ALLOWED_PLAYER_IDS` 设置为允许接收数据的机器人的真实**服务端实体ID字符串**。从 `bot.self().unique_id` 读取，不使用玩家昵称或浮点数。初始列表为空；名单来自服务器文件，不允许客户端自行授权。

事件通信使用 `YouyuanBridge / ObserverV1`。服务端只接受引擎注入的 `__id__`，验证已配置名单；`Hello` 返回匹配请求ID的 `Capabilities`。机器人执行 `observerBridge.rpcProbe(3000)`，必须 `confirmed=true` 后才算真实联通。当前会话实体ID变化时需更新名单。

## 已实现事件

| 引擎来源 | 转发事件 | 字段与边界 |
|---|---|---|
| HealthChangeServerEvent | health_changed | entity_id，health.current/to，previous/from；最大血量通过引擎属性读取，未知为null |
| PlayerAttackEntityEvent | entity_attack | player_id、entity_id，真实攻击开始事件；此服务端事件的damage初值为0，不当作实际扣血 |
| PlayerDieEvent | player_death | 死者、引擎提供的攻击者、原因；不寻找最近玩家冒充击杀者 |
| MobDieEvent | entity_death | 实体死亡及攻击者；不把同一次玩家死亡的两个引擎事件重复计分 |
| DelServerPlayerEvent | 清理订阅 | 离开后停止该接收者 |

转发调用 `NotifyToMultiClients`，目的地只包含已授权且完成握手的机器人；不添加距离和维度筛选。不改原伤害、模式、位置或事件取消状态。当前没有经核实的网易服务端空挥事件，因此明确 `swing_supported=false`，不伪造该能力。

只读查询：`observerBridge.rpcQuery({query="entity_health",target="精确实体ID"},3000)`，以及 `player_online_list`（返回引擎实体ID列表）。`entity_health`成功后会同步health_snapshot事件，宿主独立缓存可用 `observerBridge.serverSnapshot(id)` 读取。

## 验收条件

真实引擎内启用组件 → 授权机器人 → 握手成功 → 真人在远处和其他维度操作 → 比较同一时间窗口的引擎日志与机器人回调。当前只完成官方接口核对及替身SDK契约回归，不以这些回归代替游戏实测。

定义依据：[网易实体事件](https://mc.163.com/dev/mcmanual/mc-dev/mcdocs/1-ModAPI/事件/实体.html)、[玩家事件](https://mc.163.com/dev/mcmanual/mc-dev/mcdocs/1-ModAPI/事件/玩家.html)、[事件通信和引擎注入身份](https://mc.163.com/dev/mcmanual/mc-dev/mcdocs/1-ModAPI/接口/通用/事件.html)、[实体属性](https://mc.163.com/dev/mcmanual/mc-dev/mcdocs/1-ModAPI/接口/实体/属性.html)。
