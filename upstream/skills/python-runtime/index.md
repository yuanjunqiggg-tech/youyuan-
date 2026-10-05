# Python 常驻插件开发手册

游园 0.10.9 / SDK 1.13。Python逻辑在CPython运行，通过host调用当前账号的Go游戏功能；不是把Python翻译成Lua。Android内置Python，Linux使用Python子进程；第三方Android扩展必须匹配对应ABI。

## 导入与入口

ZIP根包括manifest.json、main.py；可包含lib/、resources/和说明。

```json
{"id":"my_python","name":"Python游戏插件","language":"python","entry":"main.py","version":"1.0.0","events":["chat"],"dependencies":{}}
```

```python
count = 0

def on_init(host):
    host.game.registerTrigger([".你好"], hello)

def hello(name, args):
    global count
    count += 1
    host.game.sendTargeted(name, "§a常驻计数：%d" % count)

def on_stop(host):
    host.save({"count": count})
```

宿主在执行main.py前注入host及game/player/world等命名空间；推荐显式host.game。Python没有`import game`这个安装包。支持on_init(host)、on_event(event,host)、on_stop(host)及async版本。显式回调只收规定参数，不额外加host。数据是dict/list，列表索引0起；promptChoice返回的选项序号为兼容API仍从1起。错误为RuntimeError，可try/except；多返回值是list。

每插件常驻asyncio循环，不每次asyncio.run；耗时HTTP可await host.invoke_async("game.execute",cmd)。后台任务停止时取消，自己创建的线程需自行结束。Android同一账号共享GIL；不宣称计算速度提高1000倍。使用from .lib import helper避免插件同名模块冲突。标准库json/pathlib/asyncio可以直接导入；SDK不是ToolDelta、NeOmega、Minecraft官方Script API的包兼容层。

## 哪些功能可以组成原来的链

| 游戏目的 | 已有入口 | 合适的链步骤 | 结果核对 |
|---|---|---|---|
| 雪球菜单、连续计分、CD、选择器检测 | game.sendCommand / sendCommandsFast | 预读TXT，整轮按序连续提交 | 分数或菜单实际变化 |
| 上一条成功才继续 | game.execute / 有条件链 | 上一条和有条件节点设置确认点 | CommandOutput；超时未知 |
| 命令链选择与启停 | chains.menu / handleChat / start / stop | 事件触发已有独立链 | chains.get/list的状态 |
| 玩家挥臂或拾取物触发技能 | events.onSwing / onItemPickup | 收到真实事件后启动链 | source与runtime_id匹配 |
| 查玩家位置、背包或血量后分支 | game.getPlayerPos / queryInventory / player.health | 先查询或读取观测，再选择链 | unknown/stale/权限与返回值 |
| 建筑、声音、特效 | setblock/fill/playsound/particle指令 | 转成TXT命令再交同一执行器 | 区块、游戏声音和输出 |
| 真实挖掘、攻击、移动 | world.mineBlock/useBlock、bot.attack/startMovement | 插件动作回调；不能伪装成同刻原子命令 | 服务端位置、UpdateBlock、伤害等 |

TXT的0,0,0等是游园预解析语法，不发给游戏；第一位0连锁/1脉冲/2循环，第二位有无条件，第三位保留旧红石格式。`.`断开结果继承，`!`周期分界。具体控制语法读取command-chain/control-flow.md。Hz是**一整轮**目标频率；默认快速路径不逐条等回包、不每轮写文件、不重新解析TXT、不设置默认CPS等待。window是确认路径的在途数量，既不是一轮命令条数，也不是网络数据报个数。发送完成、服务端执行、同一个游戏Tick是三个不同指标；没有服务器批处理协议就不能承诺任意长链同Tick原子执行。

## 游戏连接与可观测范围

先连接对应服务器种类及其真实ID、room_id等，检查已连接与权限，再启用插件。原版游戏功能通过命令、服务器下发包、网易扩展和本地缓存组合实现，语言本身不会提高服务器权限。OP可用网易codebuilder_actorinfo查询背包，不能因此默认看到所有玩家的血量或输入。PlayerAuthInput通常是自己→服务器；旁观机器人不自动收到其他玩家的输入。RemoveActor是离开视野/移除，不足以确定死亡；Animate挥臂不能确定攻击命中或击杀者；UpdateBlock不能单独确定破坏者。没有真实数据时保留未知。

