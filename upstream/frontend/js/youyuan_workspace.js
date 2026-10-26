var yyLanguageFilter = "all", yyWorkspaceActive = null, yyWorkspaceEditor = null, yyEditorPath = "", yyEditorDirty = false, yyEditorTimer = null, yyWorkspacePoll = null, yyWorkspaceSeq = 0, yyFileBaseHash = "";
var yyWorkspaceOpenEpoch = 0;
var yyPluginRenderEpoch = 0;
var yyLanguageNames = { python: "Python", native_cpp: "C++", lua: "Lua", wordbank: "\u8BCD\u5E93", mcfunction: "\u547D\u4EE4\u94FE" };
Rplg = async function() {
  var _a, _b;
  var epoch = ++yyPluginRenderEpoch, scope = yyReadScope();
  if (typeof yyPluginScopeBusy === "function" && yyPluginScopeBusy(scope)) {
    yyPluginOperationUI();
    return;
  }
  var results = await Promise.all([A("GET", "/api/youyuan/agent/workspaces"), A("GET", "/api/youyuan/runtime/list"), A("GET", "/api/plugin/list")]);
  if (epoch !== yyPluginRenderEpoch || scope !== yyReadScope() || typeof yyPluginScopeBusy === "function" && yyPluginScopeBusy(scope)) return;
  var failures = results.map(function(r, i) {
    return r.ok === false ? ["\u5DE5\u4F5C\u533A", "\u5E38\u9A7B\u8FD0\u884C\u73AF\u5883", "\u517C\u5BB9\u63D2\u4EF6"][i] + "\uFF1A" + (r.error || "\u8BFB\u53D6\u5931\u8D25") : "";
  }).filter(Boolean);
  if (failures.length) {
    T(failures.join("\uFF1B"), "e");
    return;
  }
  var detached = document.createElement("div");
  detached.innerHTML = yyPluginListHTML(results[2]);
  var compat = detached.querySelector(".plg-section");
  if (compat && compat.firstElementChild) compat.firstElementChild.remove();
  var compatibleHTML = compat ? compat.innerHTML : "";
  var spaces = (results[0].workspaces || []).filter(function(w) {
    return yyLanguageFilter === "all" || w.language === yyLanguageFilter;
  });
  var instances = results[1].instances || [];
  var html = '<div class="content-inner">' + yyHeading("\u591A\u8BED\u8A00\u5E38\u9A7B\u63D2\u4EF6", "Python\u3001C++ \u4E0E Lua\uFF0C\u7F16\u8F91\u3001\u8FD0\u884C\u548C AI \u5E2E\u5199\u653E\u5728\u540C\u4E00\u4E2A\u5DE5\u4F5C\u533A\u3002") + '<div class="yy-plugin-header"><div class="yy-actions"><button class="yy-primary" onclick="yyCreateWorkspaceDialog()">' + yyIcon("spark") + '\u65B0\u5EFA\u63D2\u4EF6 / AI \u5DE5\u4F5C\u533A</button><button class="yy-secondary" onclick="yyImportPython()">' + yyIcon("import") + '\u5BFC\u5165 Python / C++ ZIP</button><button class="yy-secondary" onclick="plgImport()">' + yyIcon("import") + '\u5BFC\u5165 Lua / \u517C\u5BB9\u63D2\u4EF6</button><button class="yy-secondary" onclick="pmOpenMarket()">' + yyIcon("box") + '\u63D2\u4EF6\u5E02\u573A</button></div></div><div class="yy-filters">' + [["all", "\u5168\u90E8"], ["python", "Python"], ["native_cpp", "C++"], ["lua", "Lua"], ["mcfunction", "\u547D\u4EE4\u94FE"]].map(function(l) {
    return '<button class="yy-filter' + (yyLanguageFilter === l[0] ? " active" : "") + `" onclick="yyLanguageFilter='` + l[0] + `';Rplg()">` + l[1] + "</button>";
  }).join("") + '</div><div class="yy-section-head"><h3>\u63D2\u4EF6\u4E0E\u4EFB\u52A1\u5DE5\u4F5C\u533A <small>' + spaces.length + '</small></h3></div><div class="yy-workspace-cards">';
  html += spaces.length ? spaces.map(function(w) {
    return '<article class="card yy-project-card"><div class="yy-form-head"><h3>' + yyIcon(w.language === "mcfunction" ? "code" : "plugin") + yyEscape(w.name) + "</h3>" + yyBadge(yyLanguageNames[w.language]) + '</div><p class="yy-hint">' + yyEscape(w.id) + " \xB7 " + yyEscape(w.state) + `</p><div class="yy-actions"><button class="btn-s" onclick="yyOpenWorkspace('` + w.id + `','code')">\u7F16\u8F91\u4EE3\u7801</button><button class="btn" onclick="yyOpenWorkspace('` + w.id + `','agent')">AI \u5E2E\u5199</button><button class="btn-s" onclick="yyArchiveProject('` + w.id + `')">\u79FB\u51FA\u5DE5\u4F5C\u533A</button></div></article>`;
  }).join("") : '<div class="yy-empty yy-full">\u521B\u5EFA\u4E00\u4E2A\u63D2\u4EF6\u5DE5\u4F5C\u533A\uFF0C\u9009\u62E9\u8BED\u8A00\u540E\u5F00\u59CB\u7F16\u8F91\u3002<br>\u6BCF\u4E2A\u5DE5\u4F5C\u533A\u5355\u72EC\u4FDD\u5B58\u6587\u4EF6\u3001\u5BF9\u8BDD\u548C\u672A\u5B8C\u6210\u4EFB\u52A1\u3002</div>';
  html += '</div><section class="card"><div class="yy-section-head"><h3>' + yyIcon("plugin") + '\u8FD0\u884C\u5B9E\u4F8B</h3><button class="yy-text-btn" onclick="Rplg()">\u5237\u65B0</button></div>' + (instances.length ? instances.map(function(p) {
    return '<div data-yy-plugin="' + yyEscape(p.id) + '" class="yy-runtime-row"><span class="yy-stat-icon">' + yyIcon("plugin") + "</span><div><strong>" + yyEscape(p.id) + "</strong><small>" + yyEscape(p.account) + " \xB7 " + yyEscape(p.language) + `</small></div><button class="yy-secondary" onclick="yyRuntimeAction('` + p.id + "','" + p.account + `','event')">\u89E6\u53D1</button><button class="yy-secondary" onclick="yyRuntimeAction('` + p.id + "','" + p.account + `','stop')">\u505C\u6B62</button></div>`;
  }).join("") : '<div class="yy-hint">\u6682\u65F6\u6CA1\u6709\u8FD0\u884C\u4E2D\u7684 Python / C++ \u5B9E\u4F8B\u3002</div>') + yyInstalledRuntimeHTML(results[1].installed || []) + '<pre id="yy-runtime-result" class="yy-readout"></pre></section><section class="yy-compat"><h3>Lua \u4E0E\u517C\u5BB9\u63D2\u4EF6\u7BA1\u7406</h3><div class="plg-section" style="margin-top:18px">' + compatibleHTML + "</div></section></div>";
  var host = document.getElementById("tab-tasks"), expanded = [];
  (_a = host.querySelectorAll) == null ? void 0 : _a.call(host, "[data-yy-plugin]").forEach(function(row) {
    var _a2;
    if (((_a2 = row.querySelector(".plg-body")) == null ? void 0 : _a2.style.display) === "block") expanded.push(row.getAttribute("data-yy-plugin"));
  });
  host.innerHTML = '<div id="yy-plugin-operation-status" class="card" role="status" aria-live="polite" hidden></div>' + html;
  (_b = host.querySelectorAll) == null ? void 0 : _b.call(host, "[data-yy-plugin]").forEach(function(row) {
    if (expanded.includes(row.getAttribute("data-yy-plugin"))) {
      row.querySelector(".plg-body").style.display = "block";
      var fold = row.querySelector("[data-fold]");
      if (fold) fold.style.transform = "rotate(180deg)";
    }
  });
  if (typeof yyPluginOperationUI === "function") yyPluginOperationUI();
};
yyLanguagePage = function() {
  yyNavigate("tasks");
};
async function yyRuntimeAction(id, account, action) {
  return yyPluginMutation(id, { start: "\u542F\u52A8\u63D2\u4EF6", stop: "\u505C\u6B62\u63D2\u4EF6", restart: "\u91CD\u8F7D\u63D2\u4EF6", event: "\u89E6\u53D1\u63D2\u4EF6" }[action] || "\u64CD\u4F5C\u63D2\u4EF6", "/api/youyuan/runtime/" + action, { id, account: account || "main", event: { type: "manual" } });
}
function yyCreateWorkspaceDialog(language, pluginID) {
  var ov = document.createElement("div");
  ov.className = "modal-overlay";
  ov.id = "yy-create-workspace";
  ov.innerHTML = '<div class="modal-box"><h3>\u65B0\u5EFA\u63D2\u4EF6\u5DE5\u4F5C\u533A</h3><p class="yy-hint">\u72EC\u7ACB\u6587\u4EF6\u76EE\u5F55\u3001\u4FEE\u6539\u5386\u53F2\u4E0E\u6301\u7EED\u4EFB\u52A1\u3002\u6A21\u578B\u5728 AI \u5E2E\u5199\u5185\u914D\u7F6E\u3002</p><label for="yy-new-name">\u5DE5\u4F5C\u533A\u540D\u79F0</label><input id="yy-new-name" placeholder="\u4F8B\u5982\uFF1A\u7FA4\u804A\u6307\u4EE4\u52A9\u624B"><label for="yy-new-language">\u63D2\u4EF6\u8BED\u8A00</label><select id="yy-new-language">' + Object.entries(yyLanguageNames).map(function(l) {
    return '<option value="' + l[0] + '" ' + (language === l[0] ? "selected" : "") + ">" + l[1] + "</option>";
  }).join("") + '</select><div class="yy-actions"><button class="btn" onclick="yyCreateWorkspace(' + JSON.stringify(pluginID || "").replace(/"/g, "&quot;") + `)">\u521B\u5EFA\u5E76\u7F16\u8F91</button><button class="btn-s" onclick="this.closest('.modal-overlay').remove()">\u53D6\u6D88</button></div></div>`;
  document.body.appendChild(ov);
}
async function yyCreateWorkspace(pluginID) {
  var r = await A("POST", "/api/youyuan/agent/workspaces", { name: document.getElementById("yy-new-name").value, language: document.getElementById("yy-new-language").value, plugin_id: pluginID || "" });
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  document.getElementById("yy-create-workspace").remove();
  await yyOpenWorkspace(r.id, "agent");
}
openAIAssist = async function(pluginID) {
  if (!pluginID) {
    yyCreateWorkspaceDialog();
    return;
  }
  var list = await A("GET", "/api/youyuan/agent/workspaces");
  if (!list.ok) {
    T(list.error, "e");
    return;
  }
  var found = (list.workspaces || []).find((w) => w.plugin_id === pluginID && w.language === "lua");
  if (found) {
    var sync = await A("POST", "/api/youyuan/agent/sync-plugin", { id: found.id });
    if (!sync.ok) T(sync.error, "e");
    await yyOpenWorkspace(found.id, "agent");
    return;
  }
  var plugins = await A("GET", "/api/plugin/list");
  var plugin = (plugins.plugins || []).find((p) => p.id === pluginID);
  if (plugin && plugin.type !== "lua") {
    T("\u5F53\u524D\u9879\u76EE\u578B AI \u652F\u6301 Lua \u6E90\u7801\uFF0C\u8BF7\u5728\u539F\u7F16\u8F91\u5668\u4FEE\u6539\u8BE5\u7C7B\u578B\u63D2\u4EF6", "e");
    return;
  }
  var created = await A("POST", "/api/youyuan/agent/workspaces", { name: (plugin == null ? void 0 : plugin.name) || pluginID, language: "lua", plugin_id: pluginID });
  if (!created.ok) {
    T("\u6E90\u7801\u63A5\u5165\u5931\u8D25\uFF1A" + created.error, "e");
    return;
  }
  await yyOpenWorkspace(created.id, "agent");
};
async function yySyncPluginSource() {
  if (yyEditorDirty && !await yySaveProjectFile()) return;
  var r = await A("POST", "/api/youyuan/agent/sync-plugin", { id: yyWorkspaceActive.id });
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  await yyOpenWorkspace(yyWorkspaceActive.id, "code");
}
function yyArchiveProject(id) {
  showConfirm({ title: "\u79FB\u51FA\u5DE5\u4F5C\u533A", message: "\u9879\u76EE\u6587\u4EF6\u4E0E\u4FEE\u6539\u5386\u53F2\u5C06\u79FB\u5165\u5DE5\u4F5C\u533A\u5907\u4EFD\u3002\u5DF2\u5B89\u88C5\u63D2\u4EF6\u7EE7\u7EED\u4FDD\u7559\u3002", confirmText: "\u5F52\u6863", onConfirm: async () => {
    var r = await A("POST", "/api/youyuan/agent/archive", { id });
    if (!r.ok) {
      T(r.error, "e");
      return;
    }
    await Rplg();
  } });
}
async function yyOpenWorkspace(id, pane) {
  var epoch = ++yyWorkspaceOpenEpoch, ownerScope = yyEditorAccountScope();
  if (yyEditorDirty && yyWorkspaceEditor) {
    if (!await yySaveProjectFile() || yyEditorDirty || epoch !== yyWorkspaceOpenEpoch || ownerScope !== yyEditorAccountScope()) return;
  }
  var r = await A("GET", "/api/youyuan/agent/workspaces?id=" + encodeURIComponent(id));
  if (epoch !== yyWorkspaceOpenEpoch || ownerScope !== yyEditorAccountScope()) return;
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  if (yyEditorDirty && yyWorkspaceEditor) {
    if (!await yySaveProjectFile() || yyEditorDirty || epoch !== yyWorkspaceOpenEpoch || ownerScope !== yyEditorAccountScope()) return;
  }
  yyWorkspaceFilePending = "";
  var ws = r.workspace;
  yyWorkspaceActive = ws;
  yyWorkspaceSeq = 0;
  yyEditorDirty = false;
  var title = "\u5DE5\u4F5C\u533A \xB7 " + ws.name;
  delete subPageCache[title];
  openSubPage(title, '<div class="yy-workspace-top">' + yyIntro(ws.name, yyLanguageNames[ws.language] + " \xB7 \u6587\u4EF6\u3001\u4EFB\u52A1\u548C\u5DE5\u5177\u7ED3\u679C\u72EC\u7ACB\u4FDD\u5B58\u3002") + `<div class="yy-actions"><input id="yy-workspace-account" value="main" aria-label="\u8FD0\u884C\u8D26\u53F7" style="width:130px;margin:0"><button class="yy-secondary" onclick="yyWorkspacePublish(false)">\u4FDD\u5B58\u5E76\u53D1\u5E03</button><button class="yy-primary" onclick="yyWorkspacePublish(true)">\u542F\u52A8\u63D2\u4EF6</button><button class="yy-secondary" onclick="yyWorkspaceStop()">\u505C\u6B62\u63D2\u4EF6</button></div></div><div class="yy-workspace-tabs"><button data-pane="code" onclick="yyWorkspacePane('code')">\u4EE3\u7801\u7F16\u8F91</button><button data-pane="agent" onclick="yyWorkspacePane('agent')">AI \u5E2E\u5199</button><button data-pane="context" onclick="yyWorkspacePane('context')">\u5386\u53F2 / MCP</button></div><div class="yy-workspace" id="yy-workspace"><section class="yy-code-pane"><div class="yy-editor-toolbar"><select id="yy-file-select" aria-label="\u9879\u76EE\u6587\u4EF6" onchange="yyOpenProjectFile(this.value)"></select><button class="yy-text-btn" onclick="yyNewProjectFile()">\u65B0\u5EFA\u6587\u4EF6</button><button class="yy-text-btn" onclick="yyUploadProject()">\u4E0A\u4F20\u9644\u4EF6</button></div><div id="yy-code-editor" class="yy-code-editor"></div><div class="yy-editor-footer"><span id="yy-editor-status">\u5DF2\u4FDD\u5B58</span><div><button class="yy-text-btn" onclick="yyCheckSyntax()">\u8BED\u6CD5\u68C0\u67E5</button><button class="yy-text-btn" onclick="yySaveProjectFile()">\u4FDD\u5B58</button></div></div><pre id="yy-editor-result" class="yy-readout" aria-live="polite"></pre><section class="yy-project-search"><label class="yy-search">` + yyIcon("search") + `<input id="yy-project-query" placeholder="\u8DE8\u6587\u4EF6\u641C\u7D22 / \u5F15\u7528\u5B9A\u4F4D" onkeydown="if(event.key==='Enter')yySearchProject()"></label><button class="yy-secondary" onclick="yySearchProject()">\u641C\u7D22</button></section><pre id="yy-project-search-result" class="yy-readout"></pre></section><section class="yy-agent-pane"><div class="yy-agent-header"><strong>` + yyIcon("spark") + ' AI \u5E2E\u5199</strong><span class="yy-badge" id="yy-agent-state">' + yyEscape(ws.state) + '</span></div><div class="yy-model-toolbar"><select id="yy-agent-model" aria-label="AI \u6A21\u578B" onchange="yySelectAgentModel(this.value)"></select><button class="yy-text-btn" onclick="yyAgentModels()">\u6A21\u578B\u914D\u7F6E</button></div><div class="yy-agent-chat" id="yy-agent-chat"><div class="yy-agent-welcome">' + yyIcon("spark") + `<strong>\u4ECE\u4E00\u4E2A\u76EE\u6807\u5F00\u59CB</strong><p>\u53EF\u4EE5\u4FEE\u6539\u9879\u76EE\u6587\u4EF6\u3001\u641C\u7D22\u5F15\u7528\u3001\u8C03\u7528\u5DE5\u5177\u7BB1\u4E0E MCP\u3002<br>\u8FD0\u884C\u4E2D\u7EE7\u7EED\u53D1\u6D88\u606F\uFF0C\u53EF\u4EE5\u5F15\u5BFC\u5F53\u524D\u4EFB\u52A1\u3002</p></div></div><button type="button" id="yy-agent-latest" class="yy-text-btn yy-agent-latest" hidden onclick="yyAgentJumpLatest()">\u56DE\u5230\u6700\u65B0\u72B6\u6001 \u2193</button><div class="yy-agent-compose"><textarea id="yy-agent-message" aria-label="\u5F15\u5BFC AI \u4EFB\u52A1" placeholder="\u63CF\u8FF0\u63D2\u4EF6\u9700\u6C42\uFF0C\u6216\u7EE7\u7EED\u5F15\u5BFC\u6B63\u5728\u8FD0\u884C\u7684\u4EFB\u52A1\u2026"></textarea><div class="yy-agent-compose-actions"><div><button class="yy-text-btn" onclick="yyAgentTask('pause')">\u6682\u505C</button><button class="yy-text-btn" onclick="yyAgentTask('resume')">\u7EE7\u7EED\u4EFB\u52A1</button></div><button class="yy-primary" onclick="yyAgentTask('send')">\u53D1\u9001 ` + yyIcon("arrow") + '</button></div></div></section></div><section class="yy-context-pane" id="yy-workspace-context"><div class="yy-form-grid"><section class="card">' + yyCardHead("\u4FEE\u6539\u5386\u53F2\u4E0E Diff", "clock") + '<button class="yy-secondary" onclick="yyLoadHistory()">\u67E5\u770B\u4FEE\u6539\u5386\u53F2</button><div id="yy-history-list"></div><pre id="yy-history-diff" class="yy-readout"></pre></section>' + yyMCPCardHTML() + '<section class="card yy-full">' + yyCardHead("\u5386\u53F2\u68C0\u7D22", "list") + '<details class="yy-context-diagnostics"><summary>\u4E0A\u4E0B\u6587\u8BCA\u65AD\uFF08\u53EF\u9009\uFF09</summary><pre class="yy-readout" id="yy-agent-summary">' + yyEscape(ws.summary || "\u4EFB\u52A1\u8BB0\u5FC6\u5728\u540E\u53F0\u7EF4\u62A4\uFF0C\u539F\u59CB\u5386\u53F2\u4ECD\u4FDD\u7559\u3002") + '</pre></details><label class="yy-search" style="margin-top:16px"><input id="yy-history-query" placeholder="\u6309\u5173\u952E\u8BCD\u68C0\u7D22\u4EFB\u52A1\u5386\u53F2"></label><div class="yy-actions"><button class="btn-s" onclick="yySearchHistory()">\u68C0\u7D22</button><button class="btn-s" onclick="yyWorkspaceSkill()">\u672C\u5DE5\u4F5C\u533A\u5F00\u53D1\u6587\u6863</button></div><pre class="yy-readout" id="yy-history-result"></pre></section></div></section>');
  if (ws.plugin_id) {
    var bar = document.createElement("div");
    bar.className = "yy-hint";
    bar.id = "yy-source-binding";
    bar.textContent = "\u5DF2\u7ED1\u5B9A\u63D2\u4EF6\uFF1A" + ws.plugin_id + " \xB7 \u6E90\u7801\u5165\u53E3\uFF1A" + (ws.source_entry || { lua: "main.lua", python: "main.py", native_cpp: "main.cpp", mcfunction: "main.mcfunction" }[ws.language]) + " \xB7 " + ws.files.length + " \u4E2A\u9879\u76EE\u6587\u4EF6";
    if (ws.language === "lua") {
      var sync = document.createElement("button");
      sync.className = "yy-text-btn";
      sync.textContent = "\u540C\u6B65\u5DF2\u5B89\u88C5\u6E90\u7801";
      sync.onclick = yySyncPluginSource;
      bar.appendChild(sync);
    }
    document.querySelector(".yy-workspace-top").appendChild(bar);
  }
  var live = () => epoch === yyWorkspaceOpenEpoch && ownerScope === yyEditorAccountScope() && document.getElementById("sub-page").classList.contains("show") && (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) === id;
  activeRenderers.push(function() {
    clearInterval(yyWorkspacePoll);
    if (yyEditorDirty && yyWorkspaceEditor) yySaveProjectFile();
    clearTimeout(yyEditorTimer);
    if (yyWorkspaceEditor) {
      yyWorkspaceEditor.destroy();
      yyWorkspaceEditor = null;
    }
  });
  yyWorkspacePane(pane || "code");
  yyDrawProjectFiles(ws.files);
  yyDrawMCP(ws.servers);
  yyBindAgentScroll();
  yyRestoreAgentConversation(ws);
  yyLoadAgentModels(ws.model).catch(function(e) {
    if (live()) T("\u6A21\u578B\u5217\u8868\u6682\u4E0D\u53EF\u7528\uFF0C\u53EF\u7EE7\u7EED\u7F16\u8F91\u6587\u4EF6\uFF1A" + e.message, "e");
  });
  yyWorkspaceFilePending = ws.source_entry || (ws.files[0] || {}).path || "main.py";
  if ((pane || "code") === "code") await yyEnsureWorkspaceFile();
  if (!live()) return;
  clearInterval(yyWorkspacePoll);
  yyWorkspacePoll = setInterval(yyPollWorkspace, 5e3);
}
function yyRestoreAgentConversation(ws) {
  var host = document.getElementById("yy-agent-chat");
  if (!host) return;
  var turns = ws.turns || [], lastTime = 0, fragment = document.createDocumentFragment(), start = Math.max(0, turns.length - 80), count = 0;
  if (start) {
    var older = document.createElement("button");
    older.className = "yy-secondary";
    older.textContent = "\u67E5\u770B\u66F4\u65E9\u7684\u5B8C\u6574\u5BF9\u8BDD";
    older.onclick = () => yyFullConversation();
    fragment.append(older);
  }
  turns.forEach(function(t, index) {
    var _a, _b, _c;
    lastTime = Math.max(lastTime, Date.parse(t.time) || 0);
    if (index < start) return;
    if (t.content) {
      var item = document.createElement("div");
      item.className = "yy-agent-message " + (t.role === "assistant" ? "assistant" : "user");
      item.textContent = t.content;
      item.dataset.yyTurnIndex = String(index);
      fragment.append(item);
      count++;
    }
    if (((_a = t.tool_calls) == null ? void 0 : _a.length) || ((_b = t.tool_results) == null ? void 0 : _b.length)) {
      var detail = document.createElement("details");
      detail.className = "yy-agent-message tool_result";
      var title = document.createElement("summary");
      title.textContent = ((_c = t.tool_calls) == null ? void 0 : _c.length) ? "\u5DE5\u5177\u8C03\u7528 \xB7 " + t.tool_calls.map((c) => {
        var _a2;
        return ((_a2 = c.function) == null ? void 0 : _a2.name) || c.name || "\u5DE5\u5177";
      }).join("\u3001") : "\u5DF2\u4FDD\u5B58\u7684\u5DE5\u5177\u7ED3\u679C";
      var pre = document.createElement("pre");
      pre.textContent = JSON.stringify({ calls: t.tool_calls, results: t.tool_results }, null, 2);
      detail.append(title, pre);
      fragment.append(detail);
      count++;
    }
  });
  if (count) host.replaceChildren(fragment);
  var events = ws.events || [], tail = events.filter((e) => (Date.parse(e.time) || 0) > lastTime);
  yyRenderAgentEvents(tail);
  var lastSeq = events.reduce((v, e) => Math.max(v, e.seq || 0), 0);
  yyWorkspaceSeq = Math.max(yyWorkspaceSeq, lastSeq);
  host.dataset.lastSeq = String(yyWorkspaceSeq);
  host.scrollTop = host.scrollHeight;
}
var yyWorkspaceFilePending = "";
async function yyEnsureWorkspaceFile() {
  var path = yyWorkspaceFilePending;
  if (!path) return;
  yyWorkspaceFilePending = "";
  await yyOpenProjectFile(path);
}
function yyWorkspacePane(pane) {
  var el = document.getElementById("yy-workspace");
  if (!el) return;
  el.dataset.pane = pane;
  document.getElementById("sub-page").dataset.workspacePane = pane;
  if (pane !== "agent") document.getElementById("sub-page").classList.remove("yy-agent-focus");
  document.getElementById("yy-workspace-context").classList.toggle("visible", pane === "context");
  document.querySelectorAll("[data-pane]").forEach(function(b) {
    b.classList.toggle("active", b.dataset.pane === pane);
  });
  if (pane === "code" && yyWorkspaceFilePending) yyEnsureWorkspaceFile();
  if (pane === "agent") {
    var chat = document.getElementById("yy-agent-chat");
    if (chat && chat.dataset.followLatest !== "false") chat.scrollTop = chat.scrollHeight;
  }
}
function yyDrawProjectFiles(files) {
  var el = document.getElementById("yy-file-select");
  if (!el) return;
  var value = el.value;
  var next = (files || []).map(function(f) {
    return '<option value="' + yyEscape(f.path) + '">' + yyEscape(f.path) + "</option>";
  }).join("");
  if (el.innerHTML !== next) {
    el.innerHTML = next;
    if (Array.from(el.options).some(function(o) {
      return o.value === value;
    })) el.value = value;
  }
}
function yyEditorAccountScope() {
  return typeof yyReadScope === "function" ? yyReadScope() : String(window.yyActiveAccount || "main");
}
var yyFileOpenEpoch = 0, yyEditorScope = "";
async function yyOpenProjectFile(path) {
  var owner = yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id, epoch = yyWorkspaceOpenEpoch, fileEpoch = ++yyFileOpenEpoch, ownerScope = yyEditorAccountScope();
  const live = () => {
    var _a;
    return owner === (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) && epoch === yyWorkspaceOpenEpoch && fileEpoch === yyFileOpenEpoch && ownerScope === yyEditorAccountScope() && ((_a = document.getElementById("sub-page")) == null ? void 0 : _a.classList.contains("show"));
  };
  if (yyEditorDirty) {
    var saved = await yySaveProjectFile();
    if (!saved || !live() || yyEditorDirty) return;
  }
  document.getElementById("yy-editor-status").textContent = "\u6B63\u5728\u6253\u5F00 \xB7 " + path;
  try {
    var r = await A("GET", "/api/youyuan/agent/file?id=" + encodeURIComponent(owner) + "&path=" + encodeURIComponent(path));
    if (!live()) return;
    if (!r.ok) throw Error(r.error || "\u8BFB\u53D6\u5931\u8D25");
    if (yyEditorDirty) {
      if (!await yySaveProjectFile() || !live() || yyEditorDirty) return;
    }
    yyEditorPath = path;
    yyEditorScope = ownerScope;
    yyFileBaseHash = r.hash || "";
    if (yyWorkspaceEditor) yyWorkspaceEditor.destroy();
    document.getElementById("yy-code-editor").innerHTML = "";
    yyWorkspaceEditor = null;
    if (r.binary) {
      var note = document.createElement("pre");
      note.className = "yy-readout";
      note.textContent = "\u4E8C\u8FDB\u5236\u7F16\u8BD1\u4EA7\u7269\uFF1A" + path + "\nSHA-256\uFF1A" + r.hash + "\n\u8BF7\u901A\u8FC7\u4E0A\u4F20\u6216\u7F16\u8BD1\u5DE5\u5177\u66FF\u6362\uFF0C\u4E0D\u5728\u6587\u672C\u7F16\u8F91\u5668\u4E2D\u4FEE\u6539\u3002";
      document.getElementById("yy-code-editor").appendChild(note);
      document.getElementById("yy-editor-status").textContent = "\u4E8C\u8FDB\u5236\u6587\u4EF6 \xB7 \u53EA\u8BFB";
      document.getElementById("yy-file-select").value = path;
      return;
    }
    yyWorkspaceEditor = YYEditor.create(document.getElementById("yy-code-editor"), r.content, path, yyWorkspaceActive.language, function() {
      yyEditorDirty = true;
      document.getElementById("yy-editor-status").textContent = "\u6B63\u5728\u7F16\u8F91 \xB7 \u5373\u5C06\u81EA\u52A8\u4FDD\u5B58";
      clearTimeout(yyEditorTimer);
      yyEditorTimer = setTimeout(yySaveProjectFile, 900);
    });
    document.getElementById("yy-file-select").value = path;
    document.getElementById("yy-editor-status").textContent = "\u5DF2\u4FDD\u5B58 \xB7 " + path;
  } catch (e) {
    if (live()) {
      document.getElementById("yy-editor-status").textContent = "\u6253\u5F00\u5931\u8D25 \xB7 " + path;
      T(e.message, "e");
    }
  }
}
async function yySaveProjectFile() {
  if (!yyWorkspaceEditor) return false;
  clearTimeout(yyEditorTimer);
  if (!yyEditorScope || yyEditorScope !== yyEditorAccountScope()) {
    T("\u8D26\u53F7\u6216\u767B\u5F55\u5DF2\u5207\u6362\uFF0C\u539F\u6587\u4EF6\u8349\u7A3F\u4ECD\u4FDD\u7559\uFF0C\u8BF7\u56DE\u5230\u539F\u8D26\u53F7\u4FDD\u5B58\u3002", "w");
    return false;
  }
  var editor = yyWorkspaceEditor, owner = yyWorkspaceActive.id, ownerScope = yyEditorScope, epoch = yyWorkspaceOpenEpoch, content = editor.state.doc.toString(), path = yyEditorPath;
  const live = () => editor === yyWorkspaceEditor && owner === (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) && epoch === yyWorkspaceOpenEpoch && ownerScope === yyEditorAccountScope() && yyEditorPath === path && document.getElementById("yy-editor-status");
  try {
    var r = await A("POST", "/api/youyuan/agent/file", { id: owner, path, content, base_hash: yyFileBaseHash });
    if (!live()) return !!r.ok;
    if (r.ok) {
      yyFileBaseHash = r.hash || "";
      if (editor.state.doc.toString() === content) yyEditorDirty = false;
      document.getElementById("yy-editor-status").textContent = yyEditorDirty ? "\u6709\u65B0\u7684\u7F16\u8F91\u5F85\u4FDD\u5B58" : "\u5DF2\u4FDD\u5B58 \xB7 " + path;
    } else {
      document.getElementById("yy-editor-status").textContent = "\u4FDD\u5B58\u5931\u8D25\uFF1A" + r.error;
      T(r.error, "e");
    }
    return !!r.ok;
  } catch (e) {
    if (live()) {
      document.getElementById("yy-editor-status").textContent = "\u4FDD\u5B58\u7ED3\u679C\u672A\u786E\u8BA4\uFF0C\u8BF7\u4FDD\u7559\u8349\u7A3F\u5E76\u6838\u5BF9\u6587\u4EF6";
      T(e.message, "e");
    }
    return false;
  }
}
async function yyNewProjectFile() {
  var path = prompt("\u9879\u76EE\u4E2D\u7684\u6587\u4EF6\u540D\uFF0C\u4F8B\u5982 utils.py / config.json");
  if (!path) return;
  var r = await A("POST", "/api/youyuan/agent/file", { id: yyWorkspaceActive.id, path, content: "" });
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  await yyPollWorkspace();
  await yyOpenProjectFile(path);
}
/* ── 危险操作授权 ──────────────────────────────────────────────
   前端发起的工具调用全部走 yyAgentTool，执行前先过这张表；
   AI 自己在任务里发起的高危工具，用「暂停任务 + 底部抽屉」拦下一步。
   记住的选择按工具名存在 localStorage，玩家随时可以重新打开询问。 */
var yyRiskToolMap = { mcp_call: "调用外部 MCP 工具", project_rollback: "回滚文件到旧版本", project_fetch_artifact: "从外部地址下载产物", plugin_manage: "管理插件（可能删除数据）" };
var yyRiskPattern = /(sendcommand|execute|setblock|summon|gamerule|deop|\bop\b|\bban\b|pardon|\bkick\b|\bkill\b|\bclear\b|\bgive\b|\btp\b|remove|delete|\brm\b|format)/i;
var yyRiskSessionAllow = {}, yyRiskOpenEl = null, yyRiskPending = null, yyRiskIntercepting = false;
function yyRiskOf(tool, args) {
  var name = String(tool || "").toLowerCase(), why = yyRiskToolMap[name] || "";
  if (name === "mcp_call") {
    var ext = String(args && args.tool || "");
    if (yyRiskPattern.test(ext)) why = "外部工具 " + ext + " 看起来会执行命令";
  }
  if (!why && yyRiskPattern.test(name)) why = "工具名里带危险动作";
  if (!why && args) {
    ["command", "cmd", "commands", "script"].forEach(function(k) {
      var v = args[k];
      if (!why && v && yyRiskPattern.test(String(v))) why = "参数里带命令";
    });
  }
  return why;
}
function yyRiskDetail(tool, args) {
  var text = "";
  try {
    text = JSON.stringify({ tool, args: args || {} }, null, 2);
  } catch (e) {
    text = String(tool);
  }
  return text.length > 800 ? text.slice(0, 800) + "\n…" : text;
}
function yyRiskPolicyAll(tool) {
  try {
    return JSON.parse(localStorage.getItem("yyRiskPolicy") || "{}")[tool] === "allow";
  } catch (e) {
    return false;
  }
}
function yyRiskRemember(tool) {
  try {
    var p = JSON.parse(localStorage.getItem("yyRiskPolicy") || "{}");
    p[tool] = "allow";
    localStorage.setItem("yyRiskPolicy", JSON.stringify(p));
  } catch (e) {
  }
}
function yyRiskPreapproved(tool) {
  return !!yyRiskSessionAllow["*"] || !!yyRiskSessionAllow[tool] || yyRiskPolicyAll(tool);
}
function yyRiskPolicies() {
  try {
    return JSON.parse(localStorage.getItem("yyRiskPolicy") || "{}");
  } catch (e) {
    return {};
  }
}
function yyDrawRiskPolicy() {
  var host = document.getElementById("yy-risk-policy-list");
  if (!host) return;
  var p = yyRiskPolicies(), keys = Object.keys(p).filter(function(k) {
    return p[k] === "allow";
  });
  var count = document.getElementById("yy-risk-policy-count");
  if (count) count.textContent = "已记住 " + keys.length + " 项";
  host.innerHTML = keys.length ? keys.map(function(k) {
    var arg = JSON.stringify(k).replace(/"/g, "&quot;");
    return '<div class="yy-mcp-row"><div><strong>' + yyEscape(k) + '</strong><small>已允许，不再询问</small></div><button class="yy-text-btn" onclick="yyRiskForget(' + arg + ')">恢复询问</button></div>';
  }).join("") : '<p class="yy-hint">还没有记住任何授权。</p>';
}
function yyRiskForget(tool) {
  var p = yyRiskPolicies();
  delete p[tool];
  try {
    localStorage.setItem("yyRiskPolicy", JSON.stringify(p));
  } catch (e) {
  }
  delete yyRiskSessionAllow[tool];
  delete yyRiskSessionAllow["*"];
  yyDrawRiskPolicy();
  T("已恢复询问：" + tool, "o");
}
function yyRiskClearPolicy() {
  try {
    localStorage.removeItem("yyRiskPolicy");
  } catch (e) {
  }
  yyRiskSessionAllow = {};
  yyDrawRiskPolicy();
  T("已清空授权记录", "o");
}
function yyRiskClose(choice, remember) {
  var ov = yyRiskOpenEl;
  yyRiskOpenEl = null;
  if (ov) {
    ov.classList.remove("open");
    setTimeout(function() {
      if (ov.parentNode) ov.parentNode.removeChild(ov);
    }, 220);
  }
  var done = yyRiskPending;
  yyRiskPending = null;
  if (done) done({ choice: choice || "deny", remember: !!remember });
}
function yyRiskConfirm(opts) {
  return new Promise(function(resolve) {
    if (yyRiskOpenEl) yyRiskClose("deny", false);
    yyRiskPending = resolve;
    var ov = document.createElement("div");
    ov.className = "yy-risk-overlay";
    ov.innerHTML = '<div class="yy-risk-sheet" role="dialog" aria-modal="true" aria-label="这一步有风险"><div class="yy-risk-grab" aria-hidden="true"></div><h3>这一步有风险</h3><p class="yy-risk-why">' + yyEscape(opts.why || "这个操作会真的改动东西") + '</p><div class="yy-risk-code"><span>' + yyEscape(opts.tool) + "</span><pre>" + yyEscape(opts.detail || "") + '</pre></div><label class="yy-risk-remember"><input type="checkbox" id="yy-risk-remember"><span>以后同类不再询问</span></label><div class="yy-risk-actions"><button type="button" class="yy-risk-deny">拒绝</button><button type="button" class="yy-risk-once">允许一次</button><button type="button" class="yy-risk-all">本次全部允许</button></div></div>';
    document.body.appendChild(ov);
    yyRiskOpenEl = ov;
    var sheet = ov.querySelector(".yy-risk-sheet");
    var pick = function(choice) {
      var box = document.getElementById("yy-risk-remember");
      var remembered = !!(box && box.checked);
      yyRiskClose(choice, remembered);
      // 勾了「以后同类不再询问」就当场落盘，不依赖调用方记得写
      if (remembered && choice !== "deny" && opts.tool) {
        yyRiskRemember(opts.tool);
        yyDrawRiskPolicy();
      }
    };
    ov.querySelector(".yy-risk-deny").onclick = function() {
      pick("deny");
    };
    ov.querySelector(".yy-risk-once").onclick = function() {
      pick("allow");
    };
    ov.querySelector(".yy-risk-all").onclick = function() {
      pick("allow_all");
    };
    ov.addEventListener("click", function(e) {
      if (e.target === ov) yyRiskClose("deny", false);
    });
    var startY = 0, dy = 0, dragging = false;
    var onDown = function(e) {
      dragging = true;
      startY = e.touches ? e.touches[0].clientY : e.clientY;
      dy = 0;
      sheet.style.transition = "none";
    };
    var onMove = function(e) {
      if (!dragging) return;
      var y = e.touches ? e.touches[0].clientY : e.clientY;
      dy = Math.max(0, y - startY);
      sheet.style.transform = "translateY(" + dy + "px)";
    };
    var onUp = function() {
      if (!dragging) return;
      dragging = false;
      sheet.style.transition = "";
      if (dy > 90) yyRiskClose("deny", false);
      else sheet.style.transform = "";
    };
    var grab = ov.querySelector(".yy-risk-grab");
    grab.addEventListener("touchstart", onDown, { passive: true });
    grab.addEventListener("touchmove", onMove, { passive: true });
    grab.addEventListener("touchend", onUp);
    grab.addEventListener("mousedown", onDown);
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
    requestAnimationFrame(function() {
      ov.classList.add("open");
    });
  });
}
async function yyRiskInterceptAgent(tool, args) {
  if (yyRiskIntercepting || yyRiskOpenEl) return;
  yyRiskIntercepting = true;
  try {
    await yyAgentTask("pause");
    var res = await yyRiskConfirm({ tool, detail: yyRiskDetail(tool, args), why: yyRiskOf(tool, args) });
    if (res.choice === "deny") {
      var input = document.getElementById("yy-agent-message");
      if (input) input.value = "跳过这一步：" + tool + " 已被玩家拒绝，请换一个不执行危险操作的做法。";
      T("已拒绝 " + tool + "，任务保持暂停", "w");
      return;
    }
    if (res.choice === "allow_all") yyRiskSessionAllow["*"] = true;
    if (res.remember) yyRiskRemember(tool);
    await yyAgentTask("resume");
  } finally {
    yyRiskIntercepting = false;
  }
}
async function yyAgentTool(tool, args) {
  var why = yyRiskOf(tool, args);
  if (why && !yyRiskPreapproved(tool)) {
    var res = await yyRiskConfirm({ tool, detail: yyRiskDetail(tool, args), why });
    if (res.choice === "deny") return { ok: false, error: "已拒绝执行 " + tool };
    if (res.choice === "allow_all") yyRiskSessionAllow["*"] = true;
    if (res.remember) yyRiskRemember(tool);
  }
  return A("POST", "/api/youyuan/agent/tool", { id: yyWorkspaceActive.id, tool, args: args || {} });
}
async function yySearchProject() {
  var r = await yyAgentTool("project_search", { query: document.getElementById("yy-project-query").value });
  document.getElementById("yy-project-search-result").textContent = r.result || r.error;
}
async function yyCheckSyntax() {
  if (!yyWorkspaceEditor) {
    T("\u8BF7\u9009\u62E9\u6587\u672C\u6E90\u6587\u4EF6", "e");
    return;
  }
  var r = await A("POST", "/api/youyuan/agent/validate", { id: yyWorkspaceActive.id, path: yyEditorPath, content: yyWorkspaceEditor.state.doc.toString() });
  document.getElementById("yy-editor-result").textContent = r.ok ? r.checked ? "\u8BED\u6CD5\u68C0\u67E5\u901A\u8FC7" : r.note : r.error;
}
async function yyWorkspacePublish(start) {
  if (yyEditorDirty && !await yySaveProjectFile()) return;
  var r = await yyAgentTool("project_publish");
  document.getElementById("yy-editor-result").textContent = r.result || r.error;
  if (!r.ok || !start) return;
  var published;
  try {
    published = JSON.parse(r.result);
  } catch (e) {
    return;
  }
  if (!published.ok) {
    T(published.error || "\u53D1\u5E03\u5931\u8D25", "e");
    return;
  }
  var path = "/api/youyuan/runtime/start", body = { id: published.id, account: document.getElementById("yy-workspace-account").value };
  if (yyWorkspaceActive.language === "lua") {
    path = "/api/plugin/run";
    yyWorkspaceActive.plugin_id = published.id;
  } else if (yyWorkspaceActive.language === "mcfunction") {
    path = "/api/youyuan/chains";
    body = { name: yyWorkspaceActive.id + ".txt", action: "start" };
  }
  ;
  var result = await A("POST", path, body);
  document.getElementById("yy-editor-result").textContent = JSON.stringify(result, null, 2);
}
async function yyWorkspaceStop() {
  var path = "/api/youyuan/runtime/stop", body = { id: yyWorkspaceActive.id, account: document.getElementById("yy-workspace-account").value };
  if (yyWorkspaceActive.language === "lua") {
    path = "/api/plugin/stop";
    body = { id: yyWorkspaceActive.plugin_id };
  } else if (yyWorkspaceActive.language === "mcfunction") {
    path = "/api/youyuan/chains";
    body = { name: yyWorkspaceActive.id + ".txt", action: "stop" };
  }
  ;
  var r = await A("POST", path, body);
  document.getElementById("yy-editor-result").textContent = JSON.stringify(r, null, 2);
}
function yyUploadProject() {
  var input = document.createElement("input");
  input.type = "file";
  input.multiple = true;
  input.onchange = async function() {
    if (!input.files.length) return;
    var form = new FormData();
    Array.from(input.files).forEach(function(file) {
      form.append("files", file);
    });
    try {
      var r = await (await fetch("/api/youyuan/agent/upload?id=" + encodeURIComponent(yyWorkspaceActive.id), { method: "POST", body: form })).json();
      if (!r.ok) T(r.error, "e");
      else {
        T("\u9644\u4EF6\u5DF2\u52A0\u5165\u9879\u76EE", "o");
        await yyPollWorkspace();
      }
    } catch (e) {
      T(e.message, "e");
    }
  };
  input.click();
}
var yyAgentModelsEpoch = 0;
async function yyLoadAgentModels(selected) {
  var owner = yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id, ownerScope = yyEditorAccountScope(), epoch = yyWorkspaceOpenEpoch, request = ++yyAgentModelsEpoch, el = document.getElementById("yy-agent-model");
  var results = await Promise.all([A("GET", "/api/ai/models"), A("GET", "/api/youyuan/agent/cloud-model")]);
  if (owner !== (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) || ownerScope !== yyEditorAccountScope() || epoch !== yyWorkspaceOpenEpoch || request !== yyAgentModelsEpoch || el !== document.getElementById("yy-agent-model")) return;
  if (results[0].ok === false) throw Error(results[0].error || "\u6A21\u578B\u5217\u8868\u8BFB\u53D6\u5931\u8D25");
  __ai.models = (results[0].models || []).filter((m) => m.name !== "\u6E38\u56ED\u4E91\u7AEF AI" || results[1].allowed);
  if (!el) return;
  el.innerHTML = __ai.models.length ? __ai.models.map(function(m) {
    return '<option value="' + yyEscape(m.name) + '">' + yyEscape(m.name) + "</option>";
  }).join("") : '<option value="">\u5148\u914D\u7F6E\u6A21\u578B</option>';
  if (selected && __ai.models.some((m) => m.name === selected)) el.value = selected;
}
async function yyAgentModels() {
  await yyLoadAgentModels(document.getElementById("yy-agent-model").value);
  aiOpenManage();
  var modal = document.querySelector("#ai-mlist").closest(".modal-overlay");
  var observer = new MutationObserver(function() {
    if (!document.body.contains(modal)) {
      observer.disconnect();
      yyLoadAgentModels(yyWorkspaceActive.model);
    }
  });
  observer.observe(document.body, { childList: true });
}
var yyAgentTaskPending = /* @__PURE__ */ new Set();
async function yyAgentTask(action) {
  var id = yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id, account = window.yyActiveAccount || "main", input = document.getElementById("yy-agent-message");
  if (!id || !input) return;
  var message = action === "send" ? input.value.trim() : "", key = account + ":" + id;
  if (action === "send" && !message) return;
  if (action !== "pause" && yyAgentTaskPending.has(key)) return;
  if (action !== "pause") yyAgentTaskPending.add(key);
  try {
    var r = await A("POST", "/api/youyuan/agent/task", { id, action, message, model: document.getElementById("yy-agent-model").value });
    if (id !== (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) || account !== (window.yyActiveAccount || "main") || !input.isConnected) return;
    if (!r.ok) {
      T(r.error, "e");
      return;
    }
    if (message && input.value.trim() === message) input.value = "";
    await yyPollWorkspace();
  } catch (e) {
    if (id === (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) && account === (window.yyActiveAccount || "main")) T("\u8BF7\u6C42\u672A\u786E\u8BA4\uFF1A" + e.message + "\u3002\u5148\u68C0\u67E5\u4EFB\u52A1\u72B6\u6001\uFF0C\u4E0D\u4F1A\u81EA\u52A8\u91CD\u53D1\u3002", "e");
  } finally {
    if (action !== "pause") yyAgentTaskPending.delete(key);
  }
}
function yyAgentUpdateLatestButton(host) {
  var button = document.getElementById("yy-agent-latest");
  if (!button) return;
  button.hidden = host.dataset.followLatest !== "false";
  var pending = Number(host.dataset.pendingEvents || 0);
  button.textContent = (pending ? "\u6709\u65B0\u72B6\u6001 \xB7 " : "") + "\u56DE\u5230\u6700\u65B0\u72B6\u6001 \u2193";
}
function yyBindAgentScroll() {
  var host = document.getElementById("yy-agent-chat");
  if (!host) return;
  host.dataset.followLatest = "true";
  host.dataset.pendingEvents = "0";
  host.addEventListener("scroll", function(event) {
    if (event.target !== host) {
      if (event.target.scrollHeight - event.target.clientHeight - event.target.scrollTop > 32) {
        host.dataset.followLatest = "false";
        yyAgentUpdateLatestButton(host);
      }
      return;
    }
    if (!host.clientHeight) return;
    host.dataset.followLatest = host.scrollHeight - host.clientHeight - host.scrollTop <= 32 ? "true" : "false";
    if (host.dataset.followLatest === "true") host.dataset.pendingEvents = "0";
    yyAgentUpdateLatestButton(host);
  }, { passive: true, capture: true });
}
function yyAgentJumpLatest() {
  var host = document.getElementById("yy-agent-chat");
  if (!host) return;
  host.dataset.followLatest = "true";
  host.dataset.pendingEvents = "0";
  while (host.childNodes.length > 300) host.firstChild.remove();
  host.scrollTop = host.scrollHeight;
  yyAgentUpdateLatestButton(host);
}
function yyRenderAgentEvents(events) {
  var host = document.getElementById("yy-agent-chat");
  if (!host) return;
  var follow = host.dataset.followLatest !== "false";
  if (host.clientHeight && host.scrollHeight - host.clientHeight - host.scrollTop > 32) follow = false;
  host.dataset.followLatest = follow ? "true" : "false";
  var originalTop = host.scrollTop, changed = 0;
  events.filter(function(e) {
    return e.seq > Math.max(yyWorkspaceSeq, Number(host.dataset.lastSeq || 0));
  }).forEach(function(e) {
    yyWorkspaceSeq = e.seq;
    host.dataset.lastSeq = String(e.seq);
    if (e.kind === "usage" || e.kind === "thinking" || e.kind === "file" || e.kind === "summary") return;
    if (e.kind === "assistant" && !e.text) {
      var incomplete = host.querySelector(".yy-live-output");
      if (incomplete) incomplete.classList.remove("yy-live-output");
      return;
    }
    changed++;
    var empty = host.querySelector(".yy-agent-welcome");
    if (empty) empty.remove();
    if (e.kind === "chunk") {
      var live = host.querySelector(".yy-live-output");
      if (!live) {
        live = document.createElement("div");
        live.className = "yy-agent-message assistant yy-live-output";
        host.appendChild(live);
      }
      live.textContent += e.text;
      return;
    }
    if (e.kind === "assistant") {
      var live = host.querySelector(".yy-live-output");
      if (live) {
        live.classList.remove("yy-live-output");
        live.textContent = e.text.trim();
        return;
      }
    }
    var el = document.createElement(e.kind === "tool_result" || e.kind === "tool_start" ? "details" : "div");
    el.className = "yy-agent-message " + e.kind;
    if (el.tagName === "DETAILS") {
      var summary = document.createElement("summary");
      var toolName = e.kind === "tool_start" ? String(e.text || "").split(" ")[0] : "";
      summary.textContent = e.kind === "tool_start" ? "已执行 " + (toolName || "工具") : "工具结果 " + String(e.text || "").split("\n")[0].slice(0, 42);
      el.appendChild(summary);
      var pre = document.createElement("pre");
      pre.textContent = e.text;
      el.appendChild(pre);
      if (e.kind === "tool_start" && toolName) {
        var riskArgs = {};
        try {
          riskArgs = JSON.parse(String(e.text).slice(toolName.length).trim() || "{}");
        } catch (err) {
          riskArgs = {};
        }
        var riskWhy = yyRiskOf(toolName, riskArgs);
        if (riskWhy) {
          el.classList.add("risk");
          summary.textContent = "⚠ 已执行 " + toolName;
          var stateEl = document.getElementById("yy-agent-state");
          var runningNow = stateEl && String(stateEl.textContent || "").indexOf("运行中") === 0;
          if (runningNow && !yyRiskPreapproved(toolName)) yyRiskInterceptAgent(toolName, riskArgs);
        }
      }
    } else el.textContent = e.kind === "assistant" ? e.text.trim() : e.text;
    host.appendChild(el);
  });
  if (!changed) return;
  if (follow) {
    while (host.childNodes.length > 300) host.firstChild.remove();
    if (host.clientHeight) host.scrollTop = host.scrollHeight;
  } else {
    host.scrollTop = originalTop;
    host.dataset.pendingEvents = String(Number(host.dataset.pendingEvents || 0) + changed);
  }
  yyAgentUpdateLatestButton(host);
}
var yyWorkspacePollBusy = false;
async function yyPollWorkspace() {
  if (yyWorkspacePollBusy || document.hidden) return;
  yyWorkspacePollBusy = true;
  try {
    if (!document.getElementById("yy-workspace") || !yyWorkspaceActive) return;
    var id = yyWorkspaceActive.id;
    var r = await A("GET", "/api/youyuan/agent/workspaces?id=" + encodeURIComponent(id) + "&compact=1&after=" + yyWorkspaceSeq);
    if (!r.ok || !document.getElementById("yy-workspace") || yyWorkspaceActive.id !== id) return;
    var ws = r.workspace;
    yyWorkspaceActive = ws;
    document.getElementById("yy-agent-state").textContent = ws.state + (ws.error ? " \xB7 \u6709\u9519\u8BEF" : "");
    yyDrawProjectFiles(ws.files);
    yyDrawMCP(ws.servers);
    var changed = (ws.events || []).some(function(e) {
      return e.seq > yyWorkspaceSeq && e.kind === "file" && String(e.text).startsWith(yyEditorPath + " \xB7");
    });
    if (changed && !yyEditorDirty && yyWorkspaceEditor) await yyOpenProjectFile(yyEditorPath);
    yyRenderAgentEvents(ws.events || []);
    document.getElementById("yy-agent-summary").textContent = ws.summary || "\u5C1A\u65E0\u538B\u7F29\u6458\u8981\uFF1B\u539F\u59CB\u5386\u53F2\u5DF2\u4FDD\u5B58\u3002";
  } finally {
    yyWorkspacePollBusy = false;
  }
}
async function yyLoadHistory() {
  var r = await yyAgentTool("project_history");
  var list;
  try {
    list = JSON.parse(r.result);
  } catch (e) {
    T(r.error || "\u5386\u53F2\u8BFB\u53D6\u5931\u8D25", "e");
    return;
  }
  document.getElementById("yy-history-list").innerHTML = list.map(function(rev) {
    return '<div class="yy-history-row"><div><strong>' + yyEscape(rev.path) + "</strong><small>" + yyEscape(rev.time) + `</small></div><button class="yy-text-btn" onclick="yyShowDiff('` + rev.id + `')">Diff</button><button class="yy-text-btn" onclick="yyRollback('` + rev.id + `')">\u56DE\u6EDA</button></div>`;
  }).join("");
}
async function yyShowDiff(id) {
  var r = await yyAgentTool("project_history", { revision: id });
  document.getElementById("yy-history-diff").textContent = r.result || r.error;
}
async function yyRollback(id) {
  var r = await yyAgentTool("project_rollback", { revision: id });
  document.getElementById("yy-history-diff").textContent = r.result || r.error;
  if (r.ok) {
    await yyLoadHistory();
    if (!yyEditorDirty) await yyOpenProjectFile(yyEditorPath);
  }
}
async function yySearchHistory() {
  var r = await yyAgentTool("project_history", { query: document.getElementById("yy-history-query").value });
  document.getElementById("yy-history-result").textContent = r.result || r.error;
}
async function yyWorkspaceSkill() {
  var r = await A("GET", "/api/youyuan/docs?language=" + encodeURIComponent(yyWorkspaceActive.language));
  document.getElementById("yy-history-result").textContent = r.content || r.error;
}
// 本机工具箱：只读清单，AI 默认只调用这些工具，不联网、不经过别人的服务。
var yyBuiltinTools = [
  ["project_list", "列出项目文件"],
  ["project_read", "读取源码（分页）"],
  ["project_search", "跨文件检索"],
  ["project_write", "写入文件并生成修订"],
  ["project_patch", "精确替换片段"],
  ["project_validate", "语法检查"],
  ["project_history", "修改历史与 Diff"],
  ["project_publish", "发布到插件运行目录"],
  ["game_scene", "读取游戏观测"],
  ["packet_schema", "查询包字段"],
  ["terminal_log", "连接与插件日志"],
  ["skill_search", "搜索离线技能"],
  ["load_skill", "读取技能章节"],
  ["plugin_manage", "启动 / 停止 / 重载插件"]
];
function yyMCPCardHTML() {
  return '<section class="card yy-mcp-card">' + yyCardHead("工具箱与 MCP", "connection") + '<div class="yy-mcp-builtin"><div class="yy-mcp-group-head"><strong>本机工具箱</strong><span class="yy-badge">默认 · 只读</span></div><p class="yy-hint">AI 默认只调用这里的本机工具，不联网、不经过别人的服务。</p><ul class="yy-mcp-builtin-list">' + yyBuiltinTools.map(function(t) {
    return "<li><code>" + t[0] + "</code><span>" + t[1] + "</span></li>";
  }).join("") + '</ul></div><div class="yy-mcp-group-head"><strong>外部 MCP</strong><span class="yy-badge yy-badge-risk">高级</span></div><p class="yy-hint">外部 MCP 会让 AI 调用<b>你自己的外部服务</b>，不是本机工具箱。</p><div id="yy-mcp-servers"></div><details class="yy-mcp-external" id="yy-mcp-external"><summary>外部 MCP（高级 · 会让 AI 调用你自己的外部服务，不是本机工具箱）</summary><p class="yy-mcp-warn">外部 MCP 会让 AI 调用你自己的外部服务（HTTP 地址），不是本机工具箱；只填写你信任的地址。</p><label for="yy-mcp-name">名称</label><input id="yy-mcp-name" placeholder="例如：远程编译器 / 文件系统"><label for="yy-mcp-url">Streamable HTTP 地址</label><input id="yy-mcp-url" placeholder="https://你的服务器/mcp"><label for="yy-mcp-token">Bearer Token</label><input id="yy-mcp-token" type="password" placeholder="留空保留已保存的 Token">' + yySwitch("yy-mcp-enabled", "启用此 MCP", true) + '<label class="yy-mcp-ack"><input id="yy-mcp-ack" type="checkbox"><span>我已知晓这会把工具调用发给外部服务</span></label><div class="yy-actions"><button class="btn-s" id="yy-mcp-save" disabled onclick="yySaveMCP()">添加 / 保存</button></div><label for="yy-mcp-tool">直接调用工具（外部）</label><input id="yy-mcp-tool" placeholder="工具名称"><textarea id="yy-mcp-args" placeholder="工具参数 JSON">{}</textarea><div class="yy-actions"><button class="btn-s" onclick="yyCallMCP()">执行工具</button></div><pre id="yy-mcp-result" class="yy-readout"></pre></details><div class="yy-mcp-group-head"><strong>危险操作授权</strong><span class="yy-badge" id="yy-risk-policy-count">已记住 0 项</span></div><p class="yy-hint">允许过并勾选「以后同类不再询问」的工具，下次不再弹窗；随时可以在这里恢复询问。</p><div id="yy-risk-policy-list"></div><div class="yy-actions"><button class="btn-s" onclick="yyRiskClearPolicy()">清空已记住的授权</button></div></section>';
}
function yyDrawMCP(servers) {
  var host = document.getElementById("yy-mcp-servers");
  if (!host) return;
  host.innerHTML = (servers || []).map(function(s, i) {
    return '<div class="yy-mcp-row external"><div><strong>' + yyEscape(s.id) + '<span class="yy-badge yy-badge-risk">外部</span></strong><small>' + yyEscape(s.url) + (s.enabled === false ? " · 已停用" : "") + '</small></div><button class="yy-text-btn" onclick="yySelectMCP(yyWorkspaceActive.servers[' + i + '].id)">\u914D\u7F6E</button><button class="yy-text-btn" onclick="yyMCPList(yyWorkspaceActive.servers[' + i + `].id,'tools')">Tools</button><button class="yy-text-btn" onclick="yyMCPList(yyWorkspaceActive.servers[` + i + `].id,'resources')">Resources</button><button class="yy-text-btn" onclick="yyDeleteMCP(yyWorkspaceActive.servers[` + i + '].id)">\u5220\u9664</button></div>';
  }).join("") || '<p class="yy-hint">还没有配置外部 MCP。AI 只会使用上面的本机工具箱。</p>';
  var ack = document.getElementById("yy-mcp-ack");
  if (ack && !host.dataset.ackInit) {
    host.dataset.ackInit = "1";
    var remembered = false;
    try {
      remembered = localStorage.getItem("yyMcpAck") === "1";
    } catch (e) {
    }
    ack.checked = remembered && (servers || []).length > 0;
    ack.addEventListener("change", function() {
      try {
        localStorage.setItem("yyMcpAck", ack.checked ? "1" : "0");
      } catch (e) {
      }
      yySyncMCPAck();
    });
  }
  yySyncMCPAck();
  yyDrawRiskPolicy();
}
function yySyncMCPAck() {
  var ack = document.getElementById("yy-mcp-ack"), save = document.getElementById("yy-mcp-save");
  if (save) save.disabled = !(ack && ack.checked);
}
function yySelectMCP(id) {
  var server = yyWorkspaceActive.servers.find(function(s) {
    return s.id === id;
  });
  var fold = document.getElementById("yy-mcp-external");
  if (fold) fold.open = true;
  document.getElementById("yy-mcp-name").value = server.id;
  document.getElementById("yy-mcp-url").value = server.url;
  document.getElementById("yy-mcp-enabled").checked = server.enabled;
  var ack = document.getElementById("yy-mcp-ack");
  if (ack) ack.checked = true;
  yySyncMCPAck();
}
async function yySaveMCP() {
  var ack = document.getElementById("yy-mcp-ack"), fold = document.getElementById("yy-mcp-external");
  if (ack && !ack.checked) {
    if (fold) fold.open = true;
    T("请先勾选「我已知晓」：外部 MCP 不是本机工具箱", "e");
    if (ack) ack.focus();
    return;
  }
  var r = await A("POST", "/api/youyuan/agent/mcp", { id: yyWorkspaceActive.id, action: "save", server: { id: document.getElementById("yy-mcp-name").value, url: document.getElementById("yy-mcp-url").value, token: document.getElementById("yy-mcp-token").value, enabled: document.getElementById("yy-mcp-enabled").checked } });
  if (r.ok) {
    try {
      localStorage.setItem("yyMcpAck", "1");
    } catch (e) {
    }
  }
  if (!r.ok) T(r.error, "e");
  else {
    document.getElementById("yy-mcp-token").value = "";
    await yyPollWorkspace();
  }
}
async function yyDeleteMCP(id) {
  var r = await A("POST", "/api/youyuan/agent/mcp", { id: yyWorkspaceActive.id, action: "delete", server: { id } });
  if (!r.ok) T(r.error, "e");
  await yyPollWorkspace();
}
async function yyMCPList(id, kind) {
  var r = await yyAgentTool("mcp_list", { server: id, kind });
  document.getElementById("yy-mcp-result").textContent = r.result || r.error;
}
async function yyCallMCP() {
  var args;
  try {
    args = JSON.parse(document.getElementById("yy-mcp-args").value);
  } catch (e) {
    T("\u5DE5\u5177\u53C2\u6570\u4E0D\u662F\u6709\u6548 JSON", "e");
    return;
  }
  var r = await yyAgentTool("mcp_call", { server: document.getElementById("yy-mcp-name").value, tool: document.getElementById("yy-mcp-tool").value, arguments: args });
  document.getElementById("yy-mcp-result").textContent = r.result || r.error;
}
function yyInstalledRuntimeHTML(installed) {
  var button = (fn, args, label) => '<button class="btn-s" onclick="' + yyEscape(fn + "(" + args.map((a) => JSON.stringify(a)).join(",") + ")") + '">' + label + "</button>";
  return '<div class="yy-section-head yy-installed-runtime"><h3>\u5DF2\u5B89\u88C5\u63D2\u4EF6</h3></div>' + installed.filter((p) => yyLanguageFilter === "all" || p.language === yyLanguageFilter).map((p) => '<div data-yy-plugin="' + yyEscape(p.id) + '" class="yy-runtime-row" style="flex-wrap:wrap;align-items:flex-start"><div><strong>' + yyEscape(p.name || p.id) + "</strong><small>" + yyEscape(p.id) + " \xB7 " + yyEscape(yyLanguageNames[p.language] || p.language) + '</small></div><div class="yy-actions">' + button("yyRuntimeAction", [p.id, "main", "start"], "\u542F\u52A8") + button("yyRuntimeAction", [p.id, "main", "restart"], "\u91CD\u65B0\u52A0\u8F7D") + button("yyRuntimeAction", [p.id, "main", "stop"], "\u505C\u6B62") + button("yySDKOpenPanel", [p.id, "main"], "\u63A7\u5236\u9762\u677F") + button("yyOpenInstalledRuntime", [p.id, p.language], "\u7F16\u8F91 / AI") + button("yyRuntimeDirectoryView", [p.id], "\u67E5\u770B\u76EE\u5F55") + button("yyExportRuntimePlugin", [p.id], "\u5BFC\u51FA ZIP") + button("yyDeleteInstalledRuntime", [p.id], "\u5220\u9664") + "</div></div>").join("") + '<div class="yy-actions">' + button("yyOpenLanguageDoc", ["python-runtime"], "Python \u5B8C\u6574\u624B\u518C") + button("yyOpenLanguageDoc", ["cpp-runtime"], "C++ \u5B8C\u6574\u624B\u518C") + "</div>";
}
async function yyOpenInstalledRuntime(id, language) {
  var r = await A("GET", "/api/youyuan/agent/workspaces");
  var w = (r.workspaces || []).find((w2) => (w2.id === id || w2.plugin_id === id) && w2.language === language);
  if (!w) {
    r = await A("POST", "/api/youyuan/agent/workspaces", { id, name: id, language, plugin_id: id });
    if (!r.ok) {
      T(r.error, "e");
      return;
    }
    w = { id };
  }
  await yyOpenWorkspace(w.id, "code");
}
async function yyRuntimeDirectoryView(id) {
  var r = await A("GET", "/api/youyuan/runtime/files?id=" + encodeURIComponent(id));
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  var ov = document.createElement("div");
  ov.className = "modal-overlay";
  ov.innerHTML = '<div class="modal-box"><h3>\u63D2\u4EF6\u76EE\u5F55</h3><p class="yy-hint">' + yyEscape(r.directory) + '</p><pre class="yy-readout" style="max-height:50vh;overflow:auto">' + yyEscape(r.files.map((f) => f.path + (f.directory ? "/" : " \xB7 " + f.size + " B")).join("\n")) + `</pre><button class="btn-s" onclick="this.closest('.modal-overlay').remove()">\u8FD4\u56DE</button></div>`;
  document.body.appendChild(ov);
}
function yyDeleteInstalledRuntime(id) {
  var owner = yyReadScope();
  showConfirm({ title: "\u5220\u9664\u63D2\u4EF6", message: "\u5220\u9664 " + yyEscape(id) + " \u7684\u4EE3\u7801\u3001\u914D\u7F6E\u548C\u63D2\u4EF6\u6570\u636E\u3002\u5DE5\u4F5C\u533A\u5386\u53F2\u4FDD\u7559\uFF1B\u9700\u8981\u4FDD\u7559\u63D2\u4EF6\u8D44\u6599\u53EF\u5148\u5BFC\u51FA ZIP\u3002\u5927\u76EE\u5F55\u5220\u9664\u65F6\u4F1A\u663E\u793A\u7B49\u5F85\u72B6\u6001\u3002", confirmText: "\u5220\u9664", onConfirm: () => {
    if (owner !== yyReadScope()) {
      T("\u8D26\u53F7\u5DF2\u5207\u6362\uFF0C\u8BF7\u91CD\u65B0\u786E\u8BA4\u5220\u9664", "w");
      return;
    }
    return yyPluginMutation(id, "\u5220\u9664\u63D2\u4EF6", "/api/youyuan/runtime/delete", { id });
  } });
}
async function yySelectAgentModel(name) {
  var ws = yyWorkspaceActive;
  if (!ws) return false;
  var r = await A("POST", "/api/youyuan/agent/model", { id: ws.id, model: name });
  if (!r.ok) {
    T(r.error || "\u6A21\u578B\u9009\u62E9\u4FDD\u5B58\u5931\u8D25", "e");
    if ((yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) === ws.id) {
      var el = document.getElementById("yy-agent-model");
      if (el) el.value = ws.model || "";
    }
    return false;
  }
  ws.model = r.model;
  return true;
}
