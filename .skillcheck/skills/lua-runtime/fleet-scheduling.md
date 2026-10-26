# 全局抓包调度（0.10.58）

这是宿主环境功能，插件无需复制调度实现。Lua、Python、C++调用同一真实API；外部MCP可调用`api_call`或`api_youyuan_account_pool`。

## 使用

Lua：
```lua
local state = accountsPool.captureStatus()
accountsPool.captureConfigure({enabled=true,accounts={"main","自己的账号ID"},auto_fleet=true,auto_provision=true,account_limit=0,auto_players=true,mode="camera",interval_seconds=0.016,pause_during_jobs=true})
```
Python：
```python
state = accountsPool.captureStatus()
accountsPool.captureConfigure({"enabled": True, "auto_fleet": True, "account_limit": 0})
```
C++：
```cpp
auto state = client.accountsPool.captureStatus();
client.accountsPool.captureConfigure(yy::json{{"enabled", true}, {"auto_fleet", true}, {"account_limit", 0}});
```
这些是低频控制面接口，不要每Tick保存配置或反复投放。先读取state，合并修改。账号ID取`state.candidates`，不是凭空编写。
MCP：POST `/api/youyuan/account-pool`，body `{"action":"captureStatus","args":[]}`；配置用`captureConfigure`、args第一项为patch对象。也可GET/POST `/api/youyuan/global-capture`。不是query包JSON，也不需要GitHub。

## 调度规则与参数

- `accounts`多选候选账号；本人账号和管理员明确`shared_capture=true`的共享采集号。共享标记仅开放采集调度，不开放令牌、文件、插件或任意命令；普通用户不能自行把别人账号设为共享。
- `auto_fleet`默认开启。真人1/2/3人对应1/2/3台；大于10人目标5台，大于等于15人目标6台。`account_limit=0`按此自动，非0可手动增加基础投入数量并限制最大数量；不会超出候选可用数。
- `dimension_accounts`中的0/1/2是优先维度；有真人的维度先保留观察机，空维度角色借调到负载大的维度，真人返回其维度时优先回防。
- 主名单统一排除全部受管机器人和主号。子账号旧名单不用于派发。真人维度通过一轮原生querytarget结果关联UUID/UniqueID；失败时标为`dimension_known=false`，不把缓存坐标或镜头位置当已确认维度。
- 每个观察任务只有一个定时循环。不同机器人分配不同真人主路线，并设置`phase_seconds`与`target_offset`错峰；不会为每一玩家另建一个无界线程。
- `auto_provision`自动连接缺额候选，失败15秒起退避至数分钟，尝试其他账号。成功以connected/OP/实际维度/旁观模式判断，不以保存Token或HTTP200判断。
- 新自动投放为专用`capture_only`，跳过业务插件自动加载、不打开可见控制台。独立游戏连接仍需自身进程和内存，不能宣称没有成本。
- 新采集号已连接却无OP时，合法OP主控会向已选受管账号授予OP；普通真人不会被授权。主控无OP时显示`permission_required`，不伪造就绪。
- 无真人时停止相机请求。仅专用采集号在`idle_seconds`（0默认60秒）后回收；现有带业务账号不被擅自关闭。
- `interval_seconds`是一次镜头提交间隔；不是服务器同步周期、更不是所有玩家无漏包保证。原生选择器跟随目标，包仍受服务器真实下发限制。
- 导入导出期间默认暂停镜头/跟随，恢复时不重启整个插件。
- `player_limit=0`默认22名真人；`enforce_player_limit=true`时拒绝第23名新加入者，机器人不计入；刚开启时不随机踢现有玩家，必须由OP主控实际确认kick结果。

## 结果与诊断

`plan`包含真人列表、可验证维度、排除机器人、目标观察数量、planned/active数量和分工。`states[account]`区分starting/cooldown、permission_required、waiting_dimension、observing、error；`assignment`显示目标/维度/错峰，`residency.ready`依据真实连接层维度和mode6。`submitted`只代表提交，不代表服务器执行完成或收包覆盖率。

认证400不是Minecraft墙碰撞事件：`/api/youyuan/connection-policy`显示`last_error_kind`、`failures_history`和`effective_retry_seconds`。凭据失效须重新登录；退避不能把失效凭据变成有效凭据。

## 验收边界

规则分配、授权边界、备份跨域、视频矩形与帧等待有自动化回归；真实多人/跨维度/连续血量包覆盖仍必须用新的时间窗口实测，不把算法测试写成真实无漏包。


