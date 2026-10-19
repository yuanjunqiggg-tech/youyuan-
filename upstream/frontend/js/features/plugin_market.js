// ── 插件市场:浏览 / 下载 / 付费 / 上传 ──
// 代理调用 Prism 插件市场 API(透传 multipart/二进制/JSON + 鉴权)。本地接口(install/contribute)走 A()。

// 能力中文名映射(卡片/详情/上传复用)
const PM_CAP_NAMES = {
  system_call: '调用手机系统', notify: '发送通知', message_rw: '监听/发送文件消息',
  game_command: '执行游戏指令', file_rw: '读写本地文件', network: '联网请求'
};
// 上传表单勾选顺序
// 卡片颜色预设
var PM_CARD_COLORS = ['#e74c3c', '#e67e22', '#f1c40f', '#2ecc71', '#3498db', '#9b59b6', '#ecf0f1'];

// 代理调用 Prism 插件市场 API
async function APluginMarket(m, p, body, headers) {
  var c = new AbortController(), t = setTimeout(function () { c.abort(); }, 120000);
  try {
    var o = { method: m, headers: {}, signal: c.signal };
    if (body instanceof FormData) { o.body = body; }
    else if (body) { o.headers['Content-Type'] = 'application/json'; o.body = JSON.stringify(body); }
    if (headers) Object.assign(o.headers, headers);
    var r = await fetch('/api/plugin-market-proxy' + p, o); clearTimeout(t); return r.json();
  } catch (e) { clearTimeout(t); return { ok: false, error: e.name === 'AbortError' ? '超时' : e.message }; }
}

// 子页渲染辅助(与 building_mgmt 的 bmBody/bmRender/bmOpenSub 同构)
function pmBody() { return document.querySelector('#sub-page .sp-body'); }
function pmRender(html) {
  var b = pmBody(); if (b) b.innerHTML = html;
  var titleEl = document.querySelector('.sp-title');
  if (titleEl) {
    var key = titleEl.textContent;
    var sp = document.getElementById('sub-page');
    var header = sp.querySelector('.sp-header');
    if (header) subPageCache[key] = header.outerHTML + '<div class="sp-body">' + html + '</div>';
  }
}
function pmOpenSub(title, loadingHtml) {
  delete subPageCache[title];
  openSubPage(title, loadingHtml || '<div style="text-align:center;padding:30px"><span class="spin"></span> 加载中...</div>');
}

// markdown 渲染(防御 marked 未加载)
function pmRenderMarkdown(md) {
  return (typeof marked !== 'undefined' && marked.parse) ? marked.parse(md || '')
    : '<pre style="white-space:pre-wrap;font-family:monospace;font-size:12px">' + escHtml(md || '') + '</pre>';
}

// ── 插件市场(无限滚动 + 搜索 + 分类 + 我的已上传) ──
var pmMkt = { page: 0, size: 20, total: 0, kw: '', cat: '', cats: [], list: [], loading: false, done: false, timer: 0, mine: false, showMine: false, scrollY: 0, sort: '' };

// 从详情返回(或重新打开)市场列表时:恢复滚动位置 + 重新挂载无限滚动哨兵
// (恢复缓存只写回静态 HTML,不会重建 IntersectionObserver,需在此重建)
window.__subPageRestoreHooks = window.__subPageRestoreHooks || [];
window.__subPageRestoreHooks.push({ key: '*', fn: function () {
  if (!document.getElementById('pm-mkt-q')) return;   // 当前页不是插件市场列表
  var b = pmBody(); if (!b) return;
  if (pmMkt.scrollY) b.scrollTop = pmMkt.scrollY;
  pmMktMountSort();
  pmMktWatchScroll();
} });

function pmMktUrl() {
  var base = pmMkt.mine ? '/api/plugin-market/my' : '/api/plugin-market/files';
  var u = base + '?page=' + pmMkt.page + '&page_size=' + pmMkt.size;
  if (pmMkt.cat) u += '&category=' + encodeURIComponent(pmMkt.cat);
  if (pmMkt.kw) u += '&q=' + encodeURIComponent(pmMkt.kw);
  if (!pmMkt.mine && pmMkt.sort) u += '&order=' + pmMkt.sort;
  return u;
}

