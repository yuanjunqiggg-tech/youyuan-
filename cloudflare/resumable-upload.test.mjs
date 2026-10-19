import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {webcrypto,createHash} from 'node:crypto';
const source=readFileSync(new URL('../upstream/frontend/js/youyuan_upload.js',import.meta.url),'utf8');
const CHUNK=1024*1024,sha=data=>createHash('sha256').update(data).digest('hex');
const copy=value=>JSON.parse(JSON.stringify(value));
// This is a transport/receipt simulator. It does not claim Linux persistence,
// tunnel throughput, filesystem durability, Android picker or game import proof.
function server(options={}){
 const tickets=new Map(),calls=[],commits=[],lost={...options.lose},api={tickets,calls,commits,rejectOffset:null};
 const response=(status,body)=>({ok:status>=200&&status<300,status,json:async()=>copy(body)});
 api.fetch=async function(path,request){
  const url=new URL(path,'https://toolbox.test'),method=request.method,id=url.searchParams.get('id'),prefix=url.pathname,key=prefix+'|'+id;calls.push({method,path});
  let t=tickets.get(key),kind;
  if(method==='POST'&&!id){const input=JSON.parse(request.body);const prior=tickets.get(prefix+'|'+input.id);if(prior&&(prior.name!==input.name||prior.size!==input.size))return response(409,{ok:false,error:'ticket belongs to another file'});t=prior||{id:input.id,name:input.name,size:input.size,offset:0,chunks:[],complete:false,data:[]};tickets.set(prefix+'|'+input.id,t);kind='begin'}
  else{
   if(!t)return response(404,{ok:false,error:'ticket not found'});
   if(method==='PUT'){
    const offset=Number(url.searchParams.get('offset'));if(api.rejectOffset!=null&&offset>=api.rejectOffset)return response(503,{ok:false,error:'transport temporarily unavailable'});
    const data=Buffer.from(await request.body.arrayBuffer()),digest=url.searchParams.get('sha256');if(sha(data)!==digest)return response(409,{ok:false,error:'chunk hash mismatch'});
    if(offset<t.offset){const prior=t.chunks.find(c=>c.offset===offset);if(!prior||prior.sha256!==digest||prior.size!==data.length)return response(409,{ok:false,error:'different bytes already received'})}
    else{if(offset!==t.offset)return response(409,{ok:false,error:'wrong offset'});t.chunks.push({offset,size:data.length,sha256:digest});t.data.push(data);t.offset+=data.length;commits.push({id:t.id,offset,size:data.length,prefix})}kind='put';
   }else if(method==='POST'){
    if(t.offset!==t.size)return response(409,{ok:false,error:'not complete'});t.complete=true;t.path='/uploads/'+t.id+'-'+t.name;t.sha256=sha(Buffer.concat(t.data));kind='finish';
   }else if(method==='GET')kind='state';else return response(405,{ok:false,error:'method not allowed'});
  }
  if(lost[kind]>0){lost[kind]--;throw new TypeError('simulated response lost after server commit')}
  const result=copy({...t,data:undefined});if(options.changeState&&kind==='state')options.changeState(result);
  return response(200,{ok:true,upload:result});
 };
 return api;
}
function client(api,storage=new Map(),account='main',options={}){
 const timers=new Set(),context=vm.createContext({Blob,File,URLSearchParams,Uint8Array,AbortController,crypto:webcrypto,location:{origin:'https://toolbox.test',search:'?device=linux-server'},yyPlatformUser:options.user===null?null:(options.user||{id:'fixture-owner'}),sessionStorage:{getItem:()=>options.session||'fixture-session'},localStorage:{getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,value),removeItem:key=>storage.delete(key)},yyActiveAccount:account,yyAccountPath:(path,id)=>'/api/youyuan/accounts/invoke/'+id+path,fetch:api.fetch,window:{},setTimeout:(fn,ms)=>{const timer=setTimeout(fn,ms===45000?45000:0);timers.add(timer);return timer},clearTimeout:id=>{clearTimeout(id);timers.delete(id)}});vm.runInContext(source,context);return {context,storage,upload:context.window.yyUploadFile,cleanup(){for(const timer of timers)clearTimeout(timer)}};
}
function fixture(size=CHUNK*3+31,name='test.mcstructure'){const data=Buffer.alloc(size);for(let i=0;i<data.length;i++)data[i]=(i*31+Math.floor(i/CHUNK))%251;return {data,file:new File([data],name,{lastModified:1700000000000})}}
for(const kind of ['begin','put','finish'])test('simulated lost '+kind+' response recovers the receipt without duplicate file bytes',async()=>{
 const api=server({lose:{[kind]:1}}),c=client(api),{data,file}=fixture();try{const result=await c.upload(file);assert.equal(result.ok,true);assert.equal(result.bytes,data.length);assert.equal(result.sha256,sha(data));assert.equal(api.tickets.size,1);assert.equal(api.commits.length,Math.ceil(data.length/CHUNK));assert.deepEqual(Buffer.concat([...api.tickets.values()][0].data),data);assert.ok(api.calls.every(call=>call.path.includes('/api/youyuan/files/transfer')))}finally{c.cleanup()}
});
test('a recreated client reselects the same File and continues after acknowledged bytes',async()=>{
 const api=server(),storage=new Map(),first=client(api,storage),{data,file}=fixture(CHUNK*4+11);api.rejectOffset=CHUNK*2;
 try{await assert.rejects(()=>first.upload(file),/暂停|中断/)}finally{first.cleanup()}
 assert.equal([...api.tickets.values()][0].offset,CHUNK*2);assert.equal(storage.size,1);api.rejectOffset=null;const next=client(api,storage);
 try{const result=await next.upload(new File([data],file.name,{lastModified:file.lastModified}));assert.equal(result.sha256,sha(data));assert.equal(api.tickets.size,1);assert.equal(api.commits.filter(c=>c.offset===0).length,1);assert.equal(api.commits.filter(c=>c.offset===CHUNK).length,1)}finally{next.cleanup()}
});
test('changed bytes inside the acknowledged prefix are rejected before any new PUT',async()=>{
 const api=server(),storage=new Map(),{data,file}=fixture(CHUNK*4),first=client(api,storage);api.rejectOffset=CHUNK*2;
 try{await assert.rejects(()=>first.upload(file),/暂停|中断/)}finally{first.cleanup()}
 const changed=Buffer.from(data);changed[CHUNK+123]^=255;const before=api.commits.length;api.rejectOffset=null;const next=client(api,storage);
 try{await assert.rejects(()=>next.upload(new File([changed],file.name,{lastModified:file.lastModified})),/本地文件.*不同|文件.*不同|不一致/);assert.equal(api.commits.length,before)}finally{next.cleanup()}
});
test('upload routing and persisted receipts are isolated for two selected game accounts',async()=>{
 const api=server(),storage=new Map(),{data,file}=fixture(CHUNK+3),a=client(api,storage,'account-a'),b=client(api,storage,'account-b');try{const x=await a.upload(file),y=await b.upload(file);assert.notEqual(x.path,y.path);assert.equal(api.tickets.size,2);assert.equal(storage.size,2);assert.equal(api.commits.filter(c=>c.prefix.includes('/invoke/account-a/')).reduce((n,c)=>n+c.size,0),data.length);assert.equal(api.commits.filter(c=>c.prefix.includes('/invoke/account-b/')).reduce((n,c)=>n+c.size,0),data.length)}finally{a.cleanup();b.cleanup()}
});
test('a completed receipt can be reselected without writing any bytes again',async()=>{
 const api=server(),storage=new Map(),{data,file}=fixture(CHUNK+17),first=client(api,storage);try{await first.upload(file)}finally{first.cleanup()}
 const before=api.commits.length,next=client(api,storage);try{const result=await next.upload(file);assert.equal(result.ok,true);assert.equal(result.sha256,sha(data));assert.equal(api.commits.length,before);assert.equal(api.tickets.size,1)}finally{next.cleanup()}
});
test('a stale receipt missing from the server starts a new upload instead of returning its old path',async()=>{
 const api=server(),storage=new Map(),{file}=fixture(CHUNK+7),first=client(api,storage);let prior;try{prior=await first.upload(file)}finally{first.cleanup()}api.tickets.clear();const next=client(api,storage);try{const result=await next.upload(file);assert.notEqual(result.path,prior.path);assert.equal(api.tickets.size,1)}finally{next.cleanup()}
});


