# 种子查询接口速查

> 三个命名空间并存，**新代码一律用 `/api/seed/v2/*`**（`youyuan_seedv2_routes.go:1-13` 的路由注释已明确：旧版保留兼容，不再演进）。
> 全部为 `GET`、纯本机计算、无副作用、不联网、不读写用户文件；输入非法才 400，绝不 500（`yySeedGuard` 兜 panic）。

---

## 0. 三条坐标系约定（最容易搞错）

| 接口 | `x/z` / `cx/cz` 的含义 | 换算 |
|---|---|---|
| `/api/seed/v2/*` 的 `cx/cz`、`structures` 返回的 `x/z`、`biome` 返回的 `x/z` | **区块坐标** | 方块 = 区块 × 16 + 8（区块中心，`youyuan_seedmap_v2.js:107`） |
| `/api/seed/v2/biome` 返回的 `block_x/block_z` | 方块坐标 | 已替你算好 |
| `/api/seed/*`（旧）的 `x/z`、`from_x/to_x` | **方块坐标**（biome 的批量跨度也是方块） | 区块 = floor(方块/16) |
| `/api/youyuan/world/seed` 的 `from_x/to_x` | **区块坐标**（文档注释写明） | — |

v1 与 v2 对同一个「x」的理解不同，**这是历史遗留，不要在 v2 里混用 v1 的坐标**。

错误格式统一：

```json
{"ok": false, "error": "缺少 seed 参数"}
```

---

## 1. `/api/seed/v2/*`（推荐）

公共参数（`yySeedV2ReadQuery`，`youyuan_seedv2_routes.go:146-173`）：

| 参数 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `seed` | int64 | 必填（manifest/meta 可省） | 十进制，可负 |
| `edition` | `java` / `bedrock` | `java` | 归一化处理 |
| `dim` | `overworld` / `nether` / `end` | `overworld` | 注意是 `end`，不是 `the_end` |
| `version` | 字符串 | `1.21.4` | 见 manifest 的版本表；长度 ≤16 |
| `large` | `1/true/yes/on` | false | 巨型生物群系 |

### 1.1 `GET /api/seed/v2/manifest`

能力清单。前端首屏调用（`js:589`）。

返回要点：

```json
{
  "ok": true,
  "engine": "seedv2",
  "btree_tables": ["btree18", "..."],
  "biome_engine": true,
  "render_engine": true,
  "editions": [
    {"id":"java","name":"Java 版","versions":[{"label":"1.21.4","version":"1.21.4","code":102104}]},
    {"id":"bedrock","name":"基岩版","versions":[...]}
  ],
  "dimensions": [{"id":"overworld","name":"主世界"}, ...],
  "structures": [
    {"id":"village","name":"村庄","dim":"overworld","precision":"exact",
     "placement":"region","color":"#ff3c3c","default":true,
     "java":[100700,260400],"bedrock":[101400,267000]}
  ],
  "note": "全部计算在本机完成，不联网、不上传种子"
}
```

版本表：Java 22 项（含 26.x 实验），基岩 12 项（`youyuan_seedv2_routes.go:32-52`）。
结构清单按 **Java 1.21.4** 给全量——所以切到更新版本时，可能出现 manifest 里没有的结构（前端 `S.defs` 缺失 → 灰色无名图标）。

### 1.2 `GET /api/seed/v2/meta?version=&edition=`

某版本可用的结构 + 维度。

返回：`{ok, version, version_code, edition, structures:[...], dimensions:[...]}`

⚠️ **不返回 `unsupported`**（`youyuan_seedv2_routes.go:85-89`），但前端 `readMeta()`（`js:2263-2283`）在等这个字段 → 置灰逻辑是死代码。要么后端补，要么前端删。

### 1.3 `GET /api/seed/v2/structures`

批量扫结构。

