import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import vm from 'node:vm';
import {identityHandle} from '../cloudflare/identity.mjs';

// Exercise the real worker/vault code; only the external NetEase provider is
// replaced because its WASM import belongs to Cloudflare, not Node's loader.
let source=await readFile(new URL('./worker.mjs',import.meta.url),'utf8');
source=source.replace("import {runKernel} from './runtime.mjs';",'const runKernel=async()=>{throw Error("provider must be supplied by fixture")};');
source=source.replace(/from '(\.[^']+)'/g,(_,path)=>'from '+JSON.stringify(new URL(path,import.meta.url).href));
const {default:worker,AuthVault}=await import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
function storage(){const data=new Map();const s={get:async k=>structuredClone(data.get(k)),put:async(k,v)=>data.set(k,structuredClone(v)),delete:async ks=>{for(const k of Array.isArray(ks)?ks:[ks])data.delete(k)},list:async(o={})=>new Map([...data].filter(([k])=>k.startsWith(o.prefix||''))),transaction:async f=>f(s),setAlarm:async()=>{}};return {data,storage:s}}
function fixture(){const registry=storage(),letters=[],instances=new Map();const env={AUTH_VAULT_KEY:'11'.repeat(32),CONSOLE_TOKEN:'fixture-project-owner-key',RELAY:{idFromName:n=>n,get:n=>({fetch:async req=>{if(n==='linux-server'){letters.push(await req.json());return Response.json({ok:true})}return identityHandle(req,registry,env)}})},AUTH:{idFromName:n=>n,get:n=>{if(!instances.has(n)){const ctx=storage();instances.set(n,new AuthVault(ctx,env))}return instances.get(n)}}};return {env,registry,letters,instances}}
const req=(action,body={},admin=false)=>new Request('https://auth/api/accounts',{method:'POST',headers:{'Content-Type':'application/json',...(admin?{'X-YouYuan-Project-Admin':'true'}:{})},body:JSON.stringify({action,...body})});

test('expired session recovers original SDK identity without rotating token or creating accounts',async()=>{
 const v=new AuthVault(storage(),fixture().env), row={id:'recover',game_id:'same',cookie:'private-sdk-cookie',client:{old:true},login_token:'yy//unchanged',keep_alive:true};await v.put(row);const actions=[];
 v.kernel=async p=>{actions.push(p.action);if(p.action==='refresh'){const e=Error('请先登录');e.detail={code:10};throw e;}assert.equal(p.cookie,row.cookie);return {game_id:'same',nickname:'recovered',client:{new:true}};};
 const response=await v.fetch(req('refresh',{id:row.id}));assert.equal(response.status,200);const body=await response.json();assert.equal(body.account.status,'ready');assert.deepEqual(actions,['refresh','authenticate']);const saved=await v.get(row.id);assert.equal(saved.login_token,row.login_token);assert.deepEqual(saved.client,{new:true});assert.equal((await v.rows()).length,1);assert.equal(JSON.stringify(body).includes(row.cookie),false);
 v.kernel=async p=>{if(p.action==='refresh'){const e=Error('请先登录');e.detail={code:10};throw e;}return {game_id:'different',client:{wrong:true}};};
 assert.equal((await v.fetch(req('refresh',{id:row.id}))).status,400);assert.deepEqual((await v.get(row.id)).client,{new:true});
 let attempts=0;v.kernel=async()=>{attempts++;const e=Error('网络失败');e.detail={code:500};throw e;};assert.equal((await v.fetch(req('refresh',{id:row.id}))).status,400);assert.equal(attempts,1);
 // The current Go kernel wraps the provider code in its error text without
 // exposing a typed code. Exercise that production shape, not just fixtures.
 const legacyActions=[];v.kernel=async p=>{legacyActions.push(p.action);if(p.action==='refresh')throw Error('刷新失败 (code=10): 请先登录');return {game_id:'same',client:{legacyRecovered:true}};};assert.equal((await v.fetch(req('refresh',{id:row.id}))).status,200);assert.deepEqual(legacyActions,['refresh','authenticate']);assert.deepEqual((await v.get(row.id)).client,{legacyRecovered:true});
});

