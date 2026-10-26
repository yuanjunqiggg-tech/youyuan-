# 房间发现功能审计报告

审计对象：Minecraft 网易版自动化工具（APK + Go 后端）的「输入房间号 → 自动识别 → 分类展示 → 连接」
审计日期：2026-10-21
审计方式：只读源码 + git 历史取证，未修改任何代码文件。

涉及文件：

| 层 | 文件 |
| --- | --- |
| 前端 | `upstream/frontend/js/youyuan_room_discovery.js`（225 行）、`upstream/frontend/css/youyuan_room_discovery.css`（1 行） |
| Go 网关 | `upstream/youyuan_room_discovery.go`（252 行）、路由 `upstream/main.go:4966` |
| 认证服务 | `auth-service/worker.mjs`、`auth-service/engine.go`、`auth-service/room_discovery.go`（107 行） |
| 网易协议库 | `third_party/funauth/modules/g79client/*.go` |

---

## 一、当前输入解析与调用链

### 1.1 入口与前缀分流

用户点「连接服务器」时执行的是被覆写的 `doConn`（`youyuan_room_discovery.js:192-202`）：

```js
if (/^DomainGame:|^LobbyGame:room_id=|^https?:\/\/|mc\.163\.com\//.test(raw)
    || /^\d{16,20}$/.test(raw)
    || /^[0-9a-f]{11}(?:_[1-9]\d*)?$/i.test(raw)) return previous();
return discover(false);
```

| 输入形态 | 走向 | 说明 |
| --- | --- | --- |
| `114514`（短数字/房名） | **发现流程** | 主力路径 |
| `#114514` | **发现流程** | 前端未 bypass；后端 `discoverRooms` 会剥掉 `#` |
| `@123` | **发现流程** | 前端未 bypass；`discoverRooms` 会剥掉 `@` |
| `LobbyGame:room_id=…` | 旧直连 | 显式标识，跳过发现 |
| `DomainGame:…` | 旧直连 | 山头服邀请码 |
| `mc.163.com/open/mc_realms/?realms=…` | 旧直连 | 分享链接，`yyNormalizeServer` 抽 `realms` 参数（`youyuan_connection.go:49-55`） |
| 16–20 位纯数字 | 旧直连 | 视为完整 room_id |
| 11 位十六进制（可带 `_N`） | 旧直连 | 山头服邀请码 |

结论：**「自动识别、不用用户加前缀」只对短号/房名成立**。用户一旦贴的是 `#号`、`@号`、完整 room_id、山头邀请码或分享链接，前端就直接走老的直连逻辑，不会进发现流程。`#` / `@` 之所以"还能用"，是因为它们要么被后端剥前缀，要么被 bypass。

### 1.2 完整调用链（带行号）

```
[前端] doConn 覆写            youyuan_room_discovery.js:193-202
  └─ discover(force)          :157-191
       ├─ ensureInput()       :45-57   空输入时用 /api/bot/status 的已连接服务器号预填
       └─ POST /api/youyuan/rooms   :173   body {force, token, auth, input}
[Go]  yyRoomDiscoveryHandler  youyuan_room_discovery.go:164-252   （路由 main.go:4966）
       ├─ op=current  → 反查已记忆选择        :190-201
       ├─ 记忆命中且 !force → 直接返回 selected :202-206
       ├─ op=select   → 校验候选属于本次查询（5 分钟窗口）:207-229
       └─ yyRoomDiscoverRemote()              :101-149
             POST {auth}/api/game/discover  body {"keyword": input}
[认证] worker.mjs:37-46  校验 Bearer → vault /__bearer/discover
       worker.mjs:202-207  → kernel({action:'room_discover', keyword, offset})
       engine.go:237-238   → discoverRooms(cli, keyword, offset)
[auth-service] discoverRooms  room_discovery.go:18-101
       ├─ keyword 归一化 :19-21   trim → 去 "#" → 去 "@" → 去 "LobbyGame:room_id="
       ├─ 租赁服  SearchRentalServerByName(keyword)          :34-53
       ├─ 联机房间 SearchOnlineLobbyRoomByKeyword(kw,20,off) :54-74
       └─ 本地房间 GetTransferRoomWithName(keyword)          :75-93
[Go]  二次过滤 :122-135（ID 形态校验、按 ID 去重）+ :136-147（complete 判定）
[前端] render(result)          youyuan_room_discovery.js:90-142
       若 complete && candidates.length===1 → 自动 choose()   :185
       choose() → op=select → renderChoice() → start() → previous()  :62-89
[Go]   POST /api/bot/connect → main.go:1008
       req.Server = yyNormalizeServer(yyRoomRememberedTarget(token, auth, req.Server))
       → 用 room-choices.json 里的候选 target 覆盖用户原始输入
[连接] bot_conn.go:165-172  server 以 "@" 开头 → connectTanLobby（本地房间）
       否则按 LobbyGame:room_id= / 租赁服号 走官方登录
```

