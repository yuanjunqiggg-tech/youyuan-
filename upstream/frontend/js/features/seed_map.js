/* ═══════════════════════════════════════════════════════════════════
 * 种子查询器 — 群系地图 · 结构定位 · 全维度（Canvas 本机渲染）
 *
 * 入口：「更多工具」→ 分类「建筑与世界」→「种子查询器」。
 * 与已有的「种子查询」（youyuan_seed_page.js，纯本机算法）互补：
 * 这里走 Go 侧同期新增的三个只读接口，前端按“预期契约”调用，
 * 任何字段缺失 / 接口 404 都优雅降级，绝不渲染假地图。
 *
 * 预期契约（与 Go 子智能体同步声明，字段缺失时逐项降级）：
 *   GET /api/seed/biome?seed=&dimension=overworld|nether|the_end&edition=bedrock
 *        &version=&x=&z=            → {ok:true, biome:"plains", biome_id:N,
 *                                       name:"平原", dimension:"...", edition:"...",
 *                                       precision:"exact"|"approx"|"unsupported"}
 *        批量形态（优先探测）：再带 from_x/from_z/to_x/to_z/lod →
 *                                      {ok:true, points:[{x,z,biome,...}, ...]}
 *   GET /api/seed/manifest          → 版本区间矩阵，兼容解析
 *                                      {versions:[...]} / {editions:{java:{versions:[...]}}}
 *                                      / {matrix:[...]} 等形状；解析不出用内置默认列表并标「默认」
 *   GET /api/seed/structure?seed=&edition=java&version=&dimension=&structure=
 *        &from_x=&from_z=&to_x=&to_z= → {ok:true, structures:[{type,label,x,z,precision}, ...]}
 *        回退（skills/world-seed/index.md 已文档化的接口）：
 *   GET /api/youyuan/world/seed?seed=&structure=<t>&edition=java&from_x=<区块>...
 *        → {ok:true, java_preview:{label, chunks:[[cx,cz],...], precision:"candidate", ...}}
 *   GET /api/youyuan/world/seed     → 预填当前世界种子（seed_text 优先，防 2^53 丢精度）
 *
 * 性能与配额纪律：
 *   - 取点按网格合并：批量模式一次请求拿一整片区域；仅单点模式时逐格请求，
 *     每格独立缓存（含取样步长），拖动停止后只补拉新增可视格；
 *   - 同屏最多 2 个并发请求，队列带优先级（群系 0，结构 1）；
 *   - visibilitychange hidden 暂停取点并 abort 在途请求，回到前台续跑；
 *   - 缓存键 = 步长+格子坐标，已取过的格子不重复请求。
 * ═══════════════════════════════════════════════════════════════════ */

var YSM_TITLE = '种子查询器';

/* ── 群系标准色板 ────────────────────────────────────────────────────
 * 取色沿用 Minecraft 地图配色惯例；先整名精确匹配（YSM_BIOME_EXACT），
 * 再按关键词模糊匹配（YSM_BIOME_MATCH，从上到下首个命中生效）。
 * 都未命中返回中性灰 #8a949c —— 不认识的群系不猜颜色。
 * [关键词, 颜色, 中文图例名] */
var YSM_BIOME_EXACT = {
  plains: ['#91bd59', '平原'],
  desert: ['#e0c860', '沙漠'],
  jungle: ['#2f8f1f', '丛林'],
  swamp: ['#51703d', '沼泽'],
  beach: ['#e7dba0', '沙滩'],
  river: ['#2a56c6', '河流'],
  savanna: ['#bfb755', '稀树草原'],
  badlands: ['#d97718', '恶地'],
  mushroom_fields: ['#a945a9', '蘑菇岛'],
  lush_caves: ['#4f7a35', '繁茂洞穴'],
  dripstone_caves: ['#8a7a66', '溶洞'],
  deep_dark: ['#1b2430', '深暗之域'],
  the_void: ['#0b0b12', '虚空'],
  nether_wastes: ['#7a2f22', '下界荒地'],
  crimson_forest: ['#9e2b2b', '绯红森林'],
  warped_forest: ['#2b7a8f', '诡异森林'],
  soul_sand_valley: ['#5a4436', '灵魂沙峡谷'],
  basalt_deltas: ['#484848', '玄武岩三角洲'],
  the_end: ['#e5e08a', '末地'],
  end_highlands: ['#cfcf9a', '末地高地'],
  end_midlands: ['#d6d692', '末地中部'],
  small_end_islands: ['#dede9c', '末地浮岛'],
  end_barrens: ['#c9c97f', '末地荒地']
};
/* 关键词表（子串匹配，小写比对） */
var YSM_BIOME_MATCH = [
  ['deep_frozen_ocean', '#4a5a8c', '深度冻结海洋'],
  ['frozen_ocean', '#7a8fa8', '冻结海洋'],
  ['deep_ocean', '#12306e', '深海'],        // 海洋深蓝
  ['deep_lukewarm_ocean', '#1a4a9e', '深海'],
  ['deep_cold_ocean', '#16376e', '深海'],
  ['ocean', '#16418c', '海洋'],
  ['lukewarm_ocean', '#2266b0', '海洋'],
  ['warm_ocean', '#2e7ec6', '海洋'],
  ['snowy_plains', '#f0f6f6', '雪原'],
  ['snowy', '#f0f6f6', '雪原'],             // 雪原白
  ['ice_spikes', '#dce8ec', '冰刺'],
  ['frozen', '#cfe0e8', '冻原'],
  ['snowy_beach', '#e8e4d0', '雪滩'],
  ['stony_shore', '#8f8f85', '石岸'],
  ['birch', '#9fb85a', '白桦林'],
  ['dark_forest', '#2f5b1e', '黑森林'],     // 深绿
  ['flower_forest', '#79c05a', '繁花森林'],
  ['forest', '#59a52c', '森林'],
  ['old_growth', '#3f7a26', '原始森林'],
  ['taiga', '#4a7a4a', '针叶林'],
  ['windswept_hills', '#8f8f8f', '风袭丘陵'],
  ['mountain', '#8f8f8f', '山地'],
  ['extreme_hills', '#8f8f8f', '高山'],
  ['jungle', '#2f8f1f', '丛林'],            // 丛林深绿
  ['bamboo', '#59a52c', '竹林'],
  ['savanna', '#bfb755', '稀树草原'],
  ['shattered', '#9f9f60', '破碎草原'],
  ['badlands', '#d97718', '恶地'],          // 恶地橙
  ['mesa', '#d97718', '恶地'],
  ['eroded', '#c96b12', '风蚀恶地'],
  ['wooded_badlands', '#b07a30', '疏林恶地'],
  ['swamp', '#51703d', '沼泽'],
  ['mangrove', '#5a7a3a', '红树林'],
  ['mushroom', '#a945a9', '蘑菇岛'],
  ['plains', '#91bd59', '平原'],            // 平原草绿
  ['sunflower', '#91bd59', '平原'],
  ['meadow', '#8fc97a', '草甸'],
  ['grove', '#e0e8e0', '雪林'],
  ['stony_peaks', '#9a9a8f', '石峰'],
  ['jagged_peaks', '#d8d8d8', '尖峭山峰'],
  ['frozen_peaks', '#e8f0f0', '冰封山峰'],
  ['beach', '#e7dba0', '沙滩'],
  ['river', '#2a56c6', '河流'],
  ['desert', '#e0c860', '沙漠'],            // 沙漠黄
  ['crimson', '#9e2b2b', '绯红森林'],       // 下界红
  ['warped', '#2b7a8f', '诡异森林'],        // 下界紫蓝
  ['soul_sand', '#5a4436', '灵魂沙峡谷'],
  ['soul_valley', '#5a4436', '灵魂沙峡谷'],
  ['basalt', '#484848', '玄武岩三角洲'],
  ['nether', '#7a2f22', '下界'],            // 下界红
  ['end', '#e5e08a', '末地'],               // 末地黄
  ['void', '#0b0b12', '虚空'],              // 虚空黑
  ['deltas', '#484848', '玄武岩三角洲']
];

