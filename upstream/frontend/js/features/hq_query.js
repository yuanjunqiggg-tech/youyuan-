// ═══════════════════════════════════════
// 总部查询器（HQ Query）— 本机计算 + 渲染
// ═══════════════════════════════════════
//
// 设计（照用户原话）：
//   - 默认不连机器人、不在云端跑计算。打开即用「本地内置快照（演示数据，形状与
//     /api/youyuan/world/hq 完全一致）」渲染，零网络请求。
//   - 聚合（利用率 / 健康分 / 排序 / 汇总）一律本机算；接口只给原始 rooms。
//   - 只有用户点「尝试从本地网关拉取」才发一次只读 GET；拉不到就保留本地快照。
//   - 房间利用率柱状图用 Canvas 本机绘制。
//
// API 契约见 upstream/skills/hq-query/index.md

(function () {
  'use strict';

  // 本地内置快照：演示数据，形状与接口返回一致。仅用于离线渲染，页面会明确标注。
  // 真实数据由 /api/youyuan/world/hq 提供（见 skills/hq-query/index.md）。
  var HQ_DEMO = {
    ok: true,
    hq: {
      id: 'hq-yy-cn-east',
      name: '游园总部 · 华东厅',
      region: 'cn-east',
      edition: 'bedrock',
      version: '1.21.50',
      discovered_at: '2026-10-25T08:00:00+08:00',
      source: 'demo'
    },
    sampled_at: '2026-10-25T08:00:00+08:00',
    rooms: [
      { id: 'r-101', name: '生存主世界', mode: 'survival', map: 'overworld', status: 'online', players_online: 42, capacity: 50, latency_ms: 36, uptime_pct: 99.2 },
      { id: 'r-102', name: '创造工坊', mode: 'creative', map: 'flat', status: 'online', players_online: 18, capacity: 30, latency_ms: 41, uptime_pct: 98.7 },
      { id: 'r-103', name: '极限生存', mode: 'survival', map: 'overworld', status: 'full', players_online: 60, capacity: 60, latency_ms: 58, uptime_pct: 97.1 },
      { id: 'r-104', name: '起床战争', mode: 'minigame', map: 'bedwars', status: 'online', players_online: 24, capacity: 40, latency_ms: 52, uptime_pct: 96.4 },
      { id: 'r-105', name: '空岛生存', mode: 'survival', map: 'skyblock', status: 'maintenance', players_online: 0, capacity: 25, latency_ms: 0, uptime_pct: 90.0 },
      { id: 'r-106', name: '冒险剧情', mode: 'adventure', map: 'story', status: 'offline', players_online: 0, capacity: 20, latency_ms: 0, uptime_pct: 81.3 },
      { id: 'r-107', name: '建筑展厅', mode: 'creative', map: 'gallery', status: 'online', players_online: 9, capacity: 30, latency_ms: 47, uptime_pct: 99.0 }
    ]
  };

  var hqState = {
    data: null,        // 当前数据源（演示 or 实时）
    live: false,       // 是否来自实时拉取
    sort: 'players'
  };

  function $(id) { return document.getElementById(id); }

  function esc(v) {
    return String(v == null ? '' : v).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  function toast(msg, kind) {
    if (typeof T === 'function') T(msg, kind || 'e');
  }

  function apiGet(path) {
    if (typeof A === 'function') return A('GET', path);
    return Promise.reject(new Error('当前页面没有可用的接口通道'));
  }

  var MODE_LABEL = {
    survival: '生存', creative: '创造', minigame: '小游戏', adventure: '冒险'
  };
  var STATUS_LABEL = {
    online: '在线', full: '满员', maintenance: '维护中', offline: '离线'
  };
  var STATUS_CLASS = {
    online: 'hq-on', full: 'hq-full', maintenance: 'hq-maint', offline: 'hq-off'
  };

  // ── 本机聚合（与 skills/hq-query/index.md 公式一致；不在云端算） ──
  function hqCompute(data, sort) {
    var rooms = (data && data.rooms) ? data.rooms.slice() : [];
    var total_rooms = rooms.length;
    var online_rooms = 0, full_rooms = 0, offline_rooms = 0;
    var total_players = 0, total_capacity = 0, latSum = 0, latN = 0;
    for (var i = 0; i < rooms.length; i++) {
      var r = rooms[i];
      if (r.status === 'offline') offline_rooms++;
      else { online_rooms++; if (r.status === 'full') full_rooms++; }
      total_players += (r.players_online || 0);
      total_capacity += (r.capacity || 0);
      if (r.status !== 'offline' && r.status !== 'maintenance') {
        latSum += (r.latency_ms || 0); latN++;
      }
    }
    var utilization = total_capacity > 0 ? total_players / total_capacity : 0;
    var avg_latency = latN > 0 ? latSum / latN : 0;
    var offlineRatio = total_rooms > 0 ? offline_rooms / total_rooms : 0;
    var overload = utilization > 0.85 ? (utilization - 0.85) / 0.15 : 0;
    var latPenalty = avg_latency > 200 ? (avg_latency - 200) / 300 : 0;
    var health = 100 - 40 * offlineRatio - 25 * Math.max(0, overload) - 20 * Math.max(0, latPenalty);
    if (health < 0) health = 0;
    if (health > 100) health = 100;

    if (sort === 'latency') rooms.sort(function (a, b) { return (a.latency_ms || 0) - (b.latency_ms || 0); });
    else if (sort === 'name') rooms.sort(function (a, b) { return String(a.name).localeCompare(String(b.name)); });
    else rooms.sort(function (a, b) { return (b.players_online || 0) - (a.players_online || 0); });

    return {
      total_rooms: total_rooms,
      online_rooms: online_rooms,
      full_rooms: full_rooms,
      offline_rooms: offline_rooms,
      total_players: total_players,
      total_capacity: total_capacity,
      utilization: utilization,
      avg_latency: avg_latency,
      health: health,
      rooms: rooms
    };
  }

  function hqSummaryHTML(agg) {
    function pct(v) { return (v * 100).toFixed(1) + '%'; }
    var healthCls = agg.health >= 80 ? 'hq-h-good' : agg.health >= 60 ? 'hq-h-mid' : 'hq-h-bad';
    return '' +
      '<div class="hq-summary">' +
        hqStat('总部房间', agg.total_rooms, '') +
        hqStat('在线 / 离线', agg.online_rooms + ' / ' + agg.offline_rooms, '') +
        hqStat('总在线 / 容量', agg.total_players + ' / ' + agg.total_capacity, '') +
        hqStat('利用率', pct(agg.utilization), agg.utilization > 0.85 ? 'hq-warn' : '') +
        hqStat('平均延迟', Math.round(agg.avg_latency) + ' ms', agg.avg_latency > 200 ? 'hq-warn' : '') +
        '<div class="hq-stat"><div class="hq-stat-v ' + healthCls + '">' + Math.round(agg.health) + '</div><div class="hq-stat-l">健康分</div></div>' +
      '</div>';
  }
  function hqStat(label, value, extra) {
    return '<div class="hq-stat"><div class="hq-stat-v ' + (extra || '') + '">' + esc(value) + '</div><div class="hq-stat-l">' + esc(label) + '</div></div>';
  }

  function hqTableHTML(agg) {
    var rows = '';
    for (var i = 0; i < agg.rooms.length; i++) {
      var r = agg.rooms[i];
      var u = r.capacity > 0 ? r.players_online / r.capacity : 0;
      rows += '<tr>' +
        '<td class="hq-name">' + esc(r.name) + '<small>' + esc(MODE_LABEL[r.mode] || r.mode) + ' · ' + esc(r.map) + '</small></td>' +
        '<td><span class="hq-badge ' + (STATUS_CLASS[r.status] || '') + '">' + esc(STATUS_LABEL[r.status] || r.status) + '</span></td>' +
        '<td class="hq-num">' + (r.players_online || 0) + ' / ' + (r.capacity || 0) + '</td>' +
        '<td class="hq-bar"><div class="hq-bar-track"><div class="hq-bar-fill" style="width:' + Math.min(100, u * 100).toFixed(1) + '%"></div></div><small>' + (u * 100).toFixed(0) + '%</small></td>' +
        '<td class="hq-num">' + (r.status === 'offline' ? '—' : (r.latency_ms || 0) + ' ms') + '</td>' +
        '<td class="hq-num">' + (r.uptime_pct != null ? r.uptime_pct.toFixed(1) + '%' : '—') + '</td>' +
        '</tr>';
    }
    return '<div class="card hq-table-card"><table class="hq-table"><thead><tr>' +
      '<th>房间</th><th>状态</th><th>在线/容量</th><th>利用率</th><th>延迟</th><th>在线率</th>' +
      '</tr></thead><tbody>' + rows + '</tbody></table></div>';
  }

  // Canvas 本机绘制：每房间利用率柱状图
  function hqRenderChart(canvas, agg) {
    if (!canvas) return;
    var dpr = (window.devicePixelRatio || 1);
    var W = canvas.clientWidth || 320, H = 180;
    canvas.width = W * dpr; canvas.height = H * dpr;
    canvas.style.height = H + 'px';
    var ctx = canvas.getContext('2d');
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, W, H);
    var rooms = agg.rooms.filter(function (r) { return r.status !== 'offline'; });
    if (!rooms.length) {
      ctx.fillStyle = '#64748b'; ctx.font = '12px sans-serif'; ctx.textAlign = 'center';
      ctx.fillText('无在线房间可绘制', W / 2, H / 2);
      return;
    }
    var padL = 8, padR = 8, padT = 10, padB = 22;
    var plotW = W - padL - padR, plotH = H - padT - padB;
    var n = rooms.length, gap = 6;
    var bw = (plotW - gap * (n - 1)) / n;
    ctx.strokeStyle = 'rgba(148,163,184,0.25)';
    ctx.beginPath(); ctx.moveTo(padL, padT + plotH); ctx.lineTo(W - padR, padT + plotH); ctx.stroke();
    for (var i = 0; i < n; i++) {
      var r = rooms[i];
      var u = r.capacity > 0 ? r.players_online / r.capacity : 0;
      var bh = Math.max(2, u * plotH);
      var x = padL + i * (bw + gap);
      var y = padT + plotH - bh;
      var col = u > 0.85 ? '#ef4444' : u > 0.6 ? '#f59e0b' : '#22c55e';
      ctx.fillStyle = col;
      ctx.fillRect(x, y, bw, bh);
      ctx.fillStyle = '#94a3b8'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
      var label = r.name.length > 5 ? r.name.slice(0, 5) : r.name;
      ctx.fillText(label, x + bw / 2, H - 8);
      ctx.fillText(Math.round(u * 100) + '%', x + bw / 2, y - 3);
    }
  }

  function hqRender() {
    var body = $('hq-body');
    if (!body) return;
    var data = hqState.data || HQ_DEMO;
    var agg = hqCompute(data, hqState.sort);

    var srcNote = hqState.live
      ? '<span class="hq-src-live">实时 · 来自本地网关 GET /api/youyuan/world/hq</span>'
      : '<span class="hq-src-demo">本地演示快照（未联网）· 形状与接口一致</span>';

    var html = '' +
      '<div class="card hq-head">' +
        '<div class="hq-hq">' +
          '<div class="hq-hq-name">' + esc((data.hq && data.hq.name) || '未知总部') + '</div>' +
          '<div class="hq-hq-meta">' + esc((data.hq && data.hq.region) || '') + ' · ' + esc((data.hq && data.hq.version) || '') + ' · ' + srcNote + '</div>' +
        '</div>' +
        '<div class="row hq-actions">' +
          '<button class="btn" id="hq-fetch" onclick="hqQueryFetchLive()">尝试从本地网关拉取</button>' +
          '<select id="hq-sort" class="hq-sel" onchange="hqQuerySetSort(this.value)">' +
            '<option value="players"' + (hqState.sort === 'players' ? ' selected' : '') + '>按在线人数</option>' +
            '<option value="latency"' + (hqState.sort === 'latency' ? ' selected' : '') + '>按延迟</option>' +
            '<option value="name"' + (hqState.sort === 'name' ? ' selected' : '') + '>按名称</option>' +
          '</select>' +
        '</div>' +
      '</div>' +
      hqSummaryHTML(agg) +
      '<div class="card hq-chart-card"><div class="hq-chart-title">在线房间利用率（本机绘制）</div><canvas id="hq-chart"></canvas></div>' +
      hqTableHTML(agg);

    body.innerHTML = html;
    hqRenderChart($('hq-chart'), agg);
  }

  function hqQuerySetSort(v) {
    hqState.sort = v;
    hqRender();
  }

  async function hqQueryFetchLive() {
    var btn = $('hq-fetch');
    if (btn) { btn.disabled = true; btn.textContent = '拉取中…'; }
    try {
      var r = await apiGet('/api/youyuan/world/hq');
      if (r && r.ok && r.rooms) {
        hqState.data = r;
        hqState.live = true;
        toast('已拉取实时总部快照（只读）', 'o');
      } else {
        hqState.live = false;
        toast('未获取到总部数据：' + ((r && r.error) || '未知'), 'e');
      }
    } catch (e) {
      hqState.live = false;
      toast('拉取失败（已保留本地快照）：' + e.message, 'e');
    } finally {
      if (btn) { btn.disabled = false; btn.textContent = '尝试从本地网关拉取'; }
      hqRender();
    }
  }

  function RhqQueryHTML() {
    // 首屏骨架：默认用本地快照渲染，不做任何网络请求。
    hqState.data = hqState.data || HQ_DEMO;
    setTimeout(function () { hqRender(); }, 30);
    return '<div class="card hq-wrap">' +
      '<div id="hq-body">' +
        '<div class="hq-loading">加载中…</div>' +
      '</div>' +
    '</div>';
  }

  // 缓存恢复时补跑渲染（与 others.js 里其它工具一致）。
  setTimeout(function () {
    if (typeof subPageInit !== 'function') return;
    subPageInit('总部查询器', function () {
      hqState.data = hqState.data || HQ_DEMO;
      hqRender();
    });
  }, 0);

  // 暴露给全局（others.js 卡片 act 调用）
  window.RhqQueryHTML = RhqQueryHTML;
  window.hqQueryRender = hqRender;
  window.hqQueryFetchLive = hqQueryFetchLive;
  window.hqQuerySetSort = hqQuerySetSort;
})();
