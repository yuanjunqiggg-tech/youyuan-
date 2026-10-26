# 网易我的世界「组件码 → 皮肤」接口调研 + 验证链路设计

- 调研日期：2026-10-26
- 调研范围：只看「组件码（skin_item_id / item_id）怎么变成皮肤资源、怎么穿到自己身上、进服后皮肤走哪个包哪个字段」，不写业务代码
- 纪律声明：本文所有接口均来自**实际拉取的源码或本机 vendored 源码**，URL 均用 `api.github.com` / `raw.githubusercontent.com` 实际请求过（拉不到的直接写「拉不到」，不补全）。凡属推断，均标 `【推断】`；凡未实测，均标 `【未实测】`。

---

## 〇、TL;DR（三个必须知道的结论）

1. **「官方公开接口：无」。** 网易没有发布任何「组件码 → 皮肤图片」的公开 API、SDK 或文档。GitHub 上所有能用的实现都是逆向**网易启动器（WPFLauncher / x19）私有网关**得到的。
2. **「启动器私有接口：有，且能直接用」。** `POST {ApiGatewayUrl}/pe-download-item/get-download-info`，请求 `{"item_id":"<组件码>"}`，头带 `user-id` + 动态 `user-token`，返回 `{"entity":{"entity_id":…,"res_url":"https://x19.gph.netease.com/…"}}`。`res_url` 指向组件资源包（现有实现按 zip 处理、取包内 `skin.png`；也可能是裸 PNG，`【未实测】`）。**这就是「组件码取皮肤」的唯一现实路径**，本机 `upstream/youyuan_skin_item.go` 已按此实现。
3. **「进游戏抓包」不是取皮肤的必经之路，只是验证手段。** 抓包能拿到的只有两种东西：像素（`skin.skin_data`）或组件码（`sync_skin.skin_item_ids` / `skin.cape_id`）。**拿到码之后仍然要回到第 2 条那个接口去换资源**。所以「先派机器人进服抓一次」可以用来确认组件是否真的生效、以及给没有凭据的场景留下证据，但它替代不了 API。

> 一句话给主控：**接口是有的（私有），但不在游戏协议里；游戏协议只给码，码要回私有网关换包。**

---

## 一、能力矩阵：每个接口到底能干什么

基地址：`ReleaseJSON.ApiGatewayUrl`（登录快照里带，缺省回退 `https://g79apigatewayobt.minecraft.cn`）。
鉴权：全部 `POST` + `Content-Type: application/json; charset=utf-8` + `User-Agent: WPFLauncher/0.0.0.0` + `user-id: <UserID>` + `user-token: CalculateDynamicToken(path, body, UserToken)`。无凭据时统一返回 `{"code":10,"message":"请先登录"}`（本机 2026-10-26 实测复现）。

| 能力 | 接口 | 入参 | 出参关键字段 | 证据 | 备注 |
| --- | --- | --- | --- | --- | --- |
| **组件码 → 资源包地址** | `POST /pe-download-item/get-download-info` | `{"item_id":"<纯数字>"}` | `entity.res_url`、`entity.entity_id` | `UCKETX/g79client/download_get_info.go`；本机 `third_party/funauth/modules/g79client/download_get_info.go`；本机复刻 `upstream/youyuan_skin_item.go:344` | **核心接口**。返回 zip 或裸 PNG，不含 PNG 直链 |
| 读账号当前皮肤 | `POST /pe-get-user-setting-list` | `{"settings":["skin_type","skin_data",…]}` | `entity.skin_data.item_id`、`is_slim` | `user_settings_get.go` | 只给**当前选中**的码，不给「已拥有列表」 |
| 设账号当前皮肤 | `POST /pe-set-user-setting-list` | `{"data":{"skin_data":{"item_id":"<码>"}}}` | `code/message` | `user_settings_set.go`；本机 `auth-service/engine.go:191` | **只选择，不购买**。码不属于该账号时失败【未实测具体错误码】 |
| 查**任意** user_id 的当前皮肤 | `POST /pe-get-other-user-setting-list` | `{"user_id":"<数字>","with_pet_info":false,"settings":["skin_data",…]}` | `entity.skin_data.item_id`、`is_char_pack`、`is_slim` | 本机 `third_party/funauth/modules/g79client/user_other_settings_get.go:161`（**公开镜像 UCKETX 无此文件，属较新版本**） | 这是「查别人穿什么」的正路；本机尚未接入 |
| 查组件详情（名称/简介） | `POST /item-buy-list/query/search-mcgame-items` | `{"item_ids":["…"]}` | `entities[].name/brief/behaviour_uuid` | `store_search_mcgame_items.go` | 必须先有码，不能当商城浏览用 |
| 购买组件 | `POST /user-item-purchase`（走 `DCWebURL`） | item_id 等 | 扣费结果 | `user_item_purchase.go` | **禁止接入**：真实扣费 |
| 一键换肤（含购买） | `ChangeSkin(itemID)` | 码 | — | `skin_change.go` | 内部先 `PurchaseItem`，`code==502` 被放行（`【推断】`= 已拥有）；**不要照抄**，会把「没买」变成「买了」 |
| 读账号当前头像/头像框 | `POST /pe-user-detail/get`（走 CoreServerURL） | `{}` | `head_image`、`avatar_image_url`、`frame_id` | `user_get_detail.go` | 与皮肤组件是两套东西 |

