# packet · 原始协议包

schema/describe查询字段，on/onAll监听，send请求服务器执行；不会取得服务器未下发的数据。


### `packet.describe`

```cpp
client.packet.describe(typeNameOrId);
```

- 作用：查询一个包的实际ID和字段类型。
- 参数：`typeNameOrId`；按此顺序传入。
- 返回：name/id/fields对象；未知包抛错。
- 底层与游戏效果：可离线查询。联合类型的具体分支不能仅靠顶层schema构造，请读对应协议章节。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `packet.on`

```cpp
client.packet.on(packetType, handlerName);
```

- 作用：events.subscribe的别名，回调接收字段对象。
- 参数：`packetType, handlerName`；按此顺序传入。
- 返回：注册结果。
- 底层与游戏效果：精确类型订阅，fields里_packet_type为真实类型。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `packet.onAll`

```cpp
client.packet.onAll(handlerName);
```

- 作用：events.subscribeAll的别名。
- 参数：`handlerName`；按此顺序传入。
- 返回：注册结果。
- 底层与游戏效果：回调接收包信封，fields包含实际包字段。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `packet.schema`

```cpp
client.packet.schema();
```

- 作用：查询当前编译内核的全部已注册协议包。
- 参数：`无`；按此顺序传入。
- 返回：包数组，每项name/id/fields。
- 底层与游戏效果：从真实packet.ListAllPackets枚举，不根据网上版本猜包ID。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。

### `packet.send`

```cpp
client.packet.send(typeNameOrId, fieldsTable);
```

- 作用：game.sendPacket的原始协议入口别名。
- 参数：`typeNameOrId, fieldsTable?`；按此顺序传入。
- 返回：提交状态，未确认。
- 底层与游戏效果：同game.sendPacket。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
