# 插件运行时知识库（游园工具箱 / MCORT）

> 核对基线：`C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project`，仓库 HEAD `3a11d92`（2026-10-21），Go 源码根 `upstream/`。
> 本文所有 `文件:行` 均指该源码根下的真实行号（读源码得出，不是抄文档）。
> 本文只描述**运行时事实**，不含部署、版本戳、构建步骤。

---

## 0. 一句话结论

项目里**真实存在的插件宿主只有 3 个**，且不是同一套机制：

| # | 宿主 | 机制 | 是否真实存在 |
|---|---|---|---|
| 1 | Lua | 进程内 gopher-lua 常驻 VM（`third_party/gopher-lua`），每账号每插件一个 LState | 是 |
| 2 | Python | 独立 CPython 3.12 实例：Android 用 Chaquopy 内嵌，桌面/Linux 用 python3 子进程 | 是 |
| 3 | C++（`native_cpp`） | 编译为 .so，Android 用 dlopen 进程内加载；Linux 由 Python ctypes worker 加载 | 是 |

**JavaScript / Node / QuickJS / goja 宿主：不存在。**
核对方式：`grep -rn "goja|quickjs|otto" --include=*.go upstream` 无命中（仅命中 show_bottom 等无关词）；`youyuan_import.go:55` 的导入校验只放行 python 与 native_cpp；`youyuan_runtime.go:129-143` 的启动 switch 只有这两个分支。根目录也没有任何 JS 插件开发说明。

另外有一个**"第四语言"的幽灵**：`/api/youyuan/runtime/list` 会向调用方宣告 `languages: ["python","native_cpp","lua_compat"]`（`youyuan_runtime.go:281`），但启动接口没有 lua_compat 分支（`youyuan_runtime.go:129-143`），全仓除这一行外再无 lua_compat 任何实现。详见第 7 节。

---

## 1. 先分清两套互不相通的插件体系

读源码最容易踩的坑：项目里有**两个**插件目录、**两套**清单文件、**两个**启停入口，彼此不扫描对方。

| 维度 | 体系 A：Lua 插件系统 | 体系 B：多语言运行时 |
|---|---|---|
| 控制器 | `PluginController`（`plugin_controller.go:29`） | `yyRuntimeInstance` 注册表（`youyuan_runtime.go:43`） |
| 根目录 | `插件文件/`（`plugin_controller.go:167`） | `多语言插件/`（`youyuan_runtime.go:49`） |
| 清单 | `<id>.manifest.json`（`plugin_scan.go:24`） | `manifest.json`（`youyuan_runtime.go:84`） |
| 入口 | `.lua` 或词库 `.json`（`plugin_scan.go:60`） | `main.py` 或 `plugin.so`（manifest.entry） |
| 语言 | 仅 Lua + 词库 | 仅 Python + C++（`youyuan_import.go:55`） |
| 导入 | `POST /api/plugin/import`（`main.go:5064`）→ `ImportPackage`（`plugin_pack.go:56`） | `POST /api/youyuan/runtime/import`（`youyuan_runtime.go:52`）→ `yyUnpackPlugin`（`youyuan_import.go:15`） |
| 启停 | `/api/plugin/run`、`/api/plugin/stop`、`/api/plugin/reload`、`/api/plugin/delete`（`main.go:5059-5064`） | `/api/youyuan/runtime/start`、`/stop`、`/event`（`youyuan_runtime.go:60,196,168`） |
| 配置 | `插件配置文件/<id>/<id>.json`（`plugin_config.go:10`） | `command-options.json` + 插件自管 `data/<account>/` |
| 数据 | `插件数据文件/<id>/`（`plugin_config.go:15`） | `data/<account>/`（`youyuan_runtime.go:131`） |
| 禁用持久化 | `disabled.json`（`plugin_controller.go:61`） | `resident-runtime-enabled.json`（`youyuan_runtime_restore.go:45`） |
| 随连接自动加载 | 是，`AutoLoadPlugins`（`bot_conn.go:1293`） | 是，10 秒轮询恢复（`youyuan_runtime_restore.go:49-94`） |

一个 `.lua` 包放进 `多语言插件/` 不会被任何一方加载；一个 `main.py` 包走 `/api/plugin/import` 会被直接拒绝（`plugin_pack.go:170-172` 只认 main.lua 与 词库.json）。

---

## 2. 插件清单字段表

### 2.1 体系 A：`<id>.manifest.json`（Lua / 词库）

结构体：`plugin_scan.go:12-20`；读取函数 `readPluginManifest`：`plugin_scan.go:23-34`（文件不存在或解析失败**静默返回 nil**，不报错、不阻止加载）。

| 字段 | 类型 | 必填 | 含义 | 源码位置 | 真实落点 |
|---|---|---|---|---|---|
| `id` | string | 否 | 名义唯一 id | `plugin_scan.go:13` | **扫描时被忽略**：真实 id 由文件名/目录名推导（`plugin_scan.go:63-67`）。`manifest.id` 只在 ZIP 导入时决定安装目录与代码文件名（`plugin_pack.go:127-136,141`） |
| `name` | string | 否 | 展示名 | `plugin_scan.go:14` | `PluginDef.Name`（`plugin_scan.go:78-80`），缺省用文件名 |
| `version` | string | 否 | 版本号 | `plugin_scan.go:15` | `PluginDef.Version`（`plugin_scan.go:88`）；用于 pre_plugins 版本比较（`plugin_controller.go:379`） |
| `description` | string | 否 | 简介 | `plugin_scan.go:16` | `PluginDef.Description`（`plugin_scan.go:81-83`） |
| `type` | string | 否 | lua 或 wordbank | `plugin_scan.go:17` | `PluginDef.Type`（`plugin_scan.go:84-86`）；缺省按扩展名推（`plugin_scan.go:69-72`） |
| `author` | string | 否 | 作者 | `plugin_scan.go:18` | **解析后丢弃**：`PluginDef` 无 Author 字段（`plugin_defs.go:4-15`）。见 7.6 |
| `pre_plugins` | object<string,string> | 否 | 前置插件 id → 最低版本 | `plugin_scan.go:19` | `PluginDef.PrePlugins`（`plugin_scan.go:87`），加载前递归解析并可自动拉起（`plugin_controller.go:368-399`） |

配套文件：

| 文件 | 作用 | 源码位置 |
|---|---|---|
| `<id>.docs.md` | 插件 Markdown 文档，进 `PluginStatus.Docs` | `plugin_scan.go:37-43` |
| `config.json`（仅 ZIP 内） | 导入时写入 `插件配置文件/<id>/<id>.json` | `plugin_pack.go:199-206` |
| `lib/`、`resources/` | 原样保留；`package.path` 指向代码根，可 `require` | `plugin_pack.go:207-230`、`youyuan_resource.go:201-212` |

ZIP 内命名与磁盘命名不同（导入器改名，**两段流程，不算不一致**）：

| ZIP 根 | 磁盘 | 源码 |
|---|---|---|
| `manifest.json` | `<id>.manifest.json` | `plugin_pack.go:189` |
| `main.lua` | `<id>.lua` | `plugin_pack.go:141,185` |
| `docs.md` | `<id>.docs.md` | `plugin_pack.go:193-196` |
| `词库.json` | `<id>词库.json` | `plugin_pack.go:142-145` |

### 2.2 体系 B：`manifest.json`（Python / C++）

结构体：`yyRuntimeManifest`，`youyuan_runtime.go:15-23`；导入校验：`youyuan_import.go:55`。

