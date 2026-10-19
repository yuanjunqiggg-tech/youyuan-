# 源码与新版APK对照（2026-10-16）

原有开源源码一直保留在 upstream 与第三方模块中；本轮在原有实现上修复，不用反编译结果替换完整源码。新APK为用户提供的 beta.12/132，原附件SHA256为8148245ba83bf55cf980cb1d56a8416698be0762559e0db814d7fef0e961b8cd。

| 范围 | 源码依据 | 新APK/网页证据 | 本轮处理 |
|---|---|---|---|
| 普通导入/导出与NBT工作台 | upstream/building、multi_bot.go、flowers-for-machines/nbt_assigner | APK内嵌building_mgmt.js/building.js、ELF中的SkullType/SkullNBT类型字段 | 保留工作台/结构缓存流程，新增头颅类型；0.10.65七种头颅逐项真实读回，且文件导出再导入通过；墙面方向和自定义皮肤头另验 |
| 分区与断点 | building/pool.go、state/checkpoint.go、runImportTask | 原工程已有单机断点与多机调度，不是从零写空壳 | 修复尾刷/重启状态/多任务覆盖；新增多机分区断点、原文件与布局校验、实际机器人多选 |
| 连接 | auth-service/domain.go与FunAuth/g79client | 官方房间room_id和山头列表 | #数字精确房间ID、邀请码_序号、单连接框；最终真实房间进服稳定性另行验收 |
| 皮肤预览 | 官方组件纹理与体型API | REA浏览器采集+实际账号页；AccountSheet-BtiQZSs5.js经REA静态分析，采用CSS六面、关节pivot思路 | 自行实现CSS3D六面模型、透明外层、肩胯关节、交替步态、站立/走路/体型/复位；不照搬其品牌、账号数据或收费逻辑 |
| 建筑市场 | handleMarketProxy/tryDecryptMarketList 与新版同路径前端 | 真实分类13项可读，files接口仅返回success:true；插件市场可正常列出 | 尚未解决：不能拿空成功响应冒充正常文件列表，也不能假定缺失私钥已经还原 |

REA静态取证不替代真服操作；Go原生库未完成反编译的部分保留未知。公开交付不附私人凭据、原始账号库、第三方网页源码或本项目完整宿主源码。开发文档和SDK保留必要调用说明。
