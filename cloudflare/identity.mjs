const encode = bytes => [...new Uint8Array(bytes)].map(x=>x.toString(16).padStart(2,'0')).join('');
const randomToken = () => encode(crypto.getRandomValues(new Uint8Array(32)));
const digest = async text => encode(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(text)));
async function passwordHash(password,salt){const key=await crypto.subtle.importKey('raw',new TextEncoder().encode(password),'PBKDF2',false,['deriveBits']);return encode(await crypto.subtle.deriveBits({name:'PBKDF2',salt:new TextEncoder().encode(salt),iterations:100000,hash:'SHA-256'},key,256));}
function publicUser(user){return {id:user.id,username:user.username,qq:user.qq,qq_verified:false,qq_binding_status:'已登记，尚未验证归属',legacy_owner:!!user.legacy_owner,created:user.created};}
const response=(data,status=200)=>Response.json(data,{status});
function registry(env){return env.RELAY.get(env.RELAY.idFromName('identity-registry-v1'));}
export async function identityResolve(env,token,purpose='session'){
 if(!token)return null;
 const result=await registry(env).fetch(new Request('https://identity.internal/__identity/resolve',{method:'POST',body:JSON.stringify({token,purpose})}));
 const data=await result.json();return data.ok?data:null;
}
export async function identityRoute(request,env){
 const url=new URL(request.url),action=url.pathname.split('/').pop();
 if(!['register','login','me','logout','claim','delete'].includes(action))return response({ok:false,error:'未知账号操作'},404);
 const ip=request.headers.get('CF-Connecting-IP')||'local';
 const body=request.method==='GET'?'{}':await request.text();if(body.length>16384)return response({ok:false,error:'请求过大'},413);
 return registry(env).fetch(new Request('https://identity.internal/__identity/'+action,{method:'POST',headers:{'Authorization':request.headers.get('Authorization')||'','X-Identity-IP':ip},body}));
}
export async function identityHandle(request,ctx,env){
 let input;try{input=await request.json()}catch{return response({ok:false,error:'JSON 无效'},400)}
 const action=new URL(request.url).pathname.split('/').pop(),now=Date.now();
 if(action==='resolve'){
  const hash=await digest(String(input.token||''));
  const record=await ctx.storage.get((input.purpose==='device'?'device:':'session:')+hash)|| (input.purpose==='session'?await ctx.storage.get('control:'+hash):null);
  if(!record||record.expires&&record.expires<now)return response({ok:false},401);
  const user=await ctx.storage.get('user:'+record.user);if(!user)return response({ok:false},401);
  return response({ok:true,user:publicUser(user),relay_prefix:user.legacy_owner?'':'tenant:'+user.id+':',device_slug:'primary-device'});
 }
 const bearer=(request.headers.get('Authorization')||'').replace(/^Bearer /,'');
 let user=null,sessionHash=null;
 if(bearer){sessionHash=await digest(bearer);const session=await ctx.storage.get('session:'+sessionHash);if(session&&session.expires>now)user=await ctx.storage.get('user:'+session.user);}
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
   const qq=String(input.qq||'').trim();if(!/^[1-9][0-9]{4,11}$/.test(qq))return response({ok:false,error:'请填写有效 QQ 号'},400);
   if(index)return response({ok:false,error:'账号已存在'},409);
   const salt=randomToken();user={id:crypto.randomUUID(),username:name,qq,created:now,salt,password_hash:await passwordHash(password,salt),device_token:randomToken(),control_token:randomToken()};
   try{await ctx.storage.transaction(async store=>{if(await store.get('name:'+name))throw new Error('账号已存在');await store.put('user:'+user.id,user);await store.put('name:'+name,user.id);await store.put('device:'+await digest(user.device_token),{user:user.id});await store.put('control:'+await digest(user.control_token),{user:user.id});});}catch{return response({ok:false,error:'账号已存在'},409);}
  }else{
   user=index?await ctx.storage.get('user:'+index):null;
   const actual=await passwordHash(password,user?.salt||'unknown-user-salt');
   if(!user||actual!==user.password_hash)return response({ok:false,error:'账号或密码不正确'},401);
  }
  const token=randomToken();await ctx.storage.put('session:'+await digest(token),{user:user.id,expires:now+30*86400000});
  return response({ok:true,user:publicUser(user),session_token:token,device_token:user.device_token,control_token:user.control_token,expires:now+30*86400000});
 }
 if(!user)return response({ok:false,error:'请先登录游园账号'},401);
 if(action==='me')return response({ok:true,user:publicUser(user)});
 if(action==='logout'){await ctx.storage.delete('session:'+sessionHash);return response({ok:true});}
 if(action==='claim'){
  if(!env.DEVICE_TOKEN||input.device_key!==env.DEVICE_TOKEN)return response({ok:false,error:'既有设备证明不正确'},403);
  const claimed=await ctx.storage.transaction(async store=>{const owner=await store.get('legacy-owner');if(owner&&owner!==user.id)return false;user.legacy_owner=true;await store.put('legacy-owner',user.id);await store.put('user:'+user.id,user);return true});if(!claimed)return response({ok:false,error:'设备已绑定另一平台账号'},409);return response({ok:true,user:publicUser(user)});
 }
 if(action==='delete'){
  if(await passwordHash(String(input.password||''),user.salt)!==user.password_hash)return response({ok:false,error:'密码不正确'},403);
  await ctx.storage.delete(['user:'+user.id,'name:'+user.username,'device:'+await digest(user.device_token),'control:'+await digest(user.control_token),'session:'+sessionHash]);
  if(await ctx.storage.get('legacy-owner')===user.id)await ctx.storage.delete('legacy-owner');
  return response({ok:true});
 }
 return response({ok:false,error:'未知操作'},404);
}
