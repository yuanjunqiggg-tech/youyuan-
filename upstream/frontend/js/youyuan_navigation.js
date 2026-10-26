/* Organise existing capabilities around player tasks, without extra requests. */
(function(){
 'use strict';
 const names={home:'工具总览',tasks:'插件中心',terminal:'终端',settings:'连接与设置',accounts:'游戏账号'};
 function navButton(tab,label,icon){const b=document.createElement('button');b.dataset.yyTab=tab;b.innerHTML=yyIcon(icon)+label;b.onclick=()=>yyNavigate(tab);return b}
 function moduleButton(id,label,icon){const b=document.createElement('button');b.innerHTML=yyIcon(icon)+label;b.onclick=()=>yyOpenModule(id);return b}
 const side=document.querySelector('.yy-sidebar');if(side){side.querySelectorAll('.yy-sidebar-label,.yy-side-nav').forEach(n=>n.remove());const fragment=document.createDocumentFragment();
  for(const [name,buttons] of [
   ['工作空间',[navButton('home','工具总览','home'),navButton('tasks','插件中心','plugin'),navButton('terminal','终端','terminal')]],
   ['快捷操作',[moduleButton('import','导入建筑','import'),moduleButton('export','导出建筑','export'),moduleButton('mcfunction','命令链','code')]],
   ['连接与资料',[navButton('settings','连接与设置','connection'),navButton('accounts','游戏账号','user'),moduleButton('qq','群服互通','chat'),moduleButton('remote','远程设备','cloud')]]
  ]){const label=document.createElement('div');label.className='yy-sidebar-label';label.textContent=name;const nav=document.createElement('nav');nav.className='yy-side-nav';nav.append(...buttons);fragment.append(label,nav)}
  const footer=side.querySelector('.yy-sidebar-footer');side.insertBefore(fragment,footer);if(footer)footer.querySelector('strong').textContent='把想法，放进世界';
 }
 // Terminal is a first-class destination on every screen width.
 const nav=document.querySelector('.nav');if(nav){nav.classList.add('yy-player-navigation');const labels={home:'总览',tasks:'插件',terminal:'终端',settings:'连接',accounts:'账号'};for(const [id,name]of Object.entries(labels)){const b=nav.querySelector('[data-tab="'+id+'"]');if(!b)continue;b.hidden=false;b.setAttribute('aria-label',names[id]);const node=[...b.childNodes].find(n=>n.nodeType===3&&n.textContent.trim());if(node)node.textContent=name;else{const span=b.querySelector('span:not(.ni)');if(span)span.textContent=name}}
 }
 const sync=yySyncNav;yySyncNav=function(tab){sync(tab);const crumb=document.getElementById('yy-crumb');if(crumb){crumb.replaceChildren(document.createTextNode(names[tab]||'游园'))}};
 const render=Rcfg;Rcfg=async function(){const owner=typeof yyReadScope==='function'?yyReadScope():'';await render.apply(this,arguments);if((typeof yyReadScope==='function'&&yyReadScope()!==owner)||(typeof curTab!=='undefined'&&curTab!=='settings')||(typeof subPageOpen!=='undefined'&&subPageOpen))return;const root=document.querySelector('#tab-settings>.content-inner');if(!root||root.querySelector('#yy-player-shortcuts'))return;const actions=document.createElement('div');actions.id='yy-player-shortcuts';actions.className='yy-player-shortcuts';const live=navButton('terminal','打开终端','terminal'),xp=document.createElement('button');xp.className='yy-secondary';xp.textContent='角色等级与经验';xp.onclick=()=>yyExperiencePage();actions.append(live,xp);root.prepend(actions)};
})();

/* 手机二级导航（≤768px）
   为什么需要：子页面是 position:fixed 的全屏覆盖层（layout.css:105），
   打开「群服互通」「服务器信息」这类二级页时，底部主导航被整块盖住，
   加上 app.js 进子页会给 #bottom-nav 加 .gone，结果就是「进得去、出不来」。
   这里补一条 56px 的四格跳转条，只在子页面打开时出现，桌面端不显示。
   四个格子刻意做成「目的地」而不是「上一级」：手机上返回已有 sp-back 左上角，
   缺的是横向跳到另一块的能力。 */
