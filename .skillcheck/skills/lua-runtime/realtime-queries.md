# 0.10.44 实时回调与查询

事件和定时器共享常驻 Lua 状态；不要在每个 UpdateBlock、50ms 周期或战斗事件里同步查询服务器。保留原 game.getBlockTileRaw，新增不占用 Lua 回调的真实 WS 查询：

```lua
local pending
function inspect_tick()
  if pending then
    local reply=game.commandResult(pending,0,true)
    if reply.pending then return end
    pending=nil
    if reply.error then print(reply.error) else print(reply.result.block) end
  else
    pending=game.getBlockTileAsync(1,64,1).ticket
  end
end
util.scheduleTick(2,"inspect_tick")
```
仅查询已加载区块，不传送；错误不当成空气。ticket 与插件/账号绑定，完成后consume，卸载清理；同一业务可保持一项在途，避免不必要重复查询。该策略只管理方块查询，不给命令链Fast添加回包等待或CPS限速。

Python同名：`ticket=game.getBlockTileAsync(x,y,z)`；C++：`client.game.getBlockTileAsync(x,y,z)`，同样用commandResult取得真实结果。

周期公告或人数监控用 `game.sceneStats()`（Python相同；C++ client.game.sceneStats）。返回cached（含离开视距后保留记录）、present（当前存在）、fresh/active_actors（当前会话不陈旧）、packet_count、time；不是全服实体计数。需要详细状态时用entities.get(runtimeId)，按实际变化读取；game.scene保留完整快照，适合手动诊断而非高频遍历。

plugin.stats()/MCP /api/plugin/profiler?id=... 另有last_vm_wait_ms、max_vm_wait_ms、last_callback_error，区分入队等待、等待Lua VM和回调体耗时。max是自本次插件启动累计的最大值，不能拿旧峰值当当前延迟。CPU/单插件内存未测量时继续null。

没有因此扩大未广播数据覆盖；未收到远处血量仍未知。界面响应时间、收包到插件回调、命令提交到服务器确认是不同指标，验收分别记录。


# 0.10.45 观测协作、战斗与容器接口

## 宿主三语言共用接口

`world.containerState(x,y,z)`读取当前维度真实BlockActorData类型缓存：known/is_container/block_entity_id/source/timestamp/stale/dimension/position。不读取箱内物品，不做同步WS查询。UpdateBlock替换后旧容器类型立即未知；新的BlockActorData再次建立。`events.onContainerChange(handler)`为对应规范事件。变化不等于确证某个玩家放置，没有来源时cause_known=false。

原`game.getBlockTileRaw`保留，新`game.getBlockTileAsync`后台真实查询，commandResult(ticket,0,true)消费。适合没有容器类型包时的核验，不在回调里等待。Fast清除只是submitted，不等于服务器确认已清掉，须回读更新包/Confirmed查询验收。

`game.sceneStats()`只计数，不复制或新留存全部实体明细。`game.scene({present_only=true,entity_type="minecraft:player"})`是可选过滤；无参数完整旧接口保持兼容。查一只实体用entities.get(runtimeId)，不要为查一只血量遍历整个场景。

`observerBridge.worldEvents({stream="actions",cursors=previous,limit=1024})`从独立动作历史读取：普通实体位移和非玩家旋转不会挤掉伤害/血量/挥臂等。limit可1..4096，history容量继续由diagnostics.configure(event_history)设置。默认stream=all保持旧行为。actions的lost表示发生覆盖（lost_count_known=false），不能拿全局sequence的位移间隙计算动作丢失数量。cursors按观察账号和会话保存，重连reset；没有广播的包仍未知。

`plugin.call(pluginID,exportedFunction,args数组)`只调用本账号已运行插件plugin_api的导出函数。未运行不隐式启动，私有全局不开放；忙/递归返回明确错误。Lua/Python同名，C++为client.plugin.call。MCP/HTTP：POST /api/youyuan/game/call {method:"plugin.call",args:["插件ID","函数名",[参数]]}。跨账号继续用原accounts.callPlugin/accountsPool.invoke，隔离与模块通信保留。

## 本次多功能插件的可调用接口（需要模块observation_team.lua）

```lua
local config={enabled=true,accounts={"自己的池账号ID"},targets={"目标玩家名"},
 mode="hybrid",interval_seconds=0.2,camera_height_blocks=4,camera_pitch_degrees=90,
 follow_radius_blocks=16,dwell_seconds=2,idle_seconds=60,event_limit=1024,start_accounts=true}
plugin_api.observationTeamConfigure(config)
plugin_api.observationTeamStatus()
plugin_api.observationTeamStop()
-- 空闲机器人先到位、收到新区域发布，才释放旧观察者。
plugin_api.observationTeamHandoff(incomingID,outgoingID,{player="目标",mode="hybrid",
 camera_height_blocks=4,camera_pitch_degrees=90,interval_seconds=0.2,timeout_seconds=5})
```

同一账号持续跟随一个目标，人数超过观察者数量时分时轮巡，切换错峰；有空闲接替者时用真实handoff，超时保持旧观察。账号不足/无备用时存在覆盖空档，不承诺每0.05秒全服全部玩家都收到，也不承诺不丢包。相机只是当前维度镜头；hybrid近距身体跟随可以跨维度，需要正常游戏权限。玩家不会被移动；真实测试应明确观察者/发送者OP状态与测量中是否TP。静默超时release按用户自己的池回收，不删除游戏凭据。凭据失效要真实重新认证，不能靠界面伪装成功。