## 配置、资源、重载与完整导出

plugin.codeDir/dataDir/configDir/resourceDir/tempDir区分目录；Lua旧数据和配置路径保持兼容，SDK Python/C++数据按账号隔离。plugin.readResource读取原包resources，extractResource原字节复制到持久目录，**不是ZIP解压**。完整备份导出整个目录包含资源、配置和数据；发布到市场时排除私有数据。修改代码后先校验，再发布、热重载；重载不保留RAM，自己持久化所需状态。

## 文档与工具不是权限

本手册和每命名空间章节为离线Skill；AI默认使用工作区入口语言；玩家也能多选其他语言技能，按任务需要的包/函数读取小节，不加载全体包正文。工具是否允许调用由工作区配置决定；知道函数不代表自动获准执行。原始协议目录单独加载packet-reference，包含当前编译内核所有真实已注册包和字段；每包发送能否成功仍由服务器判断。当前血量/死亡及部分物理动作尚未完成所有真人场景验收；文档不将未测能力标成实测。

## API章节目录

- [game · 游戏命令与交互](game.md)：发送指令、定向消息、记分板、背包查询、NBT、聊天菜单与机器人动作。
- [player · 玩家资料与感知](player.md)：缓存、XUID、权限、背包、位置与已同步血量。未知血量不是20。
- [world · 方块与物理操作](world.md)：查询、setblock、真实挖掘和手持物品点击。权限替换方块与生存挖掘是两种机制。
- [events · 事件与抓包](events.md)：聊天、进退服、死亡、挥臂、实体出现与拾取，以及原始协议订阅。
- [packet · 原始协议包](packet.md)：schema/describe查询字段，on/onAll监听，send请求服务器执行；不会取得服务器未下发的数据。
- [bot · 机器人控制与路由](bot.md)：选择舰队机器人、状态、输入移动、看向坐标、近战攻击。无自动寻路保证。
- [chains · 命令链管理](chains.md)：同一套TXT预解析、整链频率、启停、跨链与聊天菜单，可供插件调用。
- [building · 建筑与颜色](building.md)：结构格式检测、解析和方块颜色匹配。这些解析函数不等于放置整个建筑。
- [media · 图片与音乐](media.md)：图片字符画、MIDI解析、音高换算；解析函数本身不自动向游戏播放。
- [plugin · 实例资源与诊断](plugin.md)：目录、资源、能力与统计；代码、配置、持久数据分开管理。
- [util · 文件、时间与调度](util.md)：JSON文件、文本、数学、时间与整个回调的定时任务。
- [api · 跨插件调用](api.md)：通过plugin_api导出函数；Lua、Python、C++可相互调用已启动实例。
- [http · 网络与JSON](http.md)：返回status/body/headers，非2xx响应也可检查，不把远端错误吞掉。
- [system · Android设备](system.md)：系统桥、通知、震动、Shizuku；桌面支持范围与Android不同。
- [storage · 缓存与事务](storage.md)：内存键值、原子加分、自动持久化与条件事务。
- [web · 插件控制页面](web.md)：声明式页面、HTML资源、动作回调与事件推送，重载清理旧注册。

语法块是参数顺序参考；下方游戏用例为完整工作流。章节包含当前真实注册方法，不用内部Go函数名冒充插件API。


# 常驻内核、命令缓冲与资源重载

这是游园0.10.9实际框架接口。入口语言由工作区/manifest决定，三种语言手册都可以多选；允许读取技能不等于给工具权限。正文按章节读取，没有自动联网动作。

## 常驻与模块

Lua加载/编译main.lua一次，事件调用现存函数，全局/闭包/package.loaded保留。require("modules.death")查插件代码目录下modules/death.lua；模块return一个表，在同一Lua实例内缓存。Python是每插件长驻模块和asyncio Loop。C++是长驻对象。重载整个插件时内存会重建，应把业务数据放Storage。旧语法保持兼容，不回退到每事件重跑源码。

```lua
local death=require("modules.death")
events.onPlayerDeath("died")
function died(e) death.record(e) end
-- modules/death.lua
-- return {record=function(e) storage.atomicAdd("deaths:"..e.player,1) end}
```

## 统一时钟与防重入