(function () {
  'use strict';
  var bar = document.createElement('nav');
  bar.className = 'yy-sub-nav';
  bar.id = 'yy-sub-nav';
  bar.setAttribute('aria-label', '二级导航');
  bar.innerHTML = [
    ['home', '首页', 'home'],
    ['hub', '互通', 'chat'],
    ['tools', '工具', 'plugin'],
    ['me', '我的', 'user']
  ].map(function (n) {
    return '<button type="button" data-yy-sub="' + n[0] + '" aria-label="' + n[1] +
      '" onclick="yySubNav(\'' + n[0] + '\')"><i class="ni">' + yyIcon(n[2]) +
      '</i><span>' + n[1] + '</span></button>';
  }).join('');
  document.getElementById('app').appendChild(bar);

  function sync(key) {
    bar.querySelectorAll('[data-yy-sub]').forEach(function (b) {
      var on = b.dataset.yySub === key;
      b.classList.toggle('is-on', on);
      if (on) b.setAttribute('aria-current', 'page');
      else b.removeAttribute('aria-current');
    });
  }
  window.yySubNavSync = sync;

  // 跳转表：首页/工具/我的 走现有 tab；互通与「我的」用一张列表页，
  // 因为「互通」包含 4 个模块、「我的」包含账号与设置两块，塞进一个 tab 反而藏东西。
  var PAGES = {
    hub: {
      title: '互通',
      desc: 'QQ 群与游戏服务器之间的连接、消息与文件，都在这里。',
      rows: [
        ['qq', '群服互通', '机器人接入、能力开关与 AI 主动接话', 'chat'],
        ['remote', '公网远控', '设备连接与 MCP 密钥', 'cloud'],
        ['library', '云端资料库', '插件、建筑与音乐文件', 'folder'],
        ['settings', '服务器连接', '房间号、验证服务器与登录 Token', 'connection']
      ]
    },
    me: {
      title: '我的',
      desc: '游戏账号、连接参数与界面偏好。',
      rows: [
        ['accounts', '游戏账号', '多账号托管、状态与登录', 'user'],
        ['settings', '账号与设置', '服务器连接、群服互通与公网控制', 'settings'],
        ['personalize', '个性化', '配色、背景与常用工具排序', 'image']
      ]
    }
  };

  function openList(key) {
    var page = PAGES[key];
    var html = '<p class="yy-sub-hub-desc">' + page.desc + '</p><div class="yy-sub-hub">' +
      page.rows.map(function (r) {
        return '<button type="button" class="yy-sub-hub-row" onclick="yySubNavOpen(\'' + key + '\',\'' + r[0] + '\')">' +
          '<span class="yy-sub-hub-ico">' + yyIcon(r[3]) + '</span>' +
          '<span class="yy-sub-hub-txt"><strong>' + r[1] + '</strong><small>' + r[2] + '</small></span>' +
          '<span class="yy-sub-hub-go" aria-hidden="true">›</span></button>';
      }).join('') + '</div>';
    openSubPage(page.title, html);
    sync(key);
  }

  window.yySubNav = function (key) {
    if (key === 'home') { yyNavigate('home'); return; }
    if (key === 'tools') { yyNavigate('tasks'); return; }
    openList(key);
  };

  window.yySubNavOpen = function (pageKey, id) {
    if (id === 'accounts' || id === 'settings') { yyNavigate(id); return; }
    if (id === 'personalize') {
      if (typeof openPersonalizeSheet === 'function') openPersonalizeSheet();
      return;
    }
    // 走公共入口，才能同时记录「最近使用」并统一处理打开失败
    if (typeof yyOpenModule === 'function') yyOpenModule(id);
    // 从「互通」进的模块仍算互通，避免整条 bar 没有高亮让人以为点错了
    sync(pageKey);
  };

  // 子页面标题里带「互通」的都归到互通格子；其余二级页不高亮任何格子，
  // 因为它们不在这四个目的地里，硬点亮一个反而是错的。
  var openBase = openSubPage;
  window.openSubPage = function (title, html) {
    openBase(title, html);
    sync(/互通/.test(title || '') ? 'hub' : '');
  };

  var syncBase = yySyncNav;
  yySyncNav = function (tab) {
    syncBase(tab);
    sync(tab === 'home' ? 'home' : tab === 'tasks' ? 'tools' : tab === 'accounts' ? 'me' : '');
  };
})();

