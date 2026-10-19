import test from 'node:test';
import assert from 'node:assert/strict';
import {createTCPBridge} from './tcp-bridge.mjs';

function fixture(chunks){
 let closes=0;const writes=[],targets=[];
 const connect=(address,options)=>{targets.push({address,options});return {opened:Promise.resolve({}),closed:Promise.resolve(),readable:new ReadableStream({start(c){for(const chunk of chunks)c.enqueue(Uint8Array.from(chunk));c.close();}}),writable:new WritableStream({write(b){writes.push([...b]);}}),close(){closes++;return Promise.resolve();}}};
 return {connect,writes,targets,get closes(){return closes;}};
}
test('official Link transport preserves bytes and split reads, close is idempotent',async()=>{
 const f=fixture([[1,2,3,4],[5]]),b=createTCPBridge(f.connect),c=b.dial('223.252.200.165:10201');await c.opened;
 assert.deepEqual([...await c.read(2)],[1,2]);assert.deepEqual([...await c.read(10)],[3,4]);assert.deepEqual([...await c.read(10)],[5]);assert.equal((await c.read(1)).length,0);
 assert.equal(await c.write(Uint8Array.of(9,8)),2);assert.deepEqual(f.writes,[[9,8]]);
 assert.deepEqual(f.targets,[{address:{hostname:'223.252.200.165',port:10201},options:{secureTransport:'off',allowHalfOpen:false}}]);c.close();c.close();b.close();assert.equal(f.closes,1);
});
test('request instances do not share sockets and finalization closes pending sockets',()=>{
 const f=fixture([]),a=createTCPBridge(f.connect),b=createTCPBridge(f.connect);a.dial('223.252.200.165:10200');b.dial('106.2.57.140:10201');a.close();assert.equal(f.closes,1);b.close();assert.equal(f.closes,2);
});
test('TCP rejection is preserved and closes the failed socket',async()=>{
 let closed=0;const b=createTCPBridge(()=>({opened:Promise.reject(Error('connection refused')),closed:Promise.resolve(),close(){closed++;return Promise.resolve();}}));
 await assert.rejects(b.dial('106.2.57.140:10201').opened,/connection refused/);assert.equal(closed,1);b.close();assert.equal(closed,1);
});
test('private, malformed and non-Link destinations never reach connect',()=>{
 let n=0;const b=createTCPBridge(()=>{n++;throw Error('unexpected')});for(const address of ['127.0.0.1:10200','10.0.0.1:10201','192.168.1.1:10201','172.16.0.1:10201','169.254.169.254:10200','1.2.3.4:443','999.2.3.4:10200','api.example:10200','1.2.3:10201'])assert.throws(()=>b.dial(address));assert.equal(n,0);
});