关键点：`start()`（`:58-61`）本身**不传 candidate.target**，而是重新调用 `previous()` 用原始输入；真正把「选中的候选」落地的是服务端 `main.go:1008` 的 `yyRoomRememberedTarget`。这条链是通的，但耦合在「先写记忆、再连接」的隐式约定上。

### 1.3 keyword 归一化的缺口

`room_discovery.go:19-21` 只处理 `#`、`@`、`LobbyGame:room_id=` 三种前缀。`DomainGame:` 与 `mc.163.com` 链接**完全没有在 discoverRooms 里处理**（目前靠前端 bypass 规避）。如果以后放开前端 bypass，这两类会直接当关键词搜，必然搜不到。

---

## 二、三类房间的识别逻辑与返回结构

### 2.1 识别逻辑

| 类别 | 调用的网易接口 | 匹配字段 | 生成的 target | 合法性校验 |
| --- | --- | --- | --- | --- |
| 租赁服 `rental` | `/rental-server/query/search-by-name`（`rental_search_by_name.go:16-56`） | `server_name` | `v.Name`（租赁服**号**） | target 非空且不含 `@:#/\`（`youyuan_room_discovery.go:155-156`） |
| 联机房间 `lobby` | `/online-lobby-room/query/search-by-name-v2`（`online_lobby_room_search.go:59-104`） | `room_name`（也命中房主名） | `LobbyGame:room_id=<id>` | room_id 必须全数字（`:157-158`） |
| 本地房间 `local` | `{TransferServerNewHttpUrl}/room-with-name`（`transfer_room_with_name.go:57-105`） | `name` | `@<rid>` | rid 必须全数字（`:159-160`） |

三路是**串行全部执行**，任一路失败不影响其它两路（各自 `addSource` 记录 ok/error，`room_discovery.go:27-33`）。

`complete` 的语义（`room_discovery.go:94-99` + `youyuan_room_discovery.go:136-147`）：三类都必须查成功、且没有「还有下一页」。只有 `complete=true` 才允许自动连接，避免把"没搜完"误判成"唯一结果"。

⚠️ 风险点：租赁服用的是 **search-by-name（按名称）**，但输入 `114514` 是服号。纯数字能否命中服号取决于网易接口行为，代码层面没有任何按服号查询的兜底。测试用例（`room_discovery_test.go:17-19`）是 fixture 假造的，不能证明真实可用。

### 2.2 返回结构

前端最终拿到的 JSON（`youyuan_room_discovery.go:33-41`）：

```json
{
  "ok": true,
  "complete": true,
  "candidates": [ /* yyRoomCandidate */ ],
  "sources": [{"type":"rental","ok":true,"count":1,"more":false}, ...],
  "selected": { /* 记忆命中时才有 */ },
  "remembered": true,
  "error": ""
}
```

候选结构（`youyuan_room_discovery.go:18-32`）：

| 字段 | 类型 | rental | lobby | local |
| --- | --- | --- | --- | --- |
| `id` | string | `rental:<entity_id>` | `lobby:<room_id>` | `local:<rid>` |
| `type` | string | `rental` | `lobby` | `local` |
| `room_id` | string | entity_id | room_id | rid |
| `target` | string | 服号 | `LobbyGame:room_id=…` | `@rid` |
| `name` | string | `server_name` | `room_name` | `name` |
| `number` | string | 服号 | — | — |
| `owner` | string | — | `owner_name` | — |
| `owner_id` | string | `owner_id` | `owner_id` | `hid` |
| `cover_url` | string | `image_url` | — | — |
| `avatar_url` | string | — | — | — |
| `players` | int | `player_count` | `cur_num` | `cnt` |
| `max_players` | int | `capacity` | `max_count` | `cap` |
| `version` | string | `mc_version` | `version` | — |

---

## 三、数据可得性矩阵

图例：✅ 现在就能拿到（已进返回结构）｜🟡 接口里有、但代码没取｜❌ 当前代码未获取，且已知接口里没有该字段

### 3.1 汇总表

| 字段 | 租赁服 | 联机房间 | 本地房间 |
| --- | --- | --- | --- |
| 房间 ID | ✅ `entity_id`（`room_discovery.go:41,46`） | ✅ `room_id`（`:63,67`） | ✅ `rid`（`:82,86`） |
| 房间名称 | ✅ `server_name`（`:46`） | ✅ `room_name`（`:67`） | ✅ `name`（`:86`） |
| **房间头像/封面** | ✅ **`image_url`**（`:46` → `cover_url`） | ❌ 当前代码未获取（结构体无此字段） | ❌ 当前代码未获取（结构体无此字段） |
| 在线玩家数 | ✅ `player_count`（`:47`） | ✅ `cur_num`（`:68`） | ✅ `cnt`（`:87`） |
| 最大人数 | ✅ `capacity`（`:48`） | ✅ `max_count`（`:69`） | ✅ `cap`（`:88`） |
| 玩家列表 | 🟡 有独立接口未接入 | 🟡 `member_uids` 已声明未使用 | 🟡 `members` 已声明未使用 |
| **房主名称** | ❌ 当前代码未获取（只有 `owner_id`） | ✅ **`owner_name`**（`:67`） | ❌ 当前代码未获取（只有 `hid`） |
| 房主头像 | ❌ 未获取（可查，见 3.5） | ❌ 未获取（可查，见 3.5） | ❌ 未获取（可查，见 3.5） |
| 房间类型 | 🟡 `server_type` 已声明未用 | 🟡 `tag`/`res_id` 已声明未用 | 🟡 `type` 已声明未用 |
| 游戏版本 | ✅ `mc_version`（`:46`） | ✅ `version`（`:67`） | 🟡 `version` 已声明未取（`transfer_room_with_name.go:37`） |
| 是否满员 | 🟡 可由 `players>=max_players` 推导，未实现 | 🟡 同左 | 🟡 同左 |
| 是否需要密码 | 🟡 `has_pwd`（rental）未取；`password`（lobby）未取 | 🟡 `need_password` 未取 | 🟡 `need_password` 未取 |

### 3.2 租赁服：唯一有真实封面的一类

- 字段来源：`RentalServerEntity.ImageURL` ← `json:"image_url"`（`rental_available_list.go:24`，注释明确写「租赁服头像URL」）。
- 已经映射到 `cover_url`：`room_discovery.go:46`。
- 前端渲染：`youyuan_room_discovery.js:106` → `image(v.cover_url, "yy-room-cover")`，CSS `.yy-room-cover{width:100%;height:112px;object-fit:cover}`。
- 限制：`image()` 只接受 `https://`（`:19`），非 https 或加载失败会直接 `remove()`（`:24-26`），所以"有字段"不等于"一定显示"。
- 房主名称：`RentalServerEntity` 只有 `OwnerID`，**没有 owner name**（`rental_available_list.go:14`）。前端 `:114` 会退化成显示「房主 UID 1234567890…」——一串长数字，体验很差。
- 另注：`rental_available_list.go:19` 对 `player_count` 的注释写成「最大人数」，与 `capacity`（容量）语义冲突，需以实测为准。

