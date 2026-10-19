import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
const source=name=>readFileSync(new URL('../upstream/frontend/js/'+name,import.meta.url),'utf8');
const accounts=source('youyuan_accounts.js'),identity=source('youyuan_identity.js');
const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function setup(){
 const fields=new Map(),pages=[],calls=[],toasts=[];let role='admin';
 const field=id=>{if(!fields.has(id))fields.set(id,{value:'',textContent:'',innerHTML:'',hidden:false,isConnected:true,dataset:{},querySelectorAll:()=>[]});return fields.get(id)};
 const context=vm.createContext({URL,Map,Set,Date,JSON,Math,Number,String,Array,Promise,yyEscape:escape,yyReadScope:()=>role,yyPlatformUser:{role:'admin'},yyHeading:()=>'',yyIntro:(a,b)=>'<h2>'+escape(a)+'</h2><p>'+escape(b)+'</p>',yyBadge:String,yyIcon:()=>'',yyAllAccountRows:[],yyAccountRows:[],yyActiveAccount:'main',yyAccountPage:1,yyAccountPageSize:8,yyAccountFocusID:'',yyProjectFilter:'',yyMonitorID:'main',yyObservations:null,subPageCache:{},_savedFormState:{},window:{},T:(...args)=>toasts.push(args),document:{getElementById:field},openSubPage:(title,html)=>pages.push({title,html}),yyIdentityRequest:async(action,body)=>{calls.push({action,body});return {ok:true,users:context.fixtures||[]}},yyRootAPI:async()=>({ok:true}),yyPoolCall:async(action,args)=>{calls.push({action,args});return {accounts:context.poolFixtures||[]}},setTimeout:()=>0});
 vm.runInContext(identity.slice(identity.indexOf('var yySubscriptionUsers='),identity.indexOf('var yySubscriptionSettings=')),context);
 vm.runInContext(accounts.slice(accounts.indexOf('var yyChatPage='),accounts.indexOf('function yyDrawOnline()')),context);
 vm.runInContext(accounts.slice(accounts.indexOf('function yyAccountActionHTML'),accounts.indexOf('var yyEditingProfile=')),context);
 vm.runInContext(accounts.slice(accounts.indexOf('function yyAccountAdmin()'),accounts.indexOf('async function yyCenterCloudAccounts')),context);
 vm.runInContext(accounts.slice(accounts.indexOf('var yyPoolRows='),accounts.indexOf('function yyRenderAuthPending')),context);
 context.yyAccountStatusLabel=a=>a.connection_known===false||a.connected==null?'状态待查询':a.connected?'已连接':'游戏未连接';
 return {context,fields,field,pages,calls,toasts,member(){role='member';context.yyPlatformUser={role:'member'}}};
}
test('subscription directory searches and classifies without rendering every user form',async()=>{
 const p=setup(),now=Date.now();p.context.fixtures=[{id:'a',username:'Alice',email:'a@qq.com',role:'admin'},{id:'b',username:'Bob',email:'b@qq.com',role:'member',subscription_until:now+86400000},{id:'c',username:'Carol',role:'member',subscription_until:now-1},{id:'d',username:'Dan',role:'member'}];
 await p.context.yySubscriptionsPage();assert.equal(p.calls.length,1);assert.equal(p.calls[0].action,'users');assert.doesNotMatch(p.pages[0].html,/sub-role-/);assert.match(p.field('yy-sub-list').innerHTML,/Alice/);
 assert.deepEqual(Array.from(p.context.yySubscriptionMatches(p.context.fixtures,'b@qq','subscribed',now),u=>u.id),['b']);assert.equal(p.context.yySubscriptionState(p.context.fixtures[2],now),'expired');
 p.context.yySubscriptionDetail(1);assert.match(p.pages.at(-1).html,/sub-role-b/);assert.doesNotMatch(p.pages.at(-1).html,/sub-role-a/);assert.match(p.pages.at(-1).html,/重置密码/);assert.equal(p.calls.length,1);
});
test('subscription update keeps the parent directory cache and refreshes its in-memory state',async()=>{
 const p=setup();p.context.yySubscriptionUsers=[{id:'u',username:'user',role:'member'}];p.context.subPageCache['订阅用户管理']='parent-page';p.field('sub-date-u').value='2030-12-31';p.field('sub-role-u').value='member';p.field('sub-node-u').checked=true;
 await p.context.yySubscriptionSave('u',false,p.field('save'));
 assert.equal(p.calls[0].action,'entitlement');assert.deepEqual(Array.from(p.calls[0].body.assigned_devices),['linux-server']);assert.equal(p.context.yySubscriptionUsers[0].cloud_allowed,true);assert.equal(p.context.subPageCache['订阅用户管理'],'parent-page');assert.equal(p.field('save').disabled,false);
 await p.context.yySubscriptionSave('u',true,p.field('save'));assert.equal(p.calls[1].body.subscription_until,0);assert.equal(p.context.yySubscriptionUsers[0].cloud_allowed,false);
});
test('subscription list paginates and escapes user-supplied names',async()=>{
 const p=setup();p.context.fixtures=Array.from({length:45},(_,i)=>({id:'u'+i,username:i===0?'<script>bad</script>':'user'+i,role:'member'}));await p.context.yySubscriptionsPage();const html=p.field('yy-sub-list').innerHTML;
 assert.equal((html.match(/class="yy-compact-row"/g)||[]).length,20);assert.doesNotMatch(html,/<script>/);assert.match(html,/&lt;script&gt;/);p.context.yySubscriptionPage=3;p.context.yyRenderSubscriptions();assert.equal((p.field('yy-sub-list').innerHTML.match(/class="yy-compact-row"/g)||[]).length,5);
});
test('chat supports player, text and inclusive minute filters in chronological order',()=>{
 const p=setup(),rows=[{time:'2026-10-16T10:01:59Z',player:'A',message:'hello'},{time:'2026-10-16T10:02:00Z',player:'A',message:'later'},{time:'2026-10-16T10:00:10Z',player:'B',message:'hello'},{time:'2026-10-16T10:01:10Z',player:'A',message:'HELLO'}];
 const result=p.context.yyChatFilterRows(rows,{player:'A',text:'hello',start:'2026-10-16T10:01:00Z',end:'2026-10-16T10:01:00Z'});assert.equal(result.length,2);assert.equal(result[0].time,rows[0].time);
});
test('chat aggregation counts each real record once and selecting a bar limits messages to its interval',()=>{
 const p=setup();p.context.yyObservations={chat:[{time:'2026-10-16T10:01:00Z',player:'A',message:'one'},{time:'2026-10-16T10:02:00Z',player:'B',message:'two'},{time:'2026-10-16T11:01:00Z',player:'A',message:'three'}]};p.field('yy-chat-interval').value='3600000';p.context.yyDrawChat();
 const groups=p.context.yyChatGroupRows(p.context.yyObservations.chat,3600000);assert.deepEqual(Array.from(groups,g=>g.count),[2,1]);p.context.yyChatChooseBucket(groups[0].start,groups[0].end);assert.match(p.field('yy-chat-table').innerHTML,/one/);assert.match(p.field('yy-chat-table').innerHTML,/two/);assert.doesNotMatch(p.field('yy-chat-table').innerHTML,/three/);assert.equal(p.field('yy-chat-all-buckets').hidden,false);assert.match(p.field('yy-chat-bars').innerHTML,/yyChatChooseBucket/);
});
test('mobile chat renders wrapped vertical messages and paginates without a wide table',()=>{
 const p=setup();p.context.yyObservations={chat:Array.from({length:75},(_,i)=>({time:new Date(1792144800000+i*1000).toISOString(),player:'player',server:'test',message:i===74?'<b>newest</b>':'message-'+i}))};p.context.yyDrawChat();const html=p.field('yy-chat-table').innerHTML;
 assert.equal((html.match(/class="yy-chat-message"/g)||[]).length,50);assert.doesNotMatch(html,/<table|<td|<b>newest/);assert.match(html,/&lt;b&gt;newest/);p.context.yyChatPage=2;p.context.yyDrawChat();assert.equal((p.field('yy-chat-table').innerHTML.match(/class="yy-chat-message"/g)||[]).length,25);
 const css=readFileSync(new URL('../upstream/frontend/css/youyuan_accounts.css',import.meta.url),'utf8');assert.match(css,/\.yy-chat-message p\{[^}]*overflow-wrap:anywhere/);
});
test('local account list is collapsed by default and retains every account action',()=>{
 const p=setup();p.context.yyAllAccountRows=[{id:'main',name:'主账号',connection_known:false},{id:'child',name:'子账号',connected:true,enabled:true}];p.context.yyAccountRows=[p.context.yyAllAccountRows[1]];p.context.yyRenderAccountCards();const html=p.field('yy-accounts-grid').innerHTML;
 assert.equal((html.match(/<details /g)||[]).length,2);assert.doesNotMatch(html,/<details[^>]+ open/);for(const label of ['进入此账号工作区','配置主连接','管理连接','令牌 / 服务器','关闭连接','设备备注 / 分类','复制令牌','观察方式','查看记录'])assert.ok(html.includes(label),label);assert.match(html,/暂无官方头像/);assert.doesNotMatch(html,/default-head\.jpg|设置主账号/);
 p.field('yy-accounts-grid').querySelectorAll=()=>[{getAttribute:()=> 'child'}];p.context.yyRenderAccountCards();assert.match(p.field('yy-accounts-grid').innerHTML,/data-account-id="child"[^>]* open/);
});
test('ordinary users cannot open the administrator pool or trigger its list request',async()=>{
 const p=setup();p.member();await p.context.yyPoolPanel(true);assert.equal(p.calls.length,0);assert.equal(p.pages.length,0);assert.match(p.toasts[0][0],/管理员/);p.context.yyUpdateAdminPoolEntry();assert.equal(p.field('yy-admin-pool-entry').hidden,true);
});
test('administrator pool stays inside app and does not fan out status calls for all bots',async()=>{
 const p=setup();p.context.poolFixtures=Array.from({length:24},(_,i)=>({id:'bot'+i,name:'bot'+i,has_token:true,server:'s',enabled:i%2===0}));await p.context.yyPoolPanel(true);assert.equal(p.pages[0].title,'机器人托管池');assert.equal(p.calls.length,1);assert.equal(p.calls[0].action,'list');assert.equal(p.calls[0].args[0].all,true);assert.equal((p.field('yy-pool-list').innerHTML.match(/class="yy-compact-row"/g)||[]).length,24);
 assert.equal(p.context.yyPoolFilteredRows(p.context.poolFixtures,'','enabled').length,12);p.context.yyPoolDetail(0);assert.equal(p.calls.length,1);assert.match(p.pages.at(-1).html,/查询实时状态/);assert.match(p.pages.at(-1).html,/释放并等待 60 秒/);assert.match(p.pages.at(-1).html,/复制令牌/);
});

