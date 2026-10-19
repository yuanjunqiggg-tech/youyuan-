import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const read=path=>readFileSync(new URL('../upstream/frontend/'+path,import.meta.url),'utf8');
const deferred=()=>{let resolve;return {promise:new Promise(r=>resolve=r),resolve}};
function setup(){
 const fields=new Map(),frames=[],calls=[],copies=[],events={},streams=[];let scope='owner/main',response=async()=>({ok:true});
 function element(tag='div'){
  const el={tagName:tag.toUpperCase(),children:[],dataset:{},style:{},value:'',textContent:'',scrollTop:0,clientHeight:120,hidden:false,offsetParent:{},attributes:{},parentElement:null,focus(){},setAttribute(k,v){this.attributes[k]=v},append(...nodes){for(const n of nodes){n.parentElement=this;this.children.push(n)}},appendChild(n){this.append(n)},replaceChildren(...nodes){for(const n of this.children)n.parentElement=null;this.children=[];this.append(...nodes)},remove(){if(this.parentElement){const rows=this.parentElement.children;rows.splice(rows.indexOf(this),1);this.parentElement=null}},querySelector(selector){return this.children.find(n=>'.'+n.className===selector)||null}};
  Object.defineProperty(el,'firstChild',{get(){return this.children[0]}});Object.defineProperty(el,'scrollHeight',{get(){return this.children.length*40+200}});
  let html='';Object.defineProperty(el,'innerHTML',{get(){return html},set(v){html=v;if(this===fields.get('tab-terminal')){for(const key of [...fields.keys()])if(key!=='tab-terminal')fields.delete(key);for(const m of v.matchAll(/<([a-z]+)[^>]*\bid="([^"]+)"[^>]*>/g))fields.set(m[2],element(m[1]));}}});return el;
 }
 fields.set('tab-terminal',element());
 const context=vm.createContext({Map,Set,Number,String,Array,Promise,JSON,Date,curTab:'terminal',subPageOpen:false,tabInited:{},yyActiveAccount:'main',allLogs:[],yyReadScope:()=>scope,yyAccountName:id=>id==='main'?'真实游戏账号':'第二游戏账号',renderLogLine:m=>String(m).replace(/</g,'&lt;').replace(/>/g,'&gt;'),escHtml:String,requestAnimationFrame:fn=>{frames.push(fn);return frames.length},setTimeout:()=>0,clearTimeout(){},setInterval:()=>0,clearInterval(){},A:async(...args)=>{calls.push({scope,args});return response(...args)},U(){},updateConnStatus(){},tabTo(tab){context.curTab=tab;if(tab==='terminal'&&!context.tabInited.terminal){context.tabInited.terminal=true;context.Rconsole()}},navigator:{clipboard:{writeText:async text=>copies.push(text)}},EventSource:function(){this.listeners={};this.addEventListener=(k,fn)=>this.listeners[k]=fn;this.close=()=>this.closed=true;streams.push(this)},document:{getElementById:id=>fields.get(id)||null,createElement:element,addEventListener:(k,fn)=>events[k]=fn}});
 context.window=context;
 vm.runInContext(read('js/sse.js'),context);vm.runInContext(read('js/pages/terminal.js'),context);vm.runInContext(read('js/youyuan_terminal.js'),context);
 return {context,fields,calls,copies,events,streams,field:id=>fields.get(id),flush(){while(frames.length)frames.shift()()},response(fn){response=fn},scope(value){scope=value;context.yyActiveAccount=value.split('/').at(-1)}};
}
test('final enhancement owns terminal render after dashboard and studio, preserves business IDs and handlers',()=>{
 const html=read('index.html');assert.ok(html.indexOf('/js/youyuan_terminal.js')>html.indexOf('/js/youyuan_studio.js'));assert.ok(html.indexOf('/js/youyuan_terminal.js')>html.indexOf('/js/youyuan_dashboard.js'));assert.equal((html.match(/src="\/js\/youyuan_terminal.js"/g)||[]).length,1);assert.ok(html.indexOf('/css/youyuan_terminal.css')>html.indexOf('/css/youyuan_studio.css'));
 const p=setup();p.context.Rconsole();const markup=p.field('tab-terminal').innerHTML;for(const id of ['console-log','console-input','console-send'])assert.ok(p.field(id),id);assert.match(markup,/onclick="sendConsole\(\)"/);assert.match(markup,/onkeydown="yyTerminalKey\(event\)"/);assert.doesNotMatch(markup,/\/api\/|连接服务器|API 目录/);assert.equal(p.calls.length,0);assert.equal(p.streams.length,1);
});
test('SSE keeps the same subscription and logs survive leaving and returning to terminal',()=>{
 const p=setup();p.context.Rconsole();const stream=p.streams[0];stream.listeners.bot_log({data:JSON.stringify({msg:'first event'})});p.flush();assert.equal(p.context.allLogs.length,1);assert.equal(p.field('console-log').children[0].children[1].innerHTML,'first event');
 p.context.tabTo('home');p.field('console-log').offsetParent=null;stream.listeners.bot_log({data:JSON.stringify({msg:'event while away'})});p.context.tabTo('terminal');p.flush();assert.equal(p.context.allLogs.length,2);assert.equal(p.field('console-log').children.length,2);assert.equal(p.streams.length,1);assert.equal(stream.closed,undefined);assert.equal(p.calls.length,0);
});
test('search, type, copy, pause and local clear never send a command or reconnect',async()=>{
 const p=setup();p.context.Rconsole();p.context.addConsoleLine('danger <script>','e');p.context.addConsoleLine('normal','o');p.flush();assert.match(p.field('console-log').children[0].children[1].innerHTML,/&lt;script&gt;/);
 p.field('yy-terminal-search').value='danger';p.field('yy-terminal-search').oninput();assert.equal(p.field('console-log').children.length,1);await p.field('yy-terminal-copy').onclick();assert.deepEqual(p.copies,['[错误] danger <script>']);
 p.field('yy-terminal-search').value='';p.field('yy-terminal-search').oninput();p.field('yy-terminal-kind').value='result';p.field('yy-terminal-kind').onchange();assert.equal(p.field('console-log').children.length,1);
 p.context.yyTerminalFollow(false);const log=p.field('console-log');log.scrollTop=17;p.context.addConsoleLine('second result','o');p.flush();assert.equal(log.scrollTop,17);assert.equal(log.children.length,2);assert.match(p.field('yy-terminal-latest').textContent,/1 条/);p.context.yyTerminalFollow(true);assert.equal(log.scrollTop,log.scrollHeight);
 p.field('yy-terminal-clear').onclick();assert.equal(p.context.allLogs.length,0);assert.equal(p.calls.length,0);assert.equal(p.streams.length,1);
});
test('explicit keyboard send preserves scoped API and output, composition and Shift Enter do not send',async()=>{
 const p=setup();p.context.Rconsole();p.field('console-input').value='/say test';p.field('console-input').oninput();let prevented=0;
 for(const event of [{key:'Enter',isComposing:true},{key:'Enter',keyCode:229},{key:'Enter',shiftKey:true},{key:'a'}])p.context.yyTerminalKey({...event,preventDefault(){prevented++}});
 assert.equal(p.calls.length,0);assert.equal(prevented,0);p.response(async()=>({ok:true,output:['server result']}));p.context.yyTerminalKey({key:'Enter',preventDefault(){prevented++}});await new Promise(r=>setImmediate(r));p.flush();assert.equal(p.calls.length,1);assert.equal(p.calls[0].scope,'owner/main');assert.equal(p.calls[0].args[1],'/api/bot/console');assert.equal(p.calls[0].args[2].input,'/say test');assert.equal(prevented,1);assert.ok(p.context.allLogs.some(row=>row.msg==='▸ server result'));assert.equal(p.field('console-send').disabled,false);
});
test('pending sends cannot duplicate, repaint another account or leak a late response',async()=>{
 const p=setup(),wait=deferred();p.response(()=>wait.promise);p.context.Rconsole();p.field('console-input').value='hello';const pending=p.context.sendConsole();p.field('console-input').value='duplicate';await p.context.sendConsole();assert.equal(p.calls.length,1);
 p.scope('owner/child');p.context.Rconsole();assert.equal(p.context.allLogs.length,0);assert.equal(p.field('console-input').value,'');wait.resolve({ok:true,output:['private old output']});await pending;p.flush();assert.equal(p.context.allLogs.length,0);assert.equal(p.field('console-send').disabled,false);assert.doesNotMatch(p.field('yy-terminal-notice').textContent,/private/);
});
test('bounded buffer eviction, drafts and known disconnected state remain truthful across renders',()=>{
 const p=setup();p.context.Rconsole();p.field('console-input').value='/draft';p.field('console-input').oninput();p.context.Rconsole();assert.equal(p.field('console-input').value,'/draft');assert.match(p.field('yy-terminal-connection').textContent,/待同步/);p.context.updateConnStatus(false,false);assert.equal(p.field('yy-terminal-connection').textContent,'游戏未连接');
 for(let i=0;i<2010;i++)p.context.addConsoleLine('row '+i,'o');p.flush();assert.equal(p.context.allLogs.length,2000);assert.equal(p.field('console-log').children.length,2000);p.context.addConsoleLine('newest','o');p.flush();assert.equal(p.field('console-log').children.length,2000);assert.equal(p.field('console-log').children.at(-1).children[1].innerHTML,'newest');assert.equal(p.calls.length,0);
 p.events['yy-identity-ready']();p.context.Rconsole();assert.equal(p.context.allLogs.length,0);assert.equal(p.field('console-input').value,'');
});
test('transport errors release pending UI and never retry an uncertain game command',async()=>{
 const p=setup();p.context.Rconsole();p.response(async()=>{throw Error('response lost')});p.field('console-input').value='/one-time';await p.context.sendConsole();p.flush();assert.equal(p.calls.length,1);assert.equal(p.field('console-send').disabled,false);assert.match(p.field('yy-terminal-notice').textContent,/不会自动重发/);assert.ok(p.context.allLogs.some(row=>row.cl==='e'));
});
test('terminal history navigation is bounded, scoped and never sends while browsing',async()=>{
 const p=setup();p.context.Rconsole();for(let i=0;i<45;i++){p.field('console-input').value='/test '+i;await p.context.sendConsole()}const count=p.calls.length;
 const key=k=>p.context.yyTerminalKey({key:k,preventDefault(){}});key('ArrowUp');assert.equal(p.field('console-input').value,'/test 44');for(let i=0;i<60;i++)key('ArrowUp');assert.equal(p.field('console-input').value,'/test 5');for(let i=0;i<60;i++)key('ArrowDown');assert.equal(p.field('console-input').value,'');assert.equal(p.calls.length,count);
 p.scope('owner/child');p.context.Rconsole();key('ArrowUp');assert.equal(p.field('console-input').value,'');assert.equal(p.calls.length,count);
});
test('browsing terminal history restores the unsent draft and modified arrows remain native',async()=>{
 const p=setup();p.context.Rconsole();p.field('console-input').value='/previous';await p.context.sendConsole();p.field('console-input').value='unfinished draft';p.field('console-input').oninput();let prevented=0;
 const key=(name,extra={})=>p.context.yyTerminalKey({key:name,preventDefault(){prevented++},...extra});key('ArrowUp',{altKey:true});assert.equal(prevented,0);assert.equal(p.field('console-input').value,'unfinished draft');
 key('ArrowUp');assert.equal(p.field('console-input').value,'/previous');key('ArrowDown');assert.equal(p.field('console-input').value,'unfinished draft');assert.equal(p.calls.length,1);
 p.context.Rconsole();assert.equal(p.field('console-input').value,'unfinished draft');
});
