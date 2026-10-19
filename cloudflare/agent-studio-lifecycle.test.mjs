import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const read=name=>readFileSync(new URL('../upstream/frontend/js/'+name,import.meta.url),'utf8');
function setup(){
 let scope='owner/main';const requests=[],modals=[],fields=new Map();
 function element(){return {isConnected:true,value:'',textContent:'',disabled:false,children:[],append(...nodes){this.children.push(...nodes)},querySelector:key=>{if(!fields.has(key))fields.set(key,element());return fields.get(key)}}}
 const c=vm.createContext({yyWorkspaceActive:{id:'same-id'},yyWorkspaceOpenEpoch:1,yyEditorAccountScope:()=>scope,yyOpenWorkspace(){},yyWorkspacePane(){},yyDrawProjectFiles(){},yyOpenProjectFile(){},yyAgentPreferences(){},T(){},document:{getElementById:()=>null,createElement:element},yyAgentProfileModal(){const host=element();modals.push(host);return host},A:(...args)=>new Promise(resolve=>requests.push({args,resolve}))});c.window=c;vm.runInContext(read('youyuan_agent_studio.js'),c);
 return {c,requests,modals,fields,scope:value=>scope=value};
}
test('a budget reply from an old account, login or closed workspace cannot open a modal',async()=>{
 for(const change of ['scope','closed']){const p=setup(),pending=p.c.yyAgentRunBudgetDialog();if(change==='scope')p.scope('new-login/main');else p.c.yyWorkspaceOpenEpoch++;p.requests[0].resolve({ok:true,budget:{max_rounds:1,max_tools:2,max_minutes:3}});await pending;assert.equal(p.modals.length,0)}
});
test('an existing budget dialog cannot write into another scope or update itself from a late reply',async()=>{
 const p=setup(),pending=p.c.yyAgentRunBudgetDialog();p.requests[0].resolve({ok:true,budget:{max_rounds:1,max_tools:2,max_minutes:3}});await pending;const save=p.modals[0].children[0];p.modals[0].querySelector('#yy-budget-result');p.scope('new-login/main');await save.onclick();assert.equal(p.requests.length,1);
 p.scope('owner/main');const saving=save.onclick();assert.equal(p.requests.length,2);assert.equal(save.disabled,true);await save.onclick();assert.equal(p.requests.length,2);p.scope('new-login/main');p.requests[1].resolve({ok:true});await saving;assert.equal(p.fields.get('#yy-budget-result').textContent,'');
});
function models(){
 const pending=[],el={innerHTML:'',value:''};let scope='old-login';const c=vm.createContext({yyWorkspaceActive:{id:'same'},yyWorkspaceOpenEpoch:1,yyEditorAccountScope:()=>scope,__ai:{models:[{name:'existing'}]},yyEscape:String,document:{getElementById:()=>el},A:(...args)=>new Promise(resolve=>pending.push({args,resolve}))});
 const source=read('youyuan_workspace.js');vm.runInContext(source.slice(source.indexOf('var yyAgentModelsEpoch='),source.indexOf('async function yyAgentModels()')),c);return {c,pending,el,scope:value=>scope=value};
}
test('model requests completing out of order cannot replace the latest selected model',async()=>{
 const p=models(),first=p.c.yyLoadAgentModels('old'),second=p.c.yyLoadAgentModels('new');p.pending[2].resolve({ok:true,models:[{name:'new'}]});p.pending[3].resolve({allowed:false});await second;p.pending[0].resolve({ok:true,models:[{name:'old'}]});p.pending[1].resolve({allowed:false});await first;assert.equal(p.el.value,'new');assert.match(p.el.innerHTML,/new/);assert.doesNotMatch(p.el.innerHTML,/old/);
});
test('model errors retain the existing list, and old authentication scopes cannot replace it',async()=>{
 const p=models();let pending=p.c.yyLoadAgentModels('other');p.scope('new-login');p.pending[0].resolve({ok:true,models:[{name:'other'}]});p.pending[1].resolve({allowed:true});await pending;assert.equal(p.c.__ai.models[0].name,'existing');
 pending=p.c.yyLoadAgentModels('other');p.pending[2].resolve({ok:false,error:'offline'});p.pending[3].resolve({allowed:false});await assert.rejects(pending,/offline/);assert.equal(p.c.__ai.models[0].name,'existing');
});
test('unchanged polling retains file explorer button identity and only updates the active file',()=>{
 function element(){return {dataset:{},children:[],classList:{values:new Set(),toggle(k,on){on?this.values.add(k):this.values.delete(k)}},append(node){this.children.push(node)},replaceChildren(){this.children=[]},querySelectorAll(){return this.children}}}
 const list=element(),title={textContent:''},c=vm.createContext({yyWorkspaceActive:{files:[{path:'one.lua'},{path:'two.lua'}]},yyEditorPath:'one.lua',yyOpenWorkspace(){},yyWorkspacePane(){},yyDrawProjectFiles(){},yyOpenProjectFile(){},yyAgentPreferences(){},document:{getElementById:id=>id==='yy-studio-file-list'?list:id==='yy-studio-file-title'?title:null,createElement:element}});c.window=c;vm.runInContext(read('youyuan_agent_studio.js'),c);
 c.yyDrawProjectFiles(c.yyWorkspaceActive.files);const one=list.children[0],two=list.children[1];assert.ok(one.classList.values.has('active'));c.yyEditorPath='two.lua';c.yyDrawProjectFiles(c.yyWorkspaceActive.files);
 assert.equal(list.children[0],one);assert.equal(list.children[1],two);assert.equal(one.classList.values.has('active'),false);assert.equal(two.classList.values.has('active'),true);assert.equal(title.textContent,'two.lua');
});
