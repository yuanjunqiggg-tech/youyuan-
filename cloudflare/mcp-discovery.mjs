import {readTools} from './mcp-link-policy.mjs';

// Public discovery contains API definitions only. No user data or tool execution.
export const publicDiscoveryMethods=new Set(['initialize','ping','tools/list','notifications/initialized']);
export function oauthToolMetadata(tool){
  const securitySchemes=[{type:'oauth2',scopes:[readTools.has(tool.name)?'mcp:read':'mcp:control']}];
  return {...tool,securitySchemes,_meta:{...tool._meta,securitySchemes}};
}
export function toolAuthRequired(request,packet,scope='mcp:read',error='invalid_token'){
  const u=new URL(request.url),path=u.pathname.replace(/\/$/,''),challenge=`Bearer resource_metadata="${u.origin}/.well-known/oauth-protected-resource${path}", scope="${scope}", error="${error}", error_description="Login to YouYuan to use this tool"`;
  return Response.json({jsonrpc:'2.0',id:packet.id??null,result:{isError:true,content:[{type:'text',text:'请登录游园账号并授权后调用此工具。'}],_meta:{'mcp/www_authenticate':[challenge]}}},{headers:{'Cache-Control':'no-store'}});
}
export async function discoveryResponse(request,env,device,packet){
  if(packet.method==='notifications/initialized')return new Response(null,{status:202});
  const target=new URL(request.url);target.pathname='/mcp/';target.search='';
  const relay=env.RELAY.get(env.RELAY.idFromName(device));
  const response=await relay.fetch(new Request(target,{method:'POST',headers:{'Content-Type':'application/json','Accept':'application/json, text/event-stream'},body:JSON.stringify(packet)}));
  if(!response.ok)return response;
  const body=await response.json();
  if(packet.method==='tools/list'&&body.result?.tools)body.result.tools=body.result.tools.map(oauthToolMetadata);
  return Response.json(body,{headers:{'Cache-Control':'no-store'}});
}
