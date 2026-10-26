# 游园 0.10.23：真实观测 API、事件与容量配置

本轮修改宿主，不要求插件自己发同步查询来拼血量、装备或视角。协议数据只覆盖服务器确实给当前机器人发送的范围。以下“已实现”指源代码及相应测试；线上观测结果另列于验收文件，不能把接口存在写成已完成全服实测。

## 容量不是包种类，也不是命令频率

原来的128对应Lua普通回调队列和未读取异步命令ticket的默认容量。Python/C++回调队列原为256。协议目录从内核动态生成，没有“只能解析128种包”的限制。

在设置 → 实时感知与抓包 → 观测工作台，可以用数值输入框和保存按钮调整。高级调用：

```lua
diagnostics.configure({
  event_queue=2048, priority_queue=2048, runtime_queue=2048,
  async_tickets=4096, event_history=4096, actors=4096,
  stale_ms=15000
})
```

容量字段各为1～65536，过期阈值单位毫秒。保存到宿主数据目录`observation-limits.json`。队列容量在插件重载后生效；其他字段影响后续操作。默认值保持兼容：128 / 128 / 256 / 128 / 512 / 2048，过期15000ms。大容量只增大留存空间，不增加服务器广播范围，也不人为限制Fast命令CPS。异步结果读完应使用`game.commandResult(ticket,0,true)`消费，避免长期积压。

原始包记录单独配置：

```lua
events.captureStart({types={"UpdateAttributes","ActorEvent","MobEquipment",
  "MobArmourEquipment","MovePlayer","MoveActorDelta"},
  capacity=8192,max_bytes=8192,total_bytes=32*1024*1024,
  filter={every_ms=0}})
```

`capacity`和单次读取`limit`最高65536。默认留存序列化预算16MiB，最高256MiB；实际Go对象内存还包含运行时开销，因此预算不称为物理内存上限。环形缓冲按时间保留最近记录，覆盖旧记录时统计`overwritten`；不会每收到一包搬动全部记录。握手认证包继续排除，`*`默认不包含大区块包；可以明确选择正常大包类型。包类型筛选不再只允许32个名称。

## 接口与真实来源

| 功能 | 新接口 | 数据来源 / 边界 |
|---|---|---|
| 实体状态 | `entities.get(id)`、`diagnostics.entity(id)` | 当前会话实体身份、坐标、属性、装备、视角；移除及旧维度数据标记过期 |
| 血量 | `entity.health(id)`、`entities.health(id)` | `UpdateAttributes`的`minecraft:health`及`SetHealth`；current/max/previous/damage/known/max_known/stale/source |
| 属性及护甲点 | `entities.attributes(id)` | 原样保存同步属性；护甲点只取真实`minecraft:armor`，不凭穿戴算出一个假的护甲值 |
| 装备 | `player.equipment(name)` | `MobEquipment`、`MobArmourEquipment`、机器人自身Inventory包；ItemRegistry及连接GameData.Items翻译网络ID |
| 背包 | `player.inventory("self",windowId)` | 机器人实际收到的`InventoryContent`和`InventorySlot`；他人的完整私有背包未知 |
| 视角 | `player.rotation(name)` | `AddPlayer/AddActor`、`MovePlayer`、`MoveActorAbsolute/Delta`真实角度；仅旋转也更新，各角度单独known/time/source |
| 附近实体 | `entities.nearby({x,y,z},radius,dimension)` | 只筛选已同步、存在、同维度且位置新鲜的实体；无全世界扫描 |
| 受伤 | `events.onEntityHurt`、`entity.onDamage`、旧`events.onHurt` | 优先实际掉血，ActorEvent提示等50ms关联属性包；无属性时仍提供damage_known=false的真实受伤信号 |
| 击杀 | `events.onPlayerKill` | 服务器死亡翻译消息关联已观测玩家，唯一死亡event_id；重复详情不会重复加分 |
| 死亡及重生 | `events.onPlayerDeath/onPlayerRespawn` | ActorEvent、血量归零、网易已知OnPlayerDie、RespawnReady；重生独立于进服 |
| 进退服及维度 | `events.onSessionJoin/onSessionLeave/onDimensionChange` | PlayerList增删与ChangeDimension；视距出现/消失不冒充进退服 |
| 屏幕频道 | `hud.set/update/remove` | 按玩家、title/subtitle/actionbar类别仲裁，优先级和到期回退；只管理走HUD接口的内容 |
| 计分板 | `scoreboard.get/set/increment` | get取实际SetScore缓存；set/increment提交原生命令，提交不等于服务器成功 |
| 调试 | `diagnostics.trace/combat/stats`、`plugin.stats` | 包数量、来源、过期、事件编号、排队/回调墙钟耗时、丢弃、全宿主内存/GC、断线原因 |
| 时间 | `util.monotonicMs()`、`util.serverTick()` | 单调毫秒；serverTick只有真实非零UpdateAttributes tick才known=true。本地scheduleTick不冒充服务器同步Tick |

