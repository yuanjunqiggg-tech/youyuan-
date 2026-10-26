# OneBot 掉线与二维码诊断

> 只读诊断，未改动任何 `.go` / `.js` / `.css`。
> 行号以本次分析时的工作区版本为准（`youyuan_qq_console.js` 正被其他智能体编辑，行号可能整体偏移 40~50 行；函数名稳定，可据函数名定位）。
> 文中「未证实」= 仓库里没有代码证据，只能给出验证命令，不做结论。

---

## 结论摘要

1. **OneBot 的 WebSocket 客户端既没有心跳、也没有读超时**（`youyuan_qq.go:296-301`），链路被 NAT / 云负载均衡静默掐断后 `conn.Read` 会**永久阻塞**，`yyQQState` 一直停在 `open`——用户看到的「掉线」其实是「连接已死但程序不知道」。这是「动不动掉线 + 不提示」的第一根因（置信度：高）。
2. **掉线后前端不会自己发现**：`/api/qq/status` 只在进页面时拉一次（`youyuan_qq_console.js:1028-1057`、`1190-1213`），三个 `setInterval` 里**没有任何一个刷新 `status`**（`1105-1122`）；后端也**从不推送 `qq_state`**（全仓库无此事件，`features/qq_bridge.js:149` 的 `qbStatePush` 是死代码）。这是「不刷新」的直接根因（置信度：高）。
3. **重连是固定 3 秒、无退避、无上限、无日志**（`youyuan_qq.go:352-359`），NapCat 不在线时会以每分钟 20 次的节奏无限拨号，并且现场没有任何失败记录可供排查（置信度：高）。
4. **二维码的 `fresh` 只是「文件 mtime 是否在 110 秒内」**（`youyuan_qq_login.go:108-124`），而**后端从来没有触发 NapCat 重新生成二维码的能力**（grep `napcat` 只命中一个路径常量）。文件一旦停止被重写，`fresh` 永远 false → 前端永久盖着「正在刷新…」的遮罩，但**实际上没有任何东西在刷新**（置信度：高）。
5. 「正在刷新」永久不消失本身是一份证据：它要求文件存在且 mtime > 110s，说明 **NapCat 已不在产出二维码**（进程死了/卡住/路径指向旧文件）。所以两个现象很可能共享同一个上游原因：**NapCat 进程状态**，而不是本仓库代码把服务器 CPU 吃满了（置信度：中，需用「附：现场取证命令」的命令确认）。

---

## 现象一：掉线

### 证据

#### E1. 读循环没有任何超时，也没有心跳

`upstream/youyuan_qq.go:238-361`（`yyQQConnectLoop`）：

```go
241:  ctx, cancel := context.WithCancel(context.Background())   // 只有「配置变更」才会取消，永不超时
...
288:  conn, _, err := websocket.Dial(ctx, cf.OneBot.URL, &websocket.DialOptions{...})
289:  if err == nil {
292:      yyQQState = "open"
295:      conn.SetReadLimit(2 * 1024 * 1024)
296:      for {
297:          _, raw, e := conn.Read(ctx)     // ← 无 deadline，对端消失时永久阻塞
298:          if e != nil { err = e; break }
...
349:      conn.CloseNow()
```

- `go.mod:9` 是 `github.com/coder/websocket v1.8.14`。这个库**不会自动发 ping**，只有应用自己调用 `conn.Ping(ctx)` 才有保活；它只会在**收到**对端 ping 时自动回 pong。
- 全仓库 grep `Ping(` 在 `youyuan_qq*.go` 里**零命中**。
- 对比：官方 QQ 长连接是做了心跳 + 读超时的（`youyuan_qq_gateway.go:174-193` 心跳协程、`197-202` 每次 `Read` 带 `context.WithTimeout`）。**同一份代码里，官方通道有保活、OneBot 通道没有**，这不是设计取舍，是漏做。

后果链条：云服务器 → 公网 → NapCat 之间只要存在 NAT/安全组/负载均衡的空闲超时（常见 5~15 分钟），TCP 映射被回收时**不会发 FIN/RST**。本端 `conn.Read` 永远等不到数据也等不到错误，`yyQQState` 保持 `"open"`，`/api/qq/status` 也一直报 `open`。用户体感就是「机器人动不动就掉线」，而程序认为一切正常。

#### E2. 断线后的重连是硬编码 3 秒