for(const [field,value] of [['sha256',''],['path',''],['offset',0],['size',1]])test('completed cached receipt rejects malformed '+field+' rather than returning success',async()=>{
 let corrupt=false;const api=server({changeState:t=>{if(corrupt&&t.complete)t[field]=value}}),storage=new Map(),{file}=fixture(CHUNK+5),first=client(api,storage);try{await first.upload(file)}finally{first.cleanup()}corrupt=true;const next=client(api,storage);
 try{await assert.rejects(()=>next.upload(file),/校验|不一致|未确认|字节位置/)}finally{next.cleanup()}
});
test('stable authenticated user ID resumes across renewed login session tokens',async()=>{
 const api=server(),storage=new Map(),{file}=fixture(CHUNK*4+7);api.rejectOffset=CHUNK*2;const first=client(api,storage,'main',{user:{id:'same-user'},session:'old-session'});
 try{await assert.rejects(()=>first.upload(file),/暂停/)}finally{first.cleanup()}api.rejectOffset=null;const next=client(api,storage,'main',{user:{id:'same-user'},session:'renewed-session'});
 try{await next.upload(file);assert.equal(api.tickets.size,1);assert.equal(api.commits.filter(c=>c.offset===0).length,1)}finally{next.cleanup()}
});
test('different authenticated users cannot reuse each other local receipt even for the same filename and device',async()=>{
 const api=server(),storage=new Map(),{file}=fixture(1000),first=client(api,storage,'main',{user:{id:'user-a'}}),second=client(api,storage,'main',{user:{id:'user-b'}});
 try{const a=await first.upload(file),b=await second.upload(file);assert.notEqual(a.path,b.path);assert.equal(storage.size,2)}finally{first.cleanup();second.cleanup()}
});
test('changing selected game account after a committed chunk pauses rather than returning a path for another account',async()=>{
 const api=server(),original=api.fetch,{file}=fixture(CHUNK*3);let c;
 api.fetch=async(path,request)=>{const result=await original(path,request);if(request.method==='PUT')c.context.yyActiveAccount='new-account';return result};c=client(api);
 try{await assert.rejects(()=>c.upload(file),/账号已切换/);assert.equal([...api.tickets.values()][0].complete,false);assert.equal(api.calls.some(call=>call.path.includes('/invoke/new-account/')),false)}finally{c.cleanup()}
});
test('host file upload prevents a double click from issuing duplicate transfers',async()=>{
 let resolve;const pending=new Promise(r=>resolve=r),fields={input:{files:[{name:'file.txt'}]},status:{textContent:''},button:{disabled:false}},requests=[];
 const context=vm.createContext({Array,yyReadScope:()=> 'same',yyUploadFile:(file,options)=>{requests.push(file);options.onProgress({name:file.name,message:'writing',offset:0,total:10});return pending},T(){},yyRefreshHostFiles:async()=>{},document:{getElementById:id=>({'yy-host-file-input':fields.input,'yy-host-file-status':fields.status,'yy-host-file-submit':fields.button}[id])}});
 const host=readFileSync(new URL('../upstream/frontend/js/youyuan_host_files.js',import.meta.url),'utf8');vm.runInContext(host.slice(host.indexOf('var yyHostUploadBusy=')),context);
 const first=context.yyUploadHostFiles();await context.yyUploadHostFiles();assert.equal(requests.length,1);assert.equal(fields.input.disabled,true);assert.equal(fields.button.disabled,true);resolve({ok:true,name:'file.txt',bytes:10});await first;assert.equal(fields.button.disabled,false);assert.equal(fields.input.disabled,false);assert.equal(context.yyHostUploadBusy,false);
});
test('host file list ignores a delayed response after the selected account changes',async()=>{
 let resolve,scope='first',touches=0;const pending=new Promise(r=>resolve=r),list={replaceChildren(){touches++}},location={textContent:''};
 const context=vm.createContext({yyReadScope:()=>scope,A:()=>pending,T(){},document:{getElementById:id=>id==='yy-host-file-list'?list:location}}),host=readFileSync(new URL('../upstream/frontend/js/youyuan_host_files.js',import.meta.url),'utf8');vm.runInContext(host.slice(host.indexOf('async function yyRefreshHostFiles'),host.indexOf('var yyHostUploadBusy=')),context);
 const result=context.yyRefreshHostFiles();scope='second';resolve({ok:true,items:[],platform:'first account',directory:'/first-account'});await result;assert.equal(touches,0);assert.equal(location.textContent,'');
});
