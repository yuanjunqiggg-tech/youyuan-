function yyAgentProfileModal(title) {
  var old = document.getElementById("yy-agent-profile-modal");
  if (old) old.remove();
  var ov = document.createElement("div");
  ov.className = "modal-overlay";
  ov.id = "yy-agent-profile-modal";
  var box = document.createElement("div");
  box.className = "modal-box yy-agent-profile-box";
  box.innerHTML = '<div class="yy-form-head"><h3>' + yyEscape(title) + `</h3><button class="yy-text-btn" onclick="this.closest('.modal-overlay').remove()">\u5173\u95ED</button></div><div id="yy-agent-profile-body"></div>`;
  ov.appendChild(box);
  document.body.appendChild(ov);
  return document.getElementById("yy-agent-profile-body");
}
async function yyAgentPreferences() {
  var owner = yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id, account = window.yyActiveAccount || "main";
  var r = await A("GET", "/api/youyuan/agent/preferences?id=" + encodeURIComponent(owner));
  if (owner !== (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) || account !== (window.yyActiveAccount || "main")) return;
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  window.yyAgentPreferenceData = r;
  var host = yyAgentProfileModal(yyLanguageNames[r.language] + " \xB7 \u63D0\u793A\u8BCD\u3001\u6280\u80FD\u4E0E\u5DE5\u5177");
  host.innerHTML = '<p class="yy-hint">\u5404\u5DE5\u4F5C\u533A\u72EC\u7ACB\u4FDD\u5B58\u914D\u7F6E\u3002\u8BED\u8A00\u57FA\u7840\u63D0\u793A\u8BCD\u7531\u5BBF\u4E3B\u63D0\u4F9B\uFF0C\u81EA\u5B9A\u4E49\u5185\u5BB9\u4F5C\u4E3A\u9644\u52A0\u8981\u6C42\u3002\u4FEE\u6539\u7528\u4E8E\u4E0B\u4E00\u6B21\u6A21\u578B\u8C03\u7528\u3002</p><label>\u81EA\u5B9A\u4E49\u63D0\u793A\u8BCD</label><textarea id="yy-agent-custom-prompt" style="min-height:150px"></textarea><label>\u52A0\u8F7D\u7684\u6280\u80FD</label><div class="yy-profile-checks">' + r.available_skills.map((s) => {
    var _a;
    return '<label><input type="checkbox" data-agent-skill="' + s + '" ' + (r.skills.includes(s) ? "checked" : "") + ">" + yyEscape(((_a = (r.skill_catalog || []).find((x) => x.id === s)) == null ? void 0 : _a.name) || s) + "</label>";
  }).join("") + '</div><label>\u5141\u8BB8 Agent \u4F7F\u7528\u7684\u5DE5\u5177</label><p class="yy-hint">MCP / api_call \u5C5E\u4E8E\u901A\u7528\u63A5\u53E3\uFF1B\u52FE\u9009\u51B3\u5B9A Agent \u53EF\u89C1\u548C\u53EF\u8C03\u7528\u7684\u5DE5\u5177\uFF0C\u4E0D\u66FF\u4EE3\u6E38\u620F\u6743\u9650\u6216\u7CFB\u7EDF\u6388\u6743\u3002</p><div class="yy-profile-checks">' + r.tools.map((t) => '<label title="' + yyEscape(t.description) + '"><input type="checkbox" data-agent-tool="' + t.name + '" ' + (r.allowed_tools.includes(t.name) ? "checked" : "") + ">" + yyEscape(t.name) + "</label>").join("") + '</div><div class="yy-actions"><button class="btn" onclick="yySaveAgentPreferences()">\u4FDD\u5B58\u5DE5\u4F5C\u533A\u914D\u7F6E</button><button class="btn-s" onclick="yyLanguageDocument()">\u672C\u8BED\u8A00\u5F00\u53D1\u8BF4\u660E</button></div><pre id="yy-agent-preference-result" class="yy-readout"></pre>';
  document.getElementById("yy-agent-custom-prompt").value = r.custom_prompt || "";
}
async function yySaveAgentPreferences() {
  var r = await A("POST", "/api/youyuan/agent/preferences", { id: yyWorkspaceActive.id, custom_prompt: document.getElementById("yy-agent-custom-prompt").value, skills: Array.from(document.querySelectorAll("[data-agent-skill]:checked")).map((e) => e.dataset.agentSkill), allowed_tools: Array.from(document.querySelectorAll("[data-agent-tool]:checked")).map((e) => e.dataset.agentTool), model: document.getElementById("yy-agent-model").value });
  document.getElementById("yy-agent-preference-result").textContent = r.ok ? "\u5DF2\u4FDD\u5B58\uFF0C\u5404\u5DE5\u4F5C\u533A\u4E92\u4E0D\u5F71\u54CD\u3002" : r.error;
}
async function yyLanguageDocument(language) {
  var _a;
  language = language || yyWorkspaceActive.language;
  var skill = { python: "python-runtime", native_cpp: "cpp-runtime", lua: "lua-runtime", mcfunction: "command-chain" }[language];
  (_a = document.getElementById("yy-agent-profile-modal")) == null ? void 0 : _a.remove();
  await yyOpenLanguageDoc(skill);
}
var yyOpenAgentTabs = [], yyAgentMatchedTurns = /* @__PURE__ */ new Set();
var yyWorkspaceProfileBase = yyOpenWorkspace;
yyOpenWorkspace = async function(id, pane) {
  await yyWorkspaceProfileBase(id, pane);
  if (!document.getElementById("yy-workspace") || yyWorkspaceActive.id !== id || !document.getElementById("sub-page").classList.contains("show")) return;
  yyAgentMatchedTurns = /* @__PURE__ */ new Set();
  yyRestoreAgentTabs();
  if (!yyOpenAgentTabs.includes(id)) yyOpenAgentTabs.push(id);
  yyPersistAgentTabs();
  var top = document.querySelector(".yy-workspace-top");
  if (!document.getElementById("yy-open-agent-tabs")) {
    var tabs = document.createElement("div");
    tabs.id = "yy-open-agent-tabs";
    tabs.className = "yy-open-agent-tabs";
    document.querySelector(".yy-agent-header").after(tabs);
  }
  yyRenderOpenAgentTabs();
  document.querySelectorAll(".yy-profile-toolbar,#yy-agent-usage,.yy-agent-scroll-controls").forEach((e) => e.remove());
  var bar = document.createElement("details");
  bar.className = "yy-profile-toolbar";
  bar.innerHTML = '<summary role="button">\u6280\u80FD\u4E0E\u8BBE\u7F6E</summary><div class="yy-agent-menu"><button class="yy-text-btn" onclick="yyAgentPreferences()">\u63D0\u793A\u8BCD / \u5DE5\u5177\u6743\u9650</button><button class="yy-text-btn" onclick="yyOpenSkillLibrary()">\u6280\u80FD\u52A0\u8F7D\u5E93</button><button class="yy-text-btn" onclick="yyLanguageDocument()">\u672C\u8BED\u8A00\u5F00\u53D1\u6587\u6863</button></div>';
  document.querySelector(".yy-model-toolbar").append(bar);
  A("GET", "/api/youyuan/agent/cloud-model").then(function(r) {
    if (!r.allowed || yyWorkspaceActive.id !== id || !bar.isConnected) return;
    var button = document.createElement("button");
    button.className = "yy-text-btn";
    button.textContent = "\u542F\u7528\u6E38\u56ED\u4E91\u7AEF AI";
    button.onclick = yyUseCloudModel;
    bar.querySelector(".yy-agent-menu").append(button);
  });
  var usage = document.createElement("div");
  usage.id = "yy-agent-usage";
  usage.className = "yy-agent-usage";
  document.querySelector(".yy-agent-header").after(usage);
  var nav = document.createElement("div");
  nav.className = "yy-agent-scroll-controls";
  nav.innerHTML = '<span>\u5BF9\u8BDD</span><button type="button" onclick="yyToggleAgentFocus(this)">\u5C55\u5F00\u5BF9\u8BDD</button><button type="button" onclick="yyFullConversation()">\u5B8C\u6574\u8BB0\u5F55</button><button type="button" onclick="yyAgentJumpLatest()">\u6700\u65B0 \u2193</button>';
  document.getElementById("yy-agent-chat").before(nav);
  yyBindAgentWheel();
  yyRenderAgentUsage(yyWorkspaceActive);
  yyEnhanceAgentMessages();
  var account = document.getElementById("yy-workspace-account");
  account.value = "main";
  account.readOnly = true;
  account.title = "\u5F53\u524D\u72EC\u7ACB\u8D26\u53F7\u5185\u6838\u7684\u4E3B\u5B9E\u4F8B";
};
function yyToggleAgentFocus(button) {
  var page = document.getElementById("sub-page");
  var enabled = page.classList.toggle("yy-agent-focus");
  button.textContent = enabled ? "\u8FD4\u56DE\u5DE5\u4F5C\u533A" : "\u5C55\u5F00\u5BF9\u8BDD";
}
function yyAgentTabsKey() {
  return "yy-agent-tabs:" + (window.yyActiveAccount || "main");
}
function yyRestoreAgentTabs() {
  try {
    yyOpenAgentTabs = JSON.parse(localStorage.getItem(yyAgentTabsKey()) || "[]").filter((x) => typeof x === "string").slice(-12);
  } catch (e) {
    yyOpenAgentTabs = [];
  }
}
function yyPersistAgentTabs() {
  try {
    localStorage.setItem(yyAgentTabsKey(), JSON.stringify(yyOpenAgentTabs.slice(-12)));
  } catch (e) {
  }
}
function yyAgentScrollPage(direction) {
  var host = document.getElementById("yy-agent-chat");
  if (!host) return;
  host.scrollTop += direction * Math.max(120, host.clientHeight * 0.8);
}
function yyBindAgentWheel() {
  var pane = document.querySelector(".yy-agent-pane"), host = document.getElementById("yy-agent-chat");
  if (!pane || !host) return;
  pane.addEventListener("wheel", function(event) {
    var target = event.target;
    if (target.closest("textarea,input,select,.yy-agent-scroll-controls")) return;
    var nested = target.closest("pre");
    if (nested && nested.scrollHeight > nested.clientHeight + 2) return;
    var scale = event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? host.clientHeight : 1;
    var delta = event.deltaY * scale;
    if (!delta || host.scrollHeight <= host.clientHeight + 2) return;
    var before = host.scrollTop, max = host.scrollHeight - host.clientHeight;
    if (delta < 0 && before <= 0 || delta > 0 && before >= max) return;
    event.preventDefault();
    host.scrollTop = Math.max(0, Math.min(max, before + delta));
  }, { passive: false, capture: true });
}
function yyRenderOpenAgentTabs() {
  var el = document.getElementById("yy-open-agent-tabs");
  if (!el) return;
  el.replaceChildren();
  for (var id of yyOpenAgentTabs) {
    var group = document.createElement("span");
    group.className = "yy-session-tab";
    var button = document.createElement("button");
    button.className = "yy-secondary" + ((yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) === id ? " active" : "");
    button.textContent = (yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) === id ? yyWorkspaceActive.name : yyAgentTabNames[id] || "\u5DF2\u4FDD\u5B58\u4F1A\u8BDD";
    button.dataset.session = id;
    button.onclick = /* @__PURE__ */ (function(x) {
      return () => yyOpenWorkspace(x, "agent");
    })(id);
    var close = document.createElement("button");
    close.className = "yy-secondary";
    close.textContent = "\xD7";
    close.title = "\u5173\u95ED\u7A97\u53E3\uFF0C\u4FDD\u7559\u9879\u76EE\u4E0E\u5BF9\u8BDD";
    close.onclick = /* @__PURE__ */ (function(x) {
      return () => yyCloseAgentTab(x);
    })(id);
    group.append(button, close);
    el.append(group);
  }
  var tools = document.createElement("span");
  tools.className = "yy-session-actions";
  tools.innerHTML = '<button class="yy-secondary" onclick="yyNewAgentSession()">\uFF0B \u65B0\u4F1A\u8BDD</button><button class="yy-secondary" onclick="yySavedAgentSessions()">\u8BFB\u6863 / \u5386\u53F2\u4F1A\u8BDD</button><button class="yy-secondary" onclick="yySaveAgentSession()">\u4FDD\u5B58</button>';
  el.append(tools);
}
var yyAgentTabNames = {};
function yyCloseAgentTab(id) {
  yyOpenAgentTabs = yyOpenAgentTabs.filter((x) => x !== id);
  yyPersistAgentTabs();
  if ((yyWorkspaceActive == null ? void 0 : yyWorkspaceActive.id) === id) {
    if (yyOpenAgentTabs.length) yyOpenWorkspace(yyOpenAgentTabs[yyOpenAgentTabs.length - 1], "agent");
    else yyNavigate("tasks");
  } else yyRenderOpenAgentTabs();
}
async function yyNewAgentSession() {
  if (yyEditorDirty && !await yySaveProjectFile()) return;
  var r = await A("POST", "/api/youyuan/agent/session", { id: yyWorkspaceActive.id });
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  await yyOpenWorkspace(r.id, "agent");
  T("\u72EC\u7ACB\u4F1A\u8BDD\u5DF2\u521B\u5EFA\uFF0C\u590D\u5236\u5F53\u524D\u9879\u76EE\u6587\u4EF6\uFF0C\u539F\u5BF9\u8BDD\u4FDD\u7559", "o");
}
async function yySaveAgentSession() {
  if (yyEditorDirty && !await yySaveProjectFile()) return;
  var r = await A("POST", "/api/youyuan/agent/save", { id: yyWorkspaceActive.id });
  T(r.ok ? "\u6587\u4EF6\u3001\u5BF9\u8BDD\u4E0E\u4EFB\u52A1\u72B6\u6001\u5DF2\u4FDD\u5B58\uFF1B\u53EF\u4ECE\u201C\u8BFB\u6863 / \u5386\u53F2\u4F1A\u8BDD\u201D\u6062\u590D" : r.error, r.ok ? "o" : "e");
}
async function yySavedAgentSessions() {
  var r = await A("GET", "/api/youyuan/agent/workspaces");
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  var host = yyAgentProfileModal("\u8BFB\u6863 / \u5386\u53F2\u4F1A\u8BDD");
  host.innerHTML = '<p class="yy-hint">\u5BF9\u8BDD\u3001\u9879\u76EE\u548C\u4EFB\u52A1\u81EA\u52A8\u4FDD\u5B58\u5728\u5F53\u524D\u8D26\u53F7\u3002\u6253\u5F00\u6062\u590D\u8BB0\u5F55\uFF1B\u672A\u5B8C\u6210\u4EFB\u52A1\u53EF\u70B9\u201C\u7EE7\u7EED\u4EFB\u52A1\u201D\u3002\u4E0D\u540C\u4F1A\u8BDD\u72EC\u7ACB\u7F16\u8F91\uFF0C\u53D1\u5E03\u540C\u4E00\u4E2A\u63D2\u4EF6\u65F6\u4EE5\u6700\u540E\u4E00\u6B21\u53D1\u5E03\u4E3A\u51C6\u3002</p>';
  for (var ws of r.workspaces || []) {
    yyAgentTabNames[ws.id] = ws.name;
    var row = document.createElement("div");
    row.className = "yy-history-row";
    var text = document.createElement("div");
    text.textContent = ws.name + " \xB7 " + (yyLanguageNames[ws.language] || ws.language) + " \xB7 " + ws.state;
    var button = document.createElement("button");
    button.className = "yy-secondary";
    button.textContent = "\u6253\u5F00 / \u6062\u590D";
    button.onclick = /* @__PURE__ */ (function(id) {
      return async () => {
        var _a;
        (_a = document.getElementById("yy-agent-profile-modal")) == null ? void 0 : _a.remove();
        await yyOpenWorkspace(id, "agent");
      };
    })(ws.id);
    row.append(text, button);
    host.append(row);
  }
  yyRenderOpenAgentTabs();
}
async function yyFullConversation() {
  var id = yyWorkspaceActive.id, host = yyAgentProfileModal("\u5B8C\u6574\u5BF9\u8BDD \xB7 " + yyWorkspaceActive.name);
  host.closest(".modal-box").classList.add("yy-conversation-box");
  host.innerHTML = '<div class="yy-conversation-controls"><button class="yy-secondary" id="yy-conversation-older">\u52A0\u8F7D\u66F4\u65E9\u8BB0\u5F55</button><span id="yy-conversation-count"></span></div><div id="yy-conversation-records" tabindex="0"></div>';
  var before = null, loading = false, records = host.querySelector("#yy-conversation-records"), older = host.querySelector("#yy-conversation-older");
  async function load() {
    var _a, _b, _c;
    if (loading) return;
    loading = true;
    older.disabled = true;
    try {
      var r = await A("GET", "/api/youyuan/agent/conversation?id=" + encodeURIComponent(id) + (before === null ? "" : "&before=" + before));
      if (!r.ok) throw Error(r.error);
      if (!host.isConnected) return;
      var fragment = document.createDocumentFragment();
      for (var t of r.turns) {
        var item = document.createElement("article");
        item.className = "yy-agent-message " + t.role;
        var label = document.createElement("small");
        label.textContent = (t.role === "user" ? "\u73A9\u5BB6 / \u5DE5\u5177\u8FD4\u56DE" : "AI") + " \xB7 " + (t.time ? new Date(t.time).toLocaleString() : "\u5DF2\u4FDD\u5B58");
        item.append(label);
        if (t.content) {
          var body = document.createElement("div");
          body.textContent = t.content;
          item.append(body);
          var copy = document.createElement("button");
          copy.className = "yy-text-btn";
          copy.textContent = "\u590D\u5236";
          copy.onclick = /* @__PURE__ */ ((text) => () => yyCopyAgentText(text))(t.content);
          item.append(copy);
          if (t.role === "user" && !((_a = t.tool_results) == null ? void 0 : _a.length)) {
            var edit = document.createElement("button");
            edit.className = "yy-text-btn";
            edit.textContent = "\u4FEE\u6539 / \u5206\u53C9";
            edit.onclick = /* @__PURE__ */ ((index, text) => () => yyEditAgentMessage(index, text))(t.index, t.content);
            item.append(edit);
          }
        }
        if (((_b = t.tool_calls) == null ? void 0 : _b.length) || ((_c = t.tool_results) == null ? void 0 : _c.length)) {
          var details = document.createElement("details"), summary = document.createElement("summary"), pre = document.createElement("pre");
          summary.textContent = "\u5DE5\u5177\u8C03\u7528\u4E0E\u7ED3\u679C";
          pre.textContent = JSON.stringify({ calls: t.tool_calls, results: t.tool_results }, null, 2);
          details.append(summary, pre);
          item.append(details);
        }
        fragment.append(item);
      }
      var height = records.scrollHeight, top = records.scrollTop, first = before === null;
      records.prepend(fragment);
      before = r.before;
      records.scrollTop = first ? records.scrollHeight : top + records.scrollHeight - height;
      older.hidden = !r.has_more;
      host.querySelector("#yy-conversation-count").textContent = "\u5171 " + r.total + " \u6761\u5DF2\u4FDD\u5B58\u8BB0\u5F55 \xB7 \u539F\u59CB\u5185\u5BB9\u4FDD\u7559";
    } catch (e) {
      T(e.message, "e");
    } finally {
      loading = false;
      older.disabled = false;
    }
  }
  older.onclick = load;
  await load();
}
function yyRenderAgentUsage(ws) {
  var el = document.getElementById("yy-agent-usage");
  if (!el || !ws) return;
  var u = ws.usage || {}, cost = Number(u.estimated_cost || 0);
  el.textContent = "Token\uFF1A\u8F93\u5165 " + Number(u.input_tokens || 0) + " \xB7 \u8F93\u51FA " + Number(u.output_tokens || 0) + (cost ? " \xB7 \u4F30\u7B97\u8D39\u7528 " + cost.toFixed(5) + "\uFF08\u6309\u6A21\u578B\u81EA\u586B\u5355\u4EF7\uFF09" : " \xB7 \u672A\u8BBE\u7F6E\u5355\u4EF7\uFF0C\u4EC5\u7EDF\u8BA1\u7528\u91CF");
}
function yyCopyAgentText(text) {
  if (navigator.clipboard && navigator.clipboard.writeText) return navigator.clipboard.writeText(text).then(function() {
    T("\u5DF2\u590D\u5236", "o");
  });
  var box = document.createElement("textarea");
  box.value = text;
  document.body.appendChild(box);
  box.select();
  document.execCommand("copy");
  box.remove();
  T("\u5DF2\u590D\u5236", "o");
}
function yyEnhanceAgentMessages() {
  var host = document.getElementById("yy-agent-chat");
  if (!host || !yyWorkspaceActive) return;
  var turns = yyWorkspaceActive.turns || [];
  host.querySelectorAll(".yy-agent-message.user,.yy-agent-message.assistant").forEach(function(el) {
    if (el.classList.contains("yy-live-output")) return;
    if (!el.dataset.yyOriginal) el.dataset.yyOriginal = el.textContent;
    var text = el.dataset.yyOriginal;
    if (!el.dataset.yyCopy) {
      var copy = document.createElement("button");
      copy.className = "yy-text-btn yy-message-action";
      copy.textContent = "\u590D\u5236";
      copy.onclick = function() {
        yyCopyAgentText(text);
      };
      el.appendChild(copy);
      el.dataset.yyCopy = "1";
    }
    if (!el.classList.contains("user") || el.dataset.yyBranchReady) return;
    var index = el.dataset.yyTurnIndex !== void 0 ? Number(el.dataset.yyTurnIndex) : turns.findIndex(function(t, i) {
      return t.role === "user" && t.content === text && !yyAgentMatchedTurns.has(i);
    });
    if (index < 0) return;
    yyAgentMatchedTurns.add(index);
    el.dataset.yyTurnIndex = String(index);
    el.dataset.yyBranchReady = "1";
    var edit = document.createElement("button");
    edit.className = "yy-text-btn yy-message-action";
    edit.textContent = "\u4FEE\u6539 / \u5206\u53C9";
    edit.onclick = function() {
      yyEditAgentMessage(index, text);
    };
    el.appendChild(edit);
    var resend = document.createElement("button");
    resend.className = "yy-text-btn yy-message-action";
    resend.textContent = "\u4ECE\u8FD9\u91CC\u91CD\u53D1";
    resend.onclick = function() {
      yyForkAgentMessage(index, text);
    };
    el.appendChild(resend);
  });
}
function yyEditAgentMessage(index, text) {
  var host = yyAgentProfileModal("\u4ECE\u8FD9\u6761\u6D88\u606F\u521B\u5EFA\u5206\u652F");
  host.innerHTML = '<p class="yy-hint">\u539F\u5BF9\u8BDD\u4FDD\u7559\uFF1B\u65B0\u5206\u652F\u4ECE\u9009\u4E2D\u6D88\u606F\u91CD\u65B0\u6267\u884C\u3002</p><textarea id="yy-branch-message" style="min-height:150px"></textarea><div class="yy-actions"><button class="btn" onclick="yyForkAgentMessage(' + index + `,document.getElementById('yy-branch-message').value)">\u521B\u5EFA\u5E76\u7EE7\u7EED</button></div>`;
  document.getElementById("yy-branch-message").value = text;
}
async function yyForkAgentMessage(index, text) {
  var r = await A("POST", "/api/youyuan/agent/fork", { id: yyWorkspaceActive.id, turn_index: index, message: text });
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  var modal = document.getElementById("yy-agent-profile-modal");
  if (modal) modal.remove();
  await yyOpenWorkspace(r.id, "agent");
  T(r.files_rewound ? "\u5DF2\u521B\u5EFA\u5206\u652F\u5E76\u6062\u590D\u8BE5\u6D88\u606F\u4E4B\u524D\u7684\u9879\u76EE\u6587\u4EF6" : "\u5DF2\u521B\u5EFA\u5206\u652F\uFF1B\u65E7\u6D88\u606F\u7F3A\u5C11\u6587\u4EF6\u65F6\u95F4\u6233\uFF0C\u8BF7\u6838\u5BF9\u9879\u76EE\u6587\u4EF6", r.files_rewound ? "o" : "w");
}
var yyAgentPollBase = yyPollWorkspace;
yyPollWorkspace = async function() {
  await yyAgentPollBase();
  yyRenderAgentUsage(yyWorkspaceActive);
  yyEnhanceAgentMessages();
};
async function yyUseCloudModel() {
  var r = await A("POST", "/api/youyuan/agent/cloud-model", {});
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  await yyLoadAgentModels(r.model);
  T("\u5DF2\u9009\u62E9\u81EA\u5DF1\u7684\u4E91\u7AEF\u6A21\u578B\uFF1B\u4E5F\u53EF\u5728\u6A21\u578B\u914D\u7F6E\u4E2D\u6539\u7528\u5176\u4ED6\u4F9B\u5E94\u5546", "o");
}