| 字段 | 类型 | 必填 | 含义 | 源码位置 | 校验/行为 |
|---|---|---|---|---|---|
| `id` | string | 是 | 唯一 id，同时是安装目录名 | `youyuan_runtime.go:16` | 必须与请求 id、目录名一致（`youyuan_runtime.go:98`）；禁 `/ \ :`、`.`、`..`、保留名 native_counter（`youyuan_import.go:55`、`youyuan_runtime.go:77`） |
| `name` | string | 否 | 展示名 | `youyuan_runtime.go:17` | 仅展示 |
| `language` | string | 是 | python 或 native_cpp | `youyuan_runtime.go:18` | 白名单硬校验（`youyuan_import.go:55`）；其它值启动时报「当前内置运行时不支持 X」（`youyuan_runtime.go:142`） |
| `entry` | string | 是 | 入口文件名 | `youyuan_runtime.go:19` | 必须是直接文件名且必须存在于包内（`youyuan_import.go:55`、`youyuan_runtime.go:98`） |
| `events` | string[] | 否 | 事件类型白名单 | `youyuan_runtime.go:20` | 非空时只投递名单内类型；为空时除 packet 外全投（`youyuan_runtime.go:413-426`） |
| `version` | string | 否 | 版本号 | `youyuan_runtime.go:21` | 依赖比较要求纯数字点分（`youyuan_runtime_dependencies.go:9-21,33-39`） |
| `dependencies` | object<string,string> | 否 | 前置插件 id → 最低版本 | `youyuan_runtime.go:22` | 前置必须**已在运行**，不自动拉起（`youyuan_runtime_dependencies.go:29-32`）；`*` 表示不限（`:33`）；禁止依赖自己（`:26-28`） |

### 2.3 词库插件（wordbank）

`.json` 结尾即词库插件（`plugin_scan.go:69-72`），加载时 `loadWordBank` 解析并注册为聊天触发项（`plugin_scan.go:105-119`）。没有生命周期钩子，没有 `plugin_api`，不能跨插件被调用。

### 2.4 目录约定总表

| 路径 | 用途 | 源码位置 |
|---|---|---|
| `插件文件/<id>/<id>.lua` | 标准入口（第一候选） | `plugin_controller.go:182` |
| `插件文件/<id>/main.lua` | 兼容入口（第二候选） | `plugin_controller.go:182` |
| `插件文件/<id>/<id>.json`、`<id>词库.json` | 词库入口（第三、四候选） | `plugin_controller.go:182` |
| `插件文件/<id>.lua`（裸文件，历史布局） | 允许，但删除/导出受限 | `plugin_paths.go:32-70` |
| `插件配置文件/<id>/<id>.json` | 插件配置（自由 JSON） | `plugin_config.go:10-12` |
| `插件数据文件/<id>/` | 插件持久数据 | `plugin_config.go:15-19` |
| `插件数据文件/<id>/storage.json` | storage.* 落盘文件 | `youyuan_storage_cache.go:55` |
| `disabled.json` | 持久禁用集合 | `plugin_controller.go:61` |
| `多语言插件/<id>/manifest.json` | 运行时清单 | `youyuan_runtime.go:84` |
| `多语言插件/<id>/data/<account>/` | 运行时按账号隔离数据 | `youyuan_runtime.go:131` |
| `多语言插件/<id>/command-options.json` | 运行时命令默认参数 | `youyuan_sdk.go:70-77` |
| `resident-runtime-enabled.json` | 需开机恢复的实例清单 | `youyuan_runtime_restore.go:45` |
| `插件数据文件/ds_ai_agent/command_txt/` | 命令链 TXT 根目录 | `youyuan_commands.go:722` |

---

## 3. 三种宿主对照表

| 维度 | Lua | Python | C++（native_cpp） |
|---|---|---|---|
| 所属体系 | A（PluginController） | B（yyRuntime） | B（yyRuntime） |
| 入口文件约定 | `插件文件/<id>/<id>.lua` 或 `main.lua`（`plugin_controller.go:182`） | `多语言插件/<id>/` + `manifest.entry`（通常是 `main.py`）（`youyuan_runtime.go:131`） | `多语言插件/<id>/` + `manifest.entry`（通常是 `plugin.so`）（`youyuan_runtime.go:133-136`） |
| 入口函数名 | **无固定名**：整个 chunk 执行一次（`youyuan_resident.go:186-190`）；导出函数用全局函数名或 `plugin_api` 表（`youyuan_resident.go:331-340`） | **无 main()**：模块被 `exec_module` 加载即执行（`youyuan_plugins.py:194`）；导出用模块级 `plugin_api` dict（`youyuan_plugins.py:97-98`） | **`yy_create_v2`**（回退 `yy_create`）/ `yy_event` / `yy_destroy`（`sdk/cpp/youyuan.hpp:378-381`；`sdk/native_worker.py:66-76`） |
| 初始化钩子 | 无 `on_init`（全仓无此符号）。顶层代码就是 init | `on_init(host)`，可 async（`youyuan_plugins.py:195`） | 构造函数（`Plugin(const char*, HostAPI)`），内部 `on()`/`expose()` 注册（`sdk/cpp_seed.cpp:5-20`） |
| 事件钩子 | `events.onChat/onJoin/...` 注册**全局函数名**（`lua_events.go:124-152`）；`events.on`/`onXxx` 走感知钩子（`youyuan_sense_api.go:173-235`） | `events.onChat(handler)` 传可调用对象，宿主转成 `py_callback_N`（`youyuan_plugins.py:26,76-81`）；未命中时回落到 `on_event(event, host)`（`youyuan_plugins.py:102-103`） | `on("handler", callback)` + `client.events.onChat("handler")`（`sdk/cpp_seed.cpp:7,13`）；未命中时走 `Plugin::event()`（`sdk/cpp/youyuan.hpp:365`） |
| 卸载钩子 | 全局 `on_stop()`，**不传参**，2 秒超时（`youyuan_resident.go:245-253`） | `on_stop(host)`，可 async，结束后 `close_async()` 关 asyncio 循环（`youyuan_plugins.py:203-205`） | 析构函数（`yy_destroy` → `delete`，`sdk/cpp/youyuan.hpp:381`） |
| 能拿到的 API | 25 个命名空间直接挂全局表（`lua_api.go:15-53`），另有 `game.*` 旧兼容函数（`lua_game_legacy.go:20`） | `host.<ns>` 或模块级 `<ns>`，同样是 25 个命名空间（`youyuan_plugins.py:71,191-193`）；调用经 HTTP 回宿主（`youyuan_plugins.py:83-85`） | `client.<ns>`，内部同样走 `/api/youyuan/sdk/call`（`sdk/cpp/youyuan.hpp:340-343`） |
| API 命名空间清单 | 25 个：game, system, api, plugin, util, bot, building, media, world, player, events, http, web, chains, packet, storage, accounts, accountsPool, **mcp**, entities, entity, diagnostics, hud, scoreboard, observerBridge（`yySDKNames` 静态字面量是 24 个，`youyuan_sdk.go:20`；`mcp` 由 `youyuan_plugin_mcp.go:21` 的 `init()` 追加，故运行时 25 个，见 7.3） | 同上（`youyuan_plugins.py:71,191`，字面量里含 `mcp`） | 同上（`sdk/cpp/youyuan.hpp:327,329` 的 `Client` 动态命名空间，含 `mcp`） |
| 执行隔离 | 进程内 goroutine，共享进程 | Android 进程内（Chaquopy）；Linux/桌面独立子进程（`youyuan_runtime_desktop.go:84-98`） | Android 进程内 dlopen（`youyuan_runtime_android.go:18`）；Linux 由 Python worker 子进程 ctypes 加载（`youyuan_native_desktop.go:66-67`） |
| 并发模型 | 每账号每插件一个 LState，`r.mu` 串行（`youyuan_resident.go:273`），回调可复用 idle thread（`youyuan_resident.go:287-296`） | 每实例一个 asyncio 事件循环线程（`youyuan_plugins.py:59-70`），事件串行；批量最多 16 条一次投递（`youyuan_runtime.go:359-368`） | 每实例一个 worker 进程，`p.mu` 串行（`youyuan_native_desktop.go:98-99`） |
| 单次调用上限 | 2 分钟（`lua_botctx.go:14` `luaExecTimeout`） | 120 秒（`youyuan_plugins.py:109` await 超时） | 120 秒，可用 `YY_NATIVE_TIMEOUT` 覆盖（`youyuan_native_desktop.go:120-123`） |
| 主要限制 | 仅 Lua 5.1 语义；无 `on_init`；`on_stop` 无参数；裸文件布局不能删除/导出（`plugin_paths.go:74-84`） | 必须装 Python3（桌面），否则 `yyPythonValidate` 直接报错（`youyuan_runtime_desktop.go:42-44`）；模块级命名空间注入依赖 import 顺序 | 必须 64 位小端 ELF 且 arch 匹配（`youyuan_native_library.go:20-30`）；Windows 明确不支持（`youyuan_native_desktop.go:41-42`）；Linux 额外需要 Python3（`:63-65`） |
| 平台支持 | 全平台 | Android（Chaquopy 3.12，`android/app/build.gradle:1,22`）+ Linux/桌面（`python3`/`YY_PYTHON`） | Android arm64-v8a / x86_64（dlopen）+ Linux x86_64/aarch64（ctypes worker）；Windows 无 |

