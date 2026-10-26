/* 种子地图 v2 · 前端逻辑
 *
 * 页面：/api/seed/v2/* 的查看器。地形由服务端瓦片渲染，本文件只做「摆瓦片 +
 * 叠图标 + 报坐标」，不在前端算生物群系。
 *
 * 坐标约定（全文件统一）：
 *   - 接口里的 cx / cz / x / z 一律是【区块坐标】
 *   - 屏幕给玩家看的是【方块坐标】= 区块 × 16 + 8（区块中心）
 *   - 视图状态 S.view.x / S.view.z 存的是方块坐标，便于像素换算
 *
 * 交互：拖动平移、滚轮直接缩放（不需要按 Ctrl）、双指缩放、双击放大、
 *       悬停读方块坐标、点击弹坐标面板、侧栏结构列表点击跳转。
 *
 * 性能：瓦片 Blob + objectURL 缓存（LRU 60 张）；移动/缩放 requestAnimationFrame
 *       合并；瓦片与接口请求都用 AbortController，过期即取消。
 *
 * 依赖：app.js 的 openSubPage / activeRenderers、youyuan_account_boot.js 的
 *       yyRootFetch（没有就退回 window.fetch）、css/youyuan_seedmap_v2.css。
 */
(function () {
  'use strict';

  /* 联调开关：true 时全部走内置示例数据（假地形 + 假结构），只为看布局。
     交付必须是 false。 */
  var YY_SEEDMAP_MOCK = false;

  var API = '/api/seed/v2';
  var PAGE_TITLE = '种子地图';
  var TILE_PX = 256;              // 瓦片像素边长（接口允许 32~512）
  var MIN_SCALE = 0.04;           // 每方块多少屏幕像素
  var MAX_SCALE = 32;
  var TILE_CACHE_MAX = 60;        // 瓦片内存缓存上限
  var MAX_TILE_ELEMS = 64;        // DOM 里同时存在的瓦片上限
  var MAX_ICONS = 420;
  var MAX_SLIMES = 900;
  var STORE_KEY = 'yy-seedmap-v2';

  /* 只是兜底文案：正常都读 manifest 给的中文名，绝不把英文 id 甩给玩家 */
  var NAME_FALLBACK = {
    village: '村庄', pillagerOutpost: '掠夺者前哨站', desertPyramid: '沙漠神殿',
    jungleTemple: '丛林神庙', swampHut: '沼泽小屋', witchHut: '女巫小屋',
    igloo: '雪屋', shipwreck: '沉船', oceanMonument: '海底神殿',
    oceanRuin: '海底废墟', woodlandMansion: '林地府邸', ancientCity: '远古城市',
    trialChamber: '试炼密室', ruinedPortal: '废弃传送门', buriedTreasure: '埋藏的宝藏',
    mineshaft: '废弃矿井', fortress: '下界要塞', bastionRemnant: '堡垒遗迹',
    bastion: '堡垒遗迹', endCity: '末地城', endShip: '末地船', stronghold: '要塞'
  };
  var COLOR_FALLBACK = ['#2f9e5f', '#b0432f', '#8a5a2b', '#2f6f9e', '#7a4b9e',
    '#a8862a', '#d0453a', '#3f9e9e', '#9e6f2f', '#5f7a3a', '#6b6fa8', '#a85f8f'];

  var S = null;                   // 页面状态，每次打开重建
  var rafId = 0;
  var cleanups = [];
  var cssPromise = null;

  /* ── 基础小工具 ─────────────────────────────────────────────────── */

  function $(id) { return document.getElementById(id); }

  function esc(v) {
    return String(v == null ? '' : v).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  function clamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }

  function isNum(v) { return typeof v === 'number' && isFinite(v); }

  function closest(el, sel) {
    while (el && el.nodeType === 1) {
      if (el.matches ? el.matches(sel) : (el.msMatchesSelector && el.msMatchesSelector(sel))) return el;
      el = el.parentElement;
    }
    return null;
  }

  function toast(msg, kind) {
    if (typeof T === 'function') { T(msg, kind || 'e'); return; }
    console.warn('[种子地图]', msg);
  }

  function copyText(text, okMsg) {
    var done = function () { toast(okMsg || '已复制', 'o'); };
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
        if (ok) done(); else toast('复制没成功，请手动选中坐标', 'e');
      } catch (e) { toast('复制失败：' + e.message, 'e'); }
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, fallback);
      return;
    }
    fallback();
  }

  function fmtInt(n) { return String(Math.round(n)); }

  function blockOf(chunk) { return chunk * 16 + 8; }
  function chunkOf(block) { return Math.floor(block / 16); }

  function fmtBlocks(n) {
    n = Math.round(n);
    if (Math.abs(n) >= 10000) return (n / 1000).toFixed(1) + 'k 方块';
    return fmtInt(n) + ' 方块';
  }

  function storeRead() {
    try {
      var raw = localStorage.getItem(STORE_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch (e) { return null; }
  }

  function storeWrite() {
    if (!S || !S.started) return;
    try {
      localStorage.setItem(STORE_KEY, JSON.stringify({
        seed: S.seedText,
        edition: S.edition,
        version: S.version,
        dim: S.dim
      }));
    } catch (e) { /* 隐私模式下写不进去，忽略 */ }
  }

  /* ── 样式：只注入，不改 index.html 的既有行 ─────────────────────── */

  function ensureCss() {
    if (cssPromise) return cssPromise;
    cssPromise = new Promise(function (resolve) {
      var old = document.getElementById('yy-seedmap-v2-css');
      if (old) { resolve(); return; }
      var link = document.createElement('link');
      link.id = 'yy-seedmap-v2-css';
      link.rel = 'stylesheet';
      link.href = '/css/youyuan_seedmap_v2.css?v=2026102601';
      var done = false;
      var finish = function () { if (done) return; done = true; resolve(); };
      link.onload = finish;
      link.onerror = function () {
        console.warn('[种子地图] 样式表加载失败，页面会缺样式但不影响功能');
        finish();
      };
      document.head.appendChild(link);
      setTimeout(finish, 2500);
    });
    return cssPromise;
  }

  /* ── 内置示例数据（仅 YY_SEEDMAP_MOCK，交付为 false）─────────────── */

  var MOCK = (function () {
    var JAVA = ['1.21.4', '1.21.3', '1.21.2', '1.21.1', '1.21', '1.20.6', '1.20.5',
      '1.20.4', '1.20.3', '1.20.2', '1.20.1', '1.20', '1.19.4', '1.19.3', '1.19.2',
      '1.19.1', '1.19', '1.18.2', '1.18.1', '1.18', '1.17.1', '1.16.5'];
    var BEDROCK = ['1.21.50', '1.21.40', '1.21.30', '1.21.20', '1.21.10', '1.21.0',
      '1.20.80', '1.20.60', '1.20.50', '1.20.40', '1.20.30', '1.20.10'];

    var STRUCTS = [
      ['village', '村庄', 'overworld', 'exact', '#2f9e5f', true],
      ['pillagerOutpost', '掠夺者前哨站', 'overworld', 'exact', '#b0432f', true],
      ['desertPyramid', '沙漠神殿', 'overworld', 'exact', '#a8862a', false],
      ['jungleTemple', '丛林神庙', 'overworld', 'exact', '#5f7a3a', false],
      ['witchHut', '沼泽小屋', 'overworld', 'exact', '#6b6fa8', false],
      ['igloo', '雪屋', 'overworld', 'exact', '#7a9ea8', false],
      ['shipwreck', '沉船', 'overworld', 'candidate', '#2f6f9e', false],
      ['oceanMonument', '海底神殿', 'overworld', 'exact', '#2f8f9e', true],
      ['woodlandMansion', '林地府邸', 'overworld', 'exact', '#7a4b9e', true],
      ['ancientCity', '远古城市', 'overworld', 'candidate', '#3c4a6e', true],
      ['trialChamber', '试炼密室', 'overworld', 'candidate', '#9e6f2f', false],
      ['ruinedPortal', '废弃传送门', 'overworld', 'candidate', '#8a5a2b', false],
      ['buriedTreasure', '埋藏的宝藏', 'overworld', 'candidate', '#c2a24a', false],
      ['fortress', '下界要塞', 'nether', 'exact', '#b0432f', true],
      ['bastion', '堡垒遗迹', 'nether', 'exact', '#6f4a2b', true],
      ['ruinedPortalNether', '废弃传送门', 'nether', 'candidate', '#8a5a2b', false],
      ['endCity', '末地城', 'end', 'exact', '#8f6fa8', true],
      ['endShip', '末地船', 'end', 'exact', '#6f5a9e', false]
    ];

    function hash(n) {
      var x = Math.sin(n * 12.9898 + 78.233) * 43758.5453;
      return x - Math.floor(x);
    }

    function manifest() {
      return {
        ok: true,
        engine: 'seedv2-mock',
        btree_tables: ['btree18', 'btree192', 'btree19', 'btree20', 'btree21wd'],
        biome_engine: true,
        render_engine: true,
        editions: [
          { id: 'java', name: 'Java 版', versions: JAVA.map(function (v) { return { label: v, code: 0 }; }) },
          { id: 'bedrock', name: '基岩版', versions: BEDROCK.map(function (v) { return { label: v, code: 0 }; }) }
        ],
        dimensions: [
          { id: 'overworld', name: '主世界' },
          { id: 'nether', name: '下界' },
          { id: 'end', name: '末地' }
        ],
        structures: STRUCTS.map(function (s) {
          return { id: s[0], name: s[1], dim: s[2], precision: s[3], color: s[4], default: s[5] };
        })
      };
    }

    function pointsFor(type, r) {
      var base = 0;
      for (var i = 0; i < STRUCTS.length; i++) if (STRUCTS[i][0] === type) { base = i + 1; break; }
      var n = 20 + Math.floor(hash(base * 31) * 90);
      var out = [];
      for (var k = 0; k < n; k++) {
        var a = hash(base * 97 + k * 13) * Math.PI * 2;
        var d = Math.sqrt(hash(base * 57 + k * 7)) * r;
        out.push({ x: Math.round(Math.cos(a) * d), z: Math.round(Math.sin(a) * d) });
      }
      return out;
    }

    function structures(q) {
      var r = Math.min(512, Number(q.r) || 64);
      var wanted = (q.types || '').split(',').filter(Boolean);
      if (!wanted.length) {
        var dim = q.dim || 'overworld';
        wanted = STRUCTS.filter(function (s) { return s[2] === dim && s[5]; }).map(function (s) { return s[0]; });
      }
      var out = {}, counts = {}, total = 0;
      wanted.forEach(function (t) {
        var pts = pointsFor(t, r).filter(function (p) {
          return Math.abs(p.x) <= r && Math.abs(p.z) <= r;
        });
        out[t] = pts;
        counts[t] = pts.length;
        total += pts.length;
      });
      return { ok: true, structures: out, counts: counts, truncated: total > 600, elapsed_ms: 9 };
    }

    function tileURL(cx, cz, bpp) {
      var c = document.createElement('canvas');
      c.width = TILE_PX; c.height = TILE_PX;
      var g = c.getContext('2d');
      var palette = ['#5b7a3f', '#6e8a4a', '#8ea35a', '#4f7d7a', '#a9a86a', '#3f5f63', '#c6bb84'];
      var step = 32;
      for (var y = 0; y < TILE_PX; y += step) {
        for (var x = 0; x < TILE_PX; x += step) {
          var h = hash(cx * 131 + x * 7 + cz * 17 + y * 13 + bpp);
          g.fillStyle = palette[Math.floor(h * palette.length)];
          g.fillRect(x, y, step, step);
          if (h > 0.86) {
            g.fillStyle = 'rgba(255,255,255,.12)';
            g.fillRect(x + 6, y + 8, step - 12, step - 16);
          }
        }
      }
      g.strokeStyle = 'rgba(0,0,0,.10)';
      g.lineWidth = 1;
      for (var i = 0; i <= TILE_PX; i += 64) {
        g.beginPath(); g.moveTo(i, 0); g.lineTo(i, TILE_PX); g.stroke();
        g.beginPath(); g.moveTo(0, i); g.lineTo(TILE_PX, i); g.stroke();
      }
      return c.toDataURL('image/png');
    }

    return {
      json: function (path, q) {
        if (path === '/manifest') return manifest();
        if (path === '/meta') return { ok: true, version: q.version, edition: q.edition, structures: manifest().structures, unsupported: [] };
        if (path === '/structures') return structures(q);
        if (path === '/stronghold') {
          var arr = [{ x: -1163, z: -172 }, { x: 402, z: 1180 }, { x: 1489, z: -640 }];
          return { ok: true, strongholds: arr, count: arr.length, precision: 'exact' };
        }
        if (path === '/slime') {
          var r = Math.min(96, Number(q.r) || 16);
          var cx = Number(q.cx) || 0, cz = Number(q.cz) || 0;
          var out = [];
          for (var i = -r; i <= r; i++) {
            for (var j = -r; j <= r; j++) {
              if (hash((cx + i) * 73856093 + (cz + j) * 19349663) < 0.11) out.push({ x: cx + i, z: cz + j });
            }
          }
          return { ok: true, chunks: out, count: out.length };
        }
        if (path === '/biome') {
          var names = [['平原', '#8db360'], ['森林', '#4f7a35'], ['沙漠', '#e0c860'], ['海洋', '#16418c'], ['雪原', '#eef4f4'], ['沼泽', '#51703d']];
          var pick = names[Math.floor(hash((Number(q.x) || 0) * 31 + (Number(q.z) || 0) * 17) * names.length)];
          return { ok: true, points: [{ x: Number(q.x) || 0, z: Number(q.z) || 0, key: 'minecraft:mock', name: pick[0], color: pick[1] }] };
        }
        return { ok: false, error: '示例数据里没有这个接口' };
      },
      tile: tileURL
    };
  })();

  /* ── 状态 ───────────────────────────────────────────────────────── */

  function newState() {
    return {
      started: false,
      manifest: null,
      defs: {},                 // 结构 id -> {id,name,dim,precision,color,default}
      dims: [],
      versions: [],             // [{edition,label}]
      edition: 'java',
      version: '',
      dim: 'overworld',
      large: 0,
      seedText: '',
      seed: '',

      view: { x: 0, z: 0, scale: 0.5 },
      size: { w: 0, h: 0 },
      layout: null,             // {sc,bpp,tb,ax,az,cols,rows}
      desired: new Set(),       // 当前这一批瓦片的 key
      tileEls: new Map(),       // key -> {el,bx,bz,bpp,tb}
      tileCache: new Map(),     // key -> objectURL（LRU）
      pending: new Set(),
      tileQueue: [],
      tileActive: 0,
      tileAborts: [],
      previewImg: null,
      previewAbort: null,
      engineDown: false,
      tileOK: false,

      selected: {},             // 当前维度下 id -> true
      data: null,               // {points:{id:[{x,z}]}, counts, truncated, key}
      dataKey: '',
      loadingStruct: false,
      structAbort: null,

      slime: false,
      slimeData: null,
      slimeAbort: null,
      strongholds: null,
      shLoading: false,
      shAbort: null,
      biomeToken: 0,

      hover: null,
      sel: null,
      iconItems: [],
      slimeItems: [],
      slimeOrigin: null,
      iconBucket: '',
      closed: {},               // 侧栏分组折叠
      expanded: {},             // 侧栏分组展开更多

      timers: {},
      paintIcons: false
    };
  }

  function later(name, fn, ms) {
    if (!S) return;
    clearTimeout(S.timers[name]);
    S.timers[name] = setTimeout(function () {
      if (!S) return;
      delete S.timers[name];
      fn();
    }, ms);
  }

  /* ── 接口 ───────────────────────────────────────────────────────── */

  function qs(params) {
    var parts = [];
    Object.keys(params || {}).forEach(function (k) {
      var v = params[k];
      if (v === undefined || v === null || v === '') return;
      parts.push(encodeURIComponent(k) + '=' + encodeURIComponent(v));
    });
    return parts.length ? ('?' + parts.join('&')) : '';
  }

  function rootFetch() {
    if (typeof yyRootFetch === 'function') return yyRootFetch;
    return window.fetch.bind(window);
  }

  function apiError(status, message) {
    var e = new Error(message || ('接口返回 ' + status));
    e.status = status;
    return e;
  }

  function getJSON(path, params, ctrl) {
    if (YY_SEEDMAP_MOCK) {
      return new Promise(function (resolve, reject) {
        setTimeout(function () {
          if (ctrl && ctrl.signal && ctrl.signal.aborted) { reject(apiError(0, '已取消')); return; }
          try {
            var data = MOCK.json(path, params || {});
            if (data && data.ok === false) reject(apiError(400, data.error || '示例数据返回失败'));
            else resolve(data);
          } catch (e) { reject(e); }
        }, 60);
      });
    }
    var url = API + path + qs(params);
    return rootFetch()(url, {
      method: 'GET',
      headers: { 'Accept': 'application/json' },
      credentials: 'same-origin',
      signal: ctrl ? ctrl.signal : undefined
    }).then(function (res) {
      return res.text().then(function (text) {
        var data = null;
        try { data = JSON.parse(text); } catch (e) { /* 下面统一给中文提示 */ }
        if (!res.ok) {
          throw apiError(res.status, (data && data.error) || ('接口返回 HTTP ' + res.status));
        }
        if (!data) throw apiError(res.status, '返回的不是可用的数据');
        if (data.ok === false) throw apiError(res.status, data.error || '接口返回失败');
        return data;
      });
    }, function (err) {
      if (err && err.name === 'AbortError') throw apiError(0, '已取消');
      throw apiError(0, (err && err.message) || '网络请求失败');
    });
  }

  function tileURLOf(cx, cz, bpp) {
    // 只画地形：结构图标由 DOM 层负责（可点击、固定 19px、有 tooltip）。
    // 不传 layer 的话后端会把图标烘进 PNG，和 DOM 徽标叠成双图标，
    // 拼图时边界上的图标还会被切一半。所以这里必须显式 layer=terrain。
    return API + '/tile.png' + qs({
      seed: S.seed,
      version: S.version,
      edition: S.edition,
      dim: S.dim,
      large: S.large ? 1 : 0,
      cx: cx,
      cz: cz,
      w: TILE_PX,
      h: TILE_PX,
      bpp: bpp,
      layer: 'terrain'
    });
  }

  /* ── 页面骨架 ───────────────────────────────────────────────────── */

  function pageHtml() {
    return '' +
      '<div class="sm2" id="sm2">' +
        '<div class="sm2-main">' +
          '<div class="sm2-bar">' +
            '<div class="sm2-seedrow">' +
              '<div class="sm2-field" id="sm2-seedfield">' +
                '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7"/><line x1="20" y1="20" x2="16.2" y2="16.2"/></svg>' +
                '<input id="sm2-seed" type="text" inputmode="numeric" autocomplete="off" spellcheck="false" placeholder="输入种子，可为负数" aria-label="世界种子">' +
                '<button class="sm2-clear" id="sm2-clear" type="button" aria-label="清空种子">×</button>' +
              '</div>' +
              '<label class="sm2-ver" for="sm2-version"><span>版本</span><select id="sm2-version" aria-label="游戏版本"></select></label>' +
              '<div class="sm2-dims" id="sm2-dims" role="group" aria-label="维度"></div>' +
              '<button class="sm2-go" id="sm2-go" type="button">查看</button>' +
            '</div>' +
            '<div class="sm2-layers">' +
              '<button class="sm2-layertoggle" id="sm2-layertoggle" type="button" aria-expanded="true" aria-controls="sm2-layerbody">' +
                '<b>结构图层</b>' +
                '<span class="sm2-dots" id="sm2-dots"></span>' +
                '<svg class="sm2-caret" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>' +
              '</button>' +
              '<div class="sm2-layerbody" id="sm2-layerbody">' +
                '<div class="sm2-chips" id="sm2-chips"></div>' +
                '<div class="sm2-layeracts">' +
                  '<button class="sm2-mini" type="button" data-act="all">全选</button>' +
                  '<button class="sm2-mini" type="button" data-act="none">全不选</button>' +
                  '<button class="sm2-mini" type="button" data-act="default">本维度默认</button>' +
                  '<label class="sm2-switch" id="sm2-slimewrap"><input type="checkbox" id="sm2-slime">史莱姆区块</label>' +
                '</div>' +
              '</div>' +
            '</div>' +
          '</div>' +

          '<div class="sm2-mapcard">' +
            '<div class="sm2-map" id="sm2-map" tabindex="0" aria-label="种子地图，可拖动缩放">' +
              '<div class="sm2-plane" id="sm2-plane"></div>' +
              '<div class="sm2-slimes" id="sm2-slimes"></div>' +
              '<div class="sm2-icons" id="sm2-icons"></div>' +
              '<div class="sm2-pin" id="sm2-pin"></div>' +
              '<div class="sm2-hover" id="sm2-hover" hidden></div>' +
              '<div class="sm2-readout" id="sm2-readout"></div>' +
              '<div class="sm2-dock">' +
                '<button type="button" data-act="out" aria-label="缩小">−</button>' +
                '<input class="sm2-zoom-slider" id="sm2-zoom" type="range" min="0" max="100" step="1" value="38" aria-label="缩放">' +
                '<button type="button" data-act="in" aria-label="放大">＋</button>' +
                '<button type="button" data-act="home" aria-label="回到原点">⌂</button>' +
              '</div>' +
              '<button class="sm2-drawerbtn" id="sm2-drawer" type="button">' +
                '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="7" x2="20" y2="7"/><line x1="4" y1="12" x2="20" y2="12"/><line x1="4" y1="17" x2="20" y2="17"/></svg>' +
                '结构列表</button>' +
              '<div class="sm2-loading" id="sm2-loading" hidden>' +
                '<div class="sm2-spinner"></div><span id="sm2-loadingtext">正在画地图…</span>' +
                '<div class="sm2-skel"><i></i><i></i><i></i></div>' +
              '</div>' +
              '<div class="sm2-empty" id="sm2-empty" hidden>' +
                '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a15 15 0 0 1 0 18M12 3a15 15 0 0 0 0 18"/></svg>' +
                '<strong id="sm2-empty-title"></strong>' +
                '<p id="sm2-empty-desc"></p>' +
                '<button class="sm2-mini" id="sm2-empty-btn" type="button" hidden>重试</button>' +
              '</div>' +
              '<div class="sm2-alert" id="sm2-alert" hidden><p id="sm2-alert-text"></p><button id="sm2-alert-btn" type="button">重试</button></div>' +
              '<div class="sm2-info" id="sm2-info" hidden>' +
                '<div class="sm2-info-head"><b id="sm2-info-title">坐标详情</b><button type="button" data-act="close" aria-label="关闭">×</button></div>' +
                '<div class="sm2-info-body" id="sm2-info-body"></div>' +
                '<div class="sm2-info-foot">' +
                  '<button class="sm2-mini" type="button" data-act="center">居中到此处</button>' +
                  '<button class="sm2-mini" type="button" data-act="copy">复制坐标</button>' +
                '</div>' +
              '</div>' +
            '</div>' +
          '</div>' +
        '</div>' +

        '<aside class="sm2-side" id="sm2-side">' +
          '<div class="sm2-card">' +
            '<div class="sm2-card-head"><h3>结构概览</h3><small id="sm2-total"></small>' +
              '<button class="sm2-close-drawer" id="sm2-closedrawer" type="button" aria-label="收起列表">×</button></div>' +
            '<div class="sm2-sum" id="sm2-sum"></div>' +
            '<p class="sm2-emptyline" id="sm2-summary"></p>' +
          '</div>' +
          '<div class="sm2-card">' +
            '<div class="sm2-card-head"><h3>结构列表</h3><small id="sm2-listnote"></small></div>' +
            '<div class="sm2-listbox" id="sm2-list"></div>' +
          '</div>' +
          '<div class="sm2-card">' +
            '<div class="sm2-card-head"><h3>要塞</h3><small id="sm2-shnote"></small></div>' +
            '<div class="sm2-btnrow"><button class="sm2-mini" id="sm2-shbtn" type="button">列出全部要塞</button></div>' +
            '<div class="sm2-shlist" id="sm2-shlist"></div>' +
          '</div>' +
          '<div class="sm2-card">' +
            '<div class="sm2-tip">' +
              '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><line x1="12" y1="8" x2="12" y2="8.01"/><path d="M11 12h1v5h1"/></svg>' +
              '<p>地图按区块取值。屏幕上的方块坐标 = 区块 × 16 + 8（区块中心）。拖动平移，滚轮或双指缩放，点一下看坐标与群系。</p>' +
            '</div>' +
          '</div>' +
        '</aside>' +
        '<div class="sm2-scrim" id="sm2-scrim"></div>' +
      '</div>';
  }

  /* ── manifest：版本与结构能力 ───────────────────────────────────── */

  function normalizeManifest(m) {
    var dims = Array.isArray(m.dimensions) && m.dimensions.length ? m.dimensions : [
      { id: 'overworld', name: '主世界' },
      { id: 'nether', name: '下界' },
      { id: 'end', name: '末地' }
    ];
    var versions = [];
    (m.editions || []).forEach(function (ed) {
      (ed.versions || []).forEach(function (v) {
        var label = typeof v === 'string' ? v : v.label;
        if (!label) return;
        versions.push({ edition: ed.id, label: label, editionName: ed.name || ed.id });
      });
    });
    var defs = {};
    (m.structures || []).forEach(function (s, i) {
      if (!s || !s.id) return;
      defs[s.id] = {
        id: s.id,
        name: s.name || NAME_FALLBACK[s.id] || '未知结构',
        dim: s.dim || 'overworld',
        precision: s.precision || 'candidate',
        color: s.color || COLOR_FALLBACK[i % COLOR_FALLBACK.length],
        def: s['default'] === true
      };
    });
    return { dims: dims, versions: versions, defs: defs };
  }

  function bootManifest() {
    hideEmpty();
    setLoading(true, '正在读取版本清单…');
    S.timers.manifest && clearTimeout(S.timers.manifest);
    getJSON('/manifest', {})
      .then(function (m) {
        if (!S) return;
        var norm = normalizeManifest(m);
        if (!norm.versions.length) throw apiError(0, '版本清单是空的');
        S.manifest = m;
        S.dims = norm.dims;
        S.versions = norm.versions;
        S.defs = norm.defs;
        var saved = storeRead();
        pickDefaults(saved);
        renderVersions();
        renderDims();
        renderChips();
        S.started = true;
        setLoading(false);
        applySeedInput(saved && typeof saved.seed === 'string' ? saved.seed : '0', true);
        readMeta();
      })
      .catch(function (err) {
        if (!S) return;
        console.warn('[种子地图] 版本清单读取失败', err);
        setLoading(false);
        setEmpty('版本清单没取到', (err && err.message ? err.message + '。' : '') + '检查本机服务是否启动，或稍后重试。', '重新读取', function () {
          bootManifest();
        });
      });
  }

  function pickDefaults(saved) {
    var has = function (ed, ver) {
      return S.versions.some(function (v) { return v.edition === ed && v.label === ver; });
    };
    if (saved && saved.edition && saved.version && has(saved.edition, saved.version)) {
      S.edition = saved.edition;
      S.version = saved.version;
    } else {
      var java = S.versions.filter(function (v) { return v.edition === 'java'; });
      var first = java.length ? java[0] : S.versions[0];
      S.edition = first.edition;
      S.version = first.label;
    }
    if (saved && saved.dim && S.dims.some(function (d) { return d.id === saved.dim; })) {
      S.dim = saved.dim;
    } else {
      S.dim = S.dims[0] ? S.dims[0].id : 'overworld';
    }
    S.selected = defaultSelection(S.dim);
  }

  /* ── 工具条渲染 ─────────────────────────────────────────────────── */

  function renderVersions() {
    var sel = $('sm2-version');
    if (!sel) return;
    var groups = {};
    var order = [];
    S.versions.forEach(function (v) {
      if (!groups[v.edition]) { groups[v.edition] = []; order.push(v.edition); }
      groups[v.edition].push(v);
    });
    sel.innerHTML = order.map(function (ed) {
      var name = groups[ed][0].editionName || ed;
      return '<optgroup label="' + esc(name) + '">' + groups[ed].map(function (v) {
        return '<option value="' + esc(v.edition + '|' + v.label) + '"' +
          (v.edition === S.edition && v.label === S.version ? ' selected' : '') + '>' + esc(v.label) + '</option>';
      }).join('') + '</optgroup>';
    }).join('');
  }

  function renderDims() {
    var box = $('sm2-dims');
    if (!box) return;
    box.innerHTML = S.dims.map(function (d) {
      return '<button type="button" data-dim="' + esc(d.id) + '"' +
        (d.id === S.dim ? ' class="on"' : '') + ' aria-pressed="' + (d.id === S.dim) + '">' + esc(d.name) + '</button>';
    }).join('');
  }

  function structuresForDim(dim) {
    return Object.keys(S.defs).filter(function (id) { return S.defs[id].dim === dim; })
      .map(function (id) { return S.defs[id]; });
  }

  function defaultSelection(dim) {
    var out = {};
    var list = structuresForDim(dim);
    var picked = list.filter(function (d) { return d.def; });
    if (!picked.length) picked = list.filter(function (d) { return d.precision === 'exact'; });
    if (!picked.length) picked = list.slice(0, 4);
    picked.forEach(function (d) { out[d.id] = true; });
    return out;
  }

  function selectedIds() {
    return Object.keys(S.selected).filter(function (k) { return S.selected[k]; });
  }

  function renderChips() {
    var box = $('sm2-chips');
    if (!box) return;
    var list = structuresForDim(S.dim);
    if (!list.length) {
      box.innerHTML = '<span class="sm2-emptyline">这个维度暂时没有可查的结构。</span>';
    } else {
      box.innerHTML = list.map(function (d) {
        var on = !!S.selected[d.id];
        return '<button type="button" class="sm2-chip' + (on ? ' on' : '') + '" data-id="' + esc(d.id) + '"' +
          ' style="--c:' + esc(d.color) + '" aria-pressed="' + on + '"><i></i>' + esc(d.name) + '</button>';
      }).join('');
    }
    renderDots();
    var wrap = $('sm2-slimewrap');
    if (wrap) {
      var on = S.dim === 'overworld';
      if (!on && S.slime) { S.slime = false; var cb = $('sm2-slime'); if (cb) cb.checked = false; renderSlimes(); }
      wrap.style.display = on ? '' : 'none';
      wrap.classList.toggle('on', !!S.slime);
    }
  }

  function renderDots() {
    var box = $('sm2-dots');
    if (!box) return;
    var ids = selectedIds();
    var dots = ids.slice(0, 5).map(function (id) {
      return '<i style="--c:' + esc(S.defs[id] ? S.defs[id].color : '#888') + '"></i>';
    }).join('');
    box.innerHTML = dots + '<small>' + (ids.length ? ('已选 ' + ids.length + ' 项') : '未选结构') + '</small>';
  }

  /* ── 种子输入 ───────────────────────────────────────────────────── */

  function parseSeed(text) {
    var raw = String(text == null ? '' : text).trim();
    raw = raw.replace(/[\s,，]/g, '').replace(/[−–—]/g, '-');
    if (!raw) return '';
    if (!/^[-+]?\d{1,19}$/.test(raw)) return null;
    return raw.replace(/^\+/, '');
  }

  function applySeedInput(text, silent) {
    var parsed = parseSeed(text);
    var input = $('sm2-seed');
    if (parsed === null) {
      if (!silent) toast('种子要填十进制整数，可以是负数', 'e');
      return false;
    }
    if (parsed === '') {
      S.seed = ''; S.seedText = '';
      S.data = null;
      if (input) input.value = '';
      syncSeedField();
      renderIcons();
      renderSidebar();
      updateReadout();
      setLoading(false);
      setEmpty('先填一个种子', '在左上角输入世界种子（十进制整数，可为负数），再点「查看」。', '', null);
      return false;
    }
    S.seed = parsed;
    S.seedText = parsed;
    if (input) input.value = parsed;
    syncSeedField();
    startQuery();
    return true;
  }

  function syncSeedField() {
    var field = $('sm2-seedfield');
    if (field) field.setAttribute('data-filled', S.seedText ? '1' : '0');
  }

  /* ── 查询生命周期 ───────────────────────────────────────────────── */

  function startQuery() {
    if (!S || !S.started || !S.seed) return;
    S.view = { x: 0, z: 0, scale: 0.5 };
    S.data = null;
    S.dataKey = '';
    S.slimeData = null;
    S.strongholds = null;
    S.shLoading = false;
    S.sel = null;
    S.iconItems = [];
    S.iconBucket = '';
    S.engineDown = false;
    S.tileOK = false;
    if (S.structAbort) { try { S.structAbort.abort(); } catch (e) {} S.structAbort = null; }
    if (S.slimeAbort) { try { S.slimeAbort.abort(); } catch (e) {} S.slimeAbort = null; }
    if (S.shAbort) { try { S.shAbort.abort(); } catch (e) {} S.shAbort = null; }

    clearTiles();
    loadPreview();
    renderIcons();
    renderSlimes();
    hideInfo();
    setAlert('');
    hideEmpty();
    updateReadout();
    storeWrite();

    renderSidebar();
    S.slimeItems = [];

    relayout(true);
    loadStructures(true);
    loadStrongholds(false);
    if (S.slime) loadSlimes(true);
  }

  function onQueryChanged() {
    storeWrite();
    startQuery();
  }

  /* 只改了结构勾选：地形瓦片不受影响，所以不清瓦片、不重新下载，
     只清图标与结构列表，再按新勾选重新取一遍结构坐标。 */
  function refreshLayers() {
    if (!S || !S.started || !S.seed) return;
    storeWrite();
    S.data = null;
    S.dataKey = '';
    S.iconBucket = '';
    renderIcons();
    renderSidebar();
    loadStructures(true);
  }

  /* ── 地图：尺寸 ─────────────────────────────────────────────────── */

  function measure() {
    var el = $('sm2-map');
    if (!el || !S) return false;
    S.size.w = el.clientWidth;
    S.size.h = el.clientHeight;
    return S.size.w > 8 && S.size.h > 8;
  }

  function afterLayout() {
    var tries = 0;
    (function tick() {
      if (!S) return;
      if (measure()) {
        relayout(true);
        applyView();
        return;
      }
      if (++tries > 30) {
        console.warn('[种子地图] 地图容器一直没有尺寸，放弃首帧绘制');
        return;
      }
      setTimeout(tick, 60);
    })();
  }

  /* ── 地图：瓦片 ─────────────────────────────────────────────────── */

  function computeBpp(scale) {
    // 一张 256px 的瓦片约占 512 屏幕像素：细节和请求数都合适。
    // 注意不是「越清晰越好」——bpp 越小请求越多，服务端渲染也越慢。
    var want = 2 / Math.max(scale, 0.0001);
    var list = [1, 2, 4, 8, 16, 32];
    for (var i = 0; i < list.length; i++) if (list[i] >= want - 0.001) return list[i];
    return 32;
  }

  function tileKey(cx, cz, bpp) {
    // 瓦片只含地形，所以 key 里不带结构选择 —— 勾/取消结构不会重复下载同一块地形。
    return [S.seed, S.edition, S.version, S.dim, S.large, bpp, cx, cz].join('|');
  }

  function relayout(force) {
    if (!S || !S.started || !S.size.w) return;
    var vw = S.size.w, vh = S.size.h, sc = S.view.scale;
    var bpp = computeBpp(sc);
    var tb = TILE_PX * bpp;
    var cols, rows;
    for (;;) {
      cols = Math.ceil(vw / (tb * sc)) + 1;
      rows = Math.ceil(vh / (tb * sc)) + 1;
      if (cols * rows <= 24 || bpp >= 32) break;
      bpp *= 2; tb = TILE_PX * bpp;
    }
    var ax = Math.floor((S.view.x - (vw / 2) / sc) / tb) * tb;
    var az = Math.floor((S.view.z - (vh / 2) / sc) / tb) * tb;
    var same = S.layout && S.layout.sc === sc && S.layout.bpp === bpp &&
      S.layout.ax === ax && S.layout.az === az && S.layout.cols === cols && S.layout.rows === rows;
    if (same && !force) { applyView(); return; }

    S.layout = { sc: sc, bpp: bpp, tb: tb, ax: ax, az: az, cols: cols, rows: rows };
    S.desired = new Set();
    placePreview();

    for (var i = 0; i < cols; i++) {
      for (var j = 0; j < rows; j++) {
        var bx = ax + i * tb;
        var bz = az + j * tb;
        var key = tileKey(bx / 16, bz / 16, bpp);
        S.desired.add(key);
        ensureTile(key, bx, bz, bpp, tb, ax, az, force);
      }
    }
    // 位置统一重排（anchor 可能变了）
    S.tileEls.forEach(function (rec) {
      placeTile(rec, ax, az);
    });
    // 元素太多时立刻清掉不在本批里的
    if (S.tileEls.size > MAX_TILE_ELEMS) sweepTiles(true);
    sweepTiles(false);

    applyView();
    updateReadout();
    renderIcons();
    if (S.slimeOrigin && Math.abs(S.view.scale / S.slimeOrigin.sc - 1) > 1.5) renderSlimes();
    pumpTiles();
  }

  function placeTile(rec, ax, az) {
    rec.el.style.left = (rec.bx - ax) + 'px';
    rec.el.style.top = (rec.bz - az) + 'px';
    rec.el.style.width = rec.tb + 'px';
    rec.el.style.height = rec.tb + 'px';
    rec.el.classList.toggle('pixel', rec.bpp * S.view.scale > 1.4);
  }

  function ensureTile(key, bx, bz, bpp, tb, ax, az, force) {
    var rec = S.tileEls.get(key);
    if (rec) {
      placeTile(rec, ax, az);
      if (force) {
        rec.el.classList.remove('fail');
        if (!rec.el.getAttribute('src')) {
          S.pending.delete(key);
          queueTile(key, rec);
        }
      }
      return;
    }
    var img = document.createElement('img');
    img.className = 'sm2-tile';
    img.alt = '';
    img.draggable = false;
    img.setAttribute('decoding', 'async');
    rec = { el: img, bx: bx, bz: bz, bpp: bpp, tb: tb };
    S.tileEls.set(key, rec);
    placeTile(rec, ax, az);
    S.planeEl.appendChild(img);

    var cached = S.tileCache.get(key);
    if (cached) {
      S.tileCache.delete(key);
      S.tileCache.set(key, cached);   // 刷新 LRU 位置
      img.src = cached;
      S.tileOK = true;
      return;
    }
    queueTile(key, rec);
  }

  function queueTile(key, rec) {
    if (S.engineDown) return;
    if (S.pending.has(key)) return;
    S.pending.add(key);
    S.tileQueue.push({ key: key, rec: rec });
    updateLoading();
  }

  function pumpTiles() {
    if (!S) return;
    while (S.tileActive < 4 && S.tileQueue.length && !S.engineDown) {
      var job = S.tileQueue.shift();
      if (!S.tileEls.has(job.key)) { S.pending.delete(job.key); continue; }
      startTile(job);
    }
    if (!S.tileQueue.length) updateLoading();
    if (!S.pending.size) {
      sweepTiles(false);
      if (S.tileOK) hidePreview();
    }
  }

  /* 首屏先要一张 bpp=16 的粗图垫底，细瓦片到齐后淡出。
     粗图只发一个请求，网络慢的时候用户至少能看见地形轮廓。 */
  function loadPreview() {
    if (!S || !S.seed || YY_SEEDMAP_MOCK || S.engineDown) return;
    removePreview();
    var span = 512 * 16;
    var bx = Math.floor((S.view.x - span / 2) / 16) * 16;
    var bz = Math.floor((S.view.z - span / 2) / 16) * 16;
    var img = document.createElement('img');
    img.className = 'sm2-preview';
    img.alt = '';
    img.dataset.bx = bx;
    img.dataset.bz = bz;
    img.style.width = span + 'px';
    img.style.height = span + 'px';
    S.previewImg = img;
    placePreview();
    if (S.planeEl.firstChild) S.planeEl.insertBefore(img, S.planeEl.firstChild);
    else S.planeEl.appendChild(img);

    var ctrl = new AbortController();
    S.previewAbort = ctrl;
    var url = API + '/tile.png' + qs({
      seed: S.seed, version: S.version, edition: S.edition, dim: S.dim,
      large: S.large ? 1 : 0, cx: bx / 16, cz: bz / 16,
      w: 512, h: 512, bpp: 16, layer: 'terrain'
    });
    rootFetch()(url, { method: 'GET', credentials: 'same-origin', signal: ctrl.signal })
      .then(function (res) {
        if (!res.ok) throw apiError(res.status, '粗图没取到');
        return res.blob();
      })
      .then(function (blob) {
        if (!S || ctrl.signal.aborted || S.previewImg !== img) return;
        img._obj = URL.createObjectURL(blob);
        img.src = img._obj;
      })
      .catch(function (err) {
        if (err && err.name === 'AbortError') return;
        if (S && S.previewImg === img) removePreview();
      });
  }

  function placePreview() {
    if (!S || !S.previewImg || !S.layout) return;
    var img = S.previewImg;
    var bx = Number(img.dataset.bx) || 0;
    var bz = Number(img.dataset.bz) || 0;
    img.style.left = (bx - S.layout.ax) + 'px';
    img.style.top = (bz - S.layout.az) + 'px';
  }

  function hidePreview() {
    if (!S || !S.previewImg) return;
    var img = S.previewImg;
    img.classList.add('gone');
    S.previewImg = null;
    setTimeout(function () {
      if (img._obj) { try { URL.revokeObjectURL(img._obj); } catch (e) {} }
      if (img.parentNode) img.parentNode.removeChild(img);
    }, 260);
  }

  function removePreview() {
    if (!S) return;
    if (S.previewAbort) { try { S.previewAbort.abort(); } catch (e) {} S.previewAbort = null; }
    var img = S.previewImg;
    if (!img) return;
    S.previewImg = null;
    if (img._obj) { try { URL.revokeObjectURL(img._obj); } catch (e) {} }
    if (img.parentNode) img.parentNode.removeChild(img);
  }

  function startTile(job) {
    S.tileActive++;
    var key = job.key, rec = job.rec;
    var ctrl = new AbortController();
    rec.ctrl = ctrl;
    S.tileAborts.push(ctrl);
    var finish = function () {
      if (!S) return;
      if (rec.ctrl === ctrl) rec.ctrl = null;
      S.tileActive--;
      S.pending.delete(key);
      var idx = S.tileAborts.indexOf(ctrl);
      if (idx >= 0) S.tileAborts.splice(idx, 1);
      pumpTiles();
      updateLoading();
    };

    if (YY_SEEDMAP_MOCK) {
      setTimeout(function () {
        if (!S || ctrl.signal.aborted) { finish(); return; }
        var url = MOCK.tile(rec.bx / 16, rec.bz / 16, rec.bpp);
        rec.el.src = url;
        rec.el.classList.remove('fail');
        S.tileOK = true;
        cacheTile(key, url, true);
        finish();
      }, 40);
      return;
    }

    var url = tileURLOf(rec.bx / 16, rec.bz / 16, rec.bpp);
    rootFetch()(url, { method: 'GET', credentials: 'same-origin', signal: ctrl.signal })
      .then(function (res) {
        if (!res.ok) {
          return res.text().then(function (t) {
            var msg = '';
            try { msg = (JSON.parse(t) || {}).error || ''; } catch (e) {}
            throw apiError(res.status, msg);
          });
        }
        var ct = (res.headers.get('content-type') || '').toLowerCase();
        if (ct && ct.indexOf('image') < 0) throw apiError(res.status, '瓦片返回的不是图片');
        return res.blob();
      })
      .then(function (blob) {
        if (!S || ctrl.signal.aborted) { finish(); return; }
        var obj = URL.createObjectURL(blob);
        rec.el.src = obj;
        rec.el.classList.remove('fail');
        S.tileOK = true;
        cacheTile(key, obj, true);
        finish();
      })
      .catch(function (err) {
        if (!S) { finish(); return; }
        if (err && err.name === 'AbortError') { finish(); return; }
        if (err && err.status === 503) {
          onEngineDown(err.message);
        } else if (err && err.status !== 0) {
          console.warn('[种子地图] 瓦片加载失败', err.status, err.message);
        } else if (err && err.message !== '已取消') {
          console.warn('[种子地图] 瓦片请求出错', err.message);
        }
        rec.el.classList.add('fail');
        finish();
      });
  }

  function cacheTile(key, url, revokable) {
    if (!S) return;
    var old = S.tileCache.get(key);
    if (old && old !== url) URL.revokeObjectURL(old);
    S.tileCache.set(key, url);
    if (!revokable) return;
    if (S.tileCache.size <= TILE_CACHE_MAX) return;
    var used = new Set();
    S.tileEls.forEach(function (r) { if (r.el.src) used.add(r.el.src); });
    while (S.tileCache.size > TILE_CACHE_MAX) {
      var firstKey = S.tileCache.keys().next().value;
      var firstVal = S.tileCache.get(firstKey);
      S.tileCache.delete(firstKey);
      if (used.has(firstVal)) break;     // 正在显示的不动
      try { URL.revokeObjectURL(firstVal); } catch (e) {}
    }
  }

  function sweepTiles(force) {
    if (!S || !S.desired) return;
    if (!force && S.pending.size) return;
    if (!force && S.engineDown) return;   // 引擎挂了就保留旧图，别把画面清空
    var stale = [];
    S.tileEls.forEach(function (rec, key) {
      if (!S.desired.has(key)) stale.push(key);
    });
    stale.forEach(function (key) {
      var rec = S.tileEls.get(key);
      S.tileEls.delete(key);
      if (rec && rec.ctrl) { try { rec.ctrl.abort(); } catch (e) {} }
      if (rec && rec.el.parentNode) rec.el.parentNode.removeChild(rec.el);
    });
  }

  function clearTiles() {
    if (!S) return;
    removePreview();
    S.tileAborts.forEach(function (c) { try { c.abort(); } catch (e) {} });
    S.tileAborts = [];
    S.tileQueue = [];
    S.pending.clear();
    S.tileActive = 0;
    S.tileEls.forEach(function (rec) {
      if (rec.el.parentNode) rec.el.parentNode.removeChild(rec.el);
    });
    S.tileEls.clear();
    S.desired = new Set();
    S.layout = null;
    S.tileOK = false;
  }

  function releaseTiles() {
    if (!S) return;
    S.tileAborts.forEach(function (c) { try { c.abort(); } catch (e) {} });
    S.tileAborts = [];
    S.tileCache.forEach(function (url) { try { URL.revokeObjectURL(url); } catch (e) {} });
    S.tileCache.clear();
  }

  function onEngineDown(msg) {
    if (!S || S.engineDown) return;
    S.engineDown = true;
    S.tileQueue = [];
    S.pending.clear();
    removePreview();
    setLoading(false);
    setEmpty('地图渲染引擎正在准备',
      (msg ? msg + '。' : '') + '地形瓦片暂时画不出来，结构列表、要塞和史莱姆区块仍然能用。稍后点「重试」。',
      '重试地图', function () {
        S.engineDown = false;
        S.tileOK = false;
        hideEmpty();
        relayout(true);
      });
    console.warn('[种子地图] 地形瓦片不可用', msg || '');
  }

  /* ── 地图：视图 ─────────────────────────────────────────────────── */

  function applyView() {
    if (!S || !S.layout) return;
    var L = S.layout, sc = S.view.scale;
    var tx = S.size.w / 2 - (S.view.x - L.ax) * sc;
    var ty = S.size.h / 2 - (S.view.z - L.az) * sc;
    S.planeEl.style.transform = 'translate3d(' + tx + 'px,' + ty + 'px,0) scale(' + sc + ')';
    S.iconsEl.style.transform = 'translate3d(' + tx + 'px,' + ty + 'px,0)';
    applySlimeTransform();
    placePin();
    updateZoomSlider();
  }

  function viewChanged(zoomChanged) {
    if (!S) return;
    schedulePaint(zoomChanged);
    later('layout', function () { relayout(false); }, zoomChanged ? 170 : 90);
    later('data', function () {
      maybeReloadData();
      renderStrongholds();
    }, 480);
  }

  function schedulePaint(icons) {
    if (!S) return;
    if (icons) S.paintIcons = true;
    if (rafId) return;
    rafId = requestAnimationFrame(function () {
      rafId = 0;
      if (!S) return;
      applyView();
      updateReadout();
      if (S.paintIcons) {
        S.paintIcons = false;
        placeIcons();
      }
    });
  }

  function zoomAt(px, py, factor) {
    if (!S) return;
    var sc = S.view.scale;
    var next = clamp(sc * factor, MIN_SCALE, MAX_SCALE);
    if (Math.abs(next - sc) < 1e-6) return;
    var wx = S.view.x + (px - S.size.w / 2) / sc;
    var wz = S.view.z + (py - S.size.h / 2) / sc;
    S.view.x = wx - (px - S.size.w / 2) / next;
    S.view.z = wz - (py - S.size.h / 2) / next;
    S.view.scale = next;
    viewChanged(true);
  }

  function setScale(next, px, py) {
    if (!S) return;
    if (px == null) { px = S.size.w / 2; py = S.size.h / 2; }
    zoomAt(px, py, next / S.view.scale);
  }

  function sliderToScale(v) {
    return MIN_SCALE * Math.pow(MAX_SCALE / MIN_SCALE, clamp(v, 0, 100) / 100);
  }

  function scaleToSlider(sc) {
    return clamp(Math.log(sc / MIN_SCALE) / Math.log(MAX_SCALE / MIN_SCALE) * 100, 0, 100);
  }

  function updateZoomSlider() {
    var sl = $('sm2-zoom');
    if (!sl || !S) return;
    var v = String(Math.round(scaleToSlider(S.view.scale)));
    if (sl.value !== v) sl.value = v;
  }

  function localPoint(e) {
    var rect = S.mapEl.getBoundingClientRect();
    var k = (rect.width && S.size.w) ? rect.width / S.size.w : 1;
    return {
      x: (e.clientX - rect.left) / k,
      y: (e.clientY - rect.top) / k
    };
  }

  function updateReadout() {
    var el = $('sm2-readout');
    if (!el || !S) return;
    var cx = Math.round(S.view.x / 16);
    var cz = Math.round(S.view.z / 16);
    var per = S.layout ? Math.round(S.view.scale * S.layout.bpp * 10) / 10 : 0;
    el.innerHTML = '种子 <b>' + esc(S.seed || '—') + '</b>' +
      '<span class="sm2-sep"></span>' +
      '<span>中心</span> <b>' + fmtInt(S.view.x) + ', ' + fmtInt(S.view.z) + '</b>' +
      '<span class="sm2-sep"></span>' +
      '<span class="sm2-hide-sm">区块 ' + fmtInt(cx) + ', ' + fmtInt(cz) +
      (per ? ' · 每像素 ' + per + ' 方块' : '') + '</span>';
  }

  function placePin() {
    if (!S) return;
    var pin = $('sm2-pin');
    if (!pin) return;
    if (!S.sel) { pin.style.display = 'none'; return; }
    var sc = S.view.scale;
    var x = S.size.w / 2 + (S.sel.x - S.view.x) * sc;
    var y = S.size.h / 2 + (S.sel.z - S.view.z) * sc;
    pin.style.display = 'block';
    pin.style.left = x + 'px';
    pin.style.top = y + 'px';
  }

  /* ── 地图：指针交互 ─────────────────────────────────────────────── */

  function bindMap() {
    var map = S.mapEl;
    var pointers = new Map();
    var drag = null;
    var pinch = null;

    function onDown(e) {
      if (!S) return;
      if (e.button != null && e.button !== 0) return;
      if (closest(e.target, 'button, input, .sm2-info')) return;
      map.focus({ preventScroll: true });
      try { map.setPointerCapture(e.pointerId); } catch (err) {}
      pointers.set(e.pointerId, localPoint(e));
      if (pointers.size === 1) {
        drag = { x: e.clientX, y: e.clientY, moved: 0, at: Date.now() };
      } else if (pointers.size === 2) {
        var pts = Array.from(pointers.values());
        pinch = {
          d: Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y) || 1,
          mid: { x: (pts[0].x + pts[1].x) / 2, y: (pts[0].y + pts[1].y) / 2 }
        };
        drag = null;
      }
      map.classList.add('dragging');
      hideHover();
      if (e.pointerType === 'mouse') e.preventDefault();
    }

    function onMove(e) {
      if (!S) return;
      var p = localPoint(e);
      if (!pointers.has(e.pointerId)) {
        if (e.pointerType === 'mouse') hoverAt(p);
        return;
      }
      if (pointers.size >= 2) {
        pointers.set(e.pointerId, p);
        var pts = Array.from(pointers.values()).slice(0, 2);
        var d = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y) || 1;
        var mid = { x: (pts[0].x + pts[1].x) / 2, y: (pts[0].y + pts[1].y) / 2 };
        if (pinch) {
          var sc = S.view.scale;
          S.view.x -= (mid.x - pinch.mid.x) / sc;
          S.view.z -= (mid.y - pinch.mid.y) / sc;
          applyView();
          zoomAt(mid.x, mid.y, d / pinch.d);
          pinch = { d: d, mid: mid };
        }
        hideHover();
        return;
      }
      var prev = pointers.get(e.pointerId);
      var dx = p.x - prev.x;
      var dy = p.y - prev.y;
      pointers.set(e.pointerId, p);
      var sc = S.view.scale;
      S.view.x -= dx / sc;
      S.view.z -= dy / sc;
      if (drag) drag.moved += Math.abs(dx) + Math.abs(dy);
      hideHover();
      schedulePaint();
    }

    function onUp(e) {
      if (!S) return;
      var p = pointers.get(e.pointerId);
      var wasDrag = drag;
      pointers.delete(e.pointerId);
      if (pointers.size < 2) pinch = null;
      if (!pointers.size) {
        map.classList.remove('dragging');
        if (wasDrag && wasDrag.moved < 6 && Date.now() - wasDrag.at < 700) {
          mapClick(p || localPoint(e));
        }
        drag = null;
        viewChanged(false);
      }
    }

    function onCancel(e) {
      pointers.delete(e.pointerId);
      if (!pointers.size) { drag = null; pinch = null; map.classList.remove('dragging'); }
    }

    map.addEventListener('pointerdown', onDown);
    map.addEventListener('pointermove', onMove);
    map.addEventListener('pointerup', onUp);
    map.addEventListener('pointercancel', onCancel);

    map.addEventListener('wheel', function (e) {
      if (!S) return;
      e.preventDefault();
      var p = localPoint(e);
      var dy = e.deltaY;
      if (e.deltaMode === 1) dy *= 16;
      else if (e.deltaMode === 2) dy *= 320;
      var factor = Math.pow(1.0016, clamp(-dy, -600, 600));
      zoomAt(p.x, p.y, factor);
      hideHover();
    }, { passive: false });

    map.addEventListener('dblclick', function (e) {
      if (!S) return;
      if (closest(e.target, 'button, input, .sm2-info')) return;
      e.preventDefault();
      var p = localPoint(e);
      zoomAt(p.x, p.y, 1.9);
    });

    map.addEventListener('mouseleave', hideHover);

    map.addEventListener('keydown', function (e) {
      if (!S) return;
      var step = 80 / S.view.scale;
      if (e.key === 'ArrowLeft') { S.view.x -= step; viewChanged(false); }
      else if (e.key === 'ArrowRight') { S.view.x += step; viewChanged(false); }
      else if (e.key === 'ArrowUp') { S.view.z -= step; viewChanged(false); }
      else if (e.key === 'ArrowDown') { S.view.z += step; viewChanged(false); }
      else if (e.key === '+' || e.key === '=') { zoomAt(S.size.w / 2, S.size.h / 2, 1.5); }
      else if (e.key === '-' || e.key === '_') { zoomAt(S.size.w / 2, S.size.h / 2, 1 / 1.5); }
      else if (e.key === '0') { S.view.x = 0; S.view.z = 0; viewChanged(false); }
      else return;
      e.preventDefault();
    });

    cleanups.push(function () {
      map.removeEventListener('pointerdown', onDown);
      map.removeEventListener('pointermove', onMove);
      map.removeEventListener('pointerup', onUp);
      map.removeEventListener('pointercancel', onCancel);
    });
  }

  function mapClick(p) {
    if (!S || !S.started) return;
    if (!S.seed) return;
    var sc = S.view.scale;
    var bx = S.view.x + (p.x - S.size.w / 2) / sc;
    var bz = S.view.z + (p.y - S.size.h / 2) / sc;
    S.sel = { x: bx, z: bz };
    placePin();
    showInfo({ x: bx, z: bz });
    loadBiome(bx, bz);
  }

  /* ── 悬停读数 ───────────────────────────────────────────────────── */

  function hoverAt(p) {
    if (!S || !S.started) return;
    var hover = $('sm2-hover');
    if (!hover) return;
    var sc = S.view.scale;
    var bx = S.view.x + (p.x - S.size.w / 2) / sc;
    var bz = S.view.z + (p.y - S.size.h / 2) / sc;
    hover.hidden = false;
    hover.style.left = p.x + 'px';
    hover.style.top = p.y + 'px';
    hover.textContent = '方块 ' + fmtInt(bx) + ', ' + fmtInt(bz) +
      ' · 区块 ' + fmtInt(Math.floor(bx / 16)) + ', ' + fmtInt(Math.floor(bz / 16));
  }

  function hideHover() {
    var hover = $('sm2-hover');
    if (hover) hover.hidden = true;
  }

  /* ── 结构数据 ───────────────────────────────────────────────────── */

  function typesSignature() { return selectedIds().slice().sort().join(','); }

  function dataKeyNow() {
    return [S.seed, S.edition, S.version, S.dim, typesSignature()].join('|');
  }

  function dataRadius() {
    var span = Math.max(S.size.w, S.size.h) / Math.max(S.view.scale, 0.0001);
    return Math.round(clamp(span / 32 + 12, 24, 512));
  }

  function maybeReloadData() {
    if (!S || !S.started || !S.seed) return;
    maybeReloadSlimes();
    if (!selectedIds().length) {
      S.data = null;
      renderIcons();
      renderSidebar();
      return;
    }
    if (S.dataKey !== dataKeyNow()) { loadStructures(true); return; }
    var d = S.data;
    if (!d) { loadStructures(true); return; }
    var r = dataRadius();
    var moved = Math.abs(chunkOf(S.view.x) - d.cx) + Math.abs(chunkOf(S.view.z) - d.cz);
    if (moved > Math.max(8, d.r * 0.45) || r > d.r * 1.35) loadStructures(false);
  }

  function slimeRadius() {
    return Math.round(clamp(Math.max(S.size.w, S.size.h) / S.view.scale / 32 + 4, 8, 96));
  }

  function maybeReloadSlimes() {
    if (!S.slime || S.dim !== 'overworld') return;
    if (!S.slimeData) { loadSlimes(true); return; }
    var cx = chunkOf(S.view.x), cz = chunkOf(S.view.z);
    var moved = Math.abs(cx - S.slimeData.cx) + Math.abs(cz - S.slimeData.cz);
    if (moved > Math.max(6, S.slimeData.r * 0.5) || slimeRadius() > S.slimeData.r * 1.4) {
      loadSlimes(true);
    }
  }

  function loadStructures(force) {
    if (!S || !S.seed) return;
    var ids = selectedIds();
    if (!ids.length) {
      if (S.structAbort) { try { S.structAbort.abort(); } catch (e) {} S.structAbort = null; }
      S.data = null;
      renderIcons();
      renderSidebar();
      return;
    }
    var key = dataKeyNow();
    if (!force && S.data && S.dataKey === key) {
      var moved = Math.abs(chunkOf(S.view.x) - S.data.cx) + Math.abs(chunkOf(S.view.z) - S.data.cz);
      if (moved < Math.max(6, S.data.r * 0.4)) return;
    }
    if (S.structAbort) { try { S.structAbort.abort(); } catch (e) {} }
    var ctrl = new AbortController();
    S.structAbort = ctrl;
    S.loadingStruct = true;
    var params = {
      seed: S.seed,
      version: S.version,
      edition: S.edition,
      dim: S.dim,
      types: ids.join(','),
      cx: chunkOf(S.view.x),
      cz: chunkOf(S.view.z),
      r: dataRadius(),
      limit: 20000
    };
    renderListNote('正在统计…');

    getJSON('/structures', params, ctrl).then(function (data) {
      if (!S || ctrl.signal.aborted) return;
      S.loadingStruct = false;
      var points = {}, total = 0;
      var src = data.structures || {};
      Object.keys(src).forEach(function (id) {
        var arr = Array.isArray(src[id]) ? src[id] : [];
        var flat = [];
        for (var i = 0; i < arr.length; i++) {
          var p = arr[i];
          if (!p || !isNum(p.x) || !isNum(p.z)) continue;
          flat.push({ x: p.x, z: p.z, bx: blockOf(p.x), bz: blockOf(p.z), extra: p });
        }
        points[id] = flat;
        total += flat.length;
      });
      S.data = {
        key: key,
        cx: params.cx,
        cz: params.cz,
        r: params.r,
        points: points,
        counts: data.counts || {},
        truncated: !!data.truncated,
        total: total
      };
      S.dataKey = key;
      S.iconBucket = '';
      renderIcons();
      renderSidebar();
      setAlert('');
    }).catch(function (err) {
      if (!S || (err && err.message === '已取消')) return;
      if (ctrl.signal.aborted) return;
      S.loadingStruct = false;
      console.warn('[种子地图] 结构列表读取失败', err);
      renderListNote('没取到');
      setAlert('结构列表没取到：' + (err.message || '未知原因'), function () { loadStructures(true); });
    });
  }

  function renderListNote(text) {
    var el = $('sm2-listnote');
    if (el) el.textContent = text || '';
  }

  /* ── 图标层 ─────────────────────────────────────────────────────── */

  function iconBudget() {
    var sc = S.view.scale;
    if (sc < 0.09) return { max: 70, exactOnly: true };
    if (sc < 0.22) return { max: 170, exactOnly: true };
    if (sc < 0.6) return { max: 280, exactOnly: false };
    return { max: MAX_ICONS, exactOnly: false };
  }

  function renderIcons() {
    var box = $('sm2-icons');
    if (!box || !S) return;
    var budget = iconBudget();
    var bucket = (S.data ? S.data.key : '') + '#' + budget.max + (budget.exactOnly ? 'E' : 'A');
    if (S.iconBucket === bucket) { placeIcons(); return; }
    S.iconBucket = bucket;
    box.innerHTML = '';
    S.iconItems = [];
    if (!S.data) return;

    var sc = S.view.scale;
    var vw = S.size.w, vh = S.size.h;
    var margin = 80;
    var list = [];
    // 末地船是末地城的子集（同一坐标），后端两类都会返回。
    // 图标层只留船那一个，否则同一座城会叠两个图标。
    var shipKeys = null;
    if (S.data.points.endShip && S.data.points.endShip.length) {
      shipKeys = new Set();
      S.data.points.endShip.forEach(function (p) { shipKeys.add(p.bx + ',' + p.bz); });
    }
    Object.keys(S.data.points).forEach(function (id) {
      var def = S.defs[id];
      if (budget.exactOnly && def && def.precision !== 'exact') return;
      var arr = S.data.points[id];
      for (var i = 0; i < arr.length; i++) {
        var p = arr[i];
        if (id === 'endCity' && shipKeys && shipKeys.has(p.bx + ',' + p.bz)) continue;
        var sx = vw / 2 + (p.bx - S.view.x) * sc;
        var sz = vh / 2 + (p.bz - S.view.z) * sc;
        if (sx < -margin || sx > vw + margin || sz < -margin || sz > vh + margin) continue;
        list.push({ id: id, p: p, d: Math.abs(sx - vw / 2) + Math.abs(sz - vh / 2) });
      }
    });
    list.sort(function (a, b) { return a.d - b.d; });
    if (list.length > budget.max) list = list.slice(0, budget.max);

    var frag = document.createDocumentFragment();
    list.forEach(function (item) {
      var def = S.defs[item.id] || {};
      var name = def.name || NAME_FALLBACK[item.id] || '结构';
      var btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'sm2-icon';
      btn.style.setProperty('--c', def.color || '#888');
      btn.title = name + ' · 方块 ' + fmtInt(item.p.bx) + ', ' + fmtInt(item.p.bz);
      btn.setAttribute('aria-label', btn.title);
      if (item.id === 'stronghold') btn.setAttribute('data-strong', '1');
      var badge = document.createElement('span');
      badge.className = 'sm2-badge';
      if (YY_SEEDMAP_MOCK) {
        badge.innerHTML = glyphSvg();
      } else {
        // 后端返回的是「整枚圆形图标」（底色 + 白色几何图形），所以按徽标尺寸 19px 铺满，
        // 不要再叠一层小图标，否则会变成双圈。
        var img = document.createElement('img');
        img.src = API + '/icon.svg?id=' + encodeURIComponent(item.id) + '&size=20';
        img.alt = '';
        img.width = 19; img.height = 19;
        img.style.width = '19px'; img.style.height = '19px';
        img.onerror = function () {
          if (img.parentNode) img.parentNode.innerHTML = glyphSvg();
        };
        badge.appendChild(img);
      }
      btn.appendChild(badge);
      btn.addEventListener('click', function (e) {
        e.stopPropagation();
        if (!S) return;
        S.sel = { x: item.p.bx, z: item.p.bz };
        placePin();
        showInfo({ x: item.p.bx, z: item.p.bz, id: item.id });
        loadBiome(item.p.bx, item.p.bz);
      });
      frag.appendChild(btn);
      S.iconItems.push({ el: btn, bx: item.p.bx, bz: item.p.bz });
    });
    box.appendChild(frag);
    placeIcons();
  }

  function glyphSvg() {
    return '<svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="3.1" fill="currentColor"/></svg>';
  }

  function placeIcons() {
    if (!S || !S.layout) return;
    var sc = S.view.scale;
    var ax = S.layout.ax, az = S.layout.az;
    var vw = S.size.w, vh = S.size.h, margin = 60;
    // 图标挂在随平移移动的图层里，坐标是「相对瓦片锚点」的图层坐标，
    // 判断是否在视野内必须再加图层自身的位移，否则会误判成屏幕外。
    var tx = vw / 2 - (S.view.x - ax) * sc;
    var ty = vh / 2 - (S.view.z - az) * sc;
    for (var i = 0; i < S.iconItems.length; i++) {
      var it = S.iconItems[i];
      var lx = (it.bx - ax) * sc - 12;
      var lz = (it.bz - az) * sc - 12;
      var sx = lx + tx, sy = lz + ty;
      var onScreen = sx > -margin && sx < vw + margin && sy > -margin && sy < vh + margin;
      it.el.style.display = onScreen ? '' : 'none';
      if (onScreen) it.el.style.transform = 'translate3d(' + lx + 'px,' + lz + 'px,0)';
    }
  }

  /* ── 史莱姆区块 ─────────────────────────────────────────────────── */

  function loadSlimes(force) {
    if (!S || !S.seed) return;
    if (S.dim !== 'overworld') return;
    var params = {
      seed: S.seed,
      cx: chunkOf(S.view.x),
      cz: chunkOf(S.view.z),
      r: slimeRadius()
    };
    var key = params.cx + ',' + params.cz + ',' + params.r;
    if (!force && S.slimeData && S.slimeData.key === key) return;
    if (S.slimeAbort) { try { S.slimeAbort.abort(); } catch (e) {} }
    var ctrl = new AbortController();
    S.slimeAbort = ctrl;
    getJSON('/slime', params, ctrl).then(function (data) {
      if (!S || ctrl.signal.aborted) return;
      S.slimeData = {
        key: key, chunks: data.chunks || [],
        cx: params.cx, cz: params.cz, r: params.r
      };
      renderSlimes();
    }).catch(function (err) {
      if (!S || ctrl.signal.aborted || (err && err.message === '已取消')) return;
      console.warn('[种子地图] 史莱姆区块读取失败', err);
      toast('史莱姆区块没取到：' + (err.message || '未知原因'), 'e');
      setAlert('史莱姆区块没取到：' + (err.message || '未知原因'), function () { loadSlimes(true); });
    });
  }

  function renderSlimes() {
    var box = $('sm2-slimes');
    if (!box || !S) return;
    box.innerHTML = '';
    S.slimeItems = [];
    S.slimeOrigin = null;
    if (!S.slime || !S.slimeData || S.dim !== 'overworld') return;
    if (!S.layout) return;
    var sc = S.view.scale, ax = S.layout.ax, az = S.layout.az;
    var vw = S.size.w, vh = S.size.h;
    var tx = vw / 2 - (S.view.x - ax) * sc;
    var ty = vh / 2 - (S.view.z - az) * sc;
    var size = 16 * sc;
    if (size < 1.2) return;
    var arr = S.slimeData.chunks;
    var frag = document.createDocumentFragment();
    for (var i = 0; i < arr.length && S.slimeItems.length < MAX_SLIMES; i++) {
      var c = arr[i];
      if (!c || !isNum(c.x) || !isNum(c.z)) continue;
      var bx = c.x * 16, bz = c.z * 16;
      var lx = (bx - ax) * sc, lz = (bz - az) * sc;
      if (lx + tx < -size || lx + tx > vw + size || lz + ty < -size || lz + ty > vh + size) continue;
      var d = document.createElement('div');
      d.className = 'sm2-slime';
      d.style.left = lx + 'px';
      d.style.top = lz + 'px';
      d.style.width = size + 'px';
      d.style.height = size + 'px';
      frag.appendChild(d);
      S.slimeItems.push(d);
    }
    box.appendChild(frag);
    S.slimeOrigin = { ax: ax, az: az, sc: sc };
    applySlimeTransform();
  }

  function applySlimeTransform() {
    if (!S || !S.slimesEl) return;
    var o = S.slimeOrigin;
    if (!o) { S.slimesEl.style.transform = 'translate3d(0,0,0)'; return; }
    var sc = S.view.scale;
    var tx = S.size.w / 2 - (S.view.x - o.ax) * sc;
    var ty = S.size.h / 2 - (S.view.z - o.az) * sc;
    S.slimesEl.style.transform = 'translate3d(' + tx + 'px,' + ty + 'px,0) scale(' + (sc / o.sc) + ')';
  }

  /* ── 要塞 ───────────────────────────────────────────────────────── */

  function loadStrongholds(force) {
    if (!S || !S.seed) return;
    if (!force && S.strongholds) { renderStrongholds(); return; }
    if (S.shAbort) { try { S.shAbort.abort(); } catch (e) {} }
    var ctrl = new AbortController();
    S.shAbort = ctrl;
    S.shLoading = true;
    var note = $('sm2-shnote');
    if (note) note.textContent = '读取中…';
    renderStrongholds();
    getJSON('/stronghold', { seed: S.seed, version: S.version, edition: S.edition }, ctrl)
      .then(function (data) {
        if (!S || ctrl.signal.aborted) return;
        S.shLoading = false;
        S.strongholds = (data.strongholds || []).filter(function (p) { return isNum(p.x) && isNum(p.z); });
        renderStrongholds();
      }).catch(function (err) {
        if (!S || ctrl.signal.aborted || (err && err.message === '已取消')) return;
        S.shLoading = false;
        console.warn('[种子地图] 要塞读取失败', err);
        if (note) note.textContent = '没取到';
        var list = $('sm2-shlist');
        if (list) list.innerHTML = '<p class="sm2-emptyline">要塞没取到：' + esc(err.message || '未知原因') + '</p>';
      });
  }

  function renderStrongholds() {
    if (!S) return;
    var list = $('sm2-shlist');
    var note = $('sm2-shnote');
    if (!list) return;
    var arr = S.strongholds || [];
    if (note) note.textContent = arr.length ? (arr.length + ' 座') : (S.shLoading ? '读取中…' : '');
    if (!arr.length) {
      list.innerHTML = '<p class="sm2-emptyline">' +
        (S.shLoading ? '正在读取要塞坐标…' : '这个种子没有返回要塞坐标。') + '</p>';
      return;
    }
    list.innerHTML = arr.map(function (p, i) {
      var bx = blockOf(p.x), bz = blockOf(p.z);
      var dist = Math.hypot(bx - S.view.x, bz - S.view.z);
      return '<button class="sm2-row" type="button" data-jump="' + bx + ',' + bz + '">' +
        '<b>第 ' + (i + 1) + ' 座</b> X ' + fmtInt(bx) + ' Z ' + fmtInt(bz) + ' ' +
        '<span>距离 ' + fmtBlocks(dist) + '</span></button>';
    }).join('');
  }

  /* ── 侧栏 ───────────────────────────────────────────────────────── */

  function renderSidebar() {
    renderSummary();
    renderList();
    renderStrongholds();
  }

  function renderSummary() {
    var sum = $('sm2-sum');
    var total = $('sm2-total');
    var line = $('sm2-summary');
    if (!sum || !total || !line) return;
    if (!selectedIds().length) {
      sum.innerHTML = '';
      total.textContent = '';
      line.textContent = '还没选结构。展开上面的「结构图层」勾几项。';
      return;
    }
    if (!S.data) {
      sum.innerHTML = '';
      total.textContent = '';
      line.textContent = S.loadingStruct ? '正在统计当前视野的结构…' : '';
      return;
    }
    var ids = selectedIds();
    var counts = S.data.counts || {};
    sum.innerHTML = ids.map(function (id) {
      var def = S.defs[id] || {};
      var n = counts[id] != null ? counts[id] : (S.data.points[id] ? S.data.points[id].length : 0);
      return '<span><i style="--c:' + esc(def.color || '#888') + '"></i>' +
        esc(def.name || id) + ' <b>' + fmtInt(n) + '</b></span>';
    }).join('');
    total.textContent = '共 ' + fmtInt(S.data.total) + ' 个';
    line.textContent = S.data.truncated
      ? '当前范围结果被截断，还有更多，缩小范围再看。'
      : '范围 ' + S.data.r + ' 区块，点击列表里的坐标可直接跳过去。';
  }

  function renderList() {
    var box = $('sm2-list');
    if (!box) return;
    if (!selectedIds().length) {
      box.innerHTML = '<p class="sm2-emptyline">没有勾选任何结构。</p>';
      renderListNote('');
      return;
    }
    if (!S.data) {
      box.innerHTML = '<p class="sm2-emptyline">' + (S.loadingStruct ? '正在读取结构坐标…' : '还没有结构数据。') + '</p>';
      return;
    }
    var ids = selectedIds().filter(function (id) {
      return S.data.points[id] && S.data.points[id].length;
    });
    if (!ids.length) {
      box.innerHTML = '<p class="sm2-emptyline">这个范围里没找到选中的结构。把地图缩小一点，或换个种子试试。</p>';
      renderListNote('');
      return;
    }
    var html = '';
    ids.forEach(function (id) {
      var def = S.defs[id] || {};
      var arr = S.data.points[id].slice();
      var cx = S.view.x, cz = S.view.z;
      arr.forEach(function (p) {
        p._d = Math.hypot(p.bx - cx, p.bz - cz);
      });
      arr.sort(function (a, b) { return a._d - b._d; });
      var closed = !!S.closed[id];
      var limit = S.expanded[id] ? 40 : 8;
      html += '<div class="sm2-group' + (closed ? ' closed' : '') + '" data-group="' + esc(id) + '">' +
        '<button class="sm2-group-head" type="button"><i style="--c:' + esc(def.color || '#888') + '"></i>' +
        esc(def.name || id) + '<em>' + arr.length + ' 个</em>' +
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg></button>' +
        '<div class="sm2-rows">' +
        arr.slice(0, limit).map(function (p) {
          return '<button class="sm2-row" type="button" data-jump="' + p.bx + ',' + p.bz + '">' +
            'X ' + fmtInt(p.bx) + ' Z ' + fmtInt(p.bz) + ' ' +
            '<span>距离 ' + fmtBlocks(p._d) + '</span></button>';
        }).join('') +
        (arr.length > limit ? '<button class="sm2-row more" type="button" data-more="' + esc(id) + '">还有 ' + (arr.length - limit) + ' 个</button>' : '') +
        '</div></div>';
    });
    box.innerHTML = html;
    renderListNote(S.data.truncated ? '结果已截断' : (S.data.total + ' 个'));
  }

  /* ── 信息面板 ───────────────────────────────────────────────────── */

  function showInfo(pt) {
    if (!S) return;
    var info = $('sm2-info');
    var body = $('sm2-info-body');
    var title = $('sm2-info-title');
    if (!info || !body) return;
    var cx = chunkOf(pt.x), cz = chunkOf(pt.z);
    var nearest = nearestStructure(pt.x, pt.z);
    S.info = { x: pt.x, z: pt.z, id: pt.id || '', nearest: nearest };
    if (title) title.textContent = pt.id && S.defs[pt.id] ? S.defs[pt.id].name : '坐标详情';
    body.innerHTML = '' +
      '<div class="sm2-kv"><span>方块坐标</span><b>X ' + fmtInt(pt.x) + ' · Z ' + fmtInt(pt.z) + '</b></div>' +
      '<div class="sm2-kv"><span>区块坐标</span><b>' + fmtInt(cx) + ', ' + fmtInt(cz) + '</b></div>' +
      '<div class="sm2-kv"><span>生物群系</span><b id="sm2-info-biome">查询中…</b></div>' +
      '<div class="sm2-kv"><span>最近结构</span><b id="sm2-info-near">' +
        (nearest ? (esc(nearest.name) + ' · ' + fmtBlocks(nearest.dist))
          : (S.loadingStruct ? '统计中…' : '附近暂无')) + '</b></div>';
    info.hidden = false;
  }

  function hideInfo() {
    var info = $('sm2-info');
    if (info) info.hidden = true;
    S && (S.info = null);
  }

  function nearestStructure(bx, bz) {
    if (!S || !S.data) return null;
    var best = null;
    Object.keys(S.data.points).forEach(function (id) {
      var arr = S.data.points[id];
      for (var i = 0; i < arr.length; i++) {
        var p = arr[i];
        var d = Math.hypot(p.bx - bx, p.bz - bz);
        if (!best || d < best.dist) {
          best = { id: id, name: (S.defs[id] || {}).name || NAME_FALLBACK[id] || '结构', bx: p.bx, bz: p.bz, dist: d };
        }
      }
    });
    return best;
  }

  function loadBiome(bx, bz) {
    if (!S || !S.seed) return;
    var token = ++S.biomeToken;
    var ctrl = new AbortController();
    var chunkX = Math.floor(bx / 16), chunkZ = Math.floor(bz / 16);
    getJSON('/biome', {
      seed: S.seed, version: S.version, edition: S.edition, dim: S.dim,
      x: chunkX, z: chunkZ
    }, ctrl).then(function (data) {
      if (!S || token !== S.biomeToken) return;
      var p = data.points && data.points[0];
      var el = $('sm2-info-biome');
      if (!el) return;
      if (p && p.name) {
        el.textContent = p.name;
        if (p.color) el.style.color = p.color;
      } else {
        el.textContent = '未知群系';
      }
    }).catch(function (err) {
      if (!S || token !== S.biomeToken) return;
      var el = $('sm2-info-biome');
      if (el) el.textContent = '没取到';
      console.warn('[种子地图] 群系读取失败', err && err.message);
    });
  }

  function jumpTo(bx, bz, zoomIn) {
    if (!S) return;
    S.view.x = bx;
    S.view.z = bz;
    if (zoomIn && S.view.scale < 2) S.view.scale = 2;
    S.sel = { x: bx, z: bz };
    placePin();
    viewChanged(true);
    relayout(true);
    maybeReloadData();
  }

  /* ── 加载 / 空态 / 提示条 ───────────────────────────────────────── */

  function setLoading(on, text) {
    var box = $('sm2-loading');
    if (!box) return;
    box.hidden = !on;
    if (on && text) {
      var t = $('sm2-loadingtext');
      if (t) t.textContent = text;
    }
  }

  function updateLoading() {
    if (!S) return;
    var box = $('sm2-loading');
    if (!box) return;
    if (S.engineDown || S.tileOK) { box.hidden = true; return; }
    box.hidden = S.pending.size === 0;
    if (!box.hidden) {
      var t = $('sm2-loadingtext');
      if (t) t.textContent = '正在画地图…';
    }
  }

  function setEmpty(title, desc, btnLabel, fn) {
    var box = $('sm2-empty');
    if (!box) return;
    $('sm2-empty-title').textContent = title || '';
    $('sm2-empty-desc').textContent = desc || '';
    var btn = $('sm2-empty-btn');
    if (btn) {
      if (btnLabel && fn) {
        btn.hidden = false;
        btn.textContent = btnLabel;
        btn.onclick = function () { fn(); };
      } else {
        btn.hidden = true;
        btn.onclick = null;
      }
    }
    box.hidden = false;
  }

  function hideEmpty() {
    var box = $('sm2-empty');
    if (box) box.hidden = true;
  }

  function setAlert(msg, retry) {
    var box = $('sm2-alert');
    if (!box) return;
    if (!msg) { box.hidden = true; return; }
    var text = $('sm2-alert-text');
    if (text) text.textContent = msg;
    var btn = $('sm2-alert-btn');
    if (btn) {
      btn.hidden = !retry;
      btn.onclick = retry ? function () { box.hidden = true; retry(); } : null;
    }
    box.hidden = false;
  }

  /* ── 事件绑定 ───────────────────────────────────────────────────── */

  function bindControls() {
    var seed = $('sm2-seed');
    if (seed) {
      seed.addEventListener('input', syncSeedField);
      seed.addEventListener('paste', function () {
        setTimeout(syncSeedField, 0);
      });
      seed.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); applySeedInput(seed.value); }
      });
      seed.addEventListener('change', function () {
        if (seed.value.trim()) applySeedInput(seed.value, true);
      });
    }

    var clear = $('sm2-clear');
    if (clear) clear.addEventListener('click', function () {
      if (seed) seed.value = '';
      applySeedInput('');
      if (seed) seed.focus();
    });

    var go = $('sm2-go');
    if (go) go.addEventListener('click', function () {
      if (!S || !S.started) return;
      applySeedInput(seed ? seed.value : '');
    });

    var ver = $('sm2-version');
    if (ver) ver.addEventListener('change', function () {
      var parts = String(ver.value).split('|');
      if (!S || parts.length < 2) return;
      S.edition = parts[0];
      S.version = parts[1];
      renderChips();
      readMeta();
      onQueryChanged();
    });

    var dims = $('sm2-dims');
    if (dims) dims.addEventListener('click', function (e) {
      var b = closest(e.target, 'button[data-dim]');
      if (!b || !S) return;
      var dim = b.getAttribute('data-dim');
      if (dim === S.dim) return;
      S.dim = dim;
      S.selected = defaultSelection(dim);
      S.closed = {};
      S.slimeData = null;
      renderDims();
      renderChips();
      renderSlimes();
      readMeta();
      onQueryChanged();
    });

    var toggle = $('sm2-layertoggle');
    if (toggle) {
      toggle.addEventListener('click', function () {
        var body = $('sm2-layerbody');
        var open = toggle.getAttribute('aria-expanded') === 'true';
        toggle.setAttribute('aria-expanded', open ? 'false' : 'true');
        if (body) body.hidden = open;
      });
    }

    var chips = $('sm2-chips');
    if (chips) chips.addEventListener('click', function (e) {
      var b = closest(e.target, '.sm2-chip');
      if (!b || !S) return;
      var id = b.getAttribute('data-id');
      S.selected[id] = !S.selected[id];
      renderChips();
      refreshLayers();
    });

    var acts = $('sm2-layerbody');
    if (acts) acts.addEventListener('click', function (e) {
      var b = closest(e.target, 'button[data-act]');
      if (!b || !S) return;
      var act = b.getAttribute('data-act');
      if (act === 'all') {
        structuresForDim(S.dim).forEach(function (d) { S.selected[d.id] = true; });
      } else if (act === 'none') {
        S.selected = {};
      } else if (act === 'default') {
        S.selected = defaultSelection(S.dim);
      } else return;
      renderChips();
      refreshLayers();
    });

    var slime = $('sm2-slime');
    if (slime) slime.addEventListener('change', function () {
      if (!S) return;
      S.slime = !!slime.checked;
      var wrap = $('sm2-slimewrap');
      if (wrap) wrap.classList.toggle('on', S.slime);
      if (S.slime) loadSlimes(true);
      else { S.slimeData = null; renderSlimes(); }
    });

    var dock = document.querySelector('.sm2-dock');
    if (dock) dock.addEventListener('click', function (e) {
      var b = closest(e.target, 'button[data-act]');
      if (!b || !S) return;
      var act = b.getAttribute('data-act');
      if (act === 'in') zoomAt(S.size.w / 2, S.size.h / 2, 1.7);
      else if (act === 'out') zoomAt(S.size.w / 2, S.size.h / 2, 1 / 1.7);
      else if (act === 'home') { S.view.x = 0; S.view.z = 0; viewChanged(false); relayout(true); }
    });

    var slider = $('sm2-zoom');
    if (slider) slider.addEventListener('input', function () {
      if (!S) return;
      setScale(sliderToScale(Number(slider.value)));
    });

    var list = $('sm2-list');
    if (list) list.addEventListener('click', function (e) {
      var more = closest(e.target, 'button[data-more]');
      if (more && S) {
        var g = more.getAttribute('data-more');
        S.expanded[g] = true;
        renderList();
        return;
      }
      var head = closest(e.target, '.sm2-group-head');
      if (head && S) {
        var group = head.parentNode.getAttribute('data-group');
        S.closed[group] = !S.closed[group];
        head.parentNode.classList.toggle('closed', !!S.closed[group]);
        return;
      }
      var row = closest(e.target, 'button[data-jump]');
      if (!row || !S) return;
      var xy = String(row.getAttribute('data-jump')).split(',');
      var bx = Number(xy[0]), bz = Number(xy[1]);
      if (!isFinite(bx) || !isFinite(bz)) return;
      jumpTo(bx, bz, true);
      showInfo({ x: bx, z: bz });
      if (S.strongholds) {
        for (var i = 0; i < S.strongholds.length; i++) {
          if (blockOf(S.strongholds[i].x) === bx && blockOf(S.strongholds[i].z) === bz) {
            showInfo({ x: bx, z: bz, id: 'stronghold' });
            break;
          }
        }
      }
      loadBiome(bx, bz);
      closeDrawer();
    });

    var sh = $('sm2-shlist');
    if (sh) sh.addEventListener('click', function (e) {
      var row = closest(e.target, 'button[data-jump]');
      if (!row || !S) return;
      var xy = String(row.getAttribute('data-jump')).split(',');
      jumpTo(Number(xy[0]), Number(xy[1]), true);
      showInfo({ x: Number(xy[0]), z: Number(xy[1]), id: 'stronghold' });
      closeDrawer();
    });

    var shbtn = $('sm2-shbtn');
    if (shbtn) shbtn.addEventListener('click', function () { loadStrongholds(true); });

    var info = $('sm2-info');
    if (info) info.addEventListener('click', function (e) {
      var b = closest(e.target, 'button[data-act]');
      if (!b || !S) return;
      var act = b.getAttribute('data-act');
      if (act === 'close') hideInfo();
      else if (act === 'center' && S.info) {
        jumpTo(S.info.x, S.info.z, false);
      } else if (act === 'copy' && S.info) {
        copyText('方块坐标 X ' + fmtInt(S.info.x) + ' Z ' + fmtInt(S.info.z) +
          '（区块 ' + chunkOf(S.info.x) + ', ' + chunkOf(S.info.z) + '）', '坐标已复制');
      }
    });

    var drawer = $('sm2-drawer');
    if (drawer) drawer.addEventListener('click', function () { openDrawer(); });
    var scrim = $('sm2-scrim');
    if (scrim) scrim.addEventListener('click', function () { closeDrawer(); });
    var closeBtn = $('sm2-closedrawer');
    if (closeBtn) closeBtn.addEventListener('click', function () { closeDrawer(); });
  }

  function openDrawer() {
    var root = $('sm2');
    if (root) root.classList.add('drawer');
  }

  function closeDrawer() {
    var root = $('sm2');
    if (root) root.classList.remove('drawer');
  }

  function readMeta() {
    if (!S || !S.started) return;
    getJSON('/meta', { version: S.version, edition: S.edition }).then(function (m) {
      if (!S) return;
      var unsupported = Array.isArray(m.unsupported) ? m.unsupported : [];
      if (!unsupported.length) return;
      // 该版本没有的结构从选中里去掉，并在 chips 上标灰
      unsupported.forEach(function (id) {
        delete S.selected[id];
        var chip = document.querySelector('.sm2-chip[data-id="' + id + '"]');
        if (chip) {
          chip.classList.remove('on');
          chip.setAttribute('aria-pressed', 'false');
          chip.disabled = true;
          chip.title = '这个版本没有该结构';
        }
      });
      renderDots();
    }).catch(function (err) {
      console.warn('[种子地图] 版本能力清单没取到', err && err.message);
    });
  }

  /* ── 尺寸与清理 ─────────────────────────────────────────────────── */

  function bindResize() {
    var map = S.mapEl;
    var onResize = function () {
      if (!S) return;
      if (!measure()) return;
      if (S.size.w > 1023) closeDrawer();
      relayout(true);
      applyView();
      renderIcons();
      renderSlimes();
    };
    var ro = null;
    if (window.ResizeObserver) {
      ro = new ResizeObserver(function () { onResize(); });
      ro.observe(map);
      cleanups.push(function () { try { ro.disconnect(); } catch (e) {} });
    }
    window.addEventListener('resize', onResize);
    cleanups.push(function () { window.removeEventListener('resize', onResize); });
  }

  function teardown() {
    if (rafId) { cancelAnimationFrame(rafId); rafId = 0; }
    if (S) {
      Object.keys(S.timers).forEach(function (k) { clearTimeout(S.timers[k]); });
      if (S.structAbort) { try { S.structAbort.abort(); } catch (e) {} }
      if (S.slimeAbort) { try { S.slimeAbort.abort(); } catch (e) {} }
      if (S.shAbort) { try { S.shAbort.abort(); } catch (e) {} }
      releaseTiles();
    }
    cleanups.forEach(function (fn) { try { fn(); } catch (e) {} });
    cleanups = [];
    S = null;
  }

  /* ── 入口 ───────────────────────────────────────────────────────── */

  function yySeedMapPage() {
    ensureCss().then(function () {
      S = newState();
      delete subPageCache[PAGE_TITLE];
      openSubPage(PAGE_TITLE, pageHtml());

      S.mapEl = $('sm2-map');
      S.planeEl = $('sm2-plane');
      S.iconsEl = $('sm2-icons');
      S.slimesEl = $('sm2-slimes');

      bindControls();
      bindMap();
      bindResize();
      syncSeedField();
      // 手机上工具条本来就窄，结构 chips 先收起来，点「结构图层」再展开
      if (window.innerWidth <= 640) {
        var lt = $('sm2-layertoggle'), lb = $('sm2-layerbody');
        if (lt) lt.setAttribute('aria-expanded', 'false');
        if (lb) lb.hidden = true;
      }
      setLoading(true, '正在读取版本清单…');
      updateReadout();

      if (typeof activeRenderers !== 'undefined' && Array.isArray(activeRenderers)) {
        activeRenderers.push(teardown);
      }

      afterLayout();
      bootManifest();
    });
    return Promise.resolve();
  }

  window.yySeedMapPage = yySeedMapPage;

  /* ── 导航入口（和「种子查询」同一个分类，各自一块）───────────────── */

  if (typeof yyCategories === 'object' && yyCategories && !yyCategories.seed) {
    yyCategories.seed = '种子与地图';
  }
  // 旧页面（id='seed'，youyuan_seed_page.js）走的是 /api/seed/* 那套近似实现。
  // 玩家不该看到两个「种子」入口，所以：有旧入口就把它的 open 指到新页，
  // 没有再补一个 seedmap2。旧文件不删，万一新页挂了还能手动切回去。
  var legacy = null;
  if (typeof yyModules !== 'undefined' && Array.isArray(yyModules)) {
    for (var i = 0; i < yyModules.length; i++) {
      if (yyModules[i] && yyModules[i].id === 'seed') { legacy = yyModules[i]; break; }
    }
  }
  if (legacy) {
    legacy.name = '种子地图';
    legacy.desc = '拖动缩放看地形与结构，点一下读坐标';
    legacy.icon = legacy.icon || 'map';
    legacy.open = function () { return yySeedMapPage(); };
  } else if (typeof yyModules !== 'undefined' && Array.isArray(yyModules) &&
      !yyModules.some(function (m) { return m.id === 'seedmap2'; })) {
    yyModules.push({
      id: 'seedmap2',
      name: '种子地图',
      desc: '拖动缩放看地形与结构，点一下读坐标',
      category: 'seed',
      icon: 'map',
      open: function () { return yySeedMapPage(); }
    });
    if (typeof yyAllModules !== 'undefined' && yyAllModules !== yyModules &&
        !yyAllModules.some(function (m) { return m.id === 'seedmap2'; })) {
      yyAllModules.push(yyModules[yyModules.length - 1]);
    }
    if (typeof yyPrefs === 'object' && yyPrefs && Array.isArray(yyPrefs.order) && yyPrefs.order.indexOf('seedmap2') < 0) {
      yyPrefs.order.push('seedmap2');
      if (typeof yySavePrefs === 'function') yySavePrefs();
    }
    if (typeof yyDrawTools === 'function' && typeof curTab !== 'undefined' && curTab === 'home' &&
        typeof subPageOpen !== 'undefined' && !subPageOpen) {
      try { yyDrawTools(); } catch (e) {}
    }
  }
})();
