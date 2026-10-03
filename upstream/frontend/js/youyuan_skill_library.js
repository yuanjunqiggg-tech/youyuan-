async function yyOpenSkillLibrary(){
 const id=yyWorkspaceActive.id,r=await A('GET','/api/youyuan/agent/skills?id='+encodeURIComponent(id));
 if(!r.ok){T(r.error,'e');return} if(yyWorkspaceActive.id!==id)return;
 const host=yyAgentProfileModal('技能加载库 · '+yyLanguageNames[r.language]);
 host.innerHTML='<p class="yy-hint">自行选择加载／卸载，立即保存到当前工作区。加载是允许 AI 按需读取，内容不会一次性注入。新工作区默认开放当前语言的全部技能；显示“卸载”的表示 AI 可按需读取，正文尚未自动读入对话。其他工作区互不影响。</p><input id="yy-skill-search" placeholder="搜索技能名称或功能"><div id="yy-skill-cards"></div><p class="yy-hint">卸载会阻止后续读取；对话中已经读过的内容仍属于历史记录。需要完全干净的上下文时，新建一个工作区。</p>';
 const render=()=>{
  const search=host.querySelector('#yy-skill-search').value.toLowerCase(),cards=host.querySelector('#yy-skill-cards');cards.replaceChildren();
  for(const sk of r.catalog.filter(s=>(s.name+' '+s.id+' '+s.description).toLowerCase().includes(search))){
   const card=document.createElement('section');card.className='card';card.dataset.skillId=sk.id;
   const head=document.createElement('strong');head.textContent=sk.name;
   const desc=document.createElement('p');desc.className='yy-hint';desc.textContent=sk.description+' · '+sk.sections.length+' 个章节';
   const actions=document.createElement('div');actions.className='yy-actions';
   const toggle=document.createElement('button');toggle.className=r.skills.includes(sk.id)?'btn-s':'btn';toggle.textContent=r.skills.includes(sk.id)?'卸载':'加载';
   toggle.onclick=async()=>{toggle.disabled=true;try{const res=await A('POST','/api/youyuan/agent/skills',{id,skill:sk.id,enabled:!r.skills.includes(sk.id)});if(!res.ok)throw Error(res.error);r.skills=res.skills;render()}catch(e){toggle.disabled=false;T(e.message,'e')}};
   const preview=document.createElement('button');preview.className='btn-s';preview.textContent='查看章节';
   const details=document.createElement('div');details.hidden=true;
   preview.onclick=()=>{details.hidden=!details.hidden};
   const select=document.createElement('select');select.setAttribute('aria-label',sk.name+' 章节');
   sk.sections.forEach(s=>{const o=document.createElement('option');o.value=s;o.textContent=s;select.appendChild(o)});
   const pre=document.createElement('pre');pre.className='yy-readout yy-doc';pre.style.maxHeight='300px';pre.style.overflow='auto';
   let offset=0;const read=async(reset)=>{if(reset)offset=0;const res=await A('GET','/api/youyuan/skills/read?skill='+encodeURIComponent(sk.id)+'&section='+encodeURIComponent(select.value)+'&offset='+offset);if(!res.ok){T(res.error,'e');return}pre.textContent=res.content;offset+=3000;pre.scrollTop=0};
   select.onchange=()=>read(true);const start=document.createElement('button');start.className='btn-s';start.textContent='读取这一章';start.onclick=()=>read(true);
   const next=document.createElement('button');next.className='btn-s';next.textContent='下一页';next.onclick=()=>read(false);
   const download=document.createElement('button');download.className='btn-s';download.textContent='导出独立 ZIP';download.onclick=async()=>{try{const path='/api/youyuan/skills/export?skill='+encodeURIComponent(sk.id);if(window.android&&android.exportFile){const result=await A('POST',path,{});if(!result.ok)throw Error(result.error);android.exportFile(result.src_path,sk.id+'.zip');return}const res=await fetch(path);if(!res.ok||!res.headers.get('Content-Type')?.includes('application/zip'))throw Error('导出失败');const url=URL.createObjectURL(await res.blob()),a=document.createElement('a');a.href=url;a.download=sk.id+'.zip';a.click();setTimeout(()=>URL.revokeObjectURL(url),60000)}catch(e){T(e.message,'e')}};
   actions.append(toggle,preview,download);details.append(select,start,next,pre);card.append(head,desc,actions,details);cards.appendChild(card);
  }
 };
 host.querySelector('#yy-skill-search').oninput=render;render();
}
