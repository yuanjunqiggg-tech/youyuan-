(function() {
  let observer = null, scheduled = false;
  const definitions = [
    ["connect", "\u8FDE\u63A5\u4E0E\u8D26\u53F7", "\u670D\u52A1\u5668\u3001\u51ED\u636E\u3001\u4FDD\u6D3B\u4E0E\u591A\u673A\u5668\u4EBA"],
    ["remote", "\u7FA4\u804A\u4E0E\u8FDC\u7A0B\u63A7\u5236", "\u7FA4\u670D\u4E92\u901A\u3001MCP\u4E0E\u516C\u7F51\u8FDE\u63A5"],
    ["jobs", "\u5EFA\u7B51\u4E0E\u521B\u4F5C", "\u5BFC\u5165\u5BFC\u51FA\u3001\u5730\u56FE\u753B\u548C\u4F5C\u4E1A\u9ED8\u8BA4\u503C"],
    ["appearance", "\u754C\u9762\u4E0E\u6587\u4EF6", "\u4E3B\u9898\u3001\u80CC\u666F\u4E0E\u6587\u4EF6\u9009\u62E9"],
    ["system", "\u7EF4\u62A4\u4E0E\u8D44\u6599", "\u63D0\u793A\u6587\u6848\u3001\u6587\u6863\u3001\u4E91\u7AEF\u8D44\u6599\u548C\u7EF4\u62A4"]
  ];
  function category(title) {
    if (/群服|QQ群|MCP|公网|隧道|远控/.test(title)) return "remote";
    if (/服务器连接|验证服务器|账号|机器人|全局抓包|模式维护|保活/.test(title)) return "connect";
    if (/速度|导入|导出|作业|地图画/.test(title)) return "jobs";
    if (/界面|主题|外观|文件选择|背景/.test(title)) return "appearance";
    return "system";
  }
  function saved(key, fallback) {
    try {
      const v = localStorage.getItem("yy-settings-fold:" + key);
      return v === null ? fallback : v === "1";
    } catch {
      return fallback;
    }
  }
  function remember(node, key) {
    node.addEventListener("toggle", () => {
      try {
        localStorage.setItem("yy-settings-fold:" + key, node.open ? "1" : "0");
      } catch {
      }
    });
  }
  function group() {
    var _a, _b;
    const root = document.querySelector("#tab-settings>.content-inner");
    if (!root || curTab !== "settings" || subPageOpen) return;
    let container = root.querySelector(":scope>.yy-settings-groups");
    if (!container) {
      container = document.createElement("div");
      container.className = "yy-settings-groups";
      const grid = root.querySelector(":scope>.yy-form-grid");
      (grid || root.firstElementChild).after(container);
      for (const [id, title, desc] of definitions) {
        const d = document.createElement("details");
        d.className = "yy-settings-group";
        d.dataset.group = id;
        d.open = saved(id, id === "connect" || id === "system");
        const summary = document.createElement("summary"), name = document.createElement("strong"), description = document.createElement("span");
        name.textContent = title;
        description.textContent = desc;
        summary.append(name, description);
        d.append(summary);
        remember(d, id);
        container.append(d);
      }
    }
    const cards = [...root.querySelectorAll(".card")].filter((c) => !c.closest(".yy-settings-groups") && !c.parentElement.closest(".card"));
    for (const card of cards) {
      const title = (((_a = card.querySelector("h3")) == null ? void 0 : _a.textContent) || ((_b = card.querySelector(".yy-form-head")) == null ? void 0 : _b.textContent) || "").trim();
      if (card.id === "yy-settings-save" || card.id === "yy-settings-save-card" || card.querySelector("#yy-settings-save") || !title && card.querySelector('[onclick="saveCfg()"]')) continue;
      const module = document.createElement("details");
      module.className = "yy-settings-module";
      const summary = document.createElement("summary");
      summary.textContent = title || "\u5176\u4ED6\u8BBE\u7F6E";
      module.append(summary);
      const key = card.id || title;
      module.open = saved("module:" + key, card.id === "cfg-conn-card");
      remember(module, "module:" + key);
      module.append(card);
      container.querySelector('[data-group="' + category(title) + '"]').append(module);
    }
    for (const stack of root.querySelectorAll(".yy-form-stack")) if (!stack.children.length) stack.remove();
    for (const grid of root.querySelectorAll(":scope>.yy-form-grid")) if (!grid.children.length) grid.remove();
  }
  const base = Rcfg;
  Rcfg = async function() {
    if (observer) observer.disconnect();
    await base.apply(this, arguments);
    if (curTab !== "settings" || subPageOpen) return;
    group();
    const root = document.getElementById("tab-settings");
    observer = new MutationObserver((records) => {
      if (!records.some((r) => [...r.addedNodes].some((n) => n.nodeType === 1 && (n.matches(".card") || n.querySelector(".card")))) || scheduled) return;
      scheduled = true;
      requestAnimationFrame(() => {
        scheduled = false;
        group();
      });
    });
    observer.observe(root, { childList: true, subtree: true });
    activeRenderers.push(() => {
      observer == null ? void 0 : observer.disconnect();
    });
  };
})();