**关键边界**：
- 没有任何接口返回「这个账号拥有哪些皮肤组件」，也没有「按组件码直接返回 PNG 字节」的接口。
- `res_url` 的路径**没有可复用的拼接规律**（不是 `item_<id>.png` 这种公式），它是服务端下发的完整地址。任何「按 id 拼 URL」的做法都是编造。
- 只有 `res_url` 落在 `*.gph.netease.com` / `*.netease.com` 才可下载；本机已有白名单校验（`youyuan_skin_item.go:490`）。

---

## 二、协议层：进服时「皮肤」到底从哪来

### 2.1 三个包里只有一个带网易专属的组件码字段

| 包 | 方向 | 皮肤相关字段 | 说明 |
| --- | --- | --- | --- |
| `StartGame`（11） | S→C | 只有 `custom_skins_disabled` | **不带任何皮肤数据**（`protocol/packet/start_game.go:179-180,312`）。别在这找皮肤 |
| `PlayerList`（ADD） | S→C | `entries[i].skin`（完整 `protocol.Skin`），另有 `skin.trusted` | 常规玩家皮肤主来源；`protocol/packet/player_list.go:54` + `protocol/skin.go` |
| `PlayerSkin`（原版） | 双向 | `uuid` + `skin` | 客户端换肤时发；**网易是否转发存在代码内自相矛盾的注释，见 2.3** |
| `SyncSkin`（240，网易） | S→C | **`skin_item_ids []string`（网易：皮肤组件码）** + 单个 `skin` | `protocol/packet/sync_skin.go:10-22`。**这是协议里唯一显式叫「skin item ids」的字段** |
| `ConfirmSkin`（232，网易） | S→C | `player_data[].uuid`、`uids[]`（launcher uid） | 字段仍属「推测」（`player_info.go:358` 注释原文） |

`SyncSkin` 的结构顺序（`sync_skin.go:26-32`）值得记住，因为它决定了「码」和「像素」是不是一一对应：

```
PlayerData []NeteasePlayerData   // 每条含 unknown1/uuid/unknown2
PlayerUIDs []string              // 长度 = len(PlayerData)
Unknown    []string              // 长度 = len(PlayerData)
SkinItemIDs []string             // 长度 = len(PlayerData)  ← 组件码
Skin       protocol.Skin         // 只有一个皮肤负载，不是数组
```

即：**N 个玩家共享 1 份 Skin 负载，但码是按玩家给的**。这直接引出一个坑（见 2.2）。

### 2.2 现有实现的两个真实缺口（这是「组件皮肤抓不到」的根因）

本机 `player_info.go` 的处理逻辑：

1. `updateSkinInfo`（:255）只从 `entry.Skin` 复制，并要求 `validPlayerSkin`（:264）通过——而该函数硬性要求 `len(SkinData) == w*h*4`。**只有码、没有像素的皮肤会被静默丢弃**。
2. `handleSyncSkin`（:332）只在 `len(p.PlayerData)==1` 且 `validPlayerSkin(p.Skin)` 为真时落库；`p.SkinItemIDs` 虽然被解析、也写进了 `youyuan_debug.log`（:349），但**从未被使用**。

结论：**「组件码」在链路上是断头路**。抓到了也只能看日志，不会变成图，也不会进皮肤库。这正是设计要补的一环（见第四节）。

### 2.3 一个必须实测定论的冲突点（别当已知事实用）

- `flowers-for-machines/client/open_connection.go:75-79` 注释：*「不发送 vanilla PlayerSkin 包：实测网易服务器不会转发它…皮肤分发完全依赖登录 JWT ClientData」*
- `player_info.go:279-281` 注释：*「网易服务器会把玩家的皮肤更新/进服皮肤转发给所有客户端，这是获取已在线玩家皮肤的主要来源」*