补充事实：

- Python 与 C++ 共用同一个 Go 侧适配层 `yySDKContext`，它内部**用一个隐藏的 Lua VM 当函数表**（`youyuan_sdk.go:20`、`youyuan_sdk.go:269-288` 的 catalog 就是遍历这个 LState 生成的）。所以三语言 API 名称能保持一致，原因是同一个注册函数 `registerAllLuaAPI`（`lua_api.go:15`）。
- C++ 的 `expose()` 注册的是 `sdk_rpc` 处理器，`on()` 注册的是 `sdk_callback` 处理器（`sdk/cpp/youyuan.hpp:365-370`）；Python 同理（`youyuan_plugins.py:94-99`）。
- Python/C++ 的数据目录按账号隔离（`youyuan_runtime.go:131` 传 `data_dir`），Lua 的数据目录不按账号隔离（`plugin_config.go:15-19`）。

---

## 4. 生命周期

### 4.1 Lua 体系：扫描 → 注册 → 加载 → 运行 → 停止 → 删除

| 阶段 | 触发点 | 关键函数 | 源码位置 |
|---|---|---|---|
| 扫描 | 启动时、导入后、手动 reload（未连接时） | `ScanPlugins` | `plugin_controller.go:165`（调用点：`main.go:741,787,2548,4968`、`youyuan_market.go:1139`） |
| 扫描细节 | 遍历 `插件文件/`，目录按 4 个候选入口找代码，裸文件按扩展名过滤 | `ScanPlugins` 主体 | `plugin_controller.go:169-209` |
| 构建定义 | 读清单/文档，闭包 `Load` 内延迟读源码 | `BuildDefFromFile` | `plugin_scan.go:51-127` |
| 去重注册 | 同 id 不同入口直接报冲突，不覆盖 | `registerScannedDef` | `plugin_controller.go:217-237` |
| 注册 | 写入 `defs` map | `AddDef` | `plugin_controller.go:246-254` |
| 加载 | `/api/plugin/run`、连接后自动加载、`Reload` | `Run` | `plugin_controller.go:313` |
| 采集内核拦截 | `privateDir/capture-only` 存在时拒绝启动任何业务插件 | `yyCapturePluginPermission` | `plugin_controller.go:303-310` |
| 前置依赖 | 递归加载 `pre_plugins`，检测成环、比较版本 | `ensureDependencies` | `plugin_controller.go:368-399` |
| 配置文件就绪 | 建数据目录、缺配置写 `{}` | `EnsurePluginFiles` | `plugin_controller.go:345`、`plugin_config.go:84-89` |
| 编译并执行一次 | 注册全部 API → 注入 `__PLUGIN_ID__`/`__PLUGIN_SRC__`/代码目录 → `Load` + `PCall` | `def.Load` → `LoadLuaPlugin` → `yyLoadResident` | `plugin_scan.go:104-125`、`lua_api.go:926-929`、`youyuan_resident.go:133-216` |
| 就绪 | 编译失败 → 清订阅、关 VM、置 error；成功 → 起 3 条感知队列 goroutine，必要时补发 bot_ready | `yyLoadResident` | `youyuan_resident.go:191-215` |
| 运行 | 聊天/事件/定时器回调复用同一 VM，只调用函数不重跑源码 | `yyCallResidentState` | `youyuan_resident.go:266-357` |
| 加载失败回滚 | 移除触发器/事件/定时器/网页 | `Run` 失败分支 | `plugin_controller.go:346-356` |
| 停止 | `/api/plugin/stop`、断开连接、删除前 | `Stop` | `plugin_controller.go:430` |
| 停止细节 | `RemovePluginTriggers` → `RemovePluginEvents` → `RemovePluginSchedules` → `RemovePluginWeb` → `yyStopResident` | `Stop` | `plugin_controller.go:438-442` |
| 卸载钩子 | 取消 lifetime、清 HUD/命令票据/感知订阅，2 秒内调 `on_stop()`，然后关 LState | `yyStopResident` | `youyuan_resident.go:218-257` |
| 热重载 | 先停后起；被持久禁用的插件只重读不重启 | `Reload` | `plugin_controller.go:492-514` |
| 删除 | 停 VM、清运行态、删 `插件文件`/`插件配置文件`/`插件数据文件` 三棵树（异步 rename 到 `.trash`） | `Delete` / `yyRemoveTreeDeferred` | `plugin_controller.go:518-614`、`:462-490` |
| 自动加载时机 | 机器人连接并取得 OP 后 | `AutoLoadPlugins` | `bot_conn.go:1293` → `plugin_controller.go:126-143` |
| 断开停用 | 断开连接时全部瞬态停用，不改禁用状态 | `StopAllPlugins` | `bot_conn.go:1332` → `plugin_controller.go:146-161` |

未连接时的行为差异（容易误解）：

- `run`：不报错，返回 `queued:true`，连上后自动加载（`main.go:679-691`）。
- `reload`：不报错，只重新 `ScanPlugins`（`main.go:736-745`）。
- `stop`：幂等成功，`stopped:false`（`main.go:711-716`）。
- `delete`：不要求连接（`main.go:766`）。

### 4.2 Python / C++ 体系：导入 → 启动 → 事件循环 → 停止 → 恢复

