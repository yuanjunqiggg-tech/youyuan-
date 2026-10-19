# 0.10.23 真实观测与容量配置

新实体、装备、视角、背包、HUD、计分板和队列接口见本语言的entities.md/entity.md/diagnostics.md/hud.md/scoreboard.md及player.md/events.md/util.md补充。未知值不得编造，先用known/stale判断。

# C++ 常驻插件开发手册

游园 0.10.19 / SDK 1.15。原生动态库运行器支持 Android arm64-v8a/x86_64 与 Linux 对应架构的 ELF 库。Linux 每个插件在独立常驻子进程中加载一次；需要 Python3（仅作为 ctypes/IPC 桥）或设置 YY_PYTHON。业务函数执行 C++ 机器码。Windows 不能直接加载 ELF .so，应连接 Linux 执行端或使用 Lua/Python。游戏接口通过当前账号宿主执行，不等于 BDS 服务端插件。

## 文件与ABI

源码#include "youyuan.hpp"，自带third_party/json.hpp（nlohmann::json）。使用 C++17；Android 用对应 NDK，Linux 用对应架构 g++（例如 g++ -std=c++17 -O2 -shared -fPIC -I./sdk main.cpp -o libplugin.so）。Linux glibc 库不能直接放到 Android，APK 的库也不能直接放到 Linux。ZIP 内 manifest.json 加 libplugin.so：

```json
{"id":"my_cpp","name":"C++游戏插件","language":"native_cpp","entry":"libplugin.so","version":"1.0.0","events":["chat"],"dependencies":{}}
```

```cpp
#include "youyuan.hpp"
class Hello : public yy::Plugin {
  int count = 0;
public:
  Hello(const char* context, yy::HostAPI api):Plugin(context, api) {
    on("hello", [this](const yy::json& args) {
      client.game.sendTargeted(args.at(0), "常驻计数："+std::to_string(++count));
      return yy::json(count);
    });
    client.game.registerTrigger(yy::json::array({".你好"}), "hello");
  }
};
YY_PLUGIN(Hello)
```

宏导出yy_create_v2/yy_event/yy_destroy，构造参数context_json含account/plugin_id，不是旧账号字符串。on注册宿主回调处理器；exports通过expose注册跨插件函数。参数/返回值是yy::json：对象用{{"key",value}}，数组用yy::json::array({...})，读取用.at()/get<T>()，http.delete对应delete_。错误为std::runtime_error，在回调边界捕获。Linux 原生崩溃只退出该插件子进程，宿主随后停用该实例并清理定时器/订阅；正常业务异常不会重建对象。Android 当前仍在宿主进程加载，不能声称已实现同等级崩溃隔离。正常事件与定时调用复用同一个对象，stop销毁。不要传C++指针或文件句柄到JSON桥。

Linux 执行器等待一次 handler 的默认超时为 120 秒，可通过 YY_NATIVE_TIMEOUT=180s 修改；这是卡死回收机制，不是 CPS 或命令链长度限制。跨语言调用仍有 JSON/IPC 开销，不能据此承诺比 Lua 更快。高频命令使用 CommandBuffer.submitFast，一轮整体交给宿主，不为每个节点建立定时器。

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

plugin.stats、util.listJobs和chains.list有实际调用数、耗时、missed、丢弃、队列和整轮统计。耗时是墙钟，不是CPU占用；逐插件CPU/内存/GC尚未归因，返回null。高速发送不逐条写命令日志。极端重复大链可能让RakNet/服务器承受不住，应实测最终计分；本地提交完成不等于服务端完成。可选每Tick命令预算/overflow carry、完整混合模块manifest尚未交付。Linux 0.10.19 已接入常驻 C++ 子进程及崩溃/卡死回收；Android 原生仍在宿主进程中加载。


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


# 跨账号联通开发说明 · 0.10.11 / SDK1.14

这是同一个游园控制端内，主账号 main 与 managed-accounts 独立账号之间的协作 API。账号目录、令牌、插件和事件不自动混用；授权只开放指定操作。不是跨不同平台用户或另一台电脑的匿名接口。不同设备请使用已认证的远端 MCP 各自连接。

## 开放授权

设置 → 账号联通与连接授权 → 跨账号联通。选择源账号、目标账号、源插件 ID 与操作，添加并保存。源插件填 `*` 是该源账号全部插件，建议填写具体 ID。A→B 不自动授予 B→A；撤销后后续调用立即拒绝，正在执行的命令不会撤回。只允许运行中的目标账号，接口不会偷偷启动或替换账号。

规则 API：`GET /api/youyuan/accounts/bridge/rules` 查看；`POST` 同路径提交完整规则数组（覆盖替换，不是追加），例如：
```json
[{"source":"main","plugin":"my_lua","target":"account_b","operations":["status","sendMessage","sendCommandsFast","callPlugin"]}]
```

## 五个接口

| 方法 | 参数 | 行为 |
|---|---|---|
|accounts.list|无|账号 ID、名称、启用状态；不暴露令牌|
|accounts.status|target|实际目标连接状态；需 status 授权|
|accounts.sendMessage|target,text|目标机器人普通聊天；斜杠指令明确拒绝|
|accounts.sendCommandsFast|target,commands,options?|完整数组按序 Fast 提交；无逐条确认|
|accounts.callPlugin|target,pluginID,function,args数组|调用目标常驻 plugin_api 导出，返回结构化结果|

target 必须是 accounts.list 返回的 ID，不是显示昵称、端口或QQ号。options 复用 game.sendCommandsFast 的参数，包括 channel。数组提交数量不代表游戏执行成功；Fast 不保证同一服务器 Tick、事务原子性或网络失败重试。20秒跨账号控制请求超时后不得盲目重发有副作用操作。

### Lua
```lua
local profiles = accounts.list()
local state = accounts.status("account_b")
accounts.sendMessage("account_b", "联通测试")
accounts.sendCommandsFast("account_b", {
  "scoreboard players add @s yytest 1",
  "scoreboard players add @s yytest 1"
}, {channel="ai"})
local result = accounts.callPlugin("account_b", "target_plugin", "balance", {"Player"})
```
目标 Lua 插件只加载一次，导出例子：
```lua
plugin_api = plugin_api or {}
function plugin_api.balance(player) return {player=player,value=10} end
```

### Python
```python
profiles = accounts.list()
state = accounts.status("account_b")
accounts.sendCommandsFast("account_b", ["say A", "say B"], {"channel":"ai"})
result = accounts.callPlugin("account_b", "target_plugin", "balance", ["Player"])
```
Android 与 Linux 驻留宿主均提供 accounts 命名空间；异步插件请在自己的任务中 await host 提供的异步包装或 asyncio.to_thread，勿占住高频 Tick 回调等待远端返回。

### C++（Android / Linux 原生宿主）
```cpp
auto profiles = client.accounts.list();
auto state = client.accounts.status("account_b");
client.accounts.sendCommandsFast("account_b", yy::json::array({"say A","say B"}), yy::json{{"channel","ai"}});
auto result = client.accounts.callPlugin("account_b","target_plugin","balance",yy::json::array({"Player"}));
```
Linux 0.10.19 已接入独立常驻原生子进程；需要对应 Linux 架构的 ELF 库和 Python3 传输桥。不要把安卓版 .so 当成 Linux 插件。Windows 不能直接加载 ELF。

## 防止互相等待

跨账号 callPlugin 的导出回调里，禁止再同步调用 accounts.*，避免 A 等 B、B 又等 A 的环路。应先返回结果，再通过 schedule/setTimeout 发下一次协作。阻断在当前进程级生效；同一时段其他插件的跨账号调用可能被拒绝，应延后任务。拒绝信息可供 AI 查错，不重跑插件源码。

这是协调授权，不是对恶意本地代码的安全沙箱：本机插件本就有系统/HTTP权限。云端仍依赖平台用户和设备隔离；不能给其他订阅用户分配你的私有节点后又期望他们无法控制此节点。

## 远端与内置 AI

MCP api_youyuan_accounts_bridge_rules 管理规则；api_youyuan_accounts_bridge_execute 调用 `{source,plugin,target,operation,args}`，属于控制权限。内置 AI 可按需加载本技能，读取接口目录和错误；不能只知道插件名称。内部 `/bridge/local` 不列入 MCP，不允许通过账号代理嵌套绕过授权。

## 验证范围

代码回归验证定向授权、源插件限制、操作限制与接口注册；发布验收验证真实进程之间状态调用。没有在本轮新增游戏压测，发消息、游戏命令副作用及跨语言目标插件的游戏联调留待玩家批准恢复测试。


## 模块与跨插件通信

详见 [通信契约](communication.md)，包含三种语言的导出/调用、状态保留、跨账号权限、循环调用与崩溃清理。


# game · 游戏命令与交互

发送指令、定向消息、记分板、背包查询、NBT、聊天菜单与机器人动作。


### `game.commandResult`

ticket,wait_ms=0,consume=false；pending=true表示结果未到。completed=true附result/error；只可读本实例ticket，consume=true删除完成记录。超时不重发。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `game.dataDir`

```cpp
client.game.dataDir();
```

- 作用：返回本插件数据目录的完整路径，即 `插件数据文件/<id>/`。
- 参数：无。
- 返回：string。`__PLUGIN_ID__` 为空时返回空串。
- 报错：无。
- 底层：`pluginController.DataDir(id)`。

### `game.execute`

```cpp
client.game.execute(command, options);
```

- 作用：发送命令并等待真实CommandOutput。
- 参数：`command, options?`；按此顺序传入。
- 返回：对象：submitted/confirmed/success/success_count/output/error等；超时为结果未知。
- 底层与游戏效果：默认命令请求自动化来源，关联UUID回包。需要根据上一条成功决定下一条时使用；普通快速链避免逐条调用它。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.executeBatch`

```cpp
client.game.executeBatch(commands, options);
```

- 功能、参数、返回与约束：commands 字符串数组，options：channel=default/ai/player/console（或0/1/2字符串），mode=ordered/pipeline/submit/burst/stream，transport=command_request/settings/pyrpc（后两者仅submit，无逐条确认），window=1..16条在途命令；burst最多1024条整批协议发送，512KiB文本，默认无固定CPS等待，不保证同Tick，timeout_ms=100..10000。支持每实例默认配置与单次覆盖。ordered 逐条等待；pipeline 仅用于互不依赖的命令，不保证服务端完成顺序；submit 不等待结果。结果逐项含 submitted/confirmed/success/error/channel，超时未知不自动重发。stream单段最多4096条/512KiB，动态在途窗口最多1024，rate_cps=0无固定CPS，网络未知停止补发，正常无目标继续无条件后续；完整说明读 command-execution 技能。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.flyStart`

```cpp
client.game.flyStart();
```

- 作用：开始 / 停止飞行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `flyStart/flyStop: <err>`。
- 底层：把机器人切换为飞行/降落：优先走游戏接口的飞行控制（运动能力包），不可用时回退为发送带飞行标志的 `PlayerAuthInput` 包。

### `game.flyStop`

```cpp
client.game.flyStop();
```

- 作用：开始 / 停止飞行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `flyStart/flyStop: <err>`。
- 底层：把机器人切换为飞行/降落：优先走游戏接口的飞行控制（运动能力包），不可用时回退为发送带飞行标志的 `PlayerAuthInput` 包。

### `game.getBlockData`

```cpp
client.game.getBlockData(x, y, z);
```

- 作用：读某坐标方块的完整数据（名称 + 方块状态 + 方块实体 NBT）。读取命令方块指令、箱子物品、告示牌文字、旗帜图案等都从这里入手。会先把机器人传送过去加载区块（有移动副作用）。
- 参数：`x`/`y`/`z` number 必填。
- 返回：对象或数组 `{name = 方块名, states = {状态键 = 值, ...}, nbt = {NBT 键 = 值, ...}}`（无 NBT 时 `nbt` 省略）。
- 报错：失败抛 `getBlockData: <err>`。
- 底层：传送加载区块后，用游戏的**结构保存**机制读取：先通过 WS 自动化通道执行 `structure save "<随机名>" <区域>` 把该方块连同方块实体 NBT 存成一个临时结构，再发 `StructureTemplateDataRequest`（ExportFromSave）把结构数据拉回来，解析 `block_palette`（名称+状态）与 `block_position_data`（NBT）得到完整数据。有移动副作用。

### `game.getBlockNBT`

```cpp
client.game.getBlockNBT(x, y, z);
```

- 作用：只读方块实体 NBT。命令方块指令在 `nbt.Command`，箱子物品在 `nbt.Items`，告示牌文字在 `nbt.FrontText.Text`。
- 参数：`x`/`y`/`z` number 必填。
- 返回：对象或数组，无 NBT 时返回 `nil`。
- 报错：失败抛 `getBlockNBT: <err>`。
- 底层：同 `getBlockData` 的结构保存机制（传送 + `structure save` + `StructureTemplateDataRequest`），只保留解析出的 NBT 部分。有移动副作用。

### `game.getBlockState`

```cpp
client.game.getBlockState(x, y, z);
```

- 作用：读方块名称与方块状态（朝向/是否点亮/开关等），不含 NBT。
- 参数：`x`/`y`/`z` number 必填。
- 返回：对象或数组 `{name = 方块名, states = {...}}`。
- 报错：失败抛 `getBlockState: <err>`。
- 底层：同 `getBlockData` 的结构保存机制，只保留名称与方块状态，丢弃 NBT。有移动副作用。

### `game.getBlockTile`

```cpp
client.game.getBlockTile(x, y, z);
```

- 作用：读某坐标的方块名。用安全版：可能先把机器人传送过去加载区块再查，避免远处区块"未加载"报错，因此有移动副作用。
- 参数：`x`/`y`/`z` number 必填。
- 返回：string（方块名/方块 ID）。
- 报错：失败抛 `getBlockTile: <err>`。
- 底层：**有移动副作用**：机器人先 `tp` 到目标上方高空（y=150）把该 x,z 的区块加载出来（`testforblock` 只对已加载区块有效，远处直接查会返回 `outOfWorld`），等游戏刻 + 约 400ms 区块就绪后查询，最后传送回原位。查询本身通过 WS 自动化通道执行 `testforblock <x> <y> <z> air`：`SuccessCount > 0` → 该坐标是空气；否则从命令失败输出里解析实际方块名（兼容网易版的参数偏移）。应在独立后台协程中调用（会传送并等待）。

### `game.getBlockTileRaw`

```cpp
client.game.getBlockTileRaw(x, y, z);
```

- 作用：直接查询方块，不做传送加载（更快，但区块未加载可能报错）。
- 参数：`x`/`y`/`z` number 必填。
- 返回：string（方块名）。
- 报错：失败抛 `getBlockTileRaw: <err>`。
- 底层：通过 WS 自动化通道执行 `testforblock <x> <y> <z> air`：`SuccessCount > 0` → 空气；否则从失败输出解析实际方块名。**不做传送**，远区块可能因未加载返回 `outOfWorld` 报错。

### `game.getBotPos`

```cpp
client.game.getBotPos();
```

- 作用：读取当前活跃机器人自身坐标。
- 参数：无。
- 返回：对象或数组 `{x, y, z}`。
- 报错：无。
- 底层：通过 WS 自动化通道执行只读命令 `querytarget @s` 并解析坐标、朝向、维度；pitch 与飞行/疾跑状态取本地缓存。

### `game.getCommandOptions`

```cpp
client.game.getCommandOptions();
```

