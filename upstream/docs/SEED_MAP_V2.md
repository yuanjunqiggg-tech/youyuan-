# 种子查询 v2 · 开发文档

> 2026-10-26 第 28 条需求交付
> 全部计算与渲染在**本机**完成（APK / Linux 客户端 / WebView），不打云服务器。
> 一份实现，三个出口：**HTTP API**、**插件 SDK**、**MCP 工具**。

---

## 1. 这是什么

玩家在工具箱里点「种子查询」，选好种子和版本，就能看到：

- **地形图**：按生物群系上色的地图瓦片，可拖动、滚轮缩放、双指缩放
- **结构点**：村庄 / 林地府邸 / 海底神殿 / 要塞 / 远古城市 / 末地城 / 末地船 / 下界要塞 / 堡垒遗迹 等 28 种，点一下能看到坐标和距离
- **史莱姆区块**：Java 版逐区块精确判定
- **坐标查询**：点地图任意位置，给出方块坐标、区块坐标、生物群系

同一套能力，机器人能用来回答玩家问题，插件能用来做二次开发，MCP 客户端能当工具调。

---

## 2. 架构

```
                 ┌──────────────────────────────────────────┐
   浏览器 / WebView │  youyuan_seedmap_v2.js（瓦片拼图 + 图标层）│
                 └───────────────┬──────────────────────────┘
                                 │ GET /api/seed/v2/*
                 ┌───────────────▼──────────────────────────┐
   插件 SDK       │  youyuan_seedv2_routes.go（HTTP 端点）    │
   seed.*   ──────┤  youyuan_seedv2_sdk.go（插件分派）        │
   MCP api_seed_* │                                          │
                 └───────────────┬──────────────────────────┘
                                 │
        ┌────────────────────────┼─────────────────────────┐
        ▼                        ▼                         ▼
  seedv2_place.go         seedv2_biome.go            seedv2_render.go
  结构放置/要塞/史莱姆      climate 六轴 + btree 查表    PNG 瓦片 + 图标栅格化
        │                        │                         │
        └──────────┬─────────────┴──────────┬──────────────┘
                   ▼                        ▼
           seedv2_rand.go            seedv2_btree.go
      LCG / Xoroshiro / Perlin /     seedv2_btree.dat（go:embed）
      DoublePerlin / Voronoi         5 张生物群系参数表
```

**关键设计**：

- **不联网**。种子查询不请求任何外部服务，也不上传种子。这是用户明确的隐私与成本要求。
- **无状态**。没有包级可变状态，并发请求安全。
- **单一实现**。HTTP / 插件 / MCP 三个出口调的是同一批 Go 函数，不存在「三个地方三套算法」。
- **可截断不可崩**。范围过大一律截断 + `truncated: true`，不报 500。

---

## 3. 坐标约定（最容易搞混，先看这个）

| 概念 | 说明 | 换算 |
|---|---|---|
| **区块坐标** | API 里所有 `cx` / `cz` / `x` / `z` | — |
| **方块坐标** | 玩家在 F3 里看到的 | `block = chunk * 16 + 8`（区块中心） |
| **生物群系采样** | 引擎内部 1:4 格（voronoi） | 内部已处理，**调用方不要手乘 4** |

> 这里踩过一个致命的坑：早前版本把区块坐标直接当生物群系采样坐标用，导致地形整体偏 4 倍——地图看着还是「像地图」，只有把结构图标叠上去才暴露「神殿在陆地上」。v2 已经在 `BiomeAtBlock` 内部统一处理，调用方只需要给区块坐标。

---

## 4. HTTP API

### 4.1 公共参数

| 参数 | 取值 | 默认 |
|---|---|---|
| `seed` | 十进制整数，支持负数 | 必填（manifest/meta 除外） |
| `version` | `1.21.4` / `26.50` | `1.21.4` |
| `edition` | `java` / `bedrock` | `java` |
| `dim` | `overworld` / `nether` / `end` | `overworld` |
| `large` | `1` 大型生物群系 | `0` |

维度也接受中文（`主世界`/`下界`/`末地`）和数字（`0`/`1`/`2`）。

### 4.2 端点一览

| 端点 | 用途 | 典型耗时 |
|---|---|---|
| `GET /api/seed/v2/manifest` | 版本表 + 维度 + 结构清单 + 引擎自检 | <1ms |
| `GET /api/seed/v2/meta` | 某版本可用的结构 | <1ms |
| `GET /api/seed/v2/tile.png` | 地图瓦片（PNG） | 20~150ms |
| `GET /api/seed/v2/structures` | 结构点列表 | 5~200ms |
| `GET /api/seed/v2/biome` | 按点查生物群系 | <1ms/点 |
| `GET /api/seed/v2/slime` | 史莱姆区块 | 5~80ms |
| `GET /api/seed/v2/stronghold` | 要塞（128 个） | <5ms |
| `GET /api/seed/v2/summary` | 首屏摘要 + 最近结构 | 10~80ms |

