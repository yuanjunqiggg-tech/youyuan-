# building · 建筑与颜色

结构格式检测、解析和方块颜色匹配。这些解析函数不等于放置整个建筑。


### `building.closestColor`

```cpp
client.building.closestColor(r, g, b);
```

- 作用：按 CIELAB 感知色差，匹配给定 RGB 最接近的颜色方块。
- 参数：`r`/`g`/`b` number 必填（0-255）。
- 返回：string（最接近的颜色方块名）。
- 报错：非数值抛类型错误。
- 底层：`media.ClosestBlockForColor(r, g, b)`。

### `building.detect`

```cpp
client.building.detect(path);
```

- 作用：检测建筑文件的格式。
- 参数：`path` string 必填。
- 返回：string（格式名：`.mcstructure` / `.schematic` / `.mcworld` / `.bdx`）。
- 报错：非字符串抛类型错误。
- 底层：`building.DetectFormat(path)`。

### `building.isAir`

```cpp
client.building.isAir(name);
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isCommandBlock`

```cpp
client.building.isCommandBlock(name);
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isLava`

```cpp
client.building.isLava(name);
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.isWater`

```cpp
client.building.isWater(name);
```

- 作用：判断方块名是否为空气 / 水 / 岩浆 / 命令方块。
- 参数：`name` string 必填（方块名）。
- 返回：boolean。
- 报错：无。
- 底层：`building.IsAirBlock/IsWaterBlock/IsLavaBlock/IsCommandBlock(name)`。

### `building.parse`

```cpp
client.building.parse(path);
```

- 作用：加载并解析建筑结构文件，返回尺寸、方块统计、NBT、是否含命令方块。
- 参数：`path` string 必填。
- 返回：对象或数组 `{size={x,y,z}, blocks=number, nbt=number, has_command=boolean}`（`blocks` 为非空气方块数，`nbt` 为 NBT 方块数）。
- 报错：加载失败抛 `building.parse: <err>`。
- 底层：`building.LoadStructureFile(path)`，遍历方块计数，读 `NBTBlocks`/`HasCommand`。