`upstream/youyuan_qq.go:352-359`：

```go
352:  yyQQMu.Lock()
353:  yyQQConn = nil
354:  yyQQState = "reconnecting"
355:  if err != nil { yyQQDetail = err.Error() }
358:  yyQQMu.Unlock()
359:  time.Sleep(3 * time.Second)
```

- 无指数退避、无抖动、无最大次数（不会「永久放弃」，会无限重试）。
- 每次失败**没有任何 `yyQQLog`**，所以线上出问题时日志里查不到「什么时候掉的、掉了多少次、报什么错」。
- `yyQQDetail` 只保留**最后一条**错误字符串，没有历史环形缓冲（对比 Minecraft 连接那边有 `yyAutoConnection.history`，`youyuan_connection.go:81-84`）。

#### E3. 拨号本身也没有超时

`youyuan_qq.go:288` 用 `ctx`（无 deadline）拨号。若目标 IP 被防火墙 drop（不是 reject），`Dial` 会挂到内核 TCP 超时（Linux 约 2 分钟），这段时间状态一直停在 `"connecting"`（`youyuan_qq.go:285-287`），前端显示「正在接入」而不是「连不上」。

#### E4. 前端只有「进页面时拉一次」，没有任何状态轮询

`upstream/frontend/js/youyuan_qq_console.js:1105-1122`：

```js
function yyQQStartTimers() {
  yyQQStopTimers();
  yyQQC.timers.push(setInterval(yyQQTickQr, 1000));            // 只算二维码年龄
  yyQQC.timers.push(setInterval(function () {
    var v = yyQQView();
    if (v.key === "need_login" || (yyQQC.login && yyQQC.login.state !== "logged_in")) yyQQPollLogin(0);
  }, 20000));                                                  // 只刷 /api/qq/login，且只在「未登录」时
  yyQQC.timers.push(setInterval(function () {                  // 只刷消息流
    if (!document.getElementById("yyq-stream")) return;
    yyQQRefreshStream();
  }, 15000));
}
```

- **没有任何定时器请求 `/api/qq/status`**。`yyQQC.status` 只在 `yyQQLoadCore()`（`1028-1057`）里赋值，而 `yyQQLoadCore()` 只被 `yyQQRefreshAll()` 和进页面时的 `yyQQConsolePage()`（`1204`）调用。
- 更糟：`yyQQRefreshStream()`（`1084-1088`）每 15 秒会 `yyQQRender("status")`，但**渲染用的是缓存里那份旧 `yyQQC.status`**，所以状态栏会持续显示「已连接」，反而给了用户「一切正常」的错觉。
- 20 秒那个定时器还有一道门：`yyQQC.login.state !== "logged_in"`。而 `logged_in` 的判定掺了**游戏连接**（见 E5），所以只要游戏连着，这个定时器连 `/api/qq/login` 都不再请求，页面彻底变成静态快照。

#### E5. `logged_in` 把「QQ 登录」和「游戏连服」混成一个布尔

`upstream/youyuan_qq_login.go:87-96`：

```go
botMgrMu.Lock(); bm := botMgr; botMgrMu.Unlock()
if bm != nil { view.Connected = bm.IsConnected() }   // ← 这是 Minecraft 服务器连接
view.WSListening = yyQQWSListening(cf.OneBot.URL)
view.LoggedIn = view.WSListening && view.Connected    // ← 「QQ 已登录」= WS 在听 且 游戏在线
```

`bm` 是游戏 BotManager。结果：
- 游戏掉线 → `LoggedIn=false` → 若 NapCat 端口没在听，`state` 变 `need_login`（`104`），二维码卡片会突然冒出来；
- 游戏在线但 NapCat 挂了 → 状态判定同样失真。

这两个维度本来正交，不该相乘。

#### E6. 后端不推送状态，前端也不订阅

- 后端只发 `qq_log`：`youyuan_qq.go:66-71`。
- 全仓库 grep `qq_state`：**只在** `features/qq_bridge.js:146-153` 的注释和 `qbStatePush()` 里出现，Go 侧零命中 → `qbStatePush` 没有调用方。
- `frontend/js/sse.js` grep `qq`：**零命中**。所以连 `qq_log` 都没人接，`qbLogPush` 也从未被调用 → 设置页那个「桥接日志」面板永远是空的。
- 结论：QQ 这条链路是**纯拉取**，且只在页面进入时拉一次。