| 阶段 | 触发点 | 关键函数 | 源码位置 |
|---|---|---|---|
| 导入 | `POST /api/youyuan/runtime/import`（multipart，上限 65MB） | `yyRuntimeImport` → `yyUnpackPlugin` | `youyuan_import.go:103-133`、`:15-90` |
| 导入校验 | 路径穿越/重复路径/符号链接/总大小 64MB/文件数 5000；`manifest.id`、`language`、`entry` 必须合法 | `yyUnpackPlugin` | `youyuan_import.go:26-57` |
| 启动 | `POST /api/youyuan/runtime/start`（`id`,`account`,`restore`） | 内联 handler | `youyuan_runtime.go:60-167` |
| 清单读取 | 读 `<dir>/manifest.json`；`native_counter` 为内置特例 | 同上 | `youyuan_runtime.go:81-93` |
| 一致性校验 | `manifest.ID == id`，`entry` 是直接文件名 | 同上 | `youyuan_runtime.go:98-101` |
| 手动停用保护 | `restore:true` 时若已从 choices 移除则拒绝恢复 | 同上 | `youyuan_runtime.go:105-113` |
| 依赖校验 | 前置必须已运行 | `yyRuntimeCheckDependencies` | `youyuan_runtime.go:121`、`youyuan_runtime_dependencies.go:22-43` |
| 适配层准备 | 建 `yySDKContext`，读 `command-options.json` | `yySDKPrepare` | `youyuan_runtime.go:126`、`youyuan_sdk.go:64-81` |
| 语言分派 | `python` → `yyPythonRuntime("start")`；`native_cpp` → `yyValidateNative` + `yyNativeStart` | 同上 | `youyuan_runtime.go:129-143` |
| 启动失败清理 | `yySDKClose` + Python stop | 同上 | `youyuan_runtime.go:151-158` |
| 运行 | 每实例 `runEvents` goroutine 串行消费队列；Python 支持 16 条批投 | `runEvents` | `youyuan_runtime.go:162,341-401` |
| 事件投递 | 只投本账号机器人、按 manifest.events 白名单过滤、队列满则丢弃并计数 | `yyRuntimeEmit` | `youyuan_runtime.go:404-434` |
| 停止 | `POST /api/youyuan/runtime/stop` | 内联 handler | `youyuan_runtime.go:196-237` |
| 停止细节 | 置 Stopped、关 Done、Python `stop`（触发 `on_stop`）或 `yyNativeStop`、`yySDKClose` | 同上 | `youyuan_runtime.go:224-235` |
| 崩溃自愈 | 原生传输错误 → 标记 Fatal → 异步调 stop 防幽灵回调 | `eventLocked` | `youyuan_runtime.go:315-326` |
| 自动恢复 | 每 10 秒按 `resident-runtime-enabled.json` 重新 start | `yyRestoreRuntimeLoop` | `youyuan_runtime_restore.go:49-94` |
| 删除 | 停所有实例 → 目录 rename 成 tombstone → 后台删 | `/api/youyuan/runtime/delete` | `youyuan_runtime_management.go:66-136` |

### 4.3 三语言生命周期的关键不对称

| 对比项 | Lua | Python | C++ |
|---|---|---|---|
| 初始化失败如何表现 | `Run` 返回 error，状态置 `error`，已注册项被回滚（`plugin_controller.go:346-356`） | `dispatch` 返回 `{ok:false}`，进程被杀、`yySDKClose`（`youyuan_runtime.go:151-158`） | `yy_create_v2` 返回空指针 → 报「初始化失败」（`sdk/native_worker.py:77-78`） |
| 停用时是否保证清理 | 是，`yyStopResident` 固定顺序 | 是，`on_stop` 后 `close_async` 取消全部任务（`youyuan_plugins.py:121-132`） | 是，`yy_destroy` 析构；进程被杀兜底 |
| 重载语义 | 先 `Stop` 再 `Run`（`plugin_controller.go:502-513`） | 无热重载接口，只有 stop + start | 同 Python |
| 依赖是否自动拉起 | 是（`plugin_controller.go:392-396`） | 否，必须手动先启动（`youyuan_runtime_dependencies.go:29-32`） | 否 |

---

## 5. 插件能调的宿主 API 全量

### 5.1 先理解分发机制（决定"有没有这个 API"怎么判）

- **Lua**：`registerAllLuaAPI` 依次注册 21 组 API（`lua_api.go:15-53`），大部分是 `L.SetField(表,"名字",函数)`；另有一大批方法由**表驱动**注册：`registerSenseAPI` 遍历 `yyExtraMethods`（`youyuan_sense_api.go:136-301`），按 `"命名空间.方法"` 自动挂到对应全局表。
- **Python/C++**：不各自实现，统一走 `/api/youyuan/sdk/call`（`youyuan_sdk.go:290`），落到 `yySDKContext.call` → `senseCall` → `yySenseCall`（`youyuan_sense_api.go:337`）。未知方法报「未知感知方法 X；请读取 sdk/catalog」（`youyuan_sense_api.go:681`）。
- **权威清单**：`GET /api/youyuan/sdk/catalog` 运行时用临时 LState 反射出所有命名空间与函数名（`youyuan_sdk.go:269-288`），返回 `sdk_version:"1.17"`。判断方法是否存在应以该接口为准，本文表格是静态等价物。
- **批量**：`POST /api/youyuan/sdk/batch`，1..64 次调用，遇错即停、不回滚（`youyuan_sdk.go:220-255`）。
- **外部入口**：`POST /api/youyuan/game/call {method,args,account}`；需要插件上下文的 `plugin.*`/`util.*`/`plugin.stats` 会被拒绝（`youyuan_game_dispatch.go:39`）。

方法总数（静态提取，含少量字符串常量噪声）：**约 350 个**。

| 命名空间 | 约数 | 主要来源 |
|---|---|---|
| game | 77 | `lua_api.go:74-800`、`lua_game_legacy.go:20`、`youyuan_plugin_commands.go:271` |
| events | 47 | `lua_events.go:124`、`youyuan_sense_api.go:21-73`、`youyuan_entity_observations.go:36`、`youyuan_module_combat.go:17`、`youyuan_server_observation.go:72` |
| observerBridge | 27 | `youyuan_observer_bridge.go:18`、`youyuan_observer_patrol.go:18`、`youyuan_world_observer.go:36`、`youyuan_server_observation.go:72` |
| util | 26 | `lua_util.go:121`、`youyuan_scheduler.go:12`、`youyuan_entity_observations.go:48-49` |
| plugin | 22 | `youyuan_resource.go:13`、`youyuan_profiler.go:24`、`youyuan_capabilities.go:69`、`lua_api_cross.go:70` |
| player | 20 | `lua_player.go:29`、`youyuan_entity_observations.go:41-42`、`youyuan_sense_api.go:46-47` |
| building | 20 | `lua_building.go:11`、`youyuan_nbt_operations.go:17`、`youyuan_building_jobs.go:12` |
| accountsPool | 15 | `youyuan_sense_api.go:22-31`、`youyuan_capture_runtime.go`、`youyuan_observer_handoff.go` |
| bot | 13 | `lua_bot.go:8`、`youyuan_sense_api.go:49-53`、`youyuan_visibility.go:15` |
| chains | 13 | `youyuan_chain_api.go:20-34` |
| http | 8 | `lua_http_api.go:21` |
| world | 7 | `lua_world.go:9`、`youyuan_sense_api.go:54-57` |
| storage | 7 | `youyuan_storage_cache.go:15` |
| system | 7 | `lua_system.go:13` |
| diagnostics | 7 | `youyuan_entity_observations.go:43-47`、`youyuan_visibility.go:14` |
| packet | 6 | `youyuan_packet_api.go:323`、`youyuan_sense_api.go:33` |
| web | 5 | `web_api.go:17` |
| accounts | 5 | `youyuan_account_bridge.go:39` |
| mcp | 5 | `youyuan_plugin_mcp.go:13-17` |
| entities | 4 | `youyuan_entity_observations.go:37-40` |
| api | 4 | `lua_api_cross.go:48-80` |
| hud | 3 | `youyuan_hud_scoreboard.go:36-38` |
| scoreboard | 3 | `youyuan_hud_scoreboard.go:39-41` |
| media | 3 | `lua_media.go:16` |
| entity | 2 | `youyuan_entity_observations.go:38,51` |

