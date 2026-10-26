# 2026-10-22 紧急修复与验证入口

原文件保留于远端 `/var/backups/youyuan/20261022-0005-autostart`，三份补丁上传时保留各文件UID/GID/权限。

- `yy_txt_autostart_r64.lua`：重连尊重原生链启停；手动全开/全停仍有效。
- `native_bridge.lua`：初始化不以旧注册表覆盖已有原生配置；游戏菜单读取原生启停。
- `yy_gun_native_r65.lua`：生成枪械程序保留既有启停、频率、速率配置。
- 三份startup_test/registry_state_test：用真实gopher-lua执行断言，不以测试冒充游戏验收。

共用队列配置接口（客户端原“宿主观测容量”，MCP `api_call`，HTTP均同路）：

```json
{"method":"POST","path":"/api/youyuan/game/call","body":{"method":"diagnostics.configure","args":[{"event_queue":256,"priority_queue":256}]}}
```

读取时args=[]；容量1..65536，既有其他配置不变。保存后需重载插件；实际值从`/api/plugin/profiler?id=<插件ID>`的queue_capacity/priority_queue_capacity核对。不要把队列容量当并发数、包类型数或命令CPS。扩大容量不能修复持续积压。

最新验收见外层`outputs/2026-10-21-当前紧急修复清单.md`，原菜单完整功能与渐增卡顿尚未验收，不可标完成。


04:39新增：native_bridge.lua统一游戏频率到chains.configure，停用链也保存，失败不报成功，读取原生hz到注册表。cadence_test.lua为对应断言。原TXT不改；主菜单配置1Hz，当前仍停止。线上发布和完整分钟只读测试见外层紧急清单。