动作汇总只接受已连接观察者的server_to_bot_decoded事件，按服务器/维度/RuntimeID分组，不把他人账号输入假装普通玩家广播。组合方法扩展视距覆盖范围，但不是全局引擎监听。

## 战斗显示语义

18格内近期挥臂或已知射击活动与真实掉血相关联；多名候选不强行取最近玩家。单候选也是关联推断，不能当PvP击杀确证。只有匹配玩家显示，取消向256格旁观者群发。血量和最大值来自属性；护甲未知显示未知，不用未知装备推算真实护甲。一个combat-health管理频道，1200ms到期清除/恢复低优先级频道，不反复播放Title。相同血量的重复原始/规范事件合并；重新回血再受伤仍可显示。

真实屏幕延迟、多人跨维度丢包率和连续放置回读仍要单独验收；源码模拟通过不等于游戏验收。界面保存区不浮动；导航透镜不再采样背景文字，浏览器控制超时仍需视觉复验。


# 0.10.50 收包与常驻运行性能

优化保持原始协议字段与旧插件调用形式。packetToMap缓存固定的字段索引与snake_case名，仍生成独立结果，完整转换二进制hex/数组/NBT；不会截短订阅包。订阅列表仅在增删时发布不可变快照，分发不再每包复制列表或争抢注册锁；同次分发的回调顺序及退订语义保持。协议目录所有构造包字段对照旧转换器，带数据的移动/血量及大ID对照通过。

同一台Linux、同一组fixture、单核、各3次：0.10.49移动包转换中位4846ns，当前1681ns（2.88倍）；血量包8082ns到3919ns（2.06倍）；订阅分发5173ns到1934ns（2.67倍）。这是解码后的宿主转换/分发微基准，不是全游戏或网络RTT倍率。移动车体状态缓存约292ns保持；不会把未变化指标包装成加速。

保留game.getScore、getPlayerPosXYZ、getPlayerItem、getBlockTile、getBlockTileRaw、getTickingAreaList、getBlockData/getBlockNBT/getBlockState/getRegionBlockData及placeNBTBlock/setBlockState的同步返回/错误；等待Go网络/文件/工作台时释放常驻Lua锁，其他回调和定时器可继续运行。Lua输入先复制为Go值，返回后再拿锁写Lua；不在两个Go线程同时执行同一份Lua指令，不重跑源码或丢弃模块状态。旧查询实际耗时仍可能包含网络等待。

区域读取、NBT放置与状态修改也使用按账号空间作业租约，避免观察跟随抢占操作位置；参与作业的内部传送保留，不拦成空功能。初始化阶段保留原初始化顺序，不能将回调期协作等待承诺成初始化所有网络I/O都不等待。

plugin.stats()/MCP /api/plugin/profiler增加io_waiting、io_calls、io_total_wait_ms、io_max_wait_ms，区分网络等待与执行墙钟。total/max为本实例生命周期统计，io_total可能包含多个并行等待，不是CPU时间；cpu_ms等未测字段继续null。Python/C++通过同一宿主接收转换/订阅路径获益；本轮Lua旧同步调用锁释放修复不表示Python/C++单语言性能达到了同样倍率。

性能验收：原0.10.49真实game.getRegionBlockData一次1130.55ms，10ms心跳在这段时间运行0次；新版本需要以同样接口真实复测并保存证据。另做连接/确认RTT/队列丢弃样本，不能只用本地空计数循环判断真实服务端完成。

没有改变握手协议ID、加密方式、服务端权限和条件链结果依赖。客户端优化不能使服务端没有广播的包自行出现；当前成果不宣称整体200倍或2000倍，也不通过删字段、停功能或降低事件覆盖率换数据。

方法依据：Go官方性能诊断与CPU profile https://go.dev/doc/diagnostics；fixture CPU样本显示旧snakeCase累计约27.65%，因此优先消除反复转换和分配，而非猜测网络协议。


## 0.10.51：显示通道与页面刷新

`diagnostics.stats()` 增加 `display_logs`，作用范围是整个宿主的显示日志：
`submitted` 为送往显示通道的行数，`dropped` 为显示队列满时无法投递的行数，
`policy="nonblocking_display"`。这些数值不是游戏收包或插件事件丢弃统计。
高频业务不能等待终端消费；本次移除64处日志阻塞写入，保留游戏包与命令发送路径。
命令提交成功与服务端执行成功仍分别判断；没有增加命令CPS限制、逐条等待或更改链Hz语义。

前端同账号同URL的并发GET共用当前请求；请求结束不缓存，POST每次独立执行。
插件管理以完整快照一次更新，旧刷新或旧账号结果不得覆盖新页。
终端最多保留最近2000条显示记录，合并到浏览器绘制帧，不会减少游戏事件订阅。
界面历史显示记录与完整的插件数据/导出文件是不同途径。

Lua 使用 `diagnostics.stats()`；Python 使用 `host.diagnostics.stats()`；
C++ 使用现有 SDK 的 `client.diagnostics.stats()`，或 `client.call("diagnostics.stats")`。
字段含义一致，日志拥塞指标是全宿主统计，不能当作单插件CPU或网络RTT。


### 0.10.53 设置页请求与轮询归属
设置页的配置、QQ和远控读取返回后先核对页面代次、账号与登录会话、当前可见页。离开后不启动轮询、不回写旧配置；未完成初始化会在下次进入时重读。设置元数据轮询仅在当前页可见时工作，页面退出取消自己的计时器。这不会停止机器人重连、后台插件事件、Tick和游戏收包。
