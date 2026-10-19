import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const source=readFileSync(new URL('../upstream/frontend/js/youyuan_game_controls.js',import.meta.url),'utf8');
function deferred(){let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}}
function fixture(){
 const all=new Map(),calls=[];let owner='main',baseRender=async()=>{};
 class Element{
  constructor(){this.nodes=new Map();this.isConnected=true;this.disabled=false;this.value='';this.textContent='';this.checked=false}
  set id(v){this._id=v;all.set(v,this)}get id(){return this._id}
  set innerHTML(v){this.html=v;for(const m of v.matchAll(/<[^>]+id="([^"]+)"[^>]*>/g)){const n=new Element();n.id=m[1];n.disabled=/\sdisabled(?:[\s>])/.test(m[0]);const value=/value="([^"]*)"/.exec(m[0]);if(value)n.value=value[1];this.nodes.set('#'+n.id,n)}this.nodes.set('[role=status]',new Element())}
  querySelector(s){return this.nodes.get(s)||null}
  append(...nodes){for(const n of nodes)n.parent=this}
  remove(){this.isConnected=false;all.delete(this.id)}
 }
 const root=new Element();
 const context=vm.createContext({window:null,document:{createElement:()=>new Element(),body:new Element(),getElementById:id=>all.get(id),querySelector:s=>s==='#tab-settings>.content-inner'?root:null},Rcfg:()=>baseRender(),yyReadScope:()=>owner,curTab:'settings',subPageOpen:false,crypto:{randomUUID:()=>`request-${calls.length}`},A:async(method,url,body)=>{const d=deferred();calls.push({method,url,body,...d});return d.promise}});
 context.window=context;vm.runInContext(source,context);
 return {context,calls,all,root,owner:v=>owner=v,base:fn=>baseRender=fn,element:id=>all.get(id)};
}
const attributes={session:'test-session',player:'controlled-bot',connected:true,can_modify:true,level:3,progress:.25};

test('experience is read without writes; refresh during a pending change cannot unlock a second submission',async()=>{
 const f=fixture(),opened=f.context.yyExperiencePage();assert.equal(f.calls[0].method,'GET');f.calls[0].resolve({ok:true,experience:attributes});await opened;
 const apply=f.element('yy-xp-apply'),refresh=f.element('yy-xp-refresh');f.element('yy-xp-operation').value='add_levels';
 assert.equal(f.element('yy-xp-level').textContent,'3');assert.equal(f.element('yy-xp-progress').textContent,'25.0%');
 const change=apply.onclick();assert.equal(f.calls.length,2);assert.equal(f.calls[1].body.expected_session,'test-session');assert.equal(apply.disabled,true);
 const read=refresh.onclick();f.calls[2].resolve({ok:true,experience:attributes});await read;assert.equal(apply.disabled,true);
 await apply.onclick();assert.equal(f.calls.filter(c=>c.method==='POST').length,1);
 f.calls[1].resolve({status:'outcome_unknown',error:'回执未确认',after:attributes});await change;assert.equal(apply.disabled,false);assert.equal(f.calls.filter(c=>c.method==='POST').length,1);
});
test('an account switch discards a late attribute response',async()=>{
 const f=fixture(),opened=f.context.yyExperiencePage();f.owner('other');f.calls[0].resolve({ok:true,experience:attributes});await opened;
 assert.equal(f.element('yy-xp-level').textContent,'');assert.equal(f.calls.length,1);
});
test('failed egress read never enables saving defaults over an unknown existing configuration',async()=>{
 const f=fixture(),opened=f.context.yyGameEgressPage();f.calls[0].resolve({ok:false,error:'连接不可用'});await opened;
 assert.equal(f.element('yy-egress-save').disabled,true);await f.element('yy-egress-save').onclick();assert.equal(f.calls.length,1);
});
test('egress shows the actual Linux execution host and only saves on explicit submission',async()=>{
 const f=fixture(),opened=f.context.yyGameEgressPage();f.calls[0].resolve({ok:true,configuration:{mode:'system',has_password:true},runtime:{mode:'system',state:'configured',host_kind:'linux',verified_game_session:false}});await opened;
 assert.match(f.element('yy-egress-runtime').textContent,/Linux 执行节点网络/);assert.equal(f.calls.length,1);assert.equal(f.element('yy-egress-save').disabled,false);
 f.element('yy-egress-mode').value='socks5';f.element('yy-egress-mode').onchange();assert.equal(f.element('yy-egress-proxy').open,true);
 const saving=f.element('yy-egress-save').onclick();assert.equal(f.calls[1].body.mode,'socks5');f.calls[1].resolve({ok:true});await saving;assert.equal(f.element('yy-egress-password').value,'');
});
test('settings extension ignores an abandoned base render',async()=>{
 const f=fixture(),base=deferred();f.base(()=>base.promise);const render=f.context.Rcfg();f.context.curTab='tasks';base.resolve();await render;assert.equal(f.element('yy-game-controls-entry'),undefined);
});
