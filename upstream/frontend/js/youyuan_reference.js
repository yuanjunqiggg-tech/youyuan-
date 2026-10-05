/* Offline documentation browser and task presentation. */
function yyTransferProgress(d,task){
  var isExport=task&&task.type==='export',el=document.getElementById(isExport?'exp-stat':'imp-stat');
  if(!el)return;
  var count=d.completed!=null?d.completed:d.blocks,total=d.total_blocks!=null?d.total_blocks:d.total;
  var parts=[d.phase||(isExport?'建筑读取':'建筑写入')];
  if(count!=null)parts.push(Number(count).toLocaleString()+(total!=null?' / '+Number(total).toLocaleString():'')+' '+(d.unit||(isExport?'区块已读取':'方块已提交')));
  if(d.location)parts.push(d.location);
  el.textContent=parts.join(' · ');
}
var yyReferenceCatalog=null,yyReferenceSkill='lua-runtime';
function yyReferenceMarkdown(content){return window.marked?marked.parse(content):'<pre>'+yyEscape(content)+'</pre>';}
yyOpenLanguageDoc=async function(skill){
  yyReferenceSkill=skill||'lua-runtime';
  var r=await A('GET','/api/youyuan/docs?language='+encodeURIComponent(yyReferenceSkill));
  delete subPageCache['插件开发文档'];
  openSubPage('插件开发文档', '<div class="yy-reference"><header class="yy-reference-head"><div><small>游园 · 开发手册</small><h2>'+yyEscape({ 'lua-runtime':'Lua 常驻插件','python-runtime':'Python 常驻插件','cpp-runtime':'C++ 常驻插件','packet-reference':'游戏协议与玩法'}[skill]||'功能开发说明')+'</h2></div><div class="yy-actions"><button class="btn-s" id="yy-doc-packets">协议包</button><button class="btn-s" id="yy-doc-download">下载完整手册</button></div></header><div class="yy-reference-layout"><nav id="yy-doc-nav"><input type="search" id="yy-doc-search" placeholder="搜索游戏功能、方法或包名" aria-label="搜索开发文档"><div id="yy-doc-list"></div></nav><article class="card yy-document" id="yy-doc-body">'+yyReferenceMarkdown(r.content||r.error||'')+'</article></div></div>');
  document.getElementById('yy-doc-packets').onclick=()=>yyOpenLanguageDoc('packet-reference');
  document.getElementById('yy-doc-body').onclick=async e=>{
    var a=e.target.closest('a');if(!a)return;var href=a.getAttribute('href');
    if(href&&/^[a-z-]+\.md$/.test(href)){e.preventDefault();var r=await A('GET','/api/youyuan/docs?language='+encodeURIComponent(yyReferenceSkill)+'&section='+encodeURIComponent(href));document.getElementById('yy-doc-body').innerHTML=yyReferenceMarkdown(r.content||r.error||'');}
  };
  document.getElementById('yy-doc-download').onclick=async()=>{
    var full=await A('GET','/api/youyuan/docs?language='+encodeURIComponent(yyReferenceSkill)+(skill==='packet-reference'?'&section=catalog.md':'&full=1'));
    if(!full.ok){T(full.error,'e');return}var url=URL.createObjectURL(new Blob([full.content],{type:'text/markdown;charset=utf-8'})),a=document.createElement('a');a.href=url;a.download='游园-'+yyReferenceSkill+'-开发手册.md';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
  };
  var catalog=await A('GET','/api/youyuan/docs?language='+encodeURIComponent(yyReferenceSkill)+'&catalog=1');
  if(!catalog.ok){document.getElementById('yy-doc-nav').hidden=true;return}
  yyReferenceCatalog=catalog.catalog;
  yyReferenceRenderList('');
  document.getElementById('yy-doc-search').oninput=e=>yyReferenceRenderList(e.target.value);
};
function yyReferenceRenderList(query){
  query=query.trim().toLowerCase();var list=document.getElementById('yy-doc-list');if(!list)return;list.replaceChildren();
  var packets=yyReferenceSkill==='packet-reference',cat=yyReferenceCatalog;
  var rows=packets?cat.packets:Object.entries(cat.namespaces).map(([name,v])=>({name,title:v.title,usage:v.description,count:v.count}));
  if(!packets&&query){rows=cat.methods.filter(v=>(v.method+' '+v.content).toLowerCase().includes(query)).map(v=>({name:v.method,title:v.signature||v.method,method:true}));}
  else rows=rows.filter(v=>(v.name+' '+v.title+' '+(v.usage||'')).toLowerCase().includes(query));
  rows.forEach(row=>{var b=document.createElement('button');b.className='yy-doc-topic';b.innerHTML='<strong>'+yyEscape(row.title||row.name)+'</strong><small>'+yyEscape(row.name)+(row.count?' · '+row.count+'个接口':'')+'</small>';b.onclick=async()=>{
    var path='/api/youyuan/docs?language='+encodeURIComponent(yyReferenceSkill)+(packets?'&packet='+encodeURIComponent(row.name):row.method?'&method='+encodeURIComponent(row.name):'&section='+encodeURIComponent(row.name+'.md'));
    var r=await A('GET',path),body=document.getElementById('yy-doc-body');if(!body)return;body.innerHTML=yyReferenceMarkdown(r.content||r.error||'');list.querySelectorAll('.active').forEach(el=>el.classList.remove('active'));b.classList.add('active');if(innerWidth<700)body.scrollIntoView({block:'start',behavior:'smooth'});
  };list.append(b)});
  if(!rows.length)list.textContent='没有匹配的功能或包名';
}
