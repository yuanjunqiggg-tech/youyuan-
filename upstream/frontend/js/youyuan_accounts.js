var yyAccountPage = 1, yyAccountPageSize = 8, yyAccountFocusID = "", yyAccountRows = [], yyMonitorID = "main", yyAccountTimer = null, yyObservations = null;
async function yyRootAPI(method, path, body) {
  try {
    var r = await yyRootFetch(path, { method, headers: { "Content-Type": "application/json" }, body: body ? JSON.stringify(body) : void 0 });
    return await r.json();
  } catch (e) {
    return { ok: false, error: e.message };
  }
}
function yyAccountName(id) {
  var row = (typeof yyAllAccountRows !== "undefined" ? yyAllAccountRows : yyAccountRows).find((a) => a.id === id);
  return row ? row.name : "\u4E3B\u8D26\u53F7";
}
async function Raccounts() {
  document.getElementById("tab-accounts").innerHTML = '<div class="content-inner">' + yyHeading("\u8D26\u53F7\u4E0E\u9879\u76EE", "\u6E38\u620F\u8D26\u53F7\u4E0E\u670D\u52A1\u5668\u9879\u76EE\uFF0C\u7EDF\u4E00\u7BA1\u7406\u3002") + `<div class="yy-actions"><button class="yy-primary" onclick="yyEditAccount()">\u6DFB\u52A0\u8D26\u53F7</button><button class="yy-secondary" onclick="yyCenterRefresh()">\u5237\u65B0\u8D26\u53F7</button><button class="yy-secondary" hidden onclick="yyEnterAccount('main')">\u8FDB\u5165\u4E3B\u8D26\u53F7</button></div><p class="yy-hint">\u7B2C 5 \u4E2A\u5DE5\u4F5C\u7A7A\u95F4 \xB7 \u5F53\u524D\u64CD\u4F5C\uFF1A<b id="yy-current-account"></b>\u3002\u9009\u62E9\u8D26\u53F7\u540E\uFF0C\u53EF\u8FDE\u63A5\u670D\u52A1\u5668\u5E76\u7BA1\u7406\u5BF9\u5E94\u7684\u673A\u5668\u4EBA\u3002</p><div id="yy-account-overview" class="yy-account-overview"></div><div class="yy-center-tabs"><button class="btn-s" onclick="yyAccountPanel('accounts')">\u6E38\u620F\u8D26\u53F7</button><button class="btn-s" id="yy-admin-pool-entry" hidden onclick="yyPoolPanel(true)">\u7BA1\u7406\u5458\u6258\u7BA1\u6C60</button><button class="btn-s" onclick="yyGameAuthPage()">\u6CE8\u518C\u4E0E\u8BA4\u8BC1\u4EFB\u52A1</button></div><div id="yy-pool-panel" hidden></div><div id="yy-game-accounts-panel"><div id="yy-projects" class="yy-projects"></div><div class="yy-account-toolbar"><input id="yy-account-search" aria-label="\u641C\u7D22\u8D26\u53F7" placeholder="\u641C\u7D22\u6635\u79F0\u3001UID\u6216\u670D\u52A1\u5668" oninput="yyAccountPage=1;yyRenderAccountCards()"><select id="yy-account-page-size" aria-label="\u6BCF\u9875\u8D26\u53F7\u6570" onchange="yyAccountPageSize=Number(this.value);yyAccountPage=1;yyRenderAccountCards()"><option value="8">\u6BCF\u9875 8 \u4E2A</option><option value="16">\u6BCF\u9875 16 \u4E2A</option><option value="32">\u6BCF\u9875 32 \u4E2A</option></select></div><div id="yy-accounts-grid" class="yy-form-grid yy-account-list"></div><div id="yy-account-pages" class="yy-pagination"></div><p id="yy-account-cloud-status" class="yy-hint" role="status">\u6B63\u5728\u540C\u6B65\u5DF2\u6388\u6743\u6E38\u620F\u51ED\u636E\u2026</p></div><section class="card yy-monitor"><div class="yy-form-head"><h3>` + yyIcon("connection") + '\u8FD0\u884C\u8BB0\u5F55</h3><select id="yy-monitor-account" aria-label="\u67E5\u770B\u8D26\u53F7" onchange="yyMonitorID=this.value;yyRefreshMonitor()"></select></div><div class="yy-metrics" id="yy-monitor-metrics"></div><p class="yy-hint" id="yy-online-source"></p><details class="yy-monitor-trends"><summary>\u5728\u7EBF\u4EBA\u6570\u8D8B\u52BF\u4E0E\u5206\u5E03</summary><h4>\u5728\u7EBF\u4EBA\u6570 \xB7 \u65F6\u95F4\u7EBF</h4><div id="yy-online-chart" class="yy-chart"></div><h4>\u6BCF\u5C0F\u65F6\u4EBA\u6570\u5206\u5E03 \xB7 \u89C2\u6D4B\u5747\u503C</h4><div id="yy-distribution-chart" class="yy-chart"></div></details><div class="yy-section-head"><h3>\u804A\u5929\u8BB0\u5F55</h3></div><div class="yy-chat-filters"><label>\u641C\u7D22\u6D88\u606F<input id="yy-chat-filter" aria-label="\u641C\u7D22\u804A\u5929\u8BB0\u5F55" placeholder="\u8F93\u5165\u6D88\u606F\u5173\u952E\u8BCD" oninput="yyChatFilterChanged()"></label><label>\u73A9\u5BB6<select id="yy-chat-player" aria-label="\u6309\u73A9\u5BB6\u7B5B\u9009\u804A\u5929" onchange="yyChatFilterChanged()"><option value="">\u6240\u6709\u73A9\u5BB6</option></select></label><label>\u5F00\u59CB\u65F6\u95F4<input id="yy-chat-start" type="datetime-local" aria-label="\u804A\u5929\u5F00\u59CB\u65F6\u95F4" onchange="yyChatFilterChanged()"></label><label>\u7ED3\u675F\u65F6\u95F4\uFF08\u542B\u6B64\u5206\u949F\uFF09<input id="yy-chat-end" type="datetime-local" aria-label="\u804A\u5929\u7ED3\u675F\u65F6\u95F4" onchange="yyChatFilterChanged()"></label><label>\u6C47\u603B\u95F4\u9694<select id="yy-chat-interval" aria-label="\u804A\u5929\u6C47\u603B\u95F4\u9694" onchange="yyChatFilterChanged()"><option value="3600000">\u6BCF\u5C0F\u65F6</option><option value="900000">\u6BCF 15 \u5206\u949F</option><option value="86400000">\u6BCF\u5929</option></select></label><button class="btn-s" onclick="yyChatResetFilters()">\u6E05\u7A7A\u7B5B\u9009</button></div><div class="yy-section-head"><h4>\u65F6\u6BB5\u6D88\u606F\u91CF \xB7 \u70B9\u51FB\u67E5\u770B</h4><button class="btn-s" id="yy-chat-all-buckets" onclick="yyChatChooseBucket(null,null)" hidden>\u5168\u90E8\u65F6\u6BB5</button></div><div id="yy-chat-bars" class="yy-chat-bars"></div><p id="yy-chat-summary" class="yy-hint" role="status"></p><div id="yy-chat-table" class="yy-chat-messages"></div><div id="yy-chat-pages" class="yy-pagination"></div><p class="yy-hint">\u804A\u5929\u6309\u6700\u65B0\u65F6\u95F4\u6392\u5E8F\uFF1B\u7B5B\u9009\u4E0E\u65F6\u6BB5\u7EDF\u8BA1\u4EC5\u9488\u5BF9\u5F53\u524D\u8D26\u53F7\u5B9E\u9645\u4FDD\u7559\u7684\u8BB0\u5F55\uFF0C\u5185\u6838\u6700\u591A\u4FDD\u5B58\u6700\u8FD1 10,000 \u6761\u3002\u65F6\u95F4\u6309\u672C\u8BBE\u5907\u65F6\u533A\u663E\u793A\u3002</p></section></div>';
  yyUpdateAdminPoolEntry();
  yyRenderAccountSnapshot();
  clearInterval(yyAccountTimer);
  yyAccountTimer = setInterval(function() {
    if (curTab === "accounts" && !subPageOpen) yyRefreshAccounts();
  }, 1e4);
  activeRenderers.push(function() {
    clearInterval(yyAccountTimer);
  });
  await Promise.all([yyRefreshAccounts(), yyCenterCloudAccounts()]);
}
var yyAccountSnapshot = null, yyAccountRead = null, yyAccountRefreshEpoch = 0, yyAccountRowsScope = "", yyAccountMainConfigured = null;
function yyAccountControllerScope() {
  try {
    return JSON.stringify([typeof yyPlatformUser !== "undefined" && yyPlatformUser ? yyPlatformUser.id || yyPlatformUser.username || "" : "", typeof sessionStorage !== "undefined" ? sessionStorage.getItem("yy-cloud-key") || "" : ""]);
  } catch (e) {
    return "";
  }
}
async function yyReadAccountList(force) {
  const scope = yyAccountControllerScope();
  if (!force && (yyAccountRead == null ? void 0 : yyAccountRead.scope) === scope) return yyAccountRead.promise;
  const pending = { scope, promise: null };
  pending.promise = yyRootAPI("GET", "/api/youyuan/accounts?view=summary").then((r) => {
    if (r.ok && scope === yyAccountControllerScope() && yyAccountRead === pending) yyAccountSnapshot = { scope, result: r };
    return r;
  }).finally(() => {
    if (yyAccountRead === pending) yyAccountRead = null;
  });
  yyAccountRead = pending;
  return pending.promise;
}
function yySetAccountRows(r) {
  var _a, _b;
  yyAccountRowsScope = yyAccountControllerScope();
  yyAccountRows = (r.accounts || []).map((a) => ({ ...a }));
  yyAllAccountRows = yyAccountRows.slice();
  yyAccountMainConfigured = (_b = (_a = r.main) == null ? void 0 : _a.configured) != null ? _b : null;
  if (r.main && r.main.configured !== false) yyAllAccountRows.unshift({ ...r.main, id: "main", name: r.main.name || "\u4E3B\u8D26\u53F7", profile_role: r.main.profile_role || "owner" });
}
function yyAccountStatusLabel(account) {
  if (account.connection_known === false || account.connected == null) return "\u72B6\u6001\u5F85\u67E5\u8BE2";
  return account.connected ? "\u5DF2\u8FDE\u63A5" : account.engine_ready ? "\u5185\u6838\u5728\u7EBF \xB7 \u6E38\u620F\u672A\u8FDE\u63A5" : "\u5185\u6838\u672A\u542F\u52A8";
}
function yyDrawAccountRows() {
  var _a;
  if (!document.getElementById("yy-accounts-grid")) return;
  const rows = yyAllAccountRows;
  yyRenderProjects();
  const current = document.getElementById("yy-current-account");
  if (current) current.textContent = yyAllAccountRows.some((a) => a.id === yyActiveAccount) ? yyAccountName(yyActiveAccount) : "\u672A\u6DFB\u52A0\u4E3B\u673A\u5668\u4EBA";
  yyRenderAccountCards();
  const el = document.getElementById("yy-monitor-account");
  if (el) {
    el.innerHTML = rows.map((a) => '<option value="' + yyEscape(a.id) + '">' + yyEscape(a.name) + "</option>").join("");
    if (!rows.some((a) => a.id === yyMonitorID)) yyMonitorID = ((_a = rows[0]) == null ? void 0 : _a.id) || "";
    el.value = yyMonitorID;
  }
  const mainButton = document.querySelector(`[onclick="yyEnterAccount('main')"]`);
  if (mainButton) {
    mainButton.disabled = !rows.some((a) => a.id === "main");
    mainButton.hidden = mainButton.disabled;
  }
}
function yyRenderAccountSnapshot() {
  if ((yyAccountSnapshot == null ? void 0 : yyAccountSnapshot.scope) === yyAccountControllerScope()) {
    yySetAccountRows(yyAccountSnapshot.result);
    yyDrawAccountRows();
  } else {
    const host = document.getElementById("yy-accounts-grid");
    if (host) host.innerHTML = '<p class="yy-hint" role="status">\u6B63\u5728\u8BFB\u53D6\u672C\u8BBE\u5907\u8D26\u53F7\u2026</p>';
  }
}
async function yyRefreshAccounts(force) {
  const epoch = ++yyAccountRefreshEpoch, scope = yyAccountControllerScope(), host = document.getElementById("yy-accounts-grid");
  const live = () => scope === yyAccountControllerScope() && epoch === yyAccountRefreshEpoch && host && document.getElementById("yy-accounts-grid") === host;
  const r = await yyReadAccountList(!!force);
  if (!live()) return;
  if (!r.ok) {
    host.innerHTML = '<p class="yy-hint" role="status">' + yyEscape(r.error || "\u8D26\u53F7\u8BFB\u53D6\u5931\u8D25") + ' <button class="btn-s" onclick="yyCenterRefresh()">\u91CD\u8BD5</button></p>';
    return;
  }
  yySetAccountRows(r);
  yyDrawAccountRows();
  if (yyMonitorID) await yyRefreshMonitor();
}
async function yyWarmAccountList() {
  const r = await yyReadAccountList(true);
  if (r.ok && (yyAccountSnapshot == null ? void 0 : yyAccountSnapshot.scope) === yyAccountControllerScope() && !document.getElementById("yy-accounts-grid")) yySetAccountRows(r);
}
document.addEventListener("yy-identity-ready", function() {
  if (typeof yyUpdateAdminPoolEntry === "function") yyUpdateAdminPoolEntry();
  yyAccountSnapshot = null;
  yyAllAccountRows = [];
  yyAccountRows = [];
  yyAccountRowsScope = "";
  yyAccountMainConfigured = null;
  yyCenterCloudRows = [];
  yyWarmAccountList();
});
setTimeout(function() {
  yyWarmAccountList();
}, 0);
function yyEnterRow(index) {
  const row = yyAllAccountRows[index];
  if (row) yyEnterAccount(row.id);
}
function yyMonitorRow(index) {
  const row = yyAllAccountRows[index];
  if (!row) return;
  yyMonitorID = row.id;
  document.getElementById("yy-monitor-account").value = yyMonitorID;
  yyRefreshMonitor();
  document.querySelector(".yy-monitor").scrollIntoView({ behavior: "smooth" });
}
async function yyEnterAccount(id, tab) {
  if (yyEditorDirty && yyWorkspaceEditor && !await yySaveProjectFile()) return;
  if (id !== "main") {
    var result = await yyRootAPI("POST", "/api/youyuan/accounts", { action: "engine_start", id });
    if (!result.ok) {
      T(result.error, "e");
      return;
    }
    var ready = false;
    for (var n = 0; n < 25; n++) {
      var probe = await yyRootAPI("GET", yyAccountPath("/api/bot/status", id));
      if (probe.ok) {
        ready = true;
        break;
      }
      await new Promise((r) => setTimeout(r, 400));
    }
    if (!ready) {
      T("\u8D26\u53F7\u5185\u6838\u542F\u52A8\u4E2D\uFF0C\u8BF7\u7A0D\u540E\u518D\u6B21\u8FDB\u5165", "e");
      return;
    }
  }
  activeRenderers.forEach(function(kill) {
    try {
      kill();
    } catch (e) {
    }
  });
  activeRenderers = [];
  clearInterval(yyWorkspacePoll);
  yyWorkspaceActive = null;
  yyEditorDirty = false;
  allLogs = [];
  tabInited = {};
  subPageCache = {};
  _currentTask = null;
  yyActiveAccount = id;
  U(false, false);
  document.getElementById("yy-active-account-label").textContent = yyAccountName(id);
  if (subPageOpen) {
    subPageStack = [];
    closeSubPage();
    await new Promise((r) => setTimeout(r, 340));
  }
  tabTo(tab || "home");
  T("\u5DF2\u8FDB\u5165 " + yyAccountName(id), "o");
}
async function yyToggleAccount(index) {
  var a = yyAccountRows[index], r = await yyRootAPI("POST", "/api/youyuan/accounts", { id: a.id, action: a.enabled ? "disable" : "enable" });
  if (!r.ok) T(r.error, "e");
  else T(a.enabled ? "\u8D26\u53F7\u5DF2\u505C\u6B62" : "\u8FDE\u63A5\u8BF7\u6C42\u5DF2\u63D0\u4EA4", "o");
  await yyRefreshAccounts();
}
function yyEditAccount(index) {
  var a = typeof index === "number" ? yyAccountRows[index] : { id: "account_" + Date.now(), name: "", auth_url: "https://susu.bot.cd", server: "", keep_alive: true };
  window.yyEditingAccount = a;
  delete _savedFormState["\u8D26\u53F7\u914D\u7F6E"];
  delete subPageCache["\u8D26\u53F7\u914D\u7F6E"];
  openSubPage("\u8D26\u53F7\u914D\u7F6E", yyIntro("\u72EC\u7ACB\u8D26\u53F7", "\u4EE4\u724C\u4FDD\u5B58\u5728\u79C1\u6709\u76EE\u5F55\uFF1B\u53EF\u6309\u9700\u67E5\u770B\u548C\u590D\u5236\u81EA\u5DF1\u7684\u8D26\u53F7\u4EE4\u724C\u3002") + '<section class="card"><label>\u540D\u79F0</label><input id="yy-ac-name" value="' + yyEscape(a.name || "") + '"><label>Token</label><input id="yy-ac-token" type="password" placeholder="' + (a.has_token ? "\u5DF2\u4FDD\u5B58\uFF1B\u7559\u7A7A\u4FDD\u7559" : "\u586B\u5199\u8BE5\u8D26\u53F7\u81EA\u5DF1\u7684 Token") + '" autocomplete="new-password"><div class="yy-actions"><button class="btn-s" onclick="yyAccountTokenAction(yyEditingAccount.id,false)">\u663E\u793A / \u9690\u85CF Token</button><button class="btn-s" onclick="yyAccountTokenAction(yyEditingAccount.id,true)">\u590D\u5236 Token</button></div><label>\u9A8C\u8BC1\u670D\u52A1\u5668</label><select id="yy-ac-auth"><option value="https://susu.bot.cd">Susu \u9A8C\u8BC1\u670D\u52A1</option><option value="https://prism.adblanlu.qzz.io">\u9884\u7F6E\u9A8C\u8BC1\u670D\u52A1 2</option><option value="https://auth.youyuanqi.dpdns.org">\u6E38\u56ED\u72EC\u7ACB\u9A8C\u8BC1\u670D\u52A1</option></select><label>\u670D\u52A1\u5668\u53F7 / \u5206\u4EAB\u94FE\u63A5</label><input id="yy-ac-server" value="' + yyEscape(a.server || "") + '"><label>\u670D\u52A1\u5668\u5BC6\u7801</label><input id="yy-ac-pass" type="password" placeholder="\u7559\u7A7A\u4FDD\u7559\u5DF2\u6709\u5BC6\u7801">' + yySwitch("yy-ac-keep", "\u4FDD\u6301\u8FD0\u884C\uFF0C\u6389\u7EBF\u81EA\u52A8\u91CD\u8FDE", a.keep_alive) + '<p class="yy-hint">\u5404\u8D26\u53F7\u6709\u72EC\u7ACB\u6570\u636E\u76EE\u5F55\u3002Android \u524D\u53F0\u670D\u52A1\u4E0E\u5524\u9192\u9501\u5E2E\u52A9\u4FDD\u6301\u8FD0\u884C\uFF1B\u7CFB\u7EDF\u5F3A\u5236\u505C\u6B62\u3001\u65AD\u7F51\u6216\u670D\u52A1\u5668\u62D2\u7EDD\u4ECD\u4F1A\u79BB\u7EBF\u3002\u91CD\u65B0\u9009\u62E9\u670D\u52A1\u5668\u540E\u4FDD\u5B58\u5E76\u542F\u7528\u8FDE\u63A5\u3002</p><div class="yy-actions"><button class="btn" onclick="yySaveAccount()">\u4FDD\u5B58\u8D26\u53F7</button><button class="btn-s" onclick="yyDeleteAccount()">\u505C\u7528\u5E76\u79FB\u9664</button></div><pre id="yy-ac-result" class="yy-readout"></pre></section>');
  document.getElementById("yy-ac-auth").value = a.auth_url;
}
async function yySaveAccount() {
  var account = { id: yyEditingAccount.id, name: document.getElementById("yy-ac-name").value, token: document.getElementById("yy-ac-token").value, auth_url: document.getElementById("yy-ac-auth").value, server: document.getElementById("yy-ac-server").value, password: document.getElementById("yy-ac-pass").value, keep_alive: document.getElementById("yy-ac-keep").checked };
  var r = await yyRootAPI("POST", "/api/youyuan/accounts", { action: "save", account });
  if (!r.ok) {
    document.getElementById("yy-ac-result").textContent = r.error;
    T(r.error, "e");
    return;
  }
  delete _savedFormState["\u8D26\u53F7\u914D\u7F6E"];
  delete subPageCache["\u8D26\u53F7\u914D\u7F6E"];
  document.getElementById("yy-ac-token").value = "";
  document.getElementById("yy-ac-pass").value = "";
  var id = r.id || account.id;
  yyAccountFocusID = id;
  yyNavigate("accounts");
  T("\u8D26\u53F7\u5DF2\u6DFB\u52A0\u5E76\u4FDD\u5B58\uFF0C\u53EF\u5728\u5217\u8868\u8FDB\u5165\u6216\u8FDE\u63A5", "o");
}
async function yyDeleteAccount() {
  var r = await yyRootAPI("POST", "/api/youyuan/accounts", { action: "delete", id: yyEditingAccount.id });
  if (r.ok) {
    if (yyActiveAccount === yyEditingAccount.id) yyActiveAccount = "main";
    yyNavigate("accounts");
  } else document.getElementById("yy-ac-result").textContent = r.error;
}
var yyMonitorReadEpoch = 0;
async function yyRefreshMonitor() {
  const id = yyMonitorID, epoch = ++yyMonitorReadEpoch, scope = yyAccountControllerScope(), host = document.getElementById("yy-online-chart");
  const [r, status] = await Promise.all(["/api/youyuan/observations", "/api/bot/status"].map((path) => yyRootAPI("GET", yyAccountPath(path, id))));
  if (id !== yyMonitorID || epoch !== yyMonitorReadEpoch || scope !== yyAccountControllerScope() || !host || document.getElementById("yy-online-chart") !== host) return;
  const row = yyAllAccountRows.find((a) => a.id === id);
  if (row && status.ok !== false && typeof status.connected === "boolean") {
    Object.assign(row, { connected: status.connected, connection_known: true, engine_ready: true, server: status.server || row.server });
    if (r.ok) {
      row.online = r.online;
      row.online_source = r.source;
    }
    yyRenderProjects();
    yyRenderAccountCards();
  }
  if (!r.ok) {
    yyObservations = null;
    host.innerHTML = '<div class="yy-empty">' + yyEscape(r.error || "\u8D26\u53F7\u8BB0\u5F55\u6682\u65F6\u65E0\u6CD5\u8BFB\u53D6\uFF0C\u8FDE\u63A5\u72B6\u6001\u672A\u786E\u8BA4\u3002") + "</div>";
    document.getElementById("yy-distribution-chart").innerHTML = "";
    document.getElementById("yy-chat-table").innerHTML = "";
    document.getElementById("yy-monitor-metrics").textContent = row ? yyAccountStatusLabel(row) : "\u72B6\u6001\u5F85\u67E5\u8BE2";
    return;
  }
  yyObservations = r;
  document.getElementById("yy-online-source").textContent = r.source;
  document.getElementById("yy-monitor-metrics").innerHTML = yyBadge(row ? yyAccountStatusLabel(row) : "\u72B6\u6001\u5F85\u67E5\u8BE2", (row == null ? void 0 : row.connected) === true) + yyBadge("\u5F53\u524D\u89C2\u6D4B " + (r.online == null ? "\u2014" : r.online)) + yyBadge((r.chat || []).length + " \u6761\u804A\u5929\u8BB0\u5F55");
  yyDrawOnline();
  yyDrawChat();
}
var yyChatPage = 1, yyChatBucket = null, yyChatMonitorID = "";
function yyChatFilterRows(rows, filters) {
  const text = String(filters.text || "").trim().toLowerCase(), player = String(filters.player || ""), start = filters.start ? Date.parse(filters.start) : NaN, end = filters.end ? Date.parse(filters.end) + 59999 : NaN;
  return rows.filter(function(row) {
    const time = Date.parse(row.time);
    return (!player || row.player === player) && (!text || [row.player, row.message, row.server].join(" ").toLowerCase().includes(text)) && (!Number.isFinite(start) || time >= start) && (!Number.isFinite(end) || time <= end);
  }).slice().sort((a, b) => (Date.parse(b.time) || 0) - (Date.parse(a.time) || 0));
}
function yyChatGroupRows(rows, interval) {
  const step = [9e5, 36e5, 864e5].includes(Number(interval)) ? Number(interval) : 36e5, map = /* @__PURE__ */ new Map();
  for (const row of rows) {
    const time = Date.parse(row.time);
    if (!Number.isFinite(time)) continue;
    const date = new Date(time), start = step === 864e5 ? new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime() : Math.floor(time / step) * step;
    map.set(start, (map.get(start) || 0) + 1);
  }
  return Array.from(map, ([start, count]) => ({ start, end: step === 864e5 ? new Date(new Date(start).getFullYear(), new Date(start).getMonth(), new Date(start).getDate() + 1).getTime() : start + step, count })).sort((a, b) => a.start - b.start);
}
function yyChatFilterChanged() {
  yyChatPage = 1;
  yyChatBucket = null;
  yyDrawChat();
}
function yyChatChooseBucket(start, end) {
  yyChatPage = 1;
  yyChatBucket = start == null ? null : { start: Number(start), end: Number(end) };
  yyDrawChat();
}
function yyChatResetFilters() {
  for (const id of ["yy-chat-filter", "yy-chat-player", "yy-chat-start", "yy-chat-end"]) {
    const el = document.getElementById(id);
    if (el) el.value = "";
  }
  yyChatFilterChanged();
}
function yyChatBucketLabel(start, end) {
  return new Date(start).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }) + " \u2014 " + new Date(end).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
}
function yyDrawChat() {
  const host = document.getElementById("yy-chat-table");
  if (!host || !yyObservations) return;
  if (yyChatMonitorID !== yyMonitorID) {
    yyChatMonitorID = yyMonitorID;
    yyChatPage = 1;
    yyChatBucket = null;
    const player = document.getElementById("yy-chat-player");
    if (player) player.value = "";
  }
  const all = yyObservations.chat || [], players = [...new Set(all.map((r) => r.player).filter(Boolean))].sort((a, b) => a.localeCompare(b, "zh"));
  const picker = document.getElementById("yy-chat-player");
  if (picker) {
    const chosen = picker.value;
    picker.innerHTML = '<option value="">\u6240\u6709\u73A9\u5BB6</option>' + players.map((p) => '<option value="' + yyEscape(p) + '">' + yyEscape(p) + "</option>").join("");
    picker.value = players.includes(chosen) ? chosen : "";
  }
  const value = (id) => {
    var _a;
    return ((_a = document.getElementById(id)) == null ? void 0 : _a.value) || "";
  }, base = yyChatFilterRows(all, { text: value("yy-chat-filter"), player: value("yy-chat-player"), start: value("yy-chat-start"), end: value("yy-chat-end") }), buckets = yyChatGroupRows(base, value("yy-chat-interval")), shown = buckets.slice(-48), peak = Math.max(1, ...shown.map((b) => b.count)), bars = document.getElementById("yy-chat-bars");
  if (bars) bars.innerHTML = shown.map((b) => '<button type="button" class="yy-chat-bar' + ((yyChatBucket == null ? void 0 : yyChatBucket.start) === b.start ? " selected" : "") + '" onclick="yyChatChooseBucket(' + b.start + "," + b.end + ')" aria-label="' + yyEscape(yyChatBucketLabel(b.start, b.end) + "\uFF0C" + b.count + " \u6761\u6D88\u606F") + '"><span>' + yyEscape(yyChatBucketLabel(b.start, b.end)) + '</span><span class="yy-chat-bar-track"><i style="width:' + Math.max(2, b.count * 100 / peak).toFixed(2) + '%"></i></span><strong>' + b.count + "</strong></button>").join("") || '<p class="yy-empty">\u6240\u9009\u6761\u4EF6\u4E0B\u6682\u65E0\u65F6\u6BB5\u6570\u636E</p>';
  const rows = yyChatBucket ? base.filter((r) => {
    const t = Date.parse(r.time);
    return t >= yyChatBucket.start && t < yyChatBucket.end;
  }) : base, size = 50, pages = Math.max(1, Math.ceil(rows.length / size));
  yyChatPage = Math.min(pages, Math.max(1, yyChatPage));
  const summary = document.getElementById("yy-chat-summary");
  if (summary) summary.textContent = "\u5DF2\u4FDD\u5B58 " + all.length + " \u6761 \xB7 \u7B5B\u9009\u5339\u914D " + base.length + " \u6761" + (yyChatBucket ? " \xB7 \u5F53\u524D\u65F6\u6BB5 " + rows.length + " \u6761" : "") + (buckets.length > 48 ? " \xB7 \u56FE\u8868\u4EC5\u5C55\u793A\u6700\u8FD1 48 \u4E2A\u6709\u6D88\u606F\u7684\u65F6\u6BB5\uFF0C\u8BF7\u8C03\u6574\u65F6\u95F4\u7B5B\u9009\u67E5\u770B\u66F4\u65E9\u8BB0\u5F55" : "");
  const reset = document.getElementById("yy-chat-all-buckets");
  if (reset) reset.hidden = !yyChatBucket;
  host.innerHTML = rows.slice((yyChatPage - 1) * size, yyChatPage * size).map((r) => '<article class="yy-chat-message"><header><strong>' + yyEscape(r.player || "\u672A\u77E5\u73A9\u5BB6") + "</strong><time>" + yyEscape(new Date(r.time).toLocaleString("zh-CN")) + "</time></header>" + (r.server ? '<small class="yy-chat-server">' + yyEscape(r.server) + "</small>" : "") + "<p>" + yyEscape(r.message || "") + "</p></article>").join("") || '<p class="yy-empty">\u6CA1\u6709\u5339\u914D\u7684\u804A\u5929\u8BB0\u5F55</p>';
  const pager = document.getElementById("yy-chat-pages");
  if (pager) pager.innerHTML = '<button class="btn-s" ' + (yyChatPage === 1 ? "disabled" : "") + ' onclick="yyChatPage--;yyDrawChat()">\u4E0A\u4E00\u9875</button><span>' + yyChatPage + " / " + pages + " \u9875 \xB7 " + rows.length + ' \u6761</span><button class="btn-s" ' + (yyChatPage === pages ? "disabled" : "") + ' onclick="yyChatPage++;yyDrawChat()">\u4E0B\u4E00\u9875</button>';
}
function yyDrawOnline() {
  var rows = yyObservations.samples || [], valid = rows.filter((r) => r.online != null);
  if (!valid.length) {
    document.getElementById("yy-online-chart").innerHTML = '<div class="yy-empty">\u5C1A\u672A\u53D6\u5F97\u771F\u5B9E\u5728\u7EBF\u6570\u636E\uFF1B\u8FDE\u63A5\u540E\u6BCF 15 \u79D2\u91C7\u6837\u3002</div>';
    document.getElementById("yy-distribution-chart").innerHTML = '<div class="yy-empty">\u6682\u65E0\u4EBA\u6570\u5206\u5E03\u6570\u636E</div>';
    return;
  }
  var max = Math.max(1, ...valid.map((r) => r.online)), begin = Date.parse(rows[0].time), end = Date.parse(rows[rows.length - 1].time), span = Math.max(1, end - begin), path = "", pen = false;
  rows.forEach(function(r) {
    if (r.online == null) {
      pen = false;
      return;
    }
    path += (pen ? "L" : "M") + (45 + 610 * (Date.parse(r.time) - begin) / span).toFixed(1) + "," + (175 - 150 * r.online / max).toFixed(1) + " ";
    pen = true;
  });
  var axes = '<path d="M45 20V175H655" class="yy-chart-axis"/><text x="8" y="28">' + max + '</text><text x="22" y="175">0</text>';
  document.getElementById("yy-online-chart").innerHTML = '<svg viewBox="0 0 700 215" role="img" aria-label="\u771F\u5B9E\u89C2\u6D4B\u5728\u7EBF\u4EBA\u6570\u65F6\u95F4\u7EBF">' + axes + '<path d="' + path + '" class="yy-chart-line"/><text x="45" y="203">' + yyEscape(new Date(begin).toLocaleTimeString("zh-CN")) + '</text><text x="595" y="203">' + yyEscape(new Date(end).toLocaleTimeString("zh-CN")) + "</text></svg>";
  var hours = Array.from({ length: 24 }, () => ({ sum: 0, n: 0 }));
  valid.forEach((r) => {
    var h = hours[new Date(r.time).getHours()];
    h.sum += r.online;
    h.n++;
  });
  var avg = hours.map((h) => h.n ? h.sum / h.n : 0), peak = Math.max(1, ...avg);
  document.getElementById("yy-distribution-chart").innerHTML = '<svg viewBox="0 0 700 210" role="img" aria-label="\u6BCF\u5C0F\u65F6\u5728\u7EBF\u4EBA\u6570\u89C2\u6D4B\u5747\u503C">' + axes.replace(">" + max + "</text>", ">" + peak.toFixed(1) + "</text>") + hours.map((h, i) => '<rect x="' + (48 + i * 25) + '" y="' + (175 - avg[i] * 150 / peak) + '" width="18" height="' + avg[i] * 150 / peak + '" class="yy-chart-bar"><title>' + i + " \u65F6\uFF1A" + (h.n ? avg[i].toFixed(1) + " \u4EBA / " + h.n + " \u4E2A\u6837\u672C" : "\u65E0\u6837\u672C") + "</title></rect>" + (i % 3 === 0 ? '<text x="' + (48 + i * 25) + '" y="198">' + i + "\u65F6</text>" : "")).join("") + "</svg>";
}
TAB_ORDER.push("accounts");
var yyAccountTabBase = tabTo;
tabTo = function(t) {
  yyAccountTabBase(t);
  if (t === "accounts") {
    document.getElementById("yy-crumb").innerHTML = '\u5DE5\u4F5C\u7A7A\u95F4 <span style="margin:0 10px;opacity:.4">/</span> <b>\u8D26\u53F7\u7BA1\u7406</b>';
    Raccounts();
  }
};
var side = document.querySelector(".yy-side-nav");
var button = document.createElement("button");
button.dataset.yyTab = "accounts";
button.innerHTML = yyIcon("user") + "\u8D26\u53F7\u5DE5\u4F5C\u7A7A\u95F4";
button.onclick = function() {
  yyNavigate("accounts");
};
side.appendChild(button);
var label = document.createElement("button");
label.className = "yy-account-selector";
label.innerHTML = yyIcon("user") + '<span id="yy-active-account-label">\u4E3B\u8D26\u53F7</span>';
label.onclick = function() {
  yyNavigate("accounts");
};
document.querySelector(".yy-sidebar-brand").after(label);
async function yyToggleMainAccount() {
  var status = await yyRootAPI("GET", "/api/bot/status"), r;
  if (status.connected) r = await yyRootAPI("POST", "/api/bot/disconnect", {});
  else {
    var data = await yyRootAPI("GET", "/api/config"), c = data.config || {};
    if (!c.token || !c.server_code) {
      T("\u5148\u5728\u4E3B\u8D26\u53F7\u8BBE\u7F6E\u4E2D\u586B\u5199 Token \u4E0E\u670D\u52A1\u5668\u53F7", "e");
      return;
    }
    r = await yyRootAPI("POST", "/api/bot/connect", { token: c.token, server: c.server_code, password: c.server_pass || "", auth: c.auth_url || "https://susu.bot.cd" });
  }
  T(r.ok ? "\u5DF2\u63D0\u4EA4\u8D26\u53F7\u8FDE\u63A5\u64CD\u4F5C" : r.error, r.ok ? "o" : "e");
  yyRefreshAccounts();
}
var yyAllAccountRows = [];
function yyAccountActionHTML(fn, args, label2, kind) {
  return '<button class="' + (kind || "btn-s") + '" onclick="' + yyEscape(fn + "(" + args.map((a) => JSON.stringify(a)).join(",") + ")") + '">' + yyEscape(label2) + "</button>";
}
function yyAccountUID(row) {
  const id = row == null ? void 0 : row.game_id;
  return typeof id === "string" && /^[1-9]\d*$/.test(id) ? id : typeof id === "number" && Number.isSafeInteger(id) && id > 0 ? String(id) : "";
}
function yyLinkedCredential(account, credentials) {
  const uid = yyAccountUID(account), rows = credentials || [];
  const exact = rows.find((row) => row.id != null && (account.credential_id === row.id || account.id === "yy_" + row.id || account.id === "pool_" + row.id) && (!uid || !yyAccountUID(row) || uid === yyAccountUID(row)));
  return exact || uid && rows.find((row) => yyAccountUID(row) === uid) || null;
}
function yyMergedAccountRows() {
  const groups = /* @__PURE__ */ new Map(), credentials = typeof yyCenterCloudRows !== "undefined" ? yyCenterCloudRows : [];
  function take(key) {
    if (!groups.has(key)) groups.set(key, { key, local_accounts: [], cloud_accounts: [] });
    return groups.get(key);
  }
  for (const row of credentials) {
    const uid = yyAccountUID(row);
    take(uid ? "game:" + uid : "credential:" + row.id).cloud_accounts.push(row);
  }
  for (const local of yyAllAccountRows) {
    if (local.id === "main" && local.configured === false) continue;
    const cloud = yyLinkedCredential(local, credentials), uid = yyAccountUID(local) || yyAccountUID(cloud), key = uid ? "game:" + uid : cloud ? "credential:" + cloud.id : "local:" + local.id;
    take(key).local_accounts.push(local);
  }
  return [...groups.values()].map((group) => {
    var _a;
    const primary = group.local_accounts.find((a) => a.id === "main") || group.local_accounts[0], official = group.cloud_accounts.find((a) => a.avatar_source === "netease" && a.avatar) || group.cloud_accounts[0], localOfficial = group.local_accounts.find((a) => a.avatar_source === "netease");
    return { ...group, id: (primary == null ? void 0 : primary.id) || "credential:" + official.id, name: (official == null ? void 0 : official.nickname) || (primary == null ? void 0 : primary.name) || (official == null ? void 0 : official.name) || "\u6E38\u620F\u8D26\u53F7", game_id: yyAccountUID(official) || yyAccountUID(primary), server: (primary == null ? void 0 : primary.server) || "", connected: primary == null ? void 0 : primary.connected, connection_known: primary == null ? void 0 : primary.connection_known, engine_ready: primary == null ? void 0 : primary.engine_ready, avatar: ((_a = (official == null ? void 0 : official.avatar_source) === "netease" ? official : localOfficial) == null ? void 0 : _a.avatar) || "", avatar_source: (official == null ? void 0 : official.avatar_source) === "netease" || localOfficial ? "netease" : "" };
  });
}
function yyAccountPortrait(account) {
  const rows = typeof yyCenterCloudRows !== "undefined" ? yyCenterCloudRows : [], linked = yyLinkedCredential(account, rows);
  for (const candidate of [account, linked]) {
    if ((candidate == null ? void 0 : candidate.avatar_source) !== "netease") continue;
    try {
      const url = new URL(candidate.avatar);
      if (url.protocol === "https:" && !url.username && !url.password) return url.href;
    } catch {
    }
  }
  return "";
}
function yyAccountAvatarHTML(account) {
  const url = yyAccountPortrait(account);
  return '<span class="yy-compact-avatar yy-avatar-frame" role="img" aria-label="' + (url ? "\u7F51\u6613\u5B98\u65B9\u5934\u50CF" : "\u6682\u65E0\u5B98\u65B9\u5934\u50CF") + '"><svg viewBox="0 0 40 40" aria-hidden="true"><circle cx="20" cy="14" r="7"></circle><path d="M7 36c0-9 5-14 13-14s13 5 13 14"></path></svg>' + (url ? '<img src="' + yyEscape(url) + '" alt="" width="44" height="44" onerror="yyAccountAvatarError(this)">' : "") + "</span>";
}
function yyAccountAvatarError(image) {
  var _a;
  image.onerror = null;
  (_a = image.parentElement) == null ? void 0 : _a.setAttribute("aria-label", "\u6682\u65E0\u5B98\u65B9\u5934\u50CF");
  image.remove();
}
function yyAccountLocalActionsHTML(a) {
  const index = yyAllAccountRows.findIndex((x) => x.id === a.id), accountIndex = yyAccountRows.findIndex((x) => x.id === a.id), actions = [yyAccountActionHTML("yyEnterRow", [index], "\u8FDB\u5165\u6B64\u8D26\u53F7\u5DE5\u4F5C\u533A", "btn")];
  if (a.id !== "main") actions.push(yyAccountActionHTML("yyEditAccount", [accountIndex], "\u4EE4\u724C / \u670D\u52A1\u5668"), yyAccountActionHTML("yyToggleAccount", [accountIndex], a.enabled ? "\u5173\u95ED\u8FDE\u63A5" : "\u542F\u7528\u8FDE\u63A5"));
  else actions.push(yyAccountActionHTML("yyEnterAccount", ["main", "settings"], "\u914D\u7F6E\u4E3B\u8FDE\u63A5"), yyAccountActionHTML("yyToggleMainAccount", [], a.connection_known === false || a.connected == null ? "\u7BA1\u7406\u8FDE\u63A5" : a.connected ? "\u65AD\u5F00\u8FDE\u63A5" : "\u8FDE\u63A5\u670D\u52A1\u5668"));
  actions.push(yyAccountActionHTML("yyEditAccountProfileRow", [index], "\u8BBE\u5907\u5907\u6CE8 / \u5206\u7C7B"), yyAccountActionHTML("yyAccountTokenAction", [a.id, true], "\u590D\u5236\u4EE4\u724C"), yyAccountActionHTML("yyObservationPage", [a.id], "\u89C2\u5BDF\u65B9\u5F0F"), yyAccountActionHTML("yyMonitorRow", [index], "\u67E5\u770B\u8BB0\u5F55"));
  return actions.join("");
}
function yyAccountCredentialActionsHTML(row, hasLocal) {
  const index = yyCenterCloudRows.indexOf(row), actions = [];
  if (!hasLocal) actions.push(yyAccountActionHTML("yyUnifiedCredentialAction", [index, "host"], "\u6258\u7BA1\u5E76\u914D\u7F6E\u8FDE\u63A5", "btn"));
  for (const [action, label2] of [["token", "\u67E5\u770B / \u590D\u5236\u4EE4\u724C"], ["nickname", "\u4FEE\u6539\u6E38\u620F\u6635\u79F0"], ["skin", "\u76AE\u80A4\u9884\u89C8 / \u7EC4\u4EF6\u7801"], ["refresh", "\u5237\u65B0\u5B98\u65B9\u51ED\u636E"], ["relogin", "\u91CD\u65B0\u767B\u5F55\u6B64\u8D26\u53F7"], ["configure", row.keep_alive ? "\u5173\u95ED\u51ED\u636E\u5B9A\u65F6\u5237\u65B0" : "\u5F00\u542F\u51ED\u636E\u5B9A\u65F6\u5237\u65B0"], ["delete", "\u64A4\u9500\u4E91\u7AEF\u51ED\u636E"]]) actions.push(yyAccountActionHTML("yyUnifiedCredentialAction", [index, action], label2));
  return '<div class="yy-actions">' + actions.join("") + '</div><p class="yy-hint">' + (hasLocal ? "\u51ED\u636E\u5DF2\u5173\u8054\u6B64\u8BBE\u5907\u8D26\u53F7\u3002" : "\u4EC5\u4E91\u7AEF\u4FDD\u5B58\uFF0C\u5C1A\u672A\u6258\u7BA1\uFF1B\u70B9\u51FB\u6258\u7BA1\u540E\u586B\u5199\u670D\u52A1\u5668\u5E76\u660E\u786E\u542F\u7528\u8FDE\u63A5\uFF0C\u4E0D\u81EA\u52A8\u8FDB\u670D\u3002") + '</p><button class="btn-s" disabled>\u9009\u62E9\u5B98\u65B9\u5934\u50CF \xB7 \u63A5\u53E3\u5F85\u63A5\u5165</button><p class="yy-hint">\u5B98\u65B9\u5934\u50CF\u5217\u8868\u4E0E\u8BBE\u7F6E\u63A5\u53E3\u6838\u5B9E\u540E\u5F00\u653E\uFF1B\u672A\u77E5\u5934\u50CF\u4E0D\u81EA\u52A8\u6307\u5B9A\u89D2\u8272\u3002</p>';
}
function yyRenderAccountCards() {
  var _a, _b;
  const host = document.getElementById("yy-accounts-grid");
  if (!host) return;
  const open = new Set(Array.from(((_a = host.querySelectorAll) == null ? void 0 : _a.call(host, "details[open]")) || [], (row) => row.getAttribute("data-account-key") || row.getAttribute("data-account-id"))), needle = (((_b = document.getElementById("yy-account-search")) == null ? void 0 : _b.value) || "").toLowerCase();
  const filtered = yyMergedAccountRows().filter((a) => (!yyProjectFilter || a.local_accounts.some((local) => local.server === yyProjectFilter)) && [a.name, a.game_id, ...a.local_accounts.map((local) => local.name + " " + local.server + " " + local.id), ...a.cloud_accounts.map((row) => row.name)].join(" ").toLowerCase().includes(needle));
  if (yyAccountFocusID) {
    const index = filtered.findIndex((a) => a.id === yyAccountFocusID || a.local_accounts.some((local) => local.id === yyAccountFocusID));
    if (index >= 0) {
      yyAccountPage = Math.floor(index / yyAccountPageSize) + 1;
      open.add(filtered[index].key);
    }
    yyAccountFocusID = "";
  }
  const pages = Math.max(1, Math.ceil(filtered.length / yyAccountPageSize));
  yyAccountPage = Math.min(pages, Math.max(1, yyAccountPage));
  host.innerHTML = filtered.slice((yyAccountPage - 1) * yyAccountPageSize, yyAccountPage * yyAccountPageSize).map(function(a) {
    const localDetails = a.local_accounts.map((local) => '<section class="yy-linked-local"><h4>' + yyEscape(local.id === "main" ? "\u4E3B\u8FDE\u63A5" : local.name || local.id) + '</h4><p class="yy-hint">' + yyEscape(yyAccountStatusLabel(local) + " \xB7 " + (local.server || "\u672A\u9009\u62E9\u670D\u52A1\u5668")) + '</p><p class="yy-hint">\u63D2\u4EF6\uFF1A' + yyEscape((local.plugins || []).map((p) => p.name || p.id).join("\u3001") || (local.telemetry_loaded === false ? "\u8FDB\u5165\u5DE5\u4F5C\u533A\u67E5\u770B\u5DF2\u5B89\u88C5\u63D2\u4EF6" : "\u8FDB\u5165\u5DE5\u4F5C\u533A\u540E\u6DFB\u52A0\u63D2\u4EF6")) + '</p><div class="yy-actions">' + yyAccountLocalActionsHTML(local) + "</div></section>").join("");
    const cloudDetails = a.cloud_accounts.map((row) => '<section class="yy-linked-credential"><h4>\u5B98\u65B9\u8D26\u53F7\u8D44\u6599\u4E0E\u51ED\u636E</h4>' + yyAccountCredentialActionsHTML(row, a.local_accounts.length > 0) + "</section>").join("");
    return '<details class="card yy-account-card" data-account-id="' + yyEscape(a.id) + '" data-account-key="' + yyEscape(a.key) + '"' + (open.has(a.key) || open.has(a.id) ? " open" : "") + '><summary class="yy-account-summary">' + yyAccountAvatarHTML(a) + '<span class="yy-compact-identity"><strong>' + yyEscape(a.name) + "</strong><small>" + yyEscape((a.game_id ? "UID " + a.game_id + " \xB7 " : "") + (a.local_accounts.length ? a.server || "\u5C1A\u672A\u914D\u7F6E\u670D\u52A1\u5668" : "\u4EC5\u4E91\u7AEF\u4FDD\u5B58 \xB7 \u672A\u6258\u7BA1")) + '</small></span><span class="yy-compact-status">' + yyEscape(a.local_accounts.length ? yyAccountStatusLabel(a) : "\u672A\u6258\u7BA1") + '</span><span class="yy-row-chevron" aria-hidden="true">\u203A</span></summary><div class="yy-account-detail">' + localDetails + cloudDetails + "</div></details>";
  }).join("") || '<div class="yy-empty">\u5C1A\u65E0\u5339\u914D\u7684\u6E38\u620F\u8D26\u53F7\uFF1B\u53EF\u6DFB\u52A0\u672C\u5730\u4EE4\u724C\u6216\u6CE8\u518C\u3001\u63A5\u5165\u5B98\u65B9\u8D26\u53F7\u3002</div>';
  document.getElementById("yy-account-pages").innerHTML = '<button class="btn-s" ' + (yyAccountPage === 1 ? "disabled" : "") + ' onclick="yyAccountPage--;yyRenderAccountCards()">\u4E0A\u4E00\u9875</button><span>' + yyAccountPage + " / " + pages + " \u9875 \xB7 " + filtered.length + ' \u4E2A\u6E38\u620F\u8D26\u53F7</span><button class="btn-s" ' + (yyAccountPage === pages ? "disabled" : "") + ' onclick="yyAccountPage++;yyRenderAccountCards()">\u4E0B\u4E00\u9875</button>';
}
var yyCredentialOperations = /* @__PURE__ */ new Set();
async function yyUnifiedCredentialAction(index, action) {
  const row = yyCenterCloudRows[index], scope = yyAccountControllerScope();
  if (!row) return;
  const operation = scope + "|" + row.id + "|" + action;
  if (yyCredentialOperations.has(operation)) return;
  yyCredentialOperations.add(operation);
  try {
    if (action === "skin") {
      await yyGameAccountSkin(row);
      return;
    }
    if (action === "relogin") {
      await yyRelogGameAccount(row.id);
      return;
    }
    let body = { action: action === "host" ? "token" : action, id: row.id };
    if (action === "nickname") {
      const nickname = prompt("\u4FEE\u6539\u6B64\u6E38\u620F\u8D26\u53F7\u7684\u5B98\u65B9\u6635\u79F0", row.nickname || "");
      if (nickname === null) return;
      body.nickname = nickname;
    }
    if (action === "configure") body.keep_alive = !row.keep_alive;
    if (action === "delete" && !confirm("\u64A4\u9500\u6B64\u4E91\u7AEF\u6E38\u620F\u51ED\u636E\uFF1F\u4F7F\u7528\u5B83\u7684\u8FDE\u63A5\u53EF\u80FD\u53D7\u5F71\u54CD\uFF1B\u672C\u5730\u8D26\u53F7\u914D\u7F6E\u4E0D\u4F1A\u81EA\u52A8\u5220\u9664\u3002")) return;
    const r = await yyGameAuthAPI("POST", body);
    if (scope !== yyAccountControllerScope()) return;
    if (!r.ok) {
      T(r.error || "\u8D26\u53F7\u64CD\u4F5C\u5931\u8D25", "e");
      return;
    }
    if (action === "token") {
      if (!r.login_token) {
        T("\u672A\u8FD4\u56DE\u6B64\u8D26\u53F7\u4EE4\u724C", "e");
        return;
      }
      yyShowTokenDialog(r.login_token);
      return;
    }
    if (action === "host") {
      if (!r.login_token) {
        T("\u672A\u8FD4\u56DE\u53EF\u6258\u7BA1\u7684\u6E38\u620F\u4EE4\u724C", "e");
        return;
      }
      await yyAutoHostCredential({ ...r, account: row });
      if (scope !== yyAccountControllerScope()) return;
      await yyRefreshAccounts(true);
      if (scope !== yyAccountControllerScope()) return;
      await yyCenterCloudAccounts();
      if (scope !== yyAccountControllerScope()) return;
      const localIndex = yyAccountRows.findIndex((a) => a.id === "yy_" + row.id);
      if (localIndex >= 0) yyEditAccount(localIndex);
      T("\u5DF2\u6258\u7BA1\u5230\u6B64\u8BBE\u5907\uFF0C\u8BF7\u586B\u5199\u670D\u52A1\u5668\u5E76\u4FDD\u5B58\uFF1B\u5C1A\u672A\u542F\u7528\u8FDE\u63A5", "o");
      return;
    }
    await yyCenterCloudAccounts();
  } catch (error) {
    if (scope === yyAccountControllerScope()) T(error.message || "\u8D26\u53F7\u64CD\u4F5C\u5931\u8D25", "e");
  } finally {
    yyCredentialOperations.delete(operation);
  }
}
var yyEditingProfile = null, yyEditingProfileAvatar = null;
function yyEditAccountProfileRow(index) {
  const row = yyAllAccountRows[index];
  if (!row) return;
  yyEditingProfile = row;
  yyEditingProfileAvatar = null;
  delete subPageCache["\u8D26\u53F7\u8D44\u6599"];
  openSubPage("\u8D26\u53F7\u8D44\u6599", yyIntro("\u8BBE\u5907\u5907\u6CE8\u4E0E\u5206\u7C7B", "\u4EC5\u4FEE\u6539\u5F53\u524D\u8BBE\u5907\u7684\u663E\u793A\u8D44\u6599\uFF1B\u4E3B\u8FDE\u63A5\u4E0E\u5B98\u65B9\u8D26\u53F7\u8D44\u6599\u5728\u5404\u81EA\u5165\u53E3\u914D\u7F6E\u3002") + '<section class="card"><label>\u8BBE\u5907\u5907\u6CE8</label><input id="yy-profile-name" value="' + yyEscape(row.name || "") + '"><label>\u5DE5\u5177\u7BB1\u5206\u7C7B</label><select id="yy-profile-role"><option value="bot">\u673A\u5668\u4EBA\u8D26\u53F7</option><option value="owner">\u4E3B\u63A7\u5206\u7C7B \xB7 \u4E0D\u5207\u6362\u4E3B\u8FDE\u63A5</option></select><p class="yy-hint">\u6B64\u5206\u7C7B\u4E0D\u4EE3\u8868\u7F51\u6613\u670D\u4E3B\uFF0C\u4E5F\u4E0D\u4F1A\u8FC1\u79FB\u6B63\u5728\u8FD0\u884C\u7684\u4EFB\u52A1\u3002</p><button class="btn" onclick="yySaveAccountProfileUI.call(this)">\u4FDD\u5B58\u8BBE\u5907\u8D44\u6599</button><p id="yy-profile-result" class="yy-hint" role="status"></p></section>');
  document.getElementById("yy-profile-role").value = row.profile_role || "bot";
}
function yyDrawProfilePreview(value) {
  const host = document.getElementById("yy-profile-preview");
  if (!host) return;
  host.textContent = "";
  if (value) {
    const img = document.createElement("img");
    img.src = value;
    img.alt = "\u8D26\u53F7\u5934\u50CF\u9884\u89C8";
    host.append(img);
  } else host.innerHTML = yyIcon("user");
}
async function yyPickAccountAvatar() {
  const file = this.files[0];
  if (!file) return;
  const out = document.getElementById("yy-profile-result");
  if (file.size > 5 * 1024 * 1024 || !["image/png", "image/jpeg"].includes(file.type)) {
    out.textContent = "\u8BF7\u9009\u62E95MB\u4EE5\u5185\u7684PNG\u6216JPEG\u56FE\u7247\u3002";
    return;
  }
  const url = URL.createObjectURL(file);
  try {
    const img = new Image();
    img.src = url;
    await img.decode();
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = 144;
    const ctx = canvas.getContext("2d");
    const size = Math.min(img.width, img.height);
    ctx.drawImage(img, (img.width - size) / 2, (img.height - size) / 2, size, size, 0, 0, 144, 144);
    yyEditingProfileAvatar = canvas.toDataURL("image/png");
    yyDrawProfilePreview(yyEditingProfileAvatar);
    out.textContent = "\u5934\u50CF\u5DF2\u51C6\u5907\uFF0C\u4FDD\u5B58\u8D44\u6599\u540E\u751F\u6548\u3002";
  } catch {
    out.textContent = "\u56FE\u7247\u65E0\u6CD5\u8BFB\u53D6\uFF0C\u8BF7\u91CD\u65B0\u9009\u62E9\u3002";
  } finally {
    URL.revokeObjectURL(url);
  }
}
async function yySaveAccountProfileUI() {
  const b = this;
  b.disabled = true;
  try {
    const body = { action: "profile", id: yyEditingProfile.id, name: document.getElementById("yy-profile-name").value, profile_role: document.getElementById("yy-profile-role").value };
    if (yyEditingProfileAvatar !== null) body.avatar = yyEditingProfileAvatar;
    const r = await yyRootAPI("POST", "/api/youyuan/accounts", body);
    if (!r.ok) {
      document.getElementById("yy-profile-result").textContent = r.error;
      return;
    }
    yyAccountFocusID = yyEditingProfile.id;
    yyNavigate("accounts");
    T("\u8D26\u53F7\u8D44\u6599\u5DF2\u4FDD\u5B58", "o");
  } finally {
    b.disabled = false;
  }
}
var yyProjectFilter = "", yyCenterCloudRows = [], yyCloudReadEpoch = 0;
function yyRenderProjects() {
  const host = document.getElementById("yy-projects");
  if (!host) return;
  const servers = [...new Set(yyAllAccountRows.map((a) => a.server).filter(Boolean))];
  host.innerHTML = `<span>\u6E38\u620F\u9879\u76EE</span><button class="btn-s" onclick="yyChooseProject('')">\u5168\u90E8\u8D26\u53F7</button>`;
  for (const server of servers) {
    const b = document.createElement("button");
    b.className = "btn-s" + (yyProjectFilter === server ? " selected" : "");
    b.textContent = server + " \xB7 " + yyAllAccountRows.filter((a) => a.server === server).length + " \u4E2A";
    b.onclick = () => yyChooseProject(server);
    host.append(b);
  }
  const overview = document.getElementById("yy-account-overview");
  if (overview) overview.innerHTML = "<div><strong>" + yyMergedAccountRows().length + "</strong><span>\u6E38\u620F\u8D26\u53F7</span></div><div><strong>" + yyAllAccountRows.filter((a) => a.connected === true).length + "</strong><span>\u5DF2\u786E\u8BA4\u8FDE\u63A5 \xB7 \u5F85\u67E5\u8BE2 " + yyAllAccountRows.filter((a) => a.connection_known === false || a.connected == null).length + "</span></div><div><strong>" + servers.length + "</strong><span>\u670D\u52A1\u5668\u9879\u76EE</span></div><div><strong>" + yyAllAccountRows.filter((a) => {
    var _a;
    return ((_a = a.pool_binding) == null ? void 0 : _a.phase) === "bound";
  }).length + "</strong><span>\u7ED1\u5B9A\u4E2D\u7684\u673A\u5668\u4EBA</span></div>";
}
function yyChooseProject(server) {
  yyProjectFilter = server;
  yyAccountPage = 1;
  yyRenderProjects();
  yyRenderAccountCards();
}
function yyAccountAdmin() {
  return typeof yyPlatformUser !== "undefined" && !!yyPlatformUser && (yyPlatformUser.role === "admin" || yyPlatformUser.legacy_owner === true);
}
function yyUpdateAdminPoolEntry() {
  const button2 = document.getElementById("yy-admin-pool-entry");
  if (button2) button2.hidden = !yyAccountAdmin();
}
async function yyAccountPanel(name) {
  if (name === "pool") {
    await yyPoolPanel(true);
    return;
  }
  const host = document.getElementById("yy-game-accounts-panel");
  if (host) host.hidden = false;
}
async function yyCenterCloudAccounts() {
  const host = document.getElementById("yy-accounts-grid"), scope = yyAccountControllerScope(), epoch = ++yyCloudReadEpoch;
  if (!host) return;
  const result = await yyGameAuthAPI("GET");
  if (!host.isConnected || epoch !== yyCloudReadEpoch || scope !== yyAccountControllerScope() || document.getElementById("yy-accounts-grid") !== host) return;
  const status = document.getElementById("yy-account-cloud-status");
  if (!result.ok) {
    if (status) status.textContent = result.error || "\u5B98\u65B9\u8D26\u53F7\u8D44\u6599\u6682\u4E0D\u53EF\u7528\uFF1B\u5DF2\u663E\u793A\u672C\u8BBE\u5907\u8D26\u53F7\u3002";
    return;
  }
  yyCenterCloudRows = result.accounts || [];
  if (status) status.textContent = yyCenterCloudRows.length ? "\u5B98\u65B9\u8D44\u6599\u5DF2\u5408\u5E76\u5230\u4E0A\u65B9\u8D26\u53F7\u5217\u8868\u3002" : "\u5C1A\u672A\u63A5\u5165\u4E91\u7AEF\u5B98\u65B9\u8D26\u53F7\uFF0C\u53EF\u7EE7\u7EED\u6DFB\u52A0\u672C\u5730\u4EE4\u724C\u3002";
  yyRenderProjects();
  yyRenderAccountCards();
}
async function yyAutoHostCredential(result) {
  var _a;
  if (!result.login_token || !result.account) return;
  const scope = yyAccountControllerScope(), row = result.account, existing = yyAccountRows.find((a) => a.id === "yy_" + row.id), save = await yyRootAPI("POST", "/api/youyuan/accounts", { action: "save", defer_connect: true, account: { ...existing, id: "yy_" + row.id, name: (existing == null ? void 0 : existing.name) || row.nickname || row.name, token: result.login_token, auth_url: result.auth_url || (existing == null ? void 0 : existing.auth_url) || "https://auth.youyuanqi.dpdns.org", keep_alive: (_a = existing == null ? void 0 : existing.keep_alive) != null ? _a : true } });
  if (!save.ok) throw Error("\u6E38\u620F\u8D26\u53F7\u5DF2\u5728\u4E91\u7AEF\u4FDD\u5B58\uFF0C\u4F46\u8BBE\u5907\u6258\u7BA1\u5931\u8D25\uFF1A" + save.error);
  if (scope === yyAccountControllerScope()) yyAccountFocusID = save.id || "yy_" + row.id;
}
async function yyPoolCall(action, args) {
  const r = await yyRootAPI("POST", "/api/youyuan/account-pool", { action, args });
  if (!r.ok) throw Error(r.error || "\u8D26\u53F7\u6C60\u64CD\u4F5C\u5931\u8D25");
  return r.result;
}
var yyPoolRows = [], yyPoolQuery = "", yyPoolFilter = "all", yyPoolReadEpoch = 0, yyPoolSelectedID = "";
async function yyPoolPanel() {
  if (!yyAccountAdmin()) {
    T("\u673A\u5668\u4EBA\u6258\u7BA1\u6C60\u7531\u7BA1\u7406\u5458\u7BA1\u7406\uFF1B\u81EA\u5DF1\u7684\u8D26\u53F7\u8BF7\u5728\u6E38\u620F\u8D26\u53F7\u5217\u8868\u64CD\u4F5C", "e");
    return;
  }
  delete subPageCache["\u673A\u5668\u4EBA\u6258\u7BA1\u6C60"];
  openSubPage("\u673A\u5668\u4EBA\u6258\u7BA1\u6C60", yyIntro("\u673A\u5668\u4EBA\u6258\u7BA1\u6C60", "\u7BA1\u7406\u5458\u7EDF\u4E00\u67E5\u770B\u591A\u53F0\u673A\u5668\u4EBA\uFF1B\u53EA\u5728\u6253\u5F00\u5355\u4E2A\u8D26\u53F7\u8BE6\u60C5\u65F6\u67E5\u8BE2\u5B9E\u65F6\u72B6\u6001\u3002") + '<section class="card yy-admin-list-panel"><div class="yy-admin-toolbar"><label class="yy-search"><input id="yy-pool-search" aria-label="\u641C\u7D22\u6258\u7BA1\u673A\u5668\u4EBA" placeholder="\u641C\u7D22\u8D26\u53F7\u3001\u670D\u52A1\u5668\u6216\u7ED1\u5B9A\u73A9\u5BB6" oninput="yyPoolQuery=this.value;yyDrawPoolRows()"></label><select id="yy-pool-filter" aria-label="\u6258\u7BA1\u673A\u5668\u4EBA\u5206\u7C7B" onchange="yyPoolFilter=this.value;yyDrawPoolRows()"><option value="all">\u5168\u90E8\u8D26\u53F7</option><option value="enabled">\u5DF2\u542F\u7528\u8FDE\u63A5</option><option value="disabled">\u672A\u542F\u7528\u8FDE\u63A5</option><option value="bound">\u5DF2\u7ED1\u5B9A\u73A9\u5BB6</option><option value="idle">\u7A7A\u95F2 / \u672A\u7ED1\u5B9A</option><option value="credential_missing">\u7F3A\u5C11\u4EE4\u724C</option></select><button class="btn-s" onclick="yyPoolRefresh()">\u5237\u65B0\u5217\u8868</button></div><p id="yy-pool-feedback" class="yy-hint" role="status">\u6B63\u5728\u8BFB\u53D6\u6258\u7BA1\u6C60\u2026</p><div id="yy-pool-list" class="yy-compact-list"></div></section>');
  document.getElementById("yy-pool-search").value = yyPoolQuery;
  document.getElementById("yy-pool-filter").value = yyPoolFilter;
  await yyPoolRefresh();
}
function yyPoolFilteredRows(rows, query, state) {
  const q = String(query || "").trim().toLowerCase();
  return rows.filter((a) => {
    var _a, _b, _c;
    return (!q || [a.name, a.server, (_a = a.lease) == null ? void 0 : _a.player].join(" ").toLowerCase().includes(q)) && (state === "all" || state === "enabled" && a.enabled || state === "disabled" && !a.enabled || state === "bound" && ((_b = a.lease) == null ? void 0 : _b.phase) === "bound" || state === "idle" && ((_c = a.lease) == null ? void 0 : _c.phase) !== "bound" || state === "credential_missing" && !a.has_token);
  });
}
async function yyPoolRefresh() {
  if (!yyAccountAdmin()) return;
  const epoch = ++yyPoolReadEpoch, scope = yyReadScope(), host = document.getElementById("yy-pool-list"), out = document.getElementById("yy-pool-feedback");
  if (!host) return;
  if (out) out.textContent = "\u6B63\u5728\u5237\u65B0\u672C\u673A\u6258\u7BA1\u8D44\u6599\u2026";
  try {
    const result = await yyPoolCall("list", [{ all: true }]);
    if (epoch !== yyPoolReadEpoch || scope !== yyReadScope() || document.getElementById("yy-pool-list") !== host) return;
    yyPoolRows = result.accounts || [];
    yyDrawPoolRows();
  } catch (error) {
    if (document.getElementById("yy-pool-list") === host && out) out.textContent = error.message;
  }
}
function yyDrawPoolRows() {
  const host = document.getElementById("yy-pool-list");
  if (!host) return;
  const rows = yyPoolFilteredRows(yyPoolRows, yyPoolQuery, yyPoolFilter), out = document.getElementById("yy-pool-feedback");
  if (out) out.textContent = "\u5171 " + yyPoolRows.length + " \u4E2A\u6258\u7BA1\u8D26\u53F7 \xB7 \u5F53\u524D\u5339\u914D " + rows.length + " \u4E2A \xB7 \u5217\u8868\u4EC5\u663E\u793A\u5DF2\u4FDD\u5B58\u8D44\u6599\uFF0C\u8FDE\u63A5\u72B6\u6001\u9700\u5728\u8BE6\u60C5\u67E5\u8BE2";
  host.innerHTML = rows.map(function(a) {
    var _a;
    const index = yyPoolRows.indexOf(a), profile = yyAllAccountRows.find((p) => p.id === a.id);
    return '<button class="yy-compact-row" onclick="yyPoolDetail(' + index + ')">' + yyAccountAvatarHTML(profile || a) + '<span class="yy-compact-identity"><strong>' + yyEscape(a.name || a.id) + "</strong><small>" + yyEscape((a.server || "\u672A\u9009\u62E9\u670D\u52A1\u5668") + (((_a = a.lease) == null ? void 0 : _a.phase) === "bound" ? " \xB7 " + a.lease.player : "")) + '</small></span><span class="yy-compact-status">' + (a.has_token ? a.enabled ? "\u5DF2\u542F\u7528" : "\u672A\u542F\u7528" : "\u7F3A\u5C11\u4EE4\u724C") + '</span><span class="yy-row-chevron" aria-hidden="true">\u203A</span></button>';
  }).join("") || '<p class="yy-empty">\u6CA1\u6709\u7B26\u5408\u6761\u4EF6\u7684\u6258\u7BA1\u8D26\u53F7</p>';
}
function yyPoolDetail(index) {
  var _a;
  if (!yyAccountAdmin()) {
    T("\u9700\u8981\u7BA1\u7406\u5458\u6743\u9650", "e");
    return;
  }
  const account = yyPoolRows[index];
  if (!account) return;
  yyPoolSelectedID = account.id;
  delete subPageCache["\u6258\u7BA1\u673A\u5668\u4EBA\u8BE6\u60C5"];
  delete _savedFormState["\u6258\u7BA1\u673A\u5668\u4EBA\u8BE6\u60C5"];
  const localIndex = yyAccountRows.findIndex((a) => a.id === account.id), buttons = yyAccountActionHTML("yyPoolDetailRefresh", [], "\u67E5\u8BE2\u5B9E\u65F6\u72B6\u6001") + yyAccountActionHTML("yyAccountTokenAction", [account.id, true], "\u590D\u5236\u4EE4\u724C") + (localIndex >= 0 ? yyAccountActionHTML("yyEditAccount", [localIndex], "\u7F16\u8F91\u8D26\u53F7\u914D\u7F6E") : "");
  openSubPage("\u6258\u7BA1\u673A\u5668\u4EBA\u8BE6\u60C5", yyIntro(account.name || account.id, "\u8FDE\u63A5\u72B6\u6001\u3001\u670D\u52A1\u5668\u548C\u73A9\u5BB6\u7ED1\u5B9A\u5206\u5F00\u7BA1\u7406\u3002") + '<section class="card"><div class="yy-account-meta"><span>\u8D26\u53F7\u6807\u8BC6 <b>' + yyEscape(account.id) + "</b></span><span>\u8D26\u53F7\u51ED\u636E <b>" + (account.has_token ? "\u5DF2\u4FDD\u5B58\uFF0C\u5C1A\u672A\u9A8C\u8BC1\u672C\u6B21\u8FDB\u670D" : "\u7F3A\u5C11\u4EE4\u724C") + "</b></span><span>\u81EA\u52A8\u91CD\u8FDE <b>" + (account.keep_alive ? "\u5DF2\u542F\u7528" : "\u672A\u542F\u7528") + '</b></span></div><p class="yy-hint" id="yy-pool-live" role="status">\u8FDE\u63A5\u72B6\u6001\u5F85\u67E5\u8BE2</p><div class="yy-actions">' + buttons + '</div><label>\u76EE\u6807\u670D\u52A1\u5668<input id="yy-pool-server" value="' + yyEscape(account.server || "") + '" placeholder="\u670D\u52A1\u5668\u53F7 / \u623F\u95F4\u53F7"></label><label>\u7ED1\u5B9A\u73A9\u5BB6<input id="yy-pool-player" value="' + yyEscape(((_a = account.lease) == null ? void 0 : _a.player) || "") + '" placeholder="\u73A9\u5BB6\u6E38\u620F\u540D"></label><p class="yy-hint">\u4FDD\u5B58\u7ED1\u5B9A\u4E0D\u4F1A\u81EA\u52A8\u4F20\u9001\uFF1B\u8FDE\u63A5\u8BF7\u6C42\u548C\u89C2\u5BDF\u7B56\u7565\u5206\u522B\u6267\u884C\u3002\u505C\u6B62\u4F1A\u53D6\u6D88\u6B64\u673A\u5668\u4EBA\u7684\u6258\u7BA1\u8FDE\u63A5\u4E0E\u7ED1\u5B9A\u3002</p><div class="yy-actions">' + ["bind", "start", "release", "stop"].map((action, i) => yyAccountActionHTML("yyPoolDetailAction", [action], ["\u4FDD\u5B58\u7ED1\u5B9A", "\u8FDE\u63A5\u6B64\u670D\u52A1\u5668", "\u91CA\u653E\u5E76\u7B49\u5F85 60 \u79D2", "\u505C\u6B62\u673A\u5668\u4EBA"][i], action === "start" ? "btn" : "btn-s")).join("") + '</div><p id="yy-pool-detail-result" role="status"></p></section>');
}
async function yyPoolDetailRefresh() {
  if (!yyAccountAdmin()) return;
  const id = yyPoolSelectedID, scope = yyReadScope(), host = document.getElementById("yy-pool-live");
  if (!host || host.dataset.busy === "true") return;
  host.dataset.busy = "true";
  host.textContent = "\u6B63\u5728\u67E5\u8BE2\u8BE5\u673A\u5668\u4EBA\u2026";
  try {
    const status = await yyPoolCall("status", [id]);
    if (id !== yyPoolSelectedID || scope !== yyReadScope() || document.getElementById("yy-pool-live") !== host) return;
    host.textContent = yyAccountStatusLabel(status) + " \xB7 " + (status.server || "\u672A\u9009\u62E9\u670D\u52A1\u5668") + " \xB7 \u5728\u7EBF\u4EBA\u6570 " + (status.online == null ? "\u672A\u77E5" : status.online);
  } catch (error) {
    if (document.getElementById("yy-pool-live") === host) host.textContent = error.message;
  } finally {
    host.dataset.busy = "false";
  }
}
async function yyPoolDetailAction(action) {
  if (!yyAccountAdmin()) {
    T("\u9700\u8981\u7BA1\u7406\u5458\u6743\u9650", "e");
    return;
  }
  const id = yyPoolSelectedID, scope = yyReadScope(), out = document.getElementById("yy-pool-detail-result");
  if (!out || out.dataset.busy === "true") return;
  const args = action === "release" ? [id, { idle_seconds: 60 }] : action === "stop" ? [id] : [id, { player: document.getElementById("yy-pool-player").value.trim(), server: document.getElementById("yy-pool-server").value.trim() }];
  out.dataset.busy = "true";
  out.textContent = "\u6B63\u5728\u63D0\u4EA4\u64CD\u4F5C\u2026";
  try {
    await yyPoolCall(action, args);
    if (scope !== yyReadScope() || document.getElementById("yy-pool-detail-result") !== out) return;
    out.textContent = action === "start" ? "\u8FDE\u63A5\u8BF7\u6C42\u5DF2\u63D0\u4EA4\uFF0C\u8BF7\u67E5\u8BE2\u5B9E\u65F6\u72B6\u6001\u786E\u8BA4\u7ED3\u679C" : "\u64CD\u4F5C\u5DF2\u4FDD\u5B58";
    T(out.textContent, "o");
    const row = yyPoolRows.find((a) => a.id === id);
    if (row) {
      if (action === "bind") row.lease = { phase: "bound", player: args[1].player };
      if (action === "release") row.lease = { phase: "idle" };
      if (action === "stop") {
        row.enabled = false;
        row.lease = null;
      }
      if (action === "start") {
        row.enabled = true;
        row.server = args[1].server || row.server;
      }
    }
  } catch (error) {
    if (document.getElementById("yy-pool-detail-result") === out) out.textContent = error.message;
    T(error.message, "e");
  } finally {
    out.dataset.busy = "false";
  }
}
(window.__subPageRestoreHooks = window.__subPageRestoreHooks || []).push({ key: "\u673A\u5668\u4EBA\u6258\u7BA1\u6C60", fn: function() {
  yyDrawPoolRows();
} });
function yyRenderAuthPending(rows, batches) {
  const host = document.getElementById("yy-ga-pending");
  if (!host) return;
  host.textContent = "";
  for (const task of rows) {
    const card = document.createElement("section");
    card.className = "card";
    const title = document.createElement("h4"), status = document.createElement("p");
    title.textContent = (task.name || task.nickname || "\u6E38\u620F\u8D26\u53F7") + " \xB7 " + (task.verification_required ? "\u5F85\u672C\u4EBA\u9A8C\u8BC1" : "\u5F85\u91CD\u8BD5\u767B\u5F55");
    status.textContent = task.last_error || "\u7EE7\u7EED\u539F\u4EFB\u52A1\uFF0C\u4E0D\u91CD\u590D\u521B\u5EFA\u8D26\u53F7";
    status.setAttribute("role", "status");
    card.append(title, status);
    if (task.verification_url) {
      const b = document.createElement("button");
      b.className = "btn-s";
      b.textContent = "\u6253\u5F00\u5B98\u65B9\u9A8C\u8BC1";
      b.onclick = () => window.android && android.openVerification ? android.openVerification(task.verification_url) : window.open(task.verification_url, "_blank", "noopener");
      card.append(b);
    }
    let pwd, code;
    if (task.action === "login_4399") {
      pwd = document.createElement("input");
      pwd.type = "password";
      pwd.placeholder = "4399\u5BC6\u7801";
      pwd.autocomplete = "off";
      card.append(pwd);
      if (task.captcha_url) {
        const image = document.createElement("img");
        image.src = task.captcha_url;
        image.alt = "4399\u9A8C\u8BC1\u7801";
        code = document.createElement("input");
        code.placeholder = "\u56FE\u7247\u9A8C\u8BC1\u7801";
        card.append(image, code);
      }
    }
    const next = document.createElement("button");
    next.className = "btn";
    next.textContent = "\u91CD\u8BD5 / \u9A8C\u8BC1\u5B8C\u6210\u540E\u7EE7\u7EED";
    next.onclick = async () => {
      next.disabled = true;
      status.textContent = "\u6B63\u5728\u7EE7\u7EED\u2026";
      try {
        const r = await yyGameAuthAPI("POST", { action: task.action === "login_4399" ? "resume_4399" : "resume_guest", id: task.id, password: (pwd == null ? void 0 : pwd.value) || "", captcha: (code == null ? void 0 : code.value) || "", captcha_id: task.captcha_id || "" });
        if (!r.ok) {
          status.textContent = r.error || "\u672A\u5B8C\u6210";
          if (r.verification_kind === "captcha_4399") yy4399Verification(r);
          return;
        }
        await yyAutoHostCredential(r);
        await yyGameAuthPage();
        yyGameAuthShowCredential(r, r.account);
      } catch (e) {
        status.textContent = e.message;
      } finally {
        next.disabled = false;
      }
    };
    const cancel = document.createElement("button");
    cancel.className = "btn-s";
    cancel.textContent = "\u53D6\u6D88\u5F85\u5904\u7406\u4EFB\u52A1";
    cancel.onclick = async () => {
      const r = await yyGameAuthAPI("POST", { action: "cancel_guest", id: task.id });
      if (r.ok) await yyGameAuthRefresh();
      else status.textContent = r.error;
    };
    card.append(next, cancel);
    host.append(card);
  }
  if (batches.length) {
    const b = document.createElement("button");
    b.className = "btn-s";
    b.textContent = "\u67E5\u770B\u6279\u91CF\u4EFB\u52A1\u4E0E\u9010\u4E2A\u547D\u540D";
    b.onclick = () => window.open("https://auth.youyuanqi.dpdns.org/", "_blank", "noopener");
    host.append(b);
  }
}
async function yyCenterRefresh() {
  await Promise.all([yyRefreshAccounts(true), yyCenterCloudAccounts()]);
}
async function yyCopyAccountToken(token) {
  try {
    await navigator.clipboard.writeText(token);
    T("\u4EE4\u724C\u5DF2\u590D\u5236", "o");
  } catch (e) {
    yyShowTokenDialog(token);
    T("\u8BF7\u957F\u6309\u4EE4\u724C\u6846\u5168\u9009\u590D\u5236", "i");
  }
}
function yyShowTokenDialog(token) {
  openSubPage("\u8D26\u53F7\u4EE4\u724C", `<section class="card"><h3>\u8D26\u53F7\u4EE4\u724C</h3><p class="yy-hint">\u6B64\u4EE4\u724C\u5141\u8BB8\u8FDE\u63A5\u8BE5\u6E38\u620F\u8D26\u53F7\uFF0C\u8BF7\u59A5\u5584\u4FDD\u5B58\u3002</p><textarea id="yy-view-token" readonly rows="4" aria-label="\u8D26\u53F7\u4EE4\u724C"></textarea><div class="yy-actions"><button class="btn" onclick="yyCopyAccountToken(document.getElementById('yy-view-token').value)">\u590D\u5236\u4EE4\u724C</button><button class="btn-s" onclick="document.getElementById('yy-view-token').value='';delete subPageCache['\u8D26\u53F7\u4EE4\u724C'];delete _savedFormState['\u8D26\u53F7\u4EE4\u724C'];closeSubPage()">\u5173\u95ED</button></div></section>`);
  document.getElementById("yy-view-token").value = token;
}
async function yyAccountTokenAction(id, copy) {
  var _a;
  const field = ((_a = window.yyEditingAccount) == null ? void 0 : _a.id) === id ? document.getElementById("yy-ac-token") : null;
  if (!copy && (field == null ? void 0 : field.type) === "text") {
    field.type = "password";
    return;
  }
  let token = (field == null ? void 0 : field.value) || "";
  if (!token) {
    const r = await yyRootAPI("POST", "/api/youyuan/accounts", { action: "token", id });
    if (!r.ok || !r.token) {
      T(r.error || "\u8FD9\u4E2A\u8D26\u53F7\u5C1A\u672A\u4FDD\u5B58\u4EE4\u724C", "e");
      return;
    }
    token = r.token;
  }
  if (copy) {
    await yyCopyAccountToken(token);
  } else if (field) {
    field.value = token;
    field.type = "text";
  } else {
    yyShowTokenDialog(token);
  }
}
async function yyPatrolAPI(id, method, args = []) {
  const r = await yyRootAPI("POST", yyAccountPath("/api/youyuan/game/call", id), { method: "observerBridge." + method, args });
  if (!r.ok) throw Error(r.error);
  return r.result;
}
function yyObservationPage(id) {
  window.yyPatrolAccount = id;
  openSubPage("\u673A\u5668\u4EBA\u89C2\u5BDF", '<section class="card"><h3>\u987A\u5E8F\u8F6E\u5DE1 / \u5355\u4EBA\u8DDF\u968F</h3><p class="yy-hint">\u5F53\u524D\u8D26\u53F7\uFF1A' + yyEscape(yyAccountName(id)) + '\u3002\u9700\u8981\u673A\u5668\u4EBA OP\u3002\u53EA\u6709\u505C\u7559\u5728\u9644\u8FD1\u65F6\u624D\u53EF\u80FD\u6536\u5230\u89C6\u8DDD\u5185\u52A8\u4F5C\uFF0C\u8F6E\u5DE1\u6709\u7A7A\u6863\u3002</p><label>\u73A9\u5BB6\u540D\u5355\uFF08\u6BCF\u884C\u4E00\u4E2A\uFF1B\u7559\u7A7A\u4F7F\u7528\u5F53\u524D\u5728\u7EBF\u5217\u8868\uFF09</label><textarea id="yy-patrol-players" rows="3"></textarea><div class="yy-form-grid"><div><label>\u591A\u73A9\u5BB6\u4F20\u9001\u95F4\u9694\uFF08\u79D2\uFF09</label><input id="yy-patrol-interval" type="number" step="0.001" value="0.001"></div><div><label>\u5355\u73A9\u5BB6\u8DDF\u968F\u95F4\u9694\uFF08\u79D2\uFF09</label><input id="yy-patrol-single" type="number" step="0.1" value="2"></div><div><label>\u672C\u6B21\u8FD0\u884C\u65F6\u957F\uFF08\u79D2\uFF1B0\u4E3A\u6301\u7EED\uFF09</label><input id="yy-patrol-duration" type="number" step="1" value="60"></div></div><p class="yy-hint">\u66F4\u5FEB\u63D0\u4EA4\u4E0D\u4FDD\u8BC1\u670D\u52A1\u5668\u66F4\u5FEB\u4F20\u9001\u6216\u66F4\u591A\u6536\u5305\u3002\u505C\u6B62\u540E\u6E05\u7406\u4E34\u65F6\u5206\u6570\u5E76\u8BF7\u6C42\u8FD4\u56DE\u8D77\u70B9\u3002</p><div class="yy-actions"><button class="btn" onclick="yyStartPatrolUI()">\u5F00\u59CB\u89C2\u5BDF</button><button class="btn-s" onclick="yyStopPatrolUI()">\u505C\u6B62\u5E76\u8FD4\u56DE</button><button class="btn-s" onclick="yyPatrolStatusUI()">\u5237\u65B0\u72B6\u6001</button><button class="btn-s" onclick="closeSubPage()">\u8FD4\u56DE</button></div><div id="yy-patrol-status" class="yy-readout" aria-live="polite"></div></section>');
  yyPatrolStatusUI();
}
async function yyStartPatrolUI() {
  try {
    const players = document.getElementById("yy-patrol-players").value.split(/\r?\n/).map((x) => x.trim()).filter(Boolean);
    await yyPatrolAPI(yyPatrolAccount, "patrolStart", [{ players, interval_seconds: Number(document.getElementById("yy-patrol-interval").value), single_follow_seconds: Number(document.getElementById("yy-patrol-single").value), duration_seconds: Number(document.getElementById("yy-patrol-duration").value), restore_position: true }]);
    T("\u89C2\u5BDF\u5DF2\u542F\u52A8", "o");
    await yyPatrolStatusUI();
  } catch (e) {
    T(e.message, "e");
  }
}
async function yyStopPatrolUI() {
  try {
    await yyPatrolAPI(yyPatrolAccount, "patrolStop");
    await yyPatrolStatusUI();
    T("\u5DF2\u505C\u6B62\uFF1B\u6E05\u7406\u4E0E\u8FD4\u56DE\u5DF2\u8BF7\u6C42", "o");
  } catch (e) {
    T(e.message, "e");
  }
}
async function yyPatrolStatusUI() {
  const el = document.getElementById("yy-patrol-status");
  if (!el) return;
  try {
    const s = await yyPatrolAPI(yyPatrolAccount, "patrolStatus");
    if (document.getElementById("yy-patrol-status") !== el) return;
    el.textContent = [s.active ? "\u6B63\u5728\u89C2\u5BDF" : "\u5DF2\u505C\u6B62", "\u4F20\u9001\u63D0\u4EA4\uFF1A" + (s.submitted || 0) + " \xB7 \u8DEF\u7EBF\u8F6E\u6570\uFF1A" + (s.rounds || 0), "\u5B9E\u9645\u4F4D\u7F6E\u66F4\u65B0\uFF1A" + (s.position_updates || 0) + " \xB7 \u9519\u8FC7\u8C03\u5EA6\u95F4\u9694\uFF1A" + (s.missed_intervals || 0), "\u4E8B\u4EF6\uFF1A" + Object.entries(s.events || {}).map(([k, v]) => k + " " + v).join(" / "), "\u73A9\u5BB6\uFF1A" + (s.players || []).map((r) => r.score + ". " + r.player).join(" \u2192 "), s.error ? "\u9519\u8BEF\uFF1A" + s.error : "", "\u63D0\u4EA4\u6570\u548C\u4F4D\u7F6E\u66F4\u65B0\u6570\u90FD\u4E0D\u662F\u670D\u52A1\u7AEF\u4F20\u9001\u5B8C\u6210\u6B21\u6570"].filter(Boolean).join("\n");
  } catch (e) {
    el.textContent = e.message;
  }
}
function yyRelogGameAccount(id) {
  window.yyRelogAccountID = id;
  openSubPage("\u66F4\u65B0\u6E38\u620F\u51ED\u636E", '<section class="card"><h3>\u91CD\u65B0\u767B\u5F55\u539F\u8D26\u53F7</h3><p class="yy-hint">\u7528\u4E8E\u7F51\u6613\u51ED\u636E\u8FC7\u671F\u3002\u6E38\u620F\u8EAB\u4EFD\u5FC5\u987B\u4E0E\u539F\u8D26\u53F7\u4E00\u81F4\uFF1B\u6210\u529F\u540E\u539F\u4EE4\u724C\u3001\u6258\u7BA1\u8D26\u53F7\u4E0E\u63D2\u4EF6\u4ECD\u4FDD\u7559\u3002\u5B89\u5168\u9A8C\u8BC1\u9700\u8981\u672C\u4EBA\u5B8C\u6210\u3002</p><label>\u7F51\u6613\u90AE\u7BB1</label><input id="yy-relog-email" type="email" autocomplete="username"><label>\u7F51\u6613\u5BC6\u7801</label><input id="yy-relog-password" type="password" autocomplete="off"><div class="yy-actions"><button id="yy-relog-submit" class="btn" onclick="yySubmitGameRelog()">\u66F4\u65B0\u767B\u5F55\u51ED\u636E</button><button class="btn-s" onclick="yyCloseGameRelog()">\u8FD4\u56DE</button></div><p id="yy-relog-result" role="status"></p></section>');
}
function yyCloseGameRelog() {
  document.getElementById("yy-relog-password").value = "";
  delete subPageCache["\u66F4\u65B0\u6E38\u620F\u51ED\u636E"];
  delete _savedFormState["\u66F4\u65B0\u6E38\u620F\u51ED\u636E"];
  closeSubPage();
}
async function yySubmitGameRelog() {
  const b = document.getElementById("yy-relog-submit"), out = document.getElementById("yy-relog-result");
  b.disabled = true;
  out.textContent = "\u6B63\u5728\u767B\u5F55\u539F\u8D26\u53F7\u2026";
  try {
    const r = await yyGameAuthAPI("POST", { action: "reauthenticate", id: yyRelogAccountID, email: document.getElementById("yy-relog-email").value.trim(), password: document.getElementById("yy-relog-password").value });
    if (!r.ok) throw Error(r.error || "\u66F4\u65B0\u672A\u5B8C\u6210");
    document.getElementById("yy-relog-password").value = "";
    yyCloseGameRelog();
    await yyCenterRefresh();
    T("\u51ED\u636E\u5DF2\u66F4\u65B0\uFF0C\u539F\u4EE4\u724C\u4FDD\u6301\u6709\u6548\uFF1B\u53EF\u4EE5\u91CD\u65B0\u8FDE\u63A5", "o");
  } catch (e) {
    out.textContent = e.message;
  } finally {
    b.disabled = false;
  }
}
