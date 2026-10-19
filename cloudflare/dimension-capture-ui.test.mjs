import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFile} from 'node:fs/promises';

test('automatic all-player view hides manual list and saves all three dimension assignments together',async()=>{
 const fields=new Map(),writes=[],accounts=['main','a','b'];
 const el=id=>{if(!fields.has(id))fields.set(id,{id,value:'',checked:false,hidden:false,disabled:false,innerHTML:'',textContent:'',after(){}});return fields.get(id)};
 const cfg={enabled:true,accounts,players:[],auto_players:true,mode:'camera',interval_seconds:.025,camera_height_blocks:4,camera_pitch_degrees:90,camera_yaw_degrees:0,pause_during_jobs:true,dimension_routing:true,dimension_accounts:{'0':'main','1':'a','2':'b'}};
 const context=vm.createContext({Rcfg:async()=>{},yyReadScope:()=>'',yySettingsViewGuard:()=>()=>true,subPageCache:{},yyIntro:()=>'',yyCardHead:()=>'',yyEscape:String,yySwitch:(id,label,value)=>{el(id).checked=value;return '<input id="'+id+'">'},openSubPage(){},T(){},document:{getElementById:el,querySelectorAll:selector=>selector.includes('yy-gc-accounts')?accounts.map(value=>({value})):[]},yyRootAPI:async(verb,path,body)=>{if(verb==='POST'&&path==='/api/youyuan/global-capture'){writes.push(body);return {ok:true,config:body}};if(path==='/api/youyuan/account-pool')return {result:{accounts:[{id:'a',name:'A'},{id:'b',name:'B'}]}};return {ok:true,advanced_allowed:true,candidates:accounts.map(id=>({id,name:id})),config:cfg,states:{}}},A:async()=>({players:['new-player']})});
 vm.runInContext(await readFile(new URL('../upstream/frontend/js/youyuan_global_capture.js',import.meta.url),'utf8'),context);
 context.yyGlobalMultiChoice=()=>{};context.yyCaptureFleetStepsEditor=()=>({});
 await context.yyGlobalCapturePage();assert.equal(el('yy-gc-auto').checked,true);assert.equal(el('yy-gc-manual-players').hidden,true);
 assert.equal(el('yy-gc-dim-0').value,'main');assert.equal(el('yy-gc-dim-1').value,'a');assert.equal(el('yy-gc-dim-2').value,'b');
 for(const [id,value]of [['yy-gc-residents',3],['yy-gc-interval',25],['yy-gc-height',4],['yy-gc-pitch',90],['yy-gc-yaw',0]])el(id).value=value;
 await el('yy-gc-save').onclick.call(el('yy-gc-save'));
 assert.equal(writes.length,1);assert.equal(writes[0].player_limit,undefined);assert.equal(writes[0].enforce_player_limit,undefined);assert.equal(writes[0].pause_during_jobs,true);assert.equal(writes[0].auto_fleet,true);assert.equal(writes[0].auto_players,true);assert.equal(writes[0].dimension_routing,true);assert.deepEqual(JSON.parse(JSON.stringify(writes[0].dimension_accounts)),{'0':'main','1':'a','2':'b'});
 el('yy-gc-fleet-steps').value='1:1\n8:8\n15:12';await el('yy-gc-save').onclick.call(el('yy-gc-save'));assert.deepEqual(JSON.parse(JSON.stringify(writes[1].fleet_steps)),[{min_players:1,observers:1},{min_players:8,observers:8},{min_players:15,observers:12}]);
 el('yy-gc-auto').checked=false;el('yy-gc-auto').onchange();assert.equal(el('yy-gc-manual-players').hidden,false);
});
