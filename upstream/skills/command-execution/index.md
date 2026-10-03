## 0.10.7 框架执行路径

`game.sendCommand(command)` 保持旧无等待调用方式；新增 `game.sendCommandFast(command, options?)` 与 `game.sendCommandsFast(commands, options?)`。后者一次宿主调用提交完整数组，没有整个数组的行数限制。魔法/ai 的无等待路径使用 SettingsCommand、保留机器人 `@s` 和相对坐标；提交按输入顺序，网络层按原定时周期合并，不逐条等回包、不默认CPS等待。`submitted` 只表示提交，`confirmed=false` 不代表游戏成功。用户主动给正数 `rate_cps` 才限速。

`game.sendCommandConfirmed` 是 `game.execute` 的别名，等待 CommandOutput；超时是未知，不自动重发。`game.executeBatch` 的 ordered/pipeline/burst/stream 保留可确认路径；其中 window 是在途请求数量，不是整个文件长度。无条件链可选 `mode="fast"`；有条件节点及其直接前一条自动切到确认路径，`@wait/@call/@pause` 控制语义保留。Hz 是完整链的目标轮次/秒，超期不积压轮次。

Lua: `game.sendCommandsFast({"scoreboard players add counter test 1", "scoreboard players add counter test 1"})`。Python: `host.game.sendCommandsFast(commands)`；C++: `client.call("game.sendCommandsFast", nlohmann::json::array({commands}))`。先校验整个数组；网络失败可能已有部分提交，不应盲目重试。Fast不保证原子或同一服务端Tick，也不能自动判断每条成功。显式 PyRPC 在当前测试服100条无执行，不能将旧源码的通道推断为当前所有服务器可用。


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

# 插件命令执行 · 所有语言共用

## 0.10.2 机器人内常驻命令链

所有TXT在机器人内预解析和运行，不部署命令方块，不改世界方块，不切创造。严格顺序 mode="remote" 每条等待真实结果；有界流水线 mode="pipeline"、window=1..16（默认8；window是每批最多在途命令的条数，不是毫秒或时间窗口）、delay_ms=0：连续无条件、同通道、无延迟命令成批提交，之后按顺序收集回包。遇到有条件命令、通道切换、指定延迟形成等待屏障。条件只看上一条真实成功；. 新链不继承结果；! 周期结束。每文件独立启停，run_count=0持续，正数为整轮次数。

流水线保持提交顺序，不保证服务器完成顺序，也不保证1000条同Tick。有先后依赖的无条件命令必须用严格顺序。插件/AI/命令链批量调用按账号串行调度；stream预算0时使用有界回包背压，其余方式共享发送预算，流水线最多16条在途，批量默认无固定CPS上限，单条默认无额外等待；实际速率看服务器回包，不能把提交数当执行成功数。每条额外延迟可为0，不再重复强制增加20ms。

chains.save("工作.txt", text, {"mode":"pipeline","window":8,"delay_ms":0,"interval_ms":1000,"run_count":2,"enabled":False})；chains.start/stop/list/get/configure/menu/handleChat 共用UI/API/MCP同一运行器，账号隔离不变。Lua用table，C++用client.chains或通用call，Python用chains。runtime.executed记录尝试次数；submitted记录流水线已提交数；cycles仅完整轮完成后增加；超时结果未知，不自动重发危险指令。

插件 game.executeBatch 增加 transport="command_request"（默认，支持ordered/pipeline/submit）、"settings"、"pyrpc"。后两者仅支持submit，没有逐条成功确认，禁止当条件依据。settings为SettingsCommand，pyrpc为网易ExecuteCommandEvent，不伪装成有回包的调用。玩家channel使用玩家命令来源；ai与console在有回包接口中都使用自动化来源，换名字本身不提升速度。



Lua、Python、C++ 的 `game.sendCommand(command, options)`、`game.execute(command, options)`、`game.executeBatch(commands, options)` 使用同一通道分发。`options` 可省略，传一个通道字符串，或传对象。通道：`ai` / `魔法指令` / `0`、`player` / `玩家` / `1`、`console` / `控制台` / `2`；`default` 使用本插件的默认通道。玩家来源受玩家权限约束；魔法指令和控制台目前都使用自动化来源，不等于服务器物理控制台，不提升账号已有权限。

`game.getCommandOptions()` 返回本实例默认值；`game.setCommandOptions(options)` 修改本实例内存配置，不影响其他插件/账号。默认 `{channel:"ai",mode:"ordered",window:8,timeout_ms:3000}`。每次调用的 options 仅覆盖该次调用，不修改默认值。窗口范围 1..16，超时范围 100..10000 毫秒。错误参数报错，不默默切换通道。

用户在工作区“命令执行参数”修改的值保存到项目 `command-options.json`，可被 AI 读取、搜索、修改并保留 Diff。发布并重新加载后应用：Python/C++ 宿主从插件根目录加载，Lua 发布到插件配置的 `command_options`。源码中的 setCommandOptions 可覆盖这些默认值；给用户保留界面配置时不要在源码里重复硬编码覆盖。

## 执行与成功确认

