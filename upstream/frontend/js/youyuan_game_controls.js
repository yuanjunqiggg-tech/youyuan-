(function() {
  "use strict";
  function scope() {
    return typeof yyReadScope === "function" ? yyReadScope() : String(window.yyActiveAccount || "main");
  }
  function modal(title) {
    var _a;
    (_a = document.getElementById("yy-game-control-modal")) == null ? void 0 : _a.remove();
    const overlay = document.createElement("div");
    overlay.id = "yy-game-control-modal";
    overlay.className = "modal-overlay";
    const box = document.createElement("div");
    box.className = "modal-box yy-game-control-box";
    const head = document.createElement("div");
    head.className = "yy-form-head";
    const h = document.createElement("h3");
    h.textContent = title;
    const close = document.createElement("button");
    close.className = "yy-text-btn";
    close.textContent = "\u5173\u95ED";
    close.onclick = () => overlay.remove();
    head.append(h, close);
    const body = document.createElement("div");
    box.append(head, body);
    overlay.append(box);
    document.body.append(overlay);
    return body;
  }
  function notice(host, message) {
    host.querySelector("[role=status]").textContent = message;
  }
  function valid(host, owner) {
    if (!host.isConnected || scope() !== owner) return false;
    return true;
  }
  const base = Rcfg;
  Rcfg = async function() {
    const owner = scope();
    await base.apply(this, arguments);
    if (scope() !== owner || typeof curTab !== "undefined" && curTab !== "settings" || typeof subPageOpen !== "undefined" && subPageOpen) return;
    const root = document.querySelector("#tab-settings>.content-inner");
    if (!root || document.getElementById("yy-game-controls-entry")) return;
    const card = document.createElement("section");
    card.id = "yy-game-controls-entry";
    card.className = "card";
    card.innerHTML = '<h3>\u673A\u5668\u4EBA\u7B49\u7EA7\u3001\u7ECF\u9A8C\u4E0E\u6E38\u620F\u51FA\u53E3</h3><p class="yy-hint">\u64CD\u4F5C\u5F53\u524D\u8D26\u53F7\u7684\u673A\u5668\u4EBA\u3002\u6E38\u620F\u5C5E\u6027\u4EE5\u670D\u52A1\u5668\u540C\u6B65\u4E3A\u51C6\uFF1B\u51FA\u53E3\u8BBE\u7F6E\u7528\u4E8E\u4E0B\u4E00\u6B21\u8FDE\u63A5\u3002</p><div class="yy-actions"><button class="yy-secondary" onclick="yyExperiencePage()">\u7B49\u7EA7\u4E0E\u7ECF\u9A8C</button><button class="yy-secondary" onclick="yyGameEgressPage()">\u6E38\u620F\u51FA\u53E3\u8282\u70B9</button></div>';
    root.append(card);
  };
  window.yyExperiencePage = async function() {
    const owner = scope(), host = modal("\u5F53\u524D\u673A\u5668\u4EBA \xB7 \u7B49\u7EA7\u4E0E\u7ECF\u9A8C");
    let snapshot = null, lastRequest = "", writing = false;
    host.innerHTML = '<p class="yy-hint">\u67E5\u770B\u5F53\u524D\u89D2\u8272\u7B49\u7EA7\u548C\u7ECF\u9A8C\u6761\uFF0C\u672A\u540C\u6B65\u65F6\u53EF\u7A0D\u540E\u5237\u65B0\u3002</p><div class="yy-control-metrics"><div><small>\u6E38\u620F\u7B49\u7EA7</small><strong id="yy-xp-level">\u2014</strong></div><div><small>\u7ECF\u9A8C\u6761\u8FDB\u5EA6</small><strong id="yy-xp-progress">\u2014</strong></div></div><p id="yy-xp-source" class="yy-hint"></p><button id="yy-xp-refresh" class="yy-secondary">\u5237\u65B0\u6E38\u620F\u5C5E\u6027</button><details class="yy-control-section"><summary>\u4FEE\u6539\u7B49\u7EA7\u6216\u7ECF\u9A8C</summary><p class="yy-hint">\u9700\u8981\u5F53\u524D\u6E38\u620F\u8D26\u53F7OP\u6743\u9650\u3002\u53EA\u4FEE\u6539\u5F53\u524D\u673A\u5668\u4EBA\uFF0C\u4E0D\u5F71\u54CD\u5176\u4ED6\u73A9\u5BB6\u3002</p><div class="yy-control-grid"><label>\u4FEE\u6539\u65B9\u5F0F<select id="yy-xp-operation"><option value="set_level">\u8BBE\u7F6E\u7B49\u7EA7</option><option value="add_levels">\u589E\u52A0 / \u51CF\u5C11\u7B49\u7EA7</option><option value="add_points">\u589E\u52A0\u7ECF\u9A8C\u70B9</option></select></label><label>\u6570\u503C<input id="yy-xp-amount" type="number" step="1" value="1"></label></div><button id="yy-xp-apply" class="yy-primary">\u63D0\u4EA4\u4FEE\u6539</button></details><p role="status" class="yy-hint"></p>';
    const apply = host.querySelector("#yy-xp-apply"), refresh = host.querySelector("#yy-xp-refresh");
    function draw(v) {
      snapshot = v;
      host.querySelector("#yy-xp-level").textContent = v.level == null ? "\u672A\u540C\u6B65" : String(v.level);
      host.querySelector("#yy-xp-progress").textContent = v.progress == null ? "\u672A\u540C\u6B65" : (v.progress * 100).toFixed(1) + "%";
      host.querySelector("#yy-xp-source").textContent = (v.player || "\u672A\u8FDE\u63A5") + " \xB7 " + (v.connected ? "\u5DF2\u8FDE\u63A5" : "\u672A\u8FDE\u63A5") + " \xB7 " + (v.can_modify ? "\u53EF\u4FEE\u6539" : "\u5F53\u524D\u65E0\u4FEE\u6539\u6743\u9650") + " \xB7 \u6765\u6E90\uFF1A\u670D\u52A1\u5668\u5C5E\u6027\u540C\u6B65";
      apply.disabled = writing || !v.can_modify;
    }
    async function load() {
      refresh.disabled = true;
      try {
        if (scope() !== owner) return;
        const r = await A("GET", "/api/youyuan/player/experience" + (lastRequest ? "?request_id=" + encodeURIComponent(lastRequest) : ""));
        if (!valid(host, owner)) return;
        if (!r.ok) throw Error(r.error);
        draw(r.experience);
        if (r.receipt) notice(host, resultText(r.receipt));
      } catch (e) {
        if (valid(host, owner)) notice(host, e.message);
      } finally {
        refresh.disabled = false;
      }
    }
    function resultText(r) {
      return { verified: "\u670D\u52A1\u5668\u5DF2\u63A5\u53D7\uFF0C\u6E38\u620F\u5C5E\u6027\u5DF2\u8BFB\u56DE\u3002", accepted: "\u670D\u52A1\u5668\u5DF2\u63A5\u53D7\uFF1B\u5C1A\u672A\u6536\u5230\u66F4\u65B0\u540E\u7684\u5C5E\u6027\uFF0C\u8BF7\u5237\u65B0\u67E5\u770B\u3002", unchanged: "\u5F53\u524D\u7B49\u7EA7\u5DF2\u7ECF\u662F\u76EE\u6807\u503C\uFF0C\u65E0\u9700\u53D1\u9001\u4FEE\u6539\u3002", rejected: r.error, not_sent: r.error, outcome_unknown: r.error }[r.status] || r.error || "\u7ED3\u679C\u672A\u786E\u8BA4";
    }
    refresh.onclick = load;
    apply.onclick = async () => {
      if (!valid(host, owner) || apply.disabled || !snapshot) return;
      const amount = Number(host.querySelector("#yy-xp-amount").value);
      if (!Number.isSafeInteger(amount)) {
        notice(host, "\u8BF7\u8F93\u5165\u6574\u6570");
        return;
      }
      const operation = host.querySelector("#yy-xp-operation").value;
      writing = true;
      lastRequest = crypto.randomUUID();
      const body = { request_id: lastRequest, expected_session: snapshot.session, expected_level: snapshot.level, operation, amount };
      apply.disabled = true;
      notice(host, "\u6B63\u5728\u7B49\u5F85\u670D\u52A1\u5668\u56DE\u6267\u2026");
      try {
        const r = await A("POST", "/api/youyuan/player/experience", body);
        if (!valid(host, owner)) return;
        if (r.after) draw(r.after);
        notice(host, resultText(r));
      } catch (e) {
        if (valid(host, owner)) notice(host, "\u56DE\u6267\u672A\u786E\u8BA4\uFF0C\u53EF\u80FD\u5DF2\u6267\u884C\u3002\u8BF7\u5237\u65B0\u67E5\u770B\u4E0A\u6B21\u7ED3\u679C\uFF0C\u4E0D\u4F1A\u81EA\u52A8\u91CD\u53D1\u3002");
      } finally {
        writing = false;
        if (valid(host, owner)) apply.disabled = !(snapshot == null ? void 0 : snapshot.can_modify);
      }
    };
    await load();
  };
  window.yyGameEgressPage = async function() {
    const owner = scope(), host = modal("\u6E38\u620F\u51FA\u53E3\u8282\u70B9");
    host.innerHTML = '<p class="yy-hint">\u9009\u62E9\u6E38\u620F\u8FDE\u63A5\u4F7F\u7528\u7684\u7EBF\u8DEF\u3002\u4FDD\u5B58\u540E\uFF0C\u4E0B\u6B21\u8FDE\u63A5\u65F6\u751F\u6548\u3002</p><label>\u6E38\u620F\u8FDE\u63A5\u65B9\u5F0F<select id="yy-egress-mode"><option value="system">\u673A\u5668\u4EBA\u6240\u5728\u8BBE\u5907\u7F51\u7EDC\uFF08\u9ED8\u8BA4\uFF09</option><option value="official">\u5B98\u65B9\u52A0\u901F\u8282\u70B9</option><option value="socks5">\u6211\u7684 SOCKS5 UDP \u8282\u70B9</option></select></label><details id="yy-egress-proxy" class="yy-control-section"><summary>\u6211\u7684\u8282\u70B9\u914D\u7F6E</summary><p class="yy-hint">\u586B\u5199\u4F60\u6709\u6743\u4F7F\u7528\u3001\u652F\u6301\u6E38\u620FUDP\u8F6C\u53D1\u7684 SOCKS5 \u8282\u70B9\u3002</p><label>\u8282\u70B9\u5730\u5740<input id="yy-egress-address" placeholder="\u4F8B\u5982 192.0.2.10:1080" autocomplete="off"></label><div class="yy-control-grid"><label>\u7528\u6237\u540D\uFF08\u53EF\u9009\uFF09<input id="yy-egress-user" autocomplete="off"></label><label>\u5BC6\u7801\uFF08\u53EF\u9009\uFF09<input id="yy-egress-password" type="password" autocomplete="new-password" placeholder="\u7559\u7A7A\u4FDD\u7559\u5DF2\u4FDD\u5B58\u5BC6\u7801"></label></div><label><input id="yy-egress-authorized" type="checkbox">\u6211\u6709\u6743\u4F7F\u7528\u6B64\u8282\u70B9</label></details><div class="yy-control-section"><strong>\u672C\u6B21\u8FDE\u63A5\u7684\u5B9E\u9645\u72B6\u6001</strong><p id="yy-egress-runtime" class="yy-hint">\u6B63\u5728\u8BFB\u53D6\u2026</p></div><p class="yy-hint">\u5F53\u524D\u6E38\u620F\u4E0D\u4F1A\u56E0\u4FDD\u5B58\u8BBE\u7F6E\u800C\u65AD\u5F00\u3002</p><button id="yy-egress-save" class="yy-primary" disabled>\u4FDD\u5B58\u51FA\u53E3\u8BBE\u7F6E</button><p role="status" class="yy-hint"></p>';
    try {
      const r = await A("GET", "/api/youyuan/game-egress");
      if (!valid(host, owner)) return;
      if (!r.ok) throw Error(r.error);
      const c = r.configuration;
      host.querySelector("#yy-egress-mode").value = c.mode;
      host.querySelector("#yy-egress-address").value = c.proxy_address || "";
      host.querySelector("#yy-egress-user").value = c.proxy_username || "";
      host.querySelector("#yy-egress-authorized").checked = !!c.proxy_authorized;
      host.querySelector("#yy-egress-password").placeholder = c.has_password ? "\u5DF2\u4FDD\u5B58\u5BC6\u7801\uFF1B\u7559\u7A7A\u4FDD\u7559" : "\u672A\u8BBE\u7F6E\u5BC6\u7801";
      const state = r.runtime || {};
      const stateNames = { not_started: "\u5C1A\u672A\u5F00\u59CB\u8FDE\u63A5", configured: "\u7B49\u5F85\u8FDE\u63A5", receiving: "\u6B63\u5728\u6536\u5305", sending: "\u6B63\u5728\u53D1\u5305", dialing: "\u6B63\u5728\u8FDE\u63A5", connected: "\u4F20\u8F93\u5DF2\u8FDE\u63A5", closed: "\u5DF2\u5173\u95ED", failed: "\u8FDE\u63A5\u5931\u8D25", unsupported: "\u6B64\u623F\u95F4\u6682\u4E0D\u652F\u6301", system_transport_uninstrumented: "\u4F7F\u7528\u7CFB\u7EDF\u7EBF\u8DEF" };
      const route = state.host_kind === "linux" ? "Linux \u6267\u884C\u8282\u70B9\u7F51\u7EDC" : state.host_kind === "android" ? "\u672C\u673A\u7F51\u7EDC" : "\u6267\u884C\u8BBE\u5907\u7F51\u7EDC";
      host.querySelector("#yy-egress-runtime").textContent = (state.mode === "socks5" ? "\u6211\u7684\u8282\u70B9" : state.mode === "official" ? "\u5B98\u65B9\u8282\u70B9" : route) + " \xB7 " + (stateNames[state.state] || "\u7B49\u5F85\u72B6\u6001\u66F4\u65B0") + " \xB7 " + (state.verified_game_session ? "\u6E38\u620F\u5DF2\u8FDE\u63A5" : "\u5C1A\u672A\u9A8C\u8BC1\u6E38\u620F\u8FDE\u63A5");
      host.querySelector("#yy-egress-proxy").open = c.mode === "socks5";
      host.querySelector("#yy-egress-save").disabled = false;
    } catch (e) {
      if (valid(host, owner)) notice(host, e.message);
    }
    host.querySelector("#yy-egress-mode").onchange = function() {
      host.querySelector("#yy-egress-proxy").open = this.value === "socks5";
    };
    const save = host.querySelector("#yy-egress-save");
    save.onclick = async () => {
      if (!valid(host, owner) || save.disabled) return;
      save.disabled = true;
      const body = { mode: host.querySelector("#yy-egress-mode").value, proxy_address: host.querySelector("#yy-egress-address").value.trim(), proxy_username: host.querySelector("#yy-egress-user").value, proxy_password: host.querySelector("#yy-egress-password").value, proxy_authorized: host.querySelector("#yy-egress-authorized").checked };
      try {
        const r = await A("POST", "/api/youyuan/game-egress", body);
        if (!valid(host, owner)) return;
        host.querySelector("#yy-egress-password").value = "";
        notice(host, r.ok ? "\u51FA\u53E3\u8BBE\u7F6E\u5DF2\u4FDD\u5B58\uFF0C\u4E0B\u6B21\u8FDE\u63A5\u751F\u6548\u3002" : r.error);
      } catch (e) {
        if (valid(host, owner)) notice(host, e.message);
      } finally {
        save.disabled = false;
      }
    };
  };
})();
