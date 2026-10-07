import {identityHandle} from './identity.mjs';
const enc=new TextEncoder();
const hex=b=>[...new Uint8Array(b)].map(x=>x.toString(16).padStart(2,'0')).join('');
const hash=async s=>hex(await crypto.subtle.digest('SHA-256',enc.encode(s)));
const token=()=>hex(crypto.getRandomValues(new Uint8Array(32)));
const b64=b=>btoa(String.fromCharCode(...new Uint8Array(b))).replaceAll('+','-').replaceAll('/','_').replace(/=+$/,'');
const json=(body,status=200)=>Response.json(body,{status,headers:{'Cache-Control':'no-store'}});
const error=(name,description,status=400)=>json({error:name,error_description:description},status);
const escape=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const registry=env=>env.RELAY.get(env.RELAY.idFromName('identity-registry-v1'));
const deviceOf=resource=>{try{return new URL(resource).pathname.match(/^\/mcp\/oauth\/([A-Za-z0-9_-]{1,48})$/)?.[1]}catch{return null}};
const allowedDevice=(u,d)=>u.cloud_allowed&&(u.legacy_owner||d==='primary-device'||d.startsWith('linux-')&&u.assigned_devices.includes(d));
function redirectAllowed(s){try{const u=new URL(s);return u.origin==='https://chatgpt.com'&&!u.hash&&!u.search&&(u.pathname==='/connector_platform_oauth_redirect'||/^\/connector\/oauth\/[A-Za-z0-9_-]+$/.test(u.pathname))}catch{return false}}
export function mcpChallenge(request){const u=new URL(request.url),d=deviceOf(u.origin+u.pathname)||'linux-server';return new Response(JSON.stringify({error:'invalid_token',error_description:'请登录游园账号并授权 MCP'}),{status:401,headers:{'Content-Type':'application/json','Cache-Control':'no-store','WWW-Authenticate':`Bearer resource_metadata="${u.origin}/.well-known/oauth-protected-resource/mcp/oauth/${d}", scope="mcp:read"`}})}
export async function oauthResolve(env,access,resource){if(!access)return null;const r=await registry(env).fetch(new Request('https://identity.internal/__oauth/resolve',{method:'POST',body:JSON.stringify({access,resource})}));const d=await r.json();return d.ok?d:null}
export async function oauthRoute(request,env){
 const u=new URL(request.url),origin=u.origin;
 if(u.pathname.startsWith('/.well-known/oauth-protected-resource')){const suffix=u.pathname.slice('/.well-known/oauth-protected-resource'.length),d=deviceOf(origin+suffix)||(!suffix?'linux-server':null);if(!d)return error('invalid_request','未知 MCP 节点',404);return json({resource:origin+'/mcp/oauth/'+d,authorization_servers:[origin],scopes_supported:['mcp:read','mcp:control'],resource_name:'游园 '+d+' MCP'})}
 if(u.pathname==='/.well-known/oauth-authorization-server')return json({issuer:origin,authorization_response_iss_parameter_supported:true,authorization_endpoint:origin+'/oauth/authorize',token_endpoint:origin+'/oauth/token',registration_endpoint:origin+'/oauth/register',revocation_endpoint:origin+'/oauth/revoke',response_types_supported:['code'],grant_types_supported:['authorization_code','refresh_token'],token_endpoint_auth_methods_supported:['none'],code_challenge_methods_supported:['S256'],scopes_supported:['mcp:read','mcp:control']});
 if(!u.pathname.startsWith('/oauth/'))return null;
 const text=['GET','HEAD'].includes(request.method)?'':await request.text();if(text.length>16384)return error('invalid_request','请求过大',413);
 return registry(env).fetch(new Request('https://identity.internal/__oauth/'+u.pathname.split('/').pop()+u.search,{method:request.method,redirect:'manual',headers:{'Content-Type':request.headers.get('Content-Type')||'','X-OAuth-Origin':origin,'X-Identity-IP':request.headers.get('CF-Connecting-IP')||'local'},body:['GET','HEAD'].includes(request.method)?undefined:text}));
}
export async function oauthHandle(request,ctx,env){
 const u=new URL(request.url),action=u.pathname.split('/').pop(),origin=request.headers.get('X-OAuth-Origin'),now=Date.now(),s=ctx.storage;
 async function principal(id){const user=await s.get('user:'+id);if(!user)return null;const role=user.legacy_owner?'admin':user.role||'member';return {...user,role,cloud_allowed:role==='admin'||Number(user.subscription_until)>Date.now(),assigned_devices:user.assigned_devices||[]}}
 async function validGrant(g,resource){if(!g||g.expires<=Date.now()||g.resource!==resource||g.issuer!==new URL(resource).origin)return null;const family=await s.get('oauth-family:'+g.family),user=await principal(g.user);return family&&!family.revoked&&user&&allowedDevice(user,g.device)&&(!user.password_reset_at||g.created>user.password_reset_at)?user:null}
 if(action==='resolve'){const input=await request.json(),g=await s.get('oauth-access:'+await hash(String(input.access||''))),user=await validGrant(g,String(input.resource||''));if(!user)return json({ok:false},401);return json({ok:true,user:{id:user.id,username:user.username,role:user.role,legacy_owner:!!user.legacy_owner,cloud_allowed:user.cloud_allowed,assigned_devices:user.assigned_devices},device:g.device,scope:g.scope,oauth:true,relay_prefix:user.legacy_owner?'':'tenant:'+user.id+':'})}
 if(!origin||!/^https:\/\//.test(origin))return error('invalid_request','缺少授权服务器标识');
 const rate=async(name,limit,window=60000)=>s.transaction(async store=>{const key='oauth-rate:'+name+':'+await hash(request.headers.get('X-Identity-IP')||'local');let r=await store.get(key)||{start:now,n:0};if(now-r.start>window)r={start:now,n:0};r.n++;await store.put(key,r);return r.n<=limit});
 if(action==='register'){
  if(request.method!=='POST')return error('invalid_request','需要 POST',405);if(!await rate('register',30,3600000))return error('slow_down','注册过于频繁',429);
  let body;try{body=await request.json()}catch{return error('invalid_client_metadata','JSON 无效')}
  if(!Array.isArray(body.redirect_uris)||!body.redirect_uris.length||body.redirect_uris.length>8||body.redirect_uris.some(x=>!redirectAllowed(x))||body.token_endpoint_auth_method&&body.token_endpoint_auth_method!=='none')return error('invalid_client_metadata','仅支持 ChatGPT 官方回调与 PKCE 公共客户端');
  const id=token(),client={client_id:id,client_name:String(body.client_name||'ChatGPT').slice(0,80),redirect_uris:[...new Set(body.redirect_uris)],issuer:origin,created:now};await s.put('oauth-client:'+id,client);return json({...client,client_id_issued_at:Math.floor(now/1000),token_endpoint_auth_method:'none',grant_types:['authorization_code','refresh_token'],response_types:['code']},201);
 }
 async function issuance(store,base){const access=token(),refresh=token(),g={...base,created:Date.now(),expires:Date.now()+3600000};await store.put('oauth-access:'+await hash(access),g);await store.put('oauth-refresh:'+await hash(refresh),{...g,expires:Date.now()+30*86400000,used:false});return {access_token:access,token_type:'Bearer',expires_in:3600,refresh_token:refresh,scope:g.scope==='control'?'mcp:read mcp:control':'mcp:read',resource:g.resource}}
 if(action==='authorize'){
  if(request.method==='GET'){
   const p=u.searchParams,client=await s.get('oauth-client:'+p.get('client_id'));
   if(!client||client.issuer!==origin||!client.redirect_uris.includes(p.get('redirect_uri')))return error('invalid_request','客户端或回调未注册');
   const finishError=(code,msg)=>{const r=new URL(p.get('redirect_uri'));r.searchParams.set('error',code);r.searchParams.set('error_description',msg);r.searchParams.set('iss',origin);if(p.has('state'))r.searchParams.set('state',p.get('state'));return Response.redirect(r,302)};
   const resource=p.get('resource'),device=deviceOf(resource),scopes=(p.get('scope')||'mcp:read').split(' ').filter(Boolean);
   if(p.get('response_type')!=='code'||p.get('code_challenge_method')!=='S256'||!/^[A-Za-z0-9_-]{43}$/.test(p.get('code_challenge')||'')||!device||new URL(resource).origin!==origin||scopes.some(x=>!['mcp:read','mcp:control'].includes(x))||(p.get('state')||'').length>2048)return finishError('invalid_request','仅支持 S256、有效资源与 MCP 权限');
   if(!await rate('authorize',60))return error('slow_down','授权请求过于频繁',429);
   const tx=token();await s.put('oauth-consent:'+await hash(tx),{client:client.client_id,name:client.client_name,redirect:p.get('redirect_uri'),challenge:p.get('code_challenge'),resource,device,scope:scopes.includes('mcp:control')?'control':'read',state:p.get('state'),issuer:origin,expires:now+600000});
   const page=`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>游园 MCP 授权</title><style>body{font:16px system-ui;background:#edf5f3;color:#163a34;margin:0;padding:24px}main{max-width:460px;margin:8vh auto;background:white;border-radius:24px;padding:28px;box-shadow:0 16px 48px #14382d18}input,button{box-sizing:border-box;width:100%;padding:14px;margin:10px 0;border-radius:12px;border:1px solid #b9d4cb;font:inherit}button{background:#147a68;color:white;cursor:pointer}label{display:block;margin-top:12px}p{line-height:1.6}small{color:#526962}</style><main><h1>连接游园 MCP</h1><p>${escape(client.client_name)} 请求访问 <b>${escape(device)}</b>。</p><p>权限：${scopes.includes('mcp:control')?'管理插件、修改文件及调用工具':'只读状态与开发文档'}。仅管理员或有效订阅用户可以使用，节点权限另行核验。</p><form method="POST" action="/oauth/authorize"><input type="hidden" name="transaction" value="${tx}"><label>游园账号<input name="username" autocomplete="username" required maxlength="32"></label><label>密码<input name="password" type="password" autocomplete="current-password" required maxlength="128"></label><button name="consent" value="allow">登录并同意授权</button><button name="consent" value="deny" formnovalidate>取消</button></form><small>密码只提交给游园，不发送给 ChatGPT。忘记密码可由项目管理员重置。授权登录不会更改游戏账号或插件。</small></main></html>`;
   return new Response(page,{headers:{'Content-Type':'text/html; charset=utf-8','Cache-Control':'no-store','Content-Security-Policy':"default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'",'Referrer-Policy':'no-referrer','X-Content-Type-Options':'nosniff'}});
  }
  if(request.method!=='POST')return error('invalid_request','方法不支持',405);
  const form=await request.formData(),key='oauth-consent:'+await hash(String(form.get('transaction')||'')),tx=await s.get(key);if(!tx||tx.expires<=now||tx.issuer!==origin)return error('invalid_request','授权页面已失效，请重新连接');
  const target=new URL(tx.redirect);if(tx.state!==null)target.searchParams.set('state',tx.state);target.searchParams.set('iss',origin);
  if(form.get('consent')!=='allow'){await s.delete(key);target.searchParams.set('error','access_denied');return Response.redirect(target,302)}
  const login=await identityHandle(new Request('https://identity.internal/__identity/login',{method:'POST',headers:{'X-Identity-IP':request.headers.get('X-Identity-IP')||'local'},body:JSON.stringify({username:form.get('username'),password:form.get('password')})}),ctx,env);
  const data=await login.json();if(!data.ok)return error('access_denied',data.error,login.status);
  // This login is only used to authenticate consent: do not retain an extra website session.
  await s.delete('session:'+await hash(data.session_token));
  if(!allowedDevice(data.user,tx.device))return error('access_denied','订阅未启用或未获该节点权限',403);
  const code=token();const accepted=await s.transaction(async store=>{const current=await store.get(key);if(!current||current.expires<=Date.now())return false;await store.delete(key);await store.put('oauth-code:'+await hash(code),{...tx,user:data.user.id,expires:Date.now()+300000});return true});if(!accepted)return error('invalid_request','授权已使用');target.searchParams.set('code',code);return Response.redirect(target,302);
 }
 if(action==='token'){
  if(request.method!=='POST')return error('invalid_request','需要 POST',405);if(!await rate('token',120))return error('slow_down','兑换过于频繁',429);
  const p=await request.formData(),id=String(p.get('client_id')||''),client=await s.get('oauth-client:'+id);if(!client||client.issuer!==origin)return error('invalid_client','客户端未注册',401);
  const kind=p.get('grant_type'),resource=String(p.get('resource')||'');
  if(kind==='authorization_code'){
   const key='oauth-code:'+await hash(String(p.get('code')||'')),g=await s.get(key),verifier=String(p.get('code_verifier')||'');
   if(!g||g.expires<=now||g.issuer!==origin||g.client!==id||g.redirect!==p.get('redirect_uri')||g.resource!==resource||!/^[A-Za-z0-9._~-]{43,128}$/.test(verifier)||b64(await crypto.subtle.digest('SHA-256',enc.encode(verifier)))!==g.challenge)return error('invalid_grant','授权码或 PKCE/资源不匹配');
   const user=await principal(g.user);if(!user||!allowedDevice(user,g.device))return error('access_denied','节点权限已失效',403);
   const result=await s.transaction(async store=>{const current=await store.get(key);if(!current||current.expires<=Date.now())return null;await store.delete(key);const family=token();await store.put('oauth-family:'+family,{user:g.user,revoked:false});return issuance(store,{user:g.user,client:id,issuer:origin,resource,device:g.device,scope:g.scope,family})});return result?json(result):error('invalid_grant','授权码已经使用');
  }
  if(kind==='refresh_token'){
   const key='oauth-refresh:'+await hash(String(p.get('refresh_token')||'')),g=await s.get(key);if(!g||g.client!==id||g.issuer!==origin||g.resource!==resource)return error('invalid_grant','刷新令牌或资源不匹配');
   if(g.used){await s.put('oauth-family:'+g.family,{user:g.user,revoked:true});return error('invalid_grant','重复使用刷新令牌，整组授权已撤销')}
   if(!await validGrant(g,resource))return error('invalid_grant','授权已失效');
   const result=await s.transaction(async store=>{const cur=await store.get(key);if(cur.used){await store.put('oauth-family:'+g.family,{user:g.user,revoked:true});return null}await store.put(key,{...cur,used:true});return issuance(store,g)});return result?json(result):error('invalid_grant','刷新令牌已使用');
  }
  return error('unsupported_grant_type','仅支持授权码和刷新令牌');
 }
 if(action==='revoke'){
  if(request.method!=='POST')return error('invalid_request','需要 POST',405);const p=await request.formData(),h=await hash(String(p.get('token')||'')),g=await s.get('oauth-access:'+h)||await s.get('oauth-refresh:'+h);if(g&&g.client===p.get('client_id')&&g.issuer===origin)await s.put('oauth-family:'+g.family,{user:g.user,revoked:true});return json({});
 }
 return error('invalid_request','未知授权操作',404);
}
