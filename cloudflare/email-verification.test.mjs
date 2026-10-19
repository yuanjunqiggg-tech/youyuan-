import test from 'node:test';
import assert from 'node:assert/strict';
import {identityHandle} from './identity.mjs';
import {emailDigest, verificationKey, mailSignature} from './email-verification.mjs';

function fixture(){
 const data=new Map(),letters=[],storage={get:async k=>structuredClone(data.get(k)),put:async(k,v)=>data.set(k,structuredClone(v)),delete:async keys=>{for(const k of Array.isArray(keys)?keys:[keys])data.delete(k)},list:async(options={})=>new Map([...data].filter(([k])=>k.startsWith(options.prefix||''))),transaction:async f=>f(storage)};
 const env={CONSOLE_TOKEN:'fixture-mail-signing-secret-with-32-characters',RELAY:{idFromName:n=>n,get:n=>({fetch:async r=>{assert.equal(n,'linux-server');const body=await r.text(),stamp=r.headers.get('X-YouYuan-Mail-Timestamp');assert.equal(r.headers.get('X-YouYuan-Mail-Signature'),await mailSignature(env.CONSOLE_TOKEN,stamp,body));letters.push(JSON.parse(body));return Response.json({ok:true})}})}};
 return {data,letters,ctx:{storage},env};
}
const base={username:'email_test',email:'123456789@qq.com',password:'correct-fixture-password'};
async function invoke(f,action,body={},token=''){return identityHandle(new Request('https://identity/__identity/'+action,{method:'POST',headers:{Authorization:'Bearer '+token,'X-Identity-IP':'fixture'},body:JSON.stringify(body)}),f.ctx,f.env)}
async function register(f,body=base){assert.equal((await invoke(f,'register-code',body)).status,200);const letter=f.letters.at(-1);const r=await invoke(f,'register',{...body,email_code:letter.code});assert.equal(r.status,200);return r.json()}

test('real delivery acknowledgement precedes registration, OTP hash only, code is consumed',async()=>{
 const f=fixture();assert.equal((await invoke(f,'register',{...base,qq:'12345678',email_code:'111111'})).status,400);
 const r=await invoke(f,'register-code',{...base,email:' 123456789@QQ.COM '});assert.equal(r.status,200);const payload=await r.json();assert.equal(payload.code,undefined);
 const code=f.letters[0].code;assert.match(code,/^\d{6}$/);const saved=f.data.get(await verificationKey(base.username,base.email));assert.ok(saved.sent);assert.equal(saved.code,undefined);assert.notEqual(saved.hash,code);
 const created=await (await invoke(f,'register',{...base,email_code:code})).json();assert.ok(created.ok);assert.equal(created.user.email,base.email);assert.equal(created.user.email_verified,true);assert.equal(f.data.has(await verificationKey(base.username,base.email)),false);
 assert.equal((await invoke(f,'register',{...base,email_code:code})).status,409);
});
test('SMTP/relay failure cannot create a user or activate a code',async()=>{
 const f=fixture();f.env.RELAY.get=()=>({fetch:async()=>Response.json({ok:false},{status:503})});const r=await invoke(f,'register-code',base);assert.equal(r.status,503);assert.match((await r.json()).error,/未发出/);assert.equal(f.data.has(await verificationKey(base.username,base.email)),false);assert.equal([...f.data.keys()].some(k=>k.startsWith('user:')),false);
});
test('QQ mailbox format and per-mailbox cooldown are enforced',async()=>{
 const f=fixture();for(const email of ['12345678','a@evil.test','a@qq.com\r\nBcc: b@qq.com'])assert.equal((await invoke(f,'register-code',{...base,email})).status,400);
 assert.equal((await invoke(f,'register-code',base)).status,200);assert.equal((await invoke(f,'register-code',{...base,username:'another_user'})).status,429);assert.equal(f.letters.length,1);
});
test('expiry and wrong-code attempt limit are enforced server-side',async()=>{
 const f=fixture();await invoke(f,'register-code',base);const key=await verificationKey(base.username,base.email),saved=f.data.get(key),good=f.letters[0].code,bad=good==='000000'?'000001':'000000';
 for(let n=0;n<5;n++)assert.equal((await invoke(f,'register',{...base,email_code:bad})).status,400);assert.equal((await invoke(f,'register',{...base,email_code:good})).status,429);
 saved.attempts=0;saved.expires=Date.now()-1;f.data.set(key,saved);assert.equal((await invoke(f,'register',{...base,email_code:good})).status,400);
});
test('OTP belongs to exact normalized name and mailbox',async()=>{
 const f=fixture();await invoke(f,'register-code',base);const code=f.letters[0].code;
 assert.equal((await invoke(f,'register',{...base,username:'other_user',email_code:code})).status,400);
 assert.equal((await invoke(f,'register',{...base,email:'another@qq.com',email_code:code})).status,400);
 assert.equal((await invoke(f,'register',{...base,username:' EMAIL_TEST ',email_code:code})).status,200);
});
test('self deletion revokes all sessions OAuth grants and aliases; same name and email can register again',async()=>{
 const f=fixture(),account=await register(f),id=account.user.id;
 for(const [key,value] of [['name:legacy_alias',id],['email:alias@qq.com',id],['session:other',{user:id}],['oauth-access:other',{user:id}],['oauth-refresh:other',{user:id}],['mcp-link:other',{user:id}],['user:untouched',{id:'untouched'}]])f.data.set(key,value);
 assert.equal((await invoke(f,'delete',{password:'wrong'},account.session_token)).status,403);
 const deleted=await invoke(f,'delete',{password:base.password},account.session_token);assert.equal(deleted.status,200);assert.ok(f.data.has('user:untouched'));
 assert.equal([...f.data].some(([k,v])=>k==='user:'+id||v===id||v?.user===id),false);
 assert.equal((await invoke(f,'binding',{},account.control_token)).status,401);
 const rate=f.data.get('rate:mailbox:'+await emailDigest(base.email));rate.last=Date.now()-60001;f.data.set('rate:mailbox:'+await emailDigest(base.email),rate);
 const replacement=await register(f);assert.notEqual(replacement.user.id,id);
});
test('logout leaves an account intact and its device binding valid',async()=>{
 const f=fixture(),account=await register(f);assert.equal((await invoke(f,'logout',{},account.session_token)).status,200);assert.ok(f.data.has('user:'+account.user.id));assert.equal((await invoke(f,'binding',{},account.control_token)).status,200);assert.equal((await invoke(f,'register-code',base)).status,409);
});
test('orphan name/email indexes are repaired without deleting real accounts',async()=>{
 const f=fixture();f.data.set('name:'+base.username,'deleted-id');f.data.set('email:'+base.email,'deleted-id');const account=await register(f);assert.equal(f.data.get('name:'+base.username),account.user.id);assert.equal(f.data.get('email:'+base.email),account.user.id);assert.equal((await invoke(f,'register-code',base)).status,409);
});
test('owner cannot self delete and does not lose its credentials',async()=>{
 const f=fixture(),account=await register(f),u=f.data.get('user:'+account.user.id);u.legacy_owner=true;f.data.set('user:'+u.id,u);assert.equal((await invoke(f,'delete',{password:base.password},account.session_token)).status,403);assert.ok(f.data.has('user:'+u.id));
});