test('robot portraits require an official source and exact game or credential identity, never platform or local profile pictures',()=>{
 const p=setup();p.context.yyCenterCloudRows=[{id:'credential',game_id:'12345678901234567890',avatar_source:'netease',avatar:'https://official.example/avatar.png',name:'same-name'}];
 assert.equal(p.context.yyAccountPortrait({id:'main',game_id:'12345678901234567890'}),'https://official.example/avatar.png');
 assert.equal(p.context.yyAccountPortrait({id:'yy_credential'}),'https://official.example/avatar.png');
 assert.equal(p.context.yyAccountPortrait({id:'pool_credential'}),'https://official.example/avatar.png');
 assert.equal(p.context.yyAccountPortrait({id:'main',name:'same-name',avatar:'https://platform.example/person.png'}),'');
 assert.equal(p.context.yyAccountPortrait({id:'main',avatar_source:'netease',avatar:'javascript:alert(1)'}),'');
 assert.equal(p.context.yyAccountPortrait({id:'main',avatar_source:'netease',avatar:'https://user:secret@official.example/avatar.png'}),'');
 let removed=false,label;const img={onerror(){},parentElement:{setAttribute:(key,value)=>label=value},remove(){removed=true}};p.context.yyAccountAvatarError(img);assert.equal(removed,true);assert.equal(label,'暂无官方头像');assert.equal(img.onerror,null);assert.doesNotMatch(p.context.yyAccountAvatarHTML({id:'unknown'}),/<img|default-head|Steve|狐狸/);
});

