import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
const file=name=>readFileSync(new URL('../upstream/frontend/js/'+name,import.meta.url),'utf8');
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}};
function navigation(source){
 const panels=['home','tasks','terminal','settings'].map(name=>({id:'tab-'+name,inert:false,attrs:{},offsetWidth:300,classList:{values:new Set(name==='home'?['show']:[]),add(...c){c.forEach(x=>this.values.add(x))},remove(...c){c.forEach(x=>this.values.delete(x))},contains(c){return this.values.has(c)}},setAttribute(k,v){this.attrs[k]=v},addEventListener(){}}));
 const timers=[],context=vm.createContext({curTab:'home',TAB_ORDER:['home','tasks','terminal','settings'],tabInited:{home:true,tasks:true,terminal:true,settings:true},activeRenderers:[],document:{getElementById:id=>panels.find(p=>p.id===id),querySelector:()=>null,querySelectorAll:q=>q==='.content'?panels:[]},updateNavGlide(){},Rplg(){},setTimeout:cb=>timers.push(cb),clearInterval(){}});
 vm.runInContext(source.slice(source.indexOf('function tabTo(t){'),source.indexOf('// Android back button')),context);
 return {panels,timers,context,visible:()=>panels.filter(p=>p.classList.contains('show')).map(p=>p.id)};
}
test('rapid four-tab switches never show two panels, including deferred work',()=>{
 const p=navigation(file('app.js'));
 for(let i=0;i<400;i++){const name=p.context.TAB_ORDER[i%4];p.context.tabTo(name);assert.deepEqual(p.visible(),['tab-'+name])}
 p.timers.forEach(run=>run());assert.deepEqual(p.visible(),['tab-settings']);assert.equal(p.panels.filter(p=>!p.inert).length,1);
});
test('old outgoing-animation implementation reproduces the overlap',()=>{
 const old=readFileSync(new URL('./fixtures/tab-navigation-0.10.50.js',import.meta.url),'utf8');
 // The staged public copy is overwritten when publishing; use this regression
 // only while it is the actual old build, not as a fabricated old implementation.
 assert.ok(old.includes('setTimeout(finish,320)'));
 const p=navigation(old);p.context.tabTo('tasks');p.context.tabTo('terminal');p.context.tabTo('settings');p.timers.forEach(run=>run());assert.ok(p.visible().length>1);
});
test('cached subpage restore cannot touch a newly opened page',()=>{
 const source=file('app.js'),calls=[],timers=[];
 const context=vm.createContext({yySubPageEpoch:1,subPageOpen:true,subPageStack:['old'],setTimeout:fn=>timers.push(fn),restoreSubPageState:key=>calls.push(key),runSubPageRestore:key=>calls.push(key)});
 vm.runInContext(source.slice(source.indexOf('function yyRestoreSubPageLater'),source.indexOf('// 子页恢复钩子')),context);
 context.yyRestoreSubPageLater('old',1);context.yySubPageEpoch=2;context.subPageStack=['new'];timers.splice(0).forEach(fn=>fn());assert.deepEqual(calls,[]);
 context.yyRestoreSubPageLater('new',2);timers.splice(0).forEach(fn=>fn());assert.deepEqual(calls,['new','new']);
});
test('concurrent same-account reads share one fetch; mutations and accounts stay independent',async()=>{
 let calls=0;const requests=[],context=vm.createContext({yyActiveAccount:'main',Map,AbortController,setTimeout,clearTimeout,fetch:()=>{calls++;const d=deferred();requests.push(d);return d.promise}});
 vm.runInContext(file('api.js'),context);
 const reads=Array.from({length:100},()=>context.A('GET','/api/plugin/list'));assert.equal(calls,1);
 context.yyActiveAccount='child';const child=context.A('GET','/api/plugin/list');assert.equal(calls,2);
 const writes=Array.from({length:8},()=>context.A('POST','/api/plugin/run',{id:'test'}));assert.equal(calls,10);
 requests.forEach((r,i)=>r.resolve({json:async()=>({ok:true,index:i})}));const values=await Promise.all([...reads,child,...writes]);assert.equal(values[100].index,1);assert.equal(context.yyReadRequests.size,0);
 const retry=context.A('GET','/api/plugin/list');assert.equal(calls,11);requests.at(-1).resolve({json:async()=>({ok:true})});await retry;
});
test('read sharing never crosses an authentication session',async()=>{
 let auth='first-session',calls=0;const requests=[];
 const context=vm.createContext({Map,JSON,AbortController,setTimeout,clearTimeout,sessionStorage:{getItem:()=>auth},fetch:()=>{calls++;const d=deferred();requests.push(d);return d.promise}});
 vm.runInContext(file('api.js'),context);const first=context.A('GET','/api/plugin/list');auth='second-session';const second=context.A('GET','/api/plugin/list');assert.equal(calls,2);
 requests.forEach((r,i)=>r.resolve({json:async()=>({ok:true,owner:i})}));assert.equal((await first).owner,0);assert.equal((await second).owner,1);
});
test('plugin page commits only the latest full snapshot and ignores an old account',async()=>{
 const requests=[],host={innerHTML:''},commits=[];
 Object.defineProperty(host,'innerHTML',{get(){return commits.at(-1)||''},set(v){commits.push(v)}});
 const context=vm.createContext({yyActiveAccount:'main',yyReadScope:()=>context.yyActiveAccount,Promise,yyHeading:()=>'',yyIcon:()=>'',yyBadge:()=>'',yyEscape:String,yyInstalledRuntimeHTML:()=>'',A:()=>{const d=deferred();requests.push(d);return d.promise},document:{getElementById:()=>host,createElement:()=>({set innerHTML(v){this.section={innerHTML:v,firstElementChild:{remove(){}}}},querySelector(){return this.section}})},yyPluginListHTML:r=>r.plugins[0].name});
 const source=file('youyuan_workspace.js');vm.runInContext(source.slice(0,source.indexOf('yyLanguagePage=function')),context);
 const first=context.Rplg(),second=context.Rplg();
 for(let i=3;i<6;i++)requests[i].resolve(i===3?{workspaces:[]} : i===4?{instances:[],installed:[]} : {plugins:[{name:'new snapshot'}]});await second;assert.equal(commits.length,1);
 for(let i=0;i<3;i++)requests[i].resolve(i===0?{workspaces:[]} : i===1?{instances:[],installed:[]} : {plugins:[{name:'stale snapshot'}]});await first;assert.equal(commits.length,1);assert.match(host.innerHTML,/new snapshot/);assert.doesNotMatch(host.innerHTML,/stale snapshot/);
 const changing=context.Rplg();context.yyActiveAccount='different';for(let i=6;i<9;i++)requests[i].resolve(i===6?{workspaces:[]} : i===7?{instances:[],installed:[]} : {plugins:[{name:'wrong account'}]});await changing;assert.equal(commits.length,1);
});
test('10,000 terminal lines schedule one render and preserve the bounded newest history',()=>{
 const frames=[],context=vm.createContext({allLogs:[],curTab:'terminal',requestAnimationFrame:fn=>{frames.push(fn);return frames.length},document:{getElementById:()=>null}});
 vm.runInContext(file('pages/terminal.js'),context);
 for(let i=0;i<10000;i++)context.addConsoleLine('line-'+i,'o');assert.equal(frames.length,1);assert.equal(context.allLogs.length,2000);assert.equal(context.allLogs[0].msg,'line-8000');assert.equal(context.allLogs.at(-1).msg,'line-9999');frames[0]();assert.equal(context.yyConsoleFrame,null);
});
test('late settings initialization does not write config or start hidden polling',async()=>{
 const fields=new Map(),reads=[],timers=[],cleared=[];
 const context=vm.createContext({curTab:'settings',subPageOpen:false,tabInited:{settings:true},activeRenderers:[],yySettingsRenderEpoch:0,yyReadScope:()=> 'same-session',yyHeading:()=>'',yyCardHead:()=>'',yySwitch:()=>'',yyIcon:()=>'',useBuiltinPicker:false,appTheme:'light',setPickerMode(){},yyQQStatusLabel:String,_cfgRefreshTimer:null,refreshCfgAccount(){},setInterval:fn=>{timers.push(fn);return timers.length},clearInterval:id=>cleared.push(id),A:async()=>({state:'connected'}),refreshCfg:()=>{const d=deferred();reads.push(d);return d.promise},document:{getElementById:id=>{if(!fields.has(id))fields.set(id,{value:'unchanged',checked:false,textContent:'',innerHTML:''});return fields.get(id)}}});
 const source=file('youyuan_settings.js');vm.runInContext(source.slice(source.indexOf('var yySettingsData'),source.indexOf('function yyQQStatusLabel')),context);
 const pending=context.Rcfg();context.curTab='home';reads[0].resolve({ok:true,config:{import_speed:777}});await pending;assert.equal(timers.length,0);assert.equal(fields.has('cfg-speed'),false);assert.equal(context.tabInited.settings,false);
 context.curTab='settings';context.tabInited.settings=true;const current=context.Rcfg();reads[1].resolve({ok:true,config:{import_speed:888}});await current;assert.equal(timers.length,1);assert.equal(fields.get('cfg-speed').value,888);context.activeRenderers.forEach(stop=>stop());assert.deepEqual(cleared,[1]);assert.equal(context._cfgRefreshTimer,null);
});
test('legacy config refresh checks view ownership before touching token and other fields',async()=>{
 const d=deferred(),writes=[];const context=vm.createContext({A:()=>d.promise,document:{getElementById:id=>{writes.push(id);return {value:'',checked:false}}}});
 const source=file('pages/settings.js');vm.runInContext(source.slice(source.indexOf('async function refreshCfg('),source.indexOf('async function refreshCfgStatus')),context);
 const pending=context.refreshCfg(()=>false);d.resolve({ok:true,config:{token:'different-account'}});await pending;assert.deepEqual(writes,[]);
});


