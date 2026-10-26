var yyPlatformUser = null, yyIdentityMode = "login", yyMailRetryUntil = 0, yyMailTimer = null;
var yyIsCloud = location.hostname !== "127.0.0.1" && location.hostname !== "localhost";
var YY_IDENTITY_TIMEOUT_MS = 8e4;
async function yyIdentityRequest(action, body) {
  var path = yyIsCloud ? "/api/identity/" + action : "/api/youyuan/identity";
  var controller = new AbortController(), timer = setTimeout(function() {
    controller.abort();
  }, YY_IDENTITY_TIMEOUT_MS);
  try {
    var result = await yyRootFetch(path, { method: action === "me" ? "GET" : "POST", headers: { "Content-Type": "application/json" }, body: action === "me" ? void 0 : JSON.stringify(Object.assign({ action }, body || {})), signal: controller.signal });
    return await result.json();
  } catch (e) {
    if (e && e.name === "AbortError") return { ok: false, error: "\u4E91\u7AEF\u54CD\u5E94\u8D85\u65F6\uFF08" + Math.round(YY_IDENTITY_TIMEOUT_MS / 1e3) + "\u79D2\uFF09\u3002\u90AE\u4EF6\u53EF\u80FD\u4ECD\u5728\u53D1\u9001\uFF0C\u8D26\u53F7\u4E0D\u4F1A\u91CD\u590D\u521B\u5EFA\uFF1B\u8BF7\u7A0D\u540E\u91CD\u8BD5\uFF0C\u6216\u6362\u300C\u767B\u5F55\u300D\u786E\u8BA4\u662F\u5426\u5DF2\u6CE8\u518C\u6210\u529F\u3002", timeout: true };
    return { ok: false, error: e && e.message || "\u7F51\u7EDC\u8BF7\u6C42\u5931\u8D25\uFF0C\u8BF7\u91CD\u8BD5" };
  } finally {
    clearTimeout(timer);
  }
}
function yyShowIdentity() {
  if (yyMailTimer) {
    clearInterval(yyMailTimer);
    yyMailTimer = null;
  }
  yyMailRetryUntil = 0;
  var old = document.getElementById("yy-identity-gate");
  if (old) old.remove();
  var panel = document.createElement("div");
  panel.id = "yy-identity-gate";
  panel.innerHTML = `<form class="yy-identity-card" onsubmit="yyIdentitySubmit(event)"><div class="yy-identity-brand">\u6E38\u56ED \xB7 \u81EA\u5DF1\u7684\u521B\u4F5C\u7A7A\u95F4</div><h1>\u767B\u5F55\u6E38\u56ED</h1><p>\u5E73\u53F0\u8D26\u53F7\u7BA1\u7406\u4F60\u7684\u8BBE\u5907\u548C\u4E91\u7AEF\u8D44\u6599\uFF1B\u6E38\u620F\u4EE4\u724C\u5728\u767B\u5F55\u540E\u5355\u72EC\u6DFB\u52A0\u3002</p><div class="yy-actions"><button type="button" class="btn-s" onclick="yyIdentityChange('login')">\u767B\u5F55</button><button type="button" class="btn-s" onclick="yyIdentityChange('register')">\u6CE8\u518C\u8D26\u53F7</button></div><label for="yy-login-name">\u8D26\u53F7\u540D\u79F0</label><input id="yy-login-name" autocomplete="username" required minlength="3" maxlength="32"><div id="yy-login-email-row" hidden><label for="yy-login-email">QQ \u90AE\u7BB1</label><input id="yy-login-email" type="email" placeholder="123456@qq.com" autocomplete="email" maxlength="71"><label for="yy-login-email-code">\u90AE\u7BB1\u9A8C\u8BC1\u7801</label><div class="yy-mail-code"><input id="yy-login-email-code" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength="6" placeholder="6 \u4F4D\u9A8C\u8BC1\u7801"><button id="yy-send-email-code" type="button" class="btn-s" onclick="yyIdentitySendCode()">\u53D1\u9001\u9A8C\u8BC1\u7801</button></div><small>\u9A8C\u8BC1\u7801 10 \u5206\u949F\u5185\u6709\u6548\u3002\u586B\u5199\u81EA\u5DF1\u7684 QQ \u90AE\u7BB1\u6536\u53D6\u90AE\u4EF6\u3002</small></div><label for="yy-login-password">\u5BC6\u7801</label><input id="yy-login-password" type="password" autocomplete="current-password" required minlength="10" maxlength="128"><button id="yy-login-submit" class="btn" type="submit">\u767B\u5F55</button><p id="yy-login-result" role="status"></p>` + (yyIsCloud ? '<details><summary>\u5DF2\u6709\u8BBE\u5907\u7684\u63A7\u5236\u5BC6\u94A5</summary><label>\u63A7\u5236\u5BC6\u94A5</label><input id="yy-login-existing-key" type="password" autocomplete="off"><button type="button" class="btn-s" onclick="yyIdentityExisting()">\u8FDE\u63A5\u5DF2\u6709\u8BBE\u5907</button></details>' : "") + "</form>";
  document.body.append(panel);
  yyIdentityChange("login");
}
function yyIdentityChange(mode) {
  yyIdentityMode = mode;
  var register = mode === "register";
  document.querySelector(".yy-identity-card h1").textContent = register ? "\u521B\u5EFA\u6E38\u56ED\u8D26\u53F7" : "\u767B\u5F55\u6E38\u56ED";
  document.getElementById("yy-login-email-row").hidden = !register;
  document.getElementById("yy-login-email").required = register;
  document.getElementById("yy-login-email-code").required = register;
  document.getElementById("yy-login-submit").textContent = register ? "\u9A8C\u8BC1\u5E76\u6CE8\u518C" : "\u767B\u5F55";
  document.getElementById("yy-login-password").autocomplete = register ? "new-password" : "current-password";
  document.getElementById("yy-login-result").textContent = "";
}
async function yyIdentitySendCode() {
  var button = document.getElementById("yy-send-email-code"), out = document.getElementById("yy-login-result"), name = document.getElementById("yy-login-name"), email = document.getElementById("yy-login-email");
  if (!name.reportValidity() || !email.reportValidity()) return;
  if (!/^[a-z0-9][a-z0-9._-]{0,63}@qq\.com$/i.test(email.value.trim())) {
    out.textContent = "\u8BF7\u586B\u5199 QQ \u90AE\u7BB1\uFF0C\u4F8B\u5982 123456@qq.com";
    return;
  }
  button.disabled = true;
  out.textContent = "\u6B63\u5728\u53D1\u9001\u9A8C\u8BC1\u7801\u90AE\u4EF6\u2026";
  try {
    var r = await yyIdentityRequest("register-code", { username: name.value, email: email.value });
    if (!r.ok) throw Error(r.error || "\u9A8C\u8BC1\u7801\u672A\u53D1\u51FA");
    out.textContent = r.message;
    yyMailRetryUntil = Date.now() + r.retry_after * 1e3;
    if (yyMailTimer) clearInterval(yyMailTimer);
    var update = function() {
      var b = document.getElementById("yy-send-email-code");
      if (!b) {
        clearInterval(yyMailTimer);
        yyMailTimer = null;
        return;
      }
      var seconds = Math.max(0, Math.ceil((yyMailRetryUntil - Date.now()) / 1e3));
      b.disabled = seconds > 0;
      b.textContent = seconds ? seconds + " \u79D2\u540E\u91CD\u53D1" : "\u91CD\u65B0\u53D1\u9001";
      if (!seconds) {
        clearInterval(yyMailTimer);
        yyMailTimer = null;
      }
    };
    yyMailTimer = setInterval(update, 1e3);
    update();
  } catch (e) {
    out.textContent = e.message;
    button.disabled = false;
    button.textContent = "\u53D1\u9001\u9A8C\u8BC1\u7801";
  }
}
function yyIdentityReady(user) {
  if (yyMailTimer) {
    clearInterval(yyMailTimer);
    yyMailTimer = null;
  }
  if (yyIsCloud) {
    if (user && !user.cloud_allowed) {
      yySubscriptionGate(user);
      return;
    }
    location.reload();
    return;
  }
  yyPlatformUser = user;
  document.dispatchEvent(new Event("yy-identity-ready"));
  var gate = document.getElementById("yy-identity-gate");
  if (gate) gate.remove();
  tabInited = {};
  yyNavigate("home");
}
async function yyIdentitySubmit(event) {
  event.preventDefault();
  var button = document.getElementById("yy-login-submit"), out = document.getElementById("yy-login-result");
  button.disabled = true;
  out.textContent = "\u6B63\u5728\u8FDE\u63A5\u6E38\u56ED\u4E91\u7AEF\u2026";
  try {
    var r = await yyIdentityRequest(yyIdentityMode, { username: document.getElementById("yy-login-name").value, email: document.getElementById("yy-login-email").value, email_code: document.getElementById("yy-login-email-code").value, password: document.getElementById("yy-login-password").value });
    if (!r.ok) throw Error(r.error || "\u767B\u5F55\u5931\u8D25");
    if (yyIsCloud) {
      sessionStorage.setItem("yy-cloud-key", r.session_token);
      sessionStorage.setItem("yy-user-control", r.control_token);
    }
    document.getElementById("yy-login-password").value = "";
    yyIdentityReady(r.user);
    T("\u5DF2\u767B\u5F55\uFF1B\u8BBE\u5907\u4E0A\u7EBF\u540E\u53EF\u4F7F\u7528\u5BF9\u5E94\u5DE5\u4F5C\u533A", "o");
  } catch (e) {
    out.textContent = e.message;
    button.disabled = false;
  }
}
async function yyIdentityExisting() {
  var key = document.getElementById("yy-login-existing-key").value;
  sessionStorage.setItem("yy-cloud-key", key);
  try {
    var response = await yyRootFetch("/api/cloud/status");
    var r = await response.json();
    if (!response.ok || !r.ok) throw Error(r.error || "\u5BC6\u94A5\u4E0D\u6B63\u786E");
    sessionStorage.removeItem("yy-user-control");
    yyIdentityReady(r.user || { username: "\u5DF2\u6709\u8BBE\u5907\u63A7\u5236\u8005", qq_binding_status: "\u8BBE\u5907\u63A7\u5236\u5BC6\u94A5\u767B\u5F55" });
  } catch (e) {
    sessionStorage.removeItem("yy-cloud-key");
    document.getElementById("yy-login-result").textContent = e.message;
  }
}
async function yyPlatformPage() {
  var r = await yyIdentityRequest("me");
  if (!r.ok || !yyIsCloud && !r.authenticated) {
    yyShowIdentity();
    return;
  }
  var user = r.user || yyPlatformUser || {};
  var key = yyIsCloud ? sessionStorage.getItem("yy-user-control") : r.control_token;
  delete subPageCache["\u6E38\u56ED\u5E73\u53F0\u8D26\u53F7"];
  openSubPage("\u6E38\u56ED\u5E73\u53F0\u8D26\u53F7", yyIntro("\u5E73\u53F0\u8D26\u53F7", "\u4E91\u7AEF\u5BC6\u94A5\u5C5E\u4E8E\u5E73\u53F0\u7528\u6237\uFF1B\u672C\u673A MCP \u5BC6\u94A5\u5C5E\u4E8E\u5F53\u524D\u6E38\u620F\u8D26\u53F7\u7684\u6570\u636E\u76EE\u5F55\u3002") + '<section class="card"><p>\u8D26\u53F7\uFF1A' + yyEscape(user.username || "") + "</p><p>QQ \u90AE\u7BB1\uFF1A" + yyEscape(user.email || "\u5C1A\u672A\u7ED1\u5B9A\u90AE\u7BB1") + " \xB7 " + (user.email_verified ? "\u5DF2\u9A8C\u8BC1" : "\u672A\u9A8C\u8BC1") + "</p><label>\u81EA\u5DF1\u7684\u4E91\u7AEF MCP \u63A7\u5236\u5BC6\u94A5</label><textarea readonly>" + yyEscape(key || "\u4F7F\u7528\u5DF2\u6709\u8BBE\u5907\u79C1\u6709\u914D\u7F6E\u4E2D\u7684\u63A7\u5236\u5BC6\u94A5") + '</textarea><p class="yy-hint">\u516C\u7F51 MCP\uFF1A' + location.protocol + '//toolbox.youyuanqi.dpdns.org/mcp/\u3002\u8FD9\u4E2A\u5BC6\u94A5\u53EA\u80FD\u8BBF\u95EE\u81EA\u5DF1\u7684\u8BBE\u5907\u548C\u8D44\u6599\u3002\u5207\u6362\u6E38\u620F\u8D26\u53F7\u65F6\uFF0Capi_call \u7684 path \u4F7F\u7528 /api/youyuan/accounts/invoke/\u8D26\u53F7ID/api/ \u5BF9\u5E94\u8DEF\u5F84\u3002</p><button class="btn-s" onclick="yyPlatformLogout()">\u9000\u51FA\u5E73\u53F0\u767B\u5F55</button><button class="btn-s" onclick="yyPlatformDeletePage()">\u6CE8\u9500\u5E73\u53F0\u8D26\u53F7</button><p class="yy-hint">\u9000\u51FA\u53EA\u7ED3\u675F\u672C\u6B21\u767B\u5F55\uFF0C\u8D26\u53F7\u4ECD\u7136\u5B58\u5728\uFF1B\u6CE8\u9500\u624D\u4F1A\u91CA\u653E\u8D26\u53F7\u540D\u79F0\u548C\u5DF2\u7ED1\u5B9A\u90AE\u7BB1\u3002</p></section>');
}
async function yyPlatformLogout() {
  await yyIdentityRequest("logout");
  sessionStorage.removeItem("yy-cloud-key");
  sessionStorage.removeItem("yy-user-control");
  yyPlatformUser = null;
  if (yyIsCloud) {
    location.reload();
    return;
  }
  yyShowIdentity();
}
var yyIdentitySettingsBase = Rcfg;
Rcfg = async function() {
  var yySettingsStillLive = yySettingsViewGuard();
  await yyIdentitySettingsBase();
  if (!yySettingsStillLive()) return;
  var host = document.querySelector("#tab-settings .content-inner");
  var box = document.createElement("section");
  box.className = "card";
  box.innerHTML = yyCardHead("\u6E38\u56ED\u5E73\u53F0\u8D26\u53F7", "user") + '<p>QQ \u90AE\u7BB1\u9A8C\u8BC1\u7801\u6CE8\u518C\u3001\u8D26\u53F7\u6CE8\u9500\u3001\u4E91\u7AEF\u5BC6\u94A5\u4E0E\u8BBE\u5907\u8FDE\u63A5\u3002</p><button class="btn-s" onclick="yyPlatformPage()">\u7BA1\u7406\u5E73\u53F0\u8D26\u53F7</button>';
  host.insertBefore(box, host.querySelector(".yy-form-grid"));
};
function yySubscriptionGate(user) {
  window.yyCloudBlocked = true;
  yyPlatformUser = user;
  var old = document.getElementById("yy-identity-gate");
  if (old) old.remove();
  var panel = document.createElement("div");
  panel.id = "yy-identity-gate";
  panel.innerHTML = '<section class="yy-identity-card"><div class="yy-identity-brand">\u6E38\u56ED \xB7 \u4E91\u7AEF\u8BA2\u9605</div><h1>\u8D26\u53F7\u5DF2\u767B\u5F55</h1><p>' + yyEscape(user.username) + "\uFF0C\u4E91\u7AEF\u5DE5\u5177\u7BB1\u4EC5\u5F00\u653E\u7ED9\u7BA1\u7406\u5458\u548C\u6709\u6548\u8BA2\u9605\u7528\u6237\u3002</p><p>\u8BA2\u9605\uFF1A" + (user.subscription_until ? "\u5DF2\u5230\u671F \xB7 " + yyEscape(new Date(user.subscription_until).toLocaleString()) : "\u5C1A\u672A\u5F00\u901A") + '</p><p>\u8BF7\u8054\u7CFB\u7BA1\u7406\u5458\u5F00\u901A\u8BA2\u9605\u5E76\u5206\u914D Linux \u8282\u70B9\u3002APK \u672C\u673A\u5DE5\u5177\u4E0D\u53D7\u4E91\u7AEF\u8BA2\u9605\u9650\u5236\u3002</p><div class="yy-actions"><button class="btn" onclick="location.reload()">\u5237\u65B0\u6388\u6743</button><button class="btn-s" onclick="yyPlatformLogout()">\u9000\u51FA\u8D26\u53F7</button><button class="btn-s" onclick="yyPlatformDeletePage()">\u6CE8\u9500\u5E73\u53F0\u8D26\u53F7</button></div></section>';
  document.body.append(panel);
}
var yySubscriptionUsers = [], yySubscriptionFilter = "all", yySubscriptionQuery = "", yySubscriptionPage = 1, yySubscriptionRequestEpoch = 0;
function yySubscriptionState(user, now) {
  if (user.role === "admin") return "admin";
  if (Number(user.subscription_until) > Number(now || Date.now())) return "subscribed";
  if (Number(user.subscription_until) > 0) return "expired";
  return "member";
}
function yySubscriptionStateLabel(state) {
  return { admin: "\u7BA1\u7406\u5458", subscribed: "\u8BA2\u9605\u6709\u6548", expired: "\u8BA2\u9605\u5DF2\u5230\u671F", member: "\u666E\u901A\u7528\u6237" }[state] || "\u666E\u901A\u7528\u6237";
}
function yySubscriptionMatches(users, query, state, now) {
  const q = String(query || "").trim().toLowerCase();
  return users.filter((u) => (state === "all" || yySubscriptionState(u, now) === state) && (!q || [u.username, u.email].join(" ").toLowerCase().includes(q)));
}
async function yySubscriptionsPage() {
  const epoch = ++yySubscriptionRequestEpoch, scope = yyReadScope(), r = await yyIdentityRequest("users");
  if (epoch !== yySubscriptionRequestEpoch || scope !== yyReadScope()) return;
  if (!r.ok) {
    T(r.error || "\u65E0\u7BA1\u7406\u6743\u9650", "e");
    return;
  }
  yySubscriptionUsers = r.users || [];
  delete subPageCache["\u8BA2\u9605\u7528\u6237\u7BA1\u7406"];
  openSubPage("\u8BA2\u9605\u7528\u6237\u7BA1\u7406", yyIntro("\u8BA2\u9605\u7528\u6237\u7BA1\u7406", "\u5148\u627E\u5230\u7528\u6237\uFF0C\u518D\u6253\u5F00\u8BE6\u60C5\u8C03\u6574\u6743\u9650\u3001\u8BA2\u9605\u548C\u8BBE\u5907\u3002") + '<section class="card yy-admin-list-panel"><div class="yy-admin-toolbar"><label class="yy-search"><input id="yy-sub-search" aria-label="\u641C\u7D22\u8BA2\u9605\u7528\u6237" placeholder="\u641C\u7D22\u540D\u79F0\u6216 QQ \u90AE\u7BB1" oninput="yySubscriptionQuery=this.value;yySubscriptionPage=1;yyRenderSubscriptions()"></label><select id="yy-sub-state" aria-label="\u8BA2\u9605\u72B6\u6001\u5206\u7C7B" onchange="yySubscriptionFilter=this.value;yySubscriptionPage=1;yyRenderSubscriptions()"><option value="all">\u5168\u90E8\u7528\u6237</option><option value="admin">\u7BA1\u7406\u5458</option><option value="subscribed">\u8BA2\u9605\u6709\u6548</option><option value="expired">\u8BA2\u9605\u5DF2\u5230\u671F</option><option value="member">\u666E\u901A\u7528\u6237</option></select><button class="btn-s" onclick="yySubscriptionsPage()">\u5237\u65B0</button></div><p class="yy-hint" id="yy-sub-summary"></p><div id="yy-sub-list" class="yy-compact-list"></div><div id="yy-sub-pages" class="yy-pagination"></div></section>');
  document.getElementById("yy-sub-search").value = yySubscriptionQuery;
  document.getElementById("yy-sub-state").value = yySubscriptionFilter;
  yyRenderSubscriptions();
}
function yyRenderSubscriptions() {
  const host = document.getElementById("yy-sub-list");
  if (!host) return;
  const rows = yySubscriptionMatches(yySubscriptionUsers, yySubscriptionQuery, yySubscriptionFilter), size = 20, pages = Math.max(1, Math.ceil(rows.length / size));
  yySubscriptionPage = Math.min(pages, Math.max(1, yySubscriptionPage));
  document.getElementById("yy-sub-summary").textContent = "\u5171 " + yySubscriptionUsers.length + " \u540D\u7528\u6237 \xB7 \u5F53\u524D\u5339\u914D " + rows.length + " \u540D";
  host.innerHTML = rows.slice((yySubscriptionPage - 1) * size, yySubscriptionPage * size).map(function(u) {
    const index = yySubscriptionUsers.indexOf(u), state = yySubscriptionState(u);
    return '<button class="yy-compact-row" onclick="yySubscriptionDetail(' + index + ')"><img class="yy-compact-avatar" src="/assets/default-head.jpg" alt="\u7528\u6237\u5934\u50CF"><span class="yy-compact-identity"><strong>' + yyEscape(u.username) + "</strong><small>" + yyEscape(u.email || "\u5C1A\u672A\u7ED1\u5B9A\u90AE\u7BB1") + '</small></span><span class="yy-compact-status">' + yyEscape(yySubscriptionStateLabel(state)) + '</span><span class="yy-row-chevron" aria-hidden="true">\u203A</span></button>';
  }).join("") || '<p class="yy-empty">\u6CA1\u6709\u7B26\u5408\u6761\u4EF6\u7684\u7528\u6237</p>';
  document.getElementById("yy-sub-pages").innerHTML = '<button class="btn-s" ' + (yySubscriptionPage === 1 ? "disabled" : "") + ' onclick="yySubscriptionPage--;yyRenderSubscriptions()">\u4E0A\u4E00\u9875</button><span>' + yySubscriptionPage + " / " + pages + '</span><button class="btn-s" ' + (yySubscriptionPage === pages ? "disabled" : "") + ' onclick="yySubscriptionPage++;yyRenderSubscriptions()">\u4E0B\u4E00\u9875</button>';
}
function yySubscriptionDetail(index) {
  const user = yySubscriptionUsers[index];
  if (!user) return;
  const id = String(user.id), arg = yyEscape(JSON.stringify(id));
  delete subPageCache["\u8BA2\u9605\u7528\u6237\u8BE6\u60C5"];
  delete _savedFormState["\u8BA2\u9605\u7528\u6237\u8BE6\u60C5"];
  openSubPage("\u8BA2\u9605\u7528\u6237\u8BE6\u60C5", yyIntro(user.username, yySubscriptionStateLabel(yySubscriptionState(user))) + '<section class="card yy-subscription-detail"><div class="yy-detail-identity"><img class="yy-compact-avatar" src="/assets/default-head.jpg" alt="\u7528\u6237\u5934\u50CF"><div><strong>' + yyEscape(user.username) + '</strong><p class="yy-hint">' + yyEscape(user.email || "\u5C1A\u672A\u7ED1\u5B9A\u90AE\u7BB1") + '</p></div></div><label>\u6743\u9650<select id="sub-role-' + yyEscape(id) + '"><option value="member"' + (user.role !== "admin" ? " selected" : "") + '>\u666E\u901A / \u8BA2\u9605\u7528\u6237</option><option value="admin"' + (user.role === "admin" ? " selected" : "") + '>\u7BA1\u7406\u5458</option></select></label><label>\u8BA2\u9605\u5230\u671F\u65E5\u671F \xB7 \u5F53\u65E5\u7ED3\u675F<input type="date" id="sub-date-' + yyEscape(id) + '" value="' + (user.subscription_until ? new Date(user.subscription_until).toLocaleDateString("sv-SE") : "") + '"></label><label class="yy-inline-check"><input type="checkbox" id="sub-node-' + yyEscape(id) + '"' + ((user.assigned_devices || []).includes("linux-server") ? " checked" : "") + '>\u5206\u914D Linux \u4E91\u670D\u52A1\u5668</label><p class="yy-hint">\u5206\u914D\u5171\u4EAB Linux \u8282\u70B9\u4F1A\u6388\u4E88\u8BE5\u5DE5\u5177\u7BB1\u7684\u5B8C\u6574\u64CD\u4F5C\u6743\u9650\uFF1B\u9700\u8981\u9694\u79BB\u6570\u636E\u7684\u7528\u6237\u8BF7\u4F7F\u7528\u81EA\u5DF1\u7684\u6267\u884C\u8282\u70B9\u3002</p><div class="yy-actions"><button class="btn" onclick="yySubscriptionSave(' + arg + ',false,this)">\u4FDD\u5B58 / \u7EED\u671F</button><button class="btn-s" onclick="yySubscriptionSave(' + arg + ',true,this)">\u64A4\u9500\u6388\u6743</button><button class="btn-s" onclick="yyResetPasswordPage(' + arg + ')">\u91CD\u7F6E\u5BC6\u7801</button></div><p id="yy-sub-detail-result" role="status"></p></section>');
}
async function yySubscriptionSave(id, revoke, button) {
  const date = document.getElementById("sub-date-" + id).value, out = document.getElementById("yy-sub-detail-result");
  if (button) button.disabled = true;
  if (out) out.textContent = "\u6B63\u5728\u4FDD\u5B58\u2026";
  try {
    const body = { user_id: id, role: revoke ? "member" : document.getElementById("sub-role-" + id).value, subscription_until: revoke || !date ? 0 : (/* @__PURE__ */ new Date(date + "T23:59:59")).getTime(), assigned_devices: !revoke && document.getElementById("sub-node-" + id).checked ? ["linux-server"] : [] };
    const r = await yyIdentityRequest("entitlement", body);
    if (!r.ok) throw Error(r.error || "\u4FDD\u5B58\u5931\u8D25");
    const user = yySubscriptionUsers.find((u) => u.id === id);
    if (user) Object.assign(user, { role: body.role, subscription_until: body.subscription_until, assigned_devices: body.assigned_devices, cloud_allowed: body.role === "admin" || body.subscription_until > Date.now() });
    if (out == null ? void 0 : out.isConnected) {
      out.textContent = revoke ? "\u6388\u6743\u5DF2\u64A4\u9500" : "\u8BA2\u9605\u5DF2\u4FDD\u5B58";
      document.getElementById("sub-role-" + id).value = body.role;
      document.getElementById("sub-date-" + id).value = revoke ? "" : date;
      document.getElementById("sub-node-" + id).checked = body.assigned_devices.includes("linux-server");
    }
    T(revoke ? "\u6388\u6743\u5DF2\u64A4\u9500" : "\u8BA2\u9605\u5DF2\u4FDD\u5B58", "o");
  } catch (error) {
    if (out == null ? void 0 : out.isConnected) out.textContent = error.message;
    T(error.message, "e");
  } finally {
    if (button == null ? void 0 : button.isConnected) button.disabled = false;
  }
}
var yySubscriptionSettings = Rcfg;
Rcfg = async function() {
  var yySettingsStillLive = yySettingsViewGuard();
  await yySubscriptionSettings();
  if (!yySettingsStillLive()) return;
  if (yyIsCloud && yyPlatformUser && yyPlatformUser.role === "admin") {
    var host = document.querySelector("#tab-settings .content-inner");
    if (host) {
      var box = document.createElement("section");
      box.className = "card";
      box.innerHTML = yyCardHead("\u8BA2\u9605\u4E0E\u7528\u6237\u6743\u9650", "shield") + '<button class="btn" onclick="yySubscriptionsPage()">\u7BA1\u7406\u8BA2\u9605\u7528\u6237</button>';
      host.prepend(box);
    }
  }
};
(async function() {
  try {
    var r = await yyIdentityRequest("me");
    if (r.ok && (yyIsCloud || r.authenticated)) {
      yyPlatformUser = r.user;
      document.dispatchEvent(new Event("yy-identity-ready"));
      if (yyIsCloud && !r.user.cloud_allowed) yySubscriptionGate(r.user);
      return;
    }
    if (yyIsCloud && sessionStorage.getItem("yy-cloud-key")) {
      var result = await yyRootFetch("/api/cloud/status");
      if (result.ok) return;
    }
  } catch (e) {
  }
  yyShowIdentity();
})();
async function yyResetPasswordPage(id) {
  var result = await yyIdentityRequest("users");
  if (!result.ok) {
    T(result.error, "e");
    return;
  }
  var user = (result.users || []).find(function(u) {
    return u.id === id;
  });
  if (!user) {
    T("\u8D26\u53F7\u4E0D\u5B58\u5728", "e");
    return;
  }
  var bytes = crypto.getRandomValues(new Uint8Array(24)), password = Array.from(bytes, function(b) {
    return b.toString(16).padStart(2, "0");
  }).join("");
  delete subPageCache["\u91CD\u7F6E\u8D26\u53F7\u5BC6\u7801"];
  openSubPage("\u91CD\u7F6E\u8D26\u53F7\u5BC6\u7801", yyIntro("\u91CD\u7F6E\u8D26\u53F7\u5BC6\u7801", "\u4E3A " + user.username + " \u8BBE\u7F6E\u65B0\u5BC6\u7801\u3002\u4FDD\u5B58\u540E\u8BE5\u8D26\u53F7\u7684\u7F51\u9875\u767B\u5F55\u4F1A\u8BDD\u9000\u51FA\uFF0C\u6E38\u620F\u6570\u636E\u4FDD\u7559\u3002") + '<section class="card"><label>\u65B0\u5BC6\u7801<input id="yy-reset-new" type="password" autocomplete="new-password" minlength="10" maxlength="128" value="' + password + `"></label><label><input type="checkbox" onchange="document.getElementById('yy-reset-new').type=this.checked?'text':'password'">\u663E\u793A\u521A\u8BBE\u7F6E\u7684\u5BC6\u7801</label><div class="yy-actions"><button class="btn-s" onclick="yyCopyField('yy-reset-new')">\u590D\u5236\u65B0\u5BC6\u7801</button><button class="btn" id="yy-reset-save">\u4FDD\u5B58\u65B0\u5BC6\u7801</button></div><p id="yy-reset-result" role="status"></p></section>`);
  document.getElementById("yy-reset-save").onclick = async function() {
    var next = document.getElementById("yy-reset-new").value, r = await yyIdentityRequest("reset-password", { user_id: id, new_password: next });
    if (!r.ok) {
      T(r.error, "e");
      return;
    }
    document.getElementById("yy-reset-result").textContent = "\u5BC6\u7801\u5DF2\u91CD\u7F6E\uFF0C\u8BF7\u4FDD\u5B58\u6216\u590D\u5236\u65B0\u5BC6\u7801\u540E\u4EA4\u7ED9\u8D26\u53F7\u672C\u4EBA\u3002";
    document.getElementById("yy-reset-save").disabled = true;
    T("\u8D26\u53F7\u5BC6\u7801\u5DF2\u91CD\u7F6E", "o");
  };
}
function yyPlatformDeletePage() {
  var old = document.getElementById("yy-identity-gate");
  if (old) old.remove();
  var panel = document.createElement("div");
  panel.id = "yy-identity-gate";
  panel.innerHTML = '<form class="yy-identity-card" onsubmit="yyPlatformDelete(event)"><h1>\u6CE8\u9500\u5E73\u53F0\u8D26\u53F7</h1><p>\u6CE8\u9500\u540E\u8D26\u53F7\u540D\u548C QQ \u90AE\u7BB1\u53EF\u91CD\u65B0\u6CE8\u518C\uFF0C\u6240\u6709\u5E73\u53F0\u767B\u5F55\u3001\u4E91\u7AEF\u8BBE\u5907\u51ED\u8BC1\u53CA MCP \u6388\u6743\u4F1A\u64A4\u9500\u3002\u8BA2\u9605\u4E0D\u80FD\u8F6C\u5230\u65B0\u8D26\u53F7\u3002\u672C\u673A\u63D2\u4EF6\u548C\u6E38\u620F\u6587\u4EF6\u4FDD\u7559\uFF1B\u4E91\u7AEF\u8D44\u6599\u4E0D\u518D\u80FD\u901A\u8FC7\u65E7\u8D26\u53F7\u8BBF\u95EE\u3002</p><p>\u9879\u76EE\u6240\u6709\u8005\u9700\u8981\u5148\u5B8C\u6210\u6240\u6709\u6743\u4EA4\u63A5\uFF0C\u4E0D\u80FD\u5728\u8FD9\u91CC\u76F4\u63A5\u6CE8\u9500\u3002</p><label>\u5F53\u524D\u8D26\u53F7\u5BC6\u7801<input id="yy-delete-password" type="password" required autocomplete="current-password"></label><label><input id="yy-delete-confirm" type="checkbox" required>\u6211\u786E\u8BA4\u6CE8\u9500\u5F53\u524D\u5E73\u53F0\u8D26\u53F7</label><div class="yy-actions"><button type="button" class="btn-s" onclick="yyPlatformDeleteBack()">\u8FD4\u56DE</button><button type="submit" class="btn-s" id="yy-delete-submit">\u786E\u8BA4\u6CE8\u9500</button></div><p id="yy-delete-result" role="status"></p></form>';
  document.body.append(panel);
}
function yyPlatformDeleteBack() {
  var panel = document.getElementById("yy-identity-gate");
  if (panel) panel.remove();
  if (yyPlatformUser && !yyPlatformUser.cloud_allowed && yyIsCloud) {
    yySubscriptionGate(yyPlatformUser);
    return;
  }
  yyPlatformPage();
}
async function yyPlatformDelete(event) {
  event.preventDefault();
  var button = document.getElementById("yy-delete-submit"), out = document.getElementById("yy-delete-result");
  button.disabled = true;
  out.textContent = "\u6B63\u5728\u6CE8\u9500\u8D26\u53F7\u2026";
  try {
    var r = await yyIdentityRequest("delete", { password: document.getElementById("yy-delete-password").value });
    if (!r.ok) throw Error(r.error || "\u6CE8\u9500\u672A\u6210\u529F");
    sessionStorage.removeItem("yy-cloud-key");
    sessionStorage.removeItem("yy-user-control");
    yyPlatformUser = null;
    window.yyCloudBlocked = false;
    T(r.message || "\u5E73\u53F0\u8D26\u53F7\u5DF2\u6CE8\u9500", "o");
    if (yyIsCloud) {
      location.reload();
      return;
    }
    yyShowIdentity();
    document.getElementById("yy-login-result").textContent = "\u8D26\u53F7\u5DF2\u6CE8\u9500\uFF0C\u53EF\u4F7F\u7528 QQ \u90AE\u7BB1\u9A8C\u8BC1\u7801\u91CD\u65B0\u6CE8\u518C\u3002";
  } catch (e) {
    out.textContent = e.message;
    button.disabled = false;
  }
}
(window.__subPageRestoreHooks = window.__subPageRestoreHooks || []).push({ key: "\u8BA2\u9605\u7528\u6237\u7BA1\u7406", fn: function() {
  yyRenderSubscriptions();
} });
