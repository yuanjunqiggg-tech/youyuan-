import {normalizeEmail,validQQEmail,requestRegistrationCode,checkRegistrationCode,liveIndex} from './email-verification.mjs';
const encode = bytes => [...new Uint8Array(bytes)].map(x=>x.toString(16).padStart(2,'0')).join('');
const randomToken = () => encode(crypto.getRandomValues(new Uint8Array(32)));
const digest = async text => encode(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(text)));
async function passwordHash(password,salt){const key=await crypto.subtle.importKey('raw',new TextEncoder().encode(password),'PBKDF2',false,['deriveBits']);return encode(await crypto.subtle.deriveBits({name:'PBKDF2',salt:new TextEncoder().encode(salt),iterations:100000,hash:'SHA-256'},key,256));}
function publicUser(user){const role=user.legacy_owner?'admin':user.role||'member',expires=Number(user.subscription_until)||0;return {id:user.id,username:user.username,email:user.email||'',email_verified:!!user.email_verified,qq:user.qq,qq_verified:false,qq_binding_status:user.email_verified?'邮箱已验证':'旧版 QQ 登记，邮箱未验证',legacy_owner:!!user.legacy_owner,created:user.created,role,subscription_until:expires,cloud_allowed:role==='admin'||expires>Date.now(),assigned_devices:user.assigned_devices||[]};}
async function removeAccount(store,target,actor,now){
 const records=await store.list();
 const keys=[...records].filter(([k,v])=>k==='user:'+target.id||((k.startsWith('name:')||k.startsWith('email:'))&&v===target.id)||v?.user===target.id||v?.user_id===target.id).map(([k])=>k);
 if(keys.length)await store.delete(keys);
 if(await store.get('legacy-owner')===target.id)await store.delete('legacy-owner');
 await store.put('account-delete-audit:'+now+':'+crypto.randomUUID(),{actor,target:target.id,username:target.username,timestamp:now,revoked:keys.length});
 return keys.length;
}
const response=(data,status=200)=>Response.json(data,{status});
function registry(env){return env.RELAY.get(env.RELAY.idFromName('identity-registry-v1'));}
export async function identityResolve(env,token,purpose='session'){
 if(!token)return null;
 const result=await registry(env).fetch(new Request('https://identity.internal/__identity/resolve',{method:'POST',body:JSON.stringify({token,purpose})}));
 const data=await result.json();return data.ok?data:null;
}
export async function identityRoute(request,env){
 const url=new URL(request.url),action=url.pathname.split('/').pop();
 if(!['register-code','register','login','me','binding','logout','claim','delete','users','entitlement','password','reset-password','mcp-links','backup','delete-user'].includes(action))return response({ok:false,error:'未知账号操作'},404);
 const ip=request.headers.get('CF-Connecting-IP')||'local';
 const body=request.method==='GET'?'{}':await request.text();if(body.length>16384)return response({ok:false,error:'请求过大'},413);
 return registry(env).fetch(new Request('https://identity.internal/__identity/'+action,{method:'POST',headers:{'Authorization':request.headers.get('Authorization')||'','X-Identity-IP':ip},body}));
}
export async function identityHandle(request,ctx,env){
 let input;try{input=await request.json()}catch{return response({ok:false,error:'JSON 无效'},400)}
 const action=new URL(request.url).pathname.split('/').pop(),now=Date.now();
 if(action==='register-code')return requestRegistrationCode(request,input,ctx,env);
 if(action==='resolve'){
  const hash=await digest(String(input.token||''));
  if(input.purpose==='mcp'){
   const grant=await ctx.storage.get('mcp-link:'+hash);if(!grant||grant.expires<=now)return response({ok:false},401);
   if(grant.owner)return response({ok:true,owner:true,device:grant.device,scope:grant.scope,user:{role:'admin',cloud_allowed:true,assigned_devices:['linux-server']},relay_prefix:''});
   const account=await ctx.storage.get('user:'+grant.user);if(!account)return response({ok:false},401);
   return response({ok:true,owner:false,device:grant.device,scope:grant.scope,user:publicUser(account),relay_prefix:account.legacy_owner?'':'tenant:'+account.id+':'});
  }
  const record=await ctx.storage.get((input.purpose==='device'?'device:':'session:')+hash)|| (input.purpose==='session'?await ctx.storage.get('control:'+hash):null);
  if(!record||record.expires&&record.expires<now)return response({ok:false},401);
  const user=await ctx.storage.get('user:'+record.user);if(!user)return response({ok:false},401);
  return response({ok:true,user:publicUser(user),relay_prefix:user.legacy_owner?'':'tenant:'+user.id+':',device_slug:'primary-device'});
 }
 const bearer=(request.headers.get('Authorization')||'').replace(/^Bearer /,'');
 let user=null,sessionHash=null;
 if(bearer){sessionHash=await digest(bearer);const session=await ctx.storage.get('session:'+sessionHash);if(session&&session.expires>now)user=await ctx.storage.get('user:'+session.user);}
 if(action==='binding'&&!user&&bearer){const binding=await ctx.storage.get('control:'+sessionHash);if(binding)user=await ctx.storage.get('user:'+binding.user);}
 if(action==='me'&&env.CONSOLE_TOKEN&&bearer===env.CONSOLE_TOKEN)return response({ok:true,user:{username:'项目管理员',role:'admin',cloud_allowed:true,subscription_until:0,assigned_devices:['linux-server'],legacy_owner:true}});
 if(action==='backup'||action==='delete-user'){
  const owner=!!env.CONSOLE_TOKEN&&bearer===env.CONSOLE_TOKEN||!!user?.legacy_owner;
  if(!owner)return response({ok:false,error:'仅项目所有者可以备份或删除平台账号'},403);
  if(action==='backup'){const records=await ctx.storage.list();return response({ok:true,created:now,records:[...records]});}
  const id=String(input.user_id||'');
  return ctx.storage.transaction(async store=>{
   const target=await store.get('user:'+id);if(!target)return response({ok:false,error:'账号不存在'},404);
   if(target.legacy_owner||await store.get('legacy-owner')===id)return response({ok:false,error:'不能删除项目所有者'},403);
   const revoked=await removeAccount(store,target,user?.id||'project-owner',now);
   return response({ok:true,username:target.username,revoked});
  });
 }
 if(action==='mcp-links'){
  const owner=!!env.CONSOLE_TOKEN&&bearer===env.CONSOLE_TOKEN;
  if(!owner&&(!user||!publicUser(user).cloud_allowed))return response({ok:false,error:'仅管理员或有效订阅用户可创建 MCP 连接'},403);
  const principal=owner?'project-owner':user.id,op=input.op||'list';
  if(op==='list'){const rows=await ctx.storage.list({prefix:'mcp-link:'});return response({ok:true,links:[...rows.values()].filter(g=>g.user===principal).map(g=>({id:g.id,name:g.name,device:g.device,scope:g.scope,expires:g.expires}))});}
  if(op==='revoke'){const g=await ctx.storage.get('mcp-link:'+String(input.id||''));if(!g||g.user!==principal)return response({ok:false,error:'连接不存在'},404);await ctx.storage.delete('mcp-link:'+g.id);return response({ok:true});}
  if(op!=='create')return response({ok:false,error:'op支持list/create/revoke'},400);
  const device=String(input.device||'primary-device'),scope=input.scope||'read',hours=input.hours===undefined?720:Number(input.hours);
  if(!/^[A-Za-z0-9_-]{1,48}$/.test(device)||!['read','control'].includes(scope)||!Number.isInteger(hours)||hours<1||hours>8760)return response({ok:false,error:'设备、权限或有效期无效'},400);
  if(!owner&&!user.legacy_owner&&device.startsWith('linux-')&&!(user.assigned_devices||[]).includes(device))return response({ok:false,error:'该 Linux 节点未授权'},403);
  const rows=await ctx.storage.list({prefix:'mcp-link:'});if([...rows.values()].filter(g=>g.user===principal&&g.expires>now).length>=128)return response({ok:false,error:'请撤销不用的连接'},409);
  const token=randomToken(),id=await digest(token),g={id,user:principal,owner,device,scope,name:String(input.name||'').slice(0,80),expires:now+hours*3600000};await ctx.storage.put('mcp-link:'+id,g);
  return response({ok:true,created:{id,name:g.name,device,scope,expires:g.expires,token}});
 }
 if(action==='users'||action==='entitlement'||action==='reset-password'){
  const owner=!!env.CONSOLE_TOKEN&&bearer===env.CONSOLE_TOKEN||!!user?.legacy_owner;
  if(!owner&&(!user||publicUser(user).role!=='admin'))return response({ok:false,error:'仅管理员可以管理订阅'},403);
  if(action==='users'){const rows=await ctx.storage.list({prefix:'user:'});return response({ok:true,users:[...rows.values()].map(publicUser).sort((a,b)=>b.created-a.created)});}
  const target=await ctx.storage.get('user:'+String(input.user_id||''));if(!target)return response({ok:false,error:'用户不存在'},404);
  if(action==='reset-password'){
   if(!owner&&publicUser(target).role==='admin')return response({ok:false,error:'只有项目所有者可以重置管理员密码'},403);
   const next=String(input.new_password||'');if(next.length<10||next.length>128)return response({ok:false,error:'新密码需10–128字符'},400);
   const salt=randomToken(),hash=await passwordHash(next,salt);
   await ctx.storage.transaction(async store=>{const current=await store.get('user:'+target.id);if(!current)throw Error('用户不存在');current.salt=salt;current.password_hash=hash;current.password_reset_at=now;await store.put('user:'+current.id,current);const sessions=await store.list({prefix:'session:'});const keys=[...sessions].filter(([,v])=>v.user===current.id).map(([k])=>k);if(keys.length)await store.delete(keys);await store.put('password-reset-audit:'+now+':'+crypto.randomUUID(),{actor:owner?'project-owner':user.id,target:current.id,timestamp:now});});
   return response({ok:true,sessions_revoked:true,username:target.username});
  }
  if(target.legacy_owner)return response({ok:false,error:'项目所有者权限不能通过订阅接口修改'},403);
  if(!owner&&input.role==='admin')return response({ok:false,error:'只有项目所有者可以授予管理员'},403);
  if(!owner&&publicUser(target).role==='admin')return response({ok:false,error:'只有项目所有者可以修改管理员'},403);
  if(!['member','admin'].includes(input.role)||!Number.isSafeInteger(input.subscription_until)||input.subscription_until<0)return response({ok:false,error:'角色或到期时间无效'},400);
  const devices=input.assigned_devices||[];
  if(!Array.isArray(devices)||devices.length>8||devices.some(x=>!/^linux-[A-Za-z0-9_-]{1,40}$/.test(x)))return response({ok:false,error:'只允许分配明确的 Linux 节点'},400);
  target.role=input.role;target.subscription_until=input.subscription_until;target.assigned_devices=[...new Set(devices)];target.entitlement_updated=now;
  await ctx.storage.put('user:'+target.id,target);
  await ctx.storage.put('entitlement-audit:'+now+':'+crypto.randomUUID(),{actor:owner?'project-owner':user.id,target:target.id,role:target.role,subscription_until:target.subscription_until,assigned_devices:target.assigned_devices,timestamp:now});
  return response({ok:true,user:publicUser(target)});
 }
 if(action==='register'||action==='login'){
  const name=String(input.username||'').normalize('NFKC').trim().toLowerCase(),password=String(input.password||'');
  if(!/^[A-Za-z0-9_\-\u4e00-\u9fff]{3,32}$/.test(name)||password.length<10||password.length>128)return response({ok:false,error:'账号需 3–32 个字符，密码需 10–128 个字符'},400);
  const ip=await digest(request.headers.get('X-Identity-IP')||'local');
  // Shared household networks can onboard several users without sharing an account.
  for(const [key,limit,window] of [['rate:'+action+':'+ip,action==='register'?20:30,action==='register'?3600000:60000],['rate:login:'+name,10,60000]]){
   const allowed=await ctx.storage.transaction(async store=>{let rate=await store.get(key)||{start:now,n:0};if(now-rate.start>window)rate={start:now,n:0};rate.n++;await store.put(key,rate);return rate.n<=limit});if(!allowed)return response({ok:false,error:'尝试过于频繁，请稍后再试'},429);
  }
  const index=await ctx.storage.get('name:'+name);
  if(action==='register'){
   const email=normalizeEmail(input.email);if(!validQQEmail(email))return response({ok:false,error:'请填写有效 QQ 邮箱，例如 123456@qq.com'},400);
   const salt=randomToken();user={id:crypto.randomUUID(),username:name,email,email_verified:true,created:now,salt,password_hash:await passwordHash(password,salt),device_token:randomToken(),control_token:randomToken()};
   const rejected=await ctx.storage.transaction(async store=>{
    if(await liveIndex(store,'name:'+name))return {status:409,error:'账号已存在；退出登录不会注销账号，请登录或在平台账号中办理注销'};
    if(await liveIndex(store,'email:'+email))return {status:409,error:'该 QQ 邮箱已绑定账号，请登录原账号'};
    const verified=await checkRegistrationCode(store,name,email,String(input.email_code||''),now);if(verified.error)return verified;
    await store.put('user:'+user.id,user);await store.put('name:'+name,user.id);await store.put('email:'+email,user.id);await store.put('device:'+await digest(user.device_token),{user:user.id});await store.put('control:'+await digest(user.control_token),{user:user.id});await store.delete(verified.key);return null;
   });
   if(rejected)return response({ok:false,error:rejected.error},rejected.status);
  }else{
   user=index?await ctx.storage.get('user:'+index):null;
   const actual=await passwordHash(password,user?.salt||'unknown-user-salt');
   if(!user||actual!==user.password_hash)return response({ok:false,error:'账号或密码不正确'},401);
  }
  const token=randomToken();await ctx.storage.put('session:'+await digest(token),{user:user.id,expires:now+30*86400000});
  return response({ok:true,user:publicUser(user),session_token:token,device_token:user.device_token,control_token:user.control_token,expires:now+30*86400000});
 }
 if(!user)return response({ok:false,error:'请先登录游园账号'},401);
 if(action==='password'){
  const next=String(input.new_password||'');if(next.length<10||next.length>128)return response({ok:false,error:'新密码需10–128字符'},400);
  const rateKey='rate:password:'+user.id;const allowed=await ctx.storage.transaction(async store=>{let rate=await store.get(rateKey)||{start:now,n:0};if(now-rate.start>60000)rate={start:now,n:0};rate.n++;await store.put(rateKey,rate);return rate.n<=10});if(!allowed)return response({ok:false,error:'修改尝试过于频繁'},429);
  if(await passwordHash(String(input.current_password||''),user.salt)!==user.password_hash)return response({ok:false,error:'原密码不正确'},403);
  user.salt=randomToken();user.password_hash=await passwordHash(next,user.salt);await ctx.storage.put('user:'+user.id,user);
  const sessions=await ctx.storage.list({prefix:'session:'});const old=[...sessions].filter(([k,v])=>v.user===user.id&&k!=='session:'+sessionHash).map(([k])=>k);if(old.length)await ctx.storage.delete(old);
  return response({ok:true,other_sessions_revoked:true});
 }
 if(action==='me'||action==='binding')return response({ok:true,user:publicUser(user)});
 if(action==='logout'){await ctx.storage.delete('session:'+sessionHash);return response({ok:true});}
 if(action==='claim'){
  if(!env.DEVICE_TOKEN||input.device_key!==env.DEVICE_TOKEN)return response({ok:false,error:'既有设备证明不正确'},403);
  const claimed=await ctx.storage.transaction(async store=>{const owner=await store.get('legacy-owner');if(owner&&owner!==user.id)return false;user.legacy_owner=true;await store.put('legacy-owner',user.id);await store.put('user:'+user.id,user);return true});if(!claimed)return response({ok:false,error:'设备已绑定另一平台账号'},409);return response({ok:true,user:publicUser(user)});
 }
 if(action==='delete'){
  if(await passwordHash(String(input.password||''),user.salt)!==user.password_hash)return response({ok:false,error:'密码不正确'},403);
  if(user.legacy_owner||await ctx.storage.get('legacy-owner')===user.id)return response({ok:false,error:'项目所有者不能自助注销，请先完成项目所有权交接'},403);
  return ctx.storage.transaction(async store=>{const current=await store.get('user:'+user.id);if(!current)return response({ok:false,error:'账号已注销'},401);const revoked=await removeAccount(store,current,user.id,now);return response({ok:true,revoked,message:'平台账号已注销，账号名和邮箱可重新注册；本机插件与游戏文件保留'});});
 }
 return response({ok:false,error:'未知操作'},404);
}