- sendCommand：只发送，返回 submitted/confirmed=false，不能作为成功条件。
- execute：发送一条指令，等待游戏回包，返回 confirmed/success/success_count/output/channel。超时返回 unknown=true；不自动重发，避免重复副作用。
- executeBatch ordered：逐条等待结果，适用于有顺序依赖的命令；各命令失败后仍会继续，若需要“上一条成功才继续”，在源码判断 execute.success 或使用 TXT 条件链。
- executeBatch pipeline：每窗口按输入顺序发送再汇总回包，仅用于互不依赖的命令，不保证服务端完成顺序。返回每项的真实 submitted/confirmed/success/error。
- executeBatch submit：批量只发送，无成功确认，不能用于依赖判断。

批量每次 1..1024 条。输入命令不用 /，带 / 也接受。网络等待、服务器 Tick、账号权限会限制速度，不承诺固定倍数。以上 game.* 接口通过游戏协议 CommandRequest 发送，不借玩家聊天文本。命令链在机器人内部执行，不部署命令方块。

## Python

```python
# 参数也可由工作区 command-options.json 配置，不必硬编码。
result = game.execute("testfor @a", {"channel": "player", "timeout_ms": 3000})
if result.get("confirmed") and result.get("success"):
    game.execute("say 检测成功")
batch = game.executeBatch(["testfor @a", "list"],
                          {"channel": "ai", "mode": "pipeline", "window": 4})
```

## Lua

```lua
local options = game.getCommandOptions()
local result = game.execute("testfor @a", {channel="玩家", timeout_ms=3000})
if result.confirmed and result.success then game.execute("say 检测成功") end
local batch = game.executeBatch({"testfor @a", "list"},
                               {channel="ai", mode="pipeline", window=4})
```

## C++

```cpp
auto options = client.game.getCommandOptions();
auto result = client.game.execute("testfor @a", yy::json{{"channel","player"},{"timeout_ms",3000}});
if (result.value("confirmed", false) && result.value("success", false))
    client.game.execute("say 检测成功");
auto batch = client.game.executeBatch(yy::json::array({"testfor @a","list"}),
    yy::json{{"channel","ai"},{"mode","pipeline"},{"window",4}});
```

AI 调试可通过 sdk/call 对已启动插件调用上述方法；game_call 的 sendCommand/execute/executeBatch 也接受 options，但使用当前内核默认值，不继承指定插件的默认配置。先读取 getCommandOptions，调用后检查真实返回及终端日志。接口列表表示实现存在，具体通道仍须真实游戏回包验收。



## 整批协议发送（burst，实验性）
所有执行仍在机器人内，不放置命令方块。`game.executeBatch(commands, {mode:"burst",channel:"ai",transport:"command_request",window:1024,timeout_ms:8000})` 一次最多1024条、512KiB命令文本，将这段封装成一个网络批次，并逐条收集真实确认。整批默认 rate_cps=0（无固定CPS），0.10.4 可设置 rate_cps=20..20000，避免循环无限洪泛。网络批次相同不保证服务器同Tick执行，应依据实测首尾 `time query gametime` 和真实结果判断。
命令链 config.mode=burst、window=1..1024。无条件、相同通道、没有逐条延迟的连续段整批发送；遇到有条件命令、通道变化或延迟会等待结果，保持条件链语义。这些等待需要网络往返，不能承诺含条件的一千条都同Tick。remote严格逐条确认；pipeline窗口1..16；burst整批顺序提交，服务器实际顺序须验收。


## 0.10.4 完整附件与可调发送预算
AI工作区附件入口在对话输入区，ZIP保留目录结构，长粘贴保存文件；按需加载 toolbox-live-development 的 workspace-files.md（命令链也可从 command-chain 技能加载该章节）。用 project_read(offset,max_chars) 读取长行尾部，project_list(prefix,offset,limit) 分页列文件。
整批 game.executeBatch 的 options 和 chains.save/configure 的 config 都支持 rate_cps：0不加固定CPS等待，20..20000是配置的发送预算，非实际保证吞吐。UI可直接修改，条件等待仍是屏障，轮次不会重叠，回执超时不自动重发。


### 动态滑动窗口（0.10.4）

`game.executeBatch(commands, {mode="stream", channel="ai", window=512, rate_cps=0, timeout_ms=8000})`。Python/C++使用相同SDK options对象。stream单段1..4096条、文本512KiB，最大在途window=1..1024（默认128）。从16条起，成功回包累计达到当前窗口后倍增到上限；任何回包都释放槽位，不等待最早请求，也不等整窗全部回复。新补位命令立即按小批发送，不等待底层50ms刷新。`rate_cps=0`仅由在途窗口与回包背压约束；正数20..20000启用自定义共享账号发送预算。网络错误、超时、断开停止新发送，已发送回包仍收集，未知不自动重发。

命令链config同样支持`mode="stream",window=512,rate_cps=0`。有条件节点、通道变化、延迟、@call/@wait/@pause形成回包屏障。保持提交顺序；不保证独立命令的服务端完成顺序或同一游戏Tick。窗口仅服务于无真实依赖片段。整轮执行耗时算在周期内，仅等待剩余时间，不会堆积40Hz的未完成轮次。输出results逐条submitted/confirmed/success，peak_inflight是实测最大在途，network_batches是实际发送批数。
