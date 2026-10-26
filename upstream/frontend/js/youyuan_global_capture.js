var yyGlobalCaptureDraft = null;
function yyCaptureBehaviorMount(c) {
  const host = document.getElementById("yy-gc-enabled").closest("section");
  const box = document.createElement("section");
  box.className = "yy-form-grid yy-capture-behavior";
  box.innerHTML = '<div><label for="yy-gc-game-mode">机器人游戏模式</label><select id="yy-gc-game-mode"><option value="keep">保持当前模式</option><option value="survival">生存</option><option value="creative">创造</option><option value="adventure">冒险</option><option value="spectator">旁观</option></select></div><div><label for="yy-gc-invisible">隐身效果</label><select id="yy-gc-invisible"><option value="keep">保持当前效果</option><option value="off">关闭隐身</option><option value="on">开启隐身</option></select></div>' + yySwitch("yy-gc-quick-tp", "提高抓包率", c.quick_tp !== false) + '<p class="yy-hint">启用后应用到所选机器人。建议搭配旁观模式使用。关闭后远处事件可能抓不到。隐身不会阻止拾取。</p>';
  box.innerHTML += '<div><label for="yy-gc-respawn">死亡后自动重生</label><select id="yy-gc-respawn"><option value="keep">沿用各账号设置</option><option value="on">自动重生</option><option value="off">保持死亡状态</option></select></div><div class="yy-capture-entry"><h3>多维抓包</h3><p class="yy-hint">分别设置主世界、下界、末地的机器人，并可再添加其他维度（如 dm）。指定坐标仅在上线时使用；未指定时保留原位置。进入目标维度需要对应机器人与游戏权限。</p><div class="yy-capture-entry-grid">' + [0,1,2].map(d => '<fieldset><legend>' + ['主世界','下界','末地'][d] + '</legend><label for="yy-gc-entry-mode-' + d + '">进入时的模式</label><select id="yy-gc-entry-mode-' + d + '"><option value="">沿用上方模式</option><option value="keep">保持当前模式</option><option value="survival">生存</option><option value="creative">创造</option><option value="adventure">冒险</option><option value="spectator">旁观</option></select>' + yySwitch('yy-gc-landing-' + d,'指定上线坐标',false) + '<div class="yy-capture-coordinates">' + ['x','y','z'].map(axis => '<label>' + axis.toUpperCase() + '<input aria-label="' + ['主世界','下界','末地'][d] + axis.toUpperCase() + '坐标" id="yy-gc-landing-' + d + '-' + axis + '" type="number" step="any" placeholder="' + (axis === 'y' ? '100' : '-10000') + '"></label>').join('') + '</div></fieldset>').join('') + '</div></div>';
  host.querySelector("#yy-gc-enabled").closest("label").after(box);
  document.getElementById("yy-gc-game-mode").value = c.game_mode || "keep";
  document.getElementById("yy-gc-invisible").value = c.invisible == null ? "keep" : c.invisible ? "on" : "off";
  document.getElementById("yy-gc-respawn").value = c.auto_respawn == null ? "keep" : c.auto_respawn ? "on" : "off";
}
function yyCaptureBehaviorRead() {
  const invisible = document.getElementById("yy-gc-invisible").value;
  const respawn = document.getElementById("yy-gc-respawn").value, landings = {}, modes = {};
  yyGcDimKeys.forEach(function (key, index) {
    const mode = document.getElementById('yy-gc-entry-mode-' + index);
    if (mode && mode.value) modes[key] = mode.value;
    if (index >= 3) return;
    const enabled = document.getElementById('yy-gc-landing-' + index);
    if (!enabled || !enabled.checked) return;
    const point = {};
    for (const axis of ['x','y','z']) {
      const text = document.getElementById('yy-gc-landing-' + index + '-' + axis).value;
      if (String(text).trim() === '' || !Number.isFinite(Number(text))) throw Error('请填写完整的上线坐标');
      point[axis] = Number(text);
    }
    if (Math.abs(point.x)>30000000 || Math.abs(point.z)>30000000 || point.y < -64 || point.y > 320) throw Error('上线坐标超出世界范围');
    landings[key] = point;
  });
  return { game_mode: document.getElementById("yy-gc-game-mode").value, invisible: invisible === "keep" ? null : invisible === "on", quick_tp: document.getElementById("yy-gc-quick-tp").checked, auto_respawn: respawn === "keep" ? null : respawn === "on", dimension_landings: landings, dimension_game_modes: modes, dimension_list: yyGcDimKeys.slice(0, yyCaptureDimMax) };
}
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
  var root = document.querySelector("#tab-settings>.content-inner");
  if (!root || document.getElementById("cfg-global-capture")) return;
  var row = document.createElement("section");
  row.id = "cfg-capture-card";
  row.className = "card";
  row.innerHTML = '<h3>全局抓包</h3>';
  root.append(row);
  row.insertAdjacentHTML("beforeend", yySwitch("cfg-global-capture", "\u5168\u5C40\u6293\u5305", false) + '<div class="yy-actions"><button class="btn-s" onclick="yyGlobalCapturePage()">\u914D\u7F6E\u5168\u5C40\u6293\u5305</button></div><p class="yy-hint" id="cfg-global-capture-status">\u89C2\u6D4B\u5728\u7EBF\u73A9\u5BB6\u5468\u56F4\u7684\u6E38\u620F\u4E8B\u4EF6\u3002\u4E09\u4E2A\u7EF4\u5EA6\u540C\u65F6\u8986\u76D6\u81F3\u5C11\u9700\u8981\u4E09\u4E2A\u673A\u5668\u4EBA\uFF1B\u5B9E\u9645\u8986\u76D6\u53D6\u51B3\u4E8E\u670D\u52A1\u5668\u540C\u6B65\u3002</p>');
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
var yyCaptureDimMax = 8;
var yyCaptureDimKeyRe = /^[a-z0-9_:.-]{1,40}$/;
var yyCaptureDimBuiltin = ["0", "1", "2"];
var yyCaptureDimTitles = { "0": "主世界", "1": "下界", "2": "末地" };
var yyGcDimKeys = ["0", "1", "2"];
var yyCaptureDimAccounts = [];
var yyCaptureDimEntries = [];
var yyCaptureDimConfig = {};

