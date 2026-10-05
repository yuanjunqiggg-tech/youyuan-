
## 0.10.7 连续发送与整链频率
默认 rate_cps=0 表示不加人为 CPS 等待。game.sendCommand 不再逐条睡眠20ms；pipeline、submit、burst不再默认限制200条/秒。只有主动配置正数 rate_cps 时才启用发送上限。Lua、Python、C++共用这条分发路径。
发送缓冲按256包或256KiB刷新，部分缓冲由原有传输定时器刷新；不是每256条停一个周期。sendCommand返回submitted仅表示提交，最终效果用回包或计分板核对。无限循环仍应让出定时调度，不能把网络提交率当服务器执行率。
stream中正常回包success=false（如空选择器）不会中断后续无条件命令；有条件节点仍根据上一条真实结果跳过。断线、超时与未知结果不自动重发。32条以内且可装入用户窗口的短链一次提交；更长的连续段按4096条和512KiB拆段，整份TXT可超过这些分段数量。
整链Hz=1000/interval_ms。每轮包含整个文件，运行耗时算在周期内；完成后只等待本周期剩余时间，超过周期则不增加等待，不积压新轮次。每条的delay_ms独立且默认0。状态actual_hz显示真实完成轮/秒，last_round_ms是上一整轮耗时。
Lua批量结果results是可ipairs遍历的table，output是字符串table，success_count是数字。事件回调复用常驻Lua状态。
```lua
-- 同样的循环不需要改为特殊批量API，也不默认逐条限速。
for i=1,10000 do game.sendCommand("scoreboard players add @s yybench 1") end
-- 需要逐项核对结果，或按真实结果进行分支时再用批量/确认API。
local r=game.executeBatch(commands,{mode="stream",window=256,rate_cps=0})
for _,row in ipairs(r.results) do print(row.confirmed,row.success) end
```

## 0.10.3 跨链与有界调度
支持 @call（目标源文件行号/次数/时限/持续）、@wait（默认失败后再成功的边沿）、@pause及chains.trigger。全部机器人内执行，无递归任务，无周期积压。完整本地开发资料：load_skill("command-chain",section="control-flow.md")。单次burst<=1024，默认无固定CPS，不能声称上亿/秒或每轮同Tick。

## 最新：机器人整批协议执行
mode=burst 每次最多1024条/512KiB文本，逐条真实确认，保留账号冷却。1000条实际测试出现同Tick，也出现跨2Tick；不保证每轮同Tick。不放置命令方块，条件/延迟形成等待屏障。完整参数见下文“整批协议发送”。

# 插件调用命令链

## 0.10.2 机器人内常驻命令链

所有TXT在机器人内预解析和运行，不部署命令方块，不改世界方块，不切创造。严格顺序 mode="remote" 每条等待真实结果；有界流水线 mode="pipeline"、window=1..16（默认8；window是每批最多在途命令的条数，不是毫秒或时间窗口）、delay_ms=0：连续无条件、同通道、无延迟命令成批提交，之后按顺序收集回包。遇到有条件命令、通道切换、指定延迟形成等待屏障。条件只看上一条真实成功；. 新链不继承结果；! 周期结束。每文件独立启停，run_count=0持续，正数为整轮次数。

流水线保持提交顺序，不保证服务器完成顺序，也不保证1000条同Tick。有先后依赖的无条件命令必须用严格顺序。插件/AI/命令链批量调用按账号串行调度；stream预算0时使用有界回包背压，其余方式共享发送预算，流水线最多16条在途，批量默认无固定CPS上限，单条默认无额外等待；实际速率看服务器回包，不能把提交数当执行成功数。每条额外延迟可为0，不再重复强制增加20ms。

chains.save("工作.txt", text, {"mode":"pipeline","window":8,"delay_ms":0,"interval_ms":1000,"run_count":2,"enabled":False})；chains.start/stop/list/get/configure/menu/handleChat 共用UI/API/MCP同一运行器，账号隔离不变。Lua用table，C++用client.chains或通用call，Python用chains。runtime.executed记录尝试次数；submitted记录流水线已提交数；cycles仅完整轮完成后增加；超时结果未知，不自动重发危险指令。