/* 结构元数据（图钉色 + 中文名 + 所属维度 + 放置类型）
 *
 * ⚠ ID 必须与后端 upstream/youyuan_seed_structures.go 的注册表完全一致：
 *   后端加了结构而这里没加 → 图钉画不出来（Go 侧有测试会校验两边对齐）。
 *   prob 字段表示「按区块概率生成」，地图给不出确定坐标，只做图例说明。
 */
var YSM_STRUCT_META = {
  /* 主世界 · 区域网格 */
  village:          { label: '村庄',       color: '#2f9e5f', dim: 'overworld' },
  desert_pyramid:   { label: '沙漠神殿',   color: '#d4b24a', dim: 'overworld' },
  jungle_pyramid:   { label: '丛林神庙',   color: '#3f8f4a', dim: 'overworld' },
  swamp_hut:        { label: '女巫小屋',   color: '#6b8f3a', dim: 'overworld' },
  igloo:            { label: '雪屋',       color: '#a8cfd8', dim: 'overworld' },
  ocean_ruin:       { label: '海底废墟',   color: '#4f8fa8', dim: 'overworld' },
  shipwreck:        { label: '沉船',       color: '#8a6a45', dim: 'overworld' },
  monument:         { label: '海底神殿',   color: '#2f6f9e', dim: 'overworld' },
  mansion:          { label: '林地府邸',   color: '#7a4b9e', dim: 'overworld' },
  pillager_outpost: { label: '掠夺者前哨站', color: '#b34a4a', dim: 'overworld' },
  ruined_portal:    { label: '废弃传送门', color: '#8f6fb8', dim: 'overworld' },
  ancient_city:     { label: '远古城市',   color: '#2b3f5c', dim: 'overworld' },
  trail_ruins:      { label: '古迹废墟',   color: '#b09a6a', dim: 'overworld' },
  trial_chambers:   { label: '试炼密室',   color: '#c07a3a', dim: 'overworld' },
  /* 主世界 · 特殊放置 */
  stronghold:       { label: '要塞',       color: '#4a8fb8', dim: 'overworld' },
  mineshaft:        { label: '废弃矿井',   color: '#8a7a5a', dim: 'overworld', prob: 0.004 },
  buried_treasure:  { label: '埋藏的宝藏', color: '#c9a227', dim: 'overworld', prob: 0.01 },
  /* 下界 */
  fortress:         { label: '下界要塞',   color: '#c04a33', dim: 'nether' },
  bastion:          { label: '堡垒遗迹',   color: '#a06a2c', dim: 'nether' },
  nether_fossil:    { label: '下界化石',   color: '#7a5c46', dim: 'nether' },
  /* 末地 */
  end_city:         { label: '末地城',     color: '#d8c747', dim: 'the_end' }
};

/* 按维度分组：从 META 派生，避免两张表各写一遍然后漂移 */
var YSM_STRUCT_BY_DIM = { overworld: [], nether: [], the_end: [] };
var YSM_STRUCT_PROB = [];
Object.keys(YSM_STRUCT_META).forEach(function (id) {
  var m = YSM_STRUCT_META[id];
  if (m.prob) { YSM_STRUCT_PROB.push(id); return; }
  if (!YSM_STRUCT_BY_DIM[m.dim]) YSM_STRUCT_BY_DIM[m.dim] = [];
  YSM_STRUCT_BY_DIM[m.dim].push(id);
});

var YSM_DIM_LABEL = { overworld: '主世界', nether: '下界', the_end: '末地' };

/* 版本默认列表：manifest 接口失败时兜底，页面会标注「默认」 */
var YSM_DEFAULT_VERSIONS = ['1.21', '1.20', '1.19', '1.18', '1.17', '1.16', '1.12'];

/* ── 模块级状态 ── */
var ysmS = null;              // 页面状态（每次打开重建）
var ysmRaf = 0;               // 重绘帧
var ysmDeferTimer = 0;        // 拖动停止后的补拉定时器
var ysmInFlight = 0;          // 当前并发数
var ysmQueue = [];            // 待请求队列
var ysmQueued = {};           // 去重表：key -> 1
var ysmAbort = null;          // AbortController（隐藏/关页时掐断）
var ysmManifest = null;       // manifest 解析结果缓存（跨次打开复用）
var ysmSeedPre = null;        // 当前世界种子预填缓存 {text, zero, fromKernel}

/* ── 小工具 ── */
function ysmEsc(v) {
  return String(v == null ? '' : v).replace(/[&<>"']/g, function (c) {
    return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
  });
}
function ysmClamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }
function ysmToast(msg, kind) { if (typeof T === 'function') T(msg, kind || 'e'); }
function ysm$(id) { return document.getElementById(id); }
function ysmErrText(e) { return (e && e.message) ? String(e.message) : '网络错误'; }

/* precision 归一：如实展示，绝不冒充精确 */
function ysmPrecLabel(raw) {
  var s = String(raw || '').toLowerCase();
  if (!s) return '';
  if (s.indexOf('exact') >= 0 || s.indexOf('精确') >= 0) return '精确';
  if (s.indexOf('unsupported') >= 0 || s.indexOf('不支持') >= 0) return '不支持';
  if (s.indexOf('approx') >= 0 || s.indexOf('candidate') >= 0 || s.indexOf('preview') >= 0 || s.indexOf('近似') >= 0 || s.indexOf('试算') >= 0) return '近似（试算）';
  return 'precision: ' + raw; // 未知取值原样展示
}

/* 群系取色：返回 [颜色, 中文标签]；只认得的名字才给色 */
function ysmBiomeColor(b) {
  var key = String(b == null ? '' : b).toLowerCase();
  if (!key) return null;
  if (YSM_BIOME_EXACT[key]) return YSM_BIOME_EXACT[key];
  for (var i = 0; i < YSM_BIOME_MATCH.length; i++) {
    if (key.indexOf(YSM_BIOME_MATCH[i][0]) >= 0) return [YSM_BIOME_MATCH[i][1], YSM_BIOME_MATCH[i][2]];
  }
  return null;
}

/* ── 页面骨架 ── */
function RseedMapHTML() {
  return '' +
    '<div class="ysm">' +
      '<section class="ysm-panel">' +
        '<div class="ysm-row">' +
          '<input id="ysm-seed" class="ysm-input" inputmode="numeric" autocomplete="off" spellcheck="false" placeholder="输入种子（十进制整数，可为负）" aria-label="种子">' +
          '<button type="button" id="ysm-read" class="btn-s ysm-readbtn">读当前种子</button>' +
        '</div>' +
        '<div class="ysm-row ysm-row-ver">' +
          '<select id="ysm-version" class="ysm-select" aria-label="游戏版本"></select>' +
          '<span id="ysm-verbadge" class="ysm-badge" hidden></span>' +
          '<div class="ysm-tabs" id="ysm-dims" role="group" aria-label="维度">' +
            '<button type="button" data-dim="overworld" class="on">主世界</button>' +
            '<button type="button" data-dim="nether">下界</button>' +
            '<button type="button" data-dim="the_end">末地</button>' +
          '</div>' +
        '</div>' +
        '<p class="ysm-hint" id="ysm-note">种子只在本机与内网接口之间流转；读不到当前种子就手动输入。</p>' +
      '</section>' +
      '<section class="ysm-mapcard">' +
        '<div class="ysm-mapwrap" id="ysm-wrap">' +
          '<canvas id="ysm-canvas" aria-label="群系地图，可拖动缩放"></canvas>' +
          '<div class="ysm-overlay" id="ysm-overlay" hidden></div>' +
          '<div class="ysm-tools">' +
            '<button type="button" data-act="in" aria-label="放大">＋</button>' +
            '<button type="button" data-act="out" aria-label="缩小">－</button>' +
            '<button type="button" data-act="reset" aria-label="回到原点">复位</button>' +
          '</div>' +
          '<div class="ysm-pincard" id="ysm-pincard" hidden></div>' +
        '</div>' +
        '<div class="ysm-readout" id="ysm-readout">拖动平移 · 双指或按钮缩放 · 点图钉看详情</div>' +
      '</section>' +
      '<section class="ysm-panel">' +
        '<h3 class="ysm-legend-title">图例</h3>' +
        '<div class="ysm-legend" id="ysm-biolegend"><span class="ysm-dim">暂无群系数据</span></div>' +
        '<div class="ysm-legend ysm-slegend" id="ysm-strlegend"></div>' +
      '</section>' +
    '</div>';
}

