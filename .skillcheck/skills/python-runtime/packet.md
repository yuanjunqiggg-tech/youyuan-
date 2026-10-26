# packet · 原始协议包

schema/describe查询字段，on/onAll监听，send请求服务器执行；不会取得服务器未下发的数据。


### `packet.describe`

```python
host.packet.describe(typeNameOrId)
```

- 作用：查询一个包的实际ID和字段类型。
- 参数：`typeNameOrId`；按此顺序传入。
- 返回：name/id/fields对象；未知包抛错。
- 底层与游戏效果：可离线查询。联合类型的具体分支不能仅靠顶层schema构造，请读对应协议章节。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `packet.on`

```python
host.packet.on(packetType, handlerName)
```

- 作用：events.subscribe的别名，回调接收字段对象。
- 参数：`packetType, handlerName`；按此顺序传入。
- 返回：注册结果。
- 底层与游戏效果：精确类型订阅，fields里_packet_type为真实类型。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `packet.onAll`

```python
host.packet.onAll(handlerName)
```

- 作用：events.subscribeAll的别名。
- 参数：`handlerName`；按此顺序传入。
- 返回：注册结果。
- 底层与游戏效果：回调接收包信封，fields包含实际包字段。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `packet.schema`

```python
host.packet.schema()
```

- 作用：查询当前编译内核的全部已注册协议包。
- 参数：`无`；按此顺序传入。
- 返回：包数组，每项name/id/fields。
- 底层与游戏效果：从真实packet.ListAllPackets枚举，不根据网上版本猜包ID。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `packet.send`

```python
host.packet.send(typeNameOrId, fieldsTable)
```

- 作用：game.sendPacket的原始协议入口别名。
- 参数：`typeNameOrId, fieldsTable?`；按此顺序传入。
- 返回：提交状态，未确认。
- 底层与游戏效果：同game.sendPacket。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。


## 0.10.34 接收诊断与 1.21.120 挥臂协议

diagnostics.receiveTransport() 的 received 每项新增：
- decoded_frames：已读取完当前定义字段的帧。
- decode_panics：字段读取中断；没有返回正常包。
- trailing_bytes_frames：读取已知字段后仍有扩展字节，不等于丢包。
- returned_packets：协议转换后返回上层的包数，不保证插件回调完成。
- decode_errors：保留旧异常合计；禁止直接当丢包数量。
这些都是当前连接累计值，应对两个时间点取差值；重连后重新计数。

当前网易协议860/游戏1.21.120的 Animate 读取 action_type、entity_runtime_id、固定float32 data，划船动作另外读取 boat_rowing_time。packet.on("animate", handler) 事件保留旧字段，增加 data/boat_rowing_time。Lua、Python、C++使用同一解码链路，不需要重新抓包猜字段。挥臂的action_type=1；data是协议动画参数，不能自行解释为伤害或生命值。UpdateAttributes 的 minecraft:health 才是实体血量来源。

接收统计没有保存原始认证、地址或载荷。队列/历史容量、解码兼容性与服务器广播范围必须分别判断：修改本地缓冲不能生成服务器未发送的包；OP也不能据此宣称全服原生挥臂/血量已实现。普通租赁服的全局原生动作与血量仍未通过真人跨距离/跨维度验收。


### 0.10.34 Lua事件类型与覆盖说明

Lua、Python与C++共用服务器观测数据。Lua转换现在保留float32/int32数字、可空数值指针、固定长度向量和带类型NBT，未知值为nil；不要把指针地址当最大血量。runtime_id是十进制字符串。真实接收、解码、回调和服务器广播范围分别验证。全局RPC伤害显示不是实体实时血量；一个机器人轮巡会有未覆盖时段，使用observerBridge.coverage/health/worldEvents显示实际账号、来源、会话、游标及过期状态。
