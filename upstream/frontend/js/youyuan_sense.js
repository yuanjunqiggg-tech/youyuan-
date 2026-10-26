var yySenseSettingsBase = Rcfg;
Rcfg = async function() {
  var yySettingsStillLive = yySettingsViewGuard();
  await yySenseSettingsBase();
  if (!yySettingsStillLive()) return;
  var host = document.querySelector("#tab-settings .content-inner");
  if (host && !document.getElementById("yy-settings-sense")) {
    var card = document.createElement("section");
    card.className = "card";
    card.id = "yy-settings-sense";
    card.innerHTML = yyCardHead("\u5B9E\u65F6\u611F\u77E5\u4E0E\u6293\u5305", "terminal") + '<p class="yy-hint">\u67E5\u770B\u5F53\u524D\u8D26\u53F7\u7684\u8840\u91CF\u3001\u6B7B\u4EA1\u4E0E\u4F4D\u7F6E\u89C2\u6D4B\uFF0C\u6309\u534F\u8BAE\u5B57\u6BB5\u7B5B\u9009\u6E38\u620F\u6570\u636E\u5305\u3002</p><div class="yy-actions"><button class="btn-s" onclick="yySensePage()">\u6253\u5F00\u89C2\u6D4B\u5DE5\u4F5C\u53F0</button><button class="btn-s" onclick="yyCallbackQueueSettings()">回调容量设置</button></div>';
    host.append(card);
  }
};
/* ── 实时感知与抓包：显示层整改（2026-10-26）─────────────────────────────
   用户报障三件事，都是这里造成的：

   1) 「点进去短时间内快速显示相关内容，像在刷 API」—— 原来 2500ms 一轮，
      每轮把 <tbody> 的 innerHTML 整体重写、把事件数组整体 JSON.stringify、
      把 12 条抓包详情全部重建。行节点被销毁重建，滚动位置、展开状态跟着跳。
   2) 「界面好像真是两套系统，有概率显示另一种」—— 抓包区模板里是
      <pre id="yy-capture-records">（原始 JSON 纯文本），第一次渲染时 yyCaptureRender
      又把它换成 .yy-capture-list 的 <details> 列表。两套实现并存，页面一重建就
      退回 pre 那套，于是「有时是列表、有时是一坨 JSON」。现在模板直接输出列表
      容器 + 常驻按钮行，PRE→DIV 的运行时替换整段删掉。
   3) 「打开这个界面会刷新一次，直接覆盖原有页面」—— yySensePage 每次都
      delete subPageCache 再 openSubPage 整页骨架，进去先白一下再重填。改成：
      骨架只在第一次建，之后走 openSubPage 的缓存分支复用已有 DOM。

   现在的刷新策略：
   - 进页面先一次性拉全量铺满（表 + 事件 + 抓包），不留白；
   - 之后每 4 秒只做增量：按 runtime_id 复用 <tr>，只改内容变了的单元格；
     事件用后端 cursor 判等，没变就不碰 DOM；抓包按 seq 增量插入/剔除；
   - 表格区加「暂停刷新 / 继续刷新」，暂停就是真停轮询（后台仍在记录）；
   - 标签页在后台（document.hidden）不拉接口。
   ─────────────────────────────────────────────────────────────────────── */
var yySenseTitle = "实时感知与抓包";
var yyCaptureDisplay = { paused: false, hidden: false, last: null, sig: "" };
var yySenseState = { rows: {}, miss: {}, eventsSig: "", paused: false, timer: 0, busy: false, lastScene: null };

function yySenseActorsEl() { return document.getElementById("yy-sense-actors"); }

// 子页可能被 openSubPage 用缓存字符串重建，DOM 节点会换新，索引必须按 data-key 重建。
function yySenseIndexRows() {
  var map = {}, body = yySenseActorsEl();
  if (body) {
    for (var i = 0; i < body.children.length; i++) {
      var k = body.children[i].getAttribute("data-key");
      if (k != null) map[k] = body.children[i];
    }
  }
  yySenseState.rows = map;
}