/* ── 状态 ── */
function ysmNewState() {
  return {
    active: true,
    paused: false,
    seed: '', seedValid: false, seedFromKernel: false, seedZero: false,
    version: YSM_DEFAULT_VERSIONS[1], versions: [], versionsDefault: true,
    dim: 'overworld',
    cx: 0, cz: 0,          // 视口中心（方块坐标，可为小数）
    scale: 0.375,          // 每方块像素（初始：64 格 ≈ 24px）
    cell: 64,              // 当前取样步长（方块）
    cache: {},             // 'cell:gx,gz' -> {b:群系id, name:中文名, prec:原始precision}
    bioMode: null,         // 'batch' | 'point' | 'dead'
    bioFail: 0,
    precSeen: {},          // 出现过的 precision 原始值 -> 1
    pins: [],              // 可见图钉 {type,label,x,z,px,py,prec,trial}
    structBuckets: {},     // 'type|bx,bz' -> 1 已取
    structFailedDims: {},  // 'dim' -> 1 结构接口不可用（记录一次提示）
    pendingBio: 0,
    dragging: false
  };
}

/* 取样步长随缩放自适应：每格落在 18~40px 之间，格数上限约 320 */
var YSM_CELLS = [4, 8, 16, 32, 64, 128, 256, 512, 1024];
function ysmPickCell() {
  for (var i = 0; i < YSM_CELLS.length; i++) {
    if (YSM_CELLS[i] * ysmS.scale >= 18) return YSM_CELLS[i];
  }
  return YSM_CELLS[YSM_CELLS.length - 1];
}

/* ── 请求队列（同屏最多 2 个并发，优先级小者先发） ── */
function ysmPump() {
  if (!ysmS || !ysmS.active || ysmS.paused) return;
  while (ysmInFlight < 2 && ysmQueue.length) {
    var bi = 0;
    for (var i = 1; i < ysmQueue.length; i++) if (ysmQueue[i].prio < ysmQueue[bi].prio) bi = i;
    var job = ysmQueue.splice(bi, 1)[0];
    if (job.stale && job.stale()) { delete ysmQueued[job.key]; continue; }
    ysmRunJob(job);
  }
}
function ysmRunJob(job) {
  ysmInFlight++;
  if (window.AbortController) ysmAbort = new AbortController();
  var opt = ysmAbort ? { signal: ysmAbort.signal } : {};
  fetch(job.url, opt).then(function (r) {
    return r.json().catch(function () { return { ok: false, error: '返回不是 JSON' }; });
  }).then(function (data) {
    if (job.onOk) job.onOk(data);
  }).catch(function (e) {
    if (e && e.name === 'AbortError') { /* 暂停/切页掐断的，不算失败 */ }
    else if (job.onFail) job.onFail(e);
  }).then(function () {
    delete ysmQueued[job.key];
    ysmInFlight--;
    ysmPump();
    ysmSchedule();
  });
}
function ysmEnqueue(key, prio, url, onOk, onFail, stale) {
  if (ysmQueued[key]) return;
  ysmQueued[key] = 1;
  ysmQueue.push({ key: key, prio: prio, url: url, onOk: onOk, onFail: onFail, stale: stale || null });
}
function ysmFlushQueue() {
  ysmQueue = [];
  ysmQueued = {};
  if (ysmAbort) { try { ysmAbort.abort(); } catch (e) {} ysmAbort = null; }
  ysmInFlight = 0;
}

/* ── 接口 URL 拼装 ── */
function ysmCommonParams() {
  return 'seed=' + encodeURIComponent(ysmS.seed) +
    '&version=' + encodeURIComponent(ysmS.version) +
    '&edition=bedrock'; // 内核读的是基岩版语义种子（见 skills/world-seed/index.md）
}

/* 群系接口探测 + 补拉。三种结局：batch / point / dead */
function ysmEnsureBiome() {
  var S = ysmS;
  if (!S || !S.seedValid || S.bioMode === 'dead' || S.paused) return;
  var range = ysmCellRange();
  if (!range) return;
  var cols = Math.round((range.x1 - range.x0) / S.cell);
  var rows = Math.round((range.z1 - range.z0) / S.cell);
  if (cols * rows > 320) return; // 缩得太远时保护配额，放大后再取

  var miss = [];
  for (var gz = range.z0; gz < range.z1; gz += S.cell) {
    for (var gx = range.x0; gx < range.x1; gx += S.cell) {
      if (!S.cache[S.cell + ':' + gx + ',' + gz]) miss.push([gx, gz]);
    }
  }
  if (!miss.length) return;

  if (S.bioMode === 'batch') { ysmEnqueueBatch(miss); return; }
  if (S.bioMode === 'point') { ysmEnqueuePoints(miss.slice(0, 160)); return; }
  if (S.bioMode) return;

  /* 首次探测：先试批量，再试单点；都失败累计 2 次就进 dead（诚实空态） */
  var probeMiss = miss;
  S.bioMode = 'probing';
  var failDead = function (why) {
    S.bioFail++;
    if (S.bioFail >= 2) {
      S.bioMode = 'dead';
      ysmShowOverlay('算法接口不可用（需要更新内核）', '群系算法接口无响应：' + why + '<br>地图不会渲染假数据。');
    } else {
      S.bioMode = null; // 允许下次再试
    }
    ysmSchedule();
  };
  var batchUrl = '/api/seed/biome?' + ysmCommonParams() +
    '&dimension=' + S.dim + '&lod=1' +
    '&from_x=' + range.x0 + '&from_z=' + range.z0 + '&to_x=' + (range.x1 - 1) + '&to_z=' + (range.z1 - 1);
  ysmEnqueue('probe-batch', 0, batchUrl, function (d) {
    var pts = d && d.ok ? (d.points || d.results || d.biomes) : null;
    if (Array.isArray(pts) && pts.length) {
      S.bioMode = 'batch';
      ysmAbsorbPoints(pts);
      ysmEnsureBiome();
    } else if (d && d.ok) {
      S.bioMode = 'point'; // 批量请求通了但没有点列表 → 只当单点接口用
      ysmEnsureBiome();
    } else {
      failDead(d && d.error ? d.error : 'HTTP 批量探测失败');
    }
  }, function (e) { failDead(ysmErrText(e)); }, staleFn);

  function staleFn() { return !ysmS || ysmS !== S || !S.seedValid || S.dim !== (S.dim /*维度变化由换签逻辑清缓存兜住*/) || S.seed !== ysmSeedSig(); }
  function ysmSeedSig() { return S.seed + '|' + S.version + '|' + S.dim; }

  /* 单点探测延后到批量失败之后，避免一上来并发打两个探测 */
  var tryPoint = function () {
    if (!ysmS || ysmS.bioMode !== 'point') return;
    var mid = probeMiss[Math.floor(probeMiss.length / 2)];
    var url = '/api/seed/biome?' + ysmCommonParams() + '&dimension=' + ysmS.dim +
      '&x=' + (mid[0] + Math.floor(ysmS.cell / 2)) + '&z=' + (mid[1] + Math.floor(ysmS.cell / 2));
    ysmEnqueue('probe-point', 0, url, function (d) {
      if (d && d.ok && (d.biome != null || d.biome_id != null || d.name)) {
        ysmS.bioMode = 'point';
        ysmAbsorbPoint(mid[0], mid[1], d);
        ysmEnsureBiome();
      } else {
        failDead(d && d.error ? d.error : '单点探测无数据');
      }
    }, function (e) { failDead(ysmErrText(e)); });
  };
  /* 批量失败后走单点：在 onOk/onFail 里以 bioMode==='point' 触发 */
  var origFailDead = failDead;
  failDead = function (why) {
    if (ysmS && ysmS.bioMode === 'probing' && ysmS.bioFail === 0 && why.indexOf('HTTP') === 0) {
      ysmS.bioMode = 'point';
      tryPoint();
      return;
    }
    origFailDead(why);
  };
}
/* 上面探测流程用闭包包了 failDead，注意 batch 任务的 onOk/onFail 引用的是变量，
 * JS 闭包按引用捕获，最终生效的是包装后的版本 —— 单点探测因此会被触发。 */