| 参数 | 默认 | 上限 | 说明 |
|---|---|---|---|
| `seed` | 必填 | — | — |
| `version` / `edition` / `dim` | 同上 | — | — |
| `types` | 该维度默认集 | 24 个 | 逗号分隔，非法/不属于该维度的会被静默过滤 |
| `cx` / `cz` | 0 | — | 中心区块 |
| `r` | 256 | **2048** | 扫描半径（区块），即方块范围 ±32768 |
| `limit` | 20000 | 20000 | 总点数上限，超出 `truncated=true` |

返回：

```json
{
  "ok": true, "seed": 0,
  "version": "1.21.4", "version_code": 102104, "edition": "java", "dim": "overworld",
  "range": {"cx0": -256, "cz0": -256, "cx1": 256, "cz1": 256},
  "structures": {
    "village": [{"x": -120, "z": 340, "type": "village", "variant": "...", "zombie": true}],
    "endCity": [{"x": 12, "z": -8, "type": "endCity", "has_ship": true}]
  },
  "counts": {"village": 37},
  "total": 37,
  "truncated": false,
  "types": ["village", "..."],
  "elapsed_ms": 12
}
```

- `x/z` 是**区块**；前端转 `bx = x*16+8`
- `variant`：村庄/废墟等变体；`zombie`：僵尸村；`has_ship`：末地城带船
- `types` 是**实际参与扫描**的列表（已被版本/维度过滤），`structures` 里没有键 = 该类型 0 个
- `truncated=true` 只表示触到 `limit`，不代表范围外没有

### 1.4 `GET /api/seed/v2/biome`

三种形态：

1. **单点/多点**：`x=1&z=2&x=3&z=4`（区块坐标，成对出现，最多 **4096** 对）
2. **网格**：`cx=&cz=&r=&step=`，`r` 1..512，`step` 1..64，`n*n ≤ 4096`
3. **下界/末地**：不按气候噪声算，返回该维度代表群系 + `note`（`yySeedV2BiomeSimple`，`routes.go:526-541`）

单点返回：

```json
{"ok":true,"points":[{
  "x":12,"z":-3,"biome":1,"key":"minecraft:plains","name":"平原",
  "color":"#8db360","block_x":200,"block_z":-40
}],"count":1}
```

网格额外返回 `range:{cx,cz,r,step}`。

### 1.5 `GET /api/seed/v2/slime`

| 参数 | 默认 | 上限 | 说明 |
|---|---|---|---|
| `seed` | 必填 | — | — |
| `cx` / `cz` | 0 | — | 中心区块 |
| `r` | 16 | **256** | 半径（区块） |

**只支持 Java**；基岩版直接 400（`routes.go:549-552`）。

返回：`{ok, seed, range:{cx0,cz0,cx1,cz1}, chunks:[{x,z}], count, precision:"exact", elapsed_ms}`

### 1.6 `GET /api/seed/v2/stronghold`

| 参数 | 默认 | 上限 |
|---|---|---|
| `seed` | 必填 | — |
| `count` | 128 | 128 |

**只支持 Java**；基岩版 501（`routes.go:597-602`）。

返回：`{ok, seed, strongholds:[{x,z}], count, precision:"ring_approx", elapsed_ms, note}`

`note` 原文：环位候选，原版还会在允许群系里就近回拉几十格，落地后附近 1~2 区块内找。
⚠️ manifest 里 `stronghold` 仍标 `precision:"exact"`（`seedv2_place.go:315`），与这里矛盾，UI 不要照 manifest 写「精确」。

### 1.7 `GET /api/seed/v2/summary`

| 参数 | 默认 | 范围 |
|---|---|---|
| `cx` / `cz` | 0 | — |
| `r` | 256 | 16..512 |

返回：`{ok, dim, radius, terrain, biome_top:[{name,share}], nearest:[...], seed, version, edition, elapsed_ms}`

**当前没有任何前端调用**（v2/v1/Canvas 都没有）。可给 MCP/AI 或下一版「一句话概览」用——注意它是 8×8 采样估算，不是精确统计。

### 1.8 `GET /api/seed/v2/tile.png`