async function yySensePage() {
  var first = !subPageCache[yySenseTitle];
  if (first) {
    yyCaptureDisplay = { paused: false, hidden: false, last: null, sig: "" };
    yySenseState.rows = {}; yySenseState.miss = {}; yySenseState.eventsSig = ""; yySenseState.paused = false;
    openSubPage(yySenseTitle, yyIntro("实时感知与抓包", "当前账号独立记录。未知数据保持未知；方块更新不等于已确认某玩家破坏。") + '<section class="card"><div class="yy-actions"><button class="btn-s" onclick="yySenseRefresh(true)">刷新状态</button><button class="btn-s" id="yy-sense-pause" onclick="yySensePause(this)">暂停刷新</button>' + yySwitch("yy-sense-auto", "自动刷新", true) + '<button class="btn-s" onclick="yySenseSchema()">查看协议字段</button></div><p id="yy-sense-summary" class="yy-hint"></p><div class="yy-sense-table-wrap"><table class="yy-sense-table"><thead><tr><th>实体 / 玩家</th><th>位置</th><th>血量</th><th>状态 / 时间</th></tr></thead><tbody id="yy-sense-actors"></tbody></table></div></section><section class="card">' + yyCardHead("服务器视距与接包", "connection") + '<p class="yy-hint">请求半径与批准半径分别显示；实体广播范围由服务器控制，增大抓包容量不会产生服务器未下发的数据。</p><div class="yy-sense-fields"><label class="yy-field">请求区块半径<input id="yy-view-radius" type="number" min="1" max="2147483647" value="32"></label></div><div class="yy-actions"><button id="yy-view-request" class="btn" onclick="yyRequestViewDistance()">请求服务器视距</button></div><p id="yy-view-status" class="yy-hint"></p><details><summary>连接与广播范围详情</summary><pre id="yy-view-diagnostics" class="yy-readout"></pre></details></section><section class="card">' + yyCardHead("事件时间线", "list") + '<details><summary>展开最近 60 条事件</summary><pre id="yy-sense-events" class="yy-readout">等待观测数据</pre></details></section><section class="card">' + yyCardHead("账号游戏抓包", "terminal") + '<p class="yy-hint">解码后的服务器入站游戏包。不是手机全局 TCP/TLS 抓包。记录脱界；星号默认不包含大区块包，认证握手不记录。</p><label class="yy-field">包类型（英文逗号分隔，可用 *）<input id="yy-capture-types" value="UpdateAttributes,ActorEvent,Animate,MovePlayer,UpdateBlock,Respawn"></label><div class="yy-sense-fields"><label class="yy-field">缓冲条数<input id="yy-capture-capacity" type="number" min="1" max="65536" value="256"></label><label class="yy-field">同类包抽样间隔 / 毫秒<input id="yy-capture-sample" type="number" min="0" max="60000" value="0"></label></div><label class="yy-field">抓包留存预算 / MiB<input id="yy-capture-budget" type="number" min="1" max="256" value="16"></label><label class="yy-field">字段等值过滤 / JSON<textarea id="yy-capture-filter" rows="3" placeholder=\'{"entity_runtime_id":42}\'>{}</textarea></label><div class="yy-actions"><button class="btn" onclick="yyCaptureAction(\'start\')">开始记录</button><button class="btn-s" onclick="yyCaptureAction(\'stop\')">停止</button><button class="btn-s" onclick="yyCaptureAction(\'clear\')">清空</button><button class="btn-s" onclick="yyCaptureExport()">导出 NDJSON</button></div><div class="yy-actions"><button id="yy-capture-pause" class="btn-s" onclick="yyCaptureView(\'pause\')">暂停显示</button><button class="btn-s" onclick="yyCaptureView(\'collapse\')">全部折叠</button><button class="btn-s" onclick="yyCaptureView(\'expand\')">全部展开</button><button id="yy-capture-hide" class="btn-s" onclick="yyCaptureView(\'hide\')">隐藏内容</button></div><p id="yy-capture-status" class="yy-hint"></p><div id="yy-capture-records" class="yy-capture-list"></div></section><section class="card">' + yyCardHead("宿主观测容量", "settings") + '<p class="yy-hint">回调队列容量可按需调整，不是可识别的包种类型数量。保存后，事件队列在重载插件时生效；其他设置用于后续观测。不会给Fast命令增加CPS限速。</p><div class="yy-sense-fields"><label class="yy-field">Lua普通事件队列<input id="yy-limit-event_queue" type="number" min="1" max="65536"></label><label class="yy-field">Lua战斗事件队列<input id="yy-limit-priority_queue" type="number" min="1" max="65536"></label><label class="yy-field">Python / C++事件队列<input id="yy-limit-runtime_queue" type="number" min="1" max="65536"></label><label class="yy-field">异步命令结果<input id="yy-limit-async_tickets" type="number" min="1" max="65536"></label><label class="yy-field">事件历史条数<input id="yy-limit-event_history" type="number" min="1" max="65536"></label><label class="yy-field">实体缓存条数<input id="yy-limit-actors" type="number" min="1" max="65536"></label><label class="yy-field">观测过期 / 毫秒<input id="yy-limit-stale_ms" type="number" min="1" max="86400000"></label></div><div class="yy-actions"><button class="btn" onclick="yyObservationSave()">保存容量</button><button class="btn-s" onclick="yyObservationLoad()">读取当前配置</button></div><p id="yy-limit-status" class="yy-hint"></p></section>');
  } else {
    // 缓存命中分支：openSubPage 直接复用上次的 DOM 快照，不重建骨架、不闪白。
    openSubPage(yySenseTitle, "");
  }
  yySenseIndexRows();
  if (first) yyObservationLoad();   // 只回填一次，避免覆盖用户正在编辑的容量输入
  await yySenseRefresh(true);
  yySensePoll();
}

function yySensePoll() {
  clearTimeout(yySenseState.timer);
  yySenseState.timer = setTimeout(async function () {
    if (!yySenseActorsEl()) return;                       // 页面已关，停表
    var auto = document.getElementById("yy-sense-auto");
    if (!yySenseState.paused && !document.hidden && (!auto || auto.checked)) await yySenseRefresh(false);
    yySensePoll();
  }, 4000);
}

function yySensePause(btn) {
  yySenseState.paused = !yySenseState.paused;
  if (btn) btn.textContent = yySenseState.paused ? "继续刷新" : "暂停刷新";
  if (yySenseState.paused) { clearTimeout(yySenseState.timer); yySenseState.timer = 0; }
  else { yySenseRefresh(false); yySensePoll(); }
  yySenseDrawScene(yySenseState.lastScene || {});
}