function ysmEnqueueBatch(miss) {
  var S = ysmS;
  /* 把缺失格按 8×8 格一片分组，每片一个批量请求 */
  var groups = {};
  for (var i = 0; i < miss.length; i++) {
    var gx = miss[i][0], gz = miss[i][1];
    var bgx = Math.floor(gx / (S.cell * 8)) * S.cell * 8;
    var bgz = Math.floor(gz / (S.cell * 8)) * S.cell * 8;
    var gk = bgx + ',' + bgz;
    (groups[gk] = groups[gk] || { x0: gx, z0: gz, x1: gx + S.cell, z1: gz + S.cell }).x0 = Math.min(groups[gk].x0, gx);
    groups[gk].z0 = Math.min(groups[gk].z0, gz);
    groups[gk].x1 = Math.max(groups[gk].x1, gx + S.cell);
    groups[gk].z1 = Math.max(groups[gk].z1, gz + S.cell);
  }
  Object.keys(groups).forEach(function (gk) {
    var g = groups[gk];
    var url = '/api/seed/biome?' + ysmCommonParams() + '&dimension=' + S.dim + '&lod=1' +
      '&from_x=' + g.x0 + '&from_z=' + g.z0 + '&to_x=' + (g.x1 - 1) + '&to_z=' + (g.z1 - 1);
    ysmEnqueue('bioB:' + gk, 0, url, function (d) {
      var pts = d && d.ok ? (d.points || d.results || d.biomes) : null;
      if (Array.isArray(pts)) ysmAbsorbPoints(pts);
      if (d && d.precision) ysmS.precSeen[d.precision] = 1;
    }, function () {}, null);
  });
}
function ysmEnqueuePoints(miss) {
  var S = ysmS;
  for (var i = 0; i < miss.length; i++) {
    (function (gx, gz) {
      var url = '/api/seed/biome?' + ysmCommonParams() + '&dimension=' + S.dim +
        '&x=' + (gx + Math.floor(S.cell / 2)) + '&z=' + (gz + Math.floor(S.cell / 2));
      ysmEnqueue('bioP:' + S.cell + ':' + gx + ',' + gz, 0, url, function (d) {
        if (d && d.ok) ysmAbsorbPoint(gx, gz, d);
        if (d && d.precision) ysmS.precSeen[d.precision] = 1;
      }, function () {}, null);
    })(miss[i][0], miss[i][1]);
  }
}
/* 批量点 → 逐格写缓存（坐标按取样步长取整到所属格） */
function ysmAbsorbPoints(pts) {
  var S = ysmS;
  for (var i = 0; i < pts.length; i++) {
    var p = pts[i];
    var px = p && (p.x != null ? p.x : (Array.isArray(p) ? p[0] : null));
    var pz = p && (p.z != null ? p.z : (Array.isArray(p) ? p[1] : null));
    if (px == null || pz == null) continue;
    ysmAbsorbPoint(Math.floor(Number(px) / S.cell) * S.cell, Math.floor(Number(pz) / S.cell) * S.cell,
      Array.isArray(p) ? null : p);
  }
}
function ysmAbsorbPoint(gx, gz, d) {
  var S = ysmS;
  if (!S) return;
  var b = d ? (d.biome != null ? d.biome : (d.biome_id != null ? d.biome_id : (d.name || null))) : null;
  var name = d ? (d.name != null ? d.name : (d.biome_name != null ? d.biome_name : (b != null ? String(b) : ''))) : '';
  var key = S.cell + ':' + gx + ',' + gz;
  var col = ysmBiomeColor(typeof b === 'string' ? b : '');
  S.cache[key] = {
    b: b == null ? '' : String(b),
    name: name || (b == null ? '' : (col ? col[1] : (typeof b === 'number' ? ('群系 #' + b) : String(b)))),
    color: col ? col[0] : null,
    prec: d && d.precision ? d.precision : ''
  };
  if (S.cache[key].prec) S.precSeen[S.cache[key].prec] = 1;
  if (Object.keys(S.cache).length > 4000) S.cache = {}; // 缓存上限，防长时间滑动撑爆内存
}

/* ── 结构拉取：/api/seed/structure 优先，失败回退已文档化的 /api/youyuan/world/seed 试算 ── */
function ysmEnsureStructs() {
  var S = ysmS;
  if (!S || !S.seedValid || S.bioMode === 'dead' || S.paused) return;
  var range = ysmCellRange();
  if (!range) return;
  /* 可视范围外扩一档 2048 桶，滑动小距离不重复打接口 */
  var bx0 = Math.floor(range.x0 / 2048), bx1 = Math.floor(range.x1 / 2048);
  var bz0 = Math.floor(range.z0 / 2048), bz1 = Math.floor(range.z1 / 2048);
  var types = YSM_STRUCT_BY_DIM[S.dim] || [];
  for (var bi = 0; bi < 2; bi++) for (var bj = 0; bj < 2; bj++) {
    if (bx0 + bi > bx1 || bz0 + bj > bz1) continue;
    var tbx = bx0 + bi, tbz = bz0 + bj;
    /* 一个桶一次请求（type=all）拿该维度全部结构：
       15 种结构 × 4 桶 = 60 次配额 → 压到 4 次，直接对应用户「省额度」的要求 */
    var key = tbx + ',' + tbz;
    if (S.structBuckets[key]) continue;
    S.structBuckets[key] = 1;
    ysmFetchStruct(tbx, tbz, key);
  }
  if (!types.length) ysmRenderStrLegend('该维度暂无收录的结构类型（如实展示，不编造数据）。');
}

/* 拉一个 2048 方块桶的全部结构候选。
 * 坐标换算：后端 from_x/to_x 要区块坐标，桶是方块坐标，必须 /16；
 * 返回的 chunks 也是区块坐标，画图前要 *16 换成方块。 */
function ysmFetchStruct(tbx, tbz, bucketKey) {
  var S = ysmS;
  var fx = tbx * 2048, fz = tbz * 2048, tx = fx + 2048, tz = fz + 2048;
  var cfx = Math.floor(fx / 16), cfz = Math.floor(fz / 16);
  var ctx = Math.floor(tx / 16), ctz = Math.floor(tz / 16);
  var newUrl = '/api/seed/structure?' + ysmCommonParams() + '&dimension=' + S.dim +
    '&type=all&from_x=' + cfx + '&from_z=' + cfz + '&to_x=' + ctx + '&to_z=' + ctz;
  var stale = function () { return !ysmS || ysmS !== S || S.seed !== S.seed; };

  ysmEnqueue('stN:' + bucketKey, 1, newUrl, function (d) {
    if (d && d.ok && Array.isArray(d.structures)) {
      var got = 0;
      for (var i = 0; i < d.structures.length; i++) {
        var it = d.structures[i] || {};
        var rows = it.blocks || it.chunks || [];
        if (!rows.length) continue;
        ysmAbsorbStructs(it.type, ysmStructRowsToBlocks(it, rows), false, it.precision || d.precision);
        got++;
      }
      if (!got) ysmRenderStrLegend('当前范围没有结构候选（网格间距较大，缩小或移动视图再看）');
      return;
    }
    ysmStructFallbackAll(tbx, tbz, bucketKey);
  }, function () { ysmStructFallbackAll(tbx, tbz, bucketKey); }, stale);
}

