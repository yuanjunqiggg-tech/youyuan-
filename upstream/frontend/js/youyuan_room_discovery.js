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
  function ownerLabel(v) {
    var id = String((v && v.owner_id) || "");
    return (v && v.owner) || (id ? "ID " + id.slice(0, 8) : "未公开");
  }
  function el(tag, text, cls) {
    var n = document.createElement(tag);
    if (text != null) n.textContent = text;
    if (cls) n.className = cls;
    return n;
  }
  // allowRelative：同源相对路径（如 /api/...）也是真实图片地址，仅渲染当前服务器卡时放开。
  function image(url, cls, allowRelative) {
    if (!/^https:\/\//i.test(url || "") && !(allowRelative && /^\//.test(url || ""))) return null;
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
      markSelected(result.selected);
      await start(result.selected, original);
    } finally {
      choosing = false;
    }
  }
  function markSelected(candidate) {
    var box = document.getElementById("yy-room-results");
    if (box) {
      var old = document.getElementById("yy-room-selected-bar");
      if (old) old.remove();
      var bar = el("p", "\u5DF2\u9009\u62E9 " + labels[candidate.type] + " \u00B7 " + (candidate.name || candidate.number || candidate.room_id) + " \u2014 \u6B63\u5728\u8FDE\u63A5\u2026", "yy-room-selected-bar");
      bar.id = "yy-room-selected-bar";
      box.prepend(bar);
      var card = box.querySelector('.yy-room-card[data-id="' + candidate.id + '"]');
      if (card) {
        card.classList.add("yy-room-card-selected");
        var btn = card.querySelector("button");
        if (btn) { btn.disabled = true; btn.textContent = "\u5DF2\u9009\u62E9\uFF0C\u8FDE\u63A5\u4E2D\u2026"; }
        card.scrollIntoView({ block: "nearest" });
      }
    }
  }
  function renderChoice(candidate, original) {
    var current = document.getElementById("yy-connection-server-info");
    if (current && [candidate.target, candidate.number, candidate.room_id].includes(current.yyTarget)) return;
    var box = host();
    if (!box) return;
    box.replaceChildren();
    renderCard(candidate, box, true);
  }
  function renderCard(candidate, box, canChange) {
    // 当前服务器信息卡：封面/头像/名称/房主/人数与版本徽标齐全，
    // 重新查询仅列出候选（listOnly），绝不自动连接。
    var card = el("article", null, "yy-room-card yy-room-card-selected");
    card.setAttribute("data-id", candidate.id || "");
    var badges = el("div", null, "yy-room-badges");
    var verBadge = el("span", candidate.version || "版本未知", "yy-room-ver");
    badges.append(verBadge);
    var cntBadge = el("span", "", "yy-room-badge");
    var count = value => Number.isSafeInteger(value) && value >= 0 ? value : "未知";
    cntBadge.textContent = "在线玩家 " + count(candidate.players) + " / " + count(candidate.max_players);
    badges.append(cntBadge);
    card.append(badges);
    var cover = image(candidate.cover_url, "yy-room-cover");
    if (cover) card.append(cover);
    var content = el("div", null, "yy-room-content");
    var avatar = image(candidate.avatar_url, "yy-room-avatar");
    if (avatar) content.append(avatar);
    content.append(el("strong", candidate.name || "服务器信息待更新"));
    var details = [labels[candidate.type], "服务器 " + (candidate.number || candidate.target || "未知"), "服主 " + ownerLabel(candidate)].filter(Boolean);
    content.append(el("p", details.join(" \xB7 ")));
    if (canChange) {
    var change = el("button", "查看其他服务器", "btn-s");
    change.type = "button";
    change.onclick = function() {
      discover(true, true);
    };
    content.append(change);
    }
    card.append(content);
    box.append(card);
  }
  // 当前服务器：宽幅 hero 版式（封面横幅 + 头像 + 四项事实），不再复用候选小卡。
  function renderCurrent(candidate) {
    var box = document.getElementById("yy-connection-server-card");
    if (!box) return;
    box.replaceChildren();
    var card = el("article", null, "yy-hero-card");
    card.setAttribute("data-id", candidate.id || "");
    var cover = image(candidate.cover_url, "yy-hero-cover");
    if (cover) card.append(cover);
    else card.classList.add("yy-hero-card-plain");
    var main = el("div", null, "yy-hero-main");
    var title = el("div", null, "yy-hero-title");
    // 头像数据源：后端 yyRoomCandidate.avatar_url 只在 Go 侧定义了字段，当前没有任何赋值点，
    // 因此这里拿到空值是常态。没有真实头像就用服务器名首字占位（不编造图片、不伪造皮肤）。
    var avatar = image(candidate.avatar_url, "yy-hero-avatar", true);
    if (avatar) title.append(avatar);
    else {
      var seed = String(candidate.name || candidate.number || candidate.target || "服").trim();
      var fallback = el("span", seed.charAt(0) || "服", "yy-hero-avatar yy-hero-avatar-fallback");
      fallback.setAttribute("aria-hidden", "true");
      title.append(fallback);
    }
    var text = el("div", null, "yy-hero-title-text");
    text.append(el("strong", candidate.name || (candidate.number ? "服务器 " + candidate.number : "服务器信息待更新")));
    var tags = el("div", null, "yy-hero-tags");
    tags.append(el("span", labels[candidate.type] || "服务器", "yy-hero-tag"));
    if (candidate.version) tags.append(el("span", candidate.version, "yy-hero-tag"));
    text.append(tags);
    title.append(text);
    main.append(title);
    var count = function(value) { return Number.isSafeInteger(value) && value >= 0 ? value : "未知"; };
    var ownerID = String(candidate.owner_id || "");
    var owner = candidate.owner || (ownerID ? "ID " + ownerID.slice(0, 8) : "未公开");
    var facts = el("dl", null, "yy-hero-facts");
    [["服务器", String(candidate.number || candidate.target || "未知")],
     ["服主", owner],
     ["在线玩家", count(candidate.players) + " / " + count(candidate.max_players)],
     ["版本", candidate.version || "未知"]].forEach(function(pair) {
      var cell = el("div", null, "yy-hero-fact");
      cell.append(el("dt", pair[0]));
      cell.append(el("dd", pair[1]));
      facts.append(cell);
    });
    main.append(facts);
    card.append(main);
    box.append(card);
    var old = document.querySelector("#yy-room-results .yy-room-card-selected");
    if (old && old.getAttribute("data-id") === candidate.id) old.remove();
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
        card.setAttribute("data-id", v.id);
        var badges = el("div", null, "yy-room-badges");
        var verBadge = el("span", v.version || "", "yy-room-ver");
        if (v.version) badges.append(verBadge);
        var cntBadge = el("span", "", "yy-room-badge");
        if (v.players != null) cntBadge.textContent = "\u5728\u7EBF " + v.players + (v.max_players != null ? "/" + v.max_players : "");
        else if (v.max_players != null) cntBadge.textContent = "\u5BB9\u91CF " + v.max_players;
        if (cntBadge.textContent) badges.append(cntBadge);
        if (badges.children.length) card.append(badges);
        var cover = image(v.cover_url, "yy-room-cover");
        if (cover) card.append(cover);
        var content = el("div", null, "yy-room-content");
        var avatar = image(v.avatar_url, "yy-room-avatar");
        if (avatar) content.append(avatar);
        content.append(el("strong", v.name || v.number || v.room_id));
        var details = [labels[kind], "服务器 " + (v.number || v.target || "未知"), "服主 " + ownerLabel(v)];
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
    m.innerHTML = `<div class="modal-box" style="max-width:420px"><h3 style="margin-bottom:10px">\u600E\u4E48\u8FDE\u63A5\u670D\u52A1\u5668</h3><ol style="font-size:13px;line-height:1.9;padding-left:18px;color:var(--phx-text-secondary)"><li>\u5728\u4E0A\u65B9\u8F93\u5165\u6846\u586B\uFF1A<b>\u623F\u95F4\u53F7</b>\u3001<b>\u623F\u540D</b> \u6216 <b>mc.163.com \u5206\u4EAB\u94FE\u63A5</b>\uFF0C\u7CFB\u7EDF\u4F1A\u81EA\u52A8\u8BC6\u522B\u79DF\u8D41\u670D\u3001\u8054\u673A\u623F\u95F4\u6216\u672C\u5730\u623F\u95F4\u3002</li><li>\u70B9\u300C\u8FDE\u63A5\u670D\u52A1\u5668\u300D\uFF1A\u4F1A\u81EA\u52A8\u67E5\u8BE2\u79DF\u8D41\u670D\u3001\u8054\u673A\u623F\u95F4\u548C\u672C\u5730\u623F\u95F4\uFF1B\u6709\u591A\u4E2A\u7ED3\u679C\u65F6\u9009\u4E00\u4E2A\u5373\u53EF\u3002</li><li>\u8FDE\u63A5\u540E\u5982\u679C\u4E00\u76F4\u7B49\u5F85\uFF1A\u8FDB\u5165\u6E38\u620F\u628A <b>OP \u6743\u9650</b>\u6388\u4E88\u673A\u5668\u4EBA\uFF0C\u4F1A\u81EA\u52A8\u7EE7\u7EED\u3002</li><li>\u8FDE\u63A5\u8FC7\u7A0B\u53EF\u4EE5\u5173\u6389\u5F39\u7A97\u540E\u53F0\u7EE7\u7EED\uFF0C\u8FDB\u5EA6\u770B\u9875\u9762\u65E5\u5FD7\u3002</li></ol><div class="row" style="margin-top:14px"><button type="button" class="btn" id="yy-guide-start">\u5F00\u59CB\u8FDE\u63A5</button><button type="button" class="btn-s" onclick="document.getElementById('yy-conn-guide').remove()">\u77E5\u9053\u4E86</button></div></div>`;
    document.body.appendChild(m);
    var s = document.getElementById("yy-guide-start");
    if (s) s.onclick = function() {
      m.remove();
      discover(false);
    };
  }
  async function discover(force, listOnly) {
    if (pending) return false;
    var original = fields();
    if (typeof yySettingsViewGuard === "function") Object.defineProperty(original, "stillLive", { value: yySettingsViewGuard() });
    if (!original.input) {
      var auto = await ensureInput();
      if (!auto) {
        showGuide();
        return false;
      }
      original.input = auto;
    }
    pending = true;
    var run = ++sequence, box = host();
    if (box) box.replaceChildren(el("p", "\u6B63\u5728\u67E5\u8BE2\u79DF\u8D41\u670D\u3001\u8054\u673A\u623F\u95F4\u548C\u672C\u5730\u623F\u95F4\u2026"));
    try {
      var result = await A("POST", "/api/youyuan/rooms", Object.assign({ force: !!force }, original));
      if (run !== sequence || !same(original, fields())) return true;
      if (!result.ok) {
        if (box) box.replaceChildren(el("p", result.error || "\u623F\u95F4\u53D1\u73B0\u5931\u8D25"));
        return false;
      }
      if (result.selected) {
        if (!listOnly) {
          renderChoice(result.selected, original);
          await start(result.selected, original);
          return true;
        }
        if (!(result.candidates || []).length) { renderChoice(result.selected, original); return true; }
      }
      render(result, original);
      if (!listOnly && result.complete && (result.candidates || []).length === 1) await choose(result.candidates[0], original);
    } catch (error) {
      if (run === sequence && box) box.replaceChildren(el("p", "\u623F\u95F4\u67E5\u8BE2\u5931\u8D25\uFF0C\u8BF7\u91CD\u8BD5\u3002"));
      return false;
    } finally {
      pending = false;
    }
  }
  var previous = doConn;
  // 直连模式：分享链接/#@短号、8-20 位纯数字（网易租赁服号段，如 8 位新号 98014267）、11 位 hex 旧号（如 94F195C41B5）；7 位及以下短号有联机/本地房间歧义，走发现
  var DIRECT_LINK = /^DomainGame:|^LobbyGame:room_id=|^https?:\/\/|mc\.163\.com\//;
  var DIRECT_NUMBER = /^\d{8,20}$/;
  var DIRECT_LEGACY = /^[0-9a-f]{11}(?:_[1-9]\d*)?$/i;
  doConn = async function() {
    var raw = fields().input;
    if (!raw) raw = await ensureInput() || "";
    if (!raw) {
      showGuide();
      return;
    }
    if (DIRECT_LINK.test(raw) || DIRECT_NUMBER.test(raw) || DIRECT_LEGACY.test(raw)) return previous();
    var ok = await discover(false);
    // 房间发现失败但输入明显是服务器号（纯数字/旧号段）：回退直连，保证"连接服务器"永远启动真正的连接，不再静默无响应
    if (!ok && (DIRECT_NUMBER.test(raw) || DIRECT_LEGACY.test(raw))) {
      T("房间查询失败，正在直接连接服务器 " + raw + " ...", "w");
      return previous();
    }
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
  window.yyRoomDiscovery = { discover, fields, render, renderCurrent };
})();
