/* Reparent live controls, never clone them: input values and existing handlers survive. */
(function(){
 let observer=null,scheduled=false;
 const definitions=[
  ['connect','连接与账号','服务器、凭据、保活与多机器人'],
  ['remote','群聊与远程控制','群服互通、MCP与公网连接'],
  ['jobs','建筑与创作','导入导出、地图画和作业默认值'],
  ['appearance','界面与文件','主题、背景与文件选择'],
  ['system','维护与资料','提示文案、文档、云端资料和维护']
 ];
 function category(title){if(/群服|QQ群|MCP|公网|隧道|远控/.test(title))return 'remote';if(/服务器连接|验证服务器|账号|机器人|全局抓包|模式维护|保活/.test(title))return 'connect';if(/速度|导入|导出|作业|地图画/.test(title))return 'jobs';if(/界面|主题|外观|文件选择|背景/.test(title))return 'appearance';return 'system'}
 function saved(key,fallback){try{const v=localStorage.getItem('yy-settings-fold:'+key);return v===null?fallback:v==='1'}catch{return fallback}}
 function remember(node,key){node.addEventListener('toggle',()=>{try{localStorage.setItem('yy-settings-fold:'+key,node.open?'1':'0')}catch{}})}
 function group(){
  const root=document.querySelector('#tab-settings>.content-inner');if(!root||curTab!=='settings'||subPageOpen)return;
  let container=root.querySelector(':scope>.yy-settings-groups');
  if(!container){container=document.createElement('div');container.className='yy-settings-groups';const grid=root.querySelector(':scope>.yy-form-grid');(grid||root.firstElementChild).after(container);for(const [id,title,desc]of definitions){const d=document.createElement('details');d.className='yy-settings-group';d.dataset.group=id;d.open=saved(id,id==='connect');const summary=document.createElement('summary'),name=document.createElement('strong'),description=document.createElement('span');name.textContent=title;description.textContent=desc;summary.append(name,description);d.append(summary);remember(d,id);container.append(d)}}
  const cards=[...root.querySelectorAll('.card')].filter(c=>!c.closest('.yy-settings-groups')&&!c.parentElement.closest('.card'));
  for(const card of cards){const title=(card.querySelector('h3')?.textContent||card.querySelector('.yy-form-head')?.textContent||'').trim();
   if(card.id==='yy-settings-save'||card.id==='yy-settings-save-card'||card.querySelector('#yy-settings-save')||(!title&&card.querySelector('[onclick="saveCfg()"]')))continue;
   const module=document.createElement('details');module.className='yy-settings-module';const summary=document.createElement('summary');summary.textContent=title||'其他设置';module.append(summary);const key=card.id||title;module.open=saved('module:'+key,card.id==='cfg-conn-card');remember(module,'module:'+key);module.append(card);container.querySelector('[data-group="'+category(title)+'"]').append(module)
  }
  for(const stack of root.querySelectorAll('.yy-form-stack'))if(!stack.children.length)stack.remove();
  for(const grid of root.querySelectorAll(':scope>.yy-form-grid'))if(!grid.children.length)grid.remove();
 }
 const base=Rcfg;Rcfg=async function(){if(observer)observer.disconnect();await base.apply(this,arguments);if(curTab!=='settings'||subPageOpen)return;group();const root=document.getElementById('tab-settings');observer=new MutationObserver(records=>{if(!records.some(r=>[...r.addedNodes].some(n=>n.nodeType===1&&(n.matches('.card')||n.querySelector('.card'))))||scheduled)return;scheduled=true;requestAnimationFrame(()=>{scheduled=false;group()})});observer.observe(root,{childList:true,subtree:true});activeRenderers.push(()=>{observer?.disconnect()})};
})();