插件 game.executeBatch 增加 transport="command_request"（默认，支持ordered/pipeline/submit）、"settings"、"pyrpc"。后两者仅支持submit，没有逐条成功确认，禁止当条件依据。settings为SettingsCommand，pyrpc为网易ExecuteCommandEvent，不伪装成有回包的调用。玩家channel使用玩家命令来源；ai与console在有回包接口中都使用自动化来源，换名字本身不提升速度。




这是 APK 内置接口。Lua、Python、C++ 共用文件管理、预解析和运行器，不需要另建定时器来逐行读取 TXT。默认不安装本页示例，不自动执行任何命令链。

## 方法

| 方法 | 参数 | 返回/行为 |
|---|---|---|
| chains.list | 无 | files：每项 config、state、cycles、executed、skipped、error、commands |
| chains.get / export | name | 完整 content、config、status；export 返回数据，由插件选择写入/打包 |
| chains.validate | content | 预解析 plan，含每条命令的原行号、类型、条件、通道；语法错误直接报错 |
| chains.save | name,content,config | 创建或完整更新；运行中会先停止，enabled=true 时重新启动 |
| chains.configure | name,patch | 保留源码，合并配置，再保存/重启 |
| chains.start / stop | name | 启停；启动已启用的链不重复建立实例；停止等待当前实例退出 |
| chains.delete | name | 停止并删除 TXT 和配置 |
| chains.menu | player,page=1 | 每页8条，只发送给该玩家的聊天菜单，不阻塞等待回复 |
| chains.handleChat | player,text | 解析下述菜单操作，返回 handled；配合聊天监听 |

config/patch 字段：interval_ms（周期，至少25ms）、delay_ms（每条额外延迟，可为0）、run_count（0持续，1..1000000完整轮数）、channel（ai/player/console/default）、mode（remote/pipeline）、window（1..16）、enabled（布尔）。配置固定为本插件所属账号，不能通过 account 字段切换他人账号。保存覆盖文件前先读取；configure 保留未修改字段。返回错误不是成功。超时不要自动重复发送破坏性操作。

文件目录：插件数据文件/ds_ai_agent/command_txt/；文件名必须为直接文件名，以 .txt/.mcfunction 结尾，不能带目录。UI、插件、MCP修改的是同一份文件。各账号独立内核拥有独立目录；同一内核的多机器人按链 account 过滤，不能控制其他机器人的链。

## 游戏聊天菜单

`.命令选择` 或 `.链 列表 2` 显示分页；`.链 启动 1`、`.链 停止 1`、`.链 次数 1 3`、`.链 频率 1 1000`、`.链 通道 1 player`、`.链 源码 1`、`.链 删除 1`。序号为当前账号按文件名排序的全局序号，也可输入完整文件名。修改、删除和源码查看要求发言玩家 OP；无法确认权限时拒绝。此限制属于内置聊天控制器，插件自身直接调用仍拥有插件账号的接口能力。

Lua：
```lua
function chain_chat(player, text)
  if text == ".命令选择" or string.sub(text,1,4) == ".链" then
    chains.handleChat(player, text)
  end
end
events.onChat("chain_chat")
-- 按用户操作调用，不在 onChat 内循环：
-- chains.save("欢迎.txt", "1,0,0\nsay 欢迎\n!", {run_count=1, interval_ms=1000})
-- chains.start("欢迎.txt")
```

Python（聊天回调接收事件对象）：
```python
def on_chat(event):
    text = event.get("message", "")
    if text == ".命令选择" or text.startswith(".链 "):
        return chains.handleChat(event["player"], text)
events.onChat(on_chat)
# chains.configure("欢迎.txt", {"run_count": 3, "enabled": False})
```

