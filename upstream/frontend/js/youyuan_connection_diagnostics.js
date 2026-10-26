/* 连接诊断面板：把「连上之后又被服务器踢下线」变成设置页里看得见、看得懂的事件。
 *
 * 背景（2026-10-23 掉线报障）：
 *   - /api/bot/status 只有瞬时布尔，没有任何掉线历史/计数/原因；
 *   - 登录成功后再掉线会清空 failures/lastError，痕迹被抹掉；
 *   - 唯一有原因的 connection-errors.log 此前无任何 API 暴露。
 * 用户体感：bot 动不动掉线，工具箱里查不到，刷新也没用。
 * 本模块只读 /api/youyuan/connection-diagnostics，不做任何写操作。
 *
 * 2026-10-23 UI 整改（用户截图反馈「放这么大、占重要位置、甚至错位」）：
 *   - 不再蹭房间发现的宽幅 hero 样式（.yy-server-hero 的 20px 圆角、大阴影、
 *     渐变底、nowrap 操作行都是给服务器信息卡的，诊断面板套上就是「这么大 +
 *     错位」），改用普通卡片 .yy-conn-diag-card，样式全部重写在本文件同款 CSS 里。
 *   - 位置：优先插在 #yy-connection-server-info 之后；anchor 不在时沉到设置页
 *     内容区末尾（beforeend），绝不再 afterbegin 抢最顶部。
 *   - 默认折叠成 <details>：摘要行只放在线/离线徽章 + 今日掉线次数 + 「展开详情」。
 *     展开才拉数据、才启动 15 秒定时刷新；收起立即停表，不为看不见的面板烧流量。
 */
