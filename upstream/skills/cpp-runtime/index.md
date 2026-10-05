# C++ 常驻插件开发手册

游园 0.10.8 / SDK 1.12。当前原生动态库运行器支持Android arm64-v8a与x86_64；Linux/Windows尚无此适配器。C++插件逻辑是编译机器码，游戏接口通过当前账号宿主执行；不等于BDS服务端插件。

## 文件与ABI

源码#include "youyuan.hpp"，自带third_party/json.hpp（nlohmann::json）。使用C++17、Android NDK对应目标编译.so；桌面.so不能直接放到Android。ZIP内manifest.json加libplugin.so：

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

宏导出yy_create_v2/yy_event/yy_destroy，构造参数context_json含account/plugin_id，不是旧账号字符串。on注册宿主回调处理器；exports通过expose注册跨插件函数。参数/返回值是yy::json：对象用{{"key",value}}，数组用yy::json::array({...})，读取用.at()/get<T>()，http.delete对应delete_。错误为std::runtime_error；在回调边界捕获，不能把原生崩溃隔离误说成已实现。正常事件与定时调用复用同一个对象，stop销毁。不要传C++指针或文件句柄到JSON桥。

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

本手册和每命名空间章节为离线Skill；AI按当前语言与需要的包/函数读取小节，不加载全体包正文。工具是否允许调用由工作区配置决定；知道函数不代表自动获准执行。原始协议目录单独加载packet-reference，包含当前编译内核所有真实已注册包和字段；每包发送能否成功仍由服务器判断。当前血量/死亡及部分物理动作尚未完成所有真人场景验收；文档不将未测能力标成实测。

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
- [web · 插件控制页面](web.md)：声明式页面、HTML资源、动作回调与事件推送，重载清理旧注册。

语法块是参数顺序参考；下方游戏用例为完整工作流。章节包含当前真实注册方法，不用内部Go函数名冒充插件API。