/* 后端优先给 blocks（已是方块坐标）；只给 chunks 时这里补 *16。 */
function ysmStructRowsToBlocks(it, rows) {
  if (it && it.blocks && it.blocks.length) return it.blocks;
  var out = [];
  for (var i = 0; i < rows.length; i++) {
    var c = ysmChunkXY(rows[i]);
    if (c) out.push({ x: c[0] * 16, z: c[1] * 16 });
  }
  return out;
}

/* 新接口整桶失败时，逐个类型回退到已文档化的 world/seed 试算接口。
 * 只在真失败时才走这条路，正常情况一次请求就够。 */
function ysmStructFallbackAll(tbx, tbz, bucketKey) {
  var S = ysmS;
  if (!S) return;
  var types = YSM_STRUCT_BY_DIM[S.dim] || [];
  var fx = tbx * 2048, fz = tbz * 2048, tx = fx + 2048, tz = fz + 2048;
  for (var i = 0; i < types.length; i++) {
    ysmStructFallback(types[i], fx, fz, tx, tz, bucketKey + ':' + types[i]);
  }
}

/* 回退：world/seed 接口的 java_preview（区块坐标），文档明确要求 edition=java */
function ysmStructFallback(type, fx, fz, tx, tz, bucketKey) {
  var S = ysmS;
  var url = '/api/youyuan/world/seed?seed=' + encodeURIComponent(S.seed) +
    '&structure=' + type + '&edition=java' +
    '&from_x=' + Math.floor(fx / 16) + '&from_z=' + Math.floor(fz / 16) +
    '&to_x=' + Math.floor(tx / 16) + '&to_z=' + Math.floor(tz / 16);
  ysmEnqueue('stF:' + bucketKey, 1, url, function (d) {
    var jp = d && d.java_preview;
    if (d && d.ok && jp && Array.isArray(jp.chunks)) {
      var rows = [];
      for (var i = 0; i < jp.chunks.length; i++) {
        var c = ysmChunkXY(jp.chunks[i]);
        if (c) rows.push({ x: c[0] * 16, z: c[1] * 16 });
      }
      ysmAbsorbStructs(type, rows, true, jp.precision || 'candidate');
      if (jp.truncated) ysmToast('结构候选被截断（后端预算限制），放大或缩小范围后重试', 'w');
    }
    /* world/seed 也拿不到（404 / 400 / ok:false）→ 该桶结构数据缺失，图例注明即可 */
  }, function () {}, null);
}

function ysmChunkXY(c) {
  if (Array.isArray(c) && c.length >= 2) {
    var ax = Number(c[0]), az = Number(c[1]);
    if (isFinite(ax) && isFinite(az)) return [ax, az];
    return null;
  }
  if (c && typeof c === 'object') {
    var x = c.x != null ? c.x : (c.chunk_x != null ? c.chunk_x : (c.cx != null ? c.cx : null));
    var z = c.z != null ? c.z : (c.chunk_z != null ? c.chunk_z : (c.cz != null ? c.cz : null));
    if (x != null && z != null && isFinite(Number(x)) && isFinite(Number(z))) return [Number(x), Number(z)];
  }
  return null;
}
function ysmAbsorbStructs(type, rows, trial, prec) {
  var S = ysmS;
  var meta = YSM_STRUCT_META[type] || { label: type, color: '#777777' };
  for (var i = 0; i < rows.length; i++) {
    var r = rows[i] || {};
    var x = Number(r.x != null ? r.x : (Array.isArray(r) ? r[0] : NaN));
    var z = Number(r.z != null ? r.z : (Array.isArray(r) ? r[1] : NaN));
    if (!isFinite(x) || !isFinite(z)) continue;
    var dup = false;
    for (var k = 0; k < S.pins.length; k++) if (S.pins[k].type === type && S.pins[k].x === x && S.pins[k].z === z) { dup = true; break; }
    if (dup) continue;
    S.pins.push({
      type: type, label: (r.label || r.name || meta.label),
      x: x, z: z, color: meta.color,
      prec: prec || '', trial: !!trial
    });
  }
  ysmRenderStrLegend('');
  ysmSchedule();
}

/* ── 视口几何 ── */
function ysmCellRange() {
  var S = ysmS;
  var cv = ysm$('ysm-canvas');
  if (!cv || !S) return null;
  var rect = cv.getBoundingClientRect();
  var W = Math.max(1, rect.width), H = Math.max(1, rect.height);
  var halfW = W / 2 / S.scale, halfH = H / 2 / S.scale;
  return {
    W: W, H: H,
    x0: Math.floor((S.cx - halfW) / S.cell) * S.cell,
    x1: Math.ceil((S.cx + halfW) / S.cell) * S.cell,
    z0: Math.floor((S.cz - halfH) / S.cell) * S.cell,
    z1: Math.ceil((S.cz + halfH) / S.cell) * S.cell
  };
}
function ysmBlockToPx(x, W) { return W / 2 + (x - ysmS.cx) * ysmS.scale; }
function ysmBlockToPy(z, H) { return H / 2 + (z - ysmS.cz) * ysmS.scale; }
function ysmPxToBlockX(px, W) { return ysmS.cx + (px - W / 2) / ysmS.scale; }
function ysmPxToBlockZ(py, H) { return ysmS.cz + (py - H / 2) / ysmS.scale; }

/* ── 绘制 ── */
function ysmSchedule() {
  if (ysmRaf || !ysmS || !ysmS.active) return;
  ysmRaf = requestAnimationFrame(ysmDraw);
}
function ysmDraw() {
  ysmRaf = 0;
  var S = ysmS;
  var cv = ysm$('ysm-canvas');
  if (!cv || !S || !S.active) return;
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
  var txt = (css.getPropertyValue('--phx-text-secondary') || '#6b7a83').trim() || '#6b7a83';
  g.fillStyle = bg;
  g.fillRect(0, 0, W, H);

  var range = ysmCellRange();
  if (!range) return;

  /* 未输种子 / 接口挂了的底：只画提示，不画任何假色 */
  if (!S.seedValid) {
    ysmShowOverlay('先在上方输入种子', '种子是地图与结构的输入；读不到当前世界种子就手动填。');
  } else if (S.bioMode !== 'dead') {
    ysmHideOverlay();
  }
  if (!S.seedValid || S.bioMode === 'dead') { ysmUpdateReadout(); return; }

  /* 群系格子：缓存命中的上色，未命中的画淡格占位 */
  var cellPx = S.cell * S.scale;
  var pending = 0;
  for (var gz = range.z0; gz < range.z1; gz += S.cell) {
    for (var gx = range.x0; gx < range.x1; gx += S.cell) {
      var it = S.cache[S.cell + ':' + gx + ',' + gz];
      var px = ysmBlockToPx(gx, W), py = ysmBlockToPy(gz, H);
      if (it && it.color) {
        g.fillStyle = it.color;
        g.fillRect(px, py, Math.ceil(cellPx), Math.ceil(cellPx));
      } else if (it) {
        g.fillStyle = '#8a949c55';
        g.fillRect(px, py, Math.ceil(cellPx), Math.ceil(cellPx));
      } else {
        pending++;
        g.strokeStyle = line;
        g.lineWidth = 1;
        g.strokeRect(px + 0.5, py + 0.5, Math.max(1, cellPx - 1), Math.max(1, cellPx - 1));
      }
    }
  }

  /* 网格线 + 原点轴 */
  g.strokeStyle = line;
  g.lineWidth = 1;
  g.beginPath();
  g.moveTo(Math.round(ysmBlockToPx(0, W)) + 0.5, 0);
  g.lineTo(Math.round(ysmBlockToPx(0, W)) + 0.5, H);
  g.moveTo(0, Math.round(ysmBlockToPy(0, H)) + 0.5);
  g.lineTo(W, Math.round(ysmBlockToPy(0, H)) + 0.5);
  g.stroke();

  /* 结构图钉（画在群系之上） */
  S.pins = S.pins.filter(function (p) {
    var px = ysmBlockToPx(p.x, W), py = ysmBlockToPy(p.z, H);
    p.px = px; p.py = py;
    return px > -30 && px < W + 30 && py > -30 && py < H + 30;
  });
  var visiblePins = S.pins;
  var fullPins = visiblePins.concat(S.pins.filter(function (p) { return p.px == null; }));
  for (var i = 0; i < S.pins.length; i++) ysmDrawPin(g, S.pins[i].px, S.pins[i].py, S.pins[i].color);

  if (pending && !S.dragging) {
    g.fillStyle = txt;
    g.font = '11px system-ui,sans-serif';
    g.textAlign = 'center';
    g.fillText('群系取样中…（并发 2 · 已缓存 ' + Object.keys(S.cache).length + ' 格）', W / 2, H - 10);
    g.textAlign = 'left';
  }
  if (!visiblePins.length && (range.x1 - range.x0) / 16 <= 4096 / 16) {
    /* 结构候选可能还没回来，不提示空，等 ysmEnsureStructs 补 */
  }

  ysmDrawLegend();
  ysmUpdateReadout();

  /* 拖动静止后补拉新增可视区（拖动中不排队，等 onUp 的防抖） */
  if (!S.dragging) {
    ysmEnsureBiome();
    ysmEnsureStructs();
    ysmPump();
  }
}