> 参数列由源码 `L.CheckString/CheckNumber/CheckTable(...)` 序号顺序静态提取，`?` 为可选；表驱动方法的真实参数以对应表的说明字符串为准。

### 5.2 `game` 命名空间（77）

直接注册（`registerGameAPI`，`lua_api.go:74` 起）：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `game.sendCommand` | cmd | 发一条指令不等待（无实例选项时走快速路径） | `lua_api.go:81`、`youyuan_plugin_commands.go:295-308` |
| `game.sendTargeted` | player, text | 定向文本给玩家 | `lua_api.go:96` |
| `game.registerTrigger` | words表, handler(函数名或闭包) | 注册聊天触发词 | `lua_api.go:108` |
| `game.getScore` | objective, player | 读计分板 | `lua_api.go:142` |
| `game.getMultiScore` | 表 | 批量读分 | `lua_api.go:154` |
| `game.getPlayerPos` | player | 位置表（x/y/z/dimension） | `lua_api.go:164-177` |
| `game.getPlayerPosXYZ` | player, ? | 位置三元组 | `lua_api.go:179` |
| `game.getPlayerItem` | player, ?, slot | 槽位物品 | `lua_api.go:191` |
| `game.getBlockTile` | x,y,z | 方块实体 NBT | `lua_api.go:205` |
| `game.getBlockTileRaw` | x,y,z | 未解析方块实体 | `lua_api.go:371` |
| `game.getTarget` | player | 瞄准目标 | `lua_api.go:216` |
| `game.waitMsg` | player, 模式, 选项表 | 等待玩家消息 | `lua_api.go:230` |
| `game.showMenu` | player, 标题, 选项表 | 聊天菜单 | `lua_api.go:242` |
| `game.promptChoice` | player, 提示, 选项表 | 让玩家选择（序号从 1 起） | `lua_api.go:252` |
| `game.getConfig` | key, 默认值 | 读插件配置 | `lua_api.go:274` |
| `game.setConfigDefault` | 表 | 写配置默认值 | `lua_api.go:293` |
| `game.dataDir` | — | 旧数据目录（兼容保留） | `lua_api.go:313` |
| `game.httpGet` / `game.httpPost` | url, ? | 旧 HTTP 封装 | `lua_api.go:325`、`:335` |
| `game.jsonEncode` / `game.jsonDecode` | 值 / 字符串 | JSON 编解码 | `lua_api.go:350`、`:359` |
| `game.getTickingAreaList` | — | ticking area 列表 | `lua_api.go:382` |
| `game.queryInventory` | player | 查背包（需 OP / 网易扩展） | `lua_api.go:402` |
| `game.isCmdSuccess` | ?, ?, ? | 判定命令结果 | `lua_api.go:423` |
| `game.sendTitle` | player, 标题, 副标题 | 标题 | `lua_api.go:434` |
| `game.sendActionbar` | player, 文本 | 物品栏上方文本 | `lua_api.go:441` |
| `game.sendRichTellraw` | player, json | 富文本 | `lua_api.go:448` |
| `game.sendChat` | player, 文本 | 以玩家身份说话 | `lua_api.go:455` |
| `game.sendLines` | player, 多行 | 多行输出 | `lua_api.go:464` |
| `game.setEffect` | player, 效果, 时长, 等级 | 药水效果 | `lua_api.go:483` |
| `game.setScore` | objective, player, 值 | 写分 | `lua_api.go:494` |
| `game.teleport` / `move` / `rotate` / `jump` | 坐标或角度 | 机器人动作 | `lua_api.go:505,516,527,542` |
| `game.sneakStart`/`sneakStop`、`sprintStart`/`sprintStop`、`flyStart`/`flyStop`、`stopMove` | — | 持续动作开关 | `lua_api.go:548-590` |
| `game.getBotPos` | ? | 机器人位置 | `lua_api.go:591` |
| `game.getBlockData` / `getBlockNBT` / `getBlockState` | x,y,z | 方块数据 / NBT / 状态 | `lua_api.go:604,626,644` |
| `game.getRegionBlockData` | x1,y1,z1,x2,y2,z2 | 区域方块 | `lua_api.go:663` |
| `game.setBlock` / `placeNBTBlock` / `setBlockState` | x,y,z,块或表 | 放方块 | `lua_api.go:691,701,724` |
| `game.sendCommandWithResp` | cmd | 旧版带响应发送 | `lua_game_legacy.go:22` |
| `game.getOnlinePlayers` | — | 在线玩家（旧接口） | `lua_game_legacy.go:52` |
| `game.importBlocks` / `game.importWait` | — | 旧建筑导入 | `lua_game_legacy.go:61,178` |
| `game.getTPS`、`getServerTPS`、`serverTPS`、`getTickRate`、`getServerTickRate` | — | 同一实现，无观测返回 nil | `lua_api.go:21-32` |

命令执行族（`registerPluginCommandAPI`，`youyuan_plugin_commands.go:271-340`；这 9 个方法共享同一闭包并按实例默认选项执行）：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `game.getCommandOptions` / `game.setCommandOptions` | 表 | 读/改本实例命令默认参数 | `youyuan_plugin_commands.go:281-291` |
| `game.execute` | cmd, ?options | 等待真实 CommandOutput，超时为未知 | `youyuan_plugin_commands.go:292`、`youyuan_command_async.go:28` |
| `game.executeBatch` | 命令数组, options | 整批执行（ordered/pipeline/submit/burst/stream） | `youyuan_plugin_commands.go:292`、`youyuan_sense_api.go:45` |
| `game.sendCommandFast` | cmd, ?options | 不等待提交 | `youyuan_plugin_commands.go:292`、`youyuan_sense_api.go:40` |
| `game.sendCommandsFast` | 数组, ?options | 一次宿主边界提交整组 | `youyuan_sense_api.go:41` |
| `game.sendCommandConfirmed` | cmd, ?options | `game.execute` 别名 | `youyuan_sense_api.go:42` |
| `game.sendCommandAsync` | cmd, ?options | 异步提交 + 票据查询 | `youyuan_command_async.go:28` |
| `game.commandResult` | 票据 | 查异步结果 | `youyuan_command_async.go:29` |
| `game.getBlockTileAsync` | x,y,z | 异步读方块实体 | `youyuan_command_async.go:30` |
| `game.sendPacket` | 包名/ID, ?字段 | 发原始协议包 | `youyuan_packet_api.go:369`、`youyuan_sense_api.go:39` |
| `game.scene` / `game.sceneStats` | ?选项 | 实体快照 / 统计 | `youyuan_sense_api.go:43-44` |

另有一个 **Lua 专属语法糖**：`CommandBuffer()` 用 `L.DoString` 在 Lua 侧定义（`youyuan_plugin_commands.go:273-278`），`add/clear/size/submitFast` 最后一次性调 `game.sendCommandsFast`。Python/C++ 各有等价物：`host.command_buffer()`（`android/app/src/main/python/youyuan_plugins.py:36-45,74`）、`yy::CommandBuffer`（`sdk/cpp/youyuan.hpp:348-353`）。

