import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../upstream/frontend/js/youyuan_global_capture.js',import.meta.url),'utf8');
test('many-observer policy accepts more than two and rejects ambiguous deployment rows',()=>{
 const c=vm.createContext({Rcfg:async()=>{}});vm.runInContext(source,c);
 assert.deepEqual(JSON.parse(JSON.stringify(c.yyCaptureFleetStepsRead('1:1\n8:8\n15:24\n22:32'))),[{min_players:1,observers:1},{min_players:8,observers:8},{min_players:15,observers:24},{min_players:22,observers:32}]);
 assert.equal(c.yyCaptureFleetStepsRead('').length,0);
 for(const bad of ['2:2','1:0','1:33','1:1\n1:2','1:1\n1001:3','1:1\n10:NaN'])assert.throws(()=>c.yyCaptureFleetStepsRead(bad));
});
test('task resume distinguishes unfinished imports from running, completed and unsupported tasks',()=>{
 const c=vm.createContext({});vm.runInContext(readFileSync(new URL('../upstream/frontend/js/pages/tasks.js',import.meta.url),'utf8'),c);
 for(const type of ['import','import_multi','mapart'])for(const status of ['paused','failed','cancelled'])assert.equal(c.yyTaskResumeState({type,status}).available,true);
 for(const status of ['running','done'])assert.equal(c.yyTaskResumeState({type:'import',status}).available,false);
 assert.equal(c.yyTaskResumeState({type:'export',status:'failed'}).available,false);
 assert.match(c.yyTaskResumeState({type:'import',status:'done'}).reason,/已完成/);
});