- 作用：查看本实例默认命令参数。
- 参数：`无`；按此顺序传入。
- 返回：channel/mode/window/timeout_ms/rate_cps/transport对象。
- 底层与游戏效果：来自项目command-options.json或本实例setCommandOptions，不代表服务器实测速率。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.getConfig`

```cpp
client.game.getConfig(key, default);
```

- 作用：读本插件配置文件里某个键，缺失返回 `default`。
- 参数：`key` string 必填；`default` 任意类型可选。
- 返回：配置值（或 `default`）。
- 报错：无（读失败回退默认）。
- 底层：读取本插件配置文件 `插件配置文件/<id>/<id>.json`，命中则把值转成 Lua 返回，否则返回 `default`。依赖 `__PLUGIN_ID__`。

### `game.getMultiScore`

```cpp
client.game.getMultiScore(target);
```

- 作用：读某目标在所有记分板上的分数。
- 参数：`target` string 必填。
- 返回：对象或数组，结构 `{player = {记分板名 = 分数, ...}}`。
- 报错：失败抛 `getMultiScore: <err>`。
- 底层：通过 **WS 自动化通道**执行 `scoreboard players list <目标>` 并等待 CommandOutput，把响应里逐条输出的"玩家·记分板·分数"（`commands.scoreboard.players.list.player.entry` 行）解析成 `{玩家 = {记分板 = 分数}}`。

### `game.getPlayerItem`

```cpp
client.game.getPlayerItem(target, item, specialID);
```

- 作用：统计玩家背包里某种物品的数量，可带特殊数据 ID。
- 参数：`target` string、`item` string 必填；`specialID` number 可选，默认 0。
- 返回：number（数量）。
- 报错：失败抛 `getPlayerItem: <err>`。
- 底层：通过 **WS 自动化通道**执行游戏指令 `clear <目标> <物品> <特殊值> 0` 并等待 CommandOutput。**第 4 个参数 `0` 是游戏的"只统计不清除"技巧**——`clear` 的数量参数为 0 时不会真的清空物品，只报告玩家拥有多少这种物品（标准做法，可放心用于查数量）。响应为 `commands.clear.success` 时从输出的数量参数得到该物品数量；`commands.clear.failure.no.items`（玩家根本没有这种物品）返回 0；`commands.generic.syntax`（物品 ID 非法）报错。

### `game.getPlayerPos`

```cpp
client.game.getPlayerPos(target);
```

- 作用：读玩家坐标与所在维度。
- 参数：`target` string 必填（可为玩家名或 `@a`/`@p` 选择器）。
- 返回：对象或数组 `{x, y, z, dimension}`。
- 报错：失败抛 `getPlayerPos: <err>`。
- 底层：通过 **WS 自动化通道**执行只读命令 `querytarget <目标>` 并等待响应，解析返回的坐标（玩家脚部位置）并修正成可读值：正数向下取整到两位小数、负数先减 1、Y 再减 1.62（眼睛高度）；俯仰角 pitch 从工具箱维护的玩家缓存补取。`querytarget` 无副作用。

### `game.getPlayerPosXYZ`

```cpp
client.game.getPlayerPosXYZ(target);
```

- 作用：读玩家坐标，直接返回三个数字。
- 参数：`target` string 必填。
- 返回：三个 number，依次 x、y、z。
- 报错：失败抛 `getPlayerPosXYZ: <err>`。
- 底层：同 `getPlayerPos`——执行 `querytarget` 后只取返回坐标的 x、y、z 三个数。

### `game.getRegionBlockData`

```cpp
client.game.getRegionBlockData(x, y, z, sx, sy, sz);
```

- 作用：读一块区域（起点 `x,y,z`，尺寸 `sx×sy×sz`）所有方块的名称+状态+NBT，一次结构保存返回，适合整片区域/区块探查。
- 参数：6 个 number 均必填，尺寸需为正。
- 返回：对象或数组，键为 `"x,y,z"` 绝对坐标，值为 `{name, states, nbt}`。
- 报错：失败抛 `getRegionBlockData: <err>`。
- 底层：机器人先传送到区域中心上方加载区块，再**一次结构保存**（`structure save` + `StructureTemplateDataRequest`）把整片区域拉回，按 `block_indices` 线性索引还原成 `"x,y,z"` → 方块数据的映射。有移动副作用。

### `game.getScore`

```cpp
client.game.getScore(scoreboard, target);
```

- 作用：读某记分板里某个目标（玩家名）的分数。
- 参数：`scoreboard` string 必填；`target` string 必填。
- 返回：number（分数）。
- 报错：失败抛 `getScore: <err>`。
- 底层：通过 **WS 自动化通道**执行 `scoreboard players test <目标> <记分板> 0 0`（借用"分数是否在 0~0 之间"的判断把分数值带出来）并等待 CommandOutput，从响应的 `Parameters[0]` 读出分数。响应是 `commands.scoreboard.objectiveNotFound`（计分板不存在）、`...players.list.player.empty`（目标不存在）或 `...players.score.notFound`（目标在该记分板无分数）时，分别报对应错误。

### `game.getTarget`

```cpp
client.game.getTarget(selector);
```

- 作用：解析选择器，返回匹配的玩家名数组。
- 参数：`selector` string 必填。
- 返回：对象或数组（玩家名字符串数组，1 起索引）。
- 报错：失败抛 `getTarget: <err>`。
- 底层：通过 **WS 自动化通道**执行 `testfor <选择器>` 并等待响应，把输出参数里逗号分隔的玩家名列表解析成数组。选择器语法错误（`commands.generic.syntax`）时报错；无匹配返回空表。

### `game.getTickingAreaList`

```cpp
client.game.getTickingAreaList();
```

- 作用：读当前加载的常驻区域（ticking area）列表。
- 参数：无。
- 返回：对象或数组 `{区域名 = {dimension, start_x, start_z, end_x, end_z}}`。
- 报错：失败抛 `getTickingAreaList: <err>`。
- 底层：通过 **WS 自动化通道**执行 `tickingarea list all-dimensions` 并等待响应，把命令返回的多行文本输出（每维度一组，每组里 `- 名称: (startX, startZ) to (endX, endZ)`）解析成结构化表。

### `game.httpGet`

```cpp
client.game.httpGet(url);
```

- 作用：发起 HTTP GET 请求，返回响应体文本。
- 参数：`url` string 必填。
- 返回：string（响应体）。
- 报错：失败抛 `httpGet: <err>`。
- 底层：发起一次真实的 HTTP GET 请求，把响应体按 UTF-8 文本返回；网络错误或非 2xx 状态码都会抛错。

### `game.httpPost`

```cpp
client.game.httpPost(url, body, headers);
```

- 作用：发起 HTTP POST 请求，可带自定义请求头。
- 参数：`url` string 必填；`body` string 可选，默认 `""`；`headers` 对象或数组（键值均为 string）可选。
- 返回：string（响应体）。
- 报错：失败抛 `httpPost: <err>`。
- 底层：发起一次真实的 HTTP POST 请求（可带请求头与请求体），把响应体按 UTF-8 文本返回；网络错误或非 2xx 状态码都会抛错。

### `game.isCmdSuccess`

```cpp
client.game.isCmdSuccess(cmd);
```

- 作用：执行一条指令并判断是否成功，返回布尔。
- 参数：`cmd` string 必填。
- 返回：boolean（是否成功）。
- 报错：失败抛 `isCmdSuccess: <err>`。
- 底层：通过 **WS 自动化通道**执行指令并**等待 CommandOutput**，命令响应的 `SuccessCount > 0` 即视为成功。无 OP 或语法错误的指令，服务器返回失败输出，此函数返回 `false`。

### `game.jsonDecode`

```cpp
client.game.jsonDecode(s);
```

- 作用：把 JSON 字符串解析成 JSON兼容值。
- 参数：`s` string 必填。
- 返回：任意 JSON兼容值（表/数字/字符串/布尔）。
- 报错：解析失败抛 `jsonDecode: <err>`。
- 底层：把 JSON 解析成对应 Lua 类型（对象→表、数组→1 起索引的表、数字/字符串/布尔原样返回）。

> 这几个是挂在 `game` 上的轻量便捷方法；需要完整请求控制（超时、多种方法、错误码）时用 4.12 节的 `http` 模块。

### `game.jsonEncode`

```cpp
client.game.jsonEncode(v);
```

- 作用：把任意 JSON兼容值编码成 JSON 字符串。
- 参数：`v` 任意类型。
- 返回：string（JSON）。
- 报错：序列化失败抛 `jsonEncode: <err>`。
- 底层：把 JSON兼容值映射成对应的 JSON 类型（表→对象/数组、数字/字符串/布尔原样）后编码成字符串。

### `game.jump`

```cpp
client.game.jump();
```

- 作用：让机器人跳一下。
- 参数：无。
- 返回：无。
- 报错：失败抛 `jump: <err>`。
- 底层：模拟按一次跳跃键：置位跳跃标志让心跳携带跳跃输入，约 200ms 后自动松开。

### `game.move`

```cpp
client.game.move(dx, dy, dz);
```

- 作用：让机器人相对当前位置移动。
- 参数：`dx`/`dy`/`dz` number 必填。
- 返回：无。
- 报错：失败抛 `move: <err>`。
- 底层：在当前坐标上累加位移后发一个 `MovePlayer` 包（`MoveModeTeleport`），瞬间完成相对移动。

### `game.placeNBTBlock`

```cpp
client.game.placeNBTBlock(x, y, z, name, states, nbtTable);
```

- 作用：放置一个带 NBT 的特殊方块（命令方块/结构方块/告示牌/箱子/旗帜等），自动按方块类型选择最快可靠的放置方式：
  - 命令方块：`{Command = "say hi", Conditional = 1, TickDelay = 0}`；
  - 告示牌：`{FrontText = {Text = "你好", Color = "black"}, ...}`；
  - 箱子：`{Items = {{Count = 1, Name = "minecraft:diamond", Slot = 0}, ...}}`。
- 参数：`name` string 必填；`states` string 可选；`nbtTable` 对象或数组 可选。
- 返回：无。
- 报错：失败抛 `placeNBTBlock: <err>`。
- 底层：按 `building.ClassifyBlock` 对方块分类后选择不同游戏机制：命令方块/结构方块 → **直接发 NBT 包**（快）；告示牌 → 直接发 `PlaceSign` 包（快）；重 NBT 方块（箱子/旗帜等）→ 机器人**放到远程工作台 + 结构复制**（慢，需要结构权限）；普通方块 → `setblock` 指令。

### `game.promptChoice`

```cpp
client.game.promptChoice(player, title, options, timeoutSec);
```

- 作用：显示菜单并**阻塞等待玩家选择**，返回所选选项下标。
- 参数：`player`/`title` string、`options` 对象或数组 必填；`timeoutSec` number 可选，默认 30。
- 返回：number 或 nil。成功返回 **1 起** 下标（第一个是 1）；超时/取消/无效输入返回 `nil`。
- 报错：失败抛 `promptChoice: <err>`。
- 底层：先用 `showMenu` 把菜单发给玩家，再用 `waitMsg` 的机制**阻塞等待该玩家的下一条聊天消息**，把它解析成序号（支持 `1`、`1.`、`1、` 等写法）。超时或输入无效返回 `nil`。

### `game.queryInventory`

```cpp
client.game.queryInventory(selector);
```

- 作用：查询玩家背包物品清单。
- 参数：`selector` string 必填。
- 返回：对象或数组 `{items = {{name, count, slot, damage}, ...}}`。
- 报错：失败抛 `queryInventory: <err>`。
- 底层：通过 WS 自动化通道执行网易特有的 `codebuilder_actorinfo inventory <选择器>` 并等待响应，从命令响应的 DataSet（JSON）里解析出 `{name, count, slot, damage}` 物品清单。需要机器人具备网易 AI/指令权限。

### `game.registerTrigger`

```cpp
client.game.registerTrigger(wordsTable, handlerName);
```

- 作用：注册聊天触发词，常驻实例调用handler(player,args)。
- 参数：`wordsTable, handlerName`；按此顺序传入。
- 返回：Lua无返回；SDK注册成功为true。
- 底层与游戏效果：源码只加载一次；玩家发“触发词 参数”后在该插件常驻状态中调用函数。args为参数数组，不重置全局变量。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `game.rotate`

```cpp
client.game.rotate(pitch, yaw, seconds);
```

- 作用：设置机器人朝向。
- 参数：`pitch`/`yaw` number 必填；`seconds` number 可选——省略或 `0` 时**立即转到**目标角度，`>0` 时在 seconds 秒内**平滑转动**过去（yaw 走最短路径，如从 350° 转到 10° 只转 20°）。
- 返回：无。
- 报错：失败抛 `rotate: <err>`。
- 底层：只更新机器人本地视角状态，心跳（50ms）通过 `PlayerAuthInput` 包把新视角发送到服务器。秒数 `>0` 时先在本地逐帧插值，再由心跳带出中间帧。

### `game.scene`

```cpp
client.game.scene();
```

- 功能、参数、返回与约束：无参数。当前账号的机器人和已观测实体快照；含 known、stale、数据时间和来源。AI 没有独立游戏实体，bot 是当前账号控制的机器人。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.sendActionbar`

```cpp
client.game.sendActionbar(target, text);
```

- 作用：给玩家显示动作栏消息（物品栏上方那行）。
- 参数：`target`/`text` string 必填。
- 返回：无。
- 报错：失败抛 `sendActionbar: <err>`。
- 底层：通过当前账号命令发送通道发 `titleraw <target> actionbar {"rawtext":[{"text":"..."}]}`。

### `game.sendChat`

```cpp
client.game.sendChat(target, message);
```

- 作用：给指定玩家或选择器（如 `@a`）发一条聊天消息。
- 参数：`target` string 必填；`message` string 必填。
- 返回：无。
- 报错：失败抛 `sendChat: <err>`。
- 底层：通过当前账号命令发送通道发 `tellraw <target> {"rawtext":[{"text":"..."}]}`（target 为 `@a` 或玩家名），文本里的 `"` 会自动转义。

### `game.sendCommand`

```cpp
client.game.sendCommand(cmd, options);
```

- 作用：无等待按顺序提交命令；没有默认逐条CPS等待。
- 参数：`cmd, options?`；按此顺序传入。
- 返回：Lua旧单参数写法保持无返回值；Python/C++返回提交信息（submitted/confirmed）。
- 底层与游戏效果：当前默认魔法/ai使用SettingsCommand，套execute as机器人 at @s保留执行者与相对坐标；player/console为相应CommandRequest。不会逐条等待CommandOutput。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.sendCommandAsync`

command,options?；立刻返回ticket，不阻塞插件回调；后台等待真实CommandOutput，使用game.commandResult读取。未读取ticket容量由diagnostics.configure的async_tickets控制，默认128，满时明确报错；Fast路径不走该窗口。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `game.sendCommandConfirmed`

```cpp
client.game.sendCommandConfirmed(command, options);
```

- 功能、参数、返回与约束：command, options?；game.execute别名，等待真实CommandOutput，超时为未知，不自动重发。只在依赖前条结果时使用。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.sendCommandFast`

```cpp
client.game.sendCommandFast(command, options);
```

- 功能、参数、返回与约束：command, options?；无等待提交，魔法/ai 默认使用 SettingsCommand 并保持机器人 @s/相对坐标；其他通道保持来源。rate_cps 默认0，不逐条等回包。confirmed=false 不能当游戏成功。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.sendCommandsFast`

```cpp
client.game.sendCommandsFast(commands, options);
```

- 功能、参数、返回与约束：commands数组, options?；一次插件宿主边界按顺序提交完整数组，不限整数组行数，不等待逐条回包。先校验全部命令；中途网络失败报告已提交数量，不自动重发，不保证同一服务器Tick或原子执行。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `game.sendLines`

```cpp
client.game.sendLines(player, text, maxLines);
```

- 作用：把长文本**按换行逐条发送**，避免单条超长被截断、或某行含敏感词整段被拦。超行数时先提示"仅显示前 N 行"。空行跳过。
- 参数：`player` string 必填；`text` string 必填；`maxLines` number 可选，默认 50。
- 返回：无。
- 报错：无。
- 底层：把长文本按 `\n` 切分，逐行用 `sendTargeted` 的定向 tellraw 机制发送；超过 `maxLines` 先提示"仅显示前 N 行"，空行跳过。

### `game.sendPacket`

```cpp
client.game.sendPacket(typeNameOrId, fieldsTable);
```

- 作用：发送一个已校验的协议包，支持包名或协议ID。
- 参数：`typeNameOrId, fieldsTable?`；按此顺序传入。
- 返回：submitted=true, confirmed=false；校验、编码或网络错误抛异常。
- 底层与游戏效果：反射填充当前内核真实类型后conn.WritePacket；字段用snake_case；bytes/UUID为hex，Vec为数组，64位ID建议十进制字符串。联合接口类型不猜测，使用高级动作API或payload_hex完整包体。登录生命周期不可重发。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.sendRichTellraw`

```cpp
client.game.sendRichTellraw(msg);
```

- 作用：发送原生 tellraw 富文本（支持 JSON 样式），常用于公告。
- 参数：`msg` string 必填。
- 返回：无。
- 报错：失败抛 `sendRichTellraw: <err>`。
- 底层：通过当前账号命令发送通道发 `tellraw @a {"rawtext":[{"text":"..."}]}`，`msg` 里的 `"` 自动转义后嵌入 JSON。

### `game.sendTargeted`

```cpp
client.game.sendTargeted(player, text);
```

- 作用：只给指定玩家发一条消息，不广播给其他人。`§` 前缀可带颜色（如 `"§a成功！"`）。
- 参数：`player` string 必填；`text` string 必填。
- 返回：无。
- 报错：无（底层错误被忽略）。
- 底层：通过**当前账号命令发送通道**（无等待默认SettingsCommand，保留机器人执行上下文）发定向 tellraw：`tellraw @a[name="<玩家>"] {"rawtext":[{"text":"..."}]}`。只发给该玩家、绝不广播；用 `@a[name=...]` 选择器是为了兼容网易服（普通通道在网易不执行）。

### `game.sendTitle`

```cpp
client.game.sendTitle(target, title, subtitle);
```

- 作用：给玩家显示大标题 + 副标题。
- 参数：`target`/`title`/`subtitle` 均 string 必填。
- 返回：无。
- 报错：失败抛 `sendTitle: <err>`。
- 底层：通过当前账号命令发送通道发 `titleraw <target> title {"rawtext":[{"text":"..."}]}`；副标题非空时再追加一条 `titleraw <target> subtitle {"rawtext":[...]}`。

### `game.setBlock`

```cpp
client.game.setBlock(x, y, z, name, states);
```

- 作用：放置一个普通方块，可带方块状态字符串。
- 参数：`name` string 必填；`states` string 可选（如 `["facing_direction"=3]`，默认空）。
- 返回：无。
- 报错：失败抛 `setBlock: <err>`。
- 底层：通过**当前账号命令发送通道**发 `setblock <x> <y> <z> <方块> <states>`（PyRpc `ExecuteCommandEvent`，权限比普通命令通道高，在网易租赁服更可靠），发完稍等片刻返回。

### `game.setBlockState`

```cpp
client.game.setBlockState(x, y, z, statesTable);
```

- 作用：修改某方块的方块状态并保留原 NBT（如把命令方块设为条件模式、把方块朝向改朝北）。会先读当前方块，合并新状态后重新放置。
- 参数：`statesTable` 对象或数组 必填，键 = 状态名，值 = 数值/布尔（如 `{conditional_bit = 1, facing_direction = 3}`）。
- 返回：无。
- 报错：失败抛 `setBlockState: <err>`。
- 底层：先按 `getBlockData` 的结构保存机制读回当前方块的名称、状态与 NBT，把新状态合并进去后，再按 `placeNBTBlock` 的方式**重新放置**该方块（保留原 NBT）。有移动副作用（需要传送读取）。

> **注意**：箱子/旗帜这类"重 NBT 方块"写入在网易下走远程工作台 + 结构复制，较慢且需要结构权限；命令方块/告示牌直接发包，快。读取和写入都会传送机器人加载区块，属正常副作用。

### `game.setCommandOptions`

```cpp
client.game.setCommandOptions(options);
```

- 作用：更新本实例默认命令参数，不修改其他插件。
- 参数：`options`；按此顺序传入。
- 返回：合并后的options对象。
- 底层与游戏效果：channel=ai/player/console/default，mode=ordered/pipeline/submit/burst/stream，window、timeout_ms、rate_cps；rate_cps=0不加人为CPS等待。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `game.setConfigDefault`

```cpp
client.game.setConfigDefault(table);
```

- 作用：把配置文件中**缺失的键**写入默认值（已有的键不覆盖）。
- 参数：`对象或数组` 必填（键值任意类型）。
- 返回：无。
- 报错：无（写失败忽略）。
- 底层：先读 `插件配置文件/<id>/<id>.json`，把其中缺失的键补写进去（已存在的键不动）。依赖 `__PLUGIN_ID__`。

### `game.setEffect`

```cpp
client.game.setEffect(player, effect, duration, level, particles);
```

- 作用：给玩家施加药水效果。
- 参数：`player` string、`effect` string、`duration` number、`level` number 必填；`particles` boolean 可选，默认 `true`。
- 返回：无。
- 报错：失败抛 `setEffect: <err>`。
- 底层：通过 **WS 自动化通道**执行 `effect <玩家> <效果> <时长秒> <等级> <是否显示粒子>` 并等待响应，响应 `commands.effect.success` 才算成功。`commands.generic.noTargetMatch`（没有匹配目标）时报错；等级上限 255、时长上限 1000000 秒，超出直接报错。

### `game.setScore`

```cpp
client.game.setScore(board, target, score);
```

- 作用：写入记分板，执行 `scoreboard players set`。
- 参数：`board` string、`target` string、`score` number 均必填。
- 返回：无。
- 报错：失败抛 `setScore: <err>`。
- 底层：拼指令 `scoreboard players set <target> <board> <score>`，通过**当前账号命令发送通道**发出，不等响应。

### `game.showMenu`

```cpp
client.game.showMenu(player, title, options);
```

- 作用：给玩家显示一个聊天栏菜单（标题 + 若干选项），只展示不等待。
- 参数：`player` string、`title` string 必填；`options` 对象或数组（字符串数组）必填。
- 返回：无。
- 报错：无。
- 底层：通过当前账号命令发送通道给该玩家发**多条定向 tellraw**（绝不广播）：标题一条，每个选项一条 `§e序号. 选项文字`。

### `game.sneakStart`

```cpp
client.game.sneakStart();
```

- 作用：开始 / 停止潜行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sneakStart/sneakStop: <err>`。
- 底层：设置潜行标志，心跳持续携带潜行输入；`sneakStop` 额外发一个带 `StopSneaking` 标志的 `PlayerAuthInput` 包。

### `game.sneakStop`

```cpp
client.game.sneakStop();
```

- 作用：开始 / 停止潜行。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sneakStart/sneakStop: <err>`。
- 底层：设置潜行标志，心跳持续携带潜行输入；`sneakStop` 额外发一个带 `StopSneaking` 标志的 `PlayerAuthInput` 包。

### `game.sprintStart`

```cpp
client.game.sprintStart();
```

