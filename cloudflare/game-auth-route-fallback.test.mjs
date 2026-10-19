import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
const file=fs.readFileSync(new URL('../upstream/frontend/js/youyuan_game_auth.js',import.meta.url),'utf8');
const source=file.slice(file.indexOf('async function yyGameAuthAPI('),file.indexOf('async function yyGameAuthPage('));
function setup(cloud,replies){
 const calls=[];const context={yyIsCloud:cloud,AbortController,setTimeout,clearTimeout,yyRootFetch:async(path,options)=>{calls.push({path,options});return replies.shift()}};
 vm.createContext(context);vm.runInContext(source,context);return {calls,call:context.yyGameAuthAPI};
}
test('direct Linux gateway 404 retries only the missing cloud alias',async()=>{
 const t=setup(true,[new Response(JSON.stringify({ok:false,error:'not_found',path:'/api/game-auth/accounts'}),{status:404}),Response.json({ok:true,accounts:[{id:'owned'}]})]);
 const r=await t.call('GET');assert.equal(r.accounts[0].id,'owned');assert.deepEqual(t.calls.map(c=>c.path),['/api/game-auth/accounts','/api/youyuan/game-auth']);
});
test('authentication and unknown execution failures never replay POST',async()=>{
 for(const status of [401,403,500,503]){const t=setup(true,[new Response(JSON.stringify({ok:false,error:'blocked'}),{status})]);await t.call('POST',{action:'register_guest'});assert.equal(t.calls.length,1);}
 const t=setup(true,[new Response(JSON.stringify({ok:false,error:'account_not_found'}),{status:404})]);await t.call('POST',{action:'token'});assert.equal(t.calls.length,1);
});
test('native APK uses native credential route directly',async()=>{
 const t=setup(false,[Response.json({ok:true})]);await t.call('POST',{action:'skin_info',id:'owned'});assert.equal(t.calls[0].path,'/api/youyuan/game-auth');assert.equal(JSON.parse(t.calls[0].options.body).action,'skin_info');
});
