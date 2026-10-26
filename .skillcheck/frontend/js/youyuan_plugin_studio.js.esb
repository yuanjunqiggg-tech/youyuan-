(function() {
  "use strict";
  const labels = { lua: "Lua", wordbank: "\u8BCD\u5E93", python: "Python", native_cpp: "C++" };
  const states = { running: "\u8FD0\u884C\u4E2D", waiting: "\u7B49\u5F85\u8FDE\u63A5", stopped: "\u672A\u542F\u52A8", disabled: "\u5DF2\u505C\u7528", error: "\u5F02\u5E38", workspace: "\u5F00\u53D1\u4E2D" };
  const scopes = /* @__PURE__ */ new Map();
  let sequence = 0;
  const pendingClicks = /* @__PURE__ */ new Set();
  const actionIcons = { create: "plus", imports: "upload", market: "globe", refresh: "loop", directory: "folder", runtimeDirectory: "folder", backup: "file", runtimeExport: "file", delete: "trash", runtimeDelete: "trash", reload: "loop", chainTool: "next", downloads: "folder" };
  const icon = (name) => typeof ICONS === "object" && ICONS[name] ? '<span class="yps-icon" aria-hidden="true">' + ICONS[name] + "</span>" : "";
  const esc = (value) => String(value == null ? "" : value).replace(/[&<>"']/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[ch]);
  const scope = () => yyReadScope();
  const admin = () => typeof yyAccountAdmin === "function" ? yyAccountAdmin() : typeof yyPlatformUser === "object" && (yyPlatformUser == null ? void 0 : yyPlatformUser.role) === "admin";
  function errorHint(error) {
    if (/权限|permission|forbidden|unauthorized/i.test(error)) return "\u6682\u65E0\u64CD\u4F5C\u6743\u9650\uFF0C\u8BF7\u8054\u7CFB\u7BA1\u7406\u5458\u5F00\u901A\u540E\u91CD\u8BD5\u3002";
    if (/连接|离线|network|timeout|超时/i.test(error)) return "\u6682\u65F6\u65E0\u6CD5\u8FDE\u63A5\uFF0C\u8BF7\u68C0\u67E5\u8FDE\u63A5\u72B6\u6001\u540E\u5237\u65B0\u91CD\u8BD5\u3002";
    return "\u6682\u65F6\u65E0\u6CD5\u5B8C\u6210\uFF0C\u8BF7\u68C0\u67E5\u63D2\u4EF6\u914D\u7F6E\u540E\u91CD\u8BD5\u3002";
  }
  function diagnostics(error, id) {
    return admin() ? '<details class="yps-doc"><summary>\u7BA1\u7406\u5458\u8BCA\u65AD</summary><pre>' + esc(id ? "\u63D2\u4EF6\uFF1A" + id + "\n" : "") + esc(error) + "</pre></details>" : "";
  }
  function view() {
    const key = scope();
    if (!scopes.has(key)) {
      if (scopes.size >= 8) scopes.delete(scopes.keys().next().value);
      scopes.set(key, { query: "", language: "all", state: "all", source: "all", group: "state", open: /* @__PURE__ */ new Set(), rows: [], data: null });
    }
    return scopes.get(key);
  }
  function preferences() {
    const user = typeof yyPlatformUser === "object" && yyPlatformUser ? yyPlatformUser : {};
    let device = "local";
    try {
      device = new URL(location.href).searchParams.get("device") || "local";
    } catch (_) {
    }
    const account = typeof yyActiveAccount === "string" ? yyActiveAccount : "main";
    const key = "yy-plugin-updates:" + JSON.stringify([device, account, String(user.id || user.username || "local-owner")]);
    try {
      return JSON.parse(localStorage.getItem(key) || "{}") || {};
    } catch (_) {
      return {};
    }
  }
  function collect(data, prefs) {
    var _a, _b, _c, _d;
    const rows = [], spaces = ((_a = data[0]) == null ? void 0 : _a.ok) ? (data[0].workspaces || []).filter((w) => w.language !== "mcfunction") : [];
    const runtime = ((_b = data[1]) == null ? void 0 : _b.ok) ? data[1] : {};
    for (const p of ((_c = data[2]) == null ? void 0 : _c.ok) ? data[2].plugins || [] : []) {
      if (p.type === "mcfunction" || p.type === "chain") continue;
      rows.push({
        key: "compat:" + p.id,
        id: p.id,
        name: p.name || p.id,
        language: p.type || "lua",
        kind: "compat",
        state: p.disabled ? "disabled" : p.state === "error" ? "error" : p.running ? "running" : "stopped",
        source: ((_d = prefs.sources) == null ? void 0 : _d[p.id]) ? "linked" : "installed",
        description: p.description || "",
        version: p.version || "",
        raw: p,
        instances: [],
        spaces: []
      });
    }
    const installed = (runtime.installed || []).slice();
    for (const instance of runtime.instances || []) {
      if (instance.language === "mcfunction" || instance.language === "chain") continue;
      if (!installed.some((p) => p.id === instance.id)) installed.push({ id: instance.id, language: instance.language });
    }
    for (const p of installed) {
      if (p.language === "mcfunction" || p.language === "chain") continue;
      const instances = (runtime.instances || []).filter((item) => item.id === p.id);
      rows.push({
        key: "runtime:" + p.id,
        id: p.id,
        name: p.name || p.id,
        language: p.language,
        kind: "runtime",
        state: instances.some((i) => i.last_error) ? "error" : instances.length ? "running" : "stopped",
        source: "installed",
        description: "\u5DF2\u5B89\u88C5\uFF0C\u53EF\u968F\u65F6\u542F\u52A8",
        version: p.version || "",
        raw: p,
        instances,
        spaces: []
      });
    }
    for (const w of spaces) {
      const target = rows.find((p) => p.language === w.language && (w.plugin_id ? p.id === w.plugin_id : p.kind === "runtime" && p.id === w.id));
      if (target) target.spaces.push(w);
      else rows.push({
        key: "workspace:" + w.id,
        id: w.id,
        name: w.name || w.id,
        language: w.language,
        kind: "workspace",
        state: "workspace",
        source: "workspace",
        description: "\u7EE7\u7EED\u7F16\u5199\uFF0C\u5B8C\u6210\u540E\u53D1\u5E03\u4E3A\u63D2\u4EF6",
        version: "",
        raw: w,
        instances: [],
        spaces: [w]
      });
    }
    return rows.sort((a, b) => a.name.localeCompare(b.name, "zh"));
  }
  function filtered(rows, state) {
    const query = state.query.trim().toLocaleLowerCase();
    return rows.filter((p) => (state.language === "all" || p.language === state.language) && (state.state === "all" || p.state === state.state) && (state.source === "all" || (state.source === "installed" ? p.kind !== "workspace" : p.source === state.source)) && (!query || [p.name, p.id, p.description, labels[p.language] || p.language, ...p.instances.map((i) => i.account)].join(" ").toLocaleLowerCase().includes(query)));
  }
  function button(action, text, args, cls) {
    return '<button type="button" class="yps-button ' + (cls || "") + '" data-yps-action="' + action + '" data-yps-args="' + esc(JSON.stringify(args || [])) + '">' + icon(actionIcons[action]) + "<span>" + esc(text) + "</span></button>";
  }
  function stateLabel(p) {
    return p.kind === "runtime" && p.instances.length > 1 ? p.instances.length + " \u4E2A\u5B9E\u4F8B" : states[p.state];
  }
  function primary(p) {
    if (p.kind === "workspace") return button("workspace", "AI \u5E2E\u5199", [p.id, "agent"], "yps-primary");
    if (p.kind === "compat") return button(p.state === "disabled" ? "enable" : "switch", p.state === "disabled" ? "\u542F\u7528" : p.raw.running ? "\u505C\u6B62" : "\u542F\u52A8", [p.id, p.state !== "running" && !p.raw.running], p.raw.running ? "" : "yps-primary");
    if (p.instances.length > 1) return button("expand", "\u7BA1\u7406\u5B9E\u4F8B", [p.key]);
    const instance = p.instances[0];
    return button("runtime", instance ? "\u505C\u6B62" : "\u542F\u52A8", [p.id, (instance == null ? void 0 : instance.account) || currentAccount(), instance ? "stop" : "start"], instance ? "" : "yps-primary");
  }
  function currentAccount() {
    return typeof yyActiveAccount === "string" && yyActiveAccount ? yyActiveAccount : "main";
  }
  function runStateLine(p) {
    const raw = p.raw || {};
    let tone = "off", text = "\u672A\u5728\u8FD0\u884C", hint = "\u6253\u5F00\u6E38\u620F\u8D26\u53F7\u540E\u53EF\u542F\u52A8";
    if (raw.disabled) {
      tone = "warn";
      text = "\u5DF2\u505C\u7528";
      hint = "\u4E0B\u6B21\u6253\u5F00\u5DE5\u5177\u7BB1\u4E0D\u4F1A\u81EA\u52A8\u542F\u52A8";
    } else if (raw.running) {
      tone = "on";
      text = "\u6B63\u5728\u8FD0\u884C";
      hint = "\u968F\u8D26\u53F7\u8FDE\u63A5\u4FDD\u6301\u8FD0\u884C";
    } else if (p.state === "error") {
      tone = "error";
      text = "\u4E0A\u6B21\u542F\u52A8\u5931\u8D25";
      hint = "\u5C55\u5F00\u4E0A\u65B9\u9519\u8BEF\u4FE1\u606F\u5904\u7406\u540E\u518D\u91CD\u8BD5";
    }
    return '<p class="yps-state-line"><i class="yps-state-dot" data-tone="' + tone + '" aria-hidden="true"></i><strong>' + esc(text) + "</strong><span>" + esc(hint) + "</span></p>";
  }
  function maintenanceGroups(p) {
    const disabled = !!(p.raw && p.raw.disabled);
    let html = '<div class="yps-detail-section"><h4>\u8FD0\u884C</h4>' + runStateLine(p) + '<div class="yps-actions">' + button("reload", "\u91CD\u65B0\u52A0\u8F7D", [p.id]) + button(disabled ? "enable" : "disable", disabled ? "\u6062\u590D\u81EA\u52A8\u542F\u52A8" : "\u505C\u7528\u5E76\u5173\u95ED\u81EA\u52A8\u542F\u52A8", [p.id]) + '</div><p class="yps-note">\u91CD\u65B0\u52A0\u8F7D\u53EA\u91CD\u8BFB\u4EE3\u7801\uFF0C\u4E0D\u5F71\u54CD\u81EA\u52A8\u542F\u52A8\u8BBE\u7F6E\uFF1B\u505C\u7528\u540E\u672C\u6B21\u4E0E\u4E0B\u6B21\u90FD\u4E0D\u542F\u52A8\u3002</p></div>';
    html += '<div class="yps-detail-section"><h4>\u6570\u636E\u7559\u5B58</h4><div class="yps-actions">' + button("backup", "\u5907\u4EFD ZIP", [p.id]) + button("upload", "\u4E0A\u4F20\u5230\u5E02\u573A", [p.id]) + '</div><p class="yps-note">\u5907\u4EFD\u5305\u542B\u4EE3\u7801\u3001\u914D\u7F6E\u4E0E\u6570\u636E\u76EE\u5F55\uFF1B\u4E0A\u4F20\u5230\u5E02\u573A\u4E0D\u4F1A\u5220\u9664\u672C\u5730\u6587\u4EF6\u3002</p></div>';
    html += '<div class="yps-detail-section yps-danger-zone"><h4>\u5371\u9669\u64CD\u4F5C</h4><div class="yps-actions">' + button("delete", "\u5220\u9664\u63D2\u4EF6", [p.id], "yps-danger") + '</div><p class="yps-note">\u5220\u9664\u4F1A\u79FB\u9664\u4EE3\u7801\u3001\u914D\u7F6E\u4E0E\u6570\u636E\uFF0C\u65E0\u6CD5\u64A4\u9500\u3002\u5220\u9664\u524D\u8BF7\u5148\u300C\u5907\u4EFD ZIP\u300D\u3002</p></div>';
    return html;
  }
  function detail(p) {
    let html = '<div class="yps-detail" id="yps-detail-' + esc(p.key) + '"><div class="yps-detail-info">' + (p.name.length > 25 ? "<strong>" + esc(p.name) + "</strong>" : "") + "<p>" + esc(p.description || "\u6B64\u63D2\u4EF6\u672A\u586B\u5199\u8BF4\u660E\u3002") + "</p>" + (p.version ? "<span>\u7248\u672C\uFF1A" + esc(p.version) + "</span>" : "") + "</div>";
    if (p.kind === "compat") {
      if (p.raw.error) html += '<p class="yps-error">' + errorHint(p.raw.error) + "</p>" + diagnostics(p.raw.error, p.id);
      html += '<div class="yps-detail-section"><h4>\u7F16\u8F91\u4E0E\u8D44\u6599</h4><div class="yps-actions">' + button("edit", "\u7F16\u8F91\u4EE3\u7801", [p.id]) + (p.language === "lua" ? button("assist", "AI \u5E2E\u5199", [p.id]) : "") + button("config", "\u914D\u7F6E", [p.id]) + button("directory", "\u6587\u4EF6\u4E0E\u6570\u636E", [p.id]) + button("rename", "\u540D\u79F0\u4E0E\u8BF4\u660E", [p.id, p.name, p.raw.description || ""]) + "</div></div>";
      html += maintenanceGroups(p);
      html += '<details class="yps-doc"><summary>\u64CD\u4F5C\u6587\u6863</summary><pre>' + esc(p.raw.docs || "\u6B64\u63D2\u4EF6\u6682\u672A\u63D0\u4F9B\u64CD\u4F5C\u6587\u6863\u3002") + "</pre></details>";
    } else if (p.kind === "runtime") {
      html += '<div class="yps-detail-section"><h4>\u8D26\u53F7\u5B9E\u4F8B</h4>';
      for (const i of p.instances) html += '<div class="yps-instance"><div><strong>' + esc(i.account === "main" ? "\u4E3B\u8D26\u53F7" : i.account) + "</strong><span>" + (i.last_error ? "\u9700\u8981\u68C0\u67E5" : "\u6B63\u5728\u8FD0\u884C") + "</span>" + (i.last_error ? '<p class="yps-error">' + errorHint(i.last_error) + "</p>" + diagnostics(i.last_error, p.id) : "") + '</div><div class="yps-actions">' + button("runtime", "\u89E6\u53D1", [p.id, i.account, "event"]) + button("panel", "\u63A7\u5236\u9762\u677F", [p.id, i.account]) + button("runtime", "\u91CD\u8F7D", [p.id, i.account, "restart"]) + button("runtime", "\u505C\u6B62", [p.id, i.account, "stop"]) + "</div></div>";
      if (!p.instances.length) html += '<p class="yps-note">\u5F53\u524D\u6CA1\u6709\u8FD0\u884C\u5B9E\u4F8B\u3002\u542F\u52A8\u76EE\u6807\uFF1A' + esc(currentAccount()) + "\u3002</p>";
      html += '<div class="yps-actions">' + button("runtime", "\u542F\u52A8\u5F53\u524D\u8D26\u53F7", [p.id, currentAccount(), "start"]) + button("runtimeEdit", "\u7F16\u8F91 / AI", [p.id, p.language]) + button("runtimeDirectory", "\u6587\u4EF6\u4E0E\u6570\u636E", [p.id]) + button("runtimeExport", "\u5BFC\u51FA ZIP", [p.id]) + button("runtimeDelete", "\u5220\u9664\u63D2\u4EF6", [p.id], "yps-danger") + "</div></div>";
    }
    if (p.spaces.length) {
      html += '<div class="yps-detail-section"><h4>\u521B\u4F5C\u8349\u7A3F' + (p.kind !== "workspace" ? " \xB7 \u5DF2\u5173\u8054" : "") + "</h4>";
      for (const w of p.spaces) html += '<div class="yps-workspace"><div><strong>' + esc(w.name || w.id) + "</strong><span>" + esc(w.state || "\u7B49\u5F85\u7F16\u8F91") + '</span></div><div class="yps-actions">' + button("workspace", "\u4EE3\u7801", [w.id, "code"]) + button("workspace", "AI \u5E2E\u5199", [w.id, "agent"]) + button("archive", "\u5F52\u6863\u5DE5\u4F5C\u533A", [w.id]) + "</div></div>";
      html += "</div>";
    }
    return html + "</div>";
  }
  function row(p, state) {
    const language = labels[p.language] || p.language || "\u5176\u4ED6";
    const mark = { lua: "Lu", python: "Py", native_cpp: "C++", wordbank: "\u8BCD" }[p.language] || "\uFF0B";
    return '<details class="yps-row" data-yps-language="' + esc(p.language) + '" data-yps-key="' + esc(p.key) + '" data-yy-plugin="' + esc(p.id) + '"' + (state.open.has(p.key) ? " open" : "") + '><summary aria-controls="yps-detail-' + esc(p.key) + '"><span class="yps-chevron" aria-hidden="true">\u203A</span><span class="yps-language" aria-hidden="true"><b>' + esc(mark) + '</b><i></i></span><span class="yps-identity"><strong>' + esc(p.name) + "</strong><small>" + esc(p.description || language + " \u63D2\u4EF6") + '</small><span class="yps-item-tags"><span>' + esc(language) + "</span>" + (p.version ? "<span>v" + esc(p.version) + "</span>" : "") + (p.spaces.length && p.kind !== "workspace" ? "<span>\u5DF2\u5173\u8054\u8349\u7A3F</span>" : "") + '</span></span><span class="yps-row-meta"><span class="yps-badge yps-state-' + p.state + '" data-yps-label="' + esc(stateLabel(p)) + '">' + esc(stateLabel(p)) + "</span><small>" + esc(p.kind === "workspace" ? "\u7B49\u5F85\u53D1\u5E03" : p.kind === "runtime" && p.instances.length ? p.instances.map((i) => i.account === "main" ? "\u4E3B\u8D26\u53F7" : i.account).join(" \xB7 ") : p.raw.running ? "\u63D2\u4EF6\u5DF2\u542F\u52A8" : "\u6309\u9700\u542F\u52A8") + '</small></span><span class="yps-primary-action">' + primary(p) + '</span></summary><p class="yps-row-status" role="status" hidden></p>' + detail(p) + "</details>";
  }
  function groups(rows, by) {
    const names = by === "language" ? labels : by === "source" ? { installed: "\u5DF2\u5B89\u88C5", linked: "\u5DF2\u5173\u8054\u66F4\u65B0", workspace: "\u521B\u4F5C\u8349\u7A3F" } : states;
    const order = by === "state" ? ["running", "waiting", "error", "stopped", "disabled", "workspace"] : [];
    const map = /* @__PURE__ */ new Map();
    for (const p of rows) {
      const key = p[by];
      if (!map.has(key)) map.set(key, []);
      map.get(key).push(p);
    }
    return Array.from(map, ([key, items]) => ({ key, label: names[key] || key, items })).sort((a, b) => order.length ? order.indexOf(a.key) - order.indexOf(b.key) : a.label.localeCompare(b.label, "zh"));
  }
  function paintRows() {
    const state = view(), host = document.getElementById("yps-inventory");
    if (!host) return;
    const rows = filtered(state.rows, state);
    host.innerHTML = rows.length ? groups(rows, state.group).map((g) => '<section class="yps-group"><div class="yps-group-heading"><h3>' + esc(g.label) + "</h3><span>" + g.items.length + "</span></div>" + g.items.map((p) => row(p, state)).join("") + "</section>").join("") : '<div class="yps-empty">' + icon(state.rows.length ? "search" : "folder") + "<strong>" + (state.rows.length ? "\u6CA1\u6709\u5339\u914D\u7684\u63D2\u4EF6" : "\u4ECE\u7B2C\u4E00\u4E2A\u63D2\u4EF6\u5F00\u59CB") + "</strong><p>" + (state.rows.length ? "\u8BD5\u8BD5\u5176\u4ED6\u540D\u79F0\u3001\u8BED\u8A00\u6216\u72B6\u6001\u3002" : "\u5BFC\u5165\u5DF2\u6709\u63D2\u4EF6\uFF0C\u6216\u521B\u5EFA\u4E00\u4E2A\u65B0\u63D2\u4EF6\u3002") + "</p>" + button(state.rows.length ? "reset" : "create", state.rows.length ? "\u6E05\u9664\u7B5B\u9009" : "\u521B\u5EFA\u63D2\u4EF6") + "</div>";
    document.getElementById("yps-match-count").textContent = rows.length + " / " + state.rows.length + " \u9879";
    host.querySelectorAll(".yps-row").forEach((el) => el.addEventListener("toggle", () => {
      if (el.isConnected) el.open ? state.open.add(el.dataset.ypsKey) : state.open.delete(el.dataset.ypsKey);
    }));
    yyPluginOperationUI();
  }
  function select(id, title, options, value) {
    return "<label><span>" + title + '</span><select id="' + id + '" data-yps-filter="' + id.replace("yps-", "") + '">' + Object.entries(options).map(([key, text]) => '<option value="' + key + '"' + (value === key ? " selected" : "") + ">" + text + "</option>").join("") + "</select></label>";
  }
  function render(data, failures) {
    var _a;
    const host = document.getElementById("tab-tasks");
    if (!host) return;
    const state = view(), prefs = preferences();
    data = data.map((result, index) => {
      var _a2, _b;
      return result.ok ? result : ((_b = (_a2 = state.data) == null ? void 0 : _a2[index]) == null ? void 0 : _b.ok) ? state.data[index] : result;
    });
    state.data = data;
    state.failures = failures;
    state.rows = collect(data, prefs);
    if (!failures.length) state.resultGeneration = typeof yyPluginResultSequence === "number" ? yyPluginResultSequence : 0;
    const running = state.rows.filter((p) => p.state === "running" || p.instances.length).length;
    const installed = state.rows.filter((p) => p.kind !== "workspace").length;
    const drafts = ((_a = data[0]) == null ? void 0 : _a.ok) ? (data[0].workspaces || []).filter((w) => w.language !== "mcfunction").length : "\u2014";
    host.innerHTML = '<section class="yps-shell content-inner"><header class="yps-header"><div class="yps-title-wrap"><span class="yps-title-icon" aria-hidden="true">' + icon("folder") + '</span><div><span class="yps-eyebrow">\u6E38\u56ED\u5DE5\u5177\u7BB1 / \u6269\u5C55\u7BA1\u7406</span><h2>\u6211\u7684\u63D2\u4EF6</h2><p>\u5E38\u7528\u63D2\u4EF6\u968F\u624B\u542F\u52A8\uFF0C\u4EE3\u7801\u3001\u914D\u7F6E\u4E0E\u6587\u4EF6\u4E00\u5904\u7BA1\u7406\u3002</p></div></div><div class="yps-header-actions">' + button("imports", "\u5BFC\u5165\u63D2\u4EF6") + button("create", "\u521B\u5EFA\u63D2\u4EF6", [], "yps-primary") + '</div></header><div class="yps-overview"><div class="yps-metric yps-metric-installed">' + icon("folder") + "<div><strong>" + installed + '</strong><span>\u5DF2\u5B89\u88C5\u63D2\u4EF6</span></div></div><div class="yps-metric yps-metric-running">' + icon("radio") + "<div><strong>" + running + '</strong><span>\u6B63\u5728\u8FD0\u884C</span></div></div><div class="yps-metric yps-metric-drafts">' + icon("file") + "<div><strong>" + drafts + '</strong><span>\u521B\u4F5C\u8349\u7A3F</span></div></div><div class="yps-overview-note">' + button("refresh", "\u5237\u65B0\u72B6\u6001") + '<span>\u4EE5\u6700\u8FD1\u4E00\u6B21\u8BFB\u53D6\u4E3A\u51C6</span></div></div><div id="yy-plugin-operation-status" class="yps-operation" role="status" aria-live="polite" hidden></div><div id="yps-last-result" class="yps-result" role="status" aria-live="polite" hidden></div>' + (failures.length ? '<div class="yps-error yps-load-error" role="alert">' + failures.map((error) => esc(errorHint(error))).join("<br>") + diagnostics(failures.join("\n")) + "<p>\u90E8\u5206\u5217\u8868\u6682\u65F6\u65E0\u6CD5\u8BFB\u53D6\uFF0C\u8BF7\u5237\u65B0\u91CD\u8BD5\u3002</p></div>" : "") + '<div class="yps-layout"><aside class="yps-side"><span class="yps-section-label">\u63D2\u4EF6\u5E93</span><nav aria-label="\u63D2\u4EF6\u6765\u6E90">' + [["all", "\u5168\u90E8\u9879\u76EE"], ["installed", "\u5DF2\u5B89\u88C5"], ["linked", "\u5DF2\u5173\u8054\u66F4\u65B0"], ["workspace", "\u521B\u4F5C\u8349\u7A3F"]].map(([key, title]) => '<button type="button" data-yps-source="' + key + '" aria-pressed="' + (state.source === key) + '"><span>' + title + "</span><b>" + state.rows.filter((p) => key === "all" || (key === "installed" ? p.kind !== "workspace" : p.source === key)).length + "</b></button>").join("") + '</nav><div class="yps-discover"><span class="yps-section-label">\u53D1\u73B0\u4E0E\u5DE5\u5177</span>' + button("market", "\u6D4F\u89C8\u63D2\u4EF6\u5E02\u573A") + button("chainTool", "\u547D\u4EE4\u94FE\u5DE5\u5177") + '</div><details class="yps-resources"><summary>\u8D44\u6599\u4E0E\u5F00\u53D1</summary><div>' + button("library", "\u6211\u7684\u63D2\u4EF6\u5E02\u573A") + button("downloads", "\u63D2\u4EF6\u751F\u6210\u6587\u4EF6") + button("doc", "Python \u624B\u518C", ["python-runtime"]) + button("doc", "C++ \u624B\u518C", ["cpp-runtime"]) + '</div></details></aside><main class="yps-main"><div class="yps-toolbar"><label class="yps-search"><span>\u641C\u7D22\u63D2\u4EF6</span><span class="yps-search-field">' + icon("search") + '<input id="yps-search" type="search" autocomplete="off" placeholder="\u540D\u79F0\u3001\u8BF4\u660E\u6216\u8D26\u53F7\u2026" value="' + esc(state.query) + '"></span></label><div class="yps-selects">' + select("yps-language", "\u8BED\u8A00", { all: "\u5168\u90E8\u8BED\u8A00", ...labels }, state.language) + select("yps-state", "\u72B6\u6001", { all: "\u5168\u90E8\u72B6\u6001", ...states }, state.state) + select("yps-group", "\u5206\u7EC4", { state: "\u6309\u8FD0\u884C\u72B6\u6001", language: "\u6309\u8BED\u8A00", source: "\u6309\u6765\u6E90" }, state.group) + '</div></div><div class="yps-inventory-meta"><span id="yps-match-count"></span><span>\u5C55\u5F00\u67E5\u770B\u914D\u7F6E\u4E0E\u6587\u4EF6</span></div><div id="yps-inventory"></div><details class="yps-updates"><summary><span>' + icon("loop") + '\u6765\u6E90\u4E0E\u66F4\u65B0</span><small>\u68C0\u67E5\u7248\u672C \xB7 \u7BA1\u7406\u5173\u8054</small></summary><div><p class="yps-note">\u68C0\u67E5\u5DF2\u5173\u8054\u7684\u63D2\u4EF6\u7248\u672C\uFF0C\u6709\u66F4\u65B0\u540E\u518D\u51B3\u5B9A\u662F\u5426\u5B89\u88C5\u3002</p><div class="yps-actions"><button type="button" class="yps-button" id="yy-plugin-check" onclick="yyCheckPluginUpdates()">\u68C0\u67E5\u66F4\u65B0</button>' + button("sources", "\u5173\u8054\u66F4\u65B0\u6765\u6E90") + '</div><label class="yps-auto"><input id="yy-plugin-auto-check" type="checkbox"' + (prefs.auto === true ? " checked" : "") + '>\u8FDB\u5165\u63D2\u4EF6\u9875\u65F6\u81EA\u52A8\u68C0\u67E5\u66F4\u65B0</label><div id="yy-plugin-update-result" role="status"></div></div></details><pre id="yy-runtime-result" hidden></pre></main></div></section>';
    host.querySelector("#yps-search").addEventListener("input", (event) => {
      state.query = event.target.value;
      paintRows();
    });
    host.querySelectorAll("[data-yps-filter]").forEach((el) => el.addEventListener("change", () => {
      state[el.dataset.ypsFilter] = el.value;
      paintRows();
    }));
    host.querySelectorAll("[data-yps-source]").forEach((el) => el.addEventListener("click", () => {
      state.source = el.dataset.ypsSource;
      host.querySelectorAll("[data-yps-source]").forEach((b) => b.setAttribute("aria-pressed", String(b === el)));
      paintRows();
    }));
    host.querySelector("#yy-plugin-auto-check").addEventListener("change", (event) => yyPluginAutoCheck(event.target.checked));
    host.querySelector(".yps-shell").addEventListener("click", click);
    paintRows();
    if (prefs.auto === true && Date.now() - (prefs.lastCheck || 0) > 15 * 60 * 1e3) yyCheckPluginUpdates();
  }
  async function click(event) {
    const el = event.target.closest("[data-yps-action]");
    if (!el) return;
    event.stopPropagation();
    if (el.closest("summary")) event.preventDefault();
    const action = el.dataset.ypsAction, args = JSON.parse(el.dataset.ypsArgs || "[]");
    if (action === "expand") {
      el.closest(".yps-row").open = true;
      return;
    }
    if (action === "reset") {
      const state = view();
      state.query = "";
      state.language = state.state = state.source = "all";
      render(state.data, state.failures || []);
      return;
    }
    const handlers = { create: yyCreateWorkspaceDialog, refresh: Rplg, imports: importDialog, market: pmOpenMarket, library: yyLibraryPage, downloads: yyOpenExportCenter, doc: yyOpenLanguageDoc, sources: yyPluginSourceDialog, workspace: yyOpenWorkspace, archive: yyArchiveProject, edit: plgEdit, assist: openAIAssist, config: plgEditConfig, directory: plgDataDir, rename: plgRename, reload: plgReload, enable: plgEnable, disable: plgDisable, switch: plgSwitch, backup: plgExportSAF, upload: pmOpenUpload, delete: plgDel, runtime: yyRuntimeAction, panel: yySDKOpenPanel, runtimeEdit: yyOpenInstalledRuntime, runtimeDirectory: yyRuntimeDirectoryView, runtimeExport: yyExportRuntimePlugin, runtimeDelete: yyDeleteInstalledRuntime, chainTool: yyChainsPage };
    const fn = handlers[action];
    if (!fn) return;
    const owner = scope(), html = el.innerHTML, key = JSON.stringify([owner, action, args]);
    const pluginRow = el.closest(".yps-row");
    if (el.disabled || pendingClicks.has(key) || (pluginRow == null ? void 0 : pluginRow.getAttribute("aria-busy")) === "true") return;
    pendingClicks.add(key);
    let timer;
    try {
      const result = fn.apply(null, args);
      if (result && typeof result.then === "function") {
        el.disabled = true;
        el.textContent = "\u5904\u7406\u4E2D\u2026";
        timer = setTimeout(() => {
          if (el.isConnected && scope() === owner) el.textContent = "\u4ECD\u5728\u5904\u7406\u2026";
        }, 5e3);
        await result;
      }
    } catch (error) {
      if (scope() === owner) T(error.message || "\u64CD\u4F5C\u5931\u8D25", "e");
    } finally {
      pendingClicks.delete(key);
      clearTimeout(timer);
      if (el.isConnected && scope() === owner) {
        el.disabled = (pluginRow == null ? void 0 : pluginRow.getAttribute("aria-busy")) === "true";
        el.innerHTML = html;
      }
    }
  }
  function importDialog() {
    const overlay = document.createElement("div");
    overlay.className = "modal-overlay";
    overlay.innerHTML = '<div class="modal-box yps-import-dialog"><h3>\u5BFC\u5165\u63D2\u4EF6</h3><p class="yps-note">\u9009\u62E9\u4F60\u8981\u5BFC\u5165\u7684\u63D2\u4EF6\u7C7B\u578B\u3002</p><button type="button" class="yps-button" data-import="runtime">Python / C++ \u63D2\u4EF6\u5305</button><button type="button" class="yps-button" data-import="compat">Lua / \u8BCD\u5E93\u63D2\u4EF6\u5305</button><p class="yps-note">TXT / mcfunction \u6587\u4EF6\u8BF7\u5230\u547D\u4EE4\u94FE\u5DE5\u5177\u7BA1\u7406\u3002</p><button type="button" class="yps-button" data-import="chain-tool">\u6253\u5F00\u547D\u4EE4\u94FE\u5DE5\u5177</button><button type="button" class="yps-button" data-import="close">\u53D6\u6D88</button></div>';
    overlay.addEventListener("click", (event) => {
      const kind = event.target.dataset.import;
      if (!kind) return;
      overlay.remove();
      if (kind === "runtime") yyImportPython();
      if (kind === "compat") plgImport();
      if (kind === "chain-tool") yyChainsPage();
    });
    document.body.appendChild(overlay);
  }
  const baseOperationUI = yyPluginOperationUI;
  yyPluginOperationUI = function() {
    baseOperationUI();
    const ops = Array.from(yyPluginOperations.values()).filter((op) => op.scope === scope());
    const results = typeof yyPluginLastResults === "object" ? Array.from(yyPluginLastResults.values()).filter((op) => op.scope === scope()) : [];
    function resultText(result) {
      return result.label + (result.status === "success" ? "\u5B8C\u6210" : result.status === "unknown" ? "\u7ED3\u679C\u672A\u786E\u8BA4\uFF0C\u8BF7\u5237\u65B0\u6838\u5BF9\uFF0C\u52FF\u91CD\u590D\u63D0\u4EA4\u3002" : "\u5931\u8D25\uFF0C\u8BF7\u68C0\u67E5\u540E\u91CD\u8BD5\u3002");
    }
    const latest = results[results.length - 1], summary = document.getElementById("yps-last-result");
    if (summary) {
      summary.hidden = !latest || ops.some((op) => op.id === latest.id);
      if (!summary.hidden) {
        summary.dataset.result = latest.status;
        summary.textContent = latest.id + " \xB7 " + resultText(latest);
      }
    }
    document.querySelectorAll(".yps-row").forEach((row2) => {
      const op = ops.find((op2) => op2.id === row2.getAttribute("data-yy-plugin"));
      const result = results.find((op2) => op2.id === row2.getAttribute("data-yy-plugin"));
      const status = row2.querySelector(".yps-row-status");
      const badge = row2.querySelector(".yps-badge");
      status.hidden = !op && !result;
      if (op) {
        const seconds = Math.floor((Date.now() - op.started) / 1e3);
        status.textContent = op.label + " \xB7 \u5DF2\u7B49\u5F85 " + seconds + " \u79D2" + (seconds >= 5 ? "\u3002\u8BF7\u6C42\u4ECD\u5728\u5904\u7406\uFF0C\u8BF7\u52FF\u91CD\u590D\u63D0\u4EA4\u3002" : "\u2026");
        badge.textContent = op.label.replace("\u63D2\u4EF6", "") + "\u4E2D \xB7 " + seconds + "\u79D2";
      } else {
        if (result) status.textContent = resultText(result);
        badge.textContent = result && result.generation > (view().resultGeneration || 0) ? "\u6B63\u5728\u6838\u5BF9\u72B6\u6001" : badge.dataset.ypsLabel;
      }
    });
  };
  Rplg = async function() {
    const owner = scope(), epoch = ++yyPluginRenderEpoch, ticket = ++sequence;
    if (yyPluginScopeBusy(owner)) {
      yyPluginOperationUI();
      return;
    }
    const host = document.getElementById("tab-tasks");
    if (host && !host.querySelector(".yps-shell")) host.innerHTML = '<div class="yps-loading" role="status">\u6B63\u5728\u8BFB\u53D6\u63D2\u4EF6\u5217\u8868\u2026</div>';
    const paths = ["/api/youyuan/agent/workspaces", "/api/youyuan/runtime/list", "/api/plugin/list"];
    const responses = await Promise.allSettled(paths.map((path) => A("GET", path)));
    if (owner !== scope() || epoch !== yyPluginRenderEpoch || ticket !== sequence || yyPluginScopeBusy(owner) || typeof curTab !== "undefined" && curTab !== "tasks") return;
    const data = responses.map((result) => {
      var _a;
      return result.status === "fulfilled" ? result.value || { ok: false, error: "\u54CD\u5E94\u4E3A\u7A7A" } : { ok: false, error: ((_a = result.reason) == null ? void 0 : _a.message) || "\u8BF7\u6C42\u5931\u8D25" };
    });
    const failures = data.map((r, i) => r.ok ? "" : ["\u521B\u4F5C\u8349\u7A3F", "\u591A\u8BED\u8A00\u8FD0\u884C\u73AF\u5883", "\u517C\u5BB9\u63D2\u4EF6"][i] + "\uFF1A" + (r.error || "\u8BFB\u53D6\u5931\u8D25")).filter(Boolean);
    render(data, failures);
  };
  window.yyPluginStudioModel = { collect, filtered, groups };
})();