- 作用：开始 / 停止疾跑。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sprintStart/sprintStop: <err>`。
- 底层：设置奔跑标志，心跳持续携带奔跑输入。

### `game.sprintStop`

```cpp
client.game.sprintStop();
```

- 作用：开始 / 停止疾跑。
- 参数：无。
- 返回：无。
- 报错：失败抛 `sprintStart/sprintStop: <err>`。
- 底层：设置奔跑标志，心跳持续携带奔跑输入。

### `game.stopMove`

```cpp
client.game.stopMove();
```

- 作用：停止机器人当前移动。
- 参数：无。
- 返回：无。
- 报错：失败抛 `stopMove: <err>`。
- 底层：关闭持续移动的本地循环并清空移动输入标志，心跳随之不再携带方向输入。

### `game.teleport`

```cpp
client.game.teleport(x, y, z);
```

- 作用：传送机器人到绝对坐标。
- 参数：`x`/`y`/`z` number 必填。
- 返回：无。
- 报错：失败抛 `teleport: <err>`。
- 底层：发一个 `MovePlayer` 包（`MoveModeTeleport`）把机器人瞬间传送到目标坐标——**不是 `/tp` 指令**，是客户端传送包，服务器按传送处理。

### `game.waitMsg`

```cpp
client.game.waitMsg(player, timeoutSec);
```

- 作用：**阻塞等待玩家在聊天栏发一条消息**，返回内容。用于要输入（领地、数量等）。
- 参数：`player` string 必填；`timeoutSec` number 可选，默认 30。
- 返回：string（收到的消息）。超时返回 `nil`。
- 报错：失败抛 `waitMsg: <err>`。
- 底层：在机器人**收包循环**里为该玩家注册一个"等待通道"，玩家下一条聊天消息到达时投递给等待者。它**独占**该玩家的消息——同一玩家已有活跃等待会话时，新消息会被它消费掉，`onChat` 等其它 handler 收不到。


## 服务器关闭命令反馈时

实服已验证：gamerule sendcommandfeedback false 会使 Confirmed/querytarget 等结果接口收不到回包，即使命令已经执行。Fast 不等待回包，模式维护也使用无等待创造命令和飞行输入。需要条件链、查询结果或确认时，要由服主确保服务器允许命令反馈；超时返回未知，不能把它当失败自动重发。高频无条件链不要为语法验证逐条增加确认。


# player · 玩家资料与感知

缓存、XUID、权限、背包、位置与已同步血量。未知血量不是20。


### `player.count`

```cpp
client.player.count();
```

- 作用：返回在线玩家数量。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：玩家缓存表当前条数。

### `player.get`

```cpp
client.player.get(name);
```

- 作用：按名字查玩家信息。
- 参数：`name` string 必填。
- 返回：对象或数组 `{name, xuid, unique_id, is_op, online, dimension, x, y, z, held_item?}`（`held_item` 仅非空时出现，为网络 ID）。玩家不存在返回**空表**（不是 nil）。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表，不向游戏发指令。

### `player.getInventory`

```cpp
client.player.getInventory(name);
```

- 作用：走当前活跃机器人查询某玩家背包物品清单。
- 参数：`name` string 必填。
- 返回：对象或数组 `{items = {{name, count, slot, damage}, ...}}`。
- 报错：失败抛 `player.getInventory: <err>`。
- 底层：同 `game.queryInventory`——通过 WS 自动化通道执行网易特有命令 `codebuilder_actorinfo inventory <玩家名>`，从响应 DataSet（JSON）解析物品清单。

### `player.getUniqueID`

```cpp
client.player.getUniqueID(name);
```

- 作用：查玩家实体唯一 ID。
- 参数：`name` string 必填。
- 返回：number；玩家不存在返回 **0**。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表的实体唯一 ID 字段。

### `player.getXUID`

```cpp
client.player.getXUID(name);
```

- 作用：查玩家 XUID。
- 参数：`name` string 必填。
- 返回：string；玩家不存在返回**空串**。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表的 XUID 字段。

### `player.health`

```cpp
client.player.health(target);
```

- 功能、参数、返回与约束：target 为 self、玩家名或 runtime_id 字符串，默认 self。返回已观测信息含 health_known/health/max_health/health_time/stale，未知为 null，不推算其他玩家满血。仅能获取服务器给本机器人同步的属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `player.isOP`

```cpp
client.player.isOP(name);
```

- 作用：判断玩家是否 OP。
- 参数：`name` string 必填。
- 返回：boolean（玩家存在且为 OP 才为 true，含不存在时返回 false）。
- 报错：`name` 非 string 抛类型错误。
- 底层：直接查玩家缓存表（`IsOP` 由 `UpdateAbilities`/`AddPlayer` 等数据包维护）。

### `player.list`

```cpp
client.player.list();
```

- 作用：列出全部在线玩家信息表。
- 参数：无。
- 返回：对象或数组（1 起索引，每个元素是 `player.get` 同款信息表）。
- 报错：无。
- 底层：遍历玩家缓存表。

### `player.nearby`

```cpp
client.player.nearby(radius);
```

- 功能、参数、返回与约束：radius=0..512，默认 32；按机器人已观测位置计算同维度、仍存在的玩家距离并排序。不包括客户端不可见或未知坐标的玩家，机器人坐标未知报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `player.position`

```cpp
client.player.position(target);
```

- 功能、参数、返回与约束：target 同 player.health。返回 position_known/position/position_time/dimension/stale；坐标为收到服务器包的观测，不把本地移动请求当服务器确认。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `player.equipment`

name或runtime_id；head/chest/legs/feet/main_hand/off_hand/body，每槽含known/item_id/item_id_known/network_id/count/enchantments/enchantments_known/nbt/received_at/age_ms/stale。物品名来自本会话ItemRegistry；附魔来自NBT。未知槽或未同步附魔不编造。expires_at=null：装备包没有物品失效时间。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `player.inventory`

target=self,window_id=0；读取本机器人实际收到的InventoryContent/InventorySlot，不做同步背包查询。其他玩家背包服务器通常不广播，返回known=false；不把装备冒充整个背包。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `player.rotation`

name或runtime_id；pitch/yaw/head_yaw各自known、updated_at、source/stale。MovePlayer及Absolute/Delta真实角度，旋转无位移也更新。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。


# world · 方块与物理操作

查询、setblock、真实挖掘和手持物品点击。权限替换方块与生存挖掘是两种机制。


### `world.block`

```cpp
client.world.block(x, y, z);
```

- 作用：查询世界某坐标的方块名（需要机器人连接，目标为当前活跃机器人）。
- 参数：`x`/`y`/`z` number 必填（整数坐标）。
- 返回：string（方块名）。
- 报错：查询失败抛 `world.block: <err>`；参数非整数抛类型错误。
- 底层：同 `game.getBlockTile`——机器人先传送加载区块，再执行 `testforblock <x> <y> <z> air` 判断方块名。**有移动副作用**（会把当前活跃机器人传送过去再传回）。

### `world.blockState`

```cpp
client.world.blockState(x, y, z);
```

- 功能、参数、返回与约束：x,y,z 整数坐标。返回当前维度 layer=0 的最近 UpdateBlock 观测，未知 known=false；包含方块 runtime_id，不把任何更新都认定是玩家破坏。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.breakBlock`

```cpp
client.world.breakBlock(x, y, z, mode);
```

- 功能、参数、返回与约束：x,y,z 整数坐标，mode=command 或 creative_action，默认 command。command 用 setblock ... air destroy 等待实际结果；creative_action 发创造模式破坏动作，要求游戏模式和六格范围，不代表服务器确认。生存挖掘的装备/耗时/交易流程尚不支持。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.mineBlock`

```cpp
client.world.mineBlock(x, y, z, duration_ms, slot, face);
```

- 功能、参数、返回与约束：x,y,z,duration_ms=1000,slot=0,face=1；生存挖掘尝试，真实开始/持续/结束动作与手持库存交易，按 100ms 发送。需要最近方块观测；服务端可拒绝，返回 submitted/confirmed。硬度、工具正确与否由服务器判断。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.placeBlock`

```cpp
client.world.placeBlock(x, y, z, block);
```

- 功能、参数、返回与约束：x,y,z 整数坐标，block 例如 minecraft:stone。用服务器 setblock 指令，返回真实结果，需要权限。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `world.useBlock`

```cpp
client.world.useBlock(x, y, z, face, slot);
```

- 功能、参数、返回与约束：x,y,z,face=1,slot=0；真实手持物品点击指定方块面（可放置、开容器），需要已观测目标方块和六格范围。不是 setblock；返回提交状态，消耗与放置观察服务器。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。


# events · 事件与抓包

聊天、进退服、死亡、挥臂、实体出现与拾取，以及原始协议订阅。


### `events.captureClear`

```cpp
client.events.captureClear();
```

- 功能、参数、返回与约束：清空当前账号抓包缓冲，保持抓包开关。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureRead`

```cpp
client.events.captureRead(after_sequence, limit);
```

- 功能、参数、返回与约束：after_sequence=0,limit=256，返回记录/游标/覆盖和抽样统计。缓冲有界，单条超大小截为预览；不是手机所有 App 的网络抓包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureStart`

```cpp
client.events.captureStart(config);
```

- 功能、参数、返回与约束：config={types:[包名或*],exclude:[包名],filter:{fields:{...},every_ms:...},capacity:1..2048,max_bytes:512..65536}。开启账号级解码入站游戏抓包；默认血量/死亡/位置/方块包，认证握手不记录，* 默认不包含大区块包。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.captureStop`

```cpp
client.events.captureStop();
```

- 功能、参数、返回与约束：停止记录，保留已抓数据。当前账号的多个插件共用账号级记录；使用时协调抓包开关。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onBlockChange`

```cpp
client.events.onBlockChange(handler);
```

- 功能、参数、返回与约束：handler(event)，UpdateBlock 导出 position/runtime_id/previous/cause_known=false；不能仅从方块更新认定破坏者。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onBotReady`

```cpp
client.events.onBotReady(handlerName);
```

- 作用：注册机器人就绪事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onBotReady()`——0 参。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evBotReady, L)`。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onChat`

```cpp
client.events.onChat(handlerName);
```

- 作用：注册聊天事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onChat(player, msg)`——触发传 2 参：`player` string（玩家名）、`msg` string（聊天内容）。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evChat, L)`；触发 `fireChatEvent` → `runEventHandlers`。实时。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onDeath`

```cpp
client.events.onDeath(handler);
```

- 功能、参数、返回与约束：handler 回调接收一个事件对象，来源为 ActorEventDeath/InstantDeath 或 health<=0；同一已死亡状态去重，重生后可再触发。不推测死亡原因/攻击者。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onEntityRemove`

```cpp
client.events.onEntityRemove(handler);
```

- 功能、参数、返回与约束：handler(event)；RemoveActor下发时触发，含unique_id和已知runtime_id。消失不等于死亡，也可能是离开视距。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onEntitySpawn`

```cpp
client.events.onEntitySpawn(handler);
```

- 功能、参数、返回与约束：handler(event)；AddPlayer/AddActor/AddItemActor下发时触发，含实体类型、位置和原始fields。服务器视距内的出现，不代表世界内首次出生；血量仅取真实同步属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onHealthChange`

```cpp
client.events.onHealthChange(handler);
```

- 功能、参数、返回与约束：handler(event)，仅在观测血量初始值或变化时入队，含 previous/health/max_health/time/source。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onHurt`

handler(event)；血量下降优先；ActorEvent等50ms关联属性包，否则提供damage_known=false提示；hurt_signal仅供诊断；未知攻击者attacker_known=false。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onItemPickup`

```cpp
client.events.onItemPickup(handler);
```

- 功能、参数、返回与约束：handler(event)；TakeItemActor下发时触发，含拾取实体和掉落物runtime_id、已观测item/position。拾取者可为生物；不凭RemoveActor猜拾取。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onJoin`

```cpp
client.events.onJoin(handlerName);
```

- 作用：注册玩家加入事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onJoin(player)`——1 参 `player` string。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evJoin, L)`；由后台 5 秒玩家列表轮询差量触发（有约几秒延迟）。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onLeave`

```cpp
client.events.onLeave(handlerName);
```

- 作用：注册玩家离开事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onLeave(player)`——1 参 `player` string。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evLeave, L)`；由 5 秒轮询差量触发。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onMove`

```cpp
client.events.onMove(handler);
```

- 功能、参数、返回与约束：handler(event)，服务器实际位置初始值/变化时触发；含 previous_known/position/dimension/source。高频事件可用抓包抽样过滤减少负担。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onPacket`

```cpp
client.events.onPacket(handlerName);
```

- 作用：注册数据包事件处理器。
- 参数：`handlerName` string 必填，回调签名 `onPacket(type, dataTable)`——2 参：`type` string（包类型）、`dataTable` 对象或数组（数据）。
- 返回：无。
- 报错：`handlerName` 非 string 抛类型错误。
- 底层：`registerEvent(&evPacket, L)`；触发 `firePacketEvent`，数据经 `goToLua` 转表。实时。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onPlayerDeath`

```cpp
client.events.onPlayerDeath(handler);
```

- 功能、参数、返回与约束：handler(event)；只接受玩家死亡；来自真实ActorEvent/血量归零/网易ModEventS2C OnPlayerDie，附已知最后位置、维度、timestamp；没有位置保持position_known=false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onRespawn`

```cpp
client.events.onRespawn(handler);
```

- 功能、参数、返回与约束：handler(event)，只处理服务端 RespawnStateReadyToSpawn，清除旧血量等待新属性。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onSwing`

```cpp
client.events.onSwing(handler);
```

- 功能、参数、返回与约束：handler(event)；服务器AnimateActionSwingArm下发时触发，含runtime_id/player/source。挥臂不证明命中了目标，服务器未广播则无法观测。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.packetSchema`

```cpp
client.events.packetSchema();
```

- 功能、参数、返回与约束：无参数。返回当前内核支持包类型、协议 ID、顶层字段名与类型，供 AI 根据实际协议找包而非猜字段。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribe`

```cpp
client.events.subscribe(packetType, handlerName);
```

- 作用：**通用数据包订阅**。订阅任意一个数据包类型（按 Go 类型名，如 `"MovePlayer"`/`"SetTime"`/`"PlayerList"`/`"Text"`）。订阅后机器人只监听你订阅的包类型，收到就把**完整**数据包广播给该 handler。
- 参数：`packetType` string 必填（包类型名）；`handlerName` string 必填，回调签名 `onData(dataTable)`——1 参：`dataTable` 对象或数组（包数据，字段名 snake_case；`[]byte` 给**完整 hex**，不截断）。
- 返回：无。同插件同类型同 handler 自动去重；重跑上下文跳过注册。
- 报错：参数非 string 抛类型错误；不在插件上下文调用报错。
- 底层：`registerPacketSub` → `packetHubSubscribe`（packet_hub.go）。想查有哪些包类型可订阅，可用 AI 的 `list_packets` 工具。清理：插件停止时 `RemovePluginEvents` 一并退订。
- 示例：

- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribeAll`

```cpp
client.events.subscribeAll(handlerName);
```

- 作用：全量入站包订阅，回调接收一个包信封。
- 参数：`handlerName`；按此顺序传入。
- 返回：Lua无返回；SDK注册成功true；停止自动退订。
- 底层与游戏效果：信封含seq/at/direction/bot/packet_type/packet_id/fields。只序列化有订阅的包；全包模式会含高频及大区块数据，正式插件优先subscribe或subscribeFiltered；队列有界，dropped可查。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.subscribeFiltered`

```cpp
client.events.subscribeFiltered(packet_type, filter, handler);
```

- 功能、参数、返回与约束：packet_type 或 *，filter={fields:{字段路径:精确值},every_ms:0..60000}，handler。Python 回调接收包对象；C++/Lua 使用已注册函数名。* 接收当前账号订阅包，事件字段 _packet_type 表示真实类型，停止自动退订。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `events.onDimensionChange`

handler(event)；服务器ChangeDimension，旧维度实体数据标为过期。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onEntityHurt`

handler(event)；真实掉血优先；ActorEventHurt等50ms关联UpdateAttributes，未同步血量时damage_known=false；hurt_signal仅诊断，不计作另一伤害。含target、damage/health/max_health、attacker_known=false（无可靠攻击者包时），event_id。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onEquipmentChanged`

handler(event)；服务器装备包即时缓存更新，含已更新槽位，不同步查询玩家背包。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onHealthChanged`

同onHealthChange，含previous及真实max，未知max=null。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onPlayerKill`

handler(event)；可信服务器death.attack翻译包提供击杀者且关联已知玩家，唯一death event_id，仅触发一次；同次death_details补充不重复计分。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onPlayerRespawn`

handler(event)；与session_join独立，死亡到存活/RespawnReady的生命周期去重。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onRotation`

handler(event)；服务器角度字段变化。旋转包即使位置不变也刷新时间。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onSessionJoin`

handler(event)；真实PlayerList新增条目，不能把AddPlayer视距出现当进服。服务器不广播PlayerList则无法直接观测。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `events.onSessionLeave`

handler(event)；真实PlayerList移除条目，不把RemoveActor离开视距当退服。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。


# packet · 原始协议包

schema/describe查询字段，on/onAll监听，send请求服务器执行；不会取得服务器未下发的数据。


### `packet.describe`

```cpp
client.packet.describe(typeNameOrId);
```

- 作用：查询一个包的实际ID和字段类型。
- 参数：`typeNameOrId`；按此顺序传入。
- 返回：name/id/fields对象；未知包抛错。
- 底层与游戏效果：可离线查询。联合类型的具体分支不能仅靠顶层schema构造，请读对应协议章节。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `packet.on`

```cpp
client.packet.on(packetType, handlerName);
```

- 作用：events.subscribe的别名，回调接收字段对象。
- 参数：`packetType, handlerName`；按此顺序传入。
- 返回：注册结果。
- 底层与游戏效果：精确类型订阅，fields里_packet_type为真实类型。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `packet.onAll`

```cpp
client.packet.onAll(handlerName);
```

- 作用：events.subscribeAll的别名。
- 参数：`handlerName`；按此顺序传入。
- 返回：注册结果。
- 底层与游戏效果：回调接收包信封，fields包含实际包字段。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `packet.schema`

```cpp
client.packet.schema();
```

- 作用：查询当前编译内核的全部已注册协议包。
- 参数：`无`；按此顺序传入。
- 返回：包数组，每项name/id/fields。
- 底层与游戏效果：从真实packet.ListAllPackets枚举，不根据网上版本猜包ID。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `packet.send`

```cpp
client.packet.send(typeNameOrId, fieldsTable);
```

- 作用：game.sendPacket的原始协议入口别名。
- 参数：`typeNameOrId, fieldsTable?`；按此顺序传入。
- 返回：提交状态，未确认。
- 底层与游戏效果：同game.sendPacket。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。


# bot · 机器人控制与路由

选择舰队机器人、状态、输入移动、看向坐标、近战攻击。无自动寻路保证。


### `bot.attack`

```cpp
client.bot.attack(runtime_id, slot);
```

- 功能、参数、返回与约束：runtime_id 字符串,slot=0；真实近战库存交易，只允许当前服务器观测的六格内实体；slot 为 0..8。返回 submitted，伤害必须由服务器事件确认。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.count`

```cpp
client.bot.count();
```

- 作用：返回在线机器人数量。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：`GetFleetBots()`。

### `bot.current`

```cpp
client.bot.current();
```

- 作用：返回当前活跃机器人的名字与坐标。
- 参数：无。
- 返回：对象或数组 `{name, x, y, z}`；无活跃机器人返回空表；坐标查询失败省略 x/y/z。
- 报错：无。
- 底层：从舰队管理器读当前选中的机器人名，再通过 WS 自动化通道执行 `querytarget @s` 取该机器人的坐标。

### `bot.list`

```cpp
client.bot.list();
```

- 作用：列出在线机器人及坐标。
- 参数：无。
- 返回：对象或数组，1 起索引，每个元素 `{name, x, y, z}`（坐标查询失败省略坐标字段）。
- 报错：无。
- 底层：`GetFleetBots()` 遍历 + `QueryPosition()`。

### `bot.lookAt`

```cpp
client.bot.lookAt(x, y, z);
```

- 功能、参数、返回与约束：x,y,z 世界坐标。由已观测机器人位置计算 yaw/pitch，启用心跳发送视角；返回请求状态，无自动寻路/穿墙保证。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.select`

```cpp
client.bot.select(nameOrIndex);
```

- 作用：切换后续 `game.*` / `world.*` 调用的目标机器人。
- 参数：`nameOrIndex` string 必填（机器人名或索引的字符串形式）。
- 返回：boolean `true`。
- 报错：未找到机器人抛 `bot.select: 未找到机器人 <名称>`。
- 底层：`selectBot(L, sel)`。

### `bot.self`

```cpp
client.bot.self();
```

- 功能、参数、返回与约束：无参数。读取当前账号机器人的服务器观测状态，health/position/dead 可未知。AI Agent 使用此机器人，不自行拥有一个游戏位置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.startMovement`

```cpp
client.bot.startMovement(dx, dy, dz);
```

- 功能、参数、返回与约束：dx,dy,dz 玩家输入方向，每轴 -1..1；以 50ms PlayerAuthInput 心跳交给服务器驱动运动。dx 左右，dz=-1 前进，dy 上下，不是世界坐标路径规划；返回 submitted=true,confirmed=false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.status`

```cpp
client.bot.status();
```

- 作用：返回每个机器人的连接状态、是否 OP、错误信息。
- 参数：无。
- 返回：对象或数组，1 起索引，每个元素 `{index, name, connected, is_op, error?}`（error 仅非空时存在）。
- 报错：无。
- 底层：`FleetStatus()`。

### `bot.stopMovement`

```cpp
client.bot.stopMovement();
```

