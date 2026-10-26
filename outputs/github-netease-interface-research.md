# GitHub 开源调研：网易我的世界头像/皮肤接口

- 调研日期：2026-10-23
- 调研人：Agent-B（游园项目调研子 Agent）
- 调研动机：认证站账号页有一个禁用按钮「选择网易官方头像 · 接口核实中」（`auth-service/public/console.js:185`），团队没有找到可用的「列出账号已拥有头像」接口。本报告回答两个问题：①GitHub 上有没有现成的网易头像列表/修改接口实现；②现有开源实现里，网易头像 URL 的构造规律是什么。
- 纪律声明：本报告所有结论均来自可溯源的公开代码与仓库，未做任何猜测式编造。凡没找到的，明确写「未找到」。

---

## 〇、TL;DR（先看结论）

1. **GitHub 上不存在「列出账号已拥有头像/皮肤列表」的公开接口实现。** 多路搜索（仓库 API、代码搜索平台、中英文 6 组关键词）均未发现。所有已知开源项目都只有四类能力：读当前皮肤、设当前皮肤、查商城组件（需已知 item_id）、读当前头像 URL。
2. **网易头像 URL 不是拼出来的。** `x19.gph.netease.com` 这类地址是接口 `pe-download-item/get-download-info` 返回体里 `entity.res_url` 字段的完整值，客户端原样使用。不存在可复用的路径拼接规律——任何"拼接公式"都是编造。
3. **对按钮的处理建议（三选一，均有依据）**：
   - 方案 A（最诚实）：保留禁用状态，把文案从「接口核实中」改成明确说明，例如「暂无公开的头像列表接口」；
   - 方案 B（有接口支撑）：用 `pe-user-detail/get` 的 `head_image`/`avatar_image_url` 展示账号**当前**头像（不是列表，但至少是真实数据）；
   - 方案 C（有接口支撑但用途有限）：允许高级用户手动输入**皮肤** item_id，经 `pe-set-user-setting-list` 切换（只能设皮肤，且要求用户自己知道 item_id，头像没有对应接口）。

---

## 一、调研方法

| 途径 | 具体动作 | 结果 |
| --- | --- | --- |
| GitHub 仓库搜索 | `FunAuth netease`、`g79client`、`网易 我的世界 接口` 等关键词，按 star/更新时间排序 | 定位到全部 5 个相关仓库（见下表） |
| GitHub 代码搜索 | grep.app、searchcode 平台搜 `pe-get-user-setting-list`、`gph.netease.com`、`avatar_image_url`、`get-user-setting-list` 等接口路径与字段名 | 平台受限/不通，未获得超出仓库枚举的结果 |
| 本地 vendored 源码 | 项目 `third_party/funauth`（即 GitHub `yeah-god/FunAuth` 的 vendored 副本）全量通读 g79client 模块 50+ 个 API 文件 | 获得全部接口的一手 URL/参数/返回结构（本报告第二节全部基于此） |
| Web 搜索 | 中英文 6 组关键词（网易MC头像接口/皮肤列表接口/netease minecraft skin api 等） | 未发现任何"头像列表"接口的公开文档或逆向文章 |

---

## 二、找到的相关开源仓库

