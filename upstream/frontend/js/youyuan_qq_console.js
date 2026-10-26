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
  mic: '<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3M8.5 21h7"/>',
  pen: '<path d="m15 4 5 5M4 20l5-1L21 7l-4-4L5 15l-1 5Z"/>',
  list: '<path d="M9 5h12M9 12h12M9 19h12M3 5h1M3 12h1M3 19h1"/>',
  connection: '<rect x="7" y="3" width="10" height="7" rx="1"/><rect x="2" y="17" width="8" height="5" rx="1"/><rect x="14" y="17" width="8" height="5" rx="1"/><path d="M12 10v4M6 17v-3h12v3"/>',
  arrow: '<path d="M5 12h14m-5-5 5 5-5 5"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  spark: '<path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5L12 3Z"/>',
  user: '<circle cx="12" cy="7" r="4"/><path d="M4 21v-2a6 6 0 0 1 6-6h4a6 6 0 0 1 6 6v2"/>'
};

// QQ 号属于个人信息，界面上只显示尾号。
// 账号标识（control_token）才是真正等于令牌的东西，那个一律掩码 + 主动查看。
function yyQQMaskID(id) {
  var text = String(id == null ? "" : id).trim();
  if (!text) return "";
  if (/^\d+$/.test(text) && text.length > 4) return "QQ 尾号 " + text.slice(-4);
  return text.length > 8 ? text.slice(0, 4) + "…" + text.slice(-2) : text;
}
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
  voice: { group: "interact", icon: "mic" },
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

// 能力 kind → 指令 ID（assistant.command_aliases 的 key，见 youyuan_qq_commands.go）。
// 没有对应指令 ID 的能力（relay_history/game_skill）不显示指令编辑，保持只读展示。
var yyQQCapCmdId = {
  like: "like", emoji_like: "emoji", face: "faces", qzone: "qzone",
  delete_qzone: "delete_qzone", avatar: "avatar", long_nick: "long_nick",
  nick: "nick", kick: "kick", recall: "recall", ban: "ban", poke: "poke",
  sticker: "sticker", voice: "voice"
};

// 生效指令：玩家别名（command_aliases）优先；空串/未设 = 注册表默认指令。
// /api/qq/caps 的 command 字段永远是注册表默认值，别名只存在于 /api/qq/config，
// 所以这里本地合并，保存后立刻能看到新指令，不用等接口。
function yyQQEffectiveCmd(kind, registry) {
  var id = yyQQCapCmdId[kind];
  if (!id) return registry || "";
  var alias = ((yyQQC.cfg || {}).assistant || {}).command_aliases || {};
  var v = alias[id];
  if (typeof v === "string" && v.trim()) return v.trim();
  return registry || "";
}

var yyQQC = {
  cfg: {}, status: {}, login: null, caps: [], profile: null, faces: [],
  actions: [], messages: [],
  capGroup: "all", openCap: "",
  adapter: "onebot", picked: "",
  // 多群：picked 是当前标签，sel 是接入区正在勾选的群（保存前的草稿）。
  // readAt[群] 记「这个群我看到哪条了」，unread[群] 是标签上的角标数字，
  // msgCache[群] 存最近一次读到的正文——切回来先立刻上屏，再去后台刷新。
  sel: [], selTouched: false, readAt: {}, unread: {}, msgCache: {}, sent: {},
  stickBottom: 0, syncing: {},
  groups: [], groupsLoaded: false, groupsLoading: false, groupsErr: "",
  facesLoaded: false, facesLoading: false, facesErr: "",
  // 技能系统（GET /api/qq/assistant/skills）：默认折叠，展开才读、才出逐行编辑。
  skills: [], skillsDelay: null, skillsErr: "",
  skillsLoaded: false, skillsLoading: false, skillsOpen: false, openSkill: "",
  // AI 工具（GET/POST /api/qq/ai/tools）：分组 + 逐项开关，保存即时生效。
  toolGroups: [], toolTotal: 0, toolErr: "",
  toolsLoaded: false, toolsLoading: false, toolsOpen: false,
  loading: { caps: 1, stream: 1 },
  msgErr: "", lastMsgErr: "", coreErr: "",
  // 首页 6 个模块的折叠状态集中在这里（键 = 模块 id：chat/records/ai/robot/admin/root）。
  // 以前是 accessOpen/personaOpen/toolsOpen/facesOpen 四个互不相干的变量，
  // 每个折叠块自己记一份，整壳重建后必然对不上。
  modOpen: {}, modLoaded: {},
  accessOpen: false, personaOpen: false, facesOpen: false,
  sandbox: [], busy: {}, menu: false,
  qrBase: 0, qrFetchedAt: 0, lastQrUrl: "", timers: []
};

// ---------- 首页模块注册表 ----------
//
// 互通首页固定 6 个模块，顺序与群内「帮助」菜单一致：
//   ① 聊天与账号 ② 查询与记录 ③ AI 助手 ④ 机器人能力 ⑤ 仅管理员 ⑥ 仅超级管理员
// 其它脚本（触发词/生图/知识库/群规/危险指令/互通端信息/助手）不再包装
// window.yyQQRenderAll 往一个 grid 里塞 <details>，而是调 yyQQRegisterModule()
// 注册到对应模块；壳重建时按 order 依次 render()。
//
// def: { id, order, minRole, summary, render, load }
//   id      模块 id：chat/records/ai/robot/admin/root
//   order   同一模块内的排列顺序（小的在前）
//   minRole 0 所有人 / 1 管理员 / 2 超管；minRole>0 的模块在权限不足时整块不渲染
//   render  返回该模块里的一段 <section class="yyq-sec">…</section>
//   load    首次展开（或默认展开）时拉数据，可重复调用，自己防重
window.yyQQInteropModules = window.yyQQInteropModules || [];
function yyQQRegisterModule(def) {
  if (!def || !def.id) return null;
  var list = window.yyQQInteropModules;
  for (var i = 0; i < list.length; i++) {
    if (def.__key && list[i] && list[i].id === def.id && list[i].__key === def.__key) {
      list[i] = def;
      return def;
    }
  }
  list.push(def);
  list.sort(function (a, b) { return (Number(a.order) || 0) - (Number(b.order) || 0); });
  return def;
}

// 6 个模块的壳定义。summary 是收起时右侧那行结论。
var yyQQModMeta = [
  { id: "chat", title: "💬 聊天与账号", icon: "chat", order: 1, minRole: 0,
    open: function () { return yyQQAccessNeeded(); }, summary: yyQQModChatSum },
  { id: "records", title: "📋 查询与记录", icon: "list", order: 2, minRole: 0,
    open: function () { return false; }, summary: yyQQModRecordsSum },
  { id: "ai", title: "✨ AI 助手", icon: "spark", order: 3, minRole: 0,
    open: function () { return true; }, summary: yyQQModAISum },
  { id: "robot", title: "🤖 机器人能力", icon: "robot", order: 4, minRole: 0,
    open: function () { return true; }, summary: yyQQModRobotSum },
  { id: "admin", title: "🛠 仅管理员", icon: "key", order: 5, minRole: 1,
    open: function () { return false; }, summary: function () { return "危险指令 · MCP 作用域"; } },
  { id: "root", title: "🔑 仅超级管理员", icon: "key", order: 6, minRole: 2,
    open: function () { return false; }, summary: yyQQModRootSum }
];
function yyQQModMetaById(id) {
  for (var i = 0; i < yyQQModMeta.length; i++) if (yyQQModMeta[i].id === id) return yyQQModMeta[i];
  return null;
}
// 查看者角色：0 普通成员 / 1 管理员 / 2 超管。
// 工具箱页面在本机以内置账号打开，默认按「超管」看待；要把首页分享给
// 管理员/普通成员看时，用 window.yyQQViewerRole = 1|0 或 <html data-yy-role="1"> 指定。
function yyQQViewerRole() {
  var r = window.yyQQViewerRole;
  if (typeof r === "number" && isFinite(r)) return Math.max(0, Math.min(2, Math.round(r)));
  var attr = document.documentElement ? document.documentElement.getAttribute("data-yy-role") : "";
  if (attr !== "" && attr != null && /^[0-2]$/.test(String(attr))) return Number(attr);
  return 2;
}
function yyQQModList() {
  var role = yyQQViewerRole();
  return yyQQModMeta.filter(function (m) { return role >= (m.minRole || 0); });
}
function yyQQAccessNeeded() {
  var st = yyQQC.status || {}, cfg = yyQQC.cfg || {};
  var connected = st.state === "open" || !!(yyQQC.login && yyQQC.login.logged_in);
  return !connected || cfg.enabled === false || !(yyQQC.sel || []).length;
}
function yyQQModChatSum() {
  if (yyQQAccessNeeded()) return "待接入";
  return (yyQQC.adapter === "qqofficial" ? "QQ 官方" : "NapCat") +
    ((yyQQC.sel || []).length ? " · " + yyQQC.sel.length + " 个群" : " · 未选群");
}
function yyQQModRecordsSum() {
  var t = yyQQTodayActions();
  return t.total ? "今日 " + t.total + " 次动作" + (t.fail ? " · 失败 " + t.fail : "") : "暂无动作";
}
function yyQQModAISum() {
  var as = (yyQQC.cfg || {}).assistant || {};
  var p = as.prompt || "";
  return p ? yyQQClip(p, 18) : "未设置提示词";
}
function yyQQModRobotSum() {
  var n = (yyQQC.faces || []).length;
  var t = yyQQToolSum();
  return (n ? "表情 " + n + " 个" : "表情未读取") + " · " + t;
}
function yyQQModRootSum() {
  var as = (yyQQC.cfg || {}).assistant || {};
  return "超管 " + (as.super_admins || []).length + " 人";
}
// 模块里的一段分区（桌面两列，窄屏一列）。
function yyQQSec(inner, extraClass) {
  return '<section class="yyq-sec' + (extraClass ? " " + extraClass : "") + '">' + inner + "</section>";
}
function yyQQSecHead(title, hint) {
  return '<div class="yyq-sec-head"><h4>' + yyQQE(title) + "</h4>" +
    (hint ? '<small>' + yyQQE(hint) + "</small>" : "") + "</div>";
}
// 收集注册到某个模块的区块（按 order）。
function yyQQModSections(id) {
  var out = "";
  (window.yyQQInteropModules || []).forEach(function (m) {
    if (!m || m.id !== id || typeof m.render !== "function") return;
    try {
      var html = m.render();
      if (html) out += html;
    } catch (e) {
      out += '<p class="yyq-inline-err">' + yyQQE("这个区块渲染失败：" + ((e && e.message) || e)) + "</p>";
    }
  });
  return out;
}
function yyQQModMergedSummary(id) {
  var meta = yyQQModMetaById(id) || {};
  var bits = [];
  try { if (typeof meta.summary === "function") bits.push(meta.summary()); } catch (e) {}
  (window.yyQQInteropModules || []).forEach(function (m) {
    if (!m || m.id !== id || typeof m.summary !== "function") return;
    try { var s = m.summary(); if (s) bits.push(s); } catch (e) {}
  });
  bits = bits.filter(function (s) { return !!s; });
  return bits.join(" · ");
}
// 一个模块的完整 HTML（summary + 已渲染的分区）。
function yyQQModuleShell(id) {
  var meta = yyQQModMetaById(id);
  if (!meta) return "";
  var open = yyQQC.modOpen[id];
  if (open == null) open = !!(meta.open && meta.open());
  return '<details class="yyq-mod" id="yyq-mod-' + id + '" data-mod="' + id + '"' + (open ? " open" : "") +
    ' ontoggle="yyQQModToggle(\'' + id + '\',this)">' +
    '<summary class="yyq-mod-head">' +
      '<span class="yyq-fold-ico">' + yyQQIcon(meta.icon) + "</span>" +
      '<span class="yyq-fold-t">' + meta.title + "</span>" +
      '<span class="yyq-fold-sum">' + yyQQE(yyQQModMergedSummary(id)) + "</span>" +
      '<span class="yyq-fold-caret">' + yyQQIcon("arrow") + "</span>" +
    "</summary>" +
    '<div class="yyq-mod-body" id="yyq-mod-' + id + '-body">' +
      (yyQQModSections(id) || '<p class="yyq-mod-empty">暂无内容</p>') +
    "</div></details>";
}
function yyQQModToggle(id, el) {
  if (!el) return;
  yyQQC.modOpen[id] = !!el.open;
  if (el.open) yyQQModLoad(id);
}
// 展开时才拉数据；只拉一次，模块自己也会防重。
function yyQQModLoad(id) {
  if (yyQQC.modLoaded[id]) return;
  yyQQC.modLoaded[id] = 1;
  (window.yyQQInteropModules || []).forEach(function (m) {
    if (!m || m.id !== id || typeof m.load !== "function") return;
    try { m.load(); } catch (e) {}
  });
}
// 壳重建后：默认展开的模块补一次 load。
function yyQQModLoadOpen() {
  yyQQModList().forEach(function (meta) {
    var el = document.getElementById("yyq-mod-" + meta.id);
    if (el && el.open) yyQQModLoad(meta.id);
  });
}
// 从任意位置跳到某个模块：展开外层模块，再滚到内层锚点。
function yyQQOpenModule(id, anchor) {
  var el = document.getElementById("yyq-mod-" + id);
  if (el && el.tagName === "DETAILS" && !el.open) {
    el.open = true;
    yyQQC.modOpen[id] = true;
    yyQQModLoad(id);
  }
  if (anchor) {
    var tar = document.getElementById(anchor);
    if (tar && tar.scrollIntoView) tar.scrollIntoView({ behavior: "smooth", block: "center" });
    return;
  }
  if (el && el.scrollIntoView) el.scrollIntoView({ behavior: "smooth", block: "start" });
}

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
function yyQQGid(g) {
  return (g && (g.id || g.group_id)) || "";
}
// 当前标签那个群的显示名（发消息、沙盒 {群名} 都用它，别再拼「群A、群B」）。
function yyQQPickedName() {
  var rows = yyQQEnabledGroups();
  for (var i = 0; i < rows.length; i++) {
    if (yyQQGid(rows[i]) === yyQQC.picked) return yyQQGroupName(rows[i]);
  }
  return yyQQC.picked || "";
}
// 已读位置存在本机：换个页面回来还知道哪些是新的。读不到（隐私模式/配额满）就当空表，
// 不要把整个面板拖垮。
function yyQQReadLoad() {
  try {
    var raw = localStorage.getItem("yyqq.read");
    var map = raw ? JSON.parse(raw) : null;
    yyQQC.readAt = map && typeof map === "object" ? map : {};
  } catch (e) {
    yyQQC.readAt = {};
  }
}
function yyQQReadSave() {
  try { localStorage.setItem("yyqq.read", JSON.stringify(yyQQC.readAt || {})); } catch (e) {}
}
// 群标签：一排横滑，名字 + 未读数。放在消息面板顶栏里，只占一行。
function yyQQGroupTabs() {
  var rows = yyQQEnabledGroups();
  if (!rows.length) return "";
  var cur = yyQQC.picked;
  var html = rows.map(function (g) {
    var id = yyQQGid(g);
    var n = yyQQC.unread[id] || 0;
    var on = id === cur;
    return '<button type="button" role="tab" class="yyq-gtab" data-id="' + yyQQE(id) + '" data-on="' + (on ? "1" : "0") +
      '" aria-selected="' + (on ? "true" : "false") + '" title="' + yyQQE(yyQQGroupName(g)) +
      '" onclick="yyQQPickTab(\'' + yyQQE(id) + '\')">' +
      '<span class="yyq-gtab-name">' + yyQQE(yyQQGroupName(g)) + "</span>" +
      (n > 0 ? '<em class="yyq-gtab-n">' + (n > 99 ? "99+" : n) + "</em>" : "") +
      "</button>";
  }).join("");
  return '<div class="yyq-gtabs" role="tablist" aria-label="互通群">' + html + "</div>";
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
  // 2026-10-26 第 30 条：掉线超过 5 分钟（或自愈额度用尽）后端会升成 need_scan。
  // 这一档必须比下面的「连接失败」优先，否则面板只会说连接失败，
  // 用户根本不知道该扫码 —— 就是他反馈的「掉线 5 分钟没叫我重新登录」。
  if (lg.state === "need_scan") {
    var secs = lg.offline_seconds > 0 ? "（已掉线 " + Math.round(lg.offline_seconds / 60) + " 分钟）" : "";
    return { key: "need_scan", label: "需要扫码登录", tone: "bad",
             hint: (lg.hint || "请用手机 QQ 扫描二维码重新登录") + secs };
  }
  // 假在线兜底：socket 还开着但探测不到 QQ 会话时，后端会把 live 置 false。
  if (lg.live_ever_checked && !lg.live && lg.connected !== false) {
    return { key: "need_login", label: "QQ 已掉线", tone: "warn",
             hint: "连接还在但 QQ 会话已经没了（" + (lg.live_error || "探测失败") + "），系统正在自动恢复" };
  }
  // 2026-10-26 第 30 条：状态带也会带 need_scan（掉线 >5 分钟 / 自愈额度用尽）。
  // 登录轮询可能比状态轮询慢一拍，这里兜住，别让面板退回「连接失败」。
  if (st.need_scan) {
    return { key: "need_scan", label: "需要扫码登录", tone: "bad",
             hint: "QQ 已掉线超过 5 分钟，自动恢复没成功：请用手机 QQ 扫描二维码重新登录" };
  }
  var s = st.state || "unconfigured";
  // 假在线：socket 还开着（state=open）但后端探活已经拿不到 QQ 会话。
  // 后端 /api/qq/status 专门给了 live / last_ok_seconds 说真话（youyuan_qq.go:239），
  // 而这里以前只认 state —— 半开 TCP 不会发 FIN/RST，state 会永远停在 open，
  // 面板就一直显示「已连接」，这就是「掉线了还显示在线」的根因。
  if (s === "open" && st.live === false) {
    var offSec = st.offline_seconds || 0;
    return { key: "reconnecting", label: "QQ 已掉线", tone: "bad",
             hint: "连接还在但 QQ 会话已经没了（" + (st.live_error || st.detail || "探测失败") + "）" +
                   (offSec > 0 ? "，已掉线 " + Math.round(offSec / 60) + " 分钟" : "") +
                   "，系统正在自动重连" };
  }
  // 状态接口本身连续几次读不到：不能装作还连着，但也不能谎报掉线。
  if (s === "open" && st.stale) {
    return { key: "connecting", label: "状态读取失败", tone: "busy",
             hint: "连续几次没读到机器人状态，可能是网络断了，正在重试" };
  }
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
  var last = yyQQLastMsgAt();
  if (last) meta.push("最近消息 " + yyQQAgo(last));
  // 掉线时把「会不会自己回来」直接写在状态带上：
  // 用户不用翻到下面找「掉线自愈」，一眼就知道要不要自己动手。
  if (v.tone !== "ok" && yyQQC.login && yyQQC.login.watchdog) {
    var wd = yyQQC.login.watchdog;
    meta.push(wd.enabled
      ? "自动重连已开启，每 " + (wd.interval_seconds || 60) + " 秒重试"
      : "自动重连已关闭");
  }
  var drop = yyQQDropBanner();
  return '<div class="yyq-status" id="yyq-status" data-state="' + yyQQE(v.tone) + '" data-drop="' + (drop ? "1" : "0") + '">' +
    '<span class="yyq-dot" aria-hidden="true"></span>' +
    '<span class="yyq-status-text">' + yyQQE(v.label) + "</span>" +
    '<span class="yyq-status-meta">' + yyQQE(meta.join(" · ")) + "</span>" +
    (v.key === "need_login" || v.key === "unconfigured" || v.key === "error" || v.key === "off"
      ? '<button type="button" class="yyq-status-cta" onclick="yyQQOpenAccess()">去接入</button>'
      : "") +
    '<button type="button" class="yyq-iconbtn" onclick="yyQQRefreshAll(0,this)" aria-label="刷新状态" title="刷新状态">' + yyQQIcon("refresh") + "</button>" +
    '<button type="button" class="yyq-iconbtn" onclick="yyQQToggleMenu(event)" aria-label="更多操作" title="更多">' + yyQQIcon("more") + "</button>" +
    (yyQQC.menu ? '<div class="yyq-menu" role="menu">' +
      '<button type="button" role="menuitem" onclick="yyQQRefreshAll(1,this)">刷新全部数据</button>' +
      '<button type="button" role="menuitem" onclick="yyQQMenuFaces()">读取收藏表情</button>' +
      "</div>" : "") +
    yyQQDropBanner() +
    "</div>";
}

