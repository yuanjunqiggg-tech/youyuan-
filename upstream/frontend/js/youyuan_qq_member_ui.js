/* 群服互通 · 成员目录（第 19 条 / 重构规格 3.3-4.5）
 *
 * 旧版每个成员渲染一张卡，卡里塞「设为服主 / 修改显示昵称 / 设为管理员 /
 * 设为超级管理员 / 撤销授权」五个按钮，十个人就是一屏按钮墙，手机上没法用。
 *
 * 新版只做两件事：
 *   1) 列表 = 紧凑单行（真实头像 / 昵称 / 身份徽标 / ›），默认没有任何权限按钮；
 *   2) 点行 → 底部面板，五个操作全在面板里，撤销授权单独红区 + 二次确认。
 *
 * 数据源（2026-10-25 起改为接口驱动）：优先 GET /api/qq/assistant 的 members
 * 字典 + GET /api/qq/config 的 super_admins/admins/nicknames/companion_owner，
 * 不再从原渲染出来的卡片里反抓（yyA 是 qq_assistant.js 的闭包变量，DOM 反抓
 * 既不稳也拿不到后端真实数据）。接口失败时兜底走 DOM 反抓，两条路都读不到
 * 就保持原样不动——绝不把空列表塞给玩家。
 *
 * 头像：成员标识是 5-11 位纯数字（OneBot 下就是 QQ 号）时用 QQ 公网头像
 * https://q.qlogo.cn/g?b=qq&nk=<号>&s=100（免鉴权，img 直连）；
 * 官方适配器的 openid 不是 QQ 号，取不到公网头像，退回首字母色块。
 *
 * 不改后端：所有操作仍打原来的接口（/api/qq/assistant authorize、
 * /api/qq/config assistant.nicknames / companion_owner），语义完全一致。
 *
 * 权限模型（已核对代码，不能想当然）：
 *   super_admins[] 与 admins[] 是两份独立列表，可并存；
 *   companion_owner 是单独一个成员标识（服主），与上面两份不互斥；
 *   撤销授权 = 把该成员从两份列表里移除，不动服主。
 */
