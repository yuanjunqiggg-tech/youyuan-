import {test} from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const source=name=>readFileSync(new URL('../upstream/frontend/js/'+name,import.meta.url),'utf8');
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}};

test('post-mutation GET cannot reuse the list request started before the write',async()=>{
 const requests=[],context=vm.createContext({Map,AbortController,setTimeout,clearTimeout,fetch:(path,options)=>{const d=deferred();requests.push({path,options,...d});return d.promise}});
 vm.runInContext(source('api.js'),context);
 const before=context.A('GET','/api/plugin/list'),write=context.A('POST','/api/plugin/stop',{id:'p'});
 requests[1].resolve({json:async()=>({ok:true})});await write;
 const after=context.A('GET','/api/plugin/list');assert.equal(requests.length,3);
 requests[2].resolve({json:async()=>({ok:true,plugins:[{running:false}]})});assert.equal((await after).plugins[0].running,false);
 requests[0].resolve({json:async()=>({ok:true,plugins:[{running:true}]})});assert.equal((await before).plugins[0].running,true);assert.equal(context.yyReadRequests.size,0);
});
function plugins(){
 const requests=[],messages=[],refreshes=[],timers=[],status={hidden:true,textContent:''};let scope='owner/main';
 const context=vm.createContext({Map,Date,yyPluginRenderEpoch:0,curTab:'tasks',yyReadScope:()=>scope,A:(...args)=>{const d=deferred();requests.push({args,...d});return d.promise},T:(...m)=>messages.push(m),Rplg:async()=>refreshes.push(scope),setInterval:f=>{timers.push(f);return timers.length},clearInterval(){},document:{getElementById:()=>status,querySelectorAll:()=>[]}});
 vm.runInContext(source('pages/plugins.js'),context);context.Rplg=async()=>refreshes.push(scope);
 return {context,requests,messages,refreshes,timers,status,changeScope:value=>scope=value};
}
test('plugin lifecycle shows pending state, prevents duplicate writes and reports server failures',async()=>{
 const p=plugins(),pending=p.context.plgSwitch('p',true);assert.equal(p.requests.length,1);assert.equal(p.status.hidden,false);assert.match(p.status.textContent,/启动插件/);
 await p.context.plgSwitch('p',false);assert.equal(p.requests.length,1);
 p.requests[0].resolve({ok:false,error:'未连接'});const result=await pending;assert.equal(result.ok,false);assert.ok(p.messages.some(m=>m[0]==='未连接'&&m[1]==='e'));assert.equal(p.refreshes.length,1);assert.equal(p.status.hidden,true);assert.equal(p.context.yyPluginOperations.size,0);
});
test('late mutation result does not refresh or report success into another account',async()=>{
 const p=plugins(),pending=p.context.plgReload('p');p.changeScope('owner/second');p.requests[0].resolve({ok:true});await pending;assert.equal(p.refreshes.length,0);assert.equal(p.messages.filter(m=>m[1]==='o').length,0);assert.equal(p.context.yyPluginOperations.size,0);
});
test('deleting a plugin requires confirmation before it sends any mutation',async()=>{
 const p=plugins();let confirmation;p.context.showConfirm=opts=>confirmation=opts;p.context.escHtml=String;
 p.context.plgDel('large-plugin');assert.equal(p.requests.length,0);assert.match(confirmation.message,/备份/);const pending=confirmation.onConfirm();assert.match(p.status.textContent,/删除插件/);assert.equal(p.requests[0].args[1],'/api/plugin/delete');p.requests[0].resolve({ok:true});await pending;
});
function accounts(){
 const requests=[],frames=[],events={},timers=[],grid={innerHTML:''},monitor={innerHTML:'',value:''};let owner='first';
 const context=vm.createContext({Promise,JSON,yyPlatformUser:{id:'first'},sessionStorage:{getItem:()=>owner},yyActiveAccount:'main',yyAllAccountRows:[],yyAccountRows:[],yyMonitorID:'main',yyRootAPI:(...args)=>{const d=deferred();requests.push({args,...d});return d.promise},yyRefreshMonitor:async()=>{},yyRenderProjects(){},yyRenderAccountCards(){frames.push(context.yyAllAccountRows.map(a=>({...a})))},yyAccountName:()=>'',yyEscape:String,setTimeout:f=>timers.push(f),document:{addEventListener:(type,cb)=>events[type]=cb,getElementById:id=>id==='yy-accounts-grid'?grid:id==='yy-monitor-account'?monitor:null,querySelector:()=>null}});
 const script=source('youyuan_accounts.js');vm.runInContext(script.slice(script.indexOf('// Cache only'),script.indexOf('function yyEnterRow')),context);
 return {context,requests,frames,events,timers,grid,changeOwner(){owner='second';context.yyPlatformUser={id:'second'}}};
}
test('lightweight account rows appear before selected telemetry and never query all child runtimes',async()=>{
 const p=accounts(),monitor=deferred();p.context.yyRefreshMonitor=()=>monitor.promise;
 const pending=p.context.yyRefreshAccounts();assert.equal(p.requests.length,1);assert.equal(p.requests[0].args[1],'/api/youyuan/accounts?view=summary');
 p.requests[0].resolve({ok:true,main:{name:'main',connected:null,engine_ready:null,connection_known:false},accounts:[{id:'child',name:'child',connected:null,engine_ready:null,connection_known:false}]});await new Promise(r=>setImmediate(r));
 assert.equal(p.frames[0].length,2);assert.equal(p.frames[0][1].name,'child');assert.equal(p.context.yyAccountStatusLabel(p.frames[0][0]),'状态待查询');assert.equal(p.context.yyAccountStatusLabel(p.frames[0][1]),'状态待查询');assert.equal(p.requests.length,1);
 monitor.resolve();await pending;
});
test('startup preloads accounts and successful authentication invalidates cached identities',async()=>{
 const p=accounts();assert.equal(p.timers.length,1);p.timers[0]();assert.equal(p.requests.length,1);assert.equal(p.requests[0].args[1],'/api/youyuan/accounts?view=summary');p.requests[0].resolve({ok:true,accounts:[{id:'old',name:'old-owner'}]});await new Promise(r=>setImmediate(r));
 assert.equal(p.context.yyAccountSnapshot.result.accounts[0].id,'old');p.changeOwner();p.events['yy-identity-ready']();assert.equal(p.context.yyAccountSnapshot,null);assert.equal(p.context.yyAllAccountRows.length,0);assert.equal(p.requests.length,2);p.requests[1].resolve({ok:true,accounts:[{id:'new',name:'new-owner'}]});await new Promise(r=>setImmediate(r));assert.equal(p.context.yyAccountSnapshot.result.accounts[0].id,'new');
});
test('account metadata never leaks a previous owner main status into new owner rows',()=>{
 const p=accounts();p.context.yySetAccountRows({main:{name:'first'},accounts:[]});p.context.yyAllAccountRows[0].server='private-server';p.changeOwner();p.context.yySetAccountRows({main:{name:'second'},accounts:[]});assert.equal(p.context.yyAllAccountRows[0].server,undefined);
});


