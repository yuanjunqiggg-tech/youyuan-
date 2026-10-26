/* 审查用最小桩：只提供真实前端模块运行所需的全局符号，不改动任何项目源码。 */
window.subPageCache = {};
window._savedFormState = {};
window.activeRenderers = [];
window.subPageOpen = false;
window.curTab = "";
window.yyPlatformUser = null;
window.TAB_ORDER = [];
window.tabTo = function () {};
window.yyNavigate = function () {};
window.yyIcon = function (name) {
  return '<svg class="yy-icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/></svg>';
};
window.curTab = "";
window.yyAccountAdmin = function () { return true; };
window.yyRootFetch = function () { return Promise.reject(new Error("offline")); };
window.T = function () {};
window.yyIntro = function (title, sub) {
  return '<div class="yy-intro"><h2>' + title + "</h2><p>" + (sub || "") + "</p></div>";
};
window.yyEscape = function (v) {
  return String(v == null ? "" : v).replace(/[&<>"']/g, function (c) {
    return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
  });
};
window.yyQQE = window.yyEscape;
window.yyQQIconPathsOverride = null;

window.openSubPage = function (title, html) {
  var sp = document.getElementById("sub-page");
  sp.innerHTML = '<div class="sp-header"><button class="sp-back">&larr;</button><span class="sp-title">' + title +
    '</span></div><div class="sp-body">' + html + "</div>";
  sp.className = "sub-page show";
  document.body.classList.add("sp-open");
};

/* 所有接口统一失败：用来验证断网/接口报错时的空状态。 */
window.A = function (method, path) {
  if (window.__OFFLINE__) return Promise.resolve({ ok: false, error: "网络不可用" });
  var data = (window.__API__ || {})[path] || (window.__API__ || {})[path.split("?")[0]];
  if (typeof data === "function") return Promise.resolve(data());
  return Promise.resolve(data || { ok: false, error: "未打桩：" + path });
};
