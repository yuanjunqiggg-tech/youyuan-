// ═══ 像素画画板 ═══
// 交互式画布：绘制像素画，实时/批量同步到游戏。状态唯一真源在 Go DrawSession。
// 调色板分层：始终显示颜色类别 → 点颜色展开该类别的所有方块 → 选方块为画笔。
var drawPaletteMap = {};      // name -> {r,g,b,tex}
var drawPaletteLoaded = false;
var drawSess = { grid: null, region: null, placing: false, mode: 'realtime' };
var drawBrush = '';
var drawFill = '';
var drawCreateMode = 'transparent';
var drawSyncMode = 'realtime';
var drawTool = 'brush';
var drawZoom = 16, drawOffX = 0, drawOffY = 0;
var drawCurStroke = [];
var drawPointer = false;
var drawLastPos = null;   // 上一次指针网格坐标（用于线段插值，避免快速拖动断点）
var drawTexCache = {};    // 方块名 -> HTMLImageElement（画布显示纹理）
var drawSyncing = false;  // 同步中标记（禁用同步按钮与绘画）

// 16 个标准染布色类别
var DRAW_DYES = [
  { k: 'white', c: [250, 250, 250] }, { k: 'orange', c: [230, 120, 0] }, { k: 'magenta', c: [190, 50, 180] },
  { k: 'light_blue', c: [60, 150, 220] }, { k: 'yellow', c: [250, 200, 20] }, { k: 'lime', c: [110, 200, 20] },
  { k: 'pink', c: [240, 130, 170] }, { k: 'gray', c: [130, 130, 130] }, { k: 'light_gray', c: [180, 180, 180] },
  { k: 'cyan', c: [20, 140, 150] }, { k: 'purple', c: [120, 40, 160] }, { k: 'blue', c: [40, 60, 160] },
  { k: 'brown', c: [90, 55, 30] }, { k: 'green', c: [80, 120, 30] }, { k: 'red', c: [160, 40, 40] }, { k: 'black', c: [20, 20, 20] }
];
function drawClassify(r, g, b) {
  var best = 0, bd = 1e18;
  for (var i = 0; i < DRAW_DYES.length; i++) {
    var d = DRAW_DYES[i].c;
    var dist = (d[0] - r) * (d[0] - r) + (d[1] - g) * (d[1] - g) + (d[2] - b) * (d[2] - b);
    if (dist < bd) { bd = dist; best = i; }
  }
  return DRAW_DYES[best];
}
// 前一半纹理、后一半颜色的 CSS background
function drawSplitStyle(c) {
  if (c.tex) return 'linear-gradient(90deg, transparent 50%, rgb(' + c.r + ',' + c.g + ',' + c.b + ') 50%), url(/blocks/' + c.tex + '.png) center/cover';
  return 'rgb(' + c.r + ',' + c.g + ',' + c.b + ')';
}
// 按颜色分类
function drawCategories() {
  var cats = {};
  Object.keys(drawPaletteMap).forEach(function (name) {
    var c = drawPaletteMap[name];
    // 优先用纹理平均色分类（樱桃木等：纹理主色红/粉而非中色棕）
    var cc = (c.tc && c.tc.r != null) ? c.tc : { r: c.r, g: c.g, b: c.b };
    var dye = drawClassify(cc.r, cc.g, cc.b);
    if (!cats[dye.k]) cats[dye.k] = { k: dye.k, c: dye.c, blocks: [] };
    cats[dye.k].blocks.push(name);
  });
  return cats;
}