util.scheduleTick(1,handler)每个名义宿主Tick，Tick=50ms；scheduleTick(20,handler)每秒。scheduleHz(20,handler)、scheduleHz(10,handler)、scheduleHz(1,handler)使用同一调度Loop。Lua可传函数或全局名称；Python可传函数；C++先用on注册再传名称。当前策略skip：一个回调还没执行完，就跳过后续到期并累计missed，不补排队。不是服务器同Tick确认，调度器不能把远端服务器的50ms延迟消除。

```lua
local n=0
local job=util.scheduleTick(1,function() n=n+1 end)
-- util.listJobs()有calls/missed/last_ms/max_ms/total_ms/last_error
-- util.pauseJob(job); util.resumeJob(job); util.cancel(job)
```

## 三条命令路径

game.sendCommand或sendCommandFast：只发送，默认魔法SettingsCommand，不逐条UUID/回包/日志。sendCommandAsync：返回ticket，后台等CommandOutput；game.commandResult(ticket,0,true)读取完成结果并consume。sendCommandConfirmed/game.execute：当前回调等待真实结果。普通高频FUNCTION用Fast；仅有成功条件/查询结果才用确认。异步ticket最多128个未消费项是可见内存保护，Fast不经过这项限制。

```lua
local b=CommandBuffer()
for i=1,10000 do b:add("scoreboard players add @s ybench 1") end
local r=b:submitFast() -- 一次宿主边界，整数组按序提交；没有1024行限制
-- r.submitted是宿主接收数量，必须再查真实分数，不代表同游戏Tick执行。
local ticket=game.sendCommandAsync("scoreboard players list @s").ticket
-- 下一次回调查询：game.commandResult(ticket,0,true)
```

Python用host.command_buffer().add(cmd).submitFast()；C++用yy::CommandBuffer b(client); b.add(cmd); b.submitFast();。三种语言都可以直接sendCommandsFast命令数组。Lua缓冲在Lua表中构建，Python在list中构建，C++在json数组中构建；提交一次。Python/C++桥仍会对这一整个数组编码JSON，没有声称零拷贝/全原生ABI。

## FUNCTION IR与文件监听

TXT保存后parse成节点/控制点/行号IR，常驻调用，运行时不读TXT、不解析字符串。1,0,0脉冲；2,0,0 20为整个TXT实例20Hz；0,0,0连锁；0,1,0有条件；.独立链断开成功状态；!结束周期。未重新声明的属性继承。一个TXT实例只有一个整轮Hz；不同频率建独立TXT。第三位红石保留语法兼容，不在宿主做游戏红石门控。run_count=0持续，正数按完整文件轮次计数。

固定目录插件数据文件/ds_ai_agent/command_txt/。外部改TXT后，宿主每秒检查mtime/size，只有修改才读文件重新编译；无效修改保留原IR和报错。chains.reload(name)可立即资源重载，当前轮旧IR、下轮新IR，计数和启停保留。修改config/module仍需相应实例重载；完整reloadConfig/reloadModule尚未实现，不能假装只改一份Lua就不重启。

## Storage与事务

storage.get/set/atomicAdd/delete是插件数据目录中的内存缓存。后台约1秒把dirty数据原子写storage.json。flush同步持久化；进程被杀前还没flush的最近修改可能丢失，不能把自动保存当每次提交都已落盘。

```lua
storage.atomicAdd("points",1)
storage.transaction({
 {op="check",key="points",value=100},
 {op="add",key="points",value=-20},
 {op="set",key="paid",value=true}
})
storage.flush()
```

transaction先验证所有操作，在同一锁内复制/更新，再同步写成功后替换缓存；check失败不写入。支持set/add/delete/check数组，Python用dict/list，C++用yy::json。这不是任意语言函数事务，也不是跨文件/跨插件分布式事务。

## 性能观察与未完成部分

plugin.stats、util.listJobs和chains.list有实际调用数、耗时、missed、丢弃、队列和整轮统计。耗时是墙钟，不是CPU占用；逐插件CPU/内存/GC尚未归因，返回null。高速发送不逐条写命令日志。极端重复大链可能让RakNet/服务器承受不住，应实测最终计分；本地提交完成不等于服务端完成。可选每Tick命令预算/overflow carry、完整混合模块manifest、Linux C++适配器和native进程崩溃隔离尚未交付。


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
