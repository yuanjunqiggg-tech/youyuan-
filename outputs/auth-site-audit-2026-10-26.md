# 游园独立认证站（youyuan-game-auth）UI / 功能审查

- 审查时间：2026-10-26
- 源码：`auth-service/public/`（index.html / console.js / console.css / auth-entry.css / skin-viewer.js / return-context.js）
- 线上：`https://auth.youyuanqi.dpdns.org/?return_to=…toolbox…`
- Worker：`youyuan-game-auth`（`wrangler.jsonc` name + route `auth.youyuanqi.dpdns.org`，与 toolbox 独立）
- 方法：源码逐行 + 线上 `curl` 抓取比对（md5 / 语义 diff）+ 无头 Chrome（CDP）注入桩数据，在 360 / 390 / 768 / 1280 四档实测 DOM 与几何尺寸。
  桩数据方式：`Page.addScriptToEvaluateOnNewDocument` 覆写 `fetch`，返回 1–2 个假账号与 64×64 假皮肤纹理，因此**控制台部分为"真实前端代码 + 伪造接口数据"实测**；登录/真实账号数据未验证。

---

## 0. 一句话结论

用户 8 条投诉中，**1、2(源码)、3、7(主体) 已修**；**6、8 确认成立**；**4 存在必现 P0 回归（收起再展开立体皮肤消失）**；**5 名不副实但可用**；**2 在线上仍有残留文案，因为线上 console.js 是旧版**。

线上唯一落后文件是 `console.js`（差 2 行），其余资源与本地 md5 完全一致 → **需要重新部署 worker**。

---

## 1. 逐条核对表