两处结论相反。**本文不站队**：验证链路 B 的第一步就是把这件事测死（看 `[皮肤] PlayerSkin` 是否有记录）。

### 2.4 启动器本机缓存（抓不到像素时的最后兜底）

`ZekerZhayard/SkinModCompact`（Java 版加载器插件，GPL-3.0）暴露了两个可复用的硬事实：

- 网易客户端把皮肤落在 **`<安装目录>/cache/skin/skin_<n>.png`**，默认皮肤固定为 `skin_10000.png`（Steve）/ `skin_10001.png`（Alex）——代码里显式排除这两个文件名。
- 启动器提供**本机 IPC**：`LoadSkinReplyV2`（`SMID = 2050`）回调签名 `handler(name, skinPath, capePath, skinModel)`，返回的是**磁盘绝对路径**而不是 URL。

对本项目的意义：如果哪天要「机器人抓一次就永久有图」，`cache/skin/*.png` 是最省事的落盘来源；但它在**客户端机器**上，服务器侧工具箱拿不到，只能作为「本地部署形态」的备选。

---

## 三、GitHub 仓库清单（全部实际请求验证过）

> Star / 推送时间为 2026-10-26 通过 `api.github.com` 读取；许可证以 API 返回的 `license.spdx_id` 为准（`null` = 仓库无许可证文件，默认保留所有权利）。

