// ═══════════════════════════════════════
// 其他功能 — 子页面卡片网格
// ═══════════════════════════════════════

var yyToolsCards = [
    {i: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 7h16l-3-3m3 3-3 3M20 17H4l3-3m-3 3 3 3"/></svg>', t:'区域迁移', d:'来源服导出 · 切服后导入', cat:'建筑与世界', act:function(){openSubPage('区域迁移',RmgrtLiteHTML())}},
    {i: '<svg width="26" height="26" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="18" height="18" rx="2"/><line x1="2" y1="7" x2="20" y2="7"/><line x1="2" y1="12" x2="20" y2="12"/><line x1="2" y1="17" x2="20" y2="17"/><line x1="7" y1="2" x2="7" y2="20"/><line x1="12" y1="2" x2="12" y2="20"/><line x1="17" y1="2" x2="17" y2="20"/></svg>', t: '滚动字幕', d: 'ASCII 大字滚动', cat:'图形与媒体', act: function(){openSubPage('滚动字幕', RmarqueeHTML())}},
    {i: '<svg width="26" height="26" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>', t: 'mcfunction 执行器', d: '命令文件执行 / 语法高亮', cat:'命令与开发', act: function(){openSubPage('mcfunction 执行器', RmcfunctionHTML())}},
    {i: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/></svg>', t: '相机工具', d: 'camera 指令生成 / 镜头录制', cat:'图形与媒体', act: function(){openSubPage('相机工具', RcameraHTML())}},
    {i: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 14l-5-5-5 5"/><path d="M17 20l-5-5-5 5"/><line x1="12" y1="2" x2="12" y2="9"/></svg>', t: '飞行控制', d: '机器人移动 / 视角 / 传送', cat:'自动化与机器人', act: function(){delete subPageCache['飞行控制'];openSubPage('飞行控制', RflyHTML())}},
    {i: '<svg width="26" height="26" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>', t: '物品制作器', d: '木桶自定义物品 / 换行 / 颜色', cat:'命令与开发', act: function(){delete subPageCache['物品制作器'];openSubPage('物品制作器', RitemmakerHTML())}},
    {i: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>', t: '命令方块工程', d: '命令区导入 / 导出', cat:'命令与开发', act: function(){delete subPageCache['命令方块工程'];openSubPage('命令方块工程', RcbHTML())}},
    {i: '<svg width="26" height="26" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 3h13a1 1 0 0 1 1 1v13"/><path d="M4 3v18"/><rect x="4" y="6" width="9" height="12" rx="1"/></svg>', t: '画板', d: '像素画绘制 · 实时同步', cat:'图形与媒体', act: function(){openSubPage('画板', RdrawHTML()); setTimeout(function(){try{initDraw()}catch(e){}}, 30)}},
    {i: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 21h18"/><path d="M5 21V7l7-4 7 4v14"/><path d="M9 21v-6h6v6"/><circle cx="12" cy="10" r="1.6"/></svg>', t: '总部查询器', d: '厅服房间聚合 · 本机计算', cat:'其他实用工具', act: function(){openSubPage('总部查询器', RhqQueryHTML())}},
    // 种子查询器：群系地图 + 结构定位 + 全维度。Canvas 本机渲染，见 features/seed_map.js。
    {i: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21s-7-6.2-7-11a7 7 0 0 1 14 0c0 4.8-7 11-7 11z"/><circle cx="12" cy="10" r="2.6"/><path d="M2 21h20"/></svg>', t: '种子查询器', d: '群系地图 · 结构定位 · 全维度', cat:'建筑与世界', act: function(){delete subPageCache['种子查询器'];openSubPage('种子查询器', RseedMapHTML());setTimeout(function(){try{ysmInit()}catch(e){}},30)}},
      ];

// 分类顺序固定，空分类不渲染——「资源与文件」暂时没有工具，就不会出现光秃秃一个标题。
var yyToolsCats = ['建筑与世界', '资源与文件', '自动化与机器人', '图形与媒体', '命令与开发', '其他实用工具'];
var yyToolsRecentKey = 'yy_tools_recent_v1';

function yyToolsRecentRead() {
  try {
    var arr = JSON.parse(localStorage.getItem(yyToolsRecentKey) || '[]');
    if (!Array.isArray(arr)) return [];
    return arr.filter(function (x) { return typeof x === 'string'; }).slice(0, 4);
  } catch (e) { return []; }
}
function yyToolsRecentNote(title) {
  try {
    var arr = yyToolsRecentRead().filter(function (x) { return x !== title; });
    arr.unshift(title);
    localStorage.setItem(yyToolsRecentKey, JSON.stringify(arr.slice(0, 4)));
  } catch (e) {}
}
function yyToolsIndexByTitle(title) {
  for (var i = 0; i < yyToolsCards.length; i++) if (yyToolsCards[i].t === title) return i;
  return -1;
}
// 一行一个工具：图标（20px 以内）+ 名称（主信息）+ 描述（次要）+ 箭头。
// 不用卡片包裹，靠行距和细分隔线分组——卡片只在「整体可点且是独立对象」时才用。
function yyToolsRowHTML(card, index) {
  return '<button type="button" class="yyt-row" onclick="yyToolsRun(' + index + ')">' +
    '<span class="yyt-ico" aria-hidden="true">' + card.i + '</span>' +
    '<span class="yyt-txt"><strong class="yyt-name">' + card.t + '</strong>' +
    '<small class="yyt-desc">' + card.d + '</small></span>' +
    '<span class="yyt-go" aria-hidden="true">\u203A</span></button>';
}
function yyToolsListHTML(filter) {
  var needle = String(filter || '').trim().toLowerCase();
  var hit = function (c) {
    return !needle || (c.t + ' ' + c.d + ' ' + c.cat).toLowerCase().indexOf(needle) >= 0;
  };
  var out = '', total = 0;
  for (var c = 0; c < yyToolsCats.length; c++) {
    var rows = '';
    for (var i = 0; i < yyToolsCards.length; i++) {
      if (yyToolsCards[i].cat !== yyToolsCats[c] || !hit(yyToolsCards[i])) continue;
      rows += yyToolsRowHTML(yyToolsCards[i], i);
      total++;
    }
    if (rows) out += '<section class="yyt-cat"><h3 class="yyt-cat-title">' + yyToolsCats[c] + '</h3>' + rows + '</section>';
  }
  if (!total) out = '<p class="yyt-empty">没有匹配「' + String(filter || '').replace(/[<>&]/g, '') + '」的工具。</p>';
  return out;
}
function yyToolsRecentHTML() {
  var titles = yyToolsRecentRead(), rows = '';
  for (var i = 0; i < titles.length; i++) {
    var idx = yyToolsIndexByTitle(titles[i]);
    if (idx >= 0) rows += yyToolsRowHTML(yyToolsCards[idx], idx);
  }
  return rows ? '<section class="yyt-cat yyt-recent"><h3 class="yyt-cat-title">最近使用</h3>' + rows + '</section>' : '';
}
function yyToolsSearch(value) {
  var body = document.getElementById('yyt-body');
  if (body) body.innerHTML = yyToolsListHTML(value);
  var recent = document.getElementById('yyt-recent');
  if (recent) recent.innerHTML = String(value || '').trim() ? '' : yyToolsRecentHTML();
}
function yyToolsRun(index) {
  var card = yyToolsCards[index];
  if (!card) return;
  yyToolsRecentNote(card.t);
  try { card.act(); } catch (e) { if (typeof T === 'function') T('打开失败：' + e.message, 'e'); }
}
function RothersHTML() {
  // 首屏直接把列表灌进字符串：openSubPage 之后不再等一次 setTimeout，
  // 免得用户看到一瞬空白。搜索只重绘 #yyt-body。
  return '<div class="yyt">' +
    '<input type="search" id="yyt-search" class="yyt-search" placeholder="搜索工具" aria-label="搜索工具" oninput="yyToolsSearch(this.value)">' +
    '<div id="yyt-recent">' + yyToolsRecentHTML() + '</div>' +
    '<div id="yyt-body">' + yyToolsListHTML('') + '</div>' +
    '</div>';
}

// 「更多工具」里每个工具都是"骨架 HTML + 渲染函数"两段式：RxxxHTML() 只返回容器，
// 真正的步骤内容靠 renderCamStep / cbRenderTab 这类函数填。从缓存或"返回上一页"
// 恢复时这些函数不会被调用，页面就只剩一个空壳。这里把 8 个工具的初始化登记到
// app.js 的 subPageInits 表里，缓存恢复时会自动补跑。
// 用 setTimeout(0) 是因为 app.js 在 others.js 之后才加载，那时 subPageInit 才存在。
setTimeout(function() {
  if (typeof subPageInit !== 'function') return;
  subPageInit('相机工具', function() { renderCamStep(camState.step || 1); });
  subPageInit('mcfunction 执行器', function() { renderMcfunctionStep(mcfnState.step || 1); });
  subPageInit('物品制作器', function() { imFetchPos(); imRenderItems(); });
  subPageInit('命令方块工程', function() { cbRenderTab(); });
  subPageInit('画板', function() { initDraw(); });
  subPageInit('滚动字幕', function() { mqRefreshPreview(); });
  subPageInit('飞行控制', function() {
    if (flyState.polling) { clearInterval(flyState.polling); flyState.polling = null; }
    flyActiveDirs = {};
    flyPollStatus();
    flyInitCanvases();
    flyState.polling = setInterval(flyPollStatus, 500);
  });
}, 0);

// ═══════════════════════════════════════
// 滚动字幕 — 控制面板
// ═══════════════════════════════════════

var mcColorMap = {};
var rainbowColorIds = ['c', '6', 'e', 'a', 'b', 'd', '5'];

var mcColors = [
  {id: '0', label: '黑', color: '#000000'},
  {id: '1', label: '深蓝', color: '#0000AA'},
  {id: '2', label: '深绿', color: '#00AA00'},
  {id: '3', label: '深青', color: '#00AAAA'},
  {id: '4', label: '深红', color: '#AA0000'},
  {id: '5', label: '深紫', color: '#AA00AA'},
  {id: '6', label: '金', color: '#FFAA00'},
  {id: '7', label: '灰', color: '#AAAAAA'},
  {id: '8', label: '深灰', color: '#555555'},
  {id: '9', label: '蓝', color: '#5555FF'},
  {id: 'a', label: '绿', color: '#55FF55'},
  {id: 'b', label: '青', color: '#55FFFF'},
  {id: 'c', label: '红', color: '#FF5555'},
  {id: 'd', label: '粉', color: '#FF55FF'},
  {id: 'e', label: '黄', color: '#FFFF55'},
  {id: 'f', label: '白', color: '#FFFFFF'},
];

(function() {
  for (var i = 0; i < mcColors.length; i++) {
    mcColorMap[mcColors[i].id] = mcColors[i].color;
  }
})();

// ── 预览动画 ──

var mqPreviewTimer = null;
var mqAnimTimer = null;
var mqAnimRows = [];
var mqAnimFrame = 0;
var mqCellWidth = 2;   // 每格占几个空格宽（后端返回，用于预览对齐）

// 字符在 Minecraft 中相对空格的宽度（与后端 mcWidth 对应，方块类=2）
function mcWidthJs(s) {
  var ch = (s && s.charAt) ? s.charAt(0) : ' ';
  if ('█■▀▄▌▐░▒▓▬▪◼◆●▲▼▮▚▞▖▗▘▝'.indexOf(ch) >= 0) return 2;
  return 1;
}

function RmarqueeHTML() {
  setTimeout(function() {
    mqRefreshPreview();
    ['mq-text','mq-tchar','mq-fchar','mq-width'].forEach(function(id) {
      var el = document.getElementById(id);
      if (el) el.addEventListener('input', mqDebouncePreview);
    });
  }, 100);
  return '<div class="card">' +
    '<label>文字内容</label>' +
    '<input id="mq-text" type="text" value="HELLO" placeholder="输入要显示的文字" maxlength="30">' +

    '<div class="row">' +
      '<div><label>文字像素</label><input id="mq-tchar" type="text" value="■" maxlength="1" style="text-align:center;font-size:18px"></div>' +
      '<div style="flex:1.5"><label>背景填充</label><input id="mq-fchar" type="text" value=" " placeholder="任意长度" style="text-align:center;font-size:14px"></div>' +
      '<div style="flex:0.7"><label>宽度(列)</label><input id="mq-width" type="number" value="40" min="10" max="80"></div>' +
    '</div>' +

    '<label>颜色</label>' +
    '<div id="mq-colors" style="display:flex;gap:6px;flex-wrap:wrap">' +
      mcColors.map(function(c) {
        var sel = c.id === 'a' ? ' data-selected="true" style="outline:3px solid var(--phx-primary);outline-offset:2px"' : '';
        return '<span class="mq-cb" data-color="' + c.id + '"' + sel + ' onclick="pickMarqueeColor(this);mqDebouncePreview()" style="display:inline-block;width:32px;height:32px;border-radius:8px;background:' + c.color + ';cursor:pointer;border:2px solid rgba(0,0,0,.15)"></span>';
      }).join('') +
    '</div>' +

    '<label>滚动速度 <span id="mq-speed-lbl" style="color:var(--phx-primary)">200ms</span></label>' +
    '<input id="mq-speed" type="range" min="30" max="500" value="200" step="10" oninput="document.getElementById(\'mq-speed-lbl\').textContent=this.value+\'ms\';mqDebouncePreview()" style="width:100%">' +

    '<div class="row" style="margin-top:10px;align-items:center">' +
      '<label class="lbl-cb" style="margin:0;flex:1"><input type="checkbox" id="mq-rainbow" onchange="toggleMarqueeRainbow();mqDebouncePreview()"><span class="tgl"></span>彩虹渐变</label>' +
      '<label class="lbl-cb" style="margin:0;flex:1"><input type="checkbox" id="mq-right" onchange="mqDebouncePreview()"><span class="tgl"></span>向右滚动</label>' +
    '</div>' +

    '<div id="mq-preview" style="margin-top:10px;padding:12px;border-radius:10px;background:#0D1B1A;font-family:monospace;font-size:7px;line-height:1.5;overflow:hidden;min-height:76px;white-space:pre"></div>' +

    '<div id="mq-status" style="display:none;margin-top:10px;padding:10px 14px;border-radius:12px;background:var(--phx-success-bg);color:var(--phx-success);font-size:12px;font-weight:700;text-align:center"><span class="spin"></span> 滚动中...</div>' +

    '<div class="row" style="margin-top:12px">' +
      '<button class="btn" id="mq-go" onclick="doMarqueeStart()">启动滚动</button>' +
      '<button class="btn-d gone" id="mq-stop" onclick="doMarqueeStop()">停止</button>' +
    '</div>' +
  '</div>';
}

function mqDebouncePreview() {
  if (mqPreviewTimer) clearTimeout(mqPreviewTimer);
  mqPreviewTimer = setTimeout(mqRefreshPreview, 200);
}

function mqRefreshPreview() {
  var text = document.getElementById('mq-text');
  var tc = document.getElementById('mq-tchar');
  var fc = document.getElementById('mq-fchar');
  if (!text || !tc || !fc) return;
  var t = text.value.trim();
  if (!t) {
    if (mqAnimTimer) { clearInterval(mqAnimTimer); mqAnimTimer = null; }
    document.getElementById('mq-preview').innerHTML = '<span style="color:#64748b">输入文字预览</span>';
    return;
  }
  fetch('/api/marquee/preview?text=' + encodeURIComponent(t) + '&text_char=' + encodeURIComponent(tc.value || '■') + '&fill_char=' + encodeURIComponent(fc.value || ' '))
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.ok && data.rows) {
        mqAnimRows = data.rows;
        mqCellWidth = data.cell_width || 2;
        mqAnimFrame = 0;
        if (mqAnimTimer) { clearInterval(mqAnimTimer); }
        mqRenderPreview();
        var speed = parseInt(document.getElementById('mq-speed').value) || 200;
        mqAnimTimer = setInterval(mqRenderPreview, speed);
      }
    });
}

function mqRenderPreview() {
  if (!mqAnimRows.length) return;
  var el = document.getElementById('mq-preview');
  if (!el) return;
  var canvasWidth = mqAnimRows[0].length;
  if (canvasWidth <= 0) return;
  var isRainbow = document.getElementById('mq-rainbow').checked;
  var color = getMarqueeColor();
  var width = parseInt(document.getElementById('mq-width').value) || 40;
  var right = document.getElementById('mq-right').checked;

  var fc = document.getElementById('mq-fchar').value || ' ';
  var target = mqCellWidth || 2;
  // 填充格补空格到与文字格等宽（与后端一致）
  var fillCell = fc + Array(Math.max(0, target - mcWidthJs(fc)) + 1).join(' ');

  var maxFrame = canvasWidth + width;
  var pos = mqAnimFrame % maxFrame;
  if (right) pos = (maxFrame - mqAnimFrame % maxFrame) % maxFrame;
  mqAnimFrame++;

  var html = '';
  for (var r = 0; r < mqAnimRows.length; r++) {
    // 前后填充格 + 文字格子，再取窗口 width 个格子
    var lineCells = [];
    for (var p = 0; p < width; p++) lineCells.push(fillCell);
    lineCells = lineCells.concat(mqAnimRows[r]);
    for (var p2 = 0; p2 < width; p2++) lineCells.push(fillCell);

    var windowed = '';
    for (var k = 0; k < width; k++) windowed += lineCells[pos + k];
    while (windowed.length < width) windowed += fillCell;
    windowed = windowed.substr(0, width * fillCell.length);

    if (isRainbow) {
      for (var ci = 0; ci < windowed.length; ci++) {
        var ch = windowed[ci];
        var ri = (ci + mqAnimFrame) % rainbowColorIds.length;
        var rc = mcColorMap[rainbowColorIds[ri]] || '#55FF55';
        if (ch === ' ') {
          html += '<span style="color:' + rc + '">&nbsp;</span>';
        } else {
          html += '<span style="color:' + rc + '">' + escHtml(ch) + '</span>';
        }
      }
    } else {
      var c = mcColorMap[color] || '#55FF55';
      html += '<span style="color:' + c + '">';
      for (var ci2 = 0; ci2 < windowed.length; ci2++) {
        var ch2 = windowed[ci2];
        html += ch2 === ' ' ? '&nbsp;' : escHtml(ch2);
      }
      html += '</span>';
    }
    if (r < mqAnimRows.length - 1) html += '<br>';
  }
  el.innerHTML = html;
}

function pickMarqueeColor(el) {
  document.querySelectorAll('.mq-cb').forEach(function(x) {
    x.style.outline = 'none';
    x.removeAttribute('data-selected');
  });
  el.style.outline = '3px solid var(--phx-primary)';
  el.style.outlineOffset = '2px';
  el.setAttribute('data-selected', 'true');
}

function toggleMarqueeRainbow() {
  var rb = document.getElementById('mq-rainbow').checked;
  document.querySelectorAll('.mq-cb').forEach(function(x) {
    x.style.opacity = rb ? '0.4' : '1';
    x.style.pointerEvents = rb ? 'none' : 'auto';
  });
}

function getMarqueeColor() {
  var sel = document.querySelector('.mq-cb[data-selected="true"]');
  return sel ? sel.getAttribute('data-color') : 'a';
}

async function doMarqueeStart() {
  var text = document.getElementById('mq-text').value.trim();
  if (!text) { T('请输入文字内容', 'e'); return; }

  var st = await A('GET', '/api/bot/status');
  if (!st.ok || !st.connected) {
    T('未连接到服务器，请先连接机器人', 'e');
    return;
  }

  var config = {
    text: text,
    text_char: document.getElementById('mq-tchar').value || '■',
    fill_char: document.getElementById('mq-fchar').value || ' ',
    width: parseInt(document.getElementById('mq-width').value) || 40,
    speed: parseInt(document.getElementById('mq-speed').value) || 200,
    color: document.getElementById('mq-rainbow').checked ? 'f' : getMarqueeColor(),
    direction: document.getElementById('mq-right').checked ? 'right' : 'left',
    rainbow: document.getElementById('mq-rainbow').checked,
  };

  var r = await A('POST', '/api/marquee/start', config);
  if (r.ok) {
    T('滚动字幕已启动', 'o');
    document.getElementById('mq-go').classList.add('gone');
    document.getElementById('mq-stop').classList.remove('gone');
    document.getElementById('mq-status').style.display = 'block';
  } else {
    T('启动失败: ' + (r.error || '未知错误'), 'e');
  }
}

async function doMarqueeStop() {
  var r = await A('POST', '/api/marquee/stop', {});
  if (r.ok) {
    T('已停止', 'o');
    document.getElementById('mq-go').classList.remove('gone');
    document.getElementById('mq-stop').classList.add('gone');
    document.getElementById('mq-status').style.display = 'none';
  } else {
    T('停止失败: ' + (r.error || '未知错误'), 'e');
  }
}

function restoreMarqueeState() {
  A('GET', '/api/marquee/status').then(function(r) {
    if (r.ok && r.running) {
      var go = document.getElementById('mq-go');
      var stop = document.getElementById('mq-stop');
      var st = document.getElementById('mq-status');
      if (go) go.classList.add('gone');
      if (stop) stop.classList.remove('gone');
      if (st) st.style.display = 'block';
    }
  });
}

// ═══════════════════════════════════════
// 调试面板 — 坐标 / 状态
// ═══════════════════════════════════════

var dbgPollTimer = null;

function RdebugHTML() {
  setTimeout(function() {
    dbgPollStatus();
    if (dbgPollTimer) clearInterval(dbgPollTimer);
    dbgPollTimer = setInterval(dbgPollStatus, 1000);
  }, 100);
  return '<div class="card">' +
    '<div style="font-size:14px;font-weight:700;margin-bottom:10px">🔧 调试面板</div>' +

    // 状态信息
    '<div id="dbg-status" style="padding:10px;border-radius:10px;background:var(--phx-bg);border:1px solid var(--phx-border-light);font-size:12px;font-family:monospace;line-height:1.8;margin-bottom:10px">' +
      '加载中...' +
    '</div>' +

    // 调试日志
    '<label style="margin-top:10px">调试日志</label>' +
    '<div id="dbg-log" style="margin-top:4px;padding:8px;border-radius:8px;background:#0D1B1A;color:#a8e6cf;font-size:10px;font-family:monospace;line-height:1.5;max-height:300px;overflow-y:auto;white-space:pre-wrap;word-break:break-all"></div>' +
    '<div class="row" style="margin-top:6px;gap:6px">' +
      '<button class="btn" onclick="dbgRefreshLog()" style="flex:1;margin:0;font-size:11px">刷新日志</button>' +
      '<button class="btn-d" onclick="document.getElementById(\'dbg-log\').textContent=\'\'" style="flex:1;margin:0;font-size:11px">清空</button>' +
    '</div>' +
  '</div>';
}

function dbgCleanup() {
  if (dbgPollTimer) { clearInterval(dbgPollTimer); dbgPollTimer = null; }
}

function dbgPollStatus() {
  A('GET', '/api/fly/status').then(function(r) {
    var el = document.getElementById('dbg-status');
    if (!el) return;
    if (r.ok) {
      el.innerHTML =
        '<div style="display:flex;justify-content:space-between">' +
          '<span>坐标: <b style="color:var(--phx-primary)">' + r.x.toFixed(1) + ' / ' + r.y.toFixed(1) + ' / ' + r.z.toFixed(1) + '</b></span>' +
          '<span>状态: ' + (r.isFlying ? '🟦 飞行' : '🟫 地面') + '</span>' +
        '</div>' +
        '<div style="display:flex;justify-content:space-between;margin-top:4px">' +
          '<span>视角: <b>' + Math.round(r.pitch) + '° / ' + Math.round(r.yaw) + '°</b></span>' +
          '<span>区块: ' + (r.world_chunks || 0) + '个</span>' +
        '</div>' +
        '<div style="display:flex;justify-content:space-between;margin-top:4px">' +
          '<span>维度: ' + (r.dimension === 0 ? '主世界' : r.dimension === 1 ? '下界' : '末地') + '</span>' +
          '<span>冲刺: ' + (r.sprinting ? '🟢' : '⚪') + '</span>' +
        '</div>';
    } else {
      el.innerHTML = '<span style="color:var(--phx-error)">未连接</span>';
    }
  });
}











async function dbgRefreshLog() {
  var el = document.getElementById('dbg-log');
  if (!el) return;
  el.textContent = '加载中...';
  try {
    var r = await fetch('/api/pathfinder/log');
    var text = await r.text();
    el.textContent = text || '(无日志)';
    el.scrollTop = el.scrollHeight;
  } catch(e) {
    el.textContent = '(获取日志失败)';
  }
}