| 参数 | 默认 | 说明 |
|---|---|---|
| `seed` | 必填 | — |
| `cx` / `cz` | 0 | 瓦片左上角区块 |
| `w` / `h` | 256 | 32..512 |
| `bpp` | 4 | 方块/像素，就近吸附到 1/2/4/8/16/32 |
| `layer` | 空 | `terrain`（只地形）或 `structures`（烘图标）；空=两者 |
| `types` | 默认集 | `layer=structures` 时画哪些图标 |
| `maxicons` | 320 | 0..2000 |

返回 `image/png`，`Cache-Control: public, max-age=86400`，`X-Seed-Engine: seedv2`。
引擎未就绪/无表 → **503**（前端要能降级，不要当 404）。

> 前端固定传 `layer=terrain`（`js:437-449`），图标交给 DOM 层，避免双图标和拼图切边。

### 1.9 `GET /api/seed/v2/icon.svg?id=&size=`

- `id`：结构 id（未知 → 400）
- `size`：8..128，默认 20
- 返回 `image/svg+xml`，`max-age=604800`
- 同一份几何定义也给 PNG 烘图标用，保证「地图上的图标」和「前端叠加的图标」一致

---

## 2. `/api/seed/*`（旧版，仅兼容）

> 前端 `youyuan_seed_page.js`（v1）与 `features/seed_map.js`（Canvas）仍在用；**不要再新增调用**。

| 路径 | 关键参数 | 返回要点 | 精度 |
|---|---|---|---|
| `/api/seed/manifest` | — | 版本矩阵 + 能力 | 旧清单 |
| `/api/seed/slime` | `seed`、`x`、`z`（区块）、`radius`（默认 8，>64 夹到 64）、`edition` | `chunks`、`blocks`、`count` | exact（仅 Java） |
| `/api/seed/stronghold` | `seed`、`count`（默认 3） | `points` | `approximate`，三环 1408/4352/7424，无群系过滤 |
| `/api/seed/structure` | `seed`、`type`、`dimension`、`edition`、`from_x/from_z/to_x/to_z`（区块，跨度 ≤4096）、`lod` | 结构候选 | 按类型 |
| `/api/seed/query` | `seed`、`x`、`z`、`radius`（≤32）、`lod`、`dimension` | 附近要塞 + 史莱姆数 + 结构候选 + 中文 `summary` | 综合，150ms 预算 |
| `/api/seed/heat` | `seed`、范围、`lod` | 结构概率热力 | 概率 |
| `/api/seed/coordinates` | `seed,x,z` 编码；`code` 解码 | 分享码（CRC32） | — |
| `/api/seed/biome` | 单点 `x,z`（**方块**）；批量 `from_x/from_z/to_x/to_z`（**方块**，单轴 ≤4096，`step` 默认 16）；`dimension`（`overworld/nether/the_end`） | `biome/biome_id/name/precision/algorithm` | 1.18+ 精确 / 旧版近似 / 基岩标注 `applies_to_edition:"java"` |

旧 biome 的维度值是 `the_end`，v2 是 `end`——**不能直接把参数搬过去**。

---

## 3. `/api/youyuan/world/seed`

用途：读取**当前连接世界**的真实种子（来自 StartGame 包），或按显式种子做 Java 结构试算。

| 参数 | 说明 |
|---|---|
| `seed` | 可选，显式指定；优先于内核读到的值 |
| `structure` / `type` | 可选：`village` / `fortress` / `bastion` / `monument` / `mansion` / `temple` |
| `edition` | 只有 `java` 才有意义；**不给就报错** |
| `from_x/from_z/to_x/to_z` | 区块，默认 -256..256，单轴跨度 ≤4096 |
| `lod` | 0/1/2，默认 0 |

返回要点：

- 无 `structure`：种子快照（`seed`、`seed_text`、`seed_zero_hint`、来源 `kernel|query`、`note`）
- 未连接且未传 seed：`ok:false` + 中文提示（HTTP 200，不是 500）
- 传 `structure` 但 `edition!=java`：400，说明基岩算法不适用
- 传 `structure` 且种子未知：409

