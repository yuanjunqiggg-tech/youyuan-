# storage · 缓存与事务

内存键值、原子加分、自动持久化与条件事务。


### `storage.atomicAdd`

```cpp
client.storage.atomicAdd(key, delta);
```

- 功能、参数、返回与约束：key,delta=1；持锁原子加，默认从0开始；非数字报错，不发生读改写竞争。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.delete`

```cpp
client.storage.delete_(key);
```

- 功能、参数、返回与约束：key；删除缓存键，后台持久化。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.flush`

```cpp
client.storage.flush();
```

- 功能、参数、返回与约束：无参数；同步写临时文件、Sync、原子替换storage.json；失败报真实路径错误并保留dirty。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.get`

```cpp
client.storage.get(key, default);
```

- 功能、参数、返回与约束：key,default?；内存读取并复制值，账号/插件的数据目录独立；首次读取storage.json一次。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.set`

```cpp
client.storage.set(key, value);
```

- 功能、参数、返回与约束：key,value；复制JSON可表示的值到内存，后台约1秒自动原子持久化；重要提交用flush或transaction。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。

### `storage.transaction`

```cpp
client.storage.transaction(operations);
```

- 功能、参数、返回与约束：operations数组；op=set/add/delete/check,key,value。check值不符则整批回滚；整批验证后持锁同步持久化成功才替换缓存。不是任意Lua函数/跨文件事务。
- 宿主实现：当前实例绑定到对应账号；错误不会伪造成功；服务器未同步的数据保持未知。
