// 群服互通 · 机器人控制台
//
// 设计：把「配置表单」换成「控制台」。先回答"现在什么状态"，再让人做最小的动作改变状态。
// 分区：0 状态带 → 1 身份卡（含扫码登录）→ 2 能力矩阵 + 3 实时流 → 4 接入 / 5 人设 → 6 收藏表情
// 数据：/api/qq/{config,status,login,caps,actions,messages,bot/profile,faces,groups/available}
//
// 图标：本文件自带局部映射表（yyQQIconPaths），不依赖 youyuan_dashboard.js 的加载顺序；
//       表里没有的名字回落到全局 yyIconPaths。

var yyQQIconPaths = {
  box: '<path d="m12 3 9 5-9 5-9-5 9-5Z"/><path d="M3 8v9l9 5 9-5V8M12 13v9M7.5 5.5l9 5"/>',
  robot: '<rect x="4" y="8" width="16" height="11" rx="3"/><path d="M12 4v4"/><circle cx="12" cy="3" r="1"/><circle cx="9.5" cy="13" r="1.3"/><circle cx="14.5" cy="13" r="1.3"/><path d="M9.5 16.5h5"/>',
  power: '<path d="M12 3v9"/><path d="M6.5 6.6a8 8 0 1 0 11 0"/>',
  qr: '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><path d="M14 14h3v3h-3zM20 14v2M14 20h2M20 19v2"/>',
  activity: '<path d="M3 12h4l2.5-7 4 14 2.5-7H21"/>',
  sliders: '<path d="M4 8h9M18 8h2M4 16h3M12 16h8"/><circle cx="15.5" cy="8" r="2.2"/><circle cx="9.5" cy="16" r="2.2"/>',
  face: '<circle cx="12" cy="12" r="9"/><path d="M8.5 14.5a4.5 4.5 0 0 0 7 0M9 9.6v.01M15 9.6v.01"/>',
  refresh: '<path d="M20 12a8 8 0 1 1-2.4-5.7"/><path d="M20 4.5V9h-4.5"/>',
  ban: '<circle cx="12" cy="12" r="9"/><path d="m6 6 12 12"/>',
  key: '<circle cx="8" cy="12" r="4"/><path d="M12 12h9M17.5 12v3M15 12v2"/>',
  thumb: '<path d="M7 11v9H4.5A1.5 1.5 0 0 1 3 18.5v-6A1.5 1.5 0 0 1 4.5 11H7Z"/><path d="M7 11l3.6-7.2A2 2 0 0 1 12.6 4v4h4.6a2 2 0 0 1 2 2.4l-1.3 6.4A2 2 0 0 1 15.9 20H7"/>',
  tag: '<path d="M3 12.4V5a2 2 0 0 1 2-2h7.4a2 2 0 0 1 1.4.6l7.6 7.6a2 2 0 0 1 0 2.8l-6 6a2 2 0 0 1-2.8 0L3.6 13.8a2 2 0 0 1-.6-1.4Z"/><circle cx="8" cy="8" r="1.4"/>',
  trash: '<path d="M4 7h16M9.5 7V4.8h5V7M6.5 7l.9 13h9.2l.9-13"/>',
  more: '<circle cx="5" cy="12" r="1.4"/><circle cx="12" cy="12" r="1.4"/><circle cx="19" cy="12" r="1.4"/>',
  check: '<path d="m5 12.5 4.5 4.5L19 7"/>',
  send: '<path d="m3 11 18-8-8 18-2-8-8-2ZM11 13 21 3"/>',
  chat: '<path d="M21 11a8 8 0 0 1-8 8H7l-5 3 1.5-6A8 8 0 1 1 21 11Z"/><path d="M7 10h10M7 14h6"/>',
  image: '<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8" cy="8" r="1.5"/><path d="m3 17 5-5 4 4 4-6 5 7"/>',
  camera: '<path d="M3 7h4l2-3h6l2 3h4v13H3V7Z"/><circle cx="12" cy="13" r="4"/>',
  pen: '<path d="m15 4 5 5M4 20l5-1L21 7l-4-4L5 15l-1 5Z"/>',
  list: '<path d="M9 5h12M9 12h12M9 19h12M3 5h1M3 12h1M3 19h1"/>',
  connection: '<rect x="7" y="3" width="10" height="7" rx="1"/><rect x="2" y="17" width="8" height="5" rx="1"/><rect x="14" y="17" width="8" height="5" rx="1"/><path d="M12 10v4M6 17v-3h12v3"/>',
  arrow: '<path d="M5 12h14m-5-5 5 5-5 5"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  spark: '<path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5L12 3Z"/>',
  user: '<circle cx="12" cy="7" r="4"/><path d="M4 21v-2a6 6 0 0 1 6-6h4a6 6 0 0 1 6 6v2"/>'
};

function yyQQIcon(name) {
  var p = yyQQIconPaths[name];
  if (!p && typeof yyIconPaths !== "undefined") p = yyIconPaths[name];
  if (!p) p = yyQQIconPaths.box;
  return '<svg class="yy-icon" viewBox="0 0 24 24" aria-hidden="true">' + p + "</svg>";
}

// 能力 kind → 图标 / 分组。后端 Icon 字段是 emoji，前端一律不用。
var yyQQCapMap = {
  like: { group: "interact", icon: "thumb" },
  emoji_like: { group: "interact", icon: "face" },
  face: { group: "interact", icon: "image" },
  qzone: { group: "identity", icon: "send" },
  delete_qzone: { group: "identity", icon: "trash" },
  avatar: { group: "identity", icon: "camera" },
  long_nick: { group: "identity", icon: "pen" },
  nick: { group: "identity", icon: "tag" },
  kick: { group: "manage", icon: "ban" },
  relay_history: { group: "manage", icon: "list" }
};
var yyQQCapGroups = [
  { id: "all", label: "全部" },
  { id: "interact", label: "互动" },
  { id: "identity", label: "形象" },
  { id: "manage", label: "管理" }
];

var yyQQC = {
  cfg: {}, status: {}, login: null, caps: [], profile: null, faces: [],
  actions: [], messages: [],
  capGroup: "all", streamTab: "all", openCap: "",
  adapter: "onebot", picked: "",
  groups: [], groupsLoaded: false, groupsLoading: false, groupsErr: "",
  facesLoaded: false, facesLoading: false, facesErr: "",
  loading: { caps: 1, stream: 1 },
  msgErr: "", coreErr: "",
  accessOpen: false, personaOpen: false, facesOpen: false,
  sandbox: [], busy: {}, menu: false,
  qrBase: 0, qrFetchedAt: 0, lastQrUrl: "", timers: []
};

function yyQQE(v) {
  return typeof yyEscape === "function" ? yyEscape(v == null ? "" : String(v)) : String(v == null ? "" : v).replace(/[&<>"']/g, function (c) {
    return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
  });
}

// ---------- 小工具 ----------

