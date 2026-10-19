import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import worker from './worker.mjs';
import {identityHandle} from './identity.mjs';

function fixture(){
 const records=new Map(),calls=[],identityCalls=[];
 const ctx={storage:{get:async key=>records.get(key)}};
 const env={CONSOLE_TOKEN:'fixture-console',DEVICE_TOKEN:'fixture-device',ASSETS:{fetch:async()=>new Response('public asset')},RELAY:{
  idFromName:name=>name,
  get:name=>({fetch:async request=>{
   if(name==='identity-registry-v1'){identityCalls.push(new URL(request.url).pathname);return identityHandle(request,ctx,env);}
   calls.push({name,request});
   return Response.json({ok:true,linux_direct_url:'https://fixture.trycloudflare.com'});
  }})
 }};
 function user(id,extra={}){records.set('user:'+id,{id,username:id,subscription_until:Date.now()+3600000,assigned_devices:['linux-server'],...extra});}
 function token(value,id,kind='session'){records.set(kind+':'+createHash('sha256').update(value).digest('hex'),{user:id,expires:Date.now()+3600000});}
 user('owner',{legacy_owner:true});user('subscriber');user('admin',{role:'admin'});user('expired',{subscription_until:1});
 for(const id of ['owner','subscriber','admin','expired'])for(const kind of ['session','control'])token('fixture-'+id+'-'+kind,id,kind);
 return {env,calls,identityCalls,invoke:(token,path='/api/cloud/linux-direct',method='GET')=>worker.fetch(new Request('https://toolbox.test'+path+'?device=linux-server',{method,headers:{Authorization:'Bearer '+token}}),env)};
}

test('console-key browser login receives its real owner access state instead of an empty user',async()=>{
 const f=fixture(),r=await f.invoke('fixture-console','/api/cloud/status'),data=await r.json();
 assert.equal(r.status,200);assert.equal(data.user.cloud_allowed,true);assert.equal(data.user.role,'admin');assert.equal(data.user.legacy_owner,true);assert.equal(f.calls.length,0);
});

test('console key and a real legacy owner session discover the direct origin',async()=>{
 for(const token of ['fixture-console','fixture-owner-session']){
  const f=fixture(),r=await f.invoke(token),data=await r.json();
  assert.equal(r.status,200);assert.equal(data.origin,'https://fixture.trycloudflare.com');assert.equal(data.use_relay,undefined);
  assert.equal(r.headers.get('cache-control'),'no-store');assert.equal(f.calls.length,1);assert.equal(f.calls[0].name,'linux-server');
  assert.equal(f.identityCalls.includes('/__identity/me'),token==='fixture-owner-session');
 }
});

test('deployment can keep owners on relay when the public direct route is unreliable',async()=>{
 const f=fixture();f.env.LINUX_DIRECT_MODE='relay';
 for(const token of ['fixture-console','fixture-owner-session']){
  const r=await f.invoke(token),data=await r.json();assert.equal(r.status,200);assert.deepEqual(data,{ok:true,use_relay:true,device:'linux-server'});
 }
 assert.equal(f.calls.length,0,'relay mode must not depend on a failing direct tunnel or its health request');
 const r=await f.invoke('fixture-console','/api/cloud/status');assert.equal((await r.json()).linux_transport,'relay');
});

test('assigned subscriber sessions, personal control keys and non-owner admins stay on relay',async()=>{
 for(const token of ['fixture-subscriber-session','fixture-subscriber-control','fixture-owner-control','fixture-admin-session']){
  const f=fixture(),r=await f.invoke(token),data=await r.json();
  assert.equal(r.status,200);assert.deepEqual(data,{ok:true,use_relay:true,device:'linux-server'});
  assert.equal(r.headers.get('cache-control'),'no-store');assert.equal(f.calls.length,0,'relay-only discovery must not probe or disclose an owner endpoint');
  assert.equal(f.identityCalls.includes('/__identity/me'),token==='fixture-owner-control');
 }
});

test('invalid credentials and expired subscriptions cannot discover a direct origin',async()=>{
 const f=fixture();
 for(const [token,status] of [['invalid',401],['fixture-expired-session',403],['fixture-expired-control',403]]){
  const r=await f.invoke(token);assert.equal(r.status,status);assert.equal((await r.json()).origin,undefined);
 }
 assert.equal(f.calls.length,0);
});

test('relay-only subscriber keeps tenant actor, assigned shared node and root restrictions',async()=>{
 const f=fixture();await f.invoke('fixture-subscriber-session');
 let r=await f.invoke('fixture-subscriber-session','/api/youyuan/account-pool');assert.equal(r.status,200);
 const forwarded=f.calls[0];assert.equal(forwarded.name,'linux-server');
 const actor=JSON.parse(Buffer.from(forwarded.request.headers.get('X-YouYuan-Actor'),'base64url').toString());
 assert.equal(actor.id,'subscriber');assert.equal(actor.admin,false);
 r=await f.invoke('fixture-subscriber-session','/api/bot/connect','POST');assert.equal(r.status,403);assert.equal(f.calls.length,1);
});

test('browser API authentication failure is explicit while MCP keeps its challenge and assets stay public',async()=>{
 const f=fixture(),r=await f.invoke('invalid','/api/bot/status');
 assert.equal(r.status,401);assert.deepEqual(await r.json(),{ok:false,error:'登录状态已失效，请重新登录',code:'AUTH_REQUIRED'});
 const mcp=await f.invoke('invalid','/mcp/');assert.equal(mcp.status,401);assert.ok(mcp.headers.get('www-authenticate'));
 const asset=await f.invoke('invalid','/index.html');assert.equal(asset.status,200);assert.equal(await asset.text(),'public asset');
});
