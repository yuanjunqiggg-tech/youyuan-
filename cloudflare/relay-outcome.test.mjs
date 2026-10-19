import test from 'node:test';
import assert from 'node:assert/strict';
import {relayFailure,YouYuanRelay} from './worker.mjs';

test('offline before send is retryable, interrupted writes are not replayed',async()=>{
 const offline=await relayFailure('offline',503,'trace',false,false).json();
 assert.equal(offline.submitted,false);assert.equal(offline.retryable,true);assert.equal(offline.error_code,'not_sent');
 const ambiguous=await relayFailure('lost',503,'trace',true,false).json();
 assert.equal(ambiguous.submitted,true);assert.equal(ambiguous.retryable,false);assert.equal(ambiguous.auto_retried,false);
 const read=await relayFailure('lost',504,'trace',true,true).json();assert.equal(read.retryable,true);
});
test('a disconnected relay settles every pending request with its own trace',async()=>{
 const socket={readyState:1};const relay=new YouYuanRelay({getWebSockets:()=>[socket]},{});
 relay.socket=socket;
 const responses=[];
 for(const [id,readOnly] of [['write',false],['read',true]])relay.pending.set(id,{id,readOnly,timer:setTimeout(()=>{},60000),resolve:r=>responses.push(r)});
 relay.webSocketClose(socket);
 assert.equal(relay.pending.size,0);assert.equal(responses.length,2);
 const [a,b]=await Promise.all(responses.map(r=>r.json()));
 assert.equal(a.trace_id,'write');assert.equal(a.retryable,false);assert.equal(b.trace_id,'read');assert.equal(b.retryable,true);
});