| # | 条目 | 源码证据 | 线上证据 | 结论 |
|---|---|---|---|---|
| 1 | 「撤销云端凭证 / 撤销游戏凭据」按钮必须不存在 | 全仓 grep `撤销` 仅 2 处，均非按钮：`console.js:16` 文案"只有明确点击重签才撤销旧令牌"；`index.html:525` 文案"撤销凭据后不能重新登录"。详情"令牌与凭据维护"只有 `重签游戏令牌`（`console.js:239`，confirm 后 `action:'reissue'`），无撤销按钮 | `live-console.js:16`、`live-index.html:525` 同样仅 2 处文本；线上 HTML 与本地 md5 相同（`b8169c14…`） | ✅ 通过。**建议改词**：这两句仍让用户以为存在"撤销凭据"功能（见 P2-1） |
| 2 | 「选择网易官方头像 · 接口核实中」假禁用按钮必须不存在 | 无该按钮；无 `接口核实中`；`console.js:189-193` 改为如实说明"网易未开放头像列表与更换接口，官方头像按账号资料自动显示" | **线上仍有残留**：`live-console.js:15` = "…原位展开资料。**官方头像选择接口正在核实，当前不自动指定角色头像。**"；本地第 15 行已删掉这半句 | ⚠️ 源码已清、**线上未部署**。不是按钮，但"正在核实"这句话正是用户反复投诉的原句 |
| 3 | 头像链：网易官方 → 皮肤纹理裁头 → 占位；不得出现同名陌生人 | `console.js:102-133`：`avatar_source==='netease'` 才用官方图（`https:` + 无 user:pass 校验）→ 代理失败退直连 → 再失败 `loadMc()`；`console.js:60-91` 从 `texture` 按 (8,8) 32×32 裁剪并叠加 hat 层；占位为 `authHeadSvg`（`console.js:98`）。全仓无 `crafatar` / `minotar` | 实测（390，账号给一个必然 404 的 `https://gph.netease.com/fake-avatar.png`）：最终 `<img alt="玩家自己的游戏头像" naturalWidth=32 src="data:image/png…">`，即**回退到本人 MC 头成功**，未出现任何第三方同名头像 | ✅ 通过。但**代理链实际是死的**：`console.js:38-43` 把所有网易头像改走 toolbox `/api/skin/proxy`，实测该接口 `401 {"code":"AUTH_REQUIRED"}`（img 请求带不了 bearer/cookie）→ 每个头像先白跑一次失败请求，再打直连 `gph.netease.com`。首屏头像变慢、墙内可能久白（P1-1） |
| 4 | 立体皮肤打开即显示 + 前几个账号预拉纹理 | `console.js:138` 渲染前预取前 3 个账号 `skin_preview`；`console.js:172-178` 展开后异步建 viewer，成功才 `stageWrap.hidden=false` | 实测展开到 `.yy-skin-model` 出现 **61ms**（缓存命中）；`stageHasModel=true`（360/390/768/1280 均渲染） | ❌ **P0 回归**：`console.js:165` 收起 → `authDisposeViewers` → `viewer.dispose()` 执行 `model.remove()`；再展开时 `panel.children.length` 已非 0，**不再重建**。实测 `modelAfterCollapse=false` → `modelAfterReopen=false`，但台子(h=180)与姿势条仍显示 → 空白皮肤台 + 点不动的"走动/站立"。这几乎就是用户"立体皮肤不显示"的复现路径（刷新列表后能恢复，因 `authRender` 全量重建，实测 `modelAfterRefresh=true`） |
| 5 | 组件码预加载区能否真用 | `console.js:207-233`：只能读**当前账号**皮肤纹理，码只列条目 + "应用"按钮；`console.js:233` notice 已如实写明"未应用的组件码不显示皮肤图" | 实测（390，填 `12345`/`67890`）：生成 3 个 tile —— "当前账号皮肤"含真实 32×32 纹理头；两个码 tile 只有占位 SVG + 应用按钮；状态文案"已列出 2 个组件码…" | ⚠️ 功能可用（列码/应用/刷新预览），但**名字"预加载"名不副实**，用户会期待每个码都有缩略图 → 建议更名（P1-4） |
| 6 | 360px 下「走动/纤细/正视角」掉到下面 + 多宽度适配 | `console.css`（内联于 `index.html:304-308`）`.auth-skin-controls{display:flex;flex-wrap:wrap;justify-content:center}`，**没有任何窄屏覆盖规则**；`index.html:461-472` 的 400px 层也没管它 | 实测皮肤弹窗三组控件顶坐标：**360px → 540 / 540 / 604**；**390px → 540 / 540 / 604**；768px → 578 / 578 / 582；1280px → 578 / 578 / 582。即 360、390 下"复位视角"整组换行并居中 → 与上面两组错位（用户说的"正视角"就是"复位视角"）。页面整体无横向溢出：`documentElement.scrollWidth` = 360→360、390→390、768→753、1280→1265 | ❌ 成立。另：详情台内姿势条在 360 下实测按钮仅 **33×23px**（容器高 29px），远低于 44px 触控标准（P1-3） |
| 7 | 账号标识（令牌）不得明文展示，仅主动查看才允许 | `index.html:525` `#token` 为 `type="password"`；`console.js:155-161` 只有 `authUseToken()` 才写入；`console.js:239` 重签需 confirm | 实测：未操作时 `.auth-token-inline` 数量 = **0**；`#token.type` = **password**；`authUseToken` 后卡片内 `<code>` 出现明文 `eyJ0ZXN0…` | ⚠️ 主体通过（无默认明文），但 3 处不满足"只有主动查看"：① `console.js:16` 令牌页下拉 `onchange=()=>authUseToken()` —— **在下拉里点一下账号**就签发读取 + 自动写剪贴板 + 卡片明文常驻；② 明文 `<code>` 一直留到下次 `refresh()`，切页不消失；③ `worker.mjs:115` 每次 `/api/accounts` 都返回 `token_hint`（令牌前 5 位），前端未使用但接口层外泄（P1-2） |
| 8 | 功能拆两段、上下不对齐、繁杂、AI 味 | ① 详情面板 3 个折叠区默认全关（`console.js:179/198/236`），点开账号后还要再点 3 次；② 皮肤能力拆两处：详情 hero 只有 走动/站立（`console.js:178`），弹窗有 站立/走路/经典/纤细/复位视角（`console.js:243`），且命名不一致（走动 vs 走路）；③ 同义说明重复：头像 `console.js:185`+`193` 两段，组件码 `199`+`233` 两段；④ 添加页残留 4399 死路：`index.html:525` `option login_4399 disabled`，但整块 4399 用户名/密码/验证码/"打开4399官方注册"仍在 DOM，永久不可达 | 实测 360：详情内 `sections` = 头像与账号资料 / 游戏皮肤与组件码 / 令牌与凭据维护，`open` 全为 `false`；添加页子节点顺序为 [添加游戏账号 section, 说明段落, 游客批量账号任务折叠] —— **说明段被夹在两个区块之间**，视觉割裂 | ❌ 成立（P1-5 / P2 系列） |

---

## 2. 修复清单