// 掉线横幅：以前掉线了页面一点反应都没有，用户得自己盯着才发现。
// 现在只要「上一轮是连着、这一轮断了」就记一次，并在顶部挂一条横幅，
// 10 分钟内的掉线一直显示，带重连按钮和掉线时间。
function yyQQDropBanner() {
  var at = yyQQC.dropAt || 0;
  if (!at) return "";
  if (Date.now() - at > 10 * 60 * 1000) return "";
  var mins = Math.max(0, Math.floor((Date.now() - at) / 60000));
  var when = mins < 1 ? "刚刚" : mins + " 分钟前";
  return '<div class="yyq-drop" role="alert">' +
    '<span class="yyq-drop-ico">' + yyQQIcon("connection") + "</span>" +
    '<span class="yyq-drop-text"><b>机器人掉线了</b><small>' + yyQQE(when) + "断开 · " +
      yyQQE(yyQQC.dropReason || "连接中断") + "</small></span>" +
    '<button type="button" class="yyq-drop-act" onclick="yyQQReconnectNow()">重连</button>' +
    '<button type="button" class="yyq-drop-x" onclick="yyQQDropDismiss()" aria-label="收起">×</button>' +
    "</div>";
}
function yyQQDropDismiss() {
  yyQQC.dropAt = 0;
  yyQQRender("status");
}
async function yyQQReconnectNow() {
  yyQQC.dropAt = 0;
  yyQQRender("status");
  var r = await A("POST", "/api/qq/reconnect", {});
  if (r && r.ok) {
    if (typeof T === "function") T("已发起重连", "s");
  } else if (typeof T === "function") {
    T((r && r.error) || "重连请求失败", "e");
  }
  setTimeout(function () { yyQQRefreshAll(); }, 1200);
}
// 只在「连过 → 断了」的跳变时触发，避免开机就没连上也一直弹。
function yyQQCheckDrop(nowConnected, reason) {
  var was = yyQQC.lastConnected;
  if (was === true && nowConnected === false) {
    yyQQC.dropAt = Date.now();
    yyQQC.dropReason = reason || "";
    if (typeof T === "function") T("机器人掉线了：" + (reason || "连接中断"), "e");
  }
  if (nowConnected === true) {
    yyQQC.dropAt = 0;
    yyQQC.dropReason = "";
  }
  yyQQC.lastConnected = nowConnected;
}

function yyQQToggleMenu(ev) {  if (ev && ev.stopPropagation) ev.stopPropagation();
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
  yyQQC.modOpen.chat = true;
  yyQQOpenModule("chat", "yyq-access");
}

// ---------- 1 身份卡 ----------

function yyQQHero() {
  var v = yyQQView(), p = yyQQC.profile || {}, cfg = yyQQC.cfg || {}, lg = yyQQC.login || {};
  // 头像：底层永远铺一个机器人图标，图片加载失败时自己移除，露出底图。
  var avatar = '<span class="yyq-avatar-wrap">' +
    '<span class="yyq-avatar-fb">' + yyQQIcon("robot") + "</span>" +
    (p.avatar ? '<img class="yyq-avatar" src="' + yyQQE(p.avatar) + '" alt="" referrerpolicy="no-referrer" onerror="this.remove()">' : "") +
    "</span>";

  // 群名交给消息面板顶部的群标签，这里不再重复列一遍；
  // 链路图（NapCat → 桥接 → N 个群）也撤掉了——状态带已经说了通不通，
  // 那三条只是把同一件事拆成三层再讲一次。
  var today = yyQQTodayActions();
  var facts = [
    ["游戏身份", (cfg.assistant && cfg.assistant.bot_player) || "未绑定"],
    ["今日动作", today.total ? String(today.total) + (today.fail ? " 失败 " + today.fail : "") : "—"]
  ];

  return '<section class="yyq-hero" id="yyq-hero" data-state="' + yyQQE(v.tone) + '"' + (v.key === "need_login" || v.key === "need_scan" ? ' data-qr="1"' : "") + ">" +
    '<div class="yyq-hero-main">' +
      '<div class="yyq-hero-id">' + avatar +
        '<div class="yyq-hero-name"><strong>' + yyQQE(p.nickname || "机器人资料未读取") + "</strong>" +
          '<div class="yyq-hero-sub"><span>' + yyQQE(yyQQMaskID(p.user_id || (cfg.onebot && cfg.onebot.bot_id)) || "QQ 未读取") + "</span></div>" +
        "</div>" +
      "</div>" +
      '<dl class="yyq-facts">' + facts.map(function (f) {
        return "<div><dt>" + yyQQE(f[0]) + "</dt><dd>" + yyQQE(f[1]) + "</dd></div>";
      }).join("") + "</dl>" +
      (v.hint && v.tone !== "ok" ? '<p class="yyq-hero-hint">' + yyQQE(v.hint) + "</p>" : "") +
    "</div>" +
    (v.key === "need_login" || v.key === "need_scan" ? yyQQQRCode(lg) : "") +
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
  // 只有「真的在刷新」才盖遮罩。以前用 !q.fresh 判断，而 NapCat 不会定时重写
  // qrcode.png，文件时间一旧 fresh 就永远是 false → 遮罩永远盖着，二维码扫不了。
  return '<div class="yyq-qr" id="yyq-qr" data-fresh="' + (q.fresh ? "1" : "0") + '" data-stale="' + (q.stale ? "1" : "0") + '">' +
    '<div class="yyq-qr-box">' +
      (has
        ? '<img src="' + yyQQE(q.data_url) + '" alt="QQ 登录二维码" decoding="async">'
        : '<span class="yyq-qr-none">' + yyQQIcon("qr") + "<small>暂未取到二维码</small></span>") +
      (has && yyQQC.qrBusy ? '<span class="yyq-qr-mask">正在重新生成…</span>' : "") +
    "</div>" +
    '<p class="yyq-qr-hint">' + (has
      ? '二维码 <b id="yyq-qr-age">' + (q.age_seconds || 0) + "</b> 秒前生成" + (q.stale ? "，<span class=\"yyq-qr-stale\">可能已过期</span>" : "")
      : (lg && lg.qr_missing ? "协议端还没生成二维码，点下面「重新生成二维码」" : "等待 NapCat 生成二维码")) + "</p>" +
    '<ol class="yyq-qr-steps"><li class="is-on">手机 QQ 扫一扫</li><li>在手机上确认登录</li><li>机器人自动上线</li></ol>' +
    // 出码按钮必须一直在。以前只在 q.stale 或没图时才渲染，
    // 而 NapCat 不会定时重写 qrcode.png、fresh 永远 false 但 stale 也可能 false，
    // 结果「登录态过期」时用户根本找不到重新生成二维码的入口。
    // 后端 /api/qq/login/refresh 是真实存在的（youyuan_qq_login.go:264），
    // 没配 YY_NAPCAT_RESTART_CMD 时它会明确回错误，这里原样显示，不做假按钮。
    '<div class="yyq-qr-actions"><button type="button" class="btn-p" onclick="yyQQQrRefresh(this)">重新生成二维码</button>' +
      '<button type="button" class="btn-s" onclick="yyQQPollLogin(1)">重新读取</button></div>' +
      '<p class="yyq-qr-tip" id="yyq-qr-tip"></p>' +
    yyQQLoginModes(lg) +
    "</div>";
}

// 登录方式只列「现在真的能用」的两条：扫码、快速登录。
// 以前把不可用的账号密码登录连同三段技术解释一起铺出来，占了半屏还让人以为能选；
// 掉线之后怎么办由下面的「掉线自愈」直接管，不给假按钮。
function yyQQLoginModes(lg) {
  var modes = ((lg && lg.login_modes) || []).filter(function (m) { return m && m.available; });
  if (!modes.length) return "";
  var rows = modes.map(function (m) {
    var tag = m.active ? '<em class="yyq-mode-tag">当前</em>' : "";
    return '<li class="yyq-mode' + (m.active ? " is-on" : "") + '">' +
      '<span class="yyq-mode-dot" aria-hidden="true"></span>' +
      '<div class="yyq-mode-main"><b>' + yyQQE(m.label) + "</b>" + tag + "</div></li>";
  }).join("");

  return '<div class="yyq-modes">' +
    '<ul class="yyq-mode-list">' + rows + "</ul>" +
    yyQQWatchdogBox(lg) +
    "</div>";
}

// 掉线自愈状态条。已登录时也显示（登录正常不代表以后不掉），
// 让用户随时能看到「这台机器掉线会不会自己回来」。
function yyQQWatchdogBox(lg) {
  var w = (lg && lg.watchdog) || {};
  var tone = "is-ok", text = "";
  if (!w.has_restart_cmd) {
    tone = "is-bad";
    text = "未配置重启命令，掉线后只能人工扫码";
  } else if (w.cool_seconds) {
    tone = "is-idle";
    text = "暂停中，还需 " + Math.ceil(w.cool_seconds / 60) + " 分钟" + (w.last_error ? "：" + w.last_error : "");
  } else if (w.last_error) {
    tone = "is-bad";
    text = "上次自愈失败：" + w.last_error;
  } else if (w.last_restart_at) {
    text = "近 1 小时已自动重启 " + (w.restarts_1h || 0) + " 次，最近 " + w.last_restart_at;
  } else {
    text = "已开启，掉线后自动重启协议端走快速登录（不删登录态、不逼你扫码）";
  }
  var on = !!w.enabled;
  // A3：不再用整块红色警示底，改成和页面一致的中性状态条 + 一个小状态点；
  // 红色只留给「自愈失败 / 未配置重启命令」两种真需要人工介入的情况。
  //
  // 重连上限与间隔是真能改的：后端 POST /api/qq/watchdog 早就收
  // interval_seconds / max_restarts 并落盘，GET 也一直把当前值回在
  // interval_seconds / max_restarts 里。之前这里读的是 max_per_hour、
  // 间隔写死 60，再加个 disabled 和「后端待支持」，等于把已经通了的路堵死。
  var maxN = w.max_restarts || w.max_per_hour || 3;
  var intervalS = w.interval_seconds || 60;
  return '<div class="yyq-wd" data-tone="' + tone + '">' +
    '<span class="yyq-wd-dot" aria-hidden="true"></span>' +
    '<div class="yyq-wd-main"><b>掉线自愈</b><span>' + yyQQE(text) + "</span></div>" +
    '<button type="button" class="btn-s" onclick="yyQQWatchdogToggle(this)" data-on="' + (on ? "1" : "0") + '">' +
      (on ? "已开启" : "已关闭") + "</button>" +
    '<div class="yyq-wd-cfg">' +
      '<label class="yyq-wd-field"><span>重连次数上限</span>' +
        '<input id="yyq-wd-max" type="number" min="1" max="100" step="1" value="' + maxN + '" aria-label="重连次数上限"></label>' +
      '<label class="yyq-wd-field"><span>间隔（秒）</span>' +
        '<input id="yyq-wd-int" type="number" min="10" max="3600" step="10" value="' + intervalS + '" aria-label="重连间隔秒数"></label>' +
      '<button type="button" class="btn-s" onclick="yyQQWatchdogSave(this)">保存</button>' +
    "</div>" +
    "</div>";
}

async function yyQQWatchdogSave(btn) {
  var maxEl = document.getElementById("yyq-wd-max");
  var intEl = document.getElementById("yyq-wd-int");
  var maxN = parseInt((maxEl && maxEl.value) || "", 10);
  var intS = parseInt((intEl && intEl.value) || "", 10);
  if (!(maxN >= 1 && maxN <= 100)) { T("重连次数填 1–100", "e"); return; }
  if (!(intS >= 10 && intS <= 3600)) { T("间隔填 10–3600 秒", "e"); return; }
  if (btn) btn.disabled = true;
  var r = await A("POST", "/api/qq/watchdog", { interval_seconds: intS, max_restarts: maxN });
  if (btn) btn.disabled = false;
  if (!r || !r.ok) { T((r && r.error) || "保存失败", "e"); return; }
  T("掉线自愈已更新", "o");
  yyQQPollLogin(1);
}

async function yyQQWatchdogToggle(btn) {
  var next = !(btn && btn.getAttribute("data-on") === "1");
  if (btn) btn.disabled = true;
  var r = await A("POST", "/api/qq/watchdog", { enabled: next });
  if (btn) btn.disabled = false;
  if (!r || !r.ok) return;
  yyQQPollLogin(1);
}

// 真正去让 NapCat 重出二维码。后端没配重启命令时会明确说不行，这里原样转达，
// 不再用「正在刷新…」敷衍。
async function yyQQQrRefresh(btn) {
  var tip = document.getElementById("yyq-qr-tip");
  yyQQC.qrBusy = 1;
  if (btn) btn.disabled = true;
  if (tip) tip.textContent = "正在重新生成，约需 10-20 秒…";
  var r = await A("POST", "/api/qq/login/refresh", {});
  if (!r || !r.ok) {
    if (tip) {
      tip.setAttribute("data-bad", "1");
      tip.textContent = (r && r.error) || "刷新失败";
    }
    yyQQC.qrBusy = 0;
    if (btn) btn.disabled = false;
    return;
  }
  if (tip) tip.textContent = "已重启，等待新二维码…";
  setTimeout(function () {
    yyQQC.qrBusy = 0;
    yyQQPollLogin(1);
    setTimeout(function () {
      if (btn && document.getElementById("yyq-qr")) btn.disabled = false;
    }, 800);
  }, 6000);
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
    '<p class="yyq-caps-note">点开任意一项可改冷却、每日次数、最低角色；关掉就等于禁用这条能力</p>' +
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
    yyQQSkills() +
    '<p class="yyq-cap-foot">开关与参数即时生效 · 所有动作均留痕</p></section>';
}

// ---------- 2b 技能系统（AI 自己能用的能力） ----------
//
// 用户原话精髓：
//   「提示词里面告诉他，可以使用技能[meme] 他输出使用 能看到所有收藏的表情包
//     然后自己可以选择表情包发出去 他想点几个都行」
//   「玩家想改特定的功能词也可以设置 [meme] 他可以把表情包换成中文的表情包」
//   「更多的一些相应功能都要让玩家去配置，但是又不能让UI太丑 太多，太挤……
//    要展开的时候再展开」
//
// 所以：默认折叠成「技能（N）」一块，展开才逐行出编辑（功能词/说明/保存/
// 写入提示词）；数据只读视图 GET /api/qq/assistant/skills，保存走
// POST /api/qq/config 的 assistant.skills（PATCH 深合并，只发改的字段）。

function yyQQSkills() {
  var rows = yyQQC.skills || [];
  var sum = yyQQC.skillsLoading && !yyQQC.skillsLoaded ? "读取中…"
    : (yyQQC.skillsErr && !yyQQC.skillsLoaded ? "读取失败"
    : (yyQQC.skillsLoaded ? (rows.length ? rows.length + " 项" : "暂无") : "未读取"));
  var body;
  if (yyQQC.skillsLoading && !yyQQC.skillsLoaded) {
    body = '<ul class="yyq-cap-list">' + yyQQSkeleton(4, 60) + "</ul>";
  } else if (yyQQC.skillsErr && !yyQQC.skillsLoaded) {
    body = '<p class="yyq-inline-err">' + yyQQIcon("ban") + "<span>" + yyQQE(yyQQC.skillsErr) + "</span>" +
      '<button type="button" class="yyq-linkbtn" onclick="yyQQLoadSkills()">重试</button></p>';
  } else if (!rows.length) {
    body = yyQQEmpty("spark", "还没有读到技能", "后端未返回技能定义，稍后重试", "重试", "yyQQLoadSkills()");
  } else {
    body = '<ul class="yyq-cap-list">' + rows.map(yyQQSkillRow).join("") + "</ul>" + yyQQDelayRow();
  }
  return '<details class="yyq-fold" id="yyq-skills"' + (yyQQC.skillsOpen ? " open" : "") + ' ontoggle="yyQQSkillsFold(this)">' +
    "<summary>" +
      '<span class="yyq-fold-ico">' + yyQQIcon("spark") + "</span>" +
      '<span class="yyq-fold-t">技能</span>' +
      '<span class="yyq-fold-sum">' + yyQQE(sum) + "</span>" +
      '<span class="yyq-fold-caret">' + yyQQIcon("arrow") + "</span>" +
    "</summary>" +
    '<div class="yyq-fold-body">' +
      body +
    "</div></details>";
}

// ---------- 2c AI 工具（全量清单 + 逐项开关） ----------
//
// 用户原话：「我都要求了，把那些工具给 AI 你就给了几个」。
// 根因：工具注册表（youyuan_qq_toolbox.go：30 项、分组、中文说明、默认值）
// 和 /api/qq/ai/tools 接口早就写好了，但前端从来没有消费它——用户唯一能
// 看到工具的地方是自检里两行英文工具名，和助手面板一排「插入提示词」的
// 英文 chips：没有中文名、没有分组、更不能开关。
// 这里把注册表全量摊开：分组标题、中文名、一句话说明、危险标、角色标、
// 逐项开关。保存走 POST /api/qq/ai/tools（深合并，只发改动项），后端
// yyQQAITools 按 assistant.tools 过滤，关掉就真的不下发给模型；
// 技能闸门同步生效（youyuan_qq_skills.go 的 yyQQSkillToolOK）。
// 默认折叠成一行「AI 工具（N / M 项开启）」，展开才拉数据，不占首屏。