function RdrawHTML() {
  return '<style>.dr-cat{width:30px;height:30px;border-radius:50%;cursor:pointer;border:2px solid transparent;box-sizing:border-box;flex-shrink:0}.dr-cat:hover{border-color:#fff}.dr-cat.on{border-color:var(--phx-primary,#d48a0e)}.dr-cats{display:flex;flex-wrap:wrap;gap:8px}@keyframes drFlash{0%,100%{opacity:.35}50%{opacity:1}}.dr-syncbox{position:absolute;inset:0;z-index:5;pointer-events:none;border:3px solid #22c55e;border-radius:8px;box-shadow:0 0 14px rgba(34,197,94,.7);animation:drFlash .8s infinite}</style>' +
    '<div class="card" style="padding:14px">' +
    '<div id="dr-float-nosession" class="gone" style="padding:24px;text-align:center;color:#94a3b8;font-size:13px">请先在主界面创建画布</div>' +

    // ── 创建区 ──
    '<div id="dr-create-section">' +
    '<div class="tsk-section" style="margin-top:0"><div class="tsk-header">创建画布</div>' +
    '<label>起点</label><div class="row"><input id="dr-x1" type="number" placeholder="X1"><input id="dr-z1" type="number" placeholder="Z1"></div>' +
    '<label>两条边长（不必相等，最大各 512）</label><div class="row"><input id="dr-w" type="number" placeholder="X 边长 W"><input id="dr-d" type="number" placeholder="Z 边长 D"></div>' +
    '<label>水平高度 Y（画布平面）</label><input id="dr-y" type="number" placeholder="Y">' +
    '<label>创建模式</label><button class="btn-s" id="dr-mode-btn" onclick="drawPickCreateMode()" style="width:100%;text-align:left">纯透明（不动原有方块）</button>' +
    '<div id="dr-fill-row" class="gone" style="margin-top:6px"><label>填充方块</label><button class="btn-s" id="dr-fill-btn" onclick="drawPickFill()" style="width:100%;text-align:left">选择方块</button><div class="dim" id="dr-fill-name" style="font-size:10px">未选择</div></div>' +
    '<label>同步方式</label><button class="btn-s" id="dr-sync-btn" onclick="drawPickSync()" style="width:100%;text-align:left">实时（每笔同步）</button>' +
    '<button class="btn-s" id="dr-create" onclick="drawCreate()" style="width:100%;margin:8px 0 4px">创建画布</button>' +
    '<div class="dim" id="dr-hint" style="font-size:10px;line-height:1.5">画布默认空白，直接画。点「同步平面」才读取世界当前平面。</div>' +
    '</div></div>' +

    // ── 调色板（置于编辑器底部）──

    // ── 编辑器区 ──
    '<div id="dr-editor-section" class="gone">' +
    '<div class="row" style="gap:6px;margin:8px 0">' +
    '<button class="btn-s" id="dr-tool-brush" onclick="drawSetTool(\'brush\')" style="flex:1">画</button>' +
    '<button class="btn-s" id="dr-tool-eraser" onclick="drawSetTool(\'eraser\')" style="flex:1">擦</button>' +
    '<button class="btn-s" id="dr-tool-pick" onclick="drawSetTool(\'pick\')" style="flex:1">取色</button>' +
    '<button class="btn-s" id="dr-pickblock" onclick="drawPickBrush()" style="flex:1.4">选方块</button>' +
    '</div>' +
    '<div class="row" style="gap:6px;margin:4px 0">' +
    '<button class="btn-s" id="dr-undo" onclick="drawUndo()" style="flex:1">撤销</button>' +
    '<button class="btn-s" id="dr-redo" onclick="drawRedo()" style="flex:1">重做</button>' +
    '<button class="btn-s" id="dr-apply" onclick="drawApply()" style="flex:1">应用</button>' +
    '<button class="btn-s" id="dr-sync" onclick="drawSync()" style="flex:1">同步平面</button>' +
    '</div>' +
    '<div class="row" style="gap:6px;margin:4px 0">' +
    '<button class="btn-s" id="dr-closecanvas" onclick="drawCloseCanvas()" style="flex:1">关闭画布</button>' +
    '<button class="btn-s" id="dr-floatbtn" onclick="drawToggleFloat()" style="flex:1">悬浮窗</button>' +
    '</div>' +
    '<div id="dr-busy" class="gone" style="text-align:center;color:#ff6b6b;font-size:11px;font-weight:600;margin:2px 0">机器人放置中…</div>' +
    '<div class="dim" style="font-size:10px;margin:2px 0">左键拖动画 / 滚轮缩放 / 两指捏合缩放</div>' +
    '<div id="dr-canvas-wrap" style="position:relative;margin-top:8px;border:1px solid var(--phx-border,#334155);border-radius:8px;overflow:hidden;background:#0f1720;touch-action:none;height:40vh">' +
    '<canvas id="dr-canvas" style="display:block;width:100%;height:100%"></canvas>' +
    '<div id="dr-region-label" class="dim" style="position:absolute;top:6px;left:8px;font-size:10px"></div>' +
    '<div id="dr-sync-overlay" class="gone dr-syncbox"></div>' +
    '<div id="dr-sync-label" class="gone" style="position:absolute;top:10px;left:50%;transform:translateX(-50%);z-index:6;background:rgba(17,24,39,.82);color:#22c55e;font-size:12px;font-weight:600;padding:4px 10px;border-radius:6px;pointer-events:none;white-space:nowrap">同步中…</div>' +
    '</div>' +
    // ── 调色板（编辑器底部）──
    '<div id="dr-palette-section" style="margin-top:10px">' +
    '<label style="display:block;font-size:12px;margin-bottom:6px">调色板（选颜色 → 方块）</label>' +
    '<div id="dr-cats" class="dr-cats"></div>' +
    '<div id="dr-current" style="display:flex;align-items:center;gap:8px;margin-top:8px;padding:6px;background:#111827;border-radius:6px">' +
    '<div id="dr-current-swatch" style="width:26px;height:26px;border-radius:4px;background:#1e293b;border:1px solid #334155;flex-shrink:0;background-size:cover;background-position:center"></div>' +
    '<div style="font-size:11px;color:#94a3b8" id="dr-current-name">未选方块</div></div>' +
    '</div>' +
    '</div>' +
    '</div>';
}