test('reauthenticate preserves token and identity, rejects replacement and delegated admin',async()=>{
 const f=fixture(),v=new AuthVault(storage(),f.env);const row={id:'original',game_id:'game-one',name:'original',nickname:'old',login_token:'yy//private-original',keep_alive:true};await v.put(row);
 v.kernel=async input=>{assert.equal(input.action,'authenticate');return {game_id:'game-two',nickname:'other',client:{secret:'new'}}};
 const cookie='fixture-login-cookie-for-test';assert.equal((await v.fetch(req('reauthenticate',{id:row.id,cookie}))).status,409);assert.equal((await v.get(row.id)).login_token,row.login_token);assert.equal((await v.get(row.id)).nickname,'old');
 assert.equal((await v.fetch(req('reauthenticate',{id:row.id,cookie},true))).status,403);
 v.kernel=async()=>({game_id:'game-one',nickname:'same',client:{secret:'new'}});const ok=await (await v.fetch(req('reauthenticate',{id:row.id,cookie}))).json();assert.equal(ok.token_preserved,true);const saved=await v.get(row.id);assert.equal(saved.login_token,row.login_token);assert.equal(saved.nickname,'same');assert.equal(saved.status,'ready');assert.equal(JSON.stringify(ok).includes(cookie),false);assert.equal(JSON.stringify(ok).includes(row.login_token),false);
});

test('browser registration sends mailbox and OTP to the actual shared registry, then login succeeds',async()=>{
 const f=fixture(),html=await readFile(new URL('./public/index.html',import.meta.url),'utf8');
 const elements=new Map(),sessionStore=new Map(),calls=[];
 const el=id=>{if(!elements.has(id))elements.set(id,{value:'',hidden:false,disabled:false,textContent:'',required:false,reportValidity:()=>true,append(){},options:[],classList:{toggle(){}},setAttribute(){}});return elements.get(id)};
 const context=vm.createContext({AbortController,setTimeout,clearTimeout,document:{getElementById:el},sessionStorage:{getItem:k=>sessionStore.get(k),setItem:(k,v)=>sessionStore.set(k,v),removeItem:k=>sessionStore.delete(k)},setInterval:()=>1,clearInterval(){},Date,URLSearchParams,encodeURIComponent,fetch:async(path,opts)=>{calls.push({path,opts});if(path.startsWith('/api/identity/'))return identityHandle(new Request('https://identity/__identity/'+path.split('/').pop(),opts),f.registry,f.env);if(path.startsWith('/api/admin/'))return Response.json({ok:false,error:'not owner'},{status:403});return Response.json({ok:true,accounts:[],pending:[]})}});
 vm.runInContext(html.match(/<script>([\s\S]*?)<\/script>/)[1],context);
 el('username').value='registration_qa';el('platform-email').value='123456789@qq.com';el('password').value='fixture-password-12345';
 vm.runInContext("identityMode('register')",context);assert.equal(el('platform-email').required,true);
 await vm.runInContext('identitySendCode()',context);assert.equal(f.letters.length,1);assert.equal(el('identity-send-code').disabled,true);
 el('platform-code').value=f.letters[0].code;
 await vm.runInContext("identity('register')",context);
 assert.ok(sessionStore.get('yy-auth-device'));assert.equal(el('workspace').hidden,false);assert.equal(el('password').value,'');
 const submitted=JSON.parse(calls.find(c=>c.path==='/api/identity/register').opts.body);assert.equal(submitted.email_code,f.letters[0].code);assert.equal(submitted.qq,undefined);
 const before=f.registry.data.size;const repeat=await identityHandle(new Request('https://identity/__identity/register',{method:'POST',body:JSON.stringify(submitted)}),f.registry,f.env);assert.equal(repeat.status,409);assert.equal([...f.registry.data.keys()].filter(k=>k.startsWith('user:')).length,1);
 assert.ok(f.registry.data.size>=before);
 el('password').value='fixture-password-12345';await vm.runInContext("identity('login')",context);assert.ok(sessionStore.get('yy-auth-session'));
});

