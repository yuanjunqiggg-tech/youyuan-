# 种子查询器（世界种子 API）

与「导入 / 导出」同级的大功能。玩家在工具箱里选好版本号后即可查看、滑动地图；
**所有计算与渲染都在本机完成**（APK / Linux 安装内的 JS + Canvas），云服务器只转发这一个
只读快照接口，不替任何第三方程序跑算法。

## 接口

```
GET /api/youyuan/world/seed
```

只读、无副作用：不建连接、不发包、不写文件，单次请求 O(1)（带结构试算时除外）。

### 查询参数

| 参数 | 必填 | 说明 |
|---|---|---|
| `seed` | 否 | 显式指定种子。给了就以它为准，不依赖是否连着服务器 |
| `structure`（或 `type`） | 否 | 结构标识：`village` / `fortress` / `bastion` / `monument` / `mansion` / `temple` |
| `edition` | 否 | 只有 `java` 有意义。要结构试算必须显式写 `edition=java` |
| `from_x` `from_z` `to_x` `to_z` | 否 | 试算范围（区块坐标），默认各 `-256..256`；单轴跨度不能超过 4096 |
| `lod` | 否 | 试算分级细节 `0/1/2`，默认 `0` |

### 返回字段（成功）

| 字段 | 说明 |
|---|---|
| `seed` | JSON number。超过 2^53 时部分客户端会丢精度 |
| `seed_text` | 精确十进制字符串。**跨语言 / 插件 / MCP 一律优先用这个** |
| `seed_known` | 是否真的拿到了种子。`false` 时同时带 `error` 与 `ok:false` |
| `source` | `kernel`（内核登录包读到）或 `query`（显式传参） |
| `edition` | 固定 `bedrock`：内核走基岩版协议，读到的是基岩版语义种子 |
| `connected` / `joined` | 内核当前连接 / 进服状态 |
| `server` / `world_name` / `dimension` | 连接上的世界身份快照 |
| `sampled_at` | 快照时间（RFC3339） |
| `precision` | `exact_raw_field`；带试算时为 `exact_raw_field+candidate_preview` |
| `java_preview` | 仅 `structure` + `edition=java` 时出现，见下 |

### 结构试算（`java_preview`）

```text
GET /api/youyuan/world/seed?seed=-4172144997902289642&structure=village&edition=java&from_x=-64&from_z=-64&to_x=64&to_z=64
```

返回 `structure` / `label` / `chunks`（候选区块列表）/ `count` / `truncated` / `lod` /
`sampled` / `skipped_by_lod` / `degraded` / `reason` / `precision=candidate` /
`applies_to_world=false`。

**边界（必须如实传达，不许冒充）**：

- 这是用 **Java 版结构算法**对同一个数字做的试算，不是当前基岩世界里的真实结构位置；
  实际位置必须进游戏实测。
- 不传 `edition=java` 却传了 `structure` → 400，并说明算法不适用。
- 没拿到种子却要试算 → 409，提示先连接或显式传 `seed`。
- 范围超限 → 400（单轴区块跨度上限 4096）。
- 试算复用内核扫描器的 150ms 预算与区域上限，超限时 `truncated=true`、`degraded` 说明原因。

### 错误与状态

- 参数错误 → `400 {"ok":false,"error":"..."}`
- 方法不是 GET → `405`
- 没连服务器也没传 `seed` → `200 {"ok":false,"seed_known":false,"error":"尚未从内核读取到世界种子：请先连接服务器并完成登录（StartGame 包到达后才有值），或显式传 seed 参数。"}`

## 数据来源与「0」的含义

种子取自登录 `StartGame` 包里的 `WorldSeed` 字段，是本机从服务器**实际收到**的值。
服务端普遍会把真实种子替换成 `0`：这时接口照样返回 `0`，但 `seed_zero_hint`
明确写着「seed=0 时应视为未知，不要当成真实世界种子」。

## 插件 / MCP 用法

- MCP：`/api/youyuan/world/seed` 已在只读白名单内，可直接 `tools/call`。
- Lua / Python / C++ 常驻插件：走本机 HTTP 调用
  `GET http://127.0.0.1:<端口>/api/youyuan/world/seed`，解析 `seed_text`。
- 前端页面（`youyuan_seed_page.js` + `youyuan_seed_local.js`）把 Java 版算法
  （`java.util.Random`、Java 1.13+ 结构网格）移植成了浏览器 BigInt 实现，
  64 位运算用 `BigInt.asIntN` 保证远端坐标不丢精度；地图渲染是 Canvas，
  支持拖动 / 滚轮 / 双指缩放。页面不依赖云服务器算力。

