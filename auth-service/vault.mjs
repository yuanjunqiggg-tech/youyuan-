const enc=new TextEncoder(),dec=new TextDecoder();
const hex=b=>[...new Uint8Array(b)].map(v=>v.toString(16).padStart(2,'0')).join('');
export const token=()=>hex(crypto.getRandomValues(new Uint8Array(32)));
export const hash=async s=>hex(await crypto.subtle.digest('SHA-256',enc.encode(s)));
async function key(env){if(!/^[a-f0-9]{64}$/.test(env.AUTH_VAULT_KEY||''))throw new Error('认证资料库密钥尚未配置');const bytes=new Uint8Array(env.AUTH_VAULT_KEY.match(/../g).map(x=>parseInt(x,16)));return crypto.subtle.importKey('raw',bytes,'AES-GCM',false,['encrypt','decrypt']);}
export async function seal(env,context,value){const iv=crypto.getRandomValues(new Uint8Array(12));const ciphertext=await crypto.subtle.encrypt({name:'AES-GCM',iv,additionalData:enc.encode(context)},await key(env),enc.encode(JSON.stringify(value)));return{iv:[...iv],data:btoa(String.fromCharCode(...new Uint8Array(ciphertext)))};}
export async function open(env,context,value){const bytes=Uint8Array.from(atob(value.data),c=>c.charCodeAt(0));const plaintext=await crypto.subtle.decrypt({name:'AES-GCM',iv:new Uint8Array(value.iv),additionalData:enc.encode(context)},await key(env),bytes);return JSON.parse(dec.decode(plaintext));}
