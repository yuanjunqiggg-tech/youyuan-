# web · 插件控制页面

声明式页面、HTML资源、动作回调与事件推送，重载清理旧注册。


### `web.action`

```cpp
client.web.action(page, action, handler);
```

- 作用：绑定一个交互动作——浏览器对 `/web/api/<插件id>/<页面名>` 发请求时，调用常驻实例的 `handler` 处理。
- 参数：`page` string、`action` string 必填；`handler` **string 必填**（全局**函数名字符串**，不是函数引用）。
- 返回：无。
- 报错：缺插件上下文/空名/重复注册抛异常。
- 底层：`registerWebAction`；执行时新建 LState（2 分钟超时）进入常驻实例后 `CallByParam` 调 handler，返回值经 `luaToGo` 作为响应 `data` 回传。受独立并发信号量（容量 8）约束。
- 回调：Lua传全局函数名字符串；Python可传函数对象或模块函数名；C++先用on注册再传处理器名。停用/重载清理本实例注册。

### `web.emit`

```cpp
client.web.emit(page, event, data);
```

- 作用：把一条事件实时推送给该插件的全部 SSE 客户端。声明式页面内置 JS 按 key 更新控件；自定义页 JS 自行监听。用于后台状态/进度推送。
- 参数：`page` string 必填（作为 SSE 事件名）；`event` string 必填（会写入数据 map 的 `__event` 键）；`data` 任意 JSON兼容值必填。
- 返回：无。
- 报错：数据无法 JSON 序列化抛 `web.emit: <err>`。
- 底层：`pushWebEvent(plugin, page, dataJSON)`，构造 SSE 消息写入所有客户端通道（通道满则丢弃，不阻塞 Lua）。**此方法在重跑上下文不跳过**（供 handler 内推送用）。

### `web.page`

```cpp
client.web.page(name, desc, controls);
```

- 作用：声明一个**声明式页面**，在门户 `/web/` 显示卡片，点进由内置模板渲染控件页。
- 参数：`name` string 必填（插件内唯一，不含 `/ \ # ? %` 及控制字符）；`desc` string 可选，默认 `""`；`controls` 对象或数组 可选（控件表，可读字段 `type`=text|button|input|对象或数组|switch、`key`、`label`、`action`、`placeholder`、`columns`）。
- 返回：无。
- 报错：空名/非法字符/同插件页面名重复注册抛异常。
- 底层：`registerWebPage(&webPage{Kind:"declarative",...})`，写入全局 `webPages` map。

### `web.serve`

```cpp
client.web.serve(name, entry);
```

- 作用：声明一个**自定义 HTML 页面**，直接服务 `www/` 目录下对应入口文件。
- 参数：`name` string 必填（规则同 `web.page`）；`entry` string 可选，默认 `"index.html"`（相对 `www/`）。
- 返回：无。
- 报错：同 `web.page`。
- 底层：`registerWebPage(&webPage{Kind:"html", Entry:entry})`；静态根目录 `插件文件/<id>/www`，经 `webStaticSafe` 防路径逃逸 + `serveWebStatic` 提供。
