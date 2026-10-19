import {test} from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../upstream/frontend/js/youyuan_plugin_studio.js',import.meta.url),'utf8');
function setup(overrides={}){
 const context=vm.createContext({window:{},Map,Set,URL,Promise,yyPluginOperations:new Map(),yyPluginOperationUI(){},yyReadScope:()=> 'owner/main',yyPluginRenderEpoch:0,yyPluginScopeBusy:()=>false,curTab:'tasks',document:{getElementById:()=>null,querySelectorAll:()=>[]},...overrides});
 vm.runInContext(source,context);return context;
}
test('all installed languages share an inventory and bound projects do not duplicate installed plugins',()=>{
 const model=setup().window.yyPluginStudioModel;
 const data=[{ok:true,workspaces:[{id:'lua-work',plugin_id:'lua-id',language:'lua'},{id:'py-id',language:'python'},{id:'clock',language:'mcfunction'},{id:'draft',language:'native_cpp',name:'待发布'}]},
 {ok:true,installed:[{id:'py-id',language:'python'},{id:'cpp-id',language:'native_cpp'}],instances:[{id:'py-id',language:'python',account:'secondary',state:'running'},{id:'orphan',language:'python',account:'main'}]},
 {ok:true,plugins:[{id:'lua-id',type:'lua',name:'Lua脚本',running:true},{id:'words',type:'wordbank',disabled:true}]},
 {ok:true,files:[{config:{name:'clock.txt',enabled:true},commands:3,state:'等待连接'}]}];
 const rows=model.collect(data,{sources:{'lua-id':{id:'123'}}});
 assert.equal(rows.length,6);assert.equal(rows.find(p=>p.id==='lua-id').spaces.length,1);assert.equal(rows.find(p=>p.id==='py-id').spaces.length,1);assert.equal(rows.some(p=>p.id==='clock.txt'||p.id==='clock'),false);
 assert.equal(rows.find(p=>p.id==='words').state,'disabled');assert.equal(rows.find(p=>p.id==='lua-id').source,'linked');assert.equal(rows.find(p=>p.id==='orphan').state,'running');assert.equal(rows.filter(p=>p.kind==='workspace').length,1);
 const base={query:'secondary',language:'all',state:'all',source:'all'};assert.deepEqual(Array.from(model.filtered(rows,base),p=>p.id),['py-id']);
 assert.equal(model.filtered(rows,{...base,query:'',language:'mcfunction'}).length,0);assert.equal(model.filtered(rows,{...base,query:'',source:'workspace'}).length,1);
 for(const group of ['state','source','language'])assert.equal(model.groups(rows,group).reduce((n,g)=>n+g.items.length,0),rows.length);
});
test('three plugin inventory sources start together without requesting chain files and a tab switch discards the result',async()=>{
 const pending=[],host={innerHTML:'',querySelector:()=>null};let current='tasks';
 const context=setup({A:(...args)=>new Promise(resolve=>pending.push({args,resolve})),document:{getElementById:()=>host}});
 const render=context.Rplg();assert.equal(pending.length,3);assert.deepEqual(pending.map(r=>r.args[1]),['/api/youyuan/agent/workspaces','/api/youyuan/runtime/list','/api/plugin/list']);
 const loading=host.innerHTML;context.curTab='home';for(const r of pending)r.resolve({ok:true});await render;assert.equal(host.innerHTML,loading);
});
test('TXT command chains cannot enter the plugin inventory from installed, live, legacy or draft sources',()=>{
 const model=setup().window.yyPluginStudioModel;
 const rows=model.collect([{ok:true,workspaces:[{id:'chain-draft',language:'mcfunction'}]},
 {ok:true,installed:[{id:'chain-installed.txt',language:'mcfunction'}],instances:[{id:'chain-orphan.txt',language:'mcfunction',account:'main'}]},
 {ok:true,plugins:[{id:'lua-one',type:'lua',running:true},{id:'lua-two',type:'lua',running:true},{id:'legacy-chain',type:'mcfunction'}]},
 {ok:true,files:Array.from({length:8},(_,i)=>({config:{name:'chain-'+i+'.txt'}}))}],{});
 assert.equal(rows.length,2);assert.deepEqual(Array.from(rows,p=>p.id),['lua-one','lua-two']);
 assert.equal(model.groups(rows,'language').length,1);assert.equal(model.groups(rows,'state')[0].items.length,2);
});
test('an account change or in-flight mutation prevents a stale inventory paint',async()=>{
 let owner='main',busy=false;const pending=[],host={innerHTML:'',querySelector:()=>null};
 const context=setup({yyReadScope:()=>owner,yyPluginScopeBusy:()=>busy,A:()=>new Promise(resolve=>pending.push(resolve)),document:{getElementById:()=>host,querySelectorAll:()=>[]}});
 let render=context.Rplg();const loading=host.innerHTML;owner='second';for(const resolve of pending)resolve({ok:true});await render;assert.equal(host.innerHTML,loading);
 busy=true;pending.length=0;await context.Rplg();assert.equal(pending.length,0);
});
test('a failed source refresh retains the same account installed inventory and displays a load warning',async()=>{
 const fields=new Map(),events={};
 const field=()=>({innerHTML:'',textContent:'',hidden:true,dataset:{},addEventListener(){},querySelectorAll:()=>[]});
 for(const key of ['yps-inventory','yps-match-count'])fields.set(key,field());
 const host=field();host.querySelector=key=>{if(!events[key])events[key]=field();return events[key]};fields.set('tab-tasks',host);
 let fail=false;const context=setup({localStorage:{getItem:()=>null},document:{getElementById:key=>fields.get(key)||null,querySelectorAll:()=>[]},A:async(method,path)=>path.includes('/workspaces')?{ok:true,workspaces:[]}:path.includes('/runtime/list')?{ok:true,installed:[],instances:[]}:fail?{ok:false,error:'请求超时'}:{ok:true,plugins:[{id:'one',type:'lua',name:'原插件',running:true},{id:'two',type:'lua',running:false}]}});
 await context.Rplg();assert.match(fields.get('yps-inventory').innerHTML,/原插件/);assert.equal(fields.get('yps-match-count').textContent,'2 / 2 项');
 fail=true;await context.Rplg();assert.match(fields.get('yps-inventory').innerHTML,/原插件/);assert.equal(fields.get('yps-match-count').textContent,'2 / 2 项');assert.match(host.innerHTML,/部分列表暂时无法读取/);
});
