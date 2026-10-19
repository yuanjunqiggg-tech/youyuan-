async function yyOpenSkillLibrary(){
 const id=yyWorkspaceActive.id,r=await A('GET','/api/youyuan/agent/skills?id='+encodeURIComponent(id));
 if(!r.ok){T(r.error,'e');return} if(yyWorkspaceActive.id!==id)return;
 const host=yyAgentProfileModal('技能加载库 · '+yyLanguageNames[r.language]);
 host.innerHTML='<p class="yy-hint">勾选多份技能，点击“保存所选技能”后统一生效。加载是允许 AI 按需读取，内容不会一次性注入。新工作区默认开放技能，勾选表示 AI 可按需读取，正文尚未自动读入对话。其他工作区互不影响。</p><div class="yy-actions"><button class="btn" id="yy-skill-save">保存所选技能</button><button class="btn-s" id="yy-skill-all">全选</button><button class="btn-s" id="yy-skill-none">清空选择</button><span id="yy-skill-selection"></span></div><input id="yy-skill-search" placeholder="搜索技能名称或功能"><div id="yy-skill-cards"></div><p class="yy-hint">取消选择并保存会阻止后续读取；对话中已经读过的内容仍属于历史记录。需要完全干净的上下文时，新建一个工作区。</p>';
 const selected=new Set(r.skills);
 const status=()=>{host.querySelector('#yy-skill-selection').textContent='已选 '+selected.size+' 份 · 尚未保存';};
 const render=()=>{
  const search=host.querySelector('#yy-skill-search').value.toLowerCase(),cards=host.querySelector('#yy-skill-cards');cards.replaceChildren();
  for(const sk of r.catalog.filter(s=>(s.name+' '+s.id+' '+s.description).toLowerCase().includes(search))){
   const card=document.createElement('section');card.className='card yy-multi-choice-card';card.dataset.skillId=sk.id;card.classList.toggle('is-selected',selected.has(sk.id));
   const head=document.createElement('strong');head.textContent=sk.name;head.onclick=()=>{checkbox.checked=!checkbox.checked;checkbox.onchange()};
   const desc=document.createElement('p');desc.className='yy-hint';desc.textContent=sk.description+' · '+sk.sections.length+' 个章节';
   const actions=document.createElement('div');actions.className='yy-actions';
   const toggle=document.createElement('label');toggle.className='yy-skill-select';
   const checkbox=document.createElement('input');checkbox.type='checkbox';checkbox.checked=selected.has(sk.id);checkbox.setAttribute('aria-label','选择技能 '+sk.name);
   checkbox.onchange=()=>{checkbox.checked?selected.add(sk.id):selected.delete(sk.id);card.classList.toggle('is-selected',checkbox.checked);status()};toggle.append(checkbox,document.createTextNode('选择这份技能 · 可多选'));
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
 host.querySelector('#yy-skill-search').oninput=render;
 host.querySelector('#yy-skill-all').onclick=()=>{r.catalog.forEach(sk=>selected.add(sk.id));render();status()};
 host.querySelector('#yy-skill-none').onclick=()=>{selected.clear();render();status()};
 host.querySelector('#yy-skill-save').onclick=async()=>{const button=host.querySelector('#yy-skill-save');button.disabled=true;try{const res=await A('POST','/api/youyuan/agent/skills',{id,skills:Array.from(selected)});if(!res.ok)throw Error(res.error);r.skills=res.skills;host.querySelector('#yy-skill-selection').textContent='已保存 '+r.skills.length+' 份技能';T('技能选择已统一保存','o')}catch(e){T(e.message,'e')}finally{button.disabled=false}};
 render();host.querySelector('#yy-skill-selection').textContent='当前已保存 '+selected.size+' 份技能';
}
