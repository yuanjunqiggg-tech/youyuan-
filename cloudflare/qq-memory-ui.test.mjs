import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const read=name=>readFileSync(new URL('../upstream/frontend/js/'+name,import.meta.url),'utf8');
const source=read('youyuan_qq_memory.js');
const escape=x=>String(x??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}};
function setup(){
 const fields=new Map(),calls=[],confirmations=[];let owner='owner/main',handler=null;
 const field=id=>{if(!fields.has(id))fields.set(id,{id,value:'',checked:false,disabled:false,textContent:'',innerHTML:'',dataset:{},insertAdjacentHTML(where,html){this.innerHTML+=html;for(const m of html.matchAll(/id="([^"]+)"/g))field(m[1]);for(const m of html.matchAll(/<input[^>]+id="([^"]+)"[^>]+value="([^"]+)"/g))field(m[1]).value=m[2]}});return fields.get(id)};
 field('yy-qq-assistant-panel');
 const groups=[{id:'17923987654321098765',name:'<群一>',enabled:true},{id:'official/group?two&v=1',name:'群二',enabled:true},{id:'disabled',name:'已停用',enabled:false}];
 const status={ok:true,enabled:false,embedding_configured:false,status:'disabled',entries:0,indexed_entries:0,ttl_hours:24,context_chars:6000};
 const context=vm.createContext({Promise,JSON,String,Number,Array,yyReadScope:()=>owner,yyEscape:escape,yySwitch:(id,label,on)=>{field(id).checked=on;return '<input id="'+id+'" type="checkbox">'+label},showConfirm:opts=>confirmations.push(opts),document:{getElementById:id=>fields.get(id)||null},yyQQPage:async()=>{},A:async(method,path,body)=>{calls.push({method,path,body});if(handler)return handler(method,path,body);return path==='/api/qq/groups'?{ok:true,groups}:method==='POST'?{ok:true}:method==='DELETE'?{ok:true,inflight_answers_invalidated:true}:{...status}}});
 context.window=context;vm.runInContext(source,context);
 return {context,fields,field,calls,confirmations,groups,status,onAPI(fn){handler=fn},changeScope(){owner='owner/child'}};
}
test('memory UI is collapsed and defaults RAG off,24h,6000 chars without copying model credentials',async()=>{
 const p=setup();await p.context.yyQQMemoryMount({},[{name:'Embedding <one>',provider:'openai',model_id:'embed-v1',api_key:'fixture-model-secret'},{name:'Anthropic',provider:'anthropic',model_id:'chat'}],p.groups);
 const html=p.field('yy-qq-assistant-panel').innerHTML;assert.match(html,/<details[^>]+id="yy-qq-memory-panel"/);assert.doesNotMatch(html,/<details[^>]+\sopen(?:\s|>)/);assert.match(html,/Embedding &lt;one&gt;/);assert.doesNotMatch(html,/fixture-model-secret|>Anthropic/);
 assert.equal(p.field('yy-qq-memory-enabled').checked,false);assert.equal(p.field('yy-qq-memory-ttl').value,'24');assert.equal(p.field('yy-qq-memory-budget').value,'6000');
 assert.deepEqual(JSON.parse(JSON.stringify(p.context.yyQQMemoryDraft())),{enabled:false,embedding_model:'',ttl_hours:24,context_chars:6000});
 assert.match(html,/minmax\(min\(180px,100%\)/);
});
test('status and clearing preserve exact long group IDs and only allow enabled configured groups',async()=>{
 const p=setup();await p.context.yyQQMemoryMount({},[],p.groups);
 const html=p.field('yy-qq-memory-group').innerHTML;assert.match(html,/17923987654321098765/);assert.doesNotMatch(html,/disabled/);assert.match(html,/&lt;群一&gt;/);
 assert.equal(p.calls[0].path,'/api/qq/memory?group=17923987654321098765');
 p.field('yy-qq-memory-group').value='disabled';const before=p.calls.length;await p.context.yyQQMemoryRefreshStatus();p.context.yyQQMemoryClear();assert.equal(p.calls.length,before);assert.equal(p.confirmations.length,0);
});
test('enabled RAG without usable embedding clearly reports lexical fallback and persistence failure',async()=>{
 const p=setup();Object.assign(p.status,{enabled:true,embedding_configured:false,status:'lexical_fallback_embedding_not_configured',persistence_error:'fixture_disk_failure'});await p.context.yyQQMemoryMount({memory:{enabled:true}},[],p.groups);
 assert.match(p.field('yy-qq-memory-status').textContent,/词法回退/);assert.match(p.field('yy-qq-memory-status').textContent,/不是向量检索/);assert.match(p.field('yy-qq-memory-status').textContent,/不保证已保存到磁盘/);
 Object.assign(p.status,{embedding_configured:true,status:'',indexed_entries:0});await p.context.yyQQMemoryRefreshStatus();assert.match(p.field('yy-qq-memory-status').textContent,/尚未确认可用向量索引/);
});
test('no enabled group does not request default-group memory or expose an active clear action',async()=>{
 const p=setup();await p.context.yyQQMemoryMount({},[],[{id:'off',enabled:false}]);assert.equal(p.calls.length,0);assert.equal(p.field('yy-qq-memory-clear').disabled,true);assert.match(p.field('yy-qq-memory-status').textContent,/没有当前授权且已启用/);
});
test('save sends only bounded memory settings through the existing device API and validates before mutation',async()=>{
 const p=setup();await p.context.yyQQMemoryMount({},[],p.groups);p.field('yy-qq-memory-ttl').value='721';await p.context.yyQQMemorySave();assert.equal(p.calls.filter(c=>c.method==='POST').length,0);assert.match(p.field('yy-qq-memory-result').textContent,/1～720/);
 p.field('yy-qq-memory-ttl').value='48';p.field('yy-qq-memory-budget').value='8000';p.field('yy-qq-memory-enabled').checked=true;await p.context.yyQQMemorySave();const write=p.calls.find(c=>c.method==='POST');assert.equal(write.path,'/api/qq/config');assert.deepEqual(JSON.parse(JSON.stringify(write.body)),{assistant:{memory:{enabled:true,embedding_model:'',ttl_hours:48,context_chars:8000}}});
 assert.ok(p.calls.every(c=>!/(?:\/send|embeddings|\/api\/ai\/chat)/.test(c.path)));
});
test('clearing requires confirmation and rechecks current enablement, then deletes only the selected encoded group',async()=>{
 const p=setup();await p.context.yyQQMemoryMount({},[],p.groups);p.field('yy-qq-memory-group').value=p.groups[1].id;p.context.yyQQMemoryClear();assert.equal(p.calls.filter(c=>c.method==='DELETE').length,0);assert.equal(p.confirmations.length,1);
 await p.confirmations[0].onConfirm();const writes=p.calls.filter(c=>c.method==='DELETE');assert.equal(writes.length,1);assert.equal(writes[0].path,'/api/qq/memory?group='+encodeURIComponent(p.groups[1].id));assert.equal(writes[0].body,undefined);assert.match(p.field('yy-qq-memory-result').textContent,/其他群保留/);
});
test('a group disabled after confirmation cannot be cleared and a changed selection invalidates the confirmation',async()=>{
 const p=setup();await p.context.yyQQMemoryMount({},[],p.groups);p.context.yyQQMemoryClear();p.onAPI(async()=>({ok:true,groups:[]}));await p.confirmations[0].onConfirm();assert.equal(p.calls.filter(c=>c.method==='DELETE').length,0);assert.match(p.field('yy-qq-memory-result').textContent,/该群已停用/);
 const q=setup();await q.context.yyQQMemoryMount({},[],q.groups);q.context.yyQQMemoryClear();q.field('yy-qq-memory-group').value=q.groups[1].id;await q.confirmations[0].onConfirm();assert.equal(q.calls.filter(c=>c.method==='DELETE').length,0);assert.match(q.field('yy-qq-memory-result').textContent,/已切换/);
});
test('old group and account status responses never repaint the new selected scope',async()=>{
 const p=setup();await p.context.yyQQMemoryMount({},[],p.groups);const pending=[];p.onAPI(()=>{const d=deferred();pending.push(d);return d.promise});
 const old=p.context.yyQQMemoryRefreshStatus();p.field('yy-qq-memory-group').value=p.groups[1].id;const current=p.context.yyQQMemoryRefreshStatus();pending[1].resolve({...p.status,entries:2});await current;const expected=p.field('yy-qq-memory-status').textContent;pending[0].resolve({...p.status,entries:99});await old;assert.equal(p.field('yy-qq-memory-status').textContent,expected);
 const switched=p.context.yyQQMemoryRefreshStatus();p.changeScope();pending[2].resolve({...p.status,entries:88});await switched;assert.doesNotMatch(p.field('yy-qq-memory-status').textContent,/88/);assert.equal(p.context.yyQQMemoryDraft(),undefined);
});
test('general assistant save includes the mounted memory draft instead of silently discarding edited fields',async()=>{
 const p=setup();await p.context.yyQQMemoryMount({},[],p.groups);p.field('yy-qq-memory-ttl').value='72';
 for(const id of ['helper-enabled','companion','companion-all','companion-owner','image-model','companion-words','super','admins','death','join','leave','public-scores','ai-model','ai-prompt','mcp-url','mcp-token'])p.field('yy-qq-'+id);
 vm.runInContext(read('youyuan_qq_assistant.js'),p.context);const draft=p.context.yyQQAssistantDraft();assert.ok(draft.memory,'the common save path must include the visible memory controls');assert.equal(draft.memory.ttl_hours,72);
});
