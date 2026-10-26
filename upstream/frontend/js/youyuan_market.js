/* 自建插件市场与开发者中心：浏览、详情、发布、上传版本、收益口径。 */
(function () {
  'use strict';

  var state = {
    tab: 'market',
    q: '',
    category: 'all',
    sort: 'hot',
    mc_version: '',
    loader: '',
    page: 1,
    pageSize: 12,
    total: 0,
    mine: [],
    editId: '',
    versions: {}
  };
  var MC_VERSION_OPTIONS = ['1.18', '1.19', '1.20', '1.21', '1.22', '1.23', '1.24'];
  var LOADER_OPTIONS = ['Lua', 'Python', 'JavaScript', 'C++'];
  var epoch = 0;
  var searchTimer = 0;
  var PAGE_TITLE = '插件市场';
  var DETAIL_TITLE = '插件详情';

  var CATEGORY_LABELS = {
    all: '全部', tool: '工具', gameplay: '玩法', automation: '自动化',
    ui: '界面', library: '资料库', other: '其他'
  };
  var STATUS_LABELS = {
    draft: '草稿', pending: '审核中', approved: '已上架', rejected: '已驳回', offline: '已下架'
  };
  var SORTS = [['hot', '热门'], ['new', '最新'], ['price', '价格']];

  function esc(value) {
    return String(value == null ? '' : value).replace(/[&<>"']/g, function (ch) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch];
    });
  }
  function icon(name) {
    return typeof yyIcon === 'function' ? yyIcon(name || 'plugin') : '';
  }
  function money(cents) {
    var value = Number(cents || 0) / 100;
    return '¥' + value.toFixed(2);
  }
  function sizeText(bytes) {
    var n = Number(bytes || 0);
    if (n >= 1048576) return (n / 1048576).toFixed(1) + ' MB';
    if (n >= 1024) return (n / 1024).toFixed(1) + ' KB';
    return n + ' B';
  }
  function listOf(value) {
    return String(value || '').split(/[,，]/).map(function (item) { return item.trim(); }).filter(Boolean);
  }
  function linesOf(value) {
    return String(value || '').split(/\r?\n/).map(function (item) { return item.trim(); }).filter(Boolean);
  }
  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text != null) node.textContent = text;
    return node;
  }
  function statusBadge(status) {
    return '<span class="yy-market-badge is-' + esc(status) + '">' + esc(STATUS_LABELS[status] || status) + '</span>';
  }
  function priceBadge(cents, large) {
    var free = Number(cents || 0) === 0;
    return '<span class="yy-market-price' + (large ? ' is-large' : '') + (free ? ' is-free' : '') + '">' +
      (free ? '免费' : esc(money(cents))) + '</span>';
  }
  function ratingText(item) {
    if (!item.rating_count) return '暂无评分';
    return Number(item.rating).toFixed(1) + ' 分';
  }
  function ratingStars(item) {
    var count = Number(item.rating_count || 0);
    if (!count) return '<span class="yy-market-stars is-empty">暂无评分</span>';
    var rating = Number(item.rating || 0);
    var full = Math.floor(rating + 0.0001);
    if (full > 5) full = 5;
    var html = '<span class="yy-market-stars" aria-label="' + esc(rating.toFixed(1)) + ' 分 / 5，' + count + ' 人评分">';
    var i;
    for (i = 0; i < full; i++) html += '<span class="yy-star is-full">★</span>';
    for (i = full; i < 5; i++) html += '<span class="yy-star">★</span>';
    html += '<span class="yy-market-stars-num">' + esc(rating.toFixed(1)) + ' · ' + count + ' 人评</span></span>';
    return html;
  }

  function shell() {
    return '<div class="yy-market">' +
      '<div class="yy-market-tabs" role="tablist">' +
      '<button type="button" class="yy-market-tab" id="yy-market-tab-market" role="tab" onclick="yyMarketTab(\'market\')">插件市场</button>' +
      '<button type="button" class="yy-market-tab" id="yy-market-tab-dev" role="tab" onclick="yyMarketTab(\'dev\')">开发者中心</button>' +
      '</div><div id="yy-market-body"></div></div>';
  }

  function filterRow() {
    function opts(list, current) {
      var html = '<option value="">全部</option>';
      list.forEach(function (v) {
        html += '<option value="' + esc(v) + '"' + (v === current ? ' selected' : '') + '>' + esc(v) + '</option>';
      });
      return html;
    }
    return '<div class="yy-market-filters" role="group" aria-label="筛选">' +
      '<label class="yy-market-filter">MC 版本<select id="yy-market-mc" onchange="yyMarketSetFilter(\'mc_version\', this.value)">' + opts(MC_VERSION_OPTIONS, state.mc_version) + '</select></label>' +
      '<label class="yy-market-filter">加载器<select id="yy-market-loader" onchange="yyMarketSetFilter(\'loader\', this.value)">' + opts(LOADER_OPTIONS, state.loader) + '</select></label>' +
      '</div>';
  }

  function marketShell() {
    var sortButtons = SORTS.map(function (pair) {
      return '<button type="button" class="yy-market-sort-btn' + (state.sort === pair[0] ? ' is-on' : '') +
        '" data-sort="' + pair[0] + '" onclick="yyMarketSetSort(\'' + pair[0] + '\')">' + pair[1] + '</button>';
    }).join('');
    return '<div class="yy-market-toolbar">' +
      '<label class="yy-market-search">' + icon('search') +
      '<input id="yy-market-q" type="search" placeholder="搜索插件、作者或标签" aria-label="搜索插件" value="' + esc(state.q) + '" oninput="yyMarketSearch(this.value)"></label>' +
      '<div class="yy-market-sort" role="group" aria-label="排序">' + sortButtons + '</div>' +
      '</div>' + filterRow() +
      '<div class="yy-market-chips" id="yy-market-chips"></div>' +
      '<div class="yy-market-grid" id="yy-market-list" aria-live="polite"></div>' +
      '<div class="yy-market-pager" id="yy-market-pager"></div>';
  }

  function devShell() {
    return '<div class="yy-market-dev">' +
      '<p class="yy-market-dev-hint" id="yy-market-dev-hint" hidden></p>' +
      '<section class="card yy-market-split-hero" id="yy-market-split-hero">' +
      '<h3>分成规则</h3>' +
      '<p class="yy-market-split-hero-lead">你为自己的插件<strong>自由定价</strong>，平台统一收取 <strong>30%</strong> 服务费，开发者实收 <strong>70%</strong>；也可以完全免费，免费插件不抽成。</p>' +
      '<div class="yy-market-split-bars" aria-hidden="true"><span class="is-dev">开发者 70%</span><span class="is-platform">平台 30%</span></div>' +
      '<p class="yy-market-split-note">定价由开发者自定，金额以分为单位结算。支付通道接入后，付费插件的购买额将按上述比例自动分成到你的收益。</p>' +
      '</section>' +
      '<section class="card yy-market-earn" id="yy-market-earn"><h3>收益</h3><p class="yy-hint">正在读取…</p></section>' +
      '<section class="card yy-market-flow"><h3>发布流程</h3><p class="yy-hint">存草稿 → 上传版本 → 提交审核 → 平台审核 → 上架。审核中或已上架的插件不能修改基础信息，需下架后再调整。付费插件上架后即可产生收益，按 30/70 分成。</p></section>' +
      '<section class="card"><h3>我的插件</h3><div id="yy-market-mine"><p class="yy-hint">正在读取…</p></div></section>' +
      '<div id="yy-market-forms">' +
      '<section class="card" id="yy-market-form-card">' +
      '<h3 id="yy-market-form-title">新建插件</h3>' +
      '<p class="yy-hint">先存草稿，上传版本后再提交审核。价格填 0 表示免费。</p>' +
      '<div class="yy-market-form">' +
      '<label>插件名称<input id="yy-market-f-name" maxlength="60" placeholder="2–60 个字符"></label>' +
      '<label>分类<select id="yy-market-f-category">' +
      ['tool', 'gameplay', 'automation', 'ui', 'library', 'other'].map(function (key) {
        return '<option value="' + key + '">' + CATEGORY_LABELS[key] + '</option>';
      }).join('') + '</select></label>' +
      '<label class="is-wide">一句话简介<input id="yy-market-f-summary" maxlength="200" placeholder="列表里显示两行"></label>' +
      '<label class="is-wide">详情<textarea id="yy-market-f-description" rows="5" maxlength="20000" placeholder="玩法、依赖、注意事项。支持纯文本换行。"></textarea></label>' +
      '<label>标签<input id="yy-market-f-tags" placeholder="逗号分隔，最多 8 个"></label>' +
      '<div class="yy-market-price-block">' +
      '<div class="yy-market-price-mode" role="group" aria-label="定价方式">' +
      '<button type="button" class="yy-mode-btn" id="yy-market-f-paid" onclick="yyMarketSetFree(false)">付费定价</button>' +
      '<button type="button" class="yy-mode-btn" id="yy-market-f-free" onclick="yyMarketSetFree(true)">免费</button>' +
      '</div>' +
      '<div class="yy-market-price-row">' +
      '<label>价格 / 元<input id="yy-market-f-price" type="number" min="0" step="0.01" value="0" oninput="yyMarketSyncPriceMode()"></label>' +
      '<div class="yy-market-split" id="yy-market-split" aria-live="polite"></div>' +
      '</div>' +
      '<p class="yy-market-split-note">平台统一收取 30% 服务费，开发者实收 70%；价格由你决定，也可选择完全免费。</p>' +
      '</div>' +
      '<label>许可证<input id="yy-market-f-license" maxlength="40" placeholder="MIT / GPL-3.0 / Proprietary"></label>' +
      '<label>支持的 MC 版本<input id="yy-market-f-mc" placeholder="逗号分隔，如 1.20,1.21"></label>' +
      '<label>加载器 / 运行环境<input id="yy-market-f-loader" placeholder="逗号分隔，如 Lua,Python"></label>' +
      '<label class="is-wide">截图 URL<textarea id="yy-market-f-shots" rows="2" placeholder="每行一个 http(s) 图片地址，最多 8 张"></textarea></label>' +
      '</div><div class="yy-market-form-actions">' +
      '<button type="button" class="btn" id="yy-market-save" onclick="yyMarketSavePlugin()">保存草稿</button>' +
      '<button type="button" class="btn-s" id="yy-market-reset" onclick="yyMarketResetForm()">清空</button>' +
      '</div></section>' +
      '<section class="card" id="yy-market-upload-card"><h3>上传版本</h3>' +
      '<p class="yy-hint">支持 .zip / .lua / .py / .js，单个不超过 20 MB。上传会记录 SHA-256；已上架的插件上传新版本后需重新审核。</p>' +
      '<div class="yy-market-form">' +
      '<label>插件<select id="yy-market-u-plugin"></select></label>' +
      '<label>版本号<input id="yy-market-u-version" maxlength="32" placeholder="如 1.0.0"></label>' +
      '<label>加载器<input id="yy-market-u-loader" maxlength="24" placeholder="如 Lua"></label>' +
      '<label>支持的 MC 版本<input id="yy-market-u-mc" placeholder="逗号分隔"></label>' +
      '<label class="is-wide">更新日志<textarea id="yy-market-u-changelog" rows="3" maxlength="4000" placeholder="这个版本改了什么"></textarea></label>' +
      '<label class="is-wide">插件包<input id="yy-market-u-file" type="file" accept=".zip,.lua,.py,.js"></label>' +
      '</div><div class="yy-market-form-actions">' +
      '<button type="button" class="btn" id="yy-market-upload" onclick="yyMarketUpload()">上传版本</button>' +
      '<span class="yy-hint" id="yy-market-upload-status" role="status"></span>' +
      '</div></section></div></div>';
  }

  function markTabs() {
    var market = document.getElementById('yy-market-tab-market');
    var dev = document.getElementById('yy-market-tab-dev');
    if (market) {
      market.classList.toggle('is-on', state.tab === 'market');
      market.setAttribute('aria-selected', state.tab === 'market' ? 'true' : 'false');
    }
    if (dev) {
      dev.classList.toggle('is-on', state.tab === 'dev');
      dev.setAttribute('aria-selected', state.tab === 'dev' ? 'true' : 'false');
    }
  }

  function paint() {
    var host = document.getElementById('yy-market-body');
    if (!host) return;
    markTabs();
    host.innerHTML = state.tab === 'dev' ? devShell() : marketShell();
    if (state.tab === 'dev') {
      loadDev();
    } else {
      renderChips({});
      loadList();
    }
  }

  function renderChips(counts) {
    var host = document.getElementById('yy-market-chips');
    if (!host) return;
    host.replaceChildren();
    Object.keys(CATEGORY_LABELS).forEach(function (key) {
      var count = counts && counts[key] ? counts[key] : 0;
      var button = el('button', 'yy-market-chip' + (state.category === key ? ' is-on' : ''),
        CATEGORY_LABELS[key] + (key !== 'all' && count ? ' ' + count : ''));
      button.type = 'button';
      button.setAttribute('aria-pressed', state.category === key ? 'true' : 'false');
      button.onclick = function () {
        state.category = key;
        state.page = 1;
        renderChips(counts);
        loadList();
      };
      host.appendChild(button);
    });
  }

  function showListMessage(message) {
    var host = document.getElementById('yy-market-list');
    if (!host) return;
    host.replaceChildren();
    var box = el('div', 'yy-market-empty');
    box.appendChild(el('strong', null, message));
    var hint = el('p', 'yy-hint', '插件由开发者自行发布，平台审核后上架。');
    box.appendChild(hint);
    var button = el('button', 'btn', '去开发者中心发布');
    button.type = 'button';
    button.onclick = function () { window.yyMarketTab('dev'); };
    box.appendChild(button);
    host.appendChild(box);
  }

  function card(item) {
    var button = el('button', 'yy-market-card');
    button.type = 'button';
    button.setAttribute('aria-label', item.name + '，' + (item.free ? '免费' : money(item.price_cents)));
    button.onclick = function () { window.yyMarketOpen(item.id); };

    var head = el('div', 'yy-market-card-head');
    var badge = el('div', 'yy-market-icon');
    badge.innerHTML = icon('plugin');
    var titleBox = el('div', 'yy-market-card-title');
    titleBox.appendChild(el('strong', null, item.name || '未命名插件'));
    titleBox.appendChild(el('span', 'yy-market-author', (item.author || '未知作者') + ' · ' + (CATEGORY_LABELS[item.category] || '其他')));
    head.append(badge, titleBox, el('span', 'yy-market-price-holder'));
    head.lastChild.innerHTML = priceBadge(item.price_cents, false);
    button.appendChild(head);

    button.appendChild(el('p', 'yy-market-card-summary', item.summary || '作者还没有写简介。'));

    var tags = el('div', 'yy-market-tags');
    (item.tags || []).slice(0, 4).forEach(function (tag) {
      tags.appendChild(el('span', 'yy-market-tag', tag));
    });
    if ((item.tags || []).length > 4) tags.appendChild(el('span', 'yy-market-tag', '+' + (item.tags.length - 4)));
    button.appendChild(tags);

    var stats = el('div', 'yy-market-card-stats');
    stats.appendChild(el('span', null, '安装 ' + Number(item.installs || 0)));
    stats.appendChild(el('span', null, '下载 ' + Number(item.downloads || 0)));
    var ratingBox = el('span');
    ratingBox.innerHTML = ratingStars(item);
    stats.appendChild(ratingBox);
    if (item.latest_version) stats.appendChild(el('span', null, 'v' + item.latest_version));
    button.appendChild(stats);
    return button;
  }

  async function loadList() {
    var host = document.getElementById('yy-market-list');
    if (!host) return;
    var my = ++epoch;
    var params = '?q=' + encodeURIComponent(state.q) + '&category=' + encodeURIComponent(state.category) +
      '&sort=' + encodeURIComponent(state.sort) + '&page=' + state.page + '&page_size=' + state.pageSize;
    if (state.mc_version) params += '&mc_version=' + encodeURIComponent(state.mc_version);
    if (state.loader) params += '&loader=' + encodeURIComponent(state.loader);
    var data = await A('GET', '/api/market/plugins' + params);
    if (my !== epoch || !document.getElementById('yy-market-list')) return;
    if (!data.ok) {
      showListMessage(data.error || '读取市场失败');
      return;
    }
    state.total = Number(data.total || 0);
    renderChips(data.categories || {});
    host.replaceChildren();
    var items = data.items || [];
    if (!items.length) {
      showListMessage(state.q ? '没有找到匹配的插件' : '还没有插件');
      renderPager();
      return;
    }
    items.forEach(function (item) { host.appendChild(card(item)); });
    renderPager();
  }

  function renderPager() {
    var host = document.getElementById('yy-market-pager');
    if (!host) return;
    host.replaceChildren();
    var pages = Math.max(1, Math.ceil(state.total / state.pageSize));
    var prev = el('button', 'btn-s', '上一页');
    prev.type = 'button';
    prev.disabled = state.page <= 1;
    prev.onclick = function () { if (state.page > 1) { state.page--; loadList(); } };
    var next = el('button', 'btn-s', '下一页');
    next.type = 'button';
    next.disabled = state.page >= pages;
    next.onclick = function () { if (state.page < pages) { state.page++; loadList(); } };
    host.append(prev, el('span', 'yy-market-page-info', '第 ' + state.page + ' / ' + pages + ' 页 · 共 ' + state.total + ' 个'), next);
  }

  function paidEconHtml(p) {
    if (Number(p.price_cents) <= 0) return '';
    var gross = Number(p.price_cents);
    var netCents = Math.floor(gross * 0.7);
    var feeCents = gross - netCents;
    return '<div class="yy-market-econ">' +
      '<h4>购买与分成</h4>' +
      '<p>售价 <strong>' + esc(money(gross)) + '</strong>，平台统一收取 30% 服务费（<strong>' + esc(money(feeCents)) +
      '</strong>），开发者实收 70%（<strong>' + esc(money(netCents)) + '</strong>）。</p>' +
      '<p class="yy-market-split-note">支付通道尚未接入，付费插件暂不能购买或安装；接入后本说明即为实际分成比例，金额按分为单位结算。</p>' +
      '</div>';
  }

  function versionHistoryHtml(versions) {
    if (!versions || !versions.length) return '';
    var rows = versions.map(function (v) {
      var date = (v.created_at || '').replace('T', ' ').slice(0, 16) || '—';
      return '<li><div class="yy-market-vh-head"><strong>v' + esc(v.version) + '</strong>' +
        '<span>' + esc(date) + '</span>' +
        '<span>' + esc(sizeText(v.size_bytes)) + '</span>' +
        (v.loader ? '<span>' + esc(v.loader) + '</span>' : '') + '</div>' +
        (v.changelog ? '<p>' + esc(v.changelog) + '</p>' : '') + '</li>';
    }).join('');
    return '<div class="yy-market-vh"><h4>版本历史</h4><ul>' + rows + '</ul></div>';
  }

  function detailHtml(p) {
    var shots = (p.screenshots || []).map(function (src) {
      return '<img src="' + esc(src) + '" alt="' + esc(p.name) + ' 截图" loading="lazy" onerror="this.remove()">';
    }).join('');
    var tags = (p.tags || []).map(function (tag) { return '<span class="yy-market-tag">' + esc(tag) + '</span>'; }).join('');
    var versions = p.versions || [];
    var options = versions.map(function (v) {
      return '<option value="' + esc(v.version) + '"' + (v.version === p.latest_version ? ' selected' : '') + '>v' + esc(v.version) + ' · ' + esc(sizeText(v.size_bytes)) + '</option>';
    }).join('');
    var latest = versions.find(function (v) { return v.version === p.latest_version; }) || versions[0] || {};
    var install;
    if (Number(p.price_cents) > 0) {
      install = '<button type="button" class="btn yy-market-buy" disabled aria-disabled="true">支付未接入</button>' +
        '<p class="yy-hint">付费插件暂不能购买或安装；平台支付接入后开放。售价 ' + esc(money(p.price_cents)) + '，其中 70% 归开发者。</p>';
    } else {
      install = '<button type="button" class="btn" id="yy-market-install" onclick="yyMarketInstall(\'' + esc(p.id) + '\')">安装到本机</button>' +
        '<p class="yy-hint">免费插件直接安装到本机插件目录，安装后到「插件中心」启用。</p>';
    }
    var meta = [
      ['最新版本', p.latest_version ? 'v' + p.latest_version : '—'],
      ['体积', latest.size_bytes ? sizeText(latest.size_bytes) : '—'],
      ['许可证', p.license || '未填写'],
      ['安装量', Number(p.installs || 0)],
      ['评分', ratingText(p)],
      ['MC 版本', (p.mc_versions || []).join(' / ') || '未标注'],
      ['加载器', (p.loaders || []).join(' / ') || '未标注'],
      ['SHA-256', latest.sha256 ? String(latest.sha256).slice(0, 16) + '…' : '—']
    ].map(function (row) {
      return '<div><span>' + esc(row[0]) + '</span><strong>' + esc(row[1]) + '</strong></div>';
    }).join('');
    return '<div class="yy-market-detail">' +
      '<div class="yy-market-detail-head">' +
      '<div class="yy-market-icon is-large">' + icon('plugin') + '</div>' +
      '<div class="yy-market-detail-title"><h2>' + esc(p.name) + '</h2>' +
      '<p>' + esc(p.author || '未知作者') + ' · ' + esc(CATEGORY_LABELS[p.category] || '其他') + '</p>' +
      ratingStars(p) + '</div>' +
      priceBadge(p.price_cents, true) + '</div>' +
      '<p class="yy-market-detail-summary">' + esc(p.summary || '作者还没有写简介。') + '</p>' +
      '<div class="yy-market-author-card">' +
      '<div class="yy-market-icon">' + icon('user') + '</div>' +
      '<div class="yy-market-author-card-info"><strong>' + esc(p.author || '未知作者') + '</strong>' +
      '<span>开发者</span>' +
      '<p class="yy-market-author-card-note">' + (p.author_name ? '平台账号：' + esc(p.author_name) : '作者简介：开发者暂未填写（占位）') + '</p></div>' +
      '<div class="yy-market-author-card-right">' + priceBadge(p.price_cents, false) + '</div>' +
      '</div>' +
      '<div class="yy-market-tags">' + tags + '</div>' +
      (shots ? '<div class="yy-market-shots" aria-label="插件截图">' + shots + '</div>' : '') +
      '<div class="yy-market-meta">' + meta + '</div>' +
      '<div class="yy-market-detail-text">' + esc(p.description || '作者还没有填写详情。') + '</div>' +
      paidEconHtml(p) +
      '<section class="yy-market-version-box"><h3>版本与安装</h3>' +
      (versions.length ? '<label>选择版本<select id="yy-market-version" onchange="yyMarketVersionChange(this.value)">' + options + '</select></label>' : '<p class="yy-hint">还没有可安装的版本。</p>') +
      '<div class="yy-market-changelog"><h4>更新日志</h4><pre id="yy-market-changelog">' + esc(latest.changelog || '暂无更新日志。') + '</pre></div>' +
      versionHistoryHtml(versions) +
      '<div class="yy-market-actions">' + install + '</div></section></div>';
  }

  async function openDetail(id) {
    var my = ++epoch;
    var data = await A('GET', '/api/market/plugins/' + encodeURIComponent(id));
    if (my !== epoch) return;
    if (!data.ok) {
      T(data.error || '插件不存在', 'e');
      return;
    }
    var p = data.plugin;
    state.versions = {};
    (p.versions || []).forEach(function (v) { state.versions[v.version] = v.changelog || ''; });
    delete subPageCache[DETAIL_TITLE];
    openSubPage(DETAIL_TITLE, detailHtml(p));
    subPageInit(DETAIL_TITLE, function () {
      var select = document.getElementById('yy-market-version');
      if (select) state.versions[select.value] = state.versions[select.value] || '';
    });
  }

  function renderEarn(summary) {
    var host = document.getElementById('yy-market-earn');
    if (!host) return;
    summary = summary || {};
    var gross = Number(summary.gross_cents || 0);
    var fee = Number(summary.platform_fee_cents || 0);
    var net = Number(summary.developer_net_cents || 0);
    host.innerHTML = '<h3>收益</h3>' +
      '<div class="yy-market-money">' +
      '<div><span>总收入</span><strong>' + esc(money(gross)) + '</strong></div>' +
      '<div><span>平台服务费 30%</span><strong>' + esc(money(fee)) + '</strong></div>' +
      '<div><span>开发者实收 70%</span><strong>' + esc(money(net)) + '</strong></div>' +
      '<div><span>安装量</span><strong>' + Number(summary.installs || 0) + '</strong></div>' +
      '</div><p class="yy-market-split-note">平台服务费 30%，开发者实收 70%。金额按实际支付金额计算，以分为单位；没有订单时显示 0，不使用演示数据。</p>';
  }

  function mineRow(p) {
    var row = el('div', 'yy-market-mine-row');
    var main = el('div', 'yy-market-mine-main');
    var title = el('div', 'yy-market-mine-title');
    title.innerHTML = '<strong>' + esc(p.name) + '</strong>' + statusBadge(p.status);
    main.appendChild(title);
    main.appendChild(el('p', 'yy-market-mine-meta',
      (CATEGORY_LABELS[p.category] || '其他') + ' · ' +
      (Number(p.price_cents) > 0 ? money(p.price_cents) : '免费') + ' · 安装 ' + Number(p.installs || 0) +
      ' · ' + (p.versions || []).length + ' 个版本' +
      (p.latest_version ? ' · 最新 v' + p.latest_version : '')));
    main.appendChild(el('p', 'yy-market-mine-summary', p.summary || '还没有写简介，编辑资料补充后更容易被玩家找到。'));
    if (p.status === 'rejected' && p.reject_reason) {
      main.appendChild(el('p', 'yy-market-reject', '驳回理由：' + p.reject_reason));
    }
    if (p.developer_net_cents != null) {
      main.appendChild(el('p', 'yy-market-mine-meta',
        '实收 ' + money(p.developer_net_cents) + ' · 平台 ' + money(p.platform_fee_cents)));
    }
    var actions = el('div', 'yy-market-mine-actions');
    var view = el('button', 'btn-s', '查看');
    view.type = 'button';
    view.onclick = function () { window.yyMarketView(p.id); };
    var edit = el('button', 'btn-s', '编辑资料');
    edit.type = 'button';
    edit.disabled = p.status === 'pending' || p.status === 'approved';
    edit.onclick = function () { window.yyMarketEdit(p.id); };
    var submit = el('button', 'btn-s', '提交审核');
    submit.type = 'button';
    submit.disabled = p.status === 'pending' || p.status === 'approved';
    submit.onclick = function () { window.yyMarketSubmit(p.id); };
    var upload = el('button', 'btn-s', '上传版本');
    upload.type = 'button';
    upload.disabled = p.status === 'pending';
    upload.onclick = function () { window.yyMarketPickUpload(p.id); };
    actions.append(view, edit, submit, upload);
    row.append(main, actions);
    return row;
  }

  function fillUploadSelect() {
    var select = document.getElementById('yy-market-u-plugin');
    if (!select) return;
    var current = select.value;
    select.replaceChildren();
    state.mine.forEach(function (p) {
      var option = el('option', null, p.name + ' · ' + (STATUS_LABELS[p.status] || p.status));
      option.value = p.id;
      select.appendChild(option);
    });
    if (current && state.mine.some(function (p) { return p.id === current; })) select.value = current;
  }

  async function loadDev() {
    var my = ++epoch;
    var data = await A('GET', '/api/market/mine');
    if (my !== epoch || !document.getElementById('yy-market-earn')) return;
    var hint = document.getElementById('yy-market-dev-hint');
    var forms = document.getElementById('yy-market-forms');
    if (!data.ok) {
      if (hint) {
        hint.hidden = false;
        hint.textContent = data.error || '开发者中心需要登录游园账号。';
      }
      if (forms) forms.hidden = true;
      var earn = document.getElementById('yy-market-earn');
      if (earn) earn.innerHTML = '<h3>收益</h3><p class="yy-hint">登录后可查看收益。</p>';
      var mine = document.getElementById('yy-market-mine');
      if (mine) mine.replaceChildren(el('p', 'yy-hint', '登录后可管理自己发布的插件。'));
      return;
    }
    if (hint) hint.hidden = true;
    if (forms) forms.hidden = false;
    state.mine = data.items || [];
    renderEarn(data.summary || {});
    var list = document.getElementById('yy-market-mine');
    if (list) {
      list.replaceChildren();
      if (!state.mine.length) {
        list.appendChild(el('p', 'yy-hint', '还没有发布插件，先在下面创建草稿。'));
      } else {
        state.mine.forEach(function (p) { list.appendChild(mineRow(p)); });
      }
    }
    fillUploadSelect();
    yyMarketSyncPriceMode();
  }

  function resetForm() {
    state.editId = '';
    ['name', 'summary', 'description', 'tags', 'license', 'mc', 'loader', 'shots'].forEach(function (key) {
      var node = document.getElementById('yy-market-f-' + key);
      if (node) node.value = '';
    });
    var price = document.getElementById('yy-market-f-price');
    if (price) price.value = '0';
    yyMarketSyncPriceMode();
    var category = document.getElementById('yy-market-f-category');
    if (category) category.value = 'tool';
    var title = document.getElementById('yy-market-form-title');
    if (title) title.textContent = '新建插件';
    var save = document.getElementById('yy-market-save');
    if (save) save.textContent = '保存草稿';
  }

  function readForm() {
    var priceText = (document.getElementById('yy-market-f-price') || {}).value || '0';
    var price = Number(priceText);
    if (!Number.isFinite(price) || price < 0) throw new Error('价格必须是不小于 0 的数字');
    var cents = Math.round(price * 100);
    if (cents > 100000000) throw new Error('价格不能超过 100 万元');
    return {
      name: (document.getElementById('yy-market-f-name') || {}).value || '',
      summary: (document.getElementById('yy-market-f-summary') || {}).value || '',
      description: (document.getElementById('yy-market-f-description') || {}).value || '',
      category: (document.getElementById('yy-market-f-category') || {}).value || 'other',
      tags: listOf((document.getElementById('yy-market-f-tags') || {}).value),
      price_cents: cents,
      license: (document.getElementById('yy-market-f-license') || {}).value || '',
      screenshots: linesOf((document.getElementById('yy-market-f-shots') || {}).value),
      mc_versions: listOf((document.getElementById('yy-market-f-mc') || {}).value),
      loaders: listOf((document.getElementById('yy-market-f-loader') || {}).value)
    };
  }

  function fillForm(p) {
    state.editId = p.id;
    var set = function (key, value) {
      var node = document.getElementById('yy-market-f-' + key);
      if (node) node.value = value == null ? '' : value;
    };
    set('name', p.name);
    set('summary', p.summary);
    set('description', p.description);
    set('tags', (p.tags || []).join(', '));
    set('license', p.license);
    set('mc', (p.mc_versions || []).join(', '));
    set('loader', (p.loaders || []).join(', '));
    set('shots', (p.screenshots || []).join('\n'));
    var category = document.getElementById('yy-market-f-category');
    if (category) category.value = p.category || 'other';
    var price = document.getElementById('yy-market-f-price');
    if (price) price.value = (Number(p.price_cents || 0) / 100).toFixed(2);
    yyMarketSyncPriceMode();
    var title = document.getElementById('yy-market-form-title');
    if (title) title.textContent = '编辑：' + p.name;
    var save = document.getElementById('yy-market-save');
    if (save) save.textContent = '保存修改';
    var card = document.getElementById('yy-market-form-card');
    if (card && card.scrollIntoView) card.scrollIntoView({ block: 'start', behavior: 'smooth' });
  }

  function fileToDataURL(file) {
    return new Promise(function (resolve, reject) {
      var reader = new FileReader();
      reader.onload = function () { resolve(String(reader.result || '')); };
      reader.onerror = function () { reject(new Error('读取文件失败')); };
      reader.readAsDataURL(file);
    });
  }

  window.yyMarketPage = function () {
    state.tab = 'market';
    state.page = 1;
    epoch++;
    delete subPageCache[PAGE_TITLE];
    openSubPage(PAGE_TITLE, shell());
    subPageInit(PAGE_TITLE, paint);
    paint();
  };

  window.yyMarketTab = function (tab) {
    state.tab = tab === 'dev' ? 'dev' : 'market';
    state.page = 1;
    epoch++;
    delete subPageCache[PAGE_TITLE];
    openSubPage(PAGE_TITLE, shell());
    subPageInit(PAGE_TITLE, paint);
    paint();
  };

  window.yyMarketOpen = function (id) {
    if (id) openDetail(id);
  };

  window.yyMarketSearch = function (value) {
    state.q = String(value || '');
    state.page = 1;
    clearTimeout(searchTimer);
    searchTimer = setTimeout(loadList, 250);
  };

  window.yyMarketSetSort = function (sort) {
    if (['hot', 'new', 'price'].indexOf(sort) < 0) return;
    state.sort = sort;
    state.page = 1;
    var buttons = document.querySelectorAll('.yy-market-sort-btn');
    for (var i = 0; i < buttons.length; i++) {
      buttons[i].classList.toggle('is-on', buttons[i].dataset.sort === sort);
    }
    loadList();
  };

  window.yyMarketSetFilter = function (key, value) {
    if (key === 'mc_version') {
      state.mc_version = value || '';
    } else if (key === 'loader') {
      state.loader = value || '';
    } else {
      return;
    }
    state.page = 1;
    loadList();
  };

  window.yyMarketVersionChange = function (version) {
    var node = document.getElementById('yy-market-changelog');
    if (node) node.textContent = state.versions[version] || '暂无更新日志。';
  };

  window.yyMarketInstall = async function (id) {
    var button = document.getElementById('yy-market-install');
    var select = document.getElementById('yy-market-version');
    var version = select ? select.value : '';
    if (button) {
      button.disabled = true;
      button.textContent = '安装中…';
    }
    var data = await A('POST', '/api/market/plugins/' + encodeURIComponent(id) + '/install', { version: version });
    if (!data.ok) {
      T(data.error || '安装失败', 'e');
      if (button) {
        button.disabled = false;
        button.textContent = '安装到本机';
      }
      return;
    }
    T('已安装到本机插件目录', 'o');
    if (button) {
      button.textContent = '已安装';
      button.disabled = true;
    }
    var meta = document.querySelector('.yy-market-meta');
    if (meta) {
      var rows = meta.querySelectorAll('div');
      for (var i = 0; i < rows.length; i++) {
        if (rows[i].firstChild && rows[i].firstChild.textContent === '安装量') {
          var strong = rows[i].querySelector('strong');
          if (strong) strong.textContent = String(Number(strong.textContent || 0) + 1);
        }
      }
    }
  };

  window.yyMarketRefreshDev = function () { loadDev(); };

  window.yyMarketResetForm = function () { resetForm(); };

  var lastPaidPrice = '1';
  function yyMarketSyncPriceMode() {
    var price = document.getElementById('yy-market-f-price');
    var paid = document.getElementById('yy-market-f-paid');
    var freeBtn = document.getElementById('yy-market-f-free');
    var isFree = !price || Number(price.value) === 0;
    if (price) price.disabled = isFree;
    if (paid) paid.classList.toggle('is-on', !isFree);
    if (freeBtn) freeBtn.classList.toggle('is-on', isFree);
    yyMarketPreviewSplit();
  }
  function yyMarketPreviewSplit() {
    var host = document.getElementById('yy-market-split');
    if (!host) return;
    var price = document.getElementById('yy-market-f-price');
    var cents = Math.round(Number((price && price.value) || 0) * 100);
    if (cents <= 0) {
      host.innerHTML = '<span class="yy-market-split-free">免费</span> 用户无需付费，开发者实收 ¥0.00（100%）。';
      return;
    }
    var netCents = Math.floor(cents * 0.7);
    var feeCents = cents - netCents;
    host.innerHTML = '用户支付 <strong>' + esc(money(cents)) + '</strong> · 平台收取 30%（<strong>' +
      esc(money(feeCents)) + '</strong>） · 开发者实收 70%（<strong>' + esc(money(netCents)) + '</strong>）';
  }
  window.yyMarketSyncPriceMode = yyMarketSyncPriceMode;
  window.yyMarketSetFree = function (free) {
    var price = document.getElementById('yy-market-f-price');
    if (price) {
      if (free) { lastPaidPrice = price.value || '1'; price.value = '0'; }
      else { price.value = (Number(lastPaidPrice) > 0) ? lastPaidPrice : '1'; }
    }
    yyMarketSyncPriceMode();
  };
  window.yyMarketView = function (id) { if (id) openDetail(id); };

  window.yyMarketEdit = function (id) {
    var plugin = state.mine.find(function (p) { return p.id === id; });
    if (plugin) fillForm(plugin);
  };

  window.yyMarketPickUpload = function (id) {
    var select = document.getElementById('yy-market-u-plugin');
    if (select && id) select.value = id;
    var card = document.getElementById('yy-market-upload-card');
    if (card && card.scrollIntoView) card.scrollIntoView({ block: 'start', behavior: 'smooth' });
    var version = document.getElementById('yy-market-u-version');
    if (version) version.focus();
  };

  window.yyMarketSavePlugin = async function () {
    var button = document.getElementById('yy-market-save');
    var payload;
    try {
      payload = readForm();
      if (String(payload.name).trim().length < 2) throw new Error('插件名称至少 2 个字符');
    } catch (error) {
      T(error.message, 'e');
      return;
    }
    if (button) { button.disabled = true; button.textContent = '保存中…'; }
    var path = state.editId ? '/api/market/plugins/' + encodeURIComponent(state.editId) + '/edit' : '/api/market/plugins';
    var data = await A('POST', path, payload);
    if (button) { button.disabled = false; button.textContent = state.editId ? '保存修改' : '保存草稿'; }
    if (!data.ok) {
      T(data.error || '保存失败', 'e');
      return;
    }
    T(state.editId ? '修改已保存' : '草稿已创建，接着上传版本', 'o');
    resetForm();
    loadDev();
  };

  window.yyMarketSubmit = async function (id) {
    var data = await A('POST', '/api/market/plugins/' + encodeURIComponent(id) + '/submit', {});
    if (!data.ok) {
      T(data.error || '提交失败', 'e');
      return;
    }
    T('已提交审核', 'o');
    loadDev();
  };

  window.yyMarketUpload = async function () {
    var button = document.getElementById('yy-market-upload');
    var status = document.getElementById('yy-market-upload-status');
    var pluginSelect = document.getElementById('yy-market-u-plugin');
    var fileInput = document.getElementById('yy-market-u-file');
    var versionInput = document.getElementById('yy-market-u-version');
    if (!pluginSelect || !pluginSelect.value) {
      T('先选择一个插件', 'e');
      return;
    }
    if (!versionInput || !String(versionInput.value).trim()) {
      T('填写版本号', 'e');
      return;
    }
    var file = fileInput && fileInput.files && fileInput.files[0];
    if (!file) {
      T('选择插件包文件', 'e');
      return;
    }
    if (file.size > 20 * 1024 * 1024) {
      T('文件超过 20 MB', 'e');
      return;
    }
    if (button) { button.disabled = true; button.textContent = '上传中…'; }
    if (status) status.textContent = '正在读取并上传 ' + file.name + '…';
    try {
      var dataURL = await fileToDataURL(file);
      var payload = {
        version: String(versionInput.value).trim(),
        changelog: (document.getElementById('yy-market-u-changelog') || {}).value || '',
        loader: (document.getElementById('yy-market-u-loader') || {}).value || '',
        mc_versions: listOf((document.getElementById('yy-market-u-mc') || {}).value),
        file_name: file.name,
        content_base64: dataURL
      };
      var data = await A('POST', '/api/market/plugins/' + encodeURIComponent(pluginSelect.value) + '/versions', payload);
      if (!data.ok) throw new Error(data.error || '上传失败');
      if (status) status.textContent = '已上传 v' + payload.version + '，SHA-256 ' + String((data.version && data.version.sha256) || '').slice(0, 16) + '…';
      T('版本已上传', 'o');
      if (versionInput) versionInput.value = '';
      var changelog = document.getElementById('yy-market-u-changelog');
      if (changelog) changelog.value = '';
      if (fileInput) fileInput.value = '';
      loadDev();
    } catch (error) {
      if (status) status.textContent = '';
      T(error.message || '上传失败', 'e');
    } finally {
      if (button) { button.disabled = false; button.textContent = '上传版本'; }
    }
  };

  if (typeof yyModules !== 'undefined' && Array.isArray(yyModules) && !yyModules.some(function (m) { return m.id === 'market'; })) {
    yyModules.push({
      id: 'market',
      name: '插件市场',
      desc: '浏览、安装与发布插件',
      category: 'automation',
      icon: 'plugin',
      open: function () { return window.yyMarketPage(); }
    });
    if (typeof yyAllModules !== 'undefined' && yyAllModules !== yyModules && !yyAllModules.some(function (m) { return m.id === 'market'; })) {
      yyAllModules.push(yyModules[yyModules.length - 1]);
    }
    if (typeof yyPrefs === 'object' && yyPrefs && Array.isArray(yyPrefs.order) && yyPrefs.order.indexOf('market') < 0) {
      yyPrefs.order.push('market');
    }
  }

  // 用户 2026-10-24 第 5 条要求：市场用我们自己的，不再走第三方 Prism 代理。
  // 两个旧入口（导入弹窗的「从插件市场获取」、插件工作室的 market/upload 按钮）
  // 都在更早的脚本里定义，这里统一改成自建市场；旧脚本保持原样不动，
  // 万一本脚本加载失败，旧入口仍能打开（降级而不是报错）。
  // youyuan_settings.js 会把 pmOpenMarket 恢复成第三方版本，但它在本脚本之前执行，
  // 所以最终生效的是这里的覆盖。
  if (typeof window.yyMarketTakeOverEntries !== 'boolean') {
    window.yyMarketTakeOverEntries = true;
    if (typeof pmOpenMarket === 'function') {
      pmOpenMarket = function () { return window.yyMarketPage(); };
    }
    if (typeof pmOpenUpload === 'function') {
      // 上传/发布一律进「开发者中心」：定价、抽成、送审都在那里。
      pmOpenUpload = function () { return window.yyMarketTab('dev'); };
    }
  }
})();
