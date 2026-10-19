// ChatGPT's HTTPS client identity is resolved only on its official host.
// Dynamic registration remains available for other public PKCE clients.
// Reviewed against https://chatgpt.com/oauth/client.json on 2026-10-08.
// This explicit server-side registration avoids an outbound availability
// dependency for the official stable callback. It is not an access grant.
export function registeredChatGPTClient(id,issuer){
 if(id!=='https://chatgpt.com/oauth/client.json')return null;
 return {client_id:id,client_name:'ChatGPT',redirect_uris:['https://chatgpt.com/connector_platform_oauth_redirect'],issuer,
   source:'preconfigured-official',token_endpoint_auth_method:'none',metadata_source:id,metadata_reviewed:'2026-10-08'};
}
export async function chatGPTClientMetadata(id,issuer,fetcher=fetch){
  let url;try{url=new URL(id)}catch{return null}
  if(url.origin!=='https://chatgpt.com'||url.username||url.password||url.search||url.hash||
    !(/^\/oauth\/client\.json$/.test(url.pathname)||/^\/oauth\/[A-Za-z0-9_-]{1,128}\/client\.json$/.test(url.pathname)))return null;
  try{
    const response=await fetcher(url.href,{redirect:'error',headers:{Accept:'application/json'},signal:AbortSignal.timeout(8000)});
    if(!response.ok)return null;
    const text=await response.text();if(text.length>32768)return null;
    const meta=JSON.parse(text),expected=url.pathname==='/oauth/client.json'
      ?'https://chatgpt.com/connector_platform_oauth_redirect'
      :'https://chatgpt.com/connector/oauth/'+url.pathname.split('/')[2];
    // The official document advertises private_key_jwt as its preference and
    // explicitly allows "none" as an alternative. We negotiate none + S256.
    const methods=meta.token_endpoint_auth_methods_supported||[meta.token_endpoint_auth_method||'none'];
    if(meta.client_id!==id||!Array.isArray(meta.redirect_uris)||!meta.redirect_uris.includes(expected)||
      !Array.isArray(methods)||!methods.includes('none'))return null;
    return {client_id:id,client_name:'ChatGPT',redirect_uris:[expected],issuer,source:'cimd',
      token_endpoint_auth_method:'none',fetched_at:Date.now()};
  }catch{return null}
}