// 探测"我的已上传"是否可用(未登录/401 时不显示该 chip)
async function pmProbeMine() {
  try {
    var r = await APluginMarket('GET', '/api/plugin-market/my');
    return !!r.ok;
  } catch (e) { return false; }
}

async function pmOpenMarket() {
  pmOpenSub('插件市场');
  var catsR = await APluginMarket('GET', '/api/plugin-market/categories');
  pmMkt.cats = catsR.ok ? catsR.categories : [];
  pmMkt.cat = ''; pmMkt.kw = ''; pmMkt.mine = false; pmMkt.sort = '';
  pmMkt.showMine = await pmProbeMine();
  await pmMarketLoad(true);
}

function pmMktAll() { pmMkt.cat = ''; pmMkt.kw = ''; pmMkt.mine = false; pmMarketLoad(true); }
function pmMktMine() { pmMkt.cat = ''; pmMkt.kw = ''; pmMkt.mine = true; pmMkt.sort = ''; pmMarketLoad(true); }
function pmMktCat(cat) { pmMkt.cat = cat; pmMkt.kw = ''; pmMkt.mine = false; pmMarketLoad(true); }

async function pmMarketLoad(reset) {
  if (pmMkt.loading) return;
  if (reset) { pmMkt.page = 0; pmMkt.list = []; pmMkt.done = false; pmMkt.scrollY = 0; pmMktRender(); }
  if (pmMkt.done) { pmMktRender(); return; }
  pmMkt.loading = true;
  pmMkt.page++;
  var r = await APluginMarket('GET', pmMktUrl());
  pmMkt.loading = false;
  if (!r.ok) { T('加载失败: ' + (r.error || ''), 'e'); return; }
  var arr = r.list || r.files || [];
  pmMkt.total = r.total || 0;
  pmMkt.list = pmMkt.list.concat(arr);
  if (!arr.length || arr.length < pmMkt.size) pmMkt.done = true;
  pmMktRender();
}