function yyQQToolSum() {
  if (yyQQC.toolsLoading && !yyQQC.toolsLoaded) return "读取中…";
  if (yyQQC.toolErr && !yyQQC.toolsLoaded) return "读取失败";
  if (!yyQQC.toolsLoaded) return "未读取";
  var on = 0, total = 0;
  (yyQQC.toolGroups || []).forEach(function (g) {
    (g.tools || []).forEach(function (t) {
      total++;
      if (t.enabled) on++;
    });
  });
  return on + " / " + total + " 项开启";
}
function yyQQTools() {
  var body;
  if (yyQQC.toolsLoading && !yyQQC.toolsLoaded) {
    body = '<ul class="yyq-cap-list">' + yyQQSkeleton(5, 60) + "</ul>";
  } else if (yyQQC.toolErr && !yyQQC.toolsLoaded) {
    body = '<p class="yyq-inline-err">' + yyQQIcon("ban") + "<span>" + yyQQE(yyQQC.toolErr) + "</span>" +
      '<button type="button" class="yyq-linkbtn" onclick="yyQQLoadTools()">重试</button></p>';
  } else if (!(yyQQC.toolGroups || []).length) {
    body = yyQQEmpty("spark", "还没有读到工具清单", "后端未返回工具定义，稍后重试", "重试", "yyQQLoadTools()");
  } else {
    body = (yyQQC.toolGroups || []).map(yyQQToolGroup).join("");
  }
  return yyQQSec(yyQQSecHead("AI 工具", "关掉就真的不下发给模型") + body);
}
function yyQQToolGroup(g) {
  var tools = g.tools || [];
  var on = 0;
  tools.forEach(function (t) { if (t.enabled) on++; });
  return '<div class="yyq-tool-group" data-group="' + yyQQE(g.id) + '">' +
    '<h4>' + yyQQE(g.label) + "<small>" + on + " / " + tools.length + " 项开启</small></h4>" +
    (g.note ? '<p class="yyq-tool-group-note">' + yyQQE(g.note) + "</p>" : "") +
    '<ul class="yyq-cap-list">' + tools.map(yyQQToolRow).join("") + "</ul></div>";
}
function yyQQToolRow(t) {
  var on = !!t.enabled;
  var role = t.min_role >= 2 ? "超管" : (t.min_role === 1 ? "管理员" : "全员");
  return '<li class="yyq-cap yyq-tool" data-id="' + yyQQE(t.id) + '" data-on="' + (on ? "1" : "0") + '">' +
    '<span class="yyq-cap-ico">' + yyQQIcon(t.danger ? "ban" : "spark") + "</span>" +
    '<span class="yyq-cap-body"><strong>' + yyQQE(t.label) +
      (t.danger ? '<i class="yyq-cap-dot" style="background:var(--phx-danger,#e5484d)" title="危险操作：默认关闭，开启后 AI 才可用"></i>' : "") +
      '<code class="yyq-cap-cmd">' + yyQQE(t.id) + "</code>" +
      "</strong><small>" + yyQQE(t.desc || "") + "</small></span>" +
    '<span class="yyq-cap-meta">' + (t.available === false ? "当前角色不可用" : role) + "</span>" +
    '<label class="yyq-sw"><input type="checkbox"' + (on ? " checked" : "") +
      ' onchange="yyQQToolToggle(this)" aria-label="' + yyQQE((t.label || t.id) + " 工具开关") + '"><span></span></label>' +
    "</li>";
}
function yyQQToolsFold(el) {
  yyQQC.toolsOpen = !!el.open;
  // 展开才读数据：不占首屏请求，也符合「要展开的时候再展开」。
  if (el.open && !yyQQC.toolsLoaded && !yyQQC.toolsLoading) yyQQLoadTools();
}
async function yyQQLoadTools() {
  if (yyQQC.toolsLoading) return;
  yyQQC.toolsLoading = 1;
  yyQQRender("mod-ai");
  var r = await A("GET", "/api/qq/ai/tools");
  yyQQC.toolsLoading = 0;
  if (!r || !r.ok) {
    yyQQC.toolErr = (r && r.error) || "读取工具失败";
  } else {
    yyQQC.toolGroups = r.groups || [];
    yyQQC.toolTotal = r.total || 0;
    yyQQC.toolErr = "";
    yyQQC.toolsLoaded = 1;
  }
  yyQQRender("mod-ai");
}
// 工具开关：POST /api/qq/ai/tools 深合并，只发这一项，保存即时生效。
async function yyQQToolToggle(input) {
  var li = input.closest ? input.closest(".yyq-tool") : null;
  var id = li && li.getAttribute("data-id");
  if (!id) return;
  var want = !!input.checked;
  var patch = { tools: {} };
  patch.tools[id] = want;
  var r = await A("POST", "/api/qq/ai/tools", patch);
  if (!r || !r.ok) {
    input.checked = !want;
    T((r && r.error) || "保存失败", "e");
    return;
  }
  // 本地同步：重渲染后开关状态与新值一致，不等下一轮刷新。
  (yyQQC.toolGroups || []).forEach(function (g) {
    (g.tools || []).forEach(function (t) { if (t.id === id) t.enabled = want; });
  });
  yyQQRender("mod-ai");
  T(want ? "已开启，AI 下一轮就能用" : "已关闭，AI 不再拿到这个工具", "o");
}

// 单个技能行：名称 + 生效功能词 chip + 危险标 + 开关；展开后才出功能词输入/说明/保存。
// cap_off（底层能力被关掉）时整行置灰并禁用开关，小字说明原因。
function yyQQSkillRow(s) {
  var id = s.id || "";
  var off = !!s.cap_off;
  var on = !!s.enabled && !off;
  var open = yyQQC.openSkill === id;
  var word = s.word || "";
  return '<li class="yyq-cap" data-id="' + yyQQE(id) + '" data-on="' + (on ? "1" : "0") + '" data-open="' + (open ? "1" : "0") + '"' +
      ' tabindex="0" onclick="yyQQSkillDetail(event,this)" onkeydown="yyQQSkillKey(event,this)" aria-expanded="' + (open ? "true" : "false") + '">' +
    '<span class="yyq-cap-ico">' + yyQQIcon("spark") + "</span>" +
    '<span class="yyq-cap-body"><strong>' + yyQQE(s.label || id) +
      ' <code class="yyq-cap-cmd">技能[' + yyQQE(word) + "]</code>" +
      (s.danger ? '<i class="yyq-cap-dot" style="background:var(--phx-danger,#e5484d)" title="危险操作：默认关闭，去「AI 工具」里打开后才可用"></i>' : "") +
      "</strong>" +
      "<small>" + yyQQE(off ? "底层能力已关闭，去上面「能力」里打开" : (s.desc || "")) + "</small></span>" +
    '<span class="yyq-cap-meta">' + (off ? '<i class="yyq-cap-nb">不可用</i>' : (on ? "AI 可用" : "已停用")) + "</span>" +
    '<label class="yyq-sw"><input type="checkbox"' + (on ? " checked" : "") + (off ? " disabled" : "") +
      ' onchange="yyQQSkillToggle(this)" aria-label="' + yyQQE((s.label || id) + " 技能开关") + '"><span></span></label>' +
    '<div class="yyq-cap-edit" onclick="event.stopPropagation()">' +
      '<label class="yyq-cap-f"><span>生效功能词（AI 看到的就是它）</span>' +
        '<input type="text" maxlength="20" data-f="word" value="' + yyQQE(word) + '" placeholder="' + yyQQE(word) + '"></label>' +
      '<div class="yyq-cap-edit-act">' +
        '<button type="button" class="btn-s" onclick="yyQQSkillSave(this)">保存功能词</button>' +
        '<button type="button" class="btn-s" onclick="yyQQSkillPrompt(this)">写入提示词</button>' +
        '<span class="yyq-cap-edit-msg"></span>' +
      "</div>" +
      '<p class="yyq-cap-edit-note">' + yyQQE(s.desc || "") +
        (s.tool ? "（底层工具：" + yyQQE(s.tool) + "）" : "") + "</p>" +
    "</div></li>";
}

function yyQQSkillDetail(ev, li) {
  if (ev && ev.target && ev.target.closest && (ev.target.closest(".yyq-sw") || ev.target.closest(".yyq-cap-edit"))) return;
  var open = li.getAttribute("data-open") === "1";
  li.setAttribute("data-open", open ? "0" : "1");
  li.setAttribute("aria-expanded", open ? "false" : "true");
  yyQQC.openSkill = open ? "" : li.getAttribute("data-id");
}
function yyQQSkillKey(ev, li) {
  if (ev.key !== "Enter" && ev.key !== " ") return;
  ev.preventDefault();
  yyQQSkillDetail(ev, li);
}
function yyQQSkillsFold(el) {
  yyQQC.skillsOpen = !!el.open;
  // 展开才读数据：不占首屏请求，也符合「要展开的时候再展开」。
  if (el.open && !yyQQC.skillsLoaded && !yyQQC.skillsLoading) yyQQLoadSkills();
}
async function yyQQLoadSkills() {
  if (yyQQC.skillsLoading) return;
  yyQQC.skillsLoading = 1;
  yyQQRender("caps");
  var r = await A("GET", "/api/qq/assistant/skills");
  yyQQC.skillsLoading = 0;
  if (!r || !r.ok) {
    yyQQC.skillsErr = (r && r.error) || "读取技能失败";
  } else {
    yyQQC.skills = r.skills || [];
    yyQQC.skillsDelay = r.delay || null;
    yyQQC.skillsErr = "";
    yyQQC.skillsLoaded = 1;
  }
  yyQQRender("caps");
}
function yyQQSkillMsg(li, text, bad) {
  var el = li && li.querySelector ? li.querySelector(".yyq-cap-edit-msg") : null;
  if (!el) return;
  el.textContent = text || "";
  el.setAttribute("data-bad", bad ? "1" : "0");
}

// 技能开关：POST /api/qq/config 的 assistant.skills[id].enabled（深合并，只发这一项）。
async function yyQQSkillToggle(input) {
  var li = input.closest ? input.closest(".yyq-cap") : null;
  var id = li && li.getAttribute("data-id");
  if (!id) return;
  var want = !!input.checked;
  var patch = { assistant: { skills: {} } };
  patch.assistant.skills[id] = { enabled: want };
  var r = await A("POST", "/api/qq/config", patch);
  if (!r || !r.ok) {
    input.checked = !want;
    T((r && r.error) || "保存失败", "e");
    return;
  }
  (yyQQC.skills || []).forEach(function (x) { if (x.id === id) x.enabled = want; });
  yyQQRender("caps");
  T(want ? "技能已开启" : "技能已关闭", "o");
}

// 保存功能词：空 = 用注册表默认词（后端 TrimSpace 后为空即回退）。
async function yyQQSkillSave(btn) {
  var li = btn.closest ? btn.closest(".yyq-cap") : null;
  var id = li && li.getAttribute("data-id");
  if (!id) return;
  var box = li.querySelector(".yyq-cap-edit");
  var inp = box ? box.querySelector('[data-f="word"]') : null;
  var word = String((inp && inp.value) || "").trim().slice(0, 20);
  btn.disabled = true;
  yyQQSkillMsg(li, "保存中…");
  var patch = { assistant: { skills: {} } };
  patch.assistant.skills[id] = { word: word };
  var r = await A("POST", "/api/qq/config", patch);
  btn.disabled = false;
  if (!r || !r.ok) {
    yyQQSkillMsg(li, (r && r.error) || "保存失败", 1);
    return;
  }
  if (r.config) yyQQC.cfg = r.config;
  // 生效功能词以服务端为准（清空时要知道回退到哪个默认词），回读一次列表
  yyQQC.skillsLoaded = 0;
  await yyQQLoadSkills();
  var fresh = document.querySelector('.yyq-cap[data-id="' + id + '"]');
  if (fresh) yyQQSkillMsg(fresh, "已保存");
  T("功能词已保存", "o");
}

// 「写入提示词」：把 技能[功能词] 插进 AI 提示词输入框（#yy-qq-ai-prompt，
// 群服互通助手面板的人设区）。用户原话：「就在那个能力列表里面，点一下就
// 自动跳到提示词里面」。当时不在 DOM 里（助手面板没挂上）就先把折叠块展开。
function yyQQSkillPrompt(btn) {
  var li = btn.closest ? btn.closest(".yyq-cap") : null;
  var id = li && li.getAttribute("data-id");
  if (!id) return;
  var box = li.querySelector(".yyq-cap-edit");
  var inp = box ? box.querySelector('[data-f="word"]') : null;
  var word = String((inp && inp.value) || "").trim();
  if (!word) {
    var s = null;
    (yyQQC.skills || []).forEach(function (x) { if (x.id === id) s = x; });
    word = (s && s.word) || "";
  }
  if (!word) { T("这个技能没有功能词", "e"); return; }
  var text = "技能[" + word + "]";
  var ta = document.getElementById("yy-qq-ai-prompt");
  if (!ta) {
    var fold = document.getElementById("yyq-assistant");
    if (fold && !fold.open) fold.setAttribute("open", "");
    T("助手面板已展开，再点一次「写入提示词」", "w");
    return;
  }
  var s0 = ta.selectionStart == null ? ta.value.length : ta.selectionStart;
  var e0 = ta.selectionEnd == null ? s0 : ta.selectionEnd;
  // 与助手面板的 yyAInsertTool 一致：贴着文字插入时自动补空格，避免黏成一个词
  var ins = text;
  if (s0 > 0 && !/\s$/.test(ta.value.slice(0, s0))) ins = " " + ins;
  if (e0 < ta.value.length && !/^\s/.test(ta.value.slice(e0))) ins += " ";
  ta.value = ta.value.slice(0, s0) + ins + ta.value.slice(e0);
  try { ta.setSelectionRange(s0 + ins.length, s0 + ins.length); } catch (err) { /* 旧内核忽略 */ }
  ta.focus();
  if (ta.scrollIntoView) ta.scrollIntoView({ block: "center", behavior: "smooth" });
  T("已写入提示词：" + text, "o");
}

// AI 回复间隔（用户第 17 条：「回复的间隔也没有一个设置」）。
// 待定（双 0）时明确写出内置兜底 2-6 秒；设过即显示生效数值。
function yyQQDelayRow() {
  var d = yyQQC.skillsDelay || {};
  var pending = !!d.pending;
  var fbMin = d.fallback_min == null ? 2 : d.fallback_min;
  var fbMax = d.fallback_max == null ? 6 : d.fallback_max;
  var state = pending
    ? "待定 · 内置兜底 " + fbMin + "–" + fbMax + " 秒"
    : "随机 " + (d.effective_min == null ? 0 : d.effective_min) + "–" + (d.effective_max == null ? 0 : d.effective_max) + " 秒";
  return '<div class="yyq-delay" style="margin-top:12px;padding-top:10px;border-top:1px dashed var(--phx-border-light)">' +
    '<div class="yyq-block-head"><h4>AI 回复间隔</h4>' +
      '<span class="yyq-tagline">' + yyQQE(state) + "</span></div>" +
    '<div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(120px,1fr));gap:8px;margin-top:8px">' +
      '<label class="yyq-cap-f"><span>最短（秒）</span><input type="number" min="0" max="600" step="1" id="yyq-delay-min" value="' + (d.min || 0) + '"></label>' +
      '<label class="yyq-cap-f"><span>最长（秒）</span><input type="number" min="0" max="600" step="1" id="yyq-delay-max" value="' + (d.max || 0) + '"></label>' +
    "</div>" +
    '<div class="yyq-cap-edit-act" style="margin-top:8px">' +
      '<button type="button" class="btn-s" onclick="yyQQDelaySave(this)">保存间隔</button>' +
      '<span class="yyq-cap-edit-msg" id="yyq-delay-msg"></span>' +
    "</div>" +
    '<p class="yyq-cap-edit-note">AI 每次开口前会随机等一会儿，不像秒回那么假。两边都填 0 = 待定，用内置兜底。</p>' +
  "</div>";
}
async function yyQQDelaySave(btn) {
  function num(id) {
    var el = document.getElementById(id);
    var v = parseInt(el && el.value, 10);
    if (isNaN(v) || v < 0) v = 0;
    if (v > 600) v = 600;
    return v;
  }
  var min = num("yyq-delay-min"), max = num("yyq-delay-max");
  btn.disabled = true;
  var msg = document.getElementById("yyq-delay-msg");
  if (msg) { msg.textContent = "保存中…"; msg.setAttribute("data-bad", "0"); }
  var r = await A("POST", "/api/qq/config", { assistant: { reply_delay_min: min, reply_delay_max: max } });
  btn.disabled = false;
  if (!r || !r.ok) {
    if (msg) { msg.textContent = (r && r.error) || "保存失败"; msg.setAttribute("data-bad", "1"); }
    return;
  }
  if (r.config) yyQQC.cfg = r.config;
  // 回读生效区间：min>max 时后端会交换，以服务端算出的为准
  var rr = await A("GET", "/api/qq/assistant/skills");
  if (rr && rr.ok && rr.delay) yyQQC.skillsDelay = rr.delay;
  yyQQRender("caps");
  T("回复间隔已保存", "o");
}

