/* ═══ 游园资源浏览器 — 全屏工作区（移动端优先） ═══
 *
 * 与参考产品的弹窗式文件选择器刻意区分：
 *  - 不是窄弹窗，而是占满屏幕的工作区（顶部紧凑标题 / 数据来源 / 面包屑 / 搜索排序 / 列表 / 底部选择条）
 *  - 明确区分「执行设备已有文件」与「本机文件」两个数据源，绝不把 Android 本地路径伪装成设备路径
 *  - 行布局：名称主信息，类型 + 大小 + 时间次信息；长名截断但可点「⋯」看全名并复制
 *
 * 保留的既有能力（一个不少）：目录浏览、返回上级、搜索、排序、文件大小、格式识别、
 * 选择/取消、上传到执行设备、皮肤与音乐的虚拟文件夹、QQ 文件入口、读写作用域校验。
 */

function showFilePicker(startPath, filter, onSelect, mode, onCancel) {
  var folderMode = mode === 'folder';
  var overlay = document.createElement('div');
  overlay.className = 'fp-overlay fp-ws';
  var currentPath = startPath === '' ? '' : (startPath || '/storage/emulated/0');
  var scanEpoch = 0, closed = false;
  var pickerOwner = typeof yyReadScope === 'function' ? yyReadScope() : '';
  function pickerLive() { return !closed && (typeof yyReadScope !== 'function' || pickerOwner === yyReadScope()); }
  function closePicker(cancelled) { closed = true; scanEpoch++; overlay.remove(); if (cancelled && onCancel) onCancel(); }
  var pickerEscape = function (s) { return String(s || '').replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); };
  var selectedPath = '';
  var selectedName = '';
  var selectedItem = null;
  var dataList = [];
  var dataParent = '';
  var searchTerm = '';
  var sortMode = 'date_desc';
  var source = 'device';     // device = 执行设备已有文件 / local = 本机文件
  var virtualMode = false;   // 是否在「在线玩家皮肤」「热门歌曲」虚拟文件夹内
  var virtualLabel = '';     // 虚拟文件夹标题（面包屑用）
  var virtualPlayers = [];   // 在线玩家列表 [{name, width, height}]
  var onlinePrefix = 'online://';
  var musicHot = [];         // 服务器热门歌曲列表 [{name, heat}]
  var musicPrefix = 'music://';
  var ROOT_PATH = '/storage/emulated/0';                                                       // 文件选择器根目录
  var QQ_PATH = '/storage/emulated/0/Android/data/com.tencent.mobileqq/Tencent/QQfile_recv/';  // QQ 接收文件夹
  var qqAvailable = false;   // 根目录下是否显示「qq文件」虚拟入口
  var pendingSelectPath = ''; // 上传完成后，等目标目录扫描回来再自动选中该文件

  var sortLabels = {
    name_asc: '名称 A→Z',
    name_desc: '名称 Z→A',
    date_desc: '最新优先',
    date_asc: '最旧优先',
    size_desc: '最大优先',
    size_asc: '最小优先'
  };
  var sortShort = {
    name_asc: '名称↑',
    name_desc: '名称↓',
    date_desc: '最新',
    date_asc: '最早',
    size_desc: '最大',
    size_asc: '最小'
  };

  // 当前过滤器的可读说明：filterText 用于标题副行（必须短，否则 320px 会截断），
  // filterHint 用于空状态与本机面板（可以写全格式）
  var filterText = filter === 'building' ? '建筑文件 · 5 种格式'
    : filter === 'skin' ? '皮肤文件 · PNG'
      : filter === 'music' ? '音乐文件 · MIDI'
        : filter === 'zip' ? '插件包 · ZIP'
          : '全部文件';
  var filterHint = filter === 'building' ? '.mcstructure .schematic .schem .mcworld .bdx'
    : filter === 'skin' ? '64×64 / 128×128 皮肤 PNG'
      : filter === 'music' ? 'MIDI 音乐文件'
        : filter === 'zip' ? '插件包 ZIP'
          : '不限格式';

  overlay.innerHTML =
    '<div class="fp-sheet" role="dialog" aria-modal="true" aria-label="资源浏览器">' +
      '<div class="fp-ws-head">' +
        '<button class="fp-ws-icon" id="fp-nav-back" aria-label="返回上级目录">' + ICONS.arrowLeft + '</button>' +
        '<div class="fp-ws-title"><h3 id="fp-ws-name">' + (folderMode ? '选择文件夹' : '选择资源') + '</h3><p id="fp-ws-sub">' + pickerEscape(filterText) + '</p></div>' +
        '<button class="fp-ws-icon" id="fp-close" aria-label="关闭资源浏览器">' + ICONS.x + '</button>' +
      '</div>' +
      (folderMode ? '' :
        '<div class="fp-src" role="tablist" aria-label="数据来源">' +
          '<button class="fp-src-tab active" id="fp-src-device" role="tab" aria-selected="true">执行设备</button>' +
          '<button class="fp-src-tab" id="fp-src-local" role="tab" aria-selected="false">本机文件</button>' +
        '</div>') +
      '<div class="fp-crumbs" id="fp-crumbs"></div>' +
      '<div class="fp-toolbar">' +
        '<div class="fp-search">' +
          '<span class="fp-search-icon">' + ICONS.search + '</span>' +
          '<input class="fp-search-input" id="fp-search-input" type="text" placeholder="搜索当前目录..." autocomplete="off">' +
          '<button class="fp-search-clear" id="fp-search-clear" style="display:none" aria-label="清除搜索">' + ICONS.x + '</button>' +
        '</div>' +
        '<button class="fp-sort-btn" id="fp-sort-btn" aria-label="排序方式">' +
          '<span class="fp-sort-icon">' + ICONS.sort + '</span>' +
          '<span class="fp-sort-label" id="fp-sort-label">' + sortShort[sortMode] + '</span>' +
        '</button>' +
      '</div>' +
      '<div class="fp-local-panel gone" id="fp-local-panel">' +
        '<div class="fp-local-title">本机文件</div>' +
        '<p class="fp-local-desc">从当前手机或电脑选择文件，经上传接口写入<b>执行设备</b>后再使用。上传完成前不会产生设备路径。</p>' +
        '<button class="fp-local-btn" id="fp-upload-local">' + ICONS.upload + '<span>从本机选择并上传</span></button>' +
        '<div class="fp-local-facts">' +
          '<div><span>支持格式</span><b>' + pickerEscape(filterHint) + '</b></div>' +
          '<div><span>上传位置</span><b>执行设备接收目录</b></div>' +
        '</div>' +
        '<div class="fp-local-note" id="fp-local-note"></div>' +
      '</div>' +
      '<div class="fp-body" id="fp-body"><div class="fp-loading">加载中...</div></div>' +
      '<div class="fp-footer">' +
        '<div class="fp-sel">' +
          '<span class="fp-sel-name" id="fp-selected-label">尚未选择文件</span>' +
          '<span class="fp-sel-meta" id="fp-sel-meta"></span>' +
        '</div>' +
        '<div class="fp-foot-btns">' +
          '<button class="fp-btn fp-btn-cancel" id="fp-cancel">取消</button>' +
          '<button class="fp-btn fp-btn-ok" id="fp-ok"' + (folderMode ? '' : ' disabled') + '>' + (folderMode ? '选定此文件夹' : '选择') + '</button>' +
        '</div>' +
      '</div>' +
    '</div>';
  document.body.appendChild(overlay);

  var bodyEl = document.getElementById('fp-body');
  var crumbsEl = document.getElementById('fp-crumbs');
  var okBtn = document.getElementById('fp-ok');
  var searchInput = document.getElementById('fp-search-input');
  var searchClear = document.getElementById('fp-search-clear');
  var sortBtn = document.getElementById('fp-sort-btn');
  var sortLabel = document.getElementById('fp-sort-label');
  var selLabel = document.getElementById('fp-selected-label');
  var selMeta = document.getElementById('fp-sel-meta');
  var localPanel = document.getElementById('fp-local-panel');
  var localNote = document.getElementById('fp-local-note');
  var subEl = document.getElementById('fp-ws-sub');

  // ── 面包屑：路径分段，可点击回跳 ──
  function setCrumbs(path, labelOverride) {
    if (labelOverride) {
      crumbsEl.innerHTML = '<span class="fp-crumb cur">' + pickerEscape(labelOverride) + '</span>';
      return;
    }
    var p = String(path || '').replace(/\/+$/, '');
    if (!p) { crumbsEl.innerHTML = '<span class="fp-crumb cur">执行设备</span>'; return; }
    var segs = p.split('/').filter(Boolean);
    var html = '', acc = '';
    for (var i = 0; i < segs.length; i++) {
      acc += '/' + segs[i];
      var name = segs[i];
      if (i === 0 && name === 'storage') name = '设备存储';
      else if (acc === ROOT_PATH) name = '内部存储';
      var last = i === segs.length - 1;
      html += '<button class="fp-crumb' + (last ? ' cur' : '') + '" data-crumb="' + pickerEscape(acc) + '">' + pickerEscape(name) + '</button>';
      if (!last) html += '<span class="fp-crumb-sep" aria-hidden="true">/</span>';
    }
    crumbsEl.innerHTML = html;
    crumbsEl.querySelectorAll('[data-crumb]').forEach(function (b) {
      b.addEventListener('click', function () { navigateTo(b.dataset.crumb); });
    });
    crumbsEl.scrollLeft = crumbsEl.scrollWidth;
  }

  // ── 时间格式：今天只显示时分，其余显示月日 ──
  function fmtTime(ts) {
    if (!ts) return '';
    var d = new Date(ts * 1000);
    if (isNaN(d.getTime())) return '';
    var now = new Date();
    var pad = function (n) { return (n < 10 ? '0' : '') + n; };
    if (d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth() && d.getDate() === now.getDate())
      return pad(d.getHours()) + ':' + pad(d.getMinutes());
    return (d.getMonth() + 1) + '月' + d.getDate() + '日';
  }

  // ── 行副信息：类型 · 时间（大小在右侧单独一列，不重复写两遍） ──
  function rowMeta(item) {
    var parts = [];
    if (item.type === 'dir') {
      parts.push('文件夹');
    } else {
      var ext = String(item.ext || '').replace('.', '').toUpperCase();
      parts.push(ext ? ext + ' 文件' : '文件');
    }
    var t = fmtTime(item.mtime);
    if (t) parts.push(t);
    return parts.join(' · ');
  }

  // ── 选择状态同步：底部条 + 确定按钮 ──
  function setSelected(path, name, item) {
    selectedPath = path || '';
    selectedName = name || '';
    selectedItem = item || null;
    if (!selectedPath) {
      selLabel.textContent = folderMode ? '当前文件夹' : '尚未选择文件';
      selMeta.textContent = folderMode ? currentPath : '';
      okBtn.disabled = !folderMode;
      return;
    }
    selLabel.textContent = selectedName;
    var meta = [];
    if (item && item.type === 'file' && item.size != null && item.size > 0) meta.push(formatSize(item.size));
    meta.push(source === 'local' ? '本机上传 · 执行设备' : '执行设备');
    selMeta.textContent = meta.join(' · ');
    okBtn.disabled = false;
  }

  function navigateTo(path) {
    if (!pickerLive()) { closePicker(true); return; }
    // 只同步数据源 UI，不再触发一次导航（否则会重复扫描、拿到错的路径）
    if (source === 'local') applySourceUI('device');
    var epoch = ++scanEpoch;
    currentPath = path;
    setSelected('', '', null);
    searchTerm = '';
    searchInput.value = '';
    searchClear.style.display = 'none';
    virtualMode = false;
    virtualLabel = '';
    virtualPlayers = [];
    setCrumbs(currentPath);
    bodyEl.innerHTML = '<div class="fp-loading">加载中...</div>';
    fetch('/api/files/scan?path=' + encodeURIComponent(currentPath) + '&filter=' + encodeURIComponent(filter))
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (!pickerLive() || epoch !== scanEpoch) return;
        if (data.ok) {
          currentPath = data.path || path;
          setCrumbs(currentPath);
          dataList = data.entries || [];
          dataParent = data.parent || '';
          renderData(data);
          // 上传后自动选中刚落地的文件（等列表回来才选，否则列表里还没有它）
          if (pendingSelectPath) {
            var want = pendingSelectPath;
            pendingSelectPath = '';
            var hit = null;
            for (var qi = 0; qi < dataList.length; qi++) if (dataList[qi].path === want) { hit = dataList[qi]; break; }
            if (hit) {
              var rowEl = null;
              bodyEl.querySelectorAll('.fp-item').forEach(function (x) { if (x.dataset.path === want) rowEl = x; });
              if (rowEl) rowEl.classList.add('sel');
              setSelected(want, hit.name, hit);
            }
          }
          checkQQ();
        } else {
          bodyEl.innerHTML = '<div class="fp-empty"><b>无法读取目录</b><span>' + pickerEscape(data.error || '') + '</span></div>';
        }
      })
      .catch(function () {
        if (!pickerLive() || epoch !== scanEpoch) return;
        bodyEl.innerHTML = '<div class="fp-empty"><b>网络错误</b><span>请检查设备连接后重试</span></div>';
      });
  }

  // 判断条目里是否有符合当前过滤器、可被选中的内容
  function hasMatching(entries) {
    if (!entries) return false;
    for (var i = 0; i < entries.length; i++) {
      var it = entries[i];
      if (filter === 'building') {
        if (it.type !== 'dir' || it.has_building) return true;
      } else if (filter === 'skin') {
        if (it.type === 'dir' || (it.ext && it.ext.toLowerCase() === '.png')) return true;
      } else if (it.type === 'file') {
        return true;
      }
    }
    return false;
  }

  // 根目录下探测「qq文件」文件夹：能扫到且有符合选项才显示入口
  function checkQQ() {
    var epoch = scanEpoch;
    qqAvailable = false;
    var isRoot = currentPath.replace(/\/+$/, '') === ROOT_PATH;
    if (!isRoot || !filter || folderMode) return;
    fetch('/api/files/scan?path=' + encodeURIComponent(QQ_PATH) + '&filter=' + encodeURIComponent(filter))
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (!pickerLive() || epoch !== scanEpoch) return;
        if (data.ok && hasMatching(data.entries || [])) {
          qqAvailable = true;
          // 仍在根目录才刷新（可能已导航离开）
          if (currentPath.replace(/\/+$/, '') === ROOT_PATH) {
            renderData({ path: currentPath, entries: dataList });
          }
        }
      })
      .catch(function () { qqAvailable = false; });
  }

  function applySearch(entries) {
    if (!searchTerm) return entries;
    return entries.filter(function (item) {
      return item.name.toLowerCase().indexOf(searchTerm) !== -1;
    });
  }

  function sortEntries(entries) {
    var dirs = [];
    var files = [];
    for (var i = 0; i < entries.length; i++) {
      if (entries[i].type === 'dir') {
        dirs.push(entries[i]);
      } else {
        files.push(entries[i]);
      }
    }
    // 目录始终按名称升序
    dirs.sort(function (a, b) {
      return a.name.toLowerCase() < b.name.toLowerCase() ? -1 : 1;
    });
    // 文件按当前排序方式
    var cmp;
    switch (sortMode) {
      case 'name_asc':
        cmp = function (a, b) { return a.name.toLowerCase() < b.name.toLowerCase() ? -1 : 1; };
        break;
      case 'name_desc':
        cmp = function (a, b) { return a.name.toLowerCase() > b.name.toLowerCase() ? -1 : 1; };
        break;
      case 'date_asc':
        cmp = function (a, b) { return a.mtime - b.mtime; };
        break;
      case 'size_desc':
        cmp = function (a, b) { return (b.size || 0) - (a.size || 0); };
        break;
      case 'size_asc':
        cmp = function (a, b) { return (a.size || 0) - (b.size || 0); };
        break;
      default: // date_desc
        cmp = function (a, b) { return b.mtime - a.mtime; };
    }
    files.sort(cmp);
    return dirs.concat(files);
  }

  // 行模板：图标 + 名称/副信息 + 「⋯」详情 + 大小
  function rowHTML(opts) {
    var icon = opts.iconHtml;
    var name = opts.name;
    var meta = opts.meta || '';
    var size = opts.size || '';
    return '<div class="fp-item' + (opts.cls ? ' ' + opts.cls : '') + '"' +
      (opts.attrs || '') +
      '><div class="fp-icon ' + (opts.iconCls || '') + '">' + icon + '</div>' +
      '<div class="fp-info"><div class="fp-name">' + name + '</div>' +
      (meta ? '<div class="fp-meta">' + meta + '</div>' : '') + '</div>' +
      (size ? '<span class="fp-size">' + size + '</span>' : '') +
      (opts.more ? '<button class="fp-more" data-more="' + pickerEscape(opts.more.id) + '" aria-label="查看完整信息">⋯</button>' : '') +
      '</div>';
  }

  function renderData(data) {
    if (!pickerLive()) return;
    // 虚拟文件夹模式：直接渲染列表
    if (virtualMode) {
      if (filter === 'music') { renderMusicHot(); } else { renderVirtualPlayers(); }
      return;
    }

    // 1. 过滤：建筑过滤 + 皮肤 PNG 过滤
    var filtered = dataList;
    if (filter === 'building') {
      filtered = dataList.filter(function (item) { return item.type !== 'dir' || item.has_building; });
    } else if (filter === 'skin') {
      filtered = dataList.filter(function (item) { return item.type === 'dir' || (item.ext && item.ext.toLowerCase() === '.png'); });
    }
    // 2. 搜索过滤
    filtered = applySearch(filtered);
    // 3. 排序
    filtered = sortEntries(filtered);

    var hasContent = filtered.length > 0;
    var isRoot = currentPath.replace(/\/+$/, '') === ROOT_PATH;

    // 上级目录：皮肤模式始终显示（空文件夹也保留，便于返回）；建筑/通用仅在「有符合内容」且非根目录、非搜索时显示
    var parentPath = data.parent || dataParent;
    var html = '';
    if (parentPath && !isRoot && !searchTerm && (filter === 'skin' || hasContent)) {
      html += rowHTML({ iconHtml: ICONS.parentDir, iconCls: 'fp-icon-dir', name: '.. 上级目录', meta: '返回上一层', attrs: ' data-path="' + pickerEscape(parentPath) + '" data-type="parent"' });
    }
    // 皮肤选择器：顶部显示「在线玩家皮肤」虚拟文件夹
    if (filter === 'skin' && !searchTerm) {
      html += rowHTML({ iconHtml: ICONS.globe, iconCls: 'fp-icon-dir', name: '在线玩家皮肤', meta: '来自服务器皮肤缓存', attrs: ' data-virtual="1"' });
    }
    // 音乐模式：顶部显示「热门歌曲」虚拟文件夹
    if (filter === 'music' && !searchTerm && !folderMode) {
      html += rowHTML({ iconHtml: ICONS.musicNote, iconCls: 'fp-icon-dir', name: '热门歌曲', meta: '服务器 · 按下载量排序', attrs: ' data-music-hot="1"' });
    }
    // 根目录：探测到有符合文件的「qq文件」虚拟入口
    if (qqAvailable && isRoot && !searchTerm && !folderMode) {
      html += rowHTML({ iconHtml: ICONS.folder, iconCls: 'fp-icon-dir fp-icon-qq', name: 'qq文件', meta: 'QQ 接收文件', cls: 'fp-qq', attrs: ' data-qq="1"' });
    }

    if (!filtered.length && !html) {
      var emptyTxt = searchTerm ? '没有匹配「' + pickerEscape(searchTerm) + '」的文件'
        : filter === 'building' ? '这个目录里没有建筑文件'
          : filter === 'skin' ? '这个目录里没有皮肤 PNG'
            : filter === 'music' ? '这个目录里没有音乐文件'
              : '空目录';
      bodyEl.innerHTML = '<div class="fp-empty"><b>' + emptyTxt + '</b><span>' + pickerEscape(filterHint) + '</span></div>';
      return;
    }

    for (var i = 0; i < filtered.length; i++) {
      var item = filtered[i];
      var isDir = item.type === 'dir';
      var iconSvg = isDir ? ICONS.folder : ICONS.file;
      var iconCls = isDir ? 'fp-icon-dir' : 'fp-icon-file';
      if (!isDir && item.ext) iconCls += ' fp-icon-' + item.ext.replace('.', '');
      // 皮肤选择器：PNG 文件显示头部缩略图
      var iconHtml;
      if (filter === 'skin' && !isDir && item.ext && item.ext.toLowerCase() === '.png') {
        iconHtml = '<img src="/api/skin/head?path=' + encodeURIComponent(item.path) + '" alt="" onerror="this.style.display=\'none\'">';
        iconCls = 'fp-head';
      } else {
        iconHtml = iconSvg;
      }
      var extLabel = isDir ? '' : String(item.ext || '').replace('.', '').toUpperCase();
      html += rowHTML({
        iconHtml: iconHtml,
        iconCls: iconCls,
        name: pickerEscape(item.name),
        meta: rowMeta(item),
        size: isDir ? '' : formatSize(item.size),
        attrs: ' data-path="' + pickerEscape(item.path) + '" data-type="' + pickerEscape(item.type) + '" data-name="' + pickerEscape(item.name) + '" data-ext="' + pickerEscape(extLabel) + '" data-size="' + (item.size || 0) + '" data-mtime="' + (item.mtime || 0) + '"',
        more: { id: item.path }
      });
    }
    bodyEl.innerHTML = html;
    bindRowEvents();
  }

  // 行点击：进入目录 / 选中文件 / 虚拟入口；「⋯」打开完整信息
  function bindRowEvents() {
    bodyEl.querySelectorAll('.fp-item').forEach(function (el) {
      el.addEventListener('click', function (ev) {
        if (ev.target.closest('.fp-more')) return;
        if (!pickerLive()) { closePicker(true); return; }
        var path = el.dataset.path;
        var type = el.dataset.type;
        if (el.dataset.virtual) {
          enterVirtualFolder();
        } else if (el.dataset.musicHot) {
          enterMusicHotFolder();
        } else if (el.dataset.qq) {
          navigateTo(QQ_PATH);
        } else if (type === 'dir' || type === 'parent') {
          navigateTo(path);
        } else if (!folderMode) {
          bodyEl.querySelectorAll('.fp-item').forEach(function (x) { x.classList.remove('sel'); });
          el.classList.add('sel');
          setSelected(path, el.dataset.name || '', {
            type: 'file',
            size: Number(el.dataset.size) || 0,
            ext: el.dataset.ext ? '.' + String(el.dataset.ext).toLowerCase() : '',
            mtime: Number(el.dataset.mtime) || 0
          });
        }
      });
    });
    bodyEl.querySelectorAll('.fp-more').forEach(function (btn) {
      btn.addEventListener('click', function (ev) {
        ev.stopPropagation();
        showRowInfo(btn.dataset.more);
      });
    });
  }

  // 完整信息：长文件名截断后的出口，可看全名并复制
  function showRowInfo(path) {
    var item = null;
    for (var i = 0; i < dataList.length; i++) if (dataList[i].path === path) { item = dataList[i]; break; }
    if (!item) return;
    var sheet = document.createElement('div');
    sheet.className = 'bs-overlay';
    var rows = [
      ['名称', item.name],
      ['类型', item.type === 'dir' ? '文件夹' : (String(item.ext || '').replace('.', '').toUpperCase() || '文件')],
      ['大小', item.type === 'dir' ? '—' : formatSize(item.size)],
      ['修改时间', item.mtime ? new Date(item.mtime * 1000).toLocaleString() : '—'],
      ['路径', item.path]
    ];
    var body = '';
    for (var k = 0; k < rows.length; k++) {
      body += '<div class="fp-info-row"><span>' + rows[k][0] + '</span><b>' + pickerEscape(rows[k][1]) + '</b></div>';
    }
    sheet.innerHTML = '<div class="bs-sheet">' +
      '<div class="bs-title">文件信息</div>' +
      '<div class="bs-body">' + body + '</div>' +
      '<div class="bs-footer">' +
        '<button class="bs-btn" id="fp-info-copy">复制名称</button>' +
        '<button class="bs-btn bs-btn-cancel" id="fp-info-close">关闭</button>' +
      '</div></div>';
    document.body.appendChild(sheet);
    document.getElementById('fp-info-copy').addEventListener('click', function () {
      var txt = item.name;
      var done = function () { var b = document.getElementById('fp-info-copy'); if (b) { b.textContent = '已复制'; } };
      if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(txt).then(done, done);
      else {
        var ta = document.createElement('textarea');
        ta.value = txt; document.body.appendChild(ta); ta.select();
        try { document.execCommand('copy'); } catch (e) { }
        ta.remove(); done();
      }
    });
    document.getElementById('fp-info-close').addEventListener('click', function () { closeOverlay(sheet); });
    sheet.addEventListener('click', function (e) { if (e.target === sheet) closeOverlay(sheet); });
  }

  // 进入「在线玩家皮肤」虚拟文件夹
  function enterVirtualFolder() {
    if (!pickerLive()) { closePicker(true); return; }
    var epoch = ++scanEpoch;
    virtualMode = true;
    virtualLabel = '在线玩家皮肤';
    setSelected('', '', null);
    searchTerm = '';
    searchInput.value = '';
    searchClear.style.display = 'none';
    setCrumbs('', virtualLabel);
    bodyEl.innerHTML = '<div class="fp-loading">加载中...</div>';
    fetch('/api/skin/online-list')
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (!pickerLive() || epoch !== scanEpoch) return;
        virtualPlayers = data.ok ? (data.players || []) : [];
        renderVirtualPlayers();  // 无玩家时也由它渲染「返回上级」+ 空提示
      })
      .catch(function () {
        if (!pickerLive() || epoch !== scanEpoch) return;
        bodyEl.innerHTML = '<div class="fp-empty"><b>获取在线玩家失败</b><span>服务器皮肤缓存暂不可用</span></div>';
      });
  }

  // 渲染在线玩家皮肤列表（玩家名.png）
  function renderVirtualPlayers() {
    if (!pickerLive()) return;
    var list = searchTerm
      ? virtualPlayers.filter(function (p) { return p.name.toLowerCase().indexOf(searchTerm) !== -1; })
      : virtualPlayers;
    // 皮肤模式：在线列表始终显示「返回上级」，空列表也不隐藏（避免卡住退不出去）
    var html = searchTerm ? '' :
      rowHTML({ iconHtml: ICONS.parentDir, iconCls: 'fp-icon-dir', name: '.. 返回文件列表', meta: '退出在线玩家皮肤', attrs: ' data-virtual-back="1"' });
    if (!list.length) {
      bodyEl.innerHTML = html + '<div class="fp-empty"><b>' + (searchTerm ? '没有匹配的玩家' : '暂无在线玩家皮肤') + '</b><span>玩家上线后会自动出现在这里</span></div>';
      bindVirtualEvents();
      return;
    }
    for (var i = 0; i < list.length; i++) {
      var pl = list[i];
      html += rowHTML({
        iconHtml: '<img src="/api/skin/online-head?player=' + encodeURIComponent(pl.name) + '" alt="" onerror="this.style.display=\'none\'">',
        iconCls: 'fp-head',
        name: pickerEscape(pl.name) + '.png',
        meta: '玩家皮肤 · 来自服务器缓存',
        size: (pl.width || '') + '×' + (pl.height || ''),
        attrs: ' data-virtual-player="' + pickerEscape(pl.name) + '"'
      });
    }
    bodyEl.innerHTML = html;
    bindVirtualEvents();
  }

  function bindVirtualEvents() {
    bodyEl.querySelectorAll('.fp-item').forEach(function (el) {
      el.addEventListener('click', function () {
        if (el.dataset.virtualBack) {
          // 返回上级目录（回到文件列表）
          navigateTo(currentPath);
          return;
        }
        var pname = el.dataset.virtualPlayer;
        if (!pname) return;
        bodyEl.querySelectorAll('.fp-item').forEach(function (x) { x.classList.remove('sel'); });
        el.classList.add('sel');
        setSelected(onlinePrefix + pname, pname + '.png', null);
      });
    });
  }

  // 进入「热门歌曲」虚拟文件夹（服务器按热度排序）
  function enterMusicHotFolder() {
    if (!pickerLive()) { closePicker(true); return; }
    var epoch = ++scanEpoch;
    virtualMode = true;
    virtualLabel = '热门歌曲';
    setSelected('', '', null);
    searchTerm = '';
    searchInput.value = '';
    searchClear.style.display = 'none';
    setCrumbs('', virtualLabel);
    bodyEl.innerHTML = '<div class="fp-loading">加载中...</div>';
    fetch('/api/music-proxy/api/music/hot')
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (!pickerLive() || epoch !== scanEpoch) return;
        musicHot = data.ok ? (data.list || []) : [];
        renderMusicHot();
      })
      .catch(function () {
        if (!pickerLive() || epoch !== scanEpoch) return;
        bodyEl.innerHTML = '<div class="fp-empty"><b>获取热门歌曲失败</b><span>音乐服务暂不可用</span></div>';
      });
  }

  // 渲染热门歌曲列表（只显示歌名 + 下载量）
  function renderMusicHot() {
    if (!pickerLive()) return;
    var list = searchTerm
      ? musicHot.filter(function (s) { return (s.name || '').toLowerCase().indexOf(searchTerm) !== -1; })
      : musicHot;
    // 音乐模式：始终显示「返回上级」
    var html = searchTerm ? '' :
      rowHTML({ iconHtml: ICONS.parentDir, iconCls: 'fp-icon-dir', name: '.. 返回文件列表', meta: '退出热门歌曲', attrs: ' data-music-back="1"' });
    if (!list.length) {
      bodyEl.innerHTML = html + '<div class="fp-empty"><b>' + (searchTerm ? '没有匹配的歌曲' : '暂无热门歌曲') + '</b><span>服务器下载榜为空</span></div>';
      bindMusicVirtualEvents();
      return;
    }
    for (var i = 0; i < list.length; i++) {
      var s = list[i];
      html += rowHTML({
        iconHtml: ICONS.musicNote,
        iconCls: 'fp-icon-file fp-icon-midi',
        name: escHtml(s.name),
        meta: '服务器曲库 · 下载量 ' + (s.heat || 0),
        attrs: ' data-music-song="' + pickerEscape(s.name) + '"'
      });
    }
    bodyEl.innerHTML = html;
    bindMusicVirtualEvents();
  }

  function bindMusicVirtualEvents() {
    bodyEl.querySelectorAll('.fp-item').forEach(function (el) {
      el.addEventListener('click', function () {
        if (el.dataset.musicBack) {
          navigateTo(currentPath);
          return;
        }
        var sname = el.dataset.musicSong;
        if (!sname) return;
        bodyEl.querySelectorAll('.fp-item').forEach(function (x) { x.classList.remove('sel'); });
        el.classList.add('sel');
        setSelected(musicPrefix + sname, sname, null);
      });
    });
  }

  // ── 数据来源切换 ──
  // applySourceUI 只改界面状态；setSource 才负责导航，避免 navigateTo → setSource → navigateTo 的递归
  function applySourceUI(next) {
    source = next;
    var devTab = document.getElementById('fp-src-device');
    var locTab = document.getElementById('fp-src-local');
    if (devTab && locTab) {
      devTab.classList.toggle('active', next === 'device');
      locTab.classList.toggle('active', next === 'local');
      devTab.setAttribute('aria-selected', next === 'device' ? 'true' : 'false');
      locTab.setAttribute('aria-selected', next === 'local' ? 'true' : 'false');
    }
    var toolbar = overlay.querySelector('.fp-toolbar');
    if (next === 'local') {
      localPanel.classList.remove('gone');
      crumbsEl.classList.add('gone');
      if (toolbar) toolbar.classList.add('gone');
      bodyEl.innerHTML = '';
      bodyEl.classList.add('fp-body-flat');
      if (subEl) subEl.textContent = '本机文件 · 上传后写入执行设备';
    } else {
      localPanel.classList.add('gone');
      crumbsEl.classList.remove('gone');
      if (toolbar) toolbar.classList.remove('gone');
      bodyEl.classList.remove('fp-body-flat');
      if (subEl) subEl.textContent = filterText;
    }
  }
  function setSource(next) {
    if (source === next) return;
    applySourceUI(next);
    if (next === 'device') navigateTo(currentPath);
  }
  var devTabEl = document.getElementById('fp-src-device');
  var locTabEl = document.getElementById('fp-src-local');
  if (devTabEl) devTabEl.addEventListener('click', function () { setSource('device'); });
  if (locTabEl) locTabEl.addEventListener('click', function () { setSource('local'); });

  // ── 搜索 300ms 防抖 ──
  var debounceTimer = null;
  searchInput.addEventListener('input', function () {
    searchTerm = this.value.trim().toLowerCase();
    searchClear.style.display = searchTerm ? '' : 'none';
    clearTimeout(debounceTimer);
    if (virtualMode) {
      debounceTimer = setTimeout(function () { if (filter === 'music') renderMusicHot(); else renderVirtualPlayers(); }, 300);
    } else {
      debounceTimer = setTimeout(function () {
        renderData({ path: currentPath, entries: dataList });
      }, 300);
    }
  });

  // ── 搜索清除按钮 ──
  searchClear.addEventListener('click', function () {
    searchInput.value = '';
    searchTerm = '';
    searchClear.style.display = 'none';
    if (virtualMode) {
      if (filter === 'music') renderMusicHot(); else renderVirtualPlayers();
    } else {
      renderData({ path: currentPath, entries: dataList });
    }
    searchInput.focus();
  });

  // ── 排序弹窗 ──
  sortBtn.addEventListener('click', function () {
    var so = document.createElement('div');
    so.className = 'bs-overlay';
    var listHtml = '';
    var keys = ['name_asc', 'name_desc', 'date_desc', 'date_asc', 'size_desc', 'size_asc'];
    for (var i = 0; i < keys.length; i++) {
      var k = keys[i];
      var active = k === sortMode ? ' active' : '';
      var check = k === sortMode ? ICONS.check : '';
      listHtml += '<div class="bs-option' + active + '" data-sort="' + k + '">' +
        '<span class="bs-dot"></span>' +
        '<span class="bs-label">' + sortLabels[k] + '</span>' +
        (check ? '<span style="color:var(--phx-primary);flex-shrink:0">' + check + '</span>' : '') +
        '</div>';
    }
    so.innerHTML = '<div class="bs-sheet">' +
      '<div class="bs-title">排序方式</div>' +
      '<div class="bs-body">' + listHtml + '</div>' +
      '<div class="bs-footer"><button class="bs-btn bs-btn-cancel" id="fp-sort-cancel">取消</button></div>' +
      '</div>';
    document.body.appendChild(so);

    so.querySelectorAll('.bs-option').forEach(function (el) {
      el.addEventListener('click', function () {
        var m = el.dataset.sort;
        sortMode = m;
        sortLabel.textContent = sortShort[m];
        if (virtualMode) { if (filter === 'music') renderMusicHot(); else renderVirtualPlayers(); }
        else renderData({ path: currentPath, entries: dataList });
        closeOverlay(so);
      });
    });
    document.getElementById('fp-sort-cancel').addEventListener('click', function () { closeOverlay(so); });
    so.addEventListener('click', function (e) { if (e.target === so) closeOverlay(so); });
  });

  // ── 导航 ──
  document.getElementById('fp-nav-back').addEventListener('click', function () {
    if (source === 'local') { setSource('device'); return; }
    if (virtualMode) { navigateTo(currentPath); return; }
    if (dataParent) navigateTo(dataParent);
  });

  document.getElementById('fp-cancel').addEventListener('click', function () { closePicker(true); });
  document.getElementById('fp-close').addEventListener('click', function () { closePicker(true); });

  // 本机文件：上传到执行设备（保留原有上传链路，不伪造设备路径）
  document.getElementById('fp-upload-local').addEventListener('click', async function () {
    if (!pickerLive()) { closePicker(true); return; }
    if (folderMode) return;
    var epoch = scanEpoch;
    var upload = typeof yyStudioUploadFile === 'function' ? yyStudioUploadFile : null;
    if (!upload) {
      localNote.textContent = '当前环境不支持上传，请改从执行设备已有文件中选择。';
      return;
    }
    localNote.textContent = '正在上传...';
    var path = await upload(filter);
    if (!pickerLive() || epoch !== scanEpoch) return;
    if (!path) { localNote.textContent = '已取消上传。'; return; }
    var name = String(path).split(/[\\/]/).pop();
    localNote.textContent = '已上传到执行设备：' + path;
    // 上传完成后切回设备视图，并跳到文件真实落地的目录（只导航一次，落地后再自动选中）
    pendingSelectPath = path;
    applySourceUI('device');
    var dir = String(path).replace(/[\\/][^\\/]+$/, '') || currentPath;
    navigateTo(dir);
  });

  okBtn.addEventListener('click', function () {
    if (!pickerLive()) { closePicker(true); return; }
    if (folderMode) {
      if (onSelect) onSelect(currentPath);
    } else {
      if (!selectedPath) return;
      if (onSelect) onSelect(selectedPath);
    }
    closePicker(false);
  });

  // Esc 关闭
  overlay.addEventListener('keydown', function (e) { if (e.key === 'Escape') closePicker(true); });

  setCrumbs(currentPath);
  navigateTo(currentPath);
}
