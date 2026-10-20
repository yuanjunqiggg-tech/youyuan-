(function() {
  "use strict";
  var pending = false, choosing = false, sequence = 0;
  var labels = { rental: "\u79DF\u8D41\u670D", lobby: "\u8054\u673A\u623F\u95F4", local: "\u672C\u5730\u623F\u95F4" };
  function fields() {
    var _a, _b;
    return { token: ((_a = document.getElementById("cfg-token")) == null ? void 0 : _a.value) || "", auth: curServerUrl, input: ((_b = document.getElementById("bot-server")) == null ? void 0 : _b.value.trim()) || "" };
  }
  function same(a, b) {
    return (!a.stillLive || a.stillLive()) && a.token === b.token && a.auth === b.auth && a.input === b.input;
  }
  function el(tag, text, cls) {
    var n = document.createElement(tag);
    if (text != null) n.textContent = text;
    if (cls) n.className = cls;
    return n;
  }
  function image(url, cls) {
    if (!/^https:\/\//i.test(url || "")) return null;
    var n = el("img", null, cls);
    n.src = url;
    n.alt = "";
    n.referrerPolicy = "no-referrer";
    n.onerror = function() {
      n.remove();
    };
    return n;
  }
  function host() {
    var input = document.getElementById("bot-server");
    if (!input) return null;
    var box = document.getElementById("yy-room-results");
    if (!box) {
      box = el("section", null, "yy-room-results");
      box.id = "yy-room-results";
      input.parentElement.insertAdjacentElement("afterend", box);
    }
    return box;
  }
  function clear() {
    var _a;
    sequence++;
    (_a = document.getElementById("yy-room-results")) == null ? void 0 : _a.replaceChildren();
  }
  async function ensureInput() {
    var input = document.getElementById("bot-server");
    if (!input) return "";
    var v = input.value.trim();
    if (v) return v;
    var s = await A("GET", "/api/bot/status");
    if (s && s.ok && s.connected && s.server) {
      input.value = String(s.server);
      T("\u5DF2\u81EA\u52A8\u586B\u5165\u5F53\u524D\u8FDE\u63A5\u7684\u670D\u52A1\u5668\u53F7 " + s.server, "o");
      return String(s.server);
    }
    return "";
  }
  async function start(candidate, original) {
    if (!same(original, fields())) return;
    return previous();
  }
  async function choose(candidate, original) {
    if (choosing || !same(original, fields())) return;
    choosing = true;
    try {
      var result = await A("POST", "/api/youyuan/rooms", Object.assign({ op: "select", candidate_id: candidate.id }, original));
      if (!same(original, fields())) return;
      if (!result.ok) {
        T(result.error || "\u4FDD\u5B58\u9009\u62E9\u5931\u8D25", "e");
        return;
      }
      renderChoice(result.selected, original);
      await start(result.selected, original);
    } finally {
      choosing = false;
    }
  }
  function renderChoice(candidate, original) {
    var box = host();
    if (!box) return;
    box.replaceChildren();
    box.append(el("p", "\u5DF2\u9009\u62E9 " + labels[candidate.type] + " \xB7 " + (candidate.name || candidate.number || candidate.room_id)));
    var change = el("button", "\u91CD\u65B0\u67E5\u8BE2", "btn-s");
    change.type = "button";
    change.onclick = function() {
      discover(true);
    };
    box.append(change);
  }
  function render(result, original) {
    var box = host();
    if (!box) return;
    box.replaceChildren();
    if (!result.complete) box.append(el("p", "\u90E8\u5206\u67E5\u8BE2\u672A\u5B8C\u6210\uFF0C\u8BF7\u67E5\u770B\u4E0B\u65B9\u72B6\u6001\uFF1B\u53EF\u4EE5\u660E\u786E\u9009\u62E9\u5DF2\u627E\u5230\u7684\u623F\u95F4\u3002", "yy-room-hint"));
    var rows = result.candidates || [];
    Object.keys(labels).forEach(function(kind) {
      var group = rows.filter(function(v) {
        return v.type === kind;
      });
      if (!group.length) return;
      var section = el("section", null, "yy-room-group");
      section.append(el("h4", labels[kind] + " \xB7 " + group.length));
      var grid = el("div", null, "yy-room-grid");
      group.forEach(function(v) {
        var card = el("article", null, "yy-room-card");
        var cover = image(v.cover_url, "yy-room-cover");
        if (cover) card.append(cover);
        var content = el("div", null, "yy-room-content");
        var avatar = image(v.avatar_url, "yy-room-avatar");
        if (avatar) content.append(avatar);
        content.append(el("strong", v.name || v.number || v.room_id));
        var details = [labels[kind], v.number ? "\u53F7\u7801 " + v.number : "ID " + v.room_id];
        if (v.owner) details.push("\u623F\u4E3B " + v.owner);
        else if (v.owner_id) details.push("\u623F\u4E3B UID " + v.owner_id);
        if (v.players != null) details.push("\u5728\u7EBF " + v.players + (v.max_players != null ? " / " + v.max_players : ""));
        else if (v.max_players != null) details.push("\u5BB9\u91CF " + v.max_players);
        if (v.version) details.push(v.version);
        content.append(el("p", details.join(" \xB7 ")));
        var button = el("button", "\u9009\u62E9\u5E76\u8FDE\u63A5", "btn");
        button.type = "button";
        button.onclick = async function() {
          button.disabled = true;
          try {
            await choose(v, original);
          } finally {
            button.disabled = false;
          }
        };
        content.append(button);
        card.append(content);
        grid.append(card);
      });
      section.append(grid);
      box.append(section);
    });
    (result.sources || []).filter(function(s) {
      return !s.ok || s.more;
    }).forEach(function(s) {
      box.append(el("p", labels[s.type] + "\uFF1A" + (s.more ? "\u8FD8\u6709\u66F4\u591A\u5339\u914D\uFF0C\u8BF7\u8F93\u5165\u66F4\u5177\u4F53\u7684\u53F7\u7801\u6216\u623F\u540D\u3002" : s.error || "\u67E5\u8BE2\u5931\u8D25"), "yy-room-hint"));
    });
    if (!rows.length) box.append(el("p", result.complete ? "\u6CA1\u6709\u627E\u5230\u5339\u914D\u7684\u623F\u95F4\uFF0C\u8BF7\u786E\u8BA4\u53F7\u7801\u3001\u623F\u540D\u6216\u5206\u4EAB\u94FE\u63A5\u3002" : "\u5F53\u524D\u672A\u627E\u5230\u5019\u9009\uFF0C\u5F85\u5931\u8D25\u7C7B\u522B\u6062\u590D\u540E\u91CD\u8BD5\u3002"));
  }
  function showGuide() {
    var old = document.getElementById("yy-conn-guide");
    if (old) old.remove();
    var m = document.createElement("div");
    m.id = "yy-conn-guide";
    m.className = "modal-overlay";
    m.innerHTML = `<div class="modal-box" style="max-width:420px"><h3 style="margin-bottom:10px">\u600E\u4E48\u8FDE\u63A5\u670D\u52A1\u5668</h3><ol style="font-size:13px;line-height:1.9;padding-left:18px;color:var(--phx-text-secondary)"><li>\u5728\u4E0A\u65B9\u8F93\u5165\u6846\u586B\uFF1A<b>\u670D\u52A1\u5668\u53F7</b>\u3001<b>\u623F\u540D</b>\u3001<b>#\u623F\u95F4\u53F7</b>\u3001<b>@\u77ED\u53F7</b>\uFF0C\u6216 <b>mc.163.com \u5206\u4EAB\u94FE\u63A5</b>\u3002</li><li>\u70B9\u300C\u8FDE\u63A5\u670D\u52A1\u5668\u300D\uFF1A\u4F1A\u81EA\u52A8\u67E5\u8BE2\u79DF\u8D41\u670D\u3001\u8054\u673A\u623F\u95F4\u548C\u672C\u5730\u623F\u95F4\uFF1B\u6709\u591A\u4E2A\u7ED3\u679C\u65F6\u9009\u4E00\u4E2A\u5373\u53EF\u3002</li><li>\u8FDE\u63A5\u540E\u5982\u679C\u4E00\u76F4\u7B49\u5F85\uFF1A\u8FDB\u5165\u6E38\u620F\u628A <b>OP \u6743\u9650</b>\u6388\u4E88\u673A\u5668\u4EBA\uFF0C\u4F1A\u81EA\u52A8\u7EE7\u7EED\u3002</li><li>\u8FDE\u63A5\u8FC7\u7A0B\u53EF\u4EE5\u5173\u6389\u5F39\u7A97\u540E\u53F0\u7EE7\u7EED\uFF0C\u8FDB\u5EA6\u770B\u9875\u9762\u65E5\u5FD7\u3002</li></ol><div class="row" style="margin-top:14px"><button type="button" class="btn" id="yy-guide-start">\u5F00\u59CB\u8FDE\u63A5</button><button type="button" class="btn-s" onclick="document.getElementById('yy-conn-guide').remove()">\u77E5\u9053\u4E86</button></div></div>`;
    document.body.appendChild(m);
    var s = document.getElementById("yy-guide-start");
    if (s) s.onclick = function() {
      m.remove();
      discover(false);
    };
  }
  async function discover(force) {
    if (pending) return;
    var original = fields();
    if (typeof yySettingsViewGuard === "function") Object.defineProperty(original, "stillLive", { value: yySettingsViewGuard() });
    if (!original.input) {
      var auto = await ensureInput();
      if (!auto) {
        showGuide();
        return;
      }
      original.input = auto;
    }
    pending = true;
    var run = ++sequence, box = host();
    if (box) box.replaceChildren(el("p", "\u6B63\u5728\u67E5\u8BE2\u79DF\u8D41\u670D\u3001\u8054\u673A\u623F\u95F4\u548C\u672C\u5730\u623F\u95F4\u2026"));
    try {
      var result = await A("POST", "/api/youyuan/rooms", Object.assign({ force: !!force }, original));
      if (run !== sequence || !same(original, fields())) return;
      if (!result.ok) {
        if (box) box.replaceChildren(el("p", result.error || "\u623F\u95F4\u53D1\u73B0\u5931\u8D25"));
        return;
      }
      if (result.selected) {
        renderChoice(result.selected, original);
        await start(result.selected, original);
        return;
      }
      render(result, original);
      if (result.complete && (result.candidates || []).length === 1) await choose(result.candidates[0], original);
    } catch (error) {
      if (run === sequence && box) box.replaceChildren(el("p", "\u623F\u95F4\u67E5\u8BE2\u5931\u8D25\uFF0C\u8BF7\u91CD\u8BD5\u3002"));
    } finally {
      pending = false;
    }
  }
  var previous = doConn;
  doConn = async function() {
    var raw = fields().input;
    if (!raw) raw = await ensureInput() || "";
    if (!raw) {
      showGuide();
      return;
    }
    if (/^DomainGame:|^LobbyGame:room_id=|^https?:\/\/|mc\.163\.com\//.test(raw) || /^\d{16,20}$/.test(raw) || /^[0-9a-f]{11}(?:_[1-9]\d*)?$/i.test(raw)) return previous();
    return discover(false);
  };
  var previousSettings = Rcfg;
  Rcfg = async function() {
    var live = typeof yySettingsViewGuard === "function" ? yySettingsViewGuard() : function() {
      return true;
    };
    await previousSettings();
    if (!live()) return;
    var input = document.getElementById("bot-server");
    if (!input) return;
    input.placeholder = "\u670D\u52A1\u5668\u53F7\u3001\u77ED\u623F\u95F4\u53F7\u3001\u623F\u540D\u6216\u5206\u4EAB\u94FE\u63A5";
    input.addEventListener("input", clear);
    var hint = input.nextElementSibling;
    if (hint && /#房间号=/.test(hint.textContent)) hint.textContent = "\u81EA\u52A8\u67E5\u8BE2\u79DF\u8D41\u670D\u3001\u8054\u673A\u623F\u95F4\u548C\u672C\u5730\u623F\u95F4\uFF1B\u6709\u591A\u4E2A\u7ED3\u679C\u65F6\u9009\u62E9\u4E00\u6B21\u3002";
    var original = fields();
    Object.defineProperty(original, "stillLive", { value: live });
    if (!original.input || !original.token) return;
    var result = await A("POST", "/api/youyuan/rooms", Object.assign({ op: "current" }, original));
    if (!same(original, fields()) || !result.ok || !result.choice) return;
    input.value = result.choice.input;
    renderChoice(result.choice.candidate, fields());
  };
  window.yyRoomDiscovery = { discover, fields, render };
})();
