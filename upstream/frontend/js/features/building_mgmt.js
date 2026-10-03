// ── 建筑文件管理: 三个子板块 ──
// 文件市场 / 批量建筑预览 / 贡献文件

// 代理调用 建筑市场 API(透传 multipart/二进制/JSON + 鉴权)
async function AMarket(m, p, b, headers) {
  var c = new AbortController(), t = setTimeout(function () { c.abort(); }, 120000);
  try {
    var o = { method: m, headers: {}, signal: c.signal };
    if (b instanceof FormData) { o.body = b; }
    else if (b) { o.headers['Content-Type'] = 'application/json'; o.body = JSON.stringify(b); }
    if (headers) Object.assign(o.headers, headers);
    var r = await fetch('/api/market-proxy' + p, o); clearTimeout(t); return r.json();
  } catch (e) { clearTimeout(t); return { ok: false, error: e.name === 'AbortError' ? '超时' : e.message }; }
}

function bmBody() { return document.querySelector('#sub-page .sp-body'); }
// 渲染到子页 body,并同步更新 subPageCache,避免返回时恢复成"加载中"旧内容
function bmRender(html) {
  var b = bmBody(); if (b) b.innerHTML = html;
  var titleEl = document.querySelector('.sp-title');
  if (titleEl) {
    var key = titleEl.textContent;
    var sp = document.getElementById('sub-page');
    var header = sp.querySelector('.sp-header');
    if (header) subPageCache[key] = header.outerHTML + '<div class="sp-body">' + html + '</div>';
  }
}
// 内页作为独立子页推入 subPageStack,用标准 .sp-header 返回上一级;每次强制刷新缓存
function bmOpenSub(title, loadingHtml) {
  delete subPageCache[title];
  openSubPage(title, loadingHtml || '<div style="text-align:center;padding:30px"><span class="spin"></span> 加载中...</div>');
}
function bmCard(icon, title, sub, onclick) {
  return '<div class="grid-card" onclick="' + onclick + '"><div class="gc-icon">' + icon + '</div><div class="gc-title">' + title + '</div><div class="gc-sub">' + sub + '</div></div>';
}

// 菜单:三张子卡片
function RbuildingMgmtHTML() {
  return '<div class="content-inner" style="padding:0">' +
    '<div class="grid-2" style="margin-top:4px">' +
    bmCard(ICONS.globe, '文件市场', '浏览 · 下载 · 付费', 'bmOpenMarket()') +
    bmCard(ICONS.file, '批量建筑预览', '多角度预览图 · GIF', 'bmOpenBatch()') +
    bmCard(ICONS.upload, '贡献文件', '上传建筑到市场', 'bmOpenContribute()') +
    '</div></div>';
}

// ── 文件市场(无限滚动 + 搜索 + 分页) ──
var bmMkt = { page: 0, size: 20, total: 0, kw: '', cat: '', cats: [], list: [], loading: false, done: false, timer: 0, mine: false, showMine: false, scrollY: 0, sort: '' };

// 从详情返回(或重新打开)市场列表时:恢复滚动位置 + 重新挂载无限滚动哨兵
// (文件市场列表子页标题含"文件市场"及各分类名,故用通配并靠 #bm-mkt-q 判定当前页)
window.__subPageRestoreHooks = window.__subPageRestoreHooks || [];
window.__subPageRestoreHooks.push({ key: '*', fn: function () {
  if (!document.getElementById('bm-mkt-q')) return;   // 当前页不是文件市场列表
  var b = bmBody(); if (!b) return;
  if (bmMkt.scrollY) b.scrollTop = bmMkt.scrollY;
  bmMktMountSort();
  bmMktWatchScroll();
} });

function bmMktUrl() {
  var base = bmMkt.mine ? '/api/market/my' : '/api/market/files';
  var u = base + '?page=' + bmMkt.page + '&page_size=' + bmMkt.size;
  if (bmMkt.cat) u += '&category=' + encodeURIComponent(bmMkt.cat);
  if (bmMkt.kw) u += '&q=' + encodeURIComponent(bmMkt.kw);
  if (!bmMkt.mine && bmMkt.sort) u += '&order=' + bmMkt.sort;
  return u;
}

// 探测"我的已上传"是否可用(未登录/401 时不显示该 chip)
async function bmProbeMine() {
  try {
    var r = await AMarket('GET', '/api/market/my');
    return !!r.ok;
  } catch (e) { return false; }
}

