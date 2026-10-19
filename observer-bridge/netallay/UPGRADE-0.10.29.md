# NetAllay观测桥 0.10.29

新包针对 **Allay 0.14.0 / API 0.29.0、Java21、NetAllay 0.1.1** 编译。API 0.28.0没有PlayerSwingArmEvent，不安装此新版JAR到旧API。当前已经用发布的0.29.0实际JAR编译，尚未在真实Allay服务器启动验收。

保留原本的observer.properties授权名单、只读查询、监听器清理机制。新增真实 `PlayerSwingArmEvent` → `swing`；`PlayerPunchAirEvent` → `punch_air`，两者分开，挥臂不能等同于空挥。使用引擎明确给出的玩家UUID，不选附近玩家冒充来源。转发给全部完成握手的授权机器人，不按距离或维度挑选。

血量变化不再把requested_current当成已生效血量。取消事件之后不转发；在实体所在维度线程的后续调度读取EntityLiving的真实Health/MaxHealth，再发送结构化health.current/max/previous。快照可能包含同刻其他变化，标注measurement和event_observed_at，不承诺逐击伤害精确差值。

安装流程沿用README的名单与握手步骤，但使用新版JAR、上述真实服务器/API版本。放进游园Linux工具箱本身不会替代游戏服务器引擎。游戏OP不能通过这个JAR替网易山头服安装引擎模块。

对应机器人接口：events.onServerObservation、events.onGlobalSwing、events.onGlobalHealthChange、observerBridge.serverHistory、observerBridge.serverSnapshot。Lua/Python/C++使用同一数据结构，来源为server_module_reported；原生UpdateAttributes缓存保持独立。

定义依据：[Allay正式发布说明](https://github.com/AllayMC/Allay/releases/tag/0.14.0)、[PlayerSwingArmEvent](https://github.com/AllayMC/Allay/blob/master/api/src/main/java/org/allaymc/api/eventbus/event/player/PlayerSwingArmEvent.java)、[引擎事件列表](https://docs.allaymc.org/advanced/event-list/)。
