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
// 「新协议」开关已经从设置页移除（由后端配置决定），页面上没有 #cfg-new-protocol。
// 保存/连接时必须沿用已加载的配置值；以前直接 ?? false，会把用户原有的
// use_new_protocol 在每次保存时静默改回 false。
function yyCfgNewProtocol() {
  var el = document.getElementById("cfg-new-protocol");
  if (el) return !!el.checked;
  return typeof cfgNewProtocol !== "undefined" ? !!cfgNewProtocol : false;
}
// 设置页旧实现。真正生效的渲染链是 youyuan_settings.js 打底，
// 再由 youyuan_social → … → youyuan_game_controls → youyuan_navigation 逐层包装
// （见 outputs/fix-viewer-id/报告.md 的覆盖图谱）。本函数只作为加载早期兜底，
// 改名成 RcfgLegacy，避免和真正的那一套抢同名 id 的渲染。
async function RcfgLegacy() {
  document.getElementById("tab-settings").innerHTML = '<div class="content-inner" style="padding:4px 0">' + (typeof android !== "undefined" ? "" : '<button class="btn-d" style="width:100%;margin-bottom:12px;background:#d32f2f" onclick="restartServer()">\u91CD\u542F\u670D\u52A1</button>') + '<div class="card" id="cfg-conn-card"><div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px"><span style="font-size:14px;font-weight:800">\u670D\u52A1\u5668\u8FDE\u63A5</span><span class="dim" id="cfg-conn-st">\u68C0\u6D4B\u4E2D...</span></div><input id="bot-server" placeholder="\u670D\u52A1\u5668\u53F7" oninput="toggleServerPass()"><div class="dim" style="font-size:10px;margin:2px 0 6px;color:var(--phx-text-secondary)">#\u623F\u95F4\u53F7=\u8054\u673A\u623F\u95F4 &nbsp; @\u623F\u95F4\u53F7=\u672C\u5730\u8054\u673A</div><input id="bot-pass" type="password" class="gone" placeholder="\u670D\u52A1\u5668\u5BC6\u7801"><div class="row"><button class="btn" onclick="doConn()">\u8FDE\u63A5\u670D\u52A1\u5668</button><button class="btn-s" onclick="doDisc()">\u65AD\u5F00</button></div></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u8BA4\u8BC1\u4FE1\u606F</div><div style="display:flex;align-items:center;gap:4px"><input id="cfg-token" type="password" placeholder="\u6E38\u620F\u767B\u5F55 Token" oninput="onTokenInput()" style="flex:1"><button id="cfg-token-add" class="btn-s" style="width:32px;height:32px;padding:0;font-size:18px;line-height:1;display:none" onclick="addSubToken()">+</button></div><div id="cfg-sub-token-list"></div><div class="dim" style="font-size:10px;margin:4px 0 8px;color:var(--phx-text-secondary)">\u8BA4\u8BC1 Token \u4EC5\u53D1\u9001\u5230\u4F60\u9009\u62E9\u7684\u8BA4\u8BC1\u670D\u52A1\uFF1B\u672C\u5730\u5DE5\u5177\u65E0\u9700\u8BA2\u9605</div><label style="font-size:12px;font-weight:700;margin:6px 0 4px">\u9A8C\u8BC1\u670D\u52A1\u5668</label><div class="picker-btn" id="cfg-server-picker" data-url="https://toolbox.youyuanqi.dpdns.org/auth" onclick="showServerPicker()"><span class="picker-label">\u7528\u6237\u9009\u62E9\u7684\u8BA4\u8BC1\u670D\u52A1</span><span class="picker-arrow">\u25BE</span></div><div id="cfg-verif-btn-wrap"><button class="btn" style="width:100%;margin-top:8px" onclick="saveCfg();setTimeout(function(){openVerifSite()},200)">\u6253\u5F00\u9A8C\u8BC1\u670D\u52A1\u5668</button></div><div id="cfg-acc-card"><div class="dim" style="font-size:11px;text-align:center;padding:8px">\u52A0\u8F7D\u4E2D...</div></div></div><div class="card" id="cfg-multi-card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u591A\u673A\u5668\u4EBA\u4F5C\u4E1A</div><div class="row" style="margin-top:6px"><button class="btn" id="cfg-fleet-conn" onclick="doFleetConn()">\u8FDE\u63A5\u5B50\u673A\u5668\u4EBA</button><button class="btn-s" onclick="doFleetDisc()">\u65AD\u5F00</button></div><div id="cfg-fleet-status" style="font-size:11px;margin-top:6px;padding:4px;border-radius:4px;background:var(--phx-bg-secondary)"></div></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u9ED8\u8BA4\u53C2\u6570\uFF08-1 \u6216 0 \u8868\u793A\u4E0D\u52A0\u4EBA\u4E3A\u5EF6\u65F6\uFF09</div><label>\u5BFC\u5165\u901F\u5EA6\uFF08\u65B9\u5757/\u79D2\uFF09</label><input id="cfg-speed" type="number" value="-1"></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u5BFC\u5165\u9ED8\u8BA4\u9009\u9879</div><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-import-cmds" checked><span class="tgl"></span> \u9ED8\u8BA4\u5BFC\u5165\u6307\u4EE4</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-cmd-disabled"><span class="tgl"></span> \u9ED8\u8BA4\u5173\u95ED\u72B6\u6001\u5BFC\u5165</label><div style="font-size:10px;color:var(--phx-text-secondary);margin-bottom:6px">\u5FAA\u73AF\u547D\u4EE4\u65B9\u5757\u4EE5\u7EA2\u77F3\u63A7\u5236\u6A21\u5F0F\u5BFC\u5165</div><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-exclude-fluids"><span class="tgl"></span> \u9ED8\u8BA4\u6392\u9664\u6D41\u4F53\u65B9\u5757</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-exclude-water" checked><span class="tgl"></span> \u9ED8\u8BA4\u6392\u9664\u6C34</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-exclude-waterlogged" checked><span class="tgl"></span> \u9ED8\u8BA4\u6392\u9664\u542B\u6C34\u65B9\u5757</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-exclude-lava" checked><span class="tgl"></span> \u9ED8\u8BA4\u6392\u9664\u5CA9\u6D46</label></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u5730\u56FE\u753B\u9ED8\u8BA4\u9009\u9879</div><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-gravity"><span class="tgl"></span> \u9ED8\u8BA4\u4F7F\u7528\u91CD\u529B\u65B9\u5757</label><label class="lbl-cb" style="margin:0 0 6px"><input type="checkbox" id="cfg-platform"><span class="tgl"></span> \u9ED8\u8BA4\u653E\u7F6E\u73BB\u7483\u5E73\u53F0</label><div style="font-size:10px;color:var(--phx-text-secondary);margin-bottom:6px">\u6C34\u5E73\u5BFC\u5165\u65F6\u5728\u4E0B\u65B9\u94FA\u73BB\u7483</div></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u754C\u9762\u8BBE\u7F6E</div><label>\u4E3B\u9898\u914D\u8272</label><div class="picker-btn" id="cfg-theme-picker" onclick="showThemePicker()"><span class="picker-label">' + ({ "light": "\u6D45\u8272 \u2014 \u6696\u9EC4", "sepia": "\u590D\u53E4\u68D5", "dark": "\u6DF1\u8272 \u2014 \u84DD\u7070", "forest": "\u58A8\u7EFF \u2014 \u81EA\u7136", "amethyst": "\u6697\u7D2B \u2014 \u4F18\u96C5", "ocean": "\u6D77\u6D0B\u84DD", "sakura": "\u6A31\u82B1\u7C89", "midnight": "\u5348\u591C\u6DF1\u84DD", "glass": "\u6DB2\u6001\u73BB\u7483", "neon": "\u9713\u8679", "cream": "\u5976\u6CB9", "custom": "\u81EA\u5B9A\u4E49\uFF08\u56FE\u7247\u53D6\u8272\uFF09" }[appTheme] || "\u6D45\u8272 \u2014 \u6696\u9EC4") + '</span><span class="picker-arrow">\u25BE</span></div><button class="picker-btn" style="margin-top:8px" onclick="openPersonalizeSheet()"><span class="picker-label">\u66F4\u591A\u4E2A\u6027\u5316\u8BBE\u7F6E</span><span class="picker-arrow">\u25B8</span></button><div class="dim" style="margin-top:4px">\u754C\u9762\u7F29\u653E\u3001\u81EA\u5B9A\u4E49\u80CC\u666F\u56FE\u7247\u3001\u56FE\u7247\u53D6\u8272\u751F\u6210\u4E3B\u9898</div></div><div class="card"><div style="font-size:14px;font-weight:800;margin-bottom:10px">\u6587\u4EF6\u9009\u62E9\u5668</div><label class="lbl-cb"><input type="checkbox" id="cfg-picker" checked onchange="setPickerMode(this.checked)"><span class="tgl"></span> \u4F7F\u7528\u5185\u7F6E\u6587\u4EF6\u9009\u62E9\u5668</label><div class="dim" style="font-size:10px;margin-top:4px">\u5F00\u542F\u540E\u53EF\u6D4F\u89C8\u8BBE\u5907\u6587\u4EF6\u76EE\u5F55\u6811\uFF1B\u5173\u95ED\u5219\u4F7F\u7528\u7CFB\u7EDF\u6587\u4EF6\u9009\u62E9\u5BF9\u8BDD\u6846</div></div><button class="btn" onclick="saveCfg()">\u4FDD\u5B58\u914D\u7F6E</button><div style="text-align:center;margin-top:10px;display:flex;gap:4px"><button class="btn-s" onclick="reopenAnnouncements()" style="flex:1;margin:0">\u91CD\u65B0\u6253\u5F00\u516C\u544A</button><button class="btn-s" onclick="checkUpdateManual()" style="flex:1;margin:0">\u68C0\u6D4B\u66F4\u65B0</button><button class="btn-s" onclick="clearCache()" style="flex:1;margin:0">\u6E05\u7406\u7F13\u5B58</button><button class="btn-s" onclick="openQA()" style="flex:1;margin:0">\u5DE5\u5355\u53CD\u9988</button></div></div>';
  refreshCfg();
  if (_cfgRefreshTimer) clearInterval(_cfgRefreshTimer);
  // 隐藏时跳过：refreshCfgAccount 每次都会打 /api/bot/status，属于纯计费轮询。
  _cfgRefreshTimer = setInterval(function() {
    if (!document.hidden) refreshCfgAccount();
  }, 1e4);
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
/* ── 服务器连接弹窗 ────────────────────────────────────────────────
 * 只做「展示 + 状态绑定」：
 *   状态来自 /api/bot/status 与 /api/youyuan/connection-policy，
 *   事件来自 SSE（bot_log / conn_ready）。
 * 不估算百分比、不改协议 / 地址 / 认证 / 重连间隔。
 * 关闭弹窗 = 后台继续（连接完全不动）；「停止连接」= POST /api/bot/disconnect。
 */
var _yyConn = {
  timer: null,
  req: 0,
  busy: false,
  stopping: false,
  state: "",
  log: [],
  sensitive: false,
  status: null,
  policy: null,
  openedAt: 0,
  target: "",
  player: ""
};
var YY_CONN_STATES = {
  connecting: { label: "正在连接", tone: "wait", hint: "请求已受理，等待服务器握手", spin: true },
  retrying: { label: "重连中", tone: "wait", hint: "本轮失败，按设置的间隔自动重试", spin: true },
  connected: { label: "已连接", tone: "ok", hint: "机器人已进入服务器", spin: false },
  failed: { label: "连接失败", tone: "bad", hint: "本轮失败，自动重连未开启", spin: false },
  cancelled: { label: "已停止连接", tone: "idle", hint: "连接任务已停止", spin: false }
};
function yyConnEl(id) {
  return document.getElementById(id);
}
function yyConnToast(msg, tone) {
  if (typeof T === "function") T(msg, tone);
}
/* 默认遮挡敏感 token：日志与错误里出现的凭证只留首尾各两位。 */
function yyConnMaskValue(v) {
  var t = String(v == null ? "" : v);
  if (t.length <= 4) return "****";
  return t.slice(0, 2) + "****" + t.slice(-2);
}
function yyConnMask(text) {
  var s = String(text == null ? "" : text);
  if (_yyConn.sensitive) return s;
  s = s.replace(/((?:token|access_token|password|passwd|secret|api[_-]?key)\s*[=:]\s*["']?)([^\s"'&,;]{4,})/gi, function(all, head, value) {
    return head + yyConnMaskValue(value);
  });
  s = s.replace(/(Bearer\s+)([A-Za-z0-9._-]{6,})/gi, function(all, head, value) {
    return head + yyConnMaskValue(value);
  });
  var live = "";
  var input = yyConnEl("cfg-token");
  if (input && input.value && input.value.length >= 6) live = input.value;
  else if (typeof cfgToken === "string" && cfgToken.length >= 6) live = cfgToken;
  if (live) s = s.split(live).join(yyConnMaskValue(live));
  return s;
}
function yyConnLogOpen(box) {
  var wrap = yyConnEl("yy-conn-logwrap");
  return !!(wrap && wrap.open && box && box.offsetParent !== null);
}
function yyConnLogLine(msg) {
  var text = yyConnMask(msg);
  var inner = "";
  try {
    inner = typeof renderLogLine === "function" ? renderLogLine(text) : escHtml(text);
  } catch (e) {
    inner = typeof escHtml === "function" ? escHtml(text) : "";
  }
  return '<div class="yy-conn-line">' + inner + "</div>";
}
function yyConnAppendLog(msg) {
  _yyConn.log.push(String(msg == null ? "" : msg));
  if (_yyConn.log.length > 600) _yyConn.log.splice(0, _yyConn.log.length - 600);
  var count = yyConnEl("yy-conn-log-count");
  if (count) count.textContent = String(_yyConn.log.length);
  var box = yyConnEl("conn-log");
  if (!box || !yyConnLogOpen(box)) return;
  var follow = box.scrollHeight - box.clientHeight - box.scrollTop < 48;
  box.insertAdjacentHTML("beforeend", yyConnLogLine(_yyConn.log[_yyConn.log.length - 1]));
  while (box.children.length > 600) box.removeChild(box.firstChild);
  if (follow) box.scrollTop = box.scrollHeight;
}
function yyConnDrawLog() {
  var box = yyConnEl("conn-log");
  var count = yyConnEl("yy-conn-log-count");
  if (count) count.textContent = String(_yyConn.log.length);
  if (!box) return;
  box.innerHTML = _yyConn.log.map(yyConnLogLine).join("");
  box.scrollTop = box.scrollHeight;
}
function yyConnToggleSensitive(btn) {
  _yyConn.sensitive = !_yyConn.sensitive;
  if (btn) btn.textContent = _yyConn.sensitive ? "遮挡敏感内容" : "显示敏感内容";
  yyConnDrawLog();
}
function yyConnCopyLog() {
  if (!_yyConn.log.length) {
    yyConnToast("还没有连接日志", "w");
    return;
  }
  var text = _yyConn.log.map(function(m) {
    return yyConnMask(m);
  }).join("\n");
  var done = function() {
    yyConnToast("日志已复制", "o");
  };
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(done, function() {
      yyConnCopyFallback(text, done);
    });
    return;
  }
  yyConnCopyFallback(text, done);
}
function yyConnCopyFallback(text, done) {
  var ta = document.createElement("textarea");
  ta.value = text;
  ta.setAttribute("readonly", "");
  ta.style.position = "fixed";
  ta.style.top = "-1000px";
  ta.style.opacity = "0";
  document.body.appendChild(ta);
  ta.select();
  try {
    document.execCommand("copy");
    done();
  } catch (e) {
    yyConnToast("复制失败，请长按日志手动选择", "e");
  }
  if (ta.parentNode) ta.parentNode.removeChild(ta);
}
/* 自动重试提示：仅当后端提供重试次数 / 间隔时显示真实数据，缺数据则隐藏。 */
function yyConnSetRetry(text) {
  var el = yyConnEl("yy-conn-retry");
  if (!el) return;
  if (text) { el.textContent = text; el.hidden = false; }
  else { el.hidden = true; }
}
function yyConnSetState(key, detail) {
  var meta = YY_CONN_STATES[key] || YY_CONN_STATES.connecting;
  _yyConn.state = key;
  var label = yyConnEl("yy-conn-state-label");
  if (label) label.textContent = meta.label;
  var hint = yyConnEl("yy-conn-state-hint");
  if (hint) hint.textContent = detail || meta.hint;
  var dot = yyConnEl("yy-conn-dot");
  if (dot) dot.className = "yy-conn-dot " + meta.tone;
  var spin = yyConnEl("yy-conn-spin");
  if (spin) spin.classList.toggle("gone", !meta.spin);
  var stop = yyConnEl("yy-conn-stop");
  if (stop) stop.textContent = key === "connected" ? "断开连接" : "停止连接";
  // 非重连态一律隐藏重试提示（失败/取消/完成都不应再显示「重试中」）
  if (key !== "retrying") yyConnSetRetry("");
}
function yyConnSetTarget(server, code) {
  if (server) _yyConn.target = server;
  var el = yyConnEl("yy-conn-target-server");
  if (!el) return;
  var text = _yyConn.target || "—";
  if (code && _yyConn.target && code !== _yyConn.target) text = _yyConn.target + " · " + code;
  el.textContent = text;
  el.title = text;
}
function yyConnSetPlayer(name) {
  _yyConn.player = name || "";
  var el = yyConnEl("yy-conn-target-player");
  if (!el) return;
  var id = typeof yyActiveAccount === "string" && yyActiveAccount ? yyActiveAccount : "main";
  var text = _yyConn.player || "未识别";
  if (_yyConn.player && id !== "main") text += " · " + id;
  el.textContent = text;
}
function yyConnLocalPlayer() {
  var id = typeof yyActiveAccount === "string" && yyActiveAccount ? yyActiveAccount : "main";
  var rows = typeof yyAllAccountRows !== "undefined" && yyAllAccountRows ? yyAllAccountRows : [];
  for (var i = 0; i < rows.length; i++) {
    if (rows[i] && rows[i].id === id) return rows[i].player_name || rows[i].name || "";
  }
  return "";
}
function yyConnResolvePlayer() {
  var local = yyConnLocalPlayer();
  if (local) {
    yyConnSetPlayer(local);
    return;
  }
  A("GET", "/api/youyuan/accounts").then(function(r) {
    if (!yyConnEl("conn-modal") || !r || r.ok === false) return;
    var id = typeof yyActiveAccount === "string" && yyActiveAccount ? yyActiveAccount : "main";
    var name = "";
    var rows = r.accounts || [];
    for (var i = 0; i < rows.length; i++) {
      if (rows[i] && rows[i].id === id) {
        name = rows[i].player_name || rows[i].name || "";
        break;
      }
    }
    if (!name && r.main) name = r.main.player_name || r.main.name || "";
    if (name) yyConnSetPlayer(name);
  }).catch(function() {
  });
}
function yyConnDiagRow(label, value) {
  var text = value == null ? "—" : String(value);
  var safe = typeof escHtml === "function" ? escHtml(text) : text;
  return '<div class="yy-conn-diag-row"><label>' + label + "</label><span>" + safe + "</span></div>";
}
function yyConnDrawDiag(p, st) {
  var box = yyConnEl("yy-conn-diag-body");
  if (!box) return;
  var rows = [];
  rows.push(yyConnDiagRow("连接状态", st ? (st.connected ? (st.is_op ? "已连接（OP）" : "已连接（未授权）") : "未连接") : "读取失败"));
  rows.push(yyConnDiagRow("尝试次数", p && p.attempts != null ? p.attempts + " 次" : "—"));
  rows.push(yyConnDiagRow("连续失败", p && p.consecutive_failures != null ? p.consecutive_failures + " 次" : "—"));
  rows.push(yyConnDiagRow("自动重连", p ? (p.auto_reconnect ? (p.paused ? "已暂停（手动断开过）" : "开启") : "关闭") : "—"));
  rows.push(yyConnDiagRow("重连间隔", p ? (p.effective_retry_seconds != null ? Math.round(p.effective_retry_seconds) + " 秒" : (p.reconnect_seconds != null ? p.reconnect_seconds + " 秒" : "—")) : "—"));
  rows.push(yyConnDiagRow("最近错误", p && p.last_error ? yyConnMask(p.last_error) : "无"));
  rows.push(yyConnDiagRow("网络延迟", st && st.game_rtt_ms != null ? Math.round(st.game_rtt_ms) + " ms（RakNet ACK）" : "—"));
  rows.push(yyConnDiagRow("包队列", st && st.packet_queue_capacity ? st.packet_queue_depth + " / " + st.packet_queue_capacity : "—"));
  box.innerHTML = rows.join("");
}
function yyConnReason(p) {
  if (!p) return "无法读取连接策略";
  if (p.action_hint) return yyConnMask(p.action_hint);
  if (p.last_error) return yyConnMask(p.last_error);
  return "本轮失败，自动重连未开启";
}
function yyConnStopTimer() {
  if (_yyConn.timer) {
    clearInterval(_yyConn.timer);
    _yyConn.timer = null;
  }
}
function yyConnStartTimer() {
  yyConnStopTimer();
  if (!yyConnEl("conn-modal")) return;
  _yyConn.timer = setInterval(yyConnPoll, 2500);
}
async function yyConnPoll() {
  if (!yyConnEl("conn-modal")) {
    yyConnStopTimer();
    return;
  }
  var token = ++_yyConn.req;
  var res = await Promise.all([
    A("GET", "/api/bot/status").catch(function() {
      return null;
    }),
    A("GET", "/api/youyuan/connection-policy").catch(function() {
      return null;
    })
  ]);
  if (token !== _yyConn.req || !yyConnEl("conn-modal")) return;
  yyConnApply(res[0], res[1]);
}
function yyConnApply(status, policy) {
  var st = status && status.ok !== false ? status : null;
  var p = policy && policy.ok !== false ? policy : null;
  if (st) _yyConn.status = st;
  if (p) _yyConn.policy = p;
  if (st && st.server_target) yyConnSetTarget(st.server_target, st.server);
  yyConnDrawDiag(p || _yyConn.policy, st || _yyConn.status);
  if (_yyConn.state === "cancelled") {
    yyConnStopTimer();
    return;
  }
  if (st && st.connected) {
    yyConnSetState("connected", st.is_op ? "已进入服务器，管理员权限已验证" : "已进入服务器，等待管理员授权");
    yyConnStopTimer();
    return;
  }
  if (p && p.connected) {
    yyConnSetState("connected", "已进入服务器");
    yyConnStopTimer();
    return;
  }
  if (p && p.connecting) {
    yyConnSetState("connecting", "第 " + (p.attempts || 1) + " 次尝试，等待服务器握手");
    return;
  }
  // 刚提交请求的几秒里，后端可能还没登记本次尝试：
  // 这时候不能把上一次的失败记录当成本轮结果（会把正在连接的界面直接画成失败）。
  if (Date.now() - (_yyConn.openedAt || 0) < 6000 && _yyConn.state !== "failed") {
    yyConnSetState("connecting", "请求已受理，等待服务器握手");
    return;
  }
  if (p && ((p.consecutive_failures || 0) > 0 || p.last_error)) {
    if (p.auto_reconnect && !p.paused) {
      var gap = p.effective_retry_seconds != null ? Math.round(p.effective_retry_seconds) : Math.round(p.reconnect_seconds || 0);
      var nextAt = gap > 0 ? ("约 " + gap + " 秒后") : "稍后";
      yyConnSetState("retrying", "本轮失败，自动重连继续（间隔见诊断信息）");
      yyConnSetRetry("第 " + ((p.attempts || 0) + 1) + " 次重试 · " + nextAt + "自动重试");
    } else {
      yyConnSetState("failed", yyConnReason(p));
      yyConnStopTimer();
    }
    return;
  }
  if (_yyConn.state !== "failed") yyConnSetState("connecting", "请求已受理，等待服务器响应");
}
function yyConnEvent(msg) {
  var m = String(msg == null ? "" : msg);
  if (!m) return;
  var stage = yyConnEl("conn-stage");
  if (stage) {
    stage.textContent = yyConnMask(m);
    stage.title = stage.textContent;
  }
  var policy = _yyConn.policy;
  var autoRetry = !!(policy && policy.auto_reconnect && !policy.paused);
  if (m.indexOf("已断开") >= 0) {
    yyConnSetState("cancelled", "连接已断开");
    yyConnStopTimer();
    return;
  }
  if (m.indexOf("本轮连接失败") >= 0 || m.indexOf("连接失败") >= 0 || m.indexOf("登录失败") >= 0) {
    if (autoRetry) {
      yyConnSetState("retrying", "本轮失败，自动重连继续（间隔见诊断信息）");
      yyConnSetRetry("自动重连中，详见诊断信息");
    } else {
      yyConnSetState("failed", yyConnMask(m));
      // 还没拿到连接策略时先留一次轮询，让 /connection-policy 决定是"真失败"还是"自动重连中"
      if (policy) yyConnStopTimer();
    }
    return;
  }
  if (m.indexOf("已连接到服务器") >= 0) {
    yyConnSetState("connecting", "已进入服务器，正在初始化…");
    return;
  }
  if (m.indexOf("心跳已启动") >= 0) {
    yyConnSetState("connecting", "心跳就绪，等待管理员授权");
    return;
  }
  if (m.indexOf("命令验证失败") >= 0 || m.indexOf("请求管理员授权") >= 0 || m.indexOf("监听器已启动") >= 0) {
    yyConnSetState("connecting", "等待管理员在游戏内授权");
    return;
  }
  if (m.indexOf("验证超时") >= 0) {
    yyConnSetState("retrying", "授权验证超时，正在重试");
    if (autoRetry) yyConnSetRetry("自动重连中，详见诊断信息");
    return;
  }
  if (m.indexOf("尝试连接") >= 0) {
    yyConnSetState(autoRetry ? "retrying" : "connecting", "正在连接服务器…");
    if (autoRetry) yyConnSetRetry("自动重连中，详见诊断信息");
  }
}
function yyConnOnReady() {
  if (!yyConnEl("conn-modal")) return false;
  yyConnSetState("connected", "已进入服务器，连接就绪");
  yyConnStopTimer();
  return true;
}
function showConnModal() {
  var old = yyConnEl("conn-modal");
  if (old && old.parentNode) old.parentNode.removeChild(old);
  _yyConn.req++;
  _yyConn.openedAt = Date.now();
  var m = document.createElement("div");
  m.id = "conn-modal";
  m.className = "modal-overlay yy-conn-overlay";
  m.setAttribute("role", "dialog");
  m.setAttribute("aria-modal", "true");
  m.setAttribute("aria-labelledby", "yy-conn-title");
  m.innerHTML = '<div class="modal-box yy-conn-box">' + '<header class="yy-conn-head"><div class="yy-conn-head-main"><span class="spin yy-conn-spin" id="yy-conn-spin" aria-hidden="true"></span><h3 id="yy-conn-title">服务器连接状态</h3></div><button type="button" class="yy-conn-x" aria-label="后台继续，连接保持进行" onclick="hideConnModal()">×</button></header>' + '<div class="yy-conn-body">' + '<section class="yy-conn-id"><div class="yy-conn-id-cell"><span class="yy-conn-id-k">目标服务器</span><strong class="yy-conn-id-v" id="yy-conn-target-server">—</strong></div><div class="yy-conn-id-cell"><span class="yy-conn-id-k">关联玩家</span><strong class="yy-conn-id-v" id="yy-conn-target-player">—</strong></div></section>' + '<section class="yy-conn-status"><span class="yy-conn-dot" id="yy-conn-dot"></span><div class="yy-conn-status-text"><strong id="yy-conn-state-label" role="status">正在连接</strong><small id="yy-conn-state-hint">正在提交连接请求…</small></div></section>' + '<p class="yy-conn-event" id="conn-stage" role="status">等待第一条连接事件…</p>' + '<p class="yy-conn-retry" id="yy-conn-retry" hidden></p>' + '<details class="yy-conn-fold" id="yy-conn-diag"><summary>诊断信息<small>来自真实连接接口</small></summary><div class="yy-conn-diag-body" id="yy-conn-diag-body"></div></details>' + '<details class="yy-conn-fold" id="yy-conn-logwrap"><summary>连接日志 <small id="yy-conn-log-count">0</small></summary><div class="yy-conn-logbar"><button type="button" class="btn-s" onclick="yyConnToggleSensitive(this)">显示敏感内容</button><button type="button" class="btn-s" onclick="yyConnCopyLog()">复制日志</button></div><div class="yy-conn-log" id="conn-log" tabindex="0" aria-label="连接日志"></div></details>' + "</div>" + '<footer class="yy-conn-actions"><button type="button" class="yy-conn-bg" onclick="hideConnModal()">后台继续</button><button type="button" id="yy-conn-stop" onclick="yyConnStop()">停止连接</button></footer>' + "</div>";
  document.body.appendChild(m);
  var wrap = yyConnEl("yy-conn-logwrap");
  if (wrap) {
    wrap.addEventListener("toggle", function() {
      if (wrap.open) yyConnDrawLog();
    });
  }
  var serverInput = yyConnEl("bot-server");
  yyConnSetTarget(serverInput && serverInput.value ? serverInput.value.trim() : "", "");
  yyConnResolvePlayer();
  yyConnSetState("connecting", "正在提交连接请求…");
  yyConnDrawLog();
  yyConnStartTimer();
  yyConnPoll();
}
function hideConnModal() {
  var el = yyConnEl("conn-modal");
  if (el && el.parentNode) el.parentNode.removeChild(el);
  _yyConn.req++;
  yyConnStopTimer();
}
function updateConnStage(msg) {
  yyConnEvent(msg);
}
async function yyConnStop() {
  // 与「提交连接」分开计数：提交还在途时也要能立刻停止
  if (_yyConn.stopping) return;
  _yyConn.stopping = true;
  var btn = yyConnEl("yy-conn-stop");
  if (btn) btn.disabled = true;
  yyConnStopTimer();
  yyConnSetState("cancelled", "正在停止连接任务…");
  var failed = false;
  try {
    var r = await A("POST", "/api/bot/disconnect", {});
    if (r && r.ok === false) {
      failed = true;
      yyConnSetState("failed", "停止失败：" + (r.error || "未知原因"));
      yyConnToast(r.error || "停止失败", "e");
    } else {
      yyConnSetState("cancelled", "连接任务已停止，不会自动重试");
      yyConnToast("已停止连接");
    }
  } catch (e) {
    failed = true;
    yyConnSetState("failed", "停止失败：" + (e && e.message ? e.message : e));
  }
  _yyConn.stopping = false;
  if (btn) btn.disabled = false;
  if (failed) yyConnStartTimer();
  if (typeof refreshCfgStatus === "function") refreshCfgStatus();
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
  // 防重复点击 / 并发提交：同一次连接只允许一个在途请求
  if (_yyConn.busy) return;
  _yyConn.busy = true;
  showConnModal();
  yyConnEvent("正在提交连接请求…");
  var submitBtn = document.querySelector("#cfg-conn-card .btn");
  if (submitBtn) submitBtn.disabled = true;
  var pw = document.getElementById("bot-pass");
  var pass = "";
  if (pw && !pw.classList.contains("gone")) pass = pw.value;
  var stopPolling = false;
  try {
    var r = await A("POST", "/api/bot/connect", { token: document.getElementById("cfg-token").value, server: document.getElementById("bot-server").value, use_new_protocol: yyCfgNewProtocol(), auth: curServerUrl, password: pass });
    if (!r.ok) {
      stopPolling = true;
      var reason = r.error || "未知错误";
      yyConnSetState("failed", "提交失败：" + reason);
      yyConnEvent("提交失败：" + reason);
      T("连接失败: " + reason, "e");
      return;
    }
    yyConnSetState("connecting", "请求已受理，等待服务器握手");
  } catch (e) {
    stopPolling = true;
    var msg = e && e.message ? e.message : String(e);
    yyConnSetState("failed", "提交失败：" + msg);
    yyConnEvent("提交失败：" + msg);
  } finally {
    _yyConn.busy = false;
    if (submitBtn) submitBtn.disabled = false;
    if (stopPolling) yyConnStopTimer();
    else yyConnStartTimer();
  }
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
    use_new_protocol: yyCfgNewProtocol(),
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

// 旧实现保留但不抢 Rcfg：后续 youyuan_dashboard.js 一加载就会整份替换 Rcfg，
// 本文件这份从头到尾没被调用过（死代码）。显式挂到 RcfgLegacy，
// 只有在完全没有别的实现时（脚本缺失）才兜底成 Rcfg，保持旧行为不炸。
window.RcfgLegacy = RcfgLegacy;
if (!window.Rcfg) window.Rcfg = RcfgLegacy;
