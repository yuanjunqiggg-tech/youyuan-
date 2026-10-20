var cfgToken = "";
var updateLocked = false;
var updateURL = "";
var useBuiltinPicker = (function() {
  try {
    return localStorage.getItem("use_builtin_picker") !== "0";
  } catch (e) {
    return true;
  }
})();
var _cfgRefreshTimer = null;
var _toolboxPollTimer = null;
var _cachedToolboxInfo = null;
async function Rcfg() {
  document.getElementById("tab-settings").innerHTML = '<div class="content-inner" style="padding:4px 0">' + (typeof android !== "undefined" ? "" : '<button class="btn-d" style="width:100%;margin-bottom:12px;background:#d32f2f" onclick="restartServer()">\u91CD\u542F\u670D\u52A1</button>') + '<div class="card" id="cfg-conn-card"><div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px"><span style="font-size:14px;font-weight:800">\u670D\u52A1\u5668\u8FDE\u63A5</span><span class="dim" id="cfg-conn-st">\u68C0\u6D4B\u4E2D...</span></div><input id="bot-server" placeholder="\u670D\u52A1\u5668\u53F7" oninput="toggleServerPass()"><div class="dim" style="font-size:10px;margin:2px 0 6px;color:var(--phx-text-secondary)">#\u623F\u95F4\u53F7=\u8054\u673A\u623F\u95F4 &nbsp; @\u623F\u95F4\u53F7=\u672C\u5730\u8054\u673A</div><input id="bot-pass" type="password" class="gone" placeholder="\u670D\u52A1\u5668\u5BC6\u7801"><div class="row"><button class="btn" onclick="doConn()">\u8FDE\u63A5\u670D\u52A1\u5668</button><button class="btn-s" onclick="doDisc()">\u65AD\u5F00</button></div></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u8BA4\u8BC1\u4FE1\u606F</div><div style="display:flex;align-items:center;gap:4px"><input id="cfg-token" type="password" placeholder="\u6E38\u620F\u767B\u5F55 Token" oninput="onTokenInput()" style="flex:1"><button id="cfg-token-add" class="btn-s" style="width:32px;height:32px;padding:0;font-size:18px;line-height:1;display:none" onclick="addSubToken()">+</button></div><div id="cfg-sub-token-list"></div><div class="dim" style="font-size:10px;margin:4px 0 8px;color:var(--phx-text-secondary)">\u8BA4\u8BC1 Token \u4EC5\u53D1\u9001\u5230\u4F60\u9009\u62E9\u7684\u8BA4\u8BC1\u670D\u52A1\uFF1B\u672C\u5730\u5DE5\u5177\u65E0\u9700\u8BA2\u9605</div><label style="font-size:12px;font-weight:700;margin:6px 0 4px">\u9A8C\u8BC1\u670D\u52A1\u5668</label><div class="picker-btn" id="cfg-server-picker" data-url="https://toolbox.youyuanqi.dpdns.org/auth" onclick="showServerPicker()"><span class="picker-label">\u7528\u6237\u9009\u62E9\u7684\u8BA4\u8BC1\u670D\u52A1</span><span class="picker-arrow">\u25BE</span></div><div id="cfg-verif-btn-wrap"><button class="btn" style="width:100%;margin-top:8px" onclick="saveCfg();setTimeout(function(){openVerifSite()},200)">\u6253\u5F00\u9A8C\u8BC1\u670D\u52A1\u5668</button></div><div id="cfg-acc-card"><div class="dim" style="font-size:11px;text-align:center;padding:8px">\u52A0\u8F7D\u4E2D...</div></div></div><div class="card" id="cfg-multi-card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u591A\u673A\u5668\u4EBA\u4F5C\u4E1A</div><div class="row" style="margin-top:6px"><button class="btn" id="cfg-fleet-conn" onclick="doFleetConn()">\u8FDE\u63A5\u5B50\u673A\u5668\u4EBA</button><button class="btn-s" onclick="doFleetDisc()">\u65AD\u5F00</button></div><div id="cfg-fleet-status" style="font-size:11px;margin-top:6px;padding:4px;border-radius:4px;background:var(--phx-bg-secondary)"></div></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u9ED8\u8BA4\u53C2\u6570\uFF08-1 \u6216 0 \u8868\u793A\u4E0D\u52A0\u4EBA\u4E3A\u5EF6\u65F6\uFF09</div><label>\u5BFC\u5165\u901F\u5EA6\uFF08\u65B9\u5757/\u79D2\uFF09</label><input id="cfg-speed" type="number" value="-1"></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u5BFC\u5165\u9ED8\u8BA4\u9009\u9879</div><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-import-cmds" checked><span class="tgl"></span> \u9ED8\u8BA4\u5BFC\u5165\u6307\u4EE4</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-cmd-disabled"><span class="tgl"></span> \u9ED8\u8BA4\u5173\u95ED\u72B6\u6001\u5BFC\u5165</label><div style="font-size:10px;color:var(--phx-text-secondary);margin-bottom:6px">\u5FAA\u73AF\u547D\u4EE4\u65B9\u5757\u4EE5\u7EA2\u77F3\u63A7\u5236\u6A21\u5F0F\u5BFC\u5165</div><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-exclude-fluids"><span class="tgl"></span> \u9ED8\u8BA4\u6392\u9664\u6D41\u4F53\u65B9\u5757</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-exclude-water" checked><span class="tgl"></span> \u9ED8\u8BA4\u6392\u9664\u6C34</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-exclude-waterlogged" checked><span class="tgl"></span> \u9ED8\u8BA4\u6392\u9664\u542B\u6C34\u65B9\u5757</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-exclude-lava" checked><span class="tgl"></span> \u9ED8\u8BA4\u6392\u9664\u5CA9\u6D46</label></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u5730\u56FE\u753B\u9ED8\u8BA4\u9009\u9879</div><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-gravity"><span class="tgl"></span> \u9ED8\u8BA4\u4F7F\u7528\u91CD\u529B\u65B9\u5757</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-platform"><span class="tgl"></span> \u9ED8\u8BA4\u653E\u7F6E\u73BB\u7483\u5E73\u53F0</label><div style="font-size:10px;color:var(--phx-text-secondary);margin-bottom:6px">\u6C34\u5E73\u5BFC\u5165\u65F6\u5728\u4E0B\u65B9\u94FA\u73BB\u7483</div></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u754C\u9762\u8BBE\u7F6E</div><label>\u4E3B\u9898\u914D\u8272</label><div class="picker-btn" id="cfg-theme-picker" onclick="showThemePicker()"><span class="picker-label">' + ({ "light": "\u6D45\u8272 \u2014 \u6696\u9EC4", "sepia": "\u590D\u53E4\u68D5", "dark": "\u6DF1\u8272 \u2014 \u84DD\u7070", "forest": "\u58A8\u7EFF \u2014 \u81EA\u7136", "amethyst": "\u6697\u7D2B \u2014 \u4F18\u96C5", "ocean": "\u6D77\u6D0B\u84DD", "sakura": "\u6A31\u82B1\u7C89", "midnight": "\u5348\u591C\u6DF1\u84DD", "glass": "\u6DB2\u6001\u73BB\u7483", "neon": "\u9713\u8679", "cream": "\u5976\u6CB9", "custom": "\u81EA\u5B9A\u4E49\uFF08\u56FE\u7247\u53D6\u8272\uFF09" }[appTheme] || "\u6D45\u8272 \u2014 \u6696\u9EC4") + '</span><span class="picker-arrow">\u25BE</span></div><button class="picker-btn" style="margin-top:8px" onclick="openPersonalizeSheet()"><span class="picker-label">\u66F4\u591A\u4E2A\u6027\u5316\u8BBE\u7F6E</span><span class="picker-arrow">\u25B8</span></button><div class="dim" style="margin-top:4px">\u754C\u9762\u7F29\u653E\u3001\u81EA\u5B9A\u4E49\u80CC\u666F\u56FE\u7247\u3001\u56FE\u7247\u53D6\u8272\u751F\u6210\u4E3B\u9898</div></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u6587\u4EF6\u9009\u62E9\u5668</div><label class="lbl-cb"><input type="checkbox" id="cfg-picker" checked onchange="setPickerMode(this.checked)"><span class="tgl"></span> \u4F7F\u7528\u5185\u7F6E\u6587\u4EF6\u9009\u62E9\u5668</label><div class="dim" style="font-size:10px;margin-top:4px">\u5F00\u542F\u540E\u53EF\u6D4F\u89C8\u8BBE\u5907\u6587\u4EF6\u76EE\u5F55\u6811\uFF1B\u5173\u95ED\u5219\u4F7F\u7528\u7CFB\u7EDF\u6587\u4EF6\u9009\u62E9\u5BF9\u8BDD\u6846</div></div><button class="btn" onclick="saveCfg()">\u4FDD\u5B58\u914D\u7F6E</button><div style="text-align:center;margin-top:10px;display:flex;gap:4px"><button class="btn-s" onclick="reopenAnnouncements()" style="flex:1;margin:0">\u91CD\u65B0\u6253\u5F00\u516C\u544A</button><button class="btn-s" onclick="checkUpdateManual()" style="flex:1;margin:0">\u68C0\u6D4B\u66F4\u65B0</button><button class="btn-s" onclick="clearCache()" style="flex:1;margin:0">\u6E05\u7406\u7F13\u5B58</button><button class="btn-s" onclick="openQA()" style="flex:1;margin:0">\u5DE5\u5355\u53CD\u9988</button></div></div>';
  refreshCfg();
  if (_cfgRefreshTimer) clearInterval(_cfgRefreshTimer);
  _cfgRefreshTimer = setInterval(refreshCfgAccount, 1e4);
  if (_toolboxPollTimer) clearInterval(_toolboxPollTimer);
  _toolboxPollTimer = setInterval(pollToolboxInfo, 3e4);
}
async function refreshCfg(checkLive) {
  var c = await A("GET", "/api/config");
  if (checkLive && !checkLive()) return c;
  if (c.ok && c.config) {
    cfgToken = c.config.token || "";
    if (c.config.auth_url) {
      curServerUrl = c.config.auth_url;
      applyServerSelection();
    }
    document.getElementById("bot-server").value = c.config.server_code || "";
    document.getElementById("cfg-token").value = c.config.token || "";
    onTokenInput();
    document.getElementById("cfg-speed").value = c.config.import_speed || -1;
    document.getElementById("cfg-import-cmds").checked = c.config.import_commands !== false;
    document.getElementById("cfg-cmd-disabled").checked = c.config.cmd_disabled || false;
    document.getElementById("cfg-exclude-fluids").checked = c.config.exclude_fluids || false;
    document.getElementById("cfg-exclude-water").checked = c.config.exclude_water !== false;
    document.getElementById("cfg-exclude-waterlogged").checked = c.config.exclude_waterlogged !== false;
    document.getElementById("cfg-exclude-lava").checked = c.config.exclude_lava !== false;
    document.getElementById("cfg-gravity").checked = c.config.use_gravity_blocks || false;
    document.getElementById("cfg-platform").checked = c.config.gravity_platform || false;
    if (c.config.bot_tokens) {
      _subTokens = (c.config.bot_tokens || []).slice();
      renderSubTokenList();
    }
    toggleServerPass();
  }
  refreshCfgStatus();
  refreshCfgAccount();
  pollToolboxInfo();
  return c;
}
async function refreshCfgStatus() {
  var s = await A("GET", "/api/bot/status");
  U(s.ok && s.connected, s.ok && s.is_op, s.ok ? s.server : "");
  var el = document.getElementById("cfg-conn-st");
  if (!el) return;
  if (s.ok && s.connected) {
    el.style.color = s.is_op ? "var(--phx-success)" : "var(--phx-warning)";
    el.innerHTML = s.is_op ? ICONS.dot + " \u5DF2\u8FDE\u63A5(OP)" : ICONS.dot + " \u5DF2\u8FDE\u63A5";
  } else {
    el.style.color = "var(--phx-text-disabled)";
    el.innerHTML = ICONS.dotEmpty + " \u672A\u8FDE\u63A5";
  }
}
async function refreshCfgAccount() {
  var el = document.getElementById("cfg-acc-card");
  if (el) el.textContent = "\u591A\u8D26\u53F7\u4F7F\u7528\u4E0B\u65B9\u5B50\u673A\u5668\u4EBA Token \u5217\u8868\u914D\u7F6E\u3002";
  refreshCfgStatus();
}
function pollToolboxInfo() {
}
function reopenAnnouncements() {
  seenAnnouncements = {};
  dismissedAnnouncements = {};
  try {
    localStorage.removeItem("ann_dismiss_");
  } catch (e) {
  }
  fetchAnnouncements();
  T("\u5DF2\u91CD\u65B0\u52A0\u8F7D\u516C\u544A", "o");
}
function checkUpdateManual() {
  A("POST", "/api/version/check", {}).then(function(r) {
    if (r.ok) {
      if (r.latest_version_code > r.current_version_code) {
        handleVersionCheck(r);
      } else {
        T("\u5DF2\u662F\u6700\u65B0\u7248\u672C", "o");
        showAlertModal(
          "\u66F4\u65B0\u65E5\u5FD7",
          '<p style="color:var(--phx-success)">' + ICONS.check + " \u5F53\u524D\u5DF2\u662F\u6700\u65B0\u7248\u672C</p>" + renderMD(r.update_message || ""),
          true,
          null,
          null,
          "1001"
        );
      }
    } else {
      T("\u68C0\u6D4B\u5931\u8D25: " + (r.error || "\u672A\u77E5"), "e");
    }
  }).catch(function(e) {
    T("\u68C0\u6D4B\u5931\u8D25: " + e.message, "e");
  });
}
function openQA() {
  yyCatalogPage();
}
function showConnModal() {
  hideConnModal();
  var m = document.createElement("div");
  m.id = "conn-modal";
  m.className = "modal-overlay";
  m.innerHTML = '<div class="modal-box" style="text-align:center"><span class="spin" style="width:32px;height:32px;display:block;margin:0 auto 14px;color:var(--phx-primary)"></span><div style="font-size:15px;font-weight:800">\u6B63\u5728\u8FDE\u63A5\u670D\u52A1\u5668</div><div style="font-size:12px;color:var(--phx-text-secondary);margin-top:6px" id="conn-stage">\u6B63\u5728\u8FDE\u63A5\u670D\u52A1\u5668...</div><div class="log" id="conn-log" style="margin-top:10px;max-height:140px;text-align:left"></div><div class="yy-actions" style="margin-top:16px"><button type="button" class="btn-s" onclick="hideConnModal()">\u5173\u95ED\u63D0\u793A \xB7 \u540E\u53F0\u7EE7\u7EED\u8FDE\u63A5</button><button type="button" class="btn-d" onclick="hideConnModal();doDisc()">\u53D6\u6D88\u672C\u6B21\u8FDE\u63A5</button></div></div>';
  document.body.appendChild(m);
}
function hideConnModal() {
  var el = document.getElementById("conn-modal");
  if (el) el.remove();
}
function updateConnStage(msg) {
  var el = document.getElementById("conn-stage");
  if (el) el.textContent = msg;
}
async function restartServer() {
  var r = await A("POST", "/api/system/restart");
  if (r.ok) {
    T("\u670D\u52A1\u91CD\u542F\u4E2D\uFF0C3\u79D2\u540E\u5237\u65B0...", "o");
    setTimeout(function() {
      location.reload();
    }, 3e3);
  } else {
    T("\u91CD\u542F\u5931\u8D25: " + (r.error || "\u672A\u77E5"), "e");
  }
}
function toggleServerPass() {
  var s = document.getElementById("bot-server");
  if (!s) return;
  var p = document.getElementById("bot-pass");
  if (!p) return;
  p.classList.toggle("gone", !/^\d+$|^[@#]/.test(s.value));
}
var CFG_SERVERS = [{ url: "https://toolbox.youyuanqi.dpdns.org/auth", label: "\u7528\u6237\u9009\u62E9\u7684\u8BA4\u8BC1\u670D\u52A1" }];
var curServerUrl = "https://toolbox.youyuanqi.dpdns.org/auth";
function showServerPicker() {
  var items = CFG_SERVERS.map(function(s) {
    return { value: s.url, label: s.label };
  });
  items.push({ value: "__custom__", label: "\u81EA\u5B9A\u4E49" });
  showPicker({
    title: "\u9009\u62E9\u9A8C\u8BC1\u670D\u52A1\u5668",
    items,
    selected: curServerUrl,
    onConfirm: function(v) {
      if (v === "__custom__") {
        var input = prompt("\u8F93\u5165\u9A8C\u8BC1\u670D\u52A1\u5668\u5730\u5740", curServerUrl);
        if (input && input.trim()) {
          curServerUrl = input.trim();
          applyServerSelection();
        }
      } else {
        curServerUrl = v;
        applyServerSelection();
      }
    }
  });
}
function getServerLabel(url) {
  for (var i = 0; i < CFG_SERVERS.length; i++) {
    if (CFG_SERVERS[i].url === url) return CFG_SERVERS[i].label;
  }
  return url.replace(/^https?:\/\//, "");
}
function applyServerSelection() {
  var p = document.getElementById("cfg-server-picker");
  if (p) {
    p.setAttribute("data-url", curServerUrl);
    p.querySelector(".picker-label").textContent = getServerLabel(curServerUrl);
  }
  var wrap = document.getElementById("cfg-verif-btn-wrap");
  if (wrap) wrap.style.display = "";
}
async function doConn() {
  var _a;
  showConnModal();
  updateConnStage("\u6B63\u5728\u8FDE\u63A5\u670D\u52A1\u5668...");
  var pw = document.getElementById("bot-pass");
  var pass = "";
  if (pw && !pw.classList.contains("gone")) pass = pw.value;
  var r = await A("POST", "/api/bot/connect", { token: document.getElementById("cfg-token").value, server: document.getElementById("bot-server").value, use_new_protocol: ((_a = document.getElementById("cfg-new-protocol")) == null ? void 0 : _a.checked) || false, auth: curServerUrl, password: pass });
  if (!r.ok) {
    hideConnModal();
    T("\u8FDE\u63A5\u5931\u8D25: " + r.error, "e");
    return;
  }
  var readyCheck = setInterval(async function() {
    if (!document.getElementById("conn-modal")) {
      clearInterval(readyCheck);
      return;
    }
    var s = await A("GET", "/api/bot/status");
    if (s.ok && s.connected) {
      var el = document.getElementById("conn-stage");
      if (el) el.textContent = "\u5DF2\u8FDE\u63A5\uFF0C\u7B49\u5F85\u6743\u9650\u9A8C\u8BC1...";
      if (s.is_op) {
        var m = document.getElementById("conn-modal");
        if (m) {
          m.remove();
          T("\u5DF2\u8FDE\u63A5", "o");
        }
        clearInterval(readyCheck);
        U(true, true);
      }
    }
  }, 1500);
}
async function doDisc() {
  await A("POST", "/api/bot/disconnect");
  T("\u5DF2\u65AD\u5F00");
  U(false);
  Rhome();
}
var _subTokens = [];
function onTokenInput() {
  var el = document.getElementById("cfg-token-add");
  if (el) el.style.display = document.getElementById("cfg-token").value.length > 10 ? "block" : "none";
}
function addSubToken() {
  _subTokens.push("");
  renderSubTokenList();
}
function removeSubToken(idx) {
  _subTokens.splice(idx, 1);
  renderSubTokenList();
}
function onSubTokenInput(idx) {
  var inp = document.getElementById("cfg-sub-token-" + idx);
  if (!inp) return;
  _subTokens[idx] = inp.value;
  var btn = document.getElementById("cfg-sub-add-" + idx);
  if (btn) btn.style.display = inp.value.length > 10 ? "block" : "none";
}
function renderSubTokenList() {
  var el = document.getElementById("cfg-sub-token-list");
  if (!el) return;
  if (_subTokens.length === 0) {
    el.innerHTML = "";
    return;
  }
  el.innerHTML = _subTokens.map(function(t, i) {
    return '<div style="display:flex;align-items:center;gap:4px;margin-top:4px"><input id="cfg-sub-token-' + i + '" type="password" placeholder="\u5B50\u673A\u5668\u4EBA' + (i + 1) + ' Token" value="' + escHtml(t) + '" oninput="onSubTokenInput(' + i + ')" style="flex:1;font-size:12px"><button id="cfg-sub-add-' + i + '" class="btn-s" style="width:32px;height:32px;padding:0;font-size:18px;line-height:1;' + (t.length > 10 ? "" : "display:none") + '" onclick="addSubToken()">+</button><button class="btn-s" style="width:32px;height:32px;padding:0;font-size:14px;line-height:1;color:var(--phx-error)" onclick="removeSubToken(' + i + ')">&times;</button></div>';
  }).join("");
}
function getSubTokens() {
  var tokens = [];
  for (var i = 0; i < _subTokens.length; i++) {
    var inp = document.getElementById("cfg-sub-token-" + i);
    if (inp && inp.value.trim()) tokens.push(inp.value.trim());
  }
  return tokens;
}
async function doFleetConn() {
  var btn = document.getElementById("cfg-fleet-conn");
  if (btn) btn.disabled = true;
  await saveCfg();
  var r = await A("POST", "/api/fleet/connect");
  if (!r.ok) {
    T(r.error || "\u8FDE\u63A5\u5931\u8D25: " + r.error, "e");
    if (btn) btn.disabled = false;
    return;
  }
  T("\u6B63\u5728\u8FDE\u63A5\u5B50\u673A\u5668\u4EBA...", "o");
  var poll = setInterval(async function() {
    var s = await A("GET", "/api/fleet/status");
    var el = document.getElementById("cfg-fleet-status");
    if (!el) return;
    if (s.ok && s.status) {
      var connected = s.status.filter(function(b) {
        return b.connected;
      }).length;
      var total = s.status.length;
      el.innerHTML = '<div style="font-size:11px">\u5DF2\u8FDE\u63A5: ' + connected + "/" + total + "</div>" + s.status.map(function(b, i) {
        var icon = b.connected ? '<span style="color:#4caf50">\u25CF</span>' : '<span style="color:#888">\u25CB</span>';
        var name = b.name || "\u5B50\u673A\u5668\u4EBA" + (i + 1);
        var err = b.error ? '<span style="color:#f44">' + b.error + "</span>" : "";
        return '<div style="padding:2px 0">' + icon + " " + name + " " + (b.is_op ? "(OP)" : "") + err + "</div>";
      }).join("");
      if (connected === total) {
        clearInterval(poll);
        if (btn) btn.disabled = false;
      }
    }
  }, 1500);
}
async function doFleetDisc() {
  await A("POST", "/api/fleet/disconnect");
  T("\u5B50\u673A\u5668\u4EBA\u5DF2\u65AD\u5F00", "o");
  var el = document.getElementById("cfg-fleet-status");
  if (el) el.innerHTML = '<div class="dim" style="font-size:11px;text-align:center;padding:4px">\u5DF2\u65AD\u5F00</div>';
}
async function saveCfg() {
  var _a, _b, _c, _d, _e, _f, _g, _h, _i, _j;
  var r = await A("POST", "/api/config", {
    token: document.getElementById("cfg-token").value,
    auth_url: curServerUrl,
    server_code: document.getElementById("bot-server").value,
    use_new_protocol: ((_a = document.getElementById("cfg-new-protocol")) == null ? void 0 : _a.checked) || false,
    server_pass: ((_b = document.getElementById("bot-pass")) == null ? void 0 : _b.value) || "",
    import_speed: parseInt(document.getElementById("cfg-speed").value) || -1,
    import_commands: ((_c = document.getElementById("cfg-import-cmds")) == null ? void 0 : _c.checked) !== false,
    cmd_disabled: ((_d = document.getElementById("cfg-cmd-disabled")) == null ? void 0 : _d.checked) || false,
    exclude_fluids: ((_e = document.getElementById("cfg-exclude-fluids")) == null ? void 0 : _e.checked) || false,
    exclude_water: ((_f = document.getElementById("cfg-exclude-water")) == null ? void 0 : _f.checked) !== false,
    exclude_waterlogged: ((_g = document.getElementById("cfg-exclude-waterlogged")) == null ? void 0 : _g.checked) !== false,
    exclude_lava: ((_h = document.getElementById("cfg-exclude-lava")) == null ? void 0 : _h.checked) !== false,
    use_gravity_blocks: ((_i = document.getElementById("cfg-gravity")) == null ? void 0 : _i.checked) || false,
    gravity_platform: ((_j = document.getElementById("cfg-platform")) == null ? void 0 : _j.checked) || false,
    bot_tokens: getSubTokens()
  });
  cfgBotTokens = getSubTokens();
  T(r.ok ? "\u5DF2\u4FDD\u5B58" : "\u5931\u8D25: " + r.error, r.ok ? "o" : "e");
}
function openVerifSite() {
  window.open(curServerUrl, "_blank", "noopener");
}
function setPickerMode(enabled) {
  useBuiltinPicker = enabled;
  try {
    localStorage.setItem("use_builtin_picker", enabled ? "1" : "0");
  } catch (e) {
  }
}
var THEME_ITEMS = [{ value: "light", label: "\u6D45\u8272 \u2014 \u6696\u9EC4" }, { value: "sepia", label: "\u590D\u53E4\u68D5" }, { value: "dark", label: "\u6DF1\u8272 \u2014 \u84DD\u7070" }, { value: "forest", label: "\u58A8\u7EFF \u2014 \u81EA\u7136" }, { value: "amethyst", label: "\u6697\u7D2B \u2014 \u4F18\u96C5" }, { value: "ocean", label: "\u6D77\u6D0B\u84DD" }, { value: "sakura", label: "\u6A31\u82B1\u7C89" }, { value: "midnight", label: "\u5348\u591C\u6DF1\u84DD" }, { value: "glass", label: "\u6DB2\u6001\u73BB\u7483" }, { value: "neon", label: "\u9713\u8679" }, { value: "cream", label: "\u5976\u6CB9" }];
function showThemePicker() {
  var items = THEME_ITEMS.slice();
  if (appTheme === "custom") items.push({ value: "custom", label: "\u81EA\u5B9A\u4E49\uFF08\u56FE\u7247\u53D6\u8272\uFF09" });
  showPicker({
    title: "\u9009\u62E9\u4E3B\u9898\u914D\u8272",
    items,
    selected: appTheme,
    onConfirm: function(v) {
      var p = document.getElementById("cfg-theme-picker");
      p.setAttribute("data-value", v);
      var it = items.find(function(i) {
        return i.value === v;
      });
      if (it) p.querySelector(".picker-label").textContent = it.label;
      setTheme(v);
    }
  });
}
async function clearCache() {
  showConfirm({
    title: "\u6E05\u7406\u7F13\u5B58",
    message: "\u786E\u5B9A\u8981\u6E05\u7406\u7F13\u5B58\u6587\u4EF6\u5417\uFF1F\n\u5C06\u5220\u9664\u5DF2\u4E0A\u4F20\u7684\u5EFA\u7B51\u6587\u4EF6\u3001\u97F3\u4E50\u6587\u4EF6\u7B49\u4E34\u65F6\u6570\u636E\u3002\n\u914D\u7F6E\u548C\u8D26\u53F7\u4FE1\u606F\u4E0D\u53D7\u5F71\u54CD\u3002",
    confirmText: "\u6E05\u7406",
    onConfirm: function() {
      doClearCache();
    }
  });
}
async function doClearCache() {
  try {
    var r = await A("POST", "/api/cache/clear");
    if (r.ok) {
      var msg = "\u5DF2\u6E05\u7406 " + r.deleted + " \u4E2A\u6587\u4EF6";
      if (r.freed > 0) {
        var size = r.freed;
        var unit = "B";
        if (size > 1073741824) {
          size = (size / 1073741824).toFixed(2);
          unit = "GB";
        } else if (size > 1048576) {
          size = (size / 1048576).toFixed(2);
          unit = "MB";
        } else if (size > 1024) {
          size = (size / 1024).toFixed(2);
          unit = "KB";
        }
        msg += ", \u91CA\u653E " + size + unit;
      }
      if (r.errors && r.errors.length) msg += "\n\u90E8\u5206\u6587\u4EF6\u5220\u9664\u5931\u8D25: " + r.errors.join(", ");
      T(msg, "o");
    } else {
      T("\u6E05\u7406\u5931\u8D25: " + (r.error || "\u672A\u77E5"), "e");
    }
  } catch (e) {
    T("\u6E05\u7406\u5931\u8D25: " + e.message, "e");
  }
}
