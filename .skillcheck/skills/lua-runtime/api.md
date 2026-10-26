# api · 跨插件调用

通过plugin_api导出函数；Lua、Python、C++可相互调用已启动实例。


### `api.require`

```lua
api.require(id)
```

- 作用：获得跨插件调用代理；目标需已经加载并导出plugin_api。
- 参数：`id`；按此顺序传入。
- 返回：Lua代理表或nil；Python代理对象；C++yy::RemotePlugin。
- 底层与游戏效果：Lua→Lua调用同一常驻实例，不重跑源码；跨语言通过序列化桥调用目标导出函数。API不是安装依赖的包管理器。
- 错误：参数不符、未连接或服务器拒绝时按接口返回/异常处理；不要将提交当作游戏成功。
