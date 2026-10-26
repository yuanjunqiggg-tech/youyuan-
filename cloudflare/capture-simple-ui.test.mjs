import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../upstream/frontend/js/youyuan_global_capture.js',import.meta.url),'utf8');
function setup({candidates=['main','a','b','c','d'],configured=true}={}){
 const fields=new Map(),pages=[],calls=[],messages=[],boxes=[];
 const field=id=>{if(!fields.has(id))fields.set(id,{id,value:'',checked:false,disabled:false,textContent:'',innerHTML:'',insertAdjacentHTML(position,html){this.innerHTML+=html;parse(html)},closest(){return {querySelector:()=>({closest:()=>({after(box){parse(box.innerHTML)}})})}}});return fields.get(id)};
 function parse(html){for(const m of html.matchAll(/id="([^"]+)"/g))field(m[1]);for(const m of html.matchAll(/<input[^>]+id="([^"]+)"[^>]+value="([^"]+)"/g))field(m[1]).value=m[2]}
 const response={ok:true,advanced_allowed:false,config:{enabled:true,accounts:['main'],interval_seconds:.015,resident_observers:3,dimension_routing:true,pause_during_jobs:true,player_limit:22,enforce_player_limit:true},candidates:candidates.map(id=>({id,name:id})),states:{}};
 const context=vm.createContext({Rcfg:async()=>{},yyActiveAccount:'main',yyAccountMainConfigured:configured,yySettingsViewGuard:()=>()=>true,subPageCache:{},yyIntro:(title,desc)=>'<h2>'+title+'</h2><p>'+desc+'</p>',yyEscape:String,yySwitch:(id,label,value)=>{field(id).checked=!!value;return '<input type="checkbox" id="'+id+'">'+label},openSubPage:(title,html)=>{pages.push({title,html});parse(html)},T:(message,kind)=>messages.push({message,kind}),document:{createElement:()=>({innerHTML:"",className:""}),getElementById:id=>fields.get(id)||null,querySelectorAll:selector=>selector.endsWith('input:checked')?boxes.filter(b=>b.checked):boxes},A:async()=>{throw Error('ordinary page must not query player list')},yyRootAPI:async(method,path,body)=>{calls.push({method,path,body});return method==='POST'?{ok:true,config:{...response.config,...body}}:response}});
 vm.runInContext(source,context);
 context.yyGlobalMultiChoice=(host,items,selected)=>{boxes.length=0;for(const item of items)boxes.push({value:item.id,checked:selected.includes(item.id)})};
 return {context,fields,field,pages,calls,messages,boxes,response};
}
test('ordinary observation view exposes account behavior without frequency or allocation internals',async()=>{
 const p=setup();await p.context.yyGlobalCapturePage();assert.equal(p.calls.length,1);assert.deepEqual(p.boxes.map(b=>b.value),['a','b','c','d']);
 const html=p.pages[0].html;assert.match(html,/可不选额外账号/);assert.match(html,/告知玩家观测用途/);assert.match(html,/运行资源/);assert.match(html,/导入导出期间自动暂停/);
 assert.doesNotMatch(html,/yy-gc-(?:count|capacity|fleet-steps|interval|mode|height|pitch|yaw|enforce-limit|player-limit)/);
 await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));const write=p.calls.find(c=>c.method==='POST').body;
 assert.deepEqual(JSON.parse(JSON.stringify(write)),{enabled:true,accounts:['main'],game_mode:'keep',invisible:null,quick_tp:true,auto_respawn:null,dimension_landings:{},dimension_game_modes:{}});assert.match(p.field('yy-gc-status').textContent,/覆盖可能不足/);
});
test('extra observation accounts support more than two, while clearing them keeps the configured current robot',async()=>{
 const p=setup();await p.context.yyGlobalCapturePage();p.field('yy-gc-all').onclick();await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));
 assert.deepEqual(Array.from(p.calls.at(-1).body.accounts),['main','a','b','c','d']);
 p.field('yy-gc-none').onclick();await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));assert.deepEqual(Array.from(p.calls.at(-1).body.accounts),['main']);assert.equal(p.calls.at(-1).body.resident_observers,undefined);assert.equal(p.calls.at(-1).body.pause_during_jobs,undefined);
});
test('an unconfigured main candidate is removed and empty selection cannot pretend observation is enabled',async()=>{
 const p=setup({configured:false});await p.context.yyGlobalCapturePage();assert.deepEqual(p.boxes.map(b=>b.value),['a','b','c','d']);assert.match(p.field('yy-gc-primary').textContent,/没有可用/);
 await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));assert.equal(p.calls.filter(c=>c.method==='POST').length,0);assert.match(p.messages.at(-1).message,/至少一个可用账号/);
 p.boxes[0].checked=true;await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));assert.deepEqual(Array.from(p.calls.at(-1).body.accounts),['a']);
});
test('missing candidates never creates a fictitious main robot',async()=>{
 const p=setup({candidates:[]});await p.context.yyGlobalCapturePage();assert.equal(p.boxes.length,0);assert.match(p.field('yy-gc-accounts').textContent,/暂无可选/);await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));assert.equal(p.calls.length,1);
 p.field('yy-gc-enabled').checked=false;await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));assert.deepEqual(JSON.parse(JSON.stringify(p.calls.at(-1).body)),{enabled:false,accounts:[],game_mode:'keep',invisible:null,quick_tp:true,auto_respawn:null,dimension_landings:{},dimension_game_modes:{}});
});
test('capture save failure stays visible and never reports success or silently rewrites selection',async()=>{
 const p=setup();await p.context.yyGlobalCapturePage();p.boxes[2].checked=true;p.context.yyGlobalCaptureSave=async()=>({ok:false,error:'没有服务器管理权限'});await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));
 assert.equal(p.messages.at(-1).kind,'e');assert.equal(p.messages.at(-1).message,'没有服务器管理权限');assert.equal(p.boxes[2].checked,true);assert.equal(p.field('yy-gc-save').disabled,false);
});
test('server player limit lives in connection settings and preserves device owner authority independently of advanced observation access',async()=>{
 const p=setup();p.field('cfg-conn-card');p.context.yyServerPlayerLimitSettings(p.response);const html=p.field('cfg-conn-card').innerHTML;assert.match(html,/服务器真人人数规则/);assert.match(html,/默认关闭/);assert.match(html,/按所设人数/);assert.doesNotMatch(html,/yy-gc-interval|fleet-steps/);
 p.field('yy-server-player-limit-value').value='22';p.field('yy-server-enforce-limit').checked=true;await p.field('yy-server-player-limit-save').onclick.call(p.field('yy-server-player-limit-save'));
 assert.deepEqual(JSON.parse(JSON.stringify(p.calls[0].body)),{enforce_player_limit:true,player_limit:22});assert.equal(p.calls[0].path,'/api/youyuan/global-capture');assert.match(p.field('yy-server-player-limit-result').textContent,/已保存/);
 p.field('yy-server-player-limit-value').value='22.5';await p.field('yy-server-player-limit-save').onclick.call(p.field('yy-server-player-limit-save'));assert.equal(p.calls.length,1);assert.match(p.field('yy-server-player-limit-result').textContent,/整数/);
});
test('a rejected settings response does not expose or save server player limit controls',()=>{
 const p=setup();p.field('cfg-conn-card');p.context.yyServerPlayerLimitSettings({ok:false,error:'只能配置自己设备的账号池'});assert.equal(p.field('cfg-conn-card').innerHTML,'');assert.equal(p.calls.length,0);
});