- 功能、参数、返回与约束：无参数。清理当前机器人移动输入，仍需要服务器回包确认最终位置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `bot.teleport`

```cpp
client.bot.teleport(x, y, z);
```

- 功能、参数、返回与约束：x,y,z 世界坐标。执行服务器 tp @s 并等待真实命令结果，需要对应权限；没有权限时报错或 success=false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。


# 0.10.10 命令链修复与结构拆链

所有命令链在当前账号的机器人内预解析并长期复用，不在世界中放置命令方块。默认 mode=fast，rate_cps=0，delay_ms=0。Hz是整个TXT一次完整运行的目标轮次，不是每个节点的速率。过慢时不重入、不积压补跑。

## 发送与条件

无条件、同通道、无延迟的连续段一次交给底层，按原文順序发送；不存在Fast文件总行数上限或每批1024的限制。pipeline/burst/stream是用户明确选择的回包确认模式，它们的窗口只影响这些模式，不约束Fast。收到包的4096缓存与发送命令数量是两件事。

Fast设置逐条延迟，只做发送后等待，不自动要求回包。只有条件节点、它的直接前驱、@wait和@branch条件检测才要求真实CommandOutput。提交成功不作为条件成功。超时返回结果未知，不重发。服务器关闭sendcommandfeedback时条件/分支可能拿不到结果，应恢复反馈或使用可信事件。

## 基本格式与兼容

1,0,0为脉冲；2,0,0 20为目标整链20Hz；0,0,0为无条件连锁；0,1,0为有条件连锁。未重新声明属性沿用当前状态。`.`划分独立链并清空前一链执行结果；`!`结束当前周期。界面中的运行次数为完整TXT轮数，0持续。保留三位旧语法，红石位不模拟真实世界红石供电，不能把旧红石依赖当作已完全迁移。

旧函数执行器#组、#定义/#测试、#执行参数替换继续支持。通道ai/player/console和逐条延迟保持配置；需要结果依赖才主动切换确认通道。

## 子链调用、分支与断链

```text
say 开始
@branch {"command":"execute if score gate state matches 1","then":"技能开启.txt","else":"技能关闭.txt"}
@call {"name":"特效.txt","from_line":2,"to_line":8,"count":3}
@pause
say 已手动继续
@wait {"command":"testfor @a","poll_ms":500,"edge":true}
say 条件再次满足
```

@branch检测一次真实结果，只调用then或else一个文件，缺省分支可留空；子链结束回到下一行，不持续轮询。@call运行目标文件已编译IR指定源行范围，含两端；同账号，独立实例不会被悄悄启动。count=0必须有keep_open或duration_ms；持续调用结束于父链停止/时限。子链遵循自己的整链Hz/interval_ms，等待=周期减去本轮耗时，不再附加25毫秒下限；不创建递归任务。递归引用明确拒绝。

@wait默认先见失败、再等成功，edge=false可当前满足就继续；poll_ms控制检测频率。@pause仅等待手动trigger，UI“触发等待点”和插件chains.trigger(name)均能继续。

调用/分支不把方块转向、clone接线盲目翻译成命令。模块的状态与启停可由插件chains.start/stop/configure管理；事件如events.onSwing触发chains.start或sendCommandsFast，避免每Tick全员扫描。Lua/Python/C++均使用同一chains API，准确函数签名见各语言chains章节。

## 结构中的命令方块提取

工具→结构与指令转换→输出“提取命令方块·按朝向拆成独立链”。读取原有结构解析器保留的Command/auto/conditionalMode/TickDelay与facing_direction，箭头的下一格必须是连锁命令方块才接续；相邻但未指向的方块不合并。脉冲/循环是独立入口，闭环有限拆分并警告。下载JSON包含源坐标、NBT派生属性、连接索引和每个TXT完整原文；可一次保存全部未启动TXT，默认单轮。

接口POST /api/youyuan/convert/structure，data为文件base64，filename为文件名，target=command_chains，origin=[x,y,z]为结构原世界执行原点，rotation建议0。含~的命令用execute positioned原点+源位置包装，避免变成机器人坐标。含^还须审核原执行朝向；条件制约原方块背后关系、空节点、原生TickDelay、红石与相对执行上下文均需审核。

涉及command_block的setblock/fill/clone保留原文并列出警告；转换器不理解任意动态控制流，也不会删除真实tp/fill坐标。请核对后改为@call/@branch/chains启停。结构转任意完整玩法、自动消除坐标二分算法、通用神器效果引擎仍待实现；不把AI建议中的示意函数冒充已有API。

## 接收缓存与诊断

设置→服务器连接→收包缓存，默认4096，正整数自定义，下次重连生效；配置字段packet_queue_capacity=0采用4096。/api/bot/status返回packet_queue_depth及packet_queue_capacity。缓存是未处理包数量，不是CPS或链长度。短同步入队保序，满时对读包线程施加背压；断线/停用会解除等待，不逐包创建goroutine。

这次只做代码回归、构建和部署核验，未开展新的游戏压力测试。Fast不保证游戏服同Tick执行、原子性或固定性能倍率；既有大流量断线失败继续保留。


---
以下保留原有接口与格式参考；发送语义和新增修复以以上0.10.10说明为准。

# chains · 命令链管理

同一套TXT预解析、整链频率、启停、跨链与聊天菜单，可供插件调用。


### `chains.configure`

```cpp
client.chains.configure(name, patch);
```

- 功能、参数、返回与约束：name,patch，合并并校验配置，保留 TXT；运行中会停止并按 enabled 重启。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.delete`

```cpp
client.chains.delete_(name);
```

- 功能、参数、返回与约束：name，先停止再删除 TXT 与配置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.export`

```cpp
client.chains.exportPlugin(name);
```

- 功能、参数、返回与约束：name，返回完整 TXT 与配置，插件可自行写入导出目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.get`

```cpp
client.chains.get(name);
```

- 功能、参数、返回与约束：name，读取完整 TXT、配置和状态。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.handleChat`

```cpp
client.chains.handleChat(player, text);
```

- 功能、参数、返回与约束：player,text；处理 .命令选择 与 .链 列表/启动/停止/次数/频率/通道/源码/删除。修改要求玩家 OP，返回 handled。配合 events.onChat。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.list`

```cpp
client.chains.list();
```

- 功能、参数、返回与约束：无参数，列出当前账号的链、配置、计数、错误和状态。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.menu`

```cpp
client.chains.menu(player, page);
```

- 功能、参数、返回与约束：player,page=1，仅向该玩家发分页聊天菜单；不会阻塞等待消息。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.reload`

```cpp
client.chains.reload(name);
```

- 功能、参数、返回与约束：name；只重新读取/编译TXT资源，当前轮使用原IR，下轮用新IR；不重启插件、保留启停与统计；无效修改保留原链。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.save`

```cpp
client.chains.save(name, content, config);
```

- 功能、参数、返回与约束：name,content,config；config 支持 interval_ms(至少1，整链周期)/delay_ms/run_count/channel/enabled/mode/window/rate_cps。mode=fast无等待提交（条件及其前条确认）、remote严格顺序、pipeline有界流水线、burst整批最多1024、stream动态滑动窗口1..1024（默认128，rate_cps=0不设固定CPS，回包立即补位，网络错误停止补发；正常无目标不丢弃无条件后续）；全部在机器人内运行。TXT支持 @call JSON 跨链源文件行号范围/次数/时限/持续，@wait JSON 条件边沿等待，@pause手动触发；读 command-chain 技能。账号固定为当前插件账号。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.start`

```cpp
client.chains.start(name);
```

- 功能、参数、返回与约束：name，启用独立常驻实例；已启用不重复启动。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.stop`

```cpp
client.chains.stop(name);
```

- 功能、参数、返回与约束：name，停止并等待当前实例退出。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.trigger`

```cpp
client.chains.trigger(name);
```

- 功能、参数、返回与约束：name，释放当前链的 @pause 等待点一次；没有等待点时不会自动重启已停止的链。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.validate`

```cpp
client.chains.validate(content);
```

- 功能、参数、返回与约束：content，预解析并返回命令/条件/通道/行号；失败含报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。


# building · 建筑与颜色

结构格式检测、解析和方块颜色匹配。这些解析函数不等于放置整个建筑。


### `building.closestColor`

```cpp
client.building.closestColor(r, g, b);
```

- 作用：按 CIELAB 感知色差，匹配给定 RGB 最接近的颜色方块。
- 参数：`r`/`g`/`b` number 必填（0-255）。
- 返回：string（最接近的颜色方块名）。
- 报错：非数值抛类型错误。
- 底层：`media.ClosestBlockForColor(r, g, b)`。

### `building.detect`

```cpp
client.building.detect(path);
```

- 作用：检测建筑文件的格式。
- 参数：`path` string 必填。
- 返回：string（格式名：`.mcstructure` / `.schematic` / `.mcworld` / `.bdx`）。
- 报错：非字符串抛类型错误。
- 底层：`building.DetectFormat(path)`。

### `building.isAir`

```cpp
client.building.isAir(name);
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isCommandBlock`

```cpp
client.building.isCommandBlock(name);
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isLava`

```cpp
client.building.isLava(name);
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isWater`

```cpp
client.building.isWater(name);
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.parse`

```cpp
client.building.parse(path);
```

- 作用：加载并解析建筑结构文件，返回尺寸、方块统计、NBT、是否含命令方块。
- 参数：`path` string 必填。
- 返回：对象或数组 `{size={x,y,z}, blocks=number, nbt=number, has_command=boolean}`（`blocks` 为非空气方块数，`nbt` 为 NBT 方块数）。
- 报错：加载失败抛 `building.parse: <err>`。
- 底层：`building.LoadStructureFile(path)`，遍历方块计数，读 `NBTBlocks`/`HasCommand`。


# media · 图片与音乐

图片字符画、MIDI解析、音高换算；解析函数本身不自动向游戏播放。


### `media.imageToAscii`

```cpp
client.media.imageToAscii(path, maxW);
```

- 作用：把图片文件转成 ASCII 字符画行数组。
- 参数：`path` string 必填；`maxW` number 可选，默认 60。
- 返回：对象或数组（字符串数组，1 起索引，每行一个 ASCII 串）。
- 报错：打开/解码/转换失败抛 `media.imageToAscii: <err>`。
- 底层：`os.Open` + `image.Decode`（支持 gif/jpeg/png）→ `media.ImageToASCII(img, maxW)`。

### `media.noteToPitch`

```cpp
client.media.noteToPitch(note);
```

- 作用：把 MIDI 音符号换算为频率（Hz）。
- 参数：`note` number 必填（强转 `uint8`）。
- 返回：number（音高频率）。
- 报错：非数值抛类型错误。
- 底层：`media.NoteToPitch(uint8(note))`。

### `media.parseMidi`

```cpp
client.media.parseMidi(path);
```

- 作用：解析 MIDI 文件，返回格式/轨道/分辨率/速度及音符列表。
- 参数：`path` string 必填。
- 返回：对象或数组 `{format, tracks, division, tempo, notes={{tick, channel, note, velocity, duration, sound}}}`（`sound` 仅非空时出现）。
- 报错：读文件/解析失败抛 `media.parseMidi: <err>`。
- 底层：`os.ReadFile` → `media.ParseMIDIFile(data)`。


# plugin · 实例资源与诊断

目录、资源、能力与统计；代码、配置、持久数据分开管理。


### `plugin.capabilities`

```cpp
client.plugin.capabilities();
```

- 功能、参数、返回与约束：无参数，当前平台可用API名称和语言运行器能力；按需读取，不自动整库塞AI上下文。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.codeDir`

```cpp
client.plugin.codeDir();
```

- 功能、参数、返回与约束：无参数，当前插件源码根目录。Lua模块可require，模块只在该常驻VM首次require时加载。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.configDir`

```cpp
client.plugin.configDir();
```

- 功能、参数、返回与约束：无参数，当前插件配置目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.dataDir`

```cpp
client.plugin.dataDir();
```

- 功能、参数、返回与约束：无参数，当前插件持久数据目录；SDK按账号隔离。旧game.dataDir保持兼容。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.dependencies`

```cpp
client.plugin.dependencies();
```

- 作用：返回当前插件的依赖声明（`pre_plugins`）。
- 参数：无。
- 返回：表（原样透传全局 `plugin_dependencies` 的值）。
- 报错：无。
- 底层：读全局 `plugin_dependencies`。

### `plugin.existsResource`

```cpp
client.plugin.existsResource(relative);
```

- 功能、参数、返回与约束：relative_path，检查资源是否存在；非法路径报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.extractResource`

```cpp
client.plugin.extractResource(source, destination);
```

- 功能、参数、返回与约束：source_relative,destination_relative默认同源；将资源原字节复制至plugin.dataDir中的目标，创建父目录。不是解压ZIP，不写任意外部路径。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.has`

```cpp
client.plugin.has(method);
```

- 功能、参数、返回与约束：method或runtime.lua/runtime.python/runtime.cpp，查询宿主API是否提供；不代表服务器一定支持动作或已观测到血量。Linux 的 native C++ 常驻 ELF 运行器已开放（独立进程）；Android 同进程 native，Windows 未开放。以 plugin.capabilities() 的当前平台结果为准。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.id`

```cpp
client.plugin.id();
```

- 作用：返回当前插件的 id 字符串。
- 参数：无。
- 返回：string。
- 报错：无。
- 底层：读全局 `__PLUGIN_ID__`。

### `plugin.listResources`

```cpp
client.plugin.listResources(relative);
```

- 功能、参数、返回与约束：relative_path默认.，递归列出目录和文件，含path/dir/size；不跟随符号链接。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.readResource`

```cpp
client.plugin.readResource(relative, encoding);
```

- 功能、参数、返回与约束：relative_path,encoding=utf8；读取资源字符串；二进制跨JSON使用encoding=base64。拒绝绝对路径、越界与符号链接。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.resourceDir`

```cpp
client.plugin.resourceDir();
```

- 功能、参数、返回与约束：无参数；有resources子目录则使用该目录，否则兼容使用代码目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.stats`

```cpp
client.plugin.stats();
```

- 功能、参数、返回与约束：无参数，当前常驻实例回调数、失败、墙钟耗时及队列深度/丢弃；CPU/内存/GC当前未逐实例测量，返回null，不虚构性能倍数。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.tempDir`

```cpp
client.plugin.tempDir();
```

- 功能、参数、返回与约束：无参数；插件数据目录下.tmp，可自行删除临时文件。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。


# util · 文件、时间与调度

JSON文件、文本、数学、时间与整个回调的定时任务。


### `util.cancel`

```cpp
client.util.cancel(id);
```

- 作用：取消一个已注册的定时任务（schedule/setTimeout）。
- 参数：`id` number 必填。
- 返回：boolean——`true` 找到并取消，`false` 不存在。
- 报错：无。
- 底层：`cancelSchedJob(int64(id))`，关闭任务的 cancel channel。

### `util.ceil`

```cpp
client.util.ceil(n);
```

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（math.Round：正负0.5均远离0取整）。

### `util.floor`

```cpp
client.util.floor(n);
```

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（math.Round：正负0.5均远离0取整）。

### `util.join`

```cpp
client.util.join(tbl, sep);
```

- 作用：把 对象或数组 的所有值转字符串后用分隔符拼接。注意：遍历整表，非数组顺序不一定与下标一致。
- 参数：`tbl` 对象或数组、`sep` string 必填。
- 返回：string。
- 报错：无。
- 底层：`t.ForEach` + `LVAsString` + `strings.Join`。

### `util.json_load`

```cpp
client.util.json_load(path);
```

- 作用：读取文件并解析为 JSON 对象。
- 参数：`path` string 必填。
- 返回：任意 JSON兼容值（表/数字/字符串…）。数字用 `UseNumber()` 解码避免精度损失。
- 报错：读文件失败或 JSON 解析失败抛 `json_load: <err>`。
- 底层：`os.ReadFile` + `json.Decoder`（`UseNumber`）→ `goToLua`。

### `util.json_write`

```cpp
client.util.json_write(path, data);
```

- 作用：把任意 JSON兼容值序列化为带缩进的 JSON 写入文件，自动建父目录。
- 参数：`path` string 必填；`data` 任意类型必填。
- 返回：boolean `true`。
- 报错：建目录/序列化/写文件失败抛 `json_write: <err>`。
- 底层：`os.MkdirAll` + `json.MarshalIndent(data,"","  ")` + `os.WriteFile`。

### `util.listJobs`

```cpp
client.util.listJobs();
```

- 功能、参数、返回与约束：无参数，当前插件任务列表：id/handler/paused/calls/missed/failures/last_ms/max_ms/total_ms/last_error；耗时为墙钟含VM排队及回调，不是CPU时间。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `util.now`

```cpp
client.util.now();
```

- 作用：返回当前本地时间。
- 参数：无。
- 返回：string，格式固定 `"2006-01-02 15:04:05"`。
- 报错：无。
- 底层：`time.Now().Format("2006-01-02 15:04:05")`。

### `util.pauseJob`

```cpp
client.util.pauseJob(id);
```

- 功能、参数、返回与约束：id，暂停该插件任务后续触发；已经执行/入队的一次不强制中断。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `util.pos`

```cpp
client.util.pos(str);
```

- 作用：把空格分隔的坐标字符串（如 `"100 64 200"`）解析成坐标表，最多取前 3 个 token，缺失项补 0。
- 参数：`str` string 必填。
- 返回：对象或数组 `{x=number, y=number, z=number}`。
- 报错：无。
- 底层：`strings.Fields` + `strconv.ParseFloat`。

### `util.read_file`

```cpp
client.util.read_file(path);
```

- 作用：读取文件全部内容按字符串返回。
- 参数：`path` string 必填。
- 返回：string。
- 报错：读取失败抛 `read_file: <err>`。
- 底层：`os.ReadFile`。

### `util.resumeJob`

```cpp
client.util.resumeJob(id);
```

- 功能、参数、返回与约束：id，恢复后续触发；不会补发暂停期间的积压轮次。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `util.round`

```cpp
client.util.round(n);
```

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（math.Round：正负0.5均远离0取整）。

### `util.schedule`

```cpp
client.util.schedule(intervalSec, handlerName);
```

- 作用：按秒调用整个回调，周期0.001秒至1年。
- 参数：`intervalSec, handlerName`；按此顺序传入。
- 返回：定时任务ID；util.cancel(id)取消。
- 底层与游戏效果：源码与模块缓存复用；同任务不重入，不补无限积压。回调里整条链的所有命令连续提交，不是每次只发一条。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.scheduleHz`

```cpp
client.util.scheduleHz(hz, handler);
```

- 功能、参数、返回与约束：hz,handler；本地完整回调的频率，0<hz<=1000。慢回调不重入、不补积压轮次；统一时钟、skip策略；查看listJobs的missed。不是服务端同步Tick。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.scheduleSeconds`

```cpp
client.util.scheduleSeconds(seconds, handler);
```

- 功能、参数、返回与约束：seconds,handler；schedule别名，0.001秒..一年。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.scheduleTick`

```cpp
client.util.scheduleTick(ticks, handler);
```

- 功能、参数、返回与约束：ticks,handler；ticks>=1，统一宿主Tick Loop；本地名义Tick=50ms，不宣称服务端同Tick同步。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.setTimeout`

```cpp
client.util.setTimeout(sec, handlerName);
```

- 作用：延时后调用一次回调，可设0秒。
- 参数：`sec, handlerName`；按此顺序传入。
- 返回：定时任务ID。
- 底层与游戏效果：只调用已加载的函数，停用或重载自动取消所属定时任务。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.split`

```cpp
client.util.split(str, sep);
```

- 作用：按分隔符切分字符串为数组。
- 参数：`str` string、`sep` string 必填。
- 返回：对象或数组（字符串数组，1 起索引）。
- 报错：无。
- 底层：`strings.Split` + `RawSetInt`。

### `util.strip_color`

```cpp
client.util.strip_color(text);
```

- 作用：去掉文本里的 Minecraft 颜色/格式码（`§` 与 `&` 及其后一个字符）。
- 参数：`text` string 必填。
- 返回：string。
- 报错：无。
- 底层：手写循环用 `strings.Builder` 逐字节过滤。