### 5.3 `player` / `world` / `bot` / `building` / `media`

`player`（`lua_player.go:29` + 感知表）：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `player.get` | name | 玩家资料 | `lua_player.go:34` |
| `player.getXUID` | name | XUID | `lua_player.go:39` |
| `player.getUniqueID` | name | uniqueID | `lua_player.go:48` |
| `player.isOP` | name | 是否 OP | `lua_player.go:57` |
| `player.list` / `player.count` | — | 在线列表 / 数量 | `lua_player.go:63,71` |
| `player.getInventory` | name | 背包（别名 `game.queryInventory`） | `lua_player.go:76` |
| `player.health` | ?target(self/名字/runtime_id) | 已同步血量，未知为 null | `youyuan_sense_api.go:46` |
| `player.position` | ?target | 已同步位置 | `youyuan_sense_api.go:47` |
| `player.nearby` | ?radius(0..512，默认32) | 同维度玩家按距离排序 | `youyuan_sense_api.go:48` |
| `player.inventory` / `player.equipment` / `player.rotation` | ?target | 背包 / 装备 / 朝向观测 | `youyuan_entity_observations.go:41-42` 等 |
| `player.experience` / `player.changeExperience` | — / options | 经验读取与修改 | `youyuan_sense_api.go:338-356` |

`world`（`lua_world.go:9` + 感知表）：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `world.block` | x,y,z | 别名 `game.getBlockTile` | `lua_world.go:14` |
| `world.blockState` | x,y,z | 最近 UpdateBlock 观测，未知 known=false | `youyuan_sense_api.go:56` |
| `world.containerState` | x,y,z | 容器类型观测（不含物品） | `youyuan_sense_api.go:57` |
| `world.breakBlock` | x,y,z,?mode(command/creative_action) | 破坏方块 | `youyuan_sense_api.go:54` |
| `world.placeBlock` | x,y,z,block | setblock 放置 | `youyuan_sense_api.go:55` |
| `world.mineBlock` | x,y,z,duration_ms,slot,face | 生存挖掘动作流 | `youyuan_chain_api.go:15` |
| `world.useBlock` | x,y,z,face,slot | 手持物品点击方块 | `youyuan_chain_api.go:16` |

`bot`（`lua_bot.go:8` + 感知表）：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `bot.select` / `current` / `count` / `list` / `status` | name / — | 舰队机器人选择与状态 | `lua_bot.go:13,22,38,44,62` |
| `bot.self` | — | 本账号机器人观测状态 | `youyuan_sense_api.go:49` |
| `bot.startMovement` / `stopMovement` | dx,dy,dz / — | 玩家输入方向移动（非寻路） | `youyuan_sense_api.go:50-51` |
| `bot.lookAt` | x,y,z | 看向世界坐标 | `youyuan_sense_api.go:52` |
| `bot.teleport` | x,y,z | `tp @s` 并等结果 | `youyuan_sense_api.go:53` |
| `bot.attack` | runtime_id, slot | 六格内近战 | `youyuan_chain_api.go:14` |
| `bot.swingAir` | — | 空挥 | `youyuan_sense_api.go:405` |
| `bot.requestViewDistance` | 距离 | 请求视距 | `youyuan_visibility.go:15` |

`building` / `media`：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `building.detect` | 内容 | 结构格式检测 | `lua_building.go:16` |
| `building.parse` | 内容 | 解析结构 | `lua_building.go:23` |
| `building.isAir`/`isWater`/`isLava`/`isCommandBlock` | 块名, ?数据 | 方块判定 | `lua_building.go:54-66` |
| `building.closestColor` | r,g,b | 最近方块颜色 | `lua_building.go:72` |
| `building.importLimits`/`taskList`/`startImport`/`resumeImport`/`stopImport` | — | 建筑导入任务 | `youyuan_building_jobs.go:13-17` |
| `building.nbtCapabilities`/`normalizeNBT`/`containerData`/`placeContainer`/`placeSkull`/`nbtJob`/`planRegion`/`exportRegion` | — | NBT 与区域操作 | `youyuan_nbt_operations.go:18-25` |
| `media.imageToAscii` | 路径 | 图片转字符画 | `lua_media.go:21` |
| `media.noteToPitch` | note | 音符转音高 | `lua_media.go:46` |
| `media.parseMidi` | 路径 | MIDI 解析 | `lua_media.go:52` |

### 5.4 `events`（47）与 `packet`（6）

`events` 有两套注册机制，**同一命名空间下混用**：

1. **老式事件列表**（`lua_events.go:124-152`，仅 6 个）：`onChat`(127)、`onJoin`(131)、`onLeave`(135)、`onBotReady`(139)、`onPacket`(143)、`subscribe`(150)。它们把 handler 注册进 `evChat/evJoin/evLeave/evBotReady/evPacket` 全局切片（`lua_events.go:157-172`），触发点 `fireChatEvent`/`fireJoinEvent`/`fireLeaveEvent`/`fireBotReadyEvent`（`lua_events.go:175-199`）。
2. **感知钩子表**（`yySenseEvents`，`youyuan_sense_api.go:73`）：`onDeath`、`onPlayerDeath`、`onHealthChange`、`onHurt`、`onMove`、`onRespawn`、`onBlockChange`、`onSwing`、`onEntitySpawn`、`onEntityRemove`、`onItemPickup`、`onContainerChange`。实现见 `youyuan_sense_api.go:216-235`。

其余事件（按表）：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `events.subscribe` / `events.on` | 包类型, handler | 订阅指定协议包 | `lua_events.go:150`、`youyuan_sense_api.go:173-192` |
| `events.subscribeAll` | handler | 订阅全部包（envelope） | `youyuan_packet_api.go:342-368` |
| `events.subscribeFiltered` | 类型, filter表, handler | 带字段过滤与 `every_ms` 抽样 | `youyuan_sense_api.go:66,236-283` |
| `events.onPacket` | handler | 老式包回调 | `lua_events.go:143` |
| `events.packetSchema` / `events.catalog` | — | 当前内核包表 / 观测目录 | `youyuan_sense_api.go:71`、`youyuan_observed_events.go:17` |
| `events.define` / `events.on` / `events.onObservedAction` | 定义表, handler | 自定义观测事件 | `youyuan_observed_events.go:15-18` |
| `events.captureStart` / `captureStop` / `captureRead` / `captureClear` / `captureSummary` | config / — | 账号级解码抓包 | `youyuan_sense_api.go:67-70` |
| `events.onEntityHurt` / `onHealthChanged` / `onPlayerKill` / `onPlayerRespawn` / `onDimensionChange` / `onSessionJoin` / `onSessionLeave` / `onEquipmentChanged` / `onRotation` | handler | 实体观测事件族 | `youyuan_entity_observations.go:50-59` |
| `events.onEntityKill` / `onDamageDisplay` / `moduleHistory` | handler / — | 战斗模块事件 | `youyuan_module_combat.go:18-20` |
| `events.onServerObservation` / `onGlobalSwing` / `onGlobalHealthChange` | handler | 服务器级观测 | `youyuan_server_observation.go:73-75` |

`packet`（`youyuan_packet_api.go:323-380`）：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `packet.send` | 包名/ID, ?字段 | 同 `game.sendPacket` | `youyuan_packet_api.go:371` |
| `packet.on` | 类型, handler | 同 `events.subscribe` | `youyuan_packet_api.go:372` |
| `packet.onAll` | handler | 同 `events.subscribeAll` | `youyuan_packet_api.go:373` |
| `packet.schema` | — | 包字段表 | `youyuan_packet_api.go:374` |
| `packet.describe` | 包名或ID | 单包字段描述 | `youyuan_packet_api.go:375` |
| `packet.catalog` | — | 全部注册包概览 | `youyuan_observation_summary.go:21` |