function pmMktChips() {
  var h = '<div style="display:flex;gap:6px;overflow-x:auto;padding-bottom:8px;margin-bottom:8px">';
  h += '<span class="bm-cat' + (!pmMkt.cat && !pmMkt.mine ? ' bm-cat-on' : '') + '" onclick="pmMktAll()" style="flex-shrink:0">全部</span>';
  if (pmMkt.showMine) {
    h += '<span class="bm-cat' + (pmMkt.mine ? ' bm-cat-on' : '') + '" onclick="pmMktMine()" style="flex-shrink:0">我的已上传</span>';
  }
  (pmMkt.cats || []).forEach(function (c) {
    var on = !pmMkt.mine && pmMkt.cat === c.name;
    h += '<span class="bm-cat' + (on ? ' bm-cat-on' : '') + '" onclick="pmMktCat(\'' + c.name.replace(/'/g, "\\'") + '\')" style="flex-shrink:0">' + escHtml(c.name) + '</span>';
  });
  h += '</div>';
  return h;
}
function pmMktInput() {
  return '<div style="display:flex;gap:6px;margin-bottom:8px;align-items:center">' +
    '<div style="position:relative;flex:1">' +
    '<input id="pm-mkt-q" class="input" style="width:100%;padding-right:30px" placeholder="搜索插件标题/说明/标签..." value="' + escHtml(pmMkt.kw) + '" oninput="pmMktSearch(this.value)">' +
    (pmMkt.kw ? '<span onclick="pmMktClear()" style="position:absolute;right:8px;top:50%;transform:translateY(-50%);font-size:13px;color:var(--phx-text-secondary);cursor:pointer;padding:6px">✕</span>' : '') +
    '</div>' +
    '<div id="pm-mkt-sort-box" style="flex-shrink:0;min-width:150px"></div>' +
    '</div>';
}
// 排序下拉(项目自定义组件 prismDropdown)
function pmMktMountSort() {
  var sb = document.getElementById('pm-mkt-sort-box');
  if (!sb) return;
  if (sb._dd) { sb._dd.destroy(); sb._dd = null; }
  var opts = [
    ['', '最新上传'], ['old', '最早上传'],
    ['hot', '下载最多'], ['download_asc', '下载最少'],
    ['size_desc', '文件最大'], ['size_asc', '文件最小']
  ];
  sb._dd = prismDropdown(sb, { onChange: function (v) { pmMktSort(v); } });
  sb._dd.setOptions(opts.map(function (o) { return { value: o[0], label: o[1] }; }));
  sb._dd.setValue(pmMkt.sort || '');
}
function pmMktSort(v) { pmMkt.sort = v; pmMarketLoad(true); }
function pmMktClear() { pmMkt.kw = ''; pmMarketLoad(true); }
function pmMktSearch(v) {
  clearTimeout(pmMkt.timer);
  pmMkt.timer = setTimeout(function () { pmMkt.kw = (v || '').trim(); pmMarketLoad(true); }, 400);
}
function pmMktRender() {
  var b = pmBody(); if (!b) return;
  var q = document.getElementById('pm-mkt-q');
  if (q) q.value = pmMkt.kw;
  var h = pmMktInput() + pmMktChips() + pmFileGrid(pmMkt.list);
  if (pmMkt.done) {
    h += pmMkt.list.length ? '<div style="text-align:center;color:var(--phx-text-secondary);padding:12px">已加载全部(' + pmMkt.total + ')</div>' : '';
  } else {
    h += '<div id="pm-mkt-sentinel" style="text-align:center;padding:14px"><span class="spin"></span> 加载中...</div>';
  }
  pmRender(h);
  if (pmMkt.kw) {
    var q2 = document.getElementById('pm-mkt-q');
    if (q2) { q2.focus(); try { q2.setSelectionRange(q2.value.length, q2.value.length); } catch (e) {} }
  }
  pmMktMountSort();
  pmMktWatchScroll();
}
function pmMktWatchScroll() {
  var sent = document.getElementById('pm-mkt-sentinel');
  if (!sent) return;
  if ('IntersectionObserver' in window) {
    var io = new IntersectionObserver(function (en) {
      if (en[0].isIntersecting) { io.disconnect(); pmMarketLoad(false); }
    });
    io.observe(sent);
  } else {
    window.onscroll = function () {
      var r = sent.getBoundingClientRect();
      if (r.top < window.innerHeight + 200) { window.onscroll = null; pmMarketLoad(false); }
    };
  }
}

// 徽标 / 风险 / 能力
function pmReviewBadge(st) {
  if (st === 0) return '<span style="background:rgba(255,180,0,.18);color:var(--phx-warning);font-size:9px;padding:1px 6px;border-radius:6px">待审核</span>';
  if (st === 1) return '<span style="background:rgba(46,204,113,.18);color:var(--phx-success);font-size:9px;padding:1px 6px;border-radius:6px">已审核</span>';
  return '';
}
function pmRiskTxt(r) {
  if (r === 1) return '<span style="color:var(--phx-warning);font-size:10px">⚠ 中风险</span>';
  if (r === 2) return '<span style="color:var(--phx-error);font-size:10px">⚠ 高风险</span>';
  return '<span style="color:var(--phx-success);font-size:10px">低风险</span>';
}
function pmCapBadges(caps) {
  if (!caps || !caps.length) return '';
  var h = '';
  caps.forEach(function (c) {
    var name = PM_CAP_NAMES[c] || c;
    h += '<span style="background:rgba(128,128,128,.15);font-size:9px;padding:1px 6px;border-radius:6px;margin-right:4px">' + escHtml(name) + '</span>';
  });
  return h;
}
function pmCatTxt(f) {
  return (f.categories && f.categories.length) ? f.categories.join(' / ') : '其他';
}

function pmFileGrid(list) {
  if (!list.length) return '<div style="text-align:center;color:var(--phx-text-secondary);padding:20px">暂无插件</div>';
  var h = '<div class="grid-2">';
  list.forEach(function (f) {
    var bar = f.card_color ? '<div style="height:4px;border-radius:4px 4px 0 0;background:' + f.card_color + ';margin:-12px -14px 10px -14px"></div>' : '';
    h += '<div class="grid-card" style="padding:12px 14px;text-align:left;border-left:3px solid ' + (f.card_color || 'transparent') + '" onclick="pmOpenDetail(' + f.id + ')">' +
      bar +
      '<div style="font-size:12px;font-weight:700;margin-top:6px;line-height:1.35">' + escHtml(f.name) + '</div>' +
      '<div style="font-size:10px;color:var(--phx-text-secondary);margin-top:4px">' + (f.uploader_name ? escHtml(f.uploader_name) + ' · ' : '') + (f.price_pts > 0 ? f.price_pts + ' 🌰' : '免费') + ' · 下载 ' + (f.download_count || 0) + '</div>' +
      '<div style="font-size:10px;color:var(--phx-primary);margin-top:4px">' + escHtml(pmCatTxt(f)) + '</div>' +
      '<div style="margin-top:6px;display:flex;align-items:center;gap:6px;flex-wrap:wrap">' + pmReviewBadge(f.review_state) + pmRiskTxt(f.risk_level) + '</div>' +
      (f.declared_caps && f.declared_caps.length ? '<div style="margin-top:6px">' + pmCapBadges(f.declared_caps) + '</div>' : '') +
      '</div>';
  });
  h += '</div>';
  return h;
}

// ── 详情页 ──
async function pmOpenDetail(id, expectedOrigin) {
  var _b = pmBody(); if (_b) pmMkt.scrollY = _b.scrollTop;   // 记住离开前的滚动位置,返回时恢复
  pmOpenSub('插件详情');
  var r = await APluginMarket('GET', '/api/plugin-market/files/' + id,null,expectedOrigin?{'X-YouYuan-Market-Origin':expectedOrigin}:undefined);
  if (!r.ok) { pmRender('<div style="color:var(--phx-error)">' + escHtml(r.error || '加载失败') + '</div>'); return; }
  var f = r.file || {}, versions = r.versions || [], is_owner = !!r.is_owner;
  var h = '<div class="card" style="padding:12px">';
  h += '<div style="font-size:15px;font-weight:800">' + escHtml(f.name) + '</div>';
  h += '<div style="font-size:11px;color:var(--phx-text-secondary);margin:4px 0">' + (f.uploader_name ? '作者: ' + escHtml(f.uploader_name) : '') + ' · ' + (f.price_pts > 0 ? f.price_pts + ' 🌰' : '免费') + ' · 下载 ' + (f.download_count || 0) + '</div>';
  h += '<div style="margin:4px 0;display:flex;align-items:center;gap:8px;flex-wrap:wrap">' + pmReviewBadge(f.review_state) + pmRiskTxt(f.risk_level) + '</div>';
  if (f.declared_caps && f.declared_caps.length) h += '<div style="margin:4px 0">能力: ' + pmCapBadges(f.declared_caps) + '</div>';
  if (f.categories && f.categories.length) h += '<div style="font-size:11px;color:var(--phx-primary);margin:4px 0">分类: ' + escHtml(f.categories.join(' / ')) + '</div>';
  if (f.tags && f.tags.length) h += '<div style="font-size:11px;color:var(--phx-text-secondary);margin:4px 0">标签: ' + escHtml(f.tags.join('、')) + '</div>';
  if (f.description) h += '<div style="font-size:12px;margin:6px 0;line-height:1.6">' + escHtml(f.description) + '</div>';
  h += '<button class="btn" style="width:100%;margin-top:10px" onclick="pmInstall(' + id + ',' + (f.review_state === undefined ? -1 : f.review_state) + ')">获取插件</button>';
  h += '</div>';
  if (f.docs) {
    h += '<div class="md-view" style="background:var(--phx-bg);border:2px solid var(--phx-border-light);border-radius:10px;padding:14px;font-size:13px;line-height:1.7;margin-top:10px">' + pmRenderMarkdown(f.docs) + '</div>';
  }
  if (versions.length) {
    h += '<div class="card" style="padding:12px;margin-top:10px"><div style="font-weight:800;margin-bottom:8px">版本历史</div>';
    versions.forEach(function (v) {
      h += '<div style="padding:8px 0;border-bottom:1px solid var(--phx-border-light)">' +
        '<div style="font-size:12px;font-weight:700">' + escHtml(v.version || ('v' + v.version_code || '')) + ' <span style="font-weight:400;color:var(--phx-text-secondary)">' + escHtml((v.created_at || '').replace('T', ' ').slice(0, 16)) + '</span></div>' +
        (v.changelog ? '<div style="font-size:11px;color:var(--phx-text-secondary);margin-top:2px;white-space:pre-wrap">' + escHtml(v.changelog) + '</div>' : '') +
        '</div>';
    });
    h += '</div>';
  }
  if (is_owner) {
    h += await pmOwnerPanel(id, f);
  } else {
    h += '<button class="btn-s" style="width:100%;margin-top:8px;color:var(--phx-warning)" onclick="pmReportFile(' + id + ')">⚠ 举报此插件</button>';
  }
  pmRender(h);
  pmLoadComments(id);
}

// ── 获取 / 安装 ──
function pmInstall(id, rv) {
  if (rv === 0) { pmShowRiskModal(id); return; }
  pmDoInstall(id);
}
function pmShowRiskModal(id) {
  var ov = document.createElement('div'); ov.className = 'modal-overlay';
  ov.style.cssText = 'position:fixed;inset:0;z-index:1002;background:rgba(0,0,0,.75);display:flex;align-items:center;justify-content:center;padding:24px';
  ov.innerHTML = '<div style="background:var(--phx-card);border-radius:12px;padding:18px;width:100%;max-width:360px;text-align:center">' +
    '<div style="font-size:15px;font-weight:800;color:var(--phx-warning);margin-bottom:10px">⚠ 危险操作提醒</div>' +
    '<div style="font-size:12px;line-height:1.7;margin-bottom:14px">该插件<strong>尚未经过官方审核</strong>。未审核插件可能伴随危险性,请谨慎下载并自行承担风险。</div>' +
    '<div style="display:flex;gap:8px">' +
    '<button class="btn-s" style="flex:1" onclick="pmCloseRiskModal()">取消</button>' +
    '<button class="btn" style="flex:1" onclick="pmCloseRiskModal();pmDoInstall(' + id + ')">仍然下载</button></div></div>';
  document.body.appendChild(ov);
}
function pmCloseRiskModal() {
  document.querySelectorAll('.modal-overlay').forEach(function (x) { x.remove(); });
}
async function pmDoInstall(id) {
  var installScope=typeof yyReadScope==='function'?yyReadScope():null;
  T('正在获取插件...', 'w');
  var r = await A('POST', '/api/plugin-market/install', { id: String(id) });
  if(installScope!==null&&installScope!==yyReadScope())return;
  if (!r.ok) { T('获取失败: ' + (r.error || ''), 'e'); return; }
  if(typeof yyPluginRecordSource==='function')yyPluginRecordSource(r.id,r.source_id||id,r.source_origin);
  T('已安装到本机 Lua 插件：' + (r.id||id), 'o');
  if (typeof Rplg === 'function') { Rplg(); if(typeof yyNavigate==='function')yyNavigate('tasks'); }
}

// ── 评论(多级回复树) ──
async function pmLoadComments(id) {
  var r = await APluginMarket('GET', '/api/plugin-market/files/' + id + '/comments');
  var b = pmBody(); if (!b) return;
  var box = document.getElementById('pm-cmt-box');
  if (!box) { box = document.createElement('div'); box.id = 'pm-cmt-box'; b.appendChild(box); }
  box.innerHTML = '<div style="margin-top:12px;border-top:1px solid var(--phx-border-light);padding-top:10px">' +
    '<div style="font-weight:800;margin-bottom:8px">评论</div>' +
    '<textarea id="pm-cmt-text" class="input" rows="2" placeholder="友善评论,理性发言..."></textarea>' +
    '<button class="btn" style="width:100%;margin-top:6px" onclick="pmAddComment(' + id + ')">发表评论</button>' +
    '<div id="pm-cmt-list" style="margin-top:6px"></div></div>';
  pmRenderComments(id, (r.ok ? r.comments : []) || []);
}
function pmRenderComments(id, tree) {
  var list = document.getElementById('pm-cmt-list'); if (!list) return;
  var h = '';
  (tree || []).forEach(function (c) { h += pmCmtNode(id, c, 0); });
  list.innerHTML = h || '<div style="color:var(--phx-text-secondary);font-size:11px;padding:8px">暂无评论,来抢沙发~</div>';
}
function pmCmtNode(id, c, depth) {
  var pad = Math.min(depth, 4) * 14;
  var badge = c.is_author ? '<span style="background:var(--phx-primary);color:#fff;font-size:8px;padding:1px 5px;border-radius:6px;margin-left:4px">作者</span>' : '';
  var uname = escHtml(c.username || ('用户#' + c.user_id));
  var nameColor = c.is_author ? 'var(--phx-primary)' : 'var(--phx-text)';
  var h = '<div style="margin-left:' + pad + 'px;margin-top:8px;padding:8px;border-left:2px solid ' + (c.is_author ? 'var(--phx-primary)' : 'var(--phx-border-light)') + ';background:rgba(128,128,128,.06);border-radius:6px">';
  h += '<div style="font-size:11px;font-weight:700;color:' + nameColor + '">' + uname + badge + '</div>';
  h += '<div style="font-size:12px;margin:4px 0;line-height:1.5;white-space:pre-wrap">' + escHtml(c.content) + '</div>';
  h += '<div style="font-size:9px;color:var(--phx-text-secondary)">' + escHtml((c.created_at || '').replace('T', ' ').slice(0, 16)) + '</div>';
  h += '<div style="margin-top:4px">' +
    '<button class="btn-s" style="padding:2px 8px;font-size:10px" onclick="pmReplyTo(' + id + ',' + c.id + ')">回复</button>';
  if (c.mine || c.is_author) {
    h += '<button class="btn-s" style="padding:2px 8px;font-size:10px;color:var(--phx-error);margin-left:6px" onclick="pmDelComment(' + id + ',' + c.id + ')">删除</button>';
  }
  h += '</div>';
  if (c.replies && c.replies.length) {
    c.replies.forEach(function (rc) { h += pmCmtNode(id, rc, depth + 1); });
  }
  h += '</div>';
  return h;
}
function pmReplyTo(id, pid) {
  var t = document.getElementById('pm-cmt-text'); if (!t) return;
  t.dataset.reply = pid;
  t.placeholder = '回复该评论...';
  t.focus();
}
function pmAddComment(id) {
  var t = document.getElementById('pm-cmt-text'); if (!t) return;
  var content = (t.value || '').trim();
  if (!content) { T('请输入评论内容', 'e'); return; }
  var parent = parseInt(t.dataset.reply || '0', 10) || 0;
  APluginMarket('POST', '/api/plugin-market/files/' + id + '/comments', { parent_id: parent, content: content }).then(function (r) {
    if (!r.ok) { T('发表失败: ' + (r.error || ''), 'e'); return; }
    T('评论成功', 'o');
    pmLoadComments(id);
  });
}
function pmDelComment(id, cid) {
  APluginMarket('POST', '/api/plugin-market/comments/' + cid).then(function (r) {
    if (!r.ok) { T('删除失败: ' + (r.error || ''), 'e'); return; }
    T('已删除', 'o');
    pmLoadComments(id);
  });
}

// ── 举报 ──
function pmReportFile(id) {
  var ov = document.createElement('div');
  ov.className = 'pm-overlay';
  ov.style.cssText = 'position:fixed;inset:0;z-index:1001;background:rgba(0,0,0,.6);display:flex;align-items:center;justify-content:center;padding:24px';
  ov.onclick = function (e) { if (e.target === ov) ov.remove(); };
  ov.innerHTML = '<div style="background:var(--phx-card);border-radius:12px;padding:16px;width:100%;max-width:320px">' +
    '<div style="font-weight:800;margin-bottom:8px">举报此插件</div>' +
    '<textarea id="pm-rp-reason" class="input" rows="3" placeholder="请填写举报原因"></textarea>' +
    '<div style="display:flex;gap:8px;margin-top:10px">' +
    '<button class="btn-s" style="flex:1" onclick="pmCloseReport()">取消</button>' +
    '<button class="btn" style="flex:1" onclick="pmReportSubmit(' + id + ')">提交举报</button></div></div>';
  document.body.appendChild(ov);
}
function pmCloseReport() {
  var o = document.querySelector('.pm-overlay'); if (o) o.remove();
}
function pmReportSubmit(id) {
  var t = document.getElementById('pm-rp-reason');
  var reason = ((t && t.value) || '').trim();
  APluginMarket('POST', '/api/plugin-market/files/' + id + '/report', { reason: reason }).then(function (r) {
    if (!r.ok) { T('举报失败: ' + (r.error || ''), 'e'); return; }
    T(r.message || '已举报,感谢反馈', 'o');
    pmCloseReport();
  });
}

// ── 上传者管理面板(内联) ──
async function pmOwnerPanel(id, f) {
  var catsR = await APluginMarket('GET', '/api/plugin-market/categories');
  var cats = catsR.ok ? catsR.categories : [];
  var h = '<div class="card" style="padding:12px;margin-top:10px;display:flex;flex-direction:column;gap:8px">';
  h += '<div style="font-weight:800">管理此插件</div>';
  h += '<label>标题</label><input id="pmoe-title" class="input" value="' + escHtml(f.name) + '">';
  h += '<label>价格(板栗,0=免费)</label><input id="pmoe-price" class="input" type="number" min="0" value="' + (f.price_pts || 0) + '">';
  h += '<label class="lbl-cb"><input type="checkbox" id="pmoe-anon" ' + (f.allow_anonymous ? 'checked' : '') + '><span class="tgl"></span> 允许匿名下载</label>';
  h += '<label>说明</label><textarea id="pmoe-desc" class="input" rows="2">' + escHtml(f.description || '') + '</textarea>';
  h += '<label>标签(逗号分隔)</label><input id="pmoe-tags" class="input" value="' + escHtml((f.tags || []).join(',')) + '">';
  h += '<label>文档(markdown)</label><textarea id="pmoe-docs" class="input" rows="4">' + escHtml(f.docs || '') + '</textarea>';
  h += '<label>分类(可多选)</label><div style="display:flex;gap:6px;flex-wrap:wrap">';
  cats.forEach(function (c) {
    var on = (f.categories || []).indexOf(c.name) >= 0;
    h += '<label class="lbl-cb" style="margin:2px 4px"><input type="checkbox" class="pmoe-cat" value="' + escHtml(c.name) + '" ' + (on ? 'checked' : '') + '><span class="tgl"></span> ' + escHtml(c.name) + '</label>';
  });
  h += '</div>';
  h += '<button class="btn" style="width:100%;margin-top:4px" onclick="pmoeSave(' + id + ')">保存修改</button>';
  h += '<button class="btn-s" style="width:100%;color:var(--phx-warning)" onclick="pmoeToggleStatus(' + id + ',' + (f.status === 1 ? 0 : 1) + ')">' + (f.status === 1 ? '下架(撤销发布)' : '重新上架') + '</button>';
  h += '<button class="btn-s" style="width:100%;color:var(--phx-error);margin-top:4px" onclick="pmoeDelToggle(this,' + id + ')">删除此插件</button>';
  h += '</div>';
  return h;
}
function pmoeDelToggle(btn, id) {
  if (btn.dataset.conf) { pmoeDelete(id); return; }
  btn.dataset.conf = '1';
  btn.textContent = '再次点击确认删除';
  setTimeout(function () { btn.dataset.conf = ''; btn.textContent = '删除此插件'; }, 3000);
}
async function pmoeDelete(id) {
  var r = await APluginMarket('POST', '/api/plugin-market/files/' + id + '/delete');
  if (!r.ok) { T('删除失败: ' + (r.error || ''), 'e'); return; }
  T('已删除', 'o');
  pmOpenMarket();
}
function pmoeCollect() {
  var cats = [];
  document.querySelectorAll('.pmoe-cat:checked').forEach(function (c) { cats.push(c.value); });
  var title = (document.getElementById('pmoe-title') || {}).value || '';
  var price = parseInt((document.getElementById('pmoe-price') || {}).value, 10) || 0;
  var anon = !!((document.getElementById('pmoe-anon') || {}).checked);
  var desc = (document.getElementById('pmoe-desc') || {}).value || '';
  var docs = (document.getElementById('pmoe-docs') || {}).value || '';
  var tags = ((document.getElementById('pmoe-tags') || {}).value || '').split(',').map(function (s) { return s.trim(); }).filter(Boolean);
  return { title: title.trim(), price_pts: price, allow_anonymous: anon, description: desc, docs: docs, tags: tags, categories: cats };
}
async function pmoeSave(id) {
  var body = pmoeCollect();
  if (!body.title) { T('标题不能为空', 'e'); return; }
  var r = await APluginMarket('POST', '/api/plugin-market/files/' + id + '/edit', body);
  if (!r.ok) { T('保存失败: ' + (r.error || ''), 'e'); return; }
  T('已保存', 'o');
  pmOpenDetail(id);
}
async function pmoeToggleStatus(id, st) {
  var body = pmoeCollect();
  if (!body.title) { T('标题不能为空', 'e'); return; }
  body.status = st;
  var r = await APluginMarket('POST', '/api/plugin-market/files/' + id + '/edit', body);
  if (!r.ok) { T('操作失败: ' + (r.error || ''), 'e'); return; }
  T(st === 1 ? '已重新上架' : '已下架', 'o');
  pmOpenDetail(id);
}

// ── 上传到插件市场(预填当前插件) ──
var pmuCardColor = '';
async function pmOpenUpload(pluginId) {
  pmOpenSub('上传到插件市场');
  var catsR = await APluginMarket('GET', '/api/plugin-market/categories');
  var catList = catsR.ok ? catsR.categories : [];
  var h = '<div class="card" style="padding:12px;display:flex;flex-direction:column;gap:10px">';
  h += '<div class="dim" style="font-size:11px">上传当前插件到市场: <strong>' + escHtml(pluginId || '') + '</strong></div>';
  h += '<label>价格(板栗,0=免费)</label><input id="pmu-price" class="input" type="number" min="0" value="0">';
  h += '<label class="lbl-cb"><input type="checkbox" id="pmu-anon" checked><span class="tgl"></span> 允许匿名下载</label>';
  h += '<label>分类</label><select id="pmu-cat" class="input"><option value="">其他</option>';
  catList.forEach(function (c) { h += '<option value="' + escHtml(c.name) + '">' + escHtml(c.name) + '</option>'; });
  h += '</select>';
  h += '<label>说明</label><textarea id="pmu-desc" class="input" rows="2" placeholder="插件说明(可选)"></textarea>';
  h += '<label>更新日志</label><input id="pmu-changelog" class="input" placeholder="本次更新内容(可选)">';
  h += '<label>文档(markdown,可选,留空则自动读取插件内 docs.md)</label><textarea id="pmu-docs" class="input" rows="3" placeholder="# 使用说明..."></textarea>';
  h += '<label>卡片颜色</label><div id="pmu-colors" style="display:flex;gap:6px;flex-wrap:wrap">';
  h += '<div class="pmu-color" data-c="" onclick="pmPickColor(\'\',this)" title="主题中性色" style="width:28px;height:28px;border-radius:8px;border:2px solid var(--phx-border);background:var(--phx-card);cursor:pointer"></div>';
  PM_CARD_COLORS.forEach(function (col) {
    h += '<div class="pmu-color" data-c="' + col + '" onclick="pmPickColor(\'' + col + '\',this)" style="width:28px;height:28px;border-radius:8px;border:2px solid var(--phx-border);background:' + col + ';cursor:pointer"></div>';
  });
  h += '</div>';
  h += '<button class="btn" style="width:100%" onclick="pmSubmit(\'' + String(pluginId).replace(/'/g, "\\'") + '\')">发布到市场</button>';
  h += '</div>';
  pmRender(h);
}
function pmPickColor(col, el) {
  pmuCardColor = col || '';
  document.querySelectorAll('#pmu-colors .pmu-color').forEach(function (x) {
    x.style.outline = (x.dataset.c === (col || '')) ? '2px solid var(--phx-primary)' : 'none';
  });
}
async function pmSubmit(pluginId) {
  var price = parseInt((document.getElementById('pmu-price') || {}).value, 10) || 0;
  var anon = !!((document.getElementById('pmu-anon') || {}).checked);
  var desc = (document.getElementById('pmu-desc') || {}).value || '';
  var changelog = (document.getElementById('pmu-changelog') || {}).value || '';
  var docs = (document.getElementById('pmu-docs') || {}).value || '';
  var cats = [];
  var sel = document.getElementById('pmu-cat');
  if (sel && sel.value) cats.push(sel.value);
  // 能力声明由客户端扫描插件代码自动判定并加密上传，不在此采集
  var body = {
    plugin_id: pluginId, description: desc, docs: docs, categories: cats, tags: [],
    price_pts: price, allow_anonymous: anon, card_color: pmuCardColor, changelog: changelog
  };
  T('正在发布...', 'w');
  var r = await A('POST', '/api/plugin-market/contribute', body);
  if (!r.ok) { T('发布失败: ' + (r.error || ''), 'e'); return; }
  T('已发布,等待审核', 'o');
  pmOpenMarket();
}