function yyQQMinutes(sec) {
  if (!sec) return "不限";
  if (sec < 60) return sec + " 秒";
  if (sec % 60 === 0) return (sec / 60) + " 分";
  return Math.round(sec / 60) + " 分";
}
function yyQQAgo(ts) {
  if (!ts) return "";
  var d = Math.floor((Date.now() - ts) / 1000);
  if (d < 5) return "刚刚";
  if (d < 60) return d + " 秒前";
  if (d < 3600) return Math.floor(d / 60) + " 分钟前";
  if (d < 86400) return Math.floor(d / 3600) + " 小时前";
  return Math.floor(d / 86400) + " 天前";
}
function yyQQClock(ts) {
  if (!ts) return "";
  var d = new Date(ts);
  if (isNaN(d.getTime())) return "";
  var hh = d.getHours(), mm = d.getMinutes();
  return (hh < 10 ? "0" : "") + hh + ":" + (mm < 10 ? "0" : "") + mm;
}
// 底层错误（dial/refused/EOF）对用户没有意义，不直接展示。
function yyQQHuman(s) {
  if (!s) return "";
  if (/dial|refused|EOF|i\/o timeout|no such host|broken pipe|websocket:/i.test(s)) return "";
  return s;
}

function yyQQGroupsOf(cfg) {
  return ((cfg && cfg.groups) || []).filter(function (g) { return g && (g.id || g.group_id); });
}
function yyQQEnabledGroups() {
  return yyQQGroupsOf(yyQQC.cfg).filter(function (g) { return g.enabled !== false; });
}
function yyQQGroupName(g) {
  return g.name || g.group_name || g.id || g.group_id || "";
}
function yyQQGroupLabel() {
  var rows = yyQQEnabledGroups();
  if (!rows.length) return yyQQC.picked || "未绑定";
  return rows.map(yyQQGroupName).join("、");
}
function yyQQCapLabel(kind) {
  var c = null;
  (yyQQC.caps || []).forEach(function (x) { if (x && x.kind === kind) c = x; });
  return (c && c.label) || kind || "动作";
}
function yyQQCapIcon(kind) {
  var m = yyQQCapMap[kind];
  return (m && m.icon) || "spark";
}

// ---------- 状态推导 ----------

function yyQQView() {
  var cfg = yyQQC.cfg || {}, st = yyQQC.status || {}, lg = yyQQC.login || {};
  if (cfg.enabled === false) {
    return { key: "off", label: "已关闭", tone: "off", hint: "群服互通已停用，打开后机器人才会接入" };
  }
  if (lg.state === "need_login") {
    return { key: "need_login", label: "QQ 未登录", tone: "warn", hint: lg.hint || "用手机 QQ 扫描二维码完成授权" };
  }
  var s = st.state || "unconfigured";
  if (s === "open") return { key: "open", label: "已连接", tone: "ok", hint: "" };
  if (s === "connecting") return { key: "connecting", label: "正在接入", tone: "busy", hint: yyQQHuman(st.detail) || "正在与 OneBot 建立连接" };
  if (s === "reconnecting") return { key: "reconnecting", label: "重连中", tone: "busy", hint: yyQQHuman(st.detail) || "连接断开，正在自动重试" };
  if (s === "waiting_callback") return { key: "waiting_callback", label: "等待回调", tone: "busy", hint: "凭据已配置，等待 QQ 官方签名回调" };
  if (s === "error") return { key: "error", label: "连接失败", tone: "bad", hint: yyQQHuman(st.detail) || lg.hint || "连接中断，检查地址与 Token 后重试" };
  if (s === "stopped") return { key: "stopped", label: "已停止", tone: "off", hint: "机器人已停止，保存配置后会重新接入" };
  return { key: "unconfigured", label: "未配置", tone: "off", hint: "填写 NapCat 地址后开始接入" };
}
function yyQQLastMsgAt() {
  var t = 0;
  (yyQQC.messages || []).forEach(function (m) {
    var v = Date.parse(m.timestamp);
    if (v && v > t) t = v;
  });
  return t;
}

// ---------- 0 状态带 ----------

function yyQQStatusBar() {
  var v = yyQQView(), cfg = yyQQC.cfg || {};
  var meta = [cfg.adapter === "qqofficial" ? "QQ 官方" : "NapCat"];
  var n = yyQQEnabledGroups().length;
  if (n) meta.push(n + " 个群");
  var last = yyQQLastMsgAt();
  if (last) meta.push("最近消息 " + yyQQAgo(last));
  return '<div class="yyq-status" id="yyq-status" data-state="' + yyQQE(v.tone) + '">' +
    '<span class="yyq-dot" aria-hidden="true"></span>' +
    '<span class="yyq-status-text">' + yyQQE(v.label) + "</span>" +
    '<span class="yyq-status-meta">' + yyQQE(meta.join(" · ")) + "</span>" +
    (v.key === "need_login" || v.key === "unconfigured" || v.key === "error" || v.key === "off"
      ? '<button type="button" class="yyq-status-cta" onclick="yyQQOpenAccess()">去接入</button>'
      : "") +
    '<button type="button" class="yyq-iconbtn" onclick="yyQQRefreshAll()" aria-label="刷新状态" title="刷新状态">' + yyQQIcon("refresh") + "</button>" +
    '<button type="button" class="yyq-iconbtn" onclick="yyQQToggleMenu(event)" aria-label="更多操作" title="更多">' + yyQQIcon("more") + "</button>" +
    (yyQQC.menu ? '<div class="yyq-menu" role="menu">' +
      '<button type="button" role="menuitem" onclick="yyQQRefreshAll(1)">刷新全部数据</button>' +
      '<button type="button" role="menuitem" onclick="yyQQMenuFaces()">读取收藏表情</button>' +
      '<button type="button" role="menuitem" onclick="yyQQOpenAccess()">打开接入设置</button>' +
      "</div>" : "") +
    "</div>";
}

function yyQQToggleMenu(ev) {
  if (ev && ev.stopPropagation) ev.stopPropagation();
  yyQQC.menu = !yyQQC.menu;
  yyQQRender("status");
  if (yyQQC.menu) {
    setTimeout(function () {
      document.addEventListener("click", yyQQCloseMenu, { once: true });
    }, 0);
  }
}
function yyQQCloseMenu() {
  if (!yyQQC.menu) return;
  yyQQC.menu = false;
  yyQQRender("status");
}
function yyQQMenuFaces() {
  yyQQC.menu = false;
  yyQQRender("status");
  yyQQLoadFaces();
}
function yyQQOpenAccess() {
  yyQQC.accessOpen = true;
  var el = document.getElementById("yyq-access");
  if (el) el.setAttribute("open", "");
  yyQQRender("access");
  var box = document.getElementById("yyq-access");
  if (box && box.scrollIntoView) box.scrollIntoView({ behavior: "smooth", block: "center" });
}

// ---------- 1 身份卡 ----------