(function () {
  'use strict';

  var REFRESH_MS = 15000;
  var OPEN_KEY = 'yy-conn-diag-open';
  var host = null, timer = null, live = null, visible = null, run = 0, lastData = null;

  function field(id) { return document.getElementById(id); }
  function esc(v) {
    return String(v == null ? '' : v).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function duration(seconds) {
    var s = Math.max(0, Math.round(Number(seconds) || 0));
    if (s < 60) return s + ' 秒';
    if (s < 3600) return Math.floor(s / 60) + ' 分 ' + (s % 60) + ' 秒';
    var h = Math.floor(s / 3600);
    if (h < 24) return h + ' 小时 ' + Math.floor((s % 3600) / 60) + ' 分';
    return Math.floor(h / 24) + ' 天 ' + (h % 24) + ' 小时';
  }
  function clock(iso) {
    var t = new Date(iso);
    if (isNaN(t.getTime())) return '—';
    var p = function (n) { return (n < 10 ? '0' : '') + n; };
    return p(t.getMonth() + 1) + '-' + p(t.getDate()) + ' ' + p(t.getHours()) + ':' + p(t.getMinutes()) + ':' + p(t.getSeconds());
  }

  var TONE = {
    banned: 'e', server_kicked: 'e', server_unreachable: 'w',
    idle_timeout: 'w', local_network: 'w', conn_closed: 'w',
    unknown: 'w', connection_error: 'w'
  };

  function stat(label, strongId, smallId) {
    return '<div class="yy-diag-stat"><label>' + esc(label) + '</label><strong id="' + strongId + '">—</strong><small id="' + smallId + '"></small></div>';
  }

  function draw(d) {
    if (!d) return;
    lastData = d;
    // 用 visible 而不是 live：折叠时摘要行（徽章 + 今日掉线）也是可见的，
    // 重建后即使没展开也要把摘要画对，不能让摘要停在「未读取」。
    if (!host || !visible || !visible()) return;
    var badge = field('yy-diag-badge');
    if (badge) {
      badge.textContent = d.connected ? '在线' : '离线';
      badge.className = 'yy-badge' + (d.connected ? ' success' : '');
    }
    // 摘要行（折叠时也可见）的今日掉线次数，与详情格里的是同一个数。
    set('yy-diag-today-sum', String(d.drops_today == null ? 0 : d.drops_today));
    set('yy-diag-state', d.connected ? '已连接' : '未连接');
    set('yy-diag-uptime', d.connected ? '已连续在线 ' + duration(d.uptime_seconds) : '等待自动重连');
    set('yy-diag-today', String(d.drops_today == null ? 0 : d.drops_today));
    set('yy-diag-total', '累计 ' + (d.drops_total == null ? 0 : d.drops_total) + ' 次 · 最长连续在线 ' + duration(d.longest_uptime));
    set('yy-diag-last', d.last_drop_at ? clock(d.last_drop_at) : '今日暂无');
    set('yy-diag-last-reason', d.last_drop_label || '—');
    set('yy-diag-best', duration(d.longest_uptime));
    set('yy-diag-rtt', d.rtt_ms == null ? '延迟测量中' : 'RTT ' + Math.round(d.rtt_ms) + ' ms');
    var assess = field('yy-diag-assessment');
    if (assess) assess.textContent = d.assessment || '';
    var hint = field('yy-diag-hint');
    if (hint) hint.textContent = d.last_drop_hint || (d.drops_today ? '' : '今日暂无掉线，连接稳定。');

    var dist = field('yy-diag-dist');
    if (dist) {
      var by = d.log_by_reason || {};
      var keys = Object.keys(by).sort(function (a, b) { return by[b] - by[a]; });
      dist.innerHTML = keys.length
        ? '今日原因分布：' + keys.map(function (k) {
            var label = (d.log_events || []).filter(function (r) { return r.reason === k; }).pop();
            return esc((label && label.label) || k) + ' ' + by[k] + ' 次';
          }).join(' · ')
        : '今日暂无掉线记录。';
    }
    // 重连策略：以前这几行只长在「连接中」弹窗的诊断折叠里（pages/settings.js
    // 的 yyConnDrawDiag），同一个概念两套渲染。统一到本卡片后，弹窗那套可以删。
    var policy = d.policy || {};
    var retry = policy.effective_retry_seconds != null ? policy.effective_retry_seconds : policy.reconnect_seconds;
    set('yy-diag-policy', '重连策略 · 自动连接 ' + (policy.auto_connect ? '开' : '关') +
      ' · 自动重连 ' + (policy.auto_reconnect ? '开' : '关') +
      ' · 重试间隔 ' + (retry == null ? '—' : duration(retry)));

    var list = field('yy-diag-events');
    if (list) {
      var events = (d.events || []).slice(0, 8);
      if (!events.length) {
        list.innerHTML = '<p class="yy-hint" style="margin:0">本次启动后还没有记录到掉线。掉线事件会在这里列出时间、持续时长与原因。</p>';
      } else {
        list.innerHTML = '<div class="yy-diag-list">' + events.map(function (e) {
          var tone = TONE[e.reason] || 'w';
          var away = e.away_seconds ? ' · 恢复用了 ' + duration(e.away_seconds) : ' · 未恢复';
          return '<div class="yy-diag-row tone-' + tone + '">' +
            '<span class="yy-diag-time">' + esc(clock(e.at)) + '</span>' +
            '<span class="yy-diag-body"><strong>' + esc(e.reason_label || e.label || e.reason) + '</strong>' +
            '<small>在线 ' + duration(e.uptime_seconds) + away + (e.server ? ' · ' + esc(e.server) : '') + '</small>' +
            (e.hint ? '<small class="yy-diag-hintline">' + esc(e.hint) + '</small>' : '') +
            '</span></div>';
        }).join('') + '</div>';
      }
    }
    var log = field('yy-diag-log');
    if (log) log.textContent = d.log_path ? ('日志 ' + d.log_path + ' · 已解析 ' + (d.log_events || []).length + ' 条') : '';
  }
  function set(id, text) { var el = field(id); if (el) el.textContent = text; }

  async function load(force) {
    if (!host || !live()) return;
    var current = ++run;
    try {
      var d = await A('GET', '/api/youyuan/connection-diagnostics');
      if (current !== run || !live()) return;
      if (d && d.ok) draw(d);
      else if (field('yy-diag-assessment')) field('yy-diag-assessment').textContent = (d && d.error) || '诊断读取失败';
    } catch (e) {
      if (current === run && field('yy-diag-assessment')) field('yy-diag-assessment').textContent = '诊断读取失败：' + (e && e.message ? e.message : e);
    }
    if (force && field('yy-diag-badge')) T('诊断已刷新', 'o');
  }

  function schedule() {
    clearTimeout(timer);
    if (!live()) return;
    timer = setTimeout(function () { timer = null; if (live()) { load(false); schedule(); } }, REFRESH_MS);
  }

  // details 展开/收起：展开才拉数据并排 15 秒刷新；收起立刻停表。
  // 展开状态额外记一份，因为设置页整块 innerHTML 会被 Rcfg 重建
  // （youyuan_settings.js:28），重建后按上次状态恢复，用户不会觉得页面被重置。
  function toggle(el) {
    if (!el || el !== host) return;
    try { sessionStorage.setItem(OPEN_KEY, el.open ? '1' : '0'); } catch (_) {}
    if (el.open) { load(false); schedule(); }
    else { clearTimeout(timer); }
  }
  window.yyConnDiagToggle = toggle;

  var base = Rcfg;
  Rcfg = async function () {
    if (host && host.yyCleanup) host.yyCleanup();
    await base();
    var guard = typeof yySettingsViewGuard === 'function' ? yySettingsViewGuard() : function () { return true; };
    var anchor = document.getElementById('yy-connection-server-info');
    var root = document.querySelector('#tab-settings>.content-inner');
    if (!root) return;
    // 关键：卡片只在「不存在」时建一次。以前用 id="yy-conn-diag"，而「连接中」
    // 弹窗（pages/settings.js:478）里也有一块 <details id="yy-conn-diag">，
    // 弹窗开过一次后 getElementById 就会命中弹窗那一块，卡片这边拿到错误的 host，
    // 于是有时渲染成一张、有时渲染成另一张——用户说的「好像是两套系统」。
    // 现在卡片 id 独占为 yy-conn-diag-card，彻底不再抢同一个节点。
    if (document.getElementById('yy-conn-diag-card')) { host = document.getElementById('yy-conn-diag-card'); }    else {
      var html = '<details id="yy-conn-diag-card" class="yy-conn-diag-card" ontoggle="yyConnDiagToggle(this)">' +
        '<summary class="yy-conn-diag-sum">' +
        '<span class="yy-conn-diag-title">连接诊断</span>' +
        '<span class="yy-badge" id="yy-diag-badge">未读取</span>' +
        '<span class="yy-conn-diag-count">今日掉线 <b id="yy-diag-today-sum">—</b> 次</span>' +
        '<span class="yy-conn-diag-caret">展开详情<i></i></span>' +
        '</summary>' +
        '<div class="yy-conn-diag-body">' +
        '<div class="yy-diag-grid">' +
        stat('当前状态', 'yy-diag-state', 'yy-diag-uptime') +
        stat('今日掉线', 'yy-diag-today', 'yy-diag-total') +
        stat('最近掉线', 'yy-diag-last', 'yy-diag-last-reason') +
        stat('最长连续在线', 'yy-diag-best', 'yy-diag-rtt') +
        '</div>' +
        '<p class="yy-hint yy-conn-diag-assess" id="yy-diag-assessment" role="status">展开后自动读取连接诊断…</p>' +
        '<p class="yy-hint" id="yy-diag-hint" role="status"></p>' +
        '<p class="yy-hint" id="yy-diag-dist" role="status"></p>' +
        '<p class="yy-hint" id="yy-diag-policy" role="status"></p>' +
        '<div id="yy-diag-events"></div>' +
        '<div class="yy-conn-diag-actions"><button type="button" class="yy-text-btn" onclick="yyRefreshConnDiag(true)">刷新诊断</button><span class="yy-hint yy-conn-diag-log" id="yy-diag-log"></span></div>' +
        '</div></details>';
      // 位置：服务器信息 hero 之后优先；hero 不在就沉到内容区末尾，绝不抢设置页最顶部。
      if (anchor && anchor.parentElement === root) anchor.insertAdjacentHTML('afterend', html);
      else root.insertAdjacentHTML('beforeend', html);
      host = document.getElementById('yy-conn-diag-card');
    }
    visible = function () {
      return !host.yyDisposed && guard() && document.getElementById('yy-conn-diag-card') === host;
    };
    live = function () { return !!visible() && !!host.open; };
    host.yyDisposed = false;
    host.yyCleanup = function () { host.yyDisposed = true; clearTimeout(timer); run++; };
    if (typeof activeRenderers !== 'undefined') activeRenderers.push(host.yyCleanup);
    // 设置页整块 innerHTML 会被 Rcfg 重建（youyuan_settings.js:28），卡片每次都是新节点。
    // 所以让它「看起来没被重置」：
    //   1) 展开状态按上次记住的恢复（第一次进仍是默认折叠）；
    //   2) 内存里有上一份数据就立刻画回去，不出现一片「未读取」等接口的空窗。
    var wantOpen = false;
    try { wantOpen = sessionStorage.getItem(OPEN_KEY) === '1'; } catch (_) {}
    if (wantOpen && !host.open) host.open = true;   // 会触发 ontoggle → load + schedule
    if (lastData) draw(lastData);
    if (host.open) schedule();
    else clearTimeout(timer);
  };

  window.yyRefreshConnDiag = function (force) {
    // 折叠状态下被调用：先展开（toggle 事件随后也会 load+schedule），再立即刷一次。
    if (host && !host.open) host.open = true;
    load(!!force);
    schedule();
  };
})();
