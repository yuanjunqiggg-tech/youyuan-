import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const read=name=>readFileSync(new URL('../upstream/frontend/js/'+name,import.meta.url),'utf8');
const studio=read('youyuan_studio.js');
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}};
function setup(){
 const storage=new Map(),fields=new Map(),marketCalls=[],opened=[],messages=[];let owner='owner-one',market='https://market-one.example',marketHandler=null;
 const el=tag=>({tagName:tag.toUpperCase(),children:[],textContent:'',disabled:false,innerHTML:'',style:{},dataset:{},append(...nodes){this.children.push(...nodes)},replaceChildren(...nodes){this.children=[...nodes]}});
 for(const id of ['yy-plugin-update-result','yy-plugin-check'])fields.set(id,el('div'));
 const context=vm.createContext({JSON,Number,String,Date,Promise,Array,Map,yyActiveAccount:'main',yyPlatformUser:{id:owner},yyReadScope:()=>JSON.stringify(['main',owner,'fixture-session-marker']),localStorage:{getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,value)},T:(...m)=>messages.push(m),RimpHTML:()=>'',RexpHTML:()=>'',Rplg:async()=>{},yyQQPage:async()=>{},pick:async()=>'',showFilePicker(){},A:async()=>({ok:true,market_origin:market,plugins:[{id:'local',name:'Local',version:'1'},{id:'second',name:'Second',version:'1'}]}),APluginMarket:async(...args)=>{marketCalls.push({origin:market,args});if(marketHandler)return marketHandler(...args);return {ok:true,versions:[{version:'2'}]}},pmOpenDetail:id=>opened.push({id,origin:market}),document:{getElementById:id=>fields.get(id)||null,querySelector:()=>null,createElement:el}});
 context.window=context;vm.runInContext(studio,context);
 return {context,storage,fields,marketCalls,opened,messages,owner(value){owner=value;context.yyPlatformUser={id:value}},market(value){market=value},onMarket(handler){marketHandler=handler}};
}
test('update preferences never persist the authenticated session material in localStorage keys or values',()=>{
 const p=setup();p.context.yyPluginRecordSource('local','123','https://market-one.example');
 assert.equal(p.storage.size,1);assert.ok([...p.storage].every(([key,value])=>!key.includes('fixture-session-marker')&&!value.includes('fixture-session-marker')),'session material must remain out of persistent preferences');
});
test('an already changed market origin blocks external version queries and never installs anything',async()=>{
 const p=setup();p.context.yyPluginRecordSource('local','123','https://market-old.example');await p.context.yyCheckPluginUpdates();assert.equal(p.marketCalls.length,0);assert.match(p.fields.get('yy-plugin-update-result').children[0].textContent,/来源已变更/);
});
test('a market change during sequential checks cannot query the next old source at a new origin',async()=>{
 const p=setup();p.context.yyPluginRecordSource('local','123','https://market-one.example');p.context.yyPluginRecordSource('second','456','https://market-one.example');
 p.onMarket(async()=>{p.market('https://market-two.example');return {ok:true,versions:[{version:'2'}]}});
 await p.context.yyCheckPluginUpdates();assert.ok(p.marketCalls.every(c=>c.origin==='https://market-one.example'),'old associations must not be queried at a changed origin');
});
test('a rendered source link revalidates its origin before opening the current market detail',async()=>{
 const p=setup();p.context.yyPluginRecordSource('local','123','https://market-one.example');await p.context.yyCheckPluginUpdates();const line=p.fields.get('yy-plugin-update-result').children[0],button=line.children.find(x=>x.tagName==='BUTTON');assert.ok(button);
 p.market('https://market-two.example');await button.onclick();assert.equal(p.opened.length,0,'old market IDs must not open a different provider');
});
test('an install completing after account change cannot record its source or navigate the new account',async()=>{
 const p=setup(),pending=deferred(),navigations=[];p.context.A=()=>pending.promise;p.context.Rplg=()=>navigations.push('refresh');p.context.yyNavigate=tab=>navigations.push(tab);
 const market=read('features/plugin_market.js');vm.runInContext(market.slice(market.indexOf('async function pmDoInstall'),market.indexOf('// ── 评论')),p.context);
 const running=p.context.pmDoInstall('123');p.owner('owner-two');pending.resolve({ok:true,id:'local',source_id:'123',source_origin:'https://market-one.example'});await running;
 assert.equal(p.storage.size,0,'late source must not be written into the next account');assert.deepEqual(navigations,[]);
});
function pickerSetup(){
 const fields=new Map(),requests=[];let scope='owner/main';
 const field=id=>{if(!fields.has(id))fields.set(id,{innerHTML:'',textContent:'',value:'',disabled:false,style:{},events:{},addEventListener(type,fn){this.events[type]=fn},querySelectorAll(){return []},focus(){}});return fields.get(id)};
 const context=vm.createContext({Promise,Date,Number,String,JSON,setTimeout,clearTimeout,yyReadScope:()=>scope,ICONS:new Proxy({},{get:()=>''}),formatSize:String,fetch:path=>{const d=deferred();requests.push({path,...d});return d.promise},document:{getElementById:field,createElement:()=>({className:'',innerHTML:'',addEventListener(){},remove(){},querySelectorAll:()=>[]}),body:{appendChild(){}}}});
 vm.runInContext(read('features/filepicker.js'),context);return {context,fields,field,requests,changeScope(){scope='owner/child'}};
}
test('file picker ignores a directory response belonging to a previous account scope',async()=>{
 const p=pickerSetup();p.context.showFilePicker('','building',()=>{});p.changeScope();p.requests[0].resolve({json:async()=>({ok:true,path:'/old-account',entries:[{type:'file',name:'old-account.mcstructure',path:'/old-account/file.mcstructure',size:1}]})});await new Promise(r=>setImmediate(r));
 assert.doesNotMatch(p.field('fp-body').innerHTML,/old-account\.mcstructure/);
});
test('file picker cancel settles the selection promise and ignores pending directory work',async()=>{
 const p=pickerSetup();let cancelled=0;p.context.showFilePicker('','building',()=>{},null,()=>cancelled++);p.field('fp-cancel').events.click();p.requests[0].resolve({json:async()=>({ok:true,path:'/old',entries:[]})});await new Promise(r=>setImmediate(r));assert.equal(cancelled,1);assert.doesNotMatch(p.field('fp-body').innerHTML,/暂无建筑文件/);
});
test('a stale file picker cannot begin an upload into a newly selected account',async()=>{
 const p=pickerSetup();let uploads=0;p.context.yyStudioUploadFile=async()=>{uploads++;return '/new-account/fixture.mcstructure'};
 p.context.showFilePicker('','building',()=>{});p.changeScope();await p.field('fp-upload-local').events.click();assert.equal(uploads,0);
});
test('file picker ignores an upload path returned after the account scope changed',async()=>{
 const p=pickerSetup(),upload=deferred();p.context.yyStudioUploadFile=()=>upload.promise;p.context.showFilePicker('','building',()=>{});
 const pending=p.field('fp-upload-local').events.click();p.changeScope();upload.resolve('/old-account/old-scope-file.mcstructure');await pending;assert.doesNotMatch(p.field('fp-selected-label').textContent,/old-scope-file/);
});
