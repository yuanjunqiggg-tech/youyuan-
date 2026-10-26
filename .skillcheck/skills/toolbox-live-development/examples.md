# 内置实例、导出和开发入口

实例通过 GET /api/youyuan/runtime/list 获取真实存在与状态；POST /api/youyuan/runtime/start 或 stop 传 id/account。原生示例依赖 APK 对应 ABI 的内置库，桌面运行不保证自动存在。它们默认由用户启动；不要自动启用会发欢迎消息的插件。

| 实例 ID | 语言 | 能力及入口 |
| --- | --- | --- |
| yy_sdk_python_demo | Python | 游戏聊天触发 `.插件状态`，持久计数、2 秒计时、玩家加入回调；控制页 control/status；导出 add/state/sum_cpp/execute。 |
| yy_sdk_cpp_demo | C++ | 原生游戏控制、定时和触发示例；源码与实际导出请读 SDK 的 cpp_seed.cpp 和 youyuan.hpp。 |
| yy_sense_python_demo | Python | 血量、受伤、死亡、移动、重生、方块变化监听；UpdateAttributes 250ms 过滤。控制页 monitor 的 scene/capture_start/capture_stop；导出 scene/metrics/batch_probe/health/capture_start/capture_stop。 |
| yy_sense_cpp_demo | C++ | 相同感知监听，原生对象保存计数；monitor 网页动作；导出 scene/metrics/health/batch_probe。捕获在停止对象时释放。 |

通过 POST /api/youyuan/sdk/call 调用已启动实例，JSON 字段 plugin_id、account、method、args。例如：

```json
{"plugin_id":"yy_sense_python_demo","account":"root","method":"game.scene","args":[]}
```

这读取宿主方法；跨插件导出函数使用本语言 `api.call("yy_sense_python_demo","metrics",[])` 或 Python `api.require("yy_sense_python_demo").metrics()`。调用前确认目标已运行，同实例隔离范围内操作；目标忙碌、递归调用和不存在的导出返回错误。控制页入口从 GET /api/youyuan/sdk/pages 发现；网页动作可用 web.invoke("monitor","scene",{})，不编造未注册动作。

Python 新工作区含可修改入口模板；C++ 新工作区含 youyuan.hpp、third_party/json.hpp 和入口模板。Python 使用 on_init/on_event/on_stop、原生使用 YY_PLUGIN 与 yy::Plugin；四个完整示例也已嵌入技能，调用 load_skill("toolbox-live-development",section="python_seed.py")、section="cpp_seed.cpp"、section="python_observer.py"、section="cpp_observer.cpp" 直接读取真实代码，再用 project_write/patch 生成或修改项目。源码同样位于交付包 upstream/sdk。技能读取不会自动把全部示例写入每个项目。

示例中的模拟事件只能检验回调语法和状态保留；验证真实死亡必须保存来自游戏的事件、实体标识、模式和时间。monitor 显示未知血量是正确观测，不要用 20 覆盖它。修改示例发布后重新加载；仅改磁盘文件不会替换当前运行模块。