| 仓库 | Star | 状态 | 与任务的相关性 | 可信度 |
| --- | --- | --- | --- | --- |
| [yeah-god/FunAuth](https://github.com/yeah-god/FunAuth) | 19 | fork，最近推送 2026-06 | **网易账号认证 + g79 接口全家桶**，本报告接口细节的主要来源；项目已 vendored 到 `third_party/funauth` | 高（一手源码，与线上接口同名同参） |
| [Mono2023-PRC/FunAuth](https://github.com/Mono2023-PRC/FunAuth) | 3 | 原版仓库 | 同上，上游源头；yeah-god 版是它的 fork | 高 |
| [UCKETX/g79client](https://github.com/UCKETX/g79client) | 6 | 活跃 | 另一份 Go 版 g79 客户端，接口覆盖面与 FunAuth 类似，可交叉验证 URL/参数 | 高（交叉验证通过） |
| [Koud-Wind/Netease-minecraft-LAN-connects-to-Server](https://github.com/Koud-Wind/Netease-minecraft-LAN-connects-to-Server) | 59 | 活跃 | 网易 MC 局域网联机协议实现，含皮肤/材质协议层处理，但**不涉及 Web 头像接口** | 中（协议层参考，与头像接口无关） |
| [Hazelmeow/neomega-core](https://github.com/Hazelmeow/neomega-core) | 1 | **已归档** | 网易 MC 协议库，同上只到协议层 | 低（归档 + 与头像接口无关） |
| [mineatar-io/skin-render](https://github.com/mineatar-io/skin-render) | 51 | 活跃 | 皮肤渲染库（2D/3D 等距渲染），**设计参考用**，见配套设计文档 | 高（仅用于渲染，非接口） |
| [NickAcPT/nmsr-rs](https://github.com/NickAcPT/nmsr-rs) | 116 | 活跃 | Rust 皮肤渲染器，mc-heads.net 同源实现，**设计参考用** | 高（仅用于渲染，非接口） |

> 另核查过 `XiYueMax/SiyoX`（Xposed 模块）等仓库，均不含头像列表接口，不逐条列出。
> Star 数为调研当日观察值，会随时间变化。

---

## 三、逐个接口分析

> 以下接口细节以项目 vendored 的 `third_party/funauth/modules/g79client/` 一手源码为准。所有请求都是 POST JSON，带 `user-id` 头 + `CalculateDynamicToken` 动态计算的 `user-token` 头（签名算法在 funauth 仓库内，认证流程走 `/auth/x19` 系列，本报告不展开）。基地址分两种：`ReleaseJSON.ApiGatewayUrl`（接口网关）与 `ReleaseJSON.CoreServerURL`（核心服务），x19 渠道还有 `X19ReleaseJSON.DCWebURL`。

### 3.1 读当前皮肤：`pe-get-user-setting-list`（x19 渠道为 `/get-user-setting-list`）

- **仓库/文件**：yeah-god/FunAuth → `modules/g79client/user_settings_get.go:29`
- **URL**：`POST {ApiGatewayUrl}/pe-get-user-setting-list`（`c.IsX19` 时 `/get-user-setting-list`）
- **请求体**：
  ```json
  {"settings": ["skin_type","skin_data","persona_data","screen_config","outfit_type","personal_open","personal_ad_open","personal_tags","bag_item"]}
  ```
- **返回结构**（节选）：
  ```json
  {"entity": {"skin_data": {"item_id": "xxx", "is_slim": false}, "screen_config": {...}}}
  ```
- **能力边界**：只返回**当前选中**的皮肤 item_id 和是否纤细模型。**没有列表、没有历史、没有"已拥有"集合。**
- **可信度**：高。项目 `auth-service/engine.go:349` 的 `currentAccountSkin()` 就在用它读当前皮肤，线上跑通。
- **与任务的关系**：这是"读取"侧唯一可用的头像/皮肤数据源，但它回答不了"账号有哪些头像"。

### 3.2 设当前皮肤：`pe-set-user-setting-list`

- **仓库/文件**：yeah-god/FunAuth → `modules/g79client/user_settings_set.go:15`
- **URL**：`POST {ApiGatewayUrl}/pe-set-user-setting-list`
- **请求体**：
  ```json
  {"data": {"skin_data": {"item_id": "<item_id>"}}}
  ```
- **返回**：标准 `Response`（code/message）。
- **能力边界**：把指定 item_id 的组件**设为当前皮肤**（仅选择，不购买）。前提是你已经知道 item_id。**没有对应的"设头像"接口**——funauth 全部 50+ 接口中不存在设置头像的路径。
- **可信度**：高。项目 `auth-service/engine.go:191` action=skin 时就在调它，线上跑通。
- **与任务的关系**：方案 C 的唯一依赖。注意它只能换皮肤，换不了"官方头像"。

### 3.3 组件下载地址（头像/皮肤 CDN 地址来源）：`pe-download-item/get-download-info`

- **仓库/文件**：yeah-god/FunAuth → `modules/g79client/download_get_info.go:22`
- **URL**：`POST {ApiGatewayUrl}/pe-download-item/get-download-info`
- **请求体**：`{"item_id": "<item_id>"}`
- **返回结构**：
  ```json
  {"entity": {"entity_id": "...", "res_url": "https://x19.gph.netease.com/..."}}
  ```
- **能力边界**：输入 item_id，输出该组件的资源下载地址。**URL 的构造规律就是：没有规律，`res_url` 是接口返回的完整地址。**
- **可信度**：高。项目 `auth-service/engine.go:360-367` 的链路 `GetUserSettingList → item_id → GetDownloadInfo → SkinDownloadURL` 线上跑通。
- **与任务的关系**：解答了第二个问题——`x19.gph.netease.com` 地址只能从接口取，不存在客户端拼接公式。项目 `/api/skin/proxy` 白名单放行 `*.netease.com`/`*.163.com` 的做法与之一致。

### 3.4 读当前头像：`pe-user-detail/get`

- **仓库/文件**：yeah-god/FunAuth → `modules/g79client/user_get_detail.go:115`
- **URL**：`POST {CoreServerURL}/pe-user-detail/get`（注意这个走 CoreServerURL，不是 ApiGatewayUrl）
- **请求体**：`{}`
- **返回结构**（头像相关字段节选）：
  ```json
  {"entity": {"head_image": "https://x19.gph.netease.com/...", "avatar_image_url": "...", "frame_id": "...", "head_image_cd": ...}}
  ```
- **能力边界**：返回账号**当前**头像 URL、头像框 ID。同样**没有"已拥有头像列表"**。
- **可信度**：高（一手源码），但本项目尚未调用过它，接入前需实测字段是否随账号变化。
- **与任务的关系**：方案 B 的唯一依赖。注意 `head_image_cd` 字段暗示官方对头像更换频率有限制，产品上要接受"展示可以，随便换不行"。

### 3.5 商城组件查询：`item-buy-list/query/search-mcgame-items`

- **仓库/文件**：yeah-god/FunAuth → `modules/g79client/store_search_mcgame_items.go:45`
- **URL**：`POST {ApiGatewayUrl}/item-buy-list/query/search-mcgame-items`
- **请求体**：
  ```json
  {"item_ids": ["<id1>", "<id2>"], "entity_ids": ["<id1>", "<id2>"]}
  ```
- **返回结构**：`{"entities": [{"iid":..., "name":..., "brief":..., "stypeid":..., "behaviour_uuid":..., ...}], "total": ...}`
- **能力边界**：**必须先有 item_id 才能查**（按 ID 反查组件详情），不是"翻商城"接口。funauth 里另有 `user_hot_categories` 等分类接口，但都不提供"我的账号拥有哪些头像"。
- **可信度**：高（一手源码）。
- **与任务的关系**：如果有人手动提供 item_id，可以用它展示组件名/简介，做弱化版选择器。但它不产生列表。

### 3.6 购买组件：`user-item-purchase`

- **仓库/文件**：yeah-god/FunAuth → `modules/g79client/user_item_purchase.go:23`
- **URL**：`POST {X19ReleaseJSON.DCWebURL}/user-item-purchase`（注意走 DCWebURL）
- **能力边界**：真实扣费购买。**与"选择已有头像"无关，列出仅供完整性，禁止在本功能里引入扣费路径。**
- **可信度**：高（一手源码）。

### 3.7 旁证：查他人设置 `pe-get-other-user-setting-list`

- **仓库/文件**：yeah-god/FunAuth → `modules/g79client/user_other_settings_get.go:153`
- **能力边界**：查**任意 user_id** 的 skin_data/screen_config。它的存在进一步说明：这类"用户设置"接口的响应形状就是"当前值"，网易服务端没有把"拥有列表"暴露给客户端——否则这里会见到数组形态。
- **可信度**：高（一手源码）。

---

## 四、明确结论

### 结论 1：头像列表接口——未找到，且大概率不存在公开实现

- funauth/g79client 全部 50+ 个 API 路径中，与头像/皮肤相关的只有第三节 7 个，**没有任何一个返回"账号已拥有头像/皮肤集合"**。
- GitHub 仓库搜索 + 代码搜索 + Web 搜索三路均无收获。
- 旁证：官方客户端（WPFLauncher）的"我的外观"页面数据也不是从这类 REST 接口拉的（funauth 的 UA 头全部伪装 `WPFLauncher/0.0.0.0`，说明这些接口就是启动器同款，而启动器头像列表走的是另一套未开源的资源服协议）。
- **处理建议：不编造接口。** 按钮要么改文案说清楚，要么用 3.4 展示当前头像。任何"调个列表接口就能列出来"的承诺都是假的。

### 结论 2：头像 URL 规律——无拼接规律，只能接口取

- `x19.gph.netease.com/...` 完整 URL 来自 `pe-download-item/get-download-info` 的 `res_url`，或 `pe-user-detail/get` 的 `head_image`/`avatar_image_url`。
- 项目中 `auth-service/public/console.js:38` 的 `yyAvatarSrc()` 已统一走 `/api/skin/proxy` 代理（服务端白名单校验 + 7 天缓存），这条链路是现成的，接入 3.4 的返回 URL 时直接复用即可。

### 结论 3：按钮「选择网易官方头像 · 接口核实中」的处置建议

| 方案 | 依赖接口 | 工作量 | 诚实度 | 建议 |
| --- | --- | --- | --- | --- |
| A. 改文案，保持禁用 | 无 | 5 分钟 | 最高 | **首选**。例如「网易未开放头像列表接口，暂不支持选择」 |
| B. 展示当前官方头像 | `pe-user-detail/get` | 半天（需实测 + 接 proxy） | 高 | 与 A 叠加做：禁用选择，但把当前头像显示出来 |
| C. 手动 item_id 换皮肤 | `pe-set-user-setting-list` | 较小（engine.go 已有 action=skin 链路） | 中（只能皮肤，且要用户自备 item_id） | 仅当产品坚持要"可操作"时做，且必须标注"仅皮肤" |

---

## 五、来源链接

- yeah-god/FunAuth（项目 vendored 版上游）：https://github.com/yeah-god/FunAuth
  - 读当前皮肤：https://github.com/yeah-god/FunAuth/blob/main/modules/g79client/user_settings_get.go
  - 设当前皮肤：https://github.com/yeah-god/FunAuth/blob/main/modules/g79client/user_settings_set.go
  - 下载地址：https://github.com/yeah-god/FunAuth/blob/main/modules/g79client/download_get_info.go
  - 用户详情（当前头像）：https://github.com/yeah-god/FunAuth/blob/main/modules/g79client/user_get_detail.go
  - 商城组件查询：https://github.com/yeah-god/FunAuth/blob/main/modules/g79client/store_search_mcgame_items.go
  - 购买：https://github.com/yeah-god/FunAuth/blob/main/modules/g79client/user_item_purchase.go
  - 他人设置：https://github.com/yeah-god/FunAuth/blob/main/modules/g79client/user_other_settings_get.go
- Mono2023-PRC/FunAuth（上游原版）：https://github.com/Mono2023-PRC/FunAuth
- UCKETX/g79client（交叉验证）：https://github.com/UCKETX/g79client
- 项目内引用位置（供改代码的 Agent 参考）：
  - `auth-service/public/console.js:185`（禁用按钮）、`console.js:38`（yyAvatarSrc 代理）
  - `auth-service/engine.go:191`（SetUserSettingList 调用）、`engine.go:348-367`（currentAccountSkin 链路）
  - `third_party/funauth/modules/g79client/`（全部接口源码）

> 复核方式：以上 URL 均于 2026-10-23 经 GitHub API 实际拉取验证存在；接口细节与项目 `third_party/funauth` 本地副本逐行核对一致。