test('official game profile data from a previous controller cannot populate the current robot list',async()=>{
 const script=accounts.slice(accounts.indexOf('async function yyCenterCloudAccounts'),accounts.indexOf('async function yyAutoHostCredential'));
 let owner='first',resolve;const host={isConnected:true,textContent:''},context=vm.createContext({yyCenterCloudRows:[],yyCloudReadEpoch:0,yyAccountControllerScope:()=>owner,yyGameAuthAPI:()=>new Promise(r=>resolve=r),document:{getElementById:()=>host}});vm.runInContext(script,context);
 const pending=context.yyCenterCloudAccounts();owner='second';resolve({ok:true,accounts:[{id:'private',avatar_source:'netease',avatar:'https://official.example/old.png'}]});await pending;assert.equal(context.yyCenterCloudRows.length,0);
});

test('cloud credentials and local aliases merge by exact UID and preserve each runtime action',()=>{
 const p=setup();p.context.yyCenterCloudRows=[{id:'one',game_id:'12345678901234567890',nickname:'官方昵称',avatar_source:'netease',avatar:'https://official.example/real.png'}];
 p.context.yyAllAccountRows=[{id:'main',configured:false},{id:'yy_one',name:'设备备注',server:'s',enabled:true},{id:'manual',name:'另一个备注',game_id:'12345678901234567890',server:'other',enabled:false}];p.context.yyAccountRows=p.context.yyAllAccountRows.slice(1);
 const rows=p.context.yyMergedAccountRows();assert.equal(rows.length,1);assert.equal(rows[0].local_accounts.length,2);assert.equal(rows[0].cloud_accounts.length,1);assert.equal(rows[0].name,'官方昵称');assert.equal(rows[0].game_id,'12345678901234567890');
 p.context.yyRenderAccountCards();const html=p.field('yy-accounts-grid').innerHTML;assert.equal((html.match(/<details /g)||[]).length,1);assert.match(html,/real\.png/);assert.equal((html.match(/进入此账号工作区/g)||[]).length,2);assert.match(html,/启用连接/);assert.match(html,/关闭连接/);assert.match(html,/官方账号资料与凭据/);assert.doesNotMatch(html,/托管并配置连接/);
 p.field('yy-account-search').value='另一个备注';p.context.yyRenderAccountCards();assert.match(p.field('yy-accounts-grid').innerHTML,/官方昵称/);p.field('yy-account-search').value='12345678901234567890';p.context.yyRenderAccountCards();assert.match(p.field('yy-accounts-grid').innerHTML,/官方昵称/);
});

