# system · Android设备

系统桥、通知、震动、Shizuku；桌面支持范围与Android不同。


### `system.call`

```lua
system.call(method, paramsTable)
```

- 作用：通用底层透传——把 `method` 和参数表交给 Java 分发执行，返回任意结果。已支持的方法：`shell`、`shizukuStatus`、`shizukuRequest`、`vibrate`、`notify`、`launchApp`；未知方法返回 `{"ok":false,"error":"unknown method: ..."}`。
- 参数：`method` string 必填；`paramsTable` table 可选。
- 返回：任意 Lua 值（Java 返回 JSON 解码结果）；无返回推 `nil`。
- 报错：参数序列化失败或桥调用失败抛 `system.call: <err>`。
- 底层：参数 `json.Marshal` → `callAndroidSystem` → JNI → Java `FloatingBridge.call` switch 分发。

### `system.notify`

```lua
system.notify(title, body)
```

- 作用：发系统状态栏通知（**不需要** Shizuku/ADB 权限）。
- 参数：`title` string 必填；`body` string 可选，默认 `""`。
- 返回：table `{ok=true}`；异常时 `{ok=false, error=...}`。
- 报错：桥调用失败抛 `system.notify: <err>`。
- 底层：参数 `{"title","body"}` → JNI → Java `postDirectNotification`。

### `system.platform`

```lua
system.platform()
```

- 作用：返回当前运行平台。
- 参数：无。
- 返回：string——`"android"` 或 `"desktop"`。
- 报错：无。
- 底层：`getPlatformName()`（不经 JNI 桥）。

### `system.shell`

```lua
system.shell(cmd)
```

- 作用：以 Shizuku(ADB) 身份执行一条 shell 命令。
- 参数：`cmd` string 必填。
- 返回：table `{ok=bool, exit=number, stdout=string, stderr=string, uid=number, error?=string}`。命令本身失败不抛异常，走 `ok=false`。
- 报错：仅桥调用失败抛 `system.shell: <err>`。
- 底层：参数 `{"command":cmd}` → JNI → Java `runShell`：空命令/Shizuku 未运行/未授权各有明确错误；否则经反射调 `Shizuku.newProcess` 执行 `/system/bin/sh -c <cmd>`，读 stdout/stderr、取退出码与 uid。

### `system.shizukuRequest`

```lua
system.shizukuRequest()
```

- 作用：弹出系统授权对话框，申请 Shizuku ADB 权限。
- 参数：无。
- 返回：table `{ok=bool, requested=bool, error?=string}`。
- 报错：桥调用失败抛 `system.shizukuRequest: <err>`。
- 底层：JNI → Java `requestShizukuPermission`：pingBinder 失败返回未运行；否则 `Shizuku.requestPermission()` 弹授权框。

### `system.shizukuStatus`

```lua
system.shizukuStatus()
```

- 作用：查询 Shizuku 运行状态与授权情况。
- 参数：无。
- 返回：table `{available=bool, granted=bool, uid=number}`（失败时 uid=-1）。
- 报错：桥调用失败抛 `system.shizukuStatus: <err>`。
- 底层：JNI → Java `shizukuStatusJson`：`Shizuku.pingBinder()`、`checkSelfPermission()==GRANTED`、`getUid()`。

### `system.vibrate`

```lua
system.vibrate(ms)
```

- 作用：触发手机震动（**不需要** Shizuku/ADB 权限）。
- 参数：`ms` number 可选，默认 500（毫秒）。
- 返回：table `{ok=true}`；异常时 `{ok=false, error=...}`。
- 报错：桥调用失败抛 `system.vibrate: <err>`。
- 底层：参数 `{"ms":ms}` → JNI → Java `vibrate`（Vibrator）。