test('guest security verification resumes the same encrypted device and consumes pending registration once',async()=>{
 const f=fixture(),ctx=storage(),v=new AuthVault(ctx,f.env);let devices=0,attempts=0;
 v.kernel=async input=>{if(input.action==='device_create'){devices++;return {device:{id:'private-device-secret'}}}if(input.action==='guest_login'){attempts++;assert.equal(input.device.id,'private-device-secret');if(attempts===1){const e=Error('本人验证');e.detail={verification_required:true,verification_url:'https://official.example/verify',code:1351};throw e}return {cookie:'private-cookie-'.repeat(4)}}if(input.action==='authenticate')return {client:{token:'private-client-secret'},game_id:'game-fixture',nickname:'guest'};if(input.action==='nickname')return {client:input.client,game_id:'game-fixture',nickname:input.nickname};throw Error(input.action)};
 const failed=await v.fetch(req('register_guest',{name:'qa',nickname:'测试账号',allow_project_admin:true}));const wait=await failed.json();assert.equal(failed.status,400);assert.equal(wait.verification_required,true);assert.ok(wait.pending_id);assert.equal((await v.rows()).length,0);
 assert.equal(JSON.stringify([...ctx.data]).includes('private-device-secret'),false);
 const list=await (await v.fetch(new Request('https://auth/api/accounts'))).json();assert.equal(list.pending.length,1);assert.equal(JSON.stringify(list).includes('private-device-secret'),false);
 const completed=await (await v.fetch(req('resume_guest',{id:wait.pending_id}))).json();assert.equal(completed.ok,true);assert.equal(devices,1);assert.equal(completed.account.allow_project_admin,true);
 assert.equal((await v.fetch(req('resume_guest',{id:wait.pending_id}))).status,410);
 const read=await (await v.fetch(req('token',{id:completed.account.id}))).json();assert.equal(read.login_token,completed.login_token);
});

test('project-managed administration includes legacy accounts; user cannot disable access',async()=>{
 const f=fixture(),v=new AuthVault(storage(),f.env);
 const row={id:'granted',name:'shared',game_id:'g',nickname:'guest',login_token:'yy//fixture-game-token',allow_project_admin:true};await v.put(row);await v.put({...row,id:'private',allow_project_admin:false});
 const listed=await (await v.fetch(new Request('https://auth/api/accounts',{headers:{'X-YouYuan-Project-Admin':'true'}}))).json();assert.deepEqual(listed.accounts.map(x=>x.id),['granted','private']);assert.equal(listed.accounts.every(x=>x.allow_project_admin&&x.admin_access_policy==='project_managed'),true);assert.equal(JSON.stringify(listed).includes(row.login_token),false);
 assert.equal((await v.fetch(req('token',{id:'private'},true))).status,200);
 assert.equal((await v.fetch(req('configure',{id:'granted',allow_project_admin:false},true))).status,400);
 assert.equal((await v.fetch(req('configure',{id:'granted',allow_project_admin:false}))).status,400);await v.fetch(req('delete',{id:'granted'}));
 assert.equal((await v.fetch(req('token',{id:'granted'},true))).status,404);
});

