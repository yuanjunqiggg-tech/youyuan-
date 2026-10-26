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


## 0.10.42 导出下载中心与可配置镜头

Linux/网页插件列表→分享→导出：生成完整私有ZIP后直接下载，并保存到“导出下载中心”。中心列出文件名、日期、大小、SHA256，可重复下载、复制中心网址或只删除备份。下载中心网址为 `https://toolbox.youyuanqi.dpdns.org/?device=linux-server&exports=1`；需登录并有该执行节点权限。子账号使用 `export_account=账号ID`，由账号池校验是否本人拥有；不能通过改URL控制别人的账号。备份存在执行设备的“插件导出/”，不是原作者服务器或公开市场。每次导出独立文件，不覆盖旧备份；Lua包含插件文件/配置/数据的完整目录，Python/C++包含整个插件目录和资源。

MCP：POST `/api/plugin/export` body `{id:"插件ID"}`；Python/C++用POST `/api/youyuan/runtime/export` query `{id:"插件ID"}`。响应有export_id、export、download_url，原path/src_path保留给APK系统选择器。GET `/api/youyuan/exports`列记录；GET `/api/youyuan/exports/info?id=导出ID`读元信息；GET `/api/youyuan/exports/download?id=导出ID`由登录后的网页下载ZIP；POST `/api/youyuan/exports/remove` body `{id:"导出ID"}`只删除该备份。MCP调用download返回经权限验证的元信息且downloaded=false，不把ZIP二进制塞进AI上下文。删除需要控制权限。备份包含自己的配置和私有数据，请按需私下保管。未导出的历史磁盘ZIP不会假装已有下载记录；重新点导出即可。

Lua/Python/C++共享watchStart新增配置：camera_height_blocks（脚部坐标上方高差，默认2），camera_pitch_degrees（留空保持原行为；-90向上、90向下），camera_yaw_degrees（默认0），camera_facing_player（默认false；每次free更新时facing玩家；与固定俯仰二选一）。例如camera模式高度8、俯仰90，从玩家上方俯视；只改镜头，不移动身体。高度/俯仰不承诺扩大服务器动作广播或减少漏包。原生target_entity仍只追踪朝向，当前网易服尚不支持已测的attach_to_entity。先接替再释放、错峰规划、账号池和模块通信保留；区域就绪不等于目标事件齐全。

本轮对照须分别统计：固定低镜头/高镜头俯视、多人分区常驻/单节点切换，以及跨维度。接收与发送账号自己的权限为0；有OP主控只下指令，其收包不算普通权限结果。明确身体坐标、维度、测量时间和交集重复；受控正常权限协议账号是有效工程样本，但没有自动包含手机UI/输入协议的全部差异。


## 0.10.43 生成文件交付

### `plugin.listOutputs`

relative_dir默认.；列出当前插件数据目录内生成的文件，返回path/name/bytes/modified_at/download_url，不返回文件内容。

详细参数与示例见 plugin-outputs.md。

### `plugin.exportFile`

relative_file；取得当前插件生成文件的下载信息，保持原格式/原字节，不是备份整个插件。文件须已写入plugin.dataDir。链接需登录当前账号，snapshot=false表示下载当前文件版本。

详细参数与示例见 plugin-outputs.md。



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