#### E7. 没有明显的忙等/无界增长（回答「是不是服务器配置太差」）

代码层面排查结果：
- 重连循环有 `time.Sleep`（3s / 1s / 10s），**不是忙等**；
- 每条消息**不新建连接**，都走同一个 `yyQQConn`（`youyuan_qq.go:628-654`）；
- `yyQQRecent` 有 10 分钟 TTL 清理（`441-447`），`yyQQPending` 有 `defer delete`（`635`），`yyQQOutgoing` 是容量 256 的 channel 且满了直接丢（`571-575`），没有无界堆积。

真正浪费的两处：
1. `yyQQLoginSnapshot()` 每次请求都 `os.ReadFile` + base64（最大 512KB，`youyuan_qq_login.go:119-122`），前端 20 秒一轮 → 每次约 700KB 响应体，纯浪费。
2. 断线期间固定 3s 拨号，一天约 28800 次（如果拨号本身没超时，还会叠加 E3 的长时间挂起）。

所以：**「配置太差」不是代码证据支持的第一嫌疑**。NapCat 本体是 Node/Electron + QQ 客户端，内存占用大，小内存机器被 OOM kill 很常见——但这一点**未证实**，需要用「附：现场取证命令」命令确认。

### 根因（按可能性排序）

| # | 根因 | 置信度 | 说明 |
|---|---|---|---|
| R1 | 无心跳 + 无读超时 → 半开连接永不恢复 | **高** | E1。链路空闲被中间设备回收后，程序既不知道也不会重连；这是「掉线后界面还显示已连接」的机制性解释 |
| R2 | 前端无状态轮询、后端无状态推送 → 掉线不可见 | **高** | E4 + E6。即使后端正确切到 `reconnecting`，页面也永远显示进页面那一刻的快照 |
| R3 | 重连固定 3s、无退避无日志 → 故障不可观测、可能触发对端限流 | **高** | E2。NapCat 侧 `ws` 服务在频繁重连下可能拒绝或延迟接受 |
| R4 | NapCat 进程本身退出/卡死（登录态失效、被风控踢、OOM） | 中 | 无法从本仓库证实；但现象二的「二维码永久过期」正好是它的旁证 |
| R5 | 云服务器资源不足（OOM / CPU 抢占） | 低~中 | 代码里没有忙等，只能由宿主机证据判定，见「附：现场取证命令」 |

补充一个**不是根因但会放大体感**的点：`yyQQDetail` 被 `yyQQHuman()`（`youyuan_qq_console.js:111-115`）过滤后，`dial|refused|EOF|websocket:` 全部变成空字符串，于是状态栏只剩「重连中」三个字，没有任何可操作信息。

### 修复方案（按优先级）

#### P0-1 给 OneBot 加心跳 + 读超时（`upstream/youyuan_qq.go`）

在 `288` 行 `Dial` 成功后、`296` 行读循环之前插入心跳协程，并把 `Read` 换成带 deadline 的版本：

```go
// 288 行：拨号加超时，避免被防火墙 drop 时挂 2 分钟
dialCtx, cancelDial := context.WithTimeout(ctx, 15*time.Second)
conn, _, err := websocket.Dial(dialCtx, cf.OneBot.URL, &websocket.DialOptions{...})
cancelDial()

// 心跳：30s 一次 Ping；Ping 会等 pong，等不到就返回 error
pingCtx, stopPing := context.WithCancel(ctx)
go func() {
    t := time.NewTicker(30 * time.Second)
    defer t.Stop()
    for {
        select {
        case <-pingCtx.Done():
            return
        case <-t.C:
            pc, c := context.WithTimeout(pingCtx, 10*time.Second)
            e := conn.Ping(pc)   // coder/websocket: 收不到 pong 就报错
            c()
            if e != nil { conn.CloseNow(); return }   // 让阻塞中的 Read 立刻返回
        }
    }
}()

for {
    readCtx, cancelRead := context.WithTimeout(ctx, 90*time.Second) // 3 个心跳周期
    _, raw, e := conn.Read(readCtx)
    cancelRead()
    if e != nil { err = e; break }
    ...
}
```