### 3.3 联机房间：唯一有房主名的一类，但没有封面

- 房主名：`OnlineLobbyRoomSearchEntity.OwnerName` ← `owner_name`（`online_lobby_room_search.go:23`），已映射 `owner`（`room_discovery.go:67`），前端 `:113` 显示「房主 xxx」。
- 在线人数：`CurNum` / `MaxCount`（`:19-20`）→ `players` / `max_players`（`:68-69`）。
- 封面：`OnlineLobbyRoomSearchEntity`（`:11-40`）与详情实体 `OnlineLobbyRoomGetEntity`（`online_lobby_room_get.go:19-50`）**都没有任何 cover/icon/image 字段**。
- ⚠️ 但必须说清一个不确定项：该结构体注释写着「search-by-name-v2 返回的实体**字段子集**」（`:10`）。Go 的 `json.Unmarshal` 会丢弃未声明字段，因此**代码无法证明接口没有返回封面**。要定论只能抓一次真实响应体。

### 3.4 本地房间：字段最贫瘠

- 只有 `rid` / `name` / `hid` / `cnt` / `cap`（`transfer_room_with_name.go:19-45`），映射见 `room_discovery.go:86-88`。
- 无封面、无房主名（`hid` 是房主标识，不是昵称）、无版本（`version` 声明了但没取）。
- 前端 `:114` 同样退化成「房主 UID xxx」。

