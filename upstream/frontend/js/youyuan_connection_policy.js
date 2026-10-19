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
async function yyRefreshConnectionPolicy() {
  const account = yyActiveAccount;
  try { const r = await A('GET', '/api/youyuan/connection-policy'); if (r.ok && account === yyActiveAccount) yyDrawConnectionPolicy(r.policy); } catch (_) {}
}
async function yyResumeReconnect() {
  const r = await A('POST', '/api/youyuan/connection-policy', {resume:true});
  if (!r.ok) { T(r.error || '恢复失败', 'e'); return; }
  const box = document.getElementById('cfg-auto-reconnect'); if (box) box.checked = true;
  yySettingsData.auto_reconnect = true; yyDrawConnectionPolicy(r.policy); T('已恢复自动重连', 'o');
}
var yyConnectionPolicySettingsBase = Rcfg;
Rcfg = async function() {var yySettingsStillLive=yySettingsViewGuard();await yyConnectionPolicySettingsBase();if(!yySettingsStillLive())return;
  for (const [id,field] of [['cfg-auto-connect','auto_connect'],['cfg-auto-reconnect','auto_reconnect'],['cfg-auto-creative','auto_creative']]) {
    const input = document.getElementById(id);
    if (input) input.onchange = () => yyChangeConnectionPolicy(input,field);
  }
  const card = document.getElementById('cfg-conn-card');
  if (card && !document.getElementById('yy-connection-policy-state')) {
    card.insertAdjacentHTML('afterbegin','<div class="yy-connection-summary"><p id="yy-connection-policy-state" role="status">正在读取连接策略…</p><button id="yy-resume-reconnect" class="btn-s" hidden onclick="yyResumeReconnect()">恢复自动重连</button><button class="yy-text-btn" onclick="yyRefreshConnectionPolicy()">刷新状态</button></div>');
    card.querySelectorAll('.yy-switch-row').forEach(el => el.classList.add('yy-policy-switch'));
  }
  await yyRefreshConnectionPolicy();
};
var yyPolicyAccountRefreshBase = refreshCfgAccount;
refreshCfgAccount = async function() {
  await yyPolicyAccountRefreshBase();
  if (document.getElementById('yy-connection-policy-state')) await yyRefreshConnectionPolicy();
};