// 初始化
function initDraw() {
  if (window.IN_FLOAT) { var c = document.getElementById('dr-create-section'); if (c) c.classList.add('gone'); }
  try { loadDrawPalette(); } catch (e) {}
  try { bindDrawCanvas(); } catch (e) {}
  window.addEventListener('resize', function () { if (drawSess && drawSess.grid) { try { drawCenter(); renderDrawGrid(); } catch (e) {} } });
  try { updateDrawButtons(); } catch (e) {}
  // 恢复会话（若 Go 有画布则回编辑器，否则创建表单）
  drawRestore();
}

// 从 Go 会话恢复画布视图（初始打开 + 缓存恢复共用）
function drawRestore() {
  try {
    A('GET', '/api/draw/state').then(function (r) {
      if (r && r.ok) { drawSess = r.state; showEditor(); setTimeout(function () { try { drawCenter(); renderDrawGrid(); } catch (e) {} }, 120); }
      else if (!window.IN_FLOAT) showCreate();
      else { var n = document.getElementById('dr-float-nosession'); if (n) n.classList.remove('gone'); }
      updateDrawButtons();
    });
  } catch (e) {}
}

// 子页缓存恢复钩子：画布位图不在缓存里，恢复后重新初始化（绑定画布 + 重渲染）
window.onSubPageRestored = function (key) { if (key === '画板') { try { initDraw(); } catch (e) {} } };

// 惰性加载调色板（保证弹层打开前已就绪）
function ensureDrawPalette(cb) {
  if (drawPaletteLoaded && Object.keys(drawPaletteMap).length) { if (cb) cb(); return; }
  A('GET', '/api/draw/palette').then(function (r) {
    if (r && r.ok && r.palette) {
      drawPaletteMap = {};
      r.palette.forEach(function (e) { drawPaletteMap[e.name] = { r: e.r, g: e.g, b: e.b, tex: e.tex || '', tc: (e.tc && e.tc.length === 3) ? { r: e.tc[0], g: e.tc[1], b: e.tc[2] } : null }; });
      drawPaletteLoaded = true;
    }
    if (cb) cb();
  }).catch(function () { if (cb) cb(); });
}
function loadDrawPalette() { ensureDrawPalette(renderDrawPalette); }

// 渲染颜色类别
function renderDrawPalette() {
  var el = document.getElementById('dr-cats');
  if (!el) return;
  el.innerHTML = '';
  var cats = drawCategories();
  var keys = Object.keys(cats).sort();
  keys.forEach(function (k) {
    var cat = cats[k];
    var dot = document.createElement('div');
    dot.className = 'dr-cat';
    dot.style.background = 'rgb(' + cat.c[0] + ',' + cat.c[1] + ',' + cat.c[2] + ')';
    dot.title = k + '（' + cat.blocks.length + ' 种方块）';
    dot.onclick = function () { showDrawCategoryBlocks(k, drawSelectBlock); };
    el.appendChild(dot);
  });
  drawPreloadTextures();
}

// 预加载画布实际用到的方块贴图 + 当前画笔（而非整个调色板），加载完批量重绘一次。
// 避免一次并发加载数百张贴图导致设备卡顿。
function drawPreloadTextures() {
  if (!drawSess || !drawSess.grid) return;
  var names = {};
  if (drawBrush) names[drawBrush] = 1;
  var w = drawSess.grid.length;
  for (var x = 0; x < w; x++) {
    var col = drawSess.grid[x];
    if (!col) continue;
    for (var z = 0; z < col.length; z++) {
      var cell = col[z];
      if (cell && cell.b) names[cell.b] = 1;
    }
  }
  var pending = 0;
  Object.keys(names).forEach(function (n) {
    var c = drawPaletteMap[n];
    if (c && c.tex && !drawTexCache[n]) {
      pending++;
      var img = new Image();
      drawTexCache[n] = img;
      img.onload = img.onerror = function () { if (--pending === 0 && drawSess && drawSess.grid) renderDrawGrid(); };
      img.src = '/blocks/' + c.tex + '.png';
    }
  });
}

