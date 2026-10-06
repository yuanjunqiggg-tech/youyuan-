import {updateRoute} from './updates.mjs';
import {identityHandle,identityResolve,identityRoute} from './identity.mjs';
import {readPacketAllowed,readTools} from './mcp-link-policy.mjs';
// 自有域名中继。设备主动连接，所有请求仍由游园执行端真实执行。
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const update=await updateRoute(request,env);if(update)return update;
    if(url.pathname.startsWith('/api/identity/'))return identityRoute(request,env);
    let rawToken=(request.headers.get('Authorization')||'').replace(/^Bearer /,''),linkGrant=null,linkPacket=null;
    const link=url.pathname.match(/^\/mcp\/link\/([a-f0-9]{64})\/?$/);
    if(link){
      linkGrant=await identityResolve(env,link[1],'mcp');if(!linkGrant)return Response.json({error:'专用连接已撤销或过期'},{status:401});
      if(!linkGrant.user.cloud_allowed)return Response.json({error:'订阅已到期',code:'SUBSCRIPTION_REQUIRED'},{status:403});
      url.pathname='/mcp/';url.search='';url.searchParams.set('device',linkGrant.device);
      const headers=new Headers(request.headers);headers.delete('X-YouYuan-Device');
      request=new Request(url,{method:request.method,headers,body:['GET','HEAD'].includes(request.method)?undefined:request.body,duplex:'half'});
      if(linkGrant.scope==='read'&&request.method==='POST'){try{linkPacket=await request.clone().json()}catch{return Response.json({error:'JSON无效'},{status:400})};if(!readPacketAllowed(linkPacket))return Response.json({jsonrpc:'2.0',id:linkPacket.id||null,error:{code:-32003,message:'此连接仅可查询状态与开发文档'}},{status:200});}
    }
    const isOwner=!!linkGrant?.owner||!!env.CONSOLE_TOKEN&&rawToken===env.CONSOLE_TOKEN;
    const isLegacyDevice=!!env.DEVICE_TOKEN&&rawToken===env.DEVICE_TOKEN;
    let identity=linkGrant;
    if(!identity&&rawToken&&!isOwner&&!isLegacyDevice)identity=await identityResolve(env,rawToken,url.pathname==='/device/connect'?'device':'session');
    if(!identity&&rawToken&&!isOwner&&!isLegacyDevice&&(url.pathname.startsWith('/ai/')||url.pathname.startsWith('/api/library/')))identity=await identityResolve(env,rawToken,'device');
    const testAIAllowed=isOwner||isLegacyDevice||!!identity?.user?.legacy_owner;
    if(url.pathname==='/api/cloud/ai-capability')return Response.json({ok:true,allowed:testAIAllowed},{status:rawToken&&(isOwner||isLegacyDevice||identity)?200:401});
    // 用户选定的认证提供方与工具箱控制 API 分离；设备密钥只在自有入口验证，不传给提供方。
    if (url.pathname.startsWith('/auth/')) {
      const deviceKey=request.headers.get('X-YouYuan-Device-Key');
      if (!deviceKey || (deviceKey!==env.DEVICE_TOKEN && !await identityResolve(env,deviceKey,'device'))) return Response.json({error:'设备密钥不正确'},{status:401});
      const path=url.pathname.slice(5);
      if (!['/api/new','/api/phoenix/login','/api/phoenix/transfer_start_type','/api/phoenix/transfer_check_num','/api/phoenix/tan_lobby_login','/api/phoenix/tan_lobby_create','/api/phoenix/tan_lobby_debug'].includes(path)) return Response.json({error:'未开放的认证操作'},{status:404});
      if(!env.AUTH_SERVICE)return Response.json({error:'游园独立认证服务尚未部署'},{status:503});
      const headers=new Headers(request.headers);headers.delete('Host');
      const target='https://auth.youyuanqi.dpdns.org'+path+url.search;
      try{return await env.AUTH_SERVICE.fetch(new Request(target,{method:request.method,headers,body:['GET','HEAD'].includes(request.method)?undefined:request.body}))}catch{return Response.json({error:'游园独立认证暂时不可用'},{status:502})}
    }
    if(url.pathname.startsWith('/ai/')) {
      const token=request.headers.get('Authorization')||'';
      if(!testAIAllowed)return Response.json({error:{message:'云端测试 AI 仅限项目测试账号，请使用自己的模型密钥'}},{status:403});
      if(url.pathname!=='/ai/v1/chat/completions'||request.method!=='POST')return Response.json({error:{message:'只开放 chat/completions'}},{status:404});
      try {
        const text=await request.text();if(text.length>120000)return Response.json({error:{message:'上下文过大，请先压缩'}},{status:413});
        const body=JSON.parse(text),model='@cf/meta/llama-3.3-70b-instruct-fp8-fast';
        if(!Array.isArray(body.messages))return Response.json({error:{message:'缺少 messages'}},{status:400});
        const input={messages:body.messages,stream:false,max_tokens:Math.min(4096,Math.max(256,body.max_tokens||2048)),temperature:.2};
        if(body.tools?.length)input.tools=body.tools.map(tool=>tool.function||tool);
        const raw=await env.AI.run(model,input);
        let data=raw;if(raw instanceof Response){if(!raw.ok)return raw;data=await raw.json()}
        if(data.error)return Response.json({error:{message:JSON.stringify(data.error)}},{status:502});
        const calls=(data.tool_calls||[]).map(call=>{const fn=call.function||call;return {id:call.id||'call_'+crypto.randomUUID(),type:'function',function:{name:fn.name,arguments:typeof fn.arguments==='string'?fn.arguments:JSON.stringify(fn.arguments||{})}}});
        const message={role:'assistant',content:data.response||''};if(calls.length)message.tool_calls=calls;
        const result={id:'yy_'+crypto.randomUUID(),object:'chat.completion',created:Math.floor(Date.now()/1000),model,choices:[{index:0,message,finish_reason:calls.length?'tool_calls':'stop'}],usage:data.usage||{}};
        if(body.stream){const chunk={...result,object:'chat.completion.chunk',choices:[{index:0,delta:message,finish_reason:result.choices[0].finish_reason}]};return new Response('data: '+JSON.stringify(chunk)+'\n\ndata: [DONE]\n\n',{headers:{'content-type':'text/event-stream'}})}
        return Response.json(result);
      }catch(error){return Response.json({error:{message:'Workers AI 调用失败：'+error.message}},{status:502})}
    }
    const device = url.pathname === '/device/connect';
    const qqMatch=url.pathname.match(/^\/qq\/webhook\/([a-f0-9-]{36})\/([A-Za-z0-9_-]{1,48})(?:\/([A-Za-z0-9_-]{1,64}))?$/);
    const legacyAccount=url.pathname.match(/^\/qq\/webhook\/accounts\/([A-Za-z0-9_-]{1,64})$/);
    const qqCallback = (url.pathname === '/qq/webhook'||!!qqMatch||!!legacyAccount) && request.method === 'POST';
    const protectedPath = device || url.pathname.startsWith('/api/') || url.pathname.startsWith('/mcp') || url.pathname==='/health';
    // Check entitlement on every request, including existing control tokens.
    // Register/login/me remain available without cloud execution permission.
    if(protectedPath&&identity&&!identity.user.cloud_allowed)return Response.json({ok:false,error:'云端工具箱仅限管理员或有效订阅用户',code:'SUBSCRIPTION_REQUIRED',user:identity.user},{status:403});
    if (!protectedPath && !qqCallback && url.pathname !== '/health') return env.ASSETS.fetch(request);
    const supplied = request.headers.get('Authorization') ?? '';
    const expected = 'Bearer ' + (device ? env.DEVICE_TOKEN : env.CONSOLE_TOKEN);
    const library = url.pathname.startsWith('/api/library/');
    const libraryDevice = library && env.DEVICE_TOKEN && supplied==='Bearer '+env.DEVICE_TOKEN;
    if (!qqCallback && !libraryDevice && !identity && (!expected.slice(7) || supplied !== expected)) return Response.json({error:'中继密钥不正确'},{status:401});
    if(url.pathname==='/api/game-auth/accounts'){
      if(!env.AUTH_SERVICE)return Response.json({ok:false,error:'独立认证服务尚未部署'},{status:503});
      const target=new URL(request.url);target.hostname='auth.youyuanqi.dpdns.org';target.pathname='/api/accounts';return env.AUTH_SERVICE.fetch(new Request(target,request));
    }
    if (url.pathname==='/api/cloud/status') return Response.json({ok:true,product:'游园独立控制后端',version:'0.10.11',domain:url.hostname,runtime:'Cloudflare Workers + Durable Objects',auth_origin:'https://auth.youyuanqi.dpdns.org',execution:'APK 或 Linux 独立执行端主动连接；按设备隔离',user:identity?.user||null});
    const requestedDevice=request.headers.get('X-YouYuan-Device') || url.searchParams.get('device') || 'primary-device';
    if(!/^[A-Za-z0-9_-]{1,48}$/.test(requestedDevice))return Response.json({error:'设备标识无效'},{status:400});
    // Assigned control permission cannot replace the shared node connection.
    const assigned=!device&&identity?.user?.assigned_devices?.includes(requestedDevice)&&requestedDevice.startsWith('linux-');
    const deviceId = qqMatch?('tenant:'+qqMatch[1]+':'+qqMatch[2]):legacyAccount?'primary-device':(assigned?'':identity?.relay_prefix||'')+requestedDevice;
    const relay = env.RELAY.get(env.RELAY.idFromName(deviceId));
    if (qqCallback) {const account=legacyAccount?.[1]||qqMatch?.[3];url.pathname=account&&account!=='main'?'/api/youyuan/accounts/invoke/'+account+'/api/qq/webhook':'/api/qq/webhook';request=new Request(url,request);}
    const response=await relay.fetch(request);
    if(linkPacket?.method==='tools/list'&&response.ok){const result=await response.json();if(result.result?.tools)result.result.tools=result.result.tools.filter(t=>readTools.has(t.name));return Response.json(result,{status:response.status})}
    return response;
  }
};

