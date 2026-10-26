// 互通端信息面板 · 玩家 / 进出记录 / 日志 / 说话
// ---------------------------------------------------------------------------
// 用户的原话：「停服互通的话，也是要有互通端的一些信息的。比如说列出玩家列表、
// 查看一些日志、玩家的皮肤头像、进出服务器的记录、服务器的名称查询……还能用
// 联通的机器人的身份去服务器里面说话、运行一些指令」。
//
// 后端接口（youyuan_qq_serverinfo.go）已经有了，这里只做入口与展示：
//   GET  /api/qq/server/info     服务器名 / 群 / 机器人资料 / 在线人数
//   GET  /api/qq/server/players  玩家列表（头像走 /api/skin/online-head）
//   GET  /api/qq/server/events   进服 / 退服时间轴
//   GET  /api/qq/server/logs     机器人动作 + 技能执行
//   POST /api/qq/server/say      {text, mode:chat|command}
//
// 挂载方式与 youyuan_qq_knowledge.js 完全一致：包装 yyQQRenderAll，
// 往 .yyq-fold-grid 里插一块折叠区，不改 console 本体。
// ---------------------------------------------------------------------------

(function () {
  "use strict";

  var yySI = {
    // loading 必须从 0 起：初始为 1 会让 mount 里「首次注入就拉」的守卫永远为假，
    // 折叠态摘要永远停在「读取中…」，用户不展开就一行数据都没有。
    // 首帧骨架改由 infoHead 的「未 loaded 就显示骨架」负责，观感不变。
    loading: 0, loaded: 0, err: "",
    info: null, players: [], events: [], logs: [],
    tab: "players",     // players | events | logs | say
    q: "",              // 玩家搜索词（纯前端过滤）
    page: 1,            // 玩家列表当前页（每页 5 条）
    per: 5,
    roles: {},          // QQ → 0/1/2，来自 /api/qq/roles，只用于列表徽章
    detail: null,       // 详情抽屉：{name, data, err}
    draft: "",          // 说话输入框的内容：面板每次重绘都要能还原，否则打字会被吞
    sending: "",        // "" | chat | command
    session: [],        // 本次会话的发送记录
    msg: "", bad: 0,
    open: 0, timer: 0
  };

  function esc(v) {
    return typeof yyQQE === "function" ? yyQQE(v == null ? "" : String(v)) : String(v == null ? "" : v);
  }
  function icon(n) {
    return typeof yyQQIcon === "function" ? yyQQIcon(n) : "";
  }

  // 秒 → 「1 小时 3 分」
  function dur(sec) {
    sec = Math.max(0, Number(sec) || 0);
    if (sec < 60) return sec + " 秒";
    var m = Math.floor(sec / 60);
    if (m < 60) return m + " 分钟";
    var h = Math.floor(m / 60);
    if (h < 24) return h + " 小时 " + (m % 60) + " 分";
    return Math.floor(h / 24) + " 天 " + (h % 24) + " 小时";
  }

  // 后端统一给 RFC3339，前端只负责换算成「3 分钟前」。
  function ago(ts) {
    if (!ts) return "";
    var t = new Date(ts).getTime();
    if (isNaN(t)) return "";
    var d = Math.floor((Date.now() - t) / 1000);
    if (d < 5) return "刚刚";
    if (d < 60) return d + " 秒前";
    if (d < 3600) return Math.floor(d / 60) + " 分钟前";
    if (d < 86400) return Math.floor(d / 3600) + " 小时前";
    return Math.floor(d / 86400) + " 天前";
  }
  function clock(ts) {
    if (!ts) return "";
    var d = new Date(ts);
    if (isNaN(d.getTime())) return "";
    function two(n) { return (n < 10 ? "0" : "") + n; }
    return (d.getMonth() + 1) + "-" + two(d.getDate()) + " " + two(d.getHours()) + ":" + two(d.getMinutes());
  }

  // 头像地址：后端给的是同源相对路径（/api/skin/online-head?player=…），
  // 这种直接请求就好。只有拿到外链时才套 /api/skin/proxy 走本地缓存——
  // proxy 只接受 http(s) 绝对地址，把相对路径塞进去反而会 400。
  function avaSrc(u) {
    u = String(u || "");
    if (!u) return "";
    if (/^https?:\/\//i.test(u)) return "/api/skin/proxy?url=" + encodeURIComponent(u);
    return u;
  }

  // 代理只是「加速层」，绝不能成为单点故障。
  // 之前代理白名单没放行网易域名（gph.netease.com），所有网易玩家头像被拒，
  // 前端 onerror 直接 remove，于是每行只剩一个色块——用户看到的就是
  // 「头像变成了一个色块，还动不动刷新」。现在失败就回退原图直连，
  // 原图也失败才隐藏，露出底下的首字母占位。
  // 完整链：同源代理 → 原图直连 → 移除 img 露出垫在底下的姓首字母色块。
  // 调用方负责永远渲染一个「首字母 <i> 垫底 + img 覆盖」的结构，
  // 这样任何一层失败都不留破图、不留空白。
  function avaImg(u, cls, extra) {
    var proxied = avaSrc(u);
    if (!proxied) return "";
    var orig = /^https?:\/\//i.test(String(u || "")) ? String(u) : "";
    return '<img' + (cls ? ' class="' + cls + '"' : "") + ' src="' + esc(proxied) + '"' +
      (orig ? ' data-orig="' + esc(orig) + '"' : "") +
      ' alt="" loading="lazy"' + (extra || "") + ' onerror="yySIAvaFail(this)">';
  }
  function avaFail(el) {
    if (!el) return;
    var orig = el.getAttribute("data-orig");
    if (orig && el.getAttribute("src") !== orig) {
      el.setAttribute("src", orig);
      el.removeAttribute("data-orig");
      return;
    }
    el.remove();
  }

  // 权限：列表只显示「非成员」，成员不戴徽章，避免每行都挂一个没信息量的标签。
  function roleOf(p) {
    if (p && p.role != null) return Number(p.role) || 0;
    var qq = p && p.qq ? String(p.qq) : "";
    return qq && yySI.roles[qq] ? Number(yySI.roles[qq]) || 0 : 0;
  }
  function roleName(r) {
    return r >= 2 ? "超管" : (r === 1 ? "管理员" : "成员");
  }
  function roleFullName(r) {
    return r >= 2 ? "超级管理员" : (r === 1 ? "管理员" : "成员");
  }

  // ---------- 数据 ----------

  function load() {
    yySI.loading = 1;
    yySI.err = "";
    render();
    var jobs = [
      A("GET", "/api/qq/server/info"),
      A("GET", "/api/qq/server/players?limit=200"),
      A("GET", "/api/qq/server/events?limit=60"),
      A("GET", "/api/qq/server/logs?limit=120"),
      A("GET", "/api/qq/roles")
    ];
    Promise.all(jobs.map(function (p) {
      return p && p.then ? p.catch(function () { return null; }) : Promise.resolve(null);
    })).then(function (rs) {
      yySI.loading = 0;
      yySI.loaded = 1;
      var info = rs[0], players = rs[1], events = rs[2], logs = rs[3], roles = rs[4];
      if (info && info.ok) {
        yySI.info = info;
      } else {
        yySI.info = null;
        yySI.err = (info && info.error) || "读取服务器信息失败";
      }
      yySI.players = (players && players.ok && players.players) || [];
      yySI.events = (events && events.ok && events.events) || [];
      yySI.logs = (logs && logs.ok && logs.logs) || [];
      yySI.roles = roleMap(roles);
      // 刷新后人数变少时，别停在已经不存在的页上。
      var maxPage = pageCount();
      if (yySI.page > maxPage) yySI.page = maxPage;
      // 三个列表的 error 也要原样展示，不能吞掉。
      var extra = [];
      if (players && !players.ok && players.error) extra.push("玩家：" + players.error);
      if (events && !events.ok && events.error) extra.push("进出记录：" + events.error);
      if (logs && !logs.ok && logs.error) extra.push("日志：" + logs.error);
      if (extra.length) yySI.err = extra.join("；");
      render();
    });
  }

  // ---------- 渲染 ----------

  function block() {
    var open = yySI.open ? " open" : "";
    return '<details class="yyq-fold" id="yyq-serverinfo"' + open + ' ontoggle="yySIFoldToggle(this)">' +
      '<summary><span class="yyq-fold-ico">' + icon("connection") + "</span>" +
      '<span class="yyq-fold-t">互通端信息</span>' +
      '<span class="yyq-fold-sum">' + esc(summaryText()) + "</span>" +
      '<span class="yyq-fold-caret">' + icon("arrow") + "</span></summary>" +
      '<div class="yyq-fold-body" id="yysi-body">' + body() + "</div>" +
      "</details>";
  }

  function summaryText() {
    if (yySI.loading) return "读取中…";
    if (!yySI.loaded) return "点开查看";
    if (!yySI.info) return "读取失败";
    var bits = [];
    bits.push(yySI.info.online == null ? "在线 —" : "在线 " + yySI.info.online);
    bits.push("玩家 " + (yySI.info.players || 0));
    return bits.join(" · ");
  }

  function body() {
    var head = infoHead();
    // 玩家总数不再占一个大格子：它变成 tab 右边的一行小字（在线 1 · 共 3 人）。
    var tabs = '<div class="yyq-tabs" role="tablist">' +
      tabBtn("players", "玩家") + tabBtn("events", "进出记录") +
      tabBtn("logs", "日志") + tabBtn("say", "说话") + tabMeta() + "</div>";
    var panel = yySI.tab === "events" ? eventsPanel()
      : yySI.tab === "logs" ? logsPanel()
        : yySI.tab === "say" ? sayPanel()
          : playersPanel();
    var msg = yySI.msg ? '<p class="yysi-msg" data-bad="' + (yySI.bad ? "1" : "0") + '">' + esc(yySI.msg) + "</p>" : "";
    // 完全读不到时，只显示一次错误 + 重试，不要再叠 4 个空 tab：
    // 之前是「错误条 + 空 tab + 空列表」，同一句错误出现两遍，看起来像坏了。
    if (!yySI.info && yySI.err) {
      return head +
        '<div class="yyq-fold-actions"><button type="button" class="btn-s" onclick="yySIReload()">重新读取</button></div>' +
        msg;
    }
    return head + tabs + panel + msg;
  }

  function tabBtn(id, label) {
    var on = yySI.tab === id;
    return '<button type="button" role="tab" aria-selected="' + (on ? "true" : "false") + '"' +
      ' class="' + (on ? "is-on" : "") + '" onclick="yySITab(\'' + id + '\')">' + esc(label) + "</button>";
  }

  // 顶部服务器名片：名字、群号、机器人身份、在线人数。
  function infoHead() {
    // 未 loaded（首次注入 / 重新读取中）显示骨架；loaded 之后读不到才显示错误空态。
    if (!yySI.loaded) return '<div class="yysi-skel">正在读取互通端信息…</div>';
    var i = yySI.info;
    if (!i) {
      return '<div class="yysi-empty">' + icon("connection") +
        '<strong>读不到互通端信息</strong><span>' + esc(yySI.err || "群服互通可能还没配置好") + "</span></div>";
    }
    var bot = i.bot || null;
    // 首字母占位永远垫在 img 底下：代理、原图直连全失败被移除后露出它，
    // 绝不在头像位置留一块空白（旧版只有 avatar 为空时才渲染占位）。
    var avatar = '<span class="yysi-face yysi-face-hero"><i>' +
      esc((bot && bot.nickname ? bot.nickname : "机").slice(0, 1)) + "</i>" +
      (bot && bot.avatar ? avaImg(bot.avatar, "") : "") + "</span>";
    var botName = bot && bot.nickname ? bot.nickname : (i.bot_player || "机器人未就绪");
    var botUin = bot && bot.uin ? "QQ " + bot.uin : (i.error || "");

    var cells = [
      ["服务器", i.name || "未命名"],
      ["群号", i.group_number || i.group_id || "未配置"],
      ["在线", i.online == null ? "未知" : i.online + " 人"],
      ["已绑定", (i.bindings || 0) + " 人"],
      ["适配器", i.adapter === "qqofficial" ? "QQ 官方" : (i.adapter || "未配置")]
    ];
    var grid = '<div class="yysi-cards">' + cells.map(function (c) {
      return '<div class="yysi-card"><span>' + esc(c[0]) + "</span><strong>" + esc(c[1]) + "</strong></div>";
    }).join("") + "</div>";

    var state = i.enabled
      ? (i.game_connected ? "已启用 · 游戏已连接" : "已启用 · 游戏未连接")
      : "互通未启用";
    var note = i.online_note ? '<p class="yysi-hint">' + esc(i.online_note) + "</p>" : "";

    return '<div class="yysi-hero">' + avatar +
      '<div class="yysi-hero-txt"><strong>' + esc(botName) + "</strong>" +
      "<small>" + esc(botUin) + "</small>" +
      '<em data-on="' + (i.game_connected ? "1" : "0") + '">' + esc(state) + "</em></div>" +
      '<button type="button" class="btn-s yysi-refresh" onclick="yySIReload()">刷新</button></div>' +
      grid + note;
  }

  // ---------- 玩家 ----------

  // /api/qq/roles 给的是 [{qq,name,role:"super_admin"}]，这里压成 QQ→0/1/2，
  // 只为了在列表上挂一个「管理员 / 超管」小徽章，成员不显示。
  function roleMap(r) {
    var out = {};
    if (!r || !r.ok) return out;
    (r.super_admins || []).forEach(function (v) {
      if (v && v.qq) out[String(v.qq)] = 2;
    });
    (r.admins || []).forEach(function (v) {
      if (v && v.qq && out[String(v.qq)] !== 2) out[String(v.qq)] = 1;
    });
    return out;
  }

  function filteredPlayers() {
    var q = (yySI.q || "").trim().toLowerCase();
    if (!q) return yySI.players;
    return yySI.players.filter(function (p) {
      return String(p.player || "").toLowerCase().indexOf(q) >= 0 ||
        String(p.qq || "").indexOf(q) >= 0;
    });
  }

  function pageCount() {
    return Math.max(1, Math.ceil(filteredPlayers().length / yySI.per));
  }

  // 当前页的 5 个人。列表和预加载都读这一份，避免两处算出不同的页。
  function pageRows() {
    var rows = filteredPlayers();
    var start = (yySI.page - 1) * yySI.per;
    return rows.slice(start, start + yySI.per);
  }

  // tab 右侧那行小字：在线 1 · 共 3 人。在线数拿不到可信值就写「—」。
  function tabMeta() {
    if (yySI.tab !== "players" || !yySI.loaded) return "";
    var on;
    if (yySI.info && yySI.info.online != null) {
      on = yySI.info.online;
    } else {
      var known = false, n = 0;
      yySI.players.forEach(function (p) {
        if (p.online === true) { n++; known = true; }
        else if (p.online === false) { known = true; }
      });
      on = known ? n : "—";
    }
    return '<span class="yysi-tabmeta">' + esc("在线 " + on + " · 共 " + filteredPlayers().length + " 人") + "</span>";
  }

  function playersPanel() {
    return '<div class="yysi-search">' +
      '<input id="yysi-q" type="search" placeholder="搜玩家名或QQ号" value="' + esc(yySI.q) + '" oninput="yySISearch(this.value)">' +
      "</div>" +
      '<div id="yysi-plist">' + playerList() + "</div>";
  }

  function playerList() {
    if (!yySI.players.length) {
      return '<div class="yysi-empty">' + icon("user") +
        "<strong>还没有玩家数据</strong><span>等机器人连上服务器后，这里会自动出现进过服的人。</span></div>";
    }
    if (!filteredPlayers().length) {
      return '<div class="yysi-empty">' + icon("user") +
        "<strong>没搜到这个人</strong><span>换个名字或QQ号试试，搜索只匹配已有的记录。</span></div>";
    }
    return '<ul class="yysi-people">' + pageRows().map(personRow).join("") + "</ul>" + pager();
  }

  // 一行只放三样：32px 头像 / 名字（+权限徽章）/ 在线点 + 最近一句。
  // 别的都收进详情抽屉——用户的原话是「别一大堆内容都塞在一起」。
  function personRow(p) {
    var st = p.online === true ? "on" : (p.online === false ? "off" : "unknown");
    var r = roleOf(p);
    var initial = String(p.player || "?").slice(0, 1).toUpperCase();
    // 没头像就不放 <img src="">——空 src 会让浏览器把当前页面当图片再请求一次。
    var img = avaImg(p.avatar, "");
    return '<li class="yysi-person" data-online="' + st + '" role="button" tabindex="0"' +
      ' data-name="' + esc(p.player) + '"' +
      ' onclick="yySIPlayerFrom(this)" onkeydown="yySIKey(event)">' +
      '<span class="yysi-face"><i>' + esc(initial) + "</i>" + img + "</span>" +
      '<div class="yysi-person-txt">' +
      '<span class="yysi-name-line"><strong>' + esc(p.player) + "</strong>" +
      (r > 0 ? '<em class="yysi-role" data-role="' + r + '">' + esc(roleName(r)) + "</em>" : "") +
      "</span>" +
      '<span class="yysi-sub"><i class="yysi-dot" data-online="' + st + '"></i>' +
      "<small>" + esc(p.last_message ? "「" + p.last_message + "」" : "还没有发言") + "</small></span>" +
      "</div></li>";
  }

  // 分页：只有一页时不显示，免得「第 1/1 页」白占一行。
  function pager() {
    var pages = pageCount();
    if (pages <= 1) return "";
    var total = filteredPlayers().length;
    return '<div class="yysi-pager">' +
      '<button type="button" class="yysi-page-btn" onclick="yySIPage(-1)"' +
      (yySI.page <= 1 ? " disabled" : "") + ">上一页</button>" +
      '<span class="yysi-page-info">第 ' + yySI.page + "/" + pages + " 页 · 共 " + total + " 人</span>" +
      '<button type="button" class="yysi-page-btn" onclick="yySIPage(1)"' +
      (yySI.page >= pages ? " disabled" : "") + ">下一页</button></div>";
  }

  // 预加载：只预热当前页这 5 张服务端渲染的玩家卡。用户明确要「点开就有」，
  // 所以列表一渲染就开始下，点开抽屉时直接命中浏览器缓存，不会先看到空框。
  function prefetchPage() {
    pageRows().forEach(function (p) {
      if (!p || !p.player) return;
      var img = new Image();
      img.src = "/api/qq/player-card?player=" + encodeURIComponent(p.player);
    });
  }

  // ---------- 详情抽屉 ----------

  function openPlayer(name) {
    if (!name) return;
    yySI.detail = { name: name, data: null, err: "" };
    renderDrawer();
    A("GET", "/api/qq/server/player?name=" + encodeURIComponent(name)).then(function (r) {
      // 期间用户可能已经点了别人：只认最后一次点击。
      if (!yySI.detail || yySI.detail.name !== name) return;
      if (!r || !r.ok) {
        yySI.detail.err = (r && r.error) || "读取玩家详情失败";
      } else {
        yySI.detail.data = r;
      }
      renderDrawer();
    })["catch"](function () {
      if (!yySI.detail || yySI.detail.name !== name) return;
      yySI.detail.err = "网络错误，详情没有读到";
      renderDrawer();
    });
  }

  function closePlayer() {
    yySI.detail = null;
    renderDrawer();
  }

  function renderDrawer() {
    var old = document.getElementById("yysi-drawer");
    if (!yySI.detail) {
      if (old && old.parentNode) old.parentNode.removeChild(old);
      return;
    }
    var html = drawerHTML(yySI.detail);
    if (old) {
      old.innerHTML = html;
      return;
    }
    var box = document.createElement("div");
    box.id = "yysi-drawer";
    box.className = "yysi-drawer";
    box.setAttribute("role", "dialog");
    box.setAttribute("aria-modal", "true");
    box.setAttribute("aria-label", "玩家详情");
    box.innerHTML = html;
    // 点遮罩关闭；点卡片内部不关（否则滑一下也会误关）。
    box.addEventListener("click", function (e) {
      if (e.target === box) closePlayer();
    });
    document.body.appendChild(box);
  }

  function drawerHTML(d) {
    var name = d.name;
    var data = d.data;
    var card = "/api/qq/player-card?player=" + encodeURIComponent(name);
    var r = data ? Number(data.role) || 0 : 0;
    var st = !data ? "unknown" : (data.online === true ? "on" : (data.online === false ? "off" : "unknown"));
    var stText = st === "on" ? "在线" : (st === "off" ? "离线" : "状态未知");

    // 形象预览用服务端已经渲染好的整张玩家卡（预加载过的就是这张）。
    // 首字母占位垫在底下：卡片渲染失败（is-bad 隐藏 img）时露出它，不留破图不留空白。
    var top = '<div class="yysi-dtop">' +
      '<div class="yysi-dart-box"><i>' + esc(String(name || "?").slice(0, 1).toUpperCase()) + "</i>" +
      '<img class="yysi-dart" src="' + esc(card) + '" alt="' + esc(name) + ' 的玩家形象" loading="eager"' +
      ' onerror="this.classList.add(\'is-bad\')"></div>' +
      '<div class="yysi-did"><strong>' + esc(name) + "</strong>" +
      '<span class="yysi-dmeta">' +
      (data ? '<em class="yysi-role" data-role="' + r + '">' + esc(roleFullName(r)) + "</em>" : "") +
      '<em class="yysi-live" data-online="' + st + '">' + esc(stText) + "</em></span>" +
      "</div></div>";

    var body;
    if (!data) {
      body = '<p class="yysi-dload">' + (d.err ? esc(d.err) : "读取中…") + "</p>";
      if (d.err) {
        body += '<div class="yysi-dact"><button type="button" class="btn-s" onclick="yySIPlayer(\'' +
          esc(name.replace(/\\/g, "\\\\").replace(/'/g, "\\'")) + "')\">重试</button></div>";
      }
    } else {
      var cells = [
        ["QQ", data.qq || "未绑定"],
        ["在线时长", dur(data.online_seconds)],
        ["发言数", (data.messages || 0) + " 条"],
        ["最近发言", data.last_message || "还没有说过话"]
      ];
      body = '<div class="yysi-dgrid">' + cells.map(function (c) {
        return '<div class="yysi-dcell"><span>' + esc(c[0]) + "</span><strong>" +
          esc(c[1]) + "</strong></div>";
      }).join("") + "</div>" + levelBlock(data.level) + tagsBlock(data);
      if (data.note) body += '<p class="yysi-hint">' + esc(data.note) + "</p>";
    }

    return '<div class="yysi-dsheet">' +
      '<button type="button" class="yysi-dx" onclick="yySIDetailClose()" aria-label="关闭">×</button>' +
      top + body + "</div>";
  }

  // 积分：只放 3 个关键项（等级 / 本级经验 / 总经验）。
  // 没有开启或查不到就明确说一句，不留空块——空块比没有更让人困惑。
  function levelBlock(lv) {
    if (!lv) return "";
    if (!lv.enabled) {
      return '<div class="yysi-dsec"><h4>积分</h4><p class="yysi-hint">未启用积分</p></div>';
    }
    if (lv.note) {
      return '<div class="yysi-dsec"><h4>积分</h4><p class="yysi-hint">' + esc(lv.note) + "</p></div>";
    }
    var items = [
      ["等级", "Lv." + (lv.level || 1)],
      ["本级经验", (lv.cur || 0) + " / " + (lv.need || 0)],
      ["总经验", String(lv.xp || 0)]
    ];
    return '<div class="yysi-dsec"><h4>积分</h4><div class="yysi-dkv">' +
      items.map(function (c) {
        return '<span><em>' + esc(c[0]) + "</em><b>" + esc(c[1]) + "</b></span>";
      }).join("") + "</div></div>";
  }

  // 标签：仓库里目前没有任何「玩家标签」的数据源（插件原生口还没接进来），
  // 所以后端固定返回空数组，这里只显示一枚「暂无标签」胶囊，绝不编数据。
  function tagsBlock(d) {
    var tags = d.tags || [];
    if (!tags.length) {
      return '<div class="yysi-dsec"><h4>标签</h4>' +
        '<span class="yysi-tag is-empty">暂无标签</span>' +
        '<p class="yysi-hint">' + esc(d.tags_note || "插件原生标签接口待接入") + "</p></div>";
    }
    return '<div class="yysi-dsec"><h4>标签</h4><div class="yysi-tags">' +
      tags.map(function (t) {
        return '<span class="yysi-tag">' + esc(t) + "</span>";
      }).join("") + "</div></div>";
  }

  // ---------- 进出记录 ----------

  // 玩家名 → 头像地址，取自 /api/qq/server/players 的 p.avatar 字段
  // （后端给的是同源相对路径 /api/skin/online-head?player=…）。
  function avatarOf() {
    var out = {};
    yySI.players.forEach(function (p) {
      if (p && p.player && p.avatar) out[String(p.player)] = p.avatar;
    });
    return out;
  }
  // 进出记录的后端接口不返回 avatar（youyuan_qq_serverinfo.go 的 event 结构体
  // 只有 time/type/player/qq），玩家又可能超出 players 的 limit=200 覆盖范围，
  // 所以按玩家头像的同款规则现场拼地址，交给 avaImg 走「代理/直连/占位」链。
  function eventHeadURL(name) {
    return name ? "/api/skin/online-head?player=" + encodeURIComponent(name) : "";
  }

  function eventsPanel() {
    if (!yySI.events.length) {
      return '<div class="yysi-empty">' + icon("clock") +
        "<strong>还没有进出记录</strong><span>机器人连上服务器后，谁进来、谁离开都会记在这里。</span></div>";
    }
    var avatars = avatarOf();
    return '<ol class="yysi-timeline">' + yySI.events.map(function (e) {
      var kind = e.type === "leave" ? "leave" : (e.type === "first_join" ? "first" : "join");
      var label = kind === "leave" ? "离开服务器" : (kind === "first" ? "首次出现" : "进入服务器");
      var name = String(e.player || "");
      // 头像链与玩家行完全一致：players 已拿到的地址优先，现场拼的兜底；
      // 两层都失败时 avaFail 移除 img，露出垫在底下的姓首字母色块——
      // 不留破图、不留空白。
      var face = '<span class="yysi-face yysi-face-ev"><i>' +
        esc((name || "?").slice(0, 1).toUpperCase()) + "</i>" +
        avaImg(avatars[name] || eventHeadURL(name), "") + "</span>";
      return '<li class="yysi-ev" data-kind="' + kind + '">' +
        '<i class="yysi-dot"></i>' + face +
        '<div class="yysi-ev-txt"><strong>' + esc(e.player) + "</strong>" +
        "<span>" + esc(label) + (e.qq ? " · QQ " + esc(e.qq) : "") + "</span></div>" +
        '<time datetime="' + esc(e.time) + '" title="' + esc(clock(e.time)) + '">' + esc(ago(e.time)) + "</time>" +
        "</li>";
    }).join("") + "</ol>";
  }

  // ---------- 日志 ----------

  function logsPanel() {
    if (!yySI.logs.length) {
      return '<div class="yysi-empty">' + icon("list") +
        "<strong>还没有动作记录</strong><span>机器人在群里做过的操作（点赞、动态、执行指令）会出现在这里。</span></div>";
    }
    return '<ul class="yysi-logs">' + yySI.logs.map(function (l) {
      var detail = l.detail || l.error || "";
      return '<li class="yysi-log" data-ok="' + (l.ok ? "1" : "0") + '">' +
        '<i class="yysi-dot"></i>' +
        '<div class="yysi-log-txt"><strong>' + esc(l.kind || "动作") +
        (l.source === "skill" ? ' <em>技能</em>' : "") + "</strong>" +
        (l.target ? "<span>" + esc(l.target) + "</span>" : "") +
        (detail ? '<p>' + esc(detail) + "</p>" : "") + "</div>" +
        '<time title="' + esc(clock(l.time)) + '">' + esc(ago(l.time)) + "</time></li>";
    }).join("") + "</ul>" +
      '<p class="yysi-hint">只显示机器人的动作与技能执行记录；群聊内容走实时推送，不在这里留档。</p>';
  }

  // ---------- 说话 ----------

  function sayPanel() {
    var busy = yySI.sending ? " disabled" : "";
    var list = "";
    if (yySI.session.length) {
      list = '<ul class="yysi-sent">' + yySI.session.map(function (s) {
        return '<li data-ok="' + (s.ok ? "1" : "0") + '">' +
          '<span class="yysi-sent-tag">' + (s.mode === "command" ? "指令" : "发言") + "</span>" +
          '<span class="yysi-sent-text">' + esc(s.text) + "</span>" +
          (s.ok ? "" : '<small class="yysi-sent-err">' + esc(s.error || "失败") + "</small>") +
          '<time>' + esc(clock(s.at)) + "</time></li>";
      }).join("") + "</ul>";
    } else {
      list = '<p class="yysi-hint">还没发过消息。发出去的内容会记在下面，只保存在本页面。</p>';
    }

    return '<div class="yysi-say">' +
      '<textarea id="yysi-input" rows="3" maxlength="500" placeholder="以机器人身份对全服说话，例如：今晚八点开活动"' +
      ' oninput="yySIDraft(this.value)">' + esc(yySI.draft) + "</textarea>" +
      '<div class="yysi-say-row">' +
      '<button type="button" class="btn-s" onclick="yySISay(\'chat\')"' + busy + ">发消息</button>" +
      '<button type="button" class="btn-s" onclick="yySISay(\'command\')"' + busy + ">执行指令</button>" +
      "</div>" +
      '<p class="yysi-danger">执行指令请以 / 开头。op、deop、stop、kick、ban、关白名单、超大 fill 等危险命令会被直接拒绝。</p>' +
      list + "</div>";
  }

  function render() {
    var el = document.getElementById("yysi-body");
    if (!el) return;
    el.innerHTML = body();
    var sum = document.querySelector("#yyq-serverinfo .yyq-fold-sum");
    if (sum) sum.textContent = summaryText();
  }

  // 搜索只重画列表，避免整块重绘把输入框焦点冲掉。
  function renderPlayers() {
    var list = document.getElementById("yysi-plist");
    if (list) list.innerHTML = playerList();
    var cnt = document.getElementById("yysi-pcount");
    if (cnt) cnt.textContent = playerCountText();
  }

  // ---------- 交互 ----------

  function foldToggle(el) {
    yySI.open = el.open ? 1 : 0;
    if (yySI.open) {
      // 与 mount 同一套守卫：没拉过才拉，已在飞就不重复请求。
      if (!yySI.loaded && !yySI.loading) load();
      startTimer();
    } else {
      stopTimer();
    }
  }

  // 折叠区展开时每 30 秒静默刷新一次：在线人数是会变的，用户不该反复点刷新。
  function startTimer() {
    stopTimer();
    yySI.timer = setInterval(function () {
      if (!yySI.open) { stopTimer(); return; }
      if (yySI.sending) return;
      load();
    }, 30000);
  }
  function stopTimer() {
    if (yySI.timer) {
      clearInterval(yySI.timer);
      yySI.timer = 0;
    }
  }

  function tab(id) {
    yySI.tab = id;
    render();
  }

  function reload() {
    yySI.loaded = 0;
    load();
  }

  function search(v) {
    yySI.q = v || "";
    renderPlayers();
  }

  function flash(text, bad) {
    yySI.msg = text || "";
    yySI.bad = bad ? 1 : 0;
    render();
    if (text) {
      setTimeout(function () {
        if (yySI.msg === text) {
          yySI.msg = "";
          render();
        }
      }, 5000);
    }
  }

  function say(mode) {
    var text = (yySI.draft || "").trim();
    if (!text) {
      flash("先写点内容再发", 1);
      return;
    }
    // 前端先挡一次，省一次往返；真正的安全边界在后端。
    if (mode === "command" && text.charAt(0) !== "/") {
      flash("执行指令要以 / 开头，例如 /list", 1);
      return;
    }
    yySI.sending = mode;
    render();
    A("POST", "/api/qq/server/say", { text: text, mode: mode }).then(function (r) {
      yySI.sending = "";
      var ok = !!(r && r.ok);
      yySI.session.unshift({
        at: Date.now(),
        mode: mode,
        text: text,
        ok: ok,
        error: (r && r.error) || ""
      });
      if (yySI.session.length > 20) yySI.session.length = 20;
      if (ok) yySI.draft = "";   // 失败时保留原文，方便改一改再发
      render();
      if (!ok) flash((r && r.error) || "发送失败", 1);
      else if (r && r.note) flash(r.note, 0);
      else flash("已发送", 0);
    })["catch"](function () {
      yySI.sending = "";
      render();
      flash("网络错误，消息没有发出去", 1);
    });
  }

  function draft(v) {
    yySI.draft = v || "";
  }

  // ---------- 挂载 ----------

  function mount() {
    if (typeof window.yyQQRegisterModule !== "function") return;
    window.yyQQRegisterModule({
      id: "records",
      __key: "serverinfo",
      order: 10,
      render: function () { return block(); },
      // 首次展开就拉：否则折叠状态下摘要永远「读取中…」，像卡死。
      load: function () { if (!yySI.loaded && !yySI.loading) load(); }
    });
  }

  // inline onclick 需要全局函数
  // ---------- 内联事件入口 ----------
  //
  // 这一组函数之前**根本没定义也没导出**，而 personRow / pager 里已经写了
  // onclick="yySIPlayerFrom(this)"、onclick="yySIPage(-1)"。
  // 结果是玩家列表整行点了没反应、翻页按钮也点不动——
  // 界面上看着有按钮，实际全是死的。这里把它们补齐并挂到 window。
  function playerFrom(el) {
    if (!el) return;
    openPlayer(el.getAttribute("data-name") || "");
  }
  function keyHandler(e) {
    if (!e) return;
    // 键盘用户：Enter / 空格等同于点击（li 上有 tabindex，本就该能操作）。
    if (e.key === "Enter" || e.key === " " || e.key === "Spacebar") {
      if (e.preventDefault) e.preventDefault();
      playerFrom(e.currentTarget || e.target);
    }
  }
  function pageStep(delta) {
    var pages = pageCount();
    var next = (Number(yySI.page) || 1) + Number(delta || 0);
    if (next < 1) next = 1;
    if (next > pages) next = pages;
    if (next === yySI.page) return;
    yySI.page = next;
    render();
    // 翻页后把列表滚回顶部，否则手机上还停在上一页的位置，看着像没反应。
    var host = document.getElementById("yysi-players") || document.querySelector(".yysi-list");
    if (host && host.scrollIntoView) {
      try { host.scrollIntoView({ block: "start", behavior: "smooth" }); } catch (e) { host.scrollIntoView(); }
    }
  }

  window.yySIAvaFail = avaFail;
  window.yySIPlayerFrom = playerFrom;
  window.yySIKey = keyHandler;
  window.yySIPlayer = openPlayer;
  window.yySIDetailClose = closePlayer;
  window.yySIPage = pageStep;
  window.yySIFoldToggle = foldToggle;
  window.yySITab = tab;
  window.yySIReload = reload;
  window.yySISearch = search;
  window.yySISay = say;
  window.yySIDraft = draft;

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount);
  } else {
    mount();
  }
})();