// 按需加载单个方块的贴图（画笔更换 / 新放置方块时），加载后重绘一次以显示纹理。
function drawEnsureTexture(name) {
  var c = drawPaletteMap[name];
  if (!c || !c.tex || drawTexCache[name]) return;
  var img = new Image();
  drawTexCache[name] = img;
  img.onload = function () { if (drawSess && drawSess.grid) renderDrawGrid(); };
  img.src = '/blocks/' + c.tex + '.png';
}

// 底部弹层通用
function drawSheet(title, items, selected, cb) {
  var overlay = document.createElement('div'); overlay.className = 'bs-overlay';
  var itemsHtml = '';
  for (var i = 0; i < items.length; i++) {
    var it = items[i];
    var dotBg = it.color ? 'rgb(' + it.color[0] + ',' + it.color[1] + ',' + it.color[2] + ')' : '#444';
    if (it.tex) dotBg = 'linear-gradient(90deg, transparent 50%, rgb(' + it.color[0] + ',' + it.color[1] + ',' + it.color[2] + ') 50%), url(/blocks/' + it.tex + '.png) center/cover';
    itemsHtml += '<div class="bs-option' + (it.value === selected ? ' active' : '') + '" data-value="' + it.value + '">' +
      '<div class="bs-dot" style="background:' + dotBg + ';background-size:cover"></div>' +
      '<span class="bs-label">' + it.label + '</span></div>';
  }
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">' + title + '</div>' +
    '<div class="bs-body">' + itemsHtml + '</div>' +
    '<div class="bs-footer"><button class="bs-btn bs-btn-cancel">取消</button><button class="bs-btn bs-btn-confirm">确定</button></div></div>';
  document.body.appendChild(overlay);
  var selVal = selected;
  overlay.querySelectorAll('.bs-option').forEach(function (opt) {
    opt.addEventListener('click', function () {
      overlay.querySelectorAll('.bs-option').forEach(function (o) { o.classList.remove('active'); });
      this.classList.add('active'); selVal = this.getAttribute('data-value');
    });
  });
  overlay.querySelector('.bs-btn-cancel').addEventListener('click', function () { closeOverlay(overlay); });
  overlay.querySelector('.bs-btn-confirm').addEventListener('click', function () { closeOverlay(overlay); if (cb) cb(selVal); });
  overlay.addEventListener('click', function (e) { if (!e.target.closest('.bs-sheet')) closeOverlay(overlay); });
}

// 某颜色类别下的方块（底部弹层）
function showDrawCategoryBlocks(catK, cb) {
  var cats = drawCategories();
  var cat = cats[catK];
  if (!cat) return;
  var blocks = cat.blocks.slice().sort();
  var items = blocks.map(function (n) {
    var c = drawPaletteMap[n];
    return { value: n, label: n.replace('minecraft:', ''), color: [c.r, c.g, c.b], tex: c.tex };
  });
  items.push({ value: '__custom__', label: '自定义方块…', color: [100, 100, 100] });
  drawSheet('方块（' + catK + '）', items, null, function (v) {
    if (v === '__custom__') drawPromptCustom(cb);
    else if (cb) cb(v);
  });
}

// 自定义方块
function drawPromptCustom(cb) {
  showPrompt({ title: '自定义方块', placeholder: '如 minecraft:gold_block', onConfirm: function (raw) {
    var v = (raw || '').trim();
    if (!v) return;
    if (!drawPaletteMap[v]) {
      var h = 0; for (var i = 0; i < v.length; i++) h = (h * 31 + v.charCodeAt(i)) | 0;
      drawPaletteMap[v] = { r: 60 + (h & 127), g: 60 + ((h >> 8) & 127), b: 60 + ((h >> 16) & 127), tex: '' };
    }
    if (cb) cb(v);
  } });
}

