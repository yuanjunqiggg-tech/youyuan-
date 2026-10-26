# MCORT 主题 / 挂件 / 玩法插件框架（DeepSeek harness 式）

> 用户 2026-10-26 原话：「像 DeepSeek harness 一样，可以通过插件来改背景和挂件……只要不把 MCORT 标识去掉、不把疣螈隐藏……之后再搞一个网址我点进去就可以发公告……他们也可以外接几个新玩法。」
> 本文是这套框架的说明书 + 部署步骤。**框架代码已经存在，不是从零造。**

## 一、现在已经有什么

| 能力 | 位置 | 状态 |
|---|---|---|
| 主题 manifest / 校验 / 品牌守卫 | `upstream/youyuan_theme.go` | ✅ |
| 主题装载与变量装配 | `upstream/frontend/js/mcort_theme.js` | ✅ |
| 品牌署名（硬编码层） | `mcort_theme.js` 的 `ensureBrand/guardBrand` | ✅ 内联 `!important` + MutationObserver |
| 公告存储 + 管理员发布 | `youyuan_theme.go` 的 `yyAnnouncementStore` / `yyThemeAnnouncements` | ✅ |
| 公告页 | 浏览器打开 `/#announce` | ✅ 查看 + 管理员发布 |
| 插件发现 / 启用记录 | `yyThemeScanPlugins` + `/api/mcort/plugins` | ✅ 只发现，**不执行** |
| 示例主题 | `upstream/samples/themes/mcort-aurora/` | ✅ 本轮新增 |
| 示例玩法插件 | `upstream/samples/plugins/seed-query/` | ✅ 本轮新增（占位，不执行） |
| 示例公告 | `upstream/samples/announcements.json` | ✅ 本轮新增 |

路由（`youyuan_theme.go:712`，已在 `youyuan_remote.go:173` 注册）：

- `GET  /api/mcort/themes` —— 主题列表 + 当前启用 + 品牌行
- `GET  /api/mcort/theme` —— 当前主题详情；`POST {id}` 切换（空串 = 回默认）
- `GET  /api/mcort/theme/asset?theme=<id>&file=theme.css` —— 主题资源
- `GET  /api/mcort/announcements` —— 公告列表；`POST {op:create|update|delete}` 需管理员
- `GET  /api/mcort/plugins` —— 插件列表；`POST {id,enabled}` 记录启用

## 二、目录结构

```
<dataDir>/
  themes/
    <id>/
      theme.json      # 必需
      theme.css       # 必需
      theme.js        # 可选，只打包审阅，不执行
      assets/         # 可选，图片等
  plugins/
    <id>/
      plugin.json     # 必需
      main.js         # 入口；当前不执行
  announcements.json  # {"version":1,"items":[...]}
```

`dataDir` 在 box 上是 `/var/lib/youyuan`。

## 三、主题 manifest 规则（`yyThemeValidate`）

必须全部满足，否则主题判为不可用：

- `id` 只能小写字母/数字/`.-_`，且**必须等于目录名**
- `name` / `version` 非空
- `mcort_theme_api` 必须为 `1`
- `requires` 只能取：`theme-core` / `brand-guard` / `announcements` / `plugins` / `qq-console`
- `remove_brand: true` → **拒绝加载**（MCORT 标识属硬编码层）
- `hide_author: true` → **拒绝加载**（作者署名不可隐藏）
- `variables` 的 key 必须以 `--mcort-theme-` 开头，且不能含 `brand` / `author`；value 不能含 `< > { } ; url( @import \`
- `slots` 只有 `header` / `sidebar_footer` / `floating`，值只能是纯文本或 `assets/` 路径，≤160 字，不允许 HTML
- `theme.css` / `theme.js` 不能出现品牌节点标记（`mcort-brand` / `yy-brand-mark` 等），不能写 `display:none` / `visibility:hidden` / `opacity:0` 去藏品牌

品牌署名由前端 `ensureBrand()` 以 inline `!important` 固定，并挂 MutationObserver 补回；主题无法移除。

## 四、插件 manifest 规则

```json
{
  "id": "seed-query",
  "name": "种子查询器",
  "version": "1.0.0",
  "author": "疣螈",
  "description": "……",
  "entry": "main.js",
  "mcort_plugin_api": 1,
  "capabilities": ["panel", "api"]
}
```

- `id` 必须等于目录名
- `mcort_plugin_api` 必须为 `1`
- 入口文件必须存在
- **执行层默认关闭**：`yyThemePluginWhitelist` 为空，所有插件只记录 `enabled`，`executed` 恒为 `false`

### 要真正执行外接玩法，必须先做

1. 宿主提供**受限 API**（只读接口 + 写操作二次确认），插件不碰 DOM / 文件系统；
2. 在 `yyThemePluginWhitelist` 显式加入插件 id；
3. 最好跑在 sandbox iframe 或 worker 里，崩了不拖垮主程序。

在这之前，种子查询等玩法仍走主程序已有接口，插件目录只作为“登记 + 展示”。

## 五、部署步骤（待用户确认后再做）

```bash
# 1) 拷示例内容到 box 数据目录（不覆盖已有）
ssh root@47.102.156.125
mkdir -p /var/lib/youyuan/themes /var/lib/youyuan/plugins
cp -r /path/to/samples/themes/mcort-aurora /var/lib/youyuan/themes/
cp -r /path/to/samples/plugins/seed-query  /var/lib/youyuan/plugins/
# 公告：已有 announcements.json 就合并 items，不要直接覆盖
[ -f /var/lib/youyuan/announcements.json ] || cp /path/to/samples/announcements.json /var/lib/youyuan/announcements.json

# 2) 重启（主题只在启动时扫一次，yyThemeRoutes 里 yyThemeLoad(dataDir)）
systemctl restart youyuan.service

# 3) 验证
curl -s 127.0.0.1:19429/api/mcort/themes | head -c 400   # themes 应含 mcort-aurora
curl -s 127.0.0.1:19429/api/mcort/announcements | head -c 300
# 浏览器打开 /#announce，确认公告可见、管理员可发布
# 页面左下角必须一直有「MCORT · 作者 疣螈」
```

## 六、已知限制

1. **主题只在启动时扫描**：后放进去的主题要重启才出现。要热加载需加 `POST /api/mcort/themes {op:"reload"}`。
2. **插件不执行**：白名单为空，`executed` 恒 false。这是有意的安全设计，不是 bug。
3. 主题不能改品牌；`theme.js` 只打包审阅，不执行。
4. 公告写入需要管理员；本机回环请求默认算管理员。

## 七、给插件作者的最小示例

见 `upstream/samples/themes/mcort-aurora/`（主题）与 `upstream/samples/plugins/seed-query/`（玩法插件）。复制目录、改 `id` 与目录名一致、改配色即可。
