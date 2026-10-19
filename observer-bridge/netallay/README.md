# NetAllay 服务端观测桥

这份插件安装在可管理插件的 **Allay 游戏服务器**，不是工具箱Linux进程，也不能仅凭山头服游戏OP安装。当前网易山头服尚未安装此模块。

已用 Maven Central 的 NetAllay 0.1.1 和 Allay API 0.28.0 真正编译为 Java 21 类；协议核心5项回归使用模拟Provider，完整服务端启动、事件与客户端往返仍待真实Allay环境验证。

安装步骤：

1. 准备 Java21 与 Allay 0.28.0，安装 NetAllay 0.1.1 及其实际依赖。
2. 将 `youyuan-netallay-observer-0.10.25.jar` 放入 Allay 插件目录。插件声明 NetAllay 依赖，不包含或替换 NetAllay 内核。
3. 首次启动会在插件数据目录创建 `observer.properties`。`allowed_uuids=` 默认空，没有任何观测者被授权。
4. 填入被允许的机器人真实 UUID，多项用逗号分隔，然后重启插件。连接必须是服务端已认证身份，不能按昵称随意授权。
5. 机器人连接这个服务器后调用 `observerBridge.rpcProbe(3000)`。只有收到匹配请求ID的 Capabilities 才说明接入成功。
6. 使用 `observerBridge.rpcQuery({query="player_online_list"},3000)` 或 `{query="player_health",target="真实玩家名"}`；事件通过 `observerBridge.rpcEvents(after,limit)` 读取。

转发范围：引擎健康变化、伤害、死亡、进服、退服。适配器向全部已授权观测者转发，不额外按距离或维度筛选。数据仍受实际游戏引擎、身份权限和事件触发条件约束；没有实现空挥事件的服务端监听，不能把这些事件宣称为全局空挥。

伤害事件的 attacker 只使用引擎给出的实体，未知返回 `attacker_known=false`。可取消事件标注 `stage`，`requested_current` 是事件中的计划血量；后续监听器可能改值。真实当前血量由只读Provider读取引擎实体，不把计划扣血算作已提交结果。

禁用时自动移除引擎及RPC监听，离服清除观测者。源码为本项目新增实现，SDK依赖与上游实现不内嵌入交付JAR。编译来源与SHA256记录在交付证据中；模块只提供读与事件，不暴露任意管理指令。