function yyQQHero() {
  var v = yyQQView(), p = yyQQC.profile || {}, cfg = yyQQC.cfg || {}, lg = yyQQC.login || {};
  // 头像：底层永远铺一个机器人图标，图片加载失败时自己移除，露出底图。
  var avatar = '<span class="yyq-avatar-wrap">' +
    '<span class="yyq-avatar-fb">' + yyQQIcon("robot") + "</span>" +
    (p.avatar ? '<img class="yyq-avatar" src="' + yyQQE(p.avatar) + '" alt="" referrerpolicy="no-referrer" onerror="this.remove()">' : "") +
    "</span>";

  var groups = yyQQEnabledGroups();
  var today = yyQQTodayActions();
  var facts = [
    ["互通群", groups.length ? groups.map(yyQQGroupName).join("、") : (yyQQC.picked || "未绑定")],
    ["游戏身份", (cfg.assistant && cfg.assistant.bot_player) || "未绑定"],
    ["好友 / 群", (p.friend_count == null ? "—" : p.friend_count) + " / " + (p.group_count == null ? "—" : p.group_count)],
    ["今日动作", today.total ? String(today.total) + (today.fail ? " 失败 " + today.fail : "") : "—"]
  ];

  var chain = [
    { ok: !!lg.ws_listening, name: "NapCat", sub: (cfg.onebot && cfg.onebot.ws_url) || "" },
    { ok: v.key === "open", name: "OneBot 桥接", sub: v.key === "open" ? "已握手" : v.label },
    { ok: groups.length > 0, name: groups.length ? groups.length + " 个群" : "未绑定群", sub: "" }
  ];

  return '<section class="yyq-hero" id="yyq-hero" data-state="' + yyQQE(v.tone) + '"' + (v.key === "need_login" ? ' data-qr="1"' : "") + ">" +
    '<div class="yyq-hero-main">' +
      '<div class="yyq-hero-id">' + avatar +
        '<div class="yyq-hero-name"><strong>' + yyQQE(p.nickname || "机器人资料未读取") + "</strong>" +
          '<div class="yyq-hero-sub"><span class="yyq-statechip" data-state="' + yyQQE(v.tone) + '">' + yyQQE(v.label) + "</span>" +
          "<span>" + yyQQE(p.user_id || (cfg.onebot && cfg.onebot.bot_id) || "QQ 未读取") + "</span></div>" +
        "</div>" +
        '<button type="button" class="yyq-ghost" onclick="yyQQOpenAccess()">切换群</button>' +
      "</div>" +
      '<dl class="yyq-facts">' + facts.map(function (f) {
        return "<div><dt>" + yyQQE(f[0]) + "</dt><dd>" + yyQQE(f[1]) + "</dd></div>";
      }).join("") + "</dl>" +
      '<ol class="yyq-chain" aria-label="链路状态">' + chain.map(function (c) {
        return '<li data-ok="' + (c.ok ? "1" : "0") + '"><span>' + yyQQE(c.name) + "</span>" + (c.sub ? "<small>" + yyQQE(c.sub) + "</small>" : "") + "</li>";
      }).join("") + "</ol>" +
      (v.hint ? '<p class="yyq-hero-hint">' + yyQQE(v.hint) + "</p>" : "") +
    "</div>" +
    (v.key === "need_login" ? yyQQQRCode(lg) : "") +
  "</section>";
}

function yyQQTodayActions() {
  var start = new Date();
  start.setHours(0, 0, 0, 0);
  var total = 0, fail = 0;
  (yyQQC.actions || []).forEach(function (a) {
    var t = Date.parse(a.time);
    if (!t || t < start.getTime()) return;
    total++;
    if (!a.ok) fail++;
  });
  return { total: total, fail: fail };
}

function yyQQQRCode(lg) {
  var q = (lg && lg.qrcode) || {};
  var has = !!q.data_url;
  return '<div class="yyq-qr" id="yyq-qr" data-fresh="' + (q.fresh ? "1" : "0") + '">' +
    '<div class="yyq-qr-box">' +
      (has
        ? '<img src="' + yyQQE(q.data_url) + '" alt="QQ 登录二维码" decoding="async">'
        : '<span class="yyq-qr-none">' + yyQQIcon("qr") + "<small>暂未取到二维码</small></span>") +
      (has && !q.fresh ? '<span class="yyq-qr-mask">正在刷新…</span>' : "") +
    "</div>" +
    '<p class="yyq-qr-hint">' + (has ? '二维码 <b id="yyq-qr-age">' + (q.age_seconds || 0) + "</b> 秒前生成，过期会自动刷新" : "等待 NapCat 生成二维码") + "</p>" +
    '<ol class="yyq-qr-steps"><li class="is-on">手机 QQ 扫一扫</li><li>在手机上确认登录</li><li>机器人自动上线</li></ol>' +
    "</div>";
}

// ---------- 2 能力矩阵 ----------

function yyQQCaps() {
  var caps = yyQQC.caps || [];
  var group = yyQQC.capGroup || "all";
  var counts = {};
  caps.forEach(function (c) {
    var g = (yyQQCapMap[c.kind] || {}).group || "manage";
    counts[g] = (counts[g] || 0) + 1;
  });
  counts.all = caps.length;

  var head = '<div class="yyq-caps-head">' +
    '<h3>' + yyQQIcon("sliders") + "能力<span>" + caps.length + "</span></h3>" +
    '<p class="yyq-caps-note">关闭后群里的对应指令会被直接拒绝</p>' +
    "</div>" +
    '<div class="yyq-tabs" role="tablist">' + yyQQCapGroups.map(function (g) {
      return '<button type="button" role="tab" aria-selected="' + (group === g.id ? "true" : "false") + '"' +
        ' class="' + (group === g.id ? "is-on" : "") + '" data-group="' + g.id + '" onclick="yyQQSetCapGroup(\'' + g.id + '\')">' +
        yyQQE(g.label) + "<small>" + (counts[g.id] || 0) + "</small></button>";
    }).join("") + "</div>";

  var body;
  if (yyQQC.loading.caps) {
    body = '<div class="yyq-cap-list is-loading">' + yyQQSkeleton(6, 60) + "</div>";
  } else if (!caps.length) {
    body = yyQQEmpty("sliders", "没有读到能力列表", "机器人接口未返回能力定义，稍后重试", "重试", "yyQQRefreshAll()");
  } else {
    var rows = caps.filter(function (c) {
      return group === "all" || ((yyQQCapMap[c.kind] || {}).group || "manage") === group;
    });
    body = '<ul class="yyq-cap-list">' + rows.map(yyQQCapRow).join("") + "</ul>";
  }

  return '<section class="yyq-caps" id="yyq-caps" aria-label="机器人能力">' + head + body +
    '<p class="yyq-cap-foot">开关即时生效 · 所有动作均留痕</p></section>';
}

