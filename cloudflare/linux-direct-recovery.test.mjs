import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFile} from 'node:fs/promises';
const source=await readFile(new URL('../upstream/frontend/js/youyuan_boot.js',import.meta.url),'utf8');
function fixture(handler,discovery={ok:false}){const calls=[],events=[],context={location:{hostname:'toolbox.youyuanqi.dpdns.org',href:'https://toolbox.youyuanqi.dpdns.org/?device=linux-server',search:'?device=linux-server'},history:{replaceState(){}},localStorage:{setItem(){}},sessionStorage:{getItem:()=> 'fixture-cloud-session'},dispatchEvent:event=>events.push(event.type),fetch:async(input,opts)=>{calls.push({input,opts});if(input.startsWith('/api/cloud/'))return Response.json(discovery);return handler(input,opts)},Headers,Response,Event,AbortController,URL,URLSearchParams,Promise,Date,setTimeout:()=>1,clearTimeout(){}};context.window=context;vm.createContext(context);vm.runInContext(source,context);context.yyLinuxDirectOrigin='https://fixture.trycloudflare.com';return {context,calls,events};}

test('explicit preview/navigation abort preserves direct origin and original abort error',async()=>{
 const abort=Object.assign(Error('cancelled'),{name:'AbortError'}),f=fixture(async()=>{throw abort});
 await assert.rejects(f.context.fetch('/api/preview'),e=>e===abort);
 assert.equal(f.context.yyLinuxDirectOrigin,'https://fixture.trycloudflare.com');assert.equal(f.context.yyLinuxTransportStatus.failures,0);
 assert.equal(f.calls.filter(c=>c.input.endsWith('/api/preview')).length,1);
});
test('transient direct HTTP response retries a read once through authenticated relay',async()=>{
 const f=fixture(async(input,opts)=>{assert.equal(opts.headers.get('Authorization'),'Bearer fixture-cloud-session');return input.startsWith('https://')?Response.json({ok:false},{status:503}):Response.json({ok:true,source:'relay'});});
 assert.equal((await (await f.context.fetch('/api/bot/status')).json()).source,'relay');
 assert.equal(f.calls.filter(c=>c.input.endsWith('/api/bot/status')).length,2);assert.equal(f.context.yyLinuxDirectOrigin,'');assert.equal(f.context.yyLinuxTransportStatus.failures,1);
});
test('uncertain direct write is never replayed and exposes explicit outcome without secrets',async()=>{
 const f=fixture(async()=>{throw TypeError('network failed')});
 await assert.rejects(f.context.fetch('/api/tasks/start',{method:'POST',body:'fixture'}),e=>e.code==='LINUX_DIRECT_RESULT_UNKNOWN'&&e.outcome==='unknown'&&e.retryable===false);
 assert.equal(f.calls.filter(c=>c.input.endsWith('/api/tasks/start')).length,1);assert.equal(JSON.stringify(f.context.yyLinuxTransportStatus).includes('fixture-cloud-session'),false);
});
test('failed relay read is not replayed for a second time; authorization errors remain intact',async()=>{
 const failed=fixture(async input=>{if(input.startsWith('https://'))return Response.json({ok:false},{status:502});throw TypeError('relay also failed')});
 await assert.rejects(failed.context.fetch('/api/tasks/list'));assert.equal(failed.calls.filter(c=>c.input.endsWith('/api/tasks/list')).length,2);
 const denied=fixture(async()=>Response.json({ok:false,error:'permission denied'},{status:403}));assert.equal((await denied.context.fetch('/api/tasks/list')).status,403);assert.equal(denied.calls.filter(c=>c.input.endsWith('/api/tasks/list')).length,1);
});

test('direct 401 clears stale origin and retries a safe read only once with relay credentials',async()=>{
 const f=fixture(async(input,opts)=>{
  assert.equal(opts.headers.get('Authorization'),'Bearer fixture-cloud-session');
  return input.startsWith('https://')?Response.json({ok:false,error:'owner only'},{status:401}):Response.json({ok:true,source:'relay'});
 });
 assert.equal((await (await f.context.fetch('/api/bot/status')).json()).source,'relay');
 assert.equal(f.calls.filter(c=>c.input.endsWith('/api/bot/status')).length,2);assert.equal(f.context.yyLinuxDirectOrigin,'');
});

test('explicit pre-dispatch direct write rejection may return to relay once',async()=>{
 const f=fixture(async(input,opts)=>{
  assert.equal(opts.body,'fixture-body');
  return input.startsWith('https://')?Response.json({ok:false,code:'DIRECT_ACTOR_UNSUPPORTED',outcome:'not_sent'},{status:401}):Response.json({ok:true,source:'relay'});
 });
 assert.equal((await (await f.context.fetch('/api/tasks/start',{method:'POST',body:'fixture-body'})).json()).source,'relay');
 assert.equal(f.calls.filter(c=>c.input.endsWith('/api/tasks/start')).length,2);assert.equal(f.context.yyLinuxDirectOrigin,'');
});