function drawSelectBlock(name) {
  drawBrush = name; drawTool = 'brush';
  drawSetCurrentUI(name); drawUpdateToolButtons();
  drawEnsureTexture(name);
}
function drawSetCurrentUI(name) {
  var c = drawPaletteMap[name] || { r: 90, g: 90, b: 90, tex: '' };
  var sw = document.getElementById('dr-current-swatch');
  var nm = document.getElementById('dr-current-name');
  if (sw) sw.style.background = drawSplitStyle(c);
  if (nm) nm.textContent = name ? name.replace('minecraft:', '') : '未选方块';
}
function drawPickBrush() { ensureDrawPalette(function () { drawSheet('选择颜色类别', drawCategoryItems(), null, function (catK) { showDrawCategoryBlocks(catK, drawSelectBlock); }); }); }
function drawPickFill() {
  ensureDrawPalette(function () {
    drawSheet('选择颜色类别', drawCategoryItems(), null, function (catK) {
      showDrawCategoryBlocks(catK, function (v) {
        drawFill = v;
        var nm = document.getElementById('dr-fill-name');
        if (nm) nm.textContent = v.replace('minecraft:', '');
      });
    });
  });
}
function drawCategoryItems() {
  var cats = drawCategories();
  return Object.keys(cats).sort().map(function (k) { return { value: k, label: k, color: cats[k].c }; });
}
function drawPickCreateMode() {
  drawSheet('创建模式', [
    { value: 'transparent', label: '纯透明（不动原有方块）', color: [30, 41, 59] },
    { value: 'fill', label: '填充（整平面填一种方块）', color: [140, 140, 140] }
  ], drawCreateMode, function (v) {
    drawCreateMode = v;
    var btn = document.getElementById('dr-mode-btn');
    if (btn) btn.textContent = v === 'fill' ? '填充（整平面填一种方块）' : '纯透明（不动原有方块）';
    var fr = document.getElementById('dr-fill-row');
    if (fr) fr.classList.toggle('gone', v !== 'fill');
  });
}
function drawPickSync() {
  drawSheet('同步方式', [
    { value: 'realtime', label: '实时（每笔同步）', color: [74, 222, 128] },
    { value: 'batch', label: '批量（点应用再放）', color: [96, 165, 250] }
  ], drawSyncMode, function (v) {
    drawSyncMode = v;
    var btn = document.getElementById('dr-sync-btn');
    if (btn) btn.textContent = v === 'batch' ? '批量（点应用再放）' : '实时（每笔同步）';
  });
}

// ── 画布事件 ──
function bindDrawCanvas() {
  var cv = document.getElementById('dr-canvas');
  if (!cv) return;
  // 幂等：重复打开/恢复子页时避免重复绑定 pointer 事件（重复绑定会导致一笔画多次落块）
  if (cv.__drBound) return; cv.__drBound = true;
  cv.addEventListener('contextmenu', function (e) { e.preventDefault(); });
  cv.addEventListener('pointerdown', function (e) {
    if (!drawSess.grid || drawSess.placing || drawSess.busy || drawSyncing) return;
    e.preventDefault();
    cv.setPointerCapture(e.pointerId);
    drawPointer = true;
    var p = screenToGrid(e.offsetX, e.offsetY);
    drawLastPos = p;
    if (drawTool === 'pick') { drawPick(p); return; }
    drawLine(p, p);
  });
  cv.addEventListener('pointermove', function (e) {
    if (!drawPointer) return;
    var p = screenToGrid(e.offsetX, e.offsetY);
    if (drawTool === 'pick') return;
    if (drawLastPos) drawLine(drawLastPos, p);
    drawLastPos = p;
  });
  cv.addEventListener('pointerup', function () { if (!drawPointer) return; drawPointer = false; drawLastPos = null; drawFlushStroke(); });
  cv.addEventListener('pointercancel', function () { drawPointer = false; drawLastPos = null; drawFlushStroke(); });
  cv.addEventListener('wheel', function (e) { e.preventDefault(); drawZoomAt(e.offsetX, e.offsetY, e.deltaY < 0 ? 1.2 : 1 / 1.2); });
  var lastPinch = 0, lastMid = null;
  cv.addEventListener('touchstart', function (e) { if (e.touches.length === 2) { lastPinch = drawTouchDist(e); lastMid = drawTouchMid(e); drawPointer = false; drawCancelStroke(); e.preventDefault(); } }, { passive: false });
  cv.addEventListener('touchmove', function (e) {
    if (e.touches.length === 2) {
      var d = drawTouchDist(e), mid = drawTouchMid(e); e.preventDefault();
      if (lastPinch > 0 && lastMid) {
        // 双指移动 → 平移画布
        drawOffX += mid.x - lastMid.x; drawOffY += mid.y - lastMid.y;
        // 双指捏合 → 以双指中心缩放（非画布中心）
        if (d > 0 && d !== lastPinch) drawZoomAt(mid.x, mid.y, d / lastPinch);
        renderDrawGrid();
      }
      lastPinch = d; lastMid = mid;
    }
  }, { passive: false });
  cv.addEventListener('touchend', function () { lastPinch = 0; lastMid = null; });
}
function drawTouchDist(e) { var a = e.touches[0], b = e.touches[1]; return Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY); }
function drawTouchMid(e) { var c = document.getElementById('dr-canvas'); var r = c.getBoundingClientRect(); return { x: (e.touches[0].clientX + e.touches[1].clientX) / 2 - r.left, y: (e.touches[0].clientY + e.touches[1].clientY) / 2 - r.top }; }
function screenToGrid(sx, sy) { return { x: Math.floor((sx - drawOffX) / drawZoom), z: Math.floor((sy - drawOffY) / drawZoom) }; }
// 填充单个网格格子（加入当前笔画）
function fillStrokeCell(x, z) {
  var s = drawSess.grid;
  if (!s || x < 0 || z < 0 || x >= s.length || z >= s[0].length) return;
  var prev = (s[x][z] && s[x][z].b) || 'minecraft:air';
  var cell = drawTool === 'eraser' ? { b: 'minecraft:air' } : { b: drawBrush || 'minecraft:air' };
  s[x][z] = cell;
  drawEnsureTexture(cell.b);
  drawCurStroke.push({ x: x, z: z, block: cell.b, prev: prev });
}