function ysmDrawPin(g, x, y, color) {
  g.strokeStyle = '#ffffff';
  g.lineWidth = 1.5;
  g.fillStyle = color;
  g.beginPath();
  g.moveTo(x, y);
  g.lineTo(x + 4, y - 7);
  g.arc(x, y - 13, 6.5, -Math.PI * 0.18, Math.PI * 1.18); // 图钉头
  g.closePath();
  g.fill();
  g.stroke();
  g.fillStyle = '#ffffff';
  g.beginPath();
  g.arc(x, y - 13, 2.2, 0, Math.PI * 2);
  g.fill();
}

/* ── 图例 / 读数 / 结构图例 ── */
function ysmDrawLegend() {
  var S = ysmS;
  var box = ysm$('ysm-biolegend');
  if (!box || !S) return;
  var seen = {}, items = [];
  Object.keys(S.cache).forEach(function (k) {
    var it = S.cache[k];
    if (!it || !it.color || seen[it.name]) return;
    seen[it.name] = 1;
    items.push({ name: it.name, color: it.color });
  });
  if (!items.length) { box.innerHTML = '<span class="ysm-dim">暂无群系数据（取样中或接口未返回颜色可识别的群系）</span>'; }
  else {
    box.innerHTML = items.slice(0, 24).map(function (it) {
      return '<span class="ysm-lgi"><i style="background:' + it.color + '"></i>' + ysmEsc(it.name) + '</span>';
    }).join('');
  }
}
function ysmRenderStrLegend(note) {
  var S = ysmS;
  var box = ysm$('ysm-strlegend');
  if (!box || !S) return;
  var types = YSM_STRUCT_BY_DIM[S.dim] || [];
  var html = types.map(function (t) {
    var m = YSM_STRUCT_META[t];
    return '<span class="ysm-lgi"><i class="ysm-pin-dot" style="background:' + m.color + '"></i>' + ysmEsc(m.label) + '</span>';
  }).join('');
  /* 概率型结构（废弃矿井/埋藏的宝藏）按区块概率生成，没有网格，地图给不出点。
     如实写在这里，而不是假装「扫过了但一个都没有」。 */
  var probs = YSM_STRUCT_PROB.filter(function (id) { return YSM_STRUCT_META[id].dim === S.dim; });
  if (probs.length) {
    html += '<span class="ysm-dim">按区块概率生成、地图无法给点：' +
      probs.map(function (id) {
        var m = YSM_STRUCT_META[id];
        return ysmEsc(m.label) + ' ' + (m.prob * 100).toFixed(1) + '%';
      }).join('、') + '</span>';
  }
  if (note) html += '<span class="ysm-dim">' + ysmEsc(note) + '</span>';
  box.innerHTML = html;
}
function ysmUpdateReadout() {
  var S = ysmS;
  var el = ysm$('ysm-readout');
  if (!el || !S) return;
  var prec = '';
  var keys = Object.keys(S.precSeen);
  if (keys.length) {
    var labs = [];
    keys.forEach(function (k) { var l = ysmPrecLabel(k); if (l && labs.indexOf(l) < 0) labs.push(l); });
    prec = labs.join(' / ');
  } else {
    prec = 'precision 未标注（后端没给，不冒充精确）';
  }
  var bioState = S.bioMode === 'dead' ? ' · 群系接口不可用' :
    (S.bioMode === 'point' ? ' · 单点模式' : (S.bioMode === 'batch' ? ' · 批量模式' : ''));
  el.innerHTML = '<b>X ' + Math.round(S.cx) + ' / Z ' + Math.round(S.cz) + '</b>' +
    '<span> · ' + YSM_DIM_LABEL[S.dim] + ' · 取样步长 ' + S.cell + ' 格' + bioState + '</span>' +
    '<span class="ysm-dim"> · 数据来源：' + ysmEsc(prec) + '</span>';
}

/* ── 覆盖层（诚实空态） ── */
function ysmShowOverlay(title, sub) {
  var el = ysm$('ysm-overlay');
  if (!el) return;
  el.hidden = false;
  el.innerHTML = '<div class="ysm-overlay-box"><strong>' + ysmEsc(title) + '</strong>' +
    (sub ? '<span>' + sub + '</span>' : '') + '</div>';
}
function ysmHideOverlay() {
  var el = ysm$('ysm-overlay');
  if (el) { el.hidden = true; el.innerHTML = ''; }
}

/* ── 图钉卡片 ── */
function ysmShowPinCard(pin) {
  var card = ysm$('ysm-pincard');
  if (!card || !ysmS) return;
  var dist = Math.round(Math.hypot(pin.x - ysmS.cx, pin.z - ysmS.cz));
  var prec = pin.trial ? 'Java 算法试算候选（近似）' : (pin.prec ? ysmPrecLabel(pin.prec) : 'precision 未标注');
  card.hidden = false;
  card.innerHTML = '<div class="ysm-pincard-head"><i style="background:' + pin.color + '"></i>' +
    '<strong>' + ysmEsc(pin.label) + '</strong>' +
    '<button type="button" class="ysm-pincard-x" aria-label="关闭">×</button></div>' +
    '<div class="ysm-pincard-row">方块坐标：<b>X ' + Math.round(pin.x) + ' / Z ' + Math.round(pin.z) + '</b></div>' +
    '<div class="ysm-pincard-row">距视口中心约 ' + dist + ' 格</div>' +
    '<div class="ysm-pincard-row ysm-dim">数据来源：' + ysmEsc(prec) +
    (pin.trial ? '，非本世界真实位置，需进游戏实测' : '') + '</div>';
  var x = ysmClamp(pin.px + 10, 6, Math.max(6, card.parentElement.clientWidth - 190));
  var y = ysmClamp(pin.py + 10, 6, Math.max(6, card.parentElement.clientHeight - 120));
  card.style.left = x + 'px';
  card.style.top = y + 'px';
  var xb = card.querySelector('.ysm-pincard-x');
  if (xb) xb.onclick = function () { card.hidden = true; };
}