// A failed refresh must retain last known plugin state rather than replace it
// with an empty list; the error remains visible to the user.
test('failed plugin list reads retain the previous page instead of showing no plugins',async()=>{
 const messages=[],host={innerHTML:'existing-running-plugin'},context=vm.createContext({Map,yyReadScope:()=> 'main',yyPluginScopeBusy:()=>false,yyPluginRenderEpoch:0,T:(m)=>messages.push(m),A:async()=>({ok:false,error:'endpoint offline'}),document:{getElementById:()=>host}});
 const script=source('youyuan_workspace.js');vm.runInContext(script.slice(0,script.indexOf('yyLanguagePage=function')),context);await context.Rplg();assert.equal(host.innerHTML,'existing-running-plugin');assert.match(messages[0],/endpoint offline/);
});


test('summary unknown connection values are not rendered as disconnected',()=>{
 const p=accounts();assert.equal(p.context.yyAccountStatusLabel({connected:null,engine_ready:null,connection_known:false}),'状态待查询');assert.equal(p.context.yyAccountStatusLabel({connected:null,engine_ready:true}),'状态待查询');assert.equal(p.context.yyAccountStatusLabel({connected:false,engine_ready:true,connection_known:true}),'内核在线 · 游戏未连接');assert.equal(p.context.yyAccountStatusLabel({connected:true,engine_ready:true,connection_known:true}),'已连接');
});
function monitorSetup(){
 const p=accounts(),fields={};for(const id of ['yy-accounts-grid','yy-online-chart','yy-distribution-chart','yy-chat-table','yy-monitor-metrics','yy-online-source'])fields[id]={innerHTML:'',textContent:''};
 p.context.document.getElementById=id=>fields[id]||null;p.context.yyAccountPath=(path,id)=>'/api/youyuan/accounts/invoke/'+id+path;p.context.yyDrawOnline=()=>{};p.context.yyDrawChat=()=>{};p.context.yyBadge=String;
 p.context.yyMonitorID='child';p.context.yyAllAccountRows=[{id:'child',connected:null,engine_ready:null,connection_known:false},{id:'unselected',connected:null,connection_known:false}];
 const script=source('youyuan_accounts.js');vm.runInContext(script.slice(script.indexOf('var yyMonitorReadEpoch'),script.indexOf('function yyDrawChat')),p.context);return {...p,fields};
}
test('live telemetry queries only the selected account and changes unknown state only from a real status response',async()=>{
 const p=monitorSetup(),pending=p.context.yyRefreshMonitor();assert.equal(p.requests.length,2);assert.ok(p.requests.every(r=>r.args[1].includes('/invoke/child/')));
 p.requests[0].resolve({ok:true,connected:true,online:3,source:'real observations'});p.requests[1].resolve({ok:true,connected:true,server:'s'});await pending;
 assert.equal(p.context.yyAllAccountRows[0].connection_known,true);assert.equal(p.context.yyAllAccountRows[0].connected,true);assert.equal(p.context.yyAllAccountRows[1].connected,null);assert.equal(p.frames.at(-1)[0].online,3);
});
test('failed child status remains unknown rather than claiming the process is stopped',async()=>{
 const p=monitorSetup(),pending=p.context.yyRefreshMonitor();p.requests[0].resolve({ok:false,error:'服务503'});p.requests[1].resolve({ok:false,error:'服务503'});await pending;
 assert.equal(p.context.yyAllAccountRows[0].connected,null);assert.equal(p.fields['yy-monitor-metrics'].textContent,'状态待查询');assert.match(p.fields['yy-online-chart'].innerHTML,/服务503/);assert.doesNotMatch(p.fields['yy-online-chart'].innerHTML,/内核未启动/);
});

test('an unconfigured main robot is absent from the account directory and appears only after real configuration',()=>{
 const p=accounts();p.context.yySetAccountRows({main:{name:'主账号',configured:false},accounts:[{id:'child',name:'stored'}]});assert.equal(p.context.yyAllAccountRows.length,1);assert.equal(p.context.yyAllAccountRows[0].id,'child');assert.equal(p.context.yyAccountMainConfigured,false);
 p.context.yySetAccountRows({main:{name:'real robot',configured:true,game_id:'123'},accounts:[]});assert.equal(p.context.yyAllAccountRows.length,1);assert.equal(p.context.yyAllAccountRows[0].id,'main');assert.equal(p.context.yyAllAccountRows[0].game_id,'123');
});
