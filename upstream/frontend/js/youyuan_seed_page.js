/* 种子查询 · 独立页面（一级入口，与「建筑导入 / 建筑导出」同级）
 *
 * 交互目标（照用户原话）：
 *   点进去就能看到地图式内容 → 自己选版本号 → 拖动/缩放/双击看坐标。
 *   所有计算和渲染都在本机（WebView / APK / Linux 客户端）完成，不打服务端；
 *   服务端只负责「读当前世界种子」这一个低频接口。
 *
 * 依赖：js/youyuan_seed_local.js（YYSeedLocal，本机算法）
 *       app.js 的 openSubPage / activeRenderers、youyuan_accounts.js 的 yyRootAPI
 */
(function () {
  'use strict';

  if (typeof YYSeedLocal === 'undefined') {
    console.warn('种子查询：缺少 youyuan_seed_local.js，页面未加载');
    return;
  }

  var PAGE_TITLE = '种子查询';
  var COLORS = {
    slime: 'rgba(76, 175, 96, 0.45)',
    slimeEdge: 'rgba(56, 142, 60, 0.85)',
    village: '#2f9e5f',
    fortress: '#b0432f',
    bastion: '#8a5a2b',
    monument: '#2f6f9e',
    mansion: '#7a4b9e',
    temple: '#a8862a',
    stronghold: '#d0453a'
  };
  var TYPE_LABEL = {
    slime: '史莱姆区块', village: '村庄', stronghold: '要塞',
    monument: '海底神殿', mansion: '林地府邸', fortress: '下界要塞',
    bastion: '堡垒遗迹', temple: '神庙'
  };
  // 面板顺序：用户点名要的 6 个在前，另外两个是 Go 侧真实实现了的（堡垒遗迹 / 神庙）。
  var CHIP_ORDER = ['slime', 'village', 'stronghold', 'monument', 'mansion', 'fortress', 'bastion', 'temple'];

  var S = null;          // 页面状态（每次打开重建）
  var rafId = 0;
  var cleanup = [];

  /* ── 小工具 ─────────────────────────────────────────────────────── */

  function $(id) { return document.getElementById(id); }

  function esc(v) {
    return String(v == null ? '' : v).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  function clamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }

  function fmtInt(n) { return String(Math.round(n)); }

  function apiGet(path) {
    if (typeof yyRootAPI === 'function') return yyRootAPI('GET', path);
    if (typeof A === 'function') return A('GET', path);
    return Promise.reject(new Error('当前页面没有可用的接口通道'));
  }

  function toast(msg, kind) {
    if (typeof T === 'function') T(msg, kind || 'e');
  }

  function copyText(text, okMsg) {
    var done = function () { toast(okMsg || ('已复制：' + text), 'o'); };
    var fallback = function () {
      try {
        var ta = document.createElement('textarea');
        ta.value = text;
        ta.setAttribute('readonly', 'readonly');
        ta.style.cssText = 'position:fixed;left:-9999px;top:0';
        document.body.appendChild(ta);
        ta.select();
        var ok = document.execCommand('copy');
        document.body.removeChild(ta);
        if (ok) done(); else toast('复制失败，请手动长按选中坐标', 'e');
      } catch (e) {
        toast('复制失败：' + e.message, 'e');
      }
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, fallback);
      return;
    }
    fallback();
  }

  /* ── 状态 ───────────────────────────────────────────────────────── */

  function newState() {
    return {
      seedText: '',
      seedValid: false,
      edition: 'java',
      version: '1.20',
      preview: false,          // 基岩种子按 Java 算法试算
      seedFromKernel: false,
      worldName: '',
      seedZero: false,
      types: { slime: true, village: true, stronghold: true, monument: false, mansion: false, fortress: false, bastion: false, temple: false },
      cx: 0, cz: 0,            // 视口中心（区块坐标，可为小数）
      scale: 8,                // 每区块像素
      sel: null,               // 选中区块 {x,z}
      slimeCache: new Map(),   // "x,z" -> 1/0
      structCache: new Map(),  // "type:rx,rz" -> [cx,cz] | null
      strongholds: null,       // 当前种子+版本的要塞点
      strongholdFor: '',
      dragging: false,
      dirty: true,
      mapNote: '',
      deferPasses: 0,          // 视口太大时，结构分帧补算的次数上限
      slimeCheck: null,        // 绑定当前种子的逐区块判定函数
      readSig: '',
      listSig: ''
    };
  }

  function caps() {
    return YYSeedLocal.capabilities(S.edition, S.version, S.preview);
  }

  function structureTypes() {
    return CHIP_ORDER.filter(function (t) { return t !== 'slime' && t !== 'stronghold'; });
  }

  /* ── 页面骨架 ───────────────────────────────────────────────────── */

  function pageHtml() {
    return '' +
      '<div class="yy-seed">' +
        '<section class="yy-seed-panel">' +
          '<div class="yy-seed-row">' +
            '<div class="yy-seed-seg" id="yy-seed-edition" role="group" aria-label="版本类型">' +
              '<button type="button" data-ed="java" class="on">Java</button>' +
              '<button type="button" data-ed="bedrock">基岩</button>' +
            '</div>' +
            '<select id="yy-seed-version" aria-label="版本号"></select>' +
          '</div>' +
          '<p class="yy-seed-verdict" id="yy-seed-verdict"></p>' +
          '<div class="yy-seed-seedrow">' +
            '<input id="yy-seed-value" inputmode="numeric" autocomplete="off" spellcheck="false" placeholder="输入种子（十进制整数，可为负数）" aria-label="种子">' +
            '<button type="button" class="btn-s" id="yy-seed-read">读取当前世界种子</button>' +
          '</div>' +
          '<p class="yy-hint" id="yy-seed-src">种子只在本机计算，不会上传；点「读取当前世界种子」才会向本机内核读一次。</p>' +
          '<div class="yy-seed-banner" id="yy-seed-banner" hidden></div>' +
          '<div class="yy-seed-chips" id="yy-seed-chips"></div>' +
          '<p class="yy-hint" id="yy-seed-chipnote"></p>' +
        '</section>' +
        '<section class="yy-seed-mapcard">' +
          '<div class="yy-seed-mapwrap" id="yy-seed-mapwrap">' +
            '<canvas id="yy-seed-canvas" aria-label="种子地图，可拖动缩放"></canvas>' +
            '<div class="yy-seed-mapnote" id="yy-seed-mapnote" hidden></div>' +
            '<div class="yy-seed-tools">' +
              '<button type="button" data-act="in" aria-label="放大">＋</button>' +
              '<button type="button" data-act="out" aria-label="缩小">－</button>' +
              '<button type="button" data-act="reset" aria-label="回到原点">复位</button>' +
            '</div>' +
            '<div class="yy-seed-legend" id="yy-seed-legend"></div>' +
          '</div>' +
          '<div class="yy-seed-readout" id="yy-seed-readout">拖动平移 · 双指缩放 · 双击放大 · 点一下读坐标</div>' +
        '</section>' +
        '<section class="yy-seed-panel">' +
          '<div class="yy-seed-listhead"><h3>命中的结构</h3><span class="yy-hint" id="yy-seed-count"></span></div>' +
          '<div class="yy-seed-list" id="yy-seed-list"><p class="yy-hint">选好种子和结构类型后，这里列出视口内的候选坐标。</p></div>' +
          '<details class="yy-seed-about"><summary>精度与边界（必读）</summary>' +
            '<ul class="yy-hint">' +
              '<li><b>史莱姆区块</b>：Java 版原版算法（java.util.Random + 盐值 0x3ad8025f），精确可复现。</li>' +
              '<li><b>要塞</b>：Java 1.9+ 三环候选角（半径 1408/4352/7424 方块），省略了原版生物群系过滤，属近似。</li>' +
              '<li><b>村庄/神殿/府邸/下界要塞/堡垒/神庙</b>：Java 1.13+ 区域网格候选区块，实际是否生成取决于生物群系与版本，需进游戏实测。</li>' +
              '<li><b>基岩版</b>：基岩版随机数与盐值和 Java 不同，本机未实现；选「基岩」时只能按 Java 算法试算，结果仅供参考。</li>' +
              '<li><b>没做的</b>：出生点估算、生物群系、地形渲染、1.21 结构参数、基岩版算法。这些不会用近似值顶替。</li>' +
            '</ul>' +
          '</details>' +
        '</section>' +
      '</div>';
  }

  /* ── 版本 / 能力 UI ─────────────────────────────────────────────── */

  function renderEdition() {
    var box = $('yy-seed-edition');
    if (!box) return;
    Array.prototype.forEach.call(box.querySelectorAll('button'), function (b) {
      b.classList.toggle('on', b.getAttribute('data-ed') === S.edition);
      b.setAttribute('aria-pressed', b.getAttribute('data-ed') === S.edition ? 'true' : 'false');
    });
  }

  function renderVersion() {
    var sel = $('yy-seed-version');
    if (!sel) return;
    var list = YYSeedLocal.VERSIONS[S.edition] || [];
    if (list.indexOf(S.version) < 0) S.version = list.indexOf('1.20') >= 0 ? '1.20' : list[list.length - 1];
    sel.innerHTML = list.map(function (v) {
      var sup = S.edition === 'bedrock' ? YYSeedLocal.bedrockSupport() : YYSeedLocal.javaSupport(v);
      // 下拉里就写清支持范围：不支持的版本不藏着，选中后也不假装能算。
      var tail = '';
      if (S.edition === 'bedrock') tail = '（算法未实现）';
      else if (!sup.structure) tail = sup.slime ? '（仅史莱姆区块）' : '（不支持）';
      return '<option value="' + esc(v) + '">' + esc((S.edition === 'java' ? 'Java ' : '基岩 ') + v + tail) + '</option>';
    }).join('');
    sel.value = S.version;
  }

  function renderVerdict() {
    var el = $('yy-seed-verdict');
    if (!el) return;
    var c = caps();
    var parts = [];
    parts.push(c.slime ? '史莱姆区块可用（精确）' : '史莱姆区块不可用');
    parts.push(c.structure ? '结构候选可用（需实测）' : '结构候选不可用');
    parts.push(c.stronghold ? '要塞近似可用' : '要塞不可用');
    el.textContent = (S.edition === 'bedrock' && S.preview ? '基岩种子 · Java 试算：' : '') + parts.join(' · ');
    el.className = 'yy-seed-verdict' + (c.slime || c.structure ? '' : ' bad');
    el.title = c.reason || '';
  }

  function renderBanner() {
    var el = $('yy-seed-banner');
    if (!el) return;
    var mapNote = $('yy-seed-mapnote');
    var text = '';
    var action = '';
    if (S.edition === 'bedrock' && S.preview) {
      text = '当前种子为基岩版' + (S.seedFromKernel ? '（本机 StartGame 包读到）' : '（手动选择）') +
        '，下面地图是 <b>Java 算法试算</b>，仅供参考，不是这个世界真实的结构位置；实际位置必须进游戏实测。';
      action = '<button type="button" class="btn-s" data-act="stop-preview">停止试算</button>';
    } else if (S.edition === 'bedrock') {
      text = '基岩版算法（史莱姆 / 结构 / 要塞）本机未实现，因此地图上没有结构点。' +
        '可以按 Java 算法试算同一串数字，但那不代表基岩世界的真实位置。';
      action = '<button type="button" class="btn-s" data-act="preview">按 Java 算法试算</button>';
    }
    el.hidden = !text;
    el.innerHTML = text ? '<div class="yy-seed-banner-text">' + text + '</div>' + action : '';
    if (mapNote) {
      var note = '';
      if (S.edition === 'bedrock' && S.preview) note = '当前种子为基岩版，Java 结构试算仅供参考';
      else if (S.edition === 'bedrock') note = '基岩版算法未实现，地图上不显示结构';
      else if (S.seedFromKernel) note = '种子来自本机世界（基岩版），此处为 Java 算法试算';
      mapNote.hidden = !note;
      mapNote.textContent = note;
      S.mapNote = note;
    }
  }

  function chipReason(type) {
    var c = caps();
    if (type === 'slime') return c.slime ? '' : c.reason;
    if (type === 'stronghold') return c.stronghold ? '' : c.reason;
    return c.structure ? '' : c.reason;
  }

  function renderChips() {
    var box = $('yy-seed-chips');
    if (!box) return;
    box.innerHTML = CHIP_ORDER.map(function (t) {
      var reason = chipReason(t);
      var on = !!S.types[t] && !reason;
      var spec = YYSeedLocal.specById(t);
      var sub = t === 'slime' ? '精确' : t === 'stronghold' ? '近似' : '候选';
      var dim = spec && spec.dimension ? ' · ' + spec.dimension : '';
      return '<button type="button" class="yy-seed-chip' + (on ? ' on' : '') + (reason ? ' off' : '') + '"' +
        ' data-type="' + t + '" aria-pressed="' + (on ? 'true' : 'false') + '"' +
        (reason ? ' title="' + esc(reason) + '"' : '') + '>' +
        '<span class="yy-seed-chip-name">' + esc(TYPE_LABEL[t]) + '</span>' +
        '<span class="yy-seed-chip-sub">' + (reason ? '暂不支持' : esc(sub + dim)) + '</span>' +
        '</button>';
    }).join('');
    var note = $('yy-seed-chipnote');
    if (note) {
      var c = caps();
      note.textContent = c.slime || c.structure ? (c.reason || '') : c.reason;
    }
  }

  function renderLegend() {
    var box = $('yy-seed-legend');
    if (!box) return;
    var items = CHIP_ORDER.filter(function (t) { return S.types[t] && !chipReason(t); }).map(function (t) {
      return '<span><i style="background:' + (t === 'slime' ? COLORS.slimeEdge : COLORS[t]) + '"></i>' + esc(TYPE_LABEL[t]) + '</span>';
    });
    box.innerHTML = items.join('');
    box.hidden = !items.length;
  }

  /* ── 地图绘制 ───────────────────────────────────────────────────── */

  function chunkToPx(x, W) { return W / 2 + (x - S.cx) * S.scale; }
  function chunkToPy(z, H) { return H / 2 + (z - S.cz) * S.scale; }
  function pxToChunkX(px, W) { return S.cx + (px - W / 2) / S.scale; }
  function pxToChunkZ(py, H) { return S.cz + (py - H / 2) / S.scale; }

  function gridStep() {
    var steps = [1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048];
    for (var i = 0; i < steps.length; i++) if (steps[i] * S.scale >= 44) return steps[i];
    return steps[steps.length - 1];
  }

  function visibleRange(W, H) {
    var halfW = W / 2 / S.scale, halfH = H / 2 / S.scale;
    return {
      x0: Math.floor(S.cx - halfW) - 1,
      x1: Math.ceil(S.cx + halfW) + 1,
      z0: Math.floor(S.cz - halfH) - 1,
      z1: Math.ceil(S.cz + halfH) + 1
    };
  }

  // 只补算视口里还没有的区块：拖动时增量很小，手机上才不卡。
  function ensureSlime(x0, z0, x1, z1) {
    var total = (x1 - x0 + 1) * (z1 - z0 + 1);
    if (total > YYSeedLocal.limits.maxSlimeChunks) return 'too-many';
    if (S.slimeCache.size > 260000) S.slimeCache.clear();
    for (var x = x0; x <= x1; x++) {
      for (var z = z0; z <= z1; z++) {
        var key = x + ',' + z;
        if (S.slimeCache.has(key)) continue;
        if (!S.slimeCheck) S.slimeCheck = YYSeedLocal.slimeChecker(S.seedText);
        S.slimeCache.set(key, S.slimeCheck && S.slimeCheck(x, z) ? 1 : 0);
      }
    }
    return '';
  }

  function ensureStructures(x0, z0, x1, z1, budget) {
    var types = structureTypes().filter(function (t) { return S.types[t] && !chipReason(t); });
    var spent = 0;
    var deferred = false;
    for (var i = 0; i < types.length; i++) {
      var t = types[i];
      var spec = YYSeedLocal.specById(t);
      if (!spec) continue;
      var rx0 = Math.floor(x0 / spec.spacing), rx1 = Math.floor(x1 / spec.spacing);
      var rz0 = Math.floor(z0 / spec.spacing), rz1 = Math.floor(z1 / spec.spacing);
      for (var rz = rz0; rz <= rz1; rz++) {
        for (var rx = rx0; rx <= rx1; rx++) {
          var key = t + ':' + rx + ',' + rz;
          if (S.structCache.has(key)) continue;
          if (spent >= budget) { deferred = true; continue; }
          var got = YYSeedLocal.structureAt(S.seedText, t, rx * spec.spacing, rz * spec.spacing);
          S.structCache.set(key, got.ok ? got.chunk : null);
          spent++;
        }
      }
    }
    return deferred;
  }

  function ensureStrongholds() {
    var stamp = S.seedText + '|' + S.version;
    if (S.strongholdFor === stamp) return;
    S.strongholdFor = stamp;
    if (!caps().stronghold) { S.strongholds = null; return; }
    var r = YYSeedLocal.strongholds(S.seedText, 3);
    S.strongholds = r.ok ? r.points : null;
  }

  function draw() {
    rafId = 0;
    var cv = $('yy-seed-canvas');
    if (!cv || !S) return;
    var rect = cv.getBoundingClientRect();
    var W = Math.round(rect.width), H = Math.round(rect.height);
    if (!W || !H) return;
    var dpr = Math.min(2, window.devicePixelRatio || 1);
    if (cv.width !== Math.round(W * dpr) || cv.height !== Math.round(H * dpr)) {
      cv.width = Math.round(W * dpr);
      cv.height = Math.round(H * dpr);
    }
    var g = cv.getContext('2d');
    if (!g) return;
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
    g.clearRect(0, 0, W, H);

    var css = getComputedStyle(document.documentElement);
    var bg = (css.getPropertyValue('--phx-bg') || '#f5f7f8').trim() || '#f5f7f8';
    var line = (css.getPropertyValue('--phx-border-light') || '#e2e8ea').trim() || '#e2e8ea';
    var text = (css.getPropertyValue('--phx-text-secondary') || '#6b7a83').trim() || '#6b7a83';

    g.fillStyle = bg;
    g.fillRect(0, 0, W, H);

    var r = visibleRange(W, H);
    var step = gridStep();

    // 区块网格
    g.lineWidth = 1;
    g.strokeStyle = line;
    g.beginPath();
    for (var gx = Math.floor(r.x0 / step) * step; gx <= r.x1; gx += step) {
      var px = Math.round(chunkToPx(gx, W)) + 0.5;
      g.moveTo(px, 0);
      g.lineTo(px, H);
    }
    for (var gz = Math.floor(r.z0 / step) * step; gz <= r.z1; gz += step) {
      var py = Math.round(chunkToPy(gz, H)) + 0.5;
      g.moveTo(0, py);
      g.lineTo(W, py);
    }
    g.stroke();

    // 坐标刻度（方块坐标 = 区块 × 16）
    g.fillStyle = text;
    g.font = '10px ui-monospace,SFMono-Regular,Menlo,monospace';
    g.textBaseline = 'top';
    for (var lx = Math.floor(r.x0 / step) * step; lx <= r.x1; lx += step) {
      var lpx = chunkToPx(lx, W);
      if (lpx < 26 || lpx > W - 6) continue;
      g.fillText(String(lx * 16), lpx + 3, 4);
    }
    g.textBaseline = 'alphabetic';
    for (var lz = Math.floor(r.z0 / step) * step; lz <= r.z1; lz += step) {
      var lpy = chunkToPy(lz, H);
      if (lpy < 16 || lpy > H - 6) continue;
      g.fillText(String(lz * 16), 4, lpy - 3);
    }

    var noSeed = !S.seedValid;
    var c = caps();
    var deferred = false;
    var slimeNote = '';

    // 史莱姆区块
    if (!noSeed && c.slime && S.types.slime) {
      if (S.scale < 2.2) {
        slimeNote = '放大到每区块 2 像素以上才显示史莱姆区块';
      } else {
        slimeNote = ensureSlime(r.x0, r.z0, r.x1, r.z1);
        if (slimeNote) slimeNote = '当前视野区块太多，放大后再显示史莱姆区块';
        else {
          g.fillStyle = COLORS.slime;
          for (var sx = r.x0; sx <= r.x1; sx++) {
            for (var sz = r.z0; sz <= r.z1; sz++) {
              if (S.slimeCache.get(sx + ',' + sz) !== 1) continue;
              var rx0p = chunkToPx(sx, W), rz0p = chunkToPy(sz, H);
              g.fillRect(rx0p, rz0p, Math.max(1, S.scale - 0.5), Math.max(1, S.scale - 0.5));
            }
          }
        }
      }
    }

    // 结构候选
    if (!noSeed && c.structure) {
      deferred = ensureStructures(r.x0, r.z0, r.x1, r.z1, 2600);
      var types = structureTypes();
      for (var ti = 0; ti < types.length; ti++) {
        var t = types[ti];
        if (!S.types[t]) continue;
        var spec = YYSeedLocal.specById(t);
        if (!spec) continue;
        var rx0 = Math.floor(r.x0 / spec.spacing), rx1 = Math.floor(r.x1 / spec.spacing);
        var rz0 = Math.floor(r.z0 / spec.spacing), rz1 = Math.floor(r.z1 / spec.spacing);
        g.fillStyle = COLORS[t] || '#555';
        g.strokeStyle = '#fff';
        g.lineWidth = 1.5;
        for (var rz = rz0; rz <= rz1; rz++) {
          for (var rx = rx0; rx <= rx1; rx++) {
            var hit = S.structCache.get(t + ':' + rx + ',' + rz);
            if (!hit) continue;
            var mx = chunkToPx(hit[0] + 0.5, W), my = chunkToPy(hit[1] + 0.5, H);
            if (mx < -12 || mx > W + 12 || my < -12 || my > H + 12) continue;
            g.beginPath();
            g.arc(mx, my, 4.5, 0, Math.PI * 2);
            g.fill();
            g.stroke();
          }
        }
      }
    }

    // 要塞（近似点，画成菱形以便和结构候选区分）
    if (!noSeed && c.stronghold && S.types.stronghold) {
      ensureStrongholds();
      if (S.strongholds) {
        for (var si = 0; si < S.strongholds.length; si++) {
          var p = S.strongholds[si];
          var hx = chunkToPx(p.x / 16, W), hy = chunkToPy(p.z / 16, H);
          if (hx < -20 || hx > W + 20 || hy < -20 || hy > H + 20) continue;
          g.fillStyle = COLORS.stronghold;
          g.strokeStyle = '#fff';
          g.lineWidth = 1.5;
          g.beginPath();
          g.moveTo(hx, hy - 7);
          g.lineTo(hx + 7, hy);
          g.lineTo(hx, hy + 7);
          g.lineTo(hx - 7, hy);
          g.closePath();
          g.fill();
          g.stroke();
          g.fillStyle = '#fff';
          g.font = 'bold 9px ui-monospace,monospace';
          g.textAlign = 'center';
          g.fillText(String(si + 1), hx, hy + 3);
          g.textAlign = 'left';
        }
      }
    }

    // 视口中心十字
    g.strokeStyle = 'rgba(0,0,0,0.22)';
    g.lineWidth = 1;
    g.beginPath();
    g.moveTo(W / 2 - 8, H / 2);
    g.lineTo(W / 2 + 8, H / 2);
    g.moveTo(W / 2, H / 2 - 8);
    g.lineTo(W / 2, H / 2 + 8);
    g.stroke();

    // 选中点
    if (S.sel) {
      var selPx = chunkToPx(S.sel.x + 0.5, W), selPy = chunkToPy(S.sel.z + 0.5, H);
      g.strokeStyle = '#e08a1e';
      g.lineWidth = 2;
      g.beginPath();
      g.arc(selPx, selPy, 9, 0, Math.PI * 2);
      g.stroke();
      g.beginPath();
      g.moveTo(selPx - 14, selPy);
      g.lineTo(selPx + 14, selPy);
      g.moveTo(selPx, selPy - 14);
      g.lineTo(selPx, selPy + 14);
      g.stroke();
    }

    if (noSeed) {
      g.fillStyle = 'rgba(0,0,0,0.55)';
      g.font = '13px system-ui,sans-serif';
      g.textAlign = 'center';
      g.fillText('先在上面输入种子，地图才会出现内容', W / 2, H / 2 + 34);
      g.textAlign = 'left';
    } else if (slimeNote) {
      g.fillStyle = 'rgba(0,0,0,0.45)';
      g.font = '11px system-ui,sans-serif';
      g.textAlign = 'center';
      g.fillText(slimeNote, W / 2, H - 14);
      g.textAlign = 'left';
    }

    if (deferred && !S.dragging && S.deferPasses < 4) {
      S.deferPasses++;
      schedule();
    }

    updateReadout();
    renderList();
  }

  function schedule() {
    if (rafId) return;
    rafId = requestAnimationFrame(draw);
  }

  /* ── 读数 / 列表 ────────────────────────────────────────────────── */

  function updateReadout() {
    var el = $('yy-seed-readout');
    if (!el) return;
    var centerX = Math.round(S.cx), centerZ = Math.round(S.cz);
    var sig = [centerX, centerZ, S.scale.toFixed(2), S.seedValid, S.sel ? S.sel.x + ',' + S.sel.z : '-', S.structCache.size].join('|');
    if (sig === S.readSig) return;
    S.readSig = sig;
    var html = '<div class="yy-seed-read-row"><span>视口中心</span><b>X ' + fmtInt(centerX * 16) + ' / Z ' + fmtInt(centerZ * 16) + '</b>' +
      '<span class="yy-seed-dim">区块 ' + centerX + ', ' + centerZ + '</span>' +
      '<button type="button" class="btn-s" data-copy="' + centerX * 16 + ' ' + centerZ * 16 + '">复制中心</button></div>';
    if (S.sel) {
      var bx = S.sel.x * 16, bz = S.sel.z * 16;
      html += '<div class="yy-seed-read-row"><span>选中点</span><b>X ' + fmtInt(bx) + ' / Z ' + fmtInt(bz) + '</b>' +
        '<span class="yy-seed-dim">区块 ' + S.sel.x + ', ' + S.sel.z + '</span>' +
        '<button type="button" class="btn-s" data-copy="' + bx + ' ' + bz + '">复制坐标</button></div>';
    }
    if (!S.seedValid) {
      html += '<div class="yy-seed-read-row yy-seed-dim">还没输入种子</div>';
    } else {
      var n = 0;
      S.structCache.forEach(function (v) { if (v) n++; });
      html += '<div class="yy-seed-read-row yy-seed-dim">每区块 ' + (S.scale).toFixed(S.scale < 2 ? 2 : 0) + ' 像素 · 已算结构点 ' + n + ' · 全部在本机计算</div>';
    }
    el.innerHTML = html;
  }

  function collectPoints() {
    var out = [];
    if (!S.seedValid) return out;
    var c = caps();
    var r = null;
    var cv = $('yy-seed-canvas');
    if (cv) {
      var rect = cv.getBoundingClientRect();
      r = visibleRange(Math.max(1, rect.width), Math.max(1, rect.height));
    }
    structureTypes().forEach(function (t) {
      if (!S.types[t] || !c.structure) return;
      var spec = YYSeedLocal.specById(t);
      if (!spec || !r) return;
      var rx0 = Math.floor(r.x0 / spec.spacing), rx1 = Math.floor(r.x1 / spec.spacing);
      var rz0 = Math.floor(r.z0 / spec.spacing), rz1 = Math.floor(r.z1 / spec.spacing);
      for (var rz = rz0; rz <= rz1; rz++) {
        for (var rx = rx0; rx <= rx1; rx++) {
          var hit = S.structCache.get(t + ':' + rx + ',' + rz);
          if (!hit) continue;
          out.push({ type: t, label: TYPE_LABEL[t], x: hit[0], z: hit[1] });
        }
      }
    });
    // 史莱姆区块也进列表：只勾史莱姆时，地图上一片绿但列表空着会让人以为坏了。
    // 视野太大时（> 8000 区块）跳过，避免每帧遍历几万个缓存键。
    if (r && S.types.slime && c.slime) {
      var cells = (r.x1 - r.x0 + 1) * (r.z1 - r.z0 + 1);
      if (cells <= 8000) {
        for (var sx = r.x0; sx <= r.x1; sx++) {
          for (var sz = r.z0; sz <= r.z1; sz++) {
            if (S.slimeCache.get(sx + ',' + sz) !== 1) continue;
            out.push({ type: 'slime', label: '史莱姆区块', x: sx, z: sz });
          }
        }
      }
    }
    if (S.types.stronghold && c.stronghold && S.strongholds) {
      S.strongholds.forEach(function (p, i) {
        out.push({ type: 'stronghold', label: '要塞 ' + (i + 1) + '（近似）', x: p.chunkX, z: p.chunkZ, exact: false });
      });
    }
    out.sort(function (a, b) {
      var da = Math.abs(a.x - S.cx) + Math.abs(a.z - S.cz);
      var db = Math.abs(b.x - S.cx) + Math.abs(b.z - S.cz);
      return da - db;
    });
    return out;
  }

  function renderList() {
    var box = $('yy-seed-list');
    var cnt = $('yy-seed-count');
    if (!box) return;
    if (!S.seedValid) {
      if (S.listSig !== 'none') {
        S.listSig = 'none';
        box.innerHTML = '<p class="yy-hint">先输入种子。</p>';
        if (cnt) cnt.textContent = '';
      }
      return;
    }
    var pts = collectPoints();
    var sig = pts.length + '|' + Math.round(S.cx) + ',' + Math.round(S.cz) + '|' + (S.seedText || '');
    if (sig === S.listSig) return;
    S.listSig = sig;
    if (cnt) cnt.textContent = pts.length ? ('视口内 ' + pts.length + ' 个（按距离排序，最多显示 60 个）') : '视口内没有命中';
    if (!pts.length) {
      var c = caps();
      box.innerHTML = '<p class="yy-hint">' + esc(c.slime || c.structure ? '当前视口内没有命中。可以拖动地图或勾选更多结构类型。' : c.reason) + '</p>';
      return;
    }
    var rows = pts.slice(0, 60).map(function (p) {
      return '<button type="button" class="yy-seed-hit" data-jump="' + p.x + ',' + p.z + '">' +
        '<i style="background:' + (p.type === 'slime' ? COLORS.slimeEdge : COLORS[p.type] || '#666') + '"></i>' +
        '<span class="yy-seed-hit-name">' + esc(p.label) + '</span>' +
        '<span class="yy-seed-hit-pos">区块 ' + p.x + ', ' + p.z + '</span>' +
        '<span class="yy-seed-hit-pos yy-seed-hit-block">方块 ' + fmtInt(p.x * 16) + ', ' + fmtInt(p.z * 16) + '</span>' +
        '<span class="yy-seed-hit-go">跳转</span>' +
        '</button>';
    }).join('');
    box.innerHTML = '<div class="yy-seed-hits">' + rows + '</div>';
  }

  /* ── 交互 ───────────────────────────────────────────────────────── */

  function bindMap() {
    var cv = $('yy-seed-canvas');
    var wrap = $('yy-seed-mapwrap');
    if (!cv || !wrap) return;

    var pointers = new Map();
    var last = null;
    var pinch = null;
    var moved = 0;
    var downAt = 0;
    var downPos = null;
    var lastTap = 0;
    var lastTapPos = null;

    function localPos(e) {
      var rect = cv.getBoundingClientRect();
      return { x: e.clientX - rect.left, y: e.clientY - rect.top };
    }

    function onDown(e) {
      if (e.button != null && e.button !== 0) return;
      cv.setPointerCapture && cv.setPointerCapture(e.pointerId);
      pointers.set(e.pointerId, localPos(e));
      moved = 0;
      downAt = Date.now();
      downPos = localPos(e);
      if (pointers.size === 2) {
        var it = Array.from(pointers.values());
        pinch = {
          dist: Math.hypot(it[0].x - it[1].x, it[0].y - it[1].y) || 1,
          scale: S.scale,
          mid: { x: (it[0].x + it[1].x) / 2, y: (it[0].y + it[1].y) / 2 },
          cx: S.cx,
          cz: S.cz
        };
      }
      S.dragging = true;
      last = localPos(e);
    }

    function onMove(e) {
      if (!pointers.has(e.pointerId)) return;
      var p = localPos(e);
      pointers.set(e.pointerId, p);
      var rect = cv.getBoundingClientRect();
      if (pointers.size >= 2 && pinch) {
        var it = Array.from(pointers.values());
        var dist = Math.hypot(it[0].x - it[1].x, it[0].y - it[1].y) || 1;
        var next = clamp(pinch.scale * (dist / pinch.dist), 0.25, 40);
        // 以双指中点为锚：缩放前后中点下的区块坐标保持不变
        var anchorX = pinch.cx + (pinch.mid.x - rect.width / 2) / pinch.scale;
        var anchorZ = pinch.cz + (pinch.mid.y - rect.height / 2) / pinch.scale;
        S.scale = next;
        S.cx = anchorX - (pinch.mid.x - rect.width / 2) / next;
        S.cz = anchorZ - (pinch.mid.y - rect.height / 2) / next;
        moved += 12;
        schedule();
        return;
      }
      if (!last) { last = p; return; }
      var dx = p.x - last.x, dy = p.y - last.y;
      moved += Math.abs(dx) + Math.abs(dy);
      if (moved > 3) {
        S.cx -= dx / S.scale;
        S.cz -= dy / S.scale;
      }
      last = p;
      schedule();
    }

    function onUp(e) {
      var wasSingle = pointers.size === 1;
      pointers.delete(e.pointerId);
      if (pointers.size < 2) pinch = null;
      last = null;
      if (!pointers.size) { S.dragging = false; S.deferPasses = 0; }
      if (!wasSingle || !downPos) { schedule(); return; }
      var p = localPos(e);
      var isTap = moved < 8 && Date.now() - downAt < 500;
      if (!isTap) { schedule(); return; }

      // 双击 / 双触放大
      var now = Date.now();
      if (lastTap && now - lastTap < 320 && lastTapPos && Math.hypot(p.x - lastTapPos.x, p.y - lastTapPos.y) < 32) {
        zoomAt(p.x, p.y, 1.8);
        lastTap = 0;
        lastTapPos = null;
        return;
      }
      lastTap = now;
      lastTapPos = p;

      var rect = cv.getBoundingClientRect();
      var cx = Math.floor(pxToChunkX(p.x, rect.width));
      var cz = Math.floor(pxToChunkZ(p.y, rect.height));
      S.sel = { x: cx, z: cz };
      updateReadout();
      schedule();
    }

    function zoomAt(px, py, factor) {
      var rect = cv.getBoundingClientRect();
      var anchorX = S.cx + (px - rect.width / 2) / S.scale;
      var anchorZ = S.cz + (py - rect.height / 2) / S.scale;
      S.scale = clamp(S.scale * factor, 0.25, 40);
      S.cx = anchorX - (px - rect.width / 2) / S.scale;
      S.cz = anchorZ - (py - rect.height / 2) / S.scale;
      S.deferPasses = 0;
      schedule();
    }

    function onWheel(e) {
      e.preventDefault();
      var p = localPos(e);
      zoomAt(p.x, p.y, e.deltaY < 0 ? 1.2 : 1 / 1.2);
    }

    function onDbl(e) {
      e.preventDefault();
      var p = localPos(e);
      zoomAt(p.x, p.y, 1.8);
    }

    cv.addEventListener('pointerdown', onDown);
    cv.addEventListener('pointermove', onMove);
    cv.addEventListener('pointerup', onUp);
    cv.addEventListener('pointercancel', onUp);
    cv.addEventListener('wheel', onWheel, { passive: false });
    cv.addEventListener('dblclick', onDbl);
    // 兜底：老 WebView 没有 PointerEvent 时用触摸事件平移
    if (!window.PointerEvent) {
      var tx = 0, tz = 0, touching = false;
      cv.addEventListener('touchstart', function (e) {
        if (e.touches.length === 1) { touching = true; tx = e.touches[0].clientX; tz = e.touches[0].clientY; }
      }, { passive: true });
      cv.addEventListener('touchmove', function (e) {
        if (!touching || e.touches.length !== 1) return;
        var dx = e.touches[0].clientX - tx, dy = e.touches[0].clientY - tz;
        tx = e.touches[0].clientX; tz = e.touches[0].clientY;
        S.cx -= dx / S.scale; S.cz -= dy / S.scale;
        e.preventDefault();
        schedule();
      }, { passive: false });
      cv.addEventListener('touchend', function () { touching = false; }, { passive: true });
    }

    cleanup.push(function () {
      cv.removeEventListener('pointerdown', onDown);
      cv.removeEventListener('pointermove', onMove);
      cv.removeEventListener('pointerup', onUp);
      cv.removeEventListener('pointercancel', onUp);
      cv.removeEventListener('wheel', onWheel);
      cv.removeEventListener('dblclick', onDbl);
    });

    Array.prototype.forEach.call(wrap.querySelectorAll('[data-act]'), function (b) {
      b.onclick = function () {
        var act = b.getAttribute('data-act');
        if (act === 'in') { S.scale = clamp(S.scale * 1.6, 0.25, 40); schedule(); }
        else if (act === 'out') { S.scale = clamp(S.scale / 1.6, 0.25, 40); schedule(); }
        else if (act === 'reset') { S.cx = 0; S.cz = 0; S.scale = 8; S.sel = null; schedule(); }
      };
    });

    var ro = null;
    if (window.ResizeObserver) {
      ro = new ResizeObserver(function () { schedule(); });
      ro.observe(wrap);
      cleanup.push(function () { ro.disconnect(); });
    } else {
      var onResize = function () { schedule(); };
      window.addEventListener('resize', onResize);
      cleanup.push(function () { window.removeEventListener('resize', onResize); });
    }
  }

  /* ── 种子读取 ───────────────────────────────────────────────────── */

  function applySeed(text, meta) {
    var parsed = YYSeedLocal.parseSeed(text);
    if (!parsed.ok) {
      S.seedValid = false;
      toast(parsed.error, 'e');
      return false;
    }
    S.seedText = parsed.text;
    S.seedValid = true;
    S.slimeCache.clear();
    S.structCache.clear();
    S.slimeCheck = YYSeedLocal.slimeChecker(parsed.text);
    S.strongholdFor = '';
    S.strongholds = null;
    if (meta) {
      S.seedFromKernel = !!meta.fromKernel;
      S.worldName = meta.worldName || '';
      S.seedZero = !!meta.zero;
    }
    var input = $('yy-seed-value');
    if (input && input.value !== parsed.text) input.value = parsed.text;
    S.cx = 0; S.cz = 0; S.sel = null;
    schedule();
    return true;
  }

  async function readWorldSeed(btn) {
    if (btn) btn.disabled = true;
    var src = $('yy-seed-src');
    if (src) src.textContent = '正在从本机内核读取世界种子…';
    try {
      var r = await apiGet('/api/youyuan/world/seed');
      if (!r || !r.ok || !r.seed_known) {
        var msg = (r && r.error) || '尚未从内核读取到世界种子：请先连接服务器并完成登录，或直接手输种子';
        if (src) src.textContent = msg;
        toast(msg, 'e');
        return;
      }
      var text = r.seed_text != null ? String(r.seed_text) : String(r.seed);
      var zero = /^[+-]?0+$/.test(text);
      S.edition = 'bedrock';
      S.preview = true; // 基岩种子默认按 Java 试算，页面上有醒目提示
      renderEdition();
      renderVersion();
      if (!applySeed(text, { fromKernel: true, worldName: r.world_name, zero: zero })) return;
      if (src) {
        src.textContent = '已读取：种子 ' + text + (r.world_name ? '（' + r.world_name + '）' : '') +
          (zero ? ' · 服务端可能把真实种子替换为 0，别当成真实种子' : '') +
          ' · 来源：本机 StartGame 包';
      }
      renderVerdict();
      renderBanner();
      renderChips();
      renderLegend();
      schedule();
    } catch (e) {
      if (src) src.textContent = '读取失败：' + e.message;
      toast('读取世界种子失败：' + e.message, 'e');
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  /* ── 绑定 ───────────────────────────────────────────────────────── */

  function bindPanel() {
    var edBox = $('yy-seed-edition');
    if (edBox) {
      edBox.onclick = function (e) {
        var b = e.target.closest ? e.target.closest('button[data-ed]') : null;
        if (!b) return;
        var next = b.getAttribute('data-ed');
        if (next === S.edition) return;
        S.edition = next;
        S.preview = false;
        S.structCache.clear();
        S.slimeCache.clear();
        S.strongholdFor = '';
        renderEdition();
        renderVersion();
        renderVerdict();
        renderBanner();
        renderChips();
        renderLegend();
        schedule();
      };
    }

    var ver = $('yy-seed-version');
    if (ver) {
      ver.onchange = function () {
        S.version = ver.value;
        S.structCache.clear();
        S.strongholdFor = '';
        S.deferPasses = 0;
        renderVerdict();
        renderBanner();
        renderChips();
        renderLegend();
        schedule();
      };
    }

    var input = $('yy-seed-value');
    if (input) {
      var apply = function () {
        if (!input.value.trim()) {
          S.seedValid = false;
          S.seedText = '';
          S.slimeCache.clear();
          S.structCache.clear();
          S.strongholds = null;
          S.sel = null;
          schedule();
          return;
        }
        if (applySeed(input.value, { fromKernel: false })) {
          var src = $('yy-seed-src');
          if (src) src.textContent = '手输种子 · 全部在本机计算，不会上传';
        }
        renderVerdict();
        renderBanner();
        renderChips();
        renderLegend();
        S.deferPasses = 0;
      };
      input.onchange = apply;
      input.onkeydown = function (e) {
        if (e.key === 'Enter') { e.preventDefault(); input.blur(); apply(); }
      };
    }

    var readBtn = $('yy-seed-read');
    if (readBtn) readBtn.onclick = function () { readWorldSeed(readBtn); };

    var chips = $('yy-seed-chips');
    if (chips) {
      chips.onclick = function (e) {
        var b = e.target.closest ? e.target.closest('button[data-type]') : null;
        if (!b) return;
        var t = b.getAttribute('data-type');
        if (chipReason(t)) { toast(chipReason(t), 'e'); return; }
        S.types[t] = !S.types[t];
        S.deferPasses = 0;
        renderChips();
        renderLegend();
        schedule();
      };
    }

    var banner = $('yy-seed-banner');
    if (banner) {
      banner.onclick = function (e) {
        var b = e.target.closest ? e.target.closest('button[data-act]') : null;
        if (!b) return;
        var act = b.getAttribute('data-act');
        if (act === 'preview') {
          S.preview = true;
          S.structCache.clear();
          S.slimeCache.clear();
          S.strongholdFor = '';
        } else if (act === 'stop-preview') {
          S.preview = false;
          S.structCache.clear();
          S.slimeCache.clear();
          S.strongholdFor = '';
        }
        renderVerdict();
        renderBanner();
        renderChips();
        renderLegend();
        schedule();
      };
    }

    var readout = $('yy-seed-readout');
    if (readout) {
      readout.onclick = function (e) {
        var b = e.target.closest ? e.target.closest('button[data-copy]') : null;
        if (!b) return;
        copyText(b.getAttribute('data-copy'), '坐标已复制');
      };
    }

    var list = $('yy-seed-list');
    if (list) {
      list.onclick = function (e) {
        var b = e.target.closest ? e.target.closest('button[data-jump]') : null;
        if (!b) return;
        var xy = b.getAttribute('data-jump').split(',');
        var x = Number(xy[0]), z = Number(xy[1]);
        if (!isFinite(x) || !isFinite(z)) return;
        S.cx = x + 0.5;
        S.cz = z + 0.5;
        S.sel = { x: x, z: z };
        if (S.scale < 4) S.scale = 8;
        schedule();
      };
    }
  }

  /* ── 入口 ───────────────────────────────────────────────────────── */

  function yySeedPage() {
    S = newState();
    delete subPageCache[PAGE_TITLE];
    openSubPage(PAGE_TITLE, pageHtml());

    renderEdition();
    renderVersion();
    renderVerdict();
    renderBanner();
    renderChips();
    renderLegend();
    bindPanel();
    bindMap();

    // 页面被切走时 openSubPage/closeSubPage 会调用这些清理函数
    if (typeof activeRenderers !== 'undefined' && Array.isArray(activeRenderers)) {
      activeRenderers.push(function () {
        if (rafId) { cancelAnimationFrame(rafId); rafId = 0; }
        cleanup.forEach(function (fn) { try { fn(); } catch (e) {} });
        cleanup = [];
        S = null;
      });
    }

    // 首帧要等布局完成，否则 canvas 尺寸是 0
    setTimeout(schedule, 30);
    return Promise.resolve();
  }

  window.yySeedPage = yySeedPage;

  /* ── 工具页一级入口 ─────────────────────────────────────────────── */

  // 独立分类：种子查询不挂在「建筑工具」下面，和导入/导出同级但各占一块。
  if (typeof yyCategories === 'object' && yyCategories && !yyCategories.seed) {
    yyCategories.seed = '种子与地图';
  }

  if (typeof yyModules !== 'undefined' && Array.isArray(yyModules) && !yyModules.some(function (m) { return m.id === 'seed'; })) {
    yyModules.push({
      id: 'seed',
      name: '种子查询',
      desc: '地图式看史莱姆与结构，全部本机计算',
      category: 'seed',
      icon: 'grid',
      open: function () { return yySeedPage(); }
    });
    if (typeof yyAllModules !== 'undefined' && yyAllModules !== yyModules && !yyAllModules.some(function (m) { return m.id === 'seed'; })) {
      yyAllModules.push(yyModules[yyModules.length - 1]);
    }
    if (typeof yyPrefs === 'object' && yyPrefs && Array.isArray(yyPrefs.order) && yyPrefs.order.indexOf('seed') < 0) {
      yyPrefs.order.push('seed');
      if (typeof yySavePrefs === 'function') yySavePrefs();
    }
    // 首页工具列表可能已经画过一遍，补一次重绘，避免新入口要切页才出现。
    if (typeof yyDrawTools === 'function' && typeof curTab !== 'undefined' && curTab === 'home' && typeof subPageOpen !== 'undefined' && !subPageOpen) {
      try { yyDrawTools(); } catch (e) {}
    }
  }
})();