// 与服务端 yyCaptureDimensionKnown 同一套解析。空串表示运行时还没有这个维度
// （例如 dm）：配置保留，但不调度，也不假装已经在抓。
function yyCaptureDimResolve(key) {
  var k = String(key || "").trim().toLowerCase();
  if (/^(0|overworld|minecraft:overworld|theworld)$/.test(k)) return "0";
  if (/^(1|nether|the_nether|minecraft:nether|minecraft:the_nether)$/.test(k)) return "1";
  if (/^(2|end|the_end|minecraft:end|minecraft:the_end)$/.test(k)) return "2";
  return "";
}
function yyCaptureDimEntry(key) {
  for (var i = 0; i < yyCaptureDimEntries.length; i++) {
    var item = yyCaptureDimEntries[i];
    if (item && item.key === key) return item;
  }
  return null;
}
function yyCaptureDimMode(c, key) {
  var modes = (c || {}).dimension_game_modes || {}, dim = yyCaptureDimResolve(key);
  if (modes[key]) return modes[key];
  if (dim && modes[dim]) return modes[dim];
  return "";
}
function yyCaptureDimKeysOf(c) {
  var keys = [], seen = {}, add = function (key) {
    if (key && !seen[key] && keys.length < yyCaptureDimMax) { seen[key] = true; keys.push(key); }
  };
  yyCaptureDimBuiltin.forEach(add);
  ((c || {}).dimension_list || []).forEach(add);
  Object.keys((c || {}).dimension_accounts || {}).forEach(add);
  Object.keys((c || {}).dimension_game_modes || {}).forEach(add);
  return keys;
}
function yyCaptureDimAccountsRead() {
  var out = {};
  yyGcDimKeys.forEach(function (key, index) {
    var el = document.getElementById("yy-gc-dim-" + index);
    if (el && el.value) out[key] = el.value;
  });
  return out;
}
function yyCaptureDimCountText(key) {
  if (!yyCaptureDimResolve(key)) return "服务端维度模型暂未提供该标识：配置保留，运行时跳过并记一条 warn";
  var entry = yyCaptureDimEntry(key), workers = entry && typeof entry.workers === "number" ? entry.workers : 0;
  var accounts = entry && typeof entry.accounts === "number" ? entry.accounts : 0;
  if (workers > 0) return "该维度 " + workers + " 台机器人正在抓包";
  if (accounts > 0) return "该维度已分配 " + accounts + " 台机器人，等待连接进入";
  return "该维度暂无机器人抓包";
}
function yyCaptureDimSummary() {
  var el = document.getElementById("yy-gc-dim-summary");
  if (!el) return;
  var picked = Object.keys(yyCaptureDimAccountsRead());
  var routing = document.getElementById("yy-gc-dimensions");
  var routingOn = routing ? routing.checked : true;
  var blocked = yyGcDimKeys.filter(function (key) { return !yyCaptureDimResolve(key); });
  var usable = picked.filter(function (key) { return !!yyCaptureDimResolve(key); });
  // 一条都没指定时，后端按内置三维度轮转，所以按 3 个维度算。
  var covered = usable.length || (routingOn && !picked.length ? 3 : 0);
  var wantEl = document.getElementById("yy-gc-residents");
  var want = Number((wantEl && wantEl.value) || 0);
  var parts = ["当前 " + yyGcDimKeys.length + " 个维度条目（上限 " + yyCaptureDimMax + "），已指定 " + picked.length + " 个维度机器人"];
  if (!routingOn) parts.push("维度分工已关闭，只按账号顺序轮转");
  else if (want > covered) parts.push("当前只有 " + covered + " 个维度，多余的常驻不会生效");
  if (blocked.length) parts.push(blocked.join("、") + " 暂无法调度，配置已保留");
  el.textContent = parts.join("；");
}
function yyCaptureDimRefresh(data) {
  yyCaptureDimEntries = (data && data.dimension_entries) || [];
  yyGcDimKeys.forEach(function (key, index) {
    var note = document.getElementById("yy-gc-dim-count-" + index);
    if (note) note.textContent = yyCaptureDimCountText(key);
  });
  yyCaptureDimSummary();
}
function yyCaptureDimRow(key, index, draft) {
  var c = yyCaptureDimConfig || {}, dim = yyCaptureDimResolve(key);
  var box = document.createElement("fieldset");
  box.id = "yy-gc-dimrow-" + index;
  var legend = document.createElement("legend");
  legend.textContent = index < 3 ? yyCaptureDimTitles[key] : (dim ? yyCaptureDimTitles[dim] + " · " + key : key);
  box.append(legend);
  var picked = (draft && draft[key]) || {}, hasDraft = !!(draft && draft[key]);
  var holder = document.createElement("div");
  holder.innerHTML = '<label for="yy-gc-dim-' + index + '">负责机器人</label><select id="yy-gc-dim-' + index + '" aria-label="维度 ' + yyEscape(key) + ' 的负责机器人"><option value="">自动分配所选账号</option>' + yyCaptureDimAccounts.map(function (a) {
    return '<option value="' + yyEscape(a.id) + '">' + yyEscape(a.name) + "</option>";
  }).join("") + '</select>';
  box.append(holder);
  var select = holder.querySelector("select");
  select.value = hasDraft ? picked.account : ((c.dimension_accounts || {})[key] || "");
  var modeHolder = document.createElement("div");
  modeHolder.innerHTML = '<label for="yy-gc-entry-mode-' + index + '">进入时的模式</label><select id="yy-gc-entry-mode-' + index + '"><option value="">沿用上方模式</option><option value="keep">保持当前模式</option><option value="survival">生存</option><option value="creative">创造</option><option value="adventure">冒险</option><option value="spectator">旁观</option></select>';
  box.append(modeHolder);
  modeHolder.querySelector("select").value = hasDraft ? picked.mode : yyCaptureDimMode(c, key);
  if (index < 3) {
    // 有草稿就以草稿为准：否则用户刚关掉「指定上线坐标」，再加一个维度就会被配置里的旧值复活。
    var savedLanding = (c.dimension_landings || {})[key] || null;
    var landing = (hasDraft ? picked.landing : savedLanding) || null, on = hasDraft ? !!picked.on : !!savedLanding;
    var landingHolder = document.createElement("div");
    landingHolder.innerHTML = yySwitch("yy-gc-landing-" + index, "指定上线坐标", on);
    box.append(landingHolder);
    var coords = document.createElement("div");
    coords.className = "yy-capture-coordinates";
    coords.innerHTML = ["x", "y", "z"].map(function (axis) {
      return "<label>" + axis.toUpperCase() + '<input aria-label="' + yyCaptureDimTitles[key] + axis.toUpperCase() + '坐标" id="yy-gc-landing-' + index + "-" + axis + '" type="number" step="any" placeholder="' + (axis === "y" ? "100" : "-10000") + '"></label>';
    }).join("");
    box.append(coords);
    var enabled = box.querySelector("#yy-gc-landing-" + index);
    ["x", "y", "z"].forEach(function (axis) {
      var input = box.querySelector("#yy-gc-landing-" + index + "-" + axis);
      input.value = landing ? landing[axis] : "";
      input.disabled = !enabled.checked;
    });
    enabled.onchange = function () {
      ["x", "y", "z"].forEach(function (axis) { box.querySelector("#yy-gc-landing-" + index + "-" + axis).disabled = !enabled.checked; });
    };
  } else {
    var tip = document.createElement("p");
    tip.className = "yy-hint";
    tip.textContent = "其他维度暂不支持指定上线坐标：落点救援目前只支持主世界、下界、末地。";
    var del = document.createElement("button");
    del.type = "button";
    del.className = "btn-s";
    del.id = "yy-gc-dim-del-" + index;
    del.textContent = "删除该维度";
    del.setAttribute("aria-label", "删除维度 " + key);
    del.onclick = function () { yyCaptureDimRemove(key); };
    box.append(tip, del);
  }
  var note = document.createElement("p");
  note.className = "yy-hint";
  note.id = "yy-gc-dim-count-" + index;
  note.textContent = yyCaptureDimCountText(key);
  box.append(note);
  return box;
}
function yyCaptureDimDraft() {
  var out = {};
  yyGcDimKeys.forEach(function (key, index) {
    var select = document.getElementById("yy-gc-dim-" + index), mode = document.getElementById("yy-gc-entry-mode-" + index);
    var item = { account: select ? select.value : "", mode: mode ? mode.value : "" };
    if (index < 3) {
      var enabled = document.getElementById("yy-gc-landing-" + index);
      item.on = !!(enabled && enabled.checked);
      if (item.on) {
        item.landing = {};
        ["x", "y", "z"].forEach(function (axis) {
          var input = document.getElementById("yy-gc-landing-" + index + "-" + axis);
          item.landing[axis] = input ? input.value : "";
        });
      }
    }
    out[key] = item;
  });
  return out;
}
function yyCaptureDimRender(draft) {
  var grid = document.getElementById("yy-gc-dim-grid");
  if (!grid) return;
  // 只有明确传了草稿才按草稿渲染；否则回落到已保存配置（首次挂载时草稿是空的）。
  var values = draft && typeof draft === "object" ? draft : null;
  grid.replaceChildren();
  yyGcDimKeys.forEach(function (key, index) { grid.append(yyCaptureDimRow(key, index, values)); });
  yyCaptureDimSummary();
}
function yyCaptureDimRemove(key) {
  var index = yyGcDimKeys.indexOf(key);
  if (index < 3) return;
  var values = yyCaptureDimDraft();
  yyGcDimKeys.splice(index, 1);
  yyCaptureDimRender(values);
}
function yyCaptureDimAddOpen() {
  var box = document.getElementById("yy-gc-dim-addbox");
  if (box) box.hidden = false;
  var input = document.getElementById("yy-gc-dim-new");
  if (input) { input.value = ""; input.focus(); }
}
function yyCaptureDimAddCommit() {
  var input = document.getElementById("yy-gc-dim-new");
  var key = String((input && input.value) || "").trim().toLowerCase();
  if (!yyCaptureDimKeyRe.test(key)) throw Error("维度标识需为 1～40 位小写字母、数字或 _ : . -，例如 dm、dye_dim");
  if (yyGcDimKeys.indexOf(key) >= 0) throw Error("维度 " + key + " 已经在列表里");
  if (yyGcDimKeys.length >= yyCaptureDimMax) throw Error("维度条目最多 " + yyCaptureDimMax + " 个");
  var dim = yyCaptureDimResolve(key);
  if (dim && yyGcDimKeys.some(function (k) { return yyCaptureDimResolve(k) === dim; })) throw Error("维度 " + key + " 与已有条目指向同一个游戏维度");
  var values = yyCaptureDimDraft();
  yyGcDimKeys.push(key);
  yyCaptureDimRender(values);
  var box = document.getElementById("yy-gc-dim-addbox");
  if (box) box.hidden = true;
}
function yyCaptureDimWire() {
  var add = document.getElementById("yy-gc-dim-add");
  if (add) add.onclick = yyCaptureDimAddOpen;
  var ok = document.getElementById("yy-gc-dim-addok");
  if (ok) ok.onclick = function () { try { yyCaptureDimAddCommit(); } catch (e) { T(e.message, "e"); } };
  var cancel = document.getElementById("yy-gc-dim-addcancel");
  if (cancel) cancel.onclick = function () { var box = document.getElementById("yy-gc-dim-addbox"); if (box) box.hidden = true; };
  var input = document.getElementById("yy-gc-dim-new");
  if (input) input.onkeydown = function (ev) { if (ev.key === "Enter") { ev.preventDefault(); try { yyCaptureDimAddCommit(); } catch (e) { T(e.message, "e"); } } };
  var residents = document.getElementById("yy-gc-residents");
  if (residents) residents.oninput = yyCaptureDimSummary;
  var routing = document.getElementById("yy-gc-dimensions");
  if (routing) routing.onchange = yyCaptureDimSummary;
  var grid = document.getElementById("yy-gc-dim-grid");
  if (grid) grid.onchange = yyCaptureDimSummary;
}
function yyCaptureDimMount(c, accounts, data) {
  yyCaptureDimConfig = c || {};
  yyCaptureDimAccounts = accounts || [];
  yyCaptureDimEntries = (data && data.dimension_entries) || [];
  yyGcDimKeys = yyCaptureDimKeysOf(c);
  yyCaptureDimWire();
  yyCaptureDimRender(null);
}
async function yyGlobalCapturePage(requestedEnabled) {
  var _a;
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
  openSubPage("\u5168\u5C40\u6293\u5305", yyIntro("\u5168\u5C40\u6293\u5305", "\u9ED8\u8BA4\u81EA\u52A8\u5305\u542B\u6240\u6709\u5728\u7EBF\u73A9\u5BB6\uFF1B\u4E0D\u540C\u7EF4\u5EA6\u7531\u81EA\u5DF1\u7684\u5E38\u9A7B\u673A\u5668\u4EBA\u8D1F\u8D23\u3002") + '<section class="card">' + yySwitch("yy-gc-enabled", "\u542F\u7528\u5168\u5C40\u6293\u5305", !!c.enabled) + '<p class="yy-hint">默认只做高频抓包，不传送玩家。打开跟随后机器人机体可能被传送到目标附近或其他维度，可能触发服务端限制、改变区块加载并干扰建筑作业。跨维度与属性广播受服务器限制，不保证全服无遗漏。频率越高越占网络与服务器处理资源。</p><h3>\u673A\u5668\u4EBA\u8D26\u53F7 \xB7 \u53EF\u591A\u9009</h3><div class="yy-actions"><button class="btn-s" id="yy-gc-accounts-all">\u5168\u9009\u53EF\u7528\u8D26\u53F7</button><button class="btn-s" id="yy-gc-accounts-none">\u6E05\u7A7A</button></div><div id="yy-gc-accounts" class="yy-multi-grid"></div><p class="yy-hint">\u663E\u793A\u672C\u4EBA\u8D26\u53F7\u4E0E\u7BA1\u7406\u5458\u660E\u786E\u5171\u4EAB\u7684\u91C7\u96C6\u8D26\u53F7\uFF1B\u4E0D\u4F1A\u516C\u5F00\u5171\u4EAB\u8D26\u53F7\u4EE4\u724C\u3001\u6587\u4EF6\u6216\u63A7\u5236\u53F0\u3002\u4EC5\u63A7\u5236\u91C7\u96C6\u529F\u80FD\u3002</p><h3>\u7EF4\u5EA6\u5206\u5DE5</h3>' + yySwitch("yy-gc-dimensions", "多维抓包分工（默认）", c.dimension_routing !== false) + '<p class="yy-hint">这是各维度的优先负责人。有真人时优先回到本维度；基础常驻保留在各自维度，额外机器人分担繁忙维度。采集账号需 OP。跨维度收到包仍以实际证据为准。</p><div class="yy-form-grid" id="yy-gc-dim-grid">' + "" + "</div>" + '<div class="yy-actions"><button class="btn-s" id="yy-gc-dim-add">＋ 添加维度</button></div><div id="yy-gc-dim-addbox" hidden><label for="yy-gc-dim-new">维度标识</label><input id="yy-gc-dim-new" maxlength="40" placeholder="dm" aria-label="新维度标识，例如 dm"><button class="btn-s" id="yy-gc-dim-addok">添加</button><button class="btn-s" id="yy-gc-dim-addcancel">取消</button></div><p class="yy-hint" id="yy-gc-dim-summary"></p>' + "<h3>\u81EA\u52A8\u6295\u653E\u4E0E\u56DE\u6536</h3>" + yySwitch("yy-gc-auto-fleet", "\u6309\u771F\u4EBA\u6570\u91CF\u81EA\u52A8\u5206\u914D\u4E0E\u9519\u5CF0", c.auto_fleet !== false) + yySwitch("yy-gc-auto-provision", "自动上线 / 失败切号", c.auto_provision !== false) + '<p class="yy-hint">默认三个维度各保留一台已授权机器人；单台机器人无法跨维度。至少准备三个可用账号。没有目标的维度只驻留，不发送无意义指令。</p><label>常驻维度数量 · 默认 3，可再加其他维度</label><input id="yy-gc-residents" type="number" min="1" max="16" value="' + Number(c.resident_observers || 3) + '"><label>\u6295\u653E\u6570\u91CF\u4E0A\u9650 \xB7 0\u6309\u4EBA\u6570\u81EA\u52A8</label><input id="yy-gc-count" type="number" min="0" max="32" value="' + Number(c.account_limit || 0) + '"><label>\u4E13\u7528\u91C7\u96C6\u53F7\u7A7A\u95F2\u56DE\u6536 / \u79D2</label><input id="yy-gc-idle" type="number" min="0" max="3600" value="' + Number(c.idle_seconds || 60) + '"><p class="yy-hint">默认三台常驻；超过 10 人目标 5 台，达到 15 人目标 6 台，或展开自定义人数阶梯。手动数量可多投放帮忙，最多不超过已选账号数。没有真人不发指令。新上线采集号不自动加载业务插件、不打开独立控制台；已有业务号不会被自动关机。失败账号冷却，使用其他已选账号替补。</p><h3>\u73A9\u5BB6\u8303\u56F4</h3>' + yySwitch("yy-gc-auto", "\u81EA\u52A8\u89C2\u5BDF\u6240\u6709\u5728\u7EBF\u73A9\u5BB6\uFF08\u9ED8\u8BA4\uFF09", !!c.auto_players) + '<div id="yy-gc-manual-players"><p class="yy-hint">\u4EC5\u5173\u95ED\u81EA\u52A8\u5168\u90E8\u65F6\u4F7F\u7528\uFF1B\u65B0\u4E0A\u7EBF\u73A9\u5BB6\u4E0D\u4F1A\u81EA\u52A8\u52A0\u5165\u6307\u5B9A\u540D\u5355\u3002</p><div class="yy-actions"><button class="btn-s" id="yy-gc-players-all">\u5168\u9009\u5217\u8868</button><button class="btn-s" id="yy-gc-players-none">\u6E05\u7A7A</button></div><div id="yy-gc-players" class="yy-multi-grid"></div></div><p id="yy-gc-selection" class="yy-hint"></p><label>抓包方式</label><select id="yy-gc-mode"><option value="camera">高频抓包 · 机体静止</option><option value="hybrid">高频抓包，机体跟随（会传送）</option><option value="follow">机体跟随</option></select><label>抓包频率 / 赫兹</label><input id="yy-gc-interval" type="number" step="any" min="1" max="1000000" value="' + Math.round(1 / Number(c.interval_seconds || 0.0005)) + '"><p class="yy-hint">范围 1～1000000 赫兹（周期最短 0.001ms），默认 2000。不是越高越好：服务器里人越多，实际抓包率越低，靠增加机器人数量来补。</p><p class="yy-hint">抓包量单位：疣螈/个。高频只负责提交，服务器不保证按这个频率同步完成。</p><input id="yy-gc-height" type="hidden" value="6"><label>接近半径 / 格 · 0 关闭</label><input id="yy-gc-body-radius" type="number" step="any" min="0" max="4096" value="' + Number(c.body_radius_blocks == null ? 48 : c.body_radius_blocks) + '"><p class="yy-hint">服务端只在机器人机体附近广播实体与属性包；默认 48 格，血量与受伤显示也按此距离。设为 0 关闭（远处可能抓不到属性包）。</p><input id="yy-gc-pitch" type="hidden" value="' + Number(c.camera_pitch_degrees) + '"><input id="yy-gc-yaw" type="hidden" value="' + Number(c.camera_yaw_degrees) + '">' + yySwitch("yy-gc-pause-jobs", "导出/导入时停止高频抓包", c.pause_during_jobs !== false) + '<p class="yy-hint">导出/导入期间自动跟随与插件传送由宿主暂停，普通非移动功能继续运行。作业结束自动恢复，不关闭或重启插件。</p><div class="yy-actions"><button class="btn" id="yy-gc-save">\u4FDD\u5B58\u5168\u90E8\u914D\u7F6E</button><button class="btn-s" id="yy-gc-refresh">\u5237\u65B0\u8FD0\u884C\u72B6\u6001</button></div><details><summary>\u8FD0\u884C\u72B6\u6001\u4E0E\u5B9E\u9645\u9519\u8BEF</summary><pre id="yy-gc-state" class="yy-readout"></pre></details></section>');
  yyCaptureBehaviorMount(c);
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
  yyCaptureDimMount(c, accounts, data);
  document.getElementById("yy-gc-refresh").onclick = async () => {
    var r = await yyRootAPI("GET", "/api/youyuan/global-capture");
    document.getElementById("yy-gc-state").textContent = JSON.stringify({ plan: r.plan, accounts: r.states }, null, 2);
    if (r.ok) yyCaptureDimRefresh(r);
  };
  document.getElementById("yy-gc-state").textContent = JSON.stringify({ plan: data.plan, accounts: data.states }, null, 2);
  document.getElementById("yy-gc-save").onclick = async function() {
    this.disabled = true;
    try {
      var n = (id) => Number(document.getElementById(id).value);
      if (!document.getElementById("yy-gc-interval").checkValidity()) throw Error("抓包频率请输入 1～1000000 之间的赫兹数（周期最短 0.001ms）");
      var config = { enabled: document.getElementById("yy-gc-enabled").checked, accounts: chosen("yy-gc-accounts"), auto_fleet: document.getElementById("yy-gc-auto-fleet").checked, auto_provision: document.getElementById("yy-gc-auto-provision").checked, account_limit: n("yy-gc-count"), resident_observers: n("yy-gc-residents"), fleet_steps: yyCaptureFleetStepsRead(document.getElementById("yy-gc-fleet-steps").value), idle_seconds: n("yy-gc-idle"), dimension_routing: document.getElementById("yy-gc-dimensions").checked, dimension_accounts: yyCaptureDimAccountsRead(), dimension_list: yyGcDimKeys.slice(0, yyCaptureDimMax), players: chosen("yy-gc-players"), auto_players: document.getElementById("yy-gc-auto").checked, mode: document.getElementById("yy-gc-mode").value, interval_seconds: 1 / Math.max(1, Math.min(50, n("yy-gc-interval") || 50)), camera_height_blocks: n("yy-gc-height"), body_radius_blocks: n("yy-gc-body-radius"), camera_pitch_degrees: n("yy-gc-pitch"), camera_yaw_degrees: n("yy-gc-yaw"), pause_during_jobs: document.getElementById("yy-gc-pause-jobs").checked };
      Object.assign(config, yyCaptureBehaviorRead());
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
  yyCaptureBehaviorMount(c);
  yyCaptureDimMount(c, accounts, data);
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
      Object.assign(patch, yyCaptureBehaviorRead());
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

// 入口位置修复：设置页里那一行插在「服务器连接」卡片内部，还会被 yy-settings-groups 的
// 折叠分组收起来；手机上要翻设置页才找得到，用户体感就是「按钮没了」。
// 这里在工具页补一个固定入口，带一句用途说明，点击直接打开真实配置页
// （GET /api/youyuan/global-capture，保存走 POST 同一接口），不是空壳。
if (typeof yyModules !== "undefined" && Array.isArray(yyModules) && !yyModules.some(function (m) { return m.id === "globalcapture"; })) {
  yyModules.push({
    id: "globalcapture",
    name: "全局抓包",
    desc: "观测在线玩家周围的服务器事件",
    category: "connect",
    icon: "connection",
    open: function () { return yyGlobalCapturePage(); }
  });
  if (typeof yyAllModules !== "undefined" && yyAllModules !== yyModules && !yyAllModules.some(m => m.id === "globalcapture")) yyAllModules.push(yyModules[yyModules.length - 1]);
  // 工具排序按 yyPrefs.order 走，不同步会把新入口排到已保存顺序之外。
  if (typeof yyPrefs === "object" && yyPrefs && Array.isArray(yyPrefs.order) && yyPrefs.order.indexOf("globalcapture") < 0) yyPrefs.order.push("globalcapture");
  // 已保存过布局的老用户：直接塞进「常用工具」，省得去全部工具里翻。yyHomeIDs 现已包含它，不会被 settings.js 的过滤清掉。
  if (typeof yyPrefs === "object" && yyPrefs && Array.isArray(yyPrefs.pins) && yyPrefs.pins.indexOf("globalcapture") < 0) { yyPrefs.pins.push("globalcapture"); if (typeof yySavePrefs === "function") yySavePrefs(); }
}
