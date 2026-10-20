import test from 'node:test';import assert from 'node:assert/strict';import vm from 'node:vm';import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../upstream/frontend/js/youyuan_global_capture.js',import.meta.url),'utf8');
test('capture save reads back unknown transport result without replaying the write',async()=>{
 const calls=[],config={enabled:true,resident_observers:3,fleet_steps:[{min_players:1,observers:3}]};
 const c=vm.createContext({Rcfg:async()=>{},yyRootAPI:async(method,path,body)=>{calls.push(method);return method==='POST'?{ok:false,error:'Linux直连中断，操作结果可能未知'}:{ok:true,config}}});vm.runInContext(source,c);
 const r=await c.yyGlobalCaptureSave(config);assert.equal(r.ok,true);assert.equal(r.write_recovered,true);assert.deepEqual(calls,['POST','GET']);
 const failed=await c.yyGlobalCaptureSave({enabled:false});assert.equal(failed.ok,false);assert.match(failed.error,/保存结果尚未确认/);assert.deepEqual(calls,['POST','GET','POST','GET']);
});

// 0 is not a valid interval: yyPatrolDuration rejects sec<=0, so a user who
// typed 0 got a server-side "间隔需要正有限秒数" instead of a field hint.
// 1 ms stays the floor because 0.001 s is a documented, tested value.
test('the switch interval field cannot submit a zero interval',()=>{
 assert.match(source,/id="yy-gc-interval" type="number" step="any" min="1"/);
 assert.doesNotMatch(source,/id="yy-gc-interval" type="number" step="any" min="0"/);
 assert.match(source,/interval_seconds:Math\.max\(1,n\('yy-gc-interval'\)\)\/1000/);
});
