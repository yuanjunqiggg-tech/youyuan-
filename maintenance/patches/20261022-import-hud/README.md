# 导入导出显示与启动反馈修复

2026-10-22用户反馈游戏面板太宽、导入显示成导出、点击开始后无明确反馈。本轮只修这些入口和显示，保持之前命令链/导入引擎修复。

- `upstream/task_dispatch.go`：`regionMode`是导入分区模式，不能判断导入/导出；删除错误推断。文件解析前发“导入准备”和明确阶段事件；导出增加标准`type=export`开始事件、准备和读取进度显示。
- `upstream/youyuan_structure_hud.go`：长名截断、指标分行、缩短边框，保留进度/区块/方块/速度/余量/时间；未知TPS不猜，未知总数不写0/0。
- `upstream/youyuan_task_presentation.go`：简洁面板同样拆行，避免导出把名称、进度和指标挤成宽条。
- `upstream/frontend/js/features/building.js`和`sse.js`：请求提交立即反馈，准备阶段同步；运行中开始按钮保持禁用，终止事件先于HTTP回包时不重新锁住；导出完成按钮正确写“重新导出”。

原生实际SHA：`fb435be5a02c1e6f5b62cb5c972dec20ef3d85288c361f7fbd7a14f089141f71`。构建基于前一线上39e973b0的冻结源，只加入明确文件；未使用共享树中未完成QQ改动。备份`/var/backups/youyuan/20261022-203750-import-hud`。主包/building/state和vet通过，Node覆盖提交/拒绝/运行状态/HTTP与终止事件竞态。

真服证据均在项目外层`outputs/`：

- `2026-10-22-导入启动响应与准备阶段真服验收.json`：30区块空气流程完成，实际SSE准备/进度/完成；执行端受理约1.52ms，准备事件约18.39ms，非公网时延承诺。
- `2026-10-22-导出准备及完成事件真服验收.json`：1/1区域、280字节文件，开始/准备/完成事件收到；输出文件、临时任务已清理，原位置、菜单和观测恢复。
- 旧导出task_done未提供task_id，因此按本次唯一输出路径关联；首个不适用断言的失败样本已留证，不冒充没有完成事件。

## 独立网页定点发布

云端现行版本`ce88d0fa-f3e4-47fd-9cdc-38026b2ee954`，原版`ed638de7-dccb-4325-9dde-db261c40e702`可回滚。保留实际已发布`worker.js`字节SHA`8f43b8c4cd396453c76581854365f079e5289dc07935f7d1614cb926dae64a91`和全部绑定，通过小入口包装提供两个压缩JS；根HTML只重写对应脚本地址，其他请求交回原后台。公网两文件SHA和HTML引用已核实，MCP在线读回通过。

包装与发布工具在外层`work/cf-import-hud-20261022/ui-patch.mjs`、`work/publish_hud_web_20261022.py`、`work/cf_hud_target_20261022.py`；本次实际运行的代码/静态集合与本地正在修改的worker.mjs分开，不能盲目用后者覆盖现网。正常完整前端发布后可撤掉包装，源码和public中两份JS、缓存版本已同步，避免未来回退。

**不要为两个JS全量重传`cloudflare/public`：当前有约5.89GB历史资源。** 本次按Cloudflare官方`keep_assets`能力保留整套现有资源，只传约162KB脚本。`keep_assets`连旧路由一起保留，新入口使用原本就转入Worker的`/api/ui-patch/js/...`；首轮旧静态路由未更新已回滚。边缘传播有短暂旧版本，后续使用有界重读验证，不马上重发发布。参考[官方直接上传说明](https://developers.cloudflare.com/workers/static-assets/direct-upload/)。

凭据仅由现有Cloudflare CLI授权配置在RAM中使用，未复制进源码/日志/交接，未扫描其他AI客户端。首次网页发布回滚与最终成功分别保留证据。

实际手机游戏画面尚无截图验收，不能称不同缩放与字体观感都通过；复杂NBT、真实中断续跑、用户公网慢链路和大型建筑吞吐不在这次小样本的证明范围。
