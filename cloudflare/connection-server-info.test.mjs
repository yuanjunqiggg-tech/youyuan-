import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFile} from 'node:fs/promises';

const source = await readFile(new URL('../upstream/frontend/js/youyuan_connection_policy.js', import.meta.url), 'utf8');
function fixture(result = {ok:true,candidates:[]}) {
  const nodes = new Map(['yy-connection-policy-state','yy-connection-server-info','yy-resume-reconnect'].map(id => [id,{textContent:''}]));
  const calls = [];
  let visible = true;
  const context = vm.createContext({document:{getElementById:id=>nodes.get(id)}, yyActiveAccount:'main',
    yySettingsData:{token:'saved-fixture',auth_url:'https://auth.example'}, yySettingsViewGuard:()=>()=>visible,
    Rcfg:async()=>{},refreshCfgAccount:async()=>{},T(){},
    A:async(method,path,body)=>{calls.push({method,path,body});
      if(path==='/api/youyuan/connection-policy') return {ok:true,policy:{connected:true}};
      if(path==='/api/bot/status') return {ok:true,connected:true,server:'98014267'};
      return typeof result==='function' ? result(body) : result;
    }});
  vm.runInContext(source,context);
  return {context,calls,nodes,hide(){visible=false},text:()=>nodes.get('yy-connection-server-info').textContent};
}
const row = {target:'98014267',room_id:'entity-id',number:'98014267',name:'测试服',players:0,max_players:40,owner:'服主甲'};

test('refresh reads real server metadata, preserves zero and refreshes again on manual action', async()=>{
  const f=fixture({ok:true,candidates:[row,{...row,target:'other',room_id:'other',number:'other',name:'别的服'}]});
  await f.context.yyRefreshConnectionPolicy();
  assert.match(f.text(),/服号 \/ 房间 ID：98014267/); assert.match(f.text(),/名称：测试服/);
  assert.match(f.text(),/当前人数：0/);assert.match(f.text(),/人数上限：40/);assert.match(f.text(),/服主：服主甲/);
  const query=f.calls.find(c=>c.path==='/api/youyuan/rooms');assert.equal(query.body.force,true);assert.equal(query.body.input,'98014267');assert.equal(query.body.token,'saved-fixture');
  await f.context.yyRefreshConnectionPolicy(false);
  assert.equal(f.calls.filter(c=>c.path==='/api/youyuan/rooms').length,1);
  await f.context.yyRefreshConnectionPolicy();
  assert.equal(f.calls.filter(c=>c.path==='/api/youyuan/rooms').length,2);
  assert.equal(f.calls.some(c=>/connect$|commands/.test(c.path)),false);
});
test('missing values and failed refresh clear old counts instead of inventing zero or owner',async()=>{
  let result={ok:true,candidates:[row]}; const f=fixture(()=>result);
  await f.context.yyRefreshConnectionPolicy();result={ok:false};await f.context.yyRefreshConnectionPolicy();
  assert.match(f.text(),/当前人数：未知/);assert.match(f.text(),/服主：未知/);assert.match(f.text(),/查询失败/);assert.doesNotMatch(f.text(),/服主甲/);
  result={ok:true,candidates:[{target:'98014267',owner_id:'123',players:-1,max_players:null}]};await f.context.yyRefreshConnectionPolicy();
  assert.match(f.text(),/当前人数：未知/);assert.match(f.text(),/人数上限：未知/);assert.match(f.text(),/服主：UID 123/);
});
test('ambiguous targets never borrow another room metadata; names render as text',async()=>{
  const f=fixture({ok:true,candidates:[row,{...row,name:'同号房间'}]});await f.context.yyRefreshConnectionPolicy();
  assert.match(f.text(),/唯一匹配/);assert.match(f.text(),/当前人数：未知/);
  const g=fixture({ok:true,candidates:[{...row,owner:'<img src=x onerror=alert(1)>'}]});await g.context.yyRefreshConnectionPolicy();
  assert.match(g.text(),/<img src=x/);assert.equal(g.nodes.get('yy-connection-server-info').innerHTML,undefined);
});
test('late metadata cannot overwrite a newer query, switched account, or detached view',async()=>{
  let resolve;let call=0;const f=fixture(()=>++call===1?new Promise(r=>resolve=r):{ok:true,candidates:[{...row,owner:'新数据'}]});
  const old=f.context.yyRefreshConnectionPolicy();await new Promise(setImmediate);
  await f.context.yyRefreshConnectionPolicy();resolve({ok:true,candidates:[row]});await old;
  assert.match(f.text(),/服主：新数据/);
  for(const change of [g=>g.hide(),g=>{g.context.yyActiveAccount='other'},g=>{g.nodes.set('yy-connection-server-info',{textContent:'new view'})}]){
    let finish;const g=fixture(()=>new Promise(r=>finish=r));const pending=g.context.yyRefreshConnectionPolicy();await new Promise(setImmediate);change(g);finish({ok:true,candidates:[row]});await pending;assert.doesNotMatch(g.text(),/服主甲/);
  }
});
test('periodic refresh queries a changed saved target instead of retaining previous server details',async()=>{
  const f=fixture(body=>({ok:true,candidates:[{...row,target:body.input,number:body.input}]}));
  await f.context.yyRefreshConnectionPolicy();
  const base=f.context.A;
  f.context.A=async(method,path,body)=>path==='/api/bot/status'?{ok:true,connected:false,server:'98014267',server_target:'12345'}:base(method,path,body);
  await f.context.yyRefreshConnectionPolicy(false);
  assert.match(f.text(),/服号 \/ 房间 ID：12345/);
  assert.equal(f.calls.filter(c=>c.path==='/api/youyuan/rooms').length,2);
});
test('initial metadata lookup does not delay the rest of settings initialization',async()=>{
  let finish; const f=fixture(()=>new Promise(resolve=>finish=resolve));
  await f.context.Rcfg();await new Promise(setImmediate);
  assert.equal(typeof finish,'function');
  assert.match(f.text(),/正在查询服务器信息/);
  finish({ok:true,candidates:[row]});await new Promise(setImmediate);
  assert.match(f.text(),/服主：服主甲/);
});