(function () {
  "use strict";
  if (window.YYQQDirectory) return;

  var state = {
    members: [],      // {id, name, role, isOwner}
    query: "",
    role: "all",
    lastConfig: null
  };
  var sheetHost = null;

  // 头像图加载失败时隐藏 img，露出底下的首字母色块（onerror 里不能嵌套引号，
  // 统一走这个具名函数）。
  window.YYQDHideAvatar = function (img) {
    if (img && img.style) img.style.display = "none";
  };

  function esc(value) {
    return String(value == null ? "" : value).replace(/[&<>"']/g, function (ch) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch];
    });
  }
  function toast(text, level) {
    if (typeof T === "function") T(text, level || "s");
  }
  function api(method, path, body) {
    if (typeof A === "function") return A(method, path, body);
    return Promise.reject(new Error("接口未就绪"));
  }

  // ---------- 数据源一：接口（主） ----------
  // members 字典是 {成员标识: 显示名}，权限名单在 /api/qq/config 的
  // assistant.super_admins / admins，服主是 assistant.companion_owner。
  async function fetchMembers() {
    var results = await Promise.all([
      api("GET", "/api/qq/assistant").catch(function () { return null; }),
      api("GET", "/api/qq/config").catch(function () { return null; })
    ]);
    var info = results[0], conf = results[1];
    if (!info || info.ok === false || !info.members) return [];
    var assistant = (conf && conf.config && conf.config.assistant) || {};
    var supers = assistant.super_admins || [];
    var admins = assistant.admins || [];
    var nicknames = assistant.nicknames || {};
    var owner = assistant.companion_owner || "";
    var ids = Object.keys(info.members);
    var rows = ids.map(function (id) {
      var role = supers.indexOf(id) >= 0 ? "super" : admins.indexOf(id) >= 0 ? "admin" : "member";
      var name = nicknames[id] || info.members[id];
      // 后端在不知道昵称时会把名字存成标识本身，这时显示「群成员」更友好。
      if (!name || name === id) name = "群成员";
      return { id: id, name: name, role: role, isOwner: !!owner && owner === id };
    });
    if (owner && !rows.some(function (row) { return row.id === owner; })) {
      rows.push({ id: owner, name: "服主", role: "member", isOwner: true });
    }
    sortRows(rows);
    state.members = rows;
    return rows;
  }

  // 排序：服主 → 超管 → 管理员 → 成员，同级按昵称。两条数据源共用。
  function sortRows(rows) {
    rows.sort(function (a, b) {
      if (a.isOwner !== b.isOwner) return a.isOwner ? -1 : 1;
      var rank = { super: 0, admin: 1, member: 2 };
      if (rank[a.role] !== rank[b.role]) return rank[a.role] - rank[b.role];
      return a.name.localeCompare(b.name, "zh");
    });
    return rows;
  }

  // ---------- 数据源二：DOM 反抓（兜底） ----------
  // 接口失败时从原渲染出来的卡片里读：卡片上的 <strong>名字 · 身份</strong>
  // 与 <pre>标识</pre> 就是后端真实下发的内容。
  function collect(host) {
    var rows = [];
    var ownerBox = document.getElementById("yy-qq-companion-owner");
    var owner = (ownerBox && ownerBox.dataset && ownerBox.dataset.user) || "";
    var cards = host ? host.querySelectorAll("section.yy-card") : [];
    for (var i = 0; i < cards.length; i++) {
      var card = cards[i];
      var head = card.querySelector("strong");
      var text = head ? head.textContent.trim() : "";
      var parts = text.split(" · ");
      var level = parts.length > 1 ? parts.pop().trim() : "成员";
      var name = parts.join(" · ").trim() || "群成员";
      var pre = card.querySelector("details pre");
      var id = pre ? pre.textContent.trim() : "";
      if (!id) {
        var attr = card.querySelector("[data-owner]") || card.querySelector("[data-user]");
        id = attr ? (attr.getAttribute("data-owner") || attr.getAttribute("data-user")) : "";
      }
      if (!id) continue;
      var role = level.indexOf("超级管理员") >= 0 ? "super" : level.indexOf("管理员") >= 0 ? "admin" : "member";
      rows.push({ id: id, name: name, role: role, isOwner: !!owner && owner === id });
    }
    if (owner && !rows.some(function (row) { return row.id === owner; })) {
      rows.push({ id: owner, name: "服主", role: "member", isOwner: true });
    }
    sortRows(rows);
    state.members = rows;
    return rows;
  }

  // ---------- 头像 ----------
  // 只有 5-11 位纯数字（OneBot 的 QQ 号）能拼出 QQ 公网头像；
  // 官方适配器的 openid 不是 QQ 号，返回空 = 用首字母色块。
  function avatarURL(row) {
    var id = String((row && row.id) || "");
    if (!/^\d{5,11}$/.test(id)) return "";
    return "https://q.qlogo.cn/g?b=qq&nk=" + encodeURIComponent(id) + "&s=100";
  }
  // 首字母色块垫底，真实头像绝对定位盖上去；图挂了就隐藏 img 出色块。
  function avatarHTML(row, large) {
    var size = large ? "yyqd-avatar-lg" : "";
    var initials = String((row && row.name) || "?").trim().slice(0, 1).toUpperCase();
    var url = avatarURL(row);
    var img = url ? '<img src="' + esc(url) + '" alt="" loading="lazy" decoding="async" onerror="YYQDHideAvatar(this)">' : "";
    return '<span class="yyqd-avatar ' + size + '" aria-hidden="true">' + esc(initials) + img + "</span>";
  }

  var ROLE_TEXT = { owner: "服主", super: "超级管理员", admin: "管理员", member: "成员" };
  function roleOf(row) { return row.isOwner ? "owner" : row.role; }
  function filteredRows() {
    var needle = state.query.trim().toLowerCase();
    return state.members.filter(function (row) {
      if (state.role !== "all" && roleOf(row) !== state.role) return false;
      if (!needle) return true;
      return (row.name + " " + row.id).toLowerCase().indexOf(needle) >= 0;
    });
  }

  function shellHTML() {
    var counts = { owner: 0, super: 0, admin: 0, member: 0 };
    state.members.forEach(function (row) { counts[roleOf(row)]++; });
    var chips = [["all", "全部", state.members.length], ["owner", "服主", counts.owner], ["super", "超管", counts.super], ["admin", "管理员", counts.admin], ["member", "成员", counts.member]];
    return '<div class="yyqd" data-yyqd="1">' +
      '<div class="yyqd-tools">' +
        '<label class="yyqd-search"><span class="yyqd-visually-hidden">搜索成员</span>' +
          '<input id="yyqd-query" type="search" autocomplete="off" placeholder="搜索昵称或账号标识" value="' + esc(state.query) + '">' +
        "</label>" +
        '<div class="yyqd-chips" role="tablist">' +
          chips.map(function (chip) {
            return '<button type="button" role="tab" class="yyqd-chip' + (state.role === chip[0] ? " is-on" : "") + '" aria-selected="' + (state.role === chip[0]) + '" data-yyqd-role="' + chip[0] + '">' + esc(chip[1]) +
              '<b>' + chip[2] + "</b></button>";
          }).join("") +
        "</div>" +
      "</div>" +
      '<p class="yyqd-count" id="yyqd-count"></p>' +
      '<div class="yyqd-list" id="yyqd-list"></div>' +
      "</div>";
  }

  function rowHTML(row) {
    var key = roleOf(row);
    return '<button type="button" class="yyqd-row" data-yyqd-open="' + esc(row.id) + '">' +
      avatarHTML(row, false) +
      '<span class="yyqd-main"><b>' + esc(row.name) + "</b>" +
        '<small>' + esc(yyQQMaskID(row.id)) + "</small></span>" +
      '<span class="yyqd-role" data-role="' + key + '">' + esc(ROLE_TEXT[key]) + "</span>" +
      '<span class="yyqd-chevron" aria-hidden="true">›</span>' +
      "</button>";
  }

  function paint() {
    var list = document.getElementById("yyqd-list");
    if (!list) return;
    var rows = filteredRows();
    list.innerHTML = rows.length ? rows.map(rowHTML).join("") :
      '<div class="yyqd-empty"><b>' + (state.members.length ? "没有匹配的成员" : "还没有成员记录") + "</b><p>" +
      (state.members.length ? "换个昵称或身份再试。" : "让成员在群里 @机器人 发送 .身份，然后点上方「刷新群成员与权限」。") + "</p></div>";
    var count = document.getElementById("yyqd-count");
    if (count) count.textContent = rows.length + " / " + state.members.length + " 位成员 · 点任意一行进入身份与权限";
  }

  async function mount(force) {
    var host = document.getElementById("yy-qq-member-cards");
    if (!host) return;
    if (!force && host.querySelector("[data-yyqd]")) {
      paint();
      return;
    }
    // 接口优先，读不到再反抓 DOM；两条路都空就什么都不做——
    // 既不能给空列表，也不能在成员还没加载时把原始卡片抹掉。
    var rows = await fetchMembers();
    if (!rows.length) rows = collect(host);
    if (!rows.length) return;
    host.innerHTML = shellHTML();
    host.setAttribute("data-yyqd-mounted", "1");
    var input = host.querySelector("#yyqd-query");
    if (input) {
      input.addEventListener("input", function () {
        state.query = input.value;
        paint();
      });
    }
    host.querySelectorAll("[data-yyqd-role]").forEach(function (chip) {
      chip.addEventListener("click", function () {
        state.role = chip.getAttribute("data-yyqd-role");
        host.querySelectorAll("[data-yyqd-role]").forEach(function (other) {
          var on = other === chip;
          other.classList.toggle("is-on", on);
          other.setAttribute("aria-selected", String(on));
        });
        paint();
      });
    });
    paint();
  }

  // ---------- 成员详情底部面板 ----------
  function closeSheet() {
    if (sheetHost && sheetHost.parentNode) sheetHost.parentNode.removeChild(sheetHost);
    sheetHost = null;
  }

  function sheetHTML(row) {
    var key = roleOf(row);
    var name = esc(row.name);
    var id = esc(row.id);
    return '<div class="yyqd-sheet" role="dialog" aria-modal="true" aria-label="' + name + ' 的身份与权限">' +
      '<div class="yyqd-sheet-head">' +
        avatarHTML(row, true) +
        '<div><b>' + name + '</b><span class="yyqd-role" data-role="' + key + '">' + esc(ROLE_TEXT[key]) + "</span></div>" +
        '<button type="button" class="yyqd-sheet-close" data-yyqd-close aria-label="关闭">×</button>' +
      "</div>" +
      '<dl class="yyqd-kv"><div><dt>账号标识</dt><dd>' + id + "</dd></div>" +
        '<div><dt>群内身份</dt><dd>' + esc(ROLE_TEXT[key]) + "</dd></div>" +
        '<div><dt>权限范围</dt><dd>仅群服互通助手；与 Minecraft 服务器 OP 无关</dd></div></dl>' +
      '<div class="yyqd-group"><h4>授权</h4>' +
        '<div class="yyqd-acts">' +
          '<button type="button" class="yyqd-btn" data-yyqd-act="owner">设为服主</button>' +
          '<button type="button" class="yyqd-btn" data-yyqd-act="super">设为超级管理员</button>' +
          '<button type="button" class="yyqd-btn" data-yyqd-act="admin">设为管理员</button>' +
        "</div>" +
        '<p class="yyqd-note">服主、管理员、超级管理员是三份独立授权，可以同时存在；超级管理员权限高于管理员。</p></div>' +
      '<div class="yyqd-group"><h4>显示昵称</h4>' +
        '<div class="yyqd-acts"><button type="button" class="yyqd-btn" data-yyqd-act="nickname">修改显示昵称</button></div>' +
        '<p class="yyqd-note">当前昵称：' + name + "</p></div>" +
      '<div class="yyqd-group yyqd-danger"><h4>危险操作</h4>' +
        '<div class="yyqd-acts"><button type="button" class="yyqd-btn yyqd-btn-danger" data-yyqd-act="revoke">撤销授权</button></div>' +
        '<p class="yyqd-note">撤销会把 ' + name + " 从管理员与超级管理员名单中移除；服主身份与游戏账号不受影响。</p></div>" +
      '<p class="yyqd-status" data-yyqd-status></p>' +
      "</div>";
  }

  function openSheet(id) {
    var row = state.members.find(function (item) { return item.id === id; });
    if (!row) return;
    closeSheet();
    sheetHost = document.createElement("div");
    sheetHost.className = "yyqd-mask";
    sheetHost.innerHTML = '<div class="yyqd-mask-bg" data-yyqd-close></div>' + sheetHTML(row);
    document.body.appendChild(sheetHost);
    sheetHost.addEventListener("click", function (event) {
      if (event.target.closest("[data-yyqd-close]")) closeSheet();
      var act = event.target.closest("[data-yyqd-act]");
      if (act) runAction(act, row);
    });
  }

  function status(text, tone) {
    var el = sheetHost && sheetHost.querySelector("[data-yyqd-status]");
    if (!el) return;
    el.textContent = text;
    el.setAttribute("data-tone", tone || "");
  }

  async function refreshAfter() {
    try {
      if (typeof window.yyQQRefreshMembers === "function") await window.yyQQRefreshMembers();
    } catch (error) {
      // 刷新失败不影响已提交的权限，提示由入口统一处理。
    }
    mount(true);
  }

  async function runAction(button, row) {
    var act = button.getAttribute("data-yyqd-act");
    if (button.disabled) return;
    var cfg = (window.yyA && yyA.cfg) || {};

    if (act === "owner") {
      var box = document.getElementById("yy-qq-companion-owner");
      if (!box) {
        status("页面上找不到服主设置项，请刷新页面后重试。", "error");
        return;
      }
      box.dataset.user = row.id;
      box.value = row.name;
      if (typeof window.yyQQAssistantSave === "function") {
        button.disabled = true;
        status("正在保存服主…");
        await window.yyQQAssistantSave();
        button.disabled = false;
        status("已提交；请在页面顶部确认「服主昵称」已显示 " + row.name + "。", "ok");
        await refreshAfter();
      } else {
        status("已选择服主，请在页面里点「保存助手设置」。", "warn");
      }
      return;
    }

    if (act === "nickname") {
      var next = prompt("显示昵称（留空表示清除）", row.name);
      if (next === null) return;
      button.disabled = true;
      status("正在保存昵称…");
      var aliases = Object.assign({}, cfg.nicknames || {});
      var clean = next.trim().slice(0, 64);
      if (clean) aliases[row.id] = clean; else delete aliases[row.id];
      var saved = await api("POST", "/api/qq/config", { assistant: { nicknames: aliases } });
      button.disabled = false;
      if (!saved || saved.ok === false) {
        status("保存失败：" + ((saved && saved.error) || "接口无响应"), "error");
        return;
      }
      cfg.nicknames = aliases;
      status("已保存。", "ok");
      await refreshAfter();
      return;
    }

    if (act === "revoke") {
      var scope = row.role === "super" ? "超级管理员" : row.role === "admin" ? "管理员" : "（当前没有任何授权）";
      if (!confirm("撤销 " + row.name + " 的群服互通授权？\n\n即将撤销：" + scope + "\n影响范围：仅群服互通助手的群内指令权限\n不受影响：服主设置、Minecraft 服务器 OP、游戏账号\n\n此操作需要重新授权才能恢复。")) return;
    }

    var role = act === "revoke" ? "member" : act === "super" ? "super" : act === "admin" ? "admin" : act;
    button.disabled = true;
    status("正在提交…");
    try {
      var result = await api("POST", "/api/qq/assistant", { action: "authorize", user: row.id, role: role });
      if (!result || result.ok === false) throw new Error((result && result.error) || "接口无响应");
      var conf = await api("GET", "/api/qq/config");
      var assistant = (conf && conf.config && conf.config.assistant) || {};
      if (window.yyA) {
        yyA.cfg.super_admins = assistant.super_admins || [];
        yyA.cfg.admins = assistant.admins || [];
      }
      var sup = document.getElementById("yy-qq-super");
      var adm = document.getElementById("yy-qq-admins");
      if (sup) sup.value = (assistant.super_admins || []).join("\n");
      if (adm) adm.value = (assistant.admins || []).join("\n");
      status("已提交成功，正在同步最新权限…", "ok");
      await refreshAfter();
      var fresh = state.members.find(function (item) { return item.id === row.id; });
      var shown = fresh ? ROLE_TEXT[roleOf(fresh)] : "";
      status("已生效：" + row.name + " 现在是" + shown + "。", "ok");
    } catch (error) {
      status("提交失败：" + (error && error.message ? error.message : "未知错误"), "error");
    } finally {
      button.disabled = false;
    }
  }

  // 原刷新函数每次都会重写 #yy-qq-member-cards，监听它就能持续接管，
  // 不需要改那个 12000 字符的单行模板。
  var watching = false, pending = 0;
  // 监听整个 body：群服互通面板是后渲染的，容器出现的时间点不可预测；
  // 只盯容器本身会漏掉「容器还没出现」的那一段时间。
  function watch() {
    if (watching) return;
    watching = true;
    if (window.MutationObserver) {
      var observer = new MutationObserver(function () {
        if (pending) return;
        pending = setTimeout(function () {
          pending = 0;
          var host = document.getElementById("yy-qq-member-cards");
          if (!host) return;
          if (!host.querySelector("[data-yyqd]")) mount(true);
        }, 150);
      });
      observer.observe(document.body, { childList: true, subtree: true });
    }
    document.addEventListener("click", function (event) {
      var open = event.target.closest && event.target.closest("[data-yyqd-open]");
      if (open) openSheet(open.getAttribute("data-yyqd-open"));
    });
    document.addEventListener("keydown", function (event) {
      if (event.key === "Escape") closeSheet();
    });
  }

  window.YYQQDirectory = { mount: mount, open: openSheet, state: state };

  // 原刷新函数写完卡片就立刻接管：同一轮事件里换掉，避免和它来回抢 DOM。
  function hookRefresh() {
    var base = window.yyQQRefreshMembers;
    if (typeof base !== "function" || base.__yyqdHooked) return false;
    var wrapped = async function () {
      var result = await base.apply(this, arguments);
      try {
        var host = document.getElementById("yy-qq-member-cards");
        if (host) mount(true);
      } catch (error) {
        // 接管失败不影响原刷新结果
      }
      return result;
    };
    wrapped.__yyqdHooked = true;
    window.yyQQRefreshMembers = wrapped;
    return true;
  }

  function boot() {
    watch();
    hookRefresh();
    mount(false);
    // 兜底：原页面有自己的刷新节奏，DOM 观察器在某些 WebView 上会漏事件。
    // 800ms 检查一次容器是否被换回旧卡片，代价只有一次 getElementById。
    setInterval(function () {
      hookRefresh();
      var host = document.getElementById("yy-qq-member-cards");
      if (!host) return;
      if (!host.querySelector("[data-yyqd]")) mount(true);
    }, 800);
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot, { once: true });
  else boot();
})();
