import {test} from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const read=name=>readFileSync(new URL('../upstream/frontend/js/'+name,import.meta.url),'utf8');
const deferred=()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b});return {promise,resolve,reject}};
function setup(){
 let owner='owner/main';const requests=[],messages=[],fields=new Map(),attributes=new Map(),controlAttributes=new Map();
 const control={disabled:false,hasAttribute:k=>controlAttributes.has(k),setAttribute:(k,v)=>controlAttributes.set(k,v),getAttribute:k=>controlAttributes.get(k),removeAttribute:k=>controlAttributes.delete(k)};
 const status={hidden:true,textContent:''},badge={textContent:'运行中',dataset:{ypsLabel:'运行中'}};
 const row={setAttribute:(k,v)=>attributes.set(k,v),getAttribute:k=>k==='data-yy-plugin'?'fixture':attributes.get(k),querySelectorAll:()=>[control],querySelector:k=>k==='.yps-badge'?badge:status};
 fields.set('yy-plugin-operation-status',{hidden:true,textContent:''});fields.set('yps-last-result',{hidden:true,textContent:'',dataset:{}});
 const context=vm.createContext({Map,Set,Promise,Date,URL,window:{},yyReadScope:()=>owner,yyPluginRenderEpoch:0,curTab:'tasks',setTimeout:()=>0,clearTimeout(){},setInterval:()=>0,clearInterval(){},document:{getElementById:k=>fields.get(k)||null,querySelectorAll:()=>[row]},T:(...args)=>messages.push(args),A:(...args)=>{const d=deferred();requests.push({args,...d});return d.promise}});
 vm.runInContext(read('pages/plugins.js'),context);vm.runInContext(read('youyuan_plugin_studio.js'),context);
 let refreshes=0;context.Rplg=async()=>{refreshes++};
 return {context,requests,messages,fields,row,status,badge,control,owner:v=>owner=v,refreshes:()=>refreshes};
}
test('a delayed plugin operation locks the whole row immediately, blocks duplicate writes and retains the acknowledged result',async()=>{
 const p=setup(),running=p.context.plgSwitch('fixture',false);
 assert.equal(p.control.disabled,true);assert.equal(p.row.getAttribute('aria-busy'),'true');assert.equal(p.status.hidden,false);assert.match(p.status.textContent,/停止插件.*0 秒/);
 await p.context.plgSwitch('fixture',false);assert.equal(p.requests.length,1);
 p.requests[0].resolve({ok:true});await running;
 assert.equal(p.control.disabled,false);assert.equal(p.row.getAttribute('aria-busy'),'false');assert.match(p.status.textContent,/停止插件完成/);assert.equal(p.fields.get('yps-last-result').dataset.result,'success');
 assert.equal(p.badge.textContent,'正在核对状态','old running badge must not reappear before the new list is read');assert.equal(p.refreshes(),1);
});
test('a response failure and an unknown network outcome retain distinct result statuses',async()=>{
 const p=setup();let pending=p.context.plgReload('fixture');p.requests[0].resolve({ok:false,error:'配置无效'});await pending;
 assert.equal(p.fields.get('yps-last-result').dataset.result,'error');assert.match(p.status.textContent,/失败/);
 pending=p.context.plgReload('fixture');p.requests[1].resolve({ok:false,error:'请求超时，执行结果尚未确认，请刷新状态后再决定是否重试'});await pending;
 assert.equal(p.fields.get('yps-last-result').dataset.result,'unknown');assert.match(p.status.textContent,/结果未确认.*勿重复提交/);
 pending=p.context.plgReload('fixture');p.requests[2].reject(Error('socket closed'));await pending;
 assert.equal(p.fields.get('yps-last-result').dataset.result,'unknown');assert.equal(p.context.yyPluginOperations.size,0);
});
test('late completion is saved only for its original owner and cannot repaint another account',async()=>{
 const p=setup(),pending=p.context.plgReload('fixture');p.owner('owner/second');p.requests[0].resolve({ok:true});await pending;
 assert.equal(p.fields.get('yps-last-result').hidden,true);assert.equal(p.refreshes(),0);assert.equal(p.messages.some(m=>m[1]==='o'),false);
 const result=Array.from(p.context.yyPluginLastResults.values())[0];assert.equal(result.scope,'owner/main');
});
test('deletion confirmation from a previous account cannot submit into the current account',async()=>{
 const p=setup();let confirmation;p.context.escHtml=String;p.context.showConfirm=opts=>confirmation=opts;
 p.context.plgDel('fixture');assert.equal(p.requests.length,0);p.owner('owner/second');await confirmation.onConfirm();assert.equal(p.requests.length,0);assert.match(p.messages.at(-1)[0],/账号已切换/);
});
test('explicit not_sent or rejected receipts are failures even if their diagnostic mentions a timeout',async()=>{
 const p=setup();for(const status of ['not_sent','rejected']){const pending=p.context.plgReload('fixture');p.requests.at(-1).resolve({ok:false,status,error:'请求超时，明确未发送'});await pending;assert.equal(p.fields.get('yps-last-result').dataset.result,'error')}
});
test('relay error_code and outcome fields preserve the difference between an unsent write and an unconfirmed submitted write',async()=>{
 const p=setup();let pending=p.context.plgReload('fixture');p.requests.at(-1).resolve({ok:false,error_code:'not_sent',submitted:false,confirmed:false,outcome:'not_sent',error:'请求超时'});await pending;assert.equal(p.fields.get('yps-last-result').dataset.result,'error');
 pending=p.context.plgReload('fixture');p.requests.at(-1).resolve({ok:false,error_code:'execution_result_unknown',submitted:true,confirmed:false,outcome:'write_result_unknown',error:'中继连接关闭'});await pending;assert.equal(p.fields.get('yps-last-result').dataset.result,'unknown');
});
test('installed runtime deletion also rejects confirmation after an account change',async()=>{
 const p=setup();let confirmation;p.context.yyEscape=String;p.context.showConfirm=opts=>confirmation=opts;
 const workspace=read('youyuan_workspace.js');const start=workspace.indexOf('function yyDeleteInstalledRuntime('),end=workspace.indexOf('\n',start);
 vm.runInContext(workspace.slice(start,end<0?undefined:end),p.context);
 p.context.yyDeleteInstalledRuntime('fixture');assert.equal(p.requests.length,0);p.owner('owner/second');await confirmation.onConfirm();assert.equal(p.requests.length,0);
});
test('session result storage is bounded and a repeated plugin replaces its previous result',()=>{
 const p=setup();for(let n=0;n<40;n++)p.context.yyPluginRecordResult({scope:'owner/main',id:'p'+n,label:'停止插件'},{ok:true},false);
 assert.equal(p.context.yyPluginLastResults.size,32);assert.equal(Array.from(p.context.yyPluginLastResults.values())[0].id,'p8');
 p.context.yyPluginRecordResult({scope:'owner/main',id:'p8',label:'启动插件'},{ok:false,error:'拒绝'},false);
 assert.equal(p.context.yyPluginLastResults.size,32);assert.equal(Array.from(p.context.yyPluginLastResults.values()).at(-1).status,'error');
});