### `util.time_fmt`

```cpp
client.util.time_fmt(ts);
```

- 作用：把 Unix 时间戳格式化为可读的本地时间。
- 参数：`ts` number 可选，默认当前时间戳。
- 返回：string。
- 报错：无。
- 底层：`time.Unix(ts,0).Format("2006-01-02 15:04:05")`。

### `util.timestamp`

```cpp
client.util.timestamp();
```

- 作用：返回当前 Unix 秒级时间戳。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：`time.Now().Unix()`。

### `util.trim`

```cpp
client.util.trim(str);
```

- 作用：去除字符串两端空白。
- 参数：`str` string 必填。
- 返回：string。
- 报错：无。
- 底层：`strings.TrimSpace`。

### `util.write_file`

```cpp
client.util.write_file(path, content);
```

- 作用：把字符串写入文件，自动建父目录，权限 0644。
- 参数：`path` string、`content` string 必填。
- 返回：boolean `true`。
- 报错：建目录/写文件失败抛 `write_file: <err>`。
- 底层：`os.MkdirAll` + `os.WriteFile(path, []byte(content), 0644)`。

### `util.monotonicMs`

无参数，进程启动以来单调时间毫秒，适合间隔计算；不是Unix时间。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `util.serverTick`

返回known/tick/source/updated_at；仅观测UpdateAttributes的真实非零tick，不以本地50ms计时冒充服务器tick。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。


# api · 跨插件调用

通过plugin_api导出函数；Lua、Python、C++可相互调用已启动实例。


### `api.require`

```cpp
client.api.require(id);
```

- 作用：获得跨插件调用代理；目标需已经加载并导出plugin_api。
- 参数：`id`；按此顺序传入。
- 返回：Lua代理表或nil；Python代理对象；C++yy::RemotePlugin。
- 底层与游戏效果：Lua→Lua调用同一常驻实例，不重跑源码；跨语言通过序列化桥调用目标导出函数。API不是安装依赖的包管理器。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。


# http · 网络与JSON

返回status/body/headers，非2xx响应也可检查，不把远端错误吞掉。


### `http.delete`

```cpp
client.http.delete_(url, opts);
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.get`

```cpp
client.http.get(url, opts);
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.json_decode`

```cpp
client.http.json_decode(s);
```

- 作用：把 JSON 字符串解析为 JSON兼容值。
- 参数：`s` string 必填。
- 返回：任意 JSON兼容值。
- 报错：解析失败抛 `http.json_decode: <err>`。
- 底层：同 `game.jsonDecode`——把 JSON 解析成对应 Lua 类型（对象→表、数组→1 起索引的表）。

### `http.json_encode`

```cpp
client.http.json_encode(v);
```

- 作用：把 JSON兼容值转成 JSON 字符串。
- 参数：`v` 任意 JSON兼容值。
- 返回：string。
- 报错：序列化失败抛 `http.json_encode: <err>`。
- 底层：同 `game.jsonEncode`——把 JSON兼容值映射成 JSON 类型后编码成字符串。

### `http.post`

```cpp
client.http.post(url, body, opts);
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.put`

```cpp
client.http.put(url, body, opts);
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.querify`

```cpp
client.http.querify(paramsTable);
```

- 作用：把参数表编码成 URL query 字符串。
- 参数：`paramsTable` 对象或数组 必填（键值都转字符串）。
- 返回：string，如 `"a=1&b=2"`（按键排序，同键累加）。
- 报错：非 对象或数组 抛类型错误。
- 底层：`url.Values` + `vals.Encode()`。

> 与 `game.httpGet/httpPost` 的区别：`game.*` 返回纯文本、非 200 直接抛错，适合简单场景；`http.*` 返回完整响应对象、携带状态码与响应头、非 2xx 也能拿 body，适合需要细粒度控制的场景。

### `http.request`

```cpp
client.http.request(method, url, body, opts);
```

- 作用：发送任意 method 的 HTTP 请求，返回完整响应对象。
- 参数：`method` string 必填；`url` string 必填；`body` string 可选，默认 `""`；`opts` 对象或数组 可选（含 `headers` 表、`timeout` 秒）。
- 返回：对象或数组。成功 `{ok=true, status=number, body=string, headers=对象或数组}`；传输错误 `{ok=false, error=string}`。
- 报错：无（错误放进 `error` 字段，不抛异常）。
- 底层：`httpRequestFull(method, u, body, headers, timeout)`。


# system · Android设备

系统桥、通知、震动、Shizuku；桌面支持范围与Android不同。


### `system.call`

```cpp
client.system.call(method, paramsTable);
```

- 作用：通用底层透传——把 `method` 和参数表交给 Java 分发执行，返回任意结果。已支持的方法：`shell`、`shizukuStatus`、`shizukuRequest`、`vibrate`、`notify`、`launchApp`；未知方法返回 `{"ok":false,"error":"unknown method: ..."}`。
- 参数：`method` string 必填；`paramsTable` 对象或数组 可选。
- 返回：任意 JSON兼容值（Java 返回 JSON 解码结果）；无返回推 `nil`。
- 报错：参数序列化失败或桥调用失败抛 `system.call: <err>`。
- 底层：参数 `json.Marshal` → `callAndroidSystem` → JNI → Java `FloatingBridge.call` switch 分发。

### `system.notify`

```cpp
client.system.notify(title, body);
```

- 作用：发系统状态栏通知（**不需要** Shizuku/ADB 权限）。
- 参数：`title` string 必填；`body` string 可选，默认 `""`。
- 返回：对象或数组 `{ok=true}`；异常时 `{ok=false, error=...}`。
- 报错：桥调用失败抛 `system.notify: <err>`。
- 底层：参数 `{"title","body"}` → JNI → Java `postDirectNotification`。

### `system.platform`

```cpp
client.system.platform();
```

- 作用：返回当前运行平台。
- 参数：无。
- 返回：string——`"android"` 或 `"desktop"`。
- 报错：无。
- 底层：`getPlatformName()`（不经 JNI 桥）。

### `system.shell`

```cpp
client.system.shell(cmd);
```

- 作用：以 Shizuku(ADB) 身份执行一条 shell 命令。
- 参数：`cmd` string 必填。
- 返回：对象或数组 `{ok=bool, exit=number, stdout=string, stderr=string, uid=number, error?=string}`。命令本身失败不抛异常，走 `ok=false`。
- 报错：仅桥调用失败抛 `system.shell: <err>`。
- 底层：参数 `{"command":cmd}` → JNI → Java `runShell`：空命令/Shizuku 未运行/未授权各有明确错误；否则经反射调 `Shizuku.newProcess` 执行 `/system/bin/sh -c <cmd>`，读 stdout/stderr、取退出码与 uid。

### `system.shizukuRequest`

```cpp
client.system.shizukuRequest();
```

- 作用：弹出系统授权对话框，申请 Shizuku ADB 权限。
- 参数：无。
- 返回：对象或数组 `{ok=bool, requested=bool, error?=string}`。
- 报错：桥调用失败抛 `system.shizukuRequest: <err>`。
- 底层：JNI → Java `requestShizukuPermission`：pingBinder 失败返回未运行；否则 `Shizuku.requestPermission()` 弹授权框。

### `system.shizukuStatus`

```cpp
client.system.shizukuStatus();
```

- 作用：查询 Shizuku 运行状态与授权情况。
- 参数：无。
- 返回：对象或数组 `{available=bool, granted=bool, uid=number}`（失败时 uid=-1）。
- 报错：桥调用失败抛 `system.shizukuStatus: <err>`。
- 底层：JNI → Java `shizukuStatusJson`：`Shizuku.pingBinder()`、`checkSelfPermission()==GRANTED`、`getUid()`。

### `system.vibrate`

```cpp
client.system.vibrate(ms);
```

- 作用：触发手机震动（**不需要** Shizuku/ADB 权限）。
- 参数：`ms` number 可选，默认 500（毫秒）。
- 返回：对象或数组 `{ok=true}`；异常时 `{ok=false, error=...}`。
- 报错：桥调用失败抛 `system.vibrate: <err>`。
- 底层：参数 `{"ms":ms}` → JNI → Java `vibrate`（Vibrator）。


# web · 插件控制页面

声明式页面、HTML资源、动作回调与事件推送，重载清理旧注册。


### `web.action`

```cpp
client.web.action(page, action, handler);
```

- 作用：绑定一个交互动作——浏览器对 `/web/api/<插件id>/<页面名>` 发请求时，调用常驻实例的 `handler` 处理。
- 参数：`page` string、`action` string 必填；`handler` **string 必填**（全局**函数名字符串**，不是函数引用）。
- 返回：无。
- 报错：缺插件上下文/空名/重复注册抛异常。
- 底层：`registerWebAction`；执行时新建 LState（2 分钟超时）进入常驻实例后 `CallByParam` 调 handler，返回值经 `luaToGo` 作为响应 `data` 回传。受独立并发信号量（容量 8）约束。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `web.emit`

```cpp
client.web.emit(page, event, data);
```

- 作用：把一条事件实时推送给该插件的全部 SSE 客户端。声明式页面内置 JS 按 key 更新控件；自定义页 JS 自行监听。用于后台状态/进度推送。
- 参数：`page` string 必填（作为 SSE 事件名）；`event` string 必填（会写入数据 map 的 `__event` 键）；`data` 任意 JSON兼容值必填。
- 返回：无。
- 报错：数据无法 JSON 序列化抛 `web.emit: <err>`。
- 底层：`pushWebEvent(plugin, page, dataJSON)`，构造 SSE 消息写入所有客户端通道（通道满则丢弃，不阻塞 Lua）。**此方法在重跑上下文不跳过**（供 handler 内推送用）。

### `web.page`

```cpp
client.web.page(name, desc, controls);
```

- 作用：声明一个**声明式页面**，在门户 `/web/` 显示卡片，点进由内置模板渲染控件页。
- 参数：`name` string 必填（插件内唯一，不含 `/ \ # ? %` 及控制字符）；`desc` string 可选，默认 `""`；`controls` 对象或数组 可选（控件表，可读字段 `type`=text|button|input|对象或数组|switch、`key`、`label`、`action`、`placeholder`、`columns`）。
- 返回：无。
- 报错：空名/非法字符/同插件页面名重复注册抛异常。
- 底层：`registerWebPage(&webPage{Kind:"declarative",...})`，写入全局 `webPages` map。

### `web.serve`

```cpp
client.web.serve(name, entry);
```

- 作用：声明一个**自定义 HTML 页面**，直接服务 `www/` 目录下对应入口文件。
- 参数：`name` string 必填（规则同 `web.page`）；`entry` string 可选，默认 `"index.html"`（相对 `www/`）。
- 返回：无。
- 报错：同 `web.page`。
- 底层：`registerWebPage(&webPage{Kind:"html", Entry:entry})`；静态根目录 `插件文件/<id>/www`，经 `webStaticSafe` 防路径逃逸 + `serveWebStatic` 提供。


# storage · 缓存与事务

内存键值、原子加分、自动持久化与条件事务。


### `storage.atomicAdd`

```cpp
client.storage.atomicAdd(key, delta);
```

- 功能、参数、返回与约束：key,delta=1；持锁原子加，默认从0开始；非数字报错，不发生读改写竞争。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.delete`

```cpp
client.storage.delete_(key);
```

- 功能、参数、返回与约束：key；删除缓存键，后台持久化。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.flush`

```cpp
client.storage.flush();
```

- 功能、参数、返回与约束：无参数；同步写临时文件、Sync、原子替换storage.json；失败报真实路径错误并保留dirty。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.get`

```cpp
client.storage.get(key, default);
```

- 功能、参数、返回与约束：key,default?；内存读取并复制值，账号/插件的数据目录独立；首次读取storage.json一次。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.set`

```cpp
client.storage.set(key, value);
```

- 功能、参数、返回与约束：key,value；复制JSON可表示的值到内存，后台约1秒自动原子持久化；重要提交用flush或transaction。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.transaction`

```cpp
client.storage.transaction(operations);
```

- 功能、参数、返回与约束：operations数组；op=set/add/delete/check,key,value。check值不符则整批回滚；整批验证后持锁同步持久化成功才替换缓存。不是任意Lua函数/跨文件事务。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。


# 跨账号联通开发说明 · 0.10.11 / SDK1.14

这是同一个游园控制端内，主账号 main 与 managed-accounts 独立账号之间的协作 API。账号目录、令牌、插件和事件不自动混用；授权只开放指定操作。不是跨不同平台用户或另一台电脑的匿名接口。不同设备请使用已认证的远端 MCP 各自连接。

## 开放授权

设置 → 账号联通与连接授权 → 跨账号联通。选择源账号、目标账号、源插件 ID 与操作，添加并保存。源插件填 `*` 是该源账号全部插件，建议填写具体 ID。A→B 不自动授予 B→A；撤销后后续调用立即拒绝，正在执行的命令不会撤回。只允许运行中的目标账号，接口不会偷偷启动或替换账号。

规则 API：`GET /api/youyuan/accounts/bridge/rules` 查看；`POST` 同路径提交完整规则数组（覆盖替换，不是追加），例如：
```json
[{"source":"main","plugin":"my_lua","target":"account_b","operations":["status","sendMessage","sendCommandsFast","callPlugin"]}]
```

## 五个接口

| 方法 | 参数 | 行为 |
|---|---|---|
|accounts.list|无|账号 ID、名称、启用状态；不暴露令牌|
|accounts.status|target|实际目标连接状态；需 status 授权|
|accounts.sendMessage|target,text|目标机器人普通聊天；斜杠指令明确拒绝|
|accounts.sendCommandsFast|target,commands,options?|完整数组按序 Fast 提交；无逐条确认|
|accounts.callPlugin|target,pluginID,function,args数组|调用目标常驻 plugin_api 导出，返回结构化结果|

target 必须是 accounts.list 返回的 ID，不是显示昵称、端口或QQ号。options 复用 game.sendCommandsFast 的参数，包括 channel。数组提交数量不代表游戏执行成功；Fast 不保证同一服务器 Tick、事务原子性或网络失败重试。20秒跨账号控制请求超时后不得盲目重发有副作用操作。

### Lua
```lua
local profiles = accounts.list()
local state = accounts.status("account_b")
accounts.sendMessage("account_b", "联通测试")
accounts.sendCommandsFast("account_b", {
  "scoreboard players add @s yytest 1",
  "scoreboard players add @s yytest 1"
}, {channel="ai"})
local result = accounts.callPlugin("account_b", "target_plugin", "balance", {"Player"})
```
目标 Lua 插件只加载一次，导出例子：
```lua
plugin_api = plugin_api or {}
function plugin_api.balance(player) return {player=player,value=10} end
```

### Python
```python
profiles = accounts.list()
state = accounts.status("account_b")
accounts.sendCommandsFast("account_b", ["say A", "say B"], {"channel":"ai"})
result = accounts.callPlugin("account_b", "target_plugin", "balance", ["Player"])
```
Android 与 Linux 驻留宿主均提供 accounts 命名空间；异步插件请在自己的任务中 await host 提供的异步包装或 asyncio.to_thread，勿占住高频 Tick 回调等待远端返回。

### C++（Android / Linux 原生宿主）
```cpp
auto profiles = client.accounts.list();
auto state = client.accounts.status("account_b");
client.accounts.sendCommandsFast("account_b", yy::json::array({"say A","say B"}), yy::json{{"channel","ai"}});
auto result = client.accounts.callPlugin("account_b","target_plugin","balance",yy::json::array({"Player"}));
```
Linux 0.10.19 已接入独立常驻原生子进程；需要对应 Linux 架构的 ELF 库和 Python3 传输桥。不要把安卓版 .so 当成 Linux 插件。Windows 不能直接加载 ELF。

## 防止互相等待

跨账号 callPlugin 的导出回调里，禁止再同步调用 accounts.*，避免 A 等 B、B 又等 A 的环路。应先返回结果，再通过 schedule/setTimeout 发下一次协作。阻断在当前进程级生效；同一时段其他插件的跨账号调用可能被拒绝，应延后任务。拒绝信息可供 AI 查错，不重跑插件源码。

这是协调授权，不是对恶意本地代码的安全沙箱：本机插件本就有系统/HTTP权限。云端仍依赖平台用户和设备隔离；不能给其他订阅用户分配你的私有节点后又期望他们无法控制此节点。

## 远端与内置 AI

MCP api_youyuan_accounts_bridge_rules 管理规则；api_youyuan_accounts_bridge_execute 调用 `{source,plugin,target,operation,args}`，属于控制权限。内置 AI 可按需加载本技能，读取接口目录和错误；不能只知道插件名称。内部 `/bridge/local` 不列入 MCP，不允许通过账号代理嵌套绕过授权。

## 验证范围

代码回归验证定向授权、源插件限制、操作限制与接口注册；发布验收验证真实进程之间状态调用。没有在本轮新增游戏压测，发消息、游戏命令副作用及跨语言目标插件的游戏联调留待玩家批准恢复测试。


# entities / cpp-runtime

本章使用游园真实宿主绑定；先通过known/stale判断观测，不编造游戏完成状态。


### `entities.attributes`

target；返回服务器同步的属性字典，value/min/max/updated_at/source；armor仅服务器同步minecraft:armor时已知，不按穿戴猜护甲点数。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `entities.get`

runtime_id十进制字符串或玩家名；服务器观测快照，含身份/位置/health结构/attributes/装备/视角/present/stale。未收到为known=false，不扫描全世界。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `entities.health`

同entity.health，读取服务器UpdateAttributes/SetHealth缓存，不发送查询命令。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `entities.nearby`

position=[x,y,z],radius=32,dimension?；已观测、仍存在且位置未过期的实体；不查询不可见区域。位置省略或self时使用机器人实际位置。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。


# entity / cpp-runtime

本章使用游园真实宿主绑定；先通过known/stale判断观测，不编造游戏完成状态。


### `entity.health`

target=self或runtime_id字符串；current/max/previous/damage/known/max_known/stale/updated_at/source。最大生命未知为null，绝不假定20。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `entity.onDamage`

同events.onEntityHurt；血量下降确认伤害，只有ActorEvent时数量未知，不能由最近玩家或挥臂猜攻击者。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。


# diagnostics / cpp-runtime

本章使用游园真实宿主绑定；先通过known/stale判断观测，不编造游戏完成状态。


### `diagnostics.combat`

target；返回保留事件中的伤害/血量/死亡/击杀，含同一event_id的补充证据。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `diagnostics.configure`

config?；宿主持久配置：event_queue/priority_queue/runtime_queue/async_tickets/event_history/actors各1..65536，stale_ms=1..86400000。事件队列变更在插件重载时生效，其余立即影响后续操作。不是命令CPS限速，也不是包种类上限。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `diagnostics.entity`

target；同entities.get，额外完整字段来源及过期信息。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `diagnostics.stats`

当前账号解码包计数、最后包时间、实体数、事件缓冲及配置、断线/踢出诊断和整个宿主内存/GC；非单插件内存。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `diagnostics.trace`

event_id或sequence字符串；查询保留的事件及关联死亡/受伤详情；被覆盖返回known=false，不虚构历史。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。


# hud / cpp-runtime

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


# scoreboard / cpp-runtime

本章使用游园真实宿主绑定；先通过known/stale判断观测，不编造游戏完成状态。


### `scoreboard.get`

objective,player；读取SetScore已同步条目，未广播known=false，不同步查询全服。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `scoreboard.increment`

objective,player,amount整数；提交原生scoreboard add/remove，不本地读改写，避免竞态；提交不等于服务器成功。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `scoreboard.set`

objective,player,value整数；提交计分板设置，submitted=true/confirmed=false，调用Confirmed命令或同步SetScore才能验证结果。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。


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

 容器NBT与分区协作

完整参数、NBT例子、副作用和20种制作行为见技能 `nbt-workbench`，可按需读取 `youyuan://docs/nbt-workbench`。

### `building.nbtCapabilities`

查询底层支持方块、复杂物品和20种行为，支持表不等于全部实测。

### `building.normalizeNBT`

NBT对象；恢复Count/Slot/ench/Damage整数宽度，保留未知字段，拒绝越界和null。

### `building.containerData`