async function yySenseRefresh(full) {
  if (yySenseState.busy) return;
  yySenseState.busy = true;
  try {
    var jobs = [
      A("POST", "/api/youyuan/game/call", { method: "game.scene", args: [{ present_only: true }] }),
      A("GET", "/api/youyuan/sense/events"),
      A("GET", "/api/youyuan/packets/capture")
    ];
    // 视距/可见性诊断是慢接口，只在首次进入或手动刷新时拉，不跟着 4 秒轮询跑。
    if (full) jobs.push(A("POST", "/api/youyuan/game/call", { method: "diagnostics.visibility", args: [] }));
    var r = await Promise.all(jobs);
    if (!yySenseActorsEl()) return;
    yySenseDrawScene(r[0].result || {});
    yySenseDrawEvents(r[1]);
    yyCaptureRender(r[2]);
    if (full && r[3]) {
      var view = document.getElementById("yy-view-diagnostics");
      if (view) view.textContent = JSON.stringify(r[3].result || r[3], null, 2);
    }
  } catch (e) {
    T(e.message, "e");
  } finally {
    yySenseState.busy = false;
  }
}

function yySenseCell(td, text) {
  var t = String(text == null ? "" : text);
  if (td && td.textContent !== t) td.textContent = t;
}

// 增量渲染实体表：节点按 runtime_id 复用，只改变化的单元格，不整表重排。
function yySenseDrawScene(scene) {
  if (!scene) return;
  yySenseState.lastScene = scene;
  var summary = document.getElementById("yy-sense-summary");
  if (summary) {
    var text = (scene.connected ? "游戏已连接" : "游戏未连接") + " · 当前账号 " + (window.yyActiveAccount || "main") + " · 仅显示本机器人收到的服务器观测" + (yySenseState.paused ? " · 已暂停刷新" : "");
    if (summary.textContent !== text) summary.textContent = text;
  }
  var body = yySenseActorsEl();
  if (!body) return;
  var rows = scene.actors || [];
  var seen = {}, next = {};
  for (var i = 0; i < rows.length; i++) {
    var a = rows[i];
    var key = String(a.runtime_id != null ? a.runtime_id : (a.name || a.entity_type || i));
    if (seen[key]) continue;
    seen[key] = 1;
    var tr = yySenseState.rows[key];
    if (!tr) {
      tr = document.createElement("tr");
      tr.setAttribute("data-key", key);
      for (var c = 0; c < 4; c++) tr.appendChild(document.createElement("td"));
      body.appendChild(tr);
    }
    next[key] = tr;
    var pos = a.position_known ? (a.position || []).map(function (x) { return Number(x).toFixed(2); }).join(", ") + " / 维度 " + a.dimension : "未知";
    var health = a.health_known ? String(a.health) + (a.max_health != null ? " / " + a.max_health : "") : "未知";
    var state = a.dead === true ? "已死亡" : a.dead === false ? "存活" : "未知";
    yySenseCell(tr.children[0], a.name || a.entity_type || a.runtime_id);
    yySenseCell(tr.children[1], pos);
    yySenseCell(tr.children[2], health);
    yySenseCell(tr.children[3], state + (a.stale ? " · 旧观测" : "") + " / " + (a.last_seen || ""));
    delete yySenseState.miss[key];
  }
  // 消失的实体：连续两轮缺席才删行，避免 present_only 抖动把表格删了又加。
  var keys = Object.keys(yySenseState.rows);
  for (var j = 0; j < keys.length; j++) {
    var k = keys[j];
    if (seen[k]) continue;
    yySenseState.miss[k] = (yySenseState.miss[k] || 0) + 1;
    if (yySenseState.miss[k] >= 2) {
      var gone = yySenseState.rows[k];
      if (gone && gone.parentNode) gone.parentNode.removeChild(gone);
      delete yySenseState.miss[k];
    } else {
      next[k] = yySenseState.rows[k];
    }
  }
  yySenseState.rows = next;
  var empty = body.querySelector("tr[data-empty]");
  if (!Object.keys(next).length) {
    if (!empty) {
      var er = document.createElement("tr");
      er.setAttribute("data-empty", "1");
      var td = document.createElement("td");
      td.setAttribute("colspan", "4");
      td.textContent = "尚未收到服务器实体状态；进服后才会出现真实观测。";
      er.appendChild(td);
      body.appendChild(er);
    }
  } else if (empty) {
    empty.remove();
  }
}

// 事件时间线：用后端 cursor 判等，没新事件就一个字节都不写 DOM。
function yySenseDrawEvents(r) {
  var el = document.getElementById("yy-sense-events");
  if (!el) return;
  var evs = (r && r.events) || [];
  var sig = String(r && r.cursor != null ? r.cursor : "") + "|" + evs.length;
  if (sig === yySenseState.eventsSig) return;
  yySenseState.eventsSig = sig;
  var pick = evs.slice(-60);
  el.textContent = pick.length ? pick.map(function (e) {
    var at = String(e.at || e.time || e.timestamp || "");
    if (at.length > 19) at = at.slice(11, 19);
    var head = e.method || e.type || e.name || e.kind || e.event || "事件";
    var who = e.entity_type || e.player || "";
    var detail = e.detail || e.message || e.summary || "";
    return at + "  " + head + (who ? " · " + who : "") + (detail ? " · " + detail : "");
  }).join("\n") : "等待观测数据";
}

