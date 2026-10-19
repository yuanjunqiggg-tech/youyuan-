import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('./public/return-context.js',import.meta.url),'utf8');
function page(returnTo){const links=[{},{}];const context=vm.createContext({URL,location:{href:'https://auth.youyuanqi.dpdns.org/?return_to='+encodeURIComponent(returnTo)},document:{querySelectorAll:()=>links}});vm.runInContext(source,context);return {context,links};}
test('APK return retains explicit app return and does not switch to a cloud instance',()=>{
 const {context,links}=page('https://toolbox.youyuanqi.dpdns.org/?return_to_app=1');
 assert.equal(context.authReturnURL,'https://toolbox.youyuanqi.dpdns.org/?return_to_app=1');
 for(const link of links)assert.equal(link.href,context.authReturnURL);
});
test('web return preserves the chosen device while removing unrelated parameters and credentials',()=>{
 const {context}=page('https://toolbox.youyuanqi.dpdns.org/?device=mine&token=secret#private');
 assert.equal(context.authReturnURL,'https://toolbox.youyuanqi.dpdns.org/?device=mine');
 assert.equal(page('http://127.0.0.1:19429/?token=secret').context.authReturnURL,'http://127.0.0.1:19429/');
 for(const bad of ['https://toolbox.youyuanqi.dpdns.org.evil.com/','javascript:alert(1)','https://user:pass@toolbox.youyuanqi.dpdns.org/','http://toolbox.youyuanqi.dpdns.org/','http://localhost:9999/'])assert.equal(page(bad).context.authReturnURL,'https://toolbox.youyuanqi.dpdns.org/');
});
