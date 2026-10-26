// 命令链中心：在 youyuan_settings.js 之后加载，用包装的方式接管旧的 yyChainsPage / yyDrawChains / yyChainsRefresh。
// 旧页面的问题：列表只渲染接口返回的 files，而接口只遍历内存里已注册的链，
// 启动后才放进目录的 TXT 不出现；并且没有删除入口。这里换成“一行一个磁盘文件”的直接列表。
(function () {
  'use strict';

  var DIRECTORY = '插件数据文件/ds_ai_agent/command_txt/';
  var rows = [];          // 当前列表数据，和全局 yyChainRows 保持同一份，yyChainEdit 等旧函数还在读它
  var warnings = [];
  var loading = false;

  function esc(value) {
    if (typeof yyEscape === 'function') return yyEscape(value == null ? '' : value);
    return String(value == null ? '' : value).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function num(value) {
    var n = Number(value);
    return isFinite(n) ? n : 0;
  }
  function nameOf(row) {
    return String((row && row.config && row.config.name) || '');
  }
  function fmtSize(bytes) {
    var n = num(bytes);
    if (n <= 0) return '大小未知';
    if (n < 1024) return n + ' B';
    if (n < 1024 * 1024) return (n / 1024).toFixed(n < 10240 ? 1 : 0) + ' KB';
    return (n / 1024 / 1024).toFixed(1) + ' MB';
  }
  function fmtTime(iso) {
    if (!iso) return '时间未知';
    var t = Date.parse(iso);
    if (!t) return '时间未知';
    var diff = Math.max(0, Date.now() - t);
    var minutes = Math.floor(diff / 60000);
    if (minutes < 1) return '刚刚修改';
    if (minutes < 60) return minutes + ' 分钟前修改';
    var hours = Math.floor(minutes / 60);
    if (hours < 24) return hours + ' 小时前修改';
    var days = Math.floor(hours / 24);
    if (days < 30) return days + ' 天前修改';
    var d = new Date(t);
    return (d.getMonth() + 1) + ' 月 ' + d.getDate() + ' 日修改';
  }
  function hzOf(cfg) {
    var hz = num(cfg && cfg.hz);
    if (hz > 0) return hz;
    var interval = num(cfg && cfg.interval_ms) || 1000;
    return 1000 / interval;
  }
  function fmtHz(hz) {
    if (!hz || !isFinite(hz)) return '0 Hz';
    return (hz >= 10 ? hz.toFixed(0) : hz.toFixed(2)) + ' Hz';
  }
  // 四种视觉状态：运行中（绿）、等待连接（绿，略淡）、出错（红）、未启动/已停止（灰）。
  function statusOf(row) {
    var state = String((row && row.state) || '');
    var error = String((row && row.error) || '');
    if (row && row.missing) return { key: 'err', text: '文件已丢失' };
    if (state.indexOf('错误') >= 0) return { key: 'err', text: state };
    if (state === '运行中') return { key: 'run', text: '运行中' };
    if (state === '等待连接') return { key: 'run', text: '等待连接' };
    if (error) return { key: 'err', text: state || '出错' };
    if (state === '已完成') return { key: 'done', text: '已完成' };
    if (row && row.loaded === false) return { key: 'idle', text: '未启动' };
    return { key: 'idle', text: state || '未启动' };
  }
  function isRunning(row) {
    var key = statusOf(row).key;
    return key === 'run';
  }

  function rowHTML(row) {
    var name = nameOf(row);
    var status = statusOf(row);
    var cfg = row.config || {};
    var running = isRunning(row);
    var meta = [
      num(row.commands) + ' 条命令',
      fmtHz(hzOf(cfg)),
      num(cfg.delay_ms) + ' ms/命令',
      fmtSize(row.size_bytes),
      fmtTime(row.modified_at)
    ].join(' · ');
    var runtime = '';
    if (running) {
      runtime = '<div class="yycc-runtime">尝试 ' + num(row.executed) +
        ' · 提交 ' + num(row.submitted) +
        ' · 跳过 ' + num(row.skipped) +
        ' · 完成 ' + num(row.cycles) + ' 轮' +
        ' · 实际 ' + num(row.actual_hz).toFixed(2) + ' 轮/秒</div>';
    }
    var actions = running
      ? '<button class="yycc-act" data-act="stop">停止</button>'
      : '<button class="yycc-act is-primary" data-act="start">启动</button>';
    actions += '<button class="yycc-act" data-act="edit">编辑</button>' +
      '<button class="yycc-act is-danger" data-act="delete">删除</button>';
    return '<article class="yycc-row is-' + status.key + '" data-name="' + esc(name) + '">' +
      '<span class="yycc-strip" aria-hidden="true"></span>' +
      '<div class="yycc-body">' +
        '<div class="yycc-top"><span class="yycc-name">' + esc(name) + '</span>' +
          '<span class="yycc-badge is-' + status.key + '">' + esc(status.text) + '</span></div>' +
        '<div class="yycc-meta">' + esc(meta) + '</div>' +
        runtime +
        (row.error ? '<pre class="yycc-error-text">' + esc(row.error) + '</pre>' : '') +
        '<div class="yycc-actions">' + actions + '</div>' +
      '</div>' +
    '</article>';
  }

  function emptyHTML() {
    return '<div class="yycc-empty">' +
      '<strong>命令链目录还是空的</strong>' +
      '<p>放一个 TXT 进去、或者直接在这里新建一条链，保存后就能启动。</p>' +
      '<button class="yycc-act is-primary" onclick="yyChainEdit()">新建命令链</button>' +
    '</div>';
  }

  function overviewHTML() {
    var running = rows.filter(isRunning).length;
    return '<section class="yycc-overview">' +
      '<div class="yycc-overview-main">' +
        '<div class="yycc-overview-label">命令链文件</div>' +
        '<div class="yycc-overview-count"><b>' + rows.length + '</b> 个文件 · <b>' + running + '</b> 个运行中</div>' +
        '<div class="yycc-overview-path">目录：' + esc(DIRECTORY) + '</div>' +
      '</div>' +
      '<button class="yycc-refresh" onclick="yyChainCenterRefresh()">刷新</button>' +
    '</section>';
  }

  function renderWarnings() {
    var host = document.getElementById('yycc-warnings');
    if (!host) return;
    if (!warnings.length) { host.innerHTML = ''; return; }
    host.innerHTML = '<div class="yycc-warn"><strong>有 ' + warnings.length + ' 个文件没能读取</strong><ul>' +
      warnings.map(function (item) { return '<li>' + esc(item) + '</li>'; }).join('') + '</ul></div>';
  }

  function showError(message) {
    var text = String(message || '操作失败');
    var host = document.getElementById('yycc-notice');
    if (host) host.innerHTML = '<div class="yycc-notice is-error">' + esc(text) + '</div>';
    if (typeof T === 'function') T(text, 'e');
  }
  function clearNotice() {
    var host = document.getElementById('yycc-notice');
    if (host) host.innerHTML = '';
  }

  var boundHost = null;   // 记住已经绑过事件的那个容器；innerHTML 被还原后元素会换新，必须重新绑

  function bindList(host) {
    if (!host || boundHost === host) return;
    host.addEventListener('click', onListClick);
    boundHost = host;
  }

  function renderList() {
    var host = document.getElementById('yy-chain-list');
    if (!host) return;
    host.innerHTML = rows.length ? rows.map(rowHTML).join('') : emptyHTML();
    bindList(host);
  }

  function renderAll() {
    var overview = document.getElementById('yycc-overview-host');
    if (overview) overview.innerHTML = overviewHTML();
    renderWarnings();
    renderList();
  }

  function onListClick(event) {
    var button = event.target && event.target.closest ? event.target.closest('.yycc-act[data-act]') : null;
    if (!button) return;
    var card = button.closest('.yycc-row');
    if (!card) return;
    var name = card.dataset.name || '';
    if (!name) return;
    var act = button.dataset.act;
    if (act === 'start' || act === 'stop') return runAction(name, act, button);
    if (act === 'edit') return editChain(name);
    if (act === 'delete') return removeChain(name, button);
  }

  function setBusy(card, busy) {
    if (!card) return;
    var buttons = card.querySelectorAll('.yycc-act');
    for (var i = 0; i < buttons.length; i++) buttons[i].disabled = !!busy;
    card.classList.toggle('is-busy', !!busy);
  }

  async function load(showSkeleton) {
    if (loading) return;
    loading = true;
    var host = document.getElementById('yy-chain-list');
    if (showSkeleton && host && !rows.length) host.innerHTML = '<div class="yycc-loading">正在读取命令链目录…</div>';
    var result = await A('GET', '/api/youyuan/chains');
    loading = false;
    // 请求回来时页面可能已经切走（宿主元素被换掉），此时不要再动 DOM。
    if (!host || !document.body.contains(host)) return;
    if (!result || !result.ok) {
      var message = (result && result.error) || '读取命令链列表失败';
      showError(message);
      // 首载失败时列表里还是骨架屏文案，会永远停在那里像卡死。
      // 有数据就保留旧列表（不清空），没数据就换成错误 + 重试。
      if (!rows.length) {
        host.innerHTML = '<div class="yycc-notice is-error">' + esc(message) +
          '<button type="button" class="yycc-act" style="margin-left:8px" onclick="yyChainsRefresh()">重试</button></div>';
      }
      return;
    }
    clearNotice();
    rows = Array.isArray(result.files) ? result.files : [];
    warnings = Array.isArray(result.warnings) ? result.warnings : [];
    window.yyChainRows = rows;                                       // 旧函数（编辑页、插件 API）仍在读这个全局
    renderAll();
  }

  async function runAction(name, action, button) {
    var card = button ? button.closest('.yycc-row') : null;
    setBusy(card, true);
    var result = await A('POST', '/api/youyuan/chains', { action: action, name: name });
    setBusy(card, false);
    if (!result || !result.ok) {
      showError((result && result.error) || (action === 'start' ? '启动失败' : '停止失败'));
      return;
    }
    clearNotice();
    if (typeof T === 'function') T(action === 'start' ? '已启动：' + name : '已停止：' + name, 'o');
    await load(false);
  }

  function editChain(name) {
    if (typeof yyChainEdit === 'function') return yyChainEdit(name);
  }

  async function removeChain(name, button) {
    // 二次确认：删除会直接移走磁盘文件，不可恢复。
    if (!window.confirm('删除「' + name + '」？文件会从磁盘移除，不能恢复。')) return;
    var card = button ? button.closest('.yycc-row') : null;
    setBusy(card, true);
    var result = await A('POST', '/api/youyuan/chains', { action: 'delete', name: name });
    if (!result || !result.ok) {
      setBusy(card, false);
      showError((result && result.error) || '删除失败');
      return;
    }
    clearNotice();
    if (typeof T === 'function') T('已删除：' + name, 'o');
    // 本地移除该行并刷新计数，不整页重绘，避免闪烁。
    rows = rows.filter(function (row) { return nameOf(row) !== name; });
    window.yyChainRows = rows;
    if (card && card.parentNode) card.parentNode.removeChild(card);
    var updated = document.getElementById('yycc-overview-host');
    if (updated) updated.innerHTML = overviewHTML();
    if (!rows.length) renderList();
  }

  window.yyChainCenterRefresh = function () {
    return load(true);
  };

  window.yyChainCenterStopAll = async function () {
    var running = rows.filter(isRunning).map(nameOf);
    if (!running.length) { if (typeof T === 'function') T('当前没有运行中的命令链', 'o'); return; }
    if (!window.confirm('停止全部 ' + running.length + ' 条运行中的命令链？')) return;
    var failed = [];
    for (var i = 0; i < running.length; i++) {
      var result = await A('POST', '/api/youyuan/chains', { action: 'stop', name: running[i] });
      if (!result || !result.ok) failed.push(running[i] + '：' + ((result && result.error) || '停止失败'));
    }
    if (failed.length) showError(failed.join('；'));
    else if (typeof T === 'function') T('已停止 ' + running.length + ' 条命令链', 'o');
    await load(false);
  };

  // 接管旧页面：旧函数是全局 function 声明，直接覆盖同名全局属性即可，
  // 其他文件里的 yyChainsPage() / yyChainsRefresh() 调用会在运行时取到这里。
  window.yyChainsPage = async function () {
    if (typeof subPageCache !== 'undefined' && subPageCache) delete subPageCache['命令链条'];
    var shell = (typeof yyIntro === 'function' ? yyIntro('命令链条', '一个文件一行，直接启动、停止、编辑或删除；列表就是磁盘目录本身。') : '') +
      '<div id="yycc-notice"></div>' +
      '<div class="yycc">' +
        '<div id="yycc-overview-host"></div>' +
        '<div class="yycc-toolbar">' +
          '<button class="yycc-act is-primary" onclick="yyChainEdit()">新建命令链</button>' +
          '<button class="yycc-act" onclick="yyChainUpload()">导入 TXT</button>' +
          '<button class="yycc-act" onclick="yyCreateWorkspaceDialog(\'mcfunction\')">AI 帮写</button>' +
          '<button class="yycc-act is-quiet" onclick="yyChainCenterStopAll()">全部停止</button>' +
        '</div>' +
        '<div id="yycc-warnings"></div>' +
        '<div id="yy-chain-list" class="yycc-list"></div>' +
      '</div>';
    openSubPage('命令链条', shell);
    rows = [];
    warnings = [];
    var host = document.getElementById('yycc-overview-host');
    if (host) host.innerHTML = overviewHTML();
    await load(true);
  };

  // 旧的重绘入口一并接管，避免旧实现把新 DOM 冲掉。
  window.yyDrawChains = function () { renderAll(); };
  window.yyChainsRefresh = async function () { await load(false); };

  // 从“编辑命令链”返回时，closeSubPage 会直接还原 openSubPage 当时缓存的空壳 HTML，
  // 列表是空的、事件也没了——这正是用户说的“返回上一页只显示一个 txt / 什么都没有”。
  // 这里盯住 #sub-page 的内容替换：只要本页重新出现且列表还是空的，就补一次数据。
  function installRevive() {
    if (typeof MutationObserver === 'undefined') return;
    var subPage = document.getElementById('sub-page');
    if (!subPage) return;
    var pending = false;
    new MutationObserver(function () {
      if (pending) return;
      pending = true;
      setTimeout(function () {
        pending = false;
        var title = document.querySelector('.sp-title');
        if (!title || String(title.textContent).trim() !== '命令链条') return;
        var host = document.getElementById('yy-chain-list');
        if (!host || host.children.length) return;
        load(false);
      }, 0);
    }).observe(subPage, { childList: true });
  }
  installRevive();

  if (typeof console !== 'undefined' && console.info) console.info('[命令链中心] 已接管 yyChainsPage / yyDrawChains / yyChainsRefresh');
})();