function yyQQCapRow(c) {
  var kind = c.kind || "";
  var cmdId = yyQQCapCmdId[kind];
  var on = !!c.enabled;
  // 适配器差异：OneBot(NapCat) 独有能力（点赞/动态/改头像…）在 QQ 官方适配器下根本没有接口，
  // 以前照样显示开关，点了也没用。现在直接标出来并禁用，避免"看起来能开其实不能"。
  var unsupported = c.onebot_only && yyQQC.adapter === "qqofficial";
  // 限制默认是「待定」：用户明确要求不要我们替他拍一个默认值。
  // 待定时整行只显示一个「待定」标记，提醒他点进去自己定。
  var pending = !!c.pending;
  var limits = [];
  if (c.cool_seconds > 0) limits.push("冷却 " + yyQQMinutes(c.cool_seconds));
  if (c.gap_seconds > 0) limits.push("间隔 " + c.gap_seconds + " 秒");
  if (c.daily > 0) limits.push("每日 " + c.daily + " 次");
  if (pending) limits = ["待定"];
  else if (!limits.length) limits.push("不限流");
  var open = yyQQC.openCap === kind;
  var busy = yyQQC.busy["cap:" + kind];
  return '<li class="yyq-cap" data-kind="' + yyQQE(kind) + '" data-on="' + (on ? "1" : "0") + '" data-open="' + (open ? "1" : "0") + '"' +
      ' data-unsupported="' + (unsupported ? "1" : "0") + '"' +
      ' tabindex="0" onclick="yyQQCapDetail(event,this)" onkeydown="yyQQCapKey(event,this)" aria-expanded="' + (open ? "true" : "false") + '">' +
    '<span class="yyq-cap-ico">' + yyQQIcon(yyQQCapIcon(kind)) + "</span>" +
    '<span class="yyq-cap-body"><strong>' + yyQQE(c.label || kind) + "</strong>" +
      "<small>" + (unsupported ? "官方适配器没有这个接口，切到 NapCat 才能用" : yyQQE(c.desc || "")) + "</small></span>" +
    '<span class="yyq-cap-meta">' + (unsupported ? '<i class="yyq-cap-nb">不支持</i>' : (pending ? '<i class="yyq-cap-nb" data-pending="1">待定</i>' : yyQQE(limits.join(" · "))) + (c.custom ? '<i class="yyq-cap-dot" title="参数已被你改过"></i>' : "")) + "</span>" +
    '<label class="yyq-sw" data-busy="' + (busy ? "1" : "0") + '"><input type="checkbox"' + (on ? " checked" : "") + (unsupported ? " disabled" : "") +
      ' onchange="yyQQToggleCap(this)" aria-label="' + yyQQE((c.label || kind) + " 开关") + '"><span></span></label>' +
    '<span class="yyq-cap-more">' +
      '<span class="yyq-cap-lv">' + yyQQE(c.min_level_text || "成员") + "起可用</span>" +
      // 指令格式可改：有对应指令 ID 的能力给小输入框（placeholder = 内置默认），
      // 改完保存到 command_aliases；留空保存 = 恢复默认。没有指令 ID 的保持只读。
      (c.command && cmdId
        ? '<span class="yyq-cap-cmdwrap" style="display:inline-flex;align-items:center;gap:5px;min-width:0">指令' +
          '<input class="yyq-cap-cmd" type="text" maxlength="24" value="' + yyQQE(yyQQEffectiveCmd(kind, c.command)) + '"' +
          ' placeholder="' + yyQQE(c.command) + '" aria-label="' + yyQQE((c.label || kind) + " 指令格式") + '"' +
          ' style="height:24px;width:118px;max-width:44vw;box-sizing:border-box;border:1px solid var(--phx-border-light);background:var(--phx-bg-primary);color:var(--phx-text-primary)"' +
          ' onchange="yyQQCapCmdSave(this)" onkeydown="if(event.key===\'Enter\'){event.preventDefault();this.blur();}"></span>'
        : (c.command ? '<code class="yyq-cap-cmd">' + yyQQE(c.command) + "</code>" : "")) +
      '<span class="yyq-cap-state">' + (unsupported ? "当前适配器不可用" : on ? "运行中" : "已停用") + "</span>" +
    "</span>" +
    '<div class="yyq-cap-edit" onclick="event.stopPropagation()">' +
      '<label class="yyq-cap-f"><span>最低角色</span><select data-f="min_level">' +
        '<option value="0"' + (c.min_level === 0 ? " selected" : "") + ">成员</option>" +
        '<option value="1"' + (c.min_level === 1 ? " selected" : "") + ">管理员</option>" +
        '<option value="2"' + (c.min_level === 2 ? " selected" : "") + ">超级管理员</option>" +
      "</select></label>" +
      '<label class="yyq-cap-f"><span>冷却（秒）</span><input type="number" min="0" max="86400" step="1" data-f="cool_seconds" value="' +
        (c.cool_seconds || 0) + '"></label>' +
      '<label class="yyq-cap-f"><span>同类间隔（秒）</span><input type="number" min="0" max="3600" step="1" data-f="gap_seconds" value="' +
        (c.gap_seconds || 0) + '"></label>' +
      '<label class="yyq-cap-f"><span>每日次数（0=不限）</span><input type="number" min="0" max="10000" step="1" data-f="daily" value="' +
        (c.daily || 0) + '"></label>' +
      '<div class="yyq-cap-edit-act">' +
        '<button type="button" class="btn-s" onclick="yyQQCapSave(this)">保存参数</button>' +
        '<button type="button" class="btn-s" onclick="yyQQCapReset(this)">清空为待定</button>' +
        '<button type="button" class="btn-s" onclick="yyQQOpenTriggers()">触发词与动作</button>' +
        '<span class="yyq-cap-edit-msg"></span>' +
      "</div>" +
      '<p class="yyq-cap-edit-note">这几项现在是<b>待定</b>：框里是兜底值，不是你定的值。改成你要的数字再点保存，就按你的来（填 0 = 不限）。点「清空为待定」可以退回兜底。</p>' +
    "</div></li>";
}

function yyQQCapMsg(li, text, bad) {
  var el = li && li.querySelector ? li.querySelector(".yyq-cap-edit-msg") : null;
  if (!el) return;
  el.textContent = text || "";
  el.setAttribute("data-bad", bad ? "1" : "0");
}

// 保存玩家改过的能力参数（冷却/间隔/每日/最低角色）。
async function yyQQCapSave(btn) {
  var li = btn.closest ? btn.closest(".yyq-cap") : null;
  var kind = li && li.getAttribute("data-kind");
  if (!kind) return;
  var box = li.querySelector(".yyq-cap-edit");
  function num(f, max) {
    var el = box ? box.querySelector('[data-f="' + f + '"]') : null;
    var v = parseInt(el && el.value, 10);
    if (isNaN(v) || v < 0) v = 0;
    if (v > max) v = max;
    return v;
  }
  var patch = {};
  patch[kind] = {
    min_level: num("min_level", 2),
    cool_seconds: num("cool_seconds", 86400),
    gap_seconds: num("gap_seconds", 3600),
    daily: num("daily", 10000)
  };
  btn.disabled = true;
  yyQQCapMsg(li, "保存中…");
  var r = await A("POST", "/api/qq/caps", { limits: patch });
  btn.disabled = false;
  if (!r || !r.ok) {
    yyQQCapMsg(li, (r && r.error) || "保存失败", 1);
    return;
  }
  if (r.caps) yyQQC.caps = r.caps;
  yyQQC.openCap = kind;
  yyQQRender("caps");
}

// 恢复这一项的参数到程序默认（后端把覆盖删掉）。
async function yyQQCapReset(btn) {
  var li = btn.closest ? btn.closest(".yyq-cap") : null;
  var kind = li && li.getAttribute("data-kind");
  if (!kind) return;
  var patch = {};
  patch[kind] = null;
  btn.disabled = true;
  yyQQCapMsg(li, "清空中…");
  var r = await A("POST", "/api/qq/caps", { limits: patch });
  btn.disabled = false;
  if (!r || !r.ok) {
    yyQQCapMsg(li, (r && r.error) || "恢复失败", 1);
    return;
  }
  if (r.caps) yyQQC.caps = r.caps;
  yyQQC.openCap = kind;
  yyQQRender("caps");
}

// 保存玩家改过的指令格式（assistant.command_aliases，PATCH 深合并）。
// 留空 = 恢复内置默认（合并后落空串，yyQQCommandText 按空值回默认，等价删 key）。
async function yyQQCapCmdSave(input) {
  var li = input.closest ? input.closest(".yyq-cap") : null;
  var kind = li && li.getAttribute("data-kind");
  var id = kind && yyQQCapCmdId[kind];
  if (!id) return;
  var val = String(input.value || "").trim().slice(0, 24);
  input.disabled = true;
  var alias = {};
  alias[id] = val ? val : null;
  var r = await A("POST", "/api/qq/config", { assistant: { command_aliases: alias } });
  input.disabled = false;
  if (!r || !r.ok) {
    T((r && r.error) || "保存指令失败", "e");
    return;
  }
  if (r.config) yyQQC.cfg = r.config;
  // 局部重绘：生效指令、帮助文本（面板侧）立刻换成新值
  yyQQRender("caps");
  T(val ? "指令已改为 " + val : "指令已恢复默认", "o");
}

// 触发词规则在「机器人能力」模块里，从能力卡一键跳过去并展开。
function yyQQOpenTriggers() {
  yyQQC.modOpen.robot = true;
  yyQQOpenModule("robot", "yyq-triggers");
  var inp = document.getElementById("yyq-tf-name");
  if (inp && inp.focus) setTimeout(function () { try { inp.focus(); } catch (e) {} }, 260);
}