## 0.10.60 查询回包验收
网易execute包裹querytarget可能返回success_count=1但不保留玩家details，因此维度发现使用原生querytarget @a，解析OutputMessages或DataSet.details，缺字段不猜测。`accountsPool.captureRoster({include_bots=true})`仅用于诊断当前受管机器人真实维度，不启动任何观察、不改变生产排除名单；正常省略include_bots，只读真人。Python使用True，C++使用yy::json。视频转换支持每批1..8帧append_batch，按sequence原子写入；失败回滚文件/计数，明确未知结果时不盲目重复。帧间隔1..60000ms是提交目标，不是实际播放承诺。


## 多台机器人与自定义人数阶梯（0.10.66）

协作并非双机器人专用。`accounts`是多选候选池；`plan`是本轮分工；`states`是真实连接和执行状态。当前宿主提供32个子进程槽位，不等于32台都已成功登录，也不是可无限增加的资源承诺。

可通过同一个 `accountsPool.captureConfigure` 设置：

```json
{"fleet_steps":[{"min_players":1,"observers":1},{"min_players":3,"observers":3},{"min_players":8,"observers":8},{"min_players":15,"observers":12}],"account_limit":0}
```

以上为参数对象，Lua用表、Python用字典、C++用`yy::json`传入。MCP使用原有captureConfigure工具或game_call，不新增另一套接口。每行表示达到该真人数量时投入多少台，第一行必须从1人开始，人数递增不重复，机器人数量1..32。无人在线时始终停止观察。非空阶梯下account_limit只作上限；空阶梯保留旧默认规则和旧人工投入方式，不擅自改已有用户策略。

三个维度同时观察至少需要三台已就绪机器人；额外机器人按各维度负载借调、错峰。失效、被占用、其他服务器的账号不算就绪；只从有授权的候选池替补。导入导出期间不抢作业账号或改变其连接。规则分配测试覆盖24台/32台、候选不足、替补和三维度，不表示这些账号已在真服同时上线，也不证明全局无漏包。


## 0.10.67 常驻维度与观测证据修正

默认`resident_observers=3`（0亦使用默认3），基础三台分别驻主世界、下界、末地，空维度不再借走基础角色。额外机器人才按人数阶梯分担繁忙维度；账号不足明确shortfall，不伪造已常驻。`resident_observers`可设1..3，`account_limit`仍限制总投入。开启期间即使无人也保留基础常驻，但无目标不发镜头指令；关闭后专用采集号按原空闲时间回收。测试或其他任务借用专用采集号必须考虑此回收策略，测量时隔离并结束恢复。

全局镜头用世界向上偏移，高度默认6、俯视90度，不因目标低头而把本地坐标旋转到侧面。单镜头不能跨维度。查看`observerBridge.watchStatus`：submitted只表示命令提交；received_packet_counts是本连接会话的原始包类型计数（Animate还包含其他动作），last_received_packet_at是实际收到包的时间，均不能当成当前目标的准确覆盖率。

现有`events.onEntityHurt`、`events.onHealthChanged`、`events.onSwing`、`events.onBlockChange`已注册，三语言共用实际解码事件。`observerBridge.worldEvents`按需汇总各账号，保留来源/会话/游标和失败，不等于已自动将所有子账号事件注入主插件本地回调；不要混淆本地缓存与跨账号结果。没有任何账号收到远处事件时，不可能靠“收到事件再去看”补回该瞬时动作。

网页提示“Linux直连中断”表示临时HTTPS线路响应失败，写操作结果可能未知，不等于游戏进程已崩溃。配置保存可只读核对已保存值，不重放写入。普通API调用仍须遵守账号授权；不能因有MCP或宿主API便承诺服务器全维度广播。

0.10.67补充：普通用户固定 camera/15ms/世界向上6格/俯视90度/朝向0，自动全部真人、错峰、作业暂停和三维度常驻；管理员保留高级参数。机器人身份未确认时暂停增派，plan.robot_identities_verified/provision_hold_reason用于诊断。收到某个本地伤害包并不意味着全服伤害会广播。watchStatus.received_packet_counts是连接会话计数，不是捕获率或服务器确认。
游园导入导出面板显示真实计数、速度、估计剩余时间和SetTime采样TPS；未知TPS明确待采样。不要把提交进度当作全部方块已由服务器确认。

0.10.68：删除兼容Lua插件会先取消常驻VM和计时器/事件，再删除文件。加载未完成时拒绝交叉删除；文件删除失败明确保留error状态和可重试定义，不误报成功。不会给新用户安装本轮验收插件。