name,Items,options?；本地预检物品类型和实际槽位，不操作游戏。

### `building.placeContainer`

x,y,z,name,Items,options?；异步制作带名称和附魔的容器；返回job_id，需nbtJob读回verified。首次工作台初始化清机器人背包、切创造、移动并建地板。

### `building.nbtJob`

job_id,consume=false；当前账号任务状态和结果。consume只删除已完成任务；重启不恢复。

### `building.planRegion`

起点,尺寸；16×128×16完整覆盖规划，不是游戏实际执行。

### `building.exportRegion`

起点,尺寸,{workers=1}?；异步保存本机mcstructure。只用已配置同服Fleet，实际workers_used；缺块失败不输出成功文件。

可通过 `client.invoke(method, args)` 或新命名包装调用；Linux原生C++运行器未实现。


## 0.10.24 实际视距与收包诊断



### `diagnostics.visibility()`

返回当前账号连接的 `requested_radius_chunks`、`granted_radius_chunks`、时间及来源，`publisher.position/radius_blocks`，选中解码包计数，收包队列占用，抓包覆盖与抽样统计。未知半径为 null。`entity_broadcast_radius_known=false` 明确表示实体范围没有被服务器告知。原 `packet_counts` 只统计消费者已处理的包，不代表网络上所有数据包。源码诊断补丁新增 `ingress.types[类型]` 的 `decoded/selected/filtered/enqueued/cancelled`：依次为监听筛选前收到、被选中、被筛选、已投递、断线取消。`Unknown:ID` 区分未实现字段解码的不同包ID。仅覆盖监听注册后进入监听器的包，不含登录阶段被内核消费的包或解码失败；实时读取不是事务快照，selected可能尚在等待投递。没有逐包正文保存、抽样、写盘或新增发送限速。重连后半径与统计重建。**该ingress补丁尚未发布到0.10.24安装包或线上Linux；只有更新此源码构建才有此字段。**

### `bot.requestViewDistance(chunks, timeout_ms=5000)`

发送当前内核的 RequestChunkRadius，同时填写 ChunkRadius 和 MaxChunkRadius，等待真实 ChunkRadiusUpdated。参数 chunks 允许协议正 int32（1..2147483647），没有额外 32 区块上限；这个类型范围不等于建议运行值或服务器允许值。用户按资源情况从 16、32、64 逐步请求，查看实际批准值。大幅提高半径会增加区块数据和内存，不能靠发送极大整数获得全世界实时事件。

返回 submitted、confirmed、requested_radius_chunks、granted_radius_chunks、clamped。超时为 confirmed=false / granted_radius_chunks=null。confirmed 只确认半径回包，不确认实体广播范围。协议不带请求ID，宿主串行此高层方法；同时通过其他 raw 发包调用更改半径，无法严格关联响应，应避免。该协商只影响本次连接；不会改变命令链 Hz 或 Fast 命令限速。

Lua：

```lua
local before = diagnostics.visibility()
local result = bot.requestViewDistance(32, 5000)
print(game.jsonEncode(result))
events.onHealthChange(function(e)
    -- e.source 为真实包；未收到其他实体的新属性不能声称已知其当前血量。
    print(game.jsonEncode(e))
end)
events.onSwing(function(e) print(e.player, e.runtime_id, e.source) end)
```

Python：`host.bot.requestViewDistance(32, 5000)` 和 `host.diagnostics.visibility()`。Python 的 host 为当前插件 SDK 客户端，参数/结果与 Lua 相同。

C++：`client.bot.requestViewDistance(32, 5000)` 和 `client.diagnostics.visibility()`；Android 支持原生运行器，Linux 原生 C++ 运行器尚未支持。MCP/AI 通过实际 `api_youyuan_game_call` 或带 plugin_id 的 SDK 调用相同方法，完整技能可按需读取 `youyuan://docs/network-observation`。


完整限制与扩大覆盖方案见可选技能 network-observation。


### 入站 PyRpc 事件目录诊断（源码待发布）

显式启动包含 `PyRpc` 的抓包后，读取 `diagnostics.visibility().ingress.rpc_catalog`。
它只整理实际收到的 `ModEventS2C` 标准帧，返回 `events[]` 的 `namespace/system/event/operation_type/count/first_seen_at/last_seen_at`。
MessagePack 的字符串和二进制字符串名称均支持。不会复制事件参数到目录，不发试探请求，不改变原始包与插件订阅。
其他帧形态只增加 `unrecognised`，表示目录解析器不识别，不表示协议解码失败或服务器没有对应能力；原始抓包仍可查看。

目录容量使用 `diagnostics.configure` 的 `event_history`（1..65536），按首次发现顺序移除旧事件身份；`evicted` 显示移除次数。容量限制的是诊断目录，不是接包数量、命令频率或游戏事件广播。
一个事件移除后再次出现，其留存 `count` 重新从1开始；`observed` 是此次连接、发现采集启用期间的累计RPC数。
停止抓包后停止采集，已发现目录保留到当前游戏连接重置，时间戳使用Unix毫秒；相同名字但不同命名空间/系统/OperationType分别计数。

收到事件不能证明同名服务端处理函数可被调用，不能据此自动生成“全服挥臂”接口。
`ListenForEvent` 注册所在引擎/模块的本地监听；`NotifyToServer` 请求必须由服务器的实际处理器接收。
玩家输入代理、服务端事件转发与普通机器人收到的同步包要保留不同来源，攻击者、维度、血量未知时不编造。
此目录属于尚未部署的源码补丁，已发布0.10.24安装包和线上Linux尚无此新增字段。




## 0.10.25 接收交接与采集桥

登录阶段使用同一个同步读包者完成认证，保留应用层 ModEventS2C；先安装宿主监听器再回放缓存和启动实时读取。认证挑战本身不重复投递。缓存满时明确报错，不默默丢包。`diagnostics.visibility()` 的 `login_ingress` 与 `ingress` 是两个有重叠的阶段，不能相加计算总包数。后者含登录缓存回放；前者只保留计数及 RPC 事件名称，不保留认证参数。

新增命名空间 `observerBridge`，Lua、Python 和 C++ 通用调用同一宿主实现；插件仍须满足宿主授权配置。所有事件带来源，采集客户端报告不会写入服务器属性缓存。

| API | 参数与作用 |
|---|---|
| create | `{name, source="player_input"或"game_server", server, player_id}`；玩家来源必须绑定身份。返回独立密钥一次，存储 SHA256 摘要。 |
| sources | 无参数。来源清单、序号及最后上报时间，不返回密钥。 |
| revoke | `source_id`；撤销密钥。 |
| events | `after=0, limit=256`；分页读取采集桥事件与覆盖数。历史容量沿用可配置 EventHistory，重启清空内存历史，来源序号持久化。 |
| attachSelf | `source_id, token`；采集当前机器人自身加密前的本地提交。server 与 UUID/XUID/名称必须匹配。异步队列容量沿用 EventQueue，50ms 聚合投递；不改变游戏命令频率。 |
| inputStatus | 无参数。active/submitted/dropped/last_error/queue_capacity；submitted 是采集桥已接收，不是游戏执行成功。 |
| detachSelf | 无参数。停止自身采集，不断开机器人。 |
| rpcProbe | `timeout_ms=3000`；发送 YouyuanBridge/ObserverV1/Hello，只有匹配 request_id 的 Capabilities 才确认。 |
| rpcQuery | `{query,target?}, timeout_ms=3000`；只查询已经安装的服务端 Provider。没有模块则没有凭空读取能力。 |
| rpcEvents | `after=0,limit=256`；读取自定义 Observation，来源 server_module_reported，重连清空，独立于原生实体缓存。 |

Lua 示例：
```lua
local state = bot.self()
-- server 与 player_id 使用当前连接的真实服务器和身份，不填示例身份。
local source = observerBridge.create({name="自身采集",source="player_input",server="94F195C41B5",player_id="当前机器人名称"})
observerBridge.attachSelf(source.id, source.token)
local status = observerBridge.inputStatus()
local received = observerBridge.events(0,64)
observerBridge.detachSelf()
observerBridge.revoke(source.id)
```
Python 常驻 Host：`host.observerBridge.events(0,64)`；C++ 使用 `client.observerBridge.events(0,64)` 或通用 `client.invoke("observerBridge.events", yy::json::array({0,64}))`。密钥不写入公开文档、日志或插件发布包。

入站真实血量继续从 `UpdateAttributes.attributes[].attribute_value` 的 `minecraft:health` 更新 `entities.health(runtimeId)`；未收到值保持未知。`bot.swingAir()` 在下一正常输入心跳加入一次 PlayerAuthInput.MissedSwing，复用位置与视角，输入 Tick 递增并在重连时归零；重复排队请求合并。心跳关闭时拒绝请求，导入导出任务占用时等待心跳恢复。queued/submitted/confirmed 分开，不能把输入当服务器已接受。直接发送旧式 Animate 在本次网易服测试中导致第二机器人断线，bot.attack 已去掉该发送。攻击输入来自 InventoryTransaction 的 UseItemOnEntity/Attack，仍不证明服务器扣血。

认证交接后若服务器再次发送 StartType/GetMCPCheckNum，由同一个运行期读包者转交挑战处理器，已通过的认证不重复提交。该修正经0.10.24/0.10.25同账号对照定位，修正后 list/summon/tag/damage/kill 获服务器确认。

外部客户端/代理把真实事件摘要作为 JSONL 输入 `observer-bridge/agent.py`，POST `/api/youyuan/observer/ingest`，请求头 `X-Youyuan-Observer-Token`。该程序是认证上报器，不能自动解密或拦截游戏。每批序号持久化，重试相同序号不重复记录；默认每行立即上报。远端网关若另有登录要求，配置额外 headers。

NetAllay 集成源码及 YouyuanObserver 服务端插件入口位于 observer-bridge/netallay。协议核心5项本地回归通过；四个类已用真实 NetAllay0.1.1、Allay API0.28.0编译为Java21目标。入口读取 observer.properties 的 allowed_uuids（逗号分隔已认证UUID，默认空禁止接入），禁用时撤销监听；转发真实引擎健康变化、伤害、死亡、进退服。可取消事件明确标注阶段，不能把伤害请求视为最后扣血；攻击者无证据时返回未知。AllayReadOnlyProvider只声明在线玩家名单与指定玩家真实引擎血量，不把所有实体默认成20血。仍未在真实Allay服务器安装及运行，网易山头服不能凭OP安装Java插件；这不改变原服内核。

服务端安装后可主动转发引擎事件给已授权观测机器人，转发适配器不按机器人距离筛选。但必须由真实事件监听产生，并在服务端验证身份；跨距离/跨维度全局效果以服务端实测为准。没有匹配回复只能判断该桥未响应，不能判断全部 PyRpc 不可用。不要猜测/循环发送其他模块事件名。

## 0.10.26 多机器人联合观测

这些接口汇总当前宿主管理的已连接机器人实际收到的服务器解码包。没有收到的数据仍然未知。不会更改本机 entities 缓存，不自动创建机器人、不执行查询命令、不在收包线程进行网络访问，也不保证覆盖全世界。

| 调用 | 用法 |
|---|---|
| observerBridge.coverage(options) | options.server 默认当前服。返回观察账号、名字、位置、维度、会话、实际观测实体/已知血量数量和错误。账号断线明确显示。 |
| observerBridge.health(runtimeId, options) | runtimeId 必须十进制字符串；options 可指定 server、dimension。查询在线观察者最新真实血量，返回 current/max/previous/damage/stale/source 和 observer。身份或维度冲突不猜测，known=false。 |
| observerBridge.locator(target, options) | target 是玩家名或 UniqueID 字符串，读取真实 PlayerLocation 广播；没有 AddPlayer 也保留。返回 dimension_known=false、runtime_id=null，不把位置广播当血量或维度证据。隐藏类型不返回假坐标。 |
| observerBridge.worldEvents(options) | options 包含 server、dimension、cursors、limit；返回联合规范事件与每观察者游标。limit 默认256，1..4096，属于每次读取条数而非游戏命令/收包限速。 |
| observerBridge.localView(options) | 当前账号本地视图，用于聚合，不递归查询兄弟账号。mode=coverage/health/events；health 用 target；events 用 after、session、limit。 |

Lua 示例（目标 ID 使用真实抓包或实体事件取得的 ID，不能照抄示例数字）：
```lua
local coverage = observerBridge.coverage()
local result = observerBridge.health("真实RuntimeID", {dimension=0})
if result.known and not result.health.stale then
    print(result.health.current, result.health.max, result.observer.name)
end
local cursors = {}
-- 每次需要读取时调用；不要同步死循环或用1ms轮询HTTP。
local page = observerBridge.worldEvents({cursors=cursors,limit=256})
cursors = page.cursors
for _,event in ipairs(page.events) do
    print(event.type,event.observer.name,event.observation_id)
end
```
Python：`host.observerBridge.health(runtime_id, {"dimension":0})`；C++：`client.observerBridge.health(runtime_id, yy::json{{"dimension",0}})`；三者均通过相同宿主实现。MCP 使用游戏调用工具，method 填上述完整名称，args 为参数数组；开发文档 Resources 可读取此页。内置 AI 加载对应语言/观测技能后也能查看这些接口，不强制加载全部技能。

事件来源标为 server_to_bot_decoded；客户端输入桥仍为本地提交，不能把它当服务器已执行。多个观察者对同一事件的不同目击保留来源，不能直接把联合事件条数用作击杀计数。observation_id=账号/会话/序号，可去重同一观察记录；跨观察者业务去重需真实目标、时间和事件证据。下一页传返回的 cursors，重连 reset=true，历史丢失 lost>0；失败账号的旧游标保留，恢复后继续读取。complete 只代表本轮所有已启动账号查询无错误，并非全服覆盖完整。

当前控制器只联通自己的主账号、舰队与托管子账号；其他用户或其他机器要通过显式认证采集桥接入，不能自动跨用户读取。托管子进程仅查询自己；控制器在1秒内按需并发读取，错误显式返回。维度、服务器、连接会话不同不能混合实体；不存在的实体、已移除实体和断线缓存不充当有效观测。

修复了旧宿主把其他玩家 PlayerLocation 写入 bm.pos 的问题。现在仅相同 UniqueID 的坐标类型更新机器人自身位置，Hide 不将位置清零，其他玩家的广播进入独立 locator 缓存；MovePlayer 与 Respawn 同样校验身份。locator 缓存有可配置实体容量并在重连清空。该包没有维度字段，所以不能按维度查询 locator，也不能据它断言跨维度监听成立。

可直接用于：主插件显示远处观察机器人收到的生物血量、查看各观察账号覆盖的维度、从主账号统一读取远处健康/受伤/生命周期等已规范化事件。空挥、攻击归属、全服装备是否可读仍以服务器实际下发的包为准，接口没有创造隐藏数据。

## 0.10.27 真实远距离服务器模块战斗事件

来源是山头测试服实际接收的 PyRpc ModEventS2C，不是猜测包名，也没有强制请求服务器隐藏数据。本适配只解码经过记录验证的两个模块广播，operation_type=32346342；不能因为其他 ModEventS2C 常用9753608而忽略它。模块不存在或不广播时，接口不会伪造事件。尚未证明跨维度或任意服务器可用。

| 接口 | 参数与含义 |
|---|---|
| events.onEntityKill(handler) | 服务器 Minecraft/vipEventSystem/PlayerKillMob<UUID> 模块报告。event.attacker.unique_id、target.unique_id 都是精确十进制字符串；attacker_known 表示模块明确报告该攻击者。名字只关联实际在线 PlayerList。没有目标类型、RuntimeID、健康值就返回未知。不是 PvP onPlayerKill。 |
| events.onDamageDisplay(handler) | BREquipDamageEventServer/BREquipDamageEventServerSystem/GetBroadcastData。target.unique_id、damage_text、position、position_known。damage_text 如 '-33' 是显示文字，damage_known=false；不是真实HP扣血。无攻击者、维度字段，因此 attacker_known=false、dimension_known=false。 |
| events.moduleHistory(options) | options.session、after、limit；独立战斗记录，移动与乐器高频事件不会挤掉它。返回 session/cursor/latest_sequence/lost/reset/events/capacity。limit默认256、1..4096；容量使用diagnostics.configure的event_history。首次after=0；后续传返回cursor及session。重连清空并reset，覆盖记录会lost。 |

所有事件带 source=PyRpc、evidence=server_module_reported、module.namespace/system/event/operation_type、event_id、module_sequence、timestamp。不会据模块显示更新 entities.health，真实血量继续来自 UpdateAttributes。observerBridge.worldEvents 保留这个证据类型，另外标明 transport_evidence=server_to_bot_decoded。多观察者各自的 event_id 不等于跨账号击杀唯一号；业务跨观察者去重要按服务器/模块/攻击者/目标UniqueID处理。

Lua：
```lua
events.onEntityKill(function(e)
    local who = e.attacker.name or e.attacker.unique_id
    print(who .. " 击杀 " .. e.target.unique_id)
    -- 可在这里给明确的攻击者计分；不要拿目标缺失血量当满血。
end)
events.onDamageDisplay(function(e)
    print(e.target.unique_id, e.damage_text, e.position_known)
    -- 只用于模块报告的伤害显示；不要拿它做真实血量统计。
end)
local page = events.moduleHistory({after=0,limit=32})
-- 下一次按需读取：events.moduleHistory({session=page.session,after=page.cursor})
```
Python 通过 `host.events.onEntityKill("handler_name")` 注册宿主回调（见 Python SDK 的 handler 机制），C++ 通过 `client.events.onEntityKill("handler_name")`。三语言均可读取 moduleHistory；MCP 游戏调用 method='events.moduleHistory'、args=[options]，无需插件订阅上下文。内置AI可以按语言加载本地观测技能，看到完整示例与真实性边界。技能没有自动全部加载。

击杀去重：当前连接会话中以模块来源、攻击者UniqueID、目标UniqueID做有界缓存，容量沿用event_history；连接重建清空，缓存淘汰后旧报告重放可能再次出现，业务长期积分需自己的持久幂等键。伤害显示是连续更新，不用 requestID=1 去重；该编号可重复，不能当全局事件唯一号。

本次真人疣螈远处测试收到三条生物击杀报告及31条模块伤害显示。4753条乐器演奏事件保留在原抓包能力中，不自动变成战斗事件，也不能把它们叫作空挥。严格远距离阶段没有收到这个玩家的普通 Animate、MobEquipment、UpdateAttributes 或连续视角更新；不要把其他实体的包计到他身上。已有血量4/20后来过期仍明确stale，绝不充当远距离实时血量。

## 0.10.28 Lua坐标表修复

实际真人回调落盘确认了2次击杀和20条伤害显示，也发现显示position曾传成Lua字符串。现在position使用三语言都支持的数组绑定：Lua为table，可以直接读取e.position[1]、[2]、[3]；Python/C++/MCP仍为数值数组。position_known=false时不使用坐标。专门回归覆盖Lua数组类型与三个数值，不仅检查JSON形态。

真人最新证据保留原测试版本0.10.27和真实北京时间23:11:08～23:38:19，不把旧测试改写成0.10.28实测。该阶段11条击杀报告、56条伤害显示，最新击杀23:35:44.188，Lua插件真实回调2次击杀/20次显示并落盘；临时脚本和测试牛已清理。普通原生动作与全局实时血量仍未证明。

## 0.10.29 主动服务端转发与三语言接口

目标主要是普通网易租赁服，不依赖用户上传玩法组件。以下诊断接口在机器人本机运行；主动服务端模块为另一个可选方案，不能用山头服模块实测替代租赁服广播能力。

本地协议底层Dial使用readerLimits=false，已关闭单批包数量限制。监听器在本地业务筛选之前计数；captureStart({types={"*"},filter={every_ms=0}})可观测已下发解码包，显式指定LevelChunk等大包才能抓它们。登录握手私密数据不进入公开抓包。这里的缓存容量不是服务器广播权限；修改它不会制造未下发的数据。

新增共同API：

