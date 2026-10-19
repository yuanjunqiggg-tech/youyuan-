import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {identityHandle} from '../cloudflare/identity.mjs';
import {hash} from './vault.mjs';
import {bearerDirectory,indexGameToken,resolveGameBearer} from './game-bearer.mjs';

let source=await readFile(new URL('./worker.mjs',import.meta.url),'utf8');
source=source.replace("import {runKernel} from './runtime.mjs';",'const runKernel=async()=>{throw Error("official provider not configured in this fixture")};');
source=source.replace(/from '(\.[^']+)'/g,(_,p)=>'from '+JSON.stringify(new URL(p,import.meta.url).href));
const {default:worker,AuthVault}=await import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
function store(){const data=new Map();const storage={get:async k=>structuredClone(data.get(k)),put:async(k,v)=>data.set(k,structuredClone(v)),delete:async k=>{for(const key of Array.isArray(k)?k:[k])data.delete(key)},list:async(o={})=>new Map([...data].filter(([k])=>k.startsWith(o.prefix||''))),transaction:async fn=>fn(storage),setAlarm:async()=>{}};return {data,storage};}
function fixture(){const registry=store(),instances=new Map();let identityCalls=0;const env={AUTH_VAULT_KEY:'12'.repeat(32),CONSOLE_TOKEN:'fixture-platform-admin',RELAY:{idFromName:n=>n,get:()=>({fetch:req=>{identityCalls++;return identityHandle(req,registry,env)}})},AUTH:{idFromName:n=>n,get:n=>{if(!instances.has(n))instances.set(n,new AuthVault(store(),env));return instances.get(n)}}};return {env,registry,instances,get identityCalls(){return identityCalls}};}
const req=(path,body,authorization)=>new Request('https://auth'+path,{method:body?'POST':'GET',headers:{...(authorization?{Authorization:'Bearer '+authorization}:{}),'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});
async function row(v,id,char){const r={id,name:id,game_id:'game-'+id,login_token:'yy//'+char.repeat(64),client:{id},last_refresh:Date.now(),keep_alive:false};await v.put(r);await v.ctx.storage.put('token:'+await hash(r.login_token),id);return r;}
const successfulKernel=async p=>{assert.equal(p.action,'game_login');return {client:p.client,result:{UID:'game',EngineVersion:'test',PatchVersion:'test',IP:'127.0.0.1:19132',ChainInfo:'fixture-only'}};};

test('expired read-only rental search refreshes same account once before reconnect',async()=>{
 const f=fixture(),v=f.env.AUTH.get('account:owner'),r=await row(v,'recover-search','d'),calls=[];
 v.kernel=async p=>{calls.push(p.action);if(p.action==='refresh')return {client:{...p.client,fresh:true}};if(!p.client.fresh)throw Error('SearchRentalServerByName: 请先登录(10)');return successfulKernel(p)};
 const request=req('/api/phoenix/login',{login_token:r.login_token,server_code:'123',client_public_key:'fixture'});request.headers.set('X-Auth-Session','fixture');
 const result=await (await v.fetch(request)).json();assert.equal(result.success,true);assert.deepEqual(calls,['game_login','refresh','game_login']);assert.equal((await v.get(r.id)).client.fresh,true);
});
test('uncertain or post-GameStart failures never trigger session refresh and replay',async()=>{
 for(const message of ['GenerateRentalGameAuthV2: 请先登录(10)','SendGameStart: timeout','link.Dial: connection refused']){
  const f=fixture(),v=f.env.AUTH.get('account:owner'),r=await row(v,'no-replay','e'),calls=[];v.kernel=async p=>{calls.push(p.action);throw Error(message)};
  const request=req('/api/phoenix/login',{login_token:r.login_token,server_code:'123',client_public_key:'fixture'});request.headers.set('X-Auth-Session','fixture');
  assert.equal((await (await v.fetch(request)).json()).ok,false);assert.deepEqual(calls,['game_login']);
 }
});
test('recovery is bounded and cannot replace the original game identity',async()=>{
 const f=fixture(),v=f.env.AUTH.get('account:owner'),r=await row(v,'wrong-account','f');r.cookie='fixture-existing-owner-cookie';await v.put(r);const calls=[];
 v.kernel=async p=>{calls.push(p.action);if(p.action==='game_login')throw Error('SearchRentalServerByName: 请先登录(10)');if(p.action==='refresh'){const e=Error('expired');e.detail={code:10};throw e}return {game_id:'different-player',client:{untrusted:true}}};
 const request=req('/api/phoenix/login',{login_token:r.login_token,server_code:'123',client_public_key:'fixture'});request.headers.set('X-Auth-Session','fixture');
 const result=await (await v.fetch(request)).json();assert.equal(result.ok,false);assert.match(result.error,/不同游戏账号/);assert.deepEqual(calls,['game_login','refresh','authenticate']);assert.deepEqual((await v.get(r.id)).client,r.client);
});

test('anonymous /api/new creates game-only capability without slow platform identity calls',async()=>{
 const f=fixture(),response=await worker.fetch(req('/api/new'),f.env);assert.equal(response.status,200);assert.ok((await response.text()).length>100);assert.equal(f.identityCalls,0);
 assert.equal((await worker.fetch(req('/api/accounts',null,'yy//'+'a'.repeat(64)),f.env)).status,401);
 assert.equal((await worker.fetch(req('/api/admin/accounts',null,'yy//'+'a'.repeat(64)),f.env)).status,403);
});

test('held game token routes to its true owner without platform ownership, then revoked token and old session fail',async()=>{
 const f=fixture();f.registry.data.set('user:tenant',{id:'tenant',username:'tenant',created:Date.now()});
 const v=f.env.AUTH.get('account:tenant'),r=await row(v,'one','a');let actualProviderCalls=0;v.kernel=async p=>{actualProviderCalls++;if(p.action==='transfer_start_type')return {data:'transfer-ok'};return successfulKernel(p)};
 assert.deepEqual(await resolveGameBearer(f.env,r.login_token),{owner:'tenant',id:r.id});assert.equal(actualProviderCalls,0,'route lookup must not connect a game account');
 const validation=await worker.fetch(req('/api/game-token/validate',{},r.login_token),f.env);const validated=await validation.json();assert.equal(validation.status,200);assert.equal(validated.scope,'game:connect');assert.equal(JSON.stringify(validated).includes(r.login_token),false);assert.equal(JSON.stringify(validated).includes('tenant'),false);assert.equal(actualProviderCalls,0,'validation must not start game login');
 const session=await (await worker.fetch(req('/api/new'),f.env)).text();
 const response=await worker.fetch(req('/api/phoenix/login',{login_token:r.login_token,server_code:'123456',client_public_key:'fixture-only'},session),f.env);
 assert.equal((await response.json()).success,true);assert.equal(actualProviderCalls,1);
 const transfer=await worker.fetch(req('/api/phoenix/transfer_start_type?content=fixture',null,session),f.env);assert.equal((await transfer.json()).data,'transfer-ok');
 await v.fetch(req('/api/accounts',{action:'delete',id:r.id}));assert.equal(await resolveGameBearer(f.env,r.login_token),null);
 const denied=await worker.fetch(req('/api/phoenix/login',{login_token:r.login_token,server_code:'123456',client_public_key:'fixture'},session),f.env);assert.equal(denied.status,401);assert.equal(actualProviderCalls,2);
 const old=await worker.fetch(req('/api/phoenix/transfer_start_type?content=fixture',null,session),f.env);assert.equal((await old.json()).success,false);assert.equal(actualProviderCalls,2);
 const persisted=JSON.stringify([...f.instances.values()].flatMap(x=>[...x.ctx.data]));assert.equal(persisted.includes(r.login_token),false,'routing and vault storage must not expose plaintext bearer');
});

test('unknown token does not invoke official login or grant a management identity',async()=>{
 const f=fixture(),v=f.env.AUTH.get('account:owner');let called=false;v.kernel=async()=>{called=true;throw Error('must not run')};
 const session=await (await worker.fetch(req('/api/new'),f.env)).text();
 const result=await worker.fetch(req('/api/phoenix/login',{login_token:'yy//'+'b'.repeat(64),server_code:'1',client_public_key:'fixture'},session),f.env);assert.equal(result.status,401);assert.equal(called,false);
 const untrusted=await worker.fetch(req('/__bearer-directory',{action:'put',kind:'token',key:'a'.repeat(64),value:{owner:'owner'}}),f.env);assert.equal(untrusted.status,404);
});

test('game bearer discovery is read-only, account-bound and revoked immediately',async()=>{
 const f=fixture(),v=f.env.AUTH.get('account:owner'),r=await row(v,'discovery','j'),calls=[];
 v.kernel=async p=>{calls.push(p);assert.equal(p.action,'room_discover');return {ok:true,complete:true,candidates:[{id:'lobby:4691202525342713927',type:'lobby',room_id:'4691202525342713927',target:'LobbyGame:room_id=4691202525342713927'}],sources:[]}};
 let response=await worker.fetch(req('/api/game/discover',{keyword:'#11451'},r.login_token),f.env),data=await response.json();assert.equal(response.status,200);assert.equal(data.ok,true);assert.equal(calls.length,1);assert.equal(calls[0].keyword,'#11451');assert.deepEqual(calls[0].client,r.client);assert.equal(JSON.stringify(data).includes(r.login_token),false);
 assert.equal((await worker.fetch(req('/api/game/discover',{keyword:'x'},'yy//'+'k'.repeat(64)),f.env)).status,401);assert.equal(calls.length,1);
 await v.fetch(req('/api/accounts',{action:'delete',id:r.id}));response=await worker.fetch(req('/api/game/discover',{keyword:'#11451'},r.login_token),f.env);assert.equal(response.status,401);assert.equal(calls.length,1);
});

test('delegated discovery requires its exact current bearer and remains scoped to the grant',async()=>{
 const f=fixture();f.registry.data.set('user:tenant',{id:'tenant',username:'tenant',created:Date.now()});
 const v=f.env.AUTH.get('account:tenant'),r=await row(v,'delegated-discovery','l'),other=await row(v,'other-discovery','m');let calls=0;v.kernel=async p=>{assert.equal(p.action,'room_discover');assert.deepEqual(p.client,r.client);calls++;return {ok:true,candidates:[],sources:[],complete:true}};
 const grant=await (await worker.fetch(req('/api/admin/accounts',{user_id:'tenant',id:r.id,action:'token'},f.env.CONSOLE_TOKEN),f.env)).json();assert.ok(grant.auth_url?.includes('/delegated/'));
 const path=new URL(grant.auth_url).pathname+'/api/game/discover';
 assert.equal((await worker.fetch(req(path,{keyword:'11451'}),f.env)).status,401);assert.equal(calls,0);
 assert.equal((await worker.fetch(req(path,{keyword:'11451'},other.login_token),f.env)).status,403);assert.equal(calls,0);
 const result=await worker.fetch(req(path,{keyword:'11451'},r.login_token),f.env);assert.equal(result.status,200);assert.equal((await result.json()).ok,true);assert.equal(calls,1);
 await v.fetch(req('/api/accounts',{action:'delete',id:r.id}));assert.equal((await worker.fetch(req(path,{keyword:'11451'},r.login_token),f.env)).status,403);assert.equal(calls,1);
});

test('local TanLobby login reuses held account credentials and exact RID without rental login or replay',async()=>{
 const f=fixture(),v=f.env.AUTH.get('account:owner'),r=await row(v,'local-login','n'),calls=[];
 v.kernel=async p=>{calls.push(p);assert.equal(p.action,'tan_lobby_login');assert.deepEqual(p.client,r.client);assert.equal(p.room_id,'12345');return {client:r.client,engine_version:'e',patch_version:'p',result:{user_unique_id:123,user_player_name:'Own',room_owner_id:456,raknet_server_address:'127.0.0.1:19132',signaling_server_address:'127.0.0.1:1234',raknet_rand:'000aff',encrypt_key_bytes:'0102',room_mod_encrypt_key:['ff00']}}};
 const session=await (await worker.fetch(req('/api/new'),f.env)).text(),path='/api/phoenix/tan_lobby_login';
 let response=await worker.fetch(req(path,{login_token:r.login_token,room_id:'12345'},session),f.env),data=await response.json();assert.equal(response.status,200);assert.equal(data.success,true);assert.equal(data.raknet_rand,'000aff');assert.deepEqual(data.room_mod_encrypt_key,['ff00']);assert.equal(calls.length,1);
 v.kernel=async()=>{calls.push('failed');throw Error('selected local room not found')};response=await worker.fetch(req(path,{login_token:r.login_token,room_id:'12345'},session),f.env);data=await response.json();assert.equal(data.success,false);assert.match(data.error_info,/selected local room/);assert.equal(calls.length,2,'failed local auth must not replay');
 await v.fetch(req('/api/accounts',{action:'delete',id:r.id}));assert.equal((await worker.fetch(req(path,{login_token:r.login_token,room_id:'12345'},session),f.env)).status,401);assert.equal(calls.length,2);
});

test('negative lookup cache prevents repeated vault scans and indexing a new token clears it immediately',async()=>{
 const f=fixture(),raw='yy//'+'c'.repeat(64);assert.equal(await resolveGameBearer(f.env,raw),null);const scans=f.identityCalls;
 assert.equal(await resolveGameBearer(f.env,raw),null);assert.equal(f.identityCalls,scans);
 const v=f.env.AUTH.get('account:tenant'),r=await row(v,'new-account','c');await indexGameToken(f.env,'tenant',r);
 assert.deepEqual(await resolveGameBearer(f.env,raw),{owner:'tenant',id:r.id});assert.equal(f.identityCalls,scans);
 assert.equal((await bearerDirectory(f.env,'get','negative',await hash(raw))).value,null);
});

test('legacy migration is bounded and single-flight while expired session and negative routes are swept',async()=>{
 const f=fixture(),client=await hash('fixture-origin');
 for(let n=0;n<4;n++)await bearerDirectory(f.env,'migration_start','migration',String(n).padStart(64,'0'),{client_key:client});
 await assert.rejects(bearerDirectory(f.env,'migration_start','migration','4'.padStart(64,'0'),{client_key:client}));
 await assert.rejects(bearerDirectory(f.env,'migration_start','migration','0'.repeat(64),{client_key:client}));
 await bearerDirectory(f.env,'migration_end','migration','0'.repeat(64));await bearerDirectory(f.env,'migration_start','migration','4'.padStart(64,'0'),{client_key:client});
 const directory=f.env.AUTH.get('account:game-bearer-directory');
 await directory.ctx.storage.put('route:session:old',{owner:'tenant',expires:Date.now()-1});await directory.ctx.storage.put('route:negative:old',{expires:Date.now()-1});await directory.ctx.storage.put('route:session:fresh',{owner:'tenant',expires:Date.now()+60000});
 await directory.refreshAll();assert.equal(await directory.ctx.storage.get('route:session:old'),undefined);assert.equal(await directory.ctx.storage.get('route:negative:old'),undefined);assert.ok(await directory.ctx.storage.get('route:session:fresh'));
});

test('slow account refresh leaves account reads and another account game login available, serializes same-account writes',async()=>{
 const f=fixture(),v=f.env.AUTH.get('account:owner');const first=await row(v,'first','a'),second=await row(v,'second','b');let release,entered;
 const gate=new Promise(r=>release=r),started=new Promise(r=>entered=r);v.kernel=async p=>{if(p.action==='refresh'){entered();await gate;return {client:p.client}}return successfulKernel(p)};
 const refresh=v.fetch(req('/api/accounts',{action:'refresh',id:first.id}));await started;
 const timeout=()=>new Promise((_,reject)=>setTimeout(()=>reject(Error('read or independent login blocked by unrelated provider I/O')),250));
 const listed=await Promise.race([v.fetch(req('/api/accounts')),timeout()]);assert.equal((await listed.json()).accounts.length,2);
 const gameReq=req('/api/phoenix/login',{login_token:second.login_token,server_code:'1',client_public_key:'fixture'});gameReq.headers.set('X-Auth-Session','test-nonce');
 const game=await Promise.race([v.fetch(gameReq),timeout()]);assert.equal((await game.json()).success,true);
 let deleted=false;const deletion=v.fetch(req('/api/accounts',{action:'delete',id:first.id})).then(r=>{deleted=true;return r});await new Promise(r=>setTimeout(r,10));assert.equal(deleted,false);
 release();assert.equal((await refresh).status,200);assert.equal((await deletion).status,200);assert.equal(await v.get(first.id),null);
});
