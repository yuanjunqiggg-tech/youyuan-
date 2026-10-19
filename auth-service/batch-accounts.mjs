import {seal,open} from './vault.mjs';
const publicJob=j=>({id:j.id,count:j.count,completed:j.completed.length,state:j.state,name_prefix:j.name_prefix,nickname_prefix:j.nickname_prefix,pending_id:j.pending_id||null,pending_nickname_id:j.pending_nickname_id||null,require_nickname:!!j.require_nickname,created:j.created,last_error:j.last_error||'',verification_required:j.state==='waiting_verification',verification_url:j.verification_url||null,account_ids:j.completed});
async function save(v,j){await v.ctx.storage.put('batch:'+j.id,await seal(v.env,'batch:'+j.id,j))}
export async function batchAccountAction(v,input){
 if(input.action==='batch_list'){
  const out=[];for(const [key,value]of await v.ctx.storage.list({prefix:'batch:'}))out.push(publicJob(await open(v.env,key,value)));
  return out.sort((a,b)=>b.created-a.created).slice(0,20);
 }
 if(input.action==='batch_create'){
  const count=Number(input.count),prefix=String(input.nickname_prefix||'').trim();
  if(!Number.isInteger(count)||count<1||count>10)throw Error('每批请选择1–10个账号');
  if([...prefix].length<2||[...prefix].length>16)throw Error('昵称前缀需为2–16个字符');
  if((await v.rows()).length+count>32)throw Error('当前资料库剩余位置不足，请减少本批数量');
  const jobs=await batchAccountAction(v,{action:'batch_list'});
  if(jobs.filter(j=>!['completed','cancelled'].includes(j.state)).length>=4)throw Error('请先完成或取消已有批量任务');
  const j={id:crypto.randomUUID(),count,name_prefix:String(input.name_prefix||prefix).slice(0,40),nickname_prefix:prefix,require_nickname:input.require_nickname===true,state:'ready',completed:[],created:Date.now(),pending_id:null};
  await save(v,j);return {ok:true,batch:publicJob(j),official_sms_reuse_guaranteed:false};
 }
 const key='batch:'+String(input.id||''),raw=await v.ctx.storage.get(key);if(!raw)throw Error('批量任务不存在');
 const j=await open(v.env,key,raw);
 if(input.action==='batch_name'){
  if(j.state!=='waiting_nickname'||!j.pending_nickname_id)throw Error('本批没有待命名账号');
  return v.accountSerial(j.pending_nickname_id,async()=>{
  const nickname=String(input.nickname||'').trim();if([...nickname].length<2||[...nickname].length>20)throw Error('游戏昵称需为2–20个字符');
  const row=await v.get(j.pending_nickname_id);if(!row)throw Error('待命名账号已被撤销');
  const result=await v.kernel({action:'nickname',client:row.client,nickname});
  row.client=result.client;row.nickname=result.nickname||nickname;row.name=row.nickname;await v.put(row);
  j.completed.push(row.id);j.pending_nickname_id=null;j.state=j.completed.length===j.count?'completed':'ready';j.last_error='';await save(v,j);
  return {ok:true,batch:publicJob(j),account_id:row.id};
  });
 }
 if(input.action==='batch_cancel'){
  if(j.pending_id)await v.ctx.storage.delete('pending:'+j.pending_id);
  j.pending_id=null;j.state='cancelled';await save(v,j);return {ok:true,batch:publicJob(j)};
 }
 if(input.action!=='batch_next')throw Error('未知批量操作');
 if(['completed','cancelled','waiting_nickname'].includes(j.state))return {ok:true,batch:publicJob(j)};
 if(j.state==='waiting_verification'&&input.verification_completed!==true)return {ok:true,batch:publicJob(j)};
 const rate=await v.ctx.storage.get('batch-rate')||0;
 if(Date.now()-rate<1500)throw Error('当前账号处理中，请稍后继续');
 await v.ctx.storage.put('batch-rate',Date.now());
 const suffix=String(j.completed.length+1).padStart(2,'0');let pending=null;
 if(j.pending_id){const saved=await v.ctx.storage.get('pending:'+j.pending_id);if(!saved||saved.expires<Date.now()){j.state='failed';j.last_error='当前验证已过期，请取消本批并创建剩余任务';await save(v,j);return {ok:true,batch:publicJob(j)}}pending=await open(v.env,'pending:'+j.pending_id,saved.sealed)}
 try{
  const response=await v.create({action:'register_guest',name:j.name_prefix+suffix,nickname:j.nickname_prefix+suffix,allow_project_admin:true},pending);
  const data=await response.json();if(!response.ok||!data.ok)throw Error(data.error||'官方注册未完成');
  if(j.completed.includes(data.account.id))throw Error('同一账号重复返回，不计作新账号');
  if(j.require_nickname){j.pending_nickname_id=data.account.id;j.state='waiting_nickname'}else{j.completed.push(data.account.id);j.state=j.completed.length===j.count?'completed':'ready'}
  j.pending_id=null;j.verification_url=null;j.last_error='';
 }catch(e){
  j.last_error=e.message.split(' verify_url=')[0];j.pending_id=e.detail?.pending_id||j.pending_id;
  j.verification_url=e.detail?.verification_url||null;j.state=e.detail?.verification_required?'waiting_verification':'failed';
 }
 await save(v,j);return {ok:true,batch:publicJob(j),official_sms_reuse_guaranteed:false};
}
