import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

test('a paused job refreshes the task UI without reporting completion or failure',()=>{
 const handlers={},notices=[];let reset=0,refresh=0,pause=0;
 const ctx={EventSource:class {addEventListener(name,fn){handlers[name]=fn}close(){}},document:{getElementById(){return null}},allLogs:[],curTab:'home',subPageOpen:false,
  T:(...args)=>notices.push(args),resetTaskButtons:()=>reset++,refreshHomeTasks:()=>refresh++,cbPauseImport:()=>pause++,cbFinalizeImport:()=>assert.fail('pause must not finalize as complete or failed')};
 vm.createContext(ctx);vm.runInContext(readFileSync(new URL('../upstream/frontend/js/sse.js',import.meta.url),'utf8'),ctx);
 handlers.task_start({data:JSON.stringify({type:'import'})});
 assert.ok(ctx._currentTask);
 handlers.task_paused({data:JSON.stringify({msg:'断点已保存'})});
 assert.equal(ctx._currentTask,null);assert.equal(reset,1);assert.equal(pause,1);assert.ok(refresh>=1);
 assert.deepEqual(notices.at(-1),['断点已保存','s']);
});
