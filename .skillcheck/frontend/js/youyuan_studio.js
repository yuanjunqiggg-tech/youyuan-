(function() {
  const escape = (s) => typeof yyEscape === "function" ? yyEscape(String(s)) : String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
  function banner(kind, title, description) {
    return '<header class="yy-studio-head"><div><span class="yy-studio-kicker">\u6E38\u56ED / ' + kind + "</span><h2>" + title + "</h2><p>" + description + '</p></div><span class="yy-studio-mark" aria-hidden="true">\u25A6</span></header>';
  }
  function heading(number, title, note) {
    const h = document.createElement("div");
    h.className = "yy-studio-section-title";
    h.innerHTML = "<span>" + number + "</span><div><h3>" + title + "</h3><p>" + note + "</p></div>";
    return h;
  }
  const importBase = RimpHTML;
  RimpHTML = function() {
    const box = document.createElement("div");
    box.innerHTML = importBase.apply(this, arguments);
    const form = box.firstElementChild;
    form.classList.add("yy-studio-config");
    form.prepend(heading("02", "\u653E\u7F6E\u4E0E\u6267\u884C", "\u8BBE\u7F6E\u4F4D\u7F6E\u3001\u5199\u5165\u89C4\u5219\u548C\u53C2\u4E0E\u8D26\u53F7\u3002"));
    const waiting = document.createElement("p");
    waiting.className = "yy-studio-await";
    waiting.textContent = "\u9009\u62E9\u5E76\u89E3\u6790\u6E90\u6587\u4EF6\u540E\uFF0C\u8FD9\u91CC\u5C06\u663E\u793A\u4F4D\u7F6E\u3001\u5199\u5165\u89C4\u5219\u548C\u6267\u884C\u9009\u9879\u3002";
    form.querySelector(".yy-studio-section-title").after(waiting);
    const source = document.createElement("aside");
    source.className = "card yy-studio-source";
    source.append(heading("01", "\u5EFA\u7B51\u6E90\u6587\u4EF6", "\u5148\u9009\u6587\u4EF6\uFF0C\u518D\u67E5\u770B\u89E3\u6790\u7ED3\u679C\u3002"));
    for (const selector of [".pick", "#imp-mode-cap", "#imp-info", "#imp-mcworld-params"]) {
      const el = form.querySelector(selector);
      if (el) source.append(el);
    }
    const layout = document.createElement("div");
    layout.className = "yy-studio-grid";
    form.before(layout);
    layout.append(source, form);
    const wrapper = document.createElement("section");
    wrapper.className = "yy-studio yy-studio-import";
    wrapper.innerHTML = banner("\u5EFA\u7B51\u5199\u5165", "\u628A\u5EFA\u7B51\u5E26\u5165\u4E16\u754C", "\u6587\u4EF6\u3001\u53C2\u6570\u3001\u6267\u884C\u72B6\u6001\u5206\u533A\u5448\u73B0\uFF1B\u5BFC\u5165\u3001\u591A\u673A\u5668\u4EBA\u4E0E\u4FEE\u8865\u529F\u80FD\u5B8C\u6574\u4FDD\u7559\u3002");
    while (box.firstChild) wrapper.append(box.firstChild);
    return wrapper.outerHTML;
  };
  const exportBase = RexpHTML;
  RexpHTML = function() {
    const box = document.createElement("div");
    box.innerHTML = exportBase.apply(this, arguments);
    const form = box.firstElementChild;
    form.classList.add("yy-studio-config");
    const originalTitle = form.firstElementChild;
    if (originalTitle && !originalTitle.id && originalTitle.tagName === "DIV") originalTitle.remove();
    form.prepend(heading("01", "\u6846\u9009\u8BFB\u53D6\u533A\u57DF", "\u8F93\u5165\u4E24\u4E2A\u5BF9\u89D2\u5750\u6807\uFF0C\u4FDD\u7559\u539F\u683C\u5F0F\u548C\u534F\u4F5C\u9009\u9879\u3002"));
    const wrapper = document.createElement("section");
    wrapper.className = "yy-studio yy-studio-export";
    wrapper.innerHTML = banner("\u5EFA\u7B51\u8BFB\u53D6", "\u4FDD\u5B58\u4E16\u754C\u91CC\u7684\u4F5C\u54C1", "\u9009\u62E9\u8303\u56F4\u4E0E\u683C\u5F0F\uFF0C\u5C06\u539F\u59CB\u5EFA\u7B51\u6587\u4EF6\u4FDD\u5B58\u5230\u5F53\u524D\u6267\u884C\u8BBE\u5907\u3002");
    const layout = document.createElement("div");
    layout.className = "yy-studio-export-grid";
    form.before(layout);
    layout.append(form);
    const note = document.createElement("aside");
    note.className = "card yy-studio-guide";
    note.innerHTML = '<span class="yy-studio-kicker">\u5BFC\u51FA\u8BF4\u660E</span><h3>\u4ECE\u4E16\u754C\uFF0C\u5230\u6587\u4EF6\u3002</h3><ol><li>\u786E\u8BA4\u5F53\u524D\u8FDE\u63A5\u7684\u670D\u52A1\u5668\u4E0E\u8D26\u53F7\u3002</li><li>\u586B\u5199\u533A\u57DF\u548C\u6587\u4EF6\u540D\uFF0C\u9009\u62E9\u9700\u8981\u7684\u683C\u5F0F\u3002</li><li>\u7B49\u5F85\u4EFB\u52A1\u5B8C\u6210\u540E\u4E0B\u8F7D\u539F\u6587\u4EF6\u3002</li></ol><p>\u7F3A\u5757\u3001\u65AD\u7EBF\u6216\u5931\u8D25\u65F6\u4EE5\u4EFB\u52A1\u5B9E\u9645\u7ED3\u679C\u4E3A\u51C6\uFF1B\u5F00\u59CB\u4EFB\u52A1\u4E0D\u7B49\u4E8E\u5BFC\u51FA\u5B8C\u6210\u3002</p><button class="btn-s" onclick="yyOpenExportCenter()">\u6253\u5F00\u6587\u4EF6\u4E0B\u8F7D</button>';
    layout.append(note);
    while (box.firstChild) wrapper.append(box.firstChild);
    return wrapper.outerHTML;
  };
  const pluginsBase = Rplg;
  Rplg = async function() {
    var _a, _b, _c, _d;
    await pluginsBase.apply(this, arguments);
    const root = document.querySelector("#tab-tasks>.content-inner");
    if (!root || root.dataset.yyStudio === "1") return;
    root.dataset.yyStudio = "1";
    root.classList.add("yy-plugin-studio");
    const oldTitle = root.querySelector(":scope>.yy-heading");
    if (oldTitle) oldTitle.remove();
    root.insertAdjacentHTML("afterbegin", banner("\u6269\u5C55\u5DE5\u4F5C\u53F0", "\u63D2\u4EF6\uFF0C\u5404\u5C31\u5176\u4F4D\u3002", "\u672C\u5730\u7BA1\u7406\u4E0E\u5E02\u573A\u6765\u6E90\u5206\u5F00\uFF0C\u8FD0\u884C\u72B6\u6001\u548C\u64CD\u4F5C\u7ED3\u679C\u59CB\u7EC8\u53EF\u89C1\u3002"));
    const cards = root.querySelector(".yy-workspace-cards"), head = cards == null ? void 0 : cards.previousElementSibling;
    if (cards && head) {
      const group = document.createElement("details");
      group.className = "yy-studio-group";
      group.open = true;
      const summary = document.createElement("summary");
      summary.textContent = "\u5F00\u53D1\u5DE5\u4F5C\u533A";
      head.before(group);
      group.append(summary, head, cards);
    }
    const runtime = ((_a = root.querySelector(".yy-installed-runtime")) == null ? void 0 : _a.closest("section.card")) || Array.from(root.children).find((e) => e.tagName === "SECTION" && e.querySelector("#yy-runtime-result"));
    if (runtime) {
      const d = document.createElement("details");
      d.className = "yy-studio-group";
      d.open = true;
      d.innerHTML = "<summary>\u8FD0\u884C\u73AF\u5883\u4E0E\u5DF2\u5B89\u88C5\u5B9E\u4F8B</summary>";
      runtime.before(d);
      d.append(runtime);
    }
    const compat = root.querySelector(".yy-compat");
    if (compat) {
      const d = document.createElement("details");
      d.className = "yy-studio-group";
      d.open = true;
      d.innerHTML = "<summary>Lua \u4E0E\u517C\u5BB9\u63D2\u4EF6</summary>";
      compat.before(d);
      d.append(compat);
      (_b = compat.querySelector(":scope>h3")) == null ? void 0 : _b.remove();
    }
    const bar = document.createElement("section");
    bar.className = "yy-plugin-updates card";
    bar.innerHTML = '<div class="yy-section-head"><div><h3>\u6765\u6E90\u4E0E\u66F4\u65B0</h3><p class="yy-hint">\u53EA\u68C0\u67E5\u4F60\u5DF2\u5173\u8054\u4E14\u6709\u6743\u8BBF\u95EE\u7684\u6765\u6E90\uFF1B\u68C0\u67E5\u4E0D\u4F1A\u5B89\u88C5\u6216\u8986\u76D6\u63D2\u4EF6\u3002</p></div><button class="btn-s" id="yy-plugin-check" onclick="yyCheckPluginUpdates()">\u68C0\u67E5\u66F4\u65B0</button></div><label class="yy-update-switch"><input id="yy-plugin-auto-check" type="checkbox" onchange="yyPluginAutoCheck(this.checked)">\u8FDB\u5165\u63D2\u4EF6\u9875\u65F6\u81EA\u52A8\u68C0\u67E5\u66F4\u65B0</label><p id="yy-plugin-update-result" role="status" class="yy-hint"></p><div class="yy-actions"><button class="btn-s" onclick="yyLibraryPage()">\u6211\u7684\u63D2\u4EF6\u5E02\u573A</button><button class="btn-s" onclick="pmOpenMarket()">\u5916\u90E8\u63D2\u4EF6\u5E02\u573A</button><button class="btn-s" onclick="yyPluginSourceDialog()">\u5173\u8054\u5DF2\u5B89\u88C5\u63D2\u4EF6\u6765\u6E90</button></div>';
    (_c = root.querySelector(".yy-plugin-header")) == null ? void 0 : _c.after(bar);
    if (!bar.parentElement) root.append(bar);
    const pref = readPrefs();
    document.getElementById("yy-plugin-auto-check").checked = pref.auto === true;
    for (const group of root.querySelectorAll(".yy-studio-group")) {
      const key = group.querySelector(":scope>summary").textContent;
      group.open = typeof ((_d = pref.folds) == null ? void 0 : _d[key]) === "boolean" ? pref.folds[key] : key === "Lua \u4E0E\u517C\u5BB9\u63D2\u4EF6";
      group.addEventListener("toggle", () => {
        if (!group.isConnected) return;
        const p = readPrefs();
        p.folds = p.folds || {};
        p.folds[key] = group.open;
        savePrefs(p);
      });
    }
    if (pref.auto === true && Date.now() - (pref.lastCheck || 0) > 15 * 60 * 1e3) yyCheckPluginUpdates();
  };
  function prefKey() {
    const user = typeof yyPlatformUser === "object" && yyPlatformUser ? yyPlatformUser : {};
    const account = typeof yyActiveAccount === "string" ? yyActiveAccount : "main";
    let device = "local";
    try {
      device = new URL(location.href).searchParams.get("device") || "local";
    } catch {
    }
    return "yy-plugin-updates:" + JSON.stringify([device, account, String(user.id || user.username || "local-owner")]);
  }
  function readPrefs() {
    try {
      const r = JSON.parse(localStorage.getItem(prefKey()) || "{}");
      return r && typeof r === "object" ? r : {};
    } catch {
      return {};
    }
  }
  function savePrefs(p) {
    try {
      localStorage.setItem(prefKey(), JSON.stringify(p));
      return true;
    } catch {
      T("\u6B64\u8BBE\u5907\u65E0\u6CD5\u4FDD\u5B58\u66F4\u65B0\u504F\u597D", "e");
      return false;
    }
  }
  window.yyPluginAutoCheck = function(on) {
    const p = readPrefs();
    p.auto = on;
    savePrefs(p);
    if (on) yyCheckPluginUpdates();
  };
  window.yyPluginRecordSource = function(id, sourceID, origin) {
    if (!id || !sourceID || !origin) return;
    const p = readPrefs();
    p.sources = p.sources || {};
    p.sources[id] = { kind: "external", id: String(sourceID), origin };
    savePrefs(p);
  };
  window.yyPluginSourceDialog = async function() {
    const owner = yyReadScope(), r = await A("GET", "/api/plugin/list");
    if (owner !== yyReadScope()) return;
    if (!r.ok) {
      T(r.error || "\u63D2\u4EF6\u5217\u8868\u8BFB\u53D6\u5931\u8D25", "e");
      return;
    }
    const p = readPrefs(), rows = r.plugins || [], overlay = document.createElement("div");
    overlay.className = "modal-overlay";
    overlay.innerHTML = '<div class="modal-box yy-studio-source-dialog"><h3>\u5173\u8054\u66F4\u65B0\u6765\u6E90</h3><p class="yy-hint">\u9009\u62E9\u5DF2\u5B89\u88C5\u63D2\u4EF6\uFF0C\u518D\u586B\u5199\u6388\u6743\u5E02\u573A\u4E2D\u7684\u6570\u5B57\u63D2\u4EF6ID\u3002\u6B64\u8BB0\u5F55\u53EA\u4FDD\u5B58\u5728\u5F53\u524D\u8BBE\u5907\u548C\u8D26\u53F7\u89C6\u56FE\uFF0C\u4E0D\u5305\u542B\u4EE4\u724C\uFF0C\u4E0D\u4F1A\u81EA\u52A8\u5B89\u88C5\u3002</p><label>\u5DF2\u5B89\u88C5\u63D2\u4EF6<select id="yy-source-plugin">' + rows.map((p2) => '<option value="' + escape(p2.id) + '">' + escape(p2.name || p2.id) + "</option>").join("") + '</select></label><label>\u5E02\u573A\u63D2\u4EF6ID<input id="yy-source-market" inputmode="numeric" placeholder="\u4F8B\u5982 123"></label><p class="yy-hint" id="yy-source-result"></p><div class="yy-actions"><button class="btn" id="yy-source-save">\u4FDD\u5B58\u6765\u6E90</button><button class="btn-s" id="yy-source-remove">\u53D6\u6D88\u5173\u8054</button><button class="btn-s" id="yy-source-close">\u5173\u95ED</button></div></div>';
    document.body.append(overlay);
    const plugin = overlay.querySelector("#yy-source-plugin"), input = overlay.querySelector("#yy-source-market");
    const fill = () => {
      var _a, _b;
      return input.value = ((_b = (_a = p.sources) == null ? void 0 : _a[plugin.value]) == null ? void 0 : _b.id) || "";
    };
    fill();
    plugin.onchange = fill;
    overlay.querySelector("#yy-source-save").onclick = () => {
      if (owner !== yyReadScope()) {
        overlay.remove();
        return;
      }
      if (!plugin.value || !/^\d{1,15}$/.test(input.value.trim()) || !r.market_origin) {
        overlay.querySelector("#yy-source-result").textContent = "\u8BF7\u9009\u62E9\u63D2\u4EF6\uFF0C\u586B\u51991\uFF5E15\u4F4D\u6570\u5B57\u5E02\u573AID\uFF0C\u5E76\u786E\u8BA4\u5F53\u524D\u5E02\u573A\u5DF2\u914D\u7F6E\u3002";
        return;
      }
      yyPluginRecordSource(plugin.value, input.value.trim(), r.market_origin);
      overlay.remove();
      T("\u6765\u6E90\u5DF2\u5173\u8054\uFF1B\u53EF\u68C0\u67E5\u7248\u672C\u5E76\u6253\u5F00\u539F\u5E02\u573A\u6838\u5BF9\u3002", "o");
    };
    overlay.querySelector("#yy-source-remove").onclick = () => {
      if (owner === yyReadScope()) {
        const now = readPrefs();
        if (now.sources) delete now.sources[plugin.value];
        savePrefs(now);
      }
      overlay.remove();
    };
    overlay.querySelector("#yy-source-close").onclick = () => overlay.remove();
  };
  let updateBusy = false;
  window.yyCheckPluginUpdates = async function() {
    var _a, _b, _c;
    const result = document.getElementById("yy-plugin-update-result"), button = document.getElementById("yy-plugin-check");
    if (!result || updateBusy) return;
    const owner = yyReadScope(), p = readPrefs();
    updateBusy = true;
    if (button) button.disabled = true;
    result.textContent = "\u6B63\u5728\u68C0\u67E5\u5173\u8054\u6765\u6E90\u2026";
    const live = () => owner === yyReadScope() && document.getElementById("yy-plugin-update-result") === result;
    try {
      const r = await A("GET", "/api/plugin/list");
      if (!live()) return;
      if (!r.ok) throw Error(r.error || "\u8BFB\u53D6\u672C\u5730\u63D2\u4EF6\u5931\u8D25");
      const rows = r.plugins || [], lines = [];
      for (const plugin of rows) {
        const source = (_a = p.sources) == null ? void 0 : _a[plugin.id];
        if (!source || source.kind !== "external" || !/^\d{1,15}$/.test(String(source.id))) continue;
        if (!r.market_origin || source.origin !== r.market_origin) {
          const warning = document.createElement("div");
          warning.textContent = (plugin.name || plugin.id) + "\uFF1A\u5E02\u573A\u6765\u6E90\u5DF2\u53D8\u66F4\uFF0C\u8BF7\u91CD\u65B0\u5173\u8054\uFF0C\u672A\u5411\u65B0\u5730\u5740\u53D1\u9001\u65E7\u6765\u6E90\u67E5\u8BE2\u3002";
          lines.push(warning);
          continue;
        }
        const currentOrigin = await A("GET", "/api/plugin/list");
        if (!live()) return;
        if (!currentOrigin.ok || currentOrigin.market_origin !== source.origin) {
          const warning = document.createElement("div");
          warning.textContent = (plugin.name || plugin.id) + "\uFF1A\u6765\u6E90\u72B6\u6001\u5DF2\u53D8\u66F4\u6216\u4E0D\u53EF\u786E\u8BA4\uFF0C\u8BF7\u91CD\u65B0\u68C0\u67E5\u3002";
          lines.push(warning);
          continue;
        }
        const info = await APluginMarket("GET", "/api/plugin-market/files/" + encodeURIComponent(source.id), null, { "X-YouYuan-Market-Origin": source.origin });
        if (!live()) return;
        const line = document.createElement("div");
        line.className = "yy-update-row";
        const text = document.createElement("span");
        line.append(text);
        if (!info.ok) {
          text.textContent = (plugin.name || plugin.id) + "\uFF1A\u6765\u6E90\u4E0D\u53EF\u7528\u6216\u672A\u83B7\u6388\u6743\uFF0C\u672A\u68C0\u67E5\u6210\u529F\u3002";
        } else {
          const release = (_b = info.versions) == null ? void 0 : _b[0], version = (release == null ? void 0 : release.version) || ((_c = info.file) == null ? void 0 : _c.version);
          const known = typeof version === "string" && version !== "";
          text.textContent = (plugin.name || plugin.id) + " \xB7 \u5F53\u524D " + (plugin.version || "\u672A\u58F0\u660E\u7248\u672C") + " \u2192 \u6765\u6E90 " + (known ? version : "\u672A\u58F0\u660E\u7248\u672C") + (known && plugin.version === version ? " \xB7 \u7248\u672C\u4E00\u81F4" : known && plugin.version ? " \xB7 \u7248\u672C\u4E0D\u540C\uFF0C\u8BF7\u6838\u5BF9\u66F4\u65B0\u8BF4\u660E" : " \xB7 \u65E0\u6CD5\u81EA\u52A8\u6BD4\u8F83");
          if (release == null ? void 0 : release.changelog) {
            const small = document.createElement("small");
            small.textContent = String(release.changelog).slice(0, 500);
            line.append(small);
          }
          const open = document.createElement("button");
          open.className = "btn-s";
          open.textContent = "\u67E5\u770B\u6765\u6E90";
          open.onclick = async () => {
            if (owner !== yyReadScope()) return;
            const current2 = await A("GET", "/api/plugin/list");
            if (owner !== yyReadScope()) return;
            if (!current2.ok || current2.market_origin !== source.origin) {
              T("\u5E02\u573A\u6765\u6E90\u5DF2\u53D8\u66F4\uFF0C\u8BF7\u91CD\u65B0\u68C0\u67E5\u66F4\u65B0", "w");
              return;
            }
            pmOpenDetail(Number(source.id), source.origin);
          };
          line.append(open);
        }
        lines.push(line);
      }
      if (!live()) return;
      result.replaceChildren();
      if (!lines.length) result.textContent = "\u5C1A\u672A\u5173\u8054\u66F4\u65B0\u6765\u6E90\u3002\u672C\u5730\u5BFC\u5165\u7684\u63D2\u4EF6\u4E0D\u4F1A\u88AB\u731C\u6D4B\u5339\u914D\uFF1B\u8BF7\u5148\u5173\u8054\u6765\u6E90\uFF0C\u6216\u5230\u5E02\u573A\u67E5\u770B\u3002";
      else result.append(...lines);
      const current = readPrefs();
      current.lastCheck = Date.now();
      savePrefs(current);
    } catch (e) {
      if (live()) result.textContent = "\u68C0\u67E5\u5931\u8D25\uFF1A" + e.message;
    } finally {
      updateBusy = false;
      if (live() && button) button.disabled = false;
    }
  };
  const uploadBase = pick;
  window.yyStudioUploadFile = function(type) {
    return uploadBase(type === "zip" ? "plugin" : type);
  };
  const pickerBase = showFilePicker;
  showFilePicker = function(path, filter, select, mode, cancel) {
    const native = typeof android !== "undefined" && typeof android.pickFile === "function";
    return pickerBase(native ? path : path === "/storage/emulated/0" ? "" : path, filter, select, mode, cancel);
  };
  fpicker = function(type) {
    if ((type === "building" || type === "skin") && typeof showFilePicker === "function") return new Promise((resolve) => showFilePicker("/storage/emulated/0", type, resolve, null, () => resolve("")));
    return uploadBase(type);
  };
  pickBySetting = function(type, callback, mode) {
    showFilePicker("/storage/emulated/0", type, callback, mode);
  };
  const qqBase = yyQQPage;
  yyQQPage = async function() {
    var _a;
    await qqBase.apply(this, arguments);
    const panel = document.getElementById("yy-qq-assistant-panel");
    if (!panel || document.getElementById("yy-qq-memory-panel")) return;
    const owner = yyReadScope(), r = await Promise.all([A("GET", "/api/qq/config"), A("GET", "/api/ai/models"), A("GET", "/api/qq/groups")]);
    if (owner !== yyReadScope() || panel !== document.getElementById("yy-qq-assistant-panel")) return;
    if (r.some((x) => !x.ok)) return;
    await yyQQMemoryMount(((_a = r[0].config) == null ? void 0 : _a.assistant) || {}, r[1].models || [], r[2].groups || []);
  };
})();