| API | 参数 | 实际作用 |
|---|---|---|
| diagnostics.receiveTransport | 无参数 | 当前协商协议ID/版本、注册包ID列表、解码前帧数/字节数/未知包/解码失败；不保存包内容和认证信息；不需要服务端组件 |
| events.onServerObservation | handler(event) | 主动模块所有事件；e.event_type、e.data、e.event_id、e.session、e.received_at、e.source、e.evidence |
| events.onGlobalSwing | handler(event) | 只接受有明确player_id的swing报告；不把伤害、命中攻击或任意Animate伪装为空挥 |
| events.onGlobalHealthChange | handler(event) | 有明确entity_id且实际current合法的health_changed；返回e.health.current/max/previous |
| observerBridge.serverHistory | {session?,after=0,limit=256} | 当前连接历史与cursor/reset/lost/capacity；不是网络查询，重连清空 |
| observerBridge.serverSnapshot | 精确entity_id字符串 | 独立实体快照缓存，known/current/max/previous、session、received_at/observed_at/stale/timestamp_valid；不受无关历史滚动影响 |

缓存使用可配置settings.actors与settings.stale_ms；模块最大血量未知为null，不默认20，不从damage_text反推出生命值。ID为字符串，禁止用浮点数传超长RuntimeID或UniqueID。server_engine_verified=false明确表示宿主收到模块报告，不能代替本轮引擎验收。原生entities.health(runtimeId)、player.equipment(name)、player.rotation(name)继续基于实际下发包，不覆盖或冒充为全服实时数据。

Lua常驻插件：
```lua
events.onGlobalHealthChange(function(e)
  local h = e.health
  print("模块血量 " .. e.entity_id .. ":" .. tostring(h.current))
end)
events.onServerObservation(function(e)
  if e.event_type == "player_death" then
    local d = e.data -- entity_id/attacker_id/attacker_known/cause
    -- 仅attacker_known为true时归属击杀，按event_id去重。
  end
end)
local history = observerBridge.serverHistory({after=0,limit=32})
local snapshot = observerBridge.serverSnapshot("精确实体ID")
```

Python常驻插件（使用注入的命名空间，不每次启动解释器）：
```python
def changed(event):
    health = event['health']
    print(event['entity_id'], health['current'], health['max'])
def on_init(host):
    events.onGlobalHealthChange(changed)
    events.onGlobalSwing(lambda event: print(event['player_id']))
def snapshot(entity_id):
    return observerBridge.serverSnapshot(entity_id)
plugin_api = {'snapshot': snapshot}
```

C++常驻插件（使用sdk/cpp/youyuan.hpp，注册回调名称）：
```cpp
class Observer : public yy::Plugin {
  yy::json last_;
public:
  Observer(const char* context, yy::HostAPI host): Plugin(context,host) {
    on("health", [this](const yy::json& args) {
      last_ = args.at(0); return nullptr;
    });
    client.events.onGlobalHealthChange("health");
    expose("last", [this](const yy::json&) { return last_; });
  }
};
YY_PLUGIN(Observer)
```

events.onGlobalSwing的global表示已安装的服务端模块不按观测者视距筛选，不表示任意第三方服务器都支持。NetAllay API0.29.0模块已编译，真实Allay启动未验收；网易ModSDK行为包具备血量/攻击/死亡，只读查询与引擎注入身份白名单，但没有经核实的网易服务端空挥接口，明确swing_supported=false。必须安装在游戏服务器引擎中；仅在工具箱装一个Lua/ZIP不会建立该能力。rpcProbe匹配Capabilities确认接入后再使用。

Linux C++实际已有Python3独立进程ELF运行器，修正了plugin.has/capabilities曾错误显示“不支持”的信息；Android仍在宿主进程内执行native，不能承诺相同崩溃隔离。SDK回调和跨模块通信保留，新增接口不改变命令Fast路径、整条链频率或加命令CPS限制。


## 0.10.34 接收诊断与 1.21.120 挥臂协议

diagnostics.receiveTransport() 的 received 每项新增：
- decoded_frames：已读取完当前定义字段的帧。
- decode_panics：字段读取中断；没有返回正常包。
- trailing_bytes_frames：读取已知字段后仍有扩展字节，不等于丢包。
- returned_packets：协议转换后返回上层的包数，不保证插件回调完成。
- decode_errors：保留旧异常合计；禁止直接当丢包数量。
这些都是当前连接累计值，应对两个时间点取差值；重连后重新计数。

当前网易协议860/游戏1.21.120的 Animate 读取 action_type、entity_runtime_id、固定float32 data，划船动作另外读取 boat_rowing_time。packet.on("animate", handler) 事件保留旧字段，增加 data/boat_rowing_time。Lua、Python、C++使用同一解码链路，不需要重新抓包猜字段。挥臂的action_type=1；data是协议动画参数，不能自行解释为伤害或生命值。UpdateAttributes 的 minecraft:health 才是实体血量来源。

接收统计没有保存原始认证、地址或载荷。队列/历史容量、解码兼容性与服务器广播范围必须分别判断：修改本地缓冲不能生成服务器未发送的包；OP也不能据此宣称全服原生挥臂/血量已实现。普通租赁服的全局原生动作与血量仍未通过真人跨距离/跨维度验收。


### 0.10.34 Lua事件类型与覆盖说明

Lua、Python与C++共用服务器观测数据。Lua转换现在保留float32/int32数字、可空数值指针、固定长度向量和带类型NBT，未知值为nil；不要把指针地址当最大血量。runtime_id是十进制字符串。真实接收、解码、回调和服务器广播范围分别验证。全局RPC伤害显示不是实体实时血量；一个机器人轮巡会有未覆盖时段，使用observerBridge.coverage/health/worldEvents显示实际账号、来源、会话、游标及过期状态。

## 0.10.35 统一观测事件、可选轮巡与账号令牌

观测事件由宿主解析真实服务器下发包，Lua、Python、C++不用先开启抓包。`events.catalog()`列出类型；`events.on(kind,handler)`按类型订阅；`events.onObservedAction(handler)`合并接收swing、hurt、equipment、rotation、entity_kill、damage_display，保留原type和source。伤害事件不等于空挥，未知攻击者保持未知。模块广播不一定存在于其他租赁服。事件缓存/历史、收包队列容量可以通过diagnostics配置，但扩大留存不能让服务器广播未发送的数据。

Lua（函数或函数名）：
```lua
events.on("swing", function(e)
  -- 真Animate挥臂；不证明命中了生物
  print(e.player, e.runtime_id, e.timestamp, e.source)
end)
events.onHealthChanged(function(e)
  print(e.runtime_id, e.previous, e.health, e.max_health)
end)
events.onObservedAction(function(e) print(e.type, e.source) end)
```
Python：`events.on("swing", handler)`，`events.onHealthChanged(handler)`；C++：`on("handler", callback)`注册函数后，`client.events.on("swing","handler")`。常驻运行器投递到插件自己的回调队列；不重跑源码。停止/重载插件退订。高频回调避免逐条写盘或同步HTTP。

### 插件私有自定义事件格式
`events.define(definition,handler)`：同一格式三语言可用，映射订阅属于此插件，停止后删除，不污染其他插件事件，不执行自定义代码。字段路径使用`packet.describe()`提供的真实snake_case字段。此版字段路径支持嵌套对象，不支持数组下标，血量请直接用宿主health事件。
```lua
events.define({name="arm_seen", packet="Animate",
  filter={fields={action_type=1}, every_ms=0},
  fields={runtime_id="entity_runtime_id", action="action_type"}},
  function(e) print(e.runtime_id, e.action) end)
```
输出含`source="custom_packet_mapping"`、`packet_type`、`timestamp`、`event_id`、`observed=true`、`inferred=false`、`server_verified=false`。最后一项表示自定义语义未由宿主认证：收到字段不能证明玩家命中、全局广播或服务器确认了操作。缺失映射字段为nil/null。限制私密握手、大区块、通配包映射以保护敏感凭证和内存；它不是额外游戏命令CPS限制。

### 单机器人顺序观察
```lua
local r = observerBridge.patrolStart({players={"玩家甲","玩家乙"},
  interval_seconds=0.001, single_follow_seconds=2,
  duration_seconds=60, restore_position=true})
-- 一次建立专用目标与分数甲=1、乙=2；逐个tp @s到分数1、2、1、2
local s = observerBridge.patrolStatus()
-- 在不需要时主动停止；插件卸载自动取消自己的轮巡
observerBridge.patrolStop()
```
必须当前机器人OP且目标真实在线。空players取当前在线列表并排除自己；默认未开启。固定路线只在开启时分配一次编号；新增/离开玩家后应停止并重开来刷新路线。编号只保存在本次专用计分板，不覆盖玩家其他积分。每次新建独立目标，停止时提交移除。循环TP使用Fast路径，不等每条回包。单玩家按single_follow_seconds执行，不在1ms重复堆发。间隔允许更细的正值，系统调度器和服务器Tick仍是实际限制；调度超时合并遗漏间隔，不无限追补积压。

`patrolStatus`区分submitted（本机写入）、rounds（路线轮数）、missed_intervals（宿主错过计划间隔）、position_updates（真实位置事件）、by_player（观测事件），最后位置附时间。位置包可合并/重复，不能把position_updates称为真实TP完成次数；`actual_tp_rate_known=false`。停止/断线取消发送，duration_seconds自动停止；restore_position默认请求恢复起点。清理是无等待提交，是否生效要再读位置/计分板结果。重复启动被拒绝直到之前清理结束。

单机器人轮巡是时分采样：并非连续同时覆盖所有玩家。更高Hz不一定更高事件命中率，过快TP可能还没同步周围实体便离开。只有实际近处收到的动作才计入观测；没有收到就不补造。约48格是当前测试假设，不是宿主硬编码限制或所有服务器固定上限。

### 多机器人协作
已有accountsPool.acquire/bind/start/invoke/release供插件管理自己的账号池；管理员all=true可按权限管理全部账号。每个机器人只能一个实际游戏会话，不能重复同一Token。给每个账号选择目标后，`accountsPool.invoke`调用该账号`/api/youyuan/game/call`，method为patrolStart，players只填对应玩家，single_follow_seconds按需配置。退出等待/再分配由业务插件决定；release支持idle_seconds默认60。`observerBridge.coverage/health/worldEvents`按需汇总各真实连接观测者，游标避免重复读取；global_coverage_guaranteed一直为false，缺乏观测的动作不伪造。

### 令牌查看复制
账号卡片“复制令牌”；编辑页“显示/隐藏Token”“复制Token”；云端游戏账号“查看/复制令牌”。增加游园独立验证服务选项，避免云端新账号默认地址选不上。接口POST `/api/youyuan/accounts` `{action:"token",id:"账号ID"}`：仅原有账号授权者控制范围，read授权拒绝、普通用户他人账号拒绝，响应no-store。常规GET列表继续不带Token。云端账号沿用本人token签发接口，发起需要平台会话；不读他人账号凭证。剪贴板被浏览器拒绝时提供可选中文本框手动复制。


## 0.10.37 玩家协议代理（独立实验组件）

可运行入口是`cmd/youyuan-player-gateway`，两侧真实网易认证和密钥绑定；玩家必须实际经过该UDP入口。中心可以留在另一维度；普通手机/启动器地址适配尚未完成，不能自动监听未接入玩家。

插件继续调用共同的`observerBridge.events(after, limit)`，读取`reply.events`与`reply.cursor`。Lua使用同名函数；Python使用`host.observerBridge.events(cursor,256)`；C++使用`client.invoke("observerBridge.events", yy::json::array({cursor,256}))`；MCP通过通用game.call调用同名方法。不覆盖原模块通信、原实体缓存或命令Fast路径。

外层event包含`kind/player_id/source/event_id/received_at/server_confirmed`；event.data是采集包封装，其中`packet_type/direction/observed_at/data`保留原始来源。Animate只表示挥臂；PlayerAuthInput的内层`missed_swing=true`表示明确空挥输入；服务器UpdateAttributes的`current/max`是本人收到的实际血量。客户端输入不能当作服务器命中结果。

实测（2026-10-11北京时间19:26:56–19:27:40）：中心dimension2、协议测试客户端dimension0；Animate10/10、MissedSwing5/5、本人血量18/20；21投递0丢弃，同机投递P95=11.881ms。ItemRegistry观测失败1次但原帧仍转发，治疗命令失败；真人手机入口、多玩家管理与24小时稳定性未验收。源码技能和文档已更新，线上0.10.36嵌入资料未在此独立发布中替换。

使用与完整限制：`observer-bridge/玩家协议代理使用与验收.md`。不要反复从cursor=0处理历史；不要把输入桥误写成`events.onGlobalSwing`服务端模块自动订阅。


## 0.10.38 动作API与MCP同步

# 动作 API、事件与真实覆盖范围

## 读取玩家代理动作（已实现的共同入口）

Lua、Python、C++和MCP读取同一个`observerBridge.events(after, limit)`，参数是上一页cursor和本页条数。返回`events/cursor/head_cursor/overwritten`。保存cursor并按event_id去重，不能每轮从0重新执行历史。历史是内存，宿主重启会清空；空列表不代表所有玩家没有动作。

| 动作/数据 | kind及来源 | 事件内字段 | 本轮验收 |
|---|---|---|---|
| 挥臂 | swing_input / Animate | runtime_id，missed_swing_known=false | 10发出10收到；仅挥臂，不能判断命中 |
| 明确空挥输入 | swing_input / PlayerAuthInput | missed_swing=true，tick | 5发出5收到；是客户端输入，非服务器命中结果 |
| 攻击输入 | attack_input / InventoryTransaction | target_runtime_id，position | 已实现解析，未真人验收；不是成功伤害 |
| 移动/转动视角输入 | position_input / PlayerAuthInput | position，pitch，yaw，tick | 收到5条位置输入；未真人验收纯转头 |
| 接入者本人血量 | health_observation / UpdateAttributes | current，max，known，runtime_id，source，tick | 服务器真实18/20；未实测血量变化 |

外层event含kind/player_id/source/source_id/event_id/received_at/server_confirmed。`event.data`是采集封装，包含packet_type/direction/observed_at/data；真正动作字段在`event.data.data`。RuntimeID和Tick是十进制字符串，避免64位精度丢失。服务器血量保留direction=server_to_player、source=server_packet；外部来源不自动成为中心服务器权威缓存。

MCP工具调用：使用通用`/api/youyuan/game/call`，POST参数如下。

```json
{"method":"observerBridge.events","args":[0,256]}
```

后续用返回的cursor代替0。资源为`youyuan://docs/player-actions`；也能通过技能读取API按需加载`player-actions/index.md`。这不是要求模型自己抓原始包猜字段。

Lua：
```lua
local reply = observerBridge.events(cursor or 0, 256)
for _, e in ipairs(reply.events) do
    local packet = e.data
    local d = packet.data
    if e.kind == "swing_input" and d.missed_swing == true then
        print(e.player_id, "空挥输入", d.tick)
    elseif e.kind == "health_observation" then
        print(e.player_id, d.current, d.max)
    end
end
cursor = reply.cursor
```

Python：
```python
reply = host.observerBridge.events(cursor, 256)
for e in reply["events"]:
    fields = e["data"]["data"]
    if e["kind"] == "swing_input" and fields.get("missed_swing") is True:
        print(e["player_id"], fields["tick"])
cursor = reply["cursor"]
```

C++：
```cpp
auto reply = client.invoke("observerBridge.events", yy::json::array({cursor,256}));
for (const auto& e : reply.at("events")) {
    const auto& fields = e.at("data").at("data");
    if (e.at("kind") == "swing_input" && fields.value("missed_swing", false)) {
        // e.at("player_id")及fields.at("tick")；按业务更新已有状态。
    }
}
cursor = reply.at("cursor").get<uint64_t>();
```

## 操纵机器人与观察他人是两类 API

`bot.swingAir()`让当前连接机器人排队提交一次MissedSwing；`bot.attack(runtime_id,slot)`提交近距攻击交易；`world.mineBlock`与`world.useBlock`分别是破坏/交互提交。提交成功不等于服务器执行成功，操作仍受服务器实际权限与同步状态约束。

原生监听`events.onSwing`、`events.onEntityHurt`、`events.onPlayerDeath`以及`player.rotation`、`player.equipment`继续读取服务器同步给机器人的资料，不能通过增加客户端解析字段使未下发的信息出现。`entities.health(runtimeId)`属于原生缓存；它不会自动查询或合并代理玩家的本人血量。

`events.onGlobalSwing`是已安装并确认能力的服务端模块报告通道，**不是**本代理事件的自动回调。插件读取代理动作使用上面的observerBridge.events。不要把代理输入、原生广播、服务端模块报告混为一个无条件全局接口。

## 覆盖、延迟和未完成项

独立代理0.10.37只有实际经过UDP入口、认证UUID/UID匹配的玩家才被覆盖。本轮中心在末地dimension2、协议客户端在主世界dimension0，说明已接入玩家向中心上报不依赖中心视距；未接入的其他玩家仍不能自动监听。普通网易手机启动器一键路由、多玩家生产入口和24小时稳定性未完成。

2026-10-11北京时间19:26:56–19:27:40：21投递0丢弃，P50=7.907ms、P95=11.881ms。这只是同机代理采集到中心入库，不是公网真人输入到插件显示。手机网络、轮询周期与显示调度会增加总延迟。ItemRegistry162观测解析失败1次，原帧仍转发；不能据此宣称装备全部可用。

使用、认证配置及失败记录见独立交付包中的《玩家协议代理使用与验收.md》。资料同步不自动将普通玩家接入代理，也不表示所有动作均已完成实测。

## 0.10.39 相机观察：真实结果与边界

这是网易山头服94F195C41B5上的实验结果。挥臂对照中只有观察机器人具有OP；发送挥臂的受控账号已通过自身UpdateAbilities确认command_permissions=0、player_permissions=1。远处玩家血量另用受控账号进行小伤害测试，不将该血量试验称为普通权限手机验收。主机器人身体固定，只对它自己的相机发指令，不修改其他玩家视角。相机不提供跨维度观察，也不等于所有区域同时订阅。

现有命令接口即可使用，不存在另一个已经全局监听的隐藏API：

```lua
-- x/y/z为合法、已选择的观察坐标。使用实际观察机器人游戏名。
local r = game.execute('camera "观察机器人游戏名" set minecraft:free pos 100 80 200',
                       {channel="ai", timeout_ms=6000})
-- r.success只确认命令执行；还应核对该视角实际收到实体包。
-- 结束时恢复当前机器人的正常视角：
game.execute('camera @s clear', {channel="ai", timeout_ms=6000})
```

Python使用`host.game.execute(command, options)`；Linux C++使用`client.invoke("game.execute", yy::json::array({command,options}))`。同名命令、权限和服务器结果完全一致，不因换语言扩大接收范围。循环调度需要自行停止并清理；Fast提交不能代替观察区域已下发的确认。

原生动作仍通过`events.onSwing`或`events.on("swing", handler)`订阅；真实掉血通过`events.onHealthChanged`、`events.onEntityHurt`。无需先开启抓包才能接收这些宿主事件。`entities.health(runtimeId)`读取现有缓存，检查known/max_known/stale；未知护甲、攻击者保持未知。

2026-10-11北京时间23:09：24格挥臂10/10；1500格无相机0/10；相机观察远处10/10；清除相机0/10。发送的是受控账号真实Animate，不证明未修改手机的空挥输入，也不能判断命中。

23:15–23:16远处牛血量：观察中10→9；离开该视角后一次真实伤害未收到属性变化，原缓存标记stale；恢复视角重新同步为8，下一次伤害8→7。初始最大血量为实际同步的10，不默认20。Lua回调亦收到真实变化；测得约0.03–1.57ms是宿主事件时间到回调入口，校准误差±0.65ms，不是玩家操作到HUD的端到端延迟。

23:41–23:42，20个相隔较远的地点，每组12秒：

| 相机提交频率 | CameraInstruction | NetworkChunkPublisherUpdate | 受控账号挥臂收到/发送 |
|---|---:|---:|---:|
| 5Hz | 60 | 59 | 3/60 |
| 20Hz | 240 | 238 | 0/60 |
| 40Hz | 480 | 239 | 0/60 |

