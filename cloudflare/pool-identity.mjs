// Never accept a tenant or administrator claim from browser-supplied headers.
export function poolActor(identity,owner=false,legacy=false){
 const user=identity?.user;
 const id=owner||legacy||user?.legacy_owner?'owner':user?.id;
 if(!id)return null;
 return {id,admin:owner||legacy||!!user?.legacy_owner||user?.role==='admin',scope:identity?.scope||'control',expires:Date.now()+30000};
}
export function encodePoolActor(actor){return btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(actor)))).replaceAll('+','-').replaceAll('/','_').replace(/=+$/,'')}
export async function signPoolActor(raw,hex){
 if(!/^[a-f0-9]{64}$/.test(hex||''))throw Error('设备账号池身份尚未就绪');
 const key=await crypto.subtle.importKey('raw',Uint8Array.from(hex.match(/../g),v=>parseInt(v,16)),{name:'HMAC',hash:'SHA-256'},false,['sign']);
 return [...new Uint8Array(await crypto.subtle.sign('HMAC',key,new TextEncoder().encode(raw)))].map(x=>x.toString(16).padStart(2,'0')).join('');
}
const rootReads=new Set(['/health','/api/youyuan/catalog','/api/youyuan/sdk/catalog','/api/youyuan/skills/library','/api/youyuan/docs','/api/youyuan/account-pool','/api/youyuan/accounts']);
const docTools=new Set(['api_youyuan_catalog','api_youyuan_sdk_catalog','api_youyuan_docs','api_youyuan_skills_library']);
export function sharedPoolAllowed(path,method,packet){
 if(path.startsWith('/api/youyuan/accounts/invoke/'))return !path.startsWith('/api/youyuan/accounts/invoke/main/');
 if(path==='/api/youyuan/account-pool'||path==='/api/youyuan/accounts')return true;
 if(method==='GET'&&rootReads.has(path))return true;
 if(!path.startsWith('/mcp'))return false;
 if(method==='GET')return true;
 if(['initialize','ping','notifications/initialized','tools/list','resources/list','resources/templates/list'].includes(packet?.method))return true;
 if(packet?.method==='resources/read'){const uri=String(packet.params?.uri||'');return uri.startsWith('youyuan://docs/')||uri.startsWith('youyuan://skill/')||['youyuan://skills/catalog','youyuan://api/catalog'].includes(uri)}
 if(packet?.method!=='tools/call')return false;
 const {name,arguments:a={}}=packet.params||{};
 if(name==='api_youyuan_account_pool'||name==='api_youyuan_accounts')return !a.account||a.account==='main';
 if(a.account&&a.account!=='main')return true; // Native prefix verifies ownership.
 if(docTools.has(name)&&(a.method||'GET')==='GET')return true;
 if(name==='api_call')return sharedPoolAllowed(String(a.path||''),String(a.method||'GET'),null);
 return false;
}