export class YouYuanRelay {
  constructor(ctx, env) {
    this.ctx=ctx;this.env=env;this.socket=ctx.getWebSockets?.()[0] || null;this.pending=new Map();
    // Exactly matches the existing APK/Linux heartbeat. Native auto-response
    // keeps idle sockets hibernatable without invoking this JS handler.
    if(typeof ctx.setWebSocketAutoResponse==='function' && typeof WebSocketRequestResponsePair!=='undefined'){
      ctx.setWebSocketAutoResponse(new WebSocketRequestResponsePair('{"kind":"ping"}','{"kind":"pong"}'));
    }
  }
  async fetch(request) {
    const url=new URL(request.url);
    if(url.pathname.startsWith('/__identity/'))return identityHandle(request,this.ctx,this.env);
    if(url.pathname.startsWith('/api/library/'))return this.library(request,url);
    if(url.pathname==='/device/connect') {
      if(request.headers.get('Upgrade')?.toLowerCase()!=='websocket')return Response.json({error:'需要 WebSocket'},{status:426});
      // An authenticated replacement recovers a stale session after APK upgrade.
      // Pending writes are reported as unknown and never automatically replayed.
      if(this.socket){const old=this.socket;this.webSocketClose(old);try{old.close(1012,'device reconnected')}catch{}}
      const pair=new WebSocketPair();const [client,server]=Object.values(pair);this.ctx.acceptWebSocket(server,['device']);this.socket=server;
      return new Response(null,{status:101,webSocket:client});
    }
    if(!this.socket||this.socket.readyState!==1)return Response.json({error:'游园执行端未连接'},{status:503});
    // JSON MCP 与普通 API 转发；APK 内导入不受云端传输体积影响。
    if(!url.pathname.startsWith('/api/')&&!url.pathname.startsWith('/mcp')&&url.pathname!=='/health')return Response.json({error:'路径不可转发'},{status:404});
    if(this.pending.size>=64)return Response.json({error:'该设备待处理请求过多，请稍后重试'},{status:429,headers:{'Retry-After':'2'}});
    const body=await request.arrayBuffer();
    if(body.byteLength>32*1024*1024)return Response.json({error:'单次云端传输超过 32MB；可在 APK 导入后由云端启动任务，方块数量不受此限制'},{status:413});
    // Body reads can interleave; recheck immediately before reserving an entry.
    if(this.pending.size>=64)return Response.json({error:'该设备待处理请求过多，请稍后重试'},{status:429,headers:{'Retry-After':'2'}});
    const binary=new Uint8Array(body),chunks=[];for(let offset=0;offset<binary.length;offset+=32768)chunks.push(String.fromCharCode(...binary.subarray(offset,offset+32768)));const bytes=chunks.join('');
    const id=crypto.randomUUID();
    return new Promise(resolve=>{
      const timer=setTimeout(()=>{this.pending.delete(id);resolve(Response.json({error:'执行端响应超时；结果可能未知，请查询日志'},{status:504}));},150000);
      this.pending.set(id,{resolve,timer});
      try{this.socket.send(JSON.stringify({id,method:request.method,path:url.pathname+url.search,body:btoa(bytes),headers:{'Content-Type':request.headers.get('Content-Type')??'application/json','X-YouYuan-Market-Authorization':url.pathname.includes('market-proxy')?(request.headers.get('X-YouYuan-Market-Authorization')??''):'','Accept':request.headers.get('Accept')??'application/json','MCP-Protocol-Version':request.headers.get('MCP-Protocol-Version')??'','X-Signature-Ed25519':request.headers.get('X-Signature-Ed25519')??'','X-Signature-Timestamp':request.headers.get('X-Signature-Timestamp')??'','X-Bot-Appid':request.headers.get('X-Bot-Appid')??''}}));}
      catch(error){clearTimeout(timer);this.pending.delete(id);resolve(Response.json({error:'执行端连接失效'},{status:503}));}
    });
  }
  // 自建资料库实际写入 Durable Objects 持久存储，与原作者市场无关。
  async library(request,url) {
    if(url.pathname==='/api/library/list') {
      const rows=await this.ctx.storage.list({prefix:'library-meta:'});
      return Response.json({ok:true,items:[...rows.values()].sort((a,b)=>b.created-a.created)});
    }
    if(url.pathname==='/api/library/upload' && request.method==='POST') {
      const form=await request.formData();const file=form.get('file');
      if(!file || typeof file.arrayBuffer!=='function')return Response.json({ok:false,error:'缺少文件'},{status:400});
      if(file.size>32*1024*1024)return Response.json({ok:false,error:'云端单文件超过32 MB；更大文件可在 APK 本地导入'},{status:413});
      const bytes=new Uint8Array(await file.arrayBuffer());const id=crypto.randomUUID();const chunks=[];
      for(let start=0;start<bytes.length;start+=65536) {
        const slice=bytes.subarray(start,start+65536);let text='';
        for(let n=0;n<slice.length;n+=8192)text+=String.fromCharCode(...slice.subarray(n,n+8192));
        await this.ctx.storage.put('library-data:'+id+':'+chunks.length,btoa(text));chunks.push(slice.length);
      }
      const digest=await crypto.subtle.digest('SHA-256',bytes);
      const item={id,name:String(file.name||'file').replace(/[\\/]/g,'_'),category:String(form.get('category')||'plugins'),bytes:bytes.length,chunks:chunks.length,sha256:[...new Uint8Array(digest)].map(v=>v.toString(16).padStart(2,'0')).join(''),created:Date.now()};
      await this.ctx.storage.put('library-meta:'+id,item);return Response.json({ok:true,item});
    }
    const id=url.searchParams.get('id');if(!id || !/^[a-f0-9-]{36}$/.test(id))return Response.json({ok:false,error:'文件标识无效'},{status:400});
    const item=await this.ctx.storage.get('library-meta:'+id);if(!item)return Response.json({ok:false,error:'文件不存在'},{status:404});
    if(url.pathname==='/api/library/download') {
      const data=new Uint8Array(item.bytes);let offset=0;
      for(let n=0;n<item.chunks;n++){const text=atob(await this.ctx.storage.get('library-data:'+id+':'+n));for(let i=0;i<text.length;i++)data[offset++]=text.charCodeAt(i);}
      return new Response(data,{headers:{'Content-Type':'application/octet-stream','Content-Disposition':"attachment; filename*=UTF-8''"+encodeURIComponent(item.name),'X-Content-SHA256':item.sha256}});
    }
    if(url.pathname==='/api/library/delete' && request.method==='POST') {
      const names=['library-meta:'+id];for(let n=0;n<item.chunks;n++)names.push('library-data:'+id+':'+n);await this.ctx.storage.delete(names);return Response.json({ok:true});
    }
    return Response.json({ok:false,error:'未知资料库操作'},{status:404});
  }
  webSocketMessage(socket,message) {
    if(socket!==this.socket)return;
    try{const packet=JSON.parse(message);if(packet.kind==='ping'){socket.send(JSON.stringify({kind:'pong'}));return;}const waiter=this.pending.get(packet.id);if(!waiter)return;clearTimeout(waiter.timer);this.pending.delete(packet.id);waiter.resolve(new Response(Uint8Array.from(atob(packet.body??''),c=>c.charCodeAt(0)),{status:packet.status,headers:packet.headers}));}catch(error){console.error('设备回传错误',error.message);}
  }
  webSocketClose(socket) {
    if(socket!==this.socket)return;this.socket=null;
    for(const waiter of this.pending.values()){clearTimeout(waiter.timer);waiter.resolve(Response.json({error:'APK 执行端离线，写操作未重试'},{status:503}));}this.pending.clear();
  }
  webSocketError(socket) {this.webSocketClose(socket);}
}