固定动作周期可能与巡回周期错开，上表不能据此认定socket丢包。缓存覆盖0、收包队列末尾0、账号未掉线；40Hz虽收到480条相机指令，实际区域发布仅239次、11种位置（含过渡位置），不能把指令回显算成20区域完整覆盖。一次只看一处会漏掉其他处的瞬时动作，增加Hz无法承诺零漏检。错峰复核、账号可用数量及原始包另列验收报告。没有5～6个有效账号时不得用同账号重复登录假装并行验收。

2026-10-12北京时间00:07:40–00:08:18，独立玩家账号“超耐磨的星魂”位于约1500格外：相机观察中实际血量20→19，观察机器人收到ActorEvent和UpdateAttributes，缓存为current=19、max=20、previous=20、damage=1；清除相机后，同样真实伤害没有收到目标属性包，旧值标记stale；恢复视角后重新收到。测试结束已治疗并恢复模式、位置和维护配置，移除临时平台和加载区域。不是仅根据命令返回值猜测血量。

00:10:12–00:13:09，主观察机器人加5个独立子机器人，20个相隔较远的观察地点。5个发送账号全部实际撤销OP；逐账号固定观察正对照均为10发10收。并发动作使用90～310ms随机间隔，避免固定周期碰巧错开。每个速度运行12秒，并在每轮20次提交后移动轮巡起点，避免高频下总跳过同一批坐标。

| 请求切换频率 | 实际提交/相机回显 | 区域发布包 | 5账号挥臂收到/发出 | 宿主CPU累计秒 |
|---|---:|---:|---:|---:|
| 100Hz | 1200/1200 | 240 | 13/309（4.2%） | 2.53 |
| 400Hz | 4800/4800 | 230 | 20/300（6.7%） | 6.18 |
| 500Hz | 5976/5976 | 238 | 18/303（5.9%） | 7.35 |

500Hz下平均提交间隔约2.0ms，20地点的请求轮巡约40.2ms；但实际区域发布仍约19～20次/秒。命令成功与相机回显不表示服务器已为该地点同步实体。不能把“请求一圈小于50ms”当成“所有玩家每50ms都被有效观察”。上表是动作可见性缺失比例，不是socket丢包率；抓包缓存覆盖0，5个发送账号各阶段仍连接，非OP。视角变化输入与移动包条数也不一一对应，不能拿它作为挥臂无遗漏的证明。

账号恢复已修复：4个云端账号的会话刷新code10时，使用原保存SDK凭据重新认证，核对原游戏ID后更新会话；令牌保持不变。四账号均真实登录并参与上表测试；测试后保留账号池记录，停用临时运行实例，恢复原权限/模式/位置。

结论：该服相机确实能带来远处实体和玩家血量，以及普通权限账号挥臂，但单相机快速轮巡没有达到多人无遗漏目标。精确战斗显示应保持目标附近的观察节点，或分配多个独立节点；单相机只可作为有覆盖空档的采样方式。真人未修改手机、跨维度和24小时稳定性仍未完成，不能宣称全局无漏包。完整数据和原始包在0.10.39相机验收报告中。

## 0.10.40 多机器人错峰观察与交接

在「设置 → 实时感知与抓包 → 管理观测协作」操作。宿主提供可组合接口，不自动安装跟随插件，不强制启动观察。观察账号需要自己的有效登录、OP和同服连接；普通用户仅能操作自己的账号池，管理员沿用既有授权池。

### 常驻单目标观察

`observerBridge.watchStart({player, mode, interval_seconds, duration_seconds, restore_position})`。
`mode` 为 camera（相机）、follow（移动本机器人）、hybrid（两者），默认 camera；interval_seconds 默认2，可配置正有限值，不限制为20Hz。慢查询串行合并，missed_intervals记录未赶上的时段。没有逐事件扫描或包处理线程里的HTTP查询。玩家离线/查询失败/账号断线会停止；插件卸载自动取消自己的观察。

`watchStatus()` 返回 active/config/submitted/last_submitted_at/target_since/region_updated/region_timestamp/region_position/missed_intervals/error/cleanup。submitted只代表提交；region_updated要求本次目标对应的真实NetworkChunkPublisherUpdate，不能当成所有实体已同步或零丢包。重复提交同一位置不会重置已建立的区域证据。只有camera方式不能跨维度。

`watchStop()` 停止自己的观察，清除本次相机；follow/hybrid默认恢复已确认的原位置，restore_position=false可不恢复。清理结果分别标明submitted/confirmed，Fast清理不会冒充回包成功。与当前账号轮巡互斥，避免两个任务争抢位置。MCP手动控制具备该账号既有权限。

### 六账号规划：五个观察，一个补位

`accountsPool.observationPlan({accounts:[id1,...,id6],players:[],dwell_seconds:2,radius_blocks:48})`。
players省略时取真实在线列表。返回 assignments（account/players/phase_seconds/handoff_reserve/robot_label）、excluded_bots和targets。已知在线机器人默认排除；内部标签youyuan_observer用于规划，不偷偷给普通玩家写计分板。最后一个账号作为补位，其他账号分摊目标，轮换需调用者按路线驱动。规划本身不会登录、传送或启动任何插件。

48格是本服已有试验的参考值，不是协议承诺；广播还依赖区块、维度、服务端兴趣范围。六个机器人无法不间断覆盖20个相互分离的位置。错峰可减少覆盖空档，不能让服务器未下发的包出现。

`accountsPool.handoff(incoming_id,outgoing_id,{player,mode,interval_seconds,timeout_seconds:5,next_player?})`：接替者必须空闲，原观察者必须正在观察同一目标。两者真实连接且同服；先启用接替者，看到匹配的本次区域发布才停止原观察。超时取消接替、保留原观察；返回ready/old_released/incoming_status/old_status/error。next_player可让释放后的原观察者进入下一目标。真实覆盖仍需用worldEvents按观察账号/session/sequence去重与核对；不能把区域发布当作击杀或挥臂证据。

### 三种语言和MCP用同一套方法

Lua：
```lua
local status = observerBridge.watchStart({player="目标游戏名",mode="hybrid",interval_seconds=2})
local plan = accountsPool.observationPlan({accounts={"main","账号2","账号3","账号4","账号5","账号6"},dwell_seconds=2})
-- 插件调度策略自行选择；不需要在接包回调中反复查询。
```
Python常驻插件：
```python
status = observerBridge.watchStart({"player":"目标游戏名", "mode":"hybrid", "interval_seconds":2})
plan = accountsPool.observationPlan({"accounts":["main","账号2","账号3","账号4","账号5","账号6"], "dwell_seconds":2})
```
C++（youyuan.hpp）：
```cpp
auto status = client.observerBridge.watchStart(yy::json{{"player","目标游戏名"},{"mode","hybrid"},{"interval_seconds",2}});
auto plan = client.accountsPool.observationPlan(yy::json{{"accounts",{"main","账号2","账号3","账号4","账号5","账号6"}},{"dwell_seconds",2}});
```
MCP通用游戏调用工具（已有OAuth/控制授权）：
```json
{"method":"accountsPool.handoff","args":["账号2","main",{"player":"目标游戏名","mode":"hybrid","interval_seconds":2,"timeout_seconds":5}]}
```

### 皮肤和QQ群名单

独立认证原账号的皮肤组件与资源地址随登录传回。账号网页「选择皮肤组件」只选择账号已有的组件，不购买；提供商拒绝时显示真实错误，成功后重新连接游戏应用。默认读取原账号设置，不为缺失皮肤强制更改账号。玩家缓存接收PlayerList/PlayerSkin与单UUID明确归属的SyncSkin；空皮肤不会覆盖已有数据，归属不明时保留未知。

QQ群 `list [页码]` 或 `.玩家 [页码]` 返回12人一页的真实在线名单及收到的头像；`.查询 游戏ID`保留资料卡。缺失皮肤显示“尚未收到皮肤”，图片上传失败回退文字。统计为本设备实际观测，无坐标和私密凭据。此版本API实施和实景验收的范围见独立验收报告，不将文档示例视作测试成功。



## 0.10.43 生成文件交付

### `plugin.listOutputs`

relative_dir默认.；列出当前插件数据目录内生成的文件，返回path/name/bytes/modified_at/download_url，不返回文件内容。

详细参数与示例见 plugin-outputs.md。

### `plugin.exportFile`

relative_file；取得当前插件生成文件的下载信息，保持原格式/原字节，不是备份整个插件。文件须已写入plugin.dataDir。链接需登录当前账号，snapshot=false表示下载当前文件版本。

详细参数与示例见 plugin-outputs.md。



## 0.10.44 实时查询

`game.sceneStats()` 与 `game.getBlockTileAsync(x,y,z)` 为真实宿主接口；异步ticket用 `game.commandResult(ticket,0,true)` 读取。详见 realtime-queries.md；不要将完整 game.scene 快照放进高频周期。


## 0.10.45 观测与低延迟

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


## 插件直接控制宿主观察
以下为宿主API，不要求安装某个特定多功能插件。用户插件决定策略、目标和频率。

Lua：
```lua
observerBridge.watchStart({player="目标游戏名",mode="hybrid",interval_seconds=0.2,
 follow_radius_blocks=16,camera_height_blocks=4,camera_pitch_degrees=90,restore_position=true})
local state=observerBridge.watchStatus()
observerBridge.watchStop()
-- 自己的另一个机器人账号；实际配置中填写自己的池ID。
accountsPool.invoke("池账号ID","observerBridge.watchStart",{{player="目标游戏名",mode="follow",interval_seconds=2}})
local plan=accountsPool.observationPlan({accounts={"池账号ID"},players={"目标游戏名"},dwell_seconds=2})
```
Python：
```python
observerBridge.watchStart({"player":"目标游戏名","mode":"hybrid","interval_seconds":0.2,
 "camera_height_blocks":4,"camera_pitch_degrees":90,"follow_radius_blocks":16})
observerBridge.watchStop()
accountsPool.invoke("池账号ID", "observerBridge.watchStatus", [])
```
C++通过现有client.observerBridge.watchStart(yy::json配置对象)（或client.invoke("observerBridge.watchStart", yy::json::array({配置对象}))）同名调用；实际SDK类型按cpp-runtime章节。MCP用/api/youyuan/game/call {method:"observerBridge.watchStart",args:[配置]}；跨账号用accountsPool.invoke。UI复制插件调用代码按钮以当前参数生成Lua代码。

watch/plan/handoff接口有明确差别：watch真实提交镜头/身体跟随，plan只规划，不启动账号；handoff先收到接替区域更新才释放旧观察。必须记录OP、维度、身体/镜头是否移动。扩大覆盖不等于全服广播，不保证不丢包。停止插件自动取消其拥有的观察任务，保留模块通信和原来的低级接口。



# 0.10.48 全局抓包、作业暂停与旧注册兼容

## 设置位置和多选
设置→服务器连接→应用启动时自动连接下方→全局抓包。独立开关，配置按钮打开自己的账号与玩家多选卡片；保存全部配置后生效。技能加载库也使用明显的复选框、选中边框和计数，支持全选/清空/统一保存。

默认全局抓包关闭、camera模式，只移动本机器人原生摄像机，身体不跟随。可选择多个玩家，或动态观察在线玩家。手动勾选玩家会自动关闭动态全选，避免选中名单被自动模式忽略。开启跟随/混合模式可能传送机器人身体，会改变区块加载、触发服务端限制、抢占建筑工作位置；不会故意传送玩家。默认排除已连接所选观察账号的真实机器人名字。

每次镜头切换间隔可配置毫秒，不等于命令链节点频率。完整轮巡约为目标数量×切换间隔。高频负责提交命令，不能保证服务器同样频率完成、零漏包或全服血量同步；原生摄像机的维度与广播限制仍存在。

## 原生选择器镜头
observerBridge.watchStart支持原player字符串（兼容旧写法）以及players数组、auto_players、camera_selector、exclude_players、pause_during_jobs。多目标或camera_selector=true的纯相机模式执行：
execute at "目标玩家" run camera "观察机器人" set minecraft:free pos ~ ~4 ~ rot 90 0
目标坐标由游戏执行器原生解析，轮巡不在每个切换前等待querytarget或命令回包；只记录submitted。普通单目标旧watchStart保留原来区域确认路径，避免破坏已有handoff逻辑。

选择器路径未核验位置时target_position_known=false、target_dimension_known=false、region_updated=false，不能把零坐标或提交次数作为全局覆盖成功。需要交接区域确认时继续用旧单目标watch/handoff。跟随/混合多目标路径依然存在必要位置查询，不能把纯镜头Fast路径速度直接当身体跨维度跟随速度。

## 原生配置API
GET /api/youyuan/global-capture：config、states和global_coverage_guaranteed=false。
POST支持部分字段更新；例如{enabled:false}保留其他配置。配置字段：enabled、accounts数组、players数组、auto_players、mode(camera/hybrid/follow)、interval_seconds、camera_height_blocks、camera_pitch_degrees、camera_yaw_degrees、pause_during_jobs。只控制自己绑定设备/账号池，不能选别人的账号；控制权限与只读权限分开。
程序保存到设备global-capture.json，重启后恢复开关和参数。只管理自己的已连接账号，失效/未连接显示错误，不伪造已连接；不覆盖其他插件已运行的观察任务。需先在账号管理连接要用的机器人。关闭后释放本模块自己启动的任务。

## 导入导出作业暂停
宿主按机器人实例维护空间作业租约，覆盖主程序导入/导出/修补/地图画/皮肤建筑、多机器人导入及game.importBlocks插件导入。区域导出在每个实际参与机器人上增加租约，building.exportRegion与placeContainer的NBT异步作业也会暂停本账号观察。作业开始暂停原生跟随/TP轮巡；作业结束才恢复。多个作业引用计数，结束一个不能误恢复仍被另一作业占用的机器人。
默认相机也暂停，避免抢区块视距；配置pause_during_jobs=false只允许纯相机继续，身体跟随仍暂停。停止观察期间不会把机器人恢复传送到旧起点去打断导入导出。
game.teleport/game.move及常见自己TP的插件命令在作业期间明确返回暂停错误；普通聊天、查询和非移动命令不受影响。不关闭/重载插件，不重置Lua/Python状态。任意插件自行构造原始移动包或绕过标准API不在这种合作式暂停保证内，不能宣称隔离全部自定义行为。

Lua：
```lua
local job=observerBridge.jobState()
if not job.paused then
  observerBridge.watchStart({players={"甲","乙"},mode="camera",camera_selector=true,
   interval_seconds=0.025,camera_height_blocks=4,camera_pitch_degrees=90,pause_during_jobs=true})
end
```
Python同名接口：observerBridge.jobState()，observerBridge.watchStart({"players":["甲","乙"],"mode":"camera","camera_selector":True,"interval_seconds":0.025})。
C++：client.observerBridge.jobState()、client.observerBridge.watchStart(yy::json对象)，ABI保持不变。跨账号保留accountsPool.invoke；配置由插件自行决定，程序页面不要求依赖特定多功能插件。
高频传送插件应先检查jobState().paused并跳过当前轮，避免反复触发暂停错误。直接错误不是“已成功传送”，不得忽略错误后编造结果。

## 旧Lua注册接口兼容
注册参数保持兼容：game.registerTrigger(words,handler)、events.onChat(handler)、events.onJoin(handler)、events.onLeave(handler)、events.onBotReady(handler)、events.onPacket(handler)、events.subscribe(packetType,handler)、util.schedule(seconds,handler)、util.setTimeout(seconds,handler)、web.action(page,action,handler)。handler可为原全局函数名或函数对象。回调参数分别是onChat的(name,message)、onJoin/onLeave的(name)、onBotReady的无参数、onPacket的(packetType,data)、subscribe的(data)，不要把回调参数当注册参数。
上述注册现在也接受Lua函数对象（包括模块返回的函数与闭包）。加载一次、常驻回调，不重跑main.lua。事件、包订阅和停止清理按机器人账号隔离；停止一个账号的同名插件不应移除另一个账号的事件/定时器。
旧ZIP导入仍保留manifest、main.lua、modules、command_txt等资源目录。新的测试实际导入旧ZIP，读取模块，再触发聊天回调；不是只检查语法。没有提供的旧宿主私有注册接口不会伪造一个空函数来骗过导入；遇到具体缺失函数需按原定义补接并验收。

## 本轮证据与限度
原生选择器相机命令在Linux游戏服对本机器人测试，服务器confirmed=true、success_count=1，随后camera @s clear同样确认成功；没有身体TP。这只证明该指令路径可执行，不证明远距离真人动作、多人覆盖率或跨维度血量。
新测试覆盖：旧字符串/函数回调不重置与跨账号隔离、旧ZIP模块和TXT导入、空间租约嵌套与恢复、原生相机不拼querytarget/TP、账号池越权拒绝、多选账号/玩家/技能一次保存。界面行为用模拟DOM测试；实景视觉验收单独记录。


## 0.10.49 多选边界修复与实测
全选列表包含机器人自己时，多目标轮巡自动跳过自身，继续观察其他目标；原单目标只选自己仍明确报错。账号排除名单继续生效。旧抓包页移除重复配置卡片，入口统一在设置→自动连接下方。
0.10.48真实Linux测试：观察非OP子账号的原生镜头40次/秒提交；单方块导出生成272字节mcstructure，导出期间提交计数固定2867，结束后增长至2873，作业状态恢复。测试后关闭全局抓包、停止任务并确认清除镜头。仅为配置/暂停恢复实测，不是远距离多人收包率证明。


# 0.10.50 收包与常驻运行性能

优化保持原始协议字段与旧插件调用形式。packetToMap缓存固定的字段索引与snake_case名，仍生成独立结果，完整转换二进制hex/数组/NBT；不会截短订阅包。订阅列表仅在增删时发布不可变快照，分发不再每包复制列表或争抢注册锁；同次分发的回调顺序及退订语义保持。协议目录所有构造包字段对照旧转换器，带数据的移动/血量及大ID对照通过。

同一台Linux、同一组fixture、单核、各3次：0.10.49移动包转换中位4846ns，当前1681ns（2.88倍）；血量包8082ns到3919ns（2.06倍）；订阅分发5173ns到1934ns（2.67倍）。这是解码后的宿主转换/分发微基准，不是全游戏或网络RTT倍率。移动车体状态缓存约292ns保持；不会把未变化指标包装成加速。

保留game.getScore、getPlayerPosXYZ、getPlayerItem、getBlockTile、getBlockTileRaw、getTickingAreaList、getBlockData/getBlockNBT/getBlockState/getRegionBlockData及placeNBTBlock/setBlockState的同步返回/错误；等待Go网络/文件/工作台时释放常驻Lua锁，其他回调和定时器可继续运行。Lua输入先复制为Go值，返回后再拿锁写Lua；不在两个Go线程同时执行同一份Lua指令，不重跑源码或丢弃模块状态。旧查询实际耗时仍可能包含网络等待。

区域读取、NBT放置与状态修改也使用按账号空间作业租约，避免观察跟随抢占操作位置；参与作业的内部传送保留，不拦成空功能。初始化阶段保留原初始化顺序，不能将回调期协作等待承诺成初始化所有网络I/O都不等待。

plugin.stats()/MCP /api/plugin/profiler增加io_waiting、io_calls、io_total_wait_ms、io_max_wait_ms，区分网络等待与执行墙钟。total/max为本实例生命周期统计，io_total可能包含多个并行等待，不是CPU时间；cpu_ms等未测字段继续null。Python/C++通过同一宿主接收转换/订阅路径获益；本轮Lua旧同步调用锁释放修复不表示Python/C++单语言性能达到了同样倍率。

性能验收：原0.10.49真实game.getRegionBlockData一次1130.55ms，10ms心跳在这段时间运行0次；新版本需要以同样接口真实复测并保存证据。另做连接/确认RTT/队列丢弃样本，不能只用本地空计数循环判断真实服务端完成。

没有改变握手协议ID、加密方式、服务端权限和条件链结果依赖。客户端优化不能使服务端没有广播的包自行出现；当前成果不宣称整体200倍或2000倍，也不通过删字段、停功能或降低事件覆盖率换数据。

方法依据：Go官方性能诊断与CPU profile https://go.dev/doc/diagnostics；fixture CPU样本显示旧snakeCase累计约27.65%，因此优先消除反复转换和分配，而非猜测网络协议。
