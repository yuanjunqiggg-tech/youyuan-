# 总部查询器（HQ Query API）

与「种子查询器」同级的能力，但**默认不连机器人、不在云端跑计算**。

「总部 / 主服务器」指汇聚多个子服务器（房间 / Realm）的厅服（lobby）。
本功能只做一件事：**只读地把总部及其下属房间拉成一个结构化快照**，所有
**聚合计算与渲染都在本机完成**（APK / Linux 客户端 / 浏览器内的 JS + Canvas）。
云服务器（以及机器人）**不**默认去连接总部、也不替任何第三方程序做高频轮询或跑算法。

## 设计原则（与「不让云端跑」契合）

- 接口只读、按需、低频：单次 `GET` 拉一个快照，无副作用、不建连接、不发包、不写文件。
- **聚合计算不下放云端**：房间利用率、健康分、排序、汇总一律由前端 / 插件本机算，
  接口只返回原始 `rooms`，不在服务端算统计。
- **不默认连机器人**：页面首屏直接用本地内置快照（演示数据）渲染，不自动去连总部、
  不自动轮询。需要真实数据时由用户点一下「尝试从本地网关拉取」，且仍是只读 `GET`。
- 真实数据来自本机网关已发现 / 已登录到的厅服；没有发现总部时如实返回 `hq_known:false`，
  绝不伪造「已连接 / 已发现」状态。

## 接口

```
GET /api/youyuan/world/hq
```

只读、无副作用：不建连接、不发包、不写文件。单次请求 O(1)（仅做快照拷贝）。

### 查询参数

| 参数 | 必填 | 说明 |
|---|---|---|
| `hq_id` | 否 | 显式指定总部 id。给了就以它为准，不依赖是否连着服务器 |
| `region` | 否 | 区域过滤（如 `cn-east` / `cn-north`）；不传返回所有区域 |
| `include_offline` | 否 | 是否在列表里带上 `offline` 房间，默认 `true` |
| `sort` | 否 | 房间排序：`players`（在线人数，默认）/ `latency` / `name` |

### 返回字段（成功）

| 字段 | 说明 |
|---|---|
| `ok` | 固定 `true` |
| `hq` | 总部身份快照：`id` / `name` / `region` / `edition`（固定 `bedrock`）/ `version` / `discovered_at`（RFC3339）/ `source`（`kernel` 内核发现 / `query` 显式传参） |
| `rooms` | 房间原始列表，每项见下 |
| `sampled_at` | 快照时间（RFC3339） |

`rooms[]` 每一项：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 房间 id |
| `name` | string | 房间名 |
| `mode` | string | 模式：`survival` / `creative` / `minigame` / `adventure` |
| `map` | string | 地图标识 |
| `status` | string | `online` / `full`（满员）/ `maintenance`（维护中）/ `offline`（离线） |
| `players_online` | int | 当前在线人数 |
| `capacity` | int | 容量上限 |
| `latency_ms` | int | 到该房间的延迟（毫秒） |
| `uptime_pct` | number | 近 7 天在线率（0–100） |

**聚合（明确不在接口里算，由调用方本机算，见下「本机聚合公式」）**：
接口只返回 `hq` + `rooms` 原始数据，下面这些指标一律前端 / 插件本地算。

### 错误与状态

- 参数错误（如 `sort` 非法）→ `400 {"ok":false,"error":"..."}`
- 方法不是 GET → `405`
- 没发现总部也没传 `hq_id` → `200 {"ok":false,"hq_known":false,"error":"尚未发现总部：请先通过本机网关连上厅服（或显式传 hq_id）。"}`

## 本机聚合公式（前端 / 插件照此实现，云不代算）

设房间集合 R（默认含 `offline`）：

- `total_rooms = |R|`
- `online_rooms = count(status ∈ {online, full, maintenance})`
- `full_rooms = count(status == full)`
- `offline_rooms = count(status == offline)`
- `total_players = Σ players_online`
- `total_capacity = Σ capacity`
- `utilization = total_capacity>0 ? total_players / total_capacity : 0`（0–1）
- `avg_latency = mean(latency_ms)`（仅对 online/full 房间）
- `health_score`（0–100，越高越好）：
  `health = 100 - 40*(offline_rooms/total_rooms) - 25*max(0,utilization-0.85)/0.15 - 20*max(0,avg_latency-200)/300`
  （离线占比、过载、高延迟各扣分；任一因子为负时取 0）

排序：按 `sort` 取对应字段；`players` 降序、`latency` 升序、`name` 字典序。

## 插件 / MCP 用法

- MCP：`/api/youyuan/world/hq` 在只读白名单内，可直接 `tools/call`（`GET`）。
- Lua / Python / C++ 常驻插件：走本机 HTTP 调用
  `GET http://127.0.0.1:<端口>/api/youyuan/world/hq`，解析 `rooms`，按上面公式本机算聚合。
- 前端页面（`features/hq_query.js`）内置一份本地快照（演示数据，形状与接口一致），
  **默认离线渲染**：打开即显示，不做任何网络请求；点「尝试从本地网关拉取」才发一次
  只读 `GET`，拉不到就保留本地快照并提示。房间利用率柱状图用 Canvas 本机绘制，
  拖动 / 重排不依赖服务端。

## 明确不做的事

- 不默认让机器人 / 云端去连接总部并高频轮询；该接口只读、按需、低频。
- 不在云端跑聚合——利用率 / 健康分 / 排序一律本机算，接口只给原始 `rooms`。
- 不伪造「已发现 / 已连接」状态；`hq_known` / `source` 一律取真实值。
- 不把演示快照说成真实总部数据；页面明确标注「本地演示快照」。
