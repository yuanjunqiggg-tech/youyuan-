import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFile} from 'node:fs/promises';
const source=await readFile(new URL('../upstream/frontend/js/youyuan_identity_recovery.js',import.meta.url),'utf8');
function fixture(cloud=true){
 const events={},nodes=new Map(),state={created:0,ready:0};
 const c={yyIsCloud:cloud,yyCloudBlocked:true,yyPlatformUser:null,yyLinuxTransportStatus:{auth_required:true,last_error:'登录状态已失效，请重新登录'},yyIdentityReady(){state.ready++},yyShowIdentity(){state.created++;nodes.set('yy-identity-gate',{style:{},setAttribute(k,v){this[k]=v}});nodes.set('yy-login-name',{});nodes.set('yy-login-result',{textContent:''})},document:{getElementById:id=>nodes.get(id),addEventListener(name,fn){events['doc:'+name]=fn}},addEventListener(name,fn){events[name]=fn}};
 c.window=c;vm.createContext(c);vm.runInContext(source,c);return {c,events,nodes,state};
}
test('expired platform login opens one gate above export UI and preserves entered login',()=>{const f=fixture();f.nodes.set('yy-export-center',{retained:true});f.events['yy-cloud-auth-expired']();f.nodes.get('yy-login-name').value='pending';f.events['yy-cloud-auth-expired']();assert.equal(f.state.created,1);assert.equal(f.nodes.get('yy-login-name').value,'pending');assert.equal(f.nodes.get('yy-identity-gate').style.zIndex,'2147483000');assert.equal(f.nodes.get('yy-export-center').retained,true);assert.match(f.nodes.get('yy-login-result').textContent,/设备上的机器人、文件与任务会保留/)});
test('successful eligible login clears stale auth and subscription gates before normal navigation',()=>{const f=fixture();f.c.yyIdentityReady({cloud_allowed:true});assert.equal(f.c.yyCloudBlocked,false);assert.equal(f.c.yyLinuxTransportStatus.auth_required,false);assert.equal(f.state.ready,1)});
test('ineligible identity does not clear an actual subscription restriction',()=>{const f=fixture();f.c.yyIdentityReady({cloud_allowed:false});assert.equal(f.c.yyCloudBlocked,true);assert.equal(f.c.yyLinuxTransportStatus.auth_required,true)});
test('local application identity is not replaced by cloud authentication events',()=>{const f=fixture(false);f.events['yy-cloud-auth-expired']();assert.equal(f.state.created,0);assert.equal(f.nodes.size,0)});