test('public worker strips forged admin headers and rejects cross-user administration',async()=>{
 const f=fixture(),own=f.env.AUTH.get('account:owner');await own.put({id:'personal',name:'personal',game_id:'g',login_token:'yy//private',allow_project_admin:false});
 const fake=await worker.fetch(new Request('https://auth/api/accounts',{headers:{Authorization:'Bearer '+f.env.CONSOLE_TOKEN,'X-YouYuan-Project-Admin':'true'}}),f.env);assert.equal((await fake.json()).accounts.length,1);
 const noAuth=await worker.fetch(new Request('https://auth/api/admin/accounts',{headers:{'X-YouYuan-Project-Admin':'true'}}),f.env);assert.equal(noAuth.status,403);
 f.registry.data.set('user:tenant',{id:'tenant',username:'tenant_qa',created:Date.now(),salt:'s',password_hash:'unused'});
 const privateVault=f.env.AUTH.get('account:tenant');await privateVault.put({id:'target',name:'private',game_id:'g',login_token:'yy//other-private',allow_project_admin:false});
 const denied=await worker.fetch(new Request('https://auth/api/admin/accounts',{method:'POST',headers:{Authorization:'Bearer '+f.env.CONSOLE_TOKEN},body:JSON.stringify({user_id:'tenant',id:'target',action:'token'})}),f.env);assert.equal(denied.status,200);const managed=await denied.json();assert.equal(managed.login_token,'yy//other-private');
});

test('admin credentials establish a scoped game session; other users, other accounts and revocation are rejected',async()=>{
 const f=fixture();f.registry.data.set('user:tenant',{id:'tenant',username:'tenant_qa',created:Date.now()});
 const v=f.env.AUTH.get('account:tenant'),row={id:'shared',name:'shared',game_id:'g',login_token:'yy//fixture-shared',allow_project_admin:true,client:{},last_refresh:Date.now()};
 await v.put(row);
 const {hash}=await import('./vault.mjs');await v.ctx.storage.put('token:'+await hash(row.login_token),row.id);
 const r=await worker.fetch(new Request('https://auth/api/admin/accounts',{method:'POST',headers:{Authorization:'Bearer '+f.env.CONSOLE_TOKEN},body:JSON.stringify({user_id:'tenant',id:'shared',action:'token'})}),f.env);assert.equal(r.status,200);const issued=await r.json();assert.ok(issued.auth_url.includes('/delegated/'));assert.equal(issued.login_token,row.login_token);
 assert.equal((await worker.fetch(new Request(issued.auth_url+'/api/new'),f.env)).status,401);
 const newSession=await worker.fetch(new Request(issued.auth_url+'/api/new',{headers:{'X-YouYuan-Device-Key':f.env.CONSOLE_TOKEN}}),f.env);assert.equal(newSession.status,200);
 const bearer=await newSession.text();v.kernel=async input=>{assert.equal(input.action,'game_login');return {client:{},skin_info:{ItemID:'123',SkinDownloadURL:'https://provider.test/skin.zip',SkinIsSlim:true},result:{UID:'g',EngineVersion:'e',PatchVersion:'p',IP:'127.0.0.1:19132',ChainInfo:'fixture',BotComponent:{123:0}}}};
 const login=await worker.fetch(new Request(issued.auth_url+'/api/phoenix/login',{method:'POST',headers:{Authorization:'Bearer '+bearer},body:JSON.stringify({login_token:row.login_token,server_code:'DomainSID:test',client_public_key:'fixture-key'})}),f.env);const loginBody=await login.json();assert.equal(loginBody.success,true);assert.deepEqual(loginBody.skin_info,{entity_id:'123',res_url:'https://provider.test/skin.zip',is_slim:true});assert.deepEqual(loginBody.outfit_info,{123:0});
 await (await v.fetch(req('delete',{id:row.id}))).json();
 assert.equal((await worker.fetch(new Request(issued.auth_url+'/api/new',{headers:{'X-YouYuan-Device-Key':f.env.CONSOLE_TOKEN}}),f.env)).status,403);
 assert.equal((await worker.fetch(new Request(issued.auth_url+'/api/phoenix/login',{method:'POST',headers:{Authorization:'Bearer '+bearer},body:JSON.stringify({login_token:row.login_token})}),f.env)).status,403);
});

