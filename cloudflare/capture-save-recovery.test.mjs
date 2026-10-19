import test from 'node:test';import assert from 'node:assert/strict';import vm from 'node:vm';import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../upstream/frontend/js/youyuan_global_capture.js',import.meta.url),'utf8');
test('capture save reads back unknown transport result without replaying the write',async()=>{
 const calls=[],config={enabled:true,resident_observers:3,fleet_steps:[{min_players:1,observers:3}]};
 const c=vm.createContext({Rcfg:async()=>{},yyRootAPI:async(method,path,body)=>{calls.push(method);return method==='POST'?{ok:false,error:'Linux直连中断，操作结果可能未知'}:{ok:true,config}}});vm.runInContext(source,c);
 const r=await c.yyGlobalCaptureSave(config);assert.equal(r.ok,true);assert.equal(r.write_recovered,true);assert.deepEqual(calls,['POST','GET']);
 const failed=await c.yyGlobalCaptureSave({enabled:false});assert.equal(failed.ok,false);assert.match(failed.error,/保存结果尚未确认/);assert.deepEqual(calls,['POST','GET','POST','GET']);
});
