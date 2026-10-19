(function(){
 'use strict';
 var pending=false, choosing=false, sequence=0;
 var labels={rental:'租赁服',lobby:'联机房间',local:'本地房间'};
 function fields(){return {token:document.getElementById('cfg-token')?.value||'',auth:curServerUrl,input:document.getElementById('bot-server')?.value.trim()||''}}
 function same(a,b){return (!a.stillLive||a.stillLive())&&a.token===b.token&&a.auth===b.auth&&a.input===b.input}
 function el(tag,text,cls){var n=document.createElement(tag);if(text!=null)n.textContent=text;if(cls)n.className=cls;return n}
 function image(url,cls){if(!/^https:\/\//i.test(url||''))return null;var n=el('img',null,cls);n.src=url;n.alt='';n.referrerPolicy='no-referrer';n.onerror=function(){n.remove()};return n}
 function host(){var input=document.getElementById('bot-server');if(!input)return null;var box=document.getElementById('yy-room-results');if(!box){box=el('section',null,'yy-room-results');box.id='yy-room-results';input.parentElement.insertAdjacentElement('afterend',box)}return box}
 function clear(){sequence++;document.getElementById('yy-room-results')?.replaceChildren()}
 async function start(candidate,original){
  if(!same(original,fields()))return;
  return previous();
 }
 async function choose(candidate,original){
  if(choosing||!same(original,fields()))return;choosing=true;
  try {var result=await A('POST','/api/youyuan/rooms',Object.assign({op:'select',candidate_id:candidate.id},original));
   if(!same(original,fields()))return;
   if(!result.ok){T(result.error||'保存选择失败','e');return}
   renderChoice(result.selected,original);await start(result.selected,original);
  }finally{choosing=false}
 }
 function renderChoice(candidate,original){var box=host();if(!box)return;box.replaceChildren();box.append(el('p','已选择 '+labels[candidate.type]+' · '+(candidate.name||candidate.number||candidate.room_id)));var change=el('button','重新查询','btn-s');change.type='button';change.onclick=function(){discover(true)};box.append(change)}
 function render(result,original){
  var box=host();if(!box)return;box.replaceChildren();
  if(!result.complete)box.append(el('p','部分查询未完成，请查看下方状态；可以明确选择已找到的房间。','yy-room-hint'));
  var rows=result.candidates||[];
  Object.keys(labels).forEach(function(kind){var group=rows.filter(function(v){return v.type===kind});if(!group.length)return;var section=el('section',null,'yy-room-group');section.append(el('h4',labels[kind]+' · '+group.length));var grid=el('div',null,'yy-room-grid');group.forEach(function(v){
   var card=el('article',null,'yy-room-card');var cover=image(v.cover_url,'yy-room-cover');if(cover)card.append(cover);var content=el('div',null,'yy-room-content');var avatar=image(v.avatar_url,'yy-room-avatar');if(avatar)content.append(avatar);content.append(el('strong',v.name||v.number||v.room_id));
   var details=[labels[kind],v.number?'号码 '+v.number:'ID '+v.room_id];if(v.owner)details.push('房主 '+v.owner);else if(v.owner_id)details.push('房主 UID '+v.owner_id);if(v.players!=null)details.push('在线 '+v.players+(v.max_players!=null?' / '+v.max_players:''));else if(v.max_players!=null)details.push('容量 '+v.max_players);if(v.version)details.push(v.version);content.append(el('p',details.join(' · ')));
   var button=el('button','选择并连接','btn');button.type='button';button.onclick=async function(){button.disabled=true;try{await choose(v,original)}finally{button.disabled=false}};content.append(button);card.append(content);grid.append(card)
  });section.append(grid);box.append(section)});
  (result.sources||[]).filter(function(s){return !s.ok||s.more}).forEach(function(s){box.append(el('p',labels[s.type]+'：'+(s.more?'还有更多匹配，请输入更具体的号码或房名。':s.error||'查询失败'),'yy-room-hint'))});
  if(!rows.length)box.append(el('p',result.complete?'没有找到匹配的房间，请确认号码、房名或分享链接。':'当前未找到候选，待失败类别恢复后重试。'));
 }
 async function discover(force){
  if(pending)return;var original=fields();if(typeof yySettingsViewGuard==='function')Object.defineProperty(original,'stillLive',{value:yySettingsViewGuard()});if(!original.input){T('请输入服务器号、房间号或分享链接','e');return}
  pending=true;var run=++sequence,box=host();if(box)box.replaceChildren(el('p','正在查询租赁服、联机房间和本地房间…'));
  try {var result=await A('POST','/api/youyuan/rooms',Object.assign({force:!!force},original));if(run!==sequence||!same(original,fields()))return;if(!result.ok){if(box)box.replaceChildren(el('p',result.error||'房间发现失败'));return}if(result.selected){renderChoice(result.selected,original);await start(result.selected,original);return}render(result,original);if(result.complete&&(result.candidates||[]).length===1)await choose(result.candidates[0],original)}catch(error){if(run===sequence&&box)box.replaceChildren(el('p','房间查询失败，请重试。'))}finally{pending=false}
 }
 var previous=doConn;
 doConn=async function(){var raw=fields().input;
  // Keep supported explicit share/domain targets on the existing path.
  if(/^DomainGame:|^LobbyGame:room_id=|^https?:\/\/|mc\.163\.com\//.test(raw)||/^\d{16,20}$/.test(raw)||/^[0-9a-f]{11}(?:_[1-9]\d*)?$/i.test(raw))return previous();
  return discover(false)
 };
 var previousSettings=Rcfg;
 Rcfg=async function(){var live=typeof yySettingsViewGuard==='function'?yySettingsViewGuard():function(){return true};await previousSettings();if(!live())return;var input=document.getElementById('bot-server');if(!input)return;input.placeholder='服务器号、短房间号、房名或分享链接';input.addEventListener('input',clear);var hint=input.nextElementSibling;if(hint&&/#房间号=/.test(hint.textContent))hint.textContent='自动查询租赁服、联机房间和本地房间；有多个结果时选择一次。';var original=fields();Object.defineProperty(original,'stillLive',{value:live});if(!original.input||!original.token)return;var result=await A('POST','/api/youyuan/rooms',Object.assign({op:'current'},original));if(!same(original,fields())||!result.ok||!result.choice)return;input.value=result.choice.input;renderChoice(result.choice.candidate,fields())};
 window.yyRoomDiscovery={discover:discover,fields:fields,render:render};
})();