/* ── 交互绑定 ── */
function ysmBind() {
  var S = ysmS;
  var cv = ysm$('ysm-canvas');
  var wrap = ysm$('ysm-wrap');
  if (!cv || !wrap) return;

  var pointers = {}, last = null, pinch = null, moved = 0, downAt = 0, downPos = null;
  function localPos(e) {
    var r = cv.getBoundingClientRect();
    return { x: e.clientX - r.left, y: e.clientY - r.top };
  }
  function onDown(e) {
    if (e.button != null && e.button !== 0) return;
    if (cv.setPointerCapture) try { cv.setPointerCapture(e.pointerId); } catch (err) {}
    pointers[e.pointerId] = localPos(e);
    moved = 0; downAt = Date.now(); downPos = localPos(e);
    var arr = Object.keys(pointers);
    if (arr.length === 2) {
      var a = pointers[arr[0]], b = pointers[arr[1]];
      pinch = { dist: Math.hypot(a.x - b.x, a.y - b.y) || 1, scale: S.scale,
        mid: { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 }, cx: S.cx, cz: S.cz };
    }
    S.dragging = true; last = localPos(e);
    var pc = ysm$('ysm-pincard'); if (pc) pc.hidden = true;
  }
  function onMove(e) {
    if (!pointers[e.pointerId]) return;
    var p = localPos(e);
    pointers[e.pointerId] = p;
    var rect = cv.getBoundingClientRect();
    var ids = Object.keys(pointers);
    if (ids.length >= 2 && pinch) {
      var a = pointers[ids[0]], b = pointers[ids[1]];
      var dist = Math.hypot(a.x - b.x, a.y - b.y) || 1;
      var next = ysmClamp(pinch.scale * (dist / pinch.dist), 0.01, 8);
      var ax = pinch.cx + (pinch.mid.x - rect.width / 2) / pinch.scale;
      var az = pinch.cz + (pinch.mid.y - rect.height / 2) / pinch.scale;
      S.scale = next;
      S.cx = ax - (pinch.mid.x - rect.width / 2) / next;
      S.cz = az - (pinch.mid.y - rect.height / 2) / next;
      moved += 12; ysmSchedule(); return;
    }
    if (!last) { last = p; return; }
    var dx = p.x - last.x, dy = p.y - last.y;
    moved += Math.abs(dx) + Math.abs(dy);
    if (moved > 3) { S.cx -= dx / S.scale; S.cz -= dy / S.scale; }
    last = p;
    ysmSchedule();
  }
  function onUp(e) {
    delete pointers[e.pointerId];
    if (Object.keys(pointers).length < 2) pinch = null;
    last = null;
    if (!Object.keys(pointers).length) {
      S.dragging = false;
      /* 拖动停止 260ms 后补拉新增可视区域；期间又拖就重置计时 */
      if (ysmDeferTimer) clearTimeout(ysmDeferTimer);
      ysmDeferTimer = setTimeout(function () {
        ysmDeferTimer = 0;
        if (!ysmS || !ysmS.active) return;
        ysmSyncCell();
        ysmEnsureBiome(); ysmEnsureStructs(); ysmPump();
        ysmSchedule();
      }, 260);
    }
    if (!downPos || moved >= 8 || Date.now() - downAt > 500) { ysmSchedule(); return; }
    /* 轻点：优先命中图钉，其次只是收起卡片 */
    var p = localPos(e);
    var best = null, bestD = 26;
    for (var i = 0; i < S.pins.length; i++) {
      var d = Math.hypot(S.pins[i].px - p.x, S.pins[i].py - p.y);
      if (d < bestD) { bestD = d; best = S.pins[i]; }
    }
    if (best) ysmShowPinCard(best);
    ysmSchedule();
  }
  function zoomAt(px, py, factor) {
    var rect = cv.getBoundingClientRect();
    var ax = S.cx + (px - rect.width / 2) / S.scale;
    var az = S.cz + (py - rect.height / 2) / S.scale;
    S.scale = ysmClamp(S.scale * factor, 0.01, 8);
    S.cx = ax - (px - rect.width / 2) / S.scale;
    S.cz = az - (py - rect.height / 2) / S.scale;
    ysmSyncCell();
    if (ysmDeferTimer) clearTimeout(ysmDeferTimer);
    ysmDeferTimer = setTimeout(function () {
      ysmDeferTimer = 0;
      if (!ysmS || !ysmS.active) return;
      ysmEnsureBiome(); ysmEnsureStructs(); ysmPump(); ysmSchedule();
    }, 260);
    ysmSchedule();
  }
  cv.addEventListener('pointerdown', onDown);
  cv.addEventListener('pointermove', onMove);
  cv.addEventListener('pointerup', onUp);
  cv.addEventListener('pointercancel', onUp);
  cv.addEventListener('wheel', function (e) {
    e.preventDefault();
    var p = localPos(e);
    zoomAt(p.x, p.y, e.deltaY < 0 ? 1.25 : 1 / 1.25);
  }, { passive: false });
  /* 老 WebView 兜底：无 PointerEvent 时用触摸平移 */
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
      e.preventDefault(); ysmSchedule();
    }, { passive: false });
    cv.addEventListener('touchend', function () {
      touching = false;
      if (ysmDeferTimer) clearTimeout(ysmDeferTimer);
      ysmDeferTimer = setTimeout(function () {
        ysmDeferTimer = 0;
        if (!ysmS || !ysmS.active) return;
        ysmSyncCell(); ysmEnsureBiome(); ysmEnsureStructs(); ysmPump(); ysmSchedule();
      }, 260);
    }, { passive: true });
  }

  Array.prototype.forEach.call(wrap.querySelectorAll('.ysm-tools [data-act]'), function (b) {
    b.onclick = function () {
      var act = b.getAttribute('data-act');
      if (act === 'in') zoomAt(cv.getBoundingClientRect().width / 2, cv.getBoundingClientRect().height / 2, 1.6);
      else if (act === 'out') zoomAt(cv.getBoundingClientRect().width / 2, cv.getBoundingClientRect().height / 2, 1 / 1.6);
      else { S.cx = 0; S.cz = 0; ysmSyncCell(); ysmSchedule(); }
    };
  });

  /* 维度 tab */
  var dims = ysm$('ysm-dims');
  if (dims) dims.onclick = function (e) {
    var b = e.target.closest ? e.target.closest('button[data-dim]') : null;
    if (!b || b.getAttribute('data-dim') === S.dim) return;
    S.dim = b.getAttribute('data-dim');
    Array.prototype.forEach.call(dims.querySelectorAll('button'), function (x) {
      x.classList.toggle('on', x.getAttribute('data-dim') === S.dim);
    });
    ysmResetData();
    ysmSchedule();
  };

  /* 版本下拉 */
  var ver = ysm$('ysm-version');
  if (ver) ver.onchange = function () {
    S.version = ver.value;
    ysmResetData();
    ysmSchedule();
  };

  /* 种子输入 */
  var input = ysm$('ysm-seed');
  if (input) {
    var apply = function () {
      var raw = input.value.trim();
      if (!raw) {
        S.seed = ''; S.seedValid = false;
        ysmResetData(); ysmSchedule(); return;
      }
      if (!/^[+-]?\d{1,20}$/.test(raw)) { ysmToast('种子必须是十进制整数，例如 123456789 或 -123456789', 'e'); return; }
      var big;
      try { big = BigInt(raw); } catch (e2) { ysmToast('种子不是合法整数', 'e'); return; }
      if (big < -9223372036854775808n || big > 9223372036854775807n) { ysmToast('种子超出 64 位整数范围', 'e'); return; }
      S.seed = big.toString();
      S.seedValid = true;
      S.seedFromKernel = false;
      ysmResetData();
      var note = ysm$('ysm-note');
      if (note) note.textContent = '手输种子 · 换种子 / 版本 / 维度会自动重新取样';
      ysmSchedule();
    };
    input.onchange = apply;
    input.onkeydown = function (e) { if (e.key === 'Enter') { e.preventDefault(); input.blur(); apply(); } };
  }

  /* 读当前种子 */
  var readBtn = ysm$('ysm-read');
  if (readBtn) readBtn.onclick = function () { ysmReadWorldSeed(readBtn); };

  /* 切走暂停取点 */
  document.addEventListener('visibilitychange', ysmOnVis);
  if (window.ResizeObserver) {
    var ro = new ResizeObserver(function () { ysmSchedule(); });
    ro.observe(wrap);
    S._ro = ro;
  } else {
    window.addEventListener('resize', ysmSchedule);
  }
}