function yyQQCapDetail(ev, li) {
  // .yyq-cap-more 里现在有指令输入框，点它不能把行收起来。
  if (ev && ev.target && ev.target.closest && (ev.target.closest(".yyq-sw") || ev.target.closest(".yyq-cap-edit") || ev.target.closest(".yyq-cap-more"))) return;
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

// ---------- 3 实时流（单条时间线：别人靠左、自己靠右、动作做小字） ----------

// 面板里以机器人身份发出的消息不会进 /api/qq/messages（那个桶只收「收到的事件」），
// 所以本地留一份回声，按群存最近 30 条，刷新页面还在。
function yyQQSentLoad() {
  try {
    var raw = localStorage.getItem("yyqq.sent");
    var map = raw ? JSON.parse(raw) : null;
    yyQQC.sent = map && typeof map === "object" ? map : {};
  } catch (e) {
    yyQQC.sent = {};
  }
}
function yyQQSentSave() {
  try { localStorage.setItem("yyqq.sent", JSON.stringify(yyQQC.sent || {})); } catch (e) {}
}
function yyQQSentPush(group, text, ts) {
  if (!group || !text) return;
  var list = (yyQQC.sent[group] || []).slice(-29);
  list.push({ text: text, ts: ts || Date.now() });
  yyQQC.sent[group] = list;
  yyQQSentSave();
}

function yyQQTimeline() {
  var rows = [];
  var group = yyQQC.picked || "";
  (yyQQC.messages || []).forEach(function (m) {
    rows.push({
      t: Date.parse(m.timestamp) || 0,
      kind: m.is_bot ? "out" : "in",
      icon: m.is_bot ? "send" : "chat",
      who: m.name || m.sender_id || "群友",
      text: m.text || "",
      mine: !!m.is_bot
    });
  });
  (yyQQC.sent[group] || []).forEach(function (s) {
    // 服务端将来若也开始记发出消息，别和本地回声重复两遍。
    var dup = rows.some(function (r) {
      return r.mine && r.text === s.text && Math.abs(r.t - s.ts) < 10000;
    });
    if (dup) return;
    rows.push({ t: s.ts, kind: "out", icon: "send", who: "机器人", text: s.text, mine: true });
  });
  (yyQQC.actions || []).forEach(function (a) {
    rows.push({
      t: Date.parse(a.time) || 0, kind: a.ok ? "act" : "fail",
      icon: a.ok ? yyQQCapIcon(a.kind) : "ban",
      who: yyQQCapLabel(a.kind) + (a.target ? " · " + a.target : ""),
      text: a.detail || a.error || ""
    });
  });
  // 时间线永远「老的在上、新的在下」，跟群聊一致；新消息靠 yyQQScrollRestore 贴底。
  // 只留最近 60 行：面板是看当下的，不负责翻历史。
  rows.sort(function (x, y) { return x.t - y.t; });
  return rows.slice(-60);
}

function yyQQTlRow(r) {
  var clock = yyQQClock(r.t);
  if (r.kind === "act" || r.kind === "fail") {
    return '<li class="yyq-sys" data-bad="' + (r.kind === "fail" ? "1" : "0") + '">' +
      yyQQIcon(r.icon) + "<b>" + yyQQE(r.who) + "</b>" +
      (r.text ? "<small>" + yyQQE(r.text) + "</small>" : "") +
      "<time>" + yyQQE(clock) + "</time></li>";
  }
  return '<li class="yyq-msg" data-mine="' + (r.mine ? "1" : "0") + '">' +
    '<div class="yyq-msg-meta">' +
      (r.mine ? '<em class="yyq-msg-tag">机器人</em>' : "") +
      "<b>" + yyQQE(r.who) + "</b><time>" + yyQQE(clock) + "</time></div>" +
    '<div class="yyq-msg-b">' + yyQQE(r.text || "（无内容）") + "</div></li>";
}

function yyQQStream() {
  var rows = yyQQTimeline();
  var hasGroup = yyQQEnabledGroups().length > 0;
  var head = '<div class="yyq-stream-head">' + yyQQGroupTabs() +
    '<span class="yyq-live" data-on="' + (yyQQView().key === "open" ? "1" : "0") + '"><i></i>实时</span>' +
    "</div>";

  var body;
  if (!hasGroup) {
    body = yyQQEmpty("connection", "还没选互通群", "在下面「接入与互通群」里勾一个群", "去接入", "yyQQOpenAccess()");
  } else if (yyQQC.loading.stream) {
    body = '<div class="yyq-tl is-loading">' + yyQQSkeleton(4, 44) + "</div>";
  } else if (yyQQC.msgErr) {
    body = '<p class="yyq-inline-err">' + yyQQIcon("ban") + "<span>" + yyQQE(yyQQC.msgErr) + "</span>" +
      '<button type="button" onclick="yyQQRefreshStream()">重试</button></p>';
  } else if (!rows.length) {
    body = yyQQEmpty("chat", "这个群还没有消息", "在群里 @ 它一句，这里会立刻出现", "刷新", "yyQQRefreshStream()");
  } else {
    body = '<ol class="yyq-tl">' + rows.map(yyQQTlRow).join("") + "</ol>";
  }

  return '<aside class="yyq-stream" id="yyq-stream" aria-label="消息时间线">' + head + body +
    '<div class="yyq-stream-foot"><button type="button" class="yyq-ghost" onclick="yyQQRefreshStream()">' + yyQQIcon("refresh") + "刷新</button></div>" +
    yyQQTerm() + "</aside>";
}

// 以机器人身份说话的终端：直接长在消息流底下，和"收到/发出"的消息同屏，
// 用户要的就是把它当终端用，不再单独开一个折叠块。
function yyQQTerm() {
  var g = yyQQC.picked || "";
  var label = g ? (yyQQPickedName() || g) : "";
  var msg = yyQQC.termMsg || "";
  return '<form class="yyq-term" onsubmit="yyQQTermSend(event)">' +
    '<div class="yyq-term-head"><span>以机器人身份发送</span><small>' +
      (label ? "发到 " + yyQQE(label) : "先在接入设置里选一个群") + "</small></div>" +
    '<div class="yyq-term-row">' +
      '<input id="yyq-term-input" autocomplete="off" placeholder="' +
        (g ? "输入内容，回车发送" : "没有可用群") + '"' + (g ? "" : " disabled") + ">" +
      '<button type="submit" class="yyq-term-btn"' + (g ? "" : " disabled") + ">发送</button>" +
    "</div>" +
    (msg ? '<p class="yyq-term-msg" data-bad="' + (yyQQC.termBad ? "1" : "0") + '">' + yyQQE(msg) + "</p>" : "") +
    "</form>";
}

async function yyQQTermSend(ev) {
  if (ev && ev.preventDefault) ev.preventDefault();
  var input = document.getElementById("yyq-term-input");
  var text = ((input && input.value) || "").trim();
  if (!text) {
    yyQQC.termMsg = "先写点什么再发";
    yyQQC.termBad = 1;
    yyQQRender("stream");
    return false;
  }
  var group = yyQQC.picked || "";
  yyQQC.termMsg = "发送中…";
  yyQQC.termBad = 0;
  yyQQRender("stream");
  var r = await A("POST", "/api/qq/send", { text: text, group: group });
  if (!r || !r.ok) {
    yyQQC.termMsg = (r && r.error) || "发送失败";
    yyQQC.termBad = 1;
    yyQQRender("stream");
    return false;
  }
  yyQQC.termMsg = "已发送";
  yyQQC.termBad = 0;
  yyQQSentPush(group || yyQQC.picked, text, Date.now());
  yyQQC.stickBottom = 1;
  if (input) input.value = "";
  yyQQRender("stream");
  // 发完立刻刷新时间线，让刚发的话出现在上面
  yyQQRefreshStream();
  if (typeof T === "function") T("已发送到群", "s");
  return false;
}

// 切群：先用该群的缓存立刻上屏（点一下就有东西），再后台拉最新的。
function yyQQPickTab(id) {
  if (!id || id === yyQQC.picked) return;
  yyQQC.picked = id;
  var cached = yyQQC.msgCache[id];
  yyQQC.messages = cached ? cached.rows : [];
  yyQQC.msgErr = "";
  yyQQC.loading.stream = cached ? 0 : 1;
  yyQQC.stickBottom = 1;
  yyQQC.unread[id] = 0;
  yyQQMarkRead(id);
  yyQQRender("stream");
  yyQQLoadMessages().then(function () { yyQQRender("stream"); });
}

// ---------- 4 接入与群 ----------

function yyQQAccess() {
  var cfg = yyQQC.cfg || {}, ob = cfg.onebot || {}, q = cfg.qqofficial || {}, as = cfg.assistant || {};
  var official = yyQQC.adapter === "qqofficial";
  var sel = yyQQC.sel || [];

  var groupsHtml;
  if (yyQQC.groupsLoading) {
    groupsHtml = '<div class="yyq-group-list">' + yyQQSkeleton(2, 56) + "</div>";
  } else if (yyQQC.groupsErr) {
    groupsHtml = '<p class="yyq-inline-err">' + yyQQIcon("ban") + "<span>" + yyQQE(yyQQC.groupsErr) + "</span>" +
      '<button type="button" onclick="yyQQLoadGroups()">重试</button></p>';
  } else if (!yyQQC.groupsLoaded) {
    groupsHtml = "";
  } else if (!yyQQC.groups.length) {
    groupsHtml = yyQQEmpty("list", "机器人还没加入任何群", "先在手机 QQ 里把它拉进一个群", "重新读取", "yyQQLoadGroups()");
  } else {
    // 多选：点一下勾上/取消，可以同时绑多个群，顶部就是这些群的标签。
    groupsHtml = '<div class="yyq-group-list">' + yyQQC.groups.map(function (g) {
      var id = g.group_id || g.id;
      var on = sel.indexOf(id) >= 0;
      return '<button type="button" class="yyq-group-card" data-id="' + yyQQE(id) + '" data-on="' + (on ? "1" : "0") +
        '" aria-pressed="' + (on ? "true" : "false") + '" onclick="yyQQToggleGroup(\'' + yyQQE(id) + '\')">' +
        '<span class="yyq-group-name">' + yyQQE(g.group_name || g.name || id) + "</span>" +
        "<small>" + yyQQE(id) + (g.member_count ? " · " + g.member_count + " 人" : "") + "</small>" +
        '<span class="yyq-group-check">' + yyQQIcon("check") + "</span></button>";
    }).join("") + "</div>";
  }

  // 只返回正文，外层 <details> 由「聊天与账号」模块提供。
  // #yyq-access 这个 id 保留：yyQQSetAdapter / yyQQToggleGroup 靠它做局部更新。
  return '<div id="yyq-access" class="yyq-access-body">' +
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
        '<label class="yyq-field yyq-field-manual"><span>群号</span><input id="yyq-group" value="" placeholder="1121432132"></label>' +
      "</div>" +
      // 这个 id 以前叫 yyq-relay，和「群→游戏转发」区块的容器撞名，
      // document.getElementById 永远先拿到上面那个复选框，转发区块的局部重绘就写错节点。
      '<label class="yyq-switchrow"><span class="yyq-switchrow-text"><strong>把群聊消息转发进游戏</strong></span>' +
        '<span class="yyq-sw"><input type="checkbox"' + (as.relay_chat ? " checked" : "") + ' id="yyq-relay-chat" aria-label="转发群聊进游戏"><span></span></span></label>' +
      '<div class="yyq-fold-actions"><button type="button" class="btn" data-busy="' + (yyQQC.busy.access ? "1" : "0") + '" onclick="yyQQSaveAccess(this)">保存并重连</button>' +
        '<button type="button" class="btn-s" onclick="yyQQRefreshAll(1,this)">刷新状态</button></div>' +
    "</div>";
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
  var sum = document.querySelector("#yyq-mod-chat .yyq-fold-sum");
  if (sum) sum.textContent = (v === "qqofficial" ? "QQ 官方" : "NapCat") + ((yyQQC.sel || []).length ? " · " + yyQQC.sel.length + " 个群" : " · 未选群");
}
// 多选：只改勾选态与折叠块摘要，不整块重绘，已经填好的地址不会被冲掉。
function yyQQToggleGroup(id) {
  var sel = yyQQC.sel || (yyQQC.sel = []);
  var at = sel.indexOf(id);
  if (at >= 0) sel.splice(at, 1);
  else sel.push(id);
  yyQQC.selTouched = true;
  var cards = document.querySelectorAll("#yyq-access .yyq-group-card");
  for (var i = 0; i < cards.length; i++) {
    if (cards[i].getAttribute("data-id") !== id) continue;
    var on = at < 0;
    cards[i].setAttribute("data-on", on ? "1" : "0");
    cards[i].setAttribute("aria-pressed", on ? "true" : "false");
  }
  var sum = document.querySelector("#yyq-mod-chat .yyq-fold-sum");
  if (sum) sum.textContent = (yyQQC.adapter === "qqofficial" ? "QQ 官方" : "NapCat") + (sel.length ? " · " + sel.length + " 个群" : " · 未选群");
}

async function yyQQLoadGroups() {
  yyQQC.groupsLoading = true;
  yyQQC.groupsErr = "";
  yyQQRender("mod-chat");
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
  yyQQRender("mod-chat");
  var el = document.getElementById("yyq-access");
  if (el) el.setAttribute("open", "");
  yyQQC.accessOpen = true;
}

async function yyQQSaveAccess(btn) {
  var v = function (id) { var el = document.getElementById(id); return el ? el.value : ""; };
  var manual = v("yyq-group").trim();
  var ids = (yyQQC.sel || []).slice();
  if (manual && ids.indexOf(manual) < 0) ids.push(manual);
  // 之前配过但不在这轮勾选里的群要保留成 enabled:false，不能直接从配置里删掉，
  // 否则用户只是取消勾一个群，另一个群的备注/名字也跟着丢了。
  var seen = {};
  var groups = [];
  yyQQGroupsOf(yyQQC.cfg).forEach(function (g) {
    var id = g.id || g.group_id;
    if (!id || seen[id]) return;
    seen[id] = 1;
    groups.push({ id: id, name: g.name || "", enabled: ids.indexOf(id) >= 0 });
  });
  ids.forEach(function (id) {
    if (seen[id]) return;
    seen[id] = 1;
    groups.push({ id: id, name: "", enabled: true });
  });
  var body = {
    enabled: true,
    adapter: yyQQC.adapter,
    groups: groups,
    onebot: { ws_url: v("yyq-ws").trim() },
    qqofficial: { app_id: v("yyq-appid").trim() },
    assistant: { relay_chat: !!(document.getElementById("yyq-relay-chat") || {}).checked }
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
  // 当前标签若被取消勾选，退回第一个还勾着的群。
  if (ids.indexOf(yyQQC.picked) < 0) yyQQC.picked = ids[0] || "";
  yyQQC.selTouched = false;
  T("已保存，正在重新接入", "o");
  await yyQQRefreshAll(1);
  if (btn) { btn.setAttribute("data-busy", "0"); btn.textContent = "保存并重连"; }
}

// ---------- 5 人设与试跑 ----------
//
// 首页「AI 助手」拆成 4 段：提示词 / 行为 / 知识库 / AI 工具。
// 模型以前是下拉框，和「AI 模型配置」页各存一份、谁后存谁覆盖；
// 现在只留一行只读文字，要改就去设置页改（唯一真源）。

function yyQQPersonaPrompt() {
  var prompt = yyQQPromptValue();
  return yyQQSec(
    yyQQSecHead("提示词", "AI 回答前会先读这段") +
    '<div class="yyq-prompt-bar">' +
      '<div class="yyq-var-chips">' + ["{玩家}", "{群名}", "{时间}"].map(function (v) {
        return '<button type="button" onclick="yyQQInsertVar(\'' + v + '\')">' + v + "</button>";
      }).join("") + "</div>" +
      '<span class="yyq-count" id="yyq-prompt-count">' + prompt.length + " / 2000</span>" +
    "</div>" +
    '<textarea id="yyq-prompt" class="yyq-textarea" rows="6" oninput="yyQQPromptInput(this)" placeholder="例如：你是布汲岛的 QQ 助手，说话简短、先给结论…">' + yyQQE(prompt) + "</textarea>" +
    '<div class="yyq-fold-actions"><button type="button" class="btn" onclick="yyQQSavePersona(this)">保存人设</button></div>',
    "yyq-sec-wide");
}

function yyQQPersonaModel() {
  var as = (yyQQC.cfg || {}).assistant || {};
  return yyQQSec(
    '<p class="yyq-model-line">当前语言模型：<b>' + yyQQE(as.model || "默认模型") + "</b>" +
    '<button type="button" class="yyq-linkbtn" onclick="aiOpenManage()">去设置里改</button></p>'
  );
}

function yyQQPersonaBehavior() {
  return yyQQSec(
    yyQQSecHead("行为", "接话 / 主动搭话 / 看图 / 语音") +
    yyQQCompanion() +
    yyQQProactive() +
    yyQQVisionPanel() +
    yyQQVoicePanel()
  );
}

function yyQQPersonaSandbox() {
  return yyQQSec(
    yyQQSecHead("试跑与自检", "本地预览 · 不会发到群里") +
    yyQQAISelf() +
    '<div class="yyq-sandbox">' +
      '<div class="yyq-sandbox-log" id="yyq-sandbox-log">' + yyQQSandboxLog() + "</div>" +
      '<div class="yyq-sandbox-input"><input id="yyq-try" placeholder="写一句话，看看人设会怎么套用" onkeydown="if(event.key===\'Enter\')yyQQAsk()">' +
        '<button type="button" class="btn" onclick="yyQQAsk()">试跑</button>' +
        '<button type="button" class="btn-s" onclick="yyQQClearSandbox()">清空沙盒</button></div>' +
    "</div>"
  );
}

function yyQQPersona() {
  return yyQQPersonaPrompt() + yyQQPersonaModel() + yyQQPersonaBehavior() + yyQQPersonaSandbox();
}

// AI 主动接话（后端叫 companion）。
// 用户原话：「QQ机器人里面的AI能够自主发消息，现在都还没条件，跟他没存在一样，
// 而且各种设置也一样」——后端四个字段（companion / companion_owner /
// companion_keywords / companion_all_messages）一直存在，但面板从来没有入口，
// 所以这个能力等于不存在。这里把它摆出来，并且默认是「关」，
// 由用户自己决定开不开、什么条件下接话。
function yyQQCompanion() {
  var as = (yyQQC.cfg || {}).assistant || {};
  var on = as.companion === true;
  var all = as.companion_all_messages === true;
  var kws = (as.companion_keywords || []).join("、");
  var owner = as.companion_owner || "";
  var win = as.companion_window;
  var mode = all ? "all" : (kws ? "keyword" : "mention");
  return '<details class="yyq-cmp yyq-subfold" data-on="' + (on ? "1" : "0") + '">' +
    '<summary><b>AI 主动接话</b><small>群里聊天时它自己接，不用每次 @</small></summary>' +
    '<label class="yyq-switch"><input type="checkbox" id="yyq-cmp-on"' + (on ? " checked" : "") +
      ' onchange="yyQQCmpToggle(this)"><span>开启主动接话</span></label>' +
    '<div class="yyq-cmp-body"' + (on ? "" : ' hidden') + ">" +
      '<div class="yyq-fields">' +
        '<label class="yyq-field"><span>什么情况下接话</span>' +
          '<select id="yyq-cmp-mode">' +
            '<option value="mention"' + (mode === "mention" ? " selected" : "") + ">只在被 @ 或点名叫到时接</option>" +
            '<option value="keyword"' + (mode === "keyword" ? " selected" : "") + ">命中触发词就接</option>" +
            '<option value="all"' + (mode === "all" ? " selected" : "") + ">群里每条消息都接（容易刷屏）</option>" +
          "</select></label>" +
        '<label class="yyq-field"><span>触发词（用、或逗号分隔）</span>' +
          '<input id="yyq-cmp-kw" value="' + yyQQE(kws) + '" placeholder="例如：机器人在吗、求助、怎么弄"></label>' +
        '<label class="yyq-field"><span>主人 QQ（主人说话时重置当前话题）</span>' +
          '<input id="yyq-cmp-owner" value="' + yyQQE(owner) + '" placeholder="选填"></label>' +
        '<label class="yyq-field"><span>接话窗口（秒，留空 = 待定）</span>' +
          '<input id="yyq-cmp-win" type="number" min="5" max="600" value="' + (win == null ? "" : win) + '" placeholder="待定"></label>' +
      "</div>" +
      '<p class="yyq-cmp-note">开启后：群里有符合上面条件的消息，它会自己接着聊，' +
        '窗口内每条新消息都会续期；不会在没有消息的时候自言自语。</p>' +
      '<div class="yyq-fold-actions"><button type="button" class="btn" onclick="yyQQSaveCompanion(this)">保存接话设置</button></div>' +
    "</div></details>";
}

// AI 主动搭话（后端 /api/qq/proactive）。
// 和「主动接话」不是一回事：接话是有人说话时它跟着接，主动搭话是**没人说话时它自己开话题**。
// 后端 youyuan_qq_proactive.go 早就写完了打分/冷却/日限，但一直是死代码（全仓零调用），
// 所以这个能力等于不存在。现在接上线了，这里给入口，默认关闭——它一开口就是往群里发消息，
// 不能让用户升级完就被机器人搭话。
var yyQQProCache = null, yyQQProBusy = false;

function yyQQProNum(id, label, val, step) {
  return '<label class="yyq-field"><span>' + label + '</span><input id="' + id + '" type="number" min="0" step="' + (step || "1") + '" value="' +
    (val == null ? "" : val) + '"></label>';
}

function yyQQProactive() {
  var c = yyQQProCache;
  var on = !!(c && c.enabled);
  if (!c && !yyQQProBusy) { yyQQProBusy = true; setTimeout(yyQQProLoad, 0); }
  var body = c
    ? '<div class="yyq-fields">' +
        yyQQProNum("yyq-pro-silence", "多久没人说话才算冷场（秒）", c.min_silence_sec) +
        yyQQProNum("yyq-pro-cd", "两次主动开口最小间隔（秒）", c.cooldown_sec) +
        yyQQProNum("yyq-pro-cap", "每个群每天最多主动说几次", c.daily_cap) +
        yyQQProNum("yyq-pro-th", "开口阈值（越低越爱说话）", c.threshold, "0.1") +
      "</div>" +
      '<p class="yyq-cmp-note">没人说话、话题够热、又不吵人的时候它才会开口；' +
        "一天说满次数就自动闭嘴，不会刷屏。</p>"
    : '<p class="yyq-cmp-note">正在读取主动搭话设置…</p>';
  return '<details class="yyq-cmp yyq-pro yyq-subfold" data-on="' + (on ? "1" : "0") + '">' +
    '<summary><b>AI 主动搭话</b><small>没人说话时它自己起话题</small></summary>' +
    '<label class="yyq-switch"><input type="checkbox" id="yyq-pro-on"' + (on ? " checked" : "") +
      ' onchange="yyQQCmpToggle(this)"><span>开启主动搭话</span></label>' +
    '<div class="yyq-cmp-body"' + (on ? "" : ' hidden') + ">" + body +
      '<div class="yyq-fold-actions"><button type="button" class="btn" onclick="yyQQSaveProactive(this)">保存主动搭话设置</button></div>' +
    "</div></details>";
}

async function yyQQProLoad() {
  try {
    var r = await A("GET", "/api/qq/proactive");
    if (r && r.ok && r.config) {
      yyQQProCache = r.config;
      var box = document.querySelector(".yyq-pro");
      // 只在「从没数据变成有数据」时替换一次，避免读一次重绘一次。
      if (box && box.getAttribute("data-loaded") !== "1") {
        var wrap = document.createElement("div");
        wrap.innerHTML = yyQQProactive();
        var fresh = wrap.firstChild;
        if (fresh) { fresh.setAttribute("data-loaded", "1"); box.parentNode.replaceChild(fresh, box); }
      }
    }
  } catch (e) {
    yyQQProBusy = false;
  }
}

async function yyQQSaveProactive(btn) {
  var num = function (id, dflt) {
    var el = document.getElementById(id);
    var v = el ? Number(el.value) : NaN;
    return Number.isFinite(v) && v >= 0 ? v : dflt;
  };
  var cfg = {
    enabled: !!(document.getElementById("yyq-pro-on") || {}).checked,
    min_silence_sec: Math.round(num("yyq-pro-silence", 900)),
    cooldown_sec: Math.round(num("yyq-pro-cd", 1800)),
    daily_cap: Math.round(num("yyq-pro-cap", 8)),
    threshold: num("yyq-pro-th", 1.2),
    weights: (yyQQProCache && yyQQProCache.weights) || { silence: 1, topic: 0.6, activity: 0.8, night: 1.5 }
  };
  if (btn) btn.disabled = true;
  try {
    var r = await A("POST", "/api/qq/proactive", { config: cfg });
    if (r && r.ok) {
      yyQQProCache = r.config || cfg;
      T(cfg.enabled ? "主动搭话已开启" : "主动搭话已关闭", "o");
      if (r.persistence_error) T("设置已生效，但落盘失败：" + r.persistence_error, "w");
    } else {
      T((r && r.error) || "保存失败", "e");
    }
  } finally {
    if (btn) btn.disabled = false;
  }
}

function yyQQCmpToggle(el) {
  var box = el && el.closest ? el.closest(".yyq-cmp") : null;
  if (!box) return;
  box.setAttribute("data-on", el.checked ? "1" : "0");
  var body = box.querySelector(".yyq-cmp-body");
  if (body) { if (el.checked) body.removeAttribute("hidden"); else body.setAttribute("hidden", ""); }
}

async function yyQQSaveCompanion(btn) {
  var on = !!(document.getElementById("yyq-cmp-on") || {}).checked;
  var mode = (document.getElementById("yyq-cmp-mode") || {}).value || "mention";
  var raw = (document.getElementById("yyq-cmp-kw") || {}).value || "";
  var owner = ((document.getElementById("yyq-cmp-owner") || {}).value || "").trim();
  var winRaw = ((document.getElementById("yyq-cmp-win") || {}).value || "").trim();
  // 触发词按「、，, 空白」都拆，用户不会记得该用哪个分隔符。
  var kws = raw.split(/[、,，\s]+/).map(function (s) { return s.trim(); }).filter(Boolean).slice(0, 20);
  if (mode === "keyword" && !kws.length) { T("选了「命中触发词」，至少要填一个触发词", "w"); return; }
  var patch = {
    companion: on,
    companion_all_messages: mode === "all",
    companion_keywords: mode === "keyword" ? kws : [],
    companion_owner: owner
  };
  if (winRaw) {
    var n = parseInt(winRaw, 10);
    if (!(n >= 5 && n <= 600)) { T("接话窗口请填 5~600 秒，或留空表示待定", "w"); return; }
    patch.companion_window = n;
  } else {
    // 留空 = 待定，显式写 0，后端按默认值走。
    patch.companion_window = 0;
  }
  if (btn) { btn.setAttribute("data-busy", "1"); btn.textContent = "保存中"; }
  var r = await A("POST", "/api/qq/config", { assistant: patch });
  if (btn) { btn.setAttribute("data-busy", "0"); btn.textContent = "保存接话设置"; }
  if (!r || !r.ok) { T((r && r.error) || "保存失败", "e"); return; }
  yyQQC.cfg.assistant = yyQQC.cfg.assistant || {};
  for (var k in patch) yyQQC.cfg.assistant[k] = patch[k];
  if (btn) { btn.textContent = "✓ 已保存"; setTimeout(function () { if (btn) btn.textContent = "保存接话设置"; }, 1400); }
  T(on ? "已开启主动接话" : "已关闭主动接话", "o");
}

// AI 多模态识图开关（后端 assistant.vision 三态）。
// 起因：用户反馈「这个模型是多模态的，但是他却看不到表情包和照片」。
// 根因是旧逻辑靠模型名里的 vision/vl/4o 等字样猜，DeepSeek 这类名字看不
// 出来的多模态模型被漏判，群里的图片一张都挂不上。现在三态：
//   auto = 按模型名自动检测（默认，旧行为）
//   on   = 强制开启（模型名检测不到但确实支持视觉时选这个）
//   off  = 强制关闭
function yyQQVisionPanel() {
  var as = (yyQQC.cfg && yyQQC.cfg.assistant) || {};
  var mode = as.vision === true ? "on" : as.vision === false ? "off" : "auto";
  var s = yyQQC.aiSelf || null;
  var modelName = (s && s.model_name) || as.model || "未选择";
  return '<details class="yyq-cmp yyq-vision yyq-subfold">' +
    '<summary><b>多模态识图</b><small>让 AI 看到群里的图片和表情包</small></summary>' +
    '<div class="yyq-fields">' +
      '<label class="yyq-field"><span>识图开关</span><select id="yyq-vision-mode">' +
        '<option value="auto"' + (mode === "auto" ? " selected" : "") + '>自动检测（按模型名判断）</option>' +
        '<option value="on"' + (mode === "on" ? " selected" : "") + '>强制开启</option>' +
        '<option value="off"' + (mode === "off" ? " selected" : "") + '>强制关闭</option>' +
      "</select></label>" +
    "</div>" +
    '<p class="yyq-cmp-note">当前模型：' + yyQQE(modelName) + "。" +
      "自动检测只认模型名里的 vision / vl / 4o 等字样；" +
      "模型明明支持视觉但名字看不出来时，选「强制开启」。" +
      "群里的图片会直接挂到 AI 那一轮对话上（最多 3 张），表情包会自动换成可访问地址。</p>" +
    '<div class="yyq-fold-actions"><button type="button" class="btn" onclick="yyQQSaveVision(this)">保存识图设置</button></div>' +
    "</details>";
}

async function yyQQSaveVision(btn) {
  var el = document.getElementById("yyq-vision-mode");
  var mode = el ? el.value : "auto";
  var patch = {};
  if (mode === "on") patch.vision = true;
  else if (mode === "off") patch.vision = false;
  // auto 不带 vision 字段：后端 *bool + omitempty，缺省即自动检测。
  if (btn) { btn.setAttribute("data-busy", "1"); btn.textContent = "保存中"; }
  var r;
  try {
    r = await A("POST", "/api/qq/config", { assistant: patch });
  } finally {
    if (btn) { btn.setAttribute("data-busy", "0"); btn.textContent = "保存识图设置"; }
  }
  if (!r || !r.ok) { T((r && r.error) || "保存失败", "e"); return; }
  yyQQC.cfg = yyQQC.cfg || {};
  yyQQC.cfg.assistant = yyQQC.cfg.assistant || {};
  if (mode === "auto") delete yyQQC.cfg.assistant.vision;
  else yyQQC.cfg.assistant.vision = (mode === "on");
  yyQQC.aiSelf = null; // 自检面板重读，vision_mode 会更新
  if (btn) { btn.textContent = "已保存"; setTimeout(function () { if (btn) btn.textContent = "保存识图设置"; }, 1400); }
  T(mode === "off" ? "已关闭识图" : mode === "on" ? "已强制开启识图" : "识图保持自动检测", "o");
}

// AI 能力自检：把「AI 现在能用什么」摊开给用户看。
// 起因是用户说「AI 能自主发消息，现在都还没条件，跟他没存在一样」——
// 他没法验证我们说的「做好了」，所以做一个可查的清单：
// 模型是谁、支不支持看图、主动接话开没开。工具明细不再在这里堆英文名，
// 统一看「AI 工具」折叠块（中文名 + 分组 + 逐项开关）。
function yyQQAISelf() {
  var s = yyQQC.aiSelf;
  if (!s) {
    if (!yyQQC.aiSelfBusy) {
      yyQQC.aiSelfBusy = 1;
      A("GET", "/api/qq/ai/selfcheck").then(function (r) {
        yyQQC.aiSelfBusy = 0;
        if (r && r.ok) { yyQQC.aiSelf = r; yyQQRender("mod-ai"); }
      });
    }
    return '<div class="yyq-self" data-state="loading">正在读取 AI 能力…</div>';
  }
  var ok = !!s.model_found;
  var vision = !!s.supports_vision;
  var toolTotal = s.tool_total || 0;
  var toolOn = (s.tools || []).length + (s.social_tools || []).length;
  var rows = [
    { k: "模型", v: (s.model_name || "未选择") + (s.model_id ? "（" + s.model_id + "）" : ""), bad: !ok },
    { k: "识图", v: vision
        ? (s.vision_mode === "forced_on" ? "已强制开启" : s.vision_mode === "forced_off" ? "已强制关闭" : "支持（自动检测）")
        : "关闭（自动检测没认出视觉能力，可在下面「多模态识图」里强制开启）", bad: !vision },
    { k: "主动接话", v: s.companion ? "已开启" : "未开启（在下面「AI 主动接话」里开）", bad: !s.companion },
    { k: "绘图模型", v: s.image_model ? "已配置" : "未配置", bad: !s.image_model },
    { k: "AI 工具", v: toolTotal ? (toolOn + " / " + toolTotal + " 项开启") : "未读取", bad: false }
  ];
  return '<details class="yyq-self yyq-subfold" id="yyq-sub-self">' +
    '<summary><b>AI 能力自检</b><small>模型、识图、工具权限，一屏看清</small></summary>' +
    '<dl class="yyq-self-rows">' + rows.map(function (r) {
      return "<div" + (r.bad ? ' data-bad="1"' : "") + "><dt>" + yyQQE(r.k) + "</dt><dd>" + yyQQE(r.v) + "</dd></div>";
    }).join("") + "</dl>" +
    '<div class="yyq-fold-actions"><button type="button" class="btn-s" onclick="yyQQOpenTools()">管理 AI 工具</button></div>' +
    "</details>";
}
// 从任意位置跳到「AI 工具」分区并展开所在模块。
function yyQQOpenTools() {
  yyQQC.modOpen.ai = true;
  yyQQOpenModule("ai");
  if (!yyQQC.toolsLoaded && !yyQQC.toolsLoading) yyQQLoadTools();
}

function yyQQClip(s, n) {  s = String(s || "").replace(/\s+/g, " ").trim();
  return s.length > n ? s.slice(0, n) + "…" : s;
}

// ---------- 提示词：两个面板共用一个字段 ----------
// 提示词存在 assistant.prompt，页面上有两个输入框会写它：
//   #yyq-prompt      「人设与试跑」面板
//   #yy-qq-ai-prompt 「群服互通助手」面板
// 以前两边各自从 /api/qq/config 载入时的快照渲染、保存时整段回传，于是：
//   在 A 面板存了提示词 → B 面板的输入框还是旧（空）值 → 在 B 面板点保存
//   → 空串覆盖掉刚存的内容。这就是「存了，返回一看没了」。
// 现在只认一份草稿 yyQQPromptDraft：
//   null = 用户没动过，保存时不提交 prompt（后端深合并保留原值）；
//   非 null = 用户真改过，两个框同步成同一个值，保存时提交它。
var yyQQPromptDraft = null;
var yyQQModelDirty = 0;

function yyQQPromptEls() {
  var out = [];
  var a = document.getElementById("yyq-prompt");
  var b = document.getElementById("yy-qq-ai-prompt");
  if (a) out.push(a);
  if (b && b !== a) out.push(b);
  return out;
}
// 当前提示词：用户改过就用草稿，否则用 DOM 里已渲染的值（两个框是镜像的）。
function yyQQPromptValue() {
  if (yyQQPromptDraft !== null) return yyQQPromptDraft;
  var els = yyQQPromptEls();
  if (els.length) return String(els[0].value || "");
  var as = (yyQQC.cfg || {}).assistant || {};
  return String(as.prompt || "");
}
function yyQQPromptDirty() { return yyQQPromptDraft !== null; }
function yyQQPromptCountOf(v) {
  var n = document.getElementById("yyq-prompt-count");
  if (n) n.textContent = String(v || "").length + " / 2000";
}
// 用户在任意一个框里打字：记草稿 + 把另一个框同步成同样的值。
function yyQQPromptInput(el) {
  var v = el ? String(el.value || "") : "";
  yyQQPromptDraft = v;
  var els = yyQQPromptEls();
  for (var i = 0; i < els.length; i++) if (els[i] !== el) els[i].value = v;
  yyQQPromptCountOf(v);
}
// 保存成功后回填：清掉草稿，两个框、内存快照、折叠标题一起更新。
function yyQQPromptSync(v) {
  v = String(v == null ? "" : v);
  yyQQPromptDraft = null;
  var els = yyQQPromptEls();
  for (var i = 0; i < els.length; i++) els[i].value = v;
  yyQQC.cfg = yyQQC.cfg || {};
  yyQQC.cfg.assistant = yyQQC.cfg.assistant || {};
  yyQQC.cfg.assistant.prompt = v;
  yyQQPromptCountOf(v);
  var sum = document.querySelector("#yyq-persona .yyq-fold-sum");
  if (sum) sum.textContent = v ? yyQQClip(v, 22) : "未设置提示词";
  // 助手面板自己那份快照也一起更新，别让它下次保存时又拿旧值覆盖。
  if (typeof window.yyQQAssistantPromptSync === "function") window.yyQQAssistantPromptSync(v);
}
// 保存时决定要不要带上 prompt：没编辑过就不提交，交给后端保留原值。
function yyQQPromptPatch(obj) {
  if (yyQQPromptDraft !== null) obj.prompt = yyQQPromptDraft;
  return obj;
}
function yyQQModelPicked() { yyQQModelDirty = 1; }

function yyQQPromptCount(el) { yyQQPromptInput(el); }
function yyQQInsertVar(v) {
  var el = document.getElementById("yyq-prompt");
  if (!el) return;
  var s = el.selectionStart == null ? el.value.length : el.selectionStart;
  var e = el.selectionEnd == null ? el.value.length : el.selectionEnd;
  el.value = el.value.slice(0, s) + v + el.value.slice(e);
  el.focus();
  try { el.setSelectionRange(s + v.length, s + v.length); } catch (_) {}
  yyQQPromptInput(el);
}
function yyQQSandboxLog() {
  if (!yyQQC.sandbox.length) {
    return '<p class="yyq-sandbox-empty">还没有试跑。写一句群友可能会说的话，这里会显示人设套用后的效果。</p>';
  }
  return yyQQC.sandbox.map(function (m) {
    if (m.role === "user") {
      return '<div class="yyq-bubble" data-role="user"><span>群友</span><p>' + yyQQE(m.text) + "</p></div>";
    }
    if (m.role === "bot") {
      return '<div class="yyq-bubble" data-role="assistant"><span>' + yyQQE(m.meta || "机器人") + "</span><p>" + yyQQE(m.text) + "</p></div>";
    }
    if (m.role === "error") {
      return '<div class="yyq-bubble" data-role="error"><span>试跑失败</span><p>' + yyQQE(m.text) + "</p></div>";
    }
    return '<div class="yyq-bubble" data-role="note"><span>' + (m.role === "pending" ? "生成中" : "本地预览") + "</span><pre>" + yyQQE(m.text) + "</pre></div>";
  }).join("");
}
function yyQQClearSandbox() {
  yyQQC.sandbox = [];
  var log = document.getElementById("yyq-sandbox-log");
  if (log) log.innerHTML = yyQQSandboxLog();
}
// 沙盒试跑：真的调一次模型（/api/qq/sandbox），不经过 QQ、不写群记忆。
// 旧版只是把提示词回显成「群友说：…」，看起来像 AI 没回答，实际压根没请求模型。
async function yyQQAsk() {
  var input = document.getElementById("yyq-try");
  var text = ((input && input.value) || "").trim();
  if (!text) { T("先写一句要试的话", "w"); return; }
  var prompt = ((document.getElementById("yyq-prompt") || {}).value || "").trim();
  var model = ((document.getElementById("yyq-model") || {}).value || "");
  var group = yyQQPickedName();
  var now = new Date();
  var resolved = (prompt || "")
    .replace(/\{玩家\}/g, "沙盒测试员")
    .replace(/\{群名\}/g, group || "未绑定群")
    .replace(/\{时间\}/g, now.getFullYear() + "-" + (now.getMonth() + 1) + "-" + now.getDate() + " " + yyQQClock(now.getTime()));
  yyQQC.sandbox.push({ role: "user", text: text });
  yyQQC.sandbox.push({ role: "pending", text: "正在生成…" });
  if (yyQQC.sandbox.length > 10) yyQQC.sandbox = yyQQC.sandbox.slice(-10);
  if (input) input.value = "";
  var log = document.getElementById("yyq-sandbox-log");
  var paint = function () {
    if (!log) log = document.getElementById("yyq-sandbox-log");
    if (!log) return;
    log.innerHTML = yyQQSandboxLog();
    log.scrollTop = log.scrollHeight;
  };
  paint();
  var r = await A("POST", "/api/qq/sandbox", { text: text, prompt: resolved, model: model });
  // 去掉“正在生成”占位
  for (var i = yyQQC.sandbox.length - 1; i >= 0; i--) {
    if (yyQQC.sandbox[i].role === "pending") { yyQQC.sandbox.splice(i, 1); break; }
  }
  if (r && r.ok) {
    yyQQC.sandbox.push({ role: "bot", text: r.reply, meta: (r.model || model || "默认模型") + (r.input_tokens || r.output_tokens ? " · " + (r.input_tokens || 0) + "/" + (r.output_tokens || 0) + " tokens" : "") });
  } else {
    yyQQC.sandbox.push({ role: "error", text: (r && r.error) || "试跑失败" });
  }
  if (yyQQC.sandbox.length > 10) yyQQC.sandbox = yyQQC.sandbox.slice(-10);
  paint();
}

// 模型下拉：列出 /api/ai/models 里已配置的模型；已保存但不在列表里的值也保留。
async function yyQQLoadModels(force) {
  var sel = document.getElementById("yyq-model");
  if (!sel) return;
  var cur = sel.getAttribute("data-cur") || sel.value || "";
  var r = await A("GET", "/api/ai/models");
  var models = (r && r.ok && r.models) || [];
  var html = '<option value="">默认模型（互通当前使用的）</option>';
  models.forEach(function (m) {
    html += '<option value="' + yyQQE(m.name) + '">' + yyQQE(m.name + (m.model_id ? " (" + m.model_id + ")" : "")) + "</option>";
  });
  sel.innerHTML = html;
  if (cur) {
    if (!models.some(function (m) { return m.name === cur; })) {
      var o = document.createElement("option");
      o.value = cur; o.textContent = cur + "（不在模型库）";
      sel.appendChild(o);
    }
    sel.value = cur;
  } else {
    sel.value = "";
  }
  if (force && !models.length) T("还没有配置模型，点「配置模型」添加一个", "w");
}

// AI 语音回复（后端 assistant.voice_model / voice_voice / voice_mode）。
// 用户第 22 条：「别人这个AI聊天模型搭配onebot可以发语音 这个我们肯定也要弄一个能力」。
// 模型下拉复用 /api/ai/models：选一个支持 OpenAI 兼容 /audio/speech 的配置即可；
// 留空 = 不用语音（群里 .语音 音频URL 直发仍然可用，只是不能文字转语音）。
function yyQQVoicePanel() {
  var as = (yyQQC.cfg && yyQQC.cfg.assistant) || {};
  var mode = as.voice_mode;
  if (mode !== "auto" && mode !== "always") mode = "off";
  var model = as.voice_model || "";
  var voice = as.voice_voice || "";
  setTimeout(function () { yyQQLoadVoiceModels(); }, 40);
  return '<details class="yyq-cmp yyq-voice yyq-subfold">' +
    '<summary><b>语音回复</b><small>让 AI 用声音说话（OneBot / NapCat）</small></summary>' +
    '<div class="yyq-fields">' +
      '<label class="yyq-field"><span>语音模型（TTS）</span><select id="yyq-voice-model" data-cur="' + yyQQE(model) + '">' +
        '<option value="">不用语音模型</option>' +
      "</select></label>" +
      '<label class="yyq-field"><span>音色</span><input id="yyq-voice-name" maxlength="40" value="' + yyQQE(voice) + '" placeholder="alloy"></label>' +
    "</div>" +
    '<div class="yyq-seg yyq-seg-wide" id="yyq-voice-mode" data-mode="' + mode + '" role="tablist">' +
      '<button type="button" role="tab" class="' + (mode === "off" ? "is-on" : "") + '" aria-selected="' + (mode === "off" ? "true" : "false") + '" onclick="yyQQVoiceModePick(\'off\')">关闭</button>' +
      '<button type="button" role="tab" class="' + (mode === "auto" ? "is-on" : "") + '" aria-selected="' + (mode === "auto" ? "true" : "false") + '" onclick="yyQQVoiceModePick(\'auto\')">AI 决定</button>' +
      '<button type="button" role="tab" class="' + (mode === "always" ? "is-on" : "") + '" aria-selected="' + (mode === "always" ? "true" : "false") + '" onclick="yyQQVoiceModePick(\'always\')">总是语音</button>' +
    "</div>" +
    '<p class="yyq-cmp-note">关闭：不发语音，但群里仍可用「.语音 文本/音频URL」。' +
      "AI 决定：该用语音时它自己会发。" +
      "总是语音：每条文字回复照发，另外补一条语音（合不上成就只发文字，不会顶掉正文）。" +
      "模型需要支持 OpenAI 兼容的 /audio/speech 接口。</p>" +
    '<div class="yyq-fold-actions"><button type="button" class="btn" onclick="yyQQSaveVoice(this)">保存语音设置</button></div>' +
    "</details>";
}

// 语音模型下拉：复用模型库，已保存但不在库里的值也保留（否则一次重绘就丢了）。
async function yyQQLoadVoiceModels() {
  var sel = document.getElementById("yyq-voice-model");
  if (!sel) return;
  var cur = sel.getAttribute("data-cur") || sel.value || "";
  var r = await A("GET", "/api/ai/models");
  var models = (r && r.ok && r.models) || [];
  var html = '<option value="">不用语音模型</option>';
  models.forEach(function (m) {
    html += '<option value="' + yyQQE(m.name) + '">' + yyQQE(m.name + (m.model_id ? " (" + m.model_id + ")" : "")) + "</option>";
  });
  sel.innerHTML = html;
  if (cur && !models.some(function (m) { return m.name === cur; })) {
    var o = document.createElement("option");
    o.value = cur; o.textContent = cur + "（不在模型库）";
    sel.appendChild(o);
  }
  sel.value = cur || "";
}

function yyQQVoiceModePick(mode) {
  var box = document.getElementById("yyq-voice-mode");
  if (!box) return;
  box.setAttribute("data-mode", mode);
  var btns = box.querySelectorAll("button");
  var order = ["off", "auto", "always"];
  for (var i = 0; i < btns.length; i++) {
    var on = order[i] === mode;
    btns[i].className = on ? "is-on" : "";
    btns[i].setAttribute("aria-selected", on ? "true" : "false");
  }
}

async function yyQQSaveVoice(btn) {
  var box = document.getElementById("yyq-voice-mode");
  var mode = (box && box.getAttribute("data-mode")) || "off";
  if (mode !== "auto" && mode !== "always") mode = "off";
  var model = ((document.getElementById("yyq-voice-model") || {}).value || "").trim();
  var voice = ((document.getElementById("yyq-voice-name") || {}).value || "").trim().slice(0, 40);
  if (btn) { btn.setAttribute("data-busy", "1"); btn.textContent = "保存中"; }
  var r;
  try {
    r = await A("POST", "/api/qq/config", { assistant: { voice_model: model, voice_voice: voice, voice_mode: mode } });
  } finally {
    if (btn) { btn.setAttribute("data-busy", "0"); btn.textContent = "保存语音设置"; }
  }
  if (!r || !r.ok) { T((r && r.error) || "保存失败", "e"); return; }
  yyQQC.cfg = yyQQC.cfg || {};
  yyQQC.cfg.assistant = yyQQC.cfg.assistant || {};
  yyQQC.cfg.assistant.voice_model = model;
  yyQQC.cfg.assistant.voice_voice = voice;
  yyQQC.cfg.assistant.voice_mode = mode;
  if (btn) { btn.textContent = "✓ 已保存"; setTimeout(function () { if (btn) btn.textContent = "保存语音设置"; }, 1400); }
  T(mode === "off" ? "语音回复已关闭" : mode === "always" ? "每条回复都会补语音" : "AI 可以自己决定发语音", "o");
}

async function yyQQSavePersona(btn) {
  // 只提交用户真的动过的字段：prompt 没编辑过就不发（避免用空串覆盖），
  // model 下拉在模型列表还没回来时值是空串，同样不能拿它覆盖已保存的模型。
  var patch = {};
  yyQQPromptPatch(patch);
  var modelEl = document.getElementById("yyq-model");
  if (yyQQModelDirty && modelEl) patch.model = modelEl.value || "";
  if (!Object.keys(patch).length) { T("没有改动要保存", "w"); return; }
  if (btn) { btn.setAttribute("data-busy", "1"); btn.textContent = "保存中"; }
  var r = await A("POST", "/api/qq/config", { assistant: patch });
  if (btn) { btn.setAttribute("data-busy", "0"); btn.textContent = "保存人设"; }
  if (!r || !r.ok) { T((r && r.error) || "保存失败", "e"); return; }
  yyQQC.cfg.assistant = yyQQC.cfg.assistant || {};
  if (patch.model !== undefined) { yyQQC.cfg.assistant.model = patch.model; yyQQModelDirty = 0; }
  if (patch.prompt !== undefined) yyQQPromptSync(patch.prompt);
  if (btn) { btn.textContent = "✓ 已保存"; setTimeout(function () { if (btn) btn.textContent = "保存人设"; }, 1400); }
  T("人设已保存", "o");
}

// ---------- 6 收藏表情 ----------

function yyQQFaces() {
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
    body = '<div class="yyq-face-grid">' + yyQQC.faces.map(function (f, i) {
      return '<figure class="yyq-face"><img src="' + yyQQE(f.url) + '" alt="" loading="lazy" decoding="async" referrerpolicy="no-referrer"' +
        ' onerror="this.parentNode.classList.add(\'is-broken\')"><figcaption>' + yyQQE(f.name || f.id || "表情") + "</figcaption>" +
        // 「点一下直发」：常驻小按钮（44px 命中区），以机器人身份发到默认启用群。
        '<button type="button" class="yyq-face-send" title="以机器人身份发到默认群" aria-label="发送这个表情"' +
        ' style="position:absolute;top:6px;right:6px;width:44px;height:44px;border:0;border-radius:12px;cursor:pointer;' +
        'display:grid;place-items:center;font-size:11px;color:#fff;background:rgba(0,0,0,.55)"' +
        ' onclick="yyQQSendFace(' + i + ',this)">发送</button></figure>';
    }).join("") + "</div>";
  }
  return yyQQSec(
    yyQQSecHead("收藏表情", count ? count + " 个" : "未读取") + body,
    "yyq-sec-wide");
}

async function yyQQLoadFaces() {
  yyQQC.facesLoading = true;
  yyQQC.facesErr = "";
  yyQQC.facesOpen = true;
  yyQQRender("mod-robot");
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
  yyQQRender("mod-robot");
  var box = document.getElementById("yyq-faces");
  if (box) box.setAttribute("open", "");
}

// 「点一下直发」：以机器人身份把这张收藏表情发到当前默认启用群。
// 用户原话：「收藏的表情包当中，我们打开了工具箱，我们点一下他收藏的表情包
// 就直接以他的身份来发」。多群时先弹个小选择器让用户选群。
// 发送走面板动作接口 POST /api/qq/actions（action=sticker，file=表情URL，
// group=目标群；后端未识别的字段会被忽略，失败时错误原因原样透传）。
async function yyQQSendFace(i, btn) {
  var f = (yyQQC.faces || [])[i];
  if (!f) return;
  var url = String(f.url || "").trim();
  if (!url) { T("这个表情没有可用的图片地址，换一个试试", "e"); return; }
  var groups = yyQQEnabledGroups();
  if (!groups.length) { T("还没有启用的互通群，先在上面「接入」里配一个", "e"); return; }
  var target = groups.length === 1 ? (groups[0].id || groups[0].group_id) : "";
  if (groups.length > 1) {
    target = await yyQQPickFaceGroup(groups);
    if (!target) return;
  }
  if (btn) { btn.disabled = true; btn.textContent = "发…"; }
  var r = await A("POST", "/api/qq/actions", { action: "sticker", file: url, group: target });
  if (btn) { btn.disabled = false; btn.textContent = "发送"; }
  if (!r || !r.ok) {
    T("发送失败：" + ((r && r.error) || "未知错误"), "e");
    return;
  }
  T("表情已发出", "o");
}

// 多群选择器（发表情包用）：自包含小弹层（不依赖新增 CSS），点群卡或遮罩即关。
// 注意与接入区的 yyQQToggleGroup(id)（勾选互通群）区分，别重名。
function yyQQPickFaceGroup(groups) {
  return new Promise(function (resolve) {
    var old = document.getElementById("yyq-face-pick");
    if (old) old.parentNode.removeChild(old);
    var host = document.createElement("div");
    host.id = "yyq-face-pick";
    host.setAttribute("style", "position:fixed;inset:0;z-index:9999;display:grid;place-items:center;padding:20px;background:rgba(0,0,0,.45)");
    var card = document.createElement("div");
    card.setAttribute("style", "width:min(320px,100%);max-height:70vh;overflow:auto;display:flex;flex-direction:column;gap:8px;" +
      "padding:16px;border-radius:16px;background:var(--phx-bg-card,#fff);border:1px solid var(--phx-border-light,#e3e8ea)");
    var title = document.createElement("strong");
    title.textContent = "发到哪个群？";
    title.setAttribute("style", "font-size:13px");
    card.appendChild(title);
    function close(v) {
      if (host.parentNode) host.parentNode.removeChild(host);
      resolve(v);
    }
    groups.forEach(function (g) {
      var b = document.createElement("button");
      b.type = "button";
      b.className = "btn-s";
      b.textContent = yyQQGroupName(g) + "（" + (g.id || g.group_id) + "）";
      b.setAttribute("style", "min-height:44px;text-align:left");
      b.onclick = function () { close(g.id || g.group_id); };
      card.appendChild(b);
    });
    var cancel = document.createElement("button");
    cancel.type = "button";
    cancel.className = "yyq-ghost";
    cancel.textContent = "取消";
    cancel.setAttribute("style", "min-height:44px");
    cancel.onclick = function () { close(""); };
    card.appendChild(cancel);
    host.appendChild(card);
    host.addEventListener("click", function (e) { if (e.target === host) close(""); });
    document.body.appendChild(host);
  });
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

// 6 个模块的内置内容。其它脚本用 yyQQRegisterModule() 往同一个模块里追加分区。
function yyQQModChatRender() {
  var lg = yyQQC.login || {};
  return yyQQSec(yyQQSecHead("接入与互通群", "保存后立即重连") + yyQQAccess(), "yyq-sec-wide") +
    yyQQSec(yyQQSecHead("掉线自愈", "协议端掉线后自动重启并走快速登录") + yyQQWatchdogBox(lg), "yyq-sec-wide") +
    yyQQSec('<div class="yyq-sec-link"><span>成员授权、记忆、模型接口在完整面板里</span>' +
      '<button type="button" class="btn-s" onclick="yyQQOpenAssistantPanel()">打开群服互通助手</button></div>', "yyq-sec-wide");
}
function yyQQModRecordsRender() {
  var t = yyQQTodayActions();
  var rows = (yyQQC.actions || []).slice(-5).reverse();
  var list = rows.length ? '<ul class="yyq-cap-list">' + rows.map(function (a) {
    return '<li class="yyq-sys" data-bad="' + (a.ok ? "0" : "1") + '">' + yyQQIcon(a.ok ? "check" : "ban") +
      "<b>" + yyQQE(a.action || "动作") + "</b><small>" + yyQQE(a.detail || a.group || "") + "</small>" +
      "<time>" + yyQQE(yyQQAgo(Date.parse(a.time))) + "</time></li>";
  }).join("") + "</ul>" : '<p class="yyq-cmp-note">还没有动作记录。</p>';
  return yyQQSec(
    yyQQSecHead("今日动作", "成功 " + Math.max(0, t.total - t.fail) + " · 失败 " + t.fail) + list,
    "yyq-sec-wide");
}
function yyQQModAIRender() {
  return yyQQPersonaPrompt() + yyQQPersonaModel() + yyQQPersonaBehavior() + yyQQPersonaSandbox() + yyQQTools();
}
function yyQQModRobotRender() {
  return yyQQFaces();
}
// MCP 作用域（只读）：/api/youyuan/mcp/scope 已经返回了 scope/owner/access，
// 这里只摊开给管理员看，不改任何一条配置。
var yyQQMCPCache = null, yyQQMCPBusy = 0;
function yyQQMCPRows() {
  if (!yyQQMCPCache) {
    if (!yyQQMCPBusy) {
      yyQQMCPBusy = 1;
      A("GET", "/api/youyuan/mcp/scope").then(function (r) {
        yyQQMCPBusy = 0;
        if (r && r.ok) { yyQQMCPCache = r.servers || []; yyQQRender("mod-admin"); }
      })["catch"](function () { yyQQMCPBusy = 0; });
    }
    return '<p class="yyq-cmp-note">正在读取 MCP 作用域…</p>';
  }
  if (!yyQQMCPCache.length) return '<p class="yyq-cmp-note">还没有登记 MCP 服务器。</p>';
  return '<ul class="yyq-cap-list">' + yyQQMCPCache.map(function (s) {
    return '<li class="yyq-cap" data-on="' + (s.enabled ? "1" : "0") + '">' +
      '<span class="yyq-cap-ico">' + yyQQIcon("connection") + "</span>" +
      '<span class="yyq-cap-body"><strong>' + yyQQE(s.name || s.id) + "</strong>" +
      "<small>" + yyQQE((s.transport || "http") + " · " + (s.url || s.command || "")) + "</small></span>" +
      '<span class="yyq-cap-meta">' + yyQQE((s.scope === "private" ? "私有" : "公开") + " · " + (s.access === "owner_only" ? "仅归属人" : s.access) + (s.owner ? " · " + s.owner : "")) + "</span>" +
      "</li>";
  }).join("") + "</ul>";
}
function yyQQModAdminRender() {
  return yyQQSec(yyQQSecHead("MCP 作用域", "只读 · 改配置去「公网远控 / MCP」") + yyQQMCPRows(), "yyq-sec-wide");
}
function yyQQModRootRender() {
  var as = (yyQQC.cfg || {}).assistant || {};
  var lg = yyQQC.login || {};
  var w = lg.watchdog || {};
  var admins = (as.admins || []).join("、") || "未设置";
  var supers = (as.super_admins || []).join("、") || "未设置";
  return yyQQSec(
    yyQQSecHead("超级管理员认领", "生成后 10 分钟内有效，一次使用") +
    '<button type="button" class="btn-s" onclick="yyQQCreateClaim()">生成认领码</button>' +
    '<pre id="yy-qq-claim" class="yyq-claim" style="white-space:pre-wrap;word-break:break-all"></pre>'
  ) + yyQQSec(
    yyQQSecHead("管理员名单", "只读视图 · 授权在完整面板里") +
    '<p class="yyq-model-line">超级管理员：<b>' + yyQQE(supers) + "</b></p>" +
    '<p class="yyq-model-line">管理员：<b>' + yyQQE(admins) + "</b></p>" +
    '<button type="button" class="btn-s" onclick="yyQQOpenAssistantPanel()">去授权</button>'
  ) + yyQQSec(
    yyQQSecHead("模型库", "语言模型与绘图模型") +
    '<button type="button" class="btn-s" onclick="aiOpenManage()">打开 AI 模型配置</button>'
  ) + yyQQSec(
    yyQQSecHead("掉线自愈高级参数", "实际调整在「聊天与账号」") +
    '<p class="yyq-model-line">状态：<b>' + (w.enabled ? "已开启" : "已关闭") + "</b> · 重连上限 <b>" +
    yyQQE(String(w.max_restarts || w.max_per_hour || 3)) + "</b> 次 · 间隔 <b>" +
    yyQQE(String(w.interval_seconds || 60)) + "</b> 秒</p>" +
    (w.last_error ? '<p class="yyq-model-line">上次失败：<b>' + yyQQE(w.last_error) + "</b></p>" : "")
  );
}

// 注册内置分区（order 决定模块内顺序）。
yyQQRegisterModule({ id: "chat", order: 10, minRole: 0, __key: "access", render: yyQQModChatRender });
yyQQRegisterModule({ id: "records", order: 10, minRole: 0, __key: "today", render: yyQQModRecordsRender });
yyQQRegisterModule({ id: "ai", order: 10, minRole: 0, __key: "persona", render: yyQQModAIRender,
  load: function () { if (!yyQQC.toolsLoaded && !yyQQC.toolsLoading) yyQQLoadTools(); } });
yyQQRegisterModule({ id: "robot", order: 10, minRole: 0, __key: "faces", render: yyQQModRobotRender,
  load: function () { if (!yyQQC.facesLoaded && !yyQQC.facesLoading) yyQQLoadFaces(); } });
yyQQRegisterModule({ id: "admin", order: 20, minRole: 1, __key: "mcp-scope", render: yyQQModAdminRender });
yyQQRegisterModule({ id: "root", order: 10, minRole: 2, __key: "root", render: yyQQModRootRender });

var yyQQParts = {
  status: yyQQStatusBar,
  hero: yyQQHero,
  caps: yyQQCaps,
  stream: yyQQStream
};
yyQQModMeta.forEach(function (m) {
  yyQQParts["mod-" + m.id] = function () { return yyQQModuleShell(m.id); };
});

// 每个区块记住上次写进 DOM 的 html 指纹：内容没变就整块跳过。
// 以前 yyQQRenderAll 用 root.innerHTML 整页重建，一次刷新所有节点全换掉——
// 正在输入的框失焦、滚动位置跳动、由别的模块注入的折叠块（触发词/生图/
// 群服互通助手/知识库）被连根拔起再补回去，看起来就是「一跳一跳」。
var yyQQFp = {};
function yyQQFpSame(part, html) {
  if (yyQQFp[part] === html) return true;
  yyQQFp[part] = html;
  return false;
}
// 整页结构里由 console 自己负责的区块：4 个运行时区 + 6 个模块。
// 按角色裁掉 admin/root 时不渲染，索引也跟着变，所以用函数算。
function yyQQPartListNow() {
  var list = ["status", "hero", "caps", "stream"];
  yyQQModList().forEach(function (m) { list.push("mod-" + m.id); });
  return list;
}
// ---------- 快捷跳转 chips ----------
// 手机端这个页面很长，关键设置都在模块里面。
// 点谁就把对应模块展开并滚过去；元素不存在（角色不够）就整颗藏掉。
var yyQQQuickNavItems = [
  { id: "yyq-mod-chat", label: "聊天" },
  { id: "yyq-mod-records", label: "记录" },
  { id: "yyq-mod-ai", label: "AI" },
  { id: "yyq-mod-robot", label: "机器人" },
  { id: "yyq-mod-admin", label: "管理" },
  { id: "yyq-mod-root", label: "超管" }
];
function yyQQQuickNav() {
  return '<nav class="yyq-quicknav" id="yyq-quicknav" aria-label="快捷跳转">' +
    yyQQQuickNavItems.map(function (x) {
      return '<button type="button" class="yyq-quicknav-chip" data-yyq-go="' + x.id +
        '" onclick="yyQQQuickNavGo(\'' + x.id + '\')">' + x.label + "</button>";
    }).join("") + "</nav>";
}
function yyQQQuickNavSync() {
  var nav = document.getElementById("yyq-quicknav");
  if (!nav) return;
  var chips = nav.querySelectorAll("[data-yyq-go]");
  var shown = 0;
  for (var i = 0; i < chips.length; i++) {
    var ok = !!document.getElementById(chips[i].getAttribute("data-yyq-go"));
    chips[i].hidden = !ok;
    if (ok) shown++;
  }
  nav.hidden = shown === 0;
}
function yyQQQuickNavGo(id) {
  var el = document.getElementById(id);
  if (!el) { yyQQQuickNavSync(); return; }
  // 模块先展开再滚。只改 open 走 ontoggle 同步状态，不整块重绘，
  // 已经填进去的地址、提示词不会丢。
  if (el.tagName === "DETAILS" && !el.open) {
    el.open = true;
    if (el.getAttribute("data-mod")) {
      yyQQC.modOpen[el.getAttribute("data-mod")] = true;
      yyQQModLoad(el.getAttribute("data-mod"));
    }
  }
  yyQQQuickNavAnchor(id);
}
// 技能这类折叠块是展开后才异步拉数据，行是后插进来的。原生 scrollIntoView 的
// 平滑动画会和这些插入打架：实测点「技能」后动画跑到一半被重排打断，落点被甩出
// 近 500px，再写 scrollTop 也会被没结束的原生动画盖掉。所以平滑自己画：
// 每帧按当前布局重算目标、缓动逼近，最后按最终布局硬校正；用户中途自己滚就停。
function yyQQQuickNavScroller(el) {
  var n = el.parentElement;
  while (n && n !== document.body && n !== document.documentElement) {
    var cs = getComputedStyle(n);
    if (/(auto|scroll)/.test(cs.overflowY) && n.scrollHeight > n.clientHeight + 4) return n;
    n = n.parentElement;
  }
  return null;
}
// delta = 还要往下滚多少像素才能让目标顶边落在「状态带下面」
function yyQQQuickNavGeo(id) {
  var tar = document.getElementById(id);
  if (!tar) return null;
  var margin = parseFloat(getComputedStyle(tar).scrollMarginTop) || 0;
  var sc = yyQQQuickNavScroller(tar);
  var scTop = sc ? sc.getBoundingClientRect().top : 0;
  // 状态带是 sticky 的，实际高度会随内容变（手机上量到 74px），
  // 所以上边界取它此刻的真实底边，别把区块顶到状态带后面去。
  var bar = document.getElementById("yyq-status");
  if (bar && bar !== tar) {
    var gap = bar.getBoundingClientRect().bottom - scTop + 8;
    if (gap > margin) margin = gap;
  }
  var box = tar.getBoundingClientRect();
  var delta = sc ? box.top - scTop - margin : box.top + window.pageYOffset - margin;
  return { sc: sc, delta: delta };
}
function yyQQQuickNavApply(g) {
  if (!g || Math.abs(g.delta) < 1) return;
  if (g.sc) g.sc.scrollTop = Math.max(0, g.sc.scrollTop + g.delta);
  else window.scrollTo(0, Math.max(0, window.pageYOffset + g.delta));
}
// 连点不同 chip 时，前一次的校正循环要立刻失效，否则两个循环会互相抢滚动条。
var yyQQQuickNavToken = 0;
function yyQQQuickNavAnchor(id) {
  var token = ++yyQQQuickNavToken;
  var cancelled = false;
  var alive = function () { return token === yyQQQuickNavToken && !cancelled; };
  var cancel = function () { cancelled = true; };
  ["wheel", "touchstart", "mousedown", "keydown"].forEach(function (t) {
    window.addEventListener(t, cancel, { passive: true, once: true });
  });
  var t0 = 0;
  var step = function (now) {
    if (!t0) t0 = now;
    if (!alive()) return;
    var g = yyQQQuickNavGeo(id);
    if (!g) return;
    // 到位、或缓动超过 520ms（下面交给定时校正）就收尾硬对齐。
    if (Math.abs(g.delta) < 1 || now - t0 > 520) { yyQQQuickNavApply(g); return; }
    g.sc ? (g.sc.scrollTop = Math.max(0, g.sc.scrollTop + g.delta * 0.3))
      : window.scrollTo(0, Math.max(0, window.pageYOffset + g.delta * 0.3));
    requestAnimationFrame(step);
  };
  if (window.requestAnimationFrame) requestAnimationFrame(step);
  else yyQQQuickNavApply(yyQQQuickNavGeo(id));
  // 数据回来会把 yyq-caps 整块重绘（技能折在里面）并重排，落点会被挤偏几十像素，
  // 所以 5 秒内每 250ms 校一次位（已在位就不动），不等「稳定几次就收工」。
  var t1 = Date.now();
  var timer = setInterval(function () {
    if (!alive() || Date.now() - t1 > 5000) { clearInterval(timer); return; }
    yyQQQuickNavApply(yyQQQuickNavGeo(id));
  }, 250);
}
function yyQQShellHTML() {
  var mods = "";
  yyQQModList().forEach(function (m) { mods += yyQQModuleShell(m.id); });
  return yyQQStatusBar() +
    yyQQQuickNav() +
    yyQQHero() +
    '<div class="yyq-grid">' + yyQQCaps() + yyQQStream() + "</div>" +
    '<div class="yyq-mod-grid">' + mods + "</div>";
}
function yyQQShellReady() {
  // 只检查当前角色下应该存在的区块：admin/root 在权限不足时本来就不渲染，
  // 如果还按固定 10 项检查，ShellReady 永远 false，每次刷新都整壳重建。
  var list = yyQQPartListNow();
  for (var i = 0; i < list.length; i++) {
    if (!document.getElementById("yyq-" + list[i])) return false;
  }
  return true;
}

// 消息面板每 30 秒整块重写一次，滚动位置会跟着归零：
// 时间线要留在原来那一段（贴底时继续贴底），群标签那排也要留在原地，
// 否则用户刚滑到第 5 个群，一刷新又跳回第一个。
function yyQQScrollKeep(el) {
  var tl = el.querySelector ? el.querySelector(".yyq-tl") : null;
  var tabs = el.querySelector ? el.querySelector(".yyq-gtabs") : null;
  if (!tl && !tabs) return null;
  return {
    tl: tl ? { top: tl.scrollTop, stick: tl.scrollHeight - tl.scrollTop - tl.clientHeight < 32 } : null,
    tabs: tabs ? tabs.scrollLeft : 0
  };
}
function yyQQScrollRestore(keep) {
  var tl = document.querySelector("#yyq-stream .yyq-tl");
  if (tl && keep.tl) tl.scrollTop = (yyQQC.stickBottom || keep.tl.stick) ? tl.scrollHeight : keep.tl.top;
  var tabs = document.querySelector("#yyq-stream .yyq-gtabs");
  if (tabs) {
    tabs.scrollLeft = keep.tabs || 0;
    var on = tabs.querySelector('.yyq-gtab[data-on="1"]');
    if (on) {
      var left = on.offsetLeft - 12;
      var right = left + on.offsetWidth + 24;
      if (left < tabs.scrollLeft) tabs.scrollLeft = left;
      else if (right > tabs.scrollLeft + tabs.clientWidth) tabs.scrollLeft = right - tabs.clientWidth;
    }
  }
  yyQQC.stickBottom = 0;
}

function yyQQRender(part) {
  var fn = yyQQParts[part];
  if (!fn) return;
  var el = document.getElementById("yyq-" + part);
  if (!el) return;
  var html = fn();
  // 数据没变就不写 DOM：滚动位置、展开状态、正在输入的内容都不动。
  if (yyQQFpSame(part, html)) return;
  var wasOpen = el.open;
  var keep = part === "stream" ? yyQQScrollKeep(el) : null;
  el.outerHTML = html;
  if (keep) yyQQScrollRestore(keep);
  if (part.indexOf("mod-") === 0) {
    var mid = part.slice(4);
    var again = document.getElementById("yyq-" + part);
    var open = yyQQC.modOpen[mid];
    if (open == null) {
      var meta = yyQQModMetaById(mid);
      open = !!(meta && meta.open && meta.open());
    }
    if (again && open) again.setAttribute("open", "");
    if (again && open) yyQQModLoad(mid);
  }
}

// 首屏/整壳重建后停在最新一条：时间线是「老的在上」，不滚下去只看得见最旧的消息。
function yyQQStreamBottom() {
  var tl = document.querySelector("#yyq-stream .yyq-tl");
  if (tl) tl.scrollTop = tl.scrollHeight;
  yyQQC.stickBottom = 0;
}

function yyQQRenderAll() {
  var root = document.getElementById("yyq-root");
  if (!root) return;
  // A1：用户正在「触发词规则」等配置区块里展开/输入时，整页重绘会把 DOM 换掉、
  // 输入框失焦，看起来就是「点了没反应」。标记由 youyuan_qq_config.js 的
  // focus 守卫维护，命中就直接跳过这次重绘（10 分钟定时刷新也走这里）。
  if (window.__yyQQEditing) return;
  // 首屏或容器被整体换掉时补一次整壳；之后一律按区块增量写。
  // 数据没变的区块完全不碰 DOM。
  if (!yyQQShellReady()) {
    yyQQFp = {};
    root.innerHTML = yyQQShellHTML();
    yyQQSyncFolds();
    yyQQStreamBottom();
    yyQQModLoadOpen();
    yyQQQuickNavSync();
    setTimeout(yyQQQuickNavSync, 0);
    return;
  }
  var list = yyQQPartListNow();
  for (var i = 0; i < list.length; i++) yyQQRender(list[i]);
  yyQQSyncFolds();
  yyQQModLoadOpen();
  // 注册模块的数据是异步回来的，模块自己会重绘；这里再同步一次 chips。
  yyQQQuickNavSync();
  setTimeout(yyQQQuickNavSync, 0);
}
function yyQQSyncFolds() {
  yyQQModList().forEach(function (m) {
    var el = document.getElementById("yyq-mod-" + m.id);
    if (!el) return;
    var open = yyQQC.modOpen[m.id];
    if (open == null) open = !!(m.open && m.open());
    if (open) el.setAttribute("open", "");
    else el.removeAttribute("open");
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
  // 掉线感知：state 离开 open 就算断。以前这里什么都不做，所以掉线了页面一直显示旧状态。
  // 另外 state=open 但 live=false 也是断（半开 TCP 时 state 不会变），必须一起判。
  var stNow = yyQQC.status || {}, lgNow = yyQQC.login || {};
  var nowConnected = (stNow.state === "open" && stNow.live !== false) ||
    !!(lgNow.logged_in && lgNow.live !== false);
  yyQQCheckDrop(nowConnected, (yyQQC.status && yyQQC.status.detail) || "");
  if (!yyQQC.picked) {
    var on = yyQQEnabledGroups();
    yyQQC.picked = on.length ? (on[0].id || on[0].group_id) : (yyQQC.cfg.group_id || "");
  }
  // 接入区勾选框的草稿：没在编辑时才跟随配置，正在勾的时候别被定时刷新冲掉。
  if (!yyQQC.selTouched) yyQQC.sel = yyQQEnabledGroups().map(yyQQGid).filter(Boolean);
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
  var group = yyQQC.picked || "";
  if (!group) {
    yyQQC.messages = [];
    yyQQC.loading.stream = 0;
    yyQQC.msgErr = "";
    return;
  }
  var rows = await yyQQLoadGroupMessages(group, 40);
  // 请求飞在路上时用户切了群：这份响应已经过期，丢掉，别覆盖新群的内容。
  if (group !== yyQQC.picked) return;
  yyQQC.loading.stream = 0;
  if (rows == null) {
    yyQQC.messages = [];
    yyQQC.msgErr = yyQQC.lastMsgErr || "读取消息失败";
    return;
  }
  yyQQC.messages = rows;
  yyQQC.msgErr = "";
  yyQQMarkRead(group);
}

// 读一个群的正文并缓存。切群时先用缓存上屏，所以这个函数也可能只是刷新。
// 失败只记在 lastMsgErr 里，由「当前群」的调用方决定要不要显示——
// 后台给别的群补未读失败，不该把当前群的时间线打成错误态。
async function yyQQLoadGroupMessages(group, limit) {
  var r = await A("GET", "/api/qq/messages?group=" + encodeURIComponent(group) + "&limit=" + (limit || 40));
  if (!r || !r.ok) {
    yyQQC.lastMsgErr = (r && r.error) || "读取消息失败";
    return null;
  }
  var rows = r.messages || [];
  yyQQC.msgCache[group] = { rows: rows, at: Date.now() };
  return rows;
}

// 记「这个群我看到哪条了」：未读数按「比它新的条数」算，不用服务端维护已读状态。
function yyQQMarkRead(group) {
  var cache = yyQQC.msgCache[group];
  var rows = (cache && cache.rows) || [];
  var t = yyQQC.readAt[group] || 0;
  rows.forEach(function (m) {
    var v = Date.parse(m.timestamp) || 0;
    if (v > t) t = v;
  });
  if (t) yyQQC.readAt[group] = t;
  yyQQC.unread[group] = 0;
  yyQQReadSave();
}

// 未读汇总：一条很小的请求拿到每个群的最后一条时间，只有真有新消息的群才去拉正文。
// 首屏只拉当前群的正文（yyQQLoadMessages），其余群等这里发现新消息再拉。
async function yyQQLoadSummary() {
  var groups = yyQQEnabledGroups();
  if (!groups.length) return;
  var r = await A("GET", "/api/qq/messages/summary");
  if (!r || !r.ok) return;
  var pending = [];
  (r.groups || []).forEach(function (g) {
    var id = g.group || "";
    if (!id || id === yyQQC.picked) return;
    var at = Date.parse(g.last_at) || 0;
    // 第一次见到这个群：把现有消息当作已读，别一打开就满屏红点。
    if (!(id in yyQQC.readAt)) {
      if (at) yyQQC.readAt[id] = at;
      return;
    }
    if (at > (yyQQC.readAt[id] || 0)) pending.push(id);
  });
  yyQQReadSave();
  if (!pending.length) return;
  if (yyQQC.syncing.busy) return;
  yyQQC.syncing.busy = 1;
  var changed = 0;
  try {
    // 一次最多补 3 个群，别让"补未读"变成一串请求风暴。
    for (var i = 0; i < pending.length && i < 3; i++) {
      var id2 = pending[i];
      if (id2 === yyQQC.picked) continue;
      var rows = await yyQQLoadGroupMessages(id2, 40);
      if (!rows) continue;
      var since = yyQQC.readAt[id2] || 0;
      var n = 0;
      rows.forEach(function (m) { if ((Date.parse(m.timestamp) || 0) > since) n++; });
      if ((yyQQC.unread[id2] || 0) !== n) { yyQQC.unread[id2] = n; changed = 1; }
    }
  } finally {
    yyQQC.syncing.busy = 0;
  }
  if (changed) yyQQRender("stream");
}

async function yyQQRefreshStream() {
  await yyQQLoadMessages();
  yyQQRender("stream");
  yyQQRender("status");
  yyQQLoadSummary();
}

// btn 传进来才有 loading 态。以前这个按钮点了什么都不显示：
// 请求在后台跑，成功没提示、失败被吞掉，用户看到的就是「假按钮」。
async function yyQQRefreshAll(deep, btn) {
  if (yyQQC.refreshing) return;
  yyQQC.refreshing = 1;
  var busy = function (on) {
    if (!btn || !btn.style) return;
    btn.disabled = !!on;
    btn.style.opacity = on ? "0.45" : "";
    btn.style.cursor = on ? "progress" : "";
    if (on) btn.setAttribute("aria-busy", "true");
    else btn.removeAttribute("aria-busy");
  };
  busy(1);
  try {
    await yyQQLoadCore();
    await yyQQLoadMessages();
    // 用户正在配置区编辑时，yyQQRenderAll 会被 focus 守卫整体跳过。
    // 这时至少把状态带画出来，否则点了刷新看起来毫无反应。
    if (window.__yyQQEditing) {
      yyQQRender("status");
      if (typeof T === "function") T("正在编辑，只刷新了状态栏", "s");
    } else {
      yyQQRenderAll();
    }
    yyQQLoadProfile();
    yyQQLoadSummary();
    if (deep) yyQQPollLogin(1);
    if (typeof T === "function") T("已刷新", "s");
  } catch (e) {
    if (typeof T === "function") T("刷新失败：" + ((e && e.message) || e || "未知错误"), "e");
  } finally {
    yyQQC.refreshing = 0;
    busy(0);
  }
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
  }, 60000));
  // 实时流每 15 秒刷新。
  yyQQC.timers.push(setInterval(function () {
    if (!document.getElementById("yyq-stream")) return;
    yyQQRefreshStream();
  }, 30000));
  // 每 5 秒轻量探一次连接状态：只刷状态栏，不动别的区域，
  // 这样掉线能在 5 秒内被看见，又不会打断正在输入的框。
  yyQQC.timers.push(setInterval(function () {
    if (document.hidden) return;
    if (!document.getElementById("yyq-root")) return;
    yyQQPollStatusLight();
  }, 5000));
  // 每 10 分钟整页状态重刷一次：用户要求「停在那个界面也要自己发现掉线」。
  // 页面不可见时跳过，省电也省接口。
  yyQQC.timers.push(setInterval(function () {
    if (document.hidden) return;
    if (!document.getElementById("yyq-root")) return;
    yyQQRefreshAll();
  }, 10 * 60 * 1000));
  // 切回前台立刻刷一次，手机上从后台回来能马上看到真实状态。
  if (yyQQC.visHandler && document.removeEventListener) {
    document.removeEventListener("visibilitychange", yyQQC.visHandler);
  }
  yyQQC.visHandler = function () {
    if (document.hidden) return;
    if (!document.getElementById("yyq-root")) return;
    // 先打一发轻量状态，切回前台第一眼就是真状态，不用等整页数据拉完。
    yyQQPollStatusLight();
    yyQQRefreshAll();
  };
  document.addEventListener("visibilitychange", yyQQC.visHandler);
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
  // 只更新「是否过期」的标记，不再自动盖遮罩：
  // 遮罩只在用户主动点「重新生成」时出现（yyQQC.qrBusy），
  // 否则文件时间一旧就永远盖着，二维码明明还能扫也扫不了。
  var stale = age > 110;
  if ((box.getAttribute("data-stale") === "1") !== stale) {
    box.setAttribute("data-stale", stale ? "1" : "0");
  }
}

// 轻量状态探活：只打 /api/qq/status，拿到就判断掉线并重绘状态栏。
// 5 秒一次，正常时开销就是一个 ~200B 的 JSON，不会像整页刷新那样把输入框冲掉。
var yyQQStatusPolling = false;
async function yyQQPollStatusLight() {
  if (yyQQStatusPolling) return;
  yyQQStatusPolling = true;
  try {
    var r = await A("GET", "/api/qq/status");
    if (r && r.ok !== false) {
      yyQQC.statusFails = 0;
      var before = yyQQC.status && yyQQC.status.state;
      var beforeLive = yyQQC.status && yyQQC.status.live;
      r.stale = false;
      yyQQC.status = r;
      var nowConnected = r.state === "open" && r.live !== false;
      yyQQCheckDrop(nowConnected, r.detail || "");
      if (before !== r.state || beforeLive !== r.live || yyQQC.dropAt) yyQQRender("status");
      return;
    }
    // 探测失败必须说话。以前这里直接 return，掉线+接口打不通时面板会永远停在「已连接」。
    yyQQC.statusFails = (yyQQC.statusFails || 0) + 1;
    if (yyQQC.statusFails >= 4) {
      yyQQC.status = Object.assign({}, yyQQC.status || {}, {
        stale: true, detail: (r && r.error) || "状态接口无响应"
      });
      yyQQCheckDrop(false, (r && r.error) || "状态接口无响应");
      yyQQRender("status");
    }
  } finally {
    yyQQStatusPolling = false;
  }
}

async function yyQQPollLogin(force) {  var r = await A("GET", "/api/qq/login");
  if (!r || !r.ok) return;
  // 2026-10-26 第 30 条：need_scan（掉线 >5 分钟）和 need_login 同等对待。
  // 之前只认 need_login，掉线拖到 need_scan 后前端就"什么都不提示"了。
  var needLike = function (lg) { return !!lg && (lg.state === "need_login" || lg.state === "need_scan"); };
  var wasNeed = needLike(yyQQC.login);
  var wasScan = yyQQC.login && yyQQC.login.state === "need_scan";
  yyQQC.login = r;
  yyQQC.qrBase = (r.qrcode && r.qrcode.age_seconds) || 0;
  yyQQC.qrFetchedAt = Date.now();

  var nowNeed = needLike(r);
  var nowScan = r.state === "need_scan";
  var newUrl = (r.qrcode && r.qrcode.data_url) || "";
  var img = document.querySelector("#yyq-qr img");

  // 二维码换了新图：只换 img.src，不重绘整块，避免闪烁。
  if (nowNeed && img && newUrl && newUrl !== yyQQC.lastQrUrl) {
    img.src = newUrl;
    yyQQC.lastQrUrl = newUrl;
  }
  // 掉线自动回到扫码界面：以前只在页面打开时显示一次二维码，机器人掉了
  // 用户还停在别的地方，只会看到"连接断开"却不知道要重新扫码。
  if (!wasScan && nowScan) {
    // 升级到"必须扫码"：这是用户明确要求的那一下提示，不能只靠 hero 变个颜色。
    T("机器人掉线超过 5 分钟，自动恢复没成功，请扫码重新登录", "e");
    setTimeout(function () {
      var box = document.getElementById("yyq-qr-box") || document.getElementById("yyq-qr");
      if (box && box.scrollIntoView) box.scrollIntoView({ block: "center", behavior: "smooth" });
    }, 120);
    // 没有二维码就直接让协议端重新出一张，别让用户对着空白框等。
    if (r.qr_missing && typeof yyQQQrRefresh === "function") yyQQQrRefresh(null);
  }
  if (wasNeed !== nowNeed && nowNeed) {
    T("机器人已掉线，需要重新扫码登录", "e");
    setTimeout(function () {
      var box = document.getElementById("yyq-qr-box") || document.getElementById("yyq-qr");
      if (box && box.scrollIntoView) box.scrollIntoView({ block: "center", behavior: "smooth" });
      // 二维码过期才重新生成，避免每次轮询都去重启协议端。
      if (r.qrcode && r.qrcode.stale && typeof yyQQQrRefresh === "function") yyQQQrRefresh(null);
    }, 120);
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
  return '<div class="yyq-root" id="yyq-root">' + yyQQShellHTML() + "</div>";
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
  // 模块折叠状态每次进页面重新按默认值算，避免上一次展开的「仅管理员」带着走。
  yyQQC.modOpen = {};
  yyQQC.modLoaded = {};
  // 缓存只在本次会话里有效（配置可能刚改过），已读位置和本地回声则跨刷新保留。
  yyQQC.msgCache = {};
  yyQQC.unread = {};
  yyQQC.stickBottom = 1;
  yyQQReadLoad();
  yyQQSentLoad();

  await yyQQLoadCore();
  await yyQQLoadMessages();
  if (yyQQC.login && yyQQC.login.state === "need_login") yyQQC.accessOpen = true;

  openSubPage("群服互通", yyQQConsoleBody());
  // 知识库与上下文、触发词规则、生图配置这三个模块都不改 console 本体，
  // 靠包装 yyQQRenderAll 往 .yyq-fold-grid 里插自己的折叠块。
  // 但上面这次首屏是 openSubPage 直接灌 HTML，根本没走 yyQQRenderAll，
  // 三个模块因此永远插不进来——用户看到的就是"RAG 和相关设置全没了"。
  // 这里补一次：此时 #yyq-root 已在 DOM 里，包装函数能把块注进去。
  yyQQRenderAll();
  yyQQSyncFolds();
  yyQQStartTimers();
  yyQQLoadProfile();
  // 首屏只拉了当前群的正文；其余群的未读角标靠这条汇总补，
  // 它不带正文，所以不会变成"一次把 8 个群的消息全拉一遍"。
  yyQQLoadSummary();
  if (yyQQC.login && yyQQC.login.state === "need_login") yyQQPollLogin(1);
}

// 覆盖旧版表单页
var yyQQConsoleBase = typeof yyQQPage === "function" ? yyQQPage : null;
yyQQPage = yyQQConsolePage;
