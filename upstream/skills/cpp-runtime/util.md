# util · 文件、时间与调度

JSON文件、文本、数学、时间与整个回调的定时任务。


### `util.cancel`

```cpp
client.util.cancel(id);
```

- 作用：取消一个已注册的定时任务（schedule/setTimeout）。
- 参数：`id` number 必填。
- 返回：boolean——`true` 找到并取消，`false` 不存在。
- 报错：无。
- 底层：`cancelSchedJob(int64(id))`，关闭任务的 cancel channel。

### `util.ceil`

```cpp
client.util.ceil(n);
```

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（math.Round：正负0.5均远离0取整）。

### `util.floor`

```cpp
client.util.floor(n);
```

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（math.Round：正负0.5均远离0取整）。

### `util.join`

```cpp
client.util.join(tbl, sep);
```

- 作用：把 对象或数组 的所有值转字符串后用分隔符拼接。注意：遍历整表，非数组顺序不一定与下标一致。
- 参数：`tbl` 对象或数组、`sep` string 必填。
- 返回：string。
- 报错：无。
- 底层：`t.ForEach` + `LVAsString` + `strings.Join`。

### `util.json_load`

```cpp
client.util.json_load(path);
```

- 作用：读取文件并解析为 JSON 对象。
- 参数：`path` string 必填。
- 返回：任意 JSON兼容值（表/数字/字符串…）。数字用 `UseNumber()` 解码避免精度损失。
- 报错：读文件失败或 JSON 解析失败抛 `json_load: <err>`。
- 底层：`os.ReadFile` + `json.Decoder`（`UseNumber`）→ `goToLua`。

### `util.json_write`

```cpp
client.util.json_write(path, data);
```

- 作用：把任意 JSON兼容值序列化为带缩进的 JSON 写入文件，自动建父目录。
- 参数：`path` string 必填；`data` 任意类型必填。
- 返回：boolean `true`。
- 报错：建目录/序列化/写文件失败抛 `json_write: <err>`。
- 底层：`os.MkdirAll` + `json.MarshalIndent(data,"","  ")` + `os.WriteFile`。

### `util.listJobs`

```cpp
client.util.listJobs();
```

- 功能、参数、返回与约束：无参数，当前插件任务列表：id/handler/paused/calls/missed/failures/last_ms/max_ms/total_ms/last_error；耗时为墙钟含VM排队及回调，不是CPU时间。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `util.now`

```cpp
client.util.now();
```

- 作用：返回当前本地时间。
- 参数：无。
- 返回：string，格式固定 `"2006-01-02 15:04:05"`。
- 报错：无。
- 底层：`time.Now().Format("2006-01-02 15:04:05")`。

### `util.pauseJob`

```cpp
client.util.pauseJob(id);
```

- 功能、参数、返回与约束：id，暂停该插件任务后续触发；已经执行/入队的一次不强制中断。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `util.pos`

```cpp
client.util.pos(str);
```

- 作用：把空格分隔的坐标字符串（如 `"100 64 200"`）解析成坐标表，最多取前 3 个 token，缺失项补 0。
- 参数：`str` string 必填。
- 返回：对象或数组 `{x=number, y=number, z=number}`。
- 报错：无。
- 底层：`strings.Fields` + `strconv.ParseFloat`。

### `util.read_file`

```cpp
client.util.read_file(path);
```

- 作用：读取文件全部内容按字符串返回。
- 参数：`path` string 必填。
- 返回：string。
- 报错：读取失败抛 `read_file: <err>`。
- 底层：`os.ReadFile`。

### `util.resumeJob`

```cpp
client.util.resumeJob(id);
```

- 功能、参数、返回与约束：id，恢复后续触发；不会补发暂停期间的积压轮次。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `util.round`

```cpp
client.util.round(n);
```

- 作用：向下 / 向上 / 四舍五入取整。
- 参数：`n` number 必填。
- 返回：number。
- 报错：非数值抛类型错误。
- 底层：`math.Floor` / `math.Ceil` / `math.Round`（math.Round：正负0.5均远离0取整）。

### `util.schedule`

```cpp
client.util.schedule(intervalSec, handlerName);
```

- 作用：按秒调用整个回调，周期0.001秒至1年。
- 参数：`intervalSec, handlerName`；按此顺序传入。
- 返回：定时任务ID；util.cancel(id)取消。
- 底层与游戏效果：源码与模块缓存复用；同任务不重入，不补无限积压。回调里整条链的所有命令连续提交，不是每次只发一条。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.scheduleHz`

```cpp
client.util.scheduleHz(hz, handler);
```

- 功能、参数、返回与约束：hz,handler；本地完整回调的频率，0<hz<=1000。慢回调不重入、不补积压轮次；统一时钟、skip策略；查看listJobs的missed。不是服务端同步Tick。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.scheduleSeconds`

```cpp
client.util.scheduleSeconds(seconds, handler);
```

- 功能、参数、返回与约束：seconds,handler；schedule别名，0.001秒..一年。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.scheduleTick`

```cpp
client.util.scheduleTick(ticks, handler);
```

- 功能、参数、返回与约束：ticks,handler；ticks>=1，统一宿主Tick Loop；本地名义Tick=50ms，不宣称服务端同Tick同步。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.setTimeout`

```cpp
client.util.setTimeout(sec, handlerName);
```

- 作用：延时后调用一次回调，可设0秒。
- 参数：`sec, handlerName`；按此顺序传入。
- 返回：定时任务ID。
- 底层与游戏效果：只调用已加载的函数，停用或重载自动取消所属定时任务。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `util.split`

```cpp
client.util.split(str, sep);
```

- 作用：按分隔符切分字符串为数组。
- 参数：`str` string、`sep` string 必填。
- 返回：对象或数组（字符串数组，1 起索引）。
- 报错：无。
- 底层：`strings.Split` + `RawSetInt`。

### `util.strip_color`

```cpp
client.util.strip_color(text);
```

- 作用：去掉文本里的 Minecraft 颜色/格式码（`§` 与 `&` 及其后一个字符）。
- 参数：`text` string 必填。
- 返回：string。
- 报错：无。
- 底层：手写循环用 `strings.Builder` 逐字节过滤。

### `util.time_fmt`

```cpp
client.util.time_fmt(ts);
```

- 作用：把 Unix 时间戳格式化为可读的本地时间。
- 参数：`ts` number 可选，默认当前时间戳。
- 返回：string。
- 报错：无。
- 底层：`time.Unix(ts,0).Format("2006-01-02 15:04:05")`。

### `util.timestamp`

```cpp
client.util.timestamp();
```

- 作用：返回当前 Unix 秒级时间戳。
- 参数：无。
- 返回：number。
- 报错：无。
- 底层：`time.Now().Unix()`。

### `util.trim`

```cpp
client.util.trim(str);
```

- 作用：去除字符串两端空白。
- 参数：`str` string 必填。
- 返回：string。
- 报错：无。
- 底层：`strings.TrimSpace`。

### `util.write_file`

```cpp
client.util.write_file(path, content);
```

- 作用：把字符串写入文件，自动建父目录，权限 0644。
- 参数：`path` string、`content` string 必填。
- 返回：boolean `true`。
- 报错：建目录/写文件失败抛 `write_file: <err>`。
- 底层：`os.MkdirAll` + `os.WriteFile(path, []byte(content), 0644)`。
