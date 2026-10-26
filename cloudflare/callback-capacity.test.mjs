import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFile} from 'node:fs/promises';
const source=await readFile(new URL('../upstream/frontend/js/youyuan_sense.js',import.meta.url),'utf8');
function fixture(){
 const calls=[],nodes=new Map(),keys=['event_queue','priority_queue','runtime_queue','async_tickets','event_history','actors','stale_ms'];
 for(const k of keys)nodes.set('yy-limit-'+k,{value:256});nodes.set('yy-limit-status',{textContent:''});
 const context=vm.createContext({Rcfg:async()=>{},document:{getElementById:id=>nodes.get(id)},T(){},A:async(method,path,body)=>{calls.push({method,path,body});return {ok:true,result:{settings:{event_queue:512,priority_queue:256}}}}});
 vm.runInContext(source,context);return {context,calls,nodes};
}
test('client reads and writes the shared diagnostics.configure API',async()=>{
 const f=fixture();await f.context.yyObservationLoad();assert.equal(f.nodes.get('yy-limit-event_queue').value,512);
 f.nodes.get('yy-limit-event_queue').value=1024;await f.context.yyObservationSave();
 assert.equal(f.calls[1].path,'/api/youyuan/game/call');assert.equal(f.calls[1].body.method,'diagnostics.configure');assert.equal(f.calls[1].body.args[0].event_queue,1024);
 assert.match(f.nodes.get('yy-limit-status').textContent,/重载/);
});
test('invalid capacity does not send a change; shortcut opens the existing settings',async()=>{
 const f=fixture();f.nodes.get('yy-limit-event_queue').value=0;await f.context.yyObservationSave();assert.equal(f.calls.length,0);
 let opened=0,scrolled=0;f.context.yySensePage=async()=>{opened++};f.nodes.set('yy-limit-event_queue',{closest:()=>({scrollIntoView:()=>{scrolled++}})});
 await f.context.yyCallbackQueueSettings();assert.equal(opened,1);assert.equal(scrolled,1);
});
