/* youyuan_console_token.js —— 给所有同源 /api/ 请求自动带控制台会话 token。
 *
 * 为什么用全局 fetch 包装：前端有 44 处直接 fetch('/api/...') 的地方（文件选择器、
 * 地图画、音乐、AI 对话、上传等），逐个改容易漏，漏一个就 401。
 * 这里统一注入 X-YouYuan-Console-Token，A()/sse.js/EventSource 垫片全都覆盖到。
 *
 * token 由宿主 handleIndex 注入到 <meta name="yy-console-token">；拿不到就什么都不做，
 * 保证旧页面/测试环境不会因为缺 token 直接崩。
 */
(function () {
  "use strict";
  var meta = document.querySelector('meta[name="yy-console-token"]');
  if (!meta || !meta.content) return;
  var token = meta.content;
  var original = window.fetch;
  if (!original || original.__yyConsoleTokenWrapped) return;

  window.fetch = function (input, init) {
    try {
      var url = typeof input === "string" ? input : (input && input.url) || "";
      var isAPI = url.indexOf("/api/") === 0 || url.indexOf(location.origin + "/api/") === 0;
      if (isAPI) {
        init = init || {};
        var headers = new Headers(
          init.headers || (input && input.headers) || undefined
        );
        if (!headers.has("X-YouYuan-Console-Token") && !headers.has("Authorization")) {
          headers.set("X-YouYuan-Console-Token", token);
        }
        init.headers = headers;
      }
    } catch (e) {
      /* 注入失败就当普通请求，不阻断业务 */
    }
    return original.call(this, input, init);
  };
  window.fetch.__yyConsoleTokenWrapped = 1;
})();