要点：
- `conn.CloseNow()` 是打断阻塞 `Read` 的手段（`Ping` 失败时对端已不可用，直接关掉让 `Read` 返回）。
- 90s 读超时必须大于心跳周期，否则正常空闲也会误判。
- 状态里再加一个 `last_frame_at`，让前端能显示「已连接但 8 分钟没有数据」。

#### P0-2 重连退避 + 抖动 + 失败日志（`upstream/youyuan_qq.go:352-359`）

新增（注意 `youyuan_qq.go` 目前没有导入 `math/rand`，需要补）：

```go
var yyQQFailures uint64   // 连接成功时清零

func yyQQReconnectDelay(n uint64) time.Duration {
    d := 3 * time.Second
    for i := uint64(1); i < n && d < 60*time.Second; i++ { d *= 2 }
    if d > 60*time.Second { d = 60 * time.Second }
    // ±20% 抖动，避免多实例同频
    j := time.Duration(rand.Int63n(int64(d/5)*2+1)) - d/5
    return d + j
}
```

`359` 行改成 `time.Sleep(yyQQReconnectDelay(yyQQFailures))`；`292` 行置 `yyQQFailures = 0`；`356` 行同时 `yyQQLog(fmt.Sprintf("OneBot 连接中断（第 %d 次）：%v", yyQQFailures, err))`（用 `yyQQLog` 才有 SSE 通道，日志才不会丢）。

#### P0-3 前端 10 秒轮询状态 + 后端 SSE 推送 `qq_state`

最小改动（`upstream/frontend/js/youyuan_qq_console.js:1105-1122`）——加第四个定时器：

```js
// 状态每 10 秒拉一次；页面不在就自动退出（沿用 stream 的守卫写法）
yyQQC.timers.push(setInterval(async function () {
  if (!document.getElementById("yyq-status")) return;
  var st = await A("GET", "/api/qq/status");
  if (!st || typeof st.state !== "string") return;
  var changed = (yyQQC.status || {}).state !== st.state;
  yyQQC.status = st;
  if (changed) { yyQQRender("status"); yyQQRender("hero"); }
}, 10000));
```

再加一个「掉线提示」：`state` 从 `open` 变成非 `open` 时 `T("机器人已断开，正在重连", "e")`，这样用户不看状态栏也能立刻知道。

彻底解法（推荐，但改动更大）：后端在 `yyQQState` 每次变更处统一发 `hub.Emit("qq_state", ...)`（`main.go:89` 的 `Emit` 已经在 `qq_log` 上用通），前端在 `sse.js` 里 `e.addEventListener("qq_state", function(v){ qbStatePush(JSON.parse(v)) })`——`features/qq_bridge.js:149-153` 的 `qbStatePush` 就是为这个写的，现在没有调用方。顺手也把 `qq_log` 接上（`e.addEventListener("qq_log", function(v){ qbLogPush(v) })`），否则桥接日志面板永远是空的。

#### P1-1 拆开「QQ 登录」与「游戏连接」（`upstream/youyuan_qq_login.go:96`）

```go
// 现状
view.LoggedIn = view.WSListening && view.Connected
// 建议
view.LoggedIn = view.WSListening && yyQQStateNow() == "open"   // yyQQStateNow 已存在于 youyuan_qq_caps.go:389
```

`bm.IsConnected()` 保留在 `view.Connected` 里单独返回（前端「游戏身份」那栏需要它），但不要再参与 QQ 登录判定。

#### P1-2 让 `detail` 能被用户看到（`youyuan_qq_console.js:111-115`）

`yyQQHuman` 现在把 `dial/refused/EOF` 全过滤成空串，只剩「重连中」。建议改成：原始错误折进 `title` 属性或折叠区的「详情」，主文案给一句可操作的话（「NapCat 未启动或地址不对，请检查 3001 端口」）。过滤是为了不吓用户，但不能把唯一线索也过滤掉。

#### P2-1 配置变更重连时不要用 `context canceled` 覆盖状态（`youyuan_qq.go:352-357`）

`POST /api/qq/config` 会 `yyQQCancel()`（`147-149`），读循环随即以 `context canceled` 退出，把状态刷成 `reconnecting` + `detail="context canceled"`，覆盖掉配置接口刚设的 `"配置已保存，正在重新接入"`。加一个判断：`if ctx.Err() != nil { /* 主动重连，不要写 detail */ }`。

#### P2-2 二维码接口别每 20 秒重读 512KB 文件