## 离线种子工具组（`/api/seed/*`，纯本机计算）

上面那个接口是「读当前世界的种子」；这一组是「拿任意种子做计算」，全部 GET、只读、
不联网、不读档，可被插件与 MCP 直接调用。能力清单以 `/api/seed/manifest` 为准。

| 接口 | 精度 | 说明 |
|---|---|---|
| `/api/seed/manifest` | — | 能力清单 + 限制 + LOD 档位 + **结构清单（21 种）** + **维度清单（3 个）** + **版本覆盖矩阵（7 档）** |
| `/api/seed/slime` | exact | 史莱姆区块（Java 原版算法 + 盐值 `0x3ad8025f`） |
| `/api/seed/stronghold` | approximate | 要塞三环（半径 1408/4352/7424 方块，每环 128 候选角），省略生物群系过滤 |
| `/api/seed/structure` | candidate / approximate / unsupported | 结构候选区块，`type=<id>` 或 `type=all`；支持 `dimension` |
| `/api/seed/heat` | candidate | 结构热点图（32×32 / 64×64 / 128×128 格聚合） |
| `/api/seed/query` | 混合 | 一句话聚合：要塞 + 史莱姆 + 各类结构，带中文 `summary` |
| `/api/seed/coordinates` | exact | 坐标分享码（base36 + CRC32，可反解） |
| `/api/seed/biome` | approximate（末地 exact） | 生物群系：单点 `x&z` 或批量 `from_x/from_z/to_x/to_z&step` |

### 结构清单（21 种，按维度）

- **主世界 17 种**：`village` `desert_pyramid` `jungle_pyramid` `swamp_hut` `igloo`
  `ocean_ruin` `shipwreck` `monument` `mansion` `pillager_outpost` `ruined_portal`
  `ancient_city` `trail_ruins` `trial_chambers`（区域网格）、`stronghold`（同心环）、
  `mineshaft` / `buried_treasure`（区块概率，**给不出确定坐标，只给概率**）
- **下界 3 种**：`fortress` `bastion` `nether_fossil`
- **末地 1 种**：`end_city`
- 历史别名仍可用：`temple`→`desert_pyramid`、`witch_hut`→`swamp_hut`、
  `nether_fortress`→`fortress`、`endcity`→`end_city` 等（见 `yySeedStructureAliases`）。
- 不传 `dimension` 时按结构自身维度自动推断；显式传了冲突的维度会 400。

### 版本覆盖矩阵（`manifest.version_matrix`）

| 区间 | 群系算法 | 结构算法 | 精度 |
|---|---|---|---|
| 1.7 ~ 1.12 | 温湿度双噪声 | 不覆盖（1.13 前参数不同） | 群系 approximate / 结构 **unsupported** |
| 1.13 ~ 1.17 | 温湿度双噪声 | 区域网格 + 盐值 | approximate / candidate |
| 1.18 ~ 1.18.2 | 气候噪声栈（五参数） | 区域网格 + 盐值 | approximate / candidate |
| 1.19 ~ 1.19.4 | 气候噪声栈 | + 远古城市 | approximate / candidate |
| 1.20 ~ 1.20.6 | 气候噪声栈 | + 古迹废墟 | approximate / candidate |
| 1.21+ | 气候噪声栈 | + 试炼密室 | approximate / candidate |
| 基岩版 | 不实现（RNG 不同源） | 不实现 | **unsupported** |

`manifest.game_versions` 才是版本下拉该用的真实版本号列表（1.21.4 … 1.7）；
`version_matrix` 是覆盖区间说明，两者不要混用。

### 群系算法的两个坑（改代码前必读）

1. **噪声必须归一化**：`yyOctave` 按 `√(Σ振幅²)` 归一。早期按 `Σ振幅` 归一，把 16 层压到
   std≈0.05，阈值全失效，实测得到「54% 深海 + 98% 下界荒地」的荒谬分布。
2. **取样格点不能与噪声整数格点重合**：Perlin 在整数格点恒为 0。前端取样步长全是
   2 的幂、坐标常从 0 起，不做偏移会让湿度/怪异度整片变 0。现做法是每个八度加
   互不相同的无理相位（黄金比 / 白银比）。

`youyuan_seed_biome_diag_test.go` 用质数步长（1237/1499）做大样本分布统计，
是调整阈值时的唯一依据——别用小范围采样下结论。

## 明确不做的事

- 不把 Java 版结构结果说成基岩世界的真实结构位置。
- 不伪造「已连接」状态；`connected` / `joined` 一律取内核真实字段。
- 不为省事把 `seed` 直接当 number 传给调用方——2^53 以上必丢精度，必须用 `seed_text`。