test('same nickname never merges accounts and conflicting or imprecise UID cannot attach an official portrait',()=>{
 const p=setup();p.context.yyCenterCloudRows=[{id:'one',game_id:'123',nickname:'same',avatar_source:'netease',avatar:'https://official.example/123.png'},{id:'two',game_id:'456',nickname:'same'}];p.context.yyAllAccountRows=[{id:'yy_one',game_id:'789',name:'same'},{id:'arbitrary',name:'same'}];
 assert.equal(p.context.yyMergedAccountRows().length,4);assert.equal(p.context.yyAccountPortrait(p.context.yyAllAccountRows[0]),'');assert.equal(p.context.yyAccountPortrait(p.context.yyAllAccountRows[1]),'');assert.equal(p.context.yyAccountUID({game_id:9007199254740992}),'');assert.equal(p.context.yyAccountUID({game_id:'9007199254740992'}),'9007199254740992');assert.equal(p.context.yyAccountUID({game_id:'01'}),'');
});

test('cloud-only accounts explain explicit hosting and keep credential actions inline',()=>{
 const p=setup();p.context.yyCenterCloudRows=[{id:'one',game_id:'123',nickname:'cloud'}];p.context.yyRenderAccountCards();const html=p.field('yy-accounts-grid').innerHTML;assert.match(html,/仅云端保存 · 未托管/);assert.match(html,/托管并配置连接/);assert.match(html,/不自动进服/);for(const label of ['查看 / 复制令牌','修改游戏昵称','皮肤预览 / 组件码','刷新官方凭据','重新登录此账号','开启凭据定时刷新','撤销云端凭据'])assert.ok(html.includes(label),label);assert.match(html,/<button class="btn-s" disabled>选择官方头像/);assert.doesNotMatch(html,/yyEnterRow|yyToggleAccount|default-head/);
});