// 撤销当前未提交的笔画（双指缩放/平移时手指落点不该画画，避免留下两个点）
function drawCancelStroke() {
  if (!drawCurStroke.length) return;
  var s = drawSess && drawSess.grid;
  if (s) {
    drawCurStroke.forEach(function (p) {
      if (s[p.x]) s[p.x][p.z] = { b: p.prev };
    });
  }
  drawCurStroke = [];
  renderDrawGrid();
}

// 线段插值（Bresenham）：快速拖动时不漏格子
function drawLine(p0, p1) {
  var x0 = p0.x, y0 = p0.z, x1 = p1.x, y1 = p1.z;
  var dx = Math.abs(x1 - x0), dy = Math.abs(y1 - y0);
  var sx = x0 < x1 ? 1 : -1, sy = y0 < y1 ? 1 : -1;
  var err = dx - dy;
  for (;;) {
    fillStrokeCell(x0, y0);
    if (x0 === x1 && y0 === y1) break;
    var e2 = 2 * err;
    if (e2 > -dy) { err -= dy; x0 += sx; }
    if (e2 < dx) { err += dx; y0 += sy; }
  }
  renderDrawGrid();
}
function drawFlushStroke() {
  if (!drawCurStroke.length) return;
  var pts = drawCurStroke; drawCurStroke = [];
  var map = {}; pts.forEach(function (p) { map[p.x + ',' + p.z] = p; });
  var uniq = Object.keys(map).map(function (k) { return map[k]; });
  A('POST', '/api/draw/stroke', { points: uniq }).then(function (r) { if (r && !r.ok && r.busy) T('机器人放置中，稍后再画', 'e'); });
}
function drawPick(p) {
  var s = drawSess.grid;
  if (!s || p.x < 0 || p.z < 0 || p.x >= s.length || p.z >= s[0].length) return;
  var cell = s[p.x][p.z];
  if (cell && cell.b && cell.b !== 'minecraft:air') { drawSelectBlock(cell.b); T('已取色: ' + cell.b.replace('minecraft:', ''), 'o'); }
}

// ── 视图 ──
function showCreate() {
  var c = document.getElementById('dr-create-section');
  var e = document.getElementById('dr-editor-section');
  if (c) c.classList.remove('gone');
  if (e) e.classList.add('gone');
  drawSess = { grid: null, region: null, placing: false, mode: 'realtime' };
  updateDrawButtons();
}
function showEditor() {
  var c = document.getElementById('dr-create-section');
  var e = document.getElementById('dr-editor-section');
  if (c) c.classList.add('gone');
  if (e) e.classList.remove('gone');
  var label = document.getElementById('dr-region-label');
  if (label && drawSess.region) label.textContent = '区域 X' + drawSess.region.x0 + ' Z' + drawSess.region.z0 + ' W' + drawSess.region.w + ' D' + drawSess.region.d + ' Y' + drawSess.region.y;
  updateDrawButtons();
}

