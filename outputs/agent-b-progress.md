# Agent-B 调研进度（GitHub 开源调研 + 占位图标设计参考）

## [进行中] 阶段0：上下文读取
- 已读记忆 2026-10-23.md：项目背景=游园（MC联机工具箱+网易账号体系）；头像相关历史：x19.gph.netease.com 皮肤头像 CDN、/api/skin/proxy 白名单已放行 *.gph.netease.com。
- 已定位 console.js:92 禁用按钮「选择网易官方头像 · 接口核实中」（auth-service/public/console.js）。
- 已读 engine.go:184-220：action=skin 时 SetUserSettingList(itemID) 仅做选择不购买；currentAccountSkin 用 GetUserSettingList。
- 已读 third_party/funauth（=github.com/Yeah114/FunAuth  vendored 版）：g79client 模块全部接口清单已过一遍。
- 本地结论初判：GetUserSettingList 只返回当前皮肤 item_id；SearchMcGameItems 需已知 item_id；未见「列出账号已拥有头像/皮肤列表」接口；user_get_detail 有 head_image/avatar_image_url（当前头像）。
- 代理 http://127.0.0.1:9674 可用（GitHub API 通）。

## 下一步
- GitHub 线上调研：确认 FunAuth/g79client 仓库、找网易MC头像/皮肤列表接口开源实现、x19 URL 规律。

## [已完成] 阶段1：GitHub 接口调研
- 确认仓库：yeah-god/FunAuth（19★, fork of Mono2023-PRC/FunAuth，项目已 vendored 于 third_party/funauth）；UCKETX/g79client（6★）；Koud-Wind/Netease-minecraft-LAN-connects-to-Server（59★, 协议层皮肤Mod）；Hazelmeow/neomega-core（协议库）；XiYueMax/SiyoX（Xposed，无头像接口）。
- 枚举 funauth 全部 50+ 个 API 路径：与头像/皮肤相关的只有 pe-get-user-setting-list（当前皮肤 item_id+is_slim）、pe-set-user-setting-list（设皮肤，仅选择）、item-buy-list/query/search-mcgame-items（需已知 item_id 查商城组件）、pe-user-detail/get（当前 head_image/avatar_image_url/frame_id）、pe-download-item/get-download-info（res_url 落 gph.netease.com CDN）、user-item-purchase（购买）。
- 多路代码搜索（GitHub repo API、grep.app、searchcode、WebSearch 中英文 6 组关键词）：**未找到任何「列出账号已拥有头像/皮肤列表」的公开实现**。
- 关键结论：x19.gph.netease.com 的 URL 不是拼接出来的，是 pe-download-item/get-download-info 接口返回的 res_url 完整地址；本地 officialSkinURL 白名单已放行 *.netease.com/*.163.com。

## [已完成] 阶段2：设计调研 + 渲染验证
- 设计参考：mineatar-io/skin-render（51★, RenderHead 等距三面构图）、NickAcPT/nmsr-rs（116★, mc-heads.net 同源渲染器）、Mineatar RenderFace / mc-heads.net 8×8 五官布局。
- 下载 Mineatar/mc-heads 实际渲染图，用纯 Python（zlib 手写 PNG 解码 + 扫描线 evenodd 光栅化 + src-over 合成）建立「投影→SVG→光栅→PNG→目检」闭环。
- 踩修的关键 bug：光栅背景 alpha 初始化为 0 导致所有 opacity 渲染成实色（此前明度验证全部无效）；改为 1.0 后重新验证。
- 定稿三候选：A=等距三面头（762B，顶 0.95/正 0.68 挖双眼嘴/侧 0.38）；B=8×8 正面像素脸（554B）；C=方头+阶梯肩（684B）。浅/深主题 × 96px 全部通过；A 在 32px/40px 五官可读（眼宽从 1px 改 2px、嘴下移一行对齐 Steve 布局；脸部朝向修正为脸朝右前、暗侧在左）。

## [已完成] 阶段3：三份产出全部落盘
1. `outputs/github-netease-interface-research.md`——接口调研报告：5 个相关仓库（含 GitHub 链接）、7 个头像/皮肤相关接口逐个分析（URL/参数/返回样例/源码位置/可信度）、明确结论「不存在头像列表公开接口，不编造」、头像 URL 无拼接规律只能接口取、按钮处置三方案（A 改文案 / B 展示当前头像 / C 手动 item_id 仅皮肤）。全部 URL 于 2026-10-23 经 GitHub API 实测存在。
2. `outputs/avatar-placeholder-design.md`——设计文档：借鉴来源表、设计思路（currentColor 三档 opacity + evenodd 镂空五官）、候选 A/B/C 三个内联 SVG、推荐 A 及 5 条理由、接入建议（≥32px 显示、40px 最佳）、验证记录。
3. `outputs/assets/avatar-placeholder.svg`——推荐款 A 单独文件，763 字节，XML 校验通过，与已验证渲染版本逐字节一致（仅差末尾换行）。
- 任务完毕：未改动任何项目代码（仅 outputs 目录新增 2 个 md + 1 个 svg）。