test('player limit is off when unset and preserves an explicitly saved choice',()=>{
 for(const enabled of [undefined,false,true]){const p=setup();p.field('cfg-conn-card');p.context.yyServerPlayerLimitSettings({...p.response,config:{player_limit:22,enforce_player_limit:enabled}});assert.equal(p.field('yy-server-enforce-limit').checked,enabled===true);assert.equal(p.calls.length,0)}
});

test('players can save game mode, visibility, and quick teleport together',async()=>{
 const p=setup();await p.context.yyGlobalCapturePage();
 p.field('yy-gc-game-mode').value='spectator';p.field('yy-gc-invisible').value='off';p.field('yy-gc-quick-tp').checked=false;
 await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));
 const saved=p.calls.find(c=>c.method==='POST').body;
 assert.equal(saved.game_mode,'spectator');assert.equal(saved.invisible,false);assert.equal(saved.quick_tp,false);
 assert.equal(p.messages.at(-1).kind,'o');
});

test('dimension entry controls preserve landings and save explicit modes and respawn choice',async()=>{
 const p=setup();p.response.config.dimension_landings={'1':{x:-10000,y:100,z:-10000}};
 await p.context.yyGlobalCapturePage();
 assert.equal(p.field('yy-gc-landing-1').checked,true);assert.equal(p.field('yy-gc-landing-1-x').value,-10000);
 assert.equal(p.field('yy-gc-landing-0-x').disabled,true);
 p.field('yy-gc-entry-mode-1').value='spectator';p.field('yy-gc-respawn').value='off';
 await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));
 const write=p.calls.at(-1).body;
 assert.equal(write.auto_respawn,false);assert.equal(write.dimension_game_modes['1'],'spectator');
 assert.deepEqual(JSON.parse(JSON.stringify(write.dimension_landings)),{'1':{x:-10000,y:100,z:-10000}});
 p.field('yy-gc-landing-2').checked=true;
 await p.field('yy-gc-save').onclick.call(p.field('yy-gc-save'));
 assert.match(p.messages.at(-1).message,/完整的上线坐标/);
 assert.equal(p.calls.filter(c=>c.method==='POST').length,1);
});

test('capture settings have an independent entry without auto-connect fields',async()=>{
 const p=setup();let appended;
 p.context.document.querySelector=selector=>selector==='#tab-settings>.content-inner'?{append:node=>{appended=node}}:null;
 p.context.document.createElement=()=>p.field('generated-capture-card');
 assert.equal(p.fields.has('cfg-auto-connect'),false);
 await p.context.Rcfg();
 assert.equal(appended.id,'cfg-capture-card');assert.ok(p.fields.has('cfg-global-capture'));
 assert.equal(p.field('cfg-global-capture').checked,true);
});