64位RuntimeID、UniqueID使用十进制字符串，避免Lua/JSON浮点精度丢失。旧`bot.self()`、`player.health()`仍保留平面health/max_health字段；新结构在`health_state`中。`entities.get()`的health直接是结构，不能混用两个字段形态。

装备每个槽返回`known/item_id/item_id_known/network_id/enchantments/enchantments_known/received_at/age_ms/stale`。已明确收到空物品才返回minecraft:air；未收到槽位不假定空。NBT没有附魔字段时，无法断定无附魔。装备包不含物品有效期，因此`expires_at=null,expires_at_known=false`；更新时间与失效时间不是同一个概念。

## 插件示例

```lua
events.onEntityHurt(function(e)
  local h=entities.health(e.target_runtime_id)
  if not h.known or h.stale then return end
  if e.attacker_known and e.attacker and e.attacker.name then
    hud.update(e.attacker.name,"combat",{text=string.format("%s：%.1f / %s",
      e.target.name~="" and e.target.name or e.target.entity_type,
      h.current,tostring(h.max or "未知")),priority=100,duration_ms=2000})
  end
end)

events.onPlayerKill(function(e)
  -- 宿主会去重，自己的业务也可保存event_id抵抗插件重启后的重复消费。
  local key="kill:"..e.event_id
  if not storage.get(key) then
    storage.set(key,true)
    scoreboard.increment("kills",e.killer,1)
  end
end)
```

没有可靠攻击者证据时，当前`attacker_known=false`，示例不会向附近所有人显示，也不会猜最近玩家。`Animate`只是挥臂。普通`ActorEvent`没有攻击者ID；不能据此承诺“任何人打任何实体都百分之百识别”。服务器死亡翻译消息可以提供击杀归属，与每次普通攻击归属是两回事。

Python使用`host.entities.health('5332')`、`host.events.onEntityHurt(callback)`等同名接口。C++使用`host.entities.health("5332")`和已注册回调名。SDK目录与MCP真实宿主API目录来自同一注册表。Linux目前原生C++运行器仍未开放；C++头文件提供绑定不代表Linux加载native插件已验收。Lua与Python可按实际平台运行。

## 事件与队列

Lua受伤、血量、死亡、击杀、重生、进退服、维度、装备事件有独立优先队列和消费者。普通事件另排队；网络读包只做缓存和投递。共享Lua状态仍由VM锁保护，纯CPU回调不能真正同时执行；宿主等待I/O的协作释放允许其他回调继续。Python/C++队列容量也能配置，但本轮未把它们的原生运行器重写为多线程战斗队列，不能宣称三个语言已做到完全一样的隔离。

`plugin.stats()`新增priority_queue_depth/capacity和last/max_queue_wait_ms。回调耗时仍是墙钟，不冒充CPU；`diagnostics.stats()`的heap/GC是全宿主数据，不冒充单插件分配。50ms受伤关联是事件证据合并窗口，不是命令发送延迟，也不会给循环命令链增加逐条回包等待。

## 验收与尚未完成

