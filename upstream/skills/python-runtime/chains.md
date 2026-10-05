# chains · 命令链管理

同一套TXT预解析、整链频率、启停、跨链与聊天菜单，可供插件调用。


### `chains.configure`

```python
host.chains.configure(name, patch)
```

- 功能、参数、返回与约束：name,patch，合并并校验配置，保留 TXT；运行中会停止并按 enabled 重启。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.delete`

```python
host.chains.delete(name)
```

- 功能、参数、返回与约束：name，先停止再删除 TXT 与配置。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.export`

```python
host.chains.export(name)
```

- 功能、参数、返回与约束：name，返回完整 TXT 与配置，插件可自行写入导出目录。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.get`

```python
host.chains.get(name)
```

- 功能、参数、返回与约束：name，读取完整 TXT、配置和状态。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.handleChat`

```python
host.chains.handleChat(player, text)
```

- 功能、参数、返回与约束：player,text；处理 .命令选择 与 .链 列表/启动/停止/次数/频率/通道/源码/删除。修改要求玩家 OP，返回 handled。配合 events.onChat。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.list`

```python
host.chains.list()
```

- 功能、参数、返回与约束：无参数，列出当前账号的链、配置、计数、错误和状态。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.menu`

```python
host.chains.menu(player, page)
```

- 功能、参数、返回与约束：player,page=1，仅向该玩家发分页聊天菜单；不会阻塞等待消息。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.reload`

```python
host.chains.reload(name)
```

- 功能、参数、返回与约束：name；只重新读取/编译TXT资源，当前轮使用原IR，下轮用新IR；不重启插件、保留启停与统计；无效修改保留原链。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.save`

```python
host.chains.save(name, content, config)
```

- 功能、参数、返回与约束：name,content,config；config 支持 interval_ms(至少1，整链周期)/delay_ms/run_count/channel/enabled/mode/window/rate_cps。mode=fast无等待提交（条件及其前条确认）、remote严格顺序、pipeline有界流水线、burst整批最多1024、stream动态滑动窗口1..1024（默认128，rate_cps=0不设固定CPS，回包立即补位，网络错误停止补发；正常无目标不丢弃无条件后续）；全部在机器人内运行。TXT支持 @call JSON 跨链源文件行号范围/次数/时限/持续，@wait JSON 条件边沿等待，@pause手动触发；读 command-chain 技能。账号固定为当前插件账号。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.start`

```python
host.chains.start(name)
```

- 功能、参数、返回与约束：name，启用独立常驻实例；已启用不重复启动。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.stop`

```python
host.chains.stop(name)
```

- 功能、参数、返回与约束：name，停止并等待当前实例退出。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.trigger`

```python
host.chains.trigger(name)
```

- 功能、参数、返回与约束：name，释放当前链的 @pause 等待点一次；没有等待点时不会自动重启已停止的链。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `chains.validate`

```python
host.chains.validate(content)
```

- 功能、参数、返回与约束：content，预解析并返回命令/条件/通道/行号；失败含报错。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
