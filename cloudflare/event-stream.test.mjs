import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

function fixture(){
 const timers=[];let pending;
 const context={console,document:{hidden:false},setTimeout:fn=>{timers.push(fn);return timers.length},clearTimeout:()=>{},fetch:()=>new Promise(resolve=>{pending=resolve}),window:{}};
 context.window.fetch=context.fetch;vm.createContext(context);
 vm.runInContext(readFileSync(new URL('../upstream/frontend/js/youyuan_account_boot.js',import.meta.url),'utf8'),context);
 const source=new context.window.EventSource('/stream'),seen=[];source.addEventListener('bot_log',e=>seen.push(e.data));
 return {source,seen,timers,respond:async data=>{pending({ok:true,json:async()=>data});await new Promise(resolve=>setImmediate(resolve))}};
}
test('closing an in-flight poll cannot append duplicate terminal output',async()=>{
 const f=fixture();f.timers.shift()();f.source.close();await f.respond({ok:true,cursor:1,events:[{seq:1,type:'bot_log',data:'late'}]});assert.deepEqual(f.seen,[]);assert.equal(f.timers.length,0);
});
test('repeated sequence is ignored while genuinely repeated messages remain visible',async()=>{
 const f=fixture();f.timers.shift()();await f.respond({ok:true,cursor:0,events:[]});
 f.timers.shift()();await f.respond({ok:true,cursor:2,events:[{seq:1,type:'bot_log',data:'same'},{seq:2,type:'bot_log',data:'same'}]});
 f.timers.shift()();await f.respond({ok:true,cursor:2,events:[{seq:1,type:'bot_log',data:'same'},{seq:2,type:'bot_log',data:'same'}]});
 assert.deepEqual(f.seen,['same','same']);f.source.close();
});
