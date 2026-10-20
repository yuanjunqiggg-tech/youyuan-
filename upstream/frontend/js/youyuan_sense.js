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
    card.innerHTML = yyCardHead("\u5B9E\u65F6\u611F\u77E5\u4E0E\u6293\u5305", "terminal") + '<p class="yy-hint">\u67E5\u770B\u5F53\u524D\u8D26\u53F7\u7684\u8840\u91CF\u3001\u6B7B\u4EA1\u4E0E\u4F4D\u7F6E\u89C2\u6D4B\uFF0C\u6309\u534F\u8BAE\u5B57\u6BB5\u7B5B\u9009\u6E38\u620F\u6570\u636E\u5305\u3002</p><div class="yy-actions"><button class="btn-s" onclick="yySensePage()">\u6253\u5F00\u89C2\u6D4B\u5DE5\u4F5C\u53F0</button></div>';
    host.append(card);
  }
};
async function yySensePage() {
  yyCaptureDisplay = { paused: false, hidden: false, last: null };
  delete subPageCache["\u5B9E\u65F6\u611F\u77E5\u4E0E\u6293\u5305"];
  openSubPage("\u5B9E\u65F6\u611F\u77E5\u4E0E\u6293\u5305", yyIntro("\u5B9E\u65F6\u611F\u77E5\u4E0E\u6293\u5305", "\u5F53\u524D\u8D26\u53F7\u72EC\u7ACB\u8BB0\u5F55\u3002\u672A\u77E5\u6570\u636E\u4FDD\u6301\u672A\u77E5\uFF1B\u65B9\u5757\u66F4\u65B0\u4E0D\u7B49\u4E8E\u5DF2\u786E\u8BA4\u67D0\u73A9\u5BB6\u7834\u574F\u3002") + '<section class="card"><div class="yy-actions"><button class="btn-s" onclick="yySenseRefresh()">\u5237\u65B0\u72B6\u6001</button>' + yySwitch("yy-sense-auto", "\u81EA\u52A8\u5237\u65B0", true) + '<button class="btn-s" onclick="yySenseSchema()">\u67E5\u770B\u534F\u8BAE\u5B57\u6BB5</button></div><p id="yy-sense-summary" class="yy-hint"></p><div class="yy-sense-table-wrap"><table class="yy-sense-table"><thead><tr><th>\u5B9E\u4F53 / \u73A9\u5BB6</th><th>\u4F4D\u7F6E</th><th>\u8840\u91CF</th><th>\u72B6\u6001 / \u65F6\u95F4</th></tr></thead><tbody id="yy-sense-actors"></tbody></table></div></section><section class="card">' + yyCardHead("\u670D\u52A1\u5668\u89C6\u8DDD\u4E0E\u63A5\u5305", "connection") + '<p class="yy-hint">\u8BF7\u6C42\u534A\u5F84\u4E0E\u6279\u51C6\u534A\u5F84\u5206\u522B\u663E\u793A\uFF1B\u5B9E\u4F53\u5E7F\u64AD\u8303\u56F4\u7531\u670D\u52A1\u5668\u63A7\u5236\uFF0C\u589E\u5927\u6293\u5305\u5BB9\u91CF\u4E0D\u4F1A\u4EA7\u751F\u670D\u52A1\u5668\u672A\u4E0B\u53D1\u7684\u6570\u636E\u3002</p><div class="yy-sense-fields"><label class="yy-field">\u8BF7\u6C42\u533A\u5757\u534A\u5F84<input id="yy-view-radius" type="number" min="1" max="2147483647" value="32"></label></div><div class="yy-actions"><button id="yy-view-request" class="btn" onclick="yyRequestViewDistance()">\u8BF7\u6C42\u670D\u52A1\u5668\u89C6\u8DDD</button></div><p id="yy-view-status" class="yy-hint"></p><details><summary>\u8FDE\u63A5\u4E0E\u5E7F\u64AD\u8303\u56F4\u8BE6\u60C5</summary><pre id="yy-view-diagnostics" class="yy-readout"></pre></details></section><section class="card">' + yyCardHead("\u4E8B\u4EF6\u65F6\u95F4\u7EBF", "list") + '<details><summary>\u5C55\u5F00\u6700\u8FD120\u6761\u4E8B\u4EF6</summary><pre id="yy-sense-events" class="yy-readout">\u7B49\u5F85\u89C2\u6D4B\u6570\u636E</pre></details></section><section class="card">' + yyCardHead("\u8D26\u53F7\u6E38\u620F\u6293\u5305", "terminal") + `<p class="yy-hint">\u89E3\u7801\u540E\u7684\u670D\u52A1\u5668\u5165\u7AD9\u6E38\u620F\u5305\u3002\u4E0D\u662F\u624B\u673A\u5168\u5C40 TCP/TLS \u6293\u5305\u3002\u8BB0\u5F55\u6709\u754C\uFF1B\u661F\u53F7\u9ED8\u8BA4\u4E0D\u5305\u542B\u5927\u533A\u5757\u5305\uFF0C\u8BA4\u8BC1\u63E1\u624B\u4E0D\u8BB0\u5F55\u3002</p><label class="yy-field">\u5305\u7C7B\u578B\uFF08\u82F1\u6587\u9017\u53F7\u5206\u9694\uFF0C\u53EF\u7528 *\uFF09<input id="yy-capture-types" value="UpdateAttributes,ActorEvent,Animate,MovePlayer,UpdateBlock,Respawn"></label><div class="yy-sense-fields"><label class="yy-field">\u7F13\u51B2\u6761\u6570<input id="yy-capture-capacity" type="number" min="1" max="65536" value="256"></label><label class="yy-field">\u540C\u7C7B\u5305\u62BD\u6837\u95F4\u9694 / \u6BEB\u79D2<input id="yy-capture-sample" type="number" min="0" max="60000" value="0"></label></div><label class="yy-field">\u6293\u5305\u7559\u5B58\u9884\u7B97 / MiB<input id="yy-capture-budget" type="number" min="1" max="256" value="16"></label><label class="yy-field">\u5B57\u6BB5\u7B49\u503C\u8FC7\u6EE4 / JSON<textarea id="yy-capture-filter" rows="3" placeholder='{"entity_runtime_id":42}'>{}</textarea></label><div class="yy-actions"><button class="btn" onclick="yyCaptureAction('start')">\u5F00\u59CB\u8BB0\u5F55</button><button class="btn-s" onclick="yyCaptureAction('stop')">\u505C\u6B62</button><button class="btn-s" onclick="yyCaptureAction('clear')">\u6E05\u7A7A</button><button class="btn-s" onclick="yyCaptureExport()">\u5BFC\u51FA NDJSON</button></div><p id="yy-capture-status" class="yy-hint"></p><pre id="yy-capture-records" class="yy-readout"></pre></section><section class="card">` + yyCardHead("\u5BBF\u4E3B\u89C2\u6D4B\u5BB9\u91CF", "settings") + '<p class="yy-hint">128\u662F\u90E8\u5206\u961F\u5217\u7684\u9ED8\u8BA4\u5BB9\u91CF\uFF0C\u4E0D\u662F\u53EF\u8BC6\u522B\u7684\u5305\u79CD\u7C7B\u6570\u91CF\u3002\u4FDD\u5B58\u540E\uFF0C\u4E8B\u4EF6\u961F\u5217\u5728\u91CD\u8F7D\u63D2\u4EF6\u65F6\u751F\u6548\uFF1B\u5176\u4ED6\u8BBE\u7F6E\u7528\u4E8E\u540E\u7EED\u89C2\u6D4B\u3002\u4E0D\u4F1A\u7ED9Fast\u547D\u4EE4\u589E\u52A0CPS\u9650\u901F\u3002</p><div class="yy-sense-fields"><label class="yy-field">Lua\u666E\u901A\u4E8B\u4EF6\u961F\u5217<input id="yy-limit-event_queue" type="number" min="1" max="65536"></label><label class="yy-field">Lua\u6218\u6597\u4E8B\u4EF6\u961F\u5217<input id="yy-limit-priority_queue" type="number" min="1" max="65536"></label><label class="yy-field">Python / C++\u4E8B\u4EF6\u961F\u5217<input id="yy-limit-runtime_queue" type="number" min="1" max="65536"></label><label class="yy-field">\u5F02\u6B65\u547D\u4EE4\u7ED3\u679C<input id="yy-limit-async_tickets" type="number" min="1" max="65536"></label><label class="yy-field">\u4E8B\u4EF6\u5386\u53F2\u6761\u6570<input id="yy-limit-event_history" type="number" min="1" max="65536"></label><label class="yy-field">\u5B9E\u4F53\u7F13\u5B58\u6761\u6570<input id="yy-limit-actors" type="number" min="1" max="65536"></label><label class="yy-field">\u89C2\u6D4B\u8FC7\u671F / \u6BEB\u79D2<input id="yy-limit-stale_ms" type="number" min="1" max="86400000"></label></div><div class="yy-actions"><button class="btn" onclick="yyObservationSave()">\u4FDD\u5B58\u5BB9\u91CF</button><button class="btn-s" onclick="yyObservationLoad()">\u8BFB\u53D6\u5F53\u524D\u914D\u7F6E</button></div><p id="yy-limit-status" class="yy-hint"></p></section>');
  await yyObservationLoad();
  await yySenseRefresh();
  yySensePoll();
}
var yySensePolling = 0;
function yySensePoll() {
  clearTimeout(yySensePolling);
  yySensePolling = setTimeout(async function() {
    if (!document.getElementById("yy-sense-actors")) return;
    var auto = document.getElementById("yy-sense-auto");
    if (auto && auto.checked) await yySenseRefresh();
    yySensePoll();
  }, 2500);
}
async function yySenseRefresh() {
  try {
    var results = await Promise.all([A("POST", "/api/youyuan/game/call", { method: "game.scene", args: [{ present_only: true }] }), A("GET", "/api/youyuan/sense/events"), A("GET", "/api/youyuan/packets/capture"), A("POST", "/api/youyuan/game/call", { method: "diagnostics.visibility", args: [] })]);
    var scene = results[0].result || {};
    var summary = document.getElementById("yy-sense-summary");
    if (!summary) return;
    summary.textContent = (scene.connected ? "\u6E38\u620F\u5DF2\u8FDE\u63A5" : "\u6E38\u620F\u672A\u8FDE\u63A5") + " \xB7 \u5F53\u524D\u8D26\u53F7 " + (window.yyActiveAccount || "main") + " \xB7 \u4EC5\u663E\u793A\u672C\u673A\u5668\u4EBA\u6536\u5230\u7684\u670D\u52A1\u5668\u89C2\u6D4B";
    var rows = scene.actors || [];
    document.getElementById("yy-sense-actors").innerHTML = rows.length ? rows.map(function(a) {
      var pos = a.position_known ? (a.position || []).map((x) => Number(x).toFixed(2)).join(", ") + " / \u7EF4\u5EA6 " + a.dimension : "\u672A\u77E5";
      var health = a.health_known ? String(a.health) + (a.max_health != null ? " / " + a.max_health : "") : "\u672A\u77E5";
      var state = a.dead === true ? "\u5DF2\u6B7B\u4EA1" : a.dead === false ? "\u5B58\u6D3B" : "\u672A\u77E5";
      return "<tr><td>" + yyEscape(a.name || a.entity_type || a.runtime_id) + "</td><td>" + yyEscape(pos) + "</td><td>" + yyEscape(health) + "</td><td>" + yyEscape(state + (a.stale ? " \xB7 \u65E7\u89C2\u6D4B" : "") + " / " + (a.last_seen || "")) + "</td></tr>";
    }).join("") : '<tr><td colspan="4">\u5C1A\u672A\u6536\u5230\u670D\u52A1\u5668\u5B9E\u4F53\u72B6\u6001\uFF1B\u8FDB\u670D\u540E\u624D\u4F1A\u51FA\u73B0\u771F\u5B9E\u89C2\u6D4B\u3002</td></tr>';
    document.getElementById("yy-sense-events").textContent = JSON.stringify((results[1].events || []).slice(-20), null, 2);
    yyCaptureRender(results[2]);
    var view = document.getElementById("yy-view-diagnostics");
    if (view) view.textContent = JSON.stringify(results[3].result || results[3], null, 2);
  } catch (e) {
    T(e.message, "e");
  }
}
var yyCaptureDisplay = { paused: false, hidden: false, last: null };
function yyCaptureView(action) {
  var host = document.getElementById("yy-capture-records");
  if (!host) return;
  if (action === "pause") {
    yyCaptureDisplay.paused = !yyCaptureDisplay.paused;
    document.getElementById("yy-capture-pause").textContent = yyCaptureDisplay.paused ? "\u7EE7\u7EED\u663E\u793A" : "\u6682\u505C\u663E\u793A";
  }
  if (action === "hide") {
    yyCaptureDisplay.hidden = !yyCaptureDisplay.hidden;
    host.hidden = yyCaptureDisplay.hidden;
    document.getElementById("yy-capture-hide").textContent = yyCaptureDisplay.hidden ? "\u663E\u793A\u5185\u5BB9" : "\u9690\u85CF\u5185\u5BB9";
  }
  if (action === "collapse" || action === "expand") host.querySelectorAll("details").forEach((d) => d.open = action === "expand");
  if (!yyCaptureDisplay.paused && !yyCaptureDisplay.hidden && yyCaptureDisplay.last) yyCaptureRender(yyCaptureDisplay.last);
}
function yyCaptureRender(r) {
  yyCaptureDisplay.last = r;
  var status = document.getElementById("yy-capture-status"), host = document.getElementById("yy-capture-records");
  if (!status || !host) return;
  status.textContent = (r.active ? r.waiting_for_connection ? "\u76D1\u542C\u5DF2\u5F00\u542F\uFF0C\u7B49\u5F85\u8FDB\u670D" : "\u6B63\u5728\u8BB0\u5F55" : "\u5DF2\u505C\u6B62") + " \xB7 \u7559\u5B58 " + (r.retained_count || 0) + " \xB7 \u8986\u76D6 " + (r.overwritten || 0) + " \xB7 \u62BD\u6837\u8DF3\u8FC7 " + (r.sampled_out || 0) + (yyCaptureDisplay.paused ? " \xB7 \u663E\u793A\u5DF2\u6682\u505C\uFF0C\u540E\u53F0\u4ECD\u8BB0\u5F55" : "");
  if (host.tagName === "PRE") {
    var div = document.createElement("div");
    div.id = host.id;
    div.className = "yy-capture-list";
    host.replaceWith(div);
    host = div;
    host.insertAdjacentHTML("beforebegin", `<div class="yy-actions"><button id="yy-capture-pause" class="btn-s" onclick="yyCaptureView('pause')">\u6682\u505C\u663E\u793A</button><button class="btn-s" onclick="yyCaptureView('collapse')">\u5168\u90E8\u6298\u53E0</button><button class="btn-s" onclick="yyCaptureView('expand')">\u5168\u90E8\u5C55\u5F00</button><button id="yy-capture-hide" class="btn-s" onclick="yyCaptureView('hide')">\u9690\u85CF\u5185\u5BB9</button></div>`);
  }
  host.hidden = yyCaptureDisplay.hidden;
  if (yyCaptureDisplay.paused || yyCaptureDisplay.hidden) return;
  var open = new Set(Array.from(host.querySelectorAll("details[open]")).map((d) => d.dataset.seq));
  var scroll = host.scrollTop;
  host.style.maxHeight = "440px";
  host.style.overflow = "auto";
  host.style.overflowAnchor = "none";
  host.innerHTML = (r.records || []).slice(-12).reverse().map((p) => {
    var _a, _b, _c, _d;
    var raw = JSON.stringify(p, null, 2), key = String((_d = (_c = (_b = (_a = p.seq) != null ? _a : p.sequence) != null ? _b : p.timestamp) != null ? _c : p.time) != null ? _d : p.at), preview = raw.length > 12e3 ? raw.slice(0, 12e3) + "\n\u2026\u9884\u89C8\u5DF2\u622A\u77ED\uFF1B\u5BFC\u51FANDJSON\u53EF\u8BFB\u53D6\u5B8C\u6574\u8BB0\u5F55\u3002" : raw;
    return '<details data-seq="' + yyEscape(key) + '" ' + (open.has(key) ? "open" : "") + '><summary style="padding:12px;cursor:pointer">' + yyEscape((p.packet_type || p.type || "\u6E38\u620F\u5305") + " \xB7 " + (p.at || p.timestamp || p.time || "") + " \xB7 #" + key) + '</summary><pre class="yy-readout" style="max-height:300px;overflow:auto">' + yyEscape(preview) + "</pre></details>";
  }).join("") || '<p class="yy-hint">\u6682\u65E0\u8BB0\u5F55</p>';
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
