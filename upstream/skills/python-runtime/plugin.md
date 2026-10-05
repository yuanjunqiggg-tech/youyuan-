# plugin · 实例资源与诊断

目录、资源、能力与统计；代码、配置、持久数据分开管理。


### `plugin.capabilities`

```python
host.plugin.capabilities()
```

- 功能、参数、返回与约束：无参数，当前平台可用API名称和语言运行器能力；按需读取，不自动整库塞AI上下文。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.codeDir`

```python
host.plugin.codeDir()
```

- 功能、参数、返回与约束：无参数，当前插件源码根目录。Lua模块可require，模块只在该常驻VM首次require时加载。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.configDir`

```python
host.plugin.configDir()
```

- 功能、参数、返回与约束：无参数，当前插件配置目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.dataDir`

```python
host.plugin.dataDir()
```

- 功能、参数、返回与约束：无参数，当前插件持久数据目录；SDK按账号隔离。旧game.dataDir保持兼容。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.dependencies`

```python
host.plugin.dependencies()
```

- 作用：返回当前插件的依赖声明（`pre_plugins`）。
- 参数：无。
- 返回：表（原样透传全局 `plugin_dependencies` 的值）。
- 报错：无。
- 底层：读全局 `plugin_dependencies`。

### `plugin.existsResource`

```python
host.plugin.existsResource(relative)
```

- 功能、参数、返回与约束：relative_path，检查资源是否存在；非法路径报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.extractResource`

```python
host.plugin.extractResource(source, destination)
```

- 功能、参数、返回与约束：source_relative,destination_relative默认同源；将资源原字节复制至plugin.dataDir中的目标，创建父目录。不是解压ZIP，不写任意外部路径。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.has`

```python
host.plugin.has(method)
```

- 功能、参数、返回与约束：method或runtime.lua/runtime.python/runtime.cpp，查询宿主API是否提供；不代表服务器一定支持动作或已观测到血量。Linux C++当前false。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.id`

```python
host.plugin.id()
```

- 作用：返回当前插件的 id 字符串。
- 参数：无。
- 返回：string。
- 报错：无。
- 底层：读全局 `__PLUGIN_ID__`。

### `plugin.listResources`

```python
host.plugin.listResources(relative)
```

- 功能、参数、返回与约束：relative_path默认.，递归列出目录和文件，含path/dir/size；不跟随符号链接。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.readResource`

```python
host.plugin.readResource(relative, encoding)
```

- 功能、参数、返回与约束：relative_path,encoding=utf8；读取资源字符串；二进制跨JSON使用encoding=base64。拒绝绝对路径、越界与符号链接。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.resourceDir`

```python
host.plugin.resourceDir()
```

- 功能、参数、返回与约束：无参数；有resources子目录则使用该目录，否则兼容使用代码目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.stats`

```python
host.plugin.stats()
```

- 功能、参数、返回与约束：无参数，当前常驻实例回调数、失败、墙钟耗时及队列深度/丢弃；CPU/内存/GC当前未逐实例测量，返回null，不虚构性能倍数。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `plugin.tempDir`

```python
host.plugin.tempDir()
```

- 功能、参数、返回与约束：无参数；插件数据目录下.tmp，可自行删除临时文件。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