`yyQQLoginSnapshot()` 按 `(path, mtime, size)` 缓存 base64，或改成返回 `/api/qq/login/qr.png?t=<mtime>` 的图片 URL，让浏览器缓存。这与「服务器配置差」直接相关：现在每次轮询都在做无谓的磁盘读 + 700KB 序列化。

---

## 现象二：二维码

### 证据

#### Q1. 二维码来源：本地文件，不是接口

`upstream/youyuan_qq_login.go:19-29`：

```go
func yyNapCatDir() string {
    if v := os.Getenv("YY_NAPCAT_DIR"); v != "" { return v }
    return "/opt/qqbot/napcat"          // ← 硬编码默认值
}
func yyQQQRCodePath() string { return filepath.Join(yyNapCatDir(), "cache", "qrcode.png") }
```

**唯一的来源就是 `<NapCatDir>/cache/qrcode.png` 这个文件**。全仓库 grep `qrcode|get_qrcode|login_qrcode` 只有这一个文件命中，**没有任何调用 NapCat 接口去「要一张新二维码」的代码**，也没有重启 NapCat 的能力（grep `systemctl` 零命中）。

也就是说：**「过期会自动刷新」这句话在代码里没有实现**——它依赖 NapCat 进程自己每 ~2 分钟重写这个文件。我们的服务端只是被动读盘。

#### Q2. `fresh` 与 `age_seconds` 的算法

`upstream/youyuan_qq_login.go:108-128`：

```go
108:  path := yyQQQRCodePath()
109:  if st, e := os.Stat(path); e == nil {
110:      age := int(time.Since(st.ModTime()).Seconds())   // ← 文件 mtime，不是「生成时间」字段
111:      if age < 0 { age = 0 }
114:      view.QRCode.Path = path
115:      view.QRCode.AgeSeconds = age
116:      view.QRCode.Fresh = age <= 110
118:      if !view.LoggedIn {
119:          if raw, e2 := os.ReadFile(path); e2 == nil && len(raw) > 0 && len(raw) < 512*1024 {
120:              view.QRCode.Available = true
121:              view.QRCode.DataURL = "data:image/png;base64," + ...
122:          }
123:      }
124:  }
125:  if view.LoggedIn { view.QRCode = yyQQQRCodeView{} }   // 已登录就把二维码整个抹掉
```

关键点：
- **`Fresh` 完全等价于「文件 mtime 距今 ≤ 110 秒」**。零值时间戳不会出现（`os.Stat` 失败就直接跳过整段，返回零值结构体，`Fresh=false`、`Available=false`）。所以「时间戳零值导致永远 false」这个猜测**不成立**，真正的条件是「文件存在但很久没被重写」。
- `Available` 需要 `!LoggedIn` 且 `0 < size < 512KB`。`LoggedIn` 掺了游戏连接（E5），所以游戏一断，二维码又会被塞进响应里。
- 一旦 `LoggedIn` 为真，`QRCode` 被整体清零——包括 `fresh`/`age`。前端 `yyQQQrAge()` 在 `available=false` 时返回 `null`（`youyuan_qq_console.js:1125`），于是年龄数字**冻结在上一次渲染的值**，不会自己归零。

#### Q3. 前端的遮罩逻辑

`upstream/frontend/js/youyuan_qq_console.js:330-343`：

```js
330: function yyQQQRCode(lg) {
331:   var q = (lg && lg.qrcode) || {};
332:   var has = !!q.data_url;
333:   return '<div class="yyq-qr" id="yyq-qr" data-fresh="' + (q.fresh ? "1" : "0") + '">' +
336:       (has ? '<img src="' + yyQQE(q.data_url) + '" ...>'
337:            : '<span class="yyq-qr-none">' + ... + "<small>暂未取到二维码</small></span>") +
338:       (has && !q.fresh ? '<span class="yyq-qr-mask">正在刷新…</span>' : "") +   // ← 只要过期就盖
```

`1130-1148` 的每秒 tick 也在做同一件事：`fresh = age <= 110`，不 fresh 就 `insertAdjacentHTML` 加遮罩。

所以判定条件是 **`has && !fresh`**，与「是否真的有人在刷新」完全无关。文件停在 3 小时前 → 遮罩「正在刷新…」挂 3 小时，下面的小字还会显示「二维码 10800 秒前生成」，自相矛盾。

