# 模块与跨插件通信契约（0.10.19）

模块通信属于框架功能，优化时不得删除。分清三个层次：同插件模块共享常驻上下文；跨插件通过导出函数调用同账号的已启动实例；跨账号通过 accounts.callPlugin 和定向授权规则执行。一个账号的普通 api.require 不自动跳到另一个账号。

## 同插件模块

Lua `local counter=require("modules.counter")` 读取插件目录 `modules/counter.lua`。模块 `return {add=function(n) ... end}`；`package.loaded` 缓存结果，后续事件不重跑模块或 main.lua。Python 使用自己项目的模块导入，模块变量属于该插件常驻进程。C++ 把业务分到源文件/类，链接到同一个库；多个文件不等于多个可热重载 Runtime。整个插件重载才重建内存，需要跨重载保留的数据使用 Storage。

## Lua 导出与调用

目标插件 `counter_lua`：
```lua
local count = 0
plugin_api = {
  add = function(n) count=count+n; return count end,
  value = function() return count end
}
```
调用插件：
```lua
local counter=api.require("counter_lua")
if not counter then error("请先启动 counter_lua") end
local a=counter.add(2)
local b=counter.add(3) -- 同一个实例返回 5，不是重跑源码后的 3
```

## Python 导出与调用

目标插件：
```python
count = 0
def add(n):
    global count
    count += n
    return count
plugin_api = {'add':add,'value':lambda:count}
```
同账号调用 Lua/Python/C++：
```python
result = api.require('counter_lua').add(2)
result = api.require('counter_cpp').add(3)
```
`api.require` 返回代理；目标不存在或停用时在实际调用处报错。调用不会安装、自动启动或复制目标源码。

## C++ 导出与调用

```cpp
#include "youyuan.hpp"
class Counter : public yy::Plugin {
  long count_=0;
public:
  Counter(const char* ctx, yy::HostAPI host):Plugin(ctx,host) {
    expose("add",[this](const yy::json& args){
      count_+=args.at(0).get<long>(); return yy::json(count_);
    });
    expose("value",[this](const yy::json&){return yy::json(count_);});
    expose("sum_lua",[this](const yy::json& args){
      return client.api.require("counter_lua").call("add",args);
    });
  }
};
YY_PLUGIN(Counter)
```
`client.api.require(id).call(functionName,argsArray)` 调用同账号目标导出函数。Linux 为每个 C++ 插件加载一个独立常驻进程；Python3 仅负责 ctypes 与传输桥，业务在原生库中执行。Linux 库必须用 Linux 工具链构建，Android 库用 NDK；Windows 当前不能直接运行 ELF 库。

## 返回值、依赖与错误

跨语言支持字符串、数字、布尔、null、对象和数组；不支持跨进程传闭包、C++ 指针、Lua userdata 或打开的文件句柄。Lua→Lua 常驻代理直接转换值；跨语言兼容桥目前使用 JSON/IPC，有实际开销，不能称为零拷贝。

目标已经运行是前提。manifest dependencies 用于版本/启动依赖核对，不代替业务权限。普通并发调用排队；目标停止、缺少导出函数、参数错误、业务异常和调用环路应返回真实错误。禁止在 A→B 的同步导出回调里再调用 B→A；应返回后用 schedule/setTimeout 发下一步，避免彼此等待。不要捕获错误后假装成功，也不要靠重跑 main.lua 恢复状态。

`accounts.callPlugin(targetAccount,id,handler,args)` 属于显式跨账号通信，需要源插件、目标账号与操作类型对应规则。不能使用普通 `api.require` 绕过这些规则。

## 高频命令与通信

模块可以生成或启动同一原生命令链。每条链的 Hz 属于完整的一轮，不能变成每个节点的 timer。跨插件调用不应放在每条游戏命令之前；业务模块先准备命令数组，再通过 CommandBuffer 或 sendCommandsFast 整体提交。无条件快速命令只发送；需要真实成功结果的条件节点才使用确认路径。计分变化和服务端结果用于验证执行，IPC 返回不能证明同一 Tick 完成。

## 停用与崩溃

停用清理该实例的 timer、事件订阅、worker 和常驻状态。Linux C++ 进程崩溃/卡死只回收该子进程，并经正常停用流程清理宿主任务；业务异常保留对象。默认 handler 卡死超时 120 秒，可设 YY_NATIVE_TIMEOUT；不是命令速率限制。Android 原生运行器目前仍在宿主进程加载，不能声明具有 Linux 的进程隔离能力。


## 0.10.19 宿主兼容行为

旧 Lua 插件不需要为以下行为额外加配置：
- `api.require(id)` 调用已加载插件的 `plugin_api`。普通并发调用排队；真实循环调用报错，避免死锁。全局变量与 require 模块缓存常驻。
- `util.read_file/json_load`、`io.open`、`dofile/loadfile` 自动解析当前安装目录。读文件依次检查已有宿主相对路径、插件数据、插件源码和 resources。相对写入默认进入插件数据目录；`插件数据文件/id/...` 等旧路径映射到实际安装根目录。绝对路径仍受当前系统权限约束，Windows 路径不能原样访问 Linux 文件。
- `game.waitMsg`、`game.httpGet/httpPost`、`http.*` 在等待 I/O 时让出 Lua 执行锁。回调使用独立调用栈，共享原 VM 的变量、模块和闭包；不重跑 main.lua。等待期间同插件定时器可执行，停止插件取消等待。纯 Lua 指令仍串行，同一个 timer 防重入。其他未经适配的阻塞 API 和插件纯计算长循环仍可能延迟计时器，不能承诺所有阻塞都自动并行。
- 0.5 秒计时器仍写 `util.schedule(0.5, "handler")`。频率属于完整回调/整条链，非每条指令。计时器不逐轮补积压；可以从 `util.listJobs()` 查看 calls、missed 和耗时。
- Python 跨语言无参数调用接受空数组和 null；不会把 null 当成可展开的函数参数。
- Linux C++ 使用真实 .so 常驻子进程，插件崩溃仅结束该进程并清理 SDK/定时器。仍使用一次 JSON/HTTP/IPC 桥接，不是零开销。Windows 尚不加载 ELF；Android 原生库仍为应用同进程。

AI 修改插件时先读取真实源码和本节对应接口，不要求玩家改写为另一套模块语法。修改后进行语法检查与热重载，只报告实测结果。