function yyQQCapRow(c) {
  var kind = c.kind || "";
  var on = !!c.enabled;
  var limits = [];
  if (c.cool_seconds) limits.push("冷却 " + yyQQMinutes(c.cool_seconds));
  if (c.daily) limits.push("每日 " + c.daily + " 次");
  if (!limits.length) limits.push("不限流");
  var open = yyQQC.openCap === kind;
  var busy = yyQQC.busy["cap:" + kind];
  return '<li class="yyq-cap" data-kind="' + yyQQE(kind) + '" data-on="' + (on ? "1" : "0") + '" data-open="' + (open ? "1" : "0") + '"' +
      ' tabindex="0" onclick="yyQQCapDetail(event,this)" onkeydown="yyQQCapKey(event,this)" aria-expanded="' + (open ? "true" : "false") + '">' +
    '<span class="yyq-cap-ico">' + yyQQIcon(yyQQCapIcon(kind)) + "</span>" +
    '<span class="yyq-cap-body"><strong>' + yyQQE(c.label || kind) + "</strong>" +
      "<small>" + yyQQE(c.desc || "") + "</small></span>" +
    '<span class="yyq-cap-meta">' + yyQQE(limits.join(" · ")) + "</span>" +
    '<label class="yyq-sw" data-busy="' + (busy ? "1" : "0") + '"><input type="checkbox"' + (on ? " checked" : "") +
      ' onchange="yyQQToggleCap(this)" aria-label="' + yyQQE((c.label || kind) + " 开关") + '"><span></span></label>' +
    '<span class="yyq-cap-more">' +
      '<span class="yyq-cap-lv">' + yyQQE(c.min_level_text || "成员") + "起可用</span>" +
      (c.command ? '<code class="yyq-cap-cmd">' + yyQQE(c.command) + "</code>" : "") +
      '<span class="yyq-cap-state">' + (on ? "运行中" : "已停用") + "</span>" +
    "</span></li>";
}

function yyQQCapDetail(ev, li) {
  if (ev && ev.target && ev.target.closest && ev.target.closest(".yyq-sw")) return;
  var open = li.getAttribute("data-open") === "1";
  li.setAttribute("data-open", open ? "0" : "1");
  li.setAttribute("aria-expanded", open ? "false" : "true");
  yyQQC.openCap = open ? "" : li.getAttribute("data-kind");
}
function yyQQCapKey(ev, li) {
  if (ev.key !== "Enter" && ev.key !== " ") return;
  ev.preventDefault();
  yyQQCapDetail(ev, li);
}
function yyQQSetCapGroup(id) {
  yyQQC.capGroup = id;
  yyQQRender("caps");
}

async function yyQQToggleCap(input) {
  var li = input.closest ? input.closest(".yyq-cap") : null;
  var kind = li && li.getAttribute("data-kind");
  if (!kind) return;
  var want = !!input.checked;
  var patch = {};
  patch[kind] = want;
  yyQQC.busy["cap:" + kind] = 1;
  if (li) li.querySelector(".yyq-sw").setAttribute("data-busy", "1");
  var r = await A("POST", "/api/qq/caps", { caps: patch });
  delete yyQQC.busy["cap:" + kind];
  if (!r || !r.ok) {
    input.checked = !want;
    if (li) li.querySelector(".yyq-sw").setAttribute("data-busy", "0");
    T((r && r.error) || "保存失败", "e");
    return;
  }
  if (r.caps) yyQQC.caps = r.caps;
  else (yyQQC.caps || []).forEach(function (c) { if (c.kind === kind) c.enabled = want; });
  yyQQRender("caps");
  T((want ? "已开启 " : "已关闭 ") + yyQQCapLabel(kind), "o");
}

// ---------- 3 实时流 ----------

function yyQQTimeline() {
  var rows = [];
  (yyQQC.messages || []).forEach(function (m) {
    var t = Date.parse(m.timestamp) || 0;
    rows.push({
      t: t, kind: m.is_bot ? "out" : "in", ok: 1,
      icon: m.is_bot ? "send" : "chat",
      who: m.name || m.sender_id || "群友",
      text: m.text || ""
    });
  });
  (yyQQC.actions || []).forEach(function (a) {
    var t = Date.parse(a.time) || 0;
    rows.push({
      t: t, kind: a.ok ? "act" : "fail", ok: a.ok ? 1 : 0,
      icon: a.ok ? yyQQCapIcon(a.kind) : "ban",
      who: yyQQCapLabel(a.kind) + (a.target ? " · " + a.target : ""),
      text: a.detail || a.error || ""
    });
  });
  rows.sort(function (x, y) { return y.t - x.t; });
  return rows;
}

function yyQQStream() {
  var tab = yyQQC.streamTab || "all";
  var all = yyQQTimeline();
  var rows = all.filter(function (r) {
    if (tab === "msg") return r.kind === "in" || r.kind === "out";
    if (tab === "act") return r.kind === "act" || r.kind === "fail";
    return true;
  }).slice(0, 40);

  var head = '<div class="yyq-stream-head">' +
    '<div class="yyq-seg" role="tablist">' +
      ["all", "msg", "act"].map(function (k) {
        var label = { all: "全部", msg: "消息", act: "动作" }[k];
        return '<button type="button" role="tab" aria-selected="' + (tab === k ? "true" : "false") + '" class="' +
          (tab === k ? "is-on" : "") + '" onclick="yyQQSetStream(\'' + k + '\')">' + label + "</button>";
      }).join("") +
    "</div>" +
    '<span class="yyq-live" data-on="' + (yyQQView().key === "open" ? "1" : "0") + '"><i></i>实时</span>' +
    "</div>";

  var body;
  if (yyQQC.loading.stream) {
    body = '<div class="yyq-tl is-loading">' + yyQQSkeleton(4, 52) + "</div>";
  } else if (yyQQC.msgErr) {
    body = '<p class="yyq-inline-err">' + yyQQIcon("ban") + "<span>" + yyQQE(yyQQC.msgErr) + "</span>" +
      '<button type="button" onclick="yyQQRefreshStream()">重试</button></p>';
  } else if (!rows.length) {
    body = yyQQEmpty(tab === "act" ? "activity" : "chat",
      tab === "act" ? "今天还没有执行过动作" : "机器人还没收到群消息",
      tab === "act" ? "群里发 .点赞 123456 试试" : "在群里 @ 它一句，这里会立刻出现",
      "刷新", "yyQQRefreshStream()");
  } else {
    body = '<ol class="yyq-tl">' + rows.map(function (r) {
      return '<li class="yyq-tl-item" data-kind="' + r.kind + '" data-ok="' + r.ok + '">' +
        '<span class="yyq-tl-ico">' + yyQQIcon(r.icon) + "</span>" +
        '<div class="yyq-tl-body"><div class="yyq-tl-top"><b>' + yyQQE(r.who) + "</b><time>" + yyQQE(yyQQClock(r.t)) + "</time></div>" +
        "<p>" + yyQQE(r.text || "（无内容）") + "</p></div></li>";
    }).join("") + "</ol>";
  }

  return '<aside class="yyq-stream" id="yyq-stream" aria-label="实时流">' + head + body +
    '<div class="yyq-stream-foot"><button type="button" class="yyq-ghost" onclick="yyQQRefreshStream()">' + yyQQIcon("refresh") + "刷新</button></div></aside>";
}

function yyQQSetStream(k) {
  yyQQC.streamTab = k;
  yyQQRender("stream");
}

// ---------- 4 接入与群 ----------

