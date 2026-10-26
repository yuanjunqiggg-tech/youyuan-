# 认证站账号控制台修复 · 子 Agent A 进度

## 任务
修复 auth-service 账号页：头像链（玩家自己的 MC 头）、删撤销按钮、立体皮肤自动显示+预加载、组件码预加载区、像素占位。
文件所有权：`auth-service/public/console.js`、`console.css`、`index.html`。不部署。

## 已完成：读源码与接口契约确认
- 记忆 2026-10-23.md 15:00/16:40 节已读（头像根因、auth console 结构）。
- 接口契约（读 engine.go:207-217 / skin_preview.go / worker.mjs:272-273 确认，非猜测）：
  - `POST /api/accounts {action:'skin_preview',id}` → `{ok, texture:dataURL PNG, width, height, custom_geometry, skin_info:{ItemID,SkinIsSlim}, body_type_known, warning}`
  - texture 为标准人形 PNG：64×32（老格式）或 64×64（新格式），也允许 128 等倍率（校验：宽%64==0 且 高==宽 或 高*2==宽）
  - **接口只返回当前账号已选用皮肤的纹理，无按任意组件码取纹理的能力** → 组件码预加载区按用户预案诚实实现
- console.css 全文已内联进 index.html style 块（index.html:76 起），改 CSS 必须两边同步
- skin-viewer.js 接口确认：`new YYSkinViewer(host,texture,slim)`，`walk(bool)/model(bool)/reset()/dispose()`

## 进行中：console.js 代码修改
（下一步逐条 Edit，改完 node --check，再回填每条 before/after）

## 已完成：全部修改 + 验证（2026-10-23）

### 1. 删除「撤销游戏凭据」按钮
- before：authBuildDetail security 段第 4 个按钮「撤销游戏凭据」（confirm 后 action:'delete' 删账号）
- after：只剩 开启/关闭定时刷新、使用此账号令牌、重签游戏令牌 三个按钮（console.js:231）
- 附带：index.html 旧版兜底 refresh()（console.js 加载失败时才走的路径）里的「撤销」按钮数组项和不可达的 delete 分支也删了，两个路径都不再有撤销入口

### 2. 头像链补强（authFillAvatar，console.js:98）
- before：只有 avatar_source==='netease' 且 URL 合法才出图，否则永远圆圈+路径剪影
- after：三级链，任一级成功即显示：
  1. 网易官方头像（原逻辑，yyAvatarSrc 代理→直连回退，双失败才往下走）
  2. 玩家自己的 MC 头：authLoadSkin(row) 走 skin_preview 拿 texture → authMcHeadSrc 离屏 canvas 32×32 裁剪：头 (8,8) 32×32；64×64/128×128 再加 hat 叠加层 (40,8) 逐像素 alpha 混合（仅在有像素处叠加）；64×32/128×64 老格式（naturalHeight*2===naturalWidth）只裁头部；高清按 naturalWidth/64 放大取样。裁剪结果 toDataURL 设为 img.src
  3. 重做的 MC 像素风头形占位（见第 5 点）
- 防重：authSkinCache（Map，row.id→Promise）同一账号只请求一次；首屏外账号用 IntersectionObserver 滚到视口再拉
- 详情页同步：资料区新增大头像（.auth-avatar-large），同一套三级链，展开即有

### 3. 立体皮肤预览自动显示 + 预加载
- before：authBuildDetail 展开时 IIFE 直接 api('skin_preview')；每次展开都重新请求
- after：IIFE 改走 authLoadSkin 共享缓存（console.js:174），预加载过则秒出；authRender 对前 3 个可见账号静默预拉（console.js:50-52，index<3，不渲染 viewer 只温暖缓存）；authSkin 对话框同样走缓存（console.js:235），打开即自动读取显示；应用皮肤成功后 authSkinCache.delete(id) 精准失效，refresh() 整体 clear 兜底

### 4. 皮肤组件码预加载区（console.js:196-225）
- before：只有手动填「皮肤组件码 / 官方分享链接」+「应用已有皮肤」
- after：新增多行文本框（placeholder「每行一个数字组件码」）+「加载预览」按钮。点击后解析组件码（去重、容错分隔符），并排小预览：
  - 第一个瓦片：当前账号皮肤的真实头部（authMcHeadSrc 裁剪，只展示真实接口返回的纹理）
  - 每个组件码一个瓦片：「待应用」+ 应用按钮（action:'skin' → 失效缓存 → refresh → 重渲染预览）
  - UI 明示真实能力：「接口只能读取当前账号已选用皮肤的真实纹理……未应用的组件码不显示皮肤图」——skin_preview 无按码取图能力，不假装

### 5. MC 像素风占位图标
- before：`<svg viewBox="0 0 40 40"><circle/><path/></svg>` 圆圈+人形剪影
- after：authHeadSvg（console.js:94）8bit 像素风 MC 头形：16×16 viewBox，上宽下窄头形轮廓 path + evenodd 掏空双眼/嘴；CSS 加 shape-rendering:crispEdges（console.css:111 及 index.html 内联副本同步）

### 验证
- `node --check console.js` → 通过（node 22.22.2-6）
- index.html 两段内联脚本 `new Function` 语法检查 → 0 错误
- console.css 与 index.html 内联副本逐字节比对 → 一致（仅尾部空行）
- MC 头裁剪探针（Temp/probe_mchead.js，DOM 全桩，10 项全 PASS）：64×64 头+hat+混合、64×32 只裁头、128×128 ×2 取样、128×64 老格式高清、onerror→''
- 残留检查：public/ 三文件无「撤销游戏凭据」按钮、无 delete action、无旧剪影 SVG
- 未动 worker.mjs / account-profile.mjs / engine.go / skin-viewer.js

### 待主 Agent
- 部署（wrangler + Go 侧无需改动，本次纯前端）
- 占位图标后续可换更好的设计资产（用户许可的后续项）
