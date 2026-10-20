/* Offline documentation browser and task presentation. */
/* The transfer panel mirrors the in-game actionbar: same counters, same honesty
 * rule for TPS/RTT. A single grey text line could not answer "is it slow or is
 * it stuck", which is the question operators actually ask mid-import. */
var yyTransferMeter={key:'',at:0,blocks:0,rate:0};
function yyTransferClock(ms){
  ms=Math.max(0,Number(ms)||0);var total=Math.floor(ms/1000),h=Math.floor(total/3600),m=Math.floor(total%3600/60),s=total%60;
  return h>0?h+':'+String(m).padStart(2,'0')+':'+String(s).padStart(2,'0'):String(m).padStart(2,'0')+':'+String(s).padStart(2,'0');
}
function yyTransferCount(n){return n==null?'—':Number(n).toLocaleString('zh-CN');}
function yyTransferProgress(d,task){
  var isExport=task&&task.type==='export',el=document.getElementById(isExport?'exp-stat':'imp-stat');
  if(!el)return;
  var done=Number(d.blocks!=null?d.blocks:(d.completed!=null?d.completed:0));
  var total=Number(d.total_blocks!=null?d.total_blocks:(d.total!=null?d.total:0));
  var chunks=d.chunks!=null?Number(d.chunks):(d.chunk!=null?Number(d.chunk):null);
  var chunksTotal=d.chunks_total!=null?Number(d.chunks_total):(d.total!=null&&d.chunk!=null?Number(d.total):null);
  var now=Date.now(),key=(d.structure||'')+'|'+total;
  if(yyTransferMeter.key!==key||now-yyTransferMeter.at>6000){yyTransferMeter.key=key;yyTransferMeter.at=now;yyTransferMeter.blocks=done;yyTransferMeter.rate=0;}
  else if(now-yyTransferMeter.at>=600){
    var dt=(now-yyTransferMeter.at)/1000;
    yyTransferMeter.rate=Math.max(0,(done-yyTransferMeter.blocks)/dt);
    yyTransferMeter.at=now;yyTransferMeter.blocks=done;
  }
  var rate=yyTransferMeter.rate;
  var pct=0;
  if(total>0)pct=Math.min(100,Math.round(done*100/total));
  else if(chunksTotal>0&&chunks!=null)pct=Math.min(100,Math.round(chunks*100/chunksTotal));
  var remain=(rate>0&&total>done)?Math.round((total-done)/rate*1000):null;
  var accent='var(--phx-primary,#2ecc71)',dim='var(--phx-text-secondary,#8b929e)',ink='var(--phx-text,#e8eaed)';
  var cell=function(label,value,colour){
    return '<div style="min-width:0"><small style="display:block;font-size:10px;color:'+dim+';letter-spacing:.02em">'+label+'</small>'+
      '<b style="display:block;font-size:13px;font-weight:600;color:'+(colour||ink)+';overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+value+'</b></div>';
  };
  var phase=yyEscape(d.phase||(isExport?'建筑读取':'建筑写入'));
  var head=yyEscape(d.structure||'结构');
  var size=(d.size_x&&d.size_y&&d.size_z)?(' · '+d.size_x+'×'+d.size_y+'×'+d.size_z):'';
  var cursor=(d.chunk_x!=null&&d.chunk_z!=null)?('X'+d.chunk_x+' Z'+d.chunk_z):'';
  var tps=d.tps_known?'TPS '+Number(d.tps).toFixed(1):'TPS 不可用';
  var rtt=d.rtt_known?'RTT '+Number(d.rtt_ms)+'ms':'RTT —';
  var foot='⟳ '+(cursor?('正在写入 '+cursor+' · '):'')+tps+' · '+rtt;
  el.innerHTML=
    '<div style="margin-top:10px;border:1px solid rgba(255,255,255,.09);border-radius:12px;padding:12px;background:rgba(255,255,255,.03)">'+
      '<div style="display:flex;justify-content:space-between;align-items:baseline;gap:8px;margin-bottom:9px">'+
        '<strong style="font-size:12px;color:'+accent+'">'+phase+'</strong>'+
        '<small style="font-size:11px;color:'+dim+';overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+head+size+'</small>'+
      '</div>'+
      '<div style="height:6px;border-radius:3px;background:rgba(255,255,255,.08);overflow:hidden;margin-bottom:11px">'+
        '<i style="display:block;height:100%;width:'+pct+'%;background:'+accent+';transition:width .25s ease"></i>'+
      '</div>'+
      '<div style="display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px 8px">'+
        cell('进度',pct+'%',accent)+
        cell('区块',(chunks==null?'—':chunks)+'/'+(chunksTotal==null?'—':chunksTotal))+
        cell('方块',yyTransferCount(done)+'/'+yyTransferCount(total))+
        cell('速率',rate>0?(Math.round(rate)+' 块/s'):'未知')+
        cell('已用',d.elapsed_ms!=null?yyTransferClock(d.elapsed_ms):'—')+
        cell('剩余',remain!=null?yyTransferClock(remain):'—')+
      '</div>'+
      '<div style="margin-top:10px;padding-top:8px;border-top:1px solid rgba(255,255,255,.07);font-size:11px;color:'+dim+'">'+yyEscape(foot)+'</div>'+
    '</div>';
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