test('hosting saves through existing API with deferred connect, preserves known configuration and never enables a bot',async()=>{
 const p=setup();p.context.yyAccountControllerScope=()=> 'owner';p.context.yyAccountRows=[{id:'yy_one',name:'saved name',server:'saved server',auth_url:'https://auth.example',keep_alive:false,enabled:true}];const requests=[];p.context.yyRootAPI=async(...args)=>{requests.push(args);return {ok:true}};
 vm.runInContext(accounts.slice(accounts.indexOf('async function yyAutoHostCredential'),accounts.indexOf('async function yyPoolCall')),p.context);
 await p.context.yyAutoHostCredential({login_token:'test-only-fixture',account:{id:'one',nickname:'cloud'}});assert.equal(requests.length,1);const body=requests[0][2];assert.equal(body.action,'save');assert.equal(body.defer_connect,true);assert.equal(body.account.server,'saved server');assert.equal(body.account.keep_alive,false);assert.equal(body.account.name,'saved name');assert.equal(body.account.auth_url,'https://auth.example');assert.equal(p.context.yyAccountFocusID,'yy_one');
});

test('credential revoke asks before mutation and a controller change hides late token results',async()=>{
 const p=setup();p.context.yyCenterCloudRows=[{id:'one'}];let scope='first',resolve;const calls=[];p.context.yyAccountControllerScope=()=>scope;p.context.confirm=()=>false;p.context.yyGameAuthAPI=(method,body)=>{calls.push(body);return new Promise(r=>resolve=r)};let shown=false;p.context.yyShowTokenDialog=()=>shown=true;
 await p.context.yyUnifiedCredentialAction(0,'delete');assert.equal(calls.length,0);assert.equal(p.context.yyCredentialOperations.size,0);const pending=p.context.yyUnifiedCredentialAction(0,'token');await p.context.yyUnifiedCredentialAction(0,'token');assert.equal(calls.length,1);scope='second';resolve({ok:true,login_token:'test-only-fixture'});await pending;assert.equal(shown,false);assert.equal(p.context.yyCredentialOperations.size,0);
});

test('hosting cannot open another controller workspace after the cloud refresh completes late',async()=>{
 const p=setup();p.context.yyCenterCloudRows=[{id:'one'}];p.context.yyAccountRows=[{id:'yy_one'}];let scope='first',opened=false; p.context.yyAccountControllerScope=()=>scope;p.context.yyGameAuthAPI=async()=>({ok:true,login_token:'test-only-fixture'});p.context.yyAutoHostCredential=async()=>{};p.context.yyRefreshAccounts=async()=>{};p.context.yyCenterCloudAccounts=async()=>{scope='second'};p.context.yyEditAccount=()=>opened=true;
 await p.context.yyUnifiedCredentialAction(0,'host');assert.equal(opened,false);assert.equal(p.toasts.length,0);
});