已用真实协议类型构造测试：非20最大血量、伤前/伤后、未知最大值、NaN拒绝、旋转无移动、延迟物品注册表、真实附魔NBT、ID复用、死亡后离开视距仍关联击杀、重复kill去重、重生去重、函数回调、原异步绑定、配置持久化、3000环形记录顺序、预算覆盖、HUD优先恢复、SetScore缓存清除。完整测试和Linux结果保存在本轮输出。

仍待真实游戏配合验证：装备换装与俯仰变化的广播范围、其他玩家死亡翻译包、战斗HUD肉眼效果、长时间多插件混合负载。普通受伤的全覆盖攻击者识别取决于服务器是否提供可靠来源，当前没有伪造实现。自动重连已有宿主机制，本轮加入关闭/踢出/成功连接诊断；逐次重连失败的完整结构化报告和跨重启持久诊断仍待补齐。

以前分享对话入口已重新尝试读取，其中页面未返回完整正文／超时。本轮没有把未读取的内容当事实；以用户贴出的UpdateAttributes字段与当前实际内核协议定义核对实现。


---

# 协议到游戏功能的实际开发

先查询plugin.capabilities/packet.describe，再按需订阅。只使用本账号真正收到的包。优先高层事件，不每Tick抓所有包；原始包和高层事件可以组合，不能凭字段未知编造动作来源。

## 挥空气、死亡、移动与拾取

| 功能 | 已可调用的接口 | 包与实际边界 |
|---|---|---|
| 挥手/挥空气触发技能 | events.onSwing(handler) | AnimateActionSwingArm；不证明命中谁，旁观机器人需要服务器广播 |
| 玩家死亡/最后位置 | events.onPlayerDeath(handler) | ActorEventDeath/InstantDeath、真实血量归零、网易OnPlayerDie；last_position只有position_known=true时可用 |
| 全实体死亡 | events.onDeath(handler) | 去重；RemoveActor单独不当死亡 |
| 血量/受伤 | player.health、events.onHealthChange/onHurt | UpdateAttributes/SetHealth/ActorEvent；其他玩家未广播血量仍unknown |
| 玩家/机器人位置 | player.position、bot.self、events.onMove | MovePlayer/MoveActorAbsolute/Delta；只更新实际包含的轴 |
| 生物/掉落物出现 | events.onEntitySpawn | AddPlayer/AddActor/AddItemActor，视野进入不等于首次生成 |
| 掉落物拾取 | events.onItemPickup | TakeItemActor映射掉落物和拾取者，拾取者可能是生物 |
| 方块状态变化 | events.onBlockChange、world.blockState | UpdateBlock；不能只凭它推断破坏者 |
| 挖掘/放置/攻击/移动 | world.mineBlock/useBlock、bot.attack、bot.startMovement | 实际发送物理动作/交易/输入，服务器按距离、装备、模式和权限校验 |

死亡事件统一结构保留旧顶层字段，并增加event.player/type/data/timestamp；timestamp为毫秒Unix时间，time为RFC3339。data是同一事件数据的副本。原events.onChat(player,message)、events.subscribe(type,handler)的旧签名保留，不强改旧插件。onPlayerDeath只处理玩家；onDeath可含生物，注意entity_type。

```lua
function swung(e)
  if e.player~="" then
    storage.atomicAdd("swing:"..e.player,1)
    -- 可以按自己的权限规则调用chains.start("技能.txt")
  end
end
function died(e)
  storage.atomicAdd("death:"..e.player,1)
  if e.position_known then storage.set("lastdeath:"..e.player,{position=e.last_position,dimension=e.dimension,time=e.timestamp}) end
end
events.onSwing("swung")
events.onPlayerDeath("died")
```

```python
def on_init(host):
    def died(event):
        host.storage.atomicAdd('death:'+event['player'],1)
        if event.get('position_known'):
            host.storage.set('lastdeath:'+event['player'],event['last_position'])
    host.events.onPlayerDeath(died)
```