### 3.5 房主头像：三类都缺，但有可行接口

`avatar_url` 在 Go 侧**从未被赋值过**（全库 grep 只命中结构体声明 `youyuan_room_discovery.go:28` 和一条"禁止编造"的断言 `room_discovery_test.go:58`）。前端的 `image(v.avatar_url, "yy-room-avatar")`（`:109`）是**死代码**。

理论可行路径（有代码依据）：

| 接口 | 方法 | 返回 | 依据 |
| --- | --- | --- | --- |
| `/user-else-detail-many/` | `GetUserElseDetailMany(uids []string)` | `[]SocialProfile`（含 `nickname`、`headImage`） | `user_else_detail_many.go:19-22` + `social_profile.go:6,8` |
| `/user-detail/query/other` | `GetOtherUserDetail(entityID, ...)` | 单个 `SocialProfile` | `user_other_detail.go:19-21` |

三类候选都带 `owner_id` / `hid`，批量查一次即可同时补齐「房主昵称 + 房主头像」。**未验证点**：`owner_id` / `hid` 与 `user-detail` 要求的 `entity_id` 是否同一 ID 空间，需实测确认。

### 3.6 玩家列表：三类都有线索，但都没接

| 类别 | 现成能力 | 位置 |
| --- | --- | --- |
| 租赁服 | `SearchRentalServerPlayersByServer`，可按 `is_online` 过滤，返回 `name` + `headImage` | `rental_server_player_search_by_server.go:37-52`（`headImage` 在 `:44`） |
| 联机房间 | 搜索结果里已声明 `member_uids []string`，未使用 | `online_lobby_room_search.go:39` |
| 本地房间 | `room-with-name` 已声明 `members []Uncertain`，未使用 | `transfer_room_with_name.go:28` |

注意：这三条都是"每人一次额外请求"或"需再解析 uid"，会显著放大一次房间发现的耗时（现在整体有 35 秒超时，`youyuan_room_discovery.go:109`）。

---

## 四、历史实现取证结论

### 4.1 结论：**仓库内不存在"更完整"的历史版本**

- 仓库共 51 个 commit，最早为 `9f90893`（2026-10-03，「preserve existing framework source baseline」），属于**外部代码导入式 baseline**，之前的历史不在本仓库。
- `youyuan_room_discovery.js` / `youyuan_room_discovery.go` / `auth-service/room_discovery.go` 三个文件**首次出现即 `6f703ec`**（2026-10-19，「baseline: 接手 0.10.75 阶段」），一次性整文件加入。
- 后续只有 3 次改动，且都不涉及字段能力：
  - `d160c53`（0.10.77）：前端预填与空输入兜底，`youyuan_room_discovery.js` +13/-13。
  - `9604062`（三线合流）：**纯 ES2019 语法降级**（可选链/箭头函数改写），逐行比对内容语义完全一致，新增 `.js.esb` 副本。
  - `71ba734`：结果区 `min-height:170px` 防塌陷 + 资源版本号。
- `git log -S"avatar_url"` 只命中 `6f703ec` 与 `9604062`，即 **avatar 字段自诞生起就是空壳**，没有任何 commit 删除过它的赋值。
- 无任何被删除（`--diff-filter=D`）的 room 相关文件。

### 4.2 用户记忆里的「人数 + 头像 + 房主」来自哪里

仓库里能对上的只有两处**旧式列表**，都不是卡片、都没有头像：

| 位置 | 展示内容 |
| --- | --- |
| `upstream/frontend/js/youyuan_game_auth.js:290-357`（`yyRoomSearchPage`，渲染在 `:323-352`） | 按钮列表：`房名 · 12/20 人 · room_id xxx` |
| `auth-service/public/index.html:91`（`showRooms`） | 按钮列表：`房名 · 房主 xxx · 12/20 · room_id xxx` |

这两处的数据源是 `action:'rooms'` → `engine.go:239-259`（`room_list`），返回 `name / owner / players / max_players / password_required`，同样**没有封面**。

最可能的解释：用户看到的是「租赁服卡片上的封面图」+「联机房间卡片上的房主名」+「两类都有在线人数」这三件事**分散在不同类别上**，被记忆合并成了"一个卡片上三者齐全"。代码层面从未实现过三者同框。

