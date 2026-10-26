# 建筑导入导出的执行位置与进度

## 三个执行位置

| 操作 | 文件处理位置 | 游戏操作位置 | 文件最终位置 |
|---|---|---|---|
| 建筑导入 | 工具箱宿主读取结构、转换方块与NBT | 当前账号连接的游戏服务器 | 输入文件仍在宿主设备；建筑在游戏里 |
| 建筑导出 | 宿主合并收到的结构、编码mcstructure/schematic/mcworld | 从当前连接的游戏服务器请求结构 | 工具箱宿主的选定输出目录 |
| 插件ZIP导入/导出 | 工具箱宿主解压安装/打包目录 | 导入文件本身不会操作游戏 | 宿主插件目录/ZIP输出目录 |
| 市场下载 | 从市场接口下载文件到宿主 | 下载不会放置方块或执行插件 | 宿主下载目录，之后另行导入/安装 |

宿主就是实际运行机器人的设备：APK通常是手机或模拟器；Linux版是运行工具箱进程的服务器。浏览器通过公网连接某个APK/Linux时，远程任务的输出仍写在那个宿主上，浏览器可以再下载到自己设备。Cloudflare控制页面与中继不是建筑解析或游戏执行器。文件选择器上传的文件也先进入选定的工具箱宿主。

建筑导出走服务器的结构请求，不会把建筑发送给原作者网站转换；建筑导入走机器人协议/命令接口。市场下载保留用户指定的原市场来源，与导入执行是独立流程。游戏认证服务负责登录鉴权，不负责建筑转换。

## 游园任务提示

建筑写入使用青蓝色，建筑读取使用琥珀色，区域准备使用暖色；地图画和雕像各有任务标签。进度条按实际计数填充，不显示旧版多层百分比、Importer标题或旋转装饰。

- 写入计数表示宿主已提交的方块操作，不能只据此断言每块都已被服务器确认。
- 导出计数表示真实收到的区块。失败区块不会算成成功；失败中止且不生成缺块成功文件；分区坐标显式合并。
- 任务读取阶段、文件编码与保存阶段分开显示；下载文件到浏览器是最后独立一步。
- 自动化的数值progress字段仍保留，变更的是面向用户的提示，不伪造进度或吞吐。
- TPS只接受原生宿主实际提供的服务器测量值。当前宿主没有该数据接口，`game.getTPS/getServerTPS/serverTPS/getTickRate/getServerTickRate`在Lua返回`nil`，公共JSON调用和状态快照返回`null`，界面显示“暂不可用”。不要用世界时间、实体Tick、命令往返耗时或本机时钟补造TPS；TPS缺失不影响按所选速度导入。

MCP调用使用真实的`api_task_start`、`api_task_list`等工具，对应`POST /api/task/start`与`GET /api/task/list`。任务参数与页面一致；`building.parse`等插件解析函数只解析文件，不会自动放置建筑。`building.exportRegion` 已在 0.10.24 注册，详见容器NBT与分区协作技能；可用的宿主任务入口为 `building.startImport/taskList/resumeImport/stopImport`；不存在 `building.importFile` 别名，不要编造。执行前确认账号、连接的服务器、输入路径、输出路径与区域坐标。对未知血量、未确认方块放置等信息不要显示成功。

## 0.10.63 宿主作业、断点与机器人选择

以下名称已经注册到宿主公共API目录，Lua、Python、C++ SDK和MCP的game_call共用实现；Lua/Python使用 `building.taskList()` 等对应命名空间函数；C++使用 `client.building.taskList()` 等方法。MCP使用game_call的method/args，或直接使用对应HTTP工具。`api.call` 是跨插件调用，不是这里的调用入口。

- `building.taskList()`：列出当前账号、当前服务器的作业和原参数。
- `building.startImport({path,x,y,z,speed,dimension?,rotation?,multi?,bot_names?})`：提交宿主导入。`path`是宿主上的真实文件，Linux路径不是浏览器电脑路径。`multi=true`开启分区协作，`bot_names`为已经连接在同服的机器人游戏名称数组；不会自动登录或移动未选账号。
- `building.resumeImport(task_id)`：继续原设置的断点。多机器人恢复保存原始切分机器人数量和布局，校验原文件SHA256，已结束单元跳过；中断单元重做。旧版本没有分区断点的任务不能假装恢复。
- `building.stopImport()`：请求停止并保存断点；等待旧执行器清理期间，新作业会收到“停止清理中”。

调用返回 `submitted=true,completed=false` 仅表示提交，后续用taskList读取状态；导入进度是已执行/提交计数，不能代替服务器逐块读回。NBT是否成功应查看nbtJob的verified或另行读回目标结构。

HTTP与MCP接口：`GET /api/task/list`、`POST /api/task/start`、`POST /api/task/resume {task_id}`、`POST /api/task/stop`。`GET /api/fleet/status`新增 `workers`（name/server/is_op/main），只列当前内核已就绪连接。独立账号内核通过accountsPool.invoke调用各自任务，不要把一个账号的内存对象交给另一个账号。

建筑缓存包含文件修改时间、大小和MCWorld选区；导入前接管该缓存，防止旋转重复累加。多机器人只对实际参与者持有空间作业租约，暂停它们的观测和插件传送；结束后释放。命令方块链条仍在机器人内部执行，此建筑作业接口不会改变命令链的每链Hz语义。