| 仓库 | Star | 最近推送 | License | 与本任务的关系 | 关键文件 | 可信度 |
| --- | --- | --- | --- | --- | --- | --- |
| [UCKETX/g79client](https://github.com/UCKETX/g79client) | 6 | 2026-01-04 | **AGPL-3.0** | **公开可得的 g79 客户端最全镜像**（125 个文件）。组件码→res_url、动态 token、读写皮肤设置、查他人设置（旧版）、买/换肤都在 | `download_get_info.go`、`crypto.go:170`、`user_settings_get.go`、`user_settings_set.go`、`skin_change.go`、`store_search_mcgame_items.go` | **高**（逐文件 raw 拉取核对，与本机 vendored 一致） |
| [yeah-god/FunAuth](https://github.com/yeah-god/FunAuth) | 19 | 2026-06-06 | **无**（`null`） | 网易认证服务端实现；`modules/g79client` 是 **git submodule**，指向 **`Yeah114/g79client`（私有，API 返回 404）**，所以 GitHub 上直接点不开任何接口文件 | 只能看 `auth/`、`cmd/`、`internal/`；接口实现不可见 | 中（结构可信，文件不可得；本机 vendored 副本才是可读版本） |
| [Mono2023-PRC/FunAuth](https://github.com/Mono2023-PRC/FunAuth) | 3 | 2025-10-01 | 无 | 上游原版，yeah-god 为其 fork | 同上（同样 submodule 化） | 中 |
| [RTHLanStar/FunAuth](https://github.com/RTHLanStar/FunAuth) | 1 | 2026-10-11 | 无 | 同期 fork，可作第三方交叉参考 | 同上 | 低-中 |
| [ZekerZhayard/SkinModCompact](https://github.com/ZekerZhayard/SkinModCompact) | 2 | 2020-03-06 | **GPL-3.0** | **唯一给出「启动器本机皮肤缓存路径 + IPC 协议」的公开实现**：`SMID 2050`、`cache/skin/skin_*.png`、skinModel 0=宽臂/1=细臂 | `netease/NeteaseAPILoader.java`、`netease/reply/LoadSkinReplyV2.java`、`netease/reply/SocketHandler.java` | 中高（代码老，但字段/路径是硬事实） |
| [KawaiiNekoChanProject/LiquidBounce-Addon-NetEaseSkin](https://github.com/KawaiiNekoChanProject/LiquidBounce-Addon-NetEaseSkin) | 0 | 2026-10-22 | **GPL-3.0** | 最新（4 天前）的「Java 版显示网易皮肤」实现。用 `WPLauncherCookieRaw(cookie)` 登录 → `session.getSkinFromUUID(uuid)` → `X19Skin{id, url, mode}`，`url` 直接可下载，`id` 用于去重，`mode` 区分宽/细臂。**说明启动器还有「按 UUID 取皮肤」的通道** | `modules/ModuleNetEaseSkin.kt` | 中（调用点可信；依赖库 `net.nekocurit.x19` **既不在仓库内、也未在 gradle 中声明**，无法复核其实现，仓库大概率不能独立构建） |
| [Koud-Wind/Netease-minecraft-LAN-connects-to-Server](https://github.com/Koud-Wind/Netease-minecraft-LAN-connects-to-Server) | 59 | 2026-09-18 | GPL-3.0 | 局域网联机协议，**不涉及皮肤 HTTP 接口** | — | 仅协议参考，本任务不用 |
| [Hazelmeow/neomega-core](https://github.com/Hazelmeow/neomega-core) | 1 | 2024-11-24（已归档） | 无 | 网易协议库，同上不含皮肤接口 | — | 不用 |

**负面结论（同样重要）**：
- `grep.app` 的 API 已被 Vercel 安全校验拦截，`searchcode` 同样不可用；GitHub 代码搜索需登录令牌。**「组件码 → PNG」在公开代码里没有第二个独立实现**，能对上的只有 `get-download-info` 这一条链路。
- 没有找到任何仓库实现「列出账号已拥有的皮肤组件」（与 2026-10-23 那份头像调研结论一致）。

**许可证风险（建议主控确认，不属本次改动范围）**：`UCKETX/g79client` 是 **AGPL-3.0**，`Yeah-god/FunAuth` **无许可证**。本机 `third_party/funauth` 是后者的副本——「无许可证」在法律上等于「保留所有权利」，AGPL 则对网络服务有传染性。这是存量问题，本文只做提示。

---

## 四、可执行验证链路

设计原则：**A 链是主链（不需要游戏在线），B 链是取证链（验证组件是否真的生效）**。两条链共用同一个落盘点，所以 B 抓到的码可以直接喂给 A。

### 4.1 A 链：纯服务端，组件码 → PNG（推荐，当前代码已具备）

| # | 动作 | 具体调用 | 通过判据 | 失败点 | 回退 |
| --- | --- | --- | --- | --- | --- |
| A1 | 选一个有登录态的账号 | 认证服务 `action:"skin_info"`（只读）或 `插件数据文件/皮肤组件凭据.json` | 拿到 `UserID` + `UserToken` + `ReleaseJSON.ApiGatewayUrl` | 无账号 / 未登录平台 / 设备令牌为空（`youyuan_skin_item.go:186-212`） | 退到配置文件；两者都无 → **明确报「缺凭据」**，不拿别的图顶替 |
| A2 | 换组件资源地址 | `POST {ApiGatewayUrl}/pe-download-item/get-download-info` `{"item_id":"<码>"}` | `code==0` 且 `entity.res_url` 非空 | `code=10` 请先登录（会话过期）→ 先 `refresh` 重试一次；`entity==null` → 该码不存在/无权 | 提示具体错误码，3 分钟内负缓存不再打 |
| A3 | 域名白名单 | 校验 `res_url` host 属于 `*.netease.com` / `*.163.com` / `*.gph.netease.com` | 通过 | 非官方域（被劫持/接口改版） | 直接拒绝，不下载 |
| A4 | 下载资源 | GET `res_url`（限 8MB，超时沿用 `yyAccountSkinTimeout`） | HTTP 200 | 403/404（CDN 过期）、超时 | 视为失败，负缓存；换个账号再试 |
| A5 | 解包 | `PK` 开头按 zip 读，挑 `skin.png`（排除 `icon*`/`pack.png`），否则当裸 PNG | 得到 PNG | zip 内无 png / 超大条目 | 失败即拒，不做「猜尺寸重排」 |
| A6 | 尺寸闸 | `yySkinTexturePNG` 校验 64×32 / 64×64 / 128×128… | 宽高合法 | 自定义骨骼（`custom_geometry`）皮肤会被拒 | 已知限制，卡片显示「暂不支持此模型」 |
| A7 | 落盘 | `插件数据文件/皮肤组件缓存/<码>.png` + 同名 `.json`（含 `res_url`/来源/时间） | 文件存在且 >8 字节 | 目录无权限 | 仅内存缓存，下次重取 |
| A8 | 皮肤库展示 | `GET /api/skin/item-texture?item_id=<码>` | `200 image/png`，响应头 `X-Skin-Source: cache/account/config/worn` | 见 `{ok:false,error}` | 卡片显示失败原因（不显示假方块） |

**验收命令（本机有监听时）**：
```bash
curl -sS -D - -o /tmp/skin.png "http://127.0.0.1:<port>/api/skin/item-texture?item_id=8428087" | head -20
file /tmp/skin.png          # 期望 PNG image data, 64 x 64 或 128 x 128
```
`X-Skin-Source` 直接告诉你走了哪条分支：`cache`=命中磁盘；`account`=用账号实时凭据现取；`config`=用配置文件凭据；`worn`=机器人正穿着这一款（复用账号纹理，未打网易）。

### 4.2 B 链：机器人装组件 → 进服 → 抓包 → 落盘（取证 / 补像素）

| # | 动作 | 具体做法 | 抓什么 | 通过判据 | 失败点 | 回退 |
| --- | --- | --- | --- | --- | --- | --- |
| B0 | 让账号「穿上」该组件 | 认证服务 `action:"skin"` + `item_id`（`auth-service/engine.go:185-198`，只调 `SetUserSettingList`，**不购买**） | 返回 `ok:true` | 设置成功 | 码不属于该账号 → 官方拒绝；`reconnect_required:true` | 改用 A1 的 `skin_info` 只读取码，不改账号状态 |
| B1 | 重连进服 | 机器人用同一账号重连（皮肤随登录 JWT `ClientData.SkinIID` 上报，`protocol/login/data.go:167-169`） | `PlayerList ADD` / `PlayerSkin` / `SyncSkin(240)` / `ConfirmSkin(232)` | 收到其中任一且带皮肤信息 | 只有 `[加入]` 没有皮肤日志 | 打开调试日志逐字段看（B3） |
| B2 | 按优先级取字段 | ① `player_list.entries[i].skin.skin_data` + `skin_image_width/height`（有像素，直接落盘） ② `sync_skin.skin_item_ids[0]`（只有码，**转 A 链**） ③ `player_list.entries[i].skin.cape_id`（网易把 CapeID 复用为 item id，见 `dial.go:517-525` 注释） ④ `confirm_skin.uids[]`（launcher uid，`【推断】`可用于 `pe-get-other-user-setting-list`） | — | 拿到像素或拿到纯数字码 | 三者皆空 | 判定「服务器未下发皮肤」，记录并退出 B 链 |
| B3 | 落盘取证 | `youyuan_debug.log`（`task_dispatch.go:184-199`，8MB 轮转）已有 `[皮肤] SyncSkin … skinItemIDs=…` 与 `dumpSkinToDebugLog` 的完整元数据 | 日志 + `插件数据文件/皮肤缓存/<sha1(lower(name))>`（`youyuan_skin_disk_cache.go:54,95`） | 日志里能看到 `skinItemIDs=[<码>]` 或 `skin_data` 长度 | 日志太大/被轮转 | 调小并发、只对目标玩家开日志 |
| B4 | 变成库里的图 | 若 B2 命中①→ 直接写 `皮肤组件缓存/<码>.png`；若命中②③→ 用码走 A2~A7 | `X-Skin-Source` | 卡片出图 | 见 A 链失败点 | — |
| B5 | 展示 | 皮肤库调 `/api/skin/item-texture`，或按玩家名走 `/api/skin/online-texture?player=` | 200 + PNG | 图片正确、宽高/体型对 | 昵称≠玩家名 → 名字链路必然 miss（缓存键是名字 sha1） | 一律以组件码为主键，名字只作展示 |

**这一步的实测顺序建议（成本最低 → 最高）**：
1. 先跑 A 链（不发包、不进服、无副作用），确认「凭据 → 码 → PNG」通。
2. 再跑 B1-B3，只为了回答两个问题：**网易到底发不发像素？`SyncSkin.skin_item_ids` 里到底有没有码？**
3. 最后才决定是否需要把 B 的结果并入库（若 A 链已稳定，B 只作诊断）。

---

## 五、设计建议（最小改动，不动现有成功链路）

1. **补 `handleSyncSkin` 的断头路（收益最高）**
   当 `!validPlayerSkin(p.Skin)` 且 `len(p.SkinItemIDs)>0` 时，把 `(uuid, itemID)` 交给一个**异步、限流、带负缓存**的「码→图」任务，内部复用现成的 `yySkinItemLookup` 逻辑。理由：`skin_item_ids` 是协议里唯一明确的组件码来源，现在完全被丢弃。
2. **`PlayerList` 的 `skin.cape_id` 作为第二码源**
   `dial.go:517-525` 的实测注释写明「真实条目 CapeID = 皮肤 item_id」。读 `entry.Skin.CapeID`，纯数字就当作码。零成本，且覆盖「只有 PlayerList、没有 SyncSkin」的服务器。
3. **给前端一个「这个玩家穿的什么码」只读接口**
   例如 `GET /api/skin/player-item?name=`，数据来自内存 `PlayerInfo`（已在 `player_info.go` 维护），避免前端再按名字猜。
4. **明确不要做的事**
   - 不要调 `user-item-purchase` / `ChangeSkin`：前者真扣费，后者内部会先买。
   - 不要按 item_id 拼 `x19.gph.netease.com` 的 URL：没有规律。
   - 不要在 `validPlayerSkin` 里放宽尺寸：那会把「只有码」的包误当像素，产出花屏。
5. **凭据优先级维持现状**（账号实时凭据 > 配置文件），但建议把「为什么没图」的原文透传到卡片 title——现在的失败原因已经写得很具体（`YY-SKIN-ITEM` 各分支文案），别在前端吞掉。

---

## 六、未验证项与风险（诚实清单）

| # | 项 | 现状 | 验证成本 | 影响 |
| --- | --- | --- | --- | --- |
| 1 | `get-download-info` 对**未拥有**的组件码是否也返回 `res_url` | 未知。代码里没有任何「归属校验」 | 低（拿一个没买的码试，注意**不要**走购买） | 若能返回，说明「贴码即出图」对任意码成立；若返回错误，则皮肤库对未拥有码必须给出准确提示 |
| 2 | 组件资源包里纹理文件名 | 现有实现按 `skin.png` 优先、其余 png 兜底 | 低（A4 落地一次即可看 zip 列表） | 兜底猜错会取到 icon |
| 3 | `SyncSkin` 是否携带像素 | 未知，且现有代码要求 `len(PlayerData)==1` 才取 | 中（需真实服务器 + 至少两名玩家） | 决定「抓包」是否有取证价值 |
| 4 | 网易是否转发原版 `PlayerSkin` | **代码注释自相矛盾**（`open_connection.go:75` vs `player_info.go:279`） | 低（看日志有没有 `[皮肤] PlayerSkin`） | 影响 B2 的字段优先级 |
| 5 | `ConfirmSkin.uids` 与 g79 `user_id` 是否同一命名空间 | `【推断】` 同源 | 中（拿自己的 uid 去查 `pe-get-other-user-setting-list`） | 决定「查任意玩家当前皮肤」能否实现 |
| 6 | `X19Skin.url`（按 UUID 取皮肤）用的什么接口 | 只在第三方闭源库 `net.nekocurit.x19` 里，实现不可得 | 高（需逆向启动器或抓本地 IPC） | 若可用，是最优雅的「按人取图」；当前无法采用 |
| 7 | 私有接口稳定性 | 无文档、无 SLA，网易随时可改 | — | 所有依赖点都要有「失败即明确报错」的降级，不能静默 |

---

## 七、复现命令（可原样执行，只读）

```bash
# 1) 无凭据探测：确认接口存在且需要登录（返回 code:10）
curl -sS -m 15 -X POST "https://g79apigatewayobt.minecraft.cn/pe-download-item/get-download-info" \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "User-Agent: WPFLauncher/0.0.0.0" -H "user-id: 0" -H "user-token: 0" \
  -d '{"item_id":"8428087"}'
# 期望：{"code":10,"message":"请先登录","details":"","entity":null}

# 2) 核对公开镜像的接口实现
curl -sS "https://raw.githubusercontent.com/UCKETX/g79client/main/download_get_info.go" | sed -n '1,60p'
curl -sS "https://raw.githubusercontent.com/UCKETX/g79client/main/crypto.go" | sed -n '160,215p'

# 3) 核对本机复刻与上游一致（salt 必须相同）
grep -n "0eGsBkhl" upstream/youyuan_skin_item.go \
  third_party/funauth/modules/g79client/crypto.go

# 4) 核对协议字段（SyncSkin 的组件码）
grep -n "SkinItemIDs" upstream/flowers-for-machines/core/minecraft/protocol/packet/sync_skin.go \
  upstream/player_info.go

# 5) 核对落盘位置
grep -n "皮肤组件缓存\|皮肤缓存" upstream/youyuan_skin_item.go upstream/youyuan_skin_disk_cache.go
```

---

## 八、一句话交接

**「官方公开接口」= 无；「网易启动器私有接口」= 有（`pe-download-item/get-download-info`），而且不需要进游戏就能用。游戏内抓包只能给你 `skin_item_ids` 或像素，拿到码仍要回这个接口换资源——所以正确顺序是「先把私有接口的凭据打通，再用抓包做验证」，而不是反过来。**
