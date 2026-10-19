import test from 'node:test';import assert from 'node:assert/strict';import vm from 'node:vm';import{readFileSync}from'node:fs';
const source=readFileSync(new URL('../upstream/frontend/js/youyuan_accounts.js',import.meta.url),'utf8');
test('cloud credential save uses returned main ID and does not connect',async()=>{
 const calls=[],c=vm.createContext({yyAccountRows:[],yyAccountControllerScope:()=> 'fixture-owner',yyAccountFocusID:'',yyRootAPI:async(...args)=>{calls.push(args);return {ok:true,id:'main'}}});
 vm.runInContext(source.slice(source.indexOf('async function yyAutoHostCredential('),source.indexOf('async function yyPoolCall(')),c);
 await c.yyAutoHostCredential({login_token:'fixture-token',account:{id:'credential-id',nickname:'fixture-player'}});
 assert.equal(c.yyAccountFocusID,'main');assert.equal(calls.length,1);assert.equal(calls[0][1],'/api/youyuan/accounts');assert.equal(calls[0][2].defer_connect,true);
});
test('manual first-add focuses the account ID accepted by the host',async()=>{
 const fields=new Map(),calls=[];const field=id=>{if(!fields.has(id))fields.set(id,{value:'fixture',checked:false});return fields.get(id)};
 const c=vm.createContext({yyEditingAccount:{id:'requested-child'},document:{getElementById:field},yyRootAPI:async(...args)=>{calls.push(args);return {ok:true,id:'main'}},_savedFormState:{},subPageCache:{},yyAccountFocusID:'',yyNavigate:()=>{},T:()=>{}});
 vm.runInContext(source.slice(source.indexOf('async function yySaveAccount('),source.indexOf('async function yyDeleteAccount(')),c);await c.yySaveAccount();
 assert.equal(c.yyAccountFocusID,'main');assert.equal(calls.length,1);assert.equal(field('yy-ac-token').value,'');assert.equal(field('yy-ac-pass').value,'');
});
