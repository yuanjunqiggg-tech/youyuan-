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
