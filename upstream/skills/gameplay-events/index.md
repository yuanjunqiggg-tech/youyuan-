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