// ── 工具/动作 ──
function drawSetTool(t) { drawTool = t; drawUpdateToolButtons(); }
function drawUpdateToolButtons() {
  var b = document.getElementById('dr-tool-brush'), e = document.getElementById('dr-tool-eraser'), p = document.getElementById('dr-tool-pick');
  if (b) b.style.opacity = drawTool === 'brush' ? 1 : 0.5;
  if (e) e.style.opacity = drawTool === 'eraser' ? 1 : 0.5;
  if (p) p.style.opacity = drawTool === 'pick' ? 1 : 0.5;
}
function drawUndo() { A('POST', '/api/draw/undo', {}).then(function (r) { if (r && r.ok) { drawSess = r.state; renderDrawGrid(); } }); }
function drawRedo() { A('POST', '/api/draw/redo', {}).then(function (r) { if (r && r.ok) { drawSess = r.state; renderDrawGrid(); } }); }
function drawApply() { A('POST', '/api/draw/apply', {}).then(function (r) { if (r && r.ok) { drawSess = r.state; renderDrawGrid(); } }); }
function drawSync() { A('POST', '/api/draw/sync', {}).then(function (r) { if (r && r.ok) T('正在读取世界平面…', 'o'); else if (r) T(r.error || '同步失败', 'e'); }); }
function drawCloseCanvas() {
  A('POST', '/api/draw/close', {}).then(function () {
    drawSess = { grid: null, region: null, placing: false, mode: 'realtime' };
    if (window.IN_FLOAT && window.floating && window.floating.closeDraw) window.floating.closeDraw();
    else showCreate();
  });
}
function drawToggleFloat() {
  if (window.IN_FLOAT) return;
  if (window.android && window.android.toggleDrawWindow) window.android.toggleDrawWindow();
  else T('悬浮窗不可用', 'e');
}
function drawHideFloatBtn() {
  var b = document.getElementById('dr-floatbtn'); if (b) b.style.display = 'none';
  var c = document.getElementById('dr-closecanvas'); if (c) c.style.display = 'none';
}

// ── 创建 ──
function drawCreate() {
  function num(id) { var v = parseFloat(document.getElementById(id).value); return isNaN(v) ? null : v; }
  var x1 = num('dr-x1'), z1 = num('dr-z1'), w = num('dr-w'), d = num('dr-d'), y = num('dr-y');
  if (x1 === null || z1 === null || w === null || d === null || y === null) { T('请完整填写起点 X1/Z1、X 边长 W、Z 边长 D、高度 Y', 'e'); return; }
  var body = { x1: x1, z1: z1, w: w, d: d, y: y, mode: drawCreateMode, sync: drawSyncMode };
  if (drawCreateMode === 'fill') {
    if (!drawFill) { T('请先选择填充方块', 'e'); return; }
    body.fill_block = drawFill;
  }
  A('POST', '/api/draw/create', body).then(function (r) {
    if (r && r.ok) { drawSess = r.state; showEditor(); setTimeout(function () { drawCenter(); renderDrawGrid(); }, 120); T('画布已创建', 'o'); }
    else if (r) T(r.error || '创建失败', 'e');
  });
}
function updateDrawButtons() {
  var u = document.getElementById('dr-undo'), r = document.getElementById('dr-redo');
  if (u) u.style.opacity = (drawSess && drawSess.can_undo) ? 1 : 0.4;
  if (r) r.style.opacity = (drawSess && drawSess.can_redo) ? 1 : 0.4;
}

