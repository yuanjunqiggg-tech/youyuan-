import {test} from 'node:test';
import assert from 'node:assert/strict';
import {poolActor,encodePoolActor,signPoolActor,sharedPoolAllowed} from './pool-identity.mjs';
test('validated platform principal determines pool owner and role',()=>{
 const ordinary=poolActor({user:{id:'alice',role:'member'},scope:'read'});
 assert.equal(ordinary.id,'alice');assert.equal(ordinary.admin,false);assert.equal(ordinary.scope,'read');
 assert.equal(poolActor({user:{id:'old-id',legacy_owner:true}}).id,'owner');assert.equal(poolActor(null),null);
});
test('relay identity uses a real per-connection HMAC',async()=>{
 const actor=poolActor({user:{id:'疣',role:'member'}}),raw=encodePoolActor(actor),key='ab'.repeat(32);
 assert.deepEqual(JSON.parse(Buffer.from(raw,'base64url').toString()),actor);
 const sig=await signPoolActor(raw,key);assert.match(sig,/^[a-f0-9]{64}$/);
 assert.notEqual(await signPoolActor(raw+'x',key),sig);assert.notEqual(await signPoolActor(raw,'ac'.repeat(32)),sig);
 await assert.rejects(signPoolActor(raw,''));
});
test('shared subscription cannot operate the owner root through API or MCP',()=>{
 assert.equal(sharedPoolAllowed('/api/bot/connect','POST'),false);
 assert.equal(sharedPoolAllowed('/api/youyuan/accounts/invoke/main/api/config','GET'),false);
 assert.equal(sharedPoolAllowed('/mcp/','POST',{method:'tools/call',params:{name:'api_bot_connect',arguments:{}}}),false);
 assert.equal(sharedPoolAllowed('/mcp/','POST',{method:'resources/read',params:{uri:'youyuan://accounts/status'}}),false);
 assert.equal(sharedPoolAllowed('/mcp/','POST',{method:'tools/call',params:{name:'api_call',arguments:{path:'/api/youyuan/accounts/invoke/main/api/config'}}}),false);
 assert.equal(sharedPoolAllowed('/mcp/','POST',{method:'tools/call',params:{name:'api_bot_connect',arguments:{account:'alice_bot'}}}),true);
 assert.equal(sharedPoolAllowed('/mcp/','POST',{method:'tools/call',params:{name:'api_youyuan_account_pool',arguments:{body:{action:'acquire'}}}}),true);
 assert.equal(sharedPoolAllowed('/mcp/','POST',{method:'resources/read',params:{uri:'youyuan://docs/lua-runtime'}}),true);
});
