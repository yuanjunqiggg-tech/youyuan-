// Switches save just their own policy field immediately, through the selected
// account route. A failed save rolls back the control instead of pretending.
async function yyChangeConnectionPolicy(input, field) {
  const requested = input.checked, account = yyActiveAccount;
  input.disabled = true;
  try {
    const r = await A('POST', '/api/youyuan/connection-policy', {[field]: requested});
    if (!r.ok) throw Error(r.error || '设置未保存');
    if (account !== yyActiveAccount || !input.isConnected) return;
    yySettingsData[field] = requested;
    yyDrawConnectionPolicy(r.policy);
    T(requested ? '已开启并保存' : '已关闭并保存', 'o');
  } catch (e) {
    if (account === yyActiveAccount && input.isConnected) input.checked = !requested;
    T(e.message || '保存失败，请重试', 'e');
  } finally { input.disabled = false; }
}
function yyDrawConnectionPolicy(p) {
  const host = document.getElementById('yy-connection-policy-state');
  if (!host || !p) return;
  host.textContent = [
    p.connected ? '游戏已连接' : p.connecting ? '正在连接' : '游戏未连接',
    p.auto_reconnect ? (p.paused ? '重连已暂停' : '自动重连开启') : '自动重连关闭',
    p.auto_creative ? (p.creative_confirmed ? '创造模式已确认' : '等待创造模式确认') : '保留当前模式',
    p.auto_creative ? (p.flying_confirmed ? '飞行已确认' : '等待飞行确认') : '',
    p.consecutive_failures ? '连续失败 '+p.consecutive_failures+' 轮，按退避策略重试' : '',
    p.action_hint || ''
  ].filter(Boolean).join(' · ');
  const resume = document.getElementById('yy-resume-reconnect');
  if (resume) resume.hidden = !p.paused;
}
// 进服后封面不能消失：房间查询失败或没有精确匹配时，沿用上一次同目标服务器的信息（含封面）。
// 宿主元素每次渲染设置页都会重建，host.yyCandidate 会丢，所以按目标服务器号持久化一份。
var yyServerInfoCache=(function(){try{var v=JSON.parse(localStorage.getItem('yy-server-info-cache'));return v&&v.candidate?v:null}catch(_){return null}})();
function yySaveServerInfoCache(target,candidate){yyServerInfoCache={target:target,candidate:candidate};try{localStorage.setItem('yy-server-info-cache',JSON.stringify(yyServerInfoCache))}catch(_){}}
async function yyRefreshConnectionServerInfo(status, force, stillLive) {
  const host = document.getElementById('yy-connection-server-info');
  if (!host) return;
  const connected = !!(status.ok && status.connected && status.server);
  const target = status.ok ? String((connected ? status.server : status.server_target) || status.server || '') : '';
  const changed = host.yyTarget !== target || host.yyConnected !== connected;
  if (changed) { host.yyRun = (host.yyRun || 0) + 1; host.yyCandidate = null; }
  host.yyTarget = target;
  host.yyConnected = connected;
  host.yyStatus = status;
  const draw = (candidate, note) => {
    let shown = candidate;
    if (!shown && yyServerInfoCache && yyServerInfoCache.target === target && yyServerInfoCache.candidate) shown = yyServerInfoCache.candidate;
    const c = Object.assign({target, number:target}, shown);
    if (!connected) c.players = null;
    if (window.yyRoomDiscovery) window.yyRoomDiscovery.renderCurrent(c);
    const hint = document.getElementById('yy-connection-server-note');
    if (hint) hint.textContent = note || '';
  };
  if (!connected) {
    draw(null, status.ok ? (target ? '尚未连接服务器' : '选择服务器后连接') : '连接状态读取失败');
    return;
  }
  if (!force && !changed) return;
  if (host.yyPending) {
    if (changed) { host.yyAgain = true; draw(null, '正在读取服务器信息…'); }
    return host.yyPending;
  }
  const run = host.yyRun = (host.yyRun || 0) + 1;
  draw(host.yyCandidate, '正在更新服务器信息…');
  // Query the saved account credentials, never an unsaved token from the form.
  const config = typeof yySettingsData === 'object' && yySettingsData ? yySettingsData : {};
  if (!config.token || !config.auth_url) { draw(null, '缺少已保存的游戏认证信息'); return; }
  host.yyPending = (async () => {
    try {
      const r = await A('POST', '/api/youyuan/rooms', {force:true, input:target, token:config.token, auth:config.auth_url});
      if (!stillLive() || document.getElementById('yy-connection-server-info') !== host || host.yyRun !== run) return;
      const matches = r.ok ? (r.candidates || []).filter(c => c.target === target || c.room_id === target || c.number === target) : [];
      host.yyCandidate = matches.length === 1 ? matches[0] : null;
      if (host.yyCandidate) yySaveServerInfoCache(target, host.yyCandidate);
      draw(host.yyCandidate, host.yyCandidate ? '更新于 '+new Date().toLocaleTimeString() : (r.ok ? '尚未取得当前服务器信息' : '服务器信息暂时无法更新'));
    } catch (_) {
      if (stillLive() && document.getElementById('yy-connection-server-info') === host && host.yyRun === run) { host.yyCandidate = null; draw(null, '服务器信息暂时无法更新'); }
    } finally {
      host.yyPending = null;
      if (host.yyAgain && stillLive()) { host.yyAgain = false; return yyRefreshConnectionServerInfo(host.yyStatus, true, stillLive); }
    }
  })();
  return host.yyPending;
}
async function yyRefreshConnectionPolicy(refreshInfo = true) {
  const account = yyActiveAccount;
  const live = yySettingsViewGuard(), host = document.getElementById('yy-connection-policy-state');
  const stillLive = () => account === yyActiveAccount && live() && !document.getElementById('yy-connection-server-info')?.yyDisposed && document.getElementById('yy-connection-policy-state') === host;
  try {
    const [r, status] = await Promise.all([A('GET', '/api/youyuan/connection-policy'), A('GET', '/api/bot/status')]);
    if (!stillLive()) return;
    const connected = !!(status.ok && status.connected && status.server);
    if (r.ok && status.ok) yyDrawConnectionPolicy(Object.assign({},r.policy,{connected}));
    else if (host) host.textContent = '连接策略读取失败，请刷新重试';
    const badge = document.getElementById('cfg-conn-st');
    if (badge) badge.textContent = status.ok ? (connected ? '已连接' : '未连接') : '连接状态未知';
    if (!status.ok && host) host.textContent = '连接状态未知';
    await yyRefreshConnectionServerInfo(status, refreshInfo, stillLive);
  } catch (_) { if (stillLive() && host) host.textContent = '连接状态读取失败，请刷新重试'; }
}
var yyServerInfoCleanup = null;
function yyServerRefreshSeconds() {
  try { const value = Number(localStorage.getItem('yy-server-refresh-seconds')); if (Number.isInteger(value) && value >= 30 && value <= 60) return value; } catch (_) {}
  return 60;
}
var yyServerInfoTick = null;
// 间隔文案 + 倒计时：间隔只认现有轮询配置 yy-server-refresh-seconds（30-60 秒，默认 60），
// 不另立一套数值。排程没在跑（离开设置页、宿主没了）时显示「手动刷新」，不假装还在自动刷新。
function yyServerRefreshLabel(text) {
  const label = document.getElementById('yy-server-refresh-label');
  if (label) label.textContent = text;
}
function yyServerRefreshCountdown(host) {
  clearInterval(yyServerInfoTick);
  yyServerInfoTick = null;
  const tick = () => {
    if (!host || typeof host.yyLive !== 'function' || !host.yyLive() || !host.yyNextAt) {
      yyServerRefreshLabel('在线玩家 · 手动刷新');
      clearInterval(yyServerInfoTick);
      yyServerInfoTick = null;
      return;
    }
    const left = Math.max(0, Math.round((host.yyNextAt - Date.now()) / 1000));
    yyServerRefreshLabel('在线玩家 · 每 ' + (host.yyEverySeconds || yyServerRefreshSeconds()) + ' 秒自动刷新 · ' + left + ' 秒后');
  };
  tick();
  yyServerInfoTick = setInterval(tick, 1000);
}
function yyScheduleServerInfo(host, live) {
  clearTimeout(host.yyTimer);
  if (!live()) { host.yyNextAt = 0; yyServerRefreshCountdown(host); return; }
  const slider = document.getElementById('yy-server-refresh-seconds');
  const asked = slider ? Number(slider.value) : NaN;
  host.yyEverySeconds = Number.isInteger(asked) && asked >= 30 && asked <= 60 ? asked : yyServerRefreshSeconds();
  host.yyNextAt = Date.now() + host.yyEverySeconds * 1000;
  yyServerRefreshCountdown(host);
  host.yyTimer = setTimeout(async () => {
    host.yyTimer = null;
    if (!live()) return;
    await yyRefreshConnectionPolicy(true);
    yyScheduleServerInfo(host,live);
  },host.yyEverySeconds * 1000);
}
// 刷新按钮：真的重读连接策略 + /api/bot/status + 服务器信息，带 loading 态与失败提示。
async function yyRefreshServerInfoNow(button) {
  const btn = button || document.getElementById('yy-server-refresh-btn');
  const original = btn ? btn.textContent : '';
  if (btn) { btn.disabled = true; btn.textContent = '刷新中…'; }
  try {
    await yyRefreshConnectionPolicy(true);
    if (!document.getElementById('yy-connection-server-info')) return;
    const note = document.getElementById('yy-connection-server-note');
    const text = note ? note.textContent : '';
    if (/无法|失败|缺少|读取失败/.test(text)) T(text || '刷新失败，请稍后重试', 'e');
    else T('已刷新', 'o');
  } catch (e) {
    T((e && e.message) || '刷新失败，请稍后重试', 'e');
  } finally {
    if (btn && btn.isConnected) { btn.disabled = false; btn.textContent = original || '刷新'; }
    const host = document.getElementById('yy-connection-server-info');
    if (host && host.yyLive) yyScheduleServerInfo(host, host.yyLive);
  }
}
function yyChangeServerRefresh(input) {
  const value = Number(input.value);
  if (!Number.isInteger(value) || value < 30 || value > 60) return;
  try { localStorage.setItem('yy-server-refresh-seconds',String(value)); } catch (_) {}
  const host = document.getElementById('yy-connection-server-info');
  if (host && host.yyLive) yyScheduleServerInfo(host,host.yyLive);
}
async function yyResumeReconnect() {
  const r = await A('POST', '/api/youyuan/connection-policy', {resume:true});
  if (!r.ok) { T(r.error || '恢复失败', 'e'); return; }
  const box = document.getElementById('cfg-auto-reconnect'); if (box) box.checked = true;
  yySettingsData.auto_reconnect = true; yyDrawConnectionPolicy(r.policy); T('已恢复自动重连', 'o');
}
var yyConnectionPolicySettingsBase = Rcfg;
Rcfg = async function() {if(yyServerInfoCleanup)yyServerInfoCleanup();var yySettingsStillLive=yySettingsViewGuard();await yyConnectionPolicySettingsBase();if(!yySettingsStillLive())return;
  for (const [id,field] of [['cfg-auto-connect','auto_connect'],['cfg-auto-reconnect','auto_reconnect'],['cfg-auto-creative','auto_creative']]) {
    const input = document.getElementById(id);
    if (input) input.onchange = () => yyChangeConnectionPolicy(input,field);
  }
  const card = document.getElementById('cfg-conn-card');
  if (card) card.querySelectorAll('.yy-switch-row').forEach(el => el.classList.add('yy-policy-switch'));
  // 服务器信息独立成宽幅 hero，放在「终端 / 角色等级与经验」快捷行下方，
  // 不再塞进「服务器连接」卡片顶部。
  if (card && !document.getElementById('yy-connection-server-info')) {
    const hero = '<section id="yy-connection-server-info" class="yy-server-hero" aria-label="当前服务器"><div id="yy-connection-server-card" class="yy-hero-host"></div><div class="yy-server-hero-foot"><p id="yy-connection-policy-state" class="yy-hint yy-server-state" role="status">正在读取连接状态…</p><p id="yy-connection-server-note" class="yy-hint yy-server-note" role="status"></p><div class="yy-room-current-actions"><button id="yy-resume-reconnect" class="btn-s" hidden onclick="yyResumeReconnect()">恢复自动重连</button><button type="button" class="yy-text-btn" id="yy-server-refresh-btn" onclick="yyRefreshServerInfoNow(this)">刷新</button><label class="yy-room-refresh" title="自动刷新间隔，可在 30–60 秒之间调整"><span id="yy-server-refresh-label">在线玩家 · 每 '+yyServerRefreshSeconds()+' 秒自动刷新</span><input id="yy-server-refresh-seconds" type="range" min="30" max="60" step="1" value="'+yyServerRefreshSeconds()+'" aria-label="在线玩家更新间隔，30到60秒" oninput="yyChangeServerRefresh(this)"></label></div></div></section>';
    const root = document.querySelector('#tab-settings>.content-inner');
    const grid = root && root.querySelector(':scope>.yy-form-grid');
    if (grid) grid.insertAdjacentHTML('beforebegin', hero);
    else if (root) root.insertAdjacentHTML('afterbegin', hero);
    else card.insertAdjacentHTML('afterbegin', hero);
  }
  const host = document.getElementById('yy-connection-server-info');
  if (!host) return;
  host.yyLive = () => !host.yyDisposed && yySettingsStillLive() && document.getElementById('yy-connection-server-info') === host;
  const expand = () => {
    if (!host.yyLive()) return false;
    let found = false;
    for(let parent=host.parentElement;parent;parent=parent.parentElement)if(parent.tagName==='DETAILS'){parent.open=true;found=true;}
    return found;
  };
  const observer = new MutationObserver(() => { if(expand())observer.disconnect(); });
  if (!expand()) observer.observe(document.getElementById('tab-settings'),{childList:true,subtree:true});
  yyServerInfoCleanup = () => { host.yyDisposed = true; clearTimeout(host.yyTimer); clearInterval(yyServerInfoTick); yyServerInfoTick = null; observer.disconnect(); host.yyRun = (host.yyRun || 0) + 1; };
  activeRenderers.push(yyServerInfoCleanup);
  const label = document.querySelector('label[for="bot-server"]');
  if (label) label.textContent = '服务器 / 分享链接';
  yyScheduleServerInfo(host,host.yyLive);
  yyRefreshConnectionPolicy();
};
var yyPolicyAccountRefreshBase = refreshCfgAccount;
refreshCfgAccount = async function() {
  await yyPolicyAccountRefreshBase();
  if (document.getElementById('yy-connection-policy-state')) await yyRefreshConnectionPolicy(false);
};