function ysmOnVis() {
  if (!ysmS) return;
  if (document.hidden) {
    ysmS.paused = true;
    ysmFlushQueue(); // 掐断在途请求，省配额
  } else {
    ysmS.paused = false;
    ysmEnsureBiome(); ysmEnsureStructs(); ysmPump();
    ysmSchedule();
  }
}

/* 换种子 / 版本 / 维度后：清缓存、清队列、清结构桶 */
function ysmResetData() {
  var S = ysmS;
  if (!S) return;
  S.cache = {};
  S.pins = [];
  S.structBuckets = {};
  S.precSeen = {};
  S.bioMode = null;
  S.bioFail = 0;
  S.pendingBio = 0;
  ysmFlushQueue();
  ysmHideOverlay();
  ysmRenderStrLegend('');
  var box = ysm$('ysm-biolegend');
  if (box) box.innerHTML = '<span class="ysm-dim">暂无群系数据</span>';
}

/* 缩放后同步取样步长 */
function ysmSyncCell() {
  if (!ysmS) return;
  var next = ysmPickCell();
  if (next !== ysmS.cell) ysmS.cell = next;
}

/* ── 种子预填 / 版本清单 ── */
async function ysmReadWorldSeed(btn) {
  var note = ysm$('ysm-note');
  if (btn) btn.disabled = true;
  if (note) note.textContent = '正在从本机内核读取世界种子…';
  try {
    var r = await fetch('/api/youyuan/world/seed').then(function (x) { return x.json(); });
    if (!r || !r.ok || !r.seed_known) {
      var msg = (r && r.error) || '尚未从内核读取到世界种子：请先连接服务器，或手动输入';
      if (note) note.textContent = msg;
      ysmToast(msg, 'e');
      return;
    }
    var text = r.seed_text != null ? String(r.seed_text) : String(r.seed);
    var zero = /^[+-]?0+$/.test(text);
    ysmSeedPre = { text: text, zero: zero, fromKernel: true };
    var input = ysm$('ysm-seed');
    if (input) input.value = text;
    if (ysmS) {
      ysmS.seed = text;
      ysmS.seedValid = true;
      ysmS.seedFromKernel = true;
      ysmS.seedZero = zero;
      ysmResetData();
      ysmSchedule();
    }
    if (note) note.textContent = '已读取当前世界种子 ' + text + '（基岩版语义，来自 StartGame 包）' +
      (zero ? ' · 注意：服务端把真实种子替换成了 0，别当成真实种子' : '');
  } catch (e) {
    if (note) note.textContent = '读取失败：' + ysmErrText(e) + '，可手动输入种子';
  } finally {
    if (btn) btn.disabled = false;
  }
}

/* manifest 解析：优先 game_versions（真实游戏版本），其次旧形状；
 * version_matrix 是「覆盖区间矩阵」，不能拿来填版本下拉。
 * 解析不出 → 内置默认列表 + 「默认」徽标。 */
function ysmParseManifest(r) {
  var arr = null;
  try {
    if (r && Array.isArray(r.game_versions)) arr = r.game_versions;
    else if (r && Array.isArray(r.versions)) arr = r.versions;
    else if (r && r.editions && typeof r.editions === 'object') {
      var e = r.editions.java || r.editions.bedrock || null;
      if (e) arr = Array.isArray(e) ? e : (e.versions || e.list || null);
    } else if (r && Array.isArray(r.matrix)) arr = r.matrix;
    else if (r && Array.isArray(r.ranges)) arr = r.ranges;
    else if (r && r.ok === false) return null;
    if (arr && arr.length) {
      var list = [];
      for (var i = 0; i < arr.length; i++) {
        var it = arr[i];
        if (typeof it === 'string') list.push(it);
        else if (it && typeof it === 'object') {
          var v = it.version || it.id || it.name || it.v || it.vc;
          if (v) list.push(String(v));
        }
      }
      if (list.length) return list;
    }
  } catch (e2) {}
  return null;
}
async function ysmLoadManifest() {
  var sel = ysm$('ysm-version');
  var badge = ysm$('ysm-verbadge');
  if (!sel || !ysmS) return;
  var list = ysmManifest;
  if (!list) {
    try {
      var r = await fetch('/api/seed/manifest').then(function (x) { return x.json(); });
      list = ysmParseManifest(r);
      if (list) ysmManifest = list;
    } catch (e) { list = null; }
  }
  ysmS.versions = list || YSM_DEFAULT_VERSIONS;
  ysmS.versionsDefault = !list;
  ysmS.version = ysmS.versions.indexOf('1.20') >= 0 ? '1.20' : ysmS.versions[0];
  sel.innerHTML = ysmS.versions.map(function (v) {
    return '<option value="' + ysmEsc(v) + '">' + ysmEsc(v) + '</option>';
  }).join('');
  sel.value = ysmS.version;
  if (badge) {
    badge.hidden = false;
    badge.textContent = list ? '版本区间矩阵' : '默认';
    badge.title = list ? '来自 /api/seed/manifest' : '/api/seed/manifest 不可用，使用内置默认列表';
  }
}

/* ── 入口 / 恢复 ── */
function ysmInit() {
  ysmS = ysmNewState();
  ysmSyncCell();
  ysmBind();
  ysmLoadManifest();
  /* 预填当前世界种子：有缓存直接用，没有就拉一次 */
  if (ysmSeedPre && ysmSeedPre.text) {
    var input = ysm$('ysm-seed');
    if (input) input.value = ysmSeedPre.text;
    ysmS.seed = ysmSeedPre.text;
    ysmS.seedValid = true;
    ysmS.seedFromKernel = true;
    ysmS.seedZero = !!ysmSeedPre.zero;
    var note = ysm$('ysm-note');
    if (note) note.textContent = '已预填当前世界种子（基岩版语义）' + (ysmSeedPre.zero ? ' · seed=0 应视为未知' : '');
    ysmSchedule();
  } else {
    ysmReadWorldSeed(null);
  }
  /* 页面被切走 / 返回时 openSubPage 会调用 activeRenderers 里的清理函数 */
  if (typeof activeRenderers !== 'undefined' && Array.isArray(activeRenderers)) {
    activeRenderers.push(function () {
      if (!ysmS) return;
      ysmS.active = false;
      if (ysmDeferTimer) { clearTimeout(ysmDeferTimer); ysmDeferTimer = 0; }
      if (ysmRaf) { cancelAnimationFrame(ysmRaf); ysmRaf = 0; }
      ysmFlushQueue();
      document.removeEventListener('visibilitychange', ysmOnVis);
      window.removeEventListener('resize', ysmSchedule);
      if (ysmS._ro) { try { ysmS._ro.disconnect(); } catch (e) {} }
      ysmS = null;
    });
  }
  /* 首帧等布局完成，否则 canvas 尺寸为 0 */
  setTimeout(function () { ysmSyncCell(); ysmSchedule(); }, 30);
}
function ysmReinit() {
  /* 缓存恢复 / 返回上一页：状态是旧的 DOM 是克隆的，重建一份即可 */
  ysmInit();
}

/* 子页缓存恢复时补跑初始化（app.js 在本文件之后加载，所以 setTimeout(0)） */
setTimeout(function () {
  if (typeof subPageInit !== 'function') return;
  subPageInit(YSM_TITLE, function () { ysmReinit(); });
}, 0);
