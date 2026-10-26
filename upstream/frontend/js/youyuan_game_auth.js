var yyCloudGameAccounts = [], yyIssuedGameCredential = null;
function yyOpenAccountRegistration() {
  const url = new URL("https://auth.youyuanqi.dpdns.org/");
  if (window.android && android.openVerification) {
    url.searchParams.set("return_to", "https://toolbox.youyuanqi.dpdns.org/?return_to_app=1");
    android.openVerification(url.href);
  } else {
    const here = new URL(location.href);
    here.username = "";
    here.password = "";
    here.search = "";
    here.hash = "";
    const device = new URL(location.href).searchParams.get("device");
    if (device) here.searchParams.set("device", device);
    url.searchParams.set("return_to", here.href);
    window.open(url.href, "_blank", "noopener");
  }
}
async function yyGameAuthAPI(method, body) {
  // 超时从 80 秒收到 10 秒：认证服务慢的时候，账号页原来会干等 80 秒什么都不显示
  // （用户反馈「点开互通/账号会特别慢」）。10 秒拿不到就先用本地账号，
  // 云端账号下次刷新再补，绝不让用户对着空页面等。
  const control = new AbortController(), timer = setTimeout(() => control.abort(), 1e4);
  try {
    const options = { method, headers: { "Content-Type": "application/json" }, body: body ? JSON.stringify(body) : void 0, signal: control.signal };
    let response = await yyRootFetch(yyIsCloud ? "/api/game-auth/accounts" : "/api/youyuan/game-auth", options);
    if (yyIsCloud && response.status === 404) {
      const failure = await response.clone().json().catch(() => null);
      if ((failure == null ? void 0 : failure.error) === "not_found" && (failure == null ? void 0 : failure.path) === "/api/game-auth/accounts") response = await yyRootFetch("/api/youyuan/game-auth", options);
    }
    return await response.json();
  } catch (e) {
    return { ok: false, error: e.name === "AbortError" ? "\u8BA4\u8BC1\u8BF7\u6C42\u8D85\u65F6\u3002\u539F\u4EFB\u52A1\u5DF2\u4FDD\u7559\uFF0C\u8BF7\u5237\u65B0\u5F85\u5904\u7406\u5217\u8868\u540E\u7EE7\u7EED\u3002" : e.message };
  } finally {
    clearTimeout(timer);
  }
}
async function yyGameAuthPage() {
  delete subPageCache["\u6E38\u56ED\u51ED\u8BC1"];
  openSubPage("\u6E38\u56ED\u51ED\u8BC1", yyIntro("\u6E38\u56ED\u6E38\u620F\u8BA4\u8BC1", "\u76F4\u8FDE\u7F51\u6613\uFF1B\u6E38\u56ED\u6E38\u620F\u4EE4\u724C\u4E0E\u5E73\u53F0\u63A7\u5236\u5BC6\u94A5\u5404\u81EA\u72EC\u7ACB\u3002") + `<div class="yy-cred-page"><section class="card"><h3>\u6DFB\u52A0\u6E38\u56ED\u51ED\u8BC1</h3><div class="yy-actions"><button class="btn-s" onclick="yyOpenAccountRegistration()">\u6253\u5F00\u72EC\u7ACB\u8D26\u53F7\u8BA4\u8BC1\u7F51\u7AD9</button><button class="btn-s" onclick="yyOpen4399Registration()">\u6253\u5F004399\u5B98\u65B9\u6CE8\u518C</button></div><p class="yy-hint">\u8D26\u53F7\u6CE8\u518C\u4E0E\u6E38\u620F\u8D26\u53F7\u63A5\u5165\u662F\u4E24\u6B65\uFF1A\u5E73\u53F0QQ\u90AE\u7BB1\u6CE8\u518C\u3001\u9A8C\u8BC1\u7801\u548C\u5BC6\u7801\u627E\u56DE\u5728\u72EC\u7ACB\u8BA4\u8BC1\u7F51\u7AD9\uFF1B4399\u6E20\u9053\u5F53\u524D\u6682\u4E0D\u53EF\u7528\uFF0C\u5DF2\u4FDD\u5B58\u7684\u8D26\u53F7\u4E0D\u4F1A\u88AB\u5220\u9664\u3002</p><label>\u64CD\u4F5C</label><select id="yy-ga-mode" onchange="yyGameAuthMode()"><option value="login_email">\u7F51\u6613\u90AE\u7BB1\u767B\u5F55</option><option value="register_guest">\u521B\u5EFA\u6E38\u5BA2\u8D26\u53F7\u5E76\u8BBE\u7F6E\u6635\u79F0</option><option value="login_4399" disabled>4399\u6E20\u9053\u767B\u5F55\uFF08\u5F53\u524D\u6682\u4E0D\u53EF\u7528\uFF09</option><option value="import_cookie">\u5BFC\u5165\u5DF2\u6709\u7F51\u6613\u767B\u5F55Cookie</option></select><label>\u663E\u793A\u540D\u79F0</label><input id="yy-ga-name" maxlength="48" placeholder="\u81EA\u5DF1\u7684\u673A\u5668\u4EBA\u8D26\u53F7"><div id="yy-ga-email-fields"><label>\u7F51\u6613\u90AE\u7BB1</label><input id="yy-ga-email" type="email" autocomplete="username"><label>\u7F51\u6613\u5BC6\u7801</label><input id="yy-ga-password" type="password" autocomplete="off"></div><div id="yy-ga-guest-fields" hidden><label>\u6E38\u620F\u6635\u79F0</label><input id="yy-ga-nickname" maxlength="20" placeholder="2\u201320\u4E2A\u5B57\u7B26"></div><div id="yy-ga-4399-fields" hidden><label>4399\u7528\u6237\u540D</label><input id="yy-ga-4399-user" autocomplete="username"><label>4399\u5BC6\u7801</label><input id="yy-ga-4399-password" type="password" autocomplete="off"><p class="yy-hint">\u5B98\u65B9\u6CE8\u518C\u540E\u5728\u8FD9\u91CC\u63A5\u5165\uFF0C\u4E0D\u9700\u8981\u57284399\u6E38\u620F\u5217\u8868\u91CC\u5BFB\u627E\u6CE8\u518C\u6309\u94AE\u3002</p></div><div id="yy-ga-cookie-fields" hidden><label>\u7F51\u6613\u767B\u5F55Cookie</label><textarea id="yy-ga-cookie" autocomplete="off"></textarea></div><p class="yy-hint">\u4E0D\u4F1A\u4FDD\u5B58\u7F51\u6613\u660E\u6587\u5BC6\u7801\u3002\u767B\u5F55\u72B6\u6001\u52A0\u5BC6\u5B58\u50A8\u5728\u4F60\u7684\u4E91\u7AEF\u8D44\u6599\u5E93\uFF1B\u6BCF25\u5206\u949F\u5C1D\u8BD5\u5237\u65B0\uFF0C\u5931\u8D25\u4F1A\u663E\u793A\u72B6\u6001\u5E76\u5EF6\u8FDF\u91CD\u8BD5\u3002\u5237\u65B0\u51ED\u636E\u4E0E\u673A\u5668\u4EBA\u5728\u7EBF\u4FDD\u6D3B\u662F\u4E24\u4E2A\u6B65\u9AA4\u3002\u7F51\u6613\u98CE\u63A7\u3001\u5B9E\u540D\u6216\u9A8C\u8BC1\u7801\u5FC5\u987B\u7531\u8D26\u53F7\u672C\u4EBA\u5B8C\u6210\u3002</p><button id="yy-ga-create" class="btn" onclick="yyGameAuthCreate()">\u6DFB\u52A0\u5E76\u7B7E\u53D1\u6E38\u620F\u4EE4\u724C</button><pre id="yy-ga-result" class="yy-readout"></pre><div id="yy-ga-issued" hidden><label>\u672C\u6B21\u6E38\u620F\u4EE4\u724C</label><input id="yy-ga-token" readonly type="password"><div class="yy-actions"><button class="btn-s" onclick="yyGameAuthCopy()">\u590D\u5236\u4EE4\u724C</button><button class="btn-s" onclick="yyRevealField('yy-ga-token')">\u663E\u793A / \u9690\u85CF</button></div><label>\u76EE\u6807\u670D\u52A1\u5668\u53F7 / \u5206\u4EAB\u94FE\u63A5</label><input id="yy-ga-server" placeholder="\u670D\u52A1\u5668\u53F7\u6216\u7F51\u6613\u5206\u4EAB\u94FE\u63A5"><button class="btn" onclick="yyGameAuthUse()">\u4FDD\u5B58\u5230\u672C\u673A\u8D26\u53F7\u5DE5\u4F5C\u7A7A\u95F4</button></div></section><section class="card"><div class="yy-cred-list-head"><h3>\u6211\u7684\u6E38\u56ED\u51ED\u8BC1</h3><button class="btn-s" onclick="yyGameAuthRefresh()">\u5237\u65B0\u5217\u8868</button></div><div id="yy-ga-pending"></div><div id="yy-ga-list"></div></section></div>`);
  await yyGameAuthRefresh();
}
function yyGameAuthMode() {
  var mode = document.getElementById("yy-ga-mode").value;
  document.getElementById("yy-ga-email-fields").hidden = mode !== "login_email";
  document.getElementById("yy-ga-guest-fields").hidden = !["register_guest", "login_4399"].includes(mode);
  document.getElementById("yy-ga-4399-fields").hidden = mode !== "login_4399";
  document.getElementById("yy-ga-cookie-fields").hidden = mode !== "import_cookie";
}
function yyGameAuthShowCredential(result, row) {
  yyIssuedGameCredential = { id: row.id, name: row.name, token: result.login_token };
  document.getElementById("yy-ga-token").value = result.login_token;
  document.getElementById("yy-ga-issued").hidden = false;
}
async function yyGameAuthCreate() {
  var b = document.getElementById("yy-ga-create"), out = document.getElementById("yy-ga-result");
  b.disabled = true;
  out.textContent = "\u6B63\u5728\u76F4\u8FDE\u7F51\u6613\u5E76\u521B\u5EFA\u81EA\u5DF1\u7684\u4EE4\u724C\u2026";
  try {
    var body = { action: document.getElementById("yy-ga-mode").value, name: document.getElementById("yy-ga-name").value, email: document.getElementById("yy-ga-email").value, password: document.getElementById("yy-ga-password").value, nickname: document.getElementById("yy-ga-nickname").value, cookie: document.getElementById("yy-ga-cookie").value, username: document.getElementById("yy-ga-4399-user").value };
    if (body.action === "login_4399") body.password = document.getElementById("yy-ga-4399-password").value;
    var r = await yyGameAuthAPI("POST", body);
    if (!r.ok) {
      out.textContent = r.error || "\u8BA4\u8BC1\u5931\u8D25";
      if (r.verification_kind === "captcha_4399") {
        yy4399Verification(r);
        return;
      }
      if (r.pending_id) {
        yyGameAuthPending = r;
      }
      if (r.verification_required && r.verification_url) {
        var link = document.createElement("a");
        link.href = r.verification_url;
        link.target = "_blank";
        link.rel = "noopener noreferrer";
        link.textContent = "\u6253\u5F00\u7F51\u6613\u672C\u4EBA\u9A8C\u8BC1";
        out.append(link);
      }
      return;
    }
    for (const id of ["yy-ga-password", "yy-ga-4399-password", "yy-ga-cookie"]) {
      const f = document.getElementById(id);
      if (f) f.value = "";
    }
    out.textContent = "\u5DF2\u521B\u5EFA\u81EA\u5DF1\u7684\u6E38\u620F\u4EE4\u724C\uFF0C\u8D26\u53F7\uFF1A" + (r.account.nickname || r.account.game_id);
    yyGameAuthShowCredential(r, r.account);
    await yyGameAuthRefresh();
    await yyAutoHostCredential(r);
  } catch (e) {
    out.textContent = e.message;
  } finally {
    b.disabled = false;
  }
}
function yyGameCredentialRow(row) {
  // 头像取用顺序从前写在界面上，属于开发说明，挪到这里：
  // 账号资料上传的头像（data:）→ 网易官方头像 → 本机按玩家名裁的真实皮肤头 → 等距三面占位。
  // 取不到图时界面不再解释来源，只在图片加载失败时补一个 title。
  const panel = document.createElement("details");
  panel.className = "yy-credential-row";
  const summary = document.createElement("summary"), avatar = document.createElement("span"), identity = document.createElement("span"), name = document.createElement("strong"), meta = document.createElement("small"), status = document.createElement("span"), chevron = document.createElement("span");
  summary.className = "yy-cred-row-head";
  avatar.className = "yy-cred-avatar";
  avatar.innerHTML = typeof yyAccountAvatarHTML === "function" ? yyAccountAvatarHTML(row) : '<span class="yy-cred-avatar-ph" aria-hidden="true">\u2014</span>';
  avatar.addEventListener("error", () => {
    avatar.title = "\u4F7F\u7528\u9ED8\u8BA4\u5934\u50CF";
  }, true);
  identity.className = "yy-cred-identity";
  name.className = "yy-cred-name";
  name.textContent = row.nickname || row.name;
  meta.className = "yy-cred-meta";
  meta.textContent = "UID " + row.game_id;
  identity.append(name, meta);
  status.className = "yy-cred-state";
  status.dataset.state = row.status === "ready" ? "ok" : "warn";
  status.textContent = row.status === "ready" ? "\u51ED\u636E\u53EF\u7528" : "\u9700\u8981\u5904\u7406";
  chevron.className = "yy-row-chevron";
  chevron.setAttribute("aria-hidden", "true");
  chevron.textContent = "\u203A";
  summary.append(avatar, identity, status, chevron);
  panel.append(summary);
  return panel;
}
async function yyGameAuthRefresh() {
  const r = await yyGameAuthAPI("GET"), host = document.getElementById("yy-ga-list");
  if (!host) return;
  host.replaceChildren();
  if (!r.ok) {
    host.textContent = r.error;
    return;
  }
  yyRenderAuthPending(r.pending || [], r.batches || []);
  yyCloudGameAccounts = r.accounts || [];
  for (const row of yyCloudGameAccounts) {
    const section = yyGameCredentialRow(row), actions = document.createElement("div");
    actions.className = "yy-cred-actions";
    for (const [label, action] of [["\u4F7F\u7528\u4EE4\u724C", "token"], ["\u5237\u65B0\u51ED\u636E", "refresh"], ["\u8BBE\u7F6E\u6635\u79F0", "nickname"], [row.keep_alive ? "\u505C\u7528\u81EA\u52A8\u5237\u65B0" : "\u542F\u7528\u81EA\u52A8\u5237\u65B0", "configure"], ["\u64A4\u9500\u51ED\u636E", "delete"]]) {
      const button = document.createElement("button");
      button.className = action === "delete" ? "btn-s yy-cred-danger" : "btn-s";
      button.textContent = label;
      button.onclick = () => yyGameAuthAction(row, action);
      actions.append(button);
    }
    const skin = document.createElement("button");
    skin.className = "btn-s";
    skin.textContent = "\u76AE\u80A4\u9884\u89C8 / \u6DFB\u52A0\u7EC4\u4EF6\u7801";
    skin.onclick = () => yyGameAccountSkin(row);
    actions.append(skin);
    section.append(actions);
    if (row.last_error) {
      const error = document.createElement("p");
      error.className = "yy-cred-error";
      error.textContent = row.last_error;
      section.append(error);
    }
    host.append(section);
  }
  if (!yyCloudGameAccounts.length) host.textContent = "\u5C1A\u672A\u6DFB\u52A0\u6E38\u56ED\u51ED\u8BC1";
}
var yyGameSkinCleanup = null;
async function yyGameAccountSkin(row) {
  var _a;
  if (yyGameSkinCleanup) yyGameSkinCleanup();
  delete subPageCache["\u8D26\u53F7\u76AE\u80A4"];
  openSubPage("\u8D26\u53F7\u76AE\u80A4", yyIntro("\u8D26\u53F7\u76AE\u80A4", row.nickname || row.name) + '<section class="card"><p id="yy-ga-skin-status" role="status">\u6B63\u5728\u8BFB\u53D6\u5B98\u65B9\u76AE\u80A4\u2026</p><div id="yy-ga-skin-stage" style="height:340px;border-radius:16px;background:var(--phx-bg-secondary)"></div><div class="yy-actions"><button class="btn-s" id="yy-ga-skin-reset">\u590D\u4F4D\u89C6\u89D2</button><button class="btn-s" id="yy-ga-skin-walk">\u8D70\u8DEF\u9884\u89C8</button></div><label for="yy-ga-skin-component">\u5DF2\u62E5\u6709\u7684\u76AE\u80A4\u7EC4\u4EF6\u7801</label><input id="yy-ga-skin-component" inputmode="numeric"><button class="btn" id="yy-ga-skin-apply">\u5E94\u7528\u76AE\u80A4</button><p class="yy-hint">\u62D6\u52A8\u6A21\u578B\u8F6C\u8EAB\u3002\u4EC5\u9009\u62E9\u672C\u8D26\u53F7\u5DF2\u62E5\u6709\u7684\u7EC4\u4EF6\uFF0C\u4E0D\u81EA\u52A8\u8D2D\u4E70\uFF1B\u91CD\u65B0\u8FDE\u63A5\u6E38\u620F\u540E\u5E94\u7528\u3002</p></section>');
  const stage = document.getElementById("yy-ga-skin-stage"), status = document.getElementById("yy-ga-skin-status");
  document.getElementById("yy-ga-skin-component").value = row.skin_item_id || "";
  document.getElementById("yy-ga-skin-apply").onclick = async function() {
    this.disabled = true;
    try {
      const id = document.getElementById("yy-ga-skin-component").value.trim();
      if (!/^\d{1,32}$/.test(id)) throw Error("\u8BF7\u8F93\u5165\u6570\u5B57\u7EC4\u4EF6\u7801");
      const r = await yyGameAuthAPI("POST", { action: "skin", id: row.id, item_id: id });
      if (!r.ok) throw Error(r.error);
      status.textContent = "\u76AE\u80A4\u8BBE\u7F6E\u6210\u529F\uFF0C\u91CD\u65B0\u8FDE\u63A5\u6E38\u620F\u751F\u6548\u3002";
    } catch (e) {
      status.textContent = e.message;
    } finally {
      this.disabled = false;
    }
  };
  const result = await yyGameAuthAPI("POST", { action: "skin_preview", id: row.id });
  if (!stage.isConnected) return;
  if (!result.ok) {
    status.textContent = result.error || "\u672A\u80FD\u8BFB\u53D6\u76AE\u80A4\uFF0C\u4ECD\u53EF\u586B\u5199\u5DF2\u6709\u7EC4\u4EF6\u7801";
    return;
  }
  const viewer = new YYSkinViewer(stage, result.texture, (_a = result.skin_info) == null ? void 0 : _a.SkinIsSlim);
  status.textContent = result.warning || "\u5B98\u65B9\u76AE\u80A4\u9884\u89C8";
  let walk = false;
  document.getElementById("yy-ga-skin-reset").onclick = () => viewer.reset();
  document.getElementById("yy-ga-skin-walk").onclick = () => viewer.walk(walk = !walk);
  const observer = new MutationObserver(() => {
    if (!stage.isConnected) {
      viewer.dispose();
      observer.disconnect();
      yyGameSkinCleanup = null;
    }
  });
  observer.observe(document.body, { childList: true, subtree: true });
  yyGameSkinCleanup = () => {
    viewer.dispose();
    observer.disconnect();
    yyGameSkinCleanup = null;
  };
}
async function yyGameAuthAction(row, action) {
  if (action === "rooms") {
    await yyGameAuthRooms(row);
    return;
  }
  if (action === "servers") {
    await yyGameAuthServers(row);
    return;
  }
  var body = { id: row.id, action };
  if (action === "delete" && !confirm("\u64A4\u9500\u8BE5\u6E38\u620F\u51ED\u636E\u548C\u4EE4\u724C\uFF1F\u5DF2\u8FDE\u63A5\u7684\u6E38\u620F\u4F1A\u8BDD\u4E0D\u4F1A\u88AB\u5F3A\u5236\u8E22\u51FA\u3002")) return;
  if (action === "nickname") {
    body.nickname = prompt("\u8BBE\u7F6E\u8FD9\u4E2A\u6E38\u620F\u8D26\u53F7\u7684\u6635\u79F0", row.nickname || "");
    if (body.nickname === null) return;
  }
  if (action === "configure") {
    body.keep_alive = !row.keep_alive;
  }
  if (action === "configure_sid") {
    body.action = "configure";
    body.server_id = prompt("\u8D26\u53F7\u52A0\u5165\u591A\u4E2A\u5C71\u5934\u4E14\u65E0\u6CD5\u552F\u4E00\u5339\u914D\u65F6\uFF0C\u586B\u5199\u76EE\u6807\u5C71\u5934SID\uFF1B\u53EF\u7559\u7A7A\u81EA\u52A8\u5339\u914D", row.server_id || "");
    if (body.server_id === null) return;
  }
  var r = await yyGameAuthAPI("POST", body);
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  if (action === "token") {
    yyGameAuthShowCredential(r, row);
    document.getElementById("yy-ga-issued").scrollIntoView({ behavior: "smooth", block: "center" });
  } else await yyGameAuthRefresh();
}
async function yyGameAuthCopy() {
  try {
    await navigator.clipboard.writeText(document.getElementById("yy-ga-token").value);
    T("\u5DF2\u590D\u5236\u6E38\u620F\u4EE4\u724C", "o");
  } catch {
    T("\u8BF7\u663E\u793A\u4EE4\u724C\u540E\u957F\u6309\u590D\u5236", "e");
  }
}
async function yyGameAuthUse() {
  if (!yyIssuedGameCredential) return;
  var server = document.getElementById("yy-ga-server").value.trim();
  var r = await yyRootAPI("POST", "/api/youyuan/accounts", { action: "save", account: { id: "yy_" + yyIssuedGameCredential.id, name: yyIssuedGameCredential.name, token: yyIssuedGameCredential.token, auth_url: "https://auth.youyuanqi.dpdns.org", server, keep_alive: true } });
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  yyIssuedGameCredential = null;
  yyNavigate("accounts");
  T("\u8D26\u53F7\u5DF2\u4FDD\u5B58\uFF0C\u70B9\u51FB\u8FDE\u63A5\u8FDB\u5165\u81EA\u5DF1\u7684\u670D\u52A1\u5668", "o");
}
CFG_SERVERS.splice(2, 0, { url: "https://auth.youyuanqi.dpdns.org", label: "\u6E38\u56ED\u72EC\u7ACB\u8BA4\u8BC1 \xB7 \u81EA\u6709\u6E38\u620F\u4EE4\u724C" });
var yyGameAuthSettingsBase = Rcfg;
Rcfg = async function() {
  var yySettingsStillLive = yySettingsViewGuard();
  await yyGameAuthSettingsBase();
  if (!yySettingsStillLive()) return;
  var anchor = document.getElementById("cfg-conn-card");
  if (!anchor) return;
  var button = document.createElement("button");
  button.className = "btn-s";
  button.textContent = "\u7BA1\u7406\u6E38\u56ED\u51ED\u8BC1 / \u4E00\u952E\u6E38\u5BA2\u6CE8\u518C";
  button.onclick = yyGameAuthPage;
  anchor.append(button);
};
var yyGameAuthEditBase = yyEditAccount;
yyEditAccount = function(index) {
  yyGameAuthEditBase(index);
  var select = document.getElementById("yy-ac-auth");
  if (select) {
    var option = document.createElement("option");
    option.value = "https://auth.youyuanqi.dpdns.org";
    option.textContent = "\u6E38\u56ED\u72EC\u7ACB\u8BA4\u8BC1";
    select.append(option);
    if (typeof index !== "number") select.value = option.value;
    else if (yyEditingAccount.auth_url === option.value) select.value = option.value;
  }
};
async function yyGameAuthServers(row) {
  var _a;
  var r = await yyGameAuthAPI("POST", { action: "servers", id: row.id });
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  if (!((_a = r.servers) == null ? void 0 : _a.length)) {
    T("\u5F53\u524D\u8D26\u53F7\u5C1A\u672A\u52A0\u5165\u5C71\u5934\uFF0C\u5148\u4F7F\u7528\u9080\u8BF7\u94FE\u63A5\u8FDE\u63A5", "e");
    return;
  }
  showPicker({ title: "\u9009\u62E9 " + row.name + " \u7684\u5C71\u5934", items: r.servers.map((x) => ({ value: yyEscape(x.id), label: (x.index || "") + ". " + yyEscape(x.name || "\u5C71\u5934\u670D\u52A1\u5668") })), selected: row.server_id || "", onConfirm: async function(id) {
    var v = await yyGameAuthAPI("POST", { action: "configure", id: row.id, server_id: id });
    if (!v.ok) {
      T(v.error, "e");
      return;
    }
    document.getElementById("yy-ga-server").value = "DomainSID:" + id;
    T("\u5DF2\u9009\u62E9\u5C71\u5934\uFF0C\u4F7F\u7528\u4EE4\u724C\u540E\u8FDE\u63A5", "s");
  } });
}
async function yyRoomSearchPage() {
  delete subPageCache["\u641C\u7D22\u8054\u673A\u623F\u95F4"];
  openSubPage("\u641C\u7D22\u8054\u673A\u623F\u95F4", yyIntro("\u641C\u7D22\u8054\u673A\u623F\u95F4", "\u9009\u62E9\u5DF2\u767B\u5F55\u7684\u6E38\u620F\u8D26\u53F7\uFF0C\u4EE5\u771F\u5B9E room_id \u52A0\u5165\u623F\u95F4\u3002") + '<section class="card"><label>\u7528\u4E8E\u67E5\u8BE2\u623F\u95F4\u7684\u6E38\u620F\u8D26\u53F7</label><select id="yy-room-account"></select><label>\u623F\u95F4ID\u6216\u540D\u79F0</label><input id="yy-room-query" placeholder="\u8F93\u5165\u5173\u952E\u8BCD\uFF0C\u4E5F\u53EF\u7559\u7A7A"><div class="yy-actions"><button class="btn" id="yy-room-search">\u641C\u7D22\u623F\u95F4</button><button class="btn-s" id="yy-room-return">\u8FD4\u56DE\u8FDE\u63A5\u8BBE\u7F6E</button></div><p id="yy-room-status" role="status"></p><div id="yy-room-search-results"></div></section>');
  var r = await yyGameAuthAPI("GET"), select = document.getElementById("yy-room-account"), status = document.getElementById("yy-room-status");
  if (!select) return;
  document.getElementById("yy-room-return").onclick = () => {
    closeSubPage();
    Rcfg();
  };
  if (!r.ok) {
    status.textContent = r.error || "\u6E38\u620F\u8D26\u53F7\u5217\u8868\u8BFB\u53D6\u5931\u8D25";
    return;
  }
  var accounts = r.accounts || [];
  accounts.forEach((a) => select.add(new Option(a.nickname || a.name || a.game_id, a.id)));
  if (!accounts.length) {
    status.textContent = "\u8FD8\u6CA1\u6709\u53EF\u7528\u4E8E\u67E5\u8BE2\u7684\u6E38\u620F\u767B\u5F55\u51ED\u636E\u3002\u53EF\u4EE5\u5148\u6DFB\u52A0\u81EA\u5DF1\u7684\u6E38\u620F\u8D26\u53F7\uFF0C\u518D\u8FD4\u56DE\u641C\u7D22\u3002";
    var b = document.createElement("button");
    b.className = "btn-s";
    b.textContent = "\u6DFB\u52A0\u6E38\u620F\u8D26\u53F7";
    b.onclick = yyGameAuthPage;
    status.after(b);
    return;
  }
  async function search(offset) {
    var button = document.getElementById("yy-room-search");
    button.disabled = true;
    status.textContent = "\u6B63\u5728\u67E5\u8BE2\u623F\u95F4\u2026";
    try {
      var result = await yyGameAuthAPI("POST", { action: "rooms", id: select.value, keyword: document.getElementById("yy-room-query").value, offset: offset || 0 });
      if (!result.ok) throw Error(result.error);
      var host = document.getElementById("yy-room-search-results");
      if (!host) return;
      host.replaceChildren();
      status.textContent = (result.rooms || []).length ? "\u8BF7\u9009\u62E9\u623F\u95F4" : "\u6CA1\u6709\u5339\u914D\u7684\u5728\u7EBF\u623F\u95F4";
      for (var room of result.rooms || []) {
        let item = room;
        var btn = document.createElement("button");
        btn.className = "btn-s";
        btn.style.cssText = "display:block;width:100%;text-align:left;margin:12px 0;white-space:normal";
        btn.textContent = (item.name || "\u672A\u547D\u540D\u623F\u95F4") + " \xB7 " + item.players + "/" + item.max_players + " \u4EBA \xB7 room_id " + item.room_id;
        btn.onclick = async () => {
          closeSubPage();
          await Rcfg();
          var server = document.getElementById("bot-server"), field = document.getElementById("yy-lobby-room-id");
          if (server) {
            server.value = "LobbyGame:room_id=" + item.room_id;
            server.dispatchEvent(new Event("input"));
            if (field) field.value = item.room_id;
            toggleServerPass();
            T("\u623F\u95F4\u5DF2\u9009\u62E9\uFF0C\u8BF7\u4FDD\u5B58\u8FDE\u63A5\u8BBE\u7F6E", "s");
          }
        };
        host.append(btn);
      }
      if ((result.rooms || []).length === 20) {
        var next = document.createElement("button");
        next.className = "btn-s";
        next.textContent = "\u4E0B\u4E00\u9875";
        next.onclick = () => search((offset || 0) + 20);
        host.append(next);
      }
    } catch (e) {
      status.textContent = e.message || "\u641C\u7D22\u5931\u8D25\uFF0C\u8BF7\u91CD\u8BD5";
    } finally {
      button.disabled = false;
    }
  }
  document.getElementById("yy-room-search").onclick = () => search(0);
}
var yyRoomSettingsBase = Rcfg;
Rcfg = async function() {
  var stillLive = yySettingsViewGuard();
  await yyRoomSettingsBase();
  if (!stillLive()) return;
  var input = document.getElementById("bot-server");
  if (!input) return;
  input.placeholder = "\u79DF\u8D41\u670D\u53F7 / \u5C71\u5934\u9080\u8BF7\u7801 / \u5B8C\u6574\u6570\u5B57\u623F\u95F4ID";
  input.insertAdjacentHTML("afterend", '<details class="yy-connection-format"><summary>\u5982\u4F55\u586B\u5199\uFF1F</summary><p class="yy-hint">\u79DF\u8D41\u670D\uFF1A\u586B\u5199\u670D\u53F7\u3002\u5C71\u5934\uFF1A\u7C98\u8D34\u9080\u8BF7\u7801\u6216\u9080\u8BF7\u94FE\u63A5\u3002<br><strong>\u8054\u673A\u623F\u95F4 ID \u600E\u4E48\u627E\uFF1F</strong>\u6253\u5F00\u8054\u673A\u623F\u95F4\u7684\u4E8C\u7EF4\u7801\uFF0C\u5206\u4EAB\u5230\u5FAE\u4FE1\uFF0C\u626B\u4E00\u626B\uFF0C\u590D\u5236\u5F97\u5230\u7684\u5B8C\u6574\u6570\u5B57\uFF0C\u76F4\u63A5\u7C98\u8D34\u5230\u4E0A\u65B9\u3002<br>\u5DF2\u7ECF\u52A0\u5165\u591A\u4E2A\u5C71\u5934\uFF1F\u5728\u9080\u8BF7\u7801\u540E\u52A0 _1\uFF0C\u53EF\u8FDB\u5165\u5217\u8868\u4E2D\u7684\u7B2C\u4E00\u4E2A\u5C71\u5934\u3002</p><button class="btn-s" onclick="yyRoomSearchPage()">\u641C\u7D22\u5E76\u9009\u62E9\u8054\u673A\u623F\u95F4</button></details>');
};
function yyOpen4399Official() {
  const url = "https://web.4399.com/user/reg.php?internal=1";
  if (window.android && android.openVerification) android.openVerification(url);
  else window.open(url, "_blank", "noopener");
}
function yyOpen4399Registration() {
  delete subPageCache["4399\u6CE8\u518C\u4E0E\u63A5\u5165"];
  openSubPage("4399\u6CE8\u518C\u4E0E\u63A5\u5165", yyIntro("4399\u6CE8\u518C\u4E0E\u63A5\u5165", "\u6CE8\u518C\u7F51\u7AD9\u3001\u6E38\u620F\u6863\u6848\u3001\u6E38\u56ED\u4EE4\u724C\u662F\u4E09\u4E2A\u8FDE\u7EED\u6B65\u9AA4\u3002") + '<section class="card"><h3>1. \u5B98\u65B9\u6CE8\u518C\u4E0E\u672C\u4EBA\u9A8C\u8BC1</h3><p>\u6CA1\u67094399\u8D26\u53F7\u65F6\u6253\u5F00\u5B98\u7F51\u5B8C\u6210\u6CE8\u518C\u3001\u5B9E\u540D\u6216\u77ED\u4FE1\u9A8C\u8BC1\u3002\u51FA\u73B0\u6E38\u620F\u5217\u8868\u8BF4\u660E4399\u6CE8\u518C\u5DF2\u7ED3\u675F\uFF0C\u53EF\u4EE5\u8FD4\u56DE\u5DE5\u5177\u7BB1\u3002</p><button class="btn" onclick="yyOpen4399Official()">\u6253\u5F004399\u5B98\u65B9\u6CE8\u518C</button><h3>2. \u63A5\u5165\u6211\u7684\u4E16\u754C\u6E38\u620F\u6863\u6848</h3><p>\u56DE\u5230\u6E38\u56ED\uFF0C\u4F7F\u7528\u521A\u624D\u76844399\u7528\u6237\u540D\u548C\u5BC6\u7801\u8FDB\u884C\u6E20\u9053\u767B\u5F55\u3002\u9700\u8981\u6635\u79F0\u65F6\u586B\u5199\u672C\u4EBA\u6E38\u620F\u6635\u79F0\u3002</p><button class="btn" onclick="yy4399ReturnLogin()">\u5DF2\u6709\u8D26\u53F7\uFF0C\u7EE7\u7EED\u63A5\u5165</button><h3>3. \u4FDD\u5B58\u4EE4\u724C\u5E76\u8FDE\u63A5</h3><p>\u7B7E\u53D1\u6210\u529F\u540E\u9009\u62E9\u6E38\u56ED\u72EC\u7ACB\u8BA4\u8BC1\uFF0C\u628A\u4EE4\u724C\u4E0E\u670D\u52A1\u5668\u53F7\u4FDD\u5B58\u5230\u72EC\u7ACB\u8D26\u53F7\u5DE5\u4F5C\u7A7A\u95F4\u3002</p></section>');
}
async function yy4399ReturnLogin() {
  await yyGameAuthPage();
  document.getElementById("yy-ga-mode").value = "login_4399";
  yyGameAuthMode();
}
function yy4399Verification(r) {
  delete subPageCache["4399\u672C\u4EBA\u9A8C\u8BC1"];
  openSubPage("4399\u672C\u4EBA\u9A8C\u8BC1", yyIntro("4399\u672C\u4EBA\u9A8C\u8BC1", "\u9A8C\u8BC1\u7801\u7531\u8D26\u53F7\u672C\u4EBA\u586B\u5199\uFF0C\u4E0D\u81EA\u52A8\u8BC6\u522B\u3002") + '<section class="card"><img id="yy-ga-captcha-image" alt="4399\u9A8C\u8BC1\u7801"><label>\u9A8C\u8BC1\u7801</label><input id="yy-ga-captcha-code"><label>\u91CD\u65B0\u8F93\u51654399\u5BC6\u7801</label><input id="yy-ga-captcha-password" type="password" autocomplete="off"><button class="btn" id="yy-ga-captcha-submit">\u9A8C\u8BC1\u5E76\u7EE7\u7EED</button><p id="yy-ga-captcha-result" role="status"></p></section>');
  document.getElementById("yy-ga-captcha-image").src = r.captcha_url || "";
  document.getElementById("yy-ga-captcha-submit").onclick = async function() {
    this.disabled = true;
    try {
      const v = await yyGameAuthAPI("POST", { action: "resume_4399", id: r.pending_id, captcha_id: r.captcha_id, captcha: document.getElementById("yy-ga-captcha-code").value, password: document.getElementById("yy-ga-captcha-password").value });
      if (!v.ok) {
        document.getElementById("yy-ga-captcha-result").textContent = v.error || "\u9A8C\u8BC1\u672A\u901A\u8FC7";
        return;
      }
      await yyGameAuthPage();
      yyGameAuthShowCredential(v, v.account);
    } catch (e) {
      const status = document.getElementById("yy-ga-captcha-result");
      if (status) status.textContent = e.message || "\u7F51\u7EDC\u8BF7\u6C42\u5931\u8D25\uFF0C\u8BF7\u91CD\u8BD5";
    } finally {
      this.disabled = false;
    }
  };
}
