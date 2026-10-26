/* ═══════════════════════════════════════════════════════════════════
   MCORT 插件化主题框架 · 前端装载层
   -------------------------------------------------------------------
   由后端注入到 index.html 的 </body> 之前（见 youyuan_theme.go），
   因此在全部业务脚本之后执行：品牌署名最后落地，主题样式压不住它。

   做四件事：
     1) 强制注入署名「MCORT · 作者 疣螈」，并用 MutationObserver 复位
     2) 读取 /api/mcort/themes，装配当前主题的变量 / CSS / 槽位
     3) 提供 /#announce 公告页（查看 + 管理员发布）
     4) 列出 themes/ 与 plugins/，主题可切换、插件可记录启用（不执行 JS）

   术语：玩家=游戏内角色；用户/群友=QQ 侧的人；账号=登录工具箱的账号。
   ═══════════════════════════════════════════════════════════════════ */
(function () {
  'use strict';

  var BRAND_TEXT = 'MCORT · 作者 疣螈';
  var BRAND_ID = 'mcort-brand-signature';
  var state = {
    themes: [],
    active: '',
    plugins: [],
    announcements: [],
    admin: false,
    tab: 'announce',
    themeReady: false
  };

  /* ---------- 小工具 ---------- */

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = String(text);
    return n;
  }

  function api(path, method, body) {
    var opt = { method: method || 'GET', headers: { 'Content-Type': 'application/json' } };
    if (body) opt.body = JSON.stringify(body);
    return fetch(path, opt).then(function (r) {
      return r.json().catch(function () { return { ok: false, error: 'HTTP ' + r.status }; });
    }).catch(function (e) {
      return { ok: false, error: e && e.message ? e.message : '网络请求失败' };
    });
  }

  function toast(text, bad) {
    if (typeof T === 'function') { try { T(text, bad ? 'e' : 'o'); return; } catch (e) {} }
    if (typeof console !== 'undefined' && console.log) console.log('[MCORT] ' + text);
  }

  function fmtTime(ms) {
    if (!ms) return '';
    try { return new Date(ms).toLocaleString('zh-CN', { hour12: false }); } catch (e) { return ''; }
  }

  function assetURL(themeID, file) {
    return '/api/mcort/theme/asset?theme=' + encodeURIComponent(themeID) + '&file=' + encodeURIComponent(file);
  }

  /* ---------- 1. 品牌署名（硬编码层） ---------- */

  var brandNode = null;

  function brandStyle() {
    var bottom = (typeof window !== 'undefined' && window.innerWidth < 1024) ? '82px' : '12px';
    return [
      'position:fixed', 'left:12px', 'bottom:' + bottom, 'z-index:2147483000',
      'display:inline-flex', 'align-items:center', 'gap:6px',
      'visibility:visible', 'opacity:1', 'width:auto', 'height:auto',
      'overflow:visible', 'clip:auto', 'clip-path:none', 'transform:none',
      'font-size:11px', 'line-height:1.4', 'font-weight:600', 'letter-spacing:.2px',
      'color:#f4f7f5', 'background:rgba(23,41,43,.82)',
      'border:1px solid rgba(255,255,255,.14)', 'border-radius:8px',
      'padding:4px 9px', 'pointer-events:none', 'user-select:none',
      'white-space:nowrap', 'text-shadow:none', 'margin:0'
    ].join(' !important;') + ' !important';
  }

  function ensureBrand() {
    if (!document.body) return;
    if (!brandNode || !brandNode.isConnected) {
      brandNode = document.getElementById(BRAND_ID);
      if (!brandNode) {
        brandNode = el('div', 'mcort-brand-mark', BRAND_TEXT);
        brandNode.id = BRAND_ID;
        brandNode.setAttribute('data-mcort-hardcoded', '1');
        brandNode.setAttribute('aria-label', BRAND_TEXT);
        document.body.appendChild(brandNode);
      }
    }
    // 用内联 important 兜底：作者样式表再高的优先级也压不过内联 important
    brandNode.style.cssText = brandStyle();
    brandNode.className = 'mcort-brand-mark';
    if (brandNode.textContent !== BRAND_TEXT) brandNode.textContent = BRAND_TEXT;
  }

  function guardBrand() {
    ensureBrand();
    // 页面是 SPA，主题/业务脚本可能整块替换 body 子节点，这里监听删除并补回
    try {
      var mo = new MutationObserver(function () {
        if (!document.getElementById(BRAND_ID)) ensureBrand();
      });
      mo.observe(document.body, { childList: true, subtree: true });
    } catch (e) {}
    // 低频兜底：某些 WebView 上 MutationObserver 会被节流
    setInterval(function () {
      if (!document.getElementById(BRAND_ID)) ensureBrand();
    }, 2000);
    // 再写两处署名：meta 与 console，满足「多写几个署名」
    if (!document.querySelector('meta[name="mcort-author"]')) {
      var meta = document.createElement('meta');
      meta.name = 'mcort-author';
      meta.content = '疣螈';
      (document.head || document.documentElement).appendChild(meta);
    }
    try {
      if (console && console.log) {
        console.log('%c MCORT · 作者 疣螈 %c Minecraft Open Robot Toolbox ',
          'background:#17292b;color:#f4f7f5;border-radius:3px 0 0 3px;padding:2px 6px',
          'background:#d48a0e;color:#fff;border-radius:0 3px 3px 0;padding:2px 6px');
      }
    } catch (e) {}
  }

  /* ---------- 2. 主题装配 ---------- */

  function applyVariables(vars) {
    if (!vars) return;
    var root = document.documentElement;
    Object.keys(vars).forEach(function (k) {
      // 二次白名单：只允许 --mcort-theme-*，且不含品牌字样
      if (k.indexOf('--mcort-theme-') !== 0) return;
      if (/brand|author/i.test(k)) return;
      var v = String(vars[k] == null ? '' : vars[k]);
      if (/[<>{};\\]|expression\(|javascript:/i.test(v)) return;
      try { root.style.setProperty(k, v); } catch (e) {}
    });
  }

  function loadCSS(theme) {
    var old = document.getElementById('mcort-theme-css');
    if (old) old.remove();
    return fetch(assetURL(theme.id, 'theme.css'), { method: 'GET' }).then(function (r) {
      if (!r.ok) throw new Error('HTTP ' + r.status);
      return r.text();
    }).then(function (css) {
      var style = document.createElement('style');
      style.id = 'mcort-theme-css';
      style.setAttribute('data-mcort-theme-id', theme.id);
      style.textContent = css;
      // 插到 head 末尾：晚于 mcort_theme.css，早于署名内联样式（内联 important 仍赢）
      (document.head || document.documentElement).appendChild(style);
    });
  }

  function resolveSlot(theme, value) {
    var v = String(value == null ? '' : value).trim();
    if (!v) return null;
    if (/^assets\//.test(v)) {
      var img = el('img');
      img.src = assetURL(theme.id, v);
      img.alt = theme.name || theme.id;
      img.loading = 'lazy';
      return img;
    }
    return el('span', null, v);
  }

  function mountSlots(theme) {
    var slots = theme && theme.slots ? theme.slots : {};
    var hosts = {
      header: '#mcort-slot-header',
      sidebar_footer: '#mcort-slot-sidebar-footer',
      floating: '#mcort-slot-floating'
    };
    Object.keys(hosts).forEach(function (name) {
      var host = document.getElementById(hosts[name].slice(1));
      if (!host) return;
      var keep = host.querySelector('.mcort-fab');
      while (host.firstChild) host.removeChild(host.firstChild);
      if (keep) host.appendChild(keep);
      if (!slots[name]) return;
      // 槽位只在主题校验通过且激活时渲染；requires 不满足时后端已把主题判为不可用
      var node = resolveSlot(theme, slots[name]);
      if (node) host.appendChild(node);
    });
    var headerHost = document.getElementById('mcort-slot-header');
    if (headerHost && headerHost.parentNode) headerHost.parentNode.style.display = '';
  }

  function ensureSlotHosts() {
    if (!document.body) return;
    if (!document.getElementById('mcort-slot-header')) {
      var head = el('div', 'mcort-slot');
      head.id = 'mcort-slot-header';
      var bar = document.querySelector('.cb-center') || document.querySelector('.conn-bar');
      if (bar) bar.appendChild(head);
      else {
        head.style.position = 'fixed';
        head.style.top = '6px';
        head.style.right = '10px';
        head.style.zIndex = '2147482000';
        document.body.appendChild(head);
      }
    }
    if (!document.getElementById('mcort-slot-sidebar-footer')) {
      var side = el('div', 'mcort-slot');
      side.id = 'mcort-slot-sidebar-footer';
      var foot = document.querySelector('.yy-sidebar-footer');
      if (foot) foot.appendChild(side);
    }
    if (!document.getElementById('mcort-slot-floating')) {
      var float = el('div', 'mcort-slot');
      float.id = 'mcort-slot-floating';
      document.body.appendChild(float);
    }
    var floatHost = document.getElementById('mcort-slot-floating');
    if (floatHost && !floatHost.querySelector('.mcort-fab')) {
      var fab = el('button', 'mcort-fab', '公告');
      fab.type = 'button';
      fab.title = '打开公告页（/#announce）';
      fab.addEventListener('click', function () {
        if (location.hash === '#announce') syncHash();
        else location.hash = '#announce';
      });
      floatHost.appendChild(fab);
    }
  }

  function clearTheme() {
    document.documentElement.removeAttribute('data-mcort-theme');
    var old = document.getElementById('mcort-theme-css');
    if (old) old.remove();
    var root = document.documentElement;
    for (var i = root.style.length - 1; i >= 0; i--) {
      var p = root.style[i];
      if (p && p.indexOf('--mcort-theme-') === 0) root.style.removeProperty(p);
    }
    mountSlots(null);
  }

  function loadTheme() {
    return api('/api/mcort/themes').then(function (r) {
      if (!r || !r.ok) {
        console.warn('[MCORT] 主题接口不可用', r && r.error);
        return;
      }
      state.themes = r.themes || [];
      state.active = r.active || '';
      if (!state.active) {
        clearTheme();
        return;
      }
      var theme = null;
      for (var i = 0; i < state.themes.length; i++) {
        if (state.themes[i].id === state.active && state.themes[i].ok) { theme = state.themes[i]; break; }
      }
      if (!theme) {
        console.warn('[MCORT] 启用中的主题不可用，已回退默认外观');
        clearTheme();
        return;
      }
      document.documentElement.setAttribute('data-mcort-theme', theme.id);
      applyVariables(theme.variables);
      mountSlots(theme);
      return loadCSS(theme).then(function () {
        state.themeReady = true;
      }).catch(function (e) {
        console.warn('[MCORT] 主题样式加载失败', e);
        toast('主题样式加载失败：' + (e && e.message ? e.message : e), true);
      });
    });
  }

  /* ---------- 3. 公告页 /#announce ---------- */

  var page = null;

  function ensurePage() {
    if (page) return page;
    page = el('div');
    page.id = 'mcort-announce-page';
    page.hidden = true;

    var head = el('div', 'mcort-ann-head');
    var title = el('h2', null, '公告');
    var sub = el('small', null, 'MCORT · 作者 疣螈');
    var close = el('button', 'mcort-btn', '返回');
    close.type = 'button';
    close.addEventListener('click', function () { closePage(); });
    head.appendChild(title);
    head.appendChild(sub);
    head.appendChild(close);

    var tabs = el('div', 'mcort-ann-head');
    tabs.style.borderTop = '0';
    tabs.style.padding = '8px 18px';
    var tabAnn = el('button', 'mcort-btn primary', '公告');
    var tabTheme = el('button', 'mcort-btn', '主题');
    var tabPlg = el('button', 'mcort-btn', '玩法');
    [tabAnn, tabTheme, tabPlg].forEach(function (b) { b.type = 'button'; });
    tabAnn.addEventListener('click', function () { switchTab('announce'); });
    tabTheme.addEventListener('click', function () { switchTab('theme'); });
    tabPlg.addEventListener('click', function () { switchTab('plugins'); });
    tabs.appendChild(tabAnn);
    tabs.appendChild(tabTheme);
    tabs.appendChild(tabPlg);

    var body = el('div', 'mcort-ann-body');
    body.id = 'mcort-ann-body';

    var foot = el('div', 'mcort-ann-foot', 'MCORT · 作者 疣螈 · 公告与主题由本机管理员维护');

    page.appendChild(head);
    page.appendChild(tabs);
    page.appendChild(body);
    page.appendChild(foot);
    document.body.appendChild(page);
    page._tabs = { announce: tabAnn, theme: tabTheme, plugins: tabPlg };
    return page;
  }

  function switchTab(tab) {
    state.tab = tab;
    ensurePage();
    var map = page._tabs || {};
    Object.keys(map).forEach(function (k) {
      map[k].className = 'mcort-btn' + (k === tab ? ' primary' : '');
    });
    if (tab === 'announce') renderAnnouncements();
    else if (tab === 'theme') renderThemes();
    else renderPlugins();
  }

  function openPage(tab) {
    ensurePage();
    page.hidden = false;
    switchTab(tab || state.tab || 'announce');
  }

  function closePage() {
    if (page) page.hidden = true;
    try { history.replaceState(null, '', location.pathname + location.search); } catch (e) {}
  }

  function syncHash() {
    var h = (location.hash || '').replace(/^#/, '');
    if (h === 'announce') openPage('announce');
    else if (h === 'announce/theme') openPage('theme');
    else if (h === 'announce/plugins') openPage('plugins');
    else if (page) page.hidden = true;
  }

  function renderAnnouncements() {
    var body = document.getElementById('mcort-ann-body');
    if (!body) return;
    body.textContent = '';
    var list = el('div', 'mcort-ann-list');

    return api('/api/mcort/announcements').then(function (r) {
      if (!r || !r.ok) {
        list.appendChild(el('div', 'mcort-ann-empty', '公告读取失败：' + ((r && r.error) || '未知错误')));
        body.appendChild(list);
        return;
      }
      state.announcements = r.announcements || [];
      state.admin = !!r.admin;
      if (!state.announcements.length) {
        list.appendChild(el('div', 'mcort-ann-empty', '还没有公告。管理员可以在下方发布第一条。'));
      }
      state.announcements.forEach(function (a) {
        var card = el('div', 'mcort-ann-item');
        card.setAttribute('data-sev', a.severity || 'info');
        var h = el('h3');
        if (a.pinned) h.appendChild(el('span', 'mcort-ann-pin', '置顶'));
        h.appendChild(el('span', null, a.title || ''));
        card.appendChild(h);
        card.appendChild(el('div', 'mcort-ann-content', a.content || ''));
        var meta = el('div', 'mcort-ann-meta');
        meta.appendChild(el('span', null, (a.author ? '作者 ' + a.author : '') + (a.created_at ? ' · ' + fmtTime(a.created_at) : '')));
        if (state.admin) {
          var del = el('button', 'mcort-btn danger', '删除');
          del.type = 'button';
          del.addEventListener('click', function () {
            if (!confirm('删除这条公告？')) return;
            api('/api/mcort/announcements', 'POST', { op: 'delete', id: a.id }).then(function (x) {
              if (x && x.ok) { toast('已删除', false); renderAnnouncements(); }
              else toast((x && x.error) || '删除失败', true);
            });
          });
          meta.appendChild(del);
        }
        card.appendChild(meta);
        list.appendChild(card);
      });
      body.appendChild(list);
      body.appendChild(buildForm());
    });
  }

  function buildForm() {
    var box = el('div', 'mcort-ann-form');
    var h = el('h3', null, state.admin ? '发布公告' : '只有管理员可以发布公告');
    box.appendChild(h);
    if (!state.admin) {
      box.appendChild(el('div', 'mcort-ann-empty', '当前身份没有写入权限。绑定账号后，站长/管理员可在本机直接发布。'));
      return box;
    }
    var t = el('input');
    t.type = 'text';
    t.placeholder = '标题（≤80 字）';
    t.id = 'mcort-ann-title';
    var c = el('textarea');
    c.placeholder = '正文（≤8000 字，纯文本，换行会保留）';
    c.id = 'mcort-ann-content';
    var sev = el('select');
    sev.id = 'mcort-ann-sev';
    [['info', '普通'], ['warning', '提醒'], ['critical', '重要']].forEach(function (o) {
      var op = el('option', null, o[1]);
      op.value = o[0];
      sev.appendChild(op);
    });
    var pin = el('input');
    pin.type = 'checkbox';
    pin.id = 'mcort-ann-pin';
    var pinLabel = el('label', null, '置顶');
    pinLabel.appendChild(pin);

    var row = el('div', 'mcort-ann-row');
    row.appendChild(sev);
    row.appendChild(pinLabel);

    var actions = el('div', 'mcort-ann-actions');
    var send = el('button', 'mcort-btn primary', '发布');
    send.type = 'button';
    send.addEventListener('click', function () {
      var title = (t.value || '').trim();
      var content = (c.value || '').trim();
      if (!title) { toast('标题不能为空', true); return; }
      send.disabled = true;
      api('/api/mcort/announcements', 'POST', {
        op: 'create', title: title, content: content,
        severity: sev.value, pinned: !!pin.checked
      }).then(function (x) {
        send.disabled = false;
        if (x && x.ok) { toast('已发布', false); renderAnnouncements(); }
        else toast((x && x.error) || '发布失败', true);
      });
    });
    actions.appendChild(send);

    box.appendChild(el('label', null, '标题'));
    box.appendChild(t);
    box.appendChild(el('label', null, '正文'));
    box.appendChild(c);
    box.appendChild(row);
    box.appendChild(actions);
    return box;
  }

  /* ---------- 4. 主题面板 ---------- */

  function renderThemes() {
    var body = document.getElementById('mcort-ann-body');
    if (!body) return;
    body.textContent = '';
    var list = el('div', 'mcort-ann-list');
    return loadTheme().then(function () {
      var head = el('div', 'mcort-ann-item');
      head.appendChild(el('h3', null, '当前主题：' + (state.active || '默认（未启用主题）')));
      head.appendChild(el('div', 'mcort-ann-content',
        state.active ? '主题包只改配色、背景、圆角、字号与挂件槽位；MCORT 标识与作者署名属于硬编码层，任何主题都改不掉。'
          : '未启用主题。默认外观就是工具箱出厂样式，署名仍会显示。'));
      list.appendChild(head);

      if (!state.themes.length) {
        var empty = el('div', 'mcort-ann-empty');
        empty.appendChild(el('div', null, '还没有安装主题。'));
        var hint = el('div', null, '把主题包放到数据目录的 themes/<id>/，需要 theme.json + theme.css。');
        empty.appendChild(hint);
        list.appendChild(empty);
      }

      state.themes.forEach(function (t) {
        var card = el('div', 'mcort-ann-item');
        var h = el('h3');
        h.appendChild(el('span', null, t.name || t.id));
        if (t.id === state.active) h.appendChild(el('span', 'mcort-ann-pin', '使用中'));
        if (!t.ok) h.appendChild(el('span', 'mcort-ann-pin', '不可用'));
        card.appendChild(h);
        card.appendChild(el('div', 'mcort-ann-content',
          'id ' + t.id + ' · v' + (t.version || '?') + ' · 作者 ' + (t.author || '未署名')
            + (t.error ? '\n拒绝原因：' + t.error : '')));
        if (state.admin) {
          var row = el('div', 'mcort-ann-actions');
          if (t.ok && t.id !== state.active) {
            var use = el('button', 'mcort-btn primary', '应用');
            use.type = 'button';
            use.addEventListener('click', function () {
              api('/api/mcort/theme', 'POST', { id: t.id }).then(function (x) {
                if (x && x.ok) { toast('已切换主题，刷新后完全生效', false); renderThemes(); }
                else toast((x && x.error) || '切换失败', true);
              });
            });
            row.appendChild(use);
          }
          card.appendChild(row);
        }
        list.appendChild(card);
      });

      if (state.admin && state.active) {
        var resetRow = el('div', 'mcort-ann-actions');
        var reset = el('button', 'mcort-btn', '恢复默认外观');
        reset.type = 'button';
        reset.addEventListener('click', function () {
          api('/api/mcort/theme', 'POST', { id: '' }).then(function (x) {
            if (x && x.ok) { toast('已恢复默认外观，刷新后完全生效', false); renderThemes(); }
            else toast((x && x.error) || '操作失败', true);
          });
        });
        resetRow.appendChild(reset);
        list.appendChild(resetRow);
      }
      body.appendChild(list);
    });
  }

  /* ---------- 5. 玩法插件面板（只记录，不执行） ---------- */

  function renderPlugins() {
    var body = document.getElementById('mcort-ann-body');
    if (!body) return;
    body.textContent = '';
    var list = el('div', 'mcort-ann-list');
    return api('/api/mcort/plugins').then(function (r) {
      if (!r || !r.ok) {
        list.appendChild(el('div', 'mcort-ann-empty', '插件列表读取失败：' + ((r && r.error) || '未知错误')));
        body.appendChild(list);
        return;
      }
      state.plugins = r.plugins || [];
      var note = el('div', 'mcort-ann-item');
      note.appendChild(el('h3', null, '玩法插件（骨架阶段）'));
      note.appendChild(el('div', 'mcort-ann-content',
        '当前只做「发现 + 列出 + 记录启用」，不会加载或执行任何插件 main.js。'
          + '\n执行层要等显式白名单 + 二次确认，避免把任意 JS 直接放进工具箱进程。'));
      list.appendChild(note);

      if (!state.plugins.length) {
        list.appendChild(el('div', 'mcort-ann-empty', '还没有插件。放到数据目录的 plugins/<id>/，需要 plugin.json + main.js。'));
      }

      state.plugins.forEach(function (p) {
        var card = el('div', 'mcort-ann-item');
        var h = el('h3');
        h.appendChild(el('span', null, p.name || p.id));
        if (p.enabled) h.appendChild(el('span', 'mcort-ann-pin', '已启用（未执行）'));
        if (!p.ok) h.appendChild(el('span', 'mcort-ann-pin', '不可用'));
        card.appendChild(h);
        card.appendChild(el('div', 'mcort-ann-content',
          'id ' + p.id + ' · v' + (p.version || '?') + ' · 作者 ' + (p.author || '未署名')
            + (p.description ? '\n' + p.description : '')
            + (p.error ? '\n拒绝原因：' + p.error : '')));
        if (state.admin && p.ok) {
          var row = el('div', 'mcort-ann-actions');
          var btn = el('button', 'mcort-btn' + (p.enabled ? '' : ' primary'), p.enabled ? '停用' : '启用');
          btn.type = 'button';
          btn.addEventListener('click', function () {
            var want = !p.enabled;
            if (!confirm((want ? '启用' : '停用') + '插件「' + (p.name || p.id) + '」？\n\n当前版本只记录状态，不会执行插件代码。')) return;
            api('/api/mcort/plugins', 'POST', {
              op: want ? 'enable' : 'disable', id: p.id, confirm: true
            }).then(function (x) {
              if (x && x.ok) { toast(x.note || '已记录', false); renderPlugins(); }
              else toast((x && x.error) || '操作失败', true);
            });
          });
          row.appendChild(btn);
          card.appendChild(row);
        }
        list.appendChild(card);
      });
      body.appendChild(list);
    });
  }

  /* ---------- 启动 ---------- */

  function boot() {
    ensureSlotHosts();
    guardBrand();
    ensurePage();
    syncHash();
    loadTheme();
    // 先探一次读接口，拿到 admin 标记：直接打开 #announce/theme 时也能看到管理按钮
    api('/api/mcort/announcements').then(function (r) {
      if (r && r.ok) state.admin = !!r.admin;
    });
    window.addEventListener('hashchange', syncHash);

    window.mcortTheme = {
      version: 1,
      reload: loadTheme,
      openAnnounce: function () { location.hash = '#announce'; },
      openThemes: function () { location.hash = '#announce/theme'; },
      openPlugins: function () { location.hash = '#announce/plugins'; },
      state: state,
      brand: BRAND_TEXT
    };
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
