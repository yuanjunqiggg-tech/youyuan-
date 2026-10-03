import {runKernel} from './runtime.mjs';
import {identityResolve,identityRoute} from '../cloudflare/identity.mjs';
import {seal,open,hash,token} from './vault.mjs';
import {normalizeGameServer} from './server-code.mjs';
const json=(data,status=200)=>Response.json(data,{status,headers:{'Cache-Control':'no-store'}});
const vault=(env,owner)=>env.AUTH.get(env.AUTH.idFromName('account:'+owner));
async function actor(request,env){const key=request.headers.get('X-YouYuan-Device-Key')||(request.headers.get('Authorization')||'').replace(/^Bearer /,'');if(!key)return null;if(env.DEVICE_TOKEN&&key===env.DEVICE_TOKEN||env.CONSOLE_TOKEN&&key===env.CONSOLE_TOKEN)return 'owner';for(const purpose of ['device','session']){const identity=await identityResolve(env,key,purpose);if(identity)return identity.user.legacy_owner?'owner':identity.user.id;}return null;}
export default {
 async fetch(request,env){
  const url=new URL(request.url);
  if(url.pathname==='/health')return json({ok:true,product:'游园独立游戏认证',provider:'direct-netease',third_party_auth:false});
  if(url.pathname.startsWith('/api/identity/'))return identityRoute(request,env);
  if(url.pathname==='/'&&env.ASSETS)return env.ASSETS.fetch(request);
  if(url.pathname==='/api/new'){
   const owner=await actor(request,env);if(!owner)return json({success:false,message:'请先绑定游园账号设备密钥'},401);
   const session={owner,nonce:token(),expires:Date.now()+86400000};const sealed=await seal(env,'auth-session',session);
   return new Response(btoa(JSON.stringify(sealed)),{headers:{'Content-Type':'text/plain','Cache-Control':'no-store'}});
  }
  let owner;
  if(url.pathname.startsWith('/api/phoenix/')){
   const raw=(request.headers.get('Authorization')||'').replace(/^Bearer /,'');let session;
   try{session=await open(env,'auth-session',JSON.parse(atob(raw)))}catch{return json({success:false,message:'认证会话无效或过期'},401)}
   if(session.expires<Date.now())return json({success:false,message:'认证会话已过期'},401);
   owner=session.owner;const headers=new Headers(request.headers);headers.set('X-Auth-Session',session.nonce);return vault(env,owner).fetch(new Request(request,{headers}));
  }
  if(url.pathname!=='/api/accounts'&&url.pathname!=='/api/kernel-test')return json({ok:false,error:'未知认证接口'},404);
  owner=await actor(request,env);if(!owner)return json({ok:false,error:'请先登录游园账号'},401);
  if(url.pathname==='/api/kernel-test'){
   if(owner!=='owner'||request.method!=='POST')return json({ok:false,error:'仅用于所有者认证验收'},403);
   const body=await request.text();if(body.length>100000)return json({ok:false,error:'请求过大'},413);
   try{return json(await runKernel(JSON.parse(body)))}catch(error){return json({ok:false,error:'认证运行时：'+error.message},500)}
  }
  return vault(env,owner).fetch(request);
 }
};
function publicAccount(row){return{id:row.id,name:row.name,game_id:row.game_id,nickname:row.nickname,created:row.created,keep_alive:row.keep_alive,status:row.status,last_refresh:row.last_refresh||null,last_error:row.last_error||'',token_hint:row.login_token.slice(0,5)+'…',server_id:row.server_id||''};}
export class AuthVault{
 constructor(ctx,env){this.ctx=ctx;this.env=env;this.serial=Promise.resolve();}
 fetch(request){const result=this.serial.then(()=>this.handle(request));this.serial=result.catch(()=>{});return result;}
 async get(id){const value=await this.ctx.storage.get('account:'+id);return value?open(this.env,'account:'+id,value):null;}
 async put(row){await this.ctx.storage.put('account:'+row.id,await seal(this.env,'account:'+row.id,row));}
 async rows(){const rows=await this.ctx.storage.list({prefix:'account:'});const out=[];for(const [key,value]of rows)out.push(await open(this.env,key,value));return out;}
 async kernel(input){const result=await runKernel(input);if(!result.ok){const error=new Error(result.error||'网易认证失败');error.detail={code:result.code,verification_required:result.verification_required,verification_url:result.verification_url};throw error;}return result;}
 async handle(request){
  const url=new URL(request.url);let input={};
  try{
   if(request.method!=='GET'){const text=await request.text();if(text.length>100000)return json({ok:false,error:'请求过大'},413);input=JSON.parse(text||'{}')}
   if(url.pathname.startsWith('/api/phoenix/'))return await this.phoenix(url,request,input);
   if(request.method==='GET')return json({ok:true,accounts:(await this.rows()).map(publicAccount),provider:'direct-netease'});
   if(request.method!=='POST')return json({ok:false,error:'GET/POST required'},405);
   const action=input.action;
   if(['login_email','import_cookie','register_guest'].includes(action)){
    const rows=await this.rows();if(rows.length>=32)return json({ok:false,error:'每个游园账号最多32个游戏凭据'},409);
    if(action==='login_email'&&(!/^.{1,128}@.{1,128}$/.test(input.email||'')||!input.password))return json({ok:false,error:'请填写网易邮箱和密码'},400);
    if(action==='register_guest'&&([...String(input.nickname||'')].length<2||[...String(input.nickname||'')].length>20))return json({ok:false,error:'请填写2–20字符的游戏昵称'},400);
    const rate=await this.ctx.storage.get('login-rate')||{at:0,n:0};if(Date.now()-rate.at>60000){rate.at=Date.now();rate.n=0}rate.n++;await this.ctx.storage.put('login-rate',rate);if(rate.n>5)return json({ok:false,error:'登录操作过于频繁，请一分钟后再试'},429);
    let cookie=input.cookie;
    if(action!=='import_cookie'){
     const generated=await this.kernel({action:'device_create'});const logged=await this.kernel({action:action==='register_guest'?'guest_login':'email_login',device:generated.device,email:input.email,password:input.password});cookie=logged.cookie;
    }
    if(typeof cookie!=='string'||cookie.length<20||cookie.length>50000)return json({ok:false,error:'登录凭据格式无效'},400);
    let state=await this.kernel({action:'authenticate',cookie});
    if(action==='register_guest')state=await this.kernel({action:'nickname',client:state.client,nickname:input.nickname});
    if(rows.some(row=>row.game_id===state.game_id))return json({ok:false,error:'这个游戏账号已经添加，请刷新凭据或撤销后重新导入'},409);
    const row={id:crypto.randomUUID(),name:String(input.name||state.nickname||'游戏账号').slice(0,48),cookie,client:state.client,login_token:'yy//'+token(),game_id:state.game_id,nickname:state.nickname,created:Date.now(),keep_alive:true,status:'ready',last_refresh:Date.now(),failures:0};
    await this.put(row);await this.ctx.storage.put('token:'+await hash(row.login_token),row.id);await this.ctx.storage.setAlarm(Date.now()+1500000);return json({ok:true,account:publicAccount(row),login_token:row.login_token,auth_url:'https://auth.youyuanqi.dpdns.org'});
   }
   const row=await this.get(String(input.id||''));if(!row)return json({ok:false,error:'游戏凭据不存在'},404);
   if(action==='delete'){
    await this.ctx.storage.delete(['account:'+row.id,'token:'+await hash(row.login_token)]);return json({ok:true});
   }
   if(action==='token')return json({ok:true,login_token:row.login_token,auth_url:'https://auth.youyuanqi.dpdns.org'});
   if(action==='reissue'){
    const old=await hash(row.login_token);row.login_token='yy//'+token();await this.ctx.storage.transaction(async store=>{await store.put('account:'+row.id,await seal(this.env,'account:'+row.id,row));await store.put('token:'+await hash(row.login_token),row.id);await store.delete('token:'+old)});return json({ok:true,login_token:row.login_token});
   }
   if(action==='rooms'){const result=await this.kernel({action:'room_list',client:row.client,keyword:String(input.keyword||'').slice(0,64),offset:Number(input.offset||0)});return json({ok:true,rooms:result.rooms,total:result.total,offset:result.offset,account_id:row.id})}
   if(action==='servers'){const result=await this.kernel({action:'domain_list',client:row.client});return json({ok:true,servers:result.servers,account_id:row.id})}
   if(action==='configure'){if(typeof input.keep_alive==='boolean')row.keep_alive=input.keep_alive;if(typeof input.name==='string')row.name=input.name.slice(0,48);if(typeof input.server_id==='string')row.server_id=input.server_id.slice(0,64)}
   else if(action==='refresh'||action==='nickname'){
    const state=await this.kernel({action,client:row.client,nickname:input.nickname});row.client=state.client;row.nickname=state.nickname;row.last_refresh=Date.now();row.status='ready';row.last_error='';row.failures=0;
   }else return json({ok:false,error:'未知账号操作'},400);
   await this.put(row);if(row.keep_alive)await this.ctx.storage.setAlarm(Date.now()+1500000);return json({ok:true,account:publicAccount(row)});
  }catch(error){return json({ok:false,success:false,error:error.message,message:error.message,...error.detail},400)}
 }
 async phoenix(url,request,input){
  const nonce=request.headers.get('X-Auth-Session');const key='session:'+nonce;
  if(url.pathname==='/api/phoenix/login'){
   const id=await this.ctx.storage.get('token:'+await hash(String(input.login_token||'')));const row=id?await this.get(id):null;
   if(!row)return json({success:false,message:'游园游戏令牌不存在、已撤销或属于其他用户'});
   if(!input.client_public_key||!input.server_code)return json({success:false,message:'缺少服务器号或客户端公钥'});
   if(!row.last_refresh||Date.now()-row.last_refresh>1500000){const state=await this.kernel({action:'refresh',client:row.client});row.client=state.client;row.last_refresh=Date.now()}
   const state=await this.kernel({action:'game_login',client:row.client,server:normalizeGameServer(input.server_code),server_id:row.server_id,server_password:input.server_passcode,public_key:input.client_public_key});const r=state.result;
   row.client=state.client;row.last_used=Date.now();row.status='ready';await this.put(row);
   await this.ctx.storage.put(key,{uid:r.UID,engine:r.EngineVersion,patch:r.PatchVersion,pc:!!r.IsPC,account:row.id,token_hash:await hash(row.login_token),expires:Date.now()+86400000});
   return json({success:true,message:'正常返回',growth_level:r.BotLevel,respond_to:r.MasterName,ip_address:r.IP,chainInfo:r.ChainInfo,token:row.login_token});
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
  let next=0,processed=0;for(const row of await this.rows()){
   if(!row.keep_alive)continue;const due=(row.next_refresh||row.last_refresh+1500000);if(due>Date.now()){next=next?Math.min(next,due):due;continue}
   if(processed++>=4){next=next?Math.min(next,Date.now()+15000):Date.now()+15000;continue}
   try{const state=await this.kernel({action:'refresh',client:row.client});row.client=state.client;row.last_refresh=Date.now();row.status='ready';row.failures=0;row.last_error='';row.next_refresh=Date.now()+1500000}
   catch{row.failures=(row.failures||0)+1;row.status='refresh_failed';row.last_error='网易凭据刷新失败，请检查账号状态后重新登录';row.next_refresh=Date.now()+Math.min(1500000,300000*row.failures)}
   await this.put(row);next=next?Math.min(next,row.next_refresh):row.next_refresh;
  }if(next)await this.ctx.storage.setAlarm(next);
 }
}