test('settings extension ignores a late notice response after navigation',async()=>{
 const response=deferred(),writes=[];let nav=0;
 const context=vm.createContext({Rcfg:async()=>{},yySettingsViewGuard:()=>{const current=nav;return ()=>current===nav},yyCardHead:()=>'',yySwitch:()=>'',A:()=>response.promise,document:{getElementById:id=>id==='cfg-qq-card'?{insertAdjacentHTML(){}}:{set checked(v){writes.push(id)},set value(v){writes.push(id)}}}});
 const source=file('youyuan_social.js');vm.runInContext(source.slice(0,source.indexOf('async function yySaveNotices')),context);
 const render=context.Rcfg();await Promise.resolve();nav++;response.resolve({ok:true,config:{broadcast:true,tasks:true,creator:'old account'}});await render;assert.deepEqual(writes,[]);
});

test('settings extensions stop before inserting cards after an abandoned base render',async()=>{
 const waiting=deferred();let live=true;const touches=[];
 const context=vm.createContext({Rcfg:()=>waiting.promise,yySettingsViewGuard:()=>()=>live,document:{querySelector:s=>{touches.push(s);throw Error('stale extension touched DOM')},createElement:()=>{throw Error('stale card')}}});
 vm.runInContext(file('youyuan_connections.js').slice(file('youyuan_connections.js').indexOf('var yyConnectionsSettings')),context);
 const render=context.Rcfg();live=false;waiting.resolve();await render;assert.deepEqual(touches,[]);
});