async function bmOpenMarket() {
  bmOpenSub('文件市场');
  var catsR = await AMarket('GET', '/api/market/categories');
  bmMkt.cats = catsR.ok ? catsR.categories : [];
  bmMkt.cat = ''; bmMkt.kw = ''; bmMkt.mine = false; bmMkt.sort = '';
  bmMkt.showMine = await bmProbeMine();
  await bmMarketLoad(true);
}
async function bmOpenMarketCat(cat) {
  bmOpenSub(cat);
  var catsR = await AMarket('GET', '/api/market/categories');
  bmMkt.cats = catsR.ok ? catsR.categories : [];
  bmMkt.cat = cat; bmMkt.kw = ''; bmMkt.mine = false;
  await bmMarketLoad(true);
}
async function bmMarketLoad(reset) {
  if (bmMkt.loading) return;
  if (reset) { bmMkt.page = 0; bmMkt.list = []; bmMkt.done = false; bmMkt.scrollY = 0; bmMktRender(); }
  if (bmMkt.done) { bmMktRender(); return; }
  bmMkt.loading = true;
  bmMkt.page++;
  var r = await AMarket('GET', bmMktUrl());
  bmMkt.loading = false;
  if (!r.ok) { T('加载失败: ' + (r.error || ''), 'e'); return; }
  var arr = r.list || r.files || [];
  bmMkt.total = r.total || 0;
  bmMkt.list = bmMkt.list.concat(arr);
  if (!arr.length || arr.length < bmMkt.size) bmMkt.done = true;
  bmMktRender();
}
function bmMktChips() {
  var h = '<div style="display:flex;gap:6px;overflow-x:auto;padding-bottom:8px;margin-bottom:8px">';
  // "全部" = 清空分类筛选,回到全部文件
  h += '<span class="bm-cat' + (!bmMkt.cat && !bmMkt.mine ? ' bm-cat-on' : '') + '" onclick="bmMktAll()" style="flex-shrink:0">全部</span>';
  if (bmMkt.showMine) {
    h += '<span class="bm-cat' + (bmMkt.mine ? ' bm-cat-on' : '') + '" onclick="bmMktMine()" style="flex-shrink:0">已上传文件</span>';
  }
  bmMkt.cats.forEach(function (c) {
    var on = !bmMkt.mine && bmMkt.cat === c.name;
    h += '<span class="bm-cat' + (on ? ' bm-cat-on' : '') + '" onclick="bmOpenMarketCat(\'' + c.name.replace(/'/g, "\\'") + '\')" style="flex-shrink:0">' + escHtml(c.name) + '</span>';
  });
  h += '</div>';
  return h;
}
function bmMktInput() {
  return '<div style="display:flex;gap:6px;margin-bottom:8px;align-items:center">' +
    '<div style="position:relative;flex:1">' +
    '<input id="bm-mkt-q" class="input" style="width:100%;padding-right:30px" placeholder="搜索建筑标题/说明/标签..." value="' + escHtml(bmMkt.kw) + '" oninput="bmMktSearch(this.value)">' +
    (bmMkt.kw ? '<span class="bm-mkt-clear" onclick="bmMktClear()" style="position:absolute;right:8px;top:50%;transform:translateY(-50%);font-size:13px;color:var(--phx-text-secondary);cursor:pointer;padding:6px">✕</span>' : '') +
    '</div>' +
    '<div id="bm-mkt-sort-box" style="flex-shrink:0;min-width:150px"></div>' +
    '</div>';
}
// 排序下拉(项目自定义组件 prismDropdown)
function bmMktMountSort() {
  var sb = document.getElementById('bm-mkt-sort-box');
  if (!sb) return;
  if (sb._dd) { sb._dd.destroy(); sb._dd = null; }
  var opts = [
    ['', '最新上传'], ['old', '最早上传'],
    ['hot', '下载最多'], ['download_asc', '下载最少'],
    ['size_desc', '文件最大'], ['size_asc', '文件最小']
  ];
  sb._dd = prismDropdown(sb, { onChange: function (v) { bmMktSort(v); } });
  sb._dd.setOptions(opts.map(function (o) { return { value: o[0], label: o[1] }; }));
  sb._dd.setValue(bmMkt.sort || '');
}
function bmMktSort(v) { bmMkt.sort = v; bmMarketLoad(true); }
// 清除搜索词并重新加载(该分类下全部)
function bmMktClear() { bmMkt.kw = ''; bmMarketLoad(true); }
// 回到全部(清空分类与搜索)
function bmMktAll() { bmMkt.cat = ''; bmMkt.kw = ''; bmMkt.mine = false; bmOpenMarket(); }
// 我的已上传:列表源切到 /api/market/my
function bmMktMine() { bmMkt.cat = ''; bmMkt.kw = ''; bmMkt.mine = true; bmMkt.sort = ''; bmMarketLoad(true); }
function bmMktRender() {
  var b = bmBody(); if (!b) return;
  // 若输入框已存在则不重建(保留焦点,便于连续输入),只更新其搜索词
  var q = document.getElementById('bm-mkt-q');
  if (q) q.value = bmMkt.kw;
  var h = bmMktInput() + bmMktChips() + bmFileGrid(bmMkt.list);
  if (bmMkt.done) {
    h += bmMkt.list.length ? '<div style="text-align:center;color:var(--phx-text-secondary);padding:12px">已加载全部(' + bmMkt.total + ')</div>' : '';
  } else {
    h += '<div id="bm-mkt-sentinel" style="text-align:center;padding:14px"><span class="spin"></span> 加载中...</div>';
  }
  bmRender(h);
  // 重载后恢复焦点与光标到输入框末尾,便于继续搜索
  if (bmMkt.kw) {
    var q2 = document.getElementById('bm-mkt-q');
    if (q2) { q2.focus(); try { q2.setSelectionRange(q2.value.length, q2.value.length); } catch (e) {} }
  }
  bmMktMountSort();
  bmMktWatchScroll();
}
function bmMktSearch(v) {
  clearTimeout(bmMkt.timer);
  bmMkt.timer = setTimeout(function () { bmMkt.kw = (v || '').trim(); bmMarketLoad(true); }, 400);
}
function bmMktWatchScroll() {
  var sent = document.getElementById('bm-mkt-sentinel');
  if (!sent) return;
  if ('IntersectionObserver' in window) {
    var io = new IntersectionObserver(function (en) {
      if (en[0].isIntersecting) { io.disconnect(); bmMarketLoad(false); }
    });
    io.observe(sent);
  } else {
    window.onscroll = function () {
      var r = sent.getBoundingClientRect();
      if (r.top < window.innerHeight + 200) { window.onscroll = null; bmMarketLoad(false); }
    };
  }
}
function bmFileGrid(list) {
  if (!list.length) return '<div style="text-align:center;color:var(--phx-text-secondary);padding:20px">暂无文件</div>';
  var h = '<div class="grid-2">';
  list.forEach(function (f) {
    var cover = f.cover_part_id ? ('/api/market-proxy/api/market/parts/' + f.cover_part_id + '/preview') : '';
    var catTxt = (f.categories && f.categories.length) ? f.categories.join(' / ') : '其他';
    h += '<div class="grid-card" style="padding:8px;text-align:left" onclick="bmOpenDetail(' + f.id + ')">' +
      (cover ? '<img src="' + cover + '" style="width:100%;height:84px;object-fit:cover;border-radius:10px" onerror="this.style.display=\'none\'">' : '') +
      '<div style="font-size:12px;font-weight:700;margin-top:6px;line-height:1.3">' + escHtml(f.title) + '</div>' +
      '<div style="font-size:10px;color:var(--phx-text-secondary);margin-top:2px">' + (f.price_pts > 0 ? f.price_pts + ' 🌰' : '免费') + ' · 下载 ' + f.download_count + '</div>' +
      '<div style="font-size:10px;color:var(--phx-text-secondary);margin-top:2px">方块 ' + (f.block_count || 0) + ' · NBT ' + (f.nbt_count || 0) + '</div>' +
      '<div style="font-size:10px;color:var(--phx-primary);margin-top:2px">' + escHtml(catTxt) + '</div>' +
      (f.flagged ? '<div style="font-size:10px;color:var(--phx-warning);margin-top:2px">⚠ 已被举报</div>' : '') +
      '</div>';
  });
  h += '</div>';
  return h;
}
async function bmOpenDetail(id) {
  var _b = bmBody(); if (_b) bmMkt.scrollY = _b.scrollTop;   // 记住离开前的滚动位置,返回时恢复
  bmOpenSub('文件详情');
  var r = await AMarket('GET', '/api/market/files/' + id);
  if (!r.ok) { bmRender('<div style="color:var(--phx-error)">' + escHtml(r.error || '加载失败') + '</div>'); return; }
  var f = r.file, h = '';
  var previews = r.previews || [];
  var hasGif = r.has_gif;
  // 全部机位预览图(横向滑动)
  if (previews.length) {
    h += '<div style="display:flex;gap:6px;overflow-x:auto;margin-bottom:10px">';
    previews.forEach(function (pv, i) {
      var u = '/api/market-proxy' + pv.url;
      h += '<img src="' + u + '" style="width:120px;height:120px;object-fit:cover;border-radius:10px;flex-shrink:0;border:2px solid ' + (i === 0 ? 'var(--phx-primary)' : 'var(--phx-border-light)') + '" onclick="bmShowImage(\'' + u + '\')">';
    });
    h += '</div>';
  }
  // 环绕动画 GIF(付费需已购;未购时不显示,避免泄露)
  if (hasGif && r.can_download) {
    h += '<div style="margin-bottom:10px"><div class="dim" style="font-size:10px;margin-bottom:4px">环绕动画</div>' +
      '<img src="/api/market-proxy/api/market/files/' + f.id + '/gif" style="width:100%;border-radius:10px;background:#000"></div>';
  }
  h += '<div class="card" style="padding:12px">';
  h += '<div style="font-size:15px;font-weight:800">' + escHtml(f.title) + '</div>';
  h += '<div style="font-size:11px;color:var(--phx-text-secondary);margin:4px 0">' + (f.author ? '作者: ' + escHtml(f.author) : '') + ' · ' + (f.price_pts > 0 ? f.price_pts + ' 🌰' : '免费') + ' · 下载 ' + f.download_count + '</div>';
  // 需求8: 边长/方块数/NBT数/具体大小
  var dimsTxt = (f.block_dims && f.block_dims.x) ? (f.block_dims.x + '×' + f.block_dims.y + '×' + f.block_dims.z) : '';
  var stats = [];
  if (f.block_count) stats.push('方块 ' + f.block_count);
  if (f.nbt_count) stats.push('NBT ' + f.nbt_count);
  if (dimsTxt) stats.push('尺寸 ' + dimsTxt);
  if (f.file_size) stats.push('大小 ' + bmBytes(f.file_size));
  if (stats.length) h += '<div style="font-size:11px;color:var(--phx-text-secondary);margin:4px 0">' + stats.join(' · ') + '</div>';
  if (f.categories && f.categories.length) h += '<div style="font-size:11px;color:var(--phx-primary);margin:4px 0">分类: ' + escHtml(f.categories.join(' / ')) + '</div>';
  if (f.tags && f.tags.length) h += '<div style="font-size:11px;color:var(--phx-text-secondary);margin:4px 0">标签: ' + escHtml(f.tags.join('、')) + '</div>';
  if (f.description) h += '<div style="font-size:12px;margin:6px 0;line-height:1.6">' + escHtml(f.description) + '</div>';
  h += '</div>';
  if (f.price_pts > 0 && !r.already_purchased) {
    h += '<div style="font-size:11px;color:var(--phx-warning);margin:8px 0">付费文件 ' + f.price_pts + ' 🌰</div>';
  }
  if (f.flagged) {
    h += '<div style="font-size:11px;color:var(--phx-warning);background:rgba(255,180,0,.12);padding:8px;border-radius:8px;margin-top:8px">⚠ 此文件已被举报(' + (f.report_count || 0) + '),内容审核中</div>';
  }
  h += '<button class="btn" style="width:100%;margin-top:10px" onclick="bmDownload(' + f.id + ')">' + (f.price_pts > 0 && !r.already_purchased ? '付费下载' : '下载') + '</button>';
  // 需求4: 上传者在自己文件的详情内联管理面板(可直接操作);需求2: 非作者显示举报
  if (r.is_owner) {
    bmoeCover = 0;
    var catsR = await AMarket('GET', '/api/market/categories');
    var cats = catsR.ok ? catsR.categories : [];
    h += bmOwnerPanel(f.id, f, previews, cats);
  } else {
    h += '<button class="btn-s" style="width:100%;margin-top:8px;color:var(--phx-warning)" onclick="bmReportFile(' + f.id + ')">⚠ 举报此文件</button>';
  }
  bmRender(h);
  // 需求3: 评论
  bmLoadComments(id);
}
function bmBytes(n) {
  n = +n || 0;
  if (n < 1024) return n + ' B';
  if (n < 1048576) return (n / 1024).toFixed(1) + ' KB';
  if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB';
  return (n / 1073741824).toFixed(2) + ' GB';
}
// 下载防重复:同一文件短时间内只能触发一次;实际进度在系统通知栏(DownloadManager)
// 走专用端点 /api/market/download,由 Go 端显式带 cfg.Token 拉取,避免鉴权丢失
var bmDownloading = {};
function bmDownload(id) {
  if (bmDownloading[id]) { T('该文件已在下载中', 'w'); return; }
  bmDownloading[id] = true;
  var a = document.createElement('a');
  a.href = '/api/market/download?id=' + id;
  a.download = '';
  document.body.appendChild(a); a.click(); a.remove();
  T('已开始下载,进度见通知栏', 'o');
  setTimeout(function () { delete bmDownloading[id]; }, 5000);
}

// ── 评论(多级回复树,作者/普通用户区分) ──
async function bmLoadComments(id) {
  var r = await AMarket('GET', '/api/market/files/' + id + '/comments');
  var b = bmBody(); if (!b) return;
  // 用固定 id 去重: 已存在则整体替换,避免每次追加重复的输入框/按钮
  var box = document.getElementById('bm-cmt-box');
  if (!box) { box = document.createElement('div'); box.id = 'bm-cmt-box'; b.appendChild(box); }
  box.innerHTML = '<div style="margin-top:12px;border-top:1px solid var(--phx-border-light);padding-top:10px">' +
    '<div style="font-weight:800;margin-bottom:8px">评论</div>' +
    '<textarea id="bm-cmt-text" class="input" rows="2" placeholder="友善评论,理性发言..."></textarea>' +
    '<button class="btn" style="width:100%;margin-top:6px" onclick="bmAddComment(' + id + ')">发表评论</button>' +
    '<div id="bm-cmt-list" style="margin-top:6px"></div></div>';
  bmRenderComments(id, (r.ok ? r.comments : []) || []);
}
function bmRenderComments(id, tree) {
  var list = document.getElementById('bm-cmt-list'); if (!list) return;
  var h = '';
  (tree || []).forEach(function (c) { h += bmCmtNode(id, c, 0); });
  list.innerHTML = h || '<div style="color:var(--phx-text-secondary);font-size:11px;padding:8px">暂无评论,来抢沙发~</div>';
}
function bmCmtNode(id, c, depth) {
  var pad = Math.min(depth, 4) * 14;
  var badge = c.is_author ? '<span style="background:var(--phx-primary);color:#fff;font-size:8px;padding:1px 5px;border-radius:6px;margin-left:4px">作者</span>' : '';
  var uname = escHtml(c.username || ('用户#' + c.user_id));
  var nameColor = c.is_author ? 'var(--phx-primary)' : 'var(--phx-text)';
  var h = '<div style="margin-left:' + pad + 'px;margin-top:8px;padding:8px;border-left:2px solid ' + (c.is_author ? 'var(--phx-primary)' : 'var(--phx-border-light)') + ';background:rgba(128,128,128,.06);border-radius:6px">';
  h += '<div style="font-size:11px;font-weight:700;color:' + nameColor + '">' + uname + badge + '</div>';
  h += '<div style="font-size:12px;margin:4px 0;line-height:1.5;white-space:pre-wrap">' + escHtml(c.content) + '</div>';
  h += '<div style="font-size:9px;color:var(--phx-text-secondary)">' + escHtml((c.created_at || '').replace('T', ' ').slice(0, 16)) + '</div>';
  h += '<div style="margin-top:4px">' +
    '<button class="btn-s" style="padding:2px 8px;font-size:10px" onclick="bmReplyTo(' + id + ',' + c.id + ')">回复</button>';
  if (c.mine || c.is_author) {
    h += '<button class="btn-s" style="padding:2px 8px;font-size:10px;color:var(--phx-error);margin-left:6px" onclick="bmDelComment(' + id + ',' + c.id + ')">删除</button>';
  }
  h += '</div>';
  if (c.replies && c.replies.length) {
    c.replies.forEach(function (rc) { h += bmCmtNode(id, rc, depth + 1); });
  }
  h += '</div>';
  return h;
}
function bmReplyTo(id, pid) {
  var t = document.getElementById('bm-cmt-text'); if (!t) return;
  t.dataset.reply = pid;
  t.placeholder = '回复该评论...';
  t.focus();
}
function bmAddComment(id) {
  var t = document.getElementById('bm-cmt-text'); if (!t) return;
  var content = (t.value || '').trim();
  if (!content) { T('请输入评论内容', 'e'); return; }
  var parent = parseInt(t.dataset.reply || '0', 10) || 0;
  AMarket('POST', '/api/market/files/' + id + '/comments', { parent_id: parent, content: content }).then(function (r) {
    if (!r.ok) { T('发表失败: ' + (r.error || ''), 'e'); return; }
    T('评论成功', 'o');
    bmLoadComments(id);
  });
}
function bmDelComment(id, cid) {
  AMarket('POST', '/api/market/comments/' + cid).then(function (r) {
    if (!r.ok) { T('删除失败: ' + (r.error || ''), 'e'); return; }
    T('已删除', 'o');
    bmLoadComments(id);
  });
}

// ── 举报 ──
function bmReportFile(id) {
  var ov = document.createElement('div');
  ov.className = 'bm-overlay';
  ov.style.cssText = 'position:fixed;inset:0;z-index:1001;background:rgba(0,0,0,.6);display:flex;align-items:center;justify-content:center;padding:24px';
  ov.onclick = function (e) { if (e.target === ov) ov.remove(); };
  ov.innerHTML = '<div style="background:var(--phx-card);border-radius:12px;padding:16px;width:100%;max-width:320px">' +
    '<div style="font-weight:800;margin-bottom:8px">举报此文件</div>' +
    '<textarea id="bm-rp-reason" class="input" rows="3" placeholder="请填写举报原因"></textarea>' +
    '<div style="display:flex;gap:8px;margin-top:10px">' +
    '<button class="btn-s" style="flex:1" onclick="bmCloseReport()">取消</button>' +
    '<button class="btn" style="flex:1" onclick="bmReportSubmit(' + id + ')">提交举报</button></div></div>';
  document.body.appendChild(ov);
}
function bmCloseReport() {
  var o = document.querySelector('.bm-overlay'); if (o) o.remove();
}
function bmReportSubmit(id) {
  var t = document.getElementById('bm-rp-reason');
  var reason = ((t && t.value) || '').trim();
  AMarket('POST', '/api/market/files/' + id + '/report', { reason: reason }).then(function (r) {
    if (!r.ok) { T('举报失败: ' + (r.error || ''), 'e'); return; }
    T(r.message || '已举报,感谢反馈', 'o');
    bmCloseReport();
  });
}

// ── 需求4: 上传者在自己文件的详情内联管理面板(合并进文件详情) ──
var bmoeCover = 0;
function bmOwnerPanel(id, f, previews, cats) {
  var h = '<div class="card" style="padding:12px;margin-top:10px;display:flex;flex-direction:column;gap:8px">';
  h += '<div style="font-weight:800">管理此文件</div>';
  h += '<label>标题</label><input id="bmoe-title" class="input" value="' + escHtml(f.title) + '">';
  h += '<label>价格(板栗,0=免费)</label><input id="bmoe-price" class="input" type="number" min="0" value="' + (f.price_pts || 0) + '">';
  h += '<label class="lbl-cb"><input type="checkbox" id="bmoe-anon" ' + (f.allow_anonymous ? 'checked' : '') + '><span class="tgl"></span> 允许匿名下载</label>';
  h += '<label>说明</label><textarea id="bmoe-desc" class="input" rows="2">' + escHtml(f.description || '') + '</textarea>';
  h += '<label>标签(逗号分隔)</label><input id="bmoe-tags" class="input" value="' + escHtml((f.tags || []).join(',')) + '">';
  h += '<label>分类(可多选)</label><div style="display:flex;gap:6px;flex-wrap:wrap">';
  cats.forEach(function (c) {
    var on = (f.categories || []).indexOf(c.name) >= 0;
    h += '<label class="lbl-cb" style="margin:2px 4px"><input type="checkbox" class="bmoe-cat" value="' + escHtml(c.name) + '" ' + (on ? 'checked' : '') + '><span class="tgl"></span> ' + escHtml(c.name) + '</label>';
  });
  h += '</div>';
  // 封面(图标)选择:点选即高亮并把"封面"标签移到选中图
  h += '<label>封面图标</label><div style="display:flex;gap:6px;flex-wrap:wrap">';
  previews.forEach(function (pv) {
    var on = pv.id === (f.cover_part_id || (previews[0] && previews[0].id));
    h += '<div style="text-align:center;cursor:pointer" onclick="bmoeSetCover(' + pv.id + ',this)">' +
      '<img src="/api/market-proxy' + pv.url + '" data-cov="' + pv.id + '" style="width:56px;height:56px;object-fit:cover;border-radius:8px;border:2px solid ' + (on ? 'var(--phx-primary)' : 'var(--phx-border-light)') + '">' +
      (on ? '<div class="bmoe-cover-lbl" style="font-size:8px;color:var(--phx-primary)">封面</div>' : '') + '</div>';
  });
  h += '</div>';
  h += '<button class="btn" style="width:100%;margin-top:4px" onclick="bmoeSave(' + id + ')">保存修改</button>';
  h += '<button class="btn-s" style="width:100%;color:var(--phx-warning)" onclick="bmoeToggleStatus(' + id + ',' + (f.status === 1 ? 0 : 1) + ')">' + (f.status === 1 ? '下架(撤销发布)' : '重新上架') + '</button>';
  h += '<button class="btn-s" style="width:100%;color:var(--phx-error);margin-top:4px" onclick="bmoeDelToggle(this,' + id + ')">删除此文件</button>';
  h += '</div>';
  return h;
}
function bmoeDelToggle(btn, id) {
  if (btn.dataset.conf) { bmoeDelete(id); return; }
  btn.dataset.conf = '1';
  btn.textContent = '再次点击确认删除';
  setTimeout(function () { btn.dataset.conf = ''; btn.textContent = '删除此文件'; }, 3000);
}
async function bmoeDelete(id) {
  var r = await AMarket('POST', '/api/market/files/' + id + '/delete');
  if (!r.ok) { T('删除失败: ' + (r.error || ''), 'e'); return; }
  T('已删除', 'o');
  bmOpenMarket();
}
function bmoeSetCover(pid, el) {
  bmoeCover = pid;
  document.querySelectorAll('img[data-cov]').forEach(function (im) {
    var is = parseInt(im.dataset.cov) === pid;
    im.style.border = '2px solid ' + (is ? 'var(--phx-primary)' : 'var(--phx-border-light)');
    var wrap = im.parentElement;
    var lbl = wrap.querySelector('.bmoe-cover-lbl');
    if (is) {
      if (!lbl) {
        var d = document.createElement('div');
        d.className = 'bmoe-cover-lbl';
        d.textContent = '封面';
        d.style.cssText = 'font-size:8px;color:var(--phx-primary)';
        wrap.appendChild(d);
      }
    } else if (lbl) { lbl.remove(); }
  });
}
function bmoeCollect() {
  var cats = [];
  document.querySelectorAll('.bmoe-cat:checked').forEach(function (c) { cats.push(c.value); });
  var title = (document.getElementById('bmoe-title') || {}).value || '';
  var price = parseInt((document.getElementById('bmoe-price') || {}).value, 10) || 0;
  var anon = !!((document.getElementById('bmoe-anon') || {}).checked);
  var desc = (document.getElementById('bmoe-desc') || {}).value || '';
  var tags = ((document.getElementById('bmoe-tags') || {}).value || '').split(',').map(function (s) { return s.trim(); }).filter(Boolean);
  var body = { title: title.trim(), price_pts: price, allow_anonymous: anon, description: desc, tags: tags, categories: cats };
  if (bmoeCover) body.cover_part_id = bmoeCover;
  return body;
}
async function bmoeSave(id) {
  var body = bmoeCollect();
  if (!body.title) { T('标题不能为空', 'e'); return; }
  var r = await AMarket('POST', '/api/market/files/' + id + '/edit', body);
  if (!r.ok) { T('保存失败: ' + (r.error || ''), 'e'); return; }
  T('已保存', 'o');
  bmOpenDetail(id);
}
async function bmoeToggleStatus(id, st) {
  var body = bmoeCollect();
  if (!body.title) { T('标题不能为空', 'e'); return; }
  body.status = st;
  var r = await AMarket('POST', '/api/market/files/' + id + '/edit', body);
  if (!r.ok) { T('操作失败: ' + (r.error || ''), 'e'); return; }
  T(st === 1 ? '已重新上架' : '已下架', 'o');
  bmOpenDetail(id);
}

// ── 批量建筑预览(原逻辑不变) ──
function bmOpenBatch() {
  bmOpenSub('批量建筑预览', '<div id="preview-wrap" style="width:100%"><div id="preview-body"></div></div>');
  setTimeout(function () { if (typeof renderPreviewStep === 'function') renderPreviewStep((previewState && previewState.step) || 1); }, 50);
}

// ═══════════════ 贡献文件:自动生成预览(全内存,无写盘) ═══════════════
var bmCon = {
  struct: '', previews: [], coverIdx: 0,
  gif: false, gifFrames: null, angles: [],
  title: '', desc: '', cats: [], tags: '', price: 0, anon: true
};

async function bmOpenContribute() {
  bmOpenSub('贡献文件');
  var catsR = await AMarket('GET', '/api/market/categories');
  var catList = catsR.ok ? catsR.categories : [];
  var h = '<div class="card" style="padding:12px;display:flex;flex-direction:column;gap:10px">';

  h += '<label>建筑文件</label>' +
    '<div class="pick" onclick="bmPickStruct()"><i class="pi">' + ICONS.upload + '</i><div id="bmc-struct">点击选择建筑文件 (.mcstructure/.mcworld/.schematic/.schem/.bdx)</div></div>';

  h += '<label>预览图</label><div id="bmc-preview-zone"></div>';

  h += '<label>标题</label><input id="bmc-title" class="input" maxlength="30" placeholder="建筑标题(≤30字)">';
  h += '<label>说明</label><textarea id="bmc-desc" class="input" rows="2" maxlength="200" placeholder="描述(可选,≤200字)"></textarea>';

  h += '<label>分类(可多选,最多4个)</label><div style="display:flex;gap:6px;flex-wrap:wrap">';
  catList.forEach(function (c) {
    h += '<label class="lbl-cb" style="margin:2px 4px"><input type="checkbox" class="bmc-cat" value="' + escHtml(c.name) + '" onchange="bmCatLimit(this)"><span class="tgl"></span> ' + escHtml(c.name) + '</label>';
  });
  h += '</div>';

  h += '<label>标签(逗号分隔,最多4个,每个≤10字)</label><input id="bmc-tags" class="input" placeholder="如 现代,小型,带泳池">';

  h += '<label class="lbl-cb"><input type="checkbox" id="bmc-paid" onchange="bmPaidSync()"><span class="tgl"></span> 付费文件(🌰)</label>';
  h += '<div id="bmc-price-wrap" class="gone"><label>价格(板栗)</label><input id="bmc-price" class="input" type="number" min="1" value="1" oninput="bmCon.price=+this.value"></div>';
  h += '<div id="bmc-anon-wrap"><label class="lbl-cb"><input type="checkbox" id="bmc-anon" checked onchange="bmCon.anon=this.checked"><span class="tgl"></span> 允许匿名下载</label></div>';

  h += '<div id="bmc-upload-status"></div>';
  h += '<button class="btn" id="bmc-upload-btn" style="width:100%" onclick="bmSubmit()">上传到市场</button>';
  h += '</div>';
  bmRender(h);
  bmRenderPreviewZone();
}

// 分类最多选 4 个(本地限制)
function bmCatLimit(el) {
  var n = document.querySelectorAll('.bmc-cat:checked').length;
  if (n > 4) { el.checked = false; T('最多只能选择 4 个分类', 'w'); }
}

function bmPickStruct() {
  pickBySetting('building', function (path) {
    bmCon.struct = path;
    // 重新选择建筑文件 → 旧的预览图/机位/封面全部作废
    bmCon.previews = [];
    bmCon.coverIdx = 0;
    bmCon.gifFrames = null;
    bmCon.angles = [];
    var el = document.getElementById('bmc-struct');
    if (el) el.textContent = '已选: ' + path;
    bmRenderPreviewZone();
  });
}

// ── 预览图区块 ──
function bmRenderPreviewZone() {
  var el = document.getElementById('bmc-preview-zone');
  if (!el) return;
  var h = '';
  if (!bmCon.previews.length) {
    h += '<button class="btn-s" style="width:100%;padding:12px" onclick="bmPickAngles()">' + ICONS.file + ' 选择机位并生成预览图</button>' +
      '<div class="dim" style="font-size:10px;margin-top:6px">从建筑文件自动渲染 WebP 预览,无需手动选图</div>';
  } else {
    h += '<div style="font-size:11px;font-weight:700;color:var(--phx-text-secondary);margin-bottom:6px">已生成 ' + bmCon.previews.length + ' / 5 张预览(点"设为封面"即列表封面)</div>';
    h += '<div style="display:flex;gap:8px;flex-wrap:wrap" id="bmc-pv-grid"></div>';
    h += '<button class="btn-s" style="width:100%;margin-top:8px" onclick="bmPickAngles()">添加 / 更换机位</button>';
  }
  el.innerHTML = h;
  bmRenderPreviewGrid();
}

function bmRenderPreviewGrid() {
  var grid = document.getElementById('bmc-pv-grid');
  if (!grid || !bmCon.previews.length) return;
  grid.innerHTML = bmCon.previews.map(function (p, i) {
    var url = URL.createObjectURL(p.blob);
    var isCover = i === bmCon.coverIdx;
    return '<div style="width:72px;text-align:center;position:relative">' +
      '<div style="position:relative;width:72px;height:72px;border-radius:10px;overflow:hidden;border:2px solid ' + (isCover ? 'var(--phx-primary)' : 'var(--phx-border-light)') + '">' +
      '<img src="' + url + '" style="width:100%;height:100%;object-fit:cover" onclick="bmShowImage(\'' + url + '\')">' +
      (isCover ? '<div style="position:absolute;top:2px;left:2px;background:var(--phx-primary);color:#fff;font-size:8px;font-weight:700;padding:1px 4px;border-radius:6px">封面</div>' : '') +
      '<button style="position:absolute;top:2px;right:2px;width:18px;height:18px;border:none;border-radius:50%;background:rgba(0,0,0,.55);color:#fff;font-size:11px;line-height:18px;cursor:pointer" onclick="bmRemovePreview(' + i + ')">×</button>' +
      '</div>' +
      '<button class="btn-s" style="width:auto;padding:2px 6px;font-size:9px;margin-top:4px" onclick="bmSetCover(' + i + ')">' + (isCover ? '已设为封面' : '设为封面') + '</button>' +
      '</div>';
  }).join('');
}

// 图片灯箱:点击预览图放大居中显示,点击外部关闭
function bmShowImage(url) {
  var ov = document.createElement('div');
  ov.style.cssText = 'position:fixed;inset:0;z-index:1001;background:rgba(0,0,0,.7);display:flex;align-items:center;justify-content:center;padding:24px';
  ov.onclick = function () { ov.remove(); };
  ov.innerHTML = '<div style="max-width:92vw;max-height:88vh;display:flex" onclick="event.stopPropagation()">' +
    '<img src="' + url + '" style="max-width:92vw;max-height:88vh;object-fit:contain;border-radius:12px;box-shadow:0 12px 48px rgba(0,0,0,.5)">' +
    '</div>';
  document.body.appendChild(ov);
}

function bmRemovePreview(i) {
  if (i < 0 || i >= bmCon.previews.length) return;
  bmCon.previews.splice(i, 1);
  if (bmCon.coverIdx >= bmCon.previews.length) bmCon.coverIdx = Math.max(0, bmCon.previews.length - 1);
  bmRenderPreviewZone();
}

function bmSetCover(i) {
  if (i >= 0 && i < bmCon.previews.length) bmCon.coverIdx = i;
  bmRenderPreviewGrid();
}

// 付费联动:勾选付费 → 显示价格输入 + 隐藏匿名下载;强制价格 ≥1
function bmPaidSync() {
  var paid = document.getElementById('bmc-paid');
  var priceWrap = document.getElementById('bmc-price-wrap');
  var anonWrap = document.getElementById('bmc-anon-wrap');
  var on = paid && paid.checked;
  if (on && !bmCon.price) bmCon.price = 1;
  if (priceWrap) priceWrap.classList.toggle('gone', !on);
  if (anonWrap) anonWrap.classList.toggle('gone', on); // 付费时隐藏匿名下载
}

// ── 机位选择(嵌套弹窗,与批量预览一致:先选类别→关弹窗→再选具体) ──
var bmAnglePresets = {
  direction: { label: '正方向', items: [['front', '正面'], ['back', '背面'], ['left', '左面'], ['right', '右面']] },
  corner: { label: '对角', items: [['northeast', '东北角'], ['southeast', '东南角'], ['northwest', '西北角'], ['southwest', '西南角']] },
  octo: { label: '八角', items: [['0', '0°'], ['45', '45°'], ['90', '90°'], ['135', '135°'], ['180', '180°'], ['225', '225°'], ['270', '270°'], ['315', '315°']] },
  birdseye: { label: '鸟瞰', items: [['top', '正上方俯视']] }
};

// 第一层弹窗:选择机位类别(与批量预览一致的 bs-sheet 列表)
function bmPickAngles() {
  var overlay = document.createElement('div'); overlay.className = 'bs-overlay';
  overlay.innerHTML =
    '<div class="bs-sheet">' +
    '<div class="bs-title">选择机位</div>' +
    '<div class="bs-body">' +
    '<div class="bs-option" onclick="closeBsOverlay(this);bmPickAnglesCat(\'direction\')"><div class="bs-dot"></div><span class="bs-label">正方向</span><span class="dim">前 / 后 / 左 / 右</span></div>' +
    '<div class="bs-option" onclick="closeBsOverlay(this);bmPickAnglesCat(\'corner\')"><div class="bs-dot"></div><span class="bs-label">对角</span><span class="dim">4 个等轴测角</span></div>' +
    '<div class="bs-option" onclick="closeBsOverlay(this);bmPickAnglesCat(\'octo\')"><div class="bs-dot"></div><span class="bs-label">八角</span><span class="dim">45° 增量 8 个方向</span></div>' +
    '<div class="bs-option" onclick="closeBsOverlay(this);bmPickAnglesCat(\'birdseye\')"><div class="bs-dot"></div><span class="bs-label">鸟瞰</span><span class="dim">正上方俯视</span></div>' +
    '<div class="bs-option" onclick="closeBsOverlay(this);bmPickAnglesCustom()"><div class="bs-dot"></div><span class="bs-label">自定义</span><span class="dim">自由角度</span></div>' +
    '</div>' +
    '<div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">取消</button></div></div>';
  document.body.appendChild(overlay);
}

// 第二层弹窗:该类别下的具体机位(多选)
function bmPickAnglesCat(type) {
  var g = bmAnglePresets[type];
  if (!g) return;
  var items = '';
  g.items.forEach(function (it) {
    items += '<label class="lbl-cb" style="padding:12px 18px;margin:0;border-bottom:1px solid var(--phx-border-light)">' +
      '<input type="checkbox" class="bm-angle-cb" value="' + it[0] + '" data-label="' + it[1] + '"><span class="tgl"></span> ' + it[1] + '</label>';
  });
  var overlay = document.createElement('div'); overlay.className = 'bs-overlay';
  overlay.innerHTML =
    '<div class="bs-sheet">' +
    '<div class="bs-title">' + g.label + '</div>' +
    '<div class="bs-body">' + items + '</div>' +
    '<div class="bs-footer">' +
    '<button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this);bmPickAngles()">取消</button>' +
    '<button class="bs-btn bs-btn-confirm" onclick="bmAngleConfirm()">确定添加</button></div></div>';
  document.body.appendChild(overlay);
}

// 自定义角度弹窗
function bmPickAnglesCustom() {
  var overlay = document.createElement('div'); overlay.className = 'bs-overlay';
  overlay.innerHTML =
    '<div class="bs-sheet">' +
    '<div class="bs-title">自定义角度</div>' +
    '<div class="bs-body" style="padding:8px 18px">' +
    '<div id="bm-custom-preview-3d" style="width:100%;height:220px;border-radius:12px;border:2px solid var(--phx-border-light);margin-bottom:12px;overflow:hidden"></div>' +
    '<label>水平角</label><input id="bm-custom-yaw" type="number" value="45" style="margin:0" onchange="if(window.__previewAngleUpdate)window.__previewAngleUpdate()">' +
    '<label>仰角</label><input id="bm-custom-pitch" type="number" value="30" style="margin:0" onchange="if(window.__previewAngleUpdate)window.__previewAngleUpdate()">' +
    '<label>距离</label><input id="bm-custom-dist" type="number" value="200" style="margin:0" onchange="if(window.__previewAngleUpdate)window.__previewAngleUpdate()">' +
    '<div class="dim" style="font-size:9px;margin-top:4px">水平角 0~360,仰角 5~85,距离 50~500</div></div>' +
    '<div class="bs-footer">' +
    '<button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this);previewCleanupAnglePreview();bmPickAngles()">取消</button>' +
    '<button class="bs-btn bs-btn-confirm" onclick="bmAngleCustomConfirm()">确定添加</button></div></div>';
  document.body.appendChild(overlay);
  // 房子模型 3D 预览(与批量预览共用),实时预览当前自定义角度
  setTimeout(function() { if (typeof previewInitAnglePreview === 'function') previewInitAnglePreview('bm-custom-preview-3d', 'bm-custom-yaw', 'bm-custom-pitch', 'bm-custom-dist'); }, 100);
}

// 预设类别确定:收集勾选的具体机位
function bmAngleConfirm() {
  var angles = [];
  document.querySelectorAll('.bm-angle-cb:checked').forEach(function (cb) {
    angles.push({ type: 'preset', value: cb.value, label: cb.dataset.label });
  });
  if (!angles.length) { T('请至少选择一个机位', 'e'); return; }
  document.querySelectorAll('.bs-overlay').forEach(function (o) { closeBsOverlay(o); });
  bmAddAngles(angles);
}

// 自定义确定
function bmAngleCustomConfirm() {
  var yaw = parseInt((document.getElementById('bm-custom-yaw') || {}).value) || 45;
  var pitch = parseInt((document.getElementById('bm-custom-pitch') || {}).value) || 30;
  var dist = parseInt((document.getElementById('bm-custom-dist') || {}).value) || 200;
  previewCleanupAnglePreview();
  document.querySelectorAll('.bs-overlay').forEach(function (o) { closeBsOverlay(o); });
  bmAddAngles([{ type: 'custom', yaw: yaw, pitch: pitch, dist: dist, label: '自定义 ' + yaw + '°' }]);
}

// 追加机位并生成;预览最多 5 个,超 5 自动去掉最旧的
function bmAddAngles(newAngles) {
  bmCon.angles = bmCon.angles.concat(newAngles);
  if (bmCon.angles.length > 5) {
    bmCon.angles = bmCon.angles.slice(bmCon.angles.length - 5);
    T('预览最多 5 张,已保留最近的 5 张', 'w');
  }
  bmGeneratePreview();
}

function bmGeneratePreview() {
  if (!bmCon.struct) { T('请先选择建筑文件', 'e'); return; }
  if (!bmCon.angles.length) { T('请先选择机位', 'e'); return; }
  var zone = document.getElementById('bmc-preview-zone');
  if (zone) zone.innerHTML = '<div style="text-align:center;padding:16px"><span class="spin"></span> 正在生成预览图...</div>';
  generatePreviewsForContribute(bmCon.struct, bmCon.angles, { gif: false, width: 768, height: 768, quality: 0.7 }, function (res) {
    if (!res || res.error) { T((res && res.error) || '生成失败', 'e'); bmRenderPreviewZone(); return; }
    bmCon.previews = res.webps;
    bmCon.coverIdx = 0;
    bmCon.gifFrames = res.gifFrames || null;
    bmRenderPreviewZone();
    T('已生成 ' + bmCon.previews.length + ' 张预览', 'o');
  });
}

// ── 提交上传 ──
var bmSubmitting = false; // 防止重复提交
function blobToBase64(blob, cb) {
  var fr = new FileReader();
  fr.onload = function () { cb(String(fr.result).split(',')[1] || fr.result); };
  fr.readAsDataURL(blob);
}

function bmSubmit() {
  if (bmSubmitting) { T('正在上传中,请勿重复操作', 'w'); return; }
  if (!bmCon.struct) { T('请先选择建筑文件', 'e'); return; }
  if (!bmCon.previews.length) { T('请先生成预览图', 'e'); return; }
  if (bmCon.gif && (!bmCon.price || bmCon.price < 1)) { T('GIF 文件需设置至少 1 板栗价格', 'e'); return; }

  var cats = [];
  document.querySelectorAll('.bmc-cat:checked').forEach(function (c) { cats.push(c.value); });
  var title = (document.getElementById('bmc-title') || {}).value || '';
  var desc = (document.getElementById('bmc-desc') || {}).value || '';
  var tags = ((document.getElementById('bmc-tags') || {}).value || '').split(',').map(function (s) { return s.trim(); }).filter(Boolean);

  // ── 本地校验 ──
  if (cats.length > 4) { T('最多只能选择 4 个分类', 'e'); return; }
  if (tags.length > 4) { T('自定义标签最多 4 个', 'e'); return; }
  for (var ti = 0; ti < tags.length; ti++) { if (tags[ti].length > 10) { T('每个标签最长 10 个字: ' + tags[ti], 'e'); return; } }
  var titleT = title.trim();
  if (!titleT) { T('请填写标题', 'e'); return; }
  if (titleT.length > 30) { T('标题最长 30 个字', 'e'); return; }
  if (desc.trim().length > 200) { T('说明最长 200 个字', 'e'); return; }
  if (bmCon.price > 100) { T('金额不能超过 100 板栗', 'e'); return; }

  // 封面(coverIdx)排第一位 → 服务端取第一张 WebP 作封面
  var ordered = bmCon.previews.slice();
  var cover = ordered.splice(bmCon.coverIdx, 1)[0];
  ordered.unshift(cover);

  // 锁定:禁用按钮,防重复
  bmSubmitting = true;
  var btn = document.getElementById('bmc-upload-btn');
  if (btn) { btn.disabled = true; btn.textContent = '正在上传...'; }
  var st = document.getElementById('bmc-upload-status');
  if (st) st.innerHTML = '<div style="text-align:center;padding:8px"><span class="spin"></span> 正在上传建筑与预览图,请稍候...</div>';

  var previews = [], n = ordered.length;
  ordered.forEach(function (w, i) {
    blobToBase64(w.blob, function (b64) {
      previews[i] = { name: w.name, ext: '.webp', data: b64, w: w.width || 768, h: w.height || 768 };
      if (previews.filter(function (x) { return x; }).length === n) {
        bmDoUpload(previews, cover, title, desc, cats, tags);
      }
    });
  });
}

function bmUploadDone() {
  bmSubmitting = false;
  var btn = document.getElementById('bmc-upload-btn');
  if (btn) { btn.disabled = false; btn.textContent = '上传到市场'; }
}

function bmDoUpload(previews, cover, title, desc, cats, tags) {
  var body = {
    struct_path: bmCon.struct, previews: previews, title: title.trim(), description: desc,
    categories: cats, tags: tags, price_pts: bmCon.price || 0, allow_anonymous: bmCon.anon,
  };
  if (bmCon.gifFrames) body.gif_frames = bmCon.gifFrames;
  // 封面机位(preview_cam,可选)
  if (cover && cover.angle) {
    if (cover.angle.type === 'custom') {
      body.preview_cam = { yaw: cover.angle.yaw || 45, pitch: cover.angle.pitch || 30, dist: cover.angle.dist || 200 };
    } else {
      var am = { front: [0, 0], back: [180, 0], left: [270, 0], right: [90, 0],
        northeast: [45, 30], southeast: [135, 30], northwest: [315, 30], southwest: [225, 30],
        top: [0, 89], '0': [0, 30], '45': [45, 30], '90': [90, 30], '135': [135, 30],
        '180': [180, 30], '225': [225, 30], '270': [270, 30], '315': [315, 30] };
      var ap = am[cover.angle.value] || [45, 30];
      body.preview_cam = { yaw: ap[0], pitch: ap[1], dist: 200 };
    }
  }
  A('POST', '/api/market/contribute', body).then(function (r) {
    bmUploadDone();
    var st = document.getElementById('bmc-upload-status');
    if (r.ok) {
      if (st) st.innerHTML = '<div style="text-align:center;padding:8px;color:var(--phx-success);font-weight:700">上传成功! 文件ID: ' + (r.id || '') + ' ✓</div>';
      T('上传成功! 文件#' + (r.id || ''), 'o');
      bmCon = { struct: '', previews: [], coverIdx: 0, gif: false, gifFrames: null, angles: [], title: '', desc: '', cats: [], tags: '', price: 0, anon: true };
      setTimeout(function () { bmOpenContribute(); }, 600);
    } else {
      if (st) st.innerHTML = '<div style="text-align:center;padding:8px;color:var(--phx-error);font-weight:700">上传失败: ' + escHtml(r.error || '未知') + '</div>';
      T('上传失败: ' + (r.error || '未知'), 'e');
    }
  });
}