### 5.5 `plugin`（22）/ `api`（4）/ `util`（26）

`plugin` 分三处注册，**同一个表**：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `plugin.id` / `plugin.dependencies` | — | 自身 id / 前置依赖表 | `lua_api_cross.go:72,76` |
| `plugin.codeDir` / `dataDir` / `configDir` / `resourceDir` / `tempDir` | — | 五类目录（按需创建） | `youyuan_resource.go:14-18`、实现 `:48-58` |
| `plugin.readResource` | 相对路径, ?encoding(utf8/base64) | 读 `resources/`，拒绝越界与符号链接 | `youyuan_resource.go:19`、`:105-121` |
| `plugin.listResources` / `existsResource` | 相对路径 | 列目录 / 判存在 | `youyuan_resource.go:20-21`、`:78-104` |
| `plugin.extractResource` | 源, ?目标 | 原字节复制到数据目录（**不是解压 ZIP**） | `youyuan_resource.go:22` |
| `plugin.listOutputs` / `exportFile` | 相对路径 | 生成文件列表 / 下载信息 | `youyuan_resource.go:23-24`、`:59-68` |
| `plugin.stats` | — | 运行时统计（需插件上下文） | `youyuan_profiler.go:26` |
| `plugin.has` / `plugin.capabilities` | 名称 / — | 能力探测 | `youyuan_capabilities.go:71-72` |

`api`：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `api.require` | 插件id | 返回懒代理表，任意 key 变成跨插件调用 | `lua_api_cross.go:51-68`、stub `:83-130` |
| `api.call` | 插件id, 函数, 参数数组 | SDK 侧跨插件调用入口 | `android/app/src/main/python/youyuan_plugins.py:34`、`youyuan_sense_api.go:488-526` |
| `api.id` / `api.dependencies` | — | Python/C++ 侧别名 | `youyuan_plugins.py:71`、`sdk/cpp/youyuan.hpp:342` |

`util`（`lua_util.go:121-328` + 调度表 `youyuan_scheduler.go:12-18`）：

| 方法 | 参数 | 作用 | 位置 |
|---|---|---|---|
| `util.now` / `timestamp` / `time_fmt` | ?格式 | 时间 | `lua_util.go:126,130,135` |
| `util.json_load` / `json_write` | 路径, ?值 | JSON 文件读写 | `lua_util.go:143,159` |
| `util.read_file` / `write_file` | 路径, ?内容 | 文本读写 | `lua_util.go:176,186` |
| `util.strip_color` / `split` / `join` / `trim` | 文本等 | 字符串工具 | `lua_util.go:201,219,231,239` |
| `util.floor` / `ceil` / `round` / `pos` | 数字 | 数值工具 | `lua_util.go:245-260` |
| `util.schedule` / `setTimeout` / `cancel` | 秒, 回调 | 定时任务 | `lua_util.go:278,296,313` |
| `util.scheduleHz` / `scheduleSeconds` / `scheduleTick` | 频率/秒/tick, 回调 | 统一宿主时钟调度 | `youyuan_scheduler.go:13-15` |
| `util.listJobs` / `pauseJob` / `resumeJob` | id | 任务管理 | `youyuan_scheduler.go:16-18` |
| `util.monotonicMs` / `serverTick` | — | 单调时钟 / 服务器 tick | `youyuan_entity_observations.go:48-49` |

---

## 6. 命令链：不是插件，但总被当插件问

命令链是**内核内建能力**（`youyuan_commands.go`），不是插件：没有 manifest、不经过 `PluginController`、也不经过 `yyRuntime`。
插件只能通过 `chains.*` API 去操作它（`lua_api.go` 的 chains 命名空间）。

### 6.1 TXT 放哪、叫什么

| 项 | 事实 | 源码 |
|---|---|---|
| 根目录 | `<dataDir>/插件数据文件/ds_ai_agent/command_txt/`，进程启动时 `MkdirAll 0755` | `youyuan_commands.go:722-723` |
| 扩展名 | 只认 `.txt` 与 `.mcfunction`（大小写不敏感） | `youyuan_commands.go:341-342` |
| 命名约束 | 必须是**直接文件名**：`filepath.Base(name) == name`、不含 `/ \ :`、不以 `.` 开头 | `youyuan_commands.go:337-339` |
| 配置文件 | 同名 `<name>.json`（与 txt 同目录），存 `yyChainConfig` | `youyuan_commands.go:346-349` |
| 数据归属 | 每个账号的内核有独立 dataDir；同一内核下多机器人按 `config.account` 过滤，互相控制不到 | `youyuan_commands.go:737`（默认 `Account:"main"`） |

### 6.2 文件格式（mcfunction 兼容 + `#` 指令）

解析器 `yyParseChains`（`youyuan_commands.go:124`）：

| 语法 | 含义 | 源码 |
|---|---|---|
| 普通行 | 一条命令；**行首 `/` 会被去掉**再下发 | `:147-152`（`TrimPrefix(command, "/")` 在 `:152`） |
| `类型,条件,红石[,Hz]` | 节点头：类型 0/1/2、条件 0/1、红石 0/1，可选第 4 段为频率 | `:122`（正则）、`:262-267`（赋值） |
| `# 组：<参数>` / `# 群：` / `# 分区：` / `# 类别：` | 分组头，可带前缀/频道/速度/循环次数（循环 >1000 直接报错） | `:283`（正则）、`:169-182` |
| `# 定义：<名>` / `# 测试：` / `# 规定：` | 定义一段可复用的测试命令 | `:284` |
| `# 执行：<名> <参数>` | 展开上面定义的测试；找不到就报「找不到测试 X」 | `:285`（正则）、`:183-205` |
| 其它 `#` 行 | 注释，跳过 | `:168-182` |
| BOM / CRLF | 行首 `\ufeff` 被剥掉，`\r\n` 归一成 `\n` | `:163-164` |
| 空文件 | 报错「文件没有命令」 | `:277` |

注意：**这不是命令方块 NBT 的完整语义**，不翻译方块朝向、`clone` 接线等；它只负责按序下发命令。

### 6.3 怎么启动 / 怎么重载（最容易踩的一节）

`POST /api/youyuan/chains`，body `{action, name, config, content, redstone}`（`youyuan_commands.go:859-866`，body 上限 16MB）。
`action` 只有 8 个值，**没有 `scan`**：`preview` / `save` / `delete` / `start` / `stop` / `trigger` / `redstone` / `reload`（`:912,924,987,1019`）。

启动时机决定了「为什么我放了 txt 却没跑」：

| 场景 | 实际行为 | 源码 |
|---|---|---|
| 进程启动 | `yyCommandRoutes` 扫一次目录，**只把 `config.enabled == true` 的链自动拉起**，其余只建实例（状态「已停止」） | `youyuan_commands.go:724-745` |
| 启动后新放进来的 txt | **不会被自动发现**。目录只在启动时扫一次，没有运行态实例 | `youyuan_commands.go:763-765` 源码注释原文：「旧实现只遍历 yyChains.items，而目录只在进程启动时扫一次；启动后上传/拷贝进来的 TXT 永远不会出现在列表里」 |
| 但列表里能看到它 | `GET /api/youyuan/chains` 已改成「磁盘目录 ∪ 内存运行态」，新文件以 `state:"未启动"`、`loaded:false` 出现 | `youyuan_commands.go:763-870`（`diskRow` 在 `:789`） |
| 让它跑起来 | 必须显式 `{"action":"start","name":"x.txt"}`；`start` 分支里有 `loadFromDisk()`，现场读文件 + 解析 + 建实例 | `youyuan_commands.go:1020-1025`、`:885-913` |
| 已装载的链被改文件 | `yyStartChainWatcher` 每秒 `Stat` 一次（mtime + size），变了就重解析；解析失败**保留原 IR** 并在 `last_error` 里报 | `youyuan_chain_watch.go:44-76`、`:23-42` |
| 想立刻生效 | `{"action":"reload"}` 直接调 `yyReloadChainResource` | `youyuan_commands.go:1046-1047` |

