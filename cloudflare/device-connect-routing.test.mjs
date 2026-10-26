import test from 'node:test';
import assert from 'node:assert/strict';
import worker from './worker.mjs';

function fixture() {
  const selected=[];
  const env={DEVICE_TOKEN:'legacy-device-fixture',CONSOLE_TOKEN:'console-fixture',RELAY:{idFromName:n=>n,get:name=>({fetch:async request=>{
    if(name==='identity-registry-v1'){
      const input=await request.json();
      if(input.purpose==='mcp')return Response.json({ok:true,owner:true,device:'linux-server',scope:'control',user:{cloud_allowed:true},relay_prefix:''});
      if(input.token==='owner-device-fixture')return Response.json({ok:true,user:{cloud_allowed:true,legacy_owner:true},relay_prefix:''});
      if(input.token==='member-device-fixture')return Response.json({ok:true,user:{cloud_allowed:true,assigned_devices:['linux-server']},relay_prefix:'tenant:member:'});
      return Response.json({ok:false},{status:401});
    }
    selected.push(name);return Response.json({ok:true,node:name});
  }})}};
  const connect=async(token,{device,query}={})=>{
    const headers={Authorization:'Bearer '+token,Upgrade:'websocket'};if(device!==undefined)headers['X-YouYuan-Device']=device;
    const url=new URL('https://toolbox.test/device/connect');if(query!==undefined)url.searchParams.set('device',query);
    return worker.fetch(new Request(url,{headers}),env);
  };
  const link=()=>worker.fetch(new Request('https://toolbox.test/mcp/link/'+'a'.repeat(64),{method:'POST',body:JSON.stringify({jsonrpc:'2.0',id:1,method:'ping'})}),env);
  return {env,selected,connect,link};
}

test('legacy device with no selector uses primary-device while owner MCP link uses linux-server',async()=>{
  const f=fixture();await f.connect('legacy-device-fixture');await f.link();
  assert.deepEqual(f.selected,['primary-device','linux-server']);
});
test('explicit header or URL query aligns legacy device with the MCP link bucket; header wins conflicts',async()=>{
  for(const options of [{device:'linux-server'},{query:'linux-server'},{device:'linux-server',query:'primary-device'},{device:'',query:'linux-server'}]){
    const f=fixture();await f.connect('legacy-device-fixture',options);await f.link();
    assert.deepEqual(f.selected,['linux-server','linux-server']);
  }
});
test('claimed legacy owner device defaults to shared linux-server without a selector',async()=>{
  const f=fixture();await f.connect('owner-device-fixture');await f.link();
  assert.deepEqual(f.selected,['linux-server','linux-server']);
});
test('assigned members cannot replace shared Linux device by passing its header or query',async()=>{
  const f=fixture();await f.connect('member-device-fixture');await f.connect('member-device-fixture',{device:'linux-server'});await f.connect('member-device-fixture',{query:'linux-server'});
  assert.deepEqual(f.selected,['tenant:member:primary-device','tenant:member:linux-server','tenant:member:linux-server']);
});
test('console token is not accepted as a legacy device token; invalid device selectors are rejected',async()=>{
  const f=fixture();assert.equal((await f.connect('console-fixture',{device:'linux-server'})).status,401);
  assert.equal((await f.connect('legacy-device-fixture',{device:'tenant:someone:linux-server'})).status,400);
  assert.deepEqual(f.selected,[]);
});