```cpp
// 放在yy::Plugin构造中：on接收args数组，args.at(0)是真实Event。
on("died",[this](const yy::json& args){
  const auto& e=args.at(0);
  client.storage.atomicAdd("death:"+e.at("player").get<std::string>(),1);
  return yy::json(true);
});
client.events.onPlayerDeath("died");
```

## 网易死亡RPC

精确识别Value=["ModEventS2C",[namespace,system,"OnPlayerDie",{pid,die}],...]
pid为玩家UniqueID，不是RuntimeID；die=true死亡，false重生。只在能映射已观测玩家时触发，不搜索RPC文本猜死亡。游戏是否下发这个事件仍需实际环境抓包验证，结构识别测试不冒充真人死亡实测。raw PyRpc仍可packet.on("PyRpc",...)查看，不能任意重放登录事件。

## 用AI形成操作闭环

AI先project_list/project_read读完整源码，skill_search查询“挥臂/死亡/物品”，load_skill按章节读，packet_schema(name="Animate")只读该包字段和用途。需要原始游戏观察时listen_packet/list_packets，先记录目标类型，不开全包长期高频日志。发包或命令后检查真实响应、位置、方块或记分板。project_write可以新建模块/入口，不止改名字；project_patch能精确替换行；project_validate后project_publish，并plugin_manage start/reload，terminal_log确认。

命令链工作区默认只提供文件、语法校验和链发布工具；Lua/Python/C++工作区可按权限提供游戏/抓包/MCP工具。多选语言技能只开放文档，没有把未授权工具一起开放。玩家可以在新消息中引导运行中的任务；工具失败继续原任务，文件修订与分叉保留。

## 原始包层覆盖更广的玩法

SetActorData：实际metadata状态；MobEquipment/ArmorEquipment：同步装备；InventoryContent/Slot：本机或授权容器背包；InventoryTransaction/ItemStackRequest：本机交互请求；MobEffect：收到的药水效果；LevelSoundEvent：同步声音；SetScore/ScoreboardObjective：计分板；PlayerList：在线资料和皮肤；ChangeDimension：维度；ClientboundMapItemData：地图数据；BossEvent：首领条；PyRpc：网易扩展事件。各包精确字段在packet-reference单包目录，不从别的版本猜ID。

并非所有这些包都有高层函数：玩家三维速度/效果缓存、GUI箱子菜单、特殊物品自动识别、完整world cache和AI寻路仍需开发与真实测试。当前schema登记215种，不等于另外AI提到的“300+包”全部有解析器；未知入站包可原始观察，但没有语义时必须标记unknown。


## 0.10.9 补充：击杀者、维护开关与Linux文件

自动维护创造/飞行在新配置中默认开启；设置 auto_creative=false 可用于受伤或死亡测试，已有用户明确关闭的值保留。权限检查自身不改变模式；维护只对机器人自身处理模式/飞行变更，不反复逐Tick发送维护命令。

击杀者来源是服务器 Text 翻译型死亡消息，必须是实际已观察到的玩家。event.killer_known=true 才能把 killer 当作确定的玩家。OnPlayerDie 只提供死亡/复活，不含可靠击杀者；如果它先到，events.onPlayerDeath 可能先得到未知击杀者，随后通过 events.subscribe("death_details","handler") 收到服务器的补充。events.onDeath或死亡记录查询保留相关字段；无死亡广播时不会编造击杀者。

上传/下载工具箱文件位于设置的“文件选择器与界面”，网页选择 device=linux-server 时保存在该Linux实例的 /var/lib/youyuan/用户上传。上传支持文本、源码、ZIP等，不自动执行或覆盖同名文件。Linux的自建资料库也保存在自己的服务器磁盘，不再把文件拆分写入Cloudflare Durable Objects；Cloudflare保留网页、身份与控制中继。网页控制工具箱应用，不是任意root终端。

自己的源码附件通过 project_list/project_read/project_search 读取；真正修改后依次校验、发布、重载，不能用文件名替代正文。命令链反复运行已编译IR，只有TXT被修改时重新读取和解析。
