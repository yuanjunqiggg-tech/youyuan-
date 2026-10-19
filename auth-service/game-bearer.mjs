import {hash} from './vault.mjs';
import {identityRoute} from '../cloudflare/identity.mjs';

const vault=(env,owner)=>env.AUTH.get(env.AUTH.idFromName('account:'+owner));
const call=(env,owner,path,data)=>vault(env,owner).fetch(new Request('https://vault'+path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)}));
export async function bearerDirectory(env,action,kind,key,value){
 const response=await call(env,'game-bearer-directory','/__bearer-directory',{action,kind,key,value});
 if(!response.ok)throw Error('游戏令牌路由暂不可用');
 return response.json();
}
export async function indexGameToken(env,owner,row){
 if(!owner||!row?.login_token)return;
 await bearerDirectory(env,'put','token',await hash(row.login_token),{owner,id:row.id});
}
async function checkOwner(env,owner,digest){
 const response=await call(env,owner,'/__bearer/resolve',{digest});
 if(!response.ok)return null;
 const data=await response.json();return data.id?{owner,id:data.id}:null;
}
export async function resolveGameBearer(env,raw,clientKey='internal'){
 // Token possession is a game capability, never a platform-management login.
 // No token is placed in URLs, directory records or diagnostic messages.
 if(typeof raw!=='string'||!/^yy\/\/[A-Za-z0-9_-]{16,256}$/.test(raw))return null;
 const digest=await hash(raw),cached=await bearerDirectory(env,'get','token',digest);
 if(cached.value){const found=await checkOwner(env,cached.value.owner,digest);if(found)return found;}
 if((await bearerDirectory(env,'get','negative',digest)).value)return null;
 const legacy=await checkOwner(env,'owner',digest);
 if(legacy){await bearerDirectory(env,'put','token',digest,legacy);return legacy;}
 // Pre-index releases kept token hashes only inside owner vaults. Migrate on a
 // cache miss using the internal identity service, without exposing accounts.
 await bearerDirectory(env,'migration_start','migration',digest,{client_key:await hash(clientKey)});
 try{
 const listed=await identityRoute(new Request('https://identity/api/identity/users',{headers:{Authorization:'Bearer '+env.CONSOLE_TOKEN}}),env);
 if(!listed.ok)throw Error('游戏令牌索引迁移暂不可用，请稍后重试');
 const users=(await listed.json()).users||[],owners=[...new Set(users.map(u=>u.legacy_owner?'owner':u.id))].filter(x=>x&&x!=='owner');
 for(let start=0;start<owners.length;start+=6){
  const found=(await Promise.all(owners.slice(start,start+6).map(owner=>checkOwner(env,owner,digest)))).find(Boolean);
  if(found){await bearerDirectory(env,'put','token',digest,found);return found;}
 }
 await bearerDirectory(env,'put','negative',digest,{expires:Date.now()+30000});
 return null;
 }finally{await bearerDirectory(env,'migration_end','migration',digest);}
}