function yyQQAccess() {
  var cfg = yyQQC.cfg || {}, ob = cfg.onebot || {}, q = cfg.qqofficial || {}, as = cfg.assistant || {};
  var official = yyQQC.adapter === "qqofficial";
  var open = yyQQC.accessOpen;
  var sum = (official ? "QQ 官方" : "NapCat") + (yyQQC.picked ? " · " + yyQQC.picked : " · 未绑定群");

  var groupsHtml;
  if (yyQQC.groupsLoading) {
    groupsHtml = '<div class="yyq-group-list">' + yyQQSkeleton(2, 56) + "</div>";
  } else if (yyQQC.groupsErr) {
    groupsHtml = '<p class="yyq-inline-err">' + yyQQIcon("ban") + "<span>" + yyQQE(yyQQC.groupsErr) + "</span>" +
      '<button type="button" onclick="yyQQLoadGroups()">重试</button></p>';
  } else if (!yyQQC.groupsLoaded) {
    groupsHtml = '<p class="yyq-group-hint">' + yyQQIcon("list") + "读取机器人已加入的群，点一下即可绑定</p>";
  } else if (!yyQQC.groups.length) {
    groupsHtml = yyQQEmpty("list", "机器人还没加入任何群", "先在手机 QQ 里把它拉进一个群", "重新读取", "yyQQLoadGroups()");
  } else {
    groupsHtml = '<div class="yyq-group-list">' + yyQQC.groups.map(function (g) {
      var id = g.group_id || g.id;
      var on = id === yyQQC.picked;
      return '<button type="button" class="yyq-group-card" data-id="' + yyQQE(id) + '" data-on="' + (on ? "1" : "0") + '" onclick="yyQQPickGroup(\'' + yyQQE(id) + '\')">' +
        '<span class="yyq-group-name">' + yyQQE(g.group_name || g.name || id) + "</span>" +
        "<small>" + yyQQE(id) + (g.member_count ? " · " + g.member_count + " 人" : "") + "</small>" +
        '<span class="yyq-group-check">' + yyQQIcon("check") + "</span></button>";
    }).join("") + "</div>";
  }

  return '<details class="yyq-fold" id="yyq-access"' + (open ? " open" : "") + ' ontoggle="yyQQFoldToggle(\'access\',this)">' +
    "<summary>" +
      '<span class="yyq-fold-ico">' + yyQQIcon("connection") + "</span>" +
      '<span class="yyq-fold-t">接入与互通群</span>' +
      '<span class="yyq-fold-sum">' + yyQQE(sum) + "</span>" +
      '<span class="yyq-fold-caret">' + yyQQIcon("arrow") + "</span>" +
    "</summary>" +
    '<div class="yyq-fold-body">' +
      '<div class="yyq-seg yyq-seg-wide" role="tablist">' +
        '<button type="button" role="tab" aria-selected="' + (!official ? "true" : "false") + '" class="' + (!official ? "is-on" : "") + '" onclick="yyQQSetAdapter(\'onebot\')">OneBot / NapCat</button>' +
        '<button type="button" role="tab" aria-selected="' + (official ? "true" : "false") + '" class="' + (official ? "is-on" : "") + '" onclick="yyQQSetAdapter(\'qqofficial\')">QQ 官方机器人</button>' +
      "</div>" +
      '<div class="yyq-fields" data-mode="' + (official ? "official" : "onebot") + '">' +
        '<label class="yyq-field"><span>WebSocket 地址</span><input id="yyq-ws" placeholder="ws://127.0.0.1:3001" value="' + yyQQE(ob.ws_url) + '"></label>' +
        '<label class="yyq-field"><span>OneBot Token</span><input id="yyq-token" type="password" autocomplete="new-password" placeholder="' + (ob.has_token ? "已保存，留空不改" : "未设置") + '"></label>' +
        '<label class="yyq-field yyq-field-official"><span>AppID</span><input id="yyq-appid" value="' + yyQQE(q.app_id) + '"></label>' +
        '<label class="yyq-field yyq-field-official"><span>AppSecret</span><input id="yyq-secret" type="password" autocomplete="new-password" placeholder="' + (q.has_secret ? "已保存，留空不改" : "未设置") + '"></label>' +
      "</div>" +
      '<div class="yyq-group-block">' +
        '<div class="yyq-block-head"><h4>互通群</h4><button type="button" class="yyq-textbtn" onclick="yyQQLoadGroups()">' + yyQQIcon("refresh") + "读取群列表</button></div>" +
        groupsHtml +
        '<label class="yyq-field yyq-field-manual"><span>或手动填写群号 / 官方群 openid</span><input id="yyq-group" value="' + yyQQE(yyQQC.picked) + '" placeholder="1121432132"></label>' +
      "</div>" +
      '<label class="yyq-switchrow"><span class="yyq-switchrow-text"><strong>把群聊消息转发进游戏</strong><small>普通群聊以「[群] 昵称：内容」发到游戏；指令和 . 开头的消息仍走机器人逻辑</small></span>' +
        '<span class="yyq-sw"><input type="checkbox"' + (as.relay_chat ? " checked" : "") + ' id="yyq-relay" aria-label="转发群聊进游戏"><span></span></span></label>' +
      '<div class="yyq-fold-actions"><button type="button" class="btn" data-busy="' + (yyQQC.busy.access ? "1" : "0") + '" onclick="yyQQSaveAccess(this)">保存并重连</button>' +
        '<button type="button" class="btn-s" onclick="yyQQRefreshAll(1)">刷新状态</button></div>' +
    "</div></details>";
}

function yyQQFoldToggle(key, el) {
  yyQQC[key + "Open"] = !!el.open;
}
// 切换适配器不重绘折叠块，否则用户刚填的地址会被清空。
function yyQQSetAdapter(v) {
  yyQQC.adapter = v;
  var wrap = document.querySelector("#yyq-access .yyq-fields");
  if (wrap) wrap.setAttribute("data-mode", v === "qqofficial" ? "official" : "onebot");
  var segs = document.querySelectorAll("#yyq-access .yyq-seg-wide button");
  for (var i = 0; i < segs.length; i++) {
    var on = (i === 1) === (v === "qqofficial");
    segs[i].className = on ? "is-on" : "";
    segs[i].setAttribute("aria-selected", on ? "true" : "false");
  }
  var sum = document.querySelector("#yyq-access .yyq-fold-sum");
  if (sum) sum.textContent = (v === "qqofficial" ? "QQ 官方" : "NapCat") + (yyQQC.picked ? " · " + yyQQC.picked : " · 未绑定群");
}
// 选群同理：只改勾选态与输入框，不整块重绘。
function yyQQPickGroup(id) {
  yyQQC.picked = id;
  var input = document.getElementById("yyq-group");
  if (input) input.value = id;
  var cards = document.querySelectorAll("#yyq-access .yyq-group-card");
  for (var i = 0; i < cards.length; i++) {
    cards[i].setAttribute("data-on", cards[i].getAttribute("data-id") === id ? "1" : "0");
  }
  yyQQRefreshStream();
  yyQQRender("hero");
}

