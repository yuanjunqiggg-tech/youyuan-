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

### C++（Android原生宿主）
```cpp
auto profiles = client.accounts.list();
auto state = client.accounts.status("account_b");
client.accounts.sendCommandsFast("account_b", yy::json::array({"say A","say B"}), yy::json{{"channel","ai"}});
auto result = client.accounts.callPlugin("account_b","target_plugin","balance",yy::json::array({"Player"}));
```
Linux 原生 C++ 插件宿主仍未接入；不要把安卓版 .so 当成 Linux 插件。

## 防止互相等待

跨账号 callPlugin 的导出回调里，禁止再同步调用 accounts.*，避免 A 等 B、B 又等 A 的环路。应先返回结果，再通过 schedule/setTimeout 发下一次协作。阻断在当前进程级生效；同一时段其他插件的跨账号调用可能被拒绝，应延后任务。拒绝信息可供 AI 查错，不重跑插件源码。

这是协调授权，不是对恶意本地代码的安全沙箱：本机插件本就有系统/HTTP权限。云端仍依赖平台用户和设备隔离；不能给其他订阅用户分配你的私有节点后又期望他们无法控制此节点。

## 远端与内置 AI

MCP api_youyuan_accounts_bridge_rules 管理规则；api_youyuan_accounts_bridge_execute 调用 `{source,plugin,target,operation,args}`，属于控制权限。内置 AI 可按需加载本技能，读取接口目录和错误；不能只知道插件名称。内部 `/bridge/local` 不列入 MCP，不允许通过账号代理嵌套绕过授权。

## 验证范围

代码回归验证定向授权、源插件限制、操作限制与接口注册；发布验收验证真实进程之间状态调用。没有在本轮新增游戏压测，发消息、游戏命令副作用及跨语言目标插件的游戏联调留待玩家批准恢复测试。