### 4.3 示例

**找村庄 + 府邸：**

```bash
curl "http://127.0.0.1:19429/api/seed/v2/structures?seed=12345&version=1.21.4&cx=0&cz=0&r=512&types=village,mansion"
```

```json
{
  "ok": true, "seed": 12345, "version_code": 102104, "edition": "java", "dim": "overworld",
  "range": {"cx0": -512, "cz0": -512, "cx1": 512, "cz1": 512},
  "structures": {
    "village": [{"x": -38, "z": 12, "zombie": false}],
    "woodlandMansion": [{"x": -420, "z": 96}]
  },
  "counts": {"village": 214, "woodlandMansion": 2},
  "total": 216, "truncated": false, "elapsed_ms": 12
}
```

**取一张地图瓦片：**

```bash
curl -o tile.png "http://127.0.0.1:19429/api/seed/v2/tile.png?seed=12345&version=1.21.4&cx=-128&cz=-128&w=256&h=256&bpp=4"
```

`cx/cz` 是瓦片左上角的**区块**坐标，`bpp` 是每像素多少方块（1/2/4/8/16/32）。
一张 256×256、bpp=4 的瓦片覆盖 `256*4/16 = 64` 个区块。

**查某点的生物群系：**

```bash
curl "http://127.0.0.1:19429/api/seed/v2/biome?seed=0&cx=0&cz=0"
```

```json
{"ok":true,"points":[{"x":0,"z":0,"biome":1,"key":"minecraft:plains","name":"平原",
 "color":"#8db360","block_x":8,"block_z":8}],"count":1}
```

批量：`?x=0&x=16&x=32&z=0&z=16&z=32`（x 与 z 数量必须一致，单次上限 4096 点）。

**史莱姆区块：**

```bash
curl "http://127.0.0.1:19429/api/seed/v2/slime?seed=0&cx=0&cz=0&r=16"
```

```json
{"ok":true,"seed":0,"range":{"cx0":-16,"cz0":-16,"cx1":16,"cz1":16},
 "chunks":[{"x":-3,"z":7}],"count":231,"precision":"exact","elapsed_ms":6}
```

**要塞：**

```bash
curl "http://127.0.0.1:19429/api/seed/v2/stronghold?seed=0&version=1.21.4"
```

### 4.4 上限（写死在代码里，不会因为传大值就失控）

| 项 | 上限 | 超了会怎样 |
|---|---|---|
| `r`（结构扫描半径） | 2048 区块 | 夹到 2048 |
| `types`（结构种数） | 24 | 多余丢弃 |
| `limit`（结构条数） | 20000 | 夹住，返回 `truncated: true` |
| `w` / `h`（瓦片尺寸） | 512 | 夹住 |
| `r`（史莱姆半径） | 256 区块 | 夹住 |
| `points`（生物群系点数） | 4096 | 400 报错 |
| URL query 长度 | 2048 字节 | 400 报错 |

### 4.5 错误约定

| 情况 | HTTP | body |
|---|---|---|
| 参数非法 | 400 | `{"ok":false,"error":"seed 必须是整数"}` |
| 引擎未装载（btree 表缺失） | 503 | `{"ok":false,"error":"渲染引擎尚未就绪"}` |
| 范围过大 | 200 | 正常返回 + `truncated: true` |

**任何输入都不许 panic、不许 500。**

---

## 5. 插件接口

插件里直接 `api.call`：

```lua
-- 找当前种子附近的村庄（.通 之类的指令插件可以直接这么用）
local r = api.call("seed.structures", {
  seed = 12345, version = "1.21.4", dim = "overworld",
  types = "village,mansion", cx = 0, cz = 0, r = 512,
})
for _, v in ipairs(r.structures.village) do
  game.sendCommand(string.format("tp @s %d 64 %d", v.x * 16 + 8, v.z * 16 + 8))
end
```

| method | 参数 | 返回 |
|---|---|---|
| `seed.manifest` | 无 | 版本表 / 维度 / 结构清单 / 引擎自检 |
| `seed.structures` | `{seed, version, edition, dim, types, cx, cz, r, limit}` | 按类型分组的结构点（区块坐标） |
| `seed.biome` | `{seed, version, cx, cz}` 或 `{seed, version, points=[[x,z],...]}` | 生物群系 id/key/中文名/RGB |
| `seed.slime` | `{seed, cx, cz, r}` | 史莱姆区块列表 |
| `seed.stronghold` | `{seed, version, count}` | 要塞区块坐标 |
| `seed.summary` | `{seed, version, dim, cx, cz, r}` | 首屏摘要 + 最近结构 |

> 插件不需要机器人连接，不消耗账号，也不联网。`seed.*` 是纯计算。

---

## 6. MCP

`yyAPIRoutes` 里登记过的每个路径都会自动成为一个 MCP 工具。种子查询 v2 的工具名：