async function yyQQLoadGroups() {
  yyQQC.groupsLoading = true;
  yyQQC.groupsErr = "";
  yyQQRender("access");
  var r = await A("GET", "/api/qq/groups/available");
  yyQQC.groupsLoading = false;
  yyQQC.groupsLoaded = true;
  if (!r || !r.ok) {
    yyQQC.groups = [];
    yyQQC.groupsErr = (r && r.error) || "读取群列表失败";
  } else {
    yyQQC.groups = r.groups || [];
    yyQQC.groupsErr = "";
  }
  yyQQRender("access");
  var el = document.getElementById("yyq-access");
  if (el) el.setAttribute("open", "");
  yyQQC.accessOpen = true;
}

async function yyQQSaveAccess(btn) {
  var v = function (id) { var el = document.getElementById(id); return el ? el.value : ""; };
  var picked = v("yyq-group").trim();
  var groups = yyQQGroupsOf(yyQQC.cfg).map(function (g) {
    var id = g.id || g.group_id;
    return { id: id, name: g.name || "", enabled: id === picked };
  });
  if (picked && !groups.some(function (g) { return g.id === picked; })) {
    groups.push({ id: picked, name: "", enabled: true });
  }
  var body = {
    enabled: true,
    adapter: yyQQC.adapter,
    groups: groups,
    onebot: { ws_url: v("yyq-ws").trim() },
    qqofficial: { app_id: v("yyq-appid").trim() },
    assistant: { relay_chat: !!(document.getElementById("yyq-relay") || {}).checked }
  };
  if (v("yyq-token")) body.onebot.token = v("yyq-token");
  if (v("yyq-secret")) body.qqofficial.app_secret = v("yyq-secret");
  yyQQC.busy.access = 1;
  if (btn) { btn.setAttribute("data-busy", "1"); btn.textContent = "保存中"; }
  var r = await A("POST", "/api/qq/config", body);
  delete yyQQC.busy.access;
  if (!r || !r.ok) {
    if (btn) { btn.setAttribute("data-busy", "0"); btn.textContent = "保存并重连"; }
    T((r && r.error) || "保存失败", "e");
    return;
  }
  yyQQC.picked = picked;
  T("已保存，正在重新接入", "o");
  await yyQQRefreshAll(1);
  if (btn) { btn.setAttribute("data-busy", "0"); btn.textContent = "保存并重连"; }
}

// ---------- 5 人设与试跑 ----------

function yyQQPersona() {
  var as = (yyQQC.cfg || {}).assistant || {};
  var prompt = as.prompt || "";
  var open = yyQQC.personaOpen;
  return '<details class="yyq-fold" id="yyq-persona"' + (open ? " open" : "") + ' ontoggle="yyQQFoldToggle(\'persona\',this)">' +
    "<summary>" +
      '<span class="yyq-fold-ico">' + yyQQIcon("spark") + "</span>" +
      '<span class="yyq-fold-t">人设与试跑</span>' +
      '<span class="yyq-fold-sum">' + yyQQE(prompt ? yyQQClip(prompt, 22) : "未设置提示词") + "</span>" +
      '<span class="yyq-fold-caret">' + yyQQIcon("arrow") + "</span>" +
    "</summary>" +
    '<div class="yyq-fold-body">' +
      '<div class="yyq-prompt-bar">' +
        '<div class="yyq-var-chips">' + ["{玩家}", "{群名}", "{时间}"].map(function (v) {
          return '<button type="button" onclick="yyQQInsertVar(\'' + v + '\')">' + v + "</button>";
        }).join("") + "</div>" +
        '<span class="yyq-count" id="yyq-prompt-count">' + prompt.length + " / 2000</span>" +
      "</div>" +
      '<textarea id="yyq-prompt" class="yyq-textarea" rows="6" oninput="yyQQPromptCount(this)" placeholder="例如：你是布汲岛的 QQ 助手，说话简短、先给结论…">' + yyQQE(prompt) + "</textarea>" +
      '<div class="yyq-fields">' +
        '<label class="yyq-field"><span>使用模型</span><input id="yyq-model" value="' + yyQQE(as.model) + '" placeholder="留空用默认模型"></label>' +
      "</div>" +
      '<div class="yyq-sandbox">' +
        '<div class="yyq-block-head"><h4>沙盒试跑</h4><span class="yyq-tagline">本地预览 · 不会发到群里</span></div>' +
        '<div class="yyq-sandbox-log" id="yyq-sandbox-log">' + yyQQSandboxLog() + "</div>" +
        '<div class="yyq-sandbox-input"><input id="yyq-try" placeholder="写一句话，看看人设会怎么套用" onkeydown="if(event.key===\'Enter\')yyQQAsk()">' +
          '<button type="button" class="btn" onclick="yyQQAsk()">试跑</button></div>' +
      "</div>" +
      '<div class="yyq-fold-actions"><button type="button" class="btn" onclick="yyQQSavePersona(this)">保存人设</button>' +
        '<button type="button" class="btn-s" onclick="yyQQClearSandbox()">清空沙盒</button></div>' +
    "</div></details>";
}

function yyQQClip(s, n) {
  s = String(s || "").replace(/\s+/g, " ").trim();
  return s.length > n ? s.slice(0, n) + "…" : s;
}
function yyQQPromptCount(el) {
  var n = document.getElementById("yyq-prompt-count");
  if (n) n.textContent = (el.value || "").length + " / 2000";
}
function yyQQInsertVar(v) {
  var el = document.getElementById("yyq-prompt");
  if (!el) return;
  var s = el.selectionStart == null ? el.value.length : el.selectionStart;
  var e = el.selectionEnd == null ? el.value.length : el.selectionEnd;
  el.value = el.value.slice(0, s) + v + el.value.slice(e);
  el.focus();
  try { el.setSelectionRange(s + v.length, s + v.length); } catch (_) {}
  yyQQPromptCount(el);
}
function yyQQSandboxLog() {
  if (!yyQQC.sandbox.length) {
    return '<p class="yyq-sandbox-empty">还没有试跑。写一句群友可能会说的话，这里会显示人设套用后的效果。</p>';
  }
  return yyQQC.sandbox.map(function (m) {
    if (m.role === "user") {
      return '<div class="yyq-bubble" data-role="user"><span>你</span><p>' + yyQQE(m.text) + "</p></div>";
    }
    return '<div class="yyq-bubble" data-role="note"><span>人设预览 · 本地，未发送</span><pre>' + yyQQE(m.text) + "</pre></div>";
  }).join("");
}
function yyQQClearSandbox() {
  yyQQC.sandbox = [];
  var log = document.getElementById("yyq-sandbox-log");
  if (log) log.innerHTML = yyQQSandboxLog();
}
function yyQQAsk() {
  var input = document.getElementById("yyq-try");
  var text = ((input && input.value) || "").trim();
  if (!text) { T("先写一句要试的话", "w"); return; }
  var prompt = ((document.getElementById("yyq-prompt") || {}).value || "").trim();
  var group = yyQQGroupLabel();
  var now = new Date();
  var resolved = (prompt || "（未设置提示词）")
    .replace(/\{玩家\}/g, "沙盒测试员")
    .replace(/\{群名\}/g, group || "未绑定群")
    .replace(/\{时间\}/g, now.getFullYear() + "-" + (now.getMonth() + 1) + "-" + now.getDate() + " " + yyQQClock(now.getTime()));
  yyQQC.sandbox.push({ role: "user", text: text });
  yyQQC.sandbox.push({ role: "note", text: "群友说：" + text + "\n\n套用后的人设：\n" + resolved });
  if (yyQQC.sandbox.length > 8) yyQQC.sandbox = yyQQC.sandbox.slice(-8);
  if (input) input.value = "";
  var log = document.getElementById("yyq-sandbox-log");
  if (log) {
    log.innerHTML = yyQQSandboxLog();
    log.scrollTop = log.scrollHeight;
  }
}
async function yyQQSavePersona(btn) {
  var prompt = (document.getElementById("yyq-prompt") || {}).value || "";
  var model = (document.getElementById("yyq-model") || {}).value || "";
  if (btn) { btn.setAttribute("data-busy", "1"); btn.textContent = "保存中"; }
  var r = await A("POST", "/api/qq/config", { assistant: { prompt: prompt, model: model } });
  if (btn) { btn.setAttribute("data-busy", "0"); btn.textContent = "保存人设"; }
  if (!r || !r.ok) { T((r && r.error) || "保存失败", "e"); return; }
  yyQQC.cfg.assistant = yyQQC.cfg.assistant || {};
  yyQQC.cfg.assistant.prompt = prompt;
  yyQQC.cfg.assistant.model = model;
  if (btn) { btn.textContent = "✓ 已保存"; setTimeout(function () { if (btn) btn.textContent = "保存人设"; }, 1400); }
  T("人设已保存", "o");
}

