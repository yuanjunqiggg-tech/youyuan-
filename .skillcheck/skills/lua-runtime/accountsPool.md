# 账号池方法（0.10.41）

完整说明见account-pool-guide.md；与accounts定向消息/模块通信是不同命名空间。

### `accountsPool.add`

account={id,name,token,auth_url,server,password?,keep_alive?}。保存自己的独立游戏账号；Token与签发网址匹配。owner_id由宿主绑定。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.add。MCP游戏工具body为{"method":"accountsPool.add","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.list`

options={all:false}可省略。返回本人账号与process_slots，不返回Token；管理员可all:true。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.list。MCP游戏工具body为{"method":"accountsPool.list","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.start`

id,options={server?,password?,connect:true}。启动真实内核，请求进服。connection_requested不等于connected；默认启用保活。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.start。MCP游戏工具body为{"method":"accountsPool.start","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.status`

id。读取实际connected/engine_ready/插件状态；失败不是已经进服。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.status。MCP游戏工具body为{"method":"accountsPool.status","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.stop`

id。断开并停用该机器人、取消保活，不删除凭据。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.stop。MCP游戏工具body为{"method":"accountsPool.stop","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.bind`

id,options={player,server?}。持续绑定，不自动TP，不抢占已有绑定。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.bind。MCP游戏工具body为{"method":"accountsPool.bind","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.acquire`

options={player,server,id?,all?}。预留并绑定独立账号，优先复用空闲；不自动start、不注册新账号。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.acquire。MCP游戏工具body为{"method":"accountsPool.acquire","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.release`

id,options={idle_seconds:60}。解绑、等待期可复用，超时停止；0立即到期，宿主维护器处理。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.release。MCP游戏工具body为{"method":"accountsPool.release","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.bindings`

options={all:false}可省略。本人持久绑定/空闲状态；管理员all:true。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.bindings。MCP游戏工具body为{"method":"accountsPool.bindings","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.invoke`

id,options={method:GET|POST|DELETE,path,query?,body?}。调用该机器人自己的真实/api/路径。游戏调用body={method,args}；不能绕过账号隔离。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.invoke。MCP游戏工具body为{"method":"accountsPool.invoke","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.observationPlan`

options={accounts:[id...],players:[],dwell_seconds:2,radius_blocks:48}。规划错峰，排除同服机器人，预留一个补位；不登录或执行路线，48只是参考。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.observationPlan。MCP游戏工具body为{"method":"accountsPool.observationPlan","args":[对应参数]}。未知接口/权限失败查看实际错误。

### `accountsPool.handoff`

incoming_id,outgoing_id,options={player,mode,interval_seconds,timeout_seconds:5,next_player?,follow_radius_blocks:16}。先接替再停止原观察；同服且授权，区域证据匹配才ready，失败保留原观察。区域发布不保证所有实体同步。

Lua直接调用；Python同名函数使用dict/list；C++为client.accountsPool.handoff。MCP游戏工具body为{"method":"accountsPool.handoff","args":[对应参数]}。未知接口/权限失败查看实际错误。
