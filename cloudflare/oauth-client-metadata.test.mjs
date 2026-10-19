import test from 'node:test';import assert from 'node:assert/strict';
import {chatGPTClientMetadata} from './oauth-client-metadata.mjs';
import {oauthHandle,oauthRoute} from './oauth.mjs';
const id='https://chatgpt.com/oauth/client.json',redirect='https://chatgpt.com/connector_platform_oauth_redirect';
const official={client_id:id,redirect_uris:[redirect],token_endpoint_auth_method:'private_key_jwt',token_endpoint_auth_methods_supported:['none','private_key_jwt']};
test('ChatGPT official metadata negotiates public PKCE without accepting arbitrary HTTPS hosts',async()=>{
 let calls=0;const fetcher=async(url,options)=>{calls++;assert.equal(url,id);assert.equal(options.redirect,'error');return Response.json(official)};
 const client=await chatGPTClientMetadata(id,'https://toolbox.test',fetcher);assert.equal(client.token_endpoint_auth_method,'none');assert.deepEqual(client.redirect_uris,[redirect]);
 for(const bad of ['https://evil.test/oauth/client.json','http://chatgpt.com/oauth/client.json',id+'?x=1',id+'#x','https://chatgpt.com.evil.test/oauth/client.json','https://chatgpt.com/other','https://user@chatgpt.com/oauth/client.json'])assert.equal(await chatGPTClientMetadata(bad,'https://toolbox.test',fetcher),null);
 assert.equal(calls,1);
 for(const meta of [{...official,client_id:'other'},{...official,redirect_uris:['https://evil.test/callback']},{...official,token_endpoint_auth_methods_supported:['private_key_jwt']}])assert.equal(await chatGPTClientMetadata(id,'https://toolbox.test',async()=>Response.json(meta)),null);
 assert.equal(await chatGPTClientMetadata(id,'https://toolbox.test',async()=>{throw Error('network')}),null);
});
test('authorization accepts official HTTPS identity without DCR and binds its exact callback',async t=>{
 const data=new Map(),storage={get:async k=>structuredClone(data.get(k)),put:async(k,v)=>data.set(k,structuredClone(v)),transaction:async fn=>fn(storage)};
 t.mock.method(globalThis,'fetch',async()=>Response.json(official));
 const origin='https://toolbox.test',meta=await (await oauthRoute(new Request(origin+'/.well-known/oauth-authorization-server'),{})).json();assert.equal(meta.client_id_metadata_document_supported,true);
 const p=new URLSearchParams({client_id:id,redirect_uri:redirect,response_type:'code',resource:origin+'/mcp/oauth/linux-server',code_challenge:'A'.repeat(43),code_challenge_method:'S256',scope:'mcp:read'});
 const run=()=>oauthHandle(new Request('https://internal/__oauth/authorize?'+p,{headers:{'X-OAuth-Origin':origin}}),{storage},{});
 const page=await run();assert.equal(page.status,200);assert.match(await page.text(),/连接游园 MCP/);assert.equal(data.get('oauth-client:'+id).source,'preconfigured-official');
 p.set('redirect_uri','https://chatgpt.com/connector/oauth/unregistered');assert.equal((await run()).status,400);
});
test('preconfigured official stable client works without outbound fetch and never accepts an arbitrary callback',async t=>{
 t.mock.method(globalThis,'fetch',async()=>{throw Error('outbound unavailable')});
 const data=new Map(),storage={get:async k=>data.get(k),put:async(k,v)=>data.set(k,v),transaction:async f=>f(storage)},origin='https://toolbox.test';
 const p=new URLSearchParams({client_id:id,redirect_uri:redirect,response_type:'code',resource:origin+'/mcp/oauth/linux-server',code_challenge:'A'.repeat(43),code_challenge_method:'S256'});
 const run=()=>oauthHandle(new Request('https://internal/__oauth/authorize?'+p,{headers:{'X-OAuth-Origin':origin}}),{storage},{});
 assert.equal((await run()).status,200);assert.equal([...data.keys()].some(k=>k.startsWith('oauth-code:')),false);
 p.set('redirect_uri','https://evil.test/callback');assert.equal((await run()).status,400);
});
