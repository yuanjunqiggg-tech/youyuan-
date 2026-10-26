/* 种子查询器 · 玩法插件入口（示例）
 *
 * 现状：MCORT 的插件执行层默认关闭（youyuan_theme.go 的 yyThemePluginWhitelist 为空）。
 * 这个文件只证明目录结构可用，不会被宿主执行。
 *
 * 要真正执行，必须两步：
 *   1) 在 yyThemePluginWhitelist 里显式加入 "seed-query"；
 *   2) 宿主提供受限 API（只读接口 + 明确的写操作二次确认），插件不碰 DOM、不碰文件系统。
 *
 * 在此之前，种子查询仍走主程序已有的 /api/seed/v2/* 与前端页面。
 */
(function () {
  "use strict";
  // 不会被执行的占位入口：保留给未来的沙箱执行层。
  window.MCORT_PLUGIN_SEED_QUERY = {
    id: "seed-query",
    api: 1,
    panels: ["seed-query"],
    note: "waiting for host sandbox + whitelist"
  };
})();