// ---------- 6 收藏表情 ----------

function yyQQFaces() {
  var open = yyQQC.facesOpen;
  var count = (yyQQC.faces || []).length;
  var body;
  if (yyQQC.facesLoading) {
    body = '<div class="yyq-face-grid">' + yyQQSkeleton(6, 96) + "</div>";
  } else if (yyQQC.facesErr) {
    body = '<p class="yyq-inline-err">' + yyQQIcon("ban") + "<span>" + yyQQE(yyQQC.facesErr) + "</span>" +
      '<button type="button" onclick="yyQQLoadFaces()">重试</button></p>';
  } else if (!count) {
    body = yyQQEmpty("image", "还没读取表情包", "读取后能看清机器人收藏了哪些图", "读取表情包", "yyQQLoadFaces()");
  } else {
    body = '<div class="yyq-face-grid">' + yyQQC.faces.map(function (f) {
      return '<figure class="yyq-face"><img src="' + yyQQE(f.url) + '" alt="" loading="lazy" decoding="async" referrerpolicy="no-referrer"' +
        ' onerror="this.parentNode.classList.add(\'is-broken\')"><figcaption>' + yyQQE(f.name || f.id || "表情") + "</figcaption></figure>";
    }).join("") + "</div>";
  }
  return '<details class="yyq-fold" id="yyq-faces"' + (open ? " open" : "") + ' ontoggle="yyQQFoldToggle(\'faces\',this)">' +
    "<summary>" +
      '<span class="yyq-fold-ico">' + yyQQIcon("image") + "</span>" +
      '<span class="yyq-fold-t">收藏表情</span>' +
      '<span class="yyq-fold-sum">' + (count ? count + " 个" : "未读取") + "</span>" +
      '<span class="yyq-fold-caret">' + yyQQIcon("arrow") + "</span>" +
    "</summary>" +
    '<div class="yyq-fold-body">' + body + "</div></details>";
}

async function yyQQLoadFaces() {
  yyQQC.facesLoading = true;
  yyQQC.facesErr = "";
  yyQQC.facesOpen = true;
  yyQQRender("faces");
  var el = document.getElementById("yyq-faces");
  if (el) el.setAttribute("open", "");
  var r = await A("GET", "/api/qq/faces");
  yyQQC.facesLoading = false;
  if (!r || !r.ok) {
    yyQQC.faces = [];
    yyQQC.facesErr = (r && r.error) || "读取失败";
  } else {
    yyQQC.faces = r.faces || [];
    yyQQC.facesErr = "";
  }
  yyQQRender("faces");
  var box = document.getElementById("yyq-faces");
  if (box) box.setAttribute("open", "");
}

// ---------- 通用小块 ----------

function yyQQSkeleton(n, h) {
  var out = "";
  for (var i = 0; i < n; i++) out += '<span class="yyq-skel" style="height:' + h + 'px"></span>';
  return out;
}
function yyQQEmpty(icon, title, sub, actionLabel, action) {
  return '<div class="yyq-empty"><span class="yyq-empty-ico">' + yyQQIcon(icon) + "</span>" +
    "<p>" + yyQQE(title) + "</p>" +
    (sub ? "<small>" + yyQQE(sub) + "</small>" : "") +
    (actionLabel ? '<button type="button" class="btn-s" onclick="' + action + '">' + yyQQE(actionLabel) + "</button>" : "") +
    "</div>";
}

// ---------- 渲染 ----------

var yyQQParts = {
  status: yyQQStatusBar,
  hero: yyQQHero,
  caps: yyQQCaps,
  stream: yyQQStream,
  access: yyQQAccess,
  persona: yyQQPersona,
  faces: yyQQFaces
};

function yyQQRender(part) {
  var fn = yyQQParts[part];
  if (!fn) return;
  var el = document.getElementById("yyq-" + part);
  if (!el) return;
  var wasOpen = el.open;
  el.outerHTML = fn();
  if (part === "access" || part === "persona" || part === "faces") {
    var again = document.getElementById("yyq-" + part);
    if (again && wasOpen) again.setAttribute("open", "");
  }
}

function yyQQRenderAll() {
  var root = document.getElementById("yyq-root");
  if (!root) return;
  root.innerHTML =
    yyQQStatusBar() +
    yyQQHero() +
    '<div class="yyq-grid">' + yyQQCaps() + yyQQStream() + "</div>" +
    '<div class="yyq-fold-grid">' + yyQQAccess() + yyQQPersona() + "</div>" +
    yyQQFaces();
  yyQQSyncFolds();
}
function yyQQSyncFolds() {
  ["access", "persona", "faces"].forEach(function (k) {
    var el = document.getElementById("yyq-" + k);
    if (el && yyQQC[k + "Open"]) el.setAttribute("open", "");
  });
}

// ---------- 数据 ----------