/* 手机底部一级导航补「互通」（≤1023px）
   为什么必须补：桌面侧边栏 .yy-sidebar 只在 ≥1024px 才 display:flex
   （youyuan_design.css:9 默认 none，:26 才显示）；上面那条 .yy-sub-nav 又要
   body.sp-open（已经进了某个二级页）才 display:flex（youyuan_agent_mobile.css:727 / :782）。
   于是手机首屏的一级导航 .nav 只有 总览/插件/终端/连接/账号 五项，而
   「群服互通」是 yyModules 里 id=qq 的模块（youyuan_dashboard.js:52），桌面靠侧边栏
   moduleButton('qq') 进，手机一个一级入口都没有 —— 这就是「功能在手机上看不到」的真实原因，
   不是权限：yyModuleAllowed 只拦 admin 的 API 目录（youyuan_dashboard.js:101），qq 不受限。
   改法：在 .nav 里追加第 6 个入口，直接调 yyOpenModule('qq')。.nav 在 ≥1024px 是
   display:none!important（youyuan_design.css:26），桌面导航完全不受影响。
   360px 下 6 项各约 52px 宽、53px 高，仍 ≥44px 触控目标。 */
(function () {
  'use strict';
  var nav = document.getElementById('bottom-nav');
  if (!nav || nav.querySelector('[data-tab="hub"]')) return;

  var btn = document.createElement('button');
  btn.type = 'button';
  btn.dataset.tab = 'hub';
  btn.className = 'yy-nav-hub';
  btn.setAttribute('aria-label', '群服互通');
  btn.innerHTML = '<i class="ni">' + yyIcon('chat') + '</i>互通';
  // 放在「终端」之后、「连接」之前，和桌面侧边栏「连接与资料」那组的顺序一致
  nav.insertBefore(btn, nav.querySelector('[data-tab="settings"]'));

  function openHub() {
    if (typeof yyOpenModule === 'function') yyOpenModule('qq');
    else if (typeof yyQQPage === 'function') yyQQPage();
  }

  // app.js 的按钮点击绑定只覆盖首屏就存在的 5 个 tab，这个是后加的，得自己接。
  // 拖拽切页（app.js initNavDrag → tabTo）也会走到 tabTo，所以一并接管 hub 键，
  // 其余 tab 原样交还给 dashboard.js 包装过的 tabTo。
  btn.addEventListener('click', function () {
    if (window._navDrag && _navDrag.moved) {
      _navDrag.moved = false;
      return;
    }
    openHub();
  });
  var baseTabTo = tabTo;
  window.tabTo = function (t) {
    if (t === 'hub') {
      openHub();
      return;
    }
    return baseTabTo.apply(this, arguments);
  };
})();

/* 种子查询页按需加载（index.html 不改）
   为什么在这里自举：工具页入口必须由页面脚本自己注册，但脚本没被 index.html 引用就永远不会执行。
   这里在导航脚本末尾挂一个一次性加载器：拉 CSS + 本机算法 + 页面脚本，后两者按顺序执行，
   加载完成后由 youyuan_seed_page.js 自己 push 到 yyModules（category=building，和导入/导出同级）。 */
(function () {
  if (document.getElementById('yy-seed-boot')) return;
  var link = document.createElement('link');
  link.id = 'yy-seed-css';
  link.rel = 'stylesheet';
  link.href = '/css/youyuan_seed_page.css?v=2026102401';
  document.head.appendChild(link);
  var local = document.createElement('script');
  local.id = 'yy-seed-boot';
  local.src = '/js/youyuan_seed_local.js?v=2026102401';
  local.async = false;
  local.onload = function () {
    var page = document.createElement('script');
    page.id = 'yy-seed-page-js';
    page.src = '/js/youyuan_seed_page.js?v=2026102401';
    page.async = false;
    document.head.appendChild(page);
  };
  local.onerror = function () { console.warn('种子查询：本机算法脚本加载失败'); };
  document.head.appendChild(local);
})();
