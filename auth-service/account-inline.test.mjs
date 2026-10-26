import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
const source=readFileSync(new URL('./public/console.js',import.meta.url),'utf8');
function setup(){
 const rows=['one','two'].map(id=>{
  const panel={hidden:true,children:[],draft:''},toggle={textContent:'展开'},buttons=Array.from({length:4},()=>({setAttribute(k,v){this[k]=v}}));
  return {dataset:{accountId:id},panel,toggle,buttons,querySelector(selector){return selector==='.auth-account-details'?panel:toggle},querySelectorAll(){return buttons}};
 });
 const calls=[];
 const context=vm.createContext({el:()=>({children:rows}),authExpandedAccounts:new Set(),authBuildDetail(row,panel){calls.push(row.id);panel.children.push({});panel.draft='existing draft'}});
 vm.runInContext(source.slice(source.indexOf('function authAccountCard('),source.indexOf('function authBuildDetail(')),context);
 return {context,rows,calls};
}
test('account row expands in place and closing preserves entered fields',()=>{
 const f=setup();f.context.authOpenDetail({id:'one'});assert.equal(f.rows[0].panel.hidden,false);assert.deepEqual(f.calls,['one']);
 f.rows[0].panel.draft='unsaved nickname';f.context.authOpenDetail({id:'one'});assert.equal(f.rows[0].panel.hidden,true);
 f.context.authOpenDetail({id:'one'});assert.equal(f.rows[0].panel.draft,'unsaved nickname');assert.deepEqual(f.calls,['one']);
 assert.equal(f.rows[0].toggle.textContent,'收起');for(const b of f.rows[0].buttons)assert.equal(b['aria-expanded'],'true');
});
test('multiple account rows can be compared without navigating or collapsing each other',()=>{
 const f=setup();f.context.authOpenDetail({id:'one'});f.context.authOpenDetail({id:'two'});
 assert.equal(f.rows[0].panel.hidden,false);assert.equal(f.rows[1].panel.hidden,false);
 f.context.authSetExpanded({id:'one'},false);assert.equal(f.rows[0].panel.hidden,true);assert.equal(f.rows[1].panel.hidden,false);assert.equal(f.rows[0].toggle.textContent,'展开');
});
test('filtered-out account action does not open a detached detail or fail',()=>{
 const f=setup();assert.doesNotThrow(()=>f.context.authOpenDetail({id:'missing'}));assert.deepEqual(f.calls,[]);
});

test('console uses neutral unknown portraits and only accepts the account official HTTPS portrait',()=>{
 const element=()=>({children:[],append(child){this.children.push(child)},setAttribute(key,value){this[key]=value}});const images=[],opened=[];
 function Image(){this.remove=()=>this.removed=true;images.push(this)}
 const context=vm.createContext({URL,Image,authReturnURL:'https://toolbox.example/',authButton:(label,click)=>({...element(),click}),authText:()=>element(),authOpenDetail:row=>opened.push(row.id)});vm.runInContext(source.slice(source.indexOf('function yyAvatarSrc('),source.indexOf('function authRender(')),context);
 const unknown=context.authAvatar({id:'u',name:'unknown',avatar:'/assets/default-head.jpg'});assert.equal(images.length,0);assert.match(unknown.children[0].innerHTML,/<svg/);unknown.click();assert.deepEqual(opened,['u']);
 const official='https://official.example/avatar.png';context.authAvatar({id:'u',avatar_source:'netease',avatar:official});assert.equal(images.length,1);
 // 头像先走工具箱代理：直连网易 CDN 既慢又可能被墙
 assert.equal(images[0].src,'https://toolbox.example/api/skin/proxy?url='+encodeURIComponent(official));
 // 代理不可用时回退原图，不能直接塌成占位色块
 images[0].onerror();assert.equal(images[0].src,official);assert.notEqual(images[0].removed,true);
 // 原图也失败才撤掉图片，露出中性占位
 images[0].onerror();assert.equal(images[0].removed,true);
 for(const row of [{avatar_source:'local',avatar:'https://platform.example/avatar.png'},{avatar_source:'netease',avatar:'data:image/png;base64,AA'},{avatar_source:'netease',avatar:'https://user:secret@official.example/avatar.png'}])context.authAvatar(row);assert.equal(images.length,1);assert.doesNotMatch(source,/img\.src\s*=\s*['"]\/assets\/default-head/);
});
