import assert from 'node:assert/strict';import test from 'node:test';import worker from './worker.mjs';import {readPacketAllowed} from './mcp-link-policy.mjs';
test('read MCP grant rejects mutations and arbitrary API calls',()=>{
 for(const p of [{method:'tools/call',params:{name:'api_call',arguments:{path:'/api/bot/console'}}},{method:'tools/call',params:{name:'api_bot_status',arguments:{method:'POST'}}},{method:'resources/read',params:{uri:'youyuan://account/main/project/a/config.json'}}])assert.equal(readPacketAllowed(p),false);
 assert.equal(readPacketAllowed({method:'tools/call',params:{name:'api_bot_status'}}),true);
});
test('dedicated link pins device even with hostile query and header',async()=>{
 const selected=[],token='a'.repeat(64);const env={ASSETS:{fetch:async()=>new Response('asset')},RELAY:{idFromName:x=>x,get:name=>({fetch:async req=>{
  if(name==='identity-registry-v1')return Response.json({ok:true,owner:true,device:'linux-server',scope:'read',user:{cloud_allowed:true},relay_prefix:''});
  selected.push(name);assert.equal(new URL(req.url).searchParams.get('device'),'linux-server');assert.equal(req.headers.get('X-YouYuan-Device'),null);return Response.json({jsonrpc:'2.0',id:1,result:{tools:[{name:'api_bot_console'},{name:'api_bot_status'}]}});
 }})}};
 const response=await worker.fetch(new Request('https://example.test/mcp/link/'+token+'?device=someone-else',{method:'POST',headers:{'X-YouYuan-Device':'someone-else'},body:JSON.stringify({jsonrpc:'2.0',id:1,method:'tools/list'})}),env);
 assert.deepEqual(selected,['linux-server']);assert.deepEqual((await response.json()).result.tools,[{name:'api_bot_status'}]);
});
