// ═══ 推送通知渲染 ═══
// 由 Go 端 notify SSE 事件或启动时 /api/push/pending 回放驱动。
// display_mode: toast(一次性) / popup(临时弹窗) / banner(常驻) / modal(弹窗) / marquee·chat·tellraw(游戏内) / none(静默)
var seenNotifs = {};       // 已展示 id 去重
var notifStackEl = null;   // toast/popup 的右上堆叠容器

function ensureNotifStack() {
  if (notifStackEl) return notifStackEl;
  if (!document.getElementById('notif-in-keyframe')) {
    var st = document.createElement('style');
    st.id = 'notif-in-keyframe';
    st.textContent = '@keyframes notifIn{from{opacity:0;transform:translateX(20px)}to{opacity:1;transform:none}}';
    document.head.appendChild(st);
  }
  notifStackEl = document.createElement('div');
  notifStackEl.id = 'notif-stack';
  notifStackEl.style.cssText = 'position:fixed;top:64px;right:14px;z-index:9999;display:flex;flex-direction:column;gap:8px;max-width:320px;pointer-events:none';
  document.body.appendChild(notifStackEl);
  return notifStackEl;
}

function renderNotification(msg) {
  if (!msg || !msg.id) return;
  if (seenNotifs[msg.id]) return;
  seenNotifs[msg.id] = true;
  var mode = msg.display_mode || 'toast';
  try {
    if (mode === 'toast') showNotifToast(msg);
    else if (mode === 'popup') showNotifPopup(msg);
    else if (mode === 'banner') showNotifBanner(msg);
    else if (mode === 'modal') showNotifModal(msg);
    else if (mode === 'marquee' || mode === 'chat' || mode === 'tellraw') { /* 游戏内投递由 Go 处理，不弹成功提示 */ }
    // none：静默，不展示
  } catch (e) {}
}

// 一次性 toast：标题+正文，4s 自动消失
function showNotifToast(msg) {
  var box = mkNotifCard(msg, 'toast');
  ensureNotifStack().appendChild(box);
  setTimeout(function () { fadeNotif(box); }, 4000);
}

// 临时弹窗：更醒目，8s 自动消失，带动作按钮
function showNotifPopup(msg) {
  var box = mkNotifCard(msg, 'popup');
  if (msg.action && msg.action !== 'none') {
    var btn = document.createElement('button');
    btn.className = 'btn act-btn';
    btn.style.cssText = 'flex:1;margin:8px 0 0;font-size:12px;padding:8px;width:100%';
    btn.textContent = actionLabel(msg.action);
    btn.addEventListener('click', function () { performAction(msg); fadeNotif(box); });
    box.appendChild(btn);
  }
  ensureNotifStack().appendChild(box);
  setTimeout(function () { fadeNotif(box); }, 8000);
}

// 常驻横幅：插入首页顶部，用户关闭时才 ack
function showNotifBanner(msg) {
  var home = document.getElementById('tab-home');
  if (!home) return;
  var b = document.createElement('div');
  b.id = 'notif-banner-' + msg.id;
  b.style.cssText = 'background:var(--phx-bg-card);padding:12px 16px;margin:0 16px 10px;border-radius:var(--phx-radius-sm);box-shadow:var(--phx-shadow);position:relative;border-left:4px solid #4a90d9';
  b.innerHTML = '<div style="font-weight:800;margin-bottom:6px;font-size:14px">' + escHtml(msg.title || '通知') + '</div>' + renderMD(msg.body || '');
  var foot = document.createElement('div');
  foot.style.cssText = 'display:flex;gap:8px;margin-top:10px';
  if (msg.action && msg.action !== 'none') {
    var ab = document.createElement('button');
    ab.className = 'btn act-btn';
    ab.style.cssText = 'flex:1;margin:0;font-size:12px;padding:8px';
    ab.textContent = actionLabel(msg.action);
    ab.addEventListener('click', function () { performAction(msg); ackNotif(msg.id); b.remove(); });
    foot.appendChild(ab);
  }
  var db = document.createElement('button');
  db.className = 'btn-s';
  db.style.cssText = 'flex:1;margin:0;font-size:12px;padding:8px';
  db.textContent = '知道了';
  db.addEventListener('click', function () { ackNotif(msg.id); b.remove(); });
  foot.appendChild(db);
  b.appendChild(foot);
  home.insertBefore(b, home.firstChild);
}