test('4399 channel joins login, game authenticate and nickname; no plaintext password retained',async()=>{
 const f=fixture(),ctx=storage(),v=new AuthVault(ctx,f.env),actions=[];
 v.kernel=async input=>{actions.push(input.action);if(input.action==='device_4399_create')return {device_4399:{state:'secret-device'}};if(input.action==='login_4399'){assert.equal(input.username,'own4399');return {cookie:'fixture-channel-cookie-'.repeat(3)}}if(input.action==='authenticate')return {client:{session:'fixture'},game_id:'game4399',nickname:''};if(input.action==='nickname')return {client:input.client,game_id:'game4399',nickname:input.nickname};throw Error(input.action)};
 const result=await (await v.fetch(req('login_4399',{username:'own4399',password:'never-store-password',nickname:'自己的机器人',allow_project_admin:false}))).json();
 assert.equal(result.ok,true);assert.equal(result.account.channel,'4399');assert.equal(result.account.allow_project_admin,true);assert.deepEqual(actions,['device_4399_create','login_4399','authenticate','nickname']);assert.match(result.login_token,/^yy\/\//);
 assert.equal(JSON.stringify(await v.rows()).includes('never-store-password'),false);assert.equal((await ctx.storage.list({prefix:'pending:'})).size,0);
});
test('4399 manual captcha keeps a pending device; challenge is not counted as successful registration',async()=>{
 const f=fixture(),ctx=storage(),v=new AuthVault(ctx,f.env);let devices=0,attempts=0;
 v.kernel=async input=>{if(input.action==='device_4399_create'){devices++;return {device_4399:{state:'same-device'}}}if(input.action==='login_4399'){attempts++;assert.equal(input.device_4399.state,'same-device');if(attempts===1){const e=Error('请输入本人验证码');e.detail={verification_required:true,verification_kind:'captcha_4399',captcha_id:'c1',captcha_url:'https://ptlogin.4399.com/captcha'};throw e}assert.equal(input.captcha,'1234');return {cookie:'cookie4399'.repeat(5)}}if(input.action==='authenticate')return {client:{},game_id:'captcha-game',nickname:'existing'};throw Error(input.action)};
 const waiting=await (await v.fetch(req('login_4399',{username:'own',password:'private-password'}))).json();assert.equal(waiting.ok,false);assert.ok(waiting.pending_id);assert.equal(waiting.verification_kind,'captcha_4399');assert.equal((await v.rows()).length,0);
 assert.equal(JSON.stringify([...ctx.data]).includes('private-password'),false);
 const result=await (await v.fetch(req('resume_4399',{id:waiting.pending_id,password:'private-password',captcha:'1234',captcha_id:'c1'}))).json();assert.equal(result.ok,true);assert.equal(devices,1);
});
test('batch creates ten distinct accounts only after explicit advancement; verification pauses and preserves state',async()=>{
 const f=fixture(),ctx=storage(),v=new AuthVault(ctx,f.env);let devices=0,blocked=true,guestCalls=0;
 v.kernel=async input=>{if(input.action==='device_create')return {device:{id:String(++devices)}};if(input.action==='guest_login'){guestCalls++;if(blocked){blocked=false;const e=Error('本人短信验证');e.detail={verification_required:true,verification_url:'https://official.example/verify'};throw e}return {cookie:'fixture-cookie-for-device-'+input.device.id}}if(input.action==='authenticate')return {client:{id:input.cookie},game_id:input.cookie,nickname:'default'};if(input.action==='nickname')return {client:input.client,game_id:input.client.id,nickname:input.nickname};throw Error(input.action)};
 const created=await (await v.fetch(req('batch_create',{count:10,nickname_prefix:'测试机器人'}))).json();assert.equal(created.ok,true);assert.equal(devices,0);assert.equal(created.official_sms_reuse_guaranteed,false);
 const id=created.batch.id;let result=await (await v.fetch(req('batch_next',{id}))).json();assert.equal(result.batch.state,'waiting_verification');assert.equal(result.batch.completed,0);assert.equal(devices,1);
 await v.fetch(req('batch_next',{id}));assert.equal(guestCalls,1);
 await ctx.storage.put('batch-rate',0);result=await (await v.fetch(req('batch_next',{id,verification_completed:true}))).json();assert.equal(result.batch.completed,1);assert.equal(devices,1);
 for(let i=0;i<9;i++){await ctx.storage.put('batch-rate',0);result=await (await v.fetch(req('batch_next',{id}))).json()}
 assert.equal(result.batch.completed,10);assert.equal(result.batch.state,'completed');assert.equal(new Set((await v.rows()).map(x=>x.game_id)).size,10);assert.equal(devices,10);
 await ctx.storage.put('batch-rate',0);await v.fetch(req('batch_next',{id}));assert.equal(devices,10);
});
test('batch count validation and cancellation preserve already created credentials',async()=>{
 const f=fixture(),v=new AuthVault(storage(),f.env);assert.equal((await v.fetch(req('batch_create',{count:11,nickname_prefix:'测试机器人'}))).status,400);
 const job=await (await v.fetch(req('batch_create',{count:2,nickname_prefix:'测试机器人'}))).json();await v.put({id:'existing',name:'existing',game_id:'g',login_token:'yy//existing'});const stopped=await (await v.fetch(req('batch_cancel',{id:job.batch.id}))).json();assert.equal(stopped.batch.state,'cancelled');assert.equal((await v.rows()).length,1);
});
test('public registration page has a return tutorial, no optional admin switch or source download',async()=>{
 const html=await readFile(new URL('./public/index.html',import.meta.url),'utf8');assert.equal(html.includes('source.zip'),false);assert.equal(html.includes('id="allow-admin"'),false);assert.match(html,/4399渠道账号登录/);assert.match(html,/注册与接入教程/);assert.match(html,/官方游戏账号需要验证时暂停/);
});

test('official 4399 refusal keeps resumable task and complete challenge metadata',async()=>{
 const f=fixture(),v=new AuthVault(storage(),f.env);v.kernel=async input=>{if(input.action==='device_4399_create')return {device_4399:{id:'fixture-device'}};const e=new Error('official wait');e.detail={retryable:true,provider_status:202,verification_kind:'captcha_4399',captcha_id:'fixture-captcha',captcha_url:'https://ptlogin.4399.com/ptlogin/captcha.do?captchaId=fixture'};throw e};
 const result=await (await v.fetch(req('login_4399',{username:'fixture',password:'not-a-real-secret',nickname:'测试昵称'}))).json();assert.equal(result.ok,false);assert.ok(result.pending_id);
 const list=await (await v.fetch(new Request('https://auth/api/accounts'))).json();assert.equal(list.pending.length,1);assert.equal(list.pending[0].provider_status,202);assert.equal(list.pending[0].captcha_id,'fixture-captcha');assert.equal(list.pending[0].last_error,'official wait');assert.equal(JSON.stringify(list).includes('not-a-real-secret'),false);
});
test('interactive ten-account batch pauses for naming and does not create next account early',async()=>{
 const f=fixture(),ctx=storage(),v=new AuthVault(ctx,f.env);let created=0;v.create=async()=>{const row={id:'created-'+(++created),name:'temporary',game_id:'g'+created,login_token:'yy//fixture',client:{}};await v.put(row);return Response.json({ok:true,account:row})};v.kernel=async input=>({client:input.client,nickname:input.nickname});
 const job=await (await v.fetch(req('batch_create',{count:10,nickname_prefix:'测试机器人',require_nickname:true}))).json();const id=job.batch.id;
 const first=await (await v.fetch(req('batch_next',{id}))).json();assert.equal(first.batch.state,'waiting_nickname');assert.equal(first.batch.completed,0);assert.equal(created,1);
 await v.fetch(req('batch_next',{id}));assert.equal(created,1);
 const named=await (await v.fetch(req('batch_name',{id,nickname:'正式名字'}))).json();assert.equal(named.batch.completed,1);assert.equal(named.batch.state,'ready');assert.equal((await v.get('created-1')).nickname,'正式名字');
 assert.equal((await v.fetch(req('batch_name',{id,nickname:'重复命名'}))).status,400);
});