C++（由 Client 经宿主调用）：
```cpp
on("chain_chat", [this](const yy::json& args) {
  const auto& event = args.at(0);
  return client.chains.handleChat(event.at("player"),event.at("message"));
});
client.events.onChat("chain_chat");
// auto files=client.chains.list();
// client.chains.configure("欢迎.txt", yy::json{{"run_count",3}});
```

## AI 与 MCP

AI 加载本技能后使用项目文件工具写上述代码，或者 `game_call`，function="chains.list" / "chains.configure"，args 按表顺序给出。同样可以读取终端和语法报错。不要把完整手册作为系统身份提示词。

外部 MCP 先发现工具；`api_youyuan_game_call` 或 `api_call` POST `/api/youyuan/game/call`，body=`{"method":"chains.list","args":[]}`。可选账号通过 MCP 工具 account 路由至该用户账号。文档资源 `youyuan://docs/command-chain-api` 按需读取，`/api/youyuan/docs?language=python&method=chains.start` 可以只读单个接口。

源码语法另读 command-chain；执行通道另读 command-execution。三位状态、条件继承、独立链、运行次数等不会因为从插件调用而改变。

C++ 中 export 是关键字，使用 client.chains.export_(name)；delete 使用 delete_(name)。完整方法名仍为 chains.export / chains.delete。


## 整批协议发送（burst，实验性）
所有执行仍在机器人内，不放置命令方块。`game.executeBatch(commands, {mode:"burst",channel:"ai",transport:"command_request",window:1024,timeout_ms:8000})` 一次最多1024条、512KiB命令文本，将这段封装成一个网络批次，并逐条收集真实确认。整批默认 rate_cps=0（无固定CPS），0.10.4 可设置 rate_cps=20..20000，避免循环无限洪泛。网络批次相同不保证服务器同Tick执行，应依据实测首尾 `time query gametime` 和真实结果判断。
命令链 config.mode=burst、window=1..1024。无条件、相同通道、没有逐条延迟的连续段整批发送；遇到有条件命令、通道变化或延迟会等待结果，保持条件链语义。这些等待需要网络往返，不能承诺含条件的一千条都同Tick。remote严格逐条确认；pipeline窗口1..16；burst整批顺序提交，服务器实际顺序须验收。


## 0.10.4 完整附件与可调发送预算
AI工作区附件入口在对话输入区，ZIP保留目录结构，长粘贴保存文件；按需加载 toolbox-live-development 的 workspace-files.md（命令链也可从 command-chain 技能加载该章节）。用 project_read(offset,max_chars) 读取长行尾部，project_list(prefix,offset,limit) 分页列文件。
整批 game.executeBatch 的 options 和 chains.save/configure 的 config 都支持 rate_cps：0不加固定CPS等待，20..20000是配置的发送预算，非实际保证吞吐。UI可直接修改，条件等待仍是屏障，轮次不会重叠，回执超时不自动重发。


### 动态滑动窗口（0.10.4）

`game.executeBatch(commands, {mode="stream", channel="ai", window=512, rate_cps=0, timeout_ms=8000})`。Python/C++使用相同SDK options对象。stream单段1..4096条、文本512KiB，最大在途window=1..1024（默认128）。从16条起，成功回包累计达到当前窗口后倍增到上限；任何回包都释放槽位，不等待最早请求，也不等整窗全部回复。新补位命令立即按小批发送，不等待底层50ms刷新。`rate_cps=0`仅由在途窗口与回包背压约束；正数20..20000启用自定义共享账号发送预算。网络错误、超时、断开停止新发送，已发送回包仍收集，未知不自动重发。

命令链config同样支持`mode="stream",window=512,rate_cps=0`。有条件节点、通道变化、延迟、@call/@wait/@pause形成回包屏障。保持提交顺序；不保证独立命令的服务端完成顺序或同一游戏Tick。窗口仅服务于无真实依赖片段。整轮执行耗时算在周期内，仅等待剩余时间，不会堆积40Hz的未完成轮次。输出results逐条submitted/confirmed/success，peak_inflight是实测最大在途，network_batches是实际发送批数。