// 弹窗：模态，复用 announcements 的 showAlertModal
function showNotifModal(msg) {
  showAlertModal(
    msg.title || '通知',
    renderMD(msg.body || ''),
    true,
    function () { performAction(msg); ackNotif(msg.id); },
    (msg.action && msg.action !== 'none') ? actionLabel(msg.action) : '知道了',
    '99999'
  );
}

// 右上堆叠卡片（toast/popup 通用）
function mkNotifCard(msg, cls) {
  var box = document.createElement('div');
  box.style.cssText = 'pointer-events:auto;position:relative;background:#eafaf1;border:1px solid #2ecc71;border-left:4px solid #27ae60;border-radius:var(--phx-radius);box-shadow:var(--phx-shadow);padding:12px 30px 12px 14px;max-width:320px;animation:notifIn .25s ease';
  box.innerHTML = '<div style="font-weight:800;font-size:13px;margin-bottom:4px;color:#1e8449">' + escHtml(msg.title || '通知') + '</div>' +
    '<div style="font-size:12px;color:#566573;line-height:1.5">' + (msg.body ? renderMD(msg.body) : '') + '</div>';
  var close = document.createElement('button');
  close.textContent = '✕';
  close.style.cssText = 'position:absolute;top:6px;right:8px;background:none;border:none;font-size:14px;cursor:pointer;color:var(--phx-text-secondary)';
  close.addEventListener('click', function () { fadeNotif(box); });
  box.appendChild(close);
  return box;
}

function fadeNotif(el) {
  el.style.opacity = '0';
  el.style.transition = 'opacity .3s';
  setTimeout(function () { el.remove(); }, 300);
}

function actionLabel(a) {
  if (a === 'open_url') return '打开链接';
  if (a === 'open_toolbox') return '打开工具箱';
  if (a === 'refresh_config') return '刷新配置';
  if (a === 'command') return '执行指令';
  if (a === 'launch_app') return '启动应用';
  return '查看';
}

// 执行动作。command/marquee/chat/tellraw 由 Go 侧处理，前端无需动作。
function performAction(msg) {
  var a = msg.action, d = msg.action_data || '';
  if (a === 'open_url') openExternal(d);
  else if (a === 'open_toolbox') { var tabs = ['home', 'tasks', 'terminal', 'settings']; tabTo(tabs.indexOf(d) >= 0 ? d : 'home'); }
  else if (a === 'refresh_config') A('GET', '/api/config');
  else if (a === 'launch_app') {
    if (typeof android !== 'undefined' && android.launchApp) { try { android.launchApp(d); } catch (e) { T('启动应用失败', 'e'); } }
    else T('启动应用仅 Android 支持', 'e');
  }
}

function openExternal(u) {
  if (typeof android !== 'undefined' && android.openExternal) { try { android.openExternal(u); return; } catch (e) {} }
  window.open(u, '_blank');
}

// ack 通知（常驻 banner/modal 关闭时调用）
function ackNotif(id) {
  A('POST', '/api/push/ack', { ids: [id] });
}

// 启动回放：拉取未读常驻通知（banner/modal）
async function loadPendingNotifs() {
  try {
    var r = await A('GET', '/api/push/pending');
    if (!r.ok || !r.messages) return;
    for (var i = 0; i < r.messages.length; i++) {
      var m = r.messages[i];
      if (seenNotifs[m.id]) continue;
      var mode = m.display_mode || 'toast';
      if (mode === 'banner' || mode === 'modal') renderNotification(m);
    }
  } catch (e) {}
}

window.renderNotification = renderNotification;
window.loadPendingNotifs = loadPendingNotifs;
