var yyGlobalCaptureDraft = null;
async function yyGlobalCaptureSave(patch) {
  var result = await yyRootAPI("POST", "/api/youyuan/global-capture", patch);
  if (result.ok || !/Linux直连中断/.test(result.error || "")) return result;
  var check = await yyRootAPI("GET", "/api/youyuan/global-capture");
  function same(a, b) {
    if (a === b) return true;
    if (a == null || b == null || typeof a !== "object" || typeof b !== "object") return false;
    var ak = Object.keys(a).sort(), bk = Object.keys(b).sort();
    return ak.length === bk.length && ak.every((k, i) => k === bk[i] && same(a[k], b[k]));
  }
  if (check.ok && Object.keys(patch).every((k) => same(check.config[k], patch[k]))) return { ...check, write_recovered: true };
  return { ...result, error: "\u4FDD\u5B58\u7ED3\u679C\u5C1A\u672A\u786E\u8BA4\uFF1BLinux\u76F4\u8FDE\u54CD\u5E94\u4E2D\u65AD\uFF0C\u4E0D\u4EE3\u8868\u6E38\u620F\u5DF2\u65AD\u7EBF\u3002\u8BF7\u5237\u65B0\u8FD0\u884C\u72B6\u6001\u786E\u8BA4\u914D\u7F6E\uFF0C\u907F\u514D\u91CD\u590D\u63D0\u4EA4\u3002" };
}
function yyCaptureFleetStepsRead(text) {
  var lines = String(text || "").split(/\r?\n/).map((x) => x.trim()).filter(Boolean), previous = 0;
  if (lines.length > 32) throw Error("\u4EBA\u6570\u9636\u68AF\u6700\u591A32\u884C");
  return lines.map((line) => {
    var m = /^(\d+)\s*[:：,，]\s*(\d+)$/.exec(line);
    if (!m) throw Error("\u4EBA\u6570\u9636\u68AF\u683C\u5F0F\uFF1A\u771F\u4EBA\u6570:\u673A\u5668\u4EBA\u6570\u91CF\uFF0C\u6BCF\u884C\u4E00\u9879");
    var min = Number(m[1]), bots = Number(m[2]);
    if (min <= previous || min > 1e3 || bots < 1 || bots > 32 || previous === 0 && min !== 1) throw Error("\u7B2C\u4E00\u884C\u4ECE1\u4EBA\u5F00\u59CB\uFF0C\u771F\u4EBA\u6570\u9012\u589E\u4E14\u4E0D\u91CD\u590D\uFF1B\u673A\u5668\u4EBA\u6570\u91CF1\uFF5E32\u53F0");
    previous = min;
    return { min_players: min, observers: bots };
  });
}
function yyCaptureFleetStepsEditor(steps) {
  var box = document.createElement("details"), title = document.createElement("summary"), help = document.createElement("p"), input = document.createElement("textarea");
  title.textContent = "\u81EA\u5B9A\u4E49\u4EBA\u6570\u9636\u68AF \xB7 \u591A\u53F0\u673A\u5668\u4EBA";
  help.className = "yy-hint";
  help.textContent = "\u6BCF\u884C\u586B\u5199\u201C\u771F\u4EBA\u6570:\u673A\u5668\u4EBA\u6570\u91CF\u201D\uFF0C\u4F8B\u59821:3\u30018:6\u300115:12\u3002\u7559\u7A7A\u6CBF\u7528\u9ED8\u8BA4\u89C4\u5219\u3002\u6570\u91CF\u53D7\u5DF2\u9009\u53EF\u7528\u8D26\u53F7\u548C\u8BBE\u590732\u4E2A\u5B50\u8FDB\u7A0B\u69FD\u4F4D\u7EA6\u675F\uFF1B\u5E76\u975E\u53EA\u6709\u4E24\u53F0\u6216\u6700\u591A\u516D\u53F0\u3002\u4E09\u4E2A\u7EF4\u5EA6\u540C\u65F6\u89C2\u5BDF\u81F3\u5C11\u51C6\u5907\u4E09\u53F0\u53EF\u7528\u673A\u5668\u4EBA\u3002";
  input.id = "yy-gc-fleet-steps";
  input.rows = 5;
  input.style.cssText = "width:100%;min-height:150px;box-sizing:border-box";
  input.spellcheck = false;
  input.setAttribute("aria-label", "\u771F\u4EBA\u6570\u4E0E\u673A\u5668\u4EBA\u6295\u653E\u6570\u91CF\u9636\u68AF");
  input.placeholder = "1:3\n8:6\n15:12";
  input.value = (steps || []).map((s) => s.min_players + ":" + s.observers).join("\n");
  box.append(title, help, input);
  return box;
}
var yyGlobalCaptureSettingsBase = Rcfg;
Rcfg = async function() {
  var yySettingsStillLive = yySettingsViewGuard();
  await yyGlobalCaptureSettingsBase();
  if (!yySettingsStillLive()) return;
  var input = document.getElementById("cfg-auto-connect");
  if (!input || document.getElementById("cfg-global-capture")) return;
  var row = input.closest(".yy-switch-row");
  if (!row) return;
  row.insertAdjacentHTML("afterend", yySwitch("cfg-global-capture", "\u5168\u5C40\u6293\u5305", false) + '<div class="yy-actions"><button class="btn-s" onclick="yyGlobalCapturePage()">\u914D\u7F6E\u5168\u5C40\u6293\u5305</button></div><p class="yy-hint" id="cfg-global-capture-status">\u89C2\u6D4B\u5728\u7EBF\u73A9\u5BB6\u5468\u56F4\u7684\u6E38\u620F\u4E8B\u4EF6\u3002\u4E09\u4E2A\u7EF4\u5EA6\u540C\u65F6\u8986\u76D6\u81F3\u5C11\u9700\u8981\u4E09\u4E2A\u673A\u5668\u4EBA\uFF1B\u5B9E\u9645\u8986\u76D6\u53D6\u51B3\u4E8E\u670D\u52A1\u5668\u540C\u6B65\u3002</p>');
  var result = await yyRootAPI("GET", "/api/youyuan/global-capture");
  if (!yySettingsStillLive()) return;
  if (result.ok) {
    yyGlobalCaptureDraft = result.config;
    document.getElementById("cfg-global-capture").checked = !!result.config.enabled;
    yyServerPlayerLimitSettings(result);
  }
  document.getElementById("cfg-global-capture").onchange = async function() {
    var value = this.checked;
    this.disabled = true;
    try {
      var cfg = { enabled: value };
      var r = await yyGlobalCaptureSave(cfg);
      if (!r.ok) throw Error(r.error);
      yyGlobalCaptureDraft = r.config;
      document.getElementById("cfg-global-capture-status").textContent = value ? "\u5DF2\u5F00\u542F\uFF0C\u6309\u5F53\u524D\u771F\u4EBA\u4E0E\u4FDD\u5B58\u540D\u5355\u81EA\u52A8\u8C03\u5EA6" : "\u5DF2\u5173\u95ED\uFF0C\u91CA\u653E\u672C\u6A21\u5757\u62E5\u6709\u7684\u89C2\u5BDF\u4EFB\u52A1";
      if (value) await yyGlobalCapturePage(value);
    } catch (e) {
      this.checked = !value;
      T(e.message, "e");
      if (value) await yyGlobalCapturePage(value);
    } finally {
      this.disabled = false;
    }
  };
};
function yyGlobalMultiChoice(host, items, selected, label) {
  var set = new Set(selected || []);
  host.replaceChildren();
  for (const item of items) {
    var card = document.createElement("label");
    card.className = "yy-multi-choice";
    var box = document.createElement("input");
    box.type = "checkbox";
    box.value = item.id;
    box.checked = set.has(item.id);
    box.setAttribute("aria-label", label + " " + item.name);
    var text = document.createElement("span");
    text.textContent = item.name;
    card.append(box, text);
    host.append(card);
  }
}
async function yyGlobalCapturePage(requestedEnabled) {
  var _a, _b;
  var data = await yyRootAPI("GET", "/api/youyuan/global-capture");
  if (!data.ok) {
    T(data.error, "e");
    return;
  }
  var c = { ...data.config };
  if (typeof requestedEnabled === "boolean") c.enabled = requestedEnabled;
  yyGlobalCaptureDraft = data.config;
  var accounts = Array.isArray(data.candidates) ? data.candidates : [];
  accounts = accounts.filter((a) => a.id && !(a.id === "main" && typeof yyAccountMainConfigured !== "undefined" && yyAccountMainConfigured === false)).map((a) => ({ ...a, name: (a.name || a.id) + (a.source === "shared" ? " \xB7 \u5171\u4EAB\u91C7\u96C6" : "") }));
  if (data.advanced_allowed !== true) return yyGlobalCaptureSimplePage(data, c, accounts);
  var players = await A("GET", "/api/players/list"), raw = players.players || players.list || [];
  var names = raw.map((p) => typeof p === "string" ? p : p.name || p.username).filter(Boolean);
  (_a = c.players) == null ? void 0 : _a.forEach((n) => {
    if (!names.includes(n)) names.push(n);
  });
  delete subPageCache["\u5168\u5C40\u6293\u5305"];
  openSubPage("\u5168\u5C40\u6293\u5305", yyIntro("\u5168\u5C40\u6293\u5305", "\u9ED8\u8BA4\u81EA\u52A8\u5305\u542B\u6240\u6709\u5728\u7EBF\u73A9\u5BB6\uFF1B\u4E0D\u540C\u7EF4\u5EA6\u7531\u81EA\u5DF1\u7684\u5E38\u9A7B\u673A\u5668\u4EBA\u8D1F\u8D23\u3002") + '<section class="card">' + yySwitch("yy-gc-enabled", "\u542F\u7528\u5168\u5C40\u6293\u5305", !!c.enabled) + '<p class="yy-hint">\u9ED8\u8BA4\u4EC5\u5207\u6362\u672C\u673A\u5668\u4EBA\u7684\u539F\u751F\u6444\u50CF\u673A\u89C6\u89D2\uFF0C\u4E0D\u4F20\u9001\u73A9\u5BB6\u3002\u6253\u5F00\u81EA\u52A8\u8DDF\u968F\u540E\uFF0C\u673A\u5668\u4EBA\u8EAB\u4F53\u53EF\u80FD\u4F20\u9001\u5230\u76EE\u6807\u9644\u8FD1\u6216\u5176\u4ED6\u7EF4\u5EA6\uFF1B\u53EF\u80FD\u89E6\u53D1\u670D\u52A1\u7AEF\u9650\u5236\u3001\u6539\u53D8\u533A\u5757\u52A0\u8F7D\u5E76\u5E72\u6270\u5EFA\u7B51\u4F5C\u4E1A\u3002\u955C\u5934\u8DE8\u7EF4\u5EA6\u548C\u5C5E\u6027\u5E7F\u64AD\u5747\u53D7\u670D\u52A1\u5668\u9650\u5236\uFF0C\u4E0D\u4FDD\u8BC1\u5168\u670D\u65E0\u9057\u6F0F\u3002\u8D8A\u9AD8\u9891\u8D8A\u5360\u7F51\u7EDC\u548C\u670D\u52A1\u5668\u5904\u7406\u8D44\u6E90\u3002</p><h3>\u673A\u5668\u4EBA\u8D26\u53F7 \xB7 \u53EF\u591A\u9009</h3><div class="yy-actions"><button class="btn-s" id="yy-gc-accounts-all">\u5168\u9009\u53EF\u7528\u8D26\u53F7</button><button class="btn-s" id="yy-gc-accounts-none">\u6E05\u7A7A</button></div><div id="yy-gc-accounts" class="yy-multi-grid"></div><p class="yy-hint">\u663E\u793A\u672C\u4EBA\u8D26\u53F7\u4E0E\u7BA1\u7406\u5458\u660E\u786E\u5171\u4EAB\u7684\u91C7\u96C6\u8D26\u53F7\uFF1B\u4E0D\u4F1A\u516C\u5F00\u5171\u4EAB\u8D26\u53F7\u4EE4\u724C\u3001\u6587\u4EF6\u6216\u63A7\u5236\u53F0\u3002\u4EC5\u63A7\u5236\u91C7\u96C6\u529F\u80FD\u3002</p><h3>\u7EF4\u5EA6\u5206\u5DE5</h3>' + yySwitch("yy-gc-dimensions", "\u6309\u7EF4\u5EA6\u5E38\u9A7B\u5206\u5DE5\uFF08\u9ED8\u8BA4\uFF09", c.dimension_routing !== false) + '<p class="yy-hint">\u8FD9\u662F\u4E09\u7EF4\u5EA6\u7684\u4F18\u5148\u8D1F\u8D23\u4EBA\u3002\u6709\u771F\u4EBA\u65F6\u4F18\u5148\u56DE\u5230\u672C\u7EF4\u5EA6\uFF1B\u57FA\u7840\u5E38\u9A7B\u4FDD\u7559\u5728\u5404\u81EA\u7EF4\u5EA6\uFF0C\u989D\u5916\u673A\u5668\u4EBA\u5206\u62C5\u7E41\u5FD9\u7EF4\u5EA6\u3002\u91C7\u96C6\u8D26\u53F7\u9700OP\uFF1B\u9ED8\u8BA4\u53EA\u7528\u6444\u50CF\u673A\u89C6\u89D2\u3001\u4E0D\u5207\u65C1\u89C2\u6A21\u5F0F\uFF0C\u4E3A\u63A5\u6536\u5B9E\u4F53\u4E0E\u5C5E\u6027\u5305\u4F1A\u628A\u8EAB\u4F53\u4FDD\u6301\u5728\u76EE\u6807\u9644\u8FD1\uFF0C\u4F1A\u5173\u95ED\u5B83\u7684\u521B\u9020\u7EF4\u62A4\u3002\u8DE8\u7EF4\u5EA6\u6536\u5230\u5305\u4ECD\u4EE5\u5B9E\u9645\u8BC1\u636E\u4E3A\u51C6\u3002</p><div class="yy-form-grid">' + [0, 1, 2].map((d2) => "<div><label>" + ["\u4E3B\u4E16\u754C", "\u4E0B\u754C", "\u672B\u5730"][d2] + '</label><select id="yy-gc-dim-' + d2 + '"></select></div>').join("") + "</div><h3>\u81EA\u52A8\u6295\u653E\u4E0E\u56DE\u6536</h3>" + yySwitch("yy-gc-auto-fleet", "\u6309\u771F\u4EBA\u6570\u91CF\u81EA\u52A8\u5206\u914D\u4E0E\u9519\u5CF0", c.auto_fleet !== false) + yySwitch("yy-gc-auto-provision", "\u81EA\u52A8\u4E0A\u7EBF / \u5931\u8D25\u540E\u4F7F\u7528\u5176\u4ED6\u5DF2\u9009\u8D26\u53F7", c.auto_provision !== false) + '<p class="yy-hint">\u9ED8\u8BA4\u4E09\u4E2A\u7EF4\u5EA6\u5404\u4FDD\u7559\u4E00\u53F0\u5DF2\u6388\u6743\u673A\u5668\u4EBA\uFF1B\u5355\u53F0\u955C\u5934\u65E0\u6CD5\u8DE8\u7EF4\u5EA6\u3002\u81F3\u5C11\u51C6\u5907\u4E09\u4E2A\u53EF\u7528\u8D26\u53F7\u3002\u6CA1\u6709\u76EE\u6807\u7684\u7EF4\u5EA6\u53EA\u9A7B\u7559\uFF0C\u4E0D\u53D1\u9001\u65E0\u610F\u4E49\u955C\u5934\u547D\u4EE4\u3002</p><label>\u5E38\u9A7B\u7EF4\u5EA6\u6570\u91CF \xB7 1\uFF5E3\uFF0C\u9ED8\u8BA43</label><input id="yy-gc-residents" type="number" min="1" max="3" value="' + Number(c.resident_observers || 3) + '"><label>\u6295\u653E\u6570\u91CF\u4E0A\u9650 \xB7 0\u6309\u4EBA\u6570\u81EA\u52A8</label><input id="yy-gc-count" type="number" min="0" max="32" value="' + Number(c.account_limit || 0) + '"><label>\u4E13\u7528\u91C7\u96C6\u53F7\u7A7A\u95F2\u56DE\u6536 / \u79D2</label><input id="yy-gc-idle" type="number" min="0" max="3600" value="' + Number(c.idle_seconds || 60) + '"><p class="yy-hint">\u9ED8\u8BA4\u4E09\u53F0\u5E38\u9A7B\uFF1B\u8D85\u8FC710\u4EBA\u76EE\u68075\u53F0\uFF0C\u8FBE\u523015\u4EBA\u76EE\u68076\u53F0\uFF0C\u6216\u5C55\u5F00\u81EA\u5B9A\u4E49\u4EBA\u6570\u9636\u68AF\u3002\u624B\u52A8\u6570\u91CF\u53EF\u591A\u6295\u653E\u5E2E\u5FD9\uFF0C\u6700\u591A\u4E0D\u8D85\u8FC7\u5DF2\u9009\u8D26\u53F7\u6570\u3002\u6CA1\u6709\u771F\u4EBA\u4E0D\u53D1\u955C\u5934\u547D\u4EE4\u3002\u65B0\u4E0A\u7EBF\u91C7\u96C6\u53F7\u4E0D\u81EA\u52A8\u52A0\u8F7D\u4E1A\u52A1\u63D2\u4EF6\u3001\u4E0D\u6253\u5F00\u72EC\u7ACB\u63A7\u5236\u53F0\uFF1B\u5DF2\u6709\u4E1A\u52A1\u53F7\u4E0D\u4F1A\u88AB\u81EA\u52A8\u5173\u673A\u3002\u5931\u8D25\u8D26\u53F7\u51B7\u5374\uFF0C\u4F7F\u7528\u5176\u4ED6\u5DF2\u9009\u8D26\u53F7\u66FF\u8865\u3002</p><h3>\u73A9\u5BB6\u8303\u56F4</h3>' + yySwitch("yy-gc-auto", "\u81EA\u52A8\u89C2\u5BDF\u6240\u6709\u5728\u7EBF\u73A9\u5BB6\uFF08\u9ED8\u8BA4\uFF09", !!c.auto_players) + '<div id="yy-gc-manual-players"><p class="yy-hint">\u4EC5\u5173\u95ED\u81EA\u52A8\u5168\u90E8\u65F6\u4F7F\u7528\uFF1B\u65B0\u4E0A\u7EBF\u73A9\u5BB6\u4E0D\u4F1A\u81EA\u52A8\u52A0\u5165\u6307\u5B9A\u540D\u5355\u3002</p><div class="yy-actions"><button class="btn-s" id="yy-gc-players-all">\u5168\u9009\u5217\u8868</button><button class="btn-s" id="yy-gc-players-none">\u6E05\u7A7A</button></div><div id="yy-gc-players" class="yy-multi-grid"></div></div><p id="yy-gc-selection" class="yy-hint"></p><label>\u89C2\u5BDF\u65B9\u5F0F</label><select id="yy-gc-mode"><option value="camera">\u9AD8\u9891\u5207\u6362\u6444\u50CF\u673A \xB7 \u8EAB\u4F53\u4FDD\u6301\u5728\u76EE\u6807\u9644\u8FD1</option><option value="hybrid">\u76F8\u673A + \u8EAB\u4F53\u81EA\u52A8\u8DDF\u968F\uFF08\u53EF\u80FDTP\uFF09</option><option value="follow">\u8EAB\u4F53\u81EA\u52A8\u8DDF\u968F\uFF08\u53EF\u80FDTP\uFF09</option></select><label>\u6BCF\u6B21\u5207\u6362 / \u6BEB\u79D2</label><input id="yy-gc-interval" type="number" step="any" min="1" value="' + Number(c.interval_seconds * 1e3) + '"><p class="yy-hint">\u6BCF\u6B21\u5207\u6362\u4E00\u4E2A\u76EE\u6807\uFF1B\u4E00\u8F6E\u76EE\u6807\u65F6\u95F4\u7EA6\u4E3A\u4EBA\u6570\xD7\u95F4\u9694\u3002\u9AD8\u9891\u53EA\u8D1F\u8D23\u63D0\u4EA4\uFF0C\u670D\u52A1\u5668\u4E0D\u4FDD\u8BC1\u6309\u8FD9\u4E2A\u65F6\u95F4\u540C\u6B65\u5B8C\u6210\u3002</p><label>\u955C\u5934\u9AD8\u5EA6\u504F\u79FB / \u683C</label><input id="yy-gc-height" type="number" step="any" value="' + Number(c.camera_height_blocks) + '"><label>\u8EAB\u4F53\u63A5\u8FD1\u534A\u5F84 / \u683C \xB7 0\u5173\u95ED</label><input id="yy-gc-body-radius" type="number" step="any" min="0" max="4096" value="' + Number(c.body_radius_blocks == null ? 15 : c.body_radius_blocks) + '"><p class="yy-hint">\u670D\u52A1\u7AEF\u53EA\u5728\u673A\u5668\u4EBA\u8EAB\u4F53\u9644\u8FD1\u5E7F\u64AD\u5B9E\u4F53\u4E0E\u5C5E\u6027\u5305\uFF0C\u7EAF\u6444\u50CF\u673A\u89C6\u89D2\u6536\u4E0D\u5230\u8840\u91CF\uFF1B\u9ED8\u8BA415\u683C\uFF0C\u8840\u91CF/\u53D7\u4F24\u663E\u793A\u4E5F\u6309\u6B64\u8DDD\u79BB\u6536\u7D27\u3002\u63A5\u8FD1\u7531\u670D\u52A1\u7AEF\u8DDD\u79BB\u5224\u5B9A\u540E\u53EA\u79FB\u52A8\u673A\u5668\u4EBA\u8EAB\u4F53\uFF0C\u4E0D\u79FB\u52A8\u73A9\u5BB6\u3001\u4E0D\u5207\u65C1\u89C2\uFF1B\u8BBE\u4E3A0\u5173\u95ED\uFF08\u8FDC\u5904\u53EF\u80FD\u6293\u4E0D\u5230\u5C5E\u6027\u5305\uFF09\u3002</p><label>\u4FEF\u4EF0\u89D2 \xB7 90\u5411\u4E0B</label><input id="yy-gc-pitch" type="number" min="-90" max="90" value="' + Number(c.camera_pitch_degrees) + '"><label>\u671D\u5411</label><input id="yy-gc-yaw" type="number" step="any" value="' + Number(c.camera_yaw_degrees) + '">' + yySwitch("yy-gc-pause-jobs", "\u5BFC\u5165\u5BFC\u51FA\u671F\u95F4\u540C\u65F6\u6682\u505C\u955C\u5934\u8F6E\u5DE1", c.pause_during_jobs !== false) + '<p class="yy-hint">\u5BFC\u5165\u5BFC\u51FA\u671F\u95F4\uFF0C\u81EA\u52A8\u8DDF\u968F\u548C\u63D2\u4EF6\u4F20\u9001\u7531\u5BBF\u4E3B\u6682\u505C\uFF1B\u666E\u901A\u975E\u79FB\u52A8\u529F\u80FD\u7EE7\u7EED\u8FD0\u884C\u3002\u9ED8\u8BA4\u955C\u5934\u4E5F\u6682\u505C\u4EE5\u907F\u514D\u62A2\u533A\u5757\u89C6\u8DDD\uFF1B\u53D6\u6D88\u8FD9\u4E2A\u9009\u9879\u53EF\u4FDD\u7559\u7EAF\u76F8\u673A\u89C2\u5BDF\u3002\u4F5C\u4E1A\u7ED3\u675F\u81EA\u52A8\u6062\u590D\uFF0C\u4E0D\u5173\u95ED\u6216\u91CD\u542F\u63D2\u4EF6\u3002</p><div class="yy-actions"><button class="btn" id="yy-gc-save">\u4FDD\u5B58\u5168\u90E8\u914D\u7F6E</button><button class="btn-s" id="yy-gc-refresh">\u5237\u65B0\u8FD0\u884C\u72B6\u6001</button></div><details><summary>\u8FD0\u884C\u72B6\u6001\u4E0E\u5B9E\u9645\u9519\u8BEF</summary><pre id="yy-gc-state" class="yy-readout"></pre></details></section>');
  document.getElementById("yy-gc-count").after(yyCaptureFleetStepsEditor(c.fleet_steps));
  document.getElementById("yy-gc-mode").value = c.mode || "camera";
  yyGlobalMultiChoice(document.getElementById("yy-gc-accounts"), accounts, c.accounts, "\u673A\u5668\u4EBA");
  yyGlobalMultiChoice(document.getElementById("yy-gc-players"), names.map((n) => ({ id: n, name: n })), c.players, "\u73A9\u5BB6");
  function chosen(id) {
    return Array.from(document.querySelectorAll("#" + id + " input:checked")).map((x) => x.value);
  }
  function status() {
    document.getElementById("yy-gc-manual-players").hidden = document.getElementById("yy-gc-auto").checked;
    document.getElementById("yy-gc-selection").textContent = "\u5DF2\u9009 " + chosen("yy-gc-accounts").length + " \u4E2A\u8D26\u53F7 \xB7 " + chosen("yy-gc-players").length + " \u540D\u6307\u5B9A\u73A9\u5BB6 \xB7 " + (document.getElementById("yy-gc-auto").checked ? "\u6240\u6709\u5728\u7EBF\u73A9\u5BB6\u81EA\u52A8\u7EB3\u5165" : "\u4EC5\u6307\u5B9A\u540D\u5355") + " \xB7 \u7EDF\u4E00\u4FDD\u5B58";
  }
  for (const section of ["accounts", "players"]) {
    let host = document.getElementById("yy-gc-" + section);
    host.onchange = () => {
      if (section === "players") document.getElementById("yy-gc-auto").checked = false;
      status();
    };
    for (const action of ["all", "none"]) document.getElementById("yy-gc-" + section + "-" + action).onclick = () => {
      host.querySelectorAll("input").forEach((x) => x.checked = action === "all");
      if (section === "players") document.getElementById("yy-gc-auto").checked = false;
      status();
    };
  }
  document.getElementById("yy-gc-auto").onchange = status;
  for (var d = 0; d < 3; d++) {
    var select = document.getElementById("yy-gc-dim-" + d);
    select.innerHTML = '<option value="">\u81EA\u52A8\u5206\u914D\u6240\u9009\u8D26\u53F7</option>' + accounts.map((a) => '<option value="' + yyEscape(a.id) + '">' + yyEscape(a.name) + "</option>").join("");
    select.value = ((_b = c.dimension_accounts) == null ? void 0 : _b[String(d)]) || "";
  }
  document.getElementById("yy-gc-refresh").onclick = async () => {
    var r = await yyRootAPI("GET", "/api/youyuan/global-capture");
    document.getElementById("yy-gc-state").textContent = JSON.stringify({ plan: r.plan, accounts: r.states }, null, 2);
  };
  document.getElementById("yy-gc-state").textContent = JSON.stringify({ plan: data.plan, accounts: data.states }, null, 2);
  document.getElementById("yy-gc-save").onclick = async function() {
    this.disabled = true;
    try {
      var n = (id) => Number(document.getElementById(id).value);
      var config = { enabled: document.getElementById("yy-gc-enabled").checked, accounts: chosen("yy-gc-accounts"), auto_fleet: document.getElementById("yy-gc-auto-fleet").checked, auto_provision: document.getElementById("yy-gc-auto-provision").checked, account_limit: n("yy-gc-count"), resident_observers: n("yy-gc-residents"), fleet_steps: yyCaptureFleetStepsRead(document.getElementById("yy-gc-fleet-steps").value), idle_seconds: n("yy-gc-idle"), dimension_routing: document.getElementById("yy-gc-dimensions").checked, dimension_accounts: Object.fromEntries([0, 1, 2].map((d2) => [String(d2), document.getElementById("yy-gc-dim-" + d2).value]).filter((x) => x[1])), players: chosen("yy-gc-players"), auto_players: document.getElementById("yy-gc-auto").checked, mode: document.getElementById("yy-gc-mode").value, interval_seconds: Math.max(1, n("yy-gc-interval")) / 1e3, camera_height_blocks: n("yy-gc-height"), body_radius_blocks: n("yy-gc-body-radius"), camera_pitch_degrees: n("yy-gc-pitch"), camera_yaw_degrees: n("yy-gc-yaw"), pause_during_jobs: document.getElementById("yy-gc-pause-jobs").checked };
      var r = await yyGlobalCaptureSave(config);
      if (!r.ok) throw Error(r.error);
      yyGlobalCaptureDraft = r.config;
      var box = document.getElementById("cfg-global-capture");
      if (box) box.checked = config.enabled;
      T("\u5168\u5C40\u6293\u5305\u5168\u90E8\u914D\u7F6E\u5DF2\u4FDD\u5B58", "o");
    } catch (e) {
      T(e.message, "e");
    } finally {
      this.disabled = false;
    }
  };
  status();
}
function yyGlobalCaptureSimplePage(data, c, accounts) {
  const current = typeof yyActiveAccount !== "undefined" ? yyActiveAccount : "main", primary = accounts.find((a) => a.id === current), extra = accounts.filter((a) => a.id !== (primary == null ? void 0 : primary.id));
  delete subPageCache["\u5168\u5C40\u6293\u5305"];
  openSubPage("\u5168\u5C40\u6293\u5305", yyIntro("\u5168\u5C40\u6293\u5305", "\u89C2\u6D4B\u5728\u7EBF\u73A9\u5BB6\u5468\u56F4\u7531\u670D\u52A1\u5668\u5B9E\u9645\u4E0B\u53D1\u7684\u6E38\u620F\u4E8B\u4EF6\u3002") + '<section class="card yy-capture-simple">' + yySwitch("yy-gc-enabled", "\u542F\u7528\u5168\u5C40\u6293\u5305", !!c.enabled) + '<p class="yy-hint">\u4EC5\u7528\u4E8E\u83B7\u6388\u6743\u7684\u670D\u52A1\u5668\uFF0C\u5E76\u544A\u77E5\u73A9\u5BB6\u89C2\u6D4B\u7528\u9014\u3002\u5B9E\u9645\u8303\u56F4\u53D7\u670D\u52A1\u5668\u540C\u6B65\u4E0E\u53EF\u7528\u8D26\u53F7\u9650\u5236\uFF0C\u4E0D\u80FD\u4FDD\u8BC1\u5B8C\u6574\u8986\u76D6\u3002</p><p id="yy-gc-primary" class="yy-hint"></p><h3>\u989D\u5916\u673A\u5668\u4EBA \xB7 \u53EF\u9009\u591A\u53F0</h3><p class="yy-hint">\u53EF\u4E0D\u9009\u989D\u5916\u8D26\u53F7\uFF1B\u8D26\u53F7\u4E0D\u8DB3\u65F6\u53EF\u80FD\u65E0\u6CD5\u540C\u65F6\u8986\u76D6\u4E3B\u4E16\u754C\u3001\u4E0B\u754C\u548C\u672B\u5730\u3002\u53EF\u9009\u62E9\u591A\u4E2A\u672C\u4EBA\u6216\u660E\u786E\u5171\u4EAB\u7684\u673A\u5668\u4EBA\u8865\u5145\u8986\u76D6\u3002</p><div class="yy-actions"><button class="btn-s" id="yy-gc-all">\u5168\u9009\u989D\u5916\u8D26\u53F7</button><button class="btn-s" id="yy-gc-none">\u4E0D\u9009\u989D\u5916\u8D26\u53F7</button></div><div id="yy-gc-accounts" class="yy-multi-grid"></div><p id="yy-gc-status" class="yy-hint" role="status"></p><p class="yy-hint">\u6240\u9009\u8D26\u53F7\u4F1A\u5360\u7528\u8FDE\u63A5\u4E0E\u8FD0\u884C\u8D44\u6E90\uFF0C\u5FC5\u8981\u65F6\u5207\u6362\u7EF4\u5EA6\u5E76\u5F71\u54CD\u533A\u5757\u52A0\u8F7D\uFF1B\u5BFC\u5165\u5BFC\u51FA\u671F\u95F4\u81EA\u52A8\u6682\u505C\uFF0C\u7ED3\u675F\u540E\u6062\u590D\u3002\u8BF7\u786E\u8BA4\u8D26\u53F7\u5177\u6709\u76F8\u5E94\u670D\u52A1\u5668\u6743\u9650\u3002</p><button class="btn" id="yy-gc-save">\u4FDD\u5B58\u914D\u7F6E</button></section>');
  document.getElementById("yy-gc-primary").textContent = primary ? "\u5F53\u524D\u673A\u5668\u4EBA\uFF1A" + primary.name + "\u3002\u989D\u5916\u8D26\u53F7\u53EF\u6309\u9700\u8865\u5145\u3002" : "\u5F53\u524D\u6CA1\u6709\u53EF\u7528\u7684\u4E3B\u89C2\u5BDF\u8D26\u53F7\uFF0C\u8BF7\u9009\u62E9\u5DF2\u914D\u7F6E\u7684\u673A\u5668\u4EBA\u3002";
  yyGlobalMultiChoice(document.getElementById("yy-gc-accounts"), extra, c.accounts || [], "\u989D\u5916\u673A\u5668\u4EBA");
  if (!extra.length) document.getElementById("yy-gc-accounts").textContent = "\u6682\u65E0\u53EF\u9009\u989D\u5916\u8D26\u53F7\uFF1B\u53EF\u5148\u6DFB\u52A0\u81EA\u5DF1\u7684\u673A\u5668\u4EBA\u6216\u53D6\u5F97\u5171\u4EAB\u6388\u6743\u3002";
  const chosen = () => Array.from(document.querySelectorAll("#yy-gc-accounts input:checked")).map((x) => x.value).filter((id) => extra.some((a) => a.id === id));
  function status() {
    const count = chosen().length + (primary ? 1 : 0);
    document.getElementById("yy-gc-status").textContent = "\u5DF2\u9009\u62E9 " + count + " \u4E2A\u89C2\u6D4B\u8D26\u53F7" + (count < 3 ? "\uFF0C\u4E09\u7EF4\u5EA6\u8986\u76D6\u53EF\u80FD\u4E0D\u8DB3\u3002" : "\uFF1B\u5B9E\u9645\u8986\u76D6\u4EE5\u670D\u52A1\u5668\u4E0B\u53D1\u4E3A\u51C6\u3002");
  }
  document.getElementById("yy-gc-accounts").onchange = status;
  for (const [id, on] of [["yy-gc-all", true], ["yy-gc-none", false]]) document.getElementById(id).onclick = () => {
    document.querySelectorAll("#yy-gc-accounts input").forEach((x) => x.checked = on);
    status();
  };
  document.getElementById("yy-gc-save").onclick = async function() {
    this.disabled = true;
    try {
      const selected = chosen(), patch = { enabled: document.getElementById("yy-gc-enabled").checked, accounts: primary ? [primary.id, ...selected] : selected };
      if (patch.enabled && !patch.accounts.length) throw Error("\u5148\u914D\u7F6E\u5F53\u524D\u673A\u5668\u4EBA\u6216\u9009\u62E9\u81F3\u5C11\u4E00\u4E2A\u53EF\u7528\u8D26\u53F7");
      const r = await yyGlobalCaptureSave(patch);
      if (!r.ok) throw Error(r.error);
      yyGlobalCaptureDraft = r.config;
      const box = document.getElementById("cfg-global-capture");
      if (box) box.checked = patch.enabled;
      T("\u914D\u7F6E\u5DF2\u4FDD\u5B58", "o");
    } catch (e) {
      T(e.message, "e");
    } finally {
      this.disabled = false;
    }
  };
  status();
}
function yyServerPlayerLimitSettings(data) {
  const card = document.getElementById("cfg-conn-card");
  if (!data.ok || !card || document.getElementById("yy-server-player-limit")) return;
  const c = data.config || {};
  card.insertAdjacentHTML("beforeend", '<details id="yy-server-player-limit" class="yy-server-player-limit"><summary>\u670D\u52A1\u5668\u771F\u4EBA\u4EBA\u6570\u89C4\u5219</summary><p class="yy-hint">\u7BA1\u7406\u672C\u8BBE\u5907\u8FDE\u63A5\u670D\u52A1\u5668\u7684\u4EBA\u6570\u89C4\u5219\uFF0C\u72EC\u7ACB\u4E8E\u89C2\u6D4B\u8D26\u53F7\u9009\u62E9\uFF1B\u4EC5\u9650\u6709\u6743\u7BA1\u7406\u6B64\u670D\u52A1\u5668\u7684\u8D26\u53F7\u64CD\u4F5C\u3002</p>' + yySwitch("yy-server-enforce-limit", "\u542F\u7528\u771F\u4EBA\u5728\u7EBF\u4EBA\u6570\u4E0A\u9650", !!c.enforce_player_limit) + '<label for="yy-server-player-limit-value">\u6700\u591A\u540C\u65F6\u5728\u7EBF\u771F\u4EBA \xB7 \u673A\u5668\u4EBA\u4E0D\u8BA1\u5165</label><input id="yy-server-player-limit-value" type="number" min="1" max="1000" value="' + Number(c.player_limit || 22) + '"><p class="yy-hint">\u9ED8\u8BA4\u5173\u95ED\u3002\u5F00\u542F\u540E\u6309\u6240\u8BBE\u4EBA\u6570\u9650\u5236\u65B0\u73A9\u5BB6\u52A0\u5165\uFF0C\u673A\u5668\u4EBA\u4E0D\u5360\u540D\u989D\uFF0C\u5DF2\u5728\u7EBF\u73A9\u5BB6\u7EE7\u7EED\u4FDD\u7559\u3002</p><button class="btn-s" id="yy-server-player-limit-save">\u4FDD\u5B58\u4EBA\u6570\u89C4\u5219</button><p id="yy-server-player-limit-result" class="yy-hint" role="status"></p></details>');
  document.getElementById("yy-server-player-limit-save").onclick = async function() {
    this.disabled = true;
    const feedback = document.getElementById("yy-server-player-limit-result");
    try {
      const limit = Number(document.getElementById("yy-server-player-limit-value").value);
      if (!Number.isInteger(limit) || limit < 1 || limit > 1e3) throw Error("\u771F\u4EBA\u4E0A\u9650\u9700\u4E3A1\uFF5E1000\u7684\u6574\u6570");
      const r = await yyGlobalCaptureSave({ enforce_player_limit: document.getElementById("yy-server-enforce-limit").checked, player_limit: limit });
      if (!r.ok) throw Error(r.error || "\u4EBA\u6570\u89C4\u5219\u4FDD\u5B58\u5931\u8D25");
      yyGlobalCaptureDraft = r.config;
      feedback.textContent = "\u4EBA\u6570\u89C4\u5219\u5DF2\u4FDD\u5B58";
    } catch (e) {
      feedback.textContent = e.message;
    } finally {
      this.disabled = false;
    }
  };
}