function yyCaptureView(action) {
  var host = document.getElementById("yy-capture-records");
  if (!host) return;
  if (action === "pause") {
    yyCaptureDisplay.paused = !yyCaptureDisplay.paused;
    var b = document.getElementById("yy-capture-pause");
    if (b) b.textContent = yyCaptureDisplay.paused ? "继续显示" : "暂停显示";
  }
  if (action === "hide") {
    yyCaptureDisplay.hidden = !yyCaptureDisplay.hidden;
    host.hidden = yyCaptureDisplay.hidden;
    var hb = document.getElementById("yy-capture-hide");
    if (hb) hb.textContent = yyCaptureDisplay.hidden ? "显示内容" : "隐藏内容";
  }
  if (action === "collapse" || action === "expand") host.querySelectorAll("details").forEach(function (d) { d.open = action === "expand"; });
  if (!yyCaptureDisplay.paused && !yyCaptureDisplay.hidden && yyCaptureDisplay.last) {
    yyCaptureDisplay.sig = "";
    yyCaptureRender(yyCaptureDisplay.last);
  }
}

// 抓包列表：单一实现（列表），按 seq 增量插入/剔除；没有新包时完全不动 DOM。
function yyCaptureRender(r) {
  yyCaptureDisplay.last = r;
  var status = document.getElementById("yy-capture-status");
  var host = document.getElementById("yy-capture-records");
  if (status) {
    var st = (r.active ? r.waiting_for_connection ? "监听已开启，等待进服" : "正在记录" : "已停止") + " · 留存 " + (r.retained_count || 0) + " · 覆盖 " + (r.overwritten || 0) + " · 抽样跳过 " + (r.sampled_out || 0) + (yyCaptureDisplay.paused ? " · 显示已暂停，后台仍记录" : "");
    if (status.textContent !== st) status.textContent = st;
  }
  if (!host) return;
  host.hidden = yyCaptureDisplay.hidden;
  if (yyCaptureDisplay.paused || yyCaptureDisplay.hidden) return;
  host.style.maxHeight = "440px";
  host.style.overflow = "auto";
  host.style.overflowAnchor = "none";
  var rows = (r.records || []).slice(-12).reverse();
  var keys = rows.map(function (p) {
    var k = p.seq != null ? p.seq : p.sequence != null ? p.sequence : p.timestamp != null ? p.timestamp : p.time != null ? p.time : p.at;
    return String(k);
  });
  var sig = keys.join(",");
  if (sig === yyCaptureDisplay.sig) return;   // 没有新包：DOM 一个字节都不动
  yyCaptureDisplay.sig = sig;
  if (!rows.length) { host.innerHTML = '<p class="yy-hint">暂无记录</p>'; return; }
  var hint = host.querySelector("p.yy-hint");
  if (hint) hint.remove();
  var existing = {};
  Array.prototype.forEach.call(host.querySelectorAll("details[data-seq]"), function (d) { existing[d.getAttribute("data-seq")] = d; });
  var keep = {}, scroll = host.scrollTop;
  for (var i = 0; i < rows.length; i++) {
    var p = rows[i], key = keys[i];
    keep[key] = 1;
    var node = existing[key];
    if (!node) {
      node = document.createElement("details");
      node.setAttribute("data-seq", key);
      var sum = document.createElement("summary");
      sum.style.padding = "12px";
      sum.style.cursor = "pointer";
      sum.textContent = (p.packet_type || p.type || "游戏包") + " · " + (p.at || p.timestamp || p.time || "") + " · #" + key;
      var pre = document.createElement("pre");
      pre.className = "yy-readout";
      pre.style.maxHeight = "300px";
      pre.style.overflow = "auto";
      var raw = JSON.stringify(p, null, 2);
      pre.textContent = raw.length > 12000 ? raw.slice(0, 12000) + "\n…预览已截短；导出 NDJSON 可读取完整记录。" : raw;
      node.appendChild(sum);
      node.appendChild(pre);
    }
    var at = host.children[i];
    if (at !== node) host.insertBefore(node, at || null);
  }
  Object.keys(existing).forEach(function (k) { if (!keep[k]) existing[k].remove(); });
  host.scrollTop = scroll;
}
async function yyCaptureAction(action) {
  try {
    var body = { action };
    if (action === "start") {
      body.config = { types: document.getElementById("yy-capture-types").value.split(",").map((x) => x.trim()).filter(Boolean), capacity: Number(document.getElementById("yy-capture-capacity").value), total_bytes: Number(document.getElementById("yy-capture-budget").value) * 1024 * 1024, filter: { every_ms: Number(document.getElementById("yy-capture-sample").value), fields: JSON.parse(document.getElementById("yy-capture-filter").value) } };
    }
    var r = await A("POST", "/api/youyuan/packets/capture", body);
    if (!r.ok) {
      T(r.error, "e");
      return;
    }
    yyCaptureRender(r);
  } catch (e) {
    T(e.message, "e");
  }
}
async function yySenseSchema() {
  var r = await A("GET", "/api/youyuan/packets/schema");
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  var packets = r.packets || [];
  delete subPageCache["\u534F\u8BAE\u5B57\u6BB5"];
  openSubPage("\u534F\u8BAE\u5B57\u6BB5", yyIntro("\u534F\u8BAE\u5B57\u6BB5", "\u4ECE\u5F53\u524D\u5185\u6838\u751F\u6210\u7684\u5305\u540D\u3001\u534F\u8BAE ID \u548C\u5B57\u6BB5\u7C7B\u578B\u3002") + '<section class="card"><input id="yy-schema-query" placeholder="\u641C\u7D22\u5305\u540D\u6216\u5B57\u6BB5\uFF0C\u4F8B\u5982 health / UpdateAttributes"><pre id="yy-schema-result" class="yy-readout"></pre></section>');
  var input = document.getElementById("yy-schema-query");
  input.oninput = function() {
    var q = this.value.toLowerCase();
    document.getElementById("yy-schema-result").textContent = JSON.stringify(packets.filter((p) => !q || JSON.stringify(p).toLowerCase().includes(q)), null, 2);
  };
  input.value = "UpdateAttributes";
  input.oninput();
}
async function yyCaptureExport() {
  try {
    if (window.android && typeof android.exportFile === "function") {
      var r = await A("POST", "/api/youyuan/packets/export", {});
      if (!r.ok) {
        T(r.error, "e");
        return;
      }
      android.exportFile(r.path, r.filename);
      return;
    }
    var path = typeof yyAccountPath === "function" ? yyAccountPath("/api/youyuan/packets/export", window.yyActiveAccount || "main") : "/api/youyuan/packets/export";
    var response = await yyRootFetch(path);
    if (!response.ok) throw Error("\u6293\u5305\u5BFC\u51FA\u5931\u8D25");
    var url = URL.createObjectURL(await response.blob()), link = document.createElement("a");
    link.href = url;
    link.download = "youyuan-packets.ndjson";
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1e4);
  } catch (e) {
    T(e.message, "e");
  }
}
async function yyObservationLoad() {
  try {
    var r = await A("POST", "/api/youyuan/game/call", { method: "diagnostics.configure", args: [] });
    if (!r.ok) throw Error(r.error);
    var cfg = r.result.settings;
    Object.keys(cfg).forEach((k) => {
      var input = document.getElementById("yy-limit-" + k);
      if (input) input.value = cfg[k];
    });
    document.getElementById("yy-limit-status").textContent = "\u914D\u7F6E\u6765\u81EA\u5F53\u524D\u5BBF\u4E3B\uFF0C\u53EF\u8BC6\u522B\u7684\u5305\u79CD\u7C7B\u8BF7\u67E5\u770B\u534F\u8BAE\u5B57\u6BB5\u3002";
  } catch (e) {
    T(e.message, "e");
  }
}
async function yyObservationSave() {
  try {
    var cfg = {};
    ["event_queue", "priority_queue", "runtime_queue", "async_tickets", "event_history", "actors", "stale_ms"].forEach((k) => {
      var n = Number(document.getElementById("yy-limit-" + k).value);
      if (!Number.isInteger(n) || n < 1) throw Error("\u5BB9\u91CF\u9700\u6B63\u6574\u6570");
      cfg[k] = n;
    });
    var r = await A("POST", "/api/youyuan/game/call", { method: "diagnostics.configure", args: [cfg] });
    if (!r.ok) throw Error(r.error);
    document.getElementById("yy-limit-status").textContent = "\u5DF2\u4FDD\u5B58\u3002\u4E8B\u4EF6\u961F\u5217\u5BB9\u91CF\u5728\u91CD\u8F7D\u63D2\u4EF6\u540E\u751F\u6548\u3002";
    T("\u5BB9\u91CF\u5DF2\u4FDD\u5B58");
  } catch (e) {
    T(e.message, "e");
  }
}
async function yyRequestViewDistance() {
  var button = document.getElementById("yy-view-request"), status = document.getElementById("yy-view-status");
  try {
    var chunks = Number(document.getElementById("yy-view-radius").value);
    if (!Number.isInteger(chunks) || chunks < 1 || chunks > 2147483647) throw Error("\u8BF7\u8F93\u5165\u6B63int32\u533A\u5757\u534A\u5F84");
    button.disabled = true;
    status.textContent = "\u7B49\u5F85\u670D\u52A1\u5668\u6279\u51C6\u503C\u2026";
    var r = await A("POST", "/api/youyuan/game/call", { method: "bot.requestViewDistance", args: [chunks, 5e3] });
    if (!r.ok) throw Error(r.error);
    var v = r.result;
    status.textContent = v.confirmed ? "\u8BF7\u6C42 " + chunks + " \u533A\u5757\uFF0C\u670D\u52A1\u5668\u6279\u51C6 " + v.granted_radius_chunks + " \u533A\u5757\uFF1B\u5B9E\u4F53\u5E7F\u64AD\u8303\u56F4\u4ECD\u7531\u670D\u52A1\u5668\u51B3\u5B9A\u3002" : "\u5DF2\u63D0\u4EA4\uFF0C\u4F46\u672A\u6536\u5230\u534A\u5F84\u56DE\u5305\uFF0C\u751F\u6548\u60C5\u51B5\u672A\u77E5\u3002";
    await yySenseRefresh();
  } catch (e) {
    if (status) status.textContent = e.message;
    T(e.message, "e");
  } finally {
    if (button) button.disabled = false;
  }
}
async function yyObserverCooperationPage() {
  var r = await yyRootAPI("GET", "/api/youyuan/accounts");
  if (!r.ok) {
    T(r.error || "\u8D26\u53F7\u6C60\u8BFB\u53D6\u5931\u8D25", "e");
    return;
  }
  var rows = [{ id: "main", name: "\u4E3B\u8D26\u53F7" }].concat((r.accounts || []).filter((a) => a.id !== "main"));
  var options = rows.map((a) => '<option value="' + yyEscape(a.id) + '">' + yyEscape(a.name || a.id) + "</option>").join("");
  delete subPageCache["\u89C2\u6D4B\u534F\u4F5C"];
  openSubPage("\u89C2\u6D4B\u534F\u4F5C", yyIntro("\u89C2\u6D4B\u534F\u4F5C", "\u6BCF\u4E2A\u8D26\u53F7\u72EC\u7ACB\u63A5\u5305\uFF0C\u4E8B\u4EF6\u6C47\u603B\u4FDD\u7559\u89C2\u5BDF\u8005\u4E0E\u65F6\u95F4\u300248\u683C\u4EC5\u4F5C\u53C2\u8003\uFF0C\u6309\u5B9E\u9645\u4E0B\u53D1\u5224\u65AD\u8986\u76D6\u3002") + '<section class="card">' + yyCardHead("\u5F53\u524D\u8D26\u53F7\u5E38\u9A7B\u89C2\u5BDF", "connection") + `<div class="yy-sense-fields"><label class="yy-field">\u76EE\u6807\u6E38\u620F\u540D<input id="yy-watch-player" placeholder="\u51C6\u786E\u7684\u5728\u7EBF\u73A9\u5BB6\u540D"></label><label class="yy-field">\u89C2\u5BDF\u65B9\u5F0F<select id="yy-watch-mode"><option value="hybrid">\u8FD1\u8DDD\u8DDF\u968F + \u76F8\u673A</option><option value="camera">\u76F8\u673A\u89C2\u5BDF</option><option value="follow">\u8FD1\u8DDD\u8DDF\u968F</option></select></label><label class="yy-field">\u8DDF\u8E2A\u95F4\u9694 / \u79D2<input id="yy-watch-interval" type="number" step="any" value="2"></label><label class="yy-field">\u8EAB\u4F53\u8DDF\u968F\u534A\u5F84 / \u683C<input id="yy-watch-radius" type="number" min="0" max="4096" step="any" value="16"></label></div><div class="yy-actions"><button class="btn" onclick="yyWatchUI('Start')">\u5F00\u59CB\u89C2\u5BDF</button><button class="btn-s" onclick="yyWatchUI('Status')">\u8BFB\u53D6\u72B6\u6001</button><button class="btn-s" onclick="yyWatchUI('Stop')">\u505C\u6B62\u5E76\u6062\u590D</button><button class="btn-s" onclick="yyWatchPluginCode()">\u590D\u5236\u63D2\u4EF6\u8C03\u7528\u4EE3\u7801</button></div><div class="yy-sense-fields"><label class="yy-field">\u955C\u5934\u9AD8\u5DEE / \u683C<input id="yy-watch-height" type="number" step="any" value="4"></label><label class="yy-field">\u955C\u5934\u4FEF\u4EF0 / \u53EF\u7559\u7A7A<input id="yy-watch-pitch" type="number" min="-90" max="90" step="any" value="90" placeholder="90\u5411\u4E0B\uFF0C-90\u5411\u4E0A"></label><label class="yy-field">\u955C\u5934\u671D\u5411<input id="yy-watch-yaw" type="number" step="any" value="0"></label><label><input id="yy-watch-facing" type="checkbox">\u6BCF\u6B21\u66F4\u65B0\u65F6\u671D\u5411\u76EE\u6807\uFF08\u4FEF\u4EF0\u7559\u7A7A\uFF09</label></div><p class="yy-hint">\u53EA\u79FB\u52A8\u5F53\u524D\u673A\u5668\u4EBA\uFF0C\u4E0D\u79FB\u52A8\u73A9\u5BB6\u3002\u9700\u8981 OP\uFF1B\u76F8\u673A\u5355\u72EC\u4F7F\u7528\u4E0D\u80FD\u8DE8\u7EF4\u5EA6\u3002\u95F4\u9694\u662F\u76EE\u6807\u503C\uFF0C\u6162\u67E5\u8BE2\u5408\u5E76\uFF0C\u4E0D\u5806\u53E0\u3002\u534A\u5F84\u9ED8\u8BA416\u683C\uFF0C\u9644\u8FD1\u4FDD\u6301\u9A7B\u7559\uFF1B0\u6BCF\u8F6E\u8BF7\u6C42TP\u3002</p></section><section class="card">` + yyCardHead("\u63D2\u4EF6\u6301\u7EED\u89C2\u6D4B\u534F\u4F5C", "connection") + `<p class="yy-hint">\u8C03\u7528\u591A\u529F\u80FD\u63D2\u4EF6\u7684\u771F\u5B9E\u534F\u4F5C\u6A21\u5757\uFF1B\u5F00\u59CB\u524D\u9009\u62E9\u4E0B\u65B9\u8D26\u53F7\u3002\u65E0\u6548\u4EE4\u724C\u3001\u672A\u8FDE\u63A5\u548C\u4EA4\u63A5\u5931\u8D25\u4F1A\u663E\u793A\u5B9E\u9645\u9519\u8BEF\u3002</p><label class="yy-field">\u8FD0\u884C\u4E2D\u7684\u63D2\u4EF6 ID<input id="yy-team-plugin" value="ds_ai_agent_r21b_test"></label><label class="yy-field">\u76EE\u6807\u73A9\u5BB6\uFF08\u6BCF\u884C\u4E00\u4E2A\uFF0C\u7559\u7A7A\u4E3A\u5728\u7EBF\u771F\u4EBA\uFF09<textarea id="yy-team-targets" rows="3"></textarea></label><label class="yy-field">\u52A8\u4F5C\u4E8B\u4EF6\u8FD4\u56DE\u6761\u6570<input id="yy-team-limit" type="number" min="1" max="4096" value="1024"></label><div class="yy-actions"><button class="btn" onclick="yyTeamUI('Configure')">\u542F\u52A8\u9009\u4E2D\u8D26\u53F7\u534F\u4F5C</button><button class="btn-s" onclick="yyTeamUI('Status')">\u8BFB\u53D6\u534F\u4F5C\u72B6\u6001</button><button class="btn-s" onclick="yyTeamUI('Stop')">\u505C\u6B62\u534F\u4F5C</button></div></section><section class="card">` + yyCardHead("\u9519\u5CF0\u89C4\u5212", "list") + '<p class="yy-hint">\u9009\u62E9\u81EA\u5DF1\u7684\u89C2\u5BDF\u8D26\u53F7\uFF1B\u63A8\u83506\u4E2A\uFF0C\u4FDD\u75591\u4E2A\u4F5C\u4E3A\u4EA4\u63A5\u8865\u4F4D\u3002\u89C4\u5212\u9ED8\u8BA4\u6392\u9664\u5DF2\u77E5\u673A\u5668\u4EBA\uFF0C\u4E0D\u4F1A\u81EA\u52A8\u767B\u5F55\u6216\u8FD0\u884C\u63D2\u4EF6\u3002</p><div class="yy-sense-fields">' + rows.map((a) => '<label><input class="yy-plan-account" type="checkbox" value="' + yyEscape(a.id) + '"> ' + yyEscape(a.name || a.id) + "</label>").join("") + '</div><label class="yy-field">\u6BCF\u76EE\u6807\u9A7B\u7559 / \u79D2<input id="yy-plan-dwell" type="number" step="any" value="2"></label><button class="btn-s" onclick="yyObserverPlanUI()">\u751F\u6210\u9519\u5CF0\u89C4\u5212</button><div id="yy-plan-routes" class="yy-sense-table-wrap"></div></section><section class="card">' + yyCardHead("\u5148\u63A5\u66FF\uFF0C\u518D\u79BB\u5F00", "connection") + '<div class="yy-sense-fields"><label class="yy-field">\u63A5\u66FF\u8D26\u53F7<select id="yy-handoff-in">' + options + '</select></label><label class="yy-field">\u539F\u89C2\u5BDF\u8D26\u53F7<select id="yy-handoff-out">' + options + '</select></label><label class="yy-field">\u539F\u8D26\u53F7\u4E0B\u4E00\u76EE\u6807 / \u53EF\u7A7A<input id="yy-handoff-next"></label></div><button class="btn" onclick="yyObserverHandoffUI()">\u6267\u884C\u4EA4\u63A5</button><p class="yy-hint">\u5148\u89C2\u5BDF\u4E0A\u65B9\u540C\u4E00\u76EE\u6807\uFF0C\u6536\u5230\u5339\u914D\u7684\u65B0\u533A\u57DF\u53D1\u5E03\u624D\u91CA\u653E\u539F\u89C2\u5BDF\u3002\u8D85\u65F6\u4FDD\u7559\u539F\u89C2\u5BDF\uFF1B\u533A\u57DF\u53D1\u5E03\u4ECD\u4E0D\u7B49\u4E8E\u6240\u6709\u5B9E\u4F53\u5305\u5DF2\u540C\u6B65\u3002</p></section><section class="card"><pre id="yy-watch-output" class="yy-readout" aria-live="polite"></pre></section>');
}
function yyWatchUIOptions() {
  return { player: document.getElementById("yy-watch-player").value.trim(), mode: document.getElementById("yy-watch-mode").value, interval_seconds: Number(document.getElementById("yy-watch-interval").value), follow_radius_blocks: Number(document.getElementById("yy-watch-radius").value), camera_height_blocks: Number(document.getElementById("yy-watch-height").value), camera_pitch_degrees: document.getElementById("yy-watch-pitch").value.trim() === "" ? void 0 : Number(document.getElementById("yy-watch-pitch").value), camera_yaw_degrees: Number(document.getElementById("yy-watch-yaw").value), camera_facing_player: document.getElementById("yy-watch-facing").checked, restore_position: true };
}
function yyWatchPluginCode() {
  var o = yyWatchUIOptions(), raw = JSON.stringify(o);
  var code = "-- \u5BBF\u4E3B\u539F\u751F\u89C2\u5BDF\uFF0C\u53C2\u6570\u7531\u63D2\u4EF6\u51B3\u5B9A\uFF1B\u4E0D\u4F9D\u8D56\u591A\u529F\u80FD\u63D2\u4EF6\u6A21\u5757\nlocal options = game.jsonDecode(" + JSON.stringify(raw) + ')\nobserverBridge.watchStart(options)\n-- \u8DE8\u8D26\u53F7\uFF1AaccountsPool.invoke("\u81EA\u5DF1\u7684\u6C60\u8D26\u53F7ID", "observerBridge.watchStart", {options})\n-- \u67E5\u8BE2\uFF1AobserverBridge.watchStatus()\n-- \u505C\u6B62\uFF1AobserverBridge.watchStop()\n';
  document.getElementById("yy-watch-output").textContent = code;
  if (navigator.clipboard) navigator.clipboard.writeText(code).then(() => T("\u63D2\u4EF6\u8C03\u7528\u4EE3\u7801\u5DF2\u590D\u5236", "s")).catch(() => T("\u4EE3\u7801\u5DF2\u663E\u793A\uFF0C\u53EF\u624B\u52A8\u590D\u5236"));
}
async function yyWatchUI(action) {
  try {
    var r = await A("POST", "/api/youyuan/game/call", { method: "observerBridge.watch" + action, args: action === "Start" ? [yyWatchUIOptions()] : [] });
    if (!r.ok) throw Error(r.error);
    document.getElementById("yy-watch-output").textContent = JSON.stringify(r.result, null, 2);
  } catch (e) {
    T(e.message, "e");
  }
}
async function yyObserverPlanUI() {
  try {
    var ids = Array.from(document.querySelectorAll(".yy-plan-account:checked")).map((e) => e.value);
    var r = await yyRootAPI("POST", "/api/youyuan/account-pool", { action: "observationPlan", args: [{ accounts: ids, dwell_seconds: Number(document.getElementById("yy-plan-dwell").value) }] });
    if (!r.ok) throw Error(r.error);
    var p = r.result;
    document.getElementById("yy-plan-routes").innerHTML = '<table class="yy-sense-table"><thead><tr><th>\u8D26\u53F7</th><th>\u4EFB\u52A1</th><th>\u9519\u5CF0 / \u79D2</th></tr></thead><tbody>' + p.assignments.map((a) => "<tr><td>" + yyEscape(yyAccountName(a.account)) + "</td><td>" + yyEscape(a.handoff_reserve ? "\u4EA4\u63A5\u8865\u4F4D" : a.players.join(" \u2192 ") || "\u7A7A\u95F2") + "</td><td>" + Number(a.phase_seconds).toFixed(3) + "</td></tr>").join("") + '</tbody></table><p class="yy-hint">\u6392\u9664\u673A\u5668\u4EBA\uFF1A' + yyEscape((p.excluded_bots || []).join("\u3001") || "\u65E0\u5DF2\u77E5\u5728\u7EBF\u673A\u5668\u4EBA") + "</p>";
    document.getElementById("yy-watch-output").textContent = JSON.stringify(p, null, 2);
  } catch (e) {
    T(e.message, "e");
  }
}
async function yyObserverHandoffUI() {
  try {
    var cfg = yyWatchUIOptions();
    cfg.next_player = document.getElementById("yy-handoff-next").value.trim();
    cfg.timeout_seconds = 5;
    var r = await yyRootAPI("POST", "/api/youyuan/account-pool", { action: "handoff", args: [document.getElementById("yy-handoff-in").value, document.getElementById("yy-handoff-out").value, cfg] });
    if (!r.ok) throw Error(r.error);
    document.getElementById("yy-watch-output").textContent = JSON.stringify(r.result, null, 2);
  } catch (e) {
    T(e.message, "e");
  }
}
async function yyTeamUI(action) {
  try {
    var pid = document.getElementById("yy-team-plugin").value.trim();
    var args = [];
    if (action === "Configure") {
      var ids = Array.from(document.querySelectorAll(".yy-plan-account:checked")).map((e) => e.value);
      if (!ids.length) throw Error("\u8BF7\u5148\u9009\u62E9\u4E0B\u65B9\u89C2\u5BDF\u8D26\u53F7");
      var o = yyWatchUIOptions();
      delete o.player;
      delete o.camera_facing_player;
      o.enabled = true;
      o.accounts = ids;
      o.targets = document.getElementById("yy-team-targets").value.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
      o.start_accounts = true;
      o.event_limit = Number(document.getElementById("yy-team-limit").value);
      o.dwell_seconds = Number(document.getElementById("yy-plan-dwell").value);
      o.camera_height_blocks = Number(document.getElementById("yy-watch-height").value);
      o.camera_pitch_degrees = document.getElementById("yy-watch-pitch").value.trim() === "" ? 90 : Number(document.getElementById("yy-watch-pitch").value);
      args = [o];
    }
    var r = await A("POST", "/api/youyuan/game/call", { method: "plugin.call", args: [pid, "observationTeam" + action, args] });
    if (!r.ok) throw Error(r.error);
    document.getElementById("yy-watch-output").textContent = JSON.stringify(r.result, null, 2);
  } catch (e) {
    T(e.message, "e");
  }
}

async function yyCallbackQueueSettings() {
  await yySensePage();
  document.getElementById("yy-limit-event_queue")?.closest(".card")?.scrollIntoView({block:"start"});
}