async function yyQQLoadCore() {
  var r = await Promise.all([
    A("GET", "/api/qq/config"),
    A("GET", "/api/qq/status"),
    A("GET", "/api/qq/login"),
    A("GET", "/api/qq/caps"),
    A("GET", "/api/qq/actions")
  ]);
  var cfgR = r[0], stR = r[1], lgR = r[2], capR = r[3], actR = r[4];
  yyQQC.cfg = (cfgR && cfgR.config) || yyQQC.cfg || {};
  yyQQC.status = (stR && stR.ok !== false) ? stR : {};
  yyQQC.login = (lgR && lgR.ok) ? lgR : null;
  yyQQC.caps = (capR && capR.caps) || (actR && actR.caps) || [];
  yyQQC.actions = (actR && actR.actions) || [];
  yyQQC.adapter = yyQQC.cfg.adapter === "qqofficial" ? "qqofficial" : "onebot";
  if (!yyQQC.picked) {
    var on = yyQQEnabledGroups();
    yyQQC.picked = on.length ? (on[0].id || on[0].group_id) : (yyQQC.cfg.group_id || "");
  }
  yyQQC.loading.caps = 0;
  yyQQC.coreErr = (cfgR && cfgR.ok === false) ? (cfgR.error || "读取配置失败") : "";
  if (yyQQC.login && yyQQC.login.qrcode) {
    yyQQC.qrBase = yyQQC.login.qrcode.age_seconds || 0;
    yyQQC.qrFetchedAt = Date.now();
    yyQQC.lastQrUrl = yyQQC.login.qrcode.data_url || "";
  }
  // 没连上时直接把接入区展开，省掉"还要再点一下"的一步。
  if (yyQQC.cfg.enabled === false) yyQQC.accessOpen = true;
  if (yyQQC.login && yyQQC.login.state === "need_login") yyQQC.accessOpen = true;
}

async function yyQQLoadProfile() {
  var r = await A("GET", "/api/qq/bot/profile");
  if (!r || !r.ok) return;
  yyQQC.profile = r;
  yyQQRender("hero");
}

async function yyQQLoadMessages() {
  if (!yyQQC.picked) {
    yyQQC.messages = [];
    yyQQC.loading.stream = 0;
    yyQQC.msgErr = "";
    return;
  }
  var r = await A("GET", "/api/qq/messages?group=" + encodeURIComponent(yyQQC.picked) + "&limit=40");
  yyQQC.loading.stream = 0;
  if (!r || !r.ok) {
    yyQQC.messages = [];
    yyQQC.msgErr = (r && r.error) || "读取消息失败";
    return;
  }
  yyQQC.messages = r.messages || [];
  yyQQC.msgErr = "";
}

async function yyQQRefreshStream() {
  await yyQQLoadMessages();
  yyQQRender("stream");
  yyQQRender("status");
}

async function yyQQRefreshAll(deep) {
  await yyQQLoadCore();
  await yyQQLoadMessages();
  yyQQRenderAll();
  yyQQLoadProfile();
  if (deep) yyQQPollLogin(1);
}

// ---------- 轮询 ----------

function yyQQStopTimers() {
  (yyQQC.timers || []).forEach(function (t) { clearInterval(t); });
  yyQQC.timers = [];
}

function yyQQStartTimers() {
  yyQQStopTimers();
  // 二维码年龄每秒走一格，只在未登录时跑。
  yyQQC.timers.push(setInterval(yyQQTickQr, 1000));
  // 登录态每 20 秒轮询一次，只刷二维码区域，不整页重载。
  yyQQC.timers.push(setInterval(function () {
    var v = yyQQView();
    if (v.key === "need_login" || (yyQQC.login && yyQQC.login.state !== "logged_in")) yyQQPollLogin(0);
  }, 20000));
  // 实时流每 15 秒刷新。
  yyQQC.timers.push(setInterval(function () {
    if (!document.getElementById("yyq-stream")) return;
    yyQQRefreshStream();
  }, 15000));
  if (typeof activeRenderers !== "undefined" && activeRenderers.push) {
    activeRenderers.push(yyQQStopTimers);
  }
}

function yyQQQrAge() {
  if (!yyQQC.login || !yyQQC.login.qrcode || !yyQQC.login.qrcode.available) return null;
  var drift = Math.floor((Date.now() - yyQQC.qrFetchedAt) / 1000);
  return Math.max(0, (yyQQC.qrBase || 0) + drift);
}

function yyQQTickQr() {
  var age = yyQQQrAge();
  if (age == null) return;
  var box = document.getElementById("yyq-qr");
  if (!box) return;
  var label = document.getElementById("yyq-qr-age");
  if (label) label.textContent = age;
  var fresh = age <= 110;
  if ((box.getAttribute("data-fresh") === "1") !== fresh) {
    box.setAttribute("data-fresh", fresh ? "1" : "0");
    var mask = box.querySelector(".yyq-qr-mask");
    if (!fresh && !mask) {
      var img = box.querySelector(".yyq-qr-box");
      if (img) img.insertAdjacentHTML("beforeend", '<span class="yyq-qr-mask">正在刷新…</span>');
    } else if (fresh && mask) {
      mask.remove();
    }
  }
}

async function yyQQPollLogin(force) {
  var r = await A("GET", "/api/qq/login");
  if (!r || !r.ok) return;
  var wasNeed = yyQQC.login && yyQQC.login.state === "need_login";
  yyQQC.login = r;
  yyQQC.qrBase = (r.qrcode && r.qrcode.age_seconds) || 0;
  yyQQC.qrFetchedAt = Date.now();

  var nowNeed = r.state === "need_login";
  var newUrl = (r.qrcode && r.qrcode.data_url) || "";
  var img = document.querySelector("#yyq-qr img");

  // 二维码换了新图：只换 img.src，不重绘整块，避免闪烁。
  if (nowNeed && img && newUrl && newUrl !== yyQQC.lastQrUrl) {
    img.src = newUrl;
    yyQQC.lastQrUrl = newUrl;
  }
  if (force || wasNeed !== nowNeed || (nowNeed && !document.getElementById("yyq-qr"))) {
    yyQQC.lastQrUrl = newUrl;
    yyQQRender("hero");
    yyQQRender("status");
    if (wasNeed && !nowNeed) {
      T("机器人已登录", "o");
      yyQQRefreshAll(0);
    }
  }
}

// ---------- 页面 ----------

function yyQQConsoleBody() {
  return '<div class="yyq-root" id="yyq-root">' +
    yyQQStatusBar() +
    yyQQHero() +
    '<div class="yyq-grid">' + yyQQCaps() + yyQQStream() + "</div>" +
    '<div class="yyq-fold-grid">' + yyQQAccess() + yyQQPersona() + "</div>" +
    yyQQFaces() +
    "</div>";
}

async function yyQQConsolePage() {
  yyQQStopTimers();
  delete subPageCache["群服互通"];
  yyQQC.loading.caps = 1;
  yyQQC.loading.stream = 1;
  yyQQC.sandbox = [];
  yyQQC.menu = false;
  yyQQC.groupsLoaded = false;
  yyQQC.groups = [];
  yyQQC.facesLoaded = false;
  yyQQC.faces = [];
  yyQQC.facesOpen = false;
  yyQQC.personaOpen = false;

  await yyQQLoadCore();
  await yyQQLoadMessages();
  if (yyQQC.login && yyQQC.login.state === "need_login") yyQQC.accessOpen = true;

  openSubPage("群服互通", yyQQConsoleBody());
  yyQQSyncFolds();
  yyQQStartTimers();
  yyQQLoadProfile();
  if (yyQQC.login && yyQQC.login.state === "need_login") yyQQPollLogin(1);
}

// 覆盖旧版表单页
var yyQQConsoleBase = typeof yyQQPage === "function" ? yyQQPage : null;
yyQQPage = yyQQConsolePage;
