(function() {
  "use strict";
  const baseOpen = yyOpenWorkspace, basePane = yyWorkspacePane;
  function button(text, fn, cls) {
    const b = document.createElement("button");
    b.type = "button";
    b.textContent = text;
    b.className = cls || "yy-text-btn";
    b.onclick = fn;
    return b;
  }
  yyOpenWorkspace = async function(id, pane) {
    await baseOpen(id, pane);
    if ((yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) !== id || !document.getElementById("yy-workspace")) return;
    mount();
  };
  yyWorkspacePane = function(pane) {
    basePane(pane);
    const page = document.getElementById("sub-page");
    if (page) page.dataset.studioPane = pane;
  };
  function mount() {
    var _a, _b, _c;
    const page = document.getElementById("sub-page"), workspace = document.getElementById("yy-workspace"), agent = workspace.querySelector(".yy-agent-pane");
    if (!agent || document.getElementById("yy-agent-studio-shell")) return;
    page.classList.add("yy-agent-studio");
    page.dataset.studioPane = workspace.dataset.pane;
    const shell = document.createElement("div");
    shell.id = "yy-agent-studio-shell";
    workspace.before(shell);
    const side = document.createElement("aside");
    side.className = "yy-studio-sessions";
    side.setAttribute("aria-label", "\u5DE5\u4F5C\u533A\u4F1A\u8BDD");
    const heading = document.createElement("div");
    heading.className = "yy-studio-side-title";
    heading.textContent = "\u6E38\u56ED / \u521B\u4F5C";
    side.append(heading);
    const sessions = document.getElementById("yy-open-agent-tabs");
    if (sessions) side.append(sessions);
    const links = document.createElement("div");
    links.className = "yy-studio-side-links";
    // 只留这里独有的两个入口。「MCP 连接」「运行预算」在输入框上方的「配置」菜单里
    // 已有同一功能（离手更近），两处并存只会让侧栏挤、手机上更容易点错。
    links.append(button("\u6280\u80FD\u4E0E\u5DE5\u5177", () => yyAgentPreferences()), button("\u6280\u80FD\u52A0\u8F7D\u5E93", () => yyOpenSkillLibrary()));
    side.append(links);
    shell.append(side, workspace);
    const context = document.getElementById("yy-workspace-context");
    shell.append(context);
    const toolbar = agent.querySelector(".yy-model-toolbar"), compose = agent.querySelector(".yy-agent-compose");
    compose.append(toolbar);
    const select = document.getElementById("yy-agent-model");
    select.title = "\u672C\u5DE5\u4F5C\u533A\u4F7F\u7528\u7684\u6A21\u578B";
    (_a = toolbar.querySelector(".yy-profile-toolbar")) == null ? void 0 : _a.remove();
    const settings = document.createElement("details");
    settings.className = "yy-studio-options";
    const summary = document.createElement("summary");
    summary.textContent = "\u914D\u7F6E";
    const menu = document.createElement("div");
    menu.className = "yy-studio-options-menu";
    menu.append(button("\u6A21\u578B\u4E0E\u670D\u52A1\u5546", () => yyAgentModels()), button("\u63D0\u793A\u8BCD / \u6280\u80FD / \u5DE5\u5177", () => yyAgentPreferences()), button("MCP \u8FDE\u63A5", () => yyWorkspacePane("context")), button("\u8FD0\u884C\u9884\u7B97", yyAgentRunBudgetDialog));
    settings.append(summary, menu);
    toolbar.append(settings);
    const legacy = toolbar.querySelector("button");
    if (legacy) legacy.remove();
    const actions = compose.querySelector(".yy-agent-compose-actions");
    (_b = actions.querySelector("button")) == null ? void 0 : _b.replaceChildren(document.createTextNode("\u505C\u6B62"));
    const text = document.getElementById("yy-agent-message");
    text.placeholder = "\u63CF\u8FF0\u8981\u5B8C\u6210\u7684\u76EE\u6807\uFF0C\u6216\u8865\u5145\u5F53\u524D\u4EFB\u52A1\u2026";
    text.addEventListener("keydown", (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "Enter" && !e.isComposing) {
        e.preventDefault();
        yyAgentTask("send");
      }
    });
    const hint = document.createElement("small");
    hint.className = "yy-studio-input-hint";
    hint.textContent = "Ctrl / \u2318 + Enter \u53D1\u9001 \xB7 \u505C\u6B62\u540E\u53EF\u7EE7\u7EED\uFF0C\u8BB0\u5F55\u81EA\u52A8\u4FDD\u5B58";
    compose.append(hint);
    mountEditor(workspace);
    context.querySelectorAll(".yy-form-grid > .card").forEach((card, index) => {
      const d = document.createElement("details");
      d.className = "yy-studio-config-group";
      d.open = index === 1;
      const s = document.createElement("summary");
      s.textContent = ["\u4FEE\u6539\u5386\u53F2", "MCP \u8FDE\u63A5\u4E0E\u5DE5\u5177", "\u4EFB\u52A1\u8BB0\u5FC6\u4E0E\u5F00\u53D1\u8BF4\u660E"][index] || "\u66F4\u591A\u8BBE\u7F6E";
      card.before(d);
      d.append(s, card);
    });
    const nav = agent.querySelector(".yy-agent-scroll-controls");
    (_c = nav == null ? void 0 : nav.querySelector("button")) == null ? void 0 : _c.remove();
  }
  function drawFiles() {
    const list = document.getElementById("yy-studio-file-list");
    if (!list) return;
    const files = (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.files) || [];
    const signature = JSON.stringify(files.map((file) => file.path));
    if (list.dataset.paths !== signature) {
      list.replaceChildren();
      for (const file of files) {
        const b = button(file.path, () => yyOpenProjectFile(file.path), "yy-studio-file");
        b.title = file.path;
        b.dataset.path = file.path;
        list.append(b);
      }
      list.dataset.paths = signature;
    }
    list.querySelectorAll("[data-path]").forEach((b) => b.classList.toggle("active", b.dataset.path === yyEditorPath));
    const title = document.getElementById("yy-studio-file-title");
    if (title) title.textContent = yyEditorPath || "\u9009\u62E9\u9879\u76EE\u6587\u4EF6";
  }
  function mountEditor(workspace) {
    const pane = workspace.querySelector(".yy-code-pane");
    if (!pane || pane.querySelector(".yy-editor-layout")) return;
    const layout = document.createElement("div");
    layout.className = "yy-editor-layout";
    const explorer = document.createElement("aside");
    explorer.className = "yy-studio-explorer";
    explorer.setAttribute("aria-label", "\u9879\u76EE\u6587\u4EF6");
    const label = document.createElement("strong");
    label.textContent = "\u9879\u76EE\u6587\u4EF6";
    const list = document.createElement("div");
    list.id = "yy-studio-file-list";
    explorer.append(label, list);
    const surface = document.createElement("div");
    surface.className = "yy-editor-surface";
    while (pane.firstChild) surface.append(pane.firstChild);
    const tab = document.createElement("div");
    tab.className = "yy-studio-file-title";
    tab.innerHTML = '<span aria-hidden="true">\u25C7</span><strong id="yy-studio-file-title"></strong><small>\u81EA\u52A8\u4FDD\u5B58</small>';
    surface.querySelector(".yy-editor-toolbar").after(tab);
    layout.append(explorer, surface);
    pane.append(layout);
    drawFiles();
    const search = surface.querySelector(".yy-project-search"), result = surface.querySelector("#yy-project-search-result");
    if (search && result) {
      const fold = document.createElement("details");
      fold.className = "yy-editor-search-fold";
      const summary = document.createElement("summary");
      summary.textContent = "\u5728\u9879\u76EE\u4E2D\u641C\u7D22";
      search.before(fold);
      fold.append(summary, search, result);
    }
  }
  const drawProjectFiles = yyDrawProjectFiles;
  yyDrawProjectFiles = function(files) {
    drawProjectFiles(files);
    drawFiles();
  };
  const openProjectFile = yyOpenProjectFile;
  yyOpenProjectFile = async function(path) {
    await openProjectFile(path);
    drawFiles();
  };
  window.yyAgentRunBudgetDialog = async function() {
    const id = yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id, ownerScope = yyEditorAccountScope(), epoch = yyWorkspaceOpenEpoch;
    if (!id) return;
    const live = () => id === (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) && ownerScope === yyEditorAccountScope() && epoch === yyWorkspaceOpenEpoch;
    const r = await A("GET", "/api/youyuan/agent/run-budget?id=" + encodeURIComponent(id));
    if (!live()) return;
    if (!r.ok) {
      T(r.error, "e");
      return;
    }
    const host = yyAgentProfileModal("\u8FD0\u884C\u9884\u7B97");
    host.innerHTML = '<p class="yy-hint">\u8FBE\u5230\u9884\u7B97\u4F1A\u4FDD\u5B58\u8FDB\u5EA6\u5E76\u7B49\u5F85\u7EE7\u7EED\u3002\u9ED8\u8BA4\u4E0D\u8BBE\u8F6E\u6B21\u548C\u5DE5\u5177\u6B21\u6570\u4E0A\u9650\uFF0C\u8FDE\u7EED\u5DE5\u5177\u5931\u8D25\u4F1A\u4EA4\u7ED9\u6A21\u578B\u5904\u7406\u3002\u670D\u52A1\u5546\u7684\u914D\u989D\u548C\u63A5\u53E3\u9519\u8BEF\u4ECD\u53EF\u80FD\u6682\u505C\u4EFB\u52A1\u3002</p><div class="yy-studio-budget-grid"><label>\u8F6E\u6B21\u4E0A\u9650\uFF080 \u4E0D\u9650\u5236\uFF09<input id="yy-budget-rounds" type="number" min="0" max="10000"></label><label>\u5DE5\u5177\u6B21\u6570\uFF080 \u4E0D\u9650\u5236\uFF09<input id="yy-budget-tools" type="number" min="0" max="100000"></label><label>\u5355\u6B21\u8FD0\u884C\u5206\u949F<input id="yy-budget-minutes" type="number" min="1" max="1440"></label></div><p class="yy-hint">\u4FDD\u5B58\u7528\u4E8E\u4E0B\u6B21\u5F00\u59CB\u6216\u7EE7\u7EED\u4EFB\u52A1\uFF0C\u6B63\u5728\u8FD0\u884C\u7684\u4EFB\u52A1\u6CBF\u7528\u542F\u52A8\u65F6\u9884\u7B97\u3002</p><div id="yy-budget-result" role="status"></div>';
    host.querySelector("#yy-budget-rounds").value = r.budget.max_rounds;
    host.querySelector("#yy-budget-tools").value = r.budget.max_tools;
    host.querySelector("#yy-budget-minutes").value = r.budget.max_minutes;
    const save = button("\u4FDD\u5B58\u9884\u7B97", async () => {
      if (!live() || !host.isConnected || save.disabled) return;
      save.disabled = true;
      try {
        const budget = { max_rounds: Number(host.querySelector("#yy-budget-rounds").value), max_tools: Number(host.querySelector("#yy-budget-tools").value), max_minutes: Number(host.querySelector("#yy-budget-minutes").value) };
        const result = await A("POST", "/api/youyuan/agent/run-budget", { id, budget });
        if (live() && host.isConnected) host.querySelector("#yy-budget-result").textContent = result.ok ? "\u9884\u7B97\u5DF2\u4FDD\u5B58\uFF0C\u4E0B\u6B21\u8FD0\u884C\u751F\u6548\u3002" : result.error;
      } catch (e) {
        if (live() && host.isConnected) host.querySelector("#yy-budget-result").textContent = e.message;
      } finally {
        save.disabled = false;
      }
    }, "yy-primary");
    host.append(save);
  };
  const preferences = yyAgentPreferences;
  yyAgentPreferences = async function() {
    const id = yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id;
    await preferences();
    if (id !== (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id)) return;
    const host = document.getElementById("yy-agent-profile-body");
    if (!host) return;
    host.closest(".modal-box").classList.add("yy-studio-preferences");
    const search = document.createElement("input");
    search.type = "search";
    search.placeholder = "\u7B5B\u9009\u6280\u80FD\u6216\u5DE5\u5177\u540D\u79F0";
    search.setAttribute("aria-label", "\u7B5B\u9009\u6280\u80FD\u6216\u5DE5\u5177");
    search.oninput = () => host.querySelectorAll(".yy-profile-checks label").forEach((label) => {
      label.hidden = !label.textContent.toLowerCase().includes(search.value.trim().toLowerCase());
    });
    host.prepend(search);
    host.querySelectorAll(".yy-profile-checks").forEach((list, i) => {
      const group = document.createElement("details");
      group.className = "yy-studio-config-group";
      group.open = i === 0;
      const summary = document.createElement("summary");
      summary.textContent = i === 0 ? "\u5DF2\u52A0\u8F7D\u4E0E\u53EF\u7528\u6280\u80FD" : "\u5DE5\u5177\u6743\u9650";
      list.before(group);
      group.append(summary, list);
    });
  };
})();