⚠️ `seed=0` 可能是服务端把真实种子替换成 0，**不要当真实种子**（返回里有 `seed_zero_hint`）。

---

## 4. 前端调用映射（排查「页面为什么没数据」）

| 前端文件 | 调用 | 备注 |
|---|---|---|
| `youyuan_seedmap_v2.js:589` | `/api/seed/v2/manifest` | 首屏；失败 → 空态 + 重试 |
| `js:2265` | `/api/seed/v2/meta` | 只为 `unsupported`（死代码，见 §1.2） |
| `js:1543` | `/api/seed/v2/structures` | `r=dataRadius()`，`limit=20000` |
| `js:1721` | `/api/seed/v2/slime` | 仅主世界且开关打开 |
| `js:1794` | `/api/seed/v2/stronghold` | 点「列出全部要塞」 |
| `js:1967` | `/api/seed/v2/biome` | 点图标/地图后查单点 |
| `js:437/993/1656` | `tile.png` / `icon.svg` | 瓦片与图标 |
| `youyuan_seed_page.js` | `/api/seed/*` | 旧页 |
| `features/seed_map.js:1162` 等 | `/api/seed/manifest`、`/api/seed/structure`、`/api/youyuan/world/seed` | Canvas 版，带降级 |

---

## 5. MCP / 只读连接

`upstream/youyuan_mcp_links.go:27-49` 已把以下路径加入只读白名单：

```
/api/seed/manifest /slime /stronghold /structure /query /heat /coordinates /biome
/api/seed/v2/manifest /icon.svg /meta /tile.png /structures /biome /slime /stronghold /summary
/api/youyuan/world/seed
```

规则：

- 只读连接只能 GET；POST 一律 403
- 这几个接口都是本机计算，**不要用 MCP 做批量/高频轮询**（注释原文：「批量/高频计算请在本机做」）
- 新增种子接口必须同步 `yyMCPReadPaths`，否则只读连接看不到

---

## 6. 上限与预算速查

| 项 | 值 | 出处 |
|---|---|---|
| 结构扫描半径 | 2048 区块 | `yySeedV2MaxRadius` |
| 单次结构类型数 | 24 | `yySeedV2MaxTypes` |
| 结构点总数 | 20000 | `yySeedV2MaxLimit` |
| biome 单次点数 | 4096 | `yySeedV2MaxPoints` |
| biome 网格 r / step | 512 / 64 | handler |
| 史莱姆半径 | 256 区块 | `yySeedV2MaxSlimeRadius` |
| 瓦片边长 | 32..512 px | handler |
| 图标尺寸 | 8..128 px | handler |
| 要塞数量 | 1..128 | handler |
| summary 半径 | 16..512 | handler |
| 旧结构跨度 | 4096 区块/轴 | `yySeedMaxChunkRange` |
| 旧 biome 跨度 | 4096 方块/轴，4096 点 | `yyBiomeMaxSpan` / `yyBiomeMaxPoints` |

---

## 7. 已知坑（接口侧）

1. **`/meta` 缺 `unsupported`** → 前端置灰逻辑失效。
2. **manifest 的 `stronghold.precision` 是 `exact`**，handler 返回 `ring_approx`；以 handler 为准。
3. **manifest 只按 Java 1.21.4 生成结构清单**，切新版本可能缺 defs。
4. **`/summary` 没有前端调用**，别以为页面上的「概览」是它算的（那是 `renderSummary()` 用结构结果自己拼的）。
5. **v1/v2 坐标语义不同**（方块 vs 区块），v1 维度 `the_end` vs v2 `end`。
6. **`tile.png` 503 是正常降级信号**（引擎未就绪），不是路径错。
7. **`limit` 在 HTTP/SDK/AI 三个出口默认值不一致**（20000/4000/2000），跨入口对比数字会对不上（详见 `outputs/review-seed/报告.md` P1-8）。