test('ambiguous 401, wrong rejection fields, non-JSON and 5xx writes never replay',async()=>{
 for(const response of [
  ()=>Response.json({error:'unauthorized'},{status:401}),
  ()=>Response.json({code:'DIRECT_ACTOR_UNSUPPORTED',outcome:'unknown'},{status:401}),
  ()=>Response.json({code:'OTHER',outcome:'not_sent'},{status:401}),
  ()=>new Response('unauthorized',{status:401}),
  ()=>Response.json({code:'DIRECT_ACTOR_UNSUPPORTED',outcome:'not_sent'},{status:503})
 ]){
  const f=fixture(async()=>response()),r=await f.context.fetch('/api/tasks/start',{method:'POST',body:'fixture'});
  assert.ok([401,503].includes(r.status));assert.equal(f.calls.filter(c=>c.input.endsWith('/api/tasks/start')).length,1);assert.equal(f.context.yyLinuxDirectOrigin,'');
 }
});

test('a streaming write body is not reused even after an explicit not-sent rejection',async()=>{
 const f=fixture(async()=>Response.json({code:'DIRECT_ACTOR_UNSUPPORTED',outcome:'not_sent'},{status:401}));
 const body=new ReadableStream({start(controller){controller.enqueue(new Uint8Array([1]));controller.close();}});
 assert.equal((await f.context.fetch('/api/tasks/start',{method:'POST',body,duplex:'half'})).status,401);
 assert.equal(f.calls.filter(c=>c.input.endsWith('/api/tasks/start')).length,1);
});

test('abort signaled with a non-AbortError preserves transport without retry',async()=>{
 const controller=new AbortController(),abort=TypeError('cancelled by runtime');
 const f=fixture(async()=>{controller.abort();throw abort});
 await assert.rejects(f.context.fetch('/api/tasks/list',{signal:controller.signal}),e=>e===abort);
 assert.equal(f.context.yyLinuxDirectOrigin,'https://fixture.trycloudflare.com');assert.equal(f.context.yyLinuxTransportStatus.failures,0);
 assert.equal(f.calls.filter(c=>c.input.endsWith('/api/tasks/list')).length,1);
});

test('relay-only discovery clears any cached direct origin before subsequent API calls',async()=>{
 const f=fixture(async input=>{assert.equal(input,'/api/bot/status');return Response.json({ok:true})},{ok:true,use_relay:true});
 await f.context.yyDiscoverLinux();
 assert.equal(f.context.yyLinuxDirectOrigin,'');assert.equal(f.context.yyLinuxTransportStatus.mode,'relay');
 assert.equal((await f.context.fetch('/api/bot/status')).status,200);
});

test('a direct 401 followed by relay 401 returns the relay response without looping',async()=>{
 const f=fixture(async input=>Response.json({error:input.startsWith('https://')?'direct denied':'login required'},{status:401}));
 const response=await f.context.fetch('/api/bot/status');
 assert.equal(response.status,401);assert.equal((await response.json()).error,'login required');
 assert.equal(f.calls.filter(c=>c.input.endsWith('/api/bot/status')).length,2);
});

test('a permitted write fallback never retries a failed relay write',async()=>{
 const f=fixture(async input=>{if(input.startsWith('https://'))return Response.json({code:'DIRECT_ACTOR_UNSUPPORTED',outcome:'not_sent'},{status:401});throw TypeError('relay result unknown')});
 await assert.rejects(f.context.fetch('/api/tasks/start',{method:'POST',body:'fixture'}),/relay result unknown/);
 assert.equal(f.calls.filter(c=>c.input.endsWith('/api/tasks/start')).length,2);
});

test('known same-origin authentication failures request login without claiming device outage or replaying a write',async()=>{
 for(const body of [{ok:false,code:'AUTH_REQUIRED',error:'登录状态已失效，请重新登录'},{error:'中继密钥不正确'}]){
  const f=fixture(async()=>Response.json(body,{status:401}));f.context.yyLinuxDirectOrigin='';
  const response=await f.context.fetch('/api/tasks/start',{method:'POST',body:'fixture'});
  assert.equal(response.status,401);assert.deepEqual(f.events,['yy-cloud-auth-expired']);
  assert.equal(f.context.yyLinuxTransportStatus.auth_required,true);assert.equal(f.context.yyLinuxTransportStatus.failures,0);
  assert.equal(f.calls.filter(c=>c.input.endsWith('/api/tasks/start')).length,1);
 }
});

test('provider generic 401, direct gateway 401 and identity login failure do not expire platform login',async()=>{
 const provider=fixture(async()=>Response.json({error:'invalid API key'},{status:401}));provider.context.yyLinuxDirectOrigin='';
 await provider.context.fetch('/api/provider');assert.deepEqual(provider.events,[]);
 const direct=fixture(async()=>Response.json({code:'AUTH_REQUIRED'},{status:401}));
 await direct.context.fetch('/api/tasks/start',{method:'POST',body:'fixture'});assert.deepEqual(direct.events,[]);
 const identity=fixture(async()=>Response.json({code:'AUTH_REQUIRED'},{status:401}));
 await identity.context.fetch('/api/identity/login',{method:'POST',body:'fixture'});assert.deepEqual(identity.events,[]);
});

test('late old-session authentication errors do not expire a newly signed-in user',async()=>{
 let finish;const held=new Promise(resolve=>finish=resolve);
 const f=fixture(async()=>{await held;return Response.json({code:'AUTH_REQUIRED'},{status:401})});f.context.yyLinuxDirectOrigin='';
 const pending=f.context.fetch('/api/bot/status');f.context.sessionStorage.getItem=()=> 'fixture-new-session';finish();await pending;
 assert.deepEqual(f.events,[]);assert.notEqual(f.context.yyLinuxTransportStatus.auth_required,true);
});