**关键边界**：watcher 只遍历 `yyChains.items`（已加载实例，`youyuan_chain_watch.go:46-51`），
所以「新 txt 自动热加载」是不成立的——**新增必须 start，改动才会自动重载**。

`start` 成功后立刻把 `Enabled=true` 写进 `<name>.json`（`youyuan_commands.go:419`），
所以下次进程启动会被自动拉起；状态先置「等待连接」（`:410`），等机器人连上才真正发命令（`run` 在 `:428`）。

### 6.4 状态字段怎么读

`GET /api/youyuan/chains` 每行都带一句 `counter_semantics` 原文（`youyuan_commands.go:774`，磁盘行在 `:811`）：

> `executed=attempted nodes; submitted=host accepted; cycles=file rounds or sum of independent segment rounds, not proof of server execution`

即：`executed` 是「尝试下发的节点数」，`submitted` 是「宿主接受了」，`cycles` 是「文件轮次」。
**三者都不等于服务器真的执行了**。`results_confirmed` 在 `mode=="fast"` 时为 `false`（`:774`），
因为快速模式不等回执。

---

## 7. 文档与源码不一致处

逐项核对根目录官方文档（`Lua插件开发说明.md` / `Python插件开发说明.md` / `C++插件开发说明.md` / `命令链插件API.md` / `插件生成文件下载说明.md`）
与 `upstream/*.go` 实现，**发现 6 处需要按源码为准的地方**。

### 7.1 命令链「自动同步」的边界被读宽了

文档原话（`命令链插件API.md:200`）：「加载/修改时预解析成常驻IR；一秒检查资源mtime，未变化不读文件。显式reload立即检查。」

源码事实：这一秒一次的检查只覆盖**已经有运行态实例**的链（`youyuan_chain_watch.go:46-51` 遍历 `yyChains.items`）。
启动后才放进目录的 txt 没有实例，watcher 永远不会看到它，**必须 `action:"start"`**（`youyuan_commands.go:1020-1025`）。

→ 文档读起来像「目录级自动同步」，实际是「已加载链的文件级热重载」。

### 7.2 `manifest.json` 里的 `id` 在扫描时被忽略

文档示例（`Lua插件开发说明.md:10`）把 `id` 当唯一标识写进清单。
源码：真实 id 由**文件名**推导（`plugin_scan.go:63-68`，去掉扩展名、再去掉「词库」后缀），
`pluginManifest.ID`（`plugin_scan.go:13`）只参与 ZIP 导入时的安装目录与代码文件名（`plugin_pack.go:127-136,141`）。

→ 手工把 `插件文件/foo.lua` 的清单 id 改成 `bar`，插件在列表里**仍然叫 foo**。

### 7.3 `mcp` 命名空间：静态清单是 24 个，运行时是 25 个

`yySDKNames` 的字面量只有 24 项（`youyuan_sdk.go:20`，到 `observerBridge` 为止）。
`mcp` 是 `init()` 里追加进去的：`yySDKNames = append(yySDKNames, "mcp")`（`youyuan_plugin_mcp.go:21`），
方法文档也在同一个 `init()` 注册进 `yyExtraMethods`（`youyuan_plugin_mcp.go:11-20`）。

→ 读 `youyuan_sdk.go:20` 会误判「没有 mcp」；三语言宿主都注入了 25 个（`youyuan_plugins.py:71,191`、`sdk/cpp/youyuan.hpp:327,329`），
但 Go 侧 `未知命名空间` 的判定（`youyuan_sdk.go:637,654`）是在 `init()` 之后跑的，所以 `mcp.*` 实际可用。
本节 §3 的命名空间枚举已按此修正。

### 7.4 裸文件布局：文档只讲目录，源码也认单文件

文档只描述 `插件文件/<id>/<id>.lua` 这种目录布局。源码额外允许**裸文件** `插件文件/<id>.lua`（历史布局），
但裸文件**不能删除、不能导出**（`plugin_paths.go:32-70`、`:74-84`）。

→ 用裸文件放进去能跑，用户在 UI 里点删除/导出会失败，容易当成 bug。

### 7.5 数据目录的账号隔离：Lua 不隔离，Python/C++ 隔离

- Lua：`DataDir(id)` = `插件数据文件/<id>/`，**与账号无关**（`plugin_config.go:15-19`）。
- Python/C++：`data/<account>/`，按账号分开（`youyuan_runtime.go:131`）。

三份语言文档都在讲「插件数据目录」，但只有 Python 文档提到多账号时行为不同；
Lua 文档没有明说「多开账号时两个机器人共用同一份数据」。这会在多开场景下直接串数据。

### 7.6 `author` 字段：解析了，然后丢掉

`pluginManifest` 有 `Author` 字段（`plugin_scan.go:18`），但：

- `PluginDef` 结构体里**没有** Author 字段（`plugin_defs.go:4-15`，只有 ID/Name/Version/Type/Description/Docs/PrePlugins/SourceDir/SourceFile/Load）；
- 全仓库 `grep "\.Author"` 在非测试代码里**零命中**——读出来就没再用过。

→ 清单里写 `"author": "xxx"` 不会有任何效果，插件列表/详情里也拿不到作者名。

### 7.7 核对后确认一致的部分（避免只报问题）

| 文档说法 | 源码位置 | 结论 |
|---|---|---|
| 命令链 txt 目录 `插件数据文件/ds_ai_agent/command_txt/` | `youyuan_commands.go:722` | 一致 |
| 生成文件下载三个 GET：`/api/youyuan/plugin-outputs`、`/info`、`/download`（`插件生成文件下载说明.md:34-36`） | `youyuan_plugin_outputs.go:124-127` 的 `for _, suffix := range []string{"", "/info", "/download"}` | 一致 |
| `plugin.exportFile` / `plugin.listOutputs` / `plugin.dataDir` 真实存在 | `youyuan_resource.go:23-24,59-64`、`lua_api.go:319` | 一致 |
| Python `on_init(host)` 可为 async | `android/app/src/main/python/youyuan_plugins.py:195`（`host._resolve(module.on_init(host))`） | 一致 |
| C++ 入口优先 `yy_create_v2`、回退 `yy_create` | `sdk/native_worker.py:66-72`、`sdk/cpp/youyuan.hpp:379` | 一致 |
| Lua 单次调用上限 2 分钟 | `lua_botctx.go:14`（`luaExecTimeout`） | 一致 |

> 引用路径提醒：`youyuan_plugins.py` 不在 `upstream/` 根，真实路径是
> `upstream/android/app/src/main/python/youyuan_plugins.py`（`build/` 下是同一份的构建副本）；
> `sdk/*.py` 与 `sdk/cpp/youyuan.hpp` 在 `upstream/sdk/`。本文其余小节沿用简写，查证时按此基准补全。