### P0（必须修，直接影响"立体皮肤不显示"）
1. **收起再展开立体重建**：`console.js:165-166` `authSetExpanded` 收起时不要 `dispose` 掉 viewer，或展开时改为 `if(!panel.querySelector('.yy-skin-model'))authBuildDetail(row,panel)`。建议后者最稳：把 `panel.children.length` 判断换成"是否已渲染皮肤节点"。
2. **部署线上 `console.js`**（见第 3 节）。旧版仍在宣传"接口正在核实"。

### P1
1. **头像代理 401**：`console.js:38-43` 的 `yyAvatarSrc` 指向 toolbox `/api/skin/proxy`，跨站拿不到登录态 → 恒 401。二选一：a) 认证站自己加 `/api/avatar-proxy`（worker 侧白名单 host + 缓存）；b) 直接去掉代理，`img.src=官方 URL`，别再白跑一次失败请求。
2. **令牌收敛**：`console.js:16` 下拉框改为"选中后显示「取用令牌」按钮"，不要 onchange 直接取；`showTokenInline` 增加"关闭/隐藏"或在 8s 后自动清空；`worker.mjs:115` 删掉 `token_hint`（前端未用）。
3. **窄屏控件**：给 `.auth-skin-controls` 加 `@media(max-width:420px)`：改成 2 列 grid 或 `justify-content:flex-start` + 允许多行对齐，避免第三组单独居中掉落；`.auth-pose-bar button` 在 `pointer:coarse` 下补 `min-height:36px`（现在 360 下仅 23px）。
4. **改名**："组件码预加载区" → "组件码批量应用"；"加载预览" → "列出待应用组件码"。
5. **皮肤控件统一**：详情 hero 与弹窗共用同一组控件与同一套文案（走动/走路二选一），或详情 hero 只留"展开立体视图"入口。
6. **4399 死路**：要么放开 `login_4399`，要么从 DOM 删除 4399 整段（字段/验证码/两个"打开4399官方注册"按钮）。

### P2
1. 文案去"撤销"歧义：`console.js:16`、`index.html:525` 改成"旧令牌会立即失效"。
2. 合并重复说明：头像 hint + notice 二合一；组件码 hint + notice 二合一（AI 味主要来自这里）。
3. 详情面板默认展开第一个"头像与账号资料"，"令牌与凭据维护"保持折叠（降低点击次数）。
4. `authSkin` 三组控件建议加 `aria-label`（已有）+ 统一高度，避免 52/44px 混排造成视觉不齐（768/1280 实测高度 52 vs 44）。
5. `skin_preview` 失败时（`console.js:178` 的 `catch(e){}` 静默）在 hero 位置给一行"没能读取皮肤纹理"提示，而不是永久留空台。

---

## 3. 是否需要重新部署该 worker

**需要，且只需要重新部署 `youyuan-game-auth`。**

线上与本地逐文件比对（md5）：

| 文件 | 线上 | 本地 | 一致 |
|---|---|---|---|
| index.html | b8169c14… | b8169c14… | ✅ |
| console.css | 6411b0fb… | 6411b0fb… | ✅ |
| auth-entry.css | feafa8ef… | feafa8ef… | ✅ |
| skin-viewer.js | 81e8aded… | 81e8aded… | ✅ |
| return-context.js | cefeb198… | cefeb198… | ✅ |
| **console.js** | 32231 B | 31921 B | ❌ **落后 2 处** |

`console.js` 的 2 处差异（语义 diff，非换行）：
1. 第 15 行：线上还有"官方头像选择接口正在核实，当前不自动指定角色头像。"，本地已删。
2. 第 17 行：线上 `api('/health',null,'')`，本地 `api('/health',null,session||key)`（影响很小：`worker.mjs:33` 的 `/health` 不校验鉴权，实测 200）。

部署方式：`public/` 是 wrangler assets 目录，`console.js` 是纯静态资源，**不需要重新构建 wasm**：

```
cd auth-service && npx wrangler deploy
```

部署后复验（应为 0 命中）：

```
curl -s https://auth.youyuanqi.dpdns.org/console.js | grep -c 正在核实
```

---

## 4. 未验证 / 限制

- 未用真实账号登录，账号列表、真实 `skin_preview` 返回结构按 `worker.mjs` 代码推断，用假数据驱动真实前端代码。
- 真实网易头像域名（`gph.netease.com` 等）在墙内的可达性未实测，只实测了"代理恒 401"这一确定事实。
- 真机（iOS Safari / 微信内置浏览器）未测；360 下 23px 触控按钮在桌面无头 Chrome 模拟触屏得出，真机可能受 `pointer:coarse` 影响略大，但仍不足 44px。
- 未评估后端接口性能、并发与安全（仅前端与呈现层）。
