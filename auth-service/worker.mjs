import {officialPortrait} from './account-profile.mjs';
import {bearerDirectory,indexGameToken,resolveGameBearer} from './game-bearer.mjs';
import {runKernel} from './runtime.mjs';
import {identityResolve,identityRoute} from '../cloudflare/identity.mjs';
import {seal,open,hash,token} from './vault.mjs';
import {normalizeGameServer} from './server-code.mjs';
import {batchAccountAction} from './batch-accounts.mjs';
import {isPrivateDistributionPath} from '../cloudflare/private-distribution.mjs';
import {verifyTurnstile,turnstileRemoteip} from './turnstile.mjs';
const json=(data,status=200)=>Response.json(data,{status,headers:{'Cache-Control':'no-store'}});
const vault=(env,owner)=>env.AUTH.get(env.AUTH.idFromName('account:'+owner));
async function actor(request,env){const key=request.headers.get('X-YouYuan-Device-Key')||(request.headers.get('Authorization')||'').replace(/^Bearer /,'');if(!key)return null;if(env.DEVICE_TOKEN&&key===env.DEVICE_TOKEN||env.CONSOLE_TOKEN&&key===env.CONSOLE_TOKEN)return 'owner';for(const purpose of ['device','session']){const identity=await identityResolve(env,key,purpose);if(identity)return identity.user.legacy_owner?'owner':identity.user.id;}return null;}
const pack=value=>btoa(JSON.stringify(value)).replaceAll('+','-').replaceAll('/','_').replaceAll('=','');
const unpack=value=>JSON.parse(atob(value.replaceAll('-','+').replaceAll('_','/')));
async function delegateValid(env,grant){
 if(!grant||grant.expires<Date.now())return false;
 const people=await identityRoute(new Request('https://identity/api/identity/users',{headers:{Authorization:'Bearer '+env.CONSOLE_TOKEN}}),env);
 if(!people.ok||!(await people.json()).users.some(u=>u.id===grant.user_id))return false;
 const check=await vault(env,grant.owner).fetch(new Request('https://vault/__delegation/check',{method:'POST',body:JSON.stringify(grant)}));
 return check.ok;
}
export default {
 async fetch(request,env){
  const url=new URL(request.url);
  if(isPrivateDistributionPath(url.pathname))return json({ok:false,error:"未开放该资源"},404);
  let delegated=null;
  const delegatedPath=url.pathname.match(/^\/delegated\/([A-Za-z0-9_-]{1,4096})(\/api\/(?:new|phoenix\/.*|game\/discover))$/);
  if(delegatedPath){
   try{delegated=await open(env,'admin-delegation',unpack(delegatedPath[1]))}catch{return json({success:false,message:'授权认证地址无效'},401)}
   if(!await delegateValid(env,delegated))return json({success:false,message:'游戏账号授权已撤销、账号已注销或授权地址已到期'},403);
   url.pathname=delegatedPath[2];request=new Request(url,request);
  }
  if(url.pathname==='/health')return json({ok:true,version:'0.10.70',build:'link-tcp-20261019.4',product:'游园独立游戏认证',provider:'direct-netease',third_party_auth:false});
  if(url.pathname==='/api/turnstile-config')return json({ok:true,site_key:env.TURNSTILE_SITE_KEY||'',enabled:!!env.TURNSTILE_SECRET_KEY});
  if(url.pathname.startsWith('/api/identity/'))return identityRoute(request,env);
  if(url.pathname==='/'&&env.ASSETS)return env.ASSETS.fetch(request);
  if(url.pathname==='/api/game/discover'){
   if(request.method!=='POST')return json({ok:false,error:'POST required'},405);
   const raw=(request.headers.get('Authorization')||'').replace(/^Bearer /,'');
   try{
    const text=await request.text();if(text.length>4096)return json({ok:false,error:'请求过大'},413);
    const input=JSON.parse(text);if(typeof input.keyword!=='string'||input.keyword.length>256)return json({ok:false,error:'搜索内容无效'},400);
    const found=await resolveGameBearer(env,raw,request.headers.get('CF-Connecting-IP')||'unknown');if(!found)return json({ok:false,error:'游园游戏令牌不存在或已撤销'},401);
    if(delegated&&(found.owner!==delegated.owner||found.id!==delegated.id||await hash(raw)!==delegated.token_hash))return json({ok:false,error:'游戏令牌与授权认证地址不匹配'},403);
    return vault(env,found.owner).fetch(new Request('https://vault/__bearer/discover',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({digest:await hash(raw),keyword:input.keyword,offset:input.offset||0})}));
   }catch{return json({ok:false,error:'房间发现暂不可用，请稍后重试'},503)}
  }
  if(url.pathname==='/api/game-token/validate'){
   if(request.method!=='POST')return json({ok:false,error:'POST required'},405);
   const raw=(request.headers.get('Authorization')||'').replace(/^Bearer /,'');
   try{const found=await resolveGameBearer(env,raw,request.headers.get('CF-Connecting-IP')||'unknown');return found?json({ok:true,valid:true,scope:'game:connect',auth_url:url.origin}):json({ok:false,valid:false,error:'游园游戏令牌不存在或已撤销'},401);}
   catch{return json({ok:false,error:'游戏令牌路由暂不可用',code:'AUTH_ROUTING_UNAVAILABLE',retryable:true},503);}
  }
  if(url.pathname==='/api/new'){
   if(!delegated){const session={game_only:true,nonce:token(),expires:Date.now()+3600000};const sealed=await seal(env,'auth-session',session);return new Response(btoa(JSON.stringify(sealed)),{headers:{'Content-Type':'text/plain','Cache-Control':'no-store'}});}
   const owner=await actor(request,env);if(!owner)return json({success:false,message:'请先绑定游园账号设备密钥'},401);
   if(delegated&&owner!=='owner')return json({success:false,message:'此地址仅供项目所有者调用获授权账号'},403);
   const session={owner:delegated?delegated.owner:owner,nonce:token(),expires:delegated?Math.min(delegated.expires,Date.now()+86400000):Date.now()+86400000,...(delegated?{delegated}:{})};const sealed=await seal(env,'auth-session',session);
   return new Response(btoa(JSON.stringify(sealed)),{headers:{'Content-Type':'text/plain','Cache-Control':'no-store'}});
  }
  let owner;
  if(url.pathname.startsWith('/api/phoenix/')){
   const raw=(request.headers.get('Authorization')||'').replace(/^Bearer /,'');let session;
   try{session=await open(env,'auth-session',JSON.parse(atob(raw)))}catch{return json({success:false,message:'认证会话无效或过期'},401)}
   if(session.expires<Date.now())return json({success:false,message:'认证会话已过期'},401);
   if(session.delegated&&(!delegated||session.delegated.id!==delegated.id||session.delegated.owner!==delegated.owner||session.delegated.token_hash!==delegated.token_hash))return json({success:false,message:'授权会话与账号不匹配'},403);
   if(delegated&&!session.delegated)return json({success:false,message:'请在授权认证地址建立会话'},403);
   owner=session.owner;
   if(session.game_only){
    try{
     if(['/api/phoenix/login','/api/phoenix/tan_lobby_login'].includes(url.pathname)){
      if(request.method!=='POST')return json({success:false,message:'POST required'},405);
      const text=await request.clone().text();if(text.length>100000)return json({success:false,message:'请求过大'},413);let input;try{input=JSON.parse(text)}catch{return json({success:false,message:'JSON无效'},400);}
      const found=await resolveGameBearer(env,input.login_token,request.headers.get('CF-Connecting-IP')||'unknown');if(!found)return json({success:false,message:'游园游戏令牌不存在或已撤销'},401);owner=found.owner;
     }else{const route=await bearerDirectory(env,'get','session',session.nonce);if(!route.value||route.value.expires<Date.now())return json({success:false,message:'请先使用有效游园令牌完成服务器认证'},401);owner=route.value.owner;}
    }catch{return json({success:false,message:'游戏认证路由暂不可用，请稍后重试',code:'AUTH_ROUTING_UNAVAILABLE',retryable:true},503);}
   }
   const headers=new Headers(request.headers);headers.delete('X-YouYuan-Delegated-Account');headers.set('X-YouYuan-Vault-Owner',owner);headers.set('X-Auth-Session',session.nonce);if(session.delegated)headers.set('X-YouYuan-Delegated-Account',session.delegated.id);
   const response=await vault(env,owner).fetch(new Request(request,{headers}));
   if(session.game_only&&['/api/phoenix/login','/api/phoenix/tan_lobby_login'].includes(url.pathname)&&response.ok){const result=await response.clone().json();if(result.success)await bearerDirectory(env,'put','session',session.nonce,{owner,expires:session.expires});}
   return response;
  }
  if(url.pathname==='/api/admin/accounts'){
   if(await actor(request,env)!=='owner')return json({ok:false,error:'仅项目所有者可调用已授权游戏账号'},403);
   if(!['GET','POST'].includes(request.method))return json({ok:false,error:'GET/POST required'},405);
   let input={};if(request.method==='POST'){const text=await request.text();if(text.length>4096)return json({ok:false,error:'请求过大'},413);try{input=JSON.parse(text)}catch{return json({ok:false,error:'JSON无效'},400)}}
   const usersResponse=await identityRoute(new Request(url.origin+'/api/identity/users',{headers:{Authorization:request.headers.get('Authorization')||''}}),env);
   if(!usersResponse.ok)return usersResponse;const {users}=await usersResponse.json();
   const userId=String(input.user_id||url.searchParams.get('user_id')||'');
   if(!userId)return json({ok:true,users:users.map(u=>({id:u.id,username:u.username}))});
   const target=users.find(u=>u.id===userId);if(!target)return json({ok:false,error:'平台账号不存在或已注销'},404);
   if(request.method==='POST'&&!['token','refresh','servers','rooms','configure'].includes(input.action))return json({ok:false,error:'该管理员操作未开放'},400);
   const headers=new Headers({'Content-Type':'application/json','X-YouYuan-Project-Admin':'true'});
   const targetOwner=target.legacy_owner?'owner':target.id;
   headers.set('X-YouYuan-Vault-Owner',targetOwner);
   const result=await vault(env,targetOwner).fetch(new Request(url.origin+'/api/accounts',{method:request.method,headers,body:request.method==='POST'?JSON.stringify(input):undefined}));
   if(request.method==='POST'&&input.action==='token'&&result.ok){
    const data=await result.json();const grant={user_id:target.id,owner:targetOwner,id:input.id,token_hash:await hash(data.login_token),expires:Date.now()+86400000};
    const capability=pack(await seal(env,'admin-delegation',grant));
    return json({...data,auth_url:url.origin+'/delegated/'+capability,authorization_expires:grant.expires});
   }
   return result;
  }
  if(url.pathname!=='/api/accounts'&&url.pathname!=='/api/kernel-test')return json({ok:false,error:'未知认证接口'},404);
  owner=await actor(request,env);if(!owner)return json({ok:false,error:'请先登录游园账号'},401);
  if(url.pathname==='/api/kernel-test'){
   if(owner!=='owner'||request.method!=='POST')return json({ok:false,error:'仅用于所有者认证验收'},403);
   const body=await request.text();if(body.length>100000)return json({ok:false,error:'请求过大'},413);
   try{return json(await runKernel(JSON.parse(body)))}catch(error){return json({ok:false,error:'认证运行时：'+error.message},500)}
  }
  const headers=new Headers(request.headers);headers.delete('X-YouYuan-Project-Admin');headers.set('X-YouYuan-Vault-Owner',owner);
  return vault(env,owner).fetch(new Request(request,{headers}));
 }
};
function publicAccount(row){return{id:row.id,name:row.name,game_id:row.game_id,nickname:row.nickname,created:row.created,keep_alive:row.keep_alive,status:row.status,last_refresh:row.last_refresh||null,next_refresh:row.next_refresh||null,last_error:row.last_error||'',token_hint:row.login_token.slice(0,5)+'…',server_id:row.server_id||'',avatar:officialPortrait(row),avatar_source:officialPortrait(row)?'netease':'default',skin_item_id:row.skin_item_id||'',skin_body_known:row.skin_body_known===true,skin_slim:row.skin_slim===true,allow_project_admin:true,admin_access_policy:"project_managed",channel:row.channel||"netease"};}
export class AuthVault{
 constructor(ctx,env){this.ctx=ctx;this.env=env;this.serial=Promise.resolve();this.accountQueues=new Map();this.migrations=new Map();this.migrationRates=new Map();}
 fetch(request){const path=new URL(request.url).pathname;if(request.method==='GET'||path.startsWith('/__bearer')||path==='/__delegation/check'||path.startsWith('/api/phoenix/'))return this.handle(request);const result=this.serial.then(()=>this.handle(request));this.serial=result.catch(()=>{});return result;}
 accountSerial(id,fn){const prior=this.accountQueues.get(id)||Promise.resolve(),result=prior.catch(()=>{}).then(fn),tail=result.catch(()=>{});this.accountQueues.set(id,tail);tail.then(()=>{if(this.accountQueues.get(id)===tail)this.accountQueues.delete(id)});return result;}
 async alarmBy(at){const previous=this.ctx.storage.getAlarm?await this.ctx.storage.getAlarm():null;if(!previous||at<previous)await this.ctx.storage.setAlarm(at);}
 async get(id){const value=await this.ctx.storage.get('account:'+id);return value?open(this.env,'account:'+id,value):null;}
 async put(row){await this.ctx.storage.put('account:'+row.id,await seal(this.env,'account:'+row.id,row));const owner=await this.ctx.storage.get('__owner');if(owner)await indexGameToken(this.env,owner,row);}
 async rows(){const rows=await this.ctx.storage.list({prefix:'account:'});const out=[];for(const [key,value]of rows)out.push(await open(this.env,key,value));return out;}
 async kernel(input){const result=await runKernel(input);if(!result.ok){const error=new Error(result.error||'网易认证失败');error.detail={code:result.code,verification_required:result.verification_required,verification_url:result.verification_url,verification_kind:result.verification_kind,captcha_id:result.captcha_id,captcha_url:result.captcha_url,provider_status:result.provider_status,retryable:result.retryable,phase:result.phase};throw error;}return result;}
 async refreshSession(row){
  let state;
  try{state=await this.kernel({action:'refresh',client:row.client});}
  catch(error){
   // A game session may expire while the original SDK login is still valid.
   // Recover only that same identity, never create a replacement guest account.
   const sessionExpired=Number(error.detail?.code)===10||/^刷新失败 \(code=10\):/.test(error.message||'');
   if(!sessionExpired||!row.cookie)throw error;
   state=await this.kernel({action:'authenticate',cookie:row.cookie});
   if(String(state.game_id)!==String(row.game_id))throw new Error('恢复登录返回了不同游戏账号；原凭据未修改');
  }
  row.client=state.client;row.nickname=state.nickname||row.nickname;
  row.last_refresh=Date.now();row.next_refresh=Date.now()+1500000;
  row.status='ready';row.last_error='';row.failures=0;return state;
 }
 async create(input,pending=null){
  if((await this.rows()).length>=32)return json({ok:false,error:'每个游园账号最多32个游戏凭据'},409);
  const guest=input.action==='register_guest',channel4399=input.action==='login_4399';
  let cookie=pending?.cookie||input.cookie;
  if((guest||channel4399)&&!pending){
   const existing=await this.ctx.storage.list({prefix:'pending:'});let active=0;
   for(const [key,value]of existing){if(value.expires<Date.now())await this.ctx.storage.delete(key);else active++}
   if(active>=4)return json({ok:false,error:'请先完成或取消已有待验证账号'},409);
  }
  if(guest&&!pending){
   const generated=await this.kernel({action:'device_create'});
   pending={id:crypto.randomUUID(),expires:Date.now()+1800000,device:generated.device,name:input.name,nickname:input.nickname,allow_project_admin:true};
   await this.ctx.storage.put('pending:'+pending.id,{expires:pending.expires,sealed:await seal(this.env,'pending:'+pending.id,pending)});
  }
  if(channel4399&&!pending){
   const generated=await this.kernel({action:'device_4399_create'});
   pending={id:crypto.randomUUID(),expires:Date.now()+1800000,action:'login_4399',device_4399:generated.device_4399,name:input.name,nickname:input.nickname,username:input.username,allow_project_admin:true};
   await this.ctx.storage.put('pending:'+pending.id,{expires:pending.expires,sealed:await seal(this.env,'pending:'+pending.id,pending)});
  }
  try{
   if(input.action!=='import_cookie'&&!cookie){
    let logged;
    if(channel4399)logged=await this.kernel({action:'login_4399',device_4399:pending.device_4399,username:pending.username,password:input.password,captcha:input.captcha,captcha_id:input.captcha_id});
    else {const generated=guest?{device:pending.device}:await this.kernel({action:'device_create'});logged=await this.kernel({action:guest?'guest_login':'email_login',device:generated.device,email:input.email,password:input.password});}
    cookie=logged.cookie;
    if(pending){pending.cookie=cookie;await this.ctx.storage.put('pending:'+pending.id,{expires:pending.expires,sealed:await seal(this.env,'pending:'+pending.id,pending)})}
   }
   if(typeof cookie!=='string'||cookie.length<20||cookie.length>50000)return json({ok:false,error:'登录凭据格式无效'},400);
   let state=await this.kernel({action:'authenticate',cookie});
   if(guest||channel4399&&input.nickname)state=await this.kernel({action:'nickname',client:state.client,nickname:input.nickname});
   if((await this.rows()).some(row=>row.game_id===state.game_id)){if(pending)await this.ctx.storage.delete('pending:'+pending.id);return json({ok:false,error:'这个游戏账号已经添加，请刷新凭据或撤销后重新导入'},409)}
   const row={id:crypto.randomUUID(),name:String(input.name||state.nickname||'游戏账号').slice(0,48),cookie,client:state.client,login_token:'yy//'+token(),game_id:state.game_id,nickname:state.nickname,channel:channel4399?'4399':'netease',created:Date.now(),keep_alive:true,status:'ready',last_refresh:Date.now(),failures:0,allow_project_admin:true};
   await this.ctx.storage.transaction(async store=>{await store.put('account:'+row.id,await seal(this.env,'account:'+row.id,row));await store.put('token:'+await hash(row.login_token),row.id);if(pending)await store.delete('pending:'+pending.id)});
   const owner=await this.ctx.storage.get('__owner');if(owner)await indexGameToken(this.env,owner,row);
   await this.ctx.storage.setAlarm(Date.now()+1500000);
   return json({ok:true,account:publicAccount(row),login_token:row.login_token,auth_url:'https://auth.youyuanqi.dpdns.org'});
  }catch(error){if(pending){error.detail={...error.detail,pending_id:pending.id,pending_expires:pending.expires};error.message=error.message.split(' verify_url=')[0];pending.last_error=error.message;pending.verification={...error.detail};delete pending.verification.pending_id;delete pending.verification.pending_expires;await this.ctx.storage.put('pending:'+pending.id,{expires:pending.expires,sealed:await seal(this.env,'pending:'+pending.id,pending)})}throw error}
 }
 async handle(request){
  const url=new URL(request.url);
  if(isPrivateDistributionPath(url.pathname))return json({ok:false,error:"未开放该资源"},404);let input={};
  try{
   if(request.method!=='GET'){const text=await request.text();if(text.length>100000)return json({ok:false,error:'请求过大'},413);input=JSON.parse(text||'{}')}
   if(url.pathname==='/__bearer-directory'){
    if(!['token','session','negative','migration'].includes(input.kind)||!/^[a-f0-9]{64}$/.test(input.key||''))return json({ok:false,error:'无效路由键'},400);
    if(input.kind==='migration'){
     const now=Date.now();for(const [key,expires]of this.migrations)if(expires<=now)this.migrations.delete(key);
     if(input.action==='migration_end'){this.migrations.delete(input.key);return json({ok:true});}
     if(input.action!=='migration_start'||!/^[a-f0-9]{64}$/.test(input.value?.client_key||''))return json({ok:false},400);
     for(const [key,rate]of this.migrationRates)if(rate.expires<=now)this.migrationRates.delete(key);
     const key=input.value.client_key,rate=this.migrationRates.get(key)||{count:0,expires:now+60000};
     if(this.migrations.has(input.key)||this.migrations.size>=4||rate.count>=12||this.migrationRates.size>=1024&&!this.migrationRates.has(key))return json({ok:false,retryable:true,error:'旧令牌索引正在迁移，请稍后重试'},503);
     rate.count++;this.migrationRates.set(key,rate);this.migrations.set(input.key,now+30000);return json({ok:true});
    }
    const key='route:'+input.kind+':'+input.key;
    if(input.action==='get'){const value=await this.ctx.storage.get(key);if(value?.expires&&value.expires<=Date.now()){await this.ctx.storage.delete(key);return json({ok:true,value:null})}return json({ok:true,value:value||null});}
    if(input.action==='put'&&(input.value?.owner||input.kind==='negative'&&input.value?.expires)){
     await this.ctx.storage.put(key,input.value);if(input.kind==='token')await this.ctx.storage.delete('route:negative:'+input.key);
     if(input.value.expires)await this.alarmBy(input.value.expires+1000);return json({ok:true});
    }
    return json({ok:false,error:'无效路由操作'},400);
   }
   if(url.pathname==='/__bearer/discover'){
    if(!/^[a-f0-9]{64}$/.test(input.digest||''))return json({ok:false},400);
    const id=await this.ctx.storage.get('token:'+input.digest),row=id?await this.get(id):null;
    if(!row||await hash(row.login_token)!==input.digest)return json({ok:false,error:'游戏令牌已撤销'},401);
    return json(await this.kernel({action:'room_discover',client:row.client,keyword:input.keyword,offset:input.offset||0}));
   }
   if(url.pathname==='/__bearer/resolve'){
    if(!/^[a-f0-9]{64}$/.test(input.digest||''))return json({ok:false},400);
    const id=await this.ctx.storage.get('token:'+input.digest),row=id?await this.get(id):null;
    return row&&await hash(row.login_token)===input.digest?json({ok:true,id:row.id}):json({ok:false},404);
   }
   const owner=request.headers.get('X-YouYuan-Vault-Owner');if(owner)await this.ctx.storage.put('__owner',owner);
   if(url.pathname==='/__delegation/check'){
    const row=await this.get(String(input.id||''));return json({ok:!!row&&await hash(row.login_token)===input.token_hash},row&&await hash(row.login_token)===input.token_hash?200:403);
   }
   if(url.pathname.startsWith('/api/phoenix/'))return await this.phoenix(url,request,input);
   const admin=request.headers.get('X-YouYuan-Project-Admin')==='true';
   if(request.method==='GET'){
    const pending=[];if(!admin)for(const [key,value]of await this.ctx.storage.list({prefix:'pending:'})){if(value.expires<Date.now()){await this.ctx.storage.delete(key);continue}const p=await open(this.env,key,value.sealed);pending.push({id:p.id,name:p.name,nickname:p.nickname,expires:p.expires,action:p.action||'register_guest',last_error:p.last_error||'',...(p.verification||{})})}
    return json({ok:true,auth_version:'0.10.70',accounts:(await this.rows()).map(publicAccount),pending,batches:admin?[]:await batchAccountAction(this,{action:'batch_list'}),provider:'direct-netease',admin_access_policy:'project_managed'});
   }
   if(request.method!=='POST')return json({ok:false,error:'GET/POST required'},405);
   const action=input.action;
   if(['register_guest','login_email','login_4399','import_cookie','batch_create'].includes(action)){
    try{await verifyTurnstile(this.env,input,turnstileRemoteip(request))}catch(error){return json({ok:false,error:error.message},403)}
   }
   if(admin&&!['token','refresh','servers','rooms','configure'].includes(action))return json({ok:false,error:'管理员不能创建或更改用户授权'},403);
   if(!admin&&['batch_create','batch_next','batch_cancel','batch_name'].includes(action))return json(await batchAccountAction(this,input));
   if(action==='resume_guest'||action==='cancel_guest'||action==='resume_4399'){
    const id=String(input.id||''),saved=await this.ctx.storage.get('pending:'+id);
    if(!saved||saved.expires<Date.now()){if(saved)await this.ctx.storage.delete('pending:'+id);return json({ok:false,error:'待验证注册已失效，请重新创建'},410)}
    if(action==='cancel_guest'){await this.ctx.storage.delete('pending:'+id);return json({ok:true})}
    const rate=await this.ctx.storage.get('login-rate')||{at:0,n:0};if(Date.now()-rate.at>60000){rate.at=Date.now();rate.n=0}rate.n++;await this.ctx.storage.put('login-rate',rate);if(rate.n>5)return json({ok:false,error:'登录操作过于频繁，请一分钟后再试'},429);
    const p=await open(this.env,'pending:'+id,saved.sealed);
    if(p.action==='login_4399'){if(action!=='resume_4399'||!input.password)return json({ok:false,error:'请使用4399验证继续并重新输入密码'},400);return await this.create({...input,action:'login_4399',name:p.name,nickname:p.nickname,username:p.username},p);}
    return await this.create({action:'register_guest',name:p.name,nickname:p.nickname,allow_project_admin:true},p);
   }
   if(['login_email','import_cookie','register_guest','login_4399'].includes(action)){
    const rows=await this.rows();if(rows.length>=32)return json({ok:false,error:'每个游园账号最多32个游戏凭据'},409);
    if(action==='login_4399'&&(!String(input.username||'').trim()||!input.password))return json({ok:false,error:'请填写4399账号与密码；注册网站不会自动添加网易游戏凭据'},400);
    if(action==='login_email'&&(!/^.{1,128}@.{1,128}$/.test(input.email||'')||!input.password))return json({ok:false,error:'请填写网易邮箱和密码'},400);
    if(action==='register_guest'&&([...String(input.nickname||'')].length<2||[...String(input.nickname||'')].length>20))return json({ok:false,error:'请填写2–20字符的游戏昵称'},400);
    const rate=await this.ctx.storage.get('login-rate')||{at:0,n:0};if(Date.now()-rate.at>60000){rate.at=Date.now();rate.n=0}rate.n++;await this.ctx.storage.put('login-rate',rate);if(rate.n>5)return json({ok:false,error:'登录操作过于频繁，请一分钟后再试'},429);
    return await this.create(input);
   }
   return await this.accountSerial(String(input.id||''),async()=>{
   const row=await this.get(String(input.id||''));if(!row)return json({ok:false,error:'游戏凭据不存在'},404);
   
   if(action==='delete'){
    await this.ctx.storage.delete(['account:'+row.id,'token:'+await hash(row.login_token)]);return json({ok:true});
   }
   if(action==='token'){if(owner)await indexGameToken(this.env,owner,row);return json({ok:true,login_token:row.login_token,auth_url:'https://auth.youyuanqi.dpdns.org'});}
   if(action==='reauthenticate'){
    const rate=await this.ctx.storage.get('login-rate')||{at:0,n:0};if(Date.now()-rate.at>60000){rate.at=Date.now();rate.n=0}rate.n++;await this.ctx.storage.put('login-rate',rate);if(rate.n>5)return json({ok:false,error:'登录操作过于频繁，请一分钟后再试'},429);
    let cookie=input.cookie;
    if(!cookie){if(!input.email||!input.password)return json({ok:false,error:'请填写本人的网易邮箱与密码，或提供本人登录凭据'},400);const generated=await this.kernel({action:'device_create'});const logged=await this.kernel({action:'email_login',device:generated.device,email:input.email,password:input.password});cookie=logged.cookie}
    if(typeof cookie!=='string'||cookie.length<20||cookie.length>50000)return json({ok:false,error:'登录凭据格式无效'},400);
    const state=await this.kernel({action:'authenticate',cookie});if(String(state.game_id)!==String(row.game_id))return json({ok:false,error:'重新登录的游戏账号与原账号不同；原账号与令牌未修改'},409);
    row.cookie=cookie;row.client=state.client;row.nickname=state.nickname;row.last_refresh=Date.now();row.next_refresh=Date.now()+1500000;row.status='ready';row.last_error='';row.failures=0;await this.put(row);if(row.keep_alive)await this.ctx.storage.setAlarm(row.next_refresh);
    return json({ok:true,account:publicAccount(row),token_preserved:true});
   }
   if(action==='reissue'){
    const old=await hash(row.login_token);row.login_token='yy//'+token();await this.ctx.storage.transaction(async store=>{await store.put('account:'+row.id,await seal(this.env,'account:'+row.id,row));await store.put('token:'+await hash(row.login_token),row.id);await store.delete('token:'+old)});if(owner)await indexGameToken(this.env,owner,row);return json({ok:true,login_token:row.login_token});
   }
   if(action==='rooms'){const result=await this.kernel({action:'room_list',client:row.client,keyword:String(input.keyword||'').slice(0,64),offset:Number(input.offset||0)});return json({ok:true,rooms:result.rooms,total:result.total,offset:result.offset,account_id:row.id})}
   if(action==='servers'){const result=await this.kernel({action:'domain_list',client:row.client});return json({ok:true,servers:result.servers,account_id:row.id})}
   if(action==='configure'){if(input.allow_project_admin===false)return json({ok:false,error:'项目管理员调用权限是固定策略，不能关闭'},400);row.allow_project_admin=true;if(typeof input.keep_alive==='boolean')row.keep_alive=input.keep_alive;if(typeof input.name==='string')row.name=input.name.slice(0,48);if(typeof input.server_id==='string')row.server_id=input.server_id.slice(0,64);if(typeof input.avatar==='string'){if(input.avatar.length>150000||input.avatar&&!/^data:image\/(png|jpeg);base64,[A-Za-z0-9+/]+=*$/.test(input.avatar))return json({ok:false,error:'头像必须为小于110KB的PNG/JPEG'},400);row.avatar=input.avatar}}
   else if(action==='refresh'){await this.refreshSession(row);}
   else if(action==='nickname'){
    const state=await this.kernel({action,client:row.client,nickname:input.nickname});row.client=state.client;row.nickname=state.nickname;row.last_refresh=Date.now();row.status='ready';row.last_error='';row.failures=0;
   }else if(['skin','skin_info','skin_preview'].includes(action)){
    const state=await this.kernel({action,client:row.client,item_id:String(input.item_id||'')});if(state.client)row.client=state.client;row.skin_item_id=state.skin_info?.ItemID||'';row.skin_body_known=state.body_type_known===true;row.skin_slim=state.skin_info?.SkinIsSlim===true;await this.put(row);return json({ok:true,account:publicAccount(row),skin_info:state.skin_info,texture:state.texture,width:state.width,height:state.height,custom_geometry:state.custom_geometry,body_type_known:state.body_type_known,warning:state.warning||'',reconnect_required:action==='skin'});
   }else return json({ok:false,error:'未知账号操作'},400);
   await this.put(row);if(row.keep_alive)await this.ctx.storage.setAlarm(Date.now()+1500000);return json({ok:true,account:publicAccount(row)});
   });
  }catch(error){return json({ok:false,success:false,error:error.message,message:error.message,error_info:error.message,...error.detail},400)}
 }
 async phoenix(url,request,input){
  const nonce=request.headers.get('X-Auth-Session');const key='session:'+nonce;
  if(['/api/phoenix/login','/api/phoenix/tan_lobby_login'].includes(url.pathname)){
   const local=url.pathname==='/api/phoenix/tan_lobby_login';
   const id=await this.ctx.storage.get('token:'+await hash(String(input.login_token||'')));const row=id?await this.get(id):null;
   if(!row)return json({success:false,message:'游园游戏令牌不存在或已撤销'});
   return this.accountSerial(row.id,async()=>{
   const currentId=await this.ctx.storage.get('token:'+await hash(String(input.login_token||''))),current=currentId?await this.get(currentId):null;
   if(!current||current.id!==row.id)return json({success:false,message:'游园游戏令牌不存在或已撤销'},401);
   Object.assign(row,current);
   const delegatedAccount=request.headers.get('X-YouYuan-Delegated-Account');if(delegatedAccount&&(row.id!==delegatedAccount))return json({success:false,message:'该会话不能调用其他游戏账号'},403);
   if(local?(typeof input.room_id!=='string'||!input.room_id.trim()||input.room_id.length>128):(!input.client_public_key||!input.server_code))return json({success:false,message:'缺少房间标识或服务器认证参数',error_info:'缺少房间标识或服务器认证参数'},400);
   if(!row.last_refresh||Date.now()-row.last_refresh>1500000){await this.refreshSession(row);}
   const login=()=>this.kernel(local?{action:'tan_lobby_login',client:row.client,room_id:input.room_id}:{action:'game_login',client:row.client,server:normalizeGameServer(input.server_code),server_id:row.server_id,server_password:input.server_passcode,public_key:input.client_public_key});
   let state;
   try{state=await login();}
   catch(error){
    // This exact read-only search failure occurs BEFORE Link/GameStart. Other
    // failures may already have side effects and must never be replayed here.
    if(local||!/^SearchRentalServerByName: 请先登录\(10\)$/.test(error.message||''))throw error;
    await this.refreshSession(row);await this.put(row);
    state=await login();
   }
   const r=state.result;
   row.client=state.client;row.last_used=Date.now();row.status='ready';await this.put(row);
   if(local){
    await this.ctx.storage.put(key,{uid:String(r.user_unique_id),engine:state.engine_version,patch:state.patch_version,pc:false,account:row.id,token_hash:await hash(row.login_token),expires:Date.now()+86400000});
    const skin=state.skin_info||{};return json({success:true,error_info:'',...r,skin_info:{entity_id:skin.ItemID||'',res_url:skin.SkinDownloadURL||'',is_slim:!!skin.SkinIsSlim},skin_warning:state.skin_error||''});
   }
   await this.ctx.storage.put(key,{uid:r.UID,engine:r.EngineVersion,patch:r.PatchVersion,pc:!!r.IsPC,account:row.id,token_hash:await hash(row.login_token),expires:Date.now()+86400000});
   const skin=state.skin_info||{};
   return json({success:true,message:'正常返回',growth_level:r.BotLevel,respond_to:r.MasterName,ip_address:r.IP,chainInfo:r.ChainInfo,token:row.login_token,skin_info:{entity_id:skin.ItemID||'',res_url:skin.SkinDownloadURL||'',is_slim:!!skin.SkinIsSlim},outfit_info:r.BotComponent||{},skin_warning:state.skin_error||''});
   });
  }
  const session=await this.ctx.storage.get(key),account=session?await this.get(session.account):null;if(!session||session.expires<Date.now()||!account||session.token_hash!==await hash(account.login_token))return json({success:false,message:'请先使用有效游园令牌完成服务器认证'});
  if(url.pathname==='/api/phoenix/transfer_start_type'){
   const r=await this.kernel({action:'transfer_start_type',uid:session.uid,data:url.searchParams.get('content')||''});return json({success:true,data:r.data});
  }
  if(url.pathname==='/api/phoenix/transfer_check_num'){
   const r=await this.kernel({action:'transfer_check_num',data:input.data,engine_version:session.engine,patch_version:session.patch,is_pc:session.pc});return json({success:true,value:r.value});
  }
  return json({success:false,message:'此认证入口尚未开放该服务器类型'},404);
 }
 alarm(){const result=this.serial.then(()=>this.refreshAll());this.serial=result.catch(()=>{});return result;}
 async refreshAll(){
  let next=0,processed=0;
  const routes=await this.ctx.storage.list({prefix:'route:'});for(const [key,value]of routes){if(!value.expires)continue;if(value.expires<=Date.now())await this.ctx.storage.delete(key);else next=next?Math.min(next,value.expires+1000):value.expires+1000;}
  for(const candidate of await this.rows()){
   await this.accountSerial(candidate.id,async()=>{
   const row=await this.get(candidate.id);if(!row)return;
   if(!row.keep_alive)return;const due=(row.next_refresh||row.last_refresh+1500000);if(due>Date.now()){next=next?Math.min(next,due):due;return}
   if(processed++>=4){next=next?Math.min(next,Date.now()+15000):Date.now()+15000;return}
   try{await this.refreshSession(row);}
   catch{row.failures=(row.failures||0)+1;row.status='refresh_failed';row.last_error='网易凭据刷新失败，请检查账号状态后重新登录';row.next_refresh=Date.now()+Math.min(1500000,300000*row.failures)}
   await this.put(row);next=next?Math.min(next,row.next_refresh):row.next_refresh;
   });
  }if(next)await this.alarmBy(next);
 }
}