// ── 渲染 ──
function drawCanvasSize() { var wrap = document.getElementById('dr-canvas-wrap'); return { w: (wrap && wrap.clientWidth) || 300, h: (wrap && wrap.clientHeight) || 300 }; }
function renderDrawGrid() {
  var cv = document.getElementById('dr-canvas');
  if (!cv || !drawSess.grid) return;
  var w = drawSess.grid.length; if (!w) return;
  var d = drawSess.grid[0] ? drawSess.grid[0].length : w;
  var size = drawCanvasSize(); var dpr = window.devicePixelRatio || 1;
  cv.width = size.w * dpr; cv.height = size.h * dpr;
  cv.style.width = size.w + 'px'; cv.style.height = size.h + 'px';
  var ctx = cv.getContext('2d'); ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.fillStyle = '#0f1720'; ctx.fillRect(0, 0, size.w, size.h);
  var px = drawZoom;
  var x0 = Math.max(0, Math.floor(-drawOffX / px)), x1 = Math.min(w, Math.ceil((size.w - drawOffX) / px));
  var z0 = Math.max(0, Math.floor(-drawOffY / px)), z1 = Math.min(d, Math.ceil((size.h - drawOffY) / px));
  for (var x = x0; x < x1; x++) {
    for (var z = z0; z < z1; z++) {
      var cell = drawSess.grid[x] && drawSess.grid[x][z];
      var cx = drawOffX + x * px, cy = drawOffY + z * px;
      if (cell && cell.t) { ctx.fillStyle = (x + z) % 2 === 0 ? '#1e293b' : '#0f1720'; ctx.fillRect(cx, cy, px + 0.5, px + 0.5); }
      else {
        var name = (cell && cell.b) || '';
        var tex = drawTexCache[name];
        if (tex && tex.complete && tex.naturalWidth > 0) {
          ctx.drawImage(tex, cx, cy, px + 0.5, px + 0.5);
        } else {
          var col = drawPaletteMap[name] || { r: 90, g: 90, b: 90 };
          ctx.fillStyle = 'rgb(' + col.r + ',' + col.g + ',' + col.b + ')'; ctx.fillRect(cx, cy, px + 0.5, px + 0.5);
        }
      }
    }
  }
  if (px >= 4) {
    ctx.strokeStyle = 'rgba(255,255,255,0.08)'; ctx.lineWidth = 1; ctx.beginPath();
    for (var i = x0; i <= x1; i++) { var lx = drawOffX + i * px; ctx.moveTo(lx, 0); ctx.lineTo(lx, size.h); }
    for (var j = z0; j <= z1; j++) { var ly = drawOffY + j * px; ctx.moveTo(0, ly); ctx.lineTo(size.w, ly); }
    ctx.stroke();
  }
}
function drawZoomAt(sx, sy, factor) {
  var gx = (sx - drawOffX) / drawZoom, gy = (sy - drawOffY) / drawZoom;
  drawZoom = Math.max(2, Math.min(80, drawZoom * factor));
  drawOffX = sx - gx * drawZoom; drawOffY = sy - gy * drawZoom; renderDrawGrid();
}
function drawCenter() {
  var s = drawSess.grid; if (!s) return;
  var w = s.length, d = s[0] ? s[0].length : w;
  var size = drawCanvasSize();
  drawZoom = Math.max(2, Math.min(size.w / w, size.h / d));
  drawOffX = (size.w - w * drawZoom) / 2;
  drawOffY = (size.h - d * drawZoom) / 2;
}

// ── SSE 回调 ──
function onDrawBusy(d) { if (drawSess) drawSess.busy = d.busy; var el = document.getElementById('dr-busy'); if (el) el.classList.toggle('gone', !d.busy); updateDrawButtons(); }
function onDrawUpdate(d) {
  if (!drawSess || !drawSess.grid) return;
  if (d.state) drawSess = Object.assign(drawSess, d.state);
  if (d.changes) d.changes.forEach(function (ch) { if (drawSess.grid[ch.x]) drawSess.grid[ch.x][ch.z] = ch.next; });
  renderDrawGrid(); updateDrawButtons();
}
function onDrawPlane(d) {
  if (!d || !d.grid || !d.region) return;
  drawSess = { grid: d.grid, region: d.region, can_undo: true, can_redo: false, placing: false, busy: false, mode: 'realtime' };
  showEditor(); drawCenter(); renderDrawGrid(); T('世界平面已同步', 'o');
}
function onDrawError(d) { if (d && d.error) T(d.error, 'e'); }

// 流式同步：动画标记 + 绿框闪烁 + 禁用同步按钮与绘画 + 逐块应用
function onDrawSync(d) {
  drawSyncing = !!d.active;
  var ov = document.getElementById('dr-sync-overlay');
  var lb = document.getElementById('dr-sync-label');
  if (ov) ov.classList.toggle('gone', !d.active);
  if (lb) {
    lb.classList.toggle('gone', !d.active);
    lb.textContent = (d.active && d.total) ? ('同步中 ' + (d.current || 0) + '/' + d.total) : '同步中…';
  }
  var sb = document.getElementById('dr-sync');
  if (sb) { sb.disabled = d.active; sb.style.opacity = d.active ? 0.4 : 1; }
  if (d.cells && drawSess && drawSess.grid) {
    d.cells.forEach(function (c) { if (drawSess.grid[c.x]) drawSess.grid[c.x][c.z] = c.next; });
    renderDrawGrid();
  }
  if (!d.active) T('世界平面已同步', 'o');
}