| MCP 工具 | 对应端点 |
|---|---|
| `api_seed_v2_manifest` | `/api/seed/v2/manifest` |
| `api_seed_v2_meta` | `/api/seed/v2/meta` |
| `api_seed_v2_tile` | `/api/seed/v2/tile.png` |
| `api_seed_v2_structures` | `/api/seed/v2/structures` |
| `api_seed_v2_biome` | `/api/seed/v2/biome` |
| `api_seed_v2_slime` | `/api/seed/v2/slime` |
| `api_seed_v2_stronghold` | `/api/seed/v2/stronghold` |
| `api_seed_v2_summary` | `/api/seed/v2/summary` |

调用方式与其他 API 工具一致：

```json
{"name":"api_seed_v2_structures",
 "arguments":{"method":"GET","query":{"seed":12345,"cx":0,"cz":0,"r":256,"types":"village"}}}
```

只读连接（`readOnly`）也能调——这些路径都在 `yyMCPReadPaths` 白名单里。

---

## 7. 精度分级（不把近似值说成精确值）

| 级别 | 含义 | 哪些结构 |
|---|---|---|
| `exact` | 算法与游戏原版逐位一致，可用 | 村庄、府邸、神殿、要塞、远古城市、末地城、下界要塞、堡垒遗迹、沉船、海底废墟、埋藏宝藏、前哨、女巫小屋、雪屋、古迹废墟、试炼密室（位置层）、史莱姆区块 |
| `candidate` | 位置公式对，但缺生物群系过滤，需进游戏确认 | 试炼密室/古迹废墟的最终判定、沙漠水井、废弃营地、出生点 |

历史验证数据（来自参考实现，可直接复查）：**274 组黄金基准完全匹配**；±1024 区块范围内 **24 种结构召回 100%、漏报 0**。

### 生物群系：对 chunkbase 地表图 99.98%

用 chunkbase 引擎导出的黄金基准（`D:/种子查询器自研/docs/algorithms/golden/golden-biomes.json`，
12 case × 4 瓦片 × 1024 像素 = 49152 个点）逐点对拍：

| 方案 | 一致率 | 地表图里的洞穴群系像素 |
|---|---|---|
| 参考实现原样（depth = 1 - 83/160 + off） | 75.00% | 大量（繁茂洞穴/深暗之域铺满） |
| **本实现（地表查询把 depth 钉到参数表地表点 1.0）** | **99.98%**（49140/49152，错 12 点） | **0** |

**根因**：参数表里洞穴群系的 depth 盒子是 `span(0.2, 0.9)`，地表群系是 `point(0.0)` 和
`point(1.0)` 两片叶子。查表算法是「到参数盒的最近平方距离」，所以 depth 落在
0.2~0.9 中间时，洞穴群系离得更近，直接把地表抢走。参考实现那个
`1 - 83/160 + off ≈ 0.48` 正好落在洞穴带中间——这就是它对 chunkbase 只有 75%
的真正原因（不是「chunkbase 有私有近似」）。

**顺带否掉的方案**：用该列真实地表高度当 y（76.70%）、把 depth 钉到 0.0（99.97%）。
说明 chunkbase 的地表图本来就不是按地形高度采样的，别再往那个方向改。

回归保护：`TestSeedV2BiomeGoldenTile`（断言 ≥99.5% 且洞穴像素为 0）+
`TestSeedV2BiomeGolden`（断言除洞穴修正外与参考实现逐点一致）。

---

## 8. 算法来源与法律边界

- **结构放置**（region 公式、salt/spacing/separation、Java LCG）是**通用世界生成知识**，来自公开的 cubiomes 与 Minecraft Wiki，可自由使用。
- **生物群系**用 cubiomes 等价的 climate 噪声 + 参数表实现，全部原创代码。
- **不使用** chunkbase 的 WASM 引擎与图标素材。结构图标是自绘矢量图形。

---

## 9. 排错

| 现象 | 先查 |
|---|---|
| 地图全黑/全白 | `GET /api/seed/v2/manifest` 看 `btree_tables` 是否为空 → 空说明 `.dat` 没装进二进制 |
| 结构图标位置对、地形颜色错 | 坐标换算被改了，检查 `yyV2RenderTilePNG` 里的像素→方块公式 |
| 府邸一个都搜不到 | `largeSpread`（三角分布）丢了 —— 府邸/神殿/末地城是三角分布，不是线性 |
| 下界要塞和堡垒重叠 | `separation` 写成 23 了，应为 4 |
| 版本切换后结果不变 | 版本码解析，注意 Bedrock 与 Java 用同一个公式但历史上漂过 |
| 面板空白 | `/api/seed/v2/*` 是否 404；`yySeedV2Routes(mux)` 有没有被调用 |

---

## 10. 与旧版（`/api/seed/*`）的关系

旧版是 10-24 那批近似实现（结构只给区域候选、要塞是三环近似、生物群系是手写规则表）。
v2 落地后旧版保留端点不变，**不再演进**。新代码、前端、插件、MCP 一律走 v2。
