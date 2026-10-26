/* ═══ 文件选择器 — 支持搜索 + 排序 ═══ */

function showFilePicker(startPath, filter, onSelect, mode, onCancel) {
  var folderMode = mode === 'folder';
  var overlay = document.createElement('div');
  overlay.className = 'fp-overlay';
  var currentPath = startPath === '' ? '' : (startPath || '/storage/emulated/0');
  var scanEpoch=0, closed=false;
  var pickerOwner=typeof yyReadScope==='function'?yyReadScope():'';
  function pickerLive(){return !closed&&(typeof yyReadScope!=='function'||pickerOwner===yyReadScope())}
  function closePicker(cancelled){closed=true;scanEpoch++;overlay.remove();if(cancelled&&onCancel)onCancel()}
  var pickerEscape=function(s){return String(s||'').replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})};
  var selectedPath = '';
  var selectedName = '';
  var dataList = [];
  var dataParent = '';
  var searchTerm = '';
  var sortMode = 'date_desc';
  var virtualMode = false;   // 是否在「在线玩家皮肤」虚拟文件夹内
  var virtualPlayers = [];   // 在线玩家列表 [{name, width, height}]
  var onlinePrefix = 'online://';
  var musicHot = [];         // 服务器热门歌曲列表 [{name, heat}]
  var musicPrefix = 'music://';
  var ROOT_PATH = '/storage/emulated/0';                                                       // 文件选择器根目录
  var QQ_PATH = '/storage/emulated/0/Android/data/com.tencent.mobileqq/Tencent/QQfile_recv/';  // QQ 接收文件夹
  var qqAvailable = false;   // 根目录下是否显示「qq文件」虚拟入口

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

  overlay.innerHTML =
    '<div class="fp-modal" role="dialog" aria-modal="true" aria-label="选择文件">' +
      '<div class="fp-studio-title"><div><h3>选择资源</h3><p>当前执行设备 · 内置文件选择器</p></div><button id="fp-close" aria-label="关闭文件选择器">×</button></div>' +
      '<div class="fp-header"><button class="fp-back" id="fp-nav-back">&#8592;</button><span class="fp-path" id="fp-path">' + currentPath + '</span></div>' +
      '<div class="fp-toolbar">' +
        '<div class="fp-search">' +
          '<span class="fp-search-icon">' + ICONS.search + '</span>' +
          '<input class="fp-search-input" id="fp-search-input" type="text" placeholder="搜索文件..." autocomplete="off">' +
          '<button class="fp-search-clear" id="fp-search-clear" style="display:none">' + ICONS.x + '</button>' +
        '</div>' +
        '<button class="fp-sort-btn" id="fp-sort-btn">' +
          '<span class="fp-sort-icon">' + ICONS.sort + '</span>' +
          '<span class="fp-sort-label" id="fp-sort-label">' + sortShort[sortMode] + '</span>' +
        '</button>' +
      '</div>' +
      '<div class="fp-upload-local">文件还在手机或电脑上？ <button id="fp-upload-local">上传到当前设备</button></div>' +
      '<div class="fp-body" id="fp-body"><div class="fp-loading">加载中...</div></div>' +
      '<div class="fp-footer"><span class="fp-selected-label" id="fp-selected-label">尚未选择文件</span>' +
        '<button class="fp-btn fp-btn-cancel" id="fp-cancel">取消</button>' +
        '<button class="fp-btn fp-btn-ok" id="fp-ok"' + (folderMode ? '' : ' disabled') + '>' + (folderMode ? '确定' : '选择') + '</button>' +
      '</div>' +
    '</div>';
  document.body.appendChild(overlay);

  var pathEl = document.getElementById('fp-path');
  var bodyEl = document.getElementById('fp-body');
  var okBtn = document.getElementById('fp-ok');
  var searchInput = document.getElementById('fp-search-input');
  var searchClear = document.getElementById('fp-search-clear');
  var sortBtn = document.getElementById('fp-sort-btn');
  var sortLabel = document.getElementById('fp-sort-label');

  function navigateTo(path) {
    if(!pickerLive()){closePicker(true);return}
    var epoch=++scanEpoch;
    currentPath = path;
    document.getElementById('fp-selected-label').textContent=folderMode?'当前文件夹':'尚未选择文件';
    selectedPath = '';
    selectedName = '';
    searchTerm = '';
    searchInput.value = '';
    searchClear.style.display = 'none';
    virtualMode = false;
    virtualPlayers = [];
    if (!folderMode) okBtn.disabled = true;
    pathEl.textContent = currentPath;
    bodyEl.innerHTML = '<div class="fp-loading">加载中...</div>';
    fetch('/api/files/scan?path=' + encodeURIComponent(currentPath) + '&filter=' + encodeURIComponent(filter))
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if(!pickerLive()||epoch!==scanEpoch)return;
        if (data.ok) {
          currentPath=data.path||path;pathEl.textContent=currentPath;
          dataList = data.entries || [];
          dataParent = data.parent || '';
          renderData(data);
          checkQQ();
        } else {
          bodyEl.innerHTML = '<div class="fp-empty">无法读取目录: ' + pickerEscape(data.error || '') + '</div>';
        }
      })
      .catch(function(err) {
        if(!pickerLive()||epoch!==scanEpoch)return;
        bodyEl.innerHTML = '<div class="fp-empty">网络错误</div>';
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
    var epoch=scanEpoch;
    qqAvailable = false;
    var isRoot = currentPath.replace(/\/+$/, '') === ROOT_PATH;
    if (!isRoot || !filter || folderMode) return;
    fetch('/api/files/scan?path=' + encodeURIComponent(QQ_PATH) + '&filter=' + encodeURIComponent(filter))
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if(!pickerLive()||epoch!==scanEpoch)return;
        if (data.ok && hasMatching(data.entries || [])) {
          qqAvailable = true;
          // 仍在根目录才刷新（可能已导航离开）
          if (currentPath.replace(/\/+$/, '') === ROOT_PATH) {
            renderData({path: currentPath, entries: dataList});
          }
        }
      })
      .catch(function() { qqAvailable = false; });
  }

  function applySearch(entries) {
    if (!searchTerm) return entries;
    return entries.filter(function(item) {
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
    dirs.sort(function(a, b) {
      return a.name.toLowerCase() < b.name.toLowerCase() ? -1 : 1;
    });
    // 文件按当前排序方式
    var cmp;
    switch (sortMode) {
      case 'name_asc':
        cmp = function(a, b) { return a.name.toLowerCase() < b.name.toLowerCase() ? -1 : 1; };
        break;
      case 'name_desc':
        cmp = function(a, b) { return a.name.toLowerCase() > b.name.toLowerCase() ? -1 : 1; };
        break;
      case 'date_asc':
        cmp = function(a, b) { return a.mtime - b.mtime; };
        break;
      case 'size_desc':
        cmp = function(a, b) { return (b.size || 0) - (a.size || 0); };
        break;
      case 'size_asc':
        cmp = function(a, b) { return (a.size || 0) - (b.size || 0); };
        break;
      default: // date_desc
        cmp = function(a, b) { return b.mtime - a.mtime; };
    }
    files.sort(cmp);
    return dirs.concat(files);
  }

  function renderData(data) {
    if(!pickerLive())return;
    // 虚拟文件夹模式：直接渲染列表
    if (virtualMode) {
      if (filter === 'music') { renderMusicHot(); } else { renderVirtualPlayers(); }
      return;
    }

    // 1. 过滤：建筑过滤 + 皮肤 PNG 过滤
    var filtered = dataList;
    if (filter === 'building') {
      filtered = dataList.filter(function(item) { return item.type !== 'dir' || item.has_building; });
    } else if (filter === 'skin') {
      filtered = dataList.filter(function(item) { return item.type === 'dir' || (item.ext && item.ext.toLowerCase() === '.png'); });
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
      html = '<div class="fp-item" data-path="' + parentPath + '" data-type="parent"><div class="fp-icon fp-icon-dir">' + ICONS.parentDir + '</div><div class="fp-info"><div class="fp-name">.. (上级目录)</div></div></div>';
    }
    // 皮肤选择器：顶部显示「在线玩家皮肤」虚拟文件夹
    if (filter === 'skin' && !searchTerm) {
      html += '<div class="fp-item fp-virtual" data-virtual="1"><div class="fp-icon fp-icon-dir">' + ICONS.globe + '</div>' +
        '<div class="fp-info"><div class="fp-name">在线玩家皮肤</div></div></div>';
    }
    // 音乐模式：顶部显示「热门歌曲」虚拟文件夹
    if (filter === 'music' && !searchTerm && !folderMode) {
      html += '<div class="fp-item fp-virtual" data-music-hot="1"><div class="fp-icon fp-icon-dir">' + ICONS.musicNote + '</div>' +
        '<div class="fp-info"><div class="fp-name">热门歌曲</div><div class="fp-meta">服务器 · 按下载量排序</div></div></div>';
    }
    // 根目录：探测到有符合文件的「qq文件」虚拟入口
    if (qqAvailable && isRoot && !searchTerm && !folderMode) {
      html += '<div class="fp-item fp-qq" data-qq="1"><div class="fp-icon fp-icon-dir fp-icon-qq">' + ICONS.folder + '</div>' +
        '<div class="fp-info"><div class="fp-name">qq文件</div><div class="fp-meta">QQ 接收文件</div></div></div>';
    }

    if (!filtered.length && !html) {
      var emptyTxt = searchTerm ? '无匹配文件'
        : filter === 'building' ? '暂无建筑文件'
        : filter === 'skin' ? '暂无皮肤文件'
        : filter === 'music' ? '暂无音乐文件'
        : '空目录';
      bodyEl.innerHTML = '<div class="fp-empty">' + emptyTxt + '</div>';
      return;
    }

    for (var i = 0; i < filtered.length; i++) {
      var item = filtered[i];
      var iconClass = item.type === 'dir' ? 'fp-icon-dir' : 'fp-icon-file';
      var iconSvg = item.type === 'dir' ? ICONS.folder : ICONS.file;
      var extClass = item.ext ? 'fp-icon-' + item.ext.replace('.', '') : '';
      var sizeStr = item.type === 'file' ? formatSize(item.size) : '';
      // 皮肤选择器：PNG 文件显示头部缩略图
      var iconHtml;
      if (filter === 'skin' && item.type === 'file' && item.ext && item.ext.toLowerCase() === '.png') {
        iconHtml = '<div class="fp-icon fp-head"><img src="/api/skin/head?path=' + encodeURIComponent(item.path) + '" alt="" onerror="this.style.display=\'none\'"></div>';
      } else {
        iconHtml = '<div class="fp-icon ' + iconClass + ' ' + extClass + '">' + iconSvg + '</div>';
      }
      html += '<div class="fp-item" data-path="' + pickerEscape(item.path) + '" data-type="' + pickerEscape(item.type) + '">' +
        iconHtml +
        '<div class="fp-info"><div class="fp-name">' + pickerEscape(item.name) + '</div></div>' +
        (sizeStr ? '<span class="fp-size">' + sizeStr + '</span>' : '') +
        '</div>';
    }
    bodyEl.innerHTML = html;

    bodyEl.querySelectorAll('.fp-item').forEach(function(el) {
      el.addEventListener('click', function() {
        if(!pickerLive()){closePicker(true);return}
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
          bodyEl.querySelectorAll('.fp-item').forEach(function(x) { x.style.background = ''; });
          el.style.background = 'var(--phx-primary-bg)';
          selectedPath = path;
          selectedName = el.querySelector('.fp-name').textContent;
          document.getElementById('fp-selected-label').textContent=selectedName;
          okBtn.disabled = false;
        }
      });
    });
  }

  // 进入「在线玩家皮肤」虚拟文件夹
  function enterVirtualFolder() {
    if(!pickerLive()){closePicker(true);return}var epoch=++scanEpoch;
    virtualMode = true;
    selectedPath = '';
    selectedName = '';
    searchTerm = '';
    searchInput.value = '';
    searchClear.style.display = 'none';
    if (!folderMode) okBtn.disabled = true;
    pathEl.textContent = '在线玩家皮肤';
    bodyEl.innerHTML = '<div class="fp-loading">加载中...</div>';
    fetch('/api/skin/online-list')
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if(!pickerLive()||epoch!==scanEpoch)return;
        virtualPlayers = data.ok ? (data.players || []) : [];
        renderVirtualPlayers();  // 无玩家时也由它渲染「返回上级」+ 空提示
      })
      .catch(function() {
        if(!pickerLive()||epoch!==scanEpoch)return;
        bodyEl.innerHTML = '<div class="fp-empty">获取在线玩家失败</div>';
      });
  }

  // 渲染在线玩家皮肤列表（玩家名.png）
  function renderVirtualPlayers() {
    if(!pickerLive())return;
    var list = searchTerm
      ? virtualPlayers.filter(function(p) { return p.name.toLowerCase().indexOf(searchTerm) !== -1; })
      : virtualPlayers;
    // 皮肤模式：在线列表始终显示「返回上级」，空列表也不隐藏（避免卡住退不出去）
    var html = searchTerm ? '' :
      '<div class="fp-item fp-parent" data-virtual-back="1"><div class="fp-icon fp-icon-dir">' + ICONS.parentDir + '</div><div class="fp-info"><div class="fp-name">.. (返回上级目录)</div></div></div>';
    if (!list.length) {
      bodyEl.innerHTML = html + '<div class="fp-empty">' + (searchTerm ? '无匹配玩家' : '暂无在线玩家皮肤') + '</div>';
      bindVirtualEvents();
      return;
    }
    for (var i = 0; i < list.length; i++) {
      var pl = list[i];
      html += '<div class="fp-item" data-virtual-player="' + pickerEscape(pl.name) + '">' +
        '<div class="fp-icon fp-head"><img src="/api/skin/online-head?player=' + encodeURIComponent(pl.name) + '" alt="" onerror="this.style.display=\'none\'"></div>' +
        '<div class="fp-info"><div class="fp-name">' + pickerEscape(pl.name) + '.png</div></div>' +
        '<span class="fp-size">' + (pl.width || '') + 'x' + (pl.height || '') + '</span>' +
        '</div>';
    }
    bodyEl.innerHTML = html;
    bindVirtualEvents();
  }

  function bindVirtualEvents() {
    bodyEl.querySelectorAll('.fp-item').forEach(function(el) {
      el.addEventListener('click', function() {
        if (el.dataset.virtualBack) {
          // 返回上级目录（回到文件列表）
          navigateTo(currentPath);
          return;
        }
        var pname = el.dataset.virtualPlayer;
        if (!pname) return;
        bodyEl.querySelectorAll('.fp-item').forEach(function(x) { x.style.background = ''; });
        el.style.background = 'var(--phx-primary-bg)';
        selectedPath = onlinePrefix + pname;
        selectedName = pname + '.png';
        okBtn.disabled = false;
      });
    });
  }

  // 进入「热门歌曲」虚拟文件夹（服务器按热度排序）
  function enterMusicHotFolder() {
    if(!pickerLive()){closePicker(true);return}var epoch=++scanEpoch;
    virtualMode = true;
    selectedPath = '';
    selectedName = '';
    searchTerm = '';
    searchInput.value = '';
    searchClear.style.display = 'none';
    if (!folderMode) okBtn.disabled = true;
    pathEl.textContent = '热门歌曲';
    bodyEl.innerHTML = '<div class="fp-loading">加载中...</div>';
    fetch('/api/music-proxy/api/music/hot')
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if(!pickerLive()||epoch!==scanEpoch)return;
        musicHot = data.ok ? (data.list || []) : [];
        renderMusicHot();
      })
      .catch(function() {
        if(!pickerLive()||epoch!==scanEpoch)return;
        bodyEl.innerHTML = '<div class="fp-empty">获取热门歌曲失败</div>';
      });
  }

  // 渲染热门歌曲列表（只显示歌名 + 下载量）
  function renderMusicHot() {
    if(!pickerLive())return;
    var list = searchTerm
      ? musicHot.filter(function(s) { return (s.name || '').toLowerCase().indexOf(searchTerm) !== -1; })
      : musicHot;
    // 音乐模式：始终显示「返回上级」
    var html = searchTerm ? '' :
      '<div class="fp-item fp-parent" data-music-back="1"><div class="fp-icon fp-icon-dir">' + ICONS.parentDir + '</div><div class="fp-info"><div class="fp-name">.. (返回上级目录)</div></div></div>';
    if (!list.length) {
      bodyEl.innerHTML = html + '<div class="fp-empty">' + (searchTerm ? '无匹配歌曲' : '暂无热门歌曲') + '</div>';
      bindMusicVirtualEvents();
      return;
    }
    for (var i = 0; i < list.length; i++) {
      var s = list[i];
      html += '<div class="fp-item" data-music-song="' + pickerEscape(s.name) + '">' +
        '<div class="fp-icon fp-icon-file">' + ICONS.musicNote + '</div>' +
        '<div class="fp-info"><div class="fp-name">' + escHtml(s.name) + '</div>' +
        '<div class="fp-meta">下载量:' + (s.heat || 0) + '</div></div>' +
        '</div>';
    }
    bodyEl.innerHTML = html;
    bindMusicVirtualEvents();
  }

  function bindMusicVirtualEvents() {
    bodyEl.querySelectorAll('.fp-item').forEach(function(el) {
      el.addEventListener('click', function() {
        if (el.dataset.musicBack) {
          navigateTo(currentPath);
          return;
        }
        var sname = el.dataset.musicSong;
        if (!sname) return;
        bodyEl.querySelectorAll('.fp-item').forEach(function(x) { x.style.background = ''; });
        el.style.background = 'var(--phx-primary-bg)';
        selectedPath = musicPrefix + sname;
        selectedName = sname;
        okBtn.disabled = false;
      });
    });
  }

  // ── 搜索 300ms 防抖 ──
  var debounceTimer = null;
  searchInput.addEventListener('input', function() {
    searchTerm = this.value.trim().toLowerCase();
    searchClear.style.display = searchTerm ? '' : 'none';
    clearTimeout(debounceTimer);
    if (virtualMode) {
      debounceTimer = setTimeout(function() { if (filter === 'music') renderMusicHot(); else renderVirtualPlayers(); }, 300);
    } else {
      debounceTimer = setTimeout(function() {
        renderData({path: currentPath, entries: dataList});
      }, 300);
    }
  });

  // ── 搜索清除按钮 ──
  searchClear.addEventListener('click', function() {
    searchInput.value = '';
    searchTerm = '';
    searchClear.style.display = 'none';
    if (virtualMode) {
      if (filter === 'music') renderMusicHot(); else renderVirtualPlayers();
    } else {
      renderData({path: currentPath, entries: dataList});
    }
    searchInput.focus();
  });

  // ── 排序弹窗 ──
  sortBtn.addEventListener('click', function() {
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

    so.querySelectorAll('.bs-option').forEach(function(el) {
      el.addEventListener('click', function() {
        var mode = el.dataset.sort;
        sortMode = mode;
        sortLabel.textContent = sortShort[mode];
        renderData({path: currentPath, entries: dataList});
        closeOverlay(so);
      });
    });
    document.getElementById('fp-sort-cancel').addEventListener('click', function() { closeOverlay(so); });
    so.addEventListener('click', function(e) { if (e.target === so) closeOverlay(so); });
  });

  // ── 导航 ──
  document.getElementById('fp-nav-back').addEventListener('click', function() {
    if (dataParent) {
      navigateTo(dataParent);
    }
  });

  document.getElementById('fp-cancel').addEventListener('click', function() { closePicker(true); });
  document.getElementById('fp-close').addEventListener('click', function(){closePicker(true)});
  document.getElementById('fp-upload-local').addEventListener('click',async function(){
    if(!pickerLive()){closePicker(true);return}if(folderMode)return;var epoch=scanEpoch;var upload=typeof yyStudioUploadFile==='function'?yyStudioUploadFile:null;if(!upload)return;
    var path=await upload(filter);if(!pickerLive()||epoch!==scanEpoch||!path)return;selectedPath=path;selectedName=path.split(/[\\/]/).pop();document.getElementById('fp-selected-label').textContent=selectedName;okBtn.disabled=false;
  });
  okBtn.addEventListener('click', function() {
    if(!pickerLive()){closePicker(true);return}
    if (folderMode) {
      if (onSelect) onSelect(currentPath);
    } else {
      if (!selectedPath) return;
      if (onSelect) onSelect(selectedPath);
    }
    closePicker(false);
  });

  navigateTo(currentPath);
}