#### Q4. 刷新路径：前端 20 秒拉一次，但拉到的还是同一张旧图

`1150-1176`：

```js
1150: async function yyQQPollLogin(force) {
1151:   var r = await A("GET", "/api/qq/login");
1152:   if (!r || !r.ok) return;              // 失败就静默返回，保留旧图（这点是对的）
1154:   yyQQC.login = r;
1155:   yyQQC.qrBase = (r.qrcode && r.qrcode.age_seconds) || 0;
1156:   yyQQC.qrFetchedAt = Date.now();
...
1163:   if (nowNeed && img && newUrl && newUrl !== yyQQC.lastQrUrl) { img.src = newUrl; ... }
```

- 轮询**不会**触发 NapCat 重新生成二维码，只是把同一张过期图片的 `age_seconds` 重新算一遍 → 遮罩永远不撤。
- 401/500 时 `return` 提前退出，**不会清空已有二维码**（符合用户「不要删已有网址」的要求），但同时也**不会提示任何错误**，页面会静默停留在旧快照上（与 E4 同源）。
- 没有「刷新二维码 / 重启 NapCat」按钮，UI 上无路可走。

#### Q5. 遮罩的文案在撒谎

`yyQQQRCode` 里 `has && !q.fresh` 一律写「正在刷新…」。但后端此刻的状态可能是「NapCat 进程已经不存在」，没有任何刷新在进行。这句文案把「没有新图」误报成「正在生成新图」，直接把排查方向带偏。

### 根因

| # | 根因 | 置信度 | 说明 |
|---|---|---|---|
| R6 | 后端没有「触发 NapCat 重新生成二维码」的能力，`fresh` 只是 mtime 的映射 | **高** | Q1 + Q2。文件一旦停止被重写，`fresh` 永远 false，且没有任何代码会去修复它 |
| R7 | 「正在刷新…」的显示条件与「是否真的在刷新」无关，只与 `fresh` 有关 | **高** | Q3 + Q5。这是「一直显示正在刷新」的直接实现原因 |
| R8 | 前端轮询只重算年龄，不重新取图；没有刷新入口 | **高** | Q4 |
| R9 | NapCat 进程未运行 / 卡死 / 二维码写在别处（`YY_NAPCAT_DIR` 指错） | 中 | 逻辑推论：永久 stale 意味着文件 > 110s 未被重写；需「附：现场取证命令」命令确认是「进程死了」还是「路径指到了旧文件」 |
| R10 | `LoggedIn` 掺入游戏连接，导致二维码区块的出现/消失与游戏状态耦合 | 中 | Q2（E5）。会让二维码卡片在游戏掉线时突然出现，且此时 `available` 可能是拿的旧图 |

注：`AgeSeconds` 用 mtime 本身没有错，但云服务器 NTP 未同步时 `time.Since(mtime)` 可能为负 → 被 clamp 成 0 → 永远 `fresh`（反向问题：过期图被当成新图）。建议后端额外返回 `generated_at`（RFC3339），前端用同一台机器的时钟做差。

### 修复方案

#### P0-4 先让文案说真话（纯前端，5 行）

`youyuan_qq_console.js:338` 与 `1141-1143` 的遮罩条件改成「过期且确实正在刷新」：

```js
// 后端新增字段 refreshing: bool（本次请求是否刚触发过一次刷新）
(has && !q.fresh ? '<span class="yyq-qr-mask">' + (q.refreshing ? "正在刷新…" : "二维码已过期，请点『重新获取』") + '</span>' : "")
```

`age > 110` 时同时把二维码图片置灰（`filter: grayscale(1) opacity(.4)`），避免用户去扫一张必然失效的码。这一条能立刻消除「永远转圈」的假象。

#### P0-5 给后端加一个真正的刷新入口（`upstream/youyuan_qq_login.go`）

新增 `POST /api/qq/login/refresh`，语义明确、失败可解释：

```go
// 1) 删掉过期文件（或改名为 qrcode.stale.png），让前端立刻回到「暂未取到二维码」
//    —— 前提是 NapCat 会重写它；不重写就说明进程有问题，直接报错给用户
// 2) 依次尝试：
//    a. 若配置了 YY_NAPCAT_RESTART_CMD，执行它（由部署方决定是 systemctl restart napcat
//       还是 docker restart napcat；本仓库不硬编码，避免猜错部署方式）
//    b. 否则返回 {ok:false, error:"二维码由 NapCat 生成，当前没有重启权限；请在服务器执行 systemctl restart napcat"}
// 3) 返回 {ok:true, waited_ms: N, qrcode: {...}} —— 轮询等待文件 mtime 变新，最多 20s
```