---

## 五、要做「正方形卡片 + 头像 + 在线人数 + 房主」还缺什么

### 5.1 现状与需求的差距

| 需求 | 现状 | 差距 |
| --- | --- | --- |
| 不用加前缀 | 短号/房名 OK；`#`/`@` 靠后端剥；显式标识/分享链接直接 bypass | 基本满足，但 bypass 分支绕过了分类展示 |
| 三类分组、空类不显示 | 已实现（`render` `:96-100`） | ✅ 无差距 |
| 租赁服唯一 → 直接进 | 已实现（`:185` 单候选自动 choose） | 没有"确认连接"这一步，是**直接连** |
| 联机房间多个 → 卡片 | 已实现（`:104-131`） | 卡片是矩形，非正方形 |
| 本地房间唯一 → 点确认连接 | 单候选**自动连**，不点确认 | 与需求相反：现在没有确认动作 |
| **正方形卡片** | `.yy-room-grid` 用 `minmax(min(240px,100%),1fr)`，`.yy-room-card` 无 `aspect-ratio`，封面固定 112px 高 | 纯 CSS 问题，无数据依赖 |
| **头像** | 仅租赁服有 `image_url`（作为 112px 横幅）；`avatar_url` 是死字段 | 见 5.2 |
| **在线人数** | 三类全有，且已在卡片显示（`:115-116`） | ✅ 数据不缺 |
| **房主** | 仅联机房间有真名；租赁服/本地只有 UID | 见 5.3 |

### 5.2 头像/封面：分别卡在哪

| 类别 | 卡点 | 可行动作 |
| --- | --- | --- |
| 租赁服 | **不缺数据**，已有 `image_url`。卡在"当横幅用"而不是"当头像用"，且被 `^https://` 白名单限制 | 无需新接口，只改渲染（正方形 + 头像裁切） |
| 联机房间 | **确实没有字段**。`search-by-name-v2` 与 `online-lobby-room/get` 的 Go 结构体都没有 cover | ①先抓一次真实响应，确认是否有未声明字段；②若确实没有，可用 `owner_id` 查房主头像兜底；③`res_id` 可查商城组件图（`store_search_mcgame_items.go`），但那是**玩法封面**不是房间封面，且需要额外一次请求 |
| 本地房间 | **确实没有字段**，`room-with-name` 只有文本信息 | 只能用 `hid` 查房主头像兜底；房间本身无图 |

### 5.3 房主名称：分别卡在哪

| 类别 | 卡点 | 可行动作 |
| --- | --- | --- |
| 租赁服 | 接口只给 `owner_id` | `GetUserElseDetailMany([owner_id])` → `nickname`（`user_else_detail_many.go:19-22`）。一次批量请求可同时拿到昵称 + 头像，性价比最高 |
| 联机房间 | **不缺**，`owner_name` 直接可用 | 无需改动 |
| 本地房间 | 只有 `hid` | 同上，但需先确认 `hid` 是否为可查档案的 uid |

### 5.4 落地顺序建议（按性价比）

1. **纯前端**：`.yy-room-card` 加 `aspect-ratio:1/1` + 封面改为方形/头像位；单候选时不自动连、改为展示"确认连接"按钮（对齐"点确认直接连接"）。
2. **补昵称**：`discoverRooms` 收集 `owner_id`/`hid`，在返回前批量 `GetUserElseDetailMany` 一次，回填 `owner` 与 `avatar_url`。这一步能让三类同时拥有「房主名 + 房主头像」，是收益最大的一次改动。
3. **抓包定论**：抓 `search-by-name-v2` 原始响应，确认是否存在未声明的封面字段；若有，补进 `OnlineLobbyRoomSearchEntity` 即可。
4. **可选**：租赁服接 `SearchRentalServerPlayersByServer` 展示在线玩家列表（按需展开，避免拖慢主查询）。

### 5.5 必须提醒的两个风险

- `discoverRooms` 里租赁服走的是**按名称搜索**（`server_name`），输入纯服号能否命中未经验证；测试是 fixture 假造的。
- 一次发现最多 3 个官方请求，若再叠加"每个候选查一次房主档案"，请求数会线性膨胀；现有 35 秒超时（`youyuan_room_discovery.go:109`）和 `complete` 判定（要求三类全 ok）会被拖垮，需做批量合并与超时预算。
