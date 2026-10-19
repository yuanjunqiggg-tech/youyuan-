// Optional bounded group memory. API calls stay within the selected account;
// no operation here sends QQ messages or invokes a model/embedding endpoint.
(function(){
 let view=null;
 function scope(){return typeof yyReadScope==='function'?yyReadScope():JSON.stringify([typeof yyActiveAccount==='undefined'?'main':yyActiveAccount,typeof yyAccountControllerScope==='function'?yyAccountControllerScope():''])}
 function live(v){return v&&view===v&&scope()===v.scope&&document.getElementById('yy-qq-memory-panel')===v.panel}
 function el(id){return document.getElementById('yy-qq-memory-'+id)}
 function enabledGroups(groups){return (groups||[]).filter(g=>g.enabled===true&&typeof g.id==='string'&&g.id!=='')}
 function selected(v){const id=el('group')?.value;return live(v)?v.groups.find(g=>g.id===id):null}
 function feedback(v,text){if(live(v))el('result').textContent=text}
 function setGroups(v,groups){
  if(!live(v))return;
  const select=el('group'),previous=select.value;v.groups=enabledGroups(groups);
  select.innerHTML=v.groups.map(g=>'<option value="'+yyEscape(g.id)+'">'+yyEscape(g.name||g.number||g.id)+'</option>').join('');
  select.value=v.groups.some(g=>g.id===previous)?previous:v.groups[0]?.id||'';
  select.disabled=!v.groups.length;el('clear').disabled=!v.groups.length;
 }
 function positive(value,fallback,min,max){const n=Number(value);return Number.isInteger(n)&&n>=min?Math.min(max,n):fallback}
 window.yyQQMemoryDraft=function(){
  if(!live(view))return undefined;
  const ttl=Number(el('ttl').value),budget=Number(el('budget').value);
  if(!Number.isInteger(ttl)||ttl<1||ttl>720)throw Error('记忆保留时长需为1～720小时的整数');
  if(!Number.isInteger(budget)||budget<1000||budget>16000)throw Error('上下文预算需为1000～16000字符的整数');
  return {enabled:el('enabled').checked,embedding_model:el('model').value,ttl_hours:ttl,context_chars:budget};
 };
 function statusText(r){
  if(!r.enabled)return '长期检索未启用；近期上下文仍受保留时长与预算限制。';
  if(!r.embedding_configured)return '词法回退：未配置可用的嵌入模型；当前不是向量检索。';
  if(String(r.status||'').startsWith('lexical_fallback_'))return '词法回退：嵌入服务或索引暂不可用。';
  if(r.status==='vector_with_lexical_fallback_for_unindexed')return '向量检索可用，未索引条目仍采用词法回退。';
  if(r.status==='vector_indexed'&&Number(r.indexed_entries)>0)return '已有真实向量索引；未索引条目仍可能词法回退。';
  return '已配置嵌入模型，尚未确认可用向量索引。';
 }
 window.yyQQMemoryRefreshStatus=async function(){
  const v=view,g=selected(v);if(!live(v))return;
  const epoch=++v.statusEpoch;
  if(!g){el('status').textContent='没有当前授权且已启用的群；请先在互通群设置中启用群。';el('group-id').textContent='';return}
  el('group-id').textContent='群标识：'+g.id;el('status').textContent='正在读取此群记忆状态…';
  try{
   const r=await A('GET','/api/qq/memory?group='+encodeURIComponent(g.id));
   if(!live(v)||epoch!==v.statusEpoch||selected(v)?.id!==g.id)return;
   if(!r.ok)throw Error(r.error||'读取失败');
   const count=n=>Number.isInteger(n)&&n>=0?String(n):'未知';
   el('status').textContent=statusText(r)+'\n保存条目：'+count(r.entries)+' · 已索引条目：'+count(r.indexed_entries)+'\n有效保留：'+count(r.ttl_hours)+' 小时 · 上下文预算：'+count(r.context_chars)+' 字符'+(r.persistence_error?'\n持久化异常：当前条目不保证已保存到磁盘。':'');
  }catch(e){if(live(v)&&epoch===v.statusEpoch&&selected(v)?.id===g.id)el('status').textContent='读取失败：'+e.message}
 };
 window.yyQQMemoryRefreshGroups=async function(){
  const v=view;if(!live(v)||v.refreshing)return;v.refreshing=true;el('refresh').disabled=true;
  try{const r=await A('GET','/api/qq/groups');if(!live(v))return;if(!r.ok)throw Error(r.error||'群列表读取失败');setGroups(v,r.groups);await yyQQMemoryRefreshStatus()}
  catch(e){feedback(v,'刷新失败：'+e.message)}finally{if(live(v))el('refresh').disabled=false;v.refreshing=false}
 };
 window.yyQQMemorySave=async function(){
  const v=view;if(!live(v)||v.saving)return;v.saving=true;el('save').disabled=true;
  try{const memory=yyQQMemoryDraft(),r=await A('POST','/api/qq/config',{assistant:{memory}});if(!live(v))return;if(!r.ok)throw Error(r.error||'保存失败');feedback(v,'记忆策略已保存；适用于当前账号所有已启用的互通群。');await yyQQMemoryRefreshStatus()}
  catch(e){feedback(v,'保存失败：'+e.message)}finally{if(live(v))el('save').disabled=false;v.saving=false}
 };
 window.yyQQMemoryClear=function(){
  const v=view,g=selected(v);if(!live(v)||!g||v.clearing)return;
  const ticket=++v.clearTicket,label=g.name||g.number||g.id;
  const perform=async()=>{
   if(!live(v)||v.clearing||ticket!==v.clearTicket)return;
   if(selected(v)?.id!==g.id){feedback(v,'所选群已切换，请重新确认清空。');return}
   v.clearTicket++;v.clearing=true;el('clear').disabled=true;
   try{
    // Recheck current enablement after confirmation; the backend enforces the
    // same group and device authority at the DELETE boundary.
    const groups=await A('GET','/api/qq/groups');
    if(!live(v)||selected(v)?.id!==g.id)return;
    if(!groups.ok)throw Error(groups.error||'群授权状态读取失败');
    if(!enabledGroups(groups.groups).some(row=>row.id===g.id)){setGroups(v,groups.groups);throw Error('该群已停用或不再可用，请重新选择')}
    const r=await A('DELETE','/api/qq/memory?group='+encodeURIComponent(g.id));
    if(!live(v)||selected(v)?.id!==g.id)return;
    if(!r.ok)throw Error(r.error||'清空失败');
    feedback(v,'已清空所选群的近期上下文、AI历史和持久检索索引；其他群保留。');await yyQQMemoryRefreshStatus();
   }catch(e){feedback(v,'清空未确认：'+e.message+'。请刷新状态确认，勿盲目重复提交。')}
   finally{v.clearing=false;if(live(v))el('clear').disabled=!selected(v)}
  };
  const message='将清空群「'+label+'」的近期上下文、AI历史和持久检索索引，其他群保留。清空不能撤回已经发送到第三方服务的数据。';
  if(typeof showConfirm==='function')showConfirm({title:'清空此群的AI记忆',message:yyEscape(message),confirmText:'清空本群记忆',onConfirm:perform});
  else if(typeof confirm==='function'&&confirm(message))return perform();
 };
 window.yyQQMemoryMount=async function(assistant,models,groups){
  const host=document.getElementById('yy-qq-assistant-panel');if(!host||document.getElementById('yy-qq-memory-panel'))return;
  const c=assistant?.memory||{},available=(models||[]).filter(m=>m.provider==='openai'),saved=c.embedding_model||'';
  let options='<option value="">不使用嵌入模型 · 词法检索</option>'+available.map(m=>'<option value="'+yyEscape(m.name)+'">'+yyEscape(m.name)+' · '+yyEscape(m.model_id||'')+'</option>').join('');
  if(saved&&!available.some(m=>m.name===saved))options+='<option value="'+yyEscape(saved)+'">原模型不可用：'+yyEscape(saved)+'</option>';
  host.insertAdjacentHTML('beforeend','<details class="yy-card" id="yy-qq-memory-panel" style="min-width:0"><summary>群AI记忆与检索 · 可选</summary><p class="yy-hint">只保留机器人实际收到的群事件，不读取QQ完整历史。身份、引用与各群上下文独立；长期检索默认关闭。</p>'+yySwitch('yy-qq-memory-enabled','启用长期检索记忆（RAG）',c.enabled===true)+'<label for="yy-qq-memory-model">现有嵌入模型 · 需要支持 embeddings 接口</label><select id="yy-qq-memory-model" style="width:100%;min-width:0;box-sizing:border-box">'+options+'</select><p class="yy-hint">复用AI模型管理中的OpenAI兼容配置，不复制密钥。未配置、服务失败或索引未就绪时采用词法回退，不表示已启用向量检索。选择嵌入模型后，相关群文本可能发送到该服务。</p><div class="yy-form-grid" style="grid-template-columns:repeat(auto-fit,minmax(min(180px,100%),1fr))"><label>保留时长 / 小时<input id="yy-qq-memory-ttl" type="number" min="1" max="720" step="1" value="'+positive(c.ttl_hours,24,1,720)+'" style="width:100%;min-width:0;box-sizing:border-box"></label><label>上下文预算 / 字符<input id="yy-qq-memory-budget" type="number" min="1000" max="16000" step="1" value="'+positive(c.context_chars,6000,1000,16000)+'" style="width:100%;min-width:0;box-sizing:border-box"></label></div><button class="btn-s" id="yy-qq-memory-save" onclick="yyQQMemorySave()">保存记忆策略</button><p class="yy-hint">策略适用于当前账号所有已启用的互通群；下方查看和清空仅作用所选群。</p><label for="yy-qq-memory-group">查看记忆的群</label><select id="yy-qq-memory-group" style="width:100%;min-width:0;box-sizing:border-box"></select><p class="yy-hint" id="yy-qq-memory-group-id" style="overflow-wrap:anywhere"></p><div class="yy-actions" style="flex-wrap:wrap"><button class="btn-s" id="yy-qq-memory-refresh" onclick="yyQQMemoryRefreshGroups()">刷新群与状态</button><button class="btn-s" id="yy-qq-memory-clear" onclick="yyQQMemoryClear()">清空所选群记忆</button></div><p class="yy-hint" id="yy-qq-memory-status" role="status" style="white-space:pre-line;overflow-wrap:anywhere"></p><p class="yy-hint" id="yy-qq-memory-result" role="status" style="overflow-wrap:anywhere"></p></details>');
  view={panel:document.getElementById('yy-qq-memory-panel'),scope:scope(),groups:[],statusEpoch:0,clearTicket:0};
  el('model').value=saved;setGroups(view,groups);el('group').onchange=yyQQMemoryRefreshStatus;await yyQQMemoryRefreshStatus();
 };
})();
