# building · 建筑与颜色

结构格式检测、解析和方块颜色匹配。这些解析函数不等于放置整个建筑。


### `building.closestColor`

```python
host.building.closestColor(r, g, b)
```

- 作用：按 CIELAB 感知色差，匹配给定 RGB 最接近的颜色方块。
- 参数：`r`/`g`/`b` number 必填（0-255）。
- 返回：string（最接近的颜色方块名）。
- 报错：非数值抛类型错误。
- 底层：`media.ClosestBlockForColor(r, g, b)`。

### `building.detect`

```python
host.building.detect(path)
```

- 作用：检测建筑文件的格式。
- 参数：`path` string 必填。
- 返回：string（格式名：`.mcstructure` / `.schematic` / `.mcworld` / `.bdx`）。
- 报错：非字符串抛类型错误。
- 底层：`building.DetectFormat(path)`。

### `building.isAir`

```python
host.building.isAir(name)
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isCommandBlock`

```python
host.building.isCommandBlock(name)
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isLava`

```python
host.building.isLava(name)
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isWater`

```python
host.building.isWater(name)
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.parse`

```python
host.building.parse(path)
```

- 作用：加载并解析建筑结构文件，返回尺寸、方块统计、NBT、是否含命令方块。
- 参数：`path` string 必填。
- 返回：对象或数组 `{size={x,y,z}, blocks=number, nbt=number, has_command=boolean}`（`blocks` 为非空气方块数，`nbt` 为 NBT 方块数）。
- 报错：加载失败抛 `building.parse: <err>`。
- 底层：`building.LoadStructureFile(path)`，遍历方块计数，读 `NBTBlocks`/`HasCommand`。

## 0.10.24 容器NBT与分区协作

完整参数、NBT例子、副作用和20种制作行为见技能 `nbt-workbench`，可按需读取 `youyuan://docs/nbt-workbench`。

### `building.nbtCapabilities`

查询底层支持方块、复杂物品和20种行为，支持表不等于全部实测。

### `building.normalizeNBT`

NBT对象；恢复Count/Slot/ench/Damage整数宽度，保留未知字段，拒绝越界和null。

### `building.containerData`

name,Items,options?；本地预检物品类型和实际槽位，不操作游戏。

### `building.placeContainer`

x,y,z,name,Items,options?；异步制作带名称和附魔的容器；返回job_id，需nbtJob读回verified。首次工作台初始化清机器人背包、切创造、移动并建地板。

### `building.nbtJob`

job_id,consume=false；当前账号任务状态和结果。consume只删除已完成任务；重启不恢复。

### `building.planRegion`

起点,尺寸；16×128×16完整覆盖规划，不是游戏实际执行。

### `building.exportRegion`

起点,尺寸,{workers=1}?；异步保存本机mcstructure。只用已配置同服Fleet，实际workers_used；缺块失败不输出成功文件。


## 0.10.62 头颅类型与真实验证

`building.placeSkull(x,y,z,SkullType,states?)`：异步在指定位置制作并放置头颅，返回 `job_id`。类型 0 骷髅、1 凋灵骷髅、2 僵尸、3 玩家、4 苦力怕、5 末影龙、6 猪灵。第三位不是皮肤组件码；玩家头颅不等于任意自定义玩家皮肤。

调用 `building.nbtJob(job_id)` 直到任务完成；只有 `result.verified=true` 才表示目标结构读回一致。已提交不代表游戏完成。旧版 `minecraft:skull` 的 SkullType 和新版独立方块名都保留类型，避免格式升级时全部变成骷髅头。具体服务端版本不支持的头颅会报错。

此接口会初始化工作台、清机器人背包、移动机器人并使用创造物品。操作前暂停跟随/观测；不建议用于有贵重物品的真人账号。普通建筑导入和多机器人导入共用此处理器。附魔/命名容器仍用 `building.placeContainer`；MCStructure 保留类型化 NBT，TXT 指令导出不能保留容器物品 NBT。

## 0.10.63 宿主作业、断点与机器人选择

以下名称已经注册到宿主公共API目录，Lua、Python、C++ SDK和MCP的game_call共用实现；Lua/Python使用 `building.taskList()` 等对应命名空间函数；C++使用 `client.building.taskList()` 等方法。MCP使用game_call的method/args，或直接使用对应HTTP工具。`api.call` 是跨插件调用，不是这里的调用入口。

- `building.taskList()`：列出当前账号、当前服务器的作业和原参数。
- `building.startImport({path,x,y,z,speed,dimension?,rotation?,multi?,bot_names?})`：提交宿主导入。`path`是宿主上的真实文件，Linux路径不是浏览器电脑路径。`multi=true`开启分区协作，`bot_names`为已经连接在同服的机器人游戏名称数组；不会自动登录或移动未选账号。
- `building.resumeImport(task_id)`：继续原设置的断点。多机器人恢复保存原始切分机器人数量和布局，校验原文件SHA256，已结束单元跳过；中断单元重做。旧版本没有分区断点的任务不能假装恢复。
- `building.stopImport()`：请求停止并保存断点；等待旧执行器清理期间，新作业会收到“停止清理中”。

调用返回 `submitted=true,completed=false` 仅表示提交，后续用taskList读取状态；导入进度是已执行/提交计数，不能代替服务器逐块读回。NBT是否成功应查看nbtJob的verified或另行读回目标结构。

HTTP与MCP接口：`GET /api/task/list`、`POST /api/task/start`、`POST /api/task/resume {task_id}`、`POST /api/task/stop`。`GET /api/fleet/status`新增 `workers`（name/server/is_op/main），只列当前内核已就绪连接。独立账号内核通过accountsPool.invoke调用各自任务，不要把一个账号的内存对象交给另一个账号。

建筑缓存包含文件修改时间、大小和MCWorld选区；导入前接管该缓存，防止旋转重复累加。多机器人只对实际参与者持有空间作业租约，暂停它们的观测和插件传送；结束后释放。命令方块链条仍在机器人内部执行，此建筑作业接口不会改变命令链的每链Hz语义。