配套：`/api/qq/login` 的响应里加 `refreshing`（是否有刷新在进行）、`generated_at`、`stale_seconds = max(0, age-110)`。

**产品决策点**：是否允许这个面板重启 NapCat 进程。允许 → 需要 `YY_NAPCAT_RESTART_CMD` 的权限边界（谁能调这个接口）；不允许 → 至少要把「请去服务器执行 systemctl restart napcat」这句话显示出来，而不是让用户对着「正在刷新」干等。

#### P1-3 `fresh` 判定的口径写清楚（`youyuan_qq_login.go:116`）

保持 `age <= 110` 的阈值没问题（NapCat 约 2 分钟换一次码），但要补两点：
- 加 `generated_at`（`st.ModTime().Format(time.RFC3339)`）让前端自己算，避免时钟/时区漂移；
- `Fresh` 与 `Available` 解耦：现在 `Available` 依赖 `!LoggedIn`，导致「已登录时 fresh 恒为 false」，前端任何依赖 `fresh` 的展示都会在登录瞬间跳变。

#### P1-4 二维码不要每个轮询都 base64 一遍

同 P2-2：按 `(path, mtime, size)` 缓存，或改成图片 URL。

#### P2-3 加载失败不要静默

`1152` 行 `if (!r || !r.ok) return;` 建议加一个连续失败计数：连续 3 次失败就在状态栏显示「读取登录态失败（HTTP xxx）」，而不是永远显示上一次的快照。

---

## 需要产品决策的点

1. **面板能不能重启 NapCat？** 这决定二维码能不能真正「自动刷新」。不给权限的话，二维码这块只能做到「如实告诉你它过期了，请去服务器重启」。
2. **状态刷新频率**：用户原话是「大约 10 分钟」，但 `/api/qq/status` 只读内存，10 秒一次的成本可以忽略。建议 10 秒；10 分钟只适合「全量数据」刷新（消息、资料、群列表）。需要确认是否接受更高频的轮询。
3. **掉线要不要主动通知**（站内 toast / 桌面通知 / 群里发一条）？现在完全静默。若要「群里发一条」，得先确认 OneBot 断线时走哪条通道发——这条通道本身可能也断了，属于鸡生蛋问题。
4. **`YY_NAPCAT_DIR` 的部署约定**：默认 `/opt/qqbot/napcat` 是硬编码的。是否要在设置页暴露这个路径，让用户自己填？否则换一种部署方式（docker、别的目录）二维码永远读不到。

---

## 附：现场取证命令（只读，用于确认 R4/R5/R9）

```bash
# 1) NapCat 进程到底在不在、跑了多久（重启过的话 Elapsed 会很小）
ps -eo pid,etime,rss,cmd | grep -i napcat | grep -v grep
systemctl status napcat --no-pager

# 2) 二维码文件到底多久没更新了（关键：对比 mtime 与当前时间）
ls -l --time-style=full-iso /opt/qqbot/napcat/cache/qrcode.png

# 3) 3001 端口是否真的在监听（未登录时 NapCat 活着但不监听）
ss -ltnp | grep 3001

# 4) 是否被 OOM 杀掉过（回答「云服务器配置太差」）
dmesg -T | grep -i -E "out of memory|killed process" | tail -20
free -m

# 5) NapCat 自己的日志里有没有反复出码 / 掉登录
journalctl -u napcat --since "3 hours ago" --no-pager | grep -i -E "qrcode|login|error|kick" | tail -50
```

判读方式：
- 第 2 条 mtime 停在很久以前 + 第 3 条无监听 → **NapCat 没在产出二维码**（R9），此时「正在刷新」是纯粹的假象，先修进程，再修文案。
- 第 4 条有 `killed process` → **R5 成立**，服务器内存确实不够，需要加 swap 或升配，但**同时**仍要修 E1（心跳），因为心跳缺失与内存无关。
- 第 2 条 mtime 很新（< 2 分钟）但前端仍显示「正在刷新」→ 说明是**路径不一致**（读到了另一个目录的旧文件），查 `YY_NAPCAT_DIR`。
