import assert from 'node:assert/strict';
import test from 'node:test';
import worker from './worker.mjs';

function environment(allowed=true){
 const selected=[];
 return {selected,CONSOLE_TOKEN:'owner-fixture',DEVICE_TOKEN:'device-fixture',ASSETS:{fetch:async()=>new Response('asset')},RELAY:{
  idFromName:name=>name,
  get:name=>({fetch:async request=>{
   if(name==='identity-registry-v1')return Response.json({ok:true,user:{role:'member',cloud_allowed:allowed,assigned_devices:['linux-server']},relay_prefix:'tenant:subscriber:'});
   selected.push(name);return Response.json({ok:true,node:name});
  }})
 }};
}
test('assigned subscribers control Linux but cannot replace its device channel',async()=>{
 const env=environment();
 let r=await worker.fetch(new Request('https://toolbox.test/api/bot/status?device=linux-server',{headers:{Authorization:'Bearer subscriber-fixture'}}),env);
 assert.equal(r.status,200);assert.equal((await r.json()).node,'linux-server');
 r=await worker.fetch(new Request('https://toolbox.test/device/connect?device=linux-server',{headers:{Authorization:'Bearer subscriber-fixture',Upgrade:'websocket'}}),env);
 assert.equal((await r.json()).node,'tenant:subscriber:linux-server');
 assert.deepEqual(env.selected,['linux-server','tenant:subscriber:linux-server']);
});
test('expired old sessions cannot bypass entitlement through MCP, files or auth account list',async()=>{
 const env=environment(false);
 for(const path of ['/mcp/?device=linux-server','/api/youyuan/host/files','/api/game-auth/accounts','/health']){
  const r=await worker.fetch(new Request('https://toolbox.test'+path,{headers:{Authorization:'Bearer old-session-fixture'}}),env);
  assert.equal(r.status,403);assert.equal((await r.json()).code,'SUBSCRIPTION_REQUIRED');
 }
 assert.deepEqual(env.selected,[]);
});
